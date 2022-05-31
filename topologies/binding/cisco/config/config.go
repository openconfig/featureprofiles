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
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/prototext"
)

// TextWithSSH applies the cli confguration via ssh on the device
func TextWithSSH(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, timeout time.Duration) string {
	sshClient := dut.RawAPIs().CLI(t)
	cliOut := sshClient.Stdout()
	cliIn := sshClient.Stdin()
	if _, err := cliIn.Write([]byte(cfg)); err != nil {
		t.Fatalf("failed to write using ssh: %v", err)
	}
	buf := make([]byte, 32768) // RFC 4253 max payload size for ssh
	ch := make(chan bool)
	n := 0
	var err error
	response := ""
	go func() {
		for {
			n, err = cliOut.Read(buf)
			if err != nil {
				if err == io.EOF {
					response = fmt.Sprintf("%s%s", response, string(buf[:n]))
					if checkCLIConfigIsApplied(response) {
						ch <- true
					} else {
						ch <- false
					}

				} else {
					ch <- false
				}
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
			// add logging here
			return response
		}
		// add logging here
		t.Fatalf("Response message for ssh command is not as expected")
	case <-time.After(timeout):
		// add logging here
		t.Fatalf("Did not recieve the expected response (timeout)")
		return response
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
	resp, err := gnmiC.Set(context.Background(), setRequest)
	if err != nil {
		t.Fatalf("GNMI replace is failed; %v", err)
	}
	return resp
}

// CMDViaSSH push cli command to cisco router, (have not tested well)
func CMDViaGNMI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cmd string) string {
	gnmiC := dut.RawAPIs().GNMI().New(t)
	getRequest := &gpb.GetRequest{
		Prefix: &gpb.Path{
			Origin: "cli",
		},
		Path: []*gpb.Path{
			{
				Elem: []*gpb.PathElem{{
					Name: cmd,
				}},
			},
		},
		Encoding: gpb.Encoding_ASCII,
	}
	log.V(1).Infof("get cli (%s) via GNMI: \n %s", cmd, prototext.Format(getRequest))
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
        tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
    }
	resp, err := gnmiC.Get(ctx, getRequest); if err!=nil {
		t.Fatalf("running cmd (%s) via GNMI is failed: %v", cmd, err)
	}
	log.V(1).Infof("get cli via gnmi reply: \n %s", prototext.Format(resp))
	return string(resp.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
	// return string(gotRes.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal())

}

// GNMICommitReplace replace the router config with the cfg  (cisco text config)  on the device using gnmi replace.
func GNMICommitReplace(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string) *gpb.SetResponse {
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

// Reload excure the hw-module reload on the router. It aslo apply the configs before and after the reload.
// The reload  will fail if the router is not responsive after max wait time.
// Part of this code copied from ondtara
func Reload(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, beforeReloadConfig, afterReloadConfig string, maxTimeout time.Duration) {
	t.Logf("Realoding router %s", dut.Name())
	if beforeReloadConfig != "" {
		TextWithGNMI(ctx, t, dut, beforeReloadConfig)
		t.Logf("The configuration %s \n is loaded correctly before reloading router %s", beforeReloadConfig, dut.Name())
	}

	gnoiClient := dut.RawAPIs().GNOI().New(t)
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

	/*ctx, cncl := context.WithTimeout(context.Background(), time.Second*60)
	defer cncl()
	gnoiClient = dut.RawAPIs().GNOI().New(t) // new gno client can not be opended unless the reboot is finished*/

	/*rebootTimeout := maxTimeout
	switch {
	case rebootTimeout == 0:
		rebootTimeout = 6 * time.Minute
	case rebootTimeout < 0:
		t.Fatalf("reboot timeout must be a positive duration")
	}
	rebootDeadline := time.Now().Add(rebootTimeout)
	retry := true
	for retry {
		if time.Now().After(rebootDeadline) {
			retry = false
			break
		}
		resp, err := gnoiClient.System().RebootStatus(ctx, &spb.RebootStatusRequest{})
		switch {
		case status.Code(err) == codes.Unimplemented:
			// Unimplemented means we don't have a valid way
			// to validate health of reboot.
			t.Fatalf("Can not get the reboot status of dut %s", dut.Name())
		case err == nil:
			if !resp.GetActive() {
				t.Fatalf("Reboot failed for dut  %s", dut.Name())
			}
		default:
			// any other error just sleep.
		}
		statusWait := time.Duration(resp.GetWait()) * time.Nanosecond
		if statusWait <= 0 {
			statusWait = 30 * time.Second
		}
		time.Sleep(statusWait)
	}
	t.Fatalf("reboot of %s timed out after %s", dut.Name(), maxTimeout)

	*/
	// TODO: use select and channel to detect when the router reload is complete

	if afterReloadConfig != "" {
		TextWithGNMI(ctx, t, dut, afterReloadConfig)
		t.Logf("The configuration %s \n is loaded correctly after reloading router %s", beforeReloadConfig, dut.Name())
	}
}

// GNMICommitReplaceWithOC apply the oc config and text config on the device. The result expected to be the merge of both configuations
func GNMICommitReplaceWithOC(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, pathStruct ygot.PathStruct, ocVal interface{}) *gpb.SetResponse {
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
	path.Target = ""
	//path.Origin = "openconfig"
	if errs != nil {
		t.Fatalf("Could not resolve the path; %v", errs)
		return nil
	}

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
		//Prefix: &gpb.Path{Origin: "openconfig"},
		// setting origin at the set level when we have cli + oc can cause the request to be rejected
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
