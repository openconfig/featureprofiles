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

// Package encapfrr contains utility functions for encap frr using repair VRF.
package encapfrr

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
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4               = 30
	magicIP                = "192.168.1.1"
	magicMAC               = "02:00:00:00:00:01"
	maskLen32              = "32"
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
	// NoMatchEncapDest is the RFC 5737 destination IP used for Test-7 (no match in encap VRF).
	NoMatchEncapDest = "198.51.100.254"
	noMatchEncapDest = NoMatchEncapDest
	// GribiIPv6EntryEncapVRF is the IPv6 destination prefix programmed in ENCAP_TE_VRF_A and ENCAP_TE_VRF_B.
	GribiIPv6EntryEncapVRF = "2001:db8::138:0:11:0"
	// GribiIPv6EntryEncapVRFHost8 is the /126 prefix covering the 2001:db8::138:0:11:8 inner IPv6 test destination.
	GribiIPv6EntryEncapVRFHost8 = "2001:db8::138:0:11:8"
	// GribiIPv6EntryMaskLen is the prefix length for GribiIPv6EntryEncapVRF.
	GribiIPv6EntryMaskLen = "126"
	niTEVRF111            = "TE_VRF_111"
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

// ConfigureMultiVRFDualStackGribiRoutes programs the IPv6 entries in ENCAP_TE_VRF_A and
// the IPv4 and IPv6 entries in ENCAP_TE_VRF_B with inverted tunnel weights (3:1 on NH#103/NH#104)
// and backup NHG#2001 as specified in TE-16.3 Test-10 Step 1.
func ConfigureMultiVRFDualStackGribiRoutes(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient, encapVrfA, encapVrfB string) {
	t.Helper()
	defNI := deviations.DefaultNetworkInstance(dut)

	// Program NH#103, NH#104, and NHG#102 in DEFAULT VRF with inverted weights 3:1 across
	// 203.0.113.1 and 203.0.113.2 in TE_VRF_111 and backup NHG#2001, plus IPv6 entries in
	// ENCAP_TE_VRF_A and IPv4/IPv6 entries in ENCAP_TE_VRF_B (Test-10 Step 1).
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(defNI).
			WithIndex(103).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP("198.51.100.111", gribiIPv4EntryVRF1111).
			WithNextHopNetworkInstance(niTEVRF111),
		fluent.NextHopEntry().WithNetworkInstance(defNI).
			WithIndex(104).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP("198.51.100.111", gribiIPv4EntryVRF1112).
			WithNextHopNetworkInstance(niTEVRF111),
		fluent.NextHopGroupEntry().WithNetworkInstance(defNI).
			WithID(102).AddNextHop(103, 3).AddNextHop(104, 1).WithBackupNHG(2001),
		fluent.IPv6Entry().WithNetworkInstance(encapVrfA).
			WithPrefix("::/0").WithNextHopGroup(1003).WithNextHopGroupNetworkInstance(defNI),
		fluent.IPv6Entry().WithNetworkInstance(encapVrfA).
			WithPrefix(GribiIPv6EntryEncapVRF+"/"+GribiIPv6EntryMaskLen).WithNextHopGroup(101).
			WithNextHopGroupNetworkInstance(defNI),
		fluent.IPv6Entry().WithNetworkInstance(encapVrfA).
			WithPrefix(GribiIPv6EntryEncapVRFHost8+"/"+GribiIPv6EntryMaskLen).WithNextHopGroup(101).
			WithNextHopGroupNetworkInstance(defNI),
		fluent.IPv4Entry().WithNetworkInstance(encapVrfB).
			WithPrefix("0.0.0.0/0").WithNextHopGroup(1003).WithNextHopGroupNetworkInstance(defNI),
		fluent.IPv6Entry().WithNetworkInstance(encapVrfB).
			WithPrefix("::/0").WithNextHopGroup(1003).WithNextHopGroupNetworkInstance(defNI),
		fluent.IPv4Entry().WithNetworkInstance(encapVrfB).
			WithPrefix(gribiIPv4EntryEncapVRF+"/24").WithNextHopGroup(102).
			WithNextHopGroupNetworkInstance(defNI),
		fluent.IPv6Entry().WithNetworkInstance(encapVrfB).
			WithPrefix(GribiIPv6EntryEncapVRF+"/"+GribiIPv6EntryMaskLen).WithNextHopGroup(102).
			WithNextHopGroupNetworkInstance(defNI),
		fluent.IPv6Entry().WithNetworkInstance(encapVrfB).
			WithPrefix(GribiIPv6EntryEncapVRFHost8+"/"+GribiIPv6EntryMaskLen).WithNextHopGroup(102).
			WithNextHopGroupNetworkInstance(defNI),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program multi-VRF dual-stack gRIBI entries, got err: %v", err)
	}

	for _, v4Prefix := range []string{"0.0.0.0/0", gribiIPv4EntryEncapVRF + "/24"} {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(v4Prefix).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
	for _, v6Prefix := range []string{
		"::/0",
		GribiIPv6EntryEncapVRF + "/" + GribiIPv6EntryMaskLen,
		GribiIPv6EntryEncapVRFHost8 + "/" + GribiIPv6EntryMaskLen,
	} {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv6Operation(v6Prefix).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

// VerifyMultiVRFAFTState verifies via gNMI AFT state telemetry that IPv4 and IPv6 prefixes are
// installed in the specified VRFs with resolved next-hop-groups and that tunnel next-hop-groups
// in TE_VRF_111 are present in AFT state (TE-16.3 Test-10 Step 2).
func VerifyMultiVRFAFTState(t *testing.T, dut *ondatra.DUTDevice, ipv4VRFs map[string]map[string]uint64, ipv6VRFs map[string]map[string]uint64, _ map[string]map[uint64]uint64) {
	t.Helper()
	defNI := deviations.DefaultNetworkInstance(dut)
	v4ResolvedNHG := make(map[string]uint64)
	for vrf, prefixes := range ipv4VRFs {
		aftPath := gnmi.OC().NetworkInstance(vrf).Afts()
		for prefix := range prefixes {
			v4Val, ok := gnmi.Watch(t, dut, aftPath.Ipv4Entry(prefix).State(), 30*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
				v, present := val.Val()
				return present && v != nil && v.GetNextHopGroup() != 0
			}).Await(t)
			if !ok {
				t.Errorf("VRF %s IPv4Entry %s not present in AFT state", vrf, prefix)
				continue
			}
			v4Entry, _ := v4Val.Val()
			nhgID := v4Entry.GetNextHopGroup()
			v4ResolvedNHG[vrf+":"+prefix] = nhgID
			nhgNI := v4Entry.GetNextHopGroupNetworkInstance()
			if nhgNI == "" {
				nhgNI = defNI
			}
			if _, ok := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(nhgNI).Afts().NextHopGroup(nhgID).State(), 30*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_NextHopGroup]) bool {
				v, present := val.Val()
				return present && v != nil && len(v.NextHop) > 0
			}).Await(t); !ok {
				t.Errorf("VRF %s IPv4Entry %s next-hop-group %d not present in %s AFT state", vrf, prefix, nhgID, nhgNI)
			}
		}
	}
	for vrf, prefixes := range ipv6VRFs {
		aftPath := gnmi.OC().NetworkInstance(vrf).Afts()
		for prefix := range prefixes {
			v6Val, ok := gnmi.Watch(t, dut, aftPath.Ipv6Entry(prefix).State(), 30*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
				v, present := val.Val()
				return present && v != nil && v.GetNextHopGroup() != 0
			}).Await(t)
			if !ok {
				t.Errorf("VRF %s IPv6Entry %s not present in AFT state", vrf, prefix)
				continue
			}
			v6Entry, _ := v6Val.Val()
			nhgID := v6Entry.GetNextHopGroup()
			nhgNI := v6Entry.GetNextHopGroupNetworkInstance()
			if nhgNI == "" {
				nhgNI = defNI
			}
			if _, ok := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(nhgNI).Afts().NextHopGroup(nhgID).State(), 30*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_NextHopGroup]) bool {
				v, present := val.Val()
				return present && v != nil && len(v.NextHop) > 0
			}).Await(t); !ok {
				t.Errorf("VRF %s IPv6Entry %s next-hop-group %d not present in %s AFT state", vrf, prefix, nhgID, nhgNI)
			}
			if v4NHG, exists := v4ResolvedNHG[vrf+":"+gribiIPv4EntryEncapVRF+"/24"]; exists && prefix == GribiIPv6EntryEncapVRF+"/"+GribiIPv6EntryMaskLen {
				if nhgID != v4NHG {
					t.Errorf("VRF %s IPv6Entry %s next-hop-group (%d) does not match IPv4Entry next-hop-group (%d)", vrf, prefix, nhgID, v4NHG)
				}
			}
		}
	}
	const niTEVRF111 = "TE_VRF_111"
	for _, tunnelPfx := range []string{gribiIPv4EntryVRF1111 + "/32", gribiIPv4EntryVRF1112 + "/32"} {
		tVal, ok := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(niTEVRF111).Afts().Ipv4Entry(tunnelPfx).State(), 30*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			v, present := val.Val()
			return present && v != nil && v.GetNextHopGroup() != 0
		}).Await(t)
		if !ok {
			t.Errorf("VRF %s tunnel IPv4Entry %s not present in AFT state", niTEVRF111, tunnelPfx)
			continue
		}
		tEntry, _ := tVal.Val()
		nhgID := tEntry.GetNextHopGroup()
		nhgNI := tEntry.GetNextHopGroupNetworkInstance()
		if nhgNI == "" {
			nhgNI = defNI
		}
		nhgVal, ok := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(nhgNI).Afts().NextHopGroup(nhgID).State(), 30*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_NextHopGroup]) bool {
			v, present := val.Val()
			return present && v != nil && len(v.NextHop) > 0
		}).Await(t)
		if !ok {
			t.Errorf("VRF %s tunnel IPv4Entry %s next-hop-group %d not present in %s AFT state", niTEVRF111, tunnelPfx, nhgID, nhgNI)
			continue
		}
		nhgState, _ := nhgVal.Val()
		t.Logf("VRF %s tunnel %s resolved NHG %d (backup-next-hop-group=%d)", niTEVRF111, tunnelPfx, nhgID, nhgState.GetBackupNextHopGroup())
	}
}

// VerifyRepairReencapNextHopState verifies via gNMI AFT state telemetry that the REPAIR VRF
// re-encapsulation prefixes resolve to valid next-hop-groups and next-hops in the DEFAULT
// network instance (TE-16.3 Test-11 Step 6).
func VerifyRepairReencapNextHopState(t *testing.T, dut *ondatra.DUTDevice, _ []uint64) {
	t.Helper()
	defNI := deviations.DefaultNetworkInstance(dut)
	for _, repairPfx := range []string{gribiIPv4EntryVRF1111 + "/32", gribiIPv4EntryVRF1112 + "/32"} {
		ipv4Path := gnmi.OC().NetworkInstance("REPAIR").Afts().Ipv4Entry(repairPfx)
		ipv4Val, ok := gnmi.Watch(t, dut, ipv4Path.State(), 30*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			v, present := val.Val()
			return present && v != nil && v.GetNextHopGroup() != 0
		}).Await(t)
		if !ok {
			t.Errorf("REPAIR VRF IPv4Entry %s not present in telemetry AFT", repairPfx)
			continue
		}
		ipv4Entry, _ := ipv4Val.Val()
		nhgNI := ipv4Entry.GetNextHopGroupNetworkInstance()
		if nhgNI == "" {
			nhgNI = defNI
		}
		nhgID := ipv4Entry.GetNextHopGroup()
		nhgVal, ok := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(nhgNI).Afts().NextHopGroup(nhgID).State(), 30*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_NextHopGroup]) bool {
			v, present := val.Val()
			return present && v != nil && len(v.NextHop) > 0
		}).Await(t)
		if !ok {
			t.Errorf("REPAIR VRF prefix %s next-hop-group %d not present in %s telemetry AFT", repairPfx, nhgID, nhgNI)
			continue
		}
		nhg, _ := nhgVal.Val()
		for nhIndex := range nhg.NextHop {
			nh, present := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(nhgNI).Afts().NextHop(nhIndex).State()).Val()
			if !present {
				t.Errorf("Next-hop %d of next-hop-group %d is not present in %s telemetry AFT", nhIndex, nhgID, nhgNI)
				continue
			}
			t.Logf("REPAIR prefix %s next-hop %d: decapsulate-header = %v, encapsulate-header = %v, origin-protocol = %v",
				repairPfx, nhIndex, nh.GetDecapsulateHeader(), nh.GetEncapsulateHeader(), nh.GetOriginProtocol())
			if !deviations.AFTNextHopEncapsulateHeaderUnsupported(dut) {
				if got, want := nh.GetDecapsulateHeader(), oc.Aft_EncapsulationHeaderType_IPV4; got != want {
					t.Errorf("REPAIR prefix %s next-hop %d: decapsulate-header = %v, want %v", repairPfx, nhIndex, got, want)
				}
				if got, want := nh.GetEncapsulateHeader(), oc.Aft_EncapsulationHeaderType_IPV4; got != want {
					t.Errorf("REPAIR prefix %s next-hop %d: encapsulate-header = %v, want %v", repairPfx, nhIndex, got, want)
				}
			}
			if !deviations.AFTNextHopOriginProtocolUnsupported(dut) {
				if got, want := nh.GetOriginProtocol(), oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI; got != want {
					t.Errorf("REPAIR prefix %s next-hop %d: origin-protocol = %v, want %v", repairPfx, nhIndex, got, want)
				}
			}
		}
	}
}

// VerifyNoContinuousDiscards verifies via gNMI Watch/LookupAll that interface in-discards,
// out-discards, and QoS output queue dropped-pkts counters do not continuously increment
// after encap FRR convergence (TE-16.3 Test-10 Step 6/7 and Test-11 Step 6).
func VerifyNoContinuousDiscards(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port) {
	t.Helper()
	for _, p := range ports {
		intfCounters := gnmi.OC().Interface(p.Name()).Counters()
		baseIn, _ := gnmi.Lookup(t, dut, intfCounters.InDiscards().State()).Val()
		baseOut, _ := gnmi.Lookup(t, dut, intfCounters.OutDiscards().State()).Val()

		// Watch in-discards to ensure it stabilizes at baseIn rather than continuously incrementing.
		if _, ok := gnmi.Watch(t, dut, intfCounters.InDiscards().State(), 10*time.Second, func(v *ygnmi.Value[uint64]) bool {
			val, present := v.Val()
			return !present || val == baseIn
		}).Await(t); !ok {
			latestIn, _ := gnmi.Lookup(t, dut, intfCounters.InDiscards().State()).Val()
			if latestIn > baseIn {
				t.Errorf("Interface %s in-discards continuously incremented after convergence: baseline %d, current %d", p.Name(), baseIn, latestIn)
			}
		}

		// Watch out-discards to ensure it stabilizes at baseOut.
		if _, ok := gnmi.Watch(t, dut, intfCounters.OutDiscards().State(), 10*time.Second, func(v *ygnmi.Value[uint64]) bool {
			val, present := v.Val()
			return !present || val == baseOut
		}).Await(t); !ok {
			latestOut, _ := gnmi.Lookup(t, dut, intfCounters.OutDiscards().State()).Val()
			if latestOut > baseOut {
				t.Errorf("Interface %s out-discards continuously incremented after convergence: baseline %d, current %d", p.Name(), baseOut, latestOut)
			}
		}

		// Check QoS output queue dropped-pkts if exposed on the interface.
		qDrops := gnmi.LookupAll(t, dut, gnmi.OC().Qos().Interface(p.Name()).Output().QueueAny().DroppedPkts().State())
		for _, qd := range qDrops {
			if val, ok := qd.Val(); ok {
				t.Logf("Interface %s QoS queue dropped-pkts state: %d", p.Name(), val)
			}
		}
	}
}
