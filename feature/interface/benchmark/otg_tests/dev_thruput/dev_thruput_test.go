package dev_thruput_test

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	numSnakePorts  = 34
	totalVlans     = 17
	trafficRunTime = 30 * time.Second
	startVlanID    = 100
	burstMinGap    = 760
	lossTolerance  = 2
)

// Ports attrs
var (
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	sizeWeightProfile = []sizeWeight{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 10},
		{Size: 1500, Weight: 30},
	}
)

type sizeWeight struct {
	Size   uint32
	Weight float32
}

type portCounters struct {
	inPkts, outPkts uint64
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUT configures a 'snake' of VLANs across multiple DUT ports.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []*ondatra.Port {
	t.Helper()
	d := new(oc.Root)
	var dutPorts []*ondatra.Port
	for i := 1; i <= numSnakePorts; i++ {
		dutPorts = append(dutPorts, dut.Port(t, fmt.Sprintf("port%d", i)))
	}
	batch := new(gnmi.SetBatch)
	// Create VLANs and assign ports.
	for i := 1; i <= totalVlans; i++ {
		vlanID := uint16(startVlanID + i - 1)
		cfgplugins.ConfigureVlan(t, dut, cfgplugins.VlanParams{VlanID: vlanID})
		// Each VLAN uses a pair of ports: [2*(i-1)] and [2*(i-1)+1]
		portStart := 2 * (i - 1)
		portEnd := portStart + 2
		// Safety check to avoid out-of-range access
		if portEnd > len(dutPorts) {
			t.Fatalf("Not enough ports in dutPorts for VLAN %d (need %d, have %d)", vlanID, portEnd, len(dutPorts))
		}
		for _, portObj := range dutPorts[portStart:portEnd] {
			dutIntObj := d.GetOrCreateInterface(portObj.Name())
			dutIntObj.SetName(portObj.Name())
			dutIntObj.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
			dutIntObj.GetOrCreateEthernet().GetOrCreateSwitchedVlan().SetAccessVlan(vlanID)
			gnmi.BatchReplace(batch, gnmi.OC().Interface(portObj.Name()).Config(), dutIntObj)
		}
	}
	batch.Set(t, dut)

	return dutPorts
}

// getCounters collects unicast packet counters for a list of DUT interfaces.
func getCounters(t *testing.T, dut *ondatra.DUTDevice, dutPortList []*ondatra.Port) map[string]portCounters {
	t.Helper()
	counters := make(map[string]portCounters)
	for _, port := range dutPortList {
		counters[port.Name()] = portCounters{
			inPkts:  gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().InUnicastPkts().State()),
			outPkts: gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().OutUnicastPkts().State()),
		}
	}
	return counters
}

// configureATE configures the ATE ports with IPv4/v6 addresses.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	ateConfig := gosnappi.NewConfig()
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	p1 := ateConfig.Ports().Add().SetName(ap1.ID())
	p2 := ateConfig.Ports().Add().SetName(ap2.ID())
	// add devices
	d1 := ateConfig.Devices().Add().SetName(atePort1.Name + ".Dev1")
	d2 := ateConfig.Devices().Add().SetName(atePort2.Name + ".Dev2")

	// Configuration on port1.
	d1Eth1 := d1.Ethernets().Add().SetName(atePort1.Name + ".Eth1").SetMac(atePort1.MAC)
	d1Eth1.Connection().SetPortName(p1.Name())

	// VLAN start ID
	d1Eth1.Vlans().Add().SetName(atePort1.Name + ".vlanStart").SetId(startVlanID).SetPriority(0)

	// IPv4 & IPv6 over VLAN start ID
	d1Eth1.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").SetAddress(atePort1.IPv4).SetGateway(atePort2.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	d1Eth1.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6").SetAddress(atePort1.IPv6).SetGateway(atePort2.IPv6).SetPrefix(uint32(atePort1.IPv6Len))
	// Configuration on port2.
	d2Eth2 := d2.Ethernets().Add().SetName(atePort2.Name + ".Eth2").SetMac(atePort2.MAC)
	d2Eth2.Connection().SetPortName(p2.Name())

	// VLAN end ID
	d2Eth2.Vlans().Add().SetName(atePort2.Name + ".vlanEnd").SetId(startVlanID + totalVlans - 1).SetPriority(0)

	// IPv4 & IPv6 over VLAN end ID
	d2Eth2.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4").SetAddress(atePort2.IPv4).SetGateway(atePort1.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	d2Eth2.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6").SetAddress(atePort2.IPv6).SetGateway(atePort1.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	return ateConfig
}

// createTrafficFlow creates a traffic flow based on the test case parameters.
func createTrafficFlow(t *testing.T, name, addrFamily, dstMac string, frameSize uint32, pps uint64, isMixed bool, config gosnappi.Config) {
	t.Helper()
	config.Flows().Clear()
	flow := config.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.Rate().SetPps(pps)
	flow.TxRx().Port().SetTxName("port1").SetRxNames([]string{"port2"})
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort1.MAC)
	eth.Dst().SetValue(dstMac)
	flow.Packet().Add().Vlan().Id().SetValue(startVlanID)
	if addrFamily == "IPv4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(atePort1.IPv4)
		v4.Dst().SetValue(atePort2.IPv4)
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(atePort1.IPv6)
		v6.Dst().SetValue(atePort2.IPv6)
	}
	if isMixed {
		// Approximate a mixed frame size distribution.
		for _, swp := range sizeWeightProfile {
			flow.Size().WeightPairs().Custom().Add().SetSize(swp.Size).SetWeight(swp.Weight)
		}
		flow.Duration().Burst().SetGap(burstMinGap)
	} else {
		flow.Size().SetFixed(uint32(frameSize))
		flow.Duration().Continuous()
	}
}

// verifyDUTPortCounters validate the DUT ports counters.
func verifyDUTPortCounters(t *testing.T, dut *ondatra.DUTDevice, dutPortList []*ondatra.Port, baselineCounts map[string]portCounters) {
	t.Helper()
	if len(dutPortList)%2 != 0 {
		t.Fatalf("Port list must contain an even number of ports, got %d", len(dutPortList))
	}
	for dutIndx := 0; dutIndx < len(dutPortList); dutIndx += 2 {
		inPort := dutPortList[dutIndx]
		outPort := dutPortList[dutIndx+1]
		// Fetch IN and OUT counters
		inPkts := gnmi.Get(t, dut, gnmi.OC().Interface(inPort.Name()).Counters().InUnicastPkts().State()) - baselineCounts[inPort.Name()].inPkts
		outPkts := gnmi.Get(t, dut, gnmi.OC().Interface(outPort.Name()).Counters().OutUnicastPkts().State()) - baselineCounts[outPort.Name()].outPkts
		t.Logf("Pair [%d-%d]: IN(%s)=%d  →  OUT(%s)=%d", dutIndx, dutIndx+1, inPort.Name(), inPkts, outPort.Name(), outPkts)
		if inPkts == 0 {
			t.Fatalf("Test flow sent %d packets", inPkts)
		}
		if inPkts >= outPkts {
			lostPackets := inPkts - outPkts
			if got := (lostPackets * 100 / inPkts); got >= lossTolerance {
				t.Errorf("packets mismatch between %s (IN=%d) and %s (OUT=%d): diff=%d", inPort.Name(), inPkts, outPort.Name(), outPkts, lostPackets)
			} else {
				t.Logf("packets matched between %s (IN=%d) and %s (OUT=%d): diff=%d", inPort.Name(), inPkts, outPort.Name(), outPkts, lostPackets)
			}
		} else {
			t.Logf("packets matched between %s (IN=%d) and %s (OUT=%d): diff=%d", inPort.Name(), inPkts, outPort.Name(), outPkts, int64(inPkts)-int64(outPkts))
		}
	}
}

// verifyTrafficFlow checks that the flows sent and received the expected number of packets.
func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string) {
	t.Helper()
	flowCounters := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().State())
	txPackets := flowCounters.GetOutPkts()
	rxPackets := flowCounters.GetInPkts()
	if txPackets == 0 {
		t.Fatalf("Test flow %s sent 0 packets", flowName)
	}
	lostPackets := txPackets - rxPackets
	if got := (lostPackets * 100 / txPackets); got >= lossTolerance {
		t.Errorf("flow %s saw unexpected packet loss: sent %d, got %d", flowName, txPackets, rxPackets)
	} else {
		t.Logf("Flow %s traffic verification passed: sent %d, got %d", flowName, txPackets, rxPackets)
	}
}

// verifySystemHealth checks the CPU and Power utilization of the DUT.
func verifySystemHealth(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	// Check CPU utilization
	cpuPath := gnmi.OC().System().CpuAny().Total().State()
	cpus := gnmi.GetAll(t, dut, cpuPath)
	if len(cpus) == 0 {
		t.Fatalf("No CPU stats found on DUT %s", dut.Name())
	}

	var totalAvg uint64
	for _, cpu := range cpus {
		avg := uint64(cpu.GetAvg())
		totalAvg += avg
	}
	avgUtilization := uint8(totalAvg / uint64(len(cpus)))

	if avgUtilization > 80 {
		t.Errorf("high average CPU utilization detected: %d", avgUtilization)
	} else {
		t.Logf("Average CPU utilization across all CPUs: %d", avgUtilization)
	}

	// Check Power utilization
	powerPath := gnmi.OC().ComponentAny().PowerSupply().State()
	powers := gnmi.GetAll(t, dut, powerPath)
	if len(powers) == 0 {
		t.Fatalf("No power supply info found on DUT %s", dut.Name())
	}
	for idx, ps := range powers {
		capacityBytes := []byte(ps.GetCapacity())
		usedPowerBytes := []byte(ps.GetOutputPower())
		capacity := binary.BigEndian.Uint32(capacityBytes)
		usedPower := binary.BigEndian.Uint32(usedPowerBytes)
		if capacity <= usedPower {
			t.Errorf("powerSupply[%d] INVALID: Used Power (%d W) exceeds Capacity (%d W)", idx, usedPower, capacity)
		} else {
			t.Logf("PowerSupply[%d]: Capacity = %d W, UsedPower = %d W (OK)", idx, capacity, usedPower)
		}
	}
}

func TestInterfacePerformance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	t.Log("Configuring DUT interfaces and VLANs for snake topology...")
	dutPortList := configureDUT(t, dut)
	t.Log("Configuring ATE ports...")
	ateConfig := configureATE(t, ate)
	ate.OTG().PushConfig(t, ateConfig)
	ate.OTG().StartProtocols(t)
	dstMac := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	testCases := []struct {
		name          string
		addressFamily string
		frameSize     uint32
		pps           uint64
		isMixed       bool
		dstMac        string
	}{
		{"IPv4_64_Bytes", "IPv4", 64, 595000000, false, dstMac},
		{"IPv4_Mixed_Bytes", "IPv4", 760, 74000000, true, dstMac},
		{"IPv4_9000_Bytes", "IPv4", 9000, 5500000, false, dstMac},
		{"IPv6_64_Bytes", "IPv6", 64, 595000000, false, dstMac},
		{"IPv6_Mixed_Bytes", "IPv6", 760, 74000000, true, dstMac},
		{"IPv6_9000_Bytes", "IPv6", 9000, 5500000, false, dstMac},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flowName := fmt.Sprintf("flow_%s", tc.name)
			t.Logf("Creating traffic flow: %s", flowName)
			currentConfig := ate.OTG().FetchConfig(t)
			createTrafficFlow(t, flowName, tc.addressFamily, tc.dstMac, tc.frameSize, tc.pps, tc.isMixed, currentConfig)
			ate.OTG().PushConfig(t, currentConfig)

			t.Logf("Starting traffic for %s", flowName)
			baselineCounts := getCounters(t, dut, dutPortList)
			ate.OTG().StartTraffic(t)
			time.Sleep(trafficRunTime)
			t.Log("Verifying DUT system health...")
			verifySystemHealth(t, dut)
			ate.OTG().StopTraffic(t)
			t.Log("Verifying Counters on DUT Ports...")
			verifyDUTPortCounters(t, dut, dutPortList, baselineCounts)

			t.Log("Verifying traffic flow statistics...")
			verifyTrafficFlow(t, ate, flowName)
		})
	}
}
