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

package fptest

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// ConfigureDefaultNetworkInstance configures the default network instance name and type.
func ConfigureDefaultNetworkInstance(t testing.TB, d *ondatra.DUTDevice) {
	defNiPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(d))
	gnmi.Update(t, d, defNiPath.Config(), &oc.NetworkInstance{
		Name: ygot.String(deviations.DefaultNetworkInstance(d)),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
	})
}

// AssignToNetworkInstance attaches a subinterface to a network instance.
func AssignToNetworkInstance(t testing.TB, d *ondatra.DUTDevice, i string, ni string, si uint32) {
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
	netInstIntf.Id = ygot.String(intf.GetName() + "." + fmt.Sprint(si))
	if intf.GetOrCreateSubinterface(si) != nil {
		gnmi.Update(t, d, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}

// EnableGRIBIUnderNetworkInstance enables GRIBI protocol under network instance.
func EnableGRIBIUnderNetworkInstance(t testing.TB, d *ondatra.DUTDevice, ni string) {
	t.Helper()
	if ni == "" {
		t.Fatalf("Network instance not provided for gRIBI protocol definition")
	}

	switch d.Vendor() {
	case ondatra.NOKIA:
		gpbSetRequest := &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "srl",
			},
			Update: []*gpb.Update{{
				Path: &gpb.Path{
					Elem: []*gpb.PathElem{
						{
							Name: "network-instance",
							Key:  map[string]string{"name": ni},
						},
						{
							Name: "protocols",
						},
						{
							Name: "gribi",
						},
						{
							Name: "admin-state",
						},
					},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonIetfVal{
						JsonIetfVal: []byte(`"enable"`),
					},
				},
			}},
		}
		gnmiClient := d.RawAPIs().GNMI(t)
		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("Enabling Gribi on network-instance %s failed with unexpected error: %v", ni, err)
		}
	default:
		t.Fatalf("Vendor %s does not support 'deviation_explicit_gribi_under_network_instance'", d.Vendor())
	}
}
