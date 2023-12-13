// package debug
//
// firex records the text in the error log
package debug

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/logger"
	"github.com/openconfig/testt"
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

// NewTargets initialize the Targets
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

// TestMain sets up Ondatra tests init
func TestMain(m *testing.M) {
	os.Setenv("LOGLEVEL", "DEBUG")
	fptest.RunTests(m)
}

// TestCollectDebugFiles collects debug commands if coreFile flag is set to false, else it Skips the test
func TestCollectDebugFiles(t *testing.T) {
	if *coreFilesFlag == true {
		t.SkipNow()
	}
	logger.Logger.Debug().Msg("Function TestCollectionDebugFiles has started")
	// set up Targets
	targets := NewTargets(t)
	if *outDirFlag == "" {
		logger.Logger.Error().Msg(fmt.Sprintf("out directory flag not set correctly: [%s]", *outDirFlag))
		t.FailNow()
	} else {
		outDir = *outDirFlag
		logger.Logger.Info().Msg(fmt.Sprintf("out directory flag is: [%s]", *outDirFlag))
		timestamp = *timestampFlag
	}

	commands := []string{
		"run rm -rf /" + techDirectory,
		"mkdir " + techDirectory,
	}

	for _, t := range showTechSupport {
		commands = append(commands, fmt.Sprintf("show tech-support %s file %s", t, getTechFileName(t)))
	}

	for _, t := range pipedCmds {
		commands = append(commands, fmt.Sprintf("%s | file %s", t, getTechFileName(t)))
	}

	for dutID, targetInfo := range targets.targetInfo {

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
}
