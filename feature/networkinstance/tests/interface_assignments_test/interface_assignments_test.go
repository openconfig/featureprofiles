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

package interface_assignments_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
  "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// intfNIAssignment describes a set of interfaces assigned to a network instance.
type intfNIAssignment struct {
	// Ports is the set of ports that are to be assigned to the network instance.
	Ports []portSpec
	// Type is the type specified in the configuration.
	Type telemetry.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE
}

// portSpec describes a specific interface, subinterface
type portSpec struct {
	// Name is the name of the port in the testbed.
	Name string
	// Subintf is the index of the subinterface on the device.
	Subintf uint32
	// IPv4 is the IPv4 address that should be assigned to the interface.
	IPv4 string
	// PrefixLength is the prefix length for the IPv4 address.
	PrefixLength uint8
}

// TestInterfaceAssignment tests the assignment of an interface explicitly to
// a network instance.
func TestInterfaceAssignment(t *testing.T) {
	tests := []struct {
		desc             string
		inPreAssignments map[string]intfNIAssignment
		inAssignments    map[string]intfNIAssignment
		wantErr          bool
	}{{
		desc: "explicit assignment of port1 to DEFAULT",
		inAssignments: map[string]intfNIAssignment{
			"DEFAULT": {
				Ports: []portSpec{{
					Name:    "port1",
					Subintf: 0,
				}},
				Type: telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
			},
		},
	}, {
		desc: "explicit assignment of port1 to non-default NI",
		inAssignments: map[string]intfNIAssignment{
			"BLUE": {
				Ports: []portSpec{{
					Name:    "port1",
					Subintf: 0,
				}},
				Type: telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
			},
		},
	}, {
		desc: "assignment to a NI with no type - invalid",
		inAssignments: map[string]intfNIAssignment{
			"RED": {
				Ports: []portSpec{{
					Name:    "port1",
					Subintf: 0,
				}},
			},
		},
		wantErr: true,
	}, {
		desc: "move assignment from BLUE to RED",
		inPreAssignments: map[string]intfNIAssignment{
			"BLUE": {
				Ports: []portSpec{{
					Name:    "port1",
					Subintf: 0,
				}},
				Type: telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
			},
		},
		wantErr: true,
		inAssignments: map[string]intfNIAssignment{
			"RED": {
				Ports: []portSpec{{
					Name:    "port1",
					Subintf: 0,
				}},
				Type: telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
			},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			dut := ondatra.DUT(t, "dut")

			assignPort := func(t *testing.T, d *telemetry.Device, niName string, p portSpec) {
				dp := dut.Port(t, p.Name)
				ni := d.GetOrCreateNetworkInstance(niName)

				// Create the interface to ensure we have no missing references.
				intf := d.GetOrCreateInterface(dp.Name())
				intf.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
				intf.GetOrCreateSubinterface(p.Subintf).
					GetOrCreateIpv4().GetOrCreateAddress(p.IPv4).PrefixLength = &p.PrefixLength

				// Assign the interface to a network instance.
				i := ni.GetOrCreateInterface(p.Name)
				i.Interface = ygot.String(dp.Name())
				i.Subinterface = ygot.Uint32(p.Subintf)
			}

			initialDev := &telemetry.Device{}
			for niName, spec := range tt.inPreAssignments {
				ni := initialDev.GetOrCreateNetworkInstance(niName)
				ni.Type = spec.Type

				for _, p := range spec.Ports {
					assignPort(t, initialDev, niName, p)
				}
			}

			d := &telemetry.Device{}
			for niName, spec := range tt.inAssignments {
				ni := d.GetOrCreateNetworkInstance(niName)
				ni.Type = spec.Type

				for _, p := range spec.Ports {
					assignPort(t, d, niName, p)
				}
			}

			if got := testt.ExpectFatal(t, func(t testing.TB) {
				dut.Config().Update(t, d)
			}); tt.wantErr && got != "" || !tt.wantErr && got == "" {
				t.Fatalf("did not get expected Fatal error, got: %s, wantErr? %v", got, tt.wantErr)
			}

			// Clean up the test by removing explicit assignments that we have made.
			for niName := range tt.inPreAssignments {
				gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(niName).Interface())
			}
			for niName := range tt.inAssignments {
				gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(niName).Interface())
			}
		})
	}
}
