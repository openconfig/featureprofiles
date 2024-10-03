/*
 * Copyright (c) 2022 Cisco Systems, Inc. and its affiliates
 * All rights reserved.
 *
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package p4rtutils implements helper functions for acl_wbb_ingress_table in p4info file.
package p4rtutils

import (
	"testing"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/golang/glog"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	p4_v1 "github.com/p4lang/p4runtime/go/p4/v1"
)

// Some hardcoding to simplify things
var (
	WbbTableMap = map[string]uint32{
		"acl_wbb_ingress_table": 33554691,
	}
	WbbActionsMap = map[string]uint32{
		"acl_wbb_ingress_copy": 16777479,
		"acl_wbb_ingress_trap": 16777480,
	}
	WbbMatchMap = map[string]uint32{
		"is_ipv4":       1,
		"is_ipv6":       2,
		"ether_type":    3,
		"ttl":           4,
		"outer_vlan_id": 5,
	}
)

// ACLWbbIngressTableEntryInfo defines struct for wbb acl table
type ACLWbbIngressTableEntryInfo struct {
	Type            p4_v1.Update_Type
	IsIpv4          uint8
	IsIpv6          uint8
	EtherType       uint16
	EtherTypeMask   uint16
	TTL             uint8
	TTLMask         uint8
	OuterVlanID     uint16 // lower 12 bits
	OuterVlanIDMask uint16 // lower 12 bits
	Priority        uint32
	Metadata        string
}

// Filling up P4RT Structs is a bit cumbersome, wrap things to simplify
func aclWbbIngressTableEntryGet(info *ACLWbbIngressTableEntryInfo) *p4_v1.Update {
	if info == nil {
		glog.Fatal("Nil info")
	}

	matchFields := []*p4_v1.FieldMatch{}

	if info.IsIpv4 > 0 {
		matchFields = append(matchFields, &p4_v1.FieldMatch{
			FieldId: WbbMatchMap["is_ipv4"],
			FieldMatchType: &p4_v1.FieldMatch_Optional_{
				Optional: &p4_v1.FieldMatch_Optional{
					Value: []byte{byte(info.IsIpv4)},
				},
			},
		})
	}

	if info.IsIpv6 > 0 {
		matchFields = append(matchFields, &p4_v1.FieldMatch{
			FieldId: WbbMatchMap["is_ipv6"],
			FieldMatchType: &p4_v1.FieldMatch_Optional_{
				Optional: &p4_v1.FieldMatch_Optional{
					Value: []byte{byte(info.IsIpv6)},
				},
			},
		})
	}

	if info.EtherTypeMask > 0 {
		matchFields = append(matchFields, &p4_v1.FieldMatch{
			FieldId: WbbMatchMap["ether_type"],
			FieldMatchType: &p4_v1.FieldMatch_Ternary_{
				Ternary: &p4_v1.FieldMatch_Ternary{
					Value: []byte{
						byte(info.EtherType >> 8),
						byte(info.EtherType & 0xFF),
					},
					Mask: []byte{
						byte(info.EtherTypeMask >> 8),
						byte(info.EtherTypeMask & 0xFF),
					},
				},
			},
		})
	}

	if info.TTLMask > 0 {
		matchFields = append(matchFields, &p4_v1.FieldMatch{
			FieldId: WbbMatchMap["ttl"],
			FieldMatchType: &p4_v1.FieldMatch_Ternary_{
				Ternary: &p4_v1.FieldMatch_Ternary{
					Value: []byte{byte(info.TTL)},
					Mask:  []byte{byte(info.TTLMask)},
				},
			},
		})
	}

	if info.OuterVlanIDMask > 0 {
		matchFields = append(matchFields, &p4_v1.FieldMatch{
			FieldId: WbbMatchMap["outer_vlan_id"],
			FieldMatchType: &p4_v1.FieldMatch_Ternary_{
				Ternary: &p4_v1.FieldMatch_Ternary{
					Value: []byte{
						byte((info.OuterVlanID >> 8) & 0xF),
						byte(info.OuterVlanID & 0xFF),
					},
					Mask: []byte{
						byte((info.OuterVlanIDMask >> 8) & 0xF),
						byte(info.OuterVlanIDMask & 0xFF),
					},
				},
			},
		})
	}

	update := &p4_v1.Update{
		Type: info.Type,
		Entity: &p4_v1.Entity{
			Entity: &p4_v1.Entity_TableEntry{
				TableEntry: &p4_v1.TableEntry{
					TableId: WbbTableMap["acl_wbb_ingress_table"],
					Match:   matchFields,
					Action: &p4_v1.TableAction{
						Type: &p4_v1.TableAction_Action{
							Action: &p4_v1.Action{
								ActionId: WbbActionsMap["acl_wbb_ingress_trap"],
							},
						},
					},
					Priority: func() int32 {
						// Add Priority by default if not provided but required.
						if info.Priority == 0 && len(matchFields) > 0 {
							return 1
						}
						return int32(info.Priority)
					}(),
					Metadata: []byte(info.Metadata),
				},
			},
		},
	}
	return update
}

// ACLWbbIngressTableEntryGet returns acl table updates
func ACLWbbIngressTableEntryGet(infoList []*ACLWbbIngressTableEntryInfo) []*p4_v1.Update {
	var updates []*p4_v1.Update

	for _, info := range infoList {
		updates = append(updates, aclWbbIngressTableEntryGet(info))
	}

	return updates
}

// P4RTNodesByPort returns a map of <portID>:<P4RTNodeName> for the reserved ondatra
// ports using the component and the interface OC tree.
func P4RTNodesByPort(t testing.TB, dut *ondatra.DUTDevice) map[string]string {
	t.Helper()
	ports := make(map[string][]string) // <hardware-port>:[<portID>]
	for _, p := range dut.Ports() {
		hp := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State())
		if v, ok := hp.Val(); ok {
			if _, ok = ports[v]; !ok {
				ports[v] = []string{p.ID()}
			} else {
				ports[v] = append(ports[v], p.ID())
			}
		}
	}
	nodes := make(map[string]string) // <hardware-port>:<p4rtComponentName>
	for hp := range ports {
		p4Node := gnmi.Lookup(t, dut, gnmi.OC().Component(hp).Parent().State())
		if v, ok := p4Node.Val(); ok {
			nodes[hp] = v
		}
	}
	res := make(map[string]string) // <portID>:<P4RTNodeName>
	for k, v := range nodes {
		cType := gnmi.Lookup(t, dut, gnmi.OC().Component(v).Type().State())
		ct, ok := cType.Val()
		if !ok {
			continue
		}
		if ct != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT {
			continue
		}
		for _, p := range ports[k] {
			res[p] = v
		}
	}
	return res
}

// StreamTermErr returns any error (if present), in the P4RTStreamTermErr channel.
// Function blocks for 10 seconds if no error in channel.
func StreamTermErr(ste chan *p4rt_client.P4RTStreamTermErr) error {
	if ste == nil {
		return nil
	}
	select {
	case e := <-ste:
		return e.StreamErr
	default:
		return nil
	}
}
