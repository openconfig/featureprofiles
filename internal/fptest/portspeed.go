// Copyright 2022 Nokia, Google LLC
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
//
// This code is a Contribution to OpenConfig Feature Profiles project ("Work")
// made under the Google Software Grant and Corporate Contributor License
// Agreement ("CLA") and governed by the Apache License 2.0. No other rights
// or licenses in or to any of Nokia's intellectual property are granted for
// any other purpose. This code is provided on an "as is" basis without
// any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package fptest

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

type speedEnumInfo struct {
	// Speed value in string format in bits/second.
	speedStr string
	// Speed value in integer format in bits/second.
	speedInt uint64
}

var portSpeed = map[ondatra.Speed]oc.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed1Gb:   oc.IfEthernet_ETHERNET_SPEED_SPEED_1GB,
	ondatra.Speed5Gb:   oc.IfEthernet_ETHERNET_SPEED_SPEED_5GB,
	ondatra.Speed10Gb:  oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
}

var enumToSpeedInfoMap = map[oc.E_IfEthernet_ETHERNET_SPEED]speedEnumInfo{
	oc.IfEthernet_ETHERNET_SPEED_SPEED_10MB:   {"10M", 10_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_100MB:  {"100M", 100_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_1GB:    {"1G", 1_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_2500MB: {"2500M", 2500_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_5GB:    {"5G", 5_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB:   {"10G", 10_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_25GB:   {"25G", 25_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_40GB:   {"40G", 40_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB:   {"50G", 50_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB:  {"100G", 100_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_200GB:  {"200G", 200_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB:  {"400G", 400_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_600GB:  {"600G", 600_000_000_000},
	oc.IfEthernet_ETHERNET_SPEED_SPEED_800GB:  {"800G", 800_000_000_000},
}

// SetPortSpeed sets the DUT config for the interface port-speed according
// to ondatra.Prot.Speed()
func SetPortSpeed(t testing.TB, p *ondatra.Port) {
	speed, ok := portSpeed[p.Speed()]
	if !ok {
		// Port speed is unspecified or unrecognized. Explicit config not performed
		return
	}
	t.Logf("Configuring %v port-speed to %v", p.Name(), speed)
	gnmi.Update(t, p.Device(), gnmi.OC().Interface(p.Name()).Ethernet().PortSpeed().Config(), speed)
	time.Sleep(time.Second * 3)
}

// GetIfSpeed returns an explicit speed of an interface in OC format
func GetIfSpeed(t *testing.T, p *ondatra.Port) oc.E_IfEthernet_ETHERNET_SPEED {
	speed, ok := portSpeed[p.Speed()]
	if !ok {
		t.Logf("Explicit port speed %v was not found in the map", speed)
		return 0
	}
	t.Logf("Configuring interface %v speed %v", p.Name(), speed)
	return speed
}

// EthernetSpeedToUint64 returns the speed in uint64 format.
func EthernetSpeedToUint64(speed oc.E_IfEthernet_ETHERNET_SPEED) uint64 {
	if _, ok := enumToSpeedInfoMap[speed]; !ok {
		return 0
	}
	return enumToSpeedInfoMap[speed].speedInt
}
