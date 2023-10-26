// Copyright 2023 Nokia
//
// This code is a Contribution to OpenConfig Feature Profiles project ("Work")
// made under the Google Software Grant and Corporate Contributor License
// Agreement ("CLA") and governed by the Apache License 2.0. No other rights
// or licenses in or to any of Nokia's intellectual property are granted for
// any other purpose.  This code is provided on an "as is" basis without
// any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package large_ip_packet_transmission_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	ondatraotg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4                      = "IPv4"
	ipv4PrefixLen             = 30
	ipv6                      = "IPv6"
	ipv6PrefixLen             = 126
	mtu                       = 9_216
	trafficRunDuration        = 15 * time.Second
	trafficStopWaitDuration   = 10 * time.Second
	acceptablePacketSizeDelta = 0.5
	acceptableLossPercent     = 0.5
	subInterfaceIndex         = 0
)

var (
	dutPort1 = &attrs.Attributes{
		Name:    "dutPort1",
		MAC:     "00:12:01:01:01:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	dutPort2 = &attrs.Attributes{
		Name:    "dutPort2",
		MAC:     "00:12:02:01:01:01",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	atePort1 = &attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	atePort2 = &attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
	}

	atePorts = map[string]*attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
	}

	testCases = []testDefinition{
		{
			name:     "flow_size_9500_should_fail",
			desc:     "9500 byte flow that will be dropped",
			flowSize: mtu + 34,
		},
		{
			name:     "flow_size_1500",
			desc:     "1500 byte flow",
			flowSize: 1500,
		},
		{
			name:     "flow_size_2000",
			desc:     "2000 byte flow",
			flowSize: 2000,
		},
		{
			name:     "flow_size_4000",
			desc:     "4000 byte flow",
			flowSize: 4000,
		},
		{
			name:     "flow_size_9202",
			desc:     "9202 byte flow",
			flowSize: 9202,
		},
	}
)

type testData struct {
	flowProto  string
	ate        *ondatra.ATEDevice
	otg        *ondatraotg.OTG
	otgConfig  gosnappi.Config
	srcAtePort *attrs.Attributes
	dstAtePort *attrs.Attributes
}

func (d *testData) waitInterface(t *testing.T) {
	otgutils.WaitForARP(t, d.otg, d.otgConfig, d.flowProto)
}

func (d *testData) waitBundle(t *testing.T) {
	time.Sleep(5 * time.Second)

	for _, bundleName := range []string{d.srcAtePort.Name, d.dstAtePort.Name} {
		gnmi.Watch(
			t, d.otg, gnmi.OTG().Lag(bundleName).OperStatus().State(),
			time.Minute,
			func(val *ygnmi.Value[otgtelemetry.E_Lag_OperStatus]) bool {
				state, present := val.Val()
				return present && state.String() == "UP"
			},
		).Await(t)
	}
}

type testDefinition struct {
	name     string
	desc     string
	flowSize uint32
}

type trafficFlowParams struct {
	name  string
	proto string
	size  uint32
}

func createFlow(
	srcAtePort, dstAtePort *attrs.Attributes,
	flowParams trafficFlowParams,
) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(flowParams.name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().
		SetTxNames([]string{fmt.Sprintf("%s.%s", srcAtePort.Name, flowParams.proto)}).
		SetRxNames([]string{fmt.Sprintf("%s.%s", dstAtePort.Name, flowParams.proto)})
	flow.Packet().Add().Ethernet()
	flow.SetSize(gosnappi.NewFlowSize().SetFixed(flowParams.size))

	switch flowParams.proto {
	case ipv4:
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(srcAtePort.IPv4)
		v4.Dst().SetValue(dstAtePort.IPv4)
	case ipv6:
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(srcAtePort.IPv6)
		v6.Dst().SetValue(dstAtePort.IPv6)
	}

	flow.EgressPacket().Add().Ethernet()

	return flow
}

func runTest(
	t *testing.T,
	tt testDefinition,
	td testData,
	waitF func(t *testing.T),
) {
	t.Logf("Name: %s", tt.name)
	t.Logf("Description: %s", tt.desc)

	flowParams := createFlow(
		td.srcAtePort,
		td.dstAtePort,
		trafficFlowParams{
			name:  tt.name,
			proto: td.flowProto,
			size:  tt.flowSize,
		},
	)

	td.otgConfig.Flows().Clear()
	td.otgConfig.Flows().Append(flowParams)

	td.otg.PushConfig(t, td.otgConfig)
	td.otg.StartProtocols(t)

	waitF(t)

	td.otg.StartTraffic(t)
	time.Sleep(trafficRunDuration)

	td.otg.StopTraffic(t)
	time.Sleep(trafficStopWaitDuration)

	otgutils.LogFlowMetrics(t, td.ate.OTG(), td.otgConfig)

	flow := gnmi.OTG().Flow(tt.name)
	flowCounters := flow.Counters()

	outPkts := gnmi.Get(t, td.otg, flowCounters.OutPkts().State())
	inPkts := gnmi.Get(t, td.otg, flowCounters.InPkts().State())
	inOctets := gnmi.Get(t, td.otg, flowCounters.InOctets().State())

	if tt.flowSize > mtu {
		if inPkts == 0 {
			t.Logf(
				"flow sent '%d' packets and received '%d' packets, this is expected "+
					"due to flow size '%d' being > interface MTU of '%d' bytes",
				outPkts, inPkts, tt.flowSize, mtu,
			)
		} else {
			t.Errorf(
				"flow received packets but should *not* have due to flow size '%d' being"+
					" > interface MTU of '%d' bytes",
				tt.flowSize, mtu,
			)
		}

		return
	}

	if outPkts == 0 || inPkts == 0 {
		t.Error("flow did not send or receive any packets, this should not happen")

		return
	}

	lossPercent := (float32(outPkts-inPkts) / float32(outPkts)) * 100

	if lossPercent > acceptableLossPercent {
		t.Errorf(
			"flow sent '%d' packets and received '%d' packets, resulting in a "+
				"loss percent of '%.2f'. max acceptable loss percent is '%.2f'",
			outPkts, inPkts, lossPercent, acceptableLossPercent,
		)
	}

	avgPacketSize := uint32(inOctets / inPkts)
	packetSizeDelta := float32(avgPacketSize-tt.flowSize) /
		(float32(avgPacketSize+tt.flowSize) / 2) * 100

	if packetSizeDelta > acceptablePacketSizeDelta {
		t.Errorf(
			"flow sent '%d' packets and received '%d' packets, resulting in "+
				"averagepacket size of '%d' and a packet size delta of '%.2f' percent. "+
				"packet size delta should not exceed '%.2f'",
			outPkts, inPkts, avgPacketSize, packetSizeDelta, acceptablePacketSizeDelta,
		)
	}
}

func configureDUTPort(
	t *testing.T,
	dut *ondatra.DUTDevice,
	port *ondatra.Port,
	portAttrs *attrs.Attributes,
) {
	gnmiOCRoot := gnmi.OC()

	gnmi.Replace(
		t,
		dut,
		gnmiOCRoot.Interface(port.Name()).Config(),
		portAttrs.NewOCInterface(port.Name(), dut),
	)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, port)
	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(
			t, dut, port.Name(), deviations.DefaultNetworkInstance(dut), subInterfaceIndex,
		)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	for portName, portAttrs := range dutPorts {
		port := dut.Port(t, portName)

		configureDUTPort(t, dut, port, portAttrs)

		verifyDUTPort(t, dut, port.Name())
	}
}

func verifyDUTPort(t *testing.T, dut *ondatra.DUTDevice, portName string) {
	configuredInterfaceMtu := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Mtu().State())
	configuredIpv4SubInterfaceMtu := gnmi.GetConfig(
		t,
		dut,
		gnmi.OC().Interface(portName).Subinterface(subInterfaceIndex).Ipv4().Mtu().Config(),
	)
	configuredIpv6SubInterfaceMtu := gnmi.GetConfig(
		t,
		dut,
		gnmi.OC().Interface(portName).Subinterface(subInterfaceIndex).Ipv6().Mtu().Config(),
	)

	expectedInterfaceMtu := mtu
	expectedSuBInterfaceMtu := mtu

	if !deviations.OmitL2MTU(dut) {
		expectedInterfaceMtu = expectedInterfaceMtu + 14
	}

	if int(configuredInterfaceMtu) != expectedInterfaceMtu {
		t.Errorf(
			"dut %s configured mtu is incorrect, got: %d, want: %d",
			dut.Name(), configuredInterfaceMtu, expectedInterfaceMtu,
		)
	}

	if int(configuredIpv4SubInterfaceMtu) != expectedSuBInterfaceMtu {
		t.Errorf(
			"dut %s configured mtu is incorrect, got: %d, want: %d",
			dut.Name(), configuredIpv4SubInterfaceMtu, expectedSuBInterfaceMtu,
		)
	}

	if int(configuredIpv6SubInterfaceMtu) != expectedSuBInterfaceMtu {
		t.Errorf(
			"dut %s configured mtu is incorrect, got: %d, want: %d",
			dut.Name(), configuredIpv6SubInterfaceMtu, expectedSuBInterfaceMtu,
		)
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()

	for portName, portAttrs := range atePorts {
		port := ate.Port(t, portName)

		dutPort := dutPorts[portName]

		portAttrs.AddToOTG(otgConfig, port, dutPort)
	}

	return otgConfig
}

func TestLargeIPPacketTransmission(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	configureDUT(t, dut)

	otgConfig := configureATE(t, ate)

	t.Cleanup(func() {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			netInst := &oc.NetworkInstance{
				Name: ygot.String(deviations.DefaultNetworkInstance(dut)),
			}

			for portName := range dutPorts {
				gnmi.Delete(
					t,
					dut,
					gnmi.OC().
						NetworkInstance(*netInst.Name).
						Interface(fmt.Sprintf("%s.%d", dut.Port(t, portName).Name(), subInterfaceIndex)).
						Config(),
				)
			}
		}

		for portName := range dutPorts {
			gnmi.Delete(t, dut, gnmi.OC().Interface(dut.Port(t, portName).Name()).Mtu().Config())
			gnmi.Delete(
				t,
				dut,
				gnmi.OC().
					Interface(dut.Port(t, portName).Name()).
					Subinterface(subInterfaceIndex).
					Config(),
			)
		}
	})

	for _, tt := range testCases {
		for _, flowProto := range []string{ipv4, ipv6} {
			td := testData{
				flowProto:  flowProto,
				ate:        ate,
				otg:        otg,
				otgConfig:  otgConfig,
				srcAtePort: atePort1,
				dstAtePort: atePort2,
			}

			t.Run(fmt.Sprintf("%s-%s", tt.name, flowProto), func(t *testing.T) {
				runTest(
					t,
					tt,
					td,
					td.waitInterface,
				)
			})
		}
	}
}

func configureDUTBundle(
	t *testing.T, dut *ondatra.DUTDevice, lag *attrs.Attributes, bundleMembers []*ondatra.Port,
) string {
	bundleID := netutil.NextAggregateInterface(t, dut)

	gnmiOCRoot := gnmi.OC()
	ocRoot := &oc.Root{}

	if deviations.AggregateAtomicUpdate(dut) {
		bundle := ocRoot.GetOrCreateInterface(bundleID)
		bundle.GetOrCreateAggregation()
		bundle.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

		for _, port := range bundleMembers {
			intf := ocRoot.GetOrCreateInterface(port.Name())
			intf.GetOrCreateEthernet().AggregateId = ygot.String(bundleID)
			intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

			if deviations.InterfaceEnabled(dut) {
				intf.Enabled = ygot.Bool(true)
			}

			if deviations.ExplicitPortSpeed(dut) {
				intf.Ethernet.SetPortSpeed(fptest.GetIfSpeed(t, port))
			}
		}

		gnmi.Update(
			t,
			dut,
			gnmiOCRoot.Config(),
			ocRoot,
		)
	}

	lacp := &oc.Lacp_Interface{
		Name:     ygot.String(bundleID),
		LacpMode: oc.Lacp_LacpActivityType_UNSET,
	}
	lacpPath := gnmiOCRoot.Lacp().Interface(bundleID)
	gnmi.Replace(t, dut, lacpPath.Config(), lacp)

	agg := &oc.Interface{
		Name: ygot.String(bundleID),
		Mtu:  ygot.Uint16(mtu),
		Type: oc.IETFInterfaces_InterfaceType_ieee8023adLag,
	}
	if !deviations.OmitL2MTU(dut) {
		agg.Mtu = ygot.Uint16(mtu + 14)
	}
	agg.Description = ygot.String(fmt.Sprintf("dutLag-%s", bundleID))
	if deviations.InterfaceEnabled(dut) {
		agg.Enabled = ygot.Bool(true)
	}

	subInterface := agg.GetOrCreateSubinterface(subInterfaceIndex)
	v4SubInterface := subInterface.GetOrCreateIpv4()
	v4SubInterface.SetMtu(mtu)
	if deviations.InterfaceEnabled(dut) {
		v4SubInterface.Enabled = ygot.Bool(true)
	}
	v4Address := v4SubInterface.GetOrCreateAddress(lag.IPv4)
	v4Address.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	v6SubInterface := subInterface.GetOrCreateIpv6()
	v6SubInterface.SetMtu(mtu)
	if deviations.InterfaceEnabled(dut) {
		v6SubInterface.Enabled = ygot.Bool(true)
	}
	v6Address := v6SubInterface.GetOrCreateAddress(lag.IPv6)
	v6Address.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	intfAgg := agg.GetOrCreateAggregation()
	intfAgg.LagType = oc.IfAggregate_AggregationType_STATIC

	aggPath := gnmiOCRoot.Interface(bundleID)
	gnmi.Replace(t, dut, aggPath.Config(), agg)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(
			t, dut, bundleID, deviations.DefaultNetworkInstance(dut), subInterfaceIndex,
		)
	}

	// if we didnt setup the ports in the lag before
	if !deviations.AggregateAtomicUpdate(dut) {
		for _, port := range bundleMembers {
			intf := &oc.Interface{Name: ygot.String(port.Name())}
			intf.GetOrCreateEthernet().AggregateId = ygot.String(bundleID)
			intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

			if deviations.InterfaceEnabled(dut) {
				intf.Enabled = ygot.Bool(true)
			}

			if deviations.ExplicitPortSpeed(dut) {
				fptest.SetPortSpeed(t, port)
			}

			intfPath := gnmiOCRoot.Interface(port.Name())

			gnmi.Replace(t, dut, intfPath.Config(), intf)
		}
	}

	verifyDUTPort(t, dut, *agg.Name)

	return bundleID
}

func configureATEBundles(
	allAtePorts []*ondatra.Port,
	bundleMemberCount int,
) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()

	otgConfig = configureATEBundle(
		otgConfig,
		atePort1,
		dutPort1,
		allAtePorts[0:bundleMemberCount],
		1,
	)
	otgConfig = configureATEBundle(
		otgConfig,
		atePort2,
		dutPort2,
		allAtePorts[bundleMemberCount:2*bundleMemberCount],
		2,
	)

	portNames := make([]string, len(allAtePorts))
	for idx, port := range allAtePorts {
		portNames[idx] = port.ID()
	}

	// note that it seems max in otg containers is 9000 so bundle tests > 1500 bytes will fail,
	// for whatever reason individual ports work just fine > 1500 bytes though! also, physical gear
	// seems to work just fine as well, so we'll set this to the max we can for kne tests.
	layer1 := otgConfig.Layer1().Add().
		SetName("layerOne").
		SetPortNames(portNames).
		SetMtu(9000)

	// set the l1 speed for the otg config based on speed setting in testbed, fallthrough case is
	// do nothing which defaults to 10g
	switch allAtePorts[0].Speed() {
	case ondatra.Speed1Gb:
		layer1.SetSpeed(gosnappi.Layer1Speed.SPEED_1_GBPS)
	case ondatra.Speed10Gb:
		layer1.SetSpeed(gosnappi.Layer1Speed.SPEED_10_GBPS)
	case ondatra.Speed100Gb:
		layer1.SetSpeed(gosnappi.Layer1Speed.SPEED_100_GBPS)
	case ondatra.Speed400Gb:
		layer1.SetSpeed(gosnappi.Layer1Speed.SPEED_400_GBPS)
	default:
	}

	return otgConfig
}

func configureATEBundle(
	otgConfig gosnappi.Config,
	ateLag *attrs.Attributes,
	dutLag *attrs.Attributes,
	bundleMembers []*ondatra.Port,
	bundleID uint32,
) gosnappi.Config {
	agg := otgConfig.Lags().Add().SetName(ateLag.Name)
	agg.Protocol().Static().SetLagId(bundleID)

	for idx, port := range bundleMembers {
		_ = otgConfig.Ports().Add().SetName(port.ID())
		agg.Ports().
			Add().
			SetPortName(port.ID()).
			Ethernet().
			// wont have more than 8 members, so no need to be fancy
			SetMac(fmt.Sprintf("%s0%d", ateLag.MAC[:len(ateLag.MAC)-2], idx+2)).
			SetName("LAG-" + port.ID()).
			SetMtu(mtu)
	}

	aggDev := otgConfig.Devices().Add().SetName(agg.Name() + ".dev")
	aggEth := aggDev.Ethernets().
		Add().
		SetName(fmt.Sprintf("%s.Eth", ateLag.Name)).
		SetMac(ateLag.MAC).
		SetMtu(mtu)
	aggEth.Connection().SetLagName(agg.Name())

	aggEth.Ipv4Addresses().Add().SetName(fmt.Sprintf("%s.IPv4", ateLag.Name)).
		SetAddress(ateLag.IPv4).
		SetGateway(dutLag.IPv4).
		SetPrefix(ipv4PrefixLen)

	aggEth.Ipv6Addresses().Add().SetName(fmt.Sprintf("%s.IPv6", ateLag.Name)).
		SetAddress(ateLag.IPv6).
		SetGateway(dutLag.IPv6).
		SetPrefix(ipv6PrefixLen)

	return otgConfig
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func TestLargeIPPacketTransmissionBundle(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	allDutPorts := sortPorts(dut.Ports())
	allAtePorts := sortPorts(ate.Ports())

	if len(allDutPorts) < 2 {
		t.Fatalf("testbed requires at least two dut ports, but only has %d", len(allDutPorts))
	}

	if len(allAtePorts) < 2 {
		t.Fatalf("testbed requires at least two ate ports, but only has %d", len(allAtePorts))
	}

	bundleMemberCount := len(allDutPorts) / 2
	if len(allAtePorts) < len(allDutPorts) {
		bundleMemberCount = len(allAtePorts) / 2
	}

	lagOneDutBundleMembers := allDutPorts[0:bundleMemberCount]
	lagTwoDutBundleMembers := allDutPorts[bundleMemberCount : 2*bundleMemberCount]

	var allDutBundleMembers []*ondatra.Port
	allDutBundleMembers = append(allDutBundleMembers, lagOneDutBundleMembers...)
	allDutBundleMembers = append(allDutBundleMembers, lagTwoDutBundleMembers...)

	lagOne := configureDUTBundle(t, dut, dutPort1, lagOneDutBundleMembers)
	lagTwo := configureDUTBundle(t, dut, dutPort2, lagTwoDutBundleMembers)

	otgConfig := configureATEBundles(allAtePorts, bundleMemberCount)

	t.Cleanup(func() {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			netInst := &oc.NetworkInstance{
				Name: ygot.String(deviations.DefaultNetworkInstance(dut)),
			}

			for _, lag := range []string{lagOne, lagTwo} {
				gnmi.Delete(
					t,
					dut,
					gnmi.OC().
						NetworkInstance(*netInst.Name).
						Interface(fmt.Sprintf("%s.%d", lag, subInterfaceIndex)).
						Config(),
				)
			}
		}

		for _, port := range allDutBundleMembers {
			gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Mtu().Config())
			gnmi.Delete(
				t,
				dut,
				gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config(),
			)
		}
	})

	for _, tt := range testCases {
		for _, flowProto := range []string{ipv4, ipv6} {
			td := testData{
				flowProto:  flowProto,
				ate:        ate,
				otg:        otg,
				otgConfig:  otgConfig,
				srcAtePort: atePort1,
				dstAtePort: atePort2,
			}

			t.Run(fmt.Sprintf("%s-%s", tt.name, flowProto), func(t *testing.T) {
				runTest(
					t,
					tt,
					td,
					td.waitBundle,
				)
			})
		}
	}
}
