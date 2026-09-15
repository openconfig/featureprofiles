// Copyright 2026 Google LLC
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

package sflow_egress_test

import (
	"fmt"
	"net"
	"os"
	"slices"
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
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4        = 30
	plenIPv6        = 126
	lossTolerance   = 1
	mgmtVRF         = "mvrf1"
	sampleTolerance = 0.8
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
	sflowCfgv4 = &cfgplugins.SFlowGlobalParams{
		Ni:              mgmtVRF,
		IntfName:        "port1",
		SrcAddrV4:       "203.0.113.1",
		IP:              "IPv4",
		MinSamplingRate: 100000,
		Egress:          true,
	}
	sflowCfgv6 = &cfgplugins.SFlowGlobalParams{
		Ni:              mgmtVRF,
		IntfName:        "port1",
		SrcAddrV6:       "2001:db8::203:0:113:1",
		IP:              "IPv6",
		MinSamplingRate: 100000,
		Egress:          true,
	}
	sflowCfgv4KNE = &cfgplugins.SFlowGlobalParams{
		Ni:              mgmtVRF,
		IntfName:        "port1",
		SrcAddrV4:       "192.0.2.1",
		IP:              "IPv4",
		MinSamplingRate: 10,
		Egress:          true,
	}
	sflowCfgv6KNE = &cfgplugins.SFlowGlobalParams{
		Ni:              mgmtVRF,
		IntfName:        "port1",
		SrcAddrV6:       "2001:db8::1",
		IP:              "IPv6",
		MinSamplingRate: 10,
		Egress:          true,
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
	loopbackSubIntfNum = 1
	dutlo0Attrs        = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}
	kneDeviceModelList = []string{"ncptx", "ceos", "srlinux", "xrd"}
	flowConfigs        = []flowConfig{
		{
			name:            "flowS",
			packetsToSend:   10000000,
			ppsRate:         300000,
			frameSize:       64,
			minSamplingRate: 100000,
		},
		{
			name:            "flowM",
			packetsToSend:   10000000,
			ppsRate:         300000,
			frameSize:       512,
			minSamplingRate: 100000,
		},
		{
			name:            "flowL",
			packetsToSend:   10000000,
			ppsRate:         300000,
			frameSize:       1500,
			minSamplingRate: 100000,
		},
	}
	flowConfigsKNE = []flowConfig{
		{
			name:            "flowS",
			packetsToSend:   100,
			ppsRate:         10,
			frameSize:       64,
			minSamplingRate: 10,
		},
		{
			name:            "flowM",
			packetsToSend:   100,
			ppsRate:         10,
			frameSize:       512,
			minSamplingRate: 10,
		},
		{
			name:            "flowL",
			packetsToSend:   100,
			ppsRate:         10,
			frameSize:       1500,
			minSamplingRate: 10,
		},
	}
)

type flowConfig struct {
	name            string
	packetsToSend   uint32
	ppsRate         uint64
	frameSize       uint32
	minSamplingRate uint32
}

type IPType string

const (
	IPv4 IPType = "IPv4"
	IPv6 IPType = "IPv6"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUTBaseline configures port1 and port2 on the DUT.
func configureDUTBaseline(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	dutPortAttrs := map[*ondatra.Port]*attrs.Attributes{
		p1: dutSrc,
		p2: dutDst,
	}
	for dutPort, dutPortAttr := range dutPortAttrs {
		dutInt := dutPortAttr.NewOCInterface(dutPort.Name(), dut)
		if deviations.FrBreakoutFix(dut) && dutPort.PMD() == ondatra.PMD100GBASEFR {
			dutInt.GetOrCreateEthernet().SetPortSpeed(oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB)
			dutInt.GetOrCreateEthernet().SetDuplexMode(oc.Ethernet_DuplexMode_FULL)
			dutInt.GetOrCreateEthernet().SetAutoNegotiate(false)
		}
		gnmi.Replace(t, dut, d.Interface(dutPort.Name()).Config(), dutInt)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// TestSFlowEgressTraffic configures a DUT for sFlow egress sampling on port2 and collector endpoint,
// uses ATE to send traffic through port1 -> port2, captures the resulting sFlow datagrams on ATE port2,
// verifies their egress interface metadata and sampling counts, and verifies teardown when disabled.
func TestSFlowEgressTraffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	ate := ondatra.ATE(t, "ate")
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		loopbackSubIntfNum = 0
	}
	loopbackIntfName := netutil.LoopbackInterface(t, dut, loopbackSubIntfNum)
	if !deviations.InterfaceConfigVRFBeforeAddress(dut) {
		configureDUTBaseline(t, dut)
		configureLoopbackOnDUT(t, dut)
	}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	addInterfacesToVRF(t, dut, mgmtVRF, []string{p1.Name(), p2.Name(), loopbackIntfName})
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
	t.Cleanup(func() {
		gnmi.Delete(t, dut, gnmi.OC().Sampling().Sflow().Config())
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(mgmtVRF).Config())
		gnmi.Delete(t, dut, gnmi.OC().Interface(loopbackIntfName).Config())
	})

	var port2IfIndex uint32
	if val, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(p2.Name()).Ifindex().State(), 10*time.Second, func(val *ygnmi.Value[uint32]) bool {
		v, present := val.Val()
		return present && v != 0
	}).Await(t); ok {
		port2IfIndex, _ = val.Val()
		t.Logf("DUT port2 (%s) SNMP ifIndex: %d", p2.Name(), port2IfIndex)
	}

	t.Run("SFLOW-2.2_TestFlowEgress", func(t *testing.T) {
		t.Run("SFLOW-2.2.1_IPv4", func(t *testing.T) {
			configSflowEgress(t, dut, loopbackIntfName, IPv4)
			enableCapture(t, ate, config, IPv4)
			testFlowEgress(t, ate, config, IPv4, dut, port2IfIndex)
		})
		t.Run("SFLOW-2.2.2_IPv6", func(t *testing.T) {
			configSflowEgress(t, dut, loopbackIntfName, IPv6)
			enableCapture(t, ate, config, IPv6)
			testFlowEgress(t, ate, config, IPv6, dut, port2IfIndex)
		})
	})

	t.Run("SFLOW-2.3_DisableEgressSamplingAndVerifyTeardown", func(t *testing.T) {
		cfgplugins.DisableSFlowEgressCfg(t, dut, p2.Name())
		gnmi.Watch(t, dut, gnmi.OC().Sampling().Sflow().Interface(p2.Name()).Enabled().State(), 30*time.Second, func(val *ygnmi.Value[bool]) bool {
			v, present := val.Val()
			return !present || !v
		}).Await(t)
		enableCapture(t, ate, config, IPv4)
		fc := flowConfigs[0]
		if slices.Contains(kneDeviceModelList, dut.Model()) {
			fc = flowConfigsKNE[0]
		}
		createFlow(t, ate, config, fc, IPv4)
		cs := startCapture(t, ate, config)
		if fc.ppsRate == 0 {
			t.Fatal("ppsRate cannot be zero")
		}
		sleepTime := time.Duration(fc.packetsToSend/uint32(fc.ppsRate)) + 5
		ate.OTG().StartTraffic(t)
		time.Sleep(sleepTime * time.Second)
		ate.OTG().StopTraffic(t)
		stopCapture(t, ate, cs)
		verifyNoSFlowPackets(t, ate, config, IPv4)
	})
}

func configSflowEgress(t *testing.T, dut *ondatra.DUTDevice, loopbackIntfName string, ip IPType) {
	t.Run("SFLOW-2.1_ReplaceDUTConfigSFlowEgress", func(t *testing.T) {
		sfBatch := &gnmi.SetBatch{}
		var p *cfgplugins.SFlowGlobalParams
		var collectorAddr, srcAddr string
		collectorPort := uint16(6343)
		switch ip {
		case IPv4:
			if slices.Contains(kneDeviceModelList, dut.Model()) {
				sflowCfgv4 = sflowCfgv4KNE
			}
			sflowCfgv4.IntfName = loopbackIntfName
			p = sflowCfgv4
			collectorAddr = "192.0.2.129"
			srcAddr = p.SrcAddrV4
		case IPv6:
			if slices.Contains(kneDeviceModelList, dut.Model()) {
				sflowCfgv6 = sflowCfgv6KNE
			}
			sflowCfgv6.IntfName = loopbackIntfName
			p = sflowCfgv6
			collectorAddr = "2001:db8::129"
			srcAddr = p.SrcAddrV6
		}

		p2Name := dut.Port(t, "port2").Name()
		sfCfg := cfgplugins.NewSFlowGlobalCfg(t, sfBatch, nil, dut, p)
		sfCfg.SetEnabled(true)
		sfCfg.SetSampleSize(256)
		colCfg := sfCfg.GetOrCreateCollector(collectorAddr, collectorPort)
		colCfg.SetAddress(collectorAddr)
		colCfg.SetPort(collectorPort)
		colCfg.SetNetworkInstance(p.Ni)
		if !deviations.SflowSourceAddressUpdateUnsupported(dut) {
			colCfg.SetSourceAddress(srcAddr)
		}
		intfCfg := sfCfg.GetOrCreateInterface(p2Name)
		intfCfg.SetName(p2Name)
		intfCfg.SetEnabled(true)
		if !deviations.SflowEgressSamplingRateUnsupported(dut) {
			intfCfg.SetEgressSamplingRate(p.MinSamplingRate)
		}
		gnmi.BatchReplace(sfBatch, gnmi.OC().Sampling().Sflow().Config(), sfCfg)
		sfBatch.Set(t, dut)

		// Verify /sampling/sflow/state/enabled and /sampling/sflow/state/sample-size
		gnmi.Watch(t, dut, gnmi.OC().Sampling().Sflow().Enabled().State(), 30*time.Second, func(val *ygnmi.Value[bool]) bool {
			v, present := val.Val()
			return present && v
		}).Await(t)
		gnmi.Watch(t, dut, gnmi.OC().Sampling().Sflow().SampleSize().State(), 30*time.Second, func(val *ygnmi.Value[uint16]) bool {
			v, present := val.Val()
			return present && v == 256
		}).Await(t)

		// Verify /sampling/sflow/collectors/collector/state/...
		collectorState := gnmi.OC().Sampling().Sflow().Collector(collectorAddr, collectorPort)
		gnmi.Watch(t, dut, collectorState.Address().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
			v, present := val.Val()
			return present && v == collectorAddr
		}).Await(t)
		gnmi.Watch(t, dut, collectorState.Port().State(), 30*time.Second, func(val *ygnmi.Value[uint16]) bool {
			v, present := val.Val()
			return present && v == collectorPort
		}).Await(t)
		gnmi.Watch(t, dut, collectorState.NetworkInstance().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
			v, present := val.Val()
			return present && v == p.Ni
		}).Await(t)
		if !deviations.SflowSourceAddressUpdateUnsupported(dut) {
			gnmi.Watch(t, dut, collectorState.SourceAddress().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
				v, present := val.Val()
				return present && v == srcAddr
			}).Await(t)
		}

		// Verify /sampling/sflow/interfaces/interface/state/...
		intfState := gnmi.OC().Sampling().Sflow().Interface(p2Name)
		gnmi.Watch(t, dut, intfState.Name().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
			v, present := val.Val()
			return present && v == p2Name
		}).Await(t)
		gnmi.Watch(t, dut, intfState.Enabled().State(), 30*time.Second, func(val *ygnmi.Value[bool]) bool {
			v, present := val.Val()
			return present && v
		}).Await(t)
		if deviations.SflowEgressSamplingRateUnsupported(dut) {
			gnmi.Watch(t, dut, gnmi.OC().Sampling().Sflow().IngressSamplingRate().State(), 30*time.Second, func(val *ygnmi.Value[uint32]) bool {
				v, present := val.Val()
				return present && v == p.MinSamplingRate
			}).Await(t)
		} else {
			gnmi.Watch(t, dut, intfState.EgressSamplingRate().State(), 30*time.Second, func(val *ygnmi.Value[uint32]) bool {
				v, present := val.Val()
				return present && v == p.MinSamplingRate
			}).Await(t)
		}

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
}

func testFlowEgress(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, ip IPType, dut *ondatra.DUTDevice, port2IfIndex uint32) {
	var myFlowConfigs []flowConfig
	if slices.Contains(kneDeviceModelList, dut.Model()) {
		myFlowConfigs = flowConfigsKNE
	} else {
		myFlowConfigs = flowConfigs
	}
	for _, fc := range myFlowConfigs {
		flowName := string(ip) + fc.name
		t.Run(flowName, func(t *testing.T) {
			createFlow(t, ate, config, fc, ip)
			cs := startCapture(t, ate, config)
			if fc.ppsRate == 0 {
				t.Fatal("ppsRate cannot be zero")
			}
			sleepTime := time.Duration(fc.packetsToSend/uint32(fc.ppsRate)) + 5
			ate.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			ate.OTG().StopTraffic(t)
			stopCapture(t, ate, cs)
			otgutils.LogFlowMetrics(t, ate.OTG(), config)
			otgutils.LogPortMetrics(t, ate.OTG(), config)
			loss := otgutils.GetFlowLossPct(t, ate.OTG(), flowName, 10*time.Second)
			if loss > lossTolerance {
				t.Errorf("Loss percent for %s Traffic: got: %f, want <= %f", ip, loss, float64(lossTolerance))
			}
			processCapture(t, ate, config, ip, fc, port2IfIndex)
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

func processCapture(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, ip IPType, fc flowConfig, port2IfIndex uint32) {
	bytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(config.Ports().Items()[1].Name()))
	pcapFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v", err)
	}
	defer os.Remove(pcapFile.Name())
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v", err)
	}
	pcapFile.Close()
	validateEgressPackets(t, pcapFile.Name(), ip, fc, port2IfIndex)
}

func verifyNoSFlowPackets(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, ip IPType) {
	t.Helper()
	bytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(config.Ports().Items()[1].Name()))
	pcapFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v", err)
	}
	defer os.Remove(pcapFile.Name())
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v", err)
	}
	pcapFile.Close()
	handle, err := pcap.OpenOffline(pcapFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	sflowSamples := uint32(0)
	for packet := range packetSource.Packets() {
		if sflowLayer := packet.Layer(layers.LayerTypeSFlow); sflowLayer != nil {
			sflow := sflowLayer.(*layers.SFlowDatagram)
			sflowSamples += sflow.SampleCount
		}
	}
	t.Logf("After disabling egress sFlow: captured %d sFlow samples", sflowSamples)
	if sflowSamples != 0 {
		t.Errorf("After disabling egress sFlow, got %d samples, want 0", sflowSamples)
	}
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
	loopbackIntfName := netutil.LoopbackInterface(t, dut, loopbackSubIntfNum)
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
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(fc.packetsToSend))
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
	otgutils.WaitForARP(t, ate.OTG(), config, string(ip))
}

func validateEgressPackets(t *testing.T, filename string, ip IPType, fc flowConfig, port2IfIndex uint32) {
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
	packetCount := 0
	sflowSamples := uint32(0)
	if fc.minSamplingRate == 0 {
		t.Fatal("minSamplingRate cannot be zero")
	}
	expectedSampleCount := float64(fc.packetsToSend / fc.minSamplingRate)
	minAllowedSamples := expectedSampleCount * sampleTolerance
	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ipv4, _ := ipLayer.(*layers.IPv4)
			if ipv4.SrcIP.Equal(loopbackIP) && ipv4.TOS == 32 {
				packetCount++
			}
		} else if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
			ipv6, _ := ipLayer.(*layers.IPv6)
			if ipv6.SrcIP.Equal(loopbackIP) && ipv6.TrafficClass == 32 {
				packetCount++
			}
		}
		if sflowLayer := packet.Layer(layers.LayerTypeSFlow); sflowLayer != nil {
			sflow := sflowLayer.(*layers.SFlowDatagram)
			if sflow.DatagramVersion != 5 {
				t.Errorf("SFlow DatagramVersion got %d, want 5", sflow.DatagramVersion)
			}
			if !sflow.AgentAddress.Equal(loopbackIP) {
				t.Errorf("SFlow AgentAddress got %v, want %v", sflow.AgentAddress, loopbackIP)
			}
			for _, sample := range sflow.FlowSamples {
				if sample.OutputInterface == 0 {
					t.Errorf("Egress sFlow sample OutputInterface got 0, want non-zero egress interface index")
				} else if port2IfIndex != 0 && sample.OutputInterface != port2IfIndex {
					t.Errorf("Egress sFlow sample OutputInterface got %d, want port2 ifIndex %d", sample.OutputInterface, port2IfIndex)
				}
			}
			sflowSamples += sflow.SampleCount
		}
	}
	t.Logf("Egress SFlow Packet count: %v - SampleCount: %v", packetCount, sflowSamples)
	if sflowSamples < uint32(minAllowedSamples) {
		t.Errorf("Egress SFlow sample count %v, want >= %v", sflowSamples, minAllowedSamples)
	}
}
