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

package system_g_protocol_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
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

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func dialConn(t *testing.T, dut *ondatra.DUTDevice, svc introspect.Service, wantPort uint32, nonStandardPort bool) *grpc.ClientConn {
	t.Helper()
	if svc == introspect.GNOI || svc == introspect.GNSI {
		// Renaming service name due to gnoi and gnsi always residing on same port as gnmi.
		svc = introspect.GNMI
	}
	dialer := introspect.DUTDialer(t, dut, svc)
	t.Logf("Dialing %s on %s (Port: %d)", svc, dialer.DialTarget, dialer.DevicePort)
	if !nonStandardPort {
		if dialer.DevicePort != int(wantPort) {
			t.Fatalf("DUT is not listening on standard port for %q: got %d, want %d", svc, dialer.DevicePort, wantPort)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := dialer.Dial(ctx)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q, port: %d, error: %v", dialer.DialTarget, dialer.DevicePort, err)
	}
	t.Logf("Successfully dialed %s on %s (Port: %d)", svc, dialer.DialTarget, dialer.DevicePort)
	return conn
}

// TestGNMIClient validates that the DUT listens on standard gNMI Port.
func TestGNMIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var conn *grpc.ClientConn
	if deviations.NonStandardGRPCPort(dut) {
		conn = dialConn(t, dut, introspect.GNMI, 9339, true)
	} else {
		conn = dialConn(t, dut, introspect.GNMI, 9339, false)
	}
	c := gpb.NewGNMIClient(conn)

	var req *gpb.GetRequest
	if deviations.GNMIGetOnRootUnsupported(dut) {
		req = &gpb.GetRequest{
			Path: []*gpb.Path{{
				Elem: []*gpb.PathElem{}}},
			Type:     gpb.GetRequest_CONFIG,
			Encoding: gpb.Encoding_JSON_IETF,
		}

	} else {
		req = &gpb.GetRequest{Encoding: gpb.Encoding_JSON_IETF, Path: []*gpb.Path{{Elem: []*gpb.PathElem{}}}}
	}

	if _, err := c.Get(context.Background(), req); err != nil {
		t.Fatalf("gnmi.Get failed: %v", err)
	}
}

// TestGNOIClient validates that the DUT listens on standard gNOI Port.
func TestGNOIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var conn *grpc.ClientConn
	if deviations.NonStandardGRPCPort(dut) {
		conn = dialConn(t, dut, introspect.GNOI, 9339, true)
	} else {
		conn = dialConn(t, dut, introspect.GNOI, 9339, false)
	}
	c := spb.NewSystemClient(conn)
	if _, err := c.Ping(context.Background(), &spb.PingRequest{}); err != nil {
		t.Fatalf("gnoi.system.Ping failed: %v", err)
	}
}

// TestGNSIClient validates that the DUT listens on standard gNSI Port.
func TestGNSIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var conn *grpc.ClientConn
	if deviations.NonStandardGRPCPort(dut) {
		conn = dialConn(t, dut, introspect.GNSI, 9339, true)
	} else {
		conn = dialConn(t, dut, introspect.GNSI, 9339, false)
	}
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
	conn := dialConn(t, dut, introspect.GRIBI, 9340, false)
	c := gribipb.NewGRIBIClient(conn)
	if _, err := c.Get(context.Background(), &gribipb.GetRequest{}); err != nil {
		t.Fatalf("gribi.Get failed: %v", err)
	}
}

// TestP4RTClient validates that the DUT listens on standard P4RT Port.
func TestP4RTClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialConn(t, dut, introspect.P4RT, 9559, false)
	c := p4rtpb.NewP4RuntimeClient(conn)
	if deviations.P4RTCapabilitiesUnsupported(dut) {
		if _, err := c.Read(context.Background(), &p4rtpb.ReadRequest{
			DeviceId: 1,
			Entities: []*p4rtpb.Entity{
				{
					Entity: &p4rtpb.Entity_TableEntry{},
				},
			},
		}); err != nil {
			t.Fatalf("p4rt.Read failed: %v", err)
		}
	} else {
		if _, err := c.Capabilities(context.Background(), &p4rtpb.CapabilitiesRequest{}); err != nil {
			t.Fatalf("p4rt.Capabilites failed: %v", err)
		}
	}
}
