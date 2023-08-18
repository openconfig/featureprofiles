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

package isis_change_lsp_lifetime_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	plenIPv4       = 30
	plenIPv6       = 126
	isisInstance   = "DEFAULT"
	dutAreaAddress = "49.0001"
	ateAreaAddress = "49.0002"
	dutSysID       = "1920.0000.2001"
	ateSystemID    = "640000000001"
	lspLifetime    = 500
	v4Route1       = "203.0.113.0"
	v6Route1       = "2001:db8::203:0:113:0"
	v4Route        = "203.0.113.0/30"
	v6Route        = "2001:db8::203:0:113:0/126"
	v4IP           = "203.0.113.1"
	v6IP           = "2001:db8::203:0:113:1"
	v4NetName      = "isisv4Net"
	v6NetName      = "isisv6Net"
	v4FlowName     = "v4Flow"
	v6FlowName     = "v6Flow"
)

var (
	dutPort1Attr = attrs.Attributes{
		Desc:    "DUT to ATE port1 ",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1attr = attrs.Attributes{
		Name:    "ATE to DUT port1 ",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2Attr = attrs.Attributes{
		Desc:    "DUT to ATE port2 ",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2attr = attrs.Attributes{
		Name:    "ATE to DUT port2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T) {
	t.Helper()
	dc := gnmi.OC()
	dut := ondatra.DUT(t, "dut")

	i1 := dutPort1Attr.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	t.Log("Pushing interface config on DUT port1")
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2Attr.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	t.Log("Pushing interface config on DUT port2")
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
}

// configureISIS configures isis on DUT.
func configureISIS(t *testing.T, dut *ondatra.DUTDevice, intfName string, dutAreaAddress, dutSysID string) {
	t.Helper()
	d := &oc.Root{}
	configPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalIsis := isis.GetOrCreateGlobal()

	// Global configs
	globalIsis.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalIsis.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalIsis.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalIsis.LevelCapability = oc.Isis_LevelType_LEVEL_2

	globalIsis.GetOrCreateTimers().LspLifetimeInterval = ygot.Uint16(lspLifetime)

	// Interface configs
	intf := isis.GetOrCreateInterface(intfName)
	intf.Enabled = ygot.Bool(true)
	intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intf.InterfaceId = &intfName

	t.Log("Pushing isis config on DUT")
	gnmi.Replace(t, dut, configPath.Config(), prot)
}

// configureOTG configures the interfaces and isis on OTG.
func configureOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {
	t.Helper()
	config := otg.NewConfig(t)
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	iDut1Dev := config.Devices().Add().SetName(atePort1attr.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1attr.Name + ".Eth").SetMac(atePort1attr.MAC)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1attr.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1attr.IPv4).SetGateway(dutPort1Attr.IPv4).SetPrefix(int32(atePort1attr.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1attr.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1attr.IPv6).SetGateway(dutPort1Attr.IPv6).SetPrefix(int32(atePort1attr.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(atePort2attr.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2attr.Name + ".Eth").SetMac(atePort2attr.MAC)
	iDut2Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2attr.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2attr.IPv4).SetGateway(dutPort2Attr.IPv4).SetPrefix(int32(atePort2attr.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2attr.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2attr.IPv6).SetGateway(dutPort2Attr.IPv6).SetPrefix(int32(atePort2attr.IPv6Len))

	iDut1Dev.Isis().SetSystemId(ateSystemID).SetName("devIsis")
	iDut1Dev.Isis().Basic().SetIpv4TeRouterId(atePort1attr.IPv4).SetEnableWideMetric(false)
	iDut1Dev.Isis().Advanced().SetAreaAddresses([]string{strings.Replace(ateAreaAddress, ".", "", -1)})

	iDut1Dev.Isis().Interfaces().
		Add().
		SetEthName(iDut1Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)

	// netv4 is a simulated network containing the ipv4 addresses specified by targetNetwork
	netv4 := iDut1Dev.Isis().V4Routes().Add().SetName(v4NetName).SetLinkMetric(10)
	netv4.Addresses().Add().SetAddress(v4Route1).SetPrefix(plenIPv4)

	// netv6 is a simulated network containing the ipv6 addresses specified by targetNetwork
	netv6 := iDut1Dev.Isis().V6Routes().Add().SetName(v6NetName).SetLinkMetric(10)
	netv6.Addresses().Add().SetAddress(v6Route1).SetPrefix(plenIPv6)

	t.Log("Configuring v4 traffic flow ")
	v4Flow := config.Flows().Add().SetName(v4FlowName)
	v4Flow.Metrics().SetEnable(true)
	v4Flow.TxRx().Device().
		SetTxNames([]string{iDut2Ipv4.Name()}).
		SetRxNames([]string{v4NetName})
	v4Flow.Size().SetFixed(512)
	v4Flow.Rate().SetPps(100)
	v4Flow.Duration().SetChoice("continuous")
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort2attr.MAC)
	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort2attr.IPv4)
	v4.Dst().Increment().SetStart(v4IP).SetCount(1)

	t.Log("Configuring v6 traffic flow ")
	v6Flow := config.Flows().Add().SetName(v6FlowName)
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{iDut2Ipv6.Name()}).
		SetRxNames([]string{v6NetName})
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)
	v6Flow.Duration().SetChoice("continuous")
	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(atePort2attr.MAC)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(atePort2attr.IPv6)
	v6.Dst().Increment().SetStart(v6IP).SetCount(1)

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)

	otgutils.WaitForARP(t, otg, config, "IPv4")
	otgutils.WaitForARP(t, otg, config, "IPv6")

	return config
}

// TestISISChangeLSPLifetime verifies isis lsp telemetry paramters with configured lsp lifetime.
func TestISISChangeLSPLifetime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	intfName := dut.Port(t, "port1").Name()

	// Configure interface on the DUT.
	configureDUT(t)

	// Configure network Instance type on DUT.
	dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	t.Log("Pushing network Instance type config on DUT")
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	// Configure isis on DUT.
	configureISIS(t, dut, intfName, dutAreaAddress, dutSysID)

	// Configure interface,isis and traffic on ATE.
	otg := ate.OTG()
	otgConfig := configureOTG(t, otg)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		intfName = intfName + ".0"
	}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()

	t.Run("Isis telemetry", func(t *testing.T) {

		adjacencyPath := statePath.Interface(intfName).Level(2).AdjacencyAny().AdjacencyState().State()

		_, ok := gnmi.WatchAll(t, dut, adjacencyPath, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			return present && state == oc.Isis_IsisInterfaceAdjState_UP
		}).Await(t)
		if !ok {
			t.Fatalf("No isis adjacency reported on interface %v", intfName)
		}
		// Getting neighbors sysid.
		sysid := gnmi.GetAll(t, dut, statePath.Interface(intfName).Level(2).AdjacencyAny().SystemId().State())
		ateSysID := sysid[0]
		ateLspID := ateSysID + ".00-00"
		dutLspID := dutSysID + ".00-00"

		t.Run("Adjacency state checks", func(t *testing.T) {
			adjPath := statePath.Interface(intfName).Level(2).Adjacency(ateSysID)
			if got := gnmi.Get(t, dut, adjPath.Nlpid().State()); !cmp.Equal(got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}) {
				t.Errorf("FAIL- Expected address families not found, got %s, want %s", got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6})
			}
		})
		t.Run("Lsp checks", func(t *testing.T) {
			if got := gnmi.Get(t, dut, statePath.Global().Timers().LspLifetimeInterval().State()); got != lspLifetime {
				t.Errorf("FAIL- Expected lsp lifetime interval not found, want %d, got %d", lspLifetime, got)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).LspId().State()); got != dutLspID {
				t.Errorf("FAIL- Expected DUT lsp id not found, want %s, got %s", dutLspID, got)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).LspId().State()); got != ateLspID {
				t.Errorf("FAIL- Expected ATE lsp not found, want %s, got %s", ateLspID, got)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).PacketCounters().Lsp().Sent().State()); got == 0 {
				t.Errorf("FAIL- Expected lsp count is greater than 0, got %d", got)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).RemainingLifetime().State()); got >= lspLifetime {
				t.Errorf("FAIL- Expected remaining lifetime not found, got %d,want less then %d", got, lspLifetime)
			}
		})
		t.Run("Route checks", func(t *testing.T) {
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().Prefix(v4Route).Prefix().State()); got != v4Route {
				t.Errorf("FAIL- Expected ate v4 route not found, got %v, want %v", got, v4Route)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).Ipv6Reachability().Prefix(v6Route).Prefix().State()); got != v6Route {
				t.Errorf("FAIL- Expected v6 route not found in isis, got %v, want %v", got, v6Route)
			}
			if got := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(v4Route).State()).GetPrefix(); got != v4Route {
				t.Errorf("FAIL- Expected v4 route not found in aft, got %v, want %v", got, v4Route)
			}
			if got := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv6Entry(v6Route).State()).GetPrefix(); got != v6Route {
				t.Errorf("FAIL- Expected v6 route not found in aft, got %v, want %v", got, v6Route)
			}
		})
		seqNum1 := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).SequenceNumber().State())
		checksum1 := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).Checksum().State())
		lspSent1 := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).PacketCounters().Lsp().Sent().State())

		// Check the lsp's checksum/seq number/remaining lifetime once lsp refreshes periodically.
		t.Run("Lsp lifetime checks", func(t *testing.T) {
			_, ok := gnmi.Watch(t, dut, statePath.Interface(intfName).Level(2).PacketCounters().Lsp().Sent().State(), time.Minute*4, func(val *ygnmi.Value[uint32]) bool {
				lspSent2, present := val.Val()

				if lspSent2 > lspSent1 {
					time.Sleep(time.Second * 5)
					if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).SequenceNumber().State()); got <= seqNum1 {
						t.Errorf("FAIL- Sequence number of new lsp should increment, got %d, want greater than %d", got, seqNum1)
					}
					if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).Checksum().State()); got == checksum1 {
						t.Errorf("FAIL- Checksum of new lsp should be different from %d, got %d", checksum1, got)
					}
					if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).RemainingLifetime().State()); got >= lspLifetime || got < lspLifetime-50 {
						t.Errorf("FAIL- Expected remaining lifetime not found, got %d,expected b/w %d and %d", got, lspLifetime, lspLifetime-50)
					}
				}
				return present && lspSent2 > lspSent1
			}).Await(t)
			if !ok {
				t.Error("FAIL- Isis lsp is not refreshing periodically")
			}
		})
		t.Run("Traffic checks", func(t *testing.T) {
			t.Logf("Starting traffic")
			otg.StartTraffic(t)
			time.Sleep(time.Second * 15)
			t.Logf("Stop traffic")
			otg.StopTraffic(t)

			otgutils.LogFlowMetrics(t, otg, otgConfig)
			otgutils.LogPortMetrics(t, otg, otgConfig)

			for _, flow := range []string{v4FlowName, v6FlowName} {
				t.Log("Checking flow telemetry...")
				recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).State())
				txPackets := recvMetric.GetCounters().GetOutPkts()
				rxPackets := recvMetric.GetCounters().GetInPkts()
				lostPackets := txPackets - rxPackets
				lossPct := lostPackets * 100 / txPackets

				if lossPct > 1 {
					t.Errorf("FAIL- Got %v%% packet loss for %s ; expected < 1%%", lossPct, flow)
				}
			}
		})
	})
}
