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

package containerztest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/containerz/client"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cpb "github.com/openconfig/gnoi/containerz"
)

// Client returns a new containerz client.
func Client(t *testing.T, dut *ondatra.DUTDevice) *client.Client {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		config := `
				management api gnmi
				  transport grpc iana
				    port 9339
				    ssl profile SELFSIGNED
				    vrf mgmt
				    authorization requests
			`
		if deviations.ContainerzOCUnsupported(dut) {
			config += `
				management api gnoi
				service containerz
				  transport gnmi default
				  !
				  container runtime
					 vrf mgmt
				!
			`
		}
		helpers.GnmiCLIConfig(t, dut, config)
		// Configuring the containerz gNOI service may cause Octa to restart,
		// making gNMI temporarily unavailable. Retry write memory until it
		// succeeds or we exceed the deadline.
		gnmiSaveWithRetry(t, dut, 2*time.Minute)
		waitForGNOI(t, dut, 2*time.Minute)
		// EOS management namespace (ns-mgmt) has a default DROP policy.
		// A per-VRF CP ACL could open port 60061, but the AclAgent restart
		// race (BUG54186) silently drops rules on some EOS versions. Use a
		// direct ip6tables rule as the reliable alternative.
		dut.CLI().Run(t, "enable\nbash sudo ip netns exec ns-mgmt ip6tables -I EOS_INPUT 1 -p tcp --dport 60061 -j ACCEPT")
	case ondatra.CISCO:
		dut.Config().New().WithCiscoText(`
			appmgr docker allow-sensitive-paths
			ipv6 access-list restrict-access-ipv6
			  ! open port for cntrsrv from PROD
			  permit tcp any any eq 60061
		`).Append(t)
		t.Logf("Waiting for device to ingest its config.")
		time.Sleep(time.Minute)
	case ondatra.NOKIA, ondatra.JUNIPER:
		break
	default:
		t.Fatalf("Unsupported vendor for containerz: %v", dut.Vendor())
	}

	// Re-dial gNOI after config push -- configuring the containerz service may
	// have restarted Octa, making any prior gNOI connection stale. Use the
	// fresh clients directly rather than GNOI(t), which would return the stale
	// entry from Ondatra's cache and produce Unavailable errors on the next RPC.
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer dialCancel()
	freshClients, err := dut.RawAPIs().BindingDUT().DialGNOI(dialCtx)
	if err != nil {
		t.Logf("gNOI re-dial in Client() failed (non-fatal): %v", err)
		freshClients = dut.RawAPIs().GNOI(t)
	}
	return client.NewClientFromStub(freshClients.Containerz())
}

// waitForGNOI polls DialGNOI until the gNOI endpoint is reachable or the
// timeout elapses. Used after a config push that may restart Octa.
func waitForGNOI(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		// Use a short per-call timeout so DialGNOI does not block indefinitely
		// while the containerz service is restarting after a config push.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := dut.RawAPIs().BindingDUT().DialGNOI(ctx)
		cancel()
		if err == nil {
			t.Log("containerz gNOI service is reachable.")
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("containerz gNOI service did not become reachable within %v after config push", timeout)
		}
		time.Sleep(10 * time.Second)
	}
}

// gnmiSaveWithRetry retries "write memory" via gNMI until it succeeds or the
// deadline elapses. This handles transient Octa restarts after config changes.
func gnmiSaveWithRetry(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for attempt := 1; ; attempt++ {
		gnmiClient := dut.RawAPIs().GNMI(t)
		req, err := buildGNMICLISetRequest("write memory")
		if err != nil {
			t.Fatalf("Cannot build gNMI SetRequest for write memory: %v", err)
		}
		setCtx, setCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = gnmiClient.Set(setCtx, req)
		setCancel()
		if err == nil {
			t.Logf("write memory succeeded (attempt %d)", attempt)
			return
		}
		t.Logf("write memory attempt %d failed: %v", attempt, err)
		if time.Now().After(deadline) {
			t.Fatalf("write memory did not succeed within %v (last error: %v)", timeout, err)
		}
		time.Sleep(10 * time.Second)
	}
}

// buildGNMICLISetRequest builds a gNMI SetRequest with CLI origin.
func buildGNMICLISetRequest(config string) (*gpb.SetRequest, error) {
	return &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}, nil
}

// StartContainerOptions holds parameters for starting a container.
type StartContainerOptions struct {
	ImageName           string
	ImageTag            string
	TarPath             string
	InstanceName        string
	Command             string
	Ports               []string
	Volumes             []string
	RemoveExistingImage bool
	PollForRunningState bool
	PollTimeout         time.Duration
	PollInterval        time.Duration
	Network             string
	RestartPolicyName   string
}

// withDefaults returns a new StartContainerOptions with default values applied
// for fields that were zero-valued in the original options.
func (o StartContainerOptions) withDefaults() StartContainerOptions {
	res := o // Create a copy

	if res.ImageName == "" {
		res.ImageName = "cntrsrv_image"
	}
	if res.ImageTag == "" {
		res.ImageTag = "latest"
	}
	// TarPath is expected to be provided by a flag.
	if res.InstanceName == "" {
		res.InstanceName = "test-instance"
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
	if res.RestartPolicyName == "" {
		res.RestartPolicyName = "Always"
	}
	// Boolean fields (RemoveExistingImage, PollForRunningState) default to false (their zero value).
	return res
}

// DeployAndStart sets up and starts a container according to the provided options.
func DeployAndStart(ctx context.Context, t *testing.T, cli *client.Client, opts StartContainerOptions) error {
	t.Helper()
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
		return fmt.Errorf("initial call to PushImage for %s:%s failed: %w", opts.ImageName, opts.ImageTag, err)
	}
	for prog := range progCh {
		if prog.Error != nil {
			return fmt.Errorf("error during push of image %s:%s: %w", opts.ImageName, opts.ImageTag, prog.Error)
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
		return fmt.Errorf("failed to list images after push for %s:%s: %w", opts.ImageName, opts.ImageTag, err)
	}
	foundImage := false
	for img := range imgListCh {
		if img.Error != nil {
			return fmt.Errorf("error received during ListImage iteration for %s:%s: %w", opts.ImageName, opts.ImageTag, img.Error)
		}
		if img.ImageName == opts.ImageName && img.ImageTag == opts.ImageTag {
			foundImage = true
			break
		}
	}
	if !foundImage {
		return fmt.Errorf("image %s:%s not found after successful push", opts.ImageName, opts.ImageTag)
	}
	t.Logf("Image %s:%s verified successfully after push.", opts.ImageName, opts.ImageTag)

	// 5. Start the container.
	t.Logf("Starting container %s with image %s:%s, command '%s', ports %v, volumes %v, network %s, restart policy %s", opts.InstanceName, opts.ImageName, opts.ImageTag, opts.Command, opts.Ports, opts.Volumes, opts.Network, opts.RestartPolicyName)
	var startOpts []client.StartOption
	if len(opts.Ports) > 0 {
		startOpts = append(startOpts, client.WithPorts(opts.Ports))
	}
	if len(opts.Volumes) > 0 {
		startOpts = append(startOpts, client.WithVolumes(opts.Volumes))
	}
	if opts.Network != "" {
		startOpts = append(startOpts, client.WithNetwork(opts.Network))
	}
	if opts.RestartPolicyName != "" {
		startOpts = append(startOpts, client.WithRestartPolicy(opts.RestartPolicyName))
	}

	startResp, err := cli.StartContainer(ctx, opts.ImageName, opts.ImageTag, opts.Command, opts.InstanceName, startOpts...)
	if err != nil {
		return fmt.Errorf("unable to start container %s: %w", opts.InstanceName, err)
	}
	t.Logf("StartContainer called for %s, response: %s", opts.InstanceName, startResp)

	// 6. Wait for container to be running or fixed sleep.
	if opts.PollForRunningState {
		return WaitForRunning(ctx, t, cli, opts.InstanceName, opts.PollTimeout)
	}
	// Original behavior: fixed sleep. Use PollInterval as the sleep duration for simplicity.
	t.Logf("Waiting for %v for container %s to stabilize (fixed sleep, no polling).", opts.PollInterval, opts.InstanceName)
	time.Sleep(opts.PollInterval)
	return nil
}

// Setup is a helper function that deploys and starts a container, and returns a client
// and a cleanup function that stops and removes the container. It calls t.Fatalf on failure.
func Setup(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, opts StartContainerOptions) (*client.Client, func()) {
	t.Helper()
	cli := Client(t, dut)
	if err := DeployAndStart(ctx, t, cli, opts); err != nil {
		t.Fatalf("Failed to deploy and start container with options %+v: %v", opts, err)
	}
	t.Logf("Container %q started successfully.", opts.InstanceName)

	return cli, func() {
		// Use a fresh context so cleanup succeeds even if the test context was
		// canceled (e.g. on timeout).
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cleanupCancel()
		Stop(cleanupCtx, t, cli, opts.InstanceName)
		// Remove the container so it does not persist on the DUT in STOPPED
		// state. Without removal, Docker may restart a previously-running
		// container on its next daemon restart, contaminating subsequent tests.
		if err := cli.RemoveContainer(cleanupCtx, opts.InstanceName, true); err != nil {
			s, _ := status.FromError(err)
			if s.Code() != codes.NotFound {
				t.Logf("RemoveContainer %s encountered an issue: %v", opts.InstanceName, err)
			}
		} else {
			t.Logf("Container %s removed successfully.", opts.InstanceName)
		}
	}
}

// Stop stops a container instance.
func Stop(ctx context.Context, t *testing.T, cli *client.Client, instNameToStop string) {
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

// WaitForRunning polls until a container instance is in the RUNNING state.
func WaitForRunning(ctx context.Context, t *testing.T, cli *client.Client, instanceName string, timeout time.Duration) error {
	t.Helper()
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		// Use a per-call timeout so one slow RPC does not consume the entire budget.
		callCtx, callCancel := context.WithTimeout(pollCtx, 30*time.Second)
		listContCh, err := cli.ListContainer(callCtx, true, 0, map[string][]string{"name": {instanceName}})
		if err != nil {
			callCancel()
			t.Logf("ListContainer call for %s failed (will retry): %v", instanceName, err)
			if pollCtx.Err() != nil {
				return fmt.Errorf("timed out waiting for container %s to be RUNNING", instanceName)
			}
			time.Sleep(5 * time.Second)
			continue
		}

		var containerIsRunning bool
		var lastErr error
		for info := range listContCh {
			if info.Error != nil {
				lastErr = info.Error
				t.Logf("ListContainer stream error for %s (will retry): %v", instanceName, info.Error)
				break
			}
			t.Logf("ListContainer found: name=%s, image=%s, state=%s", info.Name, info.ImageName, info.State)
			if (info.Name == instanceName || info.Name == "/"+instanceName) && info.State == cpb.ListContainerResponse_RUNNING.String() {
				t.Logf("Container %s confirmed RUNNING.", instanceName)
				containerIsRunning = true
				break
			}
		}
		callCancel()

		if containerIsRunning {
			return nil
		}

		if pollCtx.Err() != nil {
			if lastErr != nil {
				return fmt.Errorf("timed out waiting for container %s to be RUNNING (last error: %v)", instanceName, lastErr)
			}
			return fmt.Errorf("timed out waiting for container %s to be RUNNING", instanceName)
		}
		time.Sleep(5 * time.Second)
	}
}
