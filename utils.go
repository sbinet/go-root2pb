package main

import (
	"fmt"
	go_build "go/build"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"code.google.com/p/goprotobuf/proto"
	pb_descr "code.google.com/p/goprotobuf/protoc-gen-go/descriptor"
	pb_gen "code.google.com/p/goprotobuf/protoc-gen-go/generator"
	"github.com/go-hep/croot"
)

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

func get_templates_dir() string {
	const dirname = "github.com/sbinet/go-root2pb/templates"
	for _, srcdir := range go_build.Default.SrcDirs() {
		dir := path.Join(srcdir, dirname)
		if path_exists(dir) {
			return dir
		}
	}
	return ""
}

func inspect_root_file(filename, treename string) []pb_field {

	f := croot.OpenFile(filename, "read", "ROOT file", 1, 0)
	if f == nil {
		fmt.Printf("**error** could not open ROOT file [%s]\n", filename)
		os.Exit(1)
	}
	defer f.Close("")

	tree := f.GetTree(treename)
	if tree == nil {
		fmt.Printf("**error** could not retrieve Tree [%s] from file [%s]\n",
			treename, filename)
	}

	//tree.Print("*")

	branches := tree.GetListOfBranches()
	fmt.Printf("   #-branches: %v\n", branches.GetSize())

	imax := branches.GetSize()

	type stringset map[string]struct{}

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
		accept := true
		if *brsel != "" {
			accept = false
			for _, pattern := range strings.Split(*brsel, ",") {
				switch pattern[0] {
				case '-':
					matched, err := filepath.Match(pattern[1:], name)
					if err == nil && matched {
						accept = false
						break
					}
				case '+':
					matched, err := filepath.Match(pattern[1:], name)
					if err == nil && matched {
						accept = true
						break
					}
				default:
					matched, err := filepath.Match(pattern, name)
					if err == nil && matched {
						accept = true
						break
					}
				}
			}
		}
		if accept {
			pb_fields = append(pb_fields,
				pb_field{
					Name:     pb_gen.CamelCase(name),
					Type:     pb_type,
					Id:       <-gen_id,
					Branch:   name,
					repeated: isrepeated,
				})
		}
	}

	return pb_fields
}

func convert_tree(filename, treename, descr_fname string) error {
	var err error

	// first create a workdir
	wkdir, err := ioutil.TempDir("", "go-root2pb-")
	if err != nil {
		return err
	}
	//fmt.Printf("wkdir: %v\n", wkdir)
	err = os.MkdirAll(wkdir, os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}
	//defer os.RemoveAll(wkdir)

	for _, dirname := range []string{
		path.Join("pkg", go_build.Default.GOOS+"_"+go_build.Default.GOARCH),
		"bin",
	} {
		dir := path.Join(wkdir, dirname)
		err = os.MkdirAll(dir, os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}
	}

	srcdir := path.Join(wkdir, "src")
	err = os.MkdirAll(srcdir, os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}

	err = os.Chdir(srcdir)
	if err != nil {
		return err
	}
	//fmt.Printf("srcdir: %v\n", srcdir)
	orig_gopath := go_build.Default.GOPATH
	defer os.Setenv("GOPATH", orig_gopath)

	go_build.Default.GOPATH = strings.Join(
		[]string{wkdir, go_build.Default.GOPATH},
		string(filepath.ListSeparator))
	//fmt.Printf("GOPATH: %v\n", go_build.Default.GOPATH)
	err = os.Setenv("GOPATH", go_build.Default.GOPATH)

	data, err := ioutil.ReadFile(descr_fname)
	if err != nil {
		fmt.Printf("**error** reading descriptor file: %v\n", err)
		return err
	}
	fdset := pb_descr.FileDescriptorSet{}
	err = proto.Unmarshal(data, &fdset)
	if err != nil {
		return err
	}

	pb_pkg_name := ""
	pb_msg_name := ""

	//fmt.Printf(":: fdset: %v\n", len(fdset.File))
	for _, fd := range fdset.File {
		// fmt.Printf(" name=%q\n", fd.GetName())
		// fmt.Printf(" pkg=%q\n", fd.GetPackage())
		// fmt.Printf(" deps=%v\n", fd.Dependency)
		// fmt.Printf(" public-deps=%v\n", fd.PublicDependency)
		// fmt.Printf(" #-msgs=%d\n", len(fd.MessageType))
		for _, msg := range fd.MessageType {
			// fmt.Printf("  msg[%d]: %v\n", imsg, msg.GetName())
			if msg.GetName() != "DataHeader" {
				pb_msg_name = msg.GetName()
			}
			// for _, field := range msg.Field {
			// 	typename := ""
			// 	if field.TypeName != nil {
			// 		typename = *field.TypeName
			// 	}
			// 	fmt.Printf("    field: %q type=%v name=%q\n", *field.Name, *field.Type, typename)
			// }
		}
		// create protobuf data package
		pb_pkg_name = path.Join("root2pb-data", *fd.Package)
		pkgdir := path.Join(srcdir, pb_pkg_name)
		// fmt.Printf("-->pkgdir: %v\n", pkgdir)
		err = os.MkdirAll(pkgdir, os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}
		files, err := filepath.Glob(path.Join(filepath.Dir(descr_fname), "*"))
		if err != nil {
			return err
		}
		for _, fname := range files {
			dest, err := os.Create(path.Join(pkgdir, filepath.Base(fname)))
			if err != nil {
				return err
			}
			src, err := os.Open(fname)
			if err != nil {
				return err
			}
			// fixup...
			if strings.HasSuffix(fname, ".pb.go") {
				data, err := ioutil.ReadFile(fname)
				sdata := strings.Replace(string(data),
					"google/protobuf/descriptor.pb",
					"code.google.com/p/goprotobuf/protoc-gen-go/descriptor",
					-1)
				_, err = dest.WriteString(sdata)
				if err != nil {
					return err
				}

			} else {
				_, err = io.Copy(dest, src)
				if err != nil {
					return err
				}
			}
			err = dest.Sync()
			if err != nil {
				return err
			}
		}
		cmd := exec.Command("go", "get", ".")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = pkgdir
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	tmpldir := get_templates_dir()

	//fmt.Printf("tmpldir: %v\n", tmpldir)
	t, err := template.ParseFiles(path.Join(tmpldir, "cnv.go"))
	if err != nil {
		fmt.Printf("**error** parsing template file: %v\n", err)
		return err
	}

	err = os.MkdirAll(path.Join(srcdir, "root2pb-cnv"), os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}
	cnv_fname := path.Join(srcdir, "root2pb-cnv", "cnv.go")
	//fmt.Printf("cnv: %v\n", cnv_fname)
	cnv, err := os.Create(cnv_fname)
	if err != nil {
		return err
	}

	tmpl_data := map[string]string{
		"Package":    fmt.Sprintf(`%q`, pb_pkg_name),
		"DataHeader": "DataHeader",
		"Event":      pb_msg_name,
		"FdSet":      descr_fname,
	}
	err = t.Execute(cnv, tmpl_data)
	//err = t.Execute(os.Stdout, tmpl_data)
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "get", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(cnv_fname)
	err = cmd.Run()
	if err != nil {
		return err
	}

	oname := path.Base(filename)
	oname = filepath.Join(
		path.Dir(descr_fname),
		strings.Replace(oname, ".root", ".pbuf", -1),
	)

	args := []string{
		"-fname", filename,
		"-tname", treename,
		"-evtmax", "-1",
		"-oname", oname,
	}
	cmd = exec.Command(filepath.Join(wkdir, "bin", "root2pb-cnv"), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = wkdir
	err = cmd.Run()
	if err != nil {
		return err
	}

	return err
}

// EOF
