// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cfgplugins is a collection of OpenConfig configuration libraries.
//
// Each plugin function has parameters for a gnmi Batch, values to use in the configuration
// and an ondatra.DUTDevice.  Each function returns OpenConfig values.
//
// The configuration function will modify the batch which is passed in by reference. The caller may
// pass nil as the configuration values to use in which case the function will provide a set of
// default values.  The ondatra.DUTDevice is used to determine any configuration deviations which
// may be necessary.
//
// The caller may choose to use the returned OC value to customize the values set by this
// function or for use in a non-batch use case.
package cfgplugins

import (
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
)

// normalizeNIName applies deviations related to NetworkInterface names
func normalizeNIName(niName string, d *ondatra.DUTDevice) string {
	if niName == "DEFAULT" {
		return deviations.DefaultNetworkInstance(d)
	}
	return niName
}
