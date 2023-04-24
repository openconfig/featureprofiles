package yang_post_test

import (
	"context"

	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/tools/cisco/ycov"


)



func TestMain(m *testing.M) {
	fptest.RunTests(m)
}


func TestPostCov(t *testing.T)  {
	ctx := context.Background()
	if yobj := ycov.GetYCovCtx(); yobj != nil {
		logs, err := yobj.YC.CollectCovLogs(ctx, t)
		if err != nil {
			t.Fatalf("Failure while collecting coverage: %v",err.Error())
		}
		rc, _ := yobj.ProcessYCov(logs); if rc != 0 {
			t.Errorf("RC is expected to be 0, but got %d", rc)
		}
	}
}


