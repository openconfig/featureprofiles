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

// drained_configuration_convergence_time_test is used to verify bgp test scenarios
// as given in gnmi1.3 testcase.
package drained_configuration_convergence_time_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/experimental/system/gnmi/benchmarking/ate_tests/internal/setup"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	asPathRepeatValue      = 3
	aclStatement2          = "20"
	aclStatement3          = "30"
	setAspathPrependPolicy = "SET-ASPATH-PREPEND"
	setMedPolicy           = "SET-MED"
	bgpMed                 = 25
)

// configureSetMED function us used to configure routing policy to set MED on DUT
// TODO : SetMED is not supported: https://github.com/openconfig/featureprofiles/issues/759
func configureSetMED(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// clear existing policies on dut
	dut.Config().RoutingPolicy().Replace(t, nil)
	d := &telemetry.Device{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef5 := rp.GetOrCreatePolicyDefinition(setMedPolicy)
	actions5 := pdef5.GetOrCreateStatement(aclStatement3).GetOrCreateActions()
	// TODO : SetMED is not supported: https://github.com/openconfig/featureprofiles/issues/759
	//setMedBGP := actions5.GetOrCreateBgpActions()
	//setMedBGP.SetMed = ygot.Uint32(bgpMed)
	actions5.PolicyResult = telemetry.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	dut.Config().RoutingPolicy().Replace(t, rp)
}

// configureSetASPath function is used to configure route policy set-as-path prepend on DUT
func configureSetASPath(t *testing.T, dut *ondatra.DUTDevice) {
	//clear existing policies on dut
	dut.Config().RoutingPolicy().Replace(t, nil)
	d := &telemetry.Device{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef5 := rp.GetOrCreatePolicyDefinition(setAspathPrependPolicy)
	actions5 := pdef5.GetOrCreateStatement(aclStatement2).GetOrCreateActions()
	aspend := actions5.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend()
	aspend.Asn = ygot.Uint32(setup.DutAS)
	aspend.RepeatN = ygot.Uint8(asPathRepeatValue)
	actions5.PolicyResult = telemetry.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	dut.Config().RoutingPolicy().Replace(t, rp)
}

// TODO : verifyBGPAsPath is to Validate AS Path attribute using bgp rib telemetry on ATE
// https://github.com/openconfig/ondatra/issues/45
func verifyBGPAsPath(t *testing.T, dut *ondatra.DUTDevice) {
	ate := ondatra.ATE(t, "ate")
	dutPolicyConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).
		Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
		PeerGroup(setup.PeerGrpName).ApplyPolicy().ExportPolicy()

	// Build wantArray to compare the diff
	/*var wantArray []uint32
	for i := 0; i < setup.RouteCount; i++ {
		wantArray = append(wantArray, setup.DutAS, setup.DutAS, setup.DutAS, setup.DutAS, setup.AteAS)
	}*/

	//Start the timer.
	start := time.Now()
	dutPolicyConfPath.Replace(t, []string{setAspathPrependPolicy})
	t.Run("BGP-MED-Verification", func(t *testing.T) {
		// at := ate.Telemetry()
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				//port1 is ingress, skip verification on ingress port
				continue
			}

			statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).
				Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
			prefixesv4 := statePath.Neighbor(setup.AteIPPool[ap.ID()].String()).
				AfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
			if gotSent := prefixesv4.Sent().Get(t); gotSent < setup.RouteCount {
				t.Errorf("Sent prefixes from DUT to neighbor %v is mismatch: got %v, want %v", setup.AteIPPool[ap.ID()].String(), gotSent, setup.RouteCount)
			}

			// TODO: https://github.com/openconfig/ondatra/issues/45
			/*rib := at.NetworkInstance(ap.Name()).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, fmt.Sprintf("bgp-%s", ap.Name())).Bgp().Rib()
			pref := rib.AfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
				Neighbor(setup.DutIPPool[ap.ID()].String()).AdjRibInPre().RouteAnyPrefix(0).Prefix().Get(t)
			asPath := rib.AttrSet(0).AsPath().Get(t)

			if diff := cmp.Diff(wantArray, asPath); diff != "" {
				t.Errorf("obtained MED on ATE is not as expected, got %v, want %v, prefixes %v", med, wantArray, pref)
			}*/

		}
	})

	//End the timer and calculate time
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply as path prepend policy is  %v", elapsed)

}

// verifyBGPSetMED is to Validate MED attribute using bgp rib telemetry on ATE
func verifyBGPSetMED(t *testing.T, dut *ondatra.DUTDevice) {
	ate := ondatra.ATE(t, "ate")

	dutPolicyConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).
		Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
		PeerGroup(setup.PeerGrpName).ApplyPolicy().ExportPolicy()

	// Build wantArray to compare the diff
	//var wantArray []uint32
	//for i := 0; i < setup.RouteCount; i++ {
	//	wantArray = append(wantArray, bgpMed)
	//}

	// Start the timer.
	start := time.Now()
	dutPolicyConfPath.Replace(t, []string{setMedPolicy})

	t.Run("BGP-MED-Verification", func(t *testing.T) {
		//at := ate.Telemetry()
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				//port1 is ingress, skip verification on ingress port
				continue
			}

			statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).
				Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
			prefixesv4 := statePath.Neighbor(setup.AteIPPool[ap.ID()].String()).
				AfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
			if gotSent := prefixesv4.Sent().Get(t); gotSent < setup.RouteCount {
				t.Errorf("Sent prefixes from DUT to neighbor %v is mismatch: got %v, want %v", setup.AteIPPool[ap.ID()].String(), gotSent, setup.RouteCount)
			} else {
				t.Logf("received prefixes on ATE: %v", gotSent)
			}

			/*rib := at.NetworkInstance(ap.Name()).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, fmt.Sprintf("bgp-%s", ap.Name())).Bgp().Rib()
			pref := rib.AfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
				Neighbor(setup.DutIPPool[ap.ID()].String()).AdjRibInPre().RouteAnyPrefix(0).Prefix().Get(t)
			med := rib.AttrSetAny().Med().Get(t)

			if diff := cmp.Diff(wantArray, med); diff != "" {
				t.Errorf("obtained MED on ATE is not as expected, got %v, want %v, prefixes %v", med, wantArray, pref)
			}*/
		}
	})

	//End the timer and calculate time taken to apply setMED
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply routing policy is  %v", elapsed)
}

// TestEstablish is to configure Interface, BGP and ISIS configurations
// on DUT using gnmi set request. It also verifies for bgp and isis adjacencies.
func TestEstablish(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setup.BuildIPPool(t)

	// Configure Network instance type on DUT
	t.Log("Configure Network Instance")
	dutConfNIPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance)
	dutConfNIPath.Type().Replace(t, telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	// Cleanup exisitng bgp and isis configs on DUT
	dutBGPPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	dutBGPPath.Delete(t)
	dutISISPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.IsisInstance).Isis()
	dutISISPath.Delete(t)

	setup.BuildOCUpdate(t)

	// Configure ATE with Interfaces, BGP, ISIS configs
	ate := ondatra.ATE(t, "ate")
	setup.ConfigureATE(t, ate)

	// Verify BGP Session state , should be in ESTABLISHED State
	setup.VerifyBgpTelemetry(t, dut)

	// Verify ISIS adjacency
	setup.VerifyISISTelemetry(t, dut)
}

// TestBGPBenchmarking is test time taken to apply set as path prepend and set med routing
// policies on routes in bgp rib. Verification of routing policy is done on ATE using bgp
// rib table
func TestBGPBenchmarking(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Cleanup existing policy details
	dutPolicyConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(setup.PeerGrpName).ApplyPolicy()
	dutPolicyConfPath.ExportPolicy().Delete(t)
	dut.Config().RoutingPolicy().Delete(t)

	t.Logf("Configure MED routing policy")
	configureSetMED(t)

	t.Logf("Verify time taken to apply MED to all routes in bgp rib")
	verifyBGPSetMED(t, dut)

	// Cleanup existing policy details
	dutPolicyConfPath.ExportPolicy().Replace(t, nil)
	dut.Config().RoutingPolicy().Replace(t, nil)

	t.Logf("Configure SET-AS-PATH routing policy")
	configureSetASPath(t, dut)

	t.Logf("Verify time taken to apply SET-AS-PATH to all routes in bgp rib")
	verifyBGPAsPath(t, dut)
}
