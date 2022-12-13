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
	"sort"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
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

	supervisors := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found supervisor list: %v", supervisors)
	// Only perform the standby RP rebooting for the chassis with dual RPs/Supervisors.
	if len(supervisors) != 2 {
		t.Skipf("Dual RP/SUP is required on %v: got %v, want 2", dut.Model(), len(supervisors))
	}

	rpStandby, rpActive := findStandbyRP(t, dut, supervisors)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandby, rpActive)

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{
			{
				Elem: []*tpb.PathElem{{Name: rpStandby}},
			},
		},
	}

	t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
	startReboot := time.Now()
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform component reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

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
	if got := len(lcs); got == 0 {
		t.Errorf("Get number of Linecards on %v: got %v, want > 0", dut.Model(), got)
	}

	t.Logf("Find a removable line card to reboot.")
	var removableLinecard string
	for _, lc := range lcs {
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

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{
			{
				Elem: []*tpb.PathElem{{Name: removableLinecard}},
			},
		},
	}

	intfsOperStatusUPBeforeReboot := fetchOperStatusUPIntfs(t, dut)
	t.Logf("OperStatusUP interfaces before reboot: %v", intfsOperStatusUPBeforeReboot)
	t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

	rebootDeadline := time.Now().Add(linecardBoottime)
	for retry := true; retry; {
		t.Log("Wating for 10 seconds before checking.")
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

	t.Logf("Validate interface OperStatus.")
	batch := gnmi.OCBatch()
	for _, port := range intfsOperStatusUPBeforeReboot {
		batch.AddPaths(gnmi.OC().Interface(port).OperStatus())
	}
	watch := gnmi.Watch(t, dut, batch.State(), 5*time.Minute, func(val *ygnmi.Value[*oc.Root]) bool {
		root, present := val.Val()
		if !present {
			return false
		}
		for _, port := range intfsOperStatusUPBeforeReboot {
			if root.GetInterface(port).GetOperStatus() != oc.Interface_OperStatus_UP {
				return false
			}
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("DUT did not reach target state: got %v", val)
	}

	// TODO: Check the line card uptime has been reset.
}

func findStandbyRP(t *testing.T, dut *ondatra.DUTDevice, supervisors []string) (string, string) {
	var activeRP, standbyRP string
	for _, supervisor := range supervisors {
		watch := gnmi.Watch(t, dut, gnmi.OC().Component(supervisor).RedundantRole().State(), 5*time.Minute, func(val *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
			return val.IsPresent()
		})
		if val, ok := watch.Await(t); !ok {
			t.Fatalf("DUT did not reach target state within %v: got %v", 5*time.Minute, val)
		}
		role := gnmi.Get(t, dut, gnmi.OC().Component(supervisor).RedundantRole().State())
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
	intfs := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().Name().State())
	for _, intf := range intfs {
		operStatus := gnmi.Lookup(t, dut, gnmi.OC().Interface(intf).OperStatus().State())
		if status, present := operStatus.Val(); present && status == oc.Interface_OperStatus_UP {
			intfsOperStatusUP = append(intfsOperStatusUP, intf)
		}
	}
	sort.Strings(intfsOperStatusUP)
	return intfsOperStatusUP
}
