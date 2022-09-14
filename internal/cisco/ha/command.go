package ha

import (
	//"context"
	"context"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
)

type ChannelType int

const (
	GNMI    ChannelType = 0
	GNOI                = 1
	SSH                 = 2
	CONSOLE             = 3
)

type ExecMode int

const (
	ONCE     ExecMode = 0
	PERIODIC          = 1
	CHAOUS            = 2
)


type ControlCommand int

const (
	Status     ControlCommand = 0
	Cancel          = 1
	Restart            = 2
)


type CommandStatus int

const (
	Running     CommandStatus = 0
	Failed          = 1
	Unknown         = 2
	Waiting         = 3  
	Done 	        = 4
)



type Service int

const (
	EMSD Service = 0
	FIB          = 1
	BGP          = 2
	// ...
)

type trigger interface {
	Register(testing *testing.TB, channel ChannelType) 
	Cancel(testing *testing.TB) 
	Verify(testing *testing.TB)
	run(ctx context.Context, channel ChannelType) error
	config() *config
}

type config struct {
	DUT      *ondatra.DUTDevice
	Timeout  time.Duration
	FirstRun time.Duration
	ExecMode ExecMode
	Period   time.Duration
	Services []Service
}

func runLoop(cmd trigger, status  chan CommandStatus)  {
	//ctx := context.Background()
	status <- Unknown
	switch(cmd.config().ExecMode) {
	case ONCE:
		if cmd.config().FirstRun != 0 {
			status <- Waiting
			time.Sleep(cmd.config().FirstRun)
		}
		status <- Running
		ctx := context.Background()
		var 
		if cmd.config().Timeout!=0 {
			ctx, cancel := context.WithTimeout(ctx, cmd.config().Timeout)
			defer cancel()
		}
		cmd.run(context.Background(),GNMI)
		status <- Done
	}
}