// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package setrequest_switchover_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/diffutils"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

const (
	maxSwitchoverTime        = 120
	maxConfigConvergenceTime = 30
	controlcardType          = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSwitchoverSetRequest(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	swReadyVal, swReadyPres := gnmi.Await(t, dut, switchoverReady.State(), 15*time.Minute, true).Val()
	if !swReadyPres || !swReadyVal {
		t.Errorf("switchoverReady.Get(t): got %v, want true", swReadyVal)
	}

	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}

	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, deviations.GNOISubcomponentPath(dut)),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)

	switchoverStartTime := time.Now()
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(switchoverStartTime).Seconds())
		time.Sleep(10 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(switchoverStartTime).Seconds()), uint64(maxSwitchoverTime); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(switchoverStartTime).Seconds())

	want := rpStandbyBeforeSwitch
	got := ""
	if deviations.GNOISubcomponentPath(dut) {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}

	wantConf := BuildConfig(t)
	setRequestStartTime := time.Now()
	gnmi.Update(t, dut, gnmi.OC().Config(), wantConf)
	if got := time.Since(setRequestStartTime).Seconds(); got > maxConfigConvergenceTime {
		t.Fatalf("Switchover took too long - want: %v, got: %v", maxConfigConvergenceTime, got)
	}

	gotConf := gnmi.Get(t, dut, gnmi.OC().Config())
	validateConfigMatch(t, wantConf, gotConf)
}

func validateConfigMatch(t *testing.T, want, got *oc.Root) {
	var dc diffutils.DiffCollector
	cmp.Diff(want, got,
		cmp.Reporter(&dc),
		cmpopts.IgnoreUnexported(oc.RoutingPolicy_PolicyDefinition_Statement_OrderedMap{}),
	)

	for _, de := range dc.Diff() {
		if de.Vx.IsValid() && !de.Vx.IsZero() {
			t.Errorf("Configuration differed (-want +got):\n%v", de.String())
		} else {
			t.Logf("Configuration ignored due to zero value (-want +got):\n%v", de.String())
		}
	}
}
