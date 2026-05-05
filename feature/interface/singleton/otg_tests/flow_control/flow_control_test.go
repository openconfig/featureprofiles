// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flow_control_test

import (
	"fmt"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	plen4 = 30
	plen6 = 126
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		MAC:     "02:1a:c0:00:02:01", // 02:1a+192.0.2.1
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateSrc = attrs.Attributes{
		Name:    "src",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		MAC:     "02:1a:c0:00:02:02", // 02:1a+192.0.2.2
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed utilizes of ate:port1 -> dut:port1
//
//   - Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
//
// Note that the first (.0, .4) and last (.3, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses.  This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//
// Static MAC addresses on the DUT have the form 02:1a:WW:XX:YY:ZZ
// where WW:XX:YY:ZZ are the four octets of the IPv4 in hex.  The 0x02
// means the MAC address is locally administered.

func configureDUTInterface(t *testing.T, dut *ondatra.DUTDevice, a *attrs.Attributes, flowControlMode bool) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")

	duti1 := &oc.Interface{Name: ygot.String(p1.Name())}
	a.ConfigOCInterface(duti1, dut)
	duti1.Description = ygot.String(*duti1.Description)
	s := duti1.GetOrCreateSubinterface(0)
	_ = s.GetOrCreateIpv4()
	_ = s.GetOrCreateIpv6()
	duti1.GetOrCreateEthernet().EnableFlowControl = ygot.Bool(flowControlMode)
	di1 := d.Interface(p1.Name())
	fptest.LogQuery(t, p1.String(), di1.Config(), duti1)

	gnmi.Update(t, dut, di1.Config(), duti1)
}

func verifyFlowControl(t *testing.T, dut *ondatra.DUTDevice, flowControlMode bool) {
	p1 := dut.Port(t, "port1")
	flowControlRx := gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).Ethernet().EnableFlowControl().State())
	if flowControlRx != flowControlMode {
		t.Errorf("Flow control mode got %v, want %v", flowControlRx, flowControlMode)
	}
}

func TestFlowControl(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	config := gosnappi.NewConfig()
	ateSrc.AddToOTG(config, p1, &dutSrc)
	flowControlEnabled := []bool{true, false}
	for _, mode := range flowControlEnabled {
		t.Run(fmt.Sprintf("FlowControl with mode: %v", mode), func(t *testing.T) {
			configureDUTInterface(t, dut, &dutSrc, mode)
			verifyFlowControl(t, dut, mode)
		})
	}
}
