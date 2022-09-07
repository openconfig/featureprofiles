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

package wbb

import (
	"github.com/golang/glog"
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

type AclWbbIngressTableEntryInfo struct {
	Type            p4_v1.Update_Type
	IsIpv4          uint8
	IsIpv6          uint8
	EtherType       uint16
	EtherTypeMask   uint16
	Ttl             uint8
	TtlMask         uint8
	OuterVlanId     uint16 // lower 12 bits
	OuterVlanIdMask uint16 // lower 12 bits
}

// Filling up P4RT Structs is a bit cumbersome, wrap things to simplify
func aclWbbIngressTableEntryGet(info *AclWbbIngressTableEntryInfo) *p4_v1.Update {
	if info == nil {
		glog.Fatal("Nil info")
	}

	update := &p4_v1.Update{
		Type: info.Type,
		Entity: &p4_v1.Entity{
			Entity: &p4_v1.Entity_TableEntry{
				TableEntry: &p4_v1.TableEntry{
					TableId: WbbTableMap["acl_wbb_ingress_table"],
					Match: []*p4_v1.FieldMatch{
						{
							FieldId: WbbMatchMap["is_ipv4"],
							FieldMatchType: &p4_v1.FieldMatch_Optional_{
								Optional: &p4_v1.FieldMatch_Optional{
									Value: []byte{byte(info.IsIpv4)},
								},
							},
						}, {
							FieldId: WbbMatchMap["is_ipv6"],
							FieldMatchType: &p4_v1.FieldMatch_Optional_{
								Optional: &p4_v1.FieldMatch_Optional{
									Value: []byte{byte(info.IsIpv6)},
								},
							},
						}, {
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
						}, {
							FieldId: WbbMatchMap["ttl"],
							FieldMatchType: &p4_v1.FieldMatch_Ternary_{
								Ternary: &p4_v1.FieldMatch_Ternary{
									Value: []byte{byte(info.Ttl)},
									Mask:  []byte{byte(info.TtlMask)},
								},
							},
						}, {
							FieldId: WbbMatchMap["outer_vlan_id"],
							FieldMatchType: &p4_v1.FieldMatch_Ternary_{
								Ternary: &p4_v1.FieldMatch_Ternary{
									Value: []byte{
										byte((info.OuterVlanId >> 8) & 0xF),
										byte(info.OuterVlanId & 0xFF),
									},
									Mask: []byte{
										byte((info.OuterVlanIdMask >> 8) & 0xF),
										byte(info.OuterVlanIdMask & 0xFF),
									},
								},
							},
						},
					},
					Action: &p4_v1.TableAction{
						Type: &p4_v1.TableAction_Action{
							Action: &p4_v1.Action{
								ActionId: WbbActionsMap["acl_wbb_ingress_trap"],
							},
						},
					},
				},
			},
		},
	}

	return update
}

func AclWbbIngressTableEntryGet(infoList []*AclWbbIngressTableEntryInfo) []*p4_v1.Update {
	var updates []*p4_v1.Update

	for _, info := range infoList {
		updates = append(updates, aclWbbIngressTableEntryGet(info))
	}

	return updates
}
