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

	"github.com/openconfig/featureprofiles/internal/cisco/flags"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	spb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/fluent"
)

// AddIPv4Batch adds a list of IPv4Entries mapping  prefixes to a given next hop group index within a given network instance.
func (c *Client) AddIPv4Batch(t testing.TB, prefixes []string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	resultLenBefore := len(c.fluentC.Results(t))
	ipv4Entries := []fluent.GRIBIEntry{}
	for _, prefix := range prefixes {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(instance).
			WithPrefix(prefix).
			WithNextHopGroup(nhgIndex)
		aftIpv4Entry := c.getOrCreateAft(instance).GetOrCreateIpv4Entry(prefix)
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
	resultLenAfter := len(c.fluentC.Results(t))
	newResultsCount := resultLenAfter - resultLenBefore
	expectResultCount := len(prefixes)
	if check.FIBACK {
		expectResultCount = len(prefixes) * 2
	}
	if newResultsCount != expectResultCount {
		t.Fatalf("Number of responses for programing results is not as expected, want: %d , got: %d ", expectResultCount, newResultsCount)
	}
	if check.FIBACK || check.RIBACK {
		c.checkIPV4ResultWithScale(t, resultLenBefore, resultLenAfter, expecteFailure)
	}
	if check.AFTCheck {
		for _, prefix := range prefixes {
			if instance != *ciscoFlags.DefaultNetworkInstance {
				// setting nhginstance to empty as there is no nhgInstance value set
				c.checkIPv4e(t, prefix, nhgIndex, instance, "")
			} else {
				c.checkIPv4e(t, prefix, nhgIndex, instance, nhgInstance)
			}
		}
	}
}

func (c *Client) checkIPV4ResultWithScale(t testing.TB, startIndex, endIndex int, expectFailure bool) {
	results := c.fluentC.Results(t)
	for i := startIndex; i < endIndex; i = i + 1 {
		if results[i] == nil {
			t.Fatalf("The %d th response in results is nil", i)
		}
		opResult := results[i]
		if expectFailure {
			if opResult.ProgrammingResult != spb.AFTResult_FAILED {
				t.Fatalf("The program result of ipv4 entry %s is not as expected, want:%s got:%v", opResult.Details.IPv4Prefix, "AFTResult_FAILED", opResult.ProgrammingResult)
			}
		} else {
			if !(opResult.ProgrammingResult == spb.AFTResult_FIB_PROGRAMMED || opResult.ProgrammingResult == spb.AFTResult_RIB_PROGRAMMED) {
				t.Fatalf("The program result of ipv4 entry %s is not as expected, want:%s got:%v", opResult.Details.IPv4Prefix, "AFTResult_FIB_PROGRAMMED || AFTResult_RIB_PROGRAMMED", opResult.ProgrammingResult)
			}
		}
		if opResult.Latency >= responseTimeThreshold {
			t.Logf("The response time delay for ipv4 entry %s is %d ms (larger than 10 ms) ", opResult.Details.IPv4Prefix, opResult.Latency/1000000)
		}
	}
}

// ReplaceIPv4Batch replace a list of IPv4Entries mapping  prefixes to a given next hop group index within a given network instance.
func (c *Client) ReplaceIPv4Batch(t testing.TB, prefixes []string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	resultLenBefore := len(c.fluentC.Results(t))
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

	resultLenAfter := len(c.fluentC.Results(t))
	newResultsCount := resultLenAfter - resultLenBefore
	expectResultCount := len(prefixes)
	if check.FIBACK {
		expectResultCount = len(prefixes) * 2
	}
	if newResultsCount != expectResultCount {
		t.Fatalf("Number of responses for programing results is not as expected, want: %d , got: %d ", expectResultCount, newResultsCount)
	}
	if check.FIBACK || check.RIBACK {
		c.checkIPV4ResultWithScale(t, resultLenBefore, resultLenAfter, expecteFailure)
	}

	if check.AFTCheck {
		for _, prefix := range prefixes {
			c.checkIPv4e(t, prefix, nhgIndex, instance, nhgInstance)
		}
	}
}

// DeleteIPv4Batch deletes a list of IPv4Entries mapping  prefixes to a given next hop group index within a given network instance.
func (c *Client) DeleteIPv4Batch(t testing.TB, prefixes []string, nhgIndex uint64, instance, nhgInstance string, expecteFailure bool, check *flags.GRIBICheck) {
	resultLenBefore := len(c.fluentC.Results(t))
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
	resultLenAfter := len(c.fluentC.Results(t))
	newResultsCount := resultLenAfter - resultLenBefore
	expectResultCount := len(prefixes)
	if check.FIBACK {
		expectResultCount = len(prefixes) * 2
	}
	if newResultsCount != expectResultCount {
		t.Fatalf("Number of responses for programing results is not as expected, want: %d , got: %d ", expectResultCount, newResultsCount)
	}
	if check.FIBACK || check.RIBACK {
		c.checkIPV4ResultWithScale(t, resultLenBefore, resultLenAfter, expecteFailure)
	}
}
