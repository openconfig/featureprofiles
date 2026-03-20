package lacp_fallback_test

import (
	"errors"
	"fmt"
	"net"
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
	"github.com/openconfig/ygot/ygot"
)

const (
	lagName = "Port-Channel100"

	ateLagSystemID      = "00:00:00:00:00:02"
	ateLagActorKey      = 1
	ateLagActorSysPri   = 1
	ateLagActorPortPri  = 1
	ateLagActorActivity = "active"

	fallbackTimeout    = 60
	lacpTimeout        = 90 * time.Second
	trafficStopTimeout = 30 * time.Second
	floodTrafficWait   = 10 * time.Second

	vlan10 = 10
	vlan20 = 20

	ipv4PrefixLen = 27
	ipv6PrefixLen = 64
	ateVlan10ip   = "10.10.11.2"
	ateVlan10ipv6 = "2001:f:b::2"
	dutVlan10ip   = "10.10.11.3"
	dutVlan10ipv6 = "2001:f:b::3"
	ateVlan20ip   = "10.10.10.1"
	ateVlan20ipv6 = "2001:f:a::1"
	dutVlan20ip   = "10.10.10.2"
	dutVlan20ipv6 = "2001:f:a::2"

	port1 = "port1"
	port2 = "port2"
	port3 = "port3"
	port4 = "port4"

	ateLag = "ateLag"

	noOfPackets   = 1000
	packetRatePPS = 500
	packetSize    = 128

	broadcastMAC    = "ff:ff:ff:ff:ff:ff"
	flowIPv4Flood   = "ipv4_flood"
	flowIPv6Flood   = "ipv6_flood"
	flowIPv4Forward = "ipv4_forward"
	flowIPv6Forward = "ipv6_forward"

	wantLag        = true
	expectFallback = false
	ingress        = true
	egress         = false
)

type atePortConfig struct {
	portConfig *attrs.Attributes
	peerConfig *attrs.Attributes
}

type flowConfig struct {
	ipType string
	name   string
	srcDev *gosnappi.Device
	srcIP  string
	dstDev *gosnappi.Device
	dstIP  string
}

var (
	dutVlan10Peer = attrs.Attributes{IPv4: dutVlan10ip, IPv4Len: ipv4PrefixLen, IPv6: dutVlan10ipv6, IPv6Len: ipv6PrefixLen}
	dutVlan20Peer = attrs.Attributes{IPv4: dutVlan20ip, IPv4Len: ipv4PrefixLen, IPv6: dutVlan20ipv6, IPv6Len: ipv6PrefixLen}

	ateP1 = attrs.Attributes{
		Name:    port1,
		MAC:     "02:11:01:00:01:01",
		IPv4:    ateVlan10ip,
		IPv4Len: ipv4PrefixLen,
		IPv6:    ateVlan10ipv6,
		IPv6Len: ipv6PrefixLen,
	}

	ateP2 = attrs.Attributes{
		Name: "port2",
		MAC:  "02:11:02:00:01:01",
	}

	ateP4 = attrs.Attributes{
		Name:    port4,
		MAC:     "02:11:04:00:01:01",
		IPv4:    ateVlan20ip,
		IPv4Len: ipv4PrefixLen,
		IPv6:    ateVlan20ipv6,
		IPv6Len: ipv6PrefixLen,
	}

	atePortsMap = map[string]atePortConfig{
		port1: {portConfig: &ateP1, peerConfig: &dutVlan10Peer},
		port2: {portConfig: &ateP2, peerConfig: nil},
		port4: {portConfig: &ateP4, peerConfig: &dutVlan20Peer},
	}

	vlanData = []cfgplugins.DUTSubInterfaceData{
		{
			VlanID:        vlan10,
			IPv4Address:   net.ParseIP(dutVlan10ip),
			IPv6Address:   net.ParseIP(dutVlan10ipv6),
			IPv4PrefixLen: ipv4PrefixLen,
			IPv6PrefixLen: ipv6PrefixLen,
		},
		{
			VlanID:        vlan20,
			IPv4Address:   net.ParseIP(dutVlan20ip),
			IPv6Address:   net.ParseIP(dutVlan20ipv6),
			IPv4PrefixLen: ipv4PrefixLen,
			IPv6PrefixLen: ipv6PrefixLen,
		},
	}
	lacpPacketsMap = map[string]uint64{}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, port1)
	p2 := dut.Port(t, port2)
	p3 := dut.Port(t, port3)
	p4 := dut.Port(t, port4)

	t.Logf("Configuring DUT: LAG %s with LACP fallback", lagName)

	d := gnmi.OC()
	root := &oc.Root{}

	lacpTypeActive := oc.Lacp_LacpActivityType_ACTIVE
	lacpPeriodFast := oc.Lacp_LacpPeriodType_FAST
	dutAggParams := &cfgplugins.DUTAggData{
		LagName:      lagName,
		OndatraPorts: []*ondatra.Port{p1, p2},
		AggType:      oc.IfAggregate_AggregationType_LACP,
		LacpParams: &cfgplugins.LACPParams{
			Activity: &lacpTypeActive,
			Period:   &lacpPeriodFast,
		},
	}

	lagBatch := &gnmi.SetBatch{}
	cfgplugins.NewAggregateInterface(t, dut, lagBatch, dutAggParams)

	if !deviations.LacpInterfaceFallbackOCUnsupported(dut) {
		fb := root.GetOrCreateLacp().GetOrCreateInterface(lagName)
		fb.Fallback = ygot.Bool(true)
		gnmi.BatchUpdate(lagBatch, d.Lacp().Interface(lagName).Config(), fb)
	}
	lagBatch.Set(t, dut)

	if deviations.LacpInterfaceFallbackOCUnsupported(dut) {
		cfgplugins.ConfigureLACPFallbackCLI(t, dut, lagName, fallbackTimeout)
	}

	networkInstance := deviations.DefaultNetworkInstance(dut)
	vlanBatch := &gnmi.SetBatch{}
	for _, vlan := range vlanData {
		cfgplugins.CreateVlanFromOC(t, dut, vlanBatch, networkInstance, vlan)
	}

	if deviations.VlanSubinterfaceOCUnsupported(dut) {
		vlanBatch.Set(t, dut)
		for _, vlan := range vlanData {
			cfgplugins.ConfigureVlanInterfaceFromCLI(t, dut, vlan)
		}
	} else {
		for _, vlan := range vlanData {
			cfgplugins.ConfigureVlanInterfaceFromOC(t, dut, vlanBatch, vlan)
		}
		vlanBatch.Set(t, dut)
	}

	portVLANs := map[string]int{
		p1.Name(): vlan10,
		p2.Name(): vlan10,
		p3.Name(): vlan10,
		p4.Name(): vlan20,
	}
	intfBatch := &gnmi.SetBatch{}
	for portName, vlanID := range portVLANs {
		i := root.GetOrCreateInterface(portName)
		i.Enabled = ygot.Bool(true)
		i.SetName(portName)
		i.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
		vlan := i.GetOrCreateEthernet().GetOrCreateSwitchedVlan()
		vlan.SetAccessVlan(uint16(vlanID))
		vlan.SetInterfaceMode(oc.Vlan_VlanModeType_ACCESS)
		gnmi.BatchUpdate(intfBatch, d.Interface(portName).Config(), i)
	}
	intfBatch.Set(t, dut)
}

func configureATE(t *testing.T, configureLag bool, atePorts ...*ondatra.Port) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	for _, port := range atePorts {
		if atePort, ok := atePortsMap[port.ID()]; ok && !configureLag && atePort.portConfig != nil && atePort.peerConfig != nil {
			atePort.portConfig.AddToOTG(top, port, atePort.peerConfig)
		} else {
			top.Ports().Add().SetName(port.ID())
		}
	}
	if !configureLag {
		return top
	}

	lag := top.Lags().Add().SetName(ateLag)
	lag.Protocol().Lacp().SetActorKey(ateLagActorKey).SetActorSystemPriority(ateLagActorSysPri).SetActorSystemId(ateLagSystemID)
	for index, port := range atePorts {
		lp1 := lag.Ports().Add().SetPortName(port.ID())
		lp1.Ethernet().SetMac(atePortsMap[port.ID()].portConfig.MAC).SetName(port.ID())
		lp1.Lacp().SetActorActivity(ateLagActorActivity).SetActorPortNumber(uint32(index + 1)).SetActorPortPriority(ateLagActorPortPri).SetLacpduTimeout(0)
	}

	dev := top.Devices().Add().SetName(ateLag + "Dev")
	eth := dev.Ethernets().Add().SetName(ateLag + ".eth")
	eth.Connection().SetLagName(lag.Name())
	eth.SetMac(ateP1.MAC)

	return top
}

func TestLacpFallback(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	p1 := dut.Port(t, port1)
	p2 := dut.Port(t, port2)
	p3 := dut.Port(t, port3)
	p4 := dut.Port(t, port4)

	ap1 := ate.Port(t, port1)
	ap2 := ate.Port(t, port2)
	ap3 := ate.Port(t, port3)
	ap4 := ate.Port(t, port4)

	configureDUT(t, dut)
	lacpPacketsMap[p1.Name()] = gnmi.Get(t, dut, gnmi.OC().Lacp().Interface(lagName).Member(p1.Name()).Counters().LacpOutPkts().State())
	lacpPacketsMap[p2.Name()] = gnmi.Get(t, dut, gnmi.OC().Lacp().Interface(lagName).Member(p2.Name()).Counters().LacpOutPkts().State())

	top := configureATE(t, wantLag, ap1, ap2)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	t.Log("Waiting for LACP to establish on DUT ports...")
	if err := verifyLACPPortsState(t, dut, !expectFallback, p1, p2); err != nil {
		t.Fatalf("initial LACP setup: %v", err)
	}

	t.Log("Verifying DUT LAG ports are sending LACP PDUs...")
	if err := verifyLACPPortsOutPkts(t, dut, p1, p2); err != nil {
		t.Fatalf("initial LACP PDU check: %v", err)
	}

	t.Log("Stopping ATE protocols to trigger LACP fallback on DUT...")
	ate.OTG().StopProtocols(t)

	if err := verifyLACPPortsState(t, dut, expectFallback, p1, p2); err != nil {
		t.Fatalf("initial fallback state: %v", err)
	}

	testCases := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "RT-5.15.1 - LACP fallback ports receives traffic",
			run: func(t *testing.T) error {
				top := configureATE(t, !wantLag, ap1, ap2, ap3, ap4)
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)
				otgutils.WaitForARP(t, ate.OTG(), top, cfgplugins.IPv4)
				otgutils.WaitForARP(t, ate.OTG(), top, cfgplugins.IPv6)

				dev1 := top.Devices().Items()[0]
				dev4 := top.Devices().Items()[1]

				floodFlows := []flowConfig{
					{name: flowIPv4Flood, ipType: cfgplugins.IPv4, srcDev: &dev1, srcIP: ateVlan10ip, dstIP: dutVlan10ip},
					{name: flowIPv6Flood, ipType: cfgplugins.IPv6, srcDev: &dev1, srcIP: ateVlan10ipv6, dstIP: dutVlan10ipv6},
				}
				forwardedFlows := []flowConfig{
					{name: flowIPv4Forward, ipType: cfgplugins.IPv4, srcDev: &dev1, srcIP: ateVlan10ip, dstDev: &dev4, dstIP: ateVlan20ip},
					{name: flowIPv6Forward, ipType: cfgplugins.IPv6, srcDev: &dev1, srcIP: ateVlan10ipv6, dstDev: &dev4, dstIP: ateVlan20ipv6},
				}

				baselinePkts := map[string]uint64{
					p1.Name(): gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).Counters().InPkts().State()),
					p2.Name(): gnmi.Get(t, dut, gnmi.OC().Interface(p2.Name()).Counters().OutPkts().State()),
					p3.Name(): gnmi.Get(t, dut, gnmi.OC().Interface(p3.Name()).Counters().OutPkts().State()),
					p4.Name(): gnmi.Get(t, dut, gnmi.OC().Interface(p4.Name()).Counters().OutPkts().State()),
				}

				checkDUTPortPkts := func(portName string, isIngress bool) error {
					prev := baselinePkts[portName]
					var newPkts uint64
					var direction string
					if isIngress {
						newPkts = gnmi.Get(t, dut, gnmi.OC().Interface(portName).Counters().InPkts().State())
						direction = "received"
					} else {
						newPkts = gnmi.Get(t, dut, gnmi.OC().Interface(portName).Counters().OutPkts().State())
						direction = "sent"
					}
					delta := newPkts - prev
					baselinePkts[portName] = newPkts
					if delta == 0 {
						return fmt.Errorf("DUT port %s %s 0 packets, want ~%d", portName, direction, noOfPackets)
					}
					if delta < noOfPackets {
						return fmt.Errorf("DUT port %s %s %d packets, want ~%d", portName, direction, delta, noOfPackets)
					}
					t.Logf("DUT port %s %s %d packets", portName, direction, delta)
					return nil
				}

				for _, flow := range floodFlows {
					configureFlow(t, top, flow)
				}
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)

				t.Log("Sending flood traffic...")
				ate.OTG().StartTraffic(t)
				floodErr := waitForFloodedPkts(t, ate, ap2, ap3)
				ate.OTG().StopTraffic(t)

				var errs []error
				errs = append(errs, floodErr)
				errs = append(errs, checkDUTPortPkts(p1.Name(), ingress))
				errs = append(errs, checkDUTPortPkts(p2.Name(), egress))
				errs = append(errs, checkDUTPortPkts(p3.Name(), egress))

				top.Flows().Clear()
				for _, flow := range forwardedFlows {
					configureFlow(t, top, flow)
				}
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)

				t.Log("Sending routed traffic...")
				ate.OTG().StartTraffic(t)
				for _, flow := range forwardedFlows {
					errs = append(errs, waitForTraffic(t, ate.OTG(), flow.name, trafficStopTimeout))
				}

				otgutils.LogFlowMetrics(t, ate.OTG(), top)
				otgutils.LogPortMetrics(t, ate.OTG(), top)

				errs = append(errs, checkDUTPortPkts(p1.Name(), ingress))
				errs = append(errs, checkDUTPortPkts(p4.Name(), egress))

				for _, flowName := range []string{flowIPv4Forward, flowIPv6Forward} {
					fm := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
					if got := fm.Counters.GetInPkts(); got != noOfPackets {
						errs = append(errs, fmt.Errorf("flow %s: received %d packets, want %d", flowName, got, noOfPackets))
					} else {
						t.Logf("flow %s: received %d packets with no loss", flowName, got)
					}
				}

				t.Log("Verifying DUT LAG ports are still sending LACP PDUs in fallback mode...")
				errs = append(errs, verifyLACPPortsOutPkts(t, dut, p1, p2))
				errs = append(errs, verifyLACPPortsState(t, dut, expectFallback, p1, p2))

				return errors.Join(errs...)
			},
		},
		{
			name: "RT-5.15.2 - LACP Fallback port receives LACP pdu",
			run: func(t *testing.T) error {
				top := configureATE(t, wantLag, ap1)
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)

				var errs []error
				t.Log("Waiting for DUT:Port[1] to form LACP (IN_SYNC)...")
				errs = append(errs, verifyLACPState(t, dut, p1, true))

				t.Log("Verifying DUT:Port[2] transitioned to LACP detached (OUT_SYNC)...")
				errs = append(errs, verifyLACPState(t, dut, p2, false))

				t.Log("Verifying both DUT LAG ports are still sending LACP PDUs...")
				errs = append(errs, verifyLACPPortsOutPkts(t, dut, p1, p2))

				return errors.Join(errs...)
			},
		},
		{
			name: "RT-5.15.3 - One of the LACP ports times out",
			run: func(t *testing.T) error {
				top := configureATE(t, wantLag, ap1, ap2)
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)

				var errs []error
				t.Log("Waiting for LACP to establish on DUT ports...")
				if err := verifyLACPPortsState(t, dut, !expectFallback, p1, p2); err != nil {
					t.Fatalf("RT-5.15.3 setup: %v", err)
				}

				t.Log("Removing Port[2] from ATE LAG")
				top = configureATE(t, wantLag, ap1)
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)

				t.Log("Verifying DUT:Port[2] moved to aggregate detached (OUT_SYNC)...")
				errs = append(errs, verifyLACPState(t, dut, p2, expectFallback))
				t.Log("Verifying DUT:Port[1] remains in LACP aggregate (IN_SYNC)...")
				errs = append(errs, verifyLACPState(t, dut, p1, !expectFallback))

				t.Log("Verifying both DUT LAG ports are still sending LACP PDUs...")
				errs = append(errs, verifyLACPPortsOutPkts(t, dut, p1, p2))

				t.Log("Re-enabling LACP on both ATE ports and verifying re-aggregation...")
				top = configureATE(t, wantLag, ap1, ap2)
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)
				t.Log("Waiting for LACP to establish on DUT ports...")
				errs = append(errs, verifyLACPPortsState(t, dut, !expectFallback, p1, p2))

				return errors.Join(errs...)
			},
		},
		{
			name: "RT-5.15.4 - Both LACP ports time out",
			run: func(t *testing.T) error {
				top := configureATE(t, wantLag, ap1, ap2)
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)

				var errs []error
				t.Log("Waiting for LACP to establish on DUT ports...")
				if err := verifyLACPPortsState(t, dut, !expectFallback, p1, p2); err != nil {
					t.Fatalf("RT-5.15.4 setup: %v", err)
				}

				t.Log("Stopping ATE protocols — both ports will stop sending LACP PDUs...")
				ate.OTG().StopProtocols(t)

				t.Log("Verifying both DUT ports moved to aggregate detached (OUT_SYNC)...")
				errs = append(errs, verifyLACPPortsState(t, dut, expectFallback, p1, p2))

				t.Log("Verifying both DUT LAG ports are still sending LACP PDUs in fallback mode...")
				errs = append(errs, verifyLACPPortsOutPkts(t, dut, p1, p2))

				t.Log("Re-enabling LACP on both ATE ports and verifying re-aggregation...")
				top = configureATE(t, wantLag, ap1, ap2)
				ate.OTG().PushConfig(t, top)
				ate.OTG().StartProtocols(t)
				t.Log("Waiting for LACP to establish on DUT ports...")
				errs = append(errs, verifyLACPPortsState(t, dut, !expectFallback, p1, p2))

				return errors.Join(errs...)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(t); err != nil {
				t.Errorf("test case %s failed: %v", tc.name, err)
			}
		})
	}
}

func waitForFloodedPkts(t *testing.T, ate *ondatra.ATEDevice, ports ...*ondatra.Port) error {
	t.Helper()
	expectedPkts := uint64(len(ports) * noOfPackets)
	var errs []error
	for _, port := range ports {
		portName := port.ID()
		portPath := gnmi.OTG().Port(portName).Counters().InFrames().State()
		framesCheck := func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return present && v >= expectedPkts
		}
		val, ok := gnmi.Watch(t, ate.OTG(), portPath, floodTrafficWait, framesCheck).Await(t)
		v, _ := val.Val()
		if !ok {
			errs = append(errs, fmt.Errorf("ate port %s: unexpected flood packet count %d, want ~%d", portName, v, expectedPkts))
		} else {
			t.Logf("ate port %s received %d flooded packets", portName, v)
		}
	}
	return errors.Join(errs...)
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) error {
	t.Helper()
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	checkState := func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}
	if _, ok := gnmi.Watch(t, otg, transmitPath, timeout, checkState).Await(t); !ok {
		return fmt.Errorf("traffic for flow %s did not stop within %v", flowName, timeout)
	}
	t.Logf("traffic for flow %s has stopped", flowName)
	return nil
}

func configureFlow(t *testing.T, top gosnappi.Config, flowConf flowConfig) {
	t.Helper()
	if flowConf.srcDev == nil {
		t.Fatalf("configureFlow: srcDev is nil for flow %s", flowConf.name)
	}

	flow := top.Flows().Add().SetName(flowConf.name)
	flow.Size().SetFixed(packetSize)
	flow.Rate().SetPps(packetRatePPS)
	flow.Duration().FixedPackets().SetPackets(noOfPackets)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", (*flowConf.srcDev).Name(), flowConf.ipType)})
	eth := flow.Packet().Add().Ethernet()
	if flowConf.dstDev != nil {
		flow.TxRx().Device().SetRxNames([]string{fmt.Sprintf("%s.%s", (*flowConf.dstDev).Name(), flowConf.ipType)})
		flow.Metrics().SetEnable(true)
		eth.Dst().Auto()
	} else {
		eth.Dst().SetValue(broadcastMAC)
	}
	eth.Src().SetValue((*flowConf.srcDev).Ethernets().Items()[0].Mac())

	switch flowConf.ipType {
	case cfgplugins.IPv4:
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(flowConf.srcIP)
		v4.Dst().SetValue(flowConf.dstIP)
	case cfgplugins.IPv6:
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(flowConf.srcIP)
		v6.Dst().SetValue(flowConf.dstIP)
	}
}

func verifyLACPState(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, wantSync bool) error {
	t.Helper()
	wantedState := "OUT_SYNC (fallback/detached)"
	syncState := oc.Lacp_LacpSynchronizationType_OUT_SYNC
	if wantSync {
		wantedState = "IN_SYNC (aggregated)"
		syncState = oc.Lacp_LacpSynchronizationType_IN_SYNC
	}
	t.Logf("waiting for DUT port %s LACP state: %s", p.Name(), wantedState)

	memberPath := gnmi.OC().Lacp().Interface(lagName).Member(p.Name()).State()
	lacpStateCheck := func(val *ygnmi.Value[*oc.Lacp_Interface_Member]) bool {
		if !val.IsPresent() {
			return false
		}
		m, _ := val.Val()
		return m.Synchronization == syncState &&
			m.GetCollecting() == wantSync &&
			m.GetDistributing() == wantSync &&
			(!wantSync || m.GetPartnerId() == ateLagSystemID)
	}
	val, ok := gnmi.Watch(t, dut, memberPath, lacpTimeout, lacpStateCheck).Await(t)
	if !ok {
		if _, present := val.Val(); present {
			return fmt.Errorf("port %s: timeout waiting for %s", p.Name(), wantedState)
		}
		return fmt.Errorf("port %s: timeout waiting for %s; no telemetry received", p.Name(), wantedState)
	}
	m, _ := val.Val()
	t.Logf("DUT port %s reached %s: sync=%s collecting=%t distributing=%t",
		p.Name(), wantedState, m.Synchronization, m.GetCollecting(), m.GetDistributing())
	return nil
}

func verifyLACPOutPkts(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) error {
	t.Helper()
	pktPath := gnmi.OC().Lacp().Interface(lagName).Member(p.Name()).Counters().LacpOutPkts().State()
	lacpPktCheck := func(val *ygnmi.Value[uint64]) bool {
		v, present := val.Val()
		return present && v > lacpPacketsMap[p.Name()]
	}
	val, ok := gnmi.Watch(t, dut, pktPath, lacpTimeout, lacpPktCheck).Await(t)
	v, _ := val.Val()
	if !ok {
		return fmt.Errorf("port %s: no new LACP PDUs sent; counter = %d, previous = %d",
			p.Name(), v, lacpPacketsMap[p.Name()])
	}
	t.Logf("DUT port %s LACP out-pkts delta = %d", p.Name(), v-lacpPacketsMap[p.Name()])
	lacpPacketsMap[p.Name()] = v
	return nil
}

func verifyLACPPortsState(t *testing.T, dut *ondatra.DUTDevice, wantSync bool, ports ...*ondatra.Port) error {
	t.Helper()
	var errs []error
	for _, p := range ports {
		errs = append(errs, verifyLACPState(t, dut, p, wantSync))
	}
	return errors.Join(errs...)
}

func verifyLACPPortsOutPkts(t *testing.T, dut *ondatra.DUTDevice, ports ...*ondatra.Port) error {
	t.Helper()
	var errs []error
	for _, p := range ports {
		errs = append(errs, verifyLACPOutPkts(t, dut, p))
	}
	return errors.Join(errs...)
}
