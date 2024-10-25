package copp_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func prettyPrintObj(obj interface{}) string {
	return spew.Sprintf("%#v", obj)
}

// unsupported leafs:
// - /oc-sys:system/control-plane-traffic/ingress/acl/matched-octets
// - /oc-sys:system/control-plane-traffic/ingress/qos
// - /oc-sys:system/control-plane-traffic/ingress/egress

// func TestIPv4ACL(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
//
// //	"/system/control-plane-traffic/ingress/acl/acl-set/*/set-name"
// gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet()
//
// // KEY
// //	"/system/control-plane-traffic/ingress/acl/acl-set/*/set-name"
// 	gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet().SetName()
//
// // KEY
// //	"/system/control-plane-traffic/ingress/acl/acl-set/*/type"
// 	gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet().Type()
//
// //  "/system/control-plane-traffic/ingress/acl/acl-set/acl-entries/acl-entry"
// 	gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet().AclEntryAny()
//
// //	"/system/control-plane-traffic/ingress/acl/acl-set/acl-entries/acl-entry/*/sequence-id"
// 	gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet().AclEntryAny().SequenceId()
//
// //	"/system/control-plane-traffic/ingress/acl/acl-set/acl-entries/acl-entry/state/matched-packets"
// 	gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet().AclEntryAny().MatchedPackets()
//
// //	"/system/control-plane-traffic/ingress/acl/acl-set/acl-entries/acl-entry/state/matched-octets"
// 	gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet().AclEntryAny().MatchedOctets()
//
// 	// NOT SUPPORTED
// 	// gnmi.OC().System().ControlPlaneTraffic().Egress()
//
// //	"/defined-sets/ipv4-prefix-sets/ipv4-prefix-set"
// 	gnmi.OC().DefinedSets().Ipv4PrefixSet()
//
// // KEY
// //	"/defined-sets/ipv4-prefix-sets/ipv4-prefix-set/*/name"
// 	gnmi.OC().DefinedSets().Ipv4PrefixSet().Name()
//
// //	"/defined-sets/ipv4-prefix-sets/ipv4-prefix-set/*/description"
// 	gnmi.OC().DefinedSets().Ipv4PrefixSet().Description()
//
// //	"/defined-sets/ipv4-prefix-sets/ipv4-prefix-set/*/prefix"
// 	gnmi.OC().DefinedSets().Ipv4PrefixSet().Prefix()
//
//
// //	"/defined-sets/ipv6-prefix-sets/ipv6-prefix-set"
// 	gnmi.OC().DefinedSets().Ipv6PrefixSet()
//
// // KEY
// //	"/defined-sets/ipv6-prefix-sets/ipv6-prefix-set/*/name"
// 	gnmi.OC().DefinedSets().Ipv6PrefixSetAny().Name()
//
//
// //	"/defined-sets/ipv6-prefix-sets/ipv6-prefix-set/*/description"
// 	gnmi.OC().DefinedSets().Ipv6PrefixSetAny().Description()
//
// //	"/defined-sets/ipv6-prefix-sets/ipv6-prefix-set/*/prefix"
// 	gnmi.OC().DefinedSets().Ipv6PrefixSetAny().Prefix()
//
//
// //	"/defined-sets/port-sets/port-set"
// 	gnmi.OC().DefinedSets().PortSet()
//
// // KEY
// //	"/defined-sets/port-sets/port-set/*/name"
// 	gnmi.OC().DefinedSets().PortSet().Name()
//
// //	"/defined-sets/port-sets/port-set/*/description"
// 	gnmi.OC().DefinedSets().PortSet().Description()
//
// //	"/defined-sets/port-sets/port-set/*/port"
// 	gnmi.OC().DefinedSets().PortSet().Port()
//
//
// //	"/acl/acl-sets/acl-set"
// 	gnmi.OC().Acl().AclSet()
//
// //	"/acl/acl-sets/acl-set/*/name"
// 	gnmi.OC().Acl().AclSet().Name()
//
// //	"/acl/acl-sets/acl-set/*/type"
// 	gnmi.OC().Acl().AclSet().Type()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry"
// 	gnmi.OC().Acl().AclSet().AclEntry()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/*/sequence-id"
// 	gnmi.OC().Acl().AclSet().AclEntry().SequenceId()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4"
// 	gnmi.OC().Acl().AclSet().AclEntry().Ipv4()
//
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/*/source-address-prefix-set"
// 	gnmi.OC().Acl().AclSet().AclEntry().Ipv4().SourceAddressPrefixSet()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/*/destination-address-prefix-set"
// 	gnmi.OC().Acl().AclSet().AclEntry().Ipv4().DestinationAddressPrefixSet()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6"
// 	gnmi.OC().Acl().AclSet().AclEntry().Ipv6()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/*/source-address-prefix-set"
// 	gnmi.OC().Acl().AclSet().AclEntry().Ipv6().SourceAddressPrefixSet()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/*/destination-address-prefix-set"
// 	gnmi.OC().Acl().AclSet().AclEntry().Ipv6().DestinationAddressPrefixSet()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/transport"
// 	gnmi.OC().Acl().AclSet().AclEntry().Transport()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/transport/*/source-port-set"
// 	gnmi.OC().Acl().AclSet().AclEntry().Transport().SourcePortSet()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/transport/*/destination-port-set"
// 	gnmi.OC().Acl().AclSet().AclEntry().Transport().DestinationPortSet()
//
// //	"/acl/acl-sets/acl-set/acl-entries/acl-entry/transport/*/destination-port"
// 	gnmi.OC().Acl().AclSet().AclEntry().Transport().DestinationPort()
//

func configIPv4PrefixSet() *oc.DefinedSets_Ipv4PrefixSet {
	d := &oc.Root{}

	ipv4Set := d.GetOrCreateDefinedSets().GetOrCreateIpv4PrefixSet("objv4")
	ipv4Set.Prefix = []string{
		"2.2.0.0/16",
		"3.3.3.3/32",
	}

	return ipv4Set
}

func TestDefinedSetsIpv4PrefixSets(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)

	definedSetIPv4 := configIPv4PrefixSet()

	tests := []struct {
		name         string
		expectedPass bool
		err          string
		key          string
		object       *oc.DefinedSets_Ipv4PrefixSet
	}{
		{
			name:         "Ipv4",
			expectedPass: true,
			err:          "",
			key:          "objv4",
			object:       definedSetIPv4,
		},
		{
			name:         "Ipv4NoKey",
			expectedPass: false,
			err:          "",
			key:          "",
			object:       definedSetIPv4,
		},
		{
			name:         "Ipv4BadConfig",
			expectedPass: false,
			err:          "",
			key:          "objv4",
			object: &oc.DefinedSets_Ipv4PrefixSet{
				Name:        ygot.String("objv4"),
				Description: ygot.String("obj-grp-v4-via-oc"),
				Prefix: []string{
					"invalid",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Replace", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().DefinedSets().Ipv4PrefixSet(tt.key).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Replace(t, dut, gnmi.OC().DefinedSets().Ipv4PrefixSet(tt.key).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Replace(t, dut, gnmi.OC().DefinedSets().Ipv4PrefixSet(tt.key).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					// TODO: Reenable test once CSCwm96980 fix is committed
					t.Skip()
					resp := gnmi.Get(t, dut, gnmi.OC().DefinedSets().Ipv4PrefixSet(tt.key).State())
					t.Logf("Get Response: %s", spew.Sdump(resp))
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value. got: %v, want: %v", resp, tt.object)
					}
				})
				t.Run("Get (Compatibility)", func(t *testing.T) {
					rawResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
						Path: []*gpb.Path{{
							Elem: []*gpb.PathElem{
								{Name: "defined-sets"},
								{Name: "ipv4-prefix-sets"},
								{Name: "ipv4-prefix-set", Key: map[string]string{
									"name": tt.object.GetName(),
								}},
							},
						}},
						Type: gpb.GetRequest_STATE,
						Encoding: gpb.Encoding_JSON_IETF,
					})
					t.Logf("Get response: %s", rawResponse.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
					if err != nil {
						t.Errorf("Compatibility get request failed: %s", err)
					} 
				})
			}
		})
		t.Run(tt.name+"_Update", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().DefinedSets().Ipv4PrefixSet(tt.key).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Update(t, dut, gnmi.OC().DefinedSets().Ipv4PrefixSet(tt.key).Config(), tt.object)
					t.Logf("Update Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Update(t, dut, gnmi.OC().DefinedSets().Ipv4PrefixSet(tt.key).Config(), tt.object)
				t.Logf("Update Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					// TODO: Reenable test once CSCwm96980 fix is committed
					t.Skip()
					resp := gnmi.Get(t, dut, gnmi.OC().DefinedSets().Ipv4PrefixSet(tt.key).State())
					t.Logf("Get Response: %s", spew.Sdump(resp))
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value. got: %v, want: %v", resp, tt.object)
					}
				})
			}

		})
	}
}

func configIPv6PrefixSet() *oc.DefinedSets_Ipv6PrefixSet {
	d := &oc.Root{}

	ipv6Set := d.GetOrCreateDefinedSets().GetOrCreateIpv6PrefixSet("objv6")
	ipv6Set.Prefix = []string{
		"2001:db8:85a3::8a2e:370:7334/128",
		"2001:db8:85a3::/64",
	}

	return ipv6Set
}

func TestDefinedSetsIpv6PrefixSets(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)

	definedSetIPv6 := configIPv6PrefixSet()

	tests := []struct {
		name         string
		expectedPass bool
		err          string
		key          string
		object       *oc.DefinedSets_Ipv6PrefixSet
	}{
		{
			name:         "Ipv6",
			expectedPass: true,
			err:          "",
			key:          "objv6",
			object:       definedSetIPv6,
		},
		{
			name:         "Ipv6NoKey",
			expectedPass: false,
			err:          "",
			key:          "",
			object: &oc.DefinedSets_Ipv6PrefixSet{
				Name:        ygot.String("objv6"),
				Description: ygot.String("obj-grp-v6-via-oc"),
				Prefix: []string{
					"2001:db8:85a3::8a2e:370:7334/128",
					"2001:db8:85a3::/64",
				},
			},
		},
		{
			name:         "Ipv6BadConfig",
			expectedPass: false,
			err:          "",
			key:          "objv6",
			object: &oc.DefinedSets_Ipv6PrefixSet{
				Name:        ygot.String("objv6"),
				Description: ygot.String("obj-grp-v6-via-oc"),
				Prefix: []string{
					"invalid",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Replace", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().DefinedSets().Ipv6PrefixSet(tt.key).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Replace(t, dut, gnmi.OC().DefinedSets().Ipv6PrefixSet(tt.key).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Replace(t, dut, gnmi.OC().DefinedSets().Ipv6PrefixSet(tt.key).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					// TODO: Reenable test once CSCwm96980 fix is committed
					t.Skip()
					resp := gnmi.Get(t, dut, gnmi.OC().DefinedSets().Ipv6PrefixSet(tt.key).State())
					t.Logf("Get Response: %s", spew.Sdump(resp))
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value. got: %v, want: %v", resp, tt.object)
					}
				})
				t.Run("Get (Compatibility)", func(t *testing.T) {
					rawResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
						Path: []*gpb.Path{{
							Elem: []*gpb.PathElem{
								{Name: "defined-sets"},
								{Name: "ipv6-prefix-sets"},
								{Name: "ipv6-prefix-set", Key: map[string]string{
									"name": tt.object.GetName(),
								}},
							},
						}},
						Type: gpb.GetRequest_STATE,
						Encoding: gpb.Encoding_JSON_IETF,
					})
					t.Logf("Get response: %s", rawResponse.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
					if err != nil {
						t.Errorf("Compatibility get request failed: %s", err)
					}
				})
			}
		})
		t.Run(tt.name+"_Update", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().DefinedSets().Ipv6PrefixSet(tt.key).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Update(t, dut, gnmi.OC().DefinedSets().Ipv6PrefixSet(tt.key).Config(), tt.object)
					t.Logf("Update Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Update(t, dut, gnmi.OC().DefinedSets().Ipv6PrefixSet(tt.key).Config(), tt.object)
				t.Logf("Update Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					// TODO: Reenable test once CSCwm96980 fix is committed
					t.Skip()
					resp := gnmi.Get(t, dut, gnmi.OC().DefinedSets().Ipv4PrefixSet(tt.key).State())
					t.Logf("Get Response: %s", spew.Sdump(resp))
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value. got: %v, want: %v", resp, tt.object)
					}
				})
			}

		})
	}

}

func configPortSet() *oc.DefinedSets_PortSet {
	d := &oc.Root{}

	portSet := d.GetOrCreateDefinedSets().GetOrCreatePortSet("objport")
	portSet.Port =
		[]oc.DefinedSets_PortSet_Port_Union{
			oc.UnionString("5555..6000"),
			oc.UnionString("3000"),
		}

	return portSet
}

func TestDefinedSetsPortSets(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)

	definedSetPort := configPortSet()

	tests := []struct {
		name         string
		expectedPass bool
		err          string
		key          string
		object       *oc.DefinedSets_PortSet
	}{
		{
			name:         "PortSet",
			expectedPass: true,
			err:          "",
			key:          "objport",
			object:       definedSetPort,
		},
		{
			name:         "PortSetNoKey",
			expectedPass: false,
			err:          "",
			key:          "",
			object:       definedSetPort,
		},
		{
			name:         "PortSetBadConfig",
			expectedPass: false,
			err:          "",
			key:          "objport",
			object: &oc.DefinedSets_PortSet{
				Name:        ygot.String("objport"),
				Description: ygot.String("obj-grp-port-via-oc"),
				Port: []oc.DefinedSets_PortSet_Port_Union{
					oc.E_PortSet_Port(100000000000),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Replace", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().DefinedSets().PortSet(tt.key).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Replace(t, dut, gnmi.OC().DefinedSets().PortSet(tt.key).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Replace(t, dut, gnmi.OC().DefinedSets().PortSet(tt.key).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					// TODO: Reenable test once CSCwm96980 fix is committed
					t.Skip()
					resp := gnmi.Get(t, dut, gnmi.OC().DefinedSets().PortSet(tt.key).State())
					t.Logf("Get Response: %s", spew.Sdump(resp))
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value. got: %v, want: %v", resp, tt.object)
					}
				})
				t.Run("Get (Compatibility)", func(t *testing.T) {
					rawResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
						Path: []*gpb.Path{{
							Elem: []*gpb.PathElem{
								{Name: "defined-sets"},
								{Name: "port-sets"},
								{Name: "port-set", Key: map[string]string{
									"name": tt.object.GetName(),
								}},
							},
						}},
						Type: gpb.GetRequest_STATE,
						Encoding: gpb.Encoding_JSON_IETF,
					})
					t.Logf("Get response: %s", rawResponse.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
					if err != nil {
						t.Errorf("Compatibility get request failed: %s", err)
					}
				})
			}
		})
		t.Run(tt.name+"_Update", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().DefinedSets().PortSet(tt.key).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Update(t, dut, gnmi.OC().DefinedSets().PortSet(tt.key).Config(), tt.object)
					t.Logf("Update Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Update(t, dut, gnmi.OC().DefinedSets().PortSet(tt.key).Config(), tt.object)
				t.Logf("Update Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					// TODO: Reenable test once CSCwm96980 fix is committed
					t.Skip()
					resp := gnmi.Get(t, dut, gnmi.OC().DefinedSets().PortSet(tt.key).State())
					t.Logf("Get Response: %s", spew.Sdump(resp))
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value. got: %v, want: %v", resp, tt.object)
					}
				})
			}

		})
	}

}

func configACLIPv4() *oc.Acl_AclSet {
	d := &oc.Root{}

	acl := d.GetOrCreateAcl().GetOrCreateAclSet("aclsetipv4", oc.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry10 := acl.GetOrCreateAclEntry(10)
	aclEntry10.SequenceId = ygot.Uint32(10)
	aclEntry10.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a := aclEntry10.GetOrCreateIpv4()
	a.SourceAddress = ygot.String("0.0.0.0/0")
	a.DestinationAddress = ygot.String("192.0.2.6/32")

	aclEntry20 := acl.GetOrCreateAclEntry(20)
	aclEntry20.SequenceId = ygot.Uint32(20)
	aclEntry20.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a2 := aclEntry20.GetOrCreateIpv4()
	a2.SourceAddress = ygot.String("192.0.2.6/32")
	a2.DestinationAddress = ygot.String("0.0.0.0/0")

	aclEntry30 := acl.GetOrCreateAclEntry(30)
	aclEntry30.SequenceId = ygot.Uint32(30)
	aclEntry30.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a3 := aclEntry30.GetOrCreateIpv4()
	a3.SourceAddress = ygot.String("0.0.0.0/0")
	a3.DestinationAddress = ygot.String("0.0.0.0/0")
	return acl
}

func TestAclSetIpv4(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)

	badCfg := configACLIPv4()
	badCfg.AclEntry[10].Actions.ForwardingAction = 10 
	
	aclIpv4Local := configACLIPv4()

	tests := []struct {
		name         string
		expectedPass bool
		err          string
		key          string
		object       *oc.Acl_AclSet
	}{
		{
			name:         "AclSetIPv4",
			expectedPass: true,
			err:          "",
			key:          aclIpv4Local.GetName(),
			object:       aclIpv4Local,
		},
		{
			name:         "AclSetIPv4NoKey",
			expectedPass: false,
			object:       aclIpv4Local,
		},
		{
			name:         "AclSetIPv4BadConfig",
			expectedPass: false,
			err:          "",
			key:          badCfg.GetName(),
			object:       badCfg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Replace", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().Acl().AclSet(tt.object.GetName(), tt.object.GetType()).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					resp := gnmi.Get(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).State())
					for id := range resp.AclEntry {
						resp.GetAclEntry(id).MatchedOctets = nil
						resp.GetAclEntry(id).MatchedPackets = nil
					}
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value.\ngot:\n%s\nwant:\n%s\n", prettyPrintObj(resp), prettyPrintObj(tt.object))
					}
				})
				
				t.Run("Get (Compatibility)", func(t *testing.T) {
					rawResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
						Path: []*gpb.Path{{
							Elem: []*gpb.PathElem{
								{Name: "acl"},
								{Name: "acl-sets"},
								{Name: "acl-set", Key: map[string]string{
									"name": tt.object.GetName(),
								}},
							},
						}},
						Type: gpb.GetRequest_STATE,
						Encoding: gpb.Encoding_JSON_IETF,
					})
					t.Logf("Get response: %s", rawResponse.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
					if err != nil {
						t.Errorf("Compatibility get request failed: %s", err)
					}
				})
			}
		})

		t.Run(tt.name+"_Update", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().Acl().AclSet(tt.object.GetName(), tt.object.GetType()).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Update(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Update(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					resp := gnmi.Get(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).State())
					for id := range resp.AclEntry {
						resp.GetAclEntry(id).MatchedOctets = nil
						resp.GetAclEntry(id).MatchedPackets = nil
					}
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value.\ngot:\n%s\nwant:\n%s\n", prettyPrintObj(resp), prettyPrintObj(tt.object))
					}
				})
			}

		})
	}

}

func configACLIPv6() *oc.Acl_AclSet {
	d := &oc.Root{}

	acl := d.GetOrCreateAcl().GetOrCreateAclSet("aclsetipv6", oc.Acl_ACL_TYPE_ACL_IPV6)
	aclEntry40 := acl.GetOrCreateAclEntry(40)
	aclEntry40.SequenceId = ygot.Uint32(40)
	aclEntry40.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a := aclEntry40.GetOrCreateIpv6()
	a.SourceAddress = ygot.String("2001:db8:85a3::8a2e:370:7334/128")
	a.DestinationAddress = ygot.String("2001:db8:85a3::/64")

	aclEntry50 := acl.GetOrCreateAclEntry(50)
	aclEntry50.SequenceId = ygot.Uint32(50)
	aclEntry50.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a2 := aclEntry50.GetOrCreateIpv6()
	a2.SourceAddress = ygot.String("2001:db8:85a3::8a2e:370:7334/128")
	a2.DestinationAddress = ygot.String("2001:db8:85a3::/64")

	aclEntry60 := acl.GetOrCreateAclEntry(60)
	aclEntry60.SequenceId = ygot.Uint32(60)
	aclEntry60.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a3 := aclEntry60.GetOrCreateIpv6()
	a3.SourceAddress = ygot.String("2001:db8:85a3::8a2e:370:7334/128")
	a3.DestinationAddress = ygot.String("2001:db8:85a3::/64")
	return acl
}

func TestAclSetIpv6(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)

	badCfg := configACLIPv6()
	badCfg.AclEntry[40].Actions.ForwardingAction = 10

	aclSetIPv6Local := configACLIPv6()

	tests := []struct {
		name         string
		expectedPass bool
		err          string
		key          string
		object       *oc.Acl_AclSet
	}{
		{
			name:         "AclSetIPv6",
			expectedPass: true,
			err:          "",
			key:          aclSetIPv6Local.GetName(),
			object:       aclSetIPv6Local,
		},
		{
			name:         "AclSetIPv6NoKey",
			expectedPass: false,
			err:          "",
			key:          "",
			object:       aclSetIPv6Local,
		},
		{
			name:         "AclSetIPv6BadConfig",
			expectedPass: false,
			err:          "",
			key:          badCfg.GetName(),
			object:       badCfg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Replace", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().Acl().AclSet(tt.object.GetName(), tt.object.GetType()).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					resp := gnmi.Get(t, dut, gnmi.OC().Acl().AclSet(tt.object.GetName(), tt.object.GetType()).State())
					for id := range resp.AclEntry {
						resp.GetAclEntry(id).MatchedOctets = nil
						resp.GetAclEntry(id).MatchedPackets = nil
					}
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value.\ngot:\n%s\nwant:\n%s\n", prettyPrintObj(resp), prettyPrintObj(tt.object))
					}
				})
				t.Run("Get (Compatibility)", func(t *testing.T) {
					rawResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
						Path: []*gpb.Path{{
							Elem: []*gpb.PathElem{
								{Name: "acl"},
								{Name: "acl-sets"},
								{Name: "acl-set", Key: map[string]string{
									"name": tt.object.GetName(),
								}},
							},
						}},
						Type: gpb.GetRequest_STATE,
						Encoding: gpb.Encoding_JSON_IETF,
					})
					t.Logf("Get response: %s", rawResponse.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
					if err != nil {
						t.Errorf("Compatibility get request failed: %s", err)
					}
				})
			}
		})

		t.Run(tt.name+"_Update", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().Acl().AclSet(tt.object.GetName(), tt.object.GetType()).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Update(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Update(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					resp := gnmi.Get(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).State())
					for id := range resp.AclEntry {
						resp.GetAclEntry(id).MatchedOctets = nil
						resp.GetAclEntry(id).MatchedPackets = nil
					}
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value.\ngot:\n%s\nwant:\n%s\n", prettyPrintObj(resp), prettyPrintObj(tt.object))
					}
				})
			}
		})
	}

}

func configACLTransport() *oc.Acl_AclSet {
	d := oc.Root{}

	acl := d.GetOrCreateAcl().GetOrCreateAclSet("aclsettransport", oc.Acl_ACL_TYPE_ACL_IPV4)

	aclEntry := acl.GetOrCreateAclEntry(70)
	aclEntry.SequenceId = ygot.Uint32(70)
	aclEntry.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT

	ipv4 := aclEntry.GetOrCreateIpv4()
	ipv4.SourceAddress = ygot.String("0.0.0.0/0")
	ipv4.DestinationAddress = ygot.String("192.0.2.6/32")
	ipv4.Protocol = oc.UnionUint8(6)

	transport := aclEntry.GetOrCreateTransport()
	transport.SourcePort = oc.UnionUint16(5555)
	transport.DestinationPort = oc.UnionUint16(3000)

	return acl
}

func TestAclSetTransport(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)

	badCfg := configACLTransport()
	badCfg.AclEntry[70].Actions.ForwardingAction = 10

	aclTransportLocal := configACLTransport()

	tests := []struct {
		name         string
		expectedPass bool
		err          string
		key          string
		object       *oc.Acl_AclSet
	}{
		{
			name:         "AclSetTransport",
			expectedPass: true,
			err:          "",
			key:          aclTransportLocal.GetName(),
			object:       aclTransportLocal,
		},
		{
			name:         "AclSetTransportNoKey",
			expectedPass: false,
			err:          "",
			key:          "",
			object:       aclTransportLocal,
		},
		{
			name:         "AclSetTransportBadConfig",
			expectedPass: false,
			err:          "",
			key:          badCfg.GetName(),
			object:       badCfg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Replace", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().Acl().AclSet(tt.object.GetName(), tt.object.GetType()).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					resp := gnmi.Get(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).State())
					for id := range resp.AclEntry {
						resp.GetAclEntry(id).MatchedOctets = nil
						resp.GetAclEntry(id).MatchedPackets = nil
					}
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value.\ngot:\n%s\nwant:\n%s\n", prettyPrintObj(resp), prettyPrintObj(tt.object))
					}
				})
				t.Run("Get (Compatibility)", func(t *testing.T) {
					rawResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
						Path: []*gpb.Path{{
							Elem: []*gpb.PathElem{
								{Name: "acl"},
								{Name: "acl-sets"},
								{Name: "acl-set", Key: map[string]string{
									"name": tt.object.GetName(),
								}},
							},
						}},
						Type: gpb.GetRequest_STATE,
						Encoding: gpb.Encoding_JSON_IETF,
					})
					t.Logf("Get response: %s", rawResponse.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
					if err != nil {
						t.Errorf("Compatibility get request failed: %s", err)
					}
				})
			}
		})

		t.Run(tt.name+"_Update", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().Acl().AclSet(tt.object.GetName(), tt.object.GetType()).Config())
			})

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Update(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Update(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					resp := gnmi.Get(t, dut, gnmi.OC().Acl().AclSet(tt.key, tt.object.GetType()).State())
					for id := range resp.AclEntry {
						resp.GetAclEntry(id).MatchedOctets = nil
						resp.GetAclEntry(id).MatchedPackets = nil
					}
					if !reflect.DeepEqual(resp, tt.object) {
						t.Fatalf("get response is not equal to set value.\ngot:\n%s\nwant:\n%s\n", prettyPrintObj(resp), prettyPrintObj(tt.object))
					}
				})
			}

		})
	}

}

func configACLForIngress() *oc.Acl_AclSet {
	d := &oc.Root{}

	acl := d.GetOrCreateAcl().GetOrCreateAclSet("aclsetipv4", oc.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry10 := acl.GetOrCreateAclEntry(10)
	aclEntry10.SequenceId = ygot.Uint32(10)
	aclEntry10.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a := aclEntry10.GetOrCreateIpv4()
	a.SourceAddress = ygot.String("0.0.0.0/0")
	a.DestinationAddress = ygot.String("0.0.0.0/0")

	aclEntry20 := acl.GetOrCreateAclEntry(20)
	aclEntry20.SequenceId = ygot.Uint32(20)
	aclEntry20.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a2 := aclEntry20.GetOrCreateIpv4()
	a2.SourceAddress = ygot.String("192.0.2.6/32")
	a2.DestinationAddress = ygot.String("0.0.0.0/0")

	aclEntry30 := acl.GetOrCreateAclEntry(30)
	aclEntry30.SequenceId = ygot.Uint32(30)
	aclEntry30.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a3 := aclEntry30.GetOrCreateIpv4()
	a3.SourceAddress = ygot.String("0.0.0.0/0")
	a3.DestinationAddress = ygot.String("0.0.0.0/0")
	return acl
}

func configControlPlaneTrafficIngress(s *oc.Acl_AclSet) *oc.System_ControlPlaneTraffic_Ingress_AclSet {
	d := &oc.Root{}

	ingress := d.GetOrCreateSystem().GetOrCreateControlPlaneTraffic().GetOrCreateIngress()
	aclSet := ingress.GetOrCreateAclSet(s.GetName(), s.GetType())

	return aclSet
}

func TestControlPlaneTrafficIngressACLSets(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)

	aclSetIpv4 := configACLForIngress()
	t.Log(spew.Sprint(aclSetIpv4))

	controlPlaneIngressAclSet := configControlPlaneTrafficIngress(aclSetIpv4)

	t.Log(spew.Sprint(controlPlaneIngressAclSet))

	tests := []struct {
		name         string
		expectedPass bool
		err          string
		key          string
		object       *oc.System_ControlPlaneTraffic_Ingress_AclSet
		aclSet       *oc.Acl_AclSet
	}{
		{
			name:         "IngressACLSet",
			expectedPass: true,
			err:          "",
			key:          controlPlaneIngressAclSet.GetSetName(),
			object:       controlPlaneIngressAclSet,
			aclSet:       aclSetIpv4,
		},
		{
			name:         "IngressACLSetNoKey",
			expectedPass: false,
			err:          "",
			key:          "",
			object:       controlPlaneIngressAclSet,
			aclSet:       aclSetIpv4,
		},
		{
			name:         "IngressACLSetBadConfig",
			expectedPass: false,
			err:          "",
			key:          "controlPlaneIngressAclSet",
			object: &oc.System_ControlPlaneTraffic_Ingress_AclSet{
				SetName: ygot.String("controlPlaneIngressAclSet"),
				AclEntry: map[uint32]*oc.System_ControlPlaneTraffic_Ingress_AclSet_AclEntry{
					10: {SequenceId: ygot.Uint32(10)},
					20: {SequenceId: ygot.Uint32(20)},
					30: {SequenceId: ygot.Uint32(30)},
				},
				Type: oc.Acl_ACL_TYPE_ACL_IPV4,
			},
			aclSet:       aclSetIpv4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Replace", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(tt.object.GetSetName(), tt.object.GetType()).Config())
				gnmi.Delete(t, dut, gnmi.OC().Acl().AclSet(tt.aclSet.GetName(), tt.aclSet.GetType()).Config())
			})

			gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(tt.aclSet.GetName(), tt.aclSet.GetType()).Config(), tt.aclSet)

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Replace(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
					t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Replace(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
				t.Logf("Replace Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					want := tt.object
					for seqId := range aclSetIpv4.AclEntry {
						want.AppendAclEntry(&oc.System_ControlPlaneTraffic_Ingress_AclSet_AclEntry{
							SequenceId: ygot.Uint32(seqId),
						})
					}
					resp := gnmi.Get(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(tt.object.GetSetName(), tt.object.GetType()).State())
					for id := range resp.AclEntry {
						resp.GetAclEntry(id).MatchedOctets = nil
						resp.GetAclEntry(id).MatchedPackets = nil
					}
					t.Logf("Get Response: %s", spew.Sdump(resp))
					if !reflect.DeepEqual(resp, want) {
						t.Fatalf("get response is not equal to set value. got: %v, want: %v", resp, want)
					}
					want.AclEntry = nil
				})
				t.Run("Get (Compatibility)", func(t *testing.T) {
					rawResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
						Path: []*gpb.Path{{
							Elem: []*gpb.PathElem{
								{Name: "system"},
								{Name: "control-plane-traffic"},
								{Name: "ingress"},
								{Name: "acl"},
								{Name: "acl-set", Key: map[string]string{
									"set-name": aclSetIpv4.GetName(),
								}},
							},
						}},
						Type: gpb.GetRequest_CONFIG,
						Encoding: gpb.Encoding_JSON_IETF,
					})
					t.Logf("Get response: %s", rawResponse.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
					if err != nil {
						t.Errorf("Compatibility get request failed: %s", err)
					} 
				})
			}
		})
		t.Run(tt.name+"_Update", func(t *testing.T) {
			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(tt.object.GetSetName(), tt.object.GetType()).Config())
				gnmi.Delete(t, dut, gnmi.OC().Acl().AclSet(tt.aclSet.GetName(), tt.aclSet.GetType()).Config())
			})

			gnmi.Update(t, dut, gnmi.OC().Acl().AclSet(tt.aclSet.GetName(), tt.aclSet.GetType()).Config(), tt.aclSet)

			if !tt.expectedPass {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					res := gnmi.Update(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
					t.Logf("Update Result: %s", prettyPrint(res.RawResponse))
				}); errMsg != nil {
					t.Logf("Received error %s", *errMsg)
				} else {
					t.Fatalf("Did not receive expected failure. want: %s", tt.err)
				}
			} else {
				res := gnmi.Update(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(tt.key, tt.object.GetType()).Config(), tt.object)
				t.Logf("Update Result: %s", prettyPrint(res.RawResponse))
				t.Run("Get", func(t *testing.T) {
					want := tt.object 
					for seqId := range aclSetIpv4.AclEntry {
						want.AppendAclEntry(&oc.System_ControlPlaneTraffic_Ingress_AclSet_AclEntry{
							SequenceId: ygot.Uint32(seqId),
						})
					}
					resp := gnmi.Get(t, dut, gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(tt.object.GetSetName(), tt.object.GetType()).State())
					for id := range resp.AclEntry {
						resp.GetAclEntry(id).MatchedOctets = nil
						resp.GetAclEntry(id).MatchedPackets = nil
					}
					t.Logf("Get Response: %s", spew.Sdump(resp))
					if !reflect.DeepEqual(resp, want) {
						t.Fatalf("get response is not equal to set value. got: %v, want: %v", resp, want)
					}
				})
			}

		})
	}

}
