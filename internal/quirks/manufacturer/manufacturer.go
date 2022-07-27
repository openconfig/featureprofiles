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

package manufacturer

import (
	"fmt"
	"testing"

	"github.com/openconfig/ondatra"
)

// PortChannelName returns the port channel interface name which is
// vendor specific.
func PortChannelName(t testing.TB, dut *ondatra.DUTDevice, i int) string {
	t.Helper()
	if i <= 0 {
		t.Fatalf("port-channel index must be >= 1: %d", i)
	}
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return fmt.Sprintf("Port-Channel%d", i)
	case ondatra.CISCO:
		return fmt.Sprintf("Port-channel%d", i)
	case ondatra.JUNIPER:
		return fmt.Sprintf("ae%d", i)
	}
	t.Fatalf("unsupported vendor: %s", dut.Vendor())
	return "" // Should be unreachable.
}