// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package default_copp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	broadcastMAC            = "FF:FF:FF:FF:FF:FF"
	unknownMAC              = "02:10:02:01:01:01"
	ipv4PrefixLen           = 30
	ipv6PrefixLen           = 126
	ipv4DstPfx              = "172.16.0.0"
	ethernetCsmacd          = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag           = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	thresholdCPUUtilization = 80
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutSrc",
		IPv4:    "192.168.1.1",
		IPv6:    "2001:DB8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.168.1.2",
		MAC:     "02:00:01:01:01:01",
		IPv6:    "2001:DB8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutDst = attrs.Attributes{
		Desc:    "dutDst",
		IPv4:    "192.168.1.5",
		IPv6:    "2001:DB8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	ateDst = attrs.Attributes{
		Name:    "ateDst",
		IPv4:    "192.168.1.6",
		MAC:     "02:00:02:01:01:01",
		IPv6:    "2001:DB8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

type flowParameters struct {
	pps           uint64
	packetSize    uint32
	trafficLayer  uint8
	trafficType   string
	dstMACAddress string
	srcIPAddress  string
	dstIPAddress  string
}

type commonEntities struct {
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	gnmiClient gpb.GNMIClient
	ctx        context.Context
}

type coppSystemTestcase struct {
	name               string
	flowParams         flowParameters
	increasedDropCount bool
	counters           []string
}

// configInterfaceDUT configures the interface with the Addrs.
func (ce *commonEntities) configInterfaceDUT(t *testing.T, i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	t.Helper()

	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if deviations.InterfaceEnabled(ce.dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)

	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(ce.dut) && !deviations.IPv4MissingEnabled(ce.dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	// Add IPv6 stack.
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(ce.dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// configureDUT configures port1, port2 on the DUT.
func (ce *commonEntities) configureDUT(t *testing.T) {
	t.Helper()

	d := gnmi.OC()

	p1 := ce.dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	i1.Enabled = ygot.Bool(true)
	gnmi.Update(t, ce.dut, d.Interface(p1.Name()).Config(), ce.configInterfaceDUT(t, i1, &dutSrc))

	p2 := ce.dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	i2.Enabled = ygot.Bool(true)
	gnmi.Update(t, ce.dut, d.Interface(p2.Name()).Config(), ce.configInterfaceDUT(t, i2, &dutDst))
}

// configureOTG configures port1 and port2 on the ATE.
func configureOTG(t *testing.T) gosnappi.Config {
	t.Helper()

	top := gosnappi.NewConfig()
	port1 := top.Ports().Add().SetName("port1")
	port2 := top.Ports().Add().SetName("port2")

	// Port1 Configuration.
	iDut1Dev := top.Devices().Add().SetName(ateSrc.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	iDut1Ipv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	iDut1Ipv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	// Port2 Configuration.
	iDut2Dev := top.Devices().Add().SetName(ateDst.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	iDut2Ipv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6")
	iDut2Ipv6.SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	return top
}

// getDroppedPktsForCounter returns the dropped packet count for a given counter.
func getDroppedPktsForCounter(t *testing.T, jsonData []byte, counterName string) float64 {
	t.Helper()

	logAndReturnErroredCount := func(format string, args ...any) float64 {
		t.Errorf(format, args...)
		return -1
	}
	var data map[string]any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return logAndReturnErroredCount("Error unmarshalling JSON: %v", err)
	}
	ingressVoqs, ok := data["ingressVoqs"].(map[string]any)
	if !ok {
		return logAndReturnErroredCount("Error getting ingressVoqs: %s", counterName)
	}
	sources, ok := ingressVoqs["sources"].(map[string]any)
	if !ok {
		return logAndReturnErroredCount("Error getting sources: %s", counterName)
	}
	all, ok := sources["all"].(map[string]any)
	if !ok {
		return logAndReturnErroredCount("Error getting all: %s", counterName)
	}
	cpuClasses, ok := all["cpuClasses"].(map[string]any)
	if !ok {
		return logAndReturnErroredCount("Error getting cpuClasses: %s", counterName)
	}
	classDataMap, ok := cpuClasses[counterName].(map[string]any)
	if !ok {
		return logAndReturnErroredCount("Error getting stats for counter: %s", counterName)
	}
	ports, ok := classDataMap["ports"].(map[string]any)
	if !ok {
		return logAndReturnErroredCount("Error getting ports for counter: %s", counterName)
	}
	portStats, ok := ports[""].(map[string]any)
	if !ok {
		return logAndReturnErroredCount("Error getting port stats for counter: %s", counterName)
	}
	droppedPackets, ok := portStats["droppedPackets"]
	if !ok {
		return logAndReturnErroredCount("Error getting dropped packets for counter: %s", counterName)
	}
	packetsDropped, ok := droppedPackets.(float64)
	if !ok {
		return logAndReturnErroredCount("Error getting packets dropped for counter: %s", counterName)
	}
	return packetsDropped
}

// createTrafficFlows creates traffic flows for the given flow parameters.
func (ce *commonEntities) createTrafficFlows(t *testing.T, top gosnappi.Config, flowParams *flowParameters) {
	t.Helper()

	flowName := fmt.Sprintf("%d-%s-Flow:", flowParams.trafficLayer, flowParams.trafficType)

	flow := top.Flows().Add().SetName(flowName)
	flow.TxRx().Port().
		SetTxName(ce.ate.Port(t, "port1").ID()).
		SetRxNames([]string{ce.ate.Port(t, "port2").ID()})

	flow.Metrics().SetEnable(true)
	flow.Rate().SetPps(flowParams.pps)
	flow.Size().SetFixed(flowParams.packetSize)
	flow.Duration().Continuous()

	eth := flow.Packet().Add().Ethernet()
	if flowParams.trafficType == "l3LpmOverflow" {
		eth.Src().SetValue(unknownMAC)
	} else {
		eth.Src().SetValue(ateSrc.MAC)
	}

	if flowParams.trafficType == "l2Bcast" {
		eth.Dst().SetValue(flowParams.dstMACAddress)
	} else {
		dutDstInterface := ce.dut.Port(t, "port1").Name()
		dstMac := gnmi.Get(t, ce.dut, gnmi.OC().Interface(dutDstInterface).Ethernet().MacAddress().State())
		eth.Dst().SetValue(dstMac)
	}

	if flowParams.trafficType == "lacp" {
		slowMACAddress := "01:80:c2:00:00:02"
		eth.Dst().SetValue(slowMACAddress)
		eth.EtherType().SetValue(0x8809)
	}

	if flowParams.trafficLayer == 3 {
		ip := flow.Packet().Add().Ipv4()
		if flowParams.srcIPAddress != "" {
			ip.Src().SetValue(flowParams.srcIPAddress)
		} else {
			ip.Src().SetValue(ateSrc.IPv4)
		}

		if flowParams.dstIPAddress != "" {
			ip.Dst().SetValue(flowParams.dstIPAddress)
		} else {
			dstIP := ipv4DstPfx
			ip.Dst().Increment().SetStart(dstIP).SetCount(200)
		}
	}
}

// checkCPUUtilization checks the CPU utilization of the device.
func (ce *commonEntities) checkCPUUtilization(t *testing.T) error {
	t.Helper()

	dut := ondatra.DUT(t, "dut")
	t.Helper()
	cpuList := gnmi.OC().System().CpuAny().State()
	cpus := gnmi.GetAll(t, dut, cpuList)
	for _, cpu := range cpus {
		cpuUtil := gnmi.OC().System().Cpu(cpu.GetIndex()).Total().Avg().State()
		utilization := gnmi.Get(t, dut, cpuUtil)
		if utilization > thresholdCPUUtilization {
			return fmt.Errorf("high CPU utilization seen, cpu name: %d, output: %d%%", cpu.GetIndex(), utilization)
		}
		t.Logf("CPU utilization within limit, cpu name: %d, output: %d%%\n", cpu.GetIndex(), utilization)
	}
	return nil
}

// runTraffic starts and stops the traffic flow.
func (ce *commonEntities) runTraffic(t *testing.T) {
	t.Helper()

	t.Log("Starting traffic for 15 seconds")
	ce.ate.OTG().StartTraffic(t)
	for idx := 0; idx < 3; idx++ {
		time.Sleep(5 * time.Second)
		if err := ce.checkCPUUtilization(t); err != nil {
			t.Errorf("CPU utilization check failed: %v", err)
		}
	}

	t.Log("Stopping traffic and waiting 10 seconds for traffic stats to complete")
	ce.ate.OTG().StopTraffic(t)
	time.Sleep(10 * time.Second)
}

// getDroppedPktCounts returns the dropped packet counts for the given counters.
func (ce *commonEntities) getDroppedPktCounts(t *testing.T, counters []string) []float64 {
	t.Helper()

	command := "show cpu counters queue summary"
	getRequest := &gpb.GetRequest{
		Path: []*gpb.Path{
			{
				Origin: "cli",
				Elem: []*gpb.PathElem{
					{Name: command},
				},
			},
		},
		Encoding: gpb.Encoding_JSON_IETF,
	}

	getResponse, err := ce.gnmiClient.Get(ce.ctx, getRequest)
	var droppedPkts []float64
	if err != nil {
		t.Errorf("error during gNMI Get: %s", err)
		return droppedPkts
	}
	notifications := getResponse.GetNotification()
	jsonData := notifications[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
	for _, counter := range counters {
		droppedPkts = append(droppedPkts, getDroppedPktsForCounter(t, jsonData, counter))
	}
	return droppedPkts
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// testCoppSystemHelper is a helper function for testing COPP system.
func (ce *commonEntities) testCoppSystemHelper(t *testing.T, tc *coppSystemTestcase) {
	t.Helper()

	initialPktCounts := ce.getDroppedPktCounts(t, tc.counters)
	t.Logf("Configure OTG")
	top := configureOTG(t)
	ce.createTrafficFlows(t, top, &tc.flowParams)

	t.Log("Pushing the following config to the OTG device")
	t.Log(top.String())
	otgObj := ce.ate.OTG()
	otgObj.PushConfig(t, top)
	otgObj.StartProtocols(t)

	incomingPort := "port1"
	initialCounters := gnmi.Get(t, ce.dut, gnmi.OC().Interface(ce.dut.Port(t, incomingPort).Name()).Counters().State())
	initialInPkts := initialCounters.GetInPkts()
	ce.runTraffic(t)
	otgObj.StopProtocols(t)
	finalCounters := gnmi.Get(t, ce.dut, gnmi.OC().Interface(ce.dut.Port(t, incomingPort).Name()).Counters().State())
	finalInPkts := finalCounters.GetInPkts()
	t.Logf("Testcase: %s, initial incoming packets: %v", tc.name, initialInPkts)
	t.Logf("Testcase: %s, final incoming packets: %v", tc.name, finalInPkts)
	finalPktCounts := ce.getDroppedPktCounts(t, tc.counters)
	for idx, counter := range tc.counters {
		if tc.increasedDropCount && finalPktCounts[idx] <= initialPktCounts[idx] {
			t.Errorf("Testcase: %s, Counter: %s, Drop count validation failed. Final dropped pkt count: %v, Initial dropped pkt count: %v", tc.name, counter, finalPktCounts[idx], initialPktCounts[idx])
			continue
		}
		if tc.increasedDropCount == false && finalPktCounts[idx] != initialPktCounts[idx] {
			t.Errorf("Testcase: %s, Counter: %s, Drop count validation failed. Final dropped pkt count: %v, Initial dropped pkt count: %v", tc.name, counter, finalPktCounts[idx], initialPktCounts[idx])
			continue
		}
		t.Logf("Testcase: %s, Counter: %s, Drop count validation success. Final dropped pkt count: %v, Initial dropped pkt count: %v", tc.name, counter, finalPktCounts[idx], initialPktCounts[idx])
	}
}

// TestCoppSystem tests the COPP system. It configures the DUT and ATE,
// and then runs a series of tests to validate the COPP system.
func TestCoppSystem(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Errorf("failed to dial gNMI: %v", err)
		return
	}

	ce := &commonEntities{
		dut:        dut,
		ate:        ate,
		gnmiClient: gnmiClient,
		ctx:        ctx,
	}

	ce.configureDUT(t)
	// TODO: Add test cases for BGP, LDP and LLDP traffic. Add test case for CoppSystemL3DstMiss
	testCases := []coppSystemTestcase{
		{
			name:               "CoppSystemL3LpmOverflowExceedingLimitTest",
			flowParams:         flowParameters{pps: 20000, packetSize: 512, trafficLayer: 3, trafficType: "l3LpmOverflow"},
			increasedDropCount: true,
			counters:           []string{"CoppSystemL3LpmOverflow"},
		},
		{
			name:               "CoppSystemL3LpmOverflowInLimitTest",
			flowParams:         flowParameters{pps: 200, packetSize: 512, trafficLayer: 3, trafficType: "l3LpmOverflow"},
			increasedDropCount: false,
			counters:           []string{"CoppSystemL3LpmOverflow"},
		},
		{
			name:               "CoppSystemL2UcastExceedingLimitTest",
			flowParams:         flowParameters{pps: 600000, packetSize: 512, trafficLayer: 2},
			increasedDropCount: true,
			counters:           []string{"CoppSystemL2Ucast"},
		},
		{
			name:               "CoppSystemL2UcastInLimitTest",
			flowParams:         flowParameters{pps: 600, packetSize: 512, trafficLayer: 2},
			increasedDropCount: false,
			counters:           []string{"CoppSystemL2Ucast"},
		},
		{
			name:               "CoppSystemIpUcastExceedingLimitTest",
			flowParams:         flowParameters{pps: 600000, packetSize: 512, trafficLayer: 3, trafficType: "ipUcast", dstIPAddress: dutSrc.IPv4},
			increasedDropCount: true,
			counters:           []string{"CoppSystemIpUcast"},
		},
		{
			name:               "CoppSystemIpUcastInLimitTest",
			flowParams:         flowParameters{pps: 600, packetSize: 512, trafficLayer: 3, trafficType: "ipUcast", dstIPAddress: dutSrc.IPv4},
			increasedDropCount: false,
			counters:           []string{"CoppSystemIpUcast"},
		},
		{
			name:               "CoppSystemL2BcastExceedingLimitTest",
			flowParams:         flowParameters{pps: 600000, packetSize: 512, trafficLayer: 2, trafficType: "l2Bcast", dstMACAddress: broadcastMAC},
			increasedDropCount: true,
			counters:           []string{"CoppSystemL2Bcast"},
		},
		{
			name:               "CoppSystemL2BcastInLimitTest",
			flowParams:         flowParameters{pps: 600, packetSize: 512, trafficLayer: 2, trafficType: "l2Bcast", dstMACAddress: broadcastMAC},
			increasedDropCount: false,
			counters:           []string{"CoppSystemL2Bcast"},
		},
		{
			name:               "CoppSystemLacpExceedingLimitTest",
			flowParams:         flowParameters{pps: 600000, packetSize: 512, trafficLayer: 2, trafficType: "lacp"},
			increasedDropCount: true,
			counters:           []string{"CoppSystemLacp"},
		},
		{
			name:               "CoppSystemLacpInLimitTest",
			flowParams:         flowParameters{pps: 600, packetSize: 512, trafficLayer: 2, trafficType: "lacp"},
			increasedDropCount: false,
			counters:           []string{"CoppSystemLacp"},
		},
	}

	for idx := range testCases {
		tc := &testCases[idx]
		t.Run(tc.name, func(t *testing.T) {
			ce.testCoppSystemHelper(t, tc)
		})
	}
}
