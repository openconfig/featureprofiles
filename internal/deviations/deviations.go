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

	LegacyLinkLayerAddress = flag.Bool("deviation_legacy_link_layer_address", true,
		"Arista does not support setting Static ARP neighbor link_layer_address over gNMI, so setting the interface using ZTC OC to VC (b/197782586)")

	IgnoreSessionParams = flag.Bool("deviation_ignore_session_params", false,
		"Arista gRIBI always sets ModifyResponse.SessionParamResults to an empty message which is not compliant (b/196998254)")

	OmitL2MTU = flag.Bool("deviation_omit_l2_mtu", true,
		"Arista does not support setting L2 MTU, so only set L3 MTU (b/201112079)")

	AggregateAtomicUpdate = flag.Bool("deviation_aggregate_atomic_update", true,
		"Arista requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces; otherwise lag-type will be dropped, and no member can be added to the aggregate (b/201574574)")
)
