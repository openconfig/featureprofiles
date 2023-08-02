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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func (c *Client) getOrCreateAft(instance string) *oc.NetworkInstance_Afts {
	if len(c.afts) == 0 {
		c.afts = append(c.afts, map[string]*oc.NetworkInstance_Afts{})
	}

	if _, ok := c.getCurrentAftConfig()[instance]; !ok {
		c.getCurrentAftConfig()[instance] = &oc.NetworkInstance_Afts{}
	}
	return c.getCurrentAftConfig()[instance]
}

func (c *Client) getAft(instance string) *oc.NetworkInstance_Afts {
	return c.getCurrentAftConfig()[instance]
}

func (c *Client) checkNH(t testing.TB, nhIndex uint64, address, instance, nhInstance, interfaceRef string) {
	t.Helper()
	time.Sleep(time.Duration(*ciscoFlags.GRIBINHTimer) * time.Second)
	aftNHs := gnmi.GetAll(t, c.DUT, gnmi.OC().NetworkInstance(instance).Afts().NextHopAny().State())
	found := false
	for _, nh := range aftNHs {
		if nh.GetIpAddress() == address {
			if nh.GetNetworkInstance() != nhInstance {
				t.Fatalf("AFT Check failed for aft/next-hop/state/network-instance got %s, want %s", nh.GetNetworkInstance(), nhInstance)
			}
			if nh.GetProgrammedIndex() != nhIndex {
				// t.Fatalf("AFT Check failed for aft/next-hop/state/Programmingindex got %s, want %s", nh.GetProgrammedIndex(), nhIndex)
			}
			if iref := nh.GetInterfaceRef(); iref != nil {
				if interfaceRef == "" {
					continue
				} else {
					if iref.GetInterface() != interfaceRef {
						t.Fatalf("AFT Check failed for aft/next-hop/interface-ref/state/interface got %s, want %s", iref.GetInterface(), interfaceRef)
					}
				}
			} else {
				if interfaceRef != "" {
					t.Fatalf("AFT Check failed for aft/next-hop/interface-ref got none, want interface ref %s", interfaceRef)
				}
			}
			// if len(opts) > 1 {
			// 	// if nh.GetEncapType() != 1 {
			// 	// 	t.Fatalf("AFT Check failed for aft/next-hop/EncapType got %s, want %s", nh.GetEncapType(), 1)
			// 	// }
			// 	// if ipinip := nh.GetIpInIp(); ipinip != nil {
			// 	// 	if nh.GetSrcIp() != opts[0].Src {
			// 	// 		t.Fatalf("AFT Check failed for aft/next-hop/SourceIP got %s, want %s", ipinip.SrcIp, opts[0].Src)
			// 	// 	}
			// 	// 	if nh.GetDstIp() != opts[0].Dest[0] {
			// 	// 		t.Fatalf("AFT Check failed for aft/next-hop/DestIP got %s, want %s", ipinip.DstIp, opts[0].Dest[0])
			// 	// 	}
			// 	// }
			// }
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("AFT Check failed for aft/next-hop/state/programmed-index got none want %d", nhIndex)
	}
}

func (c *Client) checkNHG(t testing.TB, nhgIndex, bkhgIndex uint64, instance string, nhWeights map[uint64]uint64, opts ...*NHGOptions) {
	t.Helper()
	time.Sleep(time.Duration(*ciscoFlags.GRIBINHGTimer) * time.Second)
	aftNHGs := gnmi.GetAll(t, c.DUT, gnmi.OC().NetworkInstance(instance).Afts().NextHopGroupAny().State())
	found := false
	for _, nhg := range aftNHGs {
		if nhg.GetProgrammedId() == nhgIndex {
			if nhg.GetBackupNextHopGroup() != 0 {
				pid := gnmi.Get(t, c.DUT, gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(nhg.GetBackupNextHopGroup()).State()).GetProgrammedId()
				if pid != bkhgIndex {
					t.Fatalf("AFT Check failed for aft/next-hop-group/state/backup-next-hop-group got %d, want %d", nhg.GetBackupNextHopGroup(), bkhgIndex)
				}
			}
			if len(nhg.NextHop) != 1 {
				for nhIndex, nh := range nhg.NextHop {
					// can be avoided by caching indices in client 'c'
					nhPIndex := gnmi.Get(t, c.DUT, gnmi.OC().NetworkInstance(instance).Afts().NextHop(nhIndex).State()).GetProgrammedIndex()

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
				if len(opts) > 0 {
					for _, opt := range opts {
						if opt != nil && opt.FRR {
							continue
						}
					}
				} else {
					for nhIndex, weight := range nhWeights {
						// extra entry in nhWeights
						t.Fatalf("AFT Check failed for aft/next-hop-group/next-hop got none, want nh:weight %d:%d", nhIndex, weight)
					}
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
	aftIPv4Path := gnmi.OC().NetworkInstance(instance).Afts().Ipv4Entry(prefix)
	aftIPv4, ok := gnmi.Watch(t, c.DUT, aftIPv4Path.State(), 60*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		return present && ipv4Entry.GetPrefix() == prefix
	}).Await(t)
	if !ok {
		t.Fatalf("Could not find address %s in telemetry NH AFT", prefix)
	}
	aftIPv4e, _ := aftIPv4.Val()
	gotNhgInstance := aftIPv4e.GetNextHopGroupNetworkInstance()
	if nhgInstance != "" {
		if gotNhgInstance != nhgInstance {
			t.Fatalf("AFT Check failed for ipv4-entry/state/next-hop-group-network-instance got %s, want %s", gotNhgInstance, nhgInstance)
		}
	}

	gotNhgIndex := aftIPv4e.GetNextHopGroup()
	aftNHG := gnmi.Get(t, c.DUT, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Afts().NextHopGroup(gotNhgIndex).State())
	nhgPID := aftNHG.GetProgrammedId()
	if nhgPID != nhgIndex {
		t.Fatalf("AFT Check failed for ipv4-entry/state/next-hop-group/state/programmed-id got %d, want %d", nhgPID, nhgIndex)
	}
}

// CheckAftNH checks a next-hop against the cached configuration
func (c *Client) CheckAftNH(t testing.TB, instance string, programmedIndex, index uint64) {
	time.Sleep(time.Duration(*ciscoFlags.GRIBIAFTChainCheckWait) * time.Second)
	want := c.getAft(instance).NextHop[programmedIndex]
	got := gnmi.Get(t, c.DUT, gnmi.OC().NetworkInstance(instance).Afts().NextHop(index).State())

	diff := cmp.Diff(want, got,
		cmpopts.IgnoreFields(oc.NetworkInstance_Afts_NextHop{}, []string{
			"Index", "ProgrammedIndex", "InterfaceRef",
		}...))
	if len(diff) > 0 {
		t.Fatalf("AFT Chain Check for aft/next-hop-group/next-hop: %s", diff)
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
}

// CheckAftNHG checks a next-hop-group against the cached configuration
func (c *Client) CheckAftNHG(t testing.TB, instance string, programmedID, id uint64) {
	time.Sleep(time.Duration(*ciscoFlags.GRIBIAFTChainCheckWait) * time.Second)
	want := c.getAft(instance).NextHopGroup[programmedID]
	got := gnmi.Get(t, c.DUT, gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(id).State())

	diff := cmp.Diff(want, got,
		cmpopts.IgnoreFields(oc.NetworkInstance_Afts_NextHopGroup{}, []string{
			"Id", "ProgrammedId", "NextHop", "BackupNextHopGroup",
		}...))
	if len(diff) > 0 {
		t.Fatalf("AFT Chain Check failed for aft/next-hop-group. Diff:\n%s", diff)
	}

	for wantIdx, wantNh := range want.NextHop {
		found := false

		for gotIdx, gotNh := range got.NextHop {
			nh := gnmi.Get(t, c.DUT, gnmi.OC().NetworkInstance(instance).Afts().NextHop(gotIdx).State())
			if *nh.ProgrammedIndex == wantIdx {
				found = true

				// if there is only 1 NH we need to ignore this check as fib doesn't store weight
				if len(got.NextHop) > 1 && *wantNh.Weight != *gotNh.Weight {
					t.Fatalf("AFT Chain Check for aft/next-hop-group/next-hop/state/weight got %d, want %d", *gotNh.Weight, *wantNh.Weight)
				} else {
					break
				}

				c.CheckAftNH(t, instance, wantIdx, gotIdx)
			}
		}
		if !found {
			t.Fatalf("AFT Chain Check failed for aft/next-hop-group/next-hop got none")
		}
	}

	if want.BackupNextHopGroup != nil {
		c.CheckAftNHG(t, instance, *want.BackupNextHopGroup, *got.BackupNextHopGroup)
	}
}

// CheckAftIPv4 checks an ipv4 entry against the cached configuration
func (c *Client) CheckAftIPv4(t testing.TB, instance, prefix string) {
	time.Sleep(time.Duration(*ciscoFlags.GRIBIAFTChainCheckWait) * time.Second)
	want := c.getAft(instance).Ipv4Entry[prefix]
	got := gnmi.Get(t, c.DUT, gnmi.OC().NetworkInstance(instance).Afts().Ipv4Entry(prefix).State())

	diff := cmp.Diff(want, got,
		cmpopts.IgnoreFields(oc.NetworkInstance_Afts_Ipv4Entry{}, []string{
			"NextHopGroup", "NextHopGroupNetworkInstance",
		}...))
	if len(diff) > 0 {
		t.Fatalf("AFT Chain Check failed for aft/ipv4-entry. Diff:\n%s", diff)
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
	afts := make(map[string]*oc.NetworkInstance_Afts, len(c.getCurrentAftConfig()))
	for k, aft := range c.getCurrentAftConfig() {
		if copy, err := ygot.DeepCopy(aft); err == nil {
			afts[k] = copy.(*oc.NetworkInstance_Afts)
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

func (c *Client) getCurrentAftConfig() map[string]*oc.NetworkInstance_Afts {
	return c.afts[len(c.afts)-1]
}

// AftRemoveIPv4 emulates the shutdown of an interface in the cached afts configuration
func (c *Client) AftRemoveIPv4(t testing.TB, instance, prefix string) {
	t.Helper()

	time.Sleep(time.Duration(*ciscoFlags.GRIBIRemoveTimer) * time.Second)

	c.getAft(instance).DeleteIpv4Entry(prefix)
	changed := true

	for changed {
		time.Sleep(time.Duration(*ciscoFlags.GRIBIRemoveTimer) * time.Second)

		changed = false
		for _, aft := range c.getCurrentAftConfig() {
			for nhIdx, nh := range aft.NextHop {
				if nh.IpAddress != nil {
					if strings.HasPrefix(prefix, *nh.IpAddress) {
						aft.DeleteNextHop(nhIdx)
						changed = true
					}
				}
			}

			for _, nhg := range aft.NextHopGroup {
				for _, nh := range nhg.NextHop {
					if _, found := aft.NextHop[*nh.Index]; !found {
						nhg.DeleteNextHop(*nh.Index)
						changed = true
					}
				}
				// Breakout since backup is deleted and there are no primary NHs present
				if len(nhg.NextHop) == 0 && len(aft.NextHopGroup) == 1 {
					changed = false
					break
				}
				// This logic is to update the database entry and delete NHG since there are no NH and BNHG associations left
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

				if _, found := c.getAft(nhgInst).NextHopGroup[*ipv4e.NextHopGroup]; !found {
					c.AftRemoveIPv4(t, nhgInst, *ipv4e.Prefix)
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
