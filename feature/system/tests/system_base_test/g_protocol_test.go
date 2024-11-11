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

package system_base_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	p4rtpb "github.com/p4lang/p4runtime/go/p4/v1"
)

func dialConn(t *testing.T, dut *ondatra.DUTDevice, svc introspect.Service, wantPort uint32) *grpc.ClientConn {
	t.Helper()
	if svc == introspect.GNOI || svc == introspect.GNSI {
		// Renaming service name due to gnoi and gnsi always residing on same port as gnmi.
		svc = introspect.GNMI
	}
	dialer := introspect.DUTDialer(t, dut, svc)
	if dialer.DevicePort != int(wantPort) {
		t.Fatalf("DUT is not listening on correct port for %q: got %d, want %d", svc, dialer.DevicePort, wantPort)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := dialer.Dial(ctx)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q", dialer.DialTarget)
	}
	return conn
}

// TestGNMIClient validates that the DUT listens on standard gNMI Port.
func TestGNMIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialConn(t, dut, introspect.GNMI, 9339)
	c := gpb.NewGNMIClient(conn)
	if _, err := c.Get(context.Background(), &gpb.GetRequest{Encoding: gpb.Encoding_JSON_IETF, Path: []*gpb.Path{{Elem: []*gpb.PathElem{}}}}); err != nil {
		t.Fatalf("gnmi.Get failed: %v", err)
	}
}

// TestGNOIClient validates that the DUT listens on standard gNOI Port.
func TestGNOIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialConn(t, dut, introspect.GNOI, 9339)
	c := spb.NewSystemClient(conn)
	if _, err := c.Ping(context.Background(), &spb.PingRequest{}); err != nil {
		t.Fatalf("gnoi.system.Ping failed: %v", err)
	}
}

// TestGNSIClient validates that the DUT listens on standard gNSI Port.
func TestGNSIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialConn(t, dut, introspect.GNSI, 9339)
	c := authzpb.NewAuthzClient(conn)
	rsp, err := c.Get(context.Background(), &authzpb.GetRequest{})
	if err != nil {
		statusError, _ := status.FromError(err)
		if statusError.Code() == codes.FailedPrecondition {
			t.Logf("Expected error FAILED_PRECONDITION seen for authz Get Request.")
		} else {
			t.Errorf("Unexpected error during authz Get Request.")
		}
	}
	t.Logf("gNSI authz get response is %s", rsp)
}

// TestGRIBIClient validates that the DUT listens on standard gRIBI Port.
func TestGRIBIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialConn(t, dut, introspect.GRIBI, 9340)
	c := gribipb.NewGRIBIClient(conn)
	if _, err := c.Get(context.Background(), &gribipb.GetRequest{}); err != nil {
		t.Fatalf("gribi.Get failed: %v", err)
	}
}

// TestP4RTClient validates that the DUT listens on standard P4RT Port.
func TestP4RTClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialConn(t, dut, introspect.P4RT, 9559)
	c := p4rtpb.NewP4RuntimeClient(conn)
	if _, err := c.Capabilities(context.Background(), &p4rtpb.CapabilitiesRequest{}); err != nil {
		t.Fatalf("p4rt.Capabilites failed: %v", err)
	}
}
