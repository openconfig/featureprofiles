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

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
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
	mtu                       = 9216
	trafficRunDuration        = 15 * time.Second
	trafficStopWaitDuration   = 10 * time.Second
	acceptablePacketSizeDelta = 0.5
	acceptableLossPercent     = 0.5
	subInterfaceIndex         = 0
)

var (
	dutSrc = &attrs.Attributes{
		Name:    "dutSrc",
		MAC:     "00:12:01:01:01:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	dutDst = &attrs.Attributes{
		Name:    "dutDst",
		MAC:     "00:12:02:01:01:01",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	ateSrc = &attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	ateDst = &attrs.Attributes{
		Name:    "ateDst",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": dutSrc,
		"port2": dutDst,
	}

	atePorts = map[string]*attrs.Attributes{
		"port1": ateSrc,
		"port2": ateDst,
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

type testDefinition struct {
	name     string
	desc     string
	flowSize uint32
}

type testData struct {
	flowProto   string
	otg         *otg.OTG
	dut         *ondatra.DUTDevice
	otgConfig   gosnappi.Config
	dutLAGNames []string
}

func (d *testData) waitInterface(t *testing.T) {
	otgutils.WaitForARP(t, d.otg, d.otgConfig, d.flowProto)
}

func createFlow(flowName string, flowSize uint32, ipv string) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().
		SetTxNames([]string{fmt.Sprintf("%s.%s", ateSrc.Name, ipv)}).
		SetRxNames([]string{fmt.Sprintf("%s.%s", ateDst.Name, ipv)})
	ethHdr := flow.Packet().Add().Ethernet()
	ethHdr.Src().SetValue(ateSrc.MAC)
	flow.SetSize(gosnappi.NewFlowSize().SetFixed(flowSize))

	switch ipv {
	case ipv4:
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
		v4.DontFragment().SetValue(1)
	case ipv6:
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(ateSrc.IPv6)
		v6.Dst().SetValue(ateDst.IPv6)
	}

	flow.EgressPacket().Add().Ethernet()

	return flow
}

func runTest(t *testing.T, tt testDefinition, td testData, waitF func(t *testing.T)) {
	t.Logf("Name: %s, Description: %s", tt.name, tt.desc)

	flowParams := createFlow(tt.name, tt.flowSize, td.flowProto)
	td.otgConfig.Flows().Clear()
	td.otgConfig.Flows().Append(flowParams)
	td.otg.PushConfig(t, td.otgConfig)
	time.Sleep(time.Second * 30)
	td.otg.StartProtocols(t)
	waitF(t)

	td.otg.StartTraffic(t)
	time.Sleep(trafficRunDuration)

	td.otg.StopTraffic(t)
	time.Sleep(trafficStopWaitDuration)

	otgutils.LogFlowMetrics(t, td.otg, td.otgConfig)

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
	packetSizeDelta := float32(avgPacketSize-tt.flowSize) / (float32(avgPacketSize+tt.flowSize) / 2) * 100

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
	gnmi.Replace(
		t,
		dut,
		gnmi.OC().Interface(port.Name()).Config(),
		portAttrs.NewOCInterface(port.Name(), dut),
	)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, port)
	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, port.Name(), deviations.DefaultNetworkInstance(dut), subInterfaceIndex)
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
	switch {
	case deviations.OmitL2MTU(dut):
		configuredIpv4SubInterfaceMtu := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Subinterface(subInterfaceIndex).Ipv4().Mtu().State())
		configuredIpv6SubInterfaceMtu := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Subinterface(subInterfaceIndex).Ipv6().Mtu().State())
		expectedSuBInterfaceMtu := mtu

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
	default:
		configuredInterfaceMtu := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Mtu().State())
		expectedInterfaceMtu := mtu + 14

		if int(configuredInterfaceMtu) != expectedInterfaceMtu {
			t.Errorf(
				"dut %s configured mtu is incorrect, got: %d, want: %d",
				dut.Name(), configuredInterfaceMtu, expectedInterfaceMtu,
			)
		}
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
		deleteBatch := &gnmi.SetBatch{}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			netInst := &oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}

			for portName := range dutPorts {
				gnmi.BatchDelete(
					deleteBatch,
					gnmi.OC().
						NetworkInstance(*netInst.Name).
						Interface(fmt.Sprintf("%s.%d", dut.Port(t, portName).Name(), subInterfaceIndex)).
						Config(),
				)
			}
		}

		for portName := range dutPorts {
			gnmi.BatchDelete(
				deleteBatch,
				gnmi.OC().
					Interface(dut.Port(t, portName).Name()).
					Subinterface(subInterfaceIndex).
					Config(),
			)
			gnmi.BatchDelete(deleteBatch, gnmi.OC().Interface(dut.Port(t, portName).Name()).Mtu().Config())
		}
		deleteBatch.Set(t, dut)
	})

	for _, tt := range testCases {
		for _, flowProto := range []string{ipv4, ipv6} {
			td := testData{
				flowProto: flowProto,
				otg:       otg,
				otgConfig: otgConfig,
			}

			t.Run(fmt.Sprintf("%s-%s", tt.name, flowProto), func(t *testing.T) {
				runTest(t, tt, td, td.waitInterface)
			})
		}
	}
}

func configureDUTBundle(t *testing.T, dut *ondatra.DUTDevice, lag *attrs.Attributes, bundleMembers []*ondatra.Port) string {
	bundleID := netutil.NextAggregateInterface(t, dut)
	ocRoot := &oc.Root{}
	if deviations.AggregateAtomicUpdate(dut) {
		deleteBatch := &gnmi.SetBatch{}
		gnmi.BatchDelete(deleteBatch, gnmi.OC().Interface(bundleID).Aggregation().MinLinks().Config())
		for _, port := range bundleMembers {
			gnmi.BatchDelete(deleteBatch, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
		}
		deleteBatch.Set(t, dut)
		bundle := ocRoot.GetOrCreateInterface(bundleID)
		bundle.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
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

		gnmi.Update(t, dut, gnmi.OC().Config(), ocRoot)
	}

	agg := ocRoot.GetOrCreateInterface(bundleID)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.Description = ygot.String(fmt.Sprintf("dutLag-%s", bundleID))
	if deviations.InterfaceEnabled(dut) {
		agg.Enabled = ygot.Bool(true)
	}

	subInterface := agg.GetOrCreateSubinterface(subInterfaceIndex)
	v4SubInterface := subInterface.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		v4SubInterface.Enabled = ygot.Bool(true)
	}
	v4Address := v4SubInterface.GetOrCreateAddress(lag.IPv4)
	v4Address.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	v6SubInterface := subInterface.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		v6SubInterface.Enabled = ygot.Bool(true)
	}
	v6Address := v6SubInterface.GetOrCreateAddress(lag.IPv6)
	v6Address.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	intfAgg := agg.GetOrCreateAggregation()
	intfAgg.LagType = oc.IfAggregate_AggregationType_STATIC

	switch {
	case deviations.OmitL2MTU(dut):
		v4SubInterface.SetMtu(mtu)
		v6SubInterface.SetMtu(mtu)
	default:
		agg.Mtu = ygot.Uint16(mtu + 14)
	}

	aggPath := gnmi.OC().Interface(bundleID)
	gnmi.Replace(t, dut, aggPath.Config(), agg)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, bundleID, deviations.DefaultNetworkInstance(dut), subInterfaceIndex)
	}

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

		intfPath := gnmi.OC().Interface(port.Name())

		gnmi.Replace(t, dut, intfPath.Config(), intf)
	}

	verifyDUTPort(t, dut, *agg.Name)

	return bundleID
}

func configureATEBundles(
	allAtePorts []*ondatra.Port,
	bundleMemberCount int,
) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()
	configureATEBundle(otgConfig, ateSrc, dutSrc, allAtePorts[0:bundleMemberCount], 1)
	configureATEBundle(otgConfig, ateDst, dutDst, allAtePorts[bundleMemberCount:2*bundleMemberCount], 2)

	portNames := make([]string, len(allAtePorts))
	for idx, port := range allAtePorts {
		portNames[idx] = port.ID()
	}

	// note that it seems max in otg containers is 9000 so bundle tests > 1500 bytes will fail,
	// for whatever reason individual ports work just fine > 1500 bytes though! also, physical gear
	// seems to work just fine as well, so we'll set this to the max we can for kne tests.
	layer1 := otgConfig.Layer1().Add().SetName("layerOne").SetPortNames(portNames).SetMtu(9000)

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
) {
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

	lagOne := configureDUTBundle(t, dut, dutSrc, lagOneDutBundleMembers)
	lagTwo := configureDUTBundle(t, dut, dutDst, lagTwoDutBundleMembers)

	otgConfig := configureATEBundles(allAtePorts, bundleMemberCount)

	t.Cleanup(func() {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			netInst := &oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}

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
			gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
		}
	})

	for _, tt := range testCases {
		for _, flowProto := range []string{ipv4, ipv6} {
			td := testData{
				flowProto:   flowProto,
				otg:         otg,
				dut:         dut,
				otgConfig:   otgConfig,
				dutLAGNames: []string{lagOne, lagTwo},
			}

			t.Run(fmt.Sprintf("%s-%s", tt.name, flowProto), func(t *testing.T) {
				runTest(t, tt, td, td.waitInterface)
			})
		}
	}
}
