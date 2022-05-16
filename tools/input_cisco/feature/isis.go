package feature

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/tools/input_cisco/proto"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"

	oc "github.com/openconfig/ondatra/telemetry"
)

func ConfigISIS(dev *ondatra.DUTDevice, t *testing.T, isis *proto.Input_ISIS) error {
	if isis.Vrf == "" {
		isis.Vrf = "default"
	}
	model := configISIS(isis)
	request := &oc.NetworkInstance{
		// Name: &isis.Vrf,
	}
	request.Protocol = map[oc.NetworkInstance_Protocol_Key]*oc.NetworkInstance_Protocol{
		{
			Name:       isis.Vrf,
			Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
		}: model,
	}
	dev.Config().NetworkInstance(isis.Vrf).Update(t, request)
	return nil

}

func UnConfigISIS(dev *ondatra.DUTDevice, t *testing.T, isis *proto.Input_ISIS) error {
	if isis.Vrf == "" {
		isis.Vrf = "default"
	}
	dev.Config().NetworkInstance(isis.Vrf).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isis.Vrf).Isis().Delete(t)
	return nil

}
func configISIS(isis *proto.Input_ISIS) *oc.NetworkInstance_Protocol {
	model := oc.NetworkInstance_Protocol{
		Name:       ygot.String(isis.Name),
		Isis:       &oc.NetworkInstance_Protocol_Isis{},
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
	}
	model.Isis.Global = &oc.NetworkInstance_Protocol_Isis_Global{}
	if isis.Level != 0 {
		model.Isis.Global.LevelCapability = getIsisLevelType(isis.Level)
	}
	if len(isis.Afisafi) > 0 {
		model.Isis.Global.Afi = getIsisglobalAfi(isis)
		model.Isis.Global.Af = getIsisglobalAf(isis)
	}
	return &model

}

func getIsisglobalAf(isis *proto.Input_ISIS) map[oc.NetworkInstance_Protocol_Isis_Global_Af_Key]*oc.NetworkInstance_Protocol_Isis_Global_Af {
	Af := map[oc.NetworkInstance_Protocol_Isis_Global_Af_Key]*oc.NetworkInstance_Protocol_Isis_Global_Af{}
	for _, afi := range isis.Afisafi {
		afiname, safiname := getIsisAfiSafiname(afi.Type)
		afimodel := &oc.NetworkInstance_Protocol_Isis_Global_Af{
			AfiName:  afiname,
			SafiName: safiname,
		}
		if afi.Metric != 0 {
			afimodel.Metric = ygot.Uint32(uint32(afi.Metric))
		}
		Af[oc.NetworkInstance_Protocol_Isis_Global_Af_Key{
			AfiName:  afiname,
			SafiName: safiname,
		}] = afimodel

	}
	return Af

}
func getIsisglobalAfi(isis *proto.Input_ISIS) map[oc.E_IsisTypes_AFI_TYPE]*oc.NetworkInstance_Protocol_Isis_Global_Afi {
	Afi := map[oc.E_IsisTypes_AFI_TYPE]*oc.NetworkInstance_Protocol_Isis_Global_Afi{}
	for _, afi := range isis.Afisafi {
		//To Do: add usecase
		fmt.Println(afi)

	}
	return Afi

}

func getIsisAfiSafiname(afisafitype proto.Input_ISIS_AfiSafiType) (oc.E_IsisTypes_AFI_TYPE, oc.E_IsisTypes_SAFI_TYPE) {

	switch afisafitype {
	case proto.Input_ISIS_IPV4_UNICAST:
		return oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST
	case proto.Input_ISIS_IPV4_MULTICAST:
		return oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_MULTICAST
	case proto.Input_ISIS_IPV6_UNICAST:
		return oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST
	case proto.Input_ISIS_IPV6_MULTICAST:
		return oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_MULTICAST
	default:
		return oc.IsisTypes_AFI_TYPE_UNSET, oc.IsisTypes_SAFI_TYPE_UNSET

	}
}
func getIsisLevelType(afisafitype proto.Input_ISIS_Level) oc.E_IsisTypes_LevelType {
	switch afisafitype {
	case proto.Input_ISIS_level_1:
		return oc.IsisTypes_LevelType_LEVEL_1
	case proto.Input_ISIS_level_2:
		return oc.IsisTypes_LevelType_LEVEL_2
	case proto.Input_ISIS_level_1_2:
		return oc.IsisTypes_LevelType_LEVEL_1_2
	default:
		return oc.IsisTypes_LevelType_UNSET

	}
}
