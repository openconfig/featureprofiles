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
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/crypto/ssh"
)

// cliFixture configures a local SSH server and client connected over
// a socket pair.  The socketpair, server transport and client
// transport will implicitly close when the handshake fails, or if the
// cli is closed.
type cliFixture struct {
	cli *cli
}

// serverPrivateKey and serverPublicKey are ed25519 key pairs
// specifically generated for testing.  This is initialized as
// serverSigner.
const (
	serverPrivateKey = `
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDIRPoqlqQZ77bQhAHmsb7y3z12517NYfPdvmbPjyCArgAAALACbVESAm1R
EgAAAAtzc2gtZWQyNTUxOQAAACDIRPoqlqQZ77bQhAHmsb7y3z12517NYfPdvmbPjyCArg
AAAEA61zv3vNLEr1ExQCdCCxgmHwu1XC9VOAWOwCjBhZA7L8hE+iqWpBnvttCEAeaxvvLf
PXbnXs1h892+Zs+PIICuAAAALGxpdWxrQGxpdWxrLW1hY2Jvb2twcm8zLnJvYW0uY29ycC
5nb29nbGUuY29tAQ==
`
	serverPublicKey = `
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMhE+iqWpBnvttCEAeaxvvLfPXbnXs1h892+Zs+PIICu test_key
`
)

func sshServerKeyPEM() []byte {
	const keyType = "OPENSSH PRIVATE KEY"
	var b bytes.Buffer
	fmt.Fprintf(&b, "-----BEGIN %s-----", keyType)
	b.WriteString(serverPrivateKey)
	fmt.Fprintf(&b, "-----END %s-----", keyType)
	b.WriteRune('\n')
	return b.Bytes()
}

var serverSigner ssh.Signer

func init() {
	var err error
	serverSigner, err = ssh.ParsePrivateKey(sshServerKeyPEM())
	if err != nil {
		panic(err)
	}
}

func (f *cliFixture) start(t testing.TB) error {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)
	if err != nil {
		return err
	}

	serverFile := os.NewFile(uintptr(fds[0]), "socketpair[0]")
	defer serverFile.Close() // Does not affect serverConn.
	serverConn, err := net.FileConn(serverFile)
	if err != nil {
		return err
	}

	clientFile := os.NewFile(uintptr(fds[1]), "socketpair[1]")
	defer clientFile.Close() // Does not affect clientConn.
	clientConn, err := net.FileConn(clientFile)
	if err != nil {
		return err
	}

	serverConfig := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == "alice" && string(password) == "bob" {
				return nil, nil
			}
			return nil, errors.New("login error")
		},
	}
	serverConfig.AddHostKey(serverSigner)

	clientConfig := &ssh.ClientConfig{
		User:            "alice",
		Auth:            []ssh.AuthMethod{ssh.Password("bob")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Server and client handshakes must be done simultaneously.
	// Conveniently, once we pass the net.Conn to ssh.NewServerConn and
	// ssh.NewClientConn, they hand the net.Conn to a mux which will
	// close the socket when handshake fails or when ssh.Client closes,
	// so we do not need to close the server or client transport.

	// Obtain the handshake status from the server and the client.
	errch := make(chan error)
	defer close(errch)

	go func() {
		t.Log("Server begins handshake.")
		_, serverChans, serverReq, err := ssh.NewServerConn(serverConn, serverConfig)
		if err != nil {
			errch <- fmt.Errorf("server error: %w", err)
			return
		}
		go f.handleServerNewChannel(serverChans)
		go ssh.DiscardRequests(serverReq)

		t.Log("Server is ready.")
		errch <- nil
	}()

	var client *ssh.Client

	go func() {
		t.Log("Client begins handshake.")
		clientTransport, clientChans, clientReq, err := ssh.NewClientConn(clientConn, "socketpair", clientConfig)
		if err != nil {
			errch <- fmt.Errorf("client error: %w", err)
			return
		}

		t.Log("Client is ready.")
		client = ssh.NewClient(clientTransport, clientChans, clientReq)
		errch <- nil
	}()

	if err1, err2 := <-errch, <-errch; err1 != nil || err2 != nil {
		return fmt.Errorf("handshake errors: %v; and %v", err1, err2)
	}

	cli, err := newCLI(client)
	if err != nil {
		return err
	}
	f.cli = cli
	return nil
}

func (f *cliFixture) handleServerNewChannel(ch <-chan ssh.NewChannel) {
	for newChannel := range ch {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Could not accept channel: %v", err)
			continue
		}
		go f.handleServerChannel(channel, requests)
	}
}

func which(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// handleServerChannel provides a basic session for testing that
// echoes stdin lines to either stdout (default) or stderr (if stdin
// line begins with "stderr"), alone with whether shell or pty has
// been requested.
//
// Example interaction:
//
//   - stdin: hello
//   - stdout: nopty noshell hello
//   - pty-req
//   - stdin: hello again
//   - stdout: pty noshell hello again
//   - shell
//   - stdin: stderr hello
//   - stderr: pty shell stderr hello
func (f *cliFixture) handleServerChannel(c ssh.Channel, reqs <-chan *ssh.Request) {
	var shell, pty bool
	r := bufio.NewReader(c)

	go func() {
		for req := range reqs {
			switch req.Type {
			case "shell":
				shell = true
				req.Reply(true, nil)
			case "pty-req":
				pty = true
				req.Reply(true, nil)
			case "exec":
				f.handleExec(c, req)
			default:
				req.Reply(false, nil)
			}
		}
	}()

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return // io.EOF or otherwise.
		}

		output := fmt.Sprintf("%s %s %s",
			which(pty, "pty", "nopty"),
			which(shell, "shell", "noshell"),
			line)
		if strings.HasPrefix(line, "stderr") {
			c.Stderr().Write([]byte(output))
		} else {
			c.Write([]byte(output))
		}
	}
}

// handleExec provides a command that prints the executed command to
// stdout, a message to stderr, and exit status 0.  The output may be
// out of order.
//
//   - stdout: exec command: <command>
//   - stderr: exec stderr
func (f *cliFixture) handleExec(c ssh.Channel, req *ssh.Request) {
	var execMsg struct{ Command string }
	if err := ssh.Unmarshal(req.Payload, &execMsg); err != nil {
		req.Reply(false, nil)
		return
	}
	req.Reply(true, nil)
	output := fmt.Sprintf("exec command: %s\n", execMsg.Command)
	c.Write([]byte(output))
	c.Stderr().Write([]byte("exec stderr\n"))
	c.CloseWrite()

	statusMsg := make([]byte, 4)
	binary.BigEndian.PutUint32(statusMsg, 0)
	c.SendRequest("exit-status", false, statusMsg)
	c.Close()
}

var cmpSortStrings = cmpopts.SortSlices(func(a, b string) bool {
	return a < b
})

func TestCLI(t *testing.T) {
	f := &cliFixture{}
	if err := f.start(t); err != nil {
		t.Fatalf("Could not start cliFixture: %v", err)
	}
	t.Log("Test is ready.")

	output, err := f.cli.SendCommand(context.Background(), "xyzzy")
	if err != nil {
		t.Fatalf("Could not execute command: %v", err)
	}
	got := strings.Split(output, "\n")
	want := []string{"exec command: xyzzy", "exec stderr", ""}
	if diff := cmp.Diff(want, got, cmpSortStrings); diff != "" {
		t.Errorf("Command output -want, +got:\n%s", diff)
	}
}
