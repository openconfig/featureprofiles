package networkinstance

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
		desc       string
		ni         *NetworkInstance
		inDevice   *oc.Device
		wantDevice *oc.Device
	}{{
  		desc:     "Static route with no next-hops",
		ni:       New("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE).WithStaticRoute("1.1.1.1", []string{""}),
		inDevice: &oc.Device{},
		wantDevice: &oc.Device{
			NetworkInstance: map[string]*oc.NetworkInstance{
				"default": {
					Name:    ygot.String("default"),
					Enabled: ygot.Bool(true),
					Type:    oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
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
			},
		},
	}, {
  		desc:     "Static route with nil next-hop",
		ni:       New("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE).WithStaticRoute("1.1.1.1", nil),
		inDevice: &oc.Device{},
		wantDevice: &oc.Device{
			NetworkInstance: map[string]*oc.NetworkInstance{
				"default": {
					Name:    ygot.String("default"),
					Enabled: ygot.Bool(true),
					Type:    oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
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
			},
		},
	}, {
  		desc:     "Static route with one next-hop",
		ni:       New("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE).WithStaticRoute("1.1.1.1", []string{"1.2.3.44"}),
		inDevice: &oc.Device{},
		wantDevice: &oc.Device{
			NetworkInstance: map[string]*oc.NetworkInstance{
				"default": {
					Name:    ygot.String("default"),
					Enabled: ygot.Bool(true),
					Type:    oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
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
									},
								},
							},
						},
					},
				},
			},
		},
	}, {
  		desc:     "Static route Multiple Next Hops",
		ni:       New("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE).WithStaticRoute("1.1.1.1", []string{"1.2.3.44", "1.2.3.45"}),
		inDevice: &oc.Device{},
		wantDevice: &oc.Device{
			NetworkInstance: map[string]*oc.NetworkInstance{
				"default": {
					Name:    ygot.String("default"),
					Enabled: ygot.Bool(true),
					Type:    oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
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
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.ni.AugmentDevice(test.inDevice); err != nil {
				t.Fatalf("error not expected: %v", err)
			}

			if diff := cmp.Diff(test.wantDevice, test.inDevice); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}
