// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package factory_reset_test

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"io"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	frpb "github.com/openconfig/gnoi/factory_reset"
	fpb "github.com/openconfig/gnoi/file"
	"github.com/openconfig/gnoi/types"
	"github.com/openconfig/gnoigo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
)

var (
	remoteFilePath = map[ondatra.Vendor]string{
		ondatra.CISCO:   "/misc/disk1/",
		ondatra.NOKIA:   "/tmp/",
		ondatra.JUNIPER: "/var/tmp/",
		ondatra.ARISTA:  "/mnt/flash/",
	}
)

const maxRebootTime = 40 // 40 mins wait time for the factory reset and sztp to kick in
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func deviceBootStatus(t *testing.T, dut *ondatra.DUTDevice) {
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
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

func gNOIPutFile(t *testing.T, dut *ondatra.DUTDevice, gnoiClient gnoigo.Clients, fName string) {
	dutVendor := dut.Vendor()
	fullPath := filepath.Join(remoteFilePath[dutVendor], fName)
	stream, err := gnoiClient.File().Put(context.Background())
	t.Logf("Attempting to send gNOI File Put here: %v", fullPath)
	if err != nil {
		t.Fatalf("Failed to create stream channel: %v", err)
	}
	defer stream.CloseSend()
	h := md5.New()
	fPutOpen := &fpb.PutRequest_Open{
		Open: &fpb.PutRequest_Details{
			RemoteFile:  fullPath,
			Permissions: 744,
		},
	}
	err = stream.Send(&fpb.PutRequest{
		Request: fPutOpen,
	})
	if err != nil {
		t.Fatalf("Stream failed to send PutRequest: %v", err)
	}

	b := make([]byte, 64*1024)
	n, err := rand.Read(b)
	if err != nil && err != io.EOF {
		t.Fatalf("Error reading bytes: %v", err)
	}
	h.Write(b[:n])
	req := &fpb.PutRequest{
		Request: &fpb.PutRequest_Contents{
			Contents: b[:n],
		},
	}
	err = stream.Send(req)
	if err != nil {
		t.Fatalf("Stream failed to send Req: %v", err)
	}

	hashReq := &fpb.PutRequest{
		Request: &fpb.PutRequest_Hash{
			Hash: &types.HashType{
				Method: types.HashType_MD5,
				Hash:   h.Sum(nil),
			},
		},
	}
	err = stream.Send(hashReq)
	if err != nil {
		t.Fatalf("Stream failed to send hash: %v", err)
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("Problem closing the stream: %v", err)
	}
}

func gNOIStatFile(t *testing.T, dut *ondatra.DUTDevice, fName string, reset bool) {
	dutVendor := dut.Vendor()
	fullPath := filepath.Join(remoteFilePath[dutVendor], fName)
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	if _, ok := remoteFilePath[dutVendor]; !ok {
		t.Fatalf("Please add support for vendor %v in var remoteFilePath ", dutVendor)
	}

	in := &fpb.StatRequest{
		Path: remoteFilePath[dutVendor],
	}
	statResp, err := gnoiClient.File().Stat(context.Background(), in)
	if err != nil {
		t.Fatalf("Error fetching stat path %v for the created file on DUT. %v", remoteFilePath[dutVendor], err)
	}

	if len(statResp.GetStats()) == 0 {
		t.Log("gNOI STAT did not find any files")
	}

	r := regexp.MustCompile(fName)
	var isCreatedFile bool

	for _, fileStats := range statResp.GetStats() {
		isCreatedFile = r.MatchString(fileStats.GetPath()) && (fileStats.GetSize() == uint64(64*1024))
		if isCreatedFile {
			break
		}
	}
	if isCreatedFile {
		if !reset {
			t.Logf("gNOI PUT successfully created file: %s", fullPath)
		} else {
			t.Errorf("gNOI PUT file was found after Factory Reset: %s", fullPath)
		}
	}
	if !isCreatedFile {
		if !reset {
			t.Error("gNOI PUT file was never Created")
		} else {
			t.Logf("Did not find %s in the list of files", fullPath)
		}
	}
}

func TestFactoryReset(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	testCases := []struct {
		name      string
		fileName  string
		fileExist bool
	}{
		{
			name:      "Random file",
			fileName:  "devrandom.log",
			fileExist: false,
		},
		{
			name:      "Startup config",
			fileName:  "startup-config",
			fileExist: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
			if err != nil {
				t.Fatalf("Error dialing gNOI: %v", err)
			}

			if !tc.fileExist {
				gNOIPutFile(t, dut, gnoiClient, tc.fileName)
			}
			gNOIStatFile(t, dut, tc.fileName, tc.fileExist)

			res, err := gnoiClient.FactoryReset().Start(context.Background(), &frpb.StartRequest{FactoryOs: false, ZeroFill: false})
			if err != nil {
				t.Fatalf("Failed to initiate Factory Reset on the device, Error : %v ", err)
			}
			t.Logf("Factory reset Response %v ", res)
			time.Sleep(2 * time.Minute)

			deviceBootStatus(t, dut)
			gNOIStatFile(t, dut, tc.fileName, true)
		})
	}
}
