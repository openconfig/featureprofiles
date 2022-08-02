// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package deviations documents and controls vendor deviation
// workarounds that exist in the featureprofiles test suite.
//
// This is a temporary workaround and will be deprecated in the
// future.
package deviations

import "flag"

// Vendor deviation flags.
var (
	InterfaceEnabled = flag.Bool("deviation_interface_enabled", false,
		"Device requires interface enabled leaf booleans to be explicitly set to true.")

	AggregateAtomicUpdate = flag.Bool("deviation_aggregate_atomic_update", true,
		"Device requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces; otherwise lag-type will be dropped, and no member can be added to the aggregate.")

	DefaultNetworkInstance = flag.String("deviation_default_network_instance", "DEFAULT", "The name used for the default network instance for VRF.  This has been standardized in OpenConfig as \"DEFAULT\" but some legacy devices are using \"default\"; tests should use this deviation as a temporary workaround.")

	SubInterfacePacketCountersSupported = flag.Bool("deviation_subinterface_counter_supported", true,
		"Subinterface discard packet counters for ipv4/ipv6 are not supported by default. Manually set it to False to skip lookup of discard counters in the test")
)
