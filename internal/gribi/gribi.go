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
	"net"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/gribi/util"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
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
}

// Fluent resturns the fluent client that can be used to directly call the gribi fluent APIs
func (c *Client) Fluent(t testing.TB) *fluent.GRIBIClient {
	return c.fluentC
}

// Start function start establish a client connection with the gribi server.
// By default the client is not the leader and for that function BecomeLeader
// needs to be called.
func (c *Client) Start(t testing.TB) error {
	t.Helper()
	t.Logf("Starting GRIBI connection for dut: %s", c.DUT.Name())
	gribiC := c.DUT.RawAPIs().GRIBI().Default(t)
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

// learnElectionID learns the current server election id by sending
// a dummy modify request with election id 1.
func (c *Client) learnElectionID(t testing.TB) (low, high uint64) {
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
	low, high := c.learnElectionID(t)
	newLow := low + 1
	if newLow < low {
		high++ // Carry to high.
	}
	c.UpdateElectionID(t, newLow, high)
}

// AddNHG adds a NextHopGroupEntry with a given index, and a map of next hop entry indices to the weights,
// in a given network instance.
func (g *Client) NHG(t testing.TB, nhgIndex uint64, bkhgIndex uint64, nhWeights map[uint64]uint64, instance string, action string, expectedResult fluent.ProgrammingResult) {
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(instance).WithID(nhgIndex)

	// checking if backup provided
	if bkhgIndex != 0 {
		nhg.WithBackupNHG(bkhgIndex)
	}

	if len(nhWeights) != 0 {
		for nhIndex, weight := range nhWeights {
			nhg.AddNextHop(nhIndex, weight)
			if action == "add" {
				g.fluentC.Modify().AddEntry(t, nhg)
			} else if action == "delete" {
				g.fluentC.Modify().DeleteEntry(t, nhg)
			} else if action == "replace" {
				g.fluentC.Modify().ReplaceEntry(t, nhg)
			}
			if err := g.AwaitTimeout(context.Background(), t, timeout); err != nil {
				t.Fatalf("Error waiting to add NHG: %v", err)
			}
		}
	}

	chk.HasResult(t, g.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(nhgIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// AddNH adds a NextHopEntry with a given index to an address within a given network instance.
func (g *Client) NH(t testing.TB, nhIndex uint64, nh_entry, instance string, nh_Instance string, action string, expectedResult fluent.ProgrammingResult) {
	addr := net.ParseIP(nh_entry)
	nh := fluent.NextHopEntry().WithNetworkInstance(instance).WithIndex(nhIndex)
	if "decap" == nh_entry {
		nh.WithDecapsulateHeader(fluent.IPinIP).WithNextHopNetworkInstance(nh_Instance)
	} else if addr != nil {
		nh.WithIPAddress(nh_entry)
	} else {
		nh.WithNextHopNetworkInstance(nh_entry)
	}

	if action == "add" {
		g.fluentC.Modify().AddEntry(t, nh)
	} else if action == "delete" {
		g.fluentC.Modify().DeleteEntry(t, nh)
	} else if action == "replace" {
		g.fluentC.Modify().ReplaceEntry(t, nh)
	}

	if err := g.AwaitTimeout(context.Background(), t, timeout); err != nil {
		t.Fatalf("Error waiting to add NH: %v", err)
	}
	chk.HasResult(t, g.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

// AddIPv4 adds an IPv4Entry mapping a prefix to a given next hop group index within a given network instance.
func (g *Client) IPv4(t testing.TB, prefix string, mask string, nhgIndex uint64, instance, nhgInstance string, action string, scale int, expectedResult fluent.ProgrammingResult) {
	for i := 0; i < scale; i++ {
		ipv4Entry := fluent.IPv4Entry().WithPrefix(util.GetIPPrefix(prefix, i, mask)).
			WithNetworkInstance(instance).
			WithNextHopGroup(nhgIndex)

		if nhgInstance != "" && nhgInstance != instance {
			ipv4Entry.WithNextHopGroupNetworkInstance(nhgInstance)
		}

		if action == "add" {
			g.fluentC.Modify().AddEntry(t, ipv4Entry)
		} else if action == "delete" {
			g.fluentC.Modify().DeleteEntry(t, ipv4Entry)
		} else if action == "replace" {
			g.fluentC.Modify().ReplaceEntry(t, ipv4Entry)
		}

		if err := g.AwaitTimeout(context.Background(), t, timeout); err != nil {
			t.Fatalf("Error waiting to add IPv4: %v", err)
		}

		chk.HasResult(t, g.fluentC.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(util.GetIPPrefix(prefix, i, mask)).
				WithOperationType(constants.Add).
				WithProgrammingResult(expectedResult).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

}
