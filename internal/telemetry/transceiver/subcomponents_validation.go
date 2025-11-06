package transceiver

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// validateTempSensorTelemetry validates the temperature sensor telemetry.
func validateTempSensorTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus, temperatureSensorStream *samplestream.SampleStream[*oc.Component]) {
	if dut.Vendor() != ondatra.NOKIA {
		return
	}
	nextTemperatureSensorSample := temperatureSensorStream.Next()
	temperatureSensorValue, ok := nextTemperatureSensorSample.Val()
	if !ok {
		t.Fatalf("Sensor Component value is empty for port %v.", p.Name())
	}
	tcs := []testcase{
		{
			desc: "Temperature Sensor Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", params.TempSensorNames[p.Name()]),
			got:  temperatureSensorValue.GetName(),
			want: params.TempSensorNames[p.Name()],
		},
		{
			desc:       "Temperature Sensor Instant Temperature Validation",
			path:       fmt.Sprintf(componentPath+"/state/temperature/instant", params.TempSensorNames[p.Name()]),
			got:        temperatureSensorValue.GetTemperature().GetInstant(),
			minAllowed: minAllowedTemperature,
			maxAllowed: maxAllowedTemperature,
		},
		{
			desc:       "Temperature Sensor Max Temperature Validation",
			path:       fmt.Sprintf(componentPath+"/state/temperature/instant", params.TempSensorNames[p.Name()]),
			got:        temperatureSensorValue.GetTemperature().GetMax(),
			minAllowed: minAllowedTemperature,
			maxAllowed: maxAllowedTemperature,
		},
		{
			desc: "Temperature Sensor Temperature Alarm Status Validation",
			path: fmt.Sprintf(componentPath+"/state/temperature/instant", params.TempSensorNames[p.Name()]),
			got:  temperatureSensorValue.GetTemperature().GetAlarmStatus(),
			want: false,
		},
	}
	for _, tc := range tcs {
		if tc.operStatus != oc.Interface_OperStatus_UNSET && tc.operStatus != wantOperStatus {
			// Skip the validation if the operStatus is not the same as the expected operStatus.
			continue
		}
		t.Run(fmt.Sprintf("%s of %v", tc.desc, p.Name()), func(t *testing.T) {
			t.Logf("\n%s: %s = %v\n\n", p.Name(), tc.path, tc.got)
			switch {
			case tc.want != nil:
				if diff := cmp.Diff(tc.got, tc.want); diff != "" {
					t.Errorf("\n%s: %s, diff (-got +want):\n%s\n\n", p.Name(), tc.path, diff)
				}
			default:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", p.Name(), tc.path, tc.got)
				}
				if val < tc.minAllowed || val > tc.maxAllowed {
					t.Errorf("\n%s: %s, out of range:\n got %v want >= %v, <= %v\n\n", p.Name(), tc.path, tc.got, tc.minAllowed, tc.maxAllowed)
				}
			}
		})
	}
}
