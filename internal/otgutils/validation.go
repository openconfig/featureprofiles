package otgutils

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
)

// ExpectedBgpMetrics struct used for validating the fetched OTG BGP stats.
type ExpectedBgpMetrics struct {
	Advertised int32
	Received   int32
}

// ExpectedIsisMetrics struct used for validating the fetched OTG ISIS stats.
type ExpectedIsisMetrics struct {
	L1SessionsUp   int32
	L2SessionsUp   int32
	L1DatabaseSize int32
	L2DatabaseSize int32
}

// ExpectedPortMetrics struct used for validating the fetched OTG Port stats.
type ExpectedPortMetrics struct {
	FramesRx int32
}

// ExpectedFlowMetrics struct used for validating the fetched OTG Flow stats.
type ExpectedFlowMetrics struct {
	FramesRx     int64
	FramesRxRate float32
}

// ExpectedState is used for creating expected otg metrics.
type ExpectedState struct {
	Port map[string]ExpectedPortMetrics
	Flow map[string]ExpectedFlowMetrics
	BGP4 map[string]ExpectedBgpMetrics
	BGP6 map[string]ExpectedBgpMetrics
	ISIS map[string]ExpectedIsisMetrics
}

// WaitForOpts is used at tests level whenever WaitFor func is called\. There are 3 parameters which could be set.
type WaitForOpts struct {
	Condition string
	Interval  time.Duration
	Timeout   time.Duration
}

// timer prints time elapsed in ms since a given start time
func timer(t *testing.T, start time.Time, name string) {
	elapsed := time.Since(start)
	t.Log(name, "took", elapsed.Milliseconds(), "ms")
}

// WaitFor returns nil once the given function param returns true. It will wait and retry for the entire timeout duration.
func WaitFor(t *testing.T, fn func() bool, opts *WaitForOpts) error {
	if opts == nil {
		opts = &WaitForOpts{
			Condition: "condition to be true",
		}
	}
	defer timer(t, time.Now(), fmt.Sprintf("Waiting for %s", opts.Condition))

	if opts.Interval == 0 {
		opts.Interval = 500 * time.Millisecond
	}
	if opts.Timeout == 0 {
		opts.Timeout = 120 * time.Second
	}

	start := time.Now()
	t.Log("Waiting for", opts.Condition)

	for {
		done := fn()
		if done {
			t.Log("Done waiting for", opts.Condition)
			return nil
		}

		if time.Since(start) > opts.Timeout {
			return (fmt.Errorf("timeout occurred while waiting for %s", opts.Condition))
		}
		time.Sleep(opts.Interval)
	}
}

// AllBGP4Up returns true if all BGPv4 sessions are up and the advertised and received routes are matching the expected input.
func AllBGP4Up(t *testing.T, otg *ondatra.OTG, c gosnappi.Config, expectedState ExpectedState) bool {
	expected := true
	for _, d := range c.Devices().Items() {
		bgp := d.Bgp()
		for _, ip := range bgp.Ipv4Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				telePeer := otg.Telemetry().BgpPeer(configPeer.Name()).Get(t)
				expectedMetrics := expectedState.BGP4[configPeer.Name()]
				inRoutes := int32(telePeer.GetCounters().GetInRoutes())
				outRoutes := int32(telePeer.GetCounters().GetOutRoutes())
				if telePeer.GetSessionState() != otgtelemetry.BgpPeer_SessionState_ESTABLISHED || outRoutes != expectedMetrics.Advertised || inRoutes != expectedMetrics.Received {
					expected = false
				}
			}
		}
	}
	return expected
}

// AllBGP6Up returns true if all BGPv6 sessions are up and the advertised and received routes are matching the expected input.
func AllBGP6Up(t *testing.T, otg *ondatra.OTG, c gosnappi.Config, expectedState ExpectedState) bool {
	expected := true
	for _, d := range c.Devices().Items() {
		bgp := d.Bgp()
		for _, ip := range bgp.Ipv6Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				telePeer := otg.Telemetry().BgpPeer(configPeer.Name()).Get(t)
				expectedMetrics := expectedState.BGP6[configPeer.Name()]
				inRoutes := int32(telePeer.GetCounters().GetInRoutes())
				outRoutes := int32(telePeer.GetCounters().GetOutRoutes())
				if telePeer.GetSessionState() != otgtelemetry.BgpPeer_SessionState_ESTABLISHED || outRoutes != expectedMetrics.Advertised || inRoutes != expectedMetrics.Received {
					expected = false
				}
			}
		}
	}
	return expected
}

// AllBGP4Down returns true if all BGPv4 sessions are down.
func AllBGP4Down(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) bool {
	expected := true
	for _, d := range c.Devices().Items() {
		bgp := d.Bgp()
		for _, ip := range bgp.Ipv4Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				telePeer := otg.Telemetry().BgpPeer(configPeer.Name()).Get(t)
				if telePeer.GetSessionState() == otgtelemetry.BgpPeer_SessionState_ESTABLISHED {
					expected = false
				}
			}
		}
	}
	return expected
}

// AllBGP6Down returns true if all BGPv6 sessions are down.
func AllBGP6Down(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) bool {
	expected := true
	for _, d := range c.Devices().Items() {
		bgp := d.Bgp()
		for _, ip := range bgp.Ipv6Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				telePeer := otg.Telemetry().BgpPeer(configPeer.Name()).Get(t)
				if telePeer.GetSessionState() == otgtelemetry.BgpPeer_SessionState_ESTABLISHED {
					expected = false
				}
			}
		}
	}
	return expected
}

// FlowMetricsOk returns true if all the expected flow stats are verified.
func FlowMetricsOk(t *testing.T, otg *ondatra.OTG, c gosnappi.Config, expectedState ExpectedState) bool {
	expected := true
	for _, f := range c.Flows().Items() {
		flowMetrics := otg.Telemetry().Flow(f.Name()).Get(t)
		expectedMetrics := expectedState.Flow[f.Name()]
		if int64(*flowMetrics.Counters.InPkts) != expectedMetrics.FramesRx || ygot.BinaryToFloat32(flowMetrics.GetInFrameRate()) != expectedMetrics.FramesRxRate {
			expected = false

		}
	}
	return expected
}

// ArpEntriesOk returns true if all the expected mac entries are verified.
func ArpEntriesOk(t *testing.T, otg *ondatra.OTG, ipType string, expectedMacEntries []string) bool {
	expected := true
	actualMacEntries := []string{}
	switch ipType {
	case "IPv4":
		actualMacEntries = otg.Telemetry().InterfaceAny().Ipv4NeighborAny().LinkLayerAddress().Get(t)
	case "IPv6":
		actualMacEntries = otg.Telemetry().InterfaceAny().Ipv6NeighborAny().LinkLayerAddress().Get(t)
	}

	t.Log("Expected Mac Entries:", expectedMacEntries)
	t.Log("OTG Mac Entries:", actualMacEntries)

	expected = expectedElementsPresent(expectedMacEntries, actualMacEntries)
	return expected
}

// ArpEntriesPresent returns true once ARP entries are present.
func ArpEntriesPresent(t *testing.T, otg *ondatra.OTG, ipType string) bool {
	actualMacEntries := []string{}
	switch ipType {
	case "ipv4":
		actualMacEntries = otg.Telemetry().InterfaceAny().Ipv4NeighborAny().LinkLayerAddress().Get(t)
	case "ipv6":
		actualMacEntries = otg.Telemetry().InterfaceAny().Ipv6NeighborAny().LinkLayerAddress().Get(t)
	}
	if len(actualMacEntries) == 0 {
		return false
	} else {
		return true
	}
}

func expectedElementsPresent(expected, actual []string) bool {
	exists := make(map[string]bool)
	for _, value := range actual {
		exists[value] = true
	}
	for _, value := range expected {
		if !exists[value] {
			return false
		}
	}
	return true
}
