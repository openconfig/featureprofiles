// Copyright 2022 Google LLC
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

package telemetry_basic_check_test

import (
	"math"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"golang.org/x/exp/slices"
)

const (
	ethernetCsmacd  = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	adminStatusUp   = oc.Interface_AdminStatus_UP
	adminStatusDown = oc.Interface_AdminStatus_DOWN
	operStatusUp    = oc.Interface_OperStatus_UP
	operStatusDown  = oc.Interface_OperStatus_DOWN
	maxPortVal      = "FFFFFEFF" // Maximum Port Value : https://github.com/openconfig/public/blob/2049164a8bca4cc9f11ffb313ef25c0e87303a24/release/models/p4rt/openconfig-p4rt.yang#L63-L81
)

var (
	vendorQueueNo = map[ondatra.Vendor]int{
		ondatra.ARISTA:  16,
		ondatra.CISCO:   6,
		ondatra.JUNIPER: 8,
		ondatra.NOKIA:   16,
	}
)

const (
	chassisType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS
	supervisorType  = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	linecardType    = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
	powerSupplyType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_POWER_SUPPLY
	fabricType      = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC
	switchChipType  = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT
	cpuType         = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CPU
	portType        = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT
	osType          = oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_OPERATING_SYSTEM
)

var portSpeed = map[ondatra.Speed]oc.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed10Gb:  oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology:
//   ate:port1 <--> port1:dut:port2 <--> ate:port2
//
// Test notes:
//
//  - P4RT needs to be configired and enabled during the binding.
//      p4-runtime
//        no shutdown
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestEthernetPortSpeed(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp)
	}
	want := portSpeed[dp.Speed()]
	got := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().PortSpeed().State())
	t.Logf("Got %s PortSpeed from telmetry: %v, expected: %v", dp.Name(), got, want)
	if got != want {
		t.Errorf("Get(DUT port1 PortSpeed): got %v, want %v", got, want)
	}
}

func TestEthernetMacAddress(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	macRegexp := "^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$"
	r, err := regexp.Compile(macRegexp)
	if err != nil {
		t.Fatalf("Cannot compile regular expression: %v", err)
	}
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().MacAddress().State())
	t.Logf("Got %s MacAddress from telmetry: %v", dp.Name(), macAddress)
	if len(r.FindString(macAddress)) == 0 {
		t.Errorf("Get(DUT port1 MacAddress): got %v, want matching regexp %v", macAddress, macRegexp)
	}
}

func TestInterfaceAdminStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp)
	}
	adminStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).AdminStatus().State())
	t.Logf("Got %s AdminStatus from telmetry: %v", dp.Name(), adminStatus)
	if adminStatus != adminStatusUp {
		t.Errorf("Get(DUT port1 OperStatus): got %v, want %v", adminStatus, adminStatusUp)
	}
}

func TestInterfaceOperStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp)
	}
	operStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State())
	t.Logf("Got %s OperStatus from telmetry: %v", dp.Name(), operStatus)
	if operStatus != operStatusUp {
		t.Errorf("Get(DUT port1 OperStatus): got %v, want %v", operStatus, operStatusUp)
	}
}

func TestInterfacePhysicalChannel(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	phyChannel := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).PhysicalChannel().State())
	t.Logf("Got %q PhysicalChannel from telmetry: %v", dp.Name(), phyChannel)
	if len(phyChannel) == 0 {
		t.Errorf("Get(DUT port1 PhysicalChannel): got empty %v, want non-empty list", phyChannel)
	}
}

func TestInterfaceStatusChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())

	cases := []struct {
		desc                string
		IntfStatus          bool
		expectedAdminStatus oc.E_Interface_AdminStatus
		expectedOperStatus  oc.E_Interface_OperStatus
	}{{
		desc:                "Disable the interface",
		IntfStatus:          false,
		expectedAdminStatus: adminStatusDown,
		expectedOperStatus:  operStatusDown,
	}, {
		desc:                "Re-enable the interface",
		IntfStatus:          true,
		expectedAdminStatus: adminStatusUp,
		expectedOperStatus:  operStatusUp,
	}}
	for _, tc := range cases {
		t.Log(tc.desc)
		intUpdateTime := 2 * time.Minute
		t.Run(tc.desc, func(t *testing.T) {
			i.Enabled = ygot.Bool(tc.IntfStatus)
			i.Type = ethernetCsmacd
			gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
			if deviations.ExplicitPortSpeed(dut) {
				fptest.SetPortSpeed(t, dp)
			}
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, tc.expectedOperStatus)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).AdminStatus().State(), intUpdateTime, tc.expectedAdminStatus)
			operStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State())
			if want := tc.expectedOperStatus; operStatus != want {
				t.Errorf("Get(DUT port1 oper status): got %v, want %v", operStatus, want)
			}
			adminStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).AdminStatus().State())
			if want := tc.expectedAdminStatus; adminStatus != want {
				t.Errorf("Get(DUT port1 admin status): got %v, want %v", adminStatus, want)
			}
		})
	}
}

func TestHardwarePort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	// Verify HardwarePort leaf is present under interface.
	got := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).HardwarePort().State())
	val, present := got.Val()
	if !present {
		t.Errorf("DUT port1 %s HardwarePort leaf not found", dp.Name())
	}
	t.Logf("For interface %s, HardwarePort is %s", dp.Name(), val)

	// Verify HardwarePort is a component of type PORT.
	typeGot := gnmi.Get(t, dut, gnmi.OC().Component(val).Type().State())
	if typeGot != portType {
		t.Errorf("HardwarePort leaf's component type got %s, want %s", typeGot, portType)
	}

	// Verify HardwarePort component has CHASSIS as an ancestor.
	verifyChassisIsAncestor(t, dut, val)
}

func TestInterfaceCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	counters := gnmi.OC().Interface(dp.Name()).Counters()
	intCounterPath := "/interfaces/interface/state/counters/"

	cases := []struct {
		desc    string
		path    string
		counter *ygnmi.Value[uint64]
	}{{
		desc:    "InUnicastPkts",
		path:    intCounterPath + "in-unicast-pkts",
		counter: gnmi.Lookup(t, dut, counters.InUnicastPkts().State()),
	}, {
		desc:    "InOctets",
		path:    intCounterPath + "in-octets",
		counter: gnmi.Lookup(t, dut, counters.InOctets().State()),
	}, {
		desc:    "InMulticastPkts",
		path:    intCounterPath + "in-multicast-pkts",
		counter: gnmi.Lookup(t, dut, counters.InMulticastPkts().State()),
	}, {
		desc:    "InBroadcastPkts",
		path:    intCounterPath + "in-broadcast-pkts",
		counter: gnmi.Lookup(t, dut, counters.InBroadcastPkts().State()),
	}, {
		desc:    "InDiscards",
		path:    intCounterPath + "in-discards",
		counter: gnmi.Lookup(t, dut, counters.InDiscards().State()),
	}, {
		desc:    "InErrors",
		path:    intCounterPath + "in-errors",
		counter: gnmi.Lookup(t, dut, counters.InErrors().State()),
	}, {
		desc:    "InFcsErrors",
		path:    intCounterPath + "in-fcs-errors",
		counter: gnmi.Lookup(t, dut, counters.InFcsErrors().State()),
	}, {
		desc:    "OutUnicastPkts",
		path:    intCounterPath + "out-unicast-pkts",
		counter: gnmi.Lookup(t, dut, counters.OutUnicastPkts().State()),
	}, {
		desc:    "OutOctets",
		path:    intCounterPath + "out-octets",
		counter: gnmi.Lookup(t, dut, counters.OutOctets().State()),
	}, {
		desc:    "OutMulticastPkts",
		path:    intCounterPath + "out-broadcast-pkts",
		counter: gnmi.Lookup(t, dut, counters.OutMulticastPkts().State()),
	}, {
		desc:    "OutBroadcastPkts",
		path:    intCounterPath + "out-multicast-pkts",
		counter: gnmi.Lookup(t, dut, counters.OutBroadcastPkts().State()),
	}, {
		desc:    "OutDiscards",
		path:    intCounterPath + "out-discards",
		counter: gnmi.Lookup(t, dut, counters.OutDiscards().State()),
	}, {
		desc:    "OutErrors",
		path:    intCounterPath + "out-errors",
		counter: gnmi.Lookup(t, dut, counters.OutErrors().State()),
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			val, present := tc.counter.Val()
			if !present {
				t.Errorf("Get IsPresent status for path %q: got false, want true", tc.path)
			}
			t.Logf("Got path/value: %s:%d", tc.path, val)
		})
	}
}

func TestQoSCounters(t *testing.T) {
	if !*args.QoSBaseConfigPresent {
		t.Skipf("Test is skipped, since the related base config for QoS is not loaded.")
	}
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	queues := gnmi.OC().Qos().Interface(dp.Name()).Output().QueueAny()
	qosQueuePath := "/qos/interfaces/interface/output/queues/queue/state/"

	cases := []struct {
		desc     string
		path     string
		counters []*ygnmi.Value[uint64]
	}{{
		desc:     "TransmitPkts",
		path:     qosQueuePath + "transmit-pkts",
		counters: gnmi.LookupAll(t, dut, queues.TransmitPkts().State()),
	}, {
		desc:     "TransmitOctets",
		path:     qosQueuePath + "transmit-octets",
		counters: gnmi.LookupAll(t, dut, queues.TransmitOctets().State()),
	}, {
		desc:     "DroppedPkts",
		path:     qosQueuePath + "dropped-pkts",
		counters: gnmi.LookupAll(t, dut, queues.DroppedPkts().State()),
	}}
	if !deviations.QOSDroppedOctets(dut) {
		cases = append(cases,
			struct {
				desc     string
				path     string
				counters []*ygnmi.Value[uint64]
			}{
				desc:     "DroppedOctets",
				path:     qosQueuePath + "dropped-octets",
				counters: gnmi.LookupAll(t, dut, queues.DroppedOctets().State()),
			})
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			if len(tc.counters) != vendorQueueNo[dut.Vendor()] {
				t.Errorf("Get QoS queue# for %q: got %d, want %d", dut.Vendor(), len(tc.counters), vendorQueueNo[dut.Vendor()])
			}
			for i, counter := range tc.counters {
				val, present := counter.Val()
				if !present {
					t.Errorf("counter.IsPresent() for queue %d): got false, want true", i)
				}
				t.Logf("Got queue %d path/value: %s:%d", i, tc.path, val)
			}
		})
	}
}

func findComponentsListByType(t *testing.T, dut *ondatra.DUTDevice) map[string][]string {
	t.Helper()
	componentType := map[string]oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT{
		"Fabric":      fabricType,
		"Linecard":    linecardType,
		"PowerSupply": powerSupplyType,
		"Supervisor":  supervisorType,
		"SwitchChip":  switchChipType,
	}
	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	s := make(map[string][]string)
	for comp := range componentType {
		for _, c := range components {
			if c.GetType() == nil {
				t.Logf("Component %s type is missing from telemetry", c.GetName())
				continue
			}
			t.Logf("Component %s has type: %v", c.GetName(), c.GetType())
			if v := c.GetType(); v == componentType[comp] {
				s[comp] = append(s[comp], c.GetName())
			}
		}
	}
	return s
}

// verifyChassisIsAncestor verifies that a given component has
// a component of type CHASSIS as an ancestor.
func verifyChassisIsAncestor(t *testing.T, dut *ondatra.DUTDevice, comp string) {
	visited := make(map[string]bool)
	for curr := comp; ; {
		if visited[curr] {
			t.Errorf("Component %s already visited; loop detected in the hierarchy.", curr)
			break
		}
		visited[curr] = true
		parent := gnmi.Lookup(t, dut, gnmi.OC().Component(curr).Parent().State())
		val, present := parent.Val()
		if !present {
			t.Errorf("Chassis component NOT found as an ancestor of component %s", comp)
			break
		}
		got := gnmi.Get(t, dut, gnmi.OC().Component(val).Type().State())
		if got == chassisType {
			t.Logf("Found chassis component as an ancestor of component %s", comp)
			break
		}
		// Not reached chassis yet; go one level up.
		curr = gnmi.Get(t, dut, gnmi.OC().Component(val).Name().State())
	}
}

func TestComponentParent(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	componentParent := map[string]oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT{
		"Fabric":      chassisType,
		"Linecard":    chassisType,
		"PowerSupply": chassisType,
		"Supervisor":  chassisType,
		"SwitchChip":  linecardType,
	}
	compList := findComponentsListByType(t, dut)
	cases := []struct {
		desc          string
		componentType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT
		parent        oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT
	}{{
		desc:          "Fabric",
		componentType: fabricType,
		parent:        componentParent["Fabric"],
	}, {
		desc:          "Linecard",
		componentType: linecardType,
		parent:        componentParent["Linecard"],
	}, {
		desc:          "PowerSupply",
		componentType: powerSupplyType,
		parent:        componentParent["PowerSupply"],
	}, {
		desc:          "Supervisor",
		componentType: supervisorType,
		parent:        componentParent["Supervisor"],
	}, {
		desc:          "SwitchChip",
		componentType: switchChipType,
		parent:        componentParent["SwitchChip"],
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			if len(compList[tc.desc]) == 0 && dut.Model() == "DCS-7280CR3K-32D4" {
				t.Skipf("Test of %v is skipped due to hardware platform compatibility", tc.componentType)
			}

			t.Logf("Found component list for type %v : %v", tc.componentType, compList[tc.desc])
			if len(compList[tc.desc]) == 0 {
				t.Fatalf("Get component list for %q: got 0, want > 0", dut.Model())
			}
			// Validate parent component.
			for _, comp := range compList[tc.desc] {
				t.Logf("Validate component %s", comp)
				verifyChassisIsAncestor(t, dut, comp)
			}
		})
	}
}

func TestSoftwareVersion(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	if deviations.SwVersionUnsupported(dut) {
		t.Skipf("Software version test is not supported by DUT")
	}

	// validate /system/state/software-version.
	swVer := gnmi.Lookup(t, dut, gnmi.OC().System().SoftwareVersion().State())
	if v, ok := swVer.Val(); ok && v != "" {
		t.Logf("Got a system software version value %q", v)
	} else {
		t.Errorf("System software version was not reported")
	}

	// validate OPERATING_SYSTEM component(s).
	osList := components.FindSWComponentsByType(t, dut, osType)
	if len(osList) == 0 {
		t.Fatalf("Get OS component list: got 0, want > 0")
	}
	t.Logf("Found OS component list: %v", osList)

	for _, os := range osList {
		swVer = gnmi.Lookup(t, dut, gnmi.OC().Component(os).SoftwareVersion().State())
		if v, ok := swVer.Val(); ok && v != "" {
			t.Logf("Got a system software version value %q for component %v", v, os)
		} else {
			t.Errorf("System software version was not reported for component %v", v)
		}

		// validate OPERATING_SYSTEM component parent.
		parent := gnmi.Lookup(t, dut, gnmi.OC().Component(os).Parent().State())
		if v, ok := parent.Val(); ok {
			got := gnmi.Get(t, dut, gnmi.OC().Component(v).Type().State())

			want := []oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT{supervisorType}
			if deviations.OSComponentParentIsChassis(dut) {
				want = []oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT{chassisType}
			}
			if deviations.OSComponentParentIsSupervisorOrLinecard(dut) {
				want = []oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT{supervisorType, linecardType}
			}

			if slices.IndexFunc(want, func(w oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) bool {
				return w == got
			}) == -1 {
				t.Errorf("Got a parent %v with a type %v for the component %v, want one of %v", v, got, os, want)
			} else {
				t.Logf("Got a valid parent %v with a type %v for the component %v", v, got, os)
			}
		} else {
			t.Errorf("Parent for the component %v was not found", os)
		}
	}
}

func TestCPU(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	cpus := components.FindComponentsByType(t, dut, cpuType)
	t.Logf("Found CPU list: %v", cpus)
	if len(cpus) == 0 {
		t.Fatalf("Get CPU list for %q: got 0, want > 0", dut.Model())
	}

	for _, cpu := range cpus {
		t.Logf("Validate CPU: %s", cpu)
		component := gnmi.OC().Component(cpu)
		if !gnmi.Lookup(t, dut, component.Description().State()).IsPresent() {
			t.Errorf("component.Description().Lookup(t).IsPresent() for %q: got false, want true", cpu)
		} else {
			t.Logf("CPU %s Description: %s", cpu, gnmi.Get(t, dut, component.Description().State()))
		}
	}
}

func TestSupervisorLastRebootInfo(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	if dut.Model() == "DCS-7280CR3K-32D4" {
		t.Skipf("Test is skipped due to hardware platform compatibility")
	}

	cards := components.FindComponentsByType(t, dut, supervisorType)
	t.Logf("Found card list: %v", cards)
	if len(cards) == 0 {
		t.Fatalf("Get Card list for %q: got 0, want > 0", dut.Model())
	}

	rebootTimeFound := false
	rebootReasonFound := false
	for _, card := range cards {
		t.Logf("Validate card %s", card)
		rebootTime := gnmi.OC().Component(card).LastRebootTime()
		if gnmi.Lookup(t, dut, rebootTime.State()).IsPresent() {
			t.Logf("Hardware card %s reboot time: %v", card, gnmi.Get(t, dut, rebootTime.State()))
			rebootTimeFound = true
		}

		rebootReason := gnmi.OC().Component(card).LastRebootReason()
		if gnmi.Lookup(t, dut, rebootReason.State()).IsPresent() {
			t.Logf("Hardware card %s reboot reason: %v", card, gnmi.Get(t, dut, rebootReason.State()))
			rebootReasonFound = true
		}
	}

	if !rebootTimeFound {
		t.Errorf("rebootTime.Lookup(t).IsPresent(): got %v, want %v", rebootTimeFound, !rebootTimeFound)
	}
	if !rebootReasonFound {
		t.Errorf("rebootReason.Lookup(t).IsPresent(): got %v, want %v", rebootReasonFound, !rebootReasonFound)
	}
}

func TestAFT(t *testing.T) {
	// TODO: Remove t.Skipf() after the issue is fixed.
	t.Skipf("Test is skipped due to the failure")

	dut := ondatra.DUT(t, "dut")
	afts := gnmi.LookupAll(t, dut, gnmi.OC().NetworkInstanceAny().Afts().State())

	if len(afts) == 0 {
		t.Errorf("Afts().Lookup(t) for %q: got 0, want > 0", dut.Name())
	}
	for i, aft := range afts {
		val, present := aft.Val()
		if !present {
			t.Errorf("aft.IsPresent() for %q: got false, want true", dut.Name())
		}
		t.Logf("Telemetry path/value %d: %v=>%v:", i, aft.Path.String(), val)
	}
}

func TestLacpMember(t *testing.T) {
	if !*args.LACPBaseConfigPresent {
		t.Skipf("Test is skipped, since the related base config for LACP is not present")
	}
	dut := ondatra.DUT(t, "dut")
	lacpIntfs := gnmi.GetAll(t, dut, gnmi.OC().Lacp().InterfaceAny().Name().State())
	if len(lacpIntfs) == 0 {
		t.Errorf("Lacp().InterfaceAny().Name().Get(t) for %q: got 0, want > 0", dut.Name())
	}
	t.Logf("Found %d LACP interfaces: %v", len(lacpIntfs)+1, lacpIntfs)

	for i, intf := range lacpIntfs {
		t.Logf("Telemetry LACP interface %d: %s:", i, intf)
		members := gnmi.LookupAll(t, dut, gnmi.OC().Lacp().Interface(intf).MemberAny().State())
		if len(members) == 0 {
			t.Errorf("MemberAny().Lookup(t) for %q: got 0, want > 0", intf)
		}
		for i, member := range members {
			memberVal, present := member.Val()
			if !present {
				t.Errorf("member.IsPresent() for %q: got false, want true", intf)
			}
			t.Logf("Telemetry path/value %d: %v=>%v:", i, member.Path.String(), memberVal)

			// Check LACP packet counters.
			counters := memberVal.GetCounters()

			lacpInPkts := counters.GetLacpInPkts()
			if lacpInPkts == 0 {
				t.Errorf("counters.GetLacpInPkts() for %q: got 0, want >0", memberVal.GetInterface())
			}
			t.Logf("counters.GetLacpInPkts() for %q: %d", memberVal.GetInterface(), lacpInPkts)

			lacpOutPkts := counters.GetLacpOutPkts()
			if lacpOutPkts == 0 {
				t.Errorf("counters.GetLacpOutPkts() for %q: got 0, want >0", memberVal.GetInterface())
			}
			t.Logf("counters.GetLacpOutPkts() for %q: %d", memberVal.GetInterface(), lacpOutPkts)

			// Check LACP interface status.
			if !memberVal.GetAggregatable() {
				t.Errorf("memberVal.GetAggregatable() for %q: got false, want true", memberVal.GetInterface())
			}
			t.Logf("memberVal.GetAggregatable() for %q: %v", memberVal.GetInterface(), memberVal.GetAggregatable())

			if !memberVal.GetCollecting() {
				t.Errorf("memberVal.GetCollecting() for %q: got false, want true", memberVal.GetInterface())
			}
			t.Logf("memberVal.GetCollecting() for %q: %v", memberVal.GetInterface(), memberVal.GetAggregatable())

			if !memberVal.GetDistributing() {
				t.Errorf("memberVal.GetDistributing() for %q: got false, want true", memberVal.GetInterface())
			}
			t.Logf("memberVal.GetDistributing() for %q: %v", memberVal.GetInterface(), memberVal.GetAggregatable())

			// Check LCP partner info.
			if memberVal.GetPartnerId() == "" {
				t.Errorf("memberVal.GetPartnerId() for %q: got empty string, want non-empty string", memberVal.GetInterface())
			}
			t.Logf("memberVal.GetPartnerId() for %q: %s", memberVal.GetInterface(), memberVal.GetPartnerId())

			if memberVal.GetPartnerKey() == 0 {
				t.Errorf("memberVal.GetPartnerKey() for %q: got 0, want > 0", memberVal.GetInterface())
			}
			t.Logf("memberVal.GetPartnerKey() for %q: %d", memberVal.GetInterface(), memberVal.GetPartnerKey())

			if memberVal.GetPartnerPortNum() == 0 {
				t.Errorf("memberVal.GetPartnerPortNum() for %q: got 0, want > 0", memberVal.GetInterface())
			}
			t.Logf("memberVal.GetPartnerPortNum() for %q: %d", memberVal.GetInterface(), memberVal.GetPartnerPortNum())
		}
	}
}

func TestP4rtInterfaceID(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())

	maxPortValDec, err := strconv.ParseUint(maxPortVal, 16, 32)
	if err != nil {
		t.Fatalf("Error while converting portID value: %v", err)
	}

	cases := []struct {
		desc   string
		portID uint32
	}{{
		desc:   "MinPortID",
		portID: 1,
	}, {
		desc:   "AvePortID",
		portID: uint32(maxPortValDec) / 2,
	}, {
		desc:   "MaxPortID",
		portID: uint32(maxPortValDec),
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			i.Id = ygot.Uint32(tc.portID)
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
			if deviations.ExplicitPortSpeed(dut) {
				fptest.SetPortSpeed(t, dp)
			}
			// Check path /interfaces/interface/state/id.
			intfID := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Id().State())
			intfVal, present := intfID.Val()
			if !present {
				t.Fatalf("intfID.IsPresent() for %q: got false, want true", dp.Name())
			}
			t.Logf("Telemetry path/value: %v=>%v:", intfID.Path.String(), intfVal)
			if intfVal != tc.portID {
				t.Fatalf("intfID.Val(t) for %q: got %d, want %d", dp.Name(), intfVal, tc.portID)
			}
		})
	}
}

func TestP4rtNodeID(t *testing.T) {
	// TODO: add p4rtNodeName to Ondatra's netutil
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	nodes := P4RTNodesByPort(t, dut)
	ic := d.GetOrCreateComponent(nodes["port1"]).GetOrCreateIntegratedCircuit()

	cases := []struct {
		desc   string
		nodeID uint64
	}{{
		desc:   "MinNodeID",
		nodeID: 1,
	}, {
		desc:   "AveNodeID",
		nodeID: math.MaxUint64 / 2,
	}, {
		desc:   "MaxNodeID",
		nodeID: math.MaxUint64,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ic.NodeId = ygot.Uint64(tc.nodeID)

			if _, ok := nodes["port1"]; !ok {
				t.Fatalf("Couldn't find P4RT Node for port: %s", "port1")
			}
			t.Logf("Configuring P4RT Node: %s", nodes["port1"])
			gnmi.Replace(t, dut, gnmi.OC().Component(nodes["port1"]).Config(), &oc.Component{
				Name:              ygot.String(nodes["port1"]),
				IntegratedCircuit: ic,
			})
			// Check path /components/component/integrated-circuit/state/node-id.
			nodeID := gnmi.Lookup(t, dut, gnmi.OC().Component(nodes["port1"]).IntegratedCircuit().NodeId().State())
			nodeIDVal, present := nodeID.Val()
			if !present {
				t.Fatalf("nodeID.IsPresent() for %q: got false, want true", nodes["port1"])
			}
			t.Logf("Telemetry path/value: %v=>%v:", nodeID.Path.String(), nodeIDVal)
			if nodeIDVal != tc.nodeID {
				t.Fatalf("nodeID.Val(t) for %q: got %d, want %d", nodes["port1"], nodeIDVal, tc.nodeID)
			}
		})
	}
}

func fetchInAndOutPkts(t *testing.T, dut *ondatra.DUTDevice, dp1, dp2 *ondatra.Port) (uint64, uint64) {
	if deviations.InterfaceCountersFromContainer(dut) {
		inPkts := *gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Counters().State()).InUnicastPkts
		outPkts := *gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Counters().State()).OutUnicastPkts
		return inPkts, outPkts
	}

	inPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Counters().InUnicastPkts().State())
	outPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Counters().OutUnicastPkts().State())
	return inPkts, outPkts
}

func TestIntfCounterUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	config := gosnappi.NewConfig()
	config.Ports().Add().SetName(ap1.ID())
	intf1 := config.Devices().Add().SetName(ap1.Name())
	eth1 := intf1.Ethernets().Add().SetName(ap1.Name() + ".Eth").SetMac("02:00:01:01:01:01")
	eth1.Connection().SetPortName(ap1.ID())
	ip4_1 := eth1.Ipv4Addresses().Add().SetName(intf1.Name() + ".IPv4").
		SetAddress("198.51.100.1").SetGateway("198.51.100.0").
		SetPrefix(31)
	config.Ports().Add().SetName(ap2.ID())
	intf2 := config.Devices().Add().SetName(ap2.Name())
	eth2 := intf2.Ethernets().Add().SetName(ap2.Name() + ".Eth").SetMac("02:00:01:02:01:01")
	eth2.Connection().SetPortName(ap2.ID())
	ip4_2 := eth2.Ipv4Addresses().Add().SetName(intf2.Name() + ".IPv4").
		SetAddress("198.51.100.3").SetGateway("198.51.100.2").
		SetPrefix(31)

	flowName := "telemetry_test_flow"
	flowipv4 := config.Flows().Add().SetName(flowName)
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{intf1.Name() + ".IPv4"}).
		SetRxNames([]string{intf2.Name() + ".IPv4"})
	flowipv4.Size().SetFixed(100)
	flowipv4.Rate().SetPercentage(1)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(eth1.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(ip4_1.Address())
	v4.Dst().SetValue(ip4_2.Address())
	v4.Priority().Dscp().Phb().SetValue(56)
	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	t.Log("Running traffic on DUT interfaces: ", dp1, dp2)
	dutInPktsBeforeTraffic, dutOutPktsBeforeTraffic := fetchInAndOutPkts(t, dut, dp1, dp2)
	t.Log("inPkts and outPkts counters before traffic: ", dutInPktsBeforeTraffic, dutOutPktsBeforeTraffic)
	otg.StartTraffic(t)
	time.Sleep(10 * time.Second)
	otg.StopTraffic(t)

	ds1 := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; ds1 != want {
		t.Errorf("Get(DUT port1 status): got %v, want %v", ds1, want)
	}
	ds2 := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; ds2 != want {
		t.Errorf("Get(DUT port2 status): got %v, want %v", ds2, want)
	}
	// Verifying the ate port link state
	for _, p := range config.Ports().Items() {
		portMetrics := gnmi.Get(t, otg, gnmi.OTG().Port(p.Name()).State())
		if portMetrics.GetLink() != otgtelemetry.Port_Link_UP {
			t.Errorf("Get(ATE %v status): got %v, want %v", p.Name(), portMetrics.GetLink(), otgtelemetry.Port_Link_UP)
		}
	}

	otgutils.LogFlowMetrics(t, otg, config)
	ateInPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().InPkts().State()))
	ateOutPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State()))

	if ateOutPkts == 0 {
		t.Errorf("Get(out packets for flow %q: got %v, want nonzero", flowName, ateOutPkts)
	}
	lossPct := (ateOutPkts - ateInPkts) * 100 / ateOutPkts
	if lossPct >= 0.1 {
		t.Errorf("Get(traffic loss for flow %q: got %v, want < 0.1", flowName, lossPct)
	}
	dutInPktsAfterTraffic, dutOutPktsAfterTraffic := fetchInAndOutPkts(t, dut, dp1, dp2)
	t.Log("inPkts and outPkts counters after traffic: ", dutInPktsAfterTraffic, dutOutPktsAfterTraffic)

	if dutInPktsAfterTraffic-dutInPktsBeforeTraffic < uint64(ateInPkts) {
		t.Errorf("Get less inPkts from telemetry: got %v, want >= %v", dutInPktsAfterTraffic-dutInPktsBeforeTraffic, ateOutPkts)
	}
	if dutOutPktsAfterTraffic-dutOutPktsBeforeTraffic < uint64(ateOutPkts) {
		t.Errorf("Get less outPkts from telemetry: got %v, want >= %v", dutOutPktsAfterTraffic-dutOutPktsBeforeTraffic, ateOutPkts)
	}
}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	dutIntfs := []struct {
		desc      string
		intfName  string
		ipAddr    string
		prefixLen uint8
	}{{
		desc:      "Input interface port1",
		intfName:  dp1.Name(),
		ipAddr:    "198.51.100.0",
		prefixLen: 31,
	}, {
		desc:      "Output interface port2",
		intfName:  dp2.Name(),
		ipAddr:    "198.51.100.2",
		prefixLen: 31,
	}}

	// Configure the interfaces.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &oc.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		i.GetOrCreateEthernet()
		s := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s.Enabled = ygot.Bool(true)
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
		fptest.SetPortSpeed(t, dp2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dp2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// P4RTNodesByPort returns a map of <portID>:<P4RTNodeName> for the reserved ondatra
// ports using the component and the interface OC tree.
func P4RTNodesByPort(t testing.TB, dut *ondatra.DUTDevice) map[string]string {
	t.Helper()
	ports := make(map[string][]string) // <hardware-port>:[<portID>]
	for _, p := range dut.Ports() {
		hp := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State())
		if v, ok := hp.Val(); ok {
			if _, ok = ports[v]; !ok {
				ports[v] = []string{p.ID()}
			} else {
				ports[v] = append(ports[v], p.ID())
			}
		}
	}
	nodes := make(map[string]string) // <hardware-port>:<p4rtComponentName>
	for hp := range ports {
		p4Node := gnmi.Lookup(t, dut, gnmi.OC().Component(hp).Parent().State())
		if v, ok := p4Node.Val(); ok {
			nodes[hp] = v
		}
	}
	res := make(map[string]string) // <portID>:<P4RTNodeName>
	for k, v := range nodes {
		cType := gnmi.Lookup(t, dut, gnmi.OC().Component(v).Type().State())
		ct, ok := cType.Val()
		if !ok {
			continue
		}
		if ct != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT {
			continue
		}
		for _, p := range ports[k] {
			res[p] = v
		}
	}
	return res
}
