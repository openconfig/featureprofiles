package utils

import (
	"context"
	"fmt"

	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	ospb "github.com/openconfig/gnoi/os"
	gnps "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"

	// "golang.org/x/exp/rand"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	lc         = "0/0/CPU0"
	active_rp  = "0/RP0/CPU0"
	standby_rp = "0/RP1/CPU0"
)

func Dorpfo(ctx context.Context, t *testing.T, gribi_reconnect bool) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	// supervisor info
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
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &gnps.SwitchControlProcessorRequest{
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

// PerformLCOperations performs LC Reboot operations for a single line card.
func DoLcReboot(t *testing.T, dut *ondatra.DUTDevice, lineCard string) {
	t.Run(lineCard, func(t *testing.T) {
		empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(lineCard).Empty().State()).Val()
		if ok && empty {
			t.Skipf("Linecard Component %s is empty, hence skipping", lineCard)
		}
		if !gnmi.Get(t, dut, gnmi.OC().Component(lineCard).Removable().State()) {
			t.Skipf("Skip the test on non-removable linecard.")
		}

		lineCardOperStatus := gnmi.Get(t, dut, gnmi.OC().Component(lineCard).OperStatus().State())
		if got, want := lineCardOperStatus, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
			t.Skipf("Linecard Component %s is already INACTIVE, hence skipping", lineCard)
		}

		gnoiClient := dut.RawAPIs().GNOI(t)
		useNameOnly := deviations.GNOISubcomponentPath(dut)
		lineCardPath := components.GetSubcomponentPath(lineCard, useNameOnly)
		rebootSubComponentRequest := &gnps.RebootRequest{
			Method: gnps.RebootMethod_COLD,
			Subcomponents: []*tpb.Path{
				lineCardPath,
			},
		}
		t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
		rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
		if err != nil {
			t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
		}
		t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

		// Verify if the LC is booted up properly within 10*60sec = 10 mins
		pollLCUntilActiveOrRetryFail(t, dut, lc, 10, 60*time.Second)

	})
}

// DoAllAvailableLcParallelReboot performs LC reboot operations for all line cards concurrently.
func DoAllAvailableLcParallelReboot(t *testing.T, dut *ondatra.DUTDevice) {
	// get a list of LC's
	lineCards := util.GetLCList(t, dut)
	var wg sync.WaitGroup
	for _, lineCard := range lineCards {
		wg.Add(1)
		go func(lc string) {
			defer wg.Done()
			DoLcReboot(t, dut, lc)
		}(lineCard)
	}
	wg.Wait()
}

// DoAllAvailableLcSequencialReboot performs LC reboot operations for all line cards Sequencially.
func DoAllAvailableLcSequencialReboot(t *testing.T, dut *ondatra.DUTDevice) {
	// get a list of LC's
	lineCards := util.GetLCList(t, dut)
	for _, lineCard := range lineCards {
		DoLcReboot(t, dut, lineCard)
	}
}

func GnoiReboot(t *testing.T, dut *ondatra.DUTDevice) {
	//Reload router
	gnoiClient := dut.RawAPIs().GNOI(t)
	_, err := gnoiClient.System().Reboot(context.Background(), &gnps.RebootRequest{
		Method:  gnps.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Reboot failed %v", err)
	}
	startReboot := time.Now()
	const maxRebootTime = 30
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
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
	// same variable name, gnoiClient, is used for the gNOI connection during the second reboot.
	// This causes the cache to reuse the old connection for the second reboot, which is no longer active due to a timeout from the previous reboot.
	// The retry mechanism clears the previous cache connection and re-establishes new connection.
	for {
		gnoiClient := dut.RawAPIs().GNOI(t)
		ctx := context.Background()
		response, err := gnoiClient.System().Time(ctx, &gnps.TimeRequest{})

		// Log the error if it occurs
		if err != nil {
			t.Logf("Error fetching device time: %v", err)
		}

		// Check if the error code indicates that the service is unavailable
		if status.Code(err) == codes.Unavailable {
			// If the service is unavailable, wait for 30 seconds before retrying
			t.Logf("Service unavailable, retrying in 30 seconds...")
			time.Sleep(30 * time.Second)
		} else {
			// If the device time is fetched successfully, log the success message
			t.Logf("Device Time fetched successfully: %v", response)
			break
		}
		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device gnoi ready time: %.2f minutes", time.Since(startReboot).Minutes())
}

func DoProcessesRestart(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, processesList []string, parllel bool) {
	processesPidMap := GetProcessPIDs(t, dut, processesList)
	if parllel {
		var wg sync.WaitGroup

		for process, pid := range processesPidMap {
			wg.Add(1)
			go func(lc string) {
				defer wg.Done()
				DoProcessRestart(ctx, t, dut, process, pid)
			}(process)
		}

		wg.Wait()
	} else {
		for process, pid := range processesPidMap {
			DoProcessRestart(ctx, t, dut, process, pid)
		}
	}
}

func GetProcessPIDs(t *testing.T, dut *ondatra.DUTDevice, processNames []string) map[string]uint64 {
	// Retrieve all processes and their states
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	processPIDMap := make(map[string]uint64)

	// Iterate over the list of process names
	for _, pName := range processNames {
		var pID uint64
		for _, proc := range pList {
			if proc.GetName() == pName {
				pID = proc.GetPid()
				t.Logf("Pid of daemon '%s' is '%d'", pName, pID)
				processPIDMap[pName] = pID
				break
			}
		}
		// If the process is not found, log a warning
		if pID == 0 {
			t.Logf("Warning: Process '%s' not found on the DUT.", pName)
		}
	}

	return processPIDMap
}

func DoProcessRestart(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string, pid ...uint64) {
	var pID uint64

	// Use the provided PID if passed, otherwise compute it
	if len(pid) > 0 {
		pID = pid[0]
		t.Logf("Using provided PID '%d' for process '%s'.", pID, pName)
	} else {
		// Retrieve the PID of the process
		pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
		for _, proc := range pList {
			if proc.GetName() == pName {
				pID = proc.GetPid()
				t.Logf("Computed PID '%d' for process '%s'.", pID, pName)
				break
			}
		}
		if pID == 0 {
			t.Fatalf("Failed to find PID for process '%s'.", pName)
		}
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	killProcessRequest := &gnps.KillProcessRequest{
		Name:    pName,
		Pid:     uint32(pID),
		Signal:  gnps.KillProcessRequest_SIGNAL_KILL,
		Restart: true,
	}
	killResponse, err := gnoiClient.System().KillProcess(context.Background(), killProcessRequest)
	t.Logf("Got kill process response: %v\n\n", killResponse)
	// bypassing the check as emsd restart causes timing issue
	if err != nil && pName != "emsd" {
		t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
	}

	// Verify that the process comes back up
	t.Logf("Verifying that the process '%s' comes back up after being killed...", pName)
	startPollTime := time.Now()
	const maxProcessRestartTime = 5 * time.Minute
	processRestarted := false

	for time.Since(startPollTime) < maxProcessRestartTime {
		time.Sleep(10 * time.Second) // Polling interval
		// Check the process state
		pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
		for _, proc := range pList {
			if proc.GetName() == pName {
				t.Logf("Process '%s' is back up and running with PID '%d'.", pName, proc.GetPid())
				t.Logf("Time taken for process '%s' to come up: %.2f seconds", pName, time.Since(startPollTime).Seconds())
				processRestarted = true
				break
			}
		}
		if processRestarted {
			break
		}
		t.Logf("Process '%s' is not yet running. Retrying...", pName)
	}

	// Fail if the process did not come up within the timeout
	if !processRestarted {
		t.Fatalf("Process '%s' failed to come back up within the expected time of %.2f minutes.", pName, maxProcessRestartTime.Minutes())
	}

	// reestablishing gribi connection
	if pName == "emsd" {
		startPollTime := time.Now()
		const maxProcessRestartTime = 10
		for time.Since(startPollTime) < maxProcessRestartTime*time.Minute {
			processStatePath := ocpath.Root().System().ProcessAny().State()
			gnmic := dut.RawAPIs().GNMI(t)
			yc, err := ygnmi.NewClient(gnmic)
			if err != nil {
				t.Logf("Could not create ygnmi.Client: %v", err)
				continue
			}
			values, err := ygnmi.LookupAll(ctx, yc, processStatePath)
			if err == nil && len(values) > 0 {
				break
			}
			t.Logf("Process list is empty, retrying in 30 seconds...")
			time.Sleep(30 * time.Second)
		}
	}
}

// DoShutUnshutAllAvailableLcParallel performs LC shut/un-shut operations for all line cards concurrently.
func DoShutUnshutAllAvailableLcParallel(t *testing.T, dut *ondatra.DUTDevice) {
	// Find line card components
	lineCards := util.GetLCList(t, dut)

	var wg sync.WaitGroup

	for _, lineCard := range lineCards {
		wg.Add(1)
		go func(lc string) {
			defer wg.Done()
			DoLCShutUnshut(t, dut, lc)
		}(lineCard)
	}

	wg.Wait()

	// Sleep while LC reloads
	time.Sleep(10 * time.Minute)
}

func DoLCShutUnshut(t *testing.T, dut *ondatra.DUTDevice, lc string) {
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

	shutDownLinecard(t, dut, lc)
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

	powerUpLineCard(t, dut, lc)

	resultIsTrue = resultIsTrue && <-passed
	if !resultIsTrue {
		t.Fatal("did not receive correct value before timeout")
	}

	t.Logf("GNMI Update notification received successfully after LC boot")

	// Verify if the LC is booted up properly within 10*60sec = 10 mins
	pollLCUntilActiveOrRetryFail(t, dut, lc, 10, 60*time.Second)
}

func shutDownLinecard(t *testing.T, dut *ondatra.DUTDevice, lc string) {
	gnoiClient := dut.RawAPIs().GNOI(t)
	rebootSubComponentRequest := &gnps.RebootRequest{
		Method:        gnps.RebootMethod_POWERDOWN,
		Subcomponents: []*tpb.Path{},
	}

	req := &gnps.RebootStatusRequest{
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

func powerUpLineCard(t *testing.T, dut *ondatra.DUTDevice, lc string) {
	t.Helper()
	const linecardBoottime = 5 * time.Minute

	gnoiClient := dut.RawAPIs().GNOI(t)
	rebootSubComponentRequest := &gnps.RebootRequest{
		Method:        gnps.RebootMethod_POWERUP,
		Subcomponents: []*tpb.Path{},
	}

	req := &gnps.RebootStatusRequest{
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

// fetchStandbySupervisorStatus checks if the DUT has a standby supervisor available in a working state.
func HasDualSUP(ctx context.Context, osc ospb.OSClient) (bool, error) {
	r, err := osc.Verify(ctx, &ospb.VerifyRequest{})
	if err != nil {
		return false, fmt.Errorf("failed to verify: %w", err)
	}
	var dualSup bool

	switch v := r.GetVerifyStandby().GetState().(type) {
	case *ospb.VerifyStandby_StandbyState:
		if v.StandbyState.GetState() == ospb.StandbyState_UNAVAILABLE {
			return false, fmt.Errorf("OS.Verify RPC reports standby supervisor in UNAVAILABLE state")
		}
		// All other supervisor states indicate this device does not support or have dual supervisors available.
		fmt.Println("DUT is detected as single supervisor.")
		dualSup = false
	case *ospb.VerifyStandby_VerifyResponse:
		fmt.Println("DUT is detected as dual supervisor.")
		dualSup = true
	default:
		return false, fmt.Errorf("unexpected OS.Verify standby state RPC response: got %v (%T)", v, v)
	}

	return dualSup, nil
}

// function to probe the line card status using gnmi untill it is ACTIVE or timeout
// the timeout duration = maxRetries * retryInterval
func pollLCUntilActiveOrRetryFail(t *testing.T, dut *ondatra.DUTDevice, lineCard string, maxRetries int, retryInterval time.Duration) {
	for i := 0; i < maxRetries; i++ {
		time.Sleep(retryInterval)
		currentOperStatus := gnmi.Get(t, dut, gnmi.OC().Component(lineCard).OperStatus().State())
		if currentOperStatus == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Logf("Linecard Component %s is booted up properly and is ACTIVE", lineCard)
			return
		}
		t.Logf("Waiting for Linecard Component %s to become ACTIVE. Current status: %v", lineCard, currentOperStatus)
	}
	t.Fatalf("Linecard Component %s failed to become ACTIVE within the expected time", lineCard)
}
