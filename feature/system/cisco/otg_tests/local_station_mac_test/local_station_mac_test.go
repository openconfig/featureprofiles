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
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
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
		MAC:     "02:11:01:00:00:01",
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
		MAC:     "02:12:01:00:00:01",
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

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	ateSrc.AddToOTG(top, p1, &dutSrc)
	ateDst.AddToOTG(top, p2, &dutDst)

	return top
}

func testFlow(
	t *testing.T,
	ate *ondatra.ATEDevice,
	top gosnappi.Config,
	dstMac string,
	ipType string,
) {
	top.Flows().Clear()
	flow := top.Flows().Add().SetName("Flow")
	flow.TxRx().Port().SetTxName("port1").SetRxName("port2")
	flow.Metrics().SetEnable(true)
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(ateSrc.MAC)
	eth.Dst().SetValue(dstMac)
	if ipType == "IPv4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
	}
	if ipType == "IPv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(ateSrc.IPv6)
		v6.Dst().SetValue(ateDst.IPv6)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	ate.OTG().StartTraffic(t)
	time.Sleep(10 * time.Second)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow("Flow").State())
	txPackets := float32(recvMetric.GetCounters().GetOutPkts())
	rxPackets := float32(recvMetric.GetCounters().GetInPkts())
	lostPackets := txPackets - rxPackets
	if txPackets == 0 {
		t.Fatalf("Tx packets should be higher than 0")
	}

	if want, got := float32(0.1), lostPackets*100/txPackets; got > want {
		t.Fatalf("Packet loss percentage for flow %s: got %f, want < %f", flow.Name(), got, want)
	}
}

func TestLocalStationMac(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// First configure DUT without local station mac
	configureDUT(t, noStationMAC)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	llAddress, found := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Interface(ateSrc.Name+".Eth").Ipv4Neighbor(dutSrc.IPv4).LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		return val.IsPresent()
	}).Await(t)
	if !found {
		t.Fatalf("Could not get the LinkLayerAddress %s", llAddress)
	}
	dstMac, _ := llAddress.Val()

	t.Run("Traffic Without Local Station Mac Configuration", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, top, dstMac, "IPv4")
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, top, dstMac, "IPv6")
		})
	})

	modifyStationMac(ctx, t, dut, updatePath, stationMAC)
	defer modifyStationMac(ctx, t, dut, deletePath, stationMAC)

	t.Run("Traffic With Local Station Mac Configuration", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, top, dstMac, "IPv4")
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, top, dstMac, "IPv6")
		})
	})

	t.Run("Traffic Destined to Local Station Mac Without Static ARP", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, top, stationMAC, "IPv4")
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, top, stationMAC, "IPv6")
		})
	})

	// Reconfigure the DUT with local station mac
	configureDUT(t, stationMAC)

	// defer deleting of static arp
	defer gnmi.Delete(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Subinterface(0).Ipv4().Neighbor(ateDst.IPv4).Config())
	defer gnmi.Delete(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Subinterface(0).Ipv6().Neighbor(ateDst.IPv6).Config())

	t.Run("Traffic Destined to Local Station Mac With Static ARP", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, top, stationMAC, "IPv4")
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, top, stationMAC, "IPv6")
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
	_, err := dut.RawAPIs().GNMI(t).Set(ctx, r)
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

	res, err := dut.RawAPIs().GNMI(t).Get(ctx, r)
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
