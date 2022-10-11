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

	"github.com/openconfig/featureprofiles/feature/experimental/isis/otg_tests/internal/assert"
	"github.com/openconfig/featureprofiles/feature/experimental/isis/otg_tests/internal/session"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOverloadBit(t *testing.T) {
	ts := session.NewWithISIS(t)
	ts.ATE = ondatra.ATE(t, "ate")
	otg := ts.ATE.OTG()
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

	//TOD: Replace this with ts.PushDUT
	ts.PushDUTOverLoadBit(t)

	overloads.Await(t, time.Second*10, 1)
	assert.Value(t, setBit, true)

	_, ok := otg.Telemetry().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Flags().Watch(
		t, time.Minute, func(val *otgtelemetry.QualifiedE_Lsps_FlagsSlice) bool {
			for _, flag := range val.Val(t) {
				if flag == otgtelemetry.Lsps_Flags_OVERLOAD {
					return true
				}
			}
			return false
		}).Await(t)

	if !ok {
		t.Fatalf("OverLoad Bit not seen on learned lsp on ATE")
	}
}

func TestMetric(t *testing.T) {
	t.Logf("Starting...")
	ts := session.NewWithISIS(t)
	ts.ATE = ondatra.ATE(t, "ate")
	configuredMetric := uint32(100)
	otg := ts.ATE.OTG()
	ts.DUTConf.GetNetworkInstance(*deviations.DefaultNetworkInstance).GetProtocol(session.PTISIS, session.ISISName).GetIsis().
		GetInterface(ts.DUT.Port(t, "port1").Name()).
		GetOrCreateLevel(2).
		GetOrCreateAf(telemetry.IsisTypes_AFI_TYPE_IPV4, telemetry.IsisTypes_SAFI_TYPE_UNICAST).
		Metric = ygot.Uint32(configuredMetric)
	ts.PushAndStart(t)
	ts.AwaitAdjacency(t)

	telemPth := ts.DUTISISTelemetry(t)
	metric := telemPth.Interface(ts.DUT.Port(t, "port1").Name()).Level(2).
		Af(telemetry.IsisTypes_AFI_TYPE_IPV4, telemetry.IsisTypes_SAFI_TYPE_UNICAST).Metric()
	assert.Value(t, metric, uint32(100))

	_, ok := otg.Telemetry().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().PrefixAny().Metric().Watch(
		t, time.Minute, func(val *otgtelemetry.QualifiedUint32) bool {
			return val.Val(t) == configuredMetric
		}).Await(t)

	metricInReceivedLsp := otg.Telemetry().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().PrefixAny().Metric().Get(t)

	if !ok {
		t.Fatalf("Metric not matched. Expected %d got %d ", configuredMetric, metricInReceivedLsp)
	}
}
