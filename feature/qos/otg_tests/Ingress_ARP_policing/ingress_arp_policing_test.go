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
	trafficDuration = 20
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
	tolerance       = 2
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
	addFlow(config)
	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), config, "IPv4")
	t.Run("Verify Qos Counters", func(t *testing.T) {
		t.Logf("Validating the traffic without apply policy-map")
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)

		t.Logf("Get configured TX/RX rate on %s", gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).Name().State()))
		rxRate := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).InRate().State())
		txRate := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).OutRate().State())
		t.Log("txRate rxRate", txRate, rxRate)
		txRateInKbps := (txRate * 512) / 1000
		rxRateInKbps := (rxRate * 512) / 1000
		diff := txRate - rxRate
		if diff <= float32(tolerance) {
			t.Logf("TX : %f RX : %f rate matched without restriction on DUT", txRateInKbps, rxRateInKbps)
		} else {
			t.Errorf("Failed to match TX : %f RX : %f rate without restriction on DUT", txRateInKbps, rxRateInKbps)
		}
		ate.OTG().StopTraffic(t)
		if verifyPortTraffic(t, ate, config) {
			t.Log("Verified Qos counters")
		} else {
			t.Error("Failed to verify Qos counters")
		}
	})
	t.Run("Verify Transmit/Octets Packets and 0.5Kbps Packet Loss", func(t *testing.T) {
		t.Logf("Validating the traffic with apply policy-map")
		configureDUTTrafficPolicy(t, dut, dut.Port(t, "port1").Name())
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		rxRate := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).InRate().State())
		txRate := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).OutRate().State())
		t.Log("After restrict rxRate, txRate :", rxRate, txRate)
		rxRateInKbps := (rxRate * 512) / 1000
		diff := float32(cir) - rxRateInKbps
		if diff < 0 {
			diff = -diff // absolute value
		}
		if diff <= float32(tolerance) {
			t.Logf("TX : %d RX : %f rate matched with CIR restriction on DUT", cir, rxRateInKbps)
		} else {
			t.Errorf("Failed to match TX : %d RX : %f rate with CIR restriction on DUT", cir, rxRateInKbps)
		}
		ate.OTG().StopTraffic(t)
		if verifyPortTrafficwithCir(t, ate, config) {
			t.Log("Verfied Transmit/Octets Packets and 0.5Kbps Packet Loss")
		} else {
			t.Error("Failed to validate Transmit/Octets Packets and 0.5Kbps Packet Loss")
		}

	})

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
	if deviations.TunnelConfigPathUnsupported(dut) {
		gnmiClient := dut.RawAPIs().GNMI(t)
		jsonConfig := fmt.Sprintf(`
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
		gpbSetRequest := buildCliConfigRequest(jsonConfig)

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
	// t.Log("Configure ATE interface")
	config := gosnappi.NewConfig()

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	otgPort1.AddToOTG(config, ap1, &dutPort1)
	otgPort2.AddToOTG(config, ap2, &dutPort2)
	return config
}

func addFlow(config gosnappi.Config) {
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

// Verify ports status
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	t.Log("Verifying port status")
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

func verifyPortTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) bool {
	otgutils.LogPortMetrics(t, ate.OTG(), config)
	countersPath := gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters()
	txRate := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters().OutFrames().State())
	isWithinTolerance := func(v uint64) bool {
		return v >= txRate-tolerance && v <= txRate+tolerance
	}
	// Wait for TX to reach expected count
	txVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.OutFrames().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && isWithinTolerance(v)
		}).Await(t)

	if !ok {
		t.Errorf("Port Stats: TX did not reach expected count (%d)", txRate)
		return false
	}
	// Wait for RX to match TX exactly
	rxVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.InFrames().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && isWithinTolerance(v)
		}).Await(t)

	if !ok {
		t.Errorf("Port Stats: RX packets did not match expected TX count (%d)", txRate)
		return false
	}

	txPkts, _ := txVal.Val()
	rxPkts, _ := rxVal.Val()
	t.Logf("Flow %q: TX=%d, RX=%d", flowName, txPkts, rxPkts)
	return true
}

func verifyPortTrafficwithCir(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) bool {
	otgutils.LogPortMetrics(t, ate.OTG(), config)
	txRate := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters().OutFrames().State())
	txisWithinTolerance := func(v uint64) bool {
		return v >= txRate-tolerance && v <= txRate+tolerance
	}
	rxRate := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters().InFrames().State())
	rxisWithinTolerance := func(v uint64) bool {
		return v >= rxRate-tolerance && v <= rxRate+tolerance
	}
	countersPath := gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters()
	// Wait for TX to reach expected count
	txVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.OutFrames().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && txisWithinTolerance(v)
		}).Await(t)

	if !ok {
		t.Errorf("Port Stats: TX did not reach expected count (%d)", txRate)
		return false
	}
	rxVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.InFrames().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && rxisWithinTolerance(v)
		}).Await(t)

	if !ok {
		t.Errorf("Port Stats: RX packets did not match after 0.5 Kbps drop packets (%d)", rxRate)
		return false
	}
	txPkts, _ := txVal.Val()
	rxPkts, _ := rxVal.Val()
	t.Logf("Port Counts %q: TX=%d, RX=%d", flowName, txPkts, rxPkts)
	rXInOctets := gnmi.Get(t, ate.OTG(), countersPath.InOctets().State())
	tXOutOctets := gnmi.Get(t, ate.OTG(), countersPath.OutOctets().State())
	if tXOutOctets > rXInOctets && rXInOctets != 0 && tXOutOctets != 0 {
		t.Logf("Port In/Out Octets %q: TX=%d, RX=%d", flowName, tXOutOctets, rXInOctets)
	} else {
		t.Errorf("Failed to validate In/Out Octets")
		return false
	}

	return true
}

// Support method to execute GNMIC commands
func buildCliConfigRequest(config string) *gpb.SetRequest {
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
