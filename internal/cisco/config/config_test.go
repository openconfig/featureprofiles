package config

import (
	"context"
	"time"
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/featureprofiles/internal/fptest"

)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBackgroundCLI(t *testing.T){
	dut:= ondatra.DUT(t,"dut")
	// This command restart the emsd process only once after one second
	BackgroundCLI(context.Background(),t,dut,"process restart emsd",[]string{"#"},[]string{".*Incomplete.*",".*Unable.*"},time.NewTimer(1*time.Second), 30*time.Second)
	// This command restart the emsd process every 5 second, note that the numbber of concurrent session for ssh is 5 by default. 
	ticker:= time.NewTicker(10*time.Second)
	BackgroundCLI(context.Background(),t,dut,"process restart emsd",[]string{"#"},[]string{".*Incomplete.*",".*Unable.*"},ticker, 30*time.Second)
	/// write the test code here
	time.Sleep(35*time.Second)
	ticker.Stop()
	time.Sleep(1*time.Second)


}