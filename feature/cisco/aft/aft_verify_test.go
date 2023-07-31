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

package aft_test

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func getaftnh(t *testing.T, dut *ondatra.DUTDevice, ipv4prefix string, ipv4nwinstance string, nhgnwinstance string) (nh []uint64, nhg uint64) {

	ipv4Entry := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(ipv4nwinstance).Afts().Ipv4Entry(ipv4prefix).State())
	t.Logf("ipv4Entry VALUE : %d", ipv4Entry)
	nhgval := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(nhgnwinstance).Afts().NextHopGroup(ipv4Entry.GetNextHopGroup()).State())

	var nhlist []uint64
	for i := range nhgval.NextHop {
		nexthopval := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(nhgnwinstance).Afts().NextHop(i).State())
		index := nexthopval.GetIndex()
		nhlist = append(nhlist, index)
		addr := nexthopval.GetIpAddress()
		pindex := nexthopval.GetProgrammedIndex()

		t.Logf("NextHop Index VALUE : %d", index)
		t.Logf("NextHop IpAddress VALUE : %s", addr)
		t.Logf("NextHop Programmed Index VALUE : %d", pindex)

	}
	return nhlist, ipv4Entry.GetNextHopGroup()
}
