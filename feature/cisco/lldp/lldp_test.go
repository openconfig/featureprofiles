package basetest

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestLldpCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)

	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), true)

	})
	t.Run("Replace//lldp/config/enabled", func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), true)

	})
	t.Run("Delete//lldp/config/enabled", func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
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
	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, peer, path.Config(), true)

	})

	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), true)

	})
	time.Sleep(30 * time.Second)
	t.Run("Subscribe//lldp/config/enabled", func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Enabled()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != true {
			t.Errorf("LLDP Enabled: got %t, want %t", val, true)

		}

	})
	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), false)

	})
	time.Sleep(30 * time.Second)
	t.Run("Subscribe//lldp/config/enabled", func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Enabled()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("LLDP Enabled: got %t, want %t", val, false)

		}

	})
	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := gnmi.OC().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), true)

	})

	peerid := gnmi.Get(t, peer, gnmi.OC().System().Hostname().State()) + "#" + peerintf.Name()
	peername := gnmi.Get(t, peer, gnmi.OC().System().Hostname().State())
	time.Sleep(30 * time.Second)
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/system-name", func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).SystemName()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != peername {
			t.Errorf("LLDP SystemName: got %s, want %s", val, peername)

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/chassis-id-type", func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).ChassisIdType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.Lldp_ChassisIdType_MAC_ADDRESS {
			t.Errorf("Lldp chassis type: got %v, want %v", val, oc.Lldp_ChassisIdType_MAC_ADDRESS)

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/port-id", func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).PortId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Lldp portid: got %s, want !=%s", val, "''")

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/port-id-type", func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).PortIdType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.Lldp_PortIdType_INTERFACE_NAME {
			t.Errorf("Lacp portIdType: got %v, want %v", val, oc.Lldp_PortIdType_INTERFACE_NAME)

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/system-description", func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).SystemDescription()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Lldp System Description: got %s, want !=%s", val, "''")

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/chassis-id", func(t *testing.T) {
		state := gnmi.OC().Lldp().Interface(iut.Name()).Neighbor(peerid).ChassisId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Lldp ChassisId: got %s, want !=%s", val, "''")

		}

	})

}
