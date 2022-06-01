package helpers

import (
	"log"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
)

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

func GetIsisMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.MetricsResponseIsisMetricIter, error) {
	defer Timer(time.Now(), "GetIsisMetrics GNMI")
	metrics := gosnappi.NewApi().NewGetMetricsResponse().StatusCode200().IsisMetrics()
	for _, d := range c.Devices().Items() {
		isis := d.Isis()
		log.Printf("Getting isis metrics for router %s\n", isis.Name())
		isisMetric := metrics.Add()
		recvMetric := otg.Telemetry().IsisRouter(isis.Name()).Get(t)
		isisMetric.SetName(recvMetric.GetName())
		isisMetric.SetL1SessionsUp(int32(recvMetric.GetCounters().GetLevel1().GetSessionsUp()))
		isisMetric.SetL1SessionFlap(int32(recvMetric.GetCounters().GetLevel1().GetSessionsFlap()))
		isisMetric.SetL1BroadcastHellosSent(int32(recvMetric.GetCounters().GetLevel1().GetOutBcastHellos()))
		isisMetric.SetL1BroadcastHellosReceived(int32(recvMetric.GetCounters().GetLevel1().GetInBcastHellos()))
		isisMetric.SetL1PointToPointHellosSent(int32(recvMetric.GetCounters().GetLevel1().GetOutP2PHellos()))
		isisMetric.SetL1PointToPointHellosReceived(int32(recvMetric.GetCounters().GetLevel1().GetInP2PHellos()))
		isisMetric.SetL1LspSent(int32(recvMetric.GetCounters().GetLevel1().GetOutLsp()))
		isisMetric.SetL1LspReceived(int32(recvMetric.GetCounters().GetLevel1().GetInLsp()))
		isisMetric.SetL1DatabaseSize(int32(recvMetric.GetCounters().GetLevel1().GetDatabaseSize()))
		isisMetric.SetL2SessionsUp(int32(recvMetric.GetCounters().GetLevel2().GetSessionsUp()))
		isisMetric.SetL2SessionFlap(int32(recvMetric.GetCounters().GetLevel2().GetSessionsFlap()))
		isisMetric.SetL2BroadcastHellosSent(int32(recvMetric.GetCounters().GetLevel2().GetOutBcastHellos()))
		isisMetric.SetL2BroadcastHellosReceived(int32(recvMetric.GetCounters().GetLevel2().GetInBcastHellos()))
		isisMetric.SetL2PointToPointHellosSent(int32(recvMetric.GetCounters().GetLevel2().GetOutP2PHellos()))
		isisMetric.SetL2PointToPointHellosReceived(int32(recvMetric.GetCounters().GetLevel2().GetInP2PHellos()))
		isisMetric.SetL2LspSent(int32(recvMetric.GetCounters().GetLevel2().GetOutLsp()))
		isisMetric.SetL2LspReceived(int32(recvMetric.GetCounters().GetLevel2().GetInLsp()))
		isisMetric.SetL2DatabaseSize(int32(recvMetric.GetCounters().GetLevel2().GetDatabaseSize()))
	}
	return metrics, nil
}

func GetIPv4NeighborStates(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.StatesResponseNeighborsv4StateIter, error) {
	defer Timer(time.Now(), "Getting IPv4 Neighbor states GNMI")
	ethNeighborMap := make(map[string][]string)
	ethernetNames := []string{}
	for _, d := range c.Devices().Items() {
		for _, eth := range d.Ethernets().Items() {
			ethernetNames = append(ethernetNames, eth.Name())
			if _, found := ethNeighborMap[eth.Name()]; !found {
				ethNeighborMap[eth.Name()] = []string{}
			}
			for _, ipv4Address := range eth.Ipv4Addresses().Items() {
				ethNeighborMap[eth.Name()] = append(ethNeighborMap[eth.Name()], ipv4Address.Gateway())
			}
		}
	}

	states := gosnappi.NewApi().NewGetStatesResponse().StatusCode200().Ipv4Neighbors()
	for _, ethernetName := range ethernetNames {
		log.Printf("Fetching IPv4 Neighbor states for ethernet: %v", ethernetName)
		for _, address := range ethNeighborMap[ethernetName] {
			recvState := otg.Telemetry().Interface(ethernetName).Ipv4Neighbor(address).Get(t)
			states.Add().
				SetEthernetName(ethernetName).
				SetIpv4Address(recvState.GetIpv4Address()).
				SetLinkLayerAddress(recvState.GetLinkLayerAddress())
		}
	}
	return states, nil
}

func GetIPv6NeighborStates(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.StatesResponseNeighborsv6StateIter, error) {
	defer Timer(time.Now(), "Getting IPv6 Neighbor states GNMI")
	ethNeighborMap := make(map[string][]string)
	ethernetNames := []string{}
	for _, d := range c.Devices().Items() {
		for _, eth := range d.Ethernets().Items() {
			ethernetNames = append(ethernetNames, eth.Name())
			if _, found := ethNeighborMap[eth.Name()]; !found {
				ethNeighborMap[eth.Name()] = []string{}
			}
			for _, ipv6Address := range eth.Ipv6Addresses().Items() {
				ethNeighborMap[eth.Name()] = append(ethNeighborMap[eth.Name()], ipv6Address.Gateway())
			}
		}
	}

	states := gosnappi.NewApi().NewGetStatesResponse().StatusCode200().Ipv6Neighbors()
	for _, ethernetName := range ethernetNames {
		log.Printf("Fetching IPv6 Neighbor states for ethernet: %v", ethernetName)
		for _, address := range ethNeighborMap[ethernetName] {
			recvState := otg.Telemetry().Interface(ethernetName).Ipv6Neighbor(address).Get(t)
			states.Add().
				SetEthernetName(ethernetName).
				SetIpv6Address(recvState.GetIpv6Address()).
				SetLinkLayerAddress(recvState.GetLinkLayerAddress())
		}
	}
	return states, nil
}

func GetAllIPv4NeighborMacEntries(t *testing.T, otg *ondatra.OTG) ([]string, error) {
	macEntries := otg.Telemetry().InterfaceAny().Ipv4NeighborAny().LinkLayerAddress().Get(t)
	return macEntries, nil
}

func GetAllIPv6NeighborMacEntries(t *testing.T, otg *ondatra.OTG) ([]string, error) {
	macEntries := otg.Telemetry().InterfaceAny().Ipv6NeighborAny().LinkLayerAddress().Get(t)
	return macEntries, nil
}
