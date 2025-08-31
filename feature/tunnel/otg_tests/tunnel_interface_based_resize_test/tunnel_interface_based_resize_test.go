package tunnel_interface_based_resize_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
)

var (
	top             = gosnappi.NewConfig()
	aggID           string
	egressLag1Ports = []string{"port2", "port3"}
	egressLag2Ports = []string{"port4", "port5"}
	staticRoute     = "10.99.20.0"
	ingressIntf     = attrs.Attributes{
		Desc:    "ingress_interface",
		IPv4:    "194.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:10:1:6::1",
		IPv6Len: 126,
	}
	egressLag1Intf = attrs.Attributes{
		Desc:    "Egress_Lag1_Intf",
		IPv4:    "169.254.0.1",
		IPv6:    "2000:0:0:2::1",
		MTU:     9202,
		IPv4Len: 30,
		IPv6Len: 126,
	}
	egressLag2Intf = attrs.Attributes{
		Desc:    "Egress_Lag2_Intf",
		IPv4:    "169.254.0.5",
		IPv6:    "2000:0:0:2::5",
		MTU:     9202,
		IPv4Len: 30,
		IPv6Len: 126,
	}

	egressIntf = attrs.Attributes{
		Desc:    "Egress_Intf",
		IPv4:    "169.254.0.9",
		IPv6:    "2000:0:0:2::9",
		MTU:     9202,
		IPv4Len: 30,
		IPv6Len: 126,
	}

	otgingress = &otgconfighelpers.Port{
		Name:       "port1",
		Interfaces: []*otgconfighelpers.InterfaceProperties{interface1},
	}

	otgagg1 = &otgconfighelpers.Port{
		Name:        "Port-Channel1",
		AggMAC:      "02:00:01:01:01:07",
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface2},
		MemberPorts: []string{"port2", "port3"},
		LagID:       1,
		IsLag:       true,
	}
	otgagg2 = &otgconfighelpers.Port{
		Name:        "Port-Channel2",
		AggMAC:      "02:00:01:01:01:01",
		MemberPorts: []string{"port4", "port5"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface3},
		LagID:       2,
		IsLag:       true,
	}

	otgegress = &otgconfighelpers.Port{
		Name:       "port6",
		Interfaces: []*otgconfighelpers.InterfaceProperties{interface4},
	}

	interface1 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "194.0.2.2",
		IPv6:        "2001:10:1:6::2",
		IPv4Gateway: "194.0.2.1",
		IPv6Gateway: "2001:10:1:6::1",
		Name:        "Ingress-Port",
		MAC:         "02:00:01:01:01:02",
		IPv4Len:     30,
		IPv6Len:     126,
	}

	interface2 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.2",
		IPv4Gateway: "169.254.0.1",
		Name:        "Port-Channel1",
		MAC:         "02:00:01:01:01:08",
		IPv6:        "2000:0:0:1::2",
		IPv6Gateway: "2000:0:0:1::1",
		IPv4Len:     30,
		IPv6Len:     126,
	}

	interface3 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.6",
		IPv4Gateway: "169.254.0.5",
		Name:        "Port-Channel2",
		MAC:         "02:00:01:01:01:09",
		IPv6:        "2000:0:0:1::6",
		IPv6Gateway: "2000:0:0:1::5",
		IPv4Len:     30,
		IPv6Len:     126,
	}
	interface4 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.10",
		IPv4Gateway: "169.254.0.9",
		Name:        "Egress-Port",
		MAC:         "02:00:01:01:01:10",
		IPv6:        "2000:0:0:1::a",
		IPv6Gateway: "2000:0:0:1::9",
		IPv4Len:     30,
		IPv6Len:     126,
	}

	flowResolveArp = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{otgingress.Interfaces[0].Name}},
	}
	// IPv4 Payload
	flowIPv4 = &otgconfighelpers.Flow{
		TxNames:   []string{interface1.Name + ".IPv4"},
		RxNames:   []string{interface2.Name + ".IPv4", interface3.Name + ".IPv4", interface4.Name + ".IPv4"},
		FrameSize: 512,
		Flowrate:  40,
		FlowName:  "traffic IPv4 interface IPv4 Payload",
		EthFlow:   &otgconfighelpers.EthFlowParams{SrcMAC: interface1.MAC},
		IPv4Flow:  &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 2000},
	}

	flowIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{otgingress.Name, otgegress.Name}},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowIPv4.FlowName, TolerancePct: 0.5},
	}
	flowIPv6 = &otgconfighelpers.Flow{
		TxNames:   []string{interface1.Name + ".IPv6"},
		RxNames:   []string{interface2.Name + ".IPv6", interface3.Name + ".IPv6", interface4.Name + ".IPv6"},
		FrameSize: 512,
		Flowrate:  40,
		FlowName:  "traffic IPv6 interface IPv6 Payload",
		EthFlow:   &otgconfighelpers.EthFlowParams{SrcMAC: interface1.MAC},
		IPv6Flow:  &otgconfighelpers.IPv6FlowParams{IPv6Src: "3000:1::1", IPv6Dst: "2000:1::1", IPv6SrcCount: 2000, IPv6SrcStep: "1::1"},
	}

	flowIPv6Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{otgingress.Name, otgegress.Name}},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowIPv6.FlowName, TolerancePct: 0.5},
	}
	ecmpValidation = &otgvalidationhelpers.OTGECMPValidation{
		PortWeightages: []otgvalidationhelpers.PortWeightage{
			{PortName: otgagg1.MemberPorts[0], Weightage: 16.0},
			{PortName: otgagg1.MemberPorts[1], Weightage: 16.0},
			{PortName: otgagg2.MemberPorts[0], Weightage: 16.0},
			{PortName: otgagg2.MemberPorts[1], Weightage: 16.0},
			{PortName: otgegress.Name, Weightage: 32.0},
		},

		Flows:        []string{flowIPv4.FlowName, flowIPv6.FlowName},
		TolerancePct: 0.2,
	}
)

func configureOTG(t *testing.T) {
	t.Helper()
	top.Captures().Clear()
	ate := ondatra.ATE(t, "ate")

	// Create a slice of aggPortData for easier iteration
	ifaces := []*otgconfighelpers.Port{otgingress, otgagg1, otgagg2, otgegress}

	// Configure OTG Interfaces
	for _, iface := range ifaces {
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, iface)
	}
	ate.OTG().PushConfig(t, top)
}

// Modified to create and pass OC root, ni, pf
func configureDut(t *testing.T, dut *ondatra.DUTDevice, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	config := &oc.Root{}

	dp1 := dut.Port(t, "port1")
	ingressp := ingressIntf.ConfigOCInterface(config.GetOrCreateInterface(dp1.Name()), dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(ingressp.GetName()).Config(), ingressp)

	dp6 := dut.Port(t, "port6")
	egressp := egressIntf.ConfigOCInterface(config.GetOrCreateInterface(dp6.Name()), dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(egressp.GetName()).Config(), egressp)

	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, egressLag1Ports, []*attrs.Attributes{&egressLag1Intf}, aggID)
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, egressLag2Ports, []*attrs.Attributes{&egressLag2Intf}, aggID)
	configureStaticRoutes(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	encapInGRE(t, dut, pf, ni, ocPFParams, ocNHGParams)

}

func TestSetup(t *testing.T) {
	t.Log("TUN:1.6.1-Setup Tunnels as per requirements")
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := fetchDefaultOcPolicyForwardingParams(t, dut)
	ocNHGParams := fetchDefaultStaticNextHopGroupParams()

	// Pass ocPFParams to ConfigureDut
	configureDut(t, dut, ocPFParams, ocNHGParams)
	configureOTG(t)
}

func TestTunnelBaselineStats(t *testing.T) {
	t.Log("TUN:1.6.2-Gather baseline stats by passing traffic")
	ate := ondatra.ATE(t, "ate")
	createflow(t, top, flowIPv4, true)
	createflow(t, top, flowIPv6, false)
	sendTraffic(t, ate)
	time.Sleep(20 * time.Second)
	ate.OTG().StopTraffic(t)
	if err := flowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := flowIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := ecmpValidation.ValidateECMP(t, ate); err != nil {
		t.Errorf("ECMPValidationFailed(): got err: %q, want nil", err)
	}

}

func TestTunnelInterfaceBasedResize(t *testing.T) {
	t.Log("TUN:1.6.3-Removal of static routes to prevent using some of the tunnels")
	ate := ondatra.ATE(t, "ate")
	dut := ondatra.DUT(t, "dut")
	sendTraffic(t, ate)
	t.Log("Reduce number of Tunnels by deleting static routes")
	deleteStaticRoutes(t, dut)

	// Verify the entry for static route is not active through AFT Telemetry.
	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(fmt.Sprintf("%s/24", staticRoute))
	if got, ok := gnmi.Watch(t, dut, ipv4Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		if !present && ipv4Entry == nil {
			return true
		}
		return false
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %v, want %s", got, staticRoute)
	} else {
		t.Logf("Prefix %s not installed in DUT as static Route", staticRoute)
	}

	time.Sleep(20 * time.Second)
	ate.OTG().StopTraffic(t)
	if err := flowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := flowIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := ecmpValidation.ValidateECMP(t, ate); err != nil {
		t.Errorf("ECMPValidationFailed(): got err: %q, want nil", err)
	}
}

func TestTunnelInterfaceBasedResizeRestoreStaticRoutes(t *testing.T) {
	t.Log("TUN:1.6.4-Restore static routes to start using all 32 tunnels")
	ate := ondatra.ATE(t, "ate")
	dut := ondatra.DUT(t, "dut")
	sendTraffic(t, ate)
	t.Log("Restore static routes to start using all 32 tunnels")
	configureStaticRoutes(t, dut)

	// Verify the entry for static Route is active through AFT Telemetry.
	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(fmt.Sprintf("%s/24", staticRoute))
	if got, ok := gnmi.Watch(t, dut, ipv4Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		t.Log(ipv4Entry.GetPrefix())
		return present && ipv4Entry.GetPrefix() == fmt.Sprintf("%s/24", staticRoute)
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %v, want %s", got, staticRoute)
	} else {
		t.Logf("Prefix %s installed in DUT as static...", staticRoute)
	}

	time.Sleep(20 * time.Second)
	ate.OTG().StopTraffic(t)
	if err := flowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := flowIPv6Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
	}
	if err := ecmpValidation.ValidateECMP(t, ate); err != nil {
		t.Errorf("ECMPValidationFailed(): got err: %q, want nil", err)
	}
}

// fetchDefaultStaticNextHopGroupParams provides default parameters for the generator.
// matching the values in the provided JSON example.
func fetchDefaultStaticNextHopGroupParams() cfgplugins.StaticNextHopGroupParams {

	nhipaddrs := []string{}

	for i := 1; i <= 32; i++ {
		nhipaddrs = append(nhipaddrs, fmt.Sprintf("10.99.%d.1", i))
	}

	return cfgplugins.StaticNextHopGroupParams{
		NHIPAddrs:     nhipaddrs,
		StaticNHGName: "gre_encap",
	}
}

// fetchDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func fetchDefaultOcPolicyForwardingParams(t *testing.T, dut *ondatra.DUTDevice) cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         dut.Port(t, "port1").Name(),
		AppliedPolicyName:   "gre_encap",
	}
}

// function should also include the OC config , within these deviations there should be a switch statement is needed
// Modified to accept pf, ni, and ocPFParams
func encapInGRE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.NextHopGroupConfig(t, dut, "dualstack", ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "dualstack", pf, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		pushPolicyForwardingConfig(t, dut, ni)
	}
}

func configureInterfaces(t *testing.T, dut *ondatra.DUTDevice, dutPorts []string, subinterfaces []*attrs.Attributes, aggID string) {
	t.Helper()
	d := gnmi.OC()
	dutAggPorts := []*ondatra.Port{}
	for _, port := range dutPorts {
		dutAggPorts = append(dutAggPorts, dut.Port(t, port))
	}
	if deviations.AggregateAtomicUpdate(dut) {
		cfgplugins.DeleteAggregate(t, dut, aggID, dutAggPorts)
		cfgplugins.SetupAggregateAtomically(t, dut, aggID, dutAggPorts)
	}

	lacp := &oc.Lacp_Interface{Name: ygot.String(aggID)}
	lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	lacpPath := d.Lacp().Interface(aggID)
	fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
	gnmi.Replace(t, dut, lacpPath.Config(), lacp)

	agg := &oc.Interface{Name: ygot.String(aggID)}
	configDUTInterface(agg, subinterfaces, dut)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = ieee8023adLag
	aggPath := d.Interface(aggID)
	fptest.LogQuery(t, aggID, aggPath.Config(), agg)
	gnmi.Replace(t, dut, aggPath.Config(), agg)

	for _, port := range dutAggPorts {
		holdTimeConfig := &oc.Interface_HoldTime{
			Up:   ygot.Uint32(3000),
			Down: ygot.Uint32(150),
		}
		intfPath := gnmi.OC().Interface(port.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)
	}
}

func configDUTInterface(i *oc.Interface, subinterfaces []*attrs.Attributes, dut *ondatra.DUTDevice) {
	for _, a := range subinterfaces {
		i.Description = ygot.String(a.Desc)
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		s1 := i.GetOrCreateSubinterface(0)
		b4 := s1.GetOrCreateIpv4()
		b6 := s1.GetOrCreateIpv6()
		b4.Mtu = ygot.Uint16(a.MTU)
		b6.Mtu = ygot.Uint32(uint32(a.MTU))
		if deviations.InterfaceEnabled(dut) {
			b4.Enabled = ygot.Bool(true)
		}
		if a.Subinterface != 0 {
			s := i.GetOrCreateSubinterface(a.Subinterface)
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(uint16(a.Subinterface))
			configureInterfaceAddress(dut, s, a)
		} else {
			configureInterfaceAddress(dut, s1, a)
		}
	}
}

func configureInterfaceAddress(dut *ondatra.DUTDevice, s *oc.Interface_Subinterface, a *attrs.Attributes) {
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	if a.IPv4 != "" {
		a4 := s4.GetOrCreateAddress(a.IPv4)
		a4.PrefixLength = ygot.Uint8(a.IPv4Len)
	}
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	if a.IPv6 != "" {
		s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(a.IPv6Len)
	}

	if a.IPv6Sec != "" {
		s62 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s62.Enabled = ygot.Bool(true)
		}
		s62.GetOrCreateAddress(a.IPv6Sec).PrefixLength = ygot.Uint8(a.IPv6Len)
	}
}

func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	b := &gnmi.SetBatch{}
	for i := 1; i <= 32; i++ {
		sV4 := &cfgplugins.StaticRouteCfg{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          fmt.Sprintf("10.99.%d.0/24", i),
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(interface2.IPv4),
				"1": oc.UnionString(interface3.IPv4),
				"2": oc.UnionString(interface4.IPv4),
			},
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
			t.Fatalf("Failed to configure IPv4 static route: %v", err)
		}
	}
	b.Set(t, dut)
}

func deleteStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	b := &gnmi.SetBatch{}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	for i := 17; i <= 32; i++ {
		gnmi.BatchDelete(b, sp.Static(fmt.Sprintf("10.99.%d.0/24", i)).Config())
	}
	b.Set(t, dut)
}

func pushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}

func createflow(t *testing.T, top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool) {
	if clearFlows {
		top.Flows().Clear()
	}
	params.CreateFlow(top)
	params.AddEthHeader()
	if params.IPv4Flow != nil {
		params.AddIPv4Header()
	}
	if params.IPv6Flow != nil {
		params.AddIPv6Header()
	}
	if params.TCPFlow != nil {
		params.AddTCPHeader()
	}
	if params.UDPFlow != nil {
		params.AddUDPHeader()
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	flowResolveArp.IsIPv4Interfaceresolved(t, ate)
	ate.OTG().StartTraffic(t)
}
