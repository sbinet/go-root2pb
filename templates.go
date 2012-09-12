package main

const pb_pkg_templ = `package {{.Package}};

import "google/protobuf/descriptor.proto";

message {{.Message}} {
 extensions 50000 to max;
{{with .Fields}}
  {{range .}} {{.Modifier}} {{.Type}} {{.Name}} = {{.Id}}{{.Attr}}; 
  {{end}}
{{end}}
}

extend google.protobuf.FieldOptions {
  optional string root_branch = 50002;
}

message DataHeader {
  // Set of .proto files which define the type.
  //required google.protobuf.FileDescriptorSet proto_files = 1;

  // number of entries in the payload message
  required uint64 nevts = 2;
}
`

// EOF
