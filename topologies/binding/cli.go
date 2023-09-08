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

func (c *cli) SendCommand(_ context.Context, cmd string) (string, error) {
	sess, err := c.ssh.NewSession()
	if err != nil {
		return "", fmt.Errorf("could not create session: %w", err)
	}
	defer sess.Close()
	buf, err := sess.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("could not execute command: %w", err)
	}
	return string(buf), nil
}
