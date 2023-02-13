// Copyright 2022 Google LLC
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

// Package setup is scoped only to be used for scripts in path
// feature/experimental/system/gnmi/benchmarking/ate_tests/
// Do not use elsewhere.
package setup

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	// ISISInstance is ISIS instance name.
	ISISInstance = "DEFAULT"
	// DUTAreaAddress is DUT ISIS area address.
	DUTAreaAddress = "49.0001"
	// DUTSysID is DUT ISIS system ID.
	DUTSysID = "1920.0000.2001"
	// PeerGrpName is BGP peer group name.
	PeerGrpName = "BGP-PEER-GROUP"
	// DUTAs is DUT AS.
	DUTAs = 64500
	// ATEAs is ATE AS.
	ATEAs = 64501

	ateAs2         = 64502
	dutStartIPAddr = "192.0.2.1"
	ateStartIPAddr = "192.0.2.2"
	plenIPv4       = 30
	authPassword   = "ISISAuthPassword"
)

var (
	// DUTIPList is DUT IP list.
	DUTIPList = make(map[string]net.IP)
	// ATEIPList is ATE IP list.
	ATEIPList = make(map[string]net.IP)
)

// BuildIPList builds list of ip addresses for the ports in binding file.
// (Both DUT and ATE ports).
func BuildIPList(dut *ondatra.DUTDevice) {
	var dutIPIndex, ipSubnet, ateIPIndex int = 1, 2, 2
	var endSubnetIndex = 253
	for _, dp := range dut.Ports() {
		dutNextIP := nextIP(net.ParseIP(dutStartIPAddr), dutIPIndex, ipSubnet)
		ateNextIP := nextIP(net.ParseIP(ateStartIPAddr), ateIPIndex, ipSubnet)
		DUTIPList[dp.ID()] = dutNextIP
		ATEIPList[dp.ID()] = ateNextIP

		// Increment DUT and ATE host ip index by 4.
		dutIPIndex = dutIPIndex + 4
		ateIPIndex = ateIPIndex + 4

		// Reset DUT and ATE IP indexes when it is greater than endSubnetIndex.
		if dutIPIndex > int(endSubnetIndex) {
			ipSubnet = ipSubnet + 1
			dutIPIndex = 1
			ateIPIndex = 2
		}
	}

}

// nextIP returns ip address based on hostIndex and subnetIndex provided.
func nextIP(ip net.IP, hostIndex int, subnetIndex int) net.IP {
	s := ip.String()
	sa := strings.Split(s, ".")
	sa[2] = strconv.Itoa(subnetIndex)
	sa[3] = strconv.Itoa(hostIndex)
	s = strings.Join(sa, ".")
	return net.ParseIP(s)
}

// BuildBenchmarkingConfig builds required configuration for DUT interfaces, ISIS and BGP.
func BuildBenchmarkingConfig(t *testing.T) *oc.Root {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}

	// Network instance and BGP configs.
	netInstance := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)

	bgp := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(DUTAs)
	global.RouterId = ygot.String(dutStartIPAddr)

	afi := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afi.Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup(PeerGrpName)
	pg.PeerAs = ygot.Uint32(ATEAs)
	pg.PeerGroupName = ygot.String(PeerGrpName)

	// ISIS configs.
	isis := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance).GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.AuthenticationCheck = ygot.Bool(true)
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", DUTAreaAddress, DUTSysID)}
	lspBit := globalISIS.GetOrCreateLspBit().GetOrCreateOverloadBit()
	lspBit.SetBit = ygot.Bool(false)
	isisTimers := globalISIS.GetOrCreateTimers()
	isisTimers.LspLifetimeInterval = ygot.Uint16(600)
	spfTimers := isisTimers.GetOrCreateSpf()
	spfTimers.SpfHoldInterval = ygot.Uint64(5000)
	spfTimers.SpfFirstInterval = ygot.Uint64(600)

	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC

	isisLevel2Auth := isisLevel2.GetOrCreateAuthentication()
	isisLevel2Auth.Enabled = ygot.Bool(true)
	isisLevel2Auth.AuthPassword = ygot.String(authPassword)
	isisLevel2Auth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
	isisLevel2Auth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY

	for _, dp := range dut.Ports() {
		// Interfaces config.
		i := d.GetOrCreateInterface(dp.Name())
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if *deviations.InterfaceEnabled {
			i.Enabled = ygot.Bool(true)
		}
		i.Description = ygot.String("from oc")
		i.Name = ygot.String(dp.Name())

		s := i.GetOrCreateSubinterface(0)
		s4 := s.GetOrCreateIpv4()
		if *deviations.InterfaceEnabled {
			s4.Enabled = ygot.Bool(true)
		}
		a4 := s4.GetOrCreateAddress(DUTIPList[dp.ID()].String())
		a4.PrefixLength = ygot.Uint8(plenIPv4)

		// BGP neighbor configs.
		nv4 := bgp.GetOrCreateNeighbor(ATEIPList[dp.ID()].String())
		nv4.PeerGroup = ygot.String(PeerGrpName)
		if dp.ID() == "port1" {
			nv4.PeerAs = ygot.Uint32(ateAs2)
		} else {
			nv4.PeerAs = ygot.Uint32(ATEAs)
		}
		nv4.Enabled = ygot.Bool(true)

		// ISIS configs.
		isisIntf := isis.GetOrCreateInterface(dp.Name())
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT

		isisIntfAuth := isisIntf.GetOrCreateAuthentication()
		isisIntfAuth.Enabled = ygot.Bool(true)
		isisIntfAuth.AuthPassword = ygot.String(authPassword)
		isisIntfAuth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
		isisIntfAuth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY

		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)

		isisIntfLevelTimers := isisIntfLevel.GetOrCreateTimers()
		isisIntfLevelTimers.HelloInterval = ygot.Uint32(1)
		isisIntfLevelTimers.HelloMultiplier = ygot.Uint8(5)

		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(200)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)
	}
	p := gnmi.OC()
	fptest.LogQuery(t, "DUT", p.Config(), d)

	return d
}
