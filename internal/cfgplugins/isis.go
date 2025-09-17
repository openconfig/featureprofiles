package cfgplugins

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// ISISGlobalParams is the data structure for the DUT data.
type ISISGlobalParams struct {
	DUTArea                string
	DUTSysID               string
	DefaultNetworkInstance string
	ISISInterfaceNames     []string
}

// NewISIS configures the DUT with ISIS protocol.
func NewISIS(t *testing.T, dut *ondatra.DUTDevice, ISISData *ISISGlobalParams) *oc.Root {
	t.Helper()
	rootPath := &oc.Root{}
	// Create network instance "Default"
	networkInstance, _ := rootPath.NewNetworkInstance(ISISData.DefaultNetworkInstance)
	protocol := networkInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISData.DefaultNetworkInstance)
	protocol.Enabled = ygot.Bool(true)
	isis := protocol.GetOrCreateIsis()

	if deviations.ISISInstanceEnabledRequired(dut) {
		isis.GetOrCreateGlobal().SetInstance(ISISData.DefaultNetworkInstance)
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
	return rootPath
}
