package transceiver

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

// validateInterfaceTelemetry validates the interface telemetry.
func validateInterfaceTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus, interfaceStream *samplestream.SampleStream[*oc.Interface]) {
	nextInterfaceSample, ok := interfaceStream.AwaitNext(timeout, func(v *ygnmi.Value[*oc.Interface]) bool {
		val, present := v.Val()
		return present && val.GetOperStatus() == wantOperStatus
	})
	if !ok {
		t.Fatalf("Interface %v is not %v after %v minutes.", p.Name(), wantOperStatus, timeout.Minutes())
	}
	interfaceValue, ok := nextInterfaceSample.Val()
	if !ok {
		t.Fatalf("Interface value is empty for port %v.", p.Name())
	}
	tcs := []testcase{
		{
			desc: "Interface Name Validation",
			path: fmt.Sprintf(interfacePath+"/state/name", p.Name()),
			got:  interfaceValue.GetName(),
			want: p.Name(),
		},
		{
			desc: "Interface OperStatus Validation",
			path: fmt.Sprintf(interfacePath+"/state/oper-status", p.Name()),
			got:  interfaceValue.GetOperStatus(),
			want: wantOperStatus,
		},
		{
			desc: "Interface Type Validation",
			path: fmt.Sprintf(interfacePath+"/state/type", p.Name()),
			got:  interfaceValue.GetType(),
			want: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		},
		{
			desc: "Interface Enabled Validation",
			path: fmt.Sprintf(interfacePath+"/state/enabled", p.Name()),
			got:  interfaceValue.GetEnabled(),
			want: wantOperStatus == oc.Interface_OperStatus_UP,
		},
	}
	switch {
	case deviations.PortSpeedDuplexModeUnsupportedForInterfaceConfig(dut):
		// No port speed config for devices that do not support it.
	case p.PMD() == ondatra.PMD400GBASEZR || p.PMD() == ondatra.PMD400GBASEZRP:
		// No port speed config for 400GZR/400GZR Plus as it is not supported.
	default:
		tcs = append(tcs, []testcase{
			{
				desc: "Interface Ethernet Speed Validation",
				path: fmt.Sprintf(interfacePath+"/ethernet/state/port-speed", p.Name()),
				got:  interfaceValue.GetEthernet().GetPortSpeed(),
				want: params.PortSpeed,
			},
		}...)
	}
	for _, tc := range tcs {
		t.Run(fmt.Sprintf("%s of %v", tc.desc, p.Name()), func(t *testing.T) {
			t.Logf("\n%s: %s = %v\n\n", p.Name(), tc.path, tc.got)
			if diff := cmp.Diff(tc.got, tc.want); diff != "" {
				t.Errorf("\n%s: %s, diff (-got +want):\n%s\n\n", p.Name(), tc.path, diff)
			}
		})
	}
}
