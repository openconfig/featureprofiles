package metadata_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/rundata"
)

func TestMetadata(t *testing.T) {
	got := rundata.Properties(context.Background(), nil)
	t.Log("rundata.Properties:", got)
	want := map[string]string{
		"test.uuid":        "123e4567-e89b-42d3-8456-426614174000",
		"test.plan_id":     "UnitTest-1.1",
		"test.description": "Metadata Unit Test",
	}
	for k := range want {
		if got[k] != want[k] {
			t.Errorf("rundata.Properties[%q] got %q, want %q", k, got[k], want[k])
		}
	}
}
