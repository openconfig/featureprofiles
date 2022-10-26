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

package pre_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestResetGRIBIServerFP(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	clientA := gribi.Client{
		DUT:                  dut,
		FibACK:               false,
		Persistence:          true,
		InitialElectionIDLow: 1,
	}
	defer clientA.Close(t)
	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	clientA.BecomeLeader(t)
	clientA.FlushServer(t)
	_, err := dut.RawAPIs().GNOI().Default(t).System().KillProcess(ctx, &system.KillProcessRequest{Name: "emsd", Restart: true, Signal: system.KillProcessRequest_SIGNAL_TERM})
	if err != nil {
		t.Fatalf("%v", err)
	}
}
// this test perfroms get after emsd to mask the first get issue until it gets resolved
func TestFirstGet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dut.Telemetry().NetworkInstance("DEFAULT").Afts().NextHopAny().Get(t)
}
