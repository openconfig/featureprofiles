package ha

import (
	"context"
	"testing"
	//"time"
	//"github.com/openconfig/ondatra"
	//"github.com/scrapli/scrapligo/platform"
)

type restart struct {
	Config *config
}

func (cmd *restart) Register(testing *testing.TB, channel ChannelType) {
}

func (cmd *restart) Cancel(testing *testing.TB)  {
}

func (cmd *restart) Verify(testing *testing.TB)  {
}

func (cmd *restart) run(ctx context.Context, channel ChannelType) error {
	return nil
}

func (cmd *restart) config(testing *testing.TB) *config{
	return cmd.Config
}
