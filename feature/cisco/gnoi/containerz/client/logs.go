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
)

// Logs retrieves the logs for a given container. It can optionally follow the logs as they
// are being produced.
func (c *Client) Logs(ctx context.Context, instance string, follow bool) (<-chan *LogMessage, error) {
	lcli, err := c.cli.Log(ctx, &cpb.LogRequest{
		InstanceName: instance,
		Follow:       follow,
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan *LogMessage, 100)
	go func() {
		defer lcli.CloseSend()
		defer close(ch)

		for {
			msg, err := lcli.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}

				nonBlockingChannelSend(ctx, ch, &LogMessage{
					Error: err,
				})
				return
			}

			if nonBlockingChannelSend(ctx, ch, &LogMessage{
				Msg: msg.GetMsg(),
			}) {
				return
			}
		}
	}()

	return ch, nil
}
