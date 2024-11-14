// Source: https://github.com/openconfig/containerz/blob/master/client/list_container.go

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
	"fmt"
	cpb "github.com/openconfig/gnoi/containerz"
	"io"
	"k8s.io/klog/v2"
	"strings"
)

// ListContainer implements the client logic for listing the existing containers on the target system.
func (c *Client) ListContainer(ctx context.Context, all bool, limit int32, filter []string) (<-chan *ContainerInfo, error) {
	filters, err := filters(filter)
	if err != nil {
		return nil, err
	}
	req := &cpb.ListContainerRequest{
		All:    all,
		Limit:  limit,
		Filter: filters,
	}

	dcli, err := c.cli.ListContainer(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *ContainerInfo, 100)
	go func() {
		defer dcli.CloseSend()
		defer close(ch)
		for {
			msg, err := dcli.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}
				nonBlockingChannelSend(ctx, ch, &ContainerInfo{
					Error: err,
				})
				return
			}

			if nonBlockingChannelSend(ctx, ch, &ContainerInfo{
				ID:        msg.GetId(),
				Name:      msg.GetName(),
				ImageName: msg.GetImageName(),
				State:     msg.GetStatus().String(),
			}) {
				klog.Warningf("operation cancelled; returning")
				return
			}
		}
	}()

	return ch, nil
}
func filters(filters []string) ([]*cpb.ListContainerRequest_Filter, error) {
	mapping := make([]*cpb.ListContainerRequest_Filter, 0, len(filters))
	for _, f := range filters {
		parts := strings.Split(f, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid filter: %s", f)
		}
		values := strings.Split(parts[1], ",")
		mapping = append(mapping, &cpb.ListContainerRequest_Filter{
			Key:   parts[0],
			Value: values,
		})
	}
	return mapping, nil
}
