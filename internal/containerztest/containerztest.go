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
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cpb "github.com/openconfig/gnoi/containerz"
)

// Client returns a new containerz client.
func Client(t *testing.T, dut *ondatra.DUTDevice) *client.Client {
	t.Helper()
	gnoiClient := dut.RawAPIs().GNOI(t)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		if deviations.ContainerzOCUnsupported(dut) {
			dut.Config().New().WithAristaText(`
				management api gnoi
				service containerz
				  transport gnmi default
				  !
				  container runtime
					 vrf mgmt
				!
			`).Append(t)
		}
		dut.Config().New().WithAristaText(`
			ipv6 access-list restrict-access-ipv6
			  ! open port for cntrsrv from PROD
			  permit tcp any any eq 60061
		`).Append(t)
		t.Logf("Waiting for device to ingest its config.")
		time.Sleep(time.Minute)
	case ondatra.NOKIA, ondatra.CISCO:
		break
	case ondatra.JUNIPER:
		break
	default:
		t.Fatalf("Unsupported vendor for containerz: %v", dut.Vendor())
	}

	return client.NewClientFromStub(gnoiClient.Containerz())
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
	Network             string
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
	t.Logf("Starting container %s with image %s:%s, command '%s', ports %v.", opts.InstanceName, opts.ImageName, opts.ImageTag, opts.Command, opts.Ports)
	var startOpts []client.StartOption
	if len(opts.Ports) > 0 {
		startOpts = append(startOpts, client.WithPorts(opts.Ports))
	}
	if opts.Network != "" {
		startOpts = append(startOpts, client.WithNetwork(opts.Network))
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
		Stop(ctx, t, cli, opts.InstanceName)
	}
}

// Stop stops and removes a container instance.
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
		listContCh, err := cli.ListContainer(pollCtx, true, 0, map[string][]string{"name": {instanceName}})
		if err != nil {
			return fmt.Errorf("unable to list container %s during polling: %w", instanceName, err)
		}

		var containerIsRunning bool
		for info := range listContCh {
			if info.Error != nil {
				return fmt.Errorf("error message received while listing container %s during polling: %w", instanceName, info.Error)
			}
			if (info.Name == instanceName || info.Name == "/"+instanceName) && info.State == cpb.ListContainerResponse_RUNNING.String() {
				t.Logf("Container %s confirmed RUNNING.", instanceName)
				containerIsRunning = true
				break
			}
		}

		if containerIsRunning {
			return nil
		}

		if pollCtx.Err() != nil {
			return fmt.Errorf("timed out waiting for container %s to be RUNNING", instanceName)
		}
		time.Sleep(5 * time.Second)
	}
}
