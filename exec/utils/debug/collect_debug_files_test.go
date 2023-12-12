// package debug
//
// firex records the text in the error log
package debug

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/logger"
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
	outDirFlag    = flag.String("outDir", "", "Directory where debug files should be copied")
	timestampFlag = flag.String("timestamp", "1", "Test start timestamp")
	coreFilesFlag = flag.Bool("core", false, "Check for core files that get generated")
	outDir        string
	timestamp     string
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
	showTechSupport = []string{
		"cef", "cef platform", "ofa", "insight", "rib", "fabric",
		"service-layer", "grpc", "spi", "hw-ac", "bundles", "cfgmgr",
		"ctrace", "ethernet interfaces", "fabric link-include", "p4rt",
		"interface", "optics", "pfi", "platform-fwd", "pbr", "rdsfs", "sysdb",
		"telemetry model-driven", "routing isis", "routing bgp", "linux networking",
		"install",
	}
	pipedCmds = []string{
		"show grpc trace all",
		"show telemetry model-driven trace all",
		"show cef global gribi aft internal location all",
		"show version",
		"show platform",
		"show install fixes active",
		"show running-config",
		"show context location all",
		"show logging",
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
		logger.Logger.Error().Msg(fmt.Sprintf("Could not set up NewTargets, [%v]", err))
		t.FailNow()
	}
	return &nt
}

// TestMain sets up Ondatra Init
func TestMain(m *testing.M) {
	os.Setenv("LOGLEVEL", "DEBUG")
	fptest.RunTests(m)
}

func TestCollectCoreFiles(t *testing.T) {
	targets := NewTargets(t)
	if *outDirFlag == "" {
		logger.Logger.Error().Msg(fmt.Sprintf("out directory flag not set correctly: [%s]", *outDirFlag))
		t.FailNow()
	} else {
		outDir = *outDirFlag
		logger.Logger.Info().Msg(fmt.Sprintf("out directory flag is: [%s]", *outDirFlag))
		timestamp = time.Now().Format(time.RFC3339Nano)
	}
	commands := []string{
		"run rm -rf /" + techDirectory,
		"mkdir " + techDirectory,
		"run find /misc/disk1 -maxdepth 1 -type f -name '*core*' -newermt @" + timestamp + " -exec cp \"{}\" /" + techDirectory + "/  \\\\;",
		//"run find /harddisk: -maxdepth 1 -type f -name '*core*' -newermt @" + timestamp + " -exec cp \"{}\" /" + techDirectory + "/  \\\\;",
	}

	for _, t := range showTechSupport {
		commands = append(commands, fmt.Sprintf("show tech-support %s file %s", t, getTechFileName(t)))
	}
	pipeCore := []string{"cd harddisk:", "dir | i *core*"}
	for _, t := range pipeCore {
		commands = append(commands, fmt.Sprintf("%s | file %s", t, getTechFileName(t)))
	}

	for dutID, targetInfo := range targets.targetInfo {
		t.Logf("Collecting debug files on %s", dutID)

		ctx := context.Background()
		cli := targets.GetOndatraCLI(t, dutID)

		for _, cmd := range commands {
			//fmt.Println(fmt.Sprintf("Running current command: [%s]", cmd))
			logger.Logger.Info().Msg(fmt.Sprintf("Running current command logger: [%s]", cmd))
			testt.CaptureFatal(t, func(t testing.TB) {
				if result, err := cli.SendCommand(ctx, cmd); err == nil {
					logger.Logger.Error().Msg(fmt.Sprintf("Error while running [%s] : [%v]", cmd, err))
					t.Logf("> %s", cmd)
					t.Log(result)
				} else {
					logger.Logger.Info().Msg(fmt.Sprintf("Command [%s] ran successfully", cmd))
					t.Logf("> %s", cmd)
					t.Log(err.Error())
				}
				t.Logf("\n")
			})
		}

		copyDebugFiles(t, targetInfo, "CollectCoreFiles")
	}
	fmt.Println("Exiting TestCollectionDebugFiles")
}
func TestCollectDebugFiles(t *testing.T) {
	logger.Logger.Debug().Msg("Function TestCollectionDebugFiles has started")
	// set up Targets
	targets := NewTargets(t)
	if *outDirFlag == "" {
		logger.Logger.Error().Msg(fmt.Sprintf("out directory flag not set correctly: [%s]", *outDirFlag))
		t.FailNow()
	} else {
		outDir = *outDirFlag
		logger.Logger.Info().Msg(fmt.Sprintf("out directory flag is: [%s]", *outDirFlag))
		timestamp = time.Now().Format(time.RFC3339Nano)
	}

	commands := []string{
		"run rm -rf /" + techDirectory,
		"mkdir " + techDirectory,
		"run find /misc/disk1 -maxdepth 1 -type f -name '*core*' -newermt @" + timestamp + " -exec cp \"{}\" /" + techDirectory + "/  \\\\;",
		//"run find /harddisk: -maxdepth 1 -type f -name '*core*' -newermt @" + timestamp + " -exec cp \"{}\" /" + techDirectory + "/  \\\\;",
	}

	for _, t := range showTechSupport {
		commands = append(commands, fmt.Sprintf("show tech-support %s file %s", t, getTechFileName(t)))
	}

	for _, t := range pipedCmds {
		commands = append(commands, fmt.Sprintf("%s | file %s", t, getTechFileName(t)))
	}

	for dutID, targetInfo := range targets.targetInfo {
		//t.Logf("Collecting debug files on %s", dutID)

		ctx := context.Background()
		cli := targets.GetOndatraCLI(t, dutID)

		for _, cmd := range commands {
			fmt.Println(fmt.Sprintf("Running current command: [%s]", cmd))
			logger.Logger.Info().Msg(fmt.Sprintf("Running current command: [%s]", cmd))
			testt.CaptureFatal(t, func(t testing.TB) {
				if result, err := cli.SendCommand(ctx, cmd); err == nil {
					logger.Logger.Error().Msg(fmt.Sprintf("Error while running [%s] : [%v]", cmd, err))
					t.Logf("> %s", cmd)
					t.Log(result)
				} else {
					logger.Logger.Info().Msg(fmt.Sprintf("Command [%s] ran successfully", cmd))
					t.Logf("> %s", cmd)
					t.Log(err.Error())
				}
				t.Logf("\n")
			})
		}

		copyDebugFiles(t, targetInfo, "CollectDebugFiles")
	}
	fmt.Println("Exiting TestCollectionDebugFiles")
}

func copyDebugFiles(t *testing.T, d targetInfo, filename string) {
	fmt.Println("Starting copyDebugFiles")
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

	dutOutDir := filepath.Join(outDir, d.dut, filename)
	if err := os.MkdirAll(dutOutDir, os.ModePerm); err != nil {
		t.Errorf("Error creating output directory: %v", err)
		return
	}

	if err := scpClient.CopyDirFromRemote("/"+techDirectory, dutOutDir, &scp.DirTransferOption{
		Timeout: scpCopyTimeout,
	}); err != nil {
		t.Errorf("Error copying debug files: %v", err)
	}
	fmt.Println("Exiting copyDebugFiles")
}

func (ti *Targets) getSSHInfo(t *testing.T) error {
	t.Helper()

	bf := flag.Lookup("binding")
	logger.Logger.Info().Msg(fmt.Sprintf("binding flag: [%s]", bf.Value.String()))
	var bindingFile string

	if bf == nil {
		logger.Logger.Error().Msg(fmt.Sprintf("binding file not set correctly : [%s]", bf.Value.String()))
		return fmt.Errorf(fmt.Sprintf("binding file not set correctly : [%s]", bf.Value.String()))
	}

	bindingFile = bf.Value.String()

	in, err := os.ReadFile(bindingFile)
	if err != nil {
		logger.Logger.Error().Msg(fmt.Sprintf("Error reading binding file: [%v]", err))
		return fmt.Errorf(fmt.Sprintf("Error reading binding file: [%v]", err))
	}
	logger.Logger.Info().Msg(string(in))
	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		logger.Logger.Error().Msg(fmt.Sprintf("Error unmarshalling binding file: [%v]", err))
		return fmt.Errorf(fmt.Sprintf("Error unmarshalling binding file: [%v]", err))
	}

	//targets := map[string]targetInfo{}
	for _, dut := range b.Duts {
		logger.Logger.Info().Msg(fmt.Sprintf("current dut :[%v]", dut))
		var sshUser, sshPass, sshIP, sshPort string
		var sshTarget []string
		if dut.Ssh != nil {
			logger.Logger.Debug().Msg(fmt.Sprintf("SSH: [%v]", dut.Ssh))
			sshUser = dut.Ssh.Username
			sshPass = dut.Ssh.Password

			if dut.Ssh.Target != "" {
				logger.Logger.Debug().Msg(fmt.Sprintf("SSH Target: [%s]", strings.Split(dut.Ssh.Target, ":")))
				sshTarget = strings.Split(dut.Ssh.Target, ":")
				sshIP = sshTarget[0]
				sshPort = "42823"
				if len(sshTarget) > 1 {
					sshPort = sshTarget[1]
				}
			}
		} else if dut.Options != nil {
			logger.Logger.Debug().Msg(fmt.Sprintf("SSH User: [%s]", dut.Options.Username))
			sshUser = dut.Options.Username
			sshPass = dut.Options.Password
		} else if b.Options != nil {
			logger.Logger.Debug().Msg(fmt.Sprintf("SSH User: [%s]", b.Options.Username))
			sshUser = b.Options.Username
			sshPass = b.Options.Password
		} else {
			logger.Logger.Error().Msg("SSH User/Password not found ")
		}

		if dut.Id != "" && sshIP != "" && sshPort != "" && sshUser != "" && sshPass != "" {
			ti.targetInfo[dut.Id] = targetInfo{
				dut:     dut.Id,
				sshIP:   sshIP,
				sshPort: sshPort,
				sshUser: sshUser,
				sshPass: sshPass,
			}
			fmt.Println(ti.targetInfo[dut.Id])

		} else {
			logger.Logger.Error().Msg("One or more values are empty  dut.Id, sshIP, sshPort, sshUser, sshPass ")
		}

	}
	return nil
}

// getTechFileName return the techDirecory + / + replacing " " with _
func getTechFileName(tech string) string {
	fmt.Println("Starting getTechFileName")
	return techDirectory + "/" + strings.ReplaceAll(tech, " ", "_")
}

// setCoreFile creates a core file. function to be deleted !!!!!!!!!!
func (ti *Targets) SetCoreFile(t *testing.T) {
	fmt.Println("Starting setCoreFile")
	t.Helper()

	cmd := "dumpcore suspended 52"

	for dutID := range ti.targetInfo {
		t.Logf("Collecting debug files on %s", dutID)

		ctx := context.Background()
		cli := ti.GetOndatraCLI(t, dutID)

		testt.CaptureFatal(t, func(t testing.TB) {
			if result, err := cli.SendCommand(ctx, cmd); err == nil {
				t.Logf("> %s", cmd)
				t.Log(result)
			} else {
				t.Logf("> %s", cmd)
				t.Log(err.Error())
			}
			t.Logf("\n")
		})

	}
	fmt.Println("Exiting setCoreFile")
}

// GetOndatraCLI
//
// returns a new streaming CLI client for the DUT.
func (ti *Targets) GetOndatraCLI(t *testing.T, dutID string) binding.CLIClient {
	t.Helper()
	dut := ondatra.DUT(t, dutID)

	return dut.RawAPIs().CLI(t)
}
