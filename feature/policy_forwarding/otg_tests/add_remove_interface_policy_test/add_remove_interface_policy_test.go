package add_remove_interface_policy_test

import (
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
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	// Test constants
	policyForwardingPolicyName = "pf-policy"
	nextHopGroupName           = "nhg-1"
	nextHop1Index              = 1
	nextHop2Index              = 2
	plen                       = 30
	trafficDuration            = 10 * time.Second
	pps                        = 100
)

var (
	// DUT configuration attributes
	dutIngress = attrs.Attributes{
		Desc:    "DUT Ingress",
		IPv4:    "192.0.2.1",
		IPv4Len: plen,
	}
	dutEgress1 = attrs.Attributes{
		Desc:    "DUT Egress 1",
		IPv4:    "192.0.2.5",
		IPv4Len: plen,
	}
	dutEgress2 = attrs.Attributes{
		Desc:    "DUT Egress 2",
		IPv4:    "192.0.2.9",
		IPv4Len: plen,
	}

	// ATE configuration attributes
	ateIngress = attrs.Attributes{
		Name:    "ATE Ingress",
		IPv4:    "192.0.2.2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plen,
	}
	ateEgress1 = attrs.Attributes{
		Name:    "ATE Egress 1",
		IPv4:    "192.0.2.6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plen,
	}
	ateEgress2 = attrs.Attributes{
		Name:    "ATE Egress 2",
		IPv4:    "192.0.2.10",
		MAC:     "02:00:03:01:01:01",
		IPv4Len: plen,
	}
)

// configureDUT configures the DUT with the necessary interfaces and policy forwarding policy.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	// Configure ingress interface
	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterface(i1, &dutIngress))

	// Configure egress interfaces
	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterface(i2, &dutEgress1))

	p3 := dut.Port(t, "port3")
	i3 := &oc.Interface{Name: ygot.String(p3.Name())}
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), configInterface(i3, &dutEgress2))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}

	// Configure network instance
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Configure policy forwarding policy
	pfPolicy := &oc.PolicyForwarding_Policy{
		Name: ygot.String(policyForwardingPolicyName),
	}
	pfRule := pfPolicy.GetOrCreateRule(1)
	pfRule.GetOrCreateIpv4().Protocol = oc.UnionUint8(6) // TCP
	action := pfRule.GetOrCreateAction()
	action.NextHopGroup = ygot.String(nextHopGroupName)

	// Configure next-hop-group with one interface
	nhg := &oc.NetworkInstance_PolicyForwarding_NextHopGroup{
		Name: ygot.String(nextHopGroupName),
	}
	nhg.GetOrCreateNextHop(nextHop1Index).NextHop = oc.UnionString(ateEgress1.IPv4)

	pf := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
	gnmi.Replace(t, dut, pf.Policy(policyForwardingPolicyName).Config(), pfPolicy)
	gnmi.Replace(t, dut, pf.NextHopGroup(nextHopGroupName).Config(), nhg)

	// Apply policy to ingress interface
	intf := pf.GetOrCreateInterface(p1.Name())
	intf.ApplyForwardingPolicy = ygot.String(policyForwardingPolicyName)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, pf.Interface(p1.Name()).Config(), intf)
}

// configInterface configures a single interface on the DUT.
func configInterface(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(a.IPv4Len)
	return i
}

// addInterfaceToNHG adds an interface to the next-hop-group.
func addInterfaceToNHG(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	pf := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
	nhg := gnmi.Get(t, dut, pf.NextHopGroup(nextHopGroupName).Config())
	nhg.GetOrCreateNextHop(nextHop2Index).NextHop = oc.UnionString(ateEgress2.IPv4)
	gnmi.Replace(t, dut, pf.NextHopGroup(nextHopGroupName).Config(), nhg)
}

// removeInterfaceFromNHG removes an interface from the next-hop-group and the device.
func removeInterfaceFromNHG(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	pfPath := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
	nhg := gnmi.Get(t, dut, pfPath.NextHopGroup(nextHopGroupName).Config())
	nhg.DeleteNextHop(nextHop2Index)

	p3 := dut.Port(t, "port3")
	i3 := &oc.Interface{Name: ygot.String(p3.Name())}

	b := &gnmi.SetBatch{}
	gnmi.BatchUpdate(b, pfPath.NextHopGroup(nextHopGroupName).Config(), nhg)
	gnmi.BatchDelete(b, d.Interface(i3.GetName()).Config())
	b.Set(t, dut)
}

// reconfigureDUTPort3 re-adds the configuration for DUT port 3.
func reconfigureDUTPort3(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p3 := dut.Port(t, "port3")
	i3 := &oc.Interface{Name: ygot.String(p3.Name())}
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), configInterface(i3, &dutEgress2))
}

// configureATE configures the ATE with the necessary interfaces and traffic flows.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	topo := gosnappi.NewConfig()

	// Ingress port
	p1 := ate.Port(t, "port1")
	topo.Ports().Add().SetName(p1.ID())
	iDut1 := topo.Devices().Add().SetName(ateIngress.Name)
	iEth1 := iDut1.Ethernets().Add().SetName(ateIngress.Name + ".Eth").SetMac(ateIngress.MAC)
	iEth1.Connection().SetPortName(p1.ID())
	iIP1 := iEth1.Ipv4Addresses().Add().SetName(ateIngress.Name + ".IPv4")
	iIP1.SetAddress(ateIngress.IPv4).SetGateway(dutIngress.IPv4).SetPrefix(uint32(ateIngress.IPv4Len))

	// Egress port 1
	p2 := ate.Port(t, "port2")
	topo.Ports().Add().SetName(p2.ID())
	eDut1 := topo.Devices().Add().SetName(ateEgress1.Name)
	eEth1 := eDut1.Ethernets().Add().SetName(ateEgress1.Name + ".Eth").SetMac(ateEgress1.MAC)
	eEth1.Connection().SetPortName(p2.ID())
	eIP1 := eEth1.Ipv4Addresses().Add().SetName(ateEgress1.Name + ".IPv4")
	eIP1.SetAddress(ateEgress1.IPv4).SetGateway(dutEgress1.IPv4).SetPrefix(uint32(ateEgress1.IPv4Len))

	// Egress port 2
	p3 := ate.Port(t, "port3")
	topo.Ports().Add().SetName(p3.ID())
	eDut2 := topo.Devices().Add().SetName(ateEgress2.Name)
	eEth2 := eDut2.Ethernets().Add().SetName(ateEgress2.Name + ".Eth").SetMac(ateEgress2.MAC)
	eEth2.Connection().SetPortName(p3.ID())
	eIP2 := eEth2.Ipv4Addresses().Add().SetName(ateEgress2.Name + ".IPv4")
	eIP2.SetAddress(ateEgress2.IPv4).SetGateway(dutEgress2.IPv4).SetPrefix(uint32(ateEgress2.IPv4Len))

	// Traffic flow
	flow := topo.Flows().Add().SetName("flow")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{iIP1.Name()}).SetRxNames([]string{eIP1.Name(), eIP2.Name()})
	flow.Size().SetFixed(512)
	flow.Rate().SetPps(pps)
	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(ateIngress.MAC)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(ateIngress.IPv4)
	v4.Dst().SetValue("198.51.100.1") // An arbitrary destination
	tcp := flow.Packet().Add().Tcp()
	tcp.SrcPort().SetValue(50001)
	tcp.DstPort().SetValue(50002)

	return topo
}

// verifyTraffic verifies that traffic is forwarded as expected.
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config) {
	otg := ate.OTG()
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)

	otgutils.LogFlowMetrics(t, otg, topo)
}

// TestAddRemoveInterface is the main test function.
func TestAddRemoveInterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var topo gosnappi.Config

	// Step 1: Configure the initial DUT and ATE state.
	t.Run("Configure initial state", func(t *testing.T) {
		configureDUT(t, dut)
		topo = configureATE(t, ate)
		ate.OTG().PushConfig(t, topo)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	})

	// Step 2: Verify traffic with a single interface in the next-hop-group.
	t.Run("Verify with single interface", func(t *testing.T) {
		verifyTraffic(t, ate, topo)
		totalSent := pps * uint64(trafficDuration.Seconds())
		totalRecv := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow("flow").State()).GetCounters().GetInPkts()
		if totalRecv < totalSent {
			t.Errorf("FAIL: Less packets received than sent. Sent: %d, Recv: %d", totalSent, totalRecv)
		}

		p2Stats := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port2").ID()).State())
		p3Stats := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port3").ID()).State())
		if p2Stats.GetCounters().GetInFrames() == 0 {
			t.Errorf("FAIL: No traffic received on port 2")
		}
		if p3Stats.GetCounters().GetInFrames() > 0 {
			t.Errorf("FAIL: Traffic received on port 3, expected 0. Got: %d", p3Stats.GetCounters().GetInFrames())
		}
	})

	// Step 3: Add a second interface to the next-hop-group and verify traffic.
	t.Run("Add interface and verify", func(t *testing.T) {
		addInterfaceToNHG(t, dut)
		verifyTraffic(t, ate, topo)
		totalSent := pps * uint64(trafficDuration.Seconds())
		totalRecv := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow("flow").State()).GetCounters().GetInPkts()
		if totalRecv < totalSent {
			t.Errorf("FAIL: Less packets received than sent. Sent: %d, Recv: %d", totalSent, totalRecv)
		}

		p2Stats := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port2").ID()).State())
		p3Stats := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port3").ID()).State())
		if p2Stats.GetCounters().GetInFrames() == 0 || p3Stats.GetCounters().GetInFrames() == 0 {
			t.Errorf("FAIL: Traffic not load-balanced between egress ports. p2: %d, p3: %d", p2Stats.GetCounters().GetInFrames(), p3Stats.GetCounters().GetInFrames())
		}
		t.Logf("Traffic load-balanced: %d packets on port 2, %d packets on port 3", p2Stats.GetCounters().GetInFrames(), p3Stats.GetCounters().GetInFrames())
	})

	// Step 4: Remove the second interface from the next-hop-group and verify traffic.
	t.Run("Remove interface and verify", func(t *testing.T) {
		removeInterfaceFromNHG(t, dut)
		verifyTraffic(t, ate, topo)
		totalSent := pps * uint64(trafficDuration.Seconds())
		totalRecv := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow("flow").State()).GetCounters().GetInPkts()
		if totalRecv < totalSent {
			t.Errorf("FAIL: Less packets received than sent. Sent: %d, Recv: %d", totalSent, totalRecv)
		}

		p2Stats := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port2").ID()).State())
		p3Stats := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port3").ID()).State())
		if p2Stats.GetCounters().GetInFrames() == 0 {
			t.Errorf("FAIL: No traffic received on port 2")
		}
		if p3Stats.GetCounters().GetInFrames() > 0 {
			t.Errorf("FAIL: Traffic received on port 3, expected 0. Got: %d", p3Stats.GetCounters().GetInFrames())
		}
		reconfigureDUTPort3(t, dut)
	})
}
