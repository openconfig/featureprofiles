// Package cfgplugins provides configuration plugins for network protocols, including IS-IS related configuration helpers.
package cfgplugins

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// ISISConfigBasic holds all parameters needed for configuring ISIS on the DUT.
type ISISConfigBasic struct {
	InstanceName string
	AreaAddress  string
	SystemID     string
	AggID        string
	Ports        []*ondatra.Port
	LoopbackIntf string
}

// ISISGlobalParams is the data structure for the DUT data.
type ISISGlobalParams struct {
	DUTArea             string
	DUTSysID            string
	NetworkInstanceName string
	ISISInterfaceNames  []string
	NetworkInstanceType *oc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE
}

// NewISIS configures the DUT with ISIS protocol.
func NewISIS(t *testing.T, dut *ondatra.DUTDevice, ISISData *ISISGlobalParams, b *gnmi.SetBatch) *oc.Root {
	t.Helper()
	rootPath := &oc.Root{}
	// Create network instance "Default"
	networkInstance, err := rootPath.NewNetworkInstance(ISISData.NetworkInstanceName)
	if err != nil {
		t.Errorf("Error creating NewNetworkInstance for %s", ISISData.NetworkInstanceName)
	}
	if ISISData.NetworkInstanceType != nil {
		networkInstance.Type = *ISISData.NetworkInstanceType
	} else {
		networkInstance.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	}

	protocol := networkInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISData.NetworkInstanceName)
	protocol.Enabled = ygot.Bool(true)
	isis := protocol.GetOrCreateIsis()

	if deviations.ISISInstanceEnabledRequired(dut) {
		isis.GetOrCreateGlobal().SetInstance(ISISData.NetworkInstanceName)
	}
	// Set global configuration parameters
	isis.GetOrCreateGlobal().Net = []string{fmt.Sprintf("%v.%v.00", ISISData.DUTArea, ISISData.DUTSysID)}
	isis.GetGlobal().SetLevelCapability(oc.Isis_LevelType_LEVEL_2)
	isis.GetGlobal().SetHelloPadding(oc.Isis_HelloPaddingType_DISABLE)
	isis.GetGlobal().GetOrCreateTimers().SetLspRefreshInterval(65218)
	isis.GetGlobal().GetOrCreateTimers().SetLspLifetimeInterval(65535)
	isis.GetGlobal().GetOrCreateTimers().GetOrCreateSpf().SetSpfFirstInterval(200)
	isis.GetGlobal().GetOrCreateTimers().GetOrCreateSpf().SetSpfHoldInterval(2000)
	if deviations.ISISLevelEnabled(dut) {
		isis.GetOrCreateLevel(2).SetEnabled(true)
	}
	isis.GetOrCreateLevel(2).SetMetricStyle(oc.Isis_MetricStyle_WIDE_METRIC)
	if !deviations.IsisMplsUnsupported(dut) {
		isis.GetGlobal().GetOrCreateMpls().GetOrCreateIgpLdpSync().SetEnabled(false)
	}

	// Set IPV4 configuration parameters
	isisV4Afi := isis.GetGlobal().GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisV4Afi.SetEnabled(true)

	// Set IPV6 configuration parameters
	isisV6Afi := isis.GetGlobal().GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisV6Afi.Enabled = ygot.Bool(true)

	for _, in := range ISISData.ISISInterfaceNames {
		if (deviations.ExplicitInterfaceInDefaultVRF(dut) || deviations.InterfaceRefInterfaceIDFormat(dut)) && !strings.Contains(in, ".") {
			in += ".0"
		}
		fmt.Println("Adding ISIS interface: ", in)
		isisInterface := isis.GetOrCreateInterface(in)
		isisInterface.SetEnabled(true)
		isisInterface.SetCircuitType(oc.Isis_CircuitType_POINT_TO_POINT)
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			isisInterface.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
			isisInterface.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		}
		isisInterface.GetOrCreateLevel(2).SetEnabled(true)
		isisInterface.GetOrCreateLevel(1).SetEnabled(false)
		isisInterface.GetOrCreateLevel(2).GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetMetric(10)
		isisInterface.GetOrCreateLevel(2).GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetMetric(10)
		isisInterface.GetOrCreateLevel(2).GetOrCreateTimers().SetHelloMultiplier(6)
		isisInterface.GetOrCreateTimers().SetLspPacingInterval(50)
		if !deviations.IsisMplsUnsupported(dut) {
			isisInterface.GetOrCreateMpls().GetOrCreateIgpLdpSync().SetEnabled(false)
		}
		if in[0:2] == "lo" {
			isisInterface.SetPassive(true)
		}
	}

	gnmi.BatchUpdate(b, gnmi.OC().NetworkInstance(ISISData.NetworkInstanceName).Config(), rootPath.GetNetworkInstance(ISISData.NetworkInstanceName))
	handleSingleTopologyDeviation(t, dut, b)
	return rootPath
}

// NewISISBasic configures ISIS on the DUT using OpenConfig. It enables ISIS globally, sets AFs, and applies interface-level config.
func NewISISBasic(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, cfg ISISConfigBasic) *oc.NetworkInstance_Protocol {
	t.Helper()

	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	// Set Protocol Config
	protocol := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, cfg.InstanceName)
	protocol.SetEnabled(true)

	isis := protocol.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()

	if deviations.ISISInstanceEnabledRequired(dut) {
		// must match the protocol 'name'
		globalISIS.SetInstance(cfg.InstanceName)
	}
	globalISIS.Net = []string{
		fmt.Sprintf("%v.%v.00", cfg.AreaAddress, cfg.SystemID),
	}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2

	if cfg.AggID != "" {
		// Enable ISIS on specified interfaces
		names := []string{cfg.AggID}
		for _, p := range cfg.Ports {
			names = append(names, p.Name())
		}

		t.Logf("Enable ISIS on interfaces: %v, plus %s", names, cfg.AggID)
		for _, intf := range names {
			isisIf := isis.GetOrCreateInterface(intf)
			isisIf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
			isisIf.SetEnabled(true)
		}
	}

	if cfg.LoopbackIntf != "" {
		// Loopback passive
		isisLo := isis.GetOrCreateInterface(cfg.LoopbackIntf)
		isisLo.SetEnabled(true)
		isisLo.SetPassive(true)
	}

	for _, port := range cfg.Ports {
		isisIf := isis.GetOrCreateInterface(port.Name())
		isisIf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIf.SetEnabled(true)
	}

	// === Add protocol subtree into the batch ===
	gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, cfg.InstanceName).Config(), protocol)

	return protocol
}

// handleSingleTopologyDeviation handles the single topology deviation for ISIS by
// setting the v6 multi-topology to have the same AFISAFI as v4.
func handleSingleTopologyDeviation(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch) {
	t.Helper()
	if !deviations.ISISSingleTopologyRequired(dut) {
		return
	}
	switch dut.Vendor() {
	case ondatra.CISCO:
		root := &oc.Root{}
		protocol := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, deviations.DefaultNetworkInstance(dut))
		v6MultiTopology := protocol.GetOrCreateIsis().GetOrCreateGlobal().
			GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).
			GetOrCreateMultiTopology()
		v6MultiTopology.SetAfiName(oc.IsisTypes_AFI_TYPE_IPV4)
		v6MultiTopology.SetSafiName(oc.IsisTypes_SAFI_TYPE_UNICAST)
		gnmi.BatchUpdate(sb, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, deviations.DefaultNetworkInstance(dut)).Config(), protocol)
	default:
		t.Fatalf("Single ISIS topology deviation not supported for vendor: %s", dut.Vendor())
	}
}

// GenerateDynamicRouteWithISIS configures the DUT to generate dynamic routes using ISIS as the trigger protocol.
func GenerateDynamicRouteWithISIS(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var cliConfig strings.Builder

		cliConfig.WriteString(`
    configure terminal
    router general
    control-functions

    { "cmd": "code unit ipv4_generate_default_conditionally", "input": "function ipv4_generate_route_conditionally()\n{\nif source_protocol is ISIS and prefix match prefix_list_v4 TRIGGER_ROUTE {\nreturn true;\n}\n}\nEOF"}
    { "cmd": "code unit ipv6_generate_route_conditionally", "input": "function ipv6_generate_route_conditionally()\n{\nif source_protocol is ISIS and prefix match prefix_list_v6 TRIGGER_ROUTE_IPV6 {\nreturn true;\n}\n}\nEOF"}
    compile
    commit
    dynamic prefix-list ipv4_generate_route
    match vrf default source-protocol any rcf ipv4_generate_route_conditionally()
    prefix-list ipv4 GENERATED_ROUTE
    !
    dynamic prefix-list ipv6_generate_route
    match vrf default source-protocol any rcf ipv6_generate_route_conditionally()
    prefix-list ipv6 GENERATED_ROUTE_IPV6
    !
    router general
    vrf default
      routes dynamic prefix-list ipv4_generate_route install drop
      routes dynamic prefix-list ipv6_generate_route install drop
    !
    router isis DEFAULT
    redistribute dynamic
    !
    router bgp 1
    redistribute dynamic
    !`)
		helpers.GnmiCLIConfig(t, dut, cliConfig.String())
	default:
		t.Fatalf("Generate dynamic route with ISIS not supported for vendor: %s", dut.Vendor())
	}
}
