// Copyright 2024 Google LLC
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
	"testing"

	"github.com/google/go-cmp/cmp"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/binding"
	opb "github.com/openconfig/ondatra/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestDynamicReservation(t *testing.T) {
	tb := &opb.Testbed{
		Duts: []*opb.Device{{
			Id: "dut",
			Ports: []*opb.Port{{
				Id:    "port1",
				Speed: opb.Port_S_100GB,
			}, {
				Id: "port2",
			}},
		}},
		Ates: []*opb.Device{{
			Id: "ate",
			Ports: []*opb.Port{{
				Id:       "port1",
				PmdValue: &opb.Port_Pmd_{Pmd: opb.Port_PMD_100GBASE_CR4},
			}, {
				Id: "port2",
			}},
		}},
		Links: []*opb.Link{{
			A: "dut:port1",
			B: "ate:port2",
		}, {
			A: "dut:port2",
			B: "ate:port1",
		}},
	}

	b := &bindpb.Binding{
		Dynamic: true,
		Duts: []*bindpb.Device{{
			Name: "dut.name",
			Ports: []*bindpb.Port{{
				Name: "Ethernet1",
			}, {
				Name:  "Ethernet2",
				Speed: opb.Port_S_100GB,
			}},
		}},
		Ates: []*bindpb.Device{{
			Name: "ate.name",
			Ports: []*bindpb.Port{{
				Name: "1/1",
				Pmd:  opb.Port_PMD_100GBASE_CR4,
			}, {
				Name: "1/2",
			}},
		}},
		Links: []*bindpb.Link{{
			A: "dut.name:Ethernet2",
			B: "ate.name:1/2",
		}, {
			A: "ate.name:1/1",
			B: "dut.name:Ethernet1",
		}},
	}

	got, err := dynamicReservation(context.Background(), tb, resolver{b})
	if err != nil {
		t.Fatalf("dynamicReservation9) got unexpected error: %v", err)
	}
	want := &binding.Reservation{
		DUTs: map[string]binding.DUT{
			"dut": &staticDUT{
				AbstractDUT: &binding.AbstractDUT{Dims: &binding.Dims{
					Name: "dut.name",
					Ports: map[string]*binding.Port{
						"port1": {
							Name:  "Ethernet2",
							Speed: opb.Port_S_100GB,
						},
						"port2": {
							Name: "Ethernet1",
						},
					},
				}},
				r: resolver{b},
				dev: &bindpb.Device{
					Name: "dut.name",
					Ports: []*bindpb.Port{
						{
							Name: "Ethernet1",
						}, {
							Name:  "Ethernet2",
							Speed: opb.Port_S_100GB,
						},
					},
				},
			},
		},
		ATEs: map[string]binding.ATE{
			"ate": &staticATE{
				AbstractATE: &binding.AbstractATE{Dims: &binding.Dims{
					Name: "ate.name",
					Ports: map[string]*binding.Port{
						"port1": {
							Name: "1/1",
							PMD:  opb.Port_PMD_100GBASE_CR4,
						},
						"port2": {
							Name: "1/2",
						},
					},
				}},
				r: resolver{b},
				dev: &bindpb.Device{
					Name: "ate.name",
					Ports: []*bindpb.Port{
						{
							Name: "1/1",
							Pmd:  opb.Port_PMD_100GBASE_CR4,
						}, {
							Name: "1/2",
						},
					},
				},
			},
		},
	}

	got.ID = ""
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(staticDUT{}, staticATE{}), protocmp.Transform()); diff != "" {
		t.Errorf("dynamicReservation() got unexpected diff (-want, +got):\n%s", diff)
	}
}

func TestDynamicReservationError(t *testing.T) {
	tb := &opb.Testbed{
		Duts: []*opb.Device{{
			Id: "dut",
			Ports: []*opb.Port{{
				Id:    "port1",
				Speed: opb.Port_S_100GB,
			}, {
				Id: "port2",
			}},
		}},
		Ates: []*opb.Device{{
			Id: "ate",
			Ports: []*opb.Port{{
				Id:       "port1",
				PmdValue: &opb.Port_Pmd_{Pmd: opb.Port_PMD_100GBASE_CR4},
			}, {
				Id: "port2",
			}},
		}},
		Links: []*opb.Link{{
			A: "dut:port1",
			B: "ate:port2",
		}, {
			A: "dut:port2",
			B: "ate:port1",
		}},
	}

	b := &bindpb.Binding{
		Dynamic: true,
		Duts: []*bindpb.Device{{
			Name: "dut.name",
			Ports: []*bindpb.Port{{
				Name: "Ethernet1",
			}, {
				Name:  "Ethernet2",
				Speed: opb.Port_S_100GB,
			}},
		}},
		Ates: []*bindpb.Device{{
			Name: "ate.name",
			Ports: []*bindpb.Port{{
				Name: "1/1",
			}, {
				Name: "1/2",
			}},
		}},
		Links: []*bindpb.Link{{
			A: "dut.name:Ethernet2",
			B: "ate.name:1/2",
		}, {
			A: "ate.name:1/1",
			B: "dut.name:Ethernet1",
		}},
	}

	got, err := dynamicReservation(context.Background(), tb, resolver{b})
	if err == nil {
		t.Fatalf("dynamicReservation() got unexpected success: %v", got)
	}
}
