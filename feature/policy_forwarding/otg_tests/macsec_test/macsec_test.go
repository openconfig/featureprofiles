// Package macsec_test tests MPLSoGRE and MPLSoGUE MACsec functionality.
package macsec_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
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
	top          = gosnappi.NewConfig()
	aggID        string
	custPorts    = []string{"port1"}
	corePorts    = []string{"port2"}
	custIntfIPv4 = attrs.Attributes{
		Desc:         "Customer_connect",
		MTU:          1500,
		IPv4:         "169.254.0.11",
		IPv4Len:      29,
		Subinterface: 20,
	}
	coreIntf = attrs.Attributes{
		Desc:    "Core_Interface",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:DB8:1:6::1",
		MTU:     1500,
		IPv4Len: 24,
		IPv6Len: 126,
	}

	agg1 = &otgconfighelpers.Port{
		Name:        "port1",
		AggMAC:      "02:00:01:01:01:07",
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface1},
		MemberPorts: []string{"port1"},
		IsLag:       false,
	}
	agg2 = &otgconfighelpers.Port{
		Name:        "Port-Channel2",
		AggMAC:      "02:00:01:01:01:01",
		MemberPorts: []string{"port2"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface7},
		LagID:       2,
		IsLag:       true,
	}

	interface1 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "172.16.1.1",
		IPv4Gateway: "172.16.1.2",
		Name:        "intf1",
		MAC:         "02:00:01:01:01:08",
		Vlan:        20,
		IPv4Len:     30,
	}
	interface7 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "192.0.2.2",
		IPv6:        "2001:DB8:1:6::2",
		IPv4Gateway: "192.0.2.1",
		IPv6Gateway: "2001:DB8:1:6::1",
		Name:        "Port-Channel2",
		MAC:         "02:00:01:01:01:02",
		IPv4Len:     24,
		IPv6Len:     126,
	}
	// Custom IMIX settings for all flows.
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 30},
		{Size: 128, Weight: 30},
		{Size: 256, Weight: 30},
		{Size: 512, Weight: 10},
	}
	// MPLSOGRE Encap IPv4 interface IPv4 Payload
	FlowIPv4 = &otgconfighelpers.Flow{
		TxNames:           []string{agg1.Interfaces[0].Name + ".IPv4"},
		RxNames:           []string{agg2.Name + ".IPv4"},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          80,
		FlowName:          "GCI traffic IPv4 interface IPv4 Payload",
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: 20},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "203.0.113.1", IPv4Dst: "198.51.100.1", IPv4SrcCount: 100, RawPriority: 0, RawPriorityCount: 100},
	}

	FlowIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{agg2.Name, agg1.Interfaces[0].Name}, Ports: append(agg1.MemberPorts, agg2.MemberPorts...)},
		Flow:      &otgvalidationhelpers.FlowParams{Name: FlowIPv4.FlowName, TolerancePct: 0.5},
	}
)

// ConfigureDut1 configures dut1.
func ConfigureDut1(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	cfgplugins.MacsecConfig(t, dut)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	cfg := fmt.Sprintf(`
interface %s
  no switchport
interface %s.20
  encapsulation dot1q vlan 20
  ip address 172.16.1.2/30
interface %s
  no switchport
  speed 10g
  mac security profile macsec-test
interface %s.20
  encapsulation dot1q vlan 20
  ip address 169.254.0.9/29
ip routing
ip route 0.0.0.0/0 169.254.0.11
`, p1.Name(), p1.Name(), p2.Name(), p2.Name())
	helpers.GnmiCLIConfig(t, dut, cfg)
}

// PF-1.17: MPLSoGRE and MPLSoGUE MACsec
func ConfigureDut(t *testing.T, dut *ondatra.DUTDevice, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	t.Log("Check the config before the hardware tcam application...")
	// Replacing static sleep with a watch on interface status as a proxy for readiness.
	gnmi.Watch(t, dut, gnmi.OC().Interface(dut.Port(t, custPorts[0]).Name()).OperStatus().State(), 5*time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, ok := val.Val()
		return ok && status == oc.Interface_OperStatus_UP
	}).Await(t)
	cfgplugins.ConfigureAnpfHardwareTcam(t, dut)
	t.Log("Waiting for ANPF hardware TCAM profile to be configured...")
	gnmi.Watch(t, dut, gnmi.OC().Interface(dut.Port(t, custPorts[0]).Name()).OperStatus().State(), 5*time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, ok := val.Val()
		return ok && status == oc.Interface_OperStatus_UP
	}).Await(t)
	cfgplugins.MacsecConfig(t, dut)
	for _, portName := range custPorts {
		port := dut.Port(t, portName)
		i := &oc.Interface{Name: ygot.String(port.Name())}
		i.Type = ethernetCsmacd
		configDUTInterface(i, []*attrs.Attributes{&custIntfIPv4}, dut)
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
		configureInterfaceProperties(t, dut, port.Name(), &custIntfIPv4, ocPFParams)
	}
	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, corePorts, []*attrs.Attributes{&coreIntf}, aggID)
	configureStaticRoute(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	EncapMPLSInGRE(t, dut, pf, ni, ocPFParams, ocNHGParams)
	if deviations.MacsecOCUnsupported(dut) {
		for _, port := range custPorts {
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s \n no switchport \n speed 10g \n mac security profile macsec-test \n", dut.Port(t, port).Name()))
			t.Cleanup(func() {
				helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s \n no mac security profile \n", dut.Port(t, port).Name()))
			})
		}
	}
}

// PF-1.17: MPLSoGRE and MPLSoGUE MACsec
func TestMPLSOGREEncapIPv4Macsec(t *testing.T) {
	t.Log("PF-1.14.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	dut1 := ondatra.DUT(t, "dut1")
	ate := ondatra.ATE(t, "ate")

	niName := "default"
	d := &oc.NetworkInstance{Name: ygot.String(niName)}
	d.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(niName).Config(), d)
	gnmi.Update(t, dut1, gnmi.OC().NetworkInstance(niName).Config(), d)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := GetDefaultOcPolicyForwardingParams()
	ocNHGParams := GetDefaultStaticNextHopGroupParams()

	// Pass ocPFParams to ConfigureDut
	ConfigureDut1(t, dut1)

	// Wait for dut1 port2 to be up
	gnmi.Watch(t, dut1, gnmi.OC().Interface(dut1.Port(t, "port2").Name()).OperStatus().State(), 5*time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, ok := val.Val()
		return ok && status == oc.Interface_OperStatus_UP
	}).Await(t)

	ConfigureDut(t, dut, ocPFParams, ocNHGParams)
	ConfigureOTG(t)
	for _, port := range custPorts {
		t.Log("Waiting for MACsec to be negotiated...")
		// Replacing static sleep with a watch on interface status as a proxy.
		gnmi.Watch(t, dut, gnmi.OC().Interface(dut.Port(t, port).Name()).OperStatus().State(), 5*time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			status, ok := val.Val()
			return ok && status == oc.Interface_OperStatus_UP
		}).Await(t)
		checkMacsecState(t, dut, dut.Port(t, port), true)
	}
	checkMacsecState(t, dut1, dut1.Port(t, "port2"), true)

	createflow(t, top, FlowIPv4, true)
	sendTraffic(t, ate)
	if err := FlowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("Validation on flows failed (): %q", err)
	}
}

// GetDefaultStaticNextHopGroupParams provides default parameters for the generator.
// matching the values in the provided JSON example.
func GetDefaultStaticNextHopGroupParams() cfgplugins.StaticNextHopGroupParams {
	return cfgplugins.StaticNextHopGroupParams{

		StaticNHGName: "MPLS_in_GRE_Encap",
		NHIPAddr1:     "nh_ip_addr_1",
		NHIPAddr2:     "nh_ip_addr_2",
		// TODO: b/417988636 - Set the MplsLabel to the correct value.
	}
}

// GetDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func GetDefaultOcPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "default",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   "customer1",
	}
}

func configureInterfaceProperties(t *testing.T, dut *ondatra.DUTDevice, aggID string, a *attrs.Attributes, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)

	if a.IPv4 != "" {
		cfgplugins.InterfacelocalProxyConfig(t, dut, a, aggID)
	}
	cfgplugins.InterfaceQosClassificationConfig(t, dut, a, aggID)
	cfgplugins.InterfacePolicyForwardingConfig(t, dut, a, aggID, pf, ocPFParams)
}

// function should also include the OC config , within these deviations there should be a switch statement is needed
// Modified to accept pf, ni, and ocPFParams
func EncapMPLSInGRE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, ocPFParams cfgplugins.OcPolicyForwardingParams, ocNHGParams cfgplugins.StaticNextHopGroupParams) {
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	// cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.NextHopGroupConfig(t, dut, "v4", ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "v4", pf, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		PushPolicyForwardingConfig(t, dut, ni)
	}
}

// ConfigureOTG configures an OTG topology with a LAG and MACsec.
func ConfigureOTG(t *testing.T) {
	t.Helper()
	top.Captures().Clear()
	ate := ondatra.ATE(t, "ate")

	aggs := []*otgconfighelpers.Port{agg1, agg2}
	for _, agg := range aggs {
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, agg)
	}
}

// OTGPreValidation validates the OTG port status and interface resolution.
func OTGPreValidation(t *testing.T, params *otgvalidationhelpers.OTGValidation, interfaceType string) {
	ate := ondatra.ATE(t, "ate")
	if err := params.ValidatePortIsActive(t, ate); err != nil {
		t.Errorf("ValidatePortIsActive(): %q", err)
	}
	if interfaceType == "IPv4" {
		if err := params.IsIPv4Interfaceresolved(t, ate); err != nil {
			t.Errorf("IsIPv4Interfaceresolved(): %q", err)
		}
	}
	if interfaceType == "IPv6" {
		if err := params.IsIPv6Interfaceresolved(t, ate); err != nil {
			t.Errorf("IsIPv6Interfaceresolved(): %q", err)
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	ate.OTG().StartTraffic(t)
	time.Sleep(120 * time.Second)
	ate.OTG().StopTraffic(t)
}

func createflow(t *testing.T, top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool) {
	if clearFlows {
		top.Flows().Clear()
	}
	params.CreateFlow(top)
	params.AddEthHeader()
	params.AddVLANHeader()
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
	// TODO - to remove this sleep later
	time.Sleep(5 * time.Second)

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
		i.GetOrCreateEthernet()
		i.Description = ygot.String(a.Desc)
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		s1 := i.GetOrCreateSubinterface(0)
		b4 := s1.GetOrCreateIpv4()
		b6 := s1.GetOrCreateIpv6()
		b4.Mtu = ygot.Uint16(a.MTU)
		b6.Mtu = ygot.Uint32(uint32(a.MTU))
		b4.Enabled = ygot.Bool(true)
		if a.Subinterface != 0 {
			s := i.GetOrCreateSubinterface(a.Subinterface)
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(uint16(a.Subinterface))
			s.GetOrCreateIpv4().Mtu = ygot.Uint16(a.MTU)
			s.GetOrCreateIpv6().Mtu = ygot.Uint32(uint32(a.MTU))
			configureInterfaceAddress(dut, s, a)
		} else {
			configureInterfaceAddress(dut, s1, a)
		}
	}
}

func configureInterfaceAddress(dut *ondatra.DUTDevice, s *oc.Interface_Subinterface, a *attrs.Attributes) {
	s4 := s.GetOrCreateIpv4()
	s4.Enabled = ygot.Bool(true)
	if a.IPv4 != "" {
		a4 := s4.GetOrCreateAddress(a.IPv4)
		a4.PrefixLength = ygot.Uint8(a.IPv4Len)
	}
	s6 := s.GetOrCreateIpv6()
	s6.Enabled = ygot.Bool(true)
	if a.IPv6 != "" {
		s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(a.IPv6Len)
	}

	if a.IPv6Sec != "" {
		s6_2 := s.GetOrCreateIpv6()
		s6_2.Enabled = ygot.Bool(true)
		s6_2.GetOrCreateAddress(a.IPv6Sec).PrefixLength = ygot.Uint8(a.IPv6Len)
	}
}

func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: "default",
		Prefix:          "10.99.1.0/24",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString("192.0.2.2"),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
}

func checkMacsecState(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, wantEnabled bool) {
	t.Helper()
	var cmd, want string
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cmd = "show mac security interface " + dp.Name()
		want = "True"
	case ondatra.CISCO:
		cmd = "show macsec interface " + dp.Name()
		want = "Secured"
	default:
		t.Fatalf("Unsupported vendor: %v", dut.Vendor())
	}

	timeout := 2 * time.Minute
	interval := 5 * time.Second
	deadline := time.Now().Add(timeout)

	var macsecOutput string
	var success bool

	for time.Now().Before(deadline) {
		macsecOutput = dut.CLI().Run(t, cmd)
		if wantEnabled == strings.Contains(macsecOutput, want) {
			success = true
			break
		}
		time.Sleep(interval)
	}

	t.Logf("Got MACsec status: %v", macsecOutput)
	if !success {
		t.Errorf("MACsec for port %s enabled status: got %v, want %v (Output: %s)", dp.Name(), !wantEnabled, wantEnabled, macsecOutput)
	}
}

func PushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}
