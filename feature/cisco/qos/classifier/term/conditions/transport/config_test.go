package qos_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Classifier"})
	bcClassifier := setup.GetAnyValue(bc.Classifier)
	setup.ResetStruct(bcClassifier, []string{"Term"})
	bcClassifierTerm := setup.GetAnyValue(bcClassifier.Term)
	setup.ResetStruct(bcClassifierTerm, []string{"Conditions"})
	bcClassifierTermConditions := bcClassifierTerm.Conditions
	setup.ResetStruct(bcClassifierTermConditions, []string{"Transport"})
	bcClassifierTermConditionsTransport := bcClassifierTermConditions.Transport
	setup.ResetStruct(bcClassifierTermConditionsTransport, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
func TestDestinationPortAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []oc.Qos_Classifier_Term_Conditions_Transport_DestinationPort_Union{
		oc.UnionUint16(30900),
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/transport/config/destination-port using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsTransport := baseConfigClassifierTermConditions.Transport
			baseConfigClassifierTermConditionsTransport.DestinationPort = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsTransport)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.DestinationPort != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/transport/config/destination-port: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.DestinationPort != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/transport/config/destination-port: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).DestinationPort != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/transport/config/destination-port fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestDestinationPortAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []oc.Qos_Classifier_Term_Conditions_Transport_DestinationPort_Union{
		oc.UnionUint16(30900),
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/transport/config/destination-port using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport().DestinationPort()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport().DestinationPort()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/transport/config/destination-port: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/transport/config/destination-port: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/transport/config/destination-port fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourcePortAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []oc.Qos_Classifier_Term_Conditions_Transport_SourcePort_Union{
		oc.UnionString("15487..65529"),
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/transport/config/source-port using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsTransport := baseConfigClassifierTermConditions.Transport
			baseConfigClassifierTermConditionsTransport.SourcePort = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsTransport)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot.SourcePort != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/transport/config/source-port: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot.SourcePort != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/transport/config/source-port: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).SourcePort != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/transport/config/source-port fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestSourcePortAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := []oc.Qos_Classifier_Term_Conditions_Transport_SourcePort_Union{
		oc.UnionString("15487..65529"),
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/transport/config/source-port using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport().SourcePort()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport().SourcePort()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					if configGot != input {
						t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/transport/config/source-port: got %v, want %v", configGot, input)
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					if stateGot != input {
						t.Errorf("State /qos/classifiers/classifier/terms/term/conditions/transport/config/source-port: got %v, want %v", stateGot, input)
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/transport/config/source-port fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestTcpFlagsAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := [][]oc.E_PacketMatchTypes_TCP_FLAGS{
		{
			oc.E_PacketMatchTypes_TCP_FLAGS(4), //TCP_FIN
			oc.E_PacketMatchTypes_TCP_FLAGS(3), //TCP_ECE
			oc.E_PacketMatchTypes_TCP_FLAGS(8), //TCP_URG
			oc.E_PacketMatchTypes_TCP_FLAGS(2), //TCP_CWR
			oc.E_PacketMatchTypes_TCP_FLAGS(5), //TCP_PSH
			oc.E_PacketMatchTypes_TCP_FLAGS(1), //TCP_ACK
			oc.E_PacketMatchTypes_TCP_FLAGS(7), //TCP_SYN
			oc.E_PacketMatchTypes_TCP_FLAGS(6), //TCP_RST
		},
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/transport/config/tcp-flags using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)
			baseConfigClassifierTermConditions := baseConfigClassifierTerm.Conditions
			baseConfigClassifierTermConditionsTransport := baseConfigClassifierTermConditions.Transport
			baseConfigClassifierTermConditionsTransport.TcpFlags = input

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport()

			t.Run("Replace container", func(t *testing.T) {
				config.Replace(t, baseConfigClassifierTermConditionsTransport)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					for i, cg := range configGot.TcpFlags {
						if cg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/transport/config/tcp-flags: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe container", func(t *testing.T) {
					stateGot := state.Get(t)
					for i, sg := range stateGot.TcpFlags {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/transport/config/tcp-flags: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete container", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs.Val(t).TcpFlags != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/transport/config/tcp-flags fail: got %v", qs)
					}
				}
			})
		})
	}
}
func TestTcpFlagsAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConfig := setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	inputs := [][]oc.E_PacketMatchTypes_TCP_FLAGS{
		{
			oc.E_PacketMatchTypes_TCP_FLAGS(4), //TCP_FIN
			oc.E_PacketMatchTypes_TCP_FLAGS(3), //TCP_ECE
			oc.E_PacketMatchTypes_TCP_FLAGS(8), //TCP_URG
			oc.E_PacketMatchTypes_TCP_FLAGS(2), //TCP_CWR
			oc.E_PacketMatchTypes_TCP_FLAGS(5), //TCP_PSH
			oc.E_PacketMatchTypes_TCP_FLAGS(1), //TCP_ACK
			oc.E_PacketMatchTypes_TCP_FLAGS(7), //TCP_SYN
			oc.E_PacketMatchTypes_TCP_FLAGS(6), //TCP_RST
		},
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /qos/classifiers/classifier/terms/term/conditions/transport/config/tcp-flags using value %v", input), func(t *testing.T) {
			baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
			baseConfigClassifierTerm := setup.GetAnyValue(baseConfigClassifier.Term)

			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport().TcpFlags()
			state := dut.Telemetry().Qos().Classifier(*baseConfigClassifier.Name).Term(*baseConfigClassifierTerm.Id).Conditions().Transport().TcpFlags()

			t.Run("Replace leaf", func(t *testing.T) {
				config.Replace(t, input)
			})
			if !setup.SkipGet() {
				t.Run("Get leaf", func(t *testing.T) {
					configGot := config.Get(t)
					for i, cg := range configGot {
						if cg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/transport/config/tcp-flags: got %v, want %v", cg, input[i])
						}
					}
				})
			}
			if !setup.SkipSubscribe() {
				t.Run("Subscribe leaf", func(t *testing.T) {
					stateGot := state.Get(t)
					for i, sg := range stateGot {
						if sg != input[i] {
							t.Errorf("Config /qos/classifiers/classifier/terms/term/conditions/transport/config/tcp-flags: got %v, want %v", sg, input[i])
						}
					}
				})
			}
			t.Run("Delete leaf", func(t *testing.T) {
				config.Delete(t)
				if !setup.SkipSubscribe() {
					if qs := config.Lookup(t); qs != nil {
						t.Errorf("Delete /qos/classifiers/classifier/terms/term/conditions/transport/config/tcp-flags fail: got %v", qs)
					}
				}
			})
		})
	}
}
