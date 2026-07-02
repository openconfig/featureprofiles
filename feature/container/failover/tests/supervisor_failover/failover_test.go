package failover_test

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/containerz/client"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/containerztest"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnoigo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	cpb "github.com/openconfig/gnoi/containerz"
	gspb "github.com/openconfig/gnoi/system"
)

var (
	containerTar = flag.String("container_tar", "/tmp/cntrsrv.tar", "Path to the container tarball")
	// containerTarPath returns the path to the container tarball.
	// This can be overridden for internal testing behavior using init().
	containerTarPath = func(t *testing.T) string {
		return *containerTar
	}
)

const (
	imageName         = "cntrsrv_image"
	tag               = "latest"
	containerName     = "cntrsrv"
	volName           = "test-failover-vol" // Used in TestContainerAndVolumePersistence
	maxSwitchoverTime = 900
	verifyTimeout     = 5 * time.Minute // baseline for non-ARISTA vendors
	pollInterval      = 1 * time.Second
)

func TestMain(m *testing.M) { fptest.RunTests(m) }

// CNTR 3.1: TestImagePersistence tests that the image persists on the standby control processor after a
// switchover.
func TestImagePersistence(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	if containerTarPath(t) == "" {
		t.Skip("container_tar flag not set, skipping test")
	}

	// Identify the standby control processor to switch to.
	standbyRPBefore, activeRPBefore := findRPsEventually(t, dut, 10*time.Minute)
	t.Logf("Found standby RP: %s, active RP: %s", standbyRPBefore, activeRPBefore)

	// Initialize clients.
	cli := containerztest.Client(t, dut)

	t.Cleanup(func() {
		t.Log("Starting cleanup...")
		cli := containerztest.Client(t, dut)
		if err := cli.RemoveImage(ctx, imageName, tag, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Errorf("Cleanup: failed to remove image %q:%q: %v", imageName, tag, err)
		}
		t.Log("Cleanup finished.")
	})

	t.Run("LoadImage", func(t *testing.T) {
		if err := loadImage(ctx, t, cli, imageName, tag, containerTarPath(t)); err != nil {
			t.Fatalf("Failed to load image: %v", err)
		}
	})

	t.Run("Switchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, activeRPBefore, standbyRPBefore)
	})

	t.Run("VerifyImagePersistence", func(t *testing.T) {
		freshClients := waitForSwitchover(t, dut)

		// Skip config push after switchover -- fresh gNOI clients from
		// waitForSwitchover are passed directly, bypassing Ondatra's cache.
		cli = containerztest.ClientWithoutConfig(t, dut, freshClients)

		// After switchover, the old active RP reboots as new standby; wait for it.
		standbyRPAfter, activeRPAfter := findRPsEventually(t, dut, 10*time.Minute)
		t.Logf("After switchover, found standby RP: %s, active RP: %s", standbyRPAfter, activeRPAfter)
		if standbyRPAfter != activeRPBefore {
			t.Errorf("After switchover, standby RP is %s, want %s", standbyRPAfter, activeRPBefore)
		}
		if activeRPAfter != standbyRPBefore {
			t.Errorf("After switchover, active RP is %s, want %s", activeRPAfter, standbyRPBefore)
		}

		t.Log("Verifying image persistence...")
		if err := verifyImageExistsEventually(ctx, t, cli, imageName, tag, verifyTimeoutForDUT(dut)); err != nil {
			t.Errorf("Image persistence failed: %v", err)
		}
	})
}

// TestContainerAndVolumePersistence implements CNTR-3.2, 3.3, and 3.4 by creating a
// volume, starting a container that mounts it, and verifying both persist and
// the container is running after a switchover.
func TestContainerAndVolumePersistence(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	if containerTarPath(t) == "" {
		t.Skip("container_tar flag not set, skipping test")
	}

	// Identify the standby control processor to switch to.
	standbyRPBefore, activeRPBefore := findRPsEventually(t, dut, 10*time.Minute)
	t.Logf("Found standby RP: %s, active RP: %s", standbyRPBefore, activeRPBefore)

	// Initialize clients.
	cli := containerztest.Client(t, dut)

	t.Cleanup(func() {
		t.Log("Starting cleanup...")
		// Use the outer cli (set to ClientWithoutConfig by VerifyRecovery) rather
		// than calling containerztest.Client(t, dut) again. Calling Client() would
		// push config, restart Octa/Docker, and cause the containerz daemon to
		// re-start the RestartPolicy=Always container -- making RemoveContainer
		// race against the daemon before it can finish cleaning up.
		if err := cli.RemoveContainer(ctx, containerName, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Errorf("Cleanup: failed to remove container %q: %v", containerName, err)
		}
		if err := cli.RemoveImage(ctx, imageName, tag, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Errorf("Cleanup: failed to remove image %q:%q: %v", imageName, tag, err)
		}
		if err := cli.RemoveVolume(ctx, volName, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Errorf("Cleanup: failed to remove volume %q: %v", volName, err)
		}
		t.Log("Cleanup finished.")
	})

	t.Run("Setup", func(t *testing.T) {
		t.Logf("Creating volume %s...", volName)
		volOpts := map[string]string{
			"type":       "none",
			"options":    "bind",
			"mountpoint": "/tmp",
		}
		if _, err := cli.CreateVolume(ctx, volName, "local", nil, volOpts); err != nil {
			t.Fatalf("Failed to create volume: %v", err)
		}
		if err := verifyVolumeExists(ctx, t, cli, volName); err != nil {
			t.Fatalf("Volume not found after creation: %v", err)
		}

		opts := containerztest.StartContainerOptions{
			ImageName:    imageName,
			InstanceName: containerName,
			Command:      "./cntrsrv",
			TarPath:      containerTarPath(t),
			Ports:        []string{"60061:60061"},
			Volumes:      []string{fmt.Sprintf("%s:%s", volName, "/data")},
		}
		if err := containerztest.DeployAndStart(ctx, t, cli, opts); err != nil {
			t.Fatalf("DeployAndStart failed: %v", err)
		}
	})

	t.Run("Switchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, activeRPBefore, standbyRPBefore)
	})

	t.Run("VerifyRecovery", func(t *testing.T) {
		freshClients := waitForSwitchover(t, dut)

		// After switchover, the old active RP reboots as new standby; wait for it.
		standbyRPAfter, activeRPAfter := findRPsEventually(t, dut, 10*time.Minute)
		t.Logf("After switchover, found standby RP: %s, active RP: %s", standbyRPAfter, activeRPAfter)
		if standbyRPAfter != activeRPBefore {
			t.Errorf("After switchover, standby RP is %s, want %s", standbyRPAfter, activeRPBefore)
		}
		if activeRPAfter != standbyRPBefore {
			t.Errorf("After switchover, active RP is %s, want %s", activeRPAfter, standbyRPBefore)
		}

		// Skip config push -- fresh gNOI clients bypass Ondatra's cache.
		// WaitForRunning retries on connection errors, so no pre-dial needed here.
		cli = containerztest.ClientWithoutConfig(t, dut, freshClients)

		t.Log("Verifying container recovery...")
		// Use WaitForRunning instead of verifyContainerStateEventually: it logs
		// each ListContainer result (found name/image/state) so we can see whether
		// the container is absent, stopped, or in another non-RUNNING state.
		if err := containerztest.WaitForRunning(ctx, t, cli, containerName, verifyTimeoutForDUT(dut)); err != nil {
			t.Errorf("Container recovery failed: %v", err)
		}

		t.Log("Verifying volume persistence...")
		if err := verifyVolumeExists(ctx, t, cli, volName); err != nil {
			t.Errorf("Volume persistence failed: %v", err)
		}
	})
}

// TestImageRemovalPersistence implements CNTR-3.5.
func TestImageRemovalPersistence(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	if containerTarPath(t) == "" {
		t.Skip("container_tar flag not set, skipping test")
	}

	cli := containerztest.Client(t, dut)

	// Client() restarts Octa/Docker. Containers with RestartPolicy=Always from
	// previous tests (e.g. TestContainerAndVolumePersistence) reappear seconds
	// after Docker comes up. Retry the removal sweep for up to 90s so that any
	// container that appears after Docker restarts is caught and removed.
	cleanDeadline := time.Now().Add(90 * time.Second)
	for {
		removeContainersUsingImage(ctx, t, cli, imageName, tag)
		if time.Now().After(cleanDeadline) {
			break
		}
		time.Sleep(10 * time.Second)
	}

	// 1. Load the image initially.
	t.Run("LoadImage", func(t *testing.T) {
		if err := loadImage(ctx, t, cli, imageName, tag, containerTarPath(t)); err != nil {
			t.Fatalf("Failed to load image: %v", err)
		}
	})

	// 2. Remove the image.
	t.Run("RemoveImage", func(t *testing.T) {
		if err := cli.RemoveImage(ctx, imageName, tag, false); err != nil {
			t.Fatalf("Failed to remove image %s:%s: %v", imageName, tag, err)
		}
		if err := verifyImageDoesNotExist(ctx, t, cli, imageName, tag); err != nil {
			t.Fatalf("Image still found after removal: %v", err)
		}
	})

	// 3. Identify standby and trigger switchover. The previous test may have triggered
	// a switchover, so the new standby might still be rebooting; retry until it appears.
	standbyRPBefore, activeRPBefore := findRPsEventually(t, dut, 10*time.Minute)

	t.Run("Switchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, activeRPBefore, standbyRPBefore)
	})

	// 4. Verify image does not exist after switchover.
	t.Run("VerifyImageRemovalPersistence", func(t *testing.T) {
		freshClients := waitForSwitchover(t, dut)

		// Skip config push after switchover -- fresh gNOI clients from
		// waitForSwitchover are passed directly, bypassing Ondatra's cache.
		cli = containerztest.ClientWithoutConfig(t, dut, freshClients)

		t.Log("Verifying image removal persistence...")
		if err := verifyImageDoesNotExistEventually(ctx, t, cli, imageName, tag, verifyTimeoutForDUT(dut)); err != nil {
			t.Errorf("Image removal persistence failed, image reappeared: %v", err)
		}
	})
}

// TestContainerRemovalPersistence implements CNTR-3.6.
func TestContainerRemovalPersistence(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	if containerTarPath(t) == "" {
		t.Skip("container_tar flag not set, skipping test")
	}

	cli := containerztest.Client(t, dut)

	// 1. Deploy and start the container.
	opts := containerztest.StartContainerOptions{
		InstanceName:        containerName,
		ImageName:           imageName,
		ImageTag:            tag,
		TarPath:             containerTarPath(t),
		RemoveExistingImage: true,
		PollForRunningState: true,
	}
	if err := containerztest.DeployAndStart(ctx, t, cli, opts); err != nil {
		t.Fatalf("Failed to deploy and start container: %v", err)
	}

	// 2. Remove the container.
	t.Run("RemoveContainer", func(t *testing.T) {
		if err := cli.RemoveContainer(ctx, containerName, true); err != nil {
			t.Fatalf("Failed to remove container %s: %v", containerName, err)
		}
		if err := verifyContainerDoesNotExist(ctx, t, cli, containerName); err != nil {
			t.Fatalf("Container still found after removal: %v", err)
		}
	})

	// 3. Identify standby and trigger switchover. The previous test may have triggered
	// a switchover, so the new standby might still be rebooting; retry until it appears.
	standbyRPBefore, activeRPBefore := findRPsEventually(t, dut, 10*time.Minute)

	t.Run("Switchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, activeRPBefore, standbyRPBefore)
	})

	// 4. Verify container does not exist after switchover.
	t.Run("VerifyContainerRemovalPersistence", func(t *testing.T) {
		freshClients := waitForSwitchover(t, dut)

		// Skip config push after switchover -- fresh gNOI clients from
		// waitForSwitchover are passed directly, bypassing Ondatra's cache.
		cli = containerztest.ClientWithoutConfig(t, dut, freshClients)

		t.Log("Verifying container removal persistence...")
		if err := verifyContainerDoesNotExistEventually(ctx, t, cli, containerName, verifyTimeoutForDUT(dut)); err != nil {
			t.Errorf("Container removal persistence failed, container reappeared: %v", err)
		}
	})
}

// TestDoubleFailoverImagePersistence implements CNTR-3.7.
func TestDoubleFailoverImagePersistence(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	if containerTarPath(t) == "" {
		t.Skip("container_tar flag not set, skipping test")
	}

	cli := containerztest.Client(t, dut)

	t.Cleanup(func() {
		t.Log("Starting cleanup...")
		cli := containerztest.Client(t, dut)
		if err := cli.RemoveImage(ctx, imageName, tag, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Errorf("Cleanup: failed to remove image %q:%q: %v", imageName, tag, err)
		}
		t.Log("Cleanup finished.")
	})

	t.Run("LoadImage", func(t *testing.T) {
		if err := loadImage(ctx, t, cli, imageName, tag, containerTarPath(t)); err != nil {
			t.Fatalf("Failed to load image: %v", err)
		}
	})

	// First Switchover. The previous test may have triggered a switchover, so the new
	// standby might still be rebooting; retry until it appears.
	standbyRP1, activeRP1 := findRPsEventually(t, dut, 10*time.Minute)
	t.Logf("Before first switchover, standby is %s, active is %s", standbyRP1, activeRP1)

	t.Run("FirstSwitchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, activeRP1, standbyRP1)
	})

	t.Run("VerifyAfterFirstSwitchover", func(t *testing.T) {
		freshClients := waitForSwitchover(t, dut)
		// Skip config push -- fresh gNOI clients from waitForSwitchover bypass Ondatra's cache.
		cli = containerztest.ClientWithoutConfig(t, dut, freshClients)

		t.Log("Verifying image persistence after first switchover...")
		if err := verifyImageExistsEventually(ctx, t, cli, imageName, tag, verifyTimeoutForDUT(dut)); err != nil {
			t.Fatalf("Image persistence failed after first switchover: %v", err)
		}
	})

	// Second Switchover (back to original active).
	// After the first switchover, the old active RP reboots as new standby; wait for it.
	standbyRP2, activeRP2 := findRPsEventually(t, dut, 10*time.Minute)
	if activeRP2 != standbyRP1 {
		t.Fatalf("Expected new active RP to be %s, but got %s", standbyRP1, activeRP2)
	}
	t.Logf("Before second switchover, standby is %s, active is %s", standbyRP2, activeRP2)

	t.Run("SecondSwitchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, activeRP2, standbyRP2)
	})

	t.Run("VerifyAfterSecondSwitchover", func(t *testing.T) {
		freshClients := waitForSwitchover(t, dut)
		// Skip config push -- fresh gNOI clients from waitForSwitchover bypass Ondatra's cache.
		cli = containerztest.ClientWithoutConfig(t, dut, freshClients)

		t.Log("Verifying image persistence after second switchover...")
		if err := verifyImageExistsEventually(ctx, t, cli, imageName, tag, verifyTimeoutForDUT(dut)); err != nil {
			t.Errorf("Image persistence failed after second switchover: %v", err)
		}
	})
}

// TestContainerPersistenceAfterColdReboot implements CNTR-3.8 checking container persistence after a chassis cold reboot.
func TestContainerPersistenceAfterColdReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	if containerTarPath(t) == "" {
		t.Skip("container_tar flag not set, skipping test")
	}

	cli := containerztest.Client(t, dut)
	sysClient := dut.RawAPIs().GNOI(t).System()

	// postRebootClients is set by VerifyPersistence so the cleanup can use a
	// fresh gNOI connection instead of the stale pre-reboot Ondatra cache.
	var postRebootClients gnoigo.Clients
	t.Cleanup(func() {
		t.Log("Starting cleanup...")
		var cli *client.Client
		if deviations.ContainerzRequireExplicitConfigSave(dut) {
			cli = containerztest.ClientWithoutConfig(t, dut, postRebootClients)
		} else {
			cli = containerztest.Client(t, dut)
		}
		if err := cli.RemoveContainer(ctx, containerName, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Errorf("Cleanup: failed to remove container %q: %v", containerName, err)
		}
		if err := cli.RemoveImage(ctx, imageName, tag, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Errorf("Cleanup: failed to remove image %q:%q: %v", imageName, tag, err)
		}
		if err := cli.RemoveVolume(ctx, volName, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Errorf("Cleanup: failed to remove volume %q: %v", volName, err)
		}
		t.Log("Cleanup finished.")
	})

	t.Run("Setup", func(t *testing.T) {
		t.Logf("Creating volume %s...", volName)
		volOpts := map[string]string{
			"type":       "none",
			"options":    "bind",
			"mountpoint": "/tmp",
		}
		if _, err := cli.CreateVolume(ctx, volName, "local", nil, volOpts); err != nil {
			t.Fatalf("Failed to create volume: %v", err)
		}
		opts := containerztest.StartContainerOptions{
			ImageName:    imageName,
			InstanceName: containerName,
			Command:      "./cntrsrv",
			TarPath:      containerTarPath(t),
			Ports:        []string{"60061:60061"},
			Volumes:      []string{fmt.Sprintf("%s:%s", volName, "/data")},
		}
		if err := containerztest.DeployAndStart(ctx, t, cli, opts); err != nil {
			t.Fatalf("DeployAndStart failed: %v", err)
		}
		// Wait for supervisors to sync before cold reboot.
		standbyRP1, activeRP1 := findRPsEventually(t, dut, 10*time.Minute)
		t.Logf("Before rebooting, standby is %s, active is %s", standbyRP1, activeRP1)
		switchoverReady := gnmi.OC().Component(activeRP1).SwitchoverReady()
		switchoverReadyTimeout := 5*time.Minute + 2*time.Duration(deviations.SwitchoverStabilizeDelayM(dut))*time.Minute
		gnmi.Await(t, dut, switchoverReady.State(), switchoverReadyTimeout, true)
		t.Logf("Supervisors synchronized, proceeding with cold reboot")

		if deviations.ContainerzRequireExplicitConfigSave(dut) {
			containerztest.SaveConfig(t, dut)
		}
	})

	t.Run("ColdReboot", func(t *testing.T) {
		t.Log("Rebooting chassis (cold reboot)...")
		rebootReq := &gspb.RebootRequest{
			Method:  gspb.RebootMethod_COLD,
			Delay:   0,
			Message: "Container persistence test reboot",
			Force:   true,
		}
		// We expect the connection to drop.
		if _, err := sysClient.Reboot(ctx, rebootReq); err != nil {
			t.Logf("Reboot returned error (expected): %v", err)
		}
	})

	t.Run("VerifyPersistence", func(t *testing.T) {
		// alreadyDown=true: the ColdReboot subtest sent the reboot command and
		// waited for the TCP timeout (~15 min). The device may have rebooted and
		// come back up before polling starts; treat first successful poll as recovery.
		postRebootClients = containerztest.WaitForReboot(t, dut, true)

		// For ARISTA: skip config push after reboot -- config was saved to startup-config
		if deviations.ContainerzRequireExplicitConfigSave(dut) {
			cli = containerztest.ClientWithoutConfig(t, dut, postRebootClients)
		} else {
			cli = containerztest.Client(t, dut)
		}

		timeout := 5 * time.Minute
		if err := verifyContainerStateEventually(ctx, t, cli, containerName, cpb.ListContainerResponse_RUNNING, timeout); err != nil {
			t.Errorf("Container persistence failed: %v", err)
		}

		if err := verifyVolumeExistsEventually(ctx, t, cli, volName, timeout); err != nil {
			t.Errorf("Volume persistence failed: %v", err)
		}
	})
}

// waitForSwitchover polls the DUT until the switchover has completed and the device is
// reachable, then returns fresh gNOI clients.
//
// Two scenarios arise after doSwitchover returns:
//  1. Switchover succeeded quickly: the device went unreachable and came back before
//     polling started. DialGNMI succeeds on the first poll.
//  2. Switchover succeeded slowly: the device is still unreachable when polling starts.
//     DialGNMI fails until the device comes back.
//  3. Switchover did not execute (SwitchControlProcessor silently failed): the device
//     was up the whole time. Indistinguishable from case 1 via connectivity alone.
//
// For cases 1 and 3, we use stableUpCount: after 4 consecutive UP polls (2 min) without
// ever observing the device go down, we return. The caller already checks RP roles and
// reports a test error if the switchover did not actually happen.
func waitForSwitchover(t *testing.T, dut *ondatra.DUTDevice) gnoigo.Clients {
	t.Helper()
	startSwitchover := time.Now()
	t.Logf("Wait for new Primary controller to boot up by polling the telemetry output.")
	timeout := time.After(time.Duration(maxSwitchoverTime) * time.Second)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var deviceWentDown bool
	var stableUpCount int
	for {
		select {
		case <-timeout:
			t.Fatalf("DUT did not complete switchover within %ds", maxSwitchoverTime)
		case <-ticker.C:
			// Always probe with a short-timeout DialGNMI before gnmi.Get to avoid
			// blocking on a stale half-open TCP connection after a switchover.
			dialCtx, dialCancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, dialErr := dut.RawAPIs().BindingDUT().DialGNMI(dialCtx)
			dialCancel()
			if dialErr != nil {
				if !deviceWentDown {
					t.Log("Device is now unreachable. Waiting for the new primary to come up.")
					deviceWentDown = true
				}
				stableUpCount = 0
				t.Logf("Time elapsed %.0f seconds, GNMI dial failed: %v", time.Since(startSwitchover).Seconds(), dialErr)
				continue
			}
			var currentTime string
			errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			})
			if errMsg != nil {
				if !deviceWentDown {
					t.Log("Device is now unreachable. Waiting for the new primary to come up.")
					deviceWentDown = true
				}
				stableUpCount = 0
				t.Logf("Time elapsed %.0f seconds, DUT not reachable yet.", time.Since(startSwitchover).Seconds())
			} else {
				if deviceWentDown {
					t.Logf("Controller switchover has completed with received time: %v", currentTime)
					t.Logf("Controller switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())
					dialCtx, dialCancel := context.WithTimeout(context.Background(), 30*time.Second)
					freshClients, err := dut.RawAPIs().BindingDUT().DialGNOI(dialCtx)
					dialCancel()
					if err != nil {
						t.Logf("gNOI re-dial after switchover failed (non-fatal): %v", err)
					}
					return freshClients
				}
				stableUpCount++
				if stableUpCount >= 4 {
					// Device has been consistently reachable for 4 polls (2 min)
					// without going down. The switchover either completed before
					// polling started or did not execute. Return and let the caller
					// verify RP roles to distinguish the two cases.
					t.Logf("Device stayed reachable for %d polls; returning (switchover may have completed before polling started).", stableUpCount)
					dialCtx, dialCancel := context.WithTimeout(context.Background(), 30*time.Second)
					freshClients, err := dut.RawAPIs().BindingDUT().DialGNOI(dialCtx)
					dialCancel()
					if err != nil {
						t.Logf("gNOI re-dial failed (non-fatal): %v", err)
					}
					return freshClients
				}
				t.Log("Device is still reachable; switchover may not have started yet.")
			}
		}
	}
}

// verifyTimeoutForDUT returns the timeout for post-switchover/reboot container and image
// verification. Supervisors may restart containerz after a switchover; the new active
// needs extra time for Docker state to settle before container and image states are final.
func verifyTimeoutForDUT(dut *ondatra.DUTDevice) time.Duration {
	return verifyTimeout + time.Duration(deviations.SwitchoverStabilizeDelayM(dut))*time.Minute
}

// awaitSwitchoverReadyAndSwitch waits for the active supervisor to report switchover-ready,
// then triggers a switchover to the standby. The active supervisor's switchover-ready leaf
// is checked because it tracks actual standby readiness; the standby's own leaf is not
// reliably maintained while in standby mode.
func awaitSwitchoverReadyAndSwitch(t *testing.T, dut *ondatra.DUTDevice, active, standby string) {
	t.Helper()
	// Config push may cause Octa to restart, temporarily interrupting standby RP
	// synchronization; SwitchoverStabilizeDelayM allows extra time for re-sync.
	timeout := 5*time.Minute + 2*time.Duration(deviations.SwitchoverStabilizeDelayM(dut))*time.Minute
	components.AwaitSwitchoverReady(t, dut, active, timeout)
	doSwitchover(t, dut, standby)
}

// doSwitchover triggers a control processor switchover to the specified standby,
// retrying on gRPC Unavailable. Per the gRPC spec, Unavailable signals the client
// to back off and retry the same call -- the device may not yet be ready to accept
// the switchover RPC even though switchover-ready is true. Retries stop when the
// RPC succeeds, the device becomes unreachable (switchover executing), or the
// timeout expires.
func doSwitchover(t *testing.T, dut *ondatra.DUTDevice, standby string) {
	t.Helper()
	t.Logf("Switching control processor to %s...", standby)

	retryTimeout := 5*time.Minute + time.Duration(deviations.SwitchoverStabilizeDelayM(dut))*time.Minute
	deadline := time.Now().Add(retryTimeout)

	switchReq := &gspb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(standby, deviations.GNOISubcomponentPath(dut)),
	}

	for {
		var sysClient gspb.SystemClient
		if deviations.GnoiRequiresFreshDialAfterSwitchover(dut) {
			dialCtx, dialCancel := context.WithTimeout(context.Background(), 30*time.Second)
			freshClients, err := dut.RawAPIs().BindingDUT().DialGNOI(dialCtx)
			dialCancel()
			if err == nil {
				sysClient = freshClients.System()
			} else {
				t.Logf("gNOI fresh dial failed, falling back to cached client: %v", err)
				sysClient = dut.RawAPIs().GNOI(t).System()
			}
		} else {
			sysClient = dut.RawAPIs().GNOI(t).System()
		}

		switchCtx, switchCancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := sysClient.SwitchControlProcessor(switchCtx, switchReq)
		switchCancel()
		if err == nil {
			t.Logf("SwitchControlProcessor succeeded.")
			return
		}
		t.Logf("SwitchControlProcessor returned: %v", err)

		if status.Code(err) == codes.Unavailable {
			// Check if the device went unreachable -- if so, the switchover is
			// already executing and waitForSwitchover will handle the rest.
			dialCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, dialErr := dut.RawAPIs().BindingDUT().DialGNMI(dialCtx)
			cancel()
			if dialErr != nil {
				t.Logf("Device unreachable after Unavailable -- switchover is executing.")
				return
			}
			if time.Now().After(deadline) {
				t.Logf("SwitchControlProcessor retries exhausted after %v.", retryTimeout)
				return
			}
			t.Logf("Device still reachable, retrying SwitchControlProcessor in 30s...")
			time.Sleep(30 * time.Second)
			continue
		}

		// Non-retryable error -- log and return, let the caller's verification fail.
		return
	}
}

// findRPs identifies the primary/active and secondary/standby control processors.
func findRPs(t *testing.T, dut *ondatra.DUTDevice) (string, string, error) {
	t.Helper()
	comps := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	var standbyRP, activeRP string
	for _, c := range comps {
		if c.GetRedundantRole() == oc.Platform_ComponentRedundantRole_SECONDARY {
			standbyRP = c.GetName()
		} else if c.GetRedundantRole() == oc.Platform_ComponentRedundantRole_PRIMARY {
			activeRP = c.GetName()
		}
	}
	if standbyRP == "" {
		return "", "", fmt.Errorf("no standby control processor found")
	}
	if activeRP == "" {
		return "", "", fmt.Errorf("no active control processor found")
	}
	return standbyRP, activeRP, nil
}

// findRPsEventually retries findRPs until the standby RP appears or the timeout expires.
// After a switchover, the old active RP takes time to reboot and register as standby.
func findRPsEventually(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) (string, string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		standby, active, err := findRPs(t, dut)
		if err == nil {
			return standby, active
		}
		if time.Now().After(deadline) {
			t.Fatalf("Standby RP did not appear within %v: %v", timeout, err)
		}
		t.Logf("Waiting for standby RP to reappear (will retry): %v", err)
		time.Sleep(30 * time.Second)
	}
}

// verifyContainerState checks if a container exists and is in the expected state.
func verifyContainerState(ctx context.Context, t *testing.T, cli *client.Client, name string, want cpb.ListContainerResponse_Status) error {
	// Use the client's ListContainer with filter.
	t.Helper()
	listCh, err := cli.ListContainer(ctx, true, 0, map[string][]string{"name": {name}})
	if err != nil {
		return fmt.Errorf("ListContainer failed: %w", err)
	}

	found := false
	for cnt := range listCh {
		if cnt.Error != nil {
			return fmt.Errorf("error listing containers: %w", cnt.Error)
		}
		// Handle potential leading slash in name returned by some vendor implementations.
		if strings.TrimPrefix(cnt.Name, "/") == name {
			found = true
			if cnt.State != want.String() {
				return fmt.Errorf("container %s state is %v, want %v", name, cnt.State, want)
			}
		}
	}
	if !found {
		return fmt.Errorf("container %s not found", name)
	}
	return nil
}

// verifyImageExists checks if an image exists.
func verifyImageExists(ctx context.Context, t *testing.T, cli *client.Client, name, tag string) error {
	t.Helper()
	imgCh, err := cli.ListImage(ctx, 0, map[string][]string{"name": {name}, "tag": {tag}})
	if err != nil {
		return fmt.Errorf("ListImage failed: %w", err)
	}

	foundImage := false
	for img := range imgCh {
		if img.Error != nil {
			return fmt.Errorf("error listing images: %w", img.Error)
		}
		if img.ImageName == name && img.ImageTag == tag {
			foundImage = true
			break
		}
	}
	if !foundImage {
		return fmt.Errorf("image %s:%s not found", name, tag)
	}
	return nil
}

// verifyImageDoesNotExist checks if an image does not exist.
func verifyImageDoesNotExist(ctx context.Context, t *testing.T, cli *client.Client, name, tag string) error {
	t.Helper()
	imgCh, err := cli.ListImage(ctx, 0, map[string][]string{"name": {name}, "tag": {tag}})
	if err != nil {
		return fmt.Errorf("ListImage failed while checking for non-existence: %w", err)
	}

	for img := range imgCh {
		if img.Error != nil {
			return fmt.Errorf("error listing images while checking for non-existence: %w", img.Error)
		}
		if img.ImageName == name && img.ImageTag == tag {
			return fmt.Errorf("image %s:%s was found, but it should not exist", name, tag)
		}
	}
	return nil
}

// verifyImageExistsEventually polls for the image to exist.
func verifyImageExistsEventually(ctx context.Context, t *testing.T, cli *client.Client, name, tag string, timeout time.Duration) error {
	t.Helper()
	desc := fmt.Sprintf("image %s:%s to exist", name, tag)
	return poll(t, desc, timeout, func() error { return verifyImageExists(ctx, t, cli, name, tag) })
}

// verifyImageDoesNotExistEventually polls for the image to not exist.
func verifyImageDoesNotExistEventually(ctx context.Context, t *testing.T, cli *client.Client, name, tag string, timeout time.Duration) error {
	t.Helper()
	desc := fmt.Sprintf("image %s:%s to not exist", name, tag)
	return poll(t, desc, timeout, func() error { return verifyImageDoesNotExist(ctx, t, cli, name, tag) })
}

// verifyContainerStateEventually polls for the container to reach the expected state.
func verifyContainerStateEventually(ctx context.Context, t *testing.T, cli *client.Client, name string, want cpb.ListContainerResponse_Status, timeout time.Duration) error {
	t.Helper()
	desc := fmt.Sprintf("container %s to be in state %s", name, want)
	return poll(t, desc, timeout, func() error { return verifyContainerState(ctx, t, cli, name, want) })
}

// verifyContainerDoesNotExist checks if a container does not exist.
func verifyContainerDoesNotExist(ctx context.Context, t *testing.T, cli *client.Client, name string) error {
	t.Helper()
	listCh, err := cli.ListContainer(ctx, true, 0, map[string][]string{"name": {name}})
	if err != nil {
		return fmt.Errorf("ListContainer failed: %w", err)
	}

	for cnt := range listCh {
		if cnt.Error != nil {
			return fmt.Errorf("error listing containers: %w", cnt.Error)
		}
		if strings.TrimPrefix(cnt.Name, "/") == name {
			return fmt.Errorf("container %s found, but it should not exist (state: %s)", name, cnt.State)
		}
	}
	return nil
}

// verifyContainerDoesNotExistEventually polls for the container to not exist.
func verifyContainerDoesNotExistEventually(ctx context.Context, t *testing.T, cli *client.Client, name string, timeout time.Duration) error {
	t.Helper()
	desc := fmt.Sprintf("container %q to not exist", name)
	return poll(t, desc, timeout, func() error { return verifyContainerDoesNotExist(ctx, t, cli, name) })
}

// verifyVolumeExistsEventually polls for the volume to exist.
func verifyVolumeExistsEventually(ctx context.Context, t *testing.T, cli *client.Client, name string, timeout time.Duration) error {
	t.Helper()
	desc := fmt.Sprintf("volume %s to exist", name)
	return poll(t, desc, timeout, func() error { return verifyVolumeExists(ctx, t, cli, name) })
}

// verifyVolumeExists checks if a volume exists.
func verifyVolumeExists(ctx context.Context, t *testing.T, cli *client.Client, name string) error {
	t.Helper()
	volCh, err := cli.ListVolume(ctx, map[string][]string{"name": {name}})
	if err != nil {
		return fmt.Errorf("ListVolume failed: %w", err)
	}

	found := false
	for vol := range volCh {
		if vol.Error != nil {
			return fmt.Errorf("error listing volumes: %w", vol.Error)
		}
		if vol.Name == name {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("volume %s not found", name)
	}
	return nil
}

// poll is a generic helper to retry a check function until it succeeds or a timeout is reached.
func poll(t *testing.T, desc string, timeout time.Duration, check func() error) error {
	t.Helper()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	timer := time.After(timeout)

	for {
		select {
		case <-timer:
			return fmt.Errorf("timed out after %v waiting for %s", timeout, desc)
		case <-ticker.C:
			if err := check(); err == nil {
				return nil // Success
			}
		}
	}
}

// removeContainersUsingImage removes all containers that use the given image so
// that a subsequent RemoveImage call can succeed without a "running container"
// error. This guards against cross-test DUT contamination when the same DUT is
// reused across tests (e.g. CNTR-1's TestContainerPersistenceAfterColdReboot
// leaves a container running with restart-policy Always, which would block
// CNTR-3's TestImageRemovalPersistence from removing the image).
func removeContainersUsingImage(ctx context.Context, t *testing.T, cli *client.Client, imageName, tag string) {
	t.Helper()
	wantImage := imageName + ":" + tag
	ch, err := cli.ListContainer(ctx, true, 0, nil)
	if err != nil {
		t.Logf("Pre-cleanup: ListContainer failed: %v", err)
		return
	}
	for info := range ch {
		if info.Error != nil {
			continue
		}
		if info.ImageName == wantImage {
			name := strings.TrimPrefix(info.Name, "/")
			t.Logf("Pre-cleanup: removing container %q using image %s", name, wantImage)
			if err := cli.RemoveContainer(ctx, name, true); err != nil {
				t.Logf("Pre-cleanup: failed to remove container %q: %v", name, err)
			}
		}
	}
}

// loadImage pushes a container image to the DUT and verifies it exists.
func loadImage(ctx context.Context, t *testing.T, cli *client.Client, imageName, tag, tarPath string) error {
	t.Helper()
	t.Logf("Pushing image %s:%s from %s.", imageName, tag, tarPath)
	progCh, err := cli.PushImage(ctx, imageName, tag, tarPath, false)
	if err != nil {
		return fmt.Errorf("Initial call to PushImage for %s:%s failed: %w", imageName, tag, err)
	}
	for prog := range progCh {
		if prog.Error != nil {
			return fmt.Errorf("Error during push of image %s:%s: %w", imageName, tag, prog.Error)
		}
		if prog.Finished {
			t.Logf("Successfully pushed image %s:%s.", prog.Image, prog.Tag)
		} else {
			t.Logf("Push progress for %s:%s: %d bytes received.", imageName, tag, prog.BytesReceived)
		}
	}
	if err := verifyImageExists(ctx, t, cli, imageName, tag); err != nil {
		return fmt.Errorf("Image not found after loading: %v", err)
	}
	t.Logf("Successfully loaded image %s:%s.", imageName, tag)
	return nil
}
