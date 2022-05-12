
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
	setup.ResetStruct(bcAclSetAclEntry, []string{"Transport", "Actions"})
	dut.Config().Acl().Replace(t, bc)
	return bc
}

func teardownAcl(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Acl) {
	dut.Config().Acl().Delete(t)
}
func TestTcpFlags(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := [][]oc.E_PacketMatchTypes_TCP_FLAGS {
		[]oc.E_PacketMatchTypes_TCP_FLAGS {
			oc.E_PacketMatchTypes_TCP_FLAGS(5), //TCP_PSH
			oc.E_PacketMatchTypes_TCP_FLAGS(8), //TCP_URG
			oc.E_PacketMatchTypes_TCP_FLAGS(2), //TCP_CWR
			oc.E_PacketMatchTypes_TCP_FLAGS(3), //TCP_ECE
			oc.E_PacketMatchTypes_TCP_FLAGS(7), //TCP_SYN
			oc.E_PacketMatchTypes_TCP_FLAGS(1), //TCP_ACK
			oc.E_PacketMatchTypes_TCP_FLAGS(6), //TCP_RST
			oc.E_PacketMatchTypes_TCP_FLAGS(4), //TCP_FIN
		},
		[]oc.E_PacketMatchTypes_TCP_FLAGS {
		},
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/tcp-flags using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryTransport := baseConfigAclSetAclEntry.Transport
			baseConfigAclSetAclEntryTransport.TcpFlags = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Transport()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Transport()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryTransport)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					for i, cg := range configGot.TcpFlags {
						if cg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/tcp-flags: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					for i, sg := range stateGot.TcpFlags {
						if sg != input[i] {
							t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/tcp-flags: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).TcpFlags != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/tcp-flags fail: got %v", qs)
				}
			})
		})
	}
}
func TestDestinationPort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Transport_DestinationPort_Union {
		oc.UnionString("17568..0765"), 
		oc.UnionUint16(43097), 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryTransport := baseConfigAclSetAclEntry.Transport
			baseConfigAclSetAclEntryTransport.DestinationPort = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Transport()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Transport()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryTransport)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.DestinationPort != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.DestinationPort != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).DestinationPort != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port fail: got %v", qs)
				}
			})
		})
	}
}
func TestSourcePort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	baseConfig := setupAcl(t, dut)
	defer teardownAcl(t, dut, baseConfig)

	inputs := []oc.Acl_AclSet_AclEntry_Transport_SourcePort_Union {
		oc.UnionString("65532..62160"), 
		oc.UnionString("61031..38337"), 
	}
	

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port using value %v", input) , func(t *testing.T) {
			baseConfigAclSet := setup.GetAnyValue(baseConfig.AclSet)
			baseConfigAclSetAclEntry := setup.GetAnyValue(baseConfigAclSet.AclEntry)
			baseConfigAclSetAclEntryTransport := baseConfigAclSetAclEntry.Transport
			baseConfigAclSetAclEntryTransport.SourcePort = input 

			config := dut.Config().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Transport()
			state := dut.Telemetry().Acl().AclSet(*baseConfigAclSet.Name,baseConfigAclSet.Type,).AclEntry(*baseConfigAclSetAclEntry.SequenceId,).Transport()

			t.Run("Replace", func(t *testing.T) {
				config.Replace(t, baseConfigAclSetAclEntryTransport)
			})
			if !setup.SkipGet() {
				t.Run("Get", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.SourcePort != input {
						t.Errorf("Config /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.SourcePort != input {
						t.Errorf("State /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				if qs := config.Lookup(t); qs.Val(t).SourcePort != nil {
					t.Errorf("Delete /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port fail: got %v", qs)
				}
			})
		})
	}
}
