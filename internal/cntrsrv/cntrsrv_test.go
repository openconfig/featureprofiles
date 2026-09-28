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
	"google.golang.org/grpc/metadata"
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
	spb.UnimplementedGRIBIServer
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
	gpb.UnimplementedGNMIServer
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

// authChecker is a gRPC interceptor that checks for username/password in metadata.
func authChecker(username, password string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}
		if len(md["username"]) == 0 || md["username"][0] != username {
			return nil, status.Errorf(codes.Unauthenticated, "invalid username")
		}
		if len(md["password"]) == 0 || md["password"][0] != password {
			return nil, status.Errorf(codes.Unauthenticated, "invalid password")
		}
		return handler(ctx, req)
	}
}

// buildAuthenticatedGNMIServer creates a fake gNMI server that requires authentication.
func buildAuthenticatedGNMIServer(t *testing.T, username, password string) func(uint) func() {
	return func(port uint) func() {
		tls, err := config.WithSelfTLSCert()
		if err != nil {
			t.Fatalf("cannot create server, %v", err)
		}
		srv := grpc.NewServer(tls, grpc.UnaryInterceptor(authChecker(username, password)))
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

// freePort returns a free port for testing.
func freePort(t *testing.T) int {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("freePort: could not resolve tcp addr: %v", err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatalf("freePort: could not listen on tcp addr: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestDial(t *testing.T) {
	timeFn = func() *timestamppb.Timestamp {
		return &timestamppb.Timestamp{
			Seconds: 42,
			Nanos:   42,
		}
	}

	// Dynamically get ports to avoid conflicts when running tests in parallel.
	serverPort := uint(freePort(t))
	badTargetPort := uint(freePort(t))
	gribiPort := uint(freePort(t))
	gnmiPort := uint(freePort(t))
	gnmiAuthPort := uint(freePort(t))
	gnmiBadAuthPort := uint(freePort(t))
	timeoutPort := uint(freePort(t))

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
		inServerPort: serverPort,
		inReq: &cpb.DialRequest{
			Addr: fmt.Sprintf("localhost:%d", serverPort),
		},
		wantResp: &cpb.DialResponse{},
	}, {
		desc:         "timeout",
		inServer:     startServer,
		inServerPort: serverPort,
		inReq: &cpb.DialRequest{
			Request: &cpb.DialRequest_Ping{
				Ping: &cpb.PingRequest{},
			},
			Addr: fmt.Sprintf("localhost:%d", timeoutPort),
		},
		wantErr: true,
	}, {
		desc:         "self-dial and ping",
		inServer:     startServer,
		inServerPort: serverPort,
		inReq: &cpb.DialRequest{
			Addr: fmt.Sprintf("localhost:%d", serverPort),
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
		inServerPort: serverPort,
		inRemote:     buildBadServer(t, PingError),
		inRemotePort: badTargetPort,
		inReq: &cpb.DialRequest{
			Addr: fmt.Sprintf("localhost:%d", badTargetPort),
			Request: &cpb.DialRequest_Ping{
				Ping: &cpb.PingRequest{},
			},
		},
		wantErr: true,
	}, {
		desc:         "gribi server",
		inServer:     startServer,
		inServerPort: serverPort,
		inRemote:     buildGRIBIServer(t),
		inRemotePort: gribiPort,
		inReq: &cpb.DialRequest{
			Addr: fmt.Sprintf("localhost:%d", gribiPort),
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
		inServerPort: serverPort,
		inRemote:     buildGNMIServer(t),
		inRemotePort: gnmiPort,
		inReq: &cpb.DialRequest{
			Addr: fmt.Sprintf("localhost:%d", gnmiPort),
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
	}, {
		desc:         "gnmi server with auth",
		inServer:     startServer,
		inServerPort: serverPort,
		inRemote:     buildAuthenticatedGNMIServer(t, "testuser", "testpass"),
		inRemotePort: gnmiAuthPort,
		inReq: &cpb.DialRequest{
			Addr:     fmt.Sprintf("localhost:%d", gnmiAuthPort),
			Username: "testuser",
			Password: "testpass",
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
	}, {
		desc:         "gnmi server with bad auth",
		inServer:     startServer,
		inServerPort: serverPort,
		inRemote:     buildAuthenticatedGNMIServer(t, "testuser", "testpass"),
		inRemotePort: gnmiBadAuthPort,
		inReq: &cpb.DialRequest{
			Addr:     fmt.Sprintf("localhost:%d", gnmiBadAuthPort),
			Username: "baduser",
			Password: "badpassword",
			Request: &cpb.DialRequest_Srv{
				Srv: cpb.Service_ST_GNMI,
			},
		},
		wantErr: true,
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
			client, stopC := newClient(t, tt.inServerPort)
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
