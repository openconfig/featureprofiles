package sztp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	frpb "github.com/openconfig/gnoi/factory_reset"
	"github.com/openconfig/ondatra"
)

var (
	sshIP             = flag.String("ssh_ip", "", "External IP address of management interface.")
	sshPort           = flag.String("ssh_port", "", "External Port of management interface")
	rm_knownhosts     = fmt.Sprintf("%s/.ssh/known_hosts", os.Getenv("HOME"))
	devicePaths       = []string{"/misc/disk1/"}
	filesCreated      = []string{}
	fileCreateDevRand = "bash  dd if=/dev/urandom of=%s bs=1M count=2"
	checkFileExists   = "bash [ -f \"%s\" ] && echo \"YES_exists\""
	fileExists        = "YES_exists"
	fileCreate        = "bash fallocate -l %dM %s"
)

func retryFunction(t *testing.T, attempts int, sleep time.Duration, f func() error) (err error) {
	for attempt := 1; attempt < attempts; attempt++ {
		t.Logf("Trying to connect to box \nAttempt : %v\n", attempt)
		if attempt > 1 {
			time.Sleep(sleep * time.Minute)
		}
		err = f()
		if err == nil {
			return nil
		} else {
			t.Logf("Exception: %v\n ", err)
		}
	}
	return err
}

// timedFunction adds timeout for a function
func timedFunction(timeout time.Duration, f func() error) error {
	result := make(chan error, 1)
	go func() {
		result <- f()
	}()
	select {
	case <-time.After(timeout * time.Second):
		return errors.New("timed out\n")
	case result := <-result:
		return result
	}
}

// checkGrpcHandle checks if GNMI handle is up
func checkGrpcHandle(t *testing.T, dut *ondatra.DUTDevice, retryCount int, retryWait int, timeout int) {
	t.Log("Check if CLI Handle is Accessible")
	err := retryFunction(t, retryCount, time.Duration(retryWait), func() error {
		return timedFunction(time.Duration(timeout), func() error {
			resp, err := dut.RawAPIs().GNMI().New(t).Get(context.Background(), &gpb.GetRequest{Path: []*gpb.Path{{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}}}, Type: gpb.GetRequest_CONFIG, Encoding: gpb.Encoding_JSON_IETF})
			t.Logf("Hostname \n  %v\n", resp)
			return err
		})
	})
	if err != nil {
		t.Error(err)
	}
}

// creating files before factory reset
func createFiles(t *testing.T, dut *ondatra.DUTDevice) {

	cli := dut.RawAPIs().CLI(t)
	for _, folderPath := range devicePaths {
		fPath := path.Join(folderPath, "devrandom.log")
		_, err := cli.SendCommand(context.Background(), fmt.Sprintf(fileCreateDevRand, fPath))
		if err != nil {
			t.Error(err)
		}
		time.Sleep(2 * time.Second)
		filesCreated = append(filesCreated, fPath)
		fPath = path.Join(folderPath, ".devrandom.log")
		_, err = cli.SendCommand(context.Background(), fmt.Sprintf(fileCreateDevRand, fPath))
		if err != nil {
			t.Error(err)

		}

		filesCreated = append(filesCreated, fPath)
		fPath = path.Join(folderPath, "largeFile.log")
		_, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), fmt.Sprintf(fileCreate, 100, fPath))
		if err != nil {
			t.Error(err)
		}

		filesCreated = append(filesCreated, fPath)
	}
	for _, fP := range filesCreated {
		resp, err := cli.SendCommand(context.Background(), fmt.Sprintf(checkFileExists, fP))
		if err != nil {
			t.Error(err)
		}
		if !strings.Contains(resp, fileExists) {
			t.Errorf("Unable to Create a file object %s in device %s", fP, dut.Name())
		}
	}

}

// checkFiles check if the files created are deleted from the device after factory reset
func checkFiles(t *testing.T, dut *ondatra.DUTDevice) {
	for _, fP := range filesCreated {

		resp, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), fmt.Sprintf(checkFileExists, fP))
		t.Logf(resp)
		if err != nil {
			t.Error(err)
		}
		if strings.Contains(resp, fileExists) == true {
			t.Errorf("File %s not cleared by system Reset, in device %s", fP, dut.Name())
		}

	}
}

// performs factory reset
func factory_reset(t *testing.T, dut *ondatra.DUTDevice) {
	createFiles(t, dut)
	gnoiClient := dut.RawAPIs().GNOI().New(t)
	facRe, err := gnoiClient.FactoryReset().Start(context.Background(), &frpb.StartRequest{FactoryOs: false, ZeroFill: false})
	t.Logf("Factory reset Response %v ", facRe)
	if err != nil {
		t.Error(err)
	}
	t.Log("Sleep for 8 mins after factory reset and try connecting to box\n")
	time.Sleep(8 * time.Minute)
	checkGrpcHandle(t, dut, 30, 3, 10)
	cmd := exec.Command("bash", "-c", fmt.Sprintf("rm -rf %s", rm_knownhosts)).Run()
	if cmd != nil {
		t.Error(cmd)
	}
	cmd1 := exec.Command("bash", "-c", fmt.Sprintf("sshpass -p cisco123 ssh -o StrictHostKeyChecking=no cafyauto@%s -p %s", *sshIP, *sshPort)).Run()
	if cmd1 != nil {
		t.Error(cmd1)
	}
	checkFiles(t, dut)
}

func TestFactoryReset(t *testing.T) {
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}

	dut := ondatra.DUT(t, "dut")
	cli_handle := dut.RawAPIs().CLI(t)
	showDiskEncryptionStatus, err := cli_handle.SendCommand(context.Background(), "show disk-encryption status")
	if err != nil {
		t.Error(err)
	}
	t.Logf("%v", (showDiskEncryptionStatus))
	if strings.Contains(showDiskEncryptionStatus, "Not Encrypted") {
		t.Log("Performing Factory reset without Encryption\n")
		factory_reset(t, dut)
		t.Log("Stablise after factory reset\n")
		time.Sleep(5 * time.Minute)
		t.Log("Activate Encryption\n")
		encrypt, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), "disk-encryption activate")
		t.Logf("Sleep for 5 mins after disk-encryption activate")
		time.Sleep(5 * time.Minute)
		t.Logf("%v", encrypt)
		if err != nil {
			t.Error(err)

		}
		checkGrpcHandle(t, dut, 30, 3, 10)
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), "show disk-encryption status")
		t.Logf("%v", encrypt)
		if err != nil {
			t.Error(err)

		}
		t.Log("Wait for the system to stabalise\n")
		time.Sleep(5 * time.Minute)
		factory_reset(t, dut)
	} else {
		t.Log("Performing Factory reset with Encryption\n")
		factory_reset(t, dut)
		t.Log("Stablise after factory reset\n")
		time.Sleep(5 * time.Minute)
		t.Log("Deactivate Encryption\n")
		encrypt, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), "disk-encryption deactivate")
		t.Logf("Sleep for 5 mins after disk-encryption deactivate")
		time.Sleep(5 * time.Minute)
		t.Logf("%v", encrypt)
		if err != nil {
			t.Error(err)

		}
		checkGrpcHandle(t, dut, 30, 3, 10)
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), "show disk-encryption status")
		t.Logf("%v", encrypt)
		if err != nil {
			t.Error(err)

		}
		t.Logf("Wait for the system to stabalise\n")
		time.Sleep(5 * time.Minute)
		factory_reset(t, dut)
	}

}
