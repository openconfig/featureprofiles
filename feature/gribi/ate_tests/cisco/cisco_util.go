package cisco_gribi

import (
	"context"
	"net"
	"testing"
	"time"

	spb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra/telemetry"
)

func getIPPrefix(IPAddr string, i int, prefixLen string) string {
	ip := net.ParseIP(IPAddr)
	ip = ip.To4()
	ip[3] = ip[3] + byte(i%256)
	ip[2] = ip[2] + byte(i/256)
	ip[1] = ip[1] + byte(i/(256*256))
	return ip.String() + "/" + prefixLen
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

func GetNextElectionIdviaStub(stub spb.GRIBIClient, t testing.TB) uint64 {
	c := fluent.NewClient()
	c.Connection().WithStub(stub)
	c.Connection().WithInitialElectionID(1, 0).WithRedundancyMode(fluent.ElectedPrimaryClient).WithPersistence()
	c.Start(context.Background(), t)
	defer c.Stop(t)
	c.StartSending(context.Background(), t)
	if err := awaitTimeout(context.Background(), c, t, time.Minute); err != nil {
		t.Fatalf("got unexpected error on client, %v", err)
	}
	for _, re := range c.Results(t) {
		if re.CurrentServerElectionID != nil {
			return re.CurrentServerElectionID.Low + 1
		}
	}
	return uint64(1)
}

func CheckTrafficPassViaPortPktCounter(pktCounters []*telemetry.Interface_Counters, threshold ...float64) bool {
	thresholdValue := float64(0.99)
	if len(threshold) > 0 {
		thresholdValue = threshold[0]
	}
	totalIn := uint64(0)
	totalOut := uint64(0)

	for _, s := range pktCounters {
		totalIn = s.GetInPkts() + totalIn
		totalOut = s.GetOutPkts() + totalOut
	}
	return float64(totalIn)/float64(totalOut) >= thresholdValue
}
