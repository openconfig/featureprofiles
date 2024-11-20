package osinstall_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"flag"

	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/protobuf/encoding/prototext"

	spb "github.com/openconfig/gnoi/system"
)

var (
	osFile                          = flag.String("osFile", "", "Path to the OS image under test for the install operation")
	osFileForceDownloadSupported    = flag.String("osFileForceDownloadSupported", "", "Path to the OS image (Force Download Supported) for the install operation")
	osFileForceDownloadNotSupported = flag.String("osFileForceDownloadNotSupported", "", "Path to the OS image ((Force Download not Supported)) for the install operation")
	timeout                         = flag.Duration("timeout", time.Minute*30, "Time to wait for reboot to complete")
)

const (
	DiskFullError         = "Too large, no disk space"
	alreadyActivatedError = "'Install' detected the 'warning' condition 'Apply atomic change in progress. Cannot accept further requests until complete'"
	imageNotAvailable     = "doesn't exist"
)
const (
	maxSwitchoverTime = 900
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

func processBindingFile(t *testing.T) (*bindpb.Binding, error) {
	t.Helper()
	t.Log("Starting processing binding file")

	bf := flag.Lookup("binding")
	if bf == nil {
		return nil, fmt.Errorf("binding file flag not set correctly")
	}

	bindingFile := bf.Value.String()
	if bindingFile == "" {
		return nil, fmt.Errorf("binding file path is empty")
	}

	in, err := os.ReadFile(bindingFile)
	if err != nil {
		return nil, fmt.Errorf("error reading binding file: %v", err)
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return nil, fmt.Errorf("error processing binding file: %v", err)
	}

	t.Log("Processing binding file successful")
	return b, nil
}

func testOSTransferDiskFull(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

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

		// Step 3: Attempt to transfer the OS and verify failure
		tc.transferOS(tc.ctx, t, false, "", DiskFullError)

	})
}

func testOSForceTransferStandby(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	// Attempt to transfer the OS with standby flag and verify failure
	t.Run("testOSForceTransferStandby", func(t *testing.T) {
		tc.transferOS(tc.ctx, t, true, "", "supervisor")
	})
}

func testOSNormalTransferStandby(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)
	t.Run("testOSNormalTransferStandby", func(t *testing.T) {
		// Attempt to transfer the OS with standby flag and verify failure
		tc.transferOS(tc.ctx, t, true, "", "supervisor")
	})
}

func testOSForceTransfer1(t *testing.T, tc testCase) {

	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	// Force Install
	t.Run("Force install No ExistingImage using Empty version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, "", "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		removeISOFile(t, tc.dut, tc.osVersion)
		if t1 != 0 && t2 == 0 {
			t.Fatal("image not force updated")
		}
	})
}

func testOSForceTransfer2(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	// Force Install
	t.Run("Force install No ExistingImage using Wrong version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, "WRONG_VERSION", "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		removeISOFile(t, tc.dut, tc.osVersion)
		if t1 != 0 && t2 == 0 {
			t.Fatal("image not force updated")
		}
	})
}

func testOSNormalTransfer1(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	t.Run("Normal install NO ExistingImage", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, tc.osVersion, "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 != 0 && t2 == 0 {
			t.Fatal("image not force updated")
		}
	})
}

func testOSNormalTransfer2(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	// Force Install
	t.Run("Normal install ExistingImage", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, tc.osVersion, "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 == 0 && t1 != t2 {
			t.Fatal("image changed")
		}
	})
}

func testOSForceTransfer3(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	// Force Install
	t.Run("Force install ExistingImage using Empty version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, "", "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 == 0 && isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})
}

func testOSForceInstall1(t *testing.T, tc testCase) {
	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	t.Run("Force install ExistingImage using Wrong version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		// tc.transferOS(ctx, t, false, tc.osVersion, "")
		tc.transferOS(tc.ctx, t, false, "WRONG_VERSION", "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 == 0 && isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})
	t.Run("Try Activating using Wrong version expected failure", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, tc.noReboot, "WRONG_VERSION", true, imageNotAvailable)

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, tc.noReboot, "WRONG_VERSION", true, imageNotAvailable)
		}
	})

	t.Run("test Supervisor Switchover", func(t *testing.T) {
		testSupervisorSwitchover(t, tc)
		tc.pollRpc(t)
	})
	if tc.dualSup {
		t.Run("Activating using correct version expected failure no image", func(t *testing.T) {
			tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, true, imageNotAvailable)

			if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
				tc.transferOS(tc.ctx, t, true, "", "")
				tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, true, imageNotAvailable)
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
	}

	t.Run("Activating using correct version - expected success", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, false, "")

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, false, "")
		}
	})

	t.Run("Activating using correct version - expected failure already activated", func(t *testing.T) {
		// time.Sleep(5 * time.Second)
		tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, true, alreadyActivatedError)

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, true, alreadyActivatedError)
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

func testSupervisorSwitchover(t *testing.T, tc testCase) {
	// dut := ondatra.DUT(t, "dut")

	controllerCards := components.FindComponentsByType(t, tc.dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", tc.dut.Model(), got, want)
	}

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, tc.dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, tc.dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, tc.dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, tc.dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}

	intfsOperStatusUPBeforeSwitch := helpers.FetchOperStatusUPIntfs(t, tc.dut, *args.CheckInterfacesInBinding)
	t.Logf("intfsOperStatusUP interfaces before switchover: %v", intfsOperStatusUPBeforeSwitch)
	if got, want := len(intfsOperStatusUPBeforeSwitch), 0; got == want {
		t.Errorf("Get the number of intfsOperStatusUP interfaces for %q: got %v, want > %v", tc.dut.ID(), got, want)
	}

	gnoiClient := tc.dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(tc.dut)
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
	if deviations.GNOISubcomponentPath(tc.dut) {
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
			currentTime = gnmi.Get(t, tc.dut, gnmi.OC().System().CurrentDatetime().State())
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

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, tc.dut, controllerCards)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	helpers.ValidateOperStatusUPIntfs(t, tc.dut, intfsOperStatusUPBeforeSwitch, 5*time.Minute)

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)

	swTime, swTimePresent := gnmi.Watch(t, tc.dut, activeRP.LastSwitchoverTime().State(), 1*time.Minute, func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }).Await(t)
	if !swTimePresent {
		t.Errorf("activeRP.LastSwitchoverTime().Watch(t).IsPresent(): got %v, want %v", false, true)
	} else {
		st, _ := swTime.Val()
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", st)
		// TODO: validate that last switchover time is correct
	}

	if got, want := gnmi.Lookup(t, tc.dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, tc.dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
}

func testOSForceInstall2(t *testing.T, tc testCase) {
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
		if t1 == 0 && isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})
	t.Run("Try Activating using Wrong version expected failure", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, noReboot, "WRONG_VERSION", true, imageNotAvailable)

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, noReboot, "WRONG_VERSION", true, imageNotAvailable)
		}
	})

	t.Run("Activating using correct version - expected success", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, noReboot, tc.osVersion, false, "")

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, noReboot, tc.osVersion, false, "")
		}
	})

	t.Run("Activating using correct version - expected failure already activated", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, noReboot, tc.osVersion, false, alreadyActivatedError)

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, noReboot, tc.osVersion, false, alreadyActivatedError)
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

func testOSForceInstall3(t *testing.T, tc testCase) {
	// if tc.osFile == *osFileForceDownloadSupported {
	// 	t.Skip()
	// }
	tc.fetchOsFileDetails(t, *osFile)

	t.Logf("Testing GNOI OS Install to version %s from file %q", tc.osVersion, tc.osFile)

	t.Run("Normal install back to original version", func(t *testing.T) {
		t1, _ := listISOFile(t, tc.dut, tc.osVersion)
		tc.transferOS(tc.ctx, t, false, tc.osVersion, "")
		t2, _ := listISOFile(t, tc.dut, tc.osVersion)
		if t1 == 0 && isGreater(t1, t2) {
			t.Fatal("image not force updated")
		}
	})

	t.Run("Activating using correct version - expected success", func(t *testing.T) {
		tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, false, "")

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, false, "")
		}
	})

	t.Run("Activating using correct version - expected failure already activated", func(t *testing.T) {
		alreadyActivatedError := "'Install' detected the 'warning' condition 'Apply atomic change in progress. Cannot accept further requests until complete'"
		tc.activateOS(tc.ctx, t, false, tc.noReboot, tc.osVersion, true, alreadyActivatedError)

		if deviations.InstallOSForStandbyRP(tc.dut) && tc.dualSup {
			tc.transferOS(tc.ctx, t, true, "", "")
			tc.activateOS(tc.ctx, t, true, tc.noReboot, tc.osVersion, true, alreadyActivatedError)
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
