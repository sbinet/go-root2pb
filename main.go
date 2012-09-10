package main

import (
	"flag"
	"fmt"
	"os"
	
	"github.com/sbinet/go-croot/pkg/croot"
	//"code.google.com/p/goprotobuf/proto"
)

var fname = flag.String("f", "", "path to input ROOT file")
var oname = flag.String("o", "event.proto", "path to output .proto file")
var tname = flag.String("t", "", "name of the ROOT TTree to convert")

func main() {
	flag.Parse()

	if *fname == "" || *tname == "" {
		flag.Usage()
		os.Exit(1)
	}

	*fname = os.ExpandEnv(*fname)
	*oname = os.ExpandEnv(*oname)

	fmt.Printf(":: root->proto ::\n")
	fmt.Printf(":: input file:  [%s]\n", *fname)
	fmt.Printf(":: output file: [%s]\n", *oname)
	fmt.Printf(":: tree:        [%s]\n", *tname)

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

	
	fmt.Printf(":: bye.\n")
}
