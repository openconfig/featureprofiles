// Copyright 2024 Google LLC
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

package singleton_with_breakouts_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnps "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

const (
	maxRebootTime   = 900 // Seconds.
	maxCompWaitTime = 900
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Method to run baseLine test.
func baseLineTest(t *testing.T, dut *ondatra.DUTDevice, portsConfigured map[string]bool, rebootCheck bool) {
	if !rebootCheck {
		configureDUT(t, dut, portsConfigured)
		// Allow breakout configuration to take effect before state verification.
		time.Sleep(20 * time.Second)
	}
	verifyInterfaceHardwarePortAndBreakoutConfig(t, dut, portsConfigured)
}

// Method to configure DUT interfaces along with breakout configurations.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, portsConfigured map[string]bool) {
	topo := gnmi.OC()
	for _, port := range dut.Ports() {
		portsConfigured[port.Name()] = false
		if port.PMD() == ondatra.PMD400GBASEDR4 {
			numOfBreakouts := uint8(4)
			hardwarePort := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).HardwarePort().State())
			comp := &oc.Component{Name: &hardwarePort}
			bmode := comp.GetOrCreatePort().GetOrCreateBreakoutMode()
			group := bmode.GetOrCreateGroup(1)
			group.BreakoutSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
			group.NumBreakouts = ygot.Uint8(numOfBreakouts)
			gnmi.Replace(t, dut, topo.Component(hardwarePort).Config(), comp)
			t.Logf("Configured Port=%v with  PMD = %v with breakout configuration num_of_breakouts= %v , break_out_speed=%v ", port.Name(), port.PMD(), numOfBreakouts, group.BreakoutSpeed)
		} else {
			i := configureInterfaceDUT(t, dut, port)
			gnmi.Replace(t, dut, topo.Interface(port.Name()).Config(), i)
			t.Logf("Configured Port=%v with PMD =%v", port.Name(), port.PMD())
		}
	}
}

// Method to configure the interface.
func configureInterfaceDUT(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(port.Name())}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, port)
	}
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	return i
}

// Method to verify if hardwarePort is populated with a reference to componentName and breakout config is applied.
func verifyInterfaceHardwarePortAndBreakoutConfig(t *testing.T, dut *ondatra.DUTDevice, portsConfigured map[string]bool) {
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	for _, i := range interfaces {
		interfaceName := i.GetName()
		if i.HardwarePort == nil {
			continue
		}
		hardwarePort := i.GetHardwarePort()

		// Validate hardware-port leaf is not empty
		if hardwarePort == "" {
			t.Logf("WARNING: Interface %v has empty hardware-port value", interfaceName)
			continue
		}

		configuredInterfaceName := interfaceName
		_, isConfiguredInterface := portsConfigured[configuredInterfaceName]
		if !isConfiguredInterface {
			if idx := strings.Index(interfaceName, ":"); idx != -1 {
				configuredInterfaceName = interfaceName[:idx]
				_, isConfiguredInterface = portsConfigured[configuredInterfaceName]
			}
		}
		if !isConfiguredInterface {
			continue
		}

		compNameLookup := gnmi.Lookup(t, dut, gnmi.OC().Component(hardwarePort).Name().State())
		compName, present := compNameLookup.Val()
		if !present {
			t.Fatalf("Failed to fetch component name for the hardware port %v for interface %v", hardwarePort, interfaceName)
		}
		if compName == "" {
			t.Fatalf("Component name for hardware-port %v of interface %v is empty", hardwarePort, interfaceName)
		}
		if hardwarePort != compName {
			t.Fatalf("Hardware-port value %v does not match component name %v for interface %v", hardwarePort, compName, interfaceName)
		}
		portsConfigured[configuredInterfaceName] = true
		t.Logf("✓ Interface %v (configured port %v): hardware-port=%v correctly references component name=%v", interfaceName, configuredInterfaceName, hardwarePort, compName)
	}

	for interfaceName, referenceChecked := range portsConfigured {
		if !referenceChecked {
			t.Fatalf("Interface %v not found in the fetched Interface list", interfaceName)
		}
		portsConfigured[interfaceName] = false
	}

	for _, port := range dut.Ports() {
		if port.PMD() != ondatra.PMD400GBASEDR4 {
			continue
		}

		portName := port.Name()
		t.Logf("Verifying breakout configuration for port: %v (PMD: %v)", portName, port.PMD())

		// Use Lookup to avoid fatal when the leaf is not present on parent interface.
		hpLookup := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).HardwarePort().State())
		hardwarePort, present := hpLookup.Val()
		if !present || hardwarePort == "" {
			// For breakout ports the parent interface may not expose hardware-port.
			// Use a child breakout interface as the representative hardware-port.
			interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
			for _, iface := range interfaces {
				if strings.HasPrefix(iface.GetName(), portName+":") && iface.HardwarePort != nil {
					childHP := iface.GetHardwarePort()
					if childHP != "" {
						hardwarePort = childHP
						break
					}
				}
			}
		}
		if hardwarePort == "" {
			t.Fatalf("Hardware-port for interface %v is empty", portName)
		}

		// Read the component config since breakout-mode is applied via State.
		comp := gnmi.Get(t, dut, gnmi.OC().Component(hardwarePort).State())

		if comp == nil {
			t.Fatalf("Failed to retrieve component for hardware-port: %v", hardwarePort)
		}

		portComp := comp.GetPort()
		if portComp == nil {
			t.Fatalf("Component %v is not a port component or missing port container", hardwarePort)
		}

		bmode := portComp.GetBreakoutMode()
		if bmode == nil {
			t.Fatalf("Breakout-mode configuration missing for component %v (hardware-port of %v)", hardwarePort, portName)
		}

		groupIdx := uint8(1)
		group := bmode.GetGroup(groupIdx)
		if group == nil {
			t.Fatalf("Breakout group %v configuration missing for component %v", groupIdx, hardwarePort)
		}

		numBreakouts := group.GetNumBreakouts()
		if numBreakouts == 0 {
			t.Fatalf("Breakout num-breakouts not set for group %v in component %v", groupIdx, hardwarePort)
		}

		breakoutSpeed := group.GetBreakoutSpeed()
		if breakoutSpeed == oc.IfEthernet_ETHERNET_SPEED_UNSET {
			t.Fatalf("Breakout speed not set for group %v in component %v", groupIdx, hardwarePort)
		}

		t.Logf("✓ Component %v (port %v): Breakout Group %v validated - num-breakouts=%v, speed=%v",
			hardwarePort, portName, groupIdx, numBreakouts, breakoutSpeed)
	}
}

// Method to reboot the DUT.
func rebootDUT(t *testing.T, dut *ondatra.DUTDevice) {
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect to gnoi server, err: %v", err)
	}
	rebootRequest := &gnps.RebootRequest{
		Method: gnps.RebootMethod_COLD,
		Force:  true,
	}
	preRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
	preRebootCompDebug := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	preCompMatrix := []string{}
	for _, preComp := range preRebootCompDebug {
		if preComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
			preCompMatrix = append(preCompMatrix, preComp.GetName()+":"+preComp.GetOperStatus().String())
		}
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	var currentTime string
	currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	t.Logf("Time Before Reboot : %v", currentTime)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	t.Logf("Got Reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}
	for {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Log("Reboot is started")
			break
		}
		t.Log("Wait for reboot to be started")
		time.Sleep(30 * time.Second)
	}
	startReboot := time.Now()
	t.Logf("Waiting for DUT to boot up by polling the telemetry output.")
	for {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg == nil {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}
		if uint64(time.Since(startReboot).Seconds()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	startComp := time.Now()
	for {
		postRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
		postRebootCompDebug := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
		postCompMatrix := []string{}
		for _, postComp := range postRebootCompDebug {
			if postComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
				postCompMatrix = append(postCompMatrix, postComp.GetName()+":"+postComp.GetOperStatus().String())
			}
		}
		if len(preRebootCompStatus) == len(postRebootCompStatus) {
			if rebootDiff := cmp.Diff(preCompMatrix, postCompMatrix); rebootDiff != "" {
				t.Logf("All components on the DUT are in responsive state")
			}
			time.Sleep(10 * time.Second)
			break
		}
		if uint64(time.Since(startComp).Seconds()) > maxCompWaitTime {
			t.Logf("DUT components status post reboot: %v", postRebootCompStatus)
			if rebootDiff := cmp.Diff(preCompMatrix, postCompMatrix); rebootDiff != "" {
				t.Logf("[DEBUG] Unexpected diff after reboot (-component missing from pre reboot, +component added from pre reboot): %v ", rebootDiff)
			}
			t.Fatalf("There's a difference in components obtained in pre reboot: %v and post reboot: %v.", len(preRebootCompStatus), len(postRebootCompStatus))
		}
		time.Sleep(10 * time.Second)
	}
}
func TestSingletonWithBreakouts(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	portsConfigured := make(map[string]bool)
	t.Run("RT-8.1 - Baseline test", func(tb *testing.T) {
		baseLineTest(tb, dut, portsConfigured, false)
	})
	t.Run("RT-8.2 - Reboot test", func(tb *testing.T) {
		rebootDUT(tb, dut)
		baseLineTest(tb, dut, portsConfigured, true)
	})
}
