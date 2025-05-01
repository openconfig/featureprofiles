// Package otgvalidationhelpers provides helper functions to validate OTG attributes for OTG tests.
package otgvalidationhelpers

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

/*
OTGValidation is a struct to hold OTG validation parameters.

	params := &OTGValidation{
		Interface: 	&InterfaceParams{Names: []string{"Interface1", "Interface2"}, Ports: []string{"Port1", "Port2"}},
		Flow:       &FlowParams{Name: "flow1", TolerancePct: 0.5},
	}

		if err := params.ValidatePortIsActive(t, ate); err != nil {
			t.Errorf("ValidatePortIsActive(): got err: %q, want nil", err)
		}
		if err := params.IsIPv4Interfaceresolved(t, ate); err != nil {
			t.Errorf("IsIPv4Interfaceresolved(): got err: %q, want nil", err)
		}
		if err := params.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("ValidateLossOnFlows(): got err: %q, want nil", err)
		}
*/
type OTGValidation struct {
	Interface *InterfaceParams
	Flow      *FlowParams
}

// InterfaceParams is a struct to hold OTG interface parameters.
type InterfaceParams struct {
	Names []string
	Ports []string
}

// FlowParams is a struct to hold OTG flow parameters.
type FlowParams struct {
	Name         string
	TolerancePct float32
}

// IsIPv4Interfaceresolved validates that the IPv4 interface is resolved based on the interface configured using otgconfighelpers.
func (v *OTGValidation) IsIPv4Interfaceresolved(t *testing.T, ate *ondatra.ATEDevice) error {
	for _, intf := range v.Interface.Names {
		val1, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().Interface(intf+".Eth").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t)
		if !ok {
			return fmt.Errorf(`IPv4 %s gateway not resolved`, intf)
		}
		t.Logf(`IPv4 %s gateway resolved to: %s`, intf, val1)
	}
	return nil
}

// IsIPv6Interfaceresolved validates that the IPv6 interface is resolved based on the interface configured using otgconfighelpers.
func (v *OTGValidation) IsIPv6Interfaceresolved(t *testing.T, ate *ondatra.ATEDevice) error {
	for _, intf := range v.Interface.Names {
		val1, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().Interface(intf+".Eth").Ipv6NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t)
		if !ok {
			return fmt.Errorf(`IPv6 %s gateway not resolved`, intf)
		}
		t.Logf(`IPv6 %s gateway resolved to: %s`, intf, val1)
	}
	return nil
}

// ValidateLossOnFlows validates the percentage of traffic loss on the flows.
func (v *OTGValidation) ValidateLossOnFlows(t *testing.T, ate *ondatra.ATEDevice) error {
	outPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v.Flow.Name).Counters().OutPkts().State())
	if outPkts == 0 {
		t.Fatalf("Get(out packets for flow %q): got %v, want nonzero", v.Flow.Name, outPkts)
	}
	inPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v.Flow.Name).Counters().InPkts().State())
	lossPct := 100 * float32(outPkts-inPkts) / float32(outPkts)
	if lossPct > v.Flow.TolerancePct {
		return fmt.Errorf("Get(traffic loss for flow %q): got %v percent, want < %v percent", v.Flow.Name, lossPct, v.Flow.TolerancePct)
	}
	t.Logf("Flow %q, inPkts %d, outPkts %d, lossPct %v", v.Flow.Name, inPkts, outPkts, lossPct)
	return nil
}

// ValidatePortIsActive validates the OTG port status.
func (v *OTGValidation) ValidatePortIsActive(t *testing.T, ate *ondatra.ATEDevice) error {
	for _, port := range v.Interface.Ports {
		PortStatus := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(port).Link().State())
		if want := otg.Port_Link_UP; PortStatus != want {
			return fmt.Errorf("Get(OTG port status): got %v, want %v", PortStatus, want)
		}
	}
	return nil
}
