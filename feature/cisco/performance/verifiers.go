package performance

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	// "github.com/openconfig/featureprofiles/exec/utils/debug"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"
	"gopkg.in/yaml.v2"
)

type targetInfo struct {
	dut     string
	sshIP   string
	sshPort string
	sshUser string
	sshPass string
}

type Targets struct {
	targetInfo map[string]targetInfo
}

const (
	localLogDirectory    = "harddisk:/firex_log_directory" // Directory for storing tech support files on the router
	scpCopyTimeout       = 300 * time.Second               // Timeout for SCP copy operations
	maxParallelExecutors = 4                               // maximum concurrent go routine for command execution and corefile decode
)

var (
	// Global map to keep track of the number of times CollectRouterLogs is called for each DUT
	dutCallCounts = make(map[string]int)
	// Mutex to ensure thread-safe access to the map
	dutCallCountsMutex sync.Mutex

	// Global nested map to store the last log line for each DUT and each command
	dutCommandLastLogLines = make(map[string]map[string]string)
	// Mutex to ensure thread-safe access to the last log lines map
	dutCommandLastLogLinesMutex sync.Mutex

	// Global map to store the start time for each DUT
	dutStartTimes = make(map[string]int64)
	// Mutex to ensure thread-safe access to the start times map
	dutStartTimesMutex sync.Mutex
)

// ParseYAML parses the YAML file and returns the command patterns as a map
func ParseYAML(filePath string) (map[string]map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config map[string]map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return config, nil
}

// start collects the current time from the router and stores it in epoch format
func Start(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) error {
	sshClient := dut.RawAPIs().CLI(t)

	epochTime, err := getTime(ctx, sshClient, t)
	if err != nil {
		return fmt.Errorf("error getting time: %v", err)
	}

	dutStartTimesMutex.Lock()
	dutStartTimes[dut.Name()] = epochTime
	dutStartTimesMutex.Unlock()

	t.Logf("Start time for DUT %s stored as %d", dut.Name(), epochTime)
	return nil
}

// getTime executes the "show clock" command and returns the current time in epoch format
func getTime(ctx context.Context, client binding.CLIClient, t *testing.T) (int64, error) {
	command := "show clock"
	output, err := executeSSHCommand(ctx, client, command, t)
	list := strings.Split(output, "\r\n")
	output = list[len(list)-2]
	if err != nil {
		return 0, err
	}

	// Parse the output to get the current time
	timeStr := strings.TrimSpace(output)
	parsedTime, err := time.Parse("15:04:05.000 MST Mon Jan 2 2006", timeStr)
	if err != nil {
		return 0, fmt.Errorf("error parsing time: %v", err)
	}

	return parsedTime.Unix(), nil
}

// CollectRouterLogs connects to the router, retrieves logs, and saves them to the specified folder.
func CollectRouterLogs(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, logDir string, testName string, commandPatterns map[string]map[string]interface{}) error {
	count := updateAndPrintCallCount(dut.Name())
	sshClient := dut.RawAPIs().CLI(t)
	targets := NewTargets(t)

	t.Logf("Remote log directory: %v", logDir)
	//  Folder initial clean up
	if count == 1 {
		_, err := cleanLogDirectory(ctx, sshClient, localLogDirectory, t)
		if err != nil {
			return err
		}
		_, err = createRemoteDirectory(ctx, sshClient, localLogDirectory, t)
		if err != nil {
			return err
		}
	}

	var collectedErrors []string

	for command, details := range commandPatterns {
		commandType := details["type"].(string)
		showLoggingFile := filepath.Join(localLogDirectory, fmt.Sprintf("%d-%s-%s", count, dut.Name(), strings.ReplaceAll(command, " ", "_")))
		var cmd string

		switch commandType {
		case "logging":
			lastLogLine := getLastLogLine(dut.Name(), command)
			if lastLogLine != "" {
				cmd = fmt.Sprintf("%s | begin %s | file %s", command, lastLogLine, showLoggingFile)
			} else {
				cmd = fmt.Sprintf("%s | file %s", command, showLoggingFile)
			}
		case "command":
			cmd = fmt.Sprintf("%s | file %s", command, showLoggingFile)
		case "show-tech":
			cmd = fmt.Sprintf("%s file %s", command, showLoggingFile)
		default:
			collectedErrors = append(collectedErrors, fmt.Sprintf("Unknown command type %s for command %s", commandType, command))
			continue
		}

		if _, err := executeSSHCommand(ctx, sshClient, cmd, t); err != nil {
			collectedErrors = append(collectedErrors, fmt.Sprintf("Error executing command %s: %v", command, err))
			continue
		}
		if commandType == "show-tech" {
			showLoggingFile += ".tgz"
		}

		dutOutDir := filepath.Join(logDir, "debug_files", fmt.Sprintf("%v_%s_%s", count, dut.ID(), testName))

		for dutID, target := range targets.targetInfo {
			if dutID == dut.Name() {
				t.Logf("copy specific file from: %s device: %s to: %s", showLoggingFile, dut.Name(), dutOutDir)
				if err := copySpecificFile(t, target, dutOutDir, showLoggingFile); err != nil {
					collectedErrors = append(collectedErrors, fmt.Sprintf("Error copying file for command %s: %v", command, err))
					continue
				}
			}
		}

		localLogPath := filepath.Join(dutOutDir, filepath.Base(showLoggingFile))
		logContent, err := os.ReadFile(localLogPath)
		if err != nil {
			collectedErrors = append(collectedErrors, fmt.Sprintf("Error reading log file for command %s: %v", command, err))
			continue
		}

		if commandType == "logging" {
			if gotLastLogLine, err := GetLastLogLine(string(logContent)); err != nil {
				t.Logf("File Empty, Failed to get last log line: %v", err)
			} else {
				storeLastLogLine(dut.Name(), command, gotLastLogLine)
			}
		}
		if errorPatterns, ok := details["errorPatterns"].([]interface{}); ok {

			// Check for error patterns in the log content
			matched := CheckForErrorPatterns(errorPatterns, string(logContent))
			fmt.Printf("Matched Error Patterns: %v\n", matched)
			if len(matched) > 0 {
				collectedErrors = append(collectedErrors, fmt.Sprintf("Count: %d Error patterns matched for command %s", len(matched), command))
			}
		} else {
			fmt.Println("Error: 'errorPatterns' is not a list of interfaces.")
		}

	}

	target := targets.targetInfo[dut.Name()]

	// Collect core files
	dutStartTimesMutex.Lock()
	startTime, exists := dutStartTimes[dut.Name()]
	dutStartTimesMutex.Unlock()

	if exists {
		coreFilesCmd := fmt.Sprintf("run find /misc/disk1 -maxdepth 1 -type f -name '*core*' -newermt @%d", startTime)
		coreFilesOutput, err := executeSSHCommand(ctx, sshClient, coreFilesCmd, t)
		if err != nil {
			collectedErrors = append(collectedErrors, fmt.Sprintf("Error executing core files command: %v", err))
		} else {
			coreFiles := strings.Split(strings.TrimSpace(coreFilesOutput), "\r\n")
			dutOutDir := filepath.Join(logDir, fmt.Sprintf("%v", count))
			for _, coreFile := range coreFiles {
				if strings.Contains(coreFile, "/misc/disk1/") {
					coreFile = strings.Replace(coreFile, "/misc/disk1/", "harddisk:/", 1)
					err := copySpecificFile(t, target, dutOutDir, coreFile)
					if err != nil {
						collectedErrors = append(collectedErrors, fmt.Sprintf("Error copying core file %s: %v", coreFile, err))
					} else {
						coreFileName := strings.Replace(coreFile, "harddisk:/", "", 1)
						coreFilepath := filepath.Join(dutOutDir, coreFileName)
						if strings.Contains(coreFilepath, "core.gz") {
							decodeCoreFile(t, coreFilepath)
						}

					}
				}
			}
		}
	}

	// Update the start time after collecting logs
	if err := Start(ctx, t, dut); err != nil {
		collectedErrors = append(collectedErrors, fmt.Sprintf("Error updating start time: %v", err))
	}

	if len(collectedErrors) > 0 {
		return fmt.Errorf("errors encountered during log collection:\n%s", strings.Join(collectedErrors, "\n"))
	}

	return nil
}

// updateAndPrintCallCount updates the call count for the specified DUT and prints the result.
func updateAndPrintCallCount(dutName string) int {
	dutCallCountsMutex.Lock()
	defer dutCallCountsMutex.Unlock()

	dutCallCounts[dutName]++
	count := dutCallCounts[dutName]
	fmt.Printf("CollectRouterLogs has been called %d times for DUT: %s\n", count, dutName)
	return count
}

// getLastLogLine retrieves the last log line for the specified DUT and command.
func getLastLogLine(dutName, command string) string {
	dutCommandLastLogLinesMutex.Lock()
	defer dutCommandLastLogLinesMutex.Unlock()

	if commands, exists := dutCommandLastLogLines[dutName]; exists {
		return commands[command]
	}
	return ""
}

// storeLastLogLine stores the last log line for the specified DUT and command.
func storeLastLogLine(dutName, command, lastLogLine string) {
	dutCommandLastLogLinesMutex.Lock()
	defer dutCommandLastLogLinesMutex.Unlock()

	if _, exists := dutCommandLastLogLines[dutName]; !exists {
		dutCommandLastLogLines[dutName] = make(map[string]string)
	}
	dutCommandLastLogLines[dutName][command] = lastLogLine
}

// createRemoteDirectory ensures that the specified directory exists on the remote machine.
func createRemoteDirectory(ctx context.Context, client binding.CLIClient, dir string, t *testing.T) (string, error) {
	command := fmt.Sprintf("mkdir %s", dir) // direct IOS XR command
	t.Logf("Creating log directory: %v", command)
	commandOuptut, err := executeSSHCommand(ctx, client, command, t)
	return commandOuptut, err
}

// cleanRemoteDirectory ensures that the specified directory exists on the remote machine.
func cleanLogDirectory(ctx context.Context, client binding.CLIClient, dir string, t *testing.T) (string, error) {
	command := fmt.Sprintf("run rm -rf /%s", dir)
	t.Logf("Cleaning log directory: %v", command)
	commandOuptut, err := executeSSHCommand(ctx, client, command, t)
	return commandOuptut, err
}

// CheckForErrorPatterns checks the logs for the presence of error patterns.
func CheckForErrorPatterns(patterns []interface{}, logContent string) []string {
	var matchedPatterns []string
	for _, pattern := range patterns {
		switch p := pattern.(type) {
		case string:
			// Treat as a simple substring match
			if matched, _ := regexp.MatchString(p, logContent); matched {
				matchedPatterns = append(matchedPatterns, p)
			}
		case *regexp.Regexp:
			// Treat as a regex pattern
			if p.MatchString(logContent) {
				matchedPatterns = append(matchedPatterns, p.String())
			}
		default:
			fmt.Println("Unsupported pattern type")
		}
	}
	return matchedPatterns
}

// GetLastLogLine extracts the last log line entry.
func GetLastLogLine(logContent string) (string, error) {
	logLines := strings.Split(logContent, "\n")
	// length is always 1 even when no log is there
	if len(logLines) > 1 {
		lastLine := logLines[len(logLines)-2]
		if len(lastLine) >= 38 {
			lastLine = lastLine[0:38]
		}
		return lastLine, nil
	}
	return "", fmt.Errorf("no log lines found")
}

// executeSSHCommand is a helper function to execute SSH commands.
func executeSSHCommand(ctx context.Context, client binding.CLIClient, command string, t *testing.T) (string, error) {
	t.Logf("Executing SSH command: %s", command)
	output, err := client.RunCommand(ctx, command)
	if err != nil {
		return "", err
	}
	return string(output.Output()), nil
}

// NewTargets initializes a new Targets instance and sets up SSH information for each target
func NewTargets(t *testing.T) *Targets {
	t.Helper()
	nt := Targets{
		targetInfo: make(map[string]targetInfo),
	}

	err := nt.getSSHInfo(t)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return &nt
}

// getSSHInfo retrieves SSH information for each target from the binding file
func (ti *Targets) getSSHInfo(t *testing.T) error {
	t.Helper()
	t.Log("Starting getSSHInfo")

	bf := flag.Lookup("binding")
	var bindingFile string

	if bf == nil {
		t.Logf("binding file not set correctly: [%v]", bf)
		return fmt.Errorf("binding file not set correctly: [%v]", bf)
	}

	bindingFile = bf.Value.String()

	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Logf("Error reading binding file: [%v]", err)
		return fmt.Errorf("error reading binding file: [%v]", err)
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return fmt.Errorf("error unmarshalling binding file: [%v]", err)
	}

	for _, dut := range b.Duts {
		sshUser := dut.Ssh.Username
		if sshUser == "" {
			sshUser = dut.Options.Username
		}
		if sshUser == "" {
			sshUser = b.Options.Username
		}

		sshPass := dut.Ssh.Password
		if sshPass == "" {
			sshPass = dut.Options.Password
		}
		if sshPass == "" {
			sshPass = b.Options.Password
		}

		sshTarget := strings.Split(dut.Ssh.Target, ":")
		sshIP := sshTarget[0]
		sshPort := "22"
		if len(sshTarget) > 1 {
			sshPort = sshTarget[1]
		}

		if dut.Id != "" && sshIP != "" && sshPort != "" && sshUser != "" && sshPass != "" {
			ti.targetInfo[dut.Id] = targetInfo{
				dut:     dut.Id,
				sshIP:   sshIP,
				sshPort: sshPort,
				sshUser: sshUser,
				sshPass: sshPass,
			}
		} else {
			switch {
			case dut.Id == "":
				return fmt.Errorf("dut.Id is empty for dut: [%v]", dut)
			case sshIP == "":
				return fmt.Errorf("sshIP is empty for dut: [%v]", dut)
			case sshPort == "":
				return fmt.Errorf("sshPort is empty for dut: [%v]", dut)
			case sshUser == "":
				return fmt.Errorf("sshUser is empty for dut: [%v]", dut)
			case sshPass == "":
				return fmt.Errorf("sshPass is empty for dut: [%v]", dut)
			}
		}
	}
	t.Log("Completed getSSHInfo")
	return nil
}

// copySpecificFile copies a specific file from the target to the local machine.
func copySpecificFile(t *testing.T, d targetInfo, logDir string, fileToCopy string) error {
	t.Helper()
	t.Logf("Starting copySpecificFile for DUT: %s", d.dut)

	target := fmt.Sprintf("%s:%s", d.sshIP, d.sshPort)
	t.Logf("Copying specific file from %s (%s)", d.dut, target)

	sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
	scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
	if err != nil {
		return fmt.Errorf("error initializing scp client: %v", err)
	}
	defer scpClient.Close()

	dutOutDir := logDir

	if err := os.MkdirAll(dutOutDir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	remoteFilePath := fmt.Sprintf("/%s", fileToCopy)
	localFilePath := filepath.Join(dutOutDir, filepath.Base(fileToCopy))
	t.Logf("Executing SCP command: Copy from %s to %s", remoteFilePath, localFilePath)
	if err := scpClient.CopyFileFromRemote(remoteFilePath, localFilePath, &scp.FileTransferOption{
		Timeout: scpCopyTimeout,
	}); err != nil {
		return fmt.Errorf("error copying file %s: %v", fileToCopy, err)
	}

	t.Logf("Completed copySpecificFile for DUT: %s", d.dut)
	return nil
}

// decodeCoreFile decodes a core file and logs the output
func decodeCoreFile(t *testing.T, coreFile string) {
	t.Logf("Starting decodeCoreFile for core file: %s", coreFile)
	txtFile := strings.TrimSuffix(coreFile, filepath.Ext(coreFile)) + ".txt"
	coreDir := filepath.Dir(coreFile)

	t.Logf("Decoding core file: %s\n", coreFile)
	t.Logf("Corresponding TXT file: %s\n", txtFile)
	t.Logf("Core file directory: %s\n", coreDir)

	// Check if the .txt file exists
	if _, err := os.Stat(txtFile); os.IsNotExist(err) {
		t.Logf("TXT file %s not found for core file %s\n", txtFile, coreFile)
		return
	}

	// Read the workspace path from the .txt file
	file, err := os.Open(txtFile)
	if err != nil {
		t.Logf("Error opening TXT file: %v\n", err)
		return
	}
	defer file.Close()

	var workspace string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Workspace") {
			workspace = strings.Split(line, " = ")[1]
			break
		}
	}

	if err := scanner.Err(); err != nil {
		t.Logf("Error reading TXT file: %v\n", err)
		return
	}

	t.Logf("Workspace path: %s\n", workspace)

	// Change to the workspace directory
	if _, err := os.Stat(workspace); os.IsNotExist(err) {
		t.Logf("Workspace directory %s not found\n", workspace)
		return
	}

	if err := os.Chdir(workspace); err != nil {
		t.Logf("Error changing to workspace directory: %v\n", err)
		return
	}

	t.Logf("Changed to workspace directory: %s\n", workspace)

	// Wait until the core file is completely copied
	var prevSize int64 = -1
	var currSize int64 = 0
	for prevSize != currSize {
		prevSize = currSize
		time.Sleep(5 * time.Second)
		fileInfo, err := os.Stat(coreFile)
		if err != nil {
			t.Logf("Error getting core file size: %v\n", err)
			return
		}
		currSize = fileInfo.Size()
	}

	// Create a temporary file to indicate that decoding is in progress
	inProgressFile := coreFile + ".decode_in_progress"
	if _, err := os.Create(inProgressFile); err != nil {
		t.Logf("Error creating in-progress file: %v\n", err)
		return
	}

	// Decode the core file in the background
	decodeOutput := filepath.Join(coreDir, filepath.Base(coreFile)+".decoded.txt")

	cmd := exec.Command("sh", "-c", fmt.Sprintf("/auto/mcp-project1/xr-decoder/xr-decode -l %s > %s && rm %s &", coreFile, decodeOutput, inProgressFile))

	if err := cmd.Start(); err != nil {
		t.Logf("Error starting decode command: %v\n", err)
		return
	}
	t.Logf("Started background decoding for core file %s\n", coreFile)

}
