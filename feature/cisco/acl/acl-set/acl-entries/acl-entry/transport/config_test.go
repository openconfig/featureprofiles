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
	setup.ResetStruct(bc, []string{"AclSet"})
	bcAclSet := setup.GetAnyValue(bc.AclSet)
	setup.ResetStruct(bcAclSet, []string{"AclEntry"})
	bcAclSetAclEntry := setup.GetAnyValue(bcAclSet.AclEntry)
	setup.ResetStruct(bcAclSetAclEntry, []string{"Transport", "Actions", "Ipv4"})
	bcAclSetAclEntryTransport := bcAclSetAclEntry.Transport
	setup.ResetStruct(bcAclSetAclEntryTransport, []string{})
	gnmi.Replace(t, dut, gnmi.OC().Acl().Config(), bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	gnmi.Delete(t, dut, gnmi.OC().Acl().Config())
}
func TestSourcePort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Transport_SourcePort_Union{
		oc.UnionString("10001..10005"),
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryTransport := baseConfigAclSetAclEntry.Transport
			baseConfigAclSetAclEntryTransport.SourcePort = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Transport()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Transport()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryTransport)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot.SourcePort != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot.SourcePort != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.SourcePort != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationPort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Transport_DestinationPort_Union{
		oc.UnionString("10001..10005"),
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryTransport := baseConfigAclSetAclEntry.Transport
			baseConfigAclSetAclEntryTransport.DestinationPort = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Transport()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Transport()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryTransport)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					if configGot.DestinationPort != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot.DestinationPort != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.DestinationPort != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestTcpFlags(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := [][]oc.E_PacketMatchTypes_TCP_FLAGS{
		{
			oc.E_PacketMatchTypes_TCP_FLAGS(1), //TCP_ACK
			oc.E_PacketMatchTypes_TCP_FLAGS(7), //TCP_SYN
		},
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/tcp-flags using value %v", input), func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryTransport := baseConfigAclSetAclEntry.Transport
			baseConfigAclSetAclEntryTransport.ExplicitTcpFlags = input

			config := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Transport()
			state := gnmi.OC().Acl().AclSet(*baseConfigAclSet.Name, baseConfigAclSet.Type).AclEntry(*baseConfigAclSetAclEntry.SequenceId).Transport()

			t.Run("Replace", func(t *testing.T) {
				gnmi.Replace(t, dut, config.Config(), baseConfigAclSetAclEntryTransport)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := gnmi.GetConfig(t, dut, config.Config())
					for i, cg := range configGot.ExplicitTcpFlags {
						if cg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/tcp-flags: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					for i, sg := range stateGot.ExplicitTcpFlags {
						if sg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/tcp-flags: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				if !setup.SkipSubscribe() {
					if qs, _ := gnmi.LookupConfig(t, dut, config.Config()).Val(); qs.ExplicitTcpFlags != nil {
						t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/tcp-flags fail: got %v", qs)
					}
				}
			})
		})
	}
}
