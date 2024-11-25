package osinstall_test

import (
	"fmt"
	"testing"
	"time"

	"flag"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/deviations"
)

var (
	// osFile                          = flag.String("osFile", "", "Path to the OS image under test for the install operation")
	// osFileForceDownloadSupported    = flag.String("osFileForceDownloadSupported", "", "Path to the OS image (Force Download Supported) for the install operation")
	// osFileForceDownloadNotSupported = flag.String("osFileForceDownloadNotSupported", "", "Path to the OS image ((Force Download not Supported)) for the install operation")
	osFile                          = flag.String("osFile", "/auto/b4ws/xr/builds/nightly/XR-DEV_NIGHTLY_24_11_15C/img-8000/8000-x64.iso", "Path to the OS image under test for the install operation")
	osFileForceDownloadSupported    = flag.String("osFileForceDownloadSupported", "/auto/b4ws/xr/builds/nightly/XR-DEV_NIGHTLY_24_11_13C/img-8000/8000-x64.iso", "Path to the OS image (Force Download Supported) for the install operation")
	osFileForceDownloadNotSupported = flag.String("osFileForceDownloadNotSupported", "/auto/b4ws/xr/builds/nightly/XR-DEV_NIGHTLY_24_11_10C/img-8000/8000-x64.iso", "Path to the OS image ((Force Download not Supported)) for the install operation")
	timeout                         = flag.Duration("timeout", time.Minute*30, "Time to wait for reboot to complete")
)

const (
	// got &{type:TOO_LARGE detail:"Too large, no disk space"} (*os.InstallResponse_InstallError)
	installDiskFullError = "Too large, no disk space"

	// got &{type:NOT_SUPPORTED_ON_BACKUP detail:"Install operation not supported on standby supervisor"}
	installStandbyNotSupportedError = "Install operation not supported on standby supervisor"

	// got &{detail:"Standby supervisor doesn't exist for fixed platform"} (*os.InstallResponse_InstallError)
	installStandbyNotPresentError = "Standby supervisor doesn't exist for fixed platform"

	// gNOI install update request: rpc error: code = Internal desc = { "cisco-grpc:errors": {  "error": [   {    "error-type": "application",    "error-tag": "operation-failed",    "error-severity": "error",    "error-message": "'Install' detected the 'warning' condition 'Packaging operation in progress. Cannot accept further requests until complete'"   }  ] }}"}"
	// gNOI install update request: rpc error: code = Internal desc = { "cisco-grpc:errors": {  "error": [   {    "error-type": "application",    "error-tag": "operation-failed",    "error-severity": "error",    "error-message": "'Install' detected the 'warning' condition 'Apply atomic change in progress. Cannot accept further requests until complete'"   }  ] }}"}"
	activateInProgressError = "Cannot accept further requests until complete"

	// OS.Activate error NON_EXISTENT_VERSION: xx.x.x.xxx doesn't exist
	activateImageNotAvailableError = "doesn't exist"

	// activate response "activate_error:{detail:"no_reboot not supported"}"
	activateNoRebootNotSupported = "no_reboot not supported"

	// activate_error:{type:NOT_SUPPORTED_ON_BACKUP detail:"activate operation on standby supervisor is not supported"
	activateOnStandbyNotSupported = "activate operation on standby supervisor is not supported"
)

// var (
// 	activateNegativeTestCases = []struct {
// 		version       bool
// 		noReboot      bool
// 		standby       bool
// 		expectFail    bool
// 		expectedError string
// 	}{
// 		//version,noReboot,standby,expectFail,expectedError
// 		// version : flase = wrong version, true = correct version
// 		{false, true, true, true, activateOnStandbyNotSupported},
// 		{false, true, false, true, activateNoRebootNotSupported},
// 		{false, false, true, true, activateOnStandbyNotSupported},
// 		{false, false, false, true, activateImageNotAvailableError},
// 		{true, true, true, true, activateOnStandbyNotSupported},
// 		{true, true, false, true, activateNoRebootNotSupported},
// 		{true, false, true, true, activateOnStandbyNotSupported},
// 	}
// )

func testOSForceTransferDiskFull(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto
	// New test case: Simulate disk full scenario
	t.Run("disk full scenario", func(t *testing.T) {
		sshClient := tc.dut.RawAPIs().CLI(t)

		// Retrieve and fill disk space
		skipFill, err := retrieveAndFillDiskSpace(tc.ctx, sshClient, 1.1)
		if err != nil {
			t.Fatalf("Error during disk space retrieval and fill: %v", err)
		}
		if skipFill {
			t.Log("Available space is less than 1.1GB, skipping disk fill step.")
			return
		}
		defer func() {
			if err := cleanupDiskSpace(tc.ctx, sshClient); err != nil {
				t.Fatalf("Cleanup failed: %v", err)
			}
		}()

		// Attempt to transfer the OS and verify failure
		tc.transferOS(tc.ctx, t, false, "", installDiskFullError)
	})
	// verifyGnoiStats(t, tc)
}

func testOSNormalTransferDiskFull(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto
	// defer // verifyGnoiStats(t, tc)
	// New test case: Simulate disk full scenario
	t.Run("disk full scenario", func(t *testing.T) {
		sshClient := tc.dut.RawAPIs().CLI(t)

		// Retrieve and fill disk space
		skipFill, err := retrieveAndFillDiskSpace(tc.ctx, sshClient, 1.1)
		if err != nil {
			t.Fatalf("Error during disk space retrieval and fill: %v", err)
		}
		if skipFill {
			t.Log("Available space is less than 1.1GB, skipping disk fill step.")
			return
		}
		defer func() {
			if err := cleanupDiskSpace(tc.ctx, sshClient); err != nil {
				t.Fatalf("Cleanup failed: %v", err)
			}
		}()

		// Attempt to transfer the OS and verify failure
		tc.transferOS(tc.ctx, t, false, tc.osVersion, installDiskFullError)
	})
	// verifyGnoiStats(t, tc)
}

// func verifyGnoiStats(t *testing.T, tc testCase) {
// 	t.Run("verify gnoi stats", func(t *testing.T) {
// 		data := cliparser.ParseShowGnoiStats(t, tc.dut).OSProto
// 		if tc.oss == data {
// 			t.Logf("want:%v, got %v", tc.oss, data)
// 		} else {
// 			t.Fatalf("want:%v, got %v", tc.oss, data)
// 		}
// 	})
// }

func testOSForceTransferStandby(t *testing.T, tc testCase) {
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto
	// defer // verifyGnoiStats(t, tc)
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	// Attempt to transfer the OS with standby flag and verify failure
	t.Run("testOSForceTransferStandby", func(t *testing.T) {
		if tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", installStandbyNotSupportedError)

		} else {
			tc.transferOS(tc.ctx, t, true, "", installStandbyNotPresentError)
		}
	})
	// verifyGnoiStats(t, tc)
}

func testOSNormalTransferStandby(t *testing.T, tc testCase) {
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	t.Run("testOSNormalTransferStandby", func(t *testing.T) {
		// Attempt to transfer the OS with standby flag and verify failure
		if tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", installStandbyNotSupportedError)

		} else {
			tc.transferOS(tc.ctx, t, true, "", installStandbyNotPresentError)
		}
	})
	// verifyGnoiStats(t, tc)
}

func testOSForceTransferNoExistingImageEmptyVersion(t *testing.T, tc testCase) {
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	// Force Install
	t.Run("Force install No ExistingImage using Empty version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, "", "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		removeISOFile(t, tc.dut, tc.osVersion)
		if t1 != 0 || t2 == 0 {
			t.Fatal("image not force updated")
		}
	})
	// verifyGnoiStats(t, tc)
}

func testOSForceTransferNoExistingImageWrongVersion(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto

	// Force Install
	t.Run("Force install No ExistingImage using Wrong version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, "WRONG_VERSION", "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		removeISOFile(t, tc.dut, tc.osVersion)
		if t1 != 0 || t2 == 0 {
			t.Fatal("image not force updated")
		}
	})
	// verifyGnoiStats(t, tc)

}

func testOSNormalTransferNoExistingImage(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto

	t.Run("Normal install NO ExistingImage", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, tc.osVersion, "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 != 0 || t2 == 0 {
			t.Fatal("image not force updated")
		}
	})
	// verifyGnoiStats(t, tc)

}

func testOSNormalTransferExistingImage(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto

	// Force Install
	t.Run("Normal install ExistingImage", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, tc.osVersion, "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 != t2 {
			t.Fatal("image changed")
		}
	})
	// verifyGnoiStats(t, tc)

}

func testOSForceTransferExistingImageEmptyVersion(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto

	// Force Install
	t.Run("Force install ExistingImage using Empty version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, "", "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 == 0 || isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})
	// verifyGnoiStats(t, tc)

}

func testOSForceInstallSupportedToSupportedImage(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto

	t.Run("Force install ExistingImage using Wrong version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		// tc.transferOS(ctx, t, false, tc.osVersion, "")
		tc.transferOS(tc.ctx, t, false, "WRONG_VERSION", "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 == 0 || isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})
	// verifyGnoiStats(t, tc)
	// tc.oss = cliparser.ParseShowGnoiStats(t, tc.dut).OSProto
	for _, activateTC := range tc.negActivateTestCases {
		version := "1.2.3.4I-wrong"
		if activateTC.version == true {
			version = tc.osVersion
		}
		t.Run(fmt.Sprintf("TestActivate Negative case Version=%s NoReboot=%t Standby=%t", version, activateTC.noReboot, activateTC.standby), func(t *testing.T) {
			tc.activateOS(tc.ctx, t, activateTC.standby, activateTC.noReboot, version, activateTC.expectFail, activateTC.expectedError)
		})
	}
	// verifyGnoiStats(t, tc)

	t.Run("test Supervisor Switchover", func(t *testing.T) {
		util.SupervisorSwitchover(t, tc.dut)
		tc.pollRpc(t)
	})
	if tc.dualSup {
		t.Run("Activating using correct version expected failure no image", func(t *testing.T) {
			tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, true, activateImageNotAvailableError)

			if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
				tc.transferOS(tc.ctx, t, true, "", "")
				tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, true, activateImageNotAvailableError)
			}
		})

		tc.fetchOsFileDetails(t, tc.osFile)

		t.Run("Force install Image using empty version", func(t *testing.T) {
			t1, _ := listISOFile(t, tc.dut, tc.osVersion)
			// tc.transferOS(ctx, t, false, tc.osVersion, "")
			tc.transferOS(tc.ctx, t, false, "", "")
			if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
				tc.transferOS(tc.ctx, t, true, "", "")
			}
			t2, _ := listISOFile(t, tc.dut, tc.osVersion)
			if isGreater(t1, t2) {
				t.Fatal("image not force updated")
			}
		})

		for _, activateTC := range tc.negActivateTestCases {
			version := "1.2.3.4I-wrong"
			if activateTC.version == true {
				version = tc.osVersion
			}
			t.Run(fmt.Sprintf("TestActivate Negative case Version=%s NoReboot=%t Standby=%t", version, activateTC.noReboot, activateTC.standby), func(t *testing.T) {
				tc.activateOS(tc.ctx, t, activateTC.standby, activateTC.noReboot, version, activateTC.expectFail, activateTC.expectedError)
			})
		}
	}

	t.Run("Activating using correct version - expected success", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, false, "")

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, false, "")
		}
	})

	t.Run("Activating using correct version - expected failure already activated", func(t *testing.T) {
		// time.Sleep(5 * time.Second)
		tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, true, activateInProgressError)

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, true, activateInProgressError)
		}
		if tc.noReboot {
			tc.rebootDUT(tc.ctx, t)
		}
	})

	t.Run(fmt.Sprintf("Verify correct image comes up, expected image: %v", tc.osVersion), func(t *testing.T) {
		tc.verifyInstall(tc.ctx, t)
	})

	// t.Run("Test interface and BGP config after install", func(t *testing.T) {
	// 	testPushAndVerifyInterfaceConfig(t, tc.dut)
	// 	testPushAndVerifyBGPConfig(t, tc.dut)
	// })
}

func testOSForceInstallSupportedToNotSupportedImage(t *testing.T, tc testCase) {
	// if tc.osFile == *osFileForceDownloadNotSupported {
	// 	t.Skip()
	// }
	// tc.fetchOsFileDetails(t, *osFile)
	tc.fetchOsFileDetails(t, *osFileForceDownloadNotSupported)
	noReboot := deviations.OSActivateNoReboot(tc.dut)
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	t.Run("Force install old image using Wrong version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		// tc.transferOS(ctx, t, false, tc.osVersion, "")
		tc.transferOS(tc.ctx, t, false, "WRONG_VERSION", "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 != 0 || isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})
	for _, activateTC := range tc.negActivateTestCases {
		version := "1.2.3.4I-wrong"
		if activateTC.version == true {
			version = tc.osVersion
		}
		t.Run(fmt.Sprintf("TestActivate Negative case Version=%s NoReboot=%t Standby=%t", version, activateTC.noReboot, activateTC.standby), func(t *testing.T) {
			tc.activateOS(tc.ctx, t, activateTC.standby, activateTC.noReboot, version, activateTC.expectFail, activateTC.expectedError)
		})
	}

	t.Run("Activating using correct version - expected success", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, noReboot, tc.osVersion, false, "")

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, noReboot, tc.osVersion, false, "")
		}
	})

	t.Run("Activating using correct version - expected failure already activated", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, noReboot, tc.osVersion, true, activateInProgressError)

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, noReboot, tc.osVersion, true, activateInProgressError)
		}
		if noReboot {
			tc.rebootDUT(tc.ctx, t)
		}
	})

	t.Run(fmt.Sprintf("Verify correct image comes up, expected image: %v", tc.osVersion), func(t *testing.T) {
		tc.verifyInstall(tc.ctx, t)
	})

	// t.Run("Test interface and BGP config after install", func(t *testing.T) {
	// 	testPushAndVerifyInterfaceConfig(t, tc.dut)
	// 	testPushAndVerifyBGPConfig(t, tc.dut)
	// })
}

func testOSNormalInstallNotSupportedToSupportedImage(t *testing.T, tc testCase) {
	// if tc.osFile == *osFileForceDownloadSupported {
	// 	t.Skip()
	// }
	tc.fetchOsFileDetails(t, *osFile)

	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	t.Run("Normal install back to original version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, tc.osVersion, "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 != 0 || t2 == 0 {
			t.Fatal("image not force updated")
		}
	})

	for _, activateTC := range tc.negActivateTestCases {
		version := "1.2.3.4I-wrong"
		if activateTC.version == true {
			version = tc.osVersion
		}
		t.Run(fmt.Sprintf("TestActivate Negative case Version=%s NoReboot=%t Standby=%t", version, activateTC.noReboot, activateTC.standby), func(t *testing.T) {
			tc.activateOS(tc.ctx, t, activateTC.standby, activateTC.noReboot, version, activateTC.expectFail, activateTC.expectedError)
		})
	}
	t.Run("Activating using correct version - expected success", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, false, "")

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, false, "")
		}
	})

	t.Run("Activating using correct version - expected failure already activated", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, true, activateInProgressError)

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, true, activateInProgressError)
		}
		if tc.noReboot {
			tc.rebootDUT(tc.ctx, t)
		}
	})

	t.Run(fmt.Sprintf("Verify correct image comes up, expected image: %v", tc.osVersion), func(t *testing.T) {
		tc.verifyInstall(tc.ctx, t)
	})

	t.Run("Test interface and BGP config after install", func(t *testing.T) {
		testPushAndVerifyInterfaceConfig(t, tc.dut)
		testPushAndVerifyBGPConfig(t, tc.dut)
	})

}
