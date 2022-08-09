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

// Package deviations defines the arguments to enable temporary workarounds
// for the featureprofiles test suite.
//
// Deviations may be introduced to temporarily work around non-compliant issues
// so further sub-tests can be implemented.  Deviations should be
// small in scope, typically affecting one sub-test, one OpenConfig
// path or small OpenConfig sub-tree.  Deviations are enabled using
// command line parameters.
//
// Passing with a deviation enabled is considered non-compliant to the
// OpenConfig featureprofiles test.
//
// To add a deviation:
//   - Submit a github issue explaining the need for the deviation.
//   - Submit a pull request referencing the above issue to add a flag to
//     this file and updates to the tests where it is intended to be used.
//
// To remove a deviation:
//   - Submit a pull request which proposes to resolve the relevant
//     github issue by removing the deviation and it's usage within tests.
//   - Typically the author or an affiliate of the author's organization
//     is expected to remove a deviation they introduced.
//
// To enable the deviations for a test run:
//   - By default, deviations are not enabled and instead require the
//     test invocation to set an argument to enable the deviation.
//   - For example:
//     go test my_test.go --deviation_interface_enabled=true
package deviations

import "flag"

// Vendor deviation flags.
var (
	InterfaceEnabled = flag.Bool("deviation_interface_enabled", false,
		"Device requires interface enabled leaf booleans to be explicitly set to true.")

	AggregateAtomicUpdate = flag.Bool("deviation_aggregate_atomic_update", true,
		"Device requires that aggregate Port-Channel and its members be defined in a single gNMI Update transaction at /interfaces; otherwise lag-type will be dropped, and no member can be added to the aggregate.")

	DefaultNetworkInstance = flag.String("deviation_default_network_instance", "DEFAULT", "The name used for the default network instance for VRF.  This has been standardized in OpenConfig as \"DEFAULT\" but some legacy devices are using \"default\"; tests should use this deviation as a temporary workaround.")
)
