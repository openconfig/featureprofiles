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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/prototext"

	cpb "github.com/openconfig/featureprofiles/internal/cntrsrv/proto/cntr"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func DialService(ctx context.Context, t *testing.T, name string, dut *ondatra.DUTDevice) (*grpc.ClientConn, func()) {
	t.Helper()

	var dialer interface {
		DialGRPC(context.Context, string, ...grpc.DialOption) (*grpc.ClientConn, error)
	}
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &dialer); err != nil {
		t.Fatalf("DUT does not support DialGRPC function: %v", err)
	}

	tlsc := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, // NOLINT
	})
	conn, err := dialer.DialGRPC(ctx, name, grpc.WithTransportCredentials(tlsc), grpc.WithBlock())
	if err != nil {
		t.Fatalf("Failed to dial %s, %v", name, err)
	}
	return conn, func() { conn.Close() }
}

// TestDial implements CNTR-2, validating that it is possible for an external caller to dial into a service
// running in a container on a DUT. The service used is the cntr service defined by cntr.proto.
func TestDial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, stop := DialService(ctx, t, "cntr", ondatra.DUT(t, "r0"))
	stop()
}

// TestDialLocal implements CNTR-3, validating that it is possible for a
// container running on the device to connect to local gRPC services that are
// running on the DUT.
func TestDialLocal(t *testing.T) {

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
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			conn, stop := DialService(ctx, t, "cntr", ondatra.DUT(t, "r0"))
			defer stop()

			client := cpb.NewCntrClient(conn)
			got, err := client.Dial(ctx, tt.inMsg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DialContext(): got unexpected error, err: %v, wantErr? %v", err, tt.wantErr)
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

	configureIPv6Addr(ondatra.DUT(t, "r0"), "port1", "fe80::cafe:1")
	configureIPv6Addr(ondatra.DUT(t, "r1"), "port1", "fe80::cafe:2")

	validateIPv6Present := func(dut *ondatra.DUTDevice, name string) {
		// Check that there is a configured IPv6 address on the interface.
		// TODO(robjs): Validate expectations as to whether autoconf link-local is returned
		// here.
		v6addr := gnmi.GetAll(t, dut, gnmi.OC().Interface(dut.Port(t, name).Name()).SubinterfaceAny().Ipv6().AddressAny().State())
		if len(v6addr) < 1 {
			t.Fatalf("%s: did not get a configured IPv6 address, got: %d (%s), want: 1", dut.Name(), len(v6addr), pretty.Sprint(v6addr))
		}
	}

	validateIPv6Present(ondatra.DUT(t, "r0"), "port1")
	validateIPv6Present(ondatra.DUT(t, "r1"), "port1")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, stop := DialService(ctx, t, "cntr", ondatra.DUT(t, "r1"))
	defer stop()

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
		inDUT        *ondatra.DUTDevice
		inRemoteAddr string
	}{{
		inDUT:        ondatra.DUT(t, "r1"),
		inRemoteAddr: "fe80::cafe:1",
	}, {
		inDUT:        ondatra.DUT(t, "r0"),
		inRemoteAddr: "fe80::cafe:2",
	}}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("dial from %s to %s", tt.inDUT, tt.inRemoteAddr), func(t *testing.T) {
			dialAddr := fmt.Sprintf("[%s%%25%s]:60061", tt.inRemoteAddr, containerInterfaceName(t, tt.inDUT, tt.inDUT.Port(t, "port1")))
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
