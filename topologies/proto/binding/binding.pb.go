// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        v3.21.12
// source: binding.proto

package binding

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// A binding configuration.
type Binding struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Duts []*Device `protobuf:"bytes,1,rep,name=duts,proto3" json:"duts,omitempty"`
	Ates []*Device `protobuf:"bytes,2,rep,name=ates,proto3" json:"ates,omitempty"`
	// Dial options across all devices, unless overridden by the device.
	Options *Options `protobuf:"bytes,3,opt,name=options,proto3" json:"options,omitempty"`
}

func (x *Binding) Reset() {
	*x = Binding{}
	if protoimpl.UnsafeEnabled {
		mi := &file_binding_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Binding) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Binding) ProtoMessage() {}

func (x *Binding) ProtoReflect() protoreflect.Message {
	mi := &file_binding_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Binding.ProtoReflect.Descriptor instead.
func (*Binding) Descriptor() ([]byte, []int) {
	return file_binding_proto_rawDescGZIP(), []int{0}
}

func (x *Binding) GetDuts() []*Device {
	if x != nil {
		return x.Duts
	}
	return nil
}

func (x *Binding) GetAtes() []*Device {
	if x != nil {
		return x.Ates
	}
	return nil
}

func (x *Binding) GetOptions() *Options {
	if x != nil {
		return x.Options
	}
	return nil
}

// Config for resetting the device before the test run.
type Configs struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Raw device config
	Cli [][]byte `protobuf:"bytes,1,rep,name=cli,proto3" json:"cli,omitempty"`
	// Path to file containing raw device config
	CliFile []string `protobuf:"bytes,2,rep,name=cli_file,json=cliFile,proto3" json:"cli_file,omitempty"`
	// Path to a file containing gNMI SetRequest as text-formatted proto.
	GnmiSetFile []string `protobuf:"bytes,3,rep,name=gnmi_set_file,json=gnmiSetFile,proto3" json:"gnmi_set_file,omitempty"`
	// Whether to flush gRIBI.  If true, this will send a FlushRequest for all
	// network instances and overriding the election ID.
	GribiFlush bool `protobuf:"varint,4,opt,name=gribi_flush,json=gribiFlush,proto3" json:"gribi_flush,omitempty"`
}

func (x *Configs) Reset() {
	*x = Configs{}
	if protoimpl.UnsafeEnabled {
		mi := &file_binding_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Configs) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Configs) ProtoMessage() {}

func (x *Configs) ProtoReflect() protoreflect.Message {
	mi := &file_binding_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Configs.ProtoReflect.Descriptor instead.
func (*Configs) Descriptor() ([]byte, []int) {
	return file_binding_proto_rawDescGZIP(), []int{1}
}

func (x *Configs) GetCli() [][]byte {
	if x != nil {
		return x.Cli
	}
	return nil
}

func (x *Configs) GetCliFile() []string {
	if x != nil {
		return x.CliFile
	}
	return nil
}

func (x *Configs) GetGnmiSetFile() []string {
	if x != nil {
		return x.GnmiSetFile
	}
	return nil
}

func (x *Configs) GetGribiFlush() bool {
	if x != nil {
		return x.GribiFlush
	}
	return false
}

// A device binding.
type Device struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Device ID as it appears in the testbed.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// The actual device hostname to be used for the binding.
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	// Dial options across all protocols of this device, unless
	// overrideen by individual protocols.
	Options *Options `protobuf:"bytes,3,opt,name=options,proto3" json:"options,omitempty"`
	// Port bindings for this device.
	Ports []*Port `protobuf:"bytes,4,rep,name=ports,proto3" json:"ports,omitempty"`
	// Configs to apply to device after binding
	Config *Configs `protobuf:"bytes,5,opt,name=config,proto3" json:"config,omitempty"`
	// Dial options for SSH (DUT only).
	Ssh *Options `protobuf:"bytes,11,opt,name=ssh,proto3" json:"ssh,omitempty"`
	// Dial options for gNMI (DUT only).
	Gnmi *Options `protobuf:"bytes,12,opt,name=gnmi,proto3" json:"gnmi,omitempty"`
	// Dial options for gNOI (DUT only).
	Gnoi *Options `protobuf:"bytes,13,opt,name=gnoi,proto3" json:"gnoi,omitempty"`
	// Dial options for gNSI (DUT only).
	Gnsi *Options `protobuf:"bytes,14,opt,name=gnsi,proto3" json:"gnsi,omitempty"`
	// Dial options for gRIBI (DUT only).
	Gribi *Options `protobuf:"bytes,15,opt,name=gribi,proto3" json:"gribi,omitempty"`
	// Dial options for P4RT (DUT only).
	P4Rt *Options `protobuf:"bytes,16,opt,name=p4rt,proto3" json:"p4rt,omitempty"`
	// Dial options for IxNetwork (ATE only).
	Ixnetwork *Options `protobuf:"bytes,17,opt,name=ixnetwork,proto3" json:"ixnetwork,omitempty"`
	// Dial options for OTG Ixia-C (HW Only).
	Otg *Options `protobuf:"bytes,18,opt,name=otg,proto3" json:"otg,omitempty"`
}

func (x *Device) Reset() {
	*x = Device{}
	if protoimpl.UnsafeEnabled {
		mi := &file_binding_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Device) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Device) ProtoMessage() {}

func (x *Device) ProtoReflect() protoreflect.Message {
	mi := &file_binding_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Device.ProtoReflect.Descriptor instead.
func (*Device) Descriptor() ([]byte, []int) {
	return file_binding_proto_rawDescGZIP(), []int{2}
}

func (x *Device) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Device) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Device) GetOptions() *Options {
	if x != nil {
		return x.Options
	}
	return nil
}

func (x *Device) GetPorts() []*Port {
	if x != nil {
		return x.Ports
	}
	return nil
}

func (x *Device) GetConfig() *Configs {
	if x != nil {
		return x.Config
	}
	return nil
}

func (x *Device) GetSsh() *Options {
	if x != nil {
		return x.Ssh
	}
	return nil
}

func (x *Device) GetGnmi() *Options {
	if x != nil {
		return x.Gnmi
	}
	return nil
}

func (x *Device) GetGnoi() *Options {
	if x != nil {
		return x.Gnoi
	}
	return nil
}

func (x *Device) GetGnsi() *Options {
	if x != nil {
		return x.Gnsi
	}
	return nil
}

func (x *Device) GetGribi() *Options {
	if x != nil {
		return x.Gribi
	}
	return nil
}

func (x *Device) GetP4Rt() *Options {
	if x != nil {
		return x.P4Rt
	}
	return nil
}

func (x *Device) GetIxnetwork() *Options {
	if x != nil {
		return x.Ixnetwork
	}
	return nil
}

func (x *Device) GetOtg() *Options {
	if x != nil {
		return x.Otg
	}
	return nil
}

// Dial options.
type Options struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// This is the dial target, typically formatted as "hostname:port".
	// If not set, it will use the device name and the default port for
	// the protocol.
	Target string `protobuf:"bytes,1,opt,name=target,proto3" json:"target,omitempty"`
	// Use plain HTTP/2 and omit TLS (gRPC only).
	Insecure bool `protobuf:"varint,2,opt,name=insecure,proto3" json:"insecure,omitempty"`
	// When using TLS, skip certificate verification (gRPC and HTTP).
	SkipVerify bool `protobuf:"varint,3,opt,name=skip_verify,json=skipVerify,proto3" json:"skip_verify,omitempty"`
	// The username for authentication.
	Username string `protobuf:"bytes,4,opt,name=username,proto3" json:"username,omitempty"`
	// The password for authentication.
	Password string `protobuf:"bytes,5,opt,name=password,proto3" json:"password,omitempty"`
	// The session_id for ATE REST API session id
	SessionId int32 `protobuf:"varint,6,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	// gRPC request timeout (second)
	Timeout int32 `protobuf:"varint,7,opt,name=timeout,proto3" json:"timeout,omitempty"`
	// gRPC dial option to set the maximum recv message size in bytes.
	MaxRecvMsgSize int32 `protobuf:"varint,8,opt,name=max_recv_msg_size,json=maxRecvMsgSize,proto3" json:"max_recv_msg_size,omitempty"`
	// For Dial SSH use keyboard-interactive as the authentication method instead of password.
	KeyboardInteractiveSsh bool `protobuf:"varint,9,opt,name=keyboard_interactive_ssh,json=keyboardInteractiveSsh,proto3" json:"keyboard_interactive_ssh,omitempty"`
}

func (x *Options) Reset() {
	*x = Options{}
	if protoimpl.UnsafeEnabled {
		mi := &file_binding_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Options) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Options) ProtoMessage() {}

func (x *Options) ProtoReflect() protoreflect.Message {
	mi := &file_binding_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Options.ProtoReflect.Descriptor instead.
func (*Options) Descriptor() ([]byte, []int) {
	return file_binding_proto_rawDescGZIP(), []int{3}
}

func (x *Options) GetTarget() string {
	if x != nil {
		return x.Target
	}
	return ""
}

func (x *Options) GetInsecure() bool {
	if x != nil {
		return x.Insecure
	}
	return false
}

func (x *Options) GetSkipVerify() bool {
	if x != nil {
		return x.SkipVerify
	}
	return false
}

func (x *Options) GetUsername() string {
	if x != nil {
		return x.Username
	}
	return ""
}

func (x *Options) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

func (x *Options) GetSessionId() int32 {
	if x != nil {
		return x.SessionId
	}
	return 0
}

func (x *Options) GetTimeout() int32 {
	if x != nil {
		return x.Timeout
	}
	return 0
}

func (x *Options) GetMaxRecvMsgSize() int32 {
	if x != nil {
		return x.MaxRecvMsgSize
	}
	return 0
}

func (x *Options) GetKeyboardInteractiveSsh() bool {
	if x != nil {
		return x.KeyboardInteractiveSsh
	}
	return false
}

// Port binding.
type Port struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Port ID as it appears in the testbed.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// The actual port name to be used for the binding.
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Port) Reset() {
	*x = Port{}
	if protoimpl.UnsafeEnabled {
		mi := &file_binding_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Port) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Port) ProtoMessage() {}

func (x *Port) ProtoReflect() protoreflect.Message {
	mi := &file_binding_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Port.ProtoReflect.Descriptor instead.
func (*Port) Descriptor() ([]byte, []int) {
	return file_binding_proto_rawDescGZIP(), []int{4}
}

func (x *Port) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Port) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

var File_binding_proto protoreflect.FileDescriptor

var file_binding_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x62, 0x69, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x12, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74,
	0x69, 0x6e, 0x67, 0x22, 0xa0, 0x01, 0x0a, 0x07, 0x42, 0x69, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x12,
	0x2e, 0x0a, 0x04, 0x64, 0x75, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1a, 0x2e,
	0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69,
	0x6e, 0x67, 0x2e, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x52, 0x04, 0x64, 0x75, 0x74, 0x73, 0x12,
	0x2e, 0x0a, 0x04, 0x61, 0x74, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1a, 0x2e,
	0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69,
	0x6e, 0x67, 0x2e, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x52, 0x04, 0x61, 0x74, 0x65, 0x73, 0x12,
	0x35, 0x0a, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1b, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65,
	0x73, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x07, 0x6f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0x7b, 0x0a, 0x07, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x73, 0x12, 0x10, 0x0a, 0x03, 0x63, 0x6c, 0x69, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x03,
	0x63, 0x6c, 0x69, 0x12, 0x19, 0x0a, 0x08, 0x63, 0x6c, 0x69, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x18,
	0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6c, 0x69, 0x46, 0x69, 0x6c, 0x65, 0x12, 0x22,
	0x0a, 0x0d, 0x67, 0x6e, 0x6d, 0x69, 0x5f, 0x73, 0x65, 0x74, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x18,
	0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0b, 0x67, 0x6e, 0x6d, 0x69, 0x53, 0x65, 0x74, 0x46, 0x69,
	0x6c, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x67, 0x72, 0x69, 0x62, 0x69, 0x5f, 0x66, 0x6c, 0x75, 0x73,
	0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0a, 0x67, 0x72, 0x69, 0x62, 0x69, 0x46, 0x6c,
	0x75, 0x73, 0x68, 0x22, 0xd8, 0x04, 0x0a, 0x06, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x12, 0x0e,
	0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x35, 0x0a, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x52, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x2e, 0x0a, 0x05, 0x70, 0x6f, 0x72,
	0x74, 0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x50, 0x6f,
	0x72, 0x74, 0x52, 0x05, 0x70, 0x6f, 0x72, 0x74, 0x73, 0x12, 0x33, 0x0a, 0x06, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x6f, 0x70, 0x65, 0x6e,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x43,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x73, 0x52, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x2d,
	0x0a, 0x03, 0x73, 0x73, 0x68, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x6f, 0x70,
	0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x67,
	0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x03, 0x73, 0x73, 0x68, 0x12, 0x2f, 0x0a,
	0x04, 0x67, 0x6e, 0x6d, 0x69, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x6f, 0x70,
	0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x67,
	0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x04, 0x67, 0x6e, 0x6d, 0x69, 0x12, 0x2f,
	0x0a, 0x04, 0x67, 0x6e, 0x6f, 0x69, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x6f,
	0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e,
	0x67, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x04, 0x67, 0x6e, 0x6f, 0x69, 0x12,
	0x2f, 0x0a, 0x04, 0x67, 0x6e, 0x73, 0x69, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e,
	0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69,
	0x6e, 0x67, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x04, 0x67, 0x6e, 0x73, 0x69,
	0x12, 0x31, 0x0a, 0x05, 0x67, 0x72, 0x69, 0x62, 0x69, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1b, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73,
	0x74, 0x69, 0x6e, 0x67, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x05, 0x67, 0x72,
	0x69, 0x62, 0x69, 0x12, 0x2f, 0x0a, 0x04, 0x70, 0x34, 0x72, 0x74, 0x18, 0x10, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x1b, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74,
	0x65, 0x73, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x04,
	0x70, 0x34, 0x72, 0x74, 0x12, 0x39, 0x0a, 0x09, 0x69, 0x78, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72,
	0x6b, 0x18, 0x11, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x4f, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x52, 0x09, 0x69, 0x78, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x12,
	0x2d, 0x0a, 0x03, 0x6f, 0x74, 0x67, 0x18, 0x12, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x6f,
	0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e,
	0x67, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x03, 0x6f, 0x74, 0x67, 0x22, 0xb4,
	0x02, 0x0a, 0x07, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x74, 0x61,
	0x72, 0x67, 0x65, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x74, 0x61, 0x72, 0x67,
	0x65, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x69, 0x6e, 0x73, 0x65, 0x63, 0x75, 0x72, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x69, 0x6e, 0x73, 0x65, 0x63, 0x75, 0x72, 0x65, 0x12, 0x1f,
	0x0a, 0x0b, 0x73, 0x6b, 0x69, 0x70, 0x5f, 0x76, 0x65, 0x72, 0x69, 0x66, 0x79, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x0a, 0x73, 0x6b, 0x69, 0x70, 0x56, 0x65, 0x72, 0x69, 0x66, 0x79, 0x12,
	0x1a, 0x0a, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x70,
	0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x70,
	0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x65, 0x73, 0x73, 0x69,
	0x6f, 0x6e, 0x5f, 0x69, 0x64, 0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x73, 0x65, 0x73,
	0x73, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75,
	0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x05, 0x52, 0x07, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74,
	0x12, 0x29, 0x0a, 0x11, 0x6d, 0x61, 0x78, 0x5f, 0x72, 0x65, 0x63, 0x76, 0x5f, 0x6d, 0x73, 0x67,
	0x5f, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0e, 0x6d, 0x61, 0x78,
	0x52, 0x65, 0x63, 0x76, 0x4d, 0x73, 0x67, 0x53, 0x69, 0x7a, 0x65, 0x12, 0x38, 0x0a, 0x18, 0x6b,
	0x65, 0x79, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x5f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x61, 0x63, 0x74,
	0x69, 0x76, 0x65, 0x5f, 0x73, 0x73, 0x68, 0x18, 0x09, 0x20, 0x01, 0x28, 0x08, 0x52, 0x16, 0x6b,
	0x65, 0x79, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x61, 0x63, 0x74, 0x69,
	0x76, 0x65, 0x53, 0x73, 0x68, 0x22, 0x2a, 0x0a, 0x04, 0x50, 0x6f, 0x72, 0x74, 0x12, 0x0e, 0x0a,
	0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x42, 0x40, 0x5a, 0x3e, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x6f, 0x70, 0x65, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f, 0x66, 0x65, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x70, 0x72, 0x6f, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x2f, 0x74, 0x6f, 0x70, 0x6f, 0x6c,
	0x6f, 0x67, 0x69, 0x65, 0x73, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x69, 0x6e, 0x64,
	0x69, 0x6e, 0x67, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_binding_proto_rawDescOnce sync.Once
	file_binding_proto_rawDescData = file_binding_proto_rawDesc
)

func file_binding_proto_rawDescGZIP() []byte {
	file_binding_proto_rawDescOnce.Do(func() {
		file_binding_proto_rawDescData = protoimpl.X.CompressGZIP(file_binding_proto_rawDescData)
	})
	return file_binding_proto_rawDescData
}

var file_binding_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_binding_proto_goTypes = []interface{}{
	(*Binding)(nil), // 0: openconfig.testing.Binding
	(*Configs)(nil), // 1: openconfig.testing.Configs
	(*Device)(nil),  // 2: openconfig.testing.Device
	(*Options)(nil), // 3: openconfig.testing.Options
	(*Port)(nil),    // 4: openconfig.testing.Port
}
var file_binding_proto_depIdxs = []int32{
	2,  // 0: openconfig.testing.Binding.duts:type_name -> openconfig.testing.Device
	2,  // 1: openconfig.testing.Binding.ates:type_name -> openconfig.testing.Device
	3,  // 2: openconfig.testing.Binding.options:type_name -> openconfig.testing.Options
	3,  // 3: openconfig.testing.Device.options:type_name -> openconfig.testing.Options
	4,  // 4: openconfig.testing.Device.ports:type_name -> openconfig.testing.Port
	1,  // 5: openconfig.testing.Device.config:type_name -> openconfig.testing.Configs
	3,  // 6: openconfig.testing.Device.ssh:type_name -> openconfig.testing.Options
	3,  // 7: openconfig.testing.Device.gnmi:type_name -> openconfig.testing.Options
	3,  // 8: openconfig.testing.Device.gnoi:type_name -> openconfig.testing.Options
	3,  // 9: openconfig.testing.Device.gnsi:type_name -> openconfig.testing.Options
	3,  // 10: openconfig.testing.Device.gribi:type_name -> openconfig.testing.Options
	3,  // 11: openconfig.testing.Device.p4rt:type_name -> openconfig.testing.Options
	3,  // 12: openconfig.testing.Device.ixnetwork:type_name -> openconfig.testing.Options
	3,  // 13: openconfig.testing.Device.otg:type_name -> openconfig.testing.Options
	14, // [14:14] is the sub-list for method output_type
	14, // [14:14] is the sub-list for method input_type
	14, // [14:14] is the sub-list for extension type_name
	14, // [14:14] is the sub-list for extension extendee
	0,  // [0:14] is the sub-list for field type_name
}

func init() { file_binding_proto_init() }
func file_binding_proto_init() {
	if File_binding_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_binding_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Binding); i {
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
		file_binding_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Configs); i {
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
		file_binding_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Device); i {
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
		file_binding_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Options); i {
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
		file_binding_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Port); i {
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
			RawDescriptor: file_binding_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_binding_proto_goTypes,
		DependencyIndexes: file_binding_proto_depIdxs,
		MessageInfos:      file_binding_proto_msgTypes,
	}.Build()
	File_binding_proto = out.File
	file_binding_proto_rawDesc = nil
	file_binding_proto_goTypes = nil
	file_binding_proto_depIdxs = nil
}
