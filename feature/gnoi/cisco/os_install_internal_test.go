// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package osinstall_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"flag"

	// dbg "github.com/openconfig/featureprofiles/exec/utils/debug"
	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	closer "github.com/openconfig/gocloser"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ospb "github.com/openconfig/gnoi/os"
	spb "github.com/openconfig/gnoi/system"
)

var packageReader func(context.Context) (io.ReadCloser, error) = func(ctx context.Context) (io.ReadCloser, error) {
	f, err := os.Open(*osFile)
	if err != nil {
		return nil, err
	}
	return f, nil
}

var (
	// osFile                          = flag.String("osFile", "", "Path to the OS image under test for the install operation")
	// osFileForceDownloadSupported    = flag.String("osFileForceDownloadSupported", "", "Path to the OS image (Force Download Supported) for the install operation")
	// osFileForceDownloadNotSupported = flag.String("osFileForceDownloadNotSupported", "", "Path to the OS image ((Force Download not Supported)) for the install operation")
	osFile                          = flag.String("osFile", "/auto/b4ws/xr/builds/nightly/latest/img-8000/8000-x64.iso", "Path to the OS image under test for the install operation")
	osFileForceDownloadSupported    = flag.String("osFileForceDownloadSupported", "/auto/prod_weekly_archive1/bin/25.1.1.21I.DT_IMAGE/8000/8000-x64-25.1.1.21I.iso", "Path to the OS image (Force Download Supported) for the install operation")
	osFileForceDownloadNotSupported = flag.String("osFileForceDownloadNotSupported", "/auto/prod_weekly_archive2/bin/24.4.1.39I.SIT_IMAGE/8000/8000-x64-24.4.1.39I.iso", "Path to the OS image ((Force Download not Supported)) for the install operation")
	osVersion                       = flag.String("osVersion", "", "Path to new OS Version, will be auto filled by new logic")
	timeout                         = flag.Duration("timeout", time.Minute*30, "Time to wait for reboot to complete")
	osFileOriginal                  = ""
	dutSrc                          = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutAttrs = attrs.Attributes{
		Desc:    "To ATE",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	bgpGlobalAttrs = bgpAttrs{
		rplName:       "ALLOW",
		grRestartTime: 60,
		prefixLimit:   200,
		dutAS:         64500,
		ateAS:         64501,
		peerGrpNamev4: "BGP-PEER-GROUP-V4",
	}
	bgpNbr1 = bgpNeighbor{localAs: bgpGlobalAttrs.dutAS, peerAs: bgpGlobalAttrs.ateAS, pfxLimit: bgpGlobalAttrs.prefixLimit, neighborip: ateSrc.IPv4, isV4: true}
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	DiskFullError = "Too large, no disk space"
)
const (
	maxSwitchoverTime = 900
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

type bgpAttrs struct {
	rplName, peerGrpNamev4    string
	prefixLimit, dutAS, ateAS uint32
	grRestartTime             uint16
}
type bgpNeighbor struct {
	localAs, peerAs, pfxLimit uint32
	neighborip                string
	isV4                      bool
}
type testCase struct {
	dut *ondatra.DUTDevice
	// dualSup indicates if the DUT has a standby supervisor available.
	dualSup bool
	reader  io.ReadCloser

	osc ospb.OSClient
	sc  spb.SystemClient
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOSTransferDiskFull(t *testing.T) {
	// dbg.CollectDebugFiles(t, "", "0", true, true, true, true, "", "")
	osFileOriginal = *osFile
	*osFile = *osFileForceDownloadSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	// noReboot := deviations.OSActivateNoReboot(dut)
	tc.fetchStandbySupervisorStatus(ctx, t)

	// New test case: Simulate disk full scenario
	t.Run("disk full scenario", func(t *testing.T) {
		sshClient := dut.RawAPIs().CLI(t)

		// Retrieve and fill disk space
		skipFill, err := retrieveAndFillDiskSpace(ctx, sshClient, 1.1)
		if err != nil {
			t.Fatalf("Error during disk space retrieval and fill: %v", err)
		}
		if skipFill {
			t.Log("Available space is less than 1.1GB, skipping disk fill step.")
			return
		}
		defer func() {
			if err := cleanupDiskSpace(ctx, sshClient); err != nil {
				t.Fatalf("Cleanup failed: %v", err)
			}
		}()

		// Step 3: Attempt to transfer the OS and verify failure
		tc.transferOS(ctx, t, false, "", DiskFullError)

	})
}
func TestOSForceTransferStandby(t *testing.T) {
	*osFile = *osFileForceDownloadSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	// noReboot := deviations.OSActivateNoReboot(dut)
	tc.fetchStandbySupervisorStatus(ctx, t)

	// Attempt to transfer the OS with standby flag and verify failure
	tc.transferOS(ctx, t, true, "", "stand")
}

func TestOSNormalTransferStandby(t *testing.T) {
	*osFile = *osFileForceDownloadSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	// noReboot := deviations.OSActivateNoReboot(dut)
	tc.fetchStandbySupervisorStatus(ctx, t)

	// Attempt to transfer the OS with standby flag and verify failure
	tc.transferOS(ctx, t, true, "", "stand")
}

func TestOSForceTransfer1(t *testing.T) {
	*osFile = *osFileForceDownloadSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	tc.fetchStandbySupervisorStatus(ctx, t)
	// Force Install
	t.Run("Force install No ExistingImage using Empty version", func(t *testing.T) {
		t1, _ := listISOFile(t, dut, *osVersion)
		tc.transferOS(ctx, t, false, "", "")
		t2, _ := listISOFile(t, dut, *osVersion)
		removeISOFile(t, dut, *osVersion)
		if t1 != 0 && t2 == 0 {
			t.Fatal("image not force updated")
		}
	})
}

func TestOSForceTransfer2(t *testing.T) {
	*osFile = *osFileForceDownloadSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	tc.fetchStandbySupervisorStatus(ctx, t)
	// Force Install
	t.Run("Force install No ExistingImage using Wrong version", func(t *testing.T) {
		t1, _ := listISOFile(t, dut, *osVersion)
		tc.transferOS(ctx, t, false, "WRONG_VERSION", "")
		t2, _ := listISOFile(t, dut, *osVersion)
		removeISOFile(t, dut, *osVersion)
		if t1 != 0 && t2 == 0 {
			t.Fatal("image not force updated")
		}
	})
}

func TestOSNormalTransfer1(t *testing.T) {
	*osFile = *osFileForceDownloadSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	tc.fetchStandbySupervisorStatus(ctx, t)

	t.Run("Normal install NO ExistingImage", func(t *testing.T) {
		t1, _ := listISOFile(t, dut, *osVersion)
		tc.transferOS(ctx, t, false, *osVersion, "")
		t2, _ := listISOFile(t, dut, *osVersion)
		if t1 != 0 && t2 == 0 {
			t.Fatal("image not force updated")
		}
	})
}

func TestOSNormalTransfer2(t *testing.T) {
	*osFile = *osFileForceDownloadSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	tc.fetchStandbySupervisorStatus(ctx, t)
	// Force Install
	t.Run("Normal install ExistingImage", func(t *testing.T) {
		t1, _ := listISOFile(t, dut, *osVersion)
		tc.transferOS(ctx, t, false, *osVersion, "")
		t2, _ := listISOFile(t, dut, *osVersion)
		if t1 == 0 && t1 != t2 {
			t.Fatal("image changed")
		}
	})
}

func TestOSForceTransfer3(t *testing.T) {
	*osFile = *osFileForceDownloadSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	tc.fetchStandbySupervisorStatus(ctx, t)
	// Force Install
	t.Run("Force install ExistingImage using Empty version", func(t *testing.T) {
		t1, _ := listISOFile(t, dut, *osVersion)
		tc.transferOS(ctx, t, false, "", "")
		t2, _ := listISOFile(t, dut, *osVersion)
		if t1 == 0 && isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})
}

func TestOSForceInstall1(t *testing.T) {
	*osFile = *osFileForceDownloadSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	noReboot := deviations.OSActivateNoReboot(dut)
	tc.fetchStandbySupervisorStatus(ctx, t)
	t.Run("Force install ExistingImage using Wrong version", func(t *testing.T) {
		t1, _ := listISOFile(t, dut, *osVersion)
		// tc.transferOS(ctx, t, false, *osVersion, "")
		tc.transferOS(ctx, t, false, "WRONG_VERSION", "")
		t2, _ := listISOFile(t, dut, *osVersion)
		if t1 == 0 && isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})
	t.Run("Try Activating using Wrong version expected failure", func(t *testing.T) {
		tc.activateOS(ctx, t, false, noReboot, "WRONG_VERSION", true)

		if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
			tc.transferOS(ctx, t, true, "", "")
			tc.activateOS(ctx, t, true, noReboot, "WRONG_VERSION", true)
		}
	})

	t.Run("test Supervisor Switchover", func(t *testing.T) {
		testSupervisorSwitchover(t)
	})

	t.Run("Activating using correct version expected failure no image", func(t *testing.T) {
		tc.activateOS(ctx, t, false, noReboot, *osVersion, true)

		if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
			tc.transferOS(ctx, t, true, "", "")
			tc.activateOS(ctx, t, true, noReboot, *osVersion, true)
		}
	})

	t.Run("Force install Image using empty version", func(t *testing.T) {
		t1, _ := listISOFile(t, dut, *osVersion)
		// tc.transferOS(ctx, t, false, *osVersion, "")
		tc.transferOS(ctx, t, false, "", "")
		t2, _ := listISOFile(t, dut, *osVersion)
		if isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})

	t.Run("Activating using correct version - expected success", func(t *testing.T) {
		tc.activateOS(ctx, t, false, noReboot, *osVersion, false)

		if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
			tc.transferOS(ctx, t, true, "", "")
			tc.activateOS(ctx, t, true, noReboot, *osVersion, false)
		}
	})

	t.Run("Activating using correct version - expected failure already activated", func(t *testing.T) {
		tc.activateOS(ctx, t, false, noReboot, *osVersion, false)

		if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
			tc.transferOS(ctx, t, true, "", "")
			tc.activateOS(ctx, t, true, noReboot, *osVersion, true)
		}
	})

	if noReboot {
		tc.rebootDUT(ctx, t)
	}

	tc.verifyInstall(ctx, t)
}

func testSupervisorSwitchover(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}

	intfsOperStatusUPBeforeSwitch := helpers.FetchOperStatusUPIntfs(t, dut, *args.CheckInterfacesInBinding)
	t.Logf("intfsOperStatusUP interfaces before switchover: %v", intfsOperStatusUPBeforeSwitch)
	if got, want := len(intfsOperStatusUPBeforeSwitch), 0; got == want {
		t.Errorf("Get the number of intfsOperStatusUP interfaces for %q: got %v, want > %v", dut.Name(), got, want)
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := rpStandbyBeforeSwitch
	got := ""
	if deviations.GNOISubcomponentPath(dut) {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}
	if got, want := switchoverResponse.GetVersion(), ""; got == want {
		t.Errorf("switchoverResponse.GetVersion(): got %v, want non-empty version", got)
	}
	if got := switchoverResponse.GetUptime(); got == 0 {
		t.Errorf("switchoverResponse.GetUptime(): got %v, want > 0", got)
	}

	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(maxSwitchoverTime); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, dut, controllerCards)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	helpers.ValidateOperStatusUPIntfs(t, dut, intfsOperStatusUPBeforeSwitch, 5*time.Minute)

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)

	swTime, swTimePresent := gnmi.Watch(t, dut, activeRP.LastSwitchoverTime().State(), 1*time.Minute, func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }).Await(t)
	if !swTimePresent {
		t.Errorf("activeRP.LastSwitchoverTime().Watch(t).IsPresent(): got %v, want %v", false, true)
	} else {
		st, _ := swTime.Val()
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", st)
		// TODO: validate that last switchover time is correct
	}

	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
}

func TestOSForceInstall2(t *testing.T) {
	*osFile = *osFileForceDownloadNotSupported
	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	noReboot := deviations.OSActivateNoReboot(dut)
	tc.fetchStandbySupervisorStatus(ctx, t)
	t.Run("Force install old image using Wrong version", func(t *testing.T) {
		t1, _ := listISOFile(t, dut, *osVersion)
		// tc.transferOS(ctx, t, false, *osVersion, "")
		tc.transferOS(ctx, t, false, "WRONG_VERSION", "")
		t2, _ := listISOFile(t, dut, *osVersion)
		if t1 == 0 && isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})
	t.Run("Try Activating using Wrong version expected failure", func(t *testing.T) {
		tc.activateOS(ctx, t, false, noReboot, "WRONG_VERSION", true)

		if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
			tc.transferOS(ctx, t, true, "", "")
			tc.activateOS(ctx, t, true, noReboot, "WRONG_VERSION", true)
		}
	})

	t.Run("Activating using correct version - expected success", func(t *testing.T) {
		tc.activateOS(ctx, t, false, noReboot, *osVersion, false)

		if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
			tc.transferOS(ctx, t, true, "", "")
			tc.activateOS(ctx, t, true, noReboot, *osVersion, false)
		}
	})

	t.Run("Activating using correct version - expected failure already activated", func(t *testing.T) {
		tc.activateOS(ctx, t, false, noReboot, *osVersion, false)

		if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
			tc.transferOS(ctx, t, true, "", "")
			tc.activateOS(ctx, t, true, noReboot, *osVersion, true)
		}
	})

	if noReboot {
		tc.rebootDUT(ctx, t)
	}

	tc.verifyInstall(ctx, t)
}

func TestOSForceInstall3(t *testing.T) {
	*osFile = osFileOriginal

	if *osFile == "" {
		t.Fatal("Missing osfile or osver args")
	} else {
		os_version, err := getIsoVersionInfo(*osFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			t.Fatalf("Error: %v\n", err)
		}
		*osVersion = os_version
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	noReboot := deviations.OSActivateNoReboot(dut)
	tc.fetchStandbySupervisorStatus(ctx, t)
	t.Run("Normal install back to original version", func(t *testing.T) {
		t1, _ := listISOFile(t, dut, *osVersion)
		tc.transferOS(ctx, t, false, *osVersion, "")
		// tc.transferOS(ctx, t, false, "WRONG_VERSION", "")
		t2, _ := listISOFile(t, dut, *osVersion)
		if t1 == 0 && isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})

	t.Run("Activating using correct version - expected success", func(t *testing.T) {
		tc.activateOS(ctx, t, false, noReboot, *osVersion, false)

		if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
			tc.transferOS(ctx, t, true, "", "")
			tc.activateOS(ctx, t, true, noReboot, *osVersion, false)
		}
	})

	t.Run("Activating using correct version - expected failure already activated", func(t *testing.T) {
		tc.activateOS(ctx, t, false, noReboot, *osVersion, false)

		if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
			tc.transferOS(ctx, t, true, "", "")
			tc.activateOS(ctx, t, true, noReboot, *osVersion, true)
		}
	})

	if noReboot {
		tc.rebootDUT(ctx, t)
	}

	tc.verifyInstall(ctx, t)
}

func (tc *testCase) activateOS(ctx context.Context, t *testing.T, standby, noReboot bool, version string, expectFail bool) {
	t.Helper()
	if standby {
		t.Log("OS.Activate is started for standby RP.")
	} else {
		t.Log("OS.Activate is started for active RP.")
	}

	act, err := tc.osc.Activate(ctx, &ospb.ActivateRequest{
		StandbySupervisor: standby,
		// Version:           *osVersion,
		Version:  version,
		NoReboot: noReboot,
	})
	if err != nil {
		t.Fatalf("OS.Activate request failed: %s", err)
	}

	switch resp := act.Response.(type) {
	case *ospb.ActivateResponse_ActivateOk:
		if standby {
			t.Log("OS.Activate standby supervisor complete.")
		} else {
			t.Log("OS.Activate complete.")
		}
	case *ospb.ActivateResponse_ActivateError:
		actErr := resp.ActivateError
		if !expectFail {
			t.Fatalf("OS.Activate error %s: %s", actErr.Type, actErr.Detail)
		}
	default:
		t.Fatalf("OS.Activate unexpected response: got %v (%T)", resp, resp)
	}
}

// fetchStandbySupervisorStatus checks if the DUT has a standby supervisor available in a working state.
func (tc *testCase) fetchStandbySupervisorStatus(ctx context.Context, t *testing.T) {
	r, err := tc.osc.Verify(ctx, &ospb.VerifyRequest{})
	if err != nil {
		t.Fatal(err)
	}

	switch v := r.GetVerifyStandby().GetState().(type) {
	case *ospb.VerifyStandby_StandbyState:
		if v.StandbyState.GetState() == ospb.StandbyState_UNAVAILABLE {
			t.Fatal("OS.Verify RPC reports standby supervisor in UNAVAILABLE state.")
		}
		// All other supervisor states indicate this device does not support or have dual supervisors available.
		t.Log("DUT is detected as single supervisor.")
		tc.dualSup = false
	case *ospb.VerifyStandby_VerifyResponse:
		t.Log("DUT is detected as dual supervisor.")
		tc.dualSup = true
	default:
		t.Fatalf("Unexpected OS.Verify Standby State RPC Response: got %v (%T)", v, v)
	}
}

func (tc *testCase) rebootDUT(ctx context.Context, t *testing.T) {
	t.Log("Send DUT Reboot Request")
	_, err := tc.sc.Reboot(ctx, &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Force:   true,
		Message: "Apply GNOI OS Software Install",
	})
	if err != nil && status.Code(err) != codes.Unavailable {
		t.Fatalf("System.Reboot request failed: %s", err)
	}
}

func (tc *testCase) transferOS(ctx context.Context, t *testing.T, standby bool, version string, ErrorString string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ic, err := tc.osc.Install(ctx)
	if err != nil {
		t.Fatalf("OS.Install client request failed: %s", err)
	}
	// versionString := ""
	// if !force {
	// 	versionString = *osVersion
	// }
	// if dummyVersion {
	// 	if !force {
	// 		t.Fatalf("only force install can have dummy version")
	// 	} else {
	// 		versionString = "DUMMY"
	// 	}
	// }

	ireq := &ospb.InstallRequest{
		Request: &ospb.InstallRequest_TransferRequest{
			TransferRequest: &ospb.TransferRequest{
				Version:           version,
				StandbySupervisor: standby,
			},
		},
	}
	if err = ic.Send(ireq); err != nil {
		t.Fatalf("OS.Install error sending install request: %s", err)
	}

	iresp, err := ic.Recv()
	if err != nil {
		t.Fatalf("OS.Install error receiving: %s", err)
	}
	Message := ""
	switch v := iresp.GetResponse().(type) {
	case *ospb.InstallResponse_TransferReady:
	case *ospb.InstallResponse_Validated:
		if standby {
			t.Log("DUT standby supervisor has valid preexisting image; skipping transfer.")
		} else {
			t.Log("DUT supervisor has valid preexisting image; skipping transfer.")
		}
		osVersionReceived := iresp.GetValidated().GetVersion()
		t.Logf("Version :: got: %v ,want: %v", osVersionReceived, *osVersion)
		if osVersionReceived != *osVersion {
			t.Fatalf("OS.Install wrong version received, osVersionActual : %s osVersion %s, ", osVersionReceived, *osVersion)
		}
		return
	case *ospb.InstallResponse_SyncProgress:
		if !tc.dualSup {
			t.Fatalf("Unexpected SyncProgress on single supervisor: got %v (%T)", v, v)
		}
		t.Logf("Sync progress: %v%% synced from supervisor", v.SyncProgress.GetPercentageTransferred())
		// Message = fmt.Sprintf("Sync progress: %v%% synced from supervisor", v.SyncProgress.GetPercentageTransferred())
		// xx := iresp.GetValidated().Version
		// yy := iresp.GetValidated().GetVersion()
		// t.Logf("%v,%v", xx, yy)
		// if err != nil {
		// 	t.Fatalf("OS.Install error receiving: %s", err)
		// }
	default:
		Message = fmt.Sprintf("Expected TransferReady following TransferRequest: got %v (%T)", v, v)
		// t.Fatalf("Expected TransferReady following TransferRequest: got %v (%T)", v, v)
		t.Logf("Expected TransferReady following TransferRequest: got %v (%T)", v, v)
	}

	awaitChan := make(chan error)
	go func() {
		err := watchStatus(t, ic, standby)
		awaitChan <- err
	}()

	if ErrorString != "" {
		if !strings.Contains(Message, ErrorString) {
			t.Fatalf("want error: %v , and got: %v", ErrorString, Message)
		} else {
			t.Logf("want error: %v , and got: %v", ErrorString, Message)
			return
		}
	}

	// xx := iresp.GetValidated().Version
	// yy := iresp.GetValidated().GetVersion()
	// t.Logf("%v,%v", xx, yy)
	// if err != nil {
	// 	t.Fatalf("OS.Install error receiving: %s", err)
	// }

	if !standby {
		err = transferContent(ic, tc.reader)
		if err != nil {
			t.Fatalf("Error transferring content: %s", err)
		}
	}

	if err = <-awaitChan; err != nil {
		t.Fatalf("Transfer receive error: %s", err)
	}

	if standby {
		t.Log("OS.Install standby supervisor image transfer complete.")
	} else {
		t.Log("OS.Install supervisor image transfer complete.")
	}
}

// verifyInstall validates the OS.Verify RPC returns no failures and version numbers match the
// newly requested software version.
func (tc *testCase) verifyInstall(ctx context.Context, t *testing.T) {
	rebootWait := time.Minute
	deadline := time.Now().Add(*timeout)
	for time.Now().Before(deadline) {
		r, err := tc.osc.Verify(ctx, &ospb.VerifyRequest{})
		switch status.Code(err) {
		case codes.OK:
		case codes.Unavailable:
			t.Log("Reboot in progress.")
			time.Sleep(rebootWait)
			continue
		default:
			t.Fatalf("OS.Verify request failed: %v", err)
		}
		// when noreboot is set to false, the device returns "in-progress" before initiating the reboot
		if r.GetActivationFailMessage() == "in-progress" {
			t.Logf("Waiting for reboot to initiate.")
			time.Sleep(rebootWait)
			continue
		}
		if got, want := r.GetActivationFailMessage(), ""; got != want {
			t.Fatalf("OS.Verify ActivationFailMessage: got %q, want %q", got, want)
		}
		if got, want := r.GetVersion(), *osVersion; got != want {
			t.Logf("Reboot has not finished with the right version: got %s , want: %s.", got, want)
			time.Sleep(rebootWait)
			continue
		}

		dut := ondatra.DUT(t, "dut")
		if !deviations.SwVersionUnsupported(dut) {
			ver, ok := gnmi.Lookup(t, dut, gnmi.OC().System().SoftwareVersion().State()).Val()
			if !ok {
				t.Log("Reboot has not finished with the right version: couldn't get system/state/software-version")
				time.Sleep(rebootWait)
				continue
			}
			if got, want := ver, *osVersion; !strings.HasPrefix(got, want) {
				t.Logf("Reboot has not finished with the right version: got %s , want: %s.", got, want)
				time.Sleep(rebootWait)
				continue
			}
		}

		if tc.dualSup {
			if got, want := r.GetVerifyStandby().GetVerifyResponse().GetActivationFailMessage(), ""; got != want {
				t.Fatalf("OS.Verify Standby ActivationFailMessage: got %q, want %q", got, want)
			}

			if got, want := r.GetVerifyStandby().GetVerifyResponse().GetVersion(), *osVersion; got != want {
				t.Log("Standby not ready.")
				time.Sleep(rebootWait)
				continue
			}
		}

		t.Log("OS.Verify complete")
		return
	}

	t.Fatal("OS.Verify did not return the correct version before deadline.")
}

func transferContent(ic ospb.OS_InstallClient, reader io.ReadCloser) error {
	// The gNOI SetPackage operation sets the maximum chunk size at 64K,
	// so assuming the install operation allows for up to the same size.
	buf := make([]byte, 64*1024)
	defer closer.CloseAndLog(reader.Close, "error closing package file")
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			tc := &ospb.InstallRequest{
				Request: &ospb.InstallRequest_TransferContent{
					TransferContent: buf[0:n],
				},
			}
			if err := ic.Send(tc); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	te := &ospb.InstallRequest{
		Request: &ospb.InstallRequest_TransferEnd{
			TransferEnd: &ospb.TransferEnd{},
		},
	}
	return ic.Send(te)
}

func watchStatus(t *testing.T, ic ospb.OS_InstallClient, standby bool) error {
	var gotProgress bool

	for {
		iresp, err := ic.Recv()
		if err != nil {
			return err
		}

		switch v := iresp.GetResponse().(type) {
		case *ospb.InstallResponse_InstallError:
			errName := ospb.InstallError_Type_name[int32(v.InstallError.Type)]
			return fmt.Errorf("installation error %q: %s", errName, v.InstallError.GetDetail())
		case *ospb.InstallResponse_TransferProgress:
			if standby {
				return fmt.Errorf("unexpected TransferProgress: got %v, want SyncProgress", v)
			}
			t.Logf("Transfer progress: %v bytes received by DUT", v.TransferProgress.GetBytesReceived())
			gotProgress = true
		case *ospb.InstallResponse_SyncProgress:
			if !standby {
				return fmt.Errorf("unexpected SyncProgress: got %v, want TransferProgress", v)
			}
			t.Logf("Transfer progress: %v%% synced from supervisor", v.SyncProgress.GetPercentageTransferred())
			gotProgress = true
		case *ospb.InstallResponse_Validated:
			if !gotProgress {
				return fmt.Errorf("transfer completed without progress status")
			}
			if got, want := v.Validated.GetVersion(), *osVersion; got != want {
				return fmt.Errorf("mismatched validation software versions: got %s, want %s", got, want)
			}
			return nil
		default:
			return fmt.Errorf("unexpected client install response: got %v (%T)", v, v)
		}
	}
}

func TestPushAndVerifyInterfaceConfig(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	t.Logf("Create and push interface config to the DUT")
	dutPort := dut.Port(t, "port1")
	dutPortName := dutPort.Name()
	intf1 := dutAttrs.NewOCInterface(dutPortName, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intf1.GetName()).Config(), intf1)

	dc := gnmi.OC().Interface(dutPortName).Config()
	in := configInterface(dutPortName, dutAttrs.Desc, dutAttrs.IPv4, dutAttrs.IPv4Len, dut)
	fptest.LogQuery(t, fmt.Sprintf("%s to Replace()", dutPort), dc, in)
	gnmi.Replace(t, dut, dc, in)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		ocPortName := dut.Port(t, "port1").Name()
		fptest.AssignToNetworkInstance(t, dut, ocPortName, deviations.DefaultNetworkInstance(dut), 0)
	}

	t.Logf("Fetch interface config from the DUT using Get RPC and verify it matches with the config that was pushed earlier")
	if val, present := gnmi.LookupConfig(t, dut, dc).Val(); present {
		compareStructs(t, in, val)
	} else {
		t.Errorf("Config %v Get() failed", dc)
	}
}

// configInterface generates an interface's configuration based on the the attributes given.
func configInterface(name, desc, ipv4 string, prefixlen uint8, dut *ondatra.DUTDevice) *oc.Interface {
	i := &oc.Interface{}
	i.Name = ygot.String(name)
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()

	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}

	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return i
}

func TestPushAndVerifyBGPConfig(t *testing.T) {
	// peer := ondatra.DUT(t, "peer")
	// fptest.ConfigureDefaultNetworkInstance(t, peer)

	// t.Logf("Create and push BGP config to the DUT")
	// peerConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(peer)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	// peerConf := bgpCreateNbr(peer)
	// gnmi.Replace(t, peer, peerConfPath.Config(), peerConf)

	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	t.Logf("Create and push BGP config to the DUT")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := bgpCreateNbr(dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

	t.Logf("Fetch BGP config from the DUT using Get RPC and verify it matches with the config that was pushed earlier")
	if val, present := gnmi.LookupConfig(t, dut, dutConfPath.Config()).Val(); present {
		compareStructs(t, dutConf, val)
	} else {
		t.Errorf("Config %v Get() failed", dutConfPath.Config())
	}
}

func compareStructs(t *testing.T, dutConf, val interface{}) {
	vdutConf := reflect.ValueOf(dutConf).Elem()
	vval := reflect.ValueOf(val).Elem()
	tdutConf := vdutConf.Type()

	for i := 0; i < vdutConf.NumField(); i++ {
		fieldName := tdutConf.Field(i).Name
		fdutConf := vdutConf.Field(i)
		fval := vval.FieldByName(fieldName)
		if fdutConf.Kind() == reflect.Ptr && fdutConf.IsNil() {
			continue
		} else {
			if fdutConf.Kind() == reflect.Ptr && fdutConf.Elem().Kind() == reflect.Struct {
				compareStructs(t, fdutConf.Interface(), fval.Interface())
			} else if fdutConf.Kind() == reflect.Map && fdutConf.IsValid() {
				for _, key := range fdutConf.MapKeys() {
					strct := fdutConf.MapIndex(key)
					strctVal := fval.MapIndex(key)
					log.Printf("strct %v \t, strctVal %v\n", strct, strctVal)
					if strct.Kind() == reflect.Map {
						compareStructs(t, strct.Interface(), strctVal.Interface())
					} else if fdutConf.Kind() == reflect.Ptr && fdutConf.IsNil() {
						continue
					} else if strct.Kind() == reflect.Ptr && strct.Elem().Kind() == reflect.Struct {
						compareStructs(t, strct.Interface(), strctVal.Interface())
					}
				}
			} else if reflect.DeepEqual(fdutConf.Interface(), fval.Interface()) {
				t.Logf("The field %s is equal in both structs\n got: %#v \t want: %#v \n", fieldName, fdutConf.Interface(), fval.Interface())
			} else {
				if !(fieldName == "SendCommunity") {
					t.Errorf("The field %s is not equal in both structs\n got: %#v \t want: %#v \n", fieldName, fdutConf.Interface(), fval.Interface())
				} else {
					continue
				}
			}
		}

	}
}

// bgpCreateNbr creates a BGP object with neighbor pointing to ateSrc
func bgpCreateNbr(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(uint32(bgpGlobalAttrs.dutAS))
	global.RouterId = ygot.String(dutSrc.IPv4)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	pgv4 := bgp.GetOrCreatePeerGroup(bgpGlobalAttrs.peerGrpNamev4)
	pgv4.PeerAs = ygot.Uint32(uint32(bgpGlobalAttrs.ateAS))
	pgv4.PeerGroupName = ygot.String(bgpGlobalAttrs.peerGrpNamev4)
	nbr := bgpNbr1
	nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
	nv4.PeerAs = ygot.Uint32(nbr.peerAs)
	nv4.Enabled = ygot.Bool(true)
	nv4.PeerGroup = ygot.String(bgpGlobalAttrs.peerGrpNamev4)
	return niProto
}

func cleanCommandOutput(output string) string {
	// Remove any leading and trailing whitespace
	output = strings.TrimSpace(output)

	// Split the output into lines
	lines := strings.Split(output, "\n")

	// Iterate through the lines and find the last non-empty line, which should contain the actual result
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}

	return ""
}

func cleanupDiskSpace(ctx context.Context, sshClient binding.CLIClient) error {
	cleanupCmd := "run rm /misc/disk1/disk_fill_file"
	if _, err := sshClient.RunCommand(ctx, cleanupCmd); err != nil {
		return fmt.Errorf("failed to clean up disk fill file: %w", err)
	}
	return nil
}

func retrieveAndFillDiskSpace(ctx context.Context, sshClient binding.CLIClient, thresholdGB float64) (bool, error) {
	// Retrieve available disk space
	spaceCmd := "run df -h /dev/mapper/main--xr--vg-install--data--disk1 | awk 'NR==2 {print $4}'"
	spaceResult, err := sshClient.RunCommand(ctx, spaceCmd)
	if err != nil {
		return false, fmt.Errorf("failed to get available space: %w", err)
	}

	// Clean the output using the utility function
	spaceStr := cleanCommandOutput(spaceResult.Output())

	// Determine the unit and parse the space
	var availableSpaceGB float64
	switch {
	case strings.HasSuffix(spaceStr, "G"):
		spaceGB, err := strconv.ParseFloat(strings.TrimSuffix(spaceStr, "G"), 64)
		if err != nil {
			return false, fmt.Errorf("failed to parse available space in GB: %w", err)
		}
		availableSpaceGB = spaceGB
	case strings.HasSuffix(spaceStr, "M"):
		spaceMB, err := strconv.ParseFloat(strings.TrimSuffix(spaceStr, "M"), 64)
		if err != nil {
			return false, fmt.Errorf("failed to parse available space in MB: %w", err)
		}
		availableSpaceGB = spaceMB / 1024
	default:
		return false, fmt.Errorf("unexpected space unit: %s", spaceStr)
	}

	// Check if disk fill should be skipped
	if availableSpaceGB < thresholdGB {
		return true, nil
	}

	// Fill the disk
	fillSpace := availableSpaceGB - 1
	fillCmd := fmt.Sprintf("run fallocate -l %dG /misc/disk1/disk_fill_file", int(fillSpace))
	if _, err := sshClient.RunCommand(ctx, fillCmd); err != nil {
		return false, fmt.Errorf("failed to fill the disk: %w", err)
	}

	return false, nil
}

// Function to get ISO version information
func getIsoVersionInfo(imagePath string) (string, error) {
	// Construct the command with the image path
	cmd := exec.Command("/auto/ioxprojects13/lindt-giso/isols.py", "--iso", imagePath, "--build-info")

	// Run the command and capture the output
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run command: %v", err)
	}

	// Parse the command output to extract version and label
	output := out.String()
	version, label := parseVersionInfo(output)

	// Format the result as <version>-<label>
	result := version
	if label != "" {
		result += "-" + label
	}

	return result, nil
}

// Helper function to parse version information from command output
func parseVersionInfo(input string) (string, string) {
	var version, label string

	// Regex to find the version number
	versionRegex := regexp.MustCompile(`Version:\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+[A-Z]?)`)
	versionMatches := versionRegex.FindStringSubmatch(input)
	if len(versionMatches) > 1 {
		version = versionMatches[1]
	}

	// Regex to find the label in GISO Build Command
	labelRegex := regexp.MustCompile(`--label\s+(\S+)`)
	labelMatches := labelRegex.FindStringSubmatch(input)
	if len(labelMatches) > 1 {
		label = labelMatches[1]
	}

	return version, label
}
func isGreater(epochTime1, epochTime2 int64) bool {
	return epochTime1 > epochTime2
}

// Extracts the creation time from the ls command output and converts it to epoch time
func extractCreationTime(output string) (int64, error) {
	// Regex to match the file creation date and time
	regex := regexp.MustCompile(`\w{3}\s+\d{1,2}\s+\d{2}:\d{2}`) // Matches "Nov 13 13:25"
	matches := regex.FindStringSubmatch(output)
	if len(matches) < 1 {
		return 0, fmt.Errorf("no match found for creation date and time")
	}

	// Parse the date and time into a time.Time object
	layout := "Jan 2 15:04"
	creationTime, err := time.Parse(layout, matches[0])
	if err != nil {
		return 0, err
	}

	// Convert to epoch time (Unix timestamp)
	return creationTime.Unix(), nil
}

// Helper function to run a command on the DUT and return the output
func runCommand(t *testing.T, dut *ondatra.DUTDevice, cmd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	sshClient := dut.RawAPIs().CLI(t)

	if result, err := sshClient.RunCommand(ctx, cmd); err == nil {
		t.Logf("%s> %s", dut.ID(), cmd)
		t.Log(result.Output())
		return result.Output()
	} else {
		t.Logf("%s> %s", dut.ID(), cmd)
		t.Log(err.Error())
		return ""
	}
}

// Removes the specified ISO file on the DUT
func removeISOFile(t *testing.T, dut *ondatra.DUTDevice, version string) {
	cmd := fmt.Sprintf("run rm -rf /misc/disk1/8000-golden-x-%s.iso", version)
	runCommand(t, dut, cmd)
}

// Lists the specified ISO file on the DUT and extracts its creation date and time
func listISOFile(t *testing.T, dut *ondatra.DUTDevice, version string) (int64, error) {
	cmd := fmt.Sprintf("run ls -ltr /misc/disk1/8000-golden-x-%s.iso", version)
	output := runCommand(t, dut, cmd)

	// Extract date and time from the output
	creationTime, err := extractCreationTime(output)
	if err != nil {
		t.Logf("Error extracting creation time: %v", err)
		return 0, err
	}

	return creationTime, nil
}
