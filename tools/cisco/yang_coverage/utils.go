package yang_coverage

import (
    "errors"
    "fmt"
    "os"
    "reflect"
    "testing"
    "time"
    "unsafe"

    "github.com/openconfig/featureprofiles/proto/cisco/ycov"
    "github.com/openconfig/ondatra"
    "google.golang.org/grpc"
)

var RAW_LOGS_PATH = "/ws/ncorran-sjc/yang-coverage/rawlogs/"

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

func (y *yCov) storeRawLogs(logs string) (int, string) {
    if logs == "" {
    	return -1, "Coverage logs are empty!!"
    }

    var outfile string

    t := time.Now()
    dt := fmt.Sprintf("%d_%02d_%02d__%02d_%02d_%02d",
    	t.Year(), t.Month(), t.Day(),
    	t.Hour(), t.Minute(), t.Second())
    if y.subCompId != "" {
    	outfile = fmt.Sprintf("%s_%s_%s_%s.json", dt, y.yc.testName, y.subCompId, y.yc.testType.String())
    } else {
    	outfile = fmt.Sprintf("%s_%s_%s.json", dt, y.yc.testName, y.yc.testType.String())
    }
    destPath := fmt.Sprintf("%s/%s", RAW_LOGS_PATH, outfile)

    f, err := os.Create(outfile)
    if err != nil {
    	return -1, err.Error()
    }

    defer f.Close()

    _, err = f.WriteString(logs)
    if err != nil {
    	return -1, err.Error()
    }
    if err = copy(outfile, destPath); err != nil {
    	return -1, fmt.Sprintf("Copy of log file %s failed to %s: %s", outfile, RAW_LOGS_PATH, err.Error())
    }
    return 0, fmt.Sprintf("Raw log file at %s", destPath)
}

func (y *yCov) processYCov(logs string, t *testing.T) (int, string) {
    return y.yc.generateReport(logs, "", y.verbose, y.prefixPaths, y.subCompId)

}

func getYCovCtx() *yCov {
    return yangCovCtx
}
