// Source: https://github.com/openconfig/containerz/blob/master/client/list_volume.go

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

// ListVolume implements the client logic for listing the existing volumes on the target system.
func (c *Client) ListVolume(ctx context.Context, filter map[string][]string) (<-chan *VolumeInfo, error) {
	req := &cpb.ListVolumeRequest{
		Filter: toVolumeFilter(filter),
	}

	dcli, err := c.cli.ListVolume(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *VolumeInfo, 100)
	go func() {
		defer dcli.CloseSend()
		defer close(ch)
		for {
			msg, err := dcli.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}
				nonBlockingChannelSend(ctx, ch, &VolumeInfo{
					Error: err,
				})
				return
			}

			if nonBlockingChannelSend(ctx, ch, &VolumeInfo{
				Name:         msg.GetName(),
				Driver:       msg.GetDriver(),
				Labels:       msg.GetLabels(),
				Options:      msg.GetOptions(),
				CreationTime: msg.GetCreated().AsTime(),
			}) {
				klog.Warningf("operation cancelled; returning")
				return
			}
		}
	}()

	return ch, nil
}

func toVolumeFilter(m map[string][]string) []*cpb.ListVolumeRequest_Filter {
	return nil
}
