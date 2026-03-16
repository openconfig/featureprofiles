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
	verifyTimeout     = 5 * time.Minute
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
	standbyRPBefore, activeRPBefore, err := findRPs(t, dut)
	if err != nil {
		t.Fatalf("Failed to find RPs before switchover: %v", err)
	}
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
		awaitSwitchoverReadyAndSwitch(t, dut, standbyRPBefore)
	})

	t.Run("VerifyImagePersistence", func(t *testing.T) {
		waitForSwitchover(t, dut)

		cli = containerztest.Client(t, dut)

		standbyRPAfter, activeRPAfter, err := findRPs(t, dut)
		if err != nil {
			t.Fatalf("Failed to find RPs after switchover: %v", err)
		}
		t.Logf("After switchover, found standby RP: %s, active RP: %s", standbyRPAfter, activeRPAfter)
		if standbyRPAfter != activeRPBefore {
			t.Errorf("After switchover, standby RP is %s, want %s", standbyRPAfter, activeRPBefore)
		}
		if activeRPAfter != standbyRPBefore {
			t.Errorf("After switchover, active RP is %s, want %s", activeRPAfter, standbyRPBefore)
		}

		t.Log("Verifying image persistence...")
		if err := verifyImageExistsEventually(ctx, t, cli, imageName, tag, verifyTimeout); err != nil {
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
	standbyRPBefore, activeRPBefore, err := findRPs(t, dut)
	if err != nil {
		t.Fatalf("Failed to find RPs before switchover: %v", err)
	}
	t.Logf("Found standby RP: %s, active RP: %s", standbyRPBefore, activeRPBefore)

	// Initialize clients.
	cli := containerztest.Client(t, dut)

	t.Cleanup(func() {
		t.Log("Starting cleanup...")
		cli := containerztest.Client(t, dut)

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

		if err := loadImage(ctx, t, cli, imageName, tag, containerTarPath(t)); err != nil {
			t.Fatalf("Failed to load image: %v", err)
		}

		t.Logf("Deploying and starting container %s...", containerName)
		startOpts := []client.StartOption{
			client.WithPorts([]string{"60061:60061"}),
			client.WithVolumes([]string{fmt.Sprintf("%s:%s", volName, "/data")}),
		}

		// Ensure container is removed before starting.
		if err := cli.RemoveContainer(ctx, containerName, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Logf("Pre-start removal of container %s failed: %v", containerName, err)
		}

		if _, err := cli.StartContainer(ctx, imageName, tag, "./cntrsrv", containerName, startOpts...); err != nil {
			t.Fatalf("Failed to start container %s with volume %s: %v", containerName, volName, err)
		}

		// Verify container is running.
		if err := containerztest.WaitForRunning(ctx, t, cli, containerName, 30*time.Second); err != nil {
			t.Fatalf("Container %s did not reach running state: %v", containerName, err)
		}
	})

	t.Run("Switchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, standbyRPBefore)
	})

	t.Run("VerifyRecovery", func(t *testing.T) {
		waitForSwitchover(t, dut)

		// Refresh clients after reconnection.
		cli = containerztest.Client(t, dut)

		standbyRPAfter, activeRPAfter, err := findRPs(t, dut)
		if err != nil {
			t.Fatalf("Failed to find RPs after switchover: %v", err)
		}
		t.Logf("After switchover, found standby RP: %s, active RP: %s", standbyRPAfter, activeRPAfter)
		if standbyRPAfter != activeRPBefore {
			t.Errorf("After switchover, standby RP is %s, want %s", standbyRPAfter, activeRPBefore)
		}
		if activeRPAfter != standbyRPBefore {
			t.Errorf("After switchover, active RP is %s, want %s", activeRPAfter, standbyRPBefore)
		}

		t.Log("Verifying container recovery...")
		if err := verifyContainerStateEventually(ctx, t, cli, containerName, cpb.ListContainerResponse_RUNNING, verifyTimeout); err != nil {
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

	// 3. Identify standby and trigger switchover.
	standbyRPBefore, _, err := findRPs(t, dut)
	if err != nil {
		t.Fatalf("Failed to find RPs before switchover: %v", err)
	}

	t.Run("Switchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, standbyRPBefore)
	})

	// 4. Verify image does not exist after switchover.
	t.Run("VerifyImageRemovalPersistence", func(t *testing.T) {
		waitForSwitchover(t, dut)

		cli = containerztest.Client(t, dut) // Re-initialize client

		t.Log("Verifying image removal persistence...")
		if err := verifyImageDoesNotExistEventually(ctx, t, cli, imageName, tag, verifyTimeout); err != nil {
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

	// 3. Identify standby and trigger switchover.
	standbyRPBefore, _, err := findRPs(t, dut)
	if err != nil {
		t.Fatalf("Failed to find RPs before switchover: %v", err)
	}

	t.Run("Switchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, standbyRPBefore)
	})

	// 4. Verify container does not exist after switchover.
	t.Run("VerifyContainerRemovalPersistence", func(t *testing.T) {
		waitForSwitchover(t, dut)

		cli = containerztest.Client(t, dut) // Re-initialize client

		t.Log("Verifying container removal persistence...")
		if err := verifyContainerDoesNotExistEventually(ctx, t, cli, containerName, verifyTimeout); err != nil {
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

	// First Switchover.
	standbyRP1, activeRP1, err := findRPs(t, dut)
	if err != nil {
		t.Fatalf("Failed to find RPs before first switchover: %v", err)
	}
	t.Logf("Before first switchover, standby is %s, active is %s", standbyRP1, activeRP1)

	t.Run("FirstSwitchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, standbyRP1)
	})

	t.Run("VerifyAfterFirstSwitchover", func(t *testing.T) {
		waitForSwitchover(t, dut)
		cli = containerztest.Client(t, dut)

		t.Log("Verifying image persistence after first switchover...")
		if err := verifyImageExistsEventually(ctx, t, cli, imageName, tag, verifyTimeout); err != nil {
			t.Fatalf("Image persistence failed after first switchover: %v", err)
		}
	})

	// Second Switchover (back to original active).
	standbyRP2, activeRP2, err := findRPs(t, dut)
	if err != nil {
		t.Fatalf("Failed to find RPs before second switchover: %v", err)
	}
	if activeRP2 != standbyRP1 {
		t.Fatalf("Expected new active RP to be %s, but got %s", standbyRP1, activeRP2)
	}
	t.Logf("Before second switchover, standby is %s, active is %s", standbyRP2, activeRP2)

	t.Run("SecondSwitchover", func(t *testing.T) {
		awaitSwitchoverReadyAndSwitch(t, dut, standbyRP2)
	})

	t.Run("VerifyAfterSecondSwitchover", func(t *testing.T) {
		waitForSwitchover(t, dut)
		cli = containerztest.Client(t, dut)

		t.Log("Verifying image persistence after second switchover...")
		if err := verifyImageExistsEventually(ctx, t, cli, imageName, tag, verifyTimeout); err != nil {
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

	t.Cleanup(func() {
		t.Log("Starting cleanup...")
		// Re-initialize client in case of connection loss
		cli := containerztest.Client(t, dut)
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
		if err := loadImage(ctx, t, cli, imageName, tag, containerTarPath(t)); err != nil {
			t.Fatalf("Failed to load image: %v", err)
		}

		t.Logf("Starting container %s...", containerName)
		startOpts := []client.StartOption{
			client.WithPorts([]string{"60061:60061"}),
			client.WithVolumes([]string{fmt.Sprintf("%s:%s", volName, "/data")}),
		}
		// Ensure container is removed before starting.
		if err := cli.RemoveContainer(ctx, containerName, true); err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.Unknown {
			t.Logf("Pre-start removal of container %s failed: %v", containerName, err)
		}
		if _, err := cli.StartContainer(ctx, imageName, tag, "./cntrsrv", containerName, startOpts...); err != nil {
			t.Fatalf("Failed to start container: %v", err)
		}
		if err := containerztest.WaitForRunning(ctx, t, cli, containerName, 30*time.Second); err != nil {
			t.Fatalf("Container did not start: %v", err)
		}
		// Wait for supervisors to sync before cold reboot.
		standbyRP1, activeRP1, err := findRPs(t, dut)
		if err != nil {
			t.Fatalf("Failed to find RPs before cold reboot: %v", err)
		}
		t.Logf("Before rebooting, standby is %s, active is %s", standbyRP1, activeRP1)
		switchoverReady := gnmi.OC().Component(standbyRP1).SwitchoverReady()
		gnmi.Await(t, dut, switchoverReady.State(), 5*time.Minute, true)
		t.Logf("Supervisors synchronized, proceeding with cold reboot")
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
		t.Log("Waiting for DUT to reboot and reconnect...")
		// Wait for reboot.
		time.Sleep(10 * time.Minute)

		// Poll for container state.
		cli = containerztest.Client(t, dut)

		// Use a generous timeout for the device to come back up and the container to start.
		timeout := 5 * time.Minute
		if err := verifyContainerStateEventually(ctx, t, cli, containerName, cpb.ListContainerResponse_RUNNING, timeout); err != nil {
			t.Errorf("Container persistence failed: %v", err)
		}

		if err := verifyVolumeExistsEventually(ctx, t, cli, volName, timeout); err != nil {
			t.Errorf("Volume persistence failed: %v", err)
		}
	})
}

// waitForSwitchover waits for the switchover to complete by polling telemetry.
func waitForSwitchover(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	startSwitchover := time.Now()
	t.Logf("Wait for new Primary controller to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(1 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Controller switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if uint64(time.Since(startSwitchover).Seconds()) > maxSwitchoverTime {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", time.Since(startSwitchover), maxSwitchoverTime)
		}
	}
	t.Logf("Controller switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())
}

// awaitSwitchoverReadyAndSwitch waits for the standby to be switchover-ready, then triggers the switchover.
func awaitSwitchoverReadyAndSwitch(t *testing.T, dut *ondatra.DUTDevice, standby string) {
	t.Helper()
	switchoverReady := gnmi.OC().Component(standby).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 5*time.Minute, true)
	t.Logf("SwitchoverReady: %v", gnmi.Get(t, dut, switchoverReady.State()))
	doSwitchover(t, dut, standby)
}

// doSwitchover triggers a control processor switchover to the specified standby.
func doSwitchover(t *testing.T, dut *ondatra.DUTDevice, standby string) {
	t.Helper()
	t.Logf("Switching control processor to %s...", standby)
	sysClient := dut.RawAPIs().GNOI(t).System()
	switchReq := &gspb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(standby, deviations.GNOISubcomponentPath(dut)),
	}
	if _, err := sysClient.SwitchControlProcessor(context.Background(), switchReq); err != nil {
		t.Logf("SwitchControlProcessor returned error (this is often expected): %v", err)
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
