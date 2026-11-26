// Copyright 2024 Google LLC
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

package ip_guev1_static_decap_subnet_range_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen       = 30
	ipv6PrefixLen       = 126
	packetCount         = 1000
	trafficRatePercent  = 10.0
	timeout             = 30 * time.Second
	ipv4Src             = "198.51.100.1"
	ipv4Dst             = "203.0.113.1"
	ipv6Src             = "2001:db8:1::1"
	ipv6Dst             = "2001:db8:2::1"
	policyName          = "decap-policy-gue"
	policyId            = 1
	outerDscpValue      = uint32(35)
	innerDscpValue      = uint32(32)
	innerTTL            = uint32(50)
	outerTTL            = uint32(70)
	srcPortMin          = 1024
	srcPortMax          = 65535
	decapDstSubnetV6    = "2001:db8:dead:beef::/64"
	decapDst1V6         = "2001:db8:dead:beef::1"
	decapDst2V6         = "2001:db8:dead:beef::2"
	decapDst3V6         = "2001:db8:dead:beef::3"
	decapDst4V6         = "2001:db8:dead:beef::4"
	gueDecapPort        = 6081
	loopbackIntfName    = "Loopback0"
	counterTolerancePct = 5

	// Some platforms (e.g. Cisco IOS-XR) refresh interface counters only every ~30s, so the DUT
	// counters are watched (gNMI subscribe) until they reflect the traffic instead of being read once.
	counterSettleTimeout = 90 * time.Second
	// DUT counters are accepted within counterTolerancePct of the 1000 packets sent (e.g. Cisco reported 995).
	minExpectedPkts = packetCount - packetCount*counterTolerancePct/100
)

var (
	atePort1 = attrs.Attributes{
		Name:    "ateP1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "ateP2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort1 = &attrs.Attributes{
		Desc:    "dutPort1",
		MAC:     "00:00:a1:a1:a1:a1",
		IPv6:    "2001:db8::1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = &attrs.Attributes{
		Desc:    "dutPort2",
		MAC:     "00:00:b1:b1:b1:b1",
		IPv6:    "2001:db8::5",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	decapDstAddrs = []string{decapDst1V6, decapDst2V6, decapDst3V6, decapDst4V6}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	name               string
	ipType             string
	ateGuePort         int
	dutGuePort         int
	trafficDestIps     []string
	trafficShouldPass  bool
	shouldDecap        bool
	verifyDropCounters bool
}

func TestIpGueStaticDecapsulation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	t.Log(dp1, dp2)

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)
	configureDUTLoopback(t, dut, decapDst3V6)
	configureStaticRoutes(t, dut)

	// Cisco IOS-XR models GUE decap through NVE overlay-encap UDP ports. The config is pushed here without
	// rebooting the DUT; a newly added NVE port only becomes active after a reload, which must be done
	// outside the test if the DUT has never had this NVE config.
	if deviations.GueGreDecapUnsupported(dut) && dut.Vendor() == ondatra.CISCO {
		cfgplugins.EnableCiscoGueDecapNVE(t, dut, gueDecapPort)
	}

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")

	// Remove the GUE decap policy/decap-group once all sub-tests are done.
	t.Cleanup(func() {
		cfgplugins.RemoveDecapGroupGue(t, dut, getDefaultOcPolicyForwardingParams(t, dut, gueDecapPort, "ipv6"))
	})

	testCases := []testCase{
		{
			name:              "PF-1.4.1: GUE Decapsulation of inner IPv4 traffic over DECAP subnet range",
			ipType:            "ipv4",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIps:    decapDstAddrs,
			trafficShouldPass: true,
			shouldDecap:       true,
		},
		{
			name:              "PF-1.4.2: GUE Decapsulation of inner IPv6 traffic over DECAP subnet range",
			ipType:            "ipv6",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIps:    decapDstAddrs,
			trafficShouldPass: true,
			shouldDecap:       true,
		},
		{
			name:               "PF-1.4.3: GUE Decapsulation of inner IPv4 traffic using non-default and unconfigured GUE UDP port (Negative).",
			ipType:             "ipv4",
			ateGuePort:         6085,
			dutGuePort:         6081,
			trafficDestIps:     decapDstAddrs,
			trafficShouldPass:  false,
			shouldDecap:        false,
			verifyDropCounters: true,
		},
		{
			name:               "PF-1.4.4: GUE Decapsulation of inner IPv6 traffic using non-default and unconfigured GUE UDP port (Negative).",
			ipType:             "ipv6",
			ateGuePort:         6085,
			dutGuePort:         6081,
			trafficDestIps:     decapDstAddrs,
			trafficShouldPass:  false,
			shouldDecap:        false,
			verifyDropCounters: true,
		},
		{
			name:              "PF-1.4.5: Inner IPV4 GUE Pass-through (Negative)",
			ipType:            "ipv4",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIps:    []string{atePort2.IPv6},
			trafficShouldPass: true,
			shouldDecap:       false,
		},
		{
			name:              "PF-1.4.6: Inner IPV6 GUE Pass-through (Negative)",
			ipType:            "ipv6",
			ateGuePort:        6081,
			dutGuePort:        6081,
			trafficDestIps:    []string{atePort2.IPv6},
			trafficShouldPass: true,
			shouldDecap:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ipType == "ipv4" {
				gueDecapInnerIpv4Traffic(t, dut, ate, topo, ate.OTG(), tc.ateGuePort, tc.dutGuePort, tc.trafficDestIps,
					tc.trafficShouldPass, tc.shouldDecap, tc.verifyDropCounters)
			} else {
				gueDecapInnerIpv6Traffic(t, dut, ate, topo, ate.OTG(), tc.ateGuePort, tc.dutGuePort, tc.trafficDestIps,
					tc.trafficShouldPass, tc.shouldDecap, tc.verifyDropCounters)
			}
		})
	}
}

// ConfigureDUTIntf configures all ports with base IPs and subinterfaces.
func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutPort1, dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
}

// Configures the given DUT interface.
func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()

	return i
}

// configureDUTLoopback configures a software loopback interface on the DUT with the given IPv6 /128 address.
func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice, addr string) {
	t.Helper()
	d := &oc.Root{}
	lo := d.GetOrCreateInterface(loopbackIntfName)
	lo.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	a := lo.GetOrCreateSubinterface(0).GetOrCreateIpv6().GetOrCreateAddress(addr)
	a.PrefixLength = ygot.Uint8(128)
	gnmi.Update(t, dut, gnmi.OC().Interface(loopbackIntfName).Config(), lo)
	t.Cleanup(func() {
		gnmi.Delete(t, dut, gnmi.OC().Interface(loopbackIntfName).Config())
	})
}

// configureStaticRoutes installs the static routes the README requires on the DUT:
// IPV4-SRC1/IPV6-SRC1 towards ATE Port 1 and IPV4-DST1/IPV6-DST1 towards ATE Port 2.
// Static routes are used (rather than eBGP) so that no routing-policy is needed on the DUT;
// IOS-XR otherwise drops all eBGP routes that have no import policy.
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	niName := deviations.DefaultNetworkInstance(dut)
	protoName := deviations.StaticProtocolName(dut)

	routes := []struct{ prefix, nextHop string }{
		{ipv4Src + "/32", atePort1.IPv4},
		{ipv6Src + "/128", atePort1.IPv6},
		{ipv4Dst + "/32", atePort2.IPv4},
		{ipv6Dst + "/128", atePort2.IPv6},
	}

	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(niName)
	sp := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, protoName)
	for _, r := range routes {
		nh := sp.GetOrCreateStatic(r.prefix).GetOrCreateNextHop("0")
		nh.NextHop = oc.UnionString(r.nextHop)
	}
	spPath := gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, protoName)
	gnmi.Update(t, dut, spPath.Config(), sp)

	t.Cleanup(func() {
		for _, r := range routes {
			gnmi.Delete(t, dut, spPath.Static(r.prefix).Config())
		}
	})
}

// ateIPv4Name / ateIPv6Name return the OTG interface names used as flow endpoints.
func ateIPv4Name(a attrs.Attributes) string { return a.Name + ".IPv4" }
func ateIPv6Name(a attrs.Attributes) string { return a.Name + ".IPv6" }

// configureATE sets up the ATE interfaces. Routing is static on the DUT (see configureStaticRoutes);
// the ATE only needs L3 interfaces, and flows are addressed to the DUT gateway (Ethernet dst auto),
// which covers the README's "static route for DECAP-DST-SUBNET-V6/64 towards the DUT" on ATE Port 1.
func configureATE(t *testing.T) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Log("Configure ATE interface")
	port1 := topo.Ports().Add().SetName("port1")
	port2 := topo.Ports().Add().SetName("port2")

	port1Dev := topo.Devices().Add().SetName(atePort1.Name + ".dev")
	port1Eth := port1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	port1Eth.Connection().SetPortName(port1.Name())
	port1Ipv4 := port1Eth.Ipv4Addresses().Add().SetName(ateIPv4Name(atePort1))
	port1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	port1Ipv6 := port1Eth.Ipv6Addresses().Add().SetName(ateIPv6Name(atePort1))
	port1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	port2Dev := topo.Devices().Add().SetName(atePort2.Name + ".dev")
	port2Eth := port2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	port2Eth.Connection().SetPortName(port2.Name())
	port2Ipv4 := port2Eth.Ipv4Addresses().Add().SetName(ateIPv4Name(atePort2))
	port2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	port2Ipv6 := port2Eth.Ipv6Addresses().Add().SetName(ateIPv6Name(atePort2))
	port2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))
	return topo
}

// counterDelta returns final-initial, or 0 if the counter went backwards (reset / stale baseline),
// so an unsigned underflow can never look like a huge packet count.
func counterDelta(final, initial uint64) uint64 {
	if final < initial {
		return 0
	}
	return final - initial
}

// waitForCounterDelta watches the counter via gNMI until it has advanced by at least want over initial,
// or the timeout expires, and returns the latest value. Interface counters on some platforms
// (e.g. Cisco IOS-XR) are refreshed only periodically, so a single read right after the traffic
// stops can still show the pre-traffic value.
func waitForCounterDelta(t *testing.T, dut *ondatra.DUTDevice, name string, q ygnmi.SingletonQuery[uint64], initial, want uint64, wait time.Duration) uint64 {
	t.Helper()
	val, ok := gnmi.Watch(t, dut, q, wait, func(v *ygnmi.Value[uint64]) bool {
		cur, present := v.Val()
		return present && cur >= initial && cur-initial >= want
	}).Await(t)
	if ok {
		if cur, present := val.Val(); present {
			return cur
		}
	}
	cur := gnmi.Get(t, dut, q)
	t.Logf("%s advanced by %d (want >= %d) within %v", name, int64(cur)-int64(initial), want, wait)
	return cur
}

// trafficStartStop starts and stops traffic, collects interface and policy counters, verifies forwarding behavior, and optionally validates drop counters.
func trafficStartStop(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config, otgConfig *otg.OTG, flow gosnappi.Flow, trafficShouldPass, shouldDecap, verifyDropCounters bool) {
	inPath := gnmi.OC().Interface(dut.Port(t, "port1").Name()).Counters().InUnicastPkts().State()
	outPath := gnmi.OC().Interface(dut.Port(t, "port2").Name()).Counters().OutUnicastPkts().State()

	initialInUnicastPkts := gnmi.Get(t, dut, inPath)
	initialOutUnicastPkts := gnmi.Get(t, dut, outPath)

	matchedPktsSupported := !deviations.PolicyRuleCountersOCUnsupported(dut)
	initialMatchedPkts := uint64(0)
	if matchedPktsSupported {
		initialMatchedPkts = gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(policyName).Rule(policyId).MatchedPkts().State())
	}
	ate.OTG().StartTraffic(t)
	// Wait until the flow has actually sent all packets. (Waiting for Transmit==false can succeed
	// before the flow has started, which stopped the traffic with 0 packets sent.)
	gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			sent, ok := val.Val()
			return ok && sent >= packetCount
		}).Await(t)
	ate.OTG().StopTraffic(t)

	// All packetCount packets reach DUT port1; they reach port2 only when the flow is forwarded
	// (decapsulated or passed through). Wait for the counters to reflect that before sampling.
	finalInUnicastPkts := waitForCounterDelta(t, dut, "port1 in-unicast-pkts", inPath, initialInUnicastPkts, minExpectedPkts, counterSettleTimeout)
	finalOutUnicastPkts := gnmi.Get(t, dut, outPath)
	if trafficShouldPass {
		finalOutUnicastPkts = waitForCounterDelta(t, dut, "port2 out-unicast-pkts", outPath, initialOutUnicastPkts, minExpectedPkts, counterSettleTimeout)
	}
	otgutils.LogFlowMetrics(t, ate.OTG(), config)
	// TODO: Replace this with drop counters telemetry if it's available on the DUT. For now, we use the interface counters to infer drops.
	verifyPolicerMatchedPackets(t, dut, &policerVerificationParams{
		Flow:                  flow,
		OTGConfig:             otgConfig,
		InitialInUnicastPkts:  initialInUnicastPkts,
		InitialOutUnicastPkts: initialOutUnicastPkts,
		FinalInUnicastPkts:    finalInUnicastPkts,
		FinalOutUnicastPkts:   finalOutUnicastPkts,
		InitialMatchedPkts:    initialMatchedPkts,
		MatchedPktsSupported:  matchedPktsSupported,
		WantIncrement:         shouldDecap,
	})
	if verifyDropCounters {
		verifyDropCountersIncrement(t, initialInUnicastPkts, finalInUnicastPkts, initialOutUnicastPkts, finalOutUnicastPkts)
	}
}

// verifyDropCountersIncrement verifies that exactly packetCount packets arrived at the DUT's
// ingress but none reached egress, confirming the DUT dropped the full flow.
func verifyDropCountersIncrement(t *testing.T, initialInUnicastPkts, finalInUnicastPkts, initialOutUnicastPkts, finalOutUnicastPkts uint64) {
	t.Helper()
	inDelta := counterDelta(finalInUnicastPkts, initialInUnicastPkts)
	outDelta := counterDelta(finalOutUnicastPkts, initialOutUnicastPkts)
	tolerance := uint64(packetCount * counterTolerancePct / 100)
	minAllowedIngress := uint64(packetCount) - tolerance
	maxAllowedIngress := uint64(packetCount) + tolerance
	maxAllowedEgress := tolerance
	if inDelta < minAllowedIngress || inDelta > maxAllowedIngress {
		t.Errorf("expected DUT ingress in-unicast-pkts on port1 to increment by %d +/- %d%% (range [%d, %d]), got %d (initial=%d final=%d)",
			packetCount, counterTolerancePct, minAllowedIngress, maxAllowedIngress, inDelta, initialInUnicastPkts, finalInUnicastPkts)
	}
	if outDelta > maxAllowedEgress {
		t.Errorf("expected DUT egress out-unicast-pkts on port2 to increase by at most %d packets (%d + %d%%), got %d (initial=%d final=%d)",
			maxAllowedEgress, packetCount, counterTolerancePct, outDelta, initialOutUnicastPkts, finalOutUnicastPkts)
	}
	if inDelta >= minAllowedIngress && inDelta <= maxAllowedIngress && outDelta <= maxAllowedEgress {
		t.Logf("PASS: DUT ingress increased by %d packets (within [%d, %d]) and egress increased by %d packets (<= %d allowed), indicating the flow was dropped",
			inDelta, minAllowedIngress, maxAllowedIngress, outDelta, maxAllowedEgress)
	}
}

// verifyTrafficFlow validate the traffic counters.
func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string, trafficShouldPass bool) {
	recvMetricV4 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())

	framesTxV4 := recvMetricV4.GetCounters().GetOutPkts()
	framesRxV4 := recvMetricV4.GetCounters().GetInPkts()

	if trafficShouldPass {
		t.Logf("Traffic validation for flow %s. Expecting %d packets transmitted and received.", flowName, packetCount)
		if framesTxV4 != packetCount {
			t.Errorf("Unexpected transmitted packet count for [%s]. Got: %d, Want: %d", flowName, framesTxV4, packetCount)
		} else if framesRxV4 != packetCount {
			t.Errorf("Unexpected received packet count for [%s]. Got: %d, Want: %d", flowName, framesRxV4, packetCount)
		} else {
			t.Logf("Traffic validation successful for [%s]. FramesTx: %d FramesRx: %d", flowName, framesTxV4, framesRxV4)
		}
	} else {
		t.Logf("traffic validation for flow %s. Expecting Traffic Loss", flowName)
		if framesTxV4 == 0 {
			t.Error("No traffic was generated and frames transmitted were 0")
		} else if framesRxV4 == 0 {
			t.Logf("PASS: Traffic Validation is successful as no packets received at the destination as Expected")
		} else {
			t.Error("FAIL: Traffic Validation is failed as no packets expected at the destination ")
		}
	}
}

// enableCapture enable the port to capture packets.
func enableCapture(t *testing.T, config gosnappi.Config, port string) {
	config.Captures().Clear()
	t.Log("Enabling capture on ", port)
	config.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
}

// processCapture process capture and return a capture file.
func processCapture(t *testing.T, ate *ondatra.ATEDevice, port string) string {
	otg := ate.OTG()
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	pcapFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	defer pcapFile.Close()
	return pcapFile.Name()
}

type policerVerificationParams struct {
	Flow                  gosnappi.Flow
	OTGConfig             *otg.OTG
	InitialInUnicastPkts  uint64
	InitialOutUnicastPkts uint64
	FinalInUnicastPkts    uint64
	FinalOutUnicastPkts   uint64
	InitialMatchedPkts    uint64
	MatchedPktsSupported  bool
	WantIncrement         bool
}

// verifyPolicerMatchedPackets verifies that the policy-rule matched packet counters (or interface-counter proxy on deviating DUTs) reflect the expected decapsulation policy match behavior.
func verifyPolicerMatchedPackets(t *testing.T, dut *ondatra.DUTDevice, p *policerVerificationParams) {
	t.Helper()

	// TO-DO: Currently PolicyForwarding not supported in DUT (Bug 457722520). Adding deviation to check the PF counters.
	if !p.MatchedPktsSupported {
		switch dut.Vendor() {
		case ondatra.ARISTA, ondatra.CISCO:
			ingressPkt := counterDelta(p.FinalInUnicastPkts, p.InitialInUnicastPkts)
			egressPkt := counterDelta(p.FinalOutUnicastPkts, p.InitialOutUnicastPkts)

			if ingressPkt == 0 {
				t.Errorf("Got the unexpected ingress packet count: %d", ingressPkt)
			}

			if p.WantIncrement {
				tolerance := uint64(packetCount * counterTolerancePct / 100)
				ingressAtePkts, egressAtePkts := otgutils.GetFlowStats(t, p.OTGConfig, p.Flow.Name(), timeout)

				if ingressPkt >= ingressAtePkts-tolerance && egressPkt >= egressAtePkts-tolerance {
					t.Logf("Interface counters reflect decapsulated packets: InUnicastPkts: %d OutUnicastPkts: %d", ingressPkt, egressPkt)
				} else {
					t.Errorf("Interface counters didn't reflect decapsulated packets")
				}
			} else {
				t.Logf("PF policy-rule counters unsupported on this DUT; interface-counter deviation path cannot confirm the decap-policy did NOT match (ingressPkt=%d egressPkt=%d)", ingressPkt, egressPkt)
			}

		default:
			t.Errorf("deviation PolicyRuleCountersOCUnsupported is not handled for DUT %v (Bug 457722520)", dut.Vendor())
		}
		return
	}

	isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
	_, ok := gnmi.Watch(
		t,
		dut,
		gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			PolicyForwarding().
			Policy(policyName).
			Rule(policyId).
			MatchedPkts().
			State(),
		timeout,
		isPresent,
	).Await(t)

	if !ok {
		t.Errorf("Unable to find matched packets")
		return
	}

	matchedPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		PolicyForwarding().
		Policy(policyName).
		Rule(policyId).
		MatchedPkts().
		State()
	finalMatchedPkts := gnmi.Get(t, dut, matchedPath)
	if p.WantIncrement {
		finalMatchedPkts = waitForCounterDelta(t, dut, "decap-policy matched-pkts", matchedPath, p.InitialMatchedPkts, minExpectedPkts, counterSettleTimeout)
	}

	delta := counterDelta(finalMatchedPkts, p.InitialMatchedPkts)

	if p.WantIncrement {
		// README: the DUT counters must reflect the 1000 decapsulated packets.
		tolerance := uint64(packetCount * counterTolerancePct / 100)
		if delta < uint64(packetCount)-tolerance || delta > uint64(packetCount)+tolerance {
			t.Errorf("decap-policy matched-pkts counter incremented by %d, want %d +/- %d%% (initial=%d final=%d)", delta, packetCount, counterTolerancePct, p.InitialMatchedPkts, finalMatchedPkts)
		} else {
			t.Logf("PASS: decap-policy matched-pkts counter incremented by %d", delta)
		}
	} else if delta != 0 {
		t.Errorf("decap-policy matched-pkts counter unexpectedly incremented by %d for a flow that should not match the decap rule (initial=%d final=%d)", delta, p.InitialMatchedPkts, finalMatchedPkts)
	} else {
		t.Logf("PASS: decap-policy matched-pkts counter correctly did not increment")
	}
}

// applyImixTrafficProfile applies an IMIX traffic profile to the given flow, setting packet sizes and weights,
// as well as the traffic rate and duration.
func applyImixTrafficProfile(flow gosnappi.Flow) {
	custom := flow.Size().WeightPairs().Custom()
	custom.Add().SetSize(128).SetWeight(7)
	custom.Add().SetSize(512).SetWeight(4)
	custom.Add().SetSize(1518).SetWeight(1)
	flow.Rate().SetPercentage(trafficRatePercent)
	flow.Duration().FixedPackets().SetPackets(packetCount)
}

// gueDecapInnerIpv4Traffic configures and validates GUE decapsulation for inner IPv4 traffic, including traffic and decap-counter verification.
func gueDecapInnerIpv4Traffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config, otgConfig *otg.OTG, ateUdpPort, dutUdpPort int, destIps []string, trafficShouldPass, shouldDecap, verifyDropCounters bool) {
	trafficID := fmt.Sprintf("Gue-Decap-Flow1-%v", ateUdpPort)
	flow := configureIPv4Traffic(t, ate, topo, trafficID, destIps, ateUdpPort)
	configureDutWithGueDecap(t, dut, dutUdpPort, "ipv4")
	enableCapture(t, topo, "port2")
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	cs := otgutils.StartCapture(t, ate.OTG())
	defer otgutils.StopCapture(t, ate.OTG(), cs)
	trafficStartStop(t, dut, ate, topo, otgConfig, flow, trafficShouldPass, shouldDecap, verifyDropCounters)
	otgutils.StopCapture(t, ate.OTG(), cs)
	if trafficShouldPass {
		verifyTrafficFlow(t, ate, trafficID, true)
		if shouldDecap {
			// PF-1.4.1 / PF-1.4.2: decapsulate — verify inner DSCP and TTL
			verifyCaptureDscpTtlValue(t, ate, "port2", "ipv4", int(innerDscpValue), int(innerTTL-1))
		} else {
			// PF-1.4.5: pass-through — verify outer packet unmodified (no decap)
			verifyPassThroughGuePacket(t, ate, "port2", "ipv4")
		}
	} else {
		// PF-1.4.3: negative — the flow must be dropped (no packets received on ATE Port 2)
		verifyTrafficFlow(t, ate, trafficID, false)
	}
}

// gueDecapInnerIpv6Traffic configures and validates GUE decapsulation for inner IPv6 traffic, including traffic and decap-counter verification.
func gueDecapInnerIpv6Traffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, topo gosnappi.Config, otgConfig *otg.OTG, ateUdpPort, dutUdpPort int, destIps []string, trafficShouldPass, shouldDecap, verifyDropCounters bool) {
	trafficID := fmt.Sprintf("Gue-Decap-Flow1-%v", ateUdpPort)
	flow := configureIPv6Traffic(t, ate, topo, trafficID, destIps, ateUdpPort)
	configureDutWithGueDecap(t, dut, dutUdpPort, "ipv6")
	enableCapture(t, topo, "port2")
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	cs := otgutils.StartCapture(t, ate.OTG())
	defer otgutils.StopCapture(t, ate.OTG(), cs)
	trafficStartStop(t, dut, ate, topo, otgConfig, flow, trafficShouldPass, shouldDecap, verifyDropCounters)
	otgutils.StopCapture(t, ate.OTG(), cs)
	if trafficShouldPass {
		verifyTrafficFlow(t, ate, trafficID, true)
		if shouldDecap {
			// PF-1.4.1 / PF-1.4.2: decapsulate — verify inner DSCP and TTL
			verifyCaptureDscpTtlValue(t, ate, "port2", "ipv6", int(innerDscpValue), int(innerTTL-1))
		} else {
			// PF-1.4.6: pass-through — verify outer packet unmodified (no decap)
			verifyPassThroughGuePacket(t, ate, "port2", "ipv6")
		}
	} else {
		// PF-1.4.4: negative — the flow must be dropped (no packets received on ATE Port 2)
		verifyTrafficFlow(t, ate, trafficID, false)
	}
}

// configureDutWithGueDecap configures GUE decapsulation on the DUT for the specified UDP port and
// IP type (IPv4 or IPv6), delegating to cfgplugins.DecapGroupConfigGue for both the native-CLI
// deviation path and the canonical OC path (a destination-address-prefix-set covering the decap
// subnet range, combined with a UDP destination-port match).
func configureDutWithGueDecap(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string) {
	t.Helper()
	t.Logf("Configure DUT with decapsulation UDP port %v", guePort)
	ocPFParams := getDefaultOcPolicyForwardingParams(t, dut, guePort, ipType)
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
}

// getDefaultOcPolicyForwardingParams provides default parameters for the generator, matching the values in the provided JSON example.
// ResetDecapState makes every sub-test push a fresh decap config: anything a previous sub-test left behind
// (e.g. the same UDP port still bound to the other payload type on Arista) is removed first.
func getDefaultOcPolicyForwardingParams(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType string) cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
		InterfaceID:         dut.Port(t, "port1").Name(),
		AppliedPolicyName:   policyName,
		TunnelIP:            decapDstSubnetV6,
		GUEPort:             uint32(guePort),
		IPType:              ipType,
		Dynamic:             true,
		ResetDecapState:     true,
	}
}

// dscpToTrafficClass converts a DSCP value to the IPv6 Traffic Class byte (DSCP occupies the upper
// six bits, the lower two are ECN). Writing the raw DSCP into the field would set DSCP 8 / ECN=CE.
func dscpToTrafficClass(dscp uint32) uint32 { return dscp << 2 }

// configureIPv4Traffic configures the outer-IPv6/GUE + inner-IPv4 stream. destIps cycles the
// outer GUE destination across the given addresses (e.g. the full DECAP-DST-SUBNET-V6/64 range,
// or a single pass-through address), per the README's traffic profile.
func configureIPv4Traffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, trafficID string, destIps []string, guePort int) gosnappi.Flow {
	t.Logf("Configure Traffic from ATE with flowname %s", trafficID)
	topo.Flows().Clear()
	flow := topo.Flows().Add().SetName(trafficID)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{ateIPv4Name(atePort1)}).SetRxNames([]string{ateIPv4Name(atePort2)})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().Auto()
	outerIpHeader := flow.Packet().Add().Ipv6()
	outerIpHeader.Src().SetValue(atePort1.IPv6)
	if len(destIps) == 1 {
		outerIpHeader.Dst().SetValue(destIps[0])
	} else {
		outerIpHeader.Dst().SetValues(destIps)
	}
	outerIpHeader.TrafficClass().SetValue(dscpToTrafficClass(outerDscpValue))
	outerIpHeader.HopLimit().SetValue(outerTTL)
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().Random().SetMin(srcPortMin).SetMax(srcPortMax)
	udpHeader.DstPort().SetValue(uint32(guePort))
	innerIpHeader := flow.Packet().Add().Ipv4()
	innerIpHeader.Src().SetValue(ipv4Src)
	innerIpHeader.Dst().SetValue(ipv4Dst)
	innerIpHeader.Priority().Dscp().Phb().SetValue(innerDscpValue)
	innerIpHeader.TimeToLive().SetValue(innerTTL)
	applyImixTrafficProfile(flow)
	return flow
}

// configureIPv6Traffic configures the outer-IPv6/GUE + inner-IPv6 stream, using the ATE IPv6 interfaces as endpoints,
// destIps cycles the outer GUE destination across the given addresses.
func configureIPv6Traffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, trafficID string, destIps []string, guePort int) gosnappi.Flow {
	t.Logf("Configure Traffic from ATE with flowname %s", trafficID)
	topo.Flows().Clear()
	flow := topo.Flows().Add().SetName(trafficID)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{ateIPv6Name(atePort1)}).SetRxNames([]string{ateIPv6Name(atePort2)})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().Auto()
	outerIpHeader := flow.Packet().Add().Ipv6()
	outerIpHeader.Src().SetValue(atePort1.IPv6)
	if len(destIps) == 1 {
		outerIpHeader.Dst().SetValue(destIps[0])
	} else {
		outerIpHeader.Dst().SetValues(destIps)
	}
	outerIpHeader.TrafficClass().SetValue(dscpToTrafficClass(outerDscpValue))
	outerIpHeader.HopLimit().SetValue(outerTTL)
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().Random().SetMin(srcPortMin).SetMax(srcPortMax)
	udpHeader.DstPort().SetValue(uint32(guePort))
	innerIpHeader := flow.Packet().Add().Ipv6()
	innerIpHeader.Src().SetValue(ipv6Src)
	innerIpHeader.Dst().SetValue(ipv6Dst)
	innerIpHeader.TrafficClass().SetValue(dscpToTrafficClass(innerDscpValue))
	innerIpHeader.HopLimit().SetValue(innerTTL)
	applyImixTrafficProfile(flow)
	return flow
}

// verifyCaptureDscpTtlValue validates the README's RX-Flow-type#1/#2: every decapsulated packet captured on
// the specified ATE port must be the inner packet (IPV4-SRC1->IPV4-DST1 or IPV6-SRC1->IPV6-DST1) with
// the inner DSCP preserved and the inner TTL decremented by exactly one.
// innerType is "ipv4" or "ipv6".
func verifyCaptureDscpTtlValue(t *testing.T, ate *ondatra.ATEDevice, port, innerType string, dscp int, ttl int) {
	t.Helper()
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	matched, bad := 0, 0
	for packet := range packetSource.Packets() {
		gotDscp, gotTTL := 0, 0
		family := ""
		dstOK := false
		if ip, ok := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4); ok && innerType == "ipv4" && ip.SrcIP.Equal(net.ParseIP(ipv4Src)) {
			family, gotDscp, gotTTL = "IPv4", int(ip.TOS>>2), int(ip.TTL)
			dstOK = ip.DstIP.Equal(net.ParseIP(ipv4Dst))
		} else if ip6, ok := packet.Layer(layers.LayerTypeIPv6).(*layers.IPv6); ok && innerType == "ipv6" && ip6.SrcIP.Equal(net.ParseIP(ipv6Src)) {
			// The DSCP is the upper six bits of the IPv6 Traffic Class.
			family, gotDscp, gotTTL = "IPv6", int(ip6.TrafficClass>>2), int(ip6.HopLimit)
			dstOK = ip6.DstIP.Equal(net.ParseIP(ipv6Dst))
		} else {
			continue
		}
		matched++
		if !dstOK || gotDscp != dscp || gotTTL != ttl {
			bad++
			if bad <= 3 {
				t.Logf("%s packet mismatch after decap. Expected: DSCP %v, TTL %v, inner dst ok=true Got: DSCP %v, TTL %v, inner dst ok=%v", family, dscp, ttl, gotDscp, gotTTL, dstOK)
			}
		}
	}
	switch {
	case matched == 0:
		t.Errorf("Could not find decapsulated %s packet with inner source IP (%s or %s) in capture", innerType, ipv4Src, ipv6Src)
	case bad > 0:
		t.Errorf("DSCP/TTL/destination not as expected after decap for %d of %d captured packets. Expected: DSCP %v, TTL %v", bad, matched, dscp, ttl)
	default:
		t.Logf("PASS: DSCP value %v and TTL value %v are preserved on all %d captured decapsulated packets", dscp, ttl, matched)
	}
}

// verifyPassThroughGuePacket validates the README's RX-Flow-type#5/#6: the GUE packet is forwarded undecapsulated
// to ATE Port 2 with only the outer TTL decremented (outer DSCP, UDP port 6081 and the inner header untouched).
// innerType is "ipv4" or "ipv6".
func verifyPassThroughGuePacket(t *testing.T, ate *ondatra.ATEDevice, port, innerType string) {
	t.Helper()
	pcapfilename := processCapture(t, ate, port)
	handle, err := pcap.OpenOffline(pcapfilename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	wantInnerVersion := byte(4)
	if innerType == "ipv6" {
		wantInnerVersion = 6
	}
	wantOuterTTL := uint8(outerTTL - 1)
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	matched, bad := 0, 0
	for packet := range packetSource.Packets() {
		ip6, ok := packet.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
		if !ok || !ip6.SrcIP.Equal(net.ParseIP(atePort1.IPv6)) {
			continue
		}
		udp, ok := packet.Layer(layers.LayerTypeUDP).(*layers.UDP)
		if !ok {
			continue
		}
		matched++
		errs := []string{}
		if !ip6.DstIP.Equal(net.ParseIP(atePort2.IPv6)) {
			errs = append(errs, fmt.Sprintf("outer dst %s, want %s", ip6.DstIP, atePort2.IPv6))
		}
		if ip6.HopLimit != wantOuterTTL {
			errs = append(errs, fmt.Sprintf("outer TTL %d, want %d", ip6.HopLimit, wantOuterTTL))
		}
		if got := uint32(ip6.TrafficClass >> 2); got != outerDscpValue {
			errs = append(errs, fmt.Sprintf("outer DSCP %d, want %d", got, outerDscpValue))
		}
		if int(udp.DstPort) != gueDecapPort {
			errs = append(errs, fmt.Sprintf("UDP dst port %d, want %d", udp.DstPort, gueDecapPort))
		}
		// GUE variant 1: the UDP payload is the inner IP packet itself. (gopacket would decode UDP/6081
		// as Geneve, so the inner header is parsed explicitly from the UDP payload.)
		if len(udp.Payload) == 0 || udp.Payload[0]>>4 != wantInnerVersion {
			errs = append(errs, "inner IP version mismatch or empty UDP payload")
		} else if wantInnerVersion == 4 {
			if in, ok := gopacket.NewPacket(udp.Payload, layers.LayerTypeIPv4, gopacket.Default).Layer(layers.LayerTypeIPv4).(*layers.IPv4); ok {
				if uint32(in.TTL) != innerTTL || uint32(in.TOS>>2) != innerDscpValue {
					errs = append(errs, fmt.Sprintf("inner IPv4 TTL/DSCP %d/%d, want %d/%d", in.TTL, in.TOS>>2, innerTTL, innerDscpValue))
				}
			} else {
				errs = append(errs, "inner IPv4 header could not be decoded")
			}
		} else {
			if in, ok := gopacket.NewPacket(udp.Payload, layers.LayerTypeIPv6, gopacket.Default).Layer(layers.LayerTypeIPv6).(*layers.IPv6); ok {
				if uint32(in.HopLimit) != innerTTL || uint32(in.TrafficClass>>2) != innerDscpValue {
					errs = append(errs, fmt.Sprintf("inner IPv6 hop-limit/DSCP %d/%d, want %d/%d", in.HopLimit, in.TrafficClass>>2, innerTTL, innerDscpValue))
				}
			} else {
				errs = append(errs, "inner IPv6 header could not be decoded")
			}
		}
		if len(errs) > 0 {
			bad++
			if bad <= 3 {
				t.Logf("pass-through packet mismatch: %v", errs)
			}
		}
	}
	switch {
	case matched == 0:
		t.Errorf("Could not find GUE pass-through packet with outer src IP (%s) in capture", atePort1.IPv6)
	case bad > 0:
		t.Errorf("%d of %d GUE pass-through packets were modified or malformed", bad, matched)
	case matched != packetCount:
		t.Errorf("Captured %d GUE pass-through packets, want %d (no packet loss expected)", matched, packetCount)
	default:
		t.Logf("PASS: %d GUE pass-through packets verified: outer dst=%s, DSCP=%d, UDP port=%d, TTL=%d (decremented by 1), inner header unchanged",
			matched, atePort2.IPv6, outerDscpValue, gueDecapPort, wantOuterTTL)
	}
}
