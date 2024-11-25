package osinstall_test

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/cliparser"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	ospb "github.com/openconfig/gnoi/os"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
)

type TestFunction struct {
	name string
	desc string
	fn   func(t *testing.T, tc testCase)
}

var (
	OSForceTransferTestCases = []TestFunction{
		{
			name: "testOSForceTransferDiskFull",
			desc: "ForceTransfer when Hard Disk is Full",
			fn:   testOSForceTransferDiskFull,
		},
		{
			name: "testOSNormalTransferDiskFull",
			desc: "NormalTransfer when Hard Disk is Full",
			fn:   testOSNormalTransferDiskFull,
		},
		{
			name: "testOSForceTransferStandby",
			desc: "ForceTransfer with Standby Flag enabled",
			fn:   testOSForceTransferStandby,
		},
		{
			name: "testOSNormalTransferStandby",
			desc: "NormalTransfer to Standby Flag enabled",
			fn:   testOSNormalTransferStandby,
		},
		{
			name: "testOSForceTransferNoExistingImageEmptyVersion",
			desc: "ForceTransfer No Existing Image with Empty Version",
			fn:   testOSForceTransferNoExistingImageEmptyVersion,
		},
		{
			name: "testOSForceTransferNoExistingImageWrongVersion",
			desc: "ForceTransfer No Existing Image with Wrong Version",
			fn:   testOSForceTransferNoExistingImageWrongVersion,
		},
		{
			name: "testOSNormalTransferNoExistingImage",
			desc: "Normal Transfer No Existing Image with correct version",
			fn:   testOSNormalTransferNoExistingImage,
		},
		{
			name: "testOSNormalTransferExistingImage",
			desc: "Normal Transfer Existing Image with correct Version",
			fn:   testOSNormalTransferExistingImage,
		},
		{
			name: "testOSForceTransferExistingImageEmptyVersion",
			desc: "ForceTransfer Existing Image with Empty Version",
			fn:   testOSForceTransferExistingImageEmptyVersion,
		},
		{
			name: "testOSForceInstallSupportedToSupportedImage",
			desc: "ForceTransfer-Activate-Verify from supported to supported image",
			fn:   testOSForceInstallSupportedToSupportedImage,
		},
		{
			name: "testOSForceInstallSupportedToNotSupportedImage",
			desc: "ForceTransfer-Activate-Verify from supported to un supported image",
			fn:   testOSForceInstallSupportedToNotSupportedImage,
		},
		{
			name: "testOSNormalInstallNotSupportedToSupportedImage",
			desc: "NormalTransfer-Activate-Verify from not supported to supported image",
			fn:   testOSNormalInstallNotSupportedToSupportedImage,
		},
	}
)

type activateNegativeTestCases struct {
	version       bool
	noReboot      bool
	standby       bool
	expectFail    bool
	expectedError string
}
type testCase struct {
	dut *ondatra.DUTDevice
	// dualSup indicates if the DUT has a standby supervisor available.
	dualSup bool
	reader  io.ReadCloser

	osc                    ospb.OSClient
	sc                     spb.SystemClient
	ctx                    context.Context
	noReboot               bool
	forceDownloadSupported bool
	oss                    cliparser.OSPProtoStats

	osFile    string
	osVersion string
	timeout   time.Duration

	negActivateTestCases []activateNegativeTestCases
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOSInstall(t *testing.T) {

	testCases := []testCase{}

	for _, device := range ondatra.DUTs(t) {
		tc1 := testCase{
			dut:      device,
			osc:      device.RawAPIs().GNOI(t).OS(),
			sc:       device.RawAPIs().GNOI(t).System(),
			ctx:      context.Background(),
			noReboot: deviations.OSActivateNoReboot(device),
		}
		tc1.negActivateTestCases = []activateNegativeTestCases{
			//version,noReboot,standby,expectFail,expectedError
			// version : flase = wrong version, true = correct version
			{false, true, true, true, activateOnStandbyNotSupported},
			{false, true, false, true, activateNoRebootNotSupported},
			{false, false, true, true, activateOnStandbyNotSupported},
			{false, false, false, true, activateImageNotAvailableError},
			{true, true, true, true, activateOnStandbyNotSupported},
			{true, true, false, true, activateNoRebootNotSupported},
			{true, false, true, true, activateOnStandbyNotSupported},
		}
		tc1.fetchStandbySupervisorStatus(t)
		tc1.fetchOsFileDetails(t, *osFileForceDownloadSupported)
		// if i == 0 {
		// 	tc1.fetchOsFileDetails(t, *osFileForceDownloadSupported)
		// } else {
		// 	tc1.fetchOsFileDetails(t, *osFileForceDownloadNotSupported)
		// }
		tc1.updatePackageReader(t)
		tc1.setTimeout(t, *timeout)
		tc1.updateForceDownloadSupport(t)
		testCases = append(testCases, tc1)
		// }

	}

	var wg sync.WaitGroup
	for _, tc := range testCases {
		// runner(t, tc)
		// Increment the WaitGroup counter
		wg.Add(1)
		// Launch a goroutine for each testcase/device
		go func(tc testCase) {
			// Decrement the counter when the goroutine completes
			defer wg.Done()
			// Run your test cases for the device
			runner(t, tc)
		}(tc)
	}
	// Wait for all goroutines to finish
	wg.Wait()
}

func runner(t *testing.T, tc testCase) {
	for _, tf := range OSForceTransferTestCases {
		t.Run(fmt.Sprintf("%v:%v", tc.dut.ID(), tf.name), func(t *testing.T) {
			t.Logf("Name: %s", tf.name)
			t.Logf("Description: %s", tf.desc)
			tf.fn(t, tc)
		})
	}
}
