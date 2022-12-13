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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/ixweb"
	"google.golang.org/grpc"

	"github.com/openconfig/featureprofiles/internal/rundata"
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
	if b.resv != nil {
		return nil, fmt.Errorf("only one reservation is allowed")
	}
	resv, err := reservation(tb, b.r)
	if err != nil {
		return nil, err
	}
	resv.ID = resvID
	b.resv = resv

	if err := b.afterReserve(ctx); err != nil {
		return nil, err
	}
	if err := b.reserveIxSessions(ctx); err != nil {
		return nil, err
	}
	return resv, nil
}

func (b *staticBind) Release(ctx context.Context) error {
	m := rundata.Timing(ctx)
	for k, v := range m {
		ondatra.Report().AddSuiteProperty(k, v)
	}
	if b.resv == nil {
		return errors.New("no reservation")
	}
	if err := b.releaseIxSessions(ctx); err != nil {
		return err
	}
	b.resv = nil
	return nil
}

func (b *staticBind) FetchReservation(ctx context.Context, id string) (*binding.Reservation, error) {
	if b.resv == nil || id != resvID {
		return nil, fmt.Errorf("reservation not found: %s", id)
	}
	if err := b.afterReserve(ctx); err != nil {
		return nil, err
	}
	return b.resv, nil
}

func (b *staticBind) afterReserve(ctx context.Context) error {
	m := rundata.Properties(ctx, b.resv)
	for k, v := range m {
		ondatra.Report().AddSuiteProperty(k, v)
	}

	if !b.pushConfig {
		return nil
	}
	return b.reset(ctx)
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
	if err := resetCLI(ctx, d.dev, d.r); err != nil {
		return err
	}
	if err := resetGNMI(ctx, d.dev, d.r); err != nil {
		return err
	}
	if err := resetGRIBI(ctx, d.dev, d.r); err != nil {
		return err
	}
	return nil
}

func (d *staticDUT) DialGNMI(ctx context.Context, opts ...grpc.DialOption) (gpb.GNMIClient, error) {
	dialer, err := d.r.gnmi(d.Name())
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return gpb.NewGNMIClient(conn), nil
}

func (d *staticDUT) DialGNOI(ctx context.Context, opts ...grpc.DialOption) (binding.GNOIClients, error) {
	dialer, err := d.r.gnoi(d.Name())
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return gnoiConn{conn: conn}, nil
}

func (d *staticDUT) DialGRIBI(ctx context.Context, opts ...grpc.DialOption) (grpb.GRIBIClient, error) {
	dialer, err := d.r.gribi(d.Name())
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return grpb.NewGRIBIClient(conn), nil
}

func (d *staticDUT) DialP4RT(ctx context.Context, opts ...grpc.DialOption) (p4pb.P4RuntimeClient, error) {
	dialer, err := d.r.p4rt(d.Name())
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return p4pb.NewP4RuntimeClient(conn), nil
}

func (d *staticDUT) DialCLI(ctx context.Context) (binding.StreamClient, error) {
	dialer, err := d.r.ssh(d.Name())
	if err != nil {
		return nil, err
	}
	sc, err := dialer.dialSSH()
	if err != nil {
		return nil, err
	}
	return newCLI(sc)
}

func (a *staticATE) DialGNMI(ctx context.Context, opts ...grpc.DialOption) (gpb.GNMIClient, error) {
	dialer, err := a.r.ateGNMI(a.Name())
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return gpb.NewGNMIClient(conn), nil
}

func (a *staticATE) DialOTG(ctx context.Context, opts ...grpc.DialOption) (gosnappi.GosnappiApi, error) {
	if a.dev.Otg == nil {
		return nil, fmt.Errorf("otg must be configured in ATE binding to run OTG test")
	}
	dialer, err := a.r.ateOtg(a.Name())
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}

	api := gosnappi.NewApi()
	grpcTransport := api.NewGrpcTransport().SetClientConnection(conn)
	if dialer.Timeout != 0 {
		grpcTransport.SetRequestTimeout(time.Duration(dialer.Timeout) * time.Second)
	}
	return api, nil
}

func (a *staticATE) DialIxNetwork(ctx context.Context) (*binding.IxNetwork, error) {
	dialer, err := a.r.ixnetwork(a.Name())
	if err != nil {
		return nil, err
	}
	ixs, err := a.ixSession(ctx, dialer)
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
	return &binding.Dims{
		Name:            bd.Name,
		Vendor:          td.Vendor,
		HardwareModel:   td.HardwareModel,
		SoftwareVersion: td.SoftwareVersion,
		Ports:           portmap,
	}, nil
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
		p, ok := portmap[bport.Id]
		if !ok {
			errs = append(errs, fmt.Errorf("binding port %q not found in testbed", bport.Id))
			continue
		}
		p.Name = bport.Name
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

		bate := b.r.ateByName(ate.Name())
		if bate == nil {
			return fmt.Errorf("missing binding for ATE %q", bate.Id)
		}
		if bate.Ixnetwork == nil {
			continue
		}

		dialer, err := b.r.ixnetwork(ate.Name())
		if err != nil {
			return err
		}
		if _, err := ate.(*staticATE).ixSession(ctx, dialer); err != nil {
			return err
		}
	}
	return nil
}

func (b *staticBind) releaseIxSessions(ctx context.Context) error {
	for _, ate := range b.resv.ATEs {
		dialer, err := b.r.ixnetwork(ate.Name())
		if err != nil {
			return err
		}
		sate := ate.(*staticATE)
		if sate.ixsess != nil && dialer.SessionId == 0 {
			if err := sate.ixweb.IxNetwork().DeleteSession(ctx, sate.ixsess.ID()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *staticATE) ixWeb(ctx context.Context, d dialer) (*ixweb.IxWeb, error) {
	if a.ixweb == nil {
		ixw, err := d.newIxWebClient(ctx)
		if err != nil {
			return nil, err
		}
		a.ixweb = ixw
	}
	return a.ixweb, nil
}

func (a *staticATE) ixSession(ctx context.Context, d dialer) (*ixweb.Session, error) {
	if a.ixsess == nil {
		ixw, err := a.ixWeb(ctx, d)
		if err != nil {
			return nil, err
		}
		if d.SessionId > 0 {
			a.ixsess, err = ixw.IxNetwork().FetchSession(ctx, int(d.SessionId))
		} else {
			a.ixsess, err = ixw.IxNetwork().NewSession(ctx, a.Name())
		}
		if err != nil {
			return nil, err
		}
	}
	return a.ixsess, nil
}
