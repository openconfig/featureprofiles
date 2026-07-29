package transceiver

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// validateHWPortTelemetry validates the hw port telemetry.
func validateHWPortTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, hwPortStream *samplestream.SampleStream[*oc.Component]) {
	if p.PMD() == ondatra.PMD400GBASEZR || p.PMD() == ondatra.PMD400GBASEZRP {
		// Skip HW Port validation for PMD400GBASEZR/PMD400GBASEZRP as it is not supported.
		return
	}
	if deviations.BreakoutModeUnsupportedForEightHundredGb(dut) && params.PortSpeed == oc.IfEthernet_ETHERNET_SPEED_SPEED_800GB {
		return
	}
	nextHWPortSample := hwPortStream.Next()
	hwPortValue, ok := nextHWPortSample.Val()
	if !ok {
		t.Fatalf("HW Port value is empty for port %v.", p.Name())
	}
	tcs := []testcase{
		{
			desc: "HW Port Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", params.HWPortNames[p.Name()]),
			got:  hwPortValue.GetName(),
			want: params.HWPortNames[p.Name()],
		},
		{
			desc:           "HW Port Location Validation",
			path:           fmt.Sprintf(componentPath+"/state/location", params.HWPortNames[p.Name()]),
			got:            hwPortValue.GetLocation(),
			patternToMatch: strings.Replace(strings.Replace(params.TransceiverNames[p.Name()], "Ethernet", "", 1), "-transceiver", "", 1),
		},
		{
			desc: "HW Port Type Validation",
			path: fmt.Sprintf(componentPath+"/state/type", params.HWPortNames[p.Name()]),
			got:  hwPortValue.GetType().(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT),
			want: oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT,
		},
		{
			desc: "HW Port Breakout Index Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/index", params.HWPortNames[p.Name()]),
			got:  hwPortValue.GetPort().GetBreakoutMode().GetGroup(1).GetIndex(),
			want: uint8(1),
		},
		{
			desc: "HW Port Breakout Speed Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/breakout-speed", params.HWPortNames[p.Name()]),
			got:  hwPortValue.GetPort().GetBreakoutMode().GetGroup(1).GetBreakoutSpeed(),
			want: params.PortSpeed,
		},
		{
			desc: "HW Port Number of Breakouts Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/num-breakouts", params.HWPortNames[p.Name()]),
			got:  hwPortValue.GetPort().GetBreakoutMode().GetGroup(1).GetNumBreakouts(),
			want: uint8(1),
		},
		{
			desc: "HW Port Number of Physical Channels Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/num-physical-channels", params.HWPortNames[p.Name()]),
			got:  hwPortValue.GetPort().GetBreakoutMode().GetGroup(1).GetNumPhysicalChannels(),
			want: params.NumPhysicalChannels,
		},
	}
	for _, tc := range tcs {
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
			default:
				if diff := cmp.Diff(tc.got, tc.want); diff != "" {
					t.Errorf("\n%s: %s, diff (-got +want):\n%s\n\n", p.Name(), tc.path, diff)
				}
			}
		})
	}
}
