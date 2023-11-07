// Copyright 2023 Google LLC
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
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/experimental/system/gnmi/benchmarking/internal/setup"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	asPathRepeatValue      = 3
	aclStatement2          = "20"
	aclStatement3          = "30"
	setASpathPrependPolicy = "SET-ASPATH-PREPEND"
	setMEDPolicy           = "SET-MED"
	setALLOWPolicy         = "ALLOW"
	bgpMED                 = 25
)

// setAllow is used to configure ALLOW routing policy on DUT.
func setAllow(t *testing.T, dut *ondatra.DUTDevice, d *oc.Root) {

	// Configure Allow Policy on DUT.
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(setALLOWPolicy)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// setMED is used to configure routing policy to set BGP MED on DUT.
func setMED(t *testing.T, dut *ondatra.DUTDevice, d *oc.Root) {

	// Configure SetMED on DUT.
	rp := d.GetOrCreateRoutingPolicy()
	pdef5 := rp.GetOrCreatePolicyDefinition(setMEDPolicy)
	stmt, err := pdef5.AppendNewStatement(aclStatement3)
	if err != nil {
		t.Fatal(err)
	}
	actions5 := stmt.GetOrCreateActions()
	setMedBGP := actions5.GetOrCreateBgpActions()
	setMedBGP.SetMed = oc.UnionUint32(bgpMED)
	actions5.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	if deviations.BGPSetMedRequiresEqualOspfSetMetric(dut) {
		actions5.GetOrCreateOspfActions().GetOrCreateSetMetric().SetMetric(bgpMED)
	}

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// setASPath is used to configure route policy set-as-path prepend on DUT.
func setASPath(t *testing.T, dut *ondatra.DUTDevice, d *oc.Root) {

	// Configure SetASPATH routing policy on DUT.
	rp := d.GetOrCreateRoutingPolicy()
	pdef5 := rp.GetOrCreatePolicyDefinition(setASpathPrependPolicy)
	stmt, err := pdef5.AppendNewStatement(aclStatement2)
	if err != nil {
		t.Fatal(err)
	}
	actions5 := stmt.GetOrCreateActions()
	actions5.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	aspend := actions5.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend()
	aspend.Asn = ygot.Uint32(setup.DUTAs)
	aspend.RepeatN = ygot.Uint8(asPathRepeatValue)

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	bgp := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	pg := bgp.GetOrCreatePeerGroup(setup.PeerGrpName)
	rpl := pg.GetOrCreateApplyPolicy()
	rpl.SetImportPolicy([]string{setALLOWPolicy})

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(setup.PeerGrpName).Config(), pg)

}

func setPolicyPeerGroup(t *testing.T, dut *ondatra.DUTDevice, d *oc.Root, policy []string) {

	// Set BGP Import policy
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	bgp := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	pg := bgp.GetOrCreatePeerGroup(setup.PeerGrpName)
	pg.PeerAs = ygot.Uint32(setup.ATEAs)
	pg.PeerGroupName = ygot.String(setup.PeerGrpName)
	afipg := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afipg.Enabled = ygot.Bool(true)
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		pgpolicy := pg.GetOrCreateApplyPolicy()
		pgpolicy.ImportPolicy = policy
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(setup.PeerGrpName).Config(), pg)

	} else {
		pgpolicy := afipg.GetOrCreateApplyPolicy()
		pgpolicy.ImportPolicy = policy
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(setup.PeerGrpName).Config(), pg)

	}
}

// isConverged function is used to check if ATE has received all the prefixes.
func isConverged(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, ap *ondatra.Port) {

	// Check if all prefixes are learned at ATE.
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
prefixLoop:
	for repeat := 4; repeat > 0; repeat-- {
		prefixesv4 := statePath.Neighbor(setup.ATEIPList[ap.ID()].String()).
			AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
		gotSent := gnmi.Get(t, dut, prefixesv4.Sent().State())
		switch {
		case gotSent == setup.RouteCount:
			t.Logf("Prefixes sent from ingress port are learnt at ATE dst port : %v are %v", setup.ATEIPList[ap.ID()].String(), setup.RouteCount)
			break prefixLoop
		case repeat > 0 && gotSent < setup.RouteCount:
			t.Logf("All the prefixes are not learnt , wait for 5 secs before retry.. got %v, want %v", gotSent, setup.RouteCount)
			time.Sleep(time.Second * 5)
		case repeat == 0 && gotSent < setup.RouteCount:
			t.Errorf("sent prefixes from DUT to neighbor %v is mismatch: got %v, want %v", setup.ATEIPList[ap.ID()].String(), gotSent, setup.RouteCount)
		}
	}

}

// verifyBGPAsPath is to Validate AS Path attribute using bgp rib telemetry on ATE.
func verifyBGPAsPath(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {

	// Start the timer.
	start := time.Now()
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
			PeerGroup(setup.PeerGrpName).ApplyPolicy().ExportPolicy()

		gnmi.Replace(t, dut, dutPolicyConfPath.Config(), []string{setASpathPrependPolicy})
	} else {
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
			PeerGroup(setup.PeerGrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ExportPolicy()
		gnmi.Replace(t, dut, dutPolicyConfPath.Config(), []string{setASpathPrependPolicy})
	}
	t.Run("BGP-AS-PATH Verification", func(t *testing.T) {
		at := gnmi.OC()
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				// port1 is ingress, skip verification on ingress port.
				continue
			}

			// Validate if all prefixes are received by ATE.
			isConverged(t, dut, ate, ap)

			rib := at.NetworkInstance(ap.Name()).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "0").Bgp().Rib()
			prefixPath := rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
				NeighborAny().AdjRibInPre().RouteAny().WithPathId(0).Prefix()

			gnmi.WatchAll(t, ate, prefixPath.State(), time.Minute, func(v *ygnmi.Value[string]) bool {
				_, present := v.Val()
				return present
			}).Await(t)

			singlepath := []uint32{setup.DUTAs, setup.DUTAs, setup.DUTAs, setup.DUTAs, setup.ATEAs2}
			_, ok := gnmi.WatchAll(t, ate, rib.AttrSetAny().AsSegmentMap().State(), 5*time.Minute, func(v *ygnmi.Value[map[uint32]*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet_AsSegment]) bool {
				val, present := v.Val()
				if present {
					for _, as := range val {
						if cmp.Equal(as.Member, singlepath) {
							return true
						}
					}
				}
				return false
			}).Await(t)
			if !ok {
				t.Errorf("Obtained AS path on ATE is not as expected")
			}
		}
	})

	// End the timer and calculate time.
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply as path prepend policy is  %v", elapsed)
}

// verifyBGPSetMED is to Validate MED attribute using bgp rib telemetry on ATE.
func verifyBGPSetMED(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {

	// Build wantSetMED to compare the diff.
	var wantSetMED []uint32
	for i := 0; i < setup.RouteCount; i++ {
		wantSetMED = append(wantSetMED, bgpMED)
	}

	// Start the timer.
	start := time.Now()
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
			PeerGroup(setup.PeerGrpName).ApplyPolicy().ExportPolicy()
		gnmi.Replace(t, dut, dutPolicyConfPath.Config(), []string{setMEDPolicy})
	} else {
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
			PeerGroup(setup.PeerGrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ExportPolicy()
		gnmi.Replace(t, dut, dutPolicyConfPath.Config(), []string{setMEDPolicy})
	}

	t.Run("BGP-MED-Verification", func(t *testing.T) {
		at := gnmi.OC()
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				continue
			}

			// Validate if all prefixes are received by ATE.
			isConverged(t, dut, ate, ap)
			rib := at.NetworkInstance(ap.Name()).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "0").Bgp().Rib()
			routeP := rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
				NeighborAny().AdjRibInPre().RouteAny().WithPathId(0)
			routes := gnmi.GetAll(t, ate, routeP.State())
			attrs := gnmi.GetAll(t, ate, rib.AttrSetAny().State())
			mask := net.IPv4Mask(255, 255, 255, 0)
			masked := net.ParseIP(setup.AdvertiseBGPRoutesv4).Mask(mask)
			var gotSetMED []uint32
			var pref []string
			for _, route := range routes {
				ip, _, _ := net.ParseCIDR(route.GetPrefix())
				pref = append(pref, route.GetPrefix())
				if ip.Mask(mask).Equal(masked) {
					idx := route.GetAttrIndex()
					if idx >= uint64(len(attrs)) {
						t.Errorf("Invalid attr-index %d for prefix: %s", idx, route.GetPrefix())
						continue
					}
					gotSetMED = append(gotSetMED, attrs[idx].GetMed())
				}
			}
			if diff := cmp.Diff(wantSetMED, gotSetMED); diff != "" {
				t.Errorf("obtained MED on ATE is not as expected, got %v, want %v, Prefixes %v", gotSetMED, wantSetMED, pref)
			}
		}
	})
	// End the timer and calculate time taken to apply setMED.
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply setMed routing policy is  %v", elapsed)
}

// TestEstablish is to configure Interface, BGP and ISIS configurations on DUT
// using gnmi set request. It also verifies for bgp and isis adjacencies.
func TestEstablish(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	dutConfigPath := gnmi.OC()

	t.Log("Configure Network Instance type to DEFAULT on DUT.")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	t.Log("Build Benchmarking BGP and ISIS test configs.")
	dutBenchmarkConfig := setup.BuildBenchmarkingConfig(t)
	if !deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.LogQuery(t, "Benchmarking configs to configure on DUT", dutConfigPath.Config(), dutBenchmarkConfig)
	}
	// Apply benchmarking configs on dut
	gnmi.Update(t, dut, dutConfigPath.Config(), dutBenchmarkConfig)

	t.Log("Configure ATE with Interfaces, BGP, ISIS configs.")
	ate := ondatra.ATE(t, "ate")
	setup.ConfigureATE(t, ate)

	t.Log("Verify BGP Session state , should be in ESTABLISHED State.")
	setup.VerifyBgpTelemetry(t, dut)

	t.Log("Verify ISIS adjacency state, should be UP.")
	setup.VerifyISISTelemetry(t, dut)
}

// TestBGPBenchmarking is test time taken to apply set as path prepend and set med routing
// policies on routes in bgp rib. Verification of routing policy is done on ATE using bgp
// rib table.
func TestBGPBenchmarking(t *testing.T) {

	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	// Cleanup existing policy details.
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(setup.PeerGrpName).ApplyPolicy()
		gnmi.Delete(t, dut, dutPolicyConfPath.ExportPolicy().Config())
		gnmi.Delete(t, dut, dutPolicyConfPath.ImportPolicy().Config())
	} else {
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(setup.PeerGrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
		gnmi.Delete(t, dut, dutPolicyConfPath.ExportPolicy().Config())
		gnmi.Delete(t, dut, dutPolicyConfPath.ImportPolicy().Config())
	}
	gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().Config())

	t.Logf("Configure Allow policy.")
	setAllow(t, dut, d)

	t.Logf("Configure MED routing policy.")
	setMED(t, dut, d)

	t.Logf("Configure Allow Import routing policy in BGP.")
	setPolicyPeerGroup(t, dut, d, []string{setALLOWPolicy})

	t.Logf("Verify time taken to apply MED to all routes in bgp rib.")
	verifyBGPSetMED(t, dut, ate)

	// Cleanup existing policy details.
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(setup.PeerGrpName).ApplyPolicy()
		gnmi.Delete(t, dut, dutPolicyConfPath.ExportPolicy().Config())
		gnmi.Delete(t, dut, dutPolicyConfPath.ImportPolicy().Config())
	} else {
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(setup.PeerGrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
		gnmi.Delete(t, dut, dutPolicyConfPath.ExportPolicy().Config())
		gnmi.Delete(t, dut, dutPolicyConfPath.ImportPolicy().Config())
	}
	gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().Config())

	t.Logf("Configure SET-AS-PATH routing policy.")
	setASPath(t, dut, d)

	t.Logf("Configure Allow Import routing policy in BGP.")
	setPolicyPeerGroup(t, dut, d, []string{setALLOWPolicy})

	t.Logf("Verify time taken to apply SET-AS-PATH to all routes in bgp rib.")
	verifyBGPAsPath(t, dut, ate)
}
