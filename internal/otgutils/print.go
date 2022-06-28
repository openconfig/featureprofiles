package otgutils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
)

// PrintFlowMetrics is displaying the otg flow statistics.
func PrintFlowMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) {
	var out strings.Builder
	out.WriteString("\nFlow Metrics\n")
	for i := 1; i <= 80; i++ {
		out.WriteString("-")
	}
	out.WriteString("\n")
	out.WriteString(fmt.Sprintf("%-25v%-15v%-15v%-15v%-15v\n", "Name", "Frames Tx", "Frames Rx", "FPS Tx", "FPS Rx"))
	for _, f := range c.Flows().Items() {
		flowMetrics := otg.Telemetry().Flow(f.Name()).Get(t)
		rxPkts := flowMetrics.GetCounters().GetInPkts()
		txPkts := flowMetrics.GetCounters().GetOutPkts()
		rxRate := ygot.BinaryToFloat32(flowMetrics.GetInFrameRate())
		txRate := ygot.BinaryToFloat32(flowMetrics.GetOutFrameRate())
		out.WriteString(fmt.Sprintf("%-25v%-15v%-15v%-15v%-15v\n", f.Name(), txPkts, rxPkts, txRate, rxRate))
	}
	for i := 1; i <= 80; i++ {
		out.WriteString("-")
	}
	out.WriteString("\n\n")
	t.Log(out.String())
}

// PrintPortMetrics is displaying otg port stats.
func PrintPortMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config) {
	var link string
	var out strings.Builder
	out.WriteString("\nPort Metrics\n")
	for i := 1; i <= 120; i++ {
		out.WriteString("-")
	}
	out.WriteString("\n")
	out.WriteString(fmt.Sprintf(
		"%-25s%-15s%-15s%-15s%-15s%-15s%-15s%-15s\n",
		"Name", "Frames Tx", "Frames Rx", "Bytes Tx", "Bytes Rx", "FPS Tx", "FPS Rx", "Link"))
	for _, p := range c.Ports().Items() {
		portMetrics := otg.Telemetry().Port(p.Name()).Get(t)
		rxFrames := portMetrics.GetCounters().GetInFrames()
		txFrames := portMetrics.GetCounters().GetOutFrames()
		rxRate := ygot.BinaryToFloat32(portMetrics.GetInRate())
		txRate := ygot.BinaryToFloat32(portMetrics.GetOutRate())
		rxBytes := portMetrics.GetCounters().GetInOctets()
		txBytes := portMetrics.GetCounters().GetOutOctets()
		link = "down"
		if portMetrics.GetLink() == otgtelemetry.Port_Link_UP {
			link = "up"
		}
		out.WriteString(fmt.Sprintf(
			"%-25v%-15v%-15v%-15v%-15v%-15v%-15v%-15v\n",
			p.Name(), txFrames, rxFrames, txBytes, rxBytes, txRate, rxRate, link,
		))
	}
	for i := 1; i <= 120; i++ {
		out.WriteString("-")
	}
	out.WriteString("\n\n")
	t.Log(out.String())
}
