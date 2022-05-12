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
	setup.ResetStruct(bc, []string{"AclSet", "Interface"})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestId(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		":cc",
		":",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/interfaces/interface/config/id using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			*baseConfigInterface.Id = input

			config := dut.Config().Acl().Interface(*baseConfigInterface.Id)
			state := dut.Telemetry().Acl().Interface(*baseConfigInterface.Id)

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigInterface)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Id != input {
						t.Errorf("Config /acl/interfaces/interface/config/id: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Id != input {
						t.Errorf("State /acl/interfaces/interface/config/id: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).Id != nil {
					t.Errorf("Delete /acl/interfaces/interface/config/id fail: got %v", qs)
				}
			})
		})
	}
}
