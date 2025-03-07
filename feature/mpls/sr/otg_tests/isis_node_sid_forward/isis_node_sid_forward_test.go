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

package isis_node_sid_forward_test

import (
  "fmt"
  "testing"
  "time"

  "github.com/openconfig/featureprofiles/internal/attrs"
  "github.com/openconfig/featureprofiles/internal/deviations"
  "github.com/openconfig/featureprofiles/internal/fptest"
  "github.com/openconfig/ondatra"
  "github.com/openconfig/ondatra/gnmi"
  "github.com/openconfig/ondatra/gnmi/oc"
  "github.com/openconfig/ygot/ygot"
  "github.com/openconfig/ondatra/otg"
  "github.com/openconfig/ondatra/otg/otg"

)

const (
  // Prefix and address-related constants.
  ipv4PrefixLen = 30
  ipv6PrefixLen = 126
  plen32        = 32
  plen48        = 48
  plen64        = 64
  plen128       = 128

  // ISIS-related constants.
  isisInstance      = "DEFAULT_INSTANCE"
  isisMetric        = 10
  isisInterfaceName = "isis" // Consistent interface name
  sysID1            = "1920.0000.0001"
  sysID2            = "1920.0000.0002"
  sysID3            = "1920.0000.0003"
  sysID4            = "1920.0000.0004"
  lspID1            = "192000000001-00"
  lspID2            = "192000000002-00"
  lspID3            = "192000000003-00"
  lspID4            = "192000000004-00"

  // Interface and address configuration constants.
  dutAS         = 65501
  otgAS         = 65500
  peerGrpName   = "BGP-PEER-GROUP"
  localAddress  = "local_addr"
  remoteAddress = "remote_addr"
  labelBegin    = 100
  labelEnd      = 5000
  //labelRangeSize = 4901  // Not directly used, can be derived.  Good to keep for documentation.
  otgPortSpeed = ondatra.Speed100Gbps

  // Topology constants
  otg1 = "port1"
  otg2 = "port2"
  otg3 = "port3"
  otg4 = "port4"
  dut1 = "port1"
  dut2 = "port2"
  dut3 = "port3"
  dut4 = "port4"
)

var (
  // Address configurations for interfaces.
  dut1Port = attrs.Attributes{
    Name:    "dutPort1",
    Desc:    "DUT Port 1",
    IPv4:    "192.0.2.1",
    IPv6:    "2001:db8::192:0:2:1",
    IPv4Len: ipv4PrefixLen,
    IPv6Len: ipv6PrefixLen,
  }
  dut2Port = attrs.Attributes{
    Name:    "dutPort2",
    Desc:    "DUT Port 2",
    IPv4:    "192.0.2.5",
    IPv6:    "2001:db8::192:0:2:5",
    IPv4Len: ipv4PrefixLen,
    IPv6Len: ipv6PrefixLen,
  }
  dut3Port = attrs.Attributes{
    Name:    "dutPort3",
    Desc:    "DUT Port 3",
    IPv4:    "192.0.2.9",
    IPv6:    "2001:db8::192:0:2:9",
    IPv4Len: ipv4PrefixLen,
    IPv6Len: ipv6PrefixLen,
  }
  dut4Port = attrs.Attributes{
    Name:    "dutPort4",
    Desc:    "DUT Port 4",
    IPv4:    "192.0.2.13",
    IPv6:    "2001:db8::192:0:2:13",
    IPv4Len: ipv4PrefixLen,
    IPv6Len: ipv6PrefixLen,
  }

  otg1Port = attrs.Attributes{
    Name:    "otgPort1",
    IPv4:    "192.0.2.2",
    IPv6:    "2001:db8::192:0:2:2",
    MAC:     "02:00:01:01:01:01",
    IPv4Len: ipv4PrefixLen,
    IPv6Len: ipv6PrefixLen,
  }
  otg2Port = attrs.Attributes{
    Name:    "otgPort2",
    IPv4:    "192.0.2.6",
    IPv6:    "2001:db8::192:0:2:6",
    MAC:     "02:00:02:01:01:01",
    IPv4Len: ipv4PrefixLen,
    IPv6Len: ipv6PrefixLen,
  }
  otg3Port = attrs.Attributes{
    Name:    "otgPort3",
    IPv4:    "192.0.2.10",
    IPv6:    "2001:db8::192:0:2:a",
    MAC:     "02:00:03:01:01:01",
    IPv4Len: ipv4PrefixLen,
    IPv6Len: ipv6PrefixLen,
  }
  otg4Port = attrs.Attributes{
    Name:    "otgPort4",
    IPv4:    "192.0.2.14",
    IPv6:    "2001:db8::192:0:2:e",
    MAC:     "02:00:04:01:01:01",
    IPv4Len: ipv4PrefixLen,
    IPv6Len: ipv6PrefixLen,
  }
)

// Function to configure ISIS on the DUT.
func configureISIS(t *testing.T, dut *ondatra.DUTDevice) {
  d := &oc.Root{}
  netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
  prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
  prot.Enabled = ygot.Bool(true)
  isis := prot.GetOrCreateIsis()
  globalISIS := isis.GetOrCreateGlobal()

  // Set Global ISIS configurations.
  globalISIS.Net = []string{fmt.Sprintf("49.0000.0%s.00", sysID1)}
  globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
  globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
  globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
  globalISIS.AuthenticationCheck = ygot.Bool(true)
  lspBit := globalISIS.GetOrCreateLspBit()
  lspBit.OverloadBit = &oc.Isis_Global_LspBit_OverloadBit{
    AdvertiseHighMetric: ygot.Bool(true),
  }

  // Configure Label Block
  mpls := netInstance.GetOrCreateMpls()
  globalMpls := mpls.GetOrCreateGlobal()
  globalMpls.GlobalId = ygot.String(sysID1)
  staticLsp := globalMpls.GetOrCreateStaticLsps().GetOrCreateStaticLsp("static_lsp")
  staticLsp.IngressLsp = []*oc.Mpls_Global_StaticLsps_StaticLsp_IngressLsp{{
    IncomingLabel:  ygot.Uint16(4000),
    LocalAddress: ygot.String("192.0.0.1"),
    NetworkAddress: ygot.String("192.0.0.2"),
    NextHop:       ygot.String("0.0.0.0"),
    PushNlri:      ygot.Bool(false),
  }}
  labelBlock := mpls.GetOrCreateLabelBlock()
  newBlock := labelBlock.GetOrCreateBlock(localAddress, remoteAddress)
  newBlock.LocalAddress = ygot.String(localAddress)
  newBlock.RemoteAddress = ygot.String(remoteAddress)
  newBlock.Begin = ygot.Uint32(labelBegin)
  newBlock.End = ygot.Uint32(labelEnd)

  // Configure interface-level ISIS settings.  Use consistent naming.
  configureISISInterface(t, dut, isis, dut1Port.Name, dut1Port, isisInterfaceName, isisMetric)
  configureISISInterface(t, dut, isis, dut2Port.Name, dut2Port, isisInterfaceName, isisMetric)
  configureISISInterface(t, dut, isis, dut3Port.Name, dut3Port, isisInterfaceName, isisMetric)
  configureISISInterface(t, dut, isis, dut4Port.Name, dut4Port, isisInterfaceName, isisMetric)

  gnmi.Replace(t, dut, prot.Config(), prot)
  gnmi.Replace(t, dut, mpls.Config(), mpls)
}

// Helper function to configure an individual ISIS interface.
func configureISISInterface(t *testing.T, dut *ondatra.DUTDevice, isis *oc.Isis, interfaceID string, interfaceAttrs attrs.Attributes, circuitID string, metric uint32) {
  isisInterface := isis.GetOrCreateInterface(interfaceID)
  isisInterface.Enabled = ygot.Bool(true)
  isisInterface.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
  isisInterface.InterfaceId = ygot.String(circuitID)
  isisInterface.GetOrCreateAuthentication().Enabled = ygot.Bool(false)
  level := isisInterface.GetOrCreateLevel(2)
  level.Enabled = ygot.Bool(true)
  level.Metric = ygot.Uint32(metric)
  level.GetOrCreateAuthentication().Enabled = ygot.Bool(false)
  isisInterface.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
  isisInterface.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
  if deviations.InterfaceEnabledWithISIS(dut) {
    intf := &oc.Interface{Name: ygot.String(interfaceID)}
    intf.Enabled = ygot.Bool(true)
    gnmi.Replace(t, dut, gnmi.OC().Interface(interfaceID).Config(), intf)  // Use gnmi.OC() for brevity
  }
}

// Function to create ISIS interface configs for OTG.
func configureOTGISIS(t *testing.T, otg *ondatra.OTG) map[string]*otg.Interface {
  // Create OTG interface configs.
  otgInterfaceMap := map[string]*otg.Interface{
    otg1: otg.AddInterface(otg1Port.Name).WithPortSpeed(otgPortSpeed),
    otg2: otg.AddInterface(otg2Port.Name).WithPortSpeed(otgPortSpeed),
    otg3: otg.AddInterface(otg3Port.Name).WithPortSpeed(otgPortSpeed),
    otg4: otg.AddInterface(otg4Port.Name).WithPortSpeed(otgPortSpeed),
  }
  // Set OTG Interface Addresses
  otgInterfaceMap[otg1].IPv4().
    WithAddress(otg1Port.IPv4CIDR()).
    WithDefaultGateway(dut1Port.IPv4).
    WithMTU(1500)
  otgInterfaceMap[otg1].IPv6().
    WithAddress(otg1Port.IPv6CIDR()).
    WithDefaultGateway(dut1Port.IPv6).
    WithMTU(1500)

  otgInterfaceMap[otg2].IPv4().
    WithAddress(otg2Port.IPv4CIDR()).
    WithDefaultGateway(dut2Port.IPv4).
    WithMTU(1500)
  otgInterfaceMap[otg2].IPv6().
    WithAddress(otg2Port.IPv6CIDR()).
    WithDefaultGateway(dut2Port.IPv6).
    WithMTU(1500)

  otgInterfaceMap[otg3].IPv4().
    WithAddress(otg3Port.IPv4CIDR()).
    WithDefaultGateway(dut3Port.IPv4).
    WithMTU(1500)
  otgInterfaceMap[otg3].IPv6().
    WithAddress(otg3Port.IPv6CIDR()).
    WithDefaultGateway(dut3Port.IPv6).
    WithMTU(1500)

  otgInterfaceMap[otg4].IPv4().
    WithAddress(otg4Port.IPv4CIDR()).
    WithDefaultGateway(dut4Port.IPv4).
    WithMTU(1500)
  otgInterfaceMap[otg4].IPv6().
    WithAddress(otg4Port.IPv6CIDR()).
    WithDefaultGateway(dut4Port.IPv6).
    WithMTU(1500)

  // Configure ISIS on OTG interfaces.
  configureOTGInterfaceISIS(otgInterfaceMap[otg1], sysID1, []string{fmt.Sprintf("49.0000.0%s.00", sysID1)})
  configureOTGInterfaceISIS(otgInterfaceMap[otg2], sysID2, []string{fmt.Sprintf("49.0000.0%s.00", sysID2)})
  configureOTGInterfaceISIS(otgInterfaceMap[otg3], sysID3, []string{fmt.Sprintf("49.0000.0%s.00", sysID3)})
  configureOTGInterfaceISIS(otgInterfaceMap[otg4], sysID4, []string{fmt.Sprintf("49.0000.0%s.00", sysID4)})

  return otgInterfaceMap
}

// Helper function to configure ISIS on an OTG interface.
func configureOTGInterfaceISIS(otgInterface *otg.Interface, sysID string, netAddress []string) {
  isis := otgInterface.ISIS()
  isis.WithLevelType(otgisis.LevelTypeLEVEL_2)
  isis.WithNetworkType(otgisis.NetworkTypePOINT_TO_POINT)
  isis.WithSystemID(sysID)
  isis.WithNET(netAddress)
  isis.WithAreaID(sysID) // Consider if Area ID should be separate from System ID

  isisv4 := isis.IPv4()
  isisv4.WithEnabled(true)
  isisv4.WithMetric(isisMetric) // Use the constant

  isisv6 := isis.IPv6()
  isisv6.WithEnabled(true)
  isisv6.WithMetric(isisMetric) // Use the constant
}

// Function to configure interfaces on DUT and OTG.
func configureInterface(t *testing.T, dut *ondatra.DUTDevice, otg *ondatra.OTG) map[string]*otg.Interface {
  // Configure DUT interfaces.
  dutPorts := []string{dut1, dut2, dut3, dut4}
  dutPortAttrs := []attrs.Attributes{dut1Port, dut2Port, dut3Port, dut4Port}
  // for i, dutPort := range dutPorts {
  //  dut


func TestMplsLabelBlock(t *testing.T) {
  // Dial OTG and DUT
  dut := ondatra.DUT(t, "dut")
  otg := ondatra.OTG(t, "otg")

  // Configure interfaces on both devices.
  otgInterfaceMap := configureInterface(t, dut, otg)

  // Configure ISIS on the DUT.
  configureISIS(t, dut)
    otgConfig := otg.NewConfig(t)
    for _, otgIntf := range otgInterfaceMap {
        otgConfig.Interfaces().Add().SetName(otgIntf.Name())
    }
  otg.PushConfig(t, otgConfig)
    otg.StartProtocols(t)


  // Verify ISIS adjacency state.
  t.Run("ISISAdjacency", func(t *testing.T) {
    state := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis().Interface(dut1Port.Name).Level(2).Adjacency("192000000001").AdjacencyState().State())
    if want := oc.Isis_IsisInterfaceAdjState_UP; state != want {
      t.Errorf("Adjacency state got %v, want %v", state, want)
    }
  })

  //Verify OTG Adjacency
  t.Run("OTGISISAdjacency", func(t *testing.T) {
        otg.Telemetry().Isis().Interface(otg1Port.Name).Level(2).Adjacency(sysID1).AdjacencyState().Poll(t, func(val *otgtelemetry.QualifiedE_Isis_IsisInterfaceAdjState) bool {
      return val.IsPresent() && val.Val(t) == otgtelemetry.Isis_IsisInterfaceAdjState_UP
    }, 1*time.Minute, 5*time.Second)

    })

  // Verify Label Block
  t.Run("LabelBlockCheck", func(t *testing.T) {
    labelBlock := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().LabelBlock().Block(localAddress, remoteAddress).State())
    if labelBlock.GetLocalAddress() != localAddress {
      t.Errorf("Label Block Local Address: got %v, want %v", labelBlock.GetLocalAddress(), localAddress)
    }
    if labelBlock.GetRemoteAddress() != remoteAddress {
      t.Errorf("Label Block Remote Address: got %v, want %v", labelBlock.GetRemoteAddress(), remoteAddress)
    }
    if labelBlock.GetBegin() != labelBegin {
      t.Errorf("Label Block Begin: got %v, want %v", labelBlock.GetBegin(), labelBegin)
    }
    if labelBlock.GetEnd() != labelEnd {
      t.Errorf("Label Block End: got %v, want %v", labelBlock.GetEnd(), labelEnd)
    }
  })

  //  - Add traffic tests to verify label forwarding. This is crucial.
  //  - Send traffic with labels within the allocated block.
  //  - Use OTG traffic flows and counters for verification.  Example below:

  /*  traffic test
  t.Run("TrafficTest", func(t *testing.T) {
    // Create OTG traffic flow...
        flow := otg.NewFlow("mplsFlow").WithSrcEndpoints(otgInterfaceMap[otg1].Ethernet()).WithDstEndpoints(otgInterfaceMap[otg4].Ethernet())
    mpls := flow.Packet().Add().MPLS()
    mpls.WithLabel(350) //example label
    otg.PushConfig(t, otg.FetchConfig(t))
        otg.StartTraffic(t)
        time.Sleep(10 * time.Second)
        otg.StopTraffic(t)

        flowMetrics := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).State())
    if flowMetrics.GetCounters().GetInPkts() != flowMetrics.GetCounters().GetOutPkts() {
      t.Errorf("Traffic Loss: Sent %d, Received: %d", flowMetrics.GetCounters().GetOutPkts(), flowMetrics.GetCounters().GetInPkts())
    }
  })
  */
}
