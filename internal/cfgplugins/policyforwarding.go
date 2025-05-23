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

// Configure GRE decapsulated. Adding deviation when device doesn't support OC
func NewConfigureGRETunnel(t *testing.T, dut *ondatra.DUTDevice, decapIp string, decapGrpName string) {
	if deviations.GreDecapsulationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig := fmt.Sprintf(`
			ip decap-group %s
			 tunnel type gre
			 tunnel decap-ip %s
			`, decapGrpName, strings.Split(decapIp, "/")[0])
			helpers.GnmiCLIConfig(t, dut, cliConfig)

		default:
			t.Errorf("Deviation GreDecapsulationUnsupported is not handled for the dut: %v", dut.Vendor())
		}
	} else {
		d := &oc.Root{}
		ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		ni1.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
		npf := ni1.GetOrCreatePolicyForwarding()
		np := npf.GetOrCreatePolicy("PBR-MAP")
		np.PolicyId = ygot.String("PBR-MAP")
		np.Type = oc.Policy_Type_PBR_POLICY

		npRule := np.GetOrCreateRule(10)
		ip := npRule.GetOrCreateIpv4()
		ip.DestinationAddressPrefixSet = ygot.String(decapIp)
		npAction := npRule.GetOrCreateAction()
		npAction.DecapsulateGre = ygot.Bool(true)

		port := dut.Port(t, "port1")
		ingressPort := port.Name()
		t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)

		intf := npf.GetOrCreateInterface(ingressPort)
		intf.ApplyForwardingPolicy = ygot.String("PBR-MAP")
		intf.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)

		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni1)
	}
}
