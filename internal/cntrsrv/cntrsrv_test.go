/*
 Copyright 2023 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/testing/fake/testing/grpc/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	cpb "github.com/openconfig/featureprofiles/internal/cntrsrv/proto/cntr"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gribi/v1/proto/service"
)

func newClient(t *testing.T, port uint) (cpb.CntrClient, func()) {
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true, // NOLINT
		})))
	if err != nil {
		t.Fatalf("cannot dial, %v", err)
	}

	return cpb.NewCntrClient(conn), func() { conn.Close() }
}

type badMode int64

const (
	_ badMode = iota
	PingError
)

type badServer struct {
	mode badMode
	*cpb.UnimplementedCntrServer
}

func (b *badServer) Ping(_ context.Context, _ *cpb.PingRequest) (*cpb.PingResponse, error) {
	switch b.mode {
	case PingError:
		return nil, status.Errorf(codes.Internal, "can't generate a ping")
	default:
		return &cpb.PingResponse{}, nil
	}
}

func buildBadServer(t *testing.T, mode badMode) func(uint) func() {
	return func(port uint) func() {
		tls, err := config.WithSelfTLSCert()
		if err != nil {
			t.Fatalf("cannot create server, %v", err)
		}
		srv := grpc.NewServer(tls)
		s := &badServer{mode: mode}
		cpb.RegisterCntrServer(srv, s)

		lis, err := net.Listen("tcp", fmt.Sprintf("[::]:%d", port))
		if err != nil {
			t.Fatalf("cannot listen on port %d, got err: %v", port, err)
		}
		go srv.Serve(lis)
		return srv.Stop
	}
}

type gRIBIServer struct {
	*spb.UnimplementedGRIBIServer
}

func (g *gRIBIServer) Get(_ *spb.GetRequest, stream spb.GRIBI_GetServer) error {
	if err := stream.Send(&spb.GetResponse{}); err != nil {
		return status.Errorf(codes.Internal, "can't send")
	}
	return nil
}

func buildGRIBIServer(t *testing.T) func(uint) func() {
	return func(port uint) func() {
		tls, err := config.WithSelfTLSCert()
		if err != nil {
			t.Fatalf("cannot create server, %v", err)
		}
		srv := grpc.NewServer(tls)
		s := &gRIBIServer{}
		spb.RegisterGRIBIServer(srv, s)
		lis, err := net.Listen("tcp", fmt.Sprintf("[::]:%d", port))
		if err != nil {
			t.Fatalf("cannot listen on port %d, got err: %v", port, err)
		}
		go srv.Serve(lis)
		return srv.Stop
	}
}

type gNMIServer struct {
	*gpb.UnimplementedGNMIServer
}

func (g *gNMIServer) Capabilities(context.Context, *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
	return &gpb.CapabilityResponse{
		GNMIVersion: "demo",
	}, nil
}

func buildGNMIServer(t *testing.T) func(uint) func() {
	return func(port uint) func() {
		tls, err := config.WithSelfTLSCert()
		if err != nil {
			t.Fatalf("cannot create server, %v", err)
		}
		srv := grpc.NewServer(tls)
		s := &gNMIServer{}
		gpb.RegisterGNMIServer(srv, s)
		lis, err := net.Listen("tcp", fmt.Sprintf("[::]:%d", port))
		if err != nil {
			t.Fatalf("cannot listen on port %d, got err: %v", port, err)
		}
		go srv.Serve(lis)
		return srv.Stop
	}
}

func TestDial(t *testing.T) {
	timeFn = func() *timestamppb.Timestamp {
		return &timestamppb.Timestamp{
			Seconds: 42,
			Nanos:   42,
		}
	}

	tests := []struct {
		desc         string
		inServer     func(uint) func()
		inServerPort uint
		inRemote     func(uint) func()
		inRemotePort uint
		inReq        *cpb.DialRequest
		wantResp     *cpb.DialResponse
		wantErr      bool
	}{{
		desc:         "self-dial",
		inServer:     startServer,
		inServerPort: 60061,
		inReq: &cpb.DialRequest{
			Addr: "localhost:60061",
		},
		wantResp: &cpb.DialResponse{},
	}, {
		desc:         "timeout",
		inServer:     startServer,
		inServerPort: 60061,
		inReq: &cpb.DialRequest{
			Request: &cpb.DialRequest_Ping{
				Ping: &cpb.PingRequest{},
			},
			Addr: "localhost:6666",
		},
		wantErr: true,
	}, {
		desc:         "self-dial and ping",
		inServer:     startServer,
		inServerPort: 60061,
		inReq: &cpb.DialRequest{
			Addr: "localhost:60061",
			Request: &cpb.DialRequest_Ping{
				Ping: &cpb.PingRequest{},
			},
		},
		wantResp: &cpb.DialResponse{
			Response: &cpb.DialResponse_Pong{
				Pong: &cpb.PingResponse{
					Timestamp: &timestamppb.Timestamp{
						Seconds: 42,
						Nanos:   42,
					},
				},
			},
		},
	}, {
		desc:         "bad target server",
		inServer:     startServer,
		inServerPort: 60061,
		inRemote:     buildBadServer(t, PingError),
		inRemotePort: 60062,
		inReq: &cpb.DialRequest{
			Addr: "localhost:60062",
			Request: &cpb.DialRequest_Ping{
				Ping: &cpb.PingRequest{},
			},
		},
		wantErr: true,
	}, {
		desc:         "gribi server",
		inServer:     startServer,
		inServerPort: 60061,
		inRemote:     buildGRIBIServer(t),
		inRemotePort: 9339,
		inReq: &cpb.DialRequest{
			Addr: "localhost:9339",
			Request: &cpb.DialRequest_Srv{
				Srv: cpb.Service_ST_GRIBI,
			},
		},
		wantResp: &cpb.DialResponse{
			Response: &cpb.DialResponse_GribiResponse{
				GribiResponse: func() *anypb.Any {
					a, err := anypb.New(&spb.GetResponse{})
					if err != nil {
						t.Fatalf("cannot create gRIBI response, %v", err)
					}
					return a
				}(),
			},
		},
	}, {
		desc:         "gnmi server",
		inServer:     startServer,
		inServerPort: 60061,
		inRemote:     buildGNMIServer(t),
		inRemotePort: 9340,
		inReq: &cpb.DialRequest{
			Addr: "localhost:9340",
			Request: &cpb.DialRequest_Srv{
				Srv: cpb.Service_ST_GNMI,
			},
		},
		wantResp: &cpb.DialResponse{
			Response: &cpb.DialResponse_GnmiResponse{
				GnmiResponse: func() *anypb.Any {
					a, err := anypb.New(&gpb.CapabilityResponse{
						GNMIVersion: "demo",
					})
					if err != nil {
						t.Fatalf("cannot create gNMI response, %v", err)
					}
					return a
				}(),
			},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			stop := tt.inServer(tt.inServerPort)
			defer stop()
			if tt.inRemote != nil {
				stopRemote := tt.inRemote(tt.inRemotePort)
				defer stopRemote()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			client, stopC := newClient(t, 60061)
			defer stopC()

			got, err := client.Dial(ctx, tt.inReq)
			if (err != nil) != tt.wantErr {
				t.Fatalf("did not get expected error, got: %v, wantErr? %v", err, tt.wantErr)
			}
			t.Logf("got err: %v", err)
			if diff := cmp.Diff(got, tt.wantResp, protocmp.Transform()); diff != "" {
				t.Fatalf("did not get expected response, diff(-got,+want):\n%s", diff)
			}
		})
	}
}
