// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sflow_base_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 30
	frameSize     = 512      // size of packets in bytes to generate with ATE
	packetsToSend = 10000000 // 10 million
	ppsRate       = 1000000  // 1 million
	plenIPv4      = 30
	plenIPv6      = 126
	lossTolerance = 0
)

var (
	staticRoute = &cfgplugins.StaticRouteCfg{
		NetworkInstance: "DEFAULT",
		Prefix:          "192.0.2.128/30",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString("192.0.2.6"),
		},
	}
	dutSrc = &attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::2",
		IPv6Len: plenIPv6,
	}
	dutDst = &attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::5",
		IPv6Len: plenIPv6,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configInterfaceDUT configures the DUT interfaces.
func configInterfaceDUT(p1 *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {

	i := &oc.Interface{Name: ygot.String(p1.Name())}

	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4.GetOrCreateAddress(a.IPv4).PrefixLength = ygot.Uint8(plenIPv4)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plenIPv6)

	return i
}

// configureDUTBaseline configures port1 and port2 on the DUT.
func configureDUTBaseline(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutSrc, dut))

	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutDst, dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// TestSFlowTraffic configures a DUT for sFlow client and collector endpoint and uses ATE to send
// traffic which the DUT should sample and send sFlow packets to a collector.  ATE captures the
// sflow packets which are decoded by the test to verify they are valid sflow packets.
func TestSFlowTraffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// configure interfaces on DUT
	// TODO: consider refactoring interface configs into cfgplugins
	configureDUTBaseline(t, dut)

	srBatch := &gnmi.SetBatch{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	cfgplugins.NewStaticRouteCfg(srBatch, staticRoute, dut)
	srBatch.Set(t, dut)

	sfBatch := &gnmi.SetBatch{}
	cfgplugins.NewSFlowGlobalCfg(sfBatch, nil, dut)
	cfgplugins.NewSFlowCollector(sfBatch, nil, dut)

	t.Run("SFLOW-1.1_ReplaceDUTConfigSFlow", func(t *testing.T) {
		sfBatch.Set(t, dut)

		gotSamplingConfig := gnmi.Get(t, dut, gnmi.OC().Sampling().Sflow().Config())
		json, err := ygot.EmitJSON(gotSamplingConfig, &ygot.EmitJSONConfig{
			Format: ygot.RFC7951,
			Indent: "  ",
			RFC7951Config: &ygot.RFC7951JSONConfig{
				AppendModuleName: true,
			},
		})
		if err != nil {
			t.Errorf("Error decoding sampling config: %v", err)
		}
		t.Logf("Got sampling config: %v", json)
	})
	/* TODO: implement this when a suitable ygot.diffBatch function exists
		// Validate DUT sampling config matches what we set it to
		diff, err := ygot.Diff(gotSamplingConfig, sfBatch)
		if err != nil {
			t.Errorf("Error attempting to compare sflow config: %v", err.Error())
		}
		if diff.String() != "" {
			t.Errorf("Want empty string, got: %v", helpers.GNMINotifString(diff))
		}
	})
	*/
	// TODO: Configure ATE
	// TODO: Send traffic, capture and decode
}
