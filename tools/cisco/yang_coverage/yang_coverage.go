package yang_coverage

import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "io"
    "os"
    "os/exec"
    "strings"
    "testing"
    "text/template"
    "time"

    "github.com/openconfig/featureprofiles/internal/cisco/util"
    "github.com/openconfig/featureprofiles/proto/cisco/ycov"
    "github.com/openconfig/ondatra"
)

var MGBL_PATH = "/ws/ncorran-sjc/yang-coverage/"

/*
 * YangCoverage provides the services to support collecting and
 * reporting XR Yang Model Coverage.
 * It provides services to supply RPCs to send to the device to
 * - clear, enable and collect logging
 * It provides services to run the coverage tooling and save
 *the results.
 */
type YangCoverage struct {
    testName  string
    testPhase ycov.TestPhase
    testType  ycov.TestType
    ws    string
    models    string
}

/*
 * Initialise with parameters needed to
 * - pass through to the pyang yang_coverage tools
 */
func New(ws string, models []string, testName string, testPhase ycov.TestPhase, testType ycov.TestType) (*YangCoverage, error) {
    var err error
    yc := &YangCoverage{
    	testName:  testName,
    	testPhase: testPhase,
    	testType:  testType,
    	ws:    ws,
    }
    if len(models) != 0 {
    	err = yc.isValidModel(models)
    }
    return yc, err
}

// Validate models - for existence, then store in the form needed for the tools
func (yc *YangCoverage) isValidModel(models []string) error {
    if len(models) == 0 {
    	return errors.New("Dependent yang models not provided!!")
    }
    for _, item := range models {
    	_, err := os.Stat(item)
    	if err != nil {
    		if errors.Is(err, os.ErrNotExist) {
    			return errors.New("Yang model " + item + " is missing!!")
    		}
    		return err
    	}
    }
    yc.models = strings.Join(models, " ")
    return nil
}

// Send clear logs request using GNOI client
func (yc *YangCoverage) clearCovLogs(ctx context.Context, t *testing.T) error {
    yclient, err := GetYcovClient(dut, t)
    if err != nil {
    	return fmt.Errorf("clearCovLogs Yclient creation Failed - %s", err.Error())
    }
    _, err = yclient.ClearLogs(ctx, &ycov.ClearLogsRequest{})
    if err != nil {
    	return fmt.Errorf("clearCovLogs Req Failed - %s", err.Error())
    }
    return nil
}

// Send enable logs request using GNMI client
func (yc *YangCoverage) enableCovLogs(ctx context.Context, t *testing.T) {
    dut := ondatra.DUT(t, dut)
    config := "aaa accounting commands default start-stop local \n"
    util.GNMIWithText(ctx, t, dut, config)
}

// Send gather yang coverage logs using GNOI client
func (yc *YangCoverage) collectCovLogs(ctx context.Context, t *testing.T) (string, error) {
    yclient, err := GetYcovClient(dut, t)
    if err != nil {
    	return "", fmt.Errorf("collectCovLogs Yclient creation Failed - %s", err.Error())
    }
    req := &ycov.GatherLogsRequest{
    	TestName:  yc.testName,
    	TestPhase: yc.testPhase,
    	TestType:  yc.testType}
    rsp, err := yclient.GatherLogs(ctx, req)
    if err != nil {
    	return "", fmt.Errorf("collectCovLogs Req Failed - %s", err.Error())
    }
    return rsp.GetLog(), nil
}

// Run the pyang validate and report steps, gathering the results
func (yc *YangCoverage) _generateReport(logFile, outFname string, verbose bool, prefixPaths string) (int, string) {
    var ppaths, vmode string
    //  Setup the files we are manipulating
    //  - validated_logs: validated output from input ycov logs + models + prefix_paths
    //  - report_outfile: report result file from processing validated_logs
    //  - ycov_logfile:   file collecting the output of running the tools
    pathPrefix := fmt.Sprintf("%s/%s", yc.ws, outFname)
    destPath := fmt.Sprintf("%s/%s_validated.json", MGBL_PATH, outFname)
    validatedLogs := fmt.Sprintf("%s_validated.json", pathPrefix)
    reportOutFile := fmt.Sprintf("%s_report.json", pathPrefix)
    ycovLogFile := fmt.Sprintf("%s_ycov.log", pathPrefix)

    //  Setup the required context for running the mgbl coverage tools as per:
    //  https://wiki.cisco.com/display/XRMGBLMOVE/Yang+Data-Model+Coverage
    //  - Setup prefix_paths and verbose_mode options if needed
    if prefixPaths != "" {
    	ppaths = fmt.Sprintf("--prefix-path=%s", prefixPaths)
    }

    if verbose {
    	vmode = "--verbose-mode"
    }
    // - Setup basic pyang invocation, extended with additional options
    pyangcmd := fmt.Sprintf("pyang -p %s/manageability/yang/pyang/modules -f yang_coverage %s %s", yc.ws, vmode, ppaths)

    data := fdata{
    	"ws":         yc.ws,
    	"pyangcmd":       pyangcmd,
    	"log_file":       logFile,
    	"validated_logs": validatedLogs,
    	"models":     yc.models,
    	"report_outfile": reportOutFile,
    }
    // - Setup the full interactions with the tools using the mgbl python and pyang env
    coverageScript, err := fstring(`
    source {{.ws}}/manageability/yang/bin/xr_mgbl_pywrap.sh;
    cd /auto/mgbl/xr-yang-scripts/pyang;
    source env.sh;
    cd /nobackup/$USER;
    rm -rf .venv/;
    echo $(get_python_exec)
    $(get_python_exec) -m venv --system-site-packages .venv/;
    source .venv/bin/activate;
    cd {{.ws}};
    {{.pyangcmd}} --validate --log-files {{.log_file}}  --output-file {{.validated_logs}} {{.models}} 2>&1;
    {{.pyangcmd}} -f yang_coverage --report --log-files {{.validated_logs}} --output-file {{.report_outfile}} {{.models}}  2>&1;
    deactivate
    `, data)
    cmd := exec.Command("bash", "-c", coverageScript)
    logp, err := os.Create(ycovLogFile)
    if err != nil {
    	return -1, fmt.Sprintf("File creation failed: %s", err.Error())
    }
    defer logp.Close()

    cmd.Stdout = logp
    err = cmd.Start()
    if err != nil {
    	return -1, err.Error()
    }
    cmd.Wait()

    if err = copy(validatedLogs, destPath); err != nil {
    	return -1, fmt.Sprintf("WARNING: Coverage logs copy to %s failed: %s \n YCov tool logs at %s \n Please run manually: cp %s %s to add your logs to the collection", MGBL_PATH, err.Error(), ycovLogFile, validatedLogs, MGBL_PATH)
    }

    return 0, fmt.Sprintf("Coverage logs stored at %s/%s_validated.json\nYCov tool logs at %s", MGBL_PATH, outFname, ycovLogFile)
}

// Run the pyang validate and report steps, gathering the results
func (yc *YangCoverage) generateReport(rawYcovLogs, outfile string, verboseMode bool, prefixPaths, subCompId string) (int, string) {
    /* Run the pyang validate and report steps, gathering the results */
    if rawYcovLogs == "" {
    	return -1, "Coverage logs are empty!!"
    }

    // - Save the logs to file
    logFile := fmt.Sprintf("%s/collected_ycov_logs.json", yc.ws)

    f, err := os.Create(logFile)
    if err != nil {
    	return -1, err.Error()
    }

    defer f.Close()

    _, err = f.WriteString(rawYcovLogs)
    if err != nil {
    	return -1, err.Error()
    }

    // Setup the file for the final report
    if outfile == "" {
    	t := time.Now()
    	dt := fmt.Sprintf("%d_%02d_%02d__%02d_%02d_%02d",
    		t.Year(), t.Month(), t.Day(),
    		t.Hour(), t.Minute(), t.Second())
    	if subCompId != "" {
    		outfile = fmt.Sprintf("%s_%s_%s_%s", dt, yc.testName, subCompId, yc.testType.String())
    	} else {
    		outfile = fmt.Sprintf("%s_%s_%s", dt, yc.testName, yc.testType.String())
    	}
    } else {
    	outfile = strings.Replace(outfile, ".json", "", -1)
    }

    // Call the internal generate worker
    return yc._generateReport(logFile, outfile, verboseMode, prefixPaths)
}

/*
 * HELPER Functions
 */

type fdata map[string]interface{}

func fstring(format string, data fdata) (string, error) {
    t, err := template.New("fstring").Parse(format)
    if err != nil {
    	return "", fmt.Errorf("error creating template: %v", err)
    }
    output := new(bytes.Buffer)
    if err := t.Execute(output, data); err != nil {
    	return "", fmt.Errorf("error executing template: %v", err)
    }
    return output.String(), nil
}

func copy(src, dst string) error {
    srcp, err := os.Open(src)
    if err != nil {
    	return err
    }
    defer srcp.Close()

    // Create new file
    dstp, err := os.Create(dst)
    if err != nil {
    	return err
    }
    defer dstp.Close()

    //This will copy
    _, err = io.Copy(dstp, srcp)
    if err != nil {
    	return err
    }
    return nil
}
