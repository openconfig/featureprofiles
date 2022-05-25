package basetest

import (
	"testing"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestLacpCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	systemIDMac := "00:03:00:04:00:05"
	priority := uint16(100)
	// t.Run("configLacp", func(t *testing.T) {
	// 	path := dut.Config().Lacp().Interface(iut.Name())
	// 	obj := &oc.Lacp_Interface{
	// 		Name:           ygot.String(iut.Name()),
	// 		Interval:       oc.Lacp_LacpPeriodType_SLOW,
	// 		SystemPriority: ygot.Uint16(priority),
	// 		SystemIdMac:    ygot.String(systemIDMac),
	// 		LacpMode:       oc.Lacp_LacpActivityType_ACTIVE,
	// 	}
	// 	defer observer.RecordYgot(t, "Update", path)
	// 	path.Update(t, obj)

	// })
	inputObj.ConfigInterfaces(dut)
	t.Cleanup(func() {
		dut.Config().Lacp().Interface(iut.Name()).Delete(t)
	})

	t.Run("Update//lacp/interfaces/interface/config/interval", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).Interval()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.Lacp_LacpPeriodType_SLOW)

	})

	t.Run("Update//lacp/interfaces/interface/config/system-priority", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).SystemPriority()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, priority)

	})
	t.Run("Update//lacp/interfaces/interface/config/system-id-mac", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).SystemIdMac()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, systemIDMac)

	})
	t.Run("Update//lacp/interfaces/interface/config/lacp-mode", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).LacpMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.Lacp_LacpActivityType_ACTIVE)

	})

}
func TestLacpState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	inputObj.ConfigInterfaces(dut)
	t.Cleanup(func() {
		dut.Config().Lacp().Interface(iut.Name()).Delete(t)
	})
	member := iut.Members()[0]
	systemIDMac := "00:03:00:04:00:05"
	priority := uint16(100)
	t.Run("Update//lacp/interfaces/interface/config/system-priority", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).SystemPriority()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, priority)

	})
	t.Run("Update//lacp/interfaces/interface/config/system-id-mac", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).SystemIdMac()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, systemIDMac)

	})
	t.Run("Update//lacp/interfaces/interface/config/lacp-mode", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).LacpMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.Lacp_LacpActivityType_ACTIVE)

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/oper-key", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).OperKey()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 {
			t.Errorf("Lacp OperKey: got %d, want !=%d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/system-id", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).SystemId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Lacp SystemId: got %s, want !=%s", val, "''")

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/port-num", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).PortNum()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 {
			t.Errorf("Lacp PortNum: got %d, want !=%d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/partner-id", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).PartnerId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Lacp PartnerId: got %s, want !=%s", val, "''")

		}

	})

}

func TestLacpCountersState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	member := iut.Members()[0]
	systemIDMac := "00:03:00:04:00:05"
	priority := uint16(100)
	t.Run("Update//lacp/interfaces/interface/config/system-priority", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).SystemPriority()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, priority)

	})
	t.Run("Update//lacp/interfaces/interface/config/system-id-mac", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).SystemIdMac()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, systemIDMac)

	})
	t.Run("Update//lacp/interfaces/interface/config/lacp-mode", func(t *testing.T) {
		path := dut.Config().Lacp().Interface(iut.Name()).LacpMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.Lacp_LacpActivityType_ACTIVE)

	})

	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-errors", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).Counters().LacpErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != 0 {
			t.Errorf("Lacp LacpErrors: got %d, want ==%d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-in-pkts", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).Counters().LacpInPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != 0 {
			t.Errorf("Lacp LacpInPkts: got %d, want %d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-out-pkts", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).Counters().LacpOutPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 {
			t.Errorf("Lacp LacpOutPkts: got %d, want %d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-unknown-errors", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).Counters().LacpUnknownErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != 0 {
			t.Errorf("Lacp LacpUnknownErrors: got %d, want %d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-rx-errors", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).Counters().LacpRxErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != 0 {
			t.Errorf("Lacp LacpRxErrors: got %d, want %d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-timeout-transitions", func(t *testing.T) {
		state := dut.Telemetry().Lacp().Interface(iut.Name()).Member(member).Counters().LacpTimeoutTransitions()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != 0 {
			t.Errorf("Lacp LacpTimeoutTransitions: got %d, want %d", val, 0)

		}

	})

}
