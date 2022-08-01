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

// Package lsp_updates_test implements RT-2.2.
package lsp_updates_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/experimental/isis/ate_tests/internal/assert"
	"github.com/openconfig/featureprofiles/feature/experimental/isis/ate_tests/internal/session"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOverloadBit(t *testing.T) {
	ts := session.NewWithISIS(t)
	ts.PushAndStart(t)
	telemPth := ts.DUTISISTelemetry(t)
	ts.AwaitAdjacency(t)
	setBit := telemPth.Global().LspBit().OverloadBit().SetBit()
	overloads := telemPth.Level(2).SystemLevelCounters().DatabaseOverloads()
	assert.ValueOrNil(t, setBit, false)
	assert.ValueOrNil(t, overloads, uint32(0))
	ts.DUTConf.
		GetNetworkInstance(*deviations.DefaultNetworkInstance).
		GetProtocol(session.PTISIS, session.ISISName).
		GetIsis().
		GetGlobal().
		GetOrCreateLspBit().
		GetOrCreateOverloadBit().SetBit = ygot.Bool(true)
	ts.PushDUT(t)
	// TODO: Verify the link state database once device support is added.
	overloads.Await(t, time.Second*10, 1)
	assert.Value(t, setBit, true)
	// TODO: Verify the link state database on the ATE once the ATE reports this properly
	// ateTelemPth := ts.ATEISISTelemetry(t)
	// ateDB := ateTelemPth.Level(2).LspAny()
	// for _, nbr := range ateDB.Tlv(telemetry.IsisLsdbTypes_ISIS_TLV_TYPE_IS_NEIGHBOR_ATTRIBUTE).IsisNeighborAttribute().NeighborAny().Get(t) {
	// }
}

func TestMetric(t *testing.T) {
	t.Logf("Starting...")
	ts := session.NewWithISIS(t)
	ts.DUTConf.GetNetworkInstance(*deviations.DefaultNetworkInstance).GetProtocol(session.PTISIS, session.ISISName).GetIsis().
		GetInterface(ts.DUT.Port(t, "port1").Name()).
		GetOrCreateLevel(2).
		GetOrCreateAf(telemetry.IsisTypes_AFI_TYPE_IPV4, telemetry.IsisTypes_SAFI_TYPE_UNICAST).
		Metric = ygot.Uint32(100)
	ts.PushAndStart(t)
	ts.AwaitAdjacency(t)

	telemPth := ts.DUTISISTelemetry(t)
	metric := telemPth.Interface(ts.DUT.Port(t, "port1").Name()).Level(2).
		Af(telemetry.IsisTypes_AFI_TYPE_IPV4, telemetry.IsisTypes_SAFI_TYPE_UNICAST).Metric()
	assert.Value(t, metric, uint32(100))
	// TODO: Verify the link state database on the ATE once the ATE reports this properly
	// ateTelemPth := ts.ATEISISTelemetry(t)
	// ateDB := ateTelemPth.Level(2).LspAny()
	// for _, nbr := range ateDB.Tlv(telemetry.IsisLsdbTypes_ISIS_TLV_TYPE_IS_NEIGHBOR_ATTRIBUTE).IsisNeighborAttribute().NeighborAny().Get(t) {
	// }
}
