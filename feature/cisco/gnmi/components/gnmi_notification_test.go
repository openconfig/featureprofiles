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
	"github.com/openconfig/testt"
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
	t.Helper()
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
			return val != nil
		})

	err := performance.RestartProcess(t, dut, "invmgr")
	if err != nil {
		t.Fatal(err)
	}

	_, passed := watcher.Await(t)

	if !passed {
		t.Fatal("did not receive correct value before timeout")
	}

	t.Logf("GNMI Update notification received successfully after process invmgr restart")

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
			return val.OperStatus == oc.PlatformTypes_COMPONENT_OPER_STATUS_INACTIVE
		})

	t.Logf("Restarting LC %s", lc)
	util.ReloadLinecards(t, []string{lc})
	_, passed := watcher.Await(t)

	if !passed {
		t.Fatal("did not receive correct value before timeout")
	}

	t.Logf("GNMI Update notification received successfully after LC Reload")

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
				return val.OperStatus == oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED
			})
		t.Logf("awaiting notification for /components/component[name=%s]", lc)
		_, ok := watcher.Await(t)
		passed <- ok
	}()

	shutDownLinecard(t, lc)
	resultIsTrue := <-passed
	t.Logf("GNMI Update notification received successfully after LC shutdown")

	go func() {
		watcher := gnmi.Watch(t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().Component(lc).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
				val, _ := v.Val()
				t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
				t.Logf("received notification OperStatus: %s for %s\n", val.OperStatus, lc)
				return val.OperStatus == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
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

	t.Logf("GNMI Update notification received successfully after LC boot")

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
	t.Logf("GNMI Delete notification for optics port received successfully after LC shutdown")

	gnmi.Await(t, dut, gnmi.OC().Component(lc).OperStatus().State(), time.Minute*10, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)
	t.Logf("awaiting state: %s", oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)

	go func() {
		watcher := gnmi.Watch(t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().Component(port).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
				val, ok := v.Val()
				t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
				return ok
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

	t.Logf("GNMI Update notification for optics port received successfully after LC boot")
}

func TestNotificationRPFO(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	rps := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	rpStandby, rpActive := components.FindStandbyRP(t, dut, rps)
	t.Logf("RPs: %v", rps)

	activeChan := make(chan bool)
	standbyChan := make(chan bool)

	go func() {
		defer close(activeChan)
		for {
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				_, ok := gnmi.Watch(t,
					dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
					gnmi.OC().Component(rpActive).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
						val, _ := v.Val()
						t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
						t.Logf("received notification RedundantRole: %s for %s, want: SECONDARY", val.RedundantRole, rpActive)
						return val.RedundantRole == oc.Platform_ComponentRedundantRole_SECONDARY
					}).Await(t)
				activeChan <- ok
			}); errMsg != nil {
				t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
				time.Sleep(time.Second * 5)
			} else {
				break
			}
		}
	}()

	go func() {
		defer close(standbyChan)
		for {
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				_, ok := gnmi.Watch(t,
					dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
					gnmi.OC().Component(rpStandby).State(), time.Minute*10, func(v *ygnmi.Value[*oc.Component]) bool {
						val, _ := v.Val()
						t.Logf("received update: \n %s\n", util.PrettyPrintJson(val))
						t.Logf("received notification RedundantRole: %s for %s, want: PRIMARY", val.RedundantRole, rpStandby)
						return val.RedundantRole == oc.Platform_ComponentRedundantRole_PRIMARY
					}).Await(t)
				standbyChan <- ok
			}); errMsg != nil {
				t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
				time.Sleep(time.Second * 5)
			}
		}
	}()

	utils.Dorpfo(context.Background(), t, false)

	passed := <-activeChan && <-standbyChan

	if !passed {
		t.Fatal("did not receive correct value before timeout")
	}

	t.Logf("GNMI Update notifications received successfully after RP failover")

}
