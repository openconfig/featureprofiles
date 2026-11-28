// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package afts_prefix_filtering_resilience_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// awaitBootTime uses one overall recovery deadline and a shorter deadline for
// each probe. The probe must honor its context, including while dialing.
func awaitBootTime(ctx context.Context, previous uint64, interval, probeTimeout time.Duration, probe func(context.Context) (uint64, error)) (uint64, error) {
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return 0, fmt.Errorf("reboot readiness stopped: %w; last probe: %v", err, lastErr)
		}
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		observed, err := probe(probeCtx)
		cancel()
		if ctx.Err() != nil {
			return 0, fmt.Errorf("reboot readiness stopped: %w; last probe: %v", ctx.Err(), err)
		}
		if err == nil && observed > previous {
			return observed, nil
		}
		lastErr = err
		if lastErr == nil {
			lastErr = fmt.Errorf("boot time has not increased: got %d, previous %d", observed, previous)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, fmt.Errorf("reboot readiness stopped: %w; last probe: %v", ctx.Err(), lastErr)
		case <-timer.C:
		}
	}
}

func TestAwaitBootTime(t *testing.T) {
	t.Run("recovered before first poll", func(t *testing.T) {
		got, err := awaitBootTime(t.Context(), 100, time.Millisecond, time.Second, func(context.Context) (uint64, error) {
			return 101, nil
		})
		if err != nil || got != 101 {
			t.Fatalf("awaitBootTime() = %d, %v; want 101, nil", got, err)
		}
	})
	t.Run("wait for changed boot time through RPC failure", func(t *testing.T) {
		calls := 0
		got, err := awaitBootTime(t.Context(), 100, time.Millisecond, time.Second, func(context.Context) (uint64, error) {
			calls++
			switch calls {
			case 1:
				return 100, nil
			case 2:
				return 0, errors.New("gNMI unavailable")
			default:
				return 101, nil
			}
		})
		if err != nil || got != 101 || calls != 3 {
			t.Fatalf("awaitBootTime() = %d, %v after %d probes; want 101, nil after 3", got, err, calls)
		}
	})
	t.Run("bounded probe retries", func(t *testing.T) {
		calls := 0
		got, err := awaitBootTime(t.Context(), 100, time.Millisecond, 10*time.Millisecond, func(ctx context.Context) (uint64, error) {
			calls++
			if calls == 1 {
				<-ctx.Done()
				return 0, ctx.Err()
			}
			return 101, nil
		})
		if err != nil || got != 101 || calls != 2 {
			t.Fatalf("awaitBootTime() = %d, %v after %d probes; want 101, nil after 2", got, err, calls)
		}
	})
	t.Run("overall deadline bounds active probe", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		_, err := awaitBootTime(ctx, 100, time.Millisecond, time.Minute, func(probeCtx context.Context) (uint64, error) {
			<-probeCtx.Done()
			return 0, probeCtx.Err()
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("awaitBootTime() error = %v; want overall deadline exceeded", err)
		}
	})
	t.Run("canceled context does not probe", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := awaitBootTime(ctx, 100, time.Millisecond, time.Second, func(context.Context) (uint64, error) {
			t.Fatal("Probe called after cancellation")
			return 0, nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("awaitBootTime() error = %v; want context canceled", err)
		}
	})
}
