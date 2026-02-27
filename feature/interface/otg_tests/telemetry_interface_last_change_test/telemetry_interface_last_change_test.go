// Copyright 2025 Google LLC
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

package telemetry_interface_last_change_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

const (
	lagName        = "LAGRx"
	ipv4LagAddress = "192.168.20.1"
	ipv4PrefixLen  = 30
	ipv6PrefixLen  = 126
	awaitTimeout   = 2 * time.Minute
	flapCount      = 10
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
		IPv4Len: ipv4PrefixLen,
	}
	dutDst = attrs.Attributes{
		Desc:    "dutDst",
		IPv4:    "192.168.1.5",
		IPv4Len: ipv4PrefixLen,
	}
	ateDst = attrs.Attributes{
		Name:    "ateDst",
		IPv4:    "192.168.1.6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: ipv4PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureOTGLAG configures a LAG on the ATE.
func configureOTGLAG(t *testing.T, ate *ondatra.ATEDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()

	top := gosnappi.NewConfig()
	agg := top.Lags().Add().SetName(lagName)
	lagID, _ := strconv.Atoi(aggID)
	agg.Protocol().Static().SetLagId(uint32(lagID))

	var portNames []string
	for i, p := range aggPorts {
		port := top.Ports().Add().SetName(p.ID())
		portNames = append(portNames, p.ID())
		agg.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(ateSrc.MAC).SetName(lagName + strconv.Itoa(i))
	}

	dstDev := top.Devices().Add().SetName(agg.Name() + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(lagName + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(lagName + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Bring up the ATE member ports.
	portStateAction := gosnappi.NewControlState()
	portStateAction.Port().Link().SetPortNames(portNames).SetState(gosnappi.StatePortLinkState.UP)
	ate.OTG().SetControlState(t, portStateAction)
}

// configureDUTLAG configures a LAG on the DUT.
func configureDUTLAG(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	d := &oc.Root{}
	agg := d.GetOrCreateInterface(aggID)
	agg.SetType(oc.IETFInterfaces_InterfaceType_ieee8023adLag)
	agg.GetOrCreateAggregation().SetLagType(oc.IfAggregate_AggregationType_STATIC)
	subIntf := agg.GetOrCreateSubinterface(0)
	subIntf.SetEnabled(true)
	if !deviations.IPv4MissingEnabled(dut) {
		subIntf.GetOrCreateIpv4().SetEnabled(true)
	}
	a := subIntf.GetOrCreateIpv4().GetOrCreateAddress(ipv4LagAddress)
	a.SetPrefixLength(ipv4PrefixLen)

	for _, port := range aggPorts {
		i := d.GetOrCreateInterface(port.Name())
		if deviations.FrBreakoutFix(dut) && port.PMD() == ondatra.PMD100GBASEFR {
			i.GetOrCreateEthernet().SetAutoNegotiate(false)
			i.GetOrCreateEthernet().SetDuplexMode(oc.Ethernet_DuplexMode_FULL)
			i.GetOrCreateEthernet().SetPortSpeed(oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB)
		}
		i.GetOrCreateEthernet().SetAggregateId(aggID)
		i.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

		if deviations.InterfaceEnabled(dut) {
			i.SetEnabled(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

// validateLastChangeIncrease verifies that the final last-change timestamp is greater than the initial one.
func validateLastChangeIncrease(t *testing.T, desc, intfName string, initialLC, finalLC uint64) {
	t.Helper()
	if finalLC <= initialLC {
		t.Errorf("%s: LastChange timestamp did not increase, initial: %d, final: %d for interface/sub-interface: %s", desc, initialLC, finalLC, intfName)
	} else {
		t.Logf("%s: LastChange timestamp increased as expected, initial: %d, final: %d for interface/sub-interface: %s", desc, initialLC, finalLC, intfName)
	}
}

// performLAGFlapTest is a helper function that flaps a LAG interface multiple times and
// validates that the last-change timestamp is updated at each flap. It takes a flapFunc
// which is responsible for toggling the LAG interface state.
func performLAGFlapTest(t *testing.T, dut *ondatra.DUTDevice, aggID string, dutAggPorts []*ondatra.Port, testName string, flapFunc func(t *testing.T, enable bool)) {
	t.Helper()
	aggIntfPath := gnmi.OC().Interface(aggID)
	numStateChanges := 2 * flapCount

	// Ensure the LAG interface is UP before starting the test.
	t.Logf("[%s] Ensuring LAG %s is UP", testName, aggID)
	flapFunc(t, true) // Set admin state to UP to ensure the interface is active.
	gnmi.Await(t, dut, aggIntfPath.OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_UP)

	// Get initial last-change values when the interface is UP.
	initialIntfLCVal, present := gnmi.Lookup(t, dut, aggIntfPath.LastChange().State()).Val()
	if !present {
		t.Fatalf("[%s] Failed to lookup initial LastChange for interface %s", testName, aggID)
	}
	initialSubintfLCVal, present := gnmi.Lookup(t, dut, aggIntfPath.Subinterface(0).LastChange().State()).Val()
	if !present {
		t.Fatalf("[%s] Failed to lookup initial LastChange for subinterface %s:0", testName, aggID)
	}
	t.Logf("[%s] Initial LastChange values: Interface %s: %d, Subinterface %s:0: %d", testName, aggID, initialIntfLCVal, aggID, initialSubintfLCVal)

	prevIntfLC := initialIntfLCVal
	prevSubintfLC := initialSubintfLCVal

	for i := 1; i <= numStateChanges; i++ {
		var action string
		var targetOperStatus oc.E_Interface_OperStatus
		var enabledState bool

		if i%2 != 0 { // In odd-numbered iterations, disable the interface.
			action = "Disable"
			enabledState = false
			targetOperStatus = oc.Interface_OperStatus_DOWN
			// For Juniper devices, when a LAG member port is disabled, the LAG oper status goes to LOWER_LAYER_DOWN instead of DOWN.
			if dut.Vendor() == ondatra.JUNIPER && (testName == "LAGMemberFlap" || testName == "OTGLAGFlap") {
				targetOperStatus = oc.Interface_OperStatus_LOWER_LAYER_DOWN
			}
		} else { // In even-numbered iterations, enable the interface.
			action = "Enable"
			enabledState = true
			targetOperStatus = oc.Interface_OperStatus_UP
		}

		t.Logf("[%s] Performing %s on LAG %s, state change %d/%d", testName, action, aggID, i, numStateChanges)

		// Trigger the interface state change using the provided function.
		flapFunc(t, enabledState)

		gnmi.Await(t, dut, aggIntfPath.OperStatus().State(), awaitTimeout, targetOperStatus)

		// Get last-change values after the state change.
		currentIntfLCVal, present := gnmi.Lookup(t, dut, aggIntfPath.LastChange().State()).Val()
		if !present {
			t.Errorf("[%s] Failed to lookup LastChange for interface %s after %s (state change %d)", testName, aggID, action, i)
			continue
		}
		currentSubintfLCVal, present := gnmi.Lookup(t, dut, aggIntfPath.Subinterface(0).LastChange().State()).Val()
		if !present {
			t.Errorf("[%s] Failed to lookup LastChange for subinterface %s:0 after %s (state change %d)", testName, aggID, action, i)
			continue
		}
		t.Logf("[%s] LastChange values after %s: Interface %s: %d, Subinterface %s:0: %d", testName, action, aggID, currentIntfLCVal, aggID, currentSubintfLCVal)

		// Verify that last-change increased for both the LAG interface and its subinterface.
		validateLastChangeIncrease(t, fmt.Sprintf("%s after %s (state change %d)", testName, action, i), aggID, prevIntfLC, currentIntfLCVal)
		validateLastChangeIncrease(t, fmt.Sprintf("%s after %s (state change %d)", testName, action, i), fmt.Sprintf("%s:%d", aggID, 0), prevSubintfLC, currentSubintfLCVal)

		// Store current timestamps for comparison in the next iteration.
		prevIntfLC = currentIntfLCVal
		prevSubintfLC = currentSubintfLCVal
	}
}

// TestLAGLastChangeState verifies that the last-change timestamp is updated on interface flap
// for a LAG interface and its subinterface.
func TestLAGLastChangeState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dutAggPorts := []*ondatra.Port{dut.Port(t, "port1")}
	aggID := netutil.NextAggregateInterface(t, dut)
	configureDUTLAG(t, dut, dutAggPorts, aggID)
	aggIntfPath := gnmi.OC().Interface(aggID)

	ateAggPorts := []*ondatra.Port{ate.Port(t, "port1")}
	configureOTGLAG(t, ate, ateAggPorts, aggID)

	t.Run("LAGInterfaceFlap", func(t *testing.T) {
		flapFunc := func(t *testing.T, enable bool) {
			action := "Enabling"
			if !enable {
				action = "Disabling"
			}
			t.Logf("%s LAG interface %s", action, aggID)
			gnmi.Update(t, dut, aggIntfPath.Enabled().Config(), enable)
		}
		performLAGFlapTest(t, dut, aggID, dutAggPorts, "LAGInterfaceFlap", flapFunc)
	})

	t.Run("LAGMemberFlap", func(t *testing.T) {
		flapFunc := func(t *testing.T, enable bool) {
			action := "Enabling"
			if !enable {
				action = "Disabling"
			}
			t.Logf("%s all member ports of LAG %s", action, aggID)
			for _, port := range dutAggPorts {
				t.Logf("%s port %s", action, port.Name())
				gnmi.Update(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), enable)
			}
		}
		performLAGFlapTest(t, dut, aggID, dutAggPorts, "LAGMemberFlap", flapFunc)
	})

	t.Run("OTGLAGFlap", func(t *testing.T) {
		flapFunc := func(t *testing.T, enable bool) {
			action := "Enabling"
			state := gosnappi.StatePortLinkState.UP
			if !enable {
				action = "Disabling"
				state = gosnappi.StatePortLinkState.DOWN
			}
			t.Logf("%s OTG member ports of LAG %s", action, aggID)
			OTGInterfaceFlap(t, ate, ate.Port(t, "port1"), state)
		}
		performLAGFlapTest(t, dut, aggID, dutAggPorts, "OTGLAGFlap", flapFunc)
	})
}

// configureInterface configures a single Ethernet interface on the DUT with IPv4 and IPv6 addresses.
func configureInterface(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port) {
	t.Helper()
	i := &oc.Interface{
		Name:        ygot.String(port.Name()),
		Description: ygot.String(fmt.Sprintf("Description for %s", port.Name())),
		Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
	}
	i.GetOrCreateEthernet()
	i.SetEnabled(true)

	s := i.GetOrCreateSubinterface(0)
	s.SetEnabled(true)

	v4 := s.GetOrCreateIpv4()
	if !deviations.IPv4MissingEnabled(dut) {
		v4.SetEnabled(true)
	}
	a4 := v4.GetOrCreateAddress(dutSrc.IPv4)
	a4.SetPrefixLength(dutSrc.IPv4Len)

	v6 := s.GetOrCreateIpv6()
	v6.SetEnabled(true)
	a6 := v6.GetOrCreateAddress(dutSrc.IPv6)
	a6.SetPrefixLength(dutSrc.IPv6Len)

	gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
}

// OTGInterfaceFlap sets the link state of an OTG port.
func OTGInterfaceFlap(t *testing.T, ate *ondatra.ATEDevice, port *ondatra.Port, state gosnappi.StatePortLinkStateEnum) {
	portStateAction := gosnappi.NewControlState()
	portStateAction.Port().Link().SetPortNames([]string{port.ID()}).SetState(state)
	ate.OTG().SetControlState(t, portStateAction)
}

// configureOTG configures a single Ethernet interface on the ATE.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice, port *ondatra.Port, ateAttr, dutAttr attrs.Attributes) {
	t.Helper()

	top := gosnappi.NewConfig()
	ateAttr.AddToOTG(top, port, &dutAttr)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	OTGInterfaceFlap(t, ate, port, gosnappi.StatePortLinkState.UP)
}

// TestEthernetInterfaceLastChangeState verifies that the last-change timestamp is updated on interface flap
// for a physical Ethernet interface and its subinterface.
func TestEthernetInterfaceLastChangeState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	port := dut.Port(t, "port1")

	configureInterface(t, dut, port)
	intfPath := gnmi.OC().Interface(port.Name())

	// Perform a series of state changes (disable then enable) to check for timestamp updates.
	numStateChanges := 2 * flapCount

	// performFlapTest is a helper that flaps an interface multiple times and validates that
	// the last-change timestamp is updated at each flap.
	performFlapTest := func(t *testing.T, testName string, flapFunc func(t *testing.T, enabled bool)) {
		t.Helper()
		// Ensure the interface is UP before starting the test.
		gnmi.Update(t, dut, intfPath.Enabled().Config(), true)
		gnmi.Await(t, dut, intfPath.OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_UP)

		// Get initial last-change values when the interface is UP.
		initialIntfLCVal, present := gnmi.Lookup(t, dut, intfPath.LastChange().State()).Val()
		if !present {
			t.Fatalf("[%s] Failed to lookup initial LastChange for interface %s", testName, port.Name())
		}
		initialSubintfLCVal, present := gnmi.Lookup(t, dut, intfPath.Subinterface(0).LastChange().State()).Val()
		if !present {
			t.Fatalf("[%s] Failed to lookup initial LastChange for subinterface %s:0", testName, port.Name())
		}
		t.Logf("[%s] Initial LastChange values: Interface %s: %d, Subinterface %s:0: %d", testName, port.Name(), initialIntfLCVal, port.Name(), initialSubintfLCVal)

		prevIntfLC := initialIntfLCVal
		prevSubintfLC := initialSubintfLCVal

		for i := 1; i <= numStateChanges; i++ {
			var action string
			var targetOperStatus oc.E_Interface_OperStatus
			var enabledState bool

			if i%2 != 0 { // For odd-numbered iterations, disable the interface.
				action = "Disable"
				enabledState = false
				targetOperStatus = oc.Interface_OperStatus_DOWN
			} else { // For even-numbered iterations, enable the interface.
				action = "Enable"
				enabledState = true
				targetOperStatus = oc.Interface_OperStatus_UP
			}

			t.Logf("[%s] Performing %s on interface %s, state change %d/%d", testName, action, port.Name(), i, numStateChanges)

			// Trigger the interface flap using the provided function.
			flapFunc(t, enabledState)

			gnmi.Await(t, dut, intfPath.OperStatus().State(), awaitTimeout, targetOperStatus)

			// Get last-change values after the state change.
			currentIntfLCVal, present := gnmi.Lookup(t, dut, intfPath.LastChange().State()).Val()
			if !present {
				t.Errorf("[%s] Failed to lookup LastChange for interface %s after %s (state change %d)", testName, port.Name(), action, i)
				continue
			}
			currentSubintfLCVal, present := gnmi.Lookup(t, dut, intfPath.Subinterface(0).LastChange().State()).Val()
			if !present {
				t.Errorf("[%s] Failed to lookup LastChange for subinterface %s:0 after %s (state change %d)", testName, port.Name(), action, i)
				continue
			}

			// Verify that last-change increased.
			subIntfName := fmt.Sprintf("%s:%d", port.Name(), 0)
			if currentIntfLCVal <= prevIntfLC {
				t.Errorf("[%s] State Change %d (%s): Interface %s LastChange timestamp did not increase, initial: %d, final: %d", testName, i, action, port.Name(), prevIntfLC, currentIntfLCVal)
			} else {
				t.Logf("[%s] State Change %d (%s): Interface %s LastChange timestamp increased as expected, initial: %d, final: %d", testName, i, action, port.Name(), prevIntfLC, currentIntfLCVal)
			}

			if currentSubintfLCVal <= prevSubintfLC {
				t.Errorf("[%s] State Change %d (%s): Subinterface %s LastChange timestamp did not increase, initial: %d, final: %d", testName, i, action, subIntfName, prevSubintfLC, currentSubintfLCVal)
			} else {
				t.Logf("[%s] State Change %d (%s): Subinterface %s LastChange timestamp increased as expected, initial: %d, final: %d", testName, i, action, subIntfName, prevSubintfLC, currentSubintfLCVal)
			}

			// Store current timestamps for the next iteration's comparison.
			prevIntfLC = currentIntfLCVal
			prevSubintfLC = currentSubintfLCVal
		}
	}

	t.Run("OTGInterfaceFlap", func(t *testing.T) {
		// Configure OTG for this test case.
		ate := ondatra.ATE(t, "ate")
		configureOTG(t, ate, ate.Port(t, "port1"), ateSrc, dutSrc)
		flapFunc := func(t *testing.T, enabled bool) {
			if enabled {
				OTGInterfaceFlap(t, ate, ate.Port(t, "port1"), gosnappi.StatePortLinkState.UP)
			} else {
				OTGInterfaceFlap(t, ate, ate.Port(t, "port1"), gosnappi.StatePortLinkState.DOWN)
			}
		}
		performFlapTest(t, "OTGInterfaceFlap", flapFunc)
	})

	t.Run("OCInterfaceFlap", func(t *testing.T) {
		flapFunc := func(t *testing.T, enabled bool) {
			gnmi.Update(t, dut, intfPath.Enabled().Config(), enabled)
		}
		performFlapTest(t, "OCInterfaceFlap", flapFunc)
	})
}
