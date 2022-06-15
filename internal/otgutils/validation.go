package otgutils

import (
	"fmt"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
)

// Struct used for validating the fetched OTG BGP stats
type ExpectedBgpMetrics struct {
	Advertised int32
	Received   int32
}

// Struct used for validating the fetched OTG ISIS stats
type ExpectedIsisMetrics struct {
	L1SessionsUp   int32
	L2SessionsUp   int32
	L1DatabaseSize int32
	L2DatabaseSize int32
}

// Struct used for validating the fetched OTG Port stats
type ExpectedPortMetrics struct {
	FramesRx int32
}

// Struct used for validating the fetched OTG Flow stats
type ExpectedFlowMetrics struct {
	FramesRx     int64
	FramesRxRate float32
}

// Struct used for creating expected otg metrics
type ExpectedState struct {
	Port map[string]ExpectedPortMetrics
	Flow map[string]ExpectedFlowMetrics
	Bgp4 map[string]ExpectedBgpMetrics
	Bgp6 map[string]ExpectedBgpMetrics
	Isis map[string]ExpectedIsisMetrics
}

func AllBgp4SessionUp(t *testing.T, otg *ondatra.OTG, c gosnappi.Config, expectedState ExpectedState) (bool, error) {
	dMetrics, err := GetBgpv4Metrics(t, otg, c)
	if err != nil {
		return false, err
	}

	PrintMetricsTable(&MetricsTableOpts{
		ClearPrevious: false,
		Bgpv4Metrics:  dMetrics,
	})

	expected := true
	for _, d := range dMetrics.Items() {
		expectedMetrics := expectedState.Bgp4[d.Name()]
		if d.SessionState() != gosnappi.Bgpv4MetricSessionState.UP || d.RoutesAdvertised() != expectedMetrics.Advertised || d.RoutesReceived() != expectedMetrics.Received {
			expected = false
		}
	}

	return expected, nil
}

func AllBgp6SessionUp(t *testing.T, otg *ondatra.OTG, c gosnappi.Config, expectedState ExpectedState) (bool, error) {
	dMetrics, err := GetBgpv6Metrics(t, otg, c)
	if err != nil {
		return false, err
	}

	PrintMetricsTable(&MetricsTableOpts{
		ClearPrevious: false,
		Bgpv6Metrics:  dMetrics,
	})

	expected := true
	for _, d := range dMetrics.Items() {
		expectedMetrics := expectedState.Bgp6[d.Name()]
		if d.SessionState() != gosnappi.Bgpv6MetricSessionState.UP || d.RoutesAdvertised() != expectedMetrics.Advertised || d.RoutesReceived() != expectedMetrics.Received {
			expected = false
		}
	}

	return expected, nil
}

func AllBgp4SessionDown(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (bool, error) {
	dMetrics, err := GetBgpv4Metrics(t, otg, c)
	if err != nil {
		return false, err
	}

	PrintMetricsTable(&MetricsTableOpts{
		ClearPrevious: false,
		Bgpv4Metrics:  dMetrics,
	})

	expected := true
	for _, d := range dMetrics.Items() {
		if d.SessionState() != gosnappi.Bgpv4MetricSessionState.DOWN {
			expected = false
		}
	}

	return expected, nil
}

func AllBgp6SessionDown(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (bool, error) {
	dMetrics, err := GetBgpv6Metrics(t, otg, c)
	if err != nil {
		return false, err
	}

	PrintMetricsTable(&MetricsTableOpts{
		ClearPrevious: false,
		Bgpv6Metrics:  dMetrics,
	})

	expected := true
	for _, d := range dMetrics.Items() {
		if d.SessionState() != gosnappi.Bgpv6MetricSessionState.DOWN {
			expected = false
		}
	}

	return expected, nil
}

func FlowMetricsOk(t *testing.T, otg *ondatra.OTG, c gosnappi.Config, expectedState ExpectedState) (bool, error) {
	fMetrics, err := GetFlowMetrics(t, otg, c)
	if err != nil {
		return false, err
	}

	PrintMetricsTable(&MetricsTableOpts{
		ClearPrevious: false,
		FlowMetrics:   fMetrics,
	})

	expected := true
	for _, f := range fMetrics.Items() {
		expectedMetrics := expectedState.Flow[f.Name()]
		if f.FramesRx() != expectedMetrics.FramesRx || f.FramesRxRate() != expectedMetrics.FramesRxRate {
			expected = false
		}
	}

	return expected, nil
}

func ArpEntriesOk(t *testing.T, otg *ondatra.OTG, ipType string, expectedMacEntries []string) (bool, error) {
	actualMacEntries := []string{}
	var err error
	switch ipType {
	case "IPv4":
		actualMacEntries, err = GetAllIPv4NeighborMacEntries(t, otg)
		if err != nil {
			return false, err
		}
	case "IPv6":
		actualMacEntries, err = GetAllIPv6NeighborMacEntries(t, otg)
		if err != nil {
			return false, err
		}
	}

	t.Logf("Expected Mac Entries: %v", expectedMacEntries)
	t.Logf("OTG Mac Entries: %v", actualMacEntries)

	expected := true
	expected = expectedElementsPresent(expectedMacEntries, actualMacEntries)
	return expected, nil
}

func ArpEntriesPresent(t *testing.T, otg *ondatra.OTG, ipType string) (bool, error) {
	actualMacEntries := []string{}
	var err error
	switch ipType {
	case "ipv4":
		actualMacEntries, err = GetAllIPv4NeighborMacEntries(t, otg)
	case "ipv6":
		actualMacEntries, err = GetAllIPv6NeighborMacEntries(t, otg)
	}
	if err != nil {
		return false, fmt.Errorf("failed to get the ARP entries for %v", ipType)
	} else {
		if len(actualMacEntries) == 0 {
			return false, nil
		}
		return true, nil
	}
}
