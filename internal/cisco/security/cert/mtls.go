package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	//"net/http"
	//"net/http/httptest"
	//"strings"
	//"time"
)


func main() {
	// read the CA keys from keys/ca and generate it if not found
	var err error
	caPrivateKey,caCert,err= loadRootCA(); if err!=nil {
			panic("Could load the CA keys")
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)
	
	keyPair, err := tls.LoadX509KeyPair("keys/nodes/cafy_auto.cert.pem", "keys/nodes/cafy_auto.key.pem"); if err!=nil {
		panic("Could load the client keys")
	}
	tls := &tls.Config{
		Certificates: []tls.Certificate{keyPair},
		RootCAs:      caCertPool,
		ClientCAs: caCertPool,
	}
	tlsConfig:=credentials.NewTLS(tls)
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(tlsConfig))
	

	c := &creds{"cafyauto", "cisco123",false}
	opts = append(opts, grpc.WithPerRPCCredentials(c))
	opts = append(opts, grpc.WithBlock())

	_,err=grpc.DialContext(context.Background(), "10.85.84.159:47402", opts...); if err!=nil {
		panic("could not establish connection")
	}


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


