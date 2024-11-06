// Source https://github.com/openconfig/containerz/blob/master/client/create_volume.go

package client

import (
	"context"
	"fmt"
	"strings"

	cpb "github.com/openconfig/gnoi/containerz"
)

// CreateVolume creates a volume. If the name is empty, the target system will create one. The
// driver default to the target defined option.
func (c *Client) CreateVolume(ctx context.Context, name, driver string, labels, options map[string]string) (string, error) {
	req, err := requestForDriver(driver, options)
	if err != nil {
		return "", err
	}

	req.Name = name
	req.Labels = labels

	resp, err := c.cli.CreateVolume(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.GetName(), nil
}

// requestForDriver returns a CreateVolumeRequest given the string representation of the driver name
// and an arbitrary list of key-value entries representing options of the driver. 
func requestForDriver(driver string, options map[string]string) (*cpb.CreateVolumeRequest, error) {
	req := &cpb.CreateVolumeRequest{}
	switch strings.ToLower(driver) {
	case "local", "":
		req.Driver = cpb.Driver_DS_LOCAL
		localOpts := &cpb.LocalDriverOptions{}
		for key, value := range options {
			switch strings.ToLower(key) {
			case "type":
				switch strings.ToLower(value) {
				case "none", "":
					localOpts.Type = cpb.LocalDriverOptions_TYPE_NONE
				default:
					return nil, fmt.Errorf("invalid type: %q", value)
				}
			case "options":
				localOpts.Options = strings.Split(value, ",")
			case "mountpoint":
				localOpts.Mountpoint = value
			default:
				return nil, fmt.Errorf("invalid key: %q", key)
			}
		}
		req.Options = &cpb.CreateVolumeRequest_LocalMountOptions{
			LocalMountOptions: localOpts,
		}
	default:
		return nil, fmt.Errorf("unknown driver: %q", driver)
	}
	return req, nil
}
