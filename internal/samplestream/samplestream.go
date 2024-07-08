// Package samplestream provides utilities for creating gNMI Subscriptions in SAMPLE mode.
package samplestream

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygnmi/ygnmi"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

const (
	intervalTolerance = time.Second
)

// SampleStream represents a gNMI Subscription with SAMPLE mode.
type SampleStream[T any] struct {
	dataMu   sync.Mutex         // Lock that protects the received data and the next channel.
	lastVal  *ygnmi.Value[T]    // Holds the last received sample.
	data     []*ygnmi.Value[T]  // Data received from gNMI call.
	cancel   context.CancelFunc // Cancels the subscription.
	interval time.Duration      // Configured interval for the SAMPLE mode stream.
}

// New creates a new SampleStream.
func New[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T], interval time.Duration) *SampleStream[T] {
	ctx, cancel := context.WithCancel(context.Background())
	s := &SampleStream[T]{
		dataMu:   sync.Mutex{},
		cancel:   cancel,
		interval: interval,
	}

	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		t.Fatalf("unable to connect to gNMI on %s: %v", dut.ID(), err)
	}
	ygnmi.Watch(ctx, c, q, func(v *ygnmi.Value[T]) error {
		s.dataMu.Lock()
		defer s.dataMu.Unlock()
		if !v.IsPresent() {
			return ygnmi.Continue
		}
		s.data = append(s.data, v)
		s.lastVal = v
		return ygnmi.Continue
	}, ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(interval))
	return s
}

// Next returns the next sample received within the sample interval.
// If no sample is received within the interval, nil is returned.
func (s *SampleStream[T]) Next() *ygnmi.Value[T] {
	time.Sleep(s.interval + intervalTolerance)
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return s.lastVal
}

// Nexts calls Next() count times and returns the slice of returned samples.
func (s *SampleStream[T]) Nexts(count int) []*ygnmi.Value[T] {
	var nexts []*ygnmi.Value[T]
	for i := 0; i < count; i++ {
		nexts = append(nexts, s.Next())
	}
	return nexts
}

// All returns the list of values that has been received thus far.
func (s *SampleStream[T]) All() []*ygnmi.Value[T] {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return s.data
}

// Close closes the gnmi subscription.
func (s *SampleStream[T]) Close() {
	s.cancel()
}
