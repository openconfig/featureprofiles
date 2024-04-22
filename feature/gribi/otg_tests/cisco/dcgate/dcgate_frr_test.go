package dcgate_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestEncapFrr(t *testing.T) {
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut, true)

	// Configure ATE
	otg := ondatra.ATE(t, "ate")
	topo := configureOTG(t, otg)

	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
		skip bool
	}{
		{
			name: "EncapWithVIPHavingBackupNHG",
			desc: "Encap_12: Test Encap with backup NHG for a VIP",
			fn:   testBackupNextHopGroup,
		},
		{
			name: "SSFRRTunnelPrimaryPathUnviable",
			desc: "SFRR_01: Test self site FRR with primary path of a tunnel being unviable and backup being present",
			fn:   testUnviableTunnelBackupNextHopGroup,
		},
		{
			name: "SSFRRTunnelPrimaryAndBackupPathUnviable",
			desc: "SFRR_03: Test self site FRR with primary and backup path of a tunnel being unviable then traffic is shared via other tunnel.",
			fn:   testUnviableTunnelBothPrimaryBackupDown,
		},
		{
			name: "SSFRRTunnelPrimaryAndBackupPathUnviableForAllTunnel",
			desc: "SFRR_02: Test self site FRR with primary and backup path of all tunnels being unviable then traffic is routed via default vrf.",
			fn:   testAllTunnelUnviable,
			skip: true,
		},
		{
			name: "SFRRBackupNHGTunneltoPrimaryTunnelWhenPrimaryTunnelUnviable",
			desc: "SFRR_06: Test self site FRR with primary and backup path of a tunnel being unviable then traffic is shared via other tunnel.",
			fn:   testBackupNHGTunnelToUnviableTunnel,
		},
		{
			name: "SFRRPrimaryBackupNHGforTunnelUnviable",
			desc: "SFRR_07: Verify when backup NextHopGroup is also unviable, the cluster traffic is NOT encap-ed and falls back to the BGP routes in the DEFAULT VRF",
			fn:   testPrimaryAndBackupNHGUnviableForTunnel,
			skip: true,
		},
		{
			name: "SFRRPrimaryPathUnviableWithooutBNHG",
			desc: "SFRR_08: Verify when original NextHopGroup is unviable and it does not have a backup NextHopGroup, the cluster traffic is NOT encap-ed and falls back to the BGP routes in the DEFAULT VRF.",
			fn:   testPrimaryPathUnviableWihoutBackupNHG,
		},
	}

	for _, tc := range test {
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
			if tc.skip {
				t.SkipNow()
			}
			tc.fn(t, tcArgs)
		})
	}
}

func testBackupNextHopGroup(t *testing.T, args *testArgs) {
	if deviations.GRIBIMACOverrideWithStaticARP(args.dut) {
		args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
		args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})

	} else {
		args.client.AddNH(t, 2, "MACwithInterface", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: args.dut.Port(t, "port2").Name(), Mac: magicMac})
		args.client.AddNH(t, 3, "MACwithInterface", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: args.dut.Port(t, "port3").Name(), Mac: magicMac})
	}
	args.client.AddNHG(t, 100, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 101, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 100})
	args.client.AddIPv4(t, cidr(vipIP1, 32), 101, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, nh1ID, vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, nh201ID, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, nh202ID, "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, nhg10ID, map[uint64]uint64{nh201ID: 1, nh202ID: 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	weights = []float64{0, 1, 0, 0}
	testTraffic(t, args, weights, true)
}

func testUnviableTunnelBackupNextHopGroup(t *testing.T, args *testArgs) {

	args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 102, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), 102, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 103, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), 103, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
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
	args.client.AddNHG(t, 501, map[uint64]uint64{401: 1, 402: 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// prefixes
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{0.25, 0.75, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	weights = []float64{0, 0.75, 0.25, 0}
	testTraffic(t, args, weights, true)
}

func testUnviableTunnelBothPrimaryBackupDown(t *testing.T, args *testArgs) {
	args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 102, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), 102, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 103, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), 103, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
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
	args.client.AddNHG(t, 501, map[uint64]uint64{401: 1, 402: 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// prefixes
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{0.25, 0.75, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 1, 0, 0}
	testTraffic(t, args, weights, true)
}

func testAllTunnelUnviable(t *testing.T, args *testArgs) {
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

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
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
	args.client.AddNHG(t, 501, map[uint64]uint64{401: 1, 402: 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// prefixes
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), 501, vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{0.25, 0.75, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown all primary and backup paths")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 0, 0, 1}
	testTraffic(t, args, weights, true)
}

func testBackupNHGTunnelToUnviableTunnel(t *testing.T, args *testArgs) {
	args.client.AddNH(t, 2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 102, map[uint64]uint64{2: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), 102, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, 3, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 103, map[uint64]uint64{3: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), 103, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
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

	weights := []float64{1, 0, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary and backup for tunnel1 traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 1, 0, 0}
	testTraffic(t, args, weights, true)
}

// SSFRR_07
func testPrimaryAndBackupNHGUnviableForTunnel(t *testing.T, args *testArgs) {
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

	// backup
	args.client.AddNH(t, 4, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, 104, map[uint64]uint64{4: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), 104, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, 203, vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, 303, map[uint64]uint64{203: 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
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

	weights := []float64{1, 0, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary and backup for tunnel1 traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 0, 0, 1}
	testTraffic(t, args, weights, true)
}

// SSFRR_08
func testPrimaryPathUnviableWihoutBackupNHG(t *testing.T, args *testArgs) {
	configFallBackVrf(t, args.dut, []string{vrfEncapA})
	configDefaultRoute(t, args.dut, cidr(ipv4FlowIP, 32), otgPort5.IPv4, cidr(ipv6FlowIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv4EntryPrefix, ipv4EntryPrefixLen)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(ipv6EntryPrefix, ipv6EntryPrefixLen)).Config())

	configureVIP1(t, args)

	args.client.AddNH(t, vipNH(1), vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), vipNHG(1), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, tunNH(1), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	args.client.AddNH(t, tunNH(2), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, tunNHG(1), map[uint64]uint64{tunNH(1): 1, tunNH(2): 3}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), tunNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), tunNHG(1), vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), tunNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), tunNHG(1), vrfEncapB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTraffic(t, args, weights, true)

	t.Log("Shutdown link carrying primary traffic")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)
	weights = []float64{0, 0, 0, 1}
	testTraffic(t, args, weights, false)
}

func TestTransitFrr(t *testing.T) {
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut, false)

	// Configure ATE
	otg := ondatra.ATE(t, "ate")
	topo := configureOTG(t, otg)

	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
		skip bool
	}{
		{
			name: "TestTransitTrafficNoPrefixInTransitVrf",
			desc: "TTT_02: Verify if there is no match for the tunnel IP in the TRANSIT_TE_VRF, then the packet is decaped and forwarded according to the routes in the DEFAULT VRF.",
			fn:   testTransitTrafficNoMatchInTransitVrf,
			skip: true,
		},
		{
			name: "TestTransitTrafficTransitNHGDownRepaiPathTunnelHandlesTraffic",
			desc: "TTT_03: If the NextHopGroup referenced by IPv4Entry in TRANSIT_TE_VRF is unviable (POPGate FRR behavior)and if the source IP is not 222.222.222.222, then verify this unviable tunnel is repaired by re-encapping the packet to a repair tunnel as specified in the REPAIR_TE_VRF.",
			fn:   testTransitTrafficNHGUnviableSendViaRepairTunnel,
		},
		{
			name: "TestTransitTrafficTransitNHGDownRepaiPathTunnelHandlesTraffic",
			desc: "TTT_04: If the NextHopGroup referenced by IPv4Entry in REPAIRED_TE_VRF is unviable (POPGate FRR behavior)and if the source IP is 222.222.222.222, then verify the packet is decapped and forwarded according to the BGP routes in the DEFAULT VRF. This is achieved by looking up the route for this packet in the REPAIRED_TE_VRF instead of the TRANSIT_TE_VRF.",
			fn:   testTransitTrafficRepairedNHGUnviableSrc222,
		},
		{
			name: "TestTransitFrrDecapSrc222",
			desc: "TFRR_01: Verify Tunnel traffic (6in4 and 4in4) arriving on WAN intefrace with source addresses 111.111.111.111 or 222.222.222.222 is decapped when matching entry exists in DECAP_TE_VRF.",
			fn:   testTransitTrafficDecapSrc222,
		},
		{
			name: "TestTransitFrrWithNonTETrafficOnWanInterface",
			desc: "TFRR_05: Verify TE disabled traffic arriving on the WAN interfaces, is routed according to the BGP routes in the DEFAULT VRF.",
			fn:   testTransitTrafficNonTETraffic,
		},
		{
			name: "TestTransitFrrWithNoMatchInDecapAndWithSrc111",
			desc: "TFRR_09: Verify TE traffic (6in4 and 4in4) arriving on the WAN or cluster facing interfaces is forwarded according to the rules in the TE_TRANSIT_VRF when there are no matching entries in the DECAP_TE_VRF and outer header IP_SrcAddr is of the format _._._.111.",
			fn:   testTransitFRRNoMatchDecapSrc111,
		},
		{
			name: "TestTransitFrrWithPrimaryNHGAndDecapEncapTunnelDown",
			desc: "TFRR_11: Verify for popgate (miss in DECAP_TE_VRF, hit in TE_TRANSIT_VRF) case, if the re-encap tunnels are also unviable, the packets are decapped and routed according to the BGP routes in the DEFAULT VRF.",
			fn:   testTransitFRRPrimaryNHGAndDecapEncapTunnelDown,
		},
		{
			name: "TestTransitFrrWithMatchInDecapThenEncapThenTransitPathPrimaryNHGAndDecapEncapTunnelDown",
			desc: "TFRR_12a: Verify for dcgate (hit in DECAP_TE_VRF) case, if the re-encap tunnels are also unviable, the packets are decapped and routed according to the BGP routes in the DEFAULT VRF.",
			fn:   testTransitFRRMachInDecapThenEncapedThenMachInTransitVrfThenPrimaryNHGAndDecapEncapTunnelDown,
		},
		{
			name: "TestTransitFrrRepairedPathWithSrc222",
			desc: "TFRR_12b: Verify Tunneled traffic that has already been repaired (identified by the source IP of 222.222.222.222) is forwarded according to the rules in the REPAIRED_TE_VRF.",
			fn:   testTransitFRRForRepairPathWithSrc222,
		},
	}

	for _, tc := range test {
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

		// Flush all existing AFT entries on the router
		c.FlushAll(t)

		tcArgs := &testArgs{
			client: &c,
			dut:    dut,
			ate:    otg,
			topo:   topo,
		}

		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.SkipNow()
			}
			t.Logf("Description: %s", tc.desc)
			tc.fn(t, tcArgs)
		})
	}
}

func testTransitTrafficNoMatchInTransitVrf(t *testing.T, args *testArgs) {
	configFallBackVrf(t, args.dut, []string{vrfTransit})
	configIPv4DefaultRoute(t, args.dut, cidr(tunnelDstIP1, 32), otgPort2.IPv4)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())

	configureVIP1(t, args)
	//backup NHG
	// if want to pass traffic as is (ipinip)
	//args.client.AddNH(t, baseNH(3), "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	// if want to decap and then pass the transit traffic
	args.client.AddNH(t, baseNH(3), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(3), map[uint64]uint64{baseNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, vipNH(1), vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTransitTraffic(t, args, weights, true)
	// note - traffic is expected to drop if transit vrf does not have prefix match
	t.Log("Delete tunnel prefix from transit vrf and verify traffic follows default route in the vrf")
	args.client.DeleteIPv4(t, cidr(tunnelDstIP1, 32), vrfTransit, fluent.InstalledInFIB)
	weights = []float64{1, 0, 0, 0}
	testTransitTraffic(t, args, weights, true)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via backup NHG")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)

	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights = []float64{0, 0, 0, 1}
	testTransitTraffic(t, args, weights, true)

}

// check with developer
func testTransitTrafficNHGUnviableSendViaRepairTunnel(t *testing.T, args *testArgs) {
	configFallBackVrf(t, args.dut, []string{vrfEncapA})
	configIPv4DefaultRoute(t, args.dut, cidr(tunnelDstIP1, 32), otgPort2.IPv4)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())

	configureVIP1(t, args)
	configureVIP3NHGWithTunnel(t, args)

	args.client.AddNH(t, vipNH(1), vipIP1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: tunNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{1, 0, 0, 0}
	testTransitTraffic(t, args, weights, true)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via repair path tunnel")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Subinterface(0).Enabled().Config(), true)

	configIPv4DefaultRoute(t, args.dut, cidr(tunnelDstIP3, 32), otgPort4.IPv4)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())

	weights = []float64{0, 0, 1, 0}
	testTransitTraffic(t, args, weights, true)
}

// check with developer
func testTransitTrafficRepairedNHGUnviableSrc222(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	faTransit.src = ipv4OuterSrc222
	oDst := faTransit.dst
	faTransit.dst = tunnelDstIP3
	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDst }()

	configureVIP3(t, args)

	args.client.AddNH(t, vipNH(3), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via repair path tunnel")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights := []float64{0, 0, 0, 1}
	testTransitTraffic(t, args, weights, true)

}

// TFRR_01
func testTransitTrafficDecapSrc222(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc222
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	args.client.AddNH(t, vipNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort2.IPv4, cidr(InnerV6DstIP, 128), otgPort2.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights := []float64{1, 0, 0, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapNoMatch, true)

}

// TFRR_05
func testTransitTrafficNonTETraffic(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrcIpInIp
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	args.client.AddNH(t, vipNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// non-te traffic should not get decapped
	configIPv4DefaultRoute(t, args.dut, cidr(tunnelDstIP1, 32), otgPort5.IPv4)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(tunnelDstIP1, 32)).Config())

	weights := []float64{0, 0, 0, 1}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapNoMatch, true)

}

// TFRR_09
func testTransitFRRNoMatchDecapSrc111(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc111
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// Dont program correct prefix in decap vrf
	configureVIP1(t, args)
	args.client.AddNH(t, vipNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(1), map[uint64]uint64{vipNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), vipNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// Install prefix in transit vrf
	configureVIP3(t, args)
	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(3), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	weights := []float64{0, 0, 1, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

}

// TFRR_10 covered using testTransitTrafficNHGUnviableSendViaRepairTunnel

// TFRR_11 -WIP -> check with de
func testTransitFRRPrimaryNHGAndDecapEncapTunnelDown(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc111
	faTransit.dst = tunnelDstIP1

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	configureVIP3NHGWithRepairTunnelHavingBackupDecapAction(t, args)
	configureVIP2(t, args)
	args.client.AddNH(t, vipNH(2), vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: tunNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// verify traffic passes through primary NHG
	weights := []float64{0, 1, 0, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via repair path tunnel")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 0, 1, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown repair tunnel path also and verify traffic passes through default vrf")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights = []float64{0, 0, 0, 1}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

}

// TFRR_12a, TFRR_13, TFRR_14
func testTransitFRRMachInDecapThenEncapedThenMachInTransitVrfThenPrimaryNHGAndDecapEncapTunnelDown(t *testing.T, args *testArgs) {
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

	// verify traffic passes through primary NHG
	weights := []float64{0, 1, 0, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown primary path for TransitVrf tunnel and verify traffic goes via repair path tunnel")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Subinterface(0).Enabled().Config(), true)

	weights = []float64{0, 0, 1, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown repair tunnel path also and verify traffic passes through default vrf")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	// configure default route
	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights = []float64{0, 0, 0, 1}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

}

// TFRR_12b
func testTransitFRRForRepairPathWithSrc222(t *testing.T, args *testArgs) {
	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc222
	faTransit.dst = tunnelDstIP3

	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// miss in decap vrf and src _222 should schedule traffic for repair vrf
	args.client.AddNH(t, decapNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, decapNHG(1), map[uint64]uint64{decapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	//args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// encap start
	args.client.AddNH(t, encapNH(1), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, encapNHG(1), map[uint64]uint64{encapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// inner hdr prefixes
	args.client.AddIPv4(t, cidr(innerV4DstIP, 32), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(InnerV6DstIP, 128), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// configure repaired path with backup
	// backup to repaired
	args.client.AddNH(t, baseNH(5), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(5), map[uint64]uint64{baseNH(5): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, "0.0.0.0/0", baseNHG(5), "DECAP", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	configureVIP3(t, args)
	// repaired path
	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(5)})
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// verify traffic passes through primary NHG
	weights := []float64{0, 0, 1, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	t.Log("Shutdown repair tunnel path also and verify traffic passes through default vrf")
	gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), false)
	defer gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Subinterface(0).Enabled().Config(), true)

	// configure default route
	configDefaultRoute(t, args.dut, cidr(innerV4DstIP, 32), otgPort5.IPv4, cidr(InnerV6DstIP, 128), otgPort5.IPv6)
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(innerV4DstIP, 32)).Config())
	defer gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(args.dut)).Static(cidr(InnerV6DstIP, 128)).Config())

	weights = []float64{0, 0, 0, 1}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)
}
