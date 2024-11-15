// Source: https://github.com/openconfig/containerz/blob/master/client/list_image.go

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
	"io"

	cpb "github.com/openconfig/gnoi/containerz"
	"k8s.io/klog/v2"
)

// ListImage implements the client logic for listing the existing images on the target system.
func (c *Client) ListImage(ctx context.Context, limit int32, filter map[string][]string) (<-chan *ImageInfo, error) {
	req := &cpb.ListImageRequest{
		Limit:  limit,
		Filter: toImageFilter(filter),
	}

	dcli, err := c.cli.ListImage(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *ImageInfo, 100)
	go func() {
		defer dcli.CloseSend()
		defer close(ch)
		for {
			msg, err := dcli.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}
				nonBlockingChannelSend(ctx, ch, &ImageInfo{
					Error: err,
				})
				return
			}

			if nonBlockingChannelSend(ctx, ch, &ImageInfo{
				ID:        msg.GetId(),
				ImageName: msg.GetImageName(),
				ImageTag:  msg.GetTag(),
			}) {
				klog.Warningf("operation cancelled; returning")
				return
			}
		}
	}()

	return ch, nil
}

func toImageFilter(m map[string][]string) []*cpb.ListImageRequest_Filter {
	// TODO(alshabib) implement this when filter field becomes a repeated.
	return nil
}
