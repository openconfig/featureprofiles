package yang_coverage

import (
	"context"
	"strings"
	"testing"
	"fmt"

	"github.com/openconfig/featureprofiles/proto/cisco/ycov"
	"github.com/openconfig/ondatra/eventlis"
)

var dut = "dut"
var yangCovCtx *yCov

/*
 * Support for XR Yang Coverage gathering and reporting through go test
Usage:
- Create YCov object
- Call add_yang_coverage once tests are setup (add pre/post YCov tests)
  - The YCov collect will post process the data, generate a report
    and log the results as required by the mgbl team
 */

type yCov struct {
	sanityName string 
	prefixPaths string
	ws string
	verbose bool	
	yc *YangCoverage
}

// Helps instantiate Yang-coverage 
func CreateInstance(sanityName string, models []string, prefixPaths []string, ws string, event eventlis.EventListener, verbose ...bool) error {
	var err error
	ycObj := &yCov{
		sanityName: sanityName,
		ws: ws,
	}
	if len(prefixPaths) != 0 {
		ycObj.prefixPaths = strings.Join(prefixPaths, ",")
	}
	if len(verbose) > 0 {
		ycObj.verbose = verbose[0]
	}
	ycObj.yc, err = New(ws, models, sanityName, ycov.TestPhase_IT, ycov.TestType_PRECOMMIT)
	if err != nil {
		return  err
	}
	event.AddBeforeTestsCallback(PreTest)
	event.AddAfterTestsCallback(PostTest)
	yangCovCtx = ycObj
	return nil
}

func PreTest(a *eventlis.BeforeTestsEvent) error {
	t := new(testing.T)
	ctx := context.Background()
	if yobj := getYCovCtx(); yobj != nil {	
		yobj.yc.clearCovLogs(ctx, t)
		yobj.yc.enableCovLogs(ctx, t)
	}
	return nil
}

func PostTest(a *eventlis.AfterTestsEvent) error {
	t := new(testing.T)
	ctx := context.Background()
	if yobj := getYCovCtx(); yobj != nil {		
		yobj.processYCov(yobj.yc.collectCovLogs(ctx, t), t)
	}
	return nil
}

func (y *yCov) processYCov(logs string, t *testing.T) {
	rc, res := y.yc.generateReport(logs, "", y.verbose, y.prefixPaths)
	if rc != 0 {
		t.Errorf(res)
	} else {
		fmt.Println(res)
	}
}

func getYCovCtx() *yCov {
	return yangCovCtx
}