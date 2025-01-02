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

package ingress_police_default_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

var (
	intf1 = attrs.Attributes{
		Name:    "ate1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "198.51.100.1",
		IPv4Len: 31,
	}

	intf2 = attrs.Attributes{
		Name:    "ate2",
		MAC:     "02:00:01:02:01:01",
		IPv4:    "198.51.100.3",
		IPv4Len: 31,
	}

	dutPort1 = attrs.Attributes{
		IPv4: "198.51.100.0",
	}
	dutPort2 = attrs.Attributes{
		IPv4: "198.51.100.2",
	}
)

type trafficData struct {
	expectedThroughputPct float32
	gbpsRate              uint32
	dscp                  uint8
	queue                 string
	inputIntf             attrs.Attributes
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureIntertfaceIngressPolicerCLI is used to configure vendor specific config statements.
func configureIntertfaceIngressPolicerCLI(t *testing.T, dut *ondatra.DUTDevice) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var defaultPolicyCLI string
		defaultPolicyCLI = fmt.Sprintf("policing \n profile %s rate %d mbps burst-size %d kbytes \n", "DP24", 1000, 1000)
		helpers.GnmiCLIConfig(t, dut, defaultPolicyCLI)
		defaultPolicyCLI = fmt.Sprintf("interface %s \n policer profile dp24 input \n", dut.Port(t, "port1").Name())
		helpers.GnmiCLIConfig(t, dut, defaultPolicyCLI)
	case ondatra.CISCO:
		var defaultPolicyCLI string
		helpers.GnmiCLIConfig(t, dut, "policy-map dp24 \n class class-default \n police 1 gbps \n")
		defaultPolicyCLI = fmt.Sprintf("interface %s \n service-policy input dp24 \n", dut.Port(t, "port1").Name())
		helpers.GnmiCLIConfig(t, dut, defaultPolicyCLI)
	default:
		t.Fatalf("Unsupported vendor %s for native command support for deviation 'GribiEncapHeaderUnsupported'", dut.Vendor())
	}
}

// Test cases:
// 1. Validate that flow experiences 0 packet loss at 0.7Gbps.
// 2. Validate that flow experiences ~50% packet loss (+/- 1%)
func TestInterfaceIngressPolicer(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	// Configure DUT interfaces and QoS.
	ConfigureDUTIntf(t, dut)
	if deviations.GribiEncapHeaderUnsupported(dut) {
		configureIntertfaceIngressPolicerCLI(t, dut)
	}

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	top := gosnappi.NewConfig()

	intf1.AddToOTG(top, ap1, &dutPort1)
	intf2.AddToOTG(top, ap2, &dutPort2)
	ate.OTG().PushConfig(t, top)

	var tolerance float32 = 3.0

	// Validate that flow experiences 0 packet loss at 0.7Gbps.
	TrafficFlows10 := map[string]*trafficData{
		"intf1-be0": {
			gbpsRate:              1,
			expectedThroughputPct: 100.0,
			queue:                 "DEFAULT",
			inputIntf:             intf1,
		},
	}

	// Validate that flow experiences ~50% packet loss.
	oversubscribedTrafficFlows11 := map[string]*trafficData{
		"intf1-be0": {
			gbpsRate:              2,
			expectedThroughputPct: 100.0,
			queue:                 "DEFAULT",
			inputIntf:             intf1,
		},
	}

	type test struct {
		desc            string
		trafficFlows    map[string]*trafficData
		trafficDuration time.Duration
	}

	cases := []test{
		{
			desc:            "Validate that flow experiences 0 packet loss at 0.7Gbps",
			trafficFlows:    TrafficFlows10,
			trafficDuration: 60 * time.Second,
		},
		{
			desc:            "Validate that flow experiences ~50% packet loss at 2Gbps",
			trafficFlows:    oversubscribedTrafficFlows11,
			trafficDuration: 60 * time.Second,
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			trafficFlows := tc.trafficFlows
			top.Flows().Clear()

			for trafficID, data := range trafficFlows {
				t.Logf("Configuring flow %s", trafficID)
				flow := top.Flows().Add().SetName(trafficID)
				flow.Metrics().SetEnable(true)
				flow.TxRx().Device().SetTxNames([]string{data.inputIntf.Name + ".IPv4"}).SetRxNames([]string{intf2.Name + ".IPv4"})
				ethHeader := flow.Packet().Add().Ethernet()
				ethHeader.Src().SetValue(data.inputIntf.MAC)

				ipHeader := flow.Packet().Add().Ipv4()
				ipHeader.Src().SetValue(data.inputIntf.IPv4)
				ipHeader.Dst().SetValue(intf2.IPv4)
				flow.Rate().SetGbps(data.gbpsRate)
			}

			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)
			otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

			ateOutPkts := make(map[string]uint64)
			ateInPkts := make(map[string]uint64)
			dutQosPktsBeforeTraffic := make(map[string]uint64)
			dutQosPktsAfterTraffic := make(map[string]uint64)
			dutQosDroppedPktsBeforeTraffic := make(map[string]uint64)
			dutQosDroppedPktsAfterTraffic := make(map[string]uint64)

			// Set the initial counters to 0.
			for _, data := range trafficFlows {
				ateOutPkts[data.queue] = 0
				ateInPkts[data.queue] = 0
				dutQosPktsBeforeTraffic[data.queue] = 0
				dutQosPktsAfterTraffic[data.queue] = 0
				dutQosDroppedPktsBeforeTraffic[data.queue] = 0
				dutQosDroppedPktsAfterTraffic[data.queue] = 0
			}

			// Get QoS egress packet counters before the traffic.
			const timeout = time.Minute
			isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
			for _, data := range trafficFlows {
				count, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State(), timeout, isPresent).Await(t)
				if !ok {
					t.Logf("TransmitPkts count for queue %q on interface %q not available within %v", dp2.Name(), data.queue, timeout)
				}
				dutQosPktsBeforeTraffic[data.queue], _ = count.Val()

				count, ok = gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).DroppedPkts().State(), timeout, isPresent).Await(t)
				if !ok {
					t.Logf("DroppedPkts count for queue %q on interface %q not available within %v", dp2.Name(), data.queue, timeout)
				}
				dutQosDroppedPktsBeforeTraffic[data.queue], _ = count.Val()
			}

			t.Logf("Running traffic 1 on DUT interfaces: %s => %s ", dp1.Name(), dp2.Name())
			t.Logf("Sending traffic flows: \n%v\n\n", trafficFlows)
			ate.OTG().StartTraffic(t)
			time.Sleep(tc.trafficDuration)
			ate.OTG().StopTraffic(t)
			time.Sleep(10 * time.Second)

			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			for trafficID, data := range trafficFlows {
				ateTxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().OutPkts().State())
				ateRxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(trafficID).Counters().InPkts().State())
				ateOutPkts[data.queue] += ateTxPkts
				ateInPkts[data.queue] += ateRxPkts
				t.Logf("ateInPkts: %v, txPkts %v, Queue: %v", ateInPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)
				if ateTxPkts == 0 {
					t.Fatalf("TxPkts == 0, want >0.")
				}
				lossPct := (float32)((float64(ateTxPkts-ateRxPkts) * 100.0) / float64(ateTxPkts))
				t.Logf("Get flow %q: lossPct: %.2f%% or rxPct: %.2f%%, want: %.2f%%\n\n", data.queue, lossPct, 100.0-lossPct, data.expectedThroughputPct)
				if got, want := 100.0-lossPct, data.expectedThroughputPct; got < want-tolerance || got > want+tolerance {
					t.Errorf("Get(throughput for queue %q): got %.2f%%, want within [%.2f%%, %.2f%%]", data.queue, got, want-tolerance, want+tolerance)
				}
			}
		})
	}
}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	dutIntfs := []struct {
		desc      string
		intfName  string
		ipAddr    string
		prefixLen uint8
	}{{
		desc:      "Input interface port1",
		intfName:  dp1.Name(),
		ipAddr:    dutPort1.IPv4,
		prefixLen: 31,
	}, {
		desc:      "Input interface port2",
		intfName:  dp2.Name(),
		ipAddr:    dutPort2.IPv4,
		prefixLen: 31,
	}}

	// Configure the interfaces.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &oc.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		i.GetOrCreateEthernet()
		s := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s.Enabled = ygot.Bool(true)
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, intf.intfName, deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
		fptest.SetPortSpeed(t, dp2)
		fptest.SetPortSpeed(t, dp3)
	}
}
