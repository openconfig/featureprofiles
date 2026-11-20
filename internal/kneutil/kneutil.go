// Package kneutil provides helpers for KNE-vs-HW conditional test behavior.
package kneutil

import (
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
)

// IsKNEDUT reports whether dut is bound via knebind (KNE topology) for any
// vendor, as opposed to static/hardware binding.
func IsKNEDUT(dut *ondatra.DUTDevice) bool {
	type serviceLookup interface {
		Service(string) (*tpb.Service, error)
	}
	var lookup serviceLookup
	return binding.DUTAs(dut.RawAPIs().BindingDUT(), &lookup) == nil
}

// IsKNEBinding reports whether dut is a Nokia device bound via knebind.
// Nokia-only KNE tweaks must gate on this so HW paths stay identical to the
// untouched featureprofiles baseline.
func IsKNEBinding(dut *ondatra.DUTDevice) bool {
	return dut.Vendor() == ondatra.NOKIA && IsKNEDUT(dut)
}
