/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package ni_address_families_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func assignPort(t *testing.T, d *oc.Root, intf, niName string, a *attrs.Attributes, dut *ondatra.DUTDevice) {
	t.Helper()
	ni := d.GetOrCreateNetworkInstance(niName)
	if niName != deviations.DefaultNetworkInstance(dut) {
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	}
	if niName != deviations.DefaultNetworkInstance(dut) || deviations.ExplicitInterfaceInDefaultVRF(dut) {
		niIntf := ni.GetOrCreateInterface(intf)
		niIntf.Interface = ygot.String(intf)
		niIntf.Subinterface = ygot.Uint32(0)
	}

	// For vendors that require n/w instance definition and interface in
	// a n/w instance set before the address configuration, set nwInstance +
	// interface creation in the nwInstance first.
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		gnmi.Update(t, dut, gnmi.OC().Config(), d)
	}

	ocInt := a.ConfigOCInterface(&oc.Interface{}, dut)
	ocInt.Name = ygot.String(intf)

	if err := d.AppendInterface(ocInt); err != nil {
		t.Fatalf("AddInterface(%v): cannot configure interface %s, %v", ocInt, intf, err)
	}
}

func unassignPort(t *testing.T, dut *ondatra.DUTDevice, intf, niName string) {
	t.Helper()
	// perform unassignment only for non-default VRFs unless ExplicitInterfaceInDefaultVRF deviation is enabled
	if niName == deviations.DefaultNetworkInstance(dut) && !deviations.ExplicitInterfaceInDefaultVRF(dut) {
		return
	}

	in := gnmi.OC().NetworkInstance(niName).Interface(intf).Config()
	gnmi.Delete(t, dut, in)
}

var (
	dutPort1 = &attrs.Attributes{
		IPv4:    "192.0.2.0",
		IPv4Len: 31,
		IPv6:    "2001:db8::1",
		IPv6Len: 64,
	}
	dutPort2 = &attrs.Attributes{
		IPv4:    "192.0.2.2",
		IPv4Len: 31,
		IPv6:    "2001:db8:1::1",
		IPv6Len: 64,
	}
	atePort1 = &attrs.Attributes{
		Name:    "port1",
		IPv4:    "192.0.2.1",
		IPv4Len: 31,
		IPv6:    "2001:db8::2",
		IPv6Len: 64,
		MAC:     "02:00:01:01:01:01",
	}
	atePort2 = &attrs.Attributes{
		Name:    "port2",
		IPv4:    "192.0.2.3",
		IPv4Len: 31,
		IPv6:    "2001:db8:1::2",
		IPv6Len: 64,
		MAC:     "02:00:02:01:01:01",
	}
)

// TestDefaultAddressFamilies verifies that both IPv4 and IPv6 are enabled by default without a need for additional
// configuration within a network instance. It does so by validating that simple IPv4 and IPv6 flows do not experience
// loss.
func TestDefaultAddressFamilies(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	atePort1.AddToOTG(top, p1, dutPort1)
	atePort2.AddToOTG(top, p2, dutPort2)
	// Create an IPv4 flow between ATE port 1 and ATE port 2.
	v4Flow := top.Flows().Add().SetName("ipv4")
	v4Flow.Metrics().SetEnable(true)
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	v4Flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.IPv4", atePort1.Name)}).SetRxNames([]string{fmt.Sprintf("%s.IPv4", atePort2.Name)})
	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().SetValue(atePort2.IPv4)

	// Create an IPv6 flow between ATE port 1 and ATE port 2.
	v6Flow := top.Flows().Add().SetName("ipv6")
	v6Flow.Metrics().SetEnable(true)
	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(atePort1.MAC)
	v6Flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.IPv6", atePort1.Name)}).SetRxNames([]string{fmt.Sprintf("%s.IPv6", atePort2.Name)})
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(atePort1.IPv6)
	v6.Dst().SetValue(atePort2.IPv6)

	ate.OTG().PushConfig(t, top)

	cases := []struct {
		desc   string
		niName string
	}{
		{
			desc:   "Default network instance",
			niName: deviations.DefaultNetworkInstance(dut),
		},
		{
			desc:   "Non default network instance",
			niName: "xyz",
		},
	}
	dutP1 := dut.Port(t, "port1")
	dutP2 := dut.Port(t, "port2")
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if deviations.ExplicitPortSpeed(dut) {
				fptest.SetPortSpeed(t, dutP1)
				fptest.SetPortSpeed(t, dutP2)
			}

			if tc.niName == deviations.DefaultNetworkInstance(dut) {
				fptest.ConfigureDefaultNetworkInstance(t, dut)
			}

			d := &oc.Root{}
			// Assign two ports into the network instance & unnasign them at the end of the test
			assignPort(t, d, dutP1.Name(), tc.niName, dutPort1, dut)
			defer unassignPort(t, dut, dutP1.Name(), tc.niName)

			assignPort(t, d, dutP2.Name(), tc.niName, dutPort2, dut)
			defer unassignPort(t, dut, dutP2.Name(), tc.niName)

			fptest.LogQuery(t, "test configuration", gnmi.OC().Config(), d)
			gnmi.Update(t, dut, gnmi.OC().Config(), d)

			ate.OTG().StartProtocols(t)
			otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
			otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

			ate.OTG().StartTraffic(t)
			time.Sleep(15 * time.Second)
			ate.OTG().StopTraffic(t)

			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			otgutils.LogPortMetrics(t, ate.OTG(), top)

			// Check that we did not lose any packets for the IPv4 and IPv6 flows.
			for _, flow := range []string{"ipv4", "ipv6"} {
				m := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow).State())
				tx := float32(m.GetCounters().GetOutPkts())
				rx := float32(m.GetCounters().GetInPkts())
				if tx == 0 {
					t.Fatalf("TxPkts == 0, want > 0")
				}
				loss := tx - rx
				lossPct := loss * 100 / tx
				if got := lossPct; got > 0 {
					t.Errorf("LossPct for flow %s: got %v, want 0", flow, got)
				}
			}
			ate.OTG().StopProtocols(t)
		})
	}
}
