// Copyright 2021-2025 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: cerbos/schema/v1/schema.proto

package schemav1

import (
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ValidationError_Source int32

const (
	ValidationError_SOURCE_UNSPECIFIED ValidationError_Source = 0
	ValidationError_SOURCE_PRINCIPAL   ValidationError_Source = 1
	ValidationError_SOURCE_RESOURCE    ValidationError_Source = 2
)

// Enum value maps for ValidationError_Source.
var (
	ValidationError_Source_name = map[int32]string{
		0: "SOURCE_UNSPECIFIED",
		1: "SOURCE_PRINCIPAL",
		2: "SOURCE_RESOURCE",
	}
	ValidationError_Source_value = map[string]int32{
		"SOURCE_UNSPECIFIED": 0,
		"SOURCE_PRINCIPAL":   1,
		"SOURCE_RESOURCE":    2,
	}
)

func (x ValidationError_Source) Enum() *ValidationError_Source {
	p := new(ValidationError_Source)
	*p = x
	return p
}

func (x ValidationError_Source) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ValidationError_Source) Descriptor() protoreflect.EnumDescriptor {
	return file_cerbos_schema_v1_schema_proto_enumTypes[0].Descriptor()
}

func (ValidationError_Source) Type() protoreflect.EnumType {
	return &file_cerbos_schema_v1_schema_proto_enumTypes[0]
}

func (x ValidationError_Source) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ValidationError_Source.Descriptor instead.
func (ValidationError_Source) EnumDescriptor() ([]byte, []int) {
	return file_cerbos_schema_v1_schema_proto_rawDescGZIP(), []int{0, 0}
}

type ValidationError struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Path          string                 `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	Message       string                 `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	Source        ValidationError_Source `protobuf:"varint,3,opt,name=source,proto3,enum=cerbos.schema.v1.ValidationError_Source" json:"source,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ValidationError) Reset() {
	*x = ValidationError{}
	mi := &file_cerbos_schema_v1_schema_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ValidationError) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ValidationError) ProtoMessage() {}

func (x *ValidationError) ProtoReflect() protoreflect.Message {
	mi := &file_cerbos_schema_v1_schema_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ValidationError.ProtoReflect.Descriptor instead.
func (*ValidationError) Descriptor() ([]byte, []int) {
	return file_cerbos_schema_v1_schema_proto_rawDescGZIP(), []int{0}
}

func (x *ValidationError) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *ValidationError) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *ValidationError) GetSource() ValidationError_Source {
	if x != nil {
		return x.Source
	}
	return ValidationError_SOURCE_UNSPECIFIED
}

type Schema struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Definition    []byte                 `protobuf:"bytes,2,opt,name=definition,proto3" json:"definition,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Schema) Reset() {
	*x = Schema{}
	mi := &file_cerbos_schema_v1_schema_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Schema) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Schema) ProtoMessage() {}

func (x *Schema) ProtoReflect() protoreflect.Message {
	mi := &file_cerbos_schema_v1_schema_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Schema.ProtoReflect.Descriptor instead.
func (*Schema) Descriptor() ([]byte, []int) {
	return file_cerbos_schema_v1_schema_proto_rawDescGZIP(), []int{1}
}

func (x *Schema) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Schema) GetDefinition() []byte {
	if x != nil {
		return x.Definition
	}
	return nil
}

var File_cerbos_schema_v1_schema_proto protoreflect.FileDescriptor

const file_cerbos_schema_v1_schema_proto_rawDesc = "" +
	"\n" +
	"\x1dcerbos/schema/v1/schema.proto\x12\x10cerbos.schema.v1\x1a\x1bbuf/validate/validate.proto\x1a\x1fgoogle/api/field_behavior.proto\x1a.protoc-gen-openapiv2/options/annotations.proto\"\xce\x01\n" +
	"\x0fValidationError\x12\x12\n" +
	"\x04path\x18\x01 \x01(\tR\x04path\x12\x18\n" +
	"\amessage\x18\x02 \x01(\tR\amessage\x12@\n" +
	"\x06source\x18\x03 \x01(\x0e2(.cerbos.schema.v1.ValidationError.SourceR\x06source\"K\n" +
	"\x06Source\x12\x16\n" +
	"\x12SOURCE_UNSPECIFIED\x10\x00\x12\x14\n" +
	"\x10SOURCE_PRINCIPAL\x10\x01\x12\x13\n" +
	"\x0fSOURCE_RESOURCE\x10\x02\"\xcf\x01\n" +
	"\x06Schema\x12W\n" +
	"\x02id\x18\x01 \x01(\tBG\x92A42 Unique identifier for the schemaJ\x10\"principal.json\"\xe0A\x02\xbaH\n" +
	"\xc8\x01\x01r\x05\x10\x01\x18\xff\x01R\x02id\x12l\n" +
	"\n" +
	"definition\x18\x02 \x01(\fBL\x92A<2\x16JSON schema definitionJ\"{\"type\":\"object\", \"properties\":{}}\xe0A\x02\xbaH\a\xc8\x01\x01z\x02\x10\n" +
	"R\n" +
	"definitionBo\n" +
	"\x18dev.cerbos.api.v1.schemaZ<github.com/cerbos/cerbos/api/genpb/cerbos/schema/v1;schemav1\xaa\x02\x14Cerbos.Api.V1.Schemab\x06proto3"

var (
	file_cerbos_schema_v1_schema_proto_rawDescOnce sync.Once
	file_cerbos_schema_v1_schema_proto_rawDescData []byte
)

func file_cerbos_schema_v1_schema_proto_rawDescGZIP() []byte {
	file_cerbos_schema_v1_schema_proto_rawDescOnce.Do(func() {
		file_cerbos_schema_v1_schema_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_cerbos_schema_v1_schema_proto_rawDesc), len(file_cerbos_schema_v1_schema_proto_rawDesc)))
	})
	return file_cerbos_schema_v1_schema_proto_rawDescData
}

var file_cerbos_schema_v1_schema_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_cerbos_schema_v1_schema_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_cerbos_schema_v1_schema_proto_goTypes = []any{
	(ValidationError_Source)(0), // 0: cerbos.schema.v1.ValidationError.Source
	(*ValidationError)(nil),     // 1: cerbos.schema.v1.ValidationError
	(*Schema)(nil),              // 2: cerbos.schema.v1.Schema
}
var file_cerbos_schema_v1_schema_proto_depIdxs = []int32{
	0, // 0: cerbos.schema.v1.ValidationError.source:type_name -> cerbos.schema.v1.ValidationError.Source
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_cerbos_schema_v1_schema_proto_init() }
func file_cerbos_schema_v1_schema_proto_init() {
	if File_cerbos_schema_v1_schema_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_cerbos_schema_v1_schema_proto_rawDesc), len(file_cerbos_schema_v1_schema_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_cerbos_schema_v1_schema_proto_goTypes,
		DependencyIndexes: file_cerbos_schema_v1_schema_proto_depIdxs,
		EnumInfos:         file_cerbos_schema_v1_schema_proto_enumTypes,
		MessageInfos:      file_cerbos_schema_v1_schema_proto_msgTypes,
	}.Build()
	File_cerbos_schema_v1_schema_proto = out.File
	file_cerbos_schema_v1_schema_proto_goTypes = nil
	file_cerbos_schema_v1_schema_proto_depIdxs = nil
}
