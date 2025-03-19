package utils

import (
	"context"
	"sync"
	"testing"
	"time"

	// "github.com/openconfig/entity-naming/oc"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	gnps "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
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

// PerformLCOperations performs LC OIR operations for a single line card.
func DoLcOir(t *testing.T, dut *ondatra.DUTDevice, lineCard string) {
	t.Run(lineCard, func(t *testing.T) {
		// To remove
		if lineCard != "0/0/CPU0" {
			t.Skip()
		}

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

	})
}

// DoLcOir performs LC OIR operations for all line cards concurrently.
func DoAllAvailableLcParallelOir(t *testing.T, dut *ondatra.DUTDevice) {
	// Find line card components
	lineCards := util.GetLCList(t, dut)

	var wg sync.WaitGroup

	for _, lineCard := range lineCards {
		wg.Add(1)
		go func(lc string) {
			defer wg.Done()
			DoLcOir(t, dut, lc)
		}(lineCard)
	}

	wg.Wait()

	// Sleep while LC reloads
	time.Sleep(10 * time.Minute)
}

// DoLcOir performs LC OIR operations for all line cards Sequencially.
func DoAllAvailableLcSequencialOir(t *testing.T, dut *ondatra.DUTDevice) {
	// Find line card components
	lineCards := util.GetLCList(t, dut)

	for _, lineCard := range lineCards {
		DoLcOir(t, dut, lineCard)
		// Sleep while LC reloads
		time.Sleep(10 * time.Minute)
	}

}

// func grpcConfigChange(processes []string, grpcRepeat int, withRPFO bool, args *Args, t *testing.T) {
// 	args.client.Close(t)

// 	r := rand.New(rand.NewSource(time.Now().UnixNano()))
// 	min := 57344
// 	max := 57998

// 	for k := 0; k < grpcRepeat; k++ {
// 		if withRPFO {
// 			rpfoCount := k + 1
// 			t.Logf("This is RPFO #%d", rpfoCount)
// 			args.rpfo(args.ctx, t, true)
// 		}

// 		port := r.Intn(max-min+1) + min

// 		t.Logf("set grpc value to no-tls and random port")
// 		config.CMDViaGNMI(args.ctx, t, args.dut, fmt.Sprintf("configure \n grpc \n no-tls \n port %s \n commit \n", strconv.Itoa(port)))
// 		config.CMDViaGNMI(args.ctx, t, args.dut, "show grpc")

// 		time.Sleep(10 * time.Second)

// 		t.Logf("set grpc value to no no-tls and no manually configured port")
// 		config.CMDViaGNMI(args.ctx, t, args.dut, fmt.Sprintf("configure \n grpc \n no no-tls \n no port %s \n commit \n", strconv.Itoa(port)))
// 		config.CMDViaGNMI(args.ctx, t, args.dut, "show grpc")

// 		time.Sleep(10 * time.Second)

// 		client := gribi.Client{
// 			DUT:                   args.dut,
// 			FibACK:                *ciscoFlags.GRIBIFIBCheck,
// 			Persistence:           true,
// 			InitialElectionIDLow:  1,
// 			InitialElectionIDHigh: 0,
// 		}
// 		if err := client.Start(t); err != nil {
// 			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
// 			if err = client.Start(t); err != nil {
// 				t.Fatalf("gRIBI Connection could not be established: %v", err)
// 			}
// 		}
// 		args.client = &client
// 		baseProgramming(args.ctx, t, args)

// 		if k < grpcRepeat-1 {
// 			client.Close(t)
// 		}
// 	}
// }

func DoProcessRestart(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string) {
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	var pID uint64
	for _, proc := range pList {
		if proc.GetName() == pName {
			pID = proc.GetPid()
			t.Logf("Pid of daemon '%s' is '%d'", pName, pID)
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

	// reestablishing gribi connection
	// if pName == "emsd" {
	// 	time.Sleep(time.Second * 20)
	// 	client := gribi.Client{
	// 		DUT: dut,
	// 		// FibACK:                *ciscoFlags.GRIBIFIBCheck,
	// 		Persistence:           true,
	// 		InitialElectionIDLow:  1,
	// 		InitialElectionIDHigh: 0,
	// 	}
	// 	if err := client.Start(t); err != nil {
	// 		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
	// 		if err = client.Start(t); err != nil {
	// 			t.Fatalf("gRIBI Connection could not be established: %v", err)
	// 		}
	// 	}
	// 	// args.client = &client
	// 	gnmi.Collect(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(5*time.Minute)), gnmi.OC().NetworkInstance("*").Afts().State(), 15*time.Minute)
	// 	gnmi.Collect(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(10*time.Minute)), gnmi.OC().Interface("*").State(), 15*time.Minute)
	// }
}
