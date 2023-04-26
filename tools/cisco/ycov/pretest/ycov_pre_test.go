package ycov_pre_test

import (
	"context"

	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/tools/cisco/ycov"

)



func TestMain(m *testing.M) {
	fptest.RunTests(m)
}


func TestPreCov(t *testing.T)  {
	ctx := context.Background()
	if err:=ycov.CreateInstance(); err!= nil {
		t.Fatalf("Initialization of yang coverage is failed: %v",err)
	}
	if yobj := ycov.GetYCovCtx(); yobj != nil {
		err := yobj.YC.ClearCovLogs(ctx, t)
		if err != nil {
			t.Errorf("Error while clearing existing coverage logs: %v",err.Error())
		}
		yobj.YC.EnableCovLogs(ctx, t)
		t.Log("Yang Coverage is enabled successfully")
	} else {
		t.Error("Yang Coverage Initialization is failed")
	}
}

