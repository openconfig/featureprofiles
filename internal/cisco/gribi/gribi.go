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
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

const (
	timeout = time.Minute
)

// Client provides access to GRIBI APIs of the DUT.
//
// Usage:
//
//   c := &Client{
//     DUT: ondatra.DUT(t, "dut"),
//     FibACK: true,
//     Persistence: true,
//   }
//   defer c.Close(t)
//   if err := c.Start(t); err != nil {
//     t.Fatalf("Could not initialize gRIBI: %v", err)
//   }
type Client struct {
	DUT                   *ondatra.DUTDevice
	FibACK                bool
	Persistence           bool
	InitialElectionIDLow  uint64
	InitialElectionIDHigh uint64

	// Unexport fields below.
	fluentC *fluent.GRIBIClient
	afts    []map[string]*telemetry.NetworkInstance_Afts
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
	c.afts = []map[string]*telemetry.NetworkInstance_Afts{
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

//
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
func (c *Client) AddNHG(t testing.TB, nhgIndex uint64, bkhgIndex uint64, nhWeights map[uint64]uint64, instance string, expecteFailure bool, check *flags.GRIBICheck) {
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)
	aftNhg, _ := c.getOrCreateAft(instance).NewNextHopGroup(nhgIndex)
	aftNhg.ProgrammedId = &nhgIndex

	if bkhgIndex != 0 {
		nhg.WithBackupNHG(bkhgIndex)
		aftNhg.BackupNextHopGroup = &bkhgIndex
	}
	for nhIndex, weight := range nhWeights {
		nhg.AddNextHop(nhIndex, weight)
		aftNh, _ := aftNhg.NewNextHop(nhIndex)
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
		c.checkNHG(t, nhgIndex, bkhgIndex, instance, nhWeights)
	}
}

//
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
func (c *Client) AddNH(t testing.TB, nhIndex uint64, address, instance string, nhInstance string, interfaceRef string, expecteFailure bool, check *flags.GRIBICheck) {
	NH := fluent.NextHopEntry().
		WithNetworkInstance(instance).
		WithIndex(nhIndex)

	aftNh, _ := c.getOrCreateAft(instance).NewNextHop(nhIndex)

	if address == "decap" {
		NH = NH.WithDecapsulateHeader(fluent.IPinIP)
		aftNh.DecapsulateHeader = telemetry.AftTypes_EncapsulationHeaderType_IPV4
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
		aftNh.InterfaceRef = &telemetry.NetworkInstance_Afts_NextHop_InterfaceRef{Interface: &interfaceRef}
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
		c.checkNH(t, nhIndex, address, instance, nhInstance, interfaceRef)
	}
}

// AddIPv4 adds an IPv4Entry mapping a prefix to a given next hop group index within a given network instance.
func (c *Client) AddIPv4(t testing.TB, prefix string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).
		WithNetworkInstance(instance).
		WithNextHopGroup(nhgIndex)
	aftIpv4Entry, _ := c.getOrCreateAft(instance).NewIpv4Entry(prefix)
	aftIpv4Entry.NextHopGroup = &nhgIndex

	if nhgInstance != "" && nhgInstance != instance {
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

// AddIPv4Batch adds a list of IPv4Entries mapping  prefixes to a given next hop group index within a given network instance.
func (c *Client) AddIPv4Batch(t testing.TB, prefixes []string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	ipv4Entries := []fluent.GRIBIEntry{}
	for _, prefix := range prefixes {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(instance).
			WithPrefix(prefix).
			WithNextHopGroup(nhgIndex)
		aftIpv4Entry, _ := c.getOrCreateAft(instance).NewIpv4Entry(prefix)
		aftIpv4Entry.NextHopGroup = &nhgIndex
		if nhgInstance != "" && nhgInstance != instance {
			ipv4Entry.WithNextHopGroupNetworkInstance(nhgInstance)
			aftIpv4Entry.NextHopGroupNetworkInstance = &nhgInstance
		}
		ipv4Entries = append(ipv4Entries, ipv4Entry)
	}
	c.fluentC.Modify().AddEntry(t, ipv4Entries...)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add IPv4 entries: %v", err)
	}
	for _, prefix := range prefixes {
		if expecteFailure {
			c.checkIPV4Result(t, fluent.ProgrammingFailed, constants.Add, prefix)
		} else {
			c.checkIPV4Result(t, fluent.InstalledInRIB, constants.Add, prefix)
			if check.FIBACK {
				c.checkIPV4Result(t, fluent.InstalledInFIB, constants.Add, prefix)
			}
		}
	}
	if check.AFTCheck {
		for _, prefix := range prefixes {
			c.checkIPv4e(t, prefix, nhgIndex, instance, nhgInstance)
		}
	}
}

// ReplaceNHG replaces a NextHopGroupEntry with a given index, and a map of next hop entry indices to the weights,
// in a given network instance.
func (c *Client) ReplaceNHG(t testing.TB, nhgIndex uint64, bkhgIndex uint64, nhWeights map[uint64]uint64, instance string, expecteFailure bool, check *flags.GRIBICheck) {
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
		aftNh, _ := aftNhg.NewNextHop(nhIndex)
		aftNh.Weight = &weight
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
		c.checkNHG(t, nhgIndex, bkhgIndex, instance, nhWeights)
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

	if address == "decap" {
		NH = NH.WithDecapsulateHeader(fluent.IPinIP)
		aftNh.DecapsulateHeader = telemetry.AftTypes_EncapsulationHeaderType_IPV4
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
		aftNh.InterfaceRef = &telemetry.NetworkInstance_Afts_NextHop_InterfaceRef{Interface: &interfaceRef}
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

//
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

// ReplaceIPv4Batch replace a list of IPv4Entries mapping  prefixes to a given next hop group index within a given network instance.
func (c *Client) ReplaceIPv4Batch(t testing.TB, prefixes []string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	ipv4Entries := []fluent.GRIBIEntry{}
	for _, prefix := range prefixes {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(instance).
			WithPrefix(prefix).
			WithNextHopGroup(nhgIndex)

		c.getOrCreateAft(instance).DeleteIpv4Entry(prefix)
		aftIpv4Entry, _ := c.getOrCreateAft(instance).NewIpv4Entry(prefix)
		aftIpv4Entry.NextHopGroup = &nhgIndex

		if nhgInstance != "" && nhgInstance != instance {
			ipv4Entry.WithNextHopGroupNetworkInstance(nhgInstance)
			aftIpv4Entry.NextHopGroupNetworkInstance = &nhgInstance
		}
		ipv4Entries = append(ipv4Entries, ipv4Entry)
	}
	c.fluentC.Modify().ReplaceEntry(t, ipv4Entries...)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add IPv4 entries: %v", err)
	}
	for _, prefix := range prefixes {
		if expecteFailure {
			c.checkIPV4Result(t, fluent.ProgrammingFailed, constants.Replace, prefix)
		} else {
			c.checkIPV4Result(t, fluent.InstalledInRIB, constants.Replace, prefix)
			if check.FIBACK {
				c.checkIPV4Result(t, fluent.InstalledInFIB, constants.Replace, prefix)
			}
		}
	}

	if check.AFTCheck {
		for _, prefix := range prefixes {
			c.checkIPv4e(t, prefix, nhgIndex, instance, nhgInstance)
		}
	}
}

// DeleteNHG deletes a NextHopGroupEntry with a given index, and a map of next hop entry indices to the weights,
// in a given network instance.
func (c *Client) DeleteNHG(t testing.TB, nhgIndex uint64, bkhgIndex uint64, nhWeights map[uint64]uint64, instance string, expecteFailure bool, check *flags.GRIBICheck) {
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)
	c.getOrCreateAft(instance).DeleteNextHopGroup(nhgIndex)

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

	if address == "decap" {
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

// DeleteIPv4Batch deletes a list of IPv4Entries mapping  prefixes to a given next hop group index within a given network instance.
func (c *Client) DeleteIPv4Batch(t testing.TB, prefixes []string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	ipv4Entries := []fluent.GRIBIEntry{}
	for _, prefix := range prefixes {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(instance).
			WithPrefix(prefix).
			WithNextHopGroup(nhgIndex).
			WithNextHopGroupNetworkInstance(nhgInstance)
		c.getOrCreateAft(instance).DeleteIpv4Entry(prefix)

		if nhgInstance != "" && nhgInstance != instance {
			ipv4Entry.WithNextHopGroupNetworkInstance(nhgInstance)
		}
		ipv4Entries = append(ipv4Entries, ipv4Entry)
	}
	c.fluentC.Modify().DeleteEntry(t, ipv4Entries...)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add IPv4 entries: %v", err)
	}
	for _, prefix := range prefixes {
		if expecteFailure {
			c.checkIPV4Result(t, fluent.ProgrammingFailed, constants.Delete, prefix)
		} else {
			c.checkIPV4Result(t, fluent.InstalledInRIB, constants.Delete, prefix)
			if check.FIBACK {
				c.checkIPV4Result(t, fluent.InstalledInFIB, constants.Delete, prefix)
			}
		}
	}
}

// FlushServer flushes all the gribi entries
func (c *Client) FlushServer(t testing.TB) {
	t.Logf("Flush Entries in All Network Instances.")
	c.afts = []map[string]*telemetry.NetworkInstance_Afts{
		{},
	}

	if _, err := c.fluentC.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("Could not remove all gribi entries from dut %s, got error: %v", c.DUT.Name(), err)
	}
}

func (c *Client) getOrCreateAft(instance string) *telemetry.NetworkInstance_Afts {
	if len(c.afts) == 0 {
		c.afts = append(c.afts, map[string]*telemetry.NetworkInstance_Afts{})
	}

	if _, ok := c.getCurrentAftConfig()[instance]; !ok {
		c.getCurrentAftConfig()[instance] = &telemetry.NetworkInstance_Afts{}
	}
	return c.getCurrentAftConfig()[instance]
}

func (c *Client) getAft(instance string) *telemetry.NetworkInstance_Afts {
	return c.getCurrentAftConfig()[instance]
}

func (c *Client) checkNH(t testing.TB, nhIndex uint64, address, instance, nhInstance, interfaceRef string) {
	t.Helper()
	aftNHs := c.DUT.Telemetry().NetworkInstance(instance).Afts().NextHopAny().Get(t)
	found := false
	for _, nh := range aftNHs {
		if nh.GetProgrammedIndex() == nhIndex {
			if nh.GetIpAddress() != address {
				t.Fatalf("AFT Check failed for aft/next-hop/state/ip-address got %s, want %s", nh.GetIpAddress(), address)
			}
			if nh.GetNetworkInstance() != nhInstance {
				t.Fatalf("AFT Check failed for aft/next-hop/state/network-instance got %s, want %s", nh.GetNetworkInstance(), nhInstance)
			}
			if iref := nh.GetInterfaceRef(); iref != nil {
				if iref.GetInterface() != interfaceRef {
					t.Fatalf("AFT Check failed for aft/next-hop/interface-ref/state/interface got %s, want %s", iref.GetInterface(), interfaceRef)
				}
			} else if interfaceRef != "" {
				t.Fatalf("AFT Check failed for aft/next-hop/interface-ref got none, want interface ref %s", interfaceRef)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("AFT Check failed for aft/next-hop/state/programmed-index got none want %d", nhIndex)
	}
}

func (c *Client) checkNHG(t testing.TB, nhgIndex, bkhgIndex uint64, instance string, nhWeights map[uint64]uint64) {
	t.Helper()
	aftNHGs := c.DUT.Telemetry().NetworkInstance(instance).Afts().NextHopGroupAny().Get(t)
	found := false
	for _, nhg := range aftNHGs {
		if nhg.GetProgrammedId() == nhgIndex {
			if nhg.GetBackupNextHopGroup() != bkhgIndex {
				t.Fatalf("AFT Check failed for aft/next-hop-group/state/backup-next-hop-group got %d, want %d", nhg.GetBackupNextHopGroup(), bkhgIndex)
			}

			for nhIndex, nh := range nhg.NextHop {
				// can be avoided by caching indices in client 'c'
				nhPIndex := c.DUT.Telemetry().NetworkInstance(instance).Afts().NextHop(nhIndex).ProgrammedIndex().Get(t)

				if weight, ok := nhWeights[nhPIndex]; ok {
					if weight != nh.GetWeight() {
						t.Fatalf("AFT Check failed for aft/next-hop-group/next-hop got nh:weight %d:%d, want nh:weight %d:%d", nhPIndex, nh.GetWeight(), nhPIndex, weight)
					}
					delete(nhWeights, nhPIndex)
				} else {
					// extra entry in NHG
					t.Fatalf("AFT Check failed for aft/next-hop-group/next-hop got nh:weight %d:%d, want none", nhPIndex, nh.GetWeight())
				}
			}

			for nhIndex, weight := range nhWeights {
				// extra entry in nhWeights
				t.Fatalf("AFT Check failed for aft/next-hop-group/next-hop got none, want nh:weight %d:%d", nhIndex, weight)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("AFT Check failed for aft/next-hop-group/state/programmed-id got none, want %d", nhgIndex)
	}
}

func (c *Client) checkIPv4e(t testing.TB, prefix string, nhgIndex uint64, instance, nhgInstance string) {
	t.Helper()
	aftIPv4e := c.DUT.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(prefix).Get(t)
	if aftIPv4e.GetPrefix() != prefix {
		t.Fatalf("AFT Check failed for ipv4-entry/state/prefix got %s, want %s", aftIPv4e.GetPrefix(), prefix)
	}
	gotNhgInstance := aftIPv4e.GetNextHopGroupNetworkInstance()
	if gotNhgInstance != nhgInstance {
		t.Fatalf("AFT Check failed for ipv4-entry/state/next-hop-group-network-instance got %s, want %s", gotNhgInstance, nhgInstance)
	}

	gotNhgIndex := aftIPv4e.GetNextHopGroup()
	nhgPId := c.DUT.Telemetry().NetworkInstance(gotNhgInstance).Afts().NextHopGroup(gotNhgIndex).ProgrammedId().Get(t)
	if nhgPId != nhgIndex {
		t.Fatalf("AFT Check failed for ipv4-entry/state/next-hop-group/state/programmed-id got %d, want %d", nhgPId, nhgIndex)
	}
}

// CheckAftNH checks a next-hop against the cached configuration
func (c *Client) CheckAftNH(t testing.TB, instance string, programmedIndex, index uint64) bool {
	want := c.getAft(instance).NextHop[programmedIndex]
	got := c.DUT.Telemetry().NetworkInstance(instance).Afts().NextHop(index).Get(t)
	if *want.IpAddress != *got.IpAddress {
		return false
	}

	diff := cmp.Diff(want, got,
		cmpopts.IgnoreFields(telemetry.NetworkInstance_Afts_NextHop{}, []string{
			"Index", "ProgrammedIndex", "InterfaceRef",
		}...))
	if len(diff) > 0 {
		t.Logf("AFT Check for aft/next-hop-group/next-hop: %s", diff)
		return false
	}

	if want.IpAddress != nil {
		ni := instance
		if want.NetworkInstance != nil {
			ni = *want.NetworkInstance
		}

		for p := range c.getCurrentAftConfig()[ni].Ipv4Entry {
			if strings.HasPrefix(p, *want.IpAddress) {
				c.CheckAftIPv4(t, ni, p)
			}
		}
	}
	return true
}

// CheckAftNHG checks a next-hop-group against the cached configuration
func (c *Client) CheckAftNHG(t testing.TB, instance string, programmedID, id uint64) {
	want := c.getAft(instance).NextHopGroup[programmedID]
	got := c.DUT.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(id).Get(t)

	diff := cmp.Diff(want, got,
		cmpopts.IgnoreFields(telemetry.NetworkInstance_Afts_NextHopGroup{}, []string{
			"Id", "ProgrammedId", "NextHop", "BackupNextHopGroup",
		}...))
	if len(diff) > 0 {
		t.Errorf("AFT Check failed for aft/next-hop-group. Diff:\n%s", diff)
	}

	for wantIdx, wantNh := range want.NextHop {
		found := false
		//TODO: match based on programmed-index (CSCwc54597)
		for gotIdx, gotNh := range got.NextHop {
			if c.CheckAftNH(t, instance, wantIdx, gotIdx) {
				found = true
			}

			if found {
				// TODO: weight returned is always 0. bug?
				if *wantNh.Weight != *gotNh.Weight {
					t.Logf("AFT Check for aft/next-hop-group/next-hop/state/weight got %d, want %d", *gotNh.Weight, *wantNh.Weight)
					found = false
				} else {
					break
				}
			}
		}
		if !found {
			t.Errorf("AFT Check failed for aft/next-hop-group/next-hop got none")
		}
	}

	if want.BackupNextHopGroup != nil {
		c.CheckAftNHG(t, instance, *want.BackupNextHopGroup, *got.BackupNextHopGroup)
	}
}

// CheckAftIPv4 checks an ipv4 entry against the cached configuration
func (c *Client) CheckAftIPv4(t testing.TB, instance, prefix string) {
	want := c.getAft(instance).Ipv4Entry[prefix]
	got := c.DUT.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(prefix).Get(t)

	diff := cmp.Diff(want, got,
		cmpopts.IgnoreFields(telemetry.NetworkInstance_Afts_Ipv4Entry{}, []string{
			"NextHopGroup", "NextHopGroupNetworkInstance",
		}...))
	if len(diff) > 0 {
		t.Errorf("AFT Check failed for aft/ipv4-entry. Diff:\n%s", diff)
	}

	//TODO: use next-hop-group-networkinstance instead of default and remove it from ignore (CSCwc57921)
	c.CheckAftNHG(t, "default", *want.NextHopGroup, *got.NextHopGroup)
}

// CheckAft checks all afts in all vrfs against the cached configuration
func (c *Client) CheckAft(t testing.TB) {
	t.Helper()
	if len(c.afts) == 0 {
		return
	}

	for instance, want := range c.getCurrentAftConfig() {
		for prefix := range want.Ipv4Entry {
			c.CheckAftIPv4(t, instance, prefix)
		}
	}
}

// AftPushConfig creates and activates a copy of the current afts cached configuration
func (c *Client) AftPushConfig(t testing.TB) {
	t.Helper()
	afts := make(map[string]*telemetry.NetworkInstance_Afts, len(c.getCurrentAftConfig()))
	for k, aft := range c.getCurrentAftConfig() {
		if copy, err := ygot.DeepCopy(aft); err == nil {
			afts[k] = copy.(*telemetry.NetworkInstance_Afts)
		} else {
			t.Fatalf("Error copying aft: %v", err)
		}
	}
	c.afts = append(c.afts, afts)
}

// AftPopConfig discards the current cached afts configuration
func (c *Client) AftPopConfig(t testing.TB) {
	t.Helper()
	if len(c.afts) == 0 {
		t.Fatalf("No active aft config")
	}

	c.afts = c.afts[0 : len(c.afts)-1]
}

func (c *Client) getCurrentAftConfig() map[string]*telemetry.NetworkInstance_Afts {
	return c.afts[len(c.afts)-1]
}

// AftRemoveIPv4 emulates the shutdown of an interface in the cached afts configuration
func (c *Client) AftRemoveIPv4(t testing.TB, instance, prefix string) {
	t.Helper()

	c.getAft(instance).DeleteIpv4Entry(prefix)
	changed := true

	for changed {
		changed = false
		for _, aft := range c.getCurrentAftConfig() {
			for nhIdx, nh := range aft.NextHop {
				//TODO: is this sufficient?
				if strings.HasPrefix(prefix, *nh.IpAddress) {
					aft.DeleteNextHop(nhIdx)
					changed = true
				}
			}

			for _, nhg := range aft.NextHopGroup {
				for _, nh := range nhg.NextHop {
					if _, found := aft.NextHop[*nh.Index]; !found {
						nhg.DeleteNextHop(*nh.Index)
						changed = true
					}
				}
				if len(nhg.NextHop) == 0 && nhg.BackupNextHopGroup == nil {
					aft.DeleteNextHopGroup(*nhg.Id)
					changed = true
				}
			}

			for _, ipv4e := range aft.Ipv4Entry {
				nhgInst := instance
				if ipv4e.NextHopGroupNetworkInstance != nil {
					nhgInst = *ipv4e.NextHopGroupNetworkInstance
				}
				//TODO: bug?
				if nhgInst == "DEFAULT" {
					nhgInst = "default"
				}

				if _, found := c.getAft(nhgInst).NextHopGroup[*ipv4e.NextHopGroup]; !found {
					aft.DeleteIpv4Entry(*ipv4e.Prefix)
					changed = true
				}
			}
		}
	}
}

// AddNHWithIPinIP adds a NextHopEntry with IPinIP
func (c *Client) AddNHWithIPinIP(t testing.TB, nhIndex uint64, address, instance string, nhInstance string, subinterfaceRef string, ipinip bool, expecteFailure bool, check *flags.GRIBICheck) {
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
	if subinterfaceRef != "" {
		NH = NH.WithSubinterfaceRef(subinterfaceRef, 1)
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
		nh := c.DUT.Telemetry().NetworkInstance(instance).Afts().NextHop(nhIndex).Get(t)
		if (*nh.Index != nhIndex) || (*nh.IpAddress != address) {
			t.Fatalf("AFT Check failed for aft/nexthop-entry got ip %s, want ip %s; got index %d , want index %d", *nh.IpAddress, address, *nh.Index, nhIndex)
		}
	}
}

// RandomEntries returns array of a few random value
func (c *Client) RandomEntries(t testing.TB, confidence float64, prefixes []string) []string {
	inResult := make(map[string]bool)
	pick := []string{}
	for i := 0; i < len(prefixes); i++ {
		randIndex := rand.Intn(len(prefixes))
		if _, ok := inResult[prefixes[randIndex]]; !ok {
			inResult[prefixes[randIndex]] = true
			pick = append(pick, prefixes[randIndex])
		}
		if len(pick) == int(float64(len(prefixes))*(confidence/100)) {
			return pick
		}
	}
	return pick
}
