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
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/golang/glog"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	p4ConfigV1 "github.com/p4lang/p4runtime/go/p4/config/v1"
	p4V1 "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	Type            p4V1.Update_Type
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
func aclWbbIngressTableEntryGet(info *ACLWbbIngressTableEntryInfo) *p4V1.Update {
	if info == nil {
		glog.Fatal("Nil info")
	}

	var matchFields []*p4V1.FieldMatch

	if info.IsIpv4 > 0 {
		matchFields = append(matchFields, &p4V1.FieldMatch{
			FieldId: WbbMatchMap["is_ipv4"],
			FieldMatchType: &p4V1.FieldMatch_Optional_{
				Optional: &p4V1.FieldMatch_Optional{
					Value: []byte{info.IsIpv4},
				},
			},
		})
	}

	if info.IsIpv6 > 0 {
		matchFields = append(matchFields, &p4V1.FieldMatch{
			FieldId: WbbMatchMap["is_ipv6"],
			FieldMatchType: &p4V1.FieldMatch_Optional_{
				Optional: &p4V1.FieldMatch_Optional{
					Value: []byte{info.IsIpv6},
				},
			},
		})
	}

	if info.EtherTypeMask > 0 {
		matchFields = append(matchFields, &p4V1.FieldMatch{
			FieldId: WbbMatchMap["ether_type"],
			FieldMatchType: &p4V1.FieldMatch_Ternary_{
				Ternary: &p4V1.FieldMatch_Ternary{
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
		matchFields = append(matchFields, &p4V1.FieldMatch{
			FieldId: WbbMatchMap["ttl"],
			FieldMatchType: &p4V1.FieldMatch_Ternary_{
				Ternary: &p4V1.FieldMatch_Ternary{
					Value: []byte{info.TTL},
					Mask:  []byte{info.TTLMask},
				},
			},
		})
	}

	if info.OuterVlanIDMask > 0 {
		matchFields = append(matchFields, &p4V1.FieldMatch{
			FieldId: WbbMatchMap["outer_vlan_id"],
			FieldMatchType: &p4V1.FieldMatch_Ternary_{
				Ternary: &p4V1.FieldMatch_Ternary{
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

	update := &p4V1.Update{
		Type: info.Type,
		Entity: &p4V1.Entity{
			Entity: &p4V1.Entity_TableEntry{
				TableEntry: &p4V1.TableEntry{
					TableId: WbbTableMap["acl_wbb_ingress_table"],
					Match:   matchFields,
					Action: &p4V1.TableAction{
						Type: &p4V1.TableAction_Action{
							Action: &p4V1.Action{
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
func ACLWbbIngressTableEntryGet(infoList []*ACLWbbIngressTableEntryInfo) []*p4V1.Update {
	var updates []*p4V1.Update

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

// StreamTermErrWithTimeout returns any error (if present), in the P4RTStreamTermErr channel.
// Function blocks for specified duration if no error in channel.
func StreamTermErrWithTimeout(ste chan *p4rt_client.P4RTStreamTermErr, timeout time.Duration) error {
	if ste == nil {
		return nil
	}
	select {
	case e := <-ste:
		return e.StreamErr
	case <-time.After(timeout):
		return nil
	}
}

// CheckRPCErrorCode returns nil if err is a gRPC status error carrying the
// wanted code, and a descriptive error otherwise (including when err is nil).
func CheckRPCErrorCode(err error, want codes.Code) error {
	if err == nil {
		return fmt.Errorf("expected %v error, got nil", want)
	}
	s, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("error is not a valid gRPC status error: %v", err)
	}
	if s.Code() != want {
		return fmt.Errorf("expected %v, got %v: %v", want, s.Code(), err)
	}
	return nil
}

// CheckRPCErrorNotFound checks if the given error is a gRPC NOT_FOUND error.
func CheckRPCErrorNotFound(err error) error {
	return CheckRPCErrorCode(err, codes.NotFound)
}

// PacketOutGet returns a StreamMessageRequest for PacketOut.
func PacketOutGet(payload []byte) *p4V1.StreamMessageRequest {
	return &p4V1.StreamMessageRequest{
		Update: &p4V1.StreamMessageRequest_Packet{
			Packet: &p4V1.PacketOut{
				Payload: payload,
			},
		},
	}
}

// PacketOutWithEgressPortGet returns a StreamMessageRequest for a PacketOut
// carrying the WBB packet_out metadata: egress_port (metadata id 1) set to the
// P4RT port id and, optionally, submit_to_ingress (metadata id 2).
func PacketOutWithEgressPortGet(payload []byte, egressPortID uint32, submitToIngress bool) *p4V1.StreamMessageRequest {
	pkt := &p4V1.PacketOut{
		Payload: payload,
		Metadata: []*p4V1.PacketMetadata{
			{
				MetadataId: uint32(1), // "egress_port"
				Value:      []byte(fmt.Sprint(egressPortID)),
			},
		},
	}
	if submitToIngress {
		pkt.Metadata = append(pkt.Metadata, &p4V1.PacketMetadata{
			MetadataId: uint32(2), // "submit_to_ingress"
			Value:      []byte{1},
		})
	}
	return &p4V1.StreamMessageRequest{
		Update: &p4V1.StreamMessageRequest_Packet{Packet: pkt},
	}
}

// DrainStreamTermErr discards, without blocking, any stream termination errors
// currently queued in the P4RTStreamTermErr channel.
func DrainStreamTermErr(ste chan *p4rt_client.P4RTStreamTermErr) {
	if ste == nil {
		return
	}
	for {
		select {
		case <-ste:
		default:
			return
		}
	}
}

// StreamTermErrForStream waits up to timeout for the termination error of the
// stream named streamName in the P4RTStreamTermErr channel. Termination entries
// for other streams (e.g. stale entries from earlier streams of the same client)
// are discarded. It returns as soon as a matching entry is received, and nil if
// no matching entry is received before the timeout expires.
func StreamTermErrForStream(ste chan *p4rt_client.P4RTStreamTermErr, streamName string, timeout time.Duration) error {
	if ste == nil {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case e := <-ste:
			if e == nil || e.StreamParams == nil || e.StreamParams.Name != streamName {
				continue
			}
			return e.StreamErr
		case <-timer.C:
			return nil
		}
	}
}

// StreamArbitrate creates a StreamChannel described by params on client, sends a
// MasterArbitrationUpdate for params.DeviceId/ElectionId and waits up to timeout
// for the outcome. It returns:
//   - nil if an arbitration response with an OK status is received;
//   - the gRPC status error with which the server terminated the stream, or a
//     gRPC status error built from a non-OK arbitration response status;
//   - a non-status error for client-side failures or timeouts.
//
// On timeout the stream is destroyed so no goroutine is left blocked.
func StreamArbitrate(client *p4rt_client.P4RTClient, params *p4rt_client.P4RTStreamParameters, timeout time.Duration) error {
	if client == nil || params == nil {
		return fmt.Errorf("StreamArbitrate: nil client or stream parameters")
	}
	name := params.Name
	if err := client.StreamChannelCreate(params); err != nil {
		return err
	}
	if err := client.StreamChannelSendMsg(&name, &p4V1.StreamMessageRequest{
		Update: &p4V1.StreamMessageRequest_Arbitration{
			Arbitration: &p4V1.MasterArbitrationUpdate{
				DeviceId: params.DeviceId,
				ElectionId: &p4V1.Uint128{
					High: params.ElectionIdH,
					Low:  params.ElectionIdL,
				},
			},
		},
	}); err != nil {
		if termErr := StreamTermErrForStream(client.StreamTermErr, name, timeout); termErr != nil {
			return termErr
		}
		return err
	}

	type arbResult struct {
		arb *p4rt_client.P4RTArbInfo
		err error
	}
	resCh := make(chan arbResult, 1)
	go func() {
		_, arb, err := client.StreamChannelGetArbitrationResp(&name, 1)
		resCh <- arbResult{arb: arb, err: err}
	}()

	select {
	case res := <-resCh:
		if res.err != nil {
			// The stream was terminated (io.EOF or stream no longer found); the
			// server's gRPC status is reported through the termination channel.
			if termErr := StreamTermErrForStream(client.StreamTermErr, name, timeout); termErr != nil {
				return termErr
			}
			return res.err
		}
		if res.arb == nil || res.arb.Arb == nil {
			return fmt.Errorf("missing MasterArbitrationUpdate response on stream %q", name)
		}
		if s := res.arb.Arb.GetStatus(); s != nil && codes.Code(s.GetCode()) != codes.OK {
			return status.Error(codes.Code(s.GetCode()), s.GetMessage())
		}
		return nil
	case <-time.After(timeout):
		client.StreamChannelDestroy(&name)
		return fmt.Errorf("timed out after %v waiting for arbitration response on stream %q", timeout, name)
	}
}

// SetForwardingPipelineConfigGet returns a VERIFY_AND_COMMIT
// SetForwardingPipelineConfigRequest for the given device, election id and P4Info.
func SetForwardingPipelineConfigGet(deviceID uint64, electionID *p4V1.Uint128, p4Info *p4ConfigV1.P4Info, cookie uint64) *p4V1.SetForwardingPipelineConfigRequest {
	return &p4V1.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: electionID,
		Action:     p4V1.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4V1.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4V1.ForwardingPipelineConfig_Cookie{
				Cookie: cookie,
			},
		},
	}
}

// ReadTableEntries reads the table entries of tableID (0 reads all tables) on
// deviceID and drains the server stream until io.EOF. Read is a server-streaming
// RPC, so a gRPC status error (e.g. NOT_FOUND) is reported by Recv(), not by the
// Read() call itself; that error is returned unchanged.
func ReadTableEntries(client *p4rt_client.P4RTClient, deviceID uint64, tableID uint32) ([]*p4V1.Entity, error) {
	if client == nil {
		return nil, fmt.Errorf("ReadTableEntries: nil client")
	}
	stream, err := client.Read(&p4V1.ReadRequest{
		DeviceId: deviceID,
		Entities: []*p4V1.Entity{{
			Entity: &p4V1.Entity_TableEntry{
				TableEntry: &p4V1.TableEntry{TableId: tableID},
			},
		}},
	})
	if err != nil {
		return nil, err
	}
	var entities []*p4V1.Entity
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return entities, nil
		}
		if err != nil {
			return nil, err
		}
		entities = append(entities, resp.GetEntities()...)
	}
}
