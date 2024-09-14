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
	techDirectory  = "harddisk:/firex_tech"
	scpCopyTimeout = 300 * time.Second
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
		"cef", "cef platform", "ofa", "insight", "rib", "fabric",
		"service-layer", "mgbl", "spi", "hw-ac", "bundles", "cfgmgr",
		"ctrace", "ethernet interfaces", "fabric link-include", "p4rt",
		"interface", "optics", "pfi", "platform-fwd", "pbr", "rdsfs", "sysdb",
		"telemetry model-driven", "routing isis", "routing bgp", "linux networking",
		"install",
	}

	pipedCmdList = []string{
		// "show grpc trace all",
		// "show telemetry model-driven trace all",
		// "show cef global gribi aft internal location all",
		// "show logging",
		"show version",
		"show platform",
		"show install fixes active",
		"show running-config",
		"show context location all",
		"show processes blocked location all",
		"show redundancy",
		"show reboot history detail",
	}
)

func NewTargets(t *testing.T) *Targets {
	t.Helper()
	// set up all ssh for the targets
	nt := Targets{
		targetInfo: make(map[string]targetInfo),
	}

	err := nt.getSSHInfo(t)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return &nt
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCollectDebugFiles(t *testing.T) {
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

	deviceDirCleanupCmds := []string{
		"run rm -rf /" + techDirectory,
		"mkdir " + techDirectory,
	}
	commands := []string{}

	if *coreCheck {
		commands = append(commands,
			"run find /misc/disk1 -maxdepth 1 -type f -name '*core*' -newermt @"+*timestamp+" -exec cp \"{}\" /"+techDirectory+"/  \\\\;",
		)
	}

	var wg sync.WaitGroup
	for dutID, target := range targets.targetInfo {
		fileNamePrefix := ""
		if !*splitPerDut && len(targets.targetInfo) > 1 {
			fileNamePrefix = dutID + "_"
		}
		ctx := context.Background()
		dut := ondatra.DUT(t, dutID)
		sshClient := dut.RawAPIs().CLI(t)
		for _, cmd := range deviceDirCleanupCmds {
			testt.CaptureFatal(t, func(t testing.TB) {
				if result, err := sshClient.RunCommand(ctx, cmd); err == nil {
					t.Logf("> %s", cmd)
					t.Log(result.Output())
				} else {
					t.Logf("> %s", cmd)
					t.Log(err.Error())
				}
				t.Logf("\n")
			})
		}
		wg.Add(1)
		go func(dutID string, target targetInfo) {
			defer wg.Done()
			executeCommandsForDUT(t, dutID, target, fileNamePrefix, commands)
		}(dutID, target)
	}
	wg.Wait()
}

func executeCommandsForDUT(t *testing.T, dutID string, target targetInfo, fileNamePrefix string, commands []string) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	dut := ondatra.DUT(t, dutID)
	sshClient := dut.RawAPIs().CLI(t)

	executeCommandsInParallel(t, ctx, sshClient, commands)
	copyDebugFiles(t, target)
}

func executeCommandsInParallel(t *testing.T, ctx context.Context, sshClient binding.CLIClient, commands []string) {
	var wg sync.WaitGroup
	commandResults := make(chan string, len(commands))

	for _, cmd := range commands {
		wg.Add(1)
		go func(cmd string) {
			defer wg.Done()
			testt.CaptureFatal(t, func(t testing.TB) {
				if result, err := sshClient.RunCommand(ctx, cmd); err == nil {
					commandResults <- fmt.Sprintf("> %s\n%s\n", cmd, result.Output())
				} else {
					commandResults <- fmt.Sprintf("> %s\n%s\n", cmd, err.Error())
				}
			})
		}(cmd)
	}

	go func() {
		wg.Wait()
		close(commandResults)
	}()

	for result := range commandResults {
		t.Log(result)
	}
}

func copyDebugFiles(t *testing.T, d targetInfo) {
	t.Helper()

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
}

func (ti *Targets) getSSHInfo(t *testing.T) error {
	t.Helper()

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
	return nil
}

// getTechFilePath return the techDirecory + / + replacing " " with _
func getTechFilePath(tech string, prefix string) string {
	return filepath.Join(techDirectory, prefix+strings.ReplaceAll(tech, " ", "_"))
}

func findCoreFile(t *testing.T, pathToMonitor string) {
	t.Logf("Processing existing core files in directory: %s\n", pathToMonitor)
	fmt.Printf("Processing existing core files in directory: %s\n", pathToMonitor)

	coreFiles := make(chan string)
	var wg sync.WaitGroup

	// Start a fixed number of worker goroutines
	numWorkers := 4
	for i := 0; i < numWorkers; i++ {
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
		fmt.Printf("Error walking the path %s: %v\n", pathToMonitor, err)
	}

	close(coreFiles)
	wg.Wait()
}

func decodeCoreFile(t *testing.T, coreFile string) {
	txtFile := strings.TrimSuffix(coreFile, filepath.Ext(coreFile)) + ".txt"
	coreDir := filepath.Dir(coreFile)

	t.Logf("Decoding core file: %s\n", coreFile)
	t.Logf("Corresponding TXT file: %s\n", txtFile)
	t.Logf("Core file directory: %s\n", coreDir)
	fmt.Printf("Decoding core file: %s\n", coreFile)
	fmt.Printf("Corresponding TXT file: %s\n", txtFile)
	fmt.Printf("Core file directory: %s\n", coreDir)

	// Check if the .txt file exists
	if _, err := os.Stat(txtFile); os.IsNotExist(err) {
		t.Logf("TXT file %s not found for core file %s\n", txtFile, coreFile)
		fmt.Printf("TXT file %s not found for core file %s\n", txtFile, coreFile)
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
	currentTime := time.Now()
	timeString := currentTime.Format("2006-01-02 15:04:05")
	t.Logf("corefile %s, background decode start time %s", coreFile, timeString)
	cmd := exec.Command("sh", "-c", fmt.Sprintf("/auto/mcp-project1/xr-decoder/xr-decode -l %s > %s && rm %s &", coreFile, decodeOutput, inProgressFile))
	currentTime = time.Now()
	timeString = currentTime.Format("2006-01-02 15:04:05")
	t.Logf("corefile %s, background decode end time %s", coreFile, timeString)
	if err := cmd.Start(); err != nil {
		t.Logf("Error starting decode command: %v\n", err)
		return
	}
	currentTime = time.Now()
	timeString = currentTime.Format("2006-01-02 15:04:05")
	t.Logf("corefile %s, background decode error time %s", coreFile, timeString)
	t.Logf("Started background decoding for core file %s\n", coreFile)

	cmd = exec.Command("ps", "-elf", fmt.Sprintf("| grep %s", coreFile))
	output, err := cmd.Output()
	if err != nil {
		t.Logf("Error ps command: %v\n", err)
		return
	}
	t.Logf("ps output: %v", output)

	// // Decode the core file
	// decodeOutput := filepath.Join(coreDir, filepath.Base(coreFile)+".decoded.txt")
	// t.Logf("Decoding output will be saved to: %s\n", decodeOutput)

	// cmd := exec.Command("/auto/mcp-project1/xr-decoder/xr-decode", "-l", coreFile)
	// output, err := cmd.Output()
	// if err != nil {
	// 	t.Logf("Error decoding core file: %v\n", err)
	// 	return
	// }

	// if err := os.WriteFile(decodeOutput, output, 0644); err != nil {
	// 	t.Logf("Error writing decode output: %v\n", err)
	// 	return
	// }

	// t.Logf("Decoded core file %s and placed the result in %s\n", coreFile, coreDir)
}
