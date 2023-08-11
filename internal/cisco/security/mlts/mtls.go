package mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"

	certUtil "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func NewGRPCMTLS(clientCert, clientKey string, username, pass string, server string) (*grpc.ClientConn, error) {
	// read the CA keys from keys/ca and generate it if not found
	var err error
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(certUtil.CACert)

	keyPair, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, err
	}
	tls := &tls.Config{
		Certificates: []tls.Certificate{keyPair},
		RootCAs:      caCertPool,
		ClientCAs:    caCertPool,
	}
	tlsConfig := credentials.NewTLS(tls)
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(tlsConfig))

	c := &creds{username, pass, false}
	opts = append(opts, grpc.WithPerRPCCredentials(c))
	opts = append(opts, grpc.WithBlock())

	mtlsClient, err := grpc.DialContext(context.Background(), server, opts...)
	if err != nil {
		return nil, err
	}
	return mtlsClient, nil

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
