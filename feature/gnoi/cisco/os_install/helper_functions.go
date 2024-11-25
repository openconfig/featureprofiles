package osinstall_test

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
)

func cleanCommandOutput(output string) string {
	// Remove any leading and trailing whitespace
	output = strings.TrimSpace(output)

	// Split the output into lines
	lines := strings.Split(output, "\n")

	// Iterate through the lines and find the last non-empty line, which should contain the actual result
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}

	return ""
}

func cleanupDiskSpace(ctx context.Context, sshClient binding.CLIClient) error {
	cleanupCmd := "run rm /misc/disk1/disk_fill_file"
	if _, err := sshClient.RunCommand(ctx, cleanupCmd); err != nil {
		return fmt.Errorf("failed to clean up disk fill file: %w", err)
	}
	return nil
}

func retrieveAndFillDiskSpace(ctx context.Context, sshClient binding.CLIClient, thresholdGB float64) (bool, error) {
	// Retrieve available disk space
	spaceCmd := "run df -h /dev/mapper/main--xr--vg-install--data--disk1 | awk 'NR==2 {print $4}'"
	spaceResult, err := sshClient.RunCommand(ctx, spaceCmd)
	if err != nil {
		return false, fmt.Errorf("failed to get available space: %w", err)
	}

	// Clean the output using the utility function
	spaceStr := cleanCommandOutput(spaceResult.Output())

	// Determine the unit and parse the space
	var availableSpaceGB float64
	switch {
	case strings.HasSuffix(spaceStr, "G"):
		spaceGB, err := strconv.ParseFloat(strings.TrimSuffix(spaceStr, "G"), 64)
		if err != nil {
			return false, fmt.Errorf("failed to parse available space in GB: %w", err)
		}
		availableSpaceGB = spaceGB
	case strings.HasSuffix(spaceStr, "M"):
		spaceMB, err := strconv.ParseFloat(strings.TrimSuffix(spaceStr, "M"), 64)
		if err != nil {
			return false, fmt.Errorf("failed to parse available space in MB: %w", err)
		}
		availableSpaceGB = spaceMB / 1024
	default:
		return false, fmt.Errorf("unexpected space unit: %s", spaceStr)
	}

	// Check if disk fill should be skipped
	if availableSpaceGB < thresholdGB {
		return true, nil
	}

	// Fill the disk
	fillSpace := availableSpaceGB - 1
	fillCmd := fmt.Sprintf("run fallocate -l %dG /misc/disk1/disk_fill_file", int(fillSpace))
	if _, err := sshClient.RunCommand(ctx, fillCmd); err != nil {
		return false, fmt.Errorf("failed to fill the disk: %w", err)
	}

	return false, nil
}

// Function to get ISO version information
func getIsoVersionInfo(imagePath string) (string, error) {
	// Construct the command with the image path
	// cmd := exec.Command("/bin/isoinfo -R -x /mdata/build-info.txt -i", imagePath)
	cmd := exec.Command("/auto/ioxprojects13/lindt-giso/isols.py", "--iso", imagePath, "--build-info")

	// Run the command and capture the output
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run command: %v", err)
	}

	// Parse the command output to extract version and label
	output := out.String()
	version, label := parseVersionInfo(output)

	// Format the result as <version>-<label>
	result := version
	if label != "" {
		result += "-" + label
	}

	return result, nil
}

// Helper function to parse version information from command output
func parseVersionInfo(input string) (string, string) {
	var version, label string

	// Regex to find the version number
	versionRegex := regexp.MustCompile(`Version:\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+[A-Z]?)`)
	// versionRegex := regexp.MustCompile(`XR version =\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+[A-Z]?)`)
	versionMatches := versionRegex.FindStringSubmatch(input)

	if len(versionMatches) > 1 {
		version = versionMatches[1]
		// version = strings.ReplaceAll(version, "-LNT", "")
	}

	// Regex to find the label in GISO Build Command
	labelRegex := regexp.MustCompile(`--label\s+(\S+)`)
	labelMatches := labelRegex.FindStringSubmatch(input)
	if len(labelMatches) > 1 {
		label = labelMatches[1]
	}

	return version, label
}
func isGreater(epochTime1, epochTime2 int64) bool {
	return epochTime1 > epochTime2
}

// Extracts the creation time from the ls command output and converts it to epoch time
func extractCreationTime(output string) (int64, error) {
	// Regex to match the full datetime format including timezone
	regex := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+ [-+]\d{4}`)
	matches := regex.FindStringSubmatch(output)
	if len(matches) < 1 {
		return 0, fmt.Errorf("no match found for creation date and time")
	}

	// Parse the date and time into a time.Time object
	layout := "2006-01-02 15:04:05.000000000 -0700"
	creationTime, err := time.Parse(layout, matches[0])
	if err != nil {
		return 0, err
	}

	// Convert to epoch time (Unix timestamp)
	return creationTime.Unix(), nil
}

// Removes the specified ISO file on the DUT
// get version and the image file name is derived
func removeISOFile(t *testing.T, dut *ondatra.DUTDevice, version string) {
	cmd := fmt.Sprintf("run rm -rf /misc/disk1/8000-golden-x-%s.iso", version)
	util.SshRunCommand(t, dut, cmd)
}

// Lists the specified ISO file on the DUT and extracts its creation date and time
func listISOFile(t *testing.T, dut *ondatra.DUTDevice, version string) (int64, error) {
	cmd := fmt.Sprintf("run ls -l --full-time /misc/disk1/8000-golden-x-%s.iso", version)
	output := util.SshRunCommand(t, dut, cmd)

	// Extract date and time from the output
	creationTime, err := extractCreationTime(output)
	if err != nil {
		t.Logf("Error extracting creation time: %v", err)
		return 0, err
	}

	return creationTime, nil
}
