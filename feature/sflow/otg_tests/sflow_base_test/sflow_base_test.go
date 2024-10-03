// Copyright 2023 Google LLC
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

package sflow_base_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen   = 30
	plenIPv4        = 30
	plenIPv6        = 126
	lossTolerance   = 1
	mgmtVRF         = "mvrf1"
	sampleTolerance = 0.8
	samplingRate    = 1000000
)

var (
	staticRouteV4 = &cfgplugins.StaticRouteCfg{
		NetworkInstance: mgmtVRF,
		Prefix:          "192.0.2.128/30",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString("192.0.2.6"),
		},
	}
	staticRouteV6 = &cfgplugins.StaticRouteCfg{
		NetworkInstance: mgmtVRF,
		Prefix:          "2001:db8::128/126",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"1": oc.UnionString("2001:db8::6"),
		},
	}
	dutSrc = &attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::1",
		IPv6Len: plenIPv6,
	}
	dutDst = &attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::5",
		IPv6Len: plenIPv6,
	}
	ateSrc = &attrs.Attributes{
		Name:    "ateSrc",
		Desc:    "ATE to DUT source",
		IPv4:    "192.0.2.2",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::2",
		IPv6Len: plenIPv6,
		MAC:     "02:00:01:01:01:01",
	}
	ateDst = &attrs.Attributes{
		Name:    "ateDst",
		Desc:    "ATE to DUT destination",
		IPv4:    "192.0.2.6",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::6",
		IPv6Len: plenIPv6,
		MAC:     "02:00:02:01:01:01",
	}
	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	flowConfigs = []flowConfig{
		{
			name:          "flowS",
			packetsToSend: 1000000,
			ppsRate:       100000,
			frameSize:     64,
		},
		{
			name:          "flowM",
			packetsToSend: 1000000,
			ppsRate:       100000,
			frameSize:     512,
		},
		{
			name:          "flowL",
			packetsToSend: 1000000,
			ppsRate:       100000,
			frameSize:     1500,
		},
	}
)

type flowConfig struct {
	name          string
	packetsToSend uint32
	ppsRate       uint64
	frameSize     uint32
}

type IPType string

const (
	IPv4 = "IPv4"
	IPv6 = "IPv6"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUTBaseline configures port1 and port2 on the DUT.
func configureDUTBaseline(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutSrc.NewOCInterface(p1.Name(), dut))

	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutDst.NewOCInterface(p2.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// TestSFlowTraffic configures a DUT for sFlow client and collector endpoint and uses ATE to send
// traffic which the DUT should sample and send sFlow packets to a collector. ATE captures the
// sflow packets which are decoded by the test to verify they are valid sflow packets.
func TestSFlowTraffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	ate := ondatra.ATE(t, "ate")

	loopbackIntfName := netutil.LoopbackInterface(t, dut, 1)

	// Configure DUT
	if !deviations.InterfaceConfigVRFBeforeAddress(dut) {
		configureDUTBaseline(t, dut)
		configureLoopbackOnDUT(t, dut)
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	addInterfacesToVRF(t, dut, mgmtVRF, []string{p1.Name(), p2.Name(), loopbackIntfName})

	// For interface configuration, Arista prefers config Vrf first then the IP address
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		configureDUTBaseline(t, dut)
		configureLoopbackOnDUT(t, dut)
	}

	config := configureATE(t, ate)
	otgutils.WaitForARP(t, ate.OTG(), config, "IPv4")

	srBatch := &gnmi.SetBatch{}
	cfgplugins.NewStaticRouteCfg(srBatch, staticRouteV4, dut)
	cfgplugins.NewStaticRouteCfg(srBatch, staticRouteV6, dut)
	srBatch.Set(t, dut)

	t.Run("SFLOW-1.1_ReplaceDUTConfigSFlow", func(t *testing.T) {
		sfBatch := &gnmi.SetBatch{}
		cfgplugins.NewSFlowGlobalCfg(t, sfBatch, nil, dut, mgmtVRF, loopbackIntfName)
		sfBatch.Set(t, dut)

		gotSamplingConfig := gnmi.Get(t, dut, gnmi.OC().Sampling().Sflow().Config())
		json, err := ygot.EmitJSON(gotSamplingConfig, &ygot.EmitJSONConfig{
			Format: ygot.RFC7951,
			Indent: "  ",
			RFC7951Config: &ygot.RFC7951JSONConfig{
				AppendModuleName: true,
			},
		})
		if err != nil {
			t.Errorf("Error decoding sampling config: %v", err)
		}
		t.Logf("Got sampling config: %v", json)
	})

	/* TODO: implement this when a suitable ygot.diffBatch function exists
		// Validate DUT sampling config matches what we set it to
		diff, err := ygot.Diff(gotSamplingConfig, sfBatch)
		if err != nil {
			t.Errorf("Error attempting to compare sflow config: %v", err.Error())
		}
		if diff.String() != "" {
			t.Errorf("Want empty string, got: %v", helpers.GNMINotifString(diff))
		}
	})
	*/

	t.Run("SFLOW-1.2_TestFlowFixed", func(t *testing.T) {
		t.Run("SFLOW-1.2.1_IPv4", func(t *testing.T) {
			enableCapture(t, ate, config, IPv4)
			testFlowFixed(t, ate, config, IPv4)
		})
		t.Run("SFLOW-1.2.2_IPv6", func(t *testing.T) {
			enableCapture(t, ate, config, IPv6)
			testFlowFixed(t, ate, config, IPv6)
		})
	})

	defer func() {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(mgmtVRF).Config())
	}()
}

func testFlowFixed(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, ip IPType) {
	for _, fc := range flowConfigs {
		flowName := string(ip) + fc.name
		t.Run(flowName, func(t *testing.T) {
			createFlow(t, ate, config, fc, ip)

			cs := startCapture(t, ate, config)

			sleepTime := time.Duration(fc.packetsToSend/uint32(fc.ppsRate)) + 5
			ate.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			ate.OTG().StopTraffic(t)

			stopCapture(t, ate, cs)

			otgutils.LogFlowMetrics(t, ate.OTG(), config)
			otgutils.LogPortMetrics(t, ate.OTG(), config)

			loss := otgutils.GetFlowLossPct(t, ate.OTG(), flowName, 10*time.Second)
			if loss > lossTolerance {
				t.Errorf("Loss percent for IPv4 Traffic: got: %f, want %f", loss, float64(lossTolerance))
			}

			processCapture(t, ate, config, ip, fc)
		})
	}
}

func startCapture(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) gosnappi.ControlState {
	t.Helper()

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	ate.OTG().SetControlState(t, cs)

	return cs
}

func stopCapture(t *testing.T, ate *ondatra.ATEDevice, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	ate.OTG().SetControlState(t, cs)
}

func processCapture(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, ip IPType, fc flowConfig) {
	bytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(config.Ports().Items()[1].Name()))
	time.Sleep(30 * time.Second)
	pcapFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	pcapFile.Close()
	validatePackets(t, pcapFile.Name(), ip, fc)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	config := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	ateSrc.AddToOTG(config, p1, dutSrc)
	ateDst.AddToOTG(config, p2, dutDst)

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)

	return config
}

func enableCapture(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, ip IPType) {
	t.Helper()

	config.Captures().Clear()
	// enable packet capture on this port
	cap := config.Captures().Add().SetName("sFlowpacketCapture").
		SetPortNames([]string{config.Ports().Items()[1].Name()}).
		SetFormat(gosnappi.CaptureFormat.PCAP)
	filter := cap.Filters().Add()
	if ip == IPv4 {
		// filter on hex value of IPv4 - 203.0.113.1
		filter.Ipv4().Src().SetValue("cb007101")
	} else {
		// filter on hex value of IPv6 - 2001:db8::203:0:113:1
		filter.Ipv6().Src().SetValue("20010db8000000000203000001130001")
	}

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)

	pb, _ := config.Marshal().ToProto()
	t.Log(pb.GetCaptures())
}

func addInterfacesToVRF(t *testing.T, dut *ondatra.DUTDevice, vrfname string, intfNames []string) {
	root := &oc.Root{}
	mgmtNI := root.GetOrCreateNetworkInstance(vrfname)
	mgmtNI.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	for _, intfName := range intfNames {
		vi := mgmtNI.GetOrCreateInterface(intfName)
		vi.Interface = ygot.String(intfName)
		vi.Subinterface = ygot.Uint32(0)
	}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(mgmtVRF).Config(), mgmtNI)
	t.Logf("Added interface %v to VRF %s", intfNames, vrfname)
}

func configureLoopbackOnDUT(t *testing.T, dut *ondatra.DUTDevice) {
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 1)
	loop := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
	loop.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	loop.Description = ygot.String(fmt.Sprintf("Port %s", loopbackIntfName))
	gnmi.Update(t, dut, gnmi.OC().Interface(loopbackIntfName).Config(), loop)
	t.Logf("Got DUT IPv4, IPv6 loopback address: %v, %v", dutlo0Attrs.IPv4, dutlo0Attrs.IPv6)
}

func createFlow(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, fc flowConfig, ip IPType) {
	config.Flows().Clear()

	t.Log("Configuring traffic flow")
	flow := config.Flows().Add().SetName(string(ip) + fc.name)
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(fc.frameSize)
	flow.Rate().SetPps(fc.ppsRate)
	flow.Duration().FixedPackets().SetPackets(fc.packetsToSend)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValues([]string{ateSrc.MAC})

	switch ip {
	case IPv4:
		flow.TxRx().Device().
			SetTxNames([]string{"ateSrc.IPv4"}).
			SetRxNames([]string{"ateDst.IPv4"})
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
	case IPv6:
		flow.TxRx().Device().
			SetTxNames([]string{"ateSrc.IPv6"}).
			SetRxNames([]string{"ateDst.IPv6"})
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(ateSrc.IPv6)
		v6.Dst().SetValue(ateDst.IPv6)
	}

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
}

func validatePackets(t *testing.T, filename string, ip IPType, fc flowConfig) {
	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	loopbackIP := net.ParseIP(dutlo0Attrs.IPv4)
	if ip == IPv6 {
		loopbackIP = net.ParseIP(dutlo0Attrs.IPv6)
	}
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	found := false
	packetCount := 0
	sflowSamples := uint32(0)
	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ipv4, _ := ipLayer.(*layers.IPv4)
			if ipv4.SrcIP.Equal(loopbackIP) {
				t.Logf("tos %d, payload %d, content %d, length %d", ipv4.TOS, len(ipv4.Payload), len(ipv4.Contents), ipv4.Length)
				if ipv4.TOS == 32 {
					found = true
					break
				}
			}
		} else if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
			ipv6, _ := ipLayer.(*layers.IPv6)
			if ipv6.SrcIP.Equal(loopbackIP) {
				t.Logf("tos %d, payload %d, content %d, length %d", ipv6.TrafficClass, len(ipv6.Payload), len(ipv6.Contents), ipv6.Length)
				if ipv6.TrafficClass == 32 {
					found = true
					break
				}
			}
		}

	}
	if !found {
		t.Error("sflow packets not found")
	}

	for packet := range packetSource.Packets() {
		if sflowLayer := packet.Layer(layers.LayerTypeSFlow); sflowLayer != nil {
			sflow := sflowLayer.(*layers.SFlowDatagram)
			packetCount++
			sflowSamples += sflow.SampleCount
			t.Logf("SFlow Packet count: %v - SampleCount: %v", packetCount, sflowSamples)
		}
	}

	expectedSampleCount := float64(fc.packetsToSend / samplingRate)
	minAllowedSamples := expectedSampleCount * sampleTolerance
	if sflowSamples < uint32(minAllowedSamples) {
		t.Errorf("SFlow sample count %v, want > %v", sflowSamples, expectedSampleCount)
	}
}
