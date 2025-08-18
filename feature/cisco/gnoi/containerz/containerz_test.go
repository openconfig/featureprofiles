package containerz_test

import (
	"context"
	"flag"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	client "github.com/openconfig/featureprofiles/internal/cisco/containerz"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

const (
	bonnetRunCmd = `/bonnet --uid= --use_doh --use_tap_instead_of_tun --loas2_force_full_key_exchange 
	--noqbone_tunnel_health_check_stubby_probes --qbone_region_file=/config/qbone_region --svelte_retry_interval_ms=1000000000 
	--qbone_resolver_cc_algo= --qbone_bonnet_use_custom_healthz_handler --qbone_client_config_file=$QBONE_CLIENT_CONFIG_FILE --logtostderr`
	ubuntuRunCmd    = `sleep 7200`
	bonnetImageHash = "0d4137e7248470633f9dad443071caa6244c4cfb7f592ea044b6832be0572aa8"
	ubuntuImageHash = "65ae7a6f3544bd2d2b6d19b13bfc64752d776bc92c510f874188bfd404d205a3"
)

var (
	bonnetTarLocation = flag.String("bonnet_tar_location",
		"/auto/tftp-idt-tools/pi_infra/b4_containerz/bonnet-g3.tar.gz", "Location of the bonnet tar file")
	ubuntuTarLocation = flag.String("ubuntu_tar_location",
		"/auto/tftp-idt-tools/pi_infra/b4_containerz/ubuntu-latest.tar.gz", "Location of the ubuntu tar file")
	dutModel string
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestContainerzWorkflow(t *testing.T) {
	t.Logf("=== STARTING TestContainerzWorkflow ===")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dut := ondatra.DUT(t, "dut")
	dutModel = dut.Model() // Set global variable for reuse
	t.Logf("Setting up gNOI containerz client for DUT: %s (Model: %s)", dut.Name(), dutModel)
	cli, err := client.NewClient(ctx, t, dut)
	if err != nil {
		t.Fatalf("Unable to create gNOI containerz client: %v", err)
	}
	t.Logf("Successfully created gNOI containerz client")

	t.Run("Deploy", func(t *testing.T) {
		t.Logf(">>> DEPLOY TEST START <<<")
		t.Logf("Deploying bonnet image from location: %s", *bonnetTarLocation)
		if err := deployImage(t, ctx, cli, "bonnet", "g3", *bonnetTarLocation, false); err != nil {
			t.Fatalf("Failed to deploy bonnet image: %v", err)
		}
		time.Sleep(60 * time.Second)
		t.Logf("Successfully deployed bonnet:g3 image")
		t.Logf(">>> DEPLOY TEST END <<<")
	})

	t.Run("ListImage", func(t *testing.T) {
		t.Logf(">>> LIST IMAGE TEST START <<<")
		t.Logf("Listing images to verify bonnet:g3 deployment")
		listImage(t, ctx, cli, client.ImageInfo{
			ID:        bonnetImageHash[:12],
			ImageName: "bonnet",
			ImageTag:  "g3",
		})
		t.Logf(">>> LIST IMAGE TEST END <<<")
	})

	testVolumeName := "bonnet-data-vol"
	t.Run("CreateVolume", func(t *testing.T) {
		t.Logf(">>> CREATE VOLUME TEST START <<<")
		t.Logf("Creating volume: %s", testVolumeName)
		volumeName, err := cli.CreateVolume(ctx, testVolumeName, "local", nil, nil)
		if err != nil {
			t.Fatalf("Failed to create volume: %v", err)
		}
		if volumeName != testVolumeName {
			t.Fatalf("Volume name mismatch: expected %s, got %s", testVolumeName, volumeName)
		}
		t.Logf("Successfully created volume: %s", volumeName)
		t.Logf(">>> CREATE VOLUME TEST END <<<")
	})

	t.Run("StartContainer NEGATIVE with docker.sock volume", func(t *testing.T) {
		t.Logf(">>> START CONTAINER NEGATIVE TEST START <<<")
		t.Logf("Attempting to start bonnet container with docker.sock volume without configuring docker socket access")
		t.Logf("Expected: Should fail because docker socket access is not configured")

		// Set volume paths including docker.sock based on DUT model
		var volumePaths []string
		commonPaths := []string{"/dev/net/tun:/dev/net/tun", testVolumeName + ":/data"}

		if dutModel == "NCS5500" {
			// volumePaths = append(commonPaths, "/misc/app_host/google/.loas:/.loas", "/misc/app_host/google/config:/config", "/misc/app_host/docker.sock:/var/run/docker.sock")
			volumePaths = append(commonPaths, "/misc/app_host/docker.sock:/var/run/docker.sock")

		} else {
			// Default to 8000 paths for all other models
			volumePaths = append(commonPaths, "/var/lib/docker/appmgr/google/.loas:/.loas", "/var/lib/docker/appmgr/google/config:/config", "/var/run/docker.sock:/var/run/docker.sock")
		}

		_, err := cli.StartContainer(ctx, "bonnet", "g3", bonnetRunCmd, "bonnet-1",
			client.WithCapabilities([]string{"NET_ADMIN"}, nil),
			client.WithNetwork("host"),
			client.WithRestartPolicy("ALWAYS"),
			client.WithVolumes(volumePaths))

		if err == nil {
			t.Fatalf("Expected error when starting container with docker.sock volume without socket access configuration, but StartContainer succeeded")
		}

		t.Logf("Got expected error for docker.sock volume without configuration: %v", err)
		t.Logf(">>> START CONTAINER NEGATIVE TEST END <<<")
	})

	t.Run("StartContainer with docker.sock volume", func(t *testing.T) {
		t.Logf(">>> START CONTAINER TEST START <<<")
		t.Logf("Starting bonnet container with name 'bonnet-1'")
		t.Logf("Using run command: %s", bonnetRunCmd)
		t.Logf("Configuring container with capabilities: NET_ADMIN, network: host, restart policy: ALWAYS")

		t.Logf("Configuring docker socket access via CLI command")
		config.TextWithGNMI(ctx, t, dut, "appmgr docker-socket-access")
		t.Logf("Successfully configured docker socket access via gNMI CLI")

		// Set volume paths based on DUT model
		var volumePaths []string
		commonPaths := []string{"/dev/net/tun:/dev/net/tun", testVolumeName + ":/data"}

		if dutModel == "NCS5500" {
			// volumePaths = append(commonPaths, "/misc/app_host/docker.sock:/var/run/docker.sock")
			volumePaths = append(commonPaths, "/misc/app_host/google/.loas:/.loas", "/misc/app_host/google/config:/config", "/misc/app_host/docker.sock:/var/run/docker.sock")

		} else {
			// Default to 8000 paths for all other models
			volumePaths = append(commonPaths, "/var/lib/docker/appmgr/google/.loas:/.loas", "/var/lib/docker/appmgr/google/config:/config", "/var/run/docker.sock:/var/run/docker.sock")
		}

		startContainer(t, ctx, cli, "bonnet", "g3", bonnetRunCmd, "bonnet-1",
			client.WithCapabilities([]string{"NET_ADMIN"}, nil),
			client.WithNetwork("host"),
			client.WithRestartPolicy("ALWAYS"),
			client.WithVolumes(volumePaths))
		t.Logf("Successfully started bonnet-1 container with attached volume: %s", testVolumeName)
		t.Logf(">>> START CONTAINER TEST END <<<")
	})

	t.Run("ListContainer", func(t *testing.T) {
		t.Logf(">>> LIST CONTAINER TEST START <<<")
		t.Logf("Listing containers to verify bonnet-1 is running")
		listContainer(t, ctx, cli, client.ContainerInfo{
			Name:      "bonnet-1",
			ImageName: "bonnet:g3",
			State:     "RUNNING",
			Labels:    nil,
			Hash:      bonnetImageHash,
		})
		t.Logf(">>> LIST CONTAINER TEST END <<<")
	})

	t.Run("ListVolume", func(t *testing.T) {
		t.Logf(">>> LIST VOLUME TEST START <<<")
		t.Logf("Listing volumes to verify %s exists and is being used", testVolumeName)

		volCh, err := cli.ListVolume(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to list volumes: %v", err)
		}

		var volumes []client.VolumeInfo
		for vol := range volCh {
			volumes = append(volumes, *vol)
		}

		// Verify our test volume exists
		found := false
		for _, vol := range volumes {
			if vol.Name == testVolumeName {
				found = true
				t.Logf("Found volume: Name=%s, Driver=%s", vol.Name, vol.Driver)
				if vol.Driver != "local" {
					t.Errorf("Expected driver 'local', got '%s'", vol.Driver)
				}
				break
			}
		}

		if !found {
			t.Fatalf("Volume %s not found in volume list", testVolumeName)
		}

		t.Logf("Successfully verified volume %s exists", testVolumeName)
		t.Logf(">>> LIST VOLUME TEST END <<<")
	})

	t.Run("Logs", func(t *testing.T) {
		t.Logf(">>> CONTAINER LOGS TEST START <<<")
		t.Logf("Retrieving logs for bonnet-1 container")
		t.Logf("Expected: Non-empty logs with bonnet application output")

		// Get logs and validate content
		logEntries, err := getContainerLogs(t, ctx, cli, "bonnet-1", false)
		if err != nil {
			t.Fatalf("Failed to retrieve logs: %v", err)
		}

		if len(logEntries) == 0 {
			t.Fatalf("Expected logs but got empty result")
		}

		t.Logf("Retrieved %d log entries", len(logEntries))

		// Show first few log entries for debugging
		maxDisplay := 3
		if len(logEntries) < maxDisplay {
			maxDisplay = len(logEntries)
		}
		for i := 0; i < maxDisplay; i++ {
			t.Logf("Log entry [%d]: %s", i+1, logEntries[i])
		}

		// Validate bonnet-specific log content
		validateBonnetLogs(t, logEntries)

		t.Logf("Successfully validated %d log entries", len(logEntries))
		t.Logf(">>> CONTAINER LOGS TEST END <<<")
	})

	t.Run("NegativeLogs", func(t *testing.T) {
		t.Logf(">>> CONTAINER LOGS ERROR TEST START <<<")
		t.Logf("Testing logs retrieval for non-existent container")
		t.Logf("Expected: Should fail with container not found error")

		_, err := getContainerLogs(t, ctx, cli, "non-existent-container", false)
		if err == nil {
			t.Fatalf("Expected error for non-existent container, but Logs succeeded")
		}

		// Check if the error indicates the container was not found
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
			t.Logf("Warning: Expected 'not found' error, got: %v", err)
		}

		t.Logf("Got expected error for non-existent container: %v", err)
		t.Logf(">>> CONTAINER LOGS ERROR TEST END <<<")
	})

	t.Run("StopContainer", func(t *testing.T) {
		t.Logf(">>> STOP CONTAINER TEST START <<<")
		t.Logf("Stopping bonnet-1 container with force=true and restart=false")
		stopContainer(t, ctx, cli, "bonnet-1", true, false)
		t.Logf("Successfully stopped bonnet-1 container")
		t.Logf(">>> STOP CONTAINER TEST END <<<")

	})

	t.Run("RemoveImage", func(t *testing.T) {
		t.Logf(">>> REMOVE IMAGE TEST START <<<")
		t.Logf("Removing bonnet:g3 image")
		removeImage(t, ctx, cli, "bonnet", "g3")
		t.Logf("Successfully removed bonnet:g3 image")
		t.Logf(">>> REMOVE IMAGE TEST END <<<")
	})

	t.Run("RemoveVolume", func(t *testing.T) {
		t.Logf(">>> REMOVE VOLUME TEST START <<<")
		t.Logf("Removing volume: %s", testVolumeName)
		err := cli.RemoveVolume(ctx, testVolumeName, true)
		if err != nil {
			t.Fatalf("Failed to remove volume: %v", err)
		}
		t.Logf("Successfully removed volume: %s", testVolumeName)
		t.Logf(">>> REMOVE VOLUME TEST END <<<")
	})

	t.Run("ListVolumeAfterRemoval", func(t *testing.T) {
		t.Logf(">>> LIST VOLUME AFTER REMOVAL TEST START <<<")
		t.Logf("Listing volumes to verify %s has been deleted", testVolumeName)

		volCh, err := cli.ListVolume(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to list volumes: %v", err)
		}

		var volumes []client.VolumeInfo
		for vol := range volCh {
			volumes = append(volumes, *vol)
		}

		// Verify our test volume no longer exists
		for _, vol := range volumes {
			if vol.Name == testVolumeName {
				t.Fatalf("Volume %s still exists after removal", testVolumeName)
			}
		}

		t.Logf("Successfully verified volume %s has been deleted", testVolumeName)
		t.Logf(">>> LIST VOLUME AFTER REMOVAL TEST END <<<")
	})

	// Remove the docker-socket-access via gNMI
	t.Run("CleanupDockerSocketAccess", func(t *testing.T) {
		t.Logf(">>> CLEANUP DOCKER SOCKET ACCESS TEST START <<<")
		t.Logf("Removing docker socket access configuration via CLI command")

		config.TextWithGNMI(ctx, t, dut, "no appmgr docker-socket-access")
		t.Logf("Successfully removed docker socket access configuration via gNMI CLI")
		t.Logf(">>> CLEANUP DOCKER SOCKET ACCESS TEST END <<<")
	})

	t.Logf("=== COMPLETED TestContainerzWorkflow ===")
}

func sTestRemoveContainer(t *testing.T) {
	t.Logf("=== STARTING TestRemoveContainer ===")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dut := ondatra.DUT(t, "dut")
	dutModel = dut.Model() // Set global variable for reuse
	t.Logf("Setting up gNOI containerz client for DUT: %s (Model: %s)", dut.Name(), dutModel)
	cli, err := client.NewClient(ctx, t, dut)
	if err != nil {
		t.Fatalf("Unable to create gNOI containerz client: %v", err)
	}
	t.Logf("Successfully created gNOI containerz client")
	time.Sleep(60 * time.Second)
	t.Run("Deploy ubuntu docker image with name='bonnet', tag='g3' and image_size=MaxUint64", func(t *testing.T) {
		t.Logf(">>> DEPLOY WITH MAX SIZE TEST START <<<")
		t.Logf("Attempting to deploy ubuntu image with impossible size (MaxUint64)")
		t.Logf("Using ubuntu tar location: %s", *ubuntuTarLocation)
		t.Logf("Expected result: Should fail with 'Not enough space to store image' error")
		err := deployImage(t, ctx, cli, "bonnet", "g3", *ubuntuTarLocation, false, uint64(math.MaxUint64))
		if err == nil {
			t.Fatalf("Expected error due to insufficient space, but deployImage succeeded")
		}

		if !strings.Contains(err.Error(), "Not enough space to store image") {
			t.Fatalf("Expected 'Not enough space to store image' error, got: %v", err)
		}

		t.Logf("Got expected error for insufficient space: %v", err)
		t.Logf(">>> DEPLOY WITH MAX SIZE TEST END <<<")
	})

	t.Run("Deploy ubuntu docker image with name='bonnet', tag='g3' and image_size=ubuntu_image_size", func(t *testing.T) {
		t.Logf(">>> DEPLOY UBUNTU IMAGE TEST START <<<")
		t.Logf("Deploying ubuntu image with name='bonnet', tag='g3'")
		t.Logf("Using ubuntu tar location: %s", *ubuntuTarLocation)
		t.Logf("Expected result: Should succeed with proper image size")
		if err := deployImage(t, ctx, cli, "bonnet", "g3", *ubuntuTarLocation, false); err != nil {
			t.Fatalf("Failed to deploy ubuntu image: %v", err)
		}
		t.Logf("Successfully deployed ubuntu image as bonnet:g3")
		t.Logf(">>> DEPLOY UBUNTU IMAGE TEST END <<<")
	})

	t.Run("List image", func(t *testing.T) {
		t.Logf(">>> LIST IMAGE AFTER DEPLOY TEST START <<<")
		t.Logf("Listing images to verify ubuntu image deployed as bonnet:g3")
		t.Logf("Expected: Single image with ID, ImageName='bonnet', ImageTag='g3'")

		listImage(t, ctx, cli, client.ImageInfo{
			ID:        ubuntuImageHash[:12],
			ImageName: "bonnet",
			ImageTag:  "g3",
		})
		t.Logf(">>> LIST IMAGE AFTER DEPLOY TEST END <<<")
	})

	t.Run("Start container with instance_name='ubuntu1',image_name='bonnet',tag='g3',labels={'app':'ubuntu'},limit={}", func(t *testing.T) {
		t.Logf(">>> START UBUNTU1 CONTAINER TEST START <<<")
		t.Logf("Starting container 'ubuntu1' with image bonnet:g3")
		t.Logf("Using run command: %s", ubuntuRunCmd)
		t.Logf("Configuring with labels: app=ubuntu, CPU limit: 1.0, soft limit: 500 MiB, hard limit: 750 MiB")
		// Expected output should be that the container is successfully started.
		startContainer(t, ctx, cli, "bonnet", "g3", ubuntuRunCmd, "ubuntu1",
			client.WithLabels(map[string]string{"app": "ubuntu"}),
			client.WithCPUs(1.0),
			client.WithSoftLimit(524288000),
			client.WithHardLimit(786432000))
		t.Logf("Successfully started ubuntu1 container")
		t.Logf(">>> START UBUNTU1 CONTAINER TEST END <<<")
	})

	t.Run("Start container with instance_name='ubuntu2',image_name='bonnet',tag='g3',devices=device_value", func(t *testing.T) {
		t.Logf(">>> START UBUNTU2 WITH DEVICES TEST START <<<")
		t.Logf("Attempting to start container 'ubuntu2' with unsupported devices parameter")
		t.Logf("Using devices: [dev1, dev2:mydev2, dev3:mydev3:rw]")
		t.Logf("Expected result: Should fail with 'Devices options are not supported' error")
		_, err := cli.StartContainer(ctx, "bonnet", "g3", ubuntuRunCmd, "ubuntu2",
			client.WithDevices([]string{"dev1", "dev2:mydev2", "dev3:mydev3:rw"}))
		if err == nil {
			t.Fatalf("Expected error for devices not supported, but StartContainer succeeded")
		}

		// Check for the exact error message
		if !strings.Contains(err.Error(), "Devices options are not supported") {
			t.Fatalf("Expected 'Devices options are not supported' error, got: %v", err)
		}

		t.Logf("Got expected error for devices not supported: %v", err)
		t.Logf(">>> START UBUNTU2 WITH DEVICES TEST END <<<")
	})

	t.Run("List container", func(t *testing.T) {
		t.Logf(">>> LIST CONTAINER AFTER UBUNTU1 START TEST START <<<")
		t.Logf("Listing containers to verify ubuntu1 is running")
		t.Logf("Expected: Single container 'ubuntu1' with bonnet:g3 image, RUNNING state, app=ubuntu label")
		listContainer(t, ctx, cli, client.ContainerInfo{
			Name:      "ubuntu1",
			ImageName: "bonnet:g3",
			State:     "RUNNING",
			Labels:    map[string]string{"app": "ubuntu"},
			Hash:      ubuntuImageHash,
		})
		t.Logf(">>> LIST CONTAINER AFTER UBUNTU1 START TEST END <<<")
	})

	time.Sleep(120 * time.Second)

	t.Run("Deploy bonnet docker image with name='bonnet', tag='g3' and image_size=bonnet_image_size", func(t *testing.T) {
		t.Logf(">>> DEPLOY BONNET IMAGE TEST START <<<")
		t.Logf("Deploying bonnet image with name='bonnet', tag='g3'")
		t.Logf("Using bonnet tar location: %s", *bonnetTarLocation)
		t.Logf("Expected result: Should succeed and replace/update existing bonnet:g3 image")
		if err := deployImage(t, ctx, cli, "bonnet", "g3", *bonnetTarLocation, false); err != nil {
			t.Fatalf("Failed to deploy bonnet image: %v", err)
		}
		t.Logf("Successfully deployed bonnet:g3 image")
		t.Logf(">>> DEPLOY BONNET IMAGE TEST END <<<")
	})

	t.Run("Start container with instance_name='bonnet1',image_name='bonnet',tag='g3',labels={'app':'bonnet'}", func(t *testing.T) {
		t.Logf(">>> START BONNET1 CONTAINER TEST START <<<")
		t.Logf("Starting container 'bonnet1' with image bonnet:g3")
		t.Logf("Using run command: %s", bonnetRunCmd)
		t.Logf("Configuring with labels: app=bonnet")
		startContainer(t, ctx, cli, "bonnet", "g3", bonnetRunCmd, "bonnet1",
			client.WithLabels(map[string]string{"app": "bonnet"}))
		t.Logf("Successfully started bonnet1 container")
		t.Logf(">>> START BONNET1 CONTAINER TEST END <<<")
	})

	t.Run("List image", func(t *testing.T) {
		t.Logf(">>> LIST IMAGES AFTER BONNET DEPLOY TEST START <<<")
		t.Logf("Listing images to verify bonnet image")
		t.Logf("Expected: Single image - tagged 'bonnet:g3'")
		listImage(t, ctx, cli,
			client.ImageInfo{
				ID:        bonnetImageHash[:12],
				ImageName: "bonnet",
				ImageTag:  "g3",
			},
		)
		t.Logf(">>> LIST IMAGES AFTER BONNET DEPLOY TEST END <<<")
	})

	t.Run("List container", func(t *testing.T) {
		t.Logf(">>> LIST BOTH CONTAINERS TEST START <<<")
		t.Logf("Listing containers to verify both ubuntu1 and bonnet1 are running")
		t.Logf("Expected: ubuntu1 with untagged image, bonnet1 with tagged bonnet:g3 image")
		listContainer(t, ctx, cli,
			client.ContainerInfo{
				Name:      "ubuntu1",
				ImageName: ubuntuImageHash[:12],
				State:     "RUNNING",
				Labels:    map[string]string{"app": "ubuntu"},
				Hash:      ubuntuImageHash,
			},
			client.ContainerInfo{
				Name:      "bonnet1",
				ImageName: "bonnet:g3",
				State:     "RUNNING",
				Labels:    map[string]string{"app": "bonnet"},
				Hash:      bonnetImageHash,
			},
		)
		t.Logf(">>> LIST BOTH CONTAINERS TEST END <<<")
	})

	t.Run("Stop container with instance_name='ubuntu1'", func(t *testing.T) {
		t.Logf(">>> STOP UBUNTU1 CONTAINER TEST START <<<")
		t.Logf("Stopping container 'ubuntu1' with force=false and restart=false")
		stopContainer(t, ctx, cli, "ubuntu1", false, false)
		t.Logf("Successfully stopped ubuntu1 container")
		t.Logf(">>> STOP UBUNTU1 CONTAINER TEST END <<<")
	})

	t.Run("Start container with instance_name='ubuntu1' and labels={'app':'ubuntu2'}", func(t *testing.T) {
		t.Logf(">>> START UBUNTU1 WITH DIFFERENT LABELS TEST START <<<")
		t.Logf("Attempting to start existing container 'ubuntu1' with different labels")
		t.Logf("Original labels: app=ubuntu, New labels: app=ubuntu2")
		t.Logf("Expected result: Should fail due to label conflict/mismatch")
		_, err := cli.StartContainer(ctx, "", "", "", "ubuntu1",
			client.WithLabels(map[string]string{"app": "ubuntu2"}))
		if err == nil {
			t.Fatalf("Expected error when starting container with different labels, but StartContainer succeeded")
		}
		if !strings.Contains(err.Error(), "label") && !strings.Contains(err.Error(), "conflict") && !strings.Contains(err.Error(), "exist") {
			t.Logf("Warning: Expected error about label conflict, got: %v", err)
		}
		t.Logf("Got expected error for label mismatch: %v", err)
		t.Logf(">>> START UBUNTU1 WITH DIFFERENT LABELS TEST END <<<")
	})

	t.Run("Start container with only instance_name='ubuntu1'", func(t *testing.T) {
		t.Logf(">>> RESTART UBUNTU1 WITH ORIGINAL CONFIG TEST START <<<")
		t.Logf("Starting container 'ubuntu1' with empty image parameters (should use original)")
		t.Logf("Expected result: Container should start with original image and configuration")
		startContainer(t, ctx, cli, "", "", "", "ubuntu1")
		t.Logf("Successfully restarted ubuntu1 container with original configuration")
		t.Logf(">>> RESTART UBUNTU1 WITH ORIGINAL CONFIG TEST END <<<")
	})

	t.Run("List container", func(t *testing.T) {
		t.Logf(">>> LIST BOTH CONTAINERS TEST START <<<")
		t.Logf("Listing containers to verify both ubuntu1 and bonnet1 are running")
		t.Logf("Expected: ubuntu1 with untagged image, bonnet1 with tagged bonnet:g3 image")
		// Expected output should be 'ubuntu1' container is running with untagged image ID whereas 'bonnet1' container is running with image name 'bonnet' and tag 'g3'
		listContainer(t, ctx, cli,
			client.ContainerInfo{
				Name:      "ubuntu1",
				ImageName: ubuntuImageHash[:12],
				State:     "RUNNING",
				Labels:    map[string]string{"app": "ubuntu"},
				Hash:      ubuntuImageHash,
			},
			client.ContainerInfo{
				Name:      "bonnet1",
				ImageName: "bonnet:g3", // Tagged image name
				State:     "RUNNING",
				Labels:    map[string]string{"app": "bonnet"},
				Hash:      bonnetImageHash,
			},
		)
		t.Logf(">>> LIST BOTH CONTAINERS TEST END <<<")
	})

	t.Run("Remove container with instance_name='ubuntu1' and force=true", func(t *testing.T) {
		t.Logf(">>> REMOVE UBUNTU1 CONTAINER WITH FORCE TEST START <<<")
		t.Logf("Removing running container 'ubuntu1' with force=true")
		t.Logf("Expected result: Container should be forcefully removed")
		// Expected output should be that the container is successfully removed.
		if err := removeContainer(t, ctx, cli, "ubuntu1", true); err != nil {
			t.Fatalf("Failed to remove container ubuntu1: %v", err)
		}
		t.Logf("Successfully removed container ubuntu1 with force=true")
		t.Logf(">>> REMOVE UBUNTU1 CONTAINER WITH FORCE TEST END <<<")
	})

	t.Run("List container", func(t *testing.T) {
		t.Logf(">>> LIST CONTAINERS AFTER UBUNTU1 REMOVAL TEST START <<<")
		t.Logf("Listing containers to verify only bonnet1 remains")
		t.Logf("Expected: Only bonnet1 container should be running")
		// Expected output should be only 'bonnet1' container is running with image name 'bonnet' and tag 'g3'
		listContainer(t, ctx, cli, client.ContainerInfo{
			Name:      "bonnet1",
			ImageName: "bonnet:g3", // Tagged image name
			State:     "RUNNING",
			Labels:    map[string]string{"app": "bonnet"},
			Hash:      bonnetImageHash, // Hash of the bonnet image
		})
		t.Logf(">>> LIST CONTAINERS AFTER UBUNTU1 REMOVAL TEST END <<<")
	})

	t.Run("List image", func(t *testing.T) {
		t.Logf(">>> LIST IMAGES AFTER UBUNTU1 REMOVAL TEST START <<<")
		t.Logf("Listing images to verify dangling ubuntu image is removed")
		t.Logf("Expected: Only tagged bonnet:g3 image should remain")
		listImage(t, ctx, cli, client.ImageInfo{
			ID:        bonnetImageHash[:12],
			ImageName: "bonnet",
			ImageTag:  "g3",
		})
		t.Logf(">>> LIST IMAGES AFTER UBUNTU1 REMOVAL TEST END <<<")
	})

	t.Run("Remove container with instance_name='ubuntu1'", func(t *testing.T) {
		t.Logf(">>> REMOVE NON-EXISTENT UBUNTU1 TEST START <<<")
		t.Logf("Attempting to remove already removed container 'ubuntu1'")
		t.Logf("Expected result: Should fail with 'resource was not found' error")
		// Expected output should be an error that the container is not found.
		err := removeContainer(t, ctx, cli, "ubuntu1", false)
		if err == nil {
			t.Fatalf("Expected error when removing non-existent container ubuntu1, but RemoveContainer succeeded")
		}
		// Check for the exact error message
		if !strings.Contains(err.Error(), "resource was not found") {
			t.Fatalf("Expected 'resource was not found' error, got: %v", err)
		}
		t.Logf("Got expected error for non-existent container: %v", err)
		t.Logf(">>> REMOVE NON-EXISTENT UBUNTU1 TEST END <<<")
	})

	t.Run("Stop container with instance_name='bonnet1'", func(t *testing.T) {
		t.Logf(">>> STOP BONNET1 CONTAINER TEST START <<<")
		t.Logf("Stopping container 'bonnet1' with force=false and restart=false")
		// Expected output should be that the container is successfully stopped.
		stopContainer(t, ctx, cli, "bonnet1", false, false)
		t.Logf("Successfully stopped bonnet1 container")
		t.Logf(">>> STOP BONNET1 CONTAINER TEST END <<<")
	})

	t.Run("Remove container with instance_name='bonnet1'", func(t *testing.T) {
		t.Logf(">>> REMOVE BONNET1 CONTAINER TEST START <<<")
		t.Logf("Removing stopped container 'bonnet1' with force=false")
		t.Logf("Expected result: Container should be successfully removed")
		// Expected output should be that the container is successfully removed.
		if err := removeContainer(t, ctx, cli, "bonnet1", false); err != nil {
			t.Fatalf("Failed to remove container bonnet1: %v", err)
		}
		t.Logf("Successfully removed container bonnet1 with force=false")
		t.Logf(">>> REMOVE BONNET1 CONTAINER TEST END <<<")
	})

	t.Run("List container", func(t *testing.T) {
		t.Logf(">>> LIST CONTAINERS AFTER ALL REMOVALS TEST START <<<")
		t.Logf("Listing containers to verify no containers remain")
		t.Logf("Expected result: Empty container list")
		// Expected output should be that no containers are running.
		listContainer(t, ctx, cli) // Empty - no containers expected
		t.Logf(">>> LIST CONTAINERS AFTER ALL REMOVALS TEST END <<<")
	})

	t.Run("List image", func(t *testing.T) {
		t.Logf(">>> LIST IMAGES AFTER CONTAINER CLEANUP TEST START <<<")
		t.Logf("Listing images to verify bonnet image still exists")
		t.Logf("Expected result: bonnet:g3 image should still be present")
		// Expected output should be that 'bonnet' image should be present.
		listImage(t, ctx, cli, client.ImageInfo{
			ID:        bonnetImageHash[:12],
			ImageName: "bonnet",
			ImageTag:  "g3",
		})
		t.Logf(">>> LIST IMAGES AFTER CONTAINER CLEANUP TEST END <<<")
	})

	t.Run("Remove image with name='bonnet' and tag='g3'", func(t *testing.T) {
		t.Logf(">>> REMOVE BONNET IMAGE TEST START <<<")
		t.Logf("Removing bonnet:g3 image")
		t.Logf("Expected result: Image should be successfully removed")
		// Expected output should be that the image is successfully removed.
		removeImage(t, ctx, cli, "bonnet", "g3")
		t.Logf("Successfully removed bonnet:g3 image")
		t.Logf(">>> REMOVE BONNET IMAGE TEST END <<<")
	})

	t.Run("List image", func(t *testing.T) {
		t.Logf(">>> LIST IMAGES AFTER IMAGE REMOVAL TEST START <<<")
		t.Logf("Listing images to verify no images remain")
		t.Logf("Expected result: Empty image list")
		// Expected output should be that no images are present.
		listImage(t, ctx, cli) // Empty - no images expected
		t.Logf(">>> LIST IMAGES AFTER IMAGE REMOVAL TEST END <<<")
	})

	t.Logf("=== COMPLETED TestRemoveContainer ===")
}
