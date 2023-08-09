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

// Package gribi provides helper APIs to simplify writing gribi test cases.
// It uses fluent APIs and provides wrapper functions to manage sessions and
// change clients roles easily without keep tracking of the server election id.
// It also packs modify operations with the corresponding verifications to
// prevent code duplications and increase the test code readability.
package gribi

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	timeout = 2 * time.Minute
)

// DECAP constant declaration
const (
	DECAP      = "decap"
	ENCAP      = "encap"
	DecapEncap = "DecapEncap"
)

// Client provides access to GRIBI APIs of the DUT.
//
// Usage:
//
//	c := &Client{
//	  DUT: ondatra.DUT(t, "dut"),
//	  FibACK: true,
//	  Persistence: true,
//	}
//	defer c.Close(t)
//	if err := c.Start(t); err != nil {
//	  t.Fatalf("Could not initialize gRIBI: %v", err)
//	}
type Client struct {
	DUT                   *ondatra.DUTDevice
	FibACK                bool
	Persistence           bool
	InitialElectionIDLow  uint64
	InitialElectionIDHigh uint64

	// Unexport fields below.
	fluentC *fluent.GRIBIClient
	afts    []map[string]*oc.NetworkInstance_Afts
}

const responseTimeThreshold = 10000000 // nanosecond (10 ML)

// NHGOptions are optional parameters to a GRIBI next-hop-group.
type NHGOptions struct {
	// BackupNHG specifies the backup next-hop-group to be used when all next-hops are unavailable.
	FRR bool
}

// NHOptions are optional parameters to a GRIBI next-hop-group.
type NHOptions struct {
	// BackupNHG specifies the backup next-hop-group to be used when all next-hops are unavailable.
	Src     string
	Dest    []string
	VrfName string
}

// Fluent resturns the fluent client that can be used to directly call the gribi fluent APIs
func (c *Client) Fluent(t testing.TB) *fluent.GRIBIClient {
	return c.fluentC
}

// Start function start establish a client connection with the gribi server.
// By default the client is not the leader and for that function BecomeLeader
// needs to be called. The client is not cached.
func (c *Client) Start(t testing.TB) error {
	t.Helper()
	t.Logf("Starting GRIBI connection for dut: %s", c.DUT.Name())
	c.afts = []map[string]*oc.NetworkInstance_Afts{
		{},
	}

	gribiC := c.DUT.RawAPIs().GRIBI().New(t)
	c.fluentC = fluent.NewClient()
	c.fluentC.Connection().WithStub(gribiC)
	if c.Persistence {
		c.fluentC.Connection().WithInitialElectionID(c.InitialElectionIDLow, c.InitialElectionIDHigh).
			WithRedundancyMode(fluent.ElectedPrimaryClient).WithPersistence()
	} else {
		c.fluentC.Connection().WithInitialElectionID(c.InitialElectionIDLow, c.InitialElectionIDHigh).
			WithRedundancyMode(fluent.ElectedPrimaryClient)
	}
	if c.FibACK {
		c.fluentC.Connection().WithFIBACK()
	}
	ctx := context.Background()
	c.fluentC.Start(ctx, t)
	c.fluentC.StartSending(ctx, t)
	err := c.AwaitTimeout(ctx, t, timeout)
	return err
}

// Close function closes the gribi session with the dut by stopping the fluent client.
func (c *Client) Close(t testing.TB) {
	t.Helper()
	t.Logf("Closing GRIBI connection for dut: %s", c.DUT.Name())
	if c.fluentC != nil {
		c.fluentC.Stop(t)
		c.fluentC = nil
	}
}

// AwaitTimeout calls a fluent client Await by adding a timeout to the context.
func (c *Client) AwaitTimeout(ctx context.Context, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.fluentC.Await(subctx, t)
}

// LearnElectionID learns the current server election id by sending
// a dummy modify request with election id 1.
func (c *Client) LearnElectionID(t testing.TB) (low, high uint64) {
	t.Helper()
	t.Logf("Learn GRIBI Election ID from dut: %s", c.DUT.Name())
	c.fluentC.Modify().UpdateElectionID(t, 1, 0)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to update Election ID: %v", err)
	}
	results := c.fluentC.Results(t)
	electionID := results[len(results)-1].CurrentServerElectionID
	return electionID.Low, electionID.High
}

// UpdateElectionID updates the election id of the dut.
// The function fails if the requsted election id is less than the server election id.
func (c *Client) UpdateElectionID(t testing.TB, lowElecID, highElecID uint64) {
	t.Helper()
	t.Logf("Setting GRIBI Election ID for dut: %s to low=%d, high=%d", c.DUT.Name(), lowElecID, highElecID)
	c.fluentC.Modify().UpdateElectionID(t, lowElecID, highElecID)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to update Election ID: %v", err)
	}
	chk.HasResult(t, c.fluentC.Results(t),
		fluent.OperationResult().
			WithCurrentServerElectionID(lowElecID, highElecID).
			AsResult(),
	)
}

// BecomeLeader learns the latest election id and the make the client leader by increasing the election id by one.
func (c *Client) BecomeLeader(t testing.TB) {
	t.Logf("Trying to be a master with increasing the election id by one on dut: %s", c.DUT.Name())
	low, high := c.LearnElectionID(t)
	newLow := low + 1
	if newLow < low {
		high++ // Carry to high.
	}
	c.UpdateElectionID(t, newLow, high)
}

func (c *Client) checkNHGResult(t testing.TB, expectedResult fluent.ProgrammingResult, operation constants.OpType, nhgIndex uint64) {
	chk.HasResult(t, c.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndex).
			WithOperationType(operation).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// AddNHG adds a NextHopGroupEntry with a given index, and a map of next hop entry indices to the weights,
// in a given network instance.
func (c *Client) AddNHG(t testing.TB, nhgIndex uint64, bkhgIndex uint64, nhWeights map[uint64]uint64, instance string, expecteFailure bool, check *flags.GRIBICheck, opts ...*NHGOptions) {
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)
	aftNhg := c.getOrCreateAft(instance).GetOrCreateNextHopGroup(nhgIndex)
	aftNhg.ProgrammedId = &nhgIndex

	if bkhgIndex != 0 {
		nhg.WithBackupNHG(bkhgIndex)
		aftNhg.BackupNextHopGroup = &bkhgIndex
	}
	for nhIndex, weight := range nhWeights {
		nhg.AddNextHop(nhIndex, weight)
		if len(opts) > 0 {
			if opts[0].FRR {
				break
			}
		}
		aftNh := aftNhg.GetOrCreateNextHop(nhIndex)
		aftNh.Weight = new(uint64)
		*aftNh.Weight = weight
	}
	c.fluentC.Modify().AddEntry(t, nhg)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NHG: %v", err)
	}
	if expecteFailure {
		c.checkNHGResult(t, fluent.ProgrammingFailed, constants.Add, nhgIndex)
	} else {
		c.checkNHGResult(t, fluent.InstalledInRIB, constants.Add, nhgIndex)
		if check.FIBACK {
			c.checkNHGResult(t, fluent.InstalledInFIB, constants.Add, nhgIndex)
		}
	}

	if check.AFTCheck {
		c.checkNHG(t, nhgIndex, bkhgIndex, instance, nhWeights, opts...)
	}
}

func (c *Client) checkNHResult(t testing.TB, expectedResult fluent.ProgrammingResult, operation constants.OpType, nhIndex uint64) {
	chk.HasResult(t, c.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(operation).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// AddNH adds a NextHopEntry with a given index to an address within a given network instance.
func (c *Client) AddNH(t testing.TB, nhIndex uint64, address, instance string, nhInstance string, interfaceRef string, expecteFailure bool, check *flags.GRIBICheck, opts ...*NHOptions) {
	NH := fluent.NextHopEntry().
		WithNetworkInstance(instance).
		WithIndex(nhIndex)

	aftNh := c.getOrCreateAft(instance).GetOrCreateNextHop(nhIndex)

	//DecapEncap case need to pass source and destination address as optimal parameter
	if address == DECAP {
		NH = NH.WithDecapsulateHeader(fluent.IPinIP)
		aftNh.DecapsulateHeader = oc.Aft_EncapsulationHeaderType_IPV4
		tempAddress := "0.0.0.0"
		aftNh.IpAddress = &tempAddress
	} else if address == ENCAP {
		NH = NH.WithEncapsulateHeader(fluent.IPinIP)
	} else if address == DecapEncap {
		NH = NH.WithDecapsulateHeader(fluent.IPinIP)
		NH = NH.WithEncapsulateHeader(fluent.IPinIP)
		for _, opt := range opts {
			for _, dst := range opt.Dest {
				NH = NH.WithIPinIP(opt.Src, dst)
			}
		}
	} else if address == "DecapEncapvrf" {
		NH = NH.WithDecapsulateHeader(fluent.IPinIP)
		NH = NH.WithEncapsulateHeader(fluent.IPinIP)
		for _, opt := range opts {
			for _, dst := range opt.Dest {
				NH = NH.WithIPinIP(opt.Src, dst)
				NH = NH.WithNextHopNetworkInstance(opt.VrfName)
			}
		}
	} else if address != "" {
		NH = NH.WithIPAddress(address)
		aftNh.IpAddress = &address
	}

	if nhInstance != "" {
		NH = NH.WithNextHopNetworkInstance(nhInstance)
		aftNh.NetworkInstance = &nhInstance
	}
	if interfaceRef != "" {
		NH = NH.WithInterfaceRef(interfaceRef)
		aftNh.InterfaceRef = &oc.NetworkInstance_Afts_NextHop_InterfaceRef{Interface: &interfaceRef}
	}
	c.fluentC.Modify().AddEntry(t, NH)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NH: %v", err)
	}
	if expecteFailure {
		c.checkNHResult(t, fluent.ProgrammingFailed, constants.Add, nhIndex)
	} else {
		c.checkNHResult(t, fluent.InstalledInRIB, constants.Add, nhIndex)
		if check.FIBACK {
			c.checkNHResult(t, fluent.InstalledInFIB, constants.Add, nhIndex)
		}
	}

	if check.AFTCheck {
		//if address is "decap", prefix will be 0.0.0.0, nhInstance is "", and InterfaceRef is Null0
		if address == DECAP || address == ENCAP || address == DecapEncap || address == "DecapEncapvrf" {
			c.checkNH(t, nhIndex, "0.0.0.0", instance, "", "Null0")
		} else {
			if address != "" {
				c.checkNH(t, nhIndex, address, instance, nhInstance, interfaceRef)
			}
		}
	}
}

// AddIPv4 adds an IPv4Entry mapping a prefix to a given next hop group index within a given network instance.
func (c *Client) AddIPv4(t testing.TB, prefix string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).
		WithNetworkInstance(instance).
		WithNextHopGroup(nhgIndex)
	aftIpv4Entry := c.getOrCreateAft(instance).GetOrCreateIpv4Entry(prefix)
	aftIpv4Entry.OriginProtocol = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI
	aftIpv4Entry.NextHopGroup = &nhgIndex

	if nhgInstance != "" {
		ipv4Entry.WithNextHopGroupNetworkInstance(nhgInstance)
		aftIpv4Entry.NextHopGroupNetworkInstance = &nhgInstance
	}
	c.fluentC.Modify().AddEntry(t, ipv4Entry)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add IPv4: %v", err)
	}
	if expecteFailure {
		c.checkIPV4Result(t, fluent.ProgrammingFailed, constants.Add, prefix)
	} else {
		c.checkIPV4Result(t, fluent.InstalledInRIB, constants.Add, prefix)
		if check.FIBACK {
			c.checkIPV4Result(t, fluent.InstalledInFIB, constants.Add, prefix)
		}
	}
	if check.AFTCheck {
		c.checkIPv4e(t, prefix, nhgIndex, instance, nhgInstance)
	}
}

// ReplaceNHG replaces a NextHopGroupEntry with a given index, and a map of next hop entry indices to the weights,
// in a given network instance.
func (c *Client) ReplaceNHG(t testing.TB, nhgIndex uint64, bkhgIndex uint64, nhWeights map[uint64]uint64, instance string, expecteFailure bool, check *flags.GRIBICheck, opts ...*NHGOptions) {
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)
	c.getOrCreateAft(instance).DeleteNextHopGroup(nhgIndex)
	aftNhg, _ := c.getOrCreateAft(instance).NewNextHopGroup(nhgIndex)
	aftNhg.ProgrammedId = &nhgIndex
	if bkhgIndex != 0 {
		nhg.WithBackupNHG(bkhgIndex)
		aftNhg.BackupNextHopGroup = &bkhgIndex
	}
	for nhIndex, weight := range nhWeights {
		nhg.AddNextHop(nhIndex, weight)
		if len(opts) > 0 {
			if opts[0].FRR {
				break
			}
		}
		aftNh := aftNhg.GetOrCreateNextHop(nhIndex)
		aftNh.Weight = new(uint64)
		*aftNh.Weight = weight
	}
	c.fluentC.Modify().ReplaceEntry(t, nhg)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NHG: %v", err)
	}
	if expecteFailure {
		c.checkNHGResult(t, fluent.ProgrammingFailed, constants.Replace, nhgIndex)
	} else {
		c.checkNHGResult(t, fluent.InstalledInRIB, constants.Replace, nhgIndex)
		if check.FIBACK {
			c.checkNHGResult(t, fluent.InstalledInFIB, constants.Replace, nhgIndex)
		}
	}

	if check.AFTCheck {
		c.checkNHG(t, nhgIndex, bkhgIndex, instance, nhWeights, opts...)
	}
}

// ReplaceNH replaces a NextHopEntry with a given index to an address within a given network instance.
func (c *Client) ReplaceNH(t testing.TB, nhIndex uint64, address, instance string, nhInstance string, interfaceRef string, expecteFailure bool, check *flags.GRIBICheck) {
	NH := fluent.NextHopEntry().
		WithNetworkInstance(instance).
		WithIndex(nhIndex)
	c.getOrCreateAft(instance).DeleteNextHop(nhIndex)
	aftNh, _ := c.getOrCreateAft(instance).NewNextHop(nhIndex)
	aftNh.ProgrammedIndex = &nhIndex

	if address == DECAP {
		NH = NH.WithDecapsulateHeader(fluent.IPinIP)
		aftNh.DecapsulateHeader = oc.Aft_EncapsulationHeaderType_IPV4
	} else if address != "" {
		NH = NH.WithIPAddress(address)
		aftNh.IpAddress = &address
	}
	if nhInstance != "" {
		NH = NH.WithNextHopNetworkInstance(nhInstance)
		aftNh.NetworkInstance = &nhInstance
	}
	if interfaceRef != "" {
		NH = NH.WithInterfaceRef(interfaceRef)
		aftNh.InterfaceRef = &oc.NetworkInstance_Afts_NextHop_InterfaceRef{Interface: &interfaceRef}
	}
	c.fluentC.Modify().ReplaceEntry(t, NH)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NH: %v", err)
	}
	if expecteFailure {
		c.checkNHResult(t, fluent.ProgrammingFailed, constants.Replace, nhIndex)
	} else {
		c.checkNHResult(t, fluent.InstalledInRIB, constants.Replace, nhIndex)
		if check.FIBACK {
			c.checkNHResult(t, fluent.InstalledInFIB, constants.Replace, nhIndex)
		}
	}

	if check.AFTCheck {
		c.checkNH(t, nhIndex, address, instance, nhInstance, interfaceRef)
	}
}

func (c *Client) checkIPV4Result(t testing.TB, expectedResult fluent.ProgrammingResult, operation constants.OpType, prefix string) {
	chk.HasResult(t, c.fluentC.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(prefix).
			WithOperationType(operation).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// ReplaceIPv4 replace an IPv4Entry mapping a prefix to a given next hop group index within a given network instance.
func (c *Client) ReplaceIPv4(t testing.TB, prefix string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).
		WithNetworkInstance(instance).
		WithNextHopGroup(nhgIndex)

	c.getOrCreateAft(instance).DeleteIpv4Entry(prefix)
	aftIpv4Entry, _ := c.getOrCreateAft(instance).NewIpv4Entry(prefix)
	aftIpv4Entry.NextHopGroup = &nhgIndex

	if nhgInstance != "" && nhgInstance != instance {
		ipv4Entry.WithNextHopGroupNetworkInstance(nhgInstance)
		aftIpv4Entry.NextHopGroupNetworkInstance = &nhgInstance
	}
	c.fluentC.Modify().ReplaceEntry(t, ipv4Entry)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add IPv4: %v", err)
	}

	if expecteFailure {
		c.checkIPV4Result(t, fluent.ProgrammingFailed, constants.Replace, prefix)
	} else {
		c.checkIPV4Result(t, fluent.InstalledInRIB, constants.Replace, prefix)
		if check.FIBACK {
			c.checkIPV4Result(t, fluent.InstalledInFIB, constants.Replace, prefix)
		}
	}

	if check.AFTCheck {
		c.checkIPv4e(t, prefix, nhgIndex, instance, nhgInstance)
	}
}

// DeleteNHG deletes a NextHopGroupEntry with a given index, and a map of next hop entry indices to the weights,
// in a given network instance.
func (c *Client) DeleteNHG(t testing.TB, nhgIndex uint64, bkhgIndex uint64, nhWeights map[uint64]uint64, instance string, expecteFailure bool, check *flags.GRIBICheck) {
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)
	c.getOrCreateAft(instance).DeleteNextHopGroup(nhgIndex)
	c.getAft(instance).DeleteNextHopGroup(nhgIndex)

	if bkhgIndex != 0 {
		nhg.WithBackupNHG(bkhgIndex)
	}
	if len(nhWeights) != 0 {
		for nhIndex, weight := range nhWeights {
			nhg.AddNextHop(nhIndex, weight)
		}
	}
	c.fluentC.Modify().DeleteEntry(t, nhg)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NHG: %v", err)
	}
	if expecteFailure {
		c.checkNHGResult(t, fluent.ProgrammingFailed, constants.Delete, nhgIndex)
	} else {
		c.checkNHGResult(t, fluent.InstalledInRIB, constants.Delete, nhgIndex)
		if check.FIBACK {
			c.checkNHGResult(t, fluent.InstalledInFIB, constants.Delete, nhgIndex)
		}
	}
}

// DeleteNH delete a NextHopEntry with a given index to an address within a given network instance.
func (c *Client) DeleteNH(t testing.TB, nhIndex uint64, address, instance string, nhInstance string, interfaceRef string, expecteFailure bool, check *flags.GRIBICheck) {
	NH := fluent.NextHopEntry().
		WithNetworkInstance(instance).
		WithIndex(nhIndex)
	c.getOrCreateAft(instance).DeleteNextHop(nhIndex)
	c.getAft(instance).DeleteNextHop(nhIndex)

	if address == DECAP {
		NH = NH.WithDecapsulateHeader(fluent.IPinIP)
	} else if address != "" {
		NH = NH.WithIPAddress(address)
	}
	if nhInstance != "" {
		NH = NH.WithNextHopNetworkInstance(nhInstance)
	}
	if interfaceRef != "" {
		NH = NH.WithInterfaceRef(interfaceRef)
	}
	c.fluentC.Modify().DeleteEntry(t, NH)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NH: %v", err)
	}
	if expecteFailure {
		c.checkNHResult(t, fluent.ProgrammingFailed, constants.Delete, nhIndex)
	} else {
		c.checkNHResult(t, fluent.InstalledInRIB, constants.Delete, nhIndex)
		if check.FIBACK {
			c.checkNHResult(t, fluent.InstalledInFIB, constants.Delete, nhIndex)
		}
	}
}

// DeleteIPv4 deletes an IPv4Entry mapping a prefix to a given next hop group index within a given network instance.
func (c *Client) DeleteIPv4(t testing.TB, prefix string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).
		WithNetworkInstance(instance).
		WithNextHopGroup(nhgIndex)
	c.getOrCreateAft(instance).DeleteIpv4Entry(prefix)
	c.getAft(instance).DeleteIpv4Entry(prefix)

	if nhgInstance != "" && nhgInstance != instance {
		ipv4Entry.WithNextHopGroupNetworkInstance(nhgInstance)
	}
	c.fluentC.Modify().DeleteEntry(t, ipv4Entry)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add IPv4: %v", err)
	}
	if expecteFailure {
		c.checkIPV4Result(t, fluent.ProgrammingFailed, constants.Delete, prefix)
	} else {
		c.checkIPV4Result(t, fluent.InstalledInRIB, constants.Delete, prefix)
		if check.FIBACK {
			c.checkIPV4Result(t, fluent.InstalledInFIB, constants.Delete, prefix)
		}
	}
}

// FlushServer flushes all the gribi entries
func (c *Client) FlushServer(t testing.TB) {
	t.Logf("Flush Entries in All Network Instances.")
	c.afts = []map[string]*oc.NetworkInstance_Afts{
		{},
	}

	if _, err := c.fluentC.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("Could not remove all gribi entries from dut %s, got error: %v", c.DUT.Name(), err)
	}
}

// AddNHWithIPinIP adds a NextHopEntry with IPinIP
func (c *Client) AddNHWithIPinIP(t testing.TB, nhIndex uint64, address, instance string, nhInstance string, interfaceRef string, subinterface uint64, ipinip bool, expecteFailure bool, check *flags.GRIBICheck) {
	NH := fluent.NextHopEntry().
		WithNetworkInstance(instance).
		WithIndex(nhIndex)

	if address == "encap" {
		NH = NH.WithEncapsulateHeader(fluent.IPinIP)
	} else if address != "" {
		if ipinip {
			NH = NH.WithIPAddress(address).WithEncapsulateHeader(fluent.IPinIP).WithIPinIP("20.20.20.1", "10.10.10.1")
		} else {
			NH = NH.WithIPAddress(address)
		}
	}
	if nhInstance != "" {
		NH = NH.WithNextHopNetworkInstance(nhInstance)
	}
	if interfaceRef != "" {
		if subinterface != 0 {
			NH = NH.WithSubinterfaceRef(interfaceRef, subinterface)
		} else {
			NH = NH.WithInterfaceRef(interfaceRef)
		}
	}
	c.fluentC.Modify().AddEntry(t, NH)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NH: %v", err)
	}
	if expecteFailure {
		c.checkNHResult(t, fluent.ProgrammingFailed, constants.Add, nhIndex)
	} else {
		c.checkNHResult(t, fluent.InstalledInRIB, constants.Add, nhIndex)
		if check.FIBACK {
			c.checkNHResult(t, fluent.InstalledInFIB, constants.Add, nhIndex)
		}
	}
	if check.AFTCheck {
		nh := gnmi.Get(t, c.DUT, gnmi.OC().NetworkInstance(instance).Afts().NextHop(nhIndex).State())
		if (*nh.Index != nhIndex) || (*nh.IpAddress != address) {
			t.Fatalf("AFT Check failed for aft/nexthop-entry got ip %s, want ip %s; got index %d , want index %d", *nh.IpAddress, address, *nh.Index, nhIndex)
		}
	}
}
