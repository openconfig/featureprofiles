// Package runner provides api to run tests and cli in background.
package runner

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/monitor"
	"github.com/openconfig/ondatra"
)

// BackgroundTest is the signature of a test function that can be run in background
type BackgroundTest func(t *testing.T, events *monitor.CachedConsumer, args ...interface{})

// RunTestInBackground runs a testing function in the background. The period can be ticker or simple timer. With simple timer the function only run once
// Eeven refers to gnmi evelenet collected with streaming telemtry.
func RunTestInBackground(ctx context.Context, t *testing.T, period interface{}, workGroup *sync.WaitGroup, events *monitor.CachedConsumer, function BackgroundTest, args ...interface{}) {
	t.Helper()
	timer, ok := period.(*time.Timer)
	if ok {
		workGroup.Add(1)
		go func() {
			defer workGroup.Done()
			<-timer.C
			function(t, events, args...)
		}()
	}
	// TODO: Not tested
	ticker, ok := period.(*time.Ticker)
	if ok {
		go func() {
			for {
				for range ticker.C {
					localWG := sync.WaitGroup{}
					localWG.Add(1) // make sure only one instance of the same test can be excuted
					workGroup.Add(1)
					go func() { // using goroutine to make sure the waitgroup can be done to prevent the whole test to be hanged when a test calls  Fatal
						defer workGroup.Done()
						defer localWG.Done()
						function(t, events, args...)
					}()
					localWG.Wait()
				}
			}
		}()
	}
}

// RunCLIInBackground runs an admin command on the backgroun and fails if the command is unsucessful or does not return earlier than timeout
// The command also fails if the response does not match the expeted reply pattern or matches the not-expected one
func RunCLIInBackground(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cmd string, expectedRep, notExpectedRep []string, period interface{}, timeOut time.Duration) {
	t.Helper()
	timer, ok := period.(*time.Timer)
	if ok {
		go func() {
			<-timer.C
			reply := config.CLIViaSSH(ctx, t, dut, cmd, timeOut)
			t.Logf("Reply for %s : %s", cmd, reply)
			verifyCLIOutput(t, reply, expectedRep, notExpectedRep)
		}()
	}

	ticker, ok := period.(*time.Ticker)
	if ok {
		go func() {
			for {
				<-ticker.C
				reply := config.CLIViaSSH(ctx, t, dut, cmd, timeOut)
				t.Logf("Reply for %s : %s", cmd, reply)
				verifyCLIOutput(t, reply, expectedRep, notExpectedRep)
			}
		}()
	}
}

func verifyCLIOutput(t *testing.T, output string, match, notMatch []string) {
	t.Helper()
	for _, pattern := range match {
		ok, err := regexp.MatchString(pattern, output)
		if err != nil || !ok {
			t.Fatalf("The command reply does not contain the expected pattern %s ", pattern)
		}
	}
	for _, pattern := range notMatch {
		ok, err := regexp.MatchString(pattern, output)
		if err == nil && ok {
			t.Fatalf("The command reply contains not expected pattern % s", pattern)
		}
	}
}
