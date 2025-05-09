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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/introspect"
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

func TestStaticReservation(t *testing.T) {
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

	got, err := staticReservation(tb, resolver{b})
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

func TestStaticReservation_Error(t *testing.T) {
	tb := &opb.Testbed{
		Duts: []*opb.Device{{
			Id: "dut.tb", // only in testbed.
		}, {
			Id:                   "dut.both",
			Vendor:               opb.Device_CIENA,                                         // only in testbed
			HardwareModelValue:   &opb.Device_HardwareModel{HardwareModel: "modelA"},       // differs from binding
			SoftwareVersionValue: &opb.Device_SoftwareVersion{SoftwareVersion: "versionB"}, // matches binding
			Ports: []*opb.Port{{
				Id: "port1",
			}, {
				Id:       "port2",
				Speed:    opb.Port_S_100GB,                                // only in testbed
				PmdValue: &opb.Port_Pmd_{Pmd: opb.Port_PMD_100GBASE_CLR4}, // differs in binding
			}},
		}},
		Ates: []*opb.Device{{
			Id: "ate.tb", // only in testbed.
		}, {
			Id: "ate.both",
			Ports: []*opb.Port{{
				Id:    "port1",
				Speed: opb.Port_S_10GB, // matches binding
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
			Id:              "dut.both",
			Name:            "dut.both.name",
			HardwareModel:   "modelB",   // differs in testbed
			SoftwareVersion: "versionB", // differs in binding
			Ports: []*bindpb.Port{{ // port1 missing, port3 extra
				Id:   "port2",
				Name: "Ethernet2",
				Pmd:  opb.Port_PMD_400GBASE_DR4, // differs in testbed
			}, {
				Id:   "port3",
				Name: "Ethernet3",
			}},
		}},
		Ates: []*bindpb.Device{{
			Id:     "ate.both",
			Name:   "ate.name",
			Vendor: opb.Device_IXIA, // only in binding
			Ports: []*bindpb.Port{{ // port2 missing, port3 extra
				Id:    "port1",
				Name:  "1/1",
				Speed: opb.Port_S_10GB,          // matches testbed
				Pmd:   opb.Port_PMD_40GBASE_SR4, // only in binding
			}, {
				Id:   "port3",
				Name: "1/3",
			}},
		}},
	}

	r, errs := staticReservation(tb, resolver{b})
	if len(errs) == 0 {
		t.Fatalf("staticReservation() unexpectedly succeeded: %v", r)
	}

	wants := []string{
		`missing binding for DUT "dut.tb"`,
		`binding vendor`,
		`binding hardware model`,
		`missing binding for port "port1" on "dut.both"`,
		`binding port speed`,
		`binding port PMD`,
		`missing binding for ATE "ate.tb"`,
		`missing binding for port "port2" on "ate.both"`,
	}
	if got, want := len(errs), len(wants); got != want {
		t.Errorf("staticReservation() got %d errors, want %d: %v", got, want, errs)
	}
	for i, err := range errs {
		if got, want := err.Error(), wants[i]; !strings.Contains(got, want) {
			t.Errorf("staticReservation() got error %q, want: %q", got, want)
		}
	}
}

func TestDialOTGTimeout(t *testing.T) {
	const timeoutSecs = 42
	a := &staticATE{
		r:   resolver{&bindpb.Binding{}},
		dev: &bindpb.Device{Otg: &bindpb.Options{Timeout: timeoutSecs}},
	}
	grpcDialContextFn = func(string, ...grpc.DialOption) (*grpc.ClientConn, error) {
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

func TestDialer(t *testing.T) {
	const (
		wantDevName = "mydev"
		wantDevPort = 1234
	)
	fakeSvc := introspect.Service("fake")
	dutSvcParams[fakeSvc] = &svcParams{
		port:   wantDevPort,
		optsFn: func(d *bindpb.Device) *bindpb.Options { return nil },
	}
	d := &staticDUT{
		r:   resolver{&bindpb.Binding{}},
		dev: &bindpb.Device{Name: wantDevName},
	}

	dialer, err := d.Dialer(fakeSvc)
	if err != nil {
		t.Fatalf("Dialer() got err: %v", err)
	}
	if dialer.DevicePort != wantDevPort {
		t.Errorf("Dialer() got DevicePort %v, want %v", dialer.DevicePort, wantDevPort)
	}
	if dialer.DialFunc == nil {
		t.Errorf("Dialer() got nil DialFunc, want non-nil DialFunc")
	}
	if len(dialer.DialOpts) == 0 {
		t.Errorf("Dialer() got empty DialOpts, want non-empty DialOpts")
	}
	if wantTarget := fmt.Sprintf("%v:%v", wantDevName, wantDevPort); dialer.DialTarget != wantTarget {
		t.Errorf("Dialer() got Target %v, want %v", dialer.DialTarget, wantTarget)
	}
}
