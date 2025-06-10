package container_lifecycle_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"

	"github.com/openconfig/containerz/client"

	cpb "github.com/openconfig/gnoi/containerz"
)

var (
	containerTar        = flag.String("container_tar", "/tmp/cntrsrv.tar", "The container tarball to deploy.")
	containerUpgradeTar = flag.String("container_upgrade_tar", "/tmp/cntrsrv-upgrade.tar", "The container tarball to upgrade to.")
	pluginTar           = flag.String("plugin_tar", "/tmp/rootfs.tar.gz", "The plugin tarball (e.g., for vieux/docker-volume-sshfs rootfs.tar.gz).")
	pluginConfig        = flag.String("plugin_config", "testdata/test_sshfs_config.json", "The plugin config.")
)

const (
	instanceName = "test-instance"
	imageName    = "cntrsrv"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func containerzClient(ctx context.Context, t *testing.T) *client.Client {

	dut := ondatra.DUT(t, "dut")
	switch dut.Vendor() {
	case ondatra.ARISTA:
		if deviations.ContainerzOCUnsupported(dut) {
			dut.Config().New().WithAristaText(`
				management api gnoi
				service containerz
				  transport gnmi default
				  !
				  container runtime
					 vrf default
				!
			`).Append(t)
		}
	default:
		t.Fatalf("dut %s does not support containerz", dut.Name())
	}

	t.Logf("Waiting for device to ingest its config.")
	time.Sleep(time.Minute)

	return client.NewClientFromStub(dut.RawAPIs().GNOI(t).Containerz())
}

// StartContainerOptions holds parameters for starting a container.
type StartContainerOptions struct {
	ImageName           string
	ImageTag            string
	TarPath             string
	InstanceName        string
	Command             string
	Ports               []string
	RemoveExistingImage bool
	PollForRunningState bool
	PollTimeout         time.Duration
	PollInterval        time.Duration
}

// withDefaults returns a new StartContainerOptions with default values applied
// for fields that were zero-valued in the original options.
func (o StartContainerOptions) withDefaults() StartContainerOptions {
	res := o // Create a copy

	if res.ImageName == "" {
		res.ImageName = imageName
	}
	if res.ImageTag == "" {
		res.ImageTag = "latest"
	}
	if res.TarPath == "" {
		res.TarPath = *containerTar
	}
	if res.InstanceName == "" {
		res.InstanceName = instanceName
	}
	if res.Command == "" {
		res.Command = "./cntrsrv"
	}
	if len(res.Ports) == 0 {
		res.Ports = []string{"60061:60061"} // Default port
	}
	if res.PollTimeout == 0 {
		res.PollTimeout = 30 * time.Second
	}
	if res.PollInterval == 0 {
		res.PollInterval = 5 * time.Second // Also used for fixed sleep if not polling
	}
	// Boolean fields (RemoveExistingImage, PollForRunningState) default to false (their zero value).
	return res
}

// deployAndStartContainer sets up and starts a container according to the provided options.
// It ensures the image is present (pushing it if necessary) and the container is started.
func deployAndStartContainer(ctx context.Context, t *testing.T, cli *client.Client, opts StartContainerOptions) error {
	// Mark as test helper
	t.Helper()

	// Apply default values for any unspecified options.
	opts = opts.withDefaults()

	// 1. Remove existing container instance to ensure a clean start.
	t.Logf("Attempting to remove existing container instance %s before start.", opts.InstanceName)
	if err := cli.RemoveContainer(ctx, opts.InstanceName, true); err != nil {
		if status.Code(err) != codes.NotFound {
			t.Logf("Pre-start removal of container %s failed: %v", opts.InstanceName, err)
		} else {
			t.Logf("Container instance %s was not found or successfully removed.", opts.InstanceName)
		}
	}

	// 2. Optionally remove existing image before push.
	if opts.RemoveExistingImage {
		t.Logf("Attempting to remove existing image %s:%s before push.", opts.ImageName, opts.ImageTag)
		if err := cli.RemoveImage(ctx, opts.ImageName, opts.ImageTag, false); err != nil {
			s, _ := status.FromError(err)
			if s.Code() != codes.NotFound && err.Error() != client.ErrNotFound.Error() {
				t.Logf("Pre-push removal of image %s:%s failed (continuing with push): %v", opts.ImageName, opts.ImageTag, err)
			} else {
				t.Logf("Image %s:%s was not found or successfully removed before push.", opts.ImageName, opts.ImageTag)
			}
		}
	}

	// 3. Push the image.
	t.Logf("Pushing image %s:%s from %s.", opts.ImageName, opts.ImageTag, opts.TarPath)
	progCh, err := cli.PushImage(ctx, opts.ImageName, opts.ImageTag, opts.TarPath, false)
	if err != nil {
		t.Fatalf("Initial call to PushImage for %s:%s failed: %v", opts.ImageName, opts.ImageTag, err)
	}
	for prog := range progCh {
		if prog.Error != nil {
			t.Fatalf("Error during push of image %s:%s: %v", opts.ImageName, opts.ImageTag, prog.Error)
		}
		if prog.Finished {
			t.Logf("Successfully pushed image %s:%s.", prog.Image, prog.Tag)
		} else {
			t.Logf("Push progress for %s:%s: %d bytes received.", opts.ImageName, opts.ImageTag, prog.BytesReceived)
		}
	}

	// 4. Verify the image exists after push.
	t.Logf("Verifying image %s:%s exists after push.", opts.ImageName, opts.ImageTag)
	imgListCh, err := cli.ListImage(ctx, 0, map[string][]string{"name": {opts.ImageName}, "tag": {opts.ImageTag}})
	if err != nil {
		t.Fatalf("Failed to list images after push for %s:%s: %v", opts.ImageName, opts.ImageTag, err)
	}
	foundImage := false
	for img := range imgListCh {
		if img.Error != nil {
			t.Fatalf("Error received during ListImage iteration for %s:%s: %v", opts.ImageName, opts.ImageTag, img.Error)
		}
		if img.ImageName == opts.ImageName && img.ImageTag == opts.ImageTag {
			foundImage = true
			break
		}
	}
	if !foundImage {
		t.Fatalf("Image %s:%s not found after successful push.", opts.ImageName, opts.ImageTag)
	}
	t.Logf("Image %s:%s verified successfully after push.", opts.ImageName, opts.ImageTag)

	// 5. Start the container.
	t.Logf("Starting container %s with image %s:%s, command '%s', ports %v.", opts.InstanceName, opts.ImageName, opts.ImageTag, opts.Command, opts.Ports)
	startResp, err := cli.StartContainer(ctx, opts.ImageName, opts.ImageTag, opts.Command, opts.InstanceName, client.WithPorts(opts.Ports))
	if err != nil {
		t.Fatalf("Unable to start container %s: %v", opts.InstanceName, err)
	}
	t.Logf("StartContainer called for %s, response: %s", opts.InstanceName, startResp)

	// 6. Wait for container to be running or fixed sleep.
	if opts.PollForRunningState {
		t.Logf("Polling for container %s to reach RUNNING state (timeout: %v, interval: %v).", opts.InstanceName, opts.PollTimeout, opts.PollInterval)
		startTime := time.Now()
		for time.Since(startTime) < opts.PollTimeout {
			listContCh, listErr := cli.ListContainer(ctx, true, 0, map[string][]string{"name": {opts.InstanceName}})
			if listErr != nil {
				return fmt.Errorf("unable to list container %s state during polling: %w", opts.InstanceName, listErr)
			}
			containerIsRunning := false
			for info := range listContCh {
				if info.Error != nil {
					return fmt.Errorf("error message received while listing container %s during polling: %w", opts.InstanceName, info.Error)
				}
				if (info.Name == opts.InstanceName || info.Name == "/"+opts.InstanceName) && info.State == cpb.ListContainerResponse_RUNNING.String() {
					t.Logf("Container %s confirmed RUNNING.", opts.InstanceName)
					containerIsRunning = true
					break
				}
			}
			if containerIsRunning {
				return nil
			}
			time.Sleep(opts.PollInterval)
		}
		return fmt.Errorf("container %s did not reach RUNNING state within %v", opts.InstanceName, opts.PollTimeout)
	}
	// Original behavior: fixed sleep. Use PollInterval as the sleep duration for simplicity.
	t.Logf("Waiting for %v for container %s to stabilize (fixed sleep, no polling).", opts.PollInterval, opts.InstanceName)
	time.Sleep(opts.PollInterval)
	return nil
}

// startContainer sets up and starts the default test container.
// It returns the client. It calls t.Fatalf on failure.
func startContainer(ctx context.Context, t *testing.T) *client.Client {
	t.Helper()
	cli := containerzClient(ctx, t)

	opts := StartContainerOptions{
		// Defaults will be used for ImageName, ImageTag, TarPath, InstanceName, Command, Ports.
		RemoveExistingImage: false,
		PollForRunningState: false,
		PollInterval:        5 * time.Second,
	}

	if err := deployAndStartContainer(ctx, t, cli, opts); err != nil {
		t.Fatalf("Failed to start default container: %v", err)
	}
	return cli
}

func stopContainer(ctx context.Context, t *testing.T, cli *client.Client, instNameToStop string) {
	t.Helper()
	t.Logf("Attempting to stop container %s", instNameToStop)
	if err := cli.StopContainer(ctx, instNameToStop, true); err != nil {
		s, _ := status.FromError(err)
		if s.Code() == codes.NotFound {
			t.Logf("StopContainer: Container %s not found (may have already been stopped and removed): %v", instNameToStop, err)
		} else {
			t.Logf("StopContainer for %s encountered an issue: %v", instNameToStop, err)
		}
	} else {
		t.Logf("Container %s stopped successfully.", instNameToStop)
	}
}

// TestDeployAndStartContainer implements CNTR-1.1 validating that it is
// possible deploy and start a container via containerz.
func TestDeployAndStartContainer(t *testing.T) {
	ctx := context.Background()

	// Positive test: Deploy and start a container successfully.
	t.Run("SuccessfulDeployAndStart", func(t *testing.T) {
		cli := containerzClient(ctx, t)
		opts := StartContainerOptions{
			InstanceName:        instanceName,
			ImageName:           imageName,
			ImageTag:            "latest",
			TarPath:             *containerTar,
			Command:             "./cntrsrv",
			Ports:               []string{"60061:60061"},
			RemoveExistingImage: true,
			PollForRunningState: true,
			PollTimeout:         30 * time.Second,
			PollInterval:        5 * time.Second,
		}

		if err := deployAndStartContainer(ctx, t, cli, opts); err != nil {
			t.Fatalf("Failed to deploy and start container %s: %v", opts.InstanceName, err)
		}
		defer stopContainer(ctx, t, cli, opts.InstanceName)
		t.Logf("Container %s successfully started and running (verified by deployAndStartContainer).", opts.InstanceName)
	})

	// Negative Test: Attempt to start container with a non-existent image
	t.Run("StartWithNonExistentImage", func(t *testing.T) {
		nonExistentImageName := "non-existent-image"
		instanceName := "test-non-existent-img"
		cli := containerzClient(ctx, t) // Get client for this subtest.
		if _, err := cli.StartContainer(ctx, nonExistentImageName, "latest", "./cmd", instanceName, client.WithPorts([]string{"60061:60061"})); err == nil {
			t.Errorf("Expected error when starting container with non-existent image %s, but got nil", nonExistentImageName)
			// Attempt to clean up if it somehow started
			if removeErr := cli.RemoveContainer(ctx, instanceName, true); removeErr != nil {
				t.Logf("Cleanup: Failed to remove container: %s after unexpected start: %v", instanceName, removeErr)
			}
		} else {
			t.Logf("Got expected error when starting with non-existent image: %v", err)
		}
	})

	// Negative Test: Attempt to start container with an existing image but non-existent tag
	t.Run("StartWithNonExistentTag", func(t *testing.T) {
		// Ensure the base image exists (pushed in the positive test or a previous run)
		// If not, this test might give a false positive for "image not found" instead of "tag not found".
		// For simplicity, we assume 'imageName' ("cntrsrv") with 'latest' tag was pushed.
		nonExistentTag := "non-existent-tag"
		instanceName := "test-non-existent-tag"
		cli := containerzClient(ctx, t)
		if _, err := cli.StartContainer(ctx, imageName, nonExistentTag, "./cmd", instanceName, client.WithPorts([]string{"60061:60061"})); err == nil {
			t.Errorf("Expected error when starting container %s with non-existent tag %s, but got nil", imageName, nonExistentTag)
			if removeErr := cli.RemoveContainer(ctx, instanceName, true); removeErr != nil {
				t.Logf("Cleanup: Failed to remove container: %s after unexpected start: %v", instanceName, removeErr)
			}
		} else {
			t.Logf("Got expected error when starting with non-existent tag: %v", err)
		}
	})
}

// TestRetrieveLogs implements CNTR-1.2 validating that logs can be retrieved from a
// running container.
func TestRetrieveLogs(t *testing.T) {
	ctx := context.Background()
	baseCli := containerzClient(ctx, t)

	// Positive Test: Retrieve logs from a running container
	t.Run("SuccessfulLogRetrieval", func(t *testing.T) {
		localStartedCli := startContainer(ctx, t)
		defer stopContainer(ctx, t, localStartedCli, instanceName) // Stops default 'instanceName'

		logCh, err := localStartedCli.Logs(ctx, instanceName, false)
		if err != nil {
			t.Errorf("Logs() for running instance %s failed: %v", instanceName, err)
			return
		}
		if logCh == nil {
			t.Fatalf("Logs() for running instance %s returned nil channel with nil error", instanceName)
		}

		var logs []string
		for msg := range logCh {
			if msg.Error != nil {
				t.Errorf("Logs() for running instance %s stream returned an error: %v", instanceName, msg.Error)
				break
			}
			logs = append(logs, msg.Msg)
		}

		if len(logs) == 0 {
			t.Errorf("No logs were returned for running instance %s", instanceName)
		} else {
			t.Logf("Retrieved %d log lines for %s. First line (sample): %s", len(logs), instanceName, logs[0])
		}
	})

	// Negative Test: Attempt to retrieve logs from a non-existent container instance
	t.Run("LogsFromNonExistentInstance", func(t *testing.T) {
		nonExistentInstanceName := "test-instance-log-does-not-exist"
		// Ensure it really doesn't exist
		if err := baseCli.RemoveContainer(ctx, nonExistentInstanceName, true); err != nil && status.Code(err) != codes.NotFound {
			t.Logf("Pre-check RemoveContainer for %s failed (continuing): %v", nonExistentInstanceName, err)
		}

		logCh, err := baseCli.Logs(ctx, nonExistentInstanceName, false)
		if err != nil {
			// Case 1: Logs() itself returns an error
			t.Logf("Got expected error when retrieving logs for non-existent instance %s: %v", nonExistentInstanceName, err)
			s, _ := status.FromError(err)
			if s.Code() != codes.NotFound && s.Code() != codes.Unknown {
				t.Errorf("Expected codes.NotFound or codes.Unknown for non-existent instance %s, but got %s.", nonExistentInstanceName, s.Code())
			}
			if logCh != nil {
				t.Errorf("Expected nil logCh when cli.Logs returns an error for non-existent instance %s, but got %v", nonExistentInstanceName, logCh)
			}
			return
		}

		// Case 2: Logs() returns (channel, nil). Error should come via channel.
		if logCh == nil {
			t.Fatalf("cli.Logs for non-existent instance %s returned nil channel and nil error, expected error via channel or direct error.", nonExistentInstanceName)
		}

		t.Logf("cli.Logs for non-existent instance %s returned nil error, expecting error via channel.", nonExistentInstanceName)

		// Timeout for receiving from the channel.
		const channelReadTimeout = 10 * time.Second
		timer := time.NewTimer(channelReadTimeout)
		defer timer.Stop()

		select {
		case msg, ok := <-logCh:
			if !ok {
				// Channel was closed without sending any message.
				t.Errorf("Expected an error message on the log channel for non-existent instance %s, but channel closed without sending a message.", nonExistentInstanceName)
			} else {
				// A message was received.
				if msg.Error != nil {
					t.Logf("Got expected error from log channel for non-existent instance %s: %v", nonExistentInstanceName, msg.Error)
					s, _ := status.FromError(msg.Error)
					if s.Code() != codes.NotFound && s.Code() != codes.Unknown {
						t.Errorf("Expected codes.NotFound or codes.Unknown from channel for non-existent instance %s, but got %s.", nonExistentInstanceName, s.Code())
					}
				} else {
					// An actual log message was received, which is an error for this test case.
					t.Errorf("Received unexpected log message '%s' for non-existent instance %s when expecting an error.", msg.Msg, nonExistentInstanceName)
				}
			}
		case <-timer.C:
			// Timeout occurred.
			t.Errorf("Timed out waiting for a message (expected error) on the log channel for non-existent instance %s after %v.", nonExistentInstanceName, channelReadTimeout)
		}
	})

	// Negative Test: Attempt to retrieve logs from a stopped container instance.
	t.Run("LogsFromStoppedInstance", func(t *testing.T) {
		stoppedInstanceName := "test-instance-for-stopped-logs"
		localImageName := imageName

		defer func() {
			if err := baseCli.RemoveContainer(ctx, stoppedInstanceName, true); err != nil && status.Code(err) != codes.NotFound {
				t.Logf("Cleanup: Failed to remove container %s: %v", stoppedInstanceName, err)
			}
		}()

		opts := StartContainerOptions{
			InstanceName:        stoppedInstanceName,
			ImageName:           localImageName,
			ImageTag:            "latest",
			TarPath:             *containerTar,
			Command:             "./cntrsrv",
			Ports:               []string{"60062:60062"},
			RemoveExistingImage: false,
			PollForRunningState: true,
			PollTimeout:         30 * time.Second,
			PollInterval:        3 * time.Second,
		}

		if err := deployAndStartContainer(ctx, t, baseCli, opts); err != nil {
			t.Fatalf("Failed to set up container %s for stopped log test: %v", stoppedInstanceName, err)
		}
		t.Logf("Container %s started for stopped log test.", stoppedInstanceName)

		// Stop the container.
		if err := baseCli.StopContainer(ctx, stoppedInstanceName, true); err != nil {
			t.Fatalf("Failed to stop container %s for stopped log test: %v", stoppedInstanceName, err)
		}
		t.Logf("Container %s stopped.", stoppedInstanceName)
		// Allow time for stop to process.
		time.Sleep(3 * time.Second)

		// 5. Attempt to retrieve logs.
		logCh, err := baseCli.Logs(ctx, stoppedInstanceName, false)
		if err != nil {
			// Case 1: Logs() itself returns an error.
			t.Logf("Got expected error when retrieving logs for stopped instance %s: %v", stoppedInstanceName, err)
			s, ok := status.FromError(err)
			if !ok {
				t.Errorf("Error for stopped instance %s was not a gRPC status error: %v", stoppedInstanceName, err)
			} else if s.Code() != codes.NotFound && s.Code() != codes.FailedPrecondition && s.Code() != codes.Unknown {
				// Allow Unknown as some systems might report it this way, similar to non-existent.
				t.Errorf("Expected codes.NotFound, codes.FailedPrecondition, or codes.Unknown for stopped instance %s, but got %s.", stoppedInstanceName, s.Code())
			}
			if logCh != nil {
				t.Errorf("Expected nil logCh when cli.Logs returns an error for stopped instance %s, but got %v", stoppedInstanceName, logCh)
			}
			return // Test finished for this path
		}

		// Case 2: Logs() returns (channel, nil). Error should come via channel.
		if logCh == nil {
			t.Fatalf("cli.Logs for stopped instance %s returned nil channel and nil error, expected error via channel or direct error.", stoppedInstanceName)
		}

		t.Logf("cli.Logs for stopped instance %s returned (channel, nil). Checking channel for error or successful completion.", stoppedInstanceName)
		foundErrorOnChannel := false
		var receivedLogs []string
		for msg := range logCh {
			if msg.Error != nil {
				t.Logf("Got expected error from log channel for stopped instance %s: %v", stoppedInstanceName, msg.Error)
				s, ok := status.FromError(msg.Error)
				if !ok {
					t.Errorf("Stream error for stopped instance %s was not a gRPC status error: %v", stoppedInstanceName, msg.Error)
				} else if s.Code() != codes.NotFound && s.Code() != codes.FailedPrecondition && s.Code() != codes.Unknown {
					t.Errorf("Expected codes.NotFound, codes.FailedPrecondition, or codes.Unknown from channel for stopped instance %s, but got %s.", stoppedInstanceName, s.Code())
				}
				foundErrorOnChannel = true
				break
			}
			// If no error, it might be an actual log message from before the container stopped.
			receivedLogs = append(receivedLogs, msg.Msg)
		}

		if !foundErrorOnChannel {
			t.Logf("For stopped instance %s, cli.Logs() did not return an initial error, and the log channel closed without an error message. Received %d log lines. This behavior is noted.", stoppedInstanceName, len(receivedLogs))
		}
	})
}

// TestListContainers implements CNTR-1.3 validating listing running containers.
func TestListContainers(t *testing.T) {
	ctx := context.Background()
	baseCli := containerzClient(ctx, t)

	t.Run("ListWhenTargetContainerIsNotRunning", func(t *testing.T) {
		// Ensure our main test container 'instanceName' is not running.
		// Handle cleanup if it was left over from a previous failed test.
		if err := baseCli.RemoveContainer(ctx, instanceName, true); err != nil {
			if status.Code(err) != codes.NotFound {
				// Log as a warning, as the container might not have existed, which is the desired state.
				t.Logf("Pre-test removal of %s encountered an issue (continuing test, desired state is 'not found'): %v", instanceName, err)
			}
		}
		// Allow time for removal to propagate if it occurred.
		time.Sleep(2 * time.Second)
		// List all containers
		listCh, err := baseCli.ListContainer(ctx, true, 0, nil)
		if err != nil {
			t.Fatalf("ListContainer() failed when target container %s should not be running: %v", instanceName, err)
		}

		foundOurInstance := false
		var allListedContainers []string
		for cnt := range listCh {
			if cnt.Error != nil {
				t.Errorf("Error received during ListContainer iteration: %v", cnt.Error)
				continue // Skip this entry and check others
			}
			allListedContainers = append(allListedContainers, cnt.Name+":"+cnt.ImageName)
			if cnt.Name == instanceName {
				foundOurInstance = true
			}
		}

		if foundOurInstance {
			t.Errorf("ListContainer() found instance %q when it should not be present. All listed containers: %v", instanceName, allListedContainers)
		} else {
			t.Logf("Instance %q correctly not found by ListContainer. All listed containers: %v", instanceName, allListedContainers)
		}
	})

	t.Run("ListFindsSpecificRunningContainer", func(t *testing.T) {
		// startContainer will ensure 'instanceName' with 'imageName:latest' is running.
		localStartedCli := startContainer(ctx, t)
		defer stopContainer(ctx, t, localStartedCli, instanceName)

		listCh, err := localStartedCli.ListContainer(ctx, true, 0, nil)
		if err != nil {
			t.Fatalf("ListContainer() failed: %v", err)
		}

		wantImg := imageName + ":latest"
		foundWantImgAndInstance := false
		var listedContainersForDebug []string

		for cnt := range listCh {
			if cnt.Error != nil {
				t.Errorf("Error received during ListContainer iteration: %v", cnt.Error)
				continue
			}
			listedContainersForDebug = append(listedContainersForDebug, cnt.Name+":"+cnt.ImageName)
			if cnt.ImageName == wantImg && strings.TrimPrefix(cnt.Name, "/") == instanceName {
				foundWantImgAndInstance = true
			}
		}

		if !foundWantImgAndInstance {
			t.Errorf("ListContainer() did not find the expected container instance %q with image %q. All listed: %v", instanceName, wantImg, listedContainersForDebug)
		} else {
			t.Logf("Successfully found instance %q with image %q.", instanceName, wantImg)
		}
	})
}

// TestStopContainer implements CNTR-1.4 validating that stopping a container works as expected.
func TestStopContainer(t *testing.T) {
	ctx := context.Background()
	baseCli := containerzClient(ctx, t)

	t.Run("StopRunningContainer", func(t *testing.T) {
		localStartedCli := startContainer(ctx, t)
		if err := localStartedCli.StopContainer(ctx, instanceName, true); err != nil {
			t.Fatalf("StopContainer() for running instance %s failed: %v", instanceName, err)
		}
		t.Logf("StopContainer called for %s", instanceName)

		// Allow time for container to stop.
		time.Sleep(5 * time.Second)

		listCh, err := localStartedCli.ListContainer(ctx, true, 0, map[string][]string{"name": {instanceName}})
		if err != nil {
			t.Errorf("ListContainer() after stopping %s failed: %v", instanceName, err)
			return
		}

		var foundContainers []string
		for cntr := range listCh {
			if cntr.Error != nil {
				t.Errorf("Error received during ListContainer iteration for %s: %v", instanceName, cntr.Error)
				continue
			}
			// Check if the specific instanceName is still listed.
			if strings.TrimPrefix(cntr.Name, "/") == instanceName && cntr.State != cpb.ListContainerResponse_STOPPED.String() {
				foundContainers = append(foundContainers, cntr.Name+":"+cntr.ImageName)
			}
		}
		if len(foundContainers) > 0 {
			t.Errorf("StopContainer() did not stop the container %s. Found running: %v", instanceName, foundContainers)
		} else {
			t.Logf("Container %s successfully stopped and not listed.", instanceName)
		}
	})

	t.Run("StopNonExistentContainer", func(t *testing.T) {
		nonExistentInstance := "test-instance-does-not-exist-for-stop"
		// Ensure it's not running (best effort cleanup)
		if err := baseCli.RemoveContainer(ctx, nonExistentInstance, true); err != nil {
			if status.Code(err) != codes.NotFound {
				t.Logf("Pre-check RemoveContainer for %s failed (continuing): %v", nonExistentInstance, err)
			}
		}
		// Allow time for removal to settle if it happened.
		time.Sleep(5 * time.Second)

		if err := baseCli.StopContainer(ctx, nonExistentInstance, true); err == nil {
			t.Errorf("StopContainer() for non-existent instance %s succeeded, but expected an error (e.g., NotFound)", nonExistentInstance)
		} else {
			t.Logf("Got expected error when stopping non-existent instance %s: %v", nonExistentInstance, err)
			s, _ := status.FromError(err)
			if s.Code() != codes.NotFound {
				t.Logf("Warning: StopContainer for non-existent instance %s returned code %s, not codes.NotFound. This might be acceptable depending on server behavior.", nonExistentInstance, s.Code())
			}
		}
	})

	t.Run("StopAlreadyStoppedContainer", func(t *testing.T) {
		// Use startContainer to set up a container, then stop it.
		localStartedCli := startContainer(ctx, t)
		if err := localStartedCli.StopContainer(ctx, instanceName, true); err != nil {
			t.Fatalf("Initial StopContainer() for %s failed: %v", instanceName, err)
		}
		t.Logf("Container %s stopped once.", instanceName)
		// Allow time for the first stop to fully process.
		time.Sleep(5 * time.Second)
		// Attempt to stop it again.
		if err := localStartedCli.StopContainer(ctx, instanceName, true); err != nil {
			s, _ := status.FromError(err)
			if s.Code() == codes.NotFound {
				t.Logf("Second StopContainer() for %s returned NotFound, which is acceptable: %v", instanceName, err)
			} else {
				t.Errorf("Second StopContainer() for already stopped instance %s failed unexpectedly: %v", instanceName, err)
			}
		} else {
			t.Logf("Second StopContainer() for already stopped instance %s succeeded (no-op), which is acceptable.", instanceName)
		}
		if err := localStartedCli.RemoveContainer(ctx, instanceName, true); err != nil && status.Code(err) != codes.NotFound {
			t.Logf("Cleanup: Failed to remove container %s after StopAlreadyStoppedContainer test: %v", instanceName, err)
		}
	})
}

// TestVolumes implements CNTR-1.5 validating that volumes can be created or removed, it does not test
// if they can actually be used.
func TestVolumes(t *testing.T) {
	ctx := context.Background()
	cli := containerzClient(ctx, t)
	volumeName := "test-vol-positive"

	// Positive Test: Create, List, and Remove a volume successfully
	t.Run("CreateListRemoveVolume", func(t *testing.T) {
		// Ensure the volume doesn't exist from a previous run
		if err := cli.RemoveVolume(ctx, volumeName, true); err != nil {
			if status.Code(err) != codes.NotFound {
				t.Logf("Pre-test RemoveVolume for %s failed (continuing): %v", volumeName, err)
			}
		}
		// Allow time for removal to settle.
		time.Sleep(5 * time.Second)

		createdVolumeName, err := cli.CreateVolume(ctx, volumeName, "local", nil, nil)
		if err != nil {
			t.Fatalf("CreateVolume(%q, \"local\", nil, nil) failed: %v", volumeName, err)
		}
		if createdVolumeName != volumeName {
			t.Errorf("CreateVolume returned name %q, want %q", createdVolumeName, volumeName)
		}
		t.Logf("Successfully created volume %q", createdVolumeName)

		// List and Verify.
		volCh, err := cli.ListVolume(ctx, map[string][]string{"name": {volumeName}})
		if err != nil {
			t.Fatalf("ListVolume() after creating %q failed: %v", volumeName, err)
		}

		foundVolume := false
		var listedVolumes []*client.VolumeInfo
		for vol := range volCh {
			if vol.Error != nil {
				t.Errorf("Error received during ListVolume iteration for %q: %v", volumeName, vol.Error)
				continue
			}
			listedVolumes = append(listedVolumes, vol)
			if vol.Name == volumeName {
				foundVolume = true
				// Basic check for driver.
				if vol.Driver != "local" {
					t.Errorf("Volume %q has driver %q, want \"local\"", vol.Name, vol.Driver)
				}
				break
			}
		}
		if !foundVolume {
			t.Errorf("ListVolume() did not find the created volume %q. All listed: %v", volumeName, listedVolumes)
		} else {
			t.Logf("Successfully listed and verified volume %q.", volumeName)
		}

		if err := cli.RemoveVolume(ctx, volumeName, true); err != nil {
			t.Fatalf("RemoveVolume(%q) failed: %v", volumeName, err)
		}
		t.Logf("Successfully removed volume %q", volumeName)

		// Verify removal by listing again.
		volChVerify, errVerify := cli.ListVolume(ctx, map[string][]string{"name": {volumeName}})
		if errVerify != nil {
			t.Fatalf("ListVolume() after removing %q failed: %v", volumeName, errVerify)
		}
		for vol := range volChVerify {
			if vol.Name == volumeName {
				t.Errorf("Volume %q found by ListVolume() after it was supposed to be removed.", volumeName)
			}
		}
	})

	// Negative Test: Attempt to remove a non-existent volume.
	t.Run("RemoveNonExistentVolume", func(t *testing.T) {
		nonExistentVolumeName := "test-vol-does-not-exist"
		// Ensure it's truly non-existent.
		if err := cli.RemoveVolume(ctx, nonExistentVolumeName, true); err != nil {
			if status.Code(err) != codes.NotFound {
				t.Logf("Pre-check RemoveVolume for %q encountered an unexpected error (continuing test): %v", nonExistentVolumeName, err)
			} else {
				t.Logf("Pre-check RemoveVolume for %q confirmed it was not found.", nonExistentVolumeName)
			}
		} else {
			// Success (no-op) for pre-check removal is also fine.
			t.Logf("Pre-check RemoveVolume for %q succeeded (was a no-op), confirming it's not present.", nonExistentVolumeName)
		}
		time.Sleep(1 * time.Second)

		if err := cli.RemoveVolume(ctx, nonExistentVolumeName, true); err == nil {
			t.Logf("RemoveVolume(%q) for a non-existent volume succeeded (no-op), which is acceptable.", nonExistentVolumeName)
		} else {
			// An error was returned. It should be codes.NotFound.
			s, ok := status.FromError(err)
			if !ok || s.Code() != codes.NotFound {
				t.Errorf("RemoveVolume(%q) for a non-existent volume returned error %v, want a gRPC error with code NotFound", nonExistentVolumeName, err)
			} else {
				t.Logf("RemoveVolume(%q) for a non-existent volume correctly returned codes.NotFound.", nonExistentVolumeName)
			}
		}
	})
}

// TestUpgrade implements CNTR-1.6 validating that the container can be upgraded to the new version of the image
// identified by a different tag than the current running container image.
func TestUpgrade(t *testing.T) {
	ctx := context.Background()
	baseCli := containerzClient(ctx, t)

	// Positive Test: Successful upgrade
	t.Run("SuccessfulUpgrade", func(t *testing.T) {
		cli := startContainer(ctx, t)
		defer stopContainer(ctx, t, cli, instanceName)
		defer cli.RemoveImage(ctx, imageName, "upgrade", true)

		progCh, err := cli.PushImage(ctx, imageName, "upgrade", *containerUpgradeTar, false)
		if err != nil {
			t.Fatalf("unable to push image %s:upgrade: %v", imageName, err)
		}

		for prog := range progCh {
			switch {
			case prog.Error != nil:
				t.Fatalf("failed to push image %s:upgrade: %v", imageName, prog.Error)
			case prog.Finished:
				t.Logf("Pushed %s/%s for upgrade test\n", prog.Image, prog.Tag)
			default:
				t.Logf(" %d bytes pushed for %s:upgrade", prog.BytesReceived, imageName)
			}
		}

		if _, err := cli.UpdateContainer(ctx, imageName, "upgrade", "./cntrsrv", instanceName, false, client.WithPorts([]string{"60061:60061"})); err != nil {
			t.Fatalf("unable to upgrade container %s to %s:upgrade: %v", instanceName, imageName, err)
		}
		t.Logf("UpdateContainer called for %s to %s:upgrade", instanceName, imageName)
		// Allow time for upgrade to complete
		time.Sleep(5 * time.Second)

		listCh, err := cli.ListContainer(ctx, true, 0, map[string][]string{"name": {instanceName}})
		if err != nil {
			t.Fatalf("unable to list container %s after upgrade: %v", instanceName, err)
		}

		foundUpgraded := false
		expectedImage := imageName + ":upgrade"
		for cnt := range listCh {
			if cnt.Error != nil {
				t.Errorf("Error listing container %s: %v", instanceName, cnt.Error)
				continue
			}
			if (cnt.Name == instanceName || cnt.Name == "/"+instanceName) && cnt.ImageName == expectedImage && cnt.State == cpb.ListContainerResponse_RUNNING.String() {
				t.Logf("Container %s successfully upgraded to %s and is RUNNING.", instanceName, expectedImage)
				foundUpgraded = true
				break
			}
			t.Logf("Found container: Name=%s, Image=%s, State=%s", cnt.Name, cnt.ImageName, cnt.State)
		}

		if !foundUpgraded {
			t.Errorf("Container %s was not found running with image %s after upgrade attempt.", instanceName, expectedImage)
		}
	})

	// Negative Test: Upgrade to a non-existent image
	t.Run("UpgradeToNonExistentImage", func(t *testing.T) {
		cli := startContainer(ctx, t) // Starts 'instanceName' with 'imageName:latest'
		defer stopContainer(ctx, t, cli, instanceName)

		nonExistentImage := "non-existent-image-for-upgrade"
		if _, err := cli.UpdateContainer(ctx, nonExistentImage, "latest", "./cntrsrv", instanceName, false, client.WithPorts([]string{"60061:60061"})); err == nil {
			t.Errorf("UpdateContainer to non-existent image %s succeeded, expected error", nonExistentImage)
		} else {
			t.Logf("Got expected error when upgrading to non-existent image %s: %v", nonExistentImage, err)
			// Optionally, check for specific gRPC status code, e.g., codes.NotFound
			s, ok := status.FromError(err)
			if ok && s.Code() != codes.NotFound {
				t.Errorf("Expected codes.NotFound for non-existent image, got %s", s.Code())
			}
		}
	})

	// Negative Test: Upgrade to an existing image but non-existent tag.
	t.Run("UpgradeToNonExistentTag", func(t *testing.T) {
		cli := startContainer(ctx, t)
		defer stopContainer(ctx, t, cli, instanceName)

		nonExistentTag := "non-existent-tag-for-upgrade"
		// Ensure the base image 'imageName:latest' exists from startContainer.
		if _, err := cli.UpdateContainer(ctx, imageName, nonExistentTag, "./cntrsrv", instanceName, false, client.WithPorts([]string{"60061:60061"})); err == nil {
			t.Errorf("UpdateContainer to image %s with non-existent tag %s succeeded, expected error", imageName, nonExistentTag)
		} else {
			t.Logf("Got expected error when upgrading to image %s with non-existent tag %s: %v", imageName, nonExistentTag, err)
			s, ok := status.FromError(err)
			if ok && s.Code() != codes.NotFound {
				t.Errorf("Expected codes.NotFound (or similar) for non-existent tag, got %s", s.Code())
			}
		}
	})

	// Negative Test: Upgrade a non-existent container instance.
	t.Run("UpgradeNonExistentInstance", func(t *testing.T) {
		nonExistentInstance := "test-instance-does-not-exist-for-upgrade"
		// Ensure the instance is not running.
		if err := baseCli.RemoveContainer(ctx, nonExistentInstance, true); err != nil && status.Code(err) != codes.NotFound {
			t.Logf("Pre-test removal of %s failed (continuing): %v", nonExistentInstance, err)
		}

		if _, err := baseCli.UpdateContainer(ctx, imageName, "latest", "./cntrsrv", nonExistentInstance, false, client.WithPorts([]string{"60061:60061"})); err == nil {
			t.Errorf("UpdateContainer for non-existent instance %s succeeded, expected error", nonExistentInstance)
		} else {
			t.Logf("Got expected error when upgrading non-existent instance %s: %v", nonExistentInstance, err)
			s, ok := status.FromError(err)
			if ok && s.Code() != codes.NotFound {
				t.Errorf("Expected codes.NotFound for non-existent instance, got %s", s.Code())
			}
		}
	})
}

// pushPluginImage handles deploying a plugin tarball as a gNOI Containerz image.
func pushPluginImage(ctx context.Context, t *testing.T, cli *client.Client, pluginTarPath, pluginName, pluginImageTag string) error {
	t.Helper()
	t.Logf("Attempting to deploy plugin tarball %q as %s:%s", pluginTarPath, pluginName, pluginImageTag)
	// The 'true' argument indicates this is a plugin image.
	progCh, err := cli.PushImage(ctx, pluginName, pluginImageTag, pluginTarPath, true)

	if err != nil {
		return fmt.Errorf("PushImage (for plugin %q) failed: %w", pluginName, err)
	}

	// Monitor push progress.
	pushFinished := false
	for prog := range progCh {
		switch {
		case prog.Error != nil:
			return fmt.Errorf("PushImage (for plugin %q) reported error: %w", pluginName, prog.Error)
		case prog.Finished:
			t.Logf("Successfully pushed plugin %s:%s", pluginName, pluginImageTag)
			pushFinished = true
		default:
			t.Logf("Plugin %s:%s push progress: %d bytes pushed", pluginName, pluginImageTag, prog.BytesReceived)
		}
	}
	if !pushFinished {
		return fmt.Errorf("PushImage (for plugin %q) did not report finishing", pluginName)
	}
	return nil
}

// TestUpgrade implements CNTR-1.7 validating lifecycle of the SSHFS volume plugin via containerz.
// Prerequisites for running this test:
// 1. Build the rootfs.tar.gz for vieux/docker-volume-sshfs as per the README.
// 2. Set the --plugin_tar flag to the path of the generated rootfs.tar.gz.
func TestPlugins(t *testing.T) {
	ctx := context.Background()
	cli := containerzClient(ctx, t)
	// Common SSH parameters for plugin setup
	const (
		sshHost        = "localhost"
		sshUser        = "testuser"
		sshPassword    = "testpass"
		pluginImageTag = "latest"
	)

	// Check if the plugin tarball exists (as it's needed for config extraction).
	if _, err := os.Stat(*pluginTar); os.IsNotExist(err) {
		t.Fatalf("Plugin tarball %q not found. Build it from vieux/docker-volume-sshfs and specify path using --plugin_tar.", *pluginTar)
	}

	t.Run("SuccessfulPluginCompleteLifecycle", func(t *testing.T) {
		pluginName := "sshfs-plugin-positive"
		pluginInstance := "sshfs-instance-positive"

		defer func() {
			fullInstanceName := pluginInstance + ":" + pluginImageTag
			t.Logf("Cleanup SuccessfulPluginCompleteLifecycle: Stopping and removing plugin instance %s", fullInstanceName)
			if err := cli.StopPlugin(ctx, fullInstanceName); err != nil {
				t.Errorf("Cleanup SuccessfulPluginCompleteLifecycle: Error stopping plugin %q err: %v", fullInstanceName, err)
			}
			if err := cli.RemovePlugin(ctx, fullInstanceName); err != nil {
				t.Errorf("Cleanup SuccessfulPluginCompleteLifecycle: Error removing plugin %q err: %v", fullInstanceName, err)
			}
			t.Logf("Cleanup SuccessfulPluginCompleteLifecycle: Removing plugin image %s:%s", pluginName, pluginImageTag)
			if err := cli.RemoveImage(ctx, pluginName, pluginImageTag, true); err != nil {
				t.Logf("Cleanup SuccessfulPluginCompleteLifecycle: Error removing plugin image %q:%s (ignoring): %v", pluginName, pluginImageTag, err)
			}
		}()

		// Push the plugin image for this specific test case.
		if err := pushPluginImage(ctx, t, cli, *pluginTar, pluginName, pluginImageTag); err != nil {
			t.Fatalf("Failed to push plugin image %s:%s: %v", pluginName, pluginImageTag, err)
		}

		t.Logf("Attempting to start plugin %q instance %q with config %q", pluginName, pluginInstance, *pluginConfig)
		if err := cli.StartPlugin(ctx, pluginName, pluginInstance, *pluginConfig); err != nil {
			t.Fatalf("StartPlugin(%q, %q, %q) failed: %v", pluginName, pluginInstance, *pluginConfig, err)
		}
		t.Logf("StartPlugin call succeeded for instance %q", pluginInstance)

		const (
			retryInterval = 2 * time.Second
			maxRetries    = 5
		)
		found := false
		expectedFullInstanceName := pluginInstance + ":" + pluginImageTag
		// Adding some retries to allow time for Plugin to start.
		for i := 0; i < maxRetries; i++ {
			t.Logf("Attempting to list plugins to verify instance %q (attempt %d/%d)", expectedFullInstanceName, i+1, maxRetries)
			plugins, listErr := cli.ListPlugin(ctx, "")
			if listErr != nil {
				t.Logf("ListPlugin(\"\") failed on attempt %d: %v. Retrying in %v...", i+1, listErr, retryInterval)
				time.Sleep(retryInterval)
				continue
			}
			for _, p := range plugins {
				if p.GetInstanceName() == expectedFullInstanceName {
					t.Logf("Found running plugin via ListPlugin: Instance=%s", p.GetInstanceName())
					found = true
					break
				}
			}
			if found {
				break
			}
			t.Logf("Plugin instance %q not found in list on attempt %d. Retrying in %v...", expectedFullInstanceName, i+1, retryInterval)
			time.Sleep(retryInterval)
		}

		if !found {
			allPlugins, listAllErr := cli.ListPlugin(ctx, "")
			if listAllErr != nil {
				t.Errorf("Plugin instance %q not found after retries. Final attempt to list all plugins also failed: %v", expectedFullInstanceName, listAllErr)
			} else {
				t.Errorf("Plugin instance %q not found after retries. Current plugins: %v", expectedFullInstanceName, allPlugins)
			}
		} else {
			t.Logf("Successfully verified plugin instance %q is listed and running.", expectedFullInstanceName)
		}
	})

	t.Run("StartWithNonExistentPluginImage", func(t *testing.T) {
		pluginName := "non-existent-plugin-image"
		pluginInstance := "test-instance-non-existent-image"
		dummyConfigFile := filepath.Join(t.TempDir(), "dummy_config.json")
		if err := os.WriteFile(dummyConfigFile, []byte(`{"description":"dummy"}`), 0644); err != nil {
			t.Fatalf("Failed to write dummy config file: %v", err)
		}

		if err := cli.StartPlugin(ctx, pluginName, pluginInstance, dummyConfigFile); err == nil {
			t.Errorf("StartPlugin with non-existent image %q succeeded, expected error", pluginName)
			// Attempt cleanup if it somehow started.
			fullInstanceName := pluginInstance + ":" + pluginImageTag
			if err = cli.StopPlugin(ctx, fullInstanceName); err != nil {
				t.Logf("Cleanup StartWithNonExistentPluginImage: Error stopping plugin %q (ignoring): %v", fullInstanceName, err)
			}
			if err = cli.RemovePlugin(ctx, fullInstanceName); err != nil {
				t.Logf("Cleanup StartWithNonExistentPluginImage: Error removing plugin %q (ignoring): %v", fullInstanceName, err)
			}
		} else {
			t.Logf("Got expected error when starting with non-existent image %q: %v", pluginName, err)
			s, ok := status.FromError(err)
			if !ok || s.Code() != codes.Unknown {
				t.Errorf("Expected gRPC status codes.Unknown for non-existent image, got: %v (status code: %s)", err, s.Code())
			}
		}
	})

	t.Run("StartAlreadyStartedInstance", func(t *testing.T) {
		pluginName := "sshfs-plugin-already-started"
		pluginInstance := "sshfs-instance-already-started"

		defer func() {
			fullInstanceName := pluginInstance + ":" + pluginImageTag
			t.Logf("Cleanup StartAlreadyStartedInstance: Stopping and removing plugin instance %s", fullInstanceName)
			if err := cli.StopPlugin(ctx, fullInstanceName); err != nil {
				t.Logf("Cleanup StartAlreadyStartedInstance: Error stopping plugin %q (ignoring): %v", fullInstanceName, err)
			}
			if err := cli.RemovePlugin(ctx, fullInstanceName); err != nil {
				t.Logf("Cleanup StartAlreadyStartedInstance: Error removing plugin %q (ignoring): %v", fullInstanceName, err)
			}
			t.Logf("Cleanup StartAlreadyStartedInstance: Removing plugin image %s:%s", pluginName, pluginImageTag)
			if err := cli.RemoveImage(ctx, pluginName, pluginImageTag, true); err != nil {
				t.Logf("Cleanup StartAlreadyStartedInstance: Error removing plugin image %q:%s (ignoring): %v", pluginName, pluginImageTag, err)
			}
		}()

		// Push the plugin image for this specific test case.
		if err := pushPluginImage(ctx, t, cli, *pluginTar, pluginName, pluginImageTag); err != nil {
			t.Fatalf("Failed to push plugin image %s:%s for StartAlreadyStartedInstance: %v", pluginName, pluginImageTag, err)
		}

		// First start (should succeed).
		if err := cli.StartPlugin(ctx, pluginName, pluginInstance, *pluginConfig); err != nil {
			t.Fatalf("Initial StartPlugin for %s, instance %s failed: %v", pluginName, pluginInstance, err)
		}
		t.Logf("Successfully started plugin %s instance %s for the first time.", pluginName, pluginInstance)
		// Allow time for the plugin to stabilize if needed.
		time.Sleep(2 * time.Second)

		// Second start (should fail).
		if err := cli.StartPlugin(ctx, pluginName, pluginInstance, *pluginConfig); err == nil {
			t.Errorf("Second StartPlugin for already started instance %s succeeded, expected error", pluginInstance)
		} else {
			t.Logf("Got expected error when starting already started instance %s: %v", pluginInstance, err)
			s, ok := status.FromError(err)
			if !ok || s.Code() != codes.Unknown {
				t.Errorf("Expected gRPC status codes.Unknown for already started instance, got: %v (status code: %s)", err, s.Code())
			}
		}
	})
}
