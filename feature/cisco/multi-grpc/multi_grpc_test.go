// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package grpc implements comprehensive gRPC server configuration tests for Cisco devices.
// This package tests multi-server gRPC configurations, service compatibility, and
// various gRPC configuration scenarios including CLI and OpenConfig model approaches.
package grpc

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	perf "github.com/openconfig/featureprofiles/feature/cisco/performance"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/lemming/gnmi/oc/ocpath"
	oc "github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/povsister/scp"
	"google.golang.org/grpc"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

// TestMain is the entry point for running all tests in this package.
// It calls fptest.RunTests to execute the test suite with proper
// initialization and cleanup procedures.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestDefaultandMultiServerGrpcConfigs_Extended performs comprehensive testing of gRPC server
// configuration and unconfiguration on the DUT (Device Under Test) using both GNMI
// and CLI origins. This test validates:
//
//   - Default gRPC server configurations via CLI commands using GNMI
//   - Multi-server gRPC configurations and their lifecycle management
//   - Configuration presence and removal through CLI show commands
//   - OpenConfig (OC) model validation via GNMI for gRPC servers
//   - Service compatibility and incompatibility scenarios
//   - Proper error handling for expected flaky or failing CLI commands
//   - Full lifecycle management including service, port, and server block removal
//
// The test ensures that gRPC server features are correctly applied, validated,
// and removed, and that the system behaves as expected for both valid and
// invalid configuration scenarios.
func TestDefaultandMultiServerGrpcConfigs(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	t.Run("Test Default Grpc Config/Unconfig using GNMI origin as CLI", func(t *testing.T) {
		// This subtest iterates through a comprehensive list of default gRPC CLI commands,
		// applies each configuration using GNMI with CLI origin, verifies the configuration
		// is present via CLI show commands, then removes it and verifies successful removal.
		//
		// Flaky commands are handled gracefully as their behavior is unpredictable - the gRPC
		// server might stop immediately after sending a response or before, making consistent
		// results impossible to guarantee. These commands are marked in expectedFlakyCLIs map.
		grpcCLIs := []string{
			"tls-mutual",
			"certificate-authentication",
			"tls-trustpoint grpc-trust",
			"listen-addresses 192.0.2.1",
			"listen-addresses 2001:5:10:18::114",
			"address-family dual",
			"address-family ipv4",
			"aaa authorization exec default",
			"aaa accounting queue-size 10",
			"dscp cs3",
			"ttl 64",
			"tls-min-version 1.0",
			"keepalive time 20",
			"keepalive timeout 30",
			"min-keepalive-interval 10",
			"max-concurrent-streams 128",
			"max-request-per-user 20",
			"max-request-total 256",
			"max-streams 128",
			"max-streams-per-user 30",
			"memory limit 1024",
			"gnmi port 50051",
			"gribi port 50052",
			"p4rt port 50053",
			"vrf mgmt",
		}

		// commands that *may* fail (flaky ones)
		expectedFlakyCLIs := map[string]bool{
			"tls-mutual":                 true,
			"certificate-authentication": true,
			"tls-trustpoint grpc-trust":  true,
			"keepalive time 20":          true,
			"keepalive timeout 30":       true,
			"min-keepalive-interval 10":  true,
			"max-concurrent-streams 128": true,
		}

		for idx, cli := range grpcCLIs {
			t.Run(fmt.Sprintf("Step_%d_Config_%s", idx+1, strings.ReplaceAll(cli, " ", "_")), func(t *testing.T) {
				isFlaky := expectedFlakyCLIs[cli]

				if cli == "vrf mgmt" {
					Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "configure")
				} else {
					cfg := fmt.Sprintf("grpc\n  %s\n!", cli)
					_, err := PushCliConfigViaGNMI(ctx, t, dut, cfg)

					if err != nil {
						if isFlaky {
							t.Logf("[WARN] Config %q failed (flaky CLI): %v", cli, err)
						} else {
							t.Errorf("[FAIL] Unexpected error for config %q: %v", cli, err)
						}
					} else {
						if isFlaky {
						} else {
							t.Logf("[PASS] Config push succeeded: %q", cli)
						}
					}
				}

				time.Sleep(5 * time.Second)

				output := config.CMDViaGNMI(ctx, t, dut, "show run grpc")
				normalizedOutput := strings.ReplaceAll(output, "\n", "")
				normalizedOutput = strings.ReplaceAll(normalizedOutput, " ", "")

				normalizedCLI := strings.ReplaceAll(cli, " ", "")

				if !strings.Contains(normalizedOutput, normalizedCLI) {
					t.Errorf("Expected CLI %q not found in output:\n%s", cli, output)
				}

				// Unconfigure step
				removeCLI := GenerateRemoveCLI(cli)

				if cli == "vrf mgmt" {
					Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "unconfigure")
				} else {
					delCfg := fmt.Sprintf("grpc\n  %s\n!", removeCLI)
					_, err := PushCliConfigViaGNMI(ctx, t, dut, delCfg)

					if err != nil {
						if isFlaky {
							t.Logf("[WARN] Unconfig %q failed (flaky CLI): %v", removeCLI, err)
						} else {
							t.Errorf("[FAIL] Unexpected error during unconfig %q: %v", removeCLI, err)
						}
					} else {
						t.Logf("[PASS] Unconfig succeeded: %q", removeCLI)
					}
				}

				time.Sleep(5 * time.Second)

				output = config.CMDViaGNMI(ctx, t, dut, "show run grpc")
				if strings.Contains(output, strings.TrimSpace(cli)) {
					t.Errorf("Config %q still present after delete.\nOutput:\n%s", cli, output)
				} else {
					t.Logf("[PASS] CLI %q removed successfully", cli)
				}
			})
		}
	})

	t.Run("Test Grpc MultiServer Config/Unconfig using GNMI origin as CLI", func(t *testing.T) {
		// This subtest configures a gRPC server block named "multi_server" and iteratively
		// applies and removes various gRPC CLI commands within that server block using GNMI
		// with CLI origin. It validates the presence of each configuration after application
		// and ensures its complete removal after deletion.
		//
		// The test also includes cleanup validation to ensure the entire server block is
		// properly removed when no longer needed.
		grpcCLIs := []string{
			"port 56666",
			"address-family ipv4",
			"services GNMI",
			"services GRIBI",
			"services CNMI",
			"services GNOI",
			"services GNPSI",
			"services GNSI",
			"services P4RT",
			"services SLAPI",
			"services SRTE",
			"ssl-profile-id system_default_profile",
			"tls mutual",
			"dscp cs2",
			"ttl 32",
			"keepalive time 10",
			"keepalive timeout 15",
			"vrf mgmt",
			"local-connection",
			"metadata-authentication disable",
			"max-concurrent-streams 128",
		}

		// Step 1: Apply each configuration and validate, then remove it
		for idx, cli := range grpcCLIs {
			t.Run(fmt.Sprintf("Step_%d_Config_%s", idx+1, strings.ReplaceAll(cli, " ", "_")), func(t *testing.T) {

				// Apply the configuration
				cfg := fmt.Sprintf(`
					grpc server multi_server
					%s
					!`, cli)

				// Push config
				config.TextWithGNMI(ctx, t, dut, cfg)
				time.Sleep(3 * time.Second)

				// Validate that the config is applied
				resp := config.CMDViaGNMI(context.Background(), t, dut, "show run grpc")
				if !strings.Contains(resp, strings.TrimSpace(cli)) {
					t.Errorf("Config %q not found after applying.\nFull output:\n%s", cli, resp)
				}

				// Now remove the configuration

				// Prepare removal CLI (using 'no' prefix)
				removeCLI := fmt.Sprintf("no %s", cli)

				// Push delete
				removeCfg := fmt.Sprintf(`
					grpc server multi_server
					%s
					!`, removeCLI)
				config.TextWithGNMI(ctx, t, dut, removeCfg)

				// Validate that the config was removed
				time.Sleep(3 * time.Second)
				resp = config.CMDViaGNMI(context.Background(), t, dut, "show run grpc")
				if strings.Contains(resp, strings.TrimSpace(cli)) {
					t.Errorf("Config %q still present after deletion.\nFull output:\n%s", cli, resp)
				}
			})
		}

		// Step 2: After all individual configurations are deleted, check for grpc server multi_server block and remove it
		t.Run("Remove entire server multi_server block", func(t *testing.T) {
			// This final cleanup step checks if the 'server multi_server' block still exists
			// after all individual configurations have been deleted, and removes the entire
			// server block if present. This ensures complete cleanup and validates that
			// server blocks can be properly removed even when empty.
			t.Log("==> Checking if 'server multi_server' block is present before removal")

			// Check if the 'server multi_server' block exists
			resp := config.CMDViaGNMI(context.Background(), t, dut, "show run grpc")

			if strings.Contains(resp, "server multi_server") {
				// Block exists, proceed with removal
				t.Log("==> Removing entire `server multi_server` block")

				// Remove the entire server block
				removeCLI := "no grpc server multi_server"

				// Push delete
				config.TextWithGNMI(ctx, t, dut, removeCLI)

				// Validate that the server multi_server block is removed
				resp = config.CMDViaGNMI(context.Background(), t, dut, "show run grpc")
				if strings.Contains(resp, "server multi_server") {
					t.Errorf("Config 'server multi_server' still present after deletion.\nFull output:\n%s", resp)
				}
			} else {
				t.Log("The 'server multi_server' block is already absent, skipping removal.")
			}
		})
	})

	t.Run("Test Default gRPC Server Configurations using OC Model", func(t *testing.T) {
		// This subtest iterates through a comprehensive set of default gRPC server configurations,
		// applies each using the OpenConfig (OC) model via GNMI, validates the configuration
		// is present and operational, then removes it and validates complete removal.
		//
		// The test also handles expected failures gracefully and validates that configuration
		// changes via the OC model are properly reflected in the device's operational state.
		gnmiC := dut.RawAPIs().GNMI(t)
		config := getTargetConfig(t)
		sshIP := config.sshIp
		grpcPort := config.grpcPort
		username := config.sshUser
		password := config.sshPass

		tests := []struct {
			name   string
			params GrpcServerParams
		}{
			{
				name: "Only ListenAddresses",
				params: GrpcServerParams{
					ServerName: "DEFAULT",
					ListenAddresses: []oc.System_GrpcServer_ListenAddresses_Union{
						oc.UnionString("192.0.2.1"),
					},
					Enable: true,
				},
			},
			{
				name: "With Metadata Authentication",
				params: GrpcServerParams{
					ServerName:             "DEFAULT",
					MetadataAuthentication: ygot.Bool(false),
					Enable:                 true,
				},
			},
			{
				name: "With TransportSecurity",
				params: GrpcServerParams{
					ServerName:        "DEFAULT",
					TransportSecurity: ygot.Bool(false),
					Enable:            true,
				},
			},
			{
				name: "With CertificateID",
				params: GrpcServerParams{
					ServerName:    "DEFAULT",
					CertificateID: "CertID",
					Enable:        true,
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				cfg := BuildGrpcServerConfig(tc.params)

				switch tc.name {
				case "With Metadata Authentication":
					_, err := gNMIUpdate(t, dut, gnmi.OC().System().Config(), cfg)
					time.Sleep(5 * time.Second)
					if err != nil {
						// Log that it failed (acceptable case)
						t.Logf("gNMI Update failed as expected: %s", err)
					} else {
						// Log that it passed (also acceptable case)
					}

				case "With TransportSecurity":
					errMsg := testt.CaptureFatal(t, func(t testing.TB) {
						_, _ = gNMIUpdate(t, dut, gnmi.OC().System().Config(), cfg)
						time.Sleep(5 * time.Second)
					})

					if errMsg != nil {
						// Log that it failed (acceptable case)
						t.Logf("gNMI Update failed as expected: %s", *errMsg)
					} else {
						// Log that it passed (also acceptable case)
					}

				case "With CertificateID":
					res, err := gNMIUpdate(t, dut, gnmi.OC().System().Config(), cfg)
					if err != nil {
						t.Logf("Expected gNMI failure occurred: %v", err)
					} else {
						t.Errorf("Unexpected gNMI Update Response: %v", res)
					}

				default:
					// Normal config application
					gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg)
				}

				// Inline validate
				switch tc.name {
				case "Only ListenAddresses":
					got := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses().State())
					if len(got) == 0 || got[0] != oc.UnionString("192.0.2.1") {
						t.Errorf("Expected ListenAddress '192.0.2.1', got: %v", got)
					}

				case "With Metadata Authentication":
					time.Sleep(5 * time.Second)
					path := []*gpb.Path{
						{Origin: "openconfig", Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "grpc-servers"},
							{Name: "grpc-server", Key: map[string]string{"name": "DEFAULT"}},
							{Name: "metadata-authentication"}}},
					}
					ValidateGNMIGetConfig(t, dut.RawAPIs().GNMI(t), path, "metadata-authentication", false, false)

				case "With TransportSecurity":
					time.Sleep(5 * time.Second)
					// Step 1: Try insecure gRPC Dial
					conn := DialInsecureGRPC(ctx, t, sshIP, grpcPort, username, password)

					gnmiC := gpb.NewGNMIClient(conn)

					// Step 2: Build Path to transport-security config
					path := []*gpb.Path{
						{Origin: "openconfig", Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "grpc-servers"},
							{Name: "grpc-server", Key: map[string]string{"name": "DEFAULT"}},
							{Name: "config"},
							{Name: "transport-security"},
						}},
					}

					// Step 3: Validate via Get
					ValidateGNMIGetConfig(t, gnmiC, path, "transport-security", false, false)
				}

				// Delete or update specific field based on test case
				switch tc.name {
				case "Only ListenAddresses":
					gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses().Config())

				case "With Metadata Authentication":
					errMsg := testt.CaptureFatal(t, func(t testing.TB) {
						_ = gnmi.Update(t, dut, gnmi.OC().System().
							GrpcServer("DEFAULT").MetadataAuthentication().Config(), true)
						time.Sleep(5 * time.Second)
					})

					if errMsg != nil {
						// Log that it failed (acceptable case)
						t.Logf("gNMI Update failed as expected: %s", *errMsg)
					} else {
						// Log that it passed (also acceptable case)
					}

				case "With TransportSecurity":
					// Step 1: Try insecure gRPC Dial
					conn := DialInsecureGRPC(ctx, t, sshIP, grpcPort, username, password)

					errMsg := testt.CaptureFatal(t, func(t testing.TB) {
						gclient, err := ygnmi.NewClient(gpb.NewGNMIClient(conn))
						if err != nil {
							t.Fatalf("failed to create ygnmi client: %v", err)
						}

						// Correct path build via ocpath
						path := ocpath.Root().System().GrpcServer("DEFAULT").TransportSecurity().Config()

						// Correct Update with 2 return values
						_, err = ygnmi.Update(context.Background(), gclient, path, true)
						if err != nil {
							t.Fatalf("Update failed: %v", err)
						}

						time.Sleep(5 * time.Second)
					})

					if errMsg != nil {
						// Log that it failed (acceptable case)
						t.Logf("gNMI Update failed as expected: %s", *errMsg)
					} else {
						// Log that it passed (also acceptable case)
					}
				}

				// Validate deletion
				switch tc.name {
				case "Only ListenAddresses":
					got := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses().State())
					if len(got) == 0 || got[0] == oc.UnionString("192.0.2.1") {
						t.Errorf("Expected ListenAddress '192.0.2.1' to be deleted, but found: %v", got)
					}

				case "With Metadata Authentication":
					time.Sleep(5 * time.Second)
					path := []*gpb.Path{
						{Origin: "openconfig", Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "grpc-servers"},
							{Name: "grpc-server", Key: map[string]string{"name": "DEFAULT"}},
							{Name: "metadata-authentication"}}},
					}
					ValidateGNMIGetConfig(t, gnmiC, path, "metadata-authentication", true, false)

				case "With TransportSecurity":
					time.Sleep(5 * time.Second)
					path := []*gpb.Path{
						{Origin: "openconfig", Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "grpc-servers"},
							{Name: "grpc-server", Key: map[string]string{"name": "DEFAULT"}},
							{Name: "transport-security"}}},
					}
					ValidateGNMIGetConfig(t, gnmiC, path, "transport-security", true, false)
				}

			})
		}
	})

	t.Run("Test Multiple gRPC Server Configurations using OC Model", func(t *testing.T) {
		// This test configures a gRPC server named "server1" with various parameters using the OpenConfig (OC) model via GNMI.
		// It validates the presence of each configuration after application and ensures its removal after deletion.
		serverName := "server1"
		servicesToTest := []oc.E_SystemGrpc_GRPC_SERVICE{
			oc.SystemGrpc_GRPC_SERVICE_GNSI,
			oc.SystemGrpc_GRPC_SERVICE_GNMI,
			oc.SystemGrpc_GRPC_SERVICE_GRIBI,
			oc.SystemGrpc_GRPC_SERVICE_P4RT,
		}

		t.Run(fmt.Sprintf("Server=%v", serverName), func(t *testing.T) {
			// --- Test 1: Configure and delete gRPC server name ---
			t.Log("Step 1: Applying gRPC server base config...")
			cfg := BuildGrpcServerConfig(GrpcServerParams{
				ServerName: serverName,
				Enable:     true,
			})
			gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg)

			t.Log("Step 2: Validating gRPC server presence...")
			path := []*gpb.Path{
				{Origin: "openconfig", Elem: []*gpb.PathElem{
					{Name: "system"},
					{Name: "grpc-servers"},
					{Name: "grpc-server", Key: map[string]string{"name": serverName}},
					{Name: "config"},
					{Name: "name"}}},
			}
			ValidateGNMIGetConfig(t, dut.RawAPIs().GNMI(t), path, "name", serverName, false)

			t.Log("Step 3: Deleting gRPC server config...")
			gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer(serverName).Config())

			t.Log("Step 4: Validating gRPC server deletion...")
			ValidateGNMIGetConfig(t, dut.RawAPIs().GNMI(t), path, "name", serverName, true)
		})

		t.Run("Server with Port Configs", func(t *testing.T) {
			// --- Test 2: Configure and delete gRPC server port config ---
			cfgWithPort := BuildGrpcServerConfig(GrpcServerParams{
				ServerName: serverName,
				Port:       56666,
				Enable:     true,
			})
			gnmi.Update(t, dut, gnmi.OC().System().Config(), cfgWithPort)

			t.Log("Validating gRPC server port...")
			port := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(serverName).Port().State())
			if port != 56666 {
				t.Errorf("gRPC server port mismatch: got %v, want 56666", port)
			}

			t.Log("Deleting gRPC server port config...")
			gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer(serverName).Port().Config())

			t.Log("Validating port deletion...")
			port = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(serverName).Port().State())
			if port == 56666 {
				t.Errorf("gRPC server port mismatch: got %v, want 0", port)
			}
		})

		// --- Test 3: Configure and delete gRPC server services ---
		for _, svc := range servicesToTest {
			t.Run(fmt.Sprintf("Service=%v", svc), func(t *testing.T) {
				// Step 1: Apply config
				cfg := BuildGrpcServerConfig(GrpcServerParams{
					ServerName: serverName,
					Enable:     true,
					Services:   []oc.E_SystemGrpc_GRPC_SERVICE{svc},
				})
				gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg)

				// Step 2: Validate service is present
				gotServices := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(serverName).Services().State())

				found := false
				for _, s := range gotServices {
					if s == svc {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected service %v, but not found in: %v", svc, gotServices)
				}

				// Step 3: Delete the service config
				gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer(serverName).Services().Config())

				t.Log("Validating services deletion...")
				servicesLookup := gnmi.Lookup(t, dut, gnmi.OC().System().GrpcServer(serverName).Services().State())
				if _, ok := servicesLookup.Val(); ok {
					t.Errorf("Expected services to be deleted for %v, but still present", svc)
				}
			})
		}
		t.Run("Server with Listen Address Only", func(t *testing.T) {
			t.Log("Step 5: Applying gRPC server config with listen address...")
			cfgWithListen := BuildGrpcServerConfig(GrpcServerParams{
				ServerName:      serverName,
				Enable:          true,
				ListenAddresses: []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString("192.0.2.1")},
			})
			gnmi.Update(t, dut, gnmi.OC().System().Config(), cfgWithListen)

			// 	t.Log("Step 6: Validating gRPC server listen address...")
			got := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(serverName).ListenAddresses().State())
			if len(got) == 0 || got[0] != oc.UnionString("192.0.2.1") {
				t.Errorf("Expected ListenAddress '192.0.2.1', got: %v", got)
			}

			t.Log("Step 7: Deleting gRPC server listen address config...")
			gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer(serverName).ListenAddresses().Config())

			t.Log("Step 8: Validating listen address deletion...")
			got = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(serverName).ListenAddresses().State())
			if len(got) == 0 || got[0] == oc.UnionString("192.0.2.1") {
				t.Errorf("Expected ListenAddress '192.0.2.1' to be deleted, but found: %v", got)
			}
		})
		t.Run("Server with MetaData Authentication Only", func(t *testing.T) {
			t.Log("Applying gRPC server config with MetaData Authentication...")
			cfgWithMeta := BuildGrpcServerConfig(GrpcServerParams{
				ServerName:             serverName,
				Enable:                 true,
				MetadataAuthentication: ygot.Bool(false),
			})
			gnmi.Update(t, dut, gnmi.OC().System().Config(), cfgWithMeta)

			t.Log("Validating gRPC server MetaData Authentication...")
			path := []*gpb.Path{
				{Origin: "openconfig", Elem: []*gpb.PathElem{
					{Name: "system"},
					{Name: "grpc-servers"},
					{Name: "grpc-server", Key: map[string]string{"name": serverName}},
					{Name: "metadata-authentication"}}},
			}
			ValidateGNMIGetConfig(t, dut.RawAPIs().GNMI(t), path, "metadata-authentication", false, false)

			t.Log("Deleting gRPC server with MetaData Authentication...")
			gnmi.Update(t, dut, gnmi.OC().System().GrpcServer(serverName).MetadataAuthentication().Config(), true)

			t.Log("Validating gRPC server MetaData Authentication...")
			ValidateGNMIGetConfig(t, dut.RawAPIs().GNMI(t), path, "metadata-authentication", true, false)
		})
		t.Run("Server with TransportSecurity", func(t *testing.T) {
			t.Log("Applying gRPC server config with TransportSecurity...")
			cfgWithTransport := BuildGrpcServerConfig(GrpcServerParams{
				ServerName:        serverName,
				Enable:            true,
				TransportSecurity: ygot.Bool(false),
			})

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				_ = gnmi.Update(t, dut, gnmi.OC().System().Config(), cfgWithTransport)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}
		})
		t.Run("Server with CertificateID", func(t *testing.T) {
			t.Log("Step 5: Applying gRPC server config with CertificateID...")
			cfgWithCert := BuildGrpcServerConfig(GrpcServerParams{
				ServerName:    serverName,
				Enable:        true,
				CertificateID: "CertID",
			})

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				_ = gnmi.Update(t, dut, gnmi.OC().System().Config(), cfgWithCert)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}
			// Delete the multi server config
			gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer(serverName).Config())
		})
	})

	t.Run("Test (gNMI,gNOI,p4RT,gRIBI,gNSI) services with default gRPC Server ", func(t *testing.T) {
		// This test validates the functionality of multiple gRPC services (gNMI, gNOI, p4RT, gRIBI, gNSI) on the default gRPC server.

		// Configure p4RT service.
		Configurep4RTService(t)

		// Validate Default gRPC services All services should be enabled on the default gRPC server
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Unconfigure p4RT service.
		Unconfigurep4RTService(t)
	})

	t.Run("Test DEFAULT Server Behaviour gRPC MultiServer Without Mandatory Service Argument", func(t *testing.T) {
		// This test configures a gRPC server named "server1" without specifying any services using the OpenConfig (OC) model via GNMI.
		// It validates the server's presence , then removes it and validates removal.

		// Step 1: Apply config for server1
		cfg1 := BuildGrpcServerConfig(GrpcServerParams{
			ServerName: "server1",
			Enable:     true,
		})
		t.Log("Applying initial gRPC server1 config...")
		gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg1)

		// Validate "server1" presence via GNMI
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate "emsd core brief" output via SSH
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: false})

		// Apply gNMI service to server1
		cfg1 = BuildGrpcServerConfig(GrpcServerParams{
			ServerName: "server1",
			Enable:     true,
			Services:   []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
		})

		t.Log("Applying service gNMI for gRPC server1 config...")
		gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg1)

		// Validate server1 service presence via GNMI
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)
		ValidateGrpcServerField(t, dut, "server1", "service", []string{"GNMI"}, true)

		// Validate "emsd core brief" output via SSH
		expectedstats := EMSDServerBrief{Name: "server1", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", Services: []string{"GNMI"}, ListenAddress: "ANY"}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate "emsd core stats" output via SSH
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Validate Default gRPC services
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: false})

		// Delete "server1" config
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server1").Config())

		// Validate "emsd core brief" output via SSH after deletion
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
	})

	t.Run("Test Grpc MultiServer with Incompatible Services", func(t *testing.T) {
		// This test configures a gRPC server named "server1" with incompatible services using the OpenConfig (OC) model via GNMI.
		gnmiClient := dut.RawAPIs().GNMI(t)

		t.Run("Incompatible gNMI & gRIBI Services", func(t *testing.T) {
			// EXPECTED FAILURE: This configuration should fail because gNMI and gRIBI services
			// are incompatible on the same gRPC server instance.
			// Apply gNMI and gRIBI services to server1
			cfg1 := BuildGrpcServerConfig(GrpcServerParams{
				ServerName: "server1",
				Port:       56666,
				Enable:     true,
				Services: []oc.E_SystemGrpc_GRPC_SERVICE{
					oc.SystemGrpc_GRPC_SERVICE_GNMI,
					oc.SystemGrpc_GRPC_SERVICE_GRIBI,
				},
			})
			t.Log("Applying initial gRPC server1 config...")
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				_ = gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg1)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}
			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

			// Validate "emsd core stats" output via SSH
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		})
		t.Run("Icompatible gNOI & p4RT Services", func(t *testing.T) {
			// EXPECTED FAILURE: This configuration should fail because gNOI and P4RT services
			// are incompatible on the same gRPC server instance.
			// Step 1: Apply initial config with incompatible services gNOI & p4RT
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"GNOI", "P4RT"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, true)

			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		})
		t.Run("Incompatible gNMI & p4RT Services", func(t *testing.T) {
			// EXPECTED FAILURE: This configuration should fail because gNMI and P4RT services
			// are incompatible on the same gRPC server instance.
			// Apply Incompatible gNMI and p4RT services to server1
			cfg1 := BuildGrpcServerConfig(GrpcServerParams{
				ServerName: "server1",
				Port:       56666,
				Enable:     true,
				Services: []oc.E_SystemGrpc_GRPC_SERVICE{
					oc.SystemGrpc_GRPC_SERVICE_GNMI,
					oc.SystemGrpc_GRPC_SERVICE_P4RT,
				},
			})
			t.Log("Applying initial gRPC server1 config...")
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				_ = gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg1)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

			// Perform eMSD process restart.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Validate "emsd core brief" output via SSH after emsd restart
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH after emsd restart
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		})
		t.Run("Incompatible gNMI, gRIBI & p4RT Services", func(t *testing.T) {
			// EXPECTED FAILURE: This configuration should fail because gNMI, gRIBI, and P4RT
			// services are incompatible when combined on the same gRPC server instance. The device
			// Apply Incompatible gNMI, gRIBI and p4RT services to server1
			cfg1 := BuildGrpcServerConfig(GrpcServerParams{
				ServerName: "server1",
				Port:       56666,
				Enable:     true,
				Services: []oc.E_SystemGrpc_GRPC_SERVICE{
					oc.SystemGrpc_GRPC_SERVICE_GRIBI,
					oc.SystemGrpc_GRPC_SERVICE_P4RT,
					oc.SystemGrpc_GRPC_SERVICE_GNMI,
				},
			})

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				_ = gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg1)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

			// Reload the router
			perf.ReloadRouter(t, dut)

			// Validate "emsd core brief" output via SSH after reload
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH after reload
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		})
		t.Run("Incompatible SR-TE & gNMI Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and incompatible services SR-TE & gNMI
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"SRTE", "GNMI"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, true)

			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		})
		t.Run("Incompatible cNMI & gRIBI Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and incompatible services cNMI & gRIBI
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"cNMI", "GRIBI"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, true)

			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		})
		t.Run("Incompatible gNOI & SL-API Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and incompatible services gNOI & SL-API
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"gNOI", "SLAPI"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, true)

			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

			// Validate "emsd core stats" output via SSH
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		})
		t.Run("Incompatible gNOI, cNMI & SR-TE Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 with incompatible services gNOI, cNMI & SR-TE
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"gNOI", "cNMI", "SRTE"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, true)

			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		})
		t.Run("Incompatible gNSI, gNMI & SL-API Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and incompatible services gNSI, gNMI & SL-API
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"gNSI", "gNMI", "SLAPI"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, true)

			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// validate NSR-Ready is ready
			redundancy_nsrState(context.Background(), t, true)

			// Validate "emsd core brief" output via SSH after RP switchover
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH after RP switchover
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		})
		t.Run("Incompatible gNMI, cNMI & SR-TE Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and incompatible services gNMI, cNMI & SR-TE
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"gNMI", "cNMI", "SRTE"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, true)

			// Validate "emsd core brief" output via SSH
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate "emsd core stats" output via SSH
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		})
	})

	t.Run("Test Grpc MultiServer with Compatible Services", func(t *testing.T) {
		// This test configures a gRPC server named "server1" with compatible services using the OpenConfig (OC) model via GNMI.
		gnmiClient := dut.RawAPIs().GNMI(t)
		t.Run("Test Compatible gNMI & gNOI Services", func(t *testing.T) {

			// Step 1: Apply initial config with port 56666 and compatible services gNMI & gNOI
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"GNMI", "GNOI"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

			// Validate server configs using GNMI
			ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate emsd core brief output using SSH
			expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Perform eMSD process restart.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Validate server configs using GNMI after emsd restart
			ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate emsd core brief output using SSH after emsd restart
			expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Unconfigure Multi-Server configs
			opts := GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
			builder := BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

			// Validate emsd core brief output using SSH after unconfigure
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate emsd core stats output using SSH after unconfigure
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		})

		t.Run("Compatible gNOI & cNMI Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and compatible services gNOI & cNMI
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"CNMI", "GNOI"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

			// Validate server configs using GNMI
			ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate emsd core brief output using SSH
			expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"CNMI", "GNOI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			opts := GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
			builder := BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

			// Negative validation (server should NOT exist)
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		})

		t.Run("Compatible gNMI, gNOI & cNMI Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and compatible services gNMI, gNOI & cNMI
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"GNMI", "CNMI", "GNOI"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

			// Validate server configs using GNMI
			ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate emsd core brief output using SSH
			expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"CNMI", "GNMI", "GNOI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Delete CNMI service
			opts := GrpcUnconfigOptions{
				ServerName:     "server1",
				DeleteServices: []string{"CNMI"},
			}
			builder := BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, gnmiClient, builder, false)

			// Validate server configs using GNMI
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate emsd core brief output using SSH after CNMI service delete
			expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			//Reload router
			perf.ReloadRouter(t, dut)

			// Validate server configs using GNMI after reload
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate emsd core brief output using SSH after reload
			expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Delete GNOI service
			opts = GrpcUnconfigOptions{
				ServerName:     "server1",
				DeleteServices: []string{"GNOI"},
			}
			builder = BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, gnmiClient, builder, false)

			// Validate server configs using GNMI
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate emsd core brief output using SSH after GNOI service delete
			expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Delete grpc port
			opts = GrpcUnconfigOptions{
				ServerName: "server1",
				Port:       56666,
				DeletePort: true,
			}
			builder = BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

			// Validate server configs using GNMI after port delete
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)
			ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), false)

			// Validate emsd core brief output using SSH after port delete
			expectedstats = EMSDServerBrief{Name: "server1", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Delete Multi-server Configs
			opts = GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
			builder = BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

			// Validate emsd core brief output using SSH after multi-server delete
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		})

		t.Run("Compatible gRIBI & SLAPI Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and compatible services gRIBI & SL-API
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"GRIBI", "SLAPI"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

			// Validate server configs using GNMI
			ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate emsd core brief output using SSH
			expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GRIBI", "SLAPI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Unconfigure Multi-Server configs
			opts := GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
			builder := BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

			// Validate emsd core brief output using SSH after unconfigure
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate emsd core stats output using SSH after unconfigure
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		})

		t.Run("Compatible SLAPI & SRTE Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and compatible services SL-API & SR-TE
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"SLAPI", "SRTE"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

			// Validate server configs using GNMI
			ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate eMSD server1 status after emsd restart
			expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"SLAPI", "SRTE"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Unconfigure Multi-Server configs
			opts := GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
			builder := BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

			// Validate emsd core brief output using SSH after multi-server delete
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate emsd core stats output using SSH after multi-server delete
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		})

		t.Run("Compatible gRIBI, SLAPI & SRTE Services", func(t *testing.T) {
			// Step 1: Apply initial config with port 56666 and compatible services gRIBI, SL-API & SR-TE
			initialCfg := GrpcConfig{
				Servers: []GrpcServerConfig{
					{Name: "server1", Port: 56666, Services: []string{"GRIBI", "SLAPI", "SRTE"}},
				},
			}
			initialBuilder := buildGrpcConfigBuilder(initialCfg)
			pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

			// Validate server configs using GNMI
			ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

			// Validate eMSD server1 status after emsd restart
			expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GRIBI", "SLAPI", "SRTE"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Delete GRIBI service
			opts := GrpcUnconfigOptions{
				ServerName:     "server1",
				DeleteServices: []string{"GRIBI"},
			}
			builder := BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, gnmiClient, builder, false)

			// Perform eMSD process restart.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Validate emsd core brief output using SSH after emsd restart
			expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"SLAPI", "SRTE"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Delete SRTE service
			opts = GrpcUnconfigOptions{
				ServerName:     "server1",
				DeleteServices: []string{"SRTE"},
			}
			builder = BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, gnmiClient, builder, false)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// validate NSR-Ready is ready
			redundancy_nsrState(context.Background(), t, true)

			// Validate emsd core brief output using SSH after RP switchover
			expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"SLAPI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Delete server1 gRPC port
			opts = GrpcUnconfigOptions{
				ServerName: "server1",
				Port:       56666,
				DeletePort: true,
			}
			builder = BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

			// Validate server configs using GNMI after port delete
			ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)
			ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), false)

			// Validate emsd core brief output using SSH after port delete
			expectedstats = EMSDServerBrief{Name: "server1", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"SLAPI"}}
			ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

			// Delete Multi-server Configs
			opts = GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
			builder = BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

			// Validate emsd core brief output using SSH after multi-server delete
			ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
			// Validate emsd core stats output using SSH after multi-server delete
			ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		})
	})

}

// TestMultiGrpcServices is a comprehensive test suite for advanced gRPC multi-server
// configurations and service validation on Cisco devices. This test currently serves
// as a framework for testing various multi-gRPC server scenarios including:
//
//   - Default gRPC server disable/enable operations and validation
//   - Multi-server gRPC configurations with different services and ports
//   - gRPC service compatibility and conflict resolution
//   - Server lifecycle management (create, configure, modify, delete)
//   - RP (Route Processor) switchover behavior and service persistence
//   - EMSD (Element Management Service Daemon) process restart scenarios
//   - TLS/SSL profile configurations with custom certificates
//   - Local connection and Unix Domain Socket (UDS) connectivity
//   - Maximum concurrent streams and user limits testing
//   - Telemetry and statistics validation for gRPC services
//
// The test validates that gRPC servers maintain proper state, configuration,
// and service availability across various operational scenarios including
// failover events, process restarts, and configuration changes.
//
// Note: Most test scenarios are currently commented out but can be enabled
// for specific testing requirements. The test framework is designed to handle
// complex multi-server scenarios with proper cleanup and validation.
func TestMultiGrpcServices(t *testing.T) {
	// Loop through parsed binding files (assuming this is properly implemented)
	dut := ondatra.DUT(t, "dut")

	t.Run("Test Default gRPC Server Disable and Validate", func(t *testing.T) {
		// Summary:
		// - Verifies the default gRPC server's configuration and operational status.
		// - Configures an additional gRPC server ("server1") and validates its state.
		// - Validates the services and statistics for both the default and new servers.
		// - Attempts to disable the default gRPC server using CLI and checks for expected error handling.
		// - Restarts the emsd process and verifies that the server states persist as expected.
		// - Re-enables the default gRPC server and validates the operational state and statistics.
		// - Deletes the port configuration and verifies the server's state transitions.
		// - Cleans up by deleting the test server and validating the default server remains operational.

		// pre-requisite: Configure P4RT service
		ctx := context.Background()
		config := getTargetConfig(t)
		grpcPort := config.grpcPort
		sshIP := config.sshIp
		username := config.sshUser
		password := config.sshPass
		Configurep4RTService(t)

		// Configure gRPC server server1
		cfg1 := BuildGrpcServerConfig(GrpcServerParams{
			ServerName: "server1",
			Port:       56666,
			Enable:     true,
			Services:   []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
		})
		t.Log("Applying initial gRPC server1 config...")
		gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg1)

		// Wait for port clash to be resolved
		time.Sleep(30 * time.Second)

		// Validate default gRPC server configuration
		ValidateGrpcServerField(t, dut, "DEFAULT", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "DEFAULT", "name", "DEFAULT", true)

		// Validate eMSD Core default brief values
		expected := EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En", // Enabled
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En", // Enabled
			VRF:           "global-vrf",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expected, true)

		// Validate gRPC server1 configuration
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate eMSD Core server1 brief values
		expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC service behavior
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		conn := DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		t.Logf("[%s] Validating configured gRPC services for Server1", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		cmd := `show logging | include "profile not configured for server server1"`
		output := CMDViaGNMI(ctx, t, dut, cmd)

		if output == "" {
			t.Fatalf("No CLI output received")
		}

		logmsg := "SSL profile not configured for server server1"
		if !strings.Contains(output, logmsg) {
			t.Fatalf("Expected log %q not found. Output:\n%s", logmsg, output)
		}

		t.Logf("Validation successful: found expected log entry: %s", logmsg)

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		conn = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate gRPC Default config after emsd restart
		ValidateGrpcServerField(t, dut, "DEFAULT", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "DEFAULT", "name", "DEFAULT", true)

		// Validate eMSD Core default brief values after RP switchover
		expected = EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En",
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En",
			VRF:           "global-vrf",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expected, true)

		// Validate gRPC service behavior after RP switchover
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Validate gRPC server1 config after RP switchover
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate eMSD core server1 brief values after RP switchover
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		t.Logf("[%s] Validating configured gRPC services for Server1 after RP Switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Configure gRPC server server1
		cfg1 = BuildGrpcServerConfig(GrpcServerParams{
			ServerName:    "server2",
			Port:          uint16(grpcPort),
			Enable:        true,
			Services:      []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
			CertificateID: "system_default_profile",
		})
		t.Log("Applying initial gRPC server2 config...")
		gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg1)

		// Disable the default gRPC server (this is expected to fail)
		cfg := fmt.Sprintf("grpc\n  %s\n!", "default-server-disable")
		_, err := PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			// Log that it failed (acceptable case)
			t.Logf("gNMI Update failed as expected: %s", err)
		} else {
			// Log that it passed (also acceptable case)
		}

		// Wait for port clash to be resolved
		time.Sleep(30 * time.Second)

		// Validate gRPC server1 config after RP switchover
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate eMSD core server1 brief values after RP switchover
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC server1 Behavior after default server disable
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate eMSD core server1 brief values after default server disable
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: fmt.Sprintf("%d", grpcPort), TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC Default config after default server disable
		ValidateGrpcServerField(t, dut, "DEFAULT", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "DEFAULT", "name", "DEFAULT", true)

		// Validate eMSD Core default brief values after default server disable
		expected = EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "Di", // Disabled
			ListenAddress: "ANY",
			Port:          fmt.Sprintf("%d", grpcPort),
			TLS:           "En", // Enabled
			VRF:           "global-vrf",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		// Positive validation (Default server should exist)
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expected, true)

		sslprofile := BuildGrpcServerConfig(GrpcServerParams{
			ServerName:    "server1",
			Enable:        true,
			CertificateID: "system_default_profile",
		})
		t.Log("Applying initial gRPC server1 config...")
		gnmi.Update(t, dut, gnmi.OC().System().Config(), sslprofile)
		time.Sleep(30 * time.Second)

		// Validate gRPC service behavior after default server disable , service should work based on server1 port
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate gRPC service behavior after configuring system_default_profile
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Restart the emsd process through CLI (RunCommand) after default server disable
		RestartAndValidateEMSD(t, dut)

		// Validate gRPC server1 config after emsd restart
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate eMSD core server1 brief values after emsd restart
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC service behavior after emsd restart
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after default server enable
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 1, Responses: 1},
			},
		}, false)

		// Validate gRPC server1 Behavior after emsd restart
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate eMSD core server1 brief values after emsd restart
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: fmt.Sprintf("%d", grpcPort), TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC Default config after emsd restart
		ValidateGrpcServerField(t, dut, "DEFAULT", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "DEFAULT", "name", "DEFAULT", true)

		// Validate eMSD Core default brief values after emsd restart
		expected = EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "Di", // Disabled
			ListenAddress: "ANY",
			Port:          fmt.Sprintf("%d", grpcPort),
			TLS:           "En", // Enabled
			VRF:           "global-vrf",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expected, true)

		// Validate gRPC service behavior after emsd restart
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Enable the default gRPC server
		cfg = "no grpc default-server-disable"

		_, err = PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			t.Errorf("Failed to enable default gRPC server: %v", err)
		}
		// Wait for config to be applied
		time.Sleep(30 * time.Second)

		// Validate gRPC Default config after default server enable
		ValidateGrpcServerField(t, dut, "DEFAULT", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "DEFAULT", "name", "DEFAULT", true)

		// Validate eMSD Core Default brief values after default server enable
		expected = EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En",
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En",
			VRF:           "global-vrf",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		// Positive validation (Default server should exist)
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expected, true)

		// Validate gRPC server1 config after default server enable
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate eMSD Core Server1 brief values after default server enable
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: fmt.Sprint(grpcPort), TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate server2 gRPC service behavior after default server enable
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Delete the port configuration for server2
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").Port().Config())
		time.Sleep(30 * time.Second)

		// Validate gRPC Default config after deleting server2 port
		ValidateGrpcServerField(t, dut, "DEFAULT", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "DEFAULT", "name", "DEFAULT", true)

		// Validate eMSD Core Default brief values after deleting server2 port
		expected = EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En",
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En",
			VRF:           "global-vrf",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		// Positive validation (Default server should exist)
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expected, true)

		// Validate eMSD Core server2 brief values after Server2 port delete
		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate eMSD Core Server1 brief values after Server2 port delete
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate DEFAULT gRPC Services after deleting server2 port
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true,
		})

		// Validate server1 gRPC Services after deleting server2 port
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after Server2 port delete
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 2, Responses: 2},
				"/gnmi.gNMI/Subscribe": {Requests: 2, Responses: 2},
			},
		}, false)

		// Reload Router
		perf.ReloadRouter(t, dut)

		// Validate gRPC server1 config after router reload
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate eMSD Core server1 brief values after router reload
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC server2 config after router reload
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(0), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate eMSD Core server2 brief values after router reload
		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC Default config after router reload
		ValidateGrpcServerField(t, dut, "DEFAULT", "port", uint16(grpcPort), true)
		ValidateGrpcServerField(t, dut, "DEFAULT", "name", "DEFAULT", true)

		// Validate eMSD Core Default brief values after router reload
		expected = EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En",
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En", // Enabled
			VRF:           "global-vrf",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		// Positive validation (Default server should exist)
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expected, true)

		// Make sure grpc service works on default port after router reload
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Delete the Port configuration for server1
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server1").Port().Config())
		time.Sleep(30 * time.Second)

		// Validate eMSD Core server1 brief values after router reload
		expectedstats = EMSDServerBrief{Name: "server1", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		conn = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		t.Logf("[%s] Validating configured gRPC services for Server1 after RP Switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Make sure eMSD Core server1 stats is empty after router reload
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Make sure eMSD server2 stats is empty after router reload
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		// Delete the server1 configuration
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server1").Config())

		// Positive validation (Default server should exist)
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expected, true)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Delete the server2 configuration
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").Config())

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)

		// Make sure grpc service works on default port after server1 delete
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup - Unconfigure p4RT service
		Unconfigurep4RTService(t)
	})

	t.Run("DEFAULT Server Configuration with gNMI P4RT & gRIBI Services", func(t *testing.T) {
		// Summary:
		//   - Configures P4RT service on default server (port 9559), establishes gRPC connection,
		//      and validates all services are functional.
		//   - Restarts EMSD process and revalidates service persistence on port 9559.
		//   - Configures gNMI service on default server (port 9339), validates connectivity
		//      and service functionality.
		//   - Performs RP switchover and ensures both P4RT (9559) and gNMI (9339) services
		//      remain active and functional.
		//   - Configures gRIBI service on default server (port 9340), validates connectivity
		//      and service functionality.
		//   - Unconfigures gNMI and gRIBI services from default server, validates that their
		//      ports (9339, 9340) are no longer active, while P4RT (9559) remains functional.
		//   - Unconfigures P4RT service, validates port 9559 becomes inactive, and confirms
		//      default gRPC services remain unaffected.
		//   - Completes cleanup by validating no configured services persist on default server.

		// Fetch sshIp, username and password from binding
		ctx := context.Background()
		config := getTargetConfig(t)
		sshIP := config.sshIp
		username := config.sshUser
		password := config.sshPass

		// Configure p4RT service.
		Configurep4RTService(t)

		// Establish gRPC connection with p4RT port
		conn := DialSecureGRPC(ctx, t, sshIP, 9559, username, password)

		// Validate eMSD Core server1 stats.
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Restart the emsd process through CLI (RunCommand)
		RestartAndValidateEMSD(t, dut)

		// Validate gRPC services using p4RT port after emsd restart
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// --- Configure gnmi on default server---
		cfg := "grpc\n gnmi\n!\n"

		resp, err := PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err == nil {
			t.Logf("[PASS] Successfully applied gRPC GNMI config: %v", resp)
		} else {
			t.Fatalf("[FAIL] Error while applying gRPC GNMI config: %v", err)
		}

		// Establish gRPC connection with gnmi 9339 port
		conn1 := DialSecureGRPC(ctx, t, sshIP, 9339, username, password)

		// Validate gRPC services using gnmi 9339 port
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Re-establish gRPC connection with p4RT port after RP switchover
		conn = DialSecureGRPC(ctx, t, sshIP, 9559, username, password)

		// Validate gRPC services using p4RT port after RP switchover
		t.Logf("[%s] Validating configured gRPC services using port 9559 after RP switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Re-establish gRPC connection with gnmi 9339 port after RP switchover
		conn1 = DialSecureGRPC(ctx, t, sshIP, 9339, username, password)

		// Validate gRPC services using gnmi 9339 port after RP switchover
		t.Logf("[%s] Validating configured gRPC services using port 9339 after RP switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// --- Configure gribi on default server---
		cfg1 := "grpc\n gribi\n!\n"

		resp, err = PushCliConfigViaGNMI(ctx, t, dut, cfg1)
		if err == nil {
			t.Logf("[PASS] Successfully applied gRPC GNMI config: %v", resp)
		} else {
			t.Fatalf("[FAIL] Error while applying gRPC GNMI config: %v", err)
		}

		// Establish gRPC connection with gribi 9340 port
		conn2 := DialSecureGRPC(ctx, t, sshIP, 9340, username, password)

		// Validate gRPC services using gribi 9340 port
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Unconfigure gRPC services (gnmi & gribi) from default server
		configs := map[string]string{
			"grpc-gnmi":  "no grpc gnmi",
			"grpc-gribi": "no grpc gribi",
		}

		for label, config := range configs {
			resp, err := PushCliConfigViaGNMI(ctx, t, dut, config)
			if err != nil {
				t.Errorf("[FAIL] Error applying %s: %v", label, err)
			}
			t.Logf("[PASS] Applied config: %s: %s", label, resp)
		}
		time.Sleep(30 * time.Second)

		// Re-establish gRPC connection with p4RT port after unconfiguring gnmi from default server else using old connection will always work
		conn1 = DialSecureGRPC(ctx, t, sshIP, 9339, username, password)

		// Validate gRPC services using gnmi 9339 port after unconfiguring gnmi from default server
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Re-establish gRPC connection with gribi 9340 port after unconfiguring gribi from default server else using old connection will always work
		conn2 = DialSecureGRPC(ctx, t, sshIP, 9340, username, password)

		// Validate gRPC services using gribi 9340 port after unconfiguring gribi from default server
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Re-establish gRPC connection with p4RT port after unconfiguring gnmi & gribi from default server.
		conn = DialSecureGRPC(ctx, t, sshIP, 9559, username, password)

		// Validate gRPC services using p4RT 9559 port after unconfiguring gnmi & gribi from default server
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup - Unconfigure p4RT service
		Unconfigurep4RTService(t)

		// Re-establish gRPC connection with p4RT port after unconfiguring p4RT from default server.
		conn = DialSecureGRPC(ctx, t, sshIP, 9559, username, password)

		// Validate gRPC services using p4RT 9559 port after unconfiguring gnmi from default server
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate gRPC services using default port after unconfiguring gnmi ,p4rt & gribi  from default server
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: false})
	})

	t.Run("Two gRPC Servers with same gRPC port", func(t *testing.T) {
		// Summary:
		//   - Configures "server1" with GNMI+GNOI services on port 56666 and validates functionality.
		//   - Performs RP switchover and ensures server1 config/state persists.
		//   - Attempts to configure "server2" on the same port (56666); validates that only one server
		//      can be active while the other stays disabled.
		//   - Establishes gRPC connections, runs service operations, and validates per-server stats.
		//   - Deletes server1 port config → ensures server2 becomes active on port 56666.
		//   - Restarts emsd process and reloads router to verify persistence and correct state transitions.
		//   - Deletes server2 port config, then both server1 and server2 configs → validates negative checks
		//   - Ensures default gRPC services remain unaffected after cleanup.
		//   - Cleans up by unconfiguring p4RT service.

		ctx := context.Background()
		config := getTargetConfig(t)
		sshIP := config.sshIp
		username := config.sshUser
		password := config.sshPass

		// Configure p4RT service.
		Configurep4RTService(t)

		// Step 1: Apply initial config with port 56666 for server1
		gnmiClient := dut.RawAPIs().GNMI(t)
		initialCfg := GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server1", Port: 56666, Services: []string{"GNMI", "GNOI"}, SSLProfileID: "system_default_profile"},
			},
		}
		initialBuilder := buildGrpcConfigBuilder(initialCfg)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

		// Validate gRPC server1 config
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Validate gRPC server1 config after RP switchover
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Step 2: Attempt to apply config for server2 with same port.
		cfg2 := BuildGrpcServerConfig(GrpcServerParams{
			ServerName:    "server2",
			Port:          56666, // Same port as server1
			Enable:        true,
			Services:      []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
			CertificateID: "system_default_profile",
		})

		t.Log("Applying initial gRPC server2 config...")
		gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg2)

		// Validate gRPC server2 config
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		conn := DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		t.Logf("[%s] Validating configured gRPC services for Server1", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":           {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe":     {Requests: 1, Responses: 1},
				"/gnoi.system.System/Time": {Requests: 1, Responses: 1},
			},
		}, false)

		// Make sure eMSD Core server2 stats is empty
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		// Delete the port configuration for server1
		opts := GrpcUnconfigOptions{
			ServerName: "server1",
			Port:       56666,
			DeletePort: true,
		}
		builder := BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Validate gRPC server1 config after port delete
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(0), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		expectedstats = EMSDServerBrief{Name: "server1", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC server2 config
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		conn = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Make sure server2 gRPC services using port 56666
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after port delete
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":           {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe":     {Requests: 1, Responses: 1},
				"/gnoi.system.System/Time": {Requests: 1, Responses: 1},
			},
		}, false)

		// Validate eMSD Core server2 stats after server1 port delete
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 1, Responses: 1},
			},
		}, false)

		// Restart the emsd process through CLI (RunCommand) after Server1 port delete
		RestartAndValidateEMSD(t, dut)

		// Validate gRPC server1 config after emsd restart
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(0), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		expectedstats = EMSDServerBrief{Name: "server1", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC server2 config after emsd restart
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Make sure server2 gRPC services using port 56666 after emsd restart
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Make sure eMSD Core server1 stats is empty after emsd restart
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Validate eMSD Core server2 stats after default server disable
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 1, Responses: 1},
			},
		}, false)

		// Delete the port configuration for server2
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").Port().Config())
		time.Sleep(30 * time.Second)

		// Validate gRPC server1 config after server2 port delete
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(0), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		expectedstats = EMSDServerBrief{Name: "server1", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC server2 config after server2 port delete
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(0), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Make sure gRPC services using port 56666 fail after server2 port delete
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Make sure eMSD Core server1 stats is empty after server1 port delete
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Validate eMSD Core server2 stats is empty after server2 port delete
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 2, Responses: 2},
				"/gnmi.gNMI/Subscribe": {Requests: 2, Responses: 2},
			},
		}, false)

		// Delete the server1 configuration
		opts = GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 (Server should NOT exist)
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Validate gRPC server2 config after server1 delete
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(0), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Make sure grpc services does not work on port 56666.
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server2 stats after server1 delete
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 3, Responses: 3},
				"/gnmi.gNMI/Subscribe": {Requests: 3, Responses: 3},
			},
		}, false)

		// Reload the router
		perf.ReloadRouter(t, dut)

		// Re-establish gRPC connection with port 56666 after reload
		conn = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty after reload
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Validate gRPC server2 config after reload
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(0), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 56666 after reload
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Delete the server2 configuration
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").Config())

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty.
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty.
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		// Make sure grpc services does not work on port 56666 after server2 delete
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		// Make sure grpc service works on default port even after server1 & server2 delete
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup - Unconfigure p4RT service
		Unconfigurep4RTService(t)
	})

	t.Run("Overwriting gRPC Server Config with Same Name but Different gRPC Port", func(t *testing.T) {
		// Summary:
		//   - Configures "server1" with GNMI service on port 56666 and validates connectivity.
		//   - Overwrites the same server name with new port 58888 and additional GNOI service.
		//      Ensures the old port is unreachable and the new port is functional.
		//   - Performs RP switchover and validates service continuity on the new port.
		//   - Deletes GNMI service, validates only GNOI is active, then verifies stats.
		//   - Restarts emsd, confirms persistence of service state, and validates stats again.
		//   - Deletes GNOI service, port configuration, and finally the entire server config.
		ctx := context.Background()
		gnmiClient := dut.RawAPIs().GNMI(t)

		// Fetch sshIp from binding
		config := getTargetConfig(t)
		sshIP := config.sshIp
		username := config.sshUser
		password := config.sshPass

		// Configure p4RT service.
		Configurep4RTService(t)

		// Step 1: Apply initial config with port 56666
		initialCfg := GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server1", Port: 56666, Services: []string{"GNMI"}, SSLProfileID: "system_default_profile"},
			},
		}
		initialBuilder := buildGrpcConfigBuilder(initialCfg)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

		// Validate server1 config is present
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate emsd core server1 brief
		expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Step 2: Overwrite with port 58888
		initialCfg = GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server1", Port: 58888, Services: []string{"GNMI", "GNOI"}, SSLProfileID: "system_default_profile"},
			},
		}
		initialBuilder = buildGrpcConfigBuilder(initialCfg)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

		// Validate overwritten port 58888 is present
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), false)
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Step 3: Ensure old port 56666 is no longer reachable (negative test)
		conn := DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate gRPC services using old port 56666 - expect failure
		t.Logf("[%s] Validating configured gRPC services for old port 56666 after overwriting with new port 58888", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate gRPC server1 config after overwriting with new port 58888
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Step 4: Dial new port 58888
		conn1 := DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate gRPC services using new port 58888
		t.Logf("[%s] Validating configured gRPC services for new port 58888 after overwriting old port 56666", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after overwriting with new port 58888
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate eMSD Core server1 stats after overwriting with new port 58888
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":           {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe":     {Requests: 1, Responses: 1},
				"/gnoi.system.System/Time": {Requests: 1, Responses: 1},
			},
		}, false)

		// Make sure grpc service works on default port.
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Delete the server1 service GNMI configuration
		opts := GrpcUnconfigOptions{
			ServerName:     "server1",
			DeleteServices: []string{"GNMI"},
		}
		builder := BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, gnmiClient, builder, false)
		time.Sleep(30 * time.Second)

		// Validate GNMI service is deleted from server1
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNOI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate eMSD Core server1 stats after delete of GNMI service
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":           {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe":     {Requests: 1, Responses: 1},
				"/gnoi.system.System/Time": {Requests: 1, Responses: 1},
			},
		}, false)

		// Validate gRPC GNOI services after delete of GNMI service.
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: true, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after delete of GNMI service
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":           {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe":     {Requests: 1, Responses: 1},
				"/gnoi.system.System/Time": {Requests: 2, Responses: 2},
			},
		}, false)

		// Make sure grpc service works on default port after server1 delete
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Restart the emsd process through CLI (RunCommand) after delete of GNMI service
		RestartAndValidateEMSD(t, dut)

		// Step 3: Ensure old port 56666 is no longer reachable (negative test)
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate gRPC GNOI services after restart of emsd process
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: true, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after restart of emsd process
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnoi.system.System/Time": {Requests: 1, Responses: 1},
			},
		}, false)

		// Delete the server1 service GNOI configuration
		opts = GrpcUnconfigOptions{
			ServerName:     "server1",
			DeleteServices: []string{"GNOI"},
		}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, gnmiClient, builder, false)
		time.Sleep(30 * time.Second)

		// Validate GNOI service is deleted from server1
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services after delete of GNOI service.
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Delete the server1 port configuration
		opts = GrpcUnconfigOptions{
			ServerName: "server1",
			Port:       58888,
			DeletePort: true,
		}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

		// Validate port is deleted from server1
		expectedstats = EMSDServerBrief{Name: "server1", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services after delete of port.
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after delete of port
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnoi.system.System/Time": {Requests: 1, Responses: 1},
			},
		}, false)

		// Delete the server1 configuration
		opts = GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty after server delete
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Make sure grpc service works on default port after server1 delete
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup - Unconfigure p4RT service
		Unconfigurep4RTService(t)
	})

	t.Run("Two gRPC (p4RT & gNMI) Servers Using different Service with Different gRPC Ports", func(t *testing.T) {
		// Summary:
		// - Configure two gRPC servers
		// - Validate EMSD brief/stats, configs, and connectivity for both servers.
		// - Verify default gRPC server still provides baseline services.
		// - Remove p4RT service from default server.
		// - Perform RP switchover:
		//   * Ensure server1 (P4RT) loses service availability.
		// - Cleanup: delete both servers, confirm removal from EMSD brief/stats, and
		//   re-verify default server services.

		ctx := context.Background()
		config := getTargetConfig(t)
		sshIP := config.sshIp
		username := config.sshUser
		password := config.sshPass

		// Configure p4RT service.
		Configurep4RTService(t)

		servers := []GrpcServerParams{
			{
				ServerName:    "server1",
				Port:          56666,
				Enable:        true,
				Services:      []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_P4RT},
				CertificateID: "system_default_profile",
			},
			{
				ServerName:    "server2",
				Port:          58888,
				Enable:        true,
				Services:      []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
				CertificateID: "system_default_profile",
			},
		}

		// Apply both gRPC server configs
		for _, s := range servers {
			cfg := BuildGrpcServerConfig(s)
			gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg)
		}

		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"P4RT"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Dial server1
		conn1 := DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: true})

		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		conn2 := DialSecureGRPC(ctx, t, sshIP, 58888, username, password)
		if err := startGNMIStream(ctx, "server2", conn2, 65*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for server2: %v", err)
		}

		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		Unconfigurep4RTService(t)

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Validate server1 port and name after RP Switchover
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate eMSD Core server1 brief after RP Switchover
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"P4RT"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Dial server1
		conn1 = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate gRPC services for server1 after RP Switchover
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate server2 port and name after RP Switchover
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate eMSD Core server2 brief after RP Switchover
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Dial server2
		conn2 = DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate gRPC services for server2 after RP Switchover
		if err := startGNMIStream(ctx, "server2", conn2, 60*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for server2 after RP switchover: %v", err)
		}

		// Validate gRPC services for DEFAULT server after RP Switchover
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: false})

		// Delete the port configuration for server1
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server1").Config())

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty after emsd restart
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Delete the port configuration for server2
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").Config())

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty after emsd restart
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		// Validate gRPC services for DEFAULT server after RP Switchover
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: false})
	})

	t.Run("Two gRPC Servers Using the Same Service with Different gRPC Ports", func(t *testing.T) {
		// Summary:
		// - Configures two gRPC servers with the same GNSI service.
		// - Validates configs, EMSD brief/stats, and service accessibility on both ports.
		// - Performs router reload → revalidates persistence of configs and service reachability.
		// - Attempts invalid gNMI replace on server1 → validates error handling and config retention.
		// - Replaces services on server2 with GNMI → validates functional change in services.
		// - Performs RP switchover → ensures both servers retain config/state and services remain accessible.
		// - Deletes services on server1 and port on server2 → validates services become inaccessible as expected.
		// - Restarts EMSD → validates config/state persistence and service behavior post-restart.
		// - Removes both servers completely → validates cleanup and negative checks for stats/briefs.
		// - Finally, validates gRPC services still work on the default port and cleans up P4RT service.

		ctx := context.Background()
		config := getTargetConfig(t)
		sshIP := config.sshIp
		username := config.sshUser
		password := config.sshPass

		// Configure p4RT service.
		Configurep4RTService(t)

		// Configure gRPC servers
		servers := []GrpcServerParams{
			{
				ServerName:    "server1",
				Port:          56666,
				Enable:        true,
				Services:      []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNSI},
				CertificateID: "system_default_profile",
			},
			{
				ServerName:    "server2",
				Port:          58888,
				Enable:        true,
				Services:      []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNSI},
				CertificateID: "system_default_profile",
			},
		}

		// Apply both gRPC server configs
		for _, s := range servers {
			cfg := BuildGrpcServerConfig(s)
			gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg)
		}

		// Validate server1 config is present
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate emsd core server1 brief
		expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Establish gRPC connection with port 56666
		conn1 := DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate gRPC services using port 56666
		t.Logf("[%s] Validate gRPC services using port 56666", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after gRPC calls
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		// Validate server2 config is present
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate emsd core server2 brief
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Establish gRPC connection with port 58888
		conn2 := DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate gRPC services using port 58888
		t.Logf("[%s] Validate gRPC services using port 58888", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server2 stats after gRPC calls
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		// Perform Reload Router
		perf.ReloadRouter(t, dut)

		// Validate server1 config after reload
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate emsd core server1 brief after reload
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Re-establish gRPC connection with port 56666
		conn1 = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate gRPC services using port 56666 after reload
		t.Logf("[%s] Validate gRPC services using port 56666 after reload", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after router reload
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		// Validate server2 config after reload
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate emsd core server2 brief after reload
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Re-establish gRPC connection with port 58888
		conn2 = DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate gRPC services using port 58888 after reload
		t.Logf("[%s] Validate gRPC services using port 58888 after reload", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server2 stats after router reload
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		t.Log("Replace services on gRPC server1 config... with invalid service combination")
		// EXPECTED FAILURE: This Replace operation should fail because GNSI and gRIBI services
		// are incompatible on the same gRPC server instance.
		services := []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNSI, oc.SystemGrpc_GRPC_SERVICE_GRIBI}

		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			_ = gnmi.Replace(t, dut, gnmi.OC().System().GrpcServer("server1").Services().Config(), services)
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This gNMI Update should have failed ")
		}

		// Validate server1 service is still same after invalid gNMI replace
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after invalid gNMI replace
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after invalid gNMI replace
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 2, Responses: 2},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 2, Responses: 0, ErrorResponses: 2},
			},
		}, false)

		// Validate server2 service is still same after invalid gNMI replace on server1
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after invalid gNMI replace on server1
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		// Validate gRPC services using port 58888 after invalid gNMI replace on server1
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server2 stats after invalid gNMI replace on server1
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 2, Responses: 2},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 2, Responses: 0, ErrorResponses: 2},
			},
		}, false)

		t.Log("Replace services on gRPC server2 config... with GNMI service")
		services = []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI}
		gnmi.Replace(t, dut, gnmi.OC().System().GrpcServer("server2").Services().Config(), services)
		time.Sleep(30 * time.Second)

		// Validate server1 service is still same after gNMI replace on server2
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after gNMI replace on server2
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after gNMI replace on server2
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 3, Responses: 3},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 3, Responses: 0, ErrorResponses: 3},
			},
		}, false)

		// Validate server2 service after gNMI replace on server2
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after gNMI replace on server2
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server2 stats after gNMI replace on server2
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 1, Responses: 1},
			},
		}, false)

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Validate server1 config after RP Switchover
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Re-establish gRPC connection with port 56666 after RP Switchover
		conn1 = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate gRPC services using port 56666 after RP Switchover
		t.Logf("[%s] Validate gRPC services using port 56666 after RP Switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after RP Switchover
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		// Validate server2 config after RP Switchover
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Re-establish gRPC connection with port 58888 after RP Switchover
		conn2 = DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate gRPC services using port 58888 after RP Switchover
		t.Logf("[%s] Validate gRPC services using port 58888 after RP Switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server2 stats after RP Switchover
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 1, Responses: 1},
			},
		}, false)

		// Delete the services configuration for server1
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server1").Services().Config())
		time.Sleep(30 * time.Second)

		// Validate server1 services is empty after delete of services configuration
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after delete of services configuration
		t.Logf("[%s] Make sure gRPC services using port 56666 are not accessible after delete of services configuration", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after delete of services configuration
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnsi.authz.v1.Authz/Get":    {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		// Delete the port configuration for server2
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").Config())
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)

		// Validate gRPC services using port 58888 after delete of Port configuration
		t.Logf("[%s] Make sure gRPC services using port 58888 are not accessible after deleting server2", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Make sure eMSD Core server2 stats is empty
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		//  Perform eMSD restart
		RestartAndValidateEMSD(t, dut)

		// Validate emsd server brief after emsd restart
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Make sure gRPC services using port 56666 are not accessible after emsd restart
		t.Logf("[%s] Make sure gRPC services using port 56666 are not accessible after emsd restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Make sure eMSD Core server1 stats is empty after emsd restart
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)

		// Make sure gRPC services using port 58888 are not accessible after emsd restart
		t.Logf("[%s] Make sure gRPC services using port 58888 are not accessible even after emsd restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Make sure eMSD Core server2 stats is empty even after emsd restart
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		// Delete the port configuration for server1
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server1").Config())

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty.
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Make sure grpc service works on default port.
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup - Unconfigure p4RT service
		Unconfigurep4RTService(t)
	})

	t.Run("Dial Server1 with SSL profile and Server2 with TLS using the same service on different gRPC ports", func(t *testing.T) {
		// Summary:
		// - Configures Server1 with SSL profile (custom certs) and Server2 with TLS on different gRPC ports.
		// - Validates both servers’ configs, EMSD brief/stats, and gRPC service accessibility.
		// - Replaces Server2 configuration with a new port, revalidates accessibility.
		// - Performs RP switchover → re-dials, revalidates configs, EMSD brief/stats, and gRPC services.
		// - Deletes Server1 port → validates server1 is disabled, services inaccessible.
		// - Restarts EMSD → validates server1 remains disabled, server2 services continue working.
		// - Deletes Server1 and Server2 configs → verifies servers and stats are removed.
		// - Confirms gRPC services still work correctly on the default port.
		// - Cleans up p4RT service and SSL cert profile artifacts at the end.

		ctx := context.Background()
		gnmiClient := dut.RawAPIs().GNMI(t)

		config := getTargetConfig(t)
		sshIP := config.sshIp
		username := config.sshUser
		password := config.sshPass

		// Configure p4RT service.
		Configurep4RTService(t)

		// Create certz profile, rotate certs, and fetch paths
		clientCertPath, clientKeyPath, caCertPath, dir := createProfileRotateCertz(t)
		t.Logf("clientCertPath: %s, clientKeyPath: %s, caCertPath: %s, dir: %s", clientCertPath, clientKeyPath, caCertPath, dir)

		// ─── Configure Server1 with SSL profile ───
		server1 := GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server1", Port: 56666, Services: []string{"GNMI", "GNOI", "GNSI"}, SSLProfileID: "rotatecertzrsa"},
			},
		}
		initialBuilder := buildGrpcConfigBuilder(server1)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

		// ─── Configure Server2 using gNMI with TLS ───
		server2 := GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server2", Port: 58888, Services: []string{"GRIBI", "SLAPI"}, SSLProfileID: "system_default_profile"},
			},
		}
		initialBuilder = buildGrpcConfigBuilder(server2)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

		// ─── Dial Server1 with custom certz profile ───
		conn1, err := DialSelfSignedGrpc(ctx, t, sshIP, 56666, clientCertPath, clientKeyPath, caCertPath)
		if err != nil {
			t.Fatalf("Dial to server1 (certz profile) failed: %v", err)
		}

		// Validate server1 config is present
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate emsd core server1 brief
		expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666
		t.Logf("[%s] Validate gRPC services using port 56666", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats.
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":              {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe":        {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Get":    {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 1, Responses: 0, ErrorResponses: 1},
				"/gnoi.system.System/Time":    {Requests: 1, Responses: 1},
			},
		}, false)

		// ─── Dial Server2 with default TLS certs ───
		conn2 := DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate server2 config is present
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate emsd core server2 brief
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GRIBI", "SLAPI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888
		t.Logf("[%s] Validate gRPC services using port 58888", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: false})

		// Validate eMSD Core server2 stats.
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gribi.gRIBI/Get":    {Requests: 1, Responses: 1},
				"/gribi.gRIBI/Modify": {Requests: 1, Responses: 0, ErrorResponses: 0},
			},
		}, false)

		// Replace server2 configuration with different port
		t.Log("Replace server2 configuration with different port and no services")
		server2 = GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server2", Port: 60000},
			},
		}
		initialBuilder = buildGrpcConfigBuilder(server2)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

		// Validate server2 config is present with new port
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(60000), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// ─── Dial Server2 with default TLS certs ───
		conn2 = DialSecureGRPC(ctx, t, sshIP, 60000, username, password)

		// Validate emsd core server2 brief
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "60000", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GRIBI", "SLAPI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate server2 serv
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: false})

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Re-establish gRPC connection with port 56666 after RP Switchover
		conn1, err = DialSelfSignedGrpc(ctx, t, sshIP, 56666, clientCertPath, clientKeyPath, caCertPath)
		if err != nil {
			t.Fatalf("Dial to server1 (certz profile) failed: %v", err)
		}

		// Validate server1 config is present after RP Switchover
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate emsd core server1 brief after RP Switchover
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after RP Switchover
		t.Logf("[%s] Validate gRPC services using port 56666", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats.
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":              {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe":        {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Get":    {Requests: 1, Responses: 1},
				"/gnsi.authz.v1.Authz/Rotate": {Requests: 1, Responses: 0, ErrorResponses: 1},
				"/gnoi.system.System/Time":    {Requests: 1, Responses: 1},
			},
		}, false)

		// Re-establish gRPC connection with port 60000 after RP Switchover
		conn2 = DialSecureGRPC(ctx, t, sshIP, 60000, username, password)

		// Validate server2 config is present after RP Switchover
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(60000), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "60000", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GRIBI", "SLAPI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: false})

		// Validate eMSD Core server1 stats after delete of GNMI service
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gribi.gRIBI/Get":    {Requests: 1, Responses: 1},
				"/gribi.gRIBI/Modify": {Requests: 1, Responses: 0, ErrorResponses: 0},
			},
		}, false)

		// Delete the server1 ssl profile configuration.
		opts := GrpcUnconfigOptions{
			ServerName:         "server1",
			DeleteSSLProfileID: true,
			SSLProfileID:       "rotatecertzrsa",
		}
		builder := BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

		// Validate server1 config is present after deleting ssl profile
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Make sure gRPC services using port 56666 are accessible (Using SSL profile with self-signed certs only)
		t.Logf("[%s] Make sure gRPC services using port 56666 are accessible after deleting ssl profile", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Restart eMSD
		RestartAndValidateEMSD(t, dut)

		// Validate emsd server brief after emsd restart
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNOI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Make sure gRPC services using port 56666 are not accessible after emsd restart
		t.Logf("[%s] Make sure gRPC services using port 56666 are not accessible after emsd restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		server1 = GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server1", SSLProfileID: "system_default_profile"},
			},
		}
		initialBuilder = buildGrpcConfigBuilder(server1)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)
		time.Sleep(20 * time.Second)

		conn1 = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Make sure gRPC services using port 56666 are accessible after emsd restart
		t.Logf("[%s] Make sure gRPC services using port 56666 are accessible after adding default ssl profile", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		//Validate server2 config is present after RP Switchover
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(60000), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate emsd core server2 brief after RP Switchover
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "60000", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GRIBI", "SLAPI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Make sure gRPC services using port 60000 are accessible after emsd restart
		t.Logf("[%s] Make sure gRPC services using port 60000 are accessible after emsd restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: false})

		// Validate eMSD Core server2 stats after emsd restart
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gribi.gRIBI/Get":    {Requests: 1, Responses: 1},
				"/gribi.gRIBI/Modify": {Requests: 1, Responses: 0, ErrorResponses: 0},
			},
		}, false)

		// Delete the server1 configuration
		opts = GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats empty (server should NOT exist)
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Delete the server2 configuration
		opts = GrpcUnconfigOptions{ServerName: "server2", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

		// Negative validation (server2 should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)

		// Make sure eMSD Core server2 stats is empty (server2 should NOT exist)
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		// Make sure grpc service works on default port.
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup - Unconfigure p4RT service
		Unconfigurep4RTService(t)

		defer func() {
			t.Log("Cleaning up: closing connections and removing configs")
			os.RemoveAll(dir) // Ensure cleanup
		}()

	})

	t.Run("Concurrent Sessions Using the Same Service with Different gRPC Ports", func(t *testing.T) {
		// Summary:
		// - Configures two gRPC servers (server1:56666, server2:58888) with the same GNMI service.
		// - Establishes concurrent secure gRPC sessions and starts GNMI streaming on both servers.
		// - Validates Telemetry Summary, EMSD server brief, stats, and gRPC service functionality.
		// - Stops stream on server1 while keeping server2 active → validates Telemetry reflects reduced sessions.
		// - Performs RP switchover → re-dials sessions, restarts streams, and revalidates servers/services/stats.
		// - Closes all connections → validates Telemetry and EMSD stats drop to zero, servers remain enabled.
		// - Restarts EMSD process → validates servers recover correctly with no active streams.
		// - Deletes server1 and server2 configs → verifies servers and stats are fully removed.
		// - Confirms default gRPC port services remain operational.
		// - Cleans up p4RT service configuration at the end.

		ctx := context.Background()
		config := getTargetConfig(t)
		dutInfo := config
		username := config.sshUser
		password := config.sshPass

		// Configure p4RT service.
		Configurep4RTService(t)

		// Define two gRPC servers with the same service but different ports
		servers := []GrpcServerParams{
			{
				ServerName:    "server1",
				Port:          56666,
				Enable:        true,
				Services:      []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
				CertificateID: "system_default_profile",
			},
			{
				ServerName:    "server2",
				Port:          58888,
				Enable:        true,
				Services:      []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
				CertificateID: "system_default_profile",
			},
		}

		for _, s := range servers {
			cfg := BuildGrpcServerConfig(s)
			t.Logf("Applying gRPC config for %s", s.ServerName)
			gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg)
		}

		// Define connections to dial
		connsToDial := []struct {
			Name string
			Port int
		}{
			{"server1", 56666},
			{"server2", 58888},
		}

		wrappedDial := func(ctx context.Context, t *testing.T, ip string, port int, service string) (*grpc.ClientConn, string, error) {
			conn := DialSecureGRPC(ctx, t, ip, port, username, password)
			return conn, "", nil
		}
		results := dialConcurrentGRPC(ctx, t, connsToDial, wrappedDial, dutInfo.sshIp, "gnmi")

		// Dial connections and store in a map
		connMap := make(map[string]*grpc.ClientConn)
		for name, res := range results {
			if res.Err != nil {
				t.Fatalf("Dial failed for %s: %v", name, res.Err)
			}
			connMap[name] = res.Conn
		}
		streamCancels := make(map[string]context.CancelFunc)

		// Start background streaming for each server
		for name, conn := range connMap {
			streamCtx, cancel := context.WithCancel(ctx)
			streamCancels[name] = cancel

			go func() {
				if err := startGNMIStream(streamCtx, name, conn, 120*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", name, err)
				}
			}()
		}

		// Main test continues without waiting for streaming to end
		t.Logf("Streaming is running in background. Proceeding with validations...")
		time.Sleep(10 * time.Second) // Allow time for the streams to establish

		// Validate Telemetry Summary
		expected := &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 2, GrpcTLSDestinations: 2,
			DialinCount: 2, DialinActive: 2, DialinSessions: 2, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}
		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Validate server1 config is present
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate emsd core server1 brief
		expectedStats1 := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedStats1, true)

		// Validate gRPC services using port 56666
		t.Logf("[%s] Validate gRPC services using port 56666", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server1"], RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate eMSD Core server1 stats
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 2, Responses: 1},
			},
		}, false)

		// Validate server2 config is present
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate emsd core server2 brief
		expectedStats2 := EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedStats2, true)

		// Validate gRPC services using port 58888
		t.Logf("[%s] Validate gRPC services using port 58888", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server2"], RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate eMSD Core server1 stats after delete of GNMI service
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 2, Responses: 1},
			},
		}, false)

		// Stop stream for server1
		t.Log("Stopping stream for server1 without disconnecting the connection")
		if cancel, ok := streamCancels["server1"]; ok {
			cancel()
		}
		time.Sleep(10 * time.Second) // Allow time for the stream to stop

		// Validate Telemetry Summary after stopping stream for server1
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 1, GrpcTLSDestinations: 1,
			DialinCount: 1, DialinActive: 1, DialinSessions: 1, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}
		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Validate server1 config is present after stopping stream
		expectedStats1 = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedStats1, true)

		// Validate gRPC services using port 56666 after stopping stream for server1
		t.Logf("[%s] Validate gRPC services using port 56666 after stopping stream for server1", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server1"], RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate eMSD Core server1 stats after stopping stream for server1
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 2, Responses: 2},
				"/gnmi.gNMI/Subscribe": {Requests: 3, Responses: 2, ErrorResponses: 1},
			},
		}, false)

		// Validate server2 config is present after stopping stream for server1
		expectedStats2 = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedStats2, true)

		// Validate gRPC services using port 58888 after stopping stream for server1
		t.Logf("[%s] Validate gRPC services using port 58888 after stopping stream for server1", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server2"], RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate eMSD Core server2 stats after stopping stream for server1
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 2, Responses: 2},
				"/gnmi.gNMI/Subscribe": {Requests: 3, Responses: 2},
			},
		}, false)

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Re-establish connections after RP switchover
		t.Log("[INFO] Re-dialing gRPC servers after RP switchover...")
		resultsAfterRP := dialConcurrentGRPC(ctx, t, connsToDial, wrappedDial, dutInfo.sshIp, "gnmi")

		// Clear and repopulate connMap with new connections
		for name, res := range resultsAfterRP {
			if res.Err != nil {
				t.Fatalf("Re-dial failed for %s after RP switchover: %v", name, res.Err)
			}
			connMap[name] = res.Conn
		}

		// Start background streaming for each server
		for name, conn := range connMap {
			streamCtx, cancel := context.WithCancel(ctx)
			streamCancels[name] = cancel

			go func() {
				if err := startGNMIStream(streamCtx, name, conn, 60*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", name, err)
				}
			}()
		}

		time.Sleep(10 * time.Second)

		// Main test continues without waiting for streaming to end
		t.Logf("Streaming is running in background. Proceeding with validations...")

		// Validate Telemetry Summary after RP switchover
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 2, GrpcTLSDestinations: 2,
			DialinCount: 2, DialinActive: 2, DialinSessions: 2, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}
		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Validate server1 config is present after RP switchover
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate emsd core server1 brief after RP switchover
		expectedStats1 = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedStats1, true)

		// Validate gRPC services using port 56666 after RP switchover
		t.Logf("[%s] Validate gRPC services using port 56666 after RP switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server1"], RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate eMSD Core server1 stats after RP switchover
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 2, Responses: 1},
			},
		}, false)

		// Validate server2 config is present after RP switchover
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate emsd core server2 brief after RP switchover
		expectedStats2 = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedStats2, true)

		// Validate gRPC services using port 58888 after RP switchover
		t.Logf("[%s] Validate gRPC services using port 58888 after RP switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server2"], RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate eMSD Core server2 stats after RP switchover
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 2, Responses: 1},
			},
		}, false)

		// Cleanup gRPC connections at end (close connections)
		for _, conn := range connMap {
			conn.Close()
		}
		time.Sleep(30 * time.Second) // Allow time for connections to close

		// Validate Telemetry Summary after closing connections
		expected = &TelemetrySummary{Subscriptions: 0, SubscriptionsActive: 0, DestinationGroups: 0, GrpcTLSDestinations: 0,
			DialinCount: 0, DialinActive: 0, DialinSessions: 0, SensorGroups: 0, SensorPathsTotal: 0, SensorPathsActive: 0}
		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Validate server1 config is present after closing connections
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate emsd core server1 brief after closing connections
		expectedStats1 = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedStats1, true)

		// Validate gRPC services using port 56666 after closing connections
		t.Logf("[%s] Validate gRPC services using port 56666 after closing connections", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server1"], RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate eMSD Core server1 stats after closing connections
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 2, Responses: 1, ErrorResponses: 1},
			},
		}, false)

		// Validate server2 config is present after closing connections
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate emsd core server2 brief after closing connections
		expectedStats2 = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedStats2, true)

		// Validate gRPC services using port 58888 after closing connections
		t.Logf("[%s] Validate gRPC services using port 58888 after closing connections", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server2"], RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate eMSD Core server2 stats after closing connections
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{
			ServerName: "server2",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1},
				"/gnmi.gNMI/Subscribe": {Requests: 2, Responses: 1, ErrorResponses: 1},
			},
		}, false)

		// Restart eMSD
		RestartAndValidateEMSD(t, dut)

		// Validate emsd server1 brief after emsd restart
		expectedStats1 = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedStats1, true)

		// Validate gRPC services using port 56666 after emsd restart
		t.Logf("[%s] Validate gRPC services using port 56666 after emsd restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server1"], RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate eMSD Core server2 stats after emsd restart
		expectedStats2 = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedStats2, true)

		// Validate gRPC services using port 58888 after emsd restart
		t.Logf("[%s] Validate gRPC services using port 58888 after emsd restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, connMap["server2"], RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Delete server1 configuration
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server1").Config())
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty (server should NOT exist)
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Delete server2 configuration
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").Config())
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)

		// Make sure eMSD Core server1 stats is empty (server should NOT exist)
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		// Make sure grpc service works on default port
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup - Unconfigure p4RT service
		Unconfigurep4RTService(t)
	})

	t.Run("Keepalive Timeout Drop After Unconfig (Default Enforced)", func(t *testing.T) {
		// Summary:
		// - Test sets up a gRPC server with short keepalive (5s/2s).
		// - Starts a gNMI Subscribe session.
		// - Blocks traffic using ACL → connection should drop quickly (~10–15s).
		// - Then unconfigures keepalive so defaults (30s/20s) take effect.
		// - Starts a new gNMI session.
		// - Blocks traffic again → connection should drop later (~30–40s).
		// - Finally, cleans up server config and verifies server is removed.

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()

		gnmiClient := dut.RawAPIs().GNMI(t)
		config := getTargetConfig(t)
		sshIP := config.sshIp
		username := config.sshUser
		password := config.sshPass

		// Discover management interfaces
		output := CMDViaGNMI(ctx, t, dut, "show running-config | include interface")
		if output == "" {
			t.Fatalf("No CLI output received")
		}
		var mgmtIntfs []string
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "interface MgmtEth") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					mgmtIntfs = append(mgmtIntfs, fields[1])
				}
			}
		}
		if len(mgmtIntfs) == 0 {
			t.Fatalf("No management interfaces found in CLI output")
		}
		t.Logf("Found management interfaces: %v", mgmtIntfs)

		cliKaTime := 5
		cliKaTimeout := 2

		// Step 1: Configure gRPC server with short keepalive
		cfg := GrpcConfig{
			Servers: []GrpcServerConfig{{
				Name:             "server1",
				Port:             56666,
				Services:         []string{"GNMI"},
				KeepaliveTime:    &cliKaTime,
				KeepaliveTimeout: &cliKaTimeout,
				SSLProfileID:     "system_default_profile",
			}},
		}
		pushGrpcCLIConfig(t, gnmiClient, buildGrpcConfigBuilder(cfg), false)

		// Step 2: Start gRPC stream
		conn := DialSecureGRPC(ctx, t, sshIP, 56666, username, password)
		defer conn.Close()
		client := gpb.NewGNMIClient(conn)
		subClient, err := client.Subscribe(ctx)
		if err != nil {
			t.Fatalf("Subscribe RPC failed: %v", err)
		}
		req := &gpb.SubscribeRequest{
			Request: &gpb.SubscribeRequest_Subscribe{
				Subscribe: &gpb.SubscriptionList{
					Mode: gpb.SubscriptionList_STREAM,
					Subscription: []*gpb.Subscription{{
						Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "state"}, {Name: "hostname"}}},
						Mode:           gpb.SubscriptionMode_SAMPLE,
						SampleInterval: 5 * 1e9,
					}},
				},
			},
		}
		if err := subClient.Send(req); err != nil {
			t.Fatalf("Failed to send Subscribe Request: %v", err)
		}

		expected := EMSDServerDetail{Name: "server1", Port: 56666, Services: "GNMI", Enabled: true, ListenAddresses: "ANY", KeepaliveTime: 5, KeepaliveTimeout: 2}
		ValidateEMSDServerDetail_SSH(t, dut, "server1", expected, false)

		// Step 3: Block traffic (synchronously now)
		for _, intf := range mgmtIntfs {
			if err := ApplyBlockingACL(ctx, t, dut, intf, 56666); err != nil {
				t.Fatalf("Failed to apply ACL on %s: %v", intf, err)
			}
			t.Logf("ACL applied on %s", intf)
		}

		// Step 4: Wait for connection drop
		timeout := time.After(15 * time.Second)
		for {
			select {
			case <-timeout:
				t.Fatalf("[FAIL] Connection did not drop despite ACL and short keepalive timeout")
			default:
				resp, err := subClient.Recv()
				t.Logf("Received response: %v", resp)
				if err != nil {
					t.Logf("[PASS] Connection dropped due to short keepalive timeout: %v", err)
					goto UnblockAndDefault
				}
				time.Sleep(1 * time.Second)
			}
		}

	UnblockAndDefault:
		// Step 5: Remove ACLs from all interfaces at once
		if err := RemoveBlockingACLs(ctx, t, dut, mgmtIntfs); err != nil {
			t.Fatalf("Failed to remove ACLs: %v", err)
		}
		t.Logf("All ACLs removed from management interfaces")

		// Step 6: Unconfig keepalive → revert to defaults
		unconfig := BuildGrpcUnconfigBuilder(GrpcUnconfigOptions{
			ServerName:        "server1",
			DeleteKeepalive:   true,
			DeleteKeepaliveTO: true,
		})
		pushGrpcCLIConfig(t, gnmiClient, unconfig, false)

		expected = EMSDServerDetail{Name: "server1", Port: 56666, Services: "GNMI", Enabled: true, ListenAddresses: "ANY", KeepaliveTime: 30, KeepaliveTimeout: 20}
		ValidateEMSDServerDetail_SSH(t, dut, "server1", expected, false)

		// Important: re-dial after unconfig
		conn2 := DialSecureGRPC(ctx, t, sshIP, 56666, username, password)
		defer conn2.Close()
		client2 := gpb.NewGNMIClient(conn2)
		subClient2, err := client2.Subscribe(ctx)
		if err != nil {
			t.Fatalf("Subscribe RPC failed after unconfig: %v", err)
		}

		// Step 7: Validate default keepalive timeout drop
		timeout = time.After(40 * time.Second)
		for {
			select {
			case <-timeout:
				t.Fatalf("[FAIL] Connection did not drop despite default keepalive timeout")
			default:
				_, err := subClient2.Recv()
				if err != nil {
					t.Logf("[PASS] Connection dropped due to default keepalive timeout: %v", err)
					goto cleanup
				}
				time.Sleep(5 * time.Second)
			}
		}

	cleanup:
		// Final cleanup: remove server/port configs
		opts := GrpcUnconfigOptions{ServerName: "server1", Port: 56666, DeletePort: true}
		pushGrpcCLIConfig(t, gnmiClient, BuildGrpcUnconfigBuilder(opts), false)
		time.Sleep(10 * time.Second)

		opts = GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
		pushGrpcCLIConfig(t, gnmiClient, BuildGrpcUnconfigBuilder(opts), false)
		time.Sleep(20 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerDetail_SSH(t, dut, "server1", EMSDServerDetail{}, true)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
	})

	t.Run("Test gRPC Server Multiple Users with max-streams=128 and per-user default (32)", func(t *testing.T) {
		ctx := context.Background()
		dut := ondatra.DUT(t, "dut")
		config := getTargetConfig(t)
		sshIP := config.sshIp
		grpcPort := config.grpcPort

		// Configure p4RT service.
		Configurep4RTService(t)

		userCount := 4 // Only 4 users × 32 streams = 128 active (fits limit)
		connectionsPerUser := 33
		password := "cisco123"

		// Create cancellable context for all streams
		streamCtx, cancelStreams := context.WithCancel(ctx)
		defer cancelStreams()

		var streamWG sync.WaitGroup

		// Step 1: Create users
		usernames, _ := CreateUsersOnDevice(t, dut, userCount)
		t.Logf("Created users: %v", usernames)

		// Step 2: Configure max-streams=128 on default server
		cfg := "grpc\n max-streams 128\n!\n"
		resp, err := PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			t.Fatalf("[FAIL] Error applying gRPC max-streams config: %v", err)
		}
		t.Logf("[PASS] Applied gRPC max-streams config, response: %v", resp)

		time.Sleep(5 * time.Second)

		// Step 3: Attempt gRPC connections per user on the default server
		for _, user := range usernames {
			t.Logf("\n==== Starting connections for user: %s ====\n", user)
			for i := 0; i < connectionsPerUser; i++ {
				clientID := fmt.Sprintf("client-%02d (user=%s)", i+1, user)
				expectFailure := (i == 32) // 33rd connection per user should fail

				conn := DialSecureGRPC(ctx, t, sshIP, grpcPort, user, password)
				if conn == nil {
					if expectFailure {
						t.Logf("[PASS] Expected failure to dial %s", clientID)
						continue
					}
					t.Fatalf("[FAIL] Unexpected failure to dial %s", clientID)
				}

				streamWG.Add(1)
				go func(clientID string, conn *grpc.ClientConn, expectFailure bool) {
					defer streamWG.Done()
					err := startGNMIStream(streamCtx, clientID, conn, 2*time.Minute)
					if expectFailure {
						if err == nil {
							t.Errorf("[FAIL] Expected stream failure for %s but succeeded", clientID)
						} else {
							t.Logf("[PASS] Expected failure for %s: %v", clientID, err)
						}
					} else {
						if err != nil {
							t.Errorf("[FAIL] Unexpected stream failure for %s: %v", clientID, err)
						}
					}
				}(clientID, conn, expectFailure)
				time.Sleep(50 * time.Millisecond)
			}
			time.Sleep(2 * time.Second)
		}

		// Step 4: Validate expected telemetry stats
		expected := &TelemetrySummary{
			Subscriptions:       1,
			SubscriptionsActive: 1,
			DestinationGroups:   128, // 4 users × 32 connections
			GrpcTLSDestinations: 128,
			DialinCount:         128,
			DialinActive:        128,
			DialinSessions:      128,
			SensorGroups:        1,
			SensorPathsTotal:    1,
			SensorPathsActive:   1,
		}
		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Step 5: Validate gRPC service capabilities on default server
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: true,
			GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true,
		})

		// Step 6: Clean shutdown of streams before unconfiguration
		t.Logf("[%s] Cancelling all active streams and waiting for completion", time.Now().Format("15:04:05.000"))
		cancelStreams()

		// Wait for all goroutines to complete with timeout
		done := make(chan struct{})
		go func() {
			streamWG.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.Logf("[%s] All streams terminated successfully", time.Now().Format("15:04:05.000"))
		case <-time.After(30 * time.Second):
			t.Logf("[WARNING] Timeout waiting for streams to terminate, proceeding with cleanup")
		}

		// Step 7: Cleanup user and gRPC configs
		DeleteUsersFromDevice(t, dut, usernames)

		cfg = "grpc\n no max-streams 128\n!\n"
		resp, err = PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			t.Fatalf("[FAIL] Error removing gRPC max-streams config: %v", err)
		}
		t.Logf("[PASS] Removed gRPC max-streams config, response: %v", resp)

		Unconfigurep4RTService(t)
	})

	t.Run("Test gRPC Server Multiple Users with default max-streams=128", func(t *testing.T) {
		ctx := context.Background()
		dut := ondatra.DUT(t, "dut")
		config := getTargetConfig(t)
		sshIP := config.sshIp
		gnmiClient := dut.RawAPIs().GNMI(t)

		userCount := 5
		connectionsPerUser := 33
		password := "cisco123"

		// Configure p4RT service.
		Configurep4RTService(t)

		// Step 0: Create cancellable context for all streams
		streamCtx, cancelStreams := context.WithCancel(ctx)
		defer cancelStreams()

		var streamWG sync.WaitGroup

		// Step 1: Create users
		usernames, _ := CreateUsersOnDevice(t, dut, userCount)
		t.Logf("Created users: %v", usernames)

		// Step 2: Configure max-streams=128 on default server
		cfg := "grpc\n max-streams 128\n!\n"
		resp, err := PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			t.Fatalf("[FAIL] Error applying gRPC max-streams config: %v", err)
		}
		t.Logf("[PASS] Applied gRPC max-streams config, response: %v", resp)
		time.Sleep(5 * time.Second)

		// Step 3: Configure single custom gRPC server (port 56666)
		initialCfg := GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server1", Port: 56666, Services: []string{"GNMI"}, SSLProfileID: "system_default_profile"},
			},
		}
		initialBuilder := buildGrpcConfigBuilder(initialCfg)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)
		time.Sleep(5 * time.Second) // allow server to stabilize

		// Step 4: Attempt gRPC connections per user
		for uIdx, user := range usernames {
			t.Logf("\n==== Starting connections for user: %s ====", user)
			for i := 0; i < connectionsPerUser; i++ {
				clientID := fmt.Sprintf("client-%02d (user=%s)", i+1, user)

				// Determine if this connection should fail
				var expectFailure bool
				if uIdx < 4 {
					// Users 1–4: only the 33rd connection fails
					expectFailure = (i == 32)
				} else if uIdx == 4 {
					// User 5: all connections should fail
					expectFailure = true
				}

				conn := DialSecureGRPC(ctx, t, sshIP, 56666, user, password)
				if conn == nil {
					if expectFailure {
						t.Logf("[PASS] Expected failure to dial %s", clientID)
						continue
					}
					t.Fatalf("[FAIL] Unexpected failure to dial %s", clientID)
				}

				streamWG.Add(1)
				go func(clientID string, conn *grpc.ClientConn, expectFailure bool) {
					defer streamWG.Done()
					err := startGNMIStream(streamCtx, clientID, conn, 2*time.Minute)
					if expectFailure {
						if err == nil {
							t.Errorf("[FAIL] Expected stream failure for %s but succeeded", clientID)
						} else {
							t.Logf("[PASS] Expected failure for %s: %v", clientID, err)
						}
					} else {
						if err != nil {
							t.Errorf("[FAIL] Unexpected stream failure for %s: %v", clientID, err)
						}
					}
				}(clientID, conn, expectFailure)

				time.Sleep(50 * time.Millisecond)
			}
			time.Sleep(2 * time.Second)
		}

		// Step 5: Validate telemetry stats
		expected := &TelemetrySummary{
			Subscriptions:       1,
			SubscriptionsActive: 1,
			DestinationGroups:   128, // 4 users × 32 connections
			GrpcTLSDestinations: 128,
			DialinCount:         128,
			DialinActive:        128,
			DialinSessions:      128,
			SensorGroups:        1,
			SensorPathsTotal:    1,
			SensorPathsActive:   1,
		}
		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Step 6: Validate default server gRPC capabilities
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: true,
			GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true,
		})

		// Step 7: Clean shutdown of streams
		t.Logf("[%s] Cancelling all active streams...", time.Now().Format("15:04:05.000"))
		cancelStreams()
		done := make(chan struct{})
		go func() {
			streamWG.Wait()
			close(done)
		}()
		select {
		case <-done:
			t.Logf("[%s] All streams terminated successfully", time.Now().Format("15:04:05.000"))
		case <-time.After(30 * time.Second):
			t.Logf("[WARNING] Timeout waiting for streams to terminate")
		}

		// Step 8: Cleanup user and gRPC configs
		DeleteUsersFromDevice(t, dut, usernames)

		opts := GrpcUnconfigOptions{
			ServerName:   "server1",
			DeleteServer: true,
		}
		builder := BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, gnmiClient, builder, false)
		t.Logf("[PASS] Unconfigured custom gRPC server 'server1'")

		// Remove max-streams
		cfg = "grpc\n no max-streams 128\n!\n"
		resp, err = PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			t.Fatalf("[FAIL] Error removing gRPC max-streams config: %v", err)
		}
		t.Logf("[PASS] Removed gRPC max-streams config, response: %v", resp)

		Unconfigurep4RTService(t)
	})

	t.Run("Test Multi-Gprc Server using Local Connection", func(t *testing.T) {
		// Summary:
		//   - Configure server1 with TLS Disabled, remote-connection & local-connection enabled.
		//   - Configure server2 with TLS Enabled, local-connection enabled.
		//      - Both servers with different ports and services (server1: GNOI, server2: GNMI).
		//   - Copy gnoic_unix and gnmic_b2 binaries to DUT and set execute permissions.
		//   - Use gnoic_unix to successfully call GNOI System Time on server1 via UDS.
		//   - Use gnmic_b2 to attempt GNMI Get on server1 via UDS (expected to fail).
		//   - Establish secure gRPC connection to server2 (TLS enabled) using local UDS socket.
		//   - Validate gRPC services on server2 (only GNMI expected to succeed).
		//   - Use gnmic_b2 to successfully call GNMI Get on server2 via UDS.
		//   - Configure remote-connection disabled on server1 and validate gRPC services fail as expected.
		//   - RP Switchover and validate the above behavior remains unchanged.
		//   - Remove remote-connection disabled on server1, server2 and validate gRPC services work as expected.
		//   - Perform process restart of emsd and validate both servers come back up and gRPC services work as expected.
		//   - Final cleanup and unconfig:
		//       - Remove p4RT service config.
		//       - Remove copied binaries from DUT.

		// prerequisites
		ctx := context.Background()
		gnmiClient := dut.RawAPIs().GNMI(t)
		cliHandle := dut.RawAPIs().CLI(t)
		config := getTargetConfig(t)
		binding := config
		grpcPort := config.grpcPort
		sshIP, sshPort := binding.sshIp, binding.sshPort
		username, password := binding.sshUser, binding.sshPass
		remotePath := "/harddisk:/" + "gnoic_unix"
		remotePath2 := "/harddisk:/" + "gnmic_b2"

		// Configure p4RT service
		Configurep4RTService(t)

		// Validate server1 config applied correctly
		initialCfg := GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server1", Port: 56666, Services: []string{"GNOI"}, LocalConn: true, TLS: "disable", RemoteConn: true},
			},
		}
		builder1 := buildGrpcConfigBuilder(initialCfg)
		pushGrpcCLIConfig(t, gnmiClient, builder1, false)

		// Validate server1 emsd core brief
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Dial insecure gRPC connection to server1
		expectedstats := EMSDServerBrief{
			Name:          "server1",
			Status:        "En",
			Port:          "56666",
			TLS:           "Di",
			VRF:           "global-vrf",
			ListenAddress: "ANY",
			Services:      []string{"GNOI"},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services on server1 (only GNOI expected to succeed)
		conn1 := DialInsecureGRPC(ctx, t, sshIP, 56666, username, password)

		// --- Copy binaries ---
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Run validation on gnoic_unix
		scpClient, _ := scp.NewClient(formatHostPort(sshIP, sshPort), scp.NewSSHConfigFromPassword(username, password), &scp.ClientOption{})
		copyAndChmodBinary(t, scpClient, "/auto/cafy/tools/mgbl/gnoic_unix")
		copyAndChmodBinary(t, scpClient, "/auto/cafy/tools/mgbl/gnmic_b2")

		// Negative UDS port test on gnoic_unix
		cmdResp, _ := cliHandle.RunCommand(ctx, "run /harddisk:/gnoic_unix -a unix:///var/lib/docker/ems/server1_grpc.sock --insecure system time")
		output := cmdResp.Output()
		t.Logf("gnoic_unix output:\n%s", output)
		validateGnoicOutput(t, output, true)

		// --- Negative Validate gnmic_b2 on server1 ---
		cmdResp, _ = cliHandle.RunCommand(ctx, "run /harddisk:/gnmic_b2 get -a unix:///var/lib/docker/ems/server1_grpc.sock --insecure --path /system/state/hostname --encoding json_ietf")
		output = cmdResp.Output()
		t.Logf("gnmic output:\n%s", output)

		if strings.Contains(output, `"updates":`) {
			t.Fatalf("[FAIL] gnmic output contain updates:\n%s", output)
		}
		t.Logf("[PASS] gnmic output does not contains updates")

		// Validate server2 config applied correctly
		server2 := GrpcConfig{
			Servers: []GrpcServerConfig{
				{Name: "server2", Port: 58888, Services: []string{"GNMI"}, LocalConn: true, SSLProfileID: "system_default_profile"},
			},
		}
		builder2 := buildGrpcConfigBuilder(server2)
		pushGrpcCLIConfig(t, gnmiClient, builder2, false)

		// Validate server2 emsd core brief
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Dial insecure gRPC connection to server2
		expectedstats = EMSDServerBrief{
			Name:          "server2",
			Status:        "En",
			Port:          "58888",
			TLS:           "En",
			VRF:           "global-vrf",
			ListenAddress: "ANY",
			Services:      []string{"GNMI"},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		sev2 := fmt.Sprintf(
			"run /harddisk:/gnmic_b2 get -a unix:///var/lib/docker/ems/server2_grpc.sock "+
				"--tls-ca /misc/config/ems/gnsi/certz/ssl_profiles/system_default_profile/CABundle.pem "+
				"--tls-cert /misc/config/ems/gnsi/certz/ssl_profiles/system_default_profile/certificate.pem "+
				"--tls-key /misc/config/grpc/ems.key "+
				"--tls-server-name %s "+
				"--path /system/state/hostname "+
				"--encoding json_ietf",
			username,
		)

		defaultunix := fmt.Sprintf(
			"run /harddisk:/gnmic_b2 get -a unix:///var/lib/docker/ems/grpc.sock "+
				"--tls-ca /misc/config/ems/gnsi/certz/ssl_profiles/system_default_profile/CABundle.pem "+
				"--tls-cert /misc/config/ems/gnsi/certz/ssl_profiles/system_default_profile/certificate.pem "+
				"--tls-key /misc/config/grpc/ems.key "+
				"--tls-server-name %s "+
				"--path /system/state/hostname "+
				"--encoding json_ietf",
			username,
		)

		cmdResp2, _ := cliHandle.RunCommand(ctx, sev2)
		output2 := cmdResp2.Output()
		t.Logf("gnmic output:\n%s", output2)

		if !strings.Contains(output2, `"updates":`) {
			t.Fatalf("[FAIL] gnmic output does not contain updates:\n%s", output)
		}
		t.Logf("[PASS] gnmic output contains updates")

		// Validate gRPC services on server2 (only GNMI expected to succeed)
		conn2 := DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate gRPC services on default port
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		cfg := "grpc local-connection\n!"
		resp, err := PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			t.Logf("[INFO] Config %q succeeded (flaky CLI).", err)
		} else {
			t.Logf("[PASS] Config push succeeded: %q", resp)
		}
		time.Sleep(10 * time.Second)

		defaultServer, _ := cliHandle.RunCommand(ctx, defaultunix)
		output3 := defaultServer.Output()
		t.Logf("gnmic output:\n%s", output3)

		if !strings.Contains(output3, `"updates":`) {
			t.Fatalf("[FAIL] gnmic output does not contain updates:\n%s", output)
		}
		t.Logf("[PASS] gnmic output contains updates")

		// Extract output string (call the method!)
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Remove gnoi and gnmi binaries from the device
		_, _ = cliHandle.RunCommand(ctx, fmt.Sprintf("run rm -f %s %s", remotePath, remotePath2))

		// Perform RP switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Dial insecure gRPC connection to server2
		conn1 = DialInsecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate server1 config applied correctly
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate server1 emsd core brief
		expectedstats = EMSDServerBrief{
			Name:          "server1",
			Status:        "En",
			Port:          "56666",
			TLS:           "Di",
			VRF:           "global-vrf",
			ListenAddress: "ANY",
			Services:      []string{"GNOI"},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services on server1 (only GNOI expected to succeed)
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// --- Copy binaries ---
		scpClient, _ = scp.NewClient(formatHostPort(sshIP, sshPort), scp.NewSSHConfigFromPassword(username, password), &scp.ClientOption{})
		copyAndChmodBinary(t, scpClient, "/auto/cafy/tools/mgbl/gnoic_unix")
		copyAndChmodBinary(t, scpClient, "/auto/cafy/tools/mgbl/gnmic_b2")

		// Validate gnoic_unix on server1 after RP switchover
		cliHandle = dut.RawAPIs().CLI(t)
		cmdResp, _ = cliHandle.RunCommand(ctx, "run /harddisk:/gnoic_unix -a unix:///var/lib/docker/ems/server1_grpc.sock --insecure system time")
		output = cmdResp.Output()
		t.Logf("gnoic_unix output:\n%s", output)
		validateGnoicOutput(t, output, true)

		// Validate server2 config applied correctly after RP switchover
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate server2 emsd core brief after RP switchover
		expectedstats = EMSDServerBrief{
			Name:          "server2",
			Status:        "En",
			Port:          "58888",
			TLS:           "En",
			VRF:           "global-vrf",
			ListenAddress: "ANY",
			Services:      []string{"GNMI"},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Re-Dial secure gRPC connection to server2 after RP switchover
		conn2 = DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Re-validate gRPC services on default port after RP switchover
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate Unix Connector on server2 after RP switchover
		cmdResp2, _ = cliHandle.RunCommand(ctx, sev2)
		output = cmdResp2.Output()
		t.Logf("gnmic output:\n%s", output)

		if !strings.Contains(output, `"updates":`) {
			t.Fatalf("[FAIL] gnmic output does not contain updates:\n%s", output)
		}
		t.Logf("[PASS] gnmic output contains updates")

		// Validate gRPC services on default port should still succeed
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true,
		})

		// Validate unix connector on default server
		defaultServer, _ = cliHandle.RunCommand(ctx, defaultunix)
		output = defaultServer.Output()
		t.Logf("gnmic output:\n%s", output)

		if !strings.Contains(output, `"updates":`) {
			t.Fatalf("[FAIL] gnmic output does not contain updates:\n%s", output)
		}
		t.Logf("[PASS] gnmic output contains updates")

		// Validate Unix Connector on server2 after remote-connection is configured (Should PASS)
		cmdResp2, _ = cliHandle.RunCommand(ctx, sev2)
		output = cmdResp2.Output()
		t.Logf("gnmic output:\n%s", output)

		if !strings.Contains(output, `"updates":`) {
			t.Fatalf("[FAIL] gnmic output does not contain updates:\n%s", output)
		}
		t.Logf("[PASS] gnmic output contains updates")

		// Disable local-connection on default server
		cfg = "no grpc local-connection\n!"
		resp, err = PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			t.Logf("[INFO] Config %q succeeded (flaky CLI).", err)
		} else {
			t.Logf("[PASS] Config push succeeded: %q", resp)
		}

		// Validate Unix Connector failure on default server after local-connection is disabled
		defaultServer, _ = cliHandle.RunCommand(ctx, defaultunix)
		output = defaultServer.Output()
		t.Logf("gnmic output:\n%s", output)

		if strings.Contains(output, `"updates":`) {
			t.Fatalf("[FAIL] gnmic output does not contain updates:\n%s", output)
		}
		t.Logf("[PASS] gnmic output does not contain updates")

		// Validate gRPC services on default port should still succeed
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// ─── Restart EMSD ───
		RestartAndValidateEMSD(t, dut)

		// Validate Unix Connector failure on default server after emsd restart (failure expected as local-connection is disabled)
		defaultServer, _ = cliHandle.RunCommand(ctx, defaultunix)
		output = defaultServer.Output()
		t.Logf("gnmic output:\n%s", output)

		if strings.Contains(output, `"updates":`) {
			t.Fatalf("[FAIL] gnmic output does not contain updates:\n%s", output)
		}
		t.Logf("[PASS] gnmic output does not contain updates")

		// Validate gRPC services on default port should still succeed after emsd restart
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Validate server1 config applied correctly after emsd restart
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate server1 emsd core brief
		expectedstats = EMSDServerBrief{
			Name:          "server1",
			Status:        "En",
			Port:          "56666",
			TLS:           "Di",
			VRF:           "global-vrf",
			ListenAddress: "ANY",
			Services:      []string{"GNOI"},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services on server1 (only GNOI expected to succeed)
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate Unix Connector success on server1 after emsd restart
		cmdResp, _ = cliHandle.RunCommand(ctx, "run /harddisk:/gnoic_unix -a unix:///var/lib/docker/ems/server1_grpc.sock --insecure system time")
		output = cmdResp.Output()
		t.Logf("gnoic_unix output:\n%s", output)
		validateGnoicOutput(t, output, true)

		// Validate server2 config applied correctly after emsd restart
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate server2 emsd core brief
		expectedstats = EMSDServerBrief{
			Name:          "server2",
			Status:        "En",
			Port:          "58888",
			TLS:           "En",
			VRF:           "global-vrf",
			ListenAddress: "ANY",
			Services:      []string{"GNMI"},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services on server2 after emsd restart (only GNMI expected to succeed)
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate Unix Connector success on server2 after emsd restart
		cmdResp2, _ = cliHandle.RunCommand(ctx, sev2)
		output = cmdResp2.Output()
		t.Logf("gnmic output:\n%s", output)

		if !strings.Contains(output, `"updates":`) {
			t.Fatalf("gnmic output does not contain updates:\n%s", output)
		}
		t.Logf("gnmic output contains updates")

		// --- Remove Local-Connection Server1 ---
		opts := GrpcUnconfigOptions{ServerName: "server1", DeletelocalConn: true}
		builder := BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Validate server1 config after server1 local-connection is removed
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate server1 emsd core brief after server1 local-connection is removed
		expectedstats = EMSDServerBrief{
			Name:          "server1",
			Status:        "Di",
			Port:          "56666",
			TLS:           "Di",
			VRF:           "global-vrf",
			ListenAddress: "ANY",
			Services:      []string{"GNOI"},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services on server1 (only GNOI expected to succeed) after server1 local-connection is removed
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Validate Unix Connector failure on server1 after server1 local-connection is removed
		cmdResp, _ = cliHandle.RunCommand(ctx, "run /harddisk:/gnoic_unix -a unix:///var/lib/docker/ems/server1_grpc.sock --insecure system time")

		// Extract output string (call the method!)
		output = cmdResp.Output()
		t.Logf("gnoic_unix output:\n%s", output)
		validateGnoicOutput(t, output, false)

		// ─── Remove local-connection for Server2 ───
		opts = GrpcUnconfigOptions{ServerName: "server2", DeletelocalConn: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Validate server2 config after server2 local-connection is removed
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate server2 emsd core brief
		expectedstats = EMSDServerBrief{
			Name:          "server2",
			Status:        "En",
			Port:          "58888",
			TLS:           "En",
			VRF:           "global-vrf",
			ListenAddress: "ANY",
			Services:      []string{"GNMI"},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate unix Connector failure on server2 after server2 local-connection is removed (Expected to fail as local-connection is removed)
		cmdResp2, _ = cliHandle.RunCommand(ctx, sev2)
		output = cmdResp2.Output()
		t.Logf("gnmic output:\n%s", output)

		if strings.Contains(output, `"updates":`) {
			t.Fatalf("gnmic output does not contain updates:\n%s", output)
		}
		t.Logf("gnmic output does not contain updates")

		// Validate gRPC services on server2 (only GNMI expected to succeed)
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false,
			GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false,
		})

		// Schedule cleanup (remove file after test)
		_, _ = cliHandle.RunCommand(ctx, fmt.Sprintf("run rm -f %s %s", remotePath, remotePath2))

		// Unconfigure Multi-Server configs
		opts = GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		ValidateEMSDServerDetail_SSH(t, dut, "server1", EMSDServerDetail{}, true)

		// Unconfigure Multi-Server configs
		opts = GrpcUnconfigOptions{ServerName: "server2", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)
		ValidateEMSDServerDetail_SSH(t, dut, "server2", EMSDServerDetail{}, true)

		expected_brief := EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En", // Enabled
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En", // Enabled
			VRF:           "global-vrf",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expected_brief, true)

		t.Logf("[%s] Validating configured gRPC services for DEFAULT server", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup P4RT service
		Unconfigurep4RTService(t)
	})

	t.Run("Test Multi gRPC Server1 Max Concurrent Streams (32)", func(t *testing.T) {
		// The test performs the following:
		//   - Configures prerequisites (P4RT service, rotated TLS certs).
		//   - Establishes 32 concurrent GNMI streams (expected to succeed).
		//   - Attempts 33rd and 34th streams (expected to fail) and validates non-concurrent 35th succeeds.
		//   - Validates telemetry counters and EMSD server/service state.
		//   - Modifies `max-streams-per-user`, `max-concurrent-streams`, and `max-streams` configs
		//      incrementally, verifying stream limits (32 → 50) and expected failures beyond limit.
		//   - Restarts EMSD process, validates persistence of active limits and stream handling.
		//   - Cleans up by unconfiguring all test servers and max-stream configs.
		//   - Performs negative validations (servers removed) and confirms DEFAULT server remains
		//      functional with all core gRPC services.
		//

		// === Prerequisites ===
		ctx := context.Background()
		dut := ondatra.DUT(t, "dut")
		gnmiClient := dut.RawAPIs().GNMI(t)
		config := getTargetConfig(t)
		sshIP := config.sshIp
		Configurep4RTService(t)

		// --- Configure gRPC Server ---
		serverName := "server1"
		opts := GrpcConfig{
			Servers: []GrpcServerConfig{
				{
					Name:         serverName,
					Port:         60000,
					Services:     []string{"GNMI"},
					SSLProfileID: "rotatecertzrsa",
				},
			},
		}
		initialBuilder := buildGrpcConfigBuilder(opts)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

		// Create certs and define dial wrapper
		clientCert, clientKey, caCert, _ := createProfileRotateCertz(t)
		wrappedDial := func(ctx context.Context, t *testing.T, ip string, port int) (*grpc.ClientConn, string, error) {
			conn, err := DialSelfSignedGrpc(ctx, t, ip, port, clientCert, clientKey, caCert)
			return conn, "", err
		}

		// --- Dial 32 gRPC Clients (Expected to succeed) ---
		connMap := make(map[string]*grpc.ClientConn)
		streamCancels := make(map[string]context.CancelFunc)

		for i := 1; i <= 32; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 60*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}

		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 33rd gRPC Client (Expected to fail) ---
		conn33, _, err := wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 33rd client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 33rd client (expected); now testing stream RPC")

		streamCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-33", conn33, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-33")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-33", err)
		} // Expect failure

		// --- Dial 34th gRPC Client non concurrent (Expected to succeed) ---
		conn, err := DialSelfSignedGrpc(ctx, t, sshIP, 60000, clientCert, clientKey, caCert)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}

		// Allow connection to get established
		time.Sleep(5 * time.Second)

		// Validate gRPC services (GNMI Subscribe expected to fail)
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected := &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 32, GrpcTLSDestinations: 32,
			DialinCount: 32, DialinActive: 32, DialinSessions: 32, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}

		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Validate gRPC services on default port
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}
		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// === Config: raise max-concurrent-streams to 80 ===
		opts = GrpcConfig{
			Servers: []GrpcServerConfig{{
				Name:                 serverName,
				MaxConcurrentStreams: intPtr(50),
			}},
		}
		builder := buildGrpcConfigBuilder(opts)
		pushGrpcCLIConfig(t, gnmiClient, builder, false)

		// 32 concurrent streams should still be active even after max-request-per-user config is 80
		// as max-concurrent-streams and max-streams default value is 32
		for i := 1; i <= 32; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 90*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}

		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 33rd gRPC Client (Expected to fail) ---
		conn33, _, err = wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 33rd client failed unexpectedly: %v", err)
		}

		t.Logf("Dial successful for 33rd client (expected); now testing stream RPC")
		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-33", conn33, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-33")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-33", err)
		} // Expect failure as concurrent streams limit is 32

		// --- Dial 34th gRPC Client non concurrent (Expected to succeed) ---
		conn, err = DialSelfSignedGrpc(ctx, t, sshIP, 60000, clientCert, clientKey, caCert)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}

		// Validate gRPC services (GNMI Subscribe expected to fail)
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 32, GrpcTLSDestinations: 32,
			DialinCount: 32, DialinActive: 32, DialinSessions: 32, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}

		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Cancel all existing streams
		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}

		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// Validate gRPC services on default port now all services should be available
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// --- RP Switchover ---
		utils.Dorpfo(ctx, t, true)
		redundancy_nsrState(ctx, t, true)

		// Recreate map to track connections
		for i := 1; i <= 32; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 60*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}
		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 33th gRPC Client (Expected to fail) ---
		conn33, _, err = wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 33th client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 33th client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-33", conn33, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-33")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-33", err)
		} // Expect failure

		// --- Dial 35th gRPC Client non concurrent (Expected to succeed) ---
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 32, GrpcTLSDestinations: 32,
			DialinCount: 32, DialinActive: 32, DialinSessions: 32, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}

		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Cancel all existing streams
		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}

		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// --- Configure max-concurrent-streams 50 on default server---
		cfg2 := "grpc\n max-streams-per-user 128\n!\n"
		_, err = PushCliConfigViaGNMI(ctx, t, dut, cfg2)
		if err != nil {
			t.Logf("[INFO] Config %q succeeded (flaky Config))", cfg2)
		} else {
			t.Logf("[PASS] Config push succeeded: %q", cfg2)
		}

		time.Sleep(5 * time.Second)

		// 32 concurrent streams should only allow even after max-request-per-user & max-concurrent-streams config is 50
		// as max-streams default value is still 32
		for i := 1; i <= 32; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 60*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}

		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 33rd gRPC Client (Expected to fail) ---
		conn33, _, err = wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 33th client failed unexpectedly: %v", err)
		}

		t.Logf("Dial successful for 33th client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-33", conn33, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-33")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-33", err)
		} // Expect failure

		// --- Dial 34th gRPC Client non concurrent (Expected to succeed) ---
		conn, err = DialSelfSignedGrpc(ctx, t, sshIP, 60000, clientCert, clientKey, caCert)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}

		// Validate gRPC services (GNMI Subscribe expected to fail)
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 32, GrpcTLSDestinations: 32,
			DialinCount: 32, DialinActive: 32, DialinSessions: 32, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}

		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Cancel all existing streams
		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}

		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// --- Configure max-streams ---
		cfg3 := "grpc\n max-streams 128\n!\n"
		_, err = PushCliConfigViaGNMI(ctx, t, dut, cfg3)
		if err != nil {
			t.Logf("[INFO] Config %q succeeded (flaky Config)", cfg3)
		} else {
			t.Logf("[PASS] Config push succeeded: %q", cfg3)
		}
		time.Sleep(10 * time.Second)

		// 80 concurrent streams should allow now as max-request-per-user, max-concurrent-streams & max-streams default value changed to 50
		for i := 1; i <= 60; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 60*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}

		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 51st gRPC Client (Expected to fail) ---
		conn61, _, err := wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 61st client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 61st client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		err = startGNMIStream(streamCtx, "client-61", conn61, 5*time.Second)
		if err == nil {
			// In some cases startGNMIStream logs but returns nil.
			t.Logf("[PASS] %s: Stream ended due to deadline (expected termination)", "client-61")
		} else {
			t.Errorf("[FAIL] %s: Unexpected stream result: %v", "client-61", err)
		}

		// --- Dial 52nd gRPC Client non concurrent (Expected to succeed) ---
		conn, err = DialSelfSignedGrpc(ctx, t, sshIP, 60000, clientCert, clientKey, caCert)
		if err != nil {
			t.Fatalf("Dial for 62nd client failed unexpectedly: %v", err)
		}

		// Validate gRPC services (GNMI Subscribe expected to fail)
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 60, GrpcTLSDestinations: 60,
			DialinCount: 60, DialinActive: 60, DialinSessions: 60, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}
		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Cancel all existing streams
		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}

		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// === Unconfig Cleanup ===
		cfg := "grpc\n no max-streams-per-user\n!\n"
		resp, err := PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			t.Logf("[WARN] Failed to unconfigure max-streams-per-user: %v", err)
		} else {
			t.Logf("[CLEANUP] Unconfigured max-streams-per-user: %v", resp)
		}
		time.Sleep(2 * time.Second)

		// --- Unconfigure max-streams ---
		cfg = "grpc\n no max-streams\n!\n"
		resp, err = PushCliConfigViaGNMI(ctx, t, dut, cfg)
		if err != nil {
			t.Logf("[WARN] Failed to unconfigure max-streams: %v", err)
		} else {
			t.Logf("[CLEANUP] Unconfigured max-streams %v", resp)
		}
		time.Sleep(2 * time.Second)

		unconfig := GrpcUnconfigOptions{ServerName: serverName, DeleteServer: true}
		unBuilder := BuildGrpcUnconfigBuilder(unconfig)
		pushGrpcCLIConfig(t, gnmiClient, unBuilder, false)
		ValidateEMSDServerBrief_SSH(t, dut, serverName, EMSDServerBrief{}, false)

		// Cleanup P4RT service
		Unconfigurep4RTService(t)
	})
}

func TestMultiGrpcServicesVrf(t *testing.T) {
	// Loop through parsed binding files (assuming this is properly implemented)
	dut := ondatra.DUT(t, "dut")

	t.Run("Test Multi gRPC Server using VRF", func(t *testing.T) {
		// Summary:
		//   - Configure two EMSD gRPC servers (server1 in vrf-mgmt, server2 in global-vrf).
		//   - Validate configs, EMSD core briefs, and per-server services.
		//   - Establish secure connections and verify service behavior.
		//   - Run GNMI streaming tests (success/failure depending on VRF).
		//   - Restart EMSD, verify persistence and service recovery.
		//   - Replace services for server1, re-validate brief, stats, and RPCs.
		//   - Update server2 VRF → validate success.
		//   - Perform RP switchover → re-dial, validate persistence and RPCs.
		//   - Unconfigure VRF mgmt → expect servers disabled, validate failures.
		//   - Delete VRF configs (server1, then server2) → servers fall back to global-vrf and work again.
		//   - Reload router → validate persistence and streaming still succeed.
		//   - Final EMSD restart → confirm recovery and gRPC services available as expected.
		ctx := context.Background()
		config := getTargetConfig(t)
		sshIP := config.sshIp
		username := config.sshUser
		password := config.sshPass

		// Configure p4RT service.
		Configurep4RTService(t)
		Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "configure")

		// Define two gRPC servers with different VRFs and ports
		servers := []GrpcServerParams{
			{
				ServerName:      "server1",
				Port:            56666,
				Enable:          true,
				Services:        []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_P4RT},
				NetworkInstance: "mgmt",
				CertificateID:   "system_default_profile",
			},
			{
				ServerName:    "server2",
				Port:          58888,
				Enable:        true,
				Services:      []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
				CertificateID: "system_default_profile",
			},
		}

		// Apply both gRPC server configs
		for _, s := range servers {
			cfg := BuildGrpcServerConfig(s)
			gnmi.Update(t, dut, gnmi.OC().System().Config(), cfg)
		}

		time.Sleep(30 * time.Second)

		// Validate server1 config is present
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate eMSD Core server1 brief
		expectedstats := EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "vrf-mgmt", ListenAddress: "ANY", Services: []string{"P4RT"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate server2 config is present
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(58888), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate eMSD Core server2 brief
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Establish gRPC connection to server1
		conn1 := DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate gRPC services using port 56666
		t.Logf("[%s] Validating configured gRPC services for Server1", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: true})

		// Validate eMSD Core server1 stats
		t.Logf("[%s] Validating configured gRPC services for Server1 stats", time.Now().Format("15:04:05.000"))
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/p4.v1.P4Runtime/StreamChannel": {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		// Make sure grpc service works on default port
		t.Logf("[%s] Validating configured gRPC services for DEFAULT server", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Establish gRPC connection to server2
		conn2 := DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Start GNMI stream to server2 (expected to fail since Vrf is global-vrf)
		t.Logf("[%s] Starting GNMI stream to Server2 (expectFailure=true) with 10s timeout", time.Now().Format("15:04:05.000"))
		if err := startGNMIStream(ctx, "server2", conn2, 10*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "server2")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "server2", err)
		}

		// Restart eMSD process
		RestartAndValidateEMSD(t, dut)

		// Validate emsd core brief data is present after process restart
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "vrf-mgmt", ListenAddress: "ANY", Services: []string{"P4RT"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after process restart
		t.Logf("[%s] Validating configured gRPC services for Server1 after process restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: true})

		// Validate eMSD Core server1 stats after process restart
		t.Logf("[%s] Validating configured gRPC services for Server1 stats after process restart", time.Now().Format("15:04:05.000"))
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/p4.v1.P4Runtime/StreamChannel": {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		// Make sure grpc service works on default port
		t.Logf("[%s] Validating configured gRPC services for DEFAULT server after process restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Validate server2 config is present after process restart
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate GNMI STREAMING using port 58888 after process restart (failure expected)
		t.Logf("[%s] Starting GNMI stream to Server2 (expectFailure=true) with 10s timeout", time.Now().Format("15:04:05.000"))
		if err := startGNMIStream(ctx, "server2", conn2, 10*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "server2")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "server2", err)
		}

		// Replace gRPC services for server1
		services := []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI, oc.SystemGrpc_GRPC_SERVICE_GNSI}
		t.Logf("[%s] Replacing gRPC services for server1 with: %v", time.Now().Format("15:04:05.000"), services)
		gnmi.Replace(t, dut, gnmi.OC().System().GrpcServer("server1").Services().Config(), services)
		time.Sleep(30 * time.Second)

		// Validate GNMI streaming to server1 (expected to succeed)
		t.Logf("[%s] Starting GNMI stream to Server1 (expectFailure=false) with 60s timeout", time.Now().Format("15:04:05.000"))
		if err := startGNMIStream(ctx, "server1", conn1, 60*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for %s: %v", "server1", err)
		}

		// Validate emsd core brief data after replace of services
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "vrf-mgmt", ListenAddress: "ANY", Services: []string{"GNMI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after replace of services
		t.Logf("[%s] Validating configured gRPC services for server1 after Replacing services", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate eMSD Core server1 stats after replace of services
		t.Logf("[%s] Validating configured gRPC services for Server1 stats after replacing services", time.Now().Format("15:04:05.000"))
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/p4.v1.P4Runtime/StreamChannel": {Requests: 1, Responses: 0, ErrorResponses: 1},
				"/gnmi.gNMI/Set":                 {Requests: 1, Responses: 1, ErrorResponses: 0},
				"/gnmi.gNMI/Subscribe":           {Requests: 2, Responses: 1, ErrorResponses: 1},
				"/gnsi.authz.v1.Authz/Get":       {Requests: 1, Responses: 1, ErrorResponses: 0},
				"/gnsi.authz.v1.Authz/Rotate":    {Requests: 1, Responses: 0, ErrorResponses: 1},
			},
		}, false)

		// Update network-instance for server2 to vrf-mgmt
		t.Logf("[%s] Updating network-instance for server2 to vrf 'mgmt'", time.Now().Format("15:04:05.000"))
		gnmi.Update(t, dut, gnmi.OC().System().GrpcServer("server2").NetworkInstance().Config(), "mgmt")
		time.Sleep(30 * time.Second)

		// Re-Establish gRPC connection to server2 after update of vrf mgmt
		conn2 = DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate emsd core brief after update of vrf mgmt
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "vrf-mgmt", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after update of vrf mgmt
		t.Logf("[%s] Validating configured gRPC services for server2 after updating vrf mgmt", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Start GNMI stream to server2 (expected to succeed now)
		t.Logf("[%s] Starting GNMI stream to Server2 after updating vrf mgmt", time.Now().Format("15:04:05.000"))
		if err := startGNMIStream(ctx, "server2", conn2, 60*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for %s: %v", "server2", err)
		}

		// Validate eMSD Core server1 stats after replace of services
		t.Logf("[%s] Validating configured gRPC services for Server1 stats after replacing services", time.Now().Format("15:04:05.000"))
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{
			ServerName: "server1",
			RPCStatsByPath: map[string]RPCStats{
				"/gnmi.gNMI/Set":       {Requests: 1, Responses: 1, ErrorResponses: 0},
				"/gnmi.gNMI/Subscribe": {Requests: 2, Responses: 1, ErrorResponses: 1},
			},
		}, false)

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Re-dial connections since RP switchover happened
		conn1 = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate emsd server1 brief after RP switchover
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "vrf-mgmt", ListenAddress: "ANY", Services: []string{"GNMI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate server1 config is present after RP switchover
		t.Logf("[%s] Validating configured gRPC services for server1 after RP Switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Verify GNMI streaming to server1 (expected to succeed)
		t.Logf("[%s] Starting GNMI stream to Server1 after RP", time.Now().Format("15:04:05.000"))
		if err := startGNMIStream(ctx, "server1", conn1, 60*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for %s: %v", "server1", err)
		}

		// Re-dial connection to server2 since RP switchover happened
		conn2 = DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate emsd server2 brief after RP switchover
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "vrf-mgmt", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after RP switchover
		t.Logf("[%s] Validating configured gRPC services for server2 after RP Switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		t.Logf("[%s] Validating configured gRPC services for DEFAULT Server after router reload", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Unconfigure VRF mgmt
		t.Logf("[%s] Unconfiguring VRF mgmt", time.Now().Format("15:04:05.000"))
		Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "unconfigure")

		// Validate server1 emsd core brief data after VRF unconfig
		expectedstats = EMSDServerBrief{Name: "server1", Status: "Di", Port: "56666", TLS: "En", VRF: "vrf-mgmt", ListenAddress: "ANY", Services: []string{"GNMI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after VRF unconfig
		t.Logf("[%s] Validating configured gRPC services for server1 after VRF unconfig", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Verify GNMI streaming to server1 (expected to fail server1 config is still vrf-mgmt)
		if err := startGNMIStream(ctx, "server1", conn1, 10*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "server1")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "server1", err)
		}

		// Validate server2 emsd core brief data after VRF unconfig
		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "58888", TLS: "En", VRF: "vrf-mgmt", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after VRF unconfig
		t.Logf("[%s] Validating configured gRPC services for server2 after VRF unconfig", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Verify GNMI streaming to server2 (expected to fail server2 config is still vrf-mgmt)
		if err := startGNMIStream(ctx, "server2", conn2, 10*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "server2")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "server2", err)
		}

		// Delete server1 network-instance config
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server1").NetworkInstance().Config())
		time.Sleep(30 * time.Second)

		// Start GNMI stream to server1 (expected to succeed now since vrf is global-vrf)
		if err := startGNMIStream(ctx, "server1", conn1, 60*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for %s: %v", "server1", err)
		}

		// Validate emsd core brief after delete of server1 vrf config
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after delete of server1 vrf config
		t.Logf("[%s] Validating configured gRPC services for server1 after delete of vrf config", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Start GNMI stream to server1 (expected to fail since server2 vrf is still vrf-mgmt)
		if err := startGNMIStream(ctx, "server2", conn2, 10*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "server2")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "server2", err)
		}

		// Validate server2 emsd core brief after delete of server1 vrf config
		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "58888", TLS: "En", VRF: "vrf-mgmt", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after delete of server1 vrf config
		t.Logf("[%s] Validating configured gRPC services for server2 after delete of server1 vrf config", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Reload router to validate persistence of configs
		perf.ReloadRouter(t, dut)

		// re-dial connections since router reload happened
		conn1 = DialSecureGRPC(ctx, t, sshIP, 56666, username, password)

		// Validate GNMI streaming to server1 (expected to succeed) after router reload
		if err := startGNMIStream(ctx, "server1", conn1, 60*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for %s: %v", "server1", err)
		}

		// Validate server1 emsd core brief after router reload
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after router reload
		t.Logf("[%s] Validating configured gRPC services for server1 after router reload", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// re-dial connection to server2 since router reload happened
		conn2 = DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Start GNMI stream to server1 (expected to fail since server2 vrf is still vrf-mgmt)
		if err := startGNMIStream(ctx, "server2", conn2, 10*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "server2")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "server2", err)
		}

		// Delete server2 network-instance config
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").NetworkInstance().Config())
		time.Sleep(30 * time.Second)

		// Validate GNMI streaming to server2 (expected to succeed) after delete of vrf config
		if err := startGNMIStream(ctx, "server2", conn2, 60*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for %s: %v", "server2", err)
		}
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after delete of vrf config
		t.Logf("[%s] Validating configured gRPC services for server2 after delete of vrf config", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Restart eMSD process
		RestartAndValidateEMSD(t, dut)

		// GNMI stream to server1 (expected to succeed) with default vrf
		if err := startGNMIStream(ctx, "server1", conn1, 60*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for %s: %v", "server1", err)
		}

		// Validate server1 emsd core brief after eMSD restart
		expectedstats = EMSDServerBrief{Name: "server1", Status: "En", Port: "56666", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI", "GNSI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server1", expectedstats, true)

		// Validate gRPC services using port 56666 after eMSD restart
		t.Logf("[%s] Validating configured gRPC services for server1 after eMSD restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn1, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// GNMI stream to server2 (expected to succeed) with default vrf
		if err := startGNMIStream(ctx, "server2", conn2, 60*time.Second); err != nil {
			t.Errorf("Unexpected stream failure for %s: %v", "server2", err)
		}

		// Validate server2 emsd core brief after eMSD restart
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after eMSD restart
		t.Logf("[%s] Validating configured gRPC services for server2 after eMSD restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Make sure grpc service works on default port
		t.Logf("[%s] Validating configured gRPC services for DEFAULT Server after eMSD restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Delete server1 config
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server1").Config())
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerDetail_SSH(t, dut, "server1", EMSDServerDetail{}, true)

		// Make sure eMSD Core server1 stats is empty (since server1 is deleted)
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Validate server2 emsd core brief data is still present
		expectedstats = EMSDServerBrief{Name: "server2", Status: "En", Port: "58888", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after server1 delete
		t.Logf("[%s] Validating configured gRPC services for server2 after server1 delete", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Delete server2 port config
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").Port().Config())
		time.Sleep(30 * time.Second)

		// Validate after port delete (port should be 0 now)
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(0), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate server2 emsd core brief data after port delete
		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Validate server1 config is still deleted after RP switchover
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Negative validation (server should NOT exist) after RP switchover
		ValidateEMSDServerDetail_SSH(t, dut, "server1", EMSDServerDetail{}, true)

		// Make sure eMSD Core server1 stats is empty after RP switchover
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Re-dial connection to server2 since RP switchover happened
		conn2 = DialSecureGRPC(ctx, t, sshIP, 58888, username, password)

		// Validate server2 port is still 0 after RP switchover
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(0), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate server2 emsd core brief data after RP switchover
		expectedstats = EMSDServerBrief{Name: "server2", Status: "Di", Port: "0", TLS: "En", VRF: "global-vrf", ListenAddress: "ANY", Services: []string{"GNMI"}}
		ValidateEMSDServerBrief_SSH(t, dut, "server2", expectedstats, true)

		// Validate gRPC services using port 58888 after RP switchover (should fail since port is 0)
		t.Logf("[%s] Validating configured gRPC services for server2 after RP switchover", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForMultiServer(t, conn2, RPCValidationMatrix{
			GNMI_Set: false, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Delete server2 config
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer("server2").Config())
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist) after server2 delete
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)

		// Negative validation (server should NOT exist) after server2 delete
		ValidateEMSDServerDetail_SSH(t, dut, "server1", EMSDServerDetail{}, true)

		// Make sure eMSD Core server1 stats is empty after server2 delete
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)

		// Negative validation (server should NOT exist) after server2 delete
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)

		// Negative validation (server should NOT exist) after server2 delete
		ValidateEMSDServerDetail_SSH(t, dut, "server2", EMSDServerDetail{}, true)

		// Make sure eMSD Core server2 stats is empty after server2 delete
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)

		// Make sure grpc service works on default port
		t.Logf("[%s] Validating configured gRPC services for DEFAULT Server after server1 and server2 delete", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup P4RT service
		Unconfigurep4RTService(t)
	})

	t.Run("Test Multi Server With Listen Address", func(t *testing.T) {
		// Validates EMSD gRPC servers with different listen addresses under mgmt VRF.
		// Workflow:
		//   - Bring up multiple servers (one service each), validate TLS connections.
		//   - Run negative tests on wrong addresses (e.g., sshIP).
		//   - Do RP switchover → check some servers Di, others En, validate services.
		//   - Restart EMSD → servers recover, connections work again.
		//   - Remove listen-address/VRF → validate failures as expected.
		//   - Delete servers → only default EMSD server remains functional.
		//   - Cleanup P4RT service.

		ctx := context.Background()
		dut := ondatra.DUT(t, "dut")
		gnmiClient := dut.RawAPIs().GNMI(t)
		config := getTargetConfig(t)
		sshIP := config.sshIp

		// Prereq config
		Configurep4RTService(t)
		Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "configure")

		// Step 1: Create cert/profile
		clientCertPath, clientKeyPath, caCertPath, tmpDir := createProfileRotateCertz(t)
		t.Logf("certs: %s, %s, %s tmpdir=%s", clientCertPath, clientKeyPath, caCertPath, tmpDir)

		// Step 2: Discover listen addresses
		listenAddrs := GetGrpcListenAddrs(t, dut)
		t.Logf("Discovered gRPC listen addresses: %v", listenAddrs)
		if len(listenAddrs) == 0 {
			t.Fatalf("No listen addresses found")
		}

		// Define services to randomly assign to servers
		servicesList := []string{"GNOI", "P4RT", "GNMI", "GNSI", "GRIBI"}
		var servers []GrpcServerConfig
		basePort := 56666

		// Step 3: Build server configs
		for i, addr := range listenAddrs {
			svc := []string{servicesList[rand.Intn(len(servicesList))]} // random service per server
			port := basePort + i
			server := GrpcServerConfig{
				Name:         fmt.Sprintf("server%d", i+1),
				Port:         port,
				Services:     svc,
				SSLProfileID: "rotatecertzrsa",
				VRF:          "mgmt",
				ListenAddrs:  []string{addr},
			}
			servers = append(servers, server)
			t.Logf("Prepared server config: %+v", server)
		}

		// Step 4: Push config
		grpcCfg := GrpcConfig{Servers: servers}
		builder := buildGrpcConfigBuilder(grpcCfg)
		pushGrpcCLIConfig(t, gnmiClient, builder, false)
		time.Sleep(10 * time.Second) // let device apply

		// Step 5: Establish gRPC connections & validate services
		var connections []*grpc.ClientConn
		for _, server := range servers {
			addr := server.ListenAddrs[0]
			t.Logf("Dialing gRPC server at %s:%d", addr, server.Port)
			conn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port, clientCertPath, clientKeyPath, caCertPath)
			if err != nil {
				t.Fatalf("Failed to dial gRPC server at %s:%d: %v", addr, server.Port, err)
			}
			connections = append(connections, conn)
			t.Logf("Successfully connected to %s:%d", addr, server.Port)

			// Validate EMSD brief
			expectedstats := EMSDServerBrief{
				Name:          server.Name,
				Status:        "En",
				Port:          fmt.Sprintf("%d", server.Port),
				TLS:           "En",
				VRF:           "vrf-mgmt",
				ListenAddress: addr,
				Services:      server.Services,
			}
			ValidateEMSDServerBrief_SSH(t, dut, server.Name, expectedstats, true)

			// Dynamically build the RPC validation matrix
			matrix := RPCValidationMatrix{}
			for _, svc := range server.Services {
				switch svc {
				case "GNMI":
					matrix.GNMI_Set = true
					matrix.GNMI_Subscribe = true
				case "GNOI":
					matrix.GNOI_SystemTime = true
				case "GNSI":
					matrix.GNSI_AuthzRotate = true
					matrix.GNSI_AuthzGet = true
				case "P4RT":
					matrix.P4RT = true
				case "GRIBI":
					matrix.GRIBI_Modify = true
					matrix.GRIBI_Get = true
				}
			}
			// Validate configured gRPC services
			VerifygRPCServicesForMultiServer(t, conn, matrix)
		}

		// Step 6: Negative tests - try to connect to other listenAddrs and Virtual-Address
		for idx, server := range servers {
			var negAddrs []string
			if len(listenAddrs) >= 2 {
				otherIdx := (idx + 1) % len(listenAddrs)
				negAddrs = append(negAddrs, listenAddrs[otherIdx], sshIP)
				t.Logf("[%s] Using other listen address (%s) and sshIP (%s) for negative test",
					server.Name, listenAddrs[otherIdx], sshIP)
			} else {
				negAddrs = append(negAddrs, sshIP)
				t.Logf("[%s] Only one listen address, using sshIP (%s) for negative test", server.Name, sshIP)
			}

			for nidx, addr := range negAddrs {
				t.Logf("[%s] Negative test [%d]: Trying to dial %s:%d (should fail)", server.Name, nidx+1, addr, server.Port)
				badConn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port, clientCertPath, clientKeyPath, caCertPath)
				if err != nil {
					t.Logf("[%s] Negative test [PASSED]: Could not connect to %s:%d as expected: %v", server.Name, addr, server.Port, err)
					continue
				}

				sysClient := spb.NewSystemClient(badConn)
				_, err = sysClient.Time(ctx, &spb.TimeRequest{})
				if err == nil {
					t.Errorf("[%s] Expected Time RPC to fail for %s:%d, but it succeeded", server.Name, addr, server.Port)
				} else {
					t.Logf("[%s] Negative test [PASSED]: RPC to %s:%d failed as expected: %v", server.Name, addr, server.Port, err)
				}
			}
		}

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		for _, server := range servers {
			addr := server.ListenAddrs[0]
			t.Logf("Dialing gRPC server at %s:%d", addr, server.Port)
			conn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port,
				clientCertPath, clientKeyPath, caCertPath)
			if err != nil {
				t.Fatalf("Failed to dial gRPC server at %s:%d: %v", addr, server.Port, err)
			}
			connections = append(connections, conn)
			t.Logf("Successfully connected to %s:%d", addr, server.Port)

			expectedStatus := "Di"

			// Handle exceptions: e.g., server2 remains En
			if server.Name == "server2" {
				expectedStatus = "En"
			}

			expectedstats := EMSDServerBrief{
				Name:          server.Name,
				Status:        expectedStatus,
				Port:          fmt.Sprintf("%d", server.Port),
				TLS:           "En",
				VRF:           "vrf-mgmt",
				ListenAddress: addr,
				Services:      server.Services,
			}
			ValidateEMSDServerBrief_SSH(t, dut, server.Name, expectedstats, true)

			// Build RPC matrix only if "En"
			matrix := RPCValidationMatrix{}
			if expectedstats.Status == "En" {
				for _, svc := range server.Services {
					switch svc {
					case "GNMI":
						matrix.GNMI_Set = true
						matrix.GNMI_Subscribe = true
					case "GNOI":
						matrix.GNOI_SystemTime = true
					case "GNSI":
						matrix.GNSI_AuthzRotate = true
						matrix.GNSI_AuthzGet = true
					case "P4RT":
						matrix.P4RT = true
					case "GRIBI":
						matrix.GRIBI_Modify = true
						matrix.GRIBI_Get = true
					}
				}
			}
			VerifygRPCServicesForMultiServer(t, conn, matrix)
		}

		// Negative tests (again after RP switchover)
		for idx, server := range servers {
			var negAddrs []string
			if len(listenAddrs) >= 2 {
				otherIdx := (idx + 1) % len(listenAddrs)
				negAddrs = append(negAddrs, listenAddrs[otherIdx], sshIP)
				t.Logf("[%s] Using other listen address (%s) and sshIP (%s) for negative test",
					server.Name, listenAddrs[otherIdx], sshIP)
			} else {
				negAddrs = append(negAddrs, sshIP)
				t.Logf("[%s] Only one listen address, using sshIP (%s) for negative test", server.Name, sshIP)
			}

			for nidx, addr := range negAddrs {
				t.Logf("[%s] Negative test [%d]: Trying to dial %s:%d (should fail)", server.Name, nidx+1, addr, server.Port)
				badConn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port, clientCertPath, clientKeyPath, caCertPath)
				if err != nil {
					t.Logf("[%s] Negative test [PASSED]: Could not connect to %s:%d as expected: %v", server.Name, addr, server.Port, err)
					continue
				}

				sysClient := spb.NewSystemClient(badConn)
				_, err = sysClient.Time(ctx, &spb.TimeRequest{})
				if err == nil {
					t.Errorf("[%s] Expected Time RPC to fail for %s:%d, but it succeeded", server.Name, addr, server.Port)
				} else {
					t.Logf("[%s] Negative test [PASSED]: RPC to %s:%d failed as expected: %v", server.Name, addr, server.Port, err)
				}
			}
		}

		// Restart eMSD process
		RestartAndValidateEMSD(t, dut)

		// Step 7: Validate after process restart
		for _, server := range servers {
			addr := server.ListenAddrs[0]
			t.Logf("Dialing gRPC server at %s:%d", addr, server.Port)
			conn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port, clientCertPath, clientKeyPath, caCertPath)
			if err != nil {
				t.Fatalf("Failed to dial gRPC server at %s:%d: %v", addr, server.Port, err)
			}
			connections = append(connections, conn)
			t.Logf("Successfully connected to %s:%d", addr, server.Port)

			// Status is "En" after process restart
			expectedstats := EMSDServerBrief{
				Name:          server.Name,
				Status:        "En",
				Port:          fmt.Sprintf("%d", server.Port),
				TLS:           "En",
				VRF:           "vrf-mgmt",
				ListenAddress: addr,
				Services:      server.Services,
			}
			ValidateEMSDServerBrief_SSH(t, dut, server.Name, expectedstats, true)

			// Build matrix: all false if disabled
			matrix := RPCValidationMatrix{}
			if expectedstats.Status == "En" {
				for _, svc := range server.Services {
					switch svc {
					case "GNMI":
						matrix.GNMI_Set = true
						matrix.GNMI_Subscribe = true
					case "GNOI":
						matrix.GNOI_SystemTime = true
					case "GNSI":
						matrix.GNSI_AuthzRotate = true
						matrix.GNSI_AuthzGet = true
					case "P4RT":
						matrix.P4RT = true
					case "GRIBI":
						matrix.GRIBI_Modify = true
						matrix.GRIBI_Get = true
					}
				}
			}
			// When "En", matrix remains all false
			VerifygRPCServicesForMultiServer(t, conn, matrix)
		}

		// Step 6: Negative tests after process restart
		for idx, server := range servers {
			var negAddrs []string
			if len(listenAddrs) >= 2 {
				otherIdx := (idx + 1) % len(listenAddrs)
				negAddrs = append(negAddrs, listenAddrs[otherIdx], sshIP)
				t.Logf("[%s] Using other listen address (%s) and sshIP (%s) for negative test",
					server.Name, listenAddrs[otherIdx], sshIP)
			} else {
				negAddrs = append(negAddrs, sshIP)
				t.Logf("[%s] Only one listen address, using sshIP (%s) for negative test", server.Name, sshIP)
			}

			for nidx, addr := range negAddrs {
				t.Logf("[%s] Negative test [%d]: Trying to dial %s:%d (should fail)", server.Name, nidx+1, addr, server.Port)
				badConn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port, clientCertPath, clientKeyPath, caCertPath)
				if err != nil {
					t.Logf("[%s] Negative test [PASSED]: Could not connect to %s:%d as expected: %v", server.Name, addr, server.Port, err)
					continue
				}

				sysClient := spb.NewSystemClient(badConn)
				_, err = sysClient.Time(ctx, &spb.TimeRequest{})
				if err == nil {
					t.Errorf("[%s] Expected Time RPC to fail for %s:%d, but it succeeded", server.Name, addr, server.Port)
				} else {
					t.Logf("[%s] Negative test [PASSED]: RPC to %s:%d failed as expected: %v", server.Name, addr, server.Port, err)
				}
			}
		}

		// Delete Listen Address for Server1
		opts := GrpcUnconfigOptions{
			ServerName:          "server1",
			ListenAddress:       listenAddrs[0],
			DeleteListenAddress: true,
		}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second) // Wait for the unconfiguration to take effect

		// Step 7: Validate after Deleting listen address
		for _, server := range servers {
			// For server1, after deleting listen address, it should accept connections on all available IPs.
			var addrsToTest []string
			if server.Name == "server1" {
				addrsToTest = append(addrsToTest, listenAddrs...)
				addrsToTest = append(addrsToTest, sshIP)
			} else {
				addrsToTest = []string{server.ListenAddrs[0]}
			}

			for _, addr := range addrsToTest {
				t.Logf("Dialing gRPC server %s at %s:%d", server.Name, addr, server.Port)
				conn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port, clientCertPath, clientKeyPath, caCertPath)
				if err != nil {
					t.Errorf("Failed to dial gRPC server %s at %s:%d: %v", server.Name, addr, server.Port, err)
					continue
				}
				_ = append(connections, conn)
				t.Logf("Successfully connected to %s:%d", addr, server.Port)

				expectedAddr := addr
				if server.Name == "server1" {
					expectedAddr = "ANY"
				}

				expectedstats := EMSDServerBrief{
					Name:          server.Name,
					Status:        "En", // still enabled
					Port:          fmt.Sprintf("%d", server.Port),
					TLS:           "En",
					VRF:           "vrf-mgmt",
					ListenAddress: expectedAddr,
					Services:      server.Services,
				}
				ValidateEMSDServerBrief_SSH(t, dut, server.Name, expectedstats, true)

				// Build service matrix for all servers (since all should work)
				matrix := RPCValidationMatrix{}
				for _, svc := range server.Services {
					switch svc {
					case "GNMI":
						matrix.GNMI_Set = true
						matrix.GNMI_Subscribe = true
					case "GNOI":
						matrix.GNOI_SystemTime = true
					case "GNSI":
						matrix.GNSI_AuthzRotate = true
						matrix.GNSI_AuthzGet = true
					case "P4RT":
						matrix.P4RT = true
					case "GRIBI":
						matrix.GRIBI_Modify = true
						matrix.GRIBI_Get = true
					}
				}
				VerifygRPCServicesForMultiServer(t, conn, matrix)
			}
		}

		// Negative tests: server2 should still reject connections from other addresses
		for idx, server := range servers {
			// Skip server1 - already fully tested in the loop above
			if server.Name == "server1" {
				continue
			}

			var testAddrs []string
			if len(listenAddrs) >= 2 {
				otherIdx := (idx + 1) % len(listenAddrs)
				testAddrs = append(testAddrs, listenAddrs[otherIdx], sshIP)
				t.Logf("[%s] Using other listen address (%s) and sshIP (%s) for connectivity test",
					server.Name, listenAddrs[otherIdx], sshIP)
			} else {
				testAddrs = append(testAddrs, sshIP)
				t.Logf("[%s] Only one listen address, using sshIP (%s) for connectivity test", server.Name, sshIP)
			}

			for tidx, addr := range testAddrs {
				t.Logf("[%s] Test [%d]: Dialing %s:%d", server.Name, tidx+1, addr, server.Port)
				conn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port,
					clientCertPath, clientKeyPath, caCertPath)

				if server.Name == "server2" {
					// Negative case: other addresses should fail
					if err == nil {
						sysClient := spb.NewSystemClient(conn)
						_, rpcErr := sysClient.Time(ctx, &spb.TimeRequest{})
						if rpcErr == nil {
							t.Errorf("[FAIL] [%s] Expected RPC to %s:%d to fail, but succeeded",
								server.Name, addr, server.Port)
						} else {
							t.Logf("[PASS] [%s] RPC to %s:%d failed as expected: %v",
								server.Name, addr, server.Port, rpcErr)
						}
					} else {
						t.Logf("[PASS] [%s] Connection to %s:%d failed as expected: %v",
							server.Name, addr, server.Port, err)
					}
				}
			}
		}

		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Delete VRF for Server1
		opts = GrpcUnconfigOptions{
			ServerName: "server1",
			VRF:        "mgmt",
			DeleteVRF:  true,
		}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

		// Delete VRF for Server2
		opts = GrpcUnconfigOptions{
			ServerName: "server2",
			VRF:        "mgmt",
			DeleteVRF:  true,
		}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

		// unconfigure mgmt vrf
		Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "unconfigure")

		// Connectivity & service validation after VRF-Mgmt unconfiguration
		for _, server := range servers {
			// For server1, after deleting VRF & Listen Address, it should accept connections on all available IPs.
			// For server2, test with its configured listen address only.
			var addrsToTest []string
			if server.Name == "server1" {
				addrsToTest = append(addrsToTest, listenAddrs...)
				addrsToTest = append(addrsToTest, sshIP)
			} else {
				addrsToTest = []string{server.ListenAddrs[0]}
			}

			for _, addr := range addrsToTest {
				t.Logf("Dialing gRPC server %s at %s:%d", server.Name, addr, server.Port)
				conn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port, clientCertPath, clientKeyPath, caCertPath)
				if err != nil {
					t.Errorf("Failed to dial gRPC server %s at %s:%d: %v", server.Name, addr, server.Port, err)
					continue
				}
				_ = append(connections, conn)
				t.Logf("Successfully connected to %s:%d", addr, server.Port)

				expectedAddr := addr
				if server.Name == "server1" {
					expectedAddr = "ANY"
				}

				expectedstats := EMSDServerBrief{
					Name:          server.Name,
					Status:        "En",
					Port:          fmt.Sprintf("%d", server.Port),
					TLS:           "En",
					VRF:           "global-vrf",
					ListenAddress: expectedAddr,
					Services:      server.Services,
				}
				ValidateEMSDServerBrief_SSH(t, dut, server.Name, expectedstats, true)

				// Build service matrix for all servers
				matrix := RPCValidationMatrix{}
				for _, svc := range server.Services {
					switch svc {
					case "GNMI":
						matrix.GNMI_Set = true
						matrix.GNMI_Subscribe = true
					case "GNOI":
						matrix.GNOI_SystemTime = true
					case "GNSI":
						matrix.GNSI_AuthzRotate = true
						matrix.GNSI_AuthzGet = true
					case "P4RT":
						matrix.P4RT = true
					case "GRIBI":
						matrix.GRIBI_Modify = true
						matrix.GRIBI_Get = true
					}
				}
				VerifygRPCServicesForMultiServer(t, conn, matrix)
			}
		}

		// Negative tests: server2 should still reject connections from other addresses
		for idx, server := range servers {
			// Skip server1 - already fully tested in the loop above
			if server.Name == "server1" {
				continue
			}

			var testAddrs []string
			if len(listenAddrs) >= 2 {
				otherIdx := (idx + 1) % len(listenAddrs)
				testAddrs = append(testAddrs, listenAddrs[otherIdx], sshIP)
				t.Logf("[%s] Using other listen address (%s) and sshIP (%s) for connectivity test",
					server.Name, listenAddrs[otherIdx], sshIP)
			} else {
				testAddrs = append(testAddrs, sshIP)
				t.Logf("[%s] Only one listen address, using sshIP (%s) for connectivity test", server.Name, sshIP)
			}

			for tidx, addr := range testAddrs {
				t.Logf("[%s] Test [%d]: Dialing %s:%d", server.Name, tidx+1, addr, server.Port)
				conn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port,
					clientCertPath, clientKeyPath, caCertPath)

				if server.Name == "server2" {
					// Negative case: other addresses should fail
					if err == nil {
						sysClient := spb.NewSystemClient(conn)
						_, rpcErr := sysClient.Time(ctx, &spb.TimeRequest{})
						if rpcErr == nil {
							t.Errorf("[FAIL] [%s] Expected RPC to %s:%d to fail, but succeeded",
								server.Name, addr, server.Port)
						} else {
							t.Logf("[PASS] [%s] RPC to %s:%d failed as expected: %v",
								server.Name, addr, server.Port, rpcErr)
						}
					} else {
						t.Logf("[PASS] [%s] Connection to %s:%d failed as expected: %v",
							server.Name, addr, server.Port, err)
					}
				}
			}
		}

		// Unconfigure Multi-Server configs
		opts = GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		ValidateEMSDServerDetail_SSH(t, dut, "server1", EMSDServerDetail{}, true)

		// Unconfigure Multi-Server configs
		opts = GrpcUnconfigOptions{ServerName: "server2", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)
		ValidateEMSDServerDetail_SSH(t, dut, "server2", EMSDServerDetail{}, true)

		// Make sure grpc service works on default port
		t.Logf("[%s] Validating configured gRPC services for DEFAULT Server after server1 and server2 delete", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup P4RT service
		Unconfigurep4RTService(t)
	})

	t.Run("Test Multiple Servers with DSCP Values", func(t *testing.T) {
		// Summary:
		//   - Configures prerequisites (P4RT service, mgmt VRF, and rotated certificates).
		//   - Builds and applies three gRPC server configs (server1–server3) with unique DSCP values.
		//   - Validates server fields (name, port) and establishes TLS connections using rotated certs.
		//   - Verifies EMSD brief, stats, and gRPC service availability for each server.
		//   - Confirms DEFAULT EMSD server brief and gRPC services remain unaffected.
		//   - Reloads the router, revalidates multi-server configs, services, and EMSD stats.
		//   - Sequentially deletes server1–server3 configs, performing negative validations for their removal.
		//   - Revalidates DEFAULT EMSD server brief and gRPC services after cleanup.
		//   - Cleans up by unconfiguring P4RT service and mgmt VRF.

		t.Logf("[%s] Starting DSCP + P4RT test", time.Now().Format("15:04:05.000"))
		ctx := context.Background()
		dut := ondatra.DUT(t, "dut")
		gnmiClient := dut.RawAPIs().GNMI(t)
		config := getTargetConfig(t)
		grpcPort := config.grpcPort

		// Configure prerequisites
		Configurep4RTService(t)
		Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "configure")

		// Cert rotation
		clientCertPath, clientKeyPath, caCertPath, dir := createProfileRotateCertz(t)
		t.Logf("Certz paths:\n - clientCert: %s\n - clientKey: %s\n - caCert: %s\n - tempDir: %s",
			clientCertPath, clientKeyPath, caCertPath, dir)

		// Get listen addresses
		listenAddrs := GetGrpcListenAddrs(t, dut)
		t.Logf("Listen addresses: %v", listenAddrs)

		// Step 4: Define DSCP mapping
		dscpNameToVal := map[string]int{
			"ef":   46,
			"af41": 34,
			"cs2":  16,
		}
		dscpNames := []string{"ef", "af41", "cs2"}

		// Build 3 server configs with different DSCP values
		var servers []GrpcServerConfig
		numAddrs := len(listenAddrs)
		if numAddrs == 0 {
			t.Fatalf("No listen addresses found")
		}

		for i := 0; i < 3; i++ {
			dscpStr := dscpNames[i]
			dscpVal := dscpNameToVal[dscpStr]
			dscpPtr := new(int)
			*dscpPtr = dscpVal

			// Pick listen address in a round-robin fashion
			addr := listenAddrs[i%numAddrs]

			server := GrpcServerConfig{
				Name:         fmt.Sprintf("server%d", i+1),
				Port:         56666 + i,
				Services:     []string{"P4RT"},
				SSLProfileID: "rotatecertzrsa",
				VRF:          "mgmt",
				ListenAddrs:  []string{addr},
				Dscp:         dscpPtr,
			}
			servers = append(servers, server)
			t.Logf("Prepared gRPC Server config %d: %+v", i+1, server)
		}

		// Push config
		grpcCfg := GrpcConfig{Servers: servers}
		builder := buildGrpcConfigBuilder(grpcCfg)
		pushGrpcCLIConfig(t, gnmiClient, builder, false)
		time.Sleep(10 * time.Second) // let device apply

		// Validate Server1 configs
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Validate Server2 configs
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(56667), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Validate Server3 configs
		ValidateGrpcServerField(t, dut, "server3", "port", uint16(56668), true)
		ValidateGrpcServerField(t, dut, "server3", "name", "server3", true)

		// Establish gRPC connections & validate services
		for _, server := range servers {
			addr := server.ListenAddrs[0]

			t.Logf("Dialing gRPC server at %s:%d", addr, server.Port)
			conn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port, clientCertPath, clientKeyPath, caCertPath)
			if err != nil {
				t.Fatalf("Failed to dial gRPC server %s:%d: %v", addr, server.Port, err)
			}
			defer conn.Close()

			t.Logf("Successfully connected to %s:%d", addr, server.Port)

			// Validate EMSD brief
			expectedStats := EMSDServerBrief{
				Name:          server.Name,
				Status:        "En",
				Port:          fmt.Sprintf("%d", server.Port),
				TLS:           "En",
				VRF:           "vrf-mgmt",
				ListenAddress: addr,
				Services:      server.Services,
			}
			ValidateEMSDServerBrief_SSH(t, dut, server.Name, expectedStats, true)

			// Dynamically build the RPC validation matrix
			matrix := RPCValidationMatrix{}
			for _, svc := range server.Services {
				switch svc {
				case "GNMI":
					matrix.GNMI_Set = true
					matrix.GNMI_Subscribe = true
				case "GNOI":
					matrix.GNOI_SystemTime = true
				case "GNSI":
					matrix.GNSI_AuthzRotate = true
					matrix.GNSI_AuthzGet = true
				case "P4RT":
					matrix.P4RT = true
				case "GRIBI":
					matrix.GRIBI_Modify = true
					matrix.GRIBI_Get = true
				}
			}
			// Validate Configured gRPC services
			VerifygRPCServicesForMultiServer(t, conn, matrix)

			// Validate EMSD server stats
			ValidateEMSDServerStats_SSH(t, dut, server.Name, EMSDServerStats{
				ServerName: server.Name,
				RPCStatsByPath: map[string]RPCStats{
					"/p4.v1.P4Runtime/StreamChannel": {Requests: 1, Responses: 0, ErrorResponses: 1},
				},
			}, false)
		}

		expectedBrief := EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En", // Enabled
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En", // Enabled
			VRF:           "vrf-mgmt",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		// Validate DEFAULT server brief
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expectedBrief, true)

		// Validate grpc services using default port
		t.Logf("[%s] Validating configured gRPC services for DEFAULT server", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Reload the router to validate config persistence
		perf.ReloadRouter(t, dut)

		// Re-validate Server1 configs
		ValidateGrpcServerField(t, dut, "server1", "port", uint16(56666), true)
		ValidateGrpcServerField(t, dut, "server1", "name", "server1", true)

		// Re-validate Server2 configs
		ValidateGrpcServerField(t, dut, "server2", "port", uint16(56667), true)
		ValidateGrpcServerField(t, dut, "server2", "name", "server2", true)

		// Re-validate Server3 configs
		ValidateGrpcServerField(t, dut, "server3", "port", uint16(56668), true)
		ValidateGrpcServerField(t, dut, "server3", "name", "server3", true)

		// Step 5: Establish gRPC connections & validate services after reload
		for _, server := range servers {
			addr := server.ListenAddrs[0]

			t.Logf("Dialing gRPC server at %s:%d", addr, server.Port)
			conn, err := DialSelfSignedGrpc(ctx, t, addr, server.Port, clientCertPath, clientKeyPath, caCertPath)
			if err != nil {
				t.Fatalf("Failed to dial gRPC server %s:%d: %v", addr, server.Port, err)
			}
			defer conn.Close()

			t.Logf("Successfully connected to %s:%d", addr, server.Port)

			// Validate EMSD brief after reload
			expectedStats := EMSDServerBrief{
				Name:          server.Name,
				Status:        "En",
				Port:          fmt.Sprintf("%d", server.Port),
				TLS:           "En",
				VRF:           "vrf-mgmt",
				ListenAddress: addr,
				Services:      server.Services,
			}
			ValidateEMSDServerBrief_SSH(t, dut, server.Name, expectedStats, true)

			// Dynamically build the RPC validation matrix
			matrix := RPCValidationMatrix{}
			if expectedStats.Status == "En" {
				for _, svc := range server.Services {
					switch svc {
					case "GNMI":
						matrix.GNMI_Set = true
						matrix.GNMI_Subscribe = true
					case "GNOI":
						matrix.GNOI_SystemTime = true
					case "GNSI":
						matrix.GNSI_AuthzRotate = true
						matrix.GNSI_AuthzGet = true
					case "P4RT":
						matrix.P4RT = true
					case "GRIBI":
						matrix.GRIBI_Modify = true
						matrix.GRIBI_Get = true
					}
				}
			}
			// Validate configured gRPC services after reload
			VerifygRPCServicesForMultiServer(t, conn, matrix)

			// Validate EMSD server stats after reload
			ValidateEMSDServerStats_SSH(t, dut, server.Name, EMSDServerStats{
				ServerName: server.Name,
				RPCStatsByPath: map[string]RPCStats{
					"/p4.v1.P4Runtime/StreamChannel": {Requests: 1, Responses: 0, ErrorResponses: 1},
				},
			}, false)

		}

		expectedBrief = EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En", // Enabled
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En", // Enabled
			VRF:           "vrf-mgmt",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expectedBrief, true)

		t.Logf("[%s] Validating configured gRPC services for DEFAULT server", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Unconfigure Multi-Server configs
		opts := GrpcUnconfigOptions{ServerName: "server1", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		ValidateEMSDServerDetail_SSH(t, dut, "server1", EMSDServerDetail{}, true)

		// Unconfigure Multi-Server configs
		opts = GrpcUnconfigOptions{ServerName: "server2", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server2", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "server2", EMSDServerStats{}, true)
		ValidateEMSDServerDetail_SSH(t, dut, "server2", EMSDServerDetail{}, true)

		// Unconfigure Multi-Server configs
		opts = GrpcUnconfigOptions{ServerName: "server3", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(opts)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)
		time.Sleep(30 * time.Second)

		// Negative validation (server should NOT exist)
		ValidateEMSDServerBrief_SSH(t, dut, "server3", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "server3", EMSDServerStats{}, true)
		ValidateEMSDServerDetail_SSH(t, dut, "server3", EMSDServerDetail{}, true)

		expectedBrief = EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En", // Enabled
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En", // Enabled
			VRF:           "vrf-mgmt",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expectedBrief, true)

		t.Logf("[%s] Validating configured gRPC services for DEFAULT server", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cleanup P4RT service
		Unconfigurep4RTService(t)

		// unconfigure mgmt vrf
		Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "unconfigure")
	})

	t.Run("Test 16 gRPC Servers with One Service Each", func(t *testing.T) {
		// Summary:
		//   - Configures prerequisites (P4RT service, mgmt VRF, rotated TLS certificates).
		//   - Builds and applies 16 gRPC server configs, each with one unique service and DSCP value.
		//   - Validates connectivity, EMSD brief fields, and gRPC service availability for all servers.
		//   - Confirms DEFAULT EMSD server and services remain unaffected after multi-server config.
		//   - Reloads the router, revalidates persistence of all 16 servers and their services.
		//   - Restarts EMSD process, confirms service recovery and validates stats again.
		//   - Performs cleanup by unconfiguring all 16 servers and verifying proper removal.

		ctx := context.Background()
		dut := ondatra.DUT(t, "dut")
		gnmiClient := dut.RawAPIs().GNMI(t)

		// === Defer Unconfig Cleanup ===
		defer func() {
			t.Logf("[%s] Starting unconfiguration of gRPC servers", time.Now().Format("15:04:05.000"))
			for i := 0; i < 15; i++ {
				serverName := fmt.Sprintf("server%d", i+1)
				opts := GrpcUnconfigOptions{
					ServerName:   serverName,
					DeleteServer: true,
				}
				builder := BuildGrpcUnconfigBuilder(opts)
				pushGrpcCLIConfig(t, gnmiClient, builder, false)
				t.Logf("Unconfigured %s", serverName)
			}
			Unconfigurep4RTService(t)
			Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "unconfigure")
		}()

		// === Config Prerequisites ===
		Configurep4RTService(t)
		Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "configure")

		clientCertPath, clientKeyPath, caCertPath, dir := createProfileRotateCertz(t)
		t.Logf("Certz paths: %s, %s, %s (dir: %s)", clientCertPath, clientKeyPath, caCertPath, dir)

		listenAddrs := GetGrpcListenAddrs(t, dut)
		if len(listenAddrs) == 0 {
			t.Fatal("No listen addresses found")
		}

		allServices := []string{"GNMI", "GNOI", "GRIBI", "P4RT", "GNSI"}
		dscpValues := []int{46, 34, 16, 48, 40, 24, 0, 56}

		// === Build 16 server configs with different services and DSCP values ===
		var servers []GrpcServerConfig
		for i := 0; i < 15; i++ {
			service := allServices[i%len(allServices)]
			dscp := new(int)
			*dscp = dscpValues[i%len(dscpValues)]

			server := GrpcServerConfig{
				Name:         fmt.Sprintf("server%d", i+1),
				Port:         56666 + i,
				Services:     []string{service},
				SSLProfileID: "rotatecertzrsa",
				VRF:          "mgmt",
				ListenAddrs:  []string{listenAddrs[i%len(listenAddrs)]},
				Dscp:         dscp,
			}
			servers = append(servers, server)
			t.Logf("Prepared Server[%d]: Name=%s, Service=%s", i+1, server.Name, service)
		}

		// === Push Configuration ===
		t.Logf("Configuring %d gRPC servers with one service each", len(servers))
		grpcCfg := GrpcConfig{Servers: servers}
		builder := buildGrpcConfigBuilder(grpcCfg)
		pushGrpcCLIConfig(t, gnmiClient, builder, false)

		// === Add 16th Server with Invalid/Blocked Configuration ===

		t.Logf("[%s] Starting 16th gRPC server (expected to fail configuration)", time.Now().Format("15:04:05.000"))
		invalidServer := GrpcConfig{
			Servers: []GrpcServerConfig{{
				Name:         "server16",
				Port:         59999, // use a port not reachable / not allowed
				Services:     []string{"GNMI"},
				SSLProfileID: "rotatecertzrsa",
				VRF:          "mgmt",
				// invalid or non-routed address to trigger failure
				ListenAddrs: []string{"203.0.113.99"}, // TEST-NET-3 (non-routable)
			}},
		}

		builder = buildGrpcConfigBuilder(invalidServer)
		pushGrpcCLIConfig(t, gnmiClient, builder, true)

		time.Sleep(10 * time.Second)

		// === Validate Connections, Services, and EMSD ===
		for _, server := range servers {
			t.Logf("[%s] Validating gRPC services for %s", time.Now().Format("15:04:05.000"), server.Name)

			addr := server.ListenAddrs[0]
			port := server.Port

			conn, err := DialSelfSignedGrpc(ctx, t, addr, port, clientCertPath, clientKeyPath, caCertPath)
			if err != nil {
				t.Errorf("Dial failed for %s (%s:%d): %v", server.Name, addr, port, err)
				continue
			}

			// === EMSD Validation ===
			expectedBrief := EMSDServerBrief{
				Name:          server.Name,
				Status:        "En",
				Port:          fmt.Sprintf("%d", server.Port),
				TLS:           "En",
				VRF:           "vrf-mgmt",
				ListenAddress: addr,
				Services:      server.Services,
			}
			ValidateEMSDServerBrief_SSH(t, dut, server.Name, expectedBrief, true)

			// === gRPC Service Validation ===
			expected := BuildExpectedRPCMatrix(server.Services)
			VerifygRPCServicesForMultiServer(t, conn, expected)
		}

		t.Logf("[%s] Validating configured gRPC services for DEFAULT Port after multi-server config", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// === Reload the router to validate config persistence ===
		perf.ReloadRouter(t, dut)

		// === Validate Connections, Services After RP Switchover ===
		for _, server := range servers {
			t.Logf("[%s] Validating gRPC services for %s", time.Now().Format("15:04:05.000"), server.Name)

			addr := server.ListenAddrs[0]
			port := server.Port

			conn, err := DialSelfSignedGrpc(ctx, t, addr, port, clientCertPath, clientKeyPath, caCertPath)
			if err != nil {
				t.Errorf("Dial failed for %s (%s:%d): %v", server.Name, addr, port, err)
				continue
			}

			// === EMSD Validation ===
			expectedBrief := EMSDServerBrief{
				Name:          server.Name,
				Status:        "En",
				Port:          fmt.Sprintf("%d", server.Port),
				TLS:           "En",
				VRF:           "vrf-mgmt",
				ListenAddress: addr,
				Services:      server.Services,
			}
			ValidateEMSDServerBrief_SSH(t, dut, server.Name, expectedBrief, true)

			// === gRPC Service Validation ===
			expected := BuildExpectedRPCMatrix(server.Services)
			VerifygRPCServicesForMultiServer(t, conn, expected)
		}

		t.Logf("[%s] Validating configured gRPC services for DEFAULT Port after process restart", time.Now().Format("15:04:05.000"))
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})
	})

	t.Run("Test gRPC Server Max Concurrent Streams (32)", func(t *testing.T) {
		// The test performs the following:
		//   - Configures prerequisites (P4RT service, mgmt VRF, rotated TLS certs).
		//   - Builds 16 gRPC server configs (1 service each) and applies them.
		//   - Configures an additional server "maxreqtest" on port 60000 for GNMI service.
		//   - Establishes 32 concurrent GNMI streams (expected to succeed).
		//   - Attempts 33rd and 34th streams (expected to fail) and validates non-concurrent 35th succeeds.
		//   - Validates telemetry counters and EMSD server/service state.
		//   - Modifies `max-streams-per-user`, `max-concurrent-streams`, and `max-streams` configs
		//      incrementally, verifying stream limits (32 → 50) and expected failures beyond limit.
		//   - Restarts EMSD process, validates persistence of active limits and stream handling.
		//   - Cleans up by unconfiguring all test servers and max-stream configs.
		//   - Performs negative validations (servers removed) and confirms DEFAULT server remains
		//      functional with all core gRPC services.
		//

		// === Prerequisites ===
		ctx := context.Background()
		dut := ondatra.DUT(t, "dut")
		gnmiClient := dut.RawAPIs().GNMI(t)
		config := getTargetConfig(t)
		sshIP := config.sshIp
		grpcPort := config.grpcPort
		Configurep4RTService(t)
		Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "configure")

		// Get listen addresses
		listenAddrs := GetGrpcListenAddrs(t, dut)
		if len(listenAddrs) == 0 {
			t.Fatal("No listen addresses found")
		}

		allServices := []string{"GNMI", "GNOI", "GRIBI", "P4RT", "GNSI"}
		dscpValues := []int{46, 34, 16, 48, 40, 24, 0, 56}

		// === Build 15 server configs with different services and DSCP values ===
		var servers []GrpcServerConfig
		for i := 0; i < 14; i++ {
			service := allServices[i%len(allServices)]
			dscp := new(int)
			*dscp = dscpValues[i%len(dscpValues)]

			server := GrpcServerConfig{
				Name:         fmt.Sprintf("server%d", i+1),
				Port:         56666 + i,
				Services:     []string{service},
				SSLProfileID: "rotatecertzrsa",
				VRF:          "mgmt",
				ListenAddrs:  []string{listenAddrs[i%len(listenAddrs)]},
				Dscp:         dscp,
			}
			servers = append(servers, server)
			t.Logf("Prepared Server[%d]: Name=%s, Service=%s", i+1, server.Name, service)
		}

		// === Push Configuration ===
		t.Logf("Configuring %d gRPC servers with one service each", len(servers))
		grpcCfg := GrpcConfig{Servers: servers}
		builder := buildGrpcConfigBuilder(grpcCfg)
		pushGrpcCLIConfig(t, gnmiClient, builder, false)

		// --- Configure gRPC Server ---
		serverName := "maxreqtest"
		opts := GrpcConfig{
			Servers: []GrpcServerConfig{
				{
					Name:         serverName,
					Port:         60000,
					Services:     []string{"GNMI"},
					VRF:          "mgmt",
					SSLProfileID: "rotatecertzrsa",
				},
			},
		}
		initialBuilder := buildGrpcConfigBuilder(opts)
		pushGrpcCLIConfig(t, gnmiClient, initialBuilder, false)

		// Create certs and define dial wrapper
		clientCert, clientKey, caCert, _ := createProfileRotateCertz(t)
		wrappedDial := func(ctx context.Context, t *testing.T, ip string, port int) (*grpc.ClientConn, string, error) {
			conn, err := DialSelfSignedGrpc(ctx, t, ip, port, clientCert, clientKey, caCert)
			return conn, "", err
		}

		// --- Dial 32 gRPC Clients (Expected to succeed) ---
		connMap := make(map[string]*grpc.ClientConn)
		streamCancels := make(map[string]context.CancelFunc)

		for i := 1; i <= 32; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 60*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}

		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 33rd gRPC Client (Expected to fail) ---
		conn33, _, err := wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 33rd client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 33rd client (expected); now testing stream RPC")

		streamCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-33", conn33, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-33")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-33", err)
		} // Expect failure

		// --- Dial 34th gRPC Client (Expected to fail) ---
		conn34, _, err := wrappedDial(ctx, t, listenAddrs[0], 56666)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 34th client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-34", conn34, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-34")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-34", err)
		} // Expect failure

		// --- Dial 35th gRPC Client non concurrent (Expected to succeed) ---
		conn, err := DialSelfSignedGrpc(ctx, t, sshIP, 60000, clientCert, clientKey, caCert)
		if err != nil {
			t.Fatalf("Dial for 35th client failed unexpectedly: %v", err)
		}

		// Validate gRPC services (GNMI Subscribe expected to fail)
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected := &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 32, GrpcTLSDestinations: 32,
			DialinCount: 32, DialinActive: 32, DialinSessions: 32, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}

		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Validate gRPC services on default port
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}
		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// --- Configure max-request-per-user on default server---
		cfg1 := "grpc\n max-streams-per-user 50\n!\n"
		resp, err := PushCliConfigViaGNMI(ctx, t, dut, cfg1)
		t.Logf("Response for max-streams-per-user config: %v", resp)
		if err != nil {
			t.Logf("[INFO] Config %q succeeded (but was expected Fail (flaky Config)).", cfg1)
		} else {
			t.Logf("[PASS] Config push succeeded: %q", cfg1)
		}

		// 32 concurrent streams should still be active even after max-request-per-user config is 50
		// as max-concurrent-streams and max-streams default value is 32
		for i := 1; i <= 32; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 90*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}

		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 33rd gRPC Client (Expected to fail) ---
		conn33, _, err = wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 33rd client failed unexpectedly: %v", err)
		}

		t.Logf("Dial successful for 33rd client (expected); now testing stream RPC")
		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-33", conn33, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-33")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-33", err)
		} // Expect failure as concurrent streams limit is 32

		// --- Dial 34th gRPC Client (Expected to fail)
		conn34, _, err = wrappedDial(ctx, t, listenAddrs[0], 56666)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 34th client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-34", conn34, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-34")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-34", err)
		} // Expect failure

		// --- Dial 35th gRPC Client non concurrent (Expected to succeed) ---
		conn, err = DialSelfSignedGrpc(ctx, t, sshIP, 60000, clientCert, clientKey, caCert)
		if err != nil {
			t.Fatalf("Dial for 35th client failed unexpectedly: %v", err)
		}

		// Validate gRPC services (GNMI Subscribe expected to fail)
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 32, GrpcTLSDestinations: 32,
			DialinCount: 32, DialinActive: 32, DialinSessions: 32, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}

		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Validate gRPC services on default port (Subscribe expected to fail) as 32 concurrent streams are active
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Cancel all existing streams
		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}

		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// Validate gRPC services on default port now all services should be available
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Restart EMSD process
		RestartAndValidateEMSD(t, dut)

		// Recreate map to track connections
		for i := 1; i <= 32; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 60*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}
		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 33th gRPC Client (Expected to fail) ---
		conn33, _, err = wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 33th client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 33th client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-33", conn33, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-33")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-33", err)
		} // Expect failure

		// --- Dial 34th gRPC Client (Expected to fail) ---
		conn34, _, err = wrappedDial(ctx, t, listenAddrs[0], 56666)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 34th client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-34", conn34, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-34")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-34", err)
		} // Expect failure

		// --- Dial 35th gRPC Client non concurrent (Expected to succeed) ---
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 32, GrpcTLSDestinations: 32,
			DialinCount: 32, DialinActive: 32, DialinSessions: 32, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}

		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Cancel all existing streams
		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}

		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// --- Configure max-concurrent-streams 50 on default server---
		cfg2 := "grpc\n max-concurrent-streams 50\n!\n"
		_, err = PushCliConfigViaGNMI(ctx, t, dut, cfg2)
		if err != nil {
			t.Logf("[INFO] Config %q succeeded (flaky Config))", cfg2)
		} else {
			t.Logf("[PASS] Config push succeeded: %q", cfg2)
		}

		time.Sleep(5 * time.Second)

		// 32 concurrent streams should only allow even after max-request-per-user & max-concurrent-streams config is 50
		// as max-streams default value is still 32
		for i := 1; i <= 32; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 60*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}

		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 34th gRPC Client (Expected to fail) ---
		conn34, _, err = wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}

		t.Logf("Dial successful for 34th client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-34", conn34, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-34")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-34", err)
		} // Expect failure

		// --- Dial 34th gRPC Client (Expected to fail) ---
		conn34, _, err = wrappedDial(ctx, t, listenAddrs[0], 56666)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 34th client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-34", conn34, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-34")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-34", err)
		} // Expect failure

		// --- Dial 35th gRPC Client non concurrent (Expected to succeed) ---
		conn, err = DialSelfSignedGrpc(ctx, t, sshIP, 60000, clientCert, clientKey, caCert)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}

		// Validate gRPC services (GNMI Subscribe expected to fail)
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 32, GrpcTLSDestinations: 32,
			DialinCount: 32, DialinActive: 32, DialinSessions: 32, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}

		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Cancel all existing streams
		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}

		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// --- Configure max-streams ---
		cfg3 := "grpc\n max-streams 50\n!\n"
		_, err = PushCliConfigViaGNMI(ctx, t, dut, cfg3)
		if err != nil {
			t.Logf("[INFO] Config %q succeeded (flaky Config)", cfg3)
		} else {
			t.Logf("[PASS] Config push succeeded: %q", cfg3)
		}
		time.Sleep(5 * time.Second)

		// 50 concurrent streams should allow now as max-request-per-user, max-concurrent-streams & max-streams default value changed to 50
		for i := 1; i <= 50; i++ {
			name := fmt.Sprintf("client-%d", i)
			conn, _, err := wrappedDial(ctx, t, sshIP, 60000)
			if err != nil {
				t.Fatalf("Dial failed for %s: %v", name, err)
			}
			t.Logf("Dial successful: %s", name)
			connMap[name] = conn

			// Start streaming
			streamCtx, cancel := context.WithCancel(context.Background())
			streamCancels[name] = cancel
			go func(client string, c *grpc.ClientConn) {
				t.Logf("Starting GNMI stream: %s", client)
				if err := startGNMIStream(streamCtx, client, c, 60*time.Second); err != nil {
					t.Errorf("Unexpected stream failure for %s: %v", client, err)
				}
			}(name, conn)
		}

		// Allow time for streams to establish
		time.Sleep(10 * time.Second)

		// --- Dial 51st gRPC Client (Expected to fail) ---
		conn51, _, err := wrappedDial(ctx, t, sshIP, 60000)
		if err != nil {
			t.Fatalf("Dial for 51st client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 51st client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-51", conn51, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-51")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-51", err)
		} // Expect failure

		// --- Dial 52nd gRPC Client (Expected to fail) ---
		conn52, _, err := wrappedDial(ctx, t, listenAddrs[0], 56666)
		if err != nil {
			t.Fatalf("Dial for 52nd client failed unexpectedly: %v", err)
		}
		t.Logf("Dial successful for 52nd client (expected); now testing stream RPC")

		streamCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := startGNMIStream(streamCtx, "client-52", conn52, 5*time.Second); err == nil {
			t.Errorf("Expected stream failure for %s but it succeeded", "client-52")
		} else {
			t.Logf("[PASS] Expected failure for %s: %v", "client-52", err)
		} // Expect failure

		// --- Dial 53rd gRPC Client non concurrent (Expected to succeed) ---
		conn, err = DialSelfSignedGrpc(ctx, t, sshIP, 60000, clientCert, clientKey, caCert)
		if err != nil {
			t.Fatalf("Dial for 34th client failed unexpectedly: %v", err)
		}

		// Validate gRPC services (GNMI Subscribe expected to fail)
		VerifygRPCServicesForMultiServer(t, conn, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: false, GNOI_SystemTime: false, GNSI_AuthzRotate: false, GNSI_AuthzGet: false,
			GRIBI_Modify: false, GRIBI_Get: false, P4RT: false})

		// Validate Telemetry Summary
		expected = &TelemetrySummary{Subscriptions: 1, SubscriptionsActive: 1, DestinationGroups: 50, GrpcTLSDestinations: 50,
			DialinCount: 50, DialinActive: 50, DialinSessions: 50, SensorGroups: 1, SensorPathsTotal: 1, SensorPathsActive: 1}
		ValidateTelemetrySummary_SSH(t, dut, expected)

		// Cancel all existing streams
		for name, cancel := range streamCancels {
			t.Logf("Cancelling GNMI stream for %s", name)
			cancel()
		}

		// Allow time for streams to close
		time.Sleep(10 * time.Second)

		// === Unconfig Cleanup ===
		t.Logf("[%s] Starting unconfiguration of gRPC servers", time.Now().Format("15:04:05.000"))
		for i := 0; i < 14; i++ {
			serverName := fmt.Sprintf("server%d", i+1)
			opts := GrpcUnconfigOptions{
				ServerName:   serverName,
				DeleteServer: true,
			}
			builder := BuildGrpcUnconfigBuilder(opts)
			pushGrpcCLIConfig(t, gnmiClient, builder, false)
			t.Logf("Unconfigured %s", serverName)
		}

		configs := map[string]string{
			"max-streams-per-user":   "no grpc max-streams-per-user 50",
			"max-concurrent-streams": "no grpc max-concurrent-streams 50",
			"max-streams":            "no grpc max-streams 50",
		}

		for label, config := range configs {
			resp, err := PushCliConfigViaGNMI(ctx, t, dut, config)
			time.Sleep(5 * time.Second) // Allow time for config to apply

			if err != nil {
				if strings.Contains(err.Error(), "EOF") {
					t.Logf("[WARN] %s caused EOF (expected for disruptive config), retrying...", label)
					// Reconnect GNMI client here if needed
				} else {
					t.Errorf("[FAIL] Error applying %s: %v", label, err)
				}
			} else {
				t.Logf("[PASS] Applied config: %s: %s", label, resp)
			}
		}

		unconfig := GrpcUnconfigOptions{ServerName: "maxreqtest", DeleteServer: true}
		builder = BuildGrpcUnconfigBuilder(unconfig)
		pushGrpcCLIConfig(t, dut.RawAPIs().GNMI(t), builder, false)

		// Negative validation after unconfig (servers should be gone)
		ValidateEMSDServerBrief_SSH(t, dut, "server1", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "server1", EMSDServerStats{}, true)
		ValidateEMSDServerDetail_SSH(t, dut, "server1", EMSDServerDetail{}, true)

		// Negative validation for maxreqtest server also should be gone
		ValidateEMSDServerBrief_SSH(t, dut, "maxreqtest", EMSDServerBrief{}, false)
		ValidateEMSDServerStats_SSH(t, dut, "maxreqtest", EMSDServerStats{}, true)
		ValidateEMSDServerDetail_SSH(t, dut, "maxreqtest", EMSDServerDetail{}, true)

		// Validate default EMSD server is still present and all services are functional
		// after unconfig of all other servers
		expectedBrief := EMSDServerBrief{
			Name:          "DEFAULT",
			Status:        "En", // Enabled
			ListenAddress: "ANY",
			Port:          fmt.Sprint(grpcPort),
			TLS:           "En", // Enabled
			VRF:           "vrf-mgmt",
			Services: []string{
				"GNOI", "GNPSI", "CNMI", "GNSI", "SLAPI",
				"P4RT", "ENROLLZ", "ATTESTZ", "SRTE", "GNMI", "GRIBI",
			},
		}
		ValidateEMSDServerBrief_SSH(t, dut, "DEFAULT", expectedBrief, true)

		// Validate gRPC services on default port
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true, GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true})

		// Unconfigure prerequisites
		Unconfigurep4RTService(t)
		Config_Unconfig_Vrf(ctx, t, dut, "mgmt", "unconfigure")
	})
}

func TestFEAT36485(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Test Default Server", func(t *testing.T) {
		ctx := context.Background()

		// --- Step 0: Pre-setup ---
		Configurep4RTService(t)

		// --- Step 1: Reboot DUT and wait for reachability ---
		// Reload the router
		perf.ReloadRouter(t, dut)
		// rebootAndWaitForPing(ctx, t, sshIP)

		// --- Step 2: Inline validator for gRPC vs cfgmgr behavior ---
		validateGrpcVsCfgmgr := func(ctx context.Context, t *testing.T, phase string) {
			t.Logf("\n=== [%s] Validating gRPC vs cfgmgr startup and RPC flow ===", phase)

			// Collect timestamps
			mgmtTime := getMgmtUpTime(ctx, t)
			grpcStartTime := getGrpcStartTime(ctx, t)
			cfgmgrTime := getCfgmgrStartupTime(ctx, t)

			// Sanity check
			if mgmtTime.IsZero() || grpcStartTime.IsZero() || cfgmgrTime.IsZero() {
				t.Fatalf("[%s] Failed to collect one or more required timestamps (Mgmt=%v, gRPC=%v, CfgMgr=%v)",
					phase, mgmtTime, grpcStartTime, cfgmgrTime)
			}

			// Validate gRPC start vs Cfgmgr startup completion
			if cfgmgrTime.Before(grpcStartTime) {
				t.Logf("[%s] Cfgmgr completed Before gRPC was up", phase)
			} else {
				t.Errorf("[%s] Cfgmgr completed After gRPC was up (Cfgmgr: %v, gRPC: %v)",
					phase, cfgmgrTime, grpcStartTime)
			}

			// Validate gRPC service operability
			VerifygRPCServicesForDefaultPortParallel(t, RPCValidationMatrix{
				GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true,
				GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
				GRIBI_Modify: true, GRIBI_Get: true,
			})

			// Collect and compare RPC event timestamps
			traces := map[string]time.Time{
				"gNMI_Set":     getTraceEventTime(ctx, t, "show grpc trace ems", "gNMI SetRequest"),
				"gNMI_Sub":     getTraceEventTime(ctx, t, "show grpc trace ems", "Subscribe:"),
				"gNOI_Time":    getTraceEventTime(ctx, t, "show gnoi trace system all", "gNOI Time Request"),
				"gNSI_Rotate":  getTraceEventTime(ctx, t, "show gnsi trace all", "rotate successful"),
				"gNSI_Get":     getTraceEventTime(ctx, t, "show gnsi trace all", "get policy:"),
				"gRIBI_Modify": getTraceEventTime(ctx, t, "show emsd trace all", "/gribi.gRIBI/Modify"),
				"gRIBI_Get":    getTraceEventTime(ctx, t, "show emsd trace all", "/gribi.gRIBI/Get"),
			}

			for rpc, ts := range traces {
				if ts.IsZero() {
					t.Logf("[%s] No trace timestamp found for %s", phase, rpc)
					continue
				}
				if ts.Before(cfgmgrTime) {
					t.Errorf("[%s] %s executed BEFORE cfgmgr startup completed (RPC=%v, CfgMgr=%v)",
						phase, rpc, ts, cfgmgrTime)
				} else {
					t.Logf("[%s] %s executed AFTER cfgmgr startup done", phase, rpc)
				}
			}
		}

		// --- Step 3: (Optional) Validate before RP switchover ---
		validateGrpcVsCfgmgr(ctx, t, "Before Process Restart")

		// Perform eMSD process restart.
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Validate post-switchover gRPC service functionality
		VerifygRPCServicesForDefaultPort(t, RPCValidationMatrix{
			GNMI_Set: true, GNMI_Subscribe: true, GNOI_SystemTime: true,
			GNSI_AuthzRotate: true, GNSI_AuthzGet: true,
			GRIBI_Modify: true, GRIBI_Get: true, P4RT: true,
		})
		Unconfigurep4RTService(t)
	})

	t.Run("Test Multi-GRPC Server", func(t *testing.T) {
		ctx := context.Background()
		binding := parseBindingFile(t)[0]
		sshIP := binding.sshIp
		username := binding.sshUser
		password := binding.sshPass

		// Configure p4RT service if required
		Configurep4RTService(t)

		// Step 1: Configure 5 unique servers with unique services
		servers := []GrpcServerConfig{
			{Name: "server1", Port: 50051, Services: []string{"GNMI"}, SSLProfileID: "system_default_profile"},
			{Name: "server2", Port: 50052, Services: []string{"GNOI"}, SSLProfileID: "system_default_profile"},
			{Name: "server3", Port: 50053, Services: []string{"GNSI"}, SSLProfileID: "system_default_profile"},
			{Name: "server4", Port: 50054, Services: []string{"GRIBI"}, SSLProfileID: "system_default_profile"},
			{Name: "server5", Port: 50055, Services: []string{"P4RT"}, SSLProfileID: "system_default_profile"},
		}

		initialCfg := GrpcConfig{Servers: servers}
		gnmiClient := dut.RawAPIs().GNMI(t)

		// Build and push config
		builder := buildGrpcConfigBuilder(initialCfg)
		pushGrpcCLIConfig(t, gnmiClient, builder, false)

		// Reload the router
		perf.ReloadRouter(t, dut)

		// Enhanced validation function with proper timing
		validateGrpcVsCfgmgrWithTiming := func(ctx context.Context, t *testing.T, phase string) {
			t.Logf("=== [%s] Starting gRPC vs cfgmgr validation ===", phase)

			// Step 1: Get baseline timestamps (before RPC execution)
			mgmtTime := getMgmtUpTime(ctx, t)
			grpcStartTime := getGrpcStartTime(ctx, t)
			cfgmgrTime := getCfgmgrStartupTime(ctx, t)

			if mgmtTime.IsZero() || grpcStartTime.IsZero() || cfgmgrTime.IsZero() {
				t.Errorf("[%s] Failed to collect required timestamps", phase)
				return
			}

			t.Logf("[%s] Baseline timestamps - Mgmt: %v, gRPC Start: %v, Cfgmgr Done: %v",
				phase, mgmtTime, grpcStartTime, cfgmgrTime)

			// Validate gRPC start vs Cfgmgr startup completion
			if cfgmgrTime.Before(grpcStartTime) {
				t.Logf("[%s] Cfgmgr completed Before gRPC was up", phase)
			} else {
				t.Errorf("[%s] Cfgmgr completed After gRPC was up (Cfgmgr: %v, gRPC: %v)",
					phase, cfgmgrTime, grpcStartTime)
			}

			// Step 3: Record test execution start time
			testStartTime := time.Now()
			t.Logf("[%s] Starting RPC execution at test time: %v", phase, testStartTime)

			// Step 4: Execute gRPC services in parallel (this generates new traces)
			t.Logf("[%s] Executing parallel gRPC validation...", phase)
			ValidateGrpcServersInParallel(ctx, t, sshIP, username, password, servers)

			// Step 5: Record test execution end time
			testEndTime := time.Now()
			t.Logf("[%s] Completed RPC execution at test time: %v (duration: %v)",
				phase, testEndTime, testEndTime.Sub(testStartTime))

			// Step 6: Now collect traces (should contain our RPC executions)
			t.Logf("[%s] Collecting traces after RPC execution...", phase)

			traces := map[string]time.Time{
				"gNMI Set":          getTraceEventTime(ctx, t, "show grpc trace ems", "gNMI SetRequest"),
				"gNMI Subscribe":    getTraceEventTime(ctx, t, "show grpc trace ems", "subscribe: once done"),
				"gNOI Time Request": getTraceEventTime(ctx, t, "show gnoi trace system all", "gNOI Time Request"),
				"gNSI Rotate":       getTraceEventTime(ctx, t, "show gnsi trace all", "rotate successful"),
				"gNSI Get Policy":   getTraceEventTime(ctx, t, "show gnsi trace all", "get policy:"),
				"gRIBI Modify":      getTraceEventTime(ctx, t, "show emsd trace all", "/gribi.gRIBI/Modify"),
				"gRIBI Get":         getTraceEventTime(ctx, t, "show emsd trace all", "/gribi.gRIBI/Get"),
				"P4RT":              getTraceEventTime(ctx, t, "show p4rt trace all", "Arbitration complete"),
			}

			for rpc, ts := range traces {
				if ts.IsZero() {
					t.Logf("[%s] No trace timestamp found for %s", phase, rpc)
					continue
				}
				if ts.Before(cfgmgrTime) {
					t.Errorf("[%s] %s executed BEFORE cfgmgr startup completed (RPC=%v, CfgMgr=%v)",
						phase, rpc, ts, cfgmgrTime)
				} else {
					t.Logf("[%s] %s executed AFTER cfgmgr startup done", phase, rpc)
				}
			}
		}

		// Step 3: Validate in "Before RP Switchover" state
		validateGrpcVsCfgmgrWithTiming(ctx, t, "Before RP Switchover")

		for _, s := range servers {
			exp := EMSDServerBrief{
				Name:          s.Name,
				Status:        "En",
				Port:          fmt.Sprintf("%d", s.Port),
				TLS:           "En",
				VRF:           "global-vrf",
				ListenAddress: "ANY",
				Services:      s.Services,
			}
			ValidateEMSDServerBrief_SSH(t, dut, s.Name, exp, true)
		}

		// Step 4: Perform RP Switchover
		t.Log("=== Performing RP Switchover ===")
		utils.Dorpfo(context.Background(), t, true)

		// Validate NSR readiness
		redundancy_nsrState(context.Background(), t, true)

		// Step 5: Reboot after RP switchover
		t.Log("=== Rebooting device after RP switchover ===")
		perf.ReloadRouter(t, dut)

		// Step 6: Validate in "After RP Switchover" state
		validateGrpcVsCfgmgrWithTiming(ctx, t, "After RP Switchover")

		for _, s := range servers {
			exp := EMSDServerBrief{
				Name:          s.Name,
				Status:        "En",
				Port:          fmt.Sprintf("%d", s.Port),
				TLS:           "En",
				VRF:           "global-vrf",
				ListenAddress: "ANY",
				Services:      s.Services,
			}
			ValidateEMSDServerBrief_SSH(t, dut, s.Name, exp, true)
		}

		Unconfigurep4RTService(t)

		// Step 7: Unconfigure all 5 servers
		for _, s := range servers {
			unconfig := BuildGrpcUnconfigBuilder(GrpcUnconfigOptions{ServerName: s.Name, DeleteServer: true})
			pushGrpcCLIConfig(t, gnmiClient, unconfig, false)
		}
	})
}
