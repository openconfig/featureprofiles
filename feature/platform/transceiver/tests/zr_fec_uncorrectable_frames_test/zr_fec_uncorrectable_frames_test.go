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

package zr_fec_uncorrectable_frames_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	transceiverType    = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	ethernetPMD        = oc.TransportTypes_ETHERNET_PMD_TYPE_ETH_400GBASE_ZR
	expectedCentreFreq = 193000000
	subscribeTimeout   = 30 * time.Second
	sampleInterval     = 10 * time.Second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func gnmiOpts(t *testing.T, dut *ondatra.DUTDevice, interval time.Duration) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(
		ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
		ygnmi.WithSampleInterval(interval),
	)
}

func TestZrUncorrectableFrames(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")

	transceivers := components.FindComponentsByType(t, dut, transceiverType)

	t.Logf("Found transceiver list: %v", transceivers)
	if len(transceivers) == 0 {
		t.Fatalf("Got transceiver list for %q: got 0, want > 0", dut.Model())
	}

	zrTransceivers := make([]string, 0)
	for _, tx := range transceivers {
		txPmd := gnmi.Get(t, dut, gnmi.OC().Component(tx).Transceiver().EthernetPmd().State())
		if txPmd == ethernetPMD {
			t.Logf("!! Got expected ethernetPMD %q for tx %q: got %q", ethernetPMD, tx, txPmd)
			zrTransceivers = append(zrTransceivers, tx)
		} else {
			t.Logf("Want ethernetPMD %q for tx %q: got %q", ethernetPMD, tx, txPmd)
		}

	}
	if len(zrTransceivers) == 0 {
		t.Fatalf("Got ZR transceiver list for %q: got 0, want > 0", dut.Model())
	}
	t.Logf("Found ZR transceiver list: %v", zrTransceivers)

	for _, tx := range zrTransceivers {
		t.Run(fmt.Sprintf("Transceiver:%s", tx), func(t *testing.T) {
			txComponent := gnmi.OC().Component(tx)

			centreFreq := gnmi.Get(t, dut, txComponent.Transceiver().Channel(0).OutputFrequency().State())

			if centreFreq != expectedCentreFreq {
				t.Errorf("Got centre frequency %d, want %d", centreFreq, expectedCentreFreq)
			}
		})
	}

	// There will be a single logical channel of type OTN for each optical channel.
	fec := gnmi.Get(t, dut, gnmi.OC().TerminalDevice().Channel(0).Otn().FecUncorrectableBlocks().State())
	if fec != 0 {
		t.Errorf("Got FecUncorrectableBlocks for got %d, want 0", fec)
	}
	gnmi.Update(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), bool(false))
	gnmi.Update(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), bool(true))

	streamedFec := gnmi.Collect(t, gnmiOpts(t, dut, sampleInterval), gnmi.OC().TerminalDevice().Channel(0).Otn().FecUncorrectableBlocks().State(), subscribeTimeout).Await(t)

	for _, f := range streamedFec {
		v, ok := f.Val()
		if !ok {
			t.Fatalf("Error capturing streaming Fec value")
		}
		if v != 0 {
			t.Errorf("Got FecUncorrectableBlocks got %d, want 0", v)
		}
	}
}
