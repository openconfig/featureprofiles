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

package ipv4_static_routing

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

var (
	dutPorts = map[string]attrs.Attributes{
		"port1": {
			IPv4:    "198.51.100.12",
			IPv6:    "2001:db8:1::12",
			IPv4Len: 31,
			IPv6Len: 127,
			Desc:    "ATE port 1 to DUT port 1",
		},
		"port2": {
			IPv4:    "198.51.100.22",
			IPv6:    "2001:db8:1::22",
			IPv4Len: 31,
			IPv6Len: 127,
			Desc:    "DUT port 2 to ATE port 2",
		},
		"port3": {
			IPv4:    "198.51.100.32",
			IPv6:    "2001:db8:1::32",
			IPv4Len: 31,
			IPv6Len: 127,
			Desc:    "DUT port 3 to ATE port 3",
		},
		"port4": {
			IPv4:    "198.51.100.42",
			IPv6:    "2001:db8:1::42",
			IPv4Len: 31,
			IPv6Len: 127,
			Desc:    "DUT port 4 to ATE port 4",
		},
	}

	atePorts = map[string]attrs.Attributes{
		"port1": {
			IPv4:    "198.51.100.13",
			IPv6:    "2001:db8:1::13",
			MAC:     "02:1a:c0:00:02:01",
			IPv4Len: 31,
			IPv6Len: 127,
		},
		"port2": {
			IPv4:    "198.51.100.23",
			IPv6:    "2001:db8:1::23",
			MAC:     "02:1a:c0:00:02:02",
			IPv4Len: 31,
			IPv6Len: 127,
		},
		"port3": {
			IPv4:    "198.51.100.33",
			IPv6:    "2001:db8:1::33",
			MAC:     "02:1a:c0:00:02:03",
			IPv4Len: 31,
			IPv6Len: 127,
		},
		"port4": {
			IPv4:    "198.51.100.43",
			IPv6:    "2001:db8:1::43",
			MAC:     "02:1a:c0:00:02:04",
			IPv4Len: 31,
			IPv6Len: 127,
		},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestStaticRouteSingleDestinationPort(t *testing.T) {

	// Configure a DUT
	dut := ondatra.DUT(t, "dut")

	for name, attributes := range dutPorts {
		pn := dut.Port(t, name).Name()
		ifCfg := &telemetry.Interface{
			Name:        ygot.String(pn),
			Description: ygot.String(attributes.Desc),
		}
		ifCfg.GetOrCreateSubinterface(0).GetOrCreateIpv4().Enabled = ygot.Bool(true)
		ifCfg.GetOrCreateSubinterface(0).
			GetOrCreateIpv4().
			GetOrCreateAddress(attributes.IPv4).PrefixLength = ygot.Uint8(attributes.IPv4Len)

		dut.Config().Interface(pn).Update(t, ifCfg)
	}

	//  Configure an ATE

	ate := ondatra.ATE(t, "ate")
	top := ate.OTG().NewConfig(t)

	for name, attributes := range atePorts {
		top.Ports().Add().SetName(name)
		i := top.Devices().Add().SetName(name)
		eth := i.Ethernets().Add().SetName(name + ".Eth").SetPortName(i.Name()).SetMac(attributes.MAC)
		eth.Ipv4Addresses().Add().SetName(name + ".IPv4").SetAddress(attributes.IPv4).SetGateway(dutPorts[name].IPv4).SetPrefix(int32(attributes.IPv4Len))
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// destinations specifies an IP destination and whether the traffic should be
	// lost.
	destinations := map[string]bool{
		"192.0.2.6":     false,
		"1.2.3.4":       true,
		"192.0.2.111":   false,
		"100.100.64.24": true,
	}

	// check that traffic can be forwarded to each of the destination ports
	for dstport := range atePorts {
		if dstport == "port1" {
			continue
		}

		ni := &telemetry.NetworkInstance{}
		ni.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "static").
			GetOrCreateStatic("192.0.2.0/24").
			GetOrCreateNextHop("h").NextHop = telemetry.UnionString(atePorts[dstport].IPv4)
		dut.Config().NetworkInstance("default").Update(t, ni)

		for dstaddr, want := range destinations {
			t.Run(fmt.Sprintf("dstaddr_%s_dstport_%s", dstaddr, dstport), func(t *testing.T) {
				// Reset the flows to remove any previous ones.
				top.Flows().Clear().Items()
				// Configure the flow.
				flow := top.Flows().Add().SetName("Flow")
				flow.TxRx().Device().SetTxNames([]string{"port1.IPv4"}).SetRxNames([]string{dstport + ".IPv4"})
				flow.Metrics().SetEnable(true)

				// Add an Ethernet header with the source address of the ATE.
				e1 := flow.Packet().Add().Ethernet()
				e1.Src().SetValue(atePorts["port1"].MAC)

				endpoint := flow.Packet().Add().Ipv4()
				endpoint.Src().SetValue(atePorts["port1"].IPv4)
				endpoint.Dst().SetValue(dstaddr)
				ate.OTG().PushConfig(t, top)

				ate.OTG().StartTraffic(t)
				time.Sleep(1 * time.Second)
				ate.OTG().StopTraffic(t)

				fp := ate.OTG().Telemetry().Flow(flow.Name()).Get(t)
				fpc := fp.GetCounters()

				outpkts := fpc.GetOutPkts()
				inpkts := fpc.GetInPkts()

				t.Logf("Destination: %s, Port: %s, IPv4 Flow Details", dstaddr, dstport)
				t.Logf("inpkts are %d", inpkts)
				t.Logf("outpkts are %d", outpkts)
				lossPct := float32((outpkts - inpkts) * 100 / outpkts)
				t.Logf("flow loss-pct %f", lossPct)
				if (lossPct > 0) != want {
					t.Fatalf("Destination: %s, got loss percentage: %2f, want loss? %v", dstaddr, lossPct, want)
				}
			})
		}
	}

}
