package p4rt_edt_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestInterfaceIdOnChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	intfName := dut.Port(t, "port1").Name()

	gnmi.Replace(t, dut, gnmi.OC().Interface(intfName).Config(), &oc.Interface{
		Name: ygot.String(intfName),
		Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
	})

	portIds := []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	nextPortId := 0
	watcher := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().Interface(intfName).Id().State(),
		time.Minute,
		func(val *ygnmi.Value[uint32]) bool {
			id, present := val.Val()
			if !present {
				return false
			}
			if id != portIds[nextPortId] {
				t.Fatalf("Incorrect port id got %v, want %v", id, portIds[nextPortId])
			} else {
				t.Logf("Got correct port id %v", id)
			}
			nextPortId += 1
			return nextPortId >= len(portIds)
		})

	for _, v := range portIds {
		time.Sleep(1 * time.Second)
		t.Logf("Setting port id to %v", v)
		gnmi.Update(t, dut, gnmi.OC().Interface(intfName).Id().Config(), v)
	}

	_, gotall := watcher.Await(t)

	if !gotall {
		t.Fatalf("Did not receive all values, got %v want %v", nextPortId, len(portIds))
	}
}

func TestInterfaceIdAnyOnChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	wantPortIds := map[string]uint32{}
	for i, p := range dut.Ports() {
		intfName := p.Name()
		wantPortIds[intfName] = uint32(i + 1)

		gnmi.Replace(t, dut, gnmi.OC().Interface(intfName).Config(), &oc.Interface{
			Name: ygot.String(intfName),
			Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		})
	}

	gotCount := 0
	watcher := gnmi.WatchAll(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().InterfaceAny().Id().State(),
		time.Minute,
		func(val *ygnmi.Value[uint32]) bool {
			got, present := val.Val()
			if !present {
				return false
			}

			if len(val.Path.Elem) < 2 {
				t.Fatalf("Got erroneous path: %v", val.Path.String())
			}

			intf, ok := val.Path.Elem[1].Key["name"]
			if !ok {
				t.Fatalf("Got erroneous path: %v", val.Path.String())
			}

			want, ok := wantPortIds[intf]
			if ok && got == want {
				gotCount += 1
				t.Logf("Interface %s id updated to target value %v", intf, got)
			}

			return gotCount == len(wantPortIds)
		})

	for intf, id := range wantPortIds {
		t.Logf("Setting interface %s id to %v", intf, id)
		gnmi.Update(t, dut, gnmi.OC().Interface(intf).Id().Config(), id)
	}

	_, gotall := watcher.Await(t)

	if !gotall {
		t.Fatalf("Did not receive all values, got %v want %v", gotCount, len(wantPortIds))
	}
}

func TestDeviceIdOnChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	p4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}

	nodeIds := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	nextNodeId := 0
	watcher := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().Component(p4rtNode).IntegratedCircuit().NodeId().State(),
		time.Minute*5,
		func(val *ygnmi.Value[uint64]) bool {
			id, present := val.Val()
			if !present {
				return false
			}
			if id != nodeIds[nextNodeId] {
				t.Fatalf("Incorrect port id got %v, want %v", id, nodeIds[nextNodeId])
			} else {
				t.Logf("Got correct port id %v", id)
			}
			nextNodeId += 1
			return nextNodeId >= len(nodeIds)
		})

	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode).Config(), &oc.Component{
		Name: ygot.String(p4rtNode),
		IntegratedCircuit: &oc.Component_IntegratedCircuit{
			NodeId: ygot.Uint64(nodeIds[0]),
		},
	})

	for _, v := range nodeIds[1:] {
		time.Sleep(1 * time.Second)
		t.Logf("Setting node id to %v", v)
		gnmi.Update(t, dut, gnmi.OC().Component(p4rtNode).IntegratedCircuit().NodeId().Config(), v)
	}

	_, gotall := watcher.Await(t)

	if !gotall {
		t.Fatalf("Did not receive all values, got %v want %v", nextNodeId, len(nodeIds))
	}
}

func TestDeviceIdAnyOnChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	nodes := p4rtutils.P4RTNodesByPort(t, dut)

	wantNodeIds := map[string]uint64{}
	for i, p := range dut.Ports() {
		nodeName := nodes[p.ID()]
		wantNodeIds[nodeName] = uint64(i + 100)
	}

	gotCount := 0
	watcher := gnmi.WatchAll(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().ComponentAny().IntegratedCircuit().NodeId().State(),
		time.Minute,
		func(val *ygnmi.Value[uint64]) bool {
			got, present := val.Val()
			if !present {
				return false
			}

			if len(val.Path.Elem) < 2 {
				t.Fatalf("Got erroneous path: %v", val.Path.String())
			}

			cmp, ok := val.Path.Elem[1].Key["name"]
			if !ok {
				t.Fatalf("Got erroneous path: %v", val.Path.String())
			}

			want, ok := wantNodeIds[cmp]
			if ok && got == want {
				gotCount += 1
				t.Logf("Component %s node-id updated to target value %v", cmp, got)
			}

			return gotCount == len(wantNodeIds)
		})

	for cmp, id := range wantNodeIds {
		t.Logf("Setting component %s node-id to %v", cmp, id)
		gnmi.Replace(t, dut, gnmi.OC().Component(cmp).Config(), &oc.Component{
			Name: ygot.String(cmp),
			IntegratedCircuit: &oc.Component_IntegratedCircuit{
				NodeId: ygot.Uint64(id),
			},
		})
	}

	_, gotall := watcher.Await(t)

	if !gotall {
		t.Fatalf("Did not receive all values, got %v want %v", gotCount, len(wantNodeIds))
	}
}
