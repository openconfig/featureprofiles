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

package zr_supply_voltage_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	transceiverType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	ethernetPMD     = oc.TransportTypes_ETHERNET_PMD_TYPE_ETH_400GBASE_ZR
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestZrSupplyVoltage(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	transceivers := components.FindComponentsByType(t, dut, transceiverType)
	t.Logf("Found transceiver list: %v", transceivers)
	if len(transceivers) == 0 {
		t.Fatalf("Got transceiver list for %q: got 0, want > 0", dut.Model())
	}

	// Create slice of transceivers with required PMD.
	zrTransceivers := make([]string, 0)
	for _, tx := range transceivers {
		if txPmd := gnmi.Get(t, dut, gnmi.OC().Component(tx).Transceiver().EthernetPmd().State()); txPmd == ethernetPMD {
			zrTransceivers = append(zrTransceivers, tx)
		}
	}
	if len(zrTransceivers) == 0 {
		t.Fatalf("Got ZR transceiver list for %q: got 0, want > 0", dut.Model())
	}

	for _, tx := range zrTransceivers {
		t.Run(fmt.Sprintf("Transceiver:%s", tx), func(t *testing.T) {
			txComponent := gnmi.OC().Component(tx)

			avgV := gnmi.Get(t, dut, txComponent.Transceiver().SupplyVoltage().Avg().State())
			minV := gnmi.Get(t, dut, txComponent.Transceiver().SupplyVoltage().Min().State())
			instV := gnmi.Get(t, dut, txComponent.Transceiver().SupplyVoltage().Instant().State())
			maxV := gnmi.Get(t, dut, txComponent.Transceiver().SupplyVoltage().Max().State())

			if avgV < minV {
				t.Errorf("Want minV < avgV for tx %q: got minVoltage %f, avgV %f", tx, minV, avgV)
			}
			if avgV > maxV {
				t.Errorf("Want maxV > avgV for tx %q: got maxVoltage %f, avgV %f", tx, maxV, avgV)
			}
			if instV < minV {
				t.Errorf("Want minV < instV for tx %q: got minVoltage %f, avgV %f", tx, minV, instV)
			}
			if instV > maxV {
				t.Errorf("Want maxV > instV for tx %q: got maxVoltage %f, avgV %f", tx, maxV, instV)
			}

		})
	}

}
