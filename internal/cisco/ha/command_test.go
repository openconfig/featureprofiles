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

var (
	simpleRun = *&config{
		ExecMode: ONCE,
	}
	simpleRunWithTimeout = *&config{
		ExecMode: ONCE,
		Timeout:  100 * time.Millisecond,
	}
	simpleRunWithDelay = *&config{
		ExecMode: ONCE,
		FirstRun: 1 * time.Second,
	}
	periodicRun = *&config{
		ExecMode: PERIODIC,
		Period:   1 * time.Second,
	}
	periodicRunWithTimeOut = *&config{
		ExecMode: PERIODIC,
		Period:   10 * time.Second,
		Timeout:  5 * time.Second,
	}
	chaoticRun = *&config{
		ExecMode: CHAOUS,
	}
)

func TestRunLoopSimpleRun(t *testing.T) {
	configs := []config {
		simpleRun, 
		simpleRunWithTimeout,
		simpleRunWithDelay, 
		//periodicRun, 
		//periodicRunWithTimeOut, 
		//chaoticRun,
	}

	for _,config := range configs {
		fakeCMD := &fakeCommand{Config: &config}
		fakeCMDWithErr := &fakeCommandWithError{Config: &config}
		triggers := map[string]trigger{
			"Normal trigger": fakeCMD,
			"Errorness trigger":fakeCMDWithErr,
		}
		for test,trigger := range triggers {
			t.Run(test, func(t *testing.T) {
				ch := make(chan CommandStatus)
				go runLoop(trigger, ch)
				for {
					status := <-ch
					if status == Done {
						break
					}
				}
				fmt.Println("Done ...")
			} )
		}

	}

}
