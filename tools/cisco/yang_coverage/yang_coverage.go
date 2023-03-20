package yang_coverage

import (
	"strings"
	"os"
	"os/exec"
	"errors"
	"fmt"
	"time"
	"text/template"
	"bytes"
	"io"

)

var MGBL_PATH = "/ws/ncorran-sjc/yang-coverage/"

type TestPhase int

const (
	UT TestPhase = iota + 10
	IT
	DT 
	CVT 
	BIT 
	TB 
	SIT 
	ACCEPTANCE
)

type TestType int 

const (
	MANUAL TestType = iota + 10
	PRECOMMIT
	NIGHTLY
	OCCASIONAL
)

// String - Creating common behavior - give the type a String function
func (tp TestPhase) String() string {
	return [...]string{"UT", "IT", "DT", "CVT", "BIT", "TB", "SIT", "ACCEPTANCE"}[tp-1]
}


// String - Creating common behavior - give the type a String function
func (tt TestType) String() string {
	return [...]string{"MANUAL", "PRECOMMIT", "NIGHTLY", "OCCASIONAL"}[tt-1]
}

type YangCoverage struct {
	testName string
	testPhase TestPhase
	testType TestType
	ws string
	models string 
}

func New(ws string, models []string, testName string, testPhase TestPhase, testType TestType) (*YangCoverage, error) {
	yc := &YangCoverage{
		testName: testName,
		testPhase: testPhase,
		testType: testType,
		ws: ws,
	}
	err := yc.isValidModel(models)
	return yc, err
}

func (yc *YangCoverage) isValidModel(models []string) error {
	if (len(models) == 0) {
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

func (yc *YangCoverage) _generateReport(logFile, outFname string, verbose bool, prefixPaths string) (int, string) {
	var ppaths, vmode string
	/* Run the pyang validate and report steps, gathering the results*/
    //  Setup the files we are manipulating
    //  - validated_logs: validated output from input ycov logs + models + prefix_paths
    //  - report_outfile: report result file from processing validated_logs
    //  - ycov_logfile:   file collecting the output of running the tools
	pathPrefix := fmt.Sprintf("%s/%s", yc.ws, outFname)
	validatedLogs := fmt.Sprintf("%s_validated.json", pathPrefix)
	reportOutFile := fmt.Sprintf("%s_report.json", pathPrefix)
	ycovLogFile := fmt.Sprintf("%s_ycov.log", pathPrefix)

	//  Setup the required context for running the mgbl coverage tools as per:
	//  https://wiki.cisco.com/display/XRMGBLMOVE/Yang+Data-Model+Coverage
    //  - Setup prefix_paths and verbose_mode options if needed
	if prefixPaths != "" {
		ppaths = fmt.Sprintf("--prefix-path={%s}", prefixPaths)
	}

	if verbose {
		vmode = "--verbose-mode"
	}
	// - Setup basic pyang invocation, extended with additional options
	pyangcmd := fmt.Sprintf("pyang -p %s/manageability/yang/pyang/modules -f yang_coverage %s %s", yc.ws, vmode, ppaths)

	data := fdata{
		"ws": yc.ws,
		"pyangcmd": pyangcmd,
		"log_file": logFile,
		"validated_logs": validatedLogs,
		"models": yc.models,
		"report_outfile": reportOutFile,
	}
	// - Setup the full interactions with the tools using the mgbl python and pyang env
	coverageScript, err := fstring(`
		source {{.ws}}/manageability/yang/bin/xr_mgbl_pywrap.sh;
		cd /auto/mgbl/xr-yang-scripts/pyang;
		source env.sh;
		cd /nobackup/$USER;
		rm -rf .venv/;
		$(get_python_exec) -m venv --system-site-packages .venv/;
		source .venv/bin/activate;
		cd {{.ws}};
		{{.pyangcmd}} --validate --log-files {{.log_file}}  --output-file {{.validated_logs}} {{.models}} 2>&1;
		{{.pyangcmd}} -f yang_coverage --report --log-files {{.validated_logs}} --output-file {{.report_outfile}} {{.models}}  2>&1;
		deactivate
	`, data)
	cmd := exec.Command(coverageScript)
	logp, err := os.Create(ycovLogFile)
	if err != nil {
		return -1, fmt.Sprintf("File creation failed: %s", err.Error())
	}
	defer logp.Close()

	cmd.Stdout = logp 
	err = cmd.Start(); if err != nil {
    	return -1, err.Error()
    }
    cmd.Wait()

	if err = copy(validatedLogs, MGBL_PATH); err != nil {
		return -1, fmt.Sprintf("WARNING: Coverage logs copy failed to %s. Current path of %s.Please copy reports manually. [Note: Copy needs to be done from sjc host.]", validatedLogs, MGBL_PATH)
	}

	return 0, fmt.Sprintf("Coverage logs stored at %s/%s_validated.json\nYCov tool logs at %s", MGBL_PATH, outFname, ycovLogFile)
}

func (yc *YangCoverage) GatherLogsRpc(testName string) {
	if testName == "" {
		testName = yc.testName
	}
	//todo: RPC
}

func (yc *YangCoverage) generateReport(rawYcovLogs, outfile string, verboseMode bool, prefixPaths string) (int, string){
	/* Run the pyang validate and report steps, gathering the results */
	rawYcovLogs = strings.TrimSpace(rawYcovLogs)
	if rawYcovLogs == "" {
		return -1, "Coverage logs are empty!!"
	}
	// Extract the logs from the RPC response (should be JSON format) between the coverage-logs XML tags
	logSlice := strings.Split(rawYcovLogs, "<coverage-logs xmlns=\"http://cisco.com/ns/yang/Cisco-IOS-XR-yang-coverage-act\">")
	logs := logSlice[len(logSlice)-1]
	logSlice = strings.Split(logs, "</coverage-logs>")
	logs = logSlice[0]

	// Clean the logs as the netconf tool can encode them into HTML
    // - Replace &quot; with " etc.
    // - Save the logs to file
	logs = strings.Replace(logs, "&quot;", "\"", -1)
	logFile := fmt.Sprintf("%s/collected_ycov_logs.json", yc.ws)

	f, err := os.Create(logFile)
    if err != nil {
        return -1, err.Error()
    }

    defer f.Close()

    _, err = f.WriteString(logs)
    if err != nil {
        return -1, err.Error()
    }

	// Setup the file for the final report
	if outfile == "" {
		t := time.Now()
		dt := fmt.Sprintf("%d_%02d_%02d__%02d_%02d_%02d",
        t.Year(), t.Month(), t.Day(),
        t.Hour(), t.Minute(), t.Second())
		outfile = fmt.Sprintf("%s_%s_%s", dt, yc.testName, yc.testType.String())
	} else {
		outfile = strings.Replace(outfile, ".json", "", -1)
	}

	// Call the internal generate worker
	return yc._generateReport(logFile, outfile, verboseMode, prefixPaths)
}

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
