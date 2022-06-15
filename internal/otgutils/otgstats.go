package otgutils

import (
	"log"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
)

// Function is used to retrieve the OTG Flow metrics in a gosnappi.MetricsResponseFlowMetricIter type
func GetFlowMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.MetricsResponseFlowMetricIter, error) {
	defer Timer(time.Now(), "GetFlowMetrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().FlowMetrics()
	for _, f := range c.Flows().Items() {
		log.Printf("Getting flow metrics for flow %s\n", f.Name())
		fMetric := metrics.Add()
		recvMetric := otg.Telemetry().Flow(f.Name()).Get(t)
		fMetric.SetName(recvMetric.GetName())
		fMetric.SetFramesRx(int64(recvMetric.GetCounters().GetInPkts()))
		fMetric.SetFramesTx(int64(recvMetric.GetCounters().GetOutPkts()))
		fMetric.SetFramesTxRate(ygot.BinaryToFloat32(recvMetric.GetOutFrameRate()))
		fMetric.SetFramesRxRate(ygot.BinaryToFloat32(recvMetric.GetInFrameRate()))
	}
	return metrics, nil
}

// Function is used to retrieve some OTG Port metrics in a gosnappi.MetricsResponsePortMetricIter type
func GetPortMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.MetricsResponsePortMetricIter, error) {
	defer Timer(time.Now(), "GetPortMetrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().PortMetrics()
	for _, p := range c.Ports().Items() {
		log.Printf("Getting port metrics for port %s\n", p.Name())
		pMetric := metrics.Add()
		recvMetric := otg.Telemetry().Port(p.Name()).Get(t)
		pMetric.SetName(recvMetric.GetName())
		pMetric.SetFramesTx(int64(recvMetric.GetCounters().GetOutFrames()))
		pMetric.SetFramesRx(int64(recvMetric.GetCounters().GetInFrames()))
		pMetric.SetFramesTxRate(ygot.BinaryToFloat32(recvMetric.GetOutRate()))
	}
	return metrics, nil
}

// Function is used to retrieve all OTG flow metrics in a gosnappi.MetricsResponsePortMetricIter type
func GetAllPortMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.MetricsResponsePortMetricIter, error) {
	defer Timer(time.Now(), "GetPortMetrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().PortMetrics()
	for _, p := range c.Ports().Items() {
		log.Printf("Getting port metrics for port %s\n", p.Name())
		pMetric := metrics.Add()
		recvMetric := otg.Telemetry().Port(p.Name()).Get(t)
		pMetric.SetName(recvMetric.GetName())
		pMetric.SetFramesTx(int64(recvMetric.GetCounters().GetOutFrames()))
		pMetric.SetFramesRx(int64(recvMetric.GetCounters().GetInFrames()))
		pMetric.SetBytesTx(int64(recvMetric.GetCounters().GetOutOctets()))
		pMetric.SetBytesRx(int64(recvMetric.GetCounters().GetInOctets()))
		pMetric.SetFramesTxRate(ygot.BinaryToFloat32(recvMetric.GetOutRate()))
		pMetric.SetFramesRxRate(ygot.BinaryToFloat32(recvMetric.GetInRate()))
		link := recvMetric.GetLink()
		if link == otgtelemetry.Port_Link_UP {
			pMetric.SetLink("up")
		} else {
			pMetric.SetLink("down")
		}

	}
	return metrics, nil
}

// Function is used to retrieve all OTG flow metrics in a gosnappi.MetricsResponseBgpv4MetricIter type
func GetBgpv4Metrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.MetricsResponseBgpv4MetricIter, error) {
	defer Timer(time.Now(), "GetBgpv4Metrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().Bgpv4Metrics()
	for _, d := range c.Devices().Items() {
		bgp := d.Bgp()
		for _, ip := range bgp.Ipv4Interfaces().Items() {
			for _, peer := range ip.Peers().Items() {
				log.Printf("Getting bgpv4 metrics for peer %s\n", peer.Name())
				bgpv4Metric := metrics.Add()
				recvMetric := otg.Telemetry().BgpPeer(peer.Name()).Get(t)
				bgpv4Metric.SetName(recvMetric.GetName())
				bgpv4Metric.SetSessionFlapCount(int32(recvMetric.GetCounters().GetFlaps()))
				bgpv4Metric.SetRoutesAdvertised(int32(recvMetric.GetCounters().GetOutRoutes()))
				bgpv4Metric.SetRoutesReceived(int32(recvMetric.GetCounters().GetInRoutes()))
				bgpv4Metric.SetRouteWithdrawsSent(int32(recvMetric.GetCounters().GetOutRouteWithdraw()))
				bgpv4Metric.SetRouteWithdrawsReceived(int32(recvMetric.GetCounters().GetInRouteWithdraw()))
				bgpv4Metric.SetKeepalivesSent(int32(recvMetric.GetCounters().GetOutKeepalives()))
				bgpv4Metric.SetKeepalivesReceived(int32(recvMetric.GetCounters().GetInKeepalives()))
				sessionState := recvMetric.GetSessionState()
				if sessionState == otgtelemetry.BgpPeer_SessionState_ESTABLISHED {
					bgpv4Metric.SetSessionState("up")
				} else {
					bgpv4Metric.SetSessionState("down")
				}
			}
		}
	}
	return metrics, nil
}

// Function is used to retrieve all OTG flow metrics in a gosnappi.MetricsResponseBgpv6MetricIter type
func GetBgpv6Metrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.MetricsResponseBgpv6MetricIter, error) {
	defer Timer(time.Now(), "GetBgpv6Metrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().Bgpv6Metrics()
	for _, d := range c.Devices().Items() {
		bgp := d.Bgp()
		for _, ipv6 := range bgp.Ipv6Interfaces().Items() {
			for _, peer := range ipv6.Peers().Items() {
				log.Printf("Getting bgpv6 metrics for peer %s\n", peer.Name())
				bgpv6Metric := metrics.Add()
				recvMetric := otg.Telemetry().BgpPeer(peer.Name()).Get(t)
				bgpv6Metric.SetName(recvMetric.GetName())
				bgpv6Metric.SetSessionFlapCount(int32(recvMetric.GetCounters().GetFlaps()))
				bgpv6Metric.SetRoutesAdvertised(int32(recvMetric.GetCounters().GetOutRoutes()))
				bgpv6Metric.SetRoutesReceived(int32(recvMetric.GetCounters().GetInRoutes()))
				bgpv6Metric.SetRouteWithdrawsSent(int32(recvMetric.GetCounters().GetOutRouteWithdraw()))
				bgpv6Metric.SetRouteWithdrawsReceived(int32(recvMetric.GetCounters().GetInRouteWithdraw()))
				bgpv6Metric.SetKeepalivesSent(int32(recvMetric.GetCounters().GetOutKeepalives()))
				bgpv6Metric.SetKeepalivesReceived(int32(recvMetric.GetCounters().GetInKeepalives()))
				sessionState := recvMetric.GetSessionState()
				if sessionState == otgtelemetry.BgpPeer_SessionState_ESTABLISHED {
					bgpv6Metric.SetSessionState("up")
				} else {
					bgpv6Metric.SetSessionState("down")
				}
			}
		}
	}
	return metrics, nil
}

// This function is used to retrieve the mac address of an ipv4 neighbour given its IP address and the iterface name of the OTG
// Fails if no entries are present in the ARP table
func GetIPv4NeighborMacEntry(t *testing.T, interfaceName string, ipAddress string, otg *ondatra.OTG) (string, error) {
	entries := otg.Telemetry().Interface(interfaceName).Ipv4Neighbor(ipAddress).LinkLayerAddress().Get(t)
	return entries, nil
}

// This function is used to retrieve the mac address of an IPv4 neighbour given its IP address and the iterface name of the OTG
// Returns empty array no entries are present in the ARP table
func GetAllIPv4NeighborMacEntries(t *testing.T, otg *ondatra.OTG) ([]string, error) {
	macEntries := otg.Telemetry().InterfaceAny().Ipv4NeighborAny().LinkLayerAddress().Get(t)
	return macEntries, nil
}

// This function is used to retrieve the mac address of an IPv6 neighbour given its IP address and the iterface name of the OTG
// Returns empty array no entries are present in the ARP table
func GetAllIPv6NeighborMacEntries(t *testing.T, otg *ondatra.OTG) ([]string, error) {
	macEntries := otg.Telemetry().InterfaceAny().Ipv6NeighborAny().LinkLayerAddress().Get(t)
	return macEntries, nil
}
