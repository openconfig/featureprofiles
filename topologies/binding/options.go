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
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
)

// IANA assigns 9339 for gNxI, and 9559 for P4RT.  There hasn't been a
// port assignment for gRIBI, so using Arista's default which is 6040.
var (
	gnmiPort  = flag.Int("gnmi_port", 9339, "default gNMI port")
	gnoiPort  = flag.Int("gnoi_port", 9339, "default gNOI port")
	gnsiPort  = flag.Int("gnsi_port", 9339, "default gNSI port")
	gribiPort = flag.Int("gribi_port", 6040, "default gRIBI port")
	p4rtPort  = flag.Int("p4rt_port", 9559, "default P4RT part")
)

// creds implements the grpc.PerRPCCredentials interface, to be used
// as a grpc.DialOption in dialGRPC.
type creds struct {
	username, password string
	secure             bool
}

func (c *creds) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": c.username,
		"password": c.password,
	}, nil
}

func (c *creds) RequireTransportSecurity() bool {
	return c.secure
}

var _ = grpc.PerRPCCredentials(&creds{})

// options wraps *bindpb.Options and implements dialers for various
// protocols.
type options struct {
	*bindpb.Options
}

// dialGRPC dials a gRPC connection using the binding options.
//lint:ignore U1000 will be used by the binding.
func (o *options) dialGRPC(ctx context.Context, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	switch {
	case o.Insecure:
		tc := insecure.NewCredentials()
		opts = append(opts, grpc.WithTransportCredentials(tc))
	case o.SkipVerify:
		tc := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
		opts = append(opts, grpc.WithTransportCredentials(tc))
	}
	if o.Username != "" {
		c := &creds{o.Username, o.Password, !o.Insecure}
		opts = append(opts, grpc.WithPerRPCCredentials(c))
	}
	return grpc.DialContext(ctx, o.Target, opts...)
}

var knownHostsFiles = []string{
	"$HOME/.ssh/known_hosts",
	"/etc/ssh/ssh_known_hosts",
}

// knownHostsCallback checks the user and system SSH known_hosts.
//lint:ignore U1000 will be used by the binding.
func knownHostsCallback() (ssh.HostKeyCallback, error) {
	var files []string
	for _, file := range knownHostsFiles {
		file = os.ExpandEnv(file)
		if _, err := os.Stat(file); err == nil {
			files = append(files, file)
		}
	}
	return knownhosts.New(files...)
}

// dialSSH dials an SSH client using the binding options.
//lint:ignore U1000 will be used by the binding.
func (o *options) dialSSH() (*ssh.Client, error) {
	c := &ssh.ClientConfig{
		User: o.Username,
		Auth: []ssh.AuthMethod{ssh.Password(o.Password)},
	}
	if o.SkipVerify {
		c.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		cb, err := knownHostsCallback()
		if err != nil {
			return nil, err
		}
		c.HostKeyCallback = cb
	}
	return ssh.Dial("tcp", o.Target, c)
}

// newHTTPClient makes an http.Client using the binding options.
//lint:ignore U1000 will be used by the binding.
func (o *options) newHTTPClient() *http.Client {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	if o.SkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{Transport: tr}
}

// merge creates a combined options struct from one or more structs.
func merge(bopts ...*bindpb.Options) options {
	result := &bindpb.Options{}
	for _, bopt := range bopts {
		if bopt != nil {
			proto.Merge(result, bopt)
		}
	}
	return options{result}
}

// resolver returns the effective options for specific devices and
// protocols.
type resolver struct {
	*bindpb.Binding
}

// dut looks up the *bindpb.Device with the given dutID.
func (r *resolver) dut(dutID string) *bindpb.Device {
	for _, dut := range r.Duts {
		if dut.Id == dutID {
			return dut
		}
	}
	return nil
}

// ate looks up the *bindpb.Device with the given ateID.
func (r *resolver) ate(ateID string) *bindpb.Device {
	for _, ate := range r.Ates {
		if ate.Id == ateID {
			return ate
		}
	}
	return nil
}

// dutOptions reconstructs the effective options for a given dut and
// protocol.
func (r *resolver) dutOptions(dutID string, port int, optionsFn func(*bindpb.Device) *bindpb.Options) (options, error) {
	dut := r.dut(dutID)
	if dut == nil {
		return options{nil}, fmt.Errorf("dut %q is missing from the binding", dutID)
	}
	targetOptions := &bindpb.Options{
		Target: fmt.Sprintf("%s:%d", dut.Name, port),
	}
	return merge(targetOptions, r.Options, dut.Options, optionsFn(dut)), nil
}

func (r *resolver) gnmi(dutID string) (options, error) {
	return r.dutOptions(dutID, *gnmiPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.Gnmi })
}

func (r *resolver) gnoi(dutID string) (options, error) {
	return r.dutOptions(dutID, *gnoiPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.Gnoi })
}

func (r *resolver) gnsi(dutID string) (options, error) {
	return r.dutOptions(dutID, *gnsiPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.Gnsi })
}

func (r *resolver) gribi(dutID string) (options, error) {
	return r.dutOptions(dutID, *gribiPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.Gribi })
}

func (r *resolver) p4rt(dutID string) (options, error) {
	return r.dutOptions(dutID, *p4rtPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.P4Rt })
}

func (r *resolver) ssh(dutID string) (options, error) {
	dut := r.dut(dutID)
	if dut == nil {
		return options{nil}, fmt.Errorf("dut %q is missing from the binding", dutID)
	}
	targetOptions := &bindpb.Options{Target: dut.Name}
	return merge(targetOptions, r.Options, dut.Options, dut.Ssh), nil
}

func (r *resolver) ixnetwork(ateID string) (options, error) {
	ate := r.ate(ateID)
	if ate == nil {
		return options{nil}, fmt.Errorf("ate %q is missing from the binding", ateID)
	}
	targetOptions := &bindpb.Options{Target: ate.Name}
	return merge(targetOptions, r.Options, ate.Options, ate.Ixnetwork), nil
}
