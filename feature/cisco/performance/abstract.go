// Abstract Trigger Space to have Routines that could be re-usable for any Test Suite
package main

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/gnoi/system"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
)

func RestartEmsd(t *testing.T, dut *ondatra.DUTDevice) error {
	resp, err := dut.RawAPIs().GNOI(t).System().KillProcess(context.Background(), &system.KillProcessRequest{
		Name:    "emsd",
		Restart: true,
		Signal:  system.KillProcessRequest_SIGNAL_TERM,
	})
	if err != nil {
		return err
	}
	if resp == nil {
		t.Error("")
	}
	time.Sleep(30 * time.Second) // Allow time for restart
	return nil
}

func RouterReload(t *testing.T, dut *ondatra.DUTDevice) error {
	gnoiClient := dut.RawAPIs().GNOI(t)
	_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
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
	return nil
}
