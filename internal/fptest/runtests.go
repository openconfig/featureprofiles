package fptest

import (
	"testing"

	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
)

// RunTests initializes the appropriate binding and runs the tests.
// It should be called from every featureprofiles tests like this:
//
//   package test
//
//   import "github.com/openconfig/featureprofiles/internal/fptest"
//
//   func TestMain(m *testing.M) {
//     fptest.RunTests(m)
//   }
//
func RunTests(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}
