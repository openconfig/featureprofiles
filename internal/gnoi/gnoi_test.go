// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gnoi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type fakeSystemClient struct {
	spb.SystemClient
	errors   []error
	requests []*spb.KillProcessRequest
}

func (c *fakeSystemClient) KillProcess(_ context.Context, request *spb.KillProcessRequest, _ ...grpc.CallOption) (*spb.KillProcessResponse, error) {
	c.requests = append(c.requests, proto.Clone(request).(*spb.KillProcessRequest))
	if len(c.requests) <= len(c.errors) {
		return nil, c.errors[len(c.requests)-1]
	}
	return &spb.KillProcessResponse{}, nil
}

type processSample struct {
	name string
	pid  uint64
}

type processReply struct {
	processes []processSample
	err       error
	block     bool
}

type fakeProcessClient struct {
	gpb.GNMIClient
	replies  []processReply
	requests []*gpb.SubscribeRequest
}

func (c *fakeProcessClient) Subscribe(ctx context.Context, _ ...grpc.CallOption) (gpb.GNMI_SubscribeClient, error) {
	reply := processReply{}
	if len(c.replies) > 0 {
		reply = c.replies[0]
		if len(c.replies) > 1 {
			c.replies = c.replies[1:]
		}
	}
	return &fakeProcessStream{ctx: ctx, client: c, reply: reply}, nil
}

type fakeProcessStream struct {
	gpb.GNMI_SubscribeClient
	ctx    context.Context
	client *fakeProcessClient
	reply  processReply
	synced bool
}

func (s *fakeProcessStream) Send(request *gpb.SubscribeRequest) error {
	s.client.requests = append(s.client.requests, proto.Clone(request).(*gpb.SubscribeRequest))
	return nil
}

func (*fakeProcessStream) CloseSend() error { return nil }

func (s *fakeProcessStream) Recv() (*gpb.SubscribeResponse, error) {
	if s.reply.block {
		<-s.ctx.Done()
		return nil, s.ctx.Err()
	}
	if s.reply.err != nil {
		return nil, s.reply.err
	}
	if s.synced {
		return &gpb.SubscribeResponse{Response: &gpb.SubscribeResponse_SyncResponse{SyncResponse: true}}, nil
	}
	s.synced = true
	var updates []*gpb.Update
	for _, p := range s.reply.processes {
		path := []*gpb.PathElem{
			{Name: "system"}, {Name: "processes"},
			{Name: "process", Key: map[string]string{"pid": fmt.Sprint(p.pid)}}, {Name: "state"},
		}
		updates = append(updates,
			&gpb.Update{
				Path: &gpb.Path{Elem: append(path, &gpb.PathElem{Name: "name"})},
				Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: p.name}},
			},
			&gpb.Update{
				Path: &gpb.Path{Elem: append(path, &gpb.PathElem{Name: "pid"})},
				Val:  &gpb.TypedValue{Value: &gpb.TypedValue_UintVal{UintVal: p.pid}},
			},
		)
	}
	return &gpb.SubscribeResponse{Response: &gpb.SubscribeResponse_Update{Update: &gpb.Notification{
		Prefix: &gpb.Path{Origin: "openconfig"}, Timestamp: time.Now().UnixNano(), Update: updates,
	}}}, nil
}

func TestKillProcess(t *testing.T) {
	unavailable := status.Error(codes.Unavailable, "connection reset by peer")
	denied := status.Error(codes.PermissionDenied, "not authorized")
	newProcess := processReply{processes: []processSample{{"emsd", 200}}}
	tests := []struct {
		name         string
		killErrors   []error
		replies      []processReply
		noWait       bool
		noRestart    bool
		wantSignals  []spb.KillProcessRequest_Signal
		wantPID      uint64
		wantErr      string
		wantDeadline bool
		wantQueries  int
	}{
		{
			name: "successful restart", replies: []processReply{newProcess},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantPID: 200, wantQueries: 1,
		},
		{
			name: "recover after reset empty telemetry and old PID",
			replies: []processReply{
				{err: unavailable}, {}, {processes: []processSample{{"emsd", 100}}},
				{processes: []processSample{{"emsd", 0}}}, {processes: []processSample{{"other", 300}}}, newProcess,
			},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantPID: 200, wantQueries: 6,
		},
		{
			name: "EOF from self termination is verified without another kill", killErrors: []error{io.EOF},
			replies:     []processReply{{err: unavailable}, newProcess},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantPID: 200, wantQueries: 2,
		},
		{
			name: "unavailable kill response is verified", killErrors: []error{unavailable}, replies: []processReply{newProcess},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantPID: 200, wantQueries: 1,
		},
		{
			name: "deadline exceeded kill response is verified", killErrors: []error{status.Error(codes.DeadlineExceeded, "response lost")},
			replies: []processReply{newProcess}, wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantPID: 200, wantQueries: 1,
		},
		{
			name: "rejected TERM falls back to KILL of original PID", killErrors: []error{status.Error(codes.InvalidArgument, "TERM unsupported")},
			replies: []processReply{newProcess}, wantSignals: []spb.KillProcessRequest_Signal{SigTerm, SigKill}, wantPID: 200, wantQueries: 1,
		},
		{
			name: "permission failure is not retried", killErrors: []error{denied},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantErr: "PermissionDenied",
		},
		{
			name: "telemetry permission failure is not retried", replies: []processReply{{err: denied}},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantErr: "PermissionDenied", wantQueries: 1,
		},
		{
			name: "no wait performs no telemetry query", noWait: true,
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm},
		},
		{
			name: "no wait cannot accept ambiguous kill failure", noWait: true, killErrors: []error{unavailable},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantErr: "Unavailable",
		},
		{
			name: "no restart cannot accept ambiguous kill failure", noRestart: true, killErrors: []error{io.EOF},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantErr: "EOF",
		},
		{
			name: "old PID does not count as recovery", replies: []processReply{{processes: []processSample{{"emsd", 100}}}},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantErr: "still has PID 100", wantDeadline: true, wantQueries: -1,
		},
		{
			name: "lost kill response without recovery fails", killErrors: []error{io.EOF}, replies: []processReply{{err: unavailable}},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantErr: "KillProcess returned EOF", wantDeadline: true, wantQueries: -1,
		},
		{
			name: "stalled telemetry request times out then retries", replies: []processReply{{block: true}, newProcess},
			wantSignals: []spb.KillProcessRequest_Signal{SigTerm}, wantPID: 200, wantQueries: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				systemClient := &fakeSystemClient{errors: test.killErrors}
				gnmiClient := &fakeProcessClient{replies: test.replies}
				processClient, err := ygnmi.NewClient(gnmiClient, ygnmi.WithTarget("dut"))
				if err != nil {
					t.Fatal(err)
				}
				request := &spb.KillProcessRequest{Name: "emsd", Pid: 100, Signal: SigTerm, Restart: !test.noRestart}
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				start := time.Now()
				pid, err := killProcess(ctx, systemClient, request, !test.noWait, processClient)
				if (err != nil) != (test.wantErr != "") || err != nil && !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("killProcess() error = %v, want %q", err, test.wantErr)
				}
				if test.wantDeadline && !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("killProcess() error = %v, want deadline exceeded", err)
				}
				if elapsed := time.Since(start); test.wantDeadline && elapsed != time.Minute {
					t.Errorf("recovery duration = %v, want one minute", elapsed)
				}
				if pid != test.wantPID {
					t.Errorf("killProcess() PID = %d, want %d", pid, test.wantPID)
				}
				if len(systemClient.requests) != len(test.wantSignals) {
					t.Fatalf("KillProcess calls = %d, want %d", len(systemClient.requests), len(test.wantSignals))
				}
				for i, got := range systemClient.requests {
					want := &spb.KillProcessRequest{Name: "emsd", Pid: 100, Signal: test.wantSignals[i], Restart: !test.noRestart}
					if !proto.Equal(got, want) {
						t.Errorf("KillProcess request %d = %v, want %v", i, got, want)
					}
				}
				if request.GetSignal() != SigTerm {
					t.Errorf("caller request mutated: %v", request)
				}
				if got := len(gnmiClient.requests); test.wantQueries >= 0 && got != test.wantQueries {
					t.Errorf("process queries = %d, want %d", got, test.wantQueries)
				}
				for _, request := range gnmiClient.requests {
					if sub := request.GetSubscribe(); sub.GetMode() != gpb.SubscriptionList_ONCE || sub.GetPrefix().GetTarget() != "dut" {
						t.Errorf("recovery query must use ONCE and preserve target, got %v", request)
					}
				}
			})
		})
	}
}
