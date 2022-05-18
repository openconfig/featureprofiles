package feature

import (
	"testing"

	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"
	"github.com/openconfig/featureprofiles/tools/inputcisco/solver"
	"github.com/openconfig/featureprofiles/tools/inputcisco/testinput"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"

	oc "github.com/openconfig/ondatra/telemetry"
)

// ConfigBGP Configures BGP as per input file
func ConfigBGP(dev *ondatra.DUTDevice, t *testing.T, bgp *proto.Input_BGP, input testinput.TestInput) error {
	model := configBGP(t, bgp, input)
	request := oc.NetworkInstance{}
	request.Name = model.Name
	request.Protocol = map[oc.NetworkInstance_Protocol_Key]*oc.NetworkInstance_Protocol{
		{
			Name:       bgp.Vrf,
			Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		}: model,
	}
	dev.Config().NetworkInstance(bgp.Vrf).Update(t, &request)
	return nil

}

// UnConfigBGP unconfigures BGP as per input file
func UnConfigBGP(dev *ondatra.DUTDevice, t *testing.T, bgp *proto.Input_BGP) error {
	if bgp.Vrf == "" {
		bgp.Vrf = "default"
	}
	dev.Config().NetworkInstance(bgp.Vrf).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Delete(t)
	return nil

}

func configBGP(t *testing.T, bgp *proto.Input_BGP, input testinput.TestInput) *oc.NetworkInstance_Protocol {

	if bgp.Vrf == "" {
		bgp.Vrf = "default"
	}
	model := oc.NetworkInstance_Protocol{
		Name: ygot.String(bgp.Vrf),

		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Bgp: &oc.NetworkInstance_Protocol_Bgp{
			Global: &oc.NetworkInstance_Protocol_Bgp_Global{},
		},
	}
	model.Bgp.Global.As = ygot.Uint32(uint32(bgp.As))
	if bgp.GracefullRestart != nil {
		model.Bgp.Global.GracefulRestart = &oc.NetworkInstance_Protocol_Bgp_Global_GracefulRestart{
			Enabled: ygot.Bool(bgp.GracefullRestart.Enabled),
		}

	}
	model.Bgp.Global.AfiSafi = map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{}
	for _, afi := range bgp.Afisafi {
		afisafi := &oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{}
		afisafi.AddPaths = getAddPathsGlobal(afi.AdditionalPaths)
		afisafi.AfiSafiName = GetAfisafiType(afi.Type)
		model.Bgp.Global.AfiSafi[afisafi.AfiSafiName] = afisafi
	}
	model.Bgp.Neighbor = getBGPneighbor(t, bgp.Neighbors, input)
	return &model

}

func getBGPneighbor(t *testing.T, neigbors []*proto.Input_BGP_Neighbor, input testinput.TestInput) map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor {
	model := map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{}
	for _, neighbor := range neigbors {
		addresses := solver.Solvetag(t, neighbor.Address, input)
		afisafimap := map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{}
		for _, afi := range neighbor.Afisafi {
			afisafi := oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{}
			afisafi.AddPaths = getAddPathsNeighbor(afi.AdditionalPaths)
			afisafi.AfiSafiName = GetAfisafiType(afi.Type)
			afisafimap[afisafi.AfiSafiName] = &afisafi
			afisafi.ApplyPolicy = getNeighborPolicy(afi.Policy)
		}
		for _, address := range addresses {
			neighboroc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{}
			if neighbor.EbgpMultihop != 0 {
				neighboroc.EbgpMultihop = &oc.NetworkInstance_Protocol_Bgp_Neighbor_EbgpMultihop{
					MultihopTtl: ygot.Uint8(uint8(neighbor.EbgpMultihop)),
				}
			}
			if neighbor.PeerAs != 0 {
				neighboroc.PeerAs = ygot.Uint32(uint32(neighbor.PeerAs))
			}

			neighboroc.NeighborAddress = ygot.String(address)
			model[address] = neighboroc

		}
	}

	return model
}
func getAddPathsGlobal(addPaths []proto.Input_BGP_AdditionalPaths) *oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi_AddPaths {
	model := &oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi_AddPaths{}
	for _, addPath := range addPaths {
		switch addPath {
		case proto.Input_BGP_recieve:
			model.Receive = ygot.Bool(true)
		case proto.Input_BGP_send:
			model.Send = ygot.Bool(true)

		}

	}

	return model
}
func getNeighborPolicy(policy *proto.Input_BGP_Policy) *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy {
	model := &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{}
	model.ImportPolicy = policy.Importpolicy
	model.ExportPolicy = policy.Exportpolicy
	return model
}
func getAddPathsNeighbor(addPaths []proto.Input_BGP_AdditionalPaths) *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths {
	model := &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{}
	for _, addPath := range addPaths {
		switch addPath {
		case proto.Input_BGP_recieve:
			model.Receive = ygot.Bool(true)
		case proto.Input_BGP_send:
			model.Send = ygot.Bool(true)

		}

	}

	return model
}
func GetAfisafiType(afisafitype proto.Input_BGP_AfiSafiType) oc.E_BgpTypes_AFI_SAFI_TYPE {
	switch afisafitype {
	case proto.Input_BGP_IPV4_FLOWSPEC:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_FLOWSPEC
	case proto.Input_BGP_IPV4_LABELED_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_LABELED_UNICAST
	case proto.Input_BGP_IPV4_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	case proto.Input_BGP_IPV6_LABELED_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV6_LABELED_UNICAST
	case proto.Input_BGP_IPV6_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	case proto.Input_BGP_L2VPN_EVPN:
		return oc.BgpTypes_AFI_SAFI_TYPE_L2VPN_EVPN
	case proto.Input_BGP_L2VPN_VPLS:
		return oc.BgpTypes_AFI_SAFI_TYPE_L2VPN_VPLS
	case proto.Input_BGP_L3VPN_IPV4_MULTICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_MULTICAST
	case proto.Input_BGP_L3VPN_IPV4_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_UNICAST
	case proto.Input_BGP_L3VPN_IPV6_MULTICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_MULTICAST
	case proto.Input_BGP_L3VPN_IPV6_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_UNICAST
	case proto.Input_BGP_LINKSTATE:
		return oc.BgpTypes_AFI_SAFI_TYPE_LINKSTATE
	case proto.Input_BGP_LINKSTATE_SPF:
		return oc.BgpTypes_AFI_SAFI_TYPE_LINKSTATE_SPF
	case proto.Input_BGP_LINKSTATE_VPN:
		return oc.BgpTypes_AFI_SAFI_TYPE_LINKSTATE_VPN
	case proto.Input_BGP_SRTE_POLICY_IPV4:
		return oc.BgpTypes_AFI_SAFI_TYPE_SRTE_POLICY_IPV4
	case proto.Input_BGP_SRTE_POLICY_IPV6:
		return oc.BgpTypes_AFI_SAFI_TYPE_SRTE_POLICY_IPV6
	case proto.Input_BGP_VPNV4_FLOWSPEC:
		return oc.BgpTypes_AFI_SAFI_TYPE_VPNV4_FLOWSPEC
	default:
		return oc.BgpTypes_AFI_SAFI_TYPE_UNSET
	}
}
