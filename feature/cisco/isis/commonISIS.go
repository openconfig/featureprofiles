package basetest

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/tools/inputcisco/testinput"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	policyTypeIsis = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	dutAreaAddress = "47.0001"
	dutSysId       = "0000.0000.0001"
	isisName       = "B4"
)

func configureBaseconfig(t *testing.T, inyaml testinput.TestInput,
	dut *ondatra.DUTDevice) {
	inyaml.ConfigVrf(dut)
	configIsis(t, dut, []string{"Bundle-Ether120", "Bundle-Ether121"})
}

// Configure network instance
func configureNetworkInstance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
}

func configIsis(t *testing.T, dut *ondatra.DUTDevice, intfNames []string) {
	dev := &oc.Root{}
	configureNetworkInstance(t)
	inst := dev.GetOrCreateNetworkInstance("DEFAULT")
	prot := inst.GetOrCreateProtocol(policyTypeIsis, isisName)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysId)}
	glob.LevelCapability = 2
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	for _, intfName := range intfNames {
		intf := isis.GetOrCreateInterface(intfName)
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intf.Enabled = ygot.Bool(true)
		isisIntfLevelAfi := intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)

		isisIntfLevelAfi6 := intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi6.Enabled = ygot.Bool(true)

	}
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = 2

	isisPath := gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName)
	gnmi.Update(t, dut, isisPath.Config(), prot)
}
