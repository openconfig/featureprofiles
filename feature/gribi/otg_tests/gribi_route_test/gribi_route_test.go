package gribi_route_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	niTransitTeVrf          = "TRANSIT_TE_VRF"
	niEncapTeVrfA           = "ENCAP_TE_VRF_A"
	niTEVRF111              = "TE_VRF_111"
	vrfPolC                 = "vrf_selection_policy_c"
	seqIDBase               = uint32(10)
	ipv4OuterSrc111         = "198.51.100.111"
	ipv4OuterSrc111WithMask = "198.51.100.111/32"
	ipv4InnerDst            = "138.0.11.8"
	ipv4OuterDst111         = "198.50.100.64"
	ipv4OuterDst222         = "198.50.100.65"
	peerGrpName             = "BGP-PEER-GROUP1"
	asn                     = 65501
	tolerancePct            = 2
	checkEncap              = true
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT Port 3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::192:0:2:9",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		Desc:    "ATE Port 1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		Desc:    "ATE Port 2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	atePort3 = attrs.Attributes{
		Name:    "port3",
		MAC:     "02:00:03:01:01:01",
		Desc:    "ATE Port 3",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::192:0:2:A",
		IPv4Len: 30,
		IPv6Len: 126,
	}
)

type bgpNeighbor struct {
	Name    string
	dutIPv4 string
	ateIPv4 string
	MAC     string
}

var (
	bgpNbr1 = bgpNeighbor{
		Name:    "port1",
		dutIPv4: "192.0.2.1",
		ateIPv4: "192.0.2.2",
		MAC:     "02:00:01:01:01:01",
	}
	bgpNbr2 = bgpNeighbor{
		Name:    "port2",
		dutIPv4: "192.0.2.5",
		ateIPv4: "192.0.2.6",
		MAC:     "02:00:02:01:01:01",
	}
	bgpNbr3 = bgpNeighbor{
		Name:    "port3",
		dutIPv4: "192.0.2.9",
		ateIPv4: "192.0.2.10",
		MAC:     "02:00:03:01:01:01",
	}
)

type packetValidation struct {
	portName        string
	outDstIP        []string
	inHdrIP         string
	validateDecap   bool
	validateNoDecap bool
	validateEncap   bool
}

type policyFwRule struct {
	SeqID      uint32
	family     string
	protocol   oc.UnionUint8
	dscpSet    []uint8
	sourceAddr string
	ni         string
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut       *ondatra.DUTDevice
	ctx       context.Context
	client    *fluent.GRIBIClient
	ate       *ondatra.ATEDevice
	otgConfig gosnappi.Config
	otg       *otg.OTG
}

type flowArgs struct {
	flowName                     string
	outHdrSrcIP, outHdrDstIP     string
	InnHdrSrcIP, InnHdrDstIP     string
	InnHdrSrcIPv6, InnHdrDstIPv6 string
	isIPInIP                     bool
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGRIBIFailover(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
	ate := ondatra.ATE(t, "ate")
	top := configureOTG(t, ate)
	t.Log("Configure VRF_Policy")
	configureVrfSelectionPolicyC(t, dut)
	t.Log("Configure GRIBI")

	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(12, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()
	client.Start(ctx, t)
	defer client.Stop(t)
	gribi.FlushAll(client)
	defer gribi.FlushAll(client)
	client.StartSending(ctx, t)
	gribi.BecomeLeader(t, client)

	tcArgs := &testArgs{
		ctx:    ctx,
		client: client,
		dut:    dut,
	}

	configureGribiRoute(t, dut, tcArgs)

	llAddress, found := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Interface("port1.Eth").Ipv4Neighbor(dutPort1.IPv4).LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		return val.IsPresent()
	}).Await(t)
	if !found {
		t.Fatalf("Could not get the LinkLayerAddress %s", llAddress)
	}
	dstMac, _ := llAddress.Val()

	verifyBgpTelemetry(t, dut)

	args := &testArgs{
		dut:       dut,
		ate:       ate,
		otgConfig: top,
		otg:       ate.OTG(),
	}
	t.Run("RT-14.2.1: Traffic Prefix Match to Tunnel Prefix, Encapped and Egress via Port2", func(t *testing.T) {
		flow := createFlow(&flowArgs{flowName: "flow4in4",
			InnHdrSrcIP: ipv4OuterSrc111, InnHdrDstIP: ipv4InnerDst}, dstMac)
		sendTraffic(t, args, top, ate, flow, 30, []string{"port2"})
		if ok := verifyTrafficFlow(t, ate, flow); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ")
		}
		captureAndValidatePackets(t, args, &packetValidation{portName: atePort2.Name,
			outDstIP: []string{ipv4OuterDst111}, inHdrIP: ipv4InnerDst, validateEncap: true})
	})

	t.Run("RT-14.2.2: Traffic Prefix not Matched to Tunnel Prefix, Egress via Port3", func(t *testing.T) {
		flow := createFlow(&flowArgs{flowName: "flow4in4",
			InnHdrSrcIP: ipv4OuterSrc111, InnHdrDstIP: ipv4OuterDst222}, dstMac)
		sendTraffic(t, args, top, ate, flow, 30, []string{"port3"})
		if ok := verifyTrafficFlow(t, ate, flow); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ")
		}
	})

	t.Run("RT-14.2.3: Traffic Match to Transit_Vrf, Match Tunnel Prefix Egress to Port2", func(t *testing.T) {
		flow := createFlow(&flowArgs{flowName: "flow4in4",
			outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: ipv4OuterDst111,
			InnHdrSrcIP: ipv4OuterSrc111, InnHdrDstIP: ipv4InnerDst, isIPInIP: true}, dstMac)
		sendTraffic(t, args, top, ate, flow, 30, []string{"port2"})
		if ok := verifyTrafficFlow(t, ate, flow); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ")
		}
		captureAndValidatePackets(t, args, &packetValidation{portName: atePort2.Name,
			outDstIP: []string{ipv4OuterDst111}, inHdrIP: ipv4InnerDst, validateNoDecap: true})
	})

	t.Run("RT-14.2.4: Traffic Match to Transit_Vrf, noMatch Tunnel Prefix Egress to Port3", func(t *testing.T) {
		flow := createFlow(&flowArgs{flowName: "flow4in4",
			outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: ipv4OuterDst222,
			InnHdrSrcIP: ipv4OuterSrc111, InnHdrDstIP: ipv4InnerDst, isIPInIP: true}, dstMac)
		sendTraffic(t, args, top, ate, flow, 30, []string{"port3"})
		if ok := verifyTrafficFlow(t, ate, flow); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ")
		}
		captureAndValidatePackets(t, args, &packetValidation{portName: atePort3.Name,
			outDstIP: []string{ipv4OuterDst222}, inHdrIP: ipv4InnerDst, validateDecap: true})
	})
}

// configureDUT configures port1-3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	t.Logf("configureDUT")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	configNonDefaultNetworkInstance(t, dut)
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(asn, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
}

func bgpCreateNbr(localAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutPort3.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg1.PeerAs = ygot.Uint32(localAs)

	bgpNbr := bgp.GetOrCreateNeighbor(bgpNbr3.ateIPv4)
	bgpNbr.PeerGroup = ygot.String(peerGrpName)
	bgpNbr.PeerAs = ygot.Uint32(localAs)
	bgpNbr.Enabled = ygot.Bool(true)
	bgpNbrT := bgpNbr.GetOrCreateTransport()
	bgpNbrT.LocalAddress = ygot.String(bgpNbr3.dutIPv4)
	af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af4.Enabled = ygot.Bool(true)
	af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af6.Enabled = ygot.Bool(true)

	return niProto
}

// configureOTGBGP configure BGP on ATE
func configureOTGBGP(t *testing.T, dev gosnappi.Device, top gosnappi.Config, nbr bgpNeighbor) {
	t.Helper()
	iDutBgp := dev.Bgp().SetRouterId(nbr.ateIPv4)
	iDutBgp4Peer := iDutBgp.Ipv4Interfaces().Add().SetIpv4Name(nbr.Name + ".IPv4").Peers().Add().SetName(nbr.Name + ".BGP4.peer")
	iDutBgp4Peer.SetPeerAddress(nbr.dutIPv4).SetAsNumber(asn).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDutBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(false)

	bgpNeti1Bgp4PeerRoutes := iDutBgp4Peer.V4Routes().Add().SetName(nbr.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(nbr.ateIPv4).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(ipv4InnerDst).SetPrefix(32).SetCount(1)
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Logf("configureOTG")
	config := gosnappi.NewConfig()
	var dev gosnappi.Device
	for i, ap := range []bgpNeighbor{bgpNbr1, bgpNbr2, bgpNbr3} {
		// DUT and ATE ports are connected by the same names.
		port := config.Ports().Add().SetName(ap.Name)
		portName := fmt.Sprintf("port%s", strconv.Itoa(i+1))
		dev = config.Devices().Add().SetName(portName)
		eth := dev.Ethernets().Add().SetName(portName + ".Eth").SetMac(ap.MAC)
		eth.Connection().SetPortName(port.Name())
		eth.Ipv4Addresses().Add().SetName(portName + ".IPv4").
			SetAddress(ap.ateIPv4).SetGateway(ap.dutIPv4).
			SetPrefix(30)
	}
	configureOTGBGP(t, dev, config, bgpNbr3)
	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
	return config
}

// seqIDOffset returns sequence ID offset added with seqIDBase (10), to avoid sequences
// like 1, 10, 11, 12,..., 2, 21, 22, ... while being sent by Ondatra to the DUT.
// It now generates sequences like 11, 12, 13, ..., 19, 20, 21,..., 99.
func seqIDOffset(dut *ondatra.DUTDevice, i uint32) uint32 {
	if deviations.PfRequireSequentialOrderPbrRules(dut) {
		return i + seqIDBase
	}
	return i
}

// configureNetworkInstance configures vrfs DECAP_TE_VRF,ENCAP_TE_VRF_A,ENCAP_TE_VRF_B,
// TE_VRF_222, TE_VRF_111.
func configNonDefaultNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{niTransitTeVrf, niEncapTeVrfA, niTEVRF111}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Name().Config(), deviations.DefaultNetworkInstance(dut))
}

func configureVrfSelectionPolicyC(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	time.Sleep(100 * time.Second)
	dutPolFwdPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()

	pfRule1 := &policyFwRule{SeqID: 1, family: "ipv4", protocol: 4, sourceAddr: ipv4OuterSrc111WithMask,
		ni: niTransitTeVrf}
	pfRule2 := &policyFwRule{SeqID: 2, family: "ipv4", sourceAddr: ipv4OuterSrc111WithMask,
		ni: niEncapTeVrfA}

	pfRuleList := []*policyFwRule{pfRule1, pfRule2}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niP := ni.GetOrCreatePolicyForwarding()
	niPf := niP.GetOrCreatePolicy(vrfPolC)
	niPf.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	for _, pfRule := range pfRuleList {
		pfR := niPf.GetOrCreateRule(seqIDOffset(dut, pfRule.SeqID))
		if pfRule.family == "ipv4" {
			pfRProtoIP := pfR.GetOrCreateIpv4()
			if pfRule.protocol != 0 {
				pfRProtoIP.Protocol = oc.UnionUint8(pfRule.protocol)
			}
			if pfRule.sourceAddr != "" {
				pfRProtoIP.SourceAddress = ygot.String(pfRule.sourceAddr)
			}
		} else if pfRule.family == "ipv6" {
			pfRProtoIP := pfR.GetOrCreateIpv6()
			if pfRule.dscpSet != nil {
				pfRProtoIP.DscpSet = pfRule.dscpSet
			}
		}

		pfRAction := pfR.GetOrCreateAction()
		pfRAction.NetworkInstance = ygot.String(pfRule.ni)
	}
	p1 := dut.Port(t, "port1")
	interfaceID := p1.Name()
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = interfaceID + ".0"
	}
	intf := niP.GetOrCreateInterface(interfaceID)
	intf.ApplyVrfSelectionPolicy = ygot.String(vrfPolC)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	// gnmi.Update(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Name().Config(), "DEFAULT")
	gnmi.Replace(t, dut, dutPolFwdPath.Config(), niP)
}

func configureGribiRoute(t *testing.T, dut *ondatra.DUTDevice, tcArgs *testArgs) {
	t.Helper()
	tcArgs.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithIndex(uint64(1)).WithDecapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithID(uint64(1)).AddNextHop(uint64(1), uint64(1)),

		fluent.IPv4Entry().WithNetworkInstance(niTransitTeVrf).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix("0.0.0.0/0").WithNextHopGroup(uint64(1)))

	if err := awaitTimeout(tcArgs.ctx, t, tcArgs.client, 90*time.Second); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	tcArgs.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithIndex(uint64(2)).WithIPAddress(atePort2.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithID(uint64(2)).AddNextHop(uint64(2), uint64(1)),

		fluent.IPv4Entry().WithNetworkInstance(niTransitTeVrf).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(ipv4OuterDst111+"/32").WithNextHopGroup(uint64(2)))

	if err := awaitTimeout(tcArgs.ctx, t, tcArgs.client, 90*time.Second); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	defaultVRFIPList := []string{"0.0.0.0/0", ipv4OuterDst111 + "/32"}
	for ip := range defaultVRFIPList {
		chk.HasResult(t, tcArgs.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(defaultVRFIPList[ip]).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	tcArgs.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithIndex(uint64(3)).WithIPAddress(atePort2.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithID(uint64(3)).AddNextHop(uint64(3), uint64(1)),

		fluent.IPv4Entry().WithNetworkInstance(niTEVRF111).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(ipv4OuterDst111+"/32").WithNextHopGroup(uint64(3)))

	if err := awaitTimeout(tcArgs.ctx, t, tcArgs.client, 90*time.Second); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, tcArgs.client.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ipv4OuterDst111+"/32").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
	// Encap
	tcArgs.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithIndex(uint64(4)).WithEncapsulateHeader(fluent.IPinIP).WithIPinIP(ipv4OuterSrc111, ipv4OuterDst111).
			WithNextHopNetworkInstance(niTEVRF111),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithID(uint64(4)).AddNextHop(uint64(4), uint64(1)),

		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfA).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(ipv4InnerDst+"/32").WithNextHopGroup(uint64(4)))

	if err := awaitTimeout(tcArgs.ctx, t, tcArgs.client, 90*time.Second); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	tcArgs.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithIndex(uint64(5)).WithIPAddress(atePort3.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithID(uint64(5)).AddNextHop(uint64(5), uint64(1)),

		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfA).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix("0.0.0.0/0").WithNextHopGroup(uint64(5)))

	if err := awaitTimeout(tcArgs.ctx, t, tcArgs.client, 90*time.Second); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	defaultVRFIPList = []string{"0.0.0.0/0", ipv4InnerDst + "/32"}
	for ip := range defaultVRFIPList {
		chk.HasResult(t, tcArgs.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(defaultVRFIPList[ip]).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, t testing.TB, c *fluent.GRIBIClient, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

func createFlow(flowValues *flowArgs, dstMac string) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(flowValues.flowName)
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(512)
	flow.Rate().SetPps(100)
	flow.Duration().Continuous()
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().SetValue(dstMac)
	// Outer IP header
	if flowValues.isIPInIP {
		outerIPHdr := flow.Packet().Add().Ipv4()
		outerIPHdr.Src().SetValue(flowValues.outHdrSrcIP)
		outerIPHdr.Dst().SetValue(flowValues.outHdrDstIP)
		innerIPHdr := flow.Packet().Add().Ipv4()
		innerIPHdr.Src().SetValue(flowValues.InnHdrSrcIP)
		innerIPHdr.Dst().SetValue(flowValues.InnHdrDstIP)
	} else {
		innerIPHdr := flow.Packet().Add().Ipv4()
		innerIPHdr.Src().SetValue(flowValues.InnHdrSrcIP)
		innerIPHdr.Dst().SetValue(flowValues.InnHdrDstIP)
	}
	return flow
}

// testTraffic sends traffic flow for duration seconds and returns the
// number of packets sent out.
func sendTraffic(t *testing.T, args *testArgs, top gosnappi.Config, ate *ondatra.ATEDevice, flow gosnappi.Flow, duration int, port []string) {
	t.Helper()
	top.Flows().Clear()

	args.otgConfig.Captures().Clear()
	args.otgConfig.Captures().Add().SetName("packetCapture").
		SetPortNames(port).
		SetFormat(gosnappi.CaptureFormat.PCAP)

	flow.TxRx().Port().SetTxName("port1").SetRxNames([]string{"port2", "port3"})
	flow.Metrics().SetEnable(true)
	top.Flows().Append(flow)

	ate.OTG().PushConfig(t, top)
	time.Sleep(30 * time.Second)
	ate.OTG().StartProtocols(t)
	time.Sleep(30 * time.Second)

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	args.otg.SetControlState(t, cs)

	ate.OTG().StartTraffic(t)
	time.Sleep(time.Duration(duration) * time.Second)
	ate.OTG().StopTraffic(t)

	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	args.otg.SetControlState(t, cs)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogPortMetrics(t, ate.OTG(), top)
}

// verifyTrafficFlow verify the each flow on ATE
func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flow gosnappi.Flow) bool {
	rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State())
	txPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State())
	lostPkt := txPkts - rxPkts
	if got := (lostPkt * 100 / txPkts); got >= tolerancePct {
		return false
	}
	return true
}

// verifyBgpTelemetry verifies BGP telemetry.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	nbrPath := bgpPath.Neighbor(bgpNbr3.ateIPv4)
	// Get BGP adjacency state.
	t.Logf("Waiting for BGP neighbor to establish...")
	var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
	status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Fatal("No BGP neighbor formed")
	}
	state, _ := status.Val()
	t.Logf("BGP adjacency for %s: %v", bgpNbr3.ateIPv4, state)
	if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
		t.Errorf("BGP peer %s status got %d, want %d", bgpNbr3.ateIPv4, state, want)
	}
}

func captureAndValidatePackets(t *testing.T, args *testArgs, packetVal *packetValidation) {
	bytes := args.otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(packetVal.portName))
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	f.Close()
	handle, err := pcap.OpenOffline(f.Name())
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	if packetVal.validateDecap {
		validateTrafficDecap(t, packetSource)
	}
	if packetVal.validateNoDecap {
		validateTrafficNonDecap(t, packetSource, packetVal.outDstIP[0], packetVal.inHdrIP)
	}
	if packetVal.validateEncap {
		validateTrafficEncap(t, packetSource, packetVal.outDstIP, packetVal.inHdrIP)
	}
	args.otgConfig.Captures().Clear()
	args.otg.PushConfig(t, args.otgConfig)
	time.Sleep(30 * time.Second)
}

func validateTrafficDecap(t *testing.T, packetSource *gopacket.PacketSource) {
	t.Helper()
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
		ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
		if ipInnerLayer != nil {
			t.Errorf("Packets are not decapped, Inner IP header is not removed.")
		}
	}
}

func validateTrafficNonDecap(t *testing.T, packetSource *gopacket.PacketSource, outDstIP, inHdrIP string) {
	t.Helper()
	t.Log("Validate traffic non decap routes")
	var packetCheckCount uint32 = 1
	for packet := range packetSource.Packets() {
		if packetCheckCount >= 5 {
			break
		}
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
		ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
		if ipInnerLayer != nil {
			if ipPacket.DstIP.String() != outDstIP {
				t.Errorf("Negatice test for Decap failed. Traffic sent to route which does not match the decap route are decaped")
			}
			ipInnerPacket, _ := ipInnerLayer.(*layers.IPv4)
			if ipInnerPacket.DstIP.String() != inHdrIP {
				t.Errorf("Negatice test for Decap failed. Traffic sent to route which does not match the decap route are decaped")
			}
			t.Logf("Traffic for non decap routes passed.")
			break
		}
	}
}

func validateTrafficEncap(t *testing.T, packetSource *gopacket.PacketSource, outDstIP []string, innerIP string) {
	t.Helper()
	t.Log("Validate traffic non decap routes")
	var packetCheckCount uint32 = 1
	for packet := range packetSource.Packets() {
		if packetCheckCount >= 5 {
			break
		}
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
		ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
		if ipInnerLayer != nil {
			if len(outDstIP) == 2 {
				if ipPacket.DstIP.String() != outDstIP[0] || ipPacket.DstIP.String() != outDstIP[1] {
					t.Errorf("Packets are not encapsulated as expected")
				}
			} else {
				if ipPacket.DstIP.String() != outDstIP[0] {
					t.Errorf("Packets are not encapsulated as expected")
				}
			}
			t.Logf("Traffic for encap routes passed.")
			break
		}
	}
}
