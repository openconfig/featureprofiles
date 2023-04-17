package yang_coverage

import (
    "context"
    "errors"
    "flag"
    "fmt"
    "os"
    "strings"
    "testing"

    "github.com/openconfig/featureprofiles/proto/cisco/ycov"
    "github.com/openconfig/ondatra/eventlis"
    "google.golang.org/protobuf/encoding/prototext"
)

var (
    dut    string
    yangCovCtx *yCov
    ycovFile   = flag.String("yang_coverage", "", "yang coverage configuration file")
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
    sanityName  string
    prefixPaths string
    ws      string
    verbose     bool
    processLog  bool
    subCompId   string
    yc      *YangCoverage
}

// Initialize Yang Coverage Package
// Param: subComp - Sub-component name to be used
//
//    as identifier in filename
func Init(subComp string) {
    err := CreateInstance(subComp)
    if err != nil {
    	fmt.Printf("\n[WARNING]: %s\n", err.Error())
    }
}

// Helps instantiate Yang-coverage
func CreateInstance(subComp string) error {
    var models []string
    var ws string
    if !flag.Parsed() {
    	flag.Parse()
    	flag.Set("test.v", "true")
    }

    if *ycovFile != "" {
    	in, err := os.ReadFile(*ycovFile)
    	if err != nil {
    		return fmt.Errorf("unable to read yang_coverage file: %w", err)
    	}
    	meta := &ycov.Metadata{}
    	if err = prototext.Unmarshal(in, meta); err != nil {
    		return fmt.Errorf("unable to parse yang_coverage file: %w", err)
    	}
    	ycObj := &yCov{
    		sanityName: meta.SanityName,
    		subCompId:  subComp,
    	}
    	dut = meta.DutId
    	if meta.ProcessLog {
    		opts := meta.Options
    		if opts == nil {
    			return errors.New("process_log enabled but no data provided.")
    		}
    		ycObj.processLog = true
    		ws = opts.XrWs
    		ycObj.ws = ws
    		ycObj.verbose = opts.Verbose
    		if len(opts.PrefixPaths) != 0 {
    			ycObj.prefixPaths = strings.Join(opts.PrefixPaths, ",")
    		}
    		models = opts.Models
    	}

    	tphase := ycov.TestPhase(ycov.TestPhase_value[meta.TestPhase.String()])
    	ttype := ycov.TestType(ycov.TestType_value[meta.TestType.String()])

    	ycObj.yc, err = New(ws, models, meta.SanityName, tphase, ttype)
    	if err != nil {
    		return err
    	}
    	event.AddBeforeTestsCallback(PreTest)
    	event.AddAfterTestsCallback(PostTest)
    	yangCovCtx = ycObj
    	fmt.Println("Yang Coverage Enabled!!")
    }
    return nil
}

func PreTest(a *eventlis.BeforeTestsEvent) error {
    t := new(testing.T)
    ctx := context.Background()
    if yobj := getYCovCtx(); yobj != nil {
    	err := yobj.yc.clearCovLogs(ctx, t)
    	if err != nil {
    		fmt.Printf("\n[WARNING]: %s\n", err.Error())
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
    		fmt.Printf("\n[WARNING]: %s\n", err.Error())
    		return nil
    	}
    	if yobj.processLog {
    		rc, res = yobj.processYCov(logs, t)
    	} else {
    		rc, res = yobj.storeRawLogs(logs)
    	}
    }
    if rc != 0 {
    	fmt.Printf("\n[WARNING]: %s\n", res)
    } else {
    	fmt.Printf("\n[OUTPUT]: %s\n", res)
    }
    return nil
}
