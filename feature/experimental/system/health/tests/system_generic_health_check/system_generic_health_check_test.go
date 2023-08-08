// Copyright 2023 Google LLC
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

package system_generic_health_check_test

import (
	"context"

	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/raw"
	"github.com/openconfig/ygnmi/ygnmi"

	fpb "github.com/openconfig/gnoi/file"
	hpb "github.com/openconfig/gnoi/healthz"
	tpb "github.com/openconfig/gnoi/types"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	vendorCoreFilePath = map[ondatra.Vendor]string{
		ondatra.JUNIPER: "/var/core/",
		ondatra.CISCO:   "/misc/disk1/",
		ondatra.NOKIA:   "/var/core/",
	}
	vendorCoreFileNamePattern = map[ondatra.Vendor]*regexp.Regexp{
		ondatra.JUNIPER: regexp.MustCompile(".*.tar.gz"),
		ondatra.CISCO:   regexp.MustCompile("/misc/disk1/.*core.*"),
		ondatra.NOKIA:   regexp.MustCompile("/var/core/coredump-.*"),
	}
	alarmSeverityCheckList = []oc.E_AlarmTypes_OPENCONFIG_ALARM_SEVERITY{
		oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_MAJOR,
		//oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_MINOR,
	}
)

const (
	cpuType               = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CPU
	lineCardType          = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
	fabricCardType        = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC
	controllerCardType    = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	cpuHighUtilization    = 80
	memoryHighUtilization = 80
)

// coreFileCheck function is used to check if cores are found on the DUT.
func coreFileCheck(t *testing.T, dut *ondatra.DUTDevice, gnoiClient raw.GNOI, sysConfigTime uint64, retry bool) {
	t.Helper()
	t.Log("Checking for core files on DUT")

	dutVendor := dut.Vendor()
	// vendorCoreFilePath and vendorCoreProcName should be provided to fetch core file on dut.
	if _, ok := vendorCoreFilePath[dutVendor]; !ok {
		t.Fatalf("Please add support for vendor %v in var vendorCoreFilePath ", dutVendor)
	}
	if _, ok := vendorCoreFileNamePattern[dutVendor]; !ok {
		t.Fatalf("Please add support for vendor %v in var vendorCoreFileNamePattern.", dutVendor)
	}

	in := &fpb.StatRequest{
		Path: vendorCoreFilePath[dutVendor],
	}
	validResponse, err := gnoiClient.File().Stat(context.Background(), in)
	if err != nil {
		if retry {
			t.Logf("Retry GNOI request to check %v for core files on DUT", vendorCoreFilePath[dutVendor])
			validResponse, err = gnoiClient.File().Stat(context.Background(), in)
		}
	}
	if err != nil {
		t.Fatalf("Unable to stat path %v for core files on DUT, %v", vendorCoreFilePath[dutVendor], err)
	}
	// Check cores creation time is greater than test start time.
	for _, fileStatsInfo := range validResponse.GetStats() {
		if fileStatsInfo.GetLastModified() > sysConfigTime {
			coreFileName := fileStatsInfo.GetPath()
			r := vendorCoreFileNamePattern[dutVendor]
			if r.MatchString(coreFileName) {
				t.Errorf("ERROR: Found core %v on DUT.", coreFileName)
			}
		}
		in = &fpb.StatRequest{
			Path: fileStatsInfo.GetPath(),
		}
		validResponse, err := gnoiClient.File().Stat(context.Background(), in)
		if err != nil {
			t.Fatalf("Unable to stat path %v for core files on DUT, %v", vendorCoreFilePath[dutVendor], err)
		}
		for _, fileStatsInfo := range validResponse.GetStats() {
			coreFileName := fileStatsInfo.GetPath()
			r := vendorCoreFileNamePattern[dutVendor]
			if r.MatchString(coreFileName) {
				t.Errorf("ERROR: Found core %v on DUT.", coreFileName)
			}
		}
	}
}

func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.Slice(ports, func(i, j int) bool {
		idi, idj := ports[i].ID(), ports[j].ID()
		li, lj := len(idi), len(idj)
		if li == lj {
			return idi < idj
		}
		return li < lj // "port2" < "port10"
	})
	return ports
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func removeElement(list []string, element string) []string {
	for i := 0; i < len(list); i++ {
		if list[i] == element {
			list = append(list[:i], list[i+1:]...)
			i-- // Adjust index to account for removed element
		}
	}
	return list
}

func TestCheckForCoreFiles(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	timestamp := uint64(time.Now().UTC().Unix())
	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	coreFileCheck(t, dut, gnoiClient, timestamp, true)
}

func TestComponentStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	controllerCards := components.FindComponentsByType(t, dut, controllerCardType)
	lineCards := components.FindComponentsByType(t, dut, lineCardType)
	fabricCards := components.FindComponentsByType(t, dut, fabricCardType)
	checkComponents := append(controllerCards, lineCards...)
	checkComponents = append(checkComponents, fabricCards...)
	// check oper-status of the components is Active.
	for _, component := range checkComponents {
		val, present := gnmi.Lookup(t, dut, gnmi.OC().Component(component).OperStatus().State()).Val()
		if !present {
			t.Errorf("ERROR: Get component %s oper-status failed", component)
		}
		if present && val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("ERROR: Get component %s oper-status failed", component)
		}
	}

	if deviations.ConsistentComponentNamesUnsupported(dut) {
		t.Skipf("Skipping test due to deviation consistent_component_names_unsupported")
	}

	// use gNOI to check components health status is not "unhealthy".
	gnoiClient := dut.RawAPIs().GNOI().New(t)
	for _, component := range checkComponents {
		componentName := map[string]string{"name": component}
		req := &hpb.GetRequest{
			Path: &tpb.Path{
				Elem: []*tpb.PathElem{
					{Name: "components"},
					{
						Name: "component",
						Key:  componentName,
					},
				},
			},
		}
		validResponse, err := gnoiClient.Healthz().Get(context.Background(), req)
		if err != nil {
			t.Errorf("Error: %v", err)
			continue
		}
		if validResponse.GetComponent().Status == hpb.Status_STATUS_UNHEALTHY {
			t.Errorf("Found unhealthy component: %s", component)
		} else {
			t.Logf("Response: %+v", validResponse)
		}
	}
}

func TestControllerCardsNoHighCPUSpike(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()

	if deviations.ControllerCardCPUUtilizationUnsupported(dut) {
		t.Skipf("Skipping test due to deviation linecard_cpu_ultilization_unsupported")
	}

	controllerCards := components.FindComponentsByType(t, dut, controllerCardType)
	cardList := controllerCards
	cpuCards := components.FindComponentsByType(t, dut, cpuType)
	for _, cpu := range cpuCards {
		query := gnmi.OC().Component(cpu).State()
		timestamp := time.Now().Round(time.Second)
		component := gnmi.Get(t, dut, query)
		cpuParent := component.GetParent()
		if cpuParent == "" {
			t.Errorf("ERROR: can't find parent information for CPU card %v", component)
		}

		if contains(cardList, cpuParent) {
			// Remove parent from the list of check cards.
			cardList = removeElement(cardList, cpuParent)
			cpuUtilization := component.GetCpu().GetUtilization()
			if cpuUtilization == nil {
				t.Errorf("ERROR: %s %s %s Type %-20s - CPU utilization data not available",
					timestamp, deviceName, cpu, cpuParent)
				continue
			}
			averageUtilization := cpuUtilization.GetAvg()
			if averageUtilization > cpuHighUtilization {
				t.Errorf("ERROR: %s %s Type %-20s %-10s - Utilization: %3d%%, exceeding threshold: %3d%%",
					timestamp, deviceName, cpu, cpuParent, averageUtilization, cpuHighUtilization)
			} else {
				t.Logf("INFO: %s %s Type %-20s %-10s - Utilization: %3d%%",
					timestamp, deviceName, cpu, cpuParent, averageUtilization)
			}
		}
	}
	if len(cardList) > 0 {
		t.Errorf("ERROR: Didn't find cpu card for checkCards %s", cardList)
	}
}

func TestLineCardsNoHighCPUSpike(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()

	if deviations.LinecardCPUUtilizationUnsupported(dut) {
		t.Skipf("Skipping test due to deviation linecard_cpu_ultilization_unsupported")
	}

	lineCards := components.FindComponentsByType(t, dut, lineCardType)
	cardList := lineCards
	cpuCards := components.FindComponentsByType(t, dut, cpuType)
	for _, cpu := range cpuCards {
		timestamp := time.Now().Round(time.Second)

		query := gnmi.OC().Component(cpu).State()
		component := gnmi.Get(t, dut, query)

		cpuParent := component.GetParent()
		if cpuParent == "" {
			t.Errorf("ERROR: can't find parent information for CPU card %v", component)
		}

		// If cpu card's parent is line card, check cpu ultilization.
		if contains(cardList, cpuParent) {
			// Remove parent from the list of check cards.
			cardList = removeElement(cardList, cpuParent)
			// Fetch CPU utilization data.
			cpuUtilization := component.GetCpu().GetUtilization()
			if cpuUtilization == nil {
				t.Errorf("ERROR: %s %s %s Type %-20s - CPU utilization data not available",
					timestamp, deviceName, cpu, cpuParent)
				continue
			}
			averageUtilization := cpuUtilization.GetAvg()
			if averageUtilization > cpuHighUtilization {
				t.Errorf("ERROR: %s %s Type %-20s %-10s - Utilization: %3d%%, exceeding threshold: %3d%%",
					timestamp, deviceName, cpu, cpuParent, averageUtilization, cpuHighUtilization)
			} else {
				t.Logf("INFO: %s %s Type %-20s %-10s - Utilization: %3d%%",
					timestamp, deviceName, cpu, cpuParent, averageUtilization)
			}
		}
	}

	if len(cardList) > 0 {
		t.Errorf("ERROR: Didn't find cpu card for checkCards %s", cardList)
	}
}

func TestComponentsNoHighMemoryUtilization(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()
	description := "Component"

	controllerCards := components.FindComponentsByType(t, dut, controllerCardType)
	lineCards := components.FindComponentsByType(t, dut, lineCardType)
	cardList := append(controllerCards, lineCards...)
	for _, component := range cardList {
		query := gnmi.OC().Component(component).State()
		timestamp := time.Now().Round(time.Second)
		componentState := gnmi.Get(t, dut, query)
		componentType := componentState.GetType()
		if componentType == lineCardType && deviations.LinecardMemoryUtilizationUnsupported(dut) {
			t.Logf("INFO: Skipping test for linecard component %s due to deviation linecard_memory_utilization_unsupported", component)
			continue
		}

		memoryState := componentState.GetMemory()
		if memoryState == nil {
			t.Errorf("ERROR: %s - Device: %s - %s: %-40s - Type: %-20s - Memory data not available",
				timestamp, deviceName, description, component, componentType)
			continue
		}

		memoryAvailable := memoryState.GetAvailable()
		memoryUtilized := memoryState.GetUtilized()
		memoryUtilization := uint8((memoryUtilized * 100) / (memoryAvailable + memoryUtilized))
		if memoryUtilization > memoryHighUtilization {
			t.Errorf("ERROR: %s - Device: %s - %s: %-40s - Type: %-20s - Utilization: %3d%% (Threshold: %3d%%)",
				timestamp, deviceName, description, component, componentType, memoryUtilization, memoryHighUtilization)
		} else {
			t.Logf("INFO: %s - Device: %s - %s: %-40s - Type: %-20s - Utilization: %3d%%",
				timestamp, deviceName, description, component, componentType, memoryUtilization)
		}
	}
}

func TestSystemProcessNoHighCPUSpike(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()

	query := gnmi.OC().System().ProcessAny().State()
	description := "System CPU Process"
	threshold := uint8(cpuHighUtilization)

	timestamp := time.Now().Round(time.Second)
	results := gnmi.GetAll(t, dut, query)
	for _, result := range results {
		processName := result.GetName()
		utilization := result.GetCpuUtilization()

		if utilization > threshold {
			t.Errorf("%s %s ERROR: %s process %-40s utilization: %3d%%, exceeding %3d%%", timestamp, deviceName, description, processName, utilization, threshold)
		} else {
			t.Logf("%s %s INFO: %s process %-40s utilization: %3d%%", timestamp, deviceName, description, processName, utilization)
		}
	}
}

func TestSystemProcessNoHighMemorySpike(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()

	query := gnmi.OC().System().ProcessAny().State()
	description := "System Process Memory"

	currentTime := time.Now().Round(time.Second)
	processes := gnmi.GetAll(t, dut, query)
	for _, process := range processes {
		processName := process.GetName()

		if process.MemoryUtilization != nil {
			utilization := process.GetMemoryUtilization()
			if utilization > memoryHighUtilization {
				t.Errorf("%s %s ERROR: %s - Process: %-40s - Utilization: %3d%%, exceeding %3d%%", currentTime, deviceName, description, processName, utilization, memoryHighUtilization)
			} else {
				t.Logf("%s %s INFO:  %s - Process: %-40s - Utilization: %3d%%", currentTime, deviceName, description, processName, utilization)
			}
		} else {
			t.Logf("%s %s INFO:  %s - Process: %-40s - Utilization data not available", currentTime, deviceName, description, processName)
		}
	}
}

func TestNoQueueDrop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	type testCase struct {
		desc     string
		path     string
		counters []*ygnmi.Value[uint64]
	}

	dutPorts := sortPorts(dut.Ports())
	interfaces := []string{}
	for _, port := range dutPorts {
		interfaces = append(interfaces, port.Name())
	}

	for _, intf := range interfaces {
		qosInterface := gnmi.OC().Qos().Interface(intf)
		cases := []testCase{
			{
				desc:     "Queue Input Dropped packets",
				path:     "/qos/interfaces/interface/input/queues/queue/state/dropped-pkts",
				counters: gnmi.LookupAll(t, dut, qosInterface.Input().QueueAny().DroppedPkts().State()),
			},
			{
				desc:     "Queue Output Dropped packets",
				path:     "/qos/interfaces/interface/output/queues/queue/state/dropped-pkts",
				counters: gnmi.LookupAll(t, dut, qosInterface.Output().QueueAny().DroppedPkts().State()),
			},
			{
				desc:     "Queue input voq-output-interface dropped packets",
				path:     "/qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/dropped-pkts",
				counters: gnmi.LookupAll(t, dut, qosInterface.Input().VoqInterfaceAny().QueueAny().DroppedPkts().State()),
			},
		}

		for _, c := range cases {
			desc := c.desc
			counters := c.counters

			t.Run(desc, func(t *testing.T) {
				if len(counters) == 0 {
					t.Skipf("%s Interface %s Telemetry Value is not present", desc, intf)
				}
				for queueID, dropPkt := range counters {
					dropCount, present := dropPkt.Val()
					if !present {
						t.Errorf("%s Interface %s %s Telemetry Value is not present", desc, intf, dropPkt.Path)
					} else {
						if present && dropCount > 0 {
							t.Errorf("%s Interface %s, Queue %d has %d drop(s)", desc, intf, queueID, dropCount)
						}
					}
				}
			})
		}
	}
}

func TestNoAsicDrop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().State()
	asicDrops := gnmi.GetAll(t, dut, query)
	if len(asicDrops) == 0 {
		t.Fatalf("ERROR: %s is not present", query)
	}
	for _, drop := range asicDrops {
		if drop.InterfaceBlock != nil {
			if drop.InterfaceBlock.GetInDrops() > 0 {
				t.Errorf("ERROR: InDrops counter got %d, want 0", drop.InterfaceBlock.GetInDrops())
			}
			if drop.InterfaceBlock.GetOutDrops() > 0 {
				t.Errorf("ERROR: InDrops counter got %d, want 0", drop.InterfaceBlock.GetOutDrops())
			}
			if drop.InterfaceBlock.GetOversubscription() > 0 {
				t.Errorf("ERROR: InDrops counter got %d, want 0", drop.InterfaceBlock.GetOversubscription())
			}
		}
		if drop.LookupBlock != nil {
			if drop.LookupBlock.GetAclDrops() > 0 {
				t.Errorf("ERROR: AclDrops counter got %d, want 0", drop.LookupBlock.GetAclDrops())
			}
			if drop.LookupBlock.GetForwardingPolicy() > 0 {
				t.Errorf("ERROR: ForwardingPolicy got %d, want 0", drop.LookupBlock.GetForwardingPolicy())
			}
			if drop.LookupBlock.GetFragmentTotalDrops() > 0 {
				t.Errorf("ERROR: FragmentTotalDrops got %d, want 0", drop.LookupBlock.GetFragmentTotalDrops())
			}
			if drop.LookupBlock.GetIncorrectSoftwareState() > 0 {
				t.Errorf("ERROR: IncorrectSoftwareState counter got %d, want 0", drop.LookupBlock.GetIncorrectSoftwareState())
			}
			if drop.LookupBlock.GetInvalidPacket() > 0 {
				t.Errorf("ERROR: InvalidPacket counter got %d, want 0", drop.LookupBlock.GetInvalidPacket())
			}
			if drop.LookupBlock.GetLookupAggregate() > 0 {
				t.Errorf("ERROR: LookupAggregate counter got %d, want 0", drop.LookupBlock.GetLookupAggregate())
			}
			if drop.LookupBlock.GetNoLabel() > 0 {
				t.Errorf("ERROR: NoLabel counter got %d, want 0", drop.LookupBlock.GetNoLabel())
			}
			if drop.LookupBlock.GetNoNexthop() > 0 {
				t.Errorf("ERROR: NoNexthop counter is %d, not zero", drop.LookupBlock.GetNoNexthop())
			}
			if drop.LookupBlock.GetNoRoute() > 0 {
				t.Errorf("ERROR: NoRoute counter is %d, not zero", drop.LookupBlock.GetNoRoute())
			}
			if drop.LookupBlock.GetOversubscription() > 0 {
				t.Errorf("ERROR: Oversubscription counter got %d, want 0", drop.LookupBlock.GetOversubscription())
			}
			if drop.LookupBlock.GetRateLimit() > 0 {
				t.Errorf("ERROR: RateLimit counter got %d, want 0", drop.LookupBlock.GetRateLimit())
			}
		}
	}
}

func TestInterfaceStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	dutPorts := sortPorts(dut.Ports())
	interfaces := []string{}
	for _, port := range dutPorts {
		interfaces = append(interfaces, port.Name())
	}
	t.Logf("Interface: %s", interfaces)

	for _, intf := range interfaces {
		query := gnmi.OC().Interface(intf).State()
		root := gnmi.Get(t, dut, query)

		if root.GetOperStatus() != oc.Interface_OperStatus_UP {
			t.Errorf("ERROR: Oper status is not up on interface %s", intf)
		}
		if root.GetAdminStatus() != oc.Interface_AdminStatus_UP {
			t.Errorf("ERROR: Admin status is not up on interface %s", intf)
		}
		if root.Type == oc.IETFInterfaces_InterfaceType_UNSET {
			t.Errorf("ERROR: Type is not present on interface %s", intf)
		}
		if root.Description == nil {
			t.Errorf("ERROR: Description is not present on interface %s", intf)
		}
		if root.GetCounters().OutOctets == nil {
			t.Errorf("Counter OutOctets is not present on interface %s", intf)
		}
		if root.GetCounters().InMulticastPkts == nil {
			t.Errorf("ERROR: Counter InMulticastPkts is not present on interface %s", intf)
		}
		if root.GetCounters().InDiscards == nil {
			t.Errorf("ERROR: Counter InDiscards is not present on interface %s", intf)
		} else {
			if root.GetCounters().GetInDiscards() > 0 {
				t.Errorf("Counter InDiscards is %d, not zero", root.GetCounters().GetInDiscards())
			}
		}
		if root.GetCounters().InErrors == nil {
			t.Errorf("ERROR: Counter InErrors is not present on interface %s", intf)
		} else {
			if root.GetCounters().GetInErrors() > 0 {
				t.Errorf("ERROR: Counter InErrors is %d, not zero", root.GetCounters().GetInErrors())
			}
		}
		if root.GetCounters().InUnknownProtos == nil {
			t.Errorf("ERROR: Counter InUnknownProtos is not present on interface %s", intf)
		} else {
			if root.GetCounters().GetInUnknownProtos() > 0 {
				t.Errorf("ERROR: Counter InUnknownProtos is %d, not zero", root.GetCounters().GetInUnknownProtos())
			}
		}
		if root.GetCounters().OutDiscards == nil {
			t.Errorf("ERROR: Counter OutDiscards is not present on interface %s", intf)
		} else {
			if root.GetCounters().GetOutDiscards() > 0 {
				t.Errorf("ERROR: Counter OutDiscards is %d, not zero", root.GetCounters().GetOutDiscards())
			}
		}
		if root.GetCounters().OutErrors == nil {
			t.Errorf("ERROR: Counter OutErrors is not present on interface %s", intf)
		} else {
			if root.GetCounters().GetOutErrors() > 0 {
				t.Errorf("ERROR: Counter OutErrors is %d, not zero", root.GetCounters().GetOutErrors())
			}
		}
		if root.GetCounters().InFcsErrors == nil {
			t.Errorf("ERROR: Counter InFcsErrors is not present on interface %s", intf)
		} else {
			if root.GetCounters().GetInFcsErrors() > 0 {
				t.Errorf("ERROR: Counter InFcsErrors is %d, not zero", root.GetCounters().GetInFcsErrors())
			}
		}
	}
}

func TestInterfacesubIntfs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	dutPorts := sortPorts(dut.Ports())
	interfaces := []string{}
	for _, port := range dutPorts {
		interfaces = append(interfaces, port.Name())
	}

	for _, intf := range interfaces {
		subIntfIndexes := gnmi.LookupAll(t, dut, gnmi.OC().Interface(intf).SubinterfaceAny().Index().State())
		for _, index := range subIntfIndexes {
			subIntfIndex, present := index.Val()
			if !present {
				t.Errorf("ERROR: subIntf index value doesn't exist")
				continue
			}

			subIntfPath := gnmi.OC().Interface(intf).Subinterface(subIntfIndex)
			subIntfState := gnmi.Get(t, dut, subIntfPath.State())
			subIntf := subIntfState.GetName()
			intfState := gnmi.Get(t, dut, gnmi.OC().Interface(intf).State())
			if subIntfState.OperStatus != oc.Interface_OperStatus_UP {
				t.Errorf("ERROR: Oper status is not up on interface %s", subIntf)
			}

			if subIntfState.AdminStatus != oc.Interface_AdminStatus_UP {
				t.Errorf("ERROR: Admin status is not up on interface %s", subIntf)
			}

			if subIntfState.Description == nil {
				t.Errorf("ERROR: Description is not present on interface %s", subIntf)
			}

			if subIntfState.GetCounters().OutOctets == nil && intfState.GetCounters().OutOctets == nil {
				t.Errorf("ERROR: Counter OutOctets is not present on interface %s, %s", subIntf, intf)
			}

			if subIntfState.GetCounters().InMulticastPkts == nil && intfState.GetCounters().InMulticastPkts == nil {
				t.Errorf("ERROR: Counter InMulticastPkts is not present on interface %s", subIntf)
			}

			counters := subIntfPath.Counters()
			parentCounters := gnmi.OC().Interface(intf).Counters()

			cases := []struct {
				desc          string
				counter       ygnmi.SingletonQuery[uint64]
				parentCounter ygnmi.SingletonQuery[uint64]
			}{
				{
					desc:          "InDiscards",
					counter:       counters.InDiscards().State(),
					parentCounter: parentCounters.InDiscards().State(),
				},
				{
					desc:          "InErrors",
					counter:       counters.InErrors().State(),
					parentCounter: parentCounters.InErrors().State(),
				},
				{
					desc:          "InUnknownProtos",
					counter:       counters.InUnknownProtos().State(),
					parentCounter: parentCounters.InUnknownProtos().State(),
				},
				{
					desc:          "OutDiscards",
					counter:       counters.OutDiscards().State(),
					parentCounter: parentCounters.OutDiscards().State(),
				},
				{
					desc:          "OutErrors",
					counter:       counters.OutErrors().State(),
					parentCounter: parentCounters.OutErrors().State(),
				},
				{
					desc:          "InFcsErrors",
					counter:       counters.InFcsErrors().State(),
					parentCounter: parentCounters.InFcsErrors().State(),
				},
			}

			for _, c := range cases {
				t.Run(c.desc, func(t *testing.T) {
					if val, present := gnmi.Lookup(t, dut, c.counter).Val(); present && (val == 0) {
						t.Logf("INFO: %s: %d", c.counter, val)
					} else if pVal, pPresent := gnmi.Lookup(t, dut, c.parentCounter).Val(); pPresent && (pVal == 0) {
						t.Logf("INFO: %s: %d", c.parentCounter, val)
					} else {
						t.Logf("ERROR: %s and %s check fail", c.counter, c.parentCounter)
					}
				})
			}
		}
	}
}

func TestInterfaceEthernetNoDrop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	dutPorts := sortPorts(dut.Ports())
	interfaces := []string{}
	for _, port := range dutPorts {
		interfaces = append(interfaces, port.Name())
	}

	for _, intf := range interfaces {
		counters := gnmi.OC().Interface(intf).Ethernet().Counters()
		cases := []struct {
			desc    string
			counter ygnmi.SingletonQuery[uint64]
		}{
			{
				desc:    "InCrcErrors",
				counter: counters.InCrcErrors().State(),
			},
			{
				desc:    "InMacPauseFrames",
				counter: counters.InMacPauseFrames().State(),
			},
			{
				desc:    "OutMacPauseFrames",
				counter: counters.OutMacPauseFrames().State(),
			},
			{
				desc:    "InBlockErrors",
				counter: counters.InBlockErrors().State(),
			},
		}

		for _, c := range cases {
			t.Run(c.desc, func(t *testing.T) {
				if val, present := gnmi.Lookup(t, dut, c.counter).Val(); present && (val == 0) {
					t.Logf("INFO: %s: want 0, got %d", c.counter, val)
				} else {
					t.Errorf("ERROR: %s: present %t, got %d, want 0", c.counter, present, val)
				}
			})
		}
	}
}

func TestSystemAlarms(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	query := gnmi.OC().System().AlarmAny().State()
	alarms := gnmi.GetAll(t, dut, query)
	if len(alarms) > 0 {
		for _, a := range alarms {
			alarmSeverity := a.GetSeverity()
			for _, alarmSeverityCheck := range alarmSeverityCheckList {
				if alarmSeverity == alarmSeverityCheck {
					t.Errorf("Error: System Alarms Severity: %s %s", alarmSeverity, a.GetText())
					break
				}
			}
		}
	}
}

func TestFabricDrop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("INFO: Check no fabric drop")
	if deviations.FabricDropCounterUnsupported(dut) {
		t.Skipf("INFO: Skipping test due to deviation fabric_drop_counter_unsupported")
	}
	query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().FabricBlock().State()
	t.Logf("query %s", query)
	fabricBlocks := gnmi.GetAll(t, dut, query)
	if len(fabricBlocks) == 0 {
		t.Fatalf("ERROR: %s is not present", query)
	}
	for _, fabricBlock := range fabricBlocks {
		drop := fabricBlock.GetLostPackets()
		if fabricBlock.LostPackets != nil && drop == 0 {
			t.Logf("INFO: Fabric drops want 0, got %d", drop)
		} else {
			t.Errorf("ERROR: Fabric drops got %d, nil or not 0", drop)
		}
	}
}
