package otgutils

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

// WaitForARP waits for ARP to resolve on all OTG interfaces for a given ipType, which is
// either "IPv4" or "IPv6".
func WaitForARP(t *testing.T, otg *otg.OTG, c gosnappi.Config, ipType string) {
	intfs := []string{}
	for _, d := range c.Devices().Items() {
		Eth := d.Ethernets().Items()[0]
		intfs = append(intfs, Eth.Name())
	}

	for _, intf := range intfs {
		switch ipType {
		case "IPv4":
			got, ok := gnmi.WatchAll(t, otg, gnmi.OTG().Interface(intf).Ipv4NeighborAny().LinkLayerAddress().State(), 2*time.Minute, func(val *ygnmi.Value[string]) bool {
				return val.IsPresent()
			}).Await(t)
			if !ok {
				t.Fatalf("Did not receive OTG Neighbor entry for interface %s, last got: %v", intf, got)
			}
		case "IPv6":
			got, ok := gnmi.WatchAll(t, otg, gnmi.OTG().Interface(intf).Ipv6NeighborAny().LinkLayerAddress().State(), 2*time.Minute, func(val *ygnmi.Value[string]) bool {
				return val.IsPresent()
			}).Await(t)
			if !ok {
				t.Fatalf("Did not receive OTG Neighbor entry for interface %s, last got: %v", intf, got)
			}
		}
	}
}
