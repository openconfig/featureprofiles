package sztp_base_test

import (
	"context"
	"flag"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
)

var (
	sshIP   = flag.String("ssh_ip", "173.39.51.67", "External IP address of management interface.")
	sshPort = flag.String("ssh_port", "5000", "External Port of management interface")
	sshUser = flag.String("ssh_user", "cafyauto", "External username for ssh")
	sshPass = flag.String("ssh_pass", "cisco123", "External password for ssh")
)

const maxRebootTime = 40 // 40 mins wait time for the factory reset and sztp to kick in
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// creating files before factory reset
func createFiles(t *testing.T, dut *ondatra.DUTDevice, devicePaths []string) {
	cli := dut.RawAPIs().CLI(t)
	for _, folderPath := range devicePaths {
		fPath := path.Join(folderPath, "devrandom.log")
		_, err := cli.SendCommand(context.Background(), fmt.Sprintf(fileCreateDevRand, fPath))
		if err != nil {
			t.Fatal(err)
		}
		t.Log("Check if the file is created")
		time.Sleep(30 * time.Second)
		filesCreated = append(filesCreated, fPath)
		fPath = path.Join(folderPath, ".devrandom.log")
		_, err = cli.SendCommand(context.Background(), fmt.Sprintf(fileCreateDevRand, fPath))
		if err != nil {
			t.Fatal(err)

		}

		filesCreated = append(filesCreated, fPath)
		fPath = path.Join(folderPath, "largeFile.log")
		_, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), fmt.Sprintf(fileCreate, 100, fPath))
		if err != nil {
			t.Fatal(err)
		}

		filesCreated = append(filesCreated, fPath)
	}
	for _, fP := range filesCreated {
		resp, err := cli.SendCommand(context.Background(), fmt.Sprintf(checkFileExists, fP))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%v", resp)
		if !strings.Contains(resp, fileExists) {
			t.Fatalf("Unable to Create a file object %s in device %s", fP, dut.Name())
		}
	}

}

// checkFiles check if the files created are deleted from the device after factory reset
func checkFiles(t *testing.T, dut *ondatra.DUTDevice) {
	for _, fP := range filesCreated {

		resp, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), fmt.Sprintf(checkFileExists, fP))
		t.Logf(resp)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(resp, fileExists) == true {
			t.Fatalf("File %s not cleared by system Reset, in device %s", fP, dut.Name())
		}

	}
}

func deviceBootStatus(t *testing.T, dut *ondatra.DUTDevice) {
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = dut.Telemetry().System().CurrentDatetime().Get(t)
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
}
