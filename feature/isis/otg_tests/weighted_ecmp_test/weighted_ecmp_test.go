package weighted_ecmp_test

import (
	"fmt"
	"testing"
	"time"

	"math/rand"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PLen          = 30
	ipv6PLen          = 126
	isisInstance      = "DEFAULT"
	dutAreaAddress    = "49.0001"
	ateAreaAddress    = "49"
	dutSysID          = "1920.0000.2001"
	asn               = 64501
	acceptRoutePolicy = "PERMIT-ALL"
	trafficPPS        = 50000 // Should be 5000000
	trafficv6PPS      = 50000 // Should be 5000000
	srcTrafficV4      = "100.0.2.1"
	srcTrafficV6      = "2001:db8:64:65::1"
	dstTrafficV4      = "100.0.1.1"
	dstTrafficV6      = "2001:db8:64:64::1"
	v4Count           = 254
	v6Count           = 1000 // Should be 10000000
	fixedPackets      = 1000000
)

type aggPortData struct {
	dutIPv4       string
	ateIPv4       string
	dutIPv6       string
	ateIPv6       string
	ateAggName    string
	ateAggMAC     string
	atePort1MAC   string
	atePort2MAC   string
	ateISISSysID  string
	ateLoopbackV4 string
	ateLoopbackV6 string
}

type ipAddr struct {
	ip     string
	prefix uint32
}

var (
	agg1 = &aggPortData{
		dutIPv4:       "192.0.2.1",
		ateIPv4:       "192.0.2.2",
		dutIPv6:       "2001:db8::1",
		ateIPv6:       "2001:db8::2",
		ateAggName:    "lag1",
		ateAggMAC:     "02:00:01:01:01:01",
		atePort1MAC:   "02:00:01:01:01:02",
		atePort2MAC:   "02:00:01:01:01:03",
		ateISISSysID:  "640000000002",
		ateLoopbackV4: "192.0.2.17",
		ateLoopbackV6: "2001:db8::17",
	}
	agg2 = &aggPortData{
		dutIPv4:       "192.0.2.5",
		ateIPv4:       "192.0.2.6",
		dutIPv6:       "2001:db8::5",
		ateIPv6:       "2001:db8::6",
		ateAggName:    "lag2",
		ateAggMAC:     "02:00:01:01:01:04",
		atePort1MAC:   "02:00:01:01:01:05",
		atePort2MAC:   "02:00:01:01:01:06",
		ateISISSysID:  "640000000003",
		ateLoopbackV4: "192.0.2.18",
		ateLoopbackV6: "2001:db8::18",
	}
	agg3 = &aggPortData{
		dutIPv4:       "192.0.2.9",
		ateIPv4:       "192.0.2.10",
		dutIPv6:       "2001:db8::11",
		ateIPv6:       "2001:db8::12",
		ateAggName:    "lag3",
		ateAggMAC:     "02:00:01:01:01:07",
		atePort1MAC:   "02:00:01:01:01:08",
		atePort2MAC:   "02:00:01:01:01:09",
		ateISISSysID:  "640000000004",
		ateLoopbackV4: "192.0.2.18",
		ateLoopbackV6: "2001:db8::18",
	}
	agg4 = &aggPortData{
		dutIPv4:       "192.0.2.13",
		ateIPv4:       "192.0.2.14",
		dutIPv6:       "2001:db8::14",
		ateIPv6:       "2001:db8::15",
		ateAggName:    "lag4",
		ateAggMAC:     "02:00:01:01:01:10",
		atePort1MAC:   "02:00:01:01:01:11",
		atePort2MAC:   "02:00:01:01:01:12",
		ateISISSysID:  "640000000005",
		ateLoopbackV4: "192.0.2.18",
		ateLoopbackV6: "2001:db8::18",
	}
	dutLoopback = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "192.0.2.21",
		IPv6:    "2001:db8::21",
		IPv4Len: 32,
		IPv6Len: 128,
	}
	ate1AdvV4 = &ipAddr{ip: "100.0.2.0", prefix: 24}
	ate1AdvV6 = &ipAddr{ip: "2001:db8:64:65::0", prefix: 64}
	ate2AdvV4 = &ipAddr{ip: "100.0.1.0", prefix: 24}
	ate2AdvV6 = &ipAddr{ip: "2001:db8:64:64::0", prefix: 64}

	equalDistributionWeights   = []uint64{33, 33, 33}
	unequalDistributionWeights = []uint64{20, 40, 40}

	ecmpTolerance = uint64(1)

	lb string

	vendor ondatra.Vendor

	isisLevel = 2
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func TestWeightedECMPForISIS(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggIDs := configureDUT(t, dut)
	vendor = dut.Vendor()
	// Enable weighted ECMP in ISIS and set LoadBalancing to Auto
	if !deviations.RibWecmp(dut) {
		b := &gnmi.SetBatch{}
		// isisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
		isisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
		gnmi.BatchReplace(b, isisPath.Global().WeightedEcmp().Config(), true)
		for _, aggID := range aggIDs {
			gnmi.BatchReplace(b, isisPath.Interface(aggID).WeightedEcmp().Config(), &oc.NetworkInstance_Protocol_Isis_Interface_WeightedEcmp{
				LoadBalancingWeight: oc.NetworkInstance_Protocol_Isis_Interface_WeightedEcmp_LoadBalancingWeight_Union(oc.WeightedEcmp_LoadBalancingWeight_auto),
			})
		}
		b.Set(t, dut)
	}
	if deviations.WecmpAutoUnsupported(dut) {
		var weight string
		switch dut.Vendor() {
		case ondatra.CISCO:
			weight = fmt.Sprintf(" router isis DEFAULT \n interface %s \n address-family ipv4 unicast \n weight 100 \n address-family ipv6 unicast \n weight 100 \n ! \n interface %s \n address-family ipv4 unicast \n weight 100 \n address-family ipv6 unicast \n weight 100 \n ! \n interface %s \n address-family ipv4 unicast \n weight 100 \n address-family ipv6 unicast \n weight 100 \n", aggIDs[1], aggIDs[2], aggIDs[3])
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'WecmpAutoUnsupported'", dut.Vendor())
		}
		helpers.GnmiCLIConfig(t, dut, weight)
	}
	top := configureATE(t, ate)
	flows := configureFlows(t, top, ate1AdvV4, ate1AdvV6, ate2AdvV4, ate2AdvV6)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	VerifyISISTelemetry(t, dut, aggIDs, []*aggPortData{agg1, agg2})
	for _, agg := range []*aggPortData{agg1, agg2} {
		bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
		gnmi.Await(t, dut, bgpPath.Neighbor(agg.ateLoopbackV4).SessionState().State(), 2*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		gnmi.Await(t, dut, bgpPath.Neighbor(agg.ateLoopbackV6).SessionState().State(), 2*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}

	startTraffic(t, ate, top)
	time.Sleep(time.Minute)
	t.Run("Equal_Distribution_Of_Traffic", func(t *testing.T) {
		for _, flow := range flows {
			loss := otgutils.GetFlowLossPct(t, ate.OTG(), flow.Name(), 20*time.Second)
			if got, want := loss, 0.0; got != want {
				t.Errorf("Flow %s loss: got %f, want %f", flow.Name(), got, want)
			}
		}
		time.Sleep(time.Minute)
		weights := trafficRXWeights(t, ate, []string{agg2.ateAggName, agg3.ateAggName, agg4.ateAggName})
		for idx, weight := range equalDistributionWeights {
			if got, want := weights[idx], weight; got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP Percentage for Aggregate Index: %d: got %d, want %d", idx+1, got, want)
			}
		}
	})

	// Disable ATE2:Port1
	if deviations.ATEPortLinkStateOperationsUnsupported(ate) {
		p3 := dut.Port(t, "port3")
		gnmi.Replace(t, dut, gnmi.OC().Interface(p3.Name()).Enabled().Config(), false)
		t.Logf("Disable ATE2:Port1: %s, %s", p3.Name(), gnmi.OC().Interface(p3.Name()).OperStatus().State())
	} else {
		p3 := ate.Port(t, "port3") // ATE:port3 is ATE2:port1
		psa := gosnappi.NewControlState()
		psa.Port().Link().SetPortNames([]string{p3.ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
		ate.OTG().SetControlState(t, psa)
		time.Sleep(10 * time.Second)
		defer func() {
			psa := gosnappi.NewControlState()
			psa.Port().Link().SetPortNames([]string{p3.ID()}).SetState(gosnappi.StatePortLinkState.UP)
			ate.OTG().SetControlState(t, psa)
		}()
	}
	p3 := dut.Port(t, "port3")
	gnmi.Await(t, dut, gnmi.OC().Interface(p3.Name()).OperStatus().State(), time.Minute*2, oc.Interface_OperStatus_DOWN)

	if deviations.WecmpAutoUnsupported(dut) {
		var weight string
		switch dut.Vendor() {
		case ondatra.CISCO:
			weight = fmt.Sprintf(" router isis DEFAULT \n interface %s \n address-family ipv4 unicast \n weight 200 \n address-family ipv6 unicast \n weight 200 \n ! \n interface %s \n address-family ipv4 unicast \n weight 400 \n address-family ipv6 unicast \n weight 400 \n ! \n interface %s \n address-family ipv4 unicast \n weight 400 \n address-family ipv6 unicast \n weight 400 \n", aggIDs[1], aggIDs[2], aggIDs[3])
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'WecmpAutoUnsupported'", dut.Vendor())
		}
		helpers.GnmiCLIConfig(t, dut, weight)
	}

	top.Flows().Clear()
	if deviations.ISISLoopbackRequired(dut) {
		flows = configureFlows(t, top, ate1AdvV4, ate1AdvV6, ate2AdvV4, ate2AdvV6)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		VerifyISISTelemetry(t, dut, aggIDs, []*aggPortData{agg1, agg2})
		for _, agg := range []*aggPortData{agg1, agg2} {
			bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
			gnmi.Await(t, dut, bgpPath.Neighbor(agg.ateLoopbackV4).SessionState().State(), 3*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
			gnmi.Await(t, dut, bgpPath.Neighbor(agg.ateLoopbackV6).SessionState().State(), 3*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		}
	}

	startTraffic(t, ate, top)
	time.Sleep(time.Minute)

	t.Run("Unequal_Distribution_Of_Traffic", func(t *testing.T) {
		for _, flow := range flows {
			loss := otgutils.GetFlowLossPct(t, ate.OTG(), flow.Name(), 20*time.Second)
			if got, want := loss, 0.0; got != want {
				t.Errorf("Flow %s loss: got %f, want %f", flow.Name(), got, want)
			}
		}
		time.Sleep(time.Minute)
		weights := trafficRXWeights(t, ate, []string{agg2.ateAggName, agg3.ateAggName, agg4.ateAggName})
		for idx, weight := range unequalDistributionWeights {
			if got, want := weights[idx], weight; got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP Percentage for Aggregate Index: %d: got %d, want %d", idx+1, got, want)
			}
		}
	})
}

func trafficRXWeights(t *testing.T, ate *ondatra.ATEDevice, aggNames []string) []uint64 {
	t.Helper()
	var rxs []uint64
	for _, aggName := range aggNames {
		metrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Lag(aggName).State())
		rxs = append(rxs, metrics.GetCounters().GetInFrames())
	}
	var total uint64
	for _, rx := range rxs {
		total += rx
	}
	for idx, rx := range rxs {
		rxs[idx] = (rx * 100) / total
	}
	return rxs
}

func startTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	ate.OTG().StartTraffic(t)
	time.Sleep(time.Minute)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogLAGMetrics(t, ate.OTG(), top)
}

func randRange(t *testing.T, start, end uint32, count int) []uint32 {
	if count > int(end-start) {
		t.Fatal("randRange: count greater than end-start.")
	}
	rand.New(rand.NewSource(time.Now().UnixNano()))
	var result []uint32
	for len(result) < count {
		diff := end - start
		randomValue := rand.Int31n(int32(diff)) + int32(start)
		result = append(result, uint32(randomValue))
	}
	return result
}

func configureFlows(t *testing.T, top gosnappi.Config, srcV4, srcV6, dstV4, dstV6 *ipAddr) []gosnappi.Flow {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	top.Flows().Clear()
	fV4 := top.Flows().Add().SetName("flowV4")
	if deviations.WeightedEcmpFixedPacketVerification(dut) {
		fV4.Duration().FixedPackets().SetPackets(fixedPackets)
	}
	fV4.Metrics().SetEnable(true)
	fV4.TxRx().Device().
		SetTxNames([]string{agg1.ateAggName + ".IPv4"}).
		SetRxNames([]string{agg2.ateAggName + ".IPv4", agg3.ateAggName + ".IPv4", agg4.ateAggName + ".IPv4"})
	fV4.Size().SetFixed(1500)
	fV4.Rate().SetPps(trafficPPS)
	eV4 := fV4.Packet().Add().Ethernet()
	eV4.Src().SetValue(agg1.ateAggMAC)
	v4 := fV4.Packet().Add().Ipv4()
	v4.Src().Increment().SetStart(srcTrafficV4).SetCount(v4Count)
	v4.Dst().Increment().SetStart(dstTrafficV4).SetCount(v4Count)
	udp := fV4.Packet().Add().Udp()
	udp.SrcPort().SetValues(randRange(t, 34525, 65535, 5000))
	udp.DstPort().SetValues(randRange(t, 49152, 65535, 5000))

	fV6 := top.Flows().Add().SetName("flowV6")
	if deviations.WeightedEcmpFixedPacketVerification(dut) {
		fV6.Duration().FixedPackets().SetPackets(fixedPackets)
	}
	fV6.Metrics().SetEnable(true)
	fV6.TxRx().Device().
		SetTxNames([]string{agg1.ateAggName + ".IPv6"}).
		SetRxNames([]string{agg2.ateAggName + ".IPv6", agg3.ateAggName + ".IPv6", agg4.ateAggName + ".IPv6"})
	fV6.Size().SetFixed(1500)
	fV6.Rate().SetPps(trafficv6PPS)
	eV6 := fV6.Packet().Add().Ethernet()
	eV6.Src().SetValue(agg1.ateAggMAC)

	v6 := fV6.Packet().Add().Ipv6()
	v6.Src().Increment().SetStart(srcTrafficV6).SetCount(v6Count)
	v6.Dst().Increment().SetStart(dstTrafficV6).SetCount(v6Count)
	udpv6 := fV6.Packet().Add().Udp()
	udpv6.SrcPort().SetValues(randRange(t, 35521, 65535, 5000))
	udpv6.DstPort().SetValues(randRange(t, 49152, 65535, 5000))

	return []gosnappi.Flow{fV4, fV6}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	pmd100GFRPorts := []string{}

	for aggIdx, a := range []*aggPortData{agg1, agg2, agg3, agg4} {
		p1 := ate.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+1))
		p2 := ate.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+2))
		top.Ports().Add().SetName(p1.ID())
		top.Ports().Add().SetName(p2.ID())
		if p1.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, p1.ID())
		}
		if p2.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, p2.ID())
		}

		agg := top.Lags().Add().SetName(a.ateAggName)
		agg.Protocol().Static().SetLagId(uint32(aggIdx + 1))

		lagDev := top.Devices().Add().SetName(agg.Name() + ".Dev")
		lagEth := lagDev.Ethernets().Add().SetName(agg.Name() + ".Eth").SetMac(a.ateAggMAC)
		lagEth.Connection().SetLagName(agg.Name())
		lagEth.Ipv4Addresses().Add().SetName(agg.Name() + ".IPv4").SetAddress(a.ateIPv4).SetGateway(a.dutIPv4).SetPrefix(ipv4PLen)
		lagEth.Ipv6Addresses().Add().SetName(agg.Name() + ".IPv6").SetAddress(a.ateIPv6).SetGateway(a.dutIPv6).SetPrefix(ipv6PLen)
		lagDev.Ipv4Loopbacks().Add().SetName(agg.Name() + ".Loopback4").SetEthName(lagEth.Name()).SetAddress(a.ateLoopbackV4)
		lagDev.Ipv6Loopbacks().Add().SetName(agg.Name() + ".Loopback6").SetEthName(lagEth.Name()).SetAddress(a.ateLoopbackV6)

		agg.Ports().Add().SetPortName(p1.ID()).Ethernet().SetMac(a.atePort1MAC).SetName(a.ateAggName + ".1")
		agg.Ports().Add().SetPortName(p2.ID()).Ethernet().SetMac(a.atePort2MAC).SetName(a.ateAggName + ".2")

		configureOTGISIS(t, lagDev, a)
		if aggIdx == 0 {
			configureOTGBGP(t, lagDev, a, ate1AdvV4, ate1AdvV6)
		} else {
			configureOTGBGP(t, lagDev, a, ate2AdvV4, ate2AdvV6)
		}
	}

	// Disable FEC for 100G-FR ports because Novus does not support it.
	if len(pmd100GFRPorts) > 0 {
		l1Settings := top.Layer1().Add().SetName("L1").SetPortNames(pmd100GFRPorts)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}
	return top
}

func configureOTGBGP(t *testing.T, dev gosnappi.Device, agg *aggPortData, advV4, advV6 *ipAddr) {
	t.Helper()
	v4 := dev.Ipv4Loopbacks().Items()[0]
	v6 := dev.Ipv6Loopbacks().Items()[0]

	iDutBgp := dev.Bgp().SetRouterId(agg.ateIPv4)
	iDutBgp4Peer := iDutBgp.Ipv4Interfaces().Add().SetIpv4Name(v4.Name()).Peers().Add().SetName(agg.ateAggName + ".BGP4.peer")
	iDutBgp4Peer.SetPeerAddress(dutLoopback.IPv4).SetAsNumber(asn).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDutBgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(false)
	iDutBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(false)

	iDutBgp6Peer := iDutBgp.Ipv6Interfaces().Add().SetIpv6Name(v6.Name()).Peers().Add().SetName(agg.ateAggName + ".BGP6.peer")
	iDutBgp6Peer.SetPeerAddress(dutLoopback.IPv6).SetAsNumber(asn).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDutBgp6Peer.Capability().SetIpv4UnicastAddPath(false).SetIpv6UnicastAddPath(true)
	iDutBgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(false).SetUnicastIpv6Prefix(true)

	bgpNeti1Bgp4PeerRoutes := iDutBgp4Peer.V4Routes().Add().SetName(agg.ateAggName + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(agg.ateLoopbackV4).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(advV4.ip).SetPrefix(advV4.prefix).SetCount(1)
	bgpNeti1Bgp4PeerRoutes.AddPath().SetPathId(1)

	bgpNeti1Bgp6PeerRoutes := iDutBgp6Peer.V6Routes().Add().SetName(agg.ateAggName + ".BGP6.Route")
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(advV6.ip).SetPrefix(advV6.prefix).SetCount(1)
	bgpNeti1Bgp6PeerRoutes.AddPath().SetPathId(1)
}

func configureOTGISIS(t *testing.T, dev gosnappi.Device, agg *aggPortData) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	isis := dev.Isis().SetSystemId(agg.ateISISSysID).SetName(agg.ateAggName + ".ISIS")
	isis.Basic().SetHostname(isis.Name())
	isis.Advanced().SetAreaAddresses([]string{ateAreaAddress})

	isisInt := isis.Interfaces().Add().
		SetEthName(dev.Ethernets().Items()[0].Name()).SetName(agg.ateAggName + ".ISISInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).SetMetric(10)
	isisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)
	if deviations.ISISLoopbackRequired(dut) {
		// configure ISIS loopback interface and advertise them via ISIS.
		isisPort2V4 := dev.Isis().V4Routes().Add().SetName(agg.ateAggName + ".ISISV4").SetLinkMetric(10)
		isisPort2V4.Addresses().Add().SetAddress(agg.ateLoopbackV4).SetPrefix(32)
		isisPort2V6 := dev.Isis().V6Routes().Add().SetName(agg.ateAggName + ".ISISV6").SetLinkMetric(10)
		isisPort2V6.Addresses().Add().SetAddress(agg.ateLoopbackV6).SetPrefix(uint32(128))
	}

}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	configureDUTLoopback(t, dut)

	var aggIDs []string
	for aggIdx, a := range []*aggPortData{agg1, agg2, agg3, agg4} {
		b := &gnmi.SetBatch{}
		d := &oc.Root{}

		aggID := netutil.NextAggregateInterface(t, dut)
		aggIDs = append(aggIDs, aggID)

		agg := d.GetOrCreateInterface(aggID)
		agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
		agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		agg.Description = ygot.String(a.ateAggName)
		if deviations.InterfaceEnabled(dut) {
			agg.Enabled = ygot.Bool(true)
		}
		s := agg.GetOrCreateSubinterface(0)
		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		a4 := s4.GetOrCreateAddress(a.dutIPv4)
		a4.PrefixLength = ygot.Uint8(ipv4PLen)

		s6 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s6.Enabled = ygot.Bool(true)
		}
		a6 := s6.GetOrCreateAddress(a.dutIPv6)
		a6.PrefixLength = ygot.Uint8(ipv6PLen)

		gnmi.BatchDelete(b, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())
		gnmi.BatchReplace(b, gnmi.OC().Interface(aggID).Config(), agg)

		p1 := dut.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+1))
		p2 := dut.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+2))
		for _, port := range []*ondatra.Port{p1, p2} {
			gnmi.BatchDelete(b, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())

			i := d.GetOrCreateInterface(port.Name())
			i.Description = ygot.String(fmt.Sprintf("LAG - Member -%s", port.Name()))
			e := i.GetOrCreateEthernet()
			e.AggregateId = ygot.String(aggID)
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			if port.PMD() == ondatra.PMD100GBASEFR && deviations.ExplicitPortSpeed(dut) {
				e.AutoNegotiate = ygot.Bool(false)
				e.DuplexMode = oc.Ethernet_DuplexMode_FULL
				e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
			}

			gnmi.BatchReplace(b, gnmi.OC().Interface(port.Name()).Config(), i)
		}
		b.Set(t, dut)
	}
	// Wait for LAG interfaces to be UP
	for _, aggID := range aggIDs {
		gnmi.Await(t, dut, gnmi.OC().Interface(aggID).AdminStatus().State(), 60*time.Second, oc.Interface_AdminStatus_UP)
	}
	if !deviations.ISISLoopbackRequired(dut) {
		configureStaticRouteToATELoopbacks(t, dut)
	}
	configureRoutingPolicy(t, dut)
	configureDUTISIS(t, dut, aggIDs)
	configureDUTBGP(t, dut)
	return aggIDs
}

func configureRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(acceptRoutePolicy)
	stmt, _ := pdef.AppendNewStatement("20")
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(acceptRoutePolicy).Config(), pdef)
}

func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	lb = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(lb).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	foundV4 := false
	for _, ip := range ipv4Addrs {
		if v, ok := ip.Val(); ok {
			foundV4 = true
			dutLoopback.IPv4 = v.GetIp()
			break
		}
	}
	foundV6 := false
	for _, ip := range ipv6Addrs {
		if v, ok := ip.Val(); ok {
			foundV6 = true
			dutLoopback.IPv6 = v.GetIp()
			break
		}
	}
	if !foundV4 || !foundV6 {
		lo1 := dutLoopback.NewOCInterface(lb, dut)
		lo1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(lb).Config(), lo1)
	}
}

func configureStaticRouteToATELoopbacks(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	sr4ATE1 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          agg1.ateLoopbackV4 + "/32",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(agg1.ateIPv4),
		},
	}
	sr6ATE1 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          agg1.ateLoopbackV6 + "/128",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(agg1.ateIPv6),
		},
	}
	sr4ATE2 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          agg2.ateLoopbackV4 + "/32",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(agg2.ateIPv4),
			"1": oc.UnionString(agg3.ateIPv4),
			"2": oc.UnionString(agg4.ateIPv4),
		},
	}
	sr6ATE2 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          agg2.ateLoopbackV6 + "/128",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(agg2.ateIPv6),
			"1": oc.UnionString(agg3.ateIPv6),
			"2": oc.UnionString(agg4.ateIPv6),
		},
	}
	b := &gnmi.SetBatch{}
	for _, cfg := range []*cfgplugins.StaticRouteCfg{sr4ATE1, sr6ATE1, sr4ATE2, sr6ATE2} {
		if _, err := cfgplugins.NewStaticRouteCfg(b, cfg, dut); err != nil {
			t.Fatalf("Failed to configure static route to ATE Loopback: %v", err)
		}
	}
	b.Set(t, dut)
}

func configureDUTISIS(t *testing.T, dut *ondatra.DUTDevice, aggIDs []string) {
	t.Helper()

	d := &oc.Root{}
	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)

	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	lspBit := globalISIS.GetOrCreateLspBit().GetOrCreateOverloadBit()
	lspBit.SetBit = ygot.Bool(false)

	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLoopbackRequired(dut) {
		gnmi.Update(t, dut, gnmi.OC().Config(), d)
		// add loopback interface to ISIS
		aggIDs = append(aggIDs, "Loopback0")
	}
	// Add other ISIS interfaces
	for _, aggID := range aggIDs {
		isisIntf := isis.GetOrCreateInterface(aggID)
		isisIntf.GetOrCreateInterfaceRef().Interface = ygot.String(aggID)
		isisIntf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
		if deviations.InterfaceRefConfigUnsupported(dut) {
			isisIntf.InterfaceRef = nil
		}
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntf.Af = nil
		}

		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)

		isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfiv4.Metric = ygot.Uint32(10)
		isisIntfLevelAfiv4.Enabled = ygot.Bool(true)
		isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfiv6.Metric = ygot.Uint32(10)
		isisIntfLevelAfiv6.Enabled = ygot.Bool(true)
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			isisIntfLevelAfiv4.Enabled = nil
			isisIntfLevelAfiv6.Enabled = nil
		}
	}
	if deviations.ISISLoopbackRequired(dut) {
		gnmi.Update(t, dut, dutConfIsisPath.Config(), prot)
	} else {
		gnmi.Update(t, dut, gnmi.OC().Config(), d)
	}
}

func configureDUTBGP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutLoopback.IPv4)
	global.As = ygot.Uint32(asn)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	pgName := "BGP-PEER-GROUP1"
	pg := bgp.GetOrCreatePeerGroup(pgName)
	pg.PeerAs = ygot.Uint32(asn)
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{acceptRoutePolicy})
		rpl.SetImportPolicy([]string{acceptRoutePolicy})
	} else {
		af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		rpl := af4.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{acceptRoutePolicy})
		rpl.SetImportPolicy([]string{acceptRoutePolicy})

		af6 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(true)
		rpl = af6.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{acceptRoutePolicy})
		rpl.SetImportPolicy([]string{acceptRoutePolicy})
	}

	for _, a := range []*aggPortData{agg1, agg2, agg3, agg4} {
		bgpNbrV4 := bgp.GetOrCreateNeighbor(a.ateLoopbackV4)
		bgpNbrV4.PeerGroup = ygot.String(pgName)
		bgpNbrV4.PeerAs = ygot.Uint32(asn)
		bgpNbrV4.Enabled = ygot.Bool(true)
		bgpNbrV4T := bgpNbrV4.GetOrCreateTransport()
		localAddressLeafv4 := dutLoopback.IPv4
		if deviations.ISISLoopbackRequired(dut) {
			localAddressLeafv4 = lb
		}
		bgpNbrV4T.LocalAddress = ygot.String(localAddressLeafv4)
		af4 := bgpNbrV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		af6 := bgpNbrV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(false)

		bgpNbrV6 := bgp.GetOrCreateNeighbor(a.ateLoopbackV6)
		bgpNbrV6.PeerGroup = ygot.String(pgName)
		bgpNbrV6.PeerAs = ygot.Uint32(asn)
		bgpNbrV6.Enabled = ygot.Bool(true)
		bgpNbrV6T := bgpNbrV6.GetOrCreateTransport()
		localAddressLeafv6 := dutLoopback.IPv6
		if deviations.ISISLoopbackRequired(dut) {
			localAddressLeafv6 = lb
		}
		bgpNbrV6T.LocalAddress = ygot.String(localAddressLeafv6)

		af4 = bgpNbrV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(false)
		af6 = bgpNbrV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(true)
	}

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), niProto)

}
func VerifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice, dutIntfs []string, loopBacks []*aggPortData) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
	for _, dutIntf := range dutIntfs {

		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			dutIntf = dutIntf + ".0"
		}
		nbrPath := statePath.Interface(dutIntf)
		query := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().State()
		_, ok := gnmi.WatchAll(t, dut, query, 3*time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			return present && state == oc.Isis_IsisInterfaceAdjState_UP
		}).Await(t)
		if !ok {
			t.Logf("IS-IS state on %v has no adjacencies", dutIntf)
			t.Fatal("No IS-IS adjacencies reported.")
		}
	}
	if deviations.ISISLoopbackRequired(dut) {
		// verify loopback has been received via ISIS
		t.Log("Starting route check")
		for _, loopBack := range loopBacks {
			batch := gnmi.OCBatch()
			statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
			id := formatID(loopBack.ateISISSysID)
			iPv4Query := statePath.Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis().Level(uint8(isisLevel)).Lsp(id).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().Prefix(fmt.Sprintf(loopBack.ateLoopbackV4 + "/32"))
			iPv6Query := statePath.Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis().Level(uint8(isisLevel)).Lsp(id).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).ExtendedIpv4Reachability().Prefix(fmt.Sprintf(loopBack.ateLoopbackV6 + "/128"))
			batch.AddPaths(iPv4Query, iPv6Query)
			_, ok := gnmi.Watch(t, dut, batch.State(), 5*time.Minute, func(val *ygnmi.Value[*oc.Root]) bool {
				_, present := val.Val()
				return present
			}).Await(t)
			if !ok {
				t.Fatalf("ISIS did not receive the route loopback %s", loopBack.ateLoopbackV4)
			}
		}
	}
}

func formatID(input string) string {
	part1 := input[:4]
	part2 := input[4:8]
	part3 := input[8:12]

	formatted := fmt.Sprintf("%s.%s.%s.00-00", part1, part2, part3)

	return formatted
}
