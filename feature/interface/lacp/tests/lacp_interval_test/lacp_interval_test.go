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

package lacp_interval_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed consists of dut1:port{1-2} -> dut2:port{1-2}
//
//   - Source: dut1:port{1-2} -> dut2:port{1-2} subnet 192.0.2.0/30 2001:db8::0/126

// Note that the first (.0) and last (.3) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses.  This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.

const (
	plen4          = 30
	plen6          = 126
	opUp           = oc.Interface_OperStatus_UP
	adminUp        = oc.Interface_AdminStatus_UP
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	lagTypeLACP    = oc.IfAggregate_AggregationType_LACP
)

var (
	dut1Src = attrs.Attributes{
		Desc:    "dutdut",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dut2Src = attrs.Attributes{
		Desc:    "dutdut",
		Name:    "atesrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

type testCase struct {
	lagType      oc.E_IfAggregate_AggregationType
	dut1         *ondatra.DUTDevice
	dut2         *ondatra.DUTDevice
	lacpInterval oc.E_Lacp_LacpPeriodType

	// dutPorts is the set of ports the DUT -- the first (i.e., dutPorts[0])
	// is not configured in the aggregate interface.
	dut1Ports []*ondatra.Port
	// atePorts is the set of ports on the ATE -- the first, as with the DUT
	// is not configured in the aggregate interface.
	// is not configured in the aggregate interface.
	dut2Ports []*ondatra.Port
	aggID     string
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func (tc *testCase) verifyAggID(t *testing.T, dp *ondatra.Port, dut *ondatra.DUTDevice) {
	dip := gnmi.OC().Interface(dp.Name())
	di := gnmi.Get(t, dut, dip.State())
	if lagID := di.GetEthernet().GetAggregateId(); lagID != tc.aggID {
		t.Errorf("%s LagID got %v, want %v", dp, lagID, tc.aggID)
	}
}

func (tc *testCase) configureDUT(t *testing.T) {
	t.Logf("dut1 ports = %v, dut2 ports = %v", tc.dut1Ports, tc.dut2Ports)
	if len(tc.dut1Ports) < 2 || len(tc.dut2Ports) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got %d and %d", len(tc.dut1Ports), len(tc.dut2Ports))
	}

	lacpMode := oc.Lacp_LacpActivityType_ACTIVE
	lacpData := &cfgplugins.LACPParams{
		Activity: &lacpMode,
		Period:   &tc.lacpInterval,
	}

	dut1AggData := &cfgplugins.DUTAggData{
		LagName:      tc.aggID,
		AggType:      tc.lagType,
		LacpParams:   lacpData,
		Attributes:   dut1Src,
		OndatraPorts: tc.dut1Ports,
	}

	dut1Batch := &gnmi.SetBatch{}
	cfgplugins.NewAggregateInterface(t, tc.dut1, dut1Batch, dut1AggData)
	dut1Batch.Set(t, tc.dut1)
	t.Logf("Configured DUT1 aggregate interface %s", tc.aggID)

	dut2AggData := &cfgplugins.DUTAggData{
		LagName:      tc.aggID,
		AggType:      tc.lagType,
		LacpParams:   lacpData,
		Attributes:   dut2Src,
		OndatraPorts: tc.dut2Ports,
	}

	dut2Batch := &gnmi.SetBatch{}
	cfgplugins.NewAggregateInterface(t, tc.dut2, dut2Batch, dut2AggData)
	dut2Batch.Set(t, tc.dut2)
	t.Logf("Configured DUT2 aggregate interface %s", tc.aggID)

	gnmi.Await(t, tc.dut2, gnmi.OC().Interface(tc.aggID).Type().State(), time.Minute, ieee8023adLag)

	if deviations.ExplicitInterfaceInDefaultVRF(tc.dut1) {
		duts := []*ondatra.DUTDevice{tc.dut1, tc.dut2}
		for _, dut := range duts {
			fptest.AssignToNetworkInstance(t, dut, tc.aggID,
				deviations.DefaultNetworkInstance(dut), 0)
		}
	}

	if deviations.ExplicitPortSpeed(tc.dut1) {
		for _, port := range tc.dut1Ports {
			fptest.SetPortSpeed(t, port)
		}
	}
	if deviations.ExplicitPortSpeed(tc.dut2) {
		for _, port := range tc.dut2Ports {
			fptest.SetPortSpeed(t, port)
		}
	}
}

func (tc *testCase) verifyDUT(t *testing.T) {
	// Wait for LAG negotiation and verify LAG type for the aggregate interface.
	gnmi.Await(t, tc.dut1, gnmi.OC().Interface(tc.aggID).Type().State(), time.Minute, ieee8023adLag)

	for _, port := range tc.dut1Ports {
		tc.verifyInterfaceDUT(t, port, tc.dut1)
		tc.verifyAggID(t, port, tc.dut1)
	}
	for _, port := range tc.dut2Ports {
		tc.verifyInterfaceDUT(t, port, tc.dut2)
		tc.verifyAggID(t, port, tc.dut2)
	}
}

func (tc *testCase) verifyLACPInterval(t *testing.T) {
	lacpIntervals := gnmi.OC().Lacp().Interface(tc.aggID).Interval().State()
	lacpIntervalDUT1 := gnmi.Get(t, tc.dut1, lacpIntervals)
	lacpIntervalDUT2 := gnmi.Get(t, tc.dut2, lacpIntervals)

	if lacpIntervalDUT1 != lacpIntervalDUT2 {
		t.Errorf("LACP Interval is not same on both the DUTs, DUT1: %v, DUT2: %v", lacpIntervalDUT1, lacpIntervalDUT2)
	}
	if lacpIntervalDUT1 != tc.lacpInterval {
		t.Errorf("LACP Interval is not same as configured, got: %v, want: %v", lacpIntervalDUT1, tc.lacpInterval)
	} else {
		t.Logf("LACP Interval is same as configured, got: %v, want: %v", lacpIntervalDUT1, tc.lacpInterval)
	}
}

func (tc *testCase) verifyInterfaceDUT(t *testing.T, dp *ondatra.Port, dut *ondatra.DUTDevice) {
	dip := gnmi.OC().Interface(dp.Name())
	di := gnmi.Get(t, dut, dip.State())
	fptest.LogQuery(t, dp.String()+" before Await", dip.State(), di)

	if got := di.GetAdminStatus(); got != adminUp {
		t.Errorf("%s admin-status got %v, want %v", dp, got, adminUp)
	}

	// LAG members may fall behind, so wait for them to be up.
	gnmi.Await(t, dut, dip.OperStatus().State(), time.Minute, opUp)
}

func TestLacpTimers(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	aggID := netutil.NextAggregateInterface(t, dut1)

	lacpIntervals := []oc.E_Lacp_LacpPeriodType{oc.Lacp_LacpPeriodType_FAST, oc.Lacp_LacpPeriodType_SLOW}

	for _, lacpInterval := range lacpIntervals {

		tc := &testCase{
			dut1:         dut1,
			dut2:         dut2,
			lagType:      lagTypeLACP,
			lacpInterval: lacpInterval,
			dut1Ports:    sortPorts(dut1.Ports()),
			dut2Ports:    sortPorts(dut2.Ports()),
			aggID:        aggID,
		}
		t.Run(fmt.Sprintf("lacpInterval=%s", lacpInterval), func(t *testing.T) {
			tc.configureDUT(t)
			tc.verifyDUT(t)
			tc.verifyLACPInterval(t)
		})
	}
}
