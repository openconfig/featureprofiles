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

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/binding/ixweb"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// IANA assigns 9339 for gNxI, and 9559 for P4RT.  There hasn't been a
// port assignment for gRIBI, so using Arista's default which is 6040.
var (
	gnmiPort    = flag.Int("gnmi_port", 9339, "default gNMI port")
	gnoiPort    = flag.Int("gnoi_port", 9339, "default gNOI port")
	gnsiPort    = flag.Int("gnsi_port", 9339, "default gNSI port")
	gribiPort   = flag.Int("gribi_port", 6040, "default gRIBI port")
	p4rtPort    = flag.Int("p4rt_port", 9559, "default P4RT part")
	ateGnmiPort = flag.Int("ate_gnmi_port", 50051, "default ATE gNMI port")
	ateOtgPort  = flag.Int("ate_grpc_port", 40051, "default ATE gRPC port for running OTG test")
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

// dialer wraps *bindpb.Options and implements dialers for various
// protocols.
type dialer struct {
	*bindpb.Options
}

// dialGRPC dials a gRPC connection using the binding options.
//
//lint:ignore U1000 will be used by the binding.
func (d *dialer) dialGRPC(ctx context.Context, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	switch {
	case d.Insecure:
		tc := insecure.NewCredentials()
		opts = append(opts, grpc.WithTransportCredentials(tc))
	case d.SkipVerify:
		tc := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
		opts = append(opts, grpc.WithTransportCredentials(tc))
	}
	if d.Username != "" {
		c := &creds{d.Username, d.Password, !d.Insecure}
		opts = append(opts, grpc.WithPerRPCCredentials(c))
	}
	if d.Timeout == 0 {
		return grpc.DialContext(ctx, d.Target, opts...)
	}
	retryOpt := grpc_retry.WithPerRetryTimeout(time.Duration(d.Timeout) * time.Second)
	opts = append(opts,
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpt)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpt)),
	)
	ctx, cancelFunc := context.WithTimeout(ctx, time.Duration(d.Timeout)*time.Second)
	defer cancelFunc()
	return grpc.DialContext(ctx, d.Target, opts...)
}

var knownHostsFiles = []string{
	"$HOME/.ssh/known_hosts",
	"/etc/ssh/ssh_known_hosts",
}

// knownHostsCallback checks the user and system SSH known_hosts.
//
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
//
//lint:ignore U1000 will be used by the binding.
func (d *dialer) dialSSH() (*ssh.Client, error) {
	c := &ssh.ClientConfig{
		User: d.Username,
		Auth: []ssh.AuthMethod{ssh.Password(d.Password)},
	}
	if d.SkipVerify {
		c.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		cb, err := knownHostsCallback()
		if err != nil {
			return nil, err
		}
		c.HostKeyCallback = cb
	}
	return ssh.Dial("tcp", d.Target, c)
}

// newHTTPClient makes an http.Client using the binding options.
//
//lint:ignore U1000 will be used by the binding.
func (d *dialer) newHTTPClient() *http.Client {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	if d.SkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{Transport: tr}
}

// newIxWebClient makes an IxWeb session using the binding options.
func (d *dialer) newIxWebClient(ctx context.Context) (*ixweb.IxWeb, error) {
	hc := d.newHTTPClient()
	username := d.GetUsername()
	password := d.GetPassword()
	if username == "" && password == "" {
		username = "admin"
		password = "admin"
	}
	ixw, err := ixweb.Connect(ctx, d.Target, ixweb.WithHTTPClient(hc), ixweb.WithLogin(username, password))
	if err != nil {
		return nil, err
	}
	return ixw, nil
}

// merge creates a dialer by combining one or more options.
func merge(bopts ...*bindpb.Options) dialer {
	result := &bindpb.Options{}
	for _, bopt := range bopts {
		if bopt != nil {
			proto.Merge(result, bopt)
		}
	}
	return dialer{result}
}

// resolver returns the dialer for specific devices and protocols.
type resolver struct {
	*bindpb.Binding
}

// dutByID looks up the *bindpb.Device with the given dutID.
func (r *resolver) dutByID(dutID string) *bindpb.Device {
	for _, dut := range r.Duts {
		if dut.Id == dutID {
			return dut
		}
	}
	return nil
}

// ateByID looks up the *bindpb.Device with the given ateID.
func (r *resolver) ateByID(ateID string) *bindpb.Device {
	for _, ate := range r.Ates {
		if ate.Id == ateID {
			return ate
		}
	}
	return nil
}

// dutByName looks up the *bindpb.Device with the given name.
func (r *resolver) dutByName(dutName string) *bindpb.Device {
	for _, dut := range r.Duts {
		if dut.Name == dutName {
			return dut
		}
	}
	return nil
}

// ateByName looks up the *bindpb.Device with the given name.
func (r *resolver) ateByName(ateName string) *bindpb.Device {
	for _, ate := range r.Ates {
		if ate.Name == ateName {
			return ate
		}
	}
	return nil
}

// dutDialer reconstructs the dialer for a given dut and protocol.
func (r *resolver) dutDialer(dutName string, port int, optionsFn func(*bindpb.Device) *bindpb.Options) (dialer, error) {
	dut := r.dutByName(dutName)
	if dut == nil {
		return dialer{nil}, fmt.Errorf("dut name %q is missing from the binding", dutName)
	}
	targetOptions := &bindpb.Options{
		Target: fmt.Sprintf("%s:%d", dut.Name, port),
	}
	return merge(targetOptions, r.Options, dut.Options, optionsFn(dut)), nil
}

func (r *resolver) ateDialer(ateName string, port int, optionsFn func(*bindpb.Device) *bindpb.Options) (dialer, error) {
	ate := r.ateByName(ateName)
	if ate == nil {
		return dialer{nil}, fmt.Errorf("ATE name %q is missing from the binding", ateName)
	}
	targetOptions := &bindpb.Options{
		Target: fmt.Sprintf("%s:%d", ate.Name, port),
	}
	return merge(targetOptions, r.Options, ate.Options, optionsFn(ate)), nil
}

func (r *resolver) gnmi(dutName string) (dialer, error) {
	return r.dutDialer(dutName, *gnmiPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.Gnmi })
}

func (r *resolver) gnoi(dutName string) (dialer, error) {
	return r.dutDialer(dutName, *gnoiPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.Gnoi })
}

func (r *resolver) gnsi(dutName string) (dialer, error) {
	return r.dutDialer(dutName, *gnsiPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.Gnsi })
}

func (r *resolver) gribi(dutName string) (dialer, error) {
	return r.dutDialer(dutName, *gribiPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.Gribi })
}

func (r *resolver) p4rt(dutName string) (dialer, error) {
	return r.dutDialer(dutName, *p4rtPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.P4Rt })
}

func (r *resolver) ssh(dutName string) (dialer, error) {
	dut := r.dutByName(dutName)
	if dut == nil {
		return dialer{nil}, fmt.Errorf("dut name %q is missing from the binding", dutName)
	}
	targetOptions := &bindpb.Options{Target: dut.Name}
	return merge(targetOptions, r.Options, dut.Options, dut.Ssh), nil
}

func (r *resolver) ateGNMI(ateName string) (dialer, error) {
	return r.ateDialer(ateName, *ateGnmiPort,
		func(ate *bindpb.Device) *bindpb.Options { return ate.Gnmi })
}

func (r *resolver) ateOtg(ateName string) (dialer, error) {
	return r.ateDialer(ateName, *ateOtgPort,
		func(ate *bindpb.Device) *bindpb.Options { return ate.Otg })
}

func (r *resolver) ixnetwork(ateName string) (dialer, error) {
	ate := r.ateByName(ateName)
	if ate == nil {
		return dialer{nil}, fmt.Errorf("ate name %q is missing from the binding", ateName)
	}
	targetOptions := &bindpb.Options{Target: ate.Name}
	return merge(targetOptions, r.Options, ate.Options, ate.Ixnetwork), nil
}
