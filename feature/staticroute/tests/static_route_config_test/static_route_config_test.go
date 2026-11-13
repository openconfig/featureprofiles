// Copyright 2026 Google LLC
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

package static_route_config_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func ethernetInterface(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	for _, intf := range gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State()) {
		name := intf.GetName()
		if strings.HasPrefix(name, "Ethernet") {
			return name
		}
	}
	t.Fatal("no Ethernet interface in state")
	return ""
}

func staticProto(dut *ondatra.DUTDevice) oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE {
	return oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC
}

// TestStaticRouteCreateDelete covers an IP next hop and an interface next hop.
func TestStaticRouteCreateDelete(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	port := ethernetInterface(t, dut)
	ni := deviations.DefaultNetworkInstance(dut)
	proto := deviations.StaticProtocolName(dut)
	const (
		viaIP   = "203.0.113.0/24"
		viaIntf = "203.0.113.10/32"
		nhIP    = "192.0.2.254"
	)
	ipPath := gnmi.OC().NetworkInstance(ni).Protocol(staticProto(dut), proto).Static(viaIP)
	intfPath := gnmi.OC().NetworkInstance(ni).Protocol(staticProto(dut), proto).Static(viaIntf)
	t.Cleanup(func() {
		gnmi.Delete(t, dut, ipPath.Config())
		gnmi.Delete(t, dut, intfPath.Config())
	})

	t.Log("Create a static route with an IP next hop")
	ipRoute := &oc.NetworkInstance_Protocol_Static{Prefix: ygot.String(viaIP)}
	ipNH := ipRoute.GetOrCreateNextHop("0")
	ipNH.NextHop = oc.UnionString(nhIP)
	gnmi.Replace(t, dut, ipPath.Config(), ipRoute)
	gotIP := fmt.Sprint(gnmi.Get(t, dut, ipPath.Config()).GetNextHop("0").GetNextHop())
	if gotIP != nhIP {
		t.Errorf("next-hop = %q, want %q", gotIP, nhIP)
	}

	t.Log("Create a static route with an interface next hop and no next-hop IP")
	intfRoute := &oc.NetworkInstance_Protocol_Static{Prefix: ygot.String(viaIntf)}
	intfNH := intfRoute.GetOrCreateNextHop("if0")
	intfNH.GetOrCreateInterfaceRef().Interface = ygot.String(port)
	gnmi.Replace(t, dut, intfPath.Config(), intfRoute)
	gotIntf := gnmi.Get(t, dut, intfPath.Config()).GetNextHop("if0").GetInterfaceRef().GetInterface()
	if gotIntf != port {
		t.Errorf("interface next hop = %q, want %q", gotIntf, port)
	}
	gnmi.Await(t, dut, ipPath.Prefix().State(), time.Minute, viaIP)

	t.Log("Delete both static routes")
	gnmi.Delete(t, dut, ipPath.Config())
	if gnmi.LookupConfig(t, dut, ipPath.Config()).IsPresent() {
		t.Errorf("static route %s still in config", viaIP)
	}
}
