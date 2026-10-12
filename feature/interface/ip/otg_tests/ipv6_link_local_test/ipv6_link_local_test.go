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

package ipv6_link_local_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	srcDUTGlobalIPv6          = "2001:db8::1"
	srcOTGGlobalIPv6          = "2001:db8::2"
	dstDUTGlobalIPv6          = "2001:db8::5"
	dstOTGGlobalIPv6          = "2001:db8::6"
	globalIPv6Len             = 126
	linkLocalPrefixLen        = 64
	minTrafficPkts            = 100
	linkLocalOTGToOTGFlowName = "SrcToDstLinkLocal"
	linkLocalOTGToDUTFlowName = "SrcToDUTLinkLocal"
	globalUnicastFlowName     = "SrcToDstGlobalUnicast"
)

// We use the same link local IPv6 fe80::1 and fe80::2 on both src and dst pairs to ensure
// that devices will allow link local addresses to be reused across different L2 domains.
var (
	dutSrc = attrs.Attributes{
		Desc:    "dutsrc",
		IPv6:    "fe80::1",
		IPv6Len: linkLocalPrefixLen,
	}

	ateSrc = attrs.Attributes{
		Name:    "atesrc",
		MAC:     "02:11:01:00:00:01",
		IPv6:    "fe80::2",
		IPv6Len: linkLocalPrefixLen,
	}

	dutDst = attrs.Attributes{
		Desc:    "dutdst",
		IPv6:    "fe80::1",
		IPv6Len: linkLocalPrefixLen,
	}

	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:12:01:00:00:01",
		IPv6:    "fe80::2",
		IPv6Len: linkLocalPrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIPv6LinkLocal(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	// RT-5.8 Subtest #1 - Configure IPv6 link local:
	// - Configure DUT port 1 and OTG port 1 with an IPv6 link local scope IP address.
	// - Configure DUT port 2 and OTG port 2 with an IPv6 link local scope IP address.
	// - Validate config and state paths are set.
	t.Run("RT-5.8.1: Subtest #1 - Configure IPv6 link local", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
		configureDUTLinkLocalInterface(t, dut)

		configureOTGInterface(t, ate, top, false)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		waitForIPv6Neighbors(t, dut, ate, top)

		t.Run("Validate Link Local Config and State Paths", func(t *testing.T) {
			verifyInterfaceTelemetry(t, dut)
		})
	})

	// RT-5.8 Subtest #2 - Verify the interface will pass IPv6 traffic as expected:
	// - Send IPv6 traffic from OTG port 1 to OTG port 2, validate OTG port 2 does not receive the traffic.
	// - Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1 receives the traffic.
	t.Run("RT-5.8.2: Subtest #2 - Verify the interface will pass IPv6 traffic as expected", func(t *testing.T) {
		t.Run("OTG Port 1 to OTG Port 2 Link Local Traffic Does Not Forward", func(t *testing.T) {
			configureOTGFlow(t, top, ateSrc.Name+".IPv6", ateDst.Name+".IPv6", ateSrc.IPv6, ateDst.IPv6, linkLocalOTGToOTGFlowName)
			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)
			waitForIPv6Neighbors(t, dut, ate, top)

			verifyLinkLocalTrafficNotForwarded(t, ate, top, linkLocalOTGToOTGFlowName)
		})

		t.Run("OTG Port 1 to DUT Port 1 Link Local Traffic Received by DUT", func(t *testing.T) {
			configureOTGFlow(t, top, ateSrc.Name+".IPv6", ateDst.Name+".IPv6", ateSrc.IPv6, dutSrc.IPv6, linkLocalOTGToDUTFlowName)
			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)
			waitForIPv6Neighbors(t, dut, ate, top)

			verifyLinkLocalTrafficReceivedByDUT(t, dut, ate, top, linkLocalOTGToDUTFlowName)
		})
	})

	// RT-5.8 Subtest #3 - Verify adding and removing global unicast address does not affect link local address:
	// - Add configuration for a global unicast address on DUT port 1 and DUT port 2.
	// - Validate config and state paths are set.
	// - Send IPv6 traffic from OTG port 1 to OTG port 2, validate OTG port 2 receives the traffic.
	// - Remove configuration for the global unicast address on DUT port 1 (and port 2).
	// - Validate that DUT port 1 link local address is still configured.
	// - Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1 receives the traffic.
	// - Send IPv6 traffic from OTG port 1 to OTG port 2, validate OTG port 2 does not receive the traffic.
	t.Run("RT-5.8.3: Subtest #3 - Verify adding and removing global unicast address does not affect link local address", func(t *testing.T) {
		p1 := dut.Port(t, "port1")
		p2 := dut.Port(t, "port2")

		// Ensure global unicast addresses are cleaned up if the subtest exits early.
		t.Cleanup(func() {
			gnmi.Delete(t, dut, gnmi.OC().Interface(p1.Name()).Subinterface(0).Ipv6().Address(srcDUTGlobalIPv6).Config())
			gnmi.Delete(t, dut, gnmi.OC().Interface(p2.Name()).Subinterface(0).Ipv6().Address(dstDUTGlobalIPv6).Config())
		})

		// Step 3.1 & 3.2: Add global unicast IPv6 configuration on DUT port 1 and port 2 and validate config and state paths.
		configureAndVerifyDUTGlobalIPv6(t, dut)

		// Configure global unicast IPv6 addresses and flow on OTG.
		configureOTGInterface(t, ate, top, true)
		configureOTGFlow(t, top, ateSrc.Name+".IPv6-1", ateDst.Name+".IPv6-1", srcOTGGlobalIPv6, dstOTGGlobalIPv6, globalUnicastFlowName)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		waitForIPv6Neighbors(t, dut, ate, top)

		// Step 3.3: Send IPv6 traffic from OTG port 1 to OTG port 2, validate OTG port 2 receives the traffic.
		t.Run("Global IPv6 Traffic Test", func(t *testing.T) {
			verifyGlobalUnicastTraffic(t, ate, top, globalUnicastFlowName)
		})

		// Step 3.4: Remove configuration for the global unicast address and await removal from state.
		deleteDUTGlobalIPv6(t, dut, p1.Name(), srcDUTGlobalIPv6)
		deleteDUTGlobalIPv6(t, dut, p2.Name(), dstDUTGlobalIPv6)

		// Step 3.5: Validate that DUT port 1 (and port 2) link local address is still configured in config and state.
		t.Run("Validate Link Local Config and State Paths After Global IPv6 Removal", func(t *testing.T) {
			verifyInterfaceTelemetry(t, dut)
		})

		// Restore OTG interfaces to link-local only after removing global unicast IPv6 from DUT.
		configureOTGInterface(t, ate, top, false)

		// Step 3.6: Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1 receives the traffic.
		t.Run("OTG Port 1 to DUT Port 1 Link Local Traffic After Global IPv6 Removal", func(t *testing.T) {
			configureOTGFlow(t, top, ateSrc.Name+".IPv6", ateDst.Name+".IPv6", ateSrc.IPv6, dutSrc.IPv6, linkLocalOTGToDUTFlowName)
			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)
			waitForIPv6Neighbors(t, dut, ate, top)

			verifyLinkLocalTrafficReceivedByDUT(t, dut, ate, top, linkLocalOTGToDUTFlowName)
		})

		// Step 3.7: Send IPv6 traffic from OTG port 1 to OTG port 2, validate OTG port 2 does not receive the traffic.
		t.Run("OTG Port 1 to OTG Port 2 Link Local Traffic Not Forwarded After Global IPv6 Removal", func(t *testing.T) {
			configureOTGFlow(t, top, ateSrc.Name+".IPv6", ateDst.Name+".IPv6", ateSrc.IPv6, ateDst.IPv6, linkLocalOTGToOTGFlowName)
			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)
			waitForIPv6Neighbors(t, dut, ate, top)

			verifyLinkLocalTrafficNotForwarded(t, ate, top, linkLocalOTGToOTGFlowName)
		})
	})

	// RT-5.8 Subtest #4 - Verify enable/disable of DUT port 1 does not affect link local address:
	// - Disable/enable the port and see if the configured link-local address stays.
	// - Validate that DUT port 1 link local address config and state paths continue to contain the address assignment.
	// - Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1 receives the traffic.
	t.Run("RT-5.8.4: Subtest #4 - Verify enable/disable of DUT port 1 does not affect link local address", func(t *testing.T) {
		p1 := dut.Port(t, "port1")

		// Step 4.1: Disable DUT port 1, register cleanup to guarantee restoration, and await DOWN state.
		gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), false)
		t.Cleanup(func() {
			gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), true)
			gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		})
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().State(), 2*time.Minute, false)
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_DOWN)

		// Re-enable DUT port 1 and await UP state.
		gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), true)
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().State(), 2*time.Minute, true)
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)

		// Step 4.2: Validate that DUT port 1 link local address config and state paths continue to contain the address assignment.
		t.Run("Validate Link Local Config and State Paths After Port Toggle", func(t *testing.T) {
			verifyInterfaceTelemetry(t, dut)
		})

		// Step 4.3: Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1 receives the traffic.
		t.Run("OTG Port 1 to DUT Port 1 Link Local Traffic After Port Toggle", func(t *testing.T) {
			configureOTGFlow(t, top, ateSrc.Name+".IPv6", ateDst.Name+".IPv6", ateSrc.IPv6, dutSrc.IPv6, linkLocalOTGToDUTFlowName)
			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)
			waitForIPv6Neighbors(t, dut, ate, top)

			verifyLinkLocalTrafficReceivedByDUT(t, dut, ate, top, linkLocalOTGToDUTFlowName)
		})
	})
}

// configureAndVerifyDUTGlobalIPv6 adds global unicast IPv6 addresses to DUT port 1 and port 2
// using cfgplugins and validates both config and state paths via gnmi.Watch/gnmi.Await.
func configureAndVerifyDUTGlobalIPv6(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	portData := []struct {
		portName string
		ipAddr   string
	}{
		{
			portName: "port1",
			ipAddr:   srcDUTGlobalIPv6,
		},
		{
			portName: "port2",
			ipAddr:   dstDUTGlobalIPv6,
		},
	}

	sb := &gnmi.SetBatch{}
	for _, pd := range portData {
		cfgplugins.ConfigureInterfaceIPv6Address(t, dut, sb, cfgplugins.IPv6AddressParams{
			PortName:         pd.portName,
			SubinterfaceIdx:  0,
			IPv6:             pd.ipAddr,
			PrefixLen:        globalIPv6Len,
			AddrType:         oc.IfIp_Ipv6AddressType_GLOBAL_UNICAST,
			ReplaceInterface: false,
		})
	}
	sb.Set(t, dut)

	for _, pd := range portData {
		p := dut.Port(t, pd.portName)
		cfgplugins.VerifyInterfaceIPv6Address(t, dut, p.Name(), 0, pd.ipAddr, globalIPv6Len, oc.IfIp_Ipv6AddressType_GLOBAL_UNICAST, 30*time.Second)
	}
}

// deleteDUTGlobalIPv6 removes a global unicast IPv6 address from a DUT port.
func deleteDUTGlobalIPv6(t *testing.T, dut *ondatra.DUTDevice, portName, ipAddr string) {
	t.Helper()
	gnmi.Delete(t, dut, gnmi.OC().Interface(portName).Subinterface(0).Ipv6().Address(ipAddr).Config())
}

// configureDUTLinkLocalInterface configures link-local IPv6 addresses on DUT port 1 and port 2
// using cfgplugins and awaits operational UP state.
func configureDUTLinkLocalInterface(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	sb := &gnmi.SetBatch{}
	cfgplugins.ConfigureInterfaceIPv6Address(t, dut, sb, cfgplugins.IPv6AddressParams{
		PortName:         "port1",
		SubinterfaceIdx:  0,
		Description:      dutSrc.Desc,
		IPv6:             dutSrc.IPv6,
		PrefixLen:        dutSrc.IPv6Len,
		AddrType:         oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST,
		ReplaceInterface: true,
	})
	cfgplugins.ConfigureInterfaceIPv6Address(t, dut, sb, cfgplugins.IPv6AddressParams{
		PortName:         "port2",
		SubinterfaceIdx:  0,
		Description:      dutDst.Desc,
		IPv6:             dutDst.IPv6,
		PrefixLen:        dutDst.IPv6Len,
		AddrType:         oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST,
		ReplaceInterface: true,
	})
	sb.Set(t, dut)

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
}

// waitForIPv6Neighbors waits for OTG IPv6 neighbor discovery to resolve the LinkLayerAddress
// for every configured IPv6 gateway on all OTG Ethernet interfaces.
func waitForIPv6Neighbors(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	for idx, d := range top.Devices().Items() {
		eth := d.Ethernets().Items()[0]
		for _, ip := range eth.Ipv6Addresses().Items() {
			gw := ip.Gateway()
			if gw == "" {
				continue
			}
			nbrPath := gnmi.OTG().Interface(eth.Name()).Ipv6Neighbor(gw).LinkLayerAddress().State()
			if _, ok := gnmi.Watch(t, ate.OTG(), nbrPath, 30*time.Second, func(val *ygnmi.Value[string]) bool {
				v, present := val.Val()
				return present && v != ""
			}).Await(t); !ok {
				portID := "port1"
				if idx == 1 {
					portID = "port2"
				}
				p := dut.Port(t, portID)
				gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Enabled().Config(), false)
				gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_DOWN)
				gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Enabled().Config(), true)
				gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
				ate.OTG().StartProtocols(t)
				if _, ok := gnmi.Watch(t, ate.OTG(), nbrPath, time.Minute, func(val *ygnmi.Value[string]) bool {
					v, present := val.Val()
					return present && v != ""
				}).Await(t); !ok {
					t.Fatalf("Did not resolve OTG IPv6 neighbor %s on interface %s", gw, eth.Name())
				}
			}
		}
	}
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
}

// configureOTGInterface configures OTG port 1 and port 2 with link-local IPv6 addresses,
// and optionally adds global unicast IPv6 addresses when includeGlobalIPv6 is true.
func configureOTGInterface(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, includeGlobalIPv6 bool) {
	t.Helper()
	top.Ports().Clear()
	top.Devices().Clear()

	p1 := ate.Port(t, "port1")
	dev1 := ateSrc.AddToOTG(top, p1, &dutSrc)

	p2 := ate.Port(t, "port2")
	dev2 := ateDst.AddToOTG(top, p2, &dutDst)

	if includeGlobalIPv6 {
		eth1 := dev1.Ethernets().Items()[0]
		ip1 := eth1.Ipv6Addresses().Add().SetName(dev1.Name() + ".IPv6-1")
		ip1.SetAddress(srcOTGGlobalIPv6).SetGateway(srcDUTGlobalIPv6).SetPrefix(uint32(globalIPv6Len))

		eth2 := dev2.Ethernets().Items()[0]
		ip2 := eth2.Ipv6Addresses().Add().SetName(dev2.Name() + ".IPv6-1")
		ip2.SetAddress(dstOTGGlobalIPv6).SetGateway(dstDUTGlobalIPv6).SetPrefix(uint32(globalIPv6Len))
	}
}

// configureOTGFlow configures an IPv6 flow on the OTG topology using otgconfighelpers.Flow.
func configureOTGFlow(t *testing.T, top gosnappi.Config, txName, rxName, srcIPv6, dstIPv6, flowName string) {
	t.Helper()
	top.Flows().Clear()
	flow := &otgconfighelpers.Flow{
		FlowName: flowName,
		TxNames:  []string{txName},
		RxNames:  []string{rxName},
		EthFlow:  &otgconfighelpers.EthFlowParams{SrcMAC: ateSrc.MAC},
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: srcIPv6, IPv6Dst: dstIPv6},
	}
	flow.CreateFlow(top)
	flow.AddEthHeader()
	flow.AddIPv6Header()
}

// verifyLinkLocalTrafficNotForwarded sends IPv6 link-local traffic from OTG port 1 toward OTG port 2
// and verifies that OTG port 2 does not receive the traffic (100% loss).
func verifyLinkLocalTrafficNotForwarded(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, flowName string) {
	t.Helper()
	otgutils.RunTrafficAndWaitForTx(t, ate.OTG(), flowName, minTrafficPkts, 2*time.Minute)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.ExpectedTrafficLoss(t, ate.OTG(), flowName, 99, 100)
}

// verifyLinkLocalTrafficReceivedByDUT sends IPv6 link-local traffic from OTG port 1 to DUT port 1
// and verifies via gnmi.Watch that DUT port 1 receives the transmitted packets.
func verifyLinkLocalTrafficReceivedByDUT(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, flowName string) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	beforeInPkts := gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).Counters().InPkts().State())

	otgutils.RunTrafficAndWaitForTx(t, ate.OTG(), flowName, minTrafficPkts, 2*time.Minute)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.VerifyDUTInterfaceInPkts(t, dut, ate.OTG(), p1.Name(), flowName, beforeInPkts, 1.0, 2*time.Minute)
}

// verifyGlobalUnicastTraffic sends global unicast IPv6 traffic from OTG port 1 to OTG port 2
// without static sleeps and verifies 0% loss at OTG port 2.
func verifyGlobalUnicastTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, flowName string) {
	t.Helper()
	otgutils.RunTrafficAndWaitForTx(t, ate.OTG(), flowName, minTrafficPkts, 2*time.Minute)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.ExpectedTrafficLoss(t, ate.OTG(), flowName, 0, 1)
}

// verifyInterfaceTelemetry validates the link-local IPv6 config and state paths on DUT port 1 and port 2.
func verifyInterfaceTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	for _, entry := range []struct {
		portName string
		attr     attrs.Attributes
	}{
		{portName: p1.Name(), attr: dutSrc},
		{portName: p2.Name(), attr: dutDst},
	} {
		cfgplugins.VerifyInterfaceIPv6Address(
			t,
			dut,
			entry.portName,
			0,
			entry.attr.IPv6,
			entry.attr.IPv6Len,
			oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST,
			30*time.Second,
		)
	}
}
