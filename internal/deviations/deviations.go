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

// Package deviations defines the arguments to enable temporary workarounds for the
// featureprofiles test suite.
//
// A test may introduce deviations in order to temporarily work around non-compliant
// issues so further sub-tests can make progress.  They are enabled using command line
// parameters.  Deviations typically work by reducing testing requirements or by changing
// the way the configuration is done.  However, the intended behavior to be tested should
// always be without the deviation.
//
// Requirements for deviations:
//
//   - Deviations may only use OpenConfig compliant behavior.
//   - Deviations should be small in scope, typically affecting one sub-test, one
//     OpenConfig path or small OpenConfig sub-tree.
//
// If a device could not pass without deviation, that is considered non-compliant
// behavior.  Ideally, a device should pass both with and without a deviation which means
// the deviation could be safely removed.  However, when the OpenConfig model allows the
// device to reject the deviated case even if it is compliant, then this should be
// explained on a case-by-case basis.
//
// To add a deviation:
//
//   - Submit a github issue explaining the need for the deviation.
//   - Submit a pull request referencing the above issue to add a flag to
//     this file and updates to the tests where it is intended to be used.
//   - Make sure the deviation defaults to false.  False (not deviated) means strictly
//     compliant behavior.  True (deviated) activates the workaround.
//
// To remove a deviation:
//
//   - Submit a pull request which proposes to resolve the relevant
//     github issue by removing the deviation and it's usage within tests.
//   - Typically the author or an affiliate of the author's organization
//     is expected to remove a deviation they introduced.
//
// To enable the deviations for a test run:
//
//   - By default, deviations are not enabled and instead require the
//     test invocation to set an argument to enable the deviation.
//   - For example:
//     go test my_test.go --deviation_interface_enabled=true
package deviations

import "flag"

// Vendor deviation flags.
var (
	InterfaceEnabled = flag.Bool("deviation_interface_enabled", false,
		"Device requires interface enabled leaf booleans to be explicitly set to true.  Fully-compliant devices should pass both with and without this deviation.")

	AggregateAtomicUpdate = flag.Bool("deviation_aggregate_atomic_update", false,
		"Device requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces; otherwise lag-type will be dropped, and no member can be added to the aggregate.  Fully-compliant devices should pass both with and without this deviation.")

	DefaultNetworkInstance = flag.String("deviation_default_network_instance", "DEFAULT",
		"The name used for the default network instance for VRF.  The default name in OpenConfig is \"DEFAULT\" but some legacy devices still use \"default\".  Fully-compliant devices should be able to use any operator-assigned values.")

	SubinterfacePacketCountersMissing = flag.Bool("deviation_subinterface_packet_counters_missing", false,
		"Device is missing subinterface packet counters for IPv4/IPv6, so the test will skip checking them.  Fully-compliant devices should pass both with and without this deviation.")
)
