package staticroute

import (
	"strings"
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
		static: New().WithRoute("192.0.2.1/32", nil),
		inNI:   &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*fpoc.NetworkInstance_Protocol_Static{
						"192.0.2.1/32": {
							Prefix: ygot.String("192.0.2.1/32"),
						},
					},
				},
			},
		},
	}, {
		desc:   "Static route with one next-hop",
		static: New().WithRoute("192.0.2.1/32", []string{"203.0.113.14"}),
		inNI:   &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*fpoc.NetworkInstance_Protocol_Static{
						"192.0.2.1/32": {
							Prefix: ygot.String("192.0.2.1/32"),
							NextHop: map[string]*fpoc.NetworkInstance_Protocol_Static_NextHop{
								"1": {
									Index:   ygot.String("1"),
									NextHop: fpoc.UnionString("203.0.113.14"),
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc:   "Static route Multiple Next Hops",
		static: New().WithRoute("192.0.2.1/32", []string{"203.0.113.14", "203.0.113.15"}),
		inNI:   &fpoc.NetworkInstance{},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*fpoc.NetworkInstance_Protocol_Static{
						"192.0.2.1/32": {
							Prefix: ygot.String("192.0.2.1/32"),
							NextHop: map[string]*fpoc.NetworkInstance_Protocol_Static_NextHop{
								"1": {
									Index:   ygot.String("1"),
									NextHop: fpoc.UnionString("203.0.113.14"),
								},
								"2": {
									Index:   ygot.String("2"),
									NextHop: fpoc.UnionString("203.0.113.15"),
								},
							},
						},
					},
				},
			},
		},
	}, {
		desc:   "NI already contains static route so no change",
		static: New().WithRoute("192.0.2.1/32", []string{"203.0.113.14", "203.0.113.15"}),
		inNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*fpoc.NetworkInstance_Protocol_Static{
						"192.0.2.1/32": {
							Prefix: ygot.String("192.0.2.1/32"),
							NextHop: map[string]*fpoc.NetworkInstance_Protocol_Static_NextHop{
								"1": {
									Index:   ygot.String("1"),
									NextHop: fpoc.UnionString("203.0.113.14"),
								},
								"2": {
									Index:   ygot.String("2"),
									NextHop: fpoc.UnionString("203.0.113.15"),
								},
							},
						},
					},
				},
			},
		},
		wantNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*fpoc.NetworkInstance_Protocol_Static{
						"192.0.2.1/32": {
							Prefix: ygot.String("192.0.2.1/32"),
							NextHop: map[string]*fpoc.NetworkInstance_Protocol_Static_NextHop{
								"1": {
									Index:   ygot.String("1"),
									NextHop: fpoc.UnionString("203.0.113.14"),
								},
								"2": {
									Index:   ygot.String("2"),
									NextHop: fpoc.UnionString("203.0.113.15"),
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

// TestAugment_Errors tests the NI augment to device OC errors.
func TestAugment_Errors(t *testing.T) {
	var protocolKey = fpoc.NetworkInstance_Protocol_Key{
		Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       "static",
	}
	tests := []struct {
		desc          string
		static        *Static
		inNI          *fpoc.NetworkInstance
		wantErrSubStr string
	}{{
		desc:          "Prefix not in CIDR format",
		static:        New().WithRoute("192.0.2.1", nil),
		inNI:          &fpoc.NetworkInstance{},
		wantErrSubStr: "does not match regular expression pattern",
	}, {
		desc:   "Same prefix with different next-hop",
		static: New().WithRoute("192.0.2.1/32", []string{"203.0.113.14"}),
		inNI: &fpoc.NetworkInstance{
			Protocol: map[fpoc.NetworkInstance_Protocol_Key]*fpoc.NetworkInstance_Protocol{
				protocolKey: {
					Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
					Name:       ygot.String("static"),
					Static: map[string]*fpoc.NetworkInstance_Protocol_Static{
						"192.0.2.1/32": {
							Prefix: ygot.String("192.0.2.1/32"),
							NextHop: map[string]*fpoc.NetworkInstance_Protocol_Static_NextHop{
								"1": {
									Index:   ygot.String("1"),
									NextHop: fpoc.UnionString("203.0.113.15"),
								},
							},
						},
					},
				},
			},
		},
		wantErrSubStr: "field was set in both src and dst and was not equal",
	}}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.static.AugmentNetworkInstance(test.inNI)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings are not equal: %v", err)
			}
		})
	}
}
