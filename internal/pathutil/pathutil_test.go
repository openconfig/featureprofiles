package pathutil

import (
	"testing"
)

func TestComputeRootPath(t *testing.T) {
	wd := "/a/b/featureprofiles/c/d"
	got, err := computeRootPath(wd)
	if err != nil {
		t.Fatalf("computeRootPath(%q) failed: %v", wd, err)
	}
	if want := "/a/b/featureprofiles"; got != want {
		t.Fatalf("computeRootPath(%q) got %q, want %q", wd, got, want)
	}
}

func TestComputeRootPathError(t *testing.T) {
	wd := "/a/b/c/d"
	if got, err := computeRootPath(wd); err == nil {
		t.Fatalf("computeRootPath(%q) got %q, want error %v", wd, got, err)
	}
}
