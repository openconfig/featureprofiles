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
	setup.ResetStruct(bc, []string{"AclSet"})
	bcAclSet := setup.GetAnyValue(bc.AclSet)
	setup.ResetStruct(bcAclSet, []string{"AclEntry"})
	bcAclSetAclEntry := setup.GetAnyValue(bcAclSet.AclEntry)
	setup.ResetStruct(bcAclSetAclEntry, []string{"Actions", "Ipv6"})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestSourceFlowLabel(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint32{
		779211,
		105645,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-flow-label using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.SourceFlowLabel = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SourceFlowLabel != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-flow-label: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SourceFlowLabel != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-flow-label: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).SourceFlowLabel != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-flow-label fail: got %v", qs)
				}
			})
		})
	}
}
func TestDestinationFlowLabel(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint32{
		258757,
		196152,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-flow-label using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.DestinationFlowLabel = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DestinationFlowLabel != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-flow-label: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DestinationFlowLabel != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-flow-label: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).DestinationFlowLabel != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-flow-label fail: got %v", qs)
				}
			})
		})
	}
}
func TestSourceAddress(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"1::1/72",
		"f:C7:0::9:1D6/8",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.SourceAddress = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.SourceAddress != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.SourceAddress != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).SourceAddress != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address fail: got %v", qs)
				}
			})
		})
	}
}
func TestDscpSet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := [][]uint8{
		{
			43,
			40,
			35,
			49,
			2,
			54,
			23,
			41,
			59,
			20,
			19,
			10,
			52,
			9,
			7,
			38,
			38,
			23,
			16,
			8,
			57,
			48,
			59,
			3,
			38,
			42,
			36,
			9,
			12,
			24,
			24,
			6,
			47,
			33,
			17,
			22,
			0,
			31,
			32,
			0,
		},
		{
			36,
			42,
			2,
			42,
			50,
			45,
			2,
			40,
			18,
			4,
			21,
			57,
			6,
			49,
			54,
		},
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp-set using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			baseConfigAclSetAclEntryIpv6.DscpSet = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					for i, cg := range configGot.DscpSet {
						if cg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp-set: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					for i, sg := range stateGot.DscpSet {
						if sg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp-set: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).DscpSet != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp-set fail: got %v", qs)
				}
			})
		})
	}
}
func TestDscp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8{
		53,
		58,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.Dscp = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Dscp != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.Dscp != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).Dscp != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/dscp fail: got %v", qs)
				}
			})
		})
	}
}
func TestProtocol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Ipv6_Protocol_Union{
		oc.UnionUint8(146),
		oc.UnionUint8(128),
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			baseConfigAclSetAclEntryIpv6.Protocol = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.Protocol != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.Protocol != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).Protocol != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol fail: got %v", qs)
				}
			})
		})
	}
}
func TestDestinationAddress(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []string{
		"b4d5:EE:74:0:6D1:1:C0c6:0E4/99",
		"c::1/109",
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.DestinationAddress = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.DestinationAddress != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.DestinationAddress != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).DestinationAddress != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address fail: got %v", qs)
				}
			})
		})
	}
}
func TestHopLimit(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []uint8{
		253,
		87,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/hop-limit using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryIpv6 := baseConfigAclSetAclEntry.Ipv6
			*baseConfigAclSetAclEntryIpv6.HopLimit = input

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Ipv6()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryIpv6)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.HopLimit != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/hop-limit: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if *stateGot.HopLimit != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/hop-limit: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).HopLimit != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/hop-limit fail: got %v", qs)
				}
			})
		})
	}
}
