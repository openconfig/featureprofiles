// Copyright 2024 Google LLC
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
	"flag"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	sampleInterval = 10 * time.Second
	intUpdateTime  = 2 * time.Minute
	otnIndexBase   = uint32(4000)
	ethIndexBase   = uint32(40000)
)

var (
	operationalModeFlag = flag.Int("operational_mode", 0, "vendor-specific operational-mode for the channel.")
	operationalMode     uint16
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func validateFecUncorrectableBlocks(t *testing.T, stream *samplestream.SampleStream[uint64], baselineValue uint64) {
	dut := ondatra.DUT(t, "dut")
	fecStream := stream.Next()
	if fecStream == nil {
		t.Fatalf("Fec Uncorrectable Blocks was not streamed in the most recent subscription interval")
	}
	currentFec, ok := fecStream.Val()
	if !ok {
		t.Fatalf("Error capturing streaming Fec value")
	}
	if reflect.TypeOf(currentFec).Kind() != reflect.Uint64 {
		t.Fatalf("fec value is not type uint64")
	}
	if deviations.NonIntervalFecErrorCounter(dut) {
		// Check if the counter is incrementing
		if currentFec > baselineValue {
			t.Fatalf("FecUncorrectableBlocks increased after flap: baseline=%d, current=%d, diff=%d",
				baselineValue, currentFec, currentFec-baselineValue)
		}
		t.Logf("FecUncorrectableBlocks not increasing: baseline=%d, current=%d", baselineValue, currentFec)
	} else if currentFec != 0 {
		t.Fatalf("Got FecUncorrectableBlocks got %d, want 0", currentFec)
	}
}

func TestZrUncorrectableFrames(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var (
		trs        = make(map[string]string)
		ochs       = make(map[string]string)
		otnIndexes = make(map[string]uint32)
		ethIndexes = make(map[string]uint32)
	)

	ports := []string{"port1", "port2"}
	operationalMode = uint16(*operationalModeFlag)
	cfgplugins.InterfaceInitialize(t, dut, operationalMode)

	for i, port := range ports {
		dp := dut.Port(t, port)
		cfgplugins.InterfaceConfig(t, dut, dp)
		trs[dp.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
		ochs[dp.Name()] = gnmi.Get(t, dut, gnmi.OC().Component(trs[dp.Name()]).Transceiver().Channel(0).AssociatedOpticalChannel().State())
		otnIndexes[dp.Name()] = otnIndexBase + uint32(i)
		ethIndexes[dp.Name()] = ethIndexBase + uint32(i)
		cfgplugins.ConfigOTNChannel(t, dut, ochs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()])
		cfgplugins.ConfigETHChannel(t, dut, dp.Name(), trs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()])
	}

	for _, port := range ports {
		t.Run(fmt.Sprintf("Port:%s", port), func(t *testing.T) {
			dp := dut.Port(t, port)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
			streamFecOtn := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State(), sampleInterval)
			defer streamFecOtn.Close()
			// Get baseline FEC value 1 second after interface is up
			time.Sleep(1 * time.Second)
			baselineFecStream := streamFecOtn.Next()
			if baselineFecStream == nil {
				t.Fatalf("Baseline Fec Uncorrectable Blocks was not streamed")
			}
			baselineFec, ok := baselineFecStream.Val()
			if !ok {
				t.Fatalf("Error capturing baseline Fec value")
			}
			// Wait another 10 seconds (total 10 seconds after flap)
			time.Sleep(10 * time.Second)

			// Validate that FEC errors haven't increased
			validateFecUncorrectableBlocks(t, streamFecOtn, baselineFec)

			// Toggle interface enabled
			d := &oc.Root{}
			i := d.GetOrCreateInterface(dp.Name())
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

			// Disable interface
			i.Enabled = ygot.Bool(false)
			gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
			// Wait for the cooling-off period
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)

			// Enable interface
			i.Enabled = ygot.Bool(true)
			gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
			// Wait for the cooling-off period
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)

			// Get baseline FEC value 1 second after interface is up
			time.Sleep(1 * time.Second)
			baselineFecStream1 := streamFecOtn.Next()
			if baselineFecStream1 == nil {
				t.Fatalf("Baseline Fec Uncorrectable Blocks was not streamed")
			}
			baselineFec1, ok := baselineFecStream1.Val()
			if !ok {
				t.Fatalf("Error capturing baseline Fec value")
			}
			// Wait another 10 seconds (total 10 seconds after flap)
			time.Sleep(10 * time.Second)

			// Validate that FEC errors haven't increased
			validateFecUncorrectableBlocks(t, streamFecOtn, baselineFec1)
		})
	}
}
