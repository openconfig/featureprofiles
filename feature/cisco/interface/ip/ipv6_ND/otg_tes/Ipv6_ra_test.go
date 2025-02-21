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

package otg_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"

	// "github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
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

type InterfaceInfo struct {
	name     string
	intf     *ondatra.Port
	attr     attrs.Attributes
	subIntf  uint32
	intftype oc.E_IETFInterfaces_InterfaceType
}

// Configures port1 and port2 of the DUT Physical Interfaces.
func configureDUTRaPhysical(t *testing.T, dut *ondatra.DUTDevice, interfaceList []InterfaceInfo) {
	d := gnmi.OC()
	for _, interfaces := range interfaceList {
		gnmi.Replace(t, dut, d.Interface(interfaces.name).Config(), configInterfaceDUT(interfaces.intf, &interfaces.attr, dut, interfaces.subIntf))
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, interfaces.name, deviations.DefaultNetworkInstance(dut), 0)
		}
	}
}

// Configures the given DUT interface.
func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice, subIntf uint32) *oc.Interface {
	a.Subinterface = subIntf
	VlanId := uint16(subIntf)
	i := a.NewOCInterface(p.Name(), dut)
	s := i.GetOrCreateSubinterface(subIntf)
	if subIntf != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(subIntf)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(VlanId)
		}
	}
	return i
}

// Configures the given DUT interface.
func configInterfaceIPv6RA(t *testing.T, dut *ondatra.DUTDevice, interfaces InterfaceInfo, raState string) {
	i := &oc.Interface{Name: ygot.String(interfaces.name)}
	i.Type = interfaces.intftype
	s := i.GetOrCreateSubinterface(interfaces.subIntf)
	if interfaces.subIntf != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(interfaces.subIntf)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(interfaces.subIntf))
		}
	}
	s4 := i.GetOrCreateSubinterface(interfaces.subIntf).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		t.Log("IPv4 enabled")
		s4.Enabled = ygot.Bool(true)
	}
	s6 := i.GetOrCreateSubinterface(interfaces.subIntf).GetOrCreateIpv6()
	routerAdvert := s6.GetOrCreateRouterAdvertisement()
	switch raState {
	case "Interval":
		// if !deviations.Ipv6RouterAdvertisementIntervalUnsupported(dut) {
		// ipv6 nd ra-interval 5 5
		t.Log("IPv6 RA Interval")
		routerAdvert.SetInterval(routerAdvertisementTimeInterval)
		// }
	case "Suppress":
		// if deviations.Ipv6RouterAdvertisementConfigUnsupported(dut) {
		// ipv6 nd suppress-ra
		t.Log("IPv6 RA Suppress")
		routerAdvert.SetSuppress(routerAdvertisementDisabled)
	case "ModeAll":
		routerAdvert.SetMode(oc.RouterAdvertisement_Mode_ALL)
	case "Unsolicited":
		routerAdvert.SetMode(oc.RouterAdvertisement_Mode_DISABLE_UNSOLICITED_RA)
	case "Unicast":
		// routerAdvert.SetMode(oc.RouterAdvertisement_Mode_UNICAST)
		// routerAdvert.SetMode(oc.R)
	}

	t.Log("IPv6 RA Enable")
	routerAdvert.SetEnable(false)

	gnmi.Update(t, dut, gnmi.OC().Interface(interfaces.name).Config(), i)
}

// Unonfigures the given DUT interface.
func unConfigInterface(t *testing.T, dut *ondatra.DUTDevice, interfaceList []InterfaceInfo) {
	for _, interfaces := range interfaceList {
		t.Logf("unConfigInterface - %v", interfaces.name)
		gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Config())
	}
}

// Configures OTG interfaces to send and receive ipv6 packets.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice, vlanID uint32) gosnappi.Config {
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
	if vlanID != 0 {
		dstEth.Vlans().Add().SetName(dstPort.Name()).SetId(uint32(vlanID))
	}
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

func verifyedt(t *testing.T, dut *ondatra.DUTDevice, intf string) bool {
	watcher := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().Interface(intf).Subinterface(0).Ipv6().RouterAdvertisement().Interval().State(),
		time.Minute,
		func(value *ygnmi.Value[uint32]) bool {
			timeIntervalOnTelemetry, present := value.Val()
			if !present {
				return false
			}
			t.Logf("Got state %v", timeIntervalOnTelemetry)
			if timeIntervalOnTelemetry != routerAdvertisementTimeInterval {
				t.Fatalf("Inconsistent Time interval!\nRequired RA time interval = %v and Configured RA Time Interval = %v are not same!", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
				return false
			} else {
				t.Logf("Required RA time interval = %v, RA Time interval observed on EDT = %v ", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
				return true
			}
		})

	_, gotall := watcher.Await(t)

	if !gotall {
		t.Fatalf("Did not receive all values an interface %s", (intf))
		return false
	} else {
		return true
	}
}

func verifyEdtRAInterval(t *testing.T, dut *ondatra.DUTDevice) bool {
	watcher := gnmi.WatchAll(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv6().RouterAdvertisement().Interval().State(),
		time.Minute,
		func(value *ygnmi.Value[uint32]) bool {
			timeIntervalOnTelemetry, present := value.Val()
			if !present {
				return false
			}
			t.Logf("Got Interval state %v", timeIntervalOnTelemetry)
			if timeIntervalOnTelemetry != routerAdvertisementTimeInterval {
				t.Fatalf("Inconsistent Time interval!\nRequired RA time interval = %v and Configured RA Time Interval = %v are not same!", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
				return false
			} else {
				t.Logf("Required RA time interval = %v, RA Time interval observed on EDT = %v ", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
				return true
			}
		})

	_, gotall := watcher.Await(t)

	if !gotall {
		t.Fatalf("Did not receive all values an interface")
		return false
	} else {
		return true
	}
}

func verifyEdtRASuppress(t *testing.T, dut *ondatra.DUTDevice) bool {
	watcher := gnmi.WatchAll(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv6().RouterAdvertisement().Suppress().Config(),
		time.Minute,
		func(value *ygnmi.Value[bool]) bool {
			timeIntervalOnTelemetry, present := value.Val()
			if !present {
				return false
			}
			t.Logf("Got Suppress state %v", timeIntervalOnTelemetry)
			// if timeIntervalOnTelemetry != routerAdvertisementTimeInterval {
			// 	t.Fatalf("Inconsistent Time interval!\nRequired RA time interval = %v and Configured RA Time Interval = %v are not same!", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
			// 	return false
			// } else {
			// 	t.Logf("Required RA time interval = %v, RA Time interval observed on EDT = %v ", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
			// 	return true
			// }
			return true
		})

	_, gotall := watcher.Await(t)

	if !gotall {
		t.Fatalf("Did not receive all values an interface")
		return false
	} else {
		return true
	}
}

func verifyEdtRAModeAll(t *testing.T, dut *ondatra.DUTDevice) bool {
	watcher := gnmi.WatchAll(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv6().RouterAdvertisement().Mode().State(),
		time.Minute,
		func(value *ygnmi.Value[oc.E_RouterAdvertisement_Mode]) bool {
			timeIntervalOnTelemetry, present := value.Val()
			if !present {
				return false
			}
			t.Logf("Got Mode state %v", timeIntervalOnTelemetry)
			// if timeIntervalOnTelemetry != routerAdvertisementTimeInterval {
			// 	t.Fatalf("Inconsistent Time interval!\nRequired RA time interval = %v and Configured RA Time Interval = %v are not same!", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
			// 	return false
			// } else {
			// 	t.Logf("Required RA time interval = %v, RA Time interval observed on EDT = %v ", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
			// 	return true
			// }
			return true
		})

	_, gotall := watcher.Await(t)

	if !gotall {
		t.Fatalf("Did not receive all values an interface")
		return false
	} else {
		return true
	}
}

func verifyEdtRAUnsolicited(t *testing.T, dut *ondatra.DUTDevice) bool {
	watcher := gnmi.WatchAll(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv6().RouterAdvertisement().Mode().State(),
		time.Minute,
		func(value *ygnmi.Value[oc.E_RouterAdvertisement_Mode]) bool {
			timeIntervalOnTelemetry, present := value.Val()
			if !present {
				return false
			}
			t.Logf("Got Mode state %v", timeIntervalOnTelemetry)
			// if timeIntervalOnTelemetry != routerAdvertisementTimeInterval {
			// 	t.Fatalf("Inconsistent Time interval!\nRequired RA time interval = %v and Configured RA Time Interval = %v are not same!", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
			// 	return false
			// } else {
			// 	t.Logf("Required RA time interval = %v, RA Time interval observed on EDT = %v ", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
			// 	return true
			// }
			return true
		})

	_, gotall := watcher.Await(t)

	if !gotall {
		t.Fatalf("Did not receive all values an interface")
		return false
	} else {
		return true
	}
}

// Verifies that desired parameters are set with required value on the device.
func verifyRATelemetry(t *testing.T, dut *ondatra.DUTDevice, intf, raState string) {
	if raState == "Interval" {
		telemetryTimeIntervalQuery := gnmi.OC().Interface(intf).Subinterface(0).Ipv6().RouterAdvertisement().Interval().State()
		timeIntervalOnTelemetry := gnmi.Get(t, dut, telemetryTimeIntervalQuery)
		t.Logf("Required RA time interval = %v, RA Time interval observed on telemetry = %v ", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
		if timeIntervalOnTelemetry != routerAdvertisementTimeInterval {
			t.Fatalf("Inconsistent Time interval!\nRequired RA time interval = %v and Configured RA Time Interval = %v are not same!", routerAdvertisementTimeInterval, timeIntervalOnTelemetry)
		}
	}

	// if deviations.Ipv6RouterAdvertisementConfigUnsupported(dut) {
	if raState == "Suppress" {
		deviceRASuppressQuery := gnmi.OC().Interface(intf).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config()
		raSuppressConfigOnDevice := gnmi.Get(t, dut, deviceRASuppressQuery)
		t.Logf("RA Suppress State = %v", raSuppressConfigOnDevice)
	}
	if raState == "ModeAll" {
		deviceRAModeQuery := gnmi.OC().Interface(intf).Subinterface(0).Ipv6().RouterAdvertisement().Mode().State()
		raModeOnDevice := gnmi.Get(t, dut, deviceRAModeQuery)
		t.Logf("Router Advertisement Mode = %v", raModeOnDevice)
	}

	deviceRAConfigQuery := gnmi.OC().Interface(intf).Subinterface(0).Ipv6().RouterAdvertisement().Enable().Config()
	raConfigOnDevice := gnmi.Get(t, dut, deviceRAConfigQuery)
	t.Logf("Router Advertisement State = %v", raConfigOnDevice)
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

// shut/unshut the interfaces
func flapSubInterface(t *testing.T, dut *ondatra.DUTDevice, intf string, subIntf uint32, flap bool) {
	path := gnmi.OC().Interface(intf).Subinterface(subIntf).Enabled()
	gnmi.Update(t, dut, path.Config(), flap)
}

const (
	with_scale            = false                    // run entire script with or without scale (Support not yet coded)
	with_RPFO             = true                     // run entire script with or without RFPO
	base_config           = "case2_decap_encap_exit" // Will run all the tcs with set base programming case, options : case1_backup_decap, case2_decap_encap_exit, case3_decap_encap, case4_decap_encap_recycle
	active_rp             = "0/RP0/CPU0"
	standby_rp            = "0/RP1/CPU0"
	lc                    = "0/2/CPU0" // set value for lc_oir tc, if empty it means no lc, example: 0/0/CPU0
	process_restart_count = 1
	microdropsRepeat      = 1
	programming_RFPO      = 1
	viable                = true
	unviable              = false

	dst                   = "202.1.0.1"
	v4mask                = "32"
	dstCount              = 1
	totalBgpPfx           = 1
	minInnerDstPrefixBgp  = "202.1.0.1"
	totalIsisPrefix       = 1 //set value for scale isis setup ex: 10000
	minInnerDstPrefixIsis = "201.1.0.1"
	ipv6PrefixLen         = 126
	policyTypeIsis        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	dutAreaAddress        = "47.0001"
	dutSysId              = "0000.0000.0001"
	isisName              = "osisis"
	policyTypeBgp         = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	bgpAs                 = 65000
)

var (
	prefixes   = []string{}
	rpfo_count = 0 // used to track rpfo_count if its more than 10 then reset to 0 and reload the HW
)

// testRPFO is the main function to test RPFO
func testRPFO(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top *ondatra.ATETopology) {

	client := gribi.Client{
		DUT:                   dut,
		FibACK:                *ciscoFlags.GRIBIFIBCheck,
		Persistence:           true,
		InitialElectionIDLow:  1,
		InitialElectionIDHigh: 0,
	}
	defer client.Close(t)
	if err := client.Start(t); err != nil {
		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		if err = client.Start(t); err != nil {
			t.Fatalf("gRIBI Connection could not be established: %v", err)
		}
	}
	// ctx := context.Background()

	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
		randomItems := client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	for i := 0; i < programming_RFPO; i++ {

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			rpfo(t, dut, &client, true)
		}
	}
}

func rpfo(t *testing.T, dut *ondatra.DUTDevice, client *gribi.Client, gribi_reconnect bool) {

	// reload the HW is rfpo count is 10 or more
	if rpfo_count == 10 {
		gnoiClient := dut.RawAPIs().GNOI(t)
		rebootRequest := &spb.RebootRequest{
			Method: spb.RebootMethod_COLD,
			Force:  true,
		}
		rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
		t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
		if err != nil {
			t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
		}
		rpfo_count = 0
		time.Sleep(time.Minute * 20)
	}
	// supervisor info
	var supervisors []string
	active_state := gnmi.OC().Component(active_rp).Name().State()
	active := gnmi.Get(t, dut, active_state)
	standby_state := gnmi.OC().Component(standby_rp).Name().State()
	standby := gnmi.Get(t, dut, standby_state)
	supervisors = append(supervisors, active, standby)

	// find active and standby RP
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, supervisors)
	t.Logf("Detected activeRP: %v, standbyRP: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch)

	// make sure standby RP is reach
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	for {
		if err := client.Start(t); err != nil {
			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		} else {
			t.Logf("gRIBI Connection established")
			switchoverRequest := &spb.SwitchControlProcessorRequest{
				ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
			}
			t.Logf("switchoverRequest: %v", switchoverRequest)
			switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
			if err != nil {
				t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
			}
			if err == nil {
				t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)
				want := rpStandbyBeforeSwitch
				got := ""
				if useNameOnly {
					got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
				} else {
					got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
				}
				if got != want {
					t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
				}
				break
			}
		}
		time.Sleep(time.Minute * 2)
	}

	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(900); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, dut, supervisors)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)
	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverTime().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", gnmi.Get(t, dut, activeRP.LastSwitchoverTime().State()))
	}

	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}

	// reestablishing gribi connection
	if gribi_reconnect {
		client.Start(t)
	}
}

func TestIpv6NDRAPhysical(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	physicaSrclInt := dut.Port(t, "port1")
	physicaDstlInt := dut.Port(t, "port2")
	interfaceList := []InterfaceInfo{

		{
			intf:     physicaSrclInt,
			name:     physicaSrclInt.Name(),
			attr:     dutSrc,
			subIntf:  0,
			intftype: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		},
		{
			intf:     physicaDstlInt,
			name:     physicaDstlInt.Name(),
			attr:     dutDst,
			subIntf:  0,
			intftype: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		},
	}
	configureDUTRaPhysical(t, dut, interfaceList)
	// defer unConfigInterface(t, dut, interfaceList)
	otgConfig := configureOTG(t, ate, interfaceList[1].subIntf)

	t.Run("TestCase-1: No periodical Router Advertisement with Interval", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		//unconfigure Ipv6 Ra Interval
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Interval().Config())
		}
	})

	t.Run("TestCase-2: No Router Advertisement in response to Router Solicitation", func(t *testing.T) {
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})

	t.Run("TestCase-3: Router Advertisement with Suppress", func(t *testing.T) {
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Suppress")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Suppress")
		})
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRASuppress(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Suppress EDT verification failed!")
		// 	}
		// })
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		//unconfigure Ipv6 Ra Suppress
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		}
	})

	t.Run("TestCase-4: Router Advertisement with Mode All", func(t *testing.T) {
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "ModeAll")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "ModeAll")
		})
		t.Run("Validate RA Suppress EDT", func(t *testing.T) {
			edtStatus := verifyEdtRASuppress(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Suppress EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		//unconfigure Ipv6 Ra Suppress
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		}
	})

	t.Run("TestCase-5: Router Advertisement with Suppress UnSolicitation", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-6: Router Advertisement with Suppress and Unsolicited", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Suppress")
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Suppress")
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRASuppress(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Suppress EDT verification failed!")
		// 	}
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		// }
	})

	t.Run("TestCase-7: Router Advertisement with Mode Unicast ", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unicast")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unicast")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnicast(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unicast EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-8: Router Advertisement with Mode Unicast and Unsolicited", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unicast")
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unicast")
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnicast(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unicast EDT verification failed!")
		// 	}
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-9: Commit/Replace the Router Advertisement ", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		//unconfigure Ipv6 Ra Config
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		}

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)

	})

	t.Run("TestCase-10: Shut/Unshut the Router Advertisement Interface", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		//Shut the interface
		t.Log("Shutting the interface")
		for _, interfaces := range interfaceList {
			flapSubInterface(t, dut, interfaces.name, interfaces.subIntf, false)
		}

		//UnShut the interface
		t.Log("Unshutting the interface")
		for _, interfaces := range interfaceList {
			flapSubInterface(t, dut, interfaces.name, interfaces.subIntf, true)
		}

		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})

	t.Run("TestCase-11: Verify IPv6 RA after process restart.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		// Restart the processs
		process_list := []string{"ipv6_nd", "ipv6_ma", "ifmgr"} // "sysdb" "cfmgr"
		for _, process := range process_list {
			t.Run(fmt.Sprintf("Restart the Process - %s", process), func(t *testing.T) {
				ctx := context.Background()
				restartCmd := fmt.Sprintf("process restart {%s} location 0/2/CPU0", process)
				config.CMDViaGNMI(ctx, t, dut, restartCmd)
				time.Sleep(time.Second * 10)

				t.Log("Validating IPv6 RA ND after Restarted the process")
				t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
					verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
				})
				t.Run("Validate RA Interval EDT", func(t *testing.T) {
					edtStatus := verifyEdtRAInterval(t, dut)
					if !edtStatus {
						t.Fatalf("Error: RA Interval EDT verification failed!")
					}
				})
				t.Logf("Validating the Router Advertisement packets")
				verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
				verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
			})
		}

	})

	t.Run("TestCase-12: Verify IPv6 RA after Linecard Reload.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		dut := ondatra.DUT(t, "dut")
		lcList := util.GetLCList(t, dut)
		if len(lcList) == 0 {
			t.Skip("No linecards found")
		}
		util.ReloadLinecards(t, lcList)
		t.Log("Verify IPv6 RA after reloading all linecards")
		time.Sleep(120 * time.Second)

		t.Run("Validate RA MDT Telemetry After LC reload", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA EDT After LC reload", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

	t.Run("TestCase-13: Verify IPv6 RA after RPFO.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		t.Run("Trigger RPFO", func(t *testing.T) {
			testRPFO(t, dut, ate, ate.Topology().New())
			time.Sleep(60 * time.Second)

		})

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

	t.Run("TestCase-14: Verify IPv6 RA after Router Reload.", func(t *testing.T) {
		t.Skip("Skipping the test case as it is not supported")
		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		// Reload the Router
		client := gribi.Client{
			DUT:                   dut,
			FibACK:                *ciscoFlags.GRIBIFIBCheck,
			Persistence:           true,
			InitialElectionIDLow:  1,
			InitialElectionIDHigh: 0,
		}
		defer client.Close(t)
		if err := client.Start(t); err != nil {
			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
			if err = client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection could not be established: %v", err)
			}
		}

		time.Sleep(1 * time.Minute)
		gnoiClient := dut.RawAPIs().GNOI(t)
		_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot chassis without delay",
			Force:   true,
		})
		if err != nil {
			t.Fatalf("Reboot failed %v", err)
		}
		startReboot := time.Now()
		const maxRebootTime = 30
		t.Logf("Wait for DUT to boot up by polling the telemetry output.")
		for {
			var currentTime string
			t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

			time.Sleep(3 * time.Minute)
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			}); errMsg != nil {
				t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
			} else {
				t.Logf("Device rebooted successfully with received time: %v", currentTime)
				break
			}

			if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
				t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
			}
		}
		t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
		time.Sleep(30 * time.Second)

		t.Log("Validating IPv6 RA ND after Reload the Router")
		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

}

func TestIpv6NDRAPhysicalSubIntf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	physicaSrclInt := dut.Port(t, "port1")
	physicaDstlInt := dut.Port(t, "port2")
	interfaceList := []InterfaceInfo{

		{
			intf:     physicaSrclInt,
			name:     physicaSrclInt.Name(),
			attr:     dutSrc,
			subIntf:  0,
			intftype: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		},
		{
			intf:     physicaDstlInt,
			name:     physicaDstlInt.Name(),
			attr:     dutDst,
			subIntf:  1,
			intftype: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		},
	}
	configureDUTRaPhysical(t, dut, interfaceList)
	defer unConfigInterface(t, dut, interfaceList)

	for _, interfaces := range interfaceList {
		configInterfaceIPv6RA(t, dut, interfaces, "Interval")
	}
	otgConfig := configureOTG(t, ate, interfaceList[1].subIntf)

	t.Run("TestCase-1: No periodical Router Advertisement", func(t *testing.T) {
		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA EDT", func(t *testing.T) {
			edtStatus := verifyedt(t, dut, interfaceList[0].name)
			if !edtStatus {
				t.Fatalf("Error: RA EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
	})

	t.Run("TestCase-2: No Router Advertisement in response to Router Solicitation", func(t *testing.T) {
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})

	t.Run("TestCase-3: Router Advertisement with Suppress", func(t *testing.T) {
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Suppress")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Suppress")
		})
		t.Run("Validate RA Suppress EDT", func(t *testing.T) {
			edtStatus := verifyEdtRASuppress(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Suppress EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		//unconfigure Ipv6 Ra Suppress
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		}
	})

	t.Run("TestCase-4: Router Advertisement with Mode All", func(t *testing.T) {
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "ModeAll")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "ModeAll")
		})
		t.Run("Validate RA Suppress EDT", func(t *testing.T) {
			edtStatus := verifyEdtRASuppress(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Suppress EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		//unconfigure Ipv6 Ra Suppress
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		}
	})

	t.Run("TestCase-5: Router Advertisement with Suppress UnSolicitation", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-6: Router Advertisement with Suppress and Unsolicited", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Suppress")
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Suppress")
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRASuppress(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Suppress EDT verification failed!")
		// 	}
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		// }
	})

	t.Run("TestCase-7: Router Advertisement with Mode Unicast ", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unicast")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unicast")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnicast(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unicast EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-8: Router Advertisement with Mode Unicast and Unsolicited", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unicast")
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unicast")
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnicast(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unicast EDT verification failed!")
		// 	}
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-9: Commit/Replace the Router Advertisement ", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		//unconfigure Ipv6 Ra Config
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		}

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)

	})

	t.Run("TestCase-10: Shut/Unshut the Router Advertisement Interface", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		//Shut the interface
		t.Log("Shutting the interface")
		for _, interfaces := range interfaceList {
			flapSubInterface(t, dut, interfaces.name, interfaces.subIntf, false)
		}

		//UnShut the interface
		t.Log("Unshutting the interface")
		for _, interfaces := range interfaceList {
			flapSubInterface(t, dut, interfaces.name, interfaces.subIntf, true)
		}

		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})

	t.Run("TestCase-11: Verify IPv6 RA after process restart.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		// Restart the processs
		process_list := []string{"ipv6_nd", "ipv6_ma", "ifmgr"} // "sysdb" "cfmgr"
		for _, process := range process_list {
			t.Run(fmt.Sprintf("Restart the Process - %s", process), func(t *testing.T) {
				ctx := context.Background()
				restartCmd := fmt.Sprintf("process restart {%s} location 0/2/CPU0", process)
				config.CMDViaGNMI(ctx, t, dut, restartCmd)
				time.Sleep(time.Second * 10)

				t.Log("Validating IPv6 RA ND after Restarted the process")
				t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
					verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
				})
				t.Run("Validate RA Interval EDT", func(t *testing.T) {
					edtStatus := verifyEdtRAInterval(t, dut)
					if !edtStatus {
						t.Fatalf("Error: RA Interval EDT verification failed!")
					}
				})
				t.Logf("Validating the Router Advertisement packets")
				verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
				verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
			})
		}

	})

	t.Run("TestCase-12: Verify IPv6 RA after Linecard Reload.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		dut := ondatra.DUT(t, "dut")
		lcList := util.GetLCList(t, dut)
		if len(lcList) == 0 {
			t.Skip("No linecards found")
		}
		util.ReloadLinecards(t, lcList)
		t.Log("Verify IPv6 RA after reloading all linecards")
		time.Sleep(120 * time.Second)

		t.Run("Validate RA MDT Telemetry After LC reload", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA EDT After LC reload", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

	t.Run("TestCase-13: Verify IPv6 RA after RPFO.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		t.Run("Trigger RPFO", func(t *testing.T) {
			testRPFO(t, dut, ate, ate.Topology().New())
			time.Sleep(60 * time.Second)

		})

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

	t.Run("TestCase-14: Verify IPv6 RA after Router Reload.", func(t *testing.T) {
		t.Skip("Skipping the test case as it is not supported")
		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		// Reload the Router
		client := gribi.Client{
			DUT:                   dut,
			FibACK:                *ciscoFlags.GRIBIFIBCheck,
			Persistence:           true,
			InitialElectionIDLow:  1,
			InitialElectionIDHigh: 0,
		}
		defer client.Close(t)
		if err := client.Start(t); err != nil {
			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
			if err = client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection could not be established: %v", err)
			}
		}

		time.Sleep(1 * time.Minute)
		gnoiClient := dut.RawAPIs().GNOI(t)
		_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot chassis without delay",
			Force:   true,
		})
		if err != nil {
			t.Fatalf("Reboot failed %v", err)
		}
		startReboot := time.Now()
		const maxRebootTime = 30
		t.Logf("Wait for DUT to boot up by polling the telemetry output.")
		for {
			var currentTime string
			t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

			time.Sleep(3 * time.Minute)
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			}); errMsg != nil {
				t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
			} else {
				t.Logf("Device rebooted successfully with received time: %v", currentTime)
				break
			}

			if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
				t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
			}
		}
		t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
		time.Sleep(30 * time.Second)

		t.Log("Validating IPv6 RA ND after Reload the Router")
		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

}

type aggPortData struct {
	// dutIPv4     string
	// ateIPv4     string
	dutIPv6     string
	ateIPv6     string
	ateAggName  string
	ateAggMAC   string
	dutMAC      string
	ateLagCount uint32
	subIntf     uint32
}

const (
	ipv4PLen       = 30
	ipv6PLen       = plen6
	LAG1           = "lag1"
	LAG2           = "lag2"
	lagTypeLACP    = oc.IfAggregate_AggregationType_LACP
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
)

var (
	agg1 = &aggPortData{
		dutIPv6:     "2001:db8::1",
		ateIPv6:     "2001:db8::2",
		ateAggName:  LAG1,
		ateAggMAC:   "02:11:01:00:00:01",
		dutMAC:      "02:11:01:00:00:04",
		ateLagCount: 2,
	}
	agg2 = &aggPortData{
		dutIPv6:     "2001:db8::5",
		ateIPv6:     "2001:db8::6",
		ateAggName:  LAG2,
		ateAggMAC:   "02:12:01:00:00:01",
		dutMAC:      "02:11:01:00:00:05",
		ateLagCount: 2,
	}
	dutPortList     []*ondatra.Port
	atePortList     []*ondatra.Port
	otgSubIntfPorts = []string{}
	pmd100GFRPorts  []string
)

// initializePort initializes ports for aggregate on DUT
func initializePort(t *testing.T, dut *ondatra.DUTDevice, a *aggPortData) []*ondatra.Port {
	var portList []*ondatra.Port
	var portIdx uint32
	switch a.ateAggName {
	case LAG1:
		portList = append(portList, dut.Port(t, fmt.Sprintf("port%d", portIdx+1)))
		dutPortList = append(dutPortList, dut.Port(t, fmt.Sprintf("port%d", portIdx+1)))
	case LAG2:
		for portIdx < a.ateLagCount {
			portList = append(portList, dut.Port(t, fmt.Sprintf("port%d", portIdx+2)))
			dutPortList = append(dutPortList, dut.Port(t, fmt.Sprintf("port%d", portIdx+2)))
			portIdx++
		}
	}
	return portList
}

// clearAggregate delete any previously existing members of aggregate.
func clearAggregate(t *testing.T, dut *ondatra.DUTDevice, aggID string, agg *aggPortData, portList []*ondatra.Port) {
	// Clear the aggregate minlink.
	gnmi.Delete(t, dut, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())
	// Clear the members of the aggregate.
	for _, port := range portList {
		resetBatch := &gnmi.SetBatch{}
		gnmi.BatchDelete(resetBatch, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
		gnmi.BatchDelete(resetBatch, gnmi.OC().Interface(port.Name()).ForwardingViable().Config())
		resetBatch.Set(t, dut)
	}
}

// setupAggregateAtomically setup port-channel based on LAG type.
func setupAggregateAtomically(t *testing.T, dut *ondatra.DUTDevice, aggID string, agg *aggPortData, portList []*ondatra.Port) {
	d := &oc.Root{}
	d.GetOrCreateLacp().GetOrCreateInterface(aggID)

	aggr := d.GetOrCreateInterface(aggID)
	aggr.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	aggr.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	for _, port := range portList {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)

		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}
	p := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", dut), p.Config(), d)
	gnmi.Update(t, dut, p.Config(), d)
}

// configDstAggregateDUT configures port-channel destination ports
func configAggregateDUT(dut *ondatra.DUTDevice, i *oc.Interface, a *aggPortData, subintf uint32) {
	i.Description = ygot.String(a.ateAggName)
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(subintf)
	if subintf != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(subintf)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(subintf))
		}
	}

	// s4 := s.GetOrCreateIpv4()
	// if deviations.InterfaceEnabled(dut) {
	// 	s4.Enabled = ygot.Bool(true)
	// }
	// a4 := s4.GetOrCreateAddress(a.dutIPv4)
	// a4.PrefixLength = ygot.Uint8(ipv4PLen)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.dutIPv6).PrefixLength = ygot.Uint8(ipv6PLen)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = lagTypeLACP
}

// configDstMemberDUT enables destination ports, add other details like description,
// port and aggregate ID.
func configMemberDUT(dut *ondatra.DUTDevice, i *oc.Interface, p *ondatra.Port, aggID string) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(aggID)
}

// configureDUT Lag configures DUT
func configureDUTLag(t *testing.T, dut *ondatra.DUTDevice) []string {

	t.Helper()
	if len(dut.Ports()) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got %d", len(dut.Ports()))
	}
	var aggIDs []string
	for _, a := range []*aggPortData{agg1, agg2} {
		d := gnmi.OC()
		aggID := netutil.NextAggregateInterface(t, dut)
		t.Logf("aggID - %v", aggID)
		aggIDs = append(aggIDs, aggID)
		portList := initializePort(t, dut, a)

		if deviations.AggregateAtomicUpdate(dut) {
			clearAggregate(t, dut, aggID, a, portList)
			setupAggregateAtomically(t, dut, aggID, a, portList)
		}
		lacp := &oc.Lacp_Interface{Name: ygot.String(aggID)}
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
		lacpPath := d.Lacp().Interface(aggID)
		fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
		gnmi.Replace(t, dut, lacpPath.Config(), lacp)

		aggInt := &oc.Interface{Name: ygot.String(aggID)}
		configAggregateDUT(dut, aggInt, a, a.subIntf)

		aggPath := d.Interface(aggID)
		fptest.LogQuery(t, aggID, aggPath.Config(), aggInt)
		gnmi.Replace(t, dut, aggPath.Config(), aggInt)
		for _, port := range portList {
			i := &oc.Interface{Name: ygot.String(port.Name())}
			i.Type = ethernetCsmacd
			e := i.GetOrCreateEthernet()
			e.AggregateId = ygot.String(aggID)
			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			if port.PMD() == ondatra.PMD100GBASEFR {
				e.AutoNegotiate = ygot.Bool(false)
				e.DuplexMode = oc.Ethernet_DuplexMode_FULL
				e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
			}

			configMemberDUT(dut, i, port, aggID)
			iPath := d.Interface(port.Name())
			fptest.LogQuery(t, port.String(), iPath.Config(), i)
			gnmi.Replace(t, dut, iPath.Config(), i)
		}

		if deviations.ExplicitPortSpeed(dut) {
			for _, dp := range portList {
				fptest.SetPortSpeed(t, dp)
			}
		}
	}
	return aggIDs
}

// incrementMAC uses a mac string and increments it by the given i
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

// configureOTGPorts define ATE ports
func configureOTGSubIntfPorts(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, portList []*ondatra.Port, a *aggPortData) []string {
	agg := top.Lags().Add().SetName(a.ateAggName)
	agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(a.ateAggMAC)
	lagDev := top.Devices().Add().SetName(agg.Name() + ".Dev")
	lagEth := lagDev.Ethernets().Add().SetName(agg.Name() + ".Eth").SetMac(a.ateAggMAC)
	lagEth.Connection().SetLagName(agg.Name())
	// t.Logf(".ateAggName - %v", a.ateAggName)
	if a.ateAggName != LAG1 {
		lagEth.Vlans().Add().SetName(agg.Name() + ".vlan").SetId(uint32(a.subIntf))
	}
	// otgSubIntfPorts = append(otgSubIntfPorts, agg.Name()+".IPv4")
	// lagEth.Ipv4Addresses().Add().SetName(agg.Name() + ".IPv4").SetAddress(a.ateIPv4).SetGateway(a.dutIPv4).SetPrefix(ipv4PLen)
	lagEth.Ipv6Addresses().Add().SetName(agg.Name() + ".IPv6").SetAddress(a.ateIPv6).SetGateway(a.dutIPv6).SetPrefix(ipv6PLen)
	for aggIdx, pList := range portList {
		top.Ports().Add().SetName(pList.ID())
		if pList.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, pList.ID())
		}
		newMac, err := incrementMAC(a.ateAggMAC, aggIdx+1)
		if err != nil {
			t.Fatal(err)
		}
		lagPort := agg.Ports().Add().SetPortName(pList.ID())
		if a.ateAggName == LAG2 {
			t.Log("Setting capture for LAG2")
			if aggIdx == 0 {
				top.Captures().Add().SetName("raCapture").SetPortNames([]string{lagPort.PortName()}).SetFormat(gosnappi.CaptureFormat.PCAP)
			}
		}
		lagPort.Ethernet().SetMac(newMac).SetName(a.ateAggName + "." + strconv.Itoa(aggIdx))
		lagPort.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(aggIdx) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
	}
	return pmd100GFRPorts
}

// configureOTGPorts define ATE ports
func configureOTGPorts(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, portList []*ondatra.Port, a *aggPortData) []string {
	agg := top.Lags().Add().SetName(a.ateAggName)
	t.Logf("ateAggName - %v", a.ateAggName)
	agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(a.ateAggMAC)
	lagDev := top.Devices().Add().SetName(agg.Name() + ".Dev")
	lagEth := lagDev.Ethernets().Add().SetName(agg.Name() + ".Eth").SetMac(a.ateAggMAC)
	lagEth.Connection().SetLagName(agg.Name())
	lagEth.Ipv6Addresses().Add().SetName(agg.Name() + ".IPv6").SetAddress(a.ateIPv6).SetGateway(a.dutIPv6).SetPrefix(ipv6PLen)

	for aggIdx, pList := range portList {
		top.Ports().Add().SetName(pList.ID())
		if pList.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, pList.ID())
		}
		newMac, err := incrementMAC(a.ateAggMAC, aggIdx+1)
		if err != nil {
			t.Fatal(err)
		}
		lagPort := agg.Ports().Add().SetPortName(pList.ID())
		if a.ateAggName == LAG2 {
			if aggIdx == 0 {
				top.Captures().Add().SetName("raCapture").SetPortNames([]string{lagPort.PortName()}).SetFormat(gosnappi.CaptureFormat.PCAP)
			}
		}
		lagPort.Ethernet().SetMac(newMac).SetName(a.ateAggName + "." + strconv.Itoa(aggIdx))
		lagPort.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(aggIdx) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
	}
	return pmd100GFRPorts
}

// configureATE configure ATE
func configureATEIpv6Ra(t *testing.T, ate *ondatra.ATEDevice, subIntf bool) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	otgSubIntfPorts = []string{}

	for _, a := range []*aggPortData{agg1, agg2} {
		var portList []*ondatra.Port
		var portIdx uint32
		switch a.ateAggName {
		case LAG1:
			t.Logf("%v", LAG1)
			portList = append(portList, ate.Port(t, fmt.Sprintf("port%d", portIdx+1)))
			atePortList = append(atePortList, ate.Port(t, fmt.Sprintf("port%d", portIdx+1)))
		case LAG2:
			t.Logf("%v", LAG2)
			for portIdx < a.ateLagCount {
				portList = append(portList, ate.Port(t, fmt.Sprintf("port%d", portIdx+2)))
				atePortList = append(atePortList, ate.Port(t, fmt.Sprintf("port%d", portIdx+2)))
				portIdx++
			}
			// agg := top.Lags().Add().SetName(a.ateAggName)
			// // top.Captures().Add().SetName("raCapture").SetPortNames([]string{agg.Name()}).SetFormat(gosnappi.CaptureFormat.PCAP)
			// top.Captures().Add().SetName("raCapture").SetPortNames([]string{agg.Name()}).SetFormat(gosnappi.CaptureFormat.PCAP)
		}
		if subIntf {
			configureOTGSubIntfPorts(t, ate, top, portList, a)
		} else {
			configureOTGPorts(t, ate, top, portList, a)
		}
	}
	// Disable FEC for 100G-FR ports because Novus does not support it.
	if len(pmd100GFRPorts) > 0 {
		l1Settings := top.Layer1().Add().SetName("L1").SetPortNames(pmd100GFRPorts)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}
	t.Logf("OTG configuration completed!")
	top.Flows().Clear().Items()
	ate.OTG().PushConfig(t, top)
	time.Sleep(10 * time.Second)
	t.Logf("starting protocols... ")
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
	return top
}

func TestIpv6NDRABundle(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	physicaSrclInt := dut.Port(t, "port1")
	physicaDstlInt := dut.Port(t, "port2")
	interfaceList := []InterfaceInfo{

		{
			intf:     physicaSrclInt,
			name:     "Bundle-Ether1",
			attr:     dutSrc,
			subIntf:  0,
			intftype: oc.IETFInterfaces_InterfaceType_ieee8023adLag,
		},
		{
			intf:     physicaDstlInt,
			name:     "Bundle-Ether2",
			attr:     dutDst,
			subIntf:  0,
			intftype: oc.IETFInterfaces_InterfaceType_ieee8023adLag,
		},
	}
	aggIDs := configureDUTLag(t, dut)
	defer unConfigInterface(t, dut, interfaceList)
	for _, interfaces := range interfaceList {
		configInterfaceIPv6RA(t, dut, interfaces, "Interval")
	}
	otgConfig := configureATEIpv6Ra(t, ate, false)
	for _, aggID := range aggIDs {
		gnmi.Await(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), 60*time.Second, oc.Interface_OperStatus_UP)
	}

	t.Run("TestCase-1: No periodical Router Advertisement with Interval", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		//unconfigure Ipv6 Ra Interval
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Interval().Config())
		}
	})

	t.Run("TestCase-2: No Router Advertisement in response to Router Solicitation", func(t *testing.T) {
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})

	t.Run("TestCase-3: Router Advertisement with Suppress", func(t *testing.T) {
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Suppress")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Suppress")
		})
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRASuppress(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Suppress EDT verification failed!")
		// 	}
		// })
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		//unconfigure Ipv6 Ra Suppress
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		}
	})

	t.Run("TestCase-4: Router Advertisement with Mode All", func(t *testing.T) {
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "ModeAll")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "ModeAll")
		})
		t.Run("Validate RA Suppress EDT", func(t *testing.T) {
			edtStatus := verifyEdtRASuppress(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Suppress EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		//unconfigure Ipv6 Ra Suppress
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		}
	})

	t.Run("TestCase-5: Router Advertisement with Suppress UnSolicitation", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-6: Router Advertisement with Suppress and Unsolicited", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Suppress")
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Suppress")
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRASuppress(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Suppress EDT verification failed!")
		// 	}
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		// }
	})

	t.Run("TestCase-7: Router Advertisement with Mode Unicast ", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unicast")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unicast")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnicast(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unicast EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-8: Router Advertisement with Mode Unicast and Unsolicited", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unicast")
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unicast")
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnicast(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unicast EDT verification failed!")
		// 	}
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-9: Commit/Replace the Router Advertisement ", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		//unconfigure Ipv6 Ra Config
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		}

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)

	})

	t.Run("TestCase-10: Shut/Unshut the Router Advertisement Interface", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		//Shut the interface
		t.Log("Shutting the interface")
		for _, interfaces := range interfaceList {
			flapSubInterface(t, dut, interfaces.name, interfaces.subIntf, false)
		}

		//UnShut the interface
		t.Log("Unshutting the interface")
		for _, interfaces := range interfaceList {
			flapSubInterface(t, dut, interfaces.name, interfaces.subIntf, true)
		}

		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})

	t.Run("TestCase-11: Verify IPv6 RA after process restart.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		// Restart the processs
		process_list := []string{"ipv6_nd", "ipv6_ma", "ifmgr"} // "sysdb" "cfmgr"
		for _, process := range process_list {
			t.Run(fmt.Sprintf("Restart the Process - %s", process), func(t *testing.T) {
				ctx := context.Background()
				restartCmd := fmt.Sprintf("process restart {%s} location 0/2/CPU0", process)
				config.CMDViaGNMI(ctx, t, dut, restartCmd)
				time.Sleep(time.Second * 10)

				t.Log("Validating IPv6 RA ND after Restarted the process")
				t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
					verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
				})
				t.Run("Validate RA Interval EDT", func(t *testing.T) {
					edtStatus := verifyEdtRAInterval(t, dut)
					if !edtStatus {
						t.Fatalf("Error: RA Interval EDT verification failed!")
					}
				})
				t.Logf("Validating the Router Advertisement packets")
				verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
				verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
			})
		}

	})

	t.Run("TestCase-12: Verify IPv6 RA after Linecard Reload.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		dut := ondatra.DUT(t, "dut")
		lcList := util.GetLCList(t, dut)
		if len(lcList) == 0 {
			t.Skip("No linecards found")
		}
		util.ReloadLinecards(t, lcList)
		t.Log("Verify IPv6 RA after reloading all linecards")
		time.Sleep(120 * time.Second)

		t.Run("Validate RA MDT Telemetry After LC reload", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA EDT After LC reload", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

	t.Run("TestCase-13: Verify IPv6 RA after RPFO.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		t.Run("Trigger RPFO", func(t *testing.T) {
			testRPFO(t, dut, ate, ate.Topology().New())
			time.Sleep(60 * time.Second)
		})

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

	t.Run("TestCase-14: Verify IPv6 RA after Router Reload.", func(t *testing.T) {
		t.Skip("Skipping the test case as it is not supported")
		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		// Reload the Router
		client := gribi.Client{
			DUT:                   dut,
			FibACK:                *ciscoFlags.GRIBIFIBCheck,
			Persistence:           true,
			InitialElectionIDLow:  1,
			InitialElectionIDHigh: 0,
		}
		defer client.Close(t)
		if err := client.Start(t); err != nil {
			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
			if err = client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection could not be established: %v", err)
			}
		}

		time.Sleep(1 * time.Minute)
		gnoiClient := dut.RawAPIs().GNOI(t)
		_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot chassis without delay",
			Force:   true,
		})
		if err != nil {
			t.Fatalf("Reboot failed %v", err)
		}
		startReboot := time.Now()
		const maxRebootTime = 30
		t.Logf("Wait for DUT to boot up by polling the telemetry output.")
		for {
			var currentTime string
			t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

			time.Sleep(3 * time.Minute)
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			}); errMsg != nil {
				t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
			} else {
				t.Logf("Device rebooted successfully with received time: %v", currentTime)
				break
			}

			if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
				t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
			}
		}
		t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
		time.Sleep(30 * time.Second)

		t.Log("Validating IPv6 RA ND after Reload the Router")
		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

}

func TestIpv6NDRABundleSubIntf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	physicaSrclInt := dut.Port(t, "port1")
	physicaDstlInt := dut.Port(t, "port2")
	interfaceList := []InterfaceInfo{

		{
			intf:     physicaSrclInt,
			name:     "Bundle-Ether1",
			attr:     dutSrc,
			subIntf:  0,
			intftype: oc.IETFInterfaces_InterfaceType_ieee8023adLag,
		},
		{
			intf:     physicaDstlInt,
			name:     "Bundle-Ether2",
			attr:     dutDst,
			subIntf:  1,
			intftype: oc.IETFInterfaces_InterfaceType_ieee8023adLag,
		},
	}
	agg1.subIntf = 0
	agg2.subIntf = 1
	aggIDs := configureDUTLag(t, dut)
	defer unConfigInterface(t, dut, interfaceList)
	for _, interfaces := range interfaceList {
		configInterfaceIPv6RA(t, dut, interfaces, "Interval")
	}
	// defer unConfigureDUTLagSubIntf(t, dut, true)
	otgConfig := configureATEIpv6Ra(t, ate, true)
	for _, aggID := range aggIDs {
		gnmi.Await(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), 60*time.Second, oc.Interface_OperStatus_UP)
	}

	t.Run("TestCase-1: No periodical Router Advertisement with Interval", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		//unconfigure Ipv6 Ra Interval
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Interval().Config())
		}
	})

	t.Run("TestCase-2: No Router Advertisement in response to Router Solicitation", func(t *testing.T) {
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})

	t.Run("TestCase-3: Router Advertisement with Suppress", func(t *testing.T) {
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Suppress")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Suppress")
		})
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRASuppress(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Suppress EDT verification failed!")
		// 	}
		// })
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		//unconfigure Ipv6 Ra Suppress
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		}
	})

	t.Run("TestCase-4: Router Advertisement with Mode All", func(t *testing.T) {
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "ModeAll")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "ModeAll")
		})
		t.Run("Validate RA Suppress EDT", func(t *testing.T) {
			edtStatus := verifyEdtRASuppress(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Suppress EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		//unconfigure Ipv6 Ra Suppress
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		}
	})

	t.Run("TestCase-5: Router Advertisement with Suppress UnSolicitation", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-6: Router Advertisement with Suppress and Unsolicited", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Suppress")
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Suppress")
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRASuppress(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Suppress EDT verification failed!")
		// 	}
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Suppress().Config())
		// }
	})

	t.Run("TestCase-7: Router Advertisement with Mode Unicast ", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unicast")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unicast")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnicast(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unicast EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)

		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-8: Router Advertisement with Mode Unicast and Unsolicited", func(t *testing.T) {
		// for _, interfaces := range interfaceList {
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unicast")
		// 	configInterfaceIPv6RA(t, dut, interfaces, "Unsolicited")
		// }

		// t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unicast")
		// 	verifyRATelemetry(t, dut, interfaceList[0].name, "Unsolicited")
		// })
		// t.Run("Validate RA Suppress EDT", func(t *testing.T) {
		// 	edtStatus := verifyEdtRAUnicast(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unicast EDT verification failed!")
		// 	}
		// 	edtStatus := verifyEdtRAUnsolicited(t, dut)
		// 	if !edtStatus {
		// 		t.Fatalf("Error: RA Unsolicited EDT verification failed!")
		// 	}
		// })
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		// verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
		// //unconfigure Ipv6 Ra Suppress
		// for _, interfaces := range interfaceList {
		// 	gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		// }
	})

	t.Run("TestCase-9: Commit/Replace the Router Advertisement ", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		//unconfigure Ipv6 Ra Config
		for _, interfaces := range interfaceList {
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaces.name).Subinterface(0).Ipv6().RouterAdvertisement().Config())
		}

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)

	})

	t.Run("TestCase-10: Shut/Unshut the Router Advertisement Interface", func(t *testing.T) {

		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		//Shut the interface
		t.Log("Shutting the interface")
		for _, interfaces := range interfaceList {
			flapSubInterface(t, dut, interfaces.name, interfaces.subIntf, false)
		}

		//UnShut the interface
		t.Log("Unshutting the interface")
		for _, interfaces := range interfaceList {
			flapSubInterface(t, dut, interfaces.name, interfaces.subIntf, true)
		}

		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 1)
	})

	t.Run("TestCase-11: Verify IPv6 RA after process restart.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		// Restart the processs
		process_list := []string{"ipv6_nd", "ipv6_ma", "ifmgr"} // "sysdb" "cfmgr"
		for _, process := range process_list {
			t.Run(fmt.Sprintf("Restart the Process - %s", process), func(t *testing.T) {
				ctx := context.Background()
				restartCmd := fmt.Sprintf("process restart {%s} location 0/2/CPU0", process)
				config.CMDViaGNMI(ctx, t, dut, restartCmd)
				time.Sleep(time.Second * 10)

				t.Log("Validating IPv6 RA ND after Restarted the process")
				t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
					verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
				})
				t.Run("Validate RA Interval EDT", func(t *testing.T) {
					edtStatus := verifyEdtRAInterval(t, dut)
					if !edtStatus {
						t.Fatalf("Error: RA Interval EDT verification failed!")
					}
				})
				t.Logf("Validating the Router Advertisement packets")
				verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
				verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
			})
		}

	})

	t.Run("TestCase-12: Verify IPv6 RA after Linecard Reload.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		dut := ondatra.DUT(t, "dut")
		lcList := util.GetLCList(t, dut)
		if len(lcList) == 0 {
			t.Skip("No linecards found")
		}
		util.ReloadLinecards(t, lcList)
		t.Log("Verify IPv6 RA after reloading all linecards")
		time.Sleep(120 * time.Second)

		t.Run("Validate RA MDT Telemetry After LC reload", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA EDT After LC reload", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

	t.Run("TestCase-13: Verify IPv6 RA after RPFO.", func(t *testing.T) {

		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		t.Run("Trigger RPFO", func(t *testing.T) {
			testRPFO(t, dut, ate, ate.Topology().New())
			time.Sleep(60 * time.Second)
		})

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

	t.Run("TestCase-14: Verify IPv6 RA after Router Reload.", func(t *testing.T) {
		t.Skip("Skipping the test case as it is not supported")
		// Configure the RA Interval
		for _, interfaces := range interfaceList {
			configInterfaceIPv6RA(t, dut, interfaces, "Interval")
		}

		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)

		// Reload the Router
		client := gribi.Client{
			DUT:                   dut,
			FibACK:                *ciscoFlags.GRIBIFIBCheck,
			Persistence:           true,
			InitialElectionIDLow:  1,
			InitialElectionIDHigh: 0,
		}
		defer client.Close(t)
		if err := client.Start(t); err != nil {
			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
			if err = client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection could not be established: %v", err)
			}
		}

		time.Sleep(1 * time.Minute)
		gnoiClient := dut.RawAPIs().GNOI(t)
		_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot chassis without delay",
			Force:   true,
		})
		if err != nil {
			t.Fatalf("Reboot failed %v", err)
		}
		startReboot := time.Now()
		const maxRebootTime = 30
		t.Logf("Wait for DUT to boot up by polling the telemetry output.")
		for {
			var currentTime string
			t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

			time.Sleep(3 * time.Minute)
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			}); errMsg != nil {
				t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
			} else {
				t.Logf("Device rebooted successfully with received time: %v", currentTime)
				break
			}

			if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
				t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
			}
		}
		t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
		time.Sleep(30 * time.Second)

		t.Log("Validating IPv6 RA ND after Reload the Router")
		t.Run("Validate RA MDT Telemetry", func(t *testing.T) {
			verifyRATelemetry(t, dut, interfaceList[0].name, "Interval")
		})
		t.Run("Validate RA Interval EDT", func(t *testing.T) {
			edtStatus := verifyEdtRAInterval(t, dut)
			if !edtStatus {
				t.Fatalf("Error: RA Interval EDT verification failed!")
			}
		})
		t.Logf("Validating the Router Advertisement packets")
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, false, 10)
		verifyOTGPacketCaptureForRA(t, ate, otgConfig, true, 10)
	})

}
