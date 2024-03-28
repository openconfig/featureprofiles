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

package singleton_with_breakouts

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
	maxCompWaitTime = 600
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Method to run baseLine test.
func baseLineTest(t *testing.T, dut *ondatra.DUTDevice, portsConfigured map[string]bool, rebootCheck bool) {
	if !rebootCheck {
		configureDUT(t, dut, portsConfigured)
	}
	verifyConsistentPMD(t, dut, portsConfigured)
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

// Method to verify if hardwarePort is populated with a reference to componentName.
func verifyConsistentPMD(t *testing.T, dut *ondatra.DUTDevice, portsConfigured map[string]bool) {
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	for _, i := range interfaces {
		interfaceName := i.GetName()
		if i.HardwarePort == nil {
			continue
		}
		hardwarePort := i.GetHardwarePort()
		var compName string
		var compNameFetchError *string
		compNameQuery := gnmi.OC().Component(hardwarePort).Name().State()
		compNameFetchError = testt.CaptureFatal(t, func(t testing.TB) {
			compName = gnmi.Get(t, dut, compNameQuery)
		})
		// Removing channelization number
		colonIndex := strings.Index(interfaceName, ":")
		if colonIndex != -1 {
			interfaceName = interfaceName[:colonIndex]
		}
		_, isConfiguredInterface := portsConfigured[interfaceName]

		if compNameFetchError != nil && isConfiguredInterface {
			t.Fatalf("%v error is seen while fetching component name for the hardware port %v for the interface  %v", compNameFetchError, hardwarePort, interfaceName)
		} else if isConfiguredInterface {
			portsConfigured[interfaceName] = true
			t.Logf("HardwarePort = %v of interface = %v is populated with a reference to component name %v", hardwarePort, interfaceName, compName)
		}
	}
	// Checking if all the configured interfaces have been checked for componentName reference.
	for interfaceName, referenceChecked := range portsConfigured {
		if referenceChecked == false {
			t.Fatalf("Interface %v not found in the fetched Interface list", interfaceName)
		}
		portsConfigured[interfaceName] = false
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
		rebootDUT(t, dut)
		baseLineTest(tb, dut, portsConfigured, true)
	})
}
