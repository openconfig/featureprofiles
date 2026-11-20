// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plq

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	linkUpTimeout      = 5 * time.Minute
	maxAggSearchOffset = 64
)

var aggIndexRE = regexp.MustCompile(`\d+$`)

// DiscoverLinkPairs finds connected DUT port pairs matching targetSpeed (or all pairs when targetSpeed == 0).
func DiscoverLinkPairs(t *testing.T, targetSpeed ondatra.Speed) []LinkPair {
	t.Helper()
	duts := sortedDUTs(t)
	if len(duts) == 0 {
		t.Fatalf("Testbed contains no DUTs")
	}

	var pairs []LinkPair
	if len(duts) == 1 {
		pairs = discoverSingleDUTPairs(t, duts[0], targetSpeed)
	} else {
		pairs = discoverTwoDUTPairs(t, duts[0], duts[1], targetSpeed)
	}

	if len(pairs) == 0 {
		t.Fatalf("No matching linked port pairs found for target speed %v across %d DUT(s)", targetSpeed, len(duts))
	}
	return pairs
}

func sortedDUTs(t *testing.T) []*ondatra.DUTDevice {
	t.Helper()
	duts := ondatra.DUTs(t)
	result := make([]*ondatra.DUTDevice, 0, len(duts))
	for _, d := range duts {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID() < result[j].ID()
	})
	return result
}

func sortedPorts(dut *ondatra.DUTDevice) []*ondatra.Port {
	ports := append([]*ondatra.Port(nil), dut.Ports()...)
	sort.Slice(ports, func(i, j int) bool {
		return portOrderKey(ports[i].ID()) < portOrderKey(ports[j].ID())
	})
	return ports
}

func portOrderKey(id string) string {
	trimmed := strings.TrimLeftFunc(id, func(r rune) bool { return !unicode.IsDigit(r) })
	if n, err := strconv.Atoi(trimmed); err == nil {
		return fmt.Sprintf("%08d_%s", n, id)
	}
	return id
}

func discoverSingleDUTPairs(t *testing.T, dut *ondatra.DUTDevice, targetSpeed ondatra.Speed) []LinkPair {
	t.Helper()
	ports := sortedPorts(dut)
	var pairs []LinkPair
	for i := 0; i+1 < len(ports); i += 2 {
		pA, pB := ports[i], ports[i+1]
		if targetSpeed != 0 && (pA.Speed() != targetSpeed || pB.Speed() != targetSpeed) {
			continue
		}
		speedBPS := resolvePairSpeedBPS(t, dut, pA, dut, pB)
		pairs = append(pairs, LinkPair{
			A:        LinkEndpoint{DUT: dut, Port: pA},
			B:        LinkEndpoint{DUT: dut, Port: pB},
			SpeedBPS: speedBPS,
		})
	}
	return pairs
}

func discoverTwoDUTPairs(t *testing.T, dut1, dut2 *ondatra.DUTDevice, targetSpeed ondatra.Speed) []LinkPair {
	t.Helper()
	ports1 := sortedPorts(dut1)
	ports2ByID := make(map[string]*ondatra.Port, len(dut2.Ports()))
	for _, p := range dut2.Ports() {
		ports2ByID[p.ID()] = p
	}

	var pairs []LinkPair
	for _, pA := range ports1 {
		pB, ok := ports2ByID[pA.ID()]
		if !ok {
			continue
		}
		if targetSpeed != 0 && (pA.Speed() != targetSpeed || pB.Speed() != targetSpeed) {
			continue
		}
		speedBPS := resolvePairSpeedBPS(t, dut1, pA, dut2, pB)
		pairs = append(pairs, LinkPair{
			A:        LinkEndpoint{DUT: dut1, Port: pA},
			B:        LinkEndpoint{DUT: dut2, Port: pB},
			SpeedBPS: speedBPS,
		})
	}
	return pairs
}

func resolvePairSpeedBPS(t *testing.T, dutA *ondatra.DUTDevice, pA *ondatra.Port, dutB *ondatra.DUTDevice, pB *ondatra.Port) uint64 {
	t.Helper()
	speedA := resolvePortSpeedBPS(t, dutA, pA)
	speedB := resolvePortSpeedBPS(t, dutB, pB)
	if speedA == 0 {
		return speedB
	}
	if speedB == 0 || speedA <= speedB {
		return speedA
	}
	return speedB
}

func resolvePortSpeedBPS(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port) uint64 {
	t.Helper()
	eth := gnmi.OC().Interface(port.Name()).Ethernet()
	if !deviations.NegotiatedPortSpeedUnsupported(dut) {
		if neg, ok := gnmi.Lookup(t, dut, eth.NegotiatedPortSpeed().State()).Val(); ok {
			if bps := ParsePortSpeed(neg); bps > 0 {
				return bps
			}
		}
	}
	if cfgSpeed, ok := gnmi.Lookup(t, dut, eth.PortSpeed().State()).Val(); ok {
		if bps := ParsePortSpeed(cfgSpeed); bps > 0 {
			return bps
		}
	}
	if bps := SpeedToBPS(port.Speed()); bps > 0 {
		return bps
	}
	return ParsePortSpeed(fptest.GetIfSpeed(t, port))
}

type interfaceAddressing struct {
	ipv4    string
	ipv4Len uint8
	ipv6    string
	ipv6Len uint8
	descr   string
}

// ConfigureSingletonLinks configures dual-stack IPv4/IPv6 singleton L3 interfaces for each link pair
// and registers cleanup handlers to revert the test-specific state.
func ConfigureSingletonLinks(t *testing.T, pairs []LinkPair) {
	t.Helper()
	configuredDUTs := make(map[*ondatra.DUTDevice]bool)
	for _, pair := range pairs {
		for _, dut := range []*ondatra.DUTDevice{pair.A.DUT, pair.B.DUT} {
			if !configuredDUTs[dut] {
				fptest.ConfigureDefaultNetworkInstance(t, dut)
				configuredDUTs[dut] = true
			}
		}
	}

	for idx, pair := range pairs {
		addrA, addrB, err := linkAddresses(idx, pair.A.DUT == pair.B.DUT, pair.A.Port.Name(), pair.B.Port.Name())
		if err != nil {
			t.Fatalf("Failed to allocate singleton link addresses for pair %d: %v", idx, err)
		}
		configureSingletonPort(t, pair.A.DUT, pair.A.Port, addrA)
		configureSingletonPort(t, pair.B.DUT, pair.B.Port, addrB)
	}

	waitForLinksUp(t, pairs)
}

func linkAddresses(idx int, sameDUT bool, nameA, nameB string) (interfaceAddressing, interfaceAddressing, error) {
	if idx < 0 {
		return interfaceAddressing{}, interfaceAddressing{}, fmt.Errorf("link index must be >= 0, got %d", idx)
	}
	if sameDUT {
		// On a single DUT, each port is in the same default VRF and needs a non-overlapping subnet.
		// Reserve slots 0..125 for up to 63 single-DUT links, disjoint from LAG slots >= 126.
		if idx*2+1 >= 126 {
			return interfaceAddressing{}, interfaceAddressing{}, fmt.Errorf("single-DUT link index %d exceeds subnet slot space", idx)
		}
		a, err := endpointAddressing(idx*2, 1, fmt.Sprintf("PLQ Link %d Port %s", idx, nameA))
		if err != nil {
			return interfaceAddressing{}, interfaceAddressing{}, err
		}
		b, err := endpointAddressing(idx*2+1, 1, fmt.Sprintf("PLQ Link %d Port %s", idx, nameB))
		return a, b, err
	}
	if idx >= 126 {
		return interfaceAddressing{}, interfaceAddressing{}, fmt.Errorf("two-DUT link index %d exceeds subnet slot space", idx)
	}
	a, err := endpointAddressing(idx, 1, fmt.Sprintf("PLQ Link %d Port %s", idx, nameA))
	if err != nil {
		return interfaceAddressing{}, interfaceAddressing{}, err
	}
	b, err := endpointAddressing(idx, 2, fmt.Sprintf("PLQ Link %d Port %s", idx, nameB))
	return a, b, err
}

func lagAddresses(sameDUT bool, aggA, aggB string) (interfaceAddressing, interfaceAddressing, error) {
	const (
		twoDUTLagSlot     = 126
		singleDUTLagSlotA = 127
		singleDUTLagSlotB = 128
	)
	if sameDUT {
		a, err := endpointAddressing(singleDUTLagSlotA, 1, fmt.Sprintf("PLQ LAG %s", aggA))
		if err != nil {
			return interfaceAddressing{}, interfaceAddressing{}, err
		}
		b, err := endpointAddressing(singleDUTLagSlotB, 1, fmt.Sprintf("PLQ LAG %s", aggB))
		return a, b, err
	}
	a, err := endpointAddressing(twoDUTLagSlot, 1, fmt.Sprintf("PLQ LAG %s", aggA))
	if err != nil {
		return interfaceAddressing{}, interfaceAddressing{}, err
	}
	b, err := endpointAddressing(twoDUTLagSlot, 2, fmt.Sprintf("PLQ LAG %s", aggB))
	return a, b, err
}

func endpointAddressing(slot, hostOffset int, descr string) (interfaceAddressing, error) {
	// Allocate RFC 5737 /30 IPv4 and RFC 3849 /126 IPv6 addresses across 189 slots:
	// slots 0..62 -> 192.0.2.0/24, 63..125 -> 198.51.100.0/24, 126..188 -> 203.0.113.0/24.
	if slot < 0 || slot >= 189 {
		return interfaceAddressing{}, fmt.Errorf("subnet slot %d out of supported range [0, 188]", slot)
	}
	base := (slot % 63) * 4
	prefix := "192.0.2"
	switch {
	case slot >= 63 && slot < 126:
		prefix = "198.51.100"
	case slot >= 126:
		prefix = "203.0.113"
	}
	return interfaceAddressing{
		ipv4:    fmt.Sprintf("%s.%d", prefix, base+hostOffset),
		ipv4Len: 30,
		ipv6:    fmt.Sprintf("2001:db8:1:%x::%d", slot+1, hostOffset),
		ipv6Len: 126,
		descr:   descr,
	}, nil
}

func newBasePhysicalInterface(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port) *oc.Interface {
	t.Helper()
	i := &oc.Interface{
		Name: ygot.String(port.Name()),
		Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
	}
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	if deviations.ExplicitPortSpeed(dut) {
		i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, port)
	}
	return i
}

func configureSingletonPort(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port, addr interfaceAddressing) {
	t.Helper()
	i := newBasePhysicalInterface(t, dut, port)
	i.Description = ygot.String(addr.descr)

	configureSubinterfaceAddressing(dut, i, addr)

	gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, port.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	ensureOpticalChannelIfNeeded(t, dut, port)

	t.Cleanup(func() {
		revertSingletonPort(t, dut, port)
	})
}

func configureSubinterfaceAddressing(dut *ondatra.DUTDevice, intf *oc.Interface, addr interfaceAddressing) {
	sub := intf.GetOrCreateSubinterface(0)
	v4 := sub.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		v4.Enabled = ygot.Bool(true)
	}
	v4.GetOrCreateAddress(addr.ipv4).PrefixLength = ygot.Uint8(addr.ipv4Len)

	v6 := sub.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		v6.Enabled = ygot.Bool(true)
	}
	v6.GetOrCreateAddress(addr.ipv6).PrefixLength = ygot.Uint8(addr.ipv6Len)
}

func isZRPMD(pmd ondatra.PMD) bool {
	switch pmd {
	case ondatra.PMD400GBASEZR, ondatra.PMD400GBASEZRP, ondatra.PMD800GBASEZR, ondatra.PMD800GBASEZRP:
		return true
	default:
		return false
	}
}

func ensureOpticalChannelIfNeeded(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port) {
	t.Helper()
	if !isZRPMD(port.PMD()) {
		return
	}
	transceiverName, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(port.Name()).Transceiver().State()).Val()
	if !ok || transceiverName == "" {
		return
	}
	ochName, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiverName).Transceiver().Channel(0).AssociatedOpticalChannel().State()).Val()
	if !ok || ochName == "" {
		return
	}
	if freq, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(ochName).OpticalChannel().Frequency().State()).Val(); ok && freq > 0 {
		return
	}

	freqMHz, powerDBm, opMode := defaultOpticalParams(dut.Vendor(), port.PMD())
	och := &oc.Component_OpticalChannel{
		Frequency:         ygot.Uint64(freqMHz),
		TargetOutputPower: ygot.Float64(powerDBm),
	}
	if opMode != 0 {
		och.OperationalMode = ygot.Uint16(opMode)
	}
	gnmi.Replace(t, dut, gnmi.OC().Component(ochName).Config(), &oc.Component{
		Name:           ygot.String(ochName),
		OpticalChannel: och,
	})
}

func defaultOpticalParams(vendor ondatra.Vendor, pmd ondatra.PMD) (freqMHz uint64, powerDBm float64, opMode uint16) {
	freqMHz = 196100000
	switch pmd {
	case ondatra.PMD400GBASEZR:
		powerDBm = -10
		if vendor == ondatra.CISCO {
			opMode = 5003
		} else {
			opMode = 1
		}
	case ondatra.PMD400GBASEZRP:
		powerDBm = -7
		if vendor == ondatra.CISCO {
			opMode = 6004
		} else {
			opMode = 4
		}
	case ondatra.PMD800GBASEZR:
		powerDBm = -7
		if vendor == ondatra.CISCO {
			opMode = 6001
		} else {
			opMode = 1
		}
	case ondatra.PMD800GBASEZRP:
		powerDBm = -4
		if vendor == ondatra.CISCO {
			opMode = 6008
		} else {
			opMode = 8
		}
	}
	return freqMHz, powerDBm, opMode
}

func revertSingletonPort(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port) {
	t.Helper()
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		defNI := deviations.DefaultNetworkInstance(dut)
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(defNI).Interface(port.Name()+".0").Config())
	}
	// Replace with a bare physical interface so vendors (such as Cisco IOS-XR) that reject
	// removing subinterface 0 via gNMI Delete alone also clear L3 addresses before LAG setup.
	gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), newBasePhysicalInterface(t, dut, port))
}

func waitForLinksUp(t *testing.T, pairs []LinkPair) {
	t.Helper()
	for _, pair := range pairs {
		for _, ep := range []LinkEndpoint{pair.A, pair.B} {
			watchInterfaceUp(t, ep.DUT, ep.Port.Name())
		}
	}
}

func watchInterfaceUp(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	t.Helper()
	val, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(intfName).OperStatus().State(), linkUpTimeout, func(v *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := v.Val()
		return present && status == oc.Interface_OperStatus_UP
	}).Await(t)
	if !ok {
		t.Fatalf("Interface %s on %s did not reach OperStatus UP (got %v) within %v", intfName, dut.Name(), val, linkUpTimeout)
	}
}

// ConfigureLAGLinks configures dual-stack static LAGs across compatible link pairs and registers
// cleanup handlers to remove LAG configuration afterwards. On a single DUT, A and B ports are
// placed into two distinct aggregate interfaces so the LAG is not self-looped.
func ConfigureLAGLinks(t *testing.T, pairs []LinkPair) bool {
	t.Helper()
	if len(pairs) == 0 {
		return false
	}

	dutA, dutB, membersA, membersB, err := resolveLAGMembers(pairs)
	if err != nil {
		t.Fatalf("Failed to resolve LAG members: %v", err)
	}

	aggA := netutil.NextAggregateInterface(t, dutA)
	var aggB string
	if dutB == dutA {
		aggB = nextAvailableAggregateAfter(t, dutA, aggA)
	} else {
		aggB = netutil.NextAggregateInterface(t, dutB)
	}

	addrA, addrB, err := lagAddresses(dutA == dutB, aggA, aggB)
	if err != nil {
		t.Fatalf("Failed to allocate LAG addresses: %v", err)
	}
	configureStaticLAG(t, dutA, aggA, membersA, addrA)
	configureStaticLAG(t, dutB, aggB, membersB, addrB)
	for _, p := range pairs {
		ensureOpticalChannelIfNeeded(t, p.A.DUT, p.A.Port)
		ensureOpticalChannelIfNeeded(t, p.B.DUT, p.B.Port)
	}

	watchInterfaceUp(t, dutA, aggA)
	watchInterfaceUp(t, dutB, aggB)
	for _, p := range membersA {
		watchInterfaceUp(t, dutA, p.Name())
	}
	for _, p := range membersB {
		watchInterfaceUp(t, dutB, p.Name())
	}
	return true
}

func resolveLAGMembers(pairs []LinkPair) (*ondatra.DUTDevice, *ondatra.DUTDevice, []*ondatra.Port, []*ondatra.Port, error) {
	if len(pairs) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("no link pairs provided for LAG configuration")
	}
	dutA := pairs[0].A.DUT
	dutB := pairs[0].B.DUT
	membersA := make([]*ondatra.Port, 0, len(pairs))
	membersB := make([]*ondatra.Port, 0, len(pairs))
	for i, p := range pairs {
		if p.A.DUT == nil || p.B.DUT == nil || p.A.Port == nil || p.B.Port == nil {
			return nil, nil, nil, nil, fmt.Errorf("pair %d has nil DUT or port endpoint", i)
		}
		if p.A.DUT != dutA || p.B.DUT != dutB {
			return nil, nil, nil, nil, fmt.Errorf("inconsistent DUT endpoints across LAG pairs")
		}
		if p.A.DUT == p.B.DUT && p.A.Port == p.B.Port {
			return nil, nil, nil, nil, fmt.Errorf("cannot form LAG from identical port endpoints on pair %d", i)
		}
		membersA = append(membersA, p.A.Port)
		membersB = append(membersB, p.B.Port)
	}
	return dutA, dutB, membersA, membersB, nil
}

func newLAGMemberInterface(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port, aggName string) *oc.Interface {
	t.Helper()
	m := newBasePhysicalInterface(t, dut, port)
	m.GetOrCreateEthernet().AggregateId = ygot.String(aggName)
	return m
}

func configureStaticLAG(t *testing.T, dut *ondatra.DUTDevice, aggName string, members []*ondatra.Port, addr interfaceAddressing) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	agg := &oc.Interface{
		Name:        ygot.String(aggName),
		Description: ygot.String(addr.descr),
		Type:        oc.IETFInterfaces_InterfaceType_ieee8023adLag,
	}
	if deviations.InterfaceEnabled(dut) {
		agg.Enabled = ygot.Bool(true)
	}
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC

	configureSubinterfaceAddressing(dut, agg, addr)

	if deviations.AggregateAtomicUpdate(dut) {
		batch := &gnmi.SetBatch{}
		gnmi.BatchReplace(batch, gnmi.OC().Interface(aggName).Config(), agg)
		for _, p := range members {
			gnmi.BatchReplace(batch, gnmi.OC().Interface(p.Name()).Config(), newLAGMemberInterface(t, dut, p, aggName))
		}
		batch.Set(t, dut)
	} else {
		gnmi.Replace(t, dut, gnmi.OC().Interface(aggName).Config(), agg)
		for _, p := range members {
			gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Config(), newLAGMemberInterface(t, dut, p, aggName))
		}
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, aggName, deviations.DefaultNetworkInstance(dut), 0)
	}

	t.Cleanup(func() {
		revertStaticLAG(t, dut, aggName, members)
	})
}

func revertStaticLAG(t *testing.T, dut *ondatra.DUTDevice, aggName string, members []*ondatra.Port) {
	t.Helper()
	for _, p := range members {
		gnmi.Delete(t, dut, gnmi.OC().Interface(p.Name()).Ethernet().AggregateId().Config())
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		defNI := deviations.DefaultNetworkInstance(dut)
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(defNI).Interface(aggName+".0").Config())
	}
	gnmi.Delete(t, dut, gnmi.OC().Interface(aggName).Config())
}

func nextAvailableAggregateAfter(t *testing.T, dut *ondatra.DUTDevice, firstAgg string) string {
	t.Helper()
	loc := aggIndexRE.FindStringIndex(firstAgg)
	if loc == nil {
		t.Fatalf("Cannot extract trailing aggregate index from %q on %s", firstAgg, dut.Name())
	}
	start, err := strconv.Atoi(firstAgg[loc[0]:loc[1]])
	if err != nil {
		t.Fatalf("Cannot parse aggregate index from %q on %s: %v", firstAgg, dut.Name(), err)
	}
	batch := gnmi.OCBatch()
	batch.AddPaths(gnmi.OC().InterfaceAny().Aggregation())
	var existing map[string]*oc.Interface
	if root, present := gnmi.Lookup(t, dut, batch.State()).Val(); present {
		existing = root.Interface
	}
	prefix := firstAgg[:loc[0]]
	for offset := 1; offset <= maxAggSearchOffset; offset++ {
		candidate := prefix + strconv.Itoa(start+offset)
		if _, present := existing[candidate]; !present {
			return candidate
		}
	}
	t.Fatalf("No unused aggregate interface found after %q on %s within %d indices", firstAgg, dut.Name(), maxAggSearchOffset)
	return ""
}
