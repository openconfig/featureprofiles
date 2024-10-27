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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"github.com/openconfig/containerz/chunker"
	cpb "github.com/openconfig/gnoi/containerz"
)

// State represents the state of the state machine
type State int

const (
	// initialise is the initial state. In this state, the in ImageTransfer message is sent to the server.
	initialise State = iota

	// ready waits for a TransferReady message from the server.
	ready

	// content sends the TransferContent message, containing the content of the image.
	content

	// progress waits for a progress message from the server indicating how many bytes have been
	// received.
	progress

	// finished sends the TransferEnd message to the server indicating that we are done sending
	// the image.
	finished

	// success waits for a TransferSuccess message which informs the client that the import operation
	// was successful.
	success
)

// PushImage implements the client logic to push an image to the target containerz server.
func (c *Client) PushImage(ctx context.Context, image string, tag string, file string) (<-chan *Progress, error) {
	dcli, err := c.cli.Deploy(ctx)
	if err != nil {
		return nil, err
	}

	reader, err := chunker.NewReader(file)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Progress, 100)
	go func() {
		defer close(ch)
		defer reader.Close()

		var chunkSize int32
		state := initialise
		for {
			switch state {
			case initialise:
				if err := dcli.Send(&cpb.DeployRequest{
					Request: &cpb.DeployRequest_ImageTransfer{
						ImageTransfer: &cpb.ImageTransfer{
							Name:      image,
							Tag:       tag,
							ImageSize: reader.Size(),
						},
					},
				}); err != nil {
					nonBlockingChannelSend(ctx, ch, &Progress{
						Error: err,
					})
					return
				}
				state = ready
			case ready:
				msg := recvMsgOrSendError[*cpb.DeployResponse_ImageTransferReady](ctx, ch, dcli)
				if msg == nil {
					return
				}

				chunkSize = msg.ImageTransferReady.GetChunkSize()
				state = content
			case content:
				buf, err := reader.Read(chunkSize)
				if err != nil {
					if err == io.EOF {
						state = finished
						continue
					}
					nonBlockingChannelSend(ctx, ch, &Progress{
						Error: err,
					})
					return
				}
				if err := dcli.Send(&cpb.DeployRequest{
					Request: &cpb.DeployRequest_Content{
						Content: buf,
					},
				}); err != nil {
					nonBlockingChannelSend(ctx, ch, &Progress{
						Error: err,
					})
					return
				}
				state = progress
			case progress:
				msg := recvMsgOrSendError[*cpb.DeployResponse_ImageTransferProgress](ctx, ch, dcli)
				if msg == nil {
					return
				}

				if nonBlockingChannelSend(ctx, ch, &Progress{
					BytesReceived: msg.ImageTransferProgress.GetBytesReceived(),
				}) {
					klog.Warningf("operation cancelled by client; returning")
					return
				}
				state = content
			case finished:
				if err := dcli.Send(&cpb.DeployRequest{
					Request: &cpb.DeployRequest_ImageTransferEnd{
						ImageTransferEnd: &cpb.ImageTransferEnd{},
					},
				}); err != nil {
					nonBlockingChannelSend(ctx, ch, &Progress{
						Error: err,
					})
					return
				}
				state = success
			case success:
				msg := recvMsgOrSendError[*cpb.DeployResponse_ImageTransferSuccess](ctx, ch, dcli)
				if msg == nil {
					return
				}
				if nonBlockingChannelSend(ctx, ch, &Progress{
					Finished: true,
					Image:    msg.ImageTransferSuccess.GetName(),
					Tag:      msg.ImageTransferSuccess.GetTag(),
				}) {
					klog.Warningf("operation cancelled by client; returning")
				}
				return
			}
		}
	}()

	return ch, nil
}

type transferTypes interface {
	*cpb.DeployResponse_ImageTransferReady |
		*cpb.DeployResponse_ImageTransferProgress |
		*cpb.DeployResponse_ImageTransferSuccess
}

func recvMsgOrSendError[T transferTypes](ctx context.Context, ch chan *Progress, dCli cpb.Containerz_DeployClient) T {
	msg, err := dCli.Recv()
	if err != nil {
		nonBlockingChannelSend(ctx, ch, &Progress{
			Error: err,
		})
		return nil
	}

	resp, ok := msg.GetResponse().(T)
	if !ok {
		nonBlockingChannelSend(ctx, ch, &Progress{
			Error: status.Errorf(codes.InvalidArgument, "received unexpected message type: %T", msg.GetResponse()),
		})
		return nil
	}

	return resp
}
