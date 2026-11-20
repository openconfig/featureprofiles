// Copyright 2026 Google LLC
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

package aggregate_config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const aggregate = "PortChannel100"

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func ethernetInterface(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	for _, intf := range gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State()) {
		name := intf.GetName()
		if strings.HasPrefix(name, "Ethernet") {
			return name
		}
	}
	t.Fatal("no Ethernet interface in state")
	return ""
}

// TestAggregateConfig covers lag-type, aggregate-id, and both LACP intervals.
func TestAggregateConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	member := ethernetInterface(t, dut)
	aggPath := gnmi.OC().Interface(aggregate)
	memberAgg := gnmi.OC().Interface(member).Ethernet().AggregateId()
	lacpPath := gnmi.OC().Lacp().Interface(aggregate)
	originalAgg, hadAgg := gnmi.LookupConfig(t, dut, memberAgg.Config()).Val()
	t.Cleanup(func() {
		gnmi.Delete(t, dut, lacpPath.Config())
		if hadAgg {
			gnmi.Replace(t, dut, memberAgg.Config(), originalAgg)
		} else {
			gnmi.Delete(t, dut, memberAgg.Config())
		}
		gnmi.Delete(t, dut, aggPath.Config())
	})

	t.Log("Create an LACP aggregate and add one member")
	agg := &oc.Interface{Name: ygot.String(aggregate)}
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().SetLagType(oc.IfAggregate_AggregationType_LACP)
	gnmi.Replace(t, dut, aggPath.Config(), agg)
	gnmi.Replace(t, dut, memberAgg.Config(), aggregate)

	if got := gnmi.Get(t, dut, aggPath.Aggregation().LagType().Config()); got != oc.IfAggregate_AggregationType_LACP {
		t.Errorf("lag-type config = %v, want LACP", got)
	}
	if got := gnmi.Get(t, dut, memberAgg.Config()); got != aggregate {
		t.Errorf("aggregate-id config = %q, want %q", got, aggregate)
	}

	for _, interval := range []oc.E_Lacp_LacpPeriodType{oc.Lacp_LacpPeriodType_FAST, oc.Lacp_LacpPeriodType_SLOW} {
		t.Logf("Set LACP interval %s", interval)
		lacp := &oc.Lacp_Interface{Name: ygot.String(aggregate)}
		lacp.SetInterval(interval)
		gnmi.Replace(t, dut, lacpPath.Config(), lacp)
		if got := gnmi.Get(t, dut, lacpPath.Interval().Config()); got != interval {
			t.Errorf("interval config = %v, want %v", got, interval)
		}
		gnmi.Await(t, dut, lacpPath.Interval().State(), time.Minute, interval)
	}
}
