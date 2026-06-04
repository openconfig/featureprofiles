// Copyright 2026 Google LLC
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

// Package union_replace_test implements tests that cover the gnmi union_replace
// operations.
package union_replace_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {

	fptest.RunTests(m)
}

const (
	awaitTimeOut = 10 * time.Second
)

var (
	dutIntf = attrs.Attributes{
		Desc:    "unionreplacetest",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: 30,
		IPv6Len: 64,
		Duplex:  "FULL",
	}
)

var portSpeed = map[ondatra.Speed]oc.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed10Gb:  oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
}

func configOCInterface(t *testing.T, sb *gnmi.SetBatch, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	i := dutIntf.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	if deviations.ExplicitPortSpeed(dut) {
		i.GetOrCreateEthernet().SetPortSpeed(fptest.GetIfSpeed(t, dp1))
	}
	inf := gnmi.OC().Interface(dp1.Name())
	// TODO: add handling for ExplicitPortSpeed deviation and ExplicitInterfaceInDefaultVRF deviation

	gnmi.BatchUnionReplace(sb, inf.Config(), i)
}

// prettyPrintYgnmiResult formats a *ygnmi.Result as JSON for logging.
// Note: ygnmi.Result contains a protobuf (SetResponse) rather than YANG data,
// so it is formatted as standard JSON via protojson rather than RFC7951.
func prettyPrintYgnmiResult(setResult *ygnmi.Result) string {
	if setResult == nil || setResult.RawResponse == nil {
		return ""
	}
	opts := protojson.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}
	b, err := opts.Marshal(setResult.RawResponse)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func setCLINoMTU(t *testing.T, dut *ondatra.DUTDevice, portName string) {
	t.Helper()
	cliConfig := ""
	if dut.Vendor() == ondatra.ARISTA {
		cliConfig = fmt.Sprintf("configure terminal\ninterface %s\nno mtu\n", portName)
	} else {
		t.Fatalf("Unsupported vendor: %v", dut.Vendor())
	}
	helpers.GnmiCLIConfig(t, dut, cliConfig)
	// Wait for the MTU to be removed (i.e., not equal to 1500).
	gnmi.Watch(t, dut, gnmi.OC().Interface(portName).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
		m, present := val.Val()
		if !present {
			t.Logf("Got MTU not present, want 1500.")
			return false
		}
		if m == 1500 {
			return true
		}
		return false
	}).Await(t)
}

// setCLIunionReplace adds any necessary modifications to the base CLI configuration
// for union replace.
func setCLIunionReplace(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	clicfg1 := ""

	if dut.Vendor() == ondatra.ARISTA {
		// Add  "operation set namespace" command from the CLI config as this is required for Arista
		// when using union replace.
		clicfg1 = "configure terminal\nmanagement api gnmi\n  provider eos-native\n  operation set persistence\n  operation set namespace\n"
		helpers.GnmiCLIConfig(t, dut, clicfg1)
		// Poll for config to be applied.
		start := time.Now()
		for {
			if time.Since(start) > 1*time.Minute {
				t.Fatal("setCLIunionReplace config not applied in time")
			}
			clicfg2 := cliConfig(t, dut)
			if strings.Contains(clicfg2, "operation set namespace") && strings.Contains(clicfg2, "operation set persistence") {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

}

// cliConfig returns the CLI config of the DUT as a string
func cliConfig(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()

	switch dut.Vendor() {
	case ondatra.ARISTA:
		return helpers.RunCliCommand(t, dut, "show running-config")
	case ondatra.CISCO:
		return helpers.RunCliCommand(t, dut, "show running-config")
	case ondatra.JUNIPER:
		return helpers.RunCliCommand(t, dut, "show | display set")
	case ondatra.NOKIA:
		return helpers.RunCliCommand(t, dut, "info | as-set")
	default:
		t.Errorf("Unsupported vendor: %v", dut.Vendor())
	}

	return ""
}

// TestUnionReplace3_1_idempotentConfig verifies the gNMI UnionReplace operation with CLI config only.
// gNMI-3.1 - Idempotent configuration
func TestUnionReplace3_1_idempotentConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// ensure union replace is enabled on the DUT and get the CLI config using union replace after setting
	// the base CLI config.
	setCLIunionReplace(t, dut)
	clicfg1 := cliConfig(t, dut)
	sb1 := &gnmi.SetBatch{}
	gnmi.BatchUnionReplaceCLI(sb1, "cli", clicfg1)
	sb1.Set(t, dut)
	time.Sleep(5 * time.Second)
	clicfg2 := helpers.RunCliCommand(t, dut, "show running-config")

	// second, set the same CLI config again.
	sb2 := &gnmi.SetBatch{}
	gnmi.BatchUnionReplaceCLI(sb2, "cli", clicfg2)
	sb2.Set(t, dut)
	time.Sleep(5 * time.Second)

	// verify the CLI config has not changed.
	clicfg3 := helpers.RunCliCommand(t, dut, "show running-config")
	if clicfg2 != clicfg3 {
		t.Errorf("cliConfig before and after do not match!")
	}
}

// TestUnionReplace3_2_1_addOCMTU verifies the gNMI UnionReplace with CLI for a base config and
// adds MTU of an interface using OC only.  This tests that the MTU leaf is updated even though the
// interface already exists in the CLI config.  It assumes that the CLI config does not contain an MTU
// value for the interface.
// gNMI-3.2.1 - Add interface using OC
func TestUnionReplace3_2_1_addOCMTU(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	sb1 := &gnmi.SetBatch{}

	setCLIunionReplace(t, dut)
	setCLINoMTU(t, dut, dp1.Name())

	// Add MTU to the interface using OC config.
	cliConfig2 := cliConfig(t, dut)
	gnmi.BatchUnionReplaceCLI(sb1, "cli", cliConfig2)
	gnmi.BatchUnionReplace(sb1, gnmi.OC().Interface(dp1.Name()).Mtu().Config(), 1400)
	t.Logf("Generated BatchUnionReplace: %#v\n", sb1.String())

	setResult := sb1.Set(t, dut)
	t.Logf("\nSetResult: %#v\n", prettyPrintYgnmiResult(setResult))

	want := uint16(1400)
	gnmi.Watch(t, dut, gnmi.OC().Interface(dp1.Name()).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
		m, present := val.Val()
		if !present {
			t.Errorf("MTU not present")
			return false
		}
		if m != want {
			t.Errorf("MTU not correct, got: %v, want: %v", m, want)
			return false
		}
		return true
	}).Await(t)

}

// TestUnionReplace3_2_2_addCLIInterface verifies the gNMI UnionReplace with CLI for a base config
// and configures MTU of an interface using CLI and OC, therefore mixing interfaces with CLI and OC.
// gNMI-3.2.2 - Add interface using CLI
func TestUnionReplace3_2_2_addCLIMTU(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp2 := dut.Port(t, "port2")
	sb1 := &gnmi.SetBatch{}
	sb2 := &gnmi.SetBatch{}

	// ensure that the device has union_replace enabled and the base CLI config does not contain an MTU
	// config for the interface being tested.
	setCLIunionReplace(t, dut)
	setCLINoMTU(t, dut, dp2.Name())

	// Add MTU to the interface using OC config to a known value.
	cliConfig1 := cliConfig(t, dut)
	gnmi.BatchUnionReplaceCLI(sb1, "cli", cliConfig1)
	gnmi.BatchUnionReplace(sb1, gnmi.OC().Interface(dp2.Name()).Mtu().Config(), 1400)
	sb1.Set(t, dut)
	gnmi.Watch(t, dut, gnmi.OC().Interface(dp2.Name()).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
		m, present := val.Val()
		if !present {
			t.Errorf("MTU not present")
			return false
		}
		if m != 1400 {
			t.Logf("MTU not yet 1400, got: %v", m)
			return false
		}
		return true
	}).Await(t)

	// Change the MTU to a different value using CLI config.
	cliConfig2 := cliConfig(t, dut)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cliConfig2 += fmt.Sprintf("interface %s\nmtu 1300\n", dp2.Name())
	case ondatra.CISCO:
		cliConfig2 += fmt.Sprintf("interface %s\nmtu 1300\n", dp2.Name())
	case ondatra.JUNIPER:
		cliConfig2 += fmt.Sprintf("set interfaces %s mtu 1300\n", dp2.Name())
	default:
		t.Errorf("Unsupported vendor: %v", dut.Vendor())
	}
	gnmi.BatchUnionReplaceCLI(sb2, "cli", cliConfig2)
	setResult := sb2.Set(t, dut)
	t.Logf("\nSetResult: %#v\n", prettyPrintYgnmiResult(setResult))

	// If union_replace option for CLI overriding OC is the DUT behavior, verify the MTU is updated
	// to the new, CLI configured value. If union_replace option for CLI and OC config error is the
	// DUT behavior, verify the MTU is not updated to the new, CLI configured value.
	switch dut.Vendor() {
	case ondatra.ARISTA:
		// CLI overrides OC
		want := uint16(1300)
		gnmi.Watch(t, dut, gnmi.OC().Interface(dp2.Name()).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
			m, present := val.Val()
			if !present {
				t.Logf("MTU not present yet")
				return false
			}
			if m != want {
				t.Logf("MTU not yet %d, got: %v", want, m)
				return false
			}
			return true
		}).Await(t)
	case ondatra.CISCO:
		// OC and CLI conflict generates an error, does not update MTU
		want := uint16(1400)
		gnmi.Watch(t, dut, gnmi.OC().Interface(dp2.Name()).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
			m, present := val.Val()
			if !present {
				t.Logf("MTU not present yet")
				return false
			}
			if m != want {
				t.Logf("MTU not yet %d, got: %v", want, m)
				return false
			}
			return true
		}).Await(t)
	case ondatra.JUNIPER:
		// OC and CLI conflict generates an error, does not update MTU
		want := uint16(1400)
		gnmi.Watch(t, dut, gnmi.OC().Interface(dp2.Name()).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
			m, present := val.Val()
			if !present {
				t.Logf("MTU not present yet")
				return false
			}
			if m != want {
				t.Logf("MTU not yet %d, got: %v", want, m)
				return false
			}
			return true
		}).Await(t)
	case ondatra.NOKIA:
		// OC and CLI conflict generates an error, does not update MTU
		want := uint16(1400)
		gnmi.Watch(t, dut, gnmi.OC().Interface(dp2.Name()).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
			m, present := val.Val()
			if !present {
				t.Logf("MTU not present yet")
				return false
			}
			if m != want {
				t.Logf("MTU not yet %d, got: %v", want, m)
				return false
			}
			return true
		}).Await(t)
	default:
		t.Errorf("Unsupported vendor: %v", dut.Vendor())
	}

}

// TestUnionReplace3_3_1_changeOCConfig verifies the gNMI UnionReplace with CLI for a base config and
// changes an OC config.
// gNMI-3.3.1 - Change OC config
func TestUnionReplace3_3_1_changeOCConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setCLIunionReplace(t, dut)
	portName := dut.Port(t, "port2").Name()
	sb := &gnmi.SetBatch{}

	// reset the CLI config for the port to remove any previous MTU config.
	setCLINoMTU(t, dut, portName)

	// Add MTU to the interface using OC config.
	cliConfig1 := cliConfig(t, dut)
	gnmi.BatchUnionReplaceCLI(sb, "cli", cliConfig1)
	gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(portName).Mtu().Config(), 1450)
	sb.Set(t, dut)

	want1 := uint16(1450)
	gnmi.Watch(t, dut, gnmi.OC().Interface(portName).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
		m, present := val.Val()
		if !present {
			t.Logf("MTU not present yet")
			return false
		}
		if m != want1 {
			t.Logf("MTU not yet %v, got: %v", want1, m)
			return false
		}
		return true
	}).Await(t)

	// Change the MTU using OC config.
	// reuse the same CLI config without any MTU config.
	sb2 := &gnmi.SetBatch{}
	gnmi.BatchUnionReplace(sb2, gnmi.OC().Interface(portName).Mtu().Config(), 1440)
	gnmi.BatchUnionReplaceCLI(sb2, "cli", cliConfig1)
	sb2.Set(t, dut)

	want2 := uint16(1440)
	gnmi.Watch(t, dut, gnmi.OC().Interface(portName).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
		m, present := val.Val()
		if !present {
			t.Logf("MTU not present yet")
			return false
		}
		if m != want2 {
			t.Logf("MTU not yet %v, got: %v", want2, m)
			return false
		}
		return true
	}).Await(t)
}

// TestUnionReplace3_3_2_changeCLIConfig verifies the gNMI UnionReplace with CLI for a base config and
// changes a CLI config.
// gNMI-3.3.2 - Change CLI config
func TestUnionReplace3_3_2_changeCLIConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setCLIunionReplace(t, dut)
	port1Name := dut.Port(t, "port1").Name()
	port1DescriptionOC := "unionreplacetest gnmi-3.3.2 OC"
	port1DescriptionCLI := "unionreplacetest gnmi-3.3.2 CLI"
	sb1 := &gnmi.SetBatch{}

	// Set the interface description to a known value using OC config.
	// Add OC interface and set description on the interface.
	gnmi.BatchUnionReplace(sb1, gnmi.OC().Interface(port1Name).Description().Config(), port1DescriptionOC)
	cliConfig1 := cliConfig(t, dut)
	gnmi.BatchUnionReplaceCLI(sb1, "cli", cliConfig1)
	sb1.Set(t, dut)
	gnmi.Watch(t, dut, gnmi.OC().Interface(port1Name).Description().State(), awaitTimeOut, func(val *ygnmi.Value[string]) bool {
		desc, present := val.Val()
		if !present {
			t.Logf("Description not present. Want: %q, got: not present", port1DescriptionOC)
			return false
		}
		if desc != port1DescriptionOC {
			t.Logf("Description not set to OC configured value. Want: %q, got: %q", port1DescriptionOC, desc)
			return false
		}
		return true
	}).Await(t)

	// Change the interface description using CLI config.
	// the OC configuration does not include an interface description.
	sb2 := &gnmi.SetBatch{}
	cliConfig2 := cliConfig(t, dut)
	cliConfig2 += fmt.Sprintf("interface %s\ndescription "+port1DescriptionCLI+"\n", dut.Port(t, "port1").Name())
	gnmi.BatchUnionReplaceCLI(sb2, "cli", cliConfig2)
	sb2.Set(t, dut)

	// Watch for the description to be updated to the CLI configured value.
	gnmi.Watch(t, dut, gnmi.OC().Interface(port1Name).Description().State(), awaitTimeOut, func(val *ygnmi.Value[string]) bool {
		desc, present := val.Val()
		if !present {
			t.Logf("Description not present. Want: %q, got: not present", port1DescriptionCLI)
			return false
		}
		if desc != port1DescriptionCLI {
			t.Logf("Description does not match the CLI configured value.  want: %q, got: %q", port1DescriptionCLI, desc)
			return false
		}
		return true // Description is now port1DescriptionCLI
	}).Await(t)
}

// TestUnionReplace3_6_1 tests the gNMI union_replace accepted with hardware mismatch.
// load the cli config from DUT
// generate OC config for 1 DUT 100Gbps port but set port speed to 10Gbps (intentionally mismatch)
// build the union replace request with the cli config and OC config
// send the request to the DUT
// verify the DUT OC config contains the port speed of 10Gbps
// verify the DUT OC /interfaces/interface/state/oper-status is DOWN
func TestUnionReplace3_6_1(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setCLIunionReplace(t, dut)
	sb := &gnmi.SetBatch{}
	targetSpeed := oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB

	// confirm the testbed defined and DUT reported port speed are not the target speed.
	dp1 := dut.Port(t, "port1")
	speedCurrent := portSpeed[dp1.Speed()]
	t.Logf("DUT %v port speed defined in the testbed is %v", dp1.Name(), speedCurrent)
	beforeSpeed := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().State())
	t.Logf("DUT reported PortSpeed state before any changes: %v", beforeSpeed)

	if speedCurrent == targetSpeed {
		t.Fatalf("Need a different topology for this test. DUT port %q current port speed must not be %q", dp1.Name(), targetSpeed)
	}

	t.Logf("Configuring DUT port %q to mismatched port-speed %q using gNMI union_replace.", dp1.Name(), targetSpeed)
	// get the cli config from DUT and add it to the SetBatch.
	clicfg1 := cliConfig(t, dut)
	gnmi.BatchUnionReplaceCLI(sb, "cli", clicfg1)
	/*
			These Arista EOS CLI commands would allow EOS to accept the port speed mismatch but are not
			included as they are not accepted as a deviation.
			system l1
		      unsupported speed action warn
	*/

	// add configuration of the OC interface to the SetBatch
	configOCInterface(t, sb, dut)
	gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().Config(), targetSpeed)
	gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(dp1.Name()).Ethernet().DuplexMode().Config(), oc.Ethernet_DuplexMode_FULL)
	t.Logf("Generated BatchUnionReplace: %#v\n", sb.String())

	// send the request to the DUT.
	setResult := sb.Set(t, dut)
	t.Logf("SetResult:\n%s", prettyPrintYgnmiResult(setResult))

	// Verify the port speed CONFIG leaf is the before speed.  It is expected that the port speed config
	// leaf is updated to the target speed.
	gnmi.Watch(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().Config(), awaitTimeOut, func(val *ygnmi.Value[oc.E_IfEthernet_ETHERNET_SPEED]) bool {
		speed, present := val.Val()
		if !present {
			t.Logf("PortSpeed config not present. Want: %v, got: not present", targetSpeed)
			return false
		}
		if speed != targetSpeed {
			t.Logf("PortSpeed config not set to target speed. Want: %v, got: %v", targetSpeed, speed)
			return false
		}
		t.Logf("PortSpeed config is set to target speed: %v", speed)
		return true
	}).Await(t)

	// Verify the port speed state leaf is the beforeSpeed or UNKNOWN.   It is expected that the
	// PortSpeed state leaf was not affected by the new configuration and reflects the actual
	// operating speed of the port.
	foundSpeed := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().State())
	if foundSpeed != beforeSpeed && foundSpeed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
		t.Errorf("DUT port1 PortSpeed state: got %v, want %v or unknown", foundSpeed, beforeSpeed)
	}

	want := oc.Interface_OperStatus_DOWN
	gnmi.Watch(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), awaitTimeOut, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := val.Val()
		if !present {
			t.Logf("OperStatus not present yet")
			return false
		}
		if status != want {
			t.Logf("OperStatus not in expected state.  Want: %v, got: %v", want, status)
			return false
		}
		t.Logf("OperStatus is in expected state: %v", status)
		return true
	}).Await(t)

}

// TestUnionReplace3_6_2 tests the gNMI union_replace accepted with hardware mismatch using CLI.
// load the cli config from DUT
// generate CLI config for 1 DUT 100Gbps port but set port speed to 10Gbps (intentionally mismatch)
// build the union replace request with the cli config and OC config
// send the request to the DUT
// verify the DUT OC config contains the port speed of 10Gbps
// verify the DUT OC /interfaces/interface/state/oper-status is DOWN
func TestUnionReplace3_6_2(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setCLIunionReplace(t, dut)
	sb := &gnmi.SetBatch{}
	targetSpeed := oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB

	// confirm the testbed defined and DUT reported port speed are not the target speed.
	dp1 := dut.Port(t, "port1")
	speedCurrent := portSpeed[dp1.Speed()]
	t.Logf("DUT %v port speed defined in the testbed is %v", dp1.Name(), speedCurrent)
	beforeSpeed := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().State())
	t.Logf("DUT reported PortSpeed state before any changes: %v", beforeSpeed)

	if speedCurrent == targetSpeed {
		t.Fatalf("Need a different topology for this test. DUT port %q current port speed must not be %q", dp1.Name(), targetSpeed)
	}

	t.Logf("Configuring DUT port %q to mismatched port-speed %q using gNMI union_replace CLI.", dp1.Name(), targetSpeed)
	// get the cli config from DUT, modify it to introduce the port speed mismatch, and add it to the SetBatch.
	clicfg1 := cliConfig(t, dut)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		clicfg1 += fmt.Sprintf("interface %s\nspeed 10g\n", dp1.Name())
	case ondatra.CISCO:
		clicfg1 += fmt.Sprintf("interface %s\nspeed 10000\n", dp1.Name())
	case ondatra.JUNIPER:
		clicfg1 += fmt.Sprintf("set interfaces %s speed 10g\n", dp1.Name())
	default:
		t.Errorf("Unsupported vendor: %v", dut.Vendor())
	}
	gnmi.BatchUnionReplaceCLI(sb, "cli", clicfg1)
	/*
			These Arista EOS CLI commands would allow EOS to accept the port speed mismatch but are not
			included as they are not accepted as a deviation.
			system l1
		   unsupported speed action warn
	*/

	// add configuration of the OC interface to the SetBatch
	configOCInterface(t, sb, dut)
	gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(dp1.Name()).Ethernet().DuplexMode().Config(), oc.Ethernet_DuplexMode_FULL)
	t.Logf("Generated BatchUnionReplace: %#v\n", sb.String())

	// send the request to the DUT.
	setResult := sb.Set(t, dut)
	t.Logf("SetResult:\n%s", prettyPrintYgnmiResult(setResult))

	// Verify the port speed CONFIG leaf is the before speed.  It is expected that the port speed config
	// leaf is updated to the target speed.
	gnmi.Watch(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().Config(), awaitTimeOut, func(val *ygnmi.Value[oc.E_IfEthernet_ETHERNET_SPEED]) bool {
		speed, present := val.Val()
		if !present {
			t.Logf("PortSpeed config not present. Want: %v, got: not present", targetSpeed)
			return false
		}
		if speed != targetSpeed {
			t.Logf("PortSpeed config not set to target speed. Want: %v, got: %v", targetSpeed, speed)
			return false
		}
		t.Logf("PortSpeed config is set to target speed: %v", speed)
		return true
	}).Await(t)

	// Verify the port speed state leaf is the beforeSpeed or UNKNOWN.   It is expected that the
	// PortSpeed state leaf was not affected by the new configuration and reflects the actual
	// operating speed of the port.
	foundSpeed := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().State())
	if foundSpeed != beforeSpeed && foundSpeed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
		t.Errorf("DUT port1 PortSpeed state: got %v, want %v or unknown", foundSpeed, beforeSpeed)
	}

	want := oc.Interface_OperStatus_DOWN
	gnmi.Watch(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), awaitTimeOut, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := val.Val()
		if !present {
			t.Logf("OperStatus not present yet")
			return false
		}
		if status != want {
			t.Logf("OperStatus not in expected state.  Want: %v, got: %v", want, status)
			return false
		}
		t.Logf("OperStatus is in expected state: %v", status)
		return true
	}).Await(t)

}
