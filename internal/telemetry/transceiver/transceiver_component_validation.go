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

type compSnapshot struct {
	name            string
	compType        oc.Component_Type_Union
	parent          string
	operStatus      oc.E_PlatformTypes_COMPONENT_OPER_STATUS
	serialNo        string
	partNo          string
	mfgName         string
	mfgDate         string
	hardwareVersion string
	firmwareVersion string
	formFactor      oc.E_TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE
	hasEnabled      bool
	enabled         bool
	hasOpMode       bool
	operationalMode uint16
}

type ingressSnapshot struct {
	intf        string
	transceiver string
}

type dutOnChangeStream struct {
	mu              sync.Mutex
	dutID           string
	watchStarted    bool
	components      map[string]compSnapshot
	intfTransceiver map[string]string
	channelIngress  map[uint32]ingressSnapshot
	watchers        map[string]*OnChangeInventoryWatcher
}

var (
	dutStreamsMu sync.Mutex
	dutStreams   = make(map[string]*dutOnChangeStream)
)

func getActiveDUTStream(dutID string) *dutOnChangeStream {
	dutStreamsMu.Lock()
	defer dutStreamsMu.Unlock()
	return dutStreams[dutID]
}

func getOrCreateDUTStream(dutID string) *dutOnChangeStream {
	dutStreamsMu.Lock()
	defer dutStreamsMu.Unlock()
	if s, ok := dutStreams[dutID]; ok {
		return s
	}
	s := &dutOnChangeStream{
		dutID:           dutID,
		components:      make(map[string]compSnapshot),
		intfTransceiver: make(map[string]string),
		channelIngress:  make(map[uint32]ingressSnapshot),
		watchers:        make(map[string]*OnChangeInventoryWatcher),
	}
	dutStreams[dutID] = s
	return s
}

func (s *dutOnChangeStream) hasActiveWatchers() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.watchers) > 0
}

func (s *dutOnChangeStream) updateFromRootLocked(dut *ondatra.DUTDevice, root *oc.Root, checkDeletes bool) {
	if root == nil {
		if checkDeletes {
			for trName, w := range s.watchers {
				w.mu.Lock()
				if w.synced {
					w.violations = append(w.violations, fmt.Sprintf("received gNMI delete or nil component notification for /components/component[name=%s]/state", trName))
				}
				w.mu.Unlock()
			}
		}
		return
	}
	for name, comp := range root.Component {
		if comp == nil {
			continue
		}
		prev := s.components[name]
		if v := comp.GetName(); v != "" {
			prev.name = v
		}
		if v := comp.GetType(); v != nil {
			prev.compType = v
		}
		if v := comp.GetParent(); v != "" {
			prev.parent = v
		}
		if v := comp.GetOperStatus(); v != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
			prev.operStatus = v
		}
		if v := comp.GetSerialNo(); v != "" {
			prev.serialNo = v
		}
		if v := comp.GetPartNo(); v != "" {
			prev.partNo = v
		}
		if v := comp.GetMfgName(); v != "" {
			prev.mfgName = v
		}
		if v := comp.GetMfgDate(); v != "" {
			prev.mfgDate = v
		}
		if v := comp.GetHardwareVersion(); v != "" {
			prev.hardwareVersion = v
		}
		if v := comp.GetFirmwareVersion(); v != "" {
			prev.firmwareVersion = v
		}
		if tr := comp.GetTransceiver(); tr != nil {
			if v := tr.GetFormFactor(); v != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_UNSET {
				prev.formFactor = v
			}
			if tr.Enabled != nil {
				prev.hasEnabled = true
				prev.enabled = tr.GetEnabled()
			}
		}
		if och := comp.GetOpticalChannel(); och != nil && och.OperationalMode != nil {
			prev.hasOpMode = true
			prev.operationalMode = och.GetOperationalMode()
		}
		s.components[name] = prev
	}
	if checkDeletes {
		for name := range s.components {
			if root.GetComponent(name) == nil {
				delete(s.components, name)
			}
		}
	}
	for intfName, intf := range root.Interface {
		if intf != nil && intf.Transceiver != nil {
			s.intfTransceiver[intfName] = intf.GetTransceiver()
		}
	}
	if td := root.GetTerminalDevice(); td != nil {
		for idx, ch := range td.Channel {
			if ch != nil && ch.GetIngress() != nil {
				prevIng := s.channelIngress[idx]
				if ch.GetIngress().Interface != nil {
					prevIng.intf = ch.GetIngress().GetInterface()
				} else if checkDeletes {
					prevIng.intf = ""
				}
				if ch.GetIngress().Transceiver != nil {
					prevIng.transceiver = ch.GetIngress().GetTransceiver()
				} else if checkDeletes {
					prevIng.transceiver = ""
				}
				s.channelIngress[idx] = prevIng
			} else if checkDeletes {
				delete(s.channelIngress, idx)
			}
		}
		if checkDeletes {
			for idx := range s.channelIngress {
				if td.GetChannel(idx) == nil {
					delete(s.channelIngress, idx)
				}
			}
		}
	} else if checkDeletes {
		for idx := range s.channelIngress {
			delete(s.channelIngress, idx)
		}
	}
	for trName, w := range s.watchers {
		w.mu.Lock()
		snap, exists := s.components[trName]
		if !exists {
			if checkDeletes && w.synced {
				w.violations = append(w.violations, fmt.Sprintf("received gNMI delete or nil component notification for /components/component[name=%s]/state", trName))
			}
			w.mu.Unlock()
			continue
		}
		w.checkAndUpdateLocked(dut, snap)
		w.mu.Unlock()
	}
}

func getOrStartDUTStream(t *testing.T, dut *ondatra.DUTDevice) *dutOnChangeStream {
	t.Helper()
	s := getOrCreateDUTStream(dut.ID())
	dutStreamsMu.Lock()
	defer dutStreamsMu.Unlock()
	if s.watchStarted {
		return s
	}
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		t.Fatalf("Unable to connect to gNMI on %s for ON_CHANGE inventory watch: %v", dut.ID(), err)
	}
	invBatch := gnmi.OCBatch()
	invBatch.AddPaths(
		gnmi.OC().ComponentAny().Type().State().PathStruct(),
		gnmi.OC().ComponentAny().Parent().State().PathStruct(),
		gnmi.OC().ComponentAny().OperStatus().State().PathStruct(),
		gnmi.OC().ComponentAny().SerialNo().State().PathStruct(),
		gnmi.OC().ComponentAny().PartNo().State().PathStruct(),
		gnmi.OC().ComponentAny().MfgName().State().PathStruct(),
		gnmi.OC().ComponentAny().HardwareVersion().State().PathStruct(),
		gnmi.OC().ComponentAny().FirmwareVersion().State().PathStruct(),
		gnmi.OC().ComponentAny().Transceiver().FormFactor().State().PathStruct(),
	)
	if !deviations.ComponentMfgDateUnsupported(dut) {
		invBatch.AddPaths(gnmi.OC().ComponentAny().MfgDate().State().PathStruct())
	}
	if !deviations.TransceiverConfigEnableUnsupported(dut) {
		invBatch.AddPaths(gnmi.OC().ComponentAny().Transceiver().Enabled().State().PathStruct())
	}
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		invBatch.AddPaths(
			gnmi.OC().TerminalDevice().ChannelAny().Ingress().Interface().State().PathStruct(),
			gnmi.OC().TerminalDevice().ChannelAny().Ingress().Transceiver().State().PathStruct(),
		)
	}
	ygnmi.Watch(context.Background(), c, invBatch.State(), func(v *ygnmi.Value[*oc.Root]) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		root, present := v.Val()
		if !present || root == nil {
			s.updateFromRootLocked(dut, nil, true)
			return ygnmi.Continue
		}
		s.updateFromRootLocked(dut, root, true)
		return ygnmi.Continue
	}, ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
	s.watchStarted = true
	return s
}

// OnChangeInventoryWatcher maintains a continuous ON_CHANGE gNMI subscription on a transceiver
// component state tree and records any gNMI delete notification or inventory string corruption.
type OnChangeInventoryWatcher struct {
	mu         sync.Mutex
	stream     *dutOnChangeStream
	trName     string
	baseline   *InventorySnapshot
	synced     bool
	violations []string
}

func (w *OnChangeInventoryWatcher) checkAndUpdateLocked(dut *ondatra.DUTDevice, snap compSnapshot) {
	if !w.synced {
		if snap.name != "" && snap.parent != "" &&
			snap.serialNo != "" && snap.partNo != "" && snap.mfgName != "" &&
			snap.hardwareVersion != "" && snap.firmwareVersion != "" &&
			(deviations.ComponentMfgDateUnsupported(dut) || snap.mfgDate != "") {
			w.synced = true
		} else {
			return
		}
	}
	if w.baseline != nil {
		if got := snap.name; got == "" || got != w.baseline.Name {
			w.violations = append(w.violations, fmt.Sprintf("component %s name changed in ON_CHANGE stream: got %q, want %q", w.trName, got, w.baseline.Name))
		}
		if got := snap.compType; got != nil && got != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER {
			w.violations = append(w.violations, fmt.Sprintf("component %s type changed in ON_CHANGE stream: got %v, want %v", w.trName, got, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER))
		}
		if got := snap.parent; got == "" || got != w.baseline.Parent {
			w.violations = append(w.violations, fmt.Sprintf("component %s parent changed in ON_CHANGE stream: got %q, want %q", w.trName, got, w.baseline.Parent))
		}
		if got := snap.serialNo; got == "" || got != w.baseline.SerialNo {
			w.violations = append(w.violations, fmt.Sprintf("component %s serial-no changed in ON_CHANGE stream: got %q, want %q", w.trName, got, w.baseline.SerialNo))
		}
		if got := snap.partNo; got == "" || got != w.baseline.PartNo {
			w.violations = append(w.violations, fmt.Sprintf("component %s part-no changed in ON_CHANGE stream: got %q, want %q", w.trName, got, w.baseline.PartNo))
		}
		if got := snap.mfgName; got == "" || got != w.baseline.MfgName {
			w.violations = append(w.violations, fmt.Sprintf("component %s mfg-name changed in ON_CHANGE stream: got %q, want %q", w.trName, got, w.baseline.MfgName))
		}
		if !deviations.ComponentMfgDateUnsupported(dut) {
			if got := snap.mfgDate; got == "" || got != w.baseline.MfgDate {
				w.violations = append(w.violations, fmt.Sprintf("component %s mfg-date changed in ON_CHANGE stream: got %q, want %q", w.trName, got, w.baseline.MfgDate))
			}
		}
		if got := snap.hardwareVersion; got == "" || got != w.baseline.HardwareVersion {
			w.violations = append(w.violations, fmt.Sprintf("component %s hardware-version changed in ON_CHANGE stream: got %q, want %q", w.trName, got, w.baseline.HardwareVersion))
		}
		if got := snap.firmwareVersion; got == "" || got != w.baseline.FirmwareVersion {
			w.violations = append(w.violations, fmt.Sprintf("component %s firmware-version changed in ON_CHANGE stream: got %q, want %q", w.trName, got, w.baseline.FirmwareVersion))
		}
	}
}

// StartOnChangeInventoryWatcher starts a continuous ON_CHANGE gNMI subscription on
// /components/component[name=<trName>]/state inventory leaves that monitors for delete notifications or inventory mismatches.
func StartOnChangeInventoryWatcher(t *testing.T, dut *ondatra.DUTDevice, trName string, baseline *InventorySnapshot) *OnChangeInventoryWatcher {
	t.Helper()
	s := getOrStartDUTStream(t, dut)
	w := &OnChangeInventoryWatcher{
		stream:   s,
		trName:   trName,
		baseline: baseline,
	}
	s.mu.Lock()
	s.watchers[trName] = w
	if snap, exists := s.components[trName]; exists {
		w.mu.Lock()
		w.checkAndUpdateLocked(dut, snap)
		w.mu.Unlock()
	}
	s.mu.Unlock()
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

// Close unregisters the watcher from the continuous ON_CHANGE gNMI subscription.
func (w *OnChangeInventoryWatcher) Close() {
	if w != nil && w.stream != nil {
		w.stream.mu.Lock()
		if w.stream.watchers[w.trName] == w {
			delete(w.stream.watchers, w.trName)
		}
		w.stream.mu.Unlock()
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
		gnmi.OC().Component(trName).Type().State().PathStruct(),
		gnmi.OC().Component(trName).Parent().State().PathStruct(),
		gnmi.OC().Component(trName).OperStatus().State().PathStruct(),
		gnmi.OC().Component(trName).SerialNo().State().PathStruct(),
		gnmi.OC().Component(trName).PartNo().State().PathStruct(),
		gnmi.OC().Component(trName).MfgName().State().PathStruct(),
		gnmi.OC().Component(trName).HardwareVersion().State().PathStruct(),
		gnmi.OC().Component(trName).FirmwareVersion().State().PathStruct(),
		gnmi.OC().Component(trName).Transceiver().FormFactor().State().PathStruct(),
	)
	if !deviations.ComponentMfgDateUnsupported(dut) {
		b.AddPaths(gnmi.OC().Component(trName).MfgDate().State().PathStruct())
	}
	return b.State()
}

// componentBasicQuery returns a batched query for a component's basic identity leaves (name, type, parent).
func componentBasicQuery(compName string) ygnmi.SingletonQuery[*oc.Root] {
	b := gnmi.OCBatch()
	b.AddPaths(
		gnmi.OC().Component(compName).Type().State().PathStruct(),
		gnmi.OC().Component(compName).Parent().State().PathStruct(),
	)
	return b.State()
}

// lookupInterfaceTransceivers queries /interfaces/interface[name=p]/state/transceiver as individual string leaves
// so ygot does not reject the response due to *oc.Root leafref cross-validation.
func lookupInterfaceTransceivers(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port) map[string]string {
	t.Helper()
	res := make(map[string]string, len(ports))
	if s := getActiveDUTStream(dut.ID()); s != nil {
		s.mu.Lock()
		allFound := true
		for _, p := range ports {
			if tr := s.intfTransceiver[p.Name()]; tr != "" {
				res[p.Name()] = tr
			} else {
				allFound = false
			}
		}
		s.mu.Unlock()
		if allFound {
			return res
		}
	}
	for _, p := range ports {
		if res[p.Name()] != "" {
			continue
		}
		if tr, ok := lookupVal(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()); ok && tr != "" {
			res[p.Name()] = tr
		}
	}
	return res
}

// CaptureAndValidateBaselineInventory waits for the transceiver component to be ACTIVE with populated
// inventory strings via ON_CHANGE gNMI subscription (gnmi.Watch) and gnmi.Lookup, validates all required
// PORT, TRANSCEIVER, and OPTICAL_CHANNEL component state leaves and leafref paths, and returns the captured baseline snapshot.
func CaptureAndValidateBaselineInventory(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, watchTimeout time.Duration) *InventorySnapshot {
	t.Helper()
	return CaptureAndValidateMultiPortBaselineInventory(t, dut, []*ondatra.Port{p}, params, watchTimeout)[p.Name()]
}

// CaptureAndValidateMultiPortBaselineInventory captures and validates baseline transceiver inventory across multiple ports
// using a single batched gNMI Lookup, falling back to per-port watch if any transceiver is not yet ACTIVE.
func CaptureAndValidateMultiPortBaselineInventory(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port, params *cfgplugins.ConfigParameters, watchTimeout time.Duration) map[string]*InventorySnapshot {
	t.Helper()
	fullBatch := gnmi.OCBatch()
	fullBatch.AddPaths(
		gnmi.OC().ComponentAny().Type().State().PathStruct(),
		gnmi.OC().ComponentAny().Parent().State().PathStruct(),
		gnmi.OC().ComponentAny().OperStatus().State().PathStruct(),
		gnmi.OC().ComponentAny().SerialNo().State().PathStruct(),
		gnmi.OC().ComponentAny().PartNo().State().PathStruct(),
		gnmi.OC().ComponentAny().MfgName().State().PathStruct(),
		gnmi.OC().ComponentAny().HardwareVersion().State().PathStruct(),
		gnmi.OC().ComponentAny().FirmwareVersion().State().PathStruct(),
		gnmi.OC().ComponentAny().Transceiver().FormFactor().State().PathStruct(),
	)
	if !deviations.ComponentMfgDateUnsupported(dut) {
		fullBatch.AddPaths(gnmi.OC().ComponentAny().MfgDate().State().PathStruct())
	}
	if !deviations.TransceiverConfigEnableUnsupported(dut) {
		fullBatch.AddPaths(gnmi.OC().ComponentAny().Transceiver().Enabled().State().PathStruct())
	}
	if !deviations.OperationalModeUnsupported(dut) {
		fullBatch.AddPaths(gnmi.OC().ComponentAny().OpticalChannel().OperationalMode().State().PathStruct())
	}
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		for _, p := range ports {
			ethIdx := params.ETHIndexes[p.Name()]
			fullBatch.AddPaths(
				gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State().PathStruct(),
				gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State().PathStruct(),
			)
		}
	}
	fullQuery := fullBatch.State()
	root, present := lookupVal(t, dut, fullQuery)
	if root != nil {
		for intfName, tr := range lookupInterfaceTransceivers(t, dut, ports) {
			root.GetOrCreateInterface(intfName).SetTransceiver(tr)
		}
	}
	if s := getOrCreateDUTStream(dut.ID()); s != nil && present && root != nil {
		s.mu.Lock()
		s.updateFromRootLocked(dut, root, false)
		s.mu.Unlock()
		getOrStartDUTStream(t, dut)
	}
	snapshots := make(map[string]*InventorySnapshot, len(ports))
	for _, p := range ports {
		snapshots[p.Name()] = validateBaselineInventoryFromRoot(t, dut, p, params, watchTimeout, fullQuery, root, present)
	}
	return snapshots
}

func validateBaselineInventoryFromRoot(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, watchTimeout time.Duration, fullQuery ygnmi.SingletonQuery[*oc.Root], root *oc.Root, present bool) *InventorySnapshot {
	t.Helper()
	trName := params.TransceiverNames[p.Name()]
	hwPortName := params.HWPortNames[p.Name()]
	ochName := params.OpticalChannelNames[p.Name()]
	ethIdx := params.ETHIndexes[p.Name()]

	invQuery := componentInventoryQuery(dut, trName)
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
			r, _ := lookupVal(t, dut, fullQuery)
			root = r
			comp = r.GetComponent(trName)
		}
	}
	if comp == nil {
		t.Fatalf("Transceiver %s on port %s state is nil", trName, p.Name())
	}

	// Validate PORT component state (name, type, parent).
	hwComp := root.GetComponent(hwPortName)
	if hwComp == nil {
		hwRoot, _ := lookupVal(t, dut, componentBasicQuery(hwPortName))
		hwComp = hwRoot.GetComponent(hwPortName)
	}
	if hwComp == nil {
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
	ochComp := root.GetComponent(ochName)
	if ochComp == nil {
		ochRoot, _ := lookupVal(t, dut, componentBasicQuery(ochName))
		ochComp = ochRoot.GetComponent(ochName)
	}
	if ochComp == nil {
		t.Errorf("Port %s optical-channel component %s missing from /components/component", p.Name(), ochName)
	} else {
		if got, want := ochComp.GetName(), ochName; got != want {
			t.Errorf("Port %s /components/component[name=%s]/state/name = %q, want %q", p.Name(), ochName, got, want)
		}
		if got, want := ochComp.GetType(), oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_OPTICAL_CHANNEL; got != nil && got != want {
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
		enabledVal := comp.GetTransceiver().GetEnabled()
		if !enabledVal {
			if got, ok := lookupVal(t, dut, gnmi.OC().Component(trName).Transceiver().Enabled().State()); !ok || !got {
				t.Errorf("Port %s /components/component[name=%s]/transceiver/state/enabled = %v, want true", p.Name(), trName, got)
			}
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

	// Validate interface and ETH logical channel leafref bindings via batched result or Lookup/Watch fallback.
	if got := root.GetInterface(p.Name()).GetTransceiver(); got != trName {
		if got, ok := lookupVal(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()); !ok || got != trName {
			_, ok := watchAwait(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State(), watchTimeout, func(val *ygnmi.Value[string]) bool {
				v, present := val.Val()
				return present && v == trName
			})
			if !ok {
				got, _ := lookupVal(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
				if got != trName {
					t.Errorf("Port %s /interfaces/interface[name=%s]/state/transceiver = %q, want %q", p.Name(), p.Name(), got, trName)
				}
			}
		}
	}
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		ing := root.GetTerminalDevice().GetChannel(ethIdx).GetIngress()
		if got := ing.GetInterface(); got != p.Name() {
			if got, _ := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State()); got != p.Name() {
				t.Errorf("Port %s /terminal-device/logical-channels/channel[index=%d]/ingress/state/interface = %q, want %q", p.Name(), ethIdx, got, p.Name())
			}
		}
		if got := ing.GetTransceiver(); got != trName {
			if got, _ := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State()); got != trName {
				t.Errorf("Port %s /terminal-device/logical-channels/channel[index=%d]/ingress/state/transceiver = %q, want %q", p.Name(), ethIdx, got, trName)
			}
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
	if s := getActiveDUTStream(dut.ID()); s != nil && s.hasActiveWatchers() {
		VerifyMultiPortInventoryMatchesBaseline(t, dut, []*ondatra.Port{p}, params, map[string]*InventorySnapshot{p.Name(): baseline}, expectInterfaceBound, expectActive, expectEnabled)
		return
	}
	trName := params.TransceiverNames[p.Name()]
	ethIdx := params.ETHIndexes[p.Name()]
	if s := getActiveDUTStream(dut.ID()); s != nil && (!expectInterfaceBound || !expectActive || !expectEnabled) {
		s.mu.Lock()
		if !expectInterfaceBound {
			delete(s.intfTransceiver, p.Name())
			delete(s.channelIngress, ethIdx)
		}
		if ochName := params.OpticalChannelNames[p.Name()]; ochName != "" {
			ochSnap := s.components[ochName]
			ochSnap.hasOpMode = false
			s.components[ochName] = ochSnap
		}
		s.mu.Unlock()
	}

	// Verify via gNMI Lookup (and ON_CHANGE Watch fallback if not yet converged) that the component is present,
	// inventory strings are non-empty, and (if expectActive is true) oper-status is ACTIVE.
	fullBatch := gnmi.OCBatch()
	fullBatch.AddPaths(
		gnmi.OC().ComponentAny().Type().State().PathStruct(),
		gnmi.OC().ComponentAny().Parent().State().PathStruct(),
		gnmi.OC().ComponentAny().OperStatus().State().PathStruct(),
		gnmi.OC().ComponentAny().SerialNo().State().PathStruct(),
		gnmi.OC().ComponentAny().PartNo().State().PathStruct(),
		gnmi.OC().ComponentAny().MfgName().State().PathStruct(),
		gnmi.OC().ComponentAny().HardwareVersion().State().PathStruct(),
		gnmi.OC().ComponentAny().FirmwareVersion().State().PathStruct(),
		gnmi.OC().ComponentAny().Transceiver().FormFactor().State().PathStruct(),
	)
	if !deviations.ComponentMfgDateUnsupported(dut) {
		fullBatch.AddPaths(gnmi.OC().ComponentAny().MfgDate().State().PathStruct())
	}
	if !deviations.TransceiverConfigEnableUnsupported(dut) {
		fullBatch.AddPaths(gnmi.OC().ComponentAny().Transceiver().Enabled().State().PathStruct())
	}
	if expectInterfaceBound && !deviations.EthChannelIngressParametersUnsupported(dut) {
		fullBatch.AddPaths(
			gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State().PathStruct(),
			gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State().PathStruct(),
		)
	}
	invQuery := componentInventoryQuery(dut, trName)
	rootVal, present := lookupVal(t, dut, fullBatch.State())
	if expectInterfaceBound && rootVal != nil {
		for intfName, tr := range lookupInterfaceTransceivers(t, dut, []*ondatra.Port{p}) {
			rootVal.GetOrCreateInterface(intfName).SetTransceiver(tr)
		}
	}
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
		rootVal, present = lookupVal(t, dut, fullBatch.State())
		if expectInterfaceBound && rootVal != nil {
			for intfName, tr := range lookupInterfaceTransceivers(t, dut, []*ondatra.Port{p}) {
				rootVal.GetOrCreateInterface(intfName).SetTransceiver(tr)
			}
		}
		compVal = rootVal.GetComponent(trName)
	}
	if s := getActiveDUTStream(dut.ID()); s != nil && present && rootVal != nil {
		s.mu.Lock()
		s.updateFromRootLocked(dut, rootVal, false)
		s.mu.Unlock()
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
		if compVal.GetTransceiver().GetEnabled() != expectEnabled {
			if got, ok := lookupVal(t, dut, gnmi.OC().Component(trName).Transceiver().Enabled().State()); !ok || got != expectEnabled {
				t.Errorf("Port %s /components/component[name=%s]/transceiver/state/enabled = %v, want %v", p.Name(), trName, got, expectEnabled)
			}
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
		if got := rootVal.GetInterface(p.Name()).GetTransceiver(); got != trName {
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
		}
		if !deviations.EthChannelIngressParametersUnsupported(dut) {
			ing := rootVal.GetTerminalDevice().GetChannel(ethIdx).GetIngress()
			if got := ing.GetInterface(); got != p.Name() {
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
			}
			if got := ing.GetTransceiver(); got != trName {
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
}

// VerifyMultiPortInventoryMatchesBaseline validates transceiver inventory across multiple ports using a single
// batched gNMI Lookup (or the active ON_CHANGE subscription when present), falling back to per-port watch if needed.
func VerifyMultiPortInventoryMatchesBaseline(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port, params *cfgplugins.ConfigParameters, baselines map[string]*InventorySnapshot, expectInterfaceBound bool, expectActive bool, expectEnabled bool) {
	t.Helper()
	for _, p := range ports {
		if baselines[p.Name()] == nil {
			baselines[p.Name()] = CaptureAndValidateBaselineInventory(t, dut, p, params, 30*time.Second)
		}
	}
	checkOpMode := expectInterfaceBound && expectActive && expectEnabled && !deviations.OperationalModeUnsupported(dut) && params.OperationalMode != 0

	if s := getActiveDUTStream(dut.ID()); s != nil {
		if !expectInterfaceBound || !expectActive || !expectEnabled {
			s.mu.Lock()
			for _, p := range ports {
				if !expectInterfaceBound {
					delete(s.channelIngress, params.ETHIndexes[p.Name()])
				}
				if ochName := params.OpticalChannelNames[p.Name()]; ochName != "" {
					ochSnap := s.components[ochName]
					ochSnap.hasOpMode = false
					s.components[ochName] = ochSnap
				}
			}
			s.mu.Unlock()
		}
		if s.watchStarted || s.hasActiveWatchers() {
			deadline := time.Now().Add(3 * time.Second)
			var allCompReady bool
			for {
				s.mu.Lock()
				allCompReady = true
				for _, p := range ports {
					trName := params.TransceiverNames[p.Name()]
					snap, exists := s.components[trName]
					if !exists || snap.serialNo == "" || snap.partNo == "" || snap.mfgName == "" ||
						snap.hardwareVersion == "" || snap.firmwareVersion == "" ||
						(!deviations.ComponentMfgDateUnsupported(dut) && snap.mfgDate == "") ||
						(expectActive && snap.operStatus != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE) ||
						(!deviations.TransceiverConfigEnableUnsupported(dut) && (!snap.hasEnabled || snap.enabled != expectEnabled)) {
						allCompReady = false
						break
					}
				}
				s.mu.Unlock()
				if allCompReady || time.Now().After(deadline) {
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
			if allCompReady {
				var needOpModeLookup bool
				if expectInterfaceBound {
					s.mu.Lock()
					allIntfTrReady := true
					allIngressReady := true
					for _, p := range ports {
						trName := params.TransceiverNames[p.Name()]
						ethIdx := params.ETHIndexes[p.Name()]
						if s.intfTransceiver[p.Name()] != trName {
							allIntfTrReady = false
						}
						if !deviations.EthChannelIngressParametersUnsupported(dut) {
							ing := s.channelIngress[ethIdx]
							if ing.intf != p.Name() || ing.transceiver != trName {
								allIngressReady = false
							}
						}
						if checkOpMode {
							if ochName := params.OpticalChannelNames[p.Name()]; ochName != "" {
								ochSnap := s.components[ochName]
								if !ochSnap.hasOpMode || ochSnap.operationalMode != params.OperationalMode {
									needOpModeLookup = true
								}
							}
						}
					}
					s.mu.Unlock()
					if !allIntfTrReady {
						intfTrMap := lookupInterfaceTransceivers(t, dut, ports)
						s.mu.Lock()
						for intfName, tr := range intfTrMap {
							s.intfTransceiver[intfName] = tr
						}
						s.mu.Unlock()
					}
					if !allIngressReady && !deviations.EthChannelIngressParametersUnsupported(dut) {
						for _, p := range ports {
							ethIdx := params.ETHIndexes[p.Name()]
							intf, ok1 := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State())
							tr, ok2 := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State())
							if ok1 || ok2 {
								s.mu.Lock()
								ing := s.channelIngress[ethIdx]
								if ok1 && intf != "" {
									ing.intf = intf
								}
								if ok2 && tr != "" {
									ing.transceiver = tr
								}
								s.channelIngress[ethIdx] = ing
								s.mu.Unlock()
							}
						}
					}
					if needOpModeLookup {
						opModeBatch := gnmi.OCBatch()
						opModeBatch.AddPaths(gnmi.OC().ComponentAny().OpticalChannel().OperationalMode().State().PathStruct())
						if opRoot, ok := lookupVal(t, dut, opModeBatch.State()); ok && opRoot != nil {
							s.mu.Lock()
							for _, p := range ports {
								if ochName := params.OpticalChannelNames[p.Name()]; ochName != "" {
									if och := opRoot.GetComponent(ochName).GetOpticalChannel(); och != nil && och.OperationalMode != nil {
										ochSnap := s.components[ochName]
										ochSnap.hasOpMode = true
										ochSnap.operationalMode = och.GetOperationalMode()
										s.components[ochName] = ochSnap
									}
								}
							}
							s.mu.Unlock()
						} else {
							for _, p := range ports {
								if ochName := params.OpticalChannelNames[p.Name()]; ochName != "" {
									if opMode, ok := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State()); ok {
										s.mu.Lock()
										ochSnap := s.components[ochName]
										ochSnap.hasOpMode = true
										ochSnap.operationalMode = opMode
										s.components[ochName] = ochSnap
										s.mu.Unlock()
									}
								}
							}
						}
					}
				}
				for _, p := range ports {
					baseline := baselines[p.Name()]
					trName := params.TransceiverNames[p.Name()]
					ethIdx := params.ETHIndexes[p.Name()]
					s.mu.Lock()
					snap := s.components[trName]
					gotIntfTr := s.intfTransceiver[p.Name()]
					gotIng := s.channelIngress[ethIdx]
					s.mu.Unlock()

					if got, want := snap.name, baseline.Name; got != want {
						t.Errorf("Port %s /components/component[name=%s]/state/name = %q, want %q", p.Name(), trName, got, want)
					}
					if got, want := snap.compType, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER; got != want {
						t.Errorf("Port %s /components/component[name=%s]/state/type = %v, want %v", p.Name(), trName, got, want)
					}
					if got, want := snap.parent, baseline.Parent; got != want {
						t.Errorf("Port %s /components/component[name=%s]/state/parent = %q, want %q", p.Name(), trName, got, want)
					}
					if baseline.FormFactor != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_UNSET {
						if got, want := snap.formFactor, baseline.FormFactor; got != want {
							t.Errorf("Port %s /components/component[name=%s]/transceiver/state/form-factor = %v, want %v", p.Name(), trName, got, want)
						}
					}
					if got, want := snap.serialNo, baseline.SerialNo; got != want {
						t.Errorf("Port %s /components/component[name=%s]/state/serial-no = %q, want %q", p.Name(), trName, got, want)
					}
					if got, want := snap.partNo, baseline.PartNo; got != want {
						t.Errorf("Port %s /components/component[name=%s]/state/part-no = %q, want %q", p.Name(), trName, got, want)
					}
					if got, want := snap.mfgName, baseline.MfgName; got != want {
						t.Errorf("Port %s /components/component[name=%s]/state/mfg-name = %q, want %q", p.Name(), trName, got, want)
					}
					if !deviations.ComponentMfgDateUnsupported(dut) {
						if got, want := snap.mfgDate, baseline.MfgDate; got != want {
							t.Errorf("Port %s /components/component[name=%s]/state/mfg-date = %q, want %q", p.Name(), trName, got, want)
						}
					}
					if got, want := snap.hardwareVersion, baseline.HardwareVersion; got != want {
						t.Errorf("Port %s /components/component[name=%s]/state/hardware-version = %q, want %q", p.Name(), trName, got, want)
					}
					if got, want := snap.firmwareVersion, baseline.FirmwareVersion; got != want {
						t.Errorf("Port %s /components/component[name=%s]/state/firmware-version = %q, want %q", p.Name(), trName, got, want)
					}
					if expectInterfaceBound {
						if gotIntfTr != trName {
							if got, ok := lookupVal(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()); !ok || got != trName {
								t.Errorf("Port %s /interfaces/interface[name=%s]/state/transceiver = %q, want %q", p.Name(), p.Name(), got, trName)
							}
						}
						if !deviations.EthChannelIngressParametersUnsupported(dut) {
							if gotIng.intf != p.Name() {
								if got, ok := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State()); !ok || got != p.Name() {
									t.Errorf("Port %s /terminal-device/logical-channels/channel[index=%d]/ingress/state/interface = %q, want %q", p.Name(), ethIdx, got, p.Name())
								}
							}
							if gotIng.transceiver != trName {
								if got, ok := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State()); !ok || got != trName {
									t.Errorf("Port %s /terminal-device/logical-channels/channel[index=%d]/ingress/state/transceiver = %q, want %q", p.Name(), ethIdx, got, trName)
								}
							}
						}
					}
				}
				if checkOpMode {
					for _, p := range ports {
						ochName := params.OpticalChannelNames[p.Name()]
						if ochName == "" {
							continue
						}
						s.mu.Lock()
						ochSnap := s.components[ochName]
						s.mu.Unlock()
						modeOK := ochSnap.hasOpMode && ochSnap.operationalMode == params.OperationalMode
						if !modeOK {
							_, modeOK = watchAwait(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State(), 30*time.Second, func(val *ygnmi.Value[uint16]) bool {
								v, ok := val.Val()
								return ok && v == params.OperationalMode
							})
						}
						if !modeOK {
							gotMode, _ := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State())
							if gotMode != params.OperationalMode {
								t.Errorf("Port %s /components/component[name=%s]/optical-channel/state/operational-mode = %d, want %d", p.Name(), ochName, gotMode, params.OperationalMode)
							} else {
								modeOK = true
							}
						}
						if modeOK {
							s.mu.Lock()
							ochSnap := s.components[ochName]
							ochSnap.hasOpMode = true
							ochSnap.operationalMode = params.OperationalMode
							s.components[ochName] = ochSnap
							s.mu.Unlock()
						}
					}
				}
				return
			}
		}
	}

	batch := gnmi.OCBatch()
	batch.AddPaths(
		gnmi.OC().ComponentAny().Type().State().PathStruct(),
		gnmi.OC().ComponentAny().Parent().State().PathStruct(),
		gnmi.OC().ComponentAny().OperStatus().State().PathStruct(),
		gnmi.OC().ComponentAny().SerialNo().State().PathStruct(),
		gnmi.OC().ComponentAny().PartNo().State().PathStruct(),
		gnmi.OC().ComponentAny().MfgName().State().PathStruct(),
		gnmi.OC().ComponentAny().HardwareVersion().State().PathStruct(),
		gnmi.OC().ComponentAny().FirmwareVersion().State().PathStruct(),
		gnmi.OC().ComponentAny().Transceiver().FormFactor().State().PathStruct(),
	)
	if !deviations.ComponentMfgDateUnsupported(dut) {
		batch.AddPaths(gnmi.OC().ComponentAny().MfgDate().State().PathStruct())
	}
	if !deviations.TransceiverConfigEnableUnsupported(dut) {
		batch.AddPaths(gnmi.OC().ComponentAny().Transceiver().Enabled().State().PathStruct())
	}
	s := getOrCreateDUTStream(dut.ID())
	needSlowOpModeLookup := false
	if expectInterfaceBound {
		if checkOpMode {
			s.mu.Lock()
			for _, p := range ports {
				if ochName := params.OpticalChannelNames[p.Name()]; ochName != "" {
					ochSnap := s.components[ochName]
					if !ochSnap.hasOpMode || ochSnap.operationalMode != params.OperationalMode {
						needSlowOpModeLookup = true
						break
					}
				}
			}
			s.mu.Unlock()
			if needSlowOpModeLookup {
				batch.AddPaths(gnmi.OC().ComponentAny().OpticalChannel().OperationalMode().State().PathStruct())
			}
		}
		if !deviations.EthChannelIngressParametersUnsupported(dut) {
			for _, p := range ports {
				ethIdx := params.ETHIndexes[p.Name()]
				batch.AddPaths(
					gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Interface().State().PathStruct(),
					gnmi.OC().TerminalDevice().Channel(ethIdx).Ingress().Transceiver().State().PathStruct(),
				)
			}
		}
	}
	rootVal, present := lookupVal(t, dut, batch.State())
	if expectInterfaceBound && rootVal != nil {
		for intfName, tr := range lookupInterfaceTransceivers(t, dut, ports) {
			rootVal.GetOrCreateInterface(intfName).SetTransceiver(tr)
		}
	}
	if s != nil && present && rootVal != nil {
		s.mu.Lock()
		s.updateFromRootLocked(dut, rootVal, false)
		s.mu.Unlock()
	}
	for _, p := range ports {
		baseline := baselines[p.Name()]
		trName := params.TransceiverNames[p.Name()]
		ochName := params.OpticalChannelNames[p.Name()]
		ethIdx := params.ETHIndexes[p.Name()]
		compVal := rootVal.GetComponent(trName)
		ready := present && compVal != nil &&
			(!expectActive || compVal.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE) &&
			compVal.GetSerialNo() != "" && compVal.GetPartNo() != "" && compVal.GetMfgName() != "" &&
			compVal.GetHardwareVersion() != "" && compVal.GetFirmwareVersion() != "" &&
			(deviations.ComponentMfgDateUnsupported(dut) || compVal.GetMfgDate() != "") &&
			(deviations.TransceiverConfigEnableUnsupported(dut) || compVal.GetTransceiver().GetEnabled() == expectEnabled) &&
			(!expectInterfaceBound || rootVal.GetInterface(p.Name()).GetTransceiver() == trName) &&
			(!expectInterfaceBound || deviations.EthChannelIngressParametersUnsupported(dut) ||
				(rootVal.GetTerminalDevice().GetChannel(ethIdx).GetIngress().GetInterface() == p.Name() &&
					rootVal.GetTerminalDevice().GetChannel(ethIdx).GetIngress().GetTransceiver() == trName))
		if !ready {
			VerifyInventoryMatchesBaseline(t, dut, p, params, baseline, expectInterfaceBound, expectActive, expectEnabled)
		} else {
			if got, want := compVal.GetName(), baseline.Name; got != want {
				t.Errorf("Port %s /components/component[name=%s]/state/name = %q, want %q", p.Name(), trName, got, want)
			}
			if got, want := compVal.GetType(), oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER; got != want {
				t.Errorf("Port %s /components/component[name=%s]/state/type = %v, want %v", p.Name(), trName, got, want)
			}
			if got, want := compVal.GetParent(), baseline.Parent; got != want {
				t.Errorf("Port %s /components/component[name=%s]/state/parent = %q, want %q", p.Name(), trName, got, want)
			}
			if baseline.FormFactor != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_UNSET {
				if got, want := compVal.GetTransceiver().GetFormFactor(), baseline.FormFactor; got != want {
					t.Errorf("Port %s /components/component[name=%s]/transceiver/state/form-factor = %v, want %v", p.Name(), trName, got, want)
				}
			}
			if got, want := compVal.GetSerialNo(), baseline.SerialNo; got != want {
				t.Errorf("Port %s /components/component[name=%s]/state/serial-no = %q, want %q", p.Name(), trName, got, want)
			}
			if got, want := compVal.GetPartNo(), baseline.PartNo; got != want {
				t.Errorf("Port %s /components/component[name=%s]/state/part-no = %q, want %q", p.Name(), trName, got, want)
			}
			if got, want := compVal.GetMfgName(), baseline.MfgName; got != want {
				t.Errorf("Port %s /components/component[name=%s]/state/mfg-name = %q, want %q", p.Name(), trName, got, want)
			}
			if !deviations.ComponentMfgDateUnsupported(dut) {
				if got, want := compVal.GetMfgDate(), baseline.MfgDate; got != want {
					t.Errorf("Port %s /components/component[name=%s]/state/mfg-date = %q, want %q", p.Name(), trName, got, want)
				}
			}
			if got, want := compVal.GetHardwareVersion(), baseline.HardwareVersion; got != want {
				t.Errorf("Port %s /components/component[name=%s]/state/hardware-version = %q, want %q", p.Name(), trName, got, want)
			}
			if got, want := compVal.GetFirmwareVersion(), baseline.FirmwareVersion; got != want {
				t.Errorf("Port %s /components/component[name=%s]/state/firmware-version = %q, want %q", p.Name(), trName, got, want)
			}
		}
		if checkOpMode && ochName != "" && needSlowOpModeLookup {
			modeOK := present && rootVal.GetComponent(ochName).GetOpticalChannel().GetOperationalMode() == params.OperationalMode
			if !modeOK {
				_, modeOK = watchAwait(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State(), 30*time.Second, func(val *ygnmi.Value[uint16]) bool {
					v, ok := val.Val()
					return ok && v == params.OperationalMode
				})
			}
			if !modeOK {
				gotMode, _ := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State())
				if gotMode != params.OperationalMode {
					t.Errorf("Port %s /components/component[name=%s]/optical-channel/state/operational-mode = %d, want %d", p.Name(), ochName, gotMode, params.OperationalMode)
				} else {
					modeOK = true
				}
			}
			if modeOK && s != nil {
				s.mu.Lock()
				ochSnap := s.components[ochName]
				ochSnap.hasOpMode = true
				ochSnap.operationalMode = params.OperationalMode
				s.components[ochName] = ochSnap
				s.mu.Unlock()
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
		_, ok := watchAwait(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), convergenceTimeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			v, present := val.Val()
			return present && v == oc.Interface_OperStatus_UP
		})
		if !ok {
			t.Fatalf("Interface %s did not reach OperStatus UP within %v", p.Name(), convergenceTimeout)
		}
	}

	otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
	for _, p := range ports {
		otnIdx := params.OTNIndexes[p.Name()]
		s := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIdx).State(), samplingInterval)
		defer s.Close()
		otnStreams[p.Name()] = s
	}

	stateBatch := gnmi.OCBatch()
	stateBatch.AddPaths(
		gnmi.OC().TerminalDevice().ChannelAny().Index().State().PathStruct(),
		gnmi.OC().ComponentAny().OpticalChannel().Frequency().State().PathStruct(),
		gnmi.OC().ComponentAny().OpticalChannel().TargetOutputPower().State().PathStruct(),
	)
	if !deviations.TerminalDeviceChannelAdminStateUnsupported(dut) {
		stateBatch.AddPaths(gnmi.OC().TerminalDevice().ChannelAny().AdminState().State().PathStruct())
	}
	if !deviations.OperationalModeUnsupported(dut) {
		stateBatch.AddPaths(gnmi.OC().ComponentAny().OpticalChannel().OperationalMode().State().PathStruct())
	}
	stateRoot, _ := lookupVal(t, dut, stateBatch.State())

	for _, p := range ports {
		otnIdx := params.OTNIndexes[p.Name()]
		ethIdx := params.ETHIndexes[p.Name()]
		for _, chIdx := range []uint32{otnIdx, ethIdx} {
			ch := stateRoot.GetTerminalDevice().GetChannel(chIdx)
			if ch == nil || ch.GetIndex() != chIdx {
				_, ok := watchAwait(t, dut, gnmi.OC().TerminalDevice().Channel(chIdx).Index().State(), sampleTimeout, func(val *ygnmi.Value[uint32]) bool {
					v, present := val.Val()
					return present && v == chIdx
				})
				if !ok {
					t.Errorf("Port %s logical channel %d did not report valid index within %v", p.Name(), chIdx, sampleTimeout)
				}
			}
			if !deviations.TerminalDeviceChannelAdminStateUnsupported(dut) && (ch == nil || ch.GetAdminState() != oc.TerminalDevice_AdminStateType_ENABLED) {
				_, ok := watchAwait(t, dut, gnmi.OC().TerminalDevice().Channel(chIdx).AdminState().State(), sampleTimeout, func(val *ygnmi.Value[oc.E_TerminalDevice_AdminStateType]) bool {
					v, present := val.Val()
					return present && v == oc.TerminalDevice_AdminStateType_ENABLED
				})
				if !ok {
					t.Errorf("Port %s logical channel %d did not report admin-state ENABLED within %v", p.Name(), chIdx, sampleTimeout)
				}
			}
		}
	}

	for _, p := range ports {
		ochName := params.OpticalChannelNames[p.Name()]
		och := stateRoot.GetComponent(ochName).GetOpticalChannel()

		if !deviations.OperationalModeUnsupported(dut) {
			modeOK := och != nil && och.GetOperationalMode() == wantOpMode
			if !modeOK {
				_, modeOK = watchAwait(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State(), sampleTimeout, func(val *ygnmi.Value[uint16]) bool {
					v, present := val.Val()
					return present && v == wantOpMode
				})
				if !modeOK {
					gotMode, _ := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().OperationalMode().State())
					if gotMode != wantOpMode {
						t.Errorf("Port %s /components/component[name=%s]/optical-channel/state/operational-mode = %d, want %d", p.Name(), ochName, gotMode, wantOpMode)
					} else {
						modeOK = true
					}
				}
			}
			if modeOK {
				if s := getOrCreateDUTStream(dut.ID()); s != nil {
					s.mu.Lock()
					ochSnap := s.components[ochName]
					ochSnap.hasOpMode = true
					ochSnap.operationalMode = wantOpMode
					s.components[ochName] = ochSnap
					s.mu.Unlock()
				}
			}
		}

		if och == nil || och.GetFrequency() != wantFreq {
			_, ok := watchAwait(t, dut, gnmi.OC().Component(ochName).OpticalChannel().Frequency().State(), sampleTimeout, func(val *ygnmi.Value[uint64]) bool {
				v, present := val.Val()
				return present && v == wantFreq
			})
			if !ok {
				gotFreq, _ := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().Frequency().State())
				if gotFreq != wantFreq {
					t.Errorf("Port %s /components/component[name=%s]/optical-channel/state/frequency = %d, want %d", p.Name(), ochName, gotFreq, wantFreq)
				}
			}
		}

		if och == nil || och.TargetOutputPower == nil || math.Abs(och.GetTargetOutputPower()-params.TargetOpticalPower) > powerTolerance {
			_, ok := watchAwait(t, dut, gnmi.OC().Component(ochName).OpticalChannel().TargetOutputPower().State(), sampleTimeout, func(val *ygnmi.Value[float64]) bool {
				v, present := val.Val()
				return present && math.Abs(v-params.TargetOpticalPower) <= powerTolerance
			})
			if !ok {
				gotTargetPower, _ := lookupVal(t, dut, gnmi.OC().Component(ochName).OpticalChannel().TargetOutputPower().State())
				if math.Abs(gotTargetPower-params.TargetOpticalPower) > powerTolerance {
					t.Errorf("Port %s /components/component[name=%s]/optical-channel/state/target-output-power = %v, want %v (+/- %v dBm)", p.Name(), ochName, gotTargetPower, params.TargetOpticalPower, powerTolerance)
				}
			}
		}
	}

	opticsSampleBatch := gnmi.OCBatch()
	opticsSampleBatch.AddPaths(
		gnmi.OC().ComponentAny().OpticalChannel().InputPower().Instant().State().PathStruct(),
		gnmi.OC().ComponentAny().OpticalChannel().OutputPower().Instant().State().PathStruct(),
	)

	opticsBatchStream := samplestream.New(t, dut, opticsSampleBatch.State(), samplingInterval)
	defer opticsBatchStream.Close()

	var sampleMu sync.Mutex
	var sampleDone bool
	ochOK := make(map[string]bool)
	otnOK := make(map[string]bool)
	_, allOpticsOK := opticsBatchStream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[*oc.Root]) bool {
		sampleMu.Lock()
		defer sampleMu.Unlock()
		if sampleDone {
			return true
		}
		root, present := v.Val()
		if !present || root == nil {
			return false
		}
		done := true
		for _, p := range ports {
			ochName := params.OpticalChannelNames[p.Name()]
			och := root.GetComponent(ochName).GetOpticalChannel()
			if !ochOK[p.Name()] && och != nil && och.GetInputPower() != nil && och.GetOutputPower() != nil {
				inPwr := och.GetInputPower().GetInstant()
				outPwr := och.GetOutputPower().GetInstant()
				if inPwr >= minValidInputPowerdBm && inPwr <= maxValidInputPowerdBm && math.Abs(outPwr-params.TargetOpticalPower) <= powerTolerance {
					ochOK[p.Name()] = true
				}
			}
			if !ochOK[p.Name()] {
				done = false
			}
		}
		if done {
			sampleDone = true
		}
		return done
	})
	sampleMu.Lock()
	sampleDone = true
	sampleMu.Unlock()
	opticsBatchStream.Close()

	if !allOpticsOK {
		if root, present := lookupVal(t, dut, opticsSampleBatch.State()); present && root != nil {
			for _, p := range ports {
				ochName := params.OpticalChannelNames[p.Name()]
				och := root.GetComponent(ochName).GetOpticalChannel()
				if !ochOK[p.Name()] && och != nil && och.GetInputPower() != nil && och.GetOutputPower() != nil {
					inPwr := och.GetInputPower().GetInstant()
					outPwr := och.GetOutputPower().GetInstant()
					if inPwr >= minValidInputPowerdBm && inPwr <= maxValidInputPowerdBm && math.Abs(outPwr-params.TargetOpticalPower) <= powerTolerance {
						ochOK[p.Name()] = true
					}
				}
			}
		}
	}

	checkOTN := func(ch *oc.TerminalDevice_Channel, portHasPFEC *bool, portPFEC *uint64) bool {
		if ch == nil || ch.GetOtn() == nil {
			return false
		}
		ber := ch.GetOtn().GetPreFecBer().GetInstant()
		if ber < 0 || ber >= maxPreFECBER {
			return false
		}
		fec := ch.GetOtn().GetFecUncorrectableBlocks()
		if !deviations.NonIntervalFecErrorCounter(dut) && fec == 0 {
			return true
		}
		if !*portHasPFEC {
			*portPFEC = fec
			*portHasPFEC = true
			return false
		}
		if fec != *portPFEC {
			*portPFEC = fec
			return false
		}
		return true
	}

	for _, p := range ports {
		ochName := params.OpticalChannelNames[p.Name()]
		otnIdx := params.OTNIndexes[p.Name()]

		if !ochOK[p.Name()] {
			ochStream := samplestream.New(t, dut, gnmi.OC().Component(ochName).State(), samplingInterval)
			_, ok := ochStream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[*oc.Component]) bool {
				c, present := v.Val()
				if !present || c == nil || c.GetOpticalChannel() == nil {
					return false
				}
				inPwr := c.GetOpticalChannel().GetInputPower().GetInstant()
				outPwr := c.GetOpticalChannel().GetOutputPower().GetInstant()
				return inPwr >= minValidInputPowerdBm && inPwr <= maxValidInputPowerdBm &&
					math.Abs(outPwr-params.TargetOpticalPower) <= powerTolerance
			})
			ochStream.Close()
			if !ok {
				if c, present := lookupVal(t, dut, gnmi.OC().Component(ochName).State()); present && c != nil && c.GetOpticalChannel() != nil {
					inPwr := c.GetOpticalChannel().GetInputPower().GetInstant()
					outPwr := c.GetOpticalChannel().GetOutputPower().GetInstant()
					ok = inPwr >= minValidInputPowerdBm && inPwr <= maxValidInputPowerdBm &&
						math.Abs(outPwr-params.TargetOpticalPower) <= powerTolerance
				}
			}
			if !ok {
				t.Errorf("Port %s optical-channel %s did not report valid input/output power within %v", p.Name(), ochName, sampleTimeout)
			}
		}

		var portPFEC uint64
		var portHasPFEC bool
		otnStream := otnStreams[p.Name()]
		for _, v := range otnStream.All() {
			if ch, present := v.Val(); present && checkOTN(ch, &portHasPFEC, &portPFEC) {
				otnOK[p.Name()] = true
				break
			}
		}
		if !otnOK[p.Name()] {
			_, ok := otnStream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[*oc.TerminalDevice_Channel]) bool {
				ch, present := v.Val()
				return present && checkOTN(ch, &portHasPFEC, &portPFEC)
			})
			otnOK[p.Name()] = ok
		}
		otnStream.Close()
		if !otnOK[p.Name()] {
			if ch, ok := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(otnIdx).State()); ok && checkOTN(ch, &portHasPFEC, &portPFEC) {
				otnOK[p.Name()] = true
			} else if ok && ch != nil && ch.GetOtn() != nil {
				time.Sleep(samplingInterval)
				if ch2, ok2 := lookupVal(t, dut, gnmi.OC().TerminalDevice().Channel(otnIdx).State()); ok2 && checkOTN(ch2, &portHasPFEC, &portPFEC) {
					otnOK[p.Name()] = true
				}
			}
		}
		if !otnOK[p.Name()] {
			t.Errorf("Port %s OTN channel %d did not report valid pre-fec-ber (< %v) and zero fec-uncorrectable-blocks within %v", p.Name(), otnIdx, maxPreFECBER, sampleTimeout)
		}
	}
}

// VerifyLaserShutoffOutputPower waits for optical-channel output-power/instant to drop below or equal to maxLaserOffPowerdBm
// and verifies the value via gnmi.Lookup.
func VerifyLaserShutoffOutputPower(t *testing.T, dut *ondatra.DUTDevice, ochName string, maxLaserOffPowerdBm float64, timeout time.Duration) {
	t.Helper()
	pwrBatch := gnmi.OCBatch()
	pwrBatch.AddPaths(gnmi.OC().ComponentAny().OpticalChannel().OutputPower().Instant().State().PathStruct())
	ochStream := samplestream.New(t, dut, pwrBatch.State(), samplingInterval)
	defer ochStream.Close()
	var gotPower float64
	var hasPower bool
	_, ok := ochStream.AwaitNext(timeout, func(v *ygnmi.Value[*oc.Root]) bool {
		root, present := v.Val()
		if !present || root == nil {
			return false
		}
		och := root.GetComponent(ochName).GetOpticalChannel()
		if och == nil || och.GetOutputPower() == nil || och.GetOutputPower().Instant == nil {
			return false
		}
		gotPower = och.GetOutputPower().GetInstant()
		hasPower = true
		return gotPower <= maxLaserOffPowerdBm
	})
	ochStream.Close()
	if !ok {
		t.Errorf("Optical channel %s sample stream timed out waiting for output-power/instant <= %v dBm within %v", ochName, maxLaserOffPowerdBm, timeout)
		if root, present := lookupVal(t, dut, pwrBatch.State()); present && root != nil {
			if och := root.GetComponent(ochName).GetOpticalChannel(); och != nil && och.GetOutputPower() != nil && och.GetOutputPower().Instant != nil {
				gotPower = och.GetOutputPower().GetInstant()
				hasPower = true
			}
		}
		if !hasPower || gotPower > maxLaserOffPowerdBm {
			t.Errorf("/components/component[name=%s]/optical-channel/state/output-power/instant = %v dBm, want <= %v dBm when transceiver is disabled", ochName, gotPower, maxLaserOffPowerdBm)
		}
		return
	}
	if !hasPower || gotPower > maxLaserOffPowerdBm {
		t.Errorf("/components/component[name=%s]/optical-channel/state/output-power/instant = %v dBm, want <= %v dBm when transceiver is disabled", ochName, gotPower, maxLaserOffPowerdBm)
	}
}

// VerifyAFTPrefixesConverged uses a single batched gnmi.OCBatch() watch to verify that all IPv4 and IPv6
// static prefixes are installed in the network instance AFT or static routing table without N+1 sequential RPCs.
func VerifyAFTPrefixesConverged(t *testing.T, dut *ondatra.DUTDevice, niName string, ipv4Prefixes, ipv6Prefixes []string, timeout time.Duration) {
	t.Helper()
	staticProtoName := deviations.StaticProtocolName(dut)
	aftBatch := gnmi.OCBatch()
	for _, prefix := range ipv4Prefixes {
		aftBatch.AddPaths(gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, staticProtoName).Static(prefix).Prefix().State().PathStruct())
	}
	for _, prefix := range ipv6Prefixes {
		aftBatch.AddPaths(gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, staticProtoName).Static(prefix).Prefix().State().PathStruct())
	}
	foundV4 := make(map[string]bool, len(ipv4Prefixes))
	foundV6 := make(map[string]bool, len(ipv6Prefixes))

	checkInstalled := func() (missingV4, missingV6 []string) {
		if root, present := lookupVal(t, dut, aftBatch.State()); present && root != nil {
			if ni := root.GetNetworkInstance(niName); ni != nil {
				if sp := ni.GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, staticProtoName); sp != nil {
					for _, prefix := range ipv4Prefixes {
						if sp.GetStatic(prefix).GetPrefix() == prefix {
							foundV4[prefix] = true
						}
					}
					for _, prefix := range ipv6Prefixes {
						if sp.GetStatic(prefix).GetPrefix() == prefix {
							foundV6[prefix] = true
						}
					}
				}
			}
		}
		needV4AFT := false
		for _, prefix := range ipv4Prefixes {
			if !foundV4[prefix] {
				needV4AFT = true
				break
			}
		}
		if needV4AFT {
			for attempt := 0; attempt < 3; attempt++ {
				var vals []*ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]
				if msg := testt.CaptureFatal(t, func(tb testing.TB) {
					vals = gnmi.LookupAll(tb, dut, gnmi.OC().NetworkInstance(niName).Afts().Ipv4EntryAny().State())
				}); msg == nil {
					for _, v := range vals {
						if entry, ok := v.Val(); ok && entry != nil && entry.GetPrefix() != "" {
							foundV4[entry.GetPrefix()] = true
						}
					}
					break
				}
			}
		}
		needV6AFT := false
		for _, prefix := range ipv6Prefixes {
			if !foundV6[prefix] {
				needV6AFT = true
				break
			}
		}
		if needV6AFT {
			for attempt := 0; attempt < 3; attempt++ {
				var vals []*ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]
				if msg := testt.CaptureFatal(t, func(tb testing.TB) {
					vals = gnmi.LookupAll(tb, dut, gnmi.OC().NetworkInstance(niName).Afts().Ipv6EntryAny().State())
				}); msg == nil {
					for _, v := range vals {
						if entry, ok := v.Val(); ok && entry != nil && entry.GetPrefix() != "" {
							foundV6[entry.GetPrefix()] = true
						}
					}
					break
				}
			}
		}
		for _, prefix := range ipv4Prefixes {
			if !foundV4[prefix] {
				if entry, ok := lookupVal(t, dut, gnmi.OC().NetworkInstance(niName).Afts().Ipv4Entry(prefix).State()); ok && entry != nil && entry.GetPrefix() == prefix {
					foundV4[prefix] = true
				} else {
					missingV4 = append(missingV4, prefix)
				}
			}
		}
		for _, prefix := range ipv6Prefixes {
			if !foundV6[prefix] {
				if entry, ok := lookupVal(t, dut, gnmi.OC().NetworkInstance(niName).Afts().Ipv6Entry(prefix).State()); ok && entry != nil && entry.GetPrefix() == prefix {
					foundV6[prefix] = true
				} else {
					missingV6 = append(missingV6, prefix)
				}
			}
		}
		return missingV4, missingV6
	}

	missingV4, missingV6 := checkInstalled()
	if len(missingV4) == 0 && len(missingV6) == 0 {
		return
	}
	deadline := time.Now().Add(timeout)
	for (len(missingV4) > 0 || len(missingV6) > 0) && time.Now().Before(deadline) {
		waitDur := 15 * time.Second
		if rem := time.Until(deadline); rem < waitDur {
			waitDur = rem
		}
		if len(missingV4) > 0 {
			p := missingV4[0]
			watchAwait(t, dut, gnmi.OC().NetworkInstance(niName).Afts().Ipv4Entry(p).State(), waitDur, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
				e, ok := val.Val()
				return ok && e != nil && e.GetPrefix() == p
			})
		} else if len(missingV6) > 0 {
			p := missingV6[0]
			watchAwait(t, dut, gnmi.OC().NetworkInstance(niName).Afts().Ipv6Entry(p).State(), waitDur, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
				e, ok := val.Val()
				return ok && e != nil && e.GetPrefix() == p
			})
		}
		missingV4, missingV6 = checkInstalled()
	}
	for _, prefix := range missingV4 {
		t.Errorf("IPv4 static prefix %s not installed in AFT for network-instance %s", prefix, niName)
	}
	for _, prefix := range missingV6 {
		t.Errorf("IPv6 static prefix %s not installed in AFT for network-instance %s", prefix, niName)
	}
}
