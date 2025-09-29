package storage_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	dst                   = "202.1.0.1"
	v4mask                = "32"
	dstCount              = 1
	totalBgpPfx           = 1
	minInnerDstPrefixBgp  = "202.1.0.1"
	totalIsisPrefix       = 1 //set value for scale isis setup ex: 10000
	minInnerDstPrefixIsis = "201.1.0.1"
	policyTypeIsis        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	dutAreaAddress        = "47.0001"
	dutSysId              = "0000.0000.0001"
	isisName              = "osisis"
	policyTypeBgp         = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	bgpAs                 = 65000
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestStorageFileSystemCheck(t *testing.T) {

	// write log message for "MDT/EDT support for storage IO error leafs in openconfig"
	t.Log("Description: MDT/EDT support for storage IO error leafs in openconfig")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	configureDUT(t, dut)
	ate := ondatra.ATE(t, "ate")
	//top := configureATE(t, ate)

	args := &testArgs{
		dut: dut,
		ate: ate,
		//top: top,
		ctx: ctx,
	}

	// Define test cases based on the test plan
	testCases := []storageTestCase{
		{
			name:        "soft-read-error-rate",
			path:        "storage/state/counters/soft-read-error-rate",
			counterType: "counter64",
			description: "Validate soft read error rate counter",
			fn:          testSoftReadErrorRate,
		},
		{
			name:        "reallocated-sectors",
			path:        "storage/state/counters/reallocated-sectors",
			counterType: "counter64",
			description: "Validate reallocated sectors counter",
			fn:          testReallocatedSectors,
		},
		{
			name:        "end-to-end-error",
			path:        "storage/state/counters/end-to-end-error",
			counterType: "counter64",
			description: "Validate end-to-end error counter",
			fn:          testEndToEndError,
		},
		{
			name:        "offline-uncorrectable-sectors-count",
			path:        "storage/state/counters/offline-uncorrectable-sectors-count",
			counterType: "counter64",
			description: "Validate offline uncorrectable sectors count",
			fn:          testOfflineUncorrectableSectors,
		},
		{
			name:        "life-left",
			path:        "storage/state/counters/life-left",
			counterType: "integer",
			description: "Validate storage life left percentage",
			fn:          testLifeLeft,
		},
		{
			name:        "percentage-used",
			path:        "storage/state/counters/percentage-used",
			counterType: "integer",
			description: "Validate storage percentage used",
			fn:          testPercentageUsed,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(ctx, t, args, tt.path)
		})
	}

}

func testSoftReadErrorRate(ctx context.Context, t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})
	t.Run("system-events", func(t *testing.T) {
		testStorageCounterSystemEvents(t, args, ctx, pathSuffix)
	})

}

func testReallocatedSectors(ctx context.Context, t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})
	t.Run("system-events", func(t *testing.T) {
		testStorageCounterSystemEvents(t, args, ctx, pathSuffix)
	})

}

func testEndToEndError(ctx context.Context, t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})
	t.Run("system-events", func(t *testing.T) {
		testStorageCounterSystemEvents(t, args, ctx, pathSuffix)
	})

}

func testOfflineUncorrectableSectors(ctx context.Context, t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})
	t.Run("system-events", func(t *testing.T) {
		testStorageCounterSystemEvents(t, args, ctx, pathSuffix)
	})

}

func testLifeLeft(ctx context.Context, t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})
	t.Run("system-events", func(t *testing.T) {
		testStorageCounterSystemEvents(t, args, ctx, pathSuffix)
	})

}

func testPercentageUsed(ctx context.Context, t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})
	t.Run("system-events", func(t *testing.T) {
		testStorageCounterSystemEvents(t, args, ctx, pathSuffix)
	})

}
