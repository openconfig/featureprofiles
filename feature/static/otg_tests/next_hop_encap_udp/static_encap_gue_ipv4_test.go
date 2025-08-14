package gue_encapsulation_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
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
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4 = 30
	plenIPv6 = 126
	dutAS    = 65501
	ateAS    = 65502

	// Static and GUE address
	nexthopGroupName   = "GUE-NHG"
	nexthopGroupNameV6 = "GUE-NHGv6"
	GuePolicyName      = "GUE-Policy"
	pnhIPv6            = "fc00:10::1"
	pnhIPv6NGv6        = "fc00:10::2"
	staticpnhIPv6      = "fc00:10::1/128"
	staticpngv6IPv6    = "fc00:10::2/128"
	gueDstIPv4         = "10.50.50.1"
	staticgueDstIPv4   = "10.50.50.1/32"
	trafficDstNetIPv4  = "10.1.1.1"
	trafficDstNetIPv6  = "2001:DB8::10:1:1:1"

	// Random Source Address
	trafficSrcNetIPv4 = "1.1.1.1"
	trafficSrcNetIPv6 = "2001:db8::aaaa:0"

	// Lag config
	lag1Name    = "lag1"
	lag2Name    = "lag2"
	dutLag1Name = "Port-Channel1"
	dutLag2Name = "Port-Channel2"

	frameSize          = 512
	packetCount        = 12000000
	gueProtocolPort    = 6080
	gueSrcProtocolPort = 5000

	tolerancePct     = 6.0
	tolerance        = 0.06
	bgpPeerGroupName = "BGP-PEER-GROUP"
)

var (
	atePort1 = &attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.168.10.1",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:168:10:1",
		IPv6Len: plenIPv6,
		MAC:     "02:00:00:00:00:01",
	}

	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.168.10.2",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:168:10:2",
		IPv6Len: plenIPv6,
		MAC:     "00:1a:11:00:00:02",
	}

	ateLAG1 = &attrs.Attributes{
		Name:    "ateLag1",
		IPv4:    "192.168.20.1",
		IPv4Len: plenIPv4,
		MAC:     "02:00:00:00:01:01",
	}

	dutLAG1 = attrs.Attributes{
		Desc:    "DUT to ATE LAG1",
		IPv4:    "192.168.20.2",
		IPv4Len: plenIPv4,
	}

	ateLAG2 = &attrs.Attributes{
		Name:    "ateLag2",
		IPv4:    "192.168.30.1",
		IPv4Len: plenIPv4,
		MAC:     "02:00:00:00:02:01",
	}

	dutLAG2 = attrs.Attributes{
		Desc:    "DUT to ATE LAG2",
		IPv4:    "192.168.30.2",
		IPv4Len: plenIPv4,
	}

	otgPorts = map[string]*attrs.Attributes{
		"port1": atePort1,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": &dutPort1,
	}
)

var capturePorts = []string{"port2", "port3", "port4", "port5"}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))

	// Ports 2 and 3 will be part of LAG
	dutAggPorts1 := []*ondatra.Port{
		dut.Port(t, "port2"),
		dut.Port(t, "port3"),
	}
	configureDUTLag(t, dut, dutAggPorts1, dutLag1Name, dutLAG1)

	// Ports 4 and 5 will be part of LAG
	dutAggPorts2 := []*ondatra.Port{
		dut.Port(t, "port4"),
		dut.Port(t, "port5"),
	}
	configureDUTLag(t, dut, dutAggPorts2, dutLag2Name, dutLAG2)

	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	t.Log("Configuring BGP")
	bgpProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := bgpProtocol.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)

	pg := bgp.GetOrCreatePeerGroup(bgpPeerGroupName)
	pg.PeerAs = ygot.Uint32(ateAS)

	ipv6Nbr := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	ipv6Nbr.PeerGroup = ygot.String(bgpPeerGroupName)
	ipv6Nbr.PeerAs = ygot.Uint32(ateAS)
	ipv6Nbr.Enabled = ygot.Bool(true)
	ipv6Nbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	gnmi.Update(t, dut, d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgpProtocol)

	// Configure GUE Encap
	configureGueTunnel(t, dut, gueDstIPv4, 0, 0)

	t.Log("Configuring Static Routes")

	// Configuring Static Route: IPv4-DST-GUE --> ATE:LAG1:IPv4 & ATE:LAG2:IPv4.
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticgueDstIPv4,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(ateLAG1.IPv4),
			"1": oc.UnionString(ateLAG2.IPv4),
		},
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)

	// Configuring Static Route: PNH-IPv6 --> IPv4 GUE tunnel.
	sV4 = &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticpnhIPv6,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nexthopGroupName),
		},
	}

	cfgplugins.NewStaticRouteNextHopGroupCfg(t, b, sV4, dut, nexthopGroupName)

	sV4 = &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticpngv6IPv6,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"1": oc.UnionString(nexthopGroupNameV6),
		},
	}

	cfgplugins.NewStaticRouteNextHopGroupCfg(t, b, sV4, dut, nexthopGroupNameV6)

	// Apply load balancing
	if deviations.LoadIntervalNotSupported(dut) {
		cli := `
			load-balance policies
			load-balance sand profile default
			packet-type gue outer-ip
			`
		helpers.GnmiCLIConfig(t, dut, cli)
	} else {
		// TODO: OC does not yet support selecting the load-balancing hash mode on LAG members.
		t.Logf("Load balancing is currently not supported via OpenConfig. Will fix once it's implemented.")
	}
}

func configureDUTLag(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string, dutLag attrs.Attributes) {
	t.Helper()
	for _, port := range aggPorts {
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Ethernet().Config())
	}
	setupAggregateAtomically(t, dut, aggPorts, aggID)
	agg := dutLag.NewOCInterface(aggID, dut)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	gnmi.Replace(t, dut, gnmi.OC().Interface(aggID).Config(), agg)
	for _, port := range aggPorts {
		d := &oc.Root{}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
	}
}

func setupAggregateAtomically(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	d := &oc.Root{}
	agg := d.GetOrCreateInterface(aggID)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC

	for _, port := range aggPorts {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

// configureGueTunnel configures a GUE tunnel with optional ToS and TTL.
func configureGueTunnel(t *testing.T, dut *ondatra.DUTDevice, dstAddr string, tos, ttl uint8) {
	t.Helper()
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	// Create nexthop group for v4
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, "V4Udp", ni, dstAddr, nexthopGroupName, tos, ttl, false)
	// Create nexthop group for v6
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, "V6Udp", ni, dstAddr, nexthopGroupNameV6, tos, ttl, false)

	// Configure traffic policy
	if deviations.PolicyForwardingOCUnsupported(dut) {
		cfgplugins.CreatePolicyForwardingNexthopConfig(t, dut, GuePolicyName, "rule1", "ipv4", nexthopGroupName)
		cfgplugins.CreatePolicyForwardingNexthopConfig(t, dut, GuePolicyName, "rule2", "ipv6", nexthopGroupNameV6)
	} else {
		cfgplugins.ConfigurePolicyForwardingNextHopFromOC(t, dut, GuePolicyName, 1, "", nexthopGroupName)
	}

	// Apply traffic policy on interface
	interfacePolicyParams := cfgplugins.OcPolicyForwardingParams{
		InterfaceID:       dut.Port(t, "port1").Name(),
		AppliedPolicyName: GuePolicyName,
	}
	cfgplugins.InterfacePolicyForwardingApply(t, dut, dut.Port(t, "port1").Name(), GuePolicyName, ni, interfacePolicyParams)

	// Create global settingd for gue tunnel
	if deviations.PolicyForwardingOCUnsupported(dut) {
		cfgplugins.ConfigureUdpEncapHeader(t, dut, "ipv4-over-udp", fmt.Sprintf("%d", gueProtocolPort))
		cfgplugins.ConfigureUdpEncapHeader(t, dut, "ipv6-over-udp", fmt.Sprintf("%d", gueProtocolPort))
	} else {
		// TODO: OC support of gue encapsulation is not present
	}
}

func configureTosTtlOnTunnel(t *testing.T, dut *ondatra.DUTDevice, policyName string, protocolType string, nexthopGroupName string, tos, ttl uint8, deleteTos, deleteTtl bool) {
	if tos != 0 || deleteTos {
		cfgplugins.ConfigureTOSGUE(t, dut, policyName, uint32(tos>>5), dut.Port(t, "port1").Name(), deleteTos)
	}

	if ttl != 0 || deleteTtl {
		d := &oc.Root{}
		ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, protocolType, ni, "", nexthopGroupName, tos, ttl, deleteTtl)
	}

}

// configureATE creates the base OTG configuration.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	otgConfig := gosnappi.NewConfig()

	for portName, portAttrs := range otgPorts {
		port := ate.Port(t, portName)
		dutPort := dutPorts[portName]
		portAttrs.AddToOTG(otgConfig, port, dutPort)
	}

	d1 := otgConfig.Devices().Items()[0]
	bgpD := d1.Bgp().SetRouterId(atePort1.IPv4)

	iDut1Ipv6 := d1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Nbr := bgpD.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP.peer")
	bgp6Nbr.SetPeerAddress(dutPort1.IPv6).SetAsNumber(uint32(ateAS)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	// Configure IPv4-DST-NET/32 and IPv6-DST-NET/128 routes to DUT
	v4Route := bgp6Nbr.V4Routes().Add().SetName("v4Net")
	v4Route.Addresses().Add().SetAddress(trafficDstNetIPv4).SetPrefix(32).SetCount(1)
	v4Route.SetNextHopIpv6Address(pnhIPv6).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)

	v6Route := bgp6Nbr.V6Routes().Add().SetName("v6Net")
	v6Route.Addresses().Add().SetAddress(trafficDstNetIPv6).SetPrefix(128).SetCount(1)
	v6Route.SetNextHopIpv6Address(pnhIPv6NGv6).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)

	// Configure Lag1 on ATE
	ateAggPorts := []*ondatra.Port{
		ate.Port(t, "port2"),
		ate.Port(t, "port3"),
	}
	configureATEBundle(t, otgConfig, ateAggPorts, lag1Name, *ateLAG1, dutLAG1.IPv4, 1)

	// Configure Lag2 on ATE
	ateAggPorts = []*ondatra.Port{
		ate.Port(t, "port4"),
		ate.Port(t, "port5"),
	}
	configureATEBundle(t, otgConfig, ateAggPorts, lag2Name, *ateLAG2, dutLAG2.IPv4, 2)
	return otgConfig

}

func configureATEBundle(t *testing.T, top gosnappi.Config, aggPorts []*ondatra.Port, lagName string, lagAttrs attrs.Attributes, gateway string, aggID uint32) {
	lag := top.Lags().Add().SetName(lagName)
	lag.Protocol().Static().SetLagId(aggID)

	for i, p := range aggPorts {
		port := top.Ports().Add().SetName(p.ID())
		mac, err := incrementMAC(lagAttrs.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		lag.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(mac).SetName("LAGRx-" + strconv.Itoa(i))
	}

	dstDev := top.Devices().Add().SetName(lagName + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(lagName + ".Eth").SetMac(lagAttrs.MAC)
	dstEth.Connection().SetLagName(lagName)

	ipv4 := dstEth.Ipv4Addresses().Add().SetName(lagAttrs.Name + ".IPv4")
	ipv4.SetAddress(lagAttrs.IPv4).SetGateway(gateway).SetPrefix(uint32(lagAttrs.IPv4Len))

}

func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

func createFlow(otgConfig gosnappi.Config, flowName string, flowtype string, ipAddr string, tos int, ttl uint32, singleFlow bool, dstport uint32, srcPort uint32, icmp bool) {
	otgConfig.Flows().Clear()
	flow := otgConfig.Flows().Add().SetName(flowName)
	flow.TxRx().Port().
		SetTxName(otgConfig.Ports().Items()[0].Name()).
		SetRxNames([]string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()})

	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(frameSize)
	flow.Rate().SetPercentage(10)
	flow.Duration().FixedPackets().SetPackets(packetCount)

	e1 := flow.Packet().Add().Ethernet()
	e1.Dst().SetValue(dutPort1.MAC)

	// Adding outer IP header
	if flowtype == "ipv4" {
		v4 := flow.Packet().Add().Ipv4()
		if singleFlow {
			v4.Src().SetValue(trafficSrcNetIPv4)
			v4.Dst().SetValue(ipAddr)
			// udp := flow.Packet().Add().Udp()
			// udp.DstPort().SetValue(dstport)
			// udp.SrcPort().SetValue(srcPort)
		} else {
			v4.Src().Increment().SetStart(trafficSrcNetIPv4).SetCount(256)
			v4.Dst().Increment().SetStart(ipAddr).SetCount(256)
		}
		v4.Priority().Tos().Precedence().SetValue(uint32(tos >> 5))
		v4.TimeToLive().SetValue(ttl)
		if icmp {
			flow.Packet().Add().Icmp()
		}
	} else {
		v6 := flow.Packet().Add().Ipv6()
		if singleFlow {
			v6.Src().SetValue(trafficSrcNetIPv6)
			v6.Dst().SetValue(ipAddr)
			udp := flow.Packet().Add().Udp()
			udp.DstPort().SetValue(dstport)
			udp.SrcPort().SetValue(srcPort)
		} else {
			v6.Src().Increment().SetStart(trafficSrcNetIPv6).SetCount(256)
			v6.Dst().Increment().SetStart(ipAddr).SetCount(256)
			udp := flow.Packet().Add().Udp()
			udp.DstPort().Increment().SetStart(dstport).SetCount(256)
			udp.SrcPort().Increment().SetStart(srcPort).SetCount(256)
		}
		v6.TrafficClass().SetValue(uint32(tos))
		v6.HopLimit().SetValue(ttl)
		if icmp {
			flow.Packet().Add().Icmpv6()
		}
	}

}

// verifyBGPTelemetry checks that the BGP session is established.
func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := bgpPath.Neighbor(atePort1.IPv6)
	statePath := nbrPath.SessionState()
	_, ok := gnmi.Watch(t, dut, statePath.State(), time.Minute*2, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP Neighbor state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Fatal("BGP session did not establish")
	}

	t.Log("BGP session established")
}

// verifyTraffic checks packet counts and ECMP load balancing.
func verifyTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string) {
	var totalEgressPkts uint64

	for _, egressPort := range []string{"port2", "port3", "port4", "port5"} {
		port := dut.Port(t, egressPort)
		egressPktPath := gnmi.OC().Interface(port.Name()).Counters().OutUnicastPkts().State()
		egressPkt, present := gnmi.Lookup(t, dut, egressPktPath).Val()
		if !present {
			t.Errorf("Get IsPresent status for path %q: got false, want true", egressPktPath)
		}
		totalEgressPkts += egressPkt
	}

	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, config)

	txPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State())
	if txPkts == 0 {
		t.Fatal("ATE did not send any packets")
	}

	dutlossPct := (float64(txPkts) - float64(totalEgressPkts)) * 100 / float64(txPkts)
	if dutlossPct < tolerancePct {
		t.Log("The packet count of traffic sent from ATE port1 equal to the sum of all packets on DUT egress ports")
	} else {
		t.Errorf("The packet count of traffic sent from ATE port1 is not equal to the sum of all packets on DUT egress ports")
	}

	rxPortNames := []string{"port2", "port3", "port4", "port5"}
	var totalRxPkts, lag1RxPkts, lag2RxPkts uint64
	portRxPkts := make(map[string]uint64)

	for _, portName := range rxPortNames {
		rxPkts := gnmi.Get(t, otg, gnmi.OTG().Port(ate.Port(t, portName).ID()).Counters().InFrames().State())
		portRxPkts[portName] = rxPkts
		totalRxPkts += rxPkts
	}

	lossPct := (float64(txPkts) - float64(totalRxPkts)) * 100 / float64(txPkts)
	if lossPct > tolerancePct {
		t.Errorf("High packet loss detected: got %f%%, want < %f%%", lossPct, tolerancePct)
	} else {
		t.Logf("Packet loss is within tolerance: %f%%", lossPct)
	}

	if totalRxPkts == 0 {
		t.Skip("Skipping load balancing checks as no packets were received.")
	}

	lag1RxPkts = portRxPkts["port2"] + portRxPkts["port3"]
	lag2RxPkts = portRxPkts["port4"] + portRxPkts["port5"]
	t.Logf("LAG1 received %d packets, LAG2 received %d packets", lag1RxPkts, lag2RxPkts)
	ecmpDiff := float64(lag1RxPkts) - float64(lag2RxPkts)
	if ecmpDiff < 0 {
		ecmpDiff = -ecmpDiff
	}
	ecmpError := ecmpDiff / float64(totalRxPkts)
	if ecmpError > tolerance {
		t.Errorf("ECMP hashing between LAG1 and LAG2 is unbalanced. Difference: %f%%, want < %f%%", ecmpError*100, tolerance*100)
	} else {
		t.Logf("ECMP hashing between LAGs is balanced. Difference: %f%%", ecmpError*100)
	}
}

func enableCapture(t *testing.T, config gosnappi.Config, portNames []string) {
	config.Captures().Clear()
	for _, portName := range portNames {
		t.Logf("Configuring packet capture for OTG port: %s", portName)
		cap := config.Captures().Add()
		cap.SetName(portName)
		cap.SetPortNames([]string{portName})
		cap.SetFormat(gosnappi.CaptureFormat.PCAP)
	}
}

func startCapture(t *testing.T, otg *otg.OTG) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)

	return cs
}

func stopCapture(t *testing.T, otg *otg.OTG, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

func processCapture(t *testing.T, otg *otg.OTG, port string) string {
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(30 * time.Second)
	capture, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := capture.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	defer capture.Close()

	return capture.Name()
}

func validatePackets(t *testing.T, filename string, protocolType string, outertos, innertos string, outerttl, innerttl uint8, outerPacket bool) {
	var packetCount uint32 = 0

	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		packetCount += 1
		ipOuterLayer, ok := ipLayer.(*layers.IPv4)
		if !ok || ipOuterLayer == nil {
			t.Errorf("Outer IP layer not found %d", ipLayer)
			return
		}

		udpLayer := packet.Layer(layers.LayerTypeUDP)
		udp, ok := udpLayer.(*layers.UDP)
		if !ok || udp == nil {
			t.Error("GUE layer not found")
			return
		} else {
			if udp.DstPort == gueProtocolPort {
				t.Log("Got the encapsulated GUE layer")
			}

			if outerPacket {
				validateOuterPacket(t, ipOuterLayer, outertos, outerttl)
			}

			var gotInnerPacketTOS, gotInnerttl uint8
			dscp := ""

			switch protocolType {
			case "ipv4":
				innerPacket := gopacket.NewPacket(udp.Payload, layers.LayerTypeIPv4, gopacket.Default)
				ipLayer := innerPacket.Layer(layers.LayerTypeIPv4)
				if ipLayer == nil {
					t.Errorf("Inner layer of type %s not found", protocolType)
					return
				}
				ip, _ := ipLayer.(*layers.IPv4)
				gotInnerPacketTOS = ip.TOS
				dscp = fmt.Sprintf("0x%X", gotInnerPacketTOS)
				gotInnerttl = ip.TTL
			case "ipv6":
				innerPacket := gopacket.NewPacket(udp.Payload, layers.LayerTypeIPv6, gopacket.Default)
				ipLayer := innerPacket.Layer(layers.LayerTypeIPv6)
				if ipLayer == nil {
					t.Errorf("Inner layer of type %s not found", protocolType)
					return
				}
				ip, _ := ipLayer.(*layers.IPv6)
				gotInnerPacketTOS = ip.TrafficClass
				dscp = fmt.Sprintf("0x%X", gotInnerPacketTOS)
				gotInnerttl = ip.HopLimit
			}

			if innerttl != 0 {
				if innerttl == gotInnerttl {
					t.Logf("TTL matched: expected ttl %d, got ttl %d", innerttl, gotInnerttl)
				} else {
					t.Errorf("TTL mismatch: expected ttl %d, got ttl %d", innerttl, gotInnerttl)
				}
			}
			if innertos != "" {
				if dscp == innertos {
					t.Logf("TOS matched: expected TOS %v, got TOS %v", innertos, dscp)
				} else {
					t.Errorf("TOS mismatch: expected TOS %v, got TOS %v", innertos, dscp)
				}
			}

		}
		break
	}

}

func validateOuterPacket(t *testing.T, outerPacket *layers.IPv4, tos string, ttl uint8) {
	var outerTOS, outerttl uint8
	outerDSCP := ""

	outerttl = outerPacket.TTL
	outerTOS = outerPacket.TOS
	outerDSCP = fmt.Sprintf("0x%X", outerTOS)

	if ttl != 0 {
		if outerttl == ttl {
			t.Logf("Outer TTL matched: expected ttl %d, got ttl %d", ttl, outerttl)
		} else {
			t.Errorf("Outer TTL mismatch: expected ttl %d, got ttl %d", ttl, outerttl)
		}
	}
	if tos != "" {
		if outerDSCP == tos {
			t.Logf("Outer TOS matched: expected TOS %v, got TOS %v", tos, outerDSCP)
		} else {
			t.Errorf("Outer TOS mismatch: expected TOS %v, got TOS %v", tos, outerDSCP)
		}
	}
}

// verifySinglePathTraffic checks that traffic is received on only one egress port.
func verifySinglePathTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string) []string {
	otg := ate.OTG()

	otgutils.LogFlowMetrics(t, otg, config)

	txPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State())
	if txPkts == 0 {
		t.Fatal("ATE did not send any packets")
	}

	rxPortNames := []string{"port2", "port3", "port4", "port5"}
	var totalRxPkts uint64
	var activePorts []string

	for _, portName := range rxPortNames {
		rxPkts := gnmi.Get(t, otg, gnmi.OTG().Port(ate.Port(t, portName).ID()).Counters().InFrames().State())
		totalRxPkts += rxPkts
		if rxPkts >= packetCount {
			activePorts = append(activePorts, portName)
		}
	}

	lossPct := (float64(txPkts) - float64(totalRxPkts)) * 100 / float64(txPkts)
	if lossPct > tolerancePct {
		t.Errorf("High packet loss detected: got %f%%, want < %f%%", lossPct, tolerancePct)
	} else {
		t.Logf("Packet loss is within tolerance: %f%%", lossPct)
	}

	if len(activePorts) == 0 {
		t.Errorf("No traffic was received on any destination port.")
	} else if len(activePorts) > 1 {
		t.Errorf("Traffic was received on multiple ports %v, expected only one.", activePorts)
	} else {
		t.Logf("Traffic correctly hashed to a single port: %s", activePorts[0])
	}

	return activePorts
}

// verifyTTLDropAndICMP verifies that the TTL=1 flow is dropped and ICMP responses are received.
func verifyTTLDropAndICMP(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowname string, captureName string) {
	t.Helper()
	otg := ate.OTG()

	otgutils.LogFlowMetrics(t, otg, config)

	outTxPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flowname).Counters().OutPkts().State())
	if outTxPkts == 0 {
		t.Fatal("ATE did not send any packets for the TTL=1 flow")
	}

	rxPortNames := []string{"port2", "port3", "port4", "port5"}
	var totalRxPkts uint64

	for _, portName := range rxPortNames {
		rxPkts := gnmi.Get(t, otg, gnmi.OTG().Port(ate.Port(t, portName).ID()).Counters().InFrames().State())
		totalRxPkts += rxPkts
	}

	if totalRxPkts >= packetCount {
		t.Errorf("Received %d packets on destination ports, but expected 0 due to TTL expiry.", totalRxPkts)
	} else {
		t.Logf("Successfully verified that no packets for flow %s were received at the GUE destinations.", flowname)
	}

	handle, err := pcap.OpenOffline(captureName)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
		if ipv6Layer != nil {
			icmpv6Layer := packet.Layer(layers.LayerTypeICMPv6)
			if icmpv6Layer != nil {
				icmpPacket, _ := icmpv6Layer.(*layers.ICMPv6)
				if icmpPacket.TypeCode.Type() == layers.ICMPv6TypeRouterAdvertisement && icmpPacket.TypeCode.Code() == 0 {
					t.Logf("PASS: received ICMPv6 code-0")
				} else {
					t.Errorf("FAIL: did not received ICMPv6 code-0, Got Code: %v , type: %v", icmpPacket.TypeCode.Code(), icmpPacket.TypeCode.Type())
				}
			}
		}
	}
}

func TestGUEEncap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)
	otgConfig := configureATE(t, ate)

	t.Run("RT-3.53.1: IPv4 GUE Encap without explicit ToS/TTL", func(t *testing.T) {

		createFlow(otgConfig, "IPv4-GUE-Traffic", "ipv4", trafficDstNetIPv4, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)

		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)

		verifyBGPTelemetry(t, dut)

		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(50 * time.Second)
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, dut, ate, otgConfig, "IPv4-GUE-Traffic")

		for _, p := range capturePorts {
			capture := processCapture(t, ate.OTG(), p)
			validatePackets(t, capture, "ipv4", "0x80", "0x80", 64, 9, true)
		}

	})

	t.Run("RT-3.53.2: IPv6 traffic GUE encapsulation without explicit ToS/TTL configuration on tunnel", func(t *testing.T) {

		createFlow(otgConfig, "IPv6-GUE-Traffic", "ipv6", trafficDstNetIPv6, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)
		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)

		verifyBGPTelemetry(t, dut)

		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(90 * time.Second)
		t.Log("Stopping traffic")
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, dut, ate, otgConfig, "IPv6-GUE-Traffic")

		for _, p := range capturePorts {
			capture := processCapture(t, ate.OTG(), p)
			validatePackets(t, capture, "ipv6", "0x80", "0x80", 64, 9, false)
		}

	})

	t.Run("RT-3.53.3: IPv4 GUE Encap with explicit ToS", func(t *testing.T) {
		t.Log("Configure DUT with explicit ToS on GUE tunnel")
		configureTosTtlOnTunnel(t, dut, "policy1", "", "", 0x60, 0, false, false)

		createFlow(otgConfig, "IPv4-GUE-Traffic", "ipv4", trafficDstNetIPv4, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)

		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)

		verifyBGPTelemetry(t, dut)
		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(90 * time.Second)
		t.Log("Stopping traffic")
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, dut, ate, otgConfig, "IPv4-GUE-Traffic")

		for _, p := range capturePorts {
			capture := processCapture(t, ate.OTG(), p)
			validatePackets(t, capture, "ipv4", "", "0x60", 0, 0, false)
		}
	})

	t.Run("RT-3.53.4: IPv6 GUE Encap with explicit ToS", func(t *testing.T) {
		// Configure TOS on ATE, and validate the TOS is copied by DUT on outer header
		createFlow(otgConfig, "IPv6-GUE-Traffic", "ipv6", trafficDstNetIPv6, 0x60, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)
		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)

		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(90 * time.Second)
		t.Log("Stopping traffic")
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, dut, ate, otgConfig, "IPv6-GUE-Traffic")

		for _, p := range capturePorts {
			capture := processCapture(t, ate.OTG(), p)
			validatePackets(t, capture, "ipv6", "", "0x60", 0, 0, false)
		}
	})

	t.Run("RT-3.53.5: IPv4 GUE Encap with explicit TTL", func(t *testing.T) {
		configureTosTtlOnTunnel(t, dut, "policy1", "V4Udp", nexthopGroupName, 0, 20, true, false)

		createFlow(otgConfig, "IPv4-GUE-Traffic", "ipv4", trafficDstNetIPv4, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)

		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)

		verifyBGPTelemetry(t, dut)

		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(90 * time.Second)
		t.Log("Stopping traffic")
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, dut, ate, otgConfig, "IPv4-GUE-Traffic")

		// TODO: once support added, will add the code
		for _, p := range capturePorts {
			capture := processCapture(t, ate.OTG(), p)
			validatePackets(t, capture, "ipv4", "", "", 20, 0, true)
		}
	})

	t.Run("RT-3.53.6: IPv6 GUE Encap with explicit TTL", func(t *testing.T) {
		configureTosTtlOnTunnel(t, dut, "policy1", "V6Udp", nexthopGroupNameV6, 0, 20, true, false)
		createFlow(otgConfig, "IPv6-GUE-Traffic", "ipv6", trafficDstNetIPv6, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)
		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)
		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)

		verifyBGPTelemetry(t, dut)

		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(90 * time.Second)
		t.Log("Stopping traffic")
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, dut, ate, otgConfig, "IPv6-GUE-Traffic")

		for _, p := range capturePorts {
			capture := processCapture(t, ate.OTG(), p)
			validatePackets(t, capture, "ipv6", "", "", 20, 0, true)
		}
	})

	t.Run("RT-3.53.7: IPv4 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel", func(t *testing.T) {
		configureTosTtlOnTunnel(t, dut, "policy1", "V4Udp", nexthopGroupName, 0x60, 20, false, false)
		createFlow(otgConfig, "IPv4-GUE-Traffic", "ipv4", trafficDstNetIPv4, 0x60, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)

		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)

		verifyBGPTelemetry(t, dut)
		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(90 * time.Second)
		t.Log("Stopping traffic")
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, dut, ate, otgConfig, "IPv4-GUE-Traffic")

		for _, p := range capturePorts {
			capture := processCapture(t, ate.OTG(), p)
			validatePackets(t, capture, "ipv4", "0x60", "", 20, 0, true)
		}
	})

	t.Run("RT-3.53.8: IPv6 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel", func(t *testing.T) {
		configureTosTtlOnTunnel(t, dut, "policy1", "V6Udp", nexthopGroupNameV6, 0x60, 20, false, false)
		createFlow(otgConfig, "IPv6-GUE-Traffic", "ipv6", trafficDstNetIPv6, 0x60, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)
		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)
		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(90 * time.Second)
		t.Log("Stopping traffic")
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, dut, ate, otgConfig, "IPv6-GUE-Traffic")

		for _, p := range capturePorts {
			capture := processCapture(t, ate.OTG(), p)
			validatePackets(t, capture, "ipv6", "0x60", "", 20, 0, true)
		}
	})

	t.Run("RT-3.53.9: IPv4 traffic GUE encapsulation to a single 5-tuple tunnel", func(t *testing.T) {

		createFlow(otgConfig, "IPv4-GUE-Single-Traffic", "ipv4", trafficDstNetIPv4, 0x80, 10, true, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)
		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)

		verifyBGPTelemetry(t, dut)

		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(50 * time.Second)
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		// processCapture(t, ate.OTG(), "port2")
		t.Log("Verify traffic packets")
		activeport := verifySinglePathTraffic(t, ate, otgConfig, "IPv4-GUE-Single-Traffic")

		capture := processCapture(t, ate.OTG(), activeport[0])
		validatePackets(t, capture, "ipv4", "0x60", "0x60", 20, 9, true)

	})

	t.Run("RT-3.53.10: IPv6 traffic GUE encapsulation to a single 5-tuple tunnel", func(t *testing.T) {

		createFlow(otgConfig, "IPv6-GUE-Single-Traffic", "ipv6", trafficDstNetIPv6, 0x80, 10, true, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, otgConfig, capturePorts)
		ate.OTG().PushConfig(t, otgConfig)
		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)

		verifyBGPTelemetry(t, dut)

		t.Logf("Starting capture")
		cs := startCapture(t, ate.OTG())

		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		time.Sleep(50 * time.Second)
		ate.OTG().StopTraffic(t)

		otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
		otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

		t.Logf("Stop Capture")
		stopCapture(t, ate.OTG(), cs)

		t.Log("Verify traffic packets")
		activeport := verifySinglePathTraffic(t, ate, otgConfig, "IPv6-GUE-Single-Traffic")

		capture := processCapture(t, ate.OTG(), activeport[0])
		validatePackets(t, capture, "ipv6", "0x80", "0x80", 20, 9, true)

	})

	t.Run("RT-3.53.11: IPv4 traffic with TTL=1", func(t *testing.T) {
		createFlow(otgConfig, "IPv4-GUE-ttl", "ipv4", trafficDstNetIPv4, 0x80, 1, false, gueProtocolPort, gueSrcProtocolPort, true)
		enableCapture(t, otgConfig, []string{"port1"})
		ate.OTG().PushConfig(t, otgConfig)

		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)
		verifyBGPTelemetry(t, dut)

		cs := startCapture(t, ate.OTG())

		t.Log("Starting TTL=1 traffic test")
		ate.OTG().StartTraffic(t)
		time.Sleep(50 * time.Second)
		ate.OTG().StopTraffic(t)

		time.Sleep(60 * time.Second)
		stopCapture(t, ate.OTG(), cs)
		captureName := processCapture(t, ate.OTG(), "port1")

		t.Log("Verify TTL=1 packets are dropped and ICMP is returned")
		verifyTTLDropAndICMP(t, ate, otgConfig, "IPv4-GUE-ttl", captureName)
	})

	t.Run("RT-3.53.12: IPv6 traffic with TTL=1", func(t *testing.T) {
		createFlow(otgConfig, "IPv6-GUE-ttl", "ipv6", trafficDstNetIPv6, 0x80, 1, false, gueProtocolPort, gueSrcProtocolPort, true)

		enableCapture(t, otgConfig, []string{"port1"})
		ate.OTG().PushConfig(t, otgConfig)

		t.Log("Start ATE protocols and verify BGP session")
		ate.OTG().StartProtocols(t)
		verifyBGPTelemetry(t, dut)

		cs := startCapture(t, ate.OTG())

		t.Log("Starting TTL=1 traffic test")
		ate.OTG().StartTraffic(t)
		time.Sleep(50 * time.Second)
		ate.OTG().StopTraffic(t)

		time.Sleep(60 * time.Second)
		stopCapture(t, ate.OTG(), cs)
		captureName := processCapture(t, ate.OTG(), "port1")

		t.Log("Verify TTL=1 packets are dropped and ICMP is returned")
		verifyTTLDropAndICMP(t, ate, otgConfig, "IPv6-GUE-ttl", captureName)
	})
}
