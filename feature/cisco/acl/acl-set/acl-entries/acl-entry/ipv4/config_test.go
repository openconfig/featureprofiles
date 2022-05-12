
package acl_test
import (
	"testing"
	"fmt"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/featureprofiles/internal/fptest"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/featureprofiles/feature/cisco/acl/setup"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}


func setupAcl(t *testing.T, dut *ondatra.DUTDevice) *oc.Acl {
	bc := new(oc.Acl)
	*bc = setup.BaseConfig
	setup.ResetStruct(bc, []string{"AclSet"})
	bcAclSet := setup.GetAnyValue(bc.AclSet)
	setup.ResetStruct(bcAclSet, []string{"AclEntry"})
	bcAclSetAclEntry := setup.GetAnyValue(bcAclSet.AclEntry)
	setup.ResetStruct(bcAclSetAclEntry, []string{"Ipv4", "Actions"})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestDscp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8 {
		35, 
		35, 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			*baseConfigAclSetAclEntryIpv4.Dscp = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Dscp != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Dscp != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).Dscp != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp fail: got %v", qs)
				}
			})
		})
	}
}
func TestDscpSet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := [][]uint8 {
		[]uint8 {
			38, 
			8, 
			38, 
			29, 
			45, 
			53, 
			5, 
			45, 
			16, 
			7, 
			32, 
			46, 
			18, 
			31, 
			7, 
			13, 
			7, 
			38, 
			47, 
			39, 
			11, 
			37, 
			18, 
			6, 
			23, 
			52, 
			34, 
			2, 
			45, 
			2, 
		},
		[]uint8 {
			57, 
			51, 
			42, 
			33, 
			50, 
			31, 
			52, 
			57, 
			55, 
			42, 
			5, 
			14, 
			43, 
			53, 
			5, 
			57, 
			45, 
			5, 
			44, 
			40, 
			28, 
			5, 
			16, 
			10, 
			38, 
			59, 
			58, 
			10, 
			14, 
			26, 
			22, 
			28, 
			51, 
			35, 
			5, 
			60, 
			15, 
			20, 
			33, 
			5, 
			7, 
			28, 
			45, 
			63, 
			3, 
			28, 
			21, 
			23, 
			60, 
			46, 
		},
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp-set using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			baseConfigAclSetAclEntryIpv4.DscpSet = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					for i, cg := range configGot.DscpSet {
						if cg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp-set: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					for i, sg := range stateGot.DscpSet {
						if sg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).DscpSet != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/dscp-set fail: got %v", qs)
				}
			})
		})
	}
}
func TestDestinationAddress(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string {
		"253.69.4.97/21", 
		"185.9.93.8/4", 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			*baseConfigAclSetAclEntryIpv4.DestinationAddress = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DestinationAddress != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DestinationAddress != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).DestinationAddress != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address fail: got %v", qs)
				}
			})
		})
	}
}
func TestSourceAddress(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string {
		"6.24.9.227/30", 
		"126.245.4.19/31", 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			*baseConfigAclSetAclEntryIpv4.SourceAddress = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SourceAddress != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SourceAddress != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).SourceAddress != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address fail: got %v", qs)
				}
			})
		})
	}
}
func TestProtocol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Ipv4_Protocol_Union {
		oc.UnionUint8(145), 
		oc.UnionUint8(117), 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			baseConfigAclSetAclEntryIpv4.Protocol = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.Protocol != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.Protocol != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).Protocol != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol fail: got %v", qs)
				}
			})
		})
	}
}
func TestHopLimit(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8 {
		244, 
		160, 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/hop-limit using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv4 := baseConfigAclSetAclEntry.Ipv4
			*baseConfigAclSetAclEntryIpv4.HopLimit = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Ipv4()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv4)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.HopLimit != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/hop-limit: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.HopLimit != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/hop-limit: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).HopLimit != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/hop-limit fail: got %v", qs)
				}
			})
		})
	}
}
