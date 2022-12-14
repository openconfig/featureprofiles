// Copyright 2022 Google LLC
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

package osinstall_test

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	closer "github.com/openconfig/gocloser"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ospb "github.com/openconfig/gnoi/os"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

var packageReader func(context.Context) (io.ReadCloser, error) = func(ctx context.Context) (io.ReadCloser, error) {
	f, err := os.Open(*osFile)
	if err != nil {
		return nil, err
	}
	return f, nil
}

var (
	osFile    = flag.String("osfile", "", "Path to the OS image for the install operation")
	osVersion = flag.String("osver", "", "Version of the OS image for the install operation")

	timeout = flag.Duration("timeout", time.Minute*30, "Time to wait for reboot to complete")
)

type testCase struct {
	dut *ondatra.DUTDevice
	// dualSup indicates if the DUT has a standby supervisor available.
	dualSup bool
	reader  io.ReadCloser

	osc ospb.OSClient
	sc  spb.SystemClient
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOSInstall(t *testing.T) {
	if *osFile == "" || *osVersion == "" {
		t.Fatal("Missing osfile or osver args")
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI().Default(t).OS(),
		sc:     dut.RawAPIs().GNOI().Default(t).System(),
	}
	tc.fetchStandbySupervisorStatus(ctx, t)
	tc.transferOS(ctx, t, false)
	tc.activateOS(ctx, t, false)
	if tc.dualSup {
		tc.transferOS(ctx, t, true)
		tc.activateOS(ctx, t, true)
	}
	tc.rebootDUT(ctx, t)
	tc.verifyInstall(ctx, t)
}

func (tc *testCase) activateOS(ctx context.Context, t *testing.T, standby bool) {
	act, err := tc.osc.Activate(ctx, &ospb.ActivateRequest{
		StandbySupervisor: standby,
		Version:           *osVersion,
		NoReboot:          true,
	})
	if err != nil {
		t.Fatalf("OS.Activate request failed: %s", err)
	}

	switch resp := act.Response.(type) {
	case *ospb.ActivateResponse_ActivateOk:
		if standby {
			t.Log("OS.Activate standby supervisor complete.")
		} else {
			t.Log("OS.Activate complete.")
		}
	case *ospb.ActivateResponse_ActivateError:
		actErr := resp.ActivateError
		t.Fatalf("OS.Activate error %s: %s", actErr.Type, actErr.Detail)
	default:
		t.Fatalf("OS.Activate unexpected response: got %v (%T)", resp, resp)
	}
}

// fetchStandbySupervisorStatus checks if the DUT has a standby supervisor available in a working state.
func (tc *testCase) fetchStandbySupervisorStatus(ctx context.Context, t *testing.T) {
	r, err := tc.osc.Verify(ctx, &ospb.VerifyRequest{})
	if err != nil {
		t.Fatal(err)
	}

	switch v := r.GetVerifyStandby().GetState().(type) {
	case *ospb.VerifyStandby_StandbyState:
		if v.StandbyState.GetState() == ospb.StandbyState_UNAVAILABLE {
			t.Fatal("OS.Verify RPC reports standby supervisor in UNAVAILABLE state.")
		}
		// All other supervisor states indicate this device does not support or have dual supervisors available.
		t.Log("DUT is detected as single supervisor.")
		tc.dualSup = false
	case *ospb.VerifyStandby_VerifyResponse:
		t.Log("DUT is detected as dual supervisor.")
		tc.dualSup = true
	default:
		t.Fatalf("Unexpected OS.Verify Standby State RPC Response: got %v (%T)", v, v)
	}
}

func (tc *testCase) rebootDUT(ctx context.Context, t *testing.T) {
	bootTime := gnmi.Get(t, tc.dut, gnmi.OC().System().BootTime().State())
	deadline := time.Now().Add(*timeout)

	t.Log("Send DUT Reboot Request")
	_, err := tc.sc.Reboot(ctx, &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Force:   true,
		Message: "Apply GNOI OS Software Install",
	})
	if err != nil && status.Code(err) != codes.Unavailable {
		t.Fatalf("System.Reboot request failed: %s", err)
	}
	for {
		var curBootTime *ygnmi.Value[uint64]

		// While the device is rebooting, Lookup will fatal due to unresponsive GNMI service.
		testt.CaptureFatal(t, func(t testing.TB) {
			curBootTime = gnmi.Lookup(t, tc.dut, gnmi.OC().System().BootTime().State())
		})

		if curBootTime != nil {
			val, present := curBootTime.Val()
			if !present {
				t.Log("Reboot time not present")
			}
			if val > bootTime {
				t.Log("Reboot completed.")
				break
			} else if val == bootTime {
				t.Log("DUT has not rebooted.")
			}
		}

		if time.Now().After(deadline) {
			t.Fatal("Past reboot deadline")
		}
		t.Log("Waiting for reboot to complete...")
		time.Sleep(10 * time.Second)
	}
}

func (tc *testCase) transferOS(ctx context.Context, t *testing.T, standby bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ic, err := tc.osc.Install(ctx)
	if err != nil {
		t.Fatalf("OS.Install client request failed: %s", err)
	}

	ireq := &ospb.InstallRequest{
		Request: &ospb.InstallRequest_TransferRequest{
			TransferRequest: &ospb.TransferRequest{
				Version:           *osVersion,
				StandbySupervisor: standby,
			},
		},
	}
	if err = ic.Send(ireq); err != nil {
		t.Fatalf("OS.Install error sending install request: %s", err)
	}

	iresp, err := ic.Recv()
	if err != nil {
		t.Fatalf("OS.Install error receiving: %s", err)
	}
	switch v := iresp.GetResponse().(type) {
	case *ospb.InstallResponse_TransferReady:
	case *ospb.InstallResponse_Validated:
		if standby {
			t.Log("DUT standby supervisor has valid preexisting image; skipping transfer.")
		} else {
			t.Log("DUT supervisor has valid preexisting image; skipping transfer.")
		}
		return
	case *ospb.InstallResponse_SyncProgress:
		if !tc.dualSup {
			t.Fatalf("Unexpected SyncProgress on single supervisor: got %v (%T)", v, v)
		}
		t.Logf("Sync progress: %v%% synced from supervisor", v.SyncProgress.GetPercentageTransferred())
	default:
		t.Fatalf("Expected TransferReady following TransferRequest: got %v (%T)", v, v)
	}

	awaitChan := make(chan error)
	go func() {
		err := watchStatus(t, ic, standby)
		awaitChan <- err
	}()

	if !standby {
		err = transferContent(ic, tc.reader)
		if err != nil {
			t.Fatalf("Error transferring content: %s", err)
		}
	}

	if err = <-awaitChan; err != nil {
		t.Fatalf("Transfer receive error: %s", err)
	}

	if standby {
		t.Log("OS.Install standby supervisor image transfer complete.")
	} else {
		t.Log("OS.Install supervisor image transfer complete.")
	}
}

// verifyInstall validates the OS.Verify RPC returns no failures and version numbers match the
// newly requested software version.
func (tc *testCase) verifyInstall(ctx context.Context, t *testing.T) {
	r, err := tc.osc.Verify(ctx, &ospb.VerifyRequest{})
	if err != nil {
		t.Fatalf("OS.Verify request failed: %s", err)
	}

	if got, want := r.GetActivationFailMessage(), ""; got != want {
		t.Errorf("OS.Verify ActivationFailMessage: got %q, want %q", got, want)
	}
	if got, want := r.GetVersion(), *osVersion; got != want {
		t.Errorf("OS.Verify Version: got %q, want %q", got, want)
	}

	if tc.dualSup {
		if got, want := r.GetVerifyStandby().GetVerifyResponse().GetActivationFailMessage(), ""; got != want {
			t.Errorf("OS.Verify Standby ActivationFailMessage: got %q, want %q", got, want)
		}

		if got, want := r.GetVerifyStandby().GetVerifyResponse().GetVersion(), *osVersion; got != want {
			t.Errorf("OS.Verify Standby Version: got %q, want %q", got, want)
		}
	}

	t.Log("OS.Verify complete")
}

func transferContent(ic ospb.OS_InstallClient, reader io.ReadCloser) error {
	// The gNOI SetPackage operation sets the maximum chunk size at 64K,
	// so assuming the install operation allows for up to the same size.
	buf := make([]byte, 64*1024)
	defer closer.CloseAndLog(reader.Close, "error closing package file")
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			tc := &ospb.InstallRequest{
				Request: &ospb.InstallRequest_TransferContent{
					TransferContent: buf[0:n],
				},
			}
			if err := ic.Send(tc); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	te := &ospb.InstallRequest{
		Request: &ospb.InstallRequest_TransferEnd{
			TransferEnd: &ospb.TransferEnd{},
		},
	}
	return ic.Send(te)
}

func watchStatus(t *testing.T, ic ospb.OS_InstallClient, standby bool) error {
	var gotProgress bool

	for {
		iresp, err := ic.Recv()
		if err != nil {
			return err
		}

		switch v := iresp.GetResponse().(type) {
		case *ospb.InstallResponse_InstallError:
			errName := ospb.InstallError_Type_name[int32(v.InstallError.Type)]
			return fmt.Errorf("installation error %q: %s", errName, v.InstallError.GetDetail())
		case *ospb.InstallResponse_TransferProgress:
			if standby {
				return fmt.Errorf("unexpected TransferProgress: got %v, want SyncProgress", v)
			}
			t.Logf("Transfer progress: %v bytes received by DUT", v.TransferProgress.GetBytesReceived())
			gotProgress = true
		case *ospb.InstallResponse_SyncProgress:
			if !standby {
				return fmt.Errorf("unexpected SyncProgress: got %v, want TransferProgress", v)
			}
			t.Logf("Transfer progress: %v%% synced from supervisor", v.SyncProgress.GetPercentageTransferred())
			gotProgress = true
		case *ospb.InstallResponse_Validated:
			if !gotProgress {
				return fmt.Errorf("transfer completed without progress status")
			}
			if got, want := v.Validated.GetVersion(), *osVersion; got != want {
				return fmt.Errorf("mismatched validation software versions: got %s, want %s", got, want)
			}
			return nil
		default:
			return fmt.Errorf("unexpected client install response: got %v (%T)", v, v)
		}
	}
}
