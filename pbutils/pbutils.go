// Package "pbutils" provides some helpers to deal with protobuf types
package pbutils

import (
	protobuf "code.google.com/p/goprotobuf/protoc-gen-go/descriptor"
	"github.com/gonuts/ffi"
)

// FFIType returns the ffi.Type corresponding to a protobuf field descriptor.
func FFIType(fdp *protobuf.FieldDescriptorProto) ffi.Type {
	var ct ffi.Type
	pt := *fdp.Type
	switch pt {
	case protobuf.FieldDescriptorProto_TYPE_DOUBLE:
		ct = ffi.C_double
	case protobuf.FieldDescriptorProto_TYPE_FLOAT:
		ct = ffi.C_float
	case protobuf.FieldDescriptorProto_TYPE_INT64:
		ct = ffi.C_int64
	case protobuf.FieldDescriptorProto_TYPE_UINT64:
		ct = ffi.C_uint64
	case protobuf.FieldDescriptorProto_TYPE_INT32:
		ct = ffi.C_int32
	case protobuf.FieldDescriptorProto_TYPE_FIXED64:
		ct = ffi.C_uint64
	case protobuf.FieldDescriptorProto_TYPE_FIXED32:
		ct = ffi.C_uint32
	case protobuf.FieldDescriptorProto_TYPE_BOOL:
		ct = ffi.C_int
	case protobuf.FieldDescriptorProto_TYPE_STRING:
		panic("pbutils: protobuf type name [" + pt.String() + "] not implemented")
	case protobuf.FieldDescriptorProto_TYPE_GROUP:
		panic("pbutils: protobuf type name [" + pt.String() + "] not implemented")
	case protobuf.FieldDescriptorProto_TYPE_MESSAGE:
		panic("pbutils: protobuf type name [" + pt.String() + "] not implemented")
	case protobuf.FieldDescriptorProto_TYPE_BYTES:
		panic("pbutils: protobuf type name [" + pt.String() + "] not implemented")
	case protobuf.FieldDescriptorProto_TYPE_UINT32:
		ct = ffi.C_uint32
	case protobuf.FieldDescriptorProto_TYPE_ENUM:
		panic("pbutils: protobuf type name [" + pt.String() + "] not implemented")
	case protobuf.FieldDescriptorProto_TYPE_SFIXED32:
		ct = ffi.C_int32
	case protobuf.FieldDescriptorProto_TYPE_SFIXED64:
		ct = ffi.C_int64
	case protobuf.FieldDescriptorProto_TYPE_SINT32:
		ct = ffi.C_int32
	case protobuf.FieldDescriptorProto_TYPE_SINT64:
		ct = ffi.C_int64
	default:
		panic("pbutils: protobuf type name [" + pt.String() + "] not implemented")
	}
	return ct
}

// EOF
