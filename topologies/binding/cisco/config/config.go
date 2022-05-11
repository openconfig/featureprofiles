// This package contains cisco specefic binding APIs to config a router using oc and text and cli.

package config

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
)

// WithSSH applies the cli confguration via ssh on the device
func WithSSH(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, timeout time.Duration) (string, error) {
	sshClient := dut.RawAPIs().CLI(t)
	cliOut := sshClient.Stdout()
	cliIn := sshClient.Stdin()
	if _, err := cliIn.Write([]byte(cfg)); err != nil {
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
		if resp {
			// add logging here
			return response, nil
		}
		// add logging here
		return response, fmt.Errorf("response message for ssh command is not as expected")
	case <-time.After(timeout):
		// add logging here
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

// GNMICommitReplace replace the device configuration with the one as specefied via cfg  (cisco text config).
func GNMICommitReplace(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string) (string, error) {
	// create a set request
	// Encoding_ASCII
	gnmiC := dut.RawAPIs().GNMI().New(t)
	textReplaceReq := &gpb.Update{
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{
				AsciiVal: cfg,
			},
		},
	}
	replaceRequest := &gpb.SetRequest{
		Replace: []*gpb.Update{textReplaceReq},
	}
	// fmt.Println(prototext.Format(inGetRequest))
	resp, err := gnmiC.Set(context.Background(), replaceRequest)
	if err != nil {
		fmt.Printf("%v", err)
	}
	fmt.Printf("%v", resp)
	return "", nil
}

// GNMICommitReplaceWithOC apply the oc config and text config on the device. The result expected to be the merge of both configuations
func GNMICommitReplaceWithOC(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cfg string, timeout time.Duration) (string, error) {
	gnmiC := dut.RawAPIs().GNMI().New(t)
	textReplaceReq := &gpb.Update{
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{
				AsciiVal: cfg,
			},
		},
	}

	ocReplaceReq := &gpb.Update{
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{
				AsciiVal: cfg, // use ietf json  for oc models
			},
		},
	}

	replaceRequest := &gpb.SetRequest{
		Replace: []*gpb.Update{ocReplaceReq, textReplaceReq},
	}
	// fmt.Println(prototext.Format(inGetRequest))
	resp, err := gnmiC.Set(context.Background(), replaceRequest)
	if err != nil {
		fmt.Printf("%v", err)
	}
	fmt.Printf("%v", resp)
	return "", nil
}
