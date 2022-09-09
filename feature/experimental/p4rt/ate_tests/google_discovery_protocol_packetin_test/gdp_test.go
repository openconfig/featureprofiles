// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package google_discovery_protocol_packetin_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/wbb"
	"github.com/openconfig/ondatra"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

type GDPPacketIO struct {
	PacketIOPacket
	IngressPort string
}

// GetTableEntry creates wbb acl entry related to GDP.
func (gdp *GDPPacketIO) GetTableEntry(t *testing.T, delete bool) []*wbb.AclWbbIngressTableEntryInfo {
	actionType := p4_v1.Update_INSERT
	if delete {
		actionType = p4_v1.Update_DELETE
	}
	return []*wbb.AclWbbIngressTableEntryInfo{{
		Type:          actionType,
		EtherType:     0x6007,
		EtherTypeMask: 0xFFFF,
	}}
}

// GetPacketTemplate returns expected packets in PacketIn.
func (gdp *GDPPacketIO) GetPacketTemplate(t *testing.T) *PacketIOPacket {
	return &gdp.PacketIOPacket
}

// GetTrafficFlow generates ATE traffic flows for GDP.
func (gdp *GDPPacketIO) GetTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, frameSize uint32, frameRate uint64) []*ondatra.Flow {
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithSrcAddress(*gdp.SrcMAC)
	ethHeader.WithDstAddress(*gdp.DstMAC)
	ethHeader.WithEtherType(*gdp.EthernetType)

	flow := ate.Traffic().NewFlow("GDP").WithFrameSize(frameSize).WithFrameRateFPS(frameRate).WithHeaders(ethHeader)
	return []*ondatra.Flow{flow}
}

// GetEgressPort returns expected egress port info in PacketIn.
func (gdp *GDPPacketIO) GetEgressPort(t *testing.T) []string {
	return []string{"0"}
}

// GetIngressPort return expected ingress port info in PacketIn.
func (gdp *GDPPacketIO) GetIngressPort(t *testing.T) string {
	return gdp.IngressPort
}
