// Copyright 2025 Google LLC
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

package egress_strict_priority_scheduler_with_bursty_traffic_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

var (
	dutIngressPort1AteP1 = attrs.Attributes{IPv4: "198.51.100.1", IPv4Len: 30}
	dutIngressPort2AteP2 = attrs.Attributes{IPv4: "198.51.100.5", IPv4Len: 30}
	dutEgressPort3AteP3  = attrs.Attributes{IPv4: "198.51.100.9", IPv4Len: 30}

	ateTxP1 = attrs.Attributes{Name: "ate1", MAC: "0f:99:f6:9c:81:01", IPv4: "198.51.100.2", IPv4Len: 30}
	ateTxP2 = attrs.Attributes{Name: "ate2", MAC: "0f:99:f6:9c:81:02", IPv4: "198.51.100.6", IPv4Len: 30}
	ateRxP3 = attrs.Attributes{Name: "ate3", MAC: "0f:99:f6:9c:81:03", IPv4: "198.51.100.10", IPv4Len: 30}
)

// temporary replacement for trafficRate w/trafficPps for functional testing
// unfortunately, the pkt size in the test definition does not seem to be taken into account hence the variation
// between synthetic (same-pkt-size) traffic and real traffic can be significant as well as the QoS scheduling
// engine behavior

type trafficData struct {
	trafficPps            uint32
	expectedThroughputPct float32
	frameSize             uint32
	dscp                  uint8
	queue                 string
	inputIntf             attrs.Attributes
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBasicConfigWithTraffic(t *testing.T) {
	t.Logf("Test started. We're now in TestBasicConfigWithTraffic")
	dut := ondatra.DUT(t, "dut")

	// Configure DUT interfaces and QoS.
	ConfigureDUTIntf(t, dut)
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
		desc:      "DUT input intf port1",
		intfName:  dp1.Name(),
		ipAddr:    dutIngressPort1AteP1.IPv4,
		prefixLen: dutIngressPort1AteP1.IPv4Len,
	}, {
		desc:      "DUT input intf port2",
		intfName:  dp2.Name(),
		ipAddr:    dutIngressPort2AteP2.IPv4,
		prefixLen: dutIngressPort1AteP1.IPv4Len,
	}, {
		desc:      "DUT output intf port3",
		intfName:  dp3.Name(),
		ipAddr:    dutEgressPort3AteP3.IPv4,
		prefixLen: dutIngressPort1AteP1.IPv4Len,
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
			t.Logf("DUT %s %s %s requires interface enable deviation ", dut.Vendor(), dut.Model(), dut.Version())
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, intf.intfName, deviations.DefaultNetworkInstance(dut), 0)
			t.Logf("DUT %s %s %s requires explicit interface in default VRF deviation ", dut.Vendor(), dut.Model(), dut.Version())
		}
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
		fptest.SetPortSpeed(t, dp2)
		fptest.SetPortSpeed(t, dp3)
		t.Logf("DUT %s %s %s requires explicit port speed set deviation ", dut.Vendor(), dut.Model(), dut.Version())
	}

	t.Logf("Wait period reached - please make your GNMI requests now")
	time.Sleep(time.Minute * 5)
}
