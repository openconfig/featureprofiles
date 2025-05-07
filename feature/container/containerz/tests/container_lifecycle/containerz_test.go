package container_lifecycle_test

import (
	"context"
	"flag"
	"testing"
	"time"

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

func startContainer(ctx context.Context, t *testing.T) *client.Client {
	cli := containerzClient(ctx, t)

	// Call RemoveContainer here to make sure no other container is using the instanceName name or that instanceName
	// is a leftover from a previous failed test.
	if err := cli.RemoveContainer(ctx, instanceName, true); err != nil {
		if status.Code(err) != codes.NotFound {
			t.Fatalf("failed to remove container: %v", err)
		}
	}

	progCh, err := cli.PushImage(ctx, imageName, "latest", *containerTar, false)
	if err != nil {
		t.Fatalf("unable to push image: %v", err)
	}

	for prog := range progCh {
		switch {
		case prog.Error != nil:
			t.Fatalf("failed to push image: %v", prog.Error)
		case prog.Finished:
			t.Logf("Pushed %s/%s\n", prog.Image, prog.Tag)
		default:
			t.Logf(" %d bytes pushed", prog.BytesReceived)
		}
	}

	// Verify the image exists after push
	t.Logf("Verifying image %s:latest exists after push", imageName)
	imgListCh, err := cli.ListImage(ctx, 0, nil) // List all images
	if err != nil {
		t.Fatalf("Failed to list images after push: %v", err)
	}
	foundImage := false
	for img := range imgListCh {
		if img.Error != nil {
			t.Fatalf("Error received during ListImage iteration: %v", img.Error)
		}
		if img.ImageName == imageName {
			foundImage = true
			break
		}
	}
	if !foundImage {
		t.Fatalf("Image %s:latest not found after successful push.", imageName)
	}
	t.Logf("Image %s:latest verified.", imageName)

	ret, err := cli.StartContainer(ctx, imageName, "latest", "./cntrsrv", instanceName, client.WithPorts([]string{"60061:60061"}))
	if err != nil {
		t.Fatalf("unable to start container: %v", err)
	}

	t.Logf("Started %s", ret)

	time.Sleep(5 * time.Second)
	return cli
}

func stopContainer(ctx context.Context, t *testing.T, cli *client.Client) {
	if err := cli.StopContainer(ctx, instanceName, true); err != nil {
		t.Logf("container already stopping: %v", err)
	}
}

// TestDeployAndStartContainer implements CNTR-1.1 validating that it is
// possible deploy and start a container via containerz.
func TestDeployAndStartContainer(t *testing.T) {
	ctx := context.Background()
	cli := containerzClient(ctx, t)

	// Positive test: Deploy and start a container successfully.
	t.Run("SuccessfulDeployAndStart", func(t *testing.T) {
		// Ensure clean state for this sub-test
		if err := cli.RemoveContainer(ctx, instanceName, true); err != nil {
			if status.Code(err) != codes.NotFound {
				t.Logf("Pre-test removal of %s failed (continuing): %v", instanceName, err)
			}
		}
		// Remove image if it exists to ensure push is tested.
		if err := cli.RemoveImage(ctx, imageName, "latest", false); err != nil {
			if status.Code(err) != codes.NotFound && err.Error() != client.ErrNotFound.Error() {
				t.Logf("Pre-test removal of image %s:latest failed (continuing): %v", imageName, err)
			}
		}

		// Use a modified startContainer logic for this specific sub-test to avoid defer conflicts.
		progCh, err := cli.PushImage(ctx, imageName, "latest", *containerTar, false)
		if err != nil {
			t.Fatalf("unable to push image: %v", err)
		}
		for prog := range progCh {
			if prog.Error != nil {
				t.Fatalf("failed to push image: %v", prog.Error)
			}
		}

		_, err = cli.StartContainer(ctx, imageName, "latest", "./cntrsrv", instanceName, client.WithPorts([]string{"60061:60061"}))
		if err != nil {
			t.Fatalf("unable to start container: %v", err)
		}
		defer stopContainer(ctx, t, cli)

		for i := 0; i < 5; i++ {
			ch, err := cli.ListContainer(ctx, true, 0, map[string][]string{
				"name": {instanceName},
			})
			if err != nil {
				t.Fatalf("unable to list container state for %s: %v", instanceName, err)
			}

			for info := range ch {
				if info.Error != nil {
					t.Fatalf("unable to list containers: %v", info.Error)
				}
				if info.State == cpb.ListContainerResponse_RUNNING.String() {
					t.Logf("Container %s successfully started and running.", instanceName)
					return
				}
			}
			time.Sleep(5 * time.Second)
		}
		t.Fatalf("Container %s was not started.", instanceName)
	})

	// Negative Test: Attempt to start container with a non-existent image
	t.Run("StartWithNonExistentImage", func(t *testing.T) {
		nonExistentImageName := "non-existent-image"
		_, err := cli.StartContainer(ctx, nonExistentImageName, "latest", "./cmd", "test-non-existent-img", client.WithPorts([]string{"60061:60061"}))
		if err == nil {
			t.Errorf("Expected error when starting container with non-existent image %s, but got nil", nonExistentImageName)
			// Attempt to clean up if it somehow started
			cli.RemoveContainer(ctx, "test-non-existent-img", true)
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
		_, err := cli.StartContainer(ctx, imageName, nonExistentTag, "./cmd", "test-non-existent-tag", client.WithPorts([]string{"60061:60061"}))
		if err == nil {
			t.Errorf("Expected error when starting container %s with non-existent tag %s, but got nil", imageName, nonExistentTag)
			cli.RemoveContainer(ctx, "test-non-existent-tag", true)
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
		defer stopContainer(ctx, t, localStartedCli) // Stops 'instanceName'

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
		err := baseCli.RemoveContainer(ctx, nonExistentInstanceName, true)
		if err != nil && status.Code(err) != codes.NotFound {
			t.Logf("Pre-check RemoveContainer for %s failed (continuing): %v", nonExistentInstanceName, err)
		}

		logCh, err := baseCli.Logs(ctx, nonExistentInstanceName, false)
		if err != nil {
			// Case 1: Logs() itself returns an error
			t.Logf("Got expected error when retrieving logs for non-existent instance %s: %v", nonExistentInstanceName, err)
			s, _ := status.FromError(err)
			// Docker daemon might return Unknown for "No such container" when accessed via an API like containerz.
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

		// Define a timeout for receiving from the channel.
		// This prevents the test from hanging indefinitely if the channel never sends.
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
					// Test considers this path as having found the expected error.
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
		localImageTag := "latest"

		// 1. Ensure clean state for this specific container instance.
		err := baseCli.RemoveContainer(ctx, stoppedInstanceName, true)
		if err != nil && status.Code(err) != codes.NotFound {
			t.Fatalf("Pre-test removal of container %s failed: %v", stoppedInstanceName, err)
		}
		defer func() {
			if err := baseCli.RemoveContainer(ctx, stoppedInstanceName, true); err != nil && status.Code(err) != codes.NotFound {
				t.Logf("Cleanup: Failed to remove container %s: %v", stoppedInstanceName, err)
			}
		}()

		// 2. Ensure the image exists. Push if not.
		foundImage := false
		imgListCh, listErr := baseCli.ListImage(ctx, 0, map[string][]string{"name": {localImageName}, "tag": {localImageTag}})
		if listErr == nil {
			for img := range imgListCh {
				if img.Error == nil && img.ImageName == localImageName && img.ImageTag == localImageTag {
					foundImage = true
					break
				}
				if img.Error != nil {
					t.Logf("Error listing image during check for %s:%s: %v", localImageName, localImageTag, img.Error)
				}
			}
		} else {
			t.Logf("Could not list images to check for %s:%s (will attempt push): %v", localImageName, localImageTag, listErr)
		}

		if !foundImage {
			t.Logf("Image %s:%s not found for subtest, attempting to push.", localImageName, localImageTag)
			if remErr := baseCli.RemoveImage(ctx, localImageName, localImageTag, false); remErr != nil && status.Code(remErr) != codes.NotFound && remErr.Error() != client.ErrNotFound.Error() {
				t.Logf("Pre-push removal of image %s:%s failed (continuing with push): %v", localImageName, localImageTag, remErr)
			}
			progCh, pushErr := baseCli.PushImage(ctx, localImageName, localImageTag, *containerTar, false)
			if pushErr != nil {
				t.Fatalf("Failed to push image %s:%s for stopped log test: %v", localImageName, localImageTag, pushErr)
			}
			for prog := range progCh {
				if prog.Error != nil {
					t.Fatalf("Error during push of %s:%s for stopped log test: %v", localImageName, localImageTag, prog.Error)
				}
			}
			t.Logf("Successfully pushed %s:%s for stopped log test.", localImageName, localImageTag)
		}

		// 3. Start the container.
		if _, err = baseCli.StartContainer(ctx, localImageName, localImageTag, "./cntrsrv", stoppedInstanceName, client.WithPorts([]string{"60062:60062"})); err != nil {
			t.Fatalf("Failed to start container %s for stopped log test: %v", stoppedInstanceName, err)
		}
		t.Logf("Container %s started for stopped log test.", stoppedInstanceName)

		// 4. Stop the container.
		if err = baseCli.StopContainer(ctx, stoppedInstanceName, true); err != nil {
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
		defer stopContainer(ctx, t, localStartedCli)

		listCh, err := localStartedCli.ListContainer(ctx, true, 0, nil)
		if err != nil {
			t.Fatalf("ListContainer() failed: %v", err)
		}

		wantImg := imageName + ":latest" // e.g., "cntrsrv:latest"
		foundWantImgAndInstance := false
		var listedContainersForDebug []string

		for cnt := range listCh {
			if cnt.Error != nil {
				t.Errorf("Error received during ListContainer iteration: %v", cnt.Error)
				continue
			}
			listedContainersForDebug = append(listedContainersForDebug, cnt.Name+":"+cnt.ImageName)
			// Note: Docker prepends "/" to container names. Adjust if your runtime doesn't.
			if cnt.ImageName == wantImg && (cnt.Name == "/"+instanceName || cnt.Name == instanceName) {
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
			// Note: Docker prepends "/" to container names. Adjust if your runtime doesn't.
			t.Logf("%s %s %s", cntr.Name, cntr.ImageName, cntr.State)
			if (cntr.Name == "/"+instanceName || cntr.Name == instanceName) && cntr.State != cpb.ListContainerResponse_STOPPED.String() {
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

		err := baseCli.StopContainer(ctx, nonExistentInstance, true)
		if err == nil {
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
		err := localStartedCli.StopContainer(ctx, instanceName, true)
		if err != nil {
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
		time.Sleep(1 * time.Second) // Give a moment for removal to settle.

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
				// Log if the pre-check removal itself had an unexpected error (not NotFound)
				t.Logf("Pre-check RemoveVolume for %q encountered an unexpected error (continuing test): %v", nonExistentVolumeName, err)
			} else {
				// NotFound is an expected outcome for the pre-check if the volume didn't exist.
				t.Logf("Pre-check RemoveVolume for %q confirmed it was not found.", nonExistentVolumeName)
			}
		} else {
			// Success (no-op) for pre-check removal is also fine.
			t.Logf("Pre-check RemoveVolume for %q succeeded (was a no-op), confirming it's not present.", nonExistentVolumeName)
		}
		time.Sleep(1 * time.Second)

		err := cli.RemoveVolume(ctx, nonExistentVolumeName, true)
		if err == nil {
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
		defer stopContainer(ctx, t, cli)
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
		defer stopContainer(ctx, t, cli)

		nonExistentImage := "non-existent-image-for-upgrade"
		_, err := cli.UpdateContainer(ctx, nonExistentImage, "latest", "./cntrsrv", instanceName, false, client.WithPorts([]string{"60061:60061"}))
		if err == nil {
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
		defer stopContainer(ctx, t, cli)

		nonExistentTag := "non-existent-tag-for-upgrade"
		// Ensure the base image 'imageName:latest' exists from startContainer.
		_, err := cli.UpdateContainer(ctx, imageName, nonExistentTag, "./cntrsrv", instanceName, false, client.WithPorts([]string{"60061:60061"}))
		if err == nil {
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

		_, err := baseCli.UpdateContainer(ctx, imageName, "latest", "./cntrsrv", nonExistentInstance, false, client.WithPorts([]string{"60061:60061"}))
		if err == nil {
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
