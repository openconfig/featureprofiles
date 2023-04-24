package yang_coverage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/proto/cisco/ycov"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc"
)

var mgblPath = "/ws/ncorran-sjc/yang-coverage/"
var rawLogsPath = "/ws/ncorran-sjc/yang-coverage/rawlogs/"

/*
 * YangCoverage provides the services to support collecting and
 * reporting XR Yang Model Coverage.
 * It provides services to supply RPCs to send to the device to
 * - clear, enable and collect logging
 * It provides services to run the coverage tooling and save
 *the results.
 */
type YangCoverage struct {
	testName         string
	testPhase        ycov.TestPhase
	testType         ycov.TestType
	ws               string
	models           string
	prefixPaths      string
	verbose          bool
	logFile          string
	subCompId        string
	srcValidLogPath  string
	destValidLogPath string
	ycovLogPath      string
	coverageScript   string
}

/*
 * Initialise with parameters needed to
 * - pass through to the pyang yang_coverage tools
 */
func New(ws string, models []string, testName string,
	testPhase ycov.TestPhase, testType ycov.TestType,
	prefixPaths string, verbose bool, subCompId string) (*YangCoverage, error) {
	var err error
	yc := &YangCoverage{
		testName:    testName,
		testPhase:   testPhase,
		testType:    testType,
		ws:          ws,
		prefixPaths: prefixPaths,
		verbose:     verbose,
		subCompId:   subCompId,
	}
	if len(models) != 0 {
		err = yc.isValidModel(models)
	}
	if ws != "" {
		yc.logFile = fmt.Sprintf("%s/collected_ycov_logs.json", yc.ws)
		yc.coverageScript, err = yc.setupCoverageScript(yc.logFile, yc.getOutFname())
	}
	return yc, err
}

func (yc *YangCoverage) setupCoverageScript(logFile, outFname string) (coverageScript string, err error) {
	var ppaths, vmode string

	// Setup the file for the final report
	if outFname == "" {
		outFname = yc.getOutFname()
	} else {
		outFname = strings.Replace(outFname, ".json", "", -1)
	}

	//  Setup the files we are manipulating
	//  - validated_logs: validated output from input ycov logs + models + prefix_paths
	//  - report_outfile: report result file from processing validated_logs
	//  - ycov_logfile:   file collecting the output of running the tools
	pathPrefix := fmt.Sprintf("%s/%s", yc.ws, outFname)
	yc.destValidLogPath = fmt.Sprintf("%s/%s_validated.json", mgblPath, outFname)
	yc.srcValidLogPath = fmt.Sprintf("%s_validated.json", pathPrefix)
	reportOutFile := fmt.Sprintf("%s_report.json", pathPrefix)
	yc.ycovLogPath = fmt.Sprintf("%s_ycov.log", pathPrefix)

	//  Setup the required context for running the mgbl coverage tools as per:
	//  https://wiki.cisco.com/display/XRMGBLMOVE/Yang+Data-Model+Coverage
	//  - Setup prefix_paths and verbose_mode options if needed
	if yc.prefixPaths != "" {
		ppaths = fmt.Sprintf("--prefix-path=%s", yc.prefixPaths)
	}

	if yc.verbose {
		vmode = "--verbose-mode"
	}
	// - Setup basic pyang invocation, extended with additional options
	pyangcmd := fmt.Sprintf("pyang -p %s/manageability/yang/pyang/modules -f yang_coverage %s %s", yc.ws, vmode, ppaths)

	data := fdata{
		"ws":             yc.ws,
		"pyangcmd":       pyangcmd,
		"log_file":       logFile,
		"validated_logs": yc.srcValidLogPath,
		"models":         yc.models,
		"report_outfile": reportOutFile,
	}
	// - Setup the full interactions with the tools using the mgbl python and pyang env
	coverageScript, err = fstring(`
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

	return
}

// Validate models - for existence, then store in the form needed for the tools
func (yc *YangCoverage) isValidModel(models []string) error {
	if len(models) == 0 {
		return errors.New("Dependent yang models not provided!!")
	}
	for _, item := range models {
		if rc, err := pathExists(item); !rc {
			return err
		}
	}
	yc.models = strings.Join(models, " ")
	return nil
}

// Get output file name prefix
func (yc *YangCoverage) getOutFname() (outfile string) {
	t := time.Now()
	dnt := fmt.Sprintf("%d_%02d_%02d__%02d_%02d_%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
	if yc.subCompId != "" {
		outfile = fmt.Sprintf("%s_%s_%s_%s", dnt, yc.testName, yc.subCompId, yc.testType.String())
	} else {
		outfile = fmt.Sprintf("%s_%s_%s", dnt, yc.testName, yc.testType.String())
	}
	return
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
func (yc *YangCoverage) generateReport(rawLogs string) (int, string) {
	/* Run the pyang validate and report steps, gathering the results */

	// Save logs to file
	rc, errstr := writeLogsToFile(rawLogs, yc.logFile)
	if rc != 0 {
		return rc, errstr
	}

	// Execute coverage script
	cmd := exec.Command("bash", "-c", yc.coverageScript)
	logp, err := os.Create(yc.ycovLogPath)
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

	// Copy validated logfile to mgbl path
	if err = copy(yc.srcValidLogPath, yc.destValidLogPath); err != nil {
		return -1, fmt.Sprintf("WARNING: Coverage logs copy to %s failed: %s \n YCov tool logs at %s \n Please run manually: cp %s %s to add your logs to the collection", mgblPath, err.Error(), yc.ycovLogPath, yc.srcValidLogPath, mgblPath)
	}

	return 0, fmt.Sprintf("Coverage logs stored at %s.\nYCov tool logs at %s", yc.destValidLogPath, yc.ycovLogPath)
}

// Stores the raw logs in case processing is not activated.
func (yc *YangCoverage) storeRawLogs(logs string) (int, string) {
	var outfile string
	outfile = fmt.Sprintf("%s.json", yc.getOutFname())
	destPath := fmt.Sprintf("%s/%s", rawLogsPath, outfile)

	// Save logs to file
	rc, errstr := writeLogsToFile(logs, outfile)
	if rc != 0 {
		return rc, errstr
	}

	// Copy log file to dest path
	if err := copy(outfile, destPath); err != nil {
		return -1, fmt.Sprintf("Copy of log file %s failed to %s: %s", outfile, rawLogsPath, err.Error())
	}
	return 0, fmt.Sprintf("Raw log file at %s", destPath)
}

func GetYcovClient(dutId string, t *testing.T) (ycov.YangCoverageClient, error) {
	dut := ondatra.DUT(t, dutId)
	gnoiConn := dut.RawAPIs().GNOI().New(t)
	gc := reflect.ValueOf(gnoiConn)
	gconn := reflect.New(gc.Type()).Elem()
	gconn.Set(gc)
	conn := gconn.FieldByName("conn")
	clientConn, ok := (reflect.NewAt(conn.Type(), unsafe.Pointer(conn.UnsafeAddr())).Elem().Interface()).(*grpc.ClientConn)
	if !ok {
		return nil, errors.New("GNOI Client connection failed.")
	}
	return ycov.NewYangCoverageClient(clientConn), nil
}
