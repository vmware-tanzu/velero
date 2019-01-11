// Code generated by protoc-gen-go. DO NOT EDIT.
// source: Shared.proto

package generated

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type Empty struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Empty) Reset()         { *m = Empty{} }
func (m *Empty) String() string { return proto.CompactTextString(m) }
func (*Empty) ProtoMessage()    {}
func (*Empty) Descriptor() ([]byte, []int) {
	return fileDescriptor_17660dd6dfc7faa5, []int{0}
}

func (m *Empty) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Empty.Unmarshal(m, b)
}
func (m *Empty) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Empty.Marshal(b, m, deterministic)
}
func (m *Empty) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Empty.Merge(m, src)
}
func (m *Empty) XXX_Size() int {
	return xxx_messageInfo_Empty.Size(m)
}
func (m *Empty) XXX_DiscardUnknown() {
	xxx_messageInfo_Empty.DiscardUnknown(m)
}

var xxx_messageInfo_Empty proto.InternalMessageInfo

type InitRequest struct {
	Plugin               string            `protobuf:"bytes,1,opt,name=plugin,proto3" json:"plugin,omitempty"`
	Config               map[string]string `protobuf:"bytes,2,rep,name=config,proto3" json:"config,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *InitRequest) Reset()         { *m = InitRequest{} }
func (m *InitRequest) String() string { return proto.CompactTextString(m) }
func (*InitRequest) ProtoMessage()    {}
func (*InitRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_17660dd6dfc7faa5, []int{1}
}

func (m *InitRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_InitRequest.Unmarshal(m, b)
}
func (m *InitRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_InitRequest.Marshal(b, m, deterministic)
}
func (m *InitRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_InitRequest.Merge(m, src)
}
func (m *InitRequest) XXX_Size() int {
	return xxx_messageInfo_InitRequest.Size(m)
}
func (m *InitRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_InitRequest.DiscardUnknown(m)
}

var xxx_messageInfo_InitRequest proto.InternalMessageInfo

func (m *InitRequest) GetPlugin() string {
	if m != nil {
		return m.Plugin
	}
	return ""
}

func (m *InitRequest) GetConfig() map[string]string {
	if m != nil {
		return m.Config
	}
	return nil
}

type AppliesToRequest struct {
	Plugin               string   `protobuf:"bytes,1,opt,name=plugin,proto3" json:"plugin,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AppliesToRequest) Reset()         { *m = AppliesToRequest{} }
func (m *AppliesToRequest) String() string { return proto.CompactTextString(m) }
func (*AppliesToRequest) ProtoMessage()    {}
func (*AppliesToRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_17660dd6dfc7faa5, []int{2}
}

func (m *AppliesToRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AppliesToRequest.Unmarshal(m, b)
}
func (m *AppliesToRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AppliesToRequest.Marshal(b, m, deterministic)
}
func (m *AppliesToRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AppliesToRequest.Merge(m, src)
}
func (m *AppliesToRequest) XXX_Size() int {
	return xxx_messageInfo_AppliesToRequest.Size(m)
}
func (m *AppliesToRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_AppliesToRequest.DiscardUnknown(m)
}

var xxx_messageInfo_AppliesToRequest proto.InternalMessageInfo

func (m *AppliesToRequest) GetPlugin() string {
	if m != nil {
		return m.Plugin
	}
	return ""
}

type AppliesToResponse struct {
	IncludedNamespaces   []string `protobuf:"bytes,1,rep,name=includedNamespaces,proto3" json:"includedNamespaces,omitempty"`
	ExcludedNamespaces   []string `protobuf:"bytes,2,rep,name=excludedNamespaces,proto3" json:"excludedNamespaces,omitempty"`
	IncludedResources    []string `protobuf:"bytes,3,rep,name=includedResources,proto3" json:"includedResources,omitempty"`
	ExcludedResources    []string `protobuf:"bytes,4,rep,name=excludedResources,proto3" json:"excludedResources,omitempty"`
	Selector             string   `protobuf:"bytes,5,opt,name=selector,proto3" json:"selector,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AppliesToResponse) Reset()         { *m = AppliesToResponse{} }
func (m *AppliesToResponse) String() string { return proto.CompactTextString(m) }
func (*AppliesToResponse) ProtoMessage()    {}
func (*AppliesToResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_17660dd6dfc7faa5, []int{3}
}

func (m *AppliesToResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AppliesToResponse.Unmarshal(m, b)
}
func (m *AppliesToResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AppliesToResponse.Marshal(b, m, deterministic)
}
func (m *AppliesToResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AppliesToResponse.Merge(m, src)
}
func (m *AppliesToResponse) XXX_Size() int {
	return xxx_messageInfo_AppliesToResponse.Size(m)
}
func (m *AppliesToResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_AppliesToResponse.DiscardUnknown(m)
}

var xxx_messageInfo_AppliesToResponse proto.InternalMessageInfo

func (m *AppliesToResponse) GetIncludedNamespaces() []string {
	if m != nil {
		return m.IncludedNamespaces
	}
	return nil
}

func (m *AppliesToResponse) GetExcludedNamespaces() []string {
	if m != nil {
		return m.ExcludedNamespaces
	}
	return nil
}

func (m *AppliesToResponse) GetIncludedResources() []string {
	if m != nil {
		return m.IncludedResources
	}
	return nil
}

func (m *AppliesToResponse) GetExcludedResources() []string {
	if m != nil {
		return m.ExcludedResources
	}
	return nil
}

func (m *AppliesToResponse) GetSelector() string {
	if m != nil {
		return m.Selector
	}
	return ""
}

func init() {
	proto.RegisterType((*Empty)(nil), "generated.Empty")
	proto.RegisterType((*InitRequest)(nil), "generated.InitRequest")
	proto.RegisterMapType((map[string]string)(nil), "generated.InitRequest.ConfigEntry")
	proto.RegisterType((*AppliesToRequest)(nil), "generated.AppliesToRequest")
	proto.RegisterType((*AppliesToResponse)(nil), "generated.AppliesToResponse")
}

func init() { proto.RegisterFile("Shared.proto", fileDescriptor_17660dd6dfc7faa5) }

var fileDescriptor_17660dd6dfc7faa5 = []byte{
	// 278 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x91, 0xcd, 0x4e, 0x84, 0x30,
	0x14, 0x85, 0x53, 0x10, 0x94, 0x8b, 0x8b, 0x99, 0xc6, 0x18, 0x32, 0x2b, 0xc2, 0x8a, 0x18, 0xc3,
	0x42, 0x37, 0x3a, 0x3b, 0x63, 0x66, 0xe1, 0xc6, 0x05, 0xfa, 0x02, 0x08, 0x57, 0x24, 0x32, 0x6d,
	0xed, 0x8f, 0x91, 0x77, 0xf1, 0x0d, 0x7d, 0x09, 0x43, 0x61, 0x46, 0x12, 0x4c, 0x66, 0xd7, 0x73,
	0xcf, 0x77, 0xbf, 0xb4, 0x29, 0x9c, 0x3e, 0xbd, 0x15, 0x12, 0xab, 0x4c, 0x48, 0xae, 0x39, 0x0d,
	0x6a, 0x64, 0x28, 0x0b, 0x8d, 0x55, 0x72, 0x0c, 0xde, 0x66, 0x2b, 0x74, 0x97, 0x7c, 0x13, 0x08,
	0x1f, 0x58, 0xa3, 0x73, 0xfc, 0x30, 0xa8, 0x34, 0x3d, 0x07, 0x5f, 0xb4, 0xa6, 0x6e, 0x58, 0x44,
	0x62, 0x92, 0x06, 0xf9, 0x98, 0xe8, 0x1a, 0xfc, 0x92, 0xb3, 0xd7, 0xa6, 0x8e, 0x9c, 0xd8, 0x4d,
	0xc3, 0xab, 0x24, 0xdb, 0xcb, 0xb2, 0xc9, 0x7e, 0x76, 0x6f, 0xa1, 0x0d, 0xd3, 0xb2, 0xcb, 0xc7,
	0x8d, 0xd5, 0x2d, 0x84, 0x93, 0x31, 0x5d, 0x80, 0xfb, 0x8e, 0xdd, 0xe8, 0xef, 0x8f, 0xf4, 0x0c,
	0xbc, 0xcf, 0xa2, 0x35, 0x18, 0x39, 0x76, 0x36, 0x84, 0xb5, 0x73, 0x43, 0x92, 0x0b, 0x58, 0xdc,
	0x09, 0xd1, 0x36, 0xa8, 0x9e, 0xf9, 0x81, 0x2b, 0x26, 0x3f, 0x04, 0x96, 0x13, 0x58, 0x09, 0xce,
	0x14, 0xd2, 0x0c, 0x68, 0xc3, 0xca, 0xd6, 0x54, 0x58, 0x3d, 0x16, 0x5b, 0x54, 0xa2, 0x28, 0x51,
	0x45, 0x24, 0x76, 0xd3, 0x20, 0xff, 0xa7, 0xe9, 0x79, 0xfc, 0x9a, 0xf1, 0xce, 0xc0, 0xcf, 0x1b,
	0x7a, 0x09, 0xcb, 0x9d, 0x25, 0x47, 0xc5, 0x8d, 0xec, 0x71, 0xd7, 0xe2, 0xf3, 0xa2, 0xa7, 0x77,
	0x8e, 0x3f, 0xfa, 0x68, 0xa0, 0x67, 0x05, 0x5d, 0xc1, 0x89, 0xc2, 0x16, 0x4b, 0xcd, 0x65, 0xe4,
	0xd9, 0xb7, 0xee, 0xf3, 0x8b, 0x6f, 0xff, 0xf4, 0xfa, 0x37, 0x00, 0x00, 0xff, 0xff, 0x73, 0x1b,
	0xfe, 0x37, 0xe3, 0x01, 0x00, 0x00,
}
