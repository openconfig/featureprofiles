package otgutils

import (
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/protojson"
)

// using protojson to marshal will emit property names with lowerCamelCase
// instead of snake_case
var protoMarshaller = protojson.MarshalOptions{UseProtoNames: true}
var prettyProtoMarshaller = protojson.MarshalOptions{UseProtoNames: true, Multiline: true}

// timer prints time elapsed in ms since a given start time
func timer(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %d ms", name, elapsed.Milliseconds())
}

// WatchFlowMetrics is displaying flow stats for the given timeout duration
func WatchFlowMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config, opts *WaitForOpts) error {
	start := time.Now()
	for {
		border := strings.Repeat("-", 32*3+10)
		var out string
		out += "\nFlow Metrics\n" + border + "\n"
		out += fmt.Sprintf("%-25s%-25s%-25s%-25s%-25s\n", "Name", "Frames Tx", "Frames Rx", "FPS Tx", "FPS Rx")
		for _, f := range c.Flows().Items() {
			flowMetrics := otg.Telemetry().Flow(f.Name()).Get(t)
			rxPkts := flowMetrics.GetCounters().GetInPkts()
			txPkts := flowMetrics.GetCounters().GetOutPkts()
			rxRate := ygot.BinaryToFloat32(flowMetrics.GetInFrameRate())
			txRate := ygot.BinaryToFloat32(flowMetrics.GetOutFrameRate())
			out += fmt.Sprintf("%-25v%-25v%-25v%-25v%-25v\n", f.Name(), txPkts, rxPkts, txRate, rxRate)
		}
		out += border + "\n\n"
		log.Println(out)
		if time.Since(start) > opts.Timeout {
			return nil
		}
		time.Sleep(opts.Interval)
	}
}

func WatchPortMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config, opts *WaitForOpts) error {
	start := time.Now()
	for {
		var link, out string
		border := strings.Repeat("-", 15*8-10)
		out += "\nPort Metrics\n" + border + "\n"
		out += fmt.Sprintf(
			"%-15s%-15s%-15s%-15s%-15s%-15s%-15s%-15s\n",
			"Name", "Frames Tx", "Frames Rx", "Bytes Tx", "Bytes Rx", "FPS Tx", "FPS Rx", "Link",
		)
		for _, p := range c.Ports().Items() {
			portMetrics := otg.Telemetry().Port(p.Name()).Get(t)
			rxFrames := portMetrics.GetCounters().GetInFrames()
			txFrames := portMetrics.GetCounters().GetOutFrames()
			rxRate := ygot.BinaryToFloat32(portMetrics.GetInRate())
			txRate := ygot.BinaryToFloat32(portMetrics.GetOutRate())
			rxBytes := portMetrics.GetCounters().GetInOctets()
			txBytes := portMetrics.GetCounters().GetOutOctets()
			if portMetrics.GetLink() == otgtelemetry.Port_Link_UP {
				link = "up"
			} else {
				link = "down"
			}
			out += fmt.Sprintf(
				"%-15v%-15v%-15v%-15v%-15v%-15v%-15v%-15v\n",
				p.Name(), txFrames, rxFrames, txBytes, rxBytes, txRate, rxRate, link,
			)
		}
		out += border + "\n\n"
		log.Println(out)
		if time.Since(start) > opts.Timeout {
			return nil
		}
		time.Sleep(opts.Interval)
	}
}
