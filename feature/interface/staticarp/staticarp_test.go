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

package staticarp

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentSubInterface tests the features of staticarp config.
func TestAugmentSubInterface(t *testing.T) {
	tests := []struct {
		desc   string
		arp    *StaticARP
		inSI   *fpoc.Interface_Subinterface
		wantSI *fpoc.Interface_Subinterface
	}{{
		desc: "ipv4 address",
		arp:  New().AddIPv4Address("192.0.2.1", 32),
		inSI: &fpoc.Interface_Subinterface{},
		wantSI: &fpoc.Interface_Subinterface{
			Ipv4: &fpoc.Interface_Subinterface_Ipv4{
				Address: map[string]*fpoc.Interface_Subinterface_Ipv4_Address{
					"192.0.2.1": {
						Ip:           ygot.String("192.0.2.1"),
						PrefixLength: ygot.Uint8(32),
					},
				},
			},
			Ipv6: &fpoc.Interface_Subinterface_Ipv6{},
		},
	}, {
		desc: "ipv6 address",
		arp:  New().AddIPv6Address("2001:db8:0::1", 128),
		inSI: &fpoc.Interface_Subinterface{},
		wantSI: &fpoc.Interface_Subinterface{
			Ipv4: &fpoc.Interface_Subinterface_Ipv4{},
			Ipv6: &fpoc.Interface_Subinterface_Ipv6{
				Address: map[string]*fpoc.Interface_Subinterface_Ipv6_Address{
					"2001:db8:0::1": {
						Ip:           ygot.String("2001:db8:0::1"),
						PrefixLength: ygot.Uint8(128),
					},
				},
			},
		},
	}, {
		desc: "ipv4 peer",
		arp:  New().AddIPv4Neighbor("192.0.2.1", "52:fe:7c:91:6e:c1"),
		inSI: &fpoc.Interface_Subinterface{},
		wantSI: &fpoc.Interface_Subinterface{
			Ipv4: &fpoc.Interface_Subinterface_Ipv4{
				Neighbor: map[string]*fpoc.Interface_Subinterface_Ipv4_Neighbor{
					"192.0.2.1": {
						Ip:               ygot.String("192.0.2.1"),
						LinkLayerAddress: ygot.String("52:fe:7c:91:6e:c1"),
					},
				},
			},
			Ipv6: &fpoc.Interface_Subinterface_Ipv6{},
		},
	}, {
		desc: "ipv6 peer",
		arp:  New().AddIPv6Neighbor("2001:db8:0::1", "52:fe:7c:91:6e:c1"),
		inSI: &fpoc.Interface_Subinterface{},
		wantSI: &fpoc.Interface_Subinterface{
			Ipv4: &fpoc.Interface_Subinterface_Ipv4{},
			Ipv6: &fpoc.Interface_Subinterface_Ipv6{
				Neighbor: map[string]*fpoc.Interface_Subinterface_Ipv6_Neighbor{
					"2001:db8:0::1": {
						Ip:               ygot.String("2001:db8:0::1"),
						LinkLayerAddress: ygot.String("52:fe:7c:91:6e:c1"),
					},
				},
			},
		},
	}, {
		desc: "Device contains SubInterface config, no conflicts",
		arp:  New().AddIPv6Neighbor("2001:db8:0::1", "52:fe:7c:91:6e:c1"),
		inSI: &fpoc.Interface_Subinterface{
			Ipv4: &fpoc.Interface_Subinterface_Ipv4{},
			Ipv6: &fpoc.Interface_Subinterface_Ipv6{
				Neighbor: map[string]*fpoc.Interface_Subinterface_Ipv6_Neighbor{
					"2001:db8:0::1": {
						Ip:               ygot.String("2001:db8:0::1"),
						LinkLayerAddress: ygot.String("52:fe:7c:91:6e:c1"),
					},
				},
			},
		},
		wantSI: &fpoc.Interface_Subinterface{
			Ipv4: &fpoc.Interface_Subinterface_Ipv4{},
			Ipv6: &fpoc.Interface_Subinterface_Ipv6{
				Neighbor: map[string]*fpoc.Interface_Subinterface_Ipv6_Neighbor{
					"2001:db8:0::1": {
						Ip:               ygot.String("2001:db8:0::1"),
						LinkLayerAddress: ygot.String("52:fe:7c:91:6e:c1"),
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.arp.AugmentSubInterface(test.inSI); err != nil {
				t.Fatalf("error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantSI, test.inSI); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentInterfaceErrors tests the error handling of AugmentInterface.
func TestAugmentInterfaceErrors(t *testing.T) {
	tests := []struct {
		desc          string
		arp           *StaticARP
		inSI          *fpoc.Interface_Subinterface
		wantErrSubStr string
	}{{
		desc: "Device contains SubInterface config with ipv4 conflicts",
		arp:  New().AddIPv4Neighbor("192.0.2.1", "52:fe:7c:91:6e:c1"),
		inSI: &fpoc.Interface_Subinterface{
			Ipv4: &fpoc.Interface_Subinterface_Ipv4{
				Neighbor: map[string]*fpoc.Interface_Subinterface_Ipv4_Neighbor{
					"192.0.2.1": {
						Ip:               ygot.String("192.0.2.1"),
						LinkLayerAddress: ygot.String("52:fe:7c:91:6e:c2"),
					},
				},
			},
		},
		wantErrSubStr: "destination value was set",
	}, {
		desc: "Device contains SubInterface config with ipv6 conflicts",
		arp:  New().AddIPv6Neighbor("2001:db8:0::1", "52:fe:7c:91:6e:c1"),
		inSI: &fpoc.Interface_Subinterface{
			Ipv6: &fpoc.Interface_Subinterface_Ipv6{
				Neighbor: map[string]*fpoc.Interface_Subinterface_Ipv6_Neighbor{
					"2001:db8:0::1": {
						Ip:               ygot.String("2001:db8:0::1"),
						LinkLayerAddress: ygot.String("52:fe:7c:91:6e:c2"),
					},
				},
			},
		},
		wantErrSubStr: "destination value was set",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.arp.AugmentSubInterface(test.inSI)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error sub-string does not match: %v", err)
			}
		})
	}
}
