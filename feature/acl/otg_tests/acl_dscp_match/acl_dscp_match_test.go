package acl_dscp_match_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
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
	aclNamev4        = "ip_dscp_match"
	aclNamev6        = "ipv6_dscp_match"
	trafficTimeout   = 10 * time.Second
	IPv4             = "IPv4"
	IPv6             = "IPv6"
	trafficFrameSize = 512
	trafficRatePps   = 1000
	noOfPackets      = 5000

	ipProtoTCP           = 6
	AF11          uint32 = 10
	AF21          uint32 = 18
	AF31          uint32 = 26
	AF41          uint32 = 34
	ipProtoICMPv6        = 58
	ipv4PrefixLen        = 32
	ipv6PrefixLen        = 128
)

var (
	// DUT ports
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "Dut port 1",
		IPv4:    "192.168.1.1",
		IPv4Len: 30,
		IPv6:    "2001:DB8::1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "Dut port 2",
		IPv4:    "192.168.1.5",
		IPv4Len: 30,
		IPv6:    "2001:DB8::5",
		IPv6Len: 126,
	}

	// ATE ports
	otgPort1 = attrs.Attributes{
		Desc:    "Otg port 1",
		Name:    "port1",
		MAC:     "00:01:12:00:00:01",
		IPv4:    "192.168.1.2",
		IPv4Len: 30,
		IPv6:    "2001:DB8::2",
		IPv6Len: 126,
	}

	otgPort2 = attrs.Attributes{
		Desc:    "Otg port 2",
		Name:    "port2",
		MAC:     "00:01:12:00:00:02",
		IPv4:    "192.168.1.6",
		IPv4Len: 30,
		IPv6:    "2001:DB8::6",
		IPv6Len: 126,
	}

	atePortPair = []attrs.Attributes{otgPort1, otgPort2}
)

type testCase struct {
	name           string
	ipType         string
	srcDstPortPair []uint32
	dscpTestValues map[uint32]bool
	flowName       string
	aclDscpValue   uint32
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestAclDscpMatch(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	configureDUT(t, dut)

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	otgPort1.AddToOTG(top, ap1, &dutPort1)
	otgPort2.AddToOTG(top, ap2, &dutPort2)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	testCases := []testCase{
		{
			name:           "ACL-1.1.1 - IPv4 Address and DSCP",
			ipType:         IPv4,
			flowName:       "ACL-1.1.1",
			dscpTestValues: map[uint32]bool{AF11: true, AF21: false},
			aclDscpValue:   AF11,
		},
		{
			name:           "ACL-1.1.2 - IPv4 Address, TCP src/dst ports and DSCP",
			ipType:         IPv4,
			srcDstPortPair: []uint32{49256, 49512},
			dscpTestValues: map[uint32]bool{AF21: true, AF31: false},
			flowName:       "ACL-1.1.2",
			aclDscpValue:   AF21,
		},
		{
			name:           "ACL-1.1.3 - IPv6 Address and DSCP",
			ipType:         IPv6,
			flowName:       "ACL-1.1.3",
			dscpTestValues: map[uint32]bool{AF31: true, AF21: false},
			aclDscpValue:   AF31,
		},
		{
			name:           "ACL-1.1.4 - IPv6 Address, TCP src/dst ports and DSCP",
			ipType:         IPv6,
			srcDstPortPair: []uint32{49256, 49512},
			dscpTestValues: map[uint32]bool{AF41: true, AF11: false},
			flowName:       "ACL-1.1.4",
			aclDscpValue:   AF41,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, tc, dut, ate, &top)
		})
	}
}

func runTest(t *testing.T, tc testCase, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config *gosnappi.Config) {
	otg := ate.OTG()
	t.Logf("Configuring ACLs for testcase %s for DSCP %d", tc.name, tc.aclDscpValue)
	configureAcl(t, dut, tc)

	for dscp, expectPass := range tc.dscpTestValues {
		if expectPass {
			t.Logf("Expecting traffic to pass for DSCP %d", dscp)
		} else {
			t.Logf("Expecting traffic to be dropped for DSCP %d", dscp)
		}
		configureFlows(t, config, tc, dscp)
		otg.PushConfig(t, *config)
		otg.StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), *config, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), *config, "IPv6")
		otg.StartTraffic(t)
		waitForTraffic(t, otg, tc.flowName, trafficTimeout)

		otgutils.LogFlowMetrics(t, otg, *config)
		otgutils.LogPortMetrics(t, otg, *config)

		flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(tc.flowName).State())
		if *flowMetrics.Counters.OutPkts == 0 {
			t.Errorf("No packets transmitted")
		}
		if expectPass {
			message := fmt.Sprintf("Expected %d packets, got %d", noOfPackets, *flowMetrics.Counters.InPkts)
			if *flowMetrics.Counters.InPkts != noOfPackets {
				t.Error(message)
			} else {
				t.Log(message)
			}
		} else {
			message := fmt.Sprintf("Expected 0 packets, got %d", *flowMetrics.Counters.InPkts)
			if *flowMetrics.Counters.InPkts != 0 {
				t.Error(message)
			} else {
				t.Log(message)
			}
		}
	}
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}).Await(t)

	if !ok {
		t.Errorf("Traffic for flow %s did not stop within the timeout of %d", flowName, timeout)
	} else {
		t.Logf("Traffic for flow %s has stopped", flowName)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	t.Logf("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1)
	configureDUTPort(t, dut, &dutPort2, dp2)
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	i.Description = ygot.String(attrs.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		i4.Enabled = ygot.Bool(true)
	}
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		i6.Enabled = ygot.Bool(true)
	}

	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		t.Logf("DUT %s %s %s requires explicit interface in default VRF deviation ", dut.Vendor(), dut.Model(), dut.Version())
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p)
	}
}

func configureFlows(t *testing.T, config *gosnappi.Config, tc testCase, dscp uint32) {
	(*config).Flows().Clear()
	flow := (*config).Flows().Add().SetName(tc.flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", atePortPair[0].Name, tc.ipType)}).SetRxNames([]string{fmt.Sprintf("%s.%s", atePortPair[1].Name, tc.ipType)})
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetPps(trafficRatePps)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(noOfPackets))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePortPair[0].MAC)

	switch tc.ipType {
	case IPv4:
		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(atePortPair[0].IPv4)
		ipv4.Dst().SetValue(atePortPair[1].IPv4)
		ipv4.Priority().Dscp().Phb().SetValue(dscp)
	case IPv6:
		ipv6 := flow.Packet().Add().Ipv6()
		ipv6.Src().SetValue(atePortPair[0].IPv6)
		ipv6.Dst().SetValue(atePortPair[1].IPv6)
		ipv6.TrafficClass().SetValue(dscp << 2)
	default:
		t.Errorf("Invalid traffic type %s", tc.ipType)
	}
	if len(tc.srcDstPortPair) == 2 {
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().SetValue(tc.srcDstPortPair[0])
		tcp.DstPort().SetValue(tc.srcDstPortPair[1])
	}
}

func configureAclInterface(t *testing.T, dut *ondatra.DUTDevice, acl *oc.Acl, tc testCase) {
	ifName := dut.Port(t, "port1").Name()

	existingIface := gnmi.Get(t, dut, gnmi.OC().Interface(ifName).State())

	iFace := acl.GetOrCreateInterface(ifName)
	if tc.ipType == IPv4 {
		iFace.GetOrCreateIngressAclSet(aclNamev4, oc.Acl_ACL_TYPE_ACL_IPV4)
	} else {
		iFace.GetOrCreateIngressAclSet(aclNamev6, oc.Acl_ACL_TYPE_ACL_IPV6)
	}

	iFace.GetOrCreateInterfaceRef().Interface = existingIface.Name
	iFace.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	gnmi.Replace(t, dut, gnmi.OC().Acl().Interface(ifName).Config(), iFace)
}

func configureAcl(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	t.Helper()
	d := &oc.Root{}
	acl := d.GetOrCreateAcl()

	if tc.ipType == IPv4 {
		// All IPv4 ACL logic goes here.
		aclSetv4 := acl.GetOrCreateAclSet(aclNamev4, oc.Acl_ACL_TYPE_ACL_IPV4)
		aclAcceptEntryv4 := aclSetv4.GetOrCreateAclEntry(10)

		ipv4Acl := aclAcceptEntryv4.GetOrCreateIpv4()
		ipv4Acl.SourceAddress = ygot.String(fmt.Sprintf("%s/%d", otgPort1.IPv4, ipv4PrefixLen))
		ipv4Acl.DestinationAddress = ygot.String(fmt.Sprintf("%s/%d", otgPort2.IPv4, ipv4PrefixLen))
		ipv4Acl.SetDscp(uint8(tc.aclDscpValue))

		if len(tc.srcDstPortPair) == 2 {
			aclAcceptEntryv4.GetOrCreateIpv4().SetProtocol(oc.UnionUint8(ipProtoTCP))
			aclTransport := aclAcceptEntryv4.GetOrCreateTransport()
			aclTransport.SetSourcePort(oc.UnionUint16(tc.srcDstPortPair[0]))
			aclTransport.SetDestinationPort(oc.UnionUint16(tc.srcDstPortPair[1]))
		}
		aclAcceptEntryv4.GetOrCreateActions().SetForwardingAction(oc.Acl_FORWARDING_ACTION_ACCEPT)

		// Add a deny all rule at the end for IPv4
		aclDropEntryv4 := aclSetv4.GetOrCreateAclEntry(30)
		aclDropEntryv4.Description = ygot.String("dscp mismatch drop v4")
		aclDropEntryv4.GetOrCreateActions().SetForwardingAction(oc.Acl_FORWARDING_ACTION_DROP)

		gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclNamev4, aclSetv4.GetType()).Config(), aclSetv4)

	} else { // This block handles IPv6
		// All IPv6 ACL logic goes here.
		aclSetv6 := acl.GetOrCreateAclSet(aclNamev6, oc.Acl_ACL_TYPE_ACL_IPV6)
		aclAcceptEntryv6 := aclSetv6.GetOrCreateAclEntry(10) // Using 10 for consistency

		ipv6Acl := aclAcceptEntryv6.GetOrCreateIpv6()
		ipv6Acl.SourceAddress = ygot.String(fmt.Sprintf("%s/%d", otgPort1.IPv6, ipv6PrefixLen))
		ipv6Acl.DestinationAddress = ygot.String(fmt.Sprintf("%s/%d", otgPort2.IPv6, ipv6PrefixLen))
		ipv6Acl.SetDscp(uint8(tc.aclDscpValue))

		if len(tc.srcDstPortPair) == 2 {
			aclAcceptEntryv6.GetOrCreateIpv6().SetProtocol(oc.UnionUint8(ipProtoTCP))
			// NOTE: For IPv6, Transport needs to be created on the entry itself, not under Ipv6.
			aclTransport := aclAcceptEntryv6.GetOrCreateTransport()
			aclTransport.SetSourcePort(oc.UnionUint16(tc.srcDstPortPair[0]))
			aclTransport.SetDestinationPort(oc.UnionUint16(tc.srcDstPortPair[1]))
		}
		aclAcceptEntryv6.GetOrCreateActions().SetForwardingAction(oc.Acl_FORWARDING_ACTION_ACCEPT)

		// Adding allow rule for IPV6 ND packets
		aclARPEntry := aclSetv6.GetOrCreateAclEntry(15)
		aclMatchipv6 := aclARPEntry.GetOrCreateIpv6()
		aclMatchipv6.Protocol = oc.UnionUint8(ipProtoICMPv6)
		aclARPEntry.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT

		// Add a deny all rule at the end for IPv6
		aclDropEntryv6 := aclSetv6.GetOrCreateAclEntry(40)
		aclDropEntryv6.Description = ygot.String("dscp mismatch drop v6")
		aclDropEntryv6.GetOrCreateActions().SetForwardingAction(oc.Acl_FORWARDING_ACTION_DROP)

		gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclNamev6, aclSetv6.GetType()).Config(), aclSetv6)
	}

	configureAclInterface(t, dut, acl, tc)
}

// adding a comment to trigger run
