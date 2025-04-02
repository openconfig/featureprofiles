package interface_neighbors_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	gnoisys "github.com/openconfig/gnoi/system"
	gnoitype "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

const (
	lc         = "0/0/CPU0"
	active_rp  = "0/RP0/CPU0"
	standby_rp = "0/RP1/CPU0"
)

func ReloadLineCards(t *testing.T, dut *ondatra.DUTDevice, wgg *sync.WaitGroup) error {

	defer wgg.Done()

	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	lcs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
	wg := sync.WaitGroup{}
	relaunched := make([]string, 0)

	for _, lc := range lcs {
		t.Logf("Restarting LC %v\n", lc)
		if empty := gnmi.Get(t, dut, gnmi.OC().Component(lc).Empty().State()); empty {
			t.Logf("Linecard Component %s is empty, skipping", lc)
		}
		if removable := gnmi.Get(t, dut, gnmi.OC().Component(lc).Removable().State()); !removable {
			t.Logf("Linecard Component %s is non-removable, skipping", lc)
		}
		oper := gnmi.Get(t, dut, gnmi.OC().Component(lc).OperStatus().State())

		if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
			t.Logf("Linecard Component %s is already INACTIVE, skipping", lc)
		}

		lineCardPath := components.GetSubcomponentPath(lc, false)
		resp, err := gnoiClient.System().Reboot(context.Background(), &gnoisys.RebootRequest{
			Method:  gnoisys.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot line card without delay",
			Subcomponents: []*gnoitype.Path{
				lineCardPath,
			},
			Force: true,
		})
		if err == nil {
			wg.Add(1)
			relaunched = append(relaunched, lc)
		} else {
			t.Fatalf("Reboot failed %v", err)
		}
		t.Logf("Reboot response: \n%v\n", resp)
	}

	// wait for all line cards to be back up
	for _, lc := range relaunched {
		go func(lc string) {
			defer wg.Done()
			timeout := time.Minute * 30
			t.Logf("Awaiting relaunch of linecard: %s", lc)
			oper := gnmi.Await[oc.E_PlatformTypes_COMPONENT_OPER_STATUS](
				t, dut,
				gnmi.OC().Component(lc).OperStatus().State(),
				timeout,
				oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
			)
			if val, ok := oper.Val(); !ok {
				t.Errorf("Reboot timed out, received status: %s", val)
				// check status if failed
			}
		}(lc)
	}

	wg.Wait()
	t.Log("All linecards successfully relaunched")

	return nil
}

func ReloadRouter(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) error {

	defer wg.Done()

	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())

	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	Resp, err := gnoiClient.System().Reboot(context.Background(), &gnoisys.RebootRequest{
		Method:  gnoisys.RebootMethod_COLD,
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

func DelMemberPortScale(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()
	batchConfig := &gnmi.SetBatch{}

	for i := 0; i < TOTAL_BUNDLE_INTF; i++ {
		pathb1m1 := gnmi.OC().Interface(mapBundleMemberPorts[dut.ID()][i].MemberPorts[0])
		gnmi.BatchDelete(batchConfig, pathb1m1.Config())
	}
	batchConfig.Set(t, dut)
}

func AddMemberPortScale(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()
	batchConfig := &gnmi.SetBatch{}

	for i := 0; i < TOTAL_BUNDLE_INTF; i++ {
		BE := generateBundleMemberInterfaceConfig(mapBundleMemberPorts[dut.ID()][i].MemberPorts[0],
			mapBundleMemberPorts[dut.ID()][i].BundleName)
		pathb1m1 := gnmi.OC().Interface(mapBundleMemberPorts[dut.ID()][i].MemberPorts[0])
		gnmi.BatchUpdate(batchConfig, pathb1m1.Config(), BE)
	}
	batchConfig.Set(t, dut)
}

func ProcessRestart(t *testing.T, dut *ondatra.DUTDevice, processName string, wg *sync.WaitGroup) {

	defer wg.Done()

	gnoiClient := dut.RawAPIs().GNOI(t)
	killProcessRequest := &gnoisys.KillProcessRequest{
		Signal:  gnoisys.KillProcessRequest_SIGNAL_KILL,
		Name:    processName,
		Restart: true,
	}
	killProcessResp, err := gnoiClient.System().KillProcess(context.Background(), killProcessRequest)
	time.Sleep(10 * time.Second)
	t.Logf("KillProcess response: %v", killProcessResp)
	if err != nil {
		t.Fatalf("Failed to restart process: %v", err)
	}
}

func RPFO(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()

	var supervisors []string
	active_state := gnmi.OC().Component(active_rp).Name().State()
	active := gnmi.Get(t, dut, active_state)
	standby_state := gnmi.OC().Component(standby_rp).Name().State()
	standby := gnmi.Get(t, dut, standby_state)
	supervisors = append(supervisors, active, standby)

	// find active and standby RP
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, supervisors)
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
	switchoverRequest := &gnoisys.SwitchControlProcessorRequest{
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

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, dut, supervisors)
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
