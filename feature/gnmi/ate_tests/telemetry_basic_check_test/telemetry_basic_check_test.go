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
	"flag"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

var (
	p4rtNodeName = flag.String("p4rt_node_name", "SwitchChip3/0", "component name for P4RT Node")
)

const (
	adminStatusUp   = telemetry.Interface_AdminStatus_UP
	adminStatusDown = telemetry.Interface_AdminStatus_DOWN
	operStatusUp    = telemetry.Interface_OperStatus_UP
	operStatusDown  = telemetry.Interface_OperStatus_DOWN
	maxPortVal      = "FFFFFEFF" // Maximum Port Value : https://github.com/openconfig/public/blob/2049164a8bca4cc9f11ffb313ef25c0e87303a24/release/models/p4rt/openconfig-p4rt.yang#L63-L81
)

type trafficData struct {
	trafficRate float64
	frameSize   uint32
	dscp        uint8
	queue       string
}

var (
	vendorQueueNo = map[ondatra.Vendor]int{
		ondatra.ARISTA:  16,
		ondatra.CISCO:   6,
		ondatra.JUNIPER: 6,
	}
)

var portSpeed = map[ondatra.Speed]telemetry.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed10Gb:  telemetry.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: telemetry.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: telemetry.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
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
	want := portSpeed[dp.Speed()]
	got := dut.Telemetry().Interface(dp.Name()).Ethernet().PortSpeed().Get(t)
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
	macAddress := dut.Telemetry().Interface(dp.Name()).Ethernet().MacAddress().Get(t)
	t.Logf("Got %s MacAddress from telmetry: %v", dp.Name(), macAddress)
	if len(r.FindString(macAddress)) == 0 {
		t.Errorf("Get(DUT port1 MacAddress): got %v, want matching regexp %v", macAddress, macRegexp)
	}
}

func TestInterfaceAdminStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	adminStatus := dut.Telemetry().Interface(dp.Name()).AdminStatus().Get(t)
	t.Logf("Got %s AdminStatus from telmetry: %v", dp.Name(), adminStatus)
	if adminStatus != adminStatusUp {
		t.Errorf("Get(DUT port1 OperStatus): got %v, want %v", adminStatus, adminStatusUp)
	}
}

func TestInterfaceOperStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	operStatus := dut.Telemetry().Interface(dp.Name()).OperStatus().Get(t)
	t.Logf("Got %s OperStatus from telmetry: %v", dp.Name(), operStatus)
	if operStatus != operStatusUp {
		t.Errorf("Get(DUT port1 OperStatus): got %v, want %v", operStatus, operStatusUp)
	}
}

func TestInterfacePhysicalChannel(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	phyChannel := dut.Telemetry().Interface(dp.Name()).PhysicalChannel().Get(t)
	t.Logf("Got %q PhysicalChannel from telmetry: %v", dp.Name(), phyChannel)
	if len(phyChannel) == 0 {
		t.Errorf("Get(DUT port1 PhysicalChannel): got empty %v, want non-empty list", phyChannel)
	}
}

func TestInterfaceStatusChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	d := &telemetry.Device{}
	i := d.GetOrCreateInterface(dp.Name())

	cases := []struct {
		desc                string
		IntfStatus          bool
		expectedAdminStatus telemetry.E_Interface_AdminStatus
		expectedOperStatus  telemetry.E_Interface_OperStatus
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
			dut.Config().Interface(dp.Name()).Replace(t, i)

			dut.Telemetry().Interface(dp.Name()).OperStatus().Await(t, intUpdateTime, tc.expectedOperStatus)
			dut.Telemetry().Interface(dp.Name()).AdminStatus().Await(t, intUpdateTime, tc.expectedAdminStatus)
			operStatus := dut.Telemetry().Interface(dp.Name()).OperStatus().Get(t)
			if want := tc.expectedOperStatus; operStatus != want {
				t.Errorf("Get(DUT port1 oper status): got %v, want %v", operStatus, want)
			}
			adminStatus := dut.Telemetry().Interface(dp.Name()).AdminStatus().Get(t)
			if want := tc.expectedAdminStatus; adminStatus != want {
				t.Errorf("Get(DUT port1 admin status): got %v, want %v", adminStatus, want)
			}
		})
	}
}

func TestHardwarePort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	// Derive hardware port from interface name by removing the port number.
	// For example, Ethernet3/35/1 hardware port is Ethernet3/35.
	i := strings.LastIndex(dp.Name(), "/")
	want := dp.Name()[:i]

	got := dut.Telemetry().Interface(dp.Name()).HardwarePort().Get(t)
	t.Logf("Got %s HardwarePort from telmetry: %v, expected: %v", dp.Name(), got, want)
	if got != want {
		t.Errorf("Get(DUT port1 HardwarePort): got %v, want %v", got, want)
	}
}

func TestInterfaceCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	counters := dut.Telemetry().Interface(dp.Name()).Counters()
	intCounterPath := "/interfaces/interface/state/counters/"

	cases := []struct {
		desc    string
		path    string
		counter *telemetry.QualifiedUint64
	}{{
		desc:    "InUnicastPkts",
		path:    intCounterPath + "in-unicast-pkts",
		counter: counters.InUnicastPkts().Lookup(t),
	}, {
		desc:    "InOctets",
		path:    intCounterPath + "in-octets",
		counter: counters.InOctets().Lookup(t),
	}, {
		desc:    "InMulticastPkts",
		path:    intCounterPath + "in-multicast-pkts",
		counter: counters.InMulticastPkts().Lookup(t),
	}, {
		desc:    "InBroadcastPkts",
		path:    intCounterPath + "in-broadcast-pkts",
		counter: counters.InBroadcastPkts().Lookup(t),
	}, {
		desc:    "InDiscards",
		path:    intCounterPath + "in-discards",
		counter: counters.InDiscards().Lookup(t),
	}, {
		desc:    "InErrors",
		path:    intCounterPath + "in-errors",
		counter: counters.InErrors().Lookup(t),
	}, {
		desc:    "InFcsErrors",
		path:    intCounterPath + "in-fcs-errors",
		counter: counters.InFcsErrors().Lookup(t),
	}, {
		desc:    "OutUnicastPkts",
		path:    intCounterPath + "out-unicast-pkts",
		counter: counters.OutUnicastPkts().Lookup(t),
	}, {
		desc:    "OutOctets",
		path:    intCounterPath + "out-octets",
		counter: counters.OutOctets().Lookup(t),
	}, {
		desc:    "OutMulticastPkts",
		path:    intCounterPath + "out-broadcast-pkts",
		counter: counters.OutMulticastPkts().Lookup(t),
	}, {
		desc:    "OutBroadcastPkts",
		path:    intCounterPath + "out-multicast-pkts",
		counter: counters.OutBroadcastPkts().Lookup(t),
	}, {
		desc:    "OutDiscards",
		path:    intCounterPath + "out-discards",
		counter: counters.OutDiscards().Lookup(t),
	}, {
		desc:    "OutErrors",
		path:    intCounterPath + "out-errors",
		counter: counters.OutErrors().Lookup(t),
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if !tc.counter.IsPresent() {
				t.Errorf("Get IsPresent status for path %q: got false, want true", tc.path)
			}
			t.Logf("Got path/value: %s:%d", tc.path, tc.counter.Val(t))
		})
	}
}

func TestQoSCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	queues := dut.Telemetry().Qos().Interface(dp.Name()).Output().QueueAny()
	qosQueuePath := "/qos/interfaces/interface/output/queues/queue/state/"

	cases := []struct {
		desc     string
		path     string
		counters []*telemetry.QualifiedUint64
	}{{
		desc:     "TransmitPkts",
		path:     qosQueuePath + "transmit-pkts",
		counters: queues.TransmitPkts().Lookup(t),
	}, {
		desc:     "TransmitOctets",
		path:     qosQueuePath + "transmit-octets",
		counters: queues.TransmitOctets().Lookup(t),
	}, {
		desc:     "DroppedPkts",
		path:     qosQueuePath + "dropped-pkts",
		counters: queues.DroppedPkts().Lookup(t),
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			if len(tc.counters) != vendorQueueNo[dut.Vendor()] {
				t.Errorf("Get QoS queue# for %q: got %d, want %d", dut.Vendor(), len(tc.counters), vendorQueueNo[dut.Vendor()])
			}
			for i, counter := range tc.counters {
				if !counter.IsPresent() {
					t.Errorf("counter.IsPresent() for queue %d): got false, want true", i)
				}
				t.Logf("Got queue %d path/value: %s:%d", i, tc.path, tc.counters[i].Val(t))
			}
		})
	}
}

func TestComponentParent(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var componentParent = map[string]string{
		"Fabric":      "Chassis",
		"FabricChip":  "Fabric",
		"Linecard":    "Chassis",
		"PowerSupply": "Chassis",
		"Supervisor":  "Chassis",
		"SwitchChip":  "Linecard",
	}

	cases := []struct {
		desc          string
		regexpPattern string
		parent        string
	}{{
		desc:          "Fabric",
		regexpPattern: "^Fabric[0-9]",
		parent:        componentParent["Fabric"],
	}, {
		desc:          "FabricChip",
		regexpPattern: "^FabricChip",
		parent:        componentParent["FabricChip"],
	}, {
		desc:          "Linecard",
		regexpPattern: "^Linecard[0-9]",
		parent:        componentParent["Linecard"],
	}, {
		desc:          "Power supply",
		regexpPattern: "^PowerSupply[0-9]",
		parent:        componentParent["PowerSupply"],
	}, {
		desc:          "Supervisor",
		regexpPattern: "^Supervisor[0-9]$",
		parent:        componentParent["Supervisor"],
	}, {
		desc:          "SwitchChip",
		regexpPattern: "^SwitchChip",
		parent:        componentParent["SwitchChip"],
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			r, err := regexp.Compile(tc.regexpPattern)
			if err != nil {
				t.Fatalf("Cannot compile regular expression: %v", err)
			}
			cards := findMatchedComponents(t, dut, r)
			t.Logf("Found card list for %v: %v", tc.desc, cards)

			if len(cards) == 0 {
				t.Errorf("Get card list for %q on %v: got 0, want > 0", tc.desc, dut.Model())
			}
			for _, card := range cards {
				t.Logf("Validate card %s", card)
				parent := dut.Telemetry().Component(card).Parent().Lookup(t)
				if !parent.IsPresent() {
					t.Errorf("parent.IsPresent() for %q: got %v, want true", card, parent.IsPresent())
				}

				t.Logf("Got %s parent: %s", tc.desc, parent.Val(t))
				if !strings.HasPrefix(parent.Val(t), tc.parent) {
					t.Errorf("Get parent for %q: got %v, want HasPrefix %v", card, parent.Val(t), tc.parent)
				}
			}
		})
	}
}

func TestSoftwareVersion(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	regexpPattern := "^Supervisor[0-9]"

	r, err := regexp.Compile(regexpPattern)
	if err != nil {
		t.Fatalf("Cannot compile regular expression: %v", err)
	}
	cards := findMatchedComponents(t, dut, r)
	t.Logf("Found card list for %v: %v", regexpPattern, cards)
	if len(cards) == 0 {
		t.Errorf("Get card list for %q on %v: got 0, want > 0", regexpPattern, dut.Model())
	}

	// Validate Supervisor components include software version.
	swVersionFound := false
	for _, card := range cards {
		t.Logf("Validate card %s", card)
		softwareVersion := ""
		// Only a subset of cards are expected to report Software Version.
		swVersion := dut.Telemetry().Component(card).SoftwareVersion().Lookup(t)
		if swVersion.IsPresent() {
			softwareVersion = swVersion.Val(t)
			t.Logf("Hardware card %s SoftwareVersion: %s", card, softwareVersion)
			swVersionFound = true
			if softwareVersion == "" {
				t.Errorf("swVersion.Val(t) for %q: got empty string, want non-empty string", card)
			}
		} else {
			t.Logf("swVersion.Val(t) for %q: got no value.", card)
		}
	}
	if !swVersionFound {
		t.Errorf("Failed to find software version from %v", cards)
	}

	// Get /components/component/state/software-version directly.
	swVersions := dut.Telemetry().ComponentAny().SoftwareVersion().Lookup(t)
	if len(swVersions) == 0 {
		t.Errorf("SoftwareVersion().Lookup(t) for %q: got none, want non-empty string", dut.Name())
	}
	for i, ver := range swVersions {
		t.Logf("Telemetry path/value %d: %v=>%v:", i, ver.GetPath().String(), ver.Val(t))
	}
}

func TestCPU(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	r := regexp.MustCompile("^CPU")
	cpus := findMatchedComponents(t, dut, r)
	t.Logf("Found CPU list: %v", cpus)
	if len(cpus) == 0 {
		t.Fatalf("Get CPU list for %q: got 0, want > 0", dut.Model())
	}

	for _, cpu := range cpus {
		t.Logf("Validate CPU: %s", cpu)
		component := dut.Telemetry().Component(cpu)
		if !component.MfgName().Lookup(t).IsPresent() {
			t.Errorf("component.MfgName().Lookup(t).IsPresent() for %q: got false, want true", cpu)
		} else {
			t.Logf("CPU %s MfgName: %s", cpu, component.MfgName().Get(t))
		}

		if !component.Description().Lookup(t).IsPresent() {
			t.Errorf("component.Description().Lookup(t).IsPresent() for %q: got false, want true", cpu)
		} else {
			t.Logf("CPU %s Description: %s", cpu, component.Description().Get(t))
		}
	}
}

func TestSupervisorLastRebootInfo(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	regexpPattern := "^Supervisor[0-9]"

	r, err := regexp.Compile(regexpPattern)
	if err != nil {
		t.Fatalf("Cannot compile regular expression: %v", err)
	}
	cards := findMatchedComponents(t, dut, r)
	t.Logf("Found card list for %v: %v", regexpPattern, cards)
	if len(cards) == 0 {
		t.Errorf("Get card list for %q on %v: got 0, want > 0", regexpPattern, dut.Model())
	}

	rebootTimeFound := false
	rebootReasonFound := false
	for _, card := range cards {
		t.Logf("Validate card %s", card)
		rebootTime := dut.Telemetry().Component(card).LastRebootTime()
		if rebootTime.Lookup(t).IsPresent() {
			t.Logf("Hardware card %s reboot time: %v", card, rebootTime.Get(t))
			rebootTimeFound = true
		}

		rebootReason := dut.Telemetry().Component(card).LastRebootReason()
		if rebootReason.Lookup(t).IsPresent() {
			t.Logf("Hardware card %s reboot reason: %v", card, rebootReason.Get(t))
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

func findMatchedComponents(t *testing.T, dut *ondatra.DUTDevice, r *regexp.Regexp) []string {
	components := dut.Telemetry().ComponentAny().Name().Get(t)
	var s []string
	for _, c := range components {
		if len(r.FindString(c)) > 0 {
			s = append(s, c)
		}
	}
	return s
}

func TestAFT(t *testing.T) {
	// TODO: Remove t.Skipf() after the issue is fixed.
	t.Skipf("Test is skipped due to the failure")

	dut := ondatra.DUT(t, "dut")
	afts := dut.Telemetry().NetworkInstanceAny().Afts().Lookup(t)

	if len(afts) == 0 {
		t.Errorf("Afts().Lookup(t) for %q: got 0, want > 0", dut.Name())
	}
	for i, aft := range afts {
		if !aft.IsPresent() {
			t.Errorf("aft.IsPresent() for %q: got false, want true", dut.Name())
		}
		t.Logf("Telemetry path/value %d: %v=>%v:", i, aft.GetPath().String(), aft.Val(t))
	}
}

func TestLacpMember(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lacpIntfs := dut.Telemetry().Lacp().InterfaceAny().Name().Get(t)
	if len(lacpIntfs) == 0 {
		t.Errorf("Lacp().InterfaceAny().Name().Get(t) for %q: got 0, want > 0", dut.Name())
	}
	t.Logf("Found %d LACP interfaces: %v", len(lacpIntfs)+1, lacpIntfs)

	for i, intf := range lacpIntfs {
		t.Logf("Telemetry LACP interface %d: %s:", i, intf)
		members := dut.Telemetry().Lacp().Interface(intf).MemberAny().Lookup(t)
		if len(members) == 0 {
			t.Errorf("MemberAny().Lookup(t) for %q: got 0, want > 0", intf)
		}
		for i, member := range members {
			if !member.IsPresent() {
				t.Errorf("member.IsPresent() for %q: got false, want true", intf)
			}
			t.Logf("Telemetry path/value %d: %v=>%v:", i, member.GetPath().String(), member.Val(t))

			// Check LACP packet counters.
			memberVal := member.Val(t)
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
	d := &telemetry.Device{}
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
			dut.Config().Interface(dp.Name()).Replace(t, i)

			// Check path /interfaces/interface/state/id.
			intfID := dut.Telemetry().Interface(dp.Name()).Id().Lookup(t)
			if !intfID.IsPresent() {
				t.Fatalf("intfID.IsPresent() for %q: got false, want true", dp.Name())
			}
			t.Logf("Telemetry path/value: %v=>%v:", intfID.GetPath().String(), intfID.Val(t))
			if intfID.Val(t) != tc.portID {
				t.Fatalf("intfID.Val(t) for %q: got %d, want %d", dp.Name(), intfID.Val(t), tc.portID)
			}
		})
	}
}

func TestP4rtNodeID(t *testing.T) {
	// TODO: add p4rtNodeName to Ondatra's netutil
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	ic := d.GetOrCreateComponent(*p4rtNodeName).GetOrCreateIntegratedCircuit()

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
			dut.Config().Component(*p4rtNodeName).IntegratedCircuit().Replace(t, ic)

			// Check path /components/component/integrated-circuit/state/node-id.
			nodeID := dut.Telemetry().Component(*p4rtNodeName).IntegratedCircuit().NodeId().Lookup(t)
			if !nodeID.IsPresent() {
				t.Fatalf("nodeID.IsPresent() for %q: got false, want true", *p4rtNodeName)
			}
			t.Logf("Telemetry path/value: %v=>%v:", nodeID.GetPath().String(), nodeID.Val(t))
			if nodeID.Val(t) != tc.nodeID {
				t.Fatalf("nodeID.Val(t) for %q: got %d, want %d", *p4rtNodeName, nodeID.Val(t), tc.nodeID)
			}
		})
	}
}

func TestIntfCounterUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	top := ate.Topology().New()
	intf1 := top.AddInterface("intf1").WithPort(ap1)
	intf1.IPv4().
		WithAddress("198.51.100.1/31").
		WithDefaultGateway("198.51.100.0")
	intf2 := top.AddInterface("intf2").WithPort(ap2)
	intf2.IPv4().
		WithAddress("198.51.100.3/31").
		WithDefaultGateway("198.51.100.2")
	top.Push(t).StartProtocols(t)

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	flow := ate.Traffic().NewFlow("streamtelemetry_test_flow").
		WithSrcEndpoints(intf1).
		WithDstEndpoints(intf2).
		WithHeaders(ethHeader, ipv4Header).
		WithFrameRatePct(15)

	t.Log("Running traffic on DUT interfaces: ", dp1, dp2)
	dutInPktsBeforeTraffic := dut.Telemetry().Interface(dp1.Name()).Counters().InUnicastPkts().Get(t)
	dutOutPktsBeforeTraffic := dut.Telemetry().Interface(dp2.Name()).Counters().OutUnicastPkts().Get(t)
	if *deviations.InterfaceCountersFromContainer {
		dutInPktsBeforeTraffic = *dut.Telemetry().Interface(dp1.Name()).Counters().Get(t).InUnicastPkts
		dutOutPktsBeforeTraffic = *dut.Telemetry().Interface(dp2.Name()).Counters().Get(t).OutUnicastPkts
	}
	t.Log("inPkts and outPkts counters before traffic: ", dutInPktsBeforeTraffic, dutOutPktsBeforeTraffic)

	ate.Traffic().Start(t, flow)
	time.Sleep(10 * time.Second)
	ate.Traffic().Stop(t)

	ds1 := dut.Telemetry().Interface(dp1.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; ds1 != want {
		t.Errorf("Get(DUT port1 status): got %v, want %v", ds1, want)
	}
	as1 := ate.Telemetry().Interface(ap1.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; as1 != want {
		t.Errorf("Get(ATE port1 status): got %v, want %v", as1, want)
	}
	ds2 := dut.Telemetry().Interface(dp2.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; ds2 != want {
		t.Errorf("Get(DUT port2 status): got %v, want %v", ds2, want)
	}
	as2 := ate.Telemetry().Interface(ap2.Name()).OperStatus().Get(t)
	if want := telemetry.Interface_OperStatus_UP; as2 != want {
		t.Errorf("Get(ATE port2 status): got %v, want %v", as2, want)
	}
	ateInPkts := ate.Telemetry().Flow(flow.Name()).Counters().InPkts().Get(t)
	ateOutPkts := ate.Telemetry().Flow(flow.Name()).Counters().OutPkts().Get(t)

	if ateOutPkts == 0 {
		t.Errorf("Get(out packets for flow %q: got %v, want nonzero", flow.Name(), ateOutPkts)
	}
	lossPct := ate.Telemetry().Flow(flow.Name()).LossPct().Get(t)
	if lossPct >= 0.1 {
		t.Errorf("Get(traffic loss for flow %q: got %v, want < 0.1", flow.Name(), lossPct)
	}
	dutInPktsAfterTraffic := dut.Telemetry().Interface(dp1.Name()).Counters().InUnicastPkts().Get(t)
	dutOutPktsAfterTraffic := dut.Telemetry().Interface(dp2.Name()).Counters().OutUnicastPkts().Get(t)
	if *deviations.InterfaceCountersFromContainer {
		dutInPktsAfterTraffic = *dut.Telemetry().Interface(dp1.Name()).Counters().Get(t).InUnicastPkts
		dutOutPktsAfterTraffic = *dut.Telemetry().Interface(dp2.Name()).Counters().Get(t).OutUnicastPkts
	}
	t.Log("inPkts and outPkts counters after traffic: ", dutInPktsAfterTraffic, dutOutPktsAfterTraffic)

	if dutInPktsAfterTraffic-dutInPktsBeforeTraffic < ateInPkts {
		t.Errorf("Get less inPkts from telemetry: got %v, want >= %v", dutInPktsAfterTraffic-dutInPktsBeforeTraffic, ateOutPkts)
	}
	if dutOutPktsAfterTraffic-dutOutPktsBeforeTraffic < ateOutPkts {
		t.Errorf("Get less outPkts from telemetry: got %v, want >= %v", dutOutPktsAfterTraffic-dutOutPktsBeforeTraffic, ateOutPkts)
	}
}

func TestQoSCounterUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	top := ate.Topology().New()
	intf1 := top.AddInterface("intf1").WithPort(ap1)
	intf1.IPv4().
		WithAddress("198.51.100.1/31").
		WithDefaultGateway("198.51.100.0")
	intf2 := top.AddInterface("intf2").WithPort(ap2)
	intf2.IPv4().
		WithAddress("198.51.100.3/31").
		WithDefaultGateway("198.51.100.2")
	top.Push(t).StartProtocols(t)

	trafficFlows := make(map[string]*trafficData)

	// TODO: Please update the QoS test after the bug is closed.
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		trafficFlows = map[string]*trafficData{
			"flow-nc1": {frameSize: 1000, trafficRate: 1, dscp: 56, queue: "3"},
			"flow-af4": {frameSize: 400, trafficRate: 4, dscp: 32, queue: "2"},
			"flow-af3": {frameSize: 300, trafficRate: 3, dscp: 24, queue: "5"},
			"flow-af2": {frameSize: 200, trafficRate: 2, dscp: 16, queue: "1"},
			"flow-af1": {frameSize: 1100, trafficRate: 1, dscp: 8, queue: "4"},
			"flow-be1": {frameSize: 1200, trafficRate: 1, dscp: 0, queue: "0"},
		}
	case ondatra.ARISTA:
		trafficFlows = map[string]*trafficData{
			"flow-nc1": {frameSize: 700, trafficRate: 7, dscp: 56, queue: dp2.Name() + "-7"},
			"flow-af4": {frameSize: 400, trafficRate: 4, dscp: 32, queue: dp2.Name() + "-4"},
			"flow-af3": {frameSize: 1300, trafficRate: 3, dscp: 24, queue: dp2.Name() + "-3"},
			"flow-af2": {frameSize: 1200, trafficRate: 2, dscp: 16, queue: dp2.Name() + "-2"},
			"flow-af1": {frameSize: 1000, trafficRate: 10, dscp: 8, queue: dp2.Name() + "-0"},
			"flow-be1": {frameSize: 1111, trafficRate: 1, dscp: 0, queue: dp2.Name() + "-1"},
		}
	}

	var flows []*ondatra.Flow
	for trafficID, data := range trafficFlows {
		t.Logf("Configuring flow %s", trafficID)
		flow := ate.Traffic().NewFlow(trafficID).
			WithSrcEndpoints(intf1).
			WithDstEndpoints(intf2).
			WithHeaders(ondatra.NewEthernetHeader(), ondatra.NewIPv4Header().WithDSCP(data.dscp)).
			WithFrameRatePct(data.trafficRate).
			WithFrameSize(data.frameSize)
		flows = append(flows, flow)
	}

	ateOutPkts := make(map[string]uint64)
	dutQosPktsBeforeTraffic := make(map[string]uint64)
	dutQosPktsAfterTraffic := make(map[string]uint64)

	// Get QoS egress packet counters before the traffic.
	for _, data := range trafficFlows {
		dutQosPktsBeforeTraffic[data.queue] = dut.Telemetry().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().Get(t)
	}

	t.Logf("Running traffic on DUT interfaces: %s and %s ", dp1.Name(), dp2.Name())
	ate.Traffic().Start(t, flows...)
	time.Sleep(10 * time.Second)
	ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)

	for trafficID, data := range trafficFlows {
		ateOutPkts[data.queue] = ate.Telemetry().Flow(trafficID).Counters().OutPkts().Get(t)
		t.Logf("ateOutPkts: %v, txPkts %v, Queue: %v", ateOutPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)
		t.Logf("Get(out packets for queue %q): got %v", data.queue, ateOutPkts[data.queue])

		lossPct := ate.Telemetry().Flow(trafficID).LossPct().Get(t)
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue %q): got %v, want < 1", data.queue, lossPct)
		}
	}

	for trafficID, data := range trafficFlows {
		ateOutPkts[data.queue] = ate.Telemetry().Flow(trafficID).Counters().OutPkts().Get(t)
		dutQosPktsAfterTraffic[data.queue] = dut.Telemetry().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().Get(t)
		t.Logf("ateOutPkts: %v, txPkts %v, Queue: %v", ateOutPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)
		t.Logf("Get(out packets for flow %q): got %v, want nonzero", trafficID, ateOutPkts)

		lossPct := ate.Telemetry().Flow(trafficID).LossPct().Get(t)
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue %q: got %v, want < 1", data.queue, lossPct)
		}
	}

	// Check QoS egress packet counters are updated correctly.
	t.Logf("QoS egress packet counters before traffic: %v", dutQosPktsBeforeTraffic)
	t.Logf("QoS egress packet counters after traffic: %v", dutQosPktsAfterTraffic)
	t.Logf("QoS packet counters from ATE: %v", ateOutPkts)
	for _, data := range trafficFlows {
		qosCounterDiff := dutQosPktsAfterTraffic[data.queue] - dutQosPktsBeforeTraffic[data.queue]
		if qosCounterDiff < ateOutPkts[data.queue] {
			t.Errorf("Get(telemetry packet update for queue %q): got %v, want >= %v", data.queue, qosCounterDiff, ateOutPkts[data.queue])
		}
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
		i := &telemetry.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		i.GetOrCreateEthernet()
		s := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		if *deviations.InterfaceEnabled {
			s.Enabled = ygot.Bool(true)
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		dut.Config().Interface(intf.intfName).Replace(t, i)
	}
}
