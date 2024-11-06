// Source: https://github.com/openconfig/containerz/blob/master/client/stop_container.go

// Copyright 2023 Google LLC
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

package client

import (
	"context"

	cpb "github.com/openconfig/gnoi/containerz"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StopContainer stops the requested instance. Stop can also force termination.
func (c *Client) StopContainer(ctx context.Context, instance string, force bool, restart bool) error {
	resp, err := c.cli.StopContainer(ctx, &cpb.StopContainerRequest{
		InstanceName: instance,
		Force:        force,
		Restart:      restart,
	})
	if err != nil {
		return err
	}
	if resp != nil {
		errorCode := resp.GetCode().String()
		return status.Errorf(codes.Internal, "Failed to stop container: %s (Error Code: %s)", resp.GetDetails(), errorCode)
	}

	return nil
}
