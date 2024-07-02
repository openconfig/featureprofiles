package static_route_test_test

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
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	tolerance     = 0.01
	ipv4DstPfx    = "203.0.1.1"
	ipv6DstPfx    = "2003:db8::1"
	vrfName       = "VRF1"
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	ateDst = attrs.Attributes{
		Name:    "ateDst",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureInterfaceVRF(t *testing.T,
	dut *ondatra.DUTDevice,
	portName string) {

	// create vrf and apply on interface
	v := &oc.NetworkInstance{
		Name: ygot.String(vrfName),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	}
	vi := v.GetOrCreateInterface(portName)
	vi.Interface = ygot.String(portName)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), v)
}

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface,
	a *attrs.Attributes,
	dut *ondatra.DUTDevice) *oc.Interface {

	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)

	// s.NetworkInstanceName = ygot.String(vrfName)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	// Add IPv6 stack.
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// configureDUT configures port1, port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	t.Logf("Configuring VRF on interface %s", p1.Name())
	configureInterfaceVRF(t, dut, p1.Name())

	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	i1.Enabled = ygot.Bool(true)
	gnmi.Update(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, dut))

	p2 := dut.Port(t, "port2")
	t.Logf("Configuring VRF on interface %s", p2.Name())
	configureInterfaceVRF(t, dut, p2.Name())
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	i2.Enabled = ygot.Bool(true)
	gnmi.Update(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, dut))

}

// configureATE configures port1 and port2 on the ATE.
func configureOTG(t *testing.T) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	port1 := top.Ports().Add().SetName("port1")
	port2 := top.Ports().Add().SetName("port2")

	// Port1 Configuration.
	iDut1Dev := top.Devices().Add().SetName(ateSrc.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	iDut1Ipv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	iDut1Ipv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	// Port2 Configuration.
	iDut2Dev := top.Devices().Add().SetName(ateDst.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	iDut2Ipv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6")
	iDut2Ipv6.SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	return top

}
func createTrafficFlows(t *testing.T,
	ate *ondatra.ATEDevice,
	dut *ondatra.DUTDevice,
	top gosnappi.Config,
	ipv4DstPfx, ipv6DstPfx string) (gosnappi.Flow, gosnappi.Flow) {

	// Get DUT MAC interface for traffic MPLS flow
	dutDstInterface := dut.Port(t, "port1").Name()
	dstMac := gnmi.Get(t, dut, gnmi.OC().Interface(dutDstInterface).Ethernet().MacAddress().State())
	t.Logf("DUT remote MAC address is %s", dstMac)

	// Common setup for both IPv4 and IPv6 flows
	setupFlow := func(ipVersion string, srcIP, dstIP string) gosnappi.Flow {
		flowName := fmt.Sprintf("%s-to-%s-Flow:", srcIP, dstIP)
		flow := top.Flows().Add().SetName(flowName)
		flow.TxRx().Port().
			SetTxName(ate.Port(t, "port1").ID()).
			SetRxNames([]string{ate.Port(t, "port2").ID()})

		flow.Metrics().SetEnable(true)
		flow.Rate().SetPps(500)
		flow.Size().SetFixed(512)
		flow.Duration().Continuous()

		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(ateSrc.MAC)
		eth.Dst().SetValue(dstMac)

		if ipVersion == "IPv4" {
			ip := flow.Packet().Add().Ipv4()
			ip.Src().SetValue(srcIP)
			// ip.Dst().SetValue(dstIP)
			ip.Dst().Increment().SetStart(ipv4DstPfx).SetCount(200)
		} else {
			ip := flow.Packet().Add().Ipv6()
			ip.Src().SetValue(srcIP)
			// ip.Dst().SetValue(dstIP)
			ip.Dst().Increment().SetStart(ipv6DstPfx).SetCount(200)
		}

		return flow
	}

	// Create IPv4 traffic flow
	trafficFlowV4 := setupFlow("IPv4", ateSrc.IPv4, ipv4DstPfx)

	// Create IPv6 traffic flow
	trafficFlowV6 := setupFlow("IPv6", ateSrc.IPv6, ipv6DstPfx)

	return trafficFlowV4, trafficFlowV6
}

// Send traffic and validate traffic.
func verifyTrafficStreams(t *testing.T,
	ate *ondatra.ATEDevice,
	top gosnappi.Config,
	otg *otg.OTG,
	trafficFlows ...gosnappi.Flow) {
	t.Helper()

	t.Log("Starting traffic for 30 seconds")
	ate.OTG().StartTraffic(t)
	time.Sleep(30 * time.Second)
	t.Log("Stopping traffic and waiting 10 seconds for traffic stats to complete")
	ate.OTG().StopTraffic(t)
	time.Sleep(10 * time.Second)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)

	// Loop through each flow to validate packets
	for _, flow := range trafficFlows {
		flowName := flow.Name()
		txPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State()))
		rxPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().InPkts().State()))

		// Calculate the acceptable lower and upper bounds for rxPkts
		lowerBound := txPkts * (1 - tolerance)
		upperBound := txPkts * (1 + tolerance)

		if rxPkts < lowerBound || rxPkts > upperBound {
			t.Errorf("Received packets for flow %s are outside of the acceptable range: %v (1%% tolerance from %v)", flowName, rxPkts, txPkts)
		} else {
			t.Logf("Received packets for flow %s are within the acceptable range: %v (1%% tolerance from %v)", flowName, rxPkts, txPkts)
		}
	}
}

func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	staticRoute1 := fmt.Sprintf("%s/%d", "0.0.0.0", uint32(0))
	staticRoute2 := fmt.Sprintf("%s/%d", "::0", uint32(0))

	ni := oc.NetworkInstance{Name: ygot.String(vrfName)}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

	sr1 := static.GetOrCreateStatic(staticRoute1)
	nh1 := sr1.GetOrCreateNextHop("0")
	nh1.NextHop = oc.UnionString(ateDst.IPv4)

	sr2 := static.GetOrCreateStatic(staticRoute2)
	nh2 := sr2.GetOrCreateNextHop("0")
	nh2.NextHop = oc.UnionString(ateDst.IPv6)

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// TestMplsStaticLabel
func TestStaticRouteToDefaultRoute(t *testing.T) {
	var top gosnappi.Config
	var v4flow, v6flow gosnappi.Flow
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otgObj := ate.OTG()

	t.Run("configureDUT Interfaces", func(t *testing.T) {
		// Configure the DUT
		configureDUT(t, dut)
	})

	t.Run("Configure Static Route", func(t *testing.T) {
		t.Log("Configure static route on DUT")
		configureStaticRoute(t, dut)
	})

	t.Run("ConfigureOTG", func(t *testing.T) {
		t.Logf("Configure OTG")
		top = configureOTG(t)
		v4flow, v6flow = createTrafficFlows(t, ate, dut, top, ipv4DstPfx, ipv6DstPfx)

		t.Log("pushing the following config to the OTG device")
		t.Log(top.String())
		otgObj.PushConfig(t, top)
		otgObj.StartProtocols(t)

	})
	t.Run("Start traffic and verify traffic", func(t *testing.T) {
		verifyTrafficStreams(t, ate, top, otgObj, v4flow, v6flow)
	})
}
