// WARNING: this collection of utils contains functions that may cause instability in testbeds. Please review docstring of each fuction before usage.
package stress

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"
)

type targetInfo struct {
	dut     string
	sshIp   string
	sshPort string
	sshUser string
	sshPass string
}

const (
	binPath        = "/auto/ng_ott_auto/tools/stress/stress"
	binDestination = "/disk0:/stress"
	binCopyTimeout = 1800 * time.Second
)

var binaryInstalled bool

func init() {
	binaryInstalled = false
}

func installBinary(t testing.TB) {
	t.Helper()
	for _, d := range parseBindingFile(t) {
		dut := ondatra.DUT(t, d.dut)
		copyFileSCP(t, &d, binPath)
		t.Logf("Installing file to %s", dut.ID())

		cli := dut.RawAPIs().CLI(t)

		cli.RunCommand(context.Background(), "run chmod +x /var/xr/scratch/stress")

		result, _ := cli.RunCommand(context.Background(), "run ls -la /var/xr/scratch/stress")
		t.Logf("output: %s", result.Output())

		if !strings.Contains(result.Output(), "stress") {
			t.Error("error verifying file copy: not found")
		}
	}
}

func copyFileSCP(t testing.TB, d *targetInfo, imagePath string) {
	t.Helper()
	target := fmt.Sprintf("%s:%s", d.sshIp, d.sshPort)
	t.Logf("Copying file to %s (%s) over scp", d.dut, target)
	sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
	scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
	if err != nil {
		t.Fatalf("Error initializing scp client: %v", err)
	}
	defer scpClient.Close()

	ticker := time.NewTicker(1 * time.Minute)
	tickerQuit := make(chan bool)

	go func() {
		for {
			select {
			case <-ticker.C:
				t.Logf("Copying file...")
			case <-tickerQuit:
				return
			}
		}
	}()

	defer func() {
		ticker.Stop()
		tickerQuit <- true
	}()

	if err := scpClient.CopyFileToRemote(imagePath, binDestination, &scp.FileTransferOption{
		Timeout: binCopyTimeout,
	}); err != nil {
		t.Fatalf("Error copying image to target %s (%s:%s): %v", d.dut, d.sshIp, d.sshPort, err)
	}
}

func parseBindingFile(t testing.TB) []targetInfo {
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

// spawns n workers to loop on sqrt()
// NOTE: May slow down system operation
func StressCPU(t testing.TB, dut *ondatra.DUTDevice, numWorkers int, duration time.Duration) {
	t.Helper()
	if !binaryInstalled {
		installBinary(t)
	}
	if duration < time.Second {
		t.Fatalf("stress time must be at least 1 second")
	}
	cmd := fmt.Sprintf("run /var/xr/scratch/stress --cpu %d --timeout %ds", numWorkers, duration.Round(time.Second)/1000000000)
	dut.CLI().RunResult(t, cmd)
}

// spawns n workers to loop on malloc()/free()
// NOTE: May slow down system operation
func StressMem(t testing.TB, dut *ondatra.DUTDevice, numWorkers int, duration time.Duration) {
	t.Helper()
	if !binaryInstalled {
		installBinary(t)
	}
	if duration < time.Second {
		t.Fatalf("stress time must be at least 1 second")
	}
	cmd := fmt.Sprintf("run /var/xr/scratch/stress --vm %d --timeout %ds", numWorkers, duration.Round(time.Second)/1000000000)
	dut.CLI().RunResult(t, cmd)
}

// allocates dummy file to disk0 for stress testing purposes
// WARNING: Recommended use on sim only, as it can fill hard drive and bring system to a halt.
func StressDisk0(t testing.TB, dut *ondatra.DUTDevice, gigabytes int, duration time.Duration) {
	t.Helper()
	if duration < time.Second {
		t.Fatalf("stress time must be at least 1 second")
	}
	cmd := fmt.Sprintf("run fallocate -l %dG big_file.iso; sleep %ds; rm big_file.iso", gigabytes, duration.Round(time.Second)/1000000000)
	dut.CLI().RunResult(t, cmd)
}

// allocates dummy file to harddisk for stress testing purposes
// WARNING: Recommended use on sim only, as it can fill hard drive and bring system to a halt.
func StressHardDisk(t testing.TB, dut *ondatra.DUTDevice, gigabytes int, duration time.Duration) {
	t.Helper()
	// allocate very large file
	if duration < time.Second {
		t.Fatalf("stress time must be at least 1 second")
	}
	cmd := fmt.Sprintf("run cd /harddisk:; fallocate -l %dG big_file.iso; sleep %ds; rm big_file.iso", gigabytes, duration.Round(time.Second)/1000000000)
	dut.CLI().RunResult(t, cmd)
}

// simulates sensor reading of power usage of supplied measure of watts
// NOTE: requires a dev image which includes the spi_envmon_test library
func StressPower(t testing.TB, dut *ondatra.DUTDevice, watts int, duration time.Duration) {
	t.Helper()
	if duration < time.Second {
		t.Fatalf("stress time must be at least 1 second")
	}
	cmd := fmt.Sprintf("./spi_envmon_test -x %dW %d", watts, duration.Round(time.Second)/1000000000)
	dut.CLI().RunResult(t, cmd)
}
