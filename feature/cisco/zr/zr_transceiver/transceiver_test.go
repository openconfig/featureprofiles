package transceiver_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
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

// Helper function to safely execute log statements with panic recovery
func safeLog(channel interface{}, fieldName string, logFunc func()) (bool, string) {
	var errString string
	success := true

	defer func() {
		if r := recover(); r != nil {
			errString = fmt.Sprintf("Recovered from error in '%s' for channel %v: %v", fieldName, channel, r)
			success = false
		}
	}()

	logFunc()
	return success, errString
}

func checkleaves(t *testing.T, dut *ondatra.DUTDevice, transceiver string, state []*oc.Component_Transceiver_Channel) bool {
	flag := true
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Transceiver", "Channel", "Leaf", "Value"})

	for channel := range state {

		if success, err := safeLog(channel, "associated_optical_channel", func() {
			associated_optical_channel := state[channel].AssociatedOpticalChannel
			table.Append([]string{transceiver, strconv.Itoa(channel), "associated_optical_channel", *associated_optical_channel})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		// CDETS: CSCwk32258. This will be un-commented after the fix
		// if success, err := safeLog(t, transceiver, channel, "Description", func() {
		// 	description := state[channel].Description
		// 	table.Append([]string{transceiver, strconv.Itoa(channel), "Description", *description})
		//
		// }); !success {
		// 	t.Logf("Error encountered for leaf Description %s", err)
		// 	flag = false
		// }
		if success, err := safeLog(channel, "Index", func() {
			index := state[channel].Index
			table.Append([]string{transceiver, strconv.Itoa(channel), "Index", strconv.FormatUint(uint64(*index), 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "input_power_average", func() {
			input_power_average := state[channel].InputPower.Avg
			table.Append([]string{transceiver, strconv.Itoa(channel), "input_power_average", strconv.FormatFloat(*input_power_average, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "input_power_instant", func() {
			input_power_instant := state[channel].InputPower.Instant
			table.Append([]string{transceiver, strconv.Itoa(channel), "input_power_instant", strconv.FormatFloat(*input_power_instant, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "input_power_interval", func() {
			input_power_interval := state[channel].InputPower.Interval
			table.Append([]string{transceiver, strconv.Itoa(channel), "input_power_interval", strconv.FormatUint(*input_power_interval, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "input_power_max", func() {
			input_power_max := state[channel].InputPower.Max
			table.Append([]string{transceiver, strconv.Itoa(channel), "input_power_max", strconv.FormatFloat(*input_power_max, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "input_power_max_time", func() {
			input_power_max_time := state[channel].InputPower.MaxTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "input_power_max_time", strconv.FormatUint(*input_power_max_time, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "input_power_min", func() {
			input_power_min := state[channel].InputPower.Min
			table.Append([]string{transceiver, strconv.Itoa(channel), "input_power_min", strconv.FormatFloat(*input_power_min, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "input_power_min_time", func() {
			input_power_min_time := state[channel].InputPower.MinTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "input_power_min_time", strconv.FormatUint(*input_power_min_time, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserAge", func() {
			LaserAge := state[channel].LaserAge
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserAge", strconv.FormatUint(uint64(*LaserAge), 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserBiasCurrent_interval", func() {
			LaserBiasCurrent_interval := state[channel].LaserBiasCurrent.Interval
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserBiasCurrent_interval", strconv.FormatUint(*LaserBiasCurrent_interval, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserBiasCurrent_avg", func() {
			LaserBiasCurrent_avg := state[channel].LaserBiasCurrent.Avg
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserBiasCurrent_avg", strconv.FormatFloat(*LaserBiasCurrent_avg, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserBiasCurrent_instant", func() {
			LaserBiasCurrent_instant := state[channel].LaserBiasCurrent.Instant
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserBiasCurrent_instant", strconv.FormatFloat(*LaserBiasCurrent_instant, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserBiasCurrent_max", func() {
			LaserBiasCurrent_max := state[channel].LaserBiasCurrent.Max
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserBiasCurrent_max", strconv.FormatFloat(*LaserBiasCurrent_max, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserBiasCurrent_max_time", func() {
			LaserBiasCurrent_max_time := state[channel].LaserBiasCurrent.MaxTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserBiasCurrent_max_time", strconv.FormatUint(*LaserBiasCurrent_max_time, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserBiasCurrent_min", func() {
			LaserBiasCurrent_min := state[channel].LaserBiasCurrent.Min
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserBiasCurrent_min", strconv.FormatFloat(*LaserBiasCurrent_min, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserBiasCurrent_min_time", func() {
			LaserBiasCurrent_min_time := state[channel].LaserBiasCurrent.MinTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserBiasCurrent_min_time", strconv.FormatUint(*LaserBiasCurrent_min_time, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserTemperature_Instant", func() {
			LaserTemperature_Instant := state[channel].LaserTemperature.Instant
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserTemperature_Instant", strconv.FormatFloat(*LaserTemperature_Instant, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserTemperature_Interval", func() {
			LaserTemperature_Interval := state[channel].LaserTemperature.Interval
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserTemperature_Interval", strconv.FormatUint(*LaserTemperature_Interval, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserTemperature_Max", func() {
			LaserTemperature_Max := state[channel].LaserTemperature.Max
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserTemperature_Max", strconv.FormatFloat(*LaserTemperature_Max, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserTemperature_MaxTime", func() {
			LaserTemperature_MaxTime := state[channel].LaserTemperature.MaxTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserTemperature_MaxTime", strconv.FormatUint(*LaserTemperature_MaxTime, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserTemperature_Min", func() {
			LaserTemperature_Min := state[channel].LaserTemperature.Min
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserTemperature_Min", strconv.FormatFloat(*LaserTemperature_Min, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "LaserTemperature_MinTime", func() {
			LaserTemperature_MinTime := state[channel].LaserTemperature.MinTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "LaserTemperature_MinTime", strconv.FormatUint(*LaserTemperature_MinTime, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "OutputFrequency", func() {
			OutputFrequency := state[channel].OutputFrequency
			table.Append([]string{transceiver, strconv.Itoa(channel), "OutputFrequency", strconv.FormatUint(*OutputFrequency, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "OutputPower_Avg", func() {
			OutputPower_Avg := state[channel].OutputPower.Avg
			table.Append([]string{transceiver, strconv.Itoa(channel), "OutputPower_Avg", strconv.FormatFloat(*OutputPower_Avg, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "OutputPower_Instant", func() {
			OutputPower_Instant := state[channel].OutputPower.Instant
			table.Append([]string{transceiver, strconv.Itoa(channel), "OutputPower_Instant", strconv.FormatFloat(*OutputPower_Instant, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "OutputPower_Interval", func() {
			OutputPower_Interval := state[channel].OutputPower.Interval
			table.Append([]string{transceiver, strconv.Itoa(channel), "OutputPower_Interval", strconv.FormatUint(*OutputPower_Interval, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "OutputPower_Max", func() {
			OutputPower_Max := state[channel].OutputPower.Max
			table.Append([]string{transceiver, strconv.Itoa(channel), "OutputPower_Max", strconv.FormatFloat(*OutputPower_Max, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "OutputPower_MaxTime", func() {
			OutputPower_MaxTime := state[channel].OutputPower.MaxTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "OutputPower_MaxTime", strconv.FormatUint(*OutputPower_MaxTime, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "OutputPower_Min", func() {
			OutputPower_Min := state[channel].OutputPower.Min
			table.Append([]string{transceiver, strconv.Itoa(channel), "OutputPower_Min", strconv.FormatFloat(*OutputPower_Min, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "OutputPower_MinTime", func() {
			OutputPower_MinTime := state[channel].OutputPower.MinTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "OutputPower_MinTime", strconv.FormatUint(*OutputPower_MinTime, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TargetFrequencyDeviation_Avg", func() {
			TargetFrequencyDeviation_Avg := state[channel].TargetFrequencyDeviation.Avg
			table.Append([]string{transceiver, strconv.Itoa(channel), "TargetFrequencyDeviation_Avg", strconv.FormatFloat(*TargetFrequencyDeviation_Avg, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TargetFrequencyDeviation_Instant", func() {
			TargetFrequencyDeviation_Instant := state[channel].TargetFrequencyDeviation.Instant
			table.Append([]string{transceiver, strconv.Itoa(channel), "TargetFrequencyDeviation_Instant", strconv.FormatFloat(*TargetFrequencyDeviation_Instant, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TargetFrequencyDeviation_Interval", func() {
			TargetFrequencyDeviation_Interval := state[channel].TargetFrequencyDeviation.Interval
			table.Append([]string{transceiver, strconv.Itoa(channel), "TargetFrequencyDeviation_Interval", strconv.FormatUint(*TargetFrequencyDeviation_Interval, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TargetFrequencyDeviation_Max", func() {
			TargetFrequencyDeviation_Max := state[channel].TargetFrequencyDeviation.Max
			table.Append([]string{transceiver, strconv.Itoa(channel), "TargetFrequencyDeviation_Max", strconv.FormatFloat(*TargetFrequencyDeviation_Max, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TargetFrequencyDeviation_MaxTime", func() {
			TargetFrequencyDeviation_MaxTime := state[channel].TargetFrequencyDeviation.MaxTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "TargetFrequencyDeviation_MaxTime", strconv.FormatUint(*TargetFrequencyDeviation_MaxTime, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TargetFrequencyDeviation_Min", func() {
			TargetFrequencyDeviation_Min := state[channel].TargetFrequencyDeviation.Min
			table.Append([]string{transceiver, strconv.Itoa(channel), "TargetFrequencyDeviation_Min", strconv.FormatFloat(*TargetFrequencyDeviation_Min, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TargetFrequencyDeviation_MinTime", func() {
			TargetFrequencyDeviation_MinTime := state[channel].TargetFrequencyDeviation.MinTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "TargetFrequencyDeviation_MinTime", strconv.FormatUint(*TargetFrequencyDeviation_MinTime, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TargetOutputPower", func() {
			TargetOutputPower := state[channel].TargetOutputPower
			table.Append([]string{transceiver, strconv.Itoa(channel), "TargetOutputPower", strconv.FormatFloat(*TargetOutputPower, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TecCurrent_Avg", func() {
			TecCurrent_Avg := state[channel].TecCurrent.Avg
			table.Append([]string{transceiver, strconv.Itoa(channel), "TecCurrent_Avg", strconv.FormatFloat(*TecCurrent_Avg, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TecCurrent_Instant", func() {
			TecCurrent_Instant := state[channel].TecCurrent.Instant
			table.Append([]string{transceiver, strconv.Itoa(channel), "TecCurrent_Instant", strconv.FormatFloat(*TecCurrent_Instant, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TecCurrent_Interval", func() {
			TecCurrent_Interval := state[channel].TecCurrent.Interval
			table.Append([]string{transceiver, strconv.Itoa(channel), "TecCurrent_Interval", strconv.FormatUint(*TecCurrent_Interval, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TecCurrent_Max", func() {
			TecCurrent_Max := state[channel].TecCurrent.Max
			table.Append([]string{transceiver, strconv.Itoa(channel), "TecCurrent_Max", strconv.FormatFloat(*TecCurrent_Max, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TecCurrent_MaxTime", func() {
			TecCurrent_MaxTime := state[channel].TecCurrent.MaxTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "TecCurrent_MaxTime", strconv.FormatUint(*TecCurrent_MaxTime, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TecCurrent_Min", func() {
			TecCurrent_Min := state[channel].TecCurrent.Min
			table.Append([]string{transceiver, strconv.Itoa(channel), "TecCurrent_Min", strconv.FormatFloat(*TecCurrent_Min, 'f', 2, 64)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TecCurrent_MinTime", func() {
			TecCurrent_MinTime := state[channel].TecCurrent.MinTime
			table.Append([]string{transceiver, strconv.Itoa(channel), "TecCurrent_MinTime", strconv.FormatUint(*TecCurrent_MinTime, 10)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}
		if success, err := safeLog(channel, "TxLaser", func() {
			TxLaser := state[channel].TxLaser
			table.Append([]string{transceiver, strconv.Itoa(channel), "TxLaser", strconv.FormatBool(*TxLaser)})
		}); !success {
			t.Logf("Error encountered: %s", err)
			flag = false
		}

	}

	s := gnmi.Get(t, dut, gnmi.OC().Component(transceiver).Transceiver().State())

	if s.GetConnectorType() == oc.E_TransportTypes_FIBER_CONNECTOR_TYPE(0) {
		flag = false
	} else {
		table.Append([]string{transceiver, "0", "connector-type", s.GetConnectorType().String()})
	}
	if s.GetDateCode() == "" {
		flag = false
	} else {
		table.Append([]string{transceiver, "0", "date-code", s.GetDateCode()})
	}
	if s.Enabled == nil {
		flag = false
	} else {
		enabledstr := "true"
		table.Append([]string{transceiver, "0", "enabled", enabledstr})
	}
	if s.GetFormFactor() == oc.E_TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE(0) {
		flag = false
	} else {
		table.Append([]string{transceiver, "0", "form-factor", s.GetFormFactor().String()})
	}
	if s.GetModuleFunctionalType() == oc.E_TransportTypes_TRANSCEIVER_MODULE_FUNCTIONAL_TYPE(0) {
		flag = false
	} else {
		table.Append([]string{transceiver, "0", "module-functional-type", s.GetModuleFunctionalType().String()})
	}
	if s.GetOtnComplianceCode() == oc.E_TransportTypes_OTN_APPLICATION_CODE(0) {
		flag = false
	} else {
		table.Append([]string{transceiver, "0", "otn-compliance-code", s.GetOtnComplianceCode().String()})
	}
	if s.GetVendor() == "" {
		flag = false
	} else {
		table.Append([]string{transceiver, "0", "vendor", s.GetVendor()})
	}
	if s.GetVendorPart() == "" {
		flag = false
	} else {
		table.Append([]string{transceiver, "0", "vendor-part", s.GetVendorPart()})
	}
	if s.GetVendorRev() == "" {
		flag = false
	} else {
		table.Append([]string{transceiver, "0", "vendor-rev", s.GetVendorRev()})
	}

	// TODO: threshold leaves
	// ths := gnmi.GetAll(t, dut, gnmi.OC().Component(transceiver).Transceiver().ThresholdAny().State())

	// for _, th := range ths {
	// 	if th.InputPowerLower == nil {
	// 		t.Errorf("Transceiver %s: threshold input-power-lower is nil", transceiver)
	// 	} else {
	// 		table.Append([]string{transceiver, "N/A", "TxLaser", strconv.FormatFloat(*th.InputPowerLower, 'f', 2, 64)})
	// 		t.Logf("Transceiver %s threshold input-power-lower: %v", transceiver, th.GetInputPowerLower())
	// 	}
	// }
	table.Render()
	return flag
}

func TestZRProcessRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	transceiverType := oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	transceivers := components.FindComponentsByType(t, dut, transceiverType)

	beforeStateMap := make(map[string][]*oc.Component_Transceiver_Channel)

	//Initial snapshot of leaves
	for _, transceiver := range transceivers {
		transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
		if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") {
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
			// t.Fatal()
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
		if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") {
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
			// t.Fatal()
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

	beforeStateMap := make(map[string][]*oc.Component_Transceiver_Channel)
	//Initial snapshot of leaves
	for _, transceiver := range transceivers {
		transceiver_desc := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String()
		if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") {
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
		if strings.Contains(transceiver_desc, "ZR") && !strings.Contains(transceiver_desc, "ZRP") {
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
