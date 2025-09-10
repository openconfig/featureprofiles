// Package util_test provides integration and helper tests for the util package.
// The helpers intentionally live in a test file so they do not become part of the exported runtime API.
package util

import "testing"

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
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("FirstOrFatal() did not fail on empty slice")
		}
	}()
	_ = FirstOrFatal(t, []int{}, "empty slice")
}

// TODO: add similar unit-tests for relevant functions in util.go
