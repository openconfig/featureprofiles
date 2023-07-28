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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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
	lspLifetime    = 500
	v4Route        = "203.0.113.0/30"
	v6Route        = "2001:db8::203:0:113:0/126"
	v4IP           = "203.0.113.1"
	v6IP           = "2001:db8::203:0:113:1"
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

	if !deviations.ISISprotocolEnabledNotRequired(dut) {
		prot.Enabled = ygot.Bool(true)
	}
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

// configureATE configures the interfaces and isis on ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	t.Helper()
	topo := ate.Topology().New()
	port1 := ate.Port(t, "port1")
	port2 := ate.Port(t, "port2")

	i1Dut := topo.AddInterface(atePort1attr.Name).WithPort(port1)
	i1Dut.IPv4().WithAddress(atePort1attr.IPv4CIDR()).WithDefaultGateway(dutPort1Attr.IPv4)
	i1Dut.IPv6().WithAddress(atePort1attr.IPv6CIDR()).WithDefaultGateway(dutPort1Attr.IPv6)

	i2Dut := topo.AddInterface(atePort2attr.Name).WithPort(port2)
	i2Dut.IPv4().WithAddress(atePort2attr.IPv4CIDR()).WithDefaultGateway(dutPort2Attr.IPv4)
	i2Dut.IPv6().WithAddress(atePort2attr.IPv6CIDR()).WithDefaultGateway(dutPort2Attr.IPv6)

	isisDut := i1Dut.ISIS()
	isisDut.
		WithAreaID(ateAreaAddress).
		WithTERouterID(atePort1attr.IPv4).
		WithNetworkTypePointToPoint().
		WithLevelL2()

	netGrp := i1Dut.AddNetwork(fmt.Sprintf("isis-%d", 1))
	netGrp.IPv4().WithAddress(v4Route)
	netGrp.ISIS().WithActive(true)
	netGrp.IPv6().WithAddress(v6Route)
	netGrp.ISIS().WithActive(true)

	t.Log("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)
	return topo
}

// createFlow returns v4 and v6 flow from atePort2 to atePort1
func createFlow(t *testing.T, ate *ondatra.ATEDevice, ateTopo *ondatra.ATETopology) []*ondatra.Flow {
	t.Helper()
	srcIntf := ateTopo.Interfaces()[atePort2attr.Name]
	dstIntf := ateTopo.Interfaces()[atePort1attr.Name]

	t.Log("Configuring v4 traffic flow ")
	v4Header := ondatra.NewIPv4Header()
	v4Header.DstAddressRange().WithMin(v4IP).WithCount(1)

	v4Flow := ate.Traffic().NewFlow("v4Flow").
		WithSrcEndpoints(srcIntf).WithDstEndpoints(dstIntf).
		WithHeaders(ondatra.NewEthernetHeader(), v4Header)

	t.Log("Configuring v6 traffic flow ")
	v6Header := ondatra.NewIPv6Header()
	v6Header.DstAddressRange().WithMin(v6IP).WithCount(1)

	v6Flow := ate.Traffic().NewFlow("v6Flow").
		WithSrcEndpoints(srcIntf).WithDstEndpoints(dstIntf).
		WithHeaders(ondatra.NewEthernetHeader(), v6Header)

	return []*ondatra.Flow{v4Flow, v6Flow}
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
	ateTopo := configureATE(t, ate)
	flows := createFlow(t, ate, ateTopo)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		intfName = intfName + ".0"
	}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()

	t.Run("Isis telemetry", func(t *testing.T) {
		t.Run("Verifying adjacency", func(t *testing.T) {
			adjacencyPath := statePath.Interface(intfName).Level(2).AdjacencyAny().AdjacencyState().State()

			_, ok := gnmi.WatchAll(t, dut, adjacencyPath, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
				state, present := val.Val()
				return present && state == oc.Isis_IsisInterfaceAdjState_UP
			}).Await(t)
			if !ok {
				t.Fatalf("No isis adjacency reported on interface %v", intfName)
			}
		})
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
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV4_INTERNAL_REACHABILITY).Ipv4InternalReachability().Prefix(v4Route).Prefix().State()); got != v4Route {
				t.Errorf("FAIL- Expected v4 route not found in isis, got %v, want %v", got, v4Route)
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
			ate.Traffic().Start(t, flows...)
			time.Sleep(time.Second * 15)
			ate.Traffic().Stop(t)

			for _, flow := range flows {
				t.Log("Checking flow telemetry...")
				telem := gnmi.OC()
				loss := gnmi.Get(t, ate, telem.Flow(flow.Name()).LossPct().State())

				if loss > 1 {
					t.Errorf("FAIL- Got %v%% packet loss for %s ; expected < 1%%", loss, flow.Name())
				}
			}
		})
	})
}
