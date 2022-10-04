//  Package runner contains utolity api for monitoring telemetry paths in background while running tests
//  A monitor pushes all event to the an event consumer that should provide process method.
//  A monitor can monitor multipe paths, however provided paths should be disjoint. 

// Package runner provides api to run tests and cli in background.

package runner

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/monitor"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	

)

// TestArgs defines the arguments that test in background will receveis. 
// For simplicity we assume only one ATE is available and the test should use ATELock before using the ATE
type TestArgs struct {
	DUT []*ondatra.DUTDevice
	// ATE lock should be aquired before using ATE. Only one test can use the ATE at a time.
	ATELock sync.Mutex
	ATE     *ondatra.ATEDevice
}


// BackgroundTest is the signature of a test function that can be run in background
type BackgroundTest func(t *testing.T, args *TestArgs, events *monitor.CachedConsumer)

// RunTestInBackground runs a testing function in the background. The period can be ticker or simple timer. With simple timer the function only run once
// Eeven refers to gnmi evelenet collected with streaming telemtry.
func RunTestInBackground(ctx context.Context, t *testing.T, period interface{}, args *TestArgs, events *monitor.CachedConsumer, function BackgroundTest, workGroup *sync.WaitGroup) {
	t.Helper()
	timer, ok := period.(*time.Timer)
	if ok {
		go func() {
			<-timer.C
			workGroup.Add(1)
			defer workGroup.Done()
			function(t, args, events)
		}()
	}
    // TODO: Not tested
	ticker, ok := period.(*time.Ticker)
	if ok {
		go func() {
			for {
				for _ = range ticker.C {
					localWG := sync.WaitGroup{}
					localWG.Add(1) // make sure only one instance of the same test can be excuted
					workGroup.Add(1) 
					go func() { // using goroutine to make sure the waitgroup can be done to prevent the whole test to be hanged when a test calls  Fatal
						defer workGroup.Done()
						defer localWG.Done()
						function(t, args, events)
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
