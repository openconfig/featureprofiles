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
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

var (
	p4rtNodeName = flag.String("p4rt_node_name", "SwitchChip3/0", "component name for P4RT Node")
)

const (
	adminStatusUp   = oc.Interface_AdminStatus_UP
	adminStatusDown = oc.Interface_AdminStatus_DOWN
	operStatusUp    = oc.Interface_OperStatus_UP
	operStatusDown  = oc.Interface_OperStatus_DOWN
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

	adminStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).AdminStatus().State())
	t.Logf("Got %s AdminStatus from telmetry: %v", dp.Name(), adminStatus)
	if adminStatus != adminStatusUp {
		t.Errorf("Get(DUT port1 OperStatus): got %v, want %v", adminStatus, adminStatusUp)
	}
}

func TestInterfaceOperStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

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
			gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)

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

	// Derive hardware port from interface name by removing the port number.
	// For example, Ethernet3/35/1 hardware port is Ethernet3/35.
	i := strings.LastIndex(dp.Name(), "/")
	want := dp.Name()[:i]

	got := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).HardwarePort().State())
	t.Logf("Got %s HardwarePort from telmetry: %v, expected: %v", dp.Name(), got, want)
	if got != want {
		t.Errorf("Get(DUT port1 HardwarePort): got %v, want %v", got, want)
	}
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
				parent := gnmi.Lookup(t, dut, gnmi.OC().Component(card).Parent().State())
				val, present := parent.Val()
				if !present {
					t.Errorf("parent.IsPresent() for %q: got %v, want true", card, parent.IsPresent())
				}

				t.Logf("Got %s parent: %s", tc.desc, val)
				if !strings.HasPrefix(val, tc.parent) {
					t.Errorf("Get parent for %q: got %v, want HasPrefix %v", card, val, tc.parent)
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
		swVersion := gnmi.Lookup(t, dut, gnmi.OC().Component(card).SoftwareVersion().State())
		if val, present := swVersion.Val(); present {
			softwareVersion = val
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
	swVersions := gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().SoftwareVersion().State())
	if len(swVersions) == 0 {
		t.Errorf("SoftwareVersion().Lookup(t) for %q: got none, want non-empty string", dut.Name())
	}
	for i, ver := range swVersions {
		val, present := ver.Val()
		if !present {
			t.Errorf("Telemetry path not present %d: %v:", i, ver.Path.String())
		}
		t.Logf("Telemetry path/value %d: %v=>%v:", i, ver.Path.String(), val)
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
		component := gnmi.OC().Component(cpu)
		if !gnmi.Lookup(t, dut, component.MfgName().State()).IsPresent() {
			t.Errorf("component.MfgName().Lookup(t).IsPresent() for %q: got false, want true", cpu)
		} else {
			t.Logf("CPU %s MfgName: %s", cpu, gnmi.Get(t, dut, component.MfgName().State()))
		}

		if !gnmi.Lookup(t, dut, component.Description().State()).IsPresent() {
			t.Errorf("component.Description().Lookup(t).IsPresent() for %q: got false, want true", cpu)
		} else {
			t.Logf("CPU %s Description: %s", cpu, gnmi.Get(t, dut, component.Description().State()))
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

func findMatchedComponents(t *testing.T, dut *ondatra.DUTDevice, r *regexp.Regexp) []string {
	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().Name().State())
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
			gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)

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
			gnmi.Replace(t, dut, gnmi.OC().Component(*p4rtNodeName).IntegratedCircuit().Config(), ic)

			// Check path /components/component/integrated-circuit/state/node-id.
			nodeID := gnmi.Lookup(t, dut, gnmi.OC().Component(*p4rtNodeName).IntegratedCircuit().NodeId().State())
			nodeIDVal, present := nodeID.Val()
			if !present {
				t.Fatalf("nodeID.IsPresent() for %q: got false, want true", *p4rtNodeName)
			}
			t.Logf("Telemetry path/value: %v=>%v:", nodeID.Path.String(), nodeIDVal)
			if nodeIDVal != tc.nodeID {
				t.Fatalf("nodeID.Val(t) for %q: got %d, want %d", *p4rtNodeName, nodeIDVal, tc.nodeID)
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
	dutInPktsBeforeTraffic := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Counters().InUnicastPkts().State())
	dutOutPktsBeforeTraffic := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Counters().OutUnicastPkts().State())
	if *deviations.InterfaceCountersFromContainer {
		dutInPktsBeforeTraffic = *gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Counters().State()).InUnicastPkts
		dutOutPktsBeforeTraffic = *gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Counters().State()).OutUnicastPkts
	}
	t.Log("inPkts and outPkts counters before traffic: ", dutInPktsBeforeTraffic, dutOutPktsBeforeTraffic)

	ate.Traffic().Start(t, flow)
	time.Sleep(10 * time.Second)
	ate.Traffic().Stop(t)

	ds1 := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; ds1 != want {
		t.Errorf("Get(DUT port1 status): got %v, want %v", ds1, want)
	}
	as1 := gnmi.Get(t, ate, gnmi.OC().Interface(ap1.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; as1 != want {
		t.Errorf("Get(ATE port1 status): got %v, want %v", as1, want)
	}
	ds2 := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; ds2 != want {
		t.Errorf("Get(DUT port2 status): got %v, want %v", ds2, want)
	}
	as2 := gnmi.Get(t, ate, gnmi.OC().Interface(ap2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; as2 != want {
		t.Errorf("Get(ATE port2 status): got %v, want %v", as2, want)
	}
	ateInPkts := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).Counters().InPkts().State())
	ateOutPkts := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).Counters().OutPkts().State())

	if ateOutPkts == 0 {
		t.Errorf("Get(out packets for flow %q: got %v, want nonzero", flow.Name(), ateOutPkts)
	}
	lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State())
	if lossPct >= 0.1 {
		t.Errorf("Get(traffic loss for flow %q: got %v, want < 0.1", flow.Name(), lossPct)
	}
	dutInPktsAfterTraffic := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Counters().InUnicastPkts().State())
	dutOutPktsAfterTraffic := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Counters().OutUnicastPkts().State())
	if *deviations.InterfaceCountersFromContainer {
		dutInPktsAfterTraffic = *gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Counters().State()).InUnicastPkts
		dutOutPktsAfterTraffic = *gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Counters().State()).OutUnicastPkts
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
		dutQosPktsBeforeTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
	}

	t.Logf("Running traffic on DUT interfaces: %s and %s ", dp1.Name(), dp2.Name())
	ate.Traffic().Start(t, flows...)
	time.Sleep(10 * time.Second)
	ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)

	for trafficID, data := range trafficFlows {
		ateOutPkts[data.queue] = gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).Counters().OutPkts().State())
		t.Logf("ateOutPkts: %v, txPkts %v, Queue: %v", ateOutPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)
		t.Logf("Get(out packets for queue %q): got %v", data.queue, ateOutPkts[data.queue])

		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).LossPct().State())
		if lossPct >= 1 {
			t.Errorf("Get(traffic loss for queue %q): got %v, want < 1", data.queue, lossPct)
		}
	}

	for trafficID, data := range trafficFlows {
		ateOutPkts[data.queue] = gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).Counters().OutPkts().State())
		dutQosPktsAfterTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp2.Name()).Output().Queue(data.queue).TransmitPkts().State())
		t.Logf("ateOutPkts: %v, txPkts %v, Queue: %v", ateOutPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)
		t.Logf("Get(out packets for flow %q): got %v, want nonzero", trafficID, ateOutPkts)

		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).LossPct().State())
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
		i := &oc.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		i.GetOrCreateEthernet()
		s := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		if *deviations.InterfaceEnabled {
			s.Enabled = ygot.Bool(true)
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
	}
}
