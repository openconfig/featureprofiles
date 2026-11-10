// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package polarization

import (
	"math"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func makeValidPolarizationComponent(ochName string, opMode uint16, inPower, outPower, pmdBase, sopmdBase, sopROCBase float64) *oc.Component {
	return &oc.Component{
		Name: ygot.String(ochName),
		OpticalChannel: &oc.Component_OpticalChannel{
			OperationalMode: ygot.Uint16(opMode),
			InputPower: &oc.Component_OpticalChannel_InputPower{
				Instant: ygot.Float64(inPower),
				Avg:     ygot.Float64(inPower),
				Min:     ygot.Float64(inPower - 0.5),
				Max:     ygot.Float64(inPower + 0.5),
			},
			OutputPower: &oc.Component_OpticalChannel_OutputPower{
				Instant: ygot.Float64(outPower),
				Avg:     ygot.Float64(outPower),
				Min:     ygot.Float64(outPower - 0.2),
				Max:     ygot.Float64(outPower + 0.2),
			},
			PolarizationModeDispersion: &oc.Component_OpticalChannel_PolarizationModeDispersion{
				Instant: ygot.Float64(pmdBase + 0.5),
				Avg:     ygot.Float64(pmdBase + 0.4),
				Min:     ygot.Float64(pmdBase),
				Max:     ygot.Float64(pmdBase + 1.0),
			},
			SecondOrderPolarizationModeDispersion: &oc.Component_OpticalChannel_SecondOrderPolarizationModeDispersion{
				Instant: ygot.Float64(sopmdBase + 5.0),
				Avg:     ygot.Float64(sopmdBase + 4.0),
				Min:     ygot.Float64(sopmdBase),
				Max:     ygot.Float64(sopmdBase + 10.0),
			},
			SopRoc: &oc.Component_OpticalChannel_SopRoc{
				Instant: ygot.Float64(sopROCBase + 25.0),
				Avg:     ygot.Float64(sopROCBase + 20.0),
				Min:     ygot.Float64(sopROCBase),
				Max:     ygot.Float64(sopROCBase + 50.0),
			},
		},
	}
}

func makeUniformPolarizationComponent(ochName string, opMode uint16, inPower, outPower, val float64) *oc.Component {
	return &oc.Component{
		Name: ygot.String(ochName),
		OpticalChannel: &oc.Component_OpticalChannel{
			OperationalMode: ygot.Uint16(opMode),
			InputPower: &oc.Component_OpticalChannel_InputPower{
				Instant: ygot.Float64(inPower),
				Avg:     ygot.Float64(inPower),
				Min:     ygot.Float64(inPower),
				Max:     ygot.Float64(inPower),
			},
			OutputPower: &oc.Component_OpticalChannel_OutputPower{
				Instant: ygot.Float64(outPower),
				Avg:     ygot.Float64(outPower),
				Min:     ygot.Float64(outPower),
				Max:     ygot.Float64(outPower),
			},
			PolarizationModeDispersion: &oc.Component_OpticalChannel_PolarizationModeDispersion{
				Instant: ygot.Float64(val),
				Avg:     ygot.Float64(val),
				Min:     ygot.Float64(val),
				Max:     ygot.Float64(val),
			},
			SecondOrderPolarizationModeDispersion: &oc.Component_OpticalChannel_SecondOrderPolarizationModeDispersion{
				Instant: ygot.Float64(val),
				Avg:     ygot.Float64(val),
				Min:     ygot.Float64(val),
				Max:     ygot.Float64(val),
			},
			SopRoc: &oc.Component_OpticalChannel_SopRoc{
				Instant: ygot.Float64(val),
				Avg:     ygot.Float64(val),
				Min:     ygot.Float64(val),
				Max:     ygot.Float64(val),
			},
		},
	}
}

func TestValidateOpticalChannelPolarizationComponent(t *testing.T) {
	params := &cfgplugins.ConfigParameters{
		OperationalMode:    1,
		TargetOpticalPower: -7.0,
		OpticalChannelNames: map[string]string{
			"port1": "Ethernet4/1-Optical0",
		},
	}

	upComp := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 20.0, 100.0)
	ValidateOpticalChannelPolarizationComponent(t, "port1", upComp, params, oc.Interface_OperStatus_UP)

	downComp := makeUniformPolarizationComponent("Ethernet4/1-Optical0", 1, -40.0, -40.0, 0.0)
	ValidateOpticalChannelPolarizationComponent(t, "port1", downComp, params, oc.Interface_OperStatus_DOWN)
}

func TestValidateOpticalChannelPolarization(t *testing.T) {
	params := &cfgplugins.ConfigParameters{
		OperationalMode:    1,
		TargetOpticalPower: -7.0,
		OpticalChannelNames: map[string]string{
			"port1": "Ethernet4/1-Optical0",
		},
	}

	tests := []struct {
		name           string
		buildComponent func() *oc.Component
		params         *cfgplugins.ConfigParameters
		wantOperStatus oc.E_Interface_OperStatus
		wantErrors     int
		wantErrSubstr  string
	}{
		{
			name: "UP valid baseline",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 2.0, 50.0, 200.0)
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     0,
		},
		{
			name: "UP valid tolerance boundary",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 10.0, 50.0, 200.0)
				c.OpticalChannel.PolarizationModeDispersion = &oc.Component_OpticalChannel_PolarizationModeDispersion{
					Min:     ygot.Float64(10.0),
					Max:     ygot.Float64(20.0),
					Instant: ygot.Float64(9.5),  // 10 - 10*0.05 = 9.5
					Avg:     ygot.Float64(21.0), // 20 + 20*0.05 = 21.0
				}
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     0,
		},
		{
			name: "UP fallback component name without params",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
			},
			params:         nil,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     0,
		},
		{
			name:           "nil component fails",
			buildComponent: func() *oc.Component { return nil },
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     1,
			wantErrSubstr:  "optical-channel component/state is nil",
		},
		{
			name: "nil optical channel container fails",
			buildComponent: func() *oc.Component {
				return &oc.Component{Name: ygot.String("Ethernet4/1-Optical0")}
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     1,
			wantErrSubstr:  "optical-channel component/state is nil",
		},
		{
			name: "unsupported target oper status fails",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_TESTING,
			wantErrors:     1,
			wantErrSubstr:  "unsupported target oper status",
		},
		{
			name: "UP missing PMD container fails",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.PolarizationModeDispersion = nil
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     1,
			wantErrSubstr:  "container is nil while oper-status is UP",
		},
		{
			name: "UP missing SOPMD container fails",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.SecondOrderPolarizationModeDispersion = nil
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     1,
			wantErrSubstr:  "container is nil while oper-status is UP",
		},
		{
			name: "UP missing SOP-ROC container fails",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.SopRoc = nil
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     1,
			wantErrSubstr:  "container is nil while oper-status is UP",
		},
		{
			name: "UP missing single leaf fails",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.PolarizationModeDispersion.Instant = nil
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     1,
			wantErrSubstr:  "leaf is nil while oper-status is UP",
		},
		{
			name: "UP NaN leaf fails",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.PolarizationModeDispersion.Instant = ygot.Float64(math.NaN())
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     1,
			wantErrSubstr:  "invalid non-finite float64 value",
		},
		{
			name: "UP PMD out of range fails",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 200.5, 10.0, 50.0)
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     4,
			wantErrSubstr:  "out of range: got",
		},
		{
			name: "UP negative PMD fails",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.PolarizationModeDispersion.Min = ygot.Float64(-0.1)
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     1,
			wantErrSubstr:  "out of range: got -0.1, want >= 0 and <= 200",
		},
		{
			name: "UP SOPMD out of range fails",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 2001.0, 50.0)
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     4,
			wantErrSubstr:  "out of range: got",
		},
		{
			name: "UP SOP-ROC out of range fails",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 5001.0)
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     4,
			wantErrSubstr:  "out of range: got",
		},
		{
			name: "UP instant outside statistical tolerance fails",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 10.0, 10.0, 50.0)
				c.OpticalChannel.SopRoc = &oc.Component_OpticalChannel_SopRoc{
					Min:     ygot.Float64(100.0),
					Max:     ygot.Float64(200.0),
					Avg:     ygot.Float64(150.0),
					Instant: ygot.Float64(211.0), // > 200 + 200*0.05 (210)
				}
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     1,
			wantErrSubstr:  "outside statistical bounds",
		},
		{
			name: "UP inverted min > max fails",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 10.0, 10.0, 50.0)
				c.OpticalChannel.SecondOrderPolarizationModeDispersion = &oc.Component_OpticalChannel_SecondOrderPolarizationModeDispersion{
					Min:     ygot.Float64(50.0),
					Max:     ygot.Float64(10.0),
					Avg:     ygot.Float64(20.0),
					Instant: ygot.Float64(20.0),
				}
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_UP,
			wantErrors:     2,
			wantErrSubstr:  "invalid statistical range: min (50) > max (10)",
		},
		{
			name: "DOWN valid zeros",
			buildComponent: func() *oc.Component {
				return &oc.Component{
					Name: ygot.String("Ethernet4/1-Optical0"),
					OpticalChannel: &oc.Component_OpticalChannel{
						OperationalMode: ygot.Uint16(1),
						PolarizationModeDispersion: &oc.Component_OpticalChannel_PolarizationModeDispersion{
							Instant: ygot.Float64(0.0),
							Avg:     ygot.Float64(0.0),
							Min:     ygot.Float64(0.0),
							Max:     ygot.Float64(0.0),
						},
						SecondOrderPolarizationModeDispersion: &oc.Component_OpticalChannel_SecondOrderPolarizationModeDispersion{
							Instant: ygot.Float64(0.0),
							Avg:     ygot.Float64(0.0),
							Min:     ygot.Float64(0.0),
							Max:     ygot.Float64(0.0),
						},
						SopRoc: &oc.Component_OpticalChannel_SopRoc{
							Instant: ygot.Float64(0.0),
							Avg:     ygot.Float64(0.0),
							Min:     ygot.Float64(0.0),
							Max:     ygot.Float64(0.0),
						},
					},
				}
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_DOWN,
			wantErrors:     0,
		},
		{
			name: "DOWN valid boundary at inactive limit",
			buildComponent: func() *oc.Component {
				return &oc.Component{
					Name: ygot.String("Ethernet4/1-Optical0"),
					OpticalChannel: &oc.Component_OpticalChannel{
						OperationalMode: ygot.Uint16(1),
						PolarizationModeDispersion: &oc.Component_OpticalChannel_PolarizationModeDispersion{
							Instant: ygot.Float64(1.0),
							Avg:     ygot.Float64(1.0),
							Min:     ygot.Float64(1.0),
							Max:     ygot.Float64(1.0),
						},
						SecondOrderPolarizationModeDispersion: &oc.Component_OpticalChannel_SecondOrderPolarizationModeDispersion{
							Instant: ygot.Float64(1.0),
							Avg:     ygot.Float64(1.0),
							Min:     ygot.Float64(1.0),
							Max:     ygot.Float64(1.0),
						},
						SopRoc: &oc.Component_OpticalChannel_SopRoc{
							Instant: ygot.Float64(1.0),
							Avg:     ygot.Float64(1.0),
							Min:     ygot.Float64(1.0),
							Max:     ygot.Float64(1.0),
						},
					},
				}
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_DOWN,
			wantErrors:     0,
		},
		{
			name: "DOWN omitted containers pass",
			buildComponent: func() *oc.Component {
				return &oc.Component{
					Name:           ygot.String("Ethernet4/1-Optical0"),
					OpticalChannel: &oc.Component_OpticalChannel{},
				}
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_DOWN,
			wantErrors:     0,
		},
		{
			name: "DOWN stale values above inactive threshold fail",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -40.0, -40.0, 2.0, 20.0, 100.0)
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_DOWN,
			wantErrors:     12,
			wantErrSubstr:  "out of range while DOWN",
		},
		{
			name: "DOWN NaN leaf fails",
			buildComponent: func() *oc.Component {
				c := makeUniformPolarizationComponent("Ethernet4/1-Optical0", 1, -40.0, -40.0, 0.0)
				c.OpticalChannel.SopRoc.Max = ygot.Float64(math.Inf(1))
				return c
			},
			params:         params,
			wantOperStatus: oc.Interface_OperStatus_DOWN,
			wantErrors:     1,
			wantErrSubstr:  "invalid non-finite float64 value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checks := validateOpticalChannelPolarization("port1", tc.buildComponent(), tc.params, tc.wantOperStatus)
			var errCount int
			var errMsgs []string
			for _, c := range checks {
				if c.err != nil {
					errCount++
					errMsgs = append(errMsgs, c.err.Error())
				}
			}
			if errCount != tc.wantErrors {
				t.Fatalf("validateOpticalChannelPolarization() returned %d errors (%v), want %d", errCount, errMsgs, tc.wantErrors)
			}
			if tc.wantErrSubstr != "" {
				joined := strings.Join(errMsgs, "; ")
				if !strings.Contains(joined, tc.wantErrSubstr) {
					t.Fatalf("error messages %q do not contain %q", joined, tc.wantErrSubstr)
				}
			}
		})
	}
}

func TestIsPolarizationStreamReady(t *testing.T) {
	params := &cfgplugins.ConfigParameters{
		OperationalMode:    1,
		TargetOpticalPower: -7.0,
	}

	tests := []struct {
		name           string
		buildComponent func() *oc.Component
		params         *cfgplugins.ConfigParameters
		operStatus     oc.E_Interface_OperStatus
		want           bool
	}{
		{
			name: "UP ready",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       true,
		},
		{
			name:           "nil component",
			buildComponent: func() *oc.Component { return nil },
			params:         params,
			operStatus:     oc.Interface_OperStatus_UP,
			want:           false,
		},
		{
			name: "nil optical channel",
			buildComponent: func() *oc.Component {
				return &oc.Component{Name: ygot.String("Ethernet4/1-Optical0")}
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "nil params",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
			},
			params:     nil,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "mismatched operational mode",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 2, -7.5, -7.0, 1.0, 10.0, 50.0)
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "missing input power container",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.InputPower = nil
				return c
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "missing output power leaf",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.OutputPower.Max = nil
				return c
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "UP missing PMD container",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.PolarizationModeDispersion = nil
				return c
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "UP missing SOPMD container",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.SecondOrderPolarizationModeDispersion = nil
				return c
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "UP missing SOP-ROC container",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.SopRoc = nil
				return c
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "UP low input power not ready",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -20.0, -7.0, 1.0, 10.0, 50.0)
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "UP output power out of tolerance",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -7.5, -7.0, 1.0, 10.0, 50.0)
				c.OpticalChannel.OutputPower.Instant = ygot.Float64(-1.0)
				return c
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_UP,
			want:       false,
		},
		{
			name: "DOWN ready",
			buildComponent: func() *oc.Component {
				c := makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -40.0, -40.0, 0.0, 0.0, 0.0)
				c.OpticalChannel.PolarizationModeDispersion = nil
				c.OpticalChannel.SecondOrderPolarizationModeDispersion = nil
				c.OpticalChannel.SopRoc = nil
				return c
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       true,
		},
		{
			name: "DOWN power still high not ready",
			buildComponent: func() *oc.Component {
				return makeValidPolarizationComponent("Ethernet4/1-Optical0", 1, -10.0, -10.0, 0.0, 0.0, 0.0)
			},
			params:     params,
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsPolarizationStreamReady(tc.buildComponent(), tc.operStatus, tc.params)
			if got != tc.want {
				t.Fatalf("IsPolarizationStreamReady() = %v, want %v", got, tc.want)
			}
		})
	}
}
