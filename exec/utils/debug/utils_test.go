package debug

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/logger"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/testt"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"
)

// copyDebugFiles copies files from the runs to an specified directory with a filename
//
// d targetInfo - contains the dut info
// filename - self-explanatory
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

// getSSHInfo adds dut ssh info to a slice targetInfo[dut.Id]
//
// return an error if any
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

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		logger.Logger.Error().Msg(fmt.Sprintf("Error unmarshalling binding file: [%v]", err))
		return fmt.Errorf(fmt.Sprintf("Error unmarshalling binding file: [%v]", err))
	}

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
		} else {
			logger.Logger.Error().Msg("One or more values are empty  dut.Id, sshIP, sshPort, sshUser, sshPass ")
			return fmt.Errorf("One or more values are empty  dut.Id, sshIP, sshPort, sshUser, sshPass ")
		}
	}
	return nil
}

// getTechFileName return the techDirecory + / + replacing " " with _
func getTechFileName(tech string) string {
	return techDirectory + "/" + strings.ReplaceAll(tech, " ", "_")
}

// setCoreFile creates a core file. function to be deleted !!!!!!!!!!
func (ti *Targets) SetCoreFile(t *testing.T) {
	fmt.Println("Starting setCoreFile")
	t.Helper()

	cmd := "dumpcore running 1215"

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
