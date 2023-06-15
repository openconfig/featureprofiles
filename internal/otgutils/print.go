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
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"

	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
)

// LogFlowMetrics displays the otg flow statistics.
func LogFlowMetrics(t testing.TB, otg *otg.OTG, c gosnappi.Config) {
	t.Helper()
	var out strings.Builder
	out.WriteString("\nOTG Flow Metrics\n")
	fmt.Fprintln(&out, strings.Repeat("-", 80))
	out.WriteString("\n")
	fmt.Fprintf(&out, "%-25v%-15v%-15v%-15v%-15v\n", "Name", "Frames Tx", "Frames Rx", "FPS Tx", "FPS Rx")
	for _, f := range c.Flows().Items() {
		flowMetrics := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
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
	out.WriteString("\nOTG Port Metrics\n")
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n")
	fmt.Fprintf(&out,
		"%-25s%-15s%-15s%-15s%-15s%-15s%-15s%-15s\n",
		"Name", "Frames Tx", "Frames Rx", "Bytes Tx", "Bytes Rx", "FPS Tx", "FPS Rx", "Link")
	for _, p := range c.Ports().Items() {
		portMetrics := gnmi.Get(t, otg, gnmi.OTG().Port(p.Name()).State())
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

// LogLAGMetrics is displaying otg lag stats.
func LogLAGMetrics(t testing.TB, otg *otg.OTG, c gosnappi.Config) {
	t.Helper()
	var out strings.Builder
	out.WriteString("\nOTG LAG Metrics\n")
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n")
	fmt.Fprintf(&out,
		"%-25s%-15s%-15s%-15s%-20s\n",
		"Name", "Oper Status", "Frames Tx", "Frames Rx", "Member Ports UP")
	for _, lag := range c.Lags().Items() {
		lagMetrics := gnmi.Get(t, otg, gnmi.OTG().Lag(lag.Name()).State())
		operStatus := lagMetrics.GetOperStatus().String()
		memberPortsUP := lagMetrics.GetCounters().GetMemberPortsUp()
		framesTx := lagMetrics.GetCounters().GetOutFrames()
		framesRx := lagMetrics.GetCounters().GetInFrames()
		out.WriteString(fmt.Sprintf(
			"%-25v%-15v%-15v%-15v%-20v\n",
			lag.Name(), operStatus, framesTx, framesRx, memberPortsUP,
		))
	}
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n\n")
	t.Log(out.String())
}

// LogLACPMetrics is displaying otg lacp stats.
func LogLACPMetrics(t testing.TB, otg *otg.OTG, c gosnappi.Config) {
	t.Helper()
	var out strings.Builder
	out.WriteString("\nOTG LACP Metrics\n")
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n")
	fmt.Fprintf(&out,
		"%-10s%-15s%-18s%-15s%-15s%-20s%-20s\n",
		"LAG",
		"Member Port",
		"Synchronization",
		"Collecting",
		"Distributing",
		"System Id",
		"Partner Id")

	for _, lag := range c.Lags().Items() {
		lagPorts := lag.Ports().Items()
		for _, lagPort := range lagPorts {
			lacpMetric := gnmi.Get(t, otg, gnmi.OTG().Lacp().LagMember(lagPort.PortName()).State())
			synchronization := lacpMetric.GetSynchronization().String()
			collecting := lacpMetric.GetCollecting()
			distributing := lacpMetric.GetDistributing()
			systemID := lacpMetric.GetSystemId()
			partnerID := lacpMetric.GetPartnerId()
			out.WriteString(fmt.Sprintf(
				"%-10v%-15v%-18v%-15v%-15v%-20v%-20v\n",
				lag.Name(), lagPort.PortName(), synchronization, collecting, distributing, systemID, partnerID,
			))

		}
	}
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n\n")
	t.Log(out.String())
}

// LogLLDPMetrics is displaying otg lldp stats.
func LogLLDPMetrics(t testing.TB, otg *otg.OTG, c gosnappi.Config) {
	t.Helper()
	var out strings.Builder
	out.WriteString("\nOTG LLDP Metrics\n")
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n")
	fmt.Fprintf(&out,
		"%-15s%-15s%-15s%-18s%-20s%-18s%-18s\n",
		"Name",
		"Frames Tx",
		"Frames Rx",
		"Frames Error Rx",
		"Frames Discard",
		"Tlvs Discard",
		"Tlvs Unknown")

	for _, lldp := range c.Lldp().Items() {
		lldpMetric := gnmi.Get(t, otg, gnmi.OTG().LldpInterface(lldp.Name()).Counters().State())
		framesTx := lldpMetric.GetFrameOut()
		framesRx := lldpMetric.GetFrameIn()
		framesErrorRx := lldpMetric.GetFrameErrorIn()
		framesDiscard := lldpMetric.GetFrameDiscard()
		tlvsDiscard := lldpMetric.GetTlvDiscard()
		tlvsUnknown := lldpMetric.GetTlvUnknown()
		out.WriteString(fmt.Sprintf(
			"%-15v%-15v%-15v%-18v%-20v%-18v%-18v\n",
			lldp.Name(), framesTx, framesRx, framesErrorRx, framesDiscard, tlvsDiscard, tlvsUnknown,
		))
	}
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n\n")
	t.Log(out.String())
}

// LogLLDPNeighborStates is displaying otg lldp neighbor states.
func LogLLDPNeighborStates(t testing.TB, otg *otg.OTG, c gosnappi.Config) {
	t.Helper()
	var out strings.Builder
	out.WriteString("\nOTG LLDP Neighbor States\n")
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n")
	fmt.Fprintf(&out,
		"%-15s%-18s%-18s%-18s%-20s%-20s\n",
		"LLDP Name",
		"System Name",
		"Port Id",
		"Port Id Type",
		"Chassis Id",
		"Chassis Id Type")

	for _, lldp := range c.Lldp().Items() {
		lldpNeighborStates := gnmi.LookupAll(t, otg, gnmi.OTG().LldpInterface(lldp.Name()).LldpNeighborDatabase().LldpNeighborAny().State())
		for _, lldpNeighborState := range lldpNeighborStates {
			v, isPresent := lldpNeighborState.Val()
			if isPresent {
				systemName := v.GetSystemName()
				portID := v.GetPortId()
				portIDType := v.GetPortIdType()
				chassisID := v.GetChassisId()
				chassisIDType := v.GetChassisIdType()
				out.WriteString(fmt.Sprintf(
					"%-15s%-18s%-18s%-18s%-20s%-20s\n",
					lldp.Name(), systemName, portID, portIDType, chassisID, chassisIDType,
				))

			}
		}
	}
	fmt.Fprintln(&out, strings.Repeat("-", 120))
	out.WriteString("\n\n")
	t.Log(out.String())
}
