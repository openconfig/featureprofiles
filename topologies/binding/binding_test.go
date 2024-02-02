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

package binding

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/binding"
	opb "github.com/openconfig/ondatra/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestReserveRelease(t *testing.T) {
	ctx := context.Background()
	tb := &opb.Testbed{}
	b := &staticBind{r: resolver{&bindpb.Binding{}}, pushConfig: false}

	if err := b.Release(ctx); err == nil {
		t.Error("Release should fail before reservation is made.")
	}

	_, err := b.Reserve(ctx, tb, 0, 0, nil)
	if err != nil {
		t.Fatalf("Could not reserve testbed: %v", err)
	}
	if err := b.Release(ctx); err != nil {
		t.Errorf("Could not release reservation: %v", err)
	}

	if err := b.Release(ctx); err == nil {
		t.Error("Release should fail after reservation is already released.")
	}
}

func TestReservation(t *testing.T) {
	tb := &opb.Testbed{
		Duts: []*opb.Device{{
			Id: "dut",
			Ports: []*opb.Port{{
				Id: "port1",
			}, {
				Id: "port2",
			}},
		}},
		Ates: []*opb.Device{{
			Id: "ate",
			Ports: []*opb.Port{{
				Id: "port1",
			}, {
				Id: "port2",
			}},
		}},
	}

	b := &bindpb.Binding{
		Duts: []*bindpb.Device{{
			Id:   "dut",
			Name: "dut.name",
			Ports: []*bindpb.Port{{
				Id:   "port1",
				Name: "Ethernet1",
			}, {
				Id:   "port2",
				Name: "Ethernet2",
			}},
		}},
		Ates: []*bindpb.Device{{
			Id:   "ate",
			Name: "ate.name",
			Ports: []*bindpb.Port{{
				Id:   "port1",
				Name: "1/1",
			}, {
				Id:   "port2",
				Name: "1/2",
			}},
		}},
	}

	got, err := reservation(tb, resolver{b})
	if err != nil {
		t.Fatalf("Error building reservation: %v", err)
	}
	want := &binding.Reservation{
		DUTs: map[string]binding.DUT{
			"dut": &staticDUT{
				AbstractDUT: &binding.AbstractDUT{Dims: &binding.Dims{
					Name: "dut.name",
					Ports: map[string]*binding.Port{
						"port1": {Name: "Ethernet1"},
						"port2": {Name: "Ethernet2"},
					},
				}},
				r: resolver{b},
				dev: &bindpb.Device{
					Id:   "dut",
					Name: "dut.name",
					Ports: []*bindpb.Port{
						{Id: "port1", Name: "Ethernet1"},
						{Id: "port2", Name: "Ethernet2"},
					},
				},
			},
		},
		ATEs: map[string]binding.ATE{
			"ate": &staticATE{
				AbstractATE: &binding.AbstractATE{Dims: &binding.Dims{
					Name: "ate.name",
					Ports: map[string]*binding.Port{
						"port1": {Name: "1/1"},
						"port2": {Name: "1/2"},
					},
				}},
				r: resolver{b},
				dev: &bindpb.Device{
					Id:   "ate",
					Name: "ate.name",
					Ports: []*bindpb.Port{
						{Id: "port1", Name: "1/1"},
						{Id: "port2", Name: "1/2"},
					},
				},
			},
		},
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(staticDUT{}, staticATE{}), protocmp.Transform()); diff != "" {
		t.Errorf("Reservation -want, +got:\n%s", diff)
	}
}

func TestReservation_Error(t *testing.T) {
	tb := &opb.Testbed{
		Duts: []*opb.Device{{
			Id: "dut.tb", // only in testbed.
		}, {
			Id: "dut.both",
			Ports: []*opb.Port{{
				Id: "port1",
			}, {
				Id: "port2",
			}},
		}},
		Ates: []*opb.Device{{
			Id: "ate.tb", // only in testbed.
		}, {
			Id: "ate.both",
			Ports: []*opb.Port{{
				Id: "port1",
			}, {
				Id: "port2",
			}},
		}},
	}

	b := &bindpb.Binding{
		Duts: []*bindpb.Device{{
			Id:   "dut.b", // only in binding.
			Name: "dut.b.name",
		}, {
			Id:   "dut.both",
			Name: "dut.both.name",
			Ports: []*bindpb.Port{{ // port1 missing, port3 extra
				Id:   "port2",
				Name: "Ethernet2",
			}, {
				Id:   "port3",
				Name: "Ethernet3",
			}},
		}},
		Ates: []*bindpb.Device{{
			Id:   "ate.both",
			Name: "ate.name",
			Ports: []*bindpb.Port{{ // port1 missing, port3 extra
				Id:   "port2",
				Name: "1/2",
			}, {
				Id:   "port3",
				Name: "1/3",
			}},
		}},
	}

	_, err := reservation(tb, resolver{b})
	if err == nil {
		t.Fatalf("Error building reservation: %v", err)
	}
	t.Logf("Got reservation errors: %v", err)

	wants := []string{
		`missing binding for DUT "dut.tb"`,
		`error binding DUT "dut.both"`,
		`binding DUT "dut.b" not found in testbed`,
		`missing binding for ATE "ate.tb"`,
		`error binding ATE "ate.both"`,
		`testbed port "port1" is missing in binding`,
	}
	errText := err.Error()

	for _, want := range wants {
		if !strings.Contains(errText, want) {
			t.Errorf("Want error not found: %s", want)
		}
	}
}

func TestDialOTGTimeout(t *testing.T) {
	const timeoutSecs = 42
	a := &staticATE{
		AbstractATE: &binding.AbstractATE{Dims: &binding.Dims{Name: "my_ate"}},
		r:           resolver{&bindpb.Binding{}},
		dev: &bindpb.Device{Otg: &bindpb.Options{
			Timeout: timeoutSecs,
		}},
	}
	grpcDialContextFn = func(context.Context, string, ...grpc.DialOption) (*grpc.ClientConn, error) {
		return nil, nil
	}
	gosnappiNewAPIFn = func() gosnappi.Api {
		return &captureAPI{Api: gosnappi.NewApi()}
	}
	api, err := a.DialOTG(context.Background())
	if err != nil {
		t.Errorf("DialOTG() got error %v", err)
	}
	gotTransport := api.(*captureAPI).gotTransport
	if gotTimeout, wantTimeout := gotTransport.RequestTimeout(), timeoutSecs*time.Second; gotTimeout != wantTimeout {
		t.Errorf("DialOTG() got timeout %v, want %v", gotTimeout, wantTimeout)
	}
}

type captureAPI struct {
	gosnappi.Api
	gotTransport gosnappi.GrpcTransport
}

func (a *captureAPI) NewGrpcTransport() gosnappi.GrpcTransport {
	a.gotTransport = a.Api.NewGrpcTransport()
	return a.gotTransport
}
