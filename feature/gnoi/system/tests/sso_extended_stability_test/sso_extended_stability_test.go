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

  "github.com/openconfig/ygot/ygot"
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

  spb "github.com/openconfig/gnoi/system"
)

const (
  maxSwitchoverTime = 900
  controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
  bgpName           = "BGP"
  ptBGP             = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
  rplPermitAll      = "PERMIT-ALL"
)

var (
  possibleCriticalProcs = []string{
    // Arista
    "AsicResourceMgr", "SandL3Ni", "FcRouteEs",
    // Cisco
    "fretta_dpa", "bgp", "cef",
    // Juniper
    "rpd", "aftd-chassis", "fpc",
    // Nokia
    "sr_engine", "bgp", "sr_mgmtd",
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

func verifyNoQoSDrops(t *testing.T, dut *ondatra.DUTDevice, ports []string, queueNames []string, baselines map[string]uint64) map[string]uint64 {
  current := make(map[string]uint64)
  for _, port := range ports {
    for _, qName := range queueNames {
      key := fmt.Sprintf("%s-%s", port, qName)
      val, present := gnmi.Lookup(t, dut, gnmi.OC().Qos().Interface(port).Output().Queue(qName).DroppedPkts().State()).Val()
      if !present {
        val = 0
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

  // Validate SwitchoverReady
  switchoverReady := gnmi.OC().Component(rpActiveBefore).SwitchoverReady()
  gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)

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

  startSwitchover := time.Now()
  t.Logf("Waiting for new active RP to boot up...")
  for {
    var currentTime string
    t.Logf("Time elapsed: %.2f seconds", time.Since(startSwitchover).Seconds())
    time.Sleep(30 * time.Second)
    if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
      currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
    }); errMsg != nil {
      t.Logf("Keep polling: %s", *errMsg)
    } else {
      t.Logf("RP switchover completed successfully. Router datetime: %v", currentTime)
      break
    }
    if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(maxSwitchoverTime); got >= want {
      t.Fatalf("Chassis supervisor switchover timed out: got %v, want < %v", got, want)
    }
  }

  rpStandbyAfter, rpActiveAfter := components.FindStandbyControllerCard(t, dut, controllerCards)
  t.Logf("Detected rpStandby after switchover: %v, rpActive after: %v", rpStandbyAfter, rpActiveAfter)
  if rpActiveAfter != rpStandbyBefore {
    t.Errorf("Expected active RP after switchover to be %s, got %s", rpStandbyBefore, rpActiveAfter)
  }

  return rpActiveAfter, rpStandbyAfter
}

func runPostSSOVerification(t *testing.T, dut *ondatra.DUTDevice, criticalProcs []string, baselines map[string]*system.ProcessInfo, qosBaselines map[string]uint64, qosPorts []string, qosQueues []string) {
  t.Log("Starting 10 minutes validation post-switchover...")
  for min := 2; min <= 10; min += 2 {
    time.Sleep(2 * time.Minute)
    t.Logf("Verifying process and device health at %d minutes mark...", min)

    infos, err := system.GetProcessInfo(t, dut, criticalProcs)
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

      baseline := baselines[name]
      // Crash Detection
      if info.Pid != baseline.Pid {
        t.Errorf("Process %s PID changed from %d to %d (crash detected)", name, baseline.Pid, info.Pid)
      }
      if info.StartTime != baseline.StartTime {
        t.Errorf("Process %s StartTime changed from %d to %d (crash/restart detected)", name, baseline.StartTime, info.StartTime)
      }

      // Memory Leak Detection
      t.Logf("Process %s Memory: Baseline = %d, Current = %d", name, baseline.MemoryUsage, info.MemoryUsage)
      if baseline.MemoryUsage > 0 && info.MemoryUsage > baseline.MemoryUsage {
        pctIncrease := float64(info.MemoryUsage-baseline.MemoryUsage) / float64(baseline.MemoryUsage)
        if pctIncrease > 0.10 {
          t.Errorf("Process %s memory increased significantly compared to baseline: got %d, baseline %d (%.2f%% increase)", name, info.MemoryUsage, baseline.MemoryUsage, pctIncrease*100)
        }
      }
    }

    // Verify QoS queue drop telemetry does not increase
    verifyNoQoSDrops(t, dut, qosPorts, qosQueues, qosBaselines)
  }
}

func TestSSOSoftwareStability(t *testing.T) {
  dut := ondatra.DUT(t, "dut")

  // Init BGPSession
  bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount4, nil)
  bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST}, []string{"port1", "port2", "port3", "port4"}, true, false)

  p1 := bs.OndatraDUTPorts[0]
  p2 := bs.OndatraDUTPorts[1]
  :wq! p3 := bs.OndatraDUTPorts[2]
  p4 := bs.OndatraDUTPorts[3]

  // 1. Delete BGP protocol under default network-instance
  defaultNiName := deviations.DefaultNetworkInstance(dut)
  if _, ok := bs.DUTConf.NetworkInstance[defaultNiName]; ok {
    delete(bs.DUTConf.NetworkInstance[defaultNiName].Protocol, oc.NetworkInstance_Protocol_Key{
      Identifier: ptBGP,
      Name:       bgpName,
    })
  }

  // 2. Configure L3VRFs and interfaces on DUTConf
  transitNi := bs.DUTConf.GetOrCreateNetworkInstance("TRANSIT_VRF")
  transitNi.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
  transitNi.GetOrCreateInterface("port1").Interface = ygot.String(p1.Name())
  transitNi.GetOrCreateInterface("port1").Subinterface = ygot.Uint32(0)
  if deviations.InterfaceRefInterfaceIDFormat(dut) {
    transitNi.GetOrCreateInterface("port1").Id = ygot.String(fmt.Sprintf("%s.0", p1.Name()))
  } else {
    transitNi.GetOrCreateInterface("port1").Id = ygot.String(p1.Name())
  }

  transitNi.GetOrCreateInterface("port2").Interface = ygot.String(p2.Name())
  transitNi.GetOrCreateInterface("port2").Subinterface = ygot.Uint32(0)
  if deviations.InterfaceRefInterfaceIDFormat(dut) {
    transitNi.GetOrCreateInterface("port2").Id = ygot.String(fmt.Sprintf("%s.0", p2.Name()))
  } else {
    transitNi.GetOrCreateInterface("port2").Id = ygot.String(p2.Name())
  }

  decapNi := bs.DUTConf.GetOrCreateNetworkInstance("DECAP_TE_VRF")
  decapNi.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
  decapNi.GetOrCreateInterface("port3").Interface = ygot.String(p3.Name())
  decapNi.GetOrCreateInterface("port3").Subinterface = ygot.Uint32(0)
  if deviations.InterfaceRefInterfaceIDFormat(dut) {
    decapNi.GetOrCreateInterface("port3").Id = ygot.String(fmt.Sprintf("%s.0", p3.Name()))
  } else {
    decapNi.GetOrCreateInterface("port3").Id = ygot.String(p3.Name())
  }

  decapNi.GetOrCreateInterface("port4").Interface = ygot.String(p4.Name())
  decapNi.GetOrCreateInterface("port4").Subinterface = ygot.Uint32(0)
  if deviations.InterfaceRefInterfaceIDFormat(dut) {
    decapNi.GetOrCreateInterface("port4").Id = ygot.String(fmt.Sprintf("%s.0", p4.Name()))
  } else {
    decapNi.GetOrCreateInterface("port4").Id = ygot.String(p4.Name())
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

  // Create PERMIT-ALL routing policy
  rp := bs.DUTConf.GetOrCreateRoutingPolicy()
  pdef := rp.GetOrCreatePolicyDefinition(rplPermitAll)
  stmt, err := pdef.AppendNewStatement("20")
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
    } else {
      nbr = decapBgp.GetOrCreateNeighbor(peerAddress)
      nbr.PeerAs = ygot.Uint32(65002)
    }
    nbr.Enabled = ygot.Bool(true)
    nbr.SendCommunityType = []oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_NONE}

    nbrPolicy := nbr.GetOrCreateApplyPolicy()
    nbrPolicy.SetExportPolicy([]string{rplPermitAll})
    nbrPolicy.SetImportPolicy([]string{rplPermitAll})

    nAfiSafi := nbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
    nAfiSafi.Enabled = ygot.Bool(true)
  }

  // 4. Configure QoS egress queue management profiles and map to all output ports
  qos := bs.DUTConf.GetOrCreateQos()
  af4Profile := qos.GetOrCreateQueueManagementProfile("AF4_PROFILE")
  af4Profile.SetName("AF4_PROFILE")
  af4Wred := af4Profile.GetOrCreateWred()
  af4Wred.GetOrCreateUniform().SetEnableEcn(true)

  be0Profile := qos.GetOrCreateQueueManagementProfile("BE0_PROFILE")
  be0Profile.SetName("BE0_PROFILE")
  be0Wred := be0Profile.GetOrCreateWred()
  be0Wred.GetOrCreateUniform().SetEnableEcn(true)

  for _, port := range []string{p1.Name(), p2.Name(), p3.Name(), p4.Name()} {
    intf := qos.GetOrCreateInterface(port)
    intf.SetInterfaceId(port)
    intf.GetOrCreateInterfaceRef().Interface = ygot.String(port)
    if deviations.InterfaceRefConfigUnsupported(dut) {
      intf.InterfaceRef = nil
    }
    output := intf.GetOrCreateOutput()

    qAF4 := output.GetOrCreateQueue("AF4")
    qAF4.SetName("AF4")
    qAF4.SetQueueManagementProfile("AF4_PROFILE")

    qBE0 := output.GetOrCreateQueue("BE0")
    qBE0.SetName("BE0")
    qBE0.SetQueueManagementProfile("BE0_PROFILE")
  }

  if deviations.QOSQueueRequiresID(dut) {
    qAF4 := qos.GetOrCreateQueue("AF4")
    qAF4.Name = ygot.String("AF4")
    qAF4.QueueId = ygot.Uint8(1)

    qBE0 := qos.GetOrCreateQueue("BE0")
    qBE0.Name = ygot.String("BE0")
    qBE0.QueueId = ygot.Uint8(2)
  }

  // 5. Configure OTG BGP Route Advertisements
  dev1 := getDeviceByName(t, bs.ATETop, "port1")
  dev2 := getDeviceByName(t, bs.ATETop, "port2")
  dev3 := getDeviceByName(t, bs.ATETop, "port3")
  dev4 := getDeviceByName(t, bs.ATETop, "port4")

  configureBGPv4Routes(dev1.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0], bs.ATEPorts[0].IPv4, "port1_routes", "198.51.100.0", 24)
  configureBGPv4Routes(dev2.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0], bs.ATEPorts[1].IPv4, "port2_routes", "198.51.101.0", 24)
  configureBGPv4Routes(dev3.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0], bs.ATEPorts[2].IPv4, "port3_routes", "198.51.102.0", 24)
  configureBGPv4Routes(dev4.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0], bs.ATEPorts[3].IPv4, "port4_routes", "198.51.103.0", 24)

  // 6. Configure OTG Traffic Flows (AF4 and BE0)
  bs.ATETop.Flows().Clear()
  flowAF4 := bs.ATETop.Flows().Add().SetName("AF4_Flow")
  flowAF4.Metrics().SetEnable(true)
  flowAF4.TxRx().Port().
    SetTxName(bs.ATEPorts[0].Name).
    SetRxName(bs.ATEPorts[2].Name)
  ethAF4 := flowAF4.Packet().Add().Ethernet()
  ethAF4.Src().SetValue(bs.ATEPorts[0].MAC)
  ipAF4 := flowAF4.Packet().Add().Ipv4()
  ipAF4.Src().SetValue(bs.ATEPorts[0].IPv4)
  ipAF4.Dst().SetValue("198.51.102.1")
  ipAF4.Priority().Dscp().Phb().SetValue(32)

  flowBE0 := bs.ATETop.Flows().Add().SetName("BE0_Flow")
  flowBE0.Metrics().SetEnable(true)
  flowBE0.TxRx().Port().
    SetTxName(bs.ATEPorts[1].Name).
    SetRxName(bs.ATEPorts[3].Name)
  ethBE0 := flowBE0.Packet().Add().Ethernet()
  ethBE0.Src().SetValue(bs.ATEPorts[1].MAC)
  ipBE0 := flowBE0.Packet().Add().Ipv4()
  ipBE0.Src().SetValue(bs.ATEPorts[1].IPv4)
  ipBE0.Dst().SetValue("198.51.103.1")
  ipBE0.Priority().Dscp().Phb().SetValue(0)

  // Post configuration to DUT & Start protocols
  bs.PushAndStart(t)

  t.Log("Verify DUT BGP sessions established in VRFs")
  for _, vrf := range []string{"TRANSIT_VRF", "DECAP_TE_VRF"} {
    statePath := gnmi.OC().NetworkInstance(vrf).Protocol(ptBGP, bgpName).Bgp().NeighborAny().SessionState().State()
    gnmi.WatchAll(t, dut, statePath, 5*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
      state, present := val.Val()
      return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
    }).Await(t)
  }

  t.Log("Verify OTG BGP sessions established")
  cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

  t.Log("Wait for ARP resolution on OTG")
  otgutils.WaitForARP(t, bs.ATE.OTG(), bs.ATETop, "IPv4")

  t.Log("Initiating continuously background traffic")
  bs.ATE.OTG().StartTraffic(t)
  time.Sleep(30 * time.Second) // Let traffic flow

  // Verify traffic flows with 0 loss initially
  for _, flow := range []string{"AF4_Flow", "BE0_Flow"} {
    loss := otgutils.GetFlowLossPct(t, bs.ATE.OTG(), flow, 10*time.Second)
    if loss > 0.0 {
      t.Fatalf("Initial traffic validation failed: flow %s has loss %f%%, want 0%%", flow, loss)
    }
  }

  // 7. Find critical hardware and routing processes to monitor
  criticalProcs := findRunningCriticalProcesses(t, dut)
  t.Logf("Monitoring critical processes: %v", criticalProcs)
  procBaselines, err := system.GetProcessInfo(t, dut, criticalProcs)
  if err != nil {
    t.Fatalf("Failed to query baseline process info: %v", err)
  }
  for name, pInfo := range procBaselines {
    t.Logf("Process %s baseline: PID=%d, StartTime=%d, Memory=%d", name, pInfo.Pid, pInfo.StartTime, pInfo.MemoryUsage)
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

  // ==== Supervisor Switchover 1 ====
  t.Log("Executing first supervisor switchover...")
  performSwitchover(t, dut, controllerCards)
  runPostSSOVerification(t, dut, criticalProcs, procBaselines, qosBaselines, qosPorts, qosQueues)

  // ==== Supervisor Switchover 2 ====
  t.Log("Executing second supervisor switchover...")
  performSwitchover(t, dut, controllerCards)
  runPostSSOVerification(t, dut, criticalProcs, procBaselines, qosBaselines, qosPorts, qosQueues)

  // Final validations
  t.Log("Stopping traffic...")
  bs.ATE.OTG().StopTraffic(t)

  // Log traffic stats and verify final traffic loss is 0%
  otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
  otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
  for _, flow := range []string{"AF4_Flow", "BE0_Flow"} {
    loss := otgutils.GetFlowLossPct(t, bs.ATE.OTG(), flow, 10*time.Second)
    if loss > 0.0 {
      t.Errorf("Final forwarding validation failed: flow %s has loss %f%%, want 0%%", flow, loss)
    }
  }

  // Verify no QoS queue drops increase
  verifyNoQoSDrops(t, dut, qosPorts, qosQueues, qosBaselines)
}
