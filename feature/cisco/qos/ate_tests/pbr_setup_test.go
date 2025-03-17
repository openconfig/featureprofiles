package qos_test

import (
	"testing"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	pbrName = "PBR"
)

type PBROptions struct {
	// BackupNHG specifies the backup next-hop-group to be used when all next-hops are unavailable.
	SrcIP string
}

func getPolicyForwardingInterfaceConfig(policyName, intf string) *oc.NetworkInstance_PolicyForwarding_Interface {

	d := &oc.Root{}
	pfCfg := d.GetOrCreateNetworkInstance("DEFAULT").GetOrCreatePolicyForwarding().GetOrCreateInterface(intf + ".0")
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(policyName)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(intf)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	return pfCfg
}

// *ciscoFlags.DefaultNetworkInstance
// ConfigureDefaultNetworkInstance configures the default network instance name and type.
func ConfigureDefaultNetworkInstance(t testing.TB, d *ondatra.DUTDevice) {
	defNiPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	gnmi.Update(t, d, defNiPath.Config(), &oc.NetworkInstance{
		Name: ygot.String(*ciscoFlags.DefaultNetworkInstance),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
	})
}

// configbasePBR, creates class map, policy and configures under source interface
func configbasePBR(t *testing.T, dut *ondatra.DUTDevice, networkInstance, iptype string, index uint32, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8, opts ...*PBROptions) {
	// pfpath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding()

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if iptype == "ipv4" {
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.SrcIP != "" {
					r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
						SourceAddress: &opt.SrcIP,
						Protocol:      protocol,
					}
				}
			}
		} else {
			r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
				Protocol: protocol,
			}
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if iptype == "ipv6" {
		r.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv6.DscpSet = dscpset
		}
	}
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r)
	InterfaceName := "Bundle-Ether120"
	intfRef := &oc.NetworkInstance_PolicyForwarding_Interface_InterfaceRef{}
	intfRef.SetInterface(InterfaceName)
	intfRef.SetSubinterface(0)
	InterfaceRef := InterfaceName + ".0"
	pf.Interface = map[string]*oc.NetworkInstance_PolicyForwarding_Interface{
		InterfaceRef: {
			InterfaceId:             ygot.String(InterfaceRef),
			ApplyVrfSelectionPolicy: ygot.String(pbrName),
			InterfaceRef:            intfRef,
		},
	}
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Config(), &pf)

	//configure PBR on ingress port
	ports := []string{"Bundle-Ether120", "Bundle-Ether122", "Bundle-Ether123"}
	for _, portInterface := range ports {
		pfPath := gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Interface(portInterface + ".0")
		pfCfg := getPolicyForwardingInterfaceConfig(pbrName, portInterface)
		gnmi.Update(t, dut, pfPath.Config(), pfCfg)
	}
}

// unconfigbasePBR, creates class map, policy and configures under source interface
func unconfigbasePBR(t *testing.T, dut *ondatra.DUTDevice) {
	pfpath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding()
	ports := []string{"Bundle-Ether120", "Bundle-Ether122", "Bundle-Ether123"}
	for _, portInterface := range ports {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Interface(portInterface+".0").Config())
	}
	gnmi.Delete(t, dut, pfpath.Policy(pbrName).Config())
	gnmi.Delete(t, dut, pfpath.Config())
}
