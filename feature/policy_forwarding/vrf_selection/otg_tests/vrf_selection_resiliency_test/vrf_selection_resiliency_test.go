// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vrf_selection_resiliency_test

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/vrfpolicy"
	"github.com/openconfig/gnoigo/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	gnmiotg "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/gnoi"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	haPolicyName = "HA_VRF_SELECTION"
	ghostVRF     = "VRF-GHOST"
	// vrfsPerFamily is the number of IPv4 VRFs (VRF-V4-1..15) and of IPv6 VRFs (VRF-V6-1..15).
	vrfsPerFamily = 15

	// defaultSubIfIndex is the index, and VLAN ID, of the DUT Port-2 sub-interface in the DEFAULT
	// network-instance.
	defaultSubIfIndex = 11
	// defaultRouteV4 and defaultRouteV6 are the default static routes of the DEFAULT
	// network-instance towards the Port-2 DEFAULT sub-interface, as required by the README. They
	// replace the testbed's own default static routes, if any, until restoreDUT restores them.
	defaultRouteV4 = "0.0.0.0/0"
	defaultRouteV6 = "::/0"
	// coveringRouteV4 and coveringRouteV6 cover every OTG destination network. See defaultNIRoutes.
	coveringRouteV4 = "198.18.0.0/16"
	coveringRouteV6 = "2001:db8:a::/48"

	ghostFlow  = "GhostFlow"
	shadowFlow = "ShadowFlow"
	innerSrcV4 = "198.51.100.200"
	innerSrcV6 = "2001:db8:100:1::1"

	flowFrameSize = 256
	// flowRatePps is the rate of every flow. A packet rate keeps the 62 flows far below the line rate
	// of every port, also when all of them leave the same egress port.
	flowRatePps = 1000
	// minTrafficPkts is the number of packets every flow transmits in each measurement.
	minTrafficPkts = 500
	// vlanTagName names the OTG egress-tracking metric tag on the VLAN ID of received packets.
	vlanTagName = "egress_vlan_id"
	// vlanTagOffset and vlanTagLength select the 5 low-order bits of the 12-bit VLAN ID for egress
	// tracking, which identify the VLANs 1-31 of the egress sub-interfaces. The ATE reserves
	// 2^vlanTagLength tracking entries per flow on its receive port: with the whole VLAN ID, a port
	// holds only 8 flows and the ATE refuses to start the traffic.
	vlanTagOffset = 7
	vlanTagLength = 5

	// maxLossPct is the loss allowed for flows that must be forwarded: the README requires 0%.
	maxLossPct = 0.0
	// minDropPct is the loss required for flows that must be dropped: the README requires the
	// Ghost Stream to be entirely dropped.
	minDropPct = 100.0

	trafficTimeout       = 1 * time.Minute
	counterSettleTimeout = 30 * time.Second
	// forwardingTimeout bounds the traffic windows of awaitConvergence; starting and stopping the
	// traffic of a window can take minutes on some traffic generators.
	forwardingTimeout       = 4 * time.Minute
	setupPortUpTimeout      = 2 * time.Minute
	oirPortUpTimeout        = 5 * time.Minute
	switchoverReadyTimeout  = 5 * time.Minute
	switchoverTimeout       = 20 * time.Minute
	configRetryTimeout      = 5 * time.Minute
	lcPowerDownTimeout      = 5 * time.Minute
	lcPowerUpTimeout        = 10 * time.Minute
	reconnectAttemptTimeout = time.Minute
)

var (
	dutPort1 = attrs.Attributes{Desc: "ingress", MAC: "02:00:01:00:00:01", IPv4: "192.0.2.1", IPv6: "2001:db8::1", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	atePort1 = attrs.Attributes{Name: "ate_port1", MAC: "02:00:01:00:00:02", IPv4: "192.0.2.2", IPv6: "2001:db8::2", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}

	dutPortIDs       = []string{"port1", "port2", "port3", "port4"}
	portDescriptions = map[string]string{
		"port1": "Ingress Port-1",
		"port2": "Egress Port-2",
		"port3": "Egress Port-3",
		"port4": "Egress Port-4",
	}
)

// egressSubIf describes a DUT egress sub-interface and the ATE interface connected to it. The
// sub-interface index is also its VLAN ID.
type egressSubIf struct {
	port  string // ondatra port ID, the same on the DUT and the ATE
	index uint32
	ni    string // network-instance, empty for the DEFAULT network-instance
	ipv4  bool
	ipv6  bool
}

func v4VRF(i int) string { return fmt.Sprintf("VRF-V4-%d", i) }
func v6VRF(i int) string { return fmt.Sprintf("VRF-V6-%d", i) }

// defaultNIRoutes returns the IPv4 and IPv6 routes of the DEFAULT network-instance towards the
// Port-2 DEFAULT sub-interface: the default routes of the README, unless the DEFAULT
// network-instance already has pre-existing default static routes.
//
// TODO: replace this check with a deviation once testbeds isolate management/TACACS+ routes from
// the DEFAULT network-instance. Replacing pre-existing 0.0.0.0/0 and ::/0 routes in DEFAULT breaks
// TACACS+ reachability and makes later gNMI requests fail with authentication/authorization errors.
// The covering routes forward the same test traffic to the same sub-interface.
func defaultNIRoutes(s *dutSnapshot) (string, string) {
	if s.staticProtocol != nil && (s.staticProtocol.GetStatic(defaultRouteV4) != nil || s.staticProtocol.GetStatic(defaultRouteV6) != nil) {
		return coveringRouteV4, coveringRouteV6
	}
	return defaultRouteV4, defaultRouteV6
}

// v4VRFSubIf returns the sub-interface index of VRF-V4-<i>: Port-2 indices 1-10 (index 11 is the
// DEFAULT sub-interface) and Port-3 indices 12-16.
func v4VRFSubIf(i int) uint32 {
	if i <= 10 {
		return uint32(i)
	}
	return uint32(i + 1)
}

// v6VRFSubIf returns the sub-interface index of VRF-V6-<i>: Port-3 indices 17-21 and Port-4
// indices 22-31.
func v6VRFSubIf(i int) uint32 { return uint32(16 + i) }

// egressSubIfs returns the 31 egress sub-interfaces of the README test environment.
func egressSubIfs() []egressSubIf {
	var subIfs []egressSubIf
	for i := 1; i <= vrfsPerFamily; i++ {
		port := "port2"
		if i > 10 {
			port = "port3"
		}
		subIfs = append(subIfs, egressSubIf{port: port, index: v4VRFSubIf(i), ni: v4VRF(i), ipv4: true})
	}
	subIfs = append(subIfs, egressSubIf{port: "port2", index: defaultSubIfIndex, ipv4: true, ipv6: true})
	for i := 1; i <= vrfsPerFamily; i++ {
		port := "port3"
		if i > 5 {
			port = "port4"
		}
		subIfs = append(subIfs, egressSubIf{port: port, index: v6VRFSubIf(i), ni: v6VRF(i), ipv6: true})
	}
	return subIfs
}

// testVRFs returns the non-default network-instances that the test creates.
func testVRFs() []string {
	vrfs := []string{ghostVRF}
	for i := 1; i <= vrfsPerFamily; i++ {
		vrfs = append(vrfs, v4VRF(i), v6VRF(i))
	}
	return vrfs
}

// egressDeviceName returns the name of the ATE interface attached to the egress sub-interface
// with the given index.
func egressDeviceName(index uint32) string { return fmt.Sprintf("ate_port_egress_%d", index) }

// egressAttrs returns the DUT and ATE addresses of the egress sub-interface with the given index.
func egressAttrs(index uint32) (attrs.Attributes, attrs.Attributes) {
	dutAttrs := attrs.Attributes{
		Desc:    fmt.Sprintf("egress_dut_%d", index),
		MAC:     fmt.Sprintf("02:00:02:00:%02x:%02x", index>>8, index&0xff),
		IPv4:    fmt.Sprintf("192.0.2.%d", 4*index+1),
		IPv4Len: ipv4PrefixLen,
		IPv6:    fmt.Sprintf("2001:db8:1:%x::1", index),
		IPv6Len: ipv6PrefixLen,
	}
	ateAttrs := attrs.Attributes{
		Name:    egressDeviceName(index),
		MAC:     fmt.Sprintf("02:00:03:00:%02x:%02x", index>>8, index&0xff),
		IPv4:    fmt.Sprintf("192.0.2.%d", 4*index+2),
		IPv4Len: ipv4PrefixLen,
		IPv6:    fmt.Sprintf("2001:db8:1:%x::2", index),
		IPv6Len: ipv6PrefixLen,
	}
	return dutAttrs, ateAttrs
}

// dutPortNames returns the names of DUT ports 1-4.
func dutPortNames(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	var names []string
	for _, id := range dutPortIDs {
		names = append(names, dut.Port(t, id).Name())
	}
	return names
}

// defaultNIInterfaceIDs returns the DEFAULT network-instance interfaces that the test assigns
// explicitly, which only happens with the ExplicitInterfaceInDefaultVRF deviation.
func defaultNIInterfaceIDs(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	if !deviations.ExplicitInterfaceInDefaultVRF(dut) {
		return nil
	}
	return []string{
		fmt.Sprintf("%s.0", dut.Port(t, "port1").Name()),
		fmt.Sprintf("%s.%d", dut.Port(t, "port2").Name(), defaultSubIfIndex),
	}
}

// newEthernetInterface returns the base configuration of a DUT Ethernet port.
func newEthernetInterface(dut *ondatra.DUTDevice, name, desc string) *oc.Interface {
	i := &oc.Interface{
		Name:        ygot.String(name),
		Description: ygot.String(desc),
		Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
	}
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	return i
}

// configureEgressSubIf configures the VLAN and the enabled leaves of an egress sub-interface and,
// if withAddresses is set, its addresses.
func configureEgressSubIf(dut *ondatra.DUTDevice, sub *oc.Interface_Subinterface, s egressSubIf, withAddresses bool) {
	cfgplugins.ConfigureVLAN(sub, dut, uint16(s.index))
	if deviations.InterfaceEnabled(dut) {
		sub.Enabled = ygot.Bool(true)
		// IPv6-only sub-interfaces also enable IPv4 on DUTs that need the VRF configured before
		// the addresses.
		if (s.ipv4 || deviations.InterfaceConfigVRFBeforeAddress(dut)) && !deviations.IPv4MissingEnabled(dut) {
			sub.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		if s.ipv6 {
			sub.GetOrCreateIpv6().Enabled = ygot.Bool(true)
		}
	}
	if !withAddresses {
		return
	}
	dutAttrs, _ := egressAttrs(s.index)
	if s.ipv4 {
		sub.GetOrCreateIpv4().GetOrCreateAddress(dutAttrs.IPv4).PrefixLength = ygot.Uint8(ipv4PrefixLen)
	}
	if s.ipv6 {
		sub.GetOrCreateIpv6().GetOrCreateAddress(dutAttrs.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)
	}
}

// addStaticRoute adds a static route with a single next-hop to the batch.
func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice, b *gnmi.SetBatch, ni, prefix, nextHop string) {
	t.Helper()
	if _, err := cfgplugins.NewStaticRouteCfg(b, &cfgplugins.StaticRouteCfg{
		NetworkInstance: ni,
		Prefix:          prefix,
		NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(nextHop)},
	}, dut); err != nil {
		t.Fatalf("NewStaticRouteCfg(%s, %s) failed: %v", ni, prefix, err)
	}
}

// configureDUT configures the README test environment on the DUT: Port-1 in the DEFAULT
// network-instance, the 31 egress sub-interfaces, the 30 VRFs and VRF-GHOST, one static route in
// every VRF and the DEFAULT network-instance default routes towards the Port-2 DEFAULT
// sub-interface.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, snapshot *dutSnapshot) {
	t.Helper()
	if deviations.VendorPreConfigRequired(dut) {
		cfgplugins.ConfigureDecapVrfSelection(t, dut)
	}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	defNI := deviations.DefaultNetworkInstance(dut)
	vrfBeforeAddress := deviations.InterfaceConfigVRFBeforeAddress(dut)
	subIfs := egressSubIfs()

	t.Log("Configuring DUT ports, sub-interfaces and network-instances")
	b := &gnmi.SetBatch{}
	p1 := dut.Port(t, "port1")
	i1 := newEthernetInterface(dut, p1.Name(), portDescriptions["port1"])
	s1 := i1.GetOrCreateSubinterface(0)
	s1.GetOrCreateIpv4().GetOrCreateAddress(dutPort1.IPv4).PrefixLength = ygot.Uint8(ipv4PrefixLen)
	s1.GetOrCreateIpv6().GetOrCreateAddress(dutPort1.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)
	if deviations.InterfaceEnabled(dut) {
		if !deviations.IPv4MissingEnabled(dut) {
			s1.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		s1.GetOrCreateIpv6().Enabled = ygot.Bool(true)
	}
	gnmi.BatchUpdate(b, gnmi.OC().Interface(p1.Name()).Config(), i1)

	egressIntfs := make(map[string]*oc.Interface)
	for _, s := range subIfs {
		name := dut.Port(t, s.port).Name()
		if _, ok := egressIntfs[name]; !ok {
			egressIntfs[name] = newEthernetInterface(dut, name, portDescriptions[s.port])
		}
		// With InterfaceConfigVRFBeforeAddress the addresses are configured after the VRFs.
		configureEgressSubIf(dut, egressIntfs[name].GetOrCreateSubinterface(s.index), s, !vrfBeforeAddress)
	}
	for name, i := range egressIntfs {
		gnmi.BatchUpdate(b, gnmi.OC().Interface(name).Config(), i)
	}
	for _, vrf := range testVRFs() {
		gnmi.BatchReplace(b, gnmi.OC().NetworkInstance(vrf).Config(), &oc.NetworkInstance{
			Name: ygot.String(vrf),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		})
	}
	b.Set(t, dut)

	t.Log("Assigning sub-interfaces to network-instances")
	nis := make(map[string]*oc.NetworkInstance)
	assign := func(ni, intf string, index uint32) {
		if _, ok := nis[ni]; !ok {
			nis[ni] = &oc.NetworkInstance{Name: ygot.String(ni)}
		}
		niIntf := nis[ni].GetOrCreateInterface(fmt.Sprintf("%s.%d", intf, index))
		niIntf.Interface = ygot.String(intf)
		niIntf.Subinterface = ygot.Uint32(index)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		assign(defNI, p1.Name(), 0)
	}
	for _, s := range subIfs {
		switch {
		case s.ni != "":
			assign(s.ni, dut.Port(t, s.port).Name(), s.index)
		case deviations.ExplicitInterfaceInDefaultVRF(dut):
			assign(defNI, dut.Port(t, s.port).Name(), s.index)
		}
	}
	b = &gnmi.SetBatch{}
	for name, ni := range nis {
		gnmi.BatchUpdate(b, gnmi.OC().NetworkInstance(name).Config(), ni)
	}
	b.Set(t, dut)

	if vrfBeforeAddress {
		t.Log("Configuring egress sub-interface addresses")
		b = &gnmi.SetBatch{}
		for _, s := range subIfs {
			sub := &oc.Interface_Subinterface{Index: ygot.Uint32(s.index)}
			configureEgressSubIf(dut, sub, s, true)
			gnmi.BatchUpdate(b, gnmi.OC().Interface(dut.Port(t, s.port).Name()).Subinterface(s.index).Config(), sub)
		}
		b.Set(t, dut)
	}

	t.Log("Configuring static routes")
	b = &gnmi.SetBatch{}
	for i := 1; i <= vrfsPerFamily; i++ {
		_, ateV4 := egressAttrs(v4VRFSubIf(i))
		addStaticRoute(t, dut, b, v4VRF(i), fmt.Sprintf("198.18.%d.0/24", i), ateV4.IPv4)
		_, ateV6 := egressAttrs(v6VRFSubIf(i))
		addStaticRoute(t, dut, b, v6VRF(i), fmt.Sprintf("2001:db8:a:%x::/64", i), ateV6.IPv6)
	}
	_, ateDefault := egressAttrs(defaultSubIfIndex)
	routeV4, routeV6 := defaultNIRoutes(snapshot)
	if routeV4 != defaultRouteV4 {
		t.Logf("WARNING: %v uses the routes %s and %s instead of the default routes %s and %s of the README, see the TODO of defaultNIRoutes", dut.Vendor(), routeV4, routeV6, defaultRouteV4, defaultRouteV6)
	}
	addStaticRoute(t, dut, b, defNI, routeV4, ateDefault.IPv4)
	addStaticRoute(t, dut, b, defNI, routeV6, ateDefault.IPv6)
	b.Set(t, dut)
}

// flowSpec describes an OTG flow sent from ATE Port-1.
type flowSpec struct {
	name  string
	ipv6  bool   // IPv6 outer header, otherwise IPv4
	encap bool   // IPinIP or IPv6-in-IPv6 encapsulated, otherwise UDP
	src   string // outer source address
	dst   string // destination address of the outer and inner headers
}

func positiveV4Flow(i int) string { return fmt.Sprintf("PositiveFlow-V4-%d", i) }
func positiveV6Flow(i int) string { return fmt.Sprintf("PositiveFlow-V6-%d", i) }
func negativeV4Flow(i int) string { return fmt.Sprintf("NegativeFlow-V4-%d", i) }
func negativeV6Flow(i int) string { return fmt.Sprintf("NegativeFlow-V6-%d", i) }

// flowSpecs returns the 30 Positive Streams, the 30 Negative Streams, the Ghost Stream and the
// Shadow Stream of the README.
func flowSpecs() []flowSpec {
	var specs []flowSpec
	for i := 1; i <= vrfsPerFamily; i++ {
		v4Src, v4Dst := fmt.Sprintf("198.51.100.%d", i), fmt.Sprintf("198.18.%d.10", i)
		v6Src, v6Dst := fmt.Sprintf("2001:db8:100::%d", i), fmt.Sprintf("2001:db8:a:%x::10", i)
		specs = append(specs,
			flowSpec{name: positiveV4Flow(i), encap: true, src: v4Src, dst: v4Dst},
			flowSpec{name: positiveV6Flow(i), ipv6: true, encap: true, src: v6Src, dst: v6Dst},
			flowSpec{name: negativeV4Flow(i), src: v4Src, dst: v4Dst},
			flowSpec{name: negativeV6Flow(i), ipv6: true, src: v6Src, dst: v6Dst},
		)
	}
	return append(specs,
		// The Ghost Stream matches Rule 31 (VRF-GHOST).
		flowSpec{name: ghostFlow, encap: true, src: "198.51.100.31", dst: "198.18.31.10"},
		// The Shadow Stream matches only the catch-all Rule 100 (VRF-V4-15).
		flowSpec{name: shadowFlow, encap: true, src: "198.51.100.100", dst: "198.18.15.10"},
	)
}

// flowExpectation is the expected forwarding result of a flow.
type flowExpectation struct {
	drop bool   // the flow must be entirely dropped
	vlan uint32 // otherwise, the egress VLAN (DUT sub-interface index) of every packet
}

// expectationsWithPolicy returns the expected result of every flow while HA_VRF_SELECTION is
// applied to DUT Port-1.
func expectationsWithPolicy() map[string]flowExpectation {
	want := map[string]flowExpectation{
		ghostFlow:  {drop: true},
		shadowFlow: {vlan: v4VRFSubIf(vrfsPerFamily)},
	}
	for i := 1; i <= vrfsPerFamily; i++ {
		want[positiveV4Flow(i)] = flowExpectation{vlan: v4VRFSubIf(i)}
		want[positiveV6Flow(i)] = flowExpectation{vlan: v6VRFSubIf(i)}
		want[negativeV4Flow(i)] = flowExpectation{vlan: defaultSubIfIndex}
		want[negativeV6Flow(i)] = flowExpectation{vlan: defaultSubIfIndex}
	}
	return want
}

// expectationsWithoutPolicy returns the expected result of every flow once the policy is removed
// from DUT Port-1: every flow is forwarded by the DEFAULT network-instance.
func expectationsWithoutPolicy() map[string]flowExpectation {
	want := make(map[string]flowExpectation)
	for _, spec := range flowSpecs() {
		want[spec.name] = flowExpectation{vlan: defaultSubIfIndex}
	}
	return want
}

// addFlow adds a flow sent from ATE Port-1, with egress tracking on the VLAN ID of the received
// packets. The receive interface is set by setRxDevices.
func addFlow(top gosnappi.Config, spec flowSpec) {
	f := top.Flows().Add().SetName(spec.name)
	f.Metrics().SetEnable(true)
	if spec.ipv6 {
		f.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"})
	} else {
		f.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"})
	}
	f.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
	if spec.ipv6 {
		outer := f.Packet().Add().Ipv6()
		outer.Src().SetValue(spec.src)
		outer.Dst().SetValue(spec.dst)
		if spec.encap {
			inner := f.Packet().Add().Ipv6()
			inner.Src().SetValue(innerSrcV6)
			inner.Dst().SetValue(spec.dst)
		}
	} else {
		outer := f.Packet().Add().Ipv4()
		outer.Src().SetValue(spec.src)
		outer.Dst().SetValue(spec.dst)
		if spec.encap {
			inner := f.Packet().Add().Ipv4()
			inner.Src().SetValue(innerSrcV4)
			inner.Dst().SetValue(spec.dst)
		}
	}
	if !spec.encap {
		udp := f.Packet().Add().Udp()
		udp.SrcPort().SetValue(1024)
		udp.DstPort().SetValue(1024)
	}
	f.Size().SetFixed(flowFrameSize)
	f.Rate().SetPps(flowRatePps)
	f.EgressPacket().Add().Ethernet()
	f.EgressPacket().Add().Vlan().Id().MetricTags().Add().SetName(vlanTagName).SetOffset(vlanTagOffset).SetLength(vlanTagLength)
}

// setRxDevices points every flow at the ATE interface on which it is expected to arrive. Dropped
// flows use the Port-2 DEFAULT sub-interface: falling back to the DEFAULT network-instance is the
// realistic leak path, and the rules cannot forward the Ghost Stream anywhere else because no VRF
// has a route to its destination.
func setRxDevices(top gosnappi.Config, want map[string]flowExpectation) {
	ipv6 := make(map[string]bool)
	for _, spec := range flowSpecs() {
		ipv6[spec.name] = spec.ipv6
	}
	for _, f := range top.Flows().Items() {
		exp := want[f.Name()]
		vlan := exp.vlan
		if exp.drop {
			vlan = defaultSubIfIndex
		}
		rx := egressDeviceName(vlan) + ".IPv4"
		if ipv6[f.Name()] {
			rx = egressDeviceName(vlan) + ".IPv6"
		}
		f.TxRx().Device().SetRxNames([]string{rx})
	}
}

// configureATE returns the OTG configuration: ATE Port-1, one ATE interface per egress
// sub-interface and all the flows of the README, each received on the ATE interface that want
// expects. The README asks the OTG to advertise the 30 destination networks; no routing protocol
// is needed because the DUT reaches them with static routes, so they are the destinations of the
// flows behind the ATE egress interfaces.
func configureATE(t *testing.T, ate *ondatra.ATEDevice, want map[string]flowExpectation) gosnappi.Config {
	t.Helper()
	t.Log("Configuring OTG")
	top := gosnappi.NewConfig()
	for _, id := range dutPortIDs {
		top.Ports().Add().SetName(ate.Port(t, id).ID())
	}

	d1 := top.Devices().Add().SetName(atePort1.Name)
	eth1 := d1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	eth1.Connection().SetPortName(ate.Port(t, "port1").ID())
	eth1.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	eth1.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6").SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	for _, s := range egressSubIfs() {
		if s.index >= 1<<vlanTagLength {
			t.Fatalf("VLAN %d does not fit in the %d VLAN ID bits that egress tracking reports", s.index, vlanTagLength)
		}
		dutAttrs, ateAttrs := egressAttrs(s.index)
		dev := top.Devices().Add().SetName(ateAttrs.Name)
		eth := dev.Ethernets().Add().SetName(ateAttrs.Name + ".Eth").SetMac(ateAttrs.MAC)
		eth.Connection().SetPortName(ate.Port(t, s.port).ID())
		eth.Vlans().Add().SetName(ateAttrs.Name + "-VLAN").SetId(s.index)
		if s.ipv4 {
			eth.Ipv4Addresses().Add().SetName(ateAttrs.Name + ".IPv4").SetAddress(ateAttrs.IPv4).SetGateway(dutAttrs.IPv4).SetPrefix(uint32(ateAttrs.IPv4Len))
		}
		if s.ipv6 {
			eth.Ipv6Addresses().Add().SetName(ateAttrs.Name + ".IPv6").SetAddress(ateAttrs.IPv6).SetGateway(dutAttrs.IPv6).SetPrefix(uint32(ateAttrs.IPv6Len))
		}
	}

	for _, spec := range flowSpecs() {
		addFlow(top, spec)
	}
	setRxDevices(top, want)
	return top
}

// dutSnapshot holds the DUT configuration that the test modifies, captured before the test.
type dutSnapshot struct {
	interfaces       map[string]*oc.Interface             // keyed by port name, absent if not configured
	policyForwarding *oc.NetworkInstance_PolicyForwarding // DEFAULT network-instance, nil if not configured
	staticProtocol   *oc.NetworkInstance_Protocol         // DEFAULT network-instance, nil if not configured
	defaultNIIntfs   map[string]bool                      // DEFAULT network-instance interfaces that were configured
}

// snapshotDUT captures the DUT configuration that the test modifies. Each path is looked up on its
// own: some DUTs serve config lookups with gNMI Get, and a multi-path Get fails as a whole with
// NOT_FOUND when any path is absent, which would hide the paths that exist.
func snapshotDUT(t *testing.T, dut *ondatra.DUTDevice) *dutSnapshot {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	s := &dutSnapshot{interfaces: make(map[string]*oc.Interface), defaultNIIntfs: make(map[string]bool)}
	for _, p := range dutPortNames(t, dut) {
		if i, ok := gnmi.LookupConfig(t, dut, gnmi.OC().Interface(p).Config()).Val(); ok {
			s.interfaces[p] = i
		}
	}
	if pf, ok := gnmi.LookupConfig(t, dut, niPath.PolicyForwarding().Config()).Val(); ok {
		s.policyForwarding = pf
	}
	staticPath := niPath.Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	if static, ok := gnmi.LookupConfig(t, dut, staticPath.Config()).Val(); ok {
		s.staticProtocol = static
	}
	for _, id := range defaultNIInterfaceIDs(t, dut) {
		s.defaultNIIntfs[id] = gnmi.LookupConfig(t, dut, niPath.Interface(id).Config()).IsPresent()
	}
	return s
}

// cleanupStep runs one cleanup step and reports its failure without skipping the next steps.
func cleanupStep(t *testing.T, desc string, step func(t testing.TB)) {
	t.Helper()
	if errMsg := testt.CaptureFatal(t, step); errMsg != nil {
		t.Errorf("Cleanup: %s failed: %s", desc, *errMsg)
	}
}

// restoreDUT removes the test configuration and restores the configuration captured by
// snapshotDUT. Only the configuration that the test touched is restored. The default routes are
// restored first, in their own request, so that no other cleanup failure leaves the testbed's
// default routes pointing at the ATE. The rest is restored in an order that never leaves a
// dangling reference: the policy, so that no rule references the VRFs, then the VRFs, so that no
// sub-interface is still bound, then the ports.
func restoreDUT(t *testing.T, dut *ondatra.DUTDevice, s *dutSnapshot) {
	t.Helper()
	t.Log("Cleanup: removing the test configuration and restoring the original DUT configuration")
	niPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	staticPath := niPath.Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	defNIIntfs := defaultNIInterfaceIDs(t, dut)
	ports := dutPortNames(t, dut)
	touchedSubIfs := map[string][]uint32{dut.Port(t, "port1").Name(): {0}}
	for _, sub := range egressSubIfs() {
		name := dut.Port(t, sub.port).Name()
		touchedSubIfs[name] = append(touchedSubIfs[name], sub.index)
	}

	cleanupStep(t, "restoring the DEFAULT network-instance default routes", func(t testing.TB) {
		b := &gnmi.SetBatch{}
		if s.staticProtocol == nil {
			gnmi.BatchDelete(b, staticPath.Config())
		} else {
			// Only the two default routes of the test are restored: the static protocol can also
			// hold other testbed routes.
			routeV4, routeV6 := defaultNIRoutes(s)
			for _, prefix := range []string{routeV4, routeV6} {
				if orig := s.staticProtocol.GetStatic(prefix); orig != nil {
					gnmi.BatchReplace(b, staticPath.Static(prefix).Config(), orig)
				} else {
					gnmi.BatchDelete(b, staticPath.Static(prefix).Config())
				}
			}
		}
		b.Set(t, dut)
	})

	cleanupStep(t, "restoring the DEFAULT network-instance policy-forwarding", func(t testing.TB) {
		if s.policyForwarding != nil {
			gnmi.Replace(t, dut, niPath.PolicyForwarding().Config(), s.policyForwarding)
		} else {
			gnmi.Delete(t, dut, niPath.PolicyForwarding().Config())
		}
	})

	cleanupStep(t, "removing the test network-instances", func(t testing.TB) {
		b := &gnmi.SetBatch{}
		for _, id := range defNIIntfs {
			if !s.defaultNIIntfs[id] {
				gnmi.BatchDelete(b, niPath.Interface(id).Config())
			}
		}
		for _, vrf := range testVRFs() {
			gnmi.BatchDelete(b, gnmi.OC().NetworkInstance(vrf).Config())
		}
		b.Set(t, dut)
	})

	cleanupStep(t, "restoring DUT ports 1-4", func(t testing.TB) {
		b := &gnmi.SetBatch{}
		for _, p := range ports {
			intfPath := gnmi.OC().Interface(p)
			orig, ok := s.interfaces[p]
			if !ok {
				gnmi.BatchDelete(b, intfPath.Config())
				continue
			}
			for _, index := range touchedSubIfs[p] {
				if sub := orig.GetSubinterface(index); sub != nil {
					gnmi.BatchReplace(b, intfPath.Subinterface(index).Config(), sub)
				} else {
					gnmi.BatchDelete(b, intfPath.Subinterface(index).Config())
				}
			}
			if orig.Description != nil {
				gnmi.BatchReplace(b, intfPath.Description().Config(), orig.GetDescription())
			} else {
				gnmi.BatchDelete(b, intfPath.Description().Config())
			}
			if orig.Enabled != nil {
				gnmi.BatchReplace(b, intfPath.Enabled().Config(), orig.GetEnabled())
			} else if deviations.InterfaceEnabled(dut) {
				gnmi.BatchDelete(b, intfPath.Enabled().Config())
			}
		}
		b.Set(t, dut)
	})
}

// waitForNeighbors waits until ARP and IPv6 neighbor discovery complete on every ATE interface.
// IPv6 is checked in a subtest so that a failure is reported without aborting the IPv4 validation.
func waitForNeighbors(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	t.Run("IPv6 neighbor discovery", func(t *testing.T) {
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
	})
}

// setupEnvironment configures the README test environment on the DUT and ATE, with every flow
// received on the ATE interface that want expects, and registers the cleanup that restores the
// DUT. It returns the OTG configuration.
func setupEnvironment(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, want map[string]flowExpectation) gosnappi.Config {
	t.Helper()
	snapshot := snapshotDUT(t, dut)
	t.Cleanup(func() { restoreDUT(t, dut, snapshot) })

	configureDUT(t, dut, snapshot)
	top := configureATE(t, ate, want)
	ate.OTG().PushConfig(t, top)
	// ATE ports may flap when the configuration is pushed.
	helpers.ValidateOperStatusUPIntfs(t, dut, dutPortNames(t, dut), setupPortUpTimeout)
	ate.OTG().StartProtocols(t)
	waitForNeighbors(t, ate, top)
	return top
}

// sortedKeys returns the keys of m in increasing order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// startTraffic starts all flows and waits until they have transmitted minTrafficPkts packets. It
// returns a function that stops the traffic; the function is also registered as a cleanup, and
// stops the traffic only once.
func startTraffic(t *testing.T, ate *ondatra.ATEDevice, want map[string]flowExpectation) (stop func()) {
	t.Helper()
	ate.OTG().StartTraffic(t)
	running := true
	stop = func() {
		if running {
			running = false
			ate.OTG().StopTraffic(t)
		}
	}
	t.Cleanup(stop)
	awaitTrafficProgress(t, ate, want)
	return stop
}

// progressFlow returns the flow whose counters show the progress of the traffic. All the flows
// start together at the same rate, so one flow stands for all of them: streaming the state of all
// the flows, with their egress-tracking metrics, is too heavy for the traffic generator.
func progressFlow(want map[string]flowExpectation) string {
	return sortedKeys(want)[0]
}

// awaitTrafficProgress waits until the traffic has transmitted at least minTrafficPkts packets more
// than when the wait started.
func awaitTrafficProgress(t *testing.T, ate *ondatra.ATEDevice, want map[string]flowExpectation) {
	t.Helper()
	name := progressFlow(want)
	var start, last uint64
	seen := false
	_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(name).Counters().OutPkts().State(), trafficTimeout, func(v *ygnmi.Value[uint64]) bool {
		out, present := v.Val()
		if !present {
			return false
		}
		last = out
		if !seen {
			start, seen = out, true
		}
		return out >= start+minTrafficPkts
	}).Await(t)
	if ok {
		return
	}
	// Distinguish a traffic generator that does not transmit from one that does not report.
	port1 := gnmi.OTG().Port(ate.Port(t, "port1").ID())
	if frames, present := gnmi.Lookup(t, ate.OTG(), port1.Counters().OutFrames().State()).Val(); present {
		t.Logf("ATE Port-1 has transmitted %d frames", frames)
	} else {
		t.Log("ATE Port-1 does not report its transmitted frames")
	}
	if seen {
		t.Fatalf("Flow %s transmitted %d packets, want %d more than %d, within %v", name, last, minTrafficPkts, start, trafficTimeout)
	}
	t.Fatalf("Flow %s reported no transmitted packets within %v", name, trafficTimeout)
}

// taggedVLAN returns the VLAN ID reported by an OTG egress-tracking metric: the tracked
// low-order bits, which hold the whole VLAN ID of every egress sub-interface.
func taggedVLAN(tm *gnmiotg.Flow_TaggedMetric) (uint32, error) {
	for _, tag := range tm.Tags {
		if tag.GetTagName() != vlanTagName {
			continue
		}
		v := tag.GetTagValue()
		switch {
		case v.GetValueAsHex() != "":
			id, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(v.GetValueAsHex()), "0x"), 16, 16)
			if err != nil {
				return 0, fmt.Errorf("invalid %s value %q: %v", vlanTagName, v.GetValueAsHex(), err)
			}
			return uint32(id), nil
		case v != nil && v.ValueAsCounter64 != nil:
			return uint32(v.GetValueAsCounter64()), nil
		default:
			return 0, fmt.Errorf("egress tracking metric %q has a %s value that is neither hex nor counter64", tm.GetNameValuePairs(), vlanTagName)
		}
	}
	return 0, fmt.Errorf("egress tracking metric %q has no %s tag", tm.GetNameValuePairs(), vlanTagName)
}

// pktsOnVLAN returns the received packets of a flow that egress tracking counted on vlan.
func pktsOnVLAN(f *gnmiotg.Flow, vlan uint32) uint64 {
	var pkts uint64
	for _, tm := range f.TaggedMetric {
		if v, err := taggedVLAN(tm); err == nil && v == vlan {
			pkts += tm.GetCounters().GetInPkts()
		}
	}
	return pkts
}

// taggedInPkts returns the received packets of a flow that egress tracking counted on any VLAN.
func taggedInPkts(f *gnmiotg.Flow) uint64 {
	var pkts uint64
	for _, tm := range f.TaggedMetric {
		pkts += tm.GetCounters().GetInPkts()
	}
	return pkts
}

// unconvergedFlows returns the flows that must be forwarded but were not received on their
// expected VLAN.
func unconvergedFlows(flows map[string]*gnmiotg.Flow, want map[string]flowExpectation) []string {
	var pending []string
	for _, name := range sortedKeys(want) {
		if exp := want[name]; !exp.drop && pktsOnVLAN(flows[name], exp.vlan) == 0 {
			pending = append(pending, name)
		}
	}
	return pending
}

// settleFlow returns the flow whose counters show that the traffic generator has finished
// counting: a flow that must be forwarded, so that its received packets catch up with the
// transmitted ones.
func settleFlow(want map[string]flowExpectation) string {
	for _, name := range sortedKeys(want) {
		if !want[name].drop {
			return name
		}
	}
	return progressFlow(want)
}

// awaitFlowsStopped waits until the traffic has stopped and, if allReceived is set, a flow that
// must be forwarded has received as many packets as it transmitted, all counted by egress
// tracking. Only that flow is watched: streaming the state of all the flows is too heavy for the
// traffic generator. It then returns the state of all the flows, also on timeout so that the
// verification reports the actual counters.
func awaitFlowsStopped(t *testing.T, ate *ondatra.ATEDevice, want map[string]flowExpectation, allReceived bool) map[string]*gnmiotg.Flow {
	t.Helper()
	name := settleFlow(want)
	_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(name).State(), counterSettleTimeout, func(v *ygnmi.Value[*gnmiotg.Flow]) bool {
		f, present := v.Val()
		if !present {
			return false
		}
		c := f.GetCounters()
		done := !f.GetTransmit() && c.GetOutPkts() > 0
		if done && allReceived {
			done = c.GetInPkts() >= c.GetOutPkts() && taggedInPkts(f) >= c.GetInPkts()
		}
		return done
	}).Await(t)
	if !ok {
		t.Logf("Counters of flow %s did not settle within %v, verifying the latest counters", name, counterSettleTimeout)
	}

	// Each flow is read on its own: the traffic generator cannot build the state of all the flows,
	// with their egress-tracking metrics, within its deadline, and stays busy for the next requests
	// when it fails to.
	flows := make(map[string]*gnmiotg.Flow, len(want))
	for _, name := range sortedKeys(want) {
		flows[name] = gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(name).State())
	}
	return flows
}

// canaryFlows returns a small representative subset of flows (one per family/role) used by
// awaitConvergence to check whether the DUT has programmed forwarding without reading all 62
// flows (~6.5 minutes on Ixia hardware) on every convergence window.
func canaryFlows(want map[string]flowExpectation) map[string]flowExpectation {
	canaries := make(map[string]flowExpectation)
	for _, name := range []string{
		positiveV4Flow(vrfsPerFamily),
		positiveV6Flow(vrfsPerFamily),
		negativeV4Flow(vrfsPerFamily),
		negativeV6Flow(vrfsPerFamily),
		shadowFlow,
	} {
		if exp, ok := want[name]; ok && !exp.drop {
			canaries[name] = exp
		}
	}
	return canaries
}

// flowsLossless reports whether every flow in flowsAlreadyRead met the full 0%-loss (or 100%-drop)
// and egress-VLAN expectations of want.
func flowsLossless(flows map[string]*gnmiotg.Flow, want map[string]flowExpectation) bool {
	for _, name := range sortedKeys(want) {
		exp, f := want[name], flows[name]
		c := f.GetCounters()
		if c == nil || c.GetOutPkts() == 0 {
			return false
		}
		out, in := c.GetOutPkts(), c.GetInPkts()
		lossPct := (float64(out) - float64(in)) * 100 / float64(out)
		if exp.drop {
			if lossPct < minDropPct {
				return false
			}
			continue
		}
		if lossPct > maxLossPct || in > out || pktsOnVLAN(f, exp.vlan) != in {
			return false
		}
	}
	return true
}

// logFlows logs the transmitted and received packets of every flow.
func logFlows(t *testing.T, flows map[string]*gnmiotg.Flow, want map[string]flowExpectation) {
	t.Helper()
	var summary strings.Builder
	fmt.Fprintf(&summary, "\n%-25s%-15s%-15s\n", "Flow", "Tx packets", "Rx packets")
	for _, name := range sortedKeys(want) {
		c := flows[name].GetCounters()
		fmt.Fprintf(&summary, "%-25s%-15d%-15d\n", name, c.GetOutPkts(), c.GetInPkts())
	}
	t.Log(summary.String())
}

// verifyEgressVLAN verifies with the OTG egress-tracking metrics that no packet of a flow was
// received on another VLAN than wantVLAN, i.e. that the flow left the DUT only on the expected
// sub-interface. If complete is set, it also verifies that every received packet was tracked.
func verifyEgressVLAN(t *testing.T, name string, f *gnmiotg.Flow, wantVLAN uint32, complete bool) {
	t.Helper()
	for _, tm := range f.TaggedMetric {
		pkts := tm.GetCounters().GetInPkts()
		if pkts == 0 {
			continue
		}
		if vlan, err := taggedVLAN(tm); err != nil {
			t.Errorf("Flow %s: %v", name, err)
		} else if vlan != wantVLAN {
			t.Errorf("Flow %s: %d packets received on VLAN %d, want VLAN %d", name, pkts, vlan, wantVLAN)
		}
	}
	if !complete {
		return
	}
	if onVLAN, in := pktsOnVLAN(f, wantVLAN), f.GetCounters().GetInPkts(); onVLAN != in {
		t.Errorf("Flow %s: egress tracking counted %d packets on VLAN %d, want all %d received packets", name, onVLAN, wantVLAN, in)
	}
}

// verifyFlows waits for the flow counters to settle after traffic was stopped, then verifies the
// loss and the egress VLAN of every flow.
func verifyFlows(t *testing.T, ate *ondatra.ATEDevice, want map[string]flowExpectation) {
	t.Helper()
	flows := awaitFlowsStopped(t, ate, want, true)
	logFlows(t, flows, want)
	for _, name := range sortedKeys(want) {
		exp, f := want[name], flows[name]
		c := f.GetCounters()
		if c == nil {
			t.Errorf("Flow %s: metrics missing", name)
			continue
		}
		out, in := c.GetOutPkts(), c.GetInPkts()
		if out == 0 {
			t.Errorf("Flow %s: no packets transmitted", name)
			continue
		}
		lossPct := (float64(out) - float64(in)) * 100 / float64(out)
		if exp.drop {
			if lossPct < minDropPct {
				t.Errorf("Flow %s: got %.4f%% loss (out=%d, in=%d), want %v%% (entirely dropped)", name, lossPct, out, in, minDropPct)
			}
			continue
		}
		if lossPct > maxLossPct || in > out {
			t.Errorf("Flow %s: got %.4f%% loss (out=%d, in=%d), want %v%% loss", name, lossPct, out, in, maxLossPct)
		}
		verifyEgressVLAN(t, name, f, exp.vlan, true)
	}
}

// verifyNoLeakage waits for the flows to stop, then verifies that dropped flows were entirely
// dropped and that no forwarded flow was received on another VLAN than its expected one. Loss is
// not verified, so that a measurement which spans an expected outage can be verified. It reports
// whether all flows also achieved 0% loss on their expected VLANs during that window.
func verifyNoLeakage(t *testing.T, ate *ondatra.ATEDevice, want map[string]flowExpectation) bool {
	t.Helper()
	flows := awaitFlowsStopped(t, ate, want, false)
	logFlows(t, flows, want)
	for _, name := range sortedKeys(want) {
		exp, f := want[name], flows[name]
		if exp.drop {
			if in := f.GetCounters().GetInPkts(); in != 0 {
				t.Errorf("Flow %s: %d packets received, want the flow entirely dropped", name, in)
			}
			continue
		}
		verifyEgressVLAN(t, name, f, exp.vlan, false)
	}
	return flowsLossless(flows, want)
}

// awaitConvergence runs short traffic windows until one shows the representative canary flows
// received on their expected VLANs, so that a following measurement does not include the time that
// the DUT takes to program forwarding. Only canaryFlows(want) are read after each window, because
// reading egress-tracking metrics for all 62 flows takes ~6.5 minutes on Ixia hardware.
func awaitConvergence(t *testing.T, ate *ondatra.ATEDevice, want map[string]flowExpectation) {
	t.Helper()
	canaries := canaryFlows(want)
	deadline := time.Now().Add(forwardingTimeout)
	for window := 1; ; window++ {
		stop := startTraffic(t, ate, want)
		stop()
		pending := unconvergedFlows(awaitFlowsStopped(t, ate, canaries, false), canaries)
		if len(pending) == 0 {
			t.Logf("Every stream is received on its expected VLAN after %d traffic windows", window)
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("Flows not received on their expected VLAN within %v: %v", forwardingTimeout, pending)
			return
		}
		t.Logf("Traffic window %d: flows not received on their expected VLAN yet: %v", window, pending)
	}
}

// measureTraffic sends minTrafficPkts packets per flow, then verifies the loss and the egress VLAN
// of every flow. The traffic generator clears the flow counters when traffic starts.
func measureTraffic(t *testing.T, ate *ondatra.ATEDevice, want map[string]flowExpectation) {
	t.Helper()
	stop := startTraffic(t, ate, want)
	stop()
	verifyFlows(t, ate, want)
}

// deleteGhostVRF deletes VRF-GHOST while Rule 31 still references it, as required by the README.
// The rule action is an OpenConfig leafref, so if a DUT enforces referential integrity and rejects
// the deletion, it verifies that VRF-GHOST remains empty (no interfaces or routes) and that Rule 31
// remains active pointing to VRF-GHOST so the Ghost Stream is still dropped.
func deleteGhostVRF(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	path := gnmi.OC().NetworkInstance(ghostVRF)
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Delete(t, dut, path.Config())
	}); errMsg != nil {
		t.Logf("Deleting %s while %s rule 31 references it was rejected by OpenConfig leafref validation; verifying %s has no interfaces/routes and rule 31 remains active: %s", ghostVRF, haPolicyName, ghostVRF, *errMsg)
		if ni, ok := gnmi.LookupConfig(t, dut, path.Config()).Val(); !ok {
			t.Errorf("%s config is missing after rejected deletion", ghostVRF)
		} else {
			if len(ni.Interface) != 0 {
				t.Errorf("%s has %d interfaces configured, want 0", ghostVRF, len(ni.Interface))
			}
			if sp := ni.GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)); sp != nil && len(sp.Static) != 0 {
				t.Errorf("%s has %d static routes configured, want 0", ghostVRF, len(sp.Static))
			}
		}
		ruleNI := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(haPolicyName).Rule(31).Action().NetworkInstance()
		if got, ok := gnmi.Lookup(t, dut, ruleNI.State()).Val(); ok {
			if got != ghostVRF {
				t.Errorf("%s rule 31 state network-instance got %q, want %q", haPolicyName, got, ghostVRF)
			}
		} else if got, ok := gnmi.LookupConfig(t, dut, ruleNI.Config()).Val(); !ok || got != ghostVRF {
			t.Errorf("%s rule 31 config network-instance got %q (present=%v), want %q", haPolicyName, got, ok, ghostVRF)
		}
		return
	}
	if gnmi.LookupConfig(t, dut, path.Config()).IsPresent() {
		t.Errorf("%s is still configured after it was deleted", ghostVRF)
	}
}

// awaitAcrossReconnect runs op until it reports done or timeout expires. Supervisor switchovers and
// linecard power transitions can tear down the gNMI connection to the DUT, so op is run again after
// a gNMI failure or after an attempt that ended without the expected result. Each attempt is
// limited to reconnectAttemptTimeout so that a stream left half-open by a switchover is replaced.
// op detects states with gnmi.Watch: this is the only retry loop of the test and it does not poll.
func awaitAcrossReconnect(t testing.TB, desc string, timeout time.Duration, op func(t testing.TB, attemptTimeout time.Duration) bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr string
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("%s: not completed within %v", desc, timeout)
		}
		attemptTimeout := min(remaining, reconnectAttemptTimeout)
		var done bool
		errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			done = op(t, attemptTimeout)
		})
		if errMsg == nil && done {
			return
		}
		if errMsg != nil && *errMsg != lastErr {
			lastErr = *errMsg
			t.Logf("%s: gNMI request failed, retrying: %s", desc, *errMsg)
		}
	}
}

// awaitState waits until q reaches want, across gNMI connection losses.
func awaitState[T comparable](t testing.TB, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T], want T, timeout time.Duration) {
	t.Helper()
	awaitAcrossReconnect(t, fmt.Sprintf("waiting for %v to be %v", q, want), timeout, func(t testing.TB, attemptTimeout time.Duration) bool {
		_, ok := gnmi.Watch(t, dut, q, attemptTimeout, func(v *ygnmi.Value[T]) bool {
			got, present := v.Val()
			return present && got == want
		}).Await(t)
		return ok
	})
}

// verifySwitchoverTelemetry verifies the last-switchover-time and last-switchover-reason of the
// new primary controller card.
func verifySwitchoverTelemetry(t *testing.T, dut *ondatra.DUTDevice, controller string) {
	t.Helper()
	comp := gnmi.OC().Component(controller)
	if switchoverTime, ok := gnmi.Lookup(t, dut, comp.LastSwitchoverTime().State()).Val(); ok {
		t.Logf("Component %s: last-switchover-time %v", controller, switchoverTime)
	} else {
		t.Errorf("Component %s: last-switchover-time is missing", controller)
	}
	reason, ok := gnmi.Lookup(t, dut, comp.LastSwitchoverReason().State()).Val()
	if !ok {
		t.Errorf("Component %s: last-switchover-reason is missing", controller)
		return
	}
	t.Logf("Component %s: last-switchover-reason trigger %v, details %q", controller, reason.GetTrigger(), reason.GetDetails())
	wantTrigger := oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED
	if deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
		wantTrigger = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_SYSTEM_INITIATED
	}
	if got := reason.GetTrigger(); got != wantTrigger {
		t.Errorf("Component %s: last-switchover-reason trigger got %v, want %v", controller, got, wantTrigger)
	}
}

// linecardForPort returns the linecard component hosting a DUT port.
func linecardForPort(t *testing.T, dut *ondatra.DUTDevice, portID string) string {
	t.Helper()
	p := dut.Port(t, portID)
	start := p.Name()
	if hwPort, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State()).Val(); ok && hwPort != "" {
		start = hwPort
	}
	return cfgplugins.FindLineCardParent(t, dut, start)
}

// setLinecardPower sets the power-admin-state of a linecard, across gNMI connection losses.
func setLinecardPower(t testing.TB, dut *ondatra.DUTDevice, lc string, state oc.E_Platform_ComponentPowerType) {
	t.Helper()
	awaitAcrossReconnect(t, fmt.Sprintf("setting %s power-admin-state to %v", lc, state), configRetryTimeout, func(t testing.TB, _ time.Duration) bool {
		if deviations.PowerDisableEnableLeafRefValidation(dut) {
			gnmi.Update(t, dut, gnmi.OC().Component(lc).Config(), &oc.Component{Name: ygot.String(lc)})
		}
		gnmi.Replace(t, dut, gnmi.OC().Component(lc).Linecard().PowerAdminState().Config(), state)
		return true
	})
}

// restoreLinecard powers on a linecard, waits until it and the DUT ports are up, and deletes the
// power-admin-state configuration if the test created it.
func restoreLinecard(t *testing.T, dut *ondatra.DUTDevice, lc string, powerConfigured bool) {
	t.Helper()
	t.Logf("Cleanup: ensuring linecard %s is POWER_ENABLED and ACTIVE", lc)
	lcPath := gnmi.OC().Component(lc)
	setLinecardPower(t, dut, lc, oc.Platform_ComponentPowerType_POWER_ENABLED)
	awaitState(t, dut, lcPath.OperStatus().State(), oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE, lcPowerUpTimeout)
	helpers.ValidateOperStatusUPIntfs(t, dut, dutPortNames(t, dut), oirPortUpTimeout)
	if powerConfigured {
		return
	}
	// POWER_ENABLED is the default, so deleting the leaf does not power-cycle the linecard.
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Delete(t, dut, lcPath.Linecard().PowerAdminState().Config())
	}); errMsg != nil {
		t.Logf("Cleanup: power-admin-state of %s stays configured to its default POWER_ENABLED: %s", lc, *errMsg)
	}
}

func TestVrfSelectionResiliency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	withPolicy := expectationsWithPolicy()
	top := setupEnvironment(t, dut, ate, withPolicy)
	// RT-3.4.2 to RT-3.4.4 test the policy that RT-3.4.1 applies to DUT Port-1: without it they
	// would switch over and power-cycle the DUT for nothing, and RT-3.4.4 would pass vacuously.
	policyApplied := false
	requirePolicy := func(t *testing.T) {
		t.Helper()
		if !policyApplied {
			t.Skipf("%s is not applied to DUT Port-1: RT-3.4.1 failed to configure it", haPolicyName)
		}
	}

	t.Run("RT-3.4.1 - Scaled VRF Selection Policy Hardware Programming", func(t *testing.T) {
		t.Log("Step 1 & 2 - Generate the VRF Selection Policy and push it with gNMI REPLACE")
		vrfpolicy.ConfigureHAVRFSelectionPolicy(t, dut, haPolicyName)
		policyApplied = true

		t.Run("Delete VRF-GHOST", func(t *testing.T) {
			deleteGhostVRF(t, dut)
		})

		t.Log("Step 3 - Send Traffic until the DUT forwards every stream on its expected VLAN")
		awaitConvergence(t, ate, withPolicy)

		t.Log("Step 4 - Validation (Baseline)")
		measureTraffic(t, ate, withPolicy)
	})

	t.Run("RT-3.4.2 - VRF Selection Policy Resilience Post Supervisor Switchover", func(t *testing.T) {
		requirePolicy(t)
		t.Log("Step 1 - Trigger Switchover")
		supervisors := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
		t.Logf("Found controller cards: %v", supervisors)
		if len(supervisors) < 2 {
			t.Fatalf("Controller cards on %v: got %d, want at least 2 (the README requires an MFF DUT)", dut.Model(), len(supervisors))
		}
		standby, active := components.FindStandbyControllerCard(t, dut, supervisors)
		components.AwaitSwitchoverReady(t, dut, active, switchoverReadyTimeout)

		// The streams run before, during and after the switchover, so that the measurement covers
		// the whole control plane transition.
		stopTraffic := startTraffic(t, ate, withPolicy)
		switchoverReq := system.NewSwitchControlProcessorOperation().Path(components.GetSubcomponentPath(standby, deviations.GNOISubcomponentPath(dut)))
		t.Logf("SwitchControlProcessor response: %v", gnoi.Execute(t, dut, switchoverReq))

		t.Logf("Waiting for %s to become the ACTIVE primary controller card", standby)
		compPath := gnmi.OC().Component(standby)
		awaitState(t, dut, compPath.RedundantRole().State(), oc.Platform_ComponentRedundantRole_PRIMARY, switchoverTimeout)
		// Restore redundancy before the next subtest: the former primary rejoins as standby.
		t.Cleanup(func() {
			components.AwaitSwitchoverReady(t, dut, standby, switchoverTimeout)
		})
		awaitState(t, dut, compPath.OperStatus().State(), oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE, switchoverTimeout)
		verifySwitchoverTelemetry(t, dut, standby)

		t.Log("Step 2 - Validation: 0% loss across the switchover and the Ghost Stream is dropped")
		awaitTrafficProgress(t, ate, withPolicy)
		stopTraffic()
		if !verifyNoLeakage(t, ate, withPolicy) {
			measureTraffic(t, ate, withPolicy)
		}
	})

	t.Run("RT-3.4.3 - VRF Selection Policy Resilience Post Linecard OIR", func(t *testing.T) {
		requirePolicy(t)
		t.Log("Step 1 - Perform Linecard Soft OIR")
		lc := linecardForPort(t, dut, "port1")
		lcPath := gnmi.OC().Component(lc)
		powerConfigured := gnmi.LookupConfig(t, dut, lcPath.Linecard().PowerAdminState().Config()).IsPresent()

		// The streams keep running through the OIR.
		stopTraffic := startTraffic(t, ate, withPolicy)
		// The linecard is restored from here on, also if the power-off request fails midway.
		t.Cleanup(func() { restoreLinecard(t, dut, lc, powerConfigured) })
		t.Logf("Powering off linecard %s", lc)
		setLinecardPower(t, dut, lc, oc.Platform_ComponentPowerType_POWER_DISABLED)
		awaitState(t, dut, lcPath.Linecard().PowerAdminState().State(), oc.Platform_ComponentPowerType_POWER_DISABLED, lcPowerDownTimeout)
		awaitState(t, dut, lcPath.OperStatus().State(), oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED, lcPowerDownTimeout)

		t.Logf("Powering on linecard %s", lc)
		setLinecardPower(t, dut, lc, oc.Platform_ComponentPowerType_POWER_ENABLED)
		if !deviations.MissingValueForDefaults(dut) {
			awaitState(t, dut, lcPath.Linecard().PowerAdminState().State(), oc.Platform_ComponentPowerType_POWER_ENABLED, lcPowerUpTimeout)
		}
		awaitState(t, dut, lcPath.OperStatus().State(), oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE, lcPowerUpTimeout)
		helpers.ValidateOperStatusUPIntfs(t, dut, dutPortNames(t, dut), oirPortUpTimeout)

		t.Log("Step 2 - Validation: the streams keep running after the ports are up")
		awaitTrafficProgress(t, ate, withPolicy)
		stopTraffic()
		t.Log("Step 2 - Validation: no cross-talk or leakage, and the Ghost Stream stayed dropped through the OIR")
		verifyNoLeakage(t, ate, withPolicy)

		t.Log("Step 2 - Validation: all streams autonomously recover to their VRFs")
		waitForNeighbors(t, ate, top)
		awaitConvergence(t, ate, withPolicy)
		t.Log("Step 2 - Validation: 0% loss and correct VRF mapping after the OIR")
		measureTraffic(t, ate, withPolicy)
	})

	t.Run("RT-3.4.4 - Policy Deletion", func(t *testing.T) {
		requirePolicy(t)
		t.Log("Step 1 & 2 - Remove the VRF Selection Policy from the ingress interface with a gNMI DELETE of its policy-forwarding interface")
		// The whole interface entry is deleted, not only apply-vrf-selection-policy: an entry
		// without a policy makes some DUTs reject every later policy-forwarding change, including
		// the cleanup.
		vrfpolicy.DeletePolicyForwarding(t, dut, "port1")

		t.Log("Step 3 - Validation: all streams are forwarded by the DEFAULT VRF")
		withoutPolicy := expectationsWithoutPolicy()
		setRxDevices(top, withoutPolicy)
		ate.OTG().PushConfig(t, top)
		helpers.ValidateOperStatusUPIntfs(t, dut, dutPortNames(t, dut), setupPortUpTimeout)
		ate.OTG().StartProtocols(t)
		waitForNeighbors(t, ate, top)
		awaitConvergence(t, ate, withoutPolicy)
		measureTraffic(t, ate, withoutPolicy)
	})
}
