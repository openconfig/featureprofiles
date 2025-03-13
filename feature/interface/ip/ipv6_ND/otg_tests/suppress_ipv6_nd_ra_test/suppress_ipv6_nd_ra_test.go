// Copyright 2025 Google LLC
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

package suppress_ipv6_nd_ra_test

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
	"github.com/openconfig/ygot/ygot"
)

// Reserving the testbed and running tests.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	dutPort = attrs.Attributes{
		Desc:    "dutport",
		IPv6:    "2001:db8::1",
		IPv6Len: 126,
		MAC:     "02:11:01:00:00:04",
	}

	atePort = attrs.Attributes{
		Name:    "ateport",
		MAC:     "02:11:01:00:00:01",
		IPv6:    "2001:db8::2",
		IPv6Len: 126,
	}
)

// Configures DUT Port.
func configureDUT(t *testing.T, a *attrs.Attributes, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")

	i := a.NewOCInterface(p1.Name(), dut)
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	routerAdvert := s6.GetOrCreateRouterAdvertisement()
	routerAdvert.SetSuppress(true)

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), i)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// Configures OTG interfaces to send and receive ipv6 packets.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	topo := gosnappi.NewConfig()

	t.Logf("Configuring OTG port1")
	dstPort := topo.Ports().Add().SetName("port1")
	dstDev := topo.Devices().Add().SetName(atePort.Name)
	dstEth := dstDev.Ethernets().Add().SetName(atePort.Name + ".Eth").SetMac(atePort.MAC)
	dstEth.Connection().SetPortName(dstPort.Name())
	dstIpv6 := dstEth.Ipv6Addresses().Add().SetName(atePort.Name + ".IPv6")
	dstIpv6.SetAddress(atePort.IPv6).SetGateway(dutPort.IPv6).SetPrefix(uint32(atePort.IPv6Len))

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

	deviceRASuppressQuery := gnmi.OC().Interface(txPort.Name()).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config()
	raSuppressOnDevice := gnmi.Get(t, dut, deviceRASuppressQuery)
	t.Logf("Router Advertisement Suppress State = %v", raSuppressOnDevice)

	deviceRAConfigQuery := gnmi.OC().Interface(txPort.Name()).Subinterface(0).Ipv6().RouterAdvertisement().Enable().Config()
	raConfigOnDevice := gnmi.Get(t, dut, deviceRAConfigQuery)
	t.Logf("Router Advertisement Config State = %v", raConfigOnDevice)
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
	validatePackets(t, f.Name(), ipv6Solicitation)
}

// To detect if the routerAdvertisement packet is found in the captured packets.
func validatePackets(t *testing.T, fileName string, ipv6Solicitation bool) {
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
				if routerAdvert != nil && !ipv6Solicitation {
					t.Fatalf("Error: Found a periodic router advertisement packet!")
				} else if routerAdvert != nil && ipv6Solicitation {
					t.Logf("Router advertisement packet found in response to router solicitation!")
				}
			}
		}
	}
}

func TestIpv6NDRA(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, &dutPort, dut)
	otgConfig := configureOTG(t, ate)
	t.Run("RT-5.11.1: No periodical Router Advertisement are sent", func(t *testing.T) {
		verifyRATelemetry(t, dut)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
	})
	t.Run("RT-5.11.2: Router Advertisement response is sent to Router Solicitation", func(t *testing.T) {
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})
}
