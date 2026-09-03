package gribi_bgp_redistribution_test

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfName              = "TEST_VRF"
	dutAS                = 65500
	ateAS                = 65502
	gribiRedPol          = "GRIBI-TO-BGP"
	bgpExportPol         = "BGP-EXPORT-POLICY"
	drainPolicy          = "peer_drain"
	efAggIPv4            = "EF_AGG_IPV4"
	efAllComm            = "EF_ALL"
	efAllCommVal         = "65535:65535"
	noCoreComm           = "NO-CORE"
	noCoreCommVal        = "65534:20420"
	gshutComm            = "GSHUT-COMMUNITY"
	maskLenExact         = "32..32"
	permitAll            = "PERMIT-ALL"
	permitAllStmtName    = "20"
	routePrefix          = "198.51.100.1/32"
	v4RoutePrefix        = "198.51.100.0"
	v4RoutePrefixLen     = uint32(26)
	nhID                 = 1001
	nhgID                = 2001
	ratePPS              = 10000
	pktSize              = 256
	trafficLossTolerance = 5
	fixedCount           = 10000
	trafficDuration      = 15 * time.Second
	drainComm            = "65535:0"
	drainMed             = 100
	drainRepeat          = 5
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		MAC:     "02:00:02:02:02:02",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		Desc:    "ATE to DUT Port 1",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		MAC:     "02:00:04:02:02:02",
		IPv4:    "203.0.113.1",
		IPv4Len: 30,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		Desc:    "ATE to DUT Port 2",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "203.0.113.2",
		IPv4Len: 30,
	}
	routePolicyPfxSet = Prefix{address: v4RoutePrefix, prefix: v4RoutePrefixLen}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func (ip *Prefix) cidr(t *testing.T) string {
	_, net, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.address, ip.prefix))
	if err != nil {
		t.Fatal(err)
	}
	return net.String()
}

type Prefix struct {
	address string
	prefix  uint32
}

type OTGBGPPrefix struct {
	PeerName     string
	Address      string
	PrefixLength uint32
}

// configureDUT configures the DUT with the necessary VRF, interfaces, BGP, and redistribution policies.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	configureHardwareInit(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, vrfName, false)
	cfgplugins.EnableDefaultNetworkInstanceBgp(t, dut, dutAS)
	configureDUTInterface(t, dut, batch, &dutPort1, p1)
	configureDUTInterface(t, dut, batch, &dutPort2, p2)
	// BGP and Redistribution Configuration
	cfgplugins.ConfigureBGPNeighbor(t, dut, nonDefaultNI, dutPort2.IPv4, atePort2.IPv4, dutAS, ateAS, "IPv4", true)

	if deviations.TableConnectionsUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Logf("Currently, gRIBI redistribution is not supported on %v.", dut.Vendor())
		}
	} else {
		// Configure gRIBI to BGP redistribution
		tc := nonDefaultNI.GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV4)
		tc.SetImportPolicy([]string{gribiRedPol})
		tc.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	}
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrfName, nonDefaultNI)
	configureDUTPort(t, dut, batch, &dutPort1, p1, deviations.DefaultNetworkInstance(dut))
	configureDUTPort(t, dut, batch, &dutPort2, p2, vrfName)
	batch.Set(t, dut)
}

// configureDUTInterface configure interfaces on DUT.
func configureDUTInterface(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	i.Description = ygot.String(attrs.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i4.Enabled = ygot.Bool(true)
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	gnmi.BatchUpdate(batch, d.Interface(p.Name()).Config(), i)
}

// configureDUTPort configure DUT ports.
func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, attrs *attrs.Attributes, p *ondatra.Port, niName string) {
	t.Helper()
	d := gnmi.OC()
	cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), niName, 0)
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.BatchUpdate(batch, d.Interface(p.Name()).Config(), i)
}

// configureRoutingPolicies configures all necessary routing policies on the DUT.
func configureRoutingPolicies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	root := &oc.Root{}
	batch := &gnmi.SetBatch{}
	rp := root.GetOrCreateRoutingPolicy()
	// ----------------------------
	// PERMIT ALL POLICY
	// ----------------------------
	pdef := rp.GetOrCreatePolicyDefinition(permitAll)
	stmt, err := pdef.AppendNewStatement(permitAllStmtName)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", permitAllStmtName, err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	// ----------------------------
	// PREFIX SET
	// ----------------------------
	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(efAggIPv4)
	prefixSet.SetMode(oc.PrefixSet_Mode_IPV4)
	prefixSet.GetOrCreatePrefix(routePolicyPfxSet.cidr(t), maskLenExact)

	// ----------------------------
	// COMMUNITY SETS
	// ----------------------------
	bgpDefinedSets := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets()

	csAll := bgpDefinedSets.GetOrCreateCommunitySet(efAllComm)
	csAll.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(efAllCommVal)})

	csCore := bgpDefinedSets.GetOrCreateCommunitySet(noCoreComm)
	csCore.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(noCoreCommVal)})

	// ----------------------------
	// gRIBI REDISTRIBUTION POLICY
	// ----------------------------
	pdGRIBI := rp.GetOrCreatePolicyDefinition(gribiRedPol)
	stmtGRIBI, err := pdGRIBI.AppendNewStatement("REDISTRIBUTE_GRIBI_IPV4")
	if err != nil {
		t.Fatalf("AppendNewStatement failed: %v", err)
	}

	stmtGRIBI.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(efAggIPv4)
	stmtGRIBI.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	bgpActions := stmtGRIBI.GetOrCreateActions().GetOrCreateBgpActions()
	setComm := bgpActions.GetOrCreateSetCommunity()
	setComm.SetMethod(oc.SetCommunity_Method_REFERENCE)
	setComm.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
	setComm.GetOrCreateReference().SetCommunitySetRefs([]string{efAllComm, noCoreComm})

	// ----------------------------
	// EXPORT POLICY
	// ----------------------------
	pdExport := rp.GetOrCreatePolicyDefinition(bgpExportPol)

	stmtExport, err := pdExport.AppendNewStatement("EXPORT_POLICY_IPV4")
	if err != nil {
		t.Fatalf("AppendNewStatement failed: %v", err)
	}
	stmtExport.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(efAggIPv4)
	if !deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmtExport.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(efAllComm)
	}
	stmtExport.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	// Push routing policy config
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)

	// ----------------------------
	// ATTACH EXPORT POLICY TO BGP
	// ----------------------------
	policy := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp().GetOrCreateNeighbor(atePort2.IPv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	policy.SetExportPolicy([]string{bgpExportPol})
	policy.SetDefaultExportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(atePort2.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()

	gnmi.BatchReplace(batch, path.Config(), policy)

	// Commit
	batch.Set(t, dut)
}

// configureDrainPolicy configures the drain policy and associated community-set.
func configureDrainPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()

	// -------------------------------------------------------
	// GSHUT COMMUNITY SET
	// -------------------------------------------------------
	csGshut := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(gshutComm)
	csGshut.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(drainComm)})

	// -------------------------------------------------------
	// DRAIN POLICY
	// -------------------------------------------------------
	pdDrain := rp.GetOrCreatePolicyDefinition(drainPolicy)

	stmtDrain, err := pdDrain.AppendNewStatement("DRAIN-ACTIONS")
	if err != nil {
		t.Fatalf("AppendNewStatement(DRAIN-ACTIONS) failed: %v", err)
	}

	bgpActions := stmtDrain.GetOrCreateActions().GetOrCreateBgpActions()

	// -----------------------------
	// SET MED
	// -----------------------------
	if !deviations.BGPSetMedActionUnsupported(dut) {
		bgpActions.SetSetMedAction(oc.BgpPolicy_BgpSetMedAction_SET)
	}

	bgpActions.SetSetMed(oc.UnionUint32(drainMed))

	// -----------------------------
	// AS PATH PREPEND
	// -----------------------------
	asp := bgpActions.GetOrCreateSetAsPathPrepend()
	asp.SetRepeatN(drainRepeat)
	asp.SetAsn(dutAS)

	// -----------------------------
	// ADD GSHUT COMMUNITY
	// -----------------------------
	setComm := bgpActions.GetOrCreateSetCommunity()
	setComm.SetMethod(oc.SetCommunity_Method_REFERENCE)
	setComm.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
	setComm.GetOrCreateReference().SetCommunitySetRefs([]string{gshutComm})

	stmtDrain.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)

	// Push config
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// appendDrainPolicyToExport appends the drain policy to the BGP export policy list applied to the neighbor connected to ATE port2.
func appendDrainPolicyToExport(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	nbrPath := gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(atePort2.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	exportPolicies := []string{drainPolicy, bgpExportPol}
	gnmi.Update(t, dut, nbrPath.ExportPolicy().Config(), exportPolicies)
}

// removeDrainPolicyFromExport removes the drain policy from the BGP export chain.
func removeDrainPolicyFromExport(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ni := gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbr := ni.Neighbor(atePort2.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	exportPolicies := []string{bgpExportPol}
	gnmi.Update(t, dut, nbr.ExportPolicy().Config(), exportPolicies)
}

// configureATE configures the ATE ports and BGP neighbor.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	topo := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	atePort1.AddToOTG(topo, p1, &dutPort1)
	d2 := atePort2.AddToOTG(topo, p2, &dutPort2)

	// Configure eBGP on OTG port2.
	ipv42 := d2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dev2BGP := d2.Bgp().SetRouterId(atePort2.IPv4)
	bgp4Peer2 := dev2BGP.Ipv4Interfaces().Add().SetIpv4Name(ipv42.Name()).Peers().Add().SetName(d2.Name() + ".BGP4.peer")
	bgp4Peer2.SetPeerAddress(dutPort2.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	bgp4Peer2.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
	netv42 := bgp4Peer2.V4Routes().Add().SetName("v4-bgpNet-dev1")
	netv42.Addresses().Add().SetAddress(v4RoutePrefix).SetPrefix(v4RoutePrefixLen)

	return topo
}

// mustNewGRIBIClient creates a gRIBI client, starts the session, and fails the test if the client cannot be created or started.
func mustNewGRIBIClient(t *testing.T, dut *ondatra.DUTDevice) *gribi.Client {
	t.Helper()

	c := &gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	if err := c.Start(t); err != nil {
		t.Fatalf("gRIBI connection failed: %v", err)
	}

	c.BecomeLeader(t)
	return c
}

// buildTestVRFEntries creates the gRIBI entries required for the test.
// It programs the following objects:
//
// 1. NextHop in the DEFAULT network instance pointing to the ATE.
// 2. NextHopGroup in the DEFAULT network instance referencing the NextHop.
// 3. IPv4 route in TEST_VRF referencing the NHG located in DEFAULT.
func buildTestVRFEntries(dut *ondatra.DUTDevice) ([]fluent.GRIBIEntry, []*client.OpResult) {
	entries := []fluent.GRIBIEntry{
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).WithIndex(nhID).WithIPAddress(atePort1.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).WithID(nhgID).AddNextHop(nhID, 1),
		fluent.IPv4Entry().WithNetworkInstance(vrfName).WithPrefix(routePrefix).WithNextHopGroup(nhgID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	}

	results := []*client.OpResult{
		fluent.OperationResult().WithNextHopOperation(nhID).WithOperationType(constants.Add).WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		fluent.OperationResult().WithNextHopGroupOperation(nhgID).WithOperationType(constants.Add).WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		fluent.OperationResult().WithIPv4Operation(routePrefix).WithOperationType(constants.Add).WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
	}

	return entries, results
}

// createFlow creates a traffic flow for MPLS-in-UDP testing.
func createFlow(t *testing.T, cfg gosnappi.Config, ate *ondatra.ATEDevice) gosnappi.Flow {
	t.Helper()
	cfg.Flows().Clear().Items()
	flow := cfg.Flows().Add().SetName("non_default")
	flow.Metrics().SetEnable(true)
	flow.Rate().SetPps(ratePPS)
	flow.Size().SetFixed(pktSize)
	flow.Duration().FixedPackets().SetPackets(fixedCount)
	flow.TxRx().Port().SetTxName("port2").SetRxNames([]string{"port1"})

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort2.MAC)
	eth.Dst().SetValue(dutPort2.MAC)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort2.IPv4)
	v4.Dst().SetValue(strings.Split(routePrefix, "/")[0])
	ate.OTG().PushConfig(t, cfg)
	ate.OTG().StartProtocols(t)
	return flow
}

// validateTrafficFlows validates OTG traffic behavior for the provided flows.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, cfg gosnappi.Config, flow gosnappi.Flow) error {
	t.Helper()
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), cfg)

	outPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
	inPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))

	if outPkts == 0 {
		return fmt.Errorf("flow %s: OutPkts=0, traffic not generated", flow.Name())
	}

	lossPct := ((outPkts - inPkts) * 100) / outPkts

	t.Logf("Flow %s: Out=%v In=%v Loss=%v", flow.Name(), outPkts, inPkts, lossPct)

	if lossPct > trafficLossTolerance {
		return fmt.Errorf("flow %s: loss %v > allowed %d", flow.Name(), lossPct, trafficLossTolerance)
	}

	return nil
}

// checkOTGBGP4Prefix verifies whether a specific IPv4 prefix is learned by an OTG BGP peer within a given timeout.
func checkOTGBGP4Prefix(t *testing.T, otg *otg.OTG, config gosnappi.Config, expectedOTGBGPPrefix OTGBGPPrefix) bool {
	t.Helper()
	_, ok := gnmi.WatchAll(t, otg, gnmi.OTG().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv4PrefixAny().State(), 2*time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			if prefix, present := v.Val(); present {
				return prefix.GetAddress() == expectedOTGBGPPrefix.Address &&
					prefix.GetPrefixLength() == expectedOTGBGPPrefix.PrefixLength
			}
			return false
		}).Await(t)
	return ok
}

// verifyDUTBGPEstablished verifies on dut for BGP peer establishment.
func verifyDUTBGPEstablished(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	sp := gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().NeighborAny().SessionState().State()
	watch := gnmi.WatchAll(t, dut, sp, 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if !ok || state != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
			return false
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("BGP sessions not established: got %v", val)
	}
	t.Log("DUT BGP sessions established")
}

// VerifyOTGBGPEstablished verifies on OTG for BGP peer establishment.
func verifyOTGBGPEstablished(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	sp := gnmi.OTG().BgpPeerAny().SessionState().State()
	watch := gnmi.WatchAll(t, ate.OTG(), sp, 2*time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
		state, ok := val.Val()
		if !ok || state != otgtelemetry.BgpPeer_SessionState_ESTABLISHED {
			return false
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("BGP sessions not established: got %v", val)
	}
	t.Log("OTG BGP sessions established")
}

// configureHardwareInit sets up the initial hardware configuration on the DUT. It pushes hardware initialization configs for VRF Selection Extended feature and Policy Forwarding feature.
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	features := []cfgplugins.FeatureType{
		cfgplugins.FeatureVrfSelectionExtended,
		cfgplugins.FeaturePolicyForwarding,
	}
	for _, feature := range features {
		hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, feature)
		if hardwareInitCfg != "" {
			cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
		}
	}
}

// validateRoutingPolicyV4 verifies the correctness of the IPv4 BGP export routing policy applied on the DUT and its effects observed on the ATE.
func validateRoutingPolicyV4(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) error {
	t.Helper()
	bgpPrefixes := gnmi.GetAll[*otgtelemetry.BgpPeer_UnicastIpv4Prefix](t, ate.OTG(), gnmi.OTG().BgpPeer("atePort2.BGP4.peer").UnicastIpv4PrefixAny().State())
	found := false
	var errMsg string

	for _, bgpPrefix := range bgpPrefixes {
		t.Logf("Received prefix %s/%d", bgpPrefix.GetAddress(), bgpPrefix.GetPrefixLength())

		if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == v4RoutePrefix && bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == v4RoutePrefixLen {

			found = true

			// ----------------------------
			// MED VERIFICATION
			// ----------------------------
			if bgpPrefix.GetMultiExitDiscriminator() != drainMed {
				msg := fmt.Sprintf("prefix %s MED mismatch: got %d want %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), drainMed)
				errMsg += msg + "\n"
			} else {
				t.Logf("MED verified: %d", bgpPrefix.GetMultiExitDiscriminator())
			}

			// ----------------------------
			// AS PATH VERIFICATION
			// ----------------------------
			total := 0
			for _, seg := range bgpPrefix.AsPath {
				for _, asn := range seg.AsNumbers {
					if asn == dutAS {
						total++
					}
				}
			}

			if total-1 != drainRepeat {
				msg := fmt.Sprintf("as prepend mismatch: got %d want %d", total-1, drainRepeat)
				errMsg += msg + "\n"
			} else {
				t.Logf("AS prepend verified: %d", drainRepeat)
			}
			break
		}
	}

	if !found {
		msg := fmt.Sprintf("prefix %s/%d not received on OTG", v4RoutePrefix, v4RoutePrefixLen)
		errMsg += msg
	}

	if errMsg != "" {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// disableBGPSession disables the BGP session between DUT and ATE Port2 by setting the neighbor "enabled" field to false using OpenConfig via gNMI.
func disableBGPSession(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	path := gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(atePort2.IPv4).Enabled()
	gnmi.Replace(t, dut, path.Config(), false)
	verifyBGPSessionDown(t, dut)
}

// verifyBGPSessionDown verifies that the BGP session between the DUT and the specified neighbor is in the IDLE state, indicating that the session is administratively disabled or not established.
func verifyBGPSessionDown(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(atePort2.IPv4).SessionState()

	state := gnmi.Get(t, dut, statePath.State())
	if state != oc.Bgp_Neighbor_SessionState_IDLE {
		t.Fatalf("BGP session not down, got state: %v", state)
	}

	t.Logf("BGP session successfully disabled, state: %v", state)
}

// enableBGPSession enables the BGP session between DUT and ATE Port2 by setting the neighbor "enabled" field to true using OpenConfig via gNMI.
func enableBGPSession(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	path := gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(atePort2.IPv4).Enabled()
	gnmi.Replace(t, dut, path.Config(), true)
	verifyBGPSessionUp(t, dut)
}

// verifyBGPSessionUp verifies that the BGP session between the DUT and the specified neighbor is successfully established.
func verifyBGPSessionUp(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	statePath := gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(atePort2.IPv4).SessionState()

	// Wait until BGP session becomes ESTABLISHED
	state, ok := gnmi.Await(t, dut, statePath.State(), 60*time.Second, oc.Bgp_Neighbor_SessionState_ESTABLISHED).Val()
	if !ok {
		t.Fatalf("BGP session did not reach ESTABLISHED state")
	}

	t.Logf("BGP session is UP. State: %v", state)
}

// TestGRIBIBGPRedistribution is the main test function.
func TestGRIBIBGPRedistribution(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	configureDUT(t, dut)
	topo := configureATE(t, ate)
	otg.PushConfig(t, topo)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")

	configureRoutingPolicies(t, dut)

	// TestID-16.4.1 : gRIBI to BGP Redistribution
	t.Run("TestID-16.4.1 : gRIBI to BGP Redistribution", func(t *testing.T) {
		c := mustNewGRIBIClient(t, dut)
		t.Cleanup(func() {
			c.FlushAll(t)
			c.Close(t)
		})
		entries, addResults := buildTestVRFEntries(dut)
		t.Log("Programming gRIBI route in TEST_VRF")
		c.AddEntries(t, entries, addResults)
		if err := c.AwaitTimeout(ctx, t, time.Minute); err != nil {
			t.Fatalf("Entries not programmed: %v", err)
		}
		flow := createFlow(t, topo, ate)
		verifyDUTBGPEstablished(t, dut)
		verifyOTGBGPEstablished(t, ate)
		parts := strings.Split(routePrefix, "/")
		prefixLen, _ := strconv.Atoi(parts[1])
		expectedOTGBGPPrefix := OTGBGPPrefix{PeerName: "atePort2.BGP4.peer", Address: parts[0], PrefixLength: uint32(prefixLen)}
		// TODO: Prefix validation should fail because gRIBI redistribution is not supported.
		if !checkOTGBGP4Prefix(t, otg, topo, expectedOTGBGPPrefix) {
			t.Errorf("prefix %v is not being learned", expectedOTGBGPPrefix.Address)
		}
		if err := validateTrafficFlows(t, ate, topo, flow); err != nil {
			t.Errorf("traffic validation failed: %v", err)
		}
		c.FlushAll(t)
		t.Logf("Verifying traffic fails after entries deleted.")
		if err := validateTrafficFlows(t, ate, topo, flow); err == nil {
			t.Error("Traffic validation succeeded unexpectedly, expected failure.")
		} else {
			t.Logf("Traffic validation failed as expected: %v", err)
		}
	})
	// TestID-16.4.2 - Drain Policy Validation
	t.Run("TestID-16.4.2 : DrainPolicyValidation", func(t *testing.T) {
		c := mustNewGRIBIClient(t, dut)
		t.Cleanup(func() {
			c.FlushAll(t)
			c.Close(t)
		})
		entries, addResults := buildTestVRFEntries(dut)
		t.Log("Program gRIBI route in TEST_VRF")
		c.AddEntries(t, entries, addResults)
		if err := c.AwaitTimeout(ctx, t, time.Minute); err != nil {
			t.Fatalf("Entries not programmed: %v", err)
		}
		verifyDUTBGPEstablished(t, dut)
		verifyOTGBGPEstablished(t, ate)
		t.Log("Configure drain policy")
		configureDrainPolicy(t, dut)

		t.Log("Append drain policy to export policy")
		appendDrainPolicyToExport(t, dut)
		// time.Sleep(20 * time.Second)
		t.Log("Verify updated attributes (MED, AS prepend, GSHUT)")
		// TODO: Attributes validation should fail because gRIBI redistribution is not supported.
		err := validateRoutingPolicyV4(t, dut, ate)
		if err != nil {
			t.Errorf("failed to validate route policy : %v", err)
		}
		t.Log("Remove drain policy")
		removeDrainPolicyFromExport(t, dut)
		t.Log("Delete gRIBI route")
		c.FlushAll(t)
		rerr := validateRoutingPolicyV4(t, dut, ate)
		if rerr == nil {
			t.Error("Route policy validation succeeded unexpectedly, expected failure.")
		} else {
			t.Logf("Route policy validation failure is expected: %v", rerr)
		}
	})
	// TestID-16.4.3 - Disable BGP session with drain policy
	t.Run("TestID-16.4.3 : Disable BGP session with drain policy", func(t *testing.T) {
		c := mustNewGRIBIClient(t, dut)
		t.Cleanup(func() {
			c.FlushAll(t)
			c.Close(t)
		})
		entries, addResults := buildTestVRFEntries(dut)
		t.Log("Program gRIBI route in TEST_VRF")
		c.AddEntries(t, entries, addResults)
		if err := c.AwaitTimeout(ctx, t, time.Minute); err != nil {
			t.Fatalf("Entries not programmed: %v", err)
		}
		verifyDUTBGPEstablished(t, dut)
		verifyOTGBGPEstablished(t, ate)
		t.Log("Configure drain policy")
		configureDrainPolicy(t, dut)

		t.Log("Append drain policy to export policy")
		appendDrainPolicyToExport(t, dut)
		t.Log("Verify updated attributes (MED, AS prepend, GSHUT)")
		// TODO: Attributes validation should fail because gRIBI redistribution is not supported.
		err := validateRoutingPolicyV4(t, dut, ate)
		if err != nil {
			t.Errorf("failed to validate route policy : %v", err)
		}
		disableBGPSession(t, dut)
		enableBGPSession(t, dut)
		t.Log("Verify updated attributes (MED, AS prepend, GSHUT) with BGP disable/enable")
		// TODO: Attributes validation should fail because gRIBI redistribution is not supported.
		verr := validateRoutingPolicyV4(t, dut, ate)
		if verr != nil {
			t.Errorf("failed to validate route policy : %v", verr)
		}
		t.Log("Remove drain policy")
		removeDrainPolicyFromExport(t, dut)
		t.Log("Delete gRIBI route")
		c.FlushAll(t)
		rerr := validateRoutingPolicyV4(t, dut, ate)
		if rerr == nil {
			t.Error("Route policy validation succeeded unexpectedly, expected failure.")
		} else {
			t.Logf("Route policy validation failure is expected: %v", rerr)
		}
	})
}
