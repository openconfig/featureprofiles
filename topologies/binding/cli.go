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

package binding

import (
	"context"
	"fmt"

	"github.com/openconfig/ondatra/binding"
	"golang.org/x/crypto/ssh"
)

// cli implements the binding.ClientClient interface using an SSH client.
type cli struct {
	*binding.AbstractCLIClient
	ssh *ssh.Client
}

func newCLI(sc *ssh.Client) (*cli, error) {
	return &cli{ssh: sc}, nil
}

func (c *cli) RunCommand(ctx context.Context, cmd string) (binding.CommandResult, error) {
	sess, err := c.ssh.NewSession()
	if err != nil {
		return nil, fmt.Errorf("could not create session: %w", err)
	}
	defer sess.Close()

	outChan := make(chan *struct {
		out string
		err error
	})

	go func() {
		out, err := sess.CombinedOutput(cmd)
		outChan <- &struct {
			out string
			err error
		}{out: string(out), err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-outChan:
		switch result.err.(type) {
		case nil:
			return &cmdResult{output: result.out}, nil
		case *ssh.ExitError, *ssh.ExitMissingError:
			return &cmdResult{output: result.out, error: result.err.Error()}, nil
		default:
			return nil, result.err
		}
	}
}

type cmdResult struct {
	*binding.AbstractCommandResult
	output, error string
}

func (r *cmdResult) Output() string {
	return r.output
}

func (r *cmdResult) Error() string {
	return r.error
}
