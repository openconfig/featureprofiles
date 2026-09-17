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
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	maxRebootTime   = 900 // Seconds.
	maxCompWaitTime = 900
)

type breakoutInfo struct {
	numBreakouts    uint8
	speed           oc.E_IfEthernet_ETHERNET_SPEED
	numPhysChannels uint8
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Method to run baseLine test.
func baseLineTest(t *testing.T, dut *ondatra.DUTDevice, rebootCheck bool) {
	var breakoutPorts map[string]breakoutInfo

	if !rebootCheck {
		breakoutPorts = configureDUT(t, dut)
	} else {
		breakoutPorts = breakoutPortsInfo(t, dut)
	}

	expectedInterfaceCount := buildExpectedCounts(t, dut, breakoutPorts)

	// Step 2: Discover and verify interfaces (Subscribe)
	discoveredInterfaces := verifyAndCollectInterfaces(t, dut, expectedInterfaceCount)

	if !rebootCheck {
		// Step 3: Configure breakout interfaces
		mustConfigureBreakoutInterfaces(t, dut, breakoutPorts, discoveredInterfaces)

		// Verify again after configuration
		verifyAndCollectInterfaces(t, dut, expectedInterfaceCount)
	}
}

// Method to configure DUT interfaces along with breakout configurations.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) map[string]breakoutInfo {
	topo := gnmi.OC()
	breakoutPorts := make(map[string]breakoutInfo)

	for _, port := range dut.Ports() {
		hardwarePort := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).HardwarePort().State())
		numBreakouts, speed, numPhysChannels := breakoutConfig(t, dut, port)
		if numBreakouts > 0 {
			comp := &oc.Component{Name: &hardwarePort}
			bmode := comp.GetOrCreatePort().GetOrCreateBreakoutMode()
			group := bmode.GetOrCreateGroup(1)
			group.Index = ygot.Uint8(1)
			group.BreakoutSpeed = speed
			group.NumBreakouts = ygot.Uint8(numBreakouts)
			if !deviations.NumPhysyicalChannelsUnsupported(dut) {
				group.NumPhysicalChannels = ygot.Uint8(numPhysChannels)
			}
			gnmi.Replace(t, dut, topo.Component(hardwarePort).Config(), comp)
			t.Logf("Configured Port=%v with PMD=%v, Speed=%v with breakout configuration num_of_breakouts=%v, break_out_speed=%v", port.Name(), port.PMD(), port.Speed(), numBreakouts, speed)

			breakoutPorts[hardwarePort] = breakoutInfo{
				numBreakouts:    numBreakouts,
				speed:           speed,
				numPhysChannels: numPhysChannels,
			}
		} else {
			i := configureInterfaceDUT(t, dut, port)
			gnmi.Replace(t, dut, topo.Interface(port.Name()).Config(), i)
			t.Logf("Configured Port=%v with PMD =%v", port.Name(), port.PMD())
		}
	}
	return breakoutPorts
}

func breakoutPortsInfo(t *testing.T, dut *ondatra.DUTDevice) map[string]breakoutInfo {
	breakoutPorts := make(map[string]breakoutInfo)
	for _, port := range dut.Ports() {
		hardwarePort := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).HardwarePort().State())
		numBreakouts, speed, numPhysChannels := breakoutConfig(t, dut, port)
		if numBreakouts > 0 {
			breakoutPorts[hardwarePort] = breakoutInfo{
				numBreakouts:    numBreakouts,
				speed:           speed,
				numPhysChannels: numPhysChannels,
			}
		}
	}
	return breakoutPorts
}

func buildExpectedCounts(t *testing.T, dut *ondatra.DUTDevice, breakoutPorts map[string]breakoutInfo) map[string]int {
	expectedCounts := make(map[string]int)
	for _, port := range dut.Ports() {
		hardwarePort := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).HardwarePort().State())
		if info, isBreakout := breakoutPorts[hardwarePort]; isBreakout {
			expectedCounts[hardwarePort] = int(info.numBreakouts)
		} else {
			expectedCounts[hardwarePort] = 1
		}
	}
	return expectedCounts
}

// breakoutConfig determines the breakout parameters (number of channels, channel speed, index)
// for a given physical port.
//
// Note: openconfig-transport-types.yang v1.5.0 (merged in openconfig/public PR #1505) defines
// the OpenConfig PMD identities ETH_800GBASE_2XDR4 (800GBASE-2xDR4), ETH_800GBASE_2XLR4
// (800GBASE-2xLR4), and ETH_800GBASE_2XPLR4 (800GBASE-2xPLR4), alongside ETH_800GBASE_2XFR4
// (800GBASE_2XFR4).
//
// TODO: Once github.com/openconfig/ondatra releases a version that imports
// openconfig-transport-types v1.5.0 and exposes the corresponding ondatra.PMD enum constants,
// and openconfig/featureprofiles updates go.mod to import that release, add the direct
// ondatra.PMD enum constants alongside the OpenConfig PMD identity/description checks below.
func breakoutConfig(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port) (uint8, oc.E_IfEthernet_ETHERNET_SPEED, uint8) {
	// Early exit for unambiguous Ondatra PMD enum values.
	switch port.PMD() {
	case ondatra.PMD400GBASEDR4:
		return 4, oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB, 1
	case ondatra.PMD400GBASEFR4, ondatra.PMD100GBASELR4, ondatra.PMD100GBASEFR:
		return 0, oc.IfEthernet_ETHERNET_SPEED_UNSET, 0
	}

	// Early exit for OpenConfig PMD identities and standard names.
	switch strings.ToUpper(port.PMD().String()) {
	case "PMD_400GBASE_DR4", "ETH_400GBASE_DR4", "400GBASE_DR4", "400GBASE-DR4":
		return 4, oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB, 1
	case "PMD_800GBASE_2XPLR4", "ETH_800GBASE_2XPLR4", "800GBASE-2XPLR4", "800GBASE_2XPLR4",
		"PMD_800GBASE_2XDR4", "ETH_800GBASE_2XDR4", "800GBASE-2XDR4", "800GBASE_2XDR4":
		return 8, oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB, 1
	case "PMD_800GBASE_2XFR4", "ETH_800GBASE_2XFR4", "800GBASE_2XFR4", "800GBASE-2XFR4",
		"PMD_800GBASE_2XLR4", "ETH_800GBASE_2XLR4", "800GBASE-2XLR4", "800GBASE_2XLR4":
		return 2, oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB, 4
	case "PMD_400GBASE_FR4", "ETH_400GBASE_FR4", "400GBASE_FR4", "400GBASE-FR4",
		"PMD_100GBASE_LR4", "ETH_100GBASE_LR4", "100GBASE_LR4", "100GBASE-LR4",
		"PMD_100GBASE_FR", "ETH_100GBASE_FR", "100GBASE_FR", "100GBASE-FR":
		return 0, oc.IfEthernet_ETHERNET_SPEED_UNSET, 0
	}

	hardwarePort := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).HardwarePort().State())
	descVal, present := gnmi.Lookup(t, dut, gnmi.OC().Component(hardwarePort).Description().State()).Val()

	var pmdParts []string
	if present && descVal != "" {
		pmdParts = append(pmdParts, strings.ToUpper(descVal))
	}
	if trName, trPresent := gnmi.Lookup(t, dut, gnmi.OC().Interface(port.Name()).Transceiver().State()).Val(); trPresent && trName != "" {
		if trDesc, trDescPresent := gnmi.Lookup(t, dut, gnmi.OC().Component(trName).Description().State()).Val(); trDescPresent && trDesc != "" {
			pmdParts = append(pmdParts, strings.ToUpper(trDesc))
		}
	}
	descStr := strings.Join(pmdParts, " ")

	// Match token-level PMD identities or optic types in component/transceiver descriptions.
	for _, token := range strings.Fields(descStr) {
		switch token {
		case "ETH_400GBASE_DR4", "400GBASE_DR4", "400GBASE-DR4":
			return 4, oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB, 1
		case "ETH_800GBASE_2XPLR4", "800GBASE-2XPLR4", "800GBASE_2XPLR4", "2XPLR4", "2PLR4",
			"ETH_800GBASE_2XDR4", "800GBASE-2XDR4", "800GBASE_2XDR4", "2XDR4", "2DR4",
			"8X100G", "8X100FR":
			return 8, oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB, 1
		case "ETH_800GBASE_2XFR4", "800GBASE_2XFR4", "800GBASE-2XFR4", "2XFR4", "2FR4",
			"ETH_800GBASE_2XLR4", "800GBASE-2XLR4", "800GBASE_2XLR4", "2XLR4", "2LR4",
			"2X400G":
			return 2, oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB, 4
		case "ETH_400GBASE_FR4", "400GBASE_FR4", "400GBASE-FR4", "400G-FR4",
			"ETH_100GBASE_LR4", "100GBASE_LR4", "100GBASE-LR4", "100G-LR",
			"ETH_100GBASE_FR", "100GBASE_FR", "100GBASE-FR", "100G-FR":
			return 0, oc.IfEthernet_ETHERNET_SPEED_UNSET, 0
		}
	}

	// Fallback substring and speed-based matching for composite hardware descriptions.
	switch {
	case containsAny(descStr, "400GBASE_DR4", "400GBASE-DR4"),
		port.Speed() == ondatra.Speed400Gb && strings.Contains(descStr, "100G"):
		return 4, oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB, 1

	case port.Speed() == ondatra.Speed800Gb,
		port.PMD() == ondatra.PMD800GBASEZR,
		port.PMD() == ondatra.PMD800GBASEZRP,
		strings.Contains(descStr, "800G"):
		switch {
		case containsAny(descStr, "2XPLR4", "2PLR4", "2XDR4", "2DR4", "8X100G", "8X100FR", "100G"):
			return 8, oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB, 1
		case containsAny(descStr, "2XFR4", "2FR4", "2XLR4", "2LR4", "2X400G", "400G"):
			return 2, oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB, 4
		default:
			t.Logf("Could not detect breakout mode from description %q, defaulting to 2x400G", descVal)
			return 2, oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB, 4
		}

	case containsAny(descStr, "400GBASE_FR4", "400G-FR4", "100GBASE_LR4", "100G-LR", "100GBASE_FR", "100G-FR"):
		return 0, oc.IfEthernet_ETHERNET_SPEED_UNSET, 0

	default:
		return 0, oc.IfEthernet_ETHERNET_SPEED_UNSET, 0
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
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

func matchHardwarePort(state, expected string) bool {
	if state == expected {
		return true
	}
	if strings.HasPrefix(state, expected+"/") {
		return true
	}
	trimmedExpected := strings.TrimSuffix(expected, "-Port")
	return strings.HasPrefix(state, trimmedExpected+"/")
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func verifyAndCollectInterfaces(t *testing.T, dut *ondatra.DUTDevice, expectedInterfaceCount map[string]int) map[string][]string {
	discoveredInterfaces := make(map[string][]string)
	interfaceToHWPort := make(map[string]string)

	query := gnmi.OC().InterfaceAny().State()

	_, ok := gnmi.WatchAll(t, dut, query, 2*time.Minute, func(val *ygnmi.Value[*oc.Interface]) bool {
		intf, present := val.Val()
		if !present {
			return false
		}

		intfName := intf.GetName()
		hwPort := intf.GetHardwarePort()

		if hwPort == "" {
			if oldHWPort, active := interfaceToHWPort[intfName]; active {
				delete(interfaceToHWPort, intfName)
				discoveredInterfaces[oldHWPort] = removeString(discoveredInterfaces[oldHWPort], intfName)
			}
			return false
		}

		matchedExpectedHWPort := ""
		for expectedHWPort := range expectedInterfaceCount {
			if matchHardwarePort(hwPort, expectedHWPort) {
				matchedExpectedHWPort = expectedHWPort
				break
			}
		}

		if matchedExpectedHWPort != "" {
			if oldHWPort, active := interfaceToHWPort[intfName]; active {
				if oldHWPort != matchedExpectedHWPort {
					discoveredInterfaces[oldHWPort] = removeString(discoveredInterfaces[oldHWPort], intfName)
				}
			}
			interfaceToHWPort[intfName] = matchedExpectedHWPort
			if !containsString(discoveredInterfaces[matchedExpectedHWPort], intfName) {
				discoveredInterfaces[matchedExpectedHWPort] = append(discoveredInterfaces[matchedExpectedHWPort], intfName)
			}
		}

		allMet := true
		for expectedHWPort, expectedCount := range expectedInterfaceCount {
			actualCount := len(discoveredInterfaces[expectedHWPort])
			if actualCount != expectedCount {
				allMet = false
				break
			}
		}
		return allMet
	}).Await(t)

	if !ok {
		t.Fatalf("Failed to discover all expected interfaces. Expected counts: %v, Discovered: %v", expectedInterfaceCount, discoveredInterfaces)
	}

	// Verify that the hardware port components actually exist in state
	for _, intfNames := range discoveredInterfaces {
		for _, intfName := range intfNames {
			actualHwPort := gnmi.Get(t, dut, gnmi.OC().Interface(intfName).HardwarePort().State())
			compNameQuery := gnmi.OC().Component(actualHwPort).Name().State()
			compName := gnmi.Get(t, dut, compNameQuery)
			t.Logf("Interface %v is associated with hardware port component %v", intfName, compName)
		}
	}

	return discoveredInterfaces
}

func mustConfigureBreakoutInterfaces(t *testing.T, dut *ondatra.DUTDevice, breakoutPorts map[string]breakoutInfo, discoveredInterfaces map[string][]string) {
	topo := gnmi.OC()
	for hwPort, info := range breakoutPorts {
		intfNames, present := discoveredInterfaces[hwPort]
		if !present {
			t.Errorf("No interfaces discovered for breakout hardware port %s", hwPort)
			continue
		}
		for _, intfName := range intfNames {
			t.Logf("Configuring breakout interface %s (pointing to %s) with speed %v", intfName, hwPort, info.speed)
			i := &oc.Interface{
				Name:    ygot.String(intfName),
				Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
				Enabled: ygot.Bool(true),
				Ethernet: &oc.Interface_Ethernet{
					PortSpeed: info.speed,
				},
			}
			gnmi.Replace(t, dut, topo.Interface(intfName).Config(), i)
		}
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
	t.Run("RT-8.1 - Baseline test", func(tb *testing.T) {
		baseLineTest(tb, dut, false)
	})
	t.Run("RT-8.2 - Reboot test", func(tb *testing.T) {
		rebootDUT(tb, dut)
		// Verify persistence first
		baseLineTest(tb, dut, true)
		// Then repeat RT-8.1 (configure and verify again)
		baseLineTest(tb, dut, false)
	})
}
