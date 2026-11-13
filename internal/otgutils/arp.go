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
// either "IPv4" or "IPv6". Interfaces without addresses of the requested family are skipped.
func WaitForARP(t *testing.T, otg *otg.OTG, c gosnappi.Config, ipType string) {
	for _, d := range c.Devices().Items() {
		eth := d.Ethernets().Items()[0]
		switch ipType {
		case "IPv4":
			if len(eth.Ipv4Addresses().Items()) == 0 {
				continue
			}
			got, ok := gnmi.WatchAll(t, otg, gnmi.OTG().Interface(eth.Name()).Ipv4NeighborAny().LinkLayerAddress().State(), 2*time.Minute, func(val *ygnmi.Value[string]) bool {
				return val.IsPresent()
			}).Await(t)
			if !ok {
				t.Fatalf("Did not receive OTG Neighbor entry for interface %s, last got: %v", eth.Name(), got)
			}
		case "IPv6":
			if len(eth.Ipv6Addresses().Items()) == 0 {
				continue
			}
			got, ok := gnmi.WatchAll(t, otg, gnmi.OTG().Interface(eth.Name()).Ipv6NeighborAny().LinkLayerAddress().State(), 2*time.Minute, func(val *ygnmi.Value[string]) bool {
				return val.IsPresent()
			}).Await(t)
			if !ok {
				t.Fatalf("Did not receive OTG Neighbor entry for interface %s, last got: %v", eth.Name(), got)
			}
		}
	}
}
