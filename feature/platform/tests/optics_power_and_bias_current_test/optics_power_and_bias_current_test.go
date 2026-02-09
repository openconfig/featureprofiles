// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package optics_power_and_bias_current_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/functional-translators/registrar"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/platform"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ethernetCsmacd         = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	transceiverType        = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	sensorType             = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR
	sleepDuration          = time.Minute
	minOpticsPower         = -40.0
	maxOpticsPower         = 10.0
	minOpticsHighThreshold = 1.0
	maxOpticsLowThreshold  = -1.0
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology:
//   ate:port1 <--> port1:dut:port2 <--> ate:port2
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func getOptsForFunctionalTranslator(t *testing.T, dut *ondatra.DUTDevice, functionalTranslatorName string) []ygnmi.Option {
	if functionalTranslatorName == "" {
		return nil
	}
	ft, ok := registrar.FunctionalTranslatorRegistry[functionalTranslatorName]
	if !ok {
		t.Fatalf("Functional translator %s is not registered", functionalTranslatorName)
	}
	deviceSoftwareVersion := strings.Split(dut.Version(), "-")[0]
	ftMetadata := ft.Metadata()
	for _, m := range ftMetadata {
		if m.SoftwareVersion == deviceSoftwareVersion {
			return []ygnmi.Option{ygnmi.WithFT(ft)}
		}
	}
	return nil
}

func gnmiOpts(t *testing.T, dut *ondatra.DUTDevice, mode gpb.SubscriptionMode, interval time.Duration, functionalTranslatorName ...string) *gnmi.Opts {
	opts := []ygnmi.Option{ygnmi.WithSubscriptionMode(mode), ygnmi.WithSampleInterval(interval)}
	if len(functionalTranslatorName) > 0 {
		opts = append(opts, getOptsForFunctionalTranslator(t, dut, functionalTranslatorName[0])...)
	}
	return dut.GNMIOpts().WithYGNMIOpts(opts...)
}

type checkThresholdParams struct {
	transceiver string
	isPortUp    bool
	opts        []ygnmi.Option
	lowerPath   ygnmi.SingletonQuery[float64]
	upperPath   ygnmi.SingletonQuery[float64]
	instantPath ygnmi.SingletonQuery[float64]
	name        string
	ftName      string
}

// checkThreshold validates a pair of lower and upper thresholds.
func checkThreshold(t *testing.T, dut *ondatra.DUTDevice, params checkThresholdParams) {
	t.Helper()
	lV := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(params.opts...), params.lowerPath)
	uV := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(params.opts...), params.upperPath)
	l, lOK := lV.Val()
	u, uOK := uV.Val()

	if !lOK {
		t.Errorf("Transceiver %s: threshold %s-lower is not set", params.transceiver, params.name)
	} else {
		t.Logf("Transceiver %s threshold %s-lower: %v", params.transceiver, params.name, l)
	}
	if !uOK {
		t.Errorf("Transceiver %s: threshold %s-upper is not set", params.transceiver, params.name)
	} else {
		t.Logf("Transceiver %s threshold %s-upper: %v", params.transceiver, params.name, u)
	}

	// OC path /components/component/transceiver/physical-channels/channel/state/laser-temperature/instant
	// is not supported by some vendors, so we skip the check.
	if params.name == "module-temperature" && dut.Vendor() != ondatra.CISCO {
		t.Logf("Skipping instant path check for module-temperature on non-Cisco vendor.")
	} else {
		iV := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(getOptsForFunctionalTranslator(t, dut, params.ftName)...), params.instantPath)
		i, iOK := iV.Val()
		if !iOK {
			t.Errorf("Transceiver %s: instant %s is not set", params.transceiver, params.name)
		} else {
			t.Logf("Transceiver %s instant %s: %v", params.transceiver, params.name, i)
		}

		if lOK && uOK && iOK {
			if !params.isPortUp {
				t.Logf("Skipping range check for transceiver %s because port is not UP.", params.transceiver)
			} else if (params.name == "input-power" || params.name == "output-power") && i == minOpticsPower {
				t.Logf("Skipping range check for transceiver %s for %s because instant value is %v, indicating link is not stable.", params.transceiver, params.name, minOpticsPower)
			} else if i < l || i > u {
				t.Errorf("Transceiver %s: instant %s %v is outside threshold range [%v, %v]", params.transceiver, params.name, i, l, u)
			} else {
				t.Logf("Transceiver %s: instant %s %v is within threshold range [%v, %v]", params.transceiver, params.name, i, l, u)
			}
		}
	}

	if lOK && uOK && l >= u {
		t.Errorf("Transceiver %s: %s-lower (%v) must be less than %s-upper (%v)", params.transceiver, params.name, l, params.name, u)
	}
}

// validateThresholds checks that threshold leaves are populated,
// are within a plausible range, and that lower thresholds are less than upper thresholds.
func validateThresholds(t *testing.T, dut *ondatra.DUTDevice, transceiver string, isPortUp bool, sev oc.E_AlarmTypes_OPENCONFIG_ALARM_SEVERITY, component *platform.ComponentPath, opts []ygnmi.Option) {
	t.Helper()
	t.Logf("Validating transceiver %s thresholds for severity: %v", transceiver, sev)
	threshold := component.Transceiver().Threshold(sev)

	checkThreshold(t, dut, checkThresholdParams{
		transceiver: transceiver,
		isPortUp:    isPortUp,
		opts:        opts,
		lowerPath:   threshold.ModuleTemperatureLower().State(),
		upperPath:   threshold.ModuleTemperatureUpper().State(),
		instantPath: component.Transceiver().Channel(0).LaserTemperature().Instant().State(),
		name:        "module-temperature",
	})
	checkThreshold(t, dut, checkThresholdParams{
		transceiver: transceiver,
		isPortUp:    isPortUp,
		opts:        opts,
		lowerPath:   threshold.InputPowerLower().State(),
		upperPath:   threshold.InputPowerUpper().State(),
		instantPath: component.Transceiver().Channel(0).InputPower().Instant().State(),
		name:        "input-power",
		ftName:      deviations.CiscoxrTransceiverFt(dut),
	})
	checkThreshold(t, dut, checkThresholdParams{
		transceiver: transceiver,
		isPortUp:    isPortUp,
		opts:        opts,
		lowerPath:   threshold.OutputPowerLower().State(),
		upperPath:   threshold.OutputPowerUpper().State(),
		instantPath: component.Transceiver().Channel(0).OutputPower().Instant().State(),
		name:        "output-power",
		ftName:      deviations.CiscoxrTransceiverFt(dut),
	})
}
func TestOpticsPowerBiasCurrent(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	transceiverToInterface := make(map[string]string)
	intfs := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	for _, intf := range intfs {
		if intf.GetTransceiver() != "" {
			transceiverToInterface[intf.GetTransceiver()] = intf.GetName()
		}
	}

	transceivers := components.FindComponentsByType(t, dut, transceiverType)
	t.Logf("Found transceiver list: %v", transceivers)
	if len(transceivers) == 0 {
		t.Fatalf("Get transceiver list for %q: got 0, want > 0", dut.Model())
	}
	var populated []string
	for _, transceiver := range transceivers {
		if gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).MfgName().State()).IsPresent() {
			populated = append(populated, transceiver)
		}
	}
	if len(populated) == 0 {
		t.Fatalf("Populated transceiver list for %q: got 0, want > 0", dut.Model())
	}

	for _, transceiver := range populated {
		t.Run(transceiver, func(t *testing.T) {
			component := gnmi.OC().Component(transceiver)

			mfgName := gnmi.Get(t, dut, component.MfgName().State())
			t.Logf("Transceiver %s MfgName: %s", transceiver, mfgName)

			inputPowers := gnmi.CollectAll(t, gnmiOpts(t, dut, gpb.SubscriptionMode_SAMPLE, time.Second*30, deviations.CiscoxrTransceiverFt(dut)), component.Transceiver().ChannelAny().InputPower().Instant().State(), time.Second*30).Await(t)
			t.Logf("Transceiver %s inputPowers: %v", transceiver, inputPowers)
			if len(inputPowers) == 0 {
				t.Errorf("Get inputPowers list for %q: got 0, want > 0", transceiver)
			}
			outputPowers := gnmi.CollectAll(t, gnmiOpts(t, dut, gpb.SubscriptionMode_SAMPLE, time.Second*30, deviations.CiscoxrTransceiverFt(dut)), component.Transceiver().ChannelAny().OutputPower().Instant().State(), time.Second*30).Await(t)
			t.Logf("Transceiver %s outputPowers: %v", transceiver, outputPowers)
			if len(outputPowers) == 0 {
				t.Errorf("Get outputPowers list for %q: got 0, want > 0", transceiver)
			}

			biasCurrents := gnmi.CollectAll(t, gnmiOpts(t, dut, gpb.SubscriptionMode_SAMPLE, time.Second*30, deviations.CiscoxrTransceiverFt(dut)), component.Transceiver().ChannelAny().LaserBiasCurrent().Instant().State(), time.Second*30).Await(t)
			t.Logf("Transceiver %s biasCurrents: %v", transceiver, biasCurrents)
			if len(biasCurrents) == 0 {
				t.Errorf("Get biasCurrents list for %q: got 0, want > 0", transceiver)
			}

			subcomponents := gnmi.LookupAll[*oc.Component_Subcomponent](t, dut, gnmi.OC().Component(transceiver).SubcomponentAny().State())
			sensorComponentChecked := false
			for _, s := range subcomponents {
				subc, ok := s.Val()
				if ok {
					sensorComponent := gnmi.Get[*oc.Component](t, dut, gnmi.OC().Component(subc.GetName()).State())
					if sensorComponent.GetType() == sensorType {
						scomponent := gnmi.OC().Component(sensorComponent.GetName())
						sensorComponentChecked = true
						if !deviations.TemperatureSensorCheck(dut) || strings.Contains(sensorComponent.GetDescription(), "Temperature Sensor") {
							v := gnmi.Lookup(t, dut, scomponent.Temperature().Instant().State())
							if _, ok := v.Val(); !ok {
								t.Errorf("Sensor %s: Temperature instant is not defined", sensorComponent.GetName())
							}
						}
					}
				}
			}
			if len(subcomponents) == 0 || sensorComponentChecked == false {
				v := gnmi.Lookup(t, dut, component.Temperature().Instant().State())
				if _, ok := v.Val(); !ok {
					t.Errorf("Transceiver %s: Temperature instant is not defined", transceiver)
				}
			}

			if deviations.TransceiverThresholdsUnsupported(dut) {
				t.Logf("Skipping verification of transceiver threshold leaves due to deviation")
				return
			}
			// TODO(ankursaikia): Validate the values for each leaf.
			opts := getOptsForFunctionalTranslator(t, dut, deviations.CiscoxrLaserFt(dut))
			var sevs []oc.E_AlarmTypes_OPENCONFIG_ALARM_SEVERITY
			for _, s := range gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), component.Transceiver().ThresholdAny().Severity().State()) {
				val, ok := s.Val()
				if !ok || val == oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_UNSET {
					t.Errorf("Transceiver %s: threshold severity is not set", transceiver)
					continue
				}
				sevs = append(sevs, val)
			}

			portName := transceiverToInterface[transceiver]
			var isPortUp bool
			if portName != "" {
				operStatus := gnmi.Get(t, dut, gnmi.OC().Interface(portName).OperStatus().State())
				isPortUp = operStatus == oc.Interface_OperStatus_UP
			} else {
				t.Logf("Could not find interface for transceiver %s, assuming port is down for threshold checks", transceiver)
			}
			for _, sev := range sevs {
				validateThresholds(t, dut, transceiver, isPortUp, sev, component, opts)
			}
		})
	}
}

func TestOpticsPowerUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())

	cases := []struct {
		desc                string
		IntfStatus          bool
		expectedStatus      oc.E_Interface_OperStatus
		expectedMaxOutPower float64
		checkMinOutPower    bool
	}{{
		// Check both input and output optics power are in normal range.
		desc:                "Check initial input and output optics powers are OK",
		IntfStatus:          true,
		expectedStatus:      oc.Interface_OperStatus_UP,
		expectedMaxOutPower: maxOpticsPower,
		checkMinOutPower:    true,
	}, {
		desc:                "Check output optics power is very small after interface is disabled",
		IntfStatus:          false,
		expectedStatus:      oc.Interface_OperStatus_DOWN,
		expectedMaxOutPower: minOpticsPower,
		checkMinOutPower:    false,
	}, {
		desc:                "Check output optics power is normal after interface is re-enabled",
		IntfStatus:          true,
		expectedStatus:      oc.Interface_OperStatus_UP,
		expectedMaxOutPower: maxOpticsPower,
		checkMinOutPower:    true,
	}}
	for _, tc := range cases {
		t.Log(tc.desc)
		intUpdateTime := 2 * time.Minute
		t.Run(tc.desc, func(t *testing.T) {
			i.Enabled = ygot.Bool(tc.IntfStatus)
			i.Type = ethernetCsmacd
			if deviations.ExplicitPortSpeed(dut) {
				i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, dp)
			}
			gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, tc.expectedStatus)

			transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())

			component := gnmi.OC().Component(transceiverName)
			if !gnmi.Lookup(t, dut, component.MfgName().State()).IsPresent() {
				t.Skipf("component.MfgName().Lookup(t).IsPresent() for %q is false. skip it", transceiverName)
			}

			mfgName := gnmi.Get(t, dut, component.MfgName().State())
			t.Logf("Transceiver MfgName: %s", mfgName)

			channels := gnmi.OC().Component(dp.Name()).Transceiver().ChannelAny()
			opts := getOptsForFunctionalTranslator(t, dut, deviations.CiscoxrTransceiverFt(dut))
			inputPowers := gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), channels.InputPower().Instant().State())
			outputPowers := gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), channels.OutputPower().Instant().State())
			for _, inputPower := range inputPowers {
				inPower, ok := inputPower.Val()
				if !ok {
					t.Errorf("Get inputPower for port %q: got 0, want > 0", dp.Name())
					continue
				}
				if inPower > maxOpticsPower || inPower < minOpticsPower {
					t.Errorf("Get inputPower for port %q): got %.2f, want within [%f, %f]", dp.Name(), inPower, minOpticsPower, maxOpticsPower)
				}
			}
			for _, outputPower := range outputPowers {
				outPower, ok := outputPower.Val()
				if !ok {
					t.Errorf("Get outputPower for port %q: got 0, want > 0", dp.Name())
					continue
				}
				if outPower > tc.expectedMaxOutPower {
					t.Errorf("Get outPower for port %q): got %.2f, want < %f", dp.Name(), outPower, tc.expectedMaxOutPower)
				}
				if tc.checkMinOutPower && outPower < minOpticsPower {
					t.Errorf("Get outPower for port %q): got %.2f, want > %f", dp.Name(), outPower, minOpticsPower)
				}
			}
			if deviations.TransceiverThresholdsUnsupported(dut) {
				t.Logf("Skipping verification of transceiver threshold leaves due to deviation")
			} else {
				opts := getOptsForFunctionalTranslator(t, dut, deviations.CiscoxrLaserFt(dut))
				var sevs []oc.E_AlarmTypes_OPENCONFIG_ALARM_SEVERITY
				for _, s := range gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), component.Transceiver().ThresholdAny().Severity().State()) {
					val, ok := s.Val()
					if !ok || val == oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_UNSET {
						t.Errorf("Transceiver %s: threshold severity is not set", transceiverName)
						continue
					}
					sevs = append(sevs, val)
				}

				isPortUp := tc.expectedStatus == oc.Interface_OperStatus_UP
				for _, sev := range sevs {
					validateThresholds(t, dut, transceiverName, isPortUp, sev, component, opts)
				}
			}
		})
	}
}

func TestInterfacesWithTransceivers(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Map of populated Transceivers to its corresponding Component state object.
	populatedTvs := make(map[string]*oc.Component)

	// Map of non populated Transceivers to its corresponding Component state object.
	emptyTvs := make(map[string]*oc.Component)

	tvs := components.FindComponentsByType(t, dut, transceiverType)
	for _, tv := range tvs {
		t.Run(fmt.Sprintf("Transceiver:%s", tv), func(t *testing.T) {
			cp := gnmi.Get(t, dut, gnmi.OC().Component(tv).State())
			if cp.GetMfgName() == "" {
				emptyTvs[tv] = cp
				t.Skipf("Skipping check for Transceiver: %q, got no MfgName.", tv)
			}
			populatedTvs[tv] = cp
			if cp.GetTransceiver() == nil || cp.GetTransceiver().GetFormFactor() == oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_UNSET {
				t.Errorf("transceiver/form-factor unset for Transceiver: %q", tv)
			}
			if deviations.TransceiverStateUnsupported(dut) {
				t.Logf("Skipping verification of transceiver vendor details due to deviation")
			} else {
				opts := getOptsForFunctionalTranslator(t, dut, deviations.CiscoxrTransceiverFt(dut))
				vendor := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().Component(tv).Transceiver().Vendor().State())
				if vendor == "" {
					t.Errorf("Transceiver %s: Vendor name is empty", tv)
				}
				vendorPart := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().Component(tv).Transceiver().VendorPart().State())
				if vendorPart == "" {
					t.Errorf("Transceiver %s: VendorPart is empty", tv)
				}
				vendorRev := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().Component(tv).Transceiver().VendorRev().State())
				if vendorRev == "" {
					t.Errorf("Transceiver %s: VendorRev is empty", tv)
				}
			}
		})
	}

	// Map of interface name to its connected Transceiver name.
	intfTransceivers := make(map[string]string)

	// Virtual interfaces that don't have physical channel.
	virtualIntfs := []string{"Loopback", "Management", "Port-Channel", "ae", "lo", "Bundle-Ether", "MgmtEth", "Null", "PTP", "vtep", "re", "pip", "pfh", "lsi", "irb", "dsc", "esi", "fti", "mgmt", "lag"}

	intfs := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	for _, intf := range intfs {
		if intf.GetName() == "" {
			continue
		}
		t.Run(fmt.Sprintf("Interface:%s", intf.GetName()), func(t *testing.T) {
			// Skipping interfaces that are not connected.
			if _, ok := emptyTvs[intf.GetTransceiver()]; ok {
				t.Skipf("Skipping check for Interface %q, got empty transceiver", intf.GetName())
			}
			if _, ok := populatedTvs[intf.GetTransceiver()]; !ok {
				t.Skipf("Skipping check for Interface %q, got empty transceiver", intf.GetName())
			}
			// Skipping Aggregate, Loopback and Management Interfaces.
			for _, vi := range virtualIntfs {
				if strings.HasPrefix(intf.GetName(), vi) {
					t.Skipf("Skipping check for virtual interface %q", intf.GetName())
				}
			}

			intfTransceivers[intf.GetName()] = intf.GetTransceiver()
			if intf.PhysicalChannel == nil {
				t.Errorf("physical-channel unset for Interface: %q", intf.GetName())
			} else {
				for _, p := range intf.PhysicalChannel {
					if p != populatedTvs[intf.GetTransceiver()].GetTransceiver().GetChannel(p).GetIndex() {
						t.Errorf("Transceiver %s failed to get channel index %v", intf.GetTransceiver(), p)
					}
				}
			}
		})
	}
	if len(intfTransceivers) == 0 {
		t.Fatalf("Populated interfaces list for %q: got 0, want > 0", dut.Model())
	}

	// Get the unique transceivers from the interface to transceiver mapping.
	intfTvs := map[string]int{}
	for _, tv := range intfTransceivers {
		intfTvs[tv] = 0
	}

	if len(intfTvs) != len(populatedTvs) {
		t.Errorf("Unexpected numbers of transceivers found, from interface/state/transceiver: %d, from component/state/name: %d", len(intfTvs), len(populatedTvs))
	}
	if len(intfTvs) > len(populatedTvs) {
		for tv := range intfTvs {
			if _, ok := populatedTvs[tv]; !ok {
				t.Errorf("Transceiver: %s, not found in components/state", tv)
			}
		}
	} else {
		for tv := range populatedTvs {
			if _, ok := intfTvs[tv]; !ok {
				t.Errorf("Transceiver: %s, not found in interface/state/transceiver", tv)
			}
		}
	}
}
