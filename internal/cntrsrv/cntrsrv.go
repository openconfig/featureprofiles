/*
 Copyright 2022 Google LLC

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

// Binary cntrserver implements the Cntr (Container) service which can be used to test base
// functionalities of a container hosting device. It implements a service that can:
//   - dial a remote address as specified by the Dial RPC
//   - respond to a ping request with a specified timestamp.
//
// By running the CNTR server on one machine, A, one can validate:
//   - An external client can connect to a gRPC service running on A.
//
// By running the CNTR server on two machines, A and B, one can:
//   - Validate that A can dial B via gRPC by calling the Dial RPC on A with B's address.
//   - Validate that A can send an RPC to B via gRPC by calling the Dial RPC on A with B's address and ping.
package main

import (
	"context"
	"crypto/tls"
	"flag" // NOLINT
	"fmt"
	"net"

	"github.com/openconfig/gnmi/testing/fake/testing/grpc/config"
	"github.com/openconfig/ondatra/knebind/creds"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"

	cpb "github.com/openconfig/featureprofiles/internal/cntrsrv/proto/cntr"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gribi/v1/proto/service"
)

var (
	// timeFn is the function that returns a timestamp for Ping. It can be overloaded in order to
	// have deterministic output for unit testing.
	timeFn = timestamppb.Now

	// port is the port that this CNTR server should listen on.
	port = flag.Uint("port", 60061, "port for CNTR service to listen on.")
)

// C is the container for the CNTR server implementation.
type C struct {
	*cpb.UnimplementedCntrServer
}

// Ping implements the Ping RPC. It responds with a PingResponse corresponding to the timeFn timestamp.
func (c *C) Ping(_ context.Context, _ *cpb.PingRequest) (*cpb.PingResponse, error) {
	return &cpb.PingResponse{
		Timestamp: timeFn(),
	}, nil
}

// rpcCredentials stores the per-RPC username and password used for authentication.
type rpcCredentials struct {
	*creds.UserPass
}

func (r *rpcCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"username": "admin",
		"password": "admin",
	}, nil
}

func (r *rpcCredentials) RequireTransportSecurity() bool {
	return true
}

// Dial connects to the remote gRPC CNTR server hosted at the address in the request proto.
func (c *C) Dial(ctx context.Context, req *cpb.DialRequest) (*cpb.DialResponse, error) {
	conn, err := grpc.NewClient(req.GetAddr(),
		grpc.WithPerRPCCredentials(&rpcCredentials{}),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true, // NOLINT
		})))
	if err != nil {
		klog.Infof("error dialling target at %s, %v", req.GetAddr(), err)
		return nil, err
	}
	defer conn.Close()

	if req.GetPing() != nil {
		cl := cpb.NewCntrClient(conn)
		pr, err := cl.Ping(ctx, &cpb.PingRequest{})
		if err != nil {
			return nil, err
		}
		return &cpb.DialResponse{
			Response: &cpb.DialResponse_Pong{Pong: pr},
		}, nil
	}

	switch req.GetSrv() {
	case cpb.Service_ST_GNMI:
		cl := gpb.NewGNMIClient(conn)
		cr, err := cl.Capabilities(ctx, &gpb.CapabilityRequest{})
		if err != nil {
			return nil, err
		}
		a, err := anypb.New(cr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "can't marshal %s to any", prototext.Format(cr))
		}
		return &cpb.DialResponse{
			Response: &cpb.DialResponse_GnmiResponse{
				GnmiResponse: a,
			},
		}, nil
	case cpb.Service_ST_GRIBI:
		cl := spb.NewGRIBIClient(conn)
		gr, err := cl.Get(ctx, &spb.GetRequest{
			NetworkInstance: &spb.GetRequest_All{
				All: &spb.Empty{},
			},
			Aft: spb.AFTType_ALL,
		})
		if err != nil {
			return nil, err
		}
		msg, err := gr.Recv()
		if err != nil {
			return nil, err
		}
		a, err := anypb.New(msg)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "can't marshal %s to any", prototext.Format(msg))
		}
		return &cpb.DialResponse{
			Response: &cpb.DialResponse_GribiResponse{
				GribiResponse: a,
			},
		}, nil
	default:
		klog.Warningf("No action was specified in request, dial-only performed, %v", req)
	}

	return &cpb.DialResponse{}, nil
}

// startServer starts a CNTR server listening on the specified port on localhost.
func startServer(port uint) func() {
	tls, err := config.WithSelfTLSCert()
	if err != nil {
		klog.Fatalf("cannot generate self-signed cert, %v", err)
	}

	srv := grpc.NewServer(tls)
	s := &C{}
	cpb.RegisterCntrServer(srv, s)

	lis, err := net.Listen("tcp", fmt.Sprintf("[::]:%d", port))
	if err != nil {
		klog.Exitf("cannot start listening, got err: %v", err)
	}

	klog.Infof("cntr server listening on %s", lis.Addr().String())

	go srv.Serve(lis)
	return srv.Stop
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	stop := startServer(*port)
	defer stop()

	select {}
}
