// Copyright 2024 Google LLC
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

package disable_ipv6_nd_ra_test

import (
	"os"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// Reserving the testbed and running tests.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	plen6                           = 126
	ipv6                            = "IPv6"
	routerAdvertisementTimeInterval = 5
	frameSize                       = 512
	pps                             = 100
	routerAdvertisementDisabled     = true
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutsrc",
		IPv6:    "2001:db8::1",
		IPv6Len: plen6,
		MAC:     "02:11:01:00:00:04",
	}

	ateSrc = attrs.Attributes{
		Name:    "atesrc",
		MAC:     "02:11:01:00:00:01",
		IPv6:    "2001:db8::2",
		IPv6Len: plen6,
	}

	dutDst = attrs.Attributes{
		Desc:    "dutdst",
		IPv6:    "2001:db8::5",
		IPv6Len: plen6,
		MAC:     "02:11:01:00:00:05",
	}
	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:12:01:00:00:01",
		IPv6:    "2001:db8::6",
		IPv6Len: plen6,
	}
)

// Configures port1 and port2 of the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, dut))
	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, dut))
}

// Configures the given DUT interface.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(plen6)
	routerAdvert := s6.GetOrCreateRouterAdvertisement()
	routerAdvert.SetInterval(*ygot.Uint32(routerAdvertisementTimeInterval))
	if deviations.Ipv6RouterAdvertisementConfigUnsupported(dut) {
		routerAdvert.SetSuppress(*ygot.Bool(routerAdvertisementDisabled))
	} else {
		routerAdvert.SetEnable(*ygot.Bool(false))
		routerAdvert.SetMode(oc.RouterAdvertisement_Mode_ALL)
	}
	return i
}

// Configures OTG interfaces to send and recieve ipv6 packets.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Logf("Configuring OTG port1")
	srcPort := topo.Ports().Add().SetName("port1")
	srcDev := topo.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetPortName(srcPort.Name())
	srcIpv6 := srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	srcIpv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))
	t.Logf("Configuring OTG port2")
	dstPort := topo.Ports().Add().SetName("port2")
	dstDev := topo.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetPortName(dstPort.Name())
	dstIpv6 := dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6")
	dstIpv6.SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))
	topo.Captures().Add().SetName("raCapture").SetPortNames([]string{dstPort.Name()}).SetFormat(gosnappi.CaptureFormat.PCAP)
	t.Logf("OTG configuration completed!")
	topo.Flows().Clear().Items()
	ate.OTG().PushConfig(t, topo)
	time.Sleep(10 * time.Second)
	t.Logf("starting protocols... ")
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	return topo
}

// Verifies that desired parameters are set with required value on the device.
func verifyRATelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	txPort := dut.Port(t, "port1")
	telemetryTimeIntervalQuery := gnmi.OC().Interface(txPort.Name()).Subinterface(0).Ipv6().RouterAdvertisement().Interval().State()
	timeIntervalOnTelemetry := gnmi.Get(t, dut, telemetryTimeIntervalQuery)
	t.Logf("Required RA time interval = %v, RA Time interval observed on telemetry = %v ", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
	if timeIntervalOnTelemetry != routerAdvertisementTimeInterval {
		t.Fatalf("Inconsistent Time interval!\nRequired RA time interval = %v and Configured RA Time Interval = %v are not same!", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
	}

	if deviations.Ipv6RouterAdvertisementConfigUnsupported(dut) {
		deviceRAConfigQuery := gnmi.OC().Interface(txPort.Name()).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config()
		raConfigOnDevice := gnmi.Get(t, dut, deviceRAConfigQuery)
		t.Logf("Router Advertisement State = %v", raConfigOnDevice)
	} else {
		deviceRAConfigQuery := gnmi.OC().Interface(txPort.Name()).Subinterface(0).Ipv6().RouterAdvertisement().Enable().Config()
		deviceRAModeQuery := gnmi.OC().Interface(txPort.Name()).Subinterface(0).Ipv6().RouterAdvertisement().Enable().Config()
		raModeOnDevice := gnmi.Get(t, dut, deviceRAModeQuery)
		raConfigOnDevice := gnmi.Get(t, dut, deviceRAConfigQuery)
		t.Logf("Router Advertisement mode = %v", raModeOnDevice)
		t.Logf("Router Advertisement State = %v", raConfigOnDevice)
	}
}

// Captures traffic statistics and verifies for the loss.
func verifyOTGPacketCaptureForRA(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, ipv6Solicitation bool, waitTime uint8) {
	otg := ate.OTG()
	otg.StartProtocols(t)

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
	if ipv6Solicitation {
		otgutils.WaitForARP(t, ate.OTG(), config, "IPv6")
	}

	time.Sleep(time.Duration(waitTime) * time.Second)
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(config.Ports().Items()[1].Name()))
	t.Logf("Config Ports %v", config.Ports().Items())
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	f.Close()
	validatePackets(t, f.Name())
}

// To detect if the routerAdvertisement packet is found in the captured packets.
func validatePackets(t *testing.T, fileName string) {
	t.Logf("Reading pcap file from : %v", fileName)
	handle, err := pcap.OpenOffline(fileName)
	if err != nil {
		t.Logf("No Packets found in the file = %v !", fileName)
		return
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
		if ipv6Layer != nil {

			icmpv6Layer := packet.Layer(layers.LayerTypeICMPv6)
			if icmpv6Layer != nil {
				routerAdvert := packet.Layer(layers.LayerTypeICMPv6RouterAdvertisement)
				if routerAdvert != nil {
					t.Fatalf("Error:Found a router advertisement packet!")
				}

			}
		}
	}
	t.Logf("No Router advertisement packets found!")
}

func TestIpv6NDRA(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	otgConfig := configureOTG(t, ate)
	t.Run("TestCase-1: No periodical Router Advertisement", func(t *testing.T) {
		verifyRATelemetry(t, dut)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
	})
	t.Run("TestCase-2: No Router Advertisement in response to Router Solicitation", func(t *testing.T) {
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})

}
