// Package latency provides gRPC interceptors for injecting latency.
package latency

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// MetadataKey is the key in the metadata that allows users to specify latency injection.
	MetadataKey = "latency"
)

// Delay reads latency from context metadata. If multiple values are present
// for MetadataKey, it uses the first value and logs a warning.
func Delay(ctx context.Context) (time.Duration, bool) {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md, ok = metadata.FromIncomingContext(ctx)
	}

	if !ok {
		return 0, false
	}
	vals := md.Get(MetadataKey)
	if len(vals) == 0 {
		return 0, false
	}
	if len(vals) > 1 {
		log.Printf("WARNING: Multiple values for %q in metadata: %v, using %q", MetadataKey, vals, vals[0])
	}
	d, err := time.ParseDuration(vals[0])
	if err != nil {
		log.Printf("WARNING: Invalid latency format in metadata: %s. Error: %v", vals[0], err)
		return 0, false
	}
	log.Printf("INFO: Found latency metadata: %s. Delay: %v", vals[0], d)
	return d, true
}

// UnaryClientInterceptor returns a UnaryClientInterceptor that injects latency.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if delay, ok := Delay(ctx); ok {
			log.Printf("INFO: Injecting latency %v for method %s", delay, method)
			time.Sleep(delay)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor returns a StreamClientInterceptor that injects latency.
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		delay, ok := Delay(ctx)
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return nil, err
		}

		if ok {
			log.Printf("INFO: Injecting latency %v for method %s", delay, method)
			return &latencyClientStream{ClientStream: clientStream, delay: delay}, nil
		}
		return clientStream, nil
	}
}

type latencyClientStream struct {
	grpc.ClientStream
	delay time.Duration
}

func (l *latencyClientStream) RecvMsg(m any) error {
	if l.delay > 0 {
		ctx := l.ClientStream.Context()
		select {
		case <-time.After(l.delay):
			// Delay finished
		case <-ctx.Done():
			// Original context cancelled during delay
			return ctx.Err()
		}
	}
	return l.ClientStream.RecvMsg(m)
}
