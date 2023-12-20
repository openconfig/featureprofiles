// Copyright 2023 Google LLC
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

package ipv6_link_local_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	srcDUTGlobalIPv6      = "2001:db8::1"
	srcOTGGlobalIPv6      = "2001:db8::2"
	dstDUTGlobalIPv6      = "2001:db8::5"
	dstOTGGlobalIPv6      = "2001:db8::6"
	globalIPv6Len         = 126
	linkLocalFlowName     = "SrcToDstLinkLocal"
	globalUnicastFlowName = "SrcToDstGlobalUnicast"
)

// We use the same link local IPv6 fe80::1 and fe80::2 on both src and dst pairs to ensure
// that devices will allow link local addresses to be reused across different L2 domains.
var (
	dutSrc = attrs.Attributes{
		Desc:    "dutsrc",
		IPv6:    "fe80::1",
		IPv6Len: 64,
	}

	ateSrc = attrs.Attributes{
		Name:    "atesrc",
		MAC:     "02:11:01:00:00:01",
		IPv6:    "fe80::2",
		IPv6Len: 64,
	}

	dutDst = attrs.Attributes{
		Desc:    "dutdst",
		IPv6:    "fe80::1",
		IPv6Len: 64,
	}

	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:12:01:00:00:01",
		IPv6:    "fe80::2",
		IPv6Len: 64,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIPv6LinkLocal(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUTLinkLocalInterface(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	configureOTGInterface(t, ate, top)
	otgSrcToDstFlow(t, top, ateSrc.IPv6, ateDst.IPv6, linkLocalFlowName)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	t.Run("Interface Telemetry", func(t *testing.T) {
		verifyInterfaceTelemetry(t, dut)
	})

	t.Run("Link Local Traffic Test", func(t *testing.T) {
		verifyLinkLocalTraffic(t, dut, ate, top, linkLocalFlowName)
	})

	t.Run("Configure And Delete Global Unicast IPv6", func(t *testing.T) {
		configureDUTGlobalIPv6(t, dut)
		otgSrcToDstFlow(t, top, srcOTGGlobalIPv6, dstOTGGlobalIPv6, globalUnicastFlowName)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

		t.Run("Global IPv6 Traffic Test", func(t *testing.T) {
			verifyGlobalUnicastTraffic(t, dut, ate, top, globalUnicastFlowName)
		})

		p1 := dut.Port(t, "port1")
		gnmi.Delete(t, dut, gnmi.OC().Interface(p1.Name()).Subinterface(0).Ipv6().Address(srcDUTGlobalIPv6).Config())
		p2 := dut.Port(t, "port2")
		gnmi.Delete(t, dut, gnmi.OC().Interface(p2.Name()).Subinterface(0).Ipv6().Address(dstDUTGlobalIPv6).Config())

		t.Run("Interface Telemetry", func(t *testing.T) {
			verifyInterfaceTelemetry(t, dut)
		})

		otgSrcToDstFlow(t, top, ateSrc.IPv6, ateDst.IPv6, linkLocalFlowName)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
		t.Run("Traffic Test", func(t *testing.T) {
			verifyLinkLocalTraffic(t, dut, ate, top, linkLocalFlowName)
		})
	})

	t.Run("Disable and Enable Port1", func(t *testing.T) {
		p1 := dut.Port(t, "port1")
		gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), false)
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().State(), 30*time.Second, false)
		gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), true)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
		t.Run("Interface Telemetry", func(t *testing.T) {
			verifyInterfaceTelemetry(t, dut)
		})
		t.Run("Traffic Test", func(t *testing.T) {
			verifyLinkLocalTraffic(t, dut, ate, top, linkLocalFlowName)
		})
	})
}

func configureDUTGlobalIPv6(t *testing.T, dut *ondatra.DUTDevice) {
	portData := []struct {
		port   *ondatra.Port
		ipAddr string
	}{
		{
			port:   dut.Port(t, "port1"),
			ipAddr: srcDUTGlobalIPv6,
		},
		{
			port:   dut.Port(t, "port2"),
			ipAddr: dstDUTGlobalIPv6,
		},
	}

	for _, pd := range portData {
		addr := &oc.Interface_Subinterface_Ipv6_Address{
			Ip:           ygot.String(pd.ipAddr),
			PrefixLength: ygot.Uint8(globalIPv6Len),
		}
		gnmi.Update(t, dut, gnmi.OC().Interface(pd.port.Name()).Subinterface(0).Ipv6().Address(pd.ipAddr).Config(), addr)

		if _, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(pd.port.Name()).Subinterface(0).Ipv6().Address(pd.ipAddr).State(), 30*time.Second, func(val *ygnmi.Value[*oc.Interface_Subinterface_Ipv6_Address]) bool {
			v, present := val.Val()
			return present && v.GetType() == oc.IfIp_Ipv6AddressType_GLOBAL_UNICAST && v.GetPrefixLength() == globalIPv6Len && v.GetIp() == pd.ipAddr
		}).Await(t); !ok {
			t.Errorf("Couldn't configure Global Unicast IPv6 on port: %s", pd.port.Name())
		}
	}
}

func configureDUTLinkLocalInterface(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	p1 := dut.Port(t, "port1")
	srcIntf := dutSrc.NewOCInterface(p1.Name(), dut)
	srcIntf.GetOrCreateSubinterface(0).GetOrCreateIpv6().GetOrCreateAddress(dutSrc.IPv6).SetType(oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), srcIntf)

	p2 := dut.Port(t, "port2")
	dstIntf := dutDst.NewOCInterface(p2.Name(), dut)
	dstIntf.GetOrCreateSubinterface(0).GetOrCreateIpv6().GetOrCreateAddress(dutDst.IPv6).SetType(oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dstIntf)
}

func configureOTGInterface(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()

	p1 := ate.Port(t, "port1")
	ateSrc.AddToOTG(top, p1, &dutSrc)

	// global IPv6 address config for port1
	dev := top.Devices().Items()[0]
	eth := dev.Ethernets().Items()[0]
	ip := eth.Ipv6Addresses().Add().SetName(dev.Name() + ".IPv6-1")
	ip.SetAddress(srcOTGGlobalIPv6).SetGateway(srcDUTGlobalIPv6).SetPrefix(uint32(globalIPv6Len))

	p2 := ate.Port(t, "port2")
	ateDst.AddToOTG(top, p2, &dutDst)

	// global IPv6 address config for port2
	for _, d := range top.Devices().Items() {
		if d.Name() == ateDst.Name {
			dev = d
		}
	}
	eth = dev.Ethernets().Items()[0]
	ip = eth.Ipv6Addresses().Add().SetName(dev.Name() + ".IPv6-1")
	ip.SetAddress(dstOTGGlobalIPv6).SetGateway(dstDUTGlobalIPv6).SetPrefix(uint32(globalIPv6Len))
}

func otgSrcToDstFlow(t *testing.T, top gosnappi.Config, srcIPv6, dstIPv6, flowName string) {
	top.Flows().Clear()
	flow := top.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(ateSrc.MAC)
	e1.Dst().SetValue(ateDst.MAC)
	flow.TxRx().Device().SetTxNames([]string{ateSrc.Name + ".IPv6"}).SetRxNames([]string{ateDst.Name + ".IPv6"})
	v6 := flow.Packet().Add().Ipv6()
	v6.Src().SetValue(srcIPv6)
	v6.Dst().SetValue(dstIPv6)
}

func verifyLinkLocalTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, flowName string) {
	p1 := dut.Port(t, "port1")
	beforeInPkts := gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).Counters().InPkts().State())
	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(15 * time.Second)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().State())
	otgTxPkts := flowMetrics.GetOutPkts()
	otgRxPkts := flowMetrics.GetInPkts()

	if otgTxPkts == 0 {
		t.Fatalf("txPackets is 0")
	}
	if got, want := 100*float32(otgTxPkts-otgRxPkts)/float32(otgTxPkts), float32(99); got < want {
		t.Errorf("LossPct for flow %s got %f, want 100", flowName, got)
	}
	afterInPkts := gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).Counters().InPkts().State())
	recvDUTPkts := afterInPkts - beforeInPkts
	if got, want := lossPct(otgTxPkts, recvDUTPkts), 1.0; got > want {
		t.Errorf("LossPct for flow %s got %f, want less than %f%%", flowName, got, want)
	}
}

func verifyGlobalUnicastTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, flowName string) {
	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(15 * time.Second)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)

	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().State())
	otgTxPkts := flowMetrics.GetOutPkts()
	otgRxPkts := flowMetrics.GetInPkts()

	if otgTxPkts == 0 {
		t.Fatalf("txPackets is 0")
	}
	if got, want := lossPct(otgTxPkts, otgRxPkts), float64(1); got > want {
		t.Errorf("LossPct for flow %s got %f, want 0", flowName, got)
	}
}

func lossPct(tx, rx uint64) float64 {
	if tx > rx {
		return 100 * float64(tx-rx) / float64(tx)
	}
	// When computing loss across DUT and OTG, DUT also counts received
	// protocol packets not part of the OTG traffic flow.
	return 0
}

func verifyInterfaceTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	for p, attr := range map[string]attrs.Attributes{
		p1.Name(): dutSrc,
		p2.Name(): dutDst,
	} {
		conf := gnmi.Get(t, dut, gnmi.OC().Interface(p).Subinterface(0).Ipv6().Address(attr.IPv6).Config())
		if got, want := conf.GetIp(), attr.IPv6; got != want {
			t.Errorf("IP address config-path mismatch for port: %s, got: %s, want: %s", p, got, want)
		}
		if got, want := conf.GetType(), oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST; got != want {
			t.Errorf("IP address type config-path mismatch for port: %s, got: %v, want: %v", p, got, want)
		}
		if got, want := conf.GetPrefixLength(), attr.IPv6Len; got != want {
			t.Errorf("IP address prefix length config-path mismatch for port: %s, got: %d, want: %d", p, got, want)
		}

		state := gnmi.Get(t, dut, gnmi.OC().Interface(p).Subinterface(0).Ipv6().Address(attr.IPv6).State())
		if got, want := state.GetIp(), attr.IPv6; got != want {
			t.Errorf("IP address state mismatch for port: %s, got: %s, want: %s", p, got, want)
		}
		if got, want := state.GetType(), oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST; got != want {
			t.Errorf("IP address type state mismatch for port: %s, got: %v, want: %v", p, got, want)
		}
		if got, want := state.GetPrefixLength(), attr.IPv6Len; got != want {
			t.Errorf("IP address prefix length state mismatch for port: %s, got: %d, want: %d", p, got, want)
		}
	}
}
