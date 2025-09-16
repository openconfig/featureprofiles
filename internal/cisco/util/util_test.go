// Package util_test provides integration and helper tests for the util package.
// The helpers intentionally live in a test file so they do not become part of the exported runtime API.
package util

import (
	"testing"

	"github.com/openconfig/testt"
)

func TestFirstOrFatalReturnsFirstElement(t *testing.T) {
	t.Parallel()
	xs := []int{42, 7, 19}
	got := FirstOrFatal(t, xs, "non-empty slice")
	if got != 42 {
		t.Errorf("FirstOrFatal() = %v, want %v", got, 42)
	}
}

func TestFirstOrFatalFailsOnEmptySlice(t *testing.T) {
	t.Parallel()
	if errMsg := testt.CaptureFatal(t, func(tb testing.TB) {
		FirstOrFatal(t, []int{}, "empty slice")
	}); errMsg != nil {
		t.Fatalf("received error %v", errMsg)
	} else {
		t.Fatal("Did not receive expected failure")
	}
}

// TODO: add similar unit-tests for relevant functions in util.go
