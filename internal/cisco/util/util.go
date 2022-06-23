package util

import (
	"context"
	"net"
	"testing"
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

// FlapInterface flaps Interface and check State
func FlapInterface(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, flapDuration time.Duration) {

	initialState := dut.Telemetry().Interface(interfaceName).Get(t).GetEnabled()
	transientState := !initialState
	SetInterfaceState(t, dut, interfaceName, transientState)
	time.Sleep(flapDuration * time.Second)
	SetInterfaceState(t, dut, interfaceName, initialState)
}

// SetInterfaceState sets interface adminState
func SetInterfaceState(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, adminState bool) {

	i := &telemetry.Interface{
		Enabled: ygot.Bool(adminState),
		Name:    ygot.String(interfaceName),
	}
	updateResponse := dut.Config().Interface(interfaceName).Update(t, i)
	t.Logf("Update response : %v", updateResponse)
	currEnabledState := dut.Telemetry().Interface(interfaceName).Get(t).GetEnabled()
	if currEnabledState != adminState {
		t.Fatalf("Failed to set interface adminState to :%v", adminState)
	} else {
		t.Logf("Interface adminState set to :%v", adminState)
	}
}

// GetIPPrefix returns the ip range with prefix
func GetIPPrefix(IPAddr string, i int, prefixLen string) string {
	ip := net.ParseIP(IPAddr)
	ip = ip.To4()
	ip[3] = ip[3] + byte(i%256)
	ip[2] = ip[2] + byte(i/256)
	ip[1] = ip[1] + byte(i/(256*256))
	return ip.String() + "/" + prefixLen
}

// CheckTrafficPassViaPortPktCounter checks traffic stats via port statistics
func CheckTrafficPassViaPortPktCounter(pktCounters []*telemetry.Interface_Counters, threshold ...float64) bool {
	thresholdValue := float64(0.99)
	if len(threshold) > 0 {
		thresholdValue = threshold[0]
	}
	totalIn := uint64(0)
	totalOut := uint64(0)

	for _, s := range pktCounters {
		totalIn = s.GetInPkts() + totalIn
		totalOut = s.GetOutPkts() + totalOut
	}
	return float64(totalIn)/float64(totalOut) >= thresholdValue
}

// ReloadDUT reloads the router using GNMI APIs
func ReloadDUT(t *testing.T, dut *ondatra.DUTDevice) {
	gnoiClient := dut.RawAPIs().GNOI().New(t)
	_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Reboot failed %v", err)
	}
	time.Sleep(600 * time.Second)
}

// GNMIWithText applies the cisco text config using gnmi
func GNMIWithText(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, config string) {
	r := &gnmipb.SetRequest{
		Update: []*gnmipb.Update{
			{
				Path: &gnmipb.Path{Origin: "cli"},
				Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_AsciiVal{AsciiVal: config}},
			},
		},
	}
	_, err := dut.RawAPIs().GNMI().Default(t).Set(ctx, r)
	if err != nil {
		t.Errorf("There is error when applying the config")
	}
}
