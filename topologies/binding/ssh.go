// Copyright 2025 Google LLC
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

// sshClient implements the binding.SSHClient interface using an SSH client.
type sshClient struct {
	*binding.AbstractSSHClient
	ssh *ssh.Client
}

func newSSH(sc *ssh.Client) (*sshClient, error) {
	return &sshClient{ssh: sc}, nil
}

func (c *sshClient) RunCommand(_ context.Context, cmd string) (binding.CommandResult, error) {
	session, err := c.ssh.NewSession()
	if err != nil {
		return nil, fmt.Errorf("could not create session: %w", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	switch err.(type) {
	case nil:
		return &commandResult{output: string(out)}, nil
	case *ssh.ExitError, *ssh.ExitMissingError:
		return &commandResult{output: string(out), error: err.Error()}, nil
	default:
		return nil, err
	}
}

func (c *sshClient) Close() error {
	return c.ssh.Close()
}

type commandResult struct {
	*binding.AbstractCommandResult
	output, error string
}

func (r *commandResult) Output() string {
	return r.output
}

func (r *commandResult) Error() string {
	return r.error
}
