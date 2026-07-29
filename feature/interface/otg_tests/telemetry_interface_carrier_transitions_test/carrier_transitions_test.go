// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package carrier_transitions_test

import (
	"context"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/functional-translators/registrar"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	dutSrc = &attrs.Attributes{
		Desc:    "DUT Source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:DB8::192:0:2:1",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	ateSrc = &attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		Desc:    "ATE Source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:DB8::192:0:2:2",
		IPv4Len: 30,
		IPv6Len: 126,
	}
)

func configureInterface(t *testing.T, dut *ondatra.DUTDevice, dp1 *ondatra.Port) {
	t.Helper()

	// Configure DUT interface
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp1.Name()).Config(), dutSrc.NewOCInterface(dp1.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	// Configure ATE to ensure link comes up
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	top := gosnappi.NewConfig()
	ateSrc.AddToOTG(top, ap1, dutSrc)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Wait for link to be UP
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), 30*time.Second, oc.Interface_OperStatus_UP)
}

func TestCarrierTransitions(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")

	configureInterface(t, dut, dp1)

	t.Run("CarrierTransitions", func(t *testing.T) {
		testCarrierTransitions(t, dut, dp1)
	})

	t.Run("PhyCarrierTransitions", func(t *testing.T) {
		testPhyCarrierTransitions(t, dut, dp1)
	})
}

func testCarrierTransitions(t *testing.T, dut *ondatra.DUTDevice, dp1 *ondatra.Port) {
	// Start metric collection (SAMPLE mode, 30s interval)
	t.Log("Starting carrier-transitions collection...")
	s := samplestream.New(t, dut, gnmi.OC().Interface(dp1.Name()).Counters().CarrierTransitions().State(), 30*time.Second)
	defer s.Close()

	// Flap the interface
	t.Log("Disabling interface...")
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp1.Name()).Enabled().Config(), false)
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), 30*time.Second, oc.Interface_OperStatus_DOWN)

	t.Log("Enabling interface...")
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp1.Name()).Enabled().Config(), true)
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), 30*time.Second, oc.Interface_OperStatus_UP)

	// Wait for one more sample interval to ensure we capture the final state
	t.Log("Waiting for next sample...")
	s.Next()
	vals := s.All()

	// Validation
	if len(vals) < 2 {
		t.Errorf("Insufficient samples collected: got %d, want at least 2", len(vals))
	}

	var initialCount, finalCount uint64
	first := true

	for i, val := range vals {
		v, present := val.Val()
		if !present {
			continue
		}
		ts := val.Timestamp
		t.Logf("Sample %d: %d at %v", i, v, ts)

		if first {
			initialCount = v
			finalCount = v
			first = false
			continue
		}

		// 1. Monotonicity check
		if v < finalCount {
			t.Errorf("Value decreased! Sample %d: %d, Previous: %d", i, v, finalCount)
		}

		// 2. Max delta check
		if v > finalCount && (v-finalCount) > 100 {
			t.Errorf("Value increased by too much! Sample %d: %d, Previous: %d, Delta: %d", i, v, finalCount, v-finalCount)
		}

		finalCount = v
	}

	// 3. Functional check (must increase)
	if finalCount <= initialCount {
		t.Errorf("Carrier transitions did not increase. Initial: %d, Final: %d", initialCount, finalCount)
	} else {
		t.Logf("Carrier transitions validated successfully. Initial: %d, Final: %d", initialCount, finalCount)
	}
}

func testPhyCarrierTransitions(t *testing.T, dut *ondatra.DUTDevice, dp1 *ondatra.Port) {
	if deviations.CarrierFt(dut) == "" {
		t.Skip("Skipping phy-carrier-transitions check as CarrierFt deviation is not populated.")
	}

	ctx := context.Background()

	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial gNMI: %v", err)
	}

	ft, ok := registrar.FunctionalTranslatorRegistry[deviations.CarrierFt(dut)]
	if !ok {
		t.Fatalf("Functional translator %q not found.", deviations.CarrierFt(dut))
	}

	var nativePaths []*gnmipb.Path
	for _, paths := range ft.OutputToInputMap() {
		nativePaths = append(nativePaths, paths...)
	}
	if len(nativePaths) == 0 {
		t.Fatalf("No native paths found for functional translator %q", deviations.CarrierFt(dut))
	}

	// Flap the interface and read raw values using Get to verify functional increase.
	readCount := func() (uint64, bool) {
		resp, err := gnmiClient.Get(ctx, &gnmipb.GetRequest{
			Path:     nativePaths,
			Type:     gnmipb.GetRequest_STATE,
			Encoding: gnmipb.Encoding_JSON_IETF,
		})
		if err != nil {
			t.Errorf("Failed to get native paths: %v", err)
			return 0, false
		}

		for _, notification := range resp.GetNotification() {
			dummySR := &gnmipb.SubscribeResponse{
				Response: &gnmipb.SubscribeResponse_Update{
					Update: notification,
				},
			}
			translatedSR, err := ft.Translate(dummySR)
			if err != nil {
				t.Errorf("Translation Failed: %v", err)
				continue
			}
			if translatedSR == nil {
				continue
			}
			for _, update := range translatedSR.GetUpdate().GetUpdate() {
				path := update.GetPath()
				elems := path.GetElem()
				if len(elems) < 6 {
					continue
				}
				if elems[0].GetName() == "interfaces" && elems[1].GetName() == "interface" {
					intfName := elems[1].GetKey()["name"]
					if intfName == dp1.Name() && elems[5].GetName() == "phy-carrier-transitions" {
						return update.GetVal().GetUintVal(), true
					}
				}
			}
		}
		return 0, false
	}

	// Read initial value
	initialCount, present := readCount()
	if !present {
		t.Logf("phy-carrier-transitions native counter not present initially.")
	} else {
		t.Logf("Initial phy-carrier-transitions count: %d", initialCount)
	}

	// Flap the interface
	t.Log("Disabling interface for phy-carrier-transitions...")
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp1.Name()).Enabled().Config(), false)
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), 30*time.Second, oc.Interface_OperStatus_DOWN)

	t.Log("Enabling interface for phy-carrier-transitions...")
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp1.Name()).Enabled().Config(), true)
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), 30*time.Second, oc.Interface_OperStatus_UP)

	// Wait up to 60 seconds for the counter to update asynchronously.
	var finalCount uint64
	present = false
	// var present bool
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		finalCount, present = readCount()
		if present && finalCount > initialCount {
			break
		}
		time.Sleep(5 * time.Second)
	}

	if !present {
		t.Fatalf("phy-carrier-transitions native counter not found after flap.")
	}
	t.Logf("Final phy-carrier-transitions count: %d", finalCount)

	if finalCount <= initialCount {
		t.Errorf("phy-carrier-transitions did not increase. Initial: %d, Final: %d", initialCount, finalCount)
	} else {
		t.Logf("phy-carrier-transitions validated successfully. Initial: %d, Final: %d", initialCount, finalCount)
	}
}
