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
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/gnoigo"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/ixweb"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	grpb "github.com/openconfig/gribi/v1/proto/service"
	opb "github.com/openconfig/ondatra/proto"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
)

// staticBind implements the binding.Binding interface by creating a
// static reservation from a binding configuration file and the
// testbed topology.
type staticBind struct {
	binding.Binding
	r          resolver
	resv       *binding.Reservation
	pushConfig bool
}

type staticDUT struct {
	*binding.AbstractDUT
	r   resolver
	dev *bindpb.Device
}

type staticATE struct {
	*binding.AbstractATE
	r      resolver
	dev    *bindpb.Device
	ixweb  *ixweb.IxWeb
	ixsess *ixweb.Session
}

var _ = binding.Binding(&staticBind{})

const resvID = "STATIC"

func (b *staticBind) Reserve(ctx context.Context, tb *opb.Testbed, runTime, waitTime time.Duration, partial map[string]string) (*binding.Reservation, error) {
	_ = runTime
	_ = waitTime
	_ = partial
	if b.resv != nil {
		return nil, fmt.Errorf("only one reservation is allowed")
	}
	resv, err := reservation(tb, b.r)
	if err != nil {
		return nil, err
	}
	resv.ID = resvID
	b.resv = resv

	if b.pushConfig {
		if err := b.reset(ctx); err != nil {
			return nil, err
		}
	}
	if err := b.reserveIxSessions(ctx); err != nil {
		return nil, err
	}
	return resv, nil
}

func (b *staticBind) Release(ctx context.Context) error {
	if b.resv == nil {
		return errors.New("no reservation")
	}
	if err := b.releaseIxSessions(ctx); err != nil {
		return err
	}
	b.resv = nil
	return nil
}

func (b *staticBind) FetchReservation(_ context.Context, id string) (*binding.Reservation, error) {
	_ = id
	return nil, errors.New("static binding does not support fetching an existing reservation")
}

func (b *staticBind) reset(ctx context.Context) error {
	for _, dut := range b.resv.DUTs {
		if sdut, ok := dut.(*staticDUT); ok {
			if err := sdut.reset(ctx); err != nil {
				return fmt.Errorf("could not reset device %s: %w", sdut.Name(), err)
			}
		}
	}
	return nil
}

func (d *staticDUT) reset(ctx context.Context) error {
	// Each of the individual reset functions should be no-op if the reset action is not
	// requested.
	if err := resetCLI(ctx, d); err != nil {
		return err
	}
	if err := resetGNMI(ctx, d); err != nil {
		return err
	}
	return resetGRIBI(ctx, d)
}

func (d *staticDUT) DialGNMI(ctx context.Context, opts ...grpc.DialOption) (gpb.GNMIClient, error) {
	bopts := d.r.gnmi(d.dev)
	conn, err := dialGRPC(ctx, bopts, opts...)
	if err != nil {
		return nil, err
	}
	return gpb.NewGNMIClient(conn), nil
}

func (d *staticDUT) DialGNOI(ctx context.Context, opts ...grpc.DialOption) (gnoigo.Clients, error) {
	bopts := d.r.gnoi(d.dev)
	conn, err := dialGRPC(ctx, bopts, opts...)
	if err != nil {
		return nil, err
	}
	return gnoigo.NewClients(conn), nil
}

func (d *staticDUT) DialGNSI(ctx context.Context, opts ...grpc.DialOption) (binding.GNSIClients, error) {
	bopts := d.r.gnsi(d.dev)
	conn, err := dialGRPC(ctx, bopts, opts...)
	if err != nil {
		return nil, err
	}
	return gnsiConn{conn: conn}, nil
}

func (d *staticDUT) DialGRIBI(ctx context.Context, opts ...grpc.DialOption) (grpb.GRIBIClient, error) {
	bopts := d.r.gribi(d.dev)
	conn, err := dialGRPC(ctx, bopts, opts...)
	if err != nil {
		return nil, err
	}
	return grpb.NewGRIBIClient(conn), nil
}

func (d *staticDUT) DialP4RT(ctx context.Context, opts ...grpc.DialOption) (p4pb.P4RuntimeClient, error) {
	bopts := d.r.p4rt(d.dev)
	conn, err := dialGRPC(ctx, bopts, opts...)
	if err != nil {
		return nil, err
	}
	return p4pb.NewP4RuntimeClient(conn), nil
}

func (d *staticDUT) DialCLI(context.Context) (binding.CLIClient, error) {
	sshOpts := d.r.ssh(d.dev)
	c := &ssh.ClientConfig{
		User: sshOpts.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshOpts.Password),
			ssh.KeyboardInteractive(sshInteractive(sshOpts.Password)),
		},
	}
	if sshOpts.SkipVerify {
		c.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		cb, err := knownHostsCallback()
		if err != nil {
			return nil, err
		}
		c.HostKeyCallback = cb
	}
	sc, err := ssh.Dial("tcp", sshOpts.Target, c)
	if err != nil {
		return nil, err
	}
	return newCLI(sc)
}

// For every question asked in an interactive login ssh session, set the answer to user password.
func sshInteractive(password string) ssh.KeyboardInteractiveChallenge {
	return func(_, _ string, questions []string, _ []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for n := range questions {
			answers[n] = password
		}
		return answers, nil
	}
}

func (a *staticATE) DialGNMI(ctx context.Context, opts ...grpc.DialOption) (gpb.GNMIClient, error) {
	bopts := a.r.ateGNMI(a.dev)
	conn, err := dialGRPC(ctx, bopts, opts...)
	if err != nil {
		return nil, err
	}
	return gpb.NewGNMIClient(conn), nil
}

func (a *staticATE) DialOTG(ctx context.Context, opts ...grpc.DialOption) (gosnappi.GosnappiApi, error) {
	if a.dev.Otg == nil {
		return nil, fmt.Errorf("otg must be configured in ATE binding to run OTG test")
	}
	bopts := a.r.ateOTG(a.dev)
	conn, err := dialGRPC(ctx, bopts, opts...)
	if err != nil {
		return nil, err
	}

	api := gosnappi.NewApi()
	grpcTransport := api.NewGrpcTransport().SetClientConnection(conn)
	if bopts.Timeout != 0 {
		grpcTransport.SetRequestTimeout(time.Duration(bopts.Timeout) * time.Second)
	}
	return api, nil
}

func (a *staticATE) DialIxNetwork(ctx context.Context) (*binding.IxNetwork, error) {
	bopts := a.r.ixnetwork(a.dev)
	ixs, err := a.ixSession(ctx, bopts)
	if err != nil {
		return nil, err
	}
	return &binding.IxNetwork{Session: ixs}, nil
}

// allerrors implements the error interface and will accumulate and
// report all errors.
type allerrors []error

var _ = error(allerrors{})

func (errs allerrors) Error() string {
	// Shortcut for no error or a single error.
	switch len(errs) {
	case 0:
		return ""
	case 1:
		return errs[0].Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d errors occurred:", len(errs))
	for _, err := range errs {
		// Replace indentation for proper nesting.
		fmt.Fprintf(&b, "\n  * %s", strings.ReplaceAll(err.Error(), "\n", "\n    "))
	}
	return b.String()
}

func reservation(tb *opb.Testbed, r resolver) (*binding.Reservation, error) {
	var errs allerrors

	duts := make(map[string]binding.DUT)
	for _, tdut := range tb.Duts {
		bdut := r.dutByID(tdut.Id)
		if bdut == nil {
			errs = append(errs, fmt.Errorf("missing binding for DUT %q", tdut.Id))
			continue
		}
		d, err := dims(tdut, bdut)
		if err != nil {
			errs = append(errs, fmt.Errorf("error binding DUT %q: %w", tdut.Id, err))
			duts[tdut.Id] = nil // mark it "found"
			continue
		}
		duts[tdut.Id] = &staticDUT{
			AbstractDUT: &binding.AbstractDUT{Dims: d},
			r:           r,
			dev:         bdut,
		}
	}
	for _, bdut := range r.Duts {
		if _, ok := duts[bdut.Id]; !ok {
			errs = append(errs, fmt.Errorf("binding DUT %q not found in testbed", bdut.Id))
		}
	}

	ates := make(map[string]binding.ATE)
	for _, tate := range tb.Ates {
		bate := r.ateByID(tate.Id)
		if bate == nil {
			errs = append(errs, fmt.Errorf("missing binding for ATE %q", tate.Id))
			continue
		}
		d, err := dims(tate, bate)
		if err != nil {
			errs = append(errs, fmt.Errorf("error binding ATE %q: %w", tate.Id, err))
			ates[tate.Id] = nil // mark it "found"
			continue
		}
		ates[tate.Id] = &staticATE{
			AbstractATE: &binding.AbstractATE{Dims: d},
			r:           r,
			dev:         bate,
		}
	}
	for _, bate := range r.Ates {
		if _, ok := ates[bate.Id]; !ok {
			errs = append(errs, fmt.Errorf("binding ATE %q not found in testbed", bate.Id))
		}
	}

	if errs != nil {
		return nil, errs
	}

	resv := &binding.Reservation{
		DUTs: duts,
		ATEs: ates,
	}
	return resv, nil
}

func dims(td *opb.Device, bd *bindpb.Device) (*binding.Dims, error) {
	portmap, err := ports(td.Ports, bd.Ports)
	if err != nil {
		return nil, err
	}
	dims := &binding.Dims{
		Name:            bd.Name,
		Vendor:          bd.GetVendor(),
		HardwareModel:   bd.GetHardwareModel(),
		SoftwareVersion: bd.GetSoftwareVersion(),
		Ports:           portmap,
	}
	// Populate empty binding dimensions with testbed dimensions.
	// TODO(prinikasn): Remove testbed override once all vendors are using binding dimensions exclusively.
	if tdVendor := td.GetVendor(); tdVendor != opb.Device_VENDOR_UNSPECIFIED {
		if dims.Vendor != opb.Device_VENDOR_UNSPECIFIED && dims.Vendor != tdVendor {
			return nil, fmt.Errorf("binding vendor %v and testbed vendor %v do not match", dims.Vendor, tdVendor)
		}
		dims.Vendor = tdVendor
	}
	if tdHardwareModel := td.GetHardwareModel(); tdHardwareModel != "" {
		if dims.HardwareModel != "" && dims.HardwareModel != tdHardwareModel {
			return nil, fmt.Errorf("binding hardware model %v and testbed hardware model %v do not match", dims.HardwareModel, tdHardwareModel)
		}
		dims.HardwareModel = tdHardwareModel
	}
	if tdSoftwareVersion := td.GetSoftwareVersion(); tdSoftwareVersion != "" {
		if dims.SoftwareVersion != "" && dims.SoftwareVersion != tdSoftwareVersion {
			return nil, fmt.Errorf("binding software version %v and testbed software version %v do not match", dims.SoftwareVersion, tdSoftwareVersion)
		}
		dims.SoftwareVersion = tdSoftwareVersion
	}

	return dims, nil
}

func ports(tports []*opb.Port, bports []*bindpb.Port) (map[string]*binding.Port, error) {
	var errs allerrors

	portmap := make(map[string]*binding.Port)
	for _, tport := range tports {
		portmap[tport.Id] = &binding.Port{
			Speed: tport.Speed,
		}
	}
	for _, bport := range bports {
		if p, ok := portmap[bport.Id]; ok {
			p.Name = bport.Name
			// If port speed is empty populate from testbed ports.
			if bport.Speed != opb.Port_SPEED_UNSPECIFIED {
				if p.Speed != opb.Port_SPEED_UNSPECIFIED && p.Speed != bport.Speed {
					return nil, fmt.Errorf("binding port speed %v and testbed port speed %v do not match", bport.Speed, p.Speed)
				}
				p.Speed = bport.Speed
			}
			// Populate the PMD type if configured.
			if bport.Pmd != opb.Port_PMD_UNSPECIFIED {
				if p.PMD != opb.Port_PMD_UNSPECIFIED && p.PMD != bport.Pmd {
					return nil, fmt.Errorf("binding port PMD type %v and testbed port PMD type %v do not match", bport.Pmd, p.PMD)
				}
				p.PMD = bport.Pmd
			}
		}
	}
	for id, p := range portmap {
		if p.Name == "" {
			errs = append(errs, fmt.Errorf("testbed port %q is missing in binding", id))
		}
	}

	if errs != nil {
		return nil, errs
	}
	return portmap, nil
}

func (b *staticBind) reserveIxSessions(ctx context.Context) error {
	ates := b.resv.ATEs
	for _, ate := range ates {
		a := ate.(*staticATE)
		if a.dev.Ixnetwork == nil {
			continue
		}
		if _, err := a.DialIxNetwork(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (b *staticBind) releaseIxSessions(ctx context.Context) error {
	for _, ate := range b.resv.ATEs {
		sate := ate.(*staticATE)
		dialer := b.r.ixnetwork(sate.dev)
		if sate.ixsess != nil && dialer.SessionId == 0 {
			if err := sate.ixweb.IxNetwork().DeleteSession(ctx, sate.ixsess.ID()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *staticATE) ixWeb(ctx context.Context, opts *bindpb.Options) (*ixweb.IxWeb, error) {
	if a.ixweb == nil {
		ixw, err := newIxWebClient(ctx, opts)
		if err != nil {
			return nil, err
		}
		a.ixweb = ixw
	}
	return a.ixweb, nil
}

func newIxWebClient(ctx context.Context, opts *bindpb.Options) (*ixweb.IxWeb, error) {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	if opts.SkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	hc := &http.Client{Transport: tr}
	username := opts.GetUsername()
	password := opts.GetPassword()
	if username == "" && password == "" {
		username = "admin"
		password = "admin"
	}
	return ixweb.Connect(ctx, opts.Target, ixweb.WithHTTPClient(hc), ixweb.WithLogin(username, password))
}

func (a *staticATE) ixSession(ctx context.Context, opts *bindpb.Options) (*ixweb.Session, error) {
	if a.ixsess == nil {
		ixw, err := a.ixWeb(ctx, opts)
		if err != nil {
			return nil, err
		}
		if opts.SessionId > 0 {
			a.ixsess, err = ixw.IxNetwork().FetchSession(ctx, int(opts.SessionId))
		} else {
			a.ixsess, err = ixw.IxNetwork().NewSession(ctx, a.Name())
		}
		if err != nil {
			return nil, err
		}
	}
	return a.ixsess, nil
}

func dialGRPC(ctx context.Context, bopts *bindpb.Options, overrideOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithBlock()}
	switch {
	case bopts.Insecure:
		tc := insecure.NewCredentials()
		opts = append(opts, grpc.WithTransportCredentials(tc))
	case bopts.SkipVerify:
		tc := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
		opts = append(opts, grpc.WithTransportCredentials(tc))
	case bopts.MutualTls:
		trusBundle, keyPair, err := loadCertificates(bopts)
		if err != nil {
			return nil, err
		}
		tls := &tls.Config{
			Certificates: []tls.Certificate{keyPair},
			RootCAs:      trusBundle,
		}
		tlsConfig := credentials.NewTLS(tls)
		opts = append(opts, grpc.WithTransportCredentials(tlsConfig))
	}
	if bopts.Username != "" {
		c := &creds{bopts.Username, bopts.Password, !bopts.Insecure}
		opts = append(opts, grpc.WithPerRPCCredentials(c))
	}
	if bopts.MaxRecvMsgSize != 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(bopts.MaxRecvMsgSize))))
	}
	if bopts.Timeout != 0 {
		retryOpt := grpc_retry.WithPerRetryTimeout(time.Duration(bopts.Timeout) * time.Second)
		opts = append(opts,
			grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpt)),
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpt)),
		)
		var cancelFunc context.CancelFunc
		ctx, cancelFunc = context.WithTimeout(ctx, time.Duration(bopts.Timeout)*time.Second)
		defer cancelFunc()
	}
	opts = append(opts, overrideOpts...)
	return grpc.DialContext(ctx, bopts.Target, opts...)
}

// load trust bundle and client key and certificate
func loadCertificates(bopts *bindpb.Options) (*x509.CertPool, tls.Certificate, error) {
	if bopts.CertFile == "" || bopts.KeyFile == "" || bopts.TrustBundleFile == "" {
		return nil, tls.Certificate{}, fmt.Errorf("cert_file, key_file, and trust_bundle_file need to be set when mutual tls is set")
	}
	caCertBytes, err := os.ReadFile(bopts.TrustBundleFile)
	if err != nil {
		return nil, tls.Certificate{}, err
	}
	trusBundle := x509.NewCertPool()
	if !trusBundle.AppendCertsFromPEM(caCertBytes) {
		return nil, tls.Certificate{}, fmt.Errorf("error in loading ca trust bundle")
	}
	keyPair, err := tls.LoadX509KeyPair(bopts.CertFile, bopts.KeyFile)
	if err != nil {
		return nil, tls.Certificate{}, err
	}
	return trusBundle, keyPair, nil
}

// creds implements the grpc.PerRPCCredentials interface, to be used
// as a grpc.DialOption in dialGRPC.
type creds struct {
	username, password string
	secure             bool
}

func (c *creds) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"username": c.username,
		"password": c.password,
	}, nil
}

func (c *creds) RequireTransportSecurity() bool {
	return c.secure
}

var _ = grpc.PerRPCCredentials(&creds{})

var knownHostsFiles = []string{
	"$HOME/.ssh/known_hosts",
	"/etc/ssh/ssh_known_hosts",
}

// knownHostsCallback checks the user and system SSH known_hosts.
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
