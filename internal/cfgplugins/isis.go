// Package cfgplugins provides configuration plugins for network protocols, including IS-IS related configuration helpers.
package cfgplugins

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
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
		if deviations.ExplicitInterfaceInDefaultVRF(dut) && !strings.Contains(in, ".") {
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
	return rootPath
}

// NewISISBasic configures ISIS on the DUT using OpenConfig. It enables ISIS globally, sets AFs, and applies interface-level config.
func NewISISBasic(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, cfg ISISConfigBasic) *oc.NetworkInstance_Protocol {
	t.Helper()

	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	// Set Protocol Config
	protocol := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, cfg.InstanceName)
	protocol.Enabled = ygot.Bool(true)

	isis := protocol.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()

	if deviations.ISISInstanceEnabledRequired(dut) {
		// must match the protocol 'name'
		globalISIS.Instance = ygot.String(cfg.InstanceName)
	}
	globalISIS.Net = []string{
		fmt.Sprintf("%v.%v.00", cfg.AreaAddress, cfg.SystemID),
	}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2

	// Enable ISIS on specified interfaces
	names := []string{cfg.AggID}
	for _, p := range cfg.Ports {
		names = append(names, p.Name())
	}

	t.Logf("Enable ISIS on interfaces: %v, plus %s", names, cfg.AggID)
	for _, intf := range names {
		isisIf := isis.GetOrCreateInterface(intf)
		isisIf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIf.Enabled = ygot.Bool(true)
	}

	// Loopback passive
	isisLo := isis.GetOrCreateInterface(cfg.LoopbackIntf)
	isisLo.Enabled = ygot.Bool(true)
	isisLo.Passive = ygot.Bool(true)

	// === Add protocol subtree into the batch ===
	gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, cfg.InstanceName).Config(), protocol)

	return protocol
}
