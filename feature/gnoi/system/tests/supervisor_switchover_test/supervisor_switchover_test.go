// Copyright 2022 Google LLC
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
package supervisor_switchover_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"

	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
)

const (
	maxSwitchoverTime = 900
	controlcardType   = telemetry.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = telemetry.PlatformTypes_ComponentRedundantRole_PRIMARY
	standbyController = telemetry.PlatformTypes_ComponentRedundantRole_SECONDARY
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  0) Check the number of route processor.
//     - Skip the test if the number of route processor is < 2.
//  1) Issue gnoi SwitchControlProcessor request to the chassis with dual supervisor.
//      - Set the path to to be standby RE/SUP name.
//  2) Validate the SwitchControlProcessorResponse has the new active supervisor as
//     the one specified in the request.
//  3) Validate the standby RE/SUP becomes the active after switchover
//  4) Validate that all connected ports are re-enabled.
//  5) Validate OC Switchover time/reason.
//     - /components/component[name=<supervisor>]/state/last-switchover-time
//     - /components/component[name=<supervisor>]/state/last-switchover-reason/trigger
//     - /components/component[name=<supervisor>]/state/last-switchover-reason/details
//
// Topology:
//   DUT
//
// Test notes:
//  - SwitchControlProcessor will switch from the current route processor to the
//    provided route processor. If the current route processor is the same as the
//    one provided it is a NOOP. If the target does not exist an error is
//    returned.
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestSupervisorSwitchover(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	supervisors := findComponentsByType(t, dut, controlcardType)
	t.Logf("Found supervisor list: %v", supervisors)
	// Only perform the switchover for the chassis with dual RPs/Supervisors.
	if len(supervisors) != 2 {
		t.Skipf("Dual RP/SUP is required on %v: got %v, want 2", dut.Model(), len(supervisors))
	}

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := findStandbyRP(t, dut, supervisors)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	intfsOperStatusUPBeforeSwitch := fetchOperStatusUPIntfs(t, dut)
	t.Logf("intfsOperStatusUP interfaces before switchover: %v", intfsOperStatusUPBeforeSwitch)
	if len(intfsOperStatusUPBeforeSwitch) == 0 {
		t.Errorf("Get the number of intfsOperStatusUP interfaces for %q: got 0, want > 0", dut.Name())
	}

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: &tpb.Path{
			Elem: []*tpb.PathElem{{Name: rpStandbyBeforeSwitch}},
		},
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := rpStandbyBeforeSwitch
	if got := switchoverResponse.GetControlProcessor().GetElem()[0].GetName(); got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}
	if got := switchoverResponse.GetVersion(); len(got) == 0 {
		t.Errorf("switchoverResponse.GetVersion(): got %v, want non-empty version", got)
	}
	if got := switchoverResponse.GetUptime(); got == 0 {
		t.Errorf("switchoverResponse.GetUptime(): got %v, want > 0", got)
	}

	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = dut.Telemetry().System().CurrentDatetime().Get(t)
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if uint64(time.Since(startSwitchover).Seconds()) > maxSwitchoverTime {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", time.Since(startSwitchover), maxSwitchoverTime)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	rpStandbyAfterSwitch, rpActiveAfterSwitch := findStandbyRP(t, dut, supervisors)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if rpActiveAfterSwitch != rpStandbyBeforeSwitch {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", rpActiveAfterSwitch, rpStandbyBeforeSwitch)
	}
	if rpStandbyAfterSwitch != rpActiveBeforeSwitch {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", rpStandbyAfterSwitch, rpActiveBeforeSwitch)
	}

	batch := dut.Telemetry().NewBatch()
	for _, port := range intfsOperStatusUPBeforeSwitch {
		dut.Telemetry().Interface(port).OperStatus().Batch(t, batch)
	}
	watch := batch.Watch(t, 5*time.Minute, func(val *telemetry.QualifiedDevice) bool {
		for _, port := range intfsOperStatusUPBeforeSwitch {
			if val.Val(t).GetInterface(port).GetOperStatus() != telemetry.Interface_OperStatus_UP {
				return false
			}
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("DUT did not reach target state withing %v: got %v", 5*time.Minute, val)
	}

	intfsOperStatusUPAfterSwitch := fetchOperStatusUPIntfs(t, dut)
	t.Logf("intfsOperStatusUP interfaces after switchover: %v", intfsOperStatusUPAfterSwitch)
	if diff := cmp.Diff(intfsOperStatusUPAfterSwitch, intfsOperStatusUPBeforeSwitch); diff != "" {
		t.Errorf("intfsOperStatusUP interfaces differed (-want +got):\n%v", diff)
	}

	t.Log("Validate OC Switchover time/reason.")
	activeRP := dut.Telemetry().Component(rpActiveAfterSwitch)
	if !activeRP.LastSwitchoverTime().Lookup(t).IsPresent() {
		t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got false, want true")
	} else {
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", activeRP.LastSwitchoverTime().Get(t))
	}

	if !activeRP.LastSwitchoverReason().Lookup(t).IsPresent() {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got false, want true")
	} else {
		lastSwitchoverReason := activeRP.LastSwitchoverReason().Get(t)
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
}

func findComponentsByType(t *testing.T, dut *ondatra.DUTDevice, cType telemetry.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	components := dut.Telemetry().ComponentAny().Name().Get(t)
	var s []string
	for _, c := range components {
		lookupType := dut.Telemetry().Component(c).Type().Lookup(t)
		if !lookupType.IsPresent() {
			t.Logf("Component %s type is not found", c)
		} else {
			componentType := lookupType.Val(t)
			t.Logf("Component %s has type: %v", c, componentType)

			switch v := componentType.(type) {
			case telemetry.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
				if v == cType {
					s = append(s, c)
				}
			default:
				t.Fatalf("Expected component type to be a hardware component, got (%T, %v)", componentType, componentType)
			}
		}
	}
	return s
}

func findStandbyRP(t *testing.T, dut *ondatra.DUTDevice, supervisors []string) (string, string) {
	var activeRP, standbyRP string
	for _, supervisor := range supervisors {
		watch := dut.Telemetry().Component(supervisor).RedundantRole().Watch(
			t, 10*time.Minute, func(val *telemetry.QualifiedE_PlatformTypes_ComponentRedundantRole) bool {
				return val.IsPresent()
			})
		if val, ok := watch.Await(t); !ok {
			t.Fatalf("DUT did not reach target state within %v: got %v", 10*time.Minute, val)
		}
		role := dut.Telemetry().Component(supervisor).RedundantRole().Get(t)
		t.Logf("Component(supervisor).RedundantRole().Get(t): %v, Role: %v", supervisor, role)
		if role == standbyController {
			standbyRP = supervisor
		} else if role == activeController {
			activeRP = supervisor
		} else {
			t.Fatalf("Expected controller %s to be active or standby, got %v", supervisor, role)
		}
	}
	if standbyRP == "" || activeRP == "" {
		t.Fatalf("Expected non-empty activeRP and standbyRP, got activeRP: %v, standbyRP: %v", activeRP, standbyRP)
	}
	t.Logf("Detected activeRP: %v, standbyRP: %v", activeRP, standbyRP)

	return standbyRP, activeRP
}

func fetchOperStatusUPIntfs(t *testing.T, dut *ondatra.DUTDevice) []string {
	intfsOperStatusUP := []string{}
	intfs := dut.Telemetry().InterfaceAny().Name().Get(t)
	for _, intf := range intfs {
		operStatus := dut.Telemetry().Interface(intf).OperStatus().Lookup(t)
		if operStatus.IsPresent() && operStatus.Val(t) == telemetry.Interface_OperStatus_UP {
			intfsOperStatusUP = append(intfsOperStatusUP, intf)
		}
	}
	sort.Strings(intfsOperStatusUP)
	return intfsOperStatusUP
}
