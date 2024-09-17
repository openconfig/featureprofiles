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
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnoigo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"golang.org/x/exp/slices"

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
		ondatra.ARISTA:  "/var/core/",
	}
	vendorCoreFileNamePattern = map[ondatra.Vendor]*regexp.Regexp{
		ondatra.JUNIPER: regexp.MustCompile(".*.tar.gz"),
		ondatra.CISCO:   regexp.MustCompile("/misc/disk1/.*core.*"),
		ondatra.NOKIA:   regexp.MustCompile("/var/core/coredump-.*"),
		ondatra.ARISTA:  regexp.MustCompile("/var/core/core.*"),
	}
)

const (
	cpuType            = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CPU
	lineCardType       = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
	fabricCardType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC
	controllerCardType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
)

// coreFileCheck function is used to check if cores are found on the DUT.
func coreFileCheck(t *testing.T, dut *ondatra.DUTDevice, gnoiClient gnoigo.Clients, sysConfigTime uint64, retry bool) {
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
		if err != nil {
			t.Fatalf("Unable to stat path %v for core files on DUT, %v", vendorCoreFilePath[dutVendor], err)
		}
	}

	// Check cores creation time is greater than test start time.
	for _, fileStatsInfo := range validResponse.GetStats() {
		if fileStatsInfo.GetLastModified() > sysConfigTime {
			coreFileName := fileStatsInfo.GetPath()
			r := vendorCoreFileNamePattern[dutVendor]
			if r.MatchString(coreFileName) {
				t.Logf("INFO: Found core %v on DUT.", coreFileName)
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
				t.Logf("INFO: Found core %v on DUT.", coreFileName)
			}
		}
	}
}

func sortedInterfaces(ports []*ondatra.Port) []string {
	var interfaces []string
	for _, port := range ports {
		interfaces = append(interfaces, port.Name())
	}
	slices.Sort(interfaces)
	return interfaces
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
	gnoiClient := dut.RawAPIs().GNOI(t)
	coreFileCheck(t, dut, gnoiClient, timestamp, true)
}

func TestComponentStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	controllerCards := components.FindComponentsByType(t, dut, controllerCardType)
	lineCards := components.FindComponentsByType(t, dut, lineCardType)
	fabricCards := components.FindComponentsByType(t, dut, fabricCardType)
	checkComponents := append(controllerCards, lineCards...)
	checkComponents = append(checkComponents, fabricCards...)
	if len(checkComponents) == 0 {
		t.Errorf("ERROR: No component has been found.")
	}
	gnoiClient := dut.RawAPIs().GNOI(t)
	// check oper-status of the components is Active.
	for _, component := range checkComponents {
		t.Run(component, func(t *testing.T) {
			compMtyVal, compMtyPresent := gnmi.Lookup(t, dut, gnmi.OC().Component(component).Empty().State()).Val()
			if compMtyPresent && compMtyVal {
				t.Skipf("INFO: Skip status check as %s is empty", component)
			}
			val, present := gnmi.Lookup(t, dut, gnmi.OC().Component(component).OperStatus().State()).Val()
			if !present {
				t.Errorf("ERROR: Get component %s oper-status failed", component)
			} else {
				t.Logf("INFO: Component %s oper-status: %s", component, val)
			}
			// use gNOI to check components health status is not "unhealthy".
			if deviations.ConsistentComponentNamesUnsupported(dut) {
				t.Skipf("Skipping test due to deviation consistent_component_names_unsupported")
			}

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
				t.Errorf("ERROR: %v", err)
			} else {
				t.Logf("INFO: Component %s Healthz Status: %s", component, validResponse.GetComponent().Status)
			}
		})
	}
}

func TestControllerCardsNoHighCPUSpike(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()

	if deviations.ControllerCardCPUUtilizationUnsupported(dut) {
		t.Skipf("Skipping test due to deviation controller_card_cpu_utilization_unsupported")
	}

	controllerCards := components.FindComponentsByType(t, dut, controllerCardType)
	cpuCards := components.FindComponentsByType(t, dut, cpuType)
	if len(controllerCards) == 0 || len(cpuCards) == 0 {
		t.Errorf("ERROR: No controllerCard or cpuCard has been found.")
	}
	for _, cpu := range cpuCards {
		t.Run(cpu, func(t *testing.T) {
			query := gnmi.OC().Component(cpu).State()
			timestamp := time.Now().Round(time.Second)
			component := gnmi.Get(t, dut, query)
			cpuParent := component.GetParent()
			if cpuParent == "" {
				t.Errorf("ERROR: can't find parent information for CPU card %v", component)
			}

			if slices.Contains(controllerCards, cpuParent) {
				// Remove parent from the list of check cards.
				controllerCards = removeElement(controllerCards, cpuParent)
				cpuUtilization := component.GetCpu().GetUtilization()
				if cpuUtilization.Avg == nil {
					t.Errorf("ERROR: %s %s %s Type %-20s - CPU utilization data not available",
						timestamp, deviceName, cpu, cpuParent)
				} else {
					t.Logf("INFO: %s %s Type %-20s %-10s - Utilization: %3d%%", timestamp, deviceName, cpu, cpuParent, cpuUtilization.GetAvg())
				}
			}
		})

	}
	if len(controllerCards) > 0 {
		t.Errorf("ERROR: Didn't find cpu card for checkCards %s", controllerCards)
	}
}

func TestLineCardsNoHighCPUSpike(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()

	if deviations.LinecardCPUUtilizationUnsupported(dut) {
		t.Skipf("Skipping test due to deviation linecard_cpu_ultilization_unsupported")
	}

	lineCards := components.FindComponentsByType(t, dut, lineCardType)
	cpuCards := components.FindComponentsByType(t, dut, cpuType)
	if len(lineCards) == 0 || len(cpuCards) == 0 {
		t.Errorf("ERROR: No controllerCard or cpuCard has been found.")
	}
	for _, cpu := range cpuCards {
		t.Run(cpu, func(t *testing.T) {
			timestamp := time.Now().Round(time.Second)
			query := gnmi.OC().Component(cpu).State()
			component := gnmi.Get(t, dut, query)

			cpuParent := component.GetParent()
			if cpuParent == "" {
				t.Errorf("ERROR: can't find parent information for CPU card %v", component)
			}

			// If cpu card's parent is line card, check cpu ultilization.
			if slices.Contains(lineCards, cpuParent) {
				// Remove parent from the list of check cards.
				lineCards = removeElement(lineCards, cpuParent)
				// Fetch CPU utilization data.
				cpuUtilization := component.GetCpu().GetUtilization()
				if cpuUtilization.Avg == nil {
					t.Errorf("ERROR: %s %s %s Type %-20s - CPU utilization data not available",
						timestamp, deviceName, cpu, cpuParent)
				} else {
					t.Logf("INFO: %s %s Type %-20s %-10s - Utilization: %3d%%", timestamp, deviceName, cpu, cpuParent, cpuUtilization.GetAvg())
				}
			}
		})
	}

	if len(lineCards) > 0 {
		t.Errorf("ERROR: Didn't find cpu card for checkCards %s", lineCards)
	}
}

func TestComponentsNoHighMemoryUtilization(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()
	description := "Component"

	controllerCards := components.FindComponentsByType(t, dut, controllerCardType)
	lineCards := components.FindComponentsByType(t, dut, lineCardType)
	cardList := append(controllerCards, lineCards...)
	if len(cardList) == 0 {
		t.Errorf("ERROR: No card has been found.")
	}
	for _, component := range cardList {
		t.Run(component, func(t *testing.T) {
			query := gnmi.OC().Component(component).State()
			timestamp := time.Now().Round(time.Second)
			componentState := gnmi.Get(t, dut, query)
			componentType := componentState.GetType()
			if componentType == lineCardType && deviations.LinecardMemoryUtilizationUnsupported(dut) {
				t.Skipf("INFO: Skipping test for linecard component %s due to deviation linecard_memory_utilization_unsupported", component)
			}

			memoryState := componentState.GetMemory()
			if memoryState == nil {
				t.Errorf("ERROR: %s - Device: %s - %s: %-40s - Type: %-20s - Memory data not available",
					timestamp, deviceName, description, component, componentType)
			} else {
				memoryAvailable := memoryState.GetAvailable()
				memoryUtilized := memoryState.GetUtilized()
				memoryUtilization := uint8((memoryUtilized * 100) / (memoryAvailable + memoryUtilized))
				t.Logf("INFO: %s - Device: %s - %s: %-40s - Type: %-20s - Utilization: %3d%%", timestamp, deviceName, description, component, componentType, memoryUtilization)
			}
		})
	}
}

func TestSystemProcessNoHighCPUSpike(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()

	query := gnmi.OC().System().ProcessAny().State()
	const description = "System CPU Process"

	timestamp := time.Now().Round(time.Second)
	results := gnmi.GetAll(t, dut, query)
	for _, result := range results {
		processName := result.GetName()
		t.Run(processName, func(t *testing.T) {
			if result.CpuUtilization == nil {
				t.Errorf("%s %s ERROR: %s process %-40s utilization not available", timestamp, deviceName, description, processName)
			} else {
				t.Logf("%s %s INFO: %s process %-40s utilization: %3d%%", timestamp, deviceName, description, processName, result.GetCpuUtilization())
			}
		})
	}
}

func TestSystemProcessNoHighMemorySpike(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()

	query := gnmi.OC().System().ProcessAny().State()
	const description = "System Process Memory"

	currentTime := time.Now().Round(time.Second)
	processes := gnmi.GetAll(t, dut, query)
	for _, process := range processes {
		processName := process.GetName()
		t.Run(processName, func(t *testing.T) {
			if process.MemoryUtilization != nil {
				t.Logf("%s %s INFO:  %s - Process: %-40s - Utilization: %3d%%", currentTime, deviceName, description, processName, process.GetMemoryUtilization())
			} else {
				t.Errorf("%s %s ERROR:  %s - Process: %-40s - Utilization data not available", currentTime, deviceName, description, processName)
			}
		})
	}
}

func TestNoQueueDrop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	type testCase struct {
		desc     string
		path     string
		counters []*ygnmi.Value[uint64]
	}
	interfaces := sortedInterfaces(dut.Ports())
	t.Logf("Interfaces: %s", interfaces)
	for _, intf := range interfaces {
		t.Run(intf, func(t *testing.T) {
			qosInterface := gnmi.OC().Qos().Interface(intf)
			if deviations.QOSInQueueDropCounterUnsupported(dut) {
				t.Skipf("INFO: Skipping test due to %s does not support Queue Input Dropped packets", dut.Vendor())
				counters := gnmi.LookupAll(t, dut, qosInterface.Input().QueueAny().DroppedPkts().State())
				t.Logf("counters: %s", counters)
				if len(counters) == 0 {
					t.Errorf("%s Interface Queue Input Dropped packets Telemetry Value is not present", intf)
				}
				for queueID, dropPkt := range counters {
					dropCount, present := dropPkt.Val()
					if !present {
						t.Errorf("%s Interface %s Telemetry Value is not present", intf, dropPkt.Path)
					} else {
						t.Logf("%s Interface %s, Queue %d has %d drop(s)", dropPkt.Path.GetOrigin(), intf, queueID, dropCount)
					}
				}
			}
			cases := []testCase{
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
				t.Run(c.desc, func(t *testing.T) {
					if dut.Vendor() == ondatra.JUNIPER && c.desc == "Queue Input Dropped packets" {
						t.Skipf("INFO: Skipping test due to %s does not support %s", dut.Vendor(), c.path)
					}
					if deviations.QOSVoqDropCounterUnsupported(dut) && c.desc == "Queue input voq-output-interface dropped packets" {
						t.Skipf("INFO: Skipping test due to deviation qos_voq_drop_counter_unsupported")
					}
					if len(c.counters) == 0 {
						t.Errorf("%s Interface %s Telemetry Value is not present", c.desc, intf)
					}
					for queueID, dropPkt := range c.counters {
						dropCount, present := dropPkt.Val()
						if !present {
							t.Errorf("%s Interface %s %s Telemetry Value is not present", c.desc, intf, dropPkt.Path)
						} else {
							t.Logf("%s Interface %s, Queue %d has %d drop(s)", dropPkt.Path.GetOrigin(), intf, queueID, dropCount)
						}
					}
				})
			}
		})
	}
}

func TestNoAsicDrop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().State()
	asicDrops := gnmi.LookupAll(t, dut, query)
	if len(asicDrops) == 0 {
		t.Fatalf("ERROR: No asic drop path exist")
	}

	for _, asicDrop := range asicDrops {
		component := asicDrop.Path.GetElem()[1].GetKey()["name"]
		t.Run("Component "+component, func(t *testing.T) {
			drop, _ := asicDrop.Val()
			if drop.InterfaceBlock != nil {
				if drop.InterfaceBlock.InDrops == nil {
					t.Errorf("ERROR: InDrops counter is not present")
				} else {
					t.Logf("INFO: InDrops counter: %d", drop.InterfaceBlock.GetInDrops())
				}
				if drop.InterfaceBlock.OutDrops == nil {
					t.Errorf("ERROR: OutDrops counter is not present")
				} else {
					t.Logf("INFO: OutDrops counter: %d", drop.InterfaceBlock.GetInDrops())
				}
				if drop.InterfaceBlock.Oversubscription == nil {
					t.Errorf("ERROR: Oversubscription counter is not present")
				} else {
					t.Logf("INFO: Oversubscription counter: %d", drop.InterfaceBlock.GetInDrops())
				}
			}
			if drop.LookupBlock != nil {
				if drop.LookupBlock.AclDrops == nil {
					t.Errorf("ERROR: AclDrops counter is not present")
				} else {
					t.Logf("INFO: AclDrops counter: %d", drop.LookupBlock.GetAclDrops())
				}
				if drop.LookupBlock.ForwardingPolicy == nil {
					t.Errorf("ERROR: ForwardingPolicy is not present")
				} else {
					t.Logf("INFO: GetForwardingPolicy counter: %d", drop.LookupBlock.GetForwardingPolicy())
				}
				if drop.LookupBlock.FragmentTotalDrops == nil {
					t.Errorf("ERROR: FragmentTotalDrops is not present")
				} else {
					t.Logf("INFO: FragmentTotalDrops: %d", drop.LookupBlock.GetFragmentTotalDrops())
				}
				if drop.LookupBlock.IncorrectSoftwareState == nil {
					t.Errorf("ERROR: IncorrectSoftwareState counter is not present")
				} else {
					t.Logf("INFO: IncorrectSoftwareState counter: %d", drop.LookupBlock.GetIncorrectSoftwareState())
				}
				if drop.LookupBlock.InvalidPacket == nil {
					t.Errorf("ERROR: InvalidPacket counter is not present")
				} else {
					t.Logf("INFO: InvalidPacket counter: %d", drop.LookupBlock.GetInvalidPacket())
				}
				if drop.LookupBlock.LookupAggregate == nil {
					t.Errorf("ERROR: LookupAggregate counter is not present")
				} else {
					t.Logf("INFO: LookupAggregate counter: %d", drop.LookupBlock.GetLookupAggregate())
				}
				if drop.LookupBlock.NoLabel == nil {
					t.Errorf("ERROR: NoLabel counter is not present")
				} else {
					t.Logf("INFO: NoLabel counter: %d", drop.LookupBlock.GetNoLabel())
				}
				if drop.LookupBlock.NoNexthop == nil {
					t.Errorf("ERROR: NoNexthop counter is not present")
				} else {
					t.Logf("INFO: NoNexthop counter: %d", drop.LookupBlock.GetNoNexthop())
				}
				if drop.LookupBlock.NoRoute == nil {
					t.Errorf("ERROR: NoRoute counter is not present")
				} else {
					t.Logf("INFO: NoRoute counter: %d", drop.LookupBlock.GetNoNexthop())
				}
				if drop.LookupBlock.RateLimit == nil {
					t.Errorf("ERROR: RateLimit counter is not present")
				} else {
					t.Logf("INFO: RateLimit counter: %d", drop.LookupBlock.GetRateLimit())
				}
			}
		})
	}
}

func TestInterfaceStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	interfaces := sortedInterfaces(dut.Ports())
	t.Logf("Interfaces: %s", interfaces)
	for _, intf := range interfaces {
		t.Run(intf, func(t *testing.T) {
			query := gnmi.OC().Interface(intf).State()
			root := gnmi.Get(t, dut, query)
			t.Logf("INFO: Interface %s: \n", intf)
			if root.OperStatus == oc.Interface_OperStatus_NOT_PRESENT {
				t.Errorf("ERROR: Oper status is not present")
			} else {
				t.Logf("INFO: Oper status: %s", root.GetOperStatus())
			}
			if root.AdminStatus == oc.Interface_AdminStatus_UNSET {
				t.Errorf("ERROR: Admin status is not set")
			} else {
				t.Logf("INFO: Admin status: %s", root.GetAdminStatus())
			}
			if root.Type == oc.IETFInterfaces_InterfaceType_UNSET {
				t.Errorf("ERROR: Type is not present")
			} else {
				t.Logf("INFO: Type: %s", root.GetType())
			}
			if root.Description == nil {
				t.Errorf("ERROR: Description is not present")
			} else {
				t.Logf("INFO: Description: %s", root.GetType())
			}
			if root.GetCounters().OutOctets == nil {
				t.Errorf("ERROR: Counter OutOctets is not present")
			} else {
				t.Logf("INFO: Counter OutOctets: %d", root.GetCounters().GetOutOctets())
			}
			if root.GetCounters().InMulticastPkts == nil {
				t.Errorf("ERROR: Counter InMulticastPkts is not present")
			} else {
				t.Logf("INFO: Counter InMulticastPkts: %d", root.GetCounters().GetInMulticastPkts())
			}
			if root.GetCounters().InDiscards == nil {
				t.Errorf("ERROR: Counter InDiscards is not present")
			} else {
				t.Logf("INFO: Counter InDiscards: %d", root.GetCounters().GetInDiscards())
			}
			if root.GetCounters().InErrors == nil {
				t.Errorf("ERROR: Counter InErrors is not present")
			} else {
				t.Logf("INFO: Counter InErrors: %d", root.GetCounters().GetInErrors())
			}
			if root.GetCounters().InUnknownProtos == nil {
				t.Errorf("ERROR: Counter InUnknownProtos is not present")
			} else {
				t.Logf("INFO: Counter InUnknownProtos: %d", root.GetCounters().GetInUnknownProtos())
			}
			if root.GetCounters().OutDiscards == nil {
				t.Errorf("ERROR: Counter OutDiscards is not present")
			} else {
				t.Logf("INFO: Counter OutDiscards: %d", root.GetCounters().GetOutDiscards())
			}
			if root.GetCounters().OutErrors == nil {
				t.Errorf("ERROR: Counter OutErrors is not present")
			} else {
				t.Logf("INFO: Counter OutErrors: %d", root.GetCounters().GetOutErrors())
			}
			if root.GetCounters().InFcsErrors == nil {
				t.Errorf("ERROR: Counter InFcsErrors is not present")
			} else {
				t.Logf("INFO: Counter InFcsErrors: %d", root.GetCounters().GetInFcsErrors())
			}
		})
	}
}

func TestInterfacesubIntfs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	interfaces := sortedInterfaces(dut.Ports())
	t.Logf("Interfaces: %s", interfaces)
	for _, intf := range interfaces {
		t.Run(intf, func(t *testing.T) {
			subIntfIndexes := gnmi.LookupAll(t, dut, gnmi.OC().Interface(intf).SubinterfaceAny().Index().State())
			for _, index := range subIntfIndexes {
				subIntfIndex, present := index.Val()
				if !present {
					t.Fatalf("ERROR: subIntf index value doesn't exist")
				}
				subIntfPath := gnmi.OC().Interface(intf).Subinterface(subIntfIndex)
				IntfPath := gnmi.OC().Interface(intf)
				subIntfState := gnmi.Get(t, dut, subIntfPath.State())
				subIntf := subIntfState.GetName()

				t.Run(subIntf, func(t *testing.T) {
					intfState := gnmi.Get(t, dut, gnmi.OC().Interface(intf).State())
					if subIntfState.OperStatus == oc.Interface_OperStatus_NOT_PRESENT {
						t.Errorf("ERROR: Oper status is not up")
					} else {
						t.Logf("INFO: Oper status: %s", subIntfState.GetOperStatus())
					}

					if subIntfState.AdminStatus == oc.Interface_AdminStatus_UNSET {
						t.Errorf("ERROR: Admin status is not up")
					} else {
						t.Logf("INFO: Admin status: %s", subIntfState.GetAdminStatus())
					}

					if subIntfState.Description == nil {
						t.Errorf("ERROR: Description is not present")
					} else {
						t.Logf("INFO: Description: %s", subIntfState.GetDescription())
					}

					if subIntfState.GetCounters().OutOctets == nil && intfState.GetCounters().OutOctets == nil {
						t.Errorf("ERROR: Counter OutOctets is not present on interface %s, %s", subIntf, intf)
					}

					if subIntfState.GetCounters().InMulticastPkts == nil && intfState.GetCounters().InMulticastPkts == nil {
						t.Errorf("ERROR: Counter InMulticastPkts is not present on interface %s, %s", subIntf, intf)
					}

					counters := IntfPath.Counters()
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
					t.Logf("Verifying counters for Interfaces: %s", interfaces)
					for _, c := range cases {
						t.Run(c.desc, func(t *testing.T) {
							if val, present := gnmi.Lookup(t, dut, c.counter).Val(); present {
								t.Logf("INFO: %s: %d", c.counter, val)
							} else if pVal, pPresent := gnmi.Lookup(t, dut, c.parentCounter).Val(); pPresent {
								t.Logf("INFO: %s: %d", c.parentCounter, pVal)
							} else {
								t.Errorf("ERROR: Neither %s nor %s is present", c.counter, c.parentCounter)
							}
						})
					}
				})
			}
		})
	}
}

func TestInterfaceEthernetNoDrop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	interfaces := sortedInterfaces(dut.Ports())
	t.Logf("Interfaces: %s", interfaces)
	for _, intf := range interfaces {
		t.Run(intf, func(t *testing.T) {
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
					if val, present := gnmi.Lookup(t, dut, c.counter).Val(); present {
						t.Logf("INFO: %s: %d", c.counter, val)
					} else {
						t.Errorf("ERROR: %s is not present", c.counter)
					}
				})
			}
		})
	}
}

func TestSystemAlarms(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	query := gnmi.OC().System().AlarmAny().State()
	alarms := gnmi.LookupAll(t, dut, query)
	if len(alarms) > 0 {
		for _, a := range alarms {
			val, _ := a.Val()
			alarmSeverity := val.GetSeverity()
			// Checking for major system alarms.
			if alarmSeverity == oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_MAJOR {
				t.Logf("INFO: System Alarm with severity %s seen: %s", alarmSeverity, val.GetText())
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
		if fabricBlock.LostPackets == nil {
			t.Errorf("ERROR: Fabric drops is not present")
		} else {
			t.Logf("INFO: Fabric drops: %d", drop)
		}
	}
}
