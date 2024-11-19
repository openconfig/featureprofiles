package osinstall_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	// dbg "github.com/openconfig/featureprofiles/exec/utils/debug"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/deviations"
	ospb "github.com/openconfig/gnoi/os"
	spb "github.com/openconfig/gnoi/system"
	closer "github.com/openconfig/gocloser"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type testCase struct {
	dut *ondatra.DUTDevice
	// dualSup indicates if the DUT has a standby supervisor available.
	dualSup bool
	reader  io.ReadCloser

	osc                    ospb.OSClient
	sc                     spb.SystemClient
	ctx                    context.Context
	noReboot               bool
	forceDownloadSupported bool

	osFile    string
	osVersion string
	timeout   time.Duration
}

var packageReader func(context.Context, string) (io.ReadCloser, error) = func(ctx context.Context, os_file string) (io.ReadCloser, error) {
	f, err := os.Open(os_file)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (tc *testCase) activateOS(ctx context.Context, t *testing.T, standby, noReboot bool, version string, expectFail bool, expectedError string) {
	t.Helper()
	if standby {
		t.Log("OS.Activate is started for standby RP.")
	} else {
		t.Log("OS.Activate is started for active RP.")
	}

	act, err := tc.osc.Activate(ctx, &ospb.ActivateRequest{
		StandbySupervisor: standby,
		// Version:           *osVersion,
		Version:  version,
		NoReboot: noReboot,
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
		v := fmt.Sprintf("%v", resp)
		if !expectFail && !strings.Contains(v, expectedError) {
			t.Fatalf("OS.Activate error want = %s ,got = %s: %s", expectedError, actErr.Type, actErr.Detail)
		}
		t.Logf("OS.Activate error %s: %s", actErr.Type, actErr.Detail)

	default:
		v := fmt.Sprintf("%v", resp)
		if !expectFail && !strings.Contains(v, expectedError) {
			t.Fatalf("OS.Activate error want: %v , got: %v (%T)", expectedError, v, v)
		}
		t.Logf("OS.Activate error want: %v , got: %v (%T)", expectedError, v, v)
	}
}

// fetchStandbySupervisorStatus checks if the DUT has a standby supervisor available in a working state.
func (tc *testCase) fetchStandbySupervisorStatus(t *testing.T) {
	r, err := tc.osc.Verify(tc.ctx, &ospb.VerifyRequest{})
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

func (tc *testCase) fetchOsFileDetails(t *testing.T, os_file string) {

	os_version, err := getIsoVersionInfo(os_file)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		t.Fatalf("Error: %v\n", err)
	}
	tc.osFile = os_file
	tc.osVersion = os_version

}

func (tc *testCase) setTimeout(t *testing.T, timeout_minute time.Duration) {
	t.Logf("testcase %v : setting timout to %v", tc, timeout_minute)
	tc.timeout = timeout_minute
}

func (tc *testCase) updatePackageReader(t *testing.T) {
	// ctx := context.Background()
	reader, err := packageReader(tc.ctx, tc.osFile)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}
	t.Logf("setting reader for testcase %v", tc)
	tc.reader = reader
}

func (tc *testCase) updateForceDownloadSupport(t *testing.T) {
	majVer, _, _, _, err := util.GetVersion(t, tc.dut)
	if err != nil {
		t.Fatal("wrong version format")
	}
	majorVersion, err := strconv.Atoi(majVer)
	if err != nil {
		t.Error("major version is not a number")
		majorVersion = 25
	}
	if majorVersion >= 25 {
		tc.forceDownloadSupported = true
	} else {
		tc.forceDownloadSupported = false
	}
}

func (tc *testCase) rebootDUT(ctx context.Context, t *testing.T) {
	t.Log("Send DUT Reboot Request")
	_, err := tc.sc.Reboot(ctx, &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Force:   true,
		Message: "Apply GNOI OS Software Install",
	})
	if err != nil && status.Code(err) != codes.Unavailable {
		t.Fatalf("System.Reboot request failed: %s", err)
	}
}

func (tc *testCase) transferOS(ctx context.Context, t *testing.T, standby bool, version string, ErrorString string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	tc.updatePackageReader(t)
	ic, err := tc.osc.Install(ctx)
	if err != nil {
		t.Fatalf("OS.Install client request failed: %s", err)
	}
	// versionString := ""
	// if !force {
	// 	versionString = *osVersion
	// }
	// if dummyVersion {
	// 	if !force {
	// 		t.Fatalf("only force install can have dummy version")
	// 	} else {
	// 		versionString = "DUMMY"
	// 	}
	// }

	ireq := &ospb.InstallRequest{
		Request: &ospb.InstallRequest_TransferRequest{
			TransferRequest: &ospb.TransferRequest{
				Version:           version,
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
	Message := ""
	switch v := iresp.GetResponse().(type) {
	case *ospb.InstallResponse_TransferReady:
	case *ospb.InstallResponse_Validated:
		if standby {
			t.Log("DUT standby supervisor has valid preexisting image; skipping transfer.")
		} else {
			t.Log("DUT supervisor has valid preexisting image; skipping transfer.")
		}
		osVersionReceived := iresp.GetValidated().GetVersion()
		t.Logf("Version :: got: %v ,want: %v", osVersionReceived, tc.osVersion)
		if osVersionReceived != tc.osVersion {
			t.Fatalf("OS.Install wrong version received, osVersionActual : %s osVersion %s, ", osVersionReceived, tc.osVersion)
		}
		return
	case *ospb.InstallResponse_SyncProgress:
		if !tc.dualSup {
			t.Fatalf("Unexpected SyncProgress on single supervisor: got %v (%T)", v, v)
		}
		t.Logf("Sync progress: %v%% synced from supervisor", v.SyncProgress.GetPercentageTransferred())
		// Message = fmt.Sprintf("Sync progress: %v%% synced from supervisor", v.SyncProgress.GetPercentageTransferred())
		// xx := iresp.GetValidated().Version
		// yy := iresp.GetValidated().GetVersion()
		// t.Logf("%v,%v", xx, yy)
		// if err != nil {
		// 	t.Fatalf("OS.Install error receiving: %s", err)
		// }
	default:
		Message = fmt.Sprintf("Expected TransferReady following TransferRequest: got %v (%T)", v, v)
		// t.Fatalf("Expected TransferReady following TransferRequest: got %v (%T)", v, v)
		t.Logf("Expected TransferReady following TransferRequest: got %v (%T)", v, v)
	}

	awaitChan := make(chan error)
	go func() {
		err := watchStatus(t, ic, standby, tc.osVersion)
		awaitChan <- err
	}()

	if ErrorString != "" {
		if !strings.Contains(Message, ErrorString) {
			t.Fatalf("want error: %v , and got: %v", ErrorString, Message)
		} else {
			t.Logf("want error: %v , and got: %v", ErrorString, Message)
			return
		}
	}

	// xx := iresp.GetValidated().Version
	// yy := iresp.GetValidated().GetVersion()
	// t.Logf("%v,%v", xx, yy)
	// if err != nil {
	// 	t.Fatalf("OS.Install error receiving: %s", err)
	// }

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
	rebootWait := time.Minute
	deadline := time.Now().Add(tc.timeout)
	for time.Now().Before(deadline) {
		r, err := tc.osc.Verify(ctx, &ospb.VerifyRequest{})
		switch status.Code(err) {
		case codes.OK:
		case codes.Unavailable:
			t.Log("Reboot in progress.")
			time.Sleep(rebootWait)
			continue
		default:
			t.Fatalf("OS.Verify request failed: %v", err)
		}
		// when noreboot is set to false, the device returns "in-progress" before initiating the reboot
		if r.GetActivationFailMessage() == "in-progress" {
			t.Logf("Waiting for reboot to initiate.")
			time.Sleep(rebootWait)
			continue
		}
		if got, want := r.GetActivationFailMessage(), ""; got != want {
			t.Fatalf("OS.Verify ActivationFailMessage: got %q, want %q", got, want)
		}
		if got, want := r.GetVersion(), tc.osVersion; got != want {
			t.Logf("Reboot has not finished with the right version: got %s , want: %s.", got, want)
			time.Sleep(rebootWait)
			continue
		}

		// dut := ondatra.DUT(t, "dut")
		if !deviations.SwVersionUnsupported(tc.dut) {
			ver, ok := gnmi.Lookup(t, tc.dut, gnmi.OC().System().SoftwareVersion().State()).Val()
			if !ok {
				t.Log("Reboot has not finished with the right version: couldn't get system/state/software-version")
				time.Sleep(rebootWait)
				continue
			}
			if got, want := ver, tc.osVersion; !strings.HasPrefix(got, want) {
				t.Logf("Reboot has not finished with the right version: got %s , want: %s.", got, want)
				time.Sleep(rebootWait)
				continue
			}
		}

		if tc.dualSup {
			if got, want := r.GetVerifyStandby().GetVerifyResponse().GetActivationFailMessage(), ""; got != want {
				t.Fatalf("OS.Verify Standby ActivationFailMessage: got %q, want %q", got, want)
			}

			if got, want := r.GetVerifyStandby().GetVerifyResponse().GetVersion(), tc.osVersion; got != want {
				t.Log("Standby not ready.")
				time.Sleep(rebootWait)
				continue
			}
		}

		t.Log("OS.Verify complete")
		return
	}

	t.Fatal("OS.Verify did not return the correct version before deadline.")
}

func (tc *testCase) pollRpc(t *testing.T) {
	startReboot := time.Now()
	const maxRebootTime = 5
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, tc.dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
	// same variable name, gnoiClient, is used for the gNOI connection during the second reboot.
	// This causes the cache to reuse the old connection for the second reboot, which is no longer active due to a timeout from the previous reboot.
	// The retry mechanism clears the previous cache connection and re-establishes new connection.
	for {
		// gnoiClient := dut.RawAPIs().GNOI(t)
		ctx := context.Background()
		response, err := tc.sc.Time(ctx, &spb.TimeRequest{})

		// Log the error if it occurs
		if err != nil {
			t.Logf("Error fetching device time: %v", err)
		}

		// Check if the error code indicates that the service is unavailable
		if status.Code(err) == codes.Unavailable {
			// If the service is unavailable, wait for 30 seconds before retrying
			t.Logf("Service unavailable, retrying in 30 seconds...")
			time.Sleep(30 * time.Second)
		} else {
			// If the device time is fetched successfully, log the success message
			t.Logf("Device Time fetched successfully: %v", response)
			break
		}
		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device gnoi System client ready time: %.2f minutes", time.Since(startReboot).Minutes())
	for {
		// gnoiClient := dut.RawAPIs().GNOI(t)
		ctx := context.Background()
		response, err := tc.osc.Verify(ctx, &ospb.VerifyRequest{})
		// Log the error if it occurs
		if err != nil {
			t.Logf("Error fetching device time: %v", err)
		}

		// Check if the error code indicates that the service is unavailable
		if status.Code(err) != codes.OK {
			// If the service is unavailable, wait for 30 seconds before retrying
			t.Logf("Service unavailable, retrying in 30 seconds...")
			time.Sleep(30 * time.Second)
		} else {
			// If the device time is fetched successfully, log the success message
			t.Logf("OS client Verify fetched successfully: %v", response)
			break
		}
		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device gnoi OS client ready time: %.2f minutes", time.Since(startReboot).Minutes())

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

func watchStatus(t *testing.T, ic ospb.OS_InstallClient, standby bool, version string) error {
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
			if got, want := v.Validated.GetVersion(), version; got != want {
				return fmt.Errorf("mismatched validation software versions: got %s, want %s", got, want)
			}
			return nil
		default:
			return fmt.Errorf("unexpected client install response: got %v (%T)", v, v)
		}
	}
}
