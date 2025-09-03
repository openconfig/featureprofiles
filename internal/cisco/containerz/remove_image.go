// Source: https://github.com/openconfig/containerz/blob/master/client/remove_image.go

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
	"errors"

	cpb "github.com/openconfig/gnoi/containerz"
)

// RemoveImage removes an image from the target system. It returns nil upon success. Otherwise it
// returns an error indicating whether the image was not found or is associated to running
// container.
func (c *Client) RemoveImage(ctx context.Context, image string, tag string, force bool) error {
	resp, err := c.cli.RemoveImage(ctx, &cpb.RemoveImageRequest{
		Name:  image,
		Tag:   tag,
		Force: force,
	})
	if err != nil {
		return err
	}

	switch resp.GetCode() {
	case cpb.RemoveImageResponse_SUCCESS:
		return nil
	case cpb.RemoveImageResponse_NOT_FOUND:
		return ErrNotFound
	case cpb.RemoveImageResponse_RUNNING:
		return ErrRunning
	default:
		return errors.New("unknown error occurred")
	}
}
