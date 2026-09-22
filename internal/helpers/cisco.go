// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package helpers provides Cisco-specific helper utilities for test cases.
package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"

	spb "github.com/openconfig/gnoi/system"
)

const (
	maxRebootTime = 30 // minutes
)

// ConfigureHwProfile configures Cisco hardware profile with route scale on the DUT.
func ConfigureHwProfile(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	ciscoConfig := `
		hw-module profile route scale lpm tcam-banks
		customshowtech GRPC_CUSTOM
		command show health gsp
		command show health sysdb
		command show tech-support gsp
		command show tech-support cfgmgr
		command show tech-support ofa
		command show tech-support pfi
		command show tech-support spi
		command show tech-support mgbl
		command show tech-support sysdb
		command show tech-support appmgr
		command show tech-support fabric
		command show tech-support yserver
		command show tech-support interface
		command show tech-support platform-fwd
		command show tech-support linux networking
		command show tech-support ethernet interfaces
		command show tech-support fabric link-include
		command show tech-support p2p-ipc process appmgr
		command show tech-support insight include-database
		command show tech-support lpts
		command show tech-support parser
		command show tech-support telemetry model-driven
		lpts pifib hardware police flow tpa rate 20000 
		`
	GnmiCLIConfig(t, dut, ciscoConfig)
	RebootDUT(t, dut)
	return nil
}

// ConfigureDefaultHwProfile resets Cisco hardware profile to default on the DUT.
func ConfigureDefaultHwProfile(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	ciscoConfig := `
	    no hw-module profile route scale lpm tcam-banks
		`
	GnmiCLIConfig(t, dut, ciscoConfig)
	RebootDUT(t, dut)
	return nil
}

// RebootDUT performs a cold reboot on the DUT and waits for it to come back up.
func RebootDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	}
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(t.Context())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v %v", bootTimeBeforeReboot, time.Now())
	t.Log("Sending reboot request to DUT")

	ctxWithTimeout, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()
	_, err = gnoiClient.System().Reboot(ctxWithTimeout, rebootRequest)
	defer gnoiClient.System().CancelReboot(t.Context(), &spb.CancelRebootRequest{})
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}

	// Wait for the device to become reachable again.
	deviceBootStatus(t, dut)
	t.Logf("Device is reachable, waiting for boot time to update.")

	bootTimeAfterReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time after reboot: %v", bootTimeAfterReboot)
}

// deviceBootStatus waits for the DUT to boot up by polling telemetry.
func deviceBootStatus(t *testing.T, dut *ondatra.DUTDevice) {
	startReboot := time.Now()
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
}
