// This package contains cisco specefic binding APIs to config a router using oc and text and cli.

package config

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	log "github.com/golang/glog"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/prototext"
)

// TextWithSSH applies the cli confguration via ssh on the device
func TextWithSSH(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, timeout time.Duration) string {
	t.Helper()
	sshClient := dut.RawAPIs().CLI(t)
	cliOut := sshClient.Stdout()
	cliIn := sshClient.Stdin()
	if _, err := cliIn.Write([]byte(cfg)); err != nil {
		t.Fatalf("Failed to write using ssh: %v", err)
	}
	buf := make([]byte, 32768) // RFC 4253 max payload size for ssh
	ch := make(chan bool)
	response := ""
	go func() {
		for {
			n, err := cliOut.Read(buf)
			if err != nil {
				if err == io.EOF {
					response = fmt.Sprintf("%s%s", response, string(buf[:n]))
					if checkCLIConfigIsApplied(response) {
						ch <- true
						break
					}
				}
				ch <- false
				break
			} else {
				response = fmt.Sprintf("%s%s", response, string(buf[:n]))
				if checkCLIConfigIsApplied(response) {
					ch <- true
					break
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()
	select {
	case resp := <-ch:
		log.V(1).Infof("ssh reply: %s", response)
		if resp {
			return response
		}
		t.Fatalf("Response message for ssh is not as expected %s", response)
	case <-time.After(timeout):
		t.Fatalf("Did not recieve the expected response (timeout)")
	}
	return ""
}

func checkCLIConfigIsApplied(output string) bool {
	// Note that we assume the config contains only one commit and ends with that commit
	/* commit
	*****
	****(config)# */
	if strings.Contains(output, "commit") && strings.HasSuffix(output, "(config)#") {
		return true
	}
	return false
}

// TextWithGNMI apply the cfg  (cisco text config)  on the device using gnmi update.
func TextWithGNMI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string) *gpb.SetResponse {
	t.Helper()
	gnmiC := dut.RawAPIs().GNMI().New(t)
	textReplaceReq := &gpb.Update{
		Path: &gpb.Path{Origin: "cli"},
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{
				AsciiVal: cfg,
			},
		},
	}
	setRequest := &gpb.SetRequest{
		Update: []*gpb.Update{textReplaceReq},
	}
	log.V(1).Info(prettySetRequest(setRequest))
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Set(ctx, setRequest)
	if err != nil {
		t.Fatalf("GNMI replace is failed; %v", err)
	}
	return resp
}

// GNMICommitReplace replace the router config with the cfg  (cisco text config)  on the device using gnmi replace.
func GNMICommitReplace(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string) *gpb.SetResponse {
	t.Helper()
	gnmiC := dut.RawAPIs().GNMI().New(t)
	textReplaceReq := &gpb.Update{
		Path: &gpb.Path{Origin: "cli"},
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{
				AsciiVal: cfg,
			},
		},
	}
	setRequest := &gpb.SetRequest{
		Replace: []*gpb.Update{textReplaceReq},
	}
	log.V(1).Info(prettySetRequest(setRequest))
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Set(ctx, setRequest)
	if err != nil {
		t.Fatalf("GNMI replace is failed; %v", err)
	}
	return resp
}

// GNMICommitReplaceWithOC apply the oc config and text config on the device. The result expected to be the merge of both configuations
func GNMICommitReplaceWithOC(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, pathStruct ygot.PathStruct, ocVal interface{}) *gpb.SetResponse {
	t.Helper()
	gnmiC := dut.RawAPIs().GNMI().New(t)
	textReplaceReq := &gpb.Update{
		Path: &gpb.Path{Origin: "cli"},
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{
				AsciiVal: cfg,
			},
		},
	}
	path, _, errs := ygot.ResolvePath(pathStruct)
	if errs != nil {
		t.Fatalf("Could not resolve the path; %v", errs)
	}
	path.Target = ""
	path.Origin = "openconfig"

	ocJSONVal, err := ygot.Marshal7951(ocVal, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: true})
	if err != nil {
		t.Fatalf("Could not encode value (ocVal) into JSON format; %v", err)
	}
	ocReplaceReq := &gpb.Update{
		Path: path,
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonIetfVal{
				JsonIetfVal: ocJSONVal,
			},
		},
	}

	setRequest := &gpb.SetRequest{
		Replace: []*gpb.Update{textReplaceReq, ocReplaceReq},
	}
	log.V(1).Info(prettySetRequest(setRequest))
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Set(ctx, setRequest)
	if err != nil {
		t.Fatalf("GNMI replace is failed; %v", err)
	}
	return resp
}

type setOperation int

const (
	// DeleteOC represents a SetRequest delete for an oc path.
	DeleteOC setOperation = iota
	// ReplaceOC represents a SetRequest replace for an oc path.
	ReplaceOC
	// 	UpdateOC represents a SetRequest update for an oc path.
	UpdateOC
	// UpdateCLI represents a SetRequest update for a cli text config.
	UpdateCLI
	// ReplaceCLI represents a SetRequest replace for a cli text config.
	ReplaceCLI
)

// BatchRequest is an struct to wrap a batch set request
type BatchSetRequest struct {
	req *gpb.SetRequest
}

// BatchRequest unifies the batch request for set and get
type BatchRequest interface {
	Send(ctx context.Context, t *testing.T, path *gpb.Path, val interface{}, op setOperation) error
	Append(ctx context.Context, t *testing.T, path *gpb.Path, val interface{}, op setOperation) error
	Reset(t *testing.T)
}

// NewBatchSetRequest initialize a batch rset request
func NewBatchSetRequest() *BatchSetRequest {
	return &BatchSetRequest{
		req: &gpb.SetRequest{},
	}
}

// Reset the batch request
func (batch *BatchSetRequest) Reset(t *testing.T) {
	batch.req.Reset()
}

// Append add a GNMI Update/Delete/Replace request to a batch request
func (batch *BatchSetRequest) Append(ctx context.Context, t *testing.T, pathStruct ygot.PathStruct, val interface{}, op setOperation) {
	t.Helper()
	if op != DeleteOC && val == nil {
		t.Fatalf("Cannot append a nil value to the batch set request")
	}

	switch op {
	case DeleteOC:
		path, _, errs := ygot.ResolvePath(pathStruct)
		if errs != nil {
			t.Fatalf("Could not resolve the path; %v", errs)
		}
		path.Target = ""
		path.Origin = "openconfig"
		batch.req.Delete = append(batch.req.Delete, path)
	case ReplaceCLI, UpdateCLI:
		cfg, ok := val.(string)
		if !ok {
			t.Fatalf("The value for cli Set and Update should be an string")
		}
		textReplaceReq := &gpb.Update{
			Path: &gpb.Path{Origin: "cli"},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: cfg,
				},
			},
		}
		if op == ReplaceCLI {
			batch.req.Replace = append(batch.req.Replace, textReplaceReq)
		} else {
			batch.req.Update = append(batch.req.Update, textReplaceReq)
		}
	case ReplaceOC, UpdateOC:
		path, _, errs := ygot.ResolvePath(pathStruct)
		path.Origin = "openconfig"
		if errs != nil {
			t.Fatalf("Could not resolve the path; %v", errs)
		}
		js, err := ygot.Marshal7951(val, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: true})
		if err != nil {
			t.Fatalf("Could not encode value into JSON format: %v", err)
		}
		update := &gpb.Update{
			Path: path,
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonIetfVal{
					JsonIetfVal: js,
				},
			},
		}
		switch op {
		case ReplaceOC:
			batch.req.Replace = append(batch.req.Replace, update)
		case UpdateOC:
			batch.req.Update = append(batch.req.Update, update)
		}
	}
}

// Send sends the batchset request  using GNMI. The batch request is mix of cli update replace  and oc replace, oc update, and oc delete.
func (batch *BatchSetRequest) Send(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) *gpb.SetResponse {
	t.Helper()
	gnmiC := dut.RawAPIs().GNMI().New(t)
	log.V(1).Infof("BatchSet Request: \n", prettySetRequest(batch.req))
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*180)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Set(ctx, batch.req)
	if err != nil {
		t.Fatalf("GNMI replace is failed; %v", err)
	}
	log.V(1).Infof("BatchSet Reply: \n %s", prototext.Format(resp))
	return resp
}

// copied from Ondatra code
func prettySetRequest(setRequest *gpb.SetRequest) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "SetRequest:\n%s\n", prototext.Format(setRequest))

	writePath := func(path *gpb.Path) {
		pathStr, err := ygot.PathToString(path)
		if err != nil {
			pathStr = prototext.Format(path)
		}
		fmt.Fprintf(&buf, "%s\n", pathStr)
	}

	writeVal := func(val *gpb.TypedValue) {
		switch v := val.Value.(type) {
		case *gpb.TypedValue_JsonIetfVal:
			fmt.Fprintf(&buf, "%s\n", v.JsonIetfVal)
		default:
			fmt.Fprintf(&buf, "%s\n", prototext.Format(val))
		}
	}

	for i, path := range setRequest.Delete {
		fmt.Fprintf(&buf, "-------delete path #%d------\n", i)
		writePath(path)
	}
	for i, update := range setRequest.Replace {
		fmt.Fprintf(&buf, "-------replace path/value pair #%d------\n", i)
		writePath(update.Path)
		writeVal(update.Val)
	}
	for i, update := range setRequest.Update {
		fmt.Fprintf(&buf, "-------update path/value pair #%d------\n", i)
		writePath(update.Path)
		writeVal(update.Val)
	}
	return buf.String()
}
