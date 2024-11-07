package basetest

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestLldpCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	path := gnmi.OC().Lldp().Enabled()

	t.Run(fmt.Sprintf("%v:Update//%v", dut.Name(), path.Config()), func(t *testing.T) {
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), true)

	})
	t.Run(fmt.Sprintf("%v:Replace//%v", dut.Name(), path.Config()), func(t *testing.T) {
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), true)

	})
	t.Run(fmt.Sprintf("%v:Delete//%v", dut.Name(), path.Config()), func(t *testing.T) {
		defer observer.RecordYgot(t, "DELETE", path)
		gnmi.Delete(t, dut, path.Config())

	})
}

func TestLldpState(t *testing.T) {

	dut := ondatra.DUT(t, device1)
	peer := ondatra.DUT(t, device2)
	inputObj, err := testInput.GetTestInput(t)
	inputObj.ConfigInterfaces(dut)
	inputObj.ConfigInterfaces(peer)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("$ports.port1")
	peerintf := inputObj.Device(peer).GetInterface("$ports.port1")
	t.Run(fmt.Sprintf("%v:Update//lldp/config/enabled", peer.Name()), func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, peer, path.Config(), true)

	})

	t.Run(fmt.Sprintf("%v:Update//lldp/config/enabled", dut.Name()), func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), true)

	})

	t.Run(fmt.Sprintf("%v:Update//lldp/interfaces/interface/config/enable", dut.Name()), func(t *testing.T) {
		d := &oc.Root{}
		i, _ := d.GetOrCreateLldp().NewInterface(iut.Name())
		i.Enabled = ygot.Bool(true)
		path := gnmi.OC().Lldp().Interface(iut.Name())
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), i)

	})
	t.Run(fmt.Sprintf("%v:Update//lldp/interfaces/interface/config/enable", peer.Name()), func(t *testing.T) {
		d := &oc.Root{}
		i, _ := d.GetOrCreateLldp().NewInterface(peerintf.Name())
		i.Enabled = ygot.Bool(true)
		path := gnmi.OC().Lldp().Interface(peerintf.Name())
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, peer, path.Config(), i)
	})
	t.Run(fmt.Sprintf("%v:Update//interfaces/interface/config/enable", dut.Name()), func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), true)

	})
	t.Run(fmt.Sprintf("%v:Update//interfaces/interface/config/enable", peer.Name()), func(t *testing.T) {
		path := gnmi.OC().Interface(peerintf.Name()).Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, peer, path.Config(), true)

	})
	time.Sleep(30 * time.Second)
	t.Run(fmt.Sprintf("%v:Subscribe//lldp/interfaces/interface/config/enabled", dut.Name()), func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Enabled()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != true {
			t.Errorf("LLDP Enabled: got %t, want %t", val, true)

		}

	})
	t.Run(fmt.Sprintf("%v:Update//lldp/config/enabled", dut.Name()), func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), false)

	})
	time.Sleep(30 * time.Second)
	t.Run(fmt.Sprintf("%v:Subscribe//lldp/interfaces/interface/config/enabled", dut.Name()), func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Enabled()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("LLDP Enabled: got %t, want %t", val, false)

		}

	})
	t.Run(fmt.Sprintf("%v:Update//lldp/config/enabled", dut.Name()), func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), true)

	})

	peerid := gnmi.Get(t, peer, gnmi.OC().System().Hostname().State()) + "#" + peerintf.Name()
	peername := gnmi.Get(t, peer, gnmi.OC().System().Hostname().State())
	time.Sleep(30 * time.Second)
	t.Run(fmt.Sprintf("%v:Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/system-name", dut.Name()), func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).SystemName()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != peername {
			t.Errorf("LLDP SystemName: got %s, want %s", val, peername)

		}

	})
	t.Run(fmt.Sprintf("%v:Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/chassis-id-type", dut.Name()), func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).ChassisIdType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.Lldp_ChassisIdType_MAC_ADDRESS {
			t.Errorf("Lldp chassis type: got %v, want %v", val, oc.Lldp_ChassisIdType_MAC_ADDRESS)

		}

	})
	t.Run(fmt.Sprintf("%v:Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/port-id", dut.Name()), func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).PortId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Lldp portid: got %s, want !=%s", val, "''")

		}

	})
	t.Run(fmt.Sprintf("%v:Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/port-id-type", dut.Name()), func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).PortIdType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.Lldp_PortIdType_INTERFACE_NAME {
			t.Errorf("Lacp portIdType: got %v, want %v", val, oc.Lldp_PortIdType_INTERFACE_NAME)

		}

	})
	t.Run(fmt.Sprintf("%v:Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/system-description", dut.Name()), func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).SystemDescription()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Lldp System Description: got %s, want !=%s", val, "''")

		}

	})
	t.Run(fmt.Sprintf("%v:Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/chassis-id", dut.Name()), func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).ChassisId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Lldp ChassisId: got %s, want !=%s", val, "''")

		}

	})

}
