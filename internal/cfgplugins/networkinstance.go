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

package cfgplugins

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// ConfigureDefaultNetworkInstance configures the default network instance name and type.
func ConfigureDefaultNetworkInstance(batch *gnmi.SetBatch, t testing.TB, d *ondatra.DUTDevice) {
	defNiPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(d))
	gnmi.BatchUpdate(batch, defNiPath.Config(), &oc.NetworkInstance{
		Name: ygot.String(deviations.DefaultNetworkInstance(d)),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
	})
}

// ConfigureCustomNetworkInstance configures a non-default network instance name and type.
func ConfigureCustomNetworkInstance(batch *gnmi.SetBatch, t testing.TB, d *ondatra.DUTDevice, ni string) {
	defNiPath := gnmi.OC().NetworkInstance(ni)
	gnmi.BatchUpdate(batch, defNiPath.Config(), &oc.NetworkInstance{
		Name: ygot.String(ni),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	})

}

// AssignToNetworkInstance attaches an interface to a network instance.
func AssignToNetworkInstance(batch *gnmi.SetBatch, t testing.TB, d *ondatra.DUTDevice, i string, ni string, si uint32) {
	t.Helper()
	if ni == "" {
		t.Fatalf("Network instance not provided for interface assignment")
	}
	netInst := &oc.NetworkInstance{Name: ygot.String(ni)}
	intf := &oc.Interface{Name: ygot.String(i)}
	netInstIntf, err := netInst.NewInterface(intf.GetName())
	if err != nil {
		t.Errorf("Error fetching NewInterface for %s", intf.GetName())
	}
	netInstIntf.Interface = ygot.String(intf.GetName())
	netInstIntf.Subinterface = ygot.Uint32(si)
	switch d.Vendor() {
	case ondatra.ARISTA:
		netInstIntf.Id = ygot.String(intf.GetName())
	case ondatra.CISCO:
		netInstIntf.Id = ygot.String(intf.GetName())
	case ondatra.NOKIA:
		netInstIntf.Id = ygot.String(intf.GetName())
	case ondatra.JUNIPER:
		netInstIntf.Id = ygot.String(intf.GetName() + "." + fmt.Sprint(si))
	default:
		netInstIntf.Id = ygot.String(intf.GetName() + "." + fmt.Sprint(si))
	}
	if intf.GetOrCreateSubinterface(si) != nil {
		gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}
