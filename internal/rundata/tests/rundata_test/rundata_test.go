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

package rundata_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/rundata"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testingDUT struct {
	binding.AbstractDUT

	t  testing.TB
	id string
}

func (td *testingDUT) DialGNMI(context.Context, ...grpc.DialOption) (gpb.GNMIClient, error) {
	t := td.t
	dut := ondatra.DUT(t, td.id)
	return dut.RawAPIs().GNMI().Default(t), nil
}

func reservation(t testing.TB) *binding.Reservation {
	resv := &binding.Reservation{}
	resv.DUTs = make(map[string]binding.DUT)
	for _, dut := range ondatra.DUTs(t) {
		id := dut.ID()
		resv.DUTs[id] = &testingDUT{
			AbstractDUT: binding.AbstractDUT{
				Dims: &binding.Dims{Name: dut.Name()},
			},
			t:  t,
			id: id,
		}
	}
	return resv
}

func TestRunData(t *testing.T) {
	m := rundata.Properties(context.Background(), reservation(t))
	t.Log("rundata.Properties:", m)

	for k, v := range m {
		ondatra.Report().AddTestProperty(t, k, v)
	}
}
