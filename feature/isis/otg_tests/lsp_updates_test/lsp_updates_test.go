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

	"github.com/openconfig/featureprofiles/internal/check"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOverloadBit(t *testing.T) {
	ts := isissession.MustNew(t).WithISIS()
	// Only push DUT config - no adjacency established yet
	if err := ts.PushDUT(context.Background(), t); err != nil {
		t.Fatalf("Unable to push initial DUT config: %v", err)
	}
	isisPath := isissession.ISISPath(ts.DUT)
	overloads := isisPath.Level(2).SystemLevelCounters().DatabaseOverloads()
	//Lookup the initial value for 'database-overloads' leaf counter after config is pushed to DUT & before adjacency is formed
	getDbOlInitCount := gnmi.Lookup(t, ts.DUT, overloads.State())
	olVal, present := getDbOlInitCount.Val()
	if !present {
		olVal = uint32(0)
	}
	ts.ATE = ondatra.ATE(t, "ate")
	otg := ts.ATE.OTG()
	ts.PushAndStart(t)
	ts.MustAdjacency(t)
	setBit := isisPath.Global().LspBit().OverloadBit().SetBit()
	deadline := time.Now().Add(time.Second * 3)
	checkSetBit := check.Equal(setBit.State(), false)
	if deviations.MissingValueForDefaults(ts.DUT) {
		checkSetBit = check.EqualOrNil(setBit.State(), false)
	}

	for _, vd := range []check.Validator{
		checkSetBit,
		check.EqualOrNil(overloads.State(), olVal),
	} {
		if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
			t.Error(err)
		}
	}
	ts.DUTConf.
		GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).
		GetProtocol(isissession.PTISIS, isissession.ISISName).
		GetIsis().
		GetGlobal().
		GetOrCreateLspBit().
		GetOrCreateOverloadBit().SetBit = ygot.Bool(true)
	ts.PushDUT(context.Background(), t)
	if err := check.Equal(overloads.State(), uint32(olVal+1)).AwaitFor(time.Second*10, ts.DUTClient); err != nil {
		t.Error(err)
	}
	if err := check.Equal(setBit.State(), true).AwaitFor(time.Second*3, ts.DUTClient); err != nil {
		t.Error(err)
	}

	_, ok := gnmi.WatchAll(t, otg, gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Flags().State(), time.Minute, func(v *ygnmi.Value[[]otgtelemetry.E_Lsps_Flags]) bool {
		flags, present := v.Val()
		if present {
			for _, flag := range flags {
				if flag == otgtelemetry.Lsps_Flags_OVERLOAD {
					return true
				}
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
	ts := isissession.MustNew(t).WithISIS()
	ts.ATE = ondatra.ATE(t, "ate")
	configuredMetric := uint32(100)
	otg := ts.ATE.OTG()
	isisIntfName := ts.DUT.Port(t, "port1").Name()
	if deviations.ExplicitInterfaceInDefaultVRF(ts.DUT) || deviations.InterfaceRefInterfaceIDFormat(ts.DUT) {
		isisIntfName = ts.DUT.Port(t, "port1").Name() + ".0"
	}
	ts.DUTConf.GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).GetProtocol(isissession.PTISIS, isissession.ISISName).GetIsis().
		GetInterface(isisIntfName).
		GetOrCreateLevel(2).
		GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).
		Metric = ygot.Uint32(configuredMetric)
	ts.DUTConf.GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).GetProtocol(isissession.PTISIS, isissession.ISISName).GetIsis().GetOrCreateLevel(2).
		MetricStyle = oc.E_Isis_MetricStyle(2)

	ts.PushAndStart(t)
	ts.MustAdjacency(t)

	metric := isissession.ISISPath(ts.DUT).Interface(isisIntfName).Level(2).
		Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric()
	if err := check.Equal(metric.State(), uint32(100)).AwaitFor(time.Second*3, ts.DUTClient); err != nil {
		t.Error(err)
	}

	_, ok := gnmi.WatchAll(t, otg, gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().PrefixAny().Metric().State(), time.Minute, func(v *ygnmi.Value[uint32]) bool {
		metric, present := v.Val()
		if present {
			if metric == configuredMetric {
				return true
			}
		}
		return false
	}).Await(t)

	metricInReceivedLsp := gnmi.GetAll(t, otg, gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().PrefixAny().Metric().State())[0]
	if !ok {
		t.Fatalf("Metric not matched. Expected %d got %d ", configuredMetric, metricInReceivedLsp)
	}
}
