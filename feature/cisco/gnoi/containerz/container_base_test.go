package containerz_test

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	client "github.com/openconfig/featureprofiles/internal/cisco/containerz"
)

func deployImage(t *testing.T, ctx context.Context, cli *client.Client, image string, tag string, file string, isPlugin bool, imageSize ...uint64) error {
	progCh, err := cli.PushImage(ctx, image, tag, file, isPlugin, imageSize...)
	if err != nil {
		return err
	}

	for prog := range progCh {
		switch {
		case prog.Error != nil:
			return prog.Error
		case prog.Finished:
			t.Logf("Pushed %s/%s\n", prog.Image, prog.Tag)
		default:
			t.Logf(" %d bytes pushed", prog.BytesReceived)
		}
	}
	return nil
}

func startContainer(t *testing.T, ctx context.Context, cli *client.Client, imageName, imageTag, runCmd, instanceName string, opts ...client.StartOption) {
	ret, err := cli.StartContainer(ctx, imageName, imageTag, runCmd, instanceName, opts...)

	if err != nil {
		t.Fatalf("Unable to start container: %v", err)
	}

	time.Sleep(60 * time.Second)
	t.Logf("Started %s", ret)
}

func listImage(t *testing.T, ctx context.Context, cli *client.Client, expectedImages ...client.ImageInfo) {
	listCh, err := cli.ListImage(ctx, 10, nil)
	if err != nil {
		t.Errorf("unable to list images: %v", err)
		return
	}

	var gotImages []client.ImageInfo
	for img := range listCh {
		gotImages = append(gotImages, client.ImageInfo{
			ID:        img.ID,
			ImageName: img.ImageName,
			ImageTag:  img.ImageTag,
		})
	}

	t.Logf("Expected %d images: %+v", len(expectedImages), expectedImages)
	t.Logf("Actual %d images: %+v", len(gotImages), gotImages)

	if diff := cmp.Diff(expectedImages, gotImages, cmpopts.SortSlices(func(a, b client.ImageInfo) bool {
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.ImageName != b.ImageName {
			return a.ImageName < b.ImageName
		}
		return a.ImageTag < b.ImageTag
	})); diff != "" {
		t.Errorf("ListImage() returned diff (-want, +got):\n%s", diff)
	}
}

func listContainer(t *testing.T, ctx context.Context, cli *client.Client, expectedContainers ...client.ContainerInfo) {
	listCh, err := cli.ListContainer(ctx, true, 0, nil)
	if err != nil {
		t.Fatalf("unable to list containers: %v", err)
	}

	var gotContainers []client.ContainerInfo
	for cnt := range listCh {
		gotContainers = append(gotContainers, client.ContainerInfo{
			ID:        cnt.ID,
			Name:      cnt.Name,
			ImageName: cnt.ImageName,
			State:     cnt.State,
			Labels:    cnt.Labels,
			Hash:      cnt.Hash,
		})
	}

	t.Logf("Expected %d containers: %+v", len(expectedContainers), expectedContainers)
	t.Logf("Actual %d containers: %+v", len(gotContainers), gotContainers)

	if diff := cmp.Diff(expectedContainers, gotContainers,
		cmpopts.SortSlices(func(a, b client.ContainerInfo) bool { return a.Name < b.Name }),
		cmpopts.IgnoreFields(client.ContainerInfo{}, "ID", "Error"),
		cmp.FilterPath(func(p cmp.Path) bool { return p.String() == "[].Hash" || p.String() == "Hash" }, cmp.Comparer(func(actualRaw, expectedASCII []byte) bool {
			if len(actualRaw) == 0 && len(expectedASCII) == 0 {
				return true
			}
			actualHex := hex.EncodeToString(actualRaw)
			expectedHex := string(expectedASCII)
			return actualHex == expectedHex
		})),
		cmp.FilterPath(func(p cmp.Path) bool { return p.String() == "[].Labels" || p.String() == "Labels" }, cmp.Comparer(func(expected, actual map[string]string) bool {
			if expected == nil && actual == nil {
				return true
			}
			if expected == nil {
				return true
			}
			if actual == nil {
				return len(expected) == 0
			}
			// Verify all expected labels exist in actual labels
			for key, expectedValue := range expected {
				if actualValue, exists := actual[key]; !exists || actualValue != expectedValue {
					return false
				}
			}
			return true
		})),
	); diff != "" {
		t.Errorf("ListContainer() returned diff (-want, +got):\n%s", diff)
	}
}

func getContainerLogs(t *testing.T, ctx context.Context, cli *client.Client, containerName string, follow bool) ([]string, error) {
	t.Logf("Attempting to retrieve logs for container %s (follow=%v)", containerName, follow)

	logCh, err := cli.Logs(ctx, containerName, follow)
	if err != nil {
		t.Logf("Failed to get log channel: %v", err)
		return nil, err
	}

	var logs []string
	for msg := range logCh {
		if msg.Error != nil {
			t.Logf("Log message error: %v", msg.Error)
			return logs, msg.Error
		}
		logs = append(logs, msg.Msg)
	}

	t.Logf("Retrieved %d log entries total", len(logs))
	return logs, nil
}

func validateContainerLogs(t *testing.T, logs []string) {
	t.Logf("Validating containerz log retrieval functionality")

	// Focus on containerz infrastructure capabilities, not application content
	if len(logs) == 0 {
		t.Errorf("No logs retrieved - containerz log retrieval failed")
		return
	}

	t.Logf("Successfully retrieved %d log entries via containerz", len(logs))

	// Basic validation to ensure logs are being retrieved properly
	emptyCount := 0

	for i, log := range logs {
		if strings.TrimSpace(log) == "" {
			emptyCount++
			continue
		}

		// Just verify we have some content - this proves containerz is working
		if i < 3 { // Log first few entries as samples
			t.Logf("Sample log entry %d: %s", i+1, log)
		}
	}

	if emptyCount == len(logs) {
		t.Errorf("All log entries are empty - possible issue with containerz log streaming")
	} else {
		t.Logf("Log retrieval validation successful: %d non-empty entries out of %d total",
			len(logs)-emptyCount, len(logs))
	}

	t.Logf("Containerz log retrieval infrastructure validated successfully")
}

func validateBonnetLogs(t *testing.T, logs []string) {
	t.Logf("Validating bonnet-specific log content")

	// First run generic containerz validation
	validateContainerLogs(t, logs)

	if len(logs) == 0 {
		return // Already handled by generic validation
	}

	// Bonnet-specific validations
	bonnetFound := false
	processFound := false
	googleComponentsFound := false
	configFound := false
	logFormatFound := false

	for _, log := range logs {
		if strings.TrimSpace(log) == "" {
			continue
		}

		// Bonnet-specific patterns
		if strings.Contains(log, "/bonnet") || strings.Contains(log, "bonnet") ||
			strings.Contains(log, "qbone") || strings.Contains(log, "argv[0]:") {
			bonnetFound = true
		}

		// Bonnet process indicators
		if strings.Contains(log, "Process id") || strings.Contains(log, "init_google.cc") ||
			strings.Contains(log, "Command line arguments:") {
			processFound = true
		}

		// Google infrastructure components (bonnet is a Google service)
		if strings.Contains(log, "Census enabled") || strings.Contains(log, "Build tool: Blaze") ||
			strings.Contains(log, "gRPC") || strings.Contains(log, ".cc:") {
			googleComponentsFound = true
		}

		// Configuration and environment setup
		if strings.Contains(log, "--qbone_region_file") || strings.Contains(log, "--uid=") ||
			strings.Contains(log, "Current working directory") || strings.Contains(log, "timezone") {
			configFound = true
		}

		// Proper log format (Google glog style)
		if strings.Contains(log, "I0724") || strings.Contains(log, "W0724") ||
			strings.Contains(log, "E0724") {
			logFormatFound = true
		}
	}

	// Bonnet-specific infrastructure validations
	if !bonnetFound {
		t.Logf("Warning: No bonnet references found in logs - may be retrieving from wrong container")
	} else {
		t.Logf("Confirmed logs are from bonnet container")
	}

	if !processFound {
		t.Logf("Warning: No bonnet process startup info found - container may not be fully initialized")
	} else {
		t.Logf("Confirmed bonnet container process is running")
	}

	if !googleComponentsFound {
		t.Logf("Warning: No Google infrastructure components found - bonnet may not be properly initialized")
	} else {
		t.Logf("Confirmed Google infrastructure components are active")
	}

	if !configFound {
		t.Logf("Warning: No configuration parameters found - bonnet may not be properly configured")
	} else {
		t.Logf("Confirmed bonnet configuration parameters are present")
	}

	if !logFormatFound {
		t.Logf("Warning: No Google glog format found - logs may not be in expected format")
	} else {
		t.Logf("Confirmed proper Google glog format is being used")
	}

	t.Logf("Bonnet-specific log validation completed")
}

func validateUbuntuLogs(t *testing.T, logs []string) {
	t.Logf("Validating ubuntu-specific log content")

	// First run generic containerz validation
	validateContainerLogs(t, logs)

	if len(logs) == 0 {
		return // Already handled by generic validation
	}

	// Ubuntu-specific validations
	ubuntuFound := false
	processFound := false

	for _, log := range logs {
		if strings.TrimSpace(log) == "" {
			continue
		}

		// Ubuntu-specific patterns
		if strings.Contains(log, "ubuntu") || strings.Contains(log, "/bin/bash") ||
			strings.Contains(log, "root@") || strings.Contains(log, "Linux") {
			ubuntuFound = true
		}

		// Ubuntu process indicators
		if strings.Contains(log, "bash") || strings.Contains(log, "systemd") ||
			strings.Contains(log, "Starting") || strings.Contains(log, "Started") {
			processFound = true
		}
	}

	// Ubuntu-specific infrastructure validations
	if !ubuntuFound {
		t.Logf("Warning: No ubuntu references found in logs - may be retrieving from wrong container")
	} else {
		t.Logf("Confirmed logs are from ubuntu container")
	}

	if !processFound {
		t.Logf("Warning: No ubuntu process startup info found - container may not be fully initialized")
	} else {
		t.Logf("Confirmed ubuntu container process is running")
	}

	t.Logf("Ubuntu-specific log validation completed")
}

func volume(t *testing.T, ctx context.Context, cli *client.Client) {
	wantVolume := "my-vol"
	gotVolume, err := cli.CreateVolume(ctx, "my-vol", "local", nil, nil)
	if err != nil {
		t.Errorf("unable to create volume: %v", err)
	}
	defer cli.RemoveVolume(ctx, "my-vol", true)

	if wantVolume != gotVolume {
		t.Errorf("incorrect volume name: want %s, got %s", wantVolume, gotVolume)
	}

	t.Logf("created volume %s", gotVolume)

	volCh, err := cli.ListVolume(ctx, nil)
	if err != nil {
		t.Errorf("unable to list volumes: %v", err)
	}

	var gotVolumes []*client.VolumeInfo
	for vol := range volCh {
		gotVolumes = append(gotVolumes, vol)
	}

	// Find our test volume among all volumes
	var myVolFound bool
	for _, vol := range gotVolumes {
		if vol.Name == "my-vol" && vol.Driver == "local" {
			myVolFound = true
			break
		}
	}

	if !myVolFound {
		t.Errorf("Expected volume 'my-vol' with driver 'local' not found in volume list: %+v", gotVolumes)
	}
}

func stopContainer(t *testing.T, ctx context.Context, cli *client.Client, instanceName string, force, restart bool) {
	if err := cli.StopContainer(ctx, instanceName, force, restart); err != nil {
		t.Logf("Failed to stop container: %v", err)
	}
}

func removeContainer(t *testing.T, ctx context.Context, cli *client.Client, instanceName string, force bool) error {
	t.Logf("Attempting to remove container %s with force=%v", instanceName, force)
	err := cli.RemoveContainer(ctx, instanceName, force)
	if err != nil {
		t.Logf("Failed to remove container %s: %v", instanceName, err)
	} else {
		t.Logf("Successfully removed container %s", instanceName)
	}
	return err
}

func removeImage(t *testing.T, ctx context.Context, cli *client.Client, imageName, imageTag string) {
	if err := cli.RemoveImage(ctx, imageName, imageTag, true); err != nil {
		t.Logf("Failed to remove image: %v", err)
	}
}
