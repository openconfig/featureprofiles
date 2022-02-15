// Package deviations documents and controls vendor deviation
// workarounds that exist in the featureprofiles test suite.
//
// This is a temporary workaround and will be deprecated in the
// future.
package deviations

import "flag"

// Vendor deviation flags.
var (
	InterfaceEnabled = flag.Bool("deviation_interface_enabled", true,
		"Arista requires interface enabled leaf booleans to be explicitly set to true (b/197141773)")

	OmitL2MTU = flag.Bool("deviation_omit_l2_mtu", true,
		"Arista does not support setting L2 MTU, so only set L3 MTU (b/201112079)")

	AggregateAtomicUpdate = flag.Bool("deviation_aggregate_atomic_update", true,
		"Arista requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces; otherwise lag-type will be dropped, and no member can be added to the aggregate (b/201574574)")
)
