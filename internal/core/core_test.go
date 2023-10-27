// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/errdiff"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/eventlis"
	"github.com/openconfig/ondatra/fakebind"
	"google.golang.org/grpc"

	fpb "github.com/openconfig/gnoi/file"
	opb "github.com/openconfig/ondatra/proto"
)

type fakeGNOI struct {
	*binding.AbstractGNOIClients
	fakeFileClient *fakeFileClient
}

func (f *fakeGNOI) File() fpb.FileClient {
	return f.fakeFileClient
}

type fakeFileClient struct {
	fpb.FileClient
	statResponses []any
}

func (f *fakeFileClient) Stat(ctx context.Context, in *fpb.StatRequest, opts ...grpc.CallOption) (*fpb.StatResponse, error) {
	if len(f.statResponses) == 0 {
		return nil, fmt.Errorf("no more responses")
	}
	resp := f.statResponses[0]
	f.statResponses = f.statResponses[1:]
	switch v := resp.(type) {
	case *fpb.StatResponse:
		return v, nil
	case error:
		return nil, v
	}
	return nil, fmt.Errorf("invalid response type: %T", resp)
}

func TestCoreValidator(t *testing.T) {
	tests := []struct {
		desc       string
		duts       map[string]binding.DUT
		startErr   string
		stopErr    string
		cores      map[string]dutCoreFiles
		startCores map[string]dutCoreFiles
	}{{
		desc: "invalid dut vendor",
		duts: map[string]binding.DUT{
			"dut1": &fakebind.DUT{
				AbstractDUT: &binding.AbstractDUT{
					Dims: &binding.Dims{
						Vendor: opb.Device_VENDOR_UNSPECIFIED,
						Name:   "dut1",
					},
				},
			},
		},
		startCores: map[string]dutCoreFiles{},
		cores:      map[string]dutCoreFiles{},
	}, {
		desc: "dut gnoi error",
		duts: map[string]binding.DUT{
			"dut1": &fakebind.DUT{
				AbstractDUT: &binding.AbstractDUT{
					Dims: &binding.Dims{
						Vendor: opb.Device_ARISTA,
						Name:   "dut1",
					},
				},
				DialGNOIFn: func(_ context.Context, _ ...grpc.DialOption) (binding.GNOIClients, error) {
					return nil, fmt.Errorf("gnoi dial failed")
				},
			},
		},
		startCores: map[string]dutCoreFiles{},
		cores:      map[string]dutCoreFiles{},
	}, {
		desc: "dut gnoi rpc match stat fail",
		duts: map[string]binding.DUT{
			"dut1": &fakebind.DUT{
				AbstractDUT: &binding.AbstractDUT{
					Dims: &binding.Dims{
						Vendor: opb.Device_ARISTA,
						Name:   "dut1",
					},
				},
				DialGNOIFn: func(_ context.Context, _ ...grpc.DialOption) (binding.GNOIClients, error) {
					return &fakeGNOI{
						fakeFileClient: &fakeFileClient{
							statResponses: []any{
								fmt.Errorf("gnoi.File.Stat failed"),
								fmt.Errorf("gnoi.File.Stat failed"),
								fmt.Errorf("gnoi.File.Stat failed"),
								fmt.Errorf("gnoi.File.Stat failed"),
							},
						},
					}, nil
				},
			},
		},
		startCores: map[string]dutCoreFiles{
			"dut1": {
				DUT:    "dut1",
				Status: `DUT "dut1" failed to check cores: DUT "/var/core/": gnoi.File.Stat failed`,
			},
		},
		cores: map[string]dutCoreFiles{
			"dut1": {
				DUT:    "dut1",
				Status: `DUT "dut1" failed to check cores: DUT "/var/core/": gnoi.File.Stat failed`,
			},
		},
	}, {
		desc: "dut gnoi rpc file stat failed",
		duts: map[string]binding.DUT{
			"dut1": &fakebind.DUT{
				AbstractDUT: &binding.AbstractDUT{
					Dims: &binding.Dims{
						Vendor: opb.Device_ARISTA,
						Name:   "dut1",
					},
				},
				DialGNOIFn: func(_ context.Context, _ ...grpc.DialOption) (binding.GNOIClients, error) {
					return &fakeGNOI{
						fakeFileClient: &fakeFileClient{
							statResponses: []any{
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}},
								},
								fmt.Errorf("gnoi.File.Stat failed"),
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}},
								},
								fmt.Errorf("gnoi.File.Stat failed"),
							},
						},
					}, nil
				},
			},
		},
		startCores: map[string]dutCoreFiles{
			"dut1": {
				DUT:    "dut1",
				Status: `DUT "dut1" failed to check cores: DUT "dut1": unable to stat file "/var/core/core.1.tar.gz", gnoi.File.Stat failed`,
			},
		},
		cores: map[string]dutCoreFiles{
			"dut1": {
				DUT:    "dut1",
				Status: `DUT "dut1" failed to check cores: DUT "dut1": unable to stat file "/var/core/core.1.tar.gz", gnoi.File.Stat failed`,
			},
		},
	}, {
		desc: "dut gnoi pass no delta",
		duts: map[string]binding.DUT{
			"dut1": &fakebind.DUT{
				AbstractDUT: &binding.AbstractDUT{
					Dims: &binding.Dims{
						Vendor: opb.Device_ARISTA,
						Name:   "dut1",
					},
				},
				DialGNOIFn: func(_ context.Context, _ ...grpc.DialOption) (binding.GNOIClients, error) {
					return &fakeGNOI{
						fakeFileClient: &fakeFileClient{
							statResponses: []any{
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}},
								},
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}},
								},
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}},
								},
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}},
								},
							},
						},
					}, nil
				},
			},
		},
		startCores: map[string]dutCoreFiles{
			"dut1": {
				DUT: "dut1",
				Files: coreFiles{
					"/var/core/core.1.tar.gz": fileInfo{
						Name: "/var/core/core.1.tar.gz",
					},
				},
				Status: "OK",
			},
		},
		cores: map[string]dutCoreFiles{
			"dut1": {
				DUT:    "dut1",
				Files:  coreFiles{},
				Status: "OK",
			},
		},
	}, {
		desc: "dut gnoi pass delta",
		duts: map[string]binding.DUT{
			"dut1": &fakebind.DUT{
				AbstractDUT: &binding.AbstractDUT{
					Dims: &binding.Dims{
						Vendor: opb.Device_ARISTA,
						Name:   "dut1",
					},
				},
				DialGNOIFn: func(_ context.Context, _ ...grpc.DialOption) (binding.GNOIClients, error) {
					return &fakeGNOI{
						fakeFileClient: &fakeFileClient{
							statResponses: []any{
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}},
								},
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}},
								},
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}, {
										Path: "/var/core/core.2.tar.gz",
									}},
								},
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.1.tar.gz",
									}},
								},
								&fpb.StatResponse{
									Stats: []*fpb.StatInfo{{
										Path: "/var/core/core.2.tar.gz",
									}},
								},
							},
						},
					}, nil
				},
			},
		},
		cores: map[string]dutCoreFiles{
			"dut1": {
				DUT: "dut1",
				Files: coreFiles{
					"/var/core/core.2.tar.gz": fileInfo{
						Name: "/var/core/core.2.tar.gz",
					},
				},
				Status: "OK",
			},
		},
		startCores: map[string]dutCoreFiles{
			"dut1": {
				DUT: "dut1",
				Files: coreFiles{
					"/var/core/core.1.tar.gz": fileInfo{
						Name: "/var/core/core.1.tar.gz",
					},
				},
				Status: "OK",
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			validator = validatorImpl{
				duts: map[string]*checker{},
			}
			cores := validator.start(tt.duts)
			if s := cmp.Diff(cores, tt.startCores); s != "" {
				t.Fatalf("Start(%+v) core check failed: %s", tt.duts, s)
			}
			cores = validator.stop()
			if s := cmp.Diff(cores, tt.cores); s != "" {
				t.Fatalf("Stop() core check failed: %s", s)
			}
		})
	}
}

func TestEventCallback(t *testing.T) {
	tests := []struct {
		desc      string
		dut       *fakebind.DUT
		beforeErr string
		afterErr  string
	}{{
		desc: "Fail to register (this will only log error)",
		dut: &fakebind.DUT{
			AbstractDUT: &binding.AbstractDUT{
				Dims: &binding.Dims{
					Vendor: opb.Device_ARISTA,
				},
			},
			DialGNOIFn: func(_ context.Context, _ ...grpc.DialOption) (binding.GNOIClients, error) {
				return &fakeGNOI{
					fakeFileClient: &fakeFileClient{
						statResponses: []any{
							fmt.Errorf("gnoi.File.Stat failed"),
						},
					},
				}, nil
			},
		},
	}, {
		desc: "Fail on stop (this will also be ignored)",
		dut: &fakebind.DUT{
			AbstractDUT: &binding.AbstractDUT{
				Dims: &binding.Dims{
					Vendor: opb.Device_ARISTA,
					Name:   "dut1",
				},
			},
			DialGNOIFn: func(_ context.Context, _ ...grpc.DialOption) (binding.GNOIClients, error) {
				return &fakeGNOI{
					fakeFileClient: &fakeFileClient{
						statResponses: []any{
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}},
							},
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}},
							},
							fmt.Errorf("gnoi.File.Stat failed"),
						},
					},
				}, nil
			},
		},
	}, {
		desc: "After returns no new core",
		dut: &fakebind.DUT{
			AbstractDUT: &binding.AbstractDUT{
				Dims: &binding.Dims{
					Vendor: opb.Device_ARISTA,
					Name:   "dut1",
				},
			},
			DialGNOIFn: func(_ context.Context, _ ...grpc.DialOption) (binding.GNOIClients, error) {
				return &fakeGNOI{
					fakeFileClient: &fakeFileClient{
						statResponses: []any{
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}},
							},
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}},
							},
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}},
							},
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}},
							},
						},
					},
				}, nil
			},
		},
	}, {
		desc: "After returns error for core found",
		dut: &fakebind.DUT{
			AbstractDUT: &binding.AbstractDUT{
				Dims: &binding.Dims{
					Vendor: opb.Device_ARISTA,
					Name:   "dut1",
				},
			},
			DialGNOIFn: func(_ context.Context, _ ...grpc.DialOption) (binding.GNOIClients, error) {
				return &fakeGNOI{
					fakeFileClient: &fakeFileClient{
						statResponses: []any{
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}},
							},
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}},
							},
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}, {
									Path: "/var/core/core.2.tar.gz",
								}},
							},
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.1.tar.gz",
								}},
							},
							&fpb.StatResponse{
								Stats: []*fpb.StatInfo{{
									Path: "/var/core/core.2.tar.gz",
								}},
							},
						},
					},
				}, nil
			},
		},
		afterErr: `Delta Core Files by DUT: 
DUT: dut1
  /var/core/core.2.tar.gz`,
	}}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			validator = validatorImpl{
				duts: map[string]*checker{},
			}
			e := &eventlis.BeforeTestsEvent{
				Reservation: &binding.Reservation{
					DUTs: map[string]binding.DUT{
						tt.dut.Name(): tt.dut,
					},
				},
			}
			beforeErr := registerBefore(e)
			if s := errdiff.Check(beforeErr, tt.beforeErr); s != "" {
				t.Fatalf("registerBefore failed: %v", s)
			}
			aE := &eventlis.AfterTestsEvent{
				ExitCode: new(int),
			}
			afterErr := registerAfter(aE)
			if s := errdiff.Check(afterErr, tt.afterErr); s != "" {
				t.Fatalf("registerAfter failed: %v", s)
			}
		})

	}
}
