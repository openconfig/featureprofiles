package qos_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestInterfaceInputClassifier(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface_ingress1.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

	for _, input := range testNameInput {
		baseConfigInterfaceInput := baseConfigInterface.Input
		baseConfigInterfaceInputClassifier := setup.GetAnyValue(baseConfigInterfaceInput.Classifier)
		*baseConfigInterfaceInputClassifier.Name = input

		config := gnmi.OC().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Classifier(baseConfigInterfaceInputClassifier.Type)
		state := gnmi.OC().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Classifier(baseConfigInterfaceInputClassifier.Type)

		// Classifier has to be applied for all 3 types
		t.Run("Replace container", func(t *testing.T) {
			t.Skip()
			gnmi.Replace(t, dut, config.Config(), baseConfigInterfaceInputClassifier)
		})

		t.Run("Get Interface Input Classifier Config", func(t *testing.T) {
			configGot := gnmi.GetConfig(t, dut, config.Config())
			if diff := cmp.Diff(*configGot, *baseConfigInterfaceInputClassifier); diff != "" {
				t.Errorf("Config Interface Input Classifier fail:\n%v", diff)
			}
		})
		// Returns telemetry data which will not be present in the baseConfig struct
		if !setup.SkipSubscribe() {
			t.Run("Get Interface Input Classifier Telemetry", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if diff := cmp.Diff(*stateGot, *baseConfigInterfaceInputClassifier); diff != "" {
					t.Errorf("Telemetry InterfaceInputClassifier fail:\n%v", diff)
				}
			})
		}
		// Deletes the pmap attached to intf
		t.Run("Delete Interface Input Classifier", func(t *testing.T) {
			gnmi.Delete(t, dut, config.Config())
			if !setup.SkipSubscribe() {
				if qs := gnmi.LookupConfig(t, dut, config.Config()); qs != nil {
					t.Errorf("Delete Interface Input Classifier fail: got %v", qs)
				}
			}
		})
	}
}

func TestInterfaceInputClassifierAtLeaf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface_ingress1.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)

	for _, input := range testNameInput {
		baseConfigInterfaceInput := baseConfigInterface.Input
		baseConfigInterfaceInputClassifier := setup.GetAnyValue(baseConfigInterfaceInput.Classifier)
		*baseConfigInterfaceInputClassifier.Name = input

		config := gnmi.OC().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Classifier(baseConfigInterfaceInputClassifier.Type).Name()
		state := gnmi.OC().Qos().Interface(*baseConfigInterface.InterfaceId).Input().Classifier(baseConfigInterfaceInputClassifier.Type).Name()

		// classifier has to applied for all 3 types
		t.Run("Replace container", func(t *testing.T) {
			t.Skip()
			gnmi.Replace(t, dut, config.Config(), input)
		})

		t.Run("Get Interface Input Classifier Config", func(t *testing.T) {
			configGot := gnmi.GetConfig(t, dut, config.Config())
			if configGot != input {
				t.Errorf("Config Interface Input Classifier: got %v want %v", configGot, input)
			}
		})
		if setup.SkipSubscribe() {
			t.Run("Get Interface Input Classifier Telemetry", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("Telemetry InterfaceInputClassifier: got %v want %v", stateGot, input)
				}
			})
		}
	}
}

func TestInterfaceInputTelemetry(t *testing.T) {

	// /qos/interfaces/interface/
	// // /qos/interfaces/interface/input/
	// /qos/interfaces/interface/input/classifiers/classifier/
	// /qos/interfaces/interface/input/classifiers/classifier/terms/term/
	// /qos/interfaces/interface/input/classifiers/classifier/terms/term/state/matched-packets
	// /qos/interfaces/interface/input/classifiers/classifier/terms/term/state/matched-octets

	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface_ingress1.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	interfaceTelemetryPath := gnmi.OC().Qos().Interface(*baseConfigInterface.InterfaceId)

	t.Run(fmt.Sprintf("Get Interface Telemetry %s", *baseConfigInterface.InterfaceId), func(t *testing.T) {
		got := gnmi.Get(t, dut, interfaceTelemetryPath.State())
		for classifierType, classifier := range got.Input.Classifier {
			for termId, term := range classifier.Term {
				t.Run(fmt.Sprintf("Verify Matched-Octets of %v %s", classifierType, termId), func(t *testing.T) {
					if !(*term.MatchedOctets == 0) {
						t.Errorf("Get Interface Telemetry fail: got %+v", *got)
					}
				})
				t.Run(fmt.Sprintf("Verify Matched-Packets of %v %s", classifierType, termId), func(t *testing.T) {
					if !(*term.MatchedPackets == 0) {
						t.Errorf("Get Interface Telemetry fail: got %+v", *got)
					}
				})
			}
		}
	})

	baseConfigInterfaceInput := baseConfigInterface.Input
	interfaceInputTelemetryPath := interfaceTelemetryPath.Input()

	baseConfigInterfaceInputClassifier := setup.GetAnyValue(baseConfigInterfaceInput.Classifier)
	interfaceInputClassifierTelemetryPath := interfaceInputTelemetryPath.Classifier(baseConfigInterfaceInputClassifier.Type)

	t.Run(fmt.Sprintf("Get Interface Input Classifier Telemetry %s %v", *baseConfigInterface.InterfaceId, baseConfigInterfaceInputClassifier.Type), func(t *testing.T) {
		got := gnmi.Get(t, dut, interfaceInputClassifierTelemetryPath.State())
		for termId, term := range got.Term {
			t.Run(fmt.Sprintf("Verify Matched-Octets of %s", termId), func(t *testing.T) {
				if !(*term.MatchedOctets == 0) {
					t.Errorf("Get Interface Input Classifier Telemetry fail: got %+v", *got)
				}
			})
			t.Run(fmt.Sprintf("Verify Matched-Packets of %s", termId), func(t *testing.T) {
				if !(*term.MatchedPackets == 0) {
					t.Errorf("Get Interface Input Classifier Telemetry fail: got %+v", *got)
				}
			})
		}
	})

	baseConfigClassifier := baseConfig.Classifier[*baseConfigInterfaceInputClassifier.Name]
	var termId string
	switch baseConfigInterfaceInputClassifier.Type {
	case 1:
		termId = "cmap_ipv4"
	case 2:
		termId = "cmap_ipv6"
	case 3:
		termId = "cmap_mpls"
	}
	baseConfigClassifierTerm := baseConfigClassifier.Term[termId]
	interfaceInputClassifierTermTelemetryPath := interfaceInputClassifierTelemetryPath.Term(*baseConfigClassifierTerm.Id)

	t.Run(fmt.Sprintf("Get Interface Input Classifier Telemetry %s %v %s", *baseConfigInterface.InterfaceId, baseConfigInterfaceInputClassifier.Type, *baseConfigClassifierTerm.Id), func(t *testing.T) {
		got := gnmi.Get(t, dut, interfaceInputClassifierTermTelemetryPath.State())
		t.Run("Verify Matched-Octets", func(t *testing.T) {
			if !(*got.MatchedOctets == 0) {
				t.Errorf("Get Interface Input Classifier Term Telemetry fail: got %+v", *got)
			}
		})
		t.Run("Verify Matched-Packets", func(t *testing.T) {
			if !(*got.MatchedPackets == 0) {
				t.Errorf("Get Interface Input Classifier Term Telemetry fail: got %+v", *got)
			}
		})
	})

	matchedOctetsPath := interfaceInputClassifierTermTelemetryPath.MatchedOctets()
	matchedPacketsPath := interfaceInputClassifierTermTelemetryPath.MatchedPackets()

	t.Run("Get Matched-Octets", func(t *testing.T) {
		matchedOctets := gnmi.Get(t, dut, matchedOctetsPath.State())
		if matchedOctets != 0 {
			t.Errorf("Get Matched-Octets fail: got %v", matchedOctets)
		}
	})
	t.Run("Get Matched-Packets", func(t *testing.T) {
		matchedPackets := gnmi.Get(t, dut, matchedPacketsPath.State())
		if matchedPackets != 0 {
			t.Errorf("Get Matched-Packets fail: got %v", matchedPackets)
		}
	})
}
