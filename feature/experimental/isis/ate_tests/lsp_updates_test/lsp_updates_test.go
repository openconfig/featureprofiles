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
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/experimental/isis/ate_tests/internal/session"
	"github.com/openconfig/featureprofiles/internal/check"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOverloadBit(t *testing.T) {
	ts := session.MustNew(t).WithISIS()
	ts.PushAndStart(t)
	ts.MustAdjacency(t)
	isisPath := session.ISISPath()
	overloads := isisPath.Level(2).SystemLevelCounters().DatabaseOverloads()
	setBit := isisPath.Global().LspBit().OverloadBit().SetBit()
	deadline := time.Now().Add(time.Second)
	checkSetBit := check.Equal(setBit.State(), false)
	if *deviations.MissingValueForDefaults {
		checkSetBit = check.EqualOrNil(setBit.State(), false)
	}

	for _, vd := range []check.Validator{
		checkSetBit,
		check.Equal(overloads.State(), uint32(0)),
	} {
		if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
			t.Error(err)
		}
	}
	ts.DUTConf.
		GetNetworkInstance(*deviations.DefaultNetworkInstance).
		GetProtocol(session.PTISIS, session.ISISName).
		GetIsis().
		GetGlobal().
		GetOrCreateLspBit().
		GetOrCreateOverloadBit().SetBit = ygot.Bool(true)
	ts.PushDUT(context.Background())
	// TODO: Verify the link state database once device support is added.
	if err := check.Equal[uint32](overloads.State(), 1).AwaitFor(time.Second*10, ts.DUTClient); err != nil {
		t.Error(err)
	}
	if err := check.Equal(setBit.State(), true).AwaitFor(time.Second, ts.DUTClient); err != nil {
		t.Error(err)
	}
	// TODO: Verify the link state database on the ATE once the ATE reports this properly
	// ateTelemPth := ts.ATEISISTelemetry(t)
	// ateDB := ateTelemPth.Level(2).LspAny()
	// for _, nbr := range ateDB.Tlv(telemetry.IsisLsdbTypes_ISIS_TLV_TYPE_IS_NEIGHBOR_ATTRIBUTE).IsisNeighborAttribute().NeighborAny().Get(t) {
	// }
}

func TestMetric(t *testing.T) {
	t.Logf("Starting...")
	ts := session.MustNew(t).WithISIS()
	ts.DUTConf.GetNetworkInstance(*deviations.DefaultNetworkInstance).GetProtocol(session.PTISIS, session.ISISName).GetIsis().
		GetInterface(ts.DUT.Port(t, "port1").Name()).
		GetOrCreateLevel(2).
		GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).
		Metric = ygot.Uint32(100)
	ts.PushAndStart(t)
	ts.MustAdjacency(t)

	metric := session.ISISPath().Interface(ts.DUTPort1.Name()).Level(2).
		Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric()
	if err := check.Equal(metric.State(), uint32(100)).AwaitFor(time.Second, ts.DUTClient); err != nil {
		t.Error(err)
	}
	// TODO: Verify the link state database on the ATE once the ATE reports this properly
	// ateTelemPth := ts.ATEISISTelemetry(t)
	// ateDB := ateTelemPth.Level(2).LspAny()
	// for _, nbr := range ateDB.Tlv(telemetry.IsisLsdbTypes_ISIS_TLV_TYPE_IS_NEIGHBOR_ATTRIBUTE).IsisNeighborAttribute().NeighborAny().Get(t) {
	// }
}
