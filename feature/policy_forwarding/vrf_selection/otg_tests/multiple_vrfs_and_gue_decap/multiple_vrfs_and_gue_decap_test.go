// Copyright 2024 Google LLC
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

package multiple_vrfs_and_gue_decap_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen     = 30
	ipv6PrefixLen     = 126
	ate1Asn           = 65001
	ate2Asn           = 65003
	dutAsn            = 65001
	ipv4Src           = "198.51.100.1"
	ipv4Dst           = "198.51.200.1"
	ipv6Src           = "2001:DB8:1::1"
	ipv6Dst           = "2001:DB8:2::1"
	peerv4Grp1Name    = "BGP-PEER-GROUP1-V4"
	peerv6Grp1Name    = "BGP-PEER-GROUP1-V6"
	peerv4Grp2Name    = "BGP-PEER-GROUP2-V4"
	peerv6Grp2Name    = "BGP-PEER-GROUP2-V6"
	v4NetName1        = "BGPv4RR1"
	v6NetName1        = "BGPv6RR1"
	v4NetName2        = "BGPv4RR2"
	v6NetName2        = "BGPv6RR2"
	packetPerSecond   = 100
	guePort           = 6080
	trafficSleepTime  = 10
	captureWait       = 10
	nonDefaultVrfName = "B2_VRF"
	packetSize        = 512
	IPv4Prefix1       = "198.51.100.1"
	IPv4Prefix2       = "198.51.100.2"
	IPv4Prefix3       = "198.51.100.3"
	IPv4Prefix4       = "198.51.100.4"
	IPv4Prefix5       = "198.51.100.5"
	IPv4Prefix6       = "198.51.200.1"
	IPv4Prefix7       = "198.51.200.2"
	IPv4Prefix8       = "198.51.200.3"
	IPv4Prefix9       = "198.51.200.4"
	IPv4Prefix10      = "198.51.200.5"
	IPv6Prefix1       = "2001:DB8:1::1"
	IPv6Prefix2       = "2001:DB8:1::2"
	IPv6Prefix3       = "2001:DB8:1::3"
	IPv6Prefix4       = "2001:DB8:1::4"
	IPv6Prefix5       = "2001:DB8:1::5"
	IPv6Prefix6       = "2001:DB8:2::1"
	IPv6Prefix7       = "2001:DB8:2::2"
	IPv6Prefix8       = "2001:DB8:2::3"
	IPv6Prefix9       = "2001:DB8:2::4"
	IPv6Prefix10      = "2001:DB8:2::5"
	policyName        = "decap-policy-gue"
)

var (
	atePort1 = attrs.Attributes{
		Name:    "ateP1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "ateP2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort1 = &attrs.Attributes{
		Desc:    "dutPort1",
		IPv6:    "2001:db8::1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = &attrs.Attributes{
		Desc:    "dutPort2",
		IPv6:    "2001:db8::5",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	name          string
	flownames     []string
	ipv4SrcIp     string
	ipv6SrcIp     string
	dscpValue     int
	verifyCapture bool
}

func TestMultipleVrfsAndGueDecap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	t.Log(dp1, dp2)

	// Configure DUT interfaces.
	createVRF(t, dut)
	ConfigureDUTIntf(t, dut)
	ConfigureBgp(t, dut)
	ConfigureQoS(t, dut)

	configureRouteLeaking(t, dut)
	configureDutWithGueDecap(t, dut, guePort, "ipv4")

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)

	enableCapture(t, topo, "port2")
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	verifyBGPTelemetry(t, dut)
	verifyLeakedRoutes(t, dut)
	configureIPv4Traffic(t, ate, topo)
	startCapture(t, ate)
	trafficStartStop(t, dut, ate, topo)
	stopCapture(t, ate)

	testCases := []testCase{
		{
			name:          "PF-2.3.1: [Baseline] Traffic flow between ATE:Port1 and ATE:Port2 via DUT's Default VRF",
			flownames:     []string{"1to6v4", "2to7v4", "3to8v4", "4to9v4", "5to10v4", "1to6v6", "2to7v6", "3to8v6", "4to9v6", "5to10v6"},
			verifyCapture: false,
		},
		{
			name:          "PF-2.3.2: BE1 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames:     []string{"1to6v4_encapped", "1to6v6_encapped"},
			ipv4SrcIp:     IPv4Prefix1,
			ipv6SrcIp:     IPv6Prefix1,
			dscpValue:     0,
			verifyCapture: true,
		},
		{
			name:          "PF-2.3.3: BE1 and AF1 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames:     []string{"1to6v4_encapped", "2to7v4_encapped", "1to6v6_encapped", "2to7v6_encapped"},
			ipv4SrcIp:     IPv4Prefix2,
			ipv6SrcIp:     IPv6Prefix2,
			dscpValue:     8,
			verifyCapture: true,
		},
		{
			name:          "PF-2.3.4: BE1, AF1 and AF2 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames:     []string{"1to6v4_encapped", "2to7v4_encapped", "3to8v4_encapped", "1to6v6_encapped", "2to7v6_encapped", "3to8v6_encapped"},
			ipv4SrcIp:     IPv4Prefix3,
			ipv6SrcIp:     IPv6Prefix3,
			dscpValue:     16,
			verifyCapture: true,
		},
		{
			name:          "PF-2.3.5: BE1, AF1, AF2 and AF3 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames:     []string{"1to6v4_encapped", "2to7v4_encapped", "3to8v4_encapped", "1to6v6_encapped", "4to9v4_encapped", "2to7v6_encapped", "3to8v6_encapped", "4to9v6_encapped"},
			ipv4SrcIp:     IPv4Prefix4,
			ipv6SrcIp:     IPv6Prefix4,
			dscpValue:     24,
			verifyCapture: true,
		},
		{
			name:          "PF-2.3.6: BE1, AF1, AF2, AF3 and AF4 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames:     []string{"1to6v4_encapped", "2to7v4_encapped", "3to8v4_encapped", "4to9v4_encapped", "5to10v4_encapped", "1to6v6_encapped", "2to7v6_encapped", "3to8v6_encapped", "4to9v6_encapped", "5to10v6_encapped"},
			ipv4SrcIp:     IPv4Prefix5,
			ipv6SrcIp:     IPv6Prefix5,
			dscpValue:     32,
			verifyCapture: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validateTrafficLoss(t, ate, tc.flownames)
			if tc.verifyCapture {
				verifyCapturePackets(t, ate, "port2", "ipv4", tc.ipv4SrcIp)
				verifyCapturePackets(t, ate, "port2", "ipv6", tc.ipv6SrcIp)
			}
		})
	}
}

func configureDutWithGueDecap(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string) {
	t.Logf("Configure DUT with decapsulation UDP port %v", guePort)
	ocPFParams := getDefaultOcPolicyForwardingParams(t, dut, guePort, ipType)
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
}

// getDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func getDefaultOcPolicyForwardingParams(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string) cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         dut.Port(t, "port1").Name(),
		AppliedPolicyName:   policyName,
		TunnelIP:            dutlo0Attrs.IPv4,
		GuePort:             uint32(guePort),
		IpType:              ipType,
		Dynamic:             true,
		DecapProtocol:       "ip",
	}
}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutPort1, dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))

	configureLoopbackInterface(t, dut)

}

type bgpNeighbor struct {
	as            uint32
	neighborip    string
	isV4          bool
	PeerGroupName string
}

func ConfigureBgp(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}

	nbr1v4 := &bgpNeighbor{as: ate1Asn, neighborip: atePort1.IPv4, isV4: true, PeerGroupName: peerv4Grp1Name}
	nbr1v6 := &bgpNeighbor{as: ate1Asn, neighborip: atePort1.IPv6, isV4: false, PeerGroupName: peerv6Grp1Name}
	nbr2v4 := &bgpNeighbor{as: ate2Asn, neighborip: atePort2.IPv4, isV4: true, PeerGroupName: peerv4Grp2Name}
	nbr2v6 := &bgpNeighbor{as: ate2Asn, neighborip: atePort2.IPv6, isV4: false, PeerGroupName: peerv6Grp2Name}

	nbrList := []*bgpNeighbor{nbr1v4, nbr2v4, nbr1v6, nbr2v6}

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAsn)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutPort2.IPv4)

	pg1v4 := bgp.GetOrCreatePeerGroup(peerv4Grp1Name)
	pg1v4.PeerAs = ygot.Uint32(ate1Asn)

	pg1v6 := bgp.GetOrCreatePeerGroup(peerv6Grp1Name)
	pg1v6.PeerAs = ygot.Uint32(ate1Asn)

	pg2v4 := bgp.GetOrCreatePeerGroup(peerv4Grp2Name)
	pg2v4.PeerAs = ygot.Uint32(ate2Asn)

	pg2v6 := bgp.GetOrCreatePeerGroup(peerv6Grp2Name)
	pg2v6.PeerAs = ygot.Uint32(ate2Asn)

	for _, nbr := range nbrList {
		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerGroup = ygot.String(nbr.PeerGroupName)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)
		if nbr.isV4 {
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
		} else {
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
		}
	}
	gnmi.Replace(t, dut, dutConfPath.Config(), niProto)

}

// Configures the given DUT interface.
func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s5 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s5.Enabled = ygot.Bool(true)
	}
	return i
}

func ConfigureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	queues := netutil.CommonTrafficQueues(t, dut)

	if deviations.QOSQueueRequiresID(dut) {
		queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE1}
		for i, queue := range queueNames {
			q1 := q.GetOrCreateQueue(queue)
			q1.Name = ygot.String(queue)
			queueid := len(queueNames) - i
			q1.QueueId = ygot.Uint8(uint8(queueid))
		}
	}
	t.Logf("Create qos forwarding groups config")
	forwardingGroups := []struct {
		desc        string
		queueName   string
		targetGroup string
	}{{
		desc:        "forwarding-group-BE1",
		queueName:   queues.BE1,
		targetGroup: "target-group-BE1",
	}, {
		desc:        "forwarding-group-AF1",
		queueName:   queues.AF1,
		targetGroup: "target-group-AF1",
	}, {
		desc:        "forwarding-group-AF2",
		queueName:   queues.AF2,
		targetGroup: "target-group-AF2",
	}, {
		desc:        "forwarding-group-AF3",
		queueName:   queues.AF3,
		targetGroup: "target-group-AF3",
	}, {
		desc:        "forwarding-group-AF4",
		queueName:   queues.AF4,
		targetGroup: "target-group-AF4",
	},
	}

	t.Logf("qos forwarding groups config: %v", forwardingGroups)
	for _, tc := range forwardingGroups {
		qoscfg.SetForwardingGroup(t, dut, q, tc.targetGroup, tc.queueName)
	}

	t.Logf("Create qos Classifiers config")
	classifiers := []struct {
		desc        string
		name        string
		classType   oc.E_Qos_Classifier_Type
		termID      string
		targetGroup string
		dscpSet     []uint8
	}{{
		desc:        "classifier_ipv4_be1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0},
	}, {
		desc:        "classifier_ipv4_af1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "1",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{8},
	}, {
		desc:        "classifier_ipv4_af2",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "2",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{16},
	}, {
		desc:        "classifier_ipv4_af3",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "3",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{24},
	}, {
		desc:        "classifier_ipv4_af4",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "4",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{32},
	}, {
		desc:        "classifier_ipv6_be1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0},
	}, {
		desc:        "classifier_ipv6_af1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "1",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{8},
	}, {
		desc:        "classifier_ipv6_af2",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "2",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{16},
	}, {
		desc:        "classifier_ipv6_af3",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "3",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{24},
	}, {
		desc:        "classifier_ipv6_af4",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "4",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{32},
	},
	}

	t.Logf("qos Classifiers config: %v", classifiers)
	for _, tc := range classifiers {
		classifier := q.GetOrCreateClassifier(tc.name)
		classifier.SetName(tc.name)
		classifier.SetType(tc.classType)
		term, err := classifier.NewTerm(tc.termID)
		if err != nil {
			t.Fatalf("Failed to create classifier.NewTerm(): %v", err)
		}
		term.SetId(tc.termID)
		action := term.GetOrCreateActions()
		action.SetTargetGroup(tc.targetGroup)

		// remark := action.GetOrCreateRemark()
		// remark.SetDscp = ygot.Uint8(0)

		condition := term.GetOrCreateConditions()
		if tc.name == "dscp_based_classifier_ipv4" {
			condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
		} else {
			condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
		}
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	}

	t.Logf("Create qos input classifier config")
	classifierIntfs := []struct {
		desc                string
		intf                string
		inputClassifierType oc.E_Input_Classifier_Type
		classifier          string
	}{{
		desc:                "Input Classifier Type IPV4",
		intf:                dp1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4",
	}, {
		desc:                "Input Classifier Type IPV6",
		intf:                dp1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
	}}
	t.Logf("qos input classifier config: %v", classifierIntfs)
	for _, tc := range classifierIntfs {
		qoscfg.SetInputClassifier(t, dut, q, tc.intf, tc.inputClassifierType, tc.classifier)
	}
}

// configureATE sets up the ATE interfaces and BGP configurations.
func configureATE(t *testing.T) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Log("Configure ATE interface")
	port1 := topo.Ports().Add().SetName("port1")
	port2 := topo.Ports().Add().SetName("port2")

	port1Dev := topo.Devices().Add().SetName(atePort1.Name + ".dev")
	port1Eth := port1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	port1Eth.Connection().SetPortName(port1.Name())
	port1Ipv4 := port1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	port1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	port1Ipv6 := port1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	port1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	bgp1 := port1Dev.Bgp().SetRouterId(atePort1.IPv4)
	bgp4Peer1 := bgp1.Ipv4Interfaces().Add().SetIpv4Name(port1Ipv4.Name()).Peers().Add().SetName(port1Dev.Name() + ".BGP4.peer")
	bgp4Peer1.SetPeerAddress(port1Ipv4.Gateway())
	bgp4Peer1.SetAsNumber(ate1Asn)
	bgp4Peer1.SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	net1v4 := bgp4Peer1.V4Routes().Add().SetName(v4NetName1)
	net1v4.Addresses().Add().SetAddress(ipv4Src).SetPrefix(32).SetCount(5).SetStep(1)

	bgp6Peer1 := bgp1.Ipv6Interfaces().Add().SetIpv6Name(port1Ipv6.Name()).Peers().Add().SetName(port1Dev.Name() + ".BGP6.peer")
	bgp6Peer1.SetPeerAddress(port1Ipv6.Gateway())
	bgp6Peer1.SetAsNumber(ate1Asn)
	bgp6Peer1.SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	net1v6 := bgp6Peer1.V6Routes().Add().SetName(v6NetName1)
	net1v6.Addresses().Add().SetAddress(ipv6Src).SetPrefix(128).SetCount(5).SetStep(1)

	port2Dev := topo.Devices().Add().SetName(atePort2.Name + ".dev")
	port2Eth := port2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	port2Eth.Connection().SetPortName(port2.Name())
	port2Ipv4 := port2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	port2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	port2Ipv6 := port2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	port2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	bgp2 := port2Dev.Bgp().SetRouterId(atePort2.IPv4)
	bgp4Peer2 := bgp2.Ipv4Interfaces().Add().SetIpv4Name(port2Ipv4.Name()).Peers().Add().SetName(port2Dev.Name() + ".BGP4.peer")
	bgp4Peer2.SetPeerAddress(port2Ipv4.Gateway())
	bgp4Peer2.SetAsNumber(ate2Asn)
	bgp4Peer2.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	net2v4 := bgp4Peer2.V4Routes().Add().SetName(v4NetName2)
	net2v4.Addresses().Add().SetAddress(ipv4Dst).SetPrefix(32).SetCount(5).SetStep(1)

	bgp6Peer2 := bgp2.Ipv6Interfaces().Add().SetIpv6Name(port2Ipv6.Name()).Peers().Add().SetName(port2Dev.Name() + ".BGP6.peer")
	bgp6Peer2.SetPeerAddress(port2Ipv6.Gateway())
	bgp6Peer2.SetAsNumber(ate2Asn)
	bgp6Peer2.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	net2v6 := bgp6Peer2.V6Routes().Add().SetName(v6NetName2)
	net2v6.Addresses().Add().SetAddress(ipv6Dst).SetPrefix(128).SetCount(5).SetStep(1)

	return topo
}

func trafficStartStop(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config) {
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), config, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), config, "IPv6")
	verifyBGPTelemetry(t, dut)
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficSleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), config)
}

func validateTrafficLoss(t *testing.T, ate *ondatra.ATEDevice, flowName []string) {
	for _, flow := range flowName {
		outPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow).Counters().InPkts().State()))
		t.Logf("Flow %s: outPkts: %v, inPkts: %v", flow, outPkts, inPkts)
		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow)
		}
		if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
			t.Fatalf("LossPct for flow %s: got %v, want 0", flow, got)
		}
	}
}

// startCapture starts the capture on the otg ports
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// stopCapture starts the capture on the otg ports
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

func enableCapture(t *testing.T, config gosnappi.Config, port string) {

	config.Captures().Clear()
	t.Log("Enabling capture on ", port)
	config.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)

}

func processCapture(t *testing.T, ate *ondatra.ATEDevice, port string) string {
	otg := ate.OTG()
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(captureWait * time.Second)
	pcapFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	defer pcapFile.Close()
	return pcapFile.Name()
}

func verifyCapturePackets(t *testing.T, ate *ondatra.ATEDevice, port string, ipType string, srcIP string) {
	var packetCount uint32 = 0
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		switch ipType {
		case "ipv4":
			if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
				ip, _ := ipLayer.(*layers.IPv4)
				if ip.SrcIP.Equal(net.ParseIP(srcIP)) {
					udpLayer := packet.Layer(layers.LayerTypeUDP)
					udp, ok := udpLayer.(*layers.UDP)
					if ok || udpLayer != nil {
						if udp.DstPort == 15 {
							packetCount += 1
						}
					}
				}
			}

		case "ipv6":
			if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
				ip, _ := ipLayer.(*layers.IPv6)
				if ip.SrcIP.Equal(net.ParseIP(srcIP)) {
					udpLayer := packet.Layer(layers.LayerTypeUDP)
					udp, ok := udpLayer.(*layers.UDP)
					if ok || udpLayer != nil {
						if udp.DstPort == 15 {
							packetCount += 1
						}
					}
				}
			}
		}
	}

	if packetCount != 2*packetPerSecond {
		t.Fatalf("Packet count is %v and packetPerSecond count is %v", packetCount, 2*packetPerSecond)
		t.Fatalf("No packet found with the decapsulated IP address of %s", srcIP)
	}
}

func configureRouteLeaking(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.NetworkInstanceImportExportPolicyOCUnsupported(dut) {
		t.Logf("Configuring route leaking through CLI")
		configureRouteLeakingFromCLI(t, dut)
	} else {
		t.Logf("Configuring route leaking through OC")
		configureRouteLeakingFromOC(t, dut)
	}
}

func configureRouteLeakingFromCLI(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	cli := fmt.Sprintf(`
	   route-map RM-ALL-ROUTES permit 10
	   router general
	   vrf %s
	      leak routes source-vrf default subscribe-policy RM-ALL-ROUTES
	`, nonDefaultVrfName)
	helpers.GnmiCLIConfig(t, dut, cli)
}

func configureRouteLeakingFromOC(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	root := &oc.Root{}

	ni1 := root.GetOrCreateNetworkInstance(nonDefaultVrfName)
	ni1Pol := ni1.GetOrCreateInterInstancePolicies()
	iexp1 := ni1Pol.GetOrCreateImportExportPolicy()
	iexp1.SetImportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ImportRouteTarget_Union{oc.UnionString("default")})
	iexp1.SetExportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ExportRouteTarget_Union{oc.UnionString("default")})
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(nonDefaultVrfName).InterInstancePolicies().Config(), ni1Pol)
}

func waitForBGPSession(t *testing.T, dut *ondatra.DUTDevice, wantEstablished bool) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	compare := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if ok {
			if wantEstablished {
				t.Logf("BGP session state: %s", state.String())
				return state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}
			return state == oc.Bgp_Neighbor_SessionState_IDLE
		}
		return false
	}

	nbrListv4 := []string{atePort1.IPv4, atePort2.IPv4}

	for _, nbr := range nbrListv4 {
		nbrPath := statePath.Neighbor(nbr)
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, compare).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			if wantEstablished {
				t.Fatal("No BGP neighbor formed...")
			} else {
				t.Fatal("BGPv4 session didn't teardown.")
			}
		}
	}
}

func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Waiting for BGPv4 neighbor to establish...")
	waitForBGPSession(t, dut, true)
}

func createVRF(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	droot := &oc.Root{}

	// DEFAULT NI
	ni := droot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE

	sb := &gnmi.SetBatch{}
	gnmi.BatchUpdate(sb, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni)

	// VRF
	ni1 := droot.GetOrCreateNetworkInstance(nonDefaultVrfName)
	ni1.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	gnmi.BatchReplace(sb, gnmi.OC().NetworkInstance(nonDefaultVrfName).Config(), ni1)

	sb.Set(t, dut)
}

func configureLoopbackInterface(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 0)
	dutlo0Attrs.Name = loopbackIntfName
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, dc.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutlo0Attrs.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutlo0Attrs.IPv6)
	}
}

func configureIPv4Traffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config) {

	topo.Flows().Clear()
	trafficFlows := []struct {
		desc     string
		dscpSet  []uint32
		ipType   string
		srcIp    string
		dstIp    string
		priority int
	}{{desc: "1to6v4_encapped", dscpSet: []uint32{0}, ipType: "ipv4oipv4", srcIp: IPv4Prefix1, dstIp: IPv4Prefix6, priority: 0},
		{desc: "2to7v4_encapped", dscpSet: []uint32{8}, ipType: "ipv4oipv4", srcIp: IPv4Prefix2, dstIp: IPv4Prefix7, priority: 8},
		{desc: "3to8v4_encapped", dscpSet: []uint32{16}, ipType: "ipv4oipv4", srcIp: IPv4Prefix3, dstIp: IPv4Prefix8, priority: 16},
		{desc: "4to9v4_encapped", dscpSet: []uint32{24}, ipType: "ipv4oipv4", srcIp: IPv4Prefix4, dstIp: IPv4Prefix9, priority: 24},
		{desc: "5to10v4_encapped", dscpSet: []uint32{32}, ipType: "ipv4oipv4", srcIp: IPv4Prefix5, dstIp: IPv4Prefix10, priority: 32},
		{desc: "1to6v6_encapped", dscpSet: []uint32{0}, ipType: "ipv6oipv4", srcIp: IPv6Prefix1, dstIp: IPv6Prefix6, priority: 0},
		{desc: "2to7v6_encapped", dscpSet: []uint32{8}, ipType: "ipv6oipv4", srcIp: IPv6Prefix2, dstIp: IPv6Prefix7, priority: 8},
		{desc: "3to8v6_encapped", dscpSet: []uint32{16}, ipType: "ipv6oipv4", srcIp: IPv6Prefix3, dstIp: IPv6Prefix8, priority: 16},
		{desc: "4to9v6_encapped", dscpSet: []uint32{24}, ipType: "ipv6oipv4", srcIp: IPv6Prefix4, dstIp: IPv6Prefix9, priority: 24},
		{desc: "5to10v6_encapped", dscpSet: []uint32{32}, ipType: "ipv6oipv4", srcIp: IPv6Prefix5, dstIp: IPv6Prefix10, priority: 32},
		{desc: "1to6v4", dscpSet: []uint32{0}, ipType: "ipv4", srcIp: IPv4Prefix1, dstIp: IPv4Prefix6, priority: 0},
		{desc: "2to7v4", dscpSet: []uint32{8}, ipType: "ipv4", srcIp: IPv4Prefix2, dstIp: IPv4Prefix7, priority: 8},
		{desc: "3to8v4", dscpSet: []uint32{16}, ipType: "ipv4", srcIp: IPv4Prefix3, dstIp: IPv4Prefix8, priority: 16},
		{desc: "4to9v4", dscpSet: []uint32{24}, ipType: "ipv4", srcIp: IPv4Prefix4, dstIp: IPv4Prefix9, priority: 24},
		{desc: "5to10v4", dscpSet: []uint32{32}, ipType: "ipv4", srcIp: IPv4Prefix5, dstIp: IPv4Prefix10, priority: 32},
		{desc: "1to6v6", dscpSet: []uint32{0}, ipType: "ipv6", srcIp: IPv6Prefix1, dstIp: IPv6Prefix6, priority: 0},
		{desc: "2to7v6", dscpSet: []uint32{8}, ipType: "ipv6", srcIp: IPv6Prefix2, dstIp: IPv6Prefix7, priority: 8},
		{desc: "3to8v6", dscpSet: []uint32{16}, ipType: "ipv6", srcIp: IPv6Prefix3, dstIp: IPv6Prefix8, priority: 16},
		{desc: "4to9v6", dscpSet: []uint32{24}, ipType: "ipv6", srcIp: IPv6Prefix4, dstIp: IPv6Prefix9, priority: 24},
		{desc: "5to10v6", dscpSet: []uint32{32}, ipType: "ipv6", srcIp: IPv6Prefix5, dstIp: IPv6Prefix10, priority: 32}}

	t.Logf("Traffic config: %v", trafficFlows)
	for _, tc := range trafficFlows {
		trafficID := tc.desc
		t.Logf("Configure Traffic from ATE with flowname %s", trafficID)
		flow := topo.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{v4NetName1}).SetRxNames([]string{v4NetName2})
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(atePort1.MAC)
		ethHeader.Dst().Auto()
		switch tc.ipType {
		case "ipv4oipv4":
			outerIpHeader := flow.Packet().Add().Ipv4()
			outerIpHeader.Src().SetValue(atePort1.IPv4)
			outerIpHeader.Dst().SetValue(dutlo0Attrs.IPv4)
			outerIpHeader.Priority().Dscp().Phb().SetValue(uint32(tc.priority))
			outerudpHeader := flow.Packet().Add().Udp()
			outerudpHeader.SrcPort().SetValue(5996)
			outerudpHeader.DstPort().SetValue(uint32(guePort))
			innerIpHeader := flow.Packet().Add().Ipv4()
			innerIpHeader.Src().SetValue(tc.srcIp)
			innerIpHeader.Dst().SetValue(tc.dstIp)
			innerIpHeader.Priority().Dscp().Phb().SetValue(uint32(tc.priority))
			innerudpHeader := flow.Packet().Add().Udp()
			innerudpHeader.SrcPort().SetValue(14)
			innerudpHeader.DstPort().SetValue(uint32(15))
		case "ipv6oipv4":
			outerIpHeader := flow.Packet().Add().Ipv4()
			outerIpHeader.Src().SetValue(atePort1.IPv4)
			outerIpHeader.Dst().SetValue(dutlo0Attrs.IPv4)
			outerIpHeader.Priority().Dscp().Phb().SetValue(uint32(tc.priority))
			outerudpHeader := flow.Packet().Add().Udp()
			outerudpHeader.SrcPort().SetValue(5996)
			outerudpHeader.DstPort().SetValue(uint32(guePort))
			innerIpHeader := flow.Packet().Add().Ipv6()
			innerIpHeader.Src().SetValue(tc.srcIp)
			innerIpHeader.Dst().SetValue(tc.dstIp)
			innerIpHeader.TrafficClass().SetValue(uint32(tc.priority))
			innerudpHeader := flow.Packet().Add().Udp()
			innerudpHeader.SrcPort().SetValue(14)
			innerudpHeader.DstPort().SetValue(uint32(15))
		case "ipv4":
			ipHeader := flow.Packet().Add().Ipv4()
			ipHeader.Src().SetValue(tc.srcIp)
			ipHeader.Dst().SetValue(tc.dstIp)
			ipHeader.Priority().Dscp().Phb().SetValue(uint32(tc.priority))
			udpHeader := flow.Packet().Add().Udp()
			udpHeader.SrcPort().SetValue(14)
			udpHeader.DstPort().SetValue(uint32(15))
		case "ipv6":
			ipHeader := flow.Packet().Add().Ipv6()
			ipHeader.Src().SetValue(tc.srcIp)
			ipHeader.Dst().SetValue(tc.dstIp)
			ipHeader.TrafficClass().SetValue(uint32(tc.priority))
			udpHeader := flow.Packet().Add().Udp()
			udpHeader.SrcPort().SetValue(14)
			udpHeader.DstPort().SetValue(uint32(15))
		}
		flow.Size().SetFixed(uint32(packetSize))
		flow.Rate().SetPps(packetPerSecond)
		flow.Duration().FixedPackets().SetPackets(packetPerSecond)
	}
	ate.OTG().PushConfig(t, topo)
}

func verifyLeakedRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var routes []string
	routes = []string{IPv4Prefix1 + "/32", IPv4Prefix2 + "/32", IPv4Prefix3 + "/32", IPv4Prefix4 + "/32", IPv4Prefix5 + "/32", IPv4Prefix6 + "/32", IPv4Prefix7 + "/32", IPv4Prefix8 + "/32", IPv4Prefix9 + "/32", IPv4Prefix10 + "/32"}
	for _, advroute := range routes {
		t.Logf("Verifying leaked route %s in %s", advroute, nonDefaultVrfName)
		aftVrf2 := gnmi.OC().NetworkInstance(nonDefaultVrfName).Afts().Ipv4Entry(advroute)
		_, ok := gnmi.Watch(t, dut, aftVrf2.State(), 15*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			return val.IsPresent()
		}).Await(t)
		if !ok {
			t.Errorf("Route %s was not leaked into %s unexpectedly", advroute, nonDefaultVrfName)
		} else {
			t.Logf("Route %s was successfully leaked into %s as expected", advroute, nonDefaultVrfName)
		}
	}
}
