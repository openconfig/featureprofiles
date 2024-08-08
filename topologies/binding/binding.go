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
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/open-traffic-generator/snappi/gosnappi"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnoigo"
	grpb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/grpcutil"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/binding/ixweb"
	opb "github.com/openconfig/ondatra/proto"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// To be stubbed out by unit tests.
	grpcDialContextFn = grpc.DialContext
	gosnappiNewAPIFn  = gosnappi.NewApi
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

var _ binding.Binding = (*staticBind)(nil)

type staticDUT struct {
	*binding.AbstractDUT
	r   resolver
	dev *bindpb.Device
}

var _ introspect.Introspector = (*staticDUT)(nil)

type staticATE struct {
	*binding.AbstractATE
	r      resolver
	dev    *bindpb.Device
	ixweb  *ixweb.IxWeb
	ixsess *ixweb.Session
}

var _ introspect.Introspector = (*staticATE)(nil)

const resvID = "STATIC"

func (b *staticBind) Reserve(ctx context.Context, tb *opb.Testbed, runTime, waitTime time.Duration, partial map[string]string) (*binding.Reservation, error) {
	_ = runTime
	_ = waitTime
	_ = partial
	if b.resv != nil {
		return nil, fmt.Errorf("only one reservation is allowed")
	}
	resv, err := reservation(ctx, tb, b.r)
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

func (d *staticDUT) Dialer(svc introspect.Service) (*introspect.Dialer, error) {
	params, ok := dutSvcParams[svc]
	if !ok {
		return nil, fmt.Errorf("no known DUT service %v", svc)
	}
	bopts := d.r.grpc(d.dev, params)
	return makeDialer(params, bopts)
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
	conn, err := dialConn(ctx, d, introspect.GNMI, opts)
	if err != nil {
		return nil, err
	}
	return gpb.NewGNMIClient(conn), nil
}

func (d *staticDUT) DialGNOI(ctx context.Context, opts ...grpc.DialOption) (gnoigo.Clients, error) {
	conn, err := dialConn(ctx, d, introspect.GNOI, opts)
	if err != nil {
		return nil, err
	}
	return gnoigo.NewClients(conn), nil
}

func (d *staticDUT) DialGNSI(ctx context.Context, opts ...grpc.DialOption) (binding.GNSIClients, error) {
	conn, err := dialConn(ctx, d, introspect.GNSI, opts)
	if err != nil {
		return nil, err
	}
	return gnsiConn{conn: conn}, nil
}

func (d *staticDUT) DialGRIBI(ctx context.Context, opts ...grpc.DialOption) (grpb.GRIBIClient, error) {
	conn, err := dialConn(ctx, d, introspect.GRIBI, opts)
	if err != nil {
		return nil, err
	}
	return grpb.NewGRIBIClient(conn), nil
}

func (d *staticDUT) DialP4RT(ctx context.Context, opts ...grpc.DialOption) (p4pb.P4RuntimeClient, error) {
	conn, err := dialConn(ctx, d, introspect.P4RT, opts)
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

func (a *staticATE) Dialer(svc introspect.Service) (*introspect.Dialer, error) {
	params, ok := ateSvcParams[svc]
	if !ok {
		return nil, fmt.Errorf("no known ATE service %v", svc)
	}
	bopts := a.r.grpc(a.dev, params)
	return makeDialer(params, bopts)
}

func (a *staticATE) DialGNMI(ctx context.Context, opts ...grpc.DialOption) (gpb.GNMIClient, error) {
	conn, err := dialConn(ctx, a, introspect.GNMI, opts)
	if err != nil {
		return nil, err
	}
	return gpb.NewGNMIClient(conn), nil
}

func (a *staticATE) DialOTG(ctx context.Context, opts ...grpc.DialOption) (gosnappi.Api, error) {
	if a.dev.Otg == nil {
		return nil, fmt.Errorf("otg must be configured in ATE binding to run OTG test")
	}
	conn, err := dialConn(ctx, a, introspect.OTG, opts)
	if err != nil {
		return nil, err
	}

	api := gosnappiNewAPIFn()
	transport := api.NewGrpcTransport().SetClientConnection(conn)
	if timeout := a.r.grpc(a.dev, ateSvcParams[introspect.OTG]).Timeout; timeout != 0 {
		transport.SetRequestTimeout(time.Duration(timeout) * time.Second)
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

func reservation(ctx context.Context, tb *opb.Testbed, r resolver) (*binding.Reservation, error) {
	if r.Dynamic {
		return dynamicReservation(ctx, tb, r)
	}
	resv, errs := staticReservation(tb, r)
	return resv, errors.Join(errs...)
}

func staticReservation(tb *opb.Testbed, r resolver) (*binding.Reservation, []error) {
	var errs []error

	bduts := make(map[string]*bindpb.Device)
	for _, bdut := range r.Duts {
		bduts[bdut.Id] = bdut
	}
	bates := make(map[string]*bindpb.Device)
	for _, bate := range r.Ates {
		bates[bate.Id] = bate
	}

	duts := make(map[string]binding.DUT)
	for _, tdut := range tb.Duts {
		bdut, ok := bduts[tdut.Id]
		if !ok {
			errs = append(errs, fmt.Errorf("missing binding for DUT %q", tdut.Id))
			continue
		}
		dims, dimErrs := staticDims(tdut, bdut)
		errs = append(errs, dimErrs...)
		duts[tdut.Id] = &staticDUT{
			AbstractDUT: &binding.AbstractDUT{Dims: dims},
			r:           r,
			dev:         bdut,
		}
	}

	ates := make(map[string]binding.ATE)
	for _, tate := range tb.Ates {
		bate, ok := bates[tate.Id]
		if !ok {
			errs = append(errs, fmt.Errorf("missing binding for ATE %q", tate.Id))
			continue
		}
		dims, dimErrs := staticDims(tate, bate)
		errs = append(errs, dimErrs...)
		ates[tate.Id] = &staticATE{
			AbstractATE: &binding.AbstractATE{Dims: dims},
			r:           r,
			dev:         bate,
		}
	}

	return &binding.Reservation{
		DUTs: duts,
		ATEs: ates,
	}, errs
}

func staticDims(td *opb.Device, bd *bindpb.Device) (*binding.Dims, []error) {
	var errs []error

	// Check that the bound device matches the testbed device.
	if tdVendor := td.GetVendor(); tdVendor != opb.Device_VENDOR_UNSPECIFIED && bd.Vendor != tdVendor {
		errs = append(errs, fmt.Errorf("binding vendor %v and testbed vendor %v do not match", bd.Vendor, tdVendor))
	}
	if tdHardwareModel := td.GetHardwareModel(); tdHardwareModel != "" && bd.HardwareModel != tdHardwareModel {
		errs = append(errs, fmt.Errorf("binding hardware model %v and testbed hardware model %v do not match", bd.HardwareModel, tdHardwareModel))
	}
	if tdSoftwareVersion := td.GetSoftwareVersion(); tdSoftwareVersion != "" && bd.SoftwareVersion != tdSoftwareVersion {
		errs = append(errs, fmt.Errorf("binding software version %v and testbed software version %v do not match", bd.SoftwareVersion, tdSoftwareVersion))
	}

	portmap, portErrs := staticPorts(td.Ports, bd)
	errs = append(errs, portErrs...)

	return &binding.Dims{
		Name:            bd.Name,
		Vendor:          bd.GetVendor(),
		HardwareModel:   bd.GetHardwareModel(),
		SoftwareVersion: bd.GetSoftwareVersion(),
		Ports:           portmap,
	}, errs
}

func staticPorts(tports []*opb.Port, bd *bindpb.Device) (map[string]*binding.Port, []error) {
	var errs []error

	bports := make(map[string]*bindpb.Port)
	for _, bport := range bd.Ports {
		bports[bport.Id] = bport
	}

	portmap := make(map[string]*binding.Port)
	for _, tport := range tports {
		bport, ok := bports[tport.Id]
		if !ok {
			errs = append(errs, fmt.Errorf("missing binding for port %q on %q", tport.Id, bd.Id))
			continue
		}
		if tport.Speed != opb.Port_SPEED_UNSPECIFIED && tport.Speed != bport.Speed {
			errs = append(errs, fmt.Errorf("binding port speed %v and testbed port speed %v do not match", bport.Speed, tport.Speed))
		}
		if tport.GetPmd() != opb.Port_PMD_UNSPECIFIED && tport.GetPmd() != bport.Pmd {
			errs = append(errs, fmt.Errorf("binding port PMD %v and testbed port PMD %v do not match", bport.Pmd, tport.GetPmd()))
		}
		portmap[tport.Id] = &binding.Port{
			Name:  bport.Name,
			PMD:   bport.Pmd,
			Speed: bport.Speed,
		}
	}
	return portmap, errs
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

func dialConn(ctx context.Context, dev introspect.Introspector, svc introspect.Service, opts []grpc.DialOption) (*grpc.ClientConn, error) {
	dialer, err := dev.Dialer(svc)
	if err != nil {
		return nil, err
	}
	return dialer.Dial(ctx, opts...)
}

func dialOpts(bopts *bindpb.Options) ([]grpc.DialOption, error) {
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
		timeout := time.Duration(bopts.Timeout) * time.Second
		retryOpt := grpc_retry.WithPerRetryTimeout(timeout)
		opts = append(opts,
			grpc.WithChainUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpt)),
			grpc.WithChainStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpt)),
			grpcutil.WithUnaryDefaultTimeout(timeout),
			grpcutil.WithStreamDefaultTimeout(timeout),
		)
	}
	return opts, nil
}

func makeDialer(params *svcParams, bopts *bindpb.Options) (*introspect.Dialer, error) {
	opts, err := dialOpts(bopts)
	if err != nil {
		return nil, err
	}
	return &introspect.Dialer{
		DevicePort: params.port,
		DialFunc: func(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
			if bopts.Timeout != 0 {
				var cancelFunc context.CancelFunc
				ctx, cancelFunc = context.WithTimeout(ctx, time.Duration(bopts.Timeout)*time.Second)
				defer cancelFunc()
			}
			return grpcDialContextFn(ctx, target, opts...)
		},
		DialTarget: bopts.Target,
		DialOpts:   opts,
	}, nil
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
