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

type interfaceHelper struct{}

// ClearInterfaceCountersAll executes a 'clear counters' CLI.
func (v *interfaceHelper) ClearInterfaceCountersAll(t *testing.T, dut []*ondatra.DUTDevice) {
	for _, device := range dut {
		// Configure "service cli interactive disable" to disable the interactive prompt
		config.TextWithGNMI(context.Background(), t, device, "service cli interactive disable")
		device.CLI().Run(t, "clear counters")
	}
}

func (v *interfaceHelper) GetPerInterfaceCounters(t testing.TB, dut *ondatra.DUTDevice, intf string) *oc.Interface_Counters {
	t.Helper()
	counters := gnmi.Get(t, dut, (gnmi.OC().Interface(intf).Counters().State()))
	return counters
}

func (v *interfaceHelper) GetPerInterfaceV4Counters(t testing.TB, dut *ondatra.DUTDevice, intf string) *oc.Interface_Subinterface_Ipv4_Counters {
	t.Helper()
	counters := gnmi.Get(t, dut, (gnmi.OC().Interface(intf).Subinterface(0).Ipv4().Counters().State()))
	return counters
}

func (v *interfaceHelper) GetPerInterfaceV6Counters(t testing.TB, dut *ondatra.DUTDevice, intf string) *oc.Interface_Subinterface_Ipv6_Counters {
	t.Helper()
	counters := gnmi.Get(t, dut, (gnmi.OC().Interface(intf).Subinterface(0).Ipv6().Counters().State()))
	return counters
}

func (v *interfaceHelper) GetAllInterfaceInUnicast(t testing.TB, dut *ondatra.DUTDevice, trafficType string) map[string]uint64 {
	t.Helper()
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

func (v *interfaceHelper) GetAllInterfaceOutUnicast(t testing.TB, dut *ondatra.DUTDevice, trafficType string) map[string]uint64 {
	t.Helper()
	var unicastStats []*ygnmi.Value[uint64]
	data := make(map[string]uint64)
	switch trafficType {
	case "ipv4":
		unicastStats = gnmi.LookupAll(t, dut, gnmi.OC().InterfaceAny().Subinterface(0).Ipv4().Counters().OutPkts().State())
	case "ipv6":
		unicastStats = gnmi.LookupAll(t, dut, gnmi.OC().InterfaceAny().Subinterface(0).Ipv6().Counters().OutPkts().State())
	default:
		unicastStats = gnmi.LookupAll(t, dut, (gnmi.OC().InterfaceAny().Counters().OutUnicastPkts().State()))
	}
	for _, counters := range unicastStats {
		if intf, ok := counters.Path.Elem[1].Key["name"]; ok {
			data[intf], _ = counters.Val()
		}
	}
	return data
}

// GetBundleMembers retrieves the bundle member interfaces for a given bundle interface.
func (v *interfaceHelper) GetBundleMembers(t testing.TB, dut *ondatra.DUTDevice, bundleInterface string) map[string][]string {
	t.Helper()
	// Create a map to store the result
	bundleMembers := make(map[string][]string)

	// Use ondatra gnmi.Lookup API to retrieve the bundle member interfaces
	members := gnmi.Lookup(t, dut, gnmi.OC().Interface(bundleInterface).Aggregation().Member().State())

	// Check if the value is present and extract it
	if memberList, ok := members.Val(); ok {
		bundleMembers[bundleInterface] = append(bundleMembers[bundleInterface], memberList...)
	}

	return bundleMembers
}
