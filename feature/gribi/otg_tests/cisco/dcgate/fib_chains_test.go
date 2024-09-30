package dcgate_test

import (
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestFibChains(t *testing.T) {

	test := []struct {
		name      string
		desc      string
		fn        func(t *testing.T, args *testArgs)
		chainType string
	}{
		{
			name:      "TestEncapDcgateOptimized",
			desc:      "Verify TTL and DSCP values in Encap Dcgate Optimized chain with triggers",
			fn:        testEncapDcgateOptimized,
			chainType: "dcgate_cluster_optimized",
		},
		{
			name:      "TestTransitDcgateOptimized",
			desc:      "Verify TTL and DSCP values in Transit Dcgate Optimized chain with triggers",
			fn:        testTransitDcgateOptimized,
			chainType: "dcgate_wan_optimized",
		},
		{
			name:      "TestTransitDcgateUnoptimized",
			desc:      "Verify TTL and DSCP values in Transit Dcgate UnOptimized chain with triggers",
			fn:        testTransitDcgateUnoptimized,
			chainType: "dcgate_wan_unoptimized",
		},
		{
			name:      "TestPopGateOptimized",
			desc:      "Verify TTL and DSCP values in PopGate Optimized chain with triggers",
			fn:        testPopGateOptimized,
			chainType: "popgate_optimized",
		},
		{
			name:      "TestPopGateUnOptimized",
			desc:      "Verify TTL and DSCP values in PopGate UnOptimized chain with triggers",
			fn:        testPopGateUnOptimized,
			chainType: "popgate_unoptimized",
		},
	}

	dut := ondatra.DUT(t, "dut")
	for _, tc := range test {

		// Configure DUT based on chain type
		switch tc.chainType {
		case "dcgate_cluster_optimized":
			configureDUT(t, dut, true)
		case "dcgate_wan_optimized":
			configureDUT(t, dut, false)
		case "dcgate_wan_unoptimized":
			configureDUT(t, dut, false)
		case "popgate_optimized":
			configureDUTforPopGate(t, dut)
		case "popgate_unoptimized":
			configureDUTforPopGate(t, dut)
		}
		// Configure ATE
		otg := ondatra.ATE(t, "ate")
		topo := configureOTG(t, otg)

		// configure gRIBI client
		c := gribi.Client{
			DUT:         dut,
			FIBACK:      true,
			Persistence: true,
		}

		if err := c.Start(t); err != nil {
			t.Fatalf("gRIBI Connection can not be established")
		}

		defer c.Close(t)
		c.BecomeLeader(t)
		defer c.FlushAll(t)

		// Flush all existing AFT entries on the router
		c.FlushAll(t)

		tcArgs := &testArgs{
			client: &c,
			dut:    dut,
			ate:    otg,
			topo:   topo,
		}

		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.desc)
			tc.fn(t, tcArgs)
		})
	}
}

func testEncapDcgateOptimized(t *testing.T, args *testArgs) {
	configFallBackVrf(t, args.dut, []string{vrfEncapA})
	configDefaultRoute(t, args.dut, cidr(ipv4FlowIP, 32), otgPort5.IPv4, cidr(ipv6FlowIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv4EntryPrefix, ipv4EntryPrefixLen)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv6EntryPrefix, ipv6EntryPrefixLen)).Config())

	args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 102, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), 102, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 103, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), 103, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup to repaired
	args.client.AddNH(t, baseNH(5), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(5), map[uint64]uint64{baseNH(5): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(5)})
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), 303, vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 403, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, 503, map[uint64]uint64{403: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// backup end

	args.client.AddNH(t, 201, vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 202, vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 301, map[uint64]uint64{201: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 503})
	args.client.AddNHG(t, 302, map[uint64]uint64{202: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), 301, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), 302, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 401, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, 402, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, 502, map[uint64]uint64{402: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 501, map[uint64]uint64{401: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 502})

	// prefixes
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
	args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
	args.flows = []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)}

	t.Run("traffic through primary path", func(t *testing.T) {
		weights := []float64{1, 0, 0, 0}
		args.capture_ports = []string{"port2"}
		testEncapTrafficTtlDscp(t, args, weights, true)
	})
	t.Run("traffic through primary path inner ttl0", func(t *testing.T) {
		weights := []float64{1, 0, 0, 0}
		args.capture_ports = []string{"port2"}

		// packet should be dropped by router, below are dummy values
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 0}

		fa4.ttl = 0
		args.flows = []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)}

		// packet gets dropped by router with below trap - show controllers npu stats traps-all instance 0 location 0/RP0/CPU0 | ex 0 0
		// V4_HEADER_ERROR_OR_TTL0(D*)                   0    68   RPLC_CPU    272   1586  0    67         150        IFG     64      0                    154
		testEncapTrafficTtlDscp(t, args, weights, false) // expect no traffic

		// restore flow
		defer func() {
			fa4.ttl = ttl
			args.flows = []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)}
		}()
	})
	t.Run("traffic through primary path inner ttl1", func(t *testing.T) {
		weights := []float64{1, 0, 0, 0}
		args.capture_ports = []string{"port2"}

		// packet should be dropped by router, below are dummy values
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}

		fa4.ttl = 1
		args.flows = []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)}

		// packet gets dropped by router with below trap - show controllers npu stats traps-all instance 0 location 0/RP0/CPU0 | ex 0 0
		// TTL_OR_HOP_COUNT_1_RX                         0    153  RPLC_CPU    277   1538  5    67         150        IFG     64      154                  0
		testEncapTrafficTtlDscp(t, args, weights, false) // expect no traffic

		// restore flow
		defer func() {
			fa4.ttl = ttl
			args.flows = []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)}
		}()
	})
	t.Run("mismatch in encap vrf and encap with fallback vrf", func(t *testing.T) {
		args.client.DeleteIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), vrfEncapA, fluent.InstalledInFIB)
		args.client.DeleteIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), vrfEncapB, fluent.InstalledInFIB)
		args.client.DeleteIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), vrfEncapA, fluent.InstalledInFIB)
		args.client.DeleteIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), vrfEncapB, fluent.InstalledInFIB)

		defer func() {
			args.client.AddNHG(t, 501, map[uint64]uint64{401: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 502})

			args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		}()

		weights := []float64{0, 0, 0, 1}
		args.capture_ports = []string{"port5"}
		args.pattr = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		testEncapTrafficTtlDscp(t, args, weights, true)
	})
	t.Run("mismatch in encap vrf and encap vrf with backup nhg", func(t *testing.T) {
		args.client.DeleteIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), vrfEncapA, fluent.InstalledInFIB)
		args.client.DeleteIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), vrfEncapB, fluent.InstalledInFIB)
		args.client.DeleteIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), vrfEncapA, fluent.InstalledInFIB)
		args.client.DeleteIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), vrfEncapB, fluent.InstalledInFIB)
		unconfigFallBackVrf(t, args.dut, []string{vrfEncapA})
		// backup path in vrfEncapA
		args.client.AddNH(t, baseNH(11), "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
		args.client.AddNHG(t, baseNHG(11), map[uint64]uint64{baseNH(11): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddNHG(t, 501, map[uint64]uint64{401: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(11)})
		args.client.AddIPv4(t, "0.0.0.0/0", baseNHG(11), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddIPv6(t, "0::0/0", baseNHG(11), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

		defer func() {
			configFallBackVrf(t, args.dut, []string{vrfEncapA})
			args.client.AddNHG(t, 501, map[uint64]uint64{401: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 502})

			args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			args.client.DeleteIPv4(t, "0.0.0.0/0", vrfEncapB, fluent.InstalledInFIB)
			args.client.DeleteIPv6(t, "0::0/0", vrfEncapA, fluent.InstalledInFIB)
		}()

		weights := []float64{0, 0, 0, 1}
		args.capture_ports = []string{"port5"}
		args.pattr = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		testEncapTrafficTtlDscp(t, args, weights, true)
	})
	t.Run("frr1 shutdown primary path for tunnel1", func(t *testing.T) {
		t.Log("Shutdown link carrying primary traffic for tunnel1 to vip1")
		gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
		defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
		args.capture_ports = []string{"port4"}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99} // tunnel traffic from repaired path with tunnelDstIP3
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		weights := []float64{0, 0, 1, 0}
		testEncapTrafficTtlDscp(t, args, weights, true)
	})
	t.Run("frr1 shutdown link carrying backup tunnel traffic to vip2", func(t *testing.T) {
		t.Log("Shutdown link carrying backup tunnel2 traffic to vip2")
		shutPorts(t, args, []string{"port2", "port3"})
		defer unshutPorts(t, args, []string{"port2", "port3"})
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.capture_ports = []string{"port4"}
		args.flows = []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)}

		weights := []float64{0, 0, 1, 0}
		testEncapTrafficTtlDscp(t, args, weights, true)
	})
	t.Run("frr2 shutdown link carrying decap encap traffic to vip3", func(t *testing.T) {
		t.Log("Shutdown link carrying decap encap traffic to vip3")
		shutPorts(t, args, []string{"port2", "port3", "port4"})
		defer unshutPorts(t, args, []string{"port2", "port3", "port4"})

		args.pattr = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.capture_ports = []string{"port5"}
		weights := []float64{0, 0, 0, 1}
		testEncapTrafficTtlDscp(t, args, weights, true)
	})
}

func testTransitDcgateOptimized(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc111
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// match in decap vrf, decap traffic and schedule to match in encap vrf
	args.client.AddNH(t, decapNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, decapNHG(1), map[uint64]uint64{decapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// encap start
	args.client.AddNH(t, encapNH(1), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, encapNHG(1), map[uint64]uint64{encapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// inner hdr prefixes
	args.client.AddIPv4(t, cidr(innerV4DstIP, 32), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(InnerV6DstIP, 128), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// configure repair path with backup
	configureVIP3NHGWithRepairTunnelHavingBackupDecapAction(t, args)

	// transit path
	configureVIP2(t, args)
	args.client.AddNH(t, vipNH(2), vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: tunNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// set packet attributes
	faTransit.innerDscp = dscpEncapA1
	faTransit.ttl = ttl
	faTransit.innerTtl = 50

	t.Run("match in decap outer ttl0", func(t *testing.T) {
		faTransit.innerDscp = dscpEncapA1
		faTransit.innerTtl = 50
		faTransit.ttl = 0
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port3"}
		weights := []float64{0, 1, 0, 0}
		// dummy values for packet attributes, as packet should get dropped
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 50}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}

		// packet gets dropped by router with below trap - show controllers npu stats traps-all instance 0 location 0/RP0/CPU0 | ex 0 0
		// V4_HEADER_ERROR_OR_TTL0(D*)                   0    68   RPLC_CPU    272   1586  0    67         150        IFG     64      0                    154
		testTransitTrafficWithTtlDscp(t, args, weights, false) // traffic should get dropped

		// restore flow
		defer func() {
			faTransit.ttl = ttl
			args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		}()
	})
	t.Run("match in decap outer ttl1", func(t *testing.T) {
		faTransit.innerDscp = dscpEncapA1
		faTransit.innerTtl = 50
		faTransit.ttl = 1
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port3"}
		weights := []float64{0, 1, 0, 0}
		// dummy values for packet attributes, as packet should get dropped
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 50}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}

		// packet gets dropped by router with below trap - show controllers npu stats traps-all instance 0 location 0/RP0/CPU0 | ex 0 0
		// TTL_OR_HOP_COUNT_1_RX                         0    153  RPLC_CPU    277   1538  5    67         150        IFG     64      154                  0
		testTransitTrafficWithTtlDscp(t, args, weights, false) // traffic should get dropped

		// restore flow
		defer func() {
			faTransit.ttl = ttl
			args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		}()
	})
	t.Run("miss in decap then fallback to transit", func(t *testing.T) {
		t.Log("Remove decap prefix from decap vrf and verify traffic goes to fallback vrf vrfTransit.")
		args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfDecap, fluent.InstalledInFIB)
		args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		// verify traffic passes through primary NHG
		faTransit.innerDscp = dscpEncapA1
		faTransit.innerTtl = 50
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port3"}
		weights := []float64{0, 1, 0, 0}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 50} //transit traffic should decrement only outer ttl
		testTransitTrafficWithTtlDscp(t, args, weights, true)
		args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfTransit, fluent.InstalledInFIB)
	})
	t.Run("miss in decap and transit with default route", func(t *testing.T) {
		t.Log("Remove tunnel prefix from decap and transit vrf and verify traffic goes via default route in vrfTransit.")
		args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfDecap, fluent.InstalledInFIB)
		args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfTransit, fluent.InstalledInFIB)
		configIPv4DefaultRoute(t, args.dut, cidr(tunnelDstIP1, 32), otgPort5.IPv4)
		// default route via NHG in vrfTransit
		args.client.AddNH(t, baseNH(11), "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
		args.client.AddNHG(t, baseNHG(11), map[uint64]uint64{baseNH(11): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddIPv4(t, "0.0.0.0/0", baseNHG(11), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddIPv6(t, "0::0/0", baseNHG(11), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

		// verify traffic passes through primary NHG
		faTransit.innerDscp = dscpEncapA1
		faTransit.innerTtl = 50
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port5"}
		weights := []float64{0, 0, 0, 1}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 50} //transit traffic should decrement only outer ttl
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		defer func() {
			args.client.DeleteIPv4(t, "0.0.0.0/0", vrfTransit, fluent.InstalledInFIB)
			args.client.DeleteIPv6(t, "0::0/0", vrfTransit, fluent.InstalledInFIB)

			args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
			gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(tunnelDstIP1, 32)).Config())
		}()

	})
	t.Run("match in decap goto encap", func(t *testing.T) {
		t.Log("Add decap prefix back to decap vrf to decapsulate traffic and then schedule to match in encap vrf")
		// verify traffic passes through primary NHG
		faTransit.innerDscp = dscpEncapA1
		faTransit.innerTtl = 50
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port3"}
		weights := []float64{0, 1, 0, 0}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		args.pattr = &packetAttr{dscp: 10, protocol: ipv6ipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("frr1 shutdown primary path goto repair path", func(t *testing.T) {
		t.Log("Shutdown primary path for transit tunnel and verify traffic goes via repair path tunnel")
		gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
		defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)
		args.capture_ports = []string{"port4"}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		weights := []float64{0, 0, 1, 0}
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		args.pattr = &packetAttr{dscp: 10, protocol: ipv6ipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("frr2 shutdown repair path goto default vrf", func(t *testing.T) {
		t.Log("Shutdown transit and repair tunnel paths and verify traffic passes through default vrf")
		shutPorts(t, args, []string{"port3", "port4"})
		defer unshutPorts(t, args, []string{"port3", "port4"})

		// configure default route
		configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())
		args.capture_ports = []string{"port5"}
		weights := []float64{0, 0, 0, 1}
		args.pattr = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("match in decap nomatch in encap", func(t *testing.T) {
		configDefaultRoute(t, args.dut, "0.0.0.0/0", otgPort5.IPv4, "0::/0", otgPort5.IPv6)
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static("0.0.0.0/0").Config())
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static("0::/0").Config())

		shutPorts(t, args, []string{"port3", "port4"})
		defer unshutPorts(t, args, []string{"port3", "port4"})
		// add static route for encap vrf
		args.client.AddNH(t, baseNH(1001), "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
		args.client.AddNHG(t, baseNHG(1001), map[uint64]uint64{baseNH(1001): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddIPv4(t, "0.0.0.0/0", baseNHG(1001), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddIPv6(t, "0::0/0", baseNHG(1001), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

		t.Log("Delete prefix from encap vrf and verify traffic goes to default vrf")
		args.client.DeleteIPv4(t, cidr(innerV4DstIP, 32), vrfEncapA, fluent.InstalledInFIB)
		args.client.DeleteIPv6(t, cidr(InnerV6DstIP, 128), vrfEncapA, fluent.InstalledInFIB)

		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port5"}
		weights := []float64{0, 0, 0, 1}
		args.pattr = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 49} //original ttl is 50
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		t.Log("Add back prefix to encap vrf")
		args.client.AddIPv4(t, cidr(innerV4DstIP, 32), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddIPv6(t, cidr(InnerV6DstIP, 128), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	})
}

// unoptimized Dcgate tests
func testTransitDcgateUnoptimized(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc111
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// match in decap vrf, decap traffic and schedule to match in encap vrf
	args.client.AddNH(t, decapNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, decapNHG(1), map[uint64]uint64{decapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// encap start
	args.client.AddNH(t, encapNH(1), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, encapNHG(1), map[uint64]uint64{encapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// inner hdr prefixes
	args.client.AddIPv4(t, cidr(innerV4DstIP, 32), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(InnerV6DstIP, 128), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// unoptimized repair path
	args.client.AddNH(t, baseNH(20), "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfRepair})
	args.client.AddNHG(t, baseNHG(20), map[uint64]uint64{baseNH(20): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// configure repair path with backup
	// backup to repair
	args.client.AddNH(t, baseNH(5), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(5), map[uint64]uint64{baseNH(5): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	configureVIP3(t, args)
	// repair path
	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(5)})
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, tunNH(3), "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, tunNHG(3), map[uint64]uint64{tunNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), tunNHG(3), vrfRepair, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), tunNHG(3), vrfRepair, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// transit path
	configureVIP2(t, args)
	args.client.AddNH(t, vipNH(2), vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(20)})
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// set packet attributes
	faTransit.innerDscp = dscpEncapA1
	faTransit.ttl = ttl
	faTransit.innerTtl = 50

	t.Run("miss in decap fallback to transit", func(t *testing.T) {
		t.Log("Remove decap prefix from decap vrf and verify traffic goes to fallback vrf vrfTransit.")
		args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfDecap, fluent.InstalledInFIB)
		args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		// verify traffic passes through primary NHG
		faTransit.innerDscp = dscpEncapA1
		faTransit.innerTtl = 50
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port3"}
		weights := []float64{0, 1, 0, 0}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 50} //transit traffic should decrement only outer ttl
		testTransitTrafficWithTtlDscp(t, args, weights, true)
		args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfTransit, fluent.InstalledInFIB)
	})
	t.Run("match in decap goto encap", func(t *testing.T) {
		t.Log("Add decap prefix back to decap vrf to decapsulate traffic and schedule to match in encap vrf")

		// verify traffic passes through primary NHG
		faTransit.innerDscp = dscpEncapA1
		faTransit.innerTtl = 50
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port3"}
		weights := []float64{0, 1, 0, 0}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		args.pattr = &packetAttr{dscp: 10, protocol: ipv6ipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("frr1 shutdown primary path goto repair path", func(t *testing.T) {
		t.Log("Shutdown primary path for transit path tunnel and verify traffic goes via repair path tunnel")
		gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
		defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)
		args.capture_ports = []string{"port4"}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		weights := []float64{0, 0, 1, 0}
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		args.pattr = &packetAttr{dscp: 10, protocol: ipv6ipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("frr2 shutdown repair path goto default vrf", func(t *testing.T) {
		t.Log("Shutdown transit and repair tunnel paths and verify traffic passes through default vrf")
		shutPorts(t, args, []string{"port3", "port4"})
		defer unshutPorts(t, args, []string{"port3", "port4"})

		// configure default route
		configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())
		args.capture_ports = []string{"port5"}
		weights := []float64{0, 0, 0, 1}
		args.pattr = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("match in decap nomatch in encap", func(t *testing.T) {
		configDefaultRoute(t, args.dut, "0.0.0.0/0", otgPort5.IPv4, "0::/0", otgPort5.IPv6)
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static("0.0.0.0/0").Config())
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static("0::/0").Config())

		shutPorts(t, args, []string{"port3", "port4"})
		defer unshutPorts(t, args, []string{"port3", "port4"})
		// add static route for encap vrf
		args.client.AddNH(t, baseNH(1001), "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
		args.client.AddNHG(t, baseNHG(1001), map[uint64]uint64{baseNH(1001): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddIPv4(t, "0.0.0.0/0", baseNHG(1001), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddIPv6(t, "0::0/0", baseNHG(1001), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

		t.Log("Delete prefix from encap vrf and verify traffic goes to default vrf")
		args.client.DeleteIPv4(t, cidr(innerV4DstIP, 32), vrfEncapA, fluent.InstalledInFIB)
		args.client.DeleteIPv6(t, cidr(InnerV6DstIP, 128), vrfEncapA, fluent.InstalledInFIB)

		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port5"}
		weights := []float64{0, 0, 0, 1}
		args.pattr = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 49} //original ttl is 50
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		t.Log("add back prefix to encap vrf")
		args.client.AddIPv4(t, cidr(innerV4DstIP, 32), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		args.client.AddIPv6(t, cidr(InnerV6DstIP, 128), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	})
}

func testPopGateUnOptimized(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc111
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// unoptimized repair path
	args.client.AddNH(t, baseNH(20), "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfRepair})
	args.client.AddNHG(t, baseNHG(20), map[uint64]uint64{baseNH(20): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// configure repair path with backup
	// backup to repaired
	args.client.AddNH(t, baseNH(5), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(5), map[uint64]uint64{baseNH(5): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// repaired path
	configureVIP3(t, args)
	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(5)})
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, tunNH(3), "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, tunNHG(3), map[uint64]uint64{tunNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), tunNHG(3), vrfRepair, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), tunNHG(3), vrfRepair, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// transit path
	configureVIP2(t, args)
	args.client.AddNH(t, vipNH(2), vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(20)})
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// set flow attributes
	faTransit.innerDscp = dscpEncapA1
	faTransit.innerTtl = 50

	t.Run("traffic via primary transit path", func(t *testing.T) {
		t.Log("Remove decap prefix from decap vrf and verify traffic goes to fallback vrf vrfTransit.")
		args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfDecap, fluent.InstalledInFIB)
		args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		// verify traffic passes through primary NHG
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port3"}
		weights := []float64{0, 1, 0, 0}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 50} //transit traffic should decrement only outer ttl
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("frr1 shutdown primary path goto repair path", func(t *testing.T) {
		t.Log("Shutdown primary path for Transit tunnel and verify traffic goes via repair path tunnel")
		gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
		defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)
		args.capture_ports = []string{"port4"}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		weights := []float64{0, 0, 1, 0}
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		args.pattr = &packetAttr{dscp: 10, protocol: ipv6ipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("frr2 shutdown repair path goto default vrf", func(t *testing.T) {
		t.Log("Shutdown transit and repair tunnel paths and verify traffic passes through default vrf")
		shutPorts(t, args, []string{"port3", "port4"})
		defer unshutPorts(t, args, []string{"port3", "port4"})

		// configure default route
		configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

		args.capture_ports = []string{"port5"}
		weights := []float64{0, 0, 0, 1}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.pattr = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
}

func testPopGateOptimized(t *testing.T, args *testArgs) {

	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc111
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// configure repair path with backup
	// backup to repaired
	args.client.AddNH(t, baseNH(5), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(5), map[uint64]uint64{baseNH(5): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// repaired path
	configureVIP3(t, args)
	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(5)})
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, tunNH(3), "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, tunNHG(3), map[uint64]uint64{tunNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), tunNHG(3), vrfRepair, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), tunNHG(3), vrfRepair, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// transit path
	configureVIP2(t, args)
	args.client.AddNH(t, vipNH(2), vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: tunNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// set flow attributes
	faTransit.innerDscp = dscpEncapA1
	faTransit.innerTtl = 50

	t.Run("traffic via primary transit path", func(t *testing.T) {
		t.Log("Remove decap prefix from decap vrf and verify traffic goes to fallback vrf vrfTransit.")
		args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfDecap, fluent.InstalledInFIB)
		args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
		// verify traffic passes through primary NHG
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.capture_ports = []string{"port3"}
		weights := []float64{0, 1, 0, 0}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 50} //transit traffic should decrement only outer ttl
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("frr1 shutdown primary path goto repair path", func(t *testing.T) {
		t.Log("Shutdown primary path for Transit tunnel and verify traffic goes via repair path tunnel")
		gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
		defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)
		args.capture_ports = []string{"port4"}
		args.pattr = &packetAttr{dscp: 10, protocol: ipipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		weights := []float64{0, 0, 1, 0}
		testTransitTrafficWithTtlDscp(t, args, weights, true)

		args.pattr = &packetAttr{dscp: 10, protocol: ipv6ipProtocol, ttl: 99}
		args.pattr.inner = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
	t.Run("frr2 shutdown repair path goto default vrf", func(t *testing.T) {
		t.Log("Shutdown transit and repair tunnel paths and verify traffic passes through default vrf")
		shutPorts(t, args, []string{"port3", "port4"})
		defer unshutPorts(t, args, []string{"port3", "port4"})

		// configure default route
		configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
		defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

		args.capture_ports = []string{"port5"}
		weights := []float64{0, 0, 0, 1}
		args.flows = []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1)}
		args.pattr = &packetAttr{dscp: 10, protocol: udpProtocol, ttl: 99}
		testTransitTrafficWithTtlDscp(t, args, weights, true)
	})
}
