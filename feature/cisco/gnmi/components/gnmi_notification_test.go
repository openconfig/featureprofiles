package delete_notification_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/cisco/performance"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ygnmi/ygnmi"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func findComponentsByTypeNoLogs(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	var s []string
	for _, c := range components {
		if c.GetType() == nil {
			continue
		}
		switch v := c.GetType().(type) {
		case oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
			if v == cType {
				s = append(s, c.GetName())
			}
		}
	}
	return s
}

func shutDownLinecard(t *testing.T, lc string) {
	const linecardBoottime = 5 * time.Minute
	dut := ondatra.DUT(t, "dut")

	gnoiClient := dut.RawAPIs().GNOI(t)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method:        spb.RebootMethod_POWERDOWN,
		Subcomponents: []*tpb.Path{},
	}

	req := &spb.RebootStatusRequest{
		Subcomponents: []*tpb.Path{},
	}

	rebootSubComponentRequest.Subcomponents = append(rebootSubComponentRequest.Subcomponents, components.GetSubcomponentPath(lc, false))
	req.Subcomponents = append(req.Subcomponents, components.GetSubcomponentPath(lc, false))

	t.Logf("Shutting down linecard: %v", lc)
	_, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card shutdown: %v", err)
	}
}

func powerUpLineCard(t *testing.T, lc string) {
	const linecardBoottime = 5 * time.Minute
	dut := ondatra.DUT(t, "dut")

	gnoiClient := dut.RawAPIs().GNOI(t)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method:        spb.RebootMethod_POWERUP,
		Subcomponents: []*tpb.Path{},
	}

	req := &spb.RebootStatusRequest{
		Subcomponents: []*tpb.Path{},
	}

	rebootSubComponentRequest.Subcomponents = append(rebootSubComponentRequest.Subcomponents, components.GetSubcomponentPath(lc, false))
	req.Subcomponents = append(req.Subcomponents, components.GetSubcomponentPath(lc, false))

	t.Logf("Powering up linecard: %v", lc)
	startTime := time.Now()
	_, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card power up: %v", err)
	}

	rebootDeadline := startTime.Add(linecardBoottime)
	for retry := true; retry; {
		t.Log("Waiting for 10 seconds before checking linecard status.")
		time.Sleep(10 * time.Second)
		if time.Now().After(rebootDeadline) {
			retry = false
			break
		}
		resp, err := gnoiClient.System().RebootStatus(context.Background(), req)
		switch {
		case status.Code(err) == codes.Unimplemented:
			t.Fatalf("Unimplemented RebootStatus RPC: %v", err)
		case err == nil:
			retry = resp.GetActive()
		default:
			// any other error just sleep.
		}
	}
	t.Logf("It took %v minutes to power up linecard.", time.Since(startTime).Minutes())
}

func TestNotificationProcessRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lc := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)[0]

	watcher := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().Component(lc).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
			val, _ := v.Val()
			t.Logf("received notification: \n %s\n", util.PrettyPrintJson(v))
			if val != nil {
				return true
			}
			return false
		})

	err := performance.RestartProcess(t, dut, "invmgr")
	if err != nil {
		t.Fatal(err)
	}

	_, passed := watcher.Await(t)

	if !passed {
		t.Fatal("did not receive correct value before timeout")
	}

}

func TestNotificationLCReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lc := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)[0]

	before := gnmi.Get(t, dut, gnmi.OC().Component(lc).State())
	t.Logf("get component before test:\n%s\n", util.PrettyPrintJson(before))

	watcher := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().Component(lc).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
			val, _ := v.Val()
			t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
			t.Logf("received notification OperStatus: %s for %s\n", val.OperStatus, lc)
			if val.OperStatus == oc.PlatformTypes_COMPONENT_OPER_STATUS_INACTIVE {
				return true
			}
			return false
		})

	t.Logf("Restarting LC %s", lc)
	util.ReloadLinecards(t, []string{lc})
	_, passed := watcher.Await(t)

	if !passed {
		t.Fatal("did not receive correct value before timeout")
	}

}

func TestNotificationLCShutUnshut(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lc := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)[0]

	before := gnmi.Get(t, dut, gnmi.OC().Component(lc).State())
	t.Logf("get component before test:\n%s\n", util.PrettyPrintJson(before))

	passed := make(chan bool)
	go func() {
		watcher := gnmi.Watch(t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().Component(lc).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
				val, _ := v.Val()
				t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
				t.Logf("received notification OperStatus: %s for %s\n", val.OperStatus, lc)
				if val.OperStatus == oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED {
					return true
				}
				return false
			})
		t.Logf("awaiting notification for /components/component[name=%s]", lc)
		_, ok := watcher.Await(t)
		passed <- ok
	}()

	shutDownLinecard(t, lc)
	resultIsTrue := <-passed

	go func() {
		watcher := gnmi.Watch(t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().Component(lc).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
				val, _ := v.Val()
				t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
				t.Logf("received notification OperStatus: %s for %s\n", val.OperStatus, lc)
				if val.OperStatus == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
					return true
				}
				return false
			})
		t.Logf("awaiting notification for /components/component[name=%s]", lc)
		_, ok := watcher.Await(t)
		passed <- ok
	}()

	powerUpLineCard(t, lc)

	resultIsTrue = resultIsTrue && <-passed
	if !resultIsTrue {
		t.Fatal("did not receive correct value before timeout")
	}

}

func TestNotificationDeletePort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lc := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)[0]
	port := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT)[0]

	before := gnmi.Get(t, dut, gnmi.OC().Component(lc).State())
	portBefore := gnmi.Get(t, dut, gnmi.OC().Component(lc).State())
	t.Logf("get component before test:\n%s\n", util.PrettyPrintJson(before))
	t.Logf("get component before test:\n%s\n", util.PrettyPrintJson(portBefore))

	passed := make(chan bool)
	go func() {
		watcher := gnmi.Watch(t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().Component(port).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
				_, ok := v.Val()
				if !ok {
					t.Logf("received delete update for %s", port)
					return true
				}
				return false
			})
		t.Logf("awaiting notification for /components/component[name=%s]", port)
		_, ok := watcher.Await(t)
		passed <- ok
	}()

	shutDownLinecard(t, lc)
	resultIsTrue := <-passed

	gnmi.Await(t, dut, gnmi.OC().Component(lc).OperStatus().State(), time.Minute*10, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)
	t.Logf("awaiting state: %s", oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)

	go func() {
		watcher := gnmi.Watch(t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().Component(port).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
				val, ok := v.Val()
				t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
				if ok {
					return true
				}
				return false
			})
		t.Logf("awaiting notification for /components/component[name=%s]", port)
		_, ok := watcher.Await(t)
		passed <- ok
	}()

	powerUpLineCard(t, lc)

	resultIsTrue = resultIsTrue && <-passed
	if !resultIsTrue {
		t.Fatal("did not receive correct value before timeout")
	}

}

func TestNotificationRPFO(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	rps := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	rpStandby, rpActive := components.FindStandbyRP(t, dut, rps)
	t.Logf("RPs: %v", rps)

	watcher := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(
			ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE),
		),
		gnmi.OC().Component(rpActive).State(), time.Minute*60, func(v *ygnmi.Value[*oc.Component]) bool {
			val, _ := v.Val()
			t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
			t.Logf("received notification RedundantRole: %s for %s, want: SECONDARY", val.RedundantRole, rpActive)
			if val.RedundantRole == oc.Platform_ComponentRedundantRole_SECONDARY {
				return true
			}
			return false
		})

	go utils.Dorpfo(context.Background(), t, false)

	watcher2 := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(
			ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE),
		),
		gnmi.OC().Component(rpStandby).State(), time.Minute*60, func(v *ygnmi.Value[*oc.Component]) bool {
			val, _ := v.Val()
			t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
			t.Logf("received notification RedundantRole: %s for %s, want: PRIMARY", val.RedundantRole, rpStandby)
			if val.RedundantRole == oc.Platform_ComponentRedundantRole_PRIMARY {
				return true
			}
			return false
		})

	_, ok := watcher.Await(t)
	_, ok2 := watcher2.Await(t)

	passed := ok && ok2

	if !passed {
		t.Fatal("did not receive correct value before timeout")
	}

}
