/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package local_station_mac_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/testing/protocmp"
)

const (
	plen4 = 30
	plen6 = 126

	noStationMAC = ""
	stationMAC   = "00:ba:ba:ba:ba:ba"
)

var (
	ctx = context.Background()
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		MAC:     "02:1a:c0:00:02:02", // 02:1a+192.0.2.2
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		MAC:     "02:1a:c0:00:02:05", // 02:1a+192.0.2.5
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateDst = attrs.Attributes{
		Name:    "dst",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configInterfaceDUT configures the interface on "me" with static ARP
// of peer.  Note that peermac is used for static ARP, and not
// peer.MAC.
func configInterfaceDUT(dut *ondatra.DUTDevice, i *oc.Interface, me, peer *attrs.Attributes, peermac string) *oc.Interface {
	i.Description = ygot.String(me.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	i.Enabled = ygot.Bool(true)

	if me.MAC != "" {
		e := i.GetOrCreateEthernet()
		e.MacAddress = ygot.String(me.MAC)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(me.IPv4)
	a.PrefixLength = ygot.Uint8(plen4)

	if peermac != noStationMAC {
		n4 := s4.GetOrCreateNeighbor(peer.IPv4)
		n4.LinkLayerAddress = ygot.String(peermac)
	}

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(me.IPv6).PrefixLength = ygot.Uint8(plen6)

	if peermac != noStationMAC {
		n6 := s6.GetOrCreateNeighbor(peer.IPv6)
		n6.LinkLayerAddress = ygot.String(peermac)
	}

	return i
}

// configureDUT configures ipv4/ipv6 and mac addresses on the dut interfaces
func configureDUT(t *testing.T, peermac string) {
	dut := ondatra.DUT(t, "dut")
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(dut, i1, &dutSrc, &ateSrc, noStationMAC))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}

	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(dut, i2, &dutDst, &ateDst, peermac))
}

// configureATE configures ATE ports with ipv4/ipv6 addresses
func configureATE(t *testing.T) (*ondatra.ATEDevice, *ondatra.ATETopology) {
	ate := ondatra.ATE(t, "ate")
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(ateSrc.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(ateSrc.IPv4CIDR()).
		WithDefaultGateway(dutSrc.IPv4)
	i1.IPv6().
		WithAddress(ateSrc.IPv6CIDR()).
		WithDefaultGateway(dutSrc.IPv6)
	i1.Ethernet()

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(ateDst.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(ateDst.IPv4CIDR()).
		WithDefaultGateway(dutDst.IPv4)
	i2.IPv6().
		WithAddress(ateDst.IPv6CIDR()).
		WithDefaultGateway(dutDst.IPv6)

	return ate, top
}

// testFlow sends traffic across ATE ports and verifies continuity.
func testFlow(
	t *testing.T,
	ate *ondatra.ATEDevice,
	top *ondatra.ATETopology,
	headers ...ondatra.Header,
) {
	i1 := top.Interfaces()[ateSrc.Name]
	i2 := top.Interfaces()[ateDst.Name]

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(i1).
		WithDstEndpoints(i2).
		WithHeaders(headers...).
		WithFrameRateFPS(100).
		WithFrameSize(512)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	flowPath := gnmi.OC().Flow(flow.Name())

	if got := gnmi.Get(t, ate, flowPath.LossPct().State()); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	}
}

func TestLocalStationMac(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// First configure DUT without local station mac
	configureDUT(t, noStationMAC)
	ate, top := configureATE(t)
	top.Push(t).StartProtocols(t)

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv6Header := ondatra.NewIPv6Header()

	t.Run("Traffic Without Local Station Mac Configuration", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, top, ethHeader, ipv4Header)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, top, ethHeader, ipv6Header)
		})
	})

	modifyStationMac(ctx, t, dut, updatePath, stationMAC)
	defer modifyStationMac(ctx, t, dut, deletePath, stationMAC)

	t.Run("Traffic With Local Station Mac Configuration", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, top, ethHeader, ipv4Header)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, top, ethHeader, ipv6Header)
		})
	})

	// Change destination mac for flow
	ethHeader.WithDstAddress(stationMAC)

	t.Run("Traffic Destined to Local Station Mac Without Static ARP", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, top, ethHeader, ipv4Header)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, top, ethHeader, ipv6Header)
		})
	})

	// Reconfigure the DUT with local station mac
	configureDUT(t, stationMAC)

	// defer deleting of static arp
	defer gnmi.Delete(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Subinterface(0).Ipv4().Neighbor(ateDst.IPv4).Config())
	defer gnmi.Delete(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Subinterface(0).Ipv6().Neighbor(ateDst.IPv6).Config())

	t.Run("Traffic Destined to Local Station Mac With Static ARP", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, top, ethHeader, ipv4Header)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, top, ethHeader, ipv6Header)
		})
	})
}

func TestStationMacConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	cases := []struct {
		test string
		op   setOperation
		want string
	}{{
		test: "Update Station Local Mac",
		op:   updatePath,
		want: "00:ba:ba:ba:ba:ba",
	},
		{
			test: "Replace Station Local Mac",
			op:   updatePath,
			want: "00:ca:ca:ca:ca:ca",
		},
		{
			test: "Delete Station Local Mac",
			op:   deletePath,
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.test, func(t *testing.T) {
			modifyStationMac(ctx, t, dut, c.op, c.want)
			got := getStationMacConfig(ctx, t, dut, c.test)
			if diff := cmp.Diff(c.want, got, protocmp.Transform()); diff != "" {
				t.Fatalf("Error detected (-want +got):\n%s", diff)
			}

		})
	}

}

// setOperation is an enum representing the different kinds of SetRequest
// operations available.
type setOperation int

const (
	// deletePath represents a SetRequest delete.
	deletePath setOperation = iota
	// replacePath represents a SetRequest replace.
	replacePath
	// updatePath represents a SetRequest update.
	updatePath
)

// modifyStationMac applies the configuration using gnmi SetRequest
func modifyStationMac(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, op setOperation, config string) {
	json_config, _ := json.Marshal(config)
	path := &gnmipb.Path{Origin: "Cisco-IOS-XR-um-local-mac-cfg", Elem: []*gnmipb.PathElem{
		{Name: "hw-module"},
		{Name: "local-mac"},
		{Name: "address"}}}
	val := &gnmipb.TypedValue{Value: &gnmipb.TypedValue_JsonIetfVal{JsonIetfVal: json_config}}
	r := &gnmipb.SetRequest{}

	switch op {
	case updatePath:
		r = &gnmipb.SetRequest{
			Update: []*gnmipb.Update{{Path: path, Val: val}},
		}

	case replacePath:
		r = &gnmipb.SetRequest{
			Replace: []*gnmipb.Update{{Path: path, Val: val}},
		}

	case deletePath:
		r = &gnmipb.SetRequest{
			Delete: []*gnmipb.Path{path},
		}

	}
	_, err := dut.RawAPIs().GNMI().Default(t).Set(ctx, r)
	if err != nil {
		t.Error("There is error when applying the config: ", err)

	}
}

// getStationMacConfig fetches location station mac configuration using gnmi GetRequest
func getStationMacConfig(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, testType string) string {

	r := &gnmipb.GetRequest{
		Path: []*gnmipb.Path{
			{Origin: "Cisco-IOS-XR-um-local-mac-cfg", Elem: []*gnmipb.PathElem{
				{Name: "hw-module"},
				{Name: "local-mac"},
				{Name: "address"}}},
		},
		Type:     gnmipb.GetRequest_CONFIG,
		Encoding: gnmipb.Encoding_JSON_IETF,
	}

	res, err := dut.RawAPIs().GNMI().Default(t).Get(ctx, r)
	if testType == "Delete Station Local Mac" {
		if err == nil {
			t.Fatal("Expected an error when getting configuration: ", err)
		} else {
			return ""
		}

	} else {
		if err != nil {
			t.Fatal("There is error when getting configuration: ", err)

		}
	}
	var address string
	json.Unmarshal(res.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal(), &address)
	return address
}
