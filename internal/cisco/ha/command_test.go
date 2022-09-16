package ha

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type test struct {
	name          string
	cmd           trigger
	expectedError error
}

type fakeCommandWithError struct {
	Config *config
}

type fakeCommand struct {
	Config *config
}

type fakeCommandWithDelay struct {
	Config *config
}

func (cmd *fakeCommand) Register(testing *testing.TB, channel ChannelType) {
}

func (cmd *fakeCommand) Cancel(testing *testing.TB) {
}

func (cmd *fakeCommand) Verify(testing *testing.TB) {
}

func (cmd *fakeCommand) run(ctx context.Context, channel ChannelType) error {
	time.Sleep(1 * time.Second)
	return nil
}

func (cmd *fakeCommand) config() *config {
	return cmd.Config
}

func (cmd *fakeCommandWithError) Register(testing *testing.TB, channel ChannelType) {
}

func (cmd *fakeCommandWithError) Cancel(testing *testing.TB) {
}

func (cmd *fakeCommandWithError) Verify(testing *testing.TB) {
}

func (cmd *fakeCommandWithError) run(ctx context.Context, channel ChannelType) error {
	return fmt.Errorf("error in running command")
}

func (cmd *fakeCommandWithError) config() *config {
	return cmd.Config
}

func (cmd *fakeCommandWithDelay) Register(testing *testing.TB, channel ChannelType) {
}

func (cmd *fakeCommandWithDelay) Cancel(testing *testing.TB) {
}

func (cmd *fakeCommandWithDelay) Verify(testing *testing.TB) {
}

func (cmd *fakeCommandWithDelay) run(ctx context.Context, channel ChannelType) error {
	time.Sleep(3*time.Second)
	return nil
}

func (cmd *fakeCommandWithDelay) config() *config {
	return cmd.Config
}



var (
	simpleRun = *&config{
		ExecMode: ONCE,
	}
	simpleRunWithDelay = *&config{
		ExecMode: ONCE,
		FirstRun: 1 * time.Second,
	}
	simpleRunWithTimeout = *&config{
		ExecMode: ONCE,
		Timeout: 1*time.Second,
	}
)

func TestRunLoopSimpleRun(t *testing.T) {
	configs := []config{
		simpleRun,
		simpleRunWithDelay,
		simpleRunWithTimeout,
	}

	for _, config := range configs {
		fakeCMD := &fakeCommand{Config: &config}
		fakeCMDWithErr := &fakeCommandWithError{Config: &config}
		triggers := map[string]trigger{
			"Normal trigger":    fakeCMD,
			"Errorness trigger": fakeCMDWithErr,
		}
		for test, trig := range triggers {
			t.Run(test, func(t *testing.T) {
				ch := make(chan CommandStatus)
				go runLoop(trig, ch)
				startTime := time.Now()
				for {
					status := <-ch
					if status == Unknown{
						continue
					}
					// checking for waiting time before running
					if status == Waiting {
						if time.Since(startTime) > config.FirstRun {
							t.Fatal("Waiting time is more than expected one")
						}
						continue
					} else if time.Since(startTime) < config.FirstRun {
						t.Fatalf("The command did not wait expected time")
					}
					// check for done status depnedeing on the trigger results
					if _, ok := trig.(*fakeCommand); ok {
						if status == Done {
							break
						} else if status == Failed {
							t.Fatal("Failure of the trigger is not expected")
						}
					} else {
						if status == Failed {
							break
						} else if status == Done {
							t.Fatal("Failure of the trigger is expected")
						}
					}
				}
			})
		}

	}

}
