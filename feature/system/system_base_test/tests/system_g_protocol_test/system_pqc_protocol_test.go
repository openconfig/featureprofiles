/*
 Copyright 2026 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 you may obtain a copy of the License at

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
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	p4rtpb "github.com/p4lang/p4runtime/go/p4/v1"
)

func dialPQCConn(t *testing.T, dut *ondatra.DUTDevice, svc introspect.Service, wantPort uint32, nonStandardPort bool) *grpc.ClientConn {
	t.Helper()
	if svc == introspect.GNOI || svc == introspect.GNSI {
		// Renaming service name due to gnoi and gnsi always residing on same port as gnmi.
		svc = introspect.GNMI
	}
	dialer := introspect.DUTDialer(t, dut, svc)
	t.Logf("Dialing %s on %s (Port: %d) with PQC", svc, dialer.DialTarget, dialer.DevicePort)
	if !nonStandardPort {
		if dialer.DevicePort != int(wantPort) {
			t.Fatalf("DUT is not listening on standard port for %q: got %d, want %d", svc, dialer.DevicePort, wantPort)
		}
	}

	// Custom TLS config for PQC
	// 0x6399 is the code point for X25519Kyber768Draft00
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		CurvePreferences:   []tls.CurveID{0x6399, tls.X25519, tls.CurveP256},
	}

	target := fmt.Sprintf("%s:%d", dialer.DialTarget, dialer.DevicePort)
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		t.Fatalf("grpc.NewClient failed to: %q, error: %v", target, err)
	}
	t.Logf("Successfully dialed %s on %s with PQC", svc, target)
	return conn
}

// TestPQCGNMIClient validates that the DUT listens on standard gNMI Port and supports PQC.
func TestPQCGNMIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialPQCConn(t, dut, introspect.GNMI, 9339, deviations.NonStandardGRPCPort(dut))
	defer conn.Close()
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

// TestPQCGNOIClient validates that the DUT listens on standard gNOI Port and supports PQC.
func TestPQCGNOIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialPQCConn(t, dut, introspect.GNOI, 9339, deviations.NonStandardGRPCPort(dut))
	defer conn.Close()
	c := spb.NewSystemClient(conn)
	if _, err := c.Ping(context.Background(), &spb.PingRequest{}); err != nil {
		t.Fatalf("gnoi.system.Ping failed: %v", err)
	}
}

// TestPQCGNSIClient validates that the DUT listens on standard gNSI Port and supports PQC.
func TestPQCGNSIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialPQCConn(t, dut, introspect.GNSI, 9339, deviations.NonStandardGRPCPort(dut))
	defer conn.Close()
	c := authzpb.NewAuthzClient(conn)
	rsp, err := c.Get(context.Background(), &authzpb.GetRequest{})
	if err != nil {
		statusError, _ := status.FromError(err)
		if statusError.Code() == codes.FailedPrecondition {
			t.Logf("Expected error FAILED_PRECONDITION seen for authz Get Request.")
			return
		}
		t.Fatalf("Unexpected error during authz Get Request: %v", err)
	}
	t.Logf("gNSI authz get response is %s", rsp)
}

// TestPQCGRIBIClient validates that the DUT listens on standard gRIBI Port and supports PQC.
func TestPQCGRIBIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialPQCConn(t, dut, introspect.GRIBI, 9340, deviations.NonStandardGRPCPort(dut))
	defer conn.Close()
	c := gribipb.NewGRIBIClient(conn)
	if _, err := c.Get(context.Background(), &gribipb.GetRequest{}); err != nil {
		t.Fatalf("gribi.Get failed: %v", err)
	}
}

// TestPQCP4RTClient validates that the DUT listens on standard P4RT Port and supports PQC.
func TestPQCP4RTClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	conn := dialPQCConn(t, dut, introspect.P4RT, 9559, deviations.NonStandardGRPCPort(dut))
	defer conn.Close()
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
