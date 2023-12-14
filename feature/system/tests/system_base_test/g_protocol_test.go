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
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/knebind/creds"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	tpb "github.com/openconfig/kne/proto/topo"
	p4rtpb "github.com/p4lang/p4runtime/go/p4/v1"
)

func resolveService(t *testing.T, dut *ondatra.DUTDevice, serviceName string, wantPort uint32) string {
	t.Helper()
	var servDUT interface {
		Service(string) (*tpb.Service, error)
	}
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &servDUT); err != nil {
		t.Skipf("DUT does not support Service function: %v", err)
	}
	if serviceName == "gnoi" || serviceName == "gnsi" {
		// Renaming service name due to gnoi and gnsi always residing on same port as gnmi.
		serviceName = "gnmi"
	}
	s, err := servDUT.Service(serviceName)
	if err != nil {
		t.Fatal(err)
	}
	if s.GetInside() != wantPort {
		t.Fatalf("DUT is not listening on correct port for %q: got %d, want %d", serviceName, s.GetInside(), wantPort)
	}
	return fmt.Sprintf("%s:%d", s.GetOutsideIp(), s.GetOutside())

}

type rpcCredentials struct {
	*creds.UserPass
}

func (r *rpcCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": "admin",
		"password": "admin",
	}, nil
}

func (r *rpcCredentials) RequireTransportSecurity() bool {
	return true
}

// TestGNMIClient validates that the DUT listens on standard gNMI Port.
func TestGNMIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	target := resolveService(t, dut, "gnmi", 9339)
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))} // NOLINT
	creds := &rpcCredentials{}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	t.Logf("gNMI standard port test: %q", target)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, target, credOpts...)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q", target)
	}
	c := gpb.NewGNMIClient(conn)
	if _, err := c.Get(context.Background(), &gpb.GetRequest{}); err != nil {
		t.Fatalf("gnmi.Get failed: %v", err)
	}
}

// TestGNOIClient validates that the DUT listens on standard gNMI Port.
func TestGNOIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	target := resolveService(t, dut, "gnoi", 9339)
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))} // NOLINT
	creds := &rpcCredentials{}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))

	t.Logf("gNOI standard port test: %q", target)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, target, credOpts...)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q", target)
	}
	c := spb.NewSystemClient(conn)
	_, err = c.Ping(context.Background(), &spb.PingRequest{})
	if err != nil {
		t.Fatalf("gnoi.system.Time failed: %v", err)
	}
}

// TestGNSIClient validates that the DUT listens on standard gNMI Port.
func TestGNSIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	target := resolveService(t, dut, "gnsi", 9339)
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))} // NOLINT
	creds := &rpcCredentials{}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))

	t.Logf("gNSI standard port test: %q", target)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, target, credOpts...)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q", target)
	}
	c := authzpb.NewAuthzClient(conn)
	_, err = c.Get(context.Background(), &authzpb.GetRequest{})
	if err != nil {
		t.Fatalf("gnsi.authz.Get failed: %v", err)
	}
}

// TestGRIBIClient validates that the DUT listens on standard gNMI Port.
func TestGRIBIClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	target := resolveService(t, dut, "gribi", 9340)
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))} // NOLINT
	creds := &rpcCredentials{}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))

	t.Logf("gRIBI standard port test: %q", target)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, target, credOpts...)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q", target)
	}
	c := gribipb.NewGRIBIClient(conn)
	_, err = c.Get(context.Background(), &gribipb.GetRequest{})
	if err != nil {
		t.Fatalf("gribi.Get failed: %v", err)
	}
}

// TestP4RTClient validates that the DUT listens on standard gNMI Port.
func TestP4RTClient(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	target := resolveService(t, dut, "p4rt", 9559)
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))} // NOLINT
	creds := &rpcCredentials{}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))

	t.Logf("P4RT standard port test: %q", target)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, target, credOpts...)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q", target)
	}
	c := p4rtpb.NewP4RuntimeClient(conn)
	_, err = c.Capabilities(context.Background(), &p4rtpb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("p4rt.Capabilites failed: %v", err)
	}
}
