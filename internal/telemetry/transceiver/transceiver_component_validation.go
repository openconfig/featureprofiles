package transceiver

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	minAllowedTemperature = 25.0
	maxAllowedTemperature = 80.0
)

// validateTranscieverTelemetry validates the transceiver telemetry.
func validateTranscieverTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus, transceiverStream *samplestream.SampleStream[*oc.Component]) {
	nextTransceiverSample, ok := transceiverStream.AwaitNext(timeout, func(v *ygnmi.Value[*oc.Component]) bool {
		val, present := v.Val()
		return present && isTransceiverStreamReady(val, wantOperStatus, params)
	})
	if !ok {
		t.Fatalf("Transceiver %v is not ready after %v minutes.", params.TransceiverNames[p.Name()], timeout.Minutes())
	}
	transceiverValue, ok := nextTransceiverSample.Val()
	if !ok {
		t.Fatalf("Transceiver value is empty for port %v.", p.Name())
	}
	tcs := []testcase{
		{
			desc: "Transceiver Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetName(),
			want: params.TransceiverNames[p.Name()],
		},
		{
			desc: "Transceiver Parent Validation",
			path: fmt.Sprintf(componentPath+"/state/parent", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetParent(),
			want: params.HWPortNames[p.Name()],
		},
		{
			desc:           "Transceiver Location Validation",
			path:           fmt.Sprintf(componentPath+"/state/location", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetLocation(),
			patternToMatch: strings.Replace(strings.Replace(params.TransceiverNames[p.Name()], "Ethernet", "", 1), "transceiver", "", 1),
		},
		{
			desc: "Transceiver Type Validation",
			path: fmt.Sprintf(componentPath+"/state/type", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetType().(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT),
			want: oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER,
		},
		{
			desc:       "Transceiver Oper-Status Validation",
			path:       fmt.Sprintf(componentPath+"/state/oper-status", params.TransceiverNames[p.Name()]),
			operStatus: oc.Interface_OperStatus_UP,
			got:        transceiverValue.GetOperStatus(),
			want:       oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
		},
		{
			desc:           "Transceiver Firmware Version Validation",
			path:           fmt.Sprintf(componentPath+"/state/firmware-version", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetFirmwareVersion(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Hardware Version Validation",
			path:           fmt.Sprintf(componentPath+"/state/hardware-version", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetHardwareVersion(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Serial Number Validation",
			path:           fmt.Sprintf(componentPath+"/state/serial-no", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetSerialNo(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Part Number Validation",
			path:           fmt.Sprintf(componentPath+"/state/part-no", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetPartNo(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver mfg-name Validation",
			path:           fmt.Sprintf(componentPath+"/state/mfg-name", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetMfgName(),
			patternToMatch: `(CIENA|CISCO|LUMENTUM|NOKIA|INFINERA|ACACIA|MARVEL|NeoPhotonics)`,
		},
		{
			desc:       "Transceiver Supply Voltage Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/supply-voltage/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetSupplyVoltage().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSupplyVoltage,
			maxAllowed: maxAllowedSupplyVoltage,
		},
		{
			desc: "Transceiver Physical Channel Index Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/index", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetTransceiver().GetChannel(0).GetIndex(),
			want: uint16(0),
		},
		{
			desc:       "Transceiver Physical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/output-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/output-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin() - math.Abs(transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance,
			maxAllowed: transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax() + math.Abs(transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Transceiver Physical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/avg", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin() - math.Abs(transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance,
			maxAllowed: transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax() + math.Abs(transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Transceiver Physical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/avg", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/min", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/min", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/max", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/max", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
	}
	if dut.Vendor() != ondatra.NOKIA {
		tcs = append(tcs, testcase{
			desc:       "Transceiver Temperature Validation",
			path:       fmt.Sprintf(componentPath+"/state/temperature/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTemperature().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedTemperature,
			maxAllowed: maxAllowedTemperature,
		})
	}
	if dut.Vendor() != ondatra.JUNIPER && dut.Vendor() != ondatra.CISCO {
		tcs = append(tcs, testcase{
			desc: "Transceiver Form Factor Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/form-factor", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetTransceiver().GetFormFactor(),
			want: params.FormFactor,
		})
	}
	for _, tc := range tcs {
		if tc.operStatus != oc.Interface_OperStatus_UNSET && tc.operStatus != wantOperStatus {
			// Skip the validation if the operStatus is not the same as the expected operStatus.
			continue
		}
		t.Run(fmt.Sprintf("%s of %v", tc.desc, p.Name()), func(t *testing.T) {
			t.Logf("\n%s: %s = %v\n\n", p.Name(), tc.path, tc.got)
			switch {
			case tc.patternToMatch != "":
				val, ok := tc.got.(string)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want string\n\n", p.Name(), tc.path, tc.got)
				}
				if !regexp.MustCompile(tc.patternToMatch).MatchString(val) {
					t.Errorf("\n%s: %s, invalid:\n got %v, want pattern %v\n\n", p.Name(), tc.path, tc.got, tc.patternToMatch)
				}
			case tc.operStatus != oc.Interface_OperStatus_UNSET && tc.want != nil:
				if diff := cmp.Diff(tc.got, tc.want); diff != "" {
					t.Errorf("\n%s: %s, diff (-got +want):\n%s\n\n", p.Name(), tc.path, diff)
				}
			case tc.operStatus == oc.Interface_OperStatus_UP:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", p.Name(), tc.path, tc.got)
				}
				if val < tc.minAllowed || val > tc.maxAllowed {
					t.Errorf("\n%s: %s, out of range:\n got %v want >= %v, <= %v\n\n", p.Name(), tc.path, tc.got, tc.minAllowed, tc.maxAllowed)
				}
			case tc.operStatus == oc.Interface_OperStatus_DOWN:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", p.Name(), tc.path, tc.got)
				}
				if val > tc.maxAllowed {
					t.Errorf("\n%s: %s, out of range:\n got %v want <= %v\n\n", p.Name(), tc.path, tc.got, tc.maxAllowed)
				}
			default:
				if diff := cmp.Diff(tc.got, tc.want); diff != "" {
					t.Errorf("\n%s: %s, diff (-got +want):\n%s\n\n", p.Name(), tc.path, diff)
				}
			}
		})
	}
}

// isTransceiverStreamReady returns true if the transceiver stream is ready for the given oper status and params.
func isTransceiverStreamReady(val *oc.Component, operStatus oc.E_Interface_OperStatus, params *cfgplugins.ConfigParameters) bool {
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		return val.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMax() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMax() >= (params.TargetOpticalPower-powerLoss-powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMin() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMin() >= (params.TargetOpticalPower-powerLoss-powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetAvg() <= (val.GetTransceiver().GetChannel(0).GetInputPower().GetMax()+math.Abs(val.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetAvg() >= (val.GetTransceiver().GetChannel(0).GetInputPower().GetMin()-math.Abs(val.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetInstant() <= (val.GetTransceiver().GetChannel(0).GetInputPower().GetMax()+math.Abs(val.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetInstant() >= (val.GetTransceiver().GetChannel(0).GetInputPower().GetMin()-math.Abs(val.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance) &&
			val.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant() >= (params.TargetOpticalPower-powerLoss-powerReadingError)
	default:
		return val.GetTransceiver().GetChannel(0).GetInputPower().GetMax() <= inactivePower &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMin() <= inactivePower &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetAvg() <= inactivePower &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetInstant() <= inactivePower &&
			val.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant() <= inactivePower
	}
}

// InventorySnapshot holds baseline hardware inventory strings and bindings for a transceiver component.
type InventorySnapshot struct {
	Name            string
	Parent          string
	FormFactor      oc.E_TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE
	SerialNo        string
	PartNo          string
	MfgName         string
	MfgDate         string
	HardwareVersion string
	FirmwareVersion string
}

// OnChangeInventoryWatcher maintains a continuous ON_CHANGE gNMI subscription on a transceiver
// component state tree and records any gNMI delete notification or inventory string corruption.
type OnChangeInventoryWatcher struct {
	mu         sync.Mutex
	trName     string
	baseline   *InventorySnapshot
	synced     bool
	violations []string
	cancel     context.CancelFunc
}

// StartOnChangeInventoryWatcher starts a continuous ON_CHANGE gNMI subscription on
// /components/component[name=<trName>]/state inventory leaves that monitors for delete notifications or inventory mismatches.
func StartOnChangeInventoryWatcher(t *testing.T, dut *ondatra.DUTDevice, trName string, baseline *InventorySnapshot) *OnChangeInventoryWatcher {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	w := &OnChangeInventoryWatcher{
		trName:   trName,
		baseline: baseline,
		cancel:   cancel,
	}
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		t.Fatalf("Unable to connect to gNMI on %s for ON_CHANGE inventory watch: %v", dut.ID(), err)
	}
	invBatch := gnmi.OCBatch()
	invBatch.AddPaths(
		gnmi.OC().Component(trName).Name(),
		gnmi.OC().Component(trName).Type(),
		gnmi.OC().Component(trName).Parent(),
		gnmi.OC().Component(trName).OperStatus(),
		gnmi.OC().Component(trName).SerialNo(),
		gnmi.OC().Component(trName).PartNo(),
		gnmi.OC().Component(trName).MfgName(),
		gnmi.OC().Component(trName).MfgDate(),
		gnmi.OC().Component(trName).HardwareVersion(),
		gnmi.OC().Component(trName).FirmwareVersion(),
	)
	// ygnmi.Watch starts an asynchronous STREAM subscription in its own internal goroutine and returns
	// *ygnmi.Watcher[T] immediately (only calling .Await() blocks). The context.Context (ctx) passed into
	// ygnmi.Watch(ctx, ...) controls the lifetime of the background ON_CHANGE stream until w.Close() cancels it.
	ygnmi.Watch(ctx, c, invBatch.State(), func(v *ygnmi.Value[*oc.Root]) error {
		w.mu.Lock()
		defer w.mu.Unlock()
		root, present := v.Val()
		if !present || root == nil || root.GetComponent(trName) == nil {
			if w.synced {
				w.violations = append(w.violations, fmt.Sprintf("received gNMI delete or nil component notification for /components/component[name=%s]/state", trName))
			}
			return ygnmi.Continue
		}
		comp := root.GetComponent(trName)
		if !w.synced {
			if comp.GetName() != "" && comp.GetParent() != "" &&
				comp.GetSerialNo() != "" && comp.GetPartNo() != "" && comp.GetMfgName() != "" &&
				comp.GetHardwareVersion() != "" && comp.GetFirmwareVersion() != "" &&
				(deviations.ComponentMfgDateUnsupported(dut) || comp.GetMfgDate() != "") {
				w.synced = true
			} else {
				return ygnmi.Continue
			}
		}
		if baseline != nil {
			if got := comp.GetName(); got == "" || got != baseline.Name {
				w.violations = append(w.violations, fmt.Sprintf("component %s name changed in ON_CHANGE stream: got %q, want %q", trName, got, baseline.Name))
			}
			if got := comp.GetType(); got != nil && got != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER {
				w.violations = append(w.violations, fmt.Sprintf("component %s type changed in ON_CHANGE stream: got %v, want %v", trName, got, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER))
			}
			if got := comp.GetParent(); got == "" || got != baseline.Parent {
				w.violations = append(w.violations, fmt.Sprintf("component %s parent changed in ON_CHANGE stream: got %q, want %q", trName, got, baseline.Parent))
			}
			if got := comp.GetSerialNo(); got == "" || got != baseline.SerialNo {
				w.violations = append(w.violations, fmt.Sprintf("component %s serial-no changed in ON_CHANGE stream: got %q, want %q", trName, got, baseline.SerialNo))
			}
			if got := comp.GetPartNo(); got == "" || got != baseline.PartNo {
				w.violations = append(w.violations, fmt.Sprintf("component %s part-no changed in ON_CHANGE stream: got %q, want %q", trName, got, baseline.PartNo))
			}
			if got := comp.GetMfgName(); got == "" || got != baseline.MfgName {
				w.violations = append(w.violations, fmt.Sprintf("component %s mfg-name changed in ON_CHANGE stream: got %q, want %q", trName, got, baseline.MfgName))
			}
			if !deviations.ComponentMfgDateUnsupported(dut) {
				if got := comp.GetMfgDate(); got == "" || got != baseline.MfgDate {
					w.violations = append(w.violations, fmt.Sprintf("component %s mfg-date changed in ON_CHANGE stream: got %q, want %q", trName, got, baseline.MfgDate))
				}
			}
			if got := comp.GetHardwareVersion(); got == "" || got != baseline.HardwareVersion {
				w.violations = append(w.violations, fmt.Sprintf("component %s hardware-version changed in ON_CHANGE stream: got %q, want %q", trName, got, baseline.HardwareVersion))
			}
			if got := comp.GetFirmwareVersion(); got == "" || got != baseline.FirmwareVersion {
				w.violations = append(w.violations, fmt.Sprintf("component %s firmware-version changed in ON_CHANGE stream: got %q, want %q", trName, got, baseline.FirmwareVersion))
			}
		}
		return ygnmi.Continue
	}, ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
	return w
}

// VerifyNoViolations checks that no delete notifications or inventory mismatches were observed on the ON_CHANGE stream.
func (w *OnChangeInventoryWatcher) VerifyNoViolations(t *testing.T) {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, v := range w.violations {
		t.Errorf("ON_CHANGE inventory persistence violation on %s: %s", w.trName, v)
	}
	w.violations = nil
}

// Close stops the continuous ON_CHANGE gNMI subscription.
func (w *OnChangeInventoryWatcher) Close() {
	if w != nil && w.cancel != nil {
		w.cancel()
	}
}

// lookupVal executes gnmi.Lookup with retry on transient gRPC proxy errors (such as Unauthenticated).
func lookupVal[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T]) (T, bool) {
	t.Helper()
	var val T
	var present bool
	var lastMsg *string
	for attempt := 0; attempt < 3; attempt++ {
		lastMsg = testt.CaptureFatal(t, func(tb testing.TB) {
			val, present = gnmi.Lookup(tb, dut, q).Val()
		})
		if lastMsg == nil {
			return val, present
		}
	}
	if lastMsg != nil {
		t.Errorf("gNMI Lookup failed after 3 attempts: %s", *lastMsg)
	}
	return val, false
}

// watchAwait executes gnmi.Watch(...).Await with retry on transient gRPC proxy errors (such as Unauthenticated).
func watchAwait[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T], timeout time.Duration, pred func(*ygnmi.Value[T]) bool) (*ygnmi.Value[T], bool) {
	t.Helper()
	var lastVal *ygnmi.Value[T]
	var ok bool
	var lastMsg *string
	for attempt := 0; attempt < 3; attempt++ {
		lastMsg = testt.CaptureFatal(t, func(tb testing.TB) {
			lastVal, ok = gnmi.Watch(tb, dut, q, timeout, pred).Await(tb)
		})
		if lastMsg == nil {
			return lastVal, ok
		}
	}
	if lastMsg != nil {
		t.Errorf("gNMI Watch Await failed after 3 attempts: %s", *lastMsg)
	}
	return lastVal, false
}

// componentInventoryQuery returns a batched query for the transceiver component's inventory and state leaves.
func componentInventoryQuery(dut *ondatra.DUTDevice, trName string) ygnmi.SingletonQuery[*oc.Root] {
	b := gnmi.OCBatch()
	b.AddPaths(
		gnmi.OC().Component(trName).Name(),
		gnmi.OC().Component(trName).Type(),
		gnmi.OC().Component(trName).Parent(),
		gnmi.OC().Component(trName).OperStatus(),
		gnmi.OC().Component(trName).SerialNo(),
		gnmi.OC().Component(trName).PartNo(),
		gnmi.OC().Component(trName).MfgName(),
		gnmi.OC().Component(trName).HardwareVersion(),
		gnmi.OC().Component(trName).FirmwareVersion(),
		gnmi.OC().Component(trName).Transceiver().FormFactor(),
	)
	if !deviations.ComponentMfgDateUnsupported(dut) {
		b.AddPaths(gnmi.OC().Component(trName).MfgDate())
	}
	if !deviations.TransceiverConfigEnableUnsupported(dut) {
		b.AddPaths(gnmi.OC().Component(trName).Transceiver().Enabled())
	}
	return b.State()
}

// componentBasicQuery returns a batched query for a component's basic identity leaves (name, type, parent).
func componentBasicQuery(compName string) ygnmi.SingletonQuery[*oc.Root] {
	b := gnmi.OCBatch()
	b.AddPaths(
		gnmi.OC().Component(compName).Name(),
		gnmi.OC().Component(compName).Type(),
		gnmi.OC().Component(compName).Parent(),
	)
	return b.State()
}

// CaptureAndValidateBaselineInventory waits for the transceiver component to be ACTIVE with populated
// inventory strings via ON_CHANGE gNMI subscription (gnmi.Watch) and gnmi.Lookup, validates all required
// PORT, TRANSCEIVER, and OPTICAL_CHANNEL component state leaves and leafref paths, and returns the captured baseline snapshot.
func CaptureAndValidateBaselineInventory(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, watchTimeout time.Duration) *InventorySnapshot {
	t.Helper()
	trName := params.TransceiverNames[p.Name()]
	hwPortName := params.HWPortNames[p.Name()]
	ochName := params.OpticalChannelNames[p.Name()]
	ethIdx := params.ETHIndexes[p.Name()]

	// Wait via ON_CHANGE gNMI subscription (gnmi.Watch) for transceiver oper-status == ACTIVE and inventory strings to populate.
	invQuery := componentInventoryQuery(dut, trName)
	root, present := lookupVal(t, dut, invQuery)
	comp := root.GetComponent(trName)
	if !present || comp == nil || comp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE ||
		comp.GetSerialNo() == "" || comp.GetPartNo() == "" || comp.GetMfgName() == "" ||
		comp.GetHardwareVersion() == "" || comp.GetFirmwareVersion() == "" {
		lastVal, ok := watchAwait(t, dut, invQuery, watchTimeout, func(val *ygnmi.Value[*oc.Root]) bool {
			r, ok := val.Val()
			if !ok || r == nil {
				return false
			}
			c := r.GetComponent(trName)
			if c == nil || c.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				return false
			}
			return c.GetSerialNo() != "" && c.GetPartNo() != "" && c.GetMfgName() != "" &&
				c.GetHardwareVersion() != "" && c.GetFirmwareVersion() != ""
		})
		if !ok {
			t.Fatalf("Transceiver %s on port %s did not reach ACTIVE state with populated inventory within %v", trName, p.Name(), watchTimeout)
		}
		if lastVal != nil {
			if r, ok := lastVal.Val(); ok && r != nil {
				comp = r.GetComponent(trName)
			}
		}
		if comp == nil {
			r, _ := lookupVal(t, dut, invQuery)
			comp = r.GetComponent(trName)
		}
	}
	if comp == nil {
		t.Fatalf("Transceiver %s on port %s state is nil", trName, p.Name())
	}

	// Validate PORT component state (name, type, parent).
	hwRoot, hwOK := lookupVal(t, dut, componentBasicQuery(hwPortName))
	if hwComp := hwRoot.GetComponent(hwPortName); !hwOK || hwComp == nil {
		t.Errorf("Port %s hardware port component %s missing from /components/component", p.Name(), hwPortName)
	} else {
		if got, want := hwComp.GetName(), hwPortName; got != want {
			t.Errorf("Port %s /components/component[name=%s]/state/name = %q, want %q", p.Name(), hwPortName, got, want)
		}
		if got, want := hwComp.GetType(), oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT; got != want {
			t.Errorf("Port %s /components/component[name=%s]/state/type = %v, want %v", p.Name(), hwPortName, got, want)
		}
		if got := hwComp.GetParent(); got == "" {
			t.Errorf("Port %s /components/component[name=%s]/state/parent is empty, want non-empty parent component", p.Name(), hwPortName)
		}
	}

	// Validate OPTICAL_CHANNEL component state (name, type, parent).
	ochRoot, ochOK := lookupVal(t, dut, componentBasicQuery(ochName))
	if ochComp := ochRoot.GetComponent(ochName); !ochOK || ochComp == nil {
		t.Errorf("Port %s optical-channel component %s missing from /components/component", p.Name(), ochName)
	} else {
		if got, want := ochComp.GetName(), ochName; got != want {
			t.Errorf("Port %s /components/component[name=%s]/state/name = %q, want %q", p.Name(), ochName, got, want)
		}
		if got, want := ochComp.GetType(), oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_OPTICAL_CHANNEL; got != want {
			t.Errorf("Port %s /components/component[name=%s]/state/type = %v, want %v", p.Name(), ochName, got, want)
		}
		if got := ochComp.GetParent(); got == "" {
			t.Errorf("Port %s /components/component[name=%s]/state/parent is empty, want non-empty parent transceiver (%s)", p.Name(), ochName, trName)
		}
	}

	// Validate TRANSCEIVER component state and inventory strings.
	if got, want := comp.GetName(), trName; got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/name = %q, want %q", p.Name(), trName, got, want)
	}
	if got, want := comp.GetType(), oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER; got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/type = %v, want %v", p.Name(), trName, got, want)
	}
	if got, want := comp.GetParent(), hwPortName; got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/parent = %q, want %q", p.Name(), trName, got, want)
	}
	if got, want := comp.GetOperStatus(), oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/oper-status = %v, want %v", p.Name(), trName, got, want)
	}
	if !deviations.TransceiverConfigEnableUnsupported(dut) {
		if got := comp.GetTransceiver().GetEnabled(); !got {
			t.Errorf("Port %s /components/component[name=%s]/transceiver/state/enabled = %v, want true", p.Name(), trName, got)
		}
	}
	if got := comp.GetTransceiver().GetFormFactor(); got != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_UNSET &&
		got != params.FormFactor &&
		got != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_OSFP &&
		got != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_QSFP56_DD &&
		got != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_QSFP56_DD_TYPE2 {
		t.Errorf("Port %s /components/component[name=%s]/transceiver/state/form-factor = %v, want OSFP or QSFP56_DD (%v)", p.Name(), trName, got, params.FormFactor)
	}
	if comp.GetSerialNo() == "" {
		t.Errorf("Port %s /components/component[name=%s]/state/serial-no is empty, want non-empty string", p.Name(), trName)
	}
	if comp.GetPartNo() == "" {
		t.Errorf("Port %s /components/component[name=%s]/state/part-no is empty, want non-empty string", p.Name(), trName)
	}
	if comp.GetMfgName() == "" {
		t.Errorf("Port %s /components/component[name=%s]/state/mfg-name is empty, want non-empty string", p.Name(), trName)
	}
	if !deviations.ComponentMfgDateUnsupported(dut) {
		if got := comp.GetMfgDate(); !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(got) {
			t.Errorf("Port %s /components/component[name=%s]/state/mfg-date = %q, want YYYY-MM-DD", p.Name(), trName, got)
		}
	}
	if comp.GetHardwareVersion() == "" {
		t.Errorf("Port %s /components/component[name=%s]/state/hardware-version is empty, want non-empty string", p.Name(), trName)
	}
	if comp.GetFirmwareVersion() == "" {
		t.Errorf("Port %s /components/component[name=%s]/state/firmware-version is empty, want non-empty string", p.Name(), trName)
	}

	// Validate interface and ETH logical channel leafref bindings via Lookup and ON_CHANGE watch fallback.
	if got, ok := lookupVal(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()); !ok || got != trName {
		_, ok := watchAwait(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State(), watchTimeout, func(val *ygnmi.Value[string]) bool {
			v, present := val.Val()
			return present && v == trName
		})
		if !ok {
			got, _ := lookupVal(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
			t.Errorf("Port %s /interfaces/interface[name=%s]/state/transceiver = %q, want %q", p.Name(), p.Name(), got, trName)
		}
	}
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		if got, _ := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State()); got != p.Name() {
			t.Errorf("Port %s /terminal-device/logical-channels/channel[index=%d]/ingress/state/interface = %q, want %q", p.Name(), ethIdx, got, p.Name())
		}
		if got, _ := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State()); got != trName {
			t.Errorf("Port %s /terminal-device/logical-channels/channel[index=%d]/ingress/state/transceiver = %q, want %q", p.Name(), ethIdx, got, trName)
		}
	}

	return &InventorySnapshot{
		Name:            comp.GetName(),
		Parent:          comp.GetParent(),
		FormFactor:      comp.GetTransceiver().GetFormFactor(),
		SerialNo:        comp.GetSerialNo(),
		PartNo:          comp.GetPartNo(),
		MfgName:         comp.GetMfgName(),
		MfgDate:         comp.GetMfgDate(),
		HardwareVersion: comp.GetHardwareVersion(),
		FirmwareVersion: comp.GetFirmwareVersion(),
	}
}

// VerifyInventoryMatchesBaseline verifies via ON_CHANGE gNMI subscription (gnmi.Watch) and gnmi.Lookup
// that the transceiver component remains present in the OpenConfig component tree (no delete notification)
// and that all inventory strings and leafref bindings match the captured baseline snapshot.
func VerifyInventoryMatchesBaseline(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, baseline *InventorySnapshot, expectInterfaceBound bool, expectActive bool, expectEnabled bool) {
	t.Helper()
	if baseline == nil {
		baseline = CaptureAndValidateBaselineInventory(t, dut, p, params, 30*time.Second)
	}
	trName := params.TransceiverNames[p.Name()]
	ethIdx := params.ETHIndexes[p.Name()]

	// Verify via gNMI Lookup (and ON_CHANGE Watch fallback if not yet converged) that the component is present,
	// inventory strings are non-empty, and (if expectActive is true) oper-status is ACTIVE.
	invQuery := componentInventoryQuery(dut, trName)
	rootVal, present := lookupVal(t, dut, invQuery)
	compVal := rootVal.GetComponent(trName)
	if !present || compVal == nil || (expectActive && compVal.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE) ||
		compVal.GetSerialNo() == "" || compVal.GetPartNo() == "" || compVal.GetMfgName() == "" {
		_, ok := watchAwait(t, dut, invQuery, 30*time.Second, func(val *ygnmi.Value[*oc.Root]) bool {
			r, present := val.Val()
			if !present || r == nil {
				return false
			}
			comp := r.GetComponent(trName)
			if comp == nil {
				return false
			}
			if expectActive && comp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				return false
			}
			return comp.GetSerialNo() != "" && comp.GetPartNo() != "" && comp.GetMfgName() != ""
		})
		if !ok {
			t.Errorf("Port %s ON_CHANGE watch on /components/component[name=%s]/state did not report present component with expected oper-status and non-empty inventory", p.Name(), trName)
		}
		rootVal, present = lookupVal(t, dut, invQuery)
		compVal = rootVal.GetComponent(trName)
	}
	if !present || compVal == nil {
		t.Fatalf("Port %s transceiver component %s is missing from /components/ tree", p.Name(), trName)
	}
	if got, want := compVal.GetName(), baseline.Name; got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/name = %q, want %q", p.Name(), trName, got, want)
	}
	if got, want := compVal.GetType(), oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER; got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/type = %v, want %v", p.Name(), trName, got, want)
	}
	if got, want := compVal.GetParent(), baseline.Parent; got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/parent = %q, want %q", p.Name(), trName, got, want)
	}
	if expectActive {
		if got, want := compVal.GetOperStatus(), oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
			t.Errorf("Port %s /components/component[name=%s]/state/oper-status = %v, want %v", p.Name(), trName, got, want)
		}
	}
	if !deviations.TransceiverConfigEnableUnsupported(dut) {
		if got, want := compVal.GetTransceiver().GetEnabled(), expectEnabled; got != want {
			t.Errorf("Port %s /components/component[name=%s]/transceiver/state/enabled = %v, want %v", p.Name(), trName, got, want)
		}
	}
	if baseline.FormFactor != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_UNSET {
		if got, want := compVal.GetTransceiver().GetFormFactor(), baseline.FormFactor; got != want {
			t.Errorf("Port %s /components/component[name=%s]/transceiver/state/form-factor = %v, want %v", p.Name(), trName, got, want)
		}
	}
	if got, want := compVal.GetSerialNo(), baseline.SerialNo; got == "" || got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/serial-no = %q, want %q", p.Name(), trName, got, want)
	}
	if got, want := compVal.GetPartNo(), baseline.PartNo; got == "" || got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/part-no = %q, want %q", p.Name(), trName, got, want)
	}
	if got, want := compVal.GetMfgName(), baseline.MfgName; got == "" || got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/mfg-name = %q, want %q", p.Name(), trName, got, want)
	}
	if !deviations.ComponentMfgDateUnsupported(dut) {
		if got, want := compVal.GetMfgDate(), baseline.MfgDate; got == "" || got != want {
			t.Errorf("Port %s /components/component[name=%s]/state/mfg-date = %q, want %q", p.Name(), trName, got, want)
		}
	}
	if got, want := compVal.GetHardwareVersion(), baseline.HardwareVersion; got == "" || got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/hardware-version = %q, want %q", p.Name(), trName, got, want)
	}
	if got, want := compVal.GetFirmwareVersion(), baseline.FirmwareVersion; got == "" || got != want {
		t.Errorf("Port %s /components/component[name=%s]/state/firmware-version = %q, want %q", p.Name(), trName, got, want)
	}

	if expectInterfaceBound {
		if got, ok := lookupVal(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()); !ok || got != trName {
			_, ok := watchAwait(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
				v, present := val.Val()
				return present && v == trName
			})
			if !ok {
				got, _ := lookupVal(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
				t.Errorf("Port %s /interfaces/interface[name=%s]/state/transceiver = %q, want %q", p.Name(), p.Name(), got, trName)
			}
		}
		if !deviations.EthChannelIngressParametersUnsupported(dut) {
			if got, ok := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State()); !ok || got != p.Name() {
				_, ok := watchAwait(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
					v, present := val.Val()
					return present && v == p.Name()
				})
				if !ok {
					got, _ := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State())
					t.Errorf("Port %s /terminal-device/logical-channels/channel[index=%d]/ingress/state/interface = %q, want %q", p.Name(), ethIdx, got, p.Name())
				}
			}
			if got, ok := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State()); !ok || got != trName {
				_, ok = watchAwait(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
					v, present := val.Val()
					return present && v == trName
				})
				if !ok {
					got, _ := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State())
					t.Errorf("Port %s /terminal-device/logical-channels/channel[index=%d]/ingress/state/transceiver = %q, want %q", p.Name(), ethIdx, got, trName)
				}
			}
		}
	}
}

// VerifyOpticalAndChannelTelemetry validates optical-channel state (operational-mode, frequency, target-output-power),
// optical input/output power levels, pre-FEC BER, and zero FEC uncorrectable blocks using ON_CHANGE and SAMPLE subscriptions.
func VerifyOpticalAndChannelTelemetry(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port, params *cfgplugins.ConfigParameters, wantOpMode uint16, wantFreq uint64, convergenceTimeout, sampleTimeout time.Duration) {
	t.Helper()
	const (
		minValidInputPowerdBm = -15.0
		maxValidInputPowerdBm = 2.0
		powerTolerance        = 1.0
		maxPreFECBER          = 1.25e-2
	)
	for _, p := range ports {
		ochName := params.OpticalChannelNames[p.Name()]
		trName := params.TransceiverNames[p.Name()]
		otnIdx := params.OTNIndexes[p.Name()]
		ethIdx := params.ETHIndexes[p.Name()]

		// Wait via ON_CHANGE subscription (gnmi.Watch) for interface oper-status == UP and logical channel link-state == UP.
		_, ok := watchAwait(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), convergenceTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			v, present := val.Val()
			return present && v == oc.Interface_OperStatus_UP
		})
		if !ok {
			t.Fatalf("Interface %s did not reach OperStatus UP within %v", p.Name(), convergenceTimeout)
		}

		for _, chIdx := range []uint32{otnIdx, ethIdx} {
			_, ok := watchAwait(t, dut, gnmi.OC().TerminalDevice().Channel(chIdx).State(), sampleTimeout, func(val *ygnmi.Value[*oc.TerminalDevice_Channel]) bool {
				v, present := val.Val()
				if !present || v == nil || v.GetIndex() != chIdx {
					return false
				}
				if !deviations.TerminalDeviceChannelAdminStateUnsupported(dut) && v.GetAdminState() != oc.TerminalDevice_AdminStateType_ENABLED {
					return false
				}
				return true
			})
			if !ok {
				t.Errorf("Port %s logical channel %d did not report valid index and admin-state ENABLED within %v", p.Name(), chIdx, sampleTimeout)
			}
		}

		if !deviations.OperationalModeUnsupported(dut) {
			_, ok := watchAwait(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State(), sampleTimeout, func(val *ygnmi.Value[uint16]) bool {
				v, present := val.Val()
				return present && v == wantOpMode
			})
			if !ok {
				gotMode, _ := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State())
				t.Errorf("Port %s /components/component[name=%s]/optical-channel/state/operational-mode = %d, want %d", p.Name(), ochName, gotMode, wantOpMode)
			}
		}

		gotFreq, _ := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().Frequency().State())
		if gotFreq != wantFreq {
			t.Errorf("Port %s /components/component[name=%s]/optical-channel/state/frequency = %d, want %d", p.Name(), ochName, gotFreq, wantFreq)
		}

		gotTargetPower, _ := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().TargetOutputPower().State())
		if math.Abs(gotTargetPower-params.TargetOpticalPower) > powerTolerance {
			t.Errorf("Port %s /components/component[name=%s]/optical-channel/state/target-output-power = %v, want %v (+/- %v dBm)", p.Name(), ochName, gotTargetPower, params.TargetOpticalPower, powerTolerance)
		}

		// Subscribe with SAMPLE mode (10s interval) to optical-channel, transceiver, and OTN logical channel telemetry.
		ochStream := samplestream.New(t, dut, gnmi.OC().Component(ochName).State(), samplingInterval)
		defer ochStream.Close()
		trStream := samplestream.New(t, dut, gnmi.OC().Component(trName).State(), samplingInterval)
		defer trStream.Close()
		otnStream := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIdx).State(), samplingInterval)
		defer otnStream.Close()

		_, ok = ochStream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[*oc.Component]) bool {
			c, present := v.Val()
			if !present || c == nil || c.GetOpticalChannel() == nil {
				return false
			}
			inPwr := c.GetOpticalChannel().GetInputPower().GetInstant()
			outPwr := c.GetOpticalChannel().GetOutputPower().GetInstant()
			return inPwr >= minValidInputPowerdBm && inPwr <= maxValidInputPowerdBm &&
				math.Abs(outPwr-params.TargetOpticalPower) <= powerTolerance
		})
		if !ok {
			t.Errorf("Port %s optical-channel %s did not report valid input/output power within %v", p.Name(), ochName, sampleTimeout)
		}

		_, ok = trStream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[*oc.Component]) bool {
			c, present := v.Val()
			if !present || c == nil || c.GetTransceiver() == nil || c.GetTransceiver().GetChannel(0) == nil {
				return false
			}
			inPwr := c.GetTransceiver().GetChannel(0).GetInputPower().GetInstant()
			outPwr := c.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant()
			return inPwr >= minValidInputPowerdBm && inPwr <= maxValidInputPowerdBm &&
				math.Abs(outPwr-params.TargetOpticalPower) <= powerTolerance
		})
		if !ok {
			t.Errorf("Port %s transceiver %s physical-channel[0] did not report valid input/output power within %v", p.Name(), trName, sampleTimeout)
		}

		var prevFEC uint64
		var hasPrevFEC bool
		_, ok = otnStream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[*oc.TerminalDevice_Channel]) bool {
			ch, present := v.Val()
			if !present || ch == nil || ch.GetOtn() == nil {
				return false
			}
			ber := ch.GetOtn().GetPreFecBer().GetInstant()
			if ber < 0 || ber >= maxPreFECBER {
				return false
			}
			fec := ch.GetOtn().GetFecUncorrectableBlocks()
			if deviations.NonIntervalFecErrorCounter(dut) {
				if !hasPrevFEC {
					prevFEC = fec
					hasPrevFEC = true
					return false
				}
				if fec != prevFEC {
					prevFEC = fec
					return false
				}
				return true
			}
			return fec == 0
		})
		if !ok {
			t.Errorf("Port %s OTN channel %d did not report valid pre-fec-ber (< %v) and zero fec-uncorrectable-blocks within %v", p.Name(), otnIdx, maxPreFECBER, sampleTimeout)
		}
		ochStream.Close()
		trStream.Close()
		otnStream.Close()
	}
}

// VerifyLaserShutoffOutputPower waits for optical-channel output-power/instant to drop below or equal to maxLaserOffPowerdBm
// and verifies the value via gnmi.Lookup.
func VerifyLaserShutoffOutputPower(t *testing.T, dut *ondatra.DUTDevice, ochName string, maxLaserOffPowerdBm float64, timeout time.Duration) {
	t.Helper()
	ochStream := samplestream.New(t, dut, gnmi.OC().Component(ochName).State(), samplingInterval)
	defer ochStream.Close()
	_, ok := ochStream.AwaitNext(timeout, func(v *ygnmi.Value[*oc.Component]) bool {
		c, present := v.Val()
		if !present || c == nil || c.GetOpticalChannel() == nil {
			return false
		}
		return c.GetOpticalChannel().GetOutputPower().GetInstant() <= maxLaserOffPowerdBm
	})
	if !ok {
		t.Errorf("Optical channel %s sample stream timed out waiting for output-power/instant <= %v dBm within %v", ochName, maxLaserOffPowerdBm, timeout)
	}
	gotPower, _ := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OutputPower().Instant().State())
	if gotPower > maxLaserOffPowerdBm {
		t.Errorf("/components/component[name=%s]/optical-channel/state/output-power/instant = %v dBm, want <= %v dBm when transceiver is disabled", ochName, gotPower, maxLaserOffPowerdBm)
	}
}

// VerifyAFTPrefixesConverged uses a single batched gnmi.OCBatch() watch to verify that all IPv4 and IPv6
// static prefixes are installed in the network instance AFT without N+1 sequential RPCs.
func VerifyAFTPrefixesConverged(t *testing.T, dut *ondatra.DUTDevice, niName string, ipv4Prefixes, ipv6Prefixes []string, timeout time.Duration) {
	t.Helper()
	aftBatch := gnmi.OCBatch()
	for _, prefix := range ipv4Prefixes {
		aftBatch.AddPaths(gnmi.OC().NetworkInstance(niName).Afts().Ipv4Entry(prefix))
	}
	for _, prefix := range ipv6Prefixes {
		aftBatch.AddPaths(gnmi.OC().NetworkInstance(niName).Afts().Ipv6Entry(prefix))
	}
	checkAllPresent := func(root *oc.Root) bool {
		if root == nil {
			return false
		}
		afts := root.GetNetworkInstance(niName).GetAfts()
		if afts == nil {
			return false
		}
		for _, prefix := range ipv4Prefixes {
			if afts.GetIpv4Entry(prefix).GetPrefix() != prefix {
				return false
			}
		}
		for _, prefix := range ipv6Prefixes {
			if afts.GetIpv6Entry(prefix).GetPrefix() != prefix {
				return false
			}
		}
		return true
	}
	if root, present := lookupVal(t, dut, aftBatch.State()); present && checkAllPresent(root) {
		return
	}
	_, ok := watchAwait(t, dut, aftBatch.State(), timeout, func(val *ygnmi.Value[*oc.Root]) bool {
		root, present := val.Val()
		return present && checkAllPresent(root)
	})
	if !ok {
		root, _ := lookupVal(t, dut, aftBatch.State())
		var afts *oc.NetworkInstance_Afts
		if root != nil {
			afts = root.GetNetworkInstance(niName).GetAfts()
		}
		for _, prefix := range ipv4Prefixes {
			if afts == nil || afts.GetIpv4Entry(prefix).GetPrefix() != prefix {
				t.Errorf("IPv4 static prefix %s not installed in AFT for network-instance %s", prefix, niName)
			}
		}
		for _, prefix := range ipv6Prefixes {
			if afts == nil || afts.GetIpv6Entry(prefix).GetPrefix() != prefix {
				t.Errorf("IPv6 static prefix %s not installed in AFT for network-instance %s", prefix, niName)
			}
		}
	}
}
