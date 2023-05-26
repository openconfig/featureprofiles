// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v3.21.12
// source: metadata.proto

package metadata_go_proto

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"

	proto "github.com/openconfig/ondatra/proto"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Types of testbeds on which the test may run.
type Metadata_Testbed int32

const (
	Metadata_TESTBED_UNSPECIFIED    Metadata_Testbed = 0
	Metadata_TESTBED_DUT            Metadata_Testbed = 1
	Metadata_TESTBED_DUT_DUT_4LINKS Metadata_Testbed = 2
	Metadata_TESTBED_DUT_ATE_2LINKS Metadata_Testbed = 3
	Metadata_TESTBED_DUT_ATE_4LINKS Metadata_Testbed = 4
	Metadata_TESTBED_DUT_ATE_9LINKS Metadata_Testbed = 5
)

// Enum value maps for Metadata_Testbed.
var (
	Metadata_Testbed_name = map[int32]string{
		0: "TESTBED_UNSPECIFIED",
		1: "TESTBED_DUT",
		2: "TESTBED_DUT_DUT_4LINKS",
		3: "TESTBED_DUT_ATE_2LINKS",
		4: "TESTBED_DUT_ATE_4LINKS",
		5: "TESTBED_DUT_ATE_9LINKS",
	}
	Metadata_Testbed_value = map[string]int32{
		"TESTBED_UNSPECIFIED":    0,
		"TESTBED_DUT":            1,
		"TESTBED_DUT_DUT_4LINKS": 2,
		"TESTBED_DUT_ATE_2LINKS": 3,
		"TESTBED_DUT_ATE_4LINKS": 4,
		"TESTBED_DUT_ATE_9LINKS": 5,
	}
)

func (x Metadata_Testbed) Enum() *Metadata_Testbed {
	p := new(Metadata_Testbed)
	*p = x
	return p
}

func (x Metadata_Testbed) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Metadata_Testbed) Descriptor() protoreflect.EnumDescriptor {
	return file_metadata_proto_enumTypes[0].Descriptor()
}

func (Metadata_Testbed) Type() protoreflect.EnumType {
	return &file_metadata_proto_enumTypes[0]
}

func (x Metadata_Testbed) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Metadata_Testbed.Descriptor instead.
func (Metadata_Testbed) EnumDescriptor() ([]byte, []int) {
	return file_metadata_proto_rawDescGZIP(), []int{0, 0}
}

// Metadata about a Feature Profiles test.
type Metadata struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// UUID of the test.
	Uuid string `protobuf:"bytes,1,opt,name=uuid,proto3" json:"uuid,omitempty"`
	// ID of the test in the test plan.
	PlanId string `protobuf:"bytes,2,opt,name=plan_id,json=planId,proto3" json:"plan_id,omitempty"`
	// One-line description of the test.
	Description string `protobuf:"bytes,3,opt,name=description,proto3" json:"description,omitempty"`
	// Testbed on which the test is intended to run.
	Testbed            Metadata_Testbed               `protobuf:"varint,4,opt,name=testbed,proto3,enum=openconfig.testing.Metadata_Testbed" json:"testbed,omitempty"`
	PlatformExceptions []*Metadata_PlatformExceptions `protobuf:"bytes,5,rep,name=platform_exceptions,json=platformExceptions,proto3" json:"platform_exceptions,omitempty"`
}

func (x *Metadata) Reset() {
	*x = Metadata{}
	if protoimpl.UnsafeEnabled {
		mi := &file_metadata_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metadata) ProtoMessage() {}

func (x *Metadata) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metadata.ProtoReflect.Descriptor instead.
func (*Metadata) Descriptor() ([]byte, []int) {
	return file_metadata_proto_rawDescGZIP(), []int{0}
}

func (x *Metadata) GetUuid() string {
	if x != nil {
		return x.Uuid
	}
	return ""
}

func (x *Metadata) GetPlanId() string {
	if x != nil {
		return x.PlanId
	}
	return ""
}

func (x *Metadata) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *Metadata) GetTestbed() Metadata_Testbed {
	if x != nil {
		return x.Testbed
	}
	return Metadata_TESTBED_UNSPECIFIED
}

func (x *Metadata) GetPlatformExceptions() []*Metadata_PlatformExceptions {
	if x != nil {
		return x.PlatformExceptions
	}
	return nil
}

type Metadata_Platform struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Vendor of the device.
	Vendor proto.Device_Vendor `protobuf:"varint,1,opt,name=vendor,proto3,enum=ondatra.Device_Vendor" json:"vendor,omitempty"`
	// Hardware models of the device.
	HardwareModel []string `protobuf:"bytes,2,rep,name=hardware_model,json=hardwareModel,proto3" json:"hardware_model,omitempty"`
}

func (x *Metadata_Platform) Reset() {
	*x = Metadata_Platform{}
	if protoimpl.UnsafeEnabled {
		mi := &file_metadata_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metadata_Platform) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metadata_Platform) ProtoMessage() {}

func (x *Metadata_Platform) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metadata_Platform.ProtoReflect.Descriptor instead.
func (*Metadata_Platform) Descriptor() ([]byte, []int) {
	return file_metadata_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Metadata_Platform) GetVendor() proto.Device_Vendor {
	if x != nil {
		return x.Vendor
	}
	return proto.Device_Vendor(0)
}

func (x *Metadata_Platform) GetHardwareModel() []string {
	if x != nil {
		return x.HardwareModel
	}
	return nil
}

type Metadata_Deviations struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Device does not support interface/ipv4/enabled,
	// so suppress configuring this leaf.
	Ipv4MissingEnabled bool `protobuf:"varint,1,opt,name=ipv4_missing_enabled,json=ipv4MissingEnabled,proto3" json:"ipv4_missing_enabled,omitempty"`
	// Device does not support fragmentation bit for traceroute.
	TracerouteFragmentation bool `protobuf:"varint,2,opt,name=traceroute_fragmentation,json=tracerouteFragmentation,proto3" json:"traceroute_fragmentation,omitempty"`
	// Device only support UDP as l4 protocol for traceroute.
	TracerouteL4ProtocolUdp bool `protobuf:"varint,3,opt,name=traceroute_l4_protocol_udp,json=tracerouteL4ProtocolUdp,proto3" json:"traceroute_l4_protocol_udp,omitempty"`
}

func (x *Metadata_Deviations) Reset() {
	*x = Metadata_Deviations{}
	if protoimpl.UnsafeEnabled {
		mi := &file_metadata_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metadata_Deviations) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metadata_Deviations) ProtoMessage() {}

func (x *Metadata_Deviations) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metadata_Deviations.ProtoReflect.Descriptor instead.
func (*Metadata_Deviations) Descriptor() ([]byte, []int) {
	return file_metadata_proto_rawDescGZIP(), []int{0, 1}
}

func (x *Metadata_Deviations) GetIpv4MissingEnabled() bool {
	if x != nil {
		return x.Ipv4MissingEnabled
	}
	return false
}

func (x *Metadata_Deviations) GetTracerouteFragmentation() bool {
	if x != nil {
		return x.TracerouteFragmentation
	}
	return false
}

func (x *Metadata_Deviations) GetTracerouteL4ProtocolUdp() bool {
	if x != nil {
		return x.TracerouteL4ProtocolUdp
	}
	return false
}

type Metadata_PlatformExceptions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Platform   *Metadata_Platform   `protobuf:"bytes,1,opt,name=platform,proto3" json:"platform,omitempty"`
	Deviations *Metadata_Deviations `protobuf:"bytes,2,opt,name=deviations,proto3" json:"deviations,omitempty"`
}

func (x *Metadata_PlatformExceptions) Reset() {
	*x = Metadata_PlatformExceptions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_metadata_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metadata_PlatformExceptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metadata_PlatformExceptions) ProtoMessage() {}

func (x *Metadata_PlatformExceptions) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metadata_PlatformExceptions.ProtoReflect.Descriptor instead.
func (*Metadata_PlatformExceptions) Descriptor() ([]byte, []int) {
	return file_metadata_proto_rawDescGZIP(), []int{0, 2}
}

func (x *Metadata_PlatformExceptions) GetPlatform() *Metadata_Platform {
	if x != nil {
		return x.Platform
	}
	return nil
}

func (x *Metadata_PlatformExceptions) GetDeviations() *Metadata_Deviations {
	if x != nil {
		return x.Deviations
	}
	return nil
}

var File_metadata_proto protoreflect.FileDescriptor

var file_metadata_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x12, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73,
	0x74, 0x69, 0x6e, 0x67, 0x1a, 0x31, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f, 0x6f, 0x6e, 0x64, 0x61,
	0x74, 0x72, 0x61, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x62, 0x65,
	0x64, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xe0, 0x06, 0x0a, 0x08, 0x4d, 0x65, 0x74, 0x61,
	0x64, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a, 0x04, 0x75, 0x75, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x75, 0x75, 0x69, 0x64, 0x12, 0x17, 0x0a, 0x07, 0x70, 0x6c, 0x61, 0x6e,
	0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x70, 0x6c, 0x61, 0x6e, 0x49,
	0x64, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x12, 0x3e, 0x0a, 0x07, 0x74, 0x65, 0x73, 0x74, 0x62, 0x65, 0x64, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x0e, 0x32, 0x24, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61,
	0x74, 0x61, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x62, 0x65, 0x64, 0x52, 0x07, 0x74, 0x65, 0x73, 0x74,
	0x62, 0x65, 0x64, 0x12, 0x60, 0x0a, 0x13, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x5f,
	0x65, 0x78, 0x63, 0x65, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x2f, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65,
	0x73, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x50,
	0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x45, 0x78, 0x63, 0x65, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x52, 0x12, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x45, 0x78, 0x63, 0x65, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x1a, 0x61, 0x0a, 0x08, 0x50, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72,
	0x6d, 0x12, 0x2e, 0x0a, 0x06, 0x76, 0x65, 0x6e, 0x64, 0x6f, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0e, 0x32, 0x16, 0x2e, 0x6f, 0x6e, 0x64, 0x61, 0x74, 0x72, 0x61, 0x2e, 0x44, 0x65, 0x76, 0x69,
	0x63, 0x65, 0x2e, 0x56, 0x65, 0x6e, 0x64, 0x6f, 0x72, 0x52, 0x06, 0x76, 0x65, 0x6e, 0x64, 0x6f,
	0x72, 0x12, 0x25, 0x0a, 0x0e, 0x68, 0x61, 0x72, 0x64, 0x77, 0x61, 0x72, 0x65, 0x5f, 0x6d, 0x6f,
	0x64, 0x65, 0x6c, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0d, 0x68, 0x61, 0x72, 0x64, 0x77,
	0x61, 0x72, 0x65, 0x4d, 0x6f, 0x64, 0x65, 0x6c, 0x1a, 0xb6, 0x01, 0x0a, 0x0a, 0x44, 0x65, 0x76,
	0x69, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x30, 0x0a, 0x14, 0x69, 0x70, 0x76, 0x34, 0x5f,
	0x6d, 0x69, 0x73, 0x73, 0x69, 0x6e, 0x67, 0x5f, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x12, 0x69, 0x70, 0x76, 0x34, 0x4d, 0x69, 0x73, 0x73, 0x69,
	0x6e, 0x67, 0x45, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x12, 0x39, 0x0a, 0x18, 0x74, 0x72, 0x61,
	0x63, 0x65, 0x72, 0x6f, 0x75, 0x74, 0x65, 0x5f, 0x66, 0x72, 0x61, 0x67, 0x6d, 0x65, 0x6e, 0x74,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x17, 0x74, 0x72, 0x61,
	0x63, 0x65, 0x72, 0x6f, 0x75, 0x74, 0x65, 0x46, 0x72, 0x61, 0x67, 0x6d, 0x65, 0x6e, 0x74, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x12, 0x3b, 0x0a, 0x1a, 0x74, 0x72, 0x61, 0x63, 0x65, 0x72, 0x6f, 0x75,
	0x74, 0x65, 0x5f, 0x6c, 0x34, 0x5f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x5f, 0x75,
	0x64, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x17, 0x74, 0x72, 0x61, 0x63, 0x65, 0x72,
	0x6f, 0x75, 0x74, 0x65, 0x4c, 0x34, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x55, 0x64,
	0x70, 0x1a, 0xa0, 0x01, 0x0a, 0x12, 0x50, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x45, 0x78,
	0x63, 0x65, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x41, 0x0a, 0x08, 0x70, 0x6c, 0x61, 0x74,
	0x66, 0x6f, 0x72, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x25, 0x2e, 0x6f, 0x70, 0x65,
	0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x67, 0x2e,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x50, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72,
	0x6d, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x12, 0x47, 0x0a, 0x0a, 0x64,
	0x65, 0x76, 0x69, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x27, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73,
	0x74, 0x69, 0x6e, 0x67, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x44, 0x65,
	0x76, 0x69, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x0a, 0x64, 0x65, 0x76, 0x69, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x22, 0xa3, 0x01, 0x0a, 0x07, 0x54, 0x65, 0x73, 0x74, 0x62, 0x65, 0x64,
	0x12, 0x17, 0x0a, 0x13, 0x54, 0x45, 0x53, 0x54, 0x42, 0x45, 0x44, 0x5f, 0x55, 0x4e, 0x53, 0x50,
	0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x0f, 0x0a, 0x0b, 0x54, 0x45, 0x53,
	0x54, 0x42, 0x45, 0x44, 0x5f, 0x44, 0x55, 0x54, 0x10, 0x01, 0x12, 0x1a, 0x0a, 0x16, 0x54, 0x45,
	0x53, 0x54, 0x42, 0x45, 0x44, 0x5f, 0x44, 0x55, 0x54, 0x5f, 0x44, 0x55, 0x54, 0x5f, 0x34, 0x4c,
	0x49, 0x4e, 0x4b, 0x53, 0x10, 0x02, 0x12, 0x1a, 0x0a, 0x16, 0x54, 0x45, 0x53, 0x54, 0x42, 0x45,
	0x44, 0x5f, 0x44, 0x55, 0x54, 0x5f, 0x41, 0x54, 0x45, 0x5f, 0x32, 0x4c, 0x49, 0x4e, 0x4b, 0x53,
	0x10, 0x03, 0x12, 0x1a, 0x0a, 0x16, 0x54, 0x45, 0x53, 0x54, 0x42, 0x45, 0x44, 0x5f, 0x44, 0x55,
	0x54, 0x5f, 0x41, 0x54, 0x45, 0x5f, 0x34, 0x4c, 0x49, 0x4e, 0x4b, 0x53, 0x10, 0x04, 0x12, 0x1a,
	0x0a, 0x16, 0x54, 0x45, 0x53, 0x54, 0x42, 0x45, 0x44, 0x5f, 0x44, 0x55, 0x54, 0x5f, 0x41, 0x54,
	0x45, 0x5f, 0x39, 0x4c, 0x49, 0x4e, 0x4b, 0x53, 0x10, 0x05, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_metadata_proto_rawDescOnce sync.Once
	file_metadata_proto_rawDescData = file_metadata_proto_rawDesc
)

func file_metadata_proto_rawDescGZIP() []byte {
	file_metadata_proto_rawDescOnce.Do(func() {
		file_metadata_proto_rawDescData = protoimpl.X.CompressGZIP(file_metadata_proto_rawDescData)
	})
	return file_metadata_proto_rawDescData
}

var file_metadata_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_metadata_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_metadata_proto_goTypes = []interface{}{
	(Metadata_Testbed)(0),               // 0: openconfig.testing.Metadata.Testbed
	(*Metadata)(nil),                    // 1: openconfig.testing.Metadata
	(*Metadata_Platform)(nil),           // 2: openconfig.testing.Metadata.Platform
	(*Metadata_Deviations)(nil),         // 3: openconfig.testing.Metadata.Deviations
	(*Metadata_PlatformExceptions)(nil), // 4: openconfig.testing.Metadata.PlatformExceptions
	(proto.Device_Vendor)(0),            // 5: ondatra.Device.Vendor
}
var file_metadata_proto_depIdxs = []int32{
	0, // 0: openconfig.testing.Metadata.testbed:type_name -> openconfig.testing.Metadata.Testbed
	4, // 1: openconfig.testing.Metadata.platform_exceptions:type_name -> openconfig.testing.Metadata.PlatformExceptions
	5, // 2: openconfig.testing.Metadata.Platform.vendor:type_name -> ondatra.Device.Vendor
	2, // 3: openconfig.testing.Metadata.PlatformExceptions.platform:type_name -> openconfig.testing.Metadata.Platform
	3, // 4: openconfig.testing.Metadata.PlatformExceptions.deviations:type_name -> openconfig.testing.Metadata.Deviations
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_metadata_proto_init() }
func file_metadata_proto_init() {
	if File_metadata_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_metadata_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metadata); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_metadata_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metadata_Platform); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_metadata_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metadata_Deviations); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_metadata_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metadata_PlatformExceptions); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_metadata_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_metadata_proto_goTypes,
		DependencyIndexes: file_metadata_proto_depIdxs,
		EnumInfos:         file_metadata_proto_enumTypes,
		MessageInfos:      file_metadata_proto_msgTypes,
	}.Build()
	File_metadata_proto = out.File
	file_metadata_proto_rawDesc = nil
	file_metadata_proto_goTypes = nil
	file_metadata_proto_depIdxs = nil
}
