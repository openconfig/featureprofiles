package staticroute

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugment tests the NI augment to device OC.
func TestAugment(t *testing.T) {
	var protocolKey = oc.NetworkInstance_Protocol_Key{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       "static",
	}
	tests := []struct {
		desc   string
		static *Static
		inNI   *oc.NetworkInstance
		wantNI *oc.NetworkInstance
	}{{
		desc:   "Static route with no next-hops",
		static: New().WithStaticRoute("1.1.1.1", []string{""}),
		inNI:   &oc.NetworkInstance{},
		wantNI: &oc.NetworkInstance{
			Protocol: map[oc.NetworkInstance_Protocol_Key]*oc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*oc.NetworkInstance_Protocol_Static{
						"primary": {
							Prefix: ygot.String("1.1.1.1"),
						},
					},
				},
			},
		},
	}, {
		desc:   "Static route with nil next-hop",
		static: New().WithStaticRoute("1.1.1.1", nil),
		inNI:   &oc.NetworkInstance{},
		wantNI: &oc.NetworkInstance{
			Protocol: map[oc.NetworkInstance_Protocol_Key]*oc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*oc.NetworkInstance_Protocol_Static{
						"primary": {
							Prefix: ygot.String("1.1.1.1"),
							NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
								"0": {
									Index:   ygot.String("0"),
									NextHop: nil,
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc:   "Static route with one next-hop",
		static: New().WithStaticRoute("1.1.1.1", []string{"1.2.3.44"}),
		inNI:   &oc.NetworkInstance{},
		wantNI: &oc.NetworkInstance{
			Protocol: map[oc.NetworkInstance_Protocol_Key]*oc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*oc.NetworkInstance_Protocol_Static{
						"1.1.1.1": {
							Prefix: ygot.String("1.1.1.1"),
							NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
								"0": {
									Index:   ygot.String("0"),
									NextHop: oc.UnionString("1.2.3.44"),
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc:   "Static route Multiple Next Hops",
		static: New().WithStaticRoute("1.1.1.1", []string{"1.2.3.44", "1.2.3.45"}),
		inNI:   &oc.NetworkInstance{},
		wantNI: &oc.NetworkInstance{
			Protocol: map[oc.NetworkInstance_Protocol_Key]*oc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*oc.NetworkInstance_Protocol_Static{
						"primary": {
							Prefix: ygot.String("1.1.1.1"),
							NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
								"0": {
									Index:   ygot.String("0"),
									NextHop: oc.UnionString("1.2.3.44"),
								},
								"1": {
									Index:   ygot.String("1"),
									NextHop: oc.UnionString("1.2.3.45"),
								},
							},
						},
					},
				},
			},
		},
	}}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.static.AugmentStaticRoute(test.inNI, "1.1.1.1"); err != nil {
				t.Fatalf("error not expected: %v", err)
			}

			if diff := cmp.Diff(test.wantNI, test.inNI); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}
