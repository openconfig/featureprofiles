package helper

import (
	"context"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"testing"
)

type InterfaceHelper struct{}

// ClearInterfaceCountersAll executes a 'clear counters' CLI.
func (v *InterfaceHelper) ClearInterfaceCountersAll(t *testing.T, dut *ondatra.DUTDevice) {

	// Configure "service cli interactive disable" to disable the interactive prompt
	config.TextWithGNMI(context.Background(), t, dut, "service cli interactive disable")
	dut.CLI().Run(t, "clear counters")
}

func (v *InterfaceHelper) GetPerInterfaceCounters(t *testing.T, dut *ondatra.DUTDevice, intf string) *oc.Interface_Counters {
	counters := gnmi.Get(t, dut, (gnmi.OC().Interface(intf).Counters().State()))
	return counters
}

func (v *InterfaceHelper) GetAllInterfaceInUnicast(t *testing.T, dut *ondatra.DUTDevice, trafficType string) map[string]uint64 {
	var unicastStats []*ygnmi.Value[uint64]
	data := make(map[string]uint64)
	switch trafficType {
	case "ipv4":
		unicastStats = gnmi.LookupAll(t, dut, gnmi.OC().InterfaceAny().Subinterface(0).Ipv4().Counters().InPkts().State())
	case "ipv6":
		unicastStats = gnmi.LookupAll(t, dut, gnmi.OC().InterfaceAny().Subinterface(0).Ipv6().Counters().InPkts().State())
	default:
		unicastStats = gnmi.LookupAll(t, dut, (gnmi.OC().InterfaceAny().Counters().InUnicastPkts().State()))
	}
	for _, counters := range unicastStats {
		if intf, ok := counters.Path.Elem[1].Key["name"]; ok {
			data[intf], _ = counters.Val()
		}
	}
	return data
}
