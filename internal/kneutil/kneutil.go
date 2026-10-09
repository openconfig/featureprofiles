// Package kneutil provides helpers for KNE-vs-HW conditional test behavior.
package kneutil

import (
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	tpb "github.com/openconfig/kne/proto/topo"
)

// IsKNEBinding reports whether dut is a Nokia device bound via knebind (KNE topology),
// as opposed to static/hardware binding. KNE-only test tweaks must gate on this so HW
// paths stay identical to the untouched featureprofiles baseline.
func IsKNEBinding(dut *ondatra.DUTDevice) bool {
	if dut.Vendor() != ondatra.NOKIA {
		return false
	}
	type serviceLookup interface {
		Service(string) (*tpb.Service, error)
	}
	var lookup serviceLookup
	return binding.DUTAs(dut.RawAPIs().BindingDUT(), &lookup) == nil
}
