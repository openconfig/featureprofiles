package transceiver_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/olekukonko/tablewriter"
	"github.com/openconfig/featureprofiles/feature/cisco/performance"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func findComponentsByTypeNoLogs(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	var s []string
	for _, c := range components {
		if c.GetType() == nil {
			continue
		}
		switch v := c.GetType().(type) {
		case oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
			if v == cType {
				s = append(s, c.GetName())
			}
		}
	}
	return s
}

func checkleaves(t *testing.T, dut *ondatra.DUTDevice, transceiver string, state []*oc.Component_Transceiver_Channel) bool {
	flag := true
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Transceiver", "Leaf", "Value"})

	for channel := range state {
		// CDETS: CSCwk32258. This will be un-commented after the fix
		// appendToTableIfNotNil(t, table, transceiver, "Description", state[channel].Description, "Description is empty for port %v")

		appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/associated-optical-channel]", state[channel].AssociatedOpticalChannel, "associated_optical_channel is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/index", state[channel].Index, "Index is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-age", state[channel].LaserAge, "LaserAge is empty for port %v")
		if state[channel].InputPower == nil {
			t.Errorf("InputPower data is empty for port %v", transceiver)
			flag = false
		} else {
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/input-power/avg", state[channel].InputPower.Avg, "input_power_average is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/input-power/instant", state[channel].InputPower.Instant, "input_power_instant is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/input-power/interval", state[channel].InputPower.Interval, "input_power_interval is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/input-power/max", state[channel].InputPower.Max, "input_power_max is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/input-power/max-time", state[channel].InputPower.MaxTime, "input_power_max_time is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/input-power/min", state[channel].InputPower.Min, "input_power_min is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/input-power/min-time", state[channel].InputPower.MinTime, "input_power_min_time is empty for port %v")
			validatePMValue(t, transceiver, "InputPower", *state[channel].InputPower.Instant, *state[channel].InputPower.Min, *state[channel].InputPower.Max, *state[channel].InputPower.Avg)
		}
		if state[channel].LaserBiasCurrent == nil {
			t.Errorf("LaserBiasCurrent data is empty for port %v", transceiver)
			flag = false
		} else {
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-bias-current/interval", state[channel].LaserBiasCurrent.Interval, "laser_bias_current_interval is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-bias-current/avg", state[channel].LaserBiasCurrent.Avg, "laser_bias_current_avg is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-bias-current/instant", state[channel].LaserBiasCurrent.Instant, "laser_bias_current_instant is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-bias-current/max", state[channel].LaserBiasCurrent.Max, "laser_bias_current_max is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-bias-current/max-time", state[channel].LaserBiasCurrent.MaxTime, "laser_bias_current_max_time is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-bias-current/min", state[channel].LaserBiasCurrent.Min, "laser_bias_current_min is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-bias-current/min-time", state[channel].LaserBiasCurrent.MinTime, "laser_bias_current_min_time is empty for port %v")
			validatePMValue(t, transceiver, "LaserBiasCurrent", *state[channel].LaserBiasCurrent.Instant, *state[channel].LaserBiasCurrent.Min, *state[channel].LaserBiasCurrent.Max, *state[channel].LaserBiasCurrent.Avg)
		}

		if state[channel].LaserTemperature == nil {
			t.Errorf("LaserTemperature data is empty for port %v", transceiver)
			flag = false
		} else {
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-temperature/instant", state[channel].LaserTemperature.Instant, "LaserTemperature_Instant is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-temperature/interval", state[channel].LaserTemperature.Interval, "LaserTemperature_Interval is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-temperature/max", state[channel].LaserTemperature.Max, "LaserTemperature_Max is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-temperature/max-time", state[channel].LaserTemperature.MaxTime, "LaserTemperature_MaxTime is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-temperature/min", state[channel].LaserTemperature.Min, "LaserTemperature_Min is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/laser-temperature/min-time", state[channel].LaserTemperature.MinTime, "LaserTemperature_MinTime is empty for port %v")
			validatePMValue(t, transceiver, "LaserTemperature", *state[channel].LaserTemperature.Instant, *state[channel].LaserTemperature.Min, *state[channel].LaserTemperature.Max, *state[channel].LaserTemperature.Instant)
		}

		appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/output-frequency", state[channel].OutputFrequency, "OutputFrequency is empty for port %v")

		if state[channel].OutputPower == nil {
			t.Errorf("OutputPower data is empty for port %v", transceiver)
			flag = false
		} else {
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/output-power/avg", state[channel].OutputPower.Avg, "OutputPower_Avg is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/output-power/instant", state[channel].OutputPower.Instant, "OutputPower_Instant is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/output-power/interval", state[channel].OutputPower.Interval, "OutputPower_Interval is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/output-power/max", state[channel].OutputPower.Max, "OutputPower_Max is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/output-power/max-time", state[channel].OutputPower.MaxTime, "OutputPower_MaxTime is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/output-power/min", state[channel].OutputPower.Min, "OutputPower_Min is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/output-power/min-time", state[channel].OutputPower.MinTime, "OutputPower_MinTime is empty for port %v")
			validatePMValue(t, transceiver, "OutputPower", *state[channel].OutputPower.Instant, *state[channel].OutputPower.Min, *state[channel].OutputPower.Max, *state[channel].OutputPower.Avg)
		}
		if state[channel].TargetFrequencyDeviation != nil {
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/target-frequency-deviation/avg", state[channel].TargetFrequencyDeviation.Avg, "TargetFrequencyDeviation_Avg is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/target-frequency-deviation/instant", state[channel].TargetFrequencyDeviation.Instant, "TargetFrequencyDeviation_Instant is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/target-frequency-deviation/interval", state[channel].TargetFrequencyDeviation.Interval, "TargetFrequencyDeviation_Interval is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/target-frequency-deviation/max", state[channel].TargetFrequencyDeviation.Max, "TargetFrequencyDeviation_Max is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/target-frequency-deviation/max-time", state[channel].TargetFrequencyDeviation.MaxTime, "TargetFrequencyDeviation_MaxTime is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/target-frequency-deviation/min", state[channel].TargetFrequencyDeviation.Min, "TargetFrequencyDeviation_Min is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/target-frequency-deviation/min-time", state[channel].TargetFrequencyDeviation.MinTime, "TargetFrequencyDeviation_MinTime is empty for port %v")
			validatePMValue(t, transceiver, "TargetFrequencyDeviation", *state[channel].TargetFrequencyDeviation.Instant, *state[channel].TargetFrequencyDeviation.Min, *state[channel].TargetFrequencyDeviation.Max, *state[channel].TargetFrequencyDeviation.Avg)
		} else {
			t.Errorf("TargetFrequencyDeviation data is empty for port %v", transceiver)
			flag = false
		}

		appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/target-output-power", state[channel].TargetOutputPower, "TargetOutputPower is empty for port %v")

		if state[channel].TecCurrent != nil {
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/tec-current/avg", state[channel].TecCurrent.Avg, "TecCurrent_Avg is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/tec-current/instant", state[channel].TecCurrent.Instant, "TecCurrent_Instant is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/tec-current/interval", state[channel].TecCurrent.Interval, "TecCurrent_Interval is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/tec-current/max", state[channel].TecCurrent.Max, "TecCurrent_Max is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/tec-current/max-time", state[channel].TecCurrent.MaxTime, "TecCurrent_MaxTime is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/tec-current/min", state[channel].TecCurrent.Min, "TecCurrent_Min is empty for port %v")
			appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/tec-current/min-time", state[channel].TecCurrent.MinTime, "TecCurrent_MinTime is empty for port %v")
			validatePMValue(t, transceiver, "TecCurrent", *state[channel].TecCurrent.Instant, *state[channel].TecCurrent.Min, *state[channel].TecCurrent.Max, *state[channel].TecCurrent.Avg)
		} else {
			t.Errorf("TecCurrent data is empty for port %v", transceiver)
			flag = false
		}

		appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform/components/component[name=*]/transceiver/physical-channels/channel[index=*]/state/tx-laser", state[channel].TxLaser, "TxLaser is empty for port %v")

	}

	s := gnmi.Get(t, dut, gnmi.OC().Component(transceiver).Transceiver().State())

	appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform:components/component[name=*]/openconfig-transceiver:transceiver/state/connector-type", s.GetConnectorType(), "connector-type is empty for port %v")
	appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform:components/component[name=*]/openconfig-transceiver:transceiver/state/date-code", s.GetDateCode(), "date-code is empty for port %v")
	appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform:components/component[name=*]/openconfig-transceiver:transceiver/state/enabled", s.Enabled, "enabled is empty for port %v")
	appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform:components/component[name=*]/openconfig-transceiver:transceiver/state/form-factor", s.GetFormFactor(), "form-factor is empty for port %v")
	appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform:components/component[name=*]/openconfig-transceiver:transceiver/state/module-functional-type", s.GetModuleFunctionalType(), "module-functional-type is empty for port %v")
	appendToTableIfNotNil(t, table, transceiver, "/openconfig-platform:components/component[name=*]/openconfig-transceiver:transceiver/state/otn-compliance-code", s.GetOtnComplianceCode(), "otn-compliance-code is empty for port %v")

	// Optical Channel Leaves
	oc := gnmi.Get(t, dut, gnmi.OC().Component(transceiver).Transceiver().Channel(0).AssociatedOpticalChannel().State())
	optical_channel := gnmi.Get(t, dut, gnmi.OC().Component(oc).OpticalChannel().State())

	if b := optical_channel.GetCarrierFrequencyOffset(); b == nil {
		t.Errorf("CarrierFrequencyOffset data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/carrier-frequency-offset/instant", b.GetInstant(), "CarrierFrequencyOffset_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/carrier-frequency-offset/min", b.GetMin(), "CarrierFrequencyOffset_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/carrier-frequency-offset/max", b.GetMax(), "CarrierFrequencyOffset_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/carrier-frequency-offset/avg", b.GetAvg(), "CarrierFrequencyOffset_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/carrier-frequency-offset/min-time", b.GetMinTime(), "CarrierFrequencyOffset_min_time is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/carrier-frequency-offset/max-time", b.GetMaxTime(), "CarrierFrequencyOffset_max_time is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/carrier-frequency-offset/interval", b.GetInterval(), "CarrierFrequencyOffset_interval is empty for port %v")
		validatePMValue(t, transceiver, "CarrierFrequencyOffset", b.GetInstant(), b.GetMin(), b.GetMax(), b.GetAvg())
	}

	if c := optical_channel.GetChromaticDispersion(); c == nil {
		t.Errorf("ChromaticDispersion data is empty for transceiver %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/chromatic-dispersion/instant", c.GetInstant(), "ChromaticDispersion_instant is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/chromatic-dispersion/min", c.GetMin(), "ChromaticDispersion_min is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/chromatic-dispersion/max", c.GetMax(), "ChromaticDispersion_max is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/chromatic-dispersion/avg", c.GetAvg(), "ChromaticDispersion_avg is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/chromatic-dispersion/min-time", c.GetMinTime(), "ChromaticDispersion_mintime is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/chromatic-dispersion/max-time", c.GetMaxTime(), "ChromaticDispersion_maxtime is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/chromatic-dispersion/interval", c.GetInterval(), "ChromaticDispersion_interval is empty for transceiver %v")
		validatePMValue(t, transceiver, "ChromaticDispersion", c.GetInstant(), c.GetMin(), c.GetMax(), c.GetAvg())
	}

	if e := optical_channel.GetEsnr(); e == nil {
		t.Errorf("ESNR data is empty for transceiver %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/esnr/instant", e.GetInstant(), "ESNR_instant is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/esnr/min", e.GetMin(), "ESNR_min is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/esnr/max", e.GetMax(), "ESNR_max is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/esnr/avg", e.GetAvg(), "ESNR_avg is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/esnr/min-time", e.GetMinTime(), "ESNR_mintime is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/esnr/max-time", e.GetMaxTime(), "ESNR_maxtime is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/esnr/interval", e.GetInterval(), "ESNR_interval is empty for transceiver %v")
		validatePMValue(t, transceiver, "ESNR", e.GetInstant(), e.GetMin(), e.GetMax(), e.GetAvg())
	}

	if f := optical_channel.GetFrequency(); f == 0 {
		t.Errorf("Frequency data is missing for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/frequency", f, "Frequency data is missing for port %v")
	}

	if ip := optical_channel.GetInputPower(); ip == nil {
		t.Errorf("InputPower data is empty for transceiver %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/input-power/instant", ip.GetInstant(), "InputPower_instant is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/input-power/min", ip.GetMin(), "InputPower_min is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/input-power/max", ip.GetMax(), "InputPower_max is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/input-power/avg", ip.GetAvg(), "InputPower_avg is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/input-power/min-time", ip.GetMinTime(), "InputPower_mintime is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/input-power/max-time", ip.GetMaxTime(), "InputPower_maxtime is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/input-power/interval", ip.GetInterval(), "InputPower_interval is empty for transceiver %v")
		validatePMValue(t, transceiver, "InputPower", ip.GetInstant(), ip.GetMin(), ip.GetMax(), ip.GetAvg())
	}

	if lb := optical_channel.GetLaserBiasCurrent(); lb == nil {
		t.Errorf("LaserBiasCurrent data is missing for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/laser-bias-current/instant", lb.GetInstant(), "LaserBiasCurrent_instant is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/laser-bias-current/min", lb.GetMin(), "LaserBiasCurrent_min is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/laser-bias-current/max", lb.GetMax(), "LaserBiasCurrent_max is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/laser-bias-current/avg", lb.GetAvg(), "LaserBiasCurrent_avg is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/laser-bias-current/min-time", lb.GetMinTime(), "LaserBiasCurrent_mintime is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/laser-bias-current/max-time", lb.GetMaxTime(), "LaserBiasCurrent_maxtime is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/laser-bias-current/interval", lb.GetInterval(), "LaserBiasCurrent_interval is missing for port %v")
		validatePMValue(t, transceiver, "LaserBiasCurrent", lb.GetInstant(), lb.GetMin(), lb.GetMax(), lb.GetAvg())
	}

	if lp := optical_channel.GetLinePort(); lp == "" {
		t.Errorf("LinePort data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/line-port", lp, "LinePort_instant is empty for port %v")
	}

	if mer := optical_channel.GetModulationErrorRatio(); mer == nil {
		t.Errorf("ModulationErrorRatio data is empty for transceiver %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulation-error-ratio/instant", mer.GetInstant(), "ModulationErrorRatio_instant is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulation-error-ratio/min", mer.GetMin(), "ModulationErrorRatio_min is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulation-error-ratio/max", mer.GetMax(), "ModulationErrorRatio_max is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulation-error-ratio/avg", mer.GetAvg(), "ModulationErrorRatio_avg is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulation-error-ratio/min-time", mer.GetMinTime(), "ModulationErrorRatio_mintime is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulation-error-ratio/max-time", mer.GetMaxTime(), "ModulationErrorRatio_maxtime is empty for transceiver %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulation-error-ratio/interval", mer.GetInterval(), "ModulationErrorRatio_interval is empty for transceiver %v")
		validatePMValue(t, transceiver, "ModulationErrorRatio", mer.GetInstant(), mer.GetMin(), mer.GetMax(), mer.GetAvg())
	}

	if mbx := optical_channel.GetModulatorBiasXPhase(); mbx == nil {
		t.Errorf("ModulatorBiasXPhase data is missing for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-x-phase/instant", mbx.GetInstant(), "ModulatorBiasXPhase_instant is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-x-phase/min", mbx.GetMin(), "ModulatorBiasXPhase_min is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-x-phase/max", mbx.GetMax(), "ModulatorBiasXPhase_max is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-x-phase/avg", mbx.GetAvg(), "ModulatorBiasXPhase_avg is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-x-phase/min-time", mbx.GetMinTime(), "ModulatorBiasXPhase_mintime is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-x-phase/max-time", mbx.GetMaxTime(), "ModulatorBiasXPhase_maxtime is missing for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-x-phase/interval", mbx.GetInterval(), "ModulatorBiasXPhase_interval is missing for port %v")
		validatePMValue(t, transceiver, "ModulatorBiasXPhase", mbx.GetInstant(), mbx.GetMin(), mbx.GetMax(), mbx.GetAvg())
	}

	if mbxi := optical_channel.GetModulatorBiasXi(); mbxi == nil {
		t.Errorf("ModulatorBiasXi data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xi/instant", mbxi.GetInstant(), "ModulatorBiasXi_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xi/min", mbxi.GetMin(), "ModulatorBiasXi_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xi/max", mbxi.GetMax(), "ModulatorBiasXi_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xi/avg", mbxi.GetAvg(), "ModulatorBiasXi_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xi/min-time", mbxi.GetMinTime(), "ModulatorBiasXi_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xi/max-time", mbxi.GetMaxTime(), "ModulatorBiasXi_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xi/interval", mbxi.GetInterval(), "ModulatorBiasXi_interval is empty for port %v")
		validatePMValue(t, transceiver, "ModulatorBiasXi", mbxi.GetInstant(), mbxi.GetMin(), mbxi.GetMax(), mbxi.GetAvg())
	}

	if mbxq := optical_channel.GetModulatorBiasXq(); mbxq == nil {
		t.Errorf("ModulatorBiasXq data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xq/instant", mbxq.GetInstant(), "ModulatorBiasXq_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xq/min", mbxq.GetMin(), "ModulatorBiasXq_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xq/max", mbxq.GetMax(), "ModulatorBiasXq_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xq/avg", mbxq.GetAvg(), "ModulatorBiasXq_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xq/min-time", mbxq.GetMinTime(), "ModulatorBiasXq_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xq/max-time", mbxq.GetMaxTime(), "ModulatorBiasXq_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-xq/interval", mbxq.GetInterval(), "ModulatorBiasXq_interval is empty for port %v")
		validatePMValue(t, transceiver, "ModulatorBiasXq", mbxq.GetInstant(), mbxq.GetMin(), mbxq.GetMax(), mbxq.GetAvg())
	}

	if mbyp := optical_channel.GetModulatorBiasYPhase(); mbyp == nil {
		t.Errorf("ModulatorBiasYPhase data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-y-phase/instant", mbyp.GetInstant(), "ModulatorBiasYPhase_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-y-phase/min", mbyp.GetMin(), "ModulatorBiasYPhase_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-y-phase/max", mbyp.GetMax(), "ModulatorBiasYPhase_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-y-phase/avg", mbyp.GetAvg(), "ModulatorBiasYPhase_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-y-phase/min-time", mbyp.GetMinTime(), "ModulatorBiasYPhase_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-y-phase/max-time", mbyp.GetMaxTime(), "ModulatorBiasYPhase_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-y-phase/interval", mbyp.GetInterval(), "ModulatorBiasYPhase_interval is empty for port %v")
		validatePMValue(t, transceiver, "ModulatorBiasYPhase", mbyp.GetInstant(), mbyp.GetMin(), mbyp.GetMax(), mbyp.GetAvg())
	}

	if mbyq := optical_channel.GetModulatorBiasYq(); mbyq == nil {
		t.Errorf("ModulatorBiasYq data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yq/instant", mbyq.GetInstant(), "ModulatorBiasYq_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yq/min", mbyq.GetMin(), "ModulatorBiasYq_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yq/max", mbyq.GetMax(), "ModulatorBiasYq_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yq/avg", mbyq.GetAvg(), "ModulatorBiasYq_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yq/min-time", mbyq.GetMinTime(), "ModulatorBiasYq_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yq/max-time", mbyq.GetMaxTime(), "ModulatorBiasYq_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yq/interval", mbyq.GetInterval(), "ModulatorBiasYq_interval is empty for port %v")
		validatePMValue(t, transceiver, "ModulatorBiasYq", mbyq.GetInstant(), mbyq.GetMin(), mbyq.GetMax(), mbyq.GetAvg())
	}

	if mbyi := optical_channel.GetModulatorBiasYi(); mbyi == nil {
		t.Errorf("ModulatorBiasYi data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yi/instant", mbyi.GetInstant(), "ModulatorBiasYi_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yi/min", mbyi.GetMin(), "ModulatorBiasYi_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yi/max", mbyi.GetMax(), "ModulatorBiasYi_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yi/avg", mbyi.GetAvg(), "ModulatorBiasYi_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yi/min-time", mbyi.GetMinTime(), "ModulatorBiasYi_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yi/max-time", mbyi.GetMaxTime(), "ModulatorBiasYi_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/modulator-bias-yi/interval", mbyi.GetInterval(), "ModulatorBiasYi_interval is empty for port %v")
		validatePMValue(t, transceiver, "ModulatorBiasYi", mbyi.GetInstant(), mbyi.GetMin(), mbyi.GetMax(), mbyi.GetAvg())
	}

	if om := optical_channel.GetOperationalMode(); om == 0 {
		t.Errorf("OperationalMode data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/operational-mode", om, "OperationalMode is empty for port %v")
	}

	if osnr := optical_channel.GetOsnr(); osnr == nil {
		t.Errorf("OSNR data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/osnr/instant", osnr.GetInstant(), "OSNR_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/osnr/min", osnr.GetMin(), "OSNR_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/osnr/max", osnr.GetMax(), "OSNR_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/osnr/avg", osnr.GetAvg(), "OSNR_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/osnr/min-time", osnr.GetMinTime(), "OSNR_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/osnr/max-time", osnr.GetMaxTime(), "OSNR_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/osnr/interval", osnr.GetInterval(), "OSNR_interval is empty for port %v")
		validatePMValue(t, transceiver, "OSNR", osnr.GetInstant(), osnr.GetMin(), osnr.GetMax(), osnr.GetAvg())
	}

	if op := optical_channel.GetOutputPower(); op == nil {
		t.Errorf("OutputPower data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/output-power/instant", op.GetInstant(), "OutputPower_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/output-power/min", op.GetMin(), "OutputPower_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/output-power/max", op.GetMax(), "OutputPower_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/output-power/avg", op.GetAvg(), "OutputPower_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/output-power/min-time", op.GetMinTime(), "OutputPower_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/output-power/max-time", op.GetMaxTime(), "OutputPower_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/output-power/interval", op.GetInterval(), "OutputPower_interval is empty for port %v")
		validatePMValue(t, transceiver, "OutputPower", op.GetInstant(), op.GetMin(), op.GetMax(), op.GetAvg())
	}

	if pd := optical_channel.GetPolarizationDependentLoss(); pd == nil {
		t.Errorf("PolarizationDependentLoss data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/polarization-dependent-loss/instant", pd.GetInstant(), "PolarizationDependentLoss_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/polarization-dependent-loss/min", pd.GetMin(), "PolarizationDependentLoss_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/polarization-dependent-loss/max", pd.GetMax(), "PolarizationDependentLoss_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/polarization-dependent-loss/avg", pd.GetAvg(), "PolarizationDependentLoss_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/polarization-dependent-loss/min-time", pd.GetMinTime(), "PolarizationDependentLoss_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/polarization-dependent-loss/max-time", pd.GetMaxTime(), "PolarizationDependentLoss_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/polarization-dependent-loss/interval", pd.GetInterval(), "PolarizationDependentLoss_interval is empty for port %v")
		validatePMValue(t, transceiver, "PolarizationDependentLoss", pd.GetInstant(), pd.GetMin(), pd.GetMax(), pd.GetAvg())
	}

	if pfb := optical_channel.GetPostFecBer(); pfb == nil {
		t.Errorf("PostFecBer data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/post-fec-ber/instant", pfb.GetInstant(), "PostFecBer_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/post-fec-ber/min", pfb.GetMin(), "PostFecBer_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/post-fec-ber/max", pfb.GetMax(), "PostFecBer_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/post-fec-ber/avg", pfb.GetAvg(), "PostFecBer_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/post-fec-ber/min-time", pfb.GetMinTime(), "PostFecBer_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/post-fec-ber/max-time", pfb.GetMaxTime(), "PostFecBer_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/post-fec-ber/interval", pfb.GetInterval(), "PostFecBer_interval is empty for port %v")
		validatePMValue(t, transceiver, "PostFecBer", pfb.GetInstant(), pfb.GetMin(), pfb.GetMax(), pfb.GetAvg())
	}

	if prfb := optical_channel.GetPreFecBer(); prfb == nil {
		t.Errorf("PreFecBer data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/pre-fec-ber/instant", prfb.GetInstant(), "PreFecBer_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/pre-fec-ber/min", prfb.GetMin(), "PreFecBer_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/pre-fec-ber/max", prfb.GetMax(), "PreFecBer_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/pre-fec-ber/avg", prfb.GetAvg(), "PreFecBer_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/pre-fec-ber/min-time", prfb.GetMinTime(), "PreFecBer_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/pre-fec-ber/max-time", prfb.GetMaxTime(), "PreFecBer_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/pre-fec-ber/interval", prfb.GetInterval(), "PreFecBer_interval is empty for port %v")
		validatePMValue(t, transceiver, "PreFecBer", prfb.GetInstant(), prfb.GetMin(), prfb.GetMax(), prfb.GetAvg())
	}

	if q := optical_channel.GetQValue(); q == nil {
		t.Errorf("QValue data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/q-value/instant", q.GetInstant(), "QValue_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/q-value/min", q.GetMin(), "QValue_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/q-value/max", q.GetMax(), "QValue_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/q-value/avg", q.GetAvg(), "QValue_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/q-value/min-time", q.GetMinTime(), "QValue_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/q-value/max-time", q.GetMaxTime(), "QValue_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/q-value/interval", q.GetInterval(), "QValue_interval is empty for port %v")
		validatePMValue(t, transceiver, "QValue", q.GetInstant(), q.GetMin(), q.GetMax(), q.GetAvg())
	}

	if sopmd := optical_channel.GetSecondOrderPolarizationModeDispersion(); sopmd == nil {
		t.Errorf("SecondOrderPolarizationModeDispersion data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/second-order-polarization-mode-dispersion/instant", sopmd.GetInstant(), "SecondOrderPolarizationModeDispersion_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/second-order-polarization-mode-dispersion/min", sopmd.GetMin(), "SecondOrderPolarizationModeDispersion_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/second-order-polarization-mode-dispersion/max", sopmd.GetMax(), "SecondOrderPolarizationModeDispersion_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/second-order-polarization-mode-dispersion/avg", sopmd.GetAvg(), "SecondOrderPolarizationModeDispersion_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/second-order-polarization-mode-dispersion/min-time", sopmd.GetMinTime(), "SecondOrderPolarizationModeDispersion_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/second-order-polarization-mode-dispersion/max-time", sopmd.GetMaxTime(), "SecondOrderPolarizationModeDispersion_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/second-order-polarization-mode-dispersion/interval", sopmd.GetInterval(), "SecondOrderPolarizationModeDispersion_interval is empty for port %v")
		validatePMValue(t, transceiver, "SecondOrderPolarizationModeDispersion", sopmd.GetInstant(), sopmd.GetMin(), sopmd.GetMax(), sopmd.GetAvg())
	}

	if sop := optical_channel.GetSopRoc(); sop == nil {
		t.Errorf("SopRoc data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/sop-roc/instant", sop.GetInstant(), "SopRoc_instant is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/sop-roc/min", sop.GetMin(), "SopRoc_min is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/sop-roc/max", sop.GetMax(), "SopRoc_max is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/sop-roc/avg", sop.GetAvg(), "SopRoc_avg is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/sop-roc/min-time", sop.GetMinTime(), "SopRoc_mintime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/sop-roc/max-time", sop.GetMaxTime(), "SopRoc_maxtime is empty for port %v")
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/sop-roc/interval", sop.GetInterval(), "SopRoc_interval is empty for port %v")
		validatePMValue(t, transceiver, "SopRoc", sop.GetInstant(), sop.GetMin(), sop.GetMax(), sop.GetAvg())
	}

	if top := optical_channel.GetTargetOutputPower(); top == 0 {
		t.Errorf("TargetOutputPower data is empty for port %v", transceiver)
	} else {
		appendToTableIfNotNil(t, table, transceiver, "/components/component[name=*]/optical-channel/state/target-output-power", top, "TargetOutputPower is empty for port %v")
	}

	// TODO: threshold leaves
	// ths := gnmi.GetAll(t, dut, gnmi.OC().Component(transceiver).Transceiver().ThresholdAny().State())

	// for _, th := range ths {
	// 	if th.InputPowerLower == nil {
	// 		t.Errorf("Transceiver %s: threshold input-power-lower is nil", transceiver)
	// 	} else {
	// 		table.Append([]string{transceiver, "N/A", "InputPowerLower", strconv.FormatFloat(*th.InputPowerLower, 'f', 2, 64)})
	// 		t.Logf("Transceiver %s threshold input-power-lower: %v", transceiver, th.GetInputPowerLower())
	// 	}
	// }
	table.Render()
	return flag
}

// validatePMValue validates the pm value.
func validatePMValue(t *testing.T, portName string, pm string, instant float64, min float64, max float64, avg float64) {
	if instant < min || instant > max {
		t.Errorf("Invalid %v sample when %v is UP --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, min, max, avg, instant)
		return
	}
	t.Logf("Valid %v sample when %v is UP --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, min, max, avg, instant)
}

// appendToTableIfNotNil appends a row to the provided table if the given value is not nil.
// If the value is nil, it logs an error using the testing framework.
//
// Parameters:
//   - t: *testing.T - The testing object used for logging errors.
//   - table: *tablewriter.Table - The table to which the row will be appended.
//   - portName: string - The name of the port being processed.
//   - leaf: string - The leaf identifier to be added to the table.
//   - value: interface{} - The value to be added to the table (must not be nil).
//   - errMsg: string - The error message format string (expects portName as a formatting argument).
//
// Behavior:
//   - If `value` is nil, logs an error with `t.Errorf` and does not append to the table.
//   - If `value` is not nil, appends a row to `table` with `portName`, `leaf`, and the formatted `value`.
//
// Example Usage:
//
//	appendToTableIfNotNil(t, table, "eth0", "Index", device.GetAssignment(1).Index, "Index is empty for port %v")
func appendToTableIfNotNil(t *testing.T, table *tablewriter.Table, portName, leaf string, value interface{}, errMsg string) {
	if value == nil {
		t.Errorf(errMsg, portName)
		return
	}

	var formattedValue string
	switch v := value.(type) {
	case *float64:
		formattedValue = fmt.Sprintf("%f", *v) // Dereference pointer
	case *float32:
		formattedValue = fmt.Sprintf("%f", *v) // Dereference pointer
	case *int, *int8, *int16, *int64, *int32, *uint64, *uint32, *uint16, *uint8:
		formattedValue = fmt.Sprintf("%d", v) // Dereference pointer
	case *string:
		formattedValue = *v // Directly use the dereferenced string
	case *bool:
		formattedValue = fmt.Sprintf("%t", *v) // Dereference pointer
	case float64, float32:
		formattedValue = fmt.Sprintf("%f", v) // Handle non-pointer floats
	case int, int8, int64, int32, int16, uint32, uint8, uint16, uint64:
		formattedValue = fmt.Sprintf("%d", v) // Handle non-pointer integers
	case string:
		formattedValue = v // Handle strings
	case bool:
		formattedValue = fmt.Sprintf("%t", v) // Handle non-pointer bool
	default:
		formattedValue = fmt.Sprintf("%v", v) // Default fallback for any other type
	}

	table.Append([]string{portName, leaf, formattedValue})
}

func TestZRProcessRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	transceiverType := oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	transceivers := components.FindComponentsByType(t, dut, transceiverType)

	beforeStateMap := make(map[string][]*oc.Component_Transceiver_Channel)

	p1_name := strings.Join(strings.Split(dut.Port(t, "port1").Name(), "/")[1:], "/")
	p2_name := strings.Join(strings.Split(dut.Port(t, "port2").Name(), "/")[1:], "/")

	//Initial snapshot of leaves
	for _, transceiver := range transceivers {
		transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
		if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") && (strings.Contains(transceiver, p1_name) || strings.Contains(transceiver, p2_name)) {
			component := gnmi.OC().Component(transceiver)
			before_state := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().State())
			beforeStateMap[transceiver] = before_state
		}
	}

	// Iterate over the map (beforeStateMap)
	for transceiver, before_state := range beforeStateMap {
		success := checkleaves(t, dut, transceiver, before_state)
		if !success {
			t.Logf("Not all leaves are verified")
			t.Fatal()
		}
	}

	err := performance.RestartProcess(t, dut, "invmgr")
	if err != nil {
		t.Fatal(err)
	}

	afterStateMap := make(map[string][]*oc.Component_Transceiver_Channel)
	//Final snapshot of leaves
	for _, transceiver := range transceivers {
		transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
		if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") && (strings.Contains(transceiver, p1_name) || strings.Contains(transceiver, p2_name)) {
			component := gnmi.OC().Component(transceiver)
			after_state := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().State())
			afterStateMap[transceiver] = after_state
		}
	}

	// Iterate over the map (afterStateMap)
	for transceiver, after_state := range afterStateMap {
		success := checkleaves(t, dut, transceiver, after_state)
		if !success {
			t.Logf("Not all leaves are verified")
			t.Fatal()
		}
	}

	t.Logf("All leaves received successfully after process invmgr restart")

}

func TestZRLCReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lc := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
	t.Logf("%s", lc)
	transceiverType := oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	transceivers := components.FindComponentsByType(t, dut, transceiverType)

	//Using port1 for shut/unshut
	re := regexp.MustCompile(`\d+/\d+/\d+/\d+`)
	matches := re.FindStringSubmatch(dut.Port(t, "port1").Name())

	beforeStateMap := make(map[string][]*oc.Component_Transceiver_Channel)

	// Extracting only port1 key
	if len(matches) > 0 {
		extractedKey := matches[0]

		//Initial snapshot of leaves
		for _, transceiver := range transceivers {
			transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
			if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") && strings.HasSuffix(transceiver, extractedKey) {
				component := gnmi.OC().Component(transceiver)
				before_state := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().State())
				beforeStateMap[transceiver] = before_state
			}
		}
	}

	var LC string

	if len(matches) > 0 {
		extractedKey := matches[0]
		subSplitKey := strings.Split(extractedKey, "/")
		if len(subSplitKey) < 3 {
			fmt.Println("Invalid key format after splitting on Optics")
			t.Fatal()
		}
		lookup := strings.Join(subSplitKey[:2], "/") + "/CPU0"
		for _, item := range lc {
			if strings.Contains(item, lookup) {
				LC = item
				break
			}
		}
	}

	// Iterate over the map (beforeStateMap)
	for transceiver, before_state := range beforeStateMap {
		success := checkleaves(t, dut, transceiver, before_state)
		if !success {
			t.Logf("Not all leaves are verified")
			t.Fatal()
		}
	}

	t.Logf("Restarting LC %s", LC)
	util.ReloadLinecards(t, []string{LC})

	afterStateMap := make(map[string][]*oc.Component_Transceiver_Channel)

	if len(matches) > 0 {
		extractedKey := matches[0]

		//Final snapshot of leaves
		for _, transceiver := range transceivers {

			transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
			if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") && strings.HasSuffix(transceiver, extractedKey) {
				component := gnmi.OC().Component(transceiver)
				after_state := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().State())
				afterStateMap[transceiver] = after_state
			}
		}
	}

	// Iterate over the map (afterStateMap)
	for transceiver, after_state := range afterStateMap {
		success := checkleaves(t, dut, transceiver, after_state)
		if !success {
			t.Logf("Not all leaves are verified")
			t.Fatal()
		}
	}

	t.Logf("All Gnmi leaves received successfully after LC Reload")

}

func TestZRShutPort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	transceiverType := oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	transceivers := components.FindComponentsByType(t, dut, transceiverType)

	beforeStateMap := make(map[string][]*oc.Component_Transceiver_Channel)

	//Using port1 for shut/unshut
	re := regexp.MustCompile(`\d+/\d+/\d+/\d+`)
	matches := re.FindStringSubmatch(dut.Port(t, "port1").Name())

	// Extracting only port1 key
	if len(matches) > 0 {
		extractedKey := matches[0]

		//Initial snapshot of leaves
		for _, transceiver := range transceivers {
			transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
			if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") && strings.HasSuffix(transceiver, extractedKey) {
				component := gnmi.OC().Component(transceiver)
				before_state := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().State())
				beforeStateMap[transceiver] = before_state
			}
		}
	}

	// Iterate over the map (beforeStateMap)
	for transceiver, before_state := range beforeStateMap {
		success := checkleaves(t, dut, transceiver, before_state)
		if !success {
			t.Logf("Not all leaves are verified")
			t.Fatal()
		}
	}

	t.Logf("Shutting down the port %s", dut.Port(t, "port1").Name())
	cfgplugins.ToggleInterface(t, dut, dut.Port(t, "port1").Name(), false)

	afterStateMap := make(map[string][]*oc.Component_Transceiver_Channel)

	//Snapshot of leaves after trigger
	if len(matches) > 0 {
		extractedKey := matches[0]

		//Initial snapshot of leaves
		for _, transceiver := range transceivers {

			transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
			if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") && strings.HasSuffix(transceiver, extractedKey) {
				component := gnmi.OC().Component(transceiver)
				after_state := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().State())
				afterStateMap[transceiver] = after_state
			}
		}
	}

	// Iterate over the map (afterStateMap)
	for transceiver, after_state := range afterStateMap {
		success := checkleaves(t, dut, transceiver, after_state)
		if !success {
			t.Logf("Not all leaves are verified")
			t.Fatal()
		}
	}

	t.Logf("Un-Shutting down the port %s", dut.Port(t, "port1").Name())
	cfgplugins.ToggleInterface(t, dut, dut.Port(t, "port1").Name(), true)

	//Snapshot of leaves after trigger
	if len(matches) > 0 {
		extractedKey := matches[0]

		//Initial snapshot of leaves
		for _, transceiver := range transceivers {

			transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
			if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") && strings.HasSuffix(transceiver, extractedKey) {
				component := gnmi.OC().Component(transceiver)
				after_state := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().State())
				afterStateMap[transceiver] = after_state
			}
		}
	}

	// Iterate over the map (afterStateMap)
	for transceiver, after_state := range afterStateMap {
		success := checkleaves(t, dut, transceiver, after_state)
		if !success {
			t.Logf("Not all leaves are verified")
			t.Fatal()
		}
	}

	t.Logf("All Gnmi leaves received successfully after port shut")

}

func TestZRRPFO(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	transceiverType := oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	transceivers := components.FindComponentsByType(t, dut, transceiverType)

	p1_name := strings.Join(strings.Split(dut.Port(t, "port1").Name(), "/")[1:], "/")
	p2_name := strings.Join(strings.Split(dut.Port(t, "port2").Name(), "/")[1:], "/")

	beforeStateMap := make(map[string][]*oc.Component_Transceiver_Channel)
	//Initial snapshot of leaves
	for _, transceiver := range transceivers {
		transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
		if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") && (strings.Contains(transceiver, p1_name) || strings.Contains(transceiver, p2_name)) {
			// t.Logf("Transceiver %s has ZR optics", transceiver)
			component := gnmi.OC().Component(transceiver)
			before_state := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().State())
			beforeStateMap[transceiver] = before_state
		}
	}

	// Iterate over the map (beforeStateMap)
	for transceiver, before_state := range beforeStateMap {
		success := checkleaves(t, dut, transceiver, before_state)
		if !success {
			t.Logf("Not all leaves are verified")
			t.Fatal()
		}
	}

	// Do RPFO
	utils.Dorpfo(context.Background(), t, true)

	afterStateMap := make(map[string][]*oc.Component_Transceiver_Channel)
	//Final snapshot of leaves
	for _, transceiver := range transceivers {
		transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
		if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") && (strings.Contains(transceiver, p1_name) || strings.Contains(transceiver, p2_name)) {
			component := gnmi.OC().Component(transceiver)
			after_state := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().State())
			afterStateMap[transceiver] = after_state
		}
	}

	// Iterate over the map (afterStateMap)
	for transceiver, after_state := range afterStateMap {
		success := checkleaves(t, dut, transceiver, after_state)
		if !success {
			t.Logf("Not all leaves are verified")
			t.Fatal()
		}
	}

	t.Logf("All leaves received successfully after RPFO")

}
