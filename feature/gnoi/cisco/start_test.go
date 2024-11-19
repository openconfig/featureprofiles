package osinstall_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
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
			name: "testOSTransferDiskFull",
			desc: "testOSTransferDiskFull",
			fn:   testOSTransferDiskFull,
		},
		{
			name: "testOSForceTransferStandby",
			desc: "testOSForceTransferStandby",
			fn:   testOSForceTransferStandby,
		},
		{
			name: "testOSNormalTransferStandby",
			desc: "testOSNormalTransferStandby",
			fn:   testOSNormalTransferStandby,
		},
		{
			name: "testOSForceTransfer1",
			desc: "testOSForceTransfer1",
			fn:   testOSForceTransfer1,
		},
		{
			name: "testOSForceTransfer2",
			desc: "testOSForceTransfer2",
			fn:   testOSForceTransfer2,
		},
		{
			name: "testOSNormalTransfer1",
			desc: "testOSNormalTransfer1",
			fn:   testOSNormalTransfer1,
		},
		{
			name: "testOSNormalTransfer2",
			desc: "testOSNormalTransfer2",
			fn:   testOSNormalTransfer2,
		},
		{
			name: "testOSForceTransfer3",
			desc: "testOSForceTransfer3",
			fn:   testOSForceTransfer3,
		},
		{
			name: "testOSForceInstall1",
			desc: "testOSForceInstall1",
			fn:   testOSForceInstall1,
		},
		{
			name: "testOSForceInstall2",
			desc: "testOSForceInstall2",
			fn:   testOSForceInstall2,
		},
		{
			name: "testOSForceInstall3",
			desc: "testOSForceInstall3",
			fn:   testOSForceInstall3,
		},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOSInstall(t *testing.T) {

	// Process the binding file to get all devices
	b, err := processBindingFile(t)
	if err != nil {
		t.Fatalf("Failed to process binding file: %v", err)
	}

	testCases := []testCase{}

	for _, device := range b.Duts {
		dutID := device.Id
		dut := ondatra.DUT(t, dutID)
		tc1 := testCase{
			dut:      dut,
			osc:      dut.RawAPIs().GNOI(t).OS(),
			sc:       dut.RawAPIs().GNOI(t).System(),
			ctx:      context.Background(),
			noReboot: deviations.OSActivateNoReboot(dut),
		}
		tc1.fetchStandbySupervisorStatus(t)
		tc1.fetchOsFileDetails(t, *osFileForceDownloadSupported)
		tc1.updatePackageReader(t)
		tc1.setTimeout(t, *timeout)
		tc1.updateForceDownloadSupport(t)
		testCases = append(testCases, tc1)
	}

	var wg sync.WaitGroup
	for _, tc := range testCases {
		// Increment the WaitGroup counter
		wg.Add(1)
		// Launch a goroutine for each device
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
		t.Run(fmt.Sprintf("%v:%v", tc.dut.Name(), tf.name), func(t *testing.T) {
			t.Logf("Name: %s", tf.name)
			t.Logf("Description: %s", tf.desc)
			tf.fn(t, tc)
		})
	}
}
