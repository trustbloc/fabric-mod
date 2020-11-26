// Code generated by protoc-gen-go. DO NOT EDIT.
// source: cache_value.proto

package statecouchdb

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
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

type CacheValue struct {
	Version              []byte   `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
	Value                []byte   `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
	Metadata             []byte   `protobuf:"bytes,3,opt,name=metadata,proto3" json:"metadata,omitempty"`
	AdditionalInfo       []byte   `protobuf:"bytes,4,opt,name=additional_info,json=additionalInfo,proto3" json:"additional_info,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CacheValue) Reset()         { *m = CacheValue{} }
func (m *CacheValue) String() string { return proto.CompactTextString(m) }
func (*CacheValue) ProtoMessage()    {}
func (*CacheValue) Descriptor() ([]byte, []int) {
	return fileDescriptor_c9816941fba5d88a, []int{0}
}

func (m *CacheValue) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CacheValue.Unmarshal(m, b)
}
func (m *CacheValue) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CacheValue.Marshal(b, m, deterministic)
}
func (m *CacheValue) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CacheValue.Merge(m, src)
}
func (m *CacheValue) XXX_Size() int {
	return xxx_messageInfo_CacheValue.Size(m)
}
func (m *CacheValue) XXX_DiscardUnknown() {
	xxx_messageInfo_CacheValue.DiscardUnknown(m)
}

var xxx_messageInfo_CacheValue proto.InternalMessageInfo

func (m *CacheValue) GetVersion() []byte {
	if m != nil {
		return m.Version
	}
	return nil
}

func (m *CacheValue) GetValue() []byte {
	if m != nil {
		return m.Value
	}
	return nil
}

func (m *CacheValue) GetMetadata() []byte {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *CacheValue) GetAdditionalInfo() []byte {
	if m != nil {
		return m.AdditionalInfo
	}
	return nil
}

type CacheUpdatesEnvelope struct {
	Updates              map[string]*CacheKeyValues `protobuf:"bytes,1,rep,name=updates,proto3" json:"updates,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}                   `json:"-"`
	XXX_unrecognized     []byte                     `json:"-"`
	XXX_sizecache        int32                      `json:"-"`
}

func (m *CacheUpdatesEnvelope) Reset()         { *m = CacheUpdatesEnvelope{} }
func (m *CacheUpdatesEnvelope) String() string { return proto.CompactTextString(m) }
func (*CacheUpdatesEnvelope) ProtoMessage()    {}
func (*CacheUpdatesEnvelope) Descriptor() ([]byte, []int) {
	return fileDescriptor_c9816941fba5d88a, []int{1}
}

func (m *CacheUpdatesEnvelope) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CacheUpdatesEnvelope.Unmarshal(m, b)
}
func (m *CacheUpdatesEnvelope) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CacheUpdatesEnvelope.Marshal(b, m, deterministic)
}
func (m *CacheUpdatesEnvelope) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CacheUpdatesEnvelope.Merge(m, src)
}
func (m *CacheUpdatesEnvelope) XXX_Size() int {
	return xxx_messageInfo_CacheUpdatesEnvelope.Size(m)
}
func (m *CacheUpdatesEnvelope) XXX_DiscardUnknown() {
	xxx_messageInfo_CacheUpdatesEnvelope.DiscardUnknown(m)
}

var xxx_messageInfo_CacheUpdatesEnvelope proto.InternalMessageInfo

func (m *CacheUpdatesEnvelope) GetUpdates() map[string]*CacheKeyValues {
	if m != nil {
		return m.Updates
	}
	return nil
}

type CacheKeyValues struct {
	Values               map[string]*CacheValue `protobuf:"bytes,1,rep,name=values,proto3" json:"values,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}               `json:"-"`
	XXX_unrecognized     []byte                 `json:"-"`
	XXX_sizecache        int32                  `json:"-"`
}

func (m *CacheKeyValues) Reset()         { *m = CacheKeyValues{} }
func (m *CacheKeyValues) String() string { return proto.CompactTextString(m) }
func (*CacheKeyValues) ProtoMessage()    {}
func (*CacheKeyValues) Descriptor() ([]byte, []int) {
	return fileDescriptor_c9816941fba5d88a, []int{2}
}

func (m *CacheKeyValues) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CacheKeyValues.Unmarshal(m, b)
}
func (m *CacheKeyValues) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CacheKeyValues.Marshal(b, m, deterministic)
}
func (m *CacheKeyValues) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CacheKeyValues.Merge(m, src)
}
func (m *CacheKeyValues) XXX_Size() int {
	return xxx_messageInfo_CacheKeyValues.Size(m)
}
func (m *CacheKeyValues) XXX_DiscardUnknown() {
	xxx_messageInfo_CacheKeyValues.DiscardUnknown(m)
}

var xxx_messageInfo_CacheKeyValues proto.InternalMessageInfo

func (m *CacheKeyValues) GetValues() map[string]*CacheValue {
	if m != nil {
		return m.Values
	}
	return nil
}

func init() {
	proto.RegisterType((*CacheValue)(nil), "statecouchdb.CacheValue")
	proto.RegisterType((*CacheUpdatesEnvelope)(nil), "statecouchdb.CacheUpdatesEnvelope")
	proto.RegisterMapType((map[string]*CacheKeyValues)(nil), "statecouchdb.CacheUpdatesEnvelope.UpdatesEntry")
	proto.RegisterType((*CacheKeyValues)(nil), "statecouchdb.CacheKeyValues")
	proto.RegisterMapType((map[string]*CacheValue)(nil), "statecouchdb.CacheKeyValues.ValuesEntry")
}

func init() { proto.RegisterFile("cache_value.proto", fileDescriptor_c9816941fba5d88a) }

var fileDescriptor_c9816941fba5d88a = []byte{
	// 339 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x52, 0x41, 0x4b, 0xf3, 0x40,
	0x10, 0x25, 0xed, 0xf7, 0xb5, 0x3a, 0x2d, 0x55, 0x97, 0x1e, 0x42, 0xf1, 0x50, 0x7a, 0xb1, 0xa7,
	0x5d, 0xa8, 0x17, 0xf1, 0x24, 0x8a, 0x87, 0x22, 0x88, 0x54, 0x14, 0xf1, 0x52, 0x36, 0xbb, 0xd3,
	0x26, 0x34, 0xc9, 0x86, 0xcd, 0x26, 0x98, 0xa3, 0xbf, 0xc6, 0x1f, 0xe1, 0x9f, 0x93, 0x6e, 0xd2,
	0x9a, 0x42, 0xf0, 0x94, 0x79, 0xef, 0xcd, 0xbc, 0x79, 0x13, 0x16, 0xce, 0x04, 0x17, 0x3e, 0x2e,
	0x73, 0x1e, 0x66, 0x48, 0x13, 0xad, 0x8c, 0x22, 0xfd, 0xd4, 0x70, 0x83, 0x42, 0x65, 0xc2, 0x97,
	0xde, 0xe4, 0xd3, 0x01, 0xb8, 0xdb, 0xf6, 0xbc, 0x6e, 0x5b, 0x88, 0x0b, 0xdd, 0x1c, 0x75, 0x1a,
	0xa8, 0xd8, 0x75, 0xc6, 0xce, 0xb4, 0xbf, 0xd8, 0x41, 0x32, 0x84, 0xff, 0xd6, 0xc5, 0x6d, 0x59,
	0xbe, 0x04, 0x64, 0x04, 0x47, 0x11, 0x1a, 0x2e, 0xb9, 0xe1, 0x6e, 0xdb, 0x0a, 0x7b, 0x4c, 0x2e,
	0xe0, 0x84, 0x4b, 0x19, 0x98, 0x40, 0xc5, 0x3c, 0x5c, 0x06, 0xf1, 0x4a, 0xb9, 0xff, 0x6c, 0xcb,
	0xe0, 0x97, 0x9e, 0xc7, 0x2b, 0x35, 0xf9, 0x76, 0x60, 0x68, 0x33, 0xbc, 0x24, 0x92, 0x1b, 0x4c,
	0xef, 0xe3, 0x1c, 0x43, 0x95, 0x20, 0x99, 0x43, 0x37, 0x2b, 0x29, 0xd7, 0x19, 0xb7, 0xa7, 0xbd,
	0x19, 0xa3, 0xf5, 0xf0, 0xb4, 0x69, 0x88, 0xee, 0xb1, 0xd1, 0xc5, 0x62, 0x37, 0x3f, 0x7a, 0x83,
	0x7e, 0x5d, 0x20, 0xa7, 0xd0, 0xde, 0x60, 0x61, 0x8f, 0x3c, 0x5e, 0x6c, 0x4b, 0x32, 0xab, 0x1f,
	0xd8, 0x9b, 0x9d, 0x37, 0xac, 0x7a, 0xc0, 0xc2, 0xfe, 0xa6, 0xb4, 0x3a, 0xff, 0xba, 0x75, 0xe5,
	0x4c, 0xbe, 0x1c, 0x18, 0x1c, 0xaa, 0xe4, 0x06, 0x3a, 0x56, 0xdf, 0xc5, 0x9e, 0xfe, 0xe5, 0x45,
	0xcb, 0x4f, 0x99, 0xb7, 0x9a, 0x1b, 0x3d, 0x43, 0xaf, 0x46, 0x37, 0xa4, 0xa5, 0x87, 0x69, 0xdd,
	0x86, 0x0d, 0xd6, 0xa0, 0x96, 0xf4, 0xf6, 0xe9, 0xfd, 0x71, 0x1d, 0x18, 0x3f, 0xf3, 0xa8, 0x50,
	0x11, 0xf3, 0x8b, 0x04, 0x75, 0x88, 0x72, 0x8d, 0x9a, 0xad, 0xb8, 0xa7, 0x03, 0xc1, 0x84, 0xd2,
	0xc8, 0x2a, 0x6a, 0x93, 0x57, 0x85, 0xf9, 0x88, 0xd6, 0x91, 0x61, 0xd6, 0x5f, 0x7a, 0xac, 0xbe,
	0xc7, 0xeb, 0xd8, 0x27, 0x75, 0xf9, 0x13, 0x00, 0x00, 0xff, 0xff, 0x17, 0x0e, 0x59, 0x49, 0x67,
	0x02, 0x00, 0x00,
}
