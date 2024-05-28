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
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	sampleInterval       = 10 * time.Second
	targetOutputPowerdBm = -10
	targetFrequencyHz    = 193100000
	intUpdateTime        = 2 * time.Minute
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func opticalChannelComponentFromPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	t.Helper()
	if deviations.MissingPortToOpticalChannelMapping(dut) {
		transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
		return fmt.Sprintf("%s-Optical0", transceiverName)
	}
	compName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State())
	for {
		comp, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(compName).State()).Val()
		if !ok {
			t.Fatalf("Recursive optical channel lookup failed for port: %s, component %s not found.", p.Name(), compName)
		}
		if comp.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_OPTICAL_CHANNEL {
			return compName
		}
		if comp.GetParent() == "" {
			t.Fatalf("Recursive optical channel lookup failed for port: %s, parent of component %s not found.", p.Name(), compName)
		}
		compName = comp.GetParent()
	}
}

func interfaceConfig(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
	ocComponent := opticalChannelComponentFromPort(t, dut, dp)
	t.Logf("Got opticalChannelComponent from port: %s", ocComponent)
	gnmi.Replace(t, dut, gnmi.OC().Component(ocComponent).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPowerdBm),
		Frequency:         ygot.Uint64(targetFrequencyHz),
	})
}

func validateFecUncorrectableBlocks(t *testing.T, stream *samplestream.SampleStream[uint64]) {
	fecStream := stream.Next()
	if fecStream == nil {
		t.Fatalf("Fec Uncorrectable Blocks was not streamed in the most recent subscription interval")
	}
	fec, ok := fecStream.Val()
	if !ok {
		t.Fatalf("Error capturing streaming Fec value")
	}
	if reflect.TypeOf(fec).Kind() != reflect.Int64 {
		t.Fatalf("fec value is not type int64")
	}
	if fec != 0 {
		t.Fatalf("Got FecUncorrectableBlocks got %d, want 0", fec)
	}
}

func TestZrUncorrectableFrames(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	interfaceConfig(t, dut, dut.Port(t, "port1"))
	interfaceConfig(t, dut, dut.Port(t, "port2"))

	for _, port := range []string{"port1", "port2"} {
		t.Run(fmt.Sprintf("Port:%s", port), func(t *testing.T) {
			dp := dut.Port(t, "port1")
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)

			streamFec := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(0).Otn().FecUncorrectableBlocks().State(), sampleInterval)
			defer streamFec.Close()
			validateFecUncorrectableBlocks(t, streamFec)

			// Toggle interface enabled
			gnmi.Update(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config(), bool(false))
			gnmi.Update(t, dut, gnmi.OC().Interface(dp.Name()).Enabled().Config(), bool(true))

			validateFecUncorrectableBlocks(t, streamFec)
		})
	}
}
