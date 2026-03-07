package failover

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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	cpb "github.com/openconfig/gnoi/containerz"
	gnoisystem "github.com/openconfig/gnoi/system"
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
	imageName     = "cntrsrv_image"
	tag           = "latest"
	containerName = "cntrsrv"
	volName       = "test-failover-vol" // Used in TestContainerAndVolumePersistence

	// Constants for interruption tests
	interruptedVolName = "test-interrupted-vol"

	switchoverWait = 30 * time.Second
	pollInterval   = 1 * time.Second
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
	// Raw system client for switchover.
	sysClient := dut.RawAPIs().GNOI(t).System()

	t.Cleanup(func() {
		t.Log("Starting cleanup...")
		cli := containerztest.Client(t, dut)
		if err := cli.RemoveImage(ctx, imageName, tag, true); err != nil && status.Code(err) != codes.NotFound {
			t.Errorf("Cleanup: failed to remove image %q:%q: %v", imageName, tag, err)
		}
		t.Log("Cleanup finished.")
	})

	t.Run("LoadImage", func(t *testing.T) {
		loadImage(t, ctx, cli, imageName, tag, containerTarPath(t))
	})

	t.Run("Switchover", func(t *testing.T) {
		t.Logf("Switching control processor to %s...", standbyRPBefore)
		switchReq := &gnoisystem.SwitchControlProcessorRequest{
			ControlProcessor: components.GetSubcomponentPath(standbyRPBefore, deviations.GNOISubcomponentPath(dut)),
		}

		if _, err := sysClient.SwitchControlProcessor(ctx, switchReq); err != nil {
			t.Logf("SwitchControlProcessor returned error: %v", err)
		}
	})

	t.Run("VerifyImagePersistence", func(t *testing.T) {
		t.Log("Waiting for DUT to reconnect...")
		time.Sleep(switchoverWait)

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
		if err := verifyImageExistsEventually(t, ctx, cli, imageName, tag, switchoverWait); err != nil {
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
	// Raw system client for switchover.
	sysClient := dut.RawAPIs().GNOI(t).System()

	t.Cleanup(func() {
		t.Log("Starting cleanup...")
		cli := containerztest.Client(t, dut)

		if err := cli.RemoveContainer(ctx, containerName, true); err != nil && status.Code(err) != codes.NotFound {
			t.Errorf("Cleanup: failed to remove container %q: %v", containerName, err)
		}
		if err := cli.RemoveImage(ctx, imageName, tag, true); err != nil && status.Code(err) != codes.NotFound {
			t.Errorf("Cleanup: failed to remove image %q:%q: %v", imageName, tag, err)
		}
		if err := cli.RemoveVolume(ctx, volName, true); err != nil && status.Code(err) != codes.NotFound {
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
		if err := verifyVolumeExists(t, ctx, cli, volName); err != nil {
			t.Fatalf("Volume not found after creation: %v", err)
		}

		loadImage(t, ctx, cli, imageName, tag, containerTarPath(t))

		t.Logf("Deploying and starting container %s...", containerName)
		startOpts := []client.StartOption{
			client.WithPorts([]string{"60061:60061"}),
			client.WithVolumes([]string{fmt.Sprintf("%s:%s", volName, "/data")}),
		}

		// Ensure container is removed before starting.
		if err := cli.RemoveContainer(ctx, containerName, true); err != nil && status.Code(err) != codes.NotFound {
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
		t.Logf("Switching control processor to %s...", standbyRPBefore)
		switchReq := &gnoisystem.SwitchControlProcessorRequest{
			ControlProcessor: components.GetSubcomponentPath(standbyRPBefore, deviations.GNOISubcomponentPath(dut)),
		}

		// Log the error but proceed to wait for the system to come back.
		if _, err := sysClient.SwitchControlProcessor(ctx, switchReq); err != nil {
			t.Logf("SwitchControlProcessor returned error: %v", err)
		}
	})

	t.Run("VerifyRecovery", func(t *testing.T) {
		t.Log("Waiting for DUT to reconnect...")
		// Allow some time for the switchover to initiate and the connection to drop.
		time.Sleep(switchoverWait)

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
		if err := verifyContainerStateEventually(t, ctx, cli, containerName, cpb.ListContainerResponse_RUNNING, switchoverWait); err != nil {
			t.Errorf("Container recovery failed: %v", err)
		}

		t.Log("Verifying volume persistence...")
		if err := verifyVolumeExists(t, ctx, cli, volName); err != nil {
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
		loadImage(t, ctx, cli, imageName, tag, containerTarPath(t))
	})

	// 2. Remove the image.
	t.Run("RemoveImage", func(t *testing.T) {
		if err := cli.RemoveImage(ctx, imageName, tag, false); err != nil {
			t.Fatalf("Failed to remove image %s:%s: %v", imageName, tag, err)
		}
		if err := verifyImageDoesNotExist(t, ctx, cli, imageName, tag); err != nil {
			t.Fatalf("Image still found after removal: %v", err)
		}
	})

	// 3. Identify standby and trigger switchover.
	standbyRPBefore, _, err := findRPs(t, dut)
	if err != nil {
		t.Fatalf("Failed to find RPs before switchover: %v", err)
	}

	t.Run("Switchover", func(t *testing.T) {
		doSwitchover(t, dut, standbyRPBefore)
	})

	// 4. Verify image does not exist after switchover.
	t.Run("VerifyImageRemovalPersistence", func(t *testing.T) {
		t.Log("Waiting for DUT to reconnect...")
		time.Sleep(switchoverWait)

		cli = containerztest.Client(t, dut) // Re-initialize client

		t.Log("Verifying image removal persistence...")
		if err := verifyImageDoesNotExistEventually(t, ctx, cli, imageName, tag, switchoverWait); err != nil {
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
		if err := verifyContainerDoesNotExist(t, ctx, cli, containerName); err != nil {
			t.Fatalf("Container still found after removal: %v", err)
		}
	})

	// 3. Identify standby and trigger switchover.
	standbyRPBefore, _, err := findRPs(t, dut)
	if err != nil {
		t.Fatalf("Failed to find RPs before switchover: %v", err)
	}

	t.Run("Switchover", func(t *testing.T) {
		doSwitchover(t, dut, standbyRPBefore)
	})

	// 4. Verify container does not exist after switchover.
	t.Run("VerifyContainerRemovalPersistence", func(t *testing.T) {
		t.Log("Waiting for DUT to reconnect...")
		time.Sleep(switchoverWait)

		cli = containerztest.Client(t, dut) // Re-initialize client

		t.Log("Verifying container removal persistence...")
		if err := verifyContainerDoesNotExistEventually(t, ctx, cli, containerName, switchoverWait); err != nil {
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
		if err := cli.RemoveImage(ctx, imageName, tag, true); err != nil && status.Code(err) != codes.NotFound {
			t.Errorf("Cleanup: failed to remove image %q:%q: %v", imageName, tag, err)
		}
		t.Log("Cleanup finished.")
	})

	t.Run("LoadImage", func(t *testing.T) {
		loadImage(t, ctx, cli, imageName, tag, containerTarPath(t))
	})

	// First Switchover.
	standbyRP1, activeRP1, err := findRPs(t, dut)
	if err != nil {
		t.Fatalf("Failed to find RPs before first switchover: %v", err)
	}
	t.Logf("Before first switchover, standby is %s, active is %s", standbyRP1, activeRP1)

	t.Run("FirstSwitchover", func(t *testing.T) {
		doSwitchover(t, dut, standbyRP1)
	})

	t.Run("VerifyAfterFirstSwitchover", func(t *testing.T) {
		t.Log("Waiting for DUT to reconnect after first switchover...")
		time.Sleep(switchoverWait)
		cli = containerztest.Client(t, dut)

		t.Log("Verifying image persistence after first switchover...")
		if err := verifyImageExistsEventually(t, ctx, cli, imageName, tag, switchoverWait); err != nil {
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
		doSwitchover(t, dut, standbyRP2)
	})

	t.Run("VerifyAfterSecondSwitchover", func(t *testing.T) {
		t.Log("Waiting for DUT to reconnect after second switchover...")
		time.Sleep(switchoverWait)
		cli = containerztest.Client(t, dut)

		t.Log("Verifying image persistence after second switchover...")
		if err := verifyImageExistsEventually(t, ctx, cli, imageName, tag, switchoverWait); err != nil {
			t.Errorf("Image persistence failed after second switchover: %v", err)
		}
	})
}

// doSwitchover triggers a control processor switchover to the specified standby.
func doSwitchover(t *testing.T, dut *ondatra.DUTDevice, standby string) {
	t.Helper()
	t.Logf("Switching control processor to %s...", standby)
	sysClient := dut.RawAPIs().GNOI(t).System()
	switchReq := &gnoisystem.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(standby, deviations.GNOISubcomponentPath(dut)),
	}
	if _, err := sysClient.SwitchControlProcessor(context.Background(), switchReq); err != nil {
		// Don't fail the test, as the connection is expected to be broken.
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
func verifyContainerState(t *testing.T, ctx context.Context, cli *client.Client, name string, want cpb.ListContainerResponse_Status) error {
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

// verifyImageDoesNotExist checks if an image does not exist.
func verifyImageDoesNotExist(t *testing.T, ctx context.Context, cli *client.Client, name, tag string) error {
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
func verifyImageExistsEventually(t *testing.T, ctx context.Context, cli *client.Client, name, tag string, timeout time.Duration) error {
	t.Helper()
	desc := fmt.Sprintf("image %s:%s to exist", name, tag)
	return poll(t, desc, timeout, func() error { return verifyImageExists(t, ctx, cli, name, tag) })
}

// verifyImageDoesNotExistEventually polls for the image to not exist.
func verifyImageDoesNotExistEventually(t *testing.T, ctx context.Context, cli *client.Client, name, tag string, timeout time.Duration) error {
	t.Helper()
	desc := fmt.Sprintf("image %s:%s to not exist", name, tag)
	return poll(t, desc, timeout, func() error { return verifyImageDoesNotExist(t, ctx, cli, name, tag) })
}

// verifyContainerStateEventually polls for the container to reach the expected state.
func verifyContainerStateEventually(t *testing.T, ctx context.Context, cli *client.Client, name string, want cpb.ListContainerResponse_Status, timeout time.Duration) error {
	t.Helper()
	desc := fmt.Sprintf("container %s to be in state %s", name, want)
	return poll(t, desc, timeout, func() error { return verifyContainerState(t, ctx, cli, name, want) })
}

// verifyContainerDoesNotExist checks if a container does not exist.
func verifyContainerDoesNotExist(t *testing.T, ctx context.Context, cli *client.Client, name string) error {
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
func verifyContainerDoesNotExistEventually(t *testing.T, ctx context.Context, cli *client.Client, name string, timeout time.Duration) error {
	t.Helper()
	desc := fmt.Sprintf("container %q to not exist", name)
	return poll(t, desc, timeout, func() error { return verifyContainerDoesNotExist(t, ctx, cli, name) })
}

// verifyVolumeExists checks if a volume exists.
func verifyVolumeExists(t *testing.T, ctx context.Context, cli *client.Client, name string) error {
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
func loadImage(t *testing.T, ctx context.Context, cli *client.Client, imageName, tag, tarPath string) {
	t.Helper()
	t.Logf("Pushing image %s:%s from %s.", imageName, tag, tarPath)
	progCh, err := cli.PushImage(ctx, imageName, tag, tarPath, false)
	if err != nil {
		t.Fatalf("Initial call to PushImage for %s:%s failed: %w", imageName, tag, err)
	}
	for prog := range progCh {
		if prog.Error != nil {
			t.Fatalf("Error during push of image %s:%s: %w", imageName, tag, prog.Error)
		}
		if !prog.Finished {
			t.Logf("Push progress for %s:%s: %d bytes received.", imageName, tag, prog.BytesReceived)
		}
	}
	if err := verifyImageExists(t, ctx, cli, imageName, tag); err != nil {
		t.Fatalf("Image not found after loading: %v", err)
	}
	t.Logf("Successfully loaded image %s:%s.", imageName, tag)
}