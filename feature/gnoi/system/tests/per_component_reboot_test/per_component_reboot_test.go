// Copyright 2022 Google LLC
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

package per_component_reboot_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	linecardType      = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
	fabricType        = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  1) Issue gnoi.system Reboot to chassis with
//     - Delay: Not set.
//     - message: Not set.
//     - method: Only the COLD method is required to be supported by all targets.
//     - subcomponents: Standby RP/supervisor or linecard name.
//  2) Set the subcomponent to a standby RP (supervisor).
//     - Verify that the standby RP has rebooted and the uptime has been reset.
//  3) Set the subcomponent to a a field-removable linecard in the system.
//     - Verify that the line card has rebooted and the uptime has been reset.
//
// Topology:
//   DUT
//
// Test notes:
//  - Reboot causes the target to reboot, possibly at some point in the future.
//    If the method of reboot is not supported then the Reboot RPC will fail.
//    If the reboot is immediate the command will block until the subcomponents
//    have restarted.
//    If a reboot on the active control processor is pending the service must
//    reject all other reboot requests.
//    If a reboot request for active control processor is initiated with other
//    pending reboot requests it must be rejected.
//  - Only standby RP/supervisor reboot is tested
//    - Active RP/RP/supervisor reboot might not be supported for some platforms.
//    - Chassis reboot or RP switchover should be performed instead of active
//      RP/RP/supervisor reboot in real world.
//
//  - TODO: Check the uptime has been reset after the reboot.
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestStandbyControllerCardReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	rpStandby, rpActive := components.FindStandbyRP(t, dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandby, rpActive)

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{
			components.GetSubcomponentPath(rpStandby, useNameOnly),
		},
	}

	t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
	startReboot := time.Now()
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform component reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

	t.Logf("Wait for a minute to allow the sub component's reboot process to start")
	time.Sleep(1 * time.Minute)

	watch := gnmi.Watch(t, dut, gnmi.OC().Component(rpStandby).RedundantRole().State(), 10*time.Minute, func(val *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
		return val.IsPresent()
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("DUT did not reach target state within %v: got %v", 10*time.Minute, val)
	}
	t.Logf("Standby controller boot time: %.2f seconds", time.Since(startReboot).Seconds())

	// TODO: Check the standby RP uptime has been reset.
}

func TestLinecardReboot(t *testing.T) {
	const linecardBoottime = 10 * time.Minute
	dut := ondatra.DUT(t, "dut")

	lcs := components.FindComponentsByType(t, dut, linecardType)
	t.Logf("Found linecard list: %v", lcs)

	var validCards []string
	// don't consider the empty linecard slots.
	if len(lcs) > *args.NumLinecards {
		for _, lc := range lcs {
			empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(lc).Empty().State()).Val()
			if !ok || (ok && !empty) {
				validCards = append(validCards, lc)
			}
		}
	} else {
		validCards = lcs
	}
	if *args.NumLinecards >= 0 && len(validCards) != *args.NumLinecards {
		t.Errorf("Incorrect number of linecards: got %v, want exactly %v (specified by flag)", len(validCards), *args.NumLinecards)
	}

	if got := len(validCards); got == 0 {
		t.Skipf("Not enough linecards for the test on %v: got %v, want > 0", dut.Model(), got)
	}

	t.Logf("Find a removable line card to reboot.")
	var removableLinecard string
	for _, lc := range validCards {
		t.Logf("Check if %s is removable", lc)
		if got := gnmi.Lookup(t, dut, gnmi.OC().Component(lc).Removable().State()).IsPresent(); !got {
			t.Logf("Detected non-removable line card: %v", lc)
			continue
		}
		if got := gnmi.Get(t, dut, gnmi.OC().Component(lc).Removable().State()); got {
			t.Logf("Found removable line card: %v", lc)
			removableLinecard = lc
		}
	}
	if removableLinecard == "" {
		t.Fatalf("Component(lc).Removable().Get(t): got none, want non-empty")
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{
			components.GetSubcomponentPath(removableLinecard, useNameOnly),
		},
	}

	intfsOperStatusUPBeforeReboot := helpers.FetchOperStatusUPIntfs(t, dut, *args.CheckInterfacesInBinding)
	t.Logf("OperStatusUP interfaces before reboot: %v", intfsOperStatusUPBeforeReboot)
	t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

	rebootDeadline := time.Now().Add(linecardBoottime)
	for retry := true; retry; {
		t.Log("Waiting for 10 seconds before checking.")
		time.Sleep(10 * time.Second)
		if time.Now().After(rebootDeadline) {
			retry = false
			break
		}
		resp, err := gnoiClient.System().RebootStatus(context.Background(), &spb.RebootStatusRequest{})
		switch {
		case status.Code(err) == codes.Unimplemented:
			t.Fatalf("Unimplemented RebootStatus() is not fully compliant with the Reboot spec.")
		case err == nil:
			retry = resp.GetActive()
		default:
			// any other error just sleep.
		}
	}

	t.Logf("Validate removable linecard %v status", removableLinecard)
	gnmi.Await(t, dut, gnmi.OC().Component(removableLinecard).Removable().State(), linecardBoottime, true)

	helpers.ValidateOperStatusUPIntfs(t, dut, intfsOperStatusUPBeforeReboot, 10*time.Minute)
	// TODO: Check the line card uptime has been reset.
}

// Reboot the fabric component on the DUT.
func TestFabricReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if deviations.GNOIFabricComponentRebootUnsupported(dut) {
		t.Skipf("Skipping test due to deviation deviation_gnoi_fabric_component_reboot_unsupported")
	}

	const fabricBootTime = 10 * time.Minute
	fabrics := components.FindComponentsByType(t, dut, fabricType)
	t.Logf("Found fabric components: %v", fabrics)

	t.Logf("Find a removable fabric component to reboot.")
	var removableFabric string
	for _, fabric := range fabrics {
		t.Logf("Check if %s is removable", fabric)
		if removable, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(fabric).Removable().State()).Val(); ok && removable {
			t.Logf("Found removable fabric component: %v", fabric)
			removableFabric = fabric
			break
		} else {
			t.Logf("Found non-removable fabric component: %v", fabric)
		}
	}
	if removableFabric == "" {
		t.Fatalf("Component(fabric).Removable().Get(t): got none, want non-empty")
	}

	// Fetch list of interfaces which are up prior to fabric component reboot.
	intfsOperStatusUPBeforeReboot := helpers.FetchOperStatusUPIntfs(t, dut, *args.CheckInterfacesInBinding)
	t.Logf("OperStatusUP interfaces before reboot: %v", intfsOperStatusUPBeforeReboot)

	// Fetch a new gnoi client.
	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{
			components.GetSubcomponentPath(removableFabric, useNameOnly),
		},
	}

	t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform fabric component reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

	rebootDeadline := time.Now().Add(fabricBootTime)
	for {
		t.Log("Waiting for 10 seconds before checking.")
		time.Sleep(10 * time.Second)
		if time.Now().After(rebootDeadline) {
			break
		}
		resp, err := gnoiClient.System().RebootStatus(context.Background(), &spb.RebootStatusRequest{})
		if status.Code(err) == codes.Unimplemented {
			t.Fatalf("Unimplemented RebootStatus() is not fully compliant with the Reboot spec.")
		}
		if !resp.GetActive() {
			break
		}
	}

	// Wait for the fabric component to come back up.
	t.Logf("Validate removable fabric component %v status", removableFabric)
	gnmi.Await(t, dut, gnmi.OC().Component(removableFabric).OperStatus().State(), fabricBootTime, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	t.Logf("Fabric component is active")
	helpers.ValidateOperStatusUPIntfs(t, dut, intfsOperStatusUPBeforeReboot, 5*time.Minute)
	// TODO: Check the fabric component uptime has been reset.
}
