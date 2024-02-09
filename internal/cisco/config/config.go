// Package config contains cisco specefic binding APIs to config a router using oc and text and cli.
package config

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	log "github.com/golang/glog"
	"github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/prototext"
)

// Reload excure the hw-module reload on the router. It aslo apply the configs before and after the reload.
// The reload  will fail if the router is not responsive after max wait time.
// Part of this code copied from ondtara
func Reload(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, beforeReloadConfig, afterReloadConfig string, maxTimeout time.Duration) {
	t.Logf("Realoding router %s", dut.Name())
	if beforeReloadConfig != "" {
		TextWithGNMI(ctx, t, dut, beforeReloadConfig)
		t.Logf("The configuration %s \n is loaded correctly before reloading router %s", beforeReloadConfig, dut.Name())
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	_, err := gnoiClient.System().Reboot(ctx, &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Reboot is failed %v", err)
	}

	time.Sleep(maxTimeout)

	if afterReloadConfig != "" {
		TextWithGNMI(ctx, t, dut, afterReloadConfig)
		t.Logf("The configuration %s \n is loaded correctly after reloading router %s", beforeReloadConfig, dut.Name())
	}
}

// TextWithSSH applies the cli confguration via ssh on the device
func TextWithSSH(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, timeout time.Duration) string {
	t.Helper()

	sshClient := dut.RawAPIs().CLI(t)
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := sshClient.RunCommand(tctx, cfg)
	if err != nil {
		t.Fatalf("Failed to write using ssh: %v", err)
	}
	if !checkCLIConfigIsApplied(response.Output()) {
		t.Fatalf("Response message for ssh is not as expected %s", response)
	}
	return response.Output()
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

// CMDViaGNMI push cli command to cisco router using GNMI, (have not tested well)
func CMDViaGNMI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cmd string) string {
	gnmiC := dut.RawAPIs().GNMI(t)
	getRequest := &gnmi.GetRequest{
		Prefix: &gnmi.Path{
			Origin: "cli",
		},
		Path: []*gnmi.Path{
			{
				Elem: []*gnmi.PathElem{{
					Name: cmd,
				}},
			},
		},
		Encoding: gnmi.Encoding_ASCII,
	}
	log.V(1).Infof("get cli (%s) via GNMI: \n %s", cmd, prototext.Format(getRequest))
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Get(ctx, getRequest)
	if err != nil {
		t.Fatalf("running cmd (%s) via GNMI is failed: %v", cmd, err)
	}
	log.V(1).Infof("get cli via gnmi reply: \n %s", prototext.Format(resp))
	return string(resp.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())

}

// TextWithGNMI applies the cfg  (cisco text config)  on the device using gnmi update.
func TextWithGNMI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string) *gnmi.SetResponse {
	t.Helper()
	gnmiC := dut.RawAPIs().GNMI(t)
	textReplaceReq := &gnmi.Update{
		Path: &gnmi.Path{Origin: "cli"},
		Val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_AsciiVal{
				AsciiVal: cfg,
			},
		},
	}
	setRequest := &gnmi.SetRequest{
		Update: []*gnmi.Update{textReplaceReq},
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
func GNMICommitReplace(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string) *gnmi.SetResponse {
	t.Helper()
	gnmiC := dut.RawAPIs().GNMI(t)
	textReplaceReq := &gnmi.Update{
		Path: &gnmi.Path{Origin: "cli"},
		Val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_AsciiVal{
				AsciiVal: cfg,
			},
		},
	}
	setRequest := &gnmi.SetRequest{
		Replace: []*gnmi.Update{textReplaceReq},
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
func GNMICommitReplaceWithOC(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, pathStruct ygnmi.PathStruct, ocVal interface{}) *gnmi.SetResponse {
	t.Helper()
	gnmiC := dut.RawAPIs().GNMI(t)
	textReplaceReq := &gnmi.Update{
		Path: &gnmi.Path{Origin: "cli"},
		Val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_AsciiVal{
				AsciiVal: cfg,
			},
		},
	}
	path, _, errs := ygnmi.ResolvePath(pathStruct)
	if errs != nil {
		t.Fatalf("Could not resolve the path; %v", errs)
	}
	path.Target = ""
	path.Origin = "openconfig"

	ocJSONVal, err := ygot.Marshal7951(ocVal, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: true})
	if err != nil {
		t.Fatalf("Could not encode value (ocVal) into JSON format; %v", err)
	}
	ocReplaceReq := &gnmi.Update{
		Path: path,
		Val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_JsonIetfVal{
				JsonIetfVal: ocJSONVal,
			},
		},
	}

	setRequest := &gnmi.SetRequest{
		Replace: []*gnmi.Update{textReplaceReq, ocReplaceReq},
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
	// UpdateOC represents a SetRequest update for an oc path.
	UpdateOC
	// UpdateCLI represents a SetRequest update for a cli text config.
	UpdateCLI
	// ReplaceCLI represents a SetRequest replace for a cli text config.
	ReplaceCLI
)

// BatchSetRequest is an struct to wrap a batch set request
type BatchSetRequest struct {
	req *gnmi.SetRequest
}

// BatchRequest unifies the batch request for set and get
type BatchRequest interface {
	Send(ctx context.Context, t *testing.T, path *gnmi.Path, val interface{}, op setOperation) error
	Append(ctx context.Context, t *testing.T, path *gnmi.Path, val interface{}, op setOperation) error
	Reset(t *testing.T)
}

// NewBatchSetRequest initialize a batch rset request
func NewBatchSetRequest() *BatchSetRequest {
	return &BatchSetRequest{
		req: &gnmi.SetRequest{},
	}
}

// Reset the batch request
func (batch *BatchSetRequest) Reset(t *testing.T) {
	batch.req.Reset()
}

// Append add a GNMI Update/Delete/Replace request to a batch request
func (batch *BatchSetRequest) Append(ctx context.Context, t *testing.T, pathStruct ygnmi.PathStruct, val interface{}, op setOperation) {
	t.Helper()
	if op != DeleteOC && val == nil {
		t.Fatalf("Cannot append a nil value to the batch set request")
	}

	switch op {
	case DeleteOC:
		path, _, errs := ygnmi.ResolvePath(pathStruct)
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
		textReplaceReq := &gnmi.Update{
			Path: &gnmi.Path{Origin: "cli"},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{
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
		path, _, errs := ygnmi.ResolvePath(pathStruct)
		path.Origin = "openconfig"
		if errs != nil {
			t.Fatalf("Could not resolve the path; %v", errs)
		}
		js, err := ygot.Marshal7951(val, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: true})
		if err != nil {
			t.Fatalf("Could not encode value into JSON format: %v", err)
		}
		update := &gnmi.Update{
			Path: path,
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
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
func (batch *BatchSetRequest) Send(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetResponse {
	t.Helper()
	gnmiC := dut.RawAPIs().GNMI(t)
	log.V(1).Infof("BatchSet Request: \n %s", prettySetRequest(batch.req))
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
func prettySetRequest(setRequest *gnmi.SetRequest) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "SetRequest:\n%s\n", prototext.Format(setRequest))

	writePath := func(path *gnmi.Path) {
		pathStr, err := ygot.PathToString(path)
		if err != nil {
			pathStr = prototext.Format(path)
		}
		fmt.Fprintf(&buf, "%s\n", pathStr)
	}

	writeVal := func(val *gnmi.TypedValue) {
		switch v := val.Value.(type) {
		case *gnmi.TypedValue_JsonIetfVal:
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

// CLIViaSSH run the cli command (show or admin) via ssh on the device
func CLIViaSSH(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cmd string, timeout time.Duration) string {
	t.Helper()
	if !strings.HasSuffix(cmd, "\n") {
		cmd = cmd + " \n"
	}
	sshClient := dut.RawAPIs().CLI(t)
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := sshClient.RunCommand(tctx, cmd)
	if err != nil {
		t.Fatalf("Failed to write using ssh: %v", err)
	}
	return response.Output()
}
