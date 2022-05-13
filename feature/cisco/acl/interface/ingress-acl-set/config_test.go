package acl_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/acl/setup"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setupAcl(t *testing.T, dut *ondatra.DUTDevice) *oc.Acl {
	bc := new(oc.Acl)
	*bc = setup.BaseConfig
	setup.ResetStruct(bc, []string{"Interface", "AclSet"})
	bcInterface := setup.GetAnyValue(bc.Interface)
	setup.ResetStruct(bcInterface, []string{"IngressAclSet"})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestType(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.E_Acl_ACL_TYPE{
		oc.E_Acl_ACL_TYPE(1), //ACL_IPV4
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/type using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceIngressAclSet := setup.GetAnyValue(baseConfigInterface.IngressAclSet)
			baseConfigInterfaceIngressAclSet.Type = input

			config := dut.Config().Acl().Interface(*baseConfigInterface.Id).IngressAclSet(*baseConfigInterfaceIngressAclSet.SetName, baseConfigInterfaceIngressAclSet.Type)
			state := dut.Telemetry().Acl().Interface(*baseConfigInterface.Id).IngressAclSet(*baseConfigInterfaceIngressAclSet.SetName, baseConfigInterfaceIngressAclSet.Type)

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceIngressAclSet)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.Type != input {
						t.Errorf("Config /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/type: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.Type != input {
						t.Errorf("State /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/type: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).Type != 0 {
						t.Errorf("Delete /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/type fail: got %v", qs)
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
		"is:a:",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/set-name using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			baseConfigInterfaceIngressAclSet := setup.GetAnyValue(baseConfigInterface.IngressAclSet)
			*baseConfigInterfaceIngressAclSet.SetName = input

			config := dut.Config().Acl().Interface(*baseConfigInterface.Id).IngressAclSet(*baseConfigInterfaceIngressAclSet.SetName, baseConfigInterfaceIngressAclSet.Type)
			state := dut.Telemetry().Acl().Interface(*baseConfigInterface.Id).IngressAclSet(*baseConfigInterfaceIngressAclSet.SetName, baseConfigInterfaceIngressAclSet.Type)

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigInterfaceIngressAclSet)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SetName != input {
						t.Errorf("Config /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/set-name: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SetName != input {
						t.Errorf("State /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/set-name: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SetName != nil {
						t.Errorf("Delete /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/set-name fail: got %v", qs)
					}
				}
			})
		})
	}
}
