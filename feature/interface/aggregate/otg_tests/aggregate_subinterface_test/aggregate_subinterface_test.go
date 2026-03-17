package aggregate_subinterface_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

const (
	vlan10 = 10
	vlan20 = 20
	// mtu             = 9216
	mtu             = 1055
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	testNI          = "test-instance"
	flowRatePercent = 5
	// frameSize       = 9210
	frameSize = 1024
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
)

// subifConfig describes an IP subinterface configuration entry.
type subifConfig struct {
	vlanID  uint16
	dutIPv4 string
	ateIPv4 string
	dutIPv6 string
	ateIPv6 string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUT configures aggregate interfaces, subinterfaces, and LACP on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, niName string, minLinks uint16, lag1Name, lag2Name string, subs map[string][]subifConfig) {
	t.Helper()
	// d := gnmi.OC()
	//d := &oc.Root{}
	var config oc.Root

	//ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	// if niName != deviations.DefaultNetworkInstance(dut) {
	// 	ni := config.GetOrCreateNetworkInstance(niName)
	// 	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	// }

	batch := &gnmi.SetBatch{}
	for i := 1; i <= 4; i++ {
		portName := fmt.Sprintf("port%d", i)
		dutPort := dut.Port(t, portName)
		intf := config.GetOrCreateInterface(dutPort.Name())
		intf.Enabled = ygot.Bool(true)
		// intf.Mtu = ygot.Uint16(mtu)
		intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		e := intf.GetOrCreateEthernet()
		if i <= 2 {
			e.AggregateId = ygot.String(lag1Name)
		} else {
			e.AggregateId = ygot.String(lag2Name)
		}
		gnmi.BatchReplace(batch, gnmi.OC().Interface(dutPort.Name()).Config(), intf)
	}

	for _, lagName := range []string{lag1Name, lag2Name} {
		lag := config.GetOrCreateInterface(lagName)
		lag.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		// lag.Mtu = ygot.Uint16(mtu)
		// Add VLAN trunk config HERE instead of on member ports:(5 lines added later - not sure)
		// lagEth := lag.GetOrCreateEthernet()
		// lagSv := lagEth.GetOrCreateSwitchedVlan()
		// lagSv.TrunkVlans = []oc.Interface_Ethernet_SwitchedVlan_TrunkVlans_Union{
		// 	oc.UnionUint16(vlan10),
		// 	oc.UnionUint16(vlan20),
		// }

		agg := lag.GetOrCreateAggregation()
		agg.LagType = oc.IfAggregate_AggregationType_LACP
		agg.MinLinks = ygot.Uint16(minLinks)

		lacp := config.GetOrCreateLacp().GetOrCreateInterface(lagName)
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
		if deviations.RequireRoutedSubinterface0(dut) {
			s0 := lag.GetOrCreateSubinterface(0)
			s0v4 := s0.GetOrCreateIpv4()
			s0v6 := s0.GetOrCreateIpv6()
			if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
				s0v4.Enabled = ygot.Bool(true)
			}
			if deviations.InterfaceEnabled(dut) {
				s0v6.Enabled = ygot.Bool(true)
			}
		}
		for _, sub := range subs[lagName] {
			subif := lag.GetOrCreateSubinterface(uint32(sub.vlanID))
			subif.Enabled = ygot.Bool(true)
			if deviations.DeprecatedVlanID(dut) {
				subif.GetOrCreateVlan().VlanId = oc.UnionUint16(sub.vlanID)
			} else {
				subif.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(sub.vlanID)
			}
			ipv4 := subif.GetOrCreateIpv4()
			ipv4.Enabled = ygot.Bool(true)
			s4 := ipv4.GetOrCreateAddress(sub.dutIPv4)
			s4.PrefixLength = ygot.Uint8(ipv4PrefixLen)
			// ipv4.Mtu = ygot.Uint16(mtu)
			ipv6 := subif.GetOrCreateIpv6()
			ipv6.Enabled = ygot.Bool(true)
			s6 := ipv6.GetOrCreateAddress(sub.dutIPv6)
			s6.PrefixLength = ygot.Uint8(ipv6PrefixLen)
			// ipv6.Mtu = ygot.Uint32(mtu)
			if niName != deviations.DefaultNetworkInstance(dut) {
				ni := config.GetOrCreateNetworkInstance(niName)
				niIntf := ni.GetOrCreateInterface(fmt.Sprintf("%s.%d", lagName, sub.vlanID))
				niIntf.Subinterface = ygot.Uint32(uint32(sub.vlanID))
				niIntf.Interface = ygot.String(lagName)
				gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(niName).Config(), ni)
			}
		}
		gnmi.BatchReplace(batch, gnmi.OC().Interface(lagName).Config(), lag)
		gnmi.BatchReplace(batch, gnmi.OC().Lacp().Interface(lagName).Config(), lacp)
		gnmi.BatchReplace(batch, gnmi.OC().Interface(lagName).Subinterface(10).Ipv4().Mtu().Config(), mtu)
		gnmi.BatchReplace(batch, gnmi.OC().Interface(lagName).Subinterface(10).Ipv6().Mtu().Config(), mtu)
	}
	// gnmi.Replace(t, dut, d.Config(), &config)
	batch.Set(t, dut)
}

// configureATE configures the ATE with LAGs, subinterfaces, and traffic flows.
func configureATE(t *testing.T, ate *ondatra.ATEDevice, lag1Name, lag2Name string, subs map[string][]subifConfig) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	// Configure Port to LAG mapping
	lag1 := top.Lags().Add().SetName(lag1Name)
	lag1.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId("02:00:01:01:01:01")
	for i, portName := range lag1Members {
		p := ate.Port(t, portName)
		top.Ports().Add().SetName(p.ID())
		lagPort := lag1.Ports().Add().SetPortName(p.ID())
		lagPort.Ethernet().SetMac("02:00:01:01:01:01").SetName(lag1Name + "-" + p.ID())
		lagPort.Lacp().SetActorActivity("passive").SetActorPortNumber(uint32(i) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
	}
	lag2 := top.Lags().Add().SetName(lag2Name)
	lag2.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId("02:00:02:01:01:01")
	for i, portName := range lag2Members {
		p := ate.Port(t, portName)
		top.Ports().Add().SetName(p.ID())
		lagPort := lag2.Ports().Add().SetPortName(p.ID())
		lagPort.Ethernet().SetMac("02:00:02:01:01:01").SetName(lag2Name + "-" + p.ID())
		lagPort.Lacp().SetActorActivity("passive").SetActorPortNumber(uint32(i) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
	}

	// Configure subinterfaces for LAG1
	for _, sub := range subs[lag1Name] {
		dev := top.Devices().Add().SetName(fmt.Sprintf("ateLag1Vlan%d", sub.vlanID))
		eth := dev.Ethernets().Add().SetName(fmt.Sprintf("ethLag1Vlan%d", sub.vlanID)).SetMac(fmt.Sprintf("02:00:01:00:00:%02x", sub.vlanID))
		eth.Connection().SetLagName(lag1.Name())
		// eth.SetMtu(uint32(mtu))
		eth.Vlans().Add().SetName(fmt.Sprintf("ateLag1.vlan%d", sub.vlanID)).SetId(uint32(sub.vlanID))
		eth.Ipv4Addresses().Add().SetName(fmt.Sprintf("ateLag1.vlan%d-v4", sub.vlanID)).SetAddress(sub.ateIPv4).SetGateway(sub.dutIPv4).SetPrefix(uint32(ipv4PrefixLen))
		eth.Ipv6Addresses().Add().SetName(fmt.Sprintf("ateLag1.vlan%d-v6", sub.vlanID)).SetAddress(sub.ateIPv6).SetGateway(sub.dutIPv6).SetPrefix(uint32(ipv6PrefixLen))
	}

	// Configure subinterfaces for LAG2
	for _, sub := range subs[lag2Name] {
		dev := top.Devices().Add().SetName(fmt.Sprintf("ateLag2Vlan%d", sub.vlanID))
		eth := dev.Ethernets().Add().SetName(fmt.Sprintf("ethLag2Vlan%d", sub.vlanID)).SetMac(fmt.Sprintf("02:00:02:00:00:%02x", sub.vlanID))
		eth.Connection().SetLagName(lag2.Name())
		// eth.SetMtu(uint32(mtu))
		eth.Vlans().Add().SetName(fmt.Sprintf("ateLag2.vlan%d", sub.vlanID)).SetId(uint32(sub.vlanID))
		eth.Ipv4Addresses().Add().SetName(fmt.Sprintf("ateLag2.vlan%d-v4", sub.vlanID)).SetAddress(sub.ateIPv4).SetGateway(sub.dutIPv4).SetPrefix(uint32(ipv4PrefixLen))
		eth.Ipv6Addresses().Add().SetName(fmt.Sprintf("ateLag2.vlan%d-v6", sub.vlanID)).SetAddress(sub.ateIPv6).SetGateway(sub.dutIPv6).SetPrefix(uint32(ipv6PrefixLen))
	}

	// Configure traffic flows
	top.Flows().Clear()

	f1 := top.Flows().Add().SetName("v4_vlan10")
	f1.Metrics().SetEnable(true)
	f1.TxRx().Device().SetTxNames([]string{"ateLag1.vlan10-v4"}).SetRxNames([]string{"ateLag2.vlan10-v4"})
	f1.Size().SetFixed(frameSize)
	f1.Rate().SetPercentage(flowRatePercent)
	e := f1.Packet().Add().Ethernet()
	e.Src().SetValue("02:00:01:01:01:0a")
	f1.Packet().Add().Vlan().Id().SetValue(10)
	f1v4 := f1.Packet().Add().Ipv4()
	f1v4.Src().SetValue("198.51.100.2")
	f1v4.Dst().SetValue("198.51.100.10")

	f2 := top.Flows().Add().SetName("v4_vlan20")
	f2.Metrics().SetEnable(true)
	f2.TxRx().Device().SetTxNames([]string{"ateLag1.vlan20-v4"}).SetRxNames([]string{"ateLag2.vlan20-v4"})
	f2.Size().SetFixed(frameSize)
	f2.Rate().SetPercentage(flowRatePercent)
	e2 := f2.Packet().Add().Ethernet()
	e2.Src().SetValue("02:00:01:01:01:14")
	f2.Packet().Add().Vlan().Id().SetValue(20)
	f2v4 := f2.Packet().Add().Ipv4()
	f2v4.Src().SetValue("198.51.100.6")
	f2v4.Dst().SetValue("198.51.100.14")

	f3 := top.Flows().Add().SetName("v6_vlan10")
	f3.Metrics().SetEnable(true)
	f3.TxRx().Device().SetTxNames([]string{"ateLag1.vlan10-v6"}).SetRxNames([]string{"ateLag2.vlan10-v6"})
	f3.Size().SetFixed(frameSize)
	f3.Rate().SetPercentage(flowRatePercent)
	e3 := f3.Packet().Add().Ethernet()
	e3.Src().SetValue("02:00:01:01:01:0a")
	f3.Packet().Add().Vlan().Id().SetValue(10)
	f3v6 := f3.Packet().Add().Ipv6()
	f3v6.Src().SetValue("2001:db8::2")
	f3v6.Dst().SetValue("2001:db8::0a")

	f4 := top.Flows().Add().SetName("v6_vlan20")
	f4.Metrics().SetEnable(true)
	f4.TxRx().Device().SetTxNames([]string{"ateLag1.vlan20-v6"}).SetRxNames([]string{"ateLag2.vlan20-v6"})
	f4.Size().SetFixed(frameSize)
	f4.Rate().SetPercentage(flowRatePercent)
	e4 := f4.Packet().Add().Ethernet()
	e4.Src().SetValue("02:00:01:01:01:14")
	f4.Packet().Add().Vlan().Id().SetValue(20)
	f4v6 := f4.Packet().Add().Ipv6()
	f4v6.Src().SetValue("2001:db8::6")
	f4v6.Dst().SetValue("2001:db8::0e")

	return top
}

// verifyLACPState validates LACP state on both DUT and ATE.
func verifyLACPState(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, lag1Name, lag2Name string) {
	t.Helper()
	otg := ate.OTG()

	lagMapping := map[string][]string{
		lag1Name: lag1Members,
		lag2Name: lag2Members,
	}

	for lagName, members := range lagMapping {
		for _, portName := range members {
			dutPort := dut.Port(t, portName)
			atePort := ate.Port(t, portName)

			dutLACP := gnmi.Get(t, dut, gnmi.OC().Lacp().Interface(lagName).Member(dutPort.Name()).State())
			ateLACP := gnmi.Get(t, otg, gnmi.OTG().Lacp().LagMember(atePort.ID()).State())

			if ateLACP.PartnerId == nil || dutLACP.SystemId == nil || !strings.EqualFold(*ateLACP.PartnerId, *dutLACP.SystemId) {
				t.Errorf("LAG %s port %s: ATE partner-id (%v) did not match DUT system-id (%v)", lagName, portName, ateLACP.PartnerId, dutLACP.SystemId)
			}

			if dutLACP.PartnerId == nil || ateLACP.SystemId == nil || !strings.EqualFold(*dutLACP.PartnerId, *ateLACP.SystemId) {
				t.Errorf("LAG %s port %s: DUT partner-id (%v) did not match ATE system-id (%v)", lagName, portName, dutLACP.PartnerId, ateLACP.SystemId)
			}

			if !dutLACP.GetCollecting() || !dutLACP.GetDistributing() {
				t.Errorf("LAG %s DUT port %s: not collecting/distributing", lagName, dutPort.Name())
			}

			if !ateLACP.GetCollecting() || !ateLACP.GetDistributing() {
				t.Errorf("LAG %s ATE port %s: not collecting/distributing", lagName, atePort.ID())
			}
		}
	}
}

// verifyTraffic confirms that traffic flows without significant loss.
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	otg := ate.OTG()
	// t.Logf("\n\n\n\n\nRDelay 60s before running traffic\n\n\n\n\n")
	otg.StartTraffic(t)
	time.Sleep(60 * time.Second)
	otg.StopTraffic(t)
	config := otg.GetConfig(t)
	t.Logf("\n\n\n\n\n%v\n\n\n\n\n", config) // Debug print of the config to help with troubleshooting if the test fails. Can be removed later.

	otgutils.LogFlowMetrics(t, otg, config)
	otgutils.LogPortMetrics(t, otg, config)
	otgutils.LogLACPMetrics(t, otg, config)

	for _, flow := range config.Flows().Items() {
		flowMetrics := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).State())
		// flowMetrics.GetCounters().
		txPackets := float32(flowMetrics.GetCounters().GetOutPkts())
		rxPackets := float32(flowMetrics.GetCounters().GetInPkts())
		if txPackets == 0 {
			t.Errorf("Flow %s had no packets sent", flow.Name())
			continue
		}
		lossPct := (txPackets - rxPackets) * 100 / txPackets
		if lossPct > 1 {
			t.Errorf("Flow %s has unexpected packet loss: got %f%%, want < 1%%", flow.Name(), lossPct)
		}
	}
}

// verifyCounters checks DUT interface counters for transmitted and discarded packets.
func verifyCounters(t *testing.T, dut *ondatra.DUTDevice, txPkts uint64, lag1Name, lag2Name string, subs map[string][]subifConfig) {
	ingress := true
	egress := false
	for lagName, direction := range map[string]bool{
		lag1Name: ingress,
		lag2Name: egress,
	} {
		lagState := gnmi.Get(t, dut, gnmi.OC().Interface(lagName).State())
		if direction == ingress { // check for in pkts
			//
			counters := lagState.GetCounters()
			t.Logf("%d", counters.GetInPkts())
			// if counters.GetInPkts() == 0 || counters.GetOutPkts() == 0 {
			// 	t.Errorf("Lag %s has zero in/out packets", subIntfName)
			// }
			// if counters.GetInDiscards() > uint64(0.01*float64(txPkts)) {
			// 	t.Errorf("Subinterface %s has high input discards: %d", subIntfName, counters.GetInDiscards())
			// }
			// if counters.GetOutDiscards() > uint64(0.01*float64(txPkts)) {
			// 	t.Errorf("Subinterface %s has high output discards: %d", subIntfName, counters.GetOutDiscards())
		}
		if direction == egress { // check for in pkts
			counters := lagState.GetCounters()
			t.Logf("%d", counters.GetOutPkts())
		}
	}
}

func awaitLAGMembersCollectingDistributing(t *testing.T, dut *ondatra.DUTDevice, lagName string, members []string) {
	t.Helper()

	for _, portName := range members {
		dutPort := dut.Port(t, portName)
		memberPath := gnmi.OC().Lacp().Interface(lagName).Member(dutPort.Name())

		gnmi.Await(t, dut, memberPath.Collecting().State(), 1*time.Minute, true)
		gnmi.Await(t, dut, memberPath.Distributing().State(), 1*time.Minute, true)
		t.Logf("%s/%s collecting=%v distributing=%v sync=%v",
			lagName, dutPort.Name(),
			gnmi.Get(t, dut, memberPath.Collecting().State()),
			gnmi.Get(t, dut, memberPath.Distributing().State()),
			gnmi.Get(t, dut, memberPath.Synchronization().State()),
		)
	}
}

func TestAggregateSubinterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	// Derive vendor-appropriate LAG interface names.
	lag1Name := netutil.NextAggregateInterface(t, dut)
	numRE := regexp.MustCompile(`\d+`)
	start, _ := strconv.Atoi(numRE.FindString(lag1Name))
	lag2Name := numRE.ReplaceAllString(lag1Name, strconv.Itoa(start+1))
	t.Logf("Using LAG names: %s, %s", lag1Name, lag2Name)

	subs := map[string][]subifConfig{
		lag1Name: {
			{vlanID: vlan10, dutIPv4: "198.51.100.1", ateIPv4: "198.51.100.2", dutIPv6: "2001:db8::1", ateIPv6: "2001:db8::2"},
			{vlanID: vlan20, dutIPv4: "198.51.100.5", ateIPv4: "198.51.100.6", dutIPv6: "2001:db8::5", ateIPv6: "2001:db8::6"},
		},
		lag2Name: {
			{vlanID: vlan10, dutIPv4: "198.51.100.9", ateIPv4: "198.51.100.10", dutIPv6: "2001:db8::9", ateIPv6: "2001:db8::0a"},
			{vlanID: vlan20, dutIPv4: "198.51.100.13", ateIPv4: "198.51.100.14", dutIPv6: "2001:db8::0d", ateIPv6: "2001:db8::0e"},
		},
	}

	t.Run("RT-5.14.1: Aggregate interface flap using min-link", func(t *testing.T) {
		configureDUT(t, dut, deviations.DefaultNetworkInstance(dut), 2, lag1Name, lag2Name, subs)
		ateConfig := configureATE(t, ate, lag1Name, lag2Name, subs)
		otg.PushConfig(t, ateConfig)
		otg.StartProtocols(t)
		awaitLAGMembersCollectingDistributing(t, dut, lag1Name, lag1Members)
		awaitLAGMembersCollectingDistributing(t, dut, lag2Name, lag2Members)
		otgutils.WaitForARP(t, otg, ateConfig, "IPv4")
		otgutils.WaitForARP(t, otg, ateConfig, "IPv6")
		for i := range 10 {
			t.Logf("Flap iteration #%d", i+1)
			gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
			// t.Logf("debug1 #%d", i+1)
			gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
			// t.Logf("debug2 #%d", i+1)
			// time.Sleep(time.Second * 30)
			gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, dutPorts["port1"]).Name()).Enabled().Config(), false)
			// t.Logf("debug3 #%d", i+1)
			gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, dutPorts["port3"]).Name()).Enabled().Config(), false)
			// t.Logf("debug4 #%d", i+1)
			// time.Sleep(time.Second * 30)
			gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_LOWER_LAYER_DOWN)
			// t.Logf("debug5 #%d", i+1)
			gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_LOWER_LAYER_DOWN)
			// t.Logf("debug6 #%d", i+1)

			gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, dutPorts["port1"]).Name()).Enabled().Config(), true)
			// t.Logf("debug7 #%d", i+1)
			gnmi.Update(t, dut, gnmi.OC().Interface(dut.Port(t, dutPorts["port3"]).Name()).Enabled().Config(), true)
			// t.Logf("debug8 #%d", i+1)
		}

		gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
		// t.Logf("debug9")
		gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
		// t.Logf("debug10")

		verifyLACPState(t, dut, ate, lag1Name, lag2Name)
		verifyTraffic(t, ate)

		var totalTxPkts uint64
		for _, flow := range gnmi.GetAll(t, otg, gnmi.OTG().FlowAny().State()) {
			totalTxPkts += flow.GetCounters().GetOutPkts()
		}
		// verifyCounters(t, dut, totalTxPkts, lag1Name, lag2Name, subs)
		verifyLACPState(t, dut, ate, lag1Name, lag2Name)

		for _, portName := range []string{"port1", "port2", "port3", "port4"} {
			name := dut.Port(t, portName).Name()
			if mtuVal := gnmi.Get(t, dut, gnmi.OC().Interface(name).State()).GetMtu(); mtuVal != mtu {
				t.Errorf("DUT interface %s MTU: got %d, want %d", name, mtuVal, mtu)
			}
		}
	})

	// t.Run("RT-5.14.2: Aggregate sub-interface in default Network Instance (NI)", func(t *testing.T) {
	// 	configureDUT(t, dut, deviations.DefaultNetworkInstance(dut), 1)
	// 	ateConfig := configureATE(t, ate)
	// 	otg.PushConfig(t, ateConfig)
	// 	otg.StartProtocols(t)

	// 	gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
	// 	gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)

	// 	verifyTraffic(t, ate)
	// 	var totalTxPkts uint64
	// 	for _, flow := range gnmi.GetAll(t, otg, gnmi.OTG().FlowAny().State()) {
	// 		totalTxPkts += flow.GetCounters().GetOutPkts()
	// 	}
	// 	verifyCounters(t, dut, totalTxPkts)
	// 	verifyLACPState(t, dut, ate)

	// 	for _, name := range []string{lag1Name, lag2Name, dutPorts["port1"], dutPorts["port2"], dutPorts["port3"], dutPorts["port4"]} {
	// 		if mtuVal := gnmi.Get(t, dut, gnmi.OC().Interface(name).State()).GetMtu(); mtuVal != mtu {
	// 			t.Errorf("DUT interface %s MTU: got %d, want %d", name, mtuVal, mtu)
	// 		}
	// 	}
	// })

	// t.Run("RT-5.14.3: Aggregate sub-interface in non-default Network Instance (NI)", func(t *testing.T) {
	// 	configureDUT(t, dut, testNI, 1)
	// 	ateConfig := configureATE(t, ate)
	// 	otg.PushConfig(t, ateConfig)
	// 	otg.StartProtocols(t)

	// 	gnmi.Await(t, dut, gnmi.OC().Interface(lag1Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
	// 	gnmi.Await(t, dut, gnmi.OC().Interface(lag2Name).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)

	// 	verifyTraffic(t, ate)
	// 	var totalTxPkts uint64
	// 	for _, flow := range gnmi.GetAll(t, otg, gnmi.OTG().FlowAny().State()) {
	// 		totalTxPkts += flow.GetCounters().GetOutPkts()
	// 	}
	// 	verifyCounters(t, dut, totalTxPkts)
	// 	verifyLACPState(t, dut, ate)

	// 	for _, name := range []string{lag1Name, lag2Name, dutPorts["port1"], dutPorts["port2"], dutPorts["port3"], dutPorts["port4"]} {
	// 		if mtuVal := gnmi.Get(t, dut, gnmi.OC().Interface(name).State()).GetMtu(); mtuVal != mtu {
	// 			t.Errorf("DUT interface %s MTU: got %d, want %d", name, mtuVal, mtu)
	// 		}
	// 	}
	// })
}
