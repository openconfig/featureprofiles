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

package ingress_traffic_classification_and_rewrite_test

import (
	"context"
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
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen        = 30
	ipv6PrefixLen        = 126
	mplsSwapLabel        = 24000
	mplsSwapLabelTo      = 25000
	mplsPopLabelv4       = 26000
	mplsPopLabelv6       = 27000
	mplsPushLabelV4      = 28000
	mplsPushLabelV6      = 29000
	ipv4DestAddr         = "203.1.1.1"
	ipv4DestAddrWithCidr = "203.1.1.1/32"
	ipv6DestAddr         = "203:0:0:1::1"
	ipv6DestAddrWithCidr = "203:0:0:1::1/128"
	frameSize            = 512
	packetPerSecond      = 100
	maxIpv6Tos           = 63
	timeout              = 30
	trafficSleepTime     = 10
	captureWait          = 10
	lspNextHopIndex      = 0
	implicitNull         = 3
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
		MAC:     "00:00:a1:a1:a1:a1",
		IPv6:    "2001:db8::1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = &attrs.Attributes{
		Desc:    "dutPort2",
		MAC:     "00:00:b1:b1:b1:b1",
		IPv6:    "2001:db8::5",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIngressTrafficClassificationAndRewrite(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	t.Log(dp1, dp2)

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)
	ConfigureQoS(t, dut)

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)
	enableCapture(t, topo, "port2")

	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")

	t.Run("DP-1.16.1 Ingress Classification and rewrite of IPv4 packets with various DSCP values", func(t *testing.T) {
		rewriteIpv4PktsWithDscp(t, dut, ate, topo)
	})
	t.Run("DP-1.16.2 Ingress Classification and rewrite of IPv6 packets with various DSCP values", func(t *testing.T) {
		rewriteIpv6PktsWithDscp(t, dut, ate, topo)
	})
	t.Run("DP-1.16.3 Ingress Classification and rewrite of MPLS traffic with swap action", func(t *testing.T) {
		rewriteMplsSwapAction(t, dut, ate, topo)
	})
	t.Run("DP-1.16.4 Ingress Classification and rewrite of IPv4-over-MPLS traffic with pop action", func(t *testing.T) {
		rewriteIpv4MplsPopAction(t, dut, ate, topo)
	})
	t.Run("DP-1.16.5 Ingress Classification and rewrite of IPv6-over-MPLS traffic with pop action", func(t *testing.T) {
		rewriteIpv6MplsPopAction(t, dut, ate, topo)
	})
	t.Run("DP-1.16.6 Ingress Classification and rewrite of IPv4 packets traffic with label push action", func(t *testing.T) {
		rewriteIpv4MplsPushAction(t, dut, ate, topo)
	})
	t.Run("DP-1.16.7 Ingress Classification and rewrite of IPv6 packets traffic with label push action", func(t *testing.T) {
		rewriteIpv6MplsPushAction(t, dut, ate, topo)
	})

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
		expSet      []uint8 // MPLS EXP values
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
		dscpSet:     []uint8{1},
	}, {
		desc:        "classifier_ipv4_af2",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "2",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{2},
	}, {
		desc:        "classifier_ipv4_af3",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "3",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{3},
	}, {
		desc:        "classifier_ipv4_af4",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "4",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{4, 5},
	}, {
		desc:        "classifier_ipv4_nc1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "6",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{6, 7},
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
		// }, {
		// 	desc:        "classifier_mpls_be1",
		// 	name:        "exp_based_classifier_mpls",
		// 	classType:   oc.Qos_Classifier_Type_MPLS,
		// 	termID:      "0",
		// 	targetGroup: "target-group-BE1",
		// 	expSet:      []uint8{0},
		// }, {
		// 	desc:        "classifier_mpls_af1",
		// 	name:        "exp_based_classifier_mpls",
		// 	classType:   oc.Qos_Classifier_Type_MPLS,
		// 	termID:      "1",
		// 	targetGroup: "target-group-AF1",
		// 	expSet:      []uint8{1},
		// }, {
		// 	desc:        "classifier_mpls_af2",
		// 	name:        "exp_based_classifier_mpls",
		// 	classType:   oc.Qos_Classifier_Type_MPLS,
		// 	termID:      "2",
		// 	targetGroup: "target-group-AF2",
		// 	expSet:      []uint8{2},
		// }, {
		// 	desc:        "classifier_mpls_af3",
		// 	name:        "exp_based_classifier_mpls",
		// 	classType:   oc.Qos_Classifier_Type_MPLS,
		// 	termID:      "3",
		// 	targetGroup: "target-group-AF3",
		// 	expSet:      []uint8{3},
		// }, {
		// 	desc:        "classifier_mpls_af4",
		// 	name:        "exp_based_classifier_mpls",
		// 	classType:   oc.Qos_Classifier_Type_MPLS,
		// 	termID:      "4",
		// 	targetGroup: "target-group-AF4",
		// 	expSet:      []uint8{4, 5},
		// }, {
		// 	desc:        "classifier_mpls_nc1",
		// 	name:        "exp_based_classifier_mpls",
		// 	classType:   oc.Qos_Classifier_Type_MPLS,
		// 	termID:      "6",
		// 	targetGroup: "target-group-NC1",
		// 	expSet:      []uint8{6, 7},
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
		} else if tc.name == "dscp_based_classifier_ipv6" {
			condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
		}
		// else if tc.name == "exp_based_classifier_mpls" {
		// 	t.Log("creating MPLS")
		// 	condition.GetOrCreateMpls().SetTrafficClass(tc.expSet[0])
		// }

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
		// }, {
		// 	desc:                "Input Classifier Type MPLS",
		// 	intf:                dp1.Name(),
		// 	inputClassifierType: oc.Input_Classifier_Type_MPLS,
		// 	classifier:          "exp_based_classifier_mpls",
	}}
	t.Logf("qos input classifier config: %v", classifierIntfs)
	for _, tc := range classifierIntfs {
		qoscfg.SetInputClassifier(t, dut, q, tc.intf, tc.inputClassifierType, tc.classifier)
	}
	t.Logf("qos input remark config: %v", classifierIntfs)
	if deviations.QosRemarkOCUnsupported(dut) {
		configureRemarkIpv4(t, dut)
		configureRemarkIpv6(t, dut)
		configureRemarkMpls(t, dut)
	}
}

func configureRemarkMpls(t *testing.T, dut *ondatra.DUTDevice) {
	gnmiClient := dut.RawAPIs().GNMI(t)
	jsonConfig := (`
		qos map exp 5 to traffic-class 4
		qos map exp 7 to traffic-class 6
		qos map traffic-class 4 to exp 4
		qos map traffic-class 6 to exp 6
	`)
	gpbSetRequest := buildCliConfigRequest(jsonConfig)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

func configureRemarkIpv4(t *testing.T, dut *ondatra.DUTDevice) {
	gnmiClient := dut.RawAPIs().GNMI(t)
	jsonConfig := `
    policy-map type quality-of-service __yang_[IPV4__dscp_based_classifier_ipv4][IPV6__dscp_based_classifier_ipv6]
	class __yang_[dscp_based_classifier_ipv4]_[4]
	set dscp 4
	class __yang_[dscp_based_classifier_ipv4]_[6]
	set dscp 6
		`
	gpbSetRequest := buildCliConfigRequest(jsonConfig)

	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

func configureRemarkIpv6(t *testing.T, dut *ondatra.DUTDevice) {
	gnmiClient := dut.RawAPIs().GNMI(t)
	jsonConfig := `
    policy-map type quality-of-service __yang_[IPV4__dscp_based_classifier_ipv4][IPV6__dscp_based_classifier_ipv6]
   class __yang_[dscp_based_classifier_ipv6]_[0]
      set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[1]
      set dscp 8
   class __yang_[dscp_based_classifier_ipv6]_[3]
   set dscp 16
   class __yang_[dscp_based_classifier_ipv6]_[2]
   set dscp 24
   class __yang_[dscp_based_classifier_ipv6]_[4]
   set dscp 32
   class __yang_[dscp_based_classifier_ipv6]_[6]
   set dscp 48
		`
	gpbSetRequest := buildCliConfigRequest(jsonConfig)

	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

// Support method to execute GNMIC commands
func buildCliConfigRequest(config string) *gpb.SetRequest {
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{
					Origin: "cli",
					Elem:   []*gpb.PathElem{},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_AsciiVal{
						AsciiVal: config,
					},
				},
			},
		},
	}
	return gpbSetRequest
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

	port2Dev := topo.Devices().Add().SetName(atePort2.Name + ".dev")
	port2Eth := port2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	port2Eth.Connection().SetPortName(port2.Name())
	port2Ipv4 := port2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	port2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	port2Ipv6 := port2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	port2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	return topo
}

func rewriteIpv6PktsWithDscp(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config) {

	topo.Flows().Clear()
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	var trafficIds []string

	const max = maxIpv6Tos
	for value := 0; value < max; value++ {
		trafficID := fmt.Sprintf("ipv6-traffic-tos%v", value)
		trafficIds = append(trafficIds, trafficID)
		t.Logf("Configuring flow %s", trafficID)
		flow := topo.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(atePort1.MAC)
		ethHeader.Dst().SetValue(macAddress)
		ipHeader := flow.Packet().Add().Ipv6()
		ipHeader.Src().SetValue(atePort1.IPv6)
		ipHeader.Dst().SetValue(atePort2.IPv6)
		ipHeader.TrafficClass().SetValue(uint32(value))
		flow.Size().SetFixed(uint32(frameSize))
		flow.Rate().SetPps(packetPerSecond)
		flow.Duration().FixedPackets().SetPackets(packetPerSecond)
	}

	ate.OTG().PushConfig(t, topo)

	intialpacket1 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "0")
	intialpacket2 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "1")
	intialpacket3 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "2")
	intialpacket4 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "3")
	intialpacket5 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "4")
	intialpacket6 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV4, "6")

	startCapture(t, ate)
	trafficStartStop(t, ate, topo, "ipv6-traffic-tos0")
	stopCapture(t, ate)

	for _, trafficID := range trafficIds {
		t.Logf("Verify Traffic flow %s", trafficID)
		verifyTrafficFlow(t, ate, trafficID)
	}
	verifyIpv6DscpCapture(t, ate, "port2")

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

func rewriteIpv4PktsWithDscp(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config) {
	type trafficData struct {
		frameSize uint32
		dscp      uint8
		queue     string
		inputIntf attrs.Attributes
	}

	queues := netutil.CommonTrafficQueues(t, dut)
	IngressIPv4TrafficFlows := map[string]*trafficData{
		"intf1-nc1-ipv4": {
			frameSize: frameSize,
			dscp:      6,
			queue:     queues.NC1,
			inputIntf: atePort1,
		},
		"intf1-nc1-ipv4_dscp7": {
			frameSize: frameSize,
			dscp:      7,
			queue:     queues.NC1,
			inputIntf: atePort1,
		},
		"intf1-af4-ipv4": {
			frameSize: frameSize,
			dscp:      4,
			queue:     queues.AF4,
			inputIntf: atePort1,
		},
		"intf1-af4-ipv4_dscp5": {
			frameSize: frameSize,
			dscp:      5,
			queue:     queues.AF4,
			inputIntf: atePort1,
		},
		"intf1-af3-ipv4": {
			frameSize: frameSize,
			dscp:      3,
			queue:     queues.AF3,
			inputIntf: atePort1,
		},
		"intf1-af2-ipv4": {
			frameSize: frameSize,
			dscp:      2,
			queue:     queues.AF2,
			inputIntf: atePort1,
		},
		"intf1-af1-ipv4": {
			frameSize: frameSize,
			dscp:      1,
			queue:     queues.AF1,
			inputIntf: atePort1,
		},
		"intf1-be1-ipv4": {
			frameSize: frameSize,
			dscp:      0,
			queue:     queues.BE1,
			inputIntf: atePort1,
		},
	}

	topo.Flows().Clear()
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())

	for trafficID, data := range IngressIPv4TrafficFlows {
		t.Logf("Configuring flow %s", trafficID)
		flow := topo.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{data.inputIntf.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(data.inputIntf.MAC)
		ethHeader.Dst().SetValue(macAddress)
		ipHeader := flow.Packet().Add().Ipv4()
		ipHeader.Src().SetValue(data.inputIntf.IPv4)
		ipHeader.Dst().SetValue(atePort2.IPv4)
		ipHeader.Priority().Dscp().Phb().SetValue(uint32(data.dscp))
		flow.Size().SetFixed(uint32(data.frameSize))
		flow.Rate().SetPps(packetPerSecond)
		flow.Duration().FixedPackets().SetPackets(packetPerSecond)
	}

	ate.OTG().PushConfig(t, topo)

	intialpacket1 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "0")
	intialpacket2 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "1")
	intialpacket3 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "2")
	intialpacket4 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "3")
	intialpacket5 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "4")
	intialpacket6 := verfiy_classifier_packets(t, dut, oc.Input_Classifier_Type_IPV6, "6")

	startCapture(t, ate)
	trafficStartStop(t, ate, topo, "intf1-nc1-ipv4")
	stopCapture(t, ate)
	for trafficID, _ := range IngressIPv4TrafficFlows {
		t.Logf("Verify Traffic flow %s", trafficID)
		verifyTrafficFlow(t, ate, trafficID)
	}
	verifyIpv4DscpCapture(t, ate, "port2")

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

func rewriteMplsSwapAction(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config) {

	topo.Flows().Clear()
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())

	t.Logf("Configuring flow for MPLS Swap Action")
	flow := topo.Flows().Add().SetName("MplsSwap")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().SetValue(macAddress)

	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(mplsSwapLabel)
	mpls.TrafficClass().SetValue(uint32(5))

	ipHeader := flow.Packet().Add().Ipv4()
	ipHeader.Src().SetValue(atePort1.IPv4)
	ipHeader.Dst().SetValue(atePort2.IPv4)
	flow.Size().SetFixed(uint32(frameSize))
	flow.Rate().SetPps(packetPerSecond)
	flow.Duration().FixedPackets().SetPackets(packetPerSecond)

	ate.OTG().PushConfig(t, topo)

	t.Logf("Configuring MPLS Swap Action with DUT")

	cfgplugins.NewStaticMplsLspSwapLabel(t, dut, "lsp-swap", mplsSwapLabel, atePort2.IPv4, mplsSwapLabelTo, lspNextHopIndex)

	startCapture(t, ate)
	trafficStartStop(t, ate, topo, "MplsSwap")
	stopCapture(t, ate)
	t.Logf("Verify Traffic flow MplsSwap")
	verifyTrafficFlow(t, ate, "MplsSwap")
	verifyMplsSwapPushCapture(t, ate, "port2", mplsSwapLabelTo, true)

	cfgplugins.RemoveStaticMplsLspSwapLabel(t, dut, "lsp-swap", mplsSwapLabel, atePort2.IPv4, mplsSwapLabelTo, lspNextHopIndex)

}

func rewriteIpv4MplsPopAction(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config) {

	topo.Flows().Clear()
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())

	t.Logf("Configuring flow for MPLS V4 POP Action")
	flow := topo.Flows().Add().SetName("MplsPopV4")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().SetValue(macAddress)

	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(mplsPopLabelv4)

	ipHeader := flow.Packet().Add().Ipv4()
	ipHeader.Src().SetValue(atePort1.IPv4)
	ipHeader.Dst().SetValue(atePort2.IPv4)
	flow.Size().SetFixed(uint32(frameSize))
	flow.Rate().SetPps(packetPerSecond)
	flow.Duration().FixedPackets().SetPackets(packetPerSecond)

	ate.OTG().PushConfig(t, topo)

	t.Logf("Configuring MPLS POP Action for IPv4 with DUT")

	cfgplugins.NewStaticMplsLspPopLabel(t, dut, "lsp-pop", mplsPopLabelv4, "", atePort2.IPv4, "ipv4")

	startCapture(t, ate)
	trafficStartStop(t, ate, topo, "MplsPopV4")
	stopCapture(t, ate)
	t.Logf("Verify Traffic flow MplsPopV4")
	verifyTrafficFlow(t, ate, "MplsPopV4")

	verifyMplsPopCapture(t, ate, "port2")

	cfgplugins.RemoveStaticMplsLspPopLabel(t, dut, "lsp-pop", mplsPopLabelv4, "", atePort2.IPv4, "ipv4")

}

func rewriteIpv6MplsPopAction(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config) {

	topo.Flows().Clear()
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())

	t.Logf("Configuring flow for MPLS V4 POP Action")
	flow := topo.Flows().Add().SetName("MplsPopV6")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().SetValue(macAddress)

	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(mplsPopLabelv6)
	ip6 := flow.Packet().Add().Ipv6()
	ip6.Src().SetValue(atePort1.IPv6)
	ip6.Dst().SetValue(atePort2.IPv6)

	flow.Size().SetFixed(uint32(frameSize))
	flow.Rate().SetPps(packetPerSecond)
	flow.Duration().FixedPackets().SetPackets(packetPerSecond)

	ate.OTG().PushConfig(t, topo)

	t.Logf("Configuring MPLS POP Action for IPv6 with DUT")

	cfgplugins.NewStaticMplsLspPopLabel(t, dut, "lsp-pop-v6", mplsPopLabelv6, "", atePort2.IPv6, "ipv6")

	startCapture(t, ate)
	trafficStartStop(t, ate, topo, "MplsPopV6")
	stopCapture(t, ate)
	t.Logf("Verify Traffic flow MplsPopV6")
	verifyTrafficFlow(t, ate, "MplsPopV6")
	verifyMplsPopCapture(t, ate, "port2")

	cfgplugins.RemoveStaticMplsLspPopLabel(t, dut, "lsp-pop-v6", mplsPopLabelv6, "", atePort2.IPv6, "ipv6")

}

func rewriteIpv4MplsPushAction(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config) {

	dp1 := dut.Port(t, "port1")
	topo.Flows().Clear()
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())

	t.Logf("Configuring flow for MPLS V4 PUSH Action")
	flow := topo.Flows().Add().SetName("MplsPushV4")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().SetValue(macAddress)
	ipHeader := flow.Packet().Add().Ipv4()
	ipHeader.Src().SetValue(atePort1.IPv4)
	ipHeader.Dst().SetValue(ipv4DestAddr)
	flow.Size().SetFixed(uint32(frameSize))
	flow.Rate().SetPps(packetPerSecond)
	flow.Duration().FixedPackets().SetPackets(packetPerSecond)
	ate.OTG().PushConfig(t, topo)

	t.Logf("Configuring MPLS Push Action with DUT")

	cfgplugins.NewStaticMplsLspPushLabel(t, dut, "mpls-lsp-push", dp1.Name(), atePort2.IPv4, ipv4DestAddrWithCidr,
		mplsPushLabelV4, lspNextHopIndex, "ipv4")

	startCapture(t, ate)
	trafficStartStop(t, ate, topo, "MplsPushV4")
	stopCapture(t, ate)

	t.Logf("Verify Traffic flow MplsPushV4")
	verifyTrafficFlow(t, ate, "MplsPushV4")
	verifyMplsSwapPushCapture(t, ate, "port2", mplsPushLabelV4, false)

	cfgplugins.RemoveStaticMplsLspPushLabel(t, dut, "mpls-lsp-push", dp1.Name())

}

func rewriteIpv6MplsPushAction(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config) {

	dp1 := dut.Port(t, "port1")
	topo.Flows().Clear()
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())

	t.Logf("Configuring flow for MPLS V6 PUSH Action")
	flow := topo.Flows().Add().SetName("MplsPushV6")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().SetValue(macAddress)
	ip6 := flow.Packet().Add().Ipv6()
	ip6.Src().SetValue(atePort1.IPv6)
	ip6.Dst().SetValue(ipv6DestAddr)
	flow.Size().SetFixed(uint32(frameSize))
	flow.Rate().SetPps(packetPerSecond)
	flow.Duration().FixedPackets().SetPackets(packetPerSecond)

	ate.OTG().PushConfig(t, topo)

	t.Logf("Configuring MPLS Push Action with DUT")

	cfgplugins.NewStaticMplsLspPushLabel(t, dut, "mpls-lsp-push-ipv6", dp1.Name(), atePort2.IPv6, ipv6DestAddrWithCidr,
		mplsPushLabelV6, lspNextHopIndex, "ipv6")

	startCapture(t, ate)
	trafficStartStop(t, ate, topo, "MplsPushV6")
	stopCapture(t, ate)

	t.Logf("Verify Traffic flow MplsPushV6")
	verifyTrafficFlow(t, ate, "MplsPushV6")

	verifyMplsSwapPushCapture(t, ate, "port2", mplsPushLabelV6, false)

	cfgplugins.RemoveStaticMplsLspPushLabel(t, dut, "mpls-lsp-push-ipv6", dp1.Name())
}

func trafficStartStop(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string) {
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

func verifyMplsSwapPushCapture(t *testing.T, ate *ondatra.ATEDevice, port string, expLabel int, checkExp bool) {
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	found := false
	foundExp := false
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if mplsLayer := packet.Layer(layers.LayerTypeMPLS); mplsLayer != nil {
			label, _ := mplsLayer.(*layers.MPLS)
			labelValue := label.Label
			if labelValue == uint32(expLabel) {
				found = true
				t.Logf("Mpls Label Swapped/Pushed as expected, Got: %v", labelValue)
				if checkExp {
					expLabel = int(label.TrafficClass)
					if expLabel == 4 {
						foundExp = true
						t.Logf("Mpls EXP bit remarked as expected, Got: %v", expLabel)
					} else {
						t.Fatalf("Mpls Exp bit not remarked as expected, Got: %v", expLabel)
					}
				}
				break
			}

		}

	}
	if !found {
		t.Fatalf(" MPLS swap did not occur %v", expLabel)
	}
	if checkExp {
		if !foundExp {
			t.Fatalf(" MPLS EXP bit not remarked")
		}
	}

}

func verifyMplsPopCapture(t *testing.T, ate *ondatra.ATEDevice, port string) {
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	found := false
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if mplsLayer := packet.Layer(layers.LayerTypeMPLS); mplsLayer != nil {
			found = true
		}
	}
	if found {
		t.Fatalf(" MPLS POP did not occur")
	}
}

func verifyIpv4DscpCapture(t *testing.T, ate *ondatra.ATEDevice, port string) {
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
				if dscpValue == 5 {
					t.Fatalf("Error: DSCP value 5 should be converted to 4 by ingress DUT")
				} else if dscpValue == 7 {
					t.Fatalf("Error: DSCP value 7 should be converted to 6 by ingress DUT")
				}
			}
		}
	}
}

func verifyIpv6DscpCapture(t *testing.T, ate *ondatra.ATEDevice, port string) {
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	var dscpValuesToConvert []int
	for i := 1; i <= maxIpv6Tos; i++ {
		if i%8 != 0 {
			dscpValuesToConvert = append(dscpValuesToConvert, i)
		}
	}

	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv6)
			if ip.SrcIP.Equal(net.ParseIP(atePort1.IPv6)) {
				dscpValue := ip.TrafficClass >> 2
				if contains(dscpValuesToConvert, int(dscpValue)) {
					t.Fatalf("Error: DSCP value %v should be converted by ingress DUT but not converted", dscpValue)
				}
			}
		}
	}
}

func contains(arr []int, target int) bool {
	for _, element := range arr {
		if element == target {
			return true
		}
	}
	return false
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