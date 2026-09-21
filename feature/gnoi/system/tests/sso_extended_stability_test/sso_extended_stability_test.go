// Copyright 2026 Google LLC
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

package sso_extended_stability_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	spb "github.com/openconfig/gnoi/system"
)

const (
	maxSwitchoverTime = 900
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	bgpName           = "BGP"
	ptBGP             = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
)

var (
	possibleCriticalProcs = []string{
		// Arista
		"AsicResourceMgr", "SandL3Ni", "FcRouteEs", "Bgp-main", "Rib", "Sysdb", "SuperServer", "SandL3Unicast", "FapNi",
		// Cisco
		"fretta_dpa", "bgp", "cef",
		// Juniper
		"rpd", "aftd-chassis", "fpc",
		// Nokia
		"sr_engine", "sr_mgmtd",
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func getDeviceByName(t *testing.T, top gosnappi.Config, name string) gosnappi.Device {
	for _, d := range top.Devices().Items() {
		if d.Name() == name {
			return d
		}
	}
	t.Fatalf("Device %s not found in OTG top", name)
	return nil
}

func configureBGPv4Routes(peer gosnappi.BgpV4Peer, nextHop string, name string, prefix string, prefixLen uint32) {
	routes := peer.V4Routes().Add().SetName(name)
	routes.SetNextHopIpv4Address(nextHop).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(prefix).
		SetPrefix(prefixLen).
		SetCount(1)
}

func findRunningCriticalProcesses(t *testing.T, dut *ondatra.DUTDevice) []string {
	var found []string
	pList := gnmi.GetAll[*oc.System_Process](t, dut, gnmi.OC().System().ProcessAny().State())
	existing := make(map[string]bool)
	for _, proc := range pList {
		existing[proc.GetName()] = true
	}
	for _, name := range possibleCriticalProcs {
		if existing[name] {
			found = append(found, name)
		}
	}
	return found
}

func getCriticalProcessInfos(t *testing.T, dut *ondatra.DUTDevice, pNames []string, preferred map[string]*system.ProcessInfo) (map[string]*system.ProcessInfo, error) {
	t.Helper()
	pList := gnmi.GetAll[*oc.System_Process](t, dut, gnmi.OC().System().ProcessAny().State())
	results := make(map[string]*system.ProcessInfo)

	nameMap := make(map[string]bool)
	for _, name := range pNames {
		nameMap[name] = true
	}

	// First match by preferred PID if provided (avoids picking a different instance when multiple processes share a name).
	if preferred != nil {
		for _, proc := range pList {
			pName := proc.GetName()
			if pref, ok := preferred[pName]; ok && proc.GetPid() == pref.Pid {
				results[pName] = &system.ProcessInfo{
					Pid:         proc.GetPid(),
					StartTime:   proc.GetStartTime(),
					MemoryUsage: proc.GetMemoryUsage(),
				}
			}
		}
	}

	// Fallback to matching by process name (picking lowest PID deterministically).
	for _, proc := range pList {
		pName := proc.GetName()
		if !nameMap[pName] {
			continue
		}
		if existing, ok := results[pName]; !ok || (preferred == nil && proc.GetPid() < existing.Pid) {
			if _, lockedByPref := preferred[pName]; lockedByPref && ok {
				continue
			}
			results[pName] = &system.ProcessInfo{
				Pid:         proc.GetPid(),
				StartTime:   proc.GetStartTime(),
				MemoryUsage: proc.GetMemoryUsage(),
			}
		}
	}

	for _, name := range pNames {
		if _, ok := results[name]; !ok {
			return nil, fmt.Errorf("process %q not found", name)
		}
	}
	return results, nil
}

func verifyNoQoSDrops(t *testing.T, dut *ondatra.DUTDevice, ports []string, queueNames []string, baselines map[string]uint64) map[string]uint64 {
	current := make(map[string]uint64)
	batch := gnmi.OCBatch()
	for _, port := range ports {
		for _, qName := range queueNames {
			batch.AddPaths(gnmi.OC().Qos().Interface(port).Output().Queue(qName).DroppedPkts())
		}
	}
	lookupRes := gnmi.Lookup(t, dut, batch.State())
	results, _ := lookupRes.Val()
	for _, port := range ports {
		for _, qName := range queueNames {
			key := fmt.Sprintf("%s-%s", port, qName)
			var val uint64
			if results != nil {
				if qos := results.GetQos(); qos != nil {
					if q := qos.GetInterface(port); q != nil {
						if out := q.GetOutput(); out != nil {
							if queue := out.GetQueue(qName); queue != nil {
								val = queue.GetDroppedPkts()
							}
						}
					}
				}
			}
			current[key] = val
			if baselines != nil && val > baselines[key] {
				t.Errorf("Port %s Queue %s dropped packets increased: got %d, baseline %d", port, qName, val, baselines[key])
			}
		}
	}
	return current
}

func performSwitchover(t *testing.T, dut *ondatra.DUTDevice, controllerCards []string) (string, string) {
	rpStandbyBefore, rpActiveBefore := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby before switchover: %v, rpActive before: %v", rpStandbyBefore, rpActiveBefore)

	gnmi.Await(t, dut, gnmi.OC().Component(rpActiveBefore).SwitchoverReady().State(), 30*time.Minute, true)

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBefore, useNameOnly),
	}
	t.Logf("Sending switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover: %v", err)
	}
	t.Logf("SwitchControlProcessor response: %v", switchoverResponse)

	// Wait for DUT to complete SSO role transition and gNMI to become reachable on the new active RP.
	// Use gnmi.Watch (which sets a per-call context.WithTimeout) rather than gnmi.Get
	// (which uses context.Background() and can block indefinitely on a half-open proxy stream during RP failover).
	switchoverDeadline := time.Now().Add(maxSwitchoverTime * time.Second)
	for time.Now().Before(switchoverDeadline) {
		remaining := time.Until(switchoverDeadline)
		if remaining <= 0 {
			break
		}
		var rolesSwitched bool
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			watchTimeout := 30 * time.Second
			if rem := time.Until(switchoverDeadline); rem < watchTimeout {
				watchTimeout = rem
			}
			_, ok0 := gnmi.Watch(t, dut, gnmi.OC().System().CurrentDatetime().State(), watchTimeout, func(val *ygnmi.Value[string]) bool {
				return val.IsPresent()
			}).Await(t)
			if !ok0 {
				return
			}
			_, ok1 := gnmi.Watch(t, dut, gnmi.OC().Component(rpStandbyBefore).RedundantRole().State(), watchTimeout, func(val *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
				role, ok := val.Val()
				return ok && role == oc.Platform_ComponentRedundantRole_PRIMARY
			}).Await(t)
			if !ok1 {
				return
			}
			_, ok2 := gnmi.Watch(t, dut, gnmi.OC().Component(rpActiveBefore).RedundantRole().State(), watchTimeout, func(val *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
				role, ok := val.Val()
				return ok && role == oc.Platform_ComponentRedundantRole_SECONDARY
			}).Await(t)
			rolesSwitched = ok0 && ok1 && ok2
		}); errMsg != nil {
			t.Logf("Transient gNMI error while waiting for RP role transition: %s", *errMsg)
			time.Sleep(5 * time.Second)
		}
		if rolesSwitched {
			break
		}
	}

	rpStandbyAfter, rpActiveAfter := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby after switchover: %v, rpActive after: %v", rpStandbyAfter, rpActiveAfter)
	if rpActiveAfter != rpStandbyBefore {
		t.Errorf("Expected active RP after switchover to be %s, got %s", rpStandbyBefore, rpActiveAfter)
	}

	return rpActiveAfter, rpStandbyAfter
}

func runPostSSOVerification(t *testing.T, dut *ondatra.DUTDevice, criticalProcs []string, _ map[string]*system.ProcessInfo, qosBaselines map[string]uint64, qosPorts []string, qosQueues []string, controllerCards []string) {
	t.Log("Starting 10 minutes validation post-switchover...")

	// Capture baseline process state on the newly active supervisor right after switchover.
	activeRPBaselines, err := getCriticalProcessInfos(t, dut, criticalProcs, nil)
	if err != nil {
		t.Errorf("Failed to query post-switchover baseline process info: %v", err)
	} else {
		for name, pInfo := range activeRPBaselines {
			t.Logf("Post-switchover active RP process %s baseline: PID=%d, StartTime=%d, Memory=%d", name, pInfo.Pid, pInfo.StartTime, pInfo.MemoryUsage)
		}
	}

	prevMemory := make(map[string]uint64)
	memIncreases := make(map[string]int)
	for name, b := range activeRPBaselines {
		prevMemory[name] = b.MemoryUsage
	}

	numPolls := 0
	for min := 2; min <= 10; min += 2 {
		time.Sleep(2 * time.Minute)
		numPolls++
		t.Logf("Verifying process and device health at %d minutes mark...", min)

		infos, err := getCriticalProcessInfos(t, dut, criticalProcs, activeRPBaselines)
		if err != nil {
			t.Errorf("Failed to query process info: %v", err)
			continue
		}

		for _, name := range criticalProcs {
			info, ok := infos[name]
			if !ok {
				t.Errorf("Process info for %s not found in results", name)
				continue
			}

			baseline, hasBaseline := activeRPBaselines[name]
			if !hasBaseline {
				activeRPBaselines[name] = info
				prevMemory[name] = info.MemoryUsage
				continue
			}

			// Crash Detection (If PID or StartTime changes on the active RP during soak, process crashed and restarted)
			if info.Pid != baseline.Pid {
				t.Errorf("Process %s PID changed from %d to %d (crash detected)", name, baseline.Pid, info.Pid)
			}
			if info.StartTime != baseline.StartTime {
				t.Errorf("Process %s StartTime changed from %d to %d (crash/restart detected)", name, baseline.StartTime, info.StartTime)
			}

			// Memory Leak Detection (Track whether memory keeps increasing over time compared to baseline)
			t.Logf("Process %s Memory: Baseline = %d, Previous = %d, Current = %d", name, baseline.MemoryUsage, prevMemory[name], info.MemoryUsage)
			if info.MemoryUsage > prevMemory[name] {
				memIncreases[name]++
			}
			prevMemory[name] = info.MemoryUsage
		}

		// Verify QoS queue drop telemetry does not increase (If there is no change in baseline then no drops)
		verifyNoQoSDrops(t, dut, qosPorts, qosQueues, qosBaselines)
	}

	for _, name := range criticalProcs {
		baseline, ok := activeRPBaselines[name]
		if !ok || baseline.MemoryUsage == 0 {
			continue
		}
		finalMem := prevMemory[name]
		if numPolls > 0 && memIncreases[name] == numPolls && finalMem > baseline.MemoryUsage {
			pctIncrease := float64(finalMem-baseline.MemoryUsage) / float64(baseline.MemoryUsage)
			if pctIncrease > 0.10 {
				t.Errorf("Process %s memory kept increasing over time compared to baseline: got %d, baseline %d (%.2f%% increase)", name, finalMem, baseline.MemoryUsage, pctIncrease*100)
			}
		}
	}

	t.Log("Validating the new active RP is switchover ready...")
	rpStandbyAfter, rpActiveAfter := components.FindStandbyControllerCard(t, dut, controllerCards)
	t.Logf("Detected rpStandby after switchover sequence: %v, rpActive after: %v", rpStandbyAfter, rpActiveAfter)
	switchoverReady := gnmi.OC().Component(rpActiveAfter).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
}

func configureVRFsAndBgp(t *testing.T, dut *ondatra.DUTDevice, bs *cfgplugins.BGPSession, p1, p2, p3, p4 *ondatra.Port) {
	// 1. Delete BGP protocol under default network-instance
	defaultNiName := deviations.DefaultNetworkInstance(dut)
	if !deviations.ExplicitEnableBGPOnDefaultVRF(dut) {
		if _, ok := bs.DUTConf.NetworkInstance[defaultNiName]; ok {
			delete(bs.DUTConf.NetworkInstance[defaultNiName].Protocol, oc.NetworkInstance_Protocol_Key{
				Identifier: ptBGP,
				Name:       bgpName,
			})
			if len(bs.DUTConf.NetworkInstance[defaultNiName].Protocol) == 0 {
				bs.DUTConf.NetworkInstance[defaultNiName].Protocol = nil
			}
		}
	} else {
		bs.DUTConf.GetOrCreateNetworkInstance(defaultNiName).GetOrCreateProtocol(ptBGP, bgpName).GetOrCreateBgp().GetOrCreateGlobal().SetAs(65000)
	}
	if deviations.BgpAfiSafiInDefaultNiBeforeOtherNi(dut) {
		// The parent address family must be initialized in the default network
		// instance before IPv4 unicast can be enabled in the L3VRFs. Only the
		// global config is added; neighbors stay in the VRFs.
		defaultGlobal := bs.DUTConf.GetOrCreateNetworkInstance(defaultNiName).GetOrCreateProtocol(ptBGP, bgpName).GetOrCreateBgp().GetOrCreateGlobal()
		defaultGlobal.SetAs(65000)
		defaultGlobal.SetRouterId(bs.DUTPorts[0].IPv4)
		defaultGlobal.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		defaultGlobal.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_UNICAST).Enabled = ygot.Bool(true)
	}

	// 2. Configure L3VRFs and interfaces on DUTConf
	assignIntf := func(ni *oc.NetworkInstance, p *ondatra.Port) {
		id := p.Name()
		if deviations.InterfaceRefInterfaceIDFormat(dut) {
			id = fmt.Sprintf("%s.0", p.Name())
		}
		niIntf := ni.GetOrCreateInterface(id)
		niIntf.Id = ygot.String(id)
		niIntf.Interface = ygot.String(p.Name())
		niIntf.Subinterface = ygot.Uint32(0)
	}

	transitNi := bs.DUTConf.GetOrCreateNetworkInstance("TRANSIT_VRF")
	transitNi.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	assignIntf(transitNi, p1)
	assignIntf(transitNi, p2)

	decapNi := bs.DUTConf.GetOrCreateNetworkInstance("DECAP_TE_VRF")
	decapNi.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	assignIntf(decapNi, p3)
	assignIntf(decapNi, p4)

	if deviations.BgpAfiSafiInDefaultNiBeforeOtherNi(dut) {
		// With the VPN parent address family in the default network instance,
		// each L3VRF needs a route distinguisher before its BGP address family
		// can be activated.
		transitNi.SetRouteDistinguisher("65000:1")
		decapNi.SetRouteDistinguisher("65000:2")
	}

	// 3. Configure BGP protocols and Graceful Restart on the VRFs
	transitBgpProto := transitNi.GetOrCreateProtocol(ptBGP, bgpName)
	transitBgp := transitBgpProto.GetOrCreateBgp()
	transitGlobal := transitBgp.GetOrCreateGlobal()
	transitGlobal.As = ygot.Uint32(65000)
	transitGlobal.RouterId = ygot.String(bs.DUTPorts[0].IPv4)
	transitGR := transitGlobal.GetOrCreateGracefulRestart()
	transitGR.Enabled = ygot.Bool(true)
	transitGR.RestartTime = ygot.Uint16(120)
	transitGR.StaleRoutesTime = ygot.Uint16(300)
	transitGlobal.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	transitPg := transitBgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1)
	transitPg.PeerAs = ygot.Uint32(65001)
	transitPg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	decapBgpProto := decapNi.GetOrCreateProtocol(ptBGP, bgpName)
	decapBgp := decapBgpProto.GetOrCreateBgp()
	decapGlobal := decapBgp.GetOrCreateGlobal()
	decapGlobal.As = ygot.Uint32(65000)
	decapGlobal.RouterId = ygot.String(bs.DUTPorts[2].IPv4)
	decapGR := decapGlobal.GetOrCreateGracefulRestart()
	decapGR.Enabled = ygot.Bool(true)
	decapGR.RestartTime = ygot.Uint16(120)
	decapGR.StaleRoutesTime = ygot.Uint16(300)
	decapGlobal.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	decapPg := decapBgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1)
	decapPg.PeerAs = ygot.Uint32(65002)
	decapPg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	// Create PERMIT-ALL routing policy
	rp := bs.DUTConf.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition("SSO-PERMIT-ALL")
	stmt, err := pdef.AppendNewStatement("SSO-PERMIT-ALL-STMT")
	if err != nil {
		t.Fatalf("Failed to create routing policy statement: %v", err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	// Assign BGP neighbors to respective VRFs BGP config
	for i, otgPort := range bs.ATEPorts {
		peerAddress := otgPort.IPv4
		var nbr *oc.NetworkInstance_Protocol_Bgp_Neighbor

		// Reuse neighbor object generated from WithEBGP or build one
		if i == 0 || i == 1 {
			nbr = transitBgp.GetOrCreateNeighbor(peerAddress)
			nbr.PeerAs = ygot.Uint32(65001)
			nbr.PeerGroup = ygot.String(cfgplugins.BGPPeerGroup1)
		} else {
			nbr = decapBgp.GetOrCreateNeighbor(peerAddress)
			nbr.PeerAs = ygot.Uint32(65002)
			nbr.PeerGroup = ygot.String(cfgplugins.BGPPeerGroup1)
		}
		nbr.Enabled = ygot.Bool(true)

		nAfiSafi := nbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		nAfiSafi.Enabled = ygot.Bool(true)

		// Apply route policy to the neighbor.
		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			nbrPolicy := nbr.GetOrCreateApplyPolicy()
			nbrPolicy.SetExportPolicy([]string{"SSO-PERMIT-ALL"})
			nbrPolicy.SetImportPolicy([]string{"SSO-PERMIT-ALL"})
		}

		if !deviations.RoutePolicyUnderAFIUnsupported(dut) {
			nbrPolicy := nAfiSafi.GetOrCreateApplyPolicy()
			nbrPolicy.SetExportPolicy([]string{"SSO-PERMIT-ALL"})
			nbrPolicy.SetImportPolicy([]string{"SSO-PERMIT-ALL"})
		}
	}

	if deviations.PeerGroupDefEbgpVrfUnsupported(dut) {
		transitBgp.PeerGroup = nil
		for _, nbr := range transitBgp.Neighbor {
			nbr.PeerGroup = nil
		}
		decapBgp.PeerGroup = nil
		for _, nbr := range decapBgp.Neighbor {
			nbr.PeerGroup = nil
		}
	}
}

func configureQoS(t *testing.T, dut *ondatra.DUTDevice, bs *cfgplugins.BGPSession, p1, p2, p3, p4 *ondatra.Port) {
	// 4. Configure QoS egress queue management profiles and map to all output ports
	qos := bs.DUTConf.GetOrCreateQos()

	allQueues := []string{"NC1", "AF4", "AF3", "AF2", "AF1", "BE0", "BE1"}
	if deviations.QOSBufferAllocationConfigRequired(dut) {
		for i, qName := range allQueues {
			qos.GetOrCreateForwardingGroup("target-group-" + qName).SetOutputQueue(qName)
			qGlobal := qos.GetOrCreateQueue(qName)
			qGlobal.SetName(qName)
			if deviations.QOSQueueRequiresID(dut) {
				qGlobal.QueueId = ygot.Uint8(uint8(len(allQueues) - i))
			}
		}
	} else {
		qos.GetOrCreateForwardingGroup("target-group-AF4").SetOutputQueue("AF4")
		qos.GetOrCreateForwardingGroup("target-group-BE0").SetOutputQueue("BE0")
		qAF4Global := qos.GetOrCreateQueue("AF4")
		qAF4Global.SetName("AF4")
		qBE0Global := qos.GetOrCreateQueue("BE0")
		qBE0Global.SetName("BE0")
		if deviations.QOSQueueRequiresID(dut) {
			// Queue IDs map to traffic classes; strict-priority queues must
			// occupy contiguous traffic classes starting at 7 (NC1=7, AF4=6).
			for i, qName := range allQueues {
				qGlobal := qos.GetOrCreateQueue(qName)
				qGlobal.SetName(qName)
				qGlobal.QueueId = ygot.Uint8(uint8(len(allQueues) - i))
			}
		}
	}

	if !deviations.QosRedUnsupported(dut) {
		af4Profile := qos.GetOrCreateQueueManagementProfile("AF4_PROFILE")
		af4Profile.SetName("AF4_PROFILE")
		wredAF4 := af4Profile.GetOrCreateWred().GetOrCreateUniform()
		wredAF4.SetMinThreshold(80000)
		wredAF4.SetMaxThreshold(3000000)
		wredAF4.SetMaxDropProbabilityPercent(100)
		wredAF4.SetEnableEcn(true)
		if !deviations.DropWeightLeavesUnsupported(dut) {
			wredAF4.SetDrop(false)
		}

		be0Profile := qos.GetOrCreateQueueManagementProfile("BE0_PROFILE")
		be0Profile.SetName("BE0_PROFILE")
		wredBE0 := be0Profile.GetOrCreateWred().GetOrCreateUniform()
		wredBE0.SetMinThreshold(80000)
		wredBE0.SetMaxThreshold(3000000)
		wredBE0.SetMaxDropProbabilityPercent(100)
		wredBE0.SetEnableEcn(true)
		if !deviations.DropWeightLeavesUnsupported(dut) {
			wredBE0.SetDrop(false)
		}
	}

	if deviations.QosSchedulerConfigRequired(dut) {
		schedulerPolicy := qos.GetOrCreateSchedulerPolicy("scheduler")
		schedulerPolicy.SetName("scheduler")

		sAF4 := schedulerPolicy.GetOrCreateScheduler(0)
		sAF4.SetSequence(0)
		sAF4.SetPriority(oc.Scheduler_Priority_STRICT)
		// Traffic class 7 (NC1) must be the highest strict priority, followed
		// contiguously by AF4.
		qos.GetOrCreateForwardingGroup("target-group-NC1").SetOutputQueue("NC1")
		inNC1 := sAF4.GetOrCreateInput("NC1")
		inNC1.SetId("NC1")
		inNC1.SetInputType(oc.Input_InputType_QUEUE)
		inNC1.SetQueue("NC1")
		inNC1.SetWeight(7)
		inAF4 := sAF4.GetOrCreateInput("AF4")
		inAF4.SetId("AF4")
		inAF4.SetInputType(oc.Input_InputType_QUEUE)
		inAF4.SetQueue("AF4")
		inAF4.SetWeight(6)

		sBE0 := schedulerPolicy.GetOrCreateScheduler(1)
		sBE0.SetSequence(1)
		sBE0.SetPriority(oc.Scheduler_Priority_UNSET)
		inBE0 := sBE0.GetOrCreateInput("BE0")
		inBE0.SetId("BE0")
		inBE0.SetInputType(oc.Input_InputType_QUEUE)
		inBE0.SetQueue("BE0")
		inBE0.SetWeight(4)
	}

	for _, port := range []string{p1.Name(), p2.Name(), p3.Name(), p4.Name()} {
		intf := qos.GetOrCreateInterface(port)
		intf.SetInterfaceId(port)
		intf.GetOrCreateInterfaceRef().Interface = ygot.String(port)
		if deviations.InterfaceRefConfigUnsupported(dut) {
			intf.InterfaceRef = nil
		}

		output := intf.GetOrCreateOutput()
		if deviations.QosSchedulerConfigRequired(dut) {
			output.GetOrCreateSchedulerPolicy().SetName("scheduler")
			// Every queue referenced by the scheduler must be enabled on the
			// interface.
			output.GetOrCreateQueue("NC1").SetName("NC1")
		}
		if deviations.QOSBufferAllocationConfigRequired(dut) {
			bufferProfile := qos.GetOrCreateBufferAllocationProfile("bufferAllocationProfile")
			for _, qName := range allQueues {
				bufferProfile.GetOrCreateQueue(qName).SetStaticSharedBufferLimit(uint32(268435456))
				qOut := output.GetOrCreateQueue(qName)
				qOut.SetName(qName)
				if !deviations.QosRedUnsupported(dut) {
					if qName == "AF4" {
						qOut.SetQueueManagementProfile("AF4_PROFILE")
					} else {
						qOut.SetQueueManagementProfile("BE0_PROFILE")
					}
				}
			}
			output.SetBufferAllocationProfile("bufferAllocationProfile")
		} else {
			qAF4 := output.GetOrCreateQueue("AF4")
			qAF4.SetName("AF4")

			qBE0 := output.GetOrCreateQueue("BE0")
			qBE0.SetName("BE0")

			if !deviations.QosRedUnsupported(dut) {
				qAF4.SetQueueManagementProfile("AF4_PROFILE")
				qBE0.SetQueueManagementProfile("BE0_PROFILE")
			}
		}
	}
}

func configureOTGBgpAndTraffic(t *testing.T, bs *cfgplugins.BGPSession) {
	// 5. Configure OTG BGP Route Advertisements
	dev1 := getDeviceByName(t, bs.ATETop, "port1")
	dev2 := getDeviceByName(t, bs.ATETop, "port2")
	dev3 := getDeviceByName(t, bs.ATETop, "port3")
	dev4 := getDeviceByName(t, bs.ATETop, "port4")

	peer1 := dev1.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
	peer2 := dev2.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
	peer3 := dev3.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
	peer4 := dev4.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]

	peer1.SetAsNumber(65001).GracefulRestart().SetEnableGr(true).SetRestartTime(120)
	peer2.SetAsNumber(65001).GracefulRestart().SetEnableGr(true).SetRestartTime(120)
	peer3.SetAsNumber(65002).GracefulRestart().SetEnableGr(true).SetRestartTime(120)
	peer4.SetAsNumber(65002).GracefulRestart().SetEnableGr(true).SetRestartTime(120)

	configureBGPv4Routes(peer1, bs.ATEPorts[0].IPv4, "port1_routes", "198.51.100.0", 24)
	configureBGPv4Routes(peer2, bs.ATEPorts[1].IPv4, "port2_routes", "198.51.101.0", 24)
	configureBGPv4Routes(peer3, bs.ATEPorts[2].IPv4, "port3_routes", "198.51.102.0", 24)
	configureBGPv4Routes(peer4, bs.ATEPorts[3].IPv4, "port4_routes", "198.51.103.0", 24)

	// 6. Configure OTG Traffic Flows (AF4 in TRANSIT_VRF and BE0 in DECAP_TE_VRF)
	bs.ATETop.Flows().Clear()
	flowAF4 := bs.ATETop.Flows().Add().SetName("AF4_Flow")
	flowAF4.Metrics().SetEnable(true)
	flowAF4.TxRx().Device().
		SetTxNames([]string{dev1.Name() + ".IPv4"}).
		SetRxNames([]string{dev2.Name() + ".IPv4"})
	ethAF4 := flowAF4.Packet().Add().Ethernet()
	ethAF4.Src().SetValue(bs.ATEPorts[0].MAC)
	ipAF4 := flowAF4.Packet().Add().Ipv4()
	ipAF4.Src().SetValue(bs.ATEPorts[0].IPv4)
	ipAF4.Dst().SetValue("198.51.101.1")
	ipAF4.Priority().Dscp().Phb().SetValue(32)

	flowBE0 := bs.ATETop.Flows().Add().SetName("BE0_Flow")
	flowBE0.Metrics().SetEnable(true)
	flowBE0.TxRx().Device().
		SetTxNames([]string{dev3.Name() + ".IPv4"}).
		SetRxNames([]string{dev4.Name() + ".IPv4"})
	ethBE0 := flowBE0.Packet().Add().Ethernet()
	ethBE0.Src().SetValue(bs.ATEPorts[2].MAC)
	ipBE0 := flowBE0.Packet().Add().Ipv4()
	ipBE0.Src().SetValue(bs.ATEPorts[2].IPv4)
	ipBE0.Dst().SetValue("198.51.103.1")
	ipBE0.Priority().Dscp().Phb().SetValue(0)
}

func TestSSOSoftwareStability(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Init BGPSession
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount4, nil)
	bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST}, []string{"port1", "port2", "port3", "port4"}, true, false)

	p1 := bs.OndatraDUTPorts[0]
	p2 := bs.OndatraDUTPorts[1]
	p3 := bs.OndatraDUTPorts[2]
	p4 := bs.OndatraDUTPorts[3]

	configureVRFsAndBgp(t, dut, bs, p1, p2, p3, p4)
	if !deviations.QosRedUnsupported(dut) {
		configureQoS(t, dut, bs, p1, p2, p3, p4)
	}
	configureOTGBgpAndTraffic(t, bs)

	t.Cleanup(func() {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("TRANSIT_VRF").Config())
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DECAP_TE_VRF").Config())
	})
	// Post configuration to DUT, verify port status, and start ATE protocols
	if err := bs.PushDUT(t); err != nil {
		t.Fatalf("Failed to push DUT config: %v", err)
	}
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		for _, p := range []*ondatra.Port{p1, p2, p3, p4} {
			gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Config(), bs.DUTConf.GetInterface(p.Name()))
		}
	}
	bs.ATE.OTG().PushConfig(t, bs.ATETop)
	for _, p := range []*ondatra.Port{p1, p2, p3, p4} {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		if deviations.InterfaceRefInterfaceIDFormat(dut) {
			gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).Subinterface(0).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		}
		t.Logf("DUT port %s (%s) is UP", p.ID(), p.Name())
	}
	bs.PushAndStartATE(t)

	t.Log("Verify DUT BGP sessions established in VRFs")
	for i, otgPort := range bs.ATEPorts {
		vrf := "TRANSIT_VRF"
		if i >= 2 {
			vrf = "DECAP_TE_VRF"
		}
		statePath := gnmi.OC().NetworkInstance(vrf).Protocol(ptBGP, bgpName).Bgp().Neighbor(otgPort.IPv4).SessionState().State()
		gnmi.Watch(t, dut, statePath, 5*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, present := val.Val()
			return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
	}

	t.Log("Verify OTG BGP sessions established")
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

	t.Log("Wait for ARP resolution on OTG")
	otgutils.WaitForARP(t, bs.ATE.OTG(), bs.ATETop, "IPv4")

	// SYS-6.1.1 - Extended Post-SSO Traffic and Process Health Soak Test
	t.Log("=== SYS-6.1.1 - Extended Post-SSO Traffic and Process Health Soak Test ===")

	// Step 1 - Start Background Traffic and Record Process State
	t.Log("Step 1 - Start Background Traffic and Record Process State")
	t.Log("Waiting for BGP traffic to converge and stabilize with 0% loss...")
	startConv := time.Now()
	for {
		if time.Since(startConv) > 60*time.Second {
			t.Fatalf("Traffic did not stabilize with 0%% loss within 60s")
		}
		bs.ATE.OTG().StartTraffic(t)
		for _, flow := range []string{"AF4_Flow", "BE0_Flow"} {
			gnmi.Watch(t, bs.ATE.OTG(), gnmi.OTG().Flow(flow).Counters().InPkts().State(), 15*time.Second, func(val *ygnmi.Value[uint64]) bool {
				pkts, ok := val.Val()
				return ok && pkts >= 100
			}).Await(t)
		}
		bs.ATE.OTG().StopTraffic(t)

		converged := true
		for _, flow := range []string{"AF4_Flow", "BE0_Flow"} {
			loss := otgutils.GetFlowLossPct(t, bs.ATE.OTG(), flow, 10*time.Second)
			if loss > 0.0 {
				converged = false
				t.Logf("Traffic not yet stabilized: flow %s has loss %f%%", flow, loss)
				break
			}
		}
		if converged {
			t.Log("Traffic achieved 0% continuous loss.")
			break
		}
	}

	// Start continuous background traffic for the duration of the test (resets flow counters cleanly).
	bs.ATE.OTG().StartTraffic(t)

	// 7. Find critical hardware and routing processes to monitor
	criticalProcs := findRunningCriticalProcesses(t, dut)
	t.Logf("Monitoring critical processes: %v", criticalProcs)
	initialProcInfos, err := getCriticalProcessInfos(t, dut, criticalProcs, nil)
	if err != nil {
		t.Fatalf("Failed to query initial process info: %v", err)
	}
	for name, pInfo := range initialProcInfos {
		t.Logf("Initial active RP process %s state: PID=%d, StartTime=%d, Memory=%d", name, pInfo.Pid, pInfo.StartTime, pInfo.MemoryUsage)
	}

	qosPorts := []string{p1.Name(), p2.Name(), p3.Name(), p4.Name()}
	qosQueues := []string{"AF4", "BE0"}
	qosBaselines := verifyNoQoSDrops(t, dut, qosPorts, qosQueues, nil)

	// 8. Find Supervisor controller cards
	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller cards: %v", controllerCards)
	if len(controllerCards) < 2 {
		t.Skipf("Skip test, not enough controller cards for switchover on %v: got %d, want >= 2", dut.Model(), len(controllerCards))
	}

	// Step 2 - Trigger Supervisor Switchover
	t.Log("Step 2 - Trigger Supervisor Switchover")
	performSwitchover(t, dut, controllerCards)
	runPostSSOVerification(t, dut, criticalProcs, initialProcInfos, qosBaselines, qosPorts, qosQueues, controllerCards)

	// Step 3 - Soak Phase
	t.Log("Step 3 - Soak Phase")
	performSwitchover(t, dut, controllerCards)
	runPostSSOVerification(t, dut, criticalProcs, initialProcInfos, qosBaselines, qosPorts, qosQueues, controllerCards)

	// Step 4 - Validation with pass/fail criteria
	t.Log("Step 4 - Validation with pass/fail criteria")
	t.Log("Stopping traffic...")
	bs.ATE.OTG().StopTraffic(t)

	// Log traffic stats and verify final traffic loss is 0%
	otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
	otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
	for _, flow := range []string{"AF4_Flow", "BE0_Flow"} {
		loss := otgutils.GetFlowLossPct(t, bs.ATE.OTG(), flow, 10*time.Second)
		if loss > float64(deviations.BGPTrafficTolerance(dut)) {
			t.Errorf("Final forwarding validation failed: flow %s has loss %f%%, want <= %d%%", flow, loss, deviations.BGPTrafficTolerance(dut))
		}
	}

	// Verify no QoS queue drops increase
	verifyNoQoSDrops(t, dut, qosPorts, qosQueues, qosBaselines)
}
