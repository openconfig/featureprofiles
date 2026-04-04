package latency

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestDelay(t *testing.T) {
	tests := []struct {
		name   string
		md     metadata.MD
		want   time.Duration
		wantOK bool
	}{
		{
			name:   "no metadata",
			md:     nil,
			want:   0,
			wantOK: false,
		},
		{
			name:   "no latency key",
			md:     metadata.Pairs("other", "value"),
			want:   0,
			wantOK: false,
		},
		{
			name:   "invalid latency value",
			md:     metadata.Pairs("latency", "invalid"),
			want:   0,
			wantOK: false,
		},
		{
			name:   "valid latency value",
			md:     metadata.Pairs("latency", "10ms"),
			want:   10 * time.Millisecond,
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			if tc.md != nil {
				ctx = metadata.NewOutgoingContext(ctx, tc.md)
			}
			got, ok := Delay(ctx)
			if ok != tc.wantOK {
				t.Errorf("Delay() ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("Delay() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUnaryClientInterceptor(t *testing.T) {
	invoked := false
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		return nil
	}

	unaryInt := UnaryClientInterceptor()
	delay := 100 * time.Millisecond
	ctx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("latency", delay.String()))

	start := time.Now()
	err := unaryInt(ctx, "method", nil, nil, nil, invoker)
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("interceptor failed: %v", err)
	}
	if !invoked {
		t.Errorf("invoker not called")
	}
	if duration < delay {
		t.Errorf("Unary call duration = %v, want at least %v", duration, delay)
	}
}

func TestUnaryClientInterceptorNoLatency(t *testing.T) {
	invoked := false
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		return nil
	}

	unaryInt := UnaryClientInterceptor()
	ctx := t.Context()
	noDelayThreshold := 50 * time.Millisecond

	start := time.Now()
	err := unaryInt(ctx, "method", nil, nil, nil, invoker)
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("interceptor failed: %v", err)
	}
	if !invoked {
		t.Errorf("invoker not called")
	}
	if duration >= noDelayThreshold {
		t.Errorf("Unary call duration = %v, should be less than %v without latency", duration, noDelayThreshold)
	}
}

type fakeClientStream struct {
	grpc.ClientStream
	ctx context.Context
}

func (f *fakeClientStream) RecvMsg(m any) error {
	return nil
}
func (f *fakeClientStream) Context() context.Context {
	return f.ctx
}

func TestStreamClientInterceptor(t *testing.T) {
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return &fakeClientStream{ctx: ctx}, nil
	}

	streamInt := StreamClientInterceptor()
	delay := 100 * time.Millisecond
	ctx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("latency", delay.String()))

	cs, err := streamInt(ctx, nil, nil, "method", streamer)
	if err != nil {
		t.Fatalf("interceptor failed: %v", err)
	}

	start := time.Now()
	cs.RecvMsg(nil)
	duration := time.Since(start)

	if duration < delay {
		t.Errorf("Stream RecvMsg duration = %v, want at least %v", duration, delay)
	}
}

func TestStreamClientInterceptorNoLatency(t *testing.T) {
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return &fakeClientStream{ctx: ctx}, nil
	}

	streamInt := StreamClientInterceptor()
	ctx := t.Context()
	noDelayThreshold := 50 * time.Millisecond

	cs, err := streamInt(ctx, nil, nil, "method", streamer)
	if err != nil {
		t.Fatalf("interceptor failed: %v", err)
	}

	start := time.Now()
	cs.RecvMsg(nil)
	duration := time.Since(start)

	if duration >= noDelayThreshold {
		t.Errorf("Stream RecvMsg duration = %v, should be less than %v without latency", duration, noDelayThreshold)
	}
}

func TestStreamClientInterceptorContextCancelled(t *testing.T) {
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return &fakeClientStream{ctx: ctx}, nil
	}

	streamInt := StreamClientInterceptor()
	delay := 100 * time.Millisecond
	ctx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("latency", delay.String()))
	ctx, cancel := context.WithCancel(ctx)

	cs, err := streamInt(ctx, nil, nil, "method", streamer)
	if err != nil {
		t.Fatalf("interceptor failed: %v", err)
	}

	cancel()
	// Allow some time for cancellation to propagate in time.After()
	time.Sleep(10 * time.Millisecond)
	err = cs.RecvMsg(nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("RecvMsg() got err %v, want %v", err, context.Canceled)
	}
}
