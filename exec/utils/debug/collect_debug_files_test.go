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
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
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
		"service-layer", "grpc", "spi", "hw-ac", "bundles", "cfgmgr",
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

	commands := []string{
		"run rm -rf /" + techDirectory,
		"mkdir " + techDirectory,
	}

	if *coreCheck {
		commands = append(commands,
			"run find /misc/disk1 -maxdepth 1 -type f -name '*core*' -newermt @"+*timestamp+" -exec cp \"{}\" /"+techDirectory+"/  \\\\;",
		)
	}

	for dutID, targetInfo := range targets.targetInfo {
		fileNamePrefix := ""
		if !*splitPerDut && len(targets.targetInfo) > 1 {
			fileNamePrefix = dutID + "_"
		}

		if *collectTech {
			for _, t := range showTechList {
				commands = append(commands, fmt.Sprintf("show tech-support %s file %s", t, getTechFileName(t, fileNamePrefix)))
			}
		}

		if *runCmds {
			for _, t := range pipedCmdList {
				commands = append(commands, fmt.Sprintf("%s | file %s", t, getTechFileName(t, fileNamePrefix)))
			}
		}

		ctx := context.Background()
		dut := ondatra.DUT(t, dutID)
		sshClient := dut.RawAPIs().CLI(t)

		for _, cmd := range commands {
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

		copyDebugFiles(t, targetInfo)
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

// getTechFileName return the techDirecory + / + replacing " " with _
func getTechFileName(tech string, prefix string) string {
	return filepath.Join(techDirectory, prefix+strings.ReplaceAll(tech, " ", "_"))
}
