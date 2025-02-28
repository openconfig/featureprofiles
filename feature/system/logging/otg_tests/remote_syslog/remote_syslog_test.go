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

package remote_syslog_test

import (
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
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

const (
	pLen4         = 30
	pLen6         = 126
	trafficPps    = 1000
	totalPackets  = 30000
	flowV4        = "flowV4"
	flowV6        = "flowV6"
	v4Route       = "203.0.113.0"
	v6Route       = "2001:db8:128:128::0"
	v4RoutePrefix = uint32(24)
	v6RoutePrefix = uint32(64)
	lossTolerance = float64(1)
)

var (
	dutSrc = &attrs.Attributes{
		Desc:    "DUT Source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:DB8::192:0:2:1",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}

	ateSrc = &attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		Desc:    "ATE Source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:DB8::192:0:2:2",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}

	dutDst = &attrs.Attributes{
		Desc:    "DUT Destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:DB8::192:0:2:5",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}

	ateDst = &attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		Desc:    "ATE Destination",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:DB8::192:0:2:6",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}

	dutLoopback = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	lb string
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestRemoteSyslog(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	ate := ondatra.ATE(t, "ate")
	lb = netutil.LoopbackInterface(t, dut, 0)

	top := configureATE(t, ate)
	createFlow(t, top, true)
	createFlow(t, top, false)
	enableCapture(t, ate, top)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	testCases := []struct {
		name string
		vrf  string
	}{
		{
			name: "Default VRF",
			vrf:  deviations.DefaultNetworkInstance(dut),
		},
		{
			name: "Non-Default VRF",
			vrf:  "nondefaultvrfx",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.vrf != deviations.DefaultNetworkInstance(dut) {
				createAndAddInterfacesToVRF(t, dut, tc.vrf, []string{p1.Name(), p2.Name(), lb}, []uint32{0, 0, 0})
			}

			configureDUT(t, dut, &tc.vrf)
			configureDUTLoopback(t, dut)
			configureStaticRoute(t, dut, tc.vrf)
			configureSyslog(t, dut, tc.vrf)

			otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
			otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

			cs := startCapture(t, ate, top)

			sleepTime := time.Duration(totalPackets/trafficPps) + 2
			ate.OTG().StartTraffic(t)
			flipATEPort(t, dut, ate, top, "port2", false)
			time.Sleep(sleepTime * time.Second)
			ate.OTG().StopTraffic(t)

			stopCapture(t, ate, cs)

			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			otgutils.LogPortMetrics(t, ate.OTG(), top)

			processCapture(t, ate, top)

			t.Cleanup(func() {
				gnmi.Delete(t, dut, gnmi.OC().Interface(p1.Name()).Config())
				gnmi.Delete(t, dut, gnmi.OC().Interface(p2.Name()).Config())
				gnmi.Delete(t, dut, gnmi.OC().Interface(lb).Config())
				gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(tc.vrf).Config())
				flipATEPort(t, dut, ate, top, "port2", true)
			})
		})
	}
}

// configureDUT configures port1 and port2 on the DUT
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, vrfName *string) {
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	gnmi.Update(t, dut, gnmi.OC().Interface(dp1.Name()).Config(), dutSrc.NewOCInterface(dp1.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(dp2.Name()).Config(), dutDst.NewOCInterface(dp2.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
		fptest.SetPortSpeed(t, dp2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dp2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	if vrfName == nil {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	}
}

// configureATE configures port1 and port2 on the ATE
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	ateSrc.AddToOTG(top, ap1, dutSrc)
	ateDst.AddToOTG(top, ap2, dutDst)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	return top
}

// configureDUTLoopback configures the loopback interface on the DUT
func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	// lb = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(lb).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	foundV4 := false
	for _, ip := range ipv4Addrs {
		if v, ok := ip.Val(); ok {
			foundV4 = true
			dutLoopback.IPv4 = v.GetIp()
			break
		}
	}
	foundV6 := false
	for _, ip := range ipv6Addrs {
		if v, ok := ip.Val(); ok {
			foundV6 = true
			dutLoopback.IPv6 = v.GetIp()
			break
		}
	}
	if !foundV4 || !foundV6 {
		lo1 := dutLoopback.NewOCInterface(lb, dut)
		lo1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(lb).Config(), lo1)
	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, lb, deviations.DefaultNetworkInstance(dut), 0)
	}
}

func createAndAddInterfacesToVRF(t *testing.T, dut *ondatra.DUTDevice, vrfname string, intfNames []string, unit []uint32) {
	root := &oc.Root{}
	batchConfig := &gnmi.SetBatch{}
	for index, intfName := range intfNames {
		i := root.GetOrCreateInterface(intfName)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		i.Description = ygot.String(fmt.Sprintf("Port %s", strconv.Itoa(index+1)))
		if intfName == netutil.LoopbackInterface(t, dut, 0) {
			i.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
			i.Description = ygot.String(fmt.Sprintf("Port %s", intfName))
		}
		si := i.GetOrCreateSubinterface(unit[index])
		si.Enabled = ygot.Bool(true)
		gnmi.BatchUpdate(batchConfig, gnmi.OC().Interface(intfName).Config(), i)
	}

	mgmtNI := root.GetOrCreateNetworkInstance(vrfname)
	mgmtNI.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	for index, intfName := range intfNames {
		vi := mgmtNI.GetOrCreateInterface(intfName)
		vi.Interface = ygot.String(intfName)
		vi.Subinterface = ygot.Uint32(unit[index])
	}
	gnmi.BatchReplace(batchConfig, gnmi.OC().NetworkInstance(vrfname).Config(), mgmtNI)
	batchConfig.Set(t, dut)
	t.Logf("Added interface %v to VRF %s", intfNames, vrfname)
}

func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice, ni string) {
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: ni,
		Prefix:          v4Route + "/30",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(ateSrc.IPv4),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: ni,
		Prefix:          v6Route + "/126",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(ateSrc.IPv6),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV6, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)
}

func configureSyslog(t *testing.T, dut *ondatra.DUTDevice, ni string) {
	root := &oc.Root{}
	logging := root.GetOrCreateSystem().GetOrCreateLogging()

	remoteServer1 := logging.GetOrCreateRemoteServer("203.0.113.1")
	remoteServer1.SetNetworkInstance(ni)
	remoteServer1.SetSourceAddress(dutLoopback.IPv4)
	remoteServer1.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL7,
		oc.SystemLogging_SyslogSeverity_DEBUG,
	)

	remoteServer2 := logging.GetOrCreateRemoteServer("203.0.113.2")
	remoteServer2.SetNetworkInstance(ni)
	remoteServer2.SetSourceAddress(dutLoopback.IPv4)
	remoteServer2.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL7,
		oc.SystemLogging_SyslogSeverity_CRITICAL,
	)

	remoteServer3 := logging.GetOrCreateRemoteServer("2001:db8:128:128::1")
	remoteServer3.SetNetworkInstance(ni)
	remoteServer3.SetRemotePort(5140)
	remoteServer3.SetSourceAddress(dutLoopback.IPv6)
	remoteServer3.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL1,
		oc.SystemLogging_SyslogSeverity_DEBUG,
	)

	remoteServer4 := logging.GetOrCreateRemoteServer("2001:db8:128:128::2")
	remoteServer4.SetNetworkInstance(ni)
	remoteServer4.SetSourceAddress(dutLoopback.IPv6)
	remoteServer4.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL7,
		oc.SystemLogging_SyslogSeverity_CRITICAL,
	)

	gnmi.Replace(t, dut, gnmi.OC().System().Logging().Config(), logging)
}

// createFlow creates a traffic flow with fixed number of packets
func createFlow(t *testing.T, top gosnappi.Config, isV4 bool) {
	flowName := flowV4
	if !isV4 {
		flowName = flowV6
	}

	flow := top.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	if isV4 {
		flow.TxRx().Device().
			SetTxNames([]string{ateSrc.Name + ".IPv4"}).
			SetRxNames([]string{ateDst.Name + ".IPv4"})
	} else {
		flow.TxRx().Device().
			SetTxNames([]string{ateSrc.Name + ".IPv6"}).
			SetRxNames([]string{ateDst.Name + ".IPv6"})
	}

	flow.Duration().FixedPackets().SetPackets(totalPackets)
	flow.Size().SetFixed(1500)
	flow.Rate().SetPps(trafficPps)

	ethHdr := flow.Packet().Add().Ethernet()
	ethHdr.Src().SetValue(ateSrc.MAC)

	if isV4 {
		ipv4Hdr := flow.Packet().Add().Ipv4()
		ipv4Hdr.Src().SetValue(ateSrc.IPv4)
		ipv4Hdr.Dst().SetValue(ateDst.IPv4)
	} else {
		ipv6Hdr := flow.Packet().Add().Ipv6()
		ipv6Hdr.Src().SetValue(ateSrc.IPv6)
		ipv6Hdr.Dst().SetValue(ateDst.IPv6)
	}
}

func flipATEPort(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, port string, enable bool) {
	expStatus := oc.Interface_OperStatus_DOWN
	if enable {
		expStatus = oc.Interface_OperStatus_UP
	}
	if deviations.ATEPortLinkStateOperationsUnsupported(ate) {
		dutP := dut.Port(t, port)
		dc := gnmi.OC()
		i := &oc.Interface{}
		i.Enabled = ygot.Bool(enable)
		i.Name = ygot.String(dutP.Name())
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		gnmi.Update(t, dut, dc.Interface(dutP.Name()).Config(), i)
	} else {
		portStateAction := gosnappi.NewControlState()
		if enable {
			portStateAction.Port().Link().SetPortNames([]string{port}).SetState(gosnappi.StatePortLinkState.UP)
		} else {
			portStateAction.Port().Link().SetPortNames([]string{port}).SetState(gosnappi.StatePortLinkState.DOWN)
		}
		ate.OTG().SetControlState(t, portStateAction)
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(dut.Port(t, port).Name()).OperStatus().State(), 2*time.Minute, expStatus)
}

func enableCapture(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) {
	t.Helper()

	config.Captures().Clear()

	// enable packet capture on this port
	config.Captures().Add().SetName("sFlowpacketCapture").
		SetPortNames([]string{config.Ports().Items()[0].Name()}).
		SetFormat(gosnappi.CaptureFormat.PCAP)

	pb, _ := config.Marshal().ToProto()
	t.Log(pb.GetCaptures())
}

func startCapture(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) gosnappi.ControlState {
	t.Helper()

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	ate.OTG().SetControlState(t, cs)

	return cs
}

func stopCapture(t *testing.T, ate *ondatra.ATEDevice, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	ate.OTG().SetControlState(t, cs)
}

func processCapture(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) {
	bytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(config.Ports().Items()[0].Name()))
	time.Sleep(30 * time.Second)
	pcapFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	pcapFile.Close()
	validatePackets(t, pcapFile.Name())
}

func validatePackets(t *testing.T, filename string) {
	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	loopbackV4 := net.ParseIP(dutLoopback.IPv4)
	loopbackV6 := net.ParseIP(dutLoopback.IPv6)
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	foundV4 := false
	foundV6 := false
	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ipv4, _ := ipLayer.(*layers.IPv4)
			if ipv4.SrcIP.Equal(loopbackV4) {
				foundV4 = true
				t.Logf("tos %d, payload %d, content %d, length %d", ipv4.TOS, len(ipv4.Payload), len(ipv4.Contents), ipv4.Length)
			}
		} else if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
			ipv6, _ := ipLayer.(*layers.IPv6)
			if ipv6.SrcIP.Equal(loopbackV6) {
				foundV6 = true
				t.Logf("tos %d, payload %d, content %d, length %d", ipv6.TrafficClass, len(ipv6.Payload), len(ipv6.Contents), ipv6.Length)
			}
		}

	}

	if !foundV4 {
		t.Errorf("sflow packets not found: v4 %v, v6 %v", foundV4, foundV6)
	}
}
