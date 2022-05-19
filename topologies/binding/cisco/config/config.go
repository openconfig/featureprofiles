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
	spb "github.com/openconfig/gnoi/system"
)

// TextWithSSH applies the cli confguration via ssh on the device
func TextWithSSH(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, timeout time.Duration) (string, error) {
	sshClient := dut.RawAPIs().CLI(t)
	cliOut := sshClient.Stdout()
	cliIn := sshClient.Stdin()
	if _, err := cliIn.Write([]byte(cfg)); err != nil {
		t.Errorf("failed to write using ssh: %v", err)
		return "", fmt.Errorf("failed to write using ssh: %w", err)
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
			return response, nil
		}
		// add logging here
		t.Error("Response message for ssh command is not as expected")
		return response, fmt.Errorf("response message for ssh command is not as expected")
	case <-time.After(timeout):
		// add logging here
		t.Error("Did not recieve the expected response (timeout)")
		return response, fmt.Errorf("did not recieve the expected response (timeout)")
	}
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
func TextWithGNMI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string) (*gpb.SetResponse, error) {
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
		t.Errorf("GNMI replace is failed; %v", err)
	}
	return resp, err
}

// GNMICommitReplace replace the router config with the cfg  (cisco text config)  on the device using gnmi replace.
func GNMICommitReplace(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string) (*gpb.SetResponse, error) {
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
	resp, err := gnmiC.Set(context.Background(), setRequest)
	if err != nil {
		t.Errorf("GNMI replace is failed; %v", err)
	}
	return resp, err
}

// Reload excure the hw-module reload on the router. It aslo apply the configs before and after the reload. 
// The reload  will fail if the router is not responsive after max wait time.
func Reload(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, beforeReloadConfig, afterReloadConfig string, maxTimeout time.Duration) {
	t.Logf("Realoding router %s", dut.Name())
	if beforeReloadConfig!="" {
		_,err := TextWithGNMI(ctx, t, dut, beforeReloadConfig); if err != nil {
			t.Fatalf("Reload failed duing applying config before reload %v", err)
		}
		t.Logf("The configuration %s \n is loaded correctly before reloading router %s", beforeReloadConfig,dut.Name() )
	}

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	// TODO: use select and channel to detect when the router reload is complete
	time.Sleep(maxTimeout)

	if afterReloadConfig!="" {
		_,err := TextWithGNMI(ctx, t, dut, afterReloadConfig); if err != nil {
			t.Fatalf("Reload failed duing applying config before reload %v", err)
		}
	}
}

// GNMICommitReplaceWithOC apply the oc config and text config on the device. The result expected to be the merge of both configuations
func GNMICommitReplaceWithOC(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, pathStruct ygot.PathStruct, ocVal interface{}) (*gpb.SetResponse, error) {
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
	path.Origin = "openconfig"
	if errs != nil {
		t.Errorf("Could not resolve the path; %v", errs)
		return nil, fmt.Errorf("could not encode value (ocVal) into JSON format: %v", errs)
	}

	ocJSONVal, err := ygot.Marshal7951(ocVal, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: true})
	if err != nil {
		t.Errorf("Could not encode value (ocVal) into JSON format; %v", err)
		return nil, err
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
	resp, err := gnmiC.Set(context.Background(), setRequest)
	if err != nil {
		t.Errorf("GNMI replace is failed; %v", err)
	}
	return resp, err
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
