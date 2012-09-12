package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"

	msgpkg {{.Package}}
	"github.com/sbinet/go-croot/pkg/croot"
	"github.com/sbinet/go-ffi/pkg/ffi"
	"github.com/sbinet/go-root2pb/pbutils"
	"code.google.com/p/goprotobuf/proto"
	pb_descr "code.google.com/p/goprotobuf/protoc-gen-go/descriptor"
)

var fname = flag.String("fname", "", "ROOT file to convert")
var tname = flag.String("tname", "", "ROOT tree to convert")
var evtmax = flag.Int64("evtmax", -1, "number of entries to convert")
var oname = flag.String("oname", "", "name of the output pbuf file")

func get_root_branch_name(opts string) string {
	n := opts[strings.Index(opts, `root_branch]:"`)+len(`root_branch]:"`):]
	n = n[:strings.Index(n, `"`)]
	return n
}

func main() {
	flag.Parse()

	if *fname == "" || *tname == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *oname == "" {
		*oname = path.Base(*fname)
		*oname = strings.Replace(*oname, ".root", ".pbuf", -1)
	}

	fmt.Printf(":: cnv ROOT file into pbuf...\n")
	fmt.Printf("::  ROOT file: [%s]\n", *fname)
	fmt.Printf("::  ROOT tree: [%s]\n", *tname)
	fmt.Printf("::  PBuf file: [%s]\n", *oname)
	fmt.Printf("::  evtmax:    [%v]\n", *evtmax)

	f := croot.OpenFile(*fname, "read", "ROOT file", 1, 0)
	if f == nil {
		fmt.Printf("**error** could not open ROOT file [%s]\n", *fname)
		os.Exit(1)
	}
	defer f.Close("")

	tree := f.GetTree(*tname)
	if tree == nil {
		fmt.Printf("**error** could not retrieve Tree [%s] from file [%s]\n",
			*tname, *fname)
	}

	if *evtmax < 0 {
		*evtmax = int64(tree.GetEntries())
	}
	
	out, err := os.Create(*oname)
	if err != nil {
		fmt.Printf("**error** could not create output file [%s]\n%v\n", 
			*oname, err)
		os.Exit(1)
	}
	defer func(){
		err := out.Sync()
		if err != nil {
			fmt.Printf("**error** problem commiting data to disk: %v\n", err)
		}
		err = out.Close()
		if err != nil {
			fmt.Printf("**error** problem closing file: %v\n", err)
		}
	}()

	// proto-buf data-hdr
	{
		hdr := msgpkg.{{.DataHeader}}{}
		hdr.Nevts = proto.Uint64(uint64(*evtmax))
		data, err := proto.Marshal(&hdr)
		if err != nil {
			fmt.Printf("**error** event-hdr: problem marshalling pbuf: %v\n", 
				err)
			os.Exit(1)
		}
		_, err = out.Write(data)
		if err != nil {
			fmt.Printf("**error** event-hdr: problem writing header: %v\n", 
				err)
			os.Exit(1)
		}
	}

	// proto-buf data
	evt := msgpkg.{{.Event}}{}
	type PbData struct {
		Fields []string
		Values []ffi.Value
		GoValues []reflect.Value
	}
	rdata := PbData{
		Fields: make([]string, 0),
		Values: make([]ffi.Value, 0),
		GoValues: make([]reflect.Value, 0),
	}

	{
		data, err := ioutil.ReadFile("{{.FdSet}}")
		if err != nil {
			fmt.Printf("**error** reading descriptor file: %v\n", err)
			os.Exit(1)
		}
		fdset := pb_descr.FileDescriptorSet{}
		err = proto.Unmarshal(data, &fdset)
		if err != nil {
			os.Exit(1)
		}

		fmt.Printf(":: fdset: %v\n", len(fdset.File))
		for _, fd := range fdset.File {
			fmt.Printf(" name=%q\n", fd.GetName())
			fmt.Printf(" pkg=%q\n", fd.GetPackage())
			fmt.Printf(" deps=%v\n", fd.Dependency)
			fmt.Printf(" public-deps=%v\n", fd.PublicDependency)
			fmt.Printf(" #-msgs=%d\n", len(fd.MessageType))
			for imsg, msg := range fd.MessageType {
				fmt.Printf("  msg[%d]: %v\n", imsg, *msg.Name)
				if *msg.Name != "{{.Event}}" {
					continue
				}
				for _, field := range msg.Field {
					name := field.GetName()
					opts := fmt.Sprintf("%v",field.GetOptions())
					is_repeated := field.GetLabel() == pb_descr.FieldDescriptorProto_LABEL_REPEATED
					// FIXME: that's a vile hack...
					// how do we retieve values out of extensions ??
					root_branch := get_root_branch_name(opts)
					//fmt.Printf("    field: %q type=%v branch=%q opts=%v\n", name, field.GetType(), root_branch, opts)
					ct := pbutils.FFIType(field)
					var cval ffi.Value
					var rval reflect.Value
					if is_repeated {
						ct, err = ffi.NewSliceType(ct)
						if err != nil {
							fmt.Printf("**error** creating slice-type: %v\n", err)
							os.Exit(1)
						}
						err = ffi.Associate(ct, reflect.ValueOf(&evt).Elem().FieldByName(name).Type())
						if err != nil {
							fmt.Printf("**error** associating slice-type: %v\n", err)
							os.Exit(1)
						}
						cval = ffi.MakeSlice(ct, 0, 10)
						rval =reflect.ValueOf(&evt).Elem().FieldByName(name)
					} else {
						cval = ffi.New(ct)
						rval =reflect.ValueOf(&evt).Elem().FieldByName(name).Addr()
					}
					rdata.Fields = append(rdata.Fields, root_branch)
					rdata.Values = append(rdata.Values, cval)
					rdata.GoValues = append(rdata.GoValues, rval)
				}
			}
		}
	}

	for i,_ := range rdata.Fields {
		k := rdata.Fields[i]
		rc := tree.SetBranchAddress(k, rdata.Values[i])
		if rc < 0 {
			fmt.Printf("**error** problem setting branch address for [%s]: %v\n", k, rc)
			os.Exit(1)
		}
	}

	for ievt := int64(0); ievt < *evtmax; ievt++ {
		rc := tree.GetEntry(ievt, 1)
		if rc <= 0 {
			fmt.Printf("**error** problem loading entry [%v]: %v\n", ievt, rc)
			os.Exit(1)
		}

		for i,_ := range rdata.Fields {
			v := rdata.Values[i].GoValue()
			switch v.Kind() {
			case reflect.Slice:
				//fmt.Printf("--> %v\n", v.Interface())
				rdata.GoValues[i].Set(v)
			default:
				rdata.GoValues[i].Elem().Set(v.Addr())
			}
		}

		data, err := proto.Marshal(&evt)
		if err != nil {
			fmt.Printf("**error** entry-#%v: problem marshalling pbuf: %v\n", 
				ievt, err)
			os.Exit(1)
		}
		// fmt.Printf("run-nbr=%v evt-nbr=%v el-nbr=%v el-eta=%v\n", 
		// 	evt.GetRunNumber(), evt.GetEventNumber(), evt.GetElN(), evt.ElEta)
		_, err = out.Write(data)
		if err != nil {
			fmt.Printf("**error** writing pbuf data to file: %v\n", err)
			os.Exit(1)
		}
	}
}

// EOF
