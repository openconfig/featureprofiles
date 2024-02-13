package metadata

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestInit(t *testing.T) {
	want := &mpb.Metadata{
		Uuid:        "TestProperties123",
		PlanId:      "TestProperties",
		Description: "TestProperties unit test",
		Testbed:     mpb.Metadata_TESTBED_DUT,
	}
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(Get(), want, protocmp.Transform()); diff != "" {
		t.Errorf("Init() got unexpected metadata diff: %s", diff)
	}
}
