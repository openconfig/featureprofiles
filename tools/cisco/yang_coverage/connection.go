package yang_coverage

import (
	"testing"
	"context"
	"fmt"
	"time"
	"crypto/tls"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	ciscobind "github.com/openconfig/featureprofiles/topologies/cisco/binding"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

var gnoiPort = 9339

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

// merge creates a dialer by combining one or more options.
func merge(bopts ...*bindpb.Options) *bindpb.Options {
	result := &bindpb.Options{}
	for _, bopt := range bopts {
		if bopt != nil {
			proto.Merge(result, bopt)
		}
	}
	return result
}

// resolver implements methods to access Binding for specific devices and protocols.
type resolver struct {
	b *bindpb.Binding
}

// dutByName looks up the *bindpb.Device with the given name.
func (r *resolver) dutByName(dutId string) *bindpb.Device {
	for _, dut := range r.b.Duts {
		if dut.Id == dutId {
			return dut
		}
	}
	return nil
}
// dutDialer reconstructs the dialer for a given dut and protocol.
func (r *resolver) dutDialer(dutId string, port int, optionsFn func(*bindpb.Device) *bindpb.Options) (*bindpb.Options, error) {
	dut := r.dutByName(dutId)
	if dut == nil {
		return nil, fmt.Errorf("dut name %q is missing from the binding", dutId)
	}
	targetOptions := &bindpb.Options{
		Target: fmt.Sprintf("%s:%d", dut.Name, port),
	}
	return merge(targetOptions, r.b.Options, dut.Options, optionsFn(dut)), nil
}

func (r *resolver) gnoi(dutId string) (*bindpb.Options, error) {
	return r.dutDialer(dutId, gnoiPort,
		func(dut *bindpb.Device) *bindpb.Options { return dut.Gnoi })
}

// dialGRPC dials a gRPC connection using the binding options.
func (r *resolver) dialGRPC(dutId string, ctx context.Context, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	d, err := r.gnoi(dutId)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	switch {
	case d.Insecure:
		tc := insecure.NewCredentials()
		opts = append(opts, grpc.WithTransportCredentials(tc))
	case d.SkipVerify:
		tc := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
		opts = append(opts, grpc.WithTransportCredentials(tc))
	}
	if d.Username != "" {
		fmt.Println("inside username")
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

func DialGNOI(ctx context.Context, dutId string, t *testing.T) (*grpc.ClientConn, error){
	b := ciscobind.GetBinding(t)
	if b != nil {
		r := &resolver{b}
		return r.dialGRPC(dutId, ctx)
	}
	return nil, fmt.Errorf("failed") 
}

