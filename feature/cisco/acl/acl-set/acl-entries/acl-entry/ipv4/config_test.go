
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
	setup.ResetStruct(bcAclSetAclEntry, []string{"Actions", "Ipv4"})
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
		39, 
		19, 
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
func TestSourceAddress(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string {
		"186.175.128.250/20", 
		"3.152.2.2/4", 
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
func TestDestinationAddress(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string {
		"253.253.22.82/30", 
		"62.33.2.6/3", 
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
func TestDscpSet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := [][]uint8 {
		[]uint8 {
			36, 
			9, 
			29, 
			16, 
			8, 
		},
		[]uint8 {
			14, 
			46, 
			17, 
			6, 
			36, 
			45, 
			47, 
			27, 
			9, 
			5, 
			13, 
			49, 
			56, 
			61, 
			6, 
			27, 
			51, 
			44, 
			31, 
			1, 
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
func TestHopLimit(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8 {
		35, 
		91, 
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
func TestProtocol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Ipv4_Protocol_Union {
		oc.UnionUint8(252), 
		oc.UnionUint8(163), 
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
