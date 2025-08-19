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

package egress_traffic_classification_and_rewrite_test

import (
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
	ipv4PrefixLen    = 30
	ipv6PrefixLen    = 126
	ate1Asn          = 65002
	ate2Asn          = 65003
	dutAsn           = 65001
	ipv4Src          = "198.51.100.1"
	ipv4Dst          = "203.0.113.1"
	ipv6Src          = "2001:DB8:1::1"
	ipv6Dst          = "2001:DB8:2::1"
	peerv4Grp1Name   = "BGP-PEER-GROUP1-V4"
	peerv6Grp1Name   = "BGP-PEER-GROUP1-V6"
	peerv4Grp2Name   = "BGP-PEER-GROUP2-V4"
	peerv6Grp2Name   = "BGP-PEER-GROUP2-V6"
	v4NetName1       = "BGPv4RR1"
	v6NetName1       = "BGPv6RR1"
	v4NetName2       = "BGPv4RR2"
	v6NetName2       = "BGPv6RR2"
	frameSize        = 512
	packetPerSecond  = 100
	guePort          = 6080
	trafficSleepTime = 10
	captureWait      = 10
	mplsPopLabelv4   = 10020
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
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	name       string
	ipType     string
	enableMpls bool
	enableGre  bool
	enableGue  bool
}

func TestIngressTrafficClassificationAndRewrite(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	t.Log(dp1, dp2)

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)
	ConfigureBgp(t, dut)
	ConfigureQoS(t, dut)

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)
	enableCapture(t, topo, "port2")

	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	verifyBGPTelemetry(t, dut)

	testCases := []testCase{
		{
			name:       "DP-1.17.1 Egress Classification and rewrite of IPv4 packets with various DSCP values",
			ipType:     "ipv4",
			enableMpls: false,
			enableGre:  false,
			enableGue:  false,
		},
		{
			name:       "DP-1.17.2 Egress Classification and rewrite of IPv6 packets with various TC values",
			ipType:     "ipv6",
			enableMpls: false,
			enableGre:  false,
			enableGue:  false,
		},
		{
			name:       "DP-1.17.3 Egress Classification and rewrite of IPoMPLSoGUE traffic with pop action",
			ipType:     "ipv4",
			enableMpls: true,
			enableGre:  false,
			enableGue:  true,
		},
		{
			name:       "DP-1.17.4 Egress Classification and rewrite of IPv6oMPLSoGUE traffic with pop action",
			ipType:     "ipv6",
			enableMpls: true,
			enableGre:  false,
			enableGue:  true,
		},
		{
			name:       "DP-1.17.5 Egress Classification and rewrite of IPoMPLSoGRE traffic with pop action",
			ipType:     "ipv4",
			enableMpls: true,
			enableGre:  true,
			enableGue:  false,
		},
		{
			name:       "DP-1.17.7 Egress Classification and rewrite of IPv6oMPLSoGRE traffic with pop action",
			ipType:     "ipv6",
			enableMpls: true,
			enableGre:  true,
			enableGue:  false,
		},
		{
			name:       "DP-1.17.8 Egress Classification and rewrite of IPoGRE traffic with decapsulate action",
			ipType:     "ipv4",
			enableMpls: false,
			enableGre:  true,
			enableGue:  false,
		},
		{
			name:       "DP-1.17.9 Egress Classification and rewrite of IPv6oGRE traffic with decapsulate action",
			ipType:     "ipv6",
			enableMpls: false,
			enableGre:  true,
			enableGue:  false,
		},
		{
			name:       "DP-1.17.10 Egress Classification and rewrite of IPoGUE traffic with pop action",
			ipType:     "ipv4",
			enableMpls: false,
			enableGre:  false,
			enableGue:  true,
		},
		{
			name:       "DP-1.17.11 Egress Classification and rewrite of IPv6oGUE traffic with pop action",
			ipType:     "ipv6",
			enableMpls: false,
			enableGre:  false,
			enableGue:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ipType == "ipv4" {
				rewriteIpv4PktsWithDscp(t, dut, ate, topo, tc.enableMpls, tc.enableGre, tc.enableGue)
			} else {
				rewriteIpv6PktsWithDscp(t, dut, ate, topo, tc.enableMpls, tc.enableGre, tc.enableGue)
			}
		})
	}
}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutPort1, dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

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
	pg2v4.PeerAs = ygot.Uint32(ate1Asn)

	pg2v6 := bgp.GetOrCreatePeerGroup(peerv6Grp2Name)
	pg2v6.PeerAs = ygot.Uint32(ate1Asn)

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
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()

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
	}, {
		desc:        "forwarding-group-NC1",
		queueName:   queues.NC1,
		targetGroup: "target-group-NC1",
	}}

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
		dscpSet:     []uint8{0, 1, 2, 3, 4, 5, 6, 7},
	}, {
		desc:        "classifier_ipv4_af1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "1",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{8, 9, 10, 11},
	}, {
		desc:        "classifier_ipv4_af2",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "2",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{16, 17, 18, 19},
	}, {
		desc:        "classifier_ipv4_af3",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "3",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{24, 25, 26, 27},
	}, {
		desc:        "classifier_ipv4_af4",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "4",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{32, 33, 34, 35},
	}, {
		desc:        "classifier_ipv4_nc1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "6",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{48, 49, 50, 51, 56, 57, 58, 59},
	}, {
		desc:        "classifier_ipv6_be1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0, 1, 2, 3, 4, 5, 6, 7},
	}, {
		desc:        "classifier_ipv6_af1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "1",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{8, 9, 10, 11, 12, 13, 14, 15},
	}, {
		desc:        "classifier_ipv6_af2",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "2",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{16, 17, 18, 19, 20, 21, 22, 23},
	}, {
		desc:        "classifier_ipv6_af3",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "3",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{24, 25, 26, 27, 28, 29, 30, 31},
	}, {
		desc:        "classifier_ipv6_af4",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "4",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47},
	}, {
		desc:        "classifier_ipv6_nc1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "6",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63},
	}}

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
	t.Logf("qos input remark config: %v", classifierIntfs)
	if deviations.QosRemarkOCUnsupported(dut) {
		configureRemarkIpv4(t, dut)
		configureRemarkIpv6(t, dut)
	}
}

func configureRemarkIpv4(t *testing.T, dut *ondatra.DUTDevice) {
	jsonConfig := `
    policy-map type quality-of-service __yang_[IPV4__dscp_based_classifier_ipv4][IPV6__dscp_based_classifier_ipv6]
	class __yang_[dscp_based_classifier_ipv4]_[0]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[1]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[2]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[3]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[4]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[6]
	set dscp 6
		`
	helpers.GnmiCLIConfig(t, dut, jsonConfig)
}

func configureRemarkIpv6(t *testing.T, dut *ondatra.DUTDevice) {
	jsonConfig := `
    policy-map type quality-of-service __yang_[IPV4__dscp_based_classifier_ipv4][IPV6__dscp_based_classifier_ipv6]
   class __yang_[dscp_based_classifier_ipv6]_[0]
      set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[1]
      set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[3]
   set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[2]
   set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[4]
   set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[6]
   set dscp 6
		`
	helpers.GnmiCLIConfig(t, dut, jsonConfig)
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
	bgp4Peer1.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	net1v4 := bgp4Peer1.V4Routes().Add().SetName(v4NetName1)
	net1v4.Addresses().Add().SetAddress(ipv4Src).SetPrefix(32).SetCount(1).SetStep(1)

	bgp6Peer1 := bgp1.Ipv6Interfaces().Add().SetIpv6Name(port1Ipv6.Name()).Peers().Add().SetName(port1Dev.Name() + ".BGP6.peer")
	bgp6Peer1.SetPeerAddress(port1Ipv6.Gateway())
	bgp6Peer1.SetAsNumber(ate1Asn)
	bgp6Peer1.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	net1v6 := bgp6Peer1.V6Routes().Add().SetName(v6NetName1)
	net1v6.Addresses().Add().SetAddress(ipv6Src).SetPrefix(128).SetCount(1).SetStep(1)

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
	net2v4.Addresses().Add().SetAddress(ipv4Dst).SetPrefix(32).SetCount(1).SetStep(1)

	bgp6Peer2 := bgp2.Ipv6Interfaces().Add().SetIpv6Name(port2Ipv6.Name()).Peers().Add().SetName(port2Dev.Name() + ".BGP6.peer")
	bgp6Peer2.SetPeerAddress(port2Ipv6.Gateway())
	bgp6Peer2.SetAsNumber(ate2Asn)
	bgp6Peer2.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	net2v6 := bgp6Peer2.V6Routes().Add().SetName(v6NetName2)
	net2v6.Addresses().Add().SetAddress(ipv6Dst).SetPrefix(128).SetCount(1).SetStep(1)

	return topo
}

func rewriteIpv6PktsWithDscp(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config, enableMpls bool, enableGre bool, enableGue bool) {

	topo.Flows().Clear()

	trafficFlows := []struct {
		desc    string
		dscpSet []uint32
	}{{
		desc:    "ipv6_be1",
		dscpSet: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
	}, {
		desc:    "ipv6_af1",
		dscpSet: []uint32{8, 9, 10, 11, 12, 13, 14, 15},
	}, {
		desc:    "ipv6_af2",
		dscpSet: []uint32{16, 17, 18, 19, 20, 21, 22, 23},
	}, {
		desc:    "ipv6_af3",
		dscpSet: []uint32{24, 25, 26, 27, 28, 29, 30, 31},
	}, {
		desc:    "ipv6_af4",
		dscpSet: []uint32{32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47},
	}, {
		desc:    "ipv6_nc1",
		dscpSet: []uint32{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63},
	}}

	t.Logf("Traffic config: %v", trafficFlows)
	for _, tc := range trafficFlows {
		trafficID := tc.desc
		t.Logf("Configuring flow %s", trafficID)
		flow := topo.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(atePort1.MAC)
		ethHeader.Dst().Auto()
		if enableGue {
			outerIpHeader := flow.Packet().Add().Ipv6()
			outerIpHeader.Src().SetValue(atePort1.IPv6)
			outerIpHeader.Dst().SetValue(atePort2.IPv6)
			outerIpHeader.TrafficClass().SetValues(tc.dscpSet)
			if enableMpls {
				mpls := flow.Packet().Add().Mpls()
				mpls.Label().SetValue(mplsPopLabelv4)
			}
			udpHeader := flow.Packet().Add().Udp()
			udpHeader.SrcPort().SetValue(30000)
			udpHeader.DstPort().SetValue(uint32(guePort))
			innerIpHeader := flow.Packet().Add().Ipv6()
			innerIpHeader.Src().SetValue(ipv6Src)
			innerIpHeader.Dst().SetValue(ipv6Dst)
			innerIpHeader.TrafficClass().SetValues(tc.dscpSet)
		} else if enableGre {
			outerIpHeader := flow.Packet().Add().Ipv4()
			outerIpHeader.Src().SetValue(atePort1.IPv4)
			outerIpHeader.Dst().SetValue(atePort2.IPv4)
			outerIpHeader.Priority().Dscp().Phb().SetValues(tc.dscpSet)
			flow.Packet().Add().Gre()
			if enableMpls {
				mpls := flow.Packet().Add().Mpls()
				mpls.Label().SetValue(mplsPopLabelv4)
			}
			grev6 := flow.Packet().Add().Ipv6()
			grev6.Src().SetValue(ipv6Src)
			grev6.Dst().SetValue(ipv6Dst)
			grev6.TrafficClass().SetValues(tc.dscpSet)

		} else {
			ipHeader := flow.Packet().Add().Ipv6()
			ipHeader.Src().SetValue(atePort1.IPv6)
			ipHeader.Dst().SetValue(atePort2.IPv6)
			ipHeader.TrafficClass().SetValues(tc.dscpSet)
		}
		flow.Size().SetFixed(uint32(frameSize))
		flow.Rate().SetPps(packetPerSecond)
		flow.Duration().FixedPackets().SetPackets(packetPerSecond)
	}

	ate.OTG().PushConfig(t, topo)

	if enableMpls {
		t.Logf("Configuring MPLS POP Action for IPv4 with DUT")
		cfgplugins.NewStaticMplsLspPopLabel(t, dut, "lsp-pop", mplsPopLabelv4, "", atePort2.IPv4, "ipv6")
	}

	if enableGue {
		configureDutWithGueDecap(t, dut, guePort, "ipv6", atePort2.IPv6)
	}
	if enableGre {
		configureDutWithGreDecap(t, dut, "ipv6", atePort2.IPv4)
	}

	intialpacket1 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "0")
	intialpacket2 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "1")
	intialpacket3 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "2")
	intialpacket4 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "3")
	intialpacket5 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "4")
	intialpacket6 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "6")

	startCapture(t, ate)
	trafficStartStop(t, dut, ate, topo)
	stopCapture(t, ate)

	if enableMpls {
		t.Logf("removing MPLS POP Action for IPv6 with DUT")
		cfgplugins.RemoveStaticMplsLspPopLabel(t, dut, "lsp-pop", mplsPopLabelv4, "", atePort2.IPv4, "ipv6")
	}

	for _, trafficID := range trafficFlows {
		t.Logf("Verify Traffic flow %s", trafficID.desc)
		verifyTrafficFlow(t, ate, trafficID.desc)
	}

	verifyIpv6DscpCapture(t, ate, "port2", enableGue, enableGre)

	finalpacket1 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "0")
	finalpacket2 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "1")
	finalpacket3 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "2")
	finalpacket4 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "3")
	finalpacket5 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "4")
	finalpacket6 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "6")

	compare_counters(t, intialpacket1, finalpacket1)
	compare_counters(t, intialpacket2, finalpacket2)
	compare_counters(t, intialpacket3, finalpacket3)
	compare_counters(t, intialpacket4, finalpacket4)
	compare_counters(t, intialpacket5, finalpacket5)
	compare_counters(t, intialpacket6, finalpacket6)

}

func rewriteIpv4PktsWithDscp(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config, enableMpls bool, enableGre bool, enableGue bool) {
	topo.Flows().Clear()
	trafficFlows := []struct {
		desc    string
		dscpSet []uint32
	}{{
		desc:    "ipv4_be1",
		dscpSet: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
	}, {
		desc:    "ipv4_af1",
		dscpSet: []uint32{8, 9, 10, 11},
	}, {
		desc:    "ipv4_af2",
		dscpSet: []uint32{16, 17, 18, 19},
	}, {
		desc:    "ipv4_af3",
		dscpSet: []uint32{24, 25, 26, 27},
	}, {
		desc:    "ipv4_af4",
		dscpSet: []uint32{32, 33, 34, 35},
	}, {
		desc:    "ipv4_nc1",
		dscpSet: []uint32{48, 49, 50, 51, 56, 57, 58, 59},
	}}

	t.Logf("Traffic config: %v", trafficFlows)
	for _, tc := range trafficFlows {
		t.Logf("Configuring flow %s", tc.desc)
		flow := topo.Flows().Add().SetName(tc.desc)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(atePort1.MAC)
		ethHeader.Dst().Auto()
		if enableGue {
			outerIpHeader := flow.Packet().Add().Ipv4()
			outerIpHeader.Src().SetValue(atePort1.IPv4)
			outerIpHeader.Dst().SetValue(atePort2.IPv4)
			outerIpHeader.Priority().Dscp().Phb().SetValues(tc.dscpSet)
			if enableMpls {
				mpls := flow.Packet().Add().Mpls()
				mpls.Label().SetValue(mplsPopLabelv4)
			}
			udpHeader := flow.Packet().Add().Udp()
			udpHeader.SrcPort().SetValue(30000)
			udpHeader.DstPort().SetValue(uint32(guePort))
			innerIpHeader := flow.Packet().Add().Ipv4()
			innerIpHeader.Src().SetValue(ipv4Src)
			innerIpHeader.Dst().SetValue(ipv4Dst)
			innerIpHeader.Priority().Dscp().Phb().SetValues(tc.dscpSet)
		} else if enableGre {
			outerIpHeader := flow.Packet().Add().Ipv4()
			outerIpHeader.Src().SetValue(atePort1.IPv4)
			outerIpHeader.Dst().SetValue(atePort2.IPv4)
			flow.Packet().Add().Gre()
			if enableMpls {
				mpls := flow.Packet().Add().Mpls()
				mpls.Label().SetValue(mplsPopLabelv4)
			}
			grev4 := flow.Packet().Add().Ipv4()
			grev4.Src().SetValue(ipv4Src)
			grev4.Dst().SetValue(ipv4Dst)
			grev4.Priority().Dscp().Phb().SetValue(32)

		} else {
			ipHeader := flow.Packet().Add().Ipv4()
			ipHeader.Src().SetValue(atePort1.IPv4)
			ipHeader.Dst().SetValue(atePort2.IPv4)
			ipHeader.Priority().Dscp().Phb().SetValues(tc.dscpSet)
		}

		flow.Size().SetFixed(uint32(frameSize))
		flow.Rate().SetPps(packetPerSecond)
		flow.Duration().FixedPackets().SetPackets(packetPerSecond)
	}

	ate.OTG().PushConfig(t, topo)
	if enableMpls {
		t.Logf("Configuring MPLS POP Action for IPv4 with DUT")
		cfgplugins.NewStaticMplsLspPopLabel(t, dut, "lsp-pop", mplsPopLabelv4, "", atePort2.IPv4, "ipv4")
	}

	if enableGue {
		configureDutWithGueDecap(t, dut, guePort, "ipv4", atePort2.IPv4)
	}
	if enableGre {
		configureDutWithGreDecap(t, dut, "ipv4", atePort2.IPv4)
	}

	intialpacket1 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "0")
	intialpacket2 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "1")
	intialpacket3 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "2")
	intialpacket4 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "3")
	intialpacket5 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "4")
	intialpacket6 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "6")

	startCapture(t, ate)
	trafficStartStop(t, dut, ate, topo)
	stopCapture(t, ate)
	if enableMpls {
		t.Logf("removing MPLS POP Action for IPv4 with DUT")
		cfgplugins.RemoveStaticMplsLspPopLabel(t, dut, "lsp-pop", mplsPopLabelv4, "", atePort2.IPv4, "ipv4")
	}
	t.Logf("Traffic config: %v", trafficFlows)
	for _, trafficID := range trafficFlows {
		t.Logf("Verify Traffic flow %s", trafficID.desc)
		verifyTrafficFlow(t, ate, trafficID.desc)
	}
	verifyIpv4DscpCapture(t, ate, "port2", enableGue, enableGre)

	finalpacket1 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "0")
	finalpacket2 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "1")
	finalpacket3 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "2")
	finalpacket4 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "3")
	finalpacket5 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "4")
	finalpacket6 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "6")

	compare_counters(t, intialpacket1, finalpacket1)
	compare_counters(t, intialpacket2, finalpacket2)
	compare_counters(t, intialpacket3, finalpacket3)
	compare_counters(t, intialpacket4, finalpacket4)
	compare_counters(t, intialpacket5, finalpacket5)
	compare_counters(t, intialpacket6, finalpacket6)

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

func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string) {
	recvMetricV4 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())

	framesTxV4 := recvMetricV4.GetCounters().GetOutPkts()
	framesRxV4 := recvMetricV4.GetCounters().GetInPkts()

	t.Logf("Starting traffic validation for flow %s", flowName)
	if framesTxV4 == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if framesRxV4 == framesTxV4 {
		t.Logf("Traffic validation successful for [%s] FramesTx: %d FramesRx: %d", flowName, framesTxV4, framesRxV4)
	} else {
		t.Errorf("Traffic validation failed for [%s] FramesTx: %d FramesRx: %d", flowName, framesTxV4, framesRxV4)
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

func verifyIpv4DscpCapture(t *testing.T, ate *ondatra.ATEDevice, port string, enableGue bool, enableGre bool) {
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)
			if ip.SrcIP.Equal(net.ParseIP(atePort1.IPv4)) {
				dscpValue := ip.TOS >> 2
				if !(dscpValue == 0 || dscpValue == 6) {
					t.Fatalf("Error: DSCP value should be converted to 0 or 6 DUT. Got %v", dscpValue)
				}
			}
		}
		if enableGue {
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			udp, ok := udpLayer.(*layers.UDP)
			if ok || udp != nil {
				t.Fatalf("GUE layer found and Not decapsulated")
			}
		}
		if enableGre {
			greLayer := packet.Layer(layers.LayerTypeGRE)
			grePacket, ok := greLayer.(*layers.GRE)
			if ok || grePacket != nil {
				t.Fatalf("GRE layer found and Not decapsulated")
			}
		}
	}

}

func verifyIpv6DscpCapture(t *testing.T, ate *ondatra.ATEDevice, port string, enableGue bool, enableGre bool) {
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv6)
			if ip.SrcIP.Equal(net.ParseIP(atePort1.IPv6)) {
				dscpValue := ip.TrafficClass >> 2
				if !(dscpValue == 0 || dscpValue == 6) {
					t.Fatalf("Error: DSCP value should be converted to 0 or 6 DUT. Got %v", dscpValue)
				}
			}
		}
		if enableGue {
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			udp, ok := udpLayer.(*layers.UDP)
			if ok || udp != nil {
				t.Fatalf("GUE layer found and Not decapsulated")
			}
		}
		if enableGre {
			greLayer := packet.Layer(layers.LayerTypeGRE)
			grePacket, ok := greLayer.(*layers.GRE)
			if ok || grePacket != nil {
				t.Error("GRE layer found and Not decapsulated")
			}
		}
	}
}

func verfiy_classifier_packets(t *testing.T, dut *ondatra.DUTDevice, classifier oc.E_Input_Classifier_Type, termId string) uint64 {
	dp1 := dut.Port(t, "port1")
	const timeout = 10 * time.Second
	isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }

	_, ok := gnmi.WatchAll(t, dut, gnmi.OC().Qos().Interface(dp1.Name()).Input().ClassifierAny().Term(termId).MatchedPackets().State(), timeout, isPresent).Await(t)
	if !ok {
		t.Errorf("Unable to find matched packets")
	}
	matchpackets := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp1.Name()).Input().Classifier(classifier).Term(termId).MatchedPackets().State())
	return matchpackets
}

func compare_counters(t *testing.T, intialpacket uint64, finalpacket uint64) {
	t.Logf("Classifier counters Before Traffic %v", intialpacket)
	t.Logf("Classifier counters After Traffic %v", finalpacket)
	if finalpacket > intialpacket {
		t.Logf("Pass : Classifier counters got incremented after start and stop traffic")
	} else {
		t.Errorf("Fail : Classifier counters not incremented after start and stop traffic. Refer BUG ID 419618177")
	}
}

func configureDutWithGueDecap(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string, ipAddr string) {
	t.Logf("Configure DUT with decapsulation UDP port %v", guePort)
	ocPFParams := GetDefaultOcPolicyForwardingParams(t, dut, guePort, ipType, ipAddr, "dcap-policy-Gue")
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
}

func configureDutWithGreDecap(t *testing.T, dut *ondatra.DUTDevice, ipType string, ipAddr string) {
	t.Logf("Configure DUT with decapsulation UDP port %v", guePort)
	ocPFParams := GetDefaultOcPolicyForwardingParams(t, dut, 47, ipType, ipAddr, "dcap-policy-Gre")
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGre(t, dut, pf, ocPFParams)
}

// GetDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func GetDefaultOcPolicyForwardingParams(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string, ipAddr string, policyName string) cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         dut.Port(t, "port1").Name(),
		AppliedPolicyName:   policyName,
		TunnelIP:            ipAddr,
		GuePort:             uint32(guePort),
		IpType:              ipType,
		Dynamic:             true,
	}
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
