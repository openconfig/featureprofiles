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

package te_14_1_gribi_scaling_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	fpargs "github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1
// and dut:port2 -> ate:port2.
// There are DefaultVRFIPv4NHCount SubInterfaces between dut:port2
// and ate:port2
//
//   - ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - ate:port2 -> dut:port2 DefaultVRFIPv4NHCount Sub interfaces, e.g.:
//   - ate:port2.0 -> dut:port2.0 VLAN-ID: 0 subnet 198.51.0.0/30
//   - ate:port2.1 -> dut:port2.1 VLAN-ID: 1 subnet 198.51.0.4/30
//   - ate:port2.2 -> dut:port2.2 VLAN-ID: 2 subnet 198.51.0.8/30
//   - ate:port2.i -> dut:port2.i VLAN-ID i subnet 198.51.0.(4*i)/30
const (
	ipv4PrefixLen          = 30 // ipv4PrefixLen is the ATE and DUT interface IP prefix length
	vrf1                   = "VRF-A"
	vrf2                   = "VRF-B"
	vrf3                   = "VRF-C"
	IPBlockDefaultVRF      = "198.18.128.0/17"
	IPBlockNonDefaultVRF   = "198.18.0.0/17"
	tunnelSrcIP            = "198.19.204.1" // tunnelSrcIP represents Source IP of IPinIP Tunnel
	policyName             = "redirect-to-VRF1"
	StaticMAC              = "00:1A:11:00:00:01"
	subifBaseIP            = "198.51.0.0"
	nextHopStartIndex      = 101 // set > 2 to avoid overlap with backup NH ids 1&2
	nextHopGroupStartIndex = 101 // set > 2 to avoid overlap with backup NHG ids 1&2
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name: "atePort2",
	}
)

// routesParam holds parameters required for provisioning
// gRIBI IP entries, next-hop-groups and next-hops
type routesParam struct {
	ipEntries     []string
	numUniqueNHs  int
	nextHops      []string
	nextHopVRF    string
	startNHIndex  int
	numUniqueNHGs int
	numNHPerNHG   int
	startNHGIndex int
	nextHopWeight []int
	backupNHG     int
	nhDecapEncap  bool
}

// Parameters needed to provision next-hop with interface reference + static MAC
type nextHopIntfRef struct {
	nextHopIPAddress string
	subintfIndex     uint32
	intfName         string
}

// Generate weights for next hops when assigning to a next-hop-group
// Weights are allocated such that there is no common divisor
func generateNextHopWeights(weightSum int, nextHopCount int) []int {
	weights := []int{}

	switch {
	case nextHopCount == 1:
		weights = append(weights, weightSum)
	case weightSum <= nextHopCount:
		for i := 0; i < nextHopCount; i++ {
			weights = append(weights, 1)
		}
	case nextHopCount == 2:
		weights = append(weights, 1, weightSum-1)
	default:
		weights = append(weights, 1, 2)
		rem := (weightSum - 1 - 2) % (nextHopCount - 2)
		weights = append(weights, rem+(weightSum-1-2)/(nextHopCount-2))
		for i := 1; i < (nextHopCount - 2); i++ {
			weights = append(weights, (weightSum-1-2)/(nextHopCount-2))
		}
	}
	return weights
}

// incrementMAC increments the MAC by i. Returns error if the mac cannot be parsed or overflows the mac address space
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

// incrementIP increments the IPv4 address by i
func incrementIP(ip string, i int) string {
	ipAddr := net.ParseIP(ip)
	convIP := binary.BigEndian.Uint32(ipAddr.To4())
	convIP = convIP + uint32(i)
	newIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(newIP, convIP)
	return newIP.String()
}

// pushIPv4Entries pushes gRIBI IPv4 entries in a specified VRF, with corresponding NHs and NHGs in the default NI
func pushIPv4Entries(t *testing.T, virtualVIPs []string, decapEncapVirtualIPs []string, args *testArgs) {

	// install backup NHGs/NHs
	// NHG {ID #1} --> NH {ID #1, network-instance: VRF-C}
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(uint64(1)).
			WithNextHopNetworkInstance(vrf3).
			WithElectionID(args.electionID.Low, args.electionID.High))

	args.client.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(uint64(1)).
			AddNextHop(uint64(1), uint64(1)).
			WithElectionID(args.electionID.Low, args.electionID.High))

	// NHG {ID #2} --> NH {ID #2, decap, network-instance: DEFAULT}
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithIndex(uint64(2)).
			WithDecapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithElectionID(args.electionID.Low, args.electionID.High))

	args.client.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(uint64(2)).
			AddNextHop(uint64(2), uint64(1)).
			WithElectionID(args.electionID.Low, args.electionID.High))

	// provision non-default VRF gRIBI entries, and associated NHGs, NHs in default instance
	vrfEntryParams := make(map[string]*routesParam)
	vrfEntryParams[vrf1] = &routesParam{
		ipEntries:     createIPv4Entries(IPBlockNonDefaultVRF)[0:*fpargs.NonDefaultVRFIPv4Count],
		numUniqueNHs:  *fpargs.NonDefaultVRFIPv4NHGCount * *fpargs.NonDefaultVRFIPv4NHSize,
		nextHops:      virtualVIPs,
		nextHopVRF:    deviations.DefaultNetworkInstance(args.dut),
		startNHIndex:  nextHopStartIndex + *fpargs.DefaultVRFIPv4NHCount,
		numUniqueNHGs: *fpargs.NonDefaultVRFIPv4NHGCount,
		numNHPerNHG:   *fpargs.NonDefaultVRFIPv4NHSize,
		nextHopWeight: generateNextHopWeights(*fpargs.NonDefaultVRFIPv4NHGWeightSum, *fpargs.NonDefaultVRFIPv4NHSize),
		startNHGIndex: nextHopGroupStartIndex + *fpargs.DefaultVRFIPv4Count,
		backupNHG:     1,
		nhDecapEncap:  false,
	}
	vrfEntryParams[vrf2] = &routesParam{
		ipEntries:     createIPv4Entries(IPBlockNonDefaultVRF)[0:*fpargs.NonDefaultVRFIPv4Count],
		numUniqueNHs:  len(decapEncapVirtualIPs),
		nextHops:      decapEncapVirtualIPs,
		nextHopVRF:    deviations.DefaultNetworkInstance(args.dut),
		startNHIndex:  vrfEntryParams[vrf1].startNHIndex + vrfEntryParams[vrf1].numUniqueNHs,
		numUniqueNHGs: *fpargs.NonDefaultVRFIPv4NHGCount,
		numNHPerNHG:   *fpargs.NonDefaultVRFIPv4NHSize,
		nextHopWeight: generateNextHopWeights(*fpargs.NonDefaultVRFIPv4NHGWeightSum, *fpargs.NonDefaultVRFIPv4NHSize),
		startNHGIndex: vrfEntryParams[vrf1].startNHGIndex + vrfEntryParams[vrf1].numUniqueNHGs,
		backupNHG:     2,
		nhDecapEncap:  false,
	}
	vrfEntryParams[vrf3] = &routesParam{
		ipEntries:     createIPv4Entries(IPBlockNonDefaultVRF)[0:*fpargs.NonDefaultVRFIPv4Count],
		numUniqueNHs:  *fpargs.DecapEncapCount,
		nextHops:      createIPv4Entries(IPBlockNonDefaultVRF)[0:*fpargs.DecapEncapCount],
		nextHopVRF:    vrf2,
		startNHIndex:  vrfEntryParams[vrf2].startNHIndex + vrfEntryParams[vrf2].numUniqueNHs,
		numUniqueNHGs: *fpargs.DecapEncapCount,
		numNHPerNHG:   1,
		nextHopWeight: []int{1},
		startNHGIndex: vrfEntryParams[vrf2].startNHGIndex + vrfEntryParams[vrf2].numUniqueNHGs,
		backupNHG:     2,
		nhDecapEncap:  true,
	}

	for _, vrf := range []string{vrf1, vrf2, vrf3} {
		installEntries(t, vrf, vrfEntryParams[vrf], args)
	}
}

// createIPv4Entries creates IPv4 Entries given the totalCount and starting prefix
func createIPv4Entries(startIP string) []string {

	_, netCIDR, _ := net.ParseCIDR(startIP)
	netMask := binary.BigEndian.Uint32(netCIDR.Mask)
	firstIP := binary.BigEndian.Uint32(netCIDR.IP)
	lastIP := (firstIP & netMask) | (netMask ^ 0xffffffff)
	entries := []string{}
	for i := firstIP; i <= lastIP; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		entries = append(entries, fmt.Sprint(ip))
	}
	return entries
}

// installEntries installs IPv4 Entries in the VRF with the given nextHops and nextHopGroups using gRIBI.
func installEntries(t *testing.T, vrf string, routeParams *routesParam, args *testArgs) {

	// Provision next-hops
	nextHopIndices := []uint64{}
	for i := 0; i < routeParams.numUniqueNHs; i++ {
		index := uint64(routeParams.startNHIndex + i)
		if routeParams.nhDecapEncap {
			args.client.Modify().AddEntry(t,
				fluent.NextHopEntry().
					WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
					WithIndex(index).
					WithIPinIP(tunnelSrcIP, routeParams.nextHops[i%len(routeParams.nextHops)]).
					WithDecapsulateHeader(fluent.IPinIP).
					WithEncapsulateHeader(fluent.IPinIP).
					WithNextHopNetworkInstance(routeParams.nextHopVRF).
					WithElectionID(args.electionID.Low, args.electionID.High))
		} else {
			args.client.Modify().AddEntry(t,
				fluent.NextHopEntry().
					WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
					WithIndex(index).
					WithIPAddress(routeParams.nextHops[i%len(routeParams.nextHops)]).
					WithElectionID(args.electionID.Low, args.electionID.High))
		}
		nextHopIndices = append(nextHopIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	// Provision next-hop-groups
	nextHopGroupIndices := []uint64{}
	for i := 0; i < routeParams.numUniqueNHGs; i++ {
		index := uint64(routeParams.startNHGIndex + i)
		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(index).
			WithBackupNHG(uint64(routeParams.backupNHG)).
			WithElectionID(args.electionID.Low, args.electionID.High)
		for j := 0; j < routeParams.numNHPerNHG; j++ {
			nhgEntry.AddNextHop(nextHopIndices[(i*routeParams.numNHPerNHG+j)%len(nextHopIndices)], uint64(routeParams.nextHopWeight[j]))
		}
		args.client.Modify().AddEntry(t, nhgEntry)
		nextHopGroupIndices = append(nextHopGroupIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	// Provision ip entires in VRF
	for i := range routeParams.ipEntries {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(routeParams.ipEntries[i]+"/32").
				WithNetworkInstance(vrf).
				WithNextHopGroup(nextHopGroupIndices[i%len(nextHopGroupIndices)]).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(args.dut)))
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	t.Logf("Installed entries VRF %s - IPv4 entry count: %d, next-hop-group count: %d (index %d - %d), next-hop count: %d (index %d - %d)", vrf, len(routeParams.ipEntries), len(nextHopGroupIndices), nextHopGroupIndices[0], nextHopGroupIndices[len(nextHopGroupIndices)-1], len(nextHopIndices), nextHopIndices[0], nextHopIndices[len(nextHopIndices)-1])
}

// pushDefaultEntries installs gRIBI next-hops, next-hop-groups and IP entries for base route resolution
func pushDefaultEntries(t *testing.T, args *testArgs, nextHops []*nextHopIntfRef) ([]string, []string) {

	primaryNextHopIfs := nextHops[0:*fpargs.DefaultVRFPrimarySubifCount]
	decapEncapNextHopIfs := nextHops[*fpargs.DefaultVRFPrimarySubifCount:]

	primaryNextHopIndices := []uint64{}
	for i := 0; i < (*fpargs.DefaultVRFIPv4NHCount - len(decapEncapNextHopIfs)); i++ {
		index := uint64(nextHopStartIndex + i)
		mac, _ := incrementMAC(StaticMAC, i)
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(index).
				WithSubinterfaceRef(primaryNextHopIfs[i%len(primaryNextHopIfs)].intfName, uint64(primaryNextHopIfs[i%len(primaryNextHopIfs)].subintfIndex)).
				WithMacAddress(mac).
				WithElectionID(args.electionID.Low, args.electionID.High))
		primaryNextHopIndices = append(primaryNextHopIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed %s VRF \"primary\" next-hop count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(primaryNextHopIndices), primaryNextHopIndices[0], primaryNextHopIndices[len(primaryNextHopIndices)-1])

	decapEncapNextHopIndices := []uint64{}
	for i := 0; i < len(decapEncapNextHopIfs); i++ {
		index := primaryNextHopIndices[len(primaryNextHopIndices)-1] + uint64(1+i)
		mac, _ := incrementMAC(StaticMAC, len(primaryNextHopIndices)+i)
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithIndex(index).
				WithSubinterfaceRef(decapEncapNextHopIfs[i%len(decapEncapNextHopIfs)].intfName, uint64(decapEncapNextHopIfs[i%len(decapEncapNextHopIfs)].subintfIndex)).
				WithMacAddress(mac).
				WithElectionID(args.electionID.Low, args.electionID.High))
		decapEncapNextHopIndices = append(decapEncapNextHopIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed %s VRF \"decap/encap\" next-hop count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(decapEncapNextHopIndices), decapEncapNextHopIndices[0], decapEncapNextHopIndices[len(decapEncapNextHopIndices)-1])

	primaryNextHopGroupIndices := []uint64{}
	for i := 0; i < (*fpargs.DefaultVRFIPv4Count - len(decapEncapNextHopIfs)); i++ {
		index := uint64(nextHopGroupStartIndex + i)
		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(index).
			WithElectionID(args.electionID.Low, args.electionID.High)
		for j := 0; j < *fpargs.DefaultVRFIPv4NHSize; j++ {
			nhgEntry.AddNextHop(primaryNextHopIndices[(i**fpargs.DefaultVRFIPv4NHSize+j)%len(primaryNextHopIndices)], uint64(generateNextHopWeights(*fpargs.DefaultVRFIPv4NHGWeightSum, *fpargs.DefaultVRFIPv4NHSize)[j]))
		}
		args.client.Modify().AddEntry(t, nhgEntry)
		primaryNextHopGroupIndices = append(primaryNextHopGroupIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed %s VRF \"primary\" next-hop-group count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(primaryNextHopGroupIndices), primaryNextHopGroupIndices[0], primaryNextHopGroupIndices[len(primaryNextHopGroupIndices)-1])

	decapEncapNextHopGroupIndices := []uint64{}
	for i := 0; i < len(decapEncapNextHopIfs); i++ {
		index := uint64(nextHopGroupStartIndex + len(primaryNextHopGroupIndices) + i)
		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
			WithID(index).
			WithElectionID(args.electionID.Low, args.electionID.High)
		for j := 0; j < *fpargs.DefaultVRFIPv4NHSize; j++ {
			nhgEntry.AddNextHop(decapEncapNextHopIndices[(i**fpargs.DefaultVRFIPv4NHSize+j)%len(decapEncapNextHopIndices)], uint64(generateNextHopWeights(*fpargs.DefaultVRFIPv4NHGWeightSum, *fpargs.DefaultVRFIPv4NHSize)[j]))
		}
		args.client.Modify().AddEntry(t, nhgEntry)
		decapEncapNextHopGroupIndices = append(decapEncapNextHopGroupIndices, index)
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}
	t.Logf("Installed %s VRF \"decap/encap\" next-hop-group count: %d (index %d - %d)", deviations.DefaultNetworkInstance(args.dut), len(decapEncapNextHopGroupIndices), decapEncapNextHopGroupIndices[0], decapEncapNextHopGroupIndices[len(decapEncapNextHopGroupIndices)-1])

	time.Sleep(time.Minute)
	virtualVIPs := createIPv4Entries(IPBlockDefaultVRF)
	primaryVirtualVIPs := virtualVIPs[0:(*fpargs.DefaultVRFIPv4Count - len(decapEncapNextHopIfs))]
	decapEncapVirtualIPs := virtualVIPs[(*fpargs.DefaultVRFIPv4Count - len(decapEncapNextHopIfs)):*fpargs.DefaultVRFIPv4Count]

	// install IPv4 entries for primary forwarding cases (referenced from vrf1)
	for i := range primaryVirtualVIPs {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(primaryVirtualVIPs[i]+"/32").
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithNextHopGroup(primaryNextHopGroupIndices[i%len(primaryNextHopGroupIndices)]).
				WithElectionID(args.electionID.Low, args.electionID.High))
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	for i := range primaryVirtualVIPs {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(primaryVirtualVIPs[i]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
	t.Logf("Installed %s VRF \"primary\" IPv4 entries, %s/32 to %s/32", deviations.DefaultNetworkInstance(args.dut), primaryVirtualVIPs[0], primaryVirtualVIPs[len(primaryVirtualVIPs)-1])

	// install IPv4 entries for decap/encap cases (referenced from vrf2)
	for i := range decapEncapVirtualIPs {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(decapEncapVirtualIPs[i]+"/32").
				WithNetworkInstance(deviations.DefaultNetworkInstance(args.dut)).
				WithNextHopGroup(decapEncapNextHopGroupIndices[i%len(decapEncapNextHopGroupIndices)]).
				WithElectionID(args.electionID.Low, args.electionID.High))
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via client, got err: %v", err)
	}

	for i := range decapEncapVirtualIPs {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(decapEncapVirtualIPs[i]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
	t.Logf("Installed %s VRF \"decap/encap\" IPv4 entries, %s/32 to %s/32", deviations.DefaultNetworkInstance(args.dut), decapEncapVirtualIPs[0], decapEncapVirtualIPs[len(decapEncapVirtualIPs)-1])
	t.Log("Pushed gRIBI default entries")

	return primaryVirtualVIPs, decapEncapVirtualIPs
}

// configureDUT configures DUT interfaces and policy forwarding. Subinterfaces on DUT port2 are configured separately
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := &oc.Root{}

	vrfs := []string{deviations.DefaultNetworkInstance(dut), vrf1, vrf2, vrf3}
	createVrf(t, dut, vrfs)

	// configure Ethernet interfaces first
	configureInterfaceDUT(t, d, dut, dp1, "src")
	configureInterfaceDUT(t, d, dut, dp2, "dst")

	// configure an L3 subinterface without vlan tagging under DUT port#1
	createSubifDUT(t, d, dut, dp1, 0, 0, dutPort1.IPv4, ipv4PrefixLen)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	applyForwardingPolicy(t, dp1.Name())
}

// createVrf takes in a list of VRF names and creates them on the target devices.
func createVrf(t *testing.T, dut *ondatra.DUTDevice, vrfs []string) {
	for _, vrf := range vrfs {
		if vrf != deviations.DefaultNetworkInstance(dut) {
			// configure non-default VRFs
			d := &oc.Root{}
			i := d.GetOrCreateNetworkInstance(vrf)
			i.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), i)
		} else {
			// configure DEFAULT vrf
			dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
			gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

		}
		if deviations.ExplicitGRIBIUnderNetworkInstance(dut) {
			fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrf)
		}
	}
	// configure PBF
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), configurePBF(dut))
}

// configurePBF returns a fully configured network-instance PF struct
func configurePBF(dut *ondatra.DUTDevice) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()
	vrfPolicy := pf.GetOrCreatePolicy(policyName)
	vrfPolicy.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	vrfPolicy.GetOrCreateRule(1).GetOrCreateIpv4().SourceAddress = ygot.String(atePort1.IPv4 + "/32")
	vrfPolicy.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String(vrf1)
	return pf
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, ingressPort string) {
	t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(ingressPort)
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(ingressPort)
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(policyName)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		pfCfg.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, pfPath.Config(), pfCfg)
}

// configureInterfaceDUT configures a single DUT port.
func configureInterfaceDUT(t *testing.T, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, desc string) {
	ifName := dutPort.Name()
	i := d.GetOrCreateInterface(ifName)
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	if deviations.ExplicitPortSpeed(dut) {
		i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, dutPort)
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(ifName).Config(), i)
	t.Logf("DUT port %s configured", dutPort)
}

// createSubifDUT creates a single L3 subinterface
func createSubifDUT(t *testing.T, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, index uint32, vlanID uint16, ipv4Addr string, ipv4SubintfPrefixLen int) {
	ifName := dutPort.Name()
	i := d.GetOrCreateInterface(dutPort.Name())
	s := i.GetOrCreateSubinterface(index)
	if vlanID != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4Addr)
	a.PrefixLength = ygot.Uint8(uint8(ipv4SubintfPrefixLen))
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(ifName).Subinterface(index).Config(), s)
}

// configureDUTSubIfs configures DefaultVRFIPv4NHCount DUT subinterfaces on the target device
func configureDUTSubIfs(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port) []*nextHopIntfRef {
	d := &oc.Root{}
	nextHops := []*nextHopIntfRef{}
	for i := 0; i < *fpargs.DefaultVRFIPv4NHCount; i++ {
		index := uint32(i)
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID = uint16(i) + 1
		}
		dutIPv4 := incrementIP(subifBaseIP, (4*i)+2)
		ateIPv4 := incrementIP(subifBaseIP, (4*i)+1)
		createSubifDUT(t, d, dut, dutPort, index, vlanID, dutIPv4, ipv4PrefixLen)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, dutPort.Name(), deviations.DefaultNetworkInstance(dut), index)
		}
		nextHops = append(nextHops, &nextHopIntfRef{
			nextHopIPAddress: ateIPv4,
			subintfIndex:     index,
			intfName:         dutPort.Name(),
		})
	}
	return nextHops
}

// configureATESubIfs configures ATE subinterfaces on the target device
func configureATESubIfs(t *testing.T, atePort *ondatra.Port, top *ondatra.ATETopology, dut *ondatra.DUTDevice) {

	for i := 0; i < *fpargs.DefaultVRFIPv4NHCount; i++ {
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID = uint16(i) + 1
		}
		dutIPv4 := incrementIP(subifBaseIP, (4*i)+2)
		ateIPv4 := incrementIP(subifBaseIP, (4*i)+1)
		name := fmt.Sprintf(`dst%d`, i)
		configureATEIf(t, top, atePort, name, vlanID, dutIPv4, ateIPv4+"/30")
	}
}

// configureATEIf configures a single ATE layer 3 interface.
func configureATEIf(t *testing.T, top *ondatra.ATETopology, atePort *ondatra.Port, Name string, vlanID uint16, dutIPv4 string, ateIPv4 string) {
	t.Helper()

	i := top.AddInterface(Name).WithPort(atePort)
	if vlanID != 0 {
		i.Ethernet().WithVLANID(vlanID)
	}
	i.IPv4().WithAddress(ateIPv4)
	i.IPv4().WithDefaultGateway(dutIPv4)
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// createFlow creates an individual traffic flow on the ATE
func createFlow(t *testing.T, ate *ondatra.ATEDevice, ateTop *ondatra.ATETopology, ateSrcIf string, ateDstIf string, flowName string) *ondatra.Flow {

	_, netCIDR, _ := net.ParseCIDR(IPBlockNonDefaultVRF)

	ethHeader := ondatra.NewEthernetHeader()
	outerIPv4Header := ondatra.NewIPv4Header()
	outerIPv4Header.WithSrcAddress(atePort1.IPv4)
	outerIPv4Header.DstAddressRange().
		WithMin(netCIDR.IP.String()).
		WithStep("0.0.0.1").
		WithCount(uint32(*fpargs.NonDefaultVRFIPv4Count))
	innerIPv4Header := ondatra.NewIPv4Header()
	innerIPv4Header.WithSrcAddress("10.51.100.1")
	innerIPv4Header.WithDstAddress("198.52.100.1")

	flow := ate.Traffic().NewFlow(flowName).
		WithSrcEndpoints(ateTop.Interfaces()[ateSrcIf]).
		WithDstEndpoints(ateTop.Interfaces()[ateDstIf]).
		WithHeaders(ethHeader, outerIPv4Header, innerIPv4Header)

	return flow
}

// checkInputArgs verifies that gribi scaling input args are set
func checkInputArgs(t *testing.T) error {
	t.Logf("Input arg DefaultVRFIPv4Count           = %d", *fpargs.DefaultVRFIPv4Count)
	t.Logf("Input arg DefaultVRFIPv4NHSize          = %d", *fpargs.DefaultVRFIPv4NHSize)
	t.Logf("Input arg DefaultVRFIPv4NHGWeightSum    = %d", *fpargs.DefaultVRFIPv4NHGWeightSum)
	t.Logf("Input arg DefaultVRFIPv4NHCount         = %d", *fpargs.DefaultVRFIPv4NHCount)
	t.Logf("Input arg NonDefaultVRFIPv4Count        = %d", *fpargs.NonDefaultVRFIPv4Count)
	t.Logf("Input arg NonDefaultVRFIPv4NHGCount     = %d", *fpargs.NonDefaultVRFIPv4NHGCount)
	t.Logf("Input arg NonDefaultVRFIPv4NHSize       = %d", *fpargs.NonDefaultVRFIPv4NHSize)
	t.Logf("Input arg NonDefaultVRFIPv4NHGWeightSum = %d", *fpargs.NonDefaultVRFIPv4NHGWeightSum)
	t.Logf("Input arg DecapEncapCount               = %d", *fpargs.DecapEncapCount)
	t.Logf("Input arg DefaultVRFPrimarySubifCount   = %d", *fpargs.DefaultVRFPrimarySubifCount)

	if *fpargs.DefaultVRFIPv4Count == -1 {
		return errors.New("Input argument DefaultVRFIPv4Count is not set")
	}
	if *fpargs.DefaultVRFIPv4NHSize == -1 {
		return errors.New("Input argument DefaultVRFIPv4NHSize is not set")
	}
	if *fpargs.DefaultVRFIPv4NHGWeightSum == -1 {
		return errors.New("Input argument DefaultVRFIPv4NHGWeightSum is not set")
	}
	if *fpargs.DefaultVRFIPv4NHCount == -1 {
		return errors.New("Input argument DefaultVRFIPv4NHCount is not set")
	}
	if *fpargs.NonDefaultVRFIPv4Count == -1 {
		return errors.New("Input argument NonDefaultVRFIPv4Count is not set")
	}
	if *fpargs.NonDefaultVRFIPv4NHGCount == -1 {
		return errors.New("Input argument NonDefaultVRFIPv4NHGCount is not set")
	}
	if *fpargs.NonDefaultVRFIPv4NHSize == -1 {
		return errors.New("Input argument NonDefaultVRFIPv4NHSize is not set")
	}
	if *fpargs.NonDefaultVRFIPv4NHGWeightSum == -1 {
		return errors.New("Input argument NonDefaultVRFIPv4NHGWeightSum is not set")
	}
	if *fpargs.DecapEncapCount == -1 {
		return errors.New("Input argument DecapEncapCount is not set")
	}
	if *fpargs.DefaultVRFPrimarySubifCount == -1 {
		return errors.New("Input argument DefaultVRFPrimarySubifCount is not set")
	}
	return nil
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx        context.Context
	client     *fluent.GRIBIClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	top        *ondatra.ATETopology
	electionID gribi.Uint128
}

func TestScaling(t *testing.T) {

	if err := checkInputArgs(t); err != nil {
		t.Fatalf("Input arguments not set: %v", err)
	}

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI().Default(t)
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	dp2 := dut.Port(t, "port2")
	top := ate.Topology().New()

	configureDUT(t, dut)
	// configure DefaultVRFIPv4NHCount L3 subinterfaces under DUT port#2 and assign them to DEFAULT vrf
	// return slice containing interface name, subinterface index and ATE next hop IP that will be used for creating gRIBI next-hop entries
	subIntfNextHops := configureDUTSubIfs(t, dut, dp2)

	configureATEIf(t, top, ap1, "src", 0, dutPort1.IPv4, atePort1.IPv4CIDR())
	// configure DefaultVRFIPv4NHCount L3 subinterfaces on ATE port#2 corresponding to subinterfaces on DUT port#2
	configureATESubIfs(t, ap2, top, dut)

	top.Push(t).StartProtocols(t)

	// Connect gRIBI client to DUT referred to as gRIBI - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI as the leader.
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()

	client.Start(ctx, t)
	defer client.Stop(t)

	defer func() {
		// Flush all entries after test.
		if err := gribi.FlushAll(client); err != nil {
			t.Error(err)
		}
	}()

	client.StartSending(ctx, t)
	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	eID := gribi.BecomeLeader(t, client)

	args := &testArgs{
		ctx:        ctx,
		client:     client,
		dut:        dut,
		ate:        ate,
		top:        top,
		electionID: eID,
	}

	// pushDefaultEntries installs gRIBI next-hops, next-hop-groups and IP entries for base route resolution
	// defaultIpv4Entries are ipv4 entries used for deriving nextHops for IPBlockNonDefaultVRF
	defaultIpv4Entries, decapEncapDefaultIpv4Entries := pushDefaultEntries(t, args, subIntfNextHops)

	// pushIPv4Entries builds the recursive scaling topology
	pushIPv4Entries(t, defaultIpv4Entries, decapEncapDefaultIpv4Entries, args)

	// create traffic flows
	trafficFlow := createFlow(t, ate, top, "src", "dst0", "gribi_scaling_test_flow")
	trafficFlow.WithFrameSize(uint32(500)).WithFrameRateFPS(uint64(10000))

	ate.Traffic().Start(t, trafficFlow)
	time.Sleep(30 * time.Second)
	ate.Traffic().Stop(t)

	t.Logf("Flow %s OutPkts : %d", trafficFlow.Name(), gnmi.Get(t, args.ate, gnmi.OC().Flow(trafficFlow.Name()).Counters().OutPkts().State()))
	t.Logf("Flow %s InPkts  : %d", trafficFlow.Name(), gnmi.Get(t, args.ate, gnmi.OC().Flow(trafficFlow.Name()).Counters().InPkts().State()))
	t.Logf("Flow %s LossPct : %.3f %%", trafficFlow.Name(), gnmi.Get(t, args.ate, gnmi.OC().Flow(trafficFlow.Name()).LossPct().State()))

	pktLossPct := gnmi.Get(t, args.ate, gnmi.OC().Flow(trafficFlow.Name()).LossPct().State())
	if pktLossPct > 0 {
		t.Errorf("LossPct for %s, got: %.3f %%, want 0", trafficFlow.Name(), pktLossPct)
	} else {
		t.Logf("LossPct OK for %s, got: %.3f %%, want 0", trafficFlow.Name(), pktLossPct)
	}
}
