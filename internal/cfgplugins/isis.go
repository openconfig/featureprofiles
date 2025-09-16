// Package cfgplugins provides configuration plugins for network protocols, including IS-IS related configuration helpers.
package cfgplugins

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// ISISConfig holds all parameters needed for configuring ISIS on the DUT.
type ISISConfig struct {
	InstanceName string
	AreaAddress  string
	SystemID     string
	AggID        string
	Ports        []*ondatra.Port
	LoopbackIntf string
}

// ConfigureISISOnDUT configures ISIS on the DUT using OpenConfig.
// It enables ISIS globally, sets AFs, and applies interface-level config.
func ConfigureISISOnDUT(t *testing.T, dut *ondatra.DUTDevice, cfg ISISConfig) *oc.NetworkInstance_Protocol {
	t.Helper()
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	// Set Protocol Config (no GetOrCreateConfig so use this)
	protocol := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, cfg.InstanceName)
	protocol.Enabled = ygot.Bool(true)

	isis := protocol.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(cfg.InstanceName) // must match the protocol 'name'
	}
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", cfg.AreaAddress, cfg.SystemID)}
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

	return protocol
}
