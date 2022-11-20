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
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"

	gpb "github.com/openconfig/gribi/v1/proto/service"
)

const (
	timeout = time.Minute
)

// Uint128 struct implements a 128 bit unsigned integer required by gRIBI
// election process
type Uint128 struct {
	Low  uint64
	High uint64
}

// Increment increases the Uint128 number by 1
func (i Uint128) Increment() Uint128 {
	newHigh := i.High
	newLow := i.Low + 1
	if newLow < i.Low {
		newHigh++ // Carry to high.
	}
	return Uint128{Low: newLow, High: newHigh}
}

// Decrement decreases the Uint128 number by 1
func (i Uint128) Decrement() Uint128 {
	newHigh := i.High
	newLow := i.Low - 1
	if newLow > i.Low {
		newHigh-- // Carry to high.
	}
	return Uint128{Low: newLow, High: newHigh}
}

// Client provides access to GRIBI APIs of the DUT.
//
// Usage:
//
//	c := &Client{
//	  DUT: ondatra.DUT(t, "dut"),
//	  FIBACK: true,
//	  Persistence: true,
//	}
//	defer c.Close(t)
//	if err := c.Start(t); err != nil {
//	  t.Fatalf("Could not initialize gRIBI: %v", err)
//	}
type Client struct {
	DUT         *ondatra.DUTDevice
	FIBACK      bool
	Persistence bool

	// Unexport fields below.
	fluentC    *fluent.GRIBIClient
	electionID Uint128
}

// Fluent resturns the fluent client that can be used to directly call the gribi fluent APIs
func (c *Client) Fluent(t testing.TB) *fluent.GRIBIClient {
	return c.fluentC
}

// NHGOptions are optional parameters to a GRIBI next-hop-group.
type NHGOptions struct {
	// BackupNHG specifies the backup next-hop-group to be used when all next-hops are unavailable.
	BackupNHG uint64
}

// Start function start establish a client connection with the gribi server.
// By default the client is not the leader and for that function BecomeLeader
// needs to be called.
func (c *Client) Start(t testing.TB) error {
	t.Helper()
	t.Logf("Starting GRIBI connection for dut: %s", c.DUT.Name())
	gribiC := c.DUT.RawAPIs().GRIBI().Default(t)
	c.fluentC = fluent.NewClient()
	c.electionID = Uint128{Low: 1, High: 0}

	conn := c.fluentC.Connection().WithStub(gribiC).WithRedundancyMode(fluent.ElectedPrimaryClient)
	conn.WithInitialElectionID(c.electionID.Low, c.electionID.High)
	if c.Persistence {
		conn.WithPersistence()
	}
	if c.FIBACK {
		conn.WithFIBACK()
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
	return awaitTimeout(ctx, t, c.fluentC, timeout)
}

// LearnElectionID learns the current server election id by sending
// a dummy modify request with election id 1.
func (c *Client) LearnElectionID(t testing.TB) (electionID Uint128) {
	return LearnElectionID(t, c.fluentC)
}

// UpdateElectionID updates the election id of the dut.
// The function fails if the requested election id is less than the server election id.
func (c *Client) UpdateElectionID(t testing.TB, electionID Uint128) {
	UpdateElectionID(t, c.fluentC, electionID)
	c.electionID = electionID
}

// BecomeLeader learns the latest election id and the make the client leader by increasing the election id by one.
func (c *Client) BecomeLeader(t testing.TB) (electionID Uint128) {
	eID := BecomeLeader(t, c.fluentC)
	c.electionID = eID
	return eID
}

// ElectionID returns the current electionID being set for the client.
func (c *Client) ElectionID() Uint128 {
	return c.electionID
}

// AddNHG adds a NextHopGroupEntry with a given index, and a map of next hop entry indices to the weights,
// in a given network instance.
func (c *Client) AddNHG(t testing.TB, nhgIndex uint64, nhWeights map[uint64]uint64, instance string, expectedResult fluent.ProgrammingResult, opts ...*NHGOptions) {
	t.Helper()
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)
	for nhIndex, weight := range nhWeights {
		nhg.AddNextHop(nhIndex, weight)
	}
	for _, opt := range opts {
		if opt != nil && opt.BackupNHG != 0 {
			nhg.WithBackupNHG(opt.BackupNHG)
		}
	}
	c.fluentC.Modify().AddEntry(t, nhg)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NHG: %v", err)
	}
	chk.HasResult(t, c.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// AddNH adds a NextHopEntry with a given index to an address within a given network instance.
func (c *Client) AddNH(t testing.TB, nhIndex uint64, address, instance string, expectedResult fluent.ProgrammingResult) {
	t.Helper()
	c.fluentC.Modify().AddEntry(t,
		fluent.NextHopEntry().
			WithNetworkInstance(instance).
			WithIndex(nhIndex).
			WithIPAddress(address))
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NH: %v", err)
	}
	chk.HasResult(t, c.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// AddIPv4 adds an IPv4Entry mapping a prefix to a given next hop group index within a given network instance.
func (c *Client) AddIPv4(t testing.TB, prefix string, nhgIndex uint64, instance, nhgInstance string, expectedResult fluent.ProgrammingResult) {
	t.Helper()
	ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).
		WithNetworkInstance(instance).
		WithNextHopGroup(nhgIndex)
	if nhgInstance != "" && nhgInstance != instance {
		ipv4Entry.WithNextHopGroupNetworkInstance(nhgInstance)
	}
	c.fluentC.Modify().AddEntry(t, ipv4Entry)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add IPv4: %v", err)
	}
	chk.HasResult(t, c.fluentC.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(prefix).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// DeleteIPv4 deletes an IPv4Entry within a network instance, given the route's prefix
func (c *Client) DeleteIPv4(t testing.TB, prefix string, instance string, expectedResult fluent.ProgrammingResult) {
	t.Helper()
	ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance(instance)
	c.fluentC.Modify().DeleteEntry(t, ipv4Entry)
	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to delete IPv4: %v", err)
	}
	chk.HasResult(t, c.fluentC.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(prefix).
			WithOperationType(constants.Delete).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// FlushAll flushes all the gribi entries
func (c *Client) FlushAll(t testing.TB) {
	if err := FlushAll(c.fluentC); err != nil {
		t.Fatal(err)
	}
}

// Flush flushes gRIBI entries specific to the provided NetworkInstance end electionID
func (c *Client) Flush(t testing.TB, electionID Uint128, networkInstanceName string) {
	_, err := Flush(c.fluentC, electionID, networkInstanceName)
	if err != nil {
		t.Fatal(err)
	}
}

// LearnElectionID learns the current server election id by sending
// a dummy modify request with election id 1.
func LearnElectionID(t testing.TB, c *fluent.GRIBIClient) (electionID Uint128) {
	t.Helper()
	t.Log("Learn GRIBI Election ID from dut.")
	c.Modify().UpdateElectionID(t, 1, 0)
	if err := awaitTimeout(context.Background(), t, c, timeout); err != nil {
		t.Fatalf("Error waiting to update Election ID: %v", err)
	}
	results := c.Results(t)
	eID := results[len(results)-1].CurrentServerElectionID
	return Uint128{Low: eID.Low, High: eID.High}
}

// UpdateElectionID updates the election id of the dut.
func UpdateElectionID(t testing.TB, c *fluent.GRIBIClient, electionID Uint128) {
	t.Helper()
	t.Logf("Setting GRIBI Election ID for dut to low=%d, high=%d", electionID.Low, electionID.High)
	c.Modify().UpdateElectionID(t, electionID.Low, electionID.High)
	if err := awaitTimeout(context.Background(), t, c, timeout); err != nil {
		t.Fatalf("Error waiting to update Election ID: %v", err)
	}
}

// BecomeLeader learns the latest election id and the make the client leader by increasing the election id by one.
func BecomeLeader(t testing.TB, c *fluent.GRIBIClient) (electionID Uint128) {
	t.Helper()
	t.Log("Trying to be a master with increasing the election id by one on dut.")
	eID := LearnElectionID(t, c)

	UpdateElectionID(t, c, eID.Increment())
	return eID.Increment()
}

// Flush flushes gRIBI entries specific to the provided NetworkInstance end electionID.
func Flush(client *fluent.GRIBIClient, electionID Uint128, networkInstanceName string) (*gpb.FlushResponse, error) {
	return client.Flush().
		WithElectionID(electionID.Low, electionID.High).
		WithNetworkInstance(networkInstanceName).
		Send()
}

// FlushAll flushes all the gribi entries.
func FlushAll(c *fluent.GRIBIClient) error {
	_, err := c.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send()
	if err != nil {
		return fmt.Errorf("could not remove all gribi entries, got error: %v", err)
	}
	return nil
}

func awaitTimeout(ctx context.Context, t testing.TB, c *fluent.GRIBIClient, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}
