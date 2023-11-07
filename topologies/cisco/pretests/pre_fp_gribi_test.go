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

package pre_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen = 30
	ateDstNetCIDR = "198.51.100.0/24"
	nhIndex       = 1
	nhgIndex      = 42
)

var (
	// dutPort1 = attrs.Attributes{
	// 	Desc:    "dutPort1",
	// 	IPv4:    "192.0.2.1",
	// 	IPv4Len: ipv4PrefixLen,
	// }

	// atePort1 = attrs.Attributes{
	// 	Name:    "atePort1",
	// 	IPv4:    "192.0.2.2",
	// 	IPv4Len: ipv4PrefixLen,
	// }

	// dutPort2 = attrs.Attributes{
	// 	Desc:    "dutPort2",
	// 	IPv4:    "192.0.2.5",
	// 	IPv4Len: ipv4PrefixLen,
	// }

	// atePort2 = attrs.Attributes{
	// 	Name:    "atePort2",
	// 	IPv4:    "192.0.2.6",
	// 	IPv4Len: ipv4PrefixLen,
	// }

	// dutPort3 = attrs.Attributes{
	// 	Desc:    "dutPort3",
	// 	IPv4:    "192.0.2.9",
	// 	IPv4Len: ipv4PrefixLen,
	// }

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
	}
)

func TestResetGRIBIServerFP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Configure the gRIBI client clientA
	clientA := gribi.Client{
		DUT:         dut,
		FIBACK:      false,
		Persistence: true,
	}
	defer clientA.Close(t)
	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	clientA.BecomeLeader(t)
	t.Logf("an IPv4Entry for %s pointing to ATE port-3 via gRIBI-B", ateDstNetCIDR)
	clientA.AddNH(t, nhIndex, atePort3.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInRIB)
	clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInRIB)
	clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex, deviations.DefaultNetworkInstance(dut), "", fluent.InstalledInRIB)

	/*nhg := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR).NextHopGroup()

	nhg.Watch(t, 33*time.Second, func(*telemetry.QualifiedUint64) bool {
		// Do nothing in this matching function, as we already filter on the prefix.
		return true
	}).Await(t)

	nhg.Watch(t, 33*time.Second, func(*telemetry.QualifiedUint64) bool {
		// Do nothing in this matching function, as we already filter on the prefix.
		return true
	}).Await(t)
	nhg.Lookup(t)*/

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(ateDstNetCIDR)
	gnmi.Lookup(t, dut, ipv4Path.State())

	gnmi.Watch(t, dut, ipv4Path.State(), 33*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {

		return true
	}).Await(t)

	got, present := gnmi.Lookup(t, dut, ipv4Path.State()).Val()
	if !present && *got.Prefix != ateDstNetCIDR {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", *got.Prefix, ateDstNetCIDR)
	}

	if got, ok := gnmi.Watch(t, dut, ipv4Path.State(), 33*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		return present && ipv4Entry.GetPrefix() == ateDstNetCIDR
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %s, want %s", got, ateDstNetCIDR)
	}

	clientA.FlushAll(t)
}
