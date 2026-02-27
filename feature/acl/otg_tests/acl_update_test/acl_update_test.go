package acl_update_test

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

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
)

const (
	aclNameV4             = "ACL-12-IPV4"
	aclNameV6             = "ACL-12-IPV6"
	port1                 = "port1"
	port2                 = "port2"
	l4SourcePort          = 1234
	l4PortRangeCount      = 500
	l4DestinationPort     = 2345
	ipv4ACLSrcAddr        = "192.168.100.1"
	ipv4ACLDstAddr        = "192.168.200.2"
	ipv6ACLSrcAddr        = "2001:db8:abcd:1::1"
	ipv6ACLDstAddr        = "2001:db8:abcd:2::2"
	ipv4UpdateACLSrcAddr  = "192.168.101.1"
	ipv6UpdateACLSrcAddr  = "2001:db8:abcd:2::1"
	trafficFrameSize      = 1500
	trafficRatePps        = 1000
	packetCount           = 10000
	maxLostTrafficTime    = 0.05
	trafficDuration       = 20 * time.Second
	staticRouteCount      = 2
	maxDroppedPackets     = uint32(trafficRatePps * maxLostTrafficTime)
	minPacketsToUpdateACL = 2000
	aclCounterTimeout     = 5 * time.Second

	expectPass = true
	expectDrop = false
)

var (
	// DUT ports
	dutPort1 = attrs.Attributes{
		Name:    port1,
		Desc:    "Dut port 1",
		IPv4:    "192.168.1.1",
		IPv4Len: 30,
		IPv6:    "2001:DB8::1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Name:    port2,
		Desc:    "Dut port 2",
		IPv4:    "192.168.1.5",
		IPv4Len: 30,
		IPv6:    "2001:DB8::5",
		IPv6Len: 126,
	}

	// ATE ports
	otgPort1 = attrs.Attributes{
		Desc:    "Otg port 1",
		Name:    port1,
		MAC:     "00:01:12:00:00:01",
		IPv4:    "192.168.1.2",
		IPv4Len: 30,
		IPv6:    "2001:DB8::2",
		IPv6Len: 126,
	}

	otgPort2 = attrs.Attributes{
		Desc:    "Otg port 2",
		Name:    port2,
		MAC:     "00:01:12:00:00:02",
		IPv4:    "192.168.1.6",
		IPv4Len: 30,
		IPv6:    "2001:DB8::6",
		IPv6Len: 126,
	}

	l4SourceRange      = fmt.Sprintf("%d %d", l4SourcePort-l4PortRangeCount, l4SourcePort+l4PortRangeCount-1)
	l4DestinationRange = fmt.Sprintf("%d %d", l4DestinationPort-l4PortRangeCount, l4DestinationPort+l4PortRangeCount-1)
	ipv4ACLSrc         = fmt.Sprintf("%s/32", ipv4ACLSrcAddr)
	ipv4ACLDst         = fmt.Sprintf("%s/32", ipv4ACLDstAddr)
	ipv6ACLSrc         = fmt.Sprintf("%s/128", ipv6ACLSrcAddr)
	ipv6ACLDst         = fmt.Sprintf("%s/128", ipv6ACLDstAddr)
	ipv4UpdateACLSrc   = fmt.Sprintf("%s/32", ipv4UpdateACLSrcAddr)
	ipv6UpdateACLSrc   = fmt.Sprintf("%s/128", ipv6UpdateACLSrcAddr)

	ipv4BaseTerms = []cfgplugins.AclTerm{
		{
			Description: "IPv4 TCP",
			SeqID:       10,
			IPSrc:       ipv4ACLSrc,
			IPDst:       ipv4ACLDst,
			L4SrcPort:   l4SourcePort,
			L4DstPort:   l4DestinationPort,
			Protocol:    cfgplugins.TCPProtocolNum,
			Log:         true,
		},
		{
			Description: "IPv4 UDP",
			SeqID:       20,
			IPSrc:       ipv4ACLSrc,
			IPDst:       ipv4ACLDst,
			L4SrcPort:   l4SourcePort,
			L4DstPort:   l4DestinationPort,
			Protocol:    cfgplugins.UDPProtocolNum,
			Log:         true,
		},
		{
			Description: "IPv4 ICMP",
			SeqID:       30,
			IPSrc:       ipv4ACLSrc,
			IPDst:       ipv4ACLDst,
			Protocol:    cfgplugins.ICMPv4ProtocolNum,
			Log:         true,
		},
		{
			Description:    "IPv4 TCP Range",
			SeqID:          40,
			IPSrc:          ipv4ACLSrc,
			IPDst:          ipv4ACLDst,
			L4SrcPortRange: l4SourceRange,
			L4DstPortRange: l4DestinationRange,
			Protocol:       cfgplugins.TCPProtocolNum,
			Log:            true,
		},
		{
			SeqID:       50,
			Description: "IPv4",
			IPSrc:       ipv4ACLSrc,
			IPDst:       ipv4ACLDst,
			Log:         true,
		},
	}

	ipv6BaseTerms = []cfgplugins.AclTerm{
		{
			Description: "IPv6 TCP",
			SeqID:       110,
			IPSrc:       ipv6ACLSrc,
			IPDst:       ipv6ACLDst,
			L4SrcPort:   l4SourcePort,
			L4DstPort:   l4DestinationPort,
			Protocol:    cfgplugins.TCPProtocolNum,
			Log:         true,
		},
		{
			Description: "IPv6 UDP",
			SeqID:       120,
			IPSrc:       ipv6ACLSrc,
			IPDst:       ipv6ACLDst,
			L4SrcPort:   l4SourcePort,
			L4DstPort:   l4DestinationPort,
			Protocol:    cfgplugins.UDPProtocolNum,
			Log:         true,
		},
		{
			Description: "IPv6 ICMP",
			SeqID:       130,
			IPSrc:       ipv6ACLSrc,
			IPDst:       ipv6ACLDst,
			Protocol:    cfgplugins.ICMPv6ProtocolNum,
			ICMPType:    int64(oc.Icmpv6Types_TYPE_ECHO_REQUEST),
			ICMPCode:    int64(oc.Icmpv6Types_CODE_ECHO_REQUEST_CODE),
			Log:         true,
		},
		{
			Description:    "IPv6 TCP Range",
			SeqID:          140,
			IPSrc:          ipv6ACLSrc,
			IPDst:          ipv6ACLDst,
			L4SrcPortRange: l4SourceRange,
			L4DstPortRange: l4DestinationRange,
			Protocol:       cfgplugins.TCPProtocolNum,
			Log:            true,
		},
		{
			SeqID:       150,
			Description: "IPv6",
			IPSrc:       ipv6ACLSrc,
			IPDst:       ipv6ACLDst,
			Log:         true,
		},
	}

	ipv4UpdateTerms = []cfgplugins.AclTerm{
		{SeqID: 910, Description: "src IP", IPSrc: ipv4UpdateACLSrc},
	}

	ipv6UpdateTerms = []cfgplugins.AclTerm{
		{SeqID: 920, Description: "src IP", IPSrc: ipv6UpdateACLSrc},
	}

	entryCounters = map[uint32]uint64{}
)

type testCase struct {
	name            string
	aclParams       []cfgplugins.AclParams
	aclUpdateParams map[string]cfgplugins.AclParams
}

type flowConfig struct {
	name        string
	ipType      string
	trafficItem cfgplugins.AclTerm
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestACLUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	dp1 := dut.Port(t, port1)
	dp2 := dut.Port(t, port2)

	configureDUT(t, dut)

	ap1 := ate.Port(t, port1)
	ap2 := ate.Port(t, port2)

	otgPort1.AddToOTG(top, ap1, &dutPort1)
	otgPort2.AddToOTG(top, ap2, &dutPort2)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, cfgplugins.IPv4)
	otgutils.WaitForARP(t, ate.OTG(), top, cfgplugins.IPv6)

	testCases := []testCase{
		{
			name: "TC1",
			aclParams: []cfgplugins.AclParams{
				{
					Name:          aclNameV4,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV4,
					Terms:         processACLTermsPermit(ipv4BaseTerms, expectDrop),
					DefaultPermit: expectPass,
					Intf:          dp1.Name(),
					Ingress:       true,
				},
				{
					Name:          aclNameV6,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV6,
					Terms:         processACLTermsPermit(ipv6BaseTerms, expectDrop),
					DefaultPermit: expectPass,
					Intf:          dp1.Name(),
					Ingress:       true,
				},
			},
		},
		{
			name: "TC2",
			aclParams: []cfgplugins.AclParams{
				{
					Name:          aclNameV4,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV4,
					Terms:         processACLTermsPermit(ipv4BaseTerms, expectPass),
					DefaultPermit: expectDrop,
					Intf:          dp1.Name(),
					Ingress:       true,
				},
				{
					Name:          aclNameV6,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV6,
					Terms:         processACLTermsPermit(ipv6BaseTerms, expectPass),
					DefaultPermit: expectDrop,
					Intf:          dp1.Name(),
					Ingress:       true,
				},
			},

			aclUpdateParams: map[string]cfgplugins.AclParams{
				aclNameV4: {
					Name:          aclNameV4,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV4,
					Terms:         processACLTermsPermit(ipv4UpdateTerms, expectPass),
					DefaultPermit: expectDrop,
					Intf:          dp1.Name(),
					Ingress:       true,
					Update:        true,
				},
				aclNameV6: {
					Name:          aclNameV6,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV6,
					Terms:         processACLTermsPermit(ipv6UpdateTerms, expectPass),
					DefaultPermit: expectDrop,
					Intf:          dp1.Name(),
					Ingress:       true,
					Update:        true,
				},
			},
		},
		{
			name: "TC3.1",
			aclParams: []cfgplugins.AclParams{
				{
					Name:          aclNameV4,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV4,
					Terms:         processACLTermsPermit(ipv4BaseTerms, expectDrop),
					DefaultPermit: expectPass,
					Intf:          dp2.Name(),
					Ingress:       false,
				},
				{
					Name:          aclNameV6,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV6,
					Terms:         processACLTermsPermit(ipv6BaseTerms, expectDrop),
					DefaultPermit: expectPass,
					Intf:          dp2.Name(),
					Ingress:       false,
				},
			},
		},
		{
			name: "TC3.2",
			aclParams: []cfgplugins.AclParams{
				{
					Name:          aclNameV4,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV4,
					Terms:         processACLTermsPermit(ipv4BaseTerms, expectPass),
					DefaultPermit: expectDrop,
					Intf:          dp2.Name(),
					Ingress:       false,
				},
				{
					Name:          aclNameV6,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV6,
					Terms:         processACLTermsPermit(ipv6BaseTerms, expectPass),
					DefaultPermit: expectDrop,
					Intf:          dp2.Name(),
					Ingress:       false,
				},
			},
			aclUpdateParams: map[string]cfgplugins.AclParams{
				aclNameV4: {
					Name:          aclNameV4,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV4,
					Terms:         processACLTermsPermit(ipv4UpdateTerms, expectPass),
					DefaultPermit: expectDrop,
					Intf:          dp2.Name(),
					Ingress:       false,
					Update:        true,
				},
				aclNameV6: {
					Name:          aclNameV6,
					ACLType:       oc.Acl_ACL_TYPE_ACL_IPV6,
					Terms:         processACLTermsPermit(ipv6UpdateTerms, expectPass),
					DefaultPermit: expectDrop,
					Intf:          dp2.Name(),
					Ingress:       false,
					Update:        true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := runTest(t, dut, ate, top, tc); err != nil {
				t.Errorf("test %s failed:\n%v", tc.name, err)
			}
		})
	}
}

func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	for index := range staticRouteCount {
		routeSrcV4, err := cfgplugins.IncrementIP(ipv4ACLDstAddr, index)
		if err != "" {
			t.Fatalf("Failed to increment IP %s: %v", ipv4ACLDstAddr, err)
		}
		routeSrcV6, err := cfgplugins.IncrementIP(ipv6ACLDstAddr, index)
		if err != "" {
			t.Fatalf("Failed to increment IP %s: %v", ipv6ACLDstAddr, err)
		}

		t.Logf("creating static route for %s", routeSrcV4)
		newStaticRoute(t, dut, batch, fmt.Sprintf("%s/32", routeSrcV4), otgPort2.IPv4, strconv.Itoa(index+1))

		t.Logf("creating static route for %s", routeSrcV6)
		newStaticRoute(t, dut, batch, fmt.Sprintf("%s/128", routeSrcV6), otgPort2.IPv6, strconv.Itoa(index+1))
	}
}

func newStaticRoute(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, prefix string, nexthop string, index string) {

	if nexthop == "Null0" {
		nexthop = "DROP"
	}
	routeCfg := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			index: oc.UnionString(nexthop),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(batch, routeCfg, dut); err != nil {
		t.Fatalf("Failed to configure static route: %v", err)
	}
}

func processACLTermsPermit(terms []cfgplugins.AclTerm, allow bool) []cfgplugins.AclTerm {
	newTerms := make([]cfgplugins.AclTerm, len(terms))
	copy(newTerms, terms)
	for i := range newTerms {
		newTerms[i].Permit = allow
	}
	return newTerms
}

func createFlowConfig(t *testing.T, ipType string, terms []cfgplugins.AclTerm, shouldMatchTerm, expectPass bool) []flowConfig {
	flows := []flowConfig{}

	for _, term := range terms {
		fc := flowConfig{
			ipType: ipType,
		}
		if term.IPSrc != "" {
			srcAddressIP, _, err := net.ParseCIDR(term.IPSrc)
			if err != nil {
				t.Fatalf("Failed to parse CIDR %s: %v", term.IPSrc, err)
			}

			srcAddress := srcAddressIP.String()
			if !shouldMatchTerm {
				srcAddressInc, err := cfgplugins.IncrementIP(srcAddress, 1)
				if err != "" {
					t.Fatalf("Failed to increment IP %s: %v", srcAddress, err)
				}
				srcAddress = srcAddressInc
			}
			fc.trafficItem.IPSrc = srcAddress
		} else {
			switch ipType {
			case cfgplugins.IPv4:
				fc.trafficItem.IPSrc = otgPort1.IPv4
			case cfgplugins.IPv6:
				fc.trafficItem.IPSrc = otgPort1.IPv6
			}
		}
		if term.IPDst != "" {
			dstAddressIP, _, err := net.ParseCIDR(term.IPDst)
			if err != nil {
				t.Fatalf("Failed to parse CIDR %s: %v", term.IPDst, err)
			}

			dstAddress := dstAddressIP.String()
			if !shouldMatchTerm {
				dstAddressInc, err := cfgplugins.IncrementIP(dstAddress, 1)
				if err != "" {
					t.Fatalf("Failed to increment IP %s: %v", dstAddress, err)
				}
				dstAddress = dstAddressInc
			}
			fc.trafficItem.IPDst = dstAddress
		} else {
			switch ipType {
			case cfgplugins.IPv4:
				fc.trafficItem.IPDst = otgPort2.IPv4
			case cfgplugins.IPv6:
				fc.trafficItem.IPDst = otgPort2.IPv6
			}
		}
		if term.L4SrcPort != 0 {
			if !shouldMatchTerm {
				fc.trafficItem.L4SrcPort = term.L4SrcPort + l4PortRangeCount + 1
			} else {
				fc.trafficItem.L4SrcPort = term.L4SrcPort
			}
		} else if term.L4SrcPortRange != "" {
			portString := term.L4SrcPortRange[strings.LastIndex(term.L4SrcPortRange, " ")+1:]
			endPort, err := strconv.ParseUint(portString, 10, 0)
			if err != nil {
				t.Fatalf("Failed to parse L4SrcPortRange %s: %v", term.L4SrcPortRange, err)
			} else {
				if !shouldMatchTerm {
					fc.trafficItem.L4SrcPort = uint32(endPort) + 1
				} else {
					fc.trafficItem.L4SrcPort = uint32(endPort) - 1
				}
			}
		}
		if term.L4DstPort != 0 {
			if !shouldMatchTerm {
				fc.trafficItem.L4DstPort = term.L4DstPort + l4PortRangeCount + 1
			} else {
				fc.trafficItem.L4DstPort = term.L4DstPort
			}
		} else if term.L4DstPortRange != "" {
			portString := term.L4DstPortRange[strings.LastIndex(term.L4DstPortRange, " ")+1:]
			endPort, err := strconv.ParseUint(portString, 10, 0)
			if err != nil {
				t.Fatalf("Failed to parse L4DstPortRange %s: %v", term.L4DstPortRange, err)
			} else {
				if !shouldMatchTerm {
					fc.trafficItem.L4DstPort = uint32(endPort) + 1
				} else {
					fc.trafficItem.L4DstPort = uint32(endPort) - 1
				}
			}
		}

		fc.trafficItem.Protocol = term.Protocol
		if term.Protocol == cfgplugins.ICMPv4ProtocolNum || term.Protocol == cfgplugins.ICMPv6ProtocolNum {
			fc.trafficItem.ICMPType = term.ICMPType
			fc.trafficItem.ICMPCode = term.ICMPCode
		}

		var match, pass string
		if shouldMatchTerm {
			match = "match"
		} else {
			match = "not-match"
		}
		if expectPass {
			pass = "pass"
		} else {
			pass = "drop"
		}

		fc.name = fmt.Sprintf("flow-%s-%s-%s", strings.ReplaceAll(term.Description, " ", "-"), match, pass)
		fc.trafficItem.SeqID = term.SeqID
		flows = append(flows, fc)
	}

	return flows
}

func verifyFlowStatistics(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string, expectPass bool, expectedDroppedPackets uint32) error {
	otg := ate.OTG()
	var validationErrors []error

	otgutils.LogFlowMetrics(t, otg, config)
	otgutils.LogPortMetrics(t, otg, config)

	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	if *flowMetrics.Counters.OutPkts != packetCount {
		validationErrors = append(validationErrors, fmt.Errorf("flow %s sent %d packets, expected %d packets", flowName, *flowMetrics.Counters.OutPkts, packetCount))
	}

	if expectPass {
		if *flowMetrics.Counters.InPkts < uint64(packetCount-expectedDroppedPackets) {
			validationErrors = append(validationErrors, fmt.Errorf("flow %s received %d packets, expected %d packets, maximum dropped packets %d", flowName, *flowMetrics.Counters.InPkts, packetCount, expectedDroppedPackets))
		}
	} else if *flowMetrics.Counters.InPkts != 0 {
		validationErrors = append(validationErrors, fmt.Errorf("flow %s received %d packets, expected 0 packets", flowName, *flowMetrics.Counters.InPkts))
	}

	if len(validationErrors) > 0 {
		err := errors.Join(validationErrors...)
		t.Log(err.Error())
		return err
	}

	if expectedDroppedPackets > 0 {
		t.Logf("Flow %s sent %d packets, received %d packets with expected dropped packets %d", flowName, *flowMetrics.Counters.OutPkts, *flowMetrics.Counters.InPkts, expectedDroppedPackets)
	} else {
		t.Logf("Flow %s sent %d packets, received %d packets as expected", flowName, *flowMetrics.Counters.OutPkts, *flowMetrics.Counters.InPkts)
	}
	return nil
}

func verifyACLCounters(t *testing.T, dut *ondatra.DUTDevice, fc flowConfig, expectPass bool, aclParams cfgplugins.AclParams, maxDroppedPackets uint32) error {
	t.Helper()

	entryID := fc.trafficItem.SeqID
	aclSetPath := gnmi.OC().Acl().AclSet(aclParams.Name, aclParams.ACLType)

	t.Logf("Verifying ACL counters for ACL %s entry %d", aclParams.Name, entryID)
	var entry *oc.Acl_AclSet_AclEntry
	check := func(val *ygnmi.Value[*oc.Acl_AclSet_AclEntry]) bool {
		e, present := val.Val()
		return present && e != nil
	}
	val, ok := gnmi.Watch(t, dut, aclSetPath.AclEntry(entryID).State(), aclCounterTimeout, check).Await(t)
	if !ok {
		return fmt.Errorf("ACL entry %d not found in ACL %s after %v seconds", entryID, aclParams.Name, aclCounterTimeout)
	}
	entry, _ = val.Val()

	var expectedPackets uint64
	matched := entry.GetMatchedPackets()
	previouslyMatched := entryCounters[entryID]
	t.Logf("ACL entry %d: matched=%d", entryID, matched)

	switch {
	case expectPass && aclParams.DefaultPermit, !expectPass && !aclParams.DefaultPermit:
		if entryID == cfgplugins.DefaultEntryID {
			expectedPackets = uint64(packetCount - maxDroppedPackets)
		} else {
			expectedPackets = 0
		}
	case expectPass && !aclParams.DefaultPermit, !expectPass && aclParams.DefaultPermit:
		if entryID == cfgplugins.DefaultEntryID {
			expectedPackets = 0
		} else {
			expectedPackets = uint64(packetCount - maxDroppedPackets)
		}
	}

	entryCounters[entryID] = matched
	message := fmt.Sprintf("expected >= %d matched packets for ACL entry %d, got %d", expectedPackets, entryID, matched-previouslyMatched)
	if matched-previouslyMatched < expectedPackets {
		err := fmt.Errorf("ACL validation failed for flow %s: %s", fc.name, message)
		t.Log(err.Error())
		return err
	}

	t.Log(message)
	return nil
}

func sendAndVerifyTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config, aclParams cfgplugins.AclParams, flowConfig flowConfig, expectPass bool, maxDroppedPackets uint32) error {
	var validationErrors []error

	otg := ate.OTG()
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, config, flowConfig.ipType)

	otg.StartTraffic(t)

	if !aclParams.Update {
		waitForTraffic(t, otg, flowConfig.name, trafficDuration)
	} else {
		waitForPackets(t, otg, flowConfig.name, minPacketsToUpdateACL, trafficDuration)
		updateBatch := &gnmi.SetBatch{}
		cfgplugins.ConfigureACL(t, dut, updateBatch, aclParams)
		updateBatch.Set(t, dut)
		t.Log("Successfully updated ACL")
		waitForTraffic(t, otg, flowConfig.name, trafficDuration)
	}
	otg.StopProtocols(t)

	if err := verifyFlowStatistics(t, ate, config, flowConfig.name, expectPass, maxDroppedPackets); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if !deviations.SkipACLCountersVerificationDuringUpdate(dut) {
		if err := verifyACLCounters(t, dut, flowConfig, expectPass, aclParams, maxDroppedPackets); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	return errors.Join(validationErrors...)
}

func waitForPackets(t *testing.T, otg *otg.OTG, flowName string, minPkts uint64, timeout time.Duration) {
	pktsPath := gnmi.OTG().Flow(flowName).Counters().InPkts().State()
	check := func(val *ygnmi.Value[uint64]) bool {
		v, present := val.Val()
		return present && v >= minPkts
	}
	if _, ok := gnmi.Watch(t, otg, pktsPath, timeout, check).Await(t); !ok {
		t.Errorf("did not receive %d packets on flow %s within %v", minPkts, flowName, timeout)
	} else {
		t.Logf("Received at least %d packets on flow %s", minPkts, flowName)
	}
}

func validateTrafficPerACLConfig(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, tc testCase, aclParams cfgplugins.AclParams) error {
	var testErrors []error

	t.Logf("Configuring ACL %s for test case %s", aclParams.Name, tc.name)
	batch := &gnmi.SetBatch{}
	cfgplugins.ConfigureACL(t, dut, batch, aclParams)
	batch.Set(t, dut)
	defer cleanupACL(t, dut, aclParams)

	if deviations.ACLCountersEnableOCUnsupported(dut) {
		cfgplugins.EnableACLCountersFromCLI(t, dut, aclParams)
	}

	if aclParams.ACLType == oc.Acl_ACL_TYPE_ACL_IPV6 && deviations.ACLIcmpTypeCodeConfigurationUnsupported(dut) {
		cfgplugins.ConfigureNDPRulesFromCLI(t, dut, aclParams)
	}

	flowConfigMap := make(map[bool][]flowConfig)
	for _, expectTraffic := range []bool{expectPass, expectDrop} {
		flowConfigMap[expectTraffic] = createFlowConfig(t, ipTypeForACL(aclParams.ACLType), aclParams.Terms, aclParams.DefaultPermit != expectTraffic, expectTraffic)
	}

	for shouldPass, flows := range flowConfigMap {
		for _, flowConfig := range flows {
			t.Logf("Configuring %s expecting pass: %t", flowConfig.name, shouldPass)
			if err := configureFlow(t, top, flowConfig); err != nil {
				testErrors = append(testErrors, err)
				continue
			}
			if err := sendAndVerifyTraffic(t, dut, ate, top, aclParams, flowConfig, shouldPass, 0); err != nil {
				testErrors = append(testErrors, err)
				continue
			}
		}
	}

	aclUpdateParams, found := tc.aclUpdateParams[aclParams.Name]
	if !found {
		return errors.Join(testErrors...)
	}

	passingFlowConfig := flowConfigMap[expectPass][0]
	passingFlowConfig.name += "-update"
	t.Logf("Configuring %s and updating ACL on the fly", passingFlowConfig.name)
	if err := configureFlow(t, top, passingFlowConfig); err != nil {
		testErrors = append(testErrors, err)
	} else {
		if err := sendAndVerifyTraffic(t, dut, ate, top, aclUpdateParams, passingFlowConfig, expectPass, maxDroppedPackets); err != nil {
			testErrors = append(testErrors, err)
		}
	}

	return errors.Join(testErrors...)
}

func runTest(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, tc testCase) error {
	var testErrors []error
	for _, aclParams := range tc.aclParams {
		if err := validateTrafficPerACLConfig(t, dut, ate, top, tc, aclParams); err != nil {
			testErrors = append(testErrors, err)
		}
	}
	return errors.Join(testErrors...)
}

func listAclCounters(t *testing.T, dut *ondatra.DUTDevice, aclParams cfgplugins.AclParams) {
	aclSetPath := gnmi.OC().Acl().AclSet(aclParams.Name, aclParams.ACLType)
	entries := gnmi.GetAll(t, dut, aclSetPath.AclEntryAny().State())
	if len(entries) == 0 {
		t.Logf("No ACL entries found for ACL set %s (type %v)", aclParams.Name, aclParams.ACLType)
		return
	}
	t.Logf("ACL Counters for ACL set %s (type %v):", aclParams.Name, aclParams.ACLType)
	for _, entry := range entries {
		t.Logf("ACL Entry %d: matched = %d", entry.GetSequenceId(), entry.GetMatchedPackets())
	}
}

func cleanupACL(t *testing.T, dut *ondatra.DUTDevice, aclParams cfgplugins.AclParams) {
	listAclCounters(t, dut, aclParams)
	deleteBatch := &gnmi.SetBatch{}
	cfgplugins.DeleteACL(t, deleteBatch, aclParams)
	deleteBatch.Set(t, dut)
	clear(entryCounters)
}

func ipTypeForACL(aclType oc.E_Acl_ACL_TYPE) string {
	switch aclType {
	case oc.Acl_ACL_TYPE_ACL_IPV4:
		return cfgplugins.IPv4
	case oc.Acl_ACL_TYPE_ACL_IPV6:
		return cfgplugins.IPv6
	}
	return ""
}

func configureFlow(t *testing.T, config gosnappi.Config, fc flowConfig) error {
	t.Helper()
	config.Flows().Clear()
	flow := config.Flows().Add().SetName(fc.name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", port1, fc.ipType)}).SetRxNames([]string{fmt.Sprintf("%s.%s", port2, fc.ipType)})
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetPps(trafficRatePps)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(packetCount))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(otgPort1.MAC)
	if fc.trafficItem.IPDst == "" || fc.trafficItem.IPSrc == "" {
		return fmt.Errorf("missing source or destination IP for flow")
	}

	switch fc.ipType {
	case cfgplugins.IPv4:
		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(fc.trafficItem.IPSrc)
		ipv4.Dst().SetValue(fc.trafficItem.IPDst)
	case cfgplugins.IPv6:
		ipv6 := flow.Packet().Add().Ipv6()
		ipv6.Src().SetValue(fc.trafficItem.IPSrc)
		ipv6.Dst().SetValue(fc.trafficItem.IPDst)
	default:
		return fmt.Errorf("invalid traffic type %s", fc.ipType)
	}

	switch fc.trafficItem.Protocol {
	case cfgplugins.TCPProtocolNum:
		if fc.trafficItem.L4SrcPort == 0 || fc.trafficItem.L4DstPort == 0 {
			return fmt.Errorf("missing L4 ports for TCP protocol")
		}
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().SetValue(fc.trafficItem.L4SrcPort)
		tcp.DstPort().SetValue(fc.trafficItem.L4DstPort)
	case cfgplugins.UDPProtocolNum:
		if fc.trafficItem.L4SrcPort == 0 || fc.trafficItem.L4DstPort == 0 {
			return fmt.Errorf("missing L4 ports for UDP protocol")
		}
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().SetValue(fc.trafficItem.L4SrcPort)
		udp.DstPort().SetValue(fc.trafficItem.L4DstPort)
	case cfgplugins.ICMPv4ProtocolNum:
		if fc.ipType != cfgplugins.IPv4 {
			return fmt.Errorf("ICMPv4 protocol specified for non-IPv4 traffic")
		}
		icmp := flow.Packet().Add().Icmp()
		icmp.Echo().SetType(gosnappi.NewPatternFlowIcmpEchoType().SetValue(uint32(fc.trafficItem.ICMPType)))
		icmp.Echo().SetCode(gosnappi.NewPatternFlowIcmpEchoCode().SetValue(uint32(fc.trafficItem.ICMPCode)))
	case cfgplugins.ICMPv6ProtocolNum:
		if fc.ipType != cfgplugins.IPv6 {
			return fmt.Errorf("ICMPv6 protocol specified for non-IPv6 traffic")
		}
		icmpv6 := flow.Packet().Add().Icmpv6()
		icmpv6.Echo().SetType(gosnappi.NewPatternFlowIcmpv6EchoType().SetValue(uint32(fc.trafficItem.ICMPType)))
		icmpv6.Echo().SetCode(gosnappi.NewPatternFlowIcmpv6EchoCode().SetValue(uint32(fc.trafficItem.ICMPCode)))
	}

	return nil
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, port1)
	p2 := dut.Port(t, port2)

	batch := &gnmi.SetBatch{}
	t.Log("Configuring Interfaces")
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))

	t.Log("Configuring Static Routes")
	configureStaticRoutes(t, dut, batch)
	batch.Set(t, dut)

	t.Logf("Configuring Hardware Init")
	configureHardwareInit(t, dut)
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureACLCounters)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	checkState := func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}
	if _, ok := gnmi.Watch(t, otg, transmitPath, timeout, checkState).Await(t); !ok {
		t.Errorf("traffic for flow %s did not stop within the timeout of %v", flowName, timeout)
		return
	}

	t.Logf("Traffic for flow %s has stopped", flowName)
}
