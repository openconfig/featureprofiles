package software_upgrade_test

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/gnoi/file"
	"github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	imageDestination = "/harddisk:/8000-x64.iso"
	installCmd       = "install replace reimage " + imageDestination + " noprompt commit"
	installStatusCmd = "sh install request"
	imgCopyTimeout   = 1800 * time.Second
	installTimeout   = 1800 * time.Second
	sshCmdTimeout    = 30 * time.Second
	statusCheckDelay = 60 * time.Second
)

var (
	imagePathFlag = flag.String("imagePath", "", "Full path to image iso")
	lineupFlag    = flag.String("lineup", "", "lineup")
	efrFlag       = flag.String("efr", "", "efr")
	forceFlag     = flag.Bool("force", false, "Force install even if image already installed")
	gnoiFlag      = flag.Bool("gnoi", false, "Use gNOI to copy image instead of SCP")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type targetInfo struct {
	dut     string
	sshIp   string
	sshPort string
	sshUser string
	sshPass string
}

func TestSoftwareUpgrade(t *testing.T) {
	if *imagePathFlag == "" {
		t.Fatal("Missing imagePath arg")
	}

	efr := *efrFlag
	lineup := *lineupFlag
	force := *forceFlag
	gnoi := *gnoiFlag
	imagePath := *imagePathFlag
	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("Image {%s} does not exist: %v", imagePath, err)
	}

	for _, d := range parseBindingFile(t) {
		dut := ondatra.DUT(t, d.dut)
		if !force && len(lineup) > 0 && len(efr) > 0 {
			if !shouldInstall(t, dut, lineup, efr) {
				t.Logf("Image already installed on %s, skipping...", dut.ID())
				continue
			}
		}

		if !gnoi {
			time.Sleep(5 * time.Second)
			copyImageSCP(t, &d, imagePath)
		} else {
			copyImageGNOI(t, dut, imagePath)
		}

		if result, err := sendCLI(t, dut, installCmd); err == nil {
			if !strings.Contains(result, "has started") {
				t.Fatalf("Unexpected response:\n%s\n", result)
			}
		} else {
			t.Fatalf("Error running command: %v", err)
		}

		success := false
		for start := time.Now(); time.Since(start) < installTimeout && !success; {
			time.Sleep(statusCheckDelay)

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				if result, err := sendCLI(t, dut, installStatusCmd); err == nil {
					if strings.Contains(result, "In progress") {
						t.Logf("Install operation in progress...")
					} else if strings.Contains(result, "Failure") {
						t.Fatalf("Image upgrade failed:\n%s\n", result)
					} else if strings.Contains(result, "Success") {
						success = true
					} else {
						t.Fatalf("Unexpected response:\n%s\n", result)
					}
				} else if err == context.DeadlineExceeded || err == io.EOF {
					t.Logf("Device is probably rebooting...")
				} else {
					t.Logf("Error running command: %v", err)
				}
			}); errMsg != nil {
				t.Logf("Device is probably rebooting...")
			}
		}

		if !success {
			t.Fatalf("Install operation timed out")
		}

		if len(lineup) > 0 && len(efr) > 0 {
			if !verifyInstall(t, dut, lineup, efr) {
				t.Fatalf("Found unexpected image after install on %v", dut.ID())
			}
		}
	}
}

func copyImageSCP(t testing.TB, d *targetInfo, imagePath string) {
	target := fmt.Sprintf("%s:%s", d.sshIp, d.sshPort)
	t.Logf("Copying image to %s (%s) over scp", d.dut, target)
	sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
	scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
	if err != nil {
		t.Fatalf("Error initializing scp client: %v", err)
	}
	defer scpClient.Close()

	if err := scpClient.CopyFileToRemote(imagePath, imageDestination, &scp.FileTransferOption{
		Timeout: imgCopyTimeout,
	}); err != nil {
		t.Fatalf("Error copying image to target %s (%s:%s): %v", d.dut, d.sshIp, d.sshPort, err)
	}
}

func copyImageGNOI(t testing.TB, dut *ondatra.DUTDevice, imagePath string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), imgCopyTimeout)
	defer cancel()
	fileClient, err := dut.RawAPIs().GNOI().New(t).File().Put(ctx)
	if err != nil {
		t.Fatalf("Could not create gNOI file client: %v", err)
	}

	if err = fileClient.Send(&file.PutRequest{
		Request: &file.PutRequest_Open{
			Open: &file.PutRequest_Details{
				RemoteFile:  imageDestination,
				Permissions: uint32(600),
			},
		},
	}); err != nil {
		t.Fatalf("Could not initiate gNOI file put request: %v", err)
	}

	imgData, imgHash := mustGetImageData(t, imagePath)
	if err = fileClient.Send(&file.PutRequest{
		Request: &file.PutRequest_Contents{
			Contents: imgData,
		},
	}); err != nil {
		t.Fatalf("Error sending image content: %v", err)
	}

	if err = fileClient.Send(&file.PutRequest{
		Request: &file.PutRequest_Hash{
			Hash: &types.HashType{
				Method: types.HashType_SHA256,
				Hash:   imgHash,
			},
		},
	}); err != nil {
		t.Fatalf("Error sending image hash: %v", err)
	}
}

func mustGetImageData(t testing.TB, imagePath string) ([]byte, []byte) {
	t.Helper()
	file, err := os.Open(imagePath)
	if err != nil {
		t.Fatalf("Could not open image: %v", err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Could not read image data: %v", err)
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		log.Fatal(err)
	}
	return data, hash.Sum(nil)
}

func sendCLI(t testing.TB, dut *ondatra.DUTDevice, cmd string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), sshCmdTimeout)
	defer cancel()
	sshClient := dut.RawAPIs().CLI(t)
	defer sshClient.Close()
	return sshClient.SendCommand(ctx, cmd)
}

func shouldInstall(t testing.TB, dut *ondatra.DUTDevice, lineup string, efr string) bool {
	if buildInfo, err := sendCLI(t, dut, "run cat /etc/build-info.txt"); err == nil {
		t.Logf("Installed image info:\n%s", buildInfo)
		return !(strings.Contains(buildInfo, lineup) && strings.Contains(buildInfo, efr))
	} else {
		t.Logf("Could not get existing image build info: %v\n. Ignoring...", err)
	}
	return true
}

func verifyInstall(t testing.TB, dut *ondatra.DUTDevice, lineup string, efr string) bool {
	return !shouldInstall(t, dut, lineup, efr)
}

func parseBindingFile(t *testing.T) []targetInfo {
	t.Helper()

	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}

	targets := []targetInfo{}
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
		sshIp := sshTarget[0]
		sshPort := "22"
		if len(sshTarget) > 1 {
			sshPort = sshTarget[1]
		}

		targets = append(targets, targetInfo{
			dut:     dut.Id,
			sshIp:   sshIp,
			sshPort: sshPort,
			sshUser: sshUser,
			sshPass: sshPass,
		})
	}

	return targets
}
