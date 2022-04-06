// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This package contains cisco specefic binding APIs such as config via ssh.

package cisco

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/ondatra/binding"

)





// CiscoCLI prvoides access to APIs for running cli command on cisco routers. 
//Usage:
//	dut := ondatra.DUT(t, "dut")
//	ciscoCLI := cisco.CiscoCLI {
//		CLI: dut.RawAPIs().CLI(t),
//	}
//	resp, err := ciscoCLI.Config(t, context.Background(), "config \n hostname test \n commit \n")
type CiscoCLI struct{
	Handle binding.StreamClient
}

// Config apply the configuration passed as cfg for cisco router. The function assumes that 
// the cfg contains only one and only one commit and ends with the commit
func (ciscocli *CiscoCLI) Config(t testing.TB,ctx context.Context, cfg string, timeout time.Duration) (string, error) {
	cliOut := ciscocli.Handle.Stdout()
	cliIn := ciscocli.Handle.Stdin()
	if _, err := cliIn.Write([]byte(cfg)); err != nil {
		return "", fmt.Errorf("failed to run: %w", err)
	}
	buf := make([]byte, 10000)
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
		}
	}()
	select {
	case resp := <-ch:
		if resp {
			return response, nil
		} else {
			return response, fmt.Errorf("response message for ssh command is not as expected")
		}
	case <-time.After(timeout):
		return response, fmt.Errorf("did not recieve the expected response (timeout)")
	}
}

func checkCLIConfigIsApplied(output string) bool {
	// Note that we assume the config contains only one commit and ends with a commit
	/* commit
	*****
	****(config)# */
	if strings.Contains(output, "commit") && strings.HasSuffix(output, "(config)#") {
		return true
	}
	return false
}

