// Copyright 2023 Google LLC
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

package cfgplugins

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

var debugNotifications = flag.Bool("debug_notifications", true, "Enable full AFT notification recording")

// PrefixesParams contains the prefixes expected to be present in the AFT cache.
type PrefixesParams struct {
	InfoAFT  *aftcache.AFTData
	Prefixes []string
	Ctx      context.Context
}

// GnmiClientSession get the GNMI client session.
func GnmiClientSession(t *testing.T, dut *ondatra.DUTDevice, cfg PrefixesParams) gpb.GNMIClient {
	t.Helper()
	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(cfg.Ctx)
	if err != nil {
		t.Fatalf("Failed to dial GNMI client: %v", err)
	}
	return gnmiClient
}

// VerifyPrefixesPresent validates expected prefixes exist.
func VerifyPrefixesPresent(t *testing.T, cfg PrefixesParams) {
	t.Helper()

	for _, pfx := range cfg.Prefixes {
		if _, ok := cfg.InfoAFT.Prefixes[pfx]; !ok {
			t.Fatalf("Expected prefix missing: %s", pfx)
		}
	}
}

// VerifyPrefixesAbsent validates prefixes do not exist.
func VerifyPrefixesAbsent(t *testing.T, cfg PrefixesParams) {
	t.Helper()
	for _, pfx := range cfg.Prefixes {
		if _, ok := cfg.InfoAFT.Prefixes[pfx]; ok {
			t.Fatalf("Unexpected prefix present: %s", pfx)
		}
	}
}

// RunCollectorParams contains the parameters required to execute an AFT collector until the supplied stopping condition is satisfied.
type RunCollectorParams struct {
	Ctx       context.Context
	Collector *aftcache.AFTStreamSession
	Stop      aftcache.PeriodicHook
	Timeout   time.Duration
}

// RunCollector starts the AFT stream collector and blocks until the supplied stopping condition is satisfied or the collector times out.
func RunCollector(t *testing.T, cfg RunCollectorParams) {
	t.Helper()
	cfg.Collector.ListenUntil(cfg.Ctx, t, cfg.Timeout, cfg.Stop)
}

// RemovePrefixFromPrefixSetParams contains the parameters required to remove a prefix entry from a routing policy prefix set.
type RemovePrefixFromPrefixSetParams struct {
	PrefixSetName string
	Prefix        string
	MaskRange     string
}

// RemovePrefixFromPrefixSet removes the specified prefix entry from the given routing-policy prefix-set on the DUT.
func RemovePrefixFromPrefixSet(t *testing.T, dut *ondatra.DUTDevice, cfg RemovePrefixFromPrefixSetParams) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	gnmi.BatchDelete(batch, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(cfg.PrefixSetName).Prefix(cfg.Prefix, cfg.MaskRange).Config())
	batch.Set(t, dut)
}

// NewCollectorParams contains the parameters required to create a new AFT stream session.
type NewCollectorParams struct {
	Context context.Context
	Client  gpb.GNMIClient
}

// NewCollector creates and returns a new AFT stream session. If debug_notifications is enabled, all received gNMI notifications are recorded in memory for later inspection and troubleshooting.
func NewCollector(t *testing.T, dut *ondatra.DUTDevice, cfg NewCollectorParams) *aftcache.AFTStreamSession {
	t.Helper()
	c := aftcache.NewAFTStreamSession(cfg.Context, t, cfg.Client, dut)
	if *debugNotifications {
		c.WithDebug()
		t.Log("DEBUG MODE ENABLED: Recording all gNMI notifications to memory.")
	}
	return c
}
