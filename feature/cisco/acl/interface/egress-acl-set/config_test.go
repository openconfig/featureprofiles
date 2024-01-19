package acl_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/acl/setup"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setupAcl(t *testing.T, dut *ondatra.DUTDevice) *oc.Acl {
	bc := new(oc.Acl)
	*bc = setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Interface", "AclSet"})
	bcAclSet := setup.GetAnyValue(bc.AclSet)
	setup.ResetStruct(bcAclSet, []string{"AclEntry"})
	bcAclSetAclEntry := setup.GetAnyValue(bcAclSet.AclEntry)
	setup.ResetStruct(bcAclSetAclEntry, []string{"Actions"})
	bcAclSetAclEntry.Actions.LogAction = oc.E_Acl_LOG_ACTION(0)
	bcInterface := setup.GetAnyValue(bc.Interface)
	setup.ResetStruct(bcInterface, []string{"EgressAclSet"})
	bcInterfaceEgressAclSet := setup.GetAnyValue(bcInterface.EgressAclSet)
	setup.ResetStruct(bcInterfaceEgressAclSet, []string{})
	gnmi.Replace(t, dut, gnmi.OC().Acl().Config(), bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	gnmi.Delete(t, dut, gnmi.OC().Acl().Config())
}
func TestType(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.E_Acl_ACL_TYPE{
		oc.E_Acl_ACL_TYPE(1), //ACL_L2
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/interfaces/interface/egress-acl-sets/egress-acl-set/config/type using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceEgressAclSet := setup.GetAnyValue(baseConfigInterface.EgressAclSet)
			baseConfigInterfaceEgressAclSet.Type = input

			config := gnmi.OC().Acl().Interface(*baseConfigInterface.Id).EgressAclSet(*baseConfigInterfaceEgressAclSet.SetName, baseConfigInterfaceEgressAclSet.Type)
			state := gnmi.OC().Acl().Interface(*baseConfigInterface.Id).EgressAclSet(*baseConfigInterfaceEgressAclSet.SetName, baseConfigInterfaceEgressAclSet.Type)

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigInterfaceEgressAclSet)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if configGot.Type != input {
						t.Errorf("Config /acl/interfaces/interface/egress-acl-sets/egress-acl-set/config/type: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot.Type != input {
						t.Errorf("State /acl/interfaces/interface/egress-acl-sets/egress-acl-set/config/type: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.Type != 0 {
						t.Errorf("Delete /acl/interfaces/interface/egress-acl-sets/egress-acl-set/config/type fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSetName(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"acl1",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/interfaces/interface/egress-acl-sets/egress-acl-set/config/set-name using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceEgressAclSet := setup.GetAnyValue(baseConfigInterface.EgressAclSet)
			*baseConfigInterfaceEgressAclSet.SetName = input

			config := gnmi.OC().Acl().Interface(*baseConfigInterface.Id).EgressAclSet(*baseConfigInterfaceEgressAclSet.SetName, baseConfigInterfaceEgressAclSet.Type)
			state := gnmi.OC().Acl().Interface(*baseConfigInterface.Id).EgressAclSet(*baseConfigInterfaceEgressAclSet.SetName, baseConfigInterfaceEgressAclSet.Type)

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigInterfaceEgressAclSet)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.Get(t, dut, config.Config())
					if *configGot.SetName != input {
						t.Errorf("Config /acl/interfaces/interface/egress-acl-sets/egress-acl-set/config/set-name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if *stateGot.SetName != input {
						t.Errorf("State /acl/interfaces/interface/egress-acl-sets/egress-acl-set/config/set-name: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.SetName != nil {
						t.Errorf("Delete /acl/interfaces/interface/egress-acl-sets/egress-acl-set/config/set-name fail: got %v", qs)
					}
				}
			})
		})
	}
}
