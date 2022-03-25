package staticroute

import (
	"errors"
	"strings"
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
		static: New().WithRoute("1.1.1.1", []string{""}),
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
		static: New().WithRoute("1.1.1.1", nil),
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
		static: New().WithRoute("1.1.1.1", []string{"1.2.3.44"}),
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
		static: New().WithRoute("1.1.1.1", []string{"1.2.3.44", "1.2.3.45"}),
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
			if err := test.static.AugmentNetworkInstance(test.inNI); err != nil {
				t.Fatalf("error not expected: %v", err)
			}

			if diff := cmp.Diff(test.wantNI, test.inNI); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

type FakeFeature struct {
	Err           error
	augmentCalled bool
	oc            *oc.NetworkInstance_Protocol_Static
}

func (f *FakeFeature) AugmentStatic(oc *oc.NetworkInstance_Protocol_Static) error {
	f.oc = oc
	f.augmentCalled = true
	return f.Err
}

// TestWithFeature tests the WithFeature method.
func TestWithFeature(t *testing.T) {
	tests := []struct {
		desc    string
		wantErr error
	}{{
		desc: "error not expected",
	}, {
		desc:    "error expected",
		wantErr: errors.New("some error"),
	}}

	for _, test := range tests {
		s := New().WithRoute("1.1.1.1", []string{"1.2.3.44"})
		ff := &FakeFeature{Err: test.wantErr}
		gotErr := s.WithFeature(ff, "1.1.1.1")
		if !ff.augmentCalled {
			t.Errorf("AugmentGlobal was not called")
		}
		if ff.oc != s.oc.GetStatic("1.1.1.1") {
			t.Errorf("Static ptr is not equal")
		}
		if test.wantErr != nil {
			if gotErr != nil {
				if !strings.Contains(gotErr.Error(), test.wantErr.Error()) {
					t.Errorf("Error strings are not equal: %v", gotErr)
				}
			}
			if gotErr == nil {
				t.Errorf("Expecting error but got none")
			}
		}
	}
}
