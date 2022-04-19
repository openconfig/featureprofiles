package helpers

import (
	"log"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/otgtelemetry"
)

func GetFlowMetrics(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) (gosnappi.MetricsResponseFlowMetricIter, error) {
	defer Timer(time.Now(), "GetFlowMetrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().FlowMetrics()
	for _, f := range c.Flows().Items() {
		log.Printf("Getting flow metrics for flow %s\n", f.Name())
		fMetric := metrics.Add()
		fMetric.SetName(ate.OTGTelemetry().Flow(f.Name()).Name().Get(t))
		fMetric.SetFramesRx(int64(ate.OTGTelemetry().Flow(f.Name()).Counters().InPkts().Get(t)))
		fMetric.SetFramesTx(int64(ate.OTGTelemetry().Flow(f.Name()).Counters().OutPkts().Get(t)))
		// fMetric.SetFramesTxRate(ate.OTGTelemetry().Flow(f.Name()).OutFrameRate().Get(t))
		// fMetric.SetFramesRxRate(ate.OTGTelemetry().Flow(f.Name()).InFrameRate().Get(t))
		fMetric.SetFramesTxRate(0)
		fMetric.SetFramesRxRate(0)
	}
	return metrics, nil
}

func GetPortMetrics(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) (gosnappi.MetricsResponsePortMetricIter, error) {
	defer Timer(time.Now(), "GetPortMetrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().PortMetrics()
	for _, p := range c.Ports().Items() {
		log.Printf("Getting port metrics for port %s\n", p.Name())
		pMetric := metrics.Add()
		pMetric.SetName(ate.OTGTelemetry().Port(p.Name()).Name().Get(t))
		pMetric.SetFramesTx(int64(ate.OTGTelemetry().Port(p.Name()).Counters().OutFrames().Get(t)))
		pMetric.SetFramesRx(int64(ate.OTGTelemetry().Port(p.Name()).Counters().InFrames().Get(t)))
		pMetric.SetFramesTxRate(ate.OTGTelemetry().Port(p.Name()).OutRate().Get(t))
	}
	return metrics, nil
}

func GetBgpv4Metrics(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) (gosnappi.MetricsResponseBgpv4MetricIter, error) {
	defer Timer(time.Now(), "GetBgpv4Metrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().Bgpv4Metrics()
	for _, d := range c.Devices().Items() {
		bgp := d.Bgp()
		for _, ip := range bgp.Ipv4Interfaces().Items() {
			for _, peer := range ip.Peers().Items() {
				log.Printf("Getting bgpv4 metrics for peer %s\n", peer.Name())
				bgpv4Metric := metrics.Add()
				bgpv4Metric.SetName(ate.OTGTelemetry().BgpPeer(peer.Name()).Name().Get(t))
				bgpv4Metric.SetSessionFlapCount(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().Flaps().Get(t)))
				bgpv4Metric.SetRoutesAdvertised(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().OutRoutes().Get(t)))
				bgpv4Metric.SetRoutesReceived(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().InRoutes().Get(t)))
				bgpv4Metric.SetRouteWithdrawsSent(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().OutRouteWithdraw().Get(t)))
				bgpv4Metric.SetRouteWithdrawsReceived(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().InRouteWithdraw().Get(t)))
				bgpv4Metric.SetKeepalivesSent(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().OutKeepalives().Get(t)))
				bgpv4Metric.SetKeepalivesReceived(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().InKeepalives().Get(t)))
				sessionState := ate.OTGTelemetry().BgpPeer(peer.Name()).SessionState().Get(t)
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

func GetBgpv6Metrics(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) (gosnappi.MetricsResponseBgpv6MetricIter, error) {
	defer Timer(time.Now(), "GetBgpv6Metrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().Bgpv6Metrics()
	for _, d := range c.Devices().Items() {
		bgp := d.Bgp()
		for _, ipv6 := range bgp.Ipv6Interfaces().Items() {
			for _, peer := range ipv6.Peers().Items() {
				log.Printf("Getting bgpv6 metrics for peer %s\n", peer.Name())
				bgpv6Metric := metrics.Add()
				bgpv6Metric.SetName(ate.OTGTelemetry().BgpPeer(peer.Name()).Name().Get(t))
				bgpv6Metric.SetSessionFlapCount(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().Flaps().Get(t)))
				bgpv6Metric.SetRoutesAdvertised(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().OutRoutes().Get(t)))
				bgpv6Metric.SetRoutesReceived(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().InRoutes().Get(t)))
				bgpv6Metric.SetRouteWithdrawsSent(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().OutRouteWithdraw().Get(t)))
				bgpv6Metric.SetRouteWithdrawsReceived(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().InRouteWithdraw().Get(t)))
				bgpv6Metric.SetKeepalivesSent(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().OutKeepalives().Get(t)))
				bgpv6Metric.SetKeepalivesReceived(int32(ate.OTGTelemetry().BgpPeer(peer.Name()).Counters().InKeepalives().Get(t)))
				sessionState := ate.OTGTelemetry().BgpPeer(peer.Name()).SessionState().Get(t)
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

func GetIsisMetrics(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) (gosnappi.MetricsResponseIsisMetricIter, error) {
	defer Timer(time.Now(), "GetIsisMetrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().IsisMetrics()
	for _, d := range c.Devices().Items() {
		isis := d.Isis()
		// log.Printf("Getting isis metrics for router %s\n", isis.Name())
		isisMetric := metrics.Add()
		isisMetric.SetName(ate.OTGTelemetry().IsisRouter(isis.Name()).Name().Get(t))
		isisMetric.SetL1SessionsUp(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level1().SessionsUp().Get(t)))
		isisMetric.SetL1SessionFlap(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level1().SessionsFlap().Get(t)))
		isisMetric.SetL1BroadcastHellosSent(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level1().OutBcastHellos().Get(t)))
		isisMetric.SetL1BroadcastHellosReceived(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level1().InBcastHellos().Get(t)))
		isisMetric.SetL1PointToPointHellosSent(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level1().OutP2PHellos().Get(t)))
		isisMetric.SetL1PointToPointHellosReceived(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level1().InP2PHellos().Get(t)))
		isisMetric.SetL1LspSent(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level1().OutLsp().Get(t)))
		isisMetric.SetL1LspReceived(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level1().InLsp().Get(t)))
		isisMetric.SetL1DatabaseSize(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level1().DatabaseSize().Get(t)))
		isisMetric.SetL2SessionsUp(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level2().SessionsUp().Get(t)))
		isisMetric.SetL2SessionFlap(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level2().SessionsFlap().Get(t)))
		isisMetric.SetL2BroadcastHellosSent(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level2().OutBcastHellos().Get(t)))
		isisMetric.SetL2BroadcastHellosReceived(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level2().InBcastHellos().Get(t)))
		isisMetric.SetL2PointToPointHellosSent(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level2().OutP2PHellos().Get(t)))
		isisMetric.SetL2PointToPointHellosReceived(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level2().InP2PHellos().Get(t)))
		isisMetric.SetL2LspSent(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level2().OutLsp().Get(t)))
		isisMetric.SetL2LspReceived(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level2().InLsp().Get(t)))
		isisMetric.SetL2DatabaseSize(int32(ate.OTGTelemetry().IsisRouter(isis.Name()).Counters().Level2().DatabaseSize().Get(t)))
	}
	return metrics, nil
}
