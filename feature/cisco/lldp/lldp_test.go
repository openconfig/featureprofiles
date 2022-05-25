package basetest

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestLldpCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)

	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := dut.Config().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, true)

	})
	t.Run("Replace//lldp/config/enabled", func(t *testing.T) {
		path := dut.Config().Lldp().Enabled()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, true)

	})
	t.Run("Delete//lldp/config/enabled", func(t *testing.T) {
		path := dut.Config().Lldp().Enabled()
		defer observer.RecordYgot(t, "DELETE", path)
		path.Delete(t)

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
	iut := inputObj.Device(dut).GetInterface("$ports.peer_dut_1")
	peerintf := inputObj.Device(peer).GetInterface("$ports.peer_dut_1")
	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := peer.Config().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, true)

	})

	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := dut.Config().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, true)

	})
	t.Run("Subscribe//lldp/config/enabled", func(t *testing.T) {
		state := dut.Telemetry().Lldp().Interface(iut.Name()).Enabled()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != true {
			t.Errorf("LLDP Enabled: got %t, want %t", val, true)

		}

	})
	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := dut.Config().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, false)

	})
	t.Run("Subscribe//lldp/config/enabled", func(t *testing.T) {
		state := dut.Telemetry().Lldp().Interface(iut.Name()).Enabled()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != false {
			t.Errorf("Lacp Enabled: got %t, want %t", val, false)

		}

	})
	t.Run("Update//lldp/config/enabled", func(t *testing.T) {
		path := dut.Config().Lldp().Enabled()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, true)

	})

	peerid := peer.Telemetry().System().Hostname().Get(t) + "#" + peerintf.Name()
	peername := peer.Telemetry().System().Hostname().Get(t)
	time.Sleep(30 * time.Second)
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/system-name", func(t *testing.T) {
		state := dut.Telemetry().Lldp().Interface(iut.Name()).Neighbor(peerid).SystemName()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != peername {
			t.Errorf("Lacp SystemName: got %s, want %s", val, peername)

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/chassis-id-type", func(t *testing.T) {
		state := dut.Telemetry().Lldp().Interface(iut.Name()).Neighbor(peerid).ChassisIdType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.LldpTypes_ChassisIdType_MAC_ADDRESS {
			t.Errorf("Lldp chassis type: got %v, want %v", val, oc.LldpTypes_ChassisIdType_MAC_ADDRESS)

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/port-id", func(t *testing.T) {
		state := dut.Telemetry().Lldp().Interface(iut.Name()).Neighbor(peerid).PortId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Lldp portid: got %s, want !=%s", val, "''")

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/port-id-type", func(t *testing.T) {
		state := dut.Telemetry().Lldp().Interface(iut.Name()).Neighbor(peerid).PortIdType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.LldpTypes_PortIdType_INTERFACE_NAME {
			t.Errorf("Lacp portIdType: got %v, want %v", val, oc.LldpTypes_PortIdType_INTERFACE_NAME)

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/system-description", func(t *testing.T) {
		state := dut.Telemetry().Lldp().Interface(iut.Name()).Neighbor(peerid).SystemDescription()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Lldp System Description: got %s, want !=%s", val, "''")

		}

	})
	t.Run("Subscribe//lldp/interfaces/interface/neighbors/neighbor/state/chassis-id", func(t *testing.T) {
		state := dut.Telemetry().Lldp().Interface(iut.Name()).Neighbor(peerid).ChassisId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Lldp ChassisId: got %s, want !=%s", val, "''")

		}

	})

}
