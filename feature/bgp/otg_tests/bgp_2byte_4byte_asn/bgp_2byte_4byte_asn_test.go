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

package bgp_2byte_4byte_asn_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

const (
	connInternal = "INTERNAL"
	connExternal = "EXTERNAL"
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:11:01:00:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: 30,
		IPv6Len: 126,
	}
)

type bgpNbr struct {
	globalAS, localAS, peerAS uint32
	peerIP                    string
	isV4                      bool
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBgpSession(t *testing.T) {
	t.Log("Clear ATE configuration")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	ate.OTG().PushConfig(t, top)

	t.Log("Configure DUT interface")
	dut := ondatra.DUT(t, "dut")
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}

	t.Log("Configure Network Instance")
	dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	cases := []struct {
		name    string
		nbr     *bgpNbr
		dutConf *oc.NetworkInstance_Protocol
		ateConf gosnappi.Config
	}{
		{
			name:    "Establish eBGP connection between ATE (2-byte) - DUT (4-byte < 65535) for ipv4 peers",
			nbr:     &bgpNbr{globalAS: 300, localAS: 100, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{globalAS: 300, localAS: 100, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{globalAS: 300, localAS: 200, peerIP: dutSrc.IPv4, peerAS: 100, isV4: true}, connExternal, 2),
		}, {
			name:    "Establish eBGP connection between ATE (2-byte) - DUT (4-byte < 65535) for ipv6 peers",
			nbr:     &bgpNbr{globalAS: 300, localAS: 100, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{globalAS: 300, localAS: 100, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{globalAS: 300, localAS: 200, peerIP: dutSrc.IPv6, peerAS: 100, isV4: false}, connExternal, 2),
		}, {
			name:    "Establish eBGP connection between ATE (4-byte) - DUT (4-byte) for ipv4 peers",
			nbr:     &bgpNbr{globalAS: 300, localAS: 70000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{globalAS: 300, localAS: 70000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{globalAS: 300, localAS: 80000, peerIP: dutSrc.IPv4, peerAS: 70000, isV4: true}, connExternal, 4),
		}, {
			name:    "Establish eBGP connection between ATE (4-byte) - DUT (4-byte) for ipv6 peers",
			nbr:     &bgpNbr{globalAS: 300, localAS: 70000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{globalAS: 300, localAS: 70000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{globalAS: 300, localAS: 80000, peerIP: dutSrc.IPv6, peerAS: 70000, isV4: false}, connExternal, 4),
		}, {
			name:    "Establish iBGP connection between ATE (2-byte) - DUT (4-byte < 65535) for ipv4 peers",
			nbr:     &bgpNbr{globalAS: 300, localAS: 200, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{globalAS: 300, localAS: 200, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{globalAS: 300, localAS: 200, peerIP: dutSrc.IPv4, peerAS: 200, isV4: true}, connInternal, 2),
		}, {
			name:    "Establish iBGP connection between ATE (4-byte) - DUT (4-byte < 65535) for ipv6 peers",
			nbr:     &bgpNbr{globalAS: 300, localAS: 200, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{globalAS: 300, localAS: 200, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{globalAS: 300, localAS: 200, peerIP: dutSrc.IPv6, peerAS: 200, isV4: false}, connInternal, 4),
		}, {
			name:    "Establish iBGP connection between ATE (4-byte) - DUT (4-byte) for ipv4 peers",
			nbr:     &bgpNbr{globalAS: 300, localAS: 80000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{globalAS: 300, localAS: 80000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{globalAS: 300, localAS: 80000, peerIP: dutSrc.IPv4, peerAS: 80000, isV4: true}, connInternal, 4),
		}, {
			name:    "Establish iBGP connection between ATE (4-byte) - DUT (4-byte) for ipv6 peers",
			nbr:     &bgpNbr{globalAS: 300, localAS: 80000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{globalAS: 300, localAS: 80000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{globalAS: 300, localAS: 80000, peerIP: dutSrc.IPv6, peerAS: 80000, isV4: false}, connInternal, 4),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ate := ondatra.ATE(t, "ate")
			t.Log("Clear BGP Configs on DUT")
			bgpClearConfig(t, dut)

			t.Log("Configure BGP on DUT")
			gnmi.Replace(t, dut, dutConfPath.Config(), tc.dutConf)

			fptest.LogQuery(t, "DUT BGP Config ", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
			t.Log("Configure BGP on ATE")
			ate.OTG().PushConfig(t, tc.ateConf)
			ate.OTG().StartProtocols(t)

			t.Log("Verify BGP session state : ESTABLISHED")
			nbrPath := statePath.Neighbor(tc.nbr.peerIP)
			gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*120, oc.Bgp_Neighbor_SessionState_ESTABLISHED)

			t.Log("Verify BGP AS numbers")
			verifyPeer(t, tc.nbr, dut)

			t.Log("Clear BGP Configs on ATE")
			ate.OTG().StopProtocols(t)
		})
	}
}

// bgpClearConfig removes all BGP configuration from the DUT.
func bgpClearConfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	resetBatch := &gnmi.SetBatch{}
	gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())

	if deviations.NetworkInstanceTableDeletionRequired(dut) {
		tablePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableAny()
		for _, table := range gnmi.LookupAll(t, dut, tablePath.Config()) {
			if val, ok := table.Val(); ok {
				if val.GetProtocol() == oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP {
					gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Table(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, val.GetAddressFamily()).Config())
				}
			}
		}
	}
	resetBatch.Set(t, dut)
}

func verifyPeer(t *testing.T, nbr *bgpNbr, dut *ondatra.DUTDevice) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(nbr.peerIP)
	glblPath := statePath.Global()

	// Check BGP peerAS from telemetry.
	peerAS := gnmi.Get(t, dut, nbrPath.State()).GetPeerAs()
	if peerAS != nbr.peerAS {
		t.Errorf("BGP peerAs: got %v, want %v", peerAS, nbr.peerAS)
	}

	// Check BGP localAS from telemetry.
	localAS := gnmi.Get(t, dut, nbrPath.State()).GetLocalAs()
	if localAS != nbr.localAS {
		t.Errorf("BGP localAS: got %v, want %v", localAS, nbr.localAS)
	}

	// Check BGP globalAS from telemetry.
	globalAS := gnmi.Get(t, dut, glblPath.State()).GetAs()
	if globalAS != nbr.globalAS {
		t.Errorf("BGP globalAS: got %v, want %v", globalAS, nbr.globalAS)
	}
}

func configureATE(t *testing.T, ateParams *bgpNbr, connectionType string, asWidth int) gosnappi.Config {
	t.Helper()
	t.Log("Configure ATE interface")
	ate := ondatra.ATE(t, "ate")
	port1 := ate.Port(t, "port1")
	topo := gosnappi.NewConfig()

	topo.Ports().Add().SetName(port1.ID())
	srcDev := topo.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetPortName(port1.ID())
	srcIpv4 := srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	srcIpv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	srcIpv6 := srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	srcIpv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	srcBgp := srcDev.Bgp().SetRouterId(srcIpv4.Address())
	if ateParams.isV4 {
		srcBgpPeer := srcBgp.Ipv4Interfaces().Add().SetIpv4Name(srcIpv4.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP4.peer")
		srcBgpPeer.SetPeerAddress(ateParams.peerIP).SetAsNumber(ateParams.localAS)
		if connectionType == connInternal {
			srcBgpPeer.SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
		} else {
			srcBgpPeer.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
		}
		if asWidth == 2 {
			srcBgpPeer.SetAsNumberWidth(gosnappi.BgpV4PeerAsNumberWidth.TWO)
		}
	} else {
		srcBgpPeer := srcBgp.Ipv6Interfaces().Add().SetIpv6Name(srcIpv6.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP6.peer")
		srcBgpPeer.SetPeerAddress(ateParams.peerIP).SetAsNumber(ateParams.localAS)
		if connectionType == connInternal {
			srcBgpPeer.SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
		} else {
			srcBgpPeer.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
		}
		if asWidth == 2 {
			srcBgpPeer.SetAsNumberWidth(gosnappi.BgpV6PeerAsNumberWidth.TWO)
		}
	}

	return topo
}

func createBgpNeighbor(nbr *bgpNbr, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(nbr.globalAS)
	global.RouterId = ygot.String(dutSrc.IPv4)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup("ATE")
	pg.PeerAs = ygot.Uint32(nbr.peerAS)
	pg.PeerGroupName = ygot.String("ATE")

	neighbor := bgp.GetOrCreateNeighbor(nbr.peerIP)
	neighbor.PeerAs = ygot.Uint32(nbr.peerAS)
	neighbor.Enabled = ygot.Bool(true)
	neighbor.PeerGroup = ygot.String("ATE")
	neighbor.LocalAs = ygot.Uint32(nbr.localAS)
	neighbor.GetOrCreateTimers().RestartTime = ygot.Uint16(75)

	if nbr.isV4 {
		afisafi := neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		afisafi.Enabled = ygot.Bool(true)
		neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)
	} else {
		afisafi6 := neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		afisafi6.Enabled = ygot.Bool(true)
		neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)
	}
	return niProto
}
