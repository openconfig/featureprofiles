package lacp_test

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const inputFile = "testdata/interface.yaml"

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	observer  = fptest.NewObserver("LACP").AddCsvRecorder("ocreport").AddCsvRecorder("LACP")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

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
		gnmi.Delete(t, dut, gnmi.OC().Lacp().Interface(iut.Name()).Config())
	})
	setInterfaceName(t, dut, iut.Name())

	t.Run("Update//lacp/interfaces/interface/config/interval", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).Interval()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), oc.Lacp_LacpPeriodType_SLOW)

	})
	t.Run("Update//lacp/interfaces/interface/config/system-priority", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).SystemPriority()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), priority)

	})
	t.Run("Update//lacp/interfaces/interface/config/system-id-mac", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).SystemIdMac()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), systemIDMac)

	})
	t.Run("Update//lacp/interfaces/interface/config/lacp-mode", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).LacpMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), oc.Lacp_LacpActivityType_ACTIVE)
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
		gnmi.Delete(t, dut, gnmi.OC().Lacp().Interface(iut.Name()).Config())
	})
	member := iut.Members()[0]
	systemIDMac := "00:03:00:04:00:05"
	priority := uint16(100)

	setInterfaceName(t, dut, iut.Name())

	t.Run("Update//lacp/interfaces/interface/config/interval", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).Interval()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), oc.Lacp_LacpPeriodType_SLOW)
	})
	t.Run("Update//lacp/interfaces/interface/config/system-priority", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).SystemPriority()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), priority)
	})
	t.Run("Update//lacp/interfaces/interface/config/system-id-mac", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).SystemIdMac()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), systemIDMac)
	})
	t.Run("Update//lacp/interfaces/interface/config/lacp-mode", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).LacpMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), oc.Lacp_LacpActivityType_ACTIVE)
	})

	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/oper-key", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).OperKey()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("Lacp OperKey: got %d, want !=%d", val, 0)

		}
	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/system-id", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).SystemId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Lacp SystemId: got %s, want !=%s", val, "''")

		}
	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/port-num", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).PortNum()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("Lacp PortNum: got %d, want !=%d", val, 0)

		}
	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/partner-id", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).PartnerId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
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
	setInterfaceName(t, dut, iut.Name())
	t.Run("Update//lacp/interfaces/interface/config/interval", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).Interval()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), oc.Lacp_LacpPeriodType_SLOW)
	})
	t.Run("Update//lacp/interfaces/interface/config/system-priority", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).SystemPriority()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), priority)

	})
	t.Run("Update//lacp/interfaces/interface/config/system-id-mac", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).SystemIdMac()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), systemIDMac)

	})
	t.Run("Update//lacp/interfaces/interface/config/lacp-mode", func(t *testing.T) {
		path := gnmi.OC().Lacp().Interface(iut.Name()).LacpMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), oc.Lacp_LacpActivityType_ACTIVE)

	})

	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-errors", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).Counters().LacpErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("Lacp LacpErrors: got %d, want ==%d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-in-pkts", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).Counters().LacpInPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("Lacp LacpInPkts: got %d, want %d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-out-pkts", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).Counters().LacpOutPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("Lacp LacpOutPkts: got %d, want %d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-unknown-errors", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).Counters().LacpUnknownErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("Lacp LacpUnknownErrors: got %d, want %d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-rx-errors", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).Counters().LacpRxErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("Lacp LacpRxErrors: got %d, want %d", val, 0)

		}

	})
	t.Run("Subscribe//lacp/interfaces/interface/members/member/state/counters/lacp-timeout-transitions", func(t *testing.T) {
		state := gnmi.OC().Lacp().Interface(iut.Name()).Member(member).Counters().LacpTimeoutTransitions()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("Lacp LacpTimeoutTransitions: got %d, want %d", val, 0)

		}

	})
}

func TestLacpTelemetry(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	systemIDMac1 := "00:03:00:04:00:05"
	systemIDMac2 := "00:03:00:04:11:15"

	systemPriority1 := uint16(100)
	systemPriority2 := uint16(200)

	// Default subscription rate is 30 seconds.
	subscriptionDuration := 50 * time.Second
	triggerDelay := 15 * time.Second
	expectedEntries := 2

	t.Run("Subscribe//lacp/interfaces/interface/state/system-id-mac", func(t *testing.T) {
		// initialise system-id-mac
		gnmi.Update(t, dut, gnmi.OC().Lacp().Interface(iut.Name()).SystemIdMac().Config(), systemIDMac1)
		t.Logf("Updated SystemIdMac :%s", gnmi.Lookup(t, dut, gnmi.OC().Lacp().Interface(iut.Name()).SystemIdMac().State()))

		// delay triggering system-id-mac change
		go func(t *testing.T) {
			time.Sleep(triggerDelay)
			gnmi.Update(t, dut, gnmi.OC().Lacp().Interface(iut.Name()).SystemIdMac().Config(), systemIDMac2)
			t.Log("Triggered system-id-mac change")
		}(t)

		path := gnmi.OC().Lacp().Interface(iut.Name()).SystemIdMac()
		defer observer.RecordYgot(t, "SUBSCRIBE", path)
		got := gnmi.Collect(t, dut, path.State(), subscriptionDuration).Await(t)

		if len(got) < expectedEntries {
			t.Errorf("Did not receive enough entries from subscription of system-id-mac: got %d, want %d", len(got), expectedEntries)
		}
		value, _ := got[len(got)-1].Val()
		if !reflect.DeepEqual(value, systemIDMac2) {
			t.Errorf("SystemIdMac change event was not recorded")
		}
	})

	t.Run("Subscribe//lacp/interfaces/interface/state/system-priority", func(t *testing.T) {

		//initialise system priority
		gnmi.Update(t, dut, gnmi.OC().Lacp().Interface(iut.Name()).SystemPriority().Config(), systemPriority1)
		t.Logf("Updated SystemPriority :%s", gnmi.Lookup(t, dut, gnmi.OC().Lacp().Interface(iut.Name()).SystemPriority().State()))

		//delay triggering system priority change
		go func(t *testing.T) {
			time.Sleep(triggerDelay)
			gnmi.Update(t, dut, gnmi.OC().Lacp().Interface(iut.Name()).SystemPriority().Config(), systemPriority2)
			t.Log("Triggered system-priority change")
		}(t)

		path := gnmi.OC().Lacp().Interface(iut.Name()).SystemPriority()
		defer observer.RecordYgot(t, "SUBSCRIBE", path)
		got := gnmi.Collect(t, dut, path.State(), subscriptionDuration).Await(t)

		if len(got) < expectedEntries {
			t.Errorf("Did not receive enough entries from subscription of system-priority: got %d, want %d", len(got), expectedEntries)
		}
		value, _ := got[len(got)-1].Val()
		if !reflect.DeepEqual(value, systemPriority2) {
			t.Errorf("SystemPriority change event was not recorded")
		}

	})
}

func setInterfaceName(t *testing.T, dev *ondatra.DUTDevice, name string) {
	ifPath := gnmi.OC().Lacp().Interface(name).Name().Config()
	gnmi.Update(t, dev, ifPath, name)
}

// TODO - future enhancement - state testcases should check for streaming telemetry rather than gnmi.Get i.e ONCE subscription

func TestLacpMemberEdt(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	iut2 := inputObj.Device(dut).GetInterface("Bundle-Ether121")
	inputObj.ConfigInterfaces(dut)
	member := iut.Members()[0]
	member1 := iut2.Members()[0]
	setInterfaceName(t, dut, iut.Name())
	setInterfaceName(t, dut, iut2.Name())

	lacpModes := []oc.E_Lacp_LacpActivityType{
		// oc.Lacp_LacpActivityType_UNSET, // under triage
		oc.Lacp_LacpActivityType_PASSIVE,
		oc.Lacp_LacpActivityType_ACTIVE,
	}
	for _, mode := range lacpModes {
		t.Run(fmt.Sprintf("LACP Mode %s: Verify an update is received by gnmi client on adding a member to a bundle interface.", mode.String()), func(t *testing.T) {
			setLacpMode(t, dut, iut.Name(), mode)

			gnmi.Delete(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config())

			watcher := edtWatchLacpMember(t, dut, iut.Name(), member)
			watcherAny := edtWatchLacpMemberAny(t, dut, iut.Name(), member)
			gnmi.Replace(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config(), iut.Name())
			if _, present := watcher.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
			if _, present := watcherAny.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
		})
		t.Run(fmt.Sprintf("LACP Mode %s: Verify an update is received by gnmi client on deleting a member from a bundle interface.", mode.String()), func(t *testing.T) {
			setLacpMode(t, dut, iut.Name(), mode)

			gnmi.Replace(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config(), iut.Name())
			watcher := edtWatchLacpMember(t, dut, iut.Name(), member)
			watcherAny := edtWatchLacpMemberAny(t, dut, iut.Name(), member)
			gnmi.Delete(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config())
			if _, present := watcher.Await(t); present {
				t.Errorf("EDT data was sent for member %s", member)
			}
			if _, present := watcherAny.Await(t); present {
				t.Errorf("EDT data was sent for member %s", member)
			}
		})
		t.Run(fmt.Sprintf("LACP Mode %s: Verify an update is received by gnmi client on shutting a member of a bundle interface.", mode.String()), func(t *testing.T) {
			setLacpMode(t, dut, iut.Name(), mode)

			gnmi.Replace(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config(), iut.Name())
			gnmi.Replace(t, dut, gnmi.OC().Interface(member1).Ethernet().AggregateId().Config(), iut.Name())
			watcher := edtWatchLacpMember(t, dut, iut.Name(), member)
			watcherAny := edtWatchLacpMemberAny(t, dut, iut.Name(), member)
			// shutdown bundle member and verify edt data is sent for the member
			gnmi.Update(t, dut, gnmi.OC().Interface(member).Enabled().Config(), false)
			defer gnmi.Update(t, dut, gnmi.OC().Interface(member).Enabled().Config(), true)
			if _, present := watcher.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
			if _, present := watcherAny.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
		})
		t.Run(fmt.Sprintf("LACP Mode %s: Verify an update is received by gnmi client on un-shutting a member of a bundle interface.", mode.String()), func(t *testing.T) {
			setLacpMode(t, dut, iut.Name(), mode)

			gnmi.Replace(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config(), iut.Name())
			gnmi.Replace(t, dut, gnmi.OC().Interface(member1).Ethernet().AggregateId().Config(), iut.Name())
			gnmi.Update(t, dut, gnmi.OC().Interface(member).Enabled().Config(), false)
			watcher := edtWatchLacpMember(t, dut, iut.Name(), member)
			watcherAny := edtWatchLacpMemberAny(t, dut, iut.Name(), member)
			// unshut bundle member and verify edt data is sent for the member
			gnmi.Update(t, dut, gnmi.OC().Interface(member).Enabled().Config(), true)
			if _, present := watcher.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
			if _, present := watcherAny.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
		})
		t.Run(fmt.Sprintf("LACP Mode %s: Verify updates are received by gnmi client when a member interface is reparented to a different bundle interface.", mode.String()), func(t *testing.T) {
			setLacpMode(t, dut, iut.Name(), mode)

			gnmi.Replace(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config(), iut.Name())
			gnmi.Replace(t, dut, gnmi.OC().Interface(member1).Ethernet().AggregateId().Config(), iut.Name())
			watcher := edtWatchLacpMember(t, dut, iut.Name(), member)
			watcherAny := edtWatchLacpMemberAny(t, dut, iut.Name(), member)
			// move bundle member1 to different bundle iut2
			gnmi.Replace(t, dut, gnmi.OC().Interface(member1).Ethernet().AggregateId().Config(), iut2.Name())
			if _, present := watcher.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
			if _, present := watcherAny.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
		})
		t.Run(fmt.Sprintf("LACP Mode %s: Verify an update is received by gnmi client on shutting down a bundle interface with member.", mode.String()), func(t *testing.T) {
			setLacpMode(t, dut, iut.Name(), mode)

			gnmi.Replace(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config(), iut.Name())
			gnmi.Replace(t, dut, gnmi.OC().Interface(member1).Ethernet().AggregateId().Config(), iut.Name())
			watcher := edtWatchLacpMember(t, dut, iut.Name(), member)
			watcherAny := edtWatchLacpMemberAny(t, dut, iut.Name(), member)
			// move bundle member1 to different bundle iut2
			gnmi.Update(t, dut, gnmi.OC().Interface(iut.Name()).Enabled().Config(), false)
			defer gnmi.Update(t, dut, gnmi.OC().Interface(iut.Name()).Enabled().Config(), true)
			if _, present := watcher.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
			if _, present := watcherAny.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
		})

		t.Run(fmt.Sprintf("LACP Mode %s: Verify an update is received by gnmi client on un-shutting a bundle interface with a member.", mode.String()), func(t *testing.T) {
			setLacpMode(t, dut, iut.Name(), mode)

			gnmi.Replace(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config(), iut.Name())
			gnmi.Replace(t, dut, gnmi.OC().Interface(member1).Ethernet().AggregateId().Config(), iut.Name())
			gnmi.Update(t, dut, gnmi.OC().Interface(iut.Name()).Enabled().Config(), false)
			watcher := edtWatchLacpMember(t, dut, iut.Name(), member)
			watcherAny := edtWatchLacpMemberAny(t, dut, iut.Name(), member)

			gnmi.Update(t, dut, gnmi.OC().Interface(iut.Name()).Enabled().Config(), true)
			if _, present := watcher.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
			if _, present := watcherAny.Await(t); !present {
				t.Errorf("EDT data was not sent for member %s", member)
			}
		})
	}
	t.Run("Verify an update is received by gnmi client on reloading an LC having members of a bundle interface.", func(t *testing.T) {
		// Get the host card for the member interface
		hostCard := lcOrRpByPort(t, dut)[member]
		t.Logf("hostCard for member %s:  is :%s", member, hostCard)
		if strings.Contains("RP", hostCard) {
			t.Skipf("Skipping test as member interface %s is hosted on RP", member)
		}

		gnoiClient := dut.RawAPIs().GNOI(t)
		rebootSubComponentRequest := &spb.RebootRequest{
			Method: spb.RebootMethod_COLD,
			Subcomponents: []*tpb.Path{
				components.GetSubcomponentPath(hostCard, false),
			},
		}

		setLacpMode(t, dut, iut.Name(), oc.Lacp_LacpActivityType_PASSIVE)

		gnmi.Replace(t, dut, gnmi.OC().Interface(member).Ethernet().AggregateId().Config(), iut.Name())
		gnmi.Replace(t, dut, gnmi.OC().Interface(member1).Ethernet().AggregateId().Config(), iut.Name())
		watcher := edtWatchLacpMember(t, dut, iut.Name(), member)
		watcherAny := edtWatchLacpMemberAny(t, dut, iut.Name(), member)
		_, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
		if err != nil {
			t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
		}
		if _, present := watcher.Await(t); !present {
			t.Errorf("EDT data was not sent for member %s", member)
		}
		if _, present := watcherAny.Await(t); !present {
			t.Errorf("EDT data was not sent for member %s", member)
		}
	})

}

// edtWatchLacpMember subscribes to a member of a bundle interface using ON_CHANGE subscription mode and
// returns a watcher.
func edtWatchLacpMember(t *testing.T, dut *ondatra.DUTDevice, bundle string, member string) *gnmi.Watcher[*oc.Lacp_Interface_Member] {
	t.Helper()
	path := gnmi.OC().Lacp().Interface(bundle).Member(member).State()
	t.Logf("TRY: subscribe ON_CHANGE to %s", path)

	edtWatch := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		path,
		time.Second*15,
		func(val *ygnmi.Value[*oc.Lacp_Interface_Member]) bool {
			data, present := val.Val()
			return present && (data.GetInterface() == member)
		})

	return edtWatch
}

// edtWatchLacpMemberAny subscribes to any member of a bundle interface using ON_CHANGE subscription mode. and
// returns a watcher.
func edtWatchLacpMemberAny(t *testing.T, dut *ondatra.DUTDevice, bundle string, member string) *gnmi.Watcher[*oc.Lacp_Interface_Member] {
	t.Helper()
	path := gnmi.OC().Lacp().Interface(bundle).MemberAny().State()
	t.Logf("TRY: subscribe ON_CHANGE to %s", path)

	edtWatch := gnmi.WatchAll(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		path,
		time.Second*15,
		func(val *ygnmi.Value[*oc.Lacp_Interface_Member]) bool {
			data, present := val.Val()
			return present && (data.GetInterface() == member)
		})

	return edtWatch
}

// lcOrRpByPort returns a map of <port>:<HostCardName> for dut
// ports using the component and the interface OC tree.
func lcOrRpByPort(t testing.TB, dut *ondatra.DUTDevice) map[string]string {
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
	nodes := make(map[string]string) // <hardware-port>:<ComponentName>
	for hp := range ports {
		p4Node := gnmi.Lookup(t, dut, gnmi.OC().Component(hp).Parent().State())
		if v, ok := p4Node.Val(); ok {
			nodes[hp] = v
		}
	}
	res := make(map[string]string) // <portID>:<NodeName>
	for _, v := range nodes {
		cType := gnmi.Lookup(t, dut, gnmi.OC().Component(v).Type().State())
		ct, ok := cType.Val()
		if !ok {
			continue
		}
		if ct != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT {
			continue
		}
		for _, p := range dut.Ports() {
			res[p.Name()] = getLCorRP(v)
		}
	}
	return res
}

// getLCorRP returns the LC or RP name from the npu component name
func getLCorRP(npu string) string {
	pattern := `\d+/(RP\d*|\d*)/CPU\d+`
	regex := regexp.MustCompile(pattern)
	match := regex.FindString(npu)
	return match
}

func setLacpMode(t *testing.T, dut *ondatra.DUTDevice, beIntf string, mode oc.E_Lacp_LacpActivityType) {
	t.Helper()
	path := gnmi.OC().Lacp().Interface(beIntf).LacpMode()
	if mode == oc.Lacp_LacpActivityType_UNSET {
		t.Log("LacpMode unset : Not setting LACP Mode")
	} else {
		gnmi.Update(t, dut, path.Config(), mode)
	}
}
