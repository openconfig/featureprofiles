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
	"log"
	"strings"
	"sync"
	"time"

	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/ixweb"
	"google.golang.org/grpc"

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
	r            resolver
	resv         *binding.Reservation
	muIxWeb      sync.Mutex
	muIxSession  sync.Mutex
	ateIxWeb     map[string]*ixweb.IxWeb
	ateIxSession map[string]*ixweb.Session
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

	if err := b.reserveATE(ctx); err != nil {
		return nil, err
	}
	return resv, nil
}

func (b *staticBind) Release(ctx context.Context) error {
	if b.resv == nil {
		return errors.New("no reservation")
	}
	if err := b.releaseATE(ctx); err != nil {
		return err
	}
	b.resv = nil
	return nil
}

func (b *staticBind) FetchReservation(ctx context.Context, id string) (*binding.Reservation, error) {
	if b.resv == nil || id != resvID {
		return nil, fmt.Errorf("reservation not found: %s", id)
	}
	return b.resv, nil
}

func (b *staticBind) PushConfig(ctx context.Context, dut *binding.DUT, config string, reset bool) error {
	// If really needed, implement this using SSH cli.
	return errors.New("featureprofiles tests should use gNMI, not PushConfig")
}

func (b *staticBind) DialGNMI(ctx context.Context, dut *binding.DUT, opts ...grpc.DialOption) (gpb.GNMIClient, error) {
	dialer, err := b.r.gnmi(dut.Name)
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return gpb.NewGNMIClient(conn), nil
}

func (b *staticBind) DialGNOI(ctx context.Context, dut *binding.DUT, opts ...grpc.DialOption) (binding.GNOIClients, error) {
	dialer, err := b.r.gnoi(dut.Name)
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return gnoiConn{conn}, nil
}

func (b *staticBind) DialGRIBI(ctx context.Context, dut *binding.DUT, opts ...grpc.DialOption) (grpb.GRIBIClient, error) {
	dialer, err := b.r.gribi(dut.Name)
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return grpb.NewGRIBIClient(conn), nil
}

func (b *staticBind) DialP4RT(ctx context.Context, dut *binding.DUT, opts ...grpc.DialOption) (p4pb.P4RuntimeClient, error) {
	dialer, err := b.r.p4rt(dut.Name)
	if err != nil {
		return nil, err
	}
	conn, err := dialer.dialGRPC(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return p4pb.NewP4RuntimeClient(conn), nil
}

func (b *staticBind) DialConsole(ctx context.Context, dut *binding.DUT, opts ...grpc.DialOption) (binding.StreamClient, error) {
	// If needed, we can implement this by assuming the console is
	// accessible as a local character special device such as ttyS0 or a
	// pty.  Please file a feature request to discuss the use case.
	return nil, errors.New("console is not supported yet by the static binding")
}

func (b *staticBind) DialCLI(ctx context.Context, dut *binding.DUT, opts ...grpc.DialOption) (binding.StreamClient, error) {
	dialer, err := b.r.ssh(dut.Name)
	if err != nil {
		return nil, err
	}
	sc, err := dialer.dialSSH()
	if err != nil {
		return nil, err
	}
	return newCLI(sc)
}

func (b *staticBind) DialIxNetwork(ctx context.Context, ate *binding.ATE) (*binding.IxNetwork, error) {
	dialer, err := b.r.ixnetwork(ate.Name)
	if err != nil {
		return nil, err
	}
	ixs, err := b.ixSessionForATE(ctx, ate.Name, dialer)
	if err != nil {
		return nil, err
	}
	return &binding.IxNetwork{Session: ixs}, nil
}

func (b *staticBind) HandleInfraFail(err error) error {
	log.Printf("Infrastructure failure: %v", err)
	return err
}

func (b *staticBind) SetTestMetadata(md *binding.TestMetadata) error {
	return nil // Unimplemented.
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

	duts := make(map[string]*binding.DUT)
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
		duts[tdut.Id] = &binding.DUT{Dims: d}
	}
	for _, bdut := range r.Duts {
		if _, ok := duts[bdut.Id]; !ok {
			errs = append(errs, fmt.Errorf("binding DUT %q not found in testbed", bdut.Id))
		}
	}

	ates := make(map[string]*binding.ATE)
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
		ates[tate.Id] = &binding.ATE{Dims: d}
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

func (b *staticBind) reserveATE(ctx context.Context) error {
	ates := b.resv.ATEs
	for _, ate := range ates {
		dialer, err := b.r.ixnetwork(ate.Name)
		if err != nil {
			return err
		}
		if _, err := b.ixSessionForATE(ctx, ate.Name, dialer); err != nil {
			return err
		}
	}
	return nil
}

func (b *staticBind) releaseATE(ctx context.Context) error {
	for ateName, ixw := range b.ateIxWeb {
		dialer, err := b.r.ixnetwork(ateName)
		if err != nil {
			return err
		}
		ixs, ok := b.ateIxSession[ateName]
		if ok && dialer.SessionId == 0 {
			if err := ixw.IxNetwork().DeleteSession(ctx, ixs.ID()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *staticBind) ixWebForATE(ctx context.Context, ateName string, d dialer) (*ixweb.IxWeb, error) {
	b.muIxWeb.Lock()
	defer b.muIxWeb.Unlock()

	if b.ateIxWeb == nil {
		b.ateIxWeb = make(map[string]*ixweb.IxWeb)
	}

	var err error
	ixw, ok := b.ateIxWeb[ateName]
	if !ok {
		ixw, err = d.newIxWebClient(ctx)
		if err != nil {
			return nil, err
		}
		b.ateIxWeb[ateName] = ixw
	}
	return ixw, nil
}

func (b *staticBind) ixSessionForATE(ctx context.Context, ateName string, d dialer) (*ixweb.Session, error) {
	b.muIxSession.Lock()
	defer b.muIxSession.Unlock()

	if b.ateIxSession == nil {
		b.ateIxSession = make(map[string]*ixweb.Session)
	}

	ixw, err := b.ixWebForATE(ctx, ateName, d)
	if err != nil {
		return nil, err
	}
	ixs, ok := b.ateIxSession[ateName]
	if !ok {
		var ixs_err error
		if d.SessionId > 0 {
			ixs, ixs_err = ixw.IxNetwork().FetchSession(ctx, int(d.SessionId))
		} else {
			ixs, ixs_err = ixw.IxNetwork().NewSession(ctx, ateName)
		}
		if ixs_err != nil {
			return nil, ixs_err
		}
		b.ateIxSession[ateName] = ixs
	}
	return ixs, nil
}
