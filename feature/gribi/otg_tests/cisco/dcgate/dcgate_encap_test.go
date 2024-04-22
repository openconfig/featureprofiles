package dcgate_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/lemming/gnmi/oc"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestBasicEncap(t *testing.T) {
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut, true)
	defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config())

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

	// Flush all existing AFT entries on the router
	c.FlushAll(t)

	programEntries(t, dut, &c)

	test := []struct {
		name               string
		pattr              packetAttr
		flows              []gosnappi.Flow
		weights            []float64
		capturePorts       []string
		validateEncapRatio bool
		skip               bool
	}{
		{
			name:               fmt.Sprintf("Test1 IPv4 Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("Test1 IPv4 Traffic WCMP Encap dscp %d", dscpEncapB1),
			pattr:              packetAttr{dscp: dscpEncapB1, protocol: ipipProtocol},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4b1", dscpEncapB1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("Test2 IPv6 Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipv6ipProtocol},
			flows:              []gosnappi.Flow{fa6.getFlow("ipv6", "ip6a1", dscpEncapA1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("Test2 IPv6 Traffic WCMP Encap dscp %d", dscpEncapB1),
			pattr:              packetAttr{dscp: dscpEncapB1, protocol: ipv6ipProtocol},
			flows:              []gosnappi.Flow{fa6.getFlow("ipv6", "ip6b1", dscpEncapB1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:  fmt.Sprintf("Test3 IPinIP Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr: packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol},
			flows: []gosnappi.Flow{faIPinIP.getFlow("ipv4in4", "ip4in4a1", dscpEncapA1),
				faIPinIP.getFlow("ipv6in4", "ip6in4a1", dscpEncapA1),
			},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		{
			name:               fmt.Sprintf("No Match Dscp %d Traffic", dscpEncapNoMatch),
			pattr:              packetAttr{protocol: udpProtocol, dscp: dscpEncapNoMatch},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4nm", dscpEncapNoMatch)},
			weights:            noMatchWeight,
			capturePorts:       otgDstPorts[:1],
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("IPv4 No Prefix In Encap Vrf %d Traffic", dscpEncapA1),
			pattr:              packetAttr{protocol: udpProtocol, dscp: dscpEncapA1},
			flows:              []gosnappi.Flow{fa4NoPrefix.getFlow("ipv4", "ip4NoPrefixEncapVrf", dscpEncapA1)},
			weights:            noMatchWeight,
			capturePorts:       otgDstPorts[:1],
			validateEncapRatio: false,
			skip:               true,
		},
		{
			name:               fmt.Sprintf("IPv6 No Prefix In Encap Vrf %d Traffic", dscpEncapA1),
			pattr:              packetAttr{protocol: udpProtocol, dscp: dscpEncapA1},
			flows:              []gosnappi.Flow{fa6NoPrefix.getFlow("ipv6", "ip6NoPrefixEncapVrf", dscpEncapA1)},
			weights:            noMatchWeight,
			capturePorts:       otgDstPorts[:1],
			validateEncapRatio: false,
		},
	}

	tcArgs := &testArgs{
		client: &c,
		dut:    dut,
		ate:    otg,
		topo:   topo,
	}

	for _, tc := range test {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			if tc.skip {
				t.SkipNow()
			}
			if strings.Contains(tc.name, "No Match Dscp") {
				configDefaultRoute(t, dut, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), otgPort2.IPv4, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), otgPort2.IPv6)
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv4EntryPrefix, ipv4EntryPrefixLen)).Config())
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv6EntryPrefix, ipv6EntryPrefixLen)).Config())
			}
			if strings.Contains(tc.name, "No Prefix In Encap Vrf") {
				configDefaultIPStaticCli(t, dut, []string{vrfEncapA})
				defer unConfigDefaultIPStaticCli(t, dut, []string{vrfEncapA})
				configDefaultRoute(t, dut, cidr(ipv4PrefixDoesNotExistInEncapVrf, 32), otgPort2.IPv4, cidr(ipv6PrefixDoesNotExistInEncapVrf, 128), otgPort2.IPv6)
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv4PrefixDoesNotExistInEncapVrf, 32)).Config())
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv6PrefixDoesNotExistInEncapVrf, 128)).Config())
			}
			if otgMutliPortCaptureSupported {
				enableCapture(t, otg.OTG(), topo, tc.capturePorts)
				t.Log("Start capture and send traffic")
				sendTraffic(t, tcArgs, tc.flows, true)
				t.Log("Validate captured packet attributes")
				var tunCounter = validatePacketCapture(t, tcArgs, tc.capturePorts, &tc.pattr)
				if tc.validateEncapRatio {
					validateTunnelEncapRatio(t, tunCounter)
				}
				clearCapture(t, otg.OTG(), topo)
			} else {
				for _, port := range tc.capturePorts {
					enableCapture(t, otg.OTG(), topo, []string{port})
					t.Log("Start capture and send traffic")
					sendTraffic(t, tcArgs, tc.flows, true)
					t.Log("Validate captured packet attributes")
					var tunCounter = validatePacketCapture(t, tcArgs, []string{port}, &tc.pattr)
					if tc.validateEncapRatio {
						validateTunnelEncapRatio(t, tunCounter)
					}
					clearCapture(t, otg.OTG(), topo)
				}
			}
			t.Log("Validate traffic flows")
			validateTrafficFlows(t, tcArgs, tc.flows, false, true)
			t.Log("Validate hierarchical traffic distribution")
			validateTrafficDistribution(t, otg, tc.weights)
		})
	}
}

// programEntries pushes RIB entries on the DUT required for Encap functionality
func programEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client) {
	// push RIB entries
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		c.AddNH(t, nh10ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh11ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh100ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh101ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort5DummyIP.IPv4, Mac: magicMac})

	} else {
		c.AddNH(t, nh10ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port2").Name(), Mac: magicMac})
		c.AddNH(t, nh11ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port3").Name(), Mac: magicMac})
		c.AddNH(t, nh100ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port4").Name(), Mac: magicMac})
		c.AddNH(t, nh101ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port5").Name(), Mac: magicMac})
	}
	c.AddNHG(t, nhg2ID, map[uint64]uint64{nh10ID: 1, nh11ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(vipIP1, 32), nhg2ID, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	c.AddNHG(t, nhg3ID, map[uint64]uint64{nh100ID: 2, nh101ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(vipIP2, 32), nhg3ID, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	c.AddNH(t, nh1ID, vipIP1, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddNH(t, nh2ID, vipIP2, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 1, nh2ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(tunnelDstIP1, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(tunnelDstIP2, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	c.AddNH(t, nh201ID, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	c.AddNH(t, nh202ID, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	c.AddNHG(t, nhg10ID, map[uint64]uint64{nh201ID: 1, nh202ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
}
