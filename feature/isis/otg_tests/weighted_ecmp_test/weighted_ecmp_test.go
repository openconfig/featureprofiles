package weighted_ecmp_test

import (
	"fmt"
	"testing"
	"time"

	"math/rand"

	"github.com/open-traffic-generator/snappi/gosnappi"
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
	srcTrafficV4      = "192.0.2.2"
	srcTrafficV6      = "2000:db8::2"
	dstTrafficV4      = "100.0.1.1"
	dstTrafficV6      = "2010:db8:64:64::1"
	v4Count           = 254
	v6Count           = 1000 // Should be 10000000
	fixedPackets      = 1000000
)

type aggPortData struct {
	dutIPv4      string
	ateIPv4      string
	dutIPv6      string
	ateIPv6      string
	ateAggName   string
	ateAggMAC    string
	atePort1MAC  string
	atePort2MAC  string
	ateISISSysID string
	v4Route      string
	v4RouteCount int
	v6Route      string
	v6RouteCount int
}

type portData struct {
	name         string
	dutIPv4      string
	ateIPv4      string
	dutIPv6      string
	ateIPv6      string
	atePortMAC   string
	ateISISSysID string
}

var (
	ateSrc = portData{
		name:         "srcPort",
		dutIPv4:      "192.0.2.1",
		ateIPv4:      "192.0.2.2",
		dutIPv6:      "2000:db8::1",
		ateIPv6:      "2000:db8::2",
		atePortMAC:   "02:00:01:01:01:02",
		ateISISSysID: "640000000002",
	}

	agg1 = &aggPortData{
		dutIPv4:      "192.0.2.5",
		ateIPv4:      "192.0.2.6",
		dutIPv6:      "2001:db8::1",
		ateIPv6:      "2001:db8::2",
		ateAggName:   "lag1",
		ateAggMAC:    "02:00:01:01:01:04",
		atePort1MAC:  "02:00:01:01:01:05",
		atePort2MAC:  "02:00:01:01:01:06",
		ateISISSysID: "640000000003",
		v4Route:      "100.0.1.1",
		v4RouteCount: 254,
		v6Route:      "2010:db8:64:64::1",
		v6RouteCount: 1000,
	}
	agg2 = &aggPortData{
		dutIPv4:      "192.0.2.9",
		ateIPv4:      "192.0.2.10",
		dutIPv6:      "2002:db8::1",
		ateIPv6:      "2002:db8::2",
		ateAggName:   "lag2",
		ateAggMAC:    "02:00:01:01:01:07",
		atePort1MAC:  "02:00:01:01:01:08",
		atePort2MAC:  "02:00:01:01:01:09",
		ateISISSysID: "640000000004",
		v4Route:      "100.0.1.1",
		v4RouteCount: 254,
		v6Route:      "2010:db8:64:64::1",
		v6RouteCount: 1000,
	}
	agg3 = &aggPortData{
		dutIPv4:      "192.0.2.13",
		ateIPv4:      "192.0.2.14",
		dutIPv6:      "2003:db8::1",
		ateIPv6:      "2003:db8::2",
		ateAggName:   "lag3",
		ateAggMAC:    "02:00:01:01:01:10",
		atePort1MAC:  "02:00:01:01:01:11",
		atePort2MAC:  "02:00:01:01:01:12",
		ateISISSysID: "640000000005",
		v4Route:      "100.0.1.1",
		v4RouteCount: 254,
		v6Route:      "2010:db8:64:64::1",
		v6RouteCount: 1000,
	}

	equalDistributionWeights   = []uint64{33, 33, 33}
	unequalDistributionWeights = []uint64{20, 40, 40}

	ecmpTolerance = uint64(1)
	vendor        ondatra.Vendor
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
	flows := configureFlows(t, top)
	ate.OTG().PushConfig(t, top)
	time.Sleep(30 * time.Second)
	ate.OTG().StartProtocols(t)
	time.Sleep(30 * time.Second)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
	VerifyISISTelemetry(t, dut, aggIDs, []*aggPortData{agg1, agg2})

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
		weights := trafficRXWeights(t, ate, []string{agg1.ateAggName, agg2.ateAggName, agg3.ateAggName})
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

	flows = configureFlows(t, top)
	ate.OTG().PushConfig(t, top)
	time.Sleep(30 * time.Second)
	ate.OTG().StartProtocols(t)
	time.Sleep(30 * time.Second)
	VerifyISISTelemetry(t, dut, aggIDs, []*aggPortData{agg1, agg2})

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
		weights := trafficRXWeights(t, ate, []string{agg1.ateAggName, agg2.ateAggName, agg3.ateAggName})
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
	time.Sleep(time.Minute)
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

func configureFlows(t *testing.T, top gosnappi.Config) []gosnappi.Flow {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	top.Flows().Clear()
	fV4 := top.Flows().Add().SetName("flowV4")
	if deviations.WeightedEcmpFixedPacketVerification(dut) {
		fV4.Duration().FixedPackets().SetPackets(fixedPackets)
	}
	fV4.Metrics().SetEnable(true)
	fV4.TxRx().Device().
		SetTxNames([]string{ateSrc.name + ".IPv4"}).
		SetRxNames([]string{agg1.ateAggName + ".ISISV4", agg2.ateAggName + ".ISISV4", agg3.ateAggName + ".ISISV4"})
	fV4.Size().SetFixed(1500)
	fV4.Rate().SetPps(trafficPPS)
	eV4 := fV4.Packet().Add().Ethernet()
	eV4.Src().SetValue(agg1.ateAggMAC)
	v4 := fV4.Packet().Add().Ipv4()
	v4.Src().Increment().SetStart(srcTrafficV4).SetCount(1)
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
		SetTxNames([]string{ateSrc.name + ".IPv6"}).
		SetRxNames([]string{agg1.ateAggName + ".ISISV6", agg2.ateAggName + ".ISISV6", agg3.ateAggName + ".ISISV6"})
	fV6.Size().SetFixed(1500)
	fV6.Rate().SetPps(trafficv6PPS)
	eV6 := fV6.Packet().Add().Ethernet()
	eV6.Src().SetValue(agg1.ateAggMAC)

	v6 := fV6.Packet().Add().Ipv6()
	v6.Src().Increment().SetStart(srcTrafficV6).SetCount(1)
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

	p0 := ate.Port(t, "port1")
	top.Ports().Add().SetName(p0.ID())
	srcDev := top.Devices().Add().SetName(ateSrc.name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.name + ".Eth").SetMac(ateSrc.atePortMAC)
	srcEth.Connection().SetPortName(p0.ID())
	srcEth.Ipv4Addresses().Add().SetName(ateSrc.name + ".IPv4").SetAddress(ateSrc.ateIPv4).SetGateway(ateSrc.dutIPv4).SetPrefix(uint32(ipv4PLen))
	srcEth.Ipv6Addresses().Add().SetName(ateSrc.name + ".IPv6").SetAddress(ateSrc.ateIPv6).SetGateway(ateSrc.dutIPv6).SetPrefix(uint32(ipv6PLen))

	for aggIdx, a := range []*aggPortData{agg1, agg2, agg3} {
		p1 := ate.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+2))
		p2 := ate.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+3))
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

		agg.Ports().Add().SetPortName(p1.ID()).Ethernet().SetMac(a.atePort1MAC).SetName(a.ateAggName + ".1")
		agg.Ports().Add().SetPortName(p2.ID()).Ethernet().SetMac(a.atePort2MAC).SetName(a.ateAggName + ".2")

		configureOTGISIS(t, lagDev, a)
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

func configureOTGISIS(t *testing.T, dev gosnappi.Device, agg *aggPortData) {
	t.Helper()

	isis := dev.Isis().SetSystemId(agg.ateISISSysID).SetName(agg.ateAggName + ".ISIS")
	isis.Basic().SetHostname(isis.Name()).SetLearnedLspFilter(true)
	isis.Advanced().SetAreaAddresses([]string{ateAreaAddress})

	isisInt := isis.Interfaces().Add().
		SetEthName(dev.Ethernets().Items()[0].Name()).SetName(agg.ateAggName + ".ISISInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).SetMetric(10)
	isisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)
	isisPort2V4 := dev.Isis().V4Routes().Add().SetName(agg.ateAggName + ".ISISV4").SetLinkMetric(10)
	isisPort2V4.Addresses().Add().SetAddress(agg.v4Route).SetPrefix(24)
	isisPort2V6 := dev.Isis().V6Routes().Add().SetName(agg.ateAggName + ".ISISV6").SetLinkMetric(10)
	isisPort2V6.Addresses().Add().SetAddress(agg.v6Route).SetPrefix(uint32(64))
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	d := &oc.Root{}
	p1 := dut.Port(t, "port1")

	i := d.GetOrCreateInterface(p1.Name())

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(ateSrc.dutIPv4)
	a4.PrefixLength = ygot.Uint8(ipv4PLen)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	a6 := s6.GetOrCreateAddress(ateSrc.dutIPv6)
	a6.PrefixLength = ygot.Uint8(ipv6PLen)

	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), i)

	var aggIDs []string
	for aggIdx, a := range []*aggPortData{agg1, agg2, agg3} {
		b := &gnmi.SetBatch{}

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

		p1 := dut.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+2))
		p2 := dut.Port(t, fmt.Sprintf("port%d", (aggIdx*2)+3))
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

	configureRoutingPolicy(t, dut)
	configureDUTISIS(t, dut, aggIDs)

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
	if deviations.ISISLevelEnabled(dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}
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
}
