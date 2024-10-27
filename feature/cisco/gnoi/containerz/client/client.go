package client

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	cpb "github.com/openconfig/gnoi/containerz"
)

// Client is a grpc containerz client.
type Client struct {
	cli cpb.ContainerzClient
}

// creds holds the username and password for basic authentication.
type creds struct {
	Username string
	Password string
}

// GetRequestMetadata is needed by credentials.PerRPCCredentials.
func (c *creds) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": c.Username,
		"password": c.Password,
	}, nil
}

// RequireTransportSecurity is needed by credentials.PerRPCCredentials.
func (c *creds) RequireTransportSecurity() bool {
	return false
}

// NewClient builds a new containerz client.
func NewClient(ctx context.Context, addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(&creds{"cisco", "cisco123"}))
	if err != nil {
		return nil, err
	}

	return &Client{
		cli: cpb.NewContainerzClient(conn),
	}, nil
}
