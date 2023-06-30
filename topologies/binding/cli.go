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
	"io"

	"github.com/openconfig/ondatra/binding"
	"golang.org/x/crypto/ssh"
)

// cli implements the binding.StreamClient interface using an SSH
// client (see also the ondatra.StreamClient returned by
// dut.RawAPIS().CLI()).  It creates a default session with pty and
// shell to service stdin, stdout, and stderr; each SendCommand will
// run in its own session but without shell or pty.
type cli struct {
	*binding.AbstractStreamClient

	ssh    *ssh.Client
	sess   *ssh.Session
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

var _ = binding.StreamClient(&cli{})

func newCLI(sc *ssh.Client) (*cli, error) {
	sess, err := sc.NewSession()
	if err != nil {
		return nil, fmt.Errorf("could not create session: %w", err)
	}
	if err := sess.RequestPty("ansi", 24, 80, nil); err != nil {
		return nil, fmt.Errorf("could not request pty: %w", err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("could not get stdin: %w", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("could not get stdout: %w", err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("could not get stderr: %w", err)
	}
	if err := sess.Shell(); err != nil {
		return nil, fmt.Errorf("could not start shell: %w", err)
	}
	c := &cli{
		ssh:    sc,
		sess:   sess,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
	return c, nil
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

func (c *cli) Stdin() io.WriteCloser {
	return c.stdin
}

func (c *cli) Stdout() io.ReadCloser {
	return io.NopCloser(c.stdout)
}

func (c *cli) Stderr() io.ReadCloser {
	return io.NopCloser(c.stderr)
}

func (c *cli) Close() error {
	return c.ssh.Close()
}
