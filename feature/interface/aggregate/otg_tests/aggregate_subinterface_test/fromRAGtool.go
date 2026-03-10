package aggregate_subinterface_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	lag1Name        = "lag1"
	lag2Name        = "lag2"
	vlan10          = 10
	vlan20          = 20
	mtu             = 9216
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	testNI          = "test-instance"
	flowRatePercent = 5
	frameSize       = 9210
)

var (
	dutPorts = map[string]string{
		"port1": "port1",
		"port2": "port2",
		"port3": "port3",
		"port4": "port4",
	}

	atePorts = map[string]string{
		"port1": "port1",
		"port2": "port2",
		"port3": "port3",
		"port4": "port4",
	}

	lag1Members = []string{"port1", "port2"}
	lag2Members = []string{"port3", "port4"}

	subinterfaces = map[string][]struct {
		vlanID  uint16
		dutIPv4 string
		ateIPv4 string
		dutIPv6 string
		ateIPv6 string
	}{
		lag1Name: {
			{vlanID: vlan10, dutIPv4: "198.51.100.1", ateIPv4: "198.51.100.2", dutIPv6: "2001:db8::1", ateIPv6: "2001:db8::2"},
			{vlanID: vlan20, dutIPv4: "198.51.100.5", ateIPv4: "198.51.100.6", dutIPv6: "2001:db8::5", ateIPv6: "2001:db8::6"},
		},
		lag2Name: {
			{vlanID: vlan10, dutIPv4: "198.51.100.9", ateIPv4: "198.51.100.10", dutIPv6: "2001:db8::9", ateIPv6: "2001:db8::10"},
			{vlanID: vlan20, dutIPv4: "198.51.100.13", ateIPv4: "198.51.100.14", dutIPv6: "2001:db8::13", ateIPv6: "2001:db8::14"},
		},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUT configures aggregate interfaces, subinterfaces, and LACP on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, niName string, minLinks uint16) {
	t.Helper()
	d := gnmi.OC()
	var config oc.Root
	if niName != deviations.DefaultNetworkInstance(dut) {
		ni := config.GetOrCreateNetworkInstance(niName)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	}

	for i := 1; i <= 4; i++ {
		portName := fmt.Sprintf("port%d", i)
		dutPort := dut.Port(t, portName)
		intf := config.GetOrCreateInterface(dutPort.Name())
		intf.Enabled = ygot.Bool(true)
		intf.Mtu = ygot.Uint16(mtu)
		intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		e := intf.GetOrCreateEthernet()
		if i <= 2 {
			e.AggregateId = ygot.String(lag1Name)
		} else {
			e.AggregateId = ygot.String(lag2Name)
		}
	}

	for _, lagName := range []string{lag1Name, lag2Name} {
		lag := config.GetOrCreateInterface(lagName)
		lag.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		lag.Mtu = ygot.Uint16(mtu)
		agg := lag.GetOrCreateAggregation()
		agg.LagType = oc.IfAggregate_AggregationType_LACP
		agg.MinLinks = minLinks

		lacp := config.GetOrCreateLacp().GetOrCreateInterface(lagName)
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE

		for _, sub := range subinterfaces[lagName] {
			subif := lag.GetOrCreateSubinterface(uint32(sub.vlanID))
			subif.GetOrCreateVlan().VlanId = sub.vlanID
			s4 := subif.GetOrCreateIpv4().GetOrCreateAddress(sub.dutIPv4)
			s4.PrefixLength = ygot.Uint8(ipv4PrefixLen)
			s6 := subif.GetOrCreateIpv6().GetOrCreateAddress(sub.dutIPv6)
			s6.PrefixLength = ygot.Uint8(ipv6PrefixLen)
			if niName != deviations.DefaultNetworkInstance(dut) {
				ni := config.GetOrCreateNetworkInstance(niName)
				niIntf := ni.GetOrCreateInterface(fmt.Sprintf("%s.%d", lagName, sub.vlanID))
				niIntf.Subinterface = ygot.Uint32(sub.vlanID)
				niIntf.Interface = ygot.String(lagName)
			}
		}
	}
	gnmi.Replace(t, dut, d.Config(), &config)
}

// configureATE configures the ATE with LAGs, subinterfaces, and traffic flows.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := ate.OTG().NewConfig(t)

	// Configure Port to LAG mapping
	lag1 := top.Lags().Add().SetName(lag1Name)
	lag1.Protocol().LACP().SetActorActivity(gosnappi.LagProtocolLacpActivity.PASSIVE)
	for _, portName := range lag1Members {
		lag1.Ports().Add().SetPortName(ate.Port(t, portName).ID())
	}
	lag2 := top.Lags().Add().SetName(lag2Name)
	lag2.Protocol().LACP().SetActorActivity(gosnappi.LagProtocolLacpActivity.PASSIVE)
	for _, portName := range lag2Members {
		lag2.Ports().Add().SetPortName(ate.Port(t, portName).ID())
	}

	// Configure devices and interfaces
	d1 := top.Devices().Add().SetName("ateLag1")
	eth1 := d1.Ethernets().Add().SetName("ethLag1")
	eth1.Connection().SetLagName(lag1.Name())
	eth1.SetMtu(mtu)

	d2 := top.Devices().Add().SetName("ateLag2")
	eth2 := d2.Ethernets().Add().SetName("ethLag2")
	eth2.Connection().SetLagName(lag2.Name())
	eth2.SetMtu(mtu)

	// Configure subinterfaces for LAG1
	for _, sub := range subinterfaces[lag1Name] {
		vlan := eth1.Vlans().Add().SetName(fmt.Sprintf("%s.vlan%d", d1.Name(), sub.vlanID))
		vlan.SetId(uint32(sub.vlanID))
		vlan.Ipv4Addresses().Add().SetName(fmt.Sprintf("%s-v4", vlan.Name())).SetAddress(sub.ateIPv4).SetGateway(sub.dutIPv4).SetPrefix(uint32(ipv4PrefixLen))
		vlan.Ipv6Addresses().Add().SetName(fmt.Sprintf("%s-v6", vlan.Name())).SetAddress(sub.ateIPv6).SetGateway(sub.dutIPv6).SetPrefix(uint32(ipv6PrefixLen))
	}

	// Configure subinterfaces for LAG2
	for _, sub := range subinterfaces[lag2Name] {
		vlan := eth2.Vlans().Add().SetName(fmt.Sprintf("%s.vlan%d", d2.Name(), sub.vlanID))
		vlan.SetId(uint32(sub.vlanID))
		vlan.Ipv4Addresses().Add().SetName(fmt.Sprintf("%s-v4", vlan.Name())).SetAddress(sub.ateIPv4).SetGateway(sub.dutIPv4).SetPrefix(uint32(ipv4PrefixLen))
		vlan.Ipv6Addresses().Add().SetName(fmt.Sprintf("%s-v6", vlan.Name())).SetAddress(sub.ateIPv6).SetGateway(sub.dutIPv6).SetPrefix(uint32(ipv6PrefixLen))
	}

	// Configure traffic flows
	f1 := top.Flows().Add().SetName("v4_vlan10")
	f1.Metrics().SetEnable(true)
	f1.TxRx().Device().SetTxNames([]string{"ateLag1.vlan10-v4"}).SetRxNames([]string{"ateLag2.vlan10-v4"})
	f1.Size().SetFixed(frameSize)
	f1.Rate().SetPercentage(flowRatePercent)

	f2 := top.Flows().Add().SetName("v4_vlan20")
	f2.Metrics().SetEnable(true)
	f2.TxRx().Device().SetTxNames([]string{"ateLag1.vlan20-v4"}).SetRxNames([]string{"ateLag2.vlan20-v4"})
	f2.Size().SetFixed(frameSize)
	f2.Rate().SetPercentage(flowRatePercent)

	f3 := top.Flows().Add().SetName("v6_vlan10")
	f3.Metrics().SetEnable(true)
	f3.TxRx().Device().SetTxNames([]string{"ateLag1.vlan10-v6"}).SetRxNames([]string{"ateLag2.vlan10-v6"})
	f3.Size().SetFixed(frameSize)
	f3.Rate().SetPercentage(flowRatePercent)

	f4 := top.Flows().Add().SetName("v6_vlan20")
	f4.Metrics().SetEnable(true)
	f4.TxRx().Device().SetTxNames([]string{"ateLag1.vlan20-v6"}).SetRxNames([]string{"ateLag2.vlan20-v6"})
	f4.Size().SetFixed(frameSize)
	f4.Rate().SetPercentage(flowRatePercent)

	return top
}

// verifyLACPState validates LACP state on both DUT and ATE.
func verifyLACPState(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Helper()
	otg := ate.OTG()

	dutSystemID := gnmi.Get(t, dut, gnmi.OC().Lacp().SystemIdMac().State())

	for _, lagName := range []string{lag1Name, lag2Name} {
		ateLagState := gnmi.Get(t, otg, gnmi.OTG().Lag(lagName).State())
		ateSystemID := ateLagState.GetLacp().GetSystemId()

		dutMemberPorts := gnmi.Get(t, dut, gnmi.OC().Interface(lagName).Aggregation().Member().State())
		for _, memberName := range dutMemberPorts {
			dutMemberState := gnmi.Get(t, dut, gnmi.OC().Lacp().Interface(memberName).State())
			if dutMemberState.GetPartnerId() != ateSystemID {
				t.Errorf("DUT port %s PartnerID: got %s, want %s", memberName, dutMemberState.GetPartnerId(), ateSystemID)
			}
			if !dutMemberState.GetCollecting() || !dutMemberState.GetDistributing() {
				t.Errorf("DUT port %s LACP state is not collecting or distributing", memberName)
			}
		}

		for _, member := range ateLagState.GetMembers() {
			if member.GetPartnerId() != dutSystemID {
				t.Errorf("ATE member %s PartnerID: got %s, want %s", member.GetName(), member.GetPartnerId(), dutSystemID)
			}
			if !member.GetLacp().GetCollecting() || !member.GetLacp().GetDistributing() {
				t.Errorf("ATE member %s LACP state is not collecting or distributing", member.GetName())
			}
		}
	}
}

// verifyTraffic confirms that traffic flows without significant loss.
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	otg := ate.OTG()
	otg.StartTraffic(t)
	time.Sleep(60 * time.Second)
	otg.StopTraffic(t)

	for _, flowName := range []string{"v4_vlan10", "v4_vlan20", "v6_vlan10", "v6_vlan20"} {
		flowMetrics := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
		txPackets := float32(flowMetrics.GetCounters().GetOutPkts())
		rxPackets := float32(flowMetrics.GetCounters().GetInPkts())
		if txPackets == 0 {
			t.Errorf("Flow %s had no packets sent", flowName)
			continue
		}
		lossPct := (txPackets - rxPackets) * 100 / txPackets
		if lossPct > 1 {
			t.Errorf("Flow %s has unexpected packet loss: got %f%%, want < 1%%", flowName, lossPct)
		}
	}
}

// verifyCounters checks DUT interface counters for transmitted and discarded packets.
func verifyCounters(t *testing.T, dut *ondatra.DUTDevice, txPkts uint64) {
	for _, lagName := range []string{lag1Name, lag2Name} {
		for _, sub := range subinterfaces[lagName] {
			subIntfName := fmt.Sprintf("%s.%d", lagName, sub.vlanID)
			counters := gnmi.Get(t, dut, gnmi.OC().Interface(subIntfName).State()).GetCounters()
			if counters.GetInPkts() == 0 || counters.GetOutPkts() == 0 {
				t.Errorf("Subinterface %s has zero in/out packets", subIntfName)
			}
			if counters.GetInDiscards() > uint64(0.01*float64(txPkts)) {
				t.Errorf("Subinterface %s has high input discards: %d", subIntfName, counters.GetInDiscards())
			}
			if counters.GetOutDiscards() > uint64(0.01*float64(txPkts)) {
				t.Errorf("Subinterface %s has high output discards: %d", subIntfName, counters.GetOutDiscards())
			}
		}
	}
}

func TestAggregateSubinterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	t.Run("RT-5.14.1: Aggregate interface flap using min-link", func(t *testing.T) {
		configureDUT(t, dut, deviations.DefaultNetworkInstance(dut), 2)
		ateConfig := configureATE(t, ate)
		otg.PushConfig(t, ateConfig)
		otg.StartProtocols(t)

		for i := 0; i < 10; i++ {
			t.Logf("Flap iteration #%d", i+1)
			gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
			gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)

			gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, dutPorts["port1"]).Name()).Enabled().Config(), false)
			gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, dutPorts["port3"]).Name()).Enabled().Config(), false)

			gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
			gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)

			gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, dutPorts["port1"]).Name()).Enabled().Config(), true)
			gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, dutPorts["port3"]).Name()).Enabled().Config(), true)
		}

		gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
		gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)

		verifyTraffic(t, ate)

		var totalTxPkts uint64
		for _, flow := range gnmi.GetAll(t, otg, gnmi.OTG().FlowAny().State()) {
			totalTxPkts += flow.GetCounters().GetOutPkts()
		}
		verifyCounters(t, dut, totalTxPkts)
		verifyLACPState(t, dut, ate)

		for _, name := range []string{lag1Name, lag2Name, dutPorts["port1"], dutPorts["port2"], dutPorts["port3"], dutPorts["port4"]} {
			if mtuVal := gnmi.Get(t, dut, gnmi.OC().Interface(name).State()).GetMtu(); mtuVal != mtu {
				t.Errorf("DUT interface %s MTU: got %d, want %d", name, mtuVal, mtu)
			}
		}
	})

	t.Run("RT-5.14.2: Aggregate sub-interface in default Network Instance (NI)", func(t *testing.T) {
		configureDUT(t, dut, deviations.DefaultNetworkInstance(dut), 1)
		ateConfig := configureATE(t, ate)
		otg.PushConfig(t, ateConfig)
		otg.StartProtocols(t)

		gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
		gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)

		verifyTraffic(t, ate)
		var totalTxPkts uint64
		for _, flow := range gnmi.GetAll(t, otg, gnmi.OTG().FlowAny().State()) {
			totalTxPkts += flow.GetCounters().GetOutPkts()
		}
		verifyCounters(t, dut, totalTxPkts)
		verifyLACPState(t, dut, ate)

		for _, name := range []string{lag1Name, lag2Name, dutPorts["port1"], dutPorts["port2"], dutPorts["port3"], dutPorts["port4"]} {
			if mtuVal := gnmi.Get(t, dut, gnmi.OC().Interface(name).State()).GetMtu(); mtuVal != mtu {
				t.Errorf("DUT interface %s MTU: got %d, want %d", name, mtuVal, mtu)
			}
		}
	})

	t.Run("RT-5.14.3: Aggregate sub-interface in non-default Network Instance (NI)", func(t *testing.T) {
		configureDUT(t, dut, testNI, 1)
		ateConfig := configureATE(t, ate)
		otg.PushConfig(t, ateConfig)
		otg.StartProtocols(t)

		gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
		gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)

		verifyTraffic(t, ate)
		var totalTxPkts uint64
		for _, flow := range gnmi.GetAll(t, otg, gnmi.OTG().FlowAny().State()) {
			totalTxPkts += flow.GetCounters().GetOutPkts()
		}
		verifyCounters(t, dut, totalTxPkts)
		verifyLACPState(t, dut, ate)

		for _, name := range []string{lag1Name, lag2Name, dutPorts["port1"], dutPorts["port2"], dutPorts["port3"], dutPorts["port4"]} {
			if mtuVal := gnmi.Get(t, dut, gnmi.OC().Interface(name).State()).GetMtu(); mtuVal != mtu {
				t.Errorf("DUT interface %s MTU: got %d, want %d", name, mtuVal, mtu)
			}
		}
	})
}
