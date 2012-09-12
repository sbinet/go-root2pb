package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"


	// import so root2pb-cnv is up to date...
	_ "github.com/sbinet/go-root2pb/pbutils"

)

var fname = flag.String("f", "", "path to input ROOT file")
var oname = flag.String("o", "out/event.proto", "path to output .proto file")
var tname = flag.String("t", "", "name of the ROOT TTree to convert")
var brsel = flag.String("sel", "", "comma-separated list of glob-patterns to select (with +foo*) and remove (with -foo*) branches from the output .proto file")
var pb_pkg_name = flag.String("pkg", "event", "name of the protobuf package to be generated")
var pb_msg_name = flag.String("msg", "Event", "name of the top-level message encoding the TTree")
var do_gen = flag.String("gen", "", "generate the pb file(s) from the .proto one for each of output languages (go,py,cpp,java)")
var do_cnv = flag.Bool("cnv", false, "convert the ROOT TTree's content into a binary pbuf file using the generated .pb.go package")
var verbose = flag.Bool("v", false, "verbose")

var rt2pb_typemap = map[string]string{
	"Char_t":   "bytes",
	"Bool_t":   "bool",
	"UInt_t":   "uint32",
	"Int_t":    "int32",
	"Int32_t":  "int32",
	"Long_t":   "int64",
	"Long64_t": "int64",
	"Float_t":  "float",
	"Double_t": "double",

	"std::string": "string",
	"string":      "string",

	"unsigned short": "uint32",
	"unsigned int":   "uint32",
	"unsigned long":  "uint64",

	"short": "int32",
	"int":   "int32",
	"long":  "int64",
}

func get_pb_type(typename string) (pb_type string, isrepeated bool) {
	v, ok := rt2pb_typemap[typename]
	if ok {
		return v, false
	}
	v = typename
	if strings.HasPrefix(v, "vector<") {
		v = strings.TrimSpace(v[len("vector<") : len(v)-1])
		isrepeated = true
	}
	if strings.HasPrefix(v, "std::vector<") {
		v = strings.TrimSpace(v[len("std::vector<") : len(v)-1])
		isrepeated = true
	}
	if isrepeated {
		v, _ = get_pb_type(v)
	}
	return v, isrepeated
}

type pb_package struct {
	Package string
	Message string
	Fields  []pb_field
}

// pb_field encodes the informations about a tree's branch or leaf.
type pb_field struct {
	Name     string
	Type     string
	Id       int
	Branch   string
	repeated bool
	//tag     string
}

func (f pb_field) Attr() string {
	attrs := []string{fmt.Sprintf(`(root_branch) = %q`, f.Branch)}
	if f.repeated && f.Type != "string" {
		attrs = append(attrs, "packed=true")
	}
	if len(attrs) > 0 {
		return fmt.Sprintf(" [%s]", strings.Join(attrs, ","))
	}
	return ""
}

func (f pb_field) Modifier() string {
	if f.repeated {
		return "repeated"
	}
	//return "required"
	return "optional"
}

var gen_id = make(chan int, 1)

func main() {
	go func() {
		cnt := 1
		for {
			gen_id <- cnt
			cnt += 1
		}
	}()

	flag.Parse()

	if *fname == "" || *tname == "" {
		flag.Usage()
		os.Exit(1)
	}

	*fname = os.ExpandEnv(*fname)
	*oname = path.Clean(os.ExpandEnv(*oname))
	outdir := path.Dir(*oname)

	fmt.Printf(":: root->proto ::\n")
	fmt.Printf(":: input file:  [%s]\n", *fname)
	fmt.Printf(":: output file: [%s]\n", *oname)
	fmt.Printf(":: outdir:      [%s]\n", outdir)
	fmt.Printf(":: tree:        [%s]\n", *tname)
	fmt.Printf(":: selection:   [%s]\n", *brsel)

	dname := path.Join(outdir, "descr.pbuf")

	abspath, err := filepath.Abs(*oname)
	if err != nil {
		fmt.Printf("**error** could not compute absolute path: %v\n", err)
		os.Exit(1)
	}
	*oname = abspath
	outdir = path.Dir(*oname)

	if !path_exists(outdir) {
		err := os.Mkdir(outdir, os.ModeDir|os.ModePerm)
		if err != nil {
			fmt.Printf("**error** could not create output dir: %v\n", err)
			os.Exit(1)
		}
	}

	pb_fields := inspect_root_file(*fname, *tname)

	fmt.Printf(":: generating .proto file...\n")
	t := template.New("Protobuf package template")
	t, err = t.Parse(pb_pkg_templ)
	if err != nil {
		fmt.Printf("**error** %v\n", err)
		os.Exit(1)
	}

	pb_file, err := os.Create(*oname)
	if err != nil {
		fmt.Printf("**error** %v\n", err)
		os.Exit(1)
	}

	pb_pkg := pb_package{
		Package: *pb_pkg_name,
		Message: *pb_msg_name,
		Fields:  pb_fields,
	}

	err = t.Execute(pb_file, pb_pkg)
	if err != nil {
		fmt.Printf("**error** %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(":: generating .proto file...[done]\n")

	if *do_gen != "" || *do_cnv {
		args := []string{}
		if strings.Contains(*do_gen, "go") || *do_cnv {
			args = append(args, "--go_out=.")
		}
		if strings.Contains(*do_gen, "py") {
			args = append(args, "--python_out=.")
		}
		if strings.Contains(*do_gen, "java") {
			args = append(args, "--java_out=.")
		}
		if strings.Contains(*do_gen, "cpp") ||
			strings.Contains(*do_gen, "cxx") {
			args = append(args, "--cpp_out=.")
		}

		args = append(args,
			fmt.Sprintf("--descriptor_set_out=%s", dname),
			"-I", outdir,
			"-I", "/usr/include",
			*oname)
		cmd := exec.Command("protoc", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = outdir
		err = cmd.Run()
		if err != nil {
			fmt.Printf("**error** running protoc: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf(":: pb file(s) generated.\n")

	}

	if *do_cnv {
		fmt.Printf(":: converting ROOT Tree's content into a pbuf...\n")
		err = convert_tree(*fname, *tname, dname)
		if err != nil {
			fmt.Printf("**error** converting ROOT Tree: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf(":: converting ROOT Tree's content into a pbuf... [done]\n")
	}

	fmt.Printf(":: bye.\n")
}
