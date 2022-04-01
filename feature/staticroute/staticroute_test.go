package staticroute

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugment tests the NI augment to device OC.
func TestAugment(t *testing.T) {
	var protocolKey = fpoc.NetworkInstance_Protocol_Key{
		Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       "static",
	}
	tests := []struct {
		desc   string
		static *Static
		inNI   *fpoc.NetworkInstance
		wantNI *fpoc.NetworkInstance
	}{{
		desc:   "Static route with nil next-hop",
		static: New().WithRoute("1.1.1.1", nil),
		inNI:   &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*fpoc.NetworkInstance_Protocol_Static{
						"1.1.1.1": {
							Prefix: ygot.String("1.1.1.1"),
						},
					},
				},
			},
		},
	}, {
		desc:   "Static route with one next-hop",
		static: New().WithRoute("1.1.1.1", []string{"1.2.3.44"}),
		inNI:   &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*fpoc.NetworkInstance_Protocol_Static{
						"1.1.1.1": {
							Prefix: ygot.String("1.1.1.1"),
							NextHop: map[string]*fpoc.NetworkInstance_Protocol_Static_NextHop{
								"1": {
									Index:   ygot.String("1"),
									NextHop: fpoc.UnionString("1.2.3.44"),
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc:   "Static route Multiple Next Hops",
		static: New().WithRoute("1.1.1.1", []string{"1.2.3.44", "1.2.3.45"}),
		inNI:   &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*fpoc.NetworkInstance_Protocol_Static{
						"1.1.1.1": {
							Prefix: ygot.String("1.1.1.1"),
							NextHop: map[string]*fpoc.NetworkInstance_Protocol_Static_NextHop{
								"1": {
									Index:   ygot.String("1"),
									NextHop: fpoc.UnionString("1.2.3.44"),
								},
								"2": {
									Index:   ygot.String("2"),
									NextHop: fpoc.UnionString("1.2.3.45"),
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
			if err := test.static.AugmentNetworkInstance(test.inNI); err != nil {
				t.Fatalf("error not expected: %v", err)
			}

			if diff := cmp.Diff(test.wantNI, test.inNI); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}
