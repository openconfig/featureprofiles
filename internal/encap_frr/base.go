// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package base contains utility functions for encap frr using repair VRF.
package base

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4               = 30
	magicIP                = "192.168.1.1"
	magicMAC               = "02:00:00:00:00:01"
	maskLen24              = "24"
	maskLen32              = "32"
	niEncapTeVrfA          = "ENCAP_TE_VRF_A"
	niTEVRF111             = "TE_VRF_111"
	niTEVRF222             = "TE_VRF_222"
	niRepairVrf            = "REPAIR_VRF"
	ipv4OuterSrc111Addr    = "198.51.100.111"
	ipv4OuterSrc222Addr    = "198.51.100.222"
	gribiIPv4EntryDefVRF1  = "192.0.2.101"
	gribiIPv4EntryDefVRF2  = "192.0.2.102"
	gribiIPv4EntryDefVRF3  = "192.0.2.103"
	gribiIPv4EntryDefVRF4  = "192.0.2.104"
	gribiIPv4EntryDefVRF5  = "192.0.2.105"
	gribiIPv4EntryVRF1111  = "203.0.113.1"
	gribiIPv4EntryVRF1112  = "203.0.113.2"
	gribiIPv4EntryVRF2221  = "203.0.113.100"
	gribiIPv4EntryVRF2222  = "203.0.113.101"
	gribiIPv4EntryEncapVRF = "138.0.11.0"
	noMatchEncapDest       = "20.0.0.1"
)

var (
	dutPort2DummyIP = attrs.Attributes{
		Desc:       "dutPort2",
		IPv4Sec:    "192.0.2.33",
		IPv4LenSec: plenIPv4,
	}

	otgPort2DummyIP = attrs.Attributes{
		Desc:    "otgPort2",
		IPv4:    "192.0.2.34",
		IPv4Len: plenIPv4,
	}

	dutPort3DummyIP = attrs.Attributes{
		Desc:       "dutPort3",
		IPv4Sec:    "192.0.2.37",
		IPv4LenSec: plenIPv4,
	}

	otgPort3DummyIP = attrs.Attributes{
		Desc:    "otgPort3",
		IPv4:    "192.0.2.38",
		IPv4Len: plenIPv4,
	}

	dutPort4DummyIP = attrs.Attributes{
		Desc:       "dutPort4",
		IPv4Sec:    "192.0.2.41",
		IPv4LenSec: plenIPv4,
	}

	otgPort4DummyIP = attrs.Attributes{
		Desc:    "otgPort4",
		IPv4:    "192.0.2.42",
		IPv4Len: plenIPv4,
	}

	dutPort5DummyIP = attrs.Attributes{
		Desc:       "dutPort5",
		IPv4Sec:    "192.0.2.45",
		IPv4LenSec: plenIPv4,
	}

	otgPort5DummyIP = attrs.Attributes{
		Desc:    "otgPort5",
		IPv4:    "192.0.2.46",
		IPv4Len: plenIPv4,
	}
	dutPort6DummyIP = attrs.Attributes{
		Desc:       "dutPort5",
		IPv4Sec:    "192.0.2.49",
		IPv4LenSec: plenIPv4,
	}

	otgPort6DummyIP = attrs.Attributes{
		Desc:    "otgPort5",
		IPv4:    "192.0.2.50",
		IPv4Len: plenIPv4,
	}
	dutPort7DummyIP = attrs.Attributes{
		Desc:       "dutPort5",
		IPv4Sec:    "192.0.2.53",
		IPv4LenSec: plenIPv4,
	}

	otgPort7DummyIP = attrs.Attributes{
		Desc:    "otgPort5",
		IPv4:    "192.0.2.54",
		IPv4Len: plenIPv4,
	}
)

func programAftWithDummyIP(t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(11).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port2").Name()).
			WithIPAddress(otgPort2DummyIP.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(12).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port3").Name()).
			WithIPAddress(otgPort3DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF1+"/"+maskLen32).WithNextHopGroup(11),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(13).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port4").Name()).
			WithIPAddress(otgPort4DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(12).AddNextHop(13, 2),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF2+"/"+maskLen32).WithNextHopGroup(12),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(14).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port5").Name()).
			WithIPAddress(otgPort5DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(13).AddNextHop(14, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF3+"/"+maskLen32).WithNextHopGroup(13),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(15).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port6").Name()).
			WithIPAddress(otgPort6DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(14).AddNextHop(15, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF4+"/"+maskLen32).WithNextHopGroup(14),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(16).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port7").Name()).
			WithIPAddress(otgPort7DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(15).AddNextHop(16, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF5+"/"+maskLen32).WithNextHopGroup(15),
	)
}

// configStaticArp configures static arp entries
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

// StaticARPWithSpecificIP configures secondary IPs and static ARP.
func StaticARPWithSpecificIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	p6 := dut.Port(t, "port6")
	p7 := dut.Port(t, "port7")
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4DummyIP.NewOCInterface(p4.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), dutPort5DummyIP.NewOCInterface(p5.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p6.Name()).Config(), dutPort6DummyIP.NewOCInterface(p6.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p7.Name()).Config(), dutPort7DummyIP.NewOCInterface(p7.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), otgPort2DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), configStaticArp(p5.Name(), otgPort5DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p6.Name()).Config(), configStaticArp(p6.Name(), otgPort6DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p7.Name()).Config(), configStaticArp(p7.Name(), otgPort7DummyIP.IPv4, magicMAC))
}

// StaticARPWithMagicUniversalIP programs the static ARP with magic universal IP
func StaticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	sb := &gnmi.SetBatch{}
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	p6 := dut.Port(t, "port6")
	p7 := dut.Port(t, "port7")
	portList := []*ondatra.Port{p2, p3, p4, p5, p6, p7}
	for idx, p := range portList {
		s := &oc.NetworkInstance_Protocol_Static{
			Prefix: ygot.String(magicIP + "/32"),
			NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
				strconv.Itoa(idx): {
					Index: ygot.String(strconv.Itoa(idx)),
					InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
						Interface: ygot.String(p.Name()),
					},
				},
			},
		}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.BatchUpdate(sb, sp.Static(magicIP+"/32").Config(), s)
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(p.Name()).Config(), configStaticArp(p.Name(), magicIP, magicMAC))
	}
	sb.Set(t, dut)
}

// ConfigureBaseGribiRoutes programs the base gribi routes for encap FRR using repair VRF
func ConfigureBaseGribiRoutes(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()

	// Programming AFT entries for prefixes in DEFAULT VRF
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(11).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port2").Name()).WithIPAddress(magicIP),
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(12).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port3").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(13).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port4").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(14).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port5").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(15).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port6").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(16).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port7").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(15).AddNextHop(16, 1),
		)
	} else if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		programAftWithDummyIP(t, dut, client)
	} else {
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(11).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port2").Name()),
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(12).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port3").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(13).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port4").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(14).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port5").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(15).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port6").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(16).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port7").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(15).AddNextHop(16, 1),
		)
	}
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	client.Modify().AddEntry(t,
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF1+"/"+maskLen32).WithNextHopGroup(11),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF2+"/"+maskLen32).WithNextHopGroup(12),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF3+"/"+maskLen32).WithNextHopGroup(13),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF4+"/"+maskLen32).WithNextHopGroup(14),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF5+"/"+maskLen32).WithNextHopGroup(15),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	defaultVRFIPList := []string{gribiIPv4EntryDefVRF1, gribiIPv4EntryDefVRF2, gribiIPv4EntryDefVRF3, gribiIPv4EntryDefVRF4, gribiIPv4EntryDefVRF5}
	for ip := range defaultVRFIPList {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(defaultVRFIPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for backup NHG
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2000).WithDecapsulateHeader(fluent.IPinIP).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2000).AddNextHop(2000, 1),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	// Programming AFT entries for prefixes in TE_VRF_222
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(3).WithIPAddress(gribiIPv4EntryDefVRF3),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2).AddNextHop(3, 1).WithBackupNHG(2000),
		fluent.IPv4Entry().WithNetworkInstance(niTEVRF222).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF2221+"/"+maskLen32).WithNextHopGroup(2),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(5).WithIPAddress(gribiIPv4EntryDefVRF5),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(4).AddNextHop(5, 1).WithBackupNHG(2000),
		fluent.IPv4Entry().WithNetworkInstance(niTEVRF222).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF2222+"/"+maskLen32).WithNextHopGroup(4),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	teVRF222IPList := []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF2222}
	for ip := range teVRF222IPList {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(teVRF222IPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for backup NHG
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1000).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2221).
			WithNextHopNetworkInstance(niTEVRF222),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1000).AddNextHop(1000, 1).WithBackupNHG(2000),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1001).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2222).
			WithNextHopNetworkInstance(niTEVRF222),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1001).AddNextHop(1001, 1).WithBackupNHG(2000),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(3000).WithNextHopNetworkInstance(niRepairVrf),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(3000).AddNextHop(3000, 1),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	// Programming AFT entries for prefixes in TE_VRF_111
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1).WithIPAddress(gribiIPv4EntryDefVRF1),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2).WithIPAddress(gribiIPv4EntryDefVRF2),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1).AddNextHop(1, 1).AddNextHop(2, 3).WithBackupNHG(3000),
		fluent.IPv4Entry().WithNetworkInstance(niTEVRF111).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1111+"/"+maskLen32).WithNextHopGroup(1),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(4).WithIPAddress(gribiIPv4EntryDefVRF4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(3).AddNextHop(4, 1).WithBackupNHG(3000),
		fluent.IPv4Entry().WithNetworkInstance(niTEVRF111).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1112+"/"+maskLen32).WithNextHopGroup(3),

		fluent.IPv4Entry().WithNetworkInstance(niRepairVrf).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1111+"/"+maskLen32).WithNextHopGroup(1000),
		fluent.IPv4Entry().WithNetworkInstance(niRepairVrf).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1112+"/"+maskLen32).WithNextHopGroup(1001),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	teVRF111IPList := []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}
	for ip := range teVRF111IPList {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(teVRF111IPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for backup NHG
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2001).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2001).AddNextHop(2001, 1),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	// Programming AFT entries for prefixes in ENCAP_TE_VRF_A
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(101).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc111Addr, gribiIPv4EntryVRF1111).
			WithNextHopNetworkInstance(niTEVRF111),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(102).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc111Addr, gribiIPv4EntryVRF1112).
			WithNextHopNetworkInstance(niTEVRF111),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(101).AddNextHop(101, 1).AddNextHop(102, 3).WithBackupNHG(2001),
		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfA).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryEncapVRF+"/"+maskLen24).WithNextHopGroup(101),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, client.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(gribiIPv4EntryEncapVRF+"/24").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// TestCase is a struct to hold the parameters for FRR test cases.
type TestCase struct {
	Desc                   string
	DownPortList           []string
	CapturePortList        []string
	EncapHeaderOuterIPList []string
	EncapHeaderInnerIPList []string
	TrafficDestIP          string
	LoadBalancePercent     []float64
	TestID                 string
}

// TestCases returns a list of base test cases for FRR tests.
func TestCases(atePortNamelist []string, ipv4InnerDst string) []*TestCase {
	cases := []*TestCase{
		{
			Desc:                   "Test-1: primary encap unviable but backup encap viable for single tunnel",
			DownPortList:           []string{"port2", "port3", "port4"},
			CapturePortList:        []string{atePortNamelist[4], atePortNamelist[5]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF1112},
			EncapHeaderInnerIPList: []string{ipv4InnerDst, ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0, 0.25, 0.75, 0, 0},
			TestID:                 "primarySingle",
		}, {
			Desc:                   "Test-2: primary and backup encap unviable for single tunnel",
			DownPortList:           []string{"port2", "port3", "port4", "port5"},
			CapturePortList:        []string{atePortNamelist[5], atePortNamelist[7]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF1112},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0, 0, 0.75, 0, 0.25},
			TestID:                 "primaryBackupSingle",
		}, {
			Desc:                   "Test-3: primary encap unviable with backup to routing for single tunnel",
			DownPortList:           []string{"port2", "port3", "port4"},
			CapturePortList:        []string{atePortNamelist[5], atePortNamelist[7]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF1112},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0, 0, 0.75, 0, 0.25},
			TestID:                 "primaryBackupRoutingSingle",
		}, {
			Desc:                   "Test-4: primary encap unviable but backup encap viable for all tunnels",
			DownPortList:           []string{"port2", "port3", "port4", "port6"},
			CapturePortList:        []string{atePortNamelist[4], atePortNamelist[6]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF2222},
			EncapHeaderInnerIPList: []string{ipv4InnerDst, ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0, 0.25, 0, 0.75, 0},
			TestID:                 "primaryAll",
		}, {
			Desc:                   "Test-5: primary and backup encap unviable for all tunnels",
			DownPortList:           []string{"port2", "port3", "port4", "port5", "port6", "port7"},
			CapturePortList:        []string{atePortNamelist[7]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0, 0, 0, 0, 1},
			TestID:                 "primaryBackupAll",
		}, {
			Desc:                   "Test-6: primary encap unviable with backup to routing for all tunnels",
			DownPortList:           []string{"port2", "port3", "port4", "port6"},
			CapturePortList:        []string{atePortNamelist[7]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0, 0, 0, 0, 1},
			TestID:                 "primaryBackupRoutingAll",
		}, {
			Desc:                   "Test-7: no match in encap VRF",
			DownPortList:           []string{},
			CapturePortList:        []string{atePortNamelist[7]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{noMatchEncapDest},
			TrafficDestIP:          noMatchEncapDest,
			LoadBalancePercent:     []float64{0, 0, 0, 0, 0, 0, 1},
			TestID:                 "encapNoMatch",
		},
	}

	return cases
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, t testing.TB, c *fluent.GRIBIClient, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}
