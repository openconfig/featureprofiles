// Source: https://github.com/openconfig/containerz/blob/master/client/remove_volume.go

package client

import (
	"context"

	cpb "github.com/openconfig/gnoi/containerz"
)

func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	req := &cpb.RemoveVolumeRequest{
		Name:  name,
		Force: force,
	}

	_, err := c.cli.RemoveVolume(ctx, req)
	return err
}
