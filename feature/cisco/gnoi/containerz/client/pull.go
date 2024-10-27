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

	"k8s.io/klog/v2"
	commonpb "github.com/openconfig/gnoi/common"
	cpb "github.com/openconfig/gnoi/containerz"
	tpb "github.com/openconfig/gnoi/types"
)

// PullImage implements the client logic for the target to pull an image from a remote location.
func (c *Client) PullImage(ctx context.Context, image string, tag string, creds *tpb.Credentials) (<-chan *Progress, error) {
	dcli, err := c.cli.Deploy(ctx)
	if err != nil {
		return nil, err
	}

	if err := dcli.Send(&cpb.DeployRequest{
		Request: &cpb.DeployRequest_ImageTransfer{
			ImageTransfer: &cpb.ImageTransfer{
				Name:           image,
				Tag:            tag,
				RemoteDownload: &commonpb.RemoteDownload{},
			},
		},
	}); err != nil {
		return nil, err
	}

	ch := make(chan *Progress, 100)
	go func() {
		defer dcli.CloseSend()
		defer close(ch)
		for {
			msg, err := dcli.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}
				klog.Warningf("server unexpectedly disconnected: %v", err)
				return
			}

			switch resp := msg.GetResponse().(type) {
			case *cpb.DeployResponse_ImageTransferProgress:
				select {
				case <-ctx.Done():
					klog.Warningf("operation has been cancelled by client.")
					return
				case ch <- &Progress{
					BytesReceived: resp.ImageTransferProgress.GetBytesReceived(),
				}:
				default:
					klog.Warningf("unable to send progress message; dropping")
				}
			}
		}
	}()

	return ch, nil
}
