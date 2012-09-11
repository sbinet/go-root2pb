package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/sbinet/go-croot/pkg/croot"
	//"code.google.com/p/goprotobuf/proto"
)

var fname = flag.String("f", "", "path to input ROOT file")
var oname = flag.String("o", "out/event.proto", "path to output .proto file")
var tname = flag.String("t", "", "name of the ROOT TTree to convert")
var pb_pkg_name = flag.String("pkg", "event", "name of the protobuf package to be generated")
var pb_msg_name = flag.String("msg", "Event", "name of the top-level message encoding the TTree")
var do_gen = flag.String("gen", "", "generate the pb file(s) from the .proto one for each of output languages (go,py,cpp,java)")

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

const pb_pkg_templ = `package {{.Package}};

message {{.Message}} {
{{with .Fields}}
  {{range .}} {{.Modifier}} {{.Type}} {{.Name}} = {{.Id}}{{.Attr}}; 
  {{end}}
{{end}}
}
`

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
	repeated bool
	//tag     string
}

func (f pb_field) Attr() string {
	attrs := []string{}
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
	return "optional"
}

var gen_id = make(chan int, 1)

func path_exists(name string) bool {
	_, err := os.Stat(name)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

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
	*oname = os.ExpandEnv(*oname)

	outdir := path.Dir(*oname)

	fmt.Printf(":: root->proto ::\n")
	fmt.Printf(":: input file:  [%s]\n", *fname)
	fmt.Printf(":: output file: [%s]\n", *oname)
	fmt.Printf(":: outdir:      [%s]\n", outdir)
	fmt.Printf(":: tree:        [%s]\n", *tname)

	if !path_exists(outdir) {
		err := os.Mkdir(outdir, os.ModeDir|os.ModePerm)
		if err != nil {
			fmt.Printf("**error** could not create output dir: %v\n", err)
			os.Exit(1)
		}
	}

	f := croot.OpenFile(*fname, "read", "ROOT file", 1, 0)
	if f == nil {
		fmt.Printf("**error** could not open ROOT file [%s]\n", *fname)
		os.Exit(1)
	}

	tree := f.GetTree(*tname)
	if tree == nil {
		fmt.Printf("**error** could not retrieve Tree [%s] from file [%s]\n",
			*tname, *fname)
	}

	//tree.Print("*")

	branches := tree.GetListOfBranches()
	fmt.Printf("   #-branches: %v\n", branches.GetSize())

	imax := branches.GetSize()
	// if imax > 20 {
	// 	imax = 20
	// }

	type stringset map[string]struct{}
	types := make(stringset)

	pb_fields := []pb_field{}

	for i := int64(0); i < imax; i++ {
		obj := branches.At(i)
		br := obj.(croot.Branch)
		typename := br.GetClassName()
		if typename == "" {
			leaf := tree.GetLeaf(br.GetName())
			typename = leaf.GetTypeName()
		}
		if *verbose {
			fmt.Printf(" [%d] -> [%v] (%v) (type:%v)\n", i, obj.GetName(), br.ClassName(), typename)
		}
		name := br.GetName()
		pb_type, isrepeated := get_pb_type(typename)
		pb_fields = append(pb_fields,
			pb_field{
				Name:     name,
				Type:     pb_type,
				Id:       <-gen_id,
				repeated: isrepeated,
			})
		types[typename] = struct{}{}
	}

	if *verbose {
		fmt.Printf(":: types in tree [%s]:\n", *tname)
		for k, _ := range types {
			fmt.Printf(" %s\n", k)
		}
	}

	fmt.Printf(":: generating .proto file...\n")
	t := template.New("Protobuf package template")
	t, err := t.Parse(pb_pkg_templ)
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

	if *do_gen != "" {
		args := []string{}
		outdir := "."
		if strings.Contains(*do_gen, "go") {
			args = append(args, fmt.Sprintf("--go_out=%s", outdir))
		}
		if strings.Contains(*do_gen, "py") {
			args = append(args, fmt.Sprintf("--python_out=%s", outdir))
		}
		if strings.Contains(*do_gen, "java") {
			args = append(args, fmt.Sprintf("--java_out=%s", outdir))
		}
		if strings.Contains(*do_gen, "cpp") ||
			strings.Contains(*do_gen, "cxx") {
			args = append(args, fmt.Sprintf("--cpp_out=%s", outdir))
		}
		args = append(args, *oname)
		cmd := exec.Command("protoc", args...)
		err = cmd.Run()
		if err != nil {
			fmt.Printf("**error** running protoc: %v\n", err)
			os.Exit(1)
		} else {
			fmt.Printf(":: pb file(s) generated.\n")
		}
	}
	fmt.Printf(":: bye.\n")
}
