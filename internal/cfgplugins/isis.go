// Package cfgplugins provides configuration plugins for network protocols, including IS-IS related configuration helpers.
package cfgplugins

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
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
	ISISAuthKey         string
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

	// Configure Level 2 LSP database authentication if ISISAuthKey is provided.
	// This secures IS-IS LSP exchanges between the DUT and peers using MD5 authentication.
	if ISISData.ISISAuthKey != "" {
		lvl2Auth := isis.GetOrCreateLevel(2).GetOrCreateAuthentication()
		lvl2Auth.Enabled = ygot.Bool(true)
		lvl2Auth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
		lvl2Auth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
		lvl2Auth.AuthPassword = ygot.String(ISISData.ISISAuthKey)
		// For Nokia SR Linux (and platforms requiring explicit level auth containers), explicit flags
		// (DisableCsnp, DisableLsp, DisablePsnp = false) must be set to ensure complete LSP authentication.
		if deviations.ISISExplicitLevelAuthenticationConfig(dut) {
			lvl2Auth.DisableCsnp = ygot.Bool(false)
			lvl2Auth.DisableLsp = ygot.Bool(false)
			lvl2Auth.DisablePsnp = ygot.Bool(false)
		}
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
		// Loopback interfaces are configured as passive and excluded from Hello authentication.
		if strings.HasPrefix(strings.ToLower(in), "lo") {
			isisInterface.SetPassive(true)
		} else if ISISData.ISISAuthKey != "" {
			// Configure interface-level MD5 authentication for IS-IS Hello packets on active links.
			if deviations.ISISInterfaceAuthenticationContainerUnsupported(dut) {
				lvl2IntfAuth := isisInterface.GetOrCreateLevel(2).GetOrCreateHelloAuthentication()
				lvl2IntfAuth.Enabled = ygot.Bool(true)
				lvl2IntfAuth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
				lvl2IntfAuth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
				lvl2IntfAuth.AuthPassword = ygot.String(ISISData.ISISAuthKey)
			} else {
				intfAuth := isisInterface.GetOrCreateAuthentication()
				intfAuth.Enabled = ygot.Bool(true)
				intfAuth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
				intfAuth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
				intfAuth.AuthPassword = ygot.String(ISISData.ISISAuthKey)
			}
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
func GenerateDynamicRouteWithISIS(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, localAggregateName string, generateRoute string, generateIPv6Route string) {
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
		dc := gnmi.OC()
		root := &oc.Root{}

		dni := deviations.DefaultNetworkInstance(dut)
		ni := root.GetOrCreateNetworkInstance(dni)

		aggProto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_LOCAL_AGGREGATE, localAggregateName)
		aggProto.SetIdentifier(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_LOCAL_AGGREGATE)
		aggProto.SetName(localAggregateName)

		aggProto.GetOrCreateAggregate(generateRoute).SetPrefix(generateRoute)
		aggProto.GetOrCreateAggregate(generateIPv6Route).SetPrefix(generateIPv6Route)
		aggProto.SetEnabled(true)

		gnmi.BatchReplace(sb, dc.NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_LOCAL_AGGREGATE, localAggregateName).Config(), aggProto)
		sb.Set(t, dut)
	}
}

// ISISInterfaceParams holds IS-IS Level-2 interface parameters.
type ISISInterfaceParams struct {
	InterfaceName string
	SubintfIndex  uint32
	Level2Metric  uint32
}

// ISISProtocolParams holds IS-IS global and per-interface configuration parameters.
type ISISProtocolParams struct {
	InstanceName string
	NetAddress   string
	LevelCap     oc.E_Isis_LevelType
	Interfaces   []ISISInterfaceParams
}

// ISISInterfaceID returns the OpenConfig IS-IS interface-id for the given interface and subinterface index,
// accounting for platform deviations.
func ISISInterfaceID(dut *ondatra.DUTDevice, intfName string, subintfIdx uint32) string {
	if deviations.ExplicitInterfaceInDefaultVRF(dut) || deviations.InterfaceRefInterfaceIDFormat(dut) {
		return fmt.Sprintf("%s.%d", intfName, subintfIdx)
	}
	return intfName
}

// ConfigureISISProtocol configures IS-IS Level-2 on the default network instance with the specified interfaces
// and adds the configuration to the provided gNMI SetBatch.
func ConfigureISISProtocol(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, params ISISProtocolParams) *gnmi.SetBatch {
	t.Helper()
	if len(params.Interfaces) == 0 {
		t.Fatalf("ConfigureISISProtocol: Interfaces slice cannot be empty")
	}
	if sb == nil {
		sb = &gnmi.SetBatch{}
	}
	dni := deviations.DefaultNetworkInstance(dut)
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(dni)
	proto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, params.InstanceName)
	proto.SetIdentifier(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS)
	proto.SetName(params.InstanceName)
	proto.SetEnabled(true)

	isis := proto.GetOrCreateIsis()
	global := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		global.SetInstance(params.InstanceName)
	}
	global.SetNet([]string{params.NetAddress})
	if params.LevelCap != oc.Isis_LevelType_UNSET {
		global.SetLevelCapability(params.LevelCap)
	} else {
		global.SetLevelCapability(oc.Isis_LevelType_LEVEL_2)
	}
	global.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	global.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)

	lvl2 := isis.GetOrCreateLevel(2)
	lvl2.SetLevelNumber(2)
	if deviations.ISISLevelEnabled(dut) {
		lvl2.SetEnabled(true)
	}
	lvl2.SetMetricStyle(oc.Isis_MetricStyle_WIDE_METRIC)

	for _, ifParam := range params.Interfaces {
		isisIntfID := ISISInterfaceID(dut, ifParam.InterfaceName, ifParam.SubintfIndex)
		isisIntf := isis.GetOrCreateInterface(isisIntfID)
		isisIntf.SetInterfaceId(isisIntfID)
		isisIntf.SetCircuitType(oc.Isis_CircuitType_POINT_TO_POINT)
		isisIntf.SetEnabled(true)

		if !deviations.InterfaceRefConfigUnsupported(dut) {
			ref := isisIntf.GetOrCreateInterfaceRef()
			ref.SetInterface(ifParam.InterfaceName)
			ref.SetSubinterface(ifParam.SubintfIndex)
		}

		if !deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
			isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
		}

		ifLvl2 := isisIntf.GetOrCreateLevel(2)
		ifLvl2.SetLevelNumber(2)
		ifLvl2.SetEnabled(true)
		if deviations.ISISInterfaceLevel1DisableRequired(dut) {
			isisIntf.GetOrCreateLevel(1).SetEnabled(false)
		}

		v4Afi := ifLvl2.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		v4Afi.SetMetric(ifParam.Level2Metric)
		v4Afi.SetEnabled(true)
		v6Afi := ifLvl2.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		v6Afi.SetMetric(ifParam.Level2Metric)
		v6Afi.SetEnabled(true)
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			v4Afi.Enabled = nil
			v6Afi.Enabled = nil
		}
	}

	if deviations.ISISSingleTopologyRequired(dut) {
		v6MultiTopology := isis.GetOrCreateGlobal().
			GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).
			GetOrCreateMultiTopology()
		v6MultiTopology.SetAfiName(oc.IsisTypes_AFI_TYPE_IPV4)
		v6MultiTopology.SetSafiName(oc.IsisTypes_SAFI_TYPE_UNICAST)
	}

	protoPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, params.InstanceName)
	gnmi.BatchUpdate(sb, protoPath.Config(), proto)
	return sb
}

// AwaitISISAdjacencyStatus verifies via gNMI WatchAll/Await whether an IS-IS Level-2 adjacency on the
// specified interface is in UP state (when wantUp is true) or torn down/not UP (when wantUp is false).
func AwaitISISAdjacencyStatus(t *testing.T, dut *ondatra.DUTDevice, instanceName, intfName string, subintfIdx uint32, wantUp bool, timeout time.Duration) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	isisIntfID := ISISInterfaceID(dut, intfName, subintfIdx)
	adjStatePath := gnmi.OC().NetworkInstance(dni).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, instanceName).
		Isis().Interface(isisIntfID).Level(2).
		AdjacencyAny().AdjacencyState().State()

	if wantUp {
		_, ok := watchAllAwaitWithRetry(t, dut, adjStatePath, timeout, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			return present && state == oc.Isis_IsisInterfaceAdjState_UP
		})
		if !ok {
			t.Errorf("IS-IS adjacency on interface %s (id=%s) did not reach UP within %v", intfName, isisIntfID, timeout)
		}
		return
	}

	// When wantUp is false, an adjacency may either transition to a non-UP state or be deleted from the list.
	isisIntfPath := gnmi.OC().NetworkInstance(dni).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, instanceName).
		Isis().Interface(isisIntfID).Level(2).State()
	_, ok := watchAwaitWithRetry(t, dut, isisIntfPath, timeout, func(val *ygnmi.Value[*oc.NetworkInstance_Protocol_Isis_Interface_Level]) bool {
		lvl, present := val.Val()
		if !present || lvl == nil {
			return true
		}
		for _, adj := range lvl.Adjacency {
			if adj.GetAdjacencyState() == oc.Isis_IsisInterfaceAdjState_UP {
				return false
			}
		}
		return true
	})
	if !ok {
		t.Errorf("IS-IS adjacency on interface %s (id=%s) remained UP after %v, want DOWN/absent", intfName, isisIntfID, timeout)
	}
}
