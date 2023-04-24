package yang_coverage

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/proto/cisco/ycov"
	"github.com/openconfig/ondatra/eventlis"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	dut        string
	yangCovCtx *yCov
	ycovFile   = flag.String("yang_coverage", "", "yang coverage configuration file")
	xrWs       = flag.String("xr_ws", "", "XR workspace path")
	event      = eventlis.EventListener{}
)

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
	ws         string
	verbose    bool
	processLog bool
	subCompId  string
	yc         *YangCoverage
}

// Initialize Yang Coverage Package
// Param: subComp - Sub-component name to be used
// as identifier in filename
func Init(subComp string) {
	err := CreateInstance(subComp)
	if err != nil {
		log.Warning(err.Error())
	}
}

// Helps instantiate Yang-coverage
func CreateInstance(subComp string) error {
	var models []string
	var ws, prefixPaths string
	if !flag.Parsed() {
		flag.Parse()
		flag.Set("test.v", "true")
	}

	// ycovFile is yang-coverage configuration file name.
	// passed using -yang_coverage option.
	// This contains necessary parameters related to dut-id,
	// test-phase, test-type, xr ws and log processing details
	if *ycovFile != "" {
		in, err := os.ReadFile(*ycovFile)
		if err != nil {
			return fmt.Errorf("unable to read yang_coverage file: %w", err)
		}
		// Unmarshal yang coverage config details into
		// Metadata struct of metadata.proto
		meta := &ycov.Metadata{}
		if err = prototext.Unmarshal(in, meta); err != nil {
			return fmt.Errorf("unable to parse yang_coverage file: %w", err)
		}
		ycObj := &yCov{
			sanityName: meta.SanityName,
			subCompId:  subComp,
		}
		dut = meta.DutId
		tphase := meta.TestPhase
		ttype := meta.TestType

		if meta.Options != nil && *xrWs != "" {
			opts := meta.Options
			// Checks if XR workspace path is provided/accessible.
			// If path is accessible then processing of logs is activated
			// and configured options are considered.
			// Else we skip processing and store raw logs.
			if rc, _ := pathExists(*xrWs); !rc {
				log.Warning("Xr Workspace path is not provided or inaccessible." +
					"Processing of logs will be skipped!")
			} else {
				ycObj.processLog = true
				ws = *xrWs
				ycObj.ws = ws
				ycObj.verbose = opts.Verbose
				if len(opts.PrefixPaths) != 0 {
					prefixPaths = strings.Join(opts.PrefixPaths, ",")
				}
				models = opts.Models
			}
		}

		ycObj.yc, err = New(ws, models, meta.SanityName,
			tphase, ttype, prefixPaths,
			ycObj.verbose, subComp)
		if err != nil {
			return err
		}
		event.AddBeforeTestsCallback(PreTest)
		event.AddAfterTestsCallback(PostTest)
		yangCovCtx = ycObj
		log.Info("Yang Coverage Enabled!!")
	}
	return nil
}

func PreTest(a *eventlis.BeforeTestsEvent) error {
	t := new(testing.T)
	ctx := context.Background()
	if yobj := getYCovCtx(); yobj != nil {
		err := yobj.yc.clearCovLogs(ctx, t)
		if err != nil {
			log.Warning(err.Error())
		}
		yobj.yc.enableCovLogs(ctx, t)
	}
	return nil
}

func PostTest(a *eventlis.AfterTestsEvent) error {
	var rc int
	var res string
	t := new(testing.T)
	ctx := context.Background()
	if yobj := getYCovCtx(); yobj != nil {
		logs, err := yobj.yc.collectCovLogs(ctx, t)
		if err != nil {
			log.Warning(err.Error())
			return nil
		}
		if yobj.processLog {
			rc, res = yobj.processYCov(logs, t)
		} else {
			rc, res = yobj.yc.storeRawLogs(logs)
		}
	}
	if rc != 0 {
		log.Warning(res)
	} else {
		log.Info(res)
	}
	return nil
}

func (y *yCov) processYCov(logs string, t *testing.T) (int, string) {
	return y.yc.generateReport(logs)

}

func getYCovCtx() *yCov {
	return yangCovCtx
}
