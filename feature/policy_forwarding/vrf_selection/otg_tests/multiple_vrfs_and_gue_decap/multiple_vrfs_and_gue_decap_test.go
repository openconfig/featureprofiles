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
	"strconv"
	"testing"
	"time"

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
	advertiseIpv4PrefixLength = 32
	advertiseIpv6PrefixLength = 128
	ipv4PrefixLen             = 30
	ipv6PrefixLen             = 126
	ate1Asn                   = 65001
	ate2Asn                   = 65003
	dutAsn                    = 65001
	ipv4Src                   = "198.51.100.1"
	ipv4Dst                   = "198.51.200.1"
	ipv6Src                   = "2001:DB8:1::1"
	ipv6Dst                   = "2001:DB8:2::1"
	rplType                   = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	rplName                   = "ALLOW"
	peerv4Grp1Name            = "BGP-PEER-GROUP1-V4"
	peerv6Grp1Name            = "BGP-PEER-GROUP1-V6"
	peerv4Grp2Name            = "BGP-PEER-GROUP2-V4"
	peerv6Grp2Name            = "BGP-PEER-GROUP2-V6"
	v4NetName1                = "BGPv4RR1"
	v6NetName1                = "BGPv6RR1"
	v4NetName2                = "BGPv4RR2"
	v6NetName2                = "BGPv6RR2"
	packetPerSecond           = 100
	guePort                   = 6080
	trafficSleepTime          = 10
	nonDefaultVrfName         = "B2_VRF"
	packetSize                = 512
	IPv4Prefix1               = "198.51.100.1"
	IPv4Prefix2               = "198.51.100.2"
	IPv4Prefix3               = "198.51.100.3"
	IPv4Prefix4               = "198.51.100.4"
	IPv4Prefix5               = "198.51.100.5"
	IPv4Prefix6               = "198.51.200.1"
	IPv4Prefix7               = "198.51.200.2"
	IPv4Prefix8               = "198.51.200.3"
	IPv4Prefix9               = "198.51.200.4"
	IPv4Prefix10              = "198.51.200.5"
	IPv6Prefix1               = "2001:DB8:1::1"
	IPv6Prefix2               = "2001:DB8:1::2"
	IPv6Prefix3               = "2001:DB8:1::3"
	IPv6Prefix4               = "2001:DB8:1::4"
	IPv6Prefix5               = "2001:DB8:1::5"
	IPv6Prefix6               = "2001:DB8:2::1"
	IPv6Prefix7               = "2001:DB8:2::2"
	IPv6Prefix8               = "2001:DB8:2::3"
	IPv6Prefix9               = "2001:DB8:2::4"
	IPv6Prefix10              = "2001:DB8:2::5"
	policyName                = "decap-policy-gue"
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
		IPv4Len: advertiseIpv4PrefixLength,
		IPv6Len: advertiseIpv6PrefixLength,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	name      string
	flownames []string
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func TestMultipleVrfsAndGueDecap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	t.Log(dp1, dp2)

	// Configure DUT interfaces.
	t.Logf("Configuring Hardware Init")
	configureHardwareInit(t, dut)
	createVRF(t, dut)
	configureDUTIntf(t, dut)
	configureRoutePolicy(t, dut, rplName, rplType)
	configureBgp(t, dut)
	configureQoSDUTIpv4Ipv6(t, dut)

	configureRouteLeaking(t, dut)
	configureDutWithGueDecap(t, dut, guePort, "ipv4")

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)

	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	verifyBGPTelemetry(t, dut)
	verifyLeakedRoutes(t, dut)
	configureIPv4Traffic(t, ate, topo)
	trafficStartStop(t, dut, ate, topo)

	testCases := []testCase{
		{
			name:      "PF-2.3.1: [Baseline] Traffic flow between ATE:Port1 and ATE:Port2 via DUT's Default VRF",
			flownames: []string{"1to6v4", "2to7v4", "3to8v4", "4to9v4", "5to10v4", "1to6v6", "2to7v6", "3to8v6", "4to9v6", "5to10v6"},
		},
		{
			name:      "PF-2.3.2: BE1 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames: []string{"1to6v4_encapped", "1to6v6_encapped", "2to7v4", "3to8v4", "4to9v4", "5to10v4", "2to7v6", "3to8v6", "4to9v6", "5to10v6"},
		},
		{
			name:      "PF-2.3.3: BE1 and AF1 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames: []string{"1to6v4_encapped", "2to7v4_encapped", "1to6v6_encapped", "2to7v6_encapped", "3to8v4", "4to9v4", "5to10v4", "3to8v6", "4to9v6", "5to10v6"},
		},
		{
			name:      "PF-2.3.4: BE1, AF1 and AF2 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames: []string{"1to6v4_encapped", "2to7v4_encapped", "3to8v4_encapped", "1to6v6_encapped", "2to7v6_encapped", "3to8v6_encapped", "4to9v4", "5to10v4", "4to9v6", "5to10v6"},
		},
		{
			name:      "PF-2.3.5: BE1, AF1, AF2 and AF3 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames: []string{"1to6v4_encapped", "2to7v4_encapped", "3to8v4_encapped", "1to6v6_encapped", "4to9v4_encapped", "2to7v6_encapped", "3to8v6_encapped", "4to9v6_encapped", "5to10v4", "5to10v6"},
		},
		{
			name:      "PF-2.3.6: BE1, AF1, AF2, AF3 and AF4 traffic from ATE:Port1 to ATE:Port2 simulated to be GUE Encaped and sent to the DUT's Default VRF by ATE:Port2",
			flownames: []string{"1to6v4_encapped", "2to7v4_encapped", "3to8v4_encapped", "4to9v4_encapped", "5to10v4_encapped", "1to6v6_encapped", "2to7v6_encapped", "3to8v6_encapped", "4to9v6_encapped", "5to10v6_encapped"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validateTrafficLoss(t, ate, tc.flownames)
		})
	}
}

// configureDutWithGueDecap configure DUT with GUE decapsulation
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
		GUEPort:             uint32(guePort),
		IPType:              ipType,
		Dynamic:             true,
		DecapProtocol:       "ip",
	}
}

// configureDUTIntf configure DUT interface IP address
func configureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutPort1, dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))

	configureLoopbackInterface(t, dut)

}

func configureBgp(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	// Define neighbors for peer-group 1
	nbr1v4 := &cfgplugins.BgpNeighbor{LocalAS: dutAsn, PeerAS: ate1Asn, Neighborip: atePort1.IPv4, IsV4: true, PeerGrp: peerv4Grp1Name}
	nbr1v6 := &cfgplugins.BgpNeighbor{LocalAS: dutAsn, PeerAS: ate1Asn, Neighborip: atePort1.IPv6, IsV4: false, PeerGrp: peerv6Grp1Name}

	// Define neighbors for peer-group 2
	nbr2v4 := &cfgplugins.BgpNeighbor{LocalAS: dutAsn, PeerAS: ate2Asn, Neighborip: atePort2.IPv4, IsV4: true, PeerGrp: peerv4Grp2Name}
	nbr2v6 := &cfgplugins.BgpNeighbor{LocalAS: dutAsn, PeerAS: ate2Asn, Neighborip: atePort2.IPv6, IsV4: false, PeerGrp: peerv6Grp2Name}

	// Prepare GNMI batch
	sb := &gnmi.SetBatch{}

	pg1Cfg := cfgplugins.BGPNeighborsConfig{
		RouterID:      dutPort2.IPv4,
		PeerGrpNameV4: peerv4Grp1Name,
		PeerGrpNameV6: peerv6Grp1Name,
		Nbrs:          []*cfgplugins.BgpNeighbor{nbr1v4, nbr1v6},
	}

	if err := cfgplugins.CreateBGPNeighbors(t, dut, sb, pg1Cfg); err != nil {
		t.Fatalf("Failed to configure peer-group 1 neighbors: %v", err)
	}

	pg2Cfg := cfgplugins.BGPNeighborsConfig{
		RouterID:      dutPort2.IPv4,
		PeerGrpNameV4: peerv4Grp2Name,
		PeerGrpNameV6: peerv6Grp2Name,
		Nbrs:          []*cfgplugins.BgpNeighbor{nbr2v4, nbr2v6},
	}

	if err := cfgplugins.CreateBGPNeighbors(t, dut, sb, pg2Cfg); err != nil {
		t.Fatalf("Failed to configure peer-group 2 neighbors: %v", err)
	}

	sb.Set(t, dut)
}

// configInterfaceDUT Configures the given DUT interface.
func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	return i
}

// configureQoSDUTIpv4Ipv6 configure dut with QoS configuration
func configureQoSDUTIpv4Ipv6(t *testing.T, dut *ondatra.DUTDevice) {
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
	forwardingGroups := []cfgplugins.ForwardingGroup{
		{
			Desc:        "forwarding-group-BE1",
			QueueName:   queues.BE1,
			TargetGroup: "target-group-BE1",
		}, {
			Desc:        "forwarding-group-AF1",
			QueueName:   queues.AF1,
			TargetGroup: "target-group-AF1",
		}, {
			Desc:        "forwarding-group-AF2",
			QueueName:   queues.AF2,
			TargetGroup: "target-group-AF2",
		}, {
			Desc:        "forwarding-group-AF3",
			QueueName:   queues.AF3,
			TargetGroup: "target-group-AF3",
		}, {
			Desc:        "forwarding-group-AF4",
			QueueName:   queues.AF4,
			TargetGroup: "target-group-AF4",
		}, {
			Desc:        "forwarding-group-NC1",
			QueueName:   queues.NC1,
			TargetGroup: "target-group-NC1",
		}}

	cfgplugins.NewQoSForwardingGroup(t, dut, q, forwardingGroups)

	t.Logf("Create qos Classifiers config")
	classifiers := []cfgplugins.QosClassifier{
		{
			Desc:        "classifier_ipv4_be1",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "0",
			TargetGroup: "target-group-BE1",
			DscpSet:     []uint8{0},
		}, {
			Desc:        "classifier_ipv4_af1",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "1",
			TargetGroup: "target-group-AF1",
			DscpSet:     []uint8{8},
		}, {
			Desc:        "classifier_ipv4_af2",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "2",
			TargetGroup: "target-group-AF2",
			DscpSet:     []uint8{16},
		}, {
			Desc:        "classifier_ipv4_af3",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "3",
			TargetGroup: "target-group-AF3",
			DscpSet:     []uint8{24},
		}, {
			Desc:        "classifier_ipv4_af4",
			Name:        "dscp_based_classifier_ipv4",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "4",
			TargetGroup: "target-group-AF4",
			DscpSet:     []uint8{32},
		}, {
			Desc:        "classifier_ipv6_be1",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "0",
			TargetGroup: "target-group-BE1",
			DscpSet:     []uint8{0},
		}, {
			Desc:        "classifier_ipv6_af1",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "1",
			TargetGroup: "target-group-AF1",
			DscpSet:     []uint8{8},
		}, {
			Desc:        "classifier_ipv6_af2",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "2",
			TargetGroup: "target-group-AF2",
			DscpSet:     []uint8{16},
		}, {
			Desc:        "classifier_ipv6_af3",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "3",
			TargetGroup: "target-group-AF3",
			DscpSet:     []uint8{24},
		}, {
			Desc:        "classifier_ipv6_af4",
			Name:        "dscp_based_classifier_ipv6",
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "4",
			TargetGroup: "target-group-AF4",
			DscpSet:     []uint8{32},
		}}

	q = cfgplugins.NewQoSClassifierConfiguration(t, dut, q, classifiers)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

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
	net1v4.Addresses().Add().SetAddress(ipv4Src).SetPrefix(advertiseIpv4PrefixLength).SetCount(5).SetStep(1)

	bgp6Peer1 := bgp1.Ipv6Interfaces().Add().SetIpv6Name(port1Ipv6.Name()).Peers().Add().SetName(port1Dev.Name() + ".BGP6.peer")
	bgp6Peer1.SetPeerAddress(port1Ipv6.Gateway())
	bgp6Peer1.SetAsNumber(ate1Asn)
	bgp6Peer1.SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	net1v6 := bgp6Peer1.V6Routes().Add().SetName(v6NetName1)
	net1v6.Addresses().Add().SetAddress(ipv6Src).SetPrefix(advertiseIpv6PrefixLength).SetCount(5).SetStep(1)

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
	net2v4.Addresses().Add().SetAddress(ipv4Dst).SetPrefix(advertiseIpv4PrefixLength).SetCount(5).SetStep(1)

	bgp6Peer2 := bgp2.Ipv6Interfaces().Add().SetIpv6Name(port2Ipv6.Name()).Peers().Add().SetName(port2Dev.Name() + ".BGP6.peer")
	bgp6Peer2.SetPeerAddress(port2Ipv6.Gateway())
	bgp6Peer2.SetAsNumber(ate2Asn)
	bgp6Peer2.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	net2v6 := bgp6Peer2.V6Routes().Add().SetName(v6NetName2)
	net2v6.Addresses().Add().SetAddress(ipv6Dst).SetPrefix(advertiseIpv6PrefixLength).SetCount(5).SetStep(1)

	return topo
}

// trafficStartStop - start and stop traffic from OTG
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

// validateTrafficLoss - validate traffic loss on each flows
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

// configureRouteLeaking - route leaking from default to non default VRF
func configureRouteLeaking(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.NetworkInstanceImportExportPolicyOCUnsupported(dut) {
		t.Logf("Configuring route leaking through CLI")
		configureRouteLeakingFromCLI(t, dut)
	} else {
		t.Logf("Configuring route leaking through OC")
		configureRouteLeakingFromOC(t, dut)
	}
}

// configureRouteLeakingFromCLI - route leaking from CLI
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

// configureRouteLeakingFromOC - configures route leaking functionality from OC command
func configureRouteLeakingFromOC(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	root := &oc.Root{}

	ni1 := root.GetOrCreateNetworkInstance(nonDefaultVrfName)
	ni1Pol := ni1.GetOrCreateInterInstancePolicies()
	iexp1 := ni1Pol.GetOrCreateImportExportPolicy()
	iexp1.SetImportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ImportRouteTarget_Union{oc.UnionString(deviations.DefaultNetworkInstance(dut))})
	iexp1.SetExportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ExportRouteTarget_Union{oc.UnionString(deviations.DefaultNetworkInstance(dut))})
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(nonDefaultVrfName).InterInstancePolicies().Config(), ni1Pol)
}

// waitForBGPSession - verify BGP session is Established
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

// verifyBGPTelemetry - verify BGP neighborship
func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Waiting for BGPv4 neighbor to establish...")
	waitForBGPSession(t, dut, true)
}

// createVRF - creates non default VRF in dut
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

// configureLoopbackInterface - create and configure loopback interface
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

// configureIPv4Traffic - create traffic flows in ATE
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

// verifyLeakedRoutes - verify ip routes in no default VRF
func verifyLeakedRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	routes := []string{IPv4Prefix1, IPv4Prefix2, IPv4Prefix3, IPv4Prefix4, IPv4Prefix5, IPv4Prefix6, IPv4Prefix7, IPv4Prefix8, IPv4Prefix9, IPv4Prefix10}
	for _, adroute := range routes {
		advroute := adroute + "/" + strconv.Itoa(advertiseIpv4PrefixLength)
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

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureVrfSelectionExtended)
	hardwareInitCfg1 := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeaturePolicyForwarding)
	hardwareInitCfg2 := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureEnableAFTSummaries)
	hardwareInitCfg3 := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureMplsTracking)
	if hardwareInitCfg == "" || hardwareInitCfg1 == "" || hardwareInitCfg2 == "" || hardwareInitCfg3 == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg1)
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg2)
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg3)
}
