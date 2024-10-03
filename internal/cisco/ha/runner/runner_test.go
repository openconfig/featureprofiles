package runner

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/ha/monitor"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/netutil"

	"github.com/openconfig/ondatra"
)

const (
	pingThreadScale          = 5    // # of parallel ping request to send
	pingScale                = 2000 // # of ping message to send
	AFTTelemtryUpdateTimeout = 120  // second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBackgroundCLI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// This command restart the emsd process only once after one second
	RunCLIInBackground(context.Background(), t, dut, "process restart emsd", []string{"#"}, []string{".*Incomplete.*", ".*Unable.*"}, time.NewTimer(1*time.Second), 30*time.Second)
	// This command restart the emsd process every 5 second, note that the numbber of concurrent session for ssh is 5 by default.
	ticker := time.NewTicker(10 * time.Second)
	RunCLIInBackground(context.Background(), t, dut, "process restart emsd", []string{"#"}, []string{".*Incomplete.*", ".*Unable.*"}, ticker, 30*time.Second)
	/// write the test code here
	time.Sleep(35 * time.Second)
	ticker.Stop()
	time.Sleep(1 * time.Second)

}

func TestLoad(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// start tests
	testGroup := &sync.WaitGroup{}

	// start gnoi  ping
	RunTestInBackground(context.Background(), t, time.NewTimer(10*time.Second), testGroup, nil, testPing, dut)

	time.Sleep(11 * time.Second) // wait until all the tests start (the timer period + 1)
	testGroup.Wait()
	time.Sleep(60 * time.Second)

}

func testPing(t *testing.T, event *monitor.CachedConsumer, args ...interface{}) {
	t.Helper()

	dut := args[0].(*ondatra.DUTDevice)

	startTime := time.Now()
	t.Logf("Ping test is started at: %v", startTime)
	fetchResponses := func(c spb.System_PingClient) ([]*spb.PingResponse, error) {
		pingResp := []*spb.PingResponse{}
		for {
			resp, err := c.Recv()
			switch {
			case err == io.EOF:
				return pingResp, nil
			case err != nil:
				return nil, err
			default:
				pingResp = append(pingResp, resp)
			}
		}
	}
	lbIntf := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(lbIntf).Subinterface(0)
	ipv4Addrs := gnmi.GetAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.GetAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv4 loopback address: %+v", ipv4Addrs)
	}
	if len(ipv6Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv6 loopback address: %+v", ipv6Addrs)
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	pingRequest := &spb.PingRequest{
		Destination: ipv4Addrs[0].GetIp(),
		Source:      ipv4Addrs[0].GetIp(),
		L3Protocol:  tpb.L3Protocol_IPV4,
	}
	for i := 0; i <= pingThreadScale; i++ {
		go func(t *testing.T) {
			for i := 0; i < pingScale; i++ {
				pingClient, err := gnoiClient.System().Ping(context.Background(), pingRequest)
				if err != nil {
					t.Errorf("Failed to query gnoi endpoint: %v", err)
				}
				responses, err := fetchResponses(pingClient)
				if err != nil {
					t.Errorf("Failed to handle gnoi ping client stream: %v", err)
				}
				if len(responses) == 0 {
					t.Errorf("Number of responses to %v: got 0, want > 0", pingRequest.Destination)
				}
			}
		}(t)
	}

	endTime := time.Now()
	t.Logf("Ping test is completed by doing ping %d times  at %v, (The completion time is %v)", pingScale*pingThreadScale, endTime, time.Since(startTime))
}
