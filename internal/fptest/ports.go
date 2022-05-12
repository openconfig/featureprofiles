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

package fptest

import (
	"fmt"
	"sort"

	"github.com/openconfig/ondatra"
)

// LAGName returns an aggregate interface name which is vendor specific.
func LAGName(vendor ondatra.Vendor, i int) (string, error) {
	if i <= 0 {
		return "", fmt.Errorf("LAG index must be >= 1: %d", i)
	}

	switch vendor {
	case ondatra.ARISTA:
		return fmt.Sprintf("Port-Channel%d", i), nil
	case ondatra.CISCO:
		return fmt.Sprintf("Bundle-Ether%d", i), nil
	case ondatra.JUNIPER:
		// Juniper technically allows 0, but the other vendors start with 1.
		return fmt.Sprintf("ae%d", i), nil
	}

	return "", fmt.Errorf("unsupported vendor: %s", vendor)
}

// SortPorts sorts the ports by their ID in the testbed.  Otherwise
// Ondatra returns the ports in arbitrary order.
func SortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		idi, idj := ports[i].ID(), ports[j].ID()
		if len(idi) < len(idj) {
			return true // "port2" < "port10"
		}
		return idi < idj
	})
	return ports
}
