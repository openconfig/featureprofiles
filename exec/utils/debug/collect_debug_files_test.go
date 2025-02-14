package debug

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/testt"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	techDirectory        = "harddisk:/firex_tech" // Directory for storing tech support files
	scpCopyTimeout       = 300 * time.Second      // Timeout for SCP copy operations
	maxParallelExecutors = 4                      // maximum concurrent go routine for command execution and corefile decode
)

var (
	outDir      = flag.String("outDir", "", "Directory where debug files should be copied")
	timestamp   = flag.String("timestamp", "1", "Test start timestamp")
	coreCheck   = flag.Bool("coreCheck", false, "Check for core file")
	collectTech = flag.Bool("collectTech", false, "Collect show tech")
	runCmds     = flag.Bool("runCmds", false, "Run commands")
	splitPerDut = flag.Bool("splitPerDut", false, "Create a folder for each dut")
	showTechs   = flag.String("showtechs", "", "Comma-separated list of show techs")
	cmds        = flag.String("cmds", "", "Comma-separated list of commands")
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

var (
	showTechList = []string{
		"cef",
		"cef platform",
		"ofa",
		"insight",
		"rib",
		"fabric",
		"service-layer",
		"mgbl",
		"spi",
		"hw-ac",
		"bundles",
		"cfgmgr",
		"ctrace",
		"ethernet interfaces",
		"fabric link-include",
		"p4rt",
		"interface",
		"optics",
		"pfi",
		"platform-fwd",
		"pbr",
		"rdsfs",
		"sysdb",
		"telemetry model-driven",
		"routing isis",
		"routing bgp",
		"linux networking",
		"install",
		"health",
		"lldp",
		"spio",
		"gnsi",
	}

	pipedCmdList = []string{
		"show grpc trace all",
		"show telemetry model-driven trace all",
		"show cef global gribi aft internal location all",
		"show logging",
		"show version",
		"show platform",
		"show install fixes active",
		"show running-config",
		"show context location all",
		"show processes blocked location all",
		"show redundancy",
		"show reboot history detail",
		"show insight database entry all",
	}
)

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

// TestMain is the entry point for running tests
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestCollectDebugFiles collects debug files from the targets
func TestCollectDebugFiles(t *testing.T) {
	t.Log("Starting TestCollectDebugFiles")
	targets := NewTargets(t)
	if *outDir == "" {
		t.Fatalf("outDir was not set")
	}

	if *showTechs != "" {
		showTechList = strings.Split(*showTechs, ",")
	}

	if *cmds != "" {
		pipedCmdList = append(pipedCmdList, strings.Split(*cmds, ",")...)
	}

	commands := []string{}
	if *coreCheck {
		commands = append(commands,
			"run find /misc/disk1 -maxdepth 1 -type f -name '*core*' -newermt @"+*timestamp+" -exec cp \"{}\" /"+techDirectory+"/  \\\\;",
		)
		// handle new corefile path /misc/disk1/coredumps
		commands = append(commands,
			"run find /misc/disk1/coredumps -maxdepth 1 -type f -name '*core*' -newermt @"+*timestamp+" -exec cp \"{}\" /"+techDirectory+"/  \\\\;",
		)
	}

	var wg sync.WaitGroup
	for dutID, target := range targets.targetInfo {
		fileNamePrefix := ""
		if !*splitPerDut && len(targets.targetInfo) > 1 {
			fileNamePrefix = dutID + "_"
		}
		wg.Add(1)
		go func(dutID string, target targetInfo) {
			defer wg.Done()
			t.Logf("Executing commands for DUT: %s", dutID)
			executeCommandsForDUT(t, dutID, target, fileNamePrefix, commands)
		}(dutID, target)
	}
	wg.Wait()

	t.Log("Completed TestCollectDebugFiles")
}

// executeCommandsForDUT executes commands for a specific DUT
func executeCommandsForDUT(t *testing.T, dutID string, target targetInfo, fileNamePrefix string, commands []string) {
	t.Logf("Starting executeCommandsForDUT for DUT: %s", dutID)
	if *collectTech {
		for _, t := range showTechList {
			fname := getTechFilePath(t, fileNamePrefix)
			if t == "sanitizer" {
				fname = filepath.Join(techDirectory, "showtech-sanitizer-"+dutID)
			}
			commands = append(commands, fmt.Sprintf("show tech-support %s file %s", t, fname))
		}
	}

	if *runCmds {
		for _, t := range pipedCmdList {
			commands = append(commands, fmt.Sprintf("%s | file %s", t, getTechFilePath(t, fileNamePrefix)))
		}
	}

	deviceDirCleanupCmds := []string{
		"run rm -rf /" + techDirectory,
		"mkdir " + techDirectory,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	dut := ondatra.DUT(t, dutID)
	sshClient := dut.RawAPIs().CLI(t)

	for _, cmd := range deviceDirCleanupCmds {
		testt.CaptureFatal(t, func(t testing.TB) {
			if result, err := sshClient.RunCommand(ctx, cmd); err == nil {
				t.Logf("%s> %s", dutID, cmd)
				t.Log(result.Output())
			} else {
				t.Logf("%s> %s", dutID, cmd)
				t.Log(err.Error())
			}
			t.Logf("\n")
		})
	}

	executeCommandsInParallel(t, ctx, dutID, sshClient, commands)
	copyDebugFiles(t, target)
	t.Logf("Completed executeCommandsForDUT for DUT: %s", dutID)
}

// executeCommandsInParallel executes commands in parallel
func executeCommandsInParallel(t *testing.T, ctx context.Context, dutID string, sshClient binding.CLIClient, commands []string) {
	t.Logf("Starting executeCommandsInParallel for DUT: %s", dutID)
	exeCommands := make(chan string)
	var wg sync.WaitGroup

	// Start a fixed number of worker goroutines
	for i := 0; i < maxParallelExecutors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cmd := range exeCommands {
				executeCommand(t, ctx, dutID, sshClient, cmd)
			}
		}()
	}

	// iterate the commands and send command  to the workers
	for _, command := range commands {
		exeCommands <- command
	}

	close(exeCommands)
	wg.Wait()
	t.Logf("Completed executeCommandsInParallel for DUT: %s", dutID)
}

func executeCommand(t *testing.T, ctx context.Context, dutID string, sshClient binding.CLIClient, cmd string) {
	if result, err := sshClient.RunCommand(ctx, cmd); err == nil {
		t.Logf("%s> %s", dutID, cmd)
		t.Log(result.Output())
	} else {
		t.Logf("%s> %s", dutID, cmd)
		t.Log(err.Error())
	}
	t.Logf("\n")
}

// copyDebugFiles copies debug files from the target to the local machine
func copyDebugFiles(t *testing.T, d targetInfo) {
	t.Helper()
	t.Logf("Starting copyDebugFiles for DUT: %s", d.dut)

	target := fmt.Sprintf("%s:%s", d.sshIP, d.sshPort)
	t.Logf("Copying debug files from %s (%s)", d.dut, target)

	sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
	scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
	if err != nil {
		t.Errorf("Error initializing scp client: %v", err)
		return
	}
	defer scpClient.Close()

	var dutOutDir string
	if *splitPerDut {
		dutOutDir = filepath.Join(*outDir, d.dut)
	} else {
		dutOutDir = *outDir
	}

	if err := os.MkdirAll(dutOutDir, os.ModePerm); err != nil {
		t.Errorf("Error creating output directory: %v", err)
		return
	}

	if err := scpClient.CopyDirFromRemote("/"+techDirectory, dutOutDir, &scp.DirTransferOption{
		Timeout: scpCopyTimeout,
	}); err != nil {
		t.Errorf("Error copying debug files: %v", err)
	}
	findCoreFile(t, dutOutDir)
	t.Logf("Completed copyDebugFiles for DUT: %s", d.dut)
}

// getSSHInfo retrieves SSH information for each target from the binding file
func (ti *Targets) getSSHInfo(t *testing.T) error {
	t.Helper()
	t.Log("Starting getSSHInfo")

	bf := flag.Lookup("binding")
	var bindingFile string

	if bf == nil {
		t.Logf(fmt.Sprintf("binding file not set correctly : [%s]", bf.Value.String()))
		return fmt.Errorf(fmt.Sprintf("binding file not set correctly : [%s]", bf.Value.String()))
	}

	bindingFile = bf.Value.String()

	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Logf(fmt.Sprintf("Error reading binding file: [%v]", err))
		return fmt.Errorf(fmt.Sprintf("Error reading binding file: [%v]", err))
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return fmt.Errorf(fmt.Sprintf("Error unmarshalling binding file: [%v]", err))
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

// getTechFilePath returns the techDirectory + / + replacing " " with _
func getTechFilePath(tech string, prefix string) string {
	return filepath.Join(techDirectory, prefix+strings.ReplaceAll(tech, " ", "_"))
}

// findCoreFile processes existing core files in the specified directory
func findCoreFile(t *testing.T, pathToMonitor string) {
	t.Logf("Processing existing core files in directory: %s\n", pathToMonitor)

	coreFiles := make(chan string)
	var wg sync.WaitGroup

	// Start a fixed number of worker goroutines
	for i := 0; i < maxParallelExecutors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for coreFile := range coreFiles {
				decodeCoreFile(t, coreFile)
			}
		}()
	}

	// Walk the directory and send core files to the workers
	err := filepath.Walk(pathToMonitor, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(info.Name(), ".core") || strings.HasSuffix(info.Name(), ".core.gz")) {
			t.Logf("Found existing core file: %s\n", path)
			coreFiles <- path
		}
		return nil
	})

	if err != nil {
		t.Logf("Error walking the path %s: %v\n", pathToMonitor, err)
	}

	close(coreFiles)
	wg.Wait()
	t.Log("Completed findCoreFile")
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
	// Check if 'buildid-db' file exists in the workspace directory
	buildIdDbPath := filepath.Join(workspace, "buildid-db")
	var cmd *exec.Cmd
	if _, err := os.Stat(buildIdDbPath); err == nil {
		// Use command with -l option if 'buildid-db' exists
		cmd = exec.Command("sh", "-c", fmt.Sprintf("/auto/mcp-project1/xr-decoder/xr-decode -l %s 2>&1 %s && rm %s &", coreFile, decodeOutput, inProgressFile))
		t.Logf("Using command with -l option")
	} else {
		// Use command without -l option if 'buildid-db' does not exist
		cmd = exec.Command("sh", "-c", fmt.Sprintf("/auto/mcp-project1/xr-decoder/xr-decode %s 2>&1 %s && rm %s &", coreFile, decodeOutput, inProgressFile))
		t.Logf("Using command without -l option")
	}

	if err := cmd.Start(); err != nil {
		t.Logf("Error starting decode command: %v\n", err)
		return
	}
	t.Logf("Started background decoding for core file %s\n", coreFile)
}
