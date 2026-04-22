// Copyright 2024 Google LLC
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

package ingress_arp_policing_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ipv4PrefixLen   = 30
	trafficDuration = 30 * time.Second
	rateKbps        = 1500
	fixedSize       = 64
	fixedTotalPkts  = 20000
	expectedLoss    = true
	cir             = 1000
	burstCount      = 100
	classifierName  = "ARP-match"
	newTermName     = "ARP"
	ethType         = 2054
	groupName       = "arp-policer"
	SchedulerName   = "ARP-policer"
	flowName        = "arp-test"
	tolerance       = 5
	macB            = "ff:ff:ff:ff:ff:ff"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "port1",
		MAC:     "02:01:00:00:00:01",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}
	otgPort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "port2",
		MAC:     "02:01:00:00:00:02",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}
	otgPort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
	timeout = 1 * time.Minute
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIngressArpPolicing(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	config := configureATE(t, ate)
	verifyPortsUp(t, dut.Device)
	addFlow(t, config)
	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), config, "IPv4")
	t.Run("VerifyQoSCounters", func(t *testing.T) {
		t.Log("Validating traffic without policy-map")
		txRateKbps, rxRateKbps := startAndMeasureTraffic(t, ate, trafficDuration)
		validateRates(t, txRateKbps, rxRateKbps, txRateKbps, tolerance, "Without policy-map")
		verifyTrafficAndLog(t, "QoS Counters", verifyPortTraffic(t, ate, config))
	})
	t.Run("VerifyTransmitOctetsWithCIR", func(t *testing.T) {
		t.Log("Validating traffic with policy-map")
		configureDUTTrafficPolicy(t, dut, dut.Port(t, "port1").Name())
		txRateKbps, rxRateKbps := startAndMeasureTraffic(t, ate, trafficDuration)
		validateRates(t, txRateKbps, rxRateKbps, float64(cir), tolerance, "With CIR restriction")
		verifyTrafficAndLog(t, "Transmit/Octets and Loss", verifyPortTrafficwithCir(t, ate, config))
	})
}

func startAndMeasureTraffic(t *testing.T, ate *ondatra.ATEDevice, duration time.Duration) (float64, float64) {
	t.Helper()
	ate.OTG().StartTraffic(t)
	time.Sleep(duration)
	// Measure only while traffic is running
	gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters()
	txRate := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).OutRate().State())
	rxRate := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).InRate().State())
	// Stop traffic *after* measurement
	ate.OTG().StopTraffic(t)
	// TODO: currently not implemented to get txRateInKbps from traffic counters.
	txRateKbps := (txRate * 512) / 1000 // assuming 64-byte avg packet (512 bits)
	rxRateKbps := (rxRate * 512) / 1000

	t.Logf("Measured TX Rate: %.2f Kbps, RX Rate: %.2f Kbps", txRateKbps, rxRateKbps)
	return float64(txRateKbps), float64(rxRateKbps)
}

func validateRates(t *testing.T, txRateKbps, rxRateKbps float64, expectedRate float64, tolerance float64, description string) {
	t.Helper()
	diff := math.Abs(rxRateKbps - expectedRate)
	if diff <= tolerance {
		t.Logf("%s Passed: TX=%.2f Kbps, RX=%.2f Kbps (Δ=%.2f Kbps within tolerance %.2f)", description, expectedRate, rxRateKbps, diff, tolerance)
	} else {
		t.Errorf("%s Failed: TX=%.2f Kbps, RX=%.2f Kbps (Δ=%.2f Kbps exceeds tolerance %.2f)", description, expectedRate, rxRateKbps, diff, tolerance)
	}
}

func verifyTrafficAndLog(t *testing.T, label string, ok bool) {
	t.Helper()
	if ok {
		t.Logf("%s: Verification passed", label)
	} else {
		t.Errorf("%s: Verification failed", label)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	// Configure interfaces
	p1 := dut.Port(t, "port1").Name()
	i1 := dutPort1.NewOCInterface(p1, dut)
	gnmi.Replace(t, dut, d.Interface(p1).Config(), i1)

	p2 := dut.Port(t, "port2").Name()
	i2 := dutPort2.NewOCInterface(p2, dut)
	gnmi.Replace(t, dut, d.Interface(p2).Config(), i2)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
}

func configureDUTTrafficPolicy(t *testing.T, dut *ondatra.DUTDevice, portName string) {
	t.Helper()
	if deviations.TunnelConfigPathUnsupported(dut) {
		gnmiClient := dut.RawAPIs().GNMI(t)
		cliCommands := fmt.Sprintf(`
		policy-map type quality-of-service ARP-policing-Qos
		class ARP-CM
		set traffic-class 2
		police rate %d kbps burst-size %d kbytes
		class-map type qos match-any ARP-CM
		match mac access-group ARP-policing-Macl
		mac access-list ARP-policing-Macl
		counters per-entry
		10 permit any any arp payload offset 1 pattern 0x00000001 mask 0xffffff00
		interface %s
		service-policy type qos input ARP-policing-Qos
		`, cir, burstCount, portName)
		gpbSetRequest := buildCLIConfigRequest(cliCommands)

		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	} else {
		dp := dut.Port(t, "port1").Name()
		config := &oc.Root{}
		qos := config.GetOrCreateQos()
		// Classifier for ARP
		classifier := qos.GetOrCreateClassifier(classifierName)
		classifier.SetName(classifierName)
		classifier.SetType(oc.Qos_Classifier_Type_ETHERNET)
		term, err := classifier.NewTerm(newTermName)
		if err != nil {
			t.Fatalf("Failed to create classifier term: %v", err)
		}
		term.SetId(newTermName)
		l2 := term.GetOrCreateConditions().GetOrCreateL2()
		l2.SetEthertype(oc.UnionUint16(ethType))
		term.GetOrCreateActions().SetTargetGroup(groupName)
		// Scheduler Policy with One-Rate Two-Color
		schedPolicy := qos.GetOrCreateSchedulerPolicy(SchedulerName)
		schedPolicy.SetName(SchedulerName)

		scheduler := schedPolicy.GetOrCreateScheduler(0)
		scheduler.SetSequence(0)
		scheduler.SetPriority(oc.Scheduler_Priority_UNSET)

		input := scheduler.GetOrCreateInput(groupName)
		input.SetId(groupName)

		or2c := scheduler.GetOrCreateOneRateTwoColor()
		or2c.SetCir(cir)
		or2c.SetBc(burstCount)
		or2c.GetOrCreateExceedAction().SetDrop(true)

		// Apply to interface
		qosIntf := qos.GetOrCreateInterface(dp)
		qosIntf.SetInterfaceId(dp)
		if deviations.InterfaceRefConfigUnsupported(dut) {
			qosIntf.GetOrCreateInterfaceRef().SetInterface(dp)
		}
		// Push full config
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qos)
	}
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	otgPort1.AddToOTG(config, ap1, &dutPort1)
	otgPort2.AddToOTG(config, ap2, &dutPort2)
	return config
}

func addFlow(t *testing.T, config gosnappi.Config) {
	t.Helper()
	config.Flows().Clear()
	flow := config.Flows().Add()
	flow.SetName(flowName)
	flow.Size().SetFixed(fixedSize)
	flow.Rate().SetKbps(rateKbps)
	flow.TxRx().Port().SetTxName("port1").SetRxNames([]string{"port1"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(otgPort1.MAC)
	ethHeader.Dst().SetValue(macB)
	arpHeader := flow.Packet().Add().Arp()
	arpHeader.SenderHardwareAddr().SetValue(otgPort1.MAC)
	arpHeader.SenderProtocolAddr().SetValue(otgPort1.IPv4)
	arpHeader.TargetHardwareAddr().SetValue("00:00:00:00:00:00")
	arpHeader.TargetProtocolAddr().SetValue(dutPort1.IPv4)
}

func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	t.Log("Verifying port status")
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if status != oc.Interface_OperStatus_UP {
			t.Fatalf("[%s]: Interface %s status: got %v, expected UP", "verifyPortsUp", p.Name(), status)
		}
	}
}

func verifyPortTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) bool {
	otgutils.LogPortMetrics(t, ate.OTG(), config)
	return validatePortTraffic(t, ate, ate.Port(t, "port1").ID(), false, "flow-unrestricted")
}

func verifyPortTrafficwithCir(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) bool {
	otgutils.LogPortMetrics(t, ate.OTG(), config)
	return validatePortTraffic(t, ate, ate.Port(t, "port1").ID(), true, "flow-CIR-restricted")
}

func validatePortTraffic(t *testing.T, ate *ondatra.ATEDevice, portID string, expectDrop bool, flowName string) bool {
	t.Helper()
	const fn = "validatePortTraffic"

	countersPath := gnmi.OTG().Port(portID).Counters()
	txRate := gnmi.Get(t, ate.OTG(), countersPath.OutFrames().State())
	rxRate := gnmi.Get(t, ate.OTG(), countersPath.InFrames().State())

	isWithinTolerance := func(expected, actual uint64) bool {
		return actual >= expected-tolerance && actual <= expected+tolerance
	}

	// TX validation
	txVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.OutFrames().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && isWithinTolerance(txRate, v)
		}).Await(t)

	if !ok {
		t.Errorf("[%s] TX did not reach expected count (%d)", fn, txRate)
		return false
	}

	// RX validation
	rxVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.InFrames().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && isWithinTolerance(rxRate, v)
		}).Await(t)

	if !ok {
		if expectDrop {
			t.Logf("[%s] Expected packet drop verified (RX lower than TX)", fn)
		} else {
			t.Errorf("[%s] RX packets did not match expected TX count (%d)", fn, txRate)
			return false
		}
	}

	// Octets validation (only for CIR test)
	if expectDrop {
		rxOctets := gnmi.Get(t, ate.OTG(), countersPath.InOctets().State())
		txOctets := gnmi.Get(t, ate.OTG(), countersPath.OutOctets().State())
		if txOctets > rxOctets && rxOctets != 0 && txOctets != 0 {
			t.Logf("[%s] In/Out Octets: TX=%d, RX=%d", fn, txOctets, rxOctets)
		} else {
			t.Errorf("[%s] Failed to validate In/Out Octets", fn)
			return false
		}
	}

	// Final log
	txPkts, _ := txVal.Val()
	rxPkts, _ := rxVal.Val()
	t.Logf("[%s] %s: TX=%d, RX=%d", fn, flowName, txPkts, rxPkts)

	return true
}

// Support method to execute GNMIC commands
func buildCLIConfigRequest(config string) *gpb.SetRequest {
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{
					Origin: "cli",
					Elem:   []*gpb.PathElem{},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_AsciiVal{
						AsciiVal: config,
					},
				},
			},
		},
	}
	return gpbSetRequest
}
