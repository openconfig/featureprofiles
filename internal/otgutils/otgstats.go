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

// GetFlowMetrics is used to retrieve the OTG Flow metrics in a gosnappi.MetricsResponseFlowMetricIter type
func GetFlowMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.MetricsResponseFlowMetricIter, error) {
	defer timer(time.Now(), "GetFlowMetrics GNMI")
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

// GetPortMetrics is used to retrieve some OTG Port metrics in a gosnappi.MetricsResponsePortMetricIter type
func GetPortMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.MetricsResponsePortMetricIter, error) {
	defer timer(time.Now(), "GetPortMetrics GNMI")
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

// GetAllPortMetrics is used to retrieve all OTG flow metrics in a gosnappi.MetricsResponsePortMetricIter type
func GetAllPortMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) (gosnappi.MetricsResponsePortMetricIter, error) {
	defer timer(time.Now(), "GetPortMetrics GNMI")
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
