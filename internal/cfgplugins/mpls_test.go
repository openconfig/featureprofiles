// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgplugins

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/metadata"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/fakebind"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	opb "github.com/openconfig/ondatra/proto"
)

type mplsGNMIClient struct {
	gpb.GNMIClient
	requests []*gpb.SetRequest
}

func (c *mplsGNMIClient) Set(_ context.Context, req *gpb.SetRequest, _ ...grpc.CallOption) (*gpb.SetResponse, error) {
	c.requests = append(c.requests, req)
	return &gpb.SetResponse{}, nil
}

// TestMPLSStaticLSPByPass checks that adding and restoring label bindings never
// replaces the MPLS parent, and exercises both supported next-hop schemas.
func TestMPLSStaticLSPByPass(t *testing.T) {
	for _, oldPath := range []bool{false, true} {
		name := "next-hop-list"
		if oldPath {
			name = "legacy-next-hop"
		}
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			md := `platform_exceptions: { platform: { vendor: CISCO } deviations: { use_old_oc_path_static_lsp_nh: false } }`
			if oldPath {
				md = `platform_exceptions: { platform: { vendor: CISCO } deviations: { use_old_oc_path_static_lsp_nh: true } }`
			}
			if err := os.WriteFile("metadata.textproto", []byte(md), 0600); err != nil {
				t.Fatal(err)
			}
			if err := metadata.Init(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { metadata.Get().Reset() })
			client := &mplsGNMIClient{}
			bind := fakebind.Setup()
			t.Cleanup(func() { bind.WithReservation(nil) })
			bind.WithReservation(&binding.Reservation{DUTs: map[string]binding.DUT{
				"dut": &fakebind.DUT{
					AbstractDUT: &binding.AbstractDUT{Dims: &binding.Dims{Name: "dut", Vendor: opb.Device_CISCO}},
					DialGNMIFn:  func(context.Context, ...grpc.DialOption) (gpb.GNMIClient, error) { return client, nil },
				},
			}})
			dut := ondatra.DUT(t, "dut")
			for _, lspName := range []string{"v4", "v6", "v4"} {
				batch := &gnmi.SetBatch{}
				MPLSStaticLSPByPass(t, batch, dut, lspName, 99991, "192.0.2.26", "ipv4", true)
				batch.Set(t, dut)
				req := client.requests[len(client.requests)-1]
				if len(req.GetReplace()) != 0 || len(req.GetDelete()) != 0 || len(req.GetUpdate()) != 1 {
					t.Fatalf("LSP Set must contain only one update, got %v", req)
				}
				u := req.GetUpdate()[0]
				elems := u.GetPath().GetElem()
				if len(elems) != 6 || elems[5].GetName() != "static-lsp" || elems[5].GetKey()["name"] != lspName {
					t.Fatalf("LSP update must target only %q, got %v", lspName, u.GetPath())
				}
				got := &oc.NetworkInstance_Mpls_Lsps_StaticLsp{}
				if err := oc.Unmarshal(u.GetVal().GetJsonIetfVal(), got, &ytypes.PreferShadowPath{}); err != nil {
					t.Fatal(err)
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("LSP configuration violates the OpenConfig schema: %v", err)
				}
				want := &oc.NetworkInstance_Mpls_Lsps_StaticLsp{Name: ygot.String(lspName)}
				egress := want.GetOrCreateEgress()
				egress.IncomingLabel = oc.UnionUint32(99991)
				if oldPath {
					egress.NextHop = ygot.String("192.0.2.26")
					egress.PushLabel = oc.Egress_PushLabel_IMPLICIT_NULL
				} else {
					nh := egress.GetOrCreateLspNextHop(0)
					nh.IpAddress = ygot.String("192.0.2.26")
					nh.PushLabel = oc.LspNextHop_PushLabel_IMPLICIT_NULL
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("LSP configuration mismatch (-want +got):\n%s", diff)
				}
			}
			RemoveStaticMplsLspPushLabel(t, dut, "v4", "")
			req := client.requests[len(client.requests)-1]
			if len(req.GetDelete()) != 1 || len(req.GetUpdate()) != 0 || len(req.GetReplace()) != 0 {
				t.Fatalf("LSP cleanup must delete only the named entry, got %v", req)
			}
			elems := req.GetDelete()[0].GetElem()
			if len(elems) != 6 || elems[5].GetName() != "static-lsp" || elems[5].GetKey()["name"] != "v4" {
				t.Fatalf("LSP cleanup must target only v4, got %v", req.GetDelete()[0])
			}
		})
	}
}
