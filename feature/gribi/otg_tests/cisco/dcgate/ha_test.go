package dcgate_test

import (
	"context"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
)

var (
	prefixes   = []string{}
	rpfo_count = 0 // used to track rpfo_count if its more than 10 then reset to 0 and reload the HW
)

const (
	with_scale            = false                    // run entire script with or without scale (Support not yet coded)
	with_RPFO             = true                     // run entire script with or without RFPO
	base_config           = "case2_decap_encap_exit" // Will run all the tcs with set base programming case, options : case1_backup_decap, case2_decap_encap_exit, case3_decap_encap, case4_decap_encap_recycle
	active_rp             = "0/RP0/CPU0"
	standby_rp            = "0/RP1/CPU0"
	lc                    = "0/RP0/CPU0" //"0/2/CPU0" // set value for lc_oir tc, if empty it means no lc, example: 0/0/CPU0
	process_restart_count = 1
	microdropsRepeat      = 1
	programming_RFPO      = 1
)

// testRPFO is the main function to test RPFO
func testRPFO(t *testing.T, dut *ondatra.DUTDevice) {

	client := gribi.Client{
		DUT:                   dut,
		FibACK:                *ciscoFlags.GRIBIFIBCheck,
		Persistence:           true,
		InitialElectionIDLow:  1,
		InitialElectionIDHigh: 0,
	}
	defer client.Close(t)
	if err := client.Start(t); err != nil {
		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		if err = client.Start(t); err != nil {
			t.Fatalf("gRIBI Connection could not be established: %v", err)
		}
	}
	// ctx := context.Background()

	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
		randomItems := client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	for i := 0; i < programming_RFPO; i++ {

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			rpfo(t, dut, &client, true)
		}
	}
}

func rpfo(t *testing.T, dut *ondatra.DUTDevice, client *gribi.Client, gribi_reconnect bool) {

	// reload the HW is rfpo count is 10 or more
	if rpfo_count == 10 {
		gnoiClient := dut.RawAPIs().GNOI(t)
		rebootRequest := &spb.RebootRequest{
			Method: spb.RebootMethod_COLD,
			Force:  true,
		}
		rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
		t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
		if err != nil {
			t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
		}
		rpfo_count = 0
		time.Sleep(time.Minute * 20)
	}
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
	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	for {
		if err := client.Start(t); err != nil {
			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		} else {
			t.Logf("gRIBI Connection established")
			switchoverRequest := &spb.SwitchControlProcessorRequest{
				ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
			}
			t.Logf("switchoverRequest: %v", switchoverRequest)
			switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
			if err != nil {
				t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
			}
			if err == nil {
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
				break
			}
		}
		time.Sleep(time.Minute * 2)
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

	// reestablishing gribi connection
	if gribi_reconnect {
		client.Start(t)
	}
}

// // shut/unshut the interfaces
// func flapInterface(t *testing.T, args *testArgs, intfs []string, flap bool) {
// 	for _, intf := range intfs {
// 		path := gnmi.OC().Interface(intf).Enabled()
// 		gnmi.Update(t, args.dut, path.Config(), flap)
// 	}
// }
