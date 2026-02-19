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

// Package cntr_test implements an ONDATRA test for container functionalities
// as described in the CNTR-[234] tests in README.md.
package cntr_test

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/openconfig/featureprofiles/internal/containerztest"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"

	cpb "github.com/openconfig/featureprofiles/internal/cntrsrv/proto/cntr"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	containerTar = flag.String("container_tar", "/tmp/cntrsrv.tar", "The container tarball to deploy.")
	// containerTarPath returns the path to the container tarball.
	// This can be overridden for internal testing behavior using init().
	containerTarPath = func(t *testing.T) string {
		return *containerTar
	}
)

const (
	imageName    = "cntrsrv_image"
	instanceName = "cntr-test-conn"
	cntrPort     = 60061
)

// setupContainer deploys and starts the cntrsrv container on the DUT.
// It returns a function to clean up the container.
func setupContainer(t *testing.T, dut *ondatra.DUTDevice) func() {
	t.Helper()
	ctx := context.Background()
	opts := containerztest.StartContainerOptions{
		ImageName:           imageName,
		InstanceName:        instanceName,
		Command:             fmt.Sprintf("./cntrsrv --port=%d", cntrPort),
		TarPath:             containerTarPath(t),
		Network:             "host",
		PollForRunningState: true,
	}
	_, cleanup := containerztest.Setup(ctx, t, dut, opts)
	return cleanup
}

// dialContainer dials a gRPC service running on a container on a device at the specified port.
func dialContainer(t *testing.T, ctx context.Context, dut *ondatra.DUTDevice, port int) *grpc.ClientConn {
	t.Helper()
	var dialer interface {
		DialGRPCWithPort(context.Context, int, ...grpc.DialOption) (*grpc.ClientConn, error)
	}
	bindingDUT := dut.RawAPIs().BindingDUT()
	if err := binding.DUTAs(bindingDUT, &dialer); err != nil {
		t.Skipf("BindingDUT %T does not implement DialGRPCWithPort, which is required for this test: %v", bindingDUT, err)
	}

	conn, err := dialer.DialGRPCWithPort(ctx, port)
	if err != nil {
		t.Fatalf("DialGRPCWithPort failed: %v", err)
	}
	return conn
}

// TestDial implements CNTR-2, validating that it is possible for an external caller to dial into a service
// running in a container on a DUT. The service used is the cntr service defined by cntr.proto.
func TestDial(t *testing.T) {
	dut := ondatra.DUT(t, "dut1")
	cleanup := setupContainer(t, dut)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn := dialContainer(t, ctx, dut, cntrPort)
	defer conn.Close()

	client := cpb.NewCntrClient(conn)

	// The container can be in a RUNNING state before the gRPC server inside is
	// ready to accept connections. We retry the Ping RPC to handle this race
	// condition.
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Ping failed after timeout, last error: %v", lastErr)
		default:
		}

		_, err := client.Ping(ctx, &cpb.PingRequest{})
		if err == nil {
			t.Log("Successfully pinged cntrsrv.")
			return // Success
		}
		lastErr = err
		t.Logf("Ping failed, retrying in 2 seconds... (error: %v)", err)
		time.Sleep(2 * time.Second)
	}
}

// DUTCredentialer is an interface for getting credentials from a DUT binding.
type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

// TestDialLocal implements CNTR-3, validating that it is possible for a
// container running on the device to connect to local gRPC services that are
// running on the DUT.
func TestDialLocal(t *testing.T) {
	dut := ondatra.DUT(t, "dut1")
	cleanup := setupContainer(t, dut)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn := dialContainer(t, ctx, dut, cntrPort)
	defer conn.Close()
	client := cpb.NewCntrClient(conn)

	var lastErr error
	for {
		if ctx.Err() != nil {
			t.Fatalf("Timed out waiting for container gRPC server to be ready, last error: %v", lastErr)
		}
		_, lastErr = client.Ping(ctx, &cpb.PingRequest{})
		if lastErr == nil {
			t.Log("Successfully pinged cntrsrv.")
			break // Success
		}
		t.Logf("Ping failed, retrying in 2 seconds... (error: %v)", lastErr)
		time.Sleep(2 * time.Second)
	}

	var creds DUTCredentialer
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("Failed to get DUT credentials using binding.DUTAs: %v. The binding for %s must implement the DUTCredentialer interface.", err, dut.Name())
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()

	var dialAddr string
	dialAddr = "[::]"
	if deviations.LocalhostForContainerz(dut) {
		dialAddr = "[fd01::1]"
	}

	// Establish a gRIBI client to program gRIBI entries.
	gribiClient := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}
	if err := gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	gribiClient.BecomeLeader(t)
	gribiClient.FlushAll(t)
	defer gribiClient.Close(t)
	defer gribiClient.FlushAll(t)

	//Program a sample gRIBI Entry on DUT for gRIBI Get query response.
	gribiClient.AddNH(t, 2001, "Decap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 201, map[uint64]uint64{2001: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	tests := []struct {
		desc     string
		inMsg    *cpb.DialRequest
		wantResp bool
		wantErr  bool
	}{{
		desc: "dial gNMI",
		inMsg: &cpb.DialRequest{
			Addr:     dialAddr + ":9339",
			Username: username,
			Password: password,
			Request: &cpb.DialRequest_Srv{
				Srv: cpb.Service_ST_GNMI,
			},
		},
		wantResp: true,
	}, {
		desc: "dial gRIBI",
		inMsg: &cpb.DialRequest{
			Addr:     dialAddr + ":9340",
			Username: username,
			Password: password,
			Request: &cpb.DialRequest_Srv{
				Srv: cpb.Service_ST_GRIBI,
			},
		},
		wantResp: true,
	}, {
		desc: "dial something not listening",
		inMsg: &cpb.DialRequest{
			Addr:     dialAddr + ":4242",
			Username: username,
			Password: password,
			Request: &cpb.DialRequest_Srv{
				Srv: cpb.Service_ST_GRIBI,
			},
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			// Use the client established before the sub-tests.
			got, err := client.Dial(tctx, tt.inMsg)
			// For the gRIBI Get RPC, an EOF error is returned if the server has no entries to
			// stream back. This is the expected behavior for a successful connection to a
			// gRIBI service on a device with an empty RIB, so we treat it as a non-error.
			if tt.inMsg.GetSrv() == cpb.Service_ST_GRIBI && err != nil && strings.Contains(err.Error(), "EOF") {
				err = nil
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("Dial(): got unexpected error, err: %v, wantErr? %v", err, tt.wantErr)
			}

			t.Logf("got response: %s", prototext.Format(got))
			if (got != nil) != tt.wantResp {
				t.Fatalf("Dial: did not get correct response, got: %s, wantResponse? %v", prototext.Format(got), tt.wantResp)
			}
		})
	}
}

// TestConnectRemote implements CNTR-4, validating that it is possible for a container to connect to a container
// on an adjacent node via gRPC using IPv6 link local addresses. r0 and r1 in the topology are configured with
// IPv6 link-local addresses via gNMI, and the CNTR service is used to trigger a connection between the two addresses.
//
// The test is repeated for r0 --> r1 and r1 --> r0.
func TestConnectRemote(t *testing.T) {
	t.Skipf("TODO(abhinavkmr): Testing pending on device. Skipping for now!")
	configureIPv6Addr := func(dut *ondatra.DUTDevice, name, addr string) {
		t.Helper()
		pn := dut.Port(t, name).Name()

		d := &oc.Interface{
			Name:    ygot.String(pn),
			Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled: ygot.Bool(true),
		}
		s := d.GetOrCreateSubinterface(0)
		s.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		v6 := s.GetOrCreateIpv6()
		// TODO(robjs): Clarify whether IPv4 enabled is required here for multiple
		// targets, otherwise add a deviation.
		v6.Enabled = ygot.Bool(true)
		a := v6.GetOrCreateAddress(addr)
		a.PrefixLength = ygot.Uint8(64)
		a.Type = oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST
		gnmi.Replace(t, dut, gnmi.OC().Interface(pn).Config(), d)

		time.Sleep(1 * time.Second)
	}

	r0 := ondatra.DUT(t, "dut1")
	r1 := ondatra.DUT(t, "dut2")

	cleanup0 := setupContainer(t, r0)
	defer cleanup0()
	cleanup1 := setupContainer(t, r1)
	defer cleanup1()

	configureIPv6Addr(r0, "port1", "fe80::cafe:1")
	configureIPv6Addr(r1, "port2", "fe80::cafe:2")

	validateIPv6Present := func(dut *ondatra.DUTDevice, name string) {
		// Check that there is a configured IPv6 address on the interface.
		t.Helper()
		// TODO(robjs): Validate expectations as to whether autoconf link-local is returned
		// here.
		v6addr := gnmi.GetAll(t, dut, gnmi.OC().Interface(dut.Port(t, name).Name()).SubinterfaceAny().Ipv6().AddressAny().State())
		if len(v6addr) < 1 {
			t.Fatalf("%s: did not get a configured IPv6 address, got: %d (%s), want: 1", dut.Name(), len(v6addr), pretty.Sprint(v6addr))
		}
	}

	validateIPv6Present(r0, "port1")
	validateIPv6Present(r1, "port2")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerInterfaceName := func(t *testing.T, d *ondatra.DUTDevice, port *ondatra.Port) string {
		portName := port.Name()
		if parts := strings.Split(port.Name(), ":"); len(parts) == 2 {
			portName = parts[1]
		}
		switch d.Vendor() {
		case ondatra.ARISTA:
			switch {
			case strings.HasPrefix(portName, "Ethernet"):
				num, _ := strings.CutPrefix(portName, "Ethernet")
				return fmt.Sprintf("eth%s", num)
			}
		}
		t.Fatalf("cannot resolve interface name into Linux interface name, %s -> %s", d.Vendor(), portName)
		return ""
	}

	tests := []struct {
		desc         string
		inRemoteAddr string
		inDialer     *ondatra.DUTDevice
		dialerPort   string
	}{{
		desc:         "r1->r0",
		inRemoteAddr: "fe80::cafe:1",
		inDialer:     r1,
		dialerPort:   "port2",
	}, {
		desc:         "r0->r1",
		inRemoteAddr: "fe80::cafe:2",
		inDialer:     r0,
		dialerPort:   "port1",
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			conn := dialContainer(t, ctx, tt.inDialer, cntrPort)
			dialAddr := fmt.Sprintf("[%s%%25%s]:%d", tt.inRemoteAddr, containerInterfaceName(t, tt.inDialer, tt.inDialer.Port(t, tt.dialerPort)), cntrPort)
			t.Logf("dialing remote address %s", dialAddr)

			client := cpb.NewCntrClient(conn)
			got, err := client.Dial(ctx, &cpb.DialRequest{
				Addr: dialAddr,
				Request: &cpb.DialRequest_Ping{
					Ping: &cpb.PingRequest{},
				},
			})
			if err != nil {
				t.Fatalf("could not make request to remote device, got err: %v", err)
			}
			t.Logf("got response, %s", prototext.Format(got))
		})
	}
}
