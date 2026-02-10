package failover

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/containerz/client"
	"github.com/openconfig/featureprofiles/internal/containerztest"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	cpb "github.com/openconfig/gnoi/containerz"
	gnoisystem "github.com/openconfig/gnoi/system"
	gnoitypes "github.com/openconfig/gnoi/types"
)

var (
	containerTar = flag.String("container_tar", "", "Path to the container tarball")
	// containerTarPath returns the path to the container tarball.
	// This can be overridden for internal testing behavior using init().
	containerTarPath = func(t *testing.T) string {
		return *containerTar
	}
)

const (
	imageName      = "cntrsrv_image"
	tag            = "latest"
	containerName  = "cntrsrv"
	volName        = "test-failover-vol"
	switchoverWait = 15 * time.Minute
	pollInterval   = 10 * time.Second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestContainerSupervisorFailover(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	if containerTarPath(t) == "" {
		t.Skip("container_tar flag not set, skipping test")
	}

	// Identify the standby control processor to switch to.
	standbyRP, err := findStandbyRP(t, dut)
	if err != nil {
		t.Fatalf("Failed to find standby RP: %v", err)
	}
	t.Logf("Found standby RP: %s", standbyRP)

	// Initialize clients.
	cli := containerztest.Client(t, dut)
	// Raw system client for switchover.
	sysClient := dut.RawAPIs().GNOI(t).System()

	t.Run("Setup", func(t *testing.T) {
		t.Logf("Deploying container image %s:%s...", imageName, tag)
		// Use PushImage from the client library.
		progCh, err := cli.PushImage(ctx, imageName, tag, containerTarPath(t), false)
		if err != nil {
			t.Fatalf("Failed to push image: %v", err)
		}
		// Drain the progress channel to ensure completion.
		for prog := range progCh {
			if prog.Error != nil {
				t.Fatalf("Image push failed: %v", prog.Error)
			}
		}

		t.Logf("Creating volume %s...", volName)
		if _, err := cli.CreateVolume(ctx, volName, "local", nil, nil); err != nil {
			t.Fatalf("Failed to create volume: %v", err)
		}

		t.Logf("Starting container %s...", containerName)
		// Use raw client for StartContainer to ensure we can pass volume mounts correctly,
		// as the high-level client options for volumes might vary or be implicit.
		// The containerztest/client package is preferred, but for specific volume mounting
		// we fall back to raw gNOI to be explicit and safe.
		rawContClient := dut.RawAPIs().GNOI(t).Containerz()
		if _, err := rawContClient.StartContainer(ctx, &cpb.StartContainerRequest{
			InstanceName: containerName,
			ImageName:    imageName,
			Tag:          tag,
			Cmd:          "tail -f /dev/null", // Keep alive
			Volumes: []*cpb.Volume{
				{
					Name:       volName,
					MountPoint: "/tmp",
					ReadOnly:   true,
				},
			},
		}); err != nil {
			t.Fatalf("Failed to start container: %v", err)
		}

		if err := verifyContainerState(ctx, cli, containerName, cpb.ListContainerResponse_RUNNING); err != nil {
			t.Fatalf("Container not running after start: %v", err)
		}
		if err := verifyVolumeExists(ctx, cli, volName); err != nil {
			t.Fatalf("Volume not found after creation: %v", err)
		}
	})

	t.Run("Switchover", func(t *testing.T) {
		t.Logf("Switching control processor to %s...", standbyRP)
		switchReq := &gnoisystem.SwitchControlProcessorRequest{
			ControlProcessor: &gnoitypes.Path{
				Elem: []*gnoitypes.PathElem{{Name: standbyRP}},
			},
		}

		// The switchover request is expected to cause a connection drop as the device reboots/switches.
		// We log the error but proceed to wait for the system to come back.
		if _, err := sysClient.SwitchControlProcessor(ctx, switchReq); err != nil {
			t.Logf("SwitchControlProcessor returned error (expected due to reboot): %v", err)
		}
	})

	t.Run("VerifyRecovery", func(t *testing.T) {
		t.Log("Waiting for DUT to reconnect...")
		// Allow some time for the switchover to initiate and the connection to drop.
		time.Sleep(30 * time.Second)

		if err := waitForReboot(t, dut); err != nil {
			t.Fatalf("DUT failed to recover after switchover: %v", err)
		}

		// Refresh clients after reconnection.
		cli = containerztest.Client(t, dut)

		t.Log("Verifying container recovery...")
		if err := verifyContainerStateEventually(ctx, cli, containerName, cpb.ListContainerResponse_RUNNING); err != nil {
			t.Errorf("Container recovery failed: %v", err)
		}

		t.Log("Verifying volume persistence...")
		if err := verifyVolumeExists(ctx, cli, volName); err != nil {
			t.Errorf("Volume persistence failed: %v", err)
		}
	})
}

// findStandbyRP identifies the secondary/standby control processor.
func findStandbyRP(t *testing.T, dut *ondatra.DUTDevice) (string, error) {
	t.Helper()
	comps := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	for _, c := range comps {
		if c.GetRedundantRole() == oc.Platform_ComponentRedundantRole_SECONDARY {
			return c.GetName(), nil
		}
	}
	return "", fmt.Errorf("no standby control processor found")
}

// verifyContainerState checks if a container exists and is in the expected state.
func verifyContainerState(ctx context.Context, cli *client.Client, name string, want cpb.ListContainerResponse_Status) error {
	// Use the client's ListContainer with filter.
	listCh, err := cli.ListContainer(ctx, true, 0, map[string][]string{"name": {name}})
	if err != nil {
		return fmt.Errorf("ListContainer failed: %w", err)
	}

	found := false
	for cnt := range listCh {
		if cnt.Error != nil {
			return fmt.Errorf("error listing containers: %w", cnt.Error)
		}
		// Handle potential leading slash in name returned by some implementations.
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

// verifyContainerStateEventually polls for the container to reach the expected state.
func verifyContainerStateEventually(ctx context.Context, cli *client.Client, name string, want cpb.ListContainerResponse_Status) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("container %s did not reach state %v within timeout", name, want)
		case <-ticker.C:
			if err := verifyContainerState(ctx, cli, name, want); err == nil {
				return nil
			}
		}
	}
}

// verifyVolumeExists checks if a volume exists.
func verifyVolumeExists(ctx context.Context, cli *client.Client, name string) error {
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

// waitForReboot waits for the DUT to become reachable via gNMI.
func waitForReboot(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	start := time.Now()
	for time.Since(start) < switchoverWait {
		// Check a basic path like System CurrentDatetime to verify gNMI connectivity.
		// Await with a short timeout allows us to poll effectively.
		_, err := gnmi.Lookup(t, dut, gnmi.OC().System().CurrentDatetime().State()).Await(10 * time.Second)
		if err == nil {
			return nil
		}
		// If error, it means we are likely still rebooting/connecting.
		time.Sleep(10 * time.Second)
	}
	return fmt.Errorf("DUT did not come back online after %v", switchoverWait)
}
