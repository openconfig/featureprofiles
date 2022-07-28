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
)

func getaftnh(t *testing.T, dut *ondatra.DUTDevice, ipv4prefix string, ipv4nwinstance string, nhgnwinstance string) (nh []uint64, nhg uint64) {

	nexthopgroup := dut.Telemetry().NetworkInstance(ipv4nwinstance).Afts().Ipv4Entry(ipv4prefix).NextHopGroup().Get(t)
	t.Logf("NextHopGroup VALUE:..............................: %d", nexthopgroup)
	nhgval := dut.Telemetry().NetworkInstance(nhgnwinstance).Afts().NextHopGroup(nexthopgroup).Get(t)

	var nhlist []uint64
	for i := range nhgval.NextHop {
		nexthopval := dut.Telemetry().NetworkInstance(nhgnwinstance).Afts().NextHop(i).Get(t)
		index := nexthopval.GetIndex()
		nhlist = append(nhlist, index)
		addr := nexthopval.GetIpAddress()
		pindex := nexthopval.GetProgrammedIndex()

		t.Logf("NextHop Index VALUE: ..............................: %d", index)
		t.Logf("NextHop IpAddress VALUE: ..............................: %s", addr)
		t.Logf("NextHop Programmed Index VALUE: ..............................: %d", pindex)

	}
	return nhlist, nexthopgroup
}
