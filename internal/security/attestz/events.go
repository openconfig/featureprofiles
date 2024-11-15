// Copyright 2024 Google LLC
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

package attestz

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	frpb "github.com/openconfig/gnoi/factory_reset"
	gnps "github.com/openconfig/gnoi/system"
	"github.com/openconfig/gnoigo/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnoi"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	maxRebootTime             = 15  // Unit is minutes
	maxFactoryResetRebootTime = 40  // Unit is minutes, includes time for bootz
	maxSwitchoverTime         = 900 // Unit is seconds
)

// SwitchoverReady waits for control cards to become switchover ready.
func SwitchoverReady(t *testing.T, dut *ondatra.DUTDevice, activeCard, standbyCard string) {
	switchoverReady := gnmi.OC().Component(activeCard).SwitchoverReady()
	_, ok := gnmi.Watch(t, dut, switchoverReady.State(), 30*time.Minute, func(val *ygnmi.Value[bool]) bool {
		ready, present := val.Val()
		return present && ready
	}).Await(t)
	if !ok {
		activeCardPath := gnmi.OC().Component(activeCard).State()
		standbyCardPath := gnmi.OC().Component(standbyCard).State()
		fptest.LogQuery(t, "Active card reported state", activeCardPath, gnmi.Get[*oc.Component](t, dut, activeCardPath))
		fptest.LogQuery(t, "Standby card reported state", standbyCardPath, gnmi.Get[*oc.Component](t, dut, standbyCardPath))
		t.Fatal("Cards are not synchronized.")
	}
}

func waitForBootup(t *testing.T, dut *ondatra.DUTDevice, maxBootTime uint64) {
	startTime := time.Now()
	for {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get[string](t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Log("Reboot is started ...")
			break
		}
		t.Log("Wait for reboot ...")
		time.Sleep(30 * time.Second)
	}
	t.Logf("Wait for DUT to boot up by polling the telemetry output ...")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startTime).Minutes())
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get[string](t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}
		if uint64(time.Since(startTime).Minutes()) > maxBootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startTime), maxBootTime)
		}
		time.Sleep(30 * time.Second)
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startTime).Minutes())
}

// RebootDut reboots the dut.
func RebootDut(t *testing.T, dut *ondatra.DUTDevice) {
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect to gNOI server, err: %v", err)
	}
	rebootRequest := &gnps.RebootRequest{
		Method: gnps.RebootMethod_COLD,
		Force:  true,
	}
	bootTimeBeforeReboot := gnmi.Get[uint64](t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", time.Unix(0, int64(bootTimeBeforeReboot)))
	currentTime := gnmi.Get[string](t, dut, gnmi.OC().System().CurrentDatetime().State())
	t.Logf("DUT system time before reboot : %s", currentTime)
	res, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected error: %v", err)
	}
	t.Logf("Reboot Response %v ", PrettyPrint(res))
	waitForBootup(t, dut, maxRebootTime)
}

// FactoryResetDut factory resets the dut.
func FactoryResetDut(t *testing.T, dut *ondatra.DUTDevice) {
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	res, err := gnoiClient.FactoryReset().Start(context.Background(), &frpb.StartRequest{FactoryOs: false, ZeroFill: false})
	if err != nil {
		t.Fatalf("Failed to initiate factory reset on the device. error : %v ", err)
	}
	t.Logf("Factory reset response: %v ", PrettyPrint(res))
	waitForBootup(t, dut, maxFactoryResetRebootTime)
}

// SwitchoverCards performs control card switchover.
func SwitchoverCards(t *testing.T, dut *ondatra.DUTDevice, activeCard, standbyCard string) {
	// Wait for cards to become switchover ready.
	SwitchoverReady(t, dut, activeCard, standbyCard)
	switchoverResponse := gnoi.Execute(t, dut, system.NewSwitchControlProcessorOperation().Path(components.GetSubcomponentPath(standbyCard, deviations.GNOISubcomponentPath(dut))))
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v", PrettyPrint(switchoverResponse))
	startSwitchover := time.Now()
	t.Logf("Wait for new primary controller to boot up by polling the telemetry output ...")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get[string](t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Controller switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if uint64(time.Since(startSwitchover).Seconds()) > maxSwitchoverTime {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", time.Since(startSwitchover), maxSwitchoverTime)
		}
	}
	t.Logf("Controller switchover time: %.2f seconds.", time.Since(startSwitchover).Seconds())
}
