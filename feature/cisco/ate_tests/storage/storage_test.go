package storage_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestStorageFileSystemCheck validates OpenConfig storage counter functionality
// Tests include: soft-read-error-rate, reallocated-sectors, end-to-end-error,
// offline-uncorrectable-sectors-count, life-left, percentage-used, and system-events
func TestStorageFileSystemCheck(t *testing.T) {

	// write log message for "MDT/EDT support for storage IO error leafs in openconfig"
	t.Log("Description: MDT/EDT support for storage IO error leafs in openconfig")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	configureDUT(t, dut)
	ate := ondatra.ATE(t, "ate")

	args := &testArgs{
		dut: dut,
		ate: ate,
		ctx: ctx,
	}

	// Storage counter test cases with different subscription modes and GET requests
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
		{
			name:        "system-events",
			counterType: "counter64",
			description: "Validate storage system events counter",
			fn:          testStorageSystemEvents,
		},
		{
			name:        "trigger-events",
			counterType: "counter64",
			description: "Validate storage trigger events counter",
			fn:          testCounterTriggerScenario,
		},
		{
			name:        "subscription-levels",
			counterType: "mixed",
			description: "Validate storage subscriptions at root, container, and leaf levels",
			fn:          testStorageSubscriptionLevels,
		},
	}

	// Execute all test cases
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(t, args, tt.path)
		})
	}
	executeCLICommands(t, dut, ctx)
}

// testSoftReadErrorRate validates soft read error rate counters across all subscription modes
// ctx is required by the function signature but not used in this implementation
func testSoftReadErrorRate(t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-target-defined", func(t *testing.T) {
		testStorageCounterTargetMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-on-change", func(t *testing.T) {
		testStorageCounterOnChangeMode(t, args, pathSuffix)
	})

	t.Run("gnmi-get-request", func(t *testing.T) {
		testStorageCounterGetMode(t, args, pathSuffix)
	})

}

// testReallocatedSectors validates reallocated sectors counters across all subscription modes
func testReallocatedSectors(t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-target-defined", func(t *testing.T) {
		testStorageCounterTargetMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-on-change", func(t *testing.T) {
		testStorageCounterOnChangeMode(t, args, pathSuffix)
	})

	t.Run("gnmi-get-request", func(t *testing.T) {
		testStorageCounterGetMode(t, args, pathSuffix)
	})

}

// testEndToEndError validates end-to-end error counters across all subscription modes
func testEndToEndError(t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-target-defined", func(t *testing.T) {
		testStorageCounterTargetMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-on-change", func(t *testing.T) {
		testStorageCounterOnChangeMode(t, args, pathSuffix)
	})

	t.Run("gnmi-get-request", func(t *testing.T) {
		testStorageCounterGetMode(t, args, pathSuffix)
	})

}

// testOfflineUncorrectableSectors validates offline uncorrectable sectors counters across all subscription modes
// ctx is required by the function signature but not used in this implementation
func testOfflineUncorrectableSectors(t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-target-defined", func(t *testing.T) {
		testStorageCounterTargetMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-on-change", func(t *testing.T) {
		testStorageCounterOnChangeMode(t, args, pathSuffix)
	})

	t.Run("gnmi-get-request", func(t *testing.T) {
		testStorageCounterGetMode(t, args, pathSuffix)
	})

}

// testLifeLeft validates storage life left percentage counters across all subscription modes
// ctx is required by the function signature but not used in this implementation
func testLifeLeft(t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-target-defined", func(t *testing.T) {
		testStorageCounterTargetMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-on-change", func(t *testing.T) {
		testStorageCounterOnChangeMode(t, args, pathSuffix)
	})

	t.Run("gnmi-get-request", func(t *testing.T) {
		testStorageCounterGetMode(t, args, pathSuffix)
	})

}

// testPercentageUsed validates storage percentage used counters across all subscription modes
func testPercentageUsed(t *testing.T, args *testArgs, pathSuffix string) {
	t.Run("subscription-mode-sample", func(t *testing.T) {
		testStorageCounterSampleMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-once", func(t *testing.T) {
		testStorageCounterOnceMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-target-defined", func(t *testing.T) {
		testStorageCounterTargetMode(t, args, pathSuffix)
	})

	t.Run("subscription-mode-on-change", func(t *testing.T) {
		testStorageCounterOnChangeMode(t, args, pathSuffix)
	})

	t.Run("gnmi-get-request", func(t *testing.T) {
		testStorageCounterGetMode(t, args, pathSuffix)
	})

}

func testStorageSystemEvents(t *testing.T, args *testArgs, path string) {
	t.Log("Description: System Events Test - Validate all storage counters before and after system events")

	t.Run("comprehensive-system-events-test", func(t *testing.T) {
		testStorageSystemEventsComprehensive(t, args)
	})
}

func testCounterTriggerScenario(t *testing.T, args *testArgs, path string) {
	t.Log("Description: Counter Trigger Scenario - Validate storage counters on specific triggers")

	t.Run("trigger scenario", func(t *testing.T) {
		testStorageCounterTriggerScenario(t, args, args.ctx, path)
	})
}

// testStorageSubscriptionLevels validates storage subscriptions at different hierarchy levels
// ctx is required by the function signature but not used in this implementation
func testStorageSubscriptionLevels(t *testing.T, args *testArgs, path string) {
	t.Log("Description: Validate storage subscriptions at root, container, and leaf levels")

	// Define the storage counter leafs
	storageCounterLeafs := []struct {
		name        string
		counterType string
		description string
	}{
		{
			name:        "soft-read-error-rate",
			counterType: "counter64",
			description: "Soft read error rate counter",
		},
		{
			name:        "reallocated-sectors",
			counterType: "counter64",
			description: "Reallocated sectors counter",
		},
		{
			name:        "end-to-end-error",
			counterType: "counter64",
			description: "End-to-end error counter",
		},
		{
			name:        "offline-uncorrectable-sectors-count",
			counterType: "counter64",
			description: "Offline uncorrectable sectors count",
		},
		{
			name:        "life-left",
			counterType: "integer",
			description: "Storage life left percentage",
		},
		{
			name:        "percentage-used",
			counterType: "integer",
			description: "Storage percentage used",
		},
	}

	// Test 1: Root level subscription
	t.Run("root-level-subscription", func(t *testing.T) {
		t.Log("=== ROOT LEVEL SUBSCRIPTION TEST ===")
		t.Log("Testing subscription to: /components")
		testRootLevelSubscription(t, args)
	})

	// Test 2: Container level subscriptions
	t.Run("container-level-subscriptions", func(t *testing.T) {
		t.Log("=== CONTAINER LEVEL SUBSCRIPTION TESTS ===")

		// Test each container level in the hierarchy
		containerLevels := []struct {
			name        string
			path        string
			description string
		}{
			{
				name:        "component-level",
				path:        "component",
				description: "Testing subscription to: /components/component[name=*]",
			},
			{
				name:        "counters-level",
				path:        "storage/state/counters",
				description: "Testing subscription to: /components/component[name=*]/storage/state/counters",
			},
		}

		for _, containerLevel := range containerLevels {
			t.Run(containerLevel.name, func(t *testing.T) {
				t.Log(containerLevel.description)
				testContainerLevelSubscription(t, args, containerLevel.path, containerLevel.description)
			})
		}
	})

	// Test 3: Leaf level subscriptions
	t.Run("leaf-level-subscriptions", func(t *testing.T) {
		t.Log("=== LEAF LEVEL SUBSCRIPTION TESTS ===")

		for _, leaf := range storageCounterLeafs {
			t.Run(leaf.name, func(t *testing.T) {
				leafPath := "storage/state/counters/" + leaf.name
				description := "Testing subscription to: /components/component[name=*]/" + leafPath
				t.Log(description)
				testLeafLevelSubscription(t, args, leafPath, leaf.name, leaf.counterType, description)
			})
		}
	})

	// Test 4: Comparative analysis across levels
	t.Run("comparative-level-analysis", func(t *testing.T) {
		t.Log("=== COMPARATIVE LEVEL ANALYSIS ===")
		t.Log("Comparing data consistency across subscription levels")
		testComparativeLevelAnalysis(t, args, storageCounterLeafs)
	})
}
