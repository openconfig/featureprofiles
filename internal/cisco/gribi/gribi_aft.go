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
	"math/rand"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

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
		if nh.GetIpAddress() == address {
			// if nh.GetProgrammedIndex() == nhIndex {
			// 	if nh.GetIpAddress() != address {
			// 		t.Fatalf("AFT Check failed for aft/next-hop/state/ip-address got %s, want %s", nh.GetIpAddress(), address)
			// 	}
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
			if nhg.GetBackupNextHopGroup() != 0 {
				pid := c.DUT.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nhg.GetBackupNextHopGroup()).ProgrammedId().Get(t)
				if pid != bkhgIndex {
					t.Fatalf("AFT Check failed for aft/next-hop-group/state/backup-next-hop-group got %d, want %d", nhg.GetBackupNextHopGroup(), bkhgIndex)
				}
			}
			if len(nhg.NextHop) != 1 {
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
	nhgPId := c.DUT.Telemetry().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Afts().NextHopGroup(gotNhgIndex).ProgrammedId().Get(t)
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
				if len(got.NextHop) > 1 && *wantNh.Weight != *gotNh.Weight {
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

	//TODO: use next-hop-group-networkinstance instead of default (*ciscoFlags.DefaultNetworkInstance) and remove it from ignore (CSCwc57921)
	c.CheckAftNHG(t, *ciscoFlags.DefaultNetworkInstance, *want.NextHopGroup, *got.NextHopGroup)
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
