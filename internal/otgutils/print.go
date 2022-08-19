// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otgutils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	otg "github.com/openconfig/ondatra/otg"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
)

// LogFlowMetrics displays the otg flow statistics.
func LogFlowMetrics(t testing.TB, otg *otg.OTG, c gosnappi.Config) {
	t.Helper()
	var out strings.Builder
	out.WriteString("\nFlow Metrics\n")
	fmt.Fprintln(&out, strings.Repeat("-", 80))
	out.WriteString("\n")
	fmt.Fprintf(&out, "%-25v%-15v%-15v%-15v%-15v\n", "Name", "Frames Tx", "Frames Rx", "FPS Tx", "FPS Rx")
	for _, f := range c.Flows().Items() {
		flowMetrics := otg.Telemetry().Flow(f.Name()).Get(t)
		rxPkts := flowMetrics.GetCounters().GetInPkts()
		txPkts := flowMetrics.GetCounters().GetOutPkts()
		rxRate := ygot.BinaryToFloat32(flowMetrics.GetInFrameRate())
		txRate := ygot.BinaryToFloat32(flowMetrics.GetOutFrameRate())
		out.WriteString(fmt.Sprintf("%-25v%-15v%-15v%-15v%-15v\n", f.Name(), txPkts, rxPkts, txRate, rxRate))
	}
	fmt.Fprintln(&out, strings.Repeat("-", 80))
	out.WriteString("\n\n")
	t.Log(out.String())
}

// LogPortMetrics displays otg port stats.
func LogPortMetrics(t testing.TB, otg *otg.OTG, c gosnappi.Config) {
	t.Helper()
	var link string
	var out strings.Builder
	out.WriteString("\nPort Metrics\n")
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n")
	fmt.Fprintf(&out,
		"%-25s%-15s%-15s%-15s%-15s%-15s%-15s%-15s\n",
		"Name", "Frames Tx", "Frames Rx", "Bytes Tx", "Bytes Rx", "FPS Tx", "FPS Rx", "Link")
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
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n\n")
	t.Log(out.String())
}
