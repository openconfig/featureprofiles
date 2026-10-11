// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package polarization_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/netip"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	trafficPps   = 40000
	totalPackets = 500000
	batchSize    = 28000
	// replayBatchSize keeps replay rounds under stricter value-list resource limits
	// that some OTG software/hardware combinations enforce.
	replayBatchSize = 15000

	// tolerancePct is the acceptable deviation from the expected per-port
	// distribution (imbalance), expressed as a percentage of total packets.
	// Distinct from lossTolerancePct. Weighted ECMP imbalance must stay within 2%.
	tolerancePct = 2

	// lossTolerancePct bounds end-to-end traffic loss. The gRIBI forwarding tree
	// spans both LAGs, so any meaningful loss means the DUT dropped forwarded
	// traffic on one of ports2-5. Kept tight because LAG/ECMP forwarding of the
	// installed routes is expected to be lossless.
	lossTolerancePct = 0.2

	// All test traffic and control-plane addresses use only reserved ranges —
	// RFC 5737 documentation blocks (192.0.2.0/24, 198.51.100.0/24,
	// 203.0.113.0/24) and the RFC 2544 benchmarking block (198.18.0.0/15) — so a
	// misconfigured DUT cannot leak or spoof real-network traffic.
	dstPfx = "198.51.100.0/24" // RFC 5737 TEST-NET-2.
	dstIP  = "198.51.100.66"   // Fixed traffic destination within dstPfx.

	// NHG weights — adjust these to control traffic distribution.
	// NHG 101 (top-level): splits between NHG 2010 and NHG 3000.
	nhg101WeightA = 8 // toward NHG 2010 (NH 1501 on LAG1 + NH 1601 on LAG2)
	nhg101WeightB = 1 // toward NHG 3000 (NH 1602 + NH 1603, both on LAG2)
	// NHG 2010: splits between NH 1501 (LAG1) and NH 1601 (LAG2).
	nhg2010WeightA = 8 // toward NH 1501 (LAG1: port2,port3)
	nhg2010WeightB = 1 // toward NH 1601 (LAG2: port4,port5)
	// NHG 3000: splits between NH 1602 and NH 1603 (both LAG2).
	nhg3000WeightA = 1 // toward NH 1602 (LAG2: port4,port5)
	nhg3000WeightB = 1 // toward NH 1603 (LAG2: port4,port5)

	plen24 = 24
	plen30 = 30
)

// ateLAG describes a static ATE LAG and the next-hop devices behind it.
type ateLAG struct {
	name  string
	id    uint32
	ports []string
	macs  []string
	nhs   []attrs.Attributes
	gw    *attrs.Attributes
}

type packetTuples struct {
	outerSrcIP     string
	outerDstIP     string
	outerL4Proto   int
	outerSrcL4Port int
	outerDstL4Port int
}

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: plen30,
	}
	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		Desc:    "ATE Port 1",
		IPv4:    "192.0.2.2",
		IPv4Len: plen30,
	}
	dutLAG1 = attrs.Attributes{
		Desc:    "DUT LAG1",
		IPv4:    "198.19.1.1",
		IPv4Len: plen24,
	}
	dutLAG2 = attrs.Attributes{
		Desc:    "DUT LAG2",
		IPv4:    "198.19.2.1",
		IPv4Len: plen24,
	}

	// The ATE mirrors the DUT bundles with static LAGs, so ARP resolves no
	// matter which member the DUT picks for a request or reply. Each gRIBI
	// next-hop is an ATE device on one of these LAGs.
	ateLAG1 = ateLAG{
		name:  "lag1",
		id:    1,
		ports: []string{"port2", "port3"},
		macs:  []string{"02:00:23:01:01:02", "02:00:23:01:01:03"},
		nhs: []attrs.Attributes{
			{Name: "nh1501", MAC: "02:00:23:00:15:01", IPv4: "198.19.1.23", IPv4Len: plen24},
		},
		gw: &dutLAG1,
	}
	ateLAG2 = ateLAG{
		name:  "lag2",
		id:    2,
		ports: []string{"port4", "port5"},
		macs:  []string{"02:00:45:01:01:01", "02:00:45:01:01:02"},
		nhs: []attrs.Attributes{
			{Name: "nh1601", MAC: "02:00:45:00:16:01", IPv4: "198.19.2.24", IPv4Len: plen24},
			{Name: "nh1602", MAC: "02:00:45:00:16:02", IPv4: "198.19.2.2", IPv4Len: plen24},
			{Name: "nh1603", MAC: "02:00:45:00:16:03", IPv4: "198.19.2.3", IPv4Len: plen24},
		},
		gw: &dutLAG2,
	}

	numRE = regexp.MustCompile(`\d+`)

	replayDstPrefix = netip.MustParsePrefix(dstPfx)
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func populatePackets(count int) []packetTuples {
	// Source addresses are drawn exclusively from the RFC 2544 benchmarking block
	// 198.18.0.0/16 (within 198.18.0.0/15). Using a reserved, non-routable range
	// guarantees generated test traffic can never be mistaken for, leak onto, or
	// spoof a real public network if the DUT is misconfigured.
	octet3, octet4 := 1, 1
	srcPort, dstPort := 512, 206
	packets := make([]packetTuples, 0, count)

	for range count {
		if octet4 > 254 {
			octet3++
			octet4 = 1
		}
		if octet3 > 254 {
			octet3 = 1
		}

		packets = append(packets, packetTuples{
			outerSrcIP:     fmt.Sprintf("198.18.%d.%d", octet3, octet4),
			outerDstIP:     dstIP,
			outerL4Proto:   17,
			outerSrcL4Port: srcPort,
			outerDstL4Port: dstPort,
		})

		if srcPort > 50000 {
			srcPort = 105
		}
		if dstPort > 50000 {
			dstPort = 206
		}
		octet4++
		srcPort++
		dstPort++
	}
	return packets
}

func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	ate.OTG().SetControlState(t, cs)
}

func stopAndReadCapture(t *testing.T, ate *ondatra.ATEDevice) []packetTuples {
	t.Helper()
	otg := ate.OTG()

	t.Log("Stopping capture on port2")
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)

	t.Log("Downloading pcap from port2 (single gRPC call)")
	pcapdata := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName("port2"))
	t.Logf("Received pcap: %d bytes", len(pcapdata))

	pcapReader, err := pcapgo.NewReader(bytes.NewReader(pcapdata))
	if err != nil {
		t.Fatalf("Failed to create pcap reader: %v", err)
	}

	var captured []packetTuples
	parseErrors := 0
	for {
		data, _, err := pcapReader.ReadPacketData()
		if err == io.EOF {
			break
		} else if err != nil {
			parseErrors++
			continue
		}

		packet := gopacket.NewPacket(data, pcapReader.LinkType(), gopacket.Default)
		tuples, err := extractTuplesFromPacket(packet)
		if err != nil {
			parseErrors++
			continue
		}
		captured = append(captured, *tuples)
	}
	t.Logf("Parsed %d packets from pcap (%d unparseable)", len(captured), parseErrors)
	return captured
}

func tupleKey(pkt packetTuples) string {
	return fmt.Sprintf("%s|%s|%d|%d|%d",
		pkt.outerSrcIP, pkt.outerDstIP, pkt.outerL4Proto, pkt.outerSrcL4Port, pkt.outerDstL4Port)
}

func isReplayCandidate(pkt packetTuples) bool {
	if pkt.outerL4Proto != int(layers.IPProtocolUDP) {
		return false
	}
	srcIP, err := netip.ParseAddr(pkt.outerSrcIP)
	if err != nil || !srcIP.Is4() {
		return false
	}
	dstIP, err := netip.ParseAddr(pkt.outerDstIP)
	if err != nil || !dstIP.Is4() {
		return false
	}
	if !replayDstPrefix.Contains(dstIP) {
		return false
	}
	if pkt.outerSrcL4Port < 1 || pkt.outerSrcL4Port > 65535 {
		return false
	}
	if pkt.outerDstL4Port < 1 || pkt.outerDstL4Port > 65535 {
		return false
	}
	return true
}

func sanitizeReplayTuples(input []packetTuples) (clean []packetTuples, dropped, deduped int) {
	seen := make(map[string]struct{}, len(input))
	clean = make([]packetTuples, 0, len(input))
	for _, pkt := range input {
		if !isReplayCandidate(pkt) {
			dropped++
			continue
		}
		key := tupleKey(pkt)
		if _, ok := seen[key]; ok {
			deduped++
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, pkt)
	}
	return clean, dropped, deduped
}

func allSameString(vals []string) bool {
	if len(vals) < 2 {
		return true
	}
	first := vals[0]
	for _, v := range vals[1:] {
		if v != first {
			return false
		}
	}
	return true
}

func logGRIBITree(t *testing.T, sent int, p2, p3, p4, p5 uint64) {
	t.Helper()
	total := float64(sent)
	nhg101A := float64(nhg101WeightA)
	nhg101B := float64(nhg101WeightB)
	nhg2010A := float64(nhg2010WeightA)
	nhg2010B := float64(nhg2010WeightB)
	nhg3000A := float64(nhg3000WeightA)
	nhg3000B := float64(nhg3000WeightB)

	frac2010 := nhg101A / (nhg101A + nhg101B)
	frac3000 := nhg101B / (nhg101A + nhg101B)
	fracLAG1via2010 := frac2010 * nhg2010A / (nhg2010A + nhg2010B)
	fracLAG2via2010 := frac2010 * nhg2010B / (nhg2010A + nhg2010B)
	fracLAG2via3000A := frac3000 * nhg3000A / (nhg3000A + nhg3000B)
	fracLAG2via3000B := frac3000 * nhg3000B / (nhg3000A + nhg3000B)

	expLAG1 := fracLAG1via2010
	expLAG2 := fracLAG2via2010 + fracLAG2via3000A + fracLAG2via3000B
	expPort2 := expLAG1 / 2
	expPort3 := expLAG1 / 2
	expPort4 := expLAG2 / 2
	expPort5 := expLAG2 / 2

	ep2 := uint64(total * expPort2)
	ep3 := uint64(total * expPort3)
	ep4 := uint64(total * expPort4)
	ep5 := uint64(total * expPort5)

	actual := p2 + p3 + p4 + p5
	pctOf := func(v uint64) string {
		if actual == 0 {
			return "  -  "
		}
		return fmt.Sprintf("%5.1f%%", float64(v)*100/float64(actual))
	}

	t.Logf("\n"+
		"  gRIBI Forwarding Tree (sent %d packets)\n"+
		"  =========================================================\n"+
		"  %s -> NHG 101 (weight %d:%d)\n"+
		"  |\n"+
		"  +--[%d/%d]--> NHG 2010 (weight %d:%d)\n"+
		"  |             |\n"+
		"  |             +--[%d/%d]--> NH 1501 --> LAG1 (port2,port3)\n"+
		"  |             |             +-- port2  expected: %6d (%5.1f%%)  actual: %6d (%s)\n"+
		"  |             |             +-- port3  expected: %6d (%5.1f%%)  actual: %6d (%s)\n"+
		"  |             |\n"+
		"  |             +--[%d/%d]--> NH 1601 --> LAG2 (port4,port5)\n"+
		"  |\n"+
		"  +--[%d/%d]--> NHG 3000 (weight %d:%d)\n"+
		"                |\n"+
		"                +--[%d/%d]--> NH 1602 --> LAG2 (port4,port5)\n"+
		"                +--[%d/%d]--> NH 1603 --> LAG2 (port4,port5)\n"+
		"                              +-- port4  expected: %6d (%5.1f%%)  actual: %6d (%s)\n"+
		"                              +-- port5  expected: %6d (%5.1f%%)  actual: %6d (%s)\n"+
		"  =========================================================\n"+
		"  Totals:  LAG1 expected: %5.1f%%  LAG2 expected: %5.1f%%",
		sent,
		dstPfx, nhg101WeightA, nhg101WeightB,
		nhg101WeightA, nhg101WeightA+nhg101WeightB, nhg2010WeightA, nhg2010WeightB,
		nhg2010WeightA, nhg2010WeightA+nhg2010WeightB,
		ep2, expPort2*100, p2, pctOf(p2),
		ep3, expPort3*100, p3, pctOf(p3),
		nhg2010WeightB, nhg2010WeightA+nhg2010WeightB,
		nhg101WeightB, nhg101WeightA+nhg101WeightB, nhg3000WeightA, nhg3000WeightB,
		nhg3000WeightA, nhg3000WeightA+nhg3000WeightB,
		nhg3000WeightB, nhg3000WeightA+nhg3000WeightB,
		ep4, expPort4*100, p4, pctOf(p4),
		ep5, expPort5*100, p5, pctOf(p5),
		expLAG1*100, expLAG2*100,
	)
}

func extractTuplesFromPacket(packet gopacket.Packet) (*packetTuples, error) {
	tuples := &packetTuples{}

	if packet.Layer(layers.LayerTypeEthernet) == nil {
		return nil, fmt.Errorf("no Ethernet layer")
	}

	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return nil, fmt.Errorf("no IPv4 layer")
	}
	ip, _ := ipLayer.(*layers.IPv4)
	tuples.outerSrcIP = ip.SrcIP.String()
	tuples.outerDstIP = ip.DstIP.String()
	tuples.outerL4Proto = int(ip.Protocol)

	switch ip.Protocol {
	case layers.IPProtocolTCP:
		tcpLayer := packet.Layer(layers.LayerTypeTCP)
		if tcpLayer == nil {
			return nil, fmt.Errorf("TCP protocol indicated but no TCP layer")
		}
		tcp, _ := tcpLayer.(*layers.TCP)
		tuples.outerSrcL4Port = int(tcp.SrcPort)
		tuples.outerDstL4Port = int(tcp.DstPort)
	case layers.IPProtocolUDP:
		udpLayer := packet.Layer(layers.LayerTypeUDP)
		if udpLayer == nil {
			return nil, fmt.Errorf("UDP protocol indicated but no UDP layer")
		}
		udp, _ := udpLayer.(*layers.UDP)
		tuples.outerSrcL4Port = int(udp.SrcPort)
		tuples.outerDstL4Port = int(udp.DstPort)
	}

	return tuples, nil
}

// nextAggregates allocates n unique aggregate interface names from the DUT.
func nextAggregates(t *testing.T, dut *ondatra.DUTDevice, n int) []string {
	t.Helper()
	firstAgg := netutil.NextAggregateInterface(t, dut)
	start, err := strconv.Atoi(numRE.FindString(firstAgg))
	if err != nil {
		t.Fatalf("Cannot extract integer from %q: %v", firstAgg, err)
	}
	aggs := []string{firstAgg}
	for i := start + 1; len(aggs) < n; i++ {
		agg := numRE.ReplaceAllStringFunc(firstAgg, func(_ string) string {
			return strconv.Itoa(i)
		})
		_, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(agg).Name().State()).Val()
		if !present {
			aggs = append(aggs, agg)
		}
	}
	return aggs
}

// TestPolarization validates that ECMP hashing does not produce polarization
// when the DUT's hash configuration is perturbed between iterations.
func TestPolarization(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	aggIDs := nextAggregates(t, dut, 2)
	agg1ID := aggIDs[0]
	agg2ID := aggIDs[1]

	t.Cleanup(func() {
		flushGRIBIEntries(t, dut)
		gnmi.Delete(t, dut, gnmi.OC().Interface(agg1ID).Config())
		gnmi.Delete(t, dut, gnmi.OC().Interface(agg2ID).Config())
		gnmi.Delete(t, dut, gnmi.OC().Interface("Loopback0").Config())
	})

	t.Log("=== Phase 1/4: Configuring DUT interfaces and LAGs ===")
	t.Logf("LAG1=%s (port2,port3), LAG2=%s (port4,port5)", agg1ID, agg2ID)
	configureDUT(t, dut, agg1ID, agg2ID)

	t.Log("=== Phase 2/4: Configuring ATE ports and protocols ===")
	topo := configureATE(t, ate)

	t.Log("=== Phase 3/4: Programming gRIBI entries ===")
	createGRIBIEntries(t, dut)
	waitForDUTNextHops(t, dut, agg1ID, agg2ID)

	t.Log("=== Setup complete, starting polarization iterations ===")

	iterations := []struct {
		desc string
	}{
		{desc: "Baseline"},
		{desc: "Replay round 1"},
		{desc: "Replay round 2"},
		{desc: "Replay round 3"},
		{desc: "Replay round 4"},
	}

	packets := populatePackets(totalPackets)
	t.Logf("Generated %d unique packet tuples (batch size %d)", totalPackets, batchSize)

	for id, tc := range iterations {
		t.Run(fmt.Sprintf("%d_%s", id, tc.desc), func(t *testing.T) {
			totalPkts := len(packets)
			if totalPkts == 0 {
				t.Fatal("No packets available for replay")
			}
			iterationBatchSize := batchSize
			if id > 0 {
				iterationBatchSize = replayBatchSize
			}
			numBatches := (totalPkts + iterationBatchSize - 1) / iterationBatchSize
			t.Logf("--- Iteration %d/%d: %s (%d packets, %d batches of %d) ---",
				id+1, len(iterations), tc.desc, totalPkts, numBatches, iterationBatchSize)

			t.Log("Perturbing DUT hash configuration")
			perturbHashConfig(t, dut, id)

			var port2Total, port3Total, port4Total, port5Total uint64
			var allCaptured []packetTuples
			topo.Captures().Clear()
			topo.Captures().Add().SetName("port2").SetPortNames([]string{"port2"}).SetFormat(gosnappi.CaptureFormat.PCAP)

			for i := 0; i < totalPkts; i += iterationBatchSize {
				end := i + iterationBatchSize
				if end > totalPkts {
					end = totalPkts
				}
				batch := packets[i:end]
				batchNum := i/iterationBatchSize + 1
				flowName := fmt.Sprintf("lbFlow_%d", batchNum-1)

				t.Logf("Batch %d/%d: pushing %d packets as flow %s",
					batchNum, numBatches, len(batch), flowName)
				pushSingleFlow(t, ate, dut, topo, flowName, batch, agg1ID, agg2ID)
				startCapture(t, ate)
				runSingleFlow(t, ate, flowName, len(batch))

				batchCaptured := stopAndReadCapture(t, ate)
				batchCaptured, batchDropped, batchDeduped := sanitizeReplayTuples(batchCaptured)
				allCaptured = append(allCaptured, batchCaptured...)

				p2 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port("port2").Counters().InFrames().State())
				p3 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port("port3").Counters().InFrames().State())
				p4 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port("port4").Counters().InFrames().State())
				p5 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port("port5").Counters().InFrames().State())
				port2Total += p2
				port3Total += p3
				port4Total += p4
				port5Total += p5
				t.Logf("Batch %d: rx p2=%d p3=%d p4=%d p5=%d, captured=%d (dropped=%d deduped=%d) (total: p2=%d p3=%d p4=%d p5=%d, captured=%d)",
					batchNum, p2, p3, p4, p5, len(batchCaptured), batchDropped, batchDeduped,
					port2Total, port3Total, port4Total, port5Total, len(allCaptured))
			}

			totalPkts = len(packets)

			otgutils.LogPortMetrics(t, ate.OTG(), topo)

			port2Packets := port2Total
			nextPackets, iterDropped, iterDeduped := sanitizeReplayTuples(allCaptured)

			logGRIBITree(t, totalPkts, port2Total, port3Total, port4Total, port5Total)

			// Guard against masked forwarding loss before the polarization check.
			// That check inspects port2 alone, so a DUT delivering port2's expected
			// share while dropping traffic on ports3-5 must still fail here. Require
			// the frames received across all four LAG-member ports to account for
			// nearly all of the packets sent this iteration.
			totalReceived := port2Total + port3Total + port4Total + port5Total
			minReceived := uint64(float64(totalPkts) * (100 - lossTolerancePct) / 100)
			if totalReceived < minReceived {
				t.Fatalf("Traffic loss detected: ports2-5 received %d frames (p2=%d p3=%d p4=%d p5=%d) of %d sent (want >= %d, tolerance %.1f%%); DUT dropped forwarded traffic",
					totalReceived, port2Total, port3Total, port4Total, port5Total, totalPkts, minReceived, lossTolerancePct)
			}

			lag1Fraction := float64(nhg101WeightA) / float64(nhg101WeightA+nhg101WeightB) *
				float64(nhg2010WeightA) / float64(nhg2010WeightA+nhg2010WeightB)
			port2Fraction := lag1Fraction / 2.0
			expected := uint64(float64(totalPkts) * port2Fraction)
			acceptableDiff := uint64(totalPkts * tolerancePct / 100)
			difference := uint64(math.Abs(float64(port2Packets) - float64(expected)))

			t.Logf("=== Iteration %d results ===", id+1)
			t.Logf("  Sent:     %d packets total", totalPkts)
			t.Logf("  Expected: %d on port2 (%.1f%%)", expected, port2Fraction*100)
			t.Logf("  Actual:   %d on port2 (accumulated across batches)", port2Packets)
			if expected > 0 {
				pct := float64(port2Packets) * 100.0 / float64(expected)
				t.Logf("  Ratio:    %.1f%% of expected", pct)
			}
			t.Logf("  Delta:    %d (threshold: %d, tolerance: %d%%)",
				difference, acceptableDiff, tolerancePct)
			t.Logf("  Captured: %d packets for next iteration (dropped=%d deduped=%d)",
				len(nextPackets), iterDropped, iterDeduped)

			if difference > acceptableDiff {
				t.Fatalf("Polarization detected: port2 difference %d exceeds %d%% tolerance (%d packets)",
					difference, tolerancePct, acceptableDiff)
			}

			if len(nextPackets) == 0 {
				t.Fatal("Captured 0 packets from port2; cannot replay in the next iteration")
			}
			packets = nextPackets
		})
	}

	t.Log("=== Test complete ===")
}

// perturbHashConfig applies a vendor-specific configuration change to alter
// the DUT's hash behavior between test iterations. Each iteration must
// produce a distinct perturbation so that flows are re-distributed.
// Add new vendors here as support is validated.
func perturbHashConfig(t *testing.T, dut *ondatra.DUTDevice, iteration int) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.CISCO:
		perturbHashCiscoXR(t, dut, iteration)
	default:
		t.Fatalf("Hash perturbation not implemented for vendor %s; please add support in perturbHashConfig", dut.Vendor())
	}
}

// perturbHashCiscoXR changes the Loopback0 IP address. IOS-XR feeds the
// loopback IP into the ECMP hash computation, so changing it alters which
// flows land on which paths.
func perturbHashCiscoXR(t *testing.T, dut *ondatra.DUTDevice, iteration int) {
	t.Helper()
	ip := fmt.Sprintf("10.%d.%d.%d", iteration+10, iteration+10, iteration+10)
	t.Logf("Cisco IOS-XR: setting Loopback0 IP to %s", ip)

	d := gnmi.OC()
	lo0 := &oc.Interface{
		Name:    ygot.String("Loopback0"),
		Enabled: ygot.Bool(true),
		Type:    oc.IETFInterfaces_InterfaceType_softwareLoopback,
	}
	s := lo0.GetOrCreateSubinterface(0)
	s.Enabled = ygot.Bool(true)
	a := s.GetOrCreateIpv4().GetOrCreateAddress(ip)
	a.PrefixLength = ygot.Uint8(32)

	gnmi.Replace(t, dut, d.Interface(lo0.GetName()).Config(), lo0)
}

func flushGRIBIEntries(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Flushing all gRIBI entries")
	gribic := dut.RawAPIs().GRIBI(t)
	c := fluent.NewClient()

	conn := c.Connection().
		WithStub(gribic).
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1, 0)
	if !deviations.GRIBIRIBAckOnly(dut) {
		conn.WithFIBACK()
	}
	conn.WithPersistence()

	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}

	gribi.BecomeLeader(t, c)
	if err := gribi.FlushAll(c); err != nil {
		t.Errorf("Cannot flush: %v", err)
	}
}

func createGRIBIEntries(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Programming gRIBI next-hops, next-hop-groups, and IPv4 entries")
	gribic := dut.RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	defaultNI := deviations.DefaultNetworkInstance(dut)

	entries := []fluent.GRIBIEntry{
		fluent.NextHopEntry().WithNetworkInstance(defaultNI).
			WithIndex(1501).WithIPAddress(ateLAG1.nhs[0].IPv4),
		fluent.NextHopEntry().WithNetworkInstance(defaultNI).
			WithIndex(1601).WithIPAddress(ateLAG2.nhs[0].IPv4),
		fluent.NextHopEntry().WithNetworkInstance(defaultNI).
			WithIndex(1602).WithIPAddress(ateLAG2.nhs[1].IPv4),
		fluent.NextHopEntry().WithNetworkInstance(defaultNI).
			WithIndex(1603).WithIPAddress(ateLAG2.nhs[2].IPv4),

		fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
			WithID(2010).AddNextHop(1501, nhg2010WeightA).AddNextHop(1601, nhg2010WeightB),
		fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
			WithID(3000).AddNextHop(1602, nhg3000WeightA).AddNextHop(1603, nhg3000WeightB),

		fluent.IPv4Entry().WithNetworkInstance(defaultNI).WithPrefix("203.0.113.1/32").
			WithNextHopGroup(2010).WithNextHopGroupNetworkInstance(defaultNI),
		fluent.IPv4Entry().WithNetworkInstance(defaultNI).WithPrefix("203.0.113.2/32").
			WithNextHopGroup(3000).WithNextHopGroupNetworkInstance(defaultNI),

		fluent.NextHopEntry().WithNetworkInstance(defaultNI).
			WithIndex(1011).WithIPAddress("203.0.113.1"),
		fluent.NextHopEntry().WithNetworkInstance(defaultNI).
			WithIndex(1012).WithIPAddress("203.0.113.2"),

		fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
			WithID(101).AddNextHop(1011, nhg101WeightA).AddNextHop(1012, nhg101WeightB),

		fluent.IPv4Entry().WithNetworkInstance(defaultNI).WithPrefix(dstPfx).
			WithNextHopGroup(101).WithNextHopGroupNetworkInstance(defaultNI),
	}

	conn := c.Connection().
		WithStub(gribic).
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1, 0)
	if !deviations.GRIBIRIBAckOnly(dut) {
		conn.WithFIBACK()
	}
	conn.WithPersistence()

	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}

	gribi.BecomeLeader(t, c)
	c.Modify().AddEntry(t, entries...)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}

	wantOperationResults := []*client.OpResult{
		fluent.OperationResult().WithNextHopOperation(1501).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithNextHopOperation(1601).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithNextHopOperation(1602).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithNextHopOperation(1603).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithNextHopOperation(1011).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithNextHopOperation(1012).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithNextHopGroupOperation(2010).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithNextHopGroupOperation(3000).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithNextHopGroupOperation(101).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithIPv4Operation("203.0.113.1/32").
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithIPv4Operation("203.0.113.2/32").
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
		fluent.OperationResult().WithIPv4Operation(dstPfx).
			WithProgrammingResult(fluent.InstalledInFIB).WithOperationType(constants.Add).AsResult(),
	}

	for _, wantResult := range wantOperationResults {
		chk.HasResult(t, c.Results(t), wantResult, chk.IgnoreOperationID())
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, agg1ID, agg2ID string) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	t.Logf("Configuring DUT port1 (%s) with IP %s/%d", p1.Name(), dutPort1.IPv4, dutPort1.IPv4Len)
	i1 := dutPort1.NewOCInterface(p1.Name(), dut)
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), i1)

	if deviations.AggregateAtomicUpdate(dut) {
		setupAggregatesAtomically(t, dut, agg1ID, agg2ID)
	}

	t.Logf("Configuring LAG members: port2,port3 -> %s; port4,port5 -> %s", agg1ID, agg2ID)
	configureLAGMember(t, dut, "port2", agg1ID)
	configureLAGMember(t, dut, "port3", agg1ID)
	configureLAGMember(t, dut, "port4", agg2ID)
	configureLAGMember(t, dut, "port5", agg2ID)

	t.Logf("Configuring LAG interfaces: %s (%s/%d), %s (%s/%d)", agg1ID, dutLAG1.IPv4, plen24, agg2ID, dutLAG2.IPv4, plen24)
	lag1 := &oc.Interface{
		Name:    ygot.String(agg1ID),
		Enabled: ygot.Bool(true),
		Type:    ieee8023adLag,
	}
	lag1.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	s1 := lag1.GetOrCreateSubinterface(0)
	s1.Index = ygot.Uint32(0)
	a1v4 := s1.GetOrCreateIpv4().GetOrCreateAddress(dutLAG1.IPv4)
	a1v4.Ip = ygot.String(dutLAG1.IPv4)
	a1v4.PrefixLength = ygot.Uint8(plen24)

	lag2 := &oc.Interface{
		Name:    ygot.String(agg2ID),
		Enabled: ygot.Bool(true),
		Type:    ieee8023adLag,
	}
	lag2.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	s2 := lag2.GetOrCreateSubinterface(0)
	s2.Index = ygot.Uint32(0)
	a2v4 := s2.GetOrCreateIpv4().GetOrCreateAddress(dutLAG2.IPv4)
	a2v4.Ip = ygot.String(dutLAG2.IPv4)
	a2v4.PrefixLength = ygot.Uint8(plen24)

	gnmi.Update(t, dut, d.Interface(lag1.GetName()).Config(), lag1)
	gnmi.Update(t, dut, d.Interface(lag2.GetName()).Config(), lag2)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, agg1ID, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, agg2ID, deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		for _, portID := range []string{"port1", "port2", "port3", "port4", "port5"} {
			fptest.SetPortSpeed(t, dut.Port(t, portID))
		}
	}
}

func setupAggregatesAtomically(t *testing.T, dut *ondatra.DUTDevice, agg1ID, agg2ID string) {
	t.Helper()
	d := &oc.Root{}

	agg1 := d.GetOrCreateInterface(agg1ID)
	agg1.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	agg1.Type = ieee8023adLag

	agg2 := d.GetOrCreateInterface(agg2ID)
	agg2.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	agg2.Type = ieee8023adLag

	for _, portID := range []string{"port2", "port3"} {
		port := dut.Port(t, portID)
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(agg1ID)
		i.Type = ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}
	for _, portID := range []string{"port4", "port5"} {
		port := dut.Port(t, portID)
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(agg2ID)
		i.Type = ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}

	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

func configureLAGMember(t *testing.T, dut *ondatra.DUTDevice, portID, aggID string) {
	t.Helper()
	d := gnmi.OC()
	port := dut.Port(t, portID)

	i := &oc.Interface{
		Name:    ygot.String(port.Name()),
		Type:    ethernetCsmacd,
		Enabled: ygot.Bool(true),
	}
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)

	gnmi.Replace(t, dut, d.Interface(port.Name()).Config(), i)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	t.Logf("Configuring ATE port1 (%s); LAG1 on port2,port3; LAG2 on port4,port5", p1.Name())
	atePort1.AddToOTG(top, p1, &dutPort1)
	for _, l := range []ateLAG{ateLAG1, ateLAG2} {
		addATELAG(t, ate, top, l)
	}

	t.Log("Pushing ATE config and starting protocols")
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	return top
}

// addATELAG adds a static LAG and one device per next-hop on that LAG.
func addATELAG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, l ateLAG) {
	t.Helper()
	lag := top.Lags().Add().SetName(l.name)
	lag.Protocol().Static().SetLagId(l.id)
	for i, portID := range l.ports {
		p := ate.Port(t, portID)
		top.Ports().Add().SetName(p.ID())
		lag.Ports().Add().SetPortName(p.ID()).Ethernet().
			SetMac(l.macs[i]).
			SetName(fmt.Sprintf("%s.member%d", l.name, i))
	}
	for _, nh := range l.nhs {
		dev := top.Devices().Add().SetName(nh.Name)
		eth := dev.Ethernets().Add().SetName(nh.Name + ".Eth").SetMac(nh.MAC)
		eth.Connection().SetLagName(l.name)
		eth.Ipv4Addresses().Add().SetName(nh.Name + ".IPv4").
			SetAddress(nh.IPv4).
			SetGateway(l.gw.IPv4).
			SetPrefix(uint32(nh.IPv4Len))
	}
}

// waitForDUTNextHops waits until the DUT has resolved every gRIBI next-hop
// with ARP. Traffic sent before that resolution is dropped.
func waitForDUTNextHops(t *testing.T, dut *ondatra.DUTDevice, agg1ID, agg2ID string) {
	t.Helper()
	for _, l := range []struct {
		intf string
		lag  ateLAG
	}{
		{agg1ID, ateLAG1},
		{agg2ID, ateLAG2},
	} {
		for _, nh := range l.lag.nhs {
			_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(l.intf).Subinterface(0).Ipv4().Neighbor(nh.IPv4).LinkLayerAddress().State(), 2*time.Minute, func(val *ygnmi.Value[string]) bool {
				return val.IsPresent()
			}).Await(t)
			if !ok {
				t.Fatalf("DUT did not resolve ARP for %s on %s", nh.IPv4, l.intf)
			}
		}
	}
}

// pushSingleFlow replaces the ATE flow and pushes the config. A push can
// flap the ATE links and clear the DUT's ARP entries, so both sides are
// re-checked before traffic starts.
func pushSingleFlow(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, topo gosnappi.Config, name string, batch []packetTuples, agg1ID, agg2ID string) {
	t.Helper()
	topo.Flows().Clear()
	createStaticFlow(t, name, ate, topo, batch)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	waitForDUTNextHops(t, dut, agg1ID, agg2ID)
}

func createStaticFlow(t *testing.T, name string, ate *ondatra.ATEDevice, ateTop gosnappi.Config, flows []packetTuples) string {
	t.Helper()
	// Port endpoints count frames on every LAG member. Device endpoints would
	// drop frames whose destination MAC is the other member's next-hop.
	flowipv4 := ateTop.Flows().Add().SetName(name)
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Port().
		SetTxName(atePort1.Name).
		SetRxNames(append(append([]string{}, ateLAG1.ports...), ateLAG2.ports...))

	dstMAC := gnmi.Get(t, ate.OTG(), gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4Neighbor(dutPort1.IPv4).LinkLayerAddress().State())

	var srcIPs, dstIPs []string
	var srcPorts, dstPorts []uint32

	for _, tuples := range flows {
		srcIPs = append(srcIPs, tuples.outerSrcIP)
		dstIPs = append(dstIPs, tuples.outerDstIP)
		srcPorts = append(srcPorts, uint32(tuples.outerSrcL4Port))
		dstPorts = append(dstPorts, uint32(tuples.outerDstL4Port))
	}

	eth := flowipv4.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort1.MAC)
	eth.Dst().SetValue(dstMAC)

	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValues(srcIPs)
	if allSameString(dstIPs) {
		v4.Dst().SetValue(dstIPs[0])
	} else {
		v4.Dst().SetValues(dstIPs)
	}

	udp := flowipv4.Packet().Add().Udp()
	udp.DstPort().SetValues(dstPorts)
	udp.SrcPort().SetValues(srcPorts)

	flowipv4.Size().SetFixed(128)
	flowipv4.Rate().SetPps(trafficPps)
	flowipv4.Duration().FixedPackets().SetPackets(uint32(len(srcIPs)))

	t.Logf("Configured %d packets for flow %s", len(srcIPs), name)
	return name
}

func runSingleFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string, batchLen int) {
	t.Helper()
	ate.OTG().StartTraffic(t)
	defer ate.OTG().StopTraffic(t)

	flowTimeout := time.Duration(batchLen/trafficPps+120) * time.Second
	t.Logf("Waiting for flow %s (%d pkts, timeout %v)", flowName, batchLen, flowTimeout)
	txPackets, rxPackets := otgutils.GetFlowStats(t, ate.OTG(), flowName, flowTimeout)

	if txPackets == 0 {
		t.Fatalf("Flow %s: TxPkts == 0, want > 0", flowName)
	}
	lossPct := (float64(txPackets) - float64(rxPackets)) * 100 / float64(txPackets)
	t.Logf("Flow %s complete: tx=%d rx=%d loss=%.2f%%", flowName, txPackets, rxPackets, lossPct)

	// Fail on traffic loss rather than only logging it. The flow's Rx endpoints
	// span all of ports2-5, so a shortfall here means the DUT dropped forwarded
	// traffic on one of those ports. Without this gate, drops on ports3-5 would
	// be masked by the port2-only polarization assertion, and the (still valid)
	// port2 packets would be replayed into the next iteration, hiding a real
	// forwarding fault.
	if lossPct > lossTolerancePct {
		t.Fatalf("Flow %s: traffic loss %.2f%% (tx=%d rx=%d) exceeds tolerance %.1f%%; DUT dropped forwarded traffic",
			flowName, lossPct, txPackets, rxPackets, lossTolerancePct)
	}
}

func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}
