// Copyright 2022 Google LLC
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

// Package topology_test configures just the ports on DUT and ATE,
// assuming that DUT port i is connected to ATE i.  It detects the
// number of ports in the testbed and can be used with the 2, 4, 12
// port variants of the atedut testbed.
package topology_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// plen is the IPv4 prefix length used for IPv4 assignments in this
// topology.
const plen = 30

// dutPortIP assigns IP addresses for DUT port i, where i is the index
// of the port slices returned by dut.Ports().
func dutPortIP(i int) string {
	return fmt.Sprintf("192.0.2.%d", i*4+1)
}

// atePortCIDR assigns IP addresses with prefixlen for ATE port i, where
// i is the index of the port slices returned by ate.Ports().
func atePortCIDR(i int) string {
	return fmt.Sprintf("192.0.2.%d/30", i*4+2)
}

func atePortMac(i int) string {
	return fmt.Sprintf("00:00:%02x:01:01:01", i*4+2)
}

func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.Slice(ports, func(i, j int) bool {
		idi, idj := ports[i].ID(), ports[j].ID()
		li, lj := len(idi), len(idj)
		if li == lj {
			return idi < idj
		}
		return li < lj // "port2" < "port10"
	})
	return ports
}

func configInterface(name, desc, ipv4 string, prefixlen uint8) *telemetry.Interface {
	i := &telemetry.Interface{}
	i.Name = ygot.String(name)
	i.Description = ygot.String(desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd

	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()

	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}

	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return i
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	var (
		badReplace []string
		badConfig  []string
		badTelem   []string
	)

	d := dut.Config()

	// TODO(liulk): configure breakout ports when Ondatra is able to
	// specify them in the testbed for reservation.

	sortedDutPorts := sortPorts(dut.Ports())
	for i, dp := range sortedDutPorts {
		di := d.Interface(dp.Name())
		in := configInterface(dp.Name(), dp.String(), dutPortIP(i), plen)
		fptest.LogYgot(t, fmt.Sprintf("%s to Replace()", dp), di, in)
		if ok := fptest.NonFatal(t, func(t testing.TB) { di.Replace(t, in) }); !ok {
			badReplace = append(badReplace, dp.Name())
		}
	}

	for _, dp := range dut.Ports() {
		di := d.Interface(dp.Name())
		if q := di.Lookup(t); q.IsPresent() {
			fptest.LogYgot(t, fmt.Sprintf("%s from Get()", dp), di, q.Val(t))
		} else {
			badConfig = append(badConfig, dp.Name())
			t.Errorf("Config %v Get() failed", di)
		}
	}

	dt := dut.Telemetry()
	for _, dp := range dut.Ports() {
		di := dt.Interface(dp.Name())
		if q := di.Lookup(t); q.IsPresent() {
			fptest.LogYgot(t, fmt.Sprintf("%s from Get()", dp), di, q.Val(t))
		} else {
			badTelem = append(badTelem, dp.Name())
			t.Errorf("Telemetry %v Get() failed", di)
		}
	}

	t.Logf("Replace issues on interfaces: %v", badReplace)
	t.Logf("Config Get issues on interfaces: %v", badConfig)
	t.Logf("Telemetry Get issues on interfaces: %v", badTelem)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otg := ate.OTG()
	sortedAtePorts := sortPorts(ate.Ports())
	top := otg.NewConfig(t)
	for i, ap := range sortedAtePorts {
		t.Logf("OTG AddInterface: ports[%d] = %v", i, ap)
		in := top.Ports().Add().SetName(ap.ID())
		dev := top.Devices().Add().SetName(ap.Name() + ".dev")
		eth := dev.Ethernets().Add().SetName(ap.Name() + ".eth").
			SetPortName(in.Name()).SetMac(atePortMac(i))
		ipv4Addr := strings.Split(atePortCIDR(i), "/")[0]
		eth.Ipv4Addresses().Add().SetName(dev.Name() + ".ipv4").
			SetAddress(ipv4Addr).SetGateway(dutPortIP(i)).
			SetPrefix(int32(plen))
	}
	otg.PushConfig(t, top)
	t.Logf("Start ATE Protocols")
	otg.StartProtocols(t)
	return top
}

func TestTopology(t *testing.T) {
	// Configure the DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "otg")
	configureATE(t, ate)

	// Query Telemetry
	dutPorts := sortPorts(dut.Ports())
	t.Run("Telemetry", func(t *testing.T) {
		const want = telemetry.Interface_OperStatus_UP

		dt := dut.Telemetry()
		for _, dp := range dutPorts {
			if got := dt.Interface(dp.Name()).OperStatus().Get(t); got != want {
				t.Errorf("%s oper-status got %v, want %v", dp, got, want)
			}
		}

	})
}
