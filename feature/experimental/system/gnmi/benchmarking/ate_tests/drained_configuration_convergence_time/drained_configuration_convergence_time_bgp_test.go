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
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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

// setMED is used to configure routing policy to set MED on DUT
// TODO : SetMED is not supported: https://github.com/openconfig/featureprofiles/issues/759
func setMED(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Clear existing routing policies on DUT
	gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().Config())

	// Configure SetMED on DUT
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef5 := rp.GetOrCreatePolicyDefinition(setMedPolicy)
	actions5 := pdef5.GetOrCreateStatement(aclStatement3).GetOrCreateActions()
	//setMedBGP := actions5.GetOrCreateBgpActions()
	// TODO : SetMED is not supported: https://github.com/openconfig/featureprofiles/issues/759
	//setMedBGP.SetMed = ygot.Uint32(bgpMed)
	actions5.GetOrCreateBgpActions().SetLocalPref = ygot.Uint32(100)
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// setASPath is used to configure route policy set-as-path prepend on DUT
func setASPath(t *testing.T, dut *ondatra.DUTDevice) {
	// Clear existing policies on dut
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), nil)

	// Configure SetASPATH routing policy on DUT
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef5 := rp.GetOrCreatePolicyDefinition(setAspathPrependPolicy)
	actions5 := pdef5.GetOrCreateStatement(aclStatement2).GetOrCreateActions()
	aspend := actions5.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend()
	aspend.Asn = ygot.Uint32(setup.DutAS)
	aspend.RepeatN = ygot.Uint8(asPathRepeatValue)
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// verifyBGPAsPath is to Validate AS Path attribute using bgp rib telemetry on ATE
// TODO : https://github.com/openconfig/ondatra/issues/45
func verifyBGPAsPath(t *testing.T, dut *ondatra.DUTDevice) {
	ate := ondatra.ATE(t, "ate")

	dutPolicyConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
		PeerGroup(setup.PeerGrpName).ApplyPolicy().ExportPolicy()

	// Build wantArray to compare the diff
	/*var wantArray []uint32
	for i := 0; i < setup.RouteCount; i++ {
		wantArray = append(wantArray, setup.DutAS, setup.DutAS, setup.DutAS, setup.DutAS, setup.AteAS)
	}*/

	// Start the timer.
	start := time.Now()
	gnmi.Replace(t, dut, dutPolicyConfPath.Config(), []string{setAspathPrependPolicy})
	t.Run("BGP-AS-PATH Verification", func(t *testing.T) {
		//at := gnmi.OC()
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				//port1 is ingress, skip verification on ingress port
				continue
			}

			statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).
				Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
		prefixLoop:
			for repeat := 4; repeat > 0; repeat-- {
				prefixesv4 := statePath.Neighbor(setup.AteIPPool[ap.ID()].String()).
					AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
				gotSent := gnmi.Get(t, dut, prefixesv4.Sent().State())
				switch {
				case gotSent == setup.RouteCount:
					t.Logf("prefixes sent from ingress port are learnt at ATE dst port : %v", setup.AteIPPool[ap.ID()].String())
					break prefixLoop
				case repeat > 0 && gotSent < setup.RouteCount:
					t.Logf("all the prefixes are not learnt , wait for 5 secs before retry.. got %v, want %v", gotSent, setup.RouteCount)
					time.Sleep(time.Second * 5)
				case repeat == 0 && gotSent < setup.RouteCount:
					t.Errorf("Sent prefixes from DUT to neighbor %v is mismatch: got %v, want %v", setup.AteIPPool[ap.ID()].String(), gotSent, setup.RouteCount)
				}
			}

			// TODO: https://github.com/openconfig/ondatra/issues/45
			/*rib := at.NetworkInstance(ap.Name()).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, fmt.Sprintf("bgp-%s", ap.Name())).Bgp().Rib()
			prefixPath := rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
				Neighbor(setup.DutIPPool[ap.ID()].String()).AdjRibInPre().RouteAnyPrefix(0).Prefix()
			pref := gnmi.Get(t, dut, prefixPath.State())
			asPath := gnmi.Get(t, dut, rib.AttrSet(0).AsPath().State())

			if diff := cmp.Diff(wantArray, asPath); diff != "" {
				t.Errorf("obtained MED on ATE is not as expected, got %v, want %v, prefixes %v", asPath, wantArray, pref)
			}*/

		}
	})

	// End the timer and calculate time
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply as path prepend policy is  %v", elapsed)

}

// verifyBGPSetMED is to Validate MED attribute using bgp rib telemetry on ATE
func verifyBGPSetMED(t *testing.T, dut *ondatra.DUTDevice) {
	ate := ondatra.ATE(t, "ate")

	dutPolicyConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
		PeerGroup(setup.PeerGrpName).ApplyPolicy().ExportPolicy()

	// Build wantArray to compare the diff
	//var wantArray []uint32
	//for i := 0; i < setup.RouteCount; i++ {
	//	wantArray = append(wantArray, bgpMed)
	//}

	// Start the timer.
	start := time.Now()
	gnmi.Replace(t, dut, dutPolicyConfPath.Config(), []string{setMedPolicy})

	t.Run("BGP-MED-Verification", func(t *testing.T) {
		// at := gnmi.OC()
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				continue
			}

			statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).
				Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
		prefixLoop:
			for repeat := 4; repeat > 0; repeat-- {
				prefixesv4 := statePath.Neighbor(setup.AteIPPool[ap.ID()].String()).
					AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
				gotSent := gnmi.Get(t, dut, prefixesv4.Sent().State())
				switch {
				case gotSent == setup.RouteCount:
					t.Logf("prefixes sent from ingress port are learnt at ATE dst port : %v", setup.AteIPPool[ap.ID()].String())
					break prefixLoop
				case repeat > 0 && gotSent < setup.RouteCount:
					t.Logf("all the prefixes are not learnt , wait for 5 secs before retry.. got %v, want %v", gotSent, setup.RouteCount)
					time.Sleep(time.Second * 5)
				case repeat == 0 && gotSent < setup.RouteCount:
					t.Errorf("Sent prefixes from DUT to neighbor %v is mismatch: got %v, want %v", setup.AteIPPool[ap.ID()].String(), gotSent, setup.RouteCount)
				}
			}
			// TODO : SetMED is not supported: https://github.com/openconfig/featureprofiles/issues/759
			/*rib := at.NetworkInstance(ap.Name()).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, fmt.Sprintf("bgp-%s", ap.Name())).Bgp().Rib()
			prefixPath := rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
				Neighbor(setup.DutIPPool[ap.ID()].String()).AdjRibInPre().RouteAnyPrefix(0).Prefix()
			pref := gnmi.Get(t, dut, prefixPath.State())
			med := gnmi.Get(t, dut, rib.AttrSetAny().Med().State())

			if diff := cmp.Diff(wantArray, med); diff != "" {
				t.Errorf("obtained MED on ATE is not as expected, got %v, want %v, prefixes %v", med, wantArray, pref)
			}*/
		}
	})

	// End the timer and calculate time taken to apply setMED
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply setMed routing policy is  %v", elapsed)
}

// TestEstablish is to configure Interface, BGP and ISIS configurations
// on DUT using gnmi set request. It also verifies for bgp and isis adjacencies.
func TestEstablish(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setup.BuildIPPool(t)

	t.Log("Configure Network Instance type to DEFAULT on DUT")
	dutConfNIPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance)
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	t.Log("Cleanup exisitng bgp and isis configs on DUT before configuring test configs")
	dutBGPPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	gnmi.Delete(t, dut, dutBGPPath.Config())
	dutISISPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.IsisInstance).Isis()
	gnmi.Delete(t, dut, dutISISPath.Config())

	t.Log("Configure BGP and ISIS test configs")
	setup.BuildOCUpdate(t)

	t.Log("Coonfigure ATE with Interfaces, BGP, ISIS configs")
	ate := ondatra.ATE(t, "ate")
	setup.ConfigureATE(t, ate)

	t.Log("Verify BGP Session state , should be in ESTABLISHED State")
	setup.VerifyBgpTelemetry(t, dut)

	t.Log("Verify ISIS adjacency state, should be UP")
	setup.VerifyISISTelemetry(t, dut)
}

// TestBGPBenchmarking is test time taken to apply set as path prepend and set med routing
// policies on routes in bgp rib. Verification of routing policy is done on ATE using bgp
// rib table
func TestBGPBenchmarking(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Cleanup existing policy details
	dutPolicyConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(setup.PeerGrpName).ApplyPolicy()
	gnmi.Delete(t, dut, dutPolicyConfPath.ExportPolicy().Config())

	gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().Config())

	// TODO : SetMED is not supported: https://github.com/openconfig/featureprofiles/issues/759
	t.Logf("Configure MED routing policy")
	setMED(t)

	t.Logf("Verify time taken to apply MED to all routes in bgp rib")
	verifyBGPSetMED(t, dut)

	// Cleanup existing policy details
	gnmi.Delete(t, dut, dutPolicyConfPath.ExportPolicy().Config())
	gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().Config())

	t.Logf("Configure SET-AS-PATH routing policy")
	setASPath(t, dut)

	t.Logf("Verify time taken to apply SET-AS-PATH to all routes in bgp rib")
	verifyBGPAsPath(t, dut)
}
