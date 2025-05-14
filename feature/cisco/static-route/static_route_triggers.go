package static_route_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/system"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	active_rp  = "0/RP0/CPU0"
	standby_rp = "0/RP1/CPU0"
)

func ProcessRestart(t *testing.T, dut *ondatra.DUTDevice, processName string) {

	waitForRestart := true
	pid := system.FindProcessIDByName(t, dut, processName)
	if pid == 0 {
		t.Fatalf("process %s not found on device", processName)
	}
	gnoiClient := dut.RawAPIs().GNOI(t)
	killProcessRequest := &spb.KillProcessRequest{
		Signal:  spb.KillProcessRequest_SIGNAL_KILL,
		Name:    processName,
		Pid:     uint32(pid),
		Restart: true,
	}
	gnoiClient.System().KillProcess(context.Background(), killProcessRequest)
	time.Sleep(30 * time.Second)

	if waitForRestart {
		gnmi.WatchAll(
			t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().System().ProcessAny().State(),
			time.Minute,
			func(p *ygnmi.Value[*oc.System_Process]) bool {
				val, ok := p.Val()
				if !ok {
					return false
				}
				return val.GetName() == processName && val.GetPid() != pid
			},
		)
	}
}

func ReloadRouter(t *testing.T, dut *ondatra.DUTDevice) error {

	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())

	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	Resp, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Reboot failed %v", err)
	}
	t.Logf("Reload Response %v ", Resp)

	startReboot := time.Now()
	time.Sleep(5 * time.Second)
	const maxRebootTime = 30
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(90 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())

		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
	return nil
}

func RPFO(t *testing.T, dut *ondatra.DUTDevice) {

	var supervisors []string
	active_state := gnmi.OC().Component(active_rp).Name().State()
	active := gnmi.Get(t, dut, active_state)
	standby_state := gnmi.OC().Component(standby_rp).Name().State()
	standby := gnmi.Get(t, dut, standby_state)
	supervisors = append(supervisors, active, standby)

	// find active and standby RP
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, supervisors)
	t.Logf("Detected activeRP: %v, standbyRP: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch)

	// make sure standby RP is reach
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
	// gnoiClient := dut.RawAPIs().GNOI(t)
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	//useNameOnly := deviations.GNOISubcomponentPath(dut)
	useNameOnly := false
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := rpStandbyBeforeSwitch
	got := ""
	if useNameOnly {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}

	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(900); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyControllerCard(t, dut, supervisors)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)
	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverTime().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", gnmi.Get(t, dut, activeRP.LastSwitchoverTime().State()))
	}

	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
}

func FlapBulkInterfaces(t *testing.T, dut *ondatra.DUTDevice, intfList []string) {

	var flapDuration time.Duration = 2
	var adminState bool

	adminState = false
	SetInterfaceStateScale(t, dut, intfList, adminState)
	time.Sleep(flapDuration * time.Second)
	adminState = true
	SetInterfaceStateScale(t, dut, intfList, adminState)
}

func SetInterfaceStateScale(t *testing.T, dut *ondatra.DUTDevice, intfList []string,
	adminState bool) {

	var intfType oc.E_IETFInterfaces_InterfaceType
	batchConfig := &gnmi.SetBatch{}

	for i := 0; i < len(intfList); i++ {
		if intfList[i][:6] == "Bundle" {
			intfType = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		} else {
			intfType = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		}
		j := &oc.Interface{
			Enabled: ygot.Bool(adminState),
			Name:    ygot.String(intfList[i]),
			Type:    intfType,
		}
		gnmi.BatchUpdate(batchConfig, gnmi.OC().Interface(intfList[i]).Config(), j)
	}
	batchConfig.Set(t, dut)
}

func DelAddMemberPort(t *testing.T, dut *ondatra.DUTDevice,
	dutPorts []string, bundlePort ...[]string) {

	batchConfig := &gnmi.SetBatch{}

	if len(bundlePort) > 0 {
		for i := 0; i < len(bundlePort); i++ {
			BE := generateBundleMemberInterfaceConfig(dutPorts[i], bundlePort[i][0])
			pathb1m1 := gnmi.OC().Interface(dutPorts[i])
			gnmi.BatchReplace(batchConfig, pathb1m1.Config(), BE)
		}
	} else {
		for i := 0; i < len(dutPorts); i++ {
			pathb1m1 := gnmi.OC().Interface(dutPorts[i])
			gnmi.BatchDelete(batchConfig, pathb1m1.Config())
		}
	}
	batchConfig.Set(t, dut)
}
