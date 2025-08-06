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
	"crypto/tls"
	"flag"
	"fmt"
	"strconv"
	"net"

	"strings"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/openconfig/containerz/client"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnoic "github.com/openconfig/gnoi/containerz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"

	cpb "github.com/openconfig/featureprofiles/internal/cntrsrv/proto/cntr"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	containerTar   = flag.String("container_tar", "/tmp/cntrsrv.tar", "The container tarball to deploy.")
)

const (
	imageName    = "cntrsrv"
	instanceName = "cntr-test-conn"
	cntrPort     = 60061
)

// waitForContainerRunning polls until a container instance is in the RUNNING state.
func waitForContainerRunning(ctx context.Context, t *testing.T, cli *client.Client, targetName, instanceName string, timeout time.Duration) error {
	t.Helper()
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		listContCh, err := cli.ListContainer(pollCtx, true, 0, map[string][]string{"name": {instanceName}})
		if err != nil {
			return fmt.Errorf("unable to list container %s on %s during polling: %w", instanceName, targetName, err)
		}

		var containerIsRunning bool
		for info := range listContCh {
			if info.Error != nil {
				return fmt.Errorf("error message received while listing container %s on %s during polling: %w", instanceName, targetName, info.Error)
			}
			if (info.Name == instanceName || info.Name == "/"+instanceName) && info.State == gnoic.ListContainerResponse_RUNNING.String() {
				t.Logf("Container %s on %s confirmed RUNNING.", instanceName, targetName)
				containerIsRunning = true
				break
			}
		}

		if containerIsRunning {
			return nil
		}

		if pollCtx.Err() != nil {
			return fmt.Errorf("timed out waiting for container %s on %s to be RUNNING", instanceName, targetName)
		}
		time.Sleep(5 * time.Second)
	}
}

// setupContainer deploys and starts the cntrsrv container on the DUT.
// It returns a function to clean up the container.
func setupContainer(t *testing.T, dut *ondatra.DUTDevice) func() {
	t.Helper()
	ctx := context.Background()

	cli := client.NewClientFromStub(dut.RawAPIs().GNOI(t).Containerz())

	// 1. Remove existing container instance to ensure a clean start.
	t.Logf("Attempting to remove existing container instance %s on %s before start.", instanceName, dut.Name())
	if err := cli.RemoveContainer(ctx, instanceName, true); err != nil {
		if status.Code(err) != codes.NotFound {
			t.Logf("Pre-start removal of container %s on %s failed: %v", instanceName, dut.Name(), err)
		}
	}

	// 2. Push the image.
	t.Logf("Pushing image %s:latest from %s to %s.", imageName, *containerTar, dut.Name())
	progCh, err := cli.PushImage(ctx, imageName, "latest", *containerTar, false)
	if err != nil {
		t.Fatalf("Initial call to PushImage for %s:latest on %s failed: %v", imageName, dut.Name(), err)
	}
	for prog := range progCh {
		if prog.Error != nil {
			t.Fatalf("Error during push of image %s:latest on %s: %v", imageName, dut.Name(), prog.Error)
		}
		if prog.Finished {
			t.Logf("Successfully pushed image %s:%s to %s.", prog.Image, prog.Tag, dut.Name())
		}
	}

	// 3. Start the container.
	startCmd := fmt.Sprintf("./cntrsrv --port=%d", cntrPort)
	t.Logf("Starting container %s on %s with image %s:latest, command '%s', and host networking.", instanceName, dut.Name(), imageName, startCmd)

	if _, err := cli.StartContainer(ctx, imageName, "latest", startCmd, instanceName, client.WithNetwork("host")); err != nil {
		t.Fatalf("Unable to start container %s on %s: %v", instanceName, dut.Name(), err)
	}
	t.Logf("StartContainer called for %s on %s", instanceName, dut.Name())

	// 4. Wait for container to be running.
	t.Logf("Polling for container %s on %s to reach RUNNING state.", instanceName, dut.Name())
	if err := waitForContainerRunning(ctx, t, cli, dut.Name(), instanceName, 30*time.Second); err != nil {
		t.Fatalf("Container %s on %s did not reach RUNNING state: %v", instanceName, dut.Name(), err)
	}

	return func() {
		t.Logf("Cleaning up container %s on %s", instanceName, dut.Name())
		if err := cli.StopContainer(ctx, instanceName, true); err != nil {
			if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
				t.Errorf("Failed to stop container %s on %s: %v", instanceName, dut.Name(), err)
			}
		}
	}
}

// dialContainer dials a gRPC service running on a container on a device at the specified port.
// It works for both KNE and static bindings by introspecting the connection details.
func dialContainer(t *testing.T, dut *ondatra.DUTDevice, port int) *grpc.ClientConn {
	t.Helper()

	var introspector introspect.Introspector
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &introspector); err != nil {
		t.Fatalf("DUT does not support introspector interface: %v", err)
	}
	// Use gNMI to get a dialer, as it's a common service that should be present.
	// We use it to discover the hostname/IP of the DUT.
	dialer, err := introspector.Dialer(introspect.GNMI)
	if err != nil {
		t.Fatalf("could not get dialer for gNMI: %v", err)
	}

	host, _, err := net.SplitHostPort(dialer.DialTarget)
	if err != nil {
		// If SplitHostPort fails, it might be because there is no port.
		// This can be the case for physical devices where target is just the hostname/IP.
		host = dialer.DialTarget
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	t.Logf("Dialing gRPC address: %s", addr)

	tlsc := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, // NOLINT
	})
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(tlsc))
	if err != nil {
		t.Fatalf("Failed to dial %s: %v", addr, err)
	}
	return conn
}



// TestDial implements CNTR-2, validating that it is possible for an external caller to dial into a service
// running in a container on a DUT. The service used is the cntr service defined by cntr.proto.
func TestDial(t *testing.T) {
	dut := ondatra.DUT(t, "r0")
	cleanup := setupContainer(t, dut)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn := dialContainer(t, dut, cntrPort)

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

		_, err := client.Ping(context.Background(), &cpb.PingRequest{})
		if err == nil {
			t.Log("Successfully pinged cntrsrv.")
			return // Success
		}
		lastErr = err
		t.Logf("Ping failed, retrying in 2 seconds... (error: %v)", err)
		time.Sleep(2 * time.Second)
	}
}

// TestDialLocal implements CNTR-3, validating that it is possible for a
// container running on the device to connect to local gRPC services that are
// running on the DUT.
func TestDialLocal(t *testing.T) {
	dut := ondatra.DUT(t, "r0")
	cleanup := setupContainer(t, dut)
	defer cleanup()

	conn := dialContainer(t, dut, cntrPort)
	defer conn.Close()
	client := cpb.NewCntrClient(conn)

	// Wait for the container's gRPC server to be ready before running sub-tests.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var lastErr error
	for {
		if ctx.Err() != nil {
			t.Fatalf("Timed out waiting for container gRPC server to be ready, last error: %v", lastErr)
		}
		_, lastErr = client.Ping(context.Background(), &cpb.PingRequest{})
		if lastErr == nil {
			break // Success
		}
		time.Sleep(2 * time.Second)
	}

	tests := []struct {
		desc     string
		inMsg    *cpb.DialRequest
		wantResp bool
		wantErr  bool
	}{{
		desc: "dial gNMI",
		inMsg: &cpb.DialRequest{
			Addr: "localhost:9339",
			Request: &cpb.DialRequest_Srv{
				Srv: cpb.Service_ST_GNMI,
			},
		},
		wantResp: true,
	}, {
		desc: "dial gRIBI",
		inMsg: &cpb.DialRequest{
			Addr: "localhost:9340",
			Request: &cpb.DialRequest_Srv{
				Srv: cpb.Service_ST_GRIBI,
			},
		},
		wantResp: true,
	}, {
		desc: "dial something not listening",
		inMsg: &cpb.DialRequest{
			Addr: "localhost:4242",
			Request: &cpb.DialRequest_Srv{
				Srv: cpb.Service_ST_GRIBI,
			},
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// Use the client established before the sub-tests.
			got, err := client.Dial(ctx, tt.inMsg)
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
	configureIPv6Addr := func(dut *ondatra.DUTDevice, name, addr string) {
		t.Helper()
		pn := dut.Port(t, "port1").Name()

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

	r0 := ondatra.DUT(t, "r0")
	r1 := ondatra.DUT(t, "r1")

	cleanup0 := setupContainer(t, r0)
	defer cleanup0()
	cleanup1 := setupContainer(t, r1)
	defer cleanup1()

	configureIPv6Addr(r0, "port1", "fe80::cafe:1")
	configureIPv6Addr(r1, "port1", "fe80::cafe:2")

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
	validateIPv6Present(r1, "port1")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerInterfaceName := func(t *testing.T, d *ondatra.DUTDevice, port *ondatra.Port) string {
		switch d.Vendor() {
		case ondatra.ARISTA:
			switch {
			case strings.HasPrefix(port.Name(), "Ethernet"):
				num, _ := strings.CutPrefix(port.Name(), "Ethernet")
				return fmt.Sprintf("eth%s", num)
			}
		}
		t.Fatalf("cannot resolve interface name into Linux interface name, %s -> %s", d.Vendor(), port.Name())
		return ""
	}

	tests := []struct {
		desc         string
		inRemoteAddr string
		inDialer     *ondatra.DUTDevice
	}{{
		desc:         "r1->r0",
		inRemoteAddr: "fe80::cafe:1",
		inDialer:     r1,
	}, {
		desc:         "r0->r1",
		inRemoteAddr: "fe80::cafe:2",
		inDialer:     r0,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			conn := dialContainer(t, tt.inDialer, cntrPort)
			dialAddr := fmt.Sprintf("[%s%%25%s]:%d", tt.inRemoteAddr, containerInterfaceName(t, tt.inDialer, tt.inDialer.Port(t, "port1")), cntrPort)
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
