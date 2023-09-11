package certz_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"

)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSimpleCertzGetProfile(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnsiC:= dut.RawAPIs().GNSI().Default(t)
	profiles, err:= gnsiC.Certz().GetProfileList(context.Background(),&certzpb.GetProfileListRequest{}); if err!=nil {
		t.Fatalf("Unexpected Error in getting profile list: %v", err)
	}
	t.Logf("Profile list get was successful, %v", profiles)
}