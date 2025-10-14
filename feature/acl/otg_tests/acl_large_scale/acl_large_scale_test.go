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

package acllargescale_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
)

const (
	trafficFrameSize = 512
	trafficPps       = 100
	noOfPackets      = 5000
	sleepTime        = time.Duration(trafficFrameSize / trafficPps)

	// BGP AS
	atePort1AS = 65002
	atePort2AS = 65003
	atePort3AS = 65004
	atePort4AS = 65005
	dutPortAS  = 65001

	// BGP Peers
	peerGrpNamev4           = "BGP-PEER-GROUP-V4"
	peerGrpNamev6           = "BGP-PEER-GROUP-V6"
	rplName                 = "ALLOW"
	peerCountMultiplePrefix = 25000
	pfxvLen22               = peerCountMultiplePrefix * 5 / 100
	pfxvLen24               = peerCountMultiplePrefix * 35 / 100
	pfxvLen30               = peerCountMultiplePrefix * 30 / 100
	pfxvLen32               = peerCountMultiplePrefix * 30 / 100
	pfxvLen48               = peerCountMultiplePrefix * 20 / 100
	pfxvLen96               = peerCountMultiplePrefix * 20 / 100
	pfxvLen126              = peerCountMultiplePrefix * 30 / 100
	pfxvLen128              = peerCountMultiplePrefix * 30 / 100
	peerCountSrcPrefix      = 64
	peerCountDstPrefix      = 1

	// Prefix ips used
	prefixV4Address1 = "100.1.0.0"
	prefix1          = 22
	prefixV4Address2 = "50.1.0.0"
	prefix2          = 24
	prefixV4Address3 = "200.1.0.0"
	prefix3          = 30
	prefixV4Address4 = "210.1.0.0"
	prefix4          = 32
	prefixV6Address1 = "1000:1::0"
	prefixV6_1       = 48
	prefixV6Address2 = "5000:1::0"
	prefixV6_2       = 96
	prefixV6Address3 = "1500:1::0"
	prefixV6_3       = 126
	prefixV6Address4 = "2000:1::0"
	prefixV6_4       = 128

	// ACL name and type
	aclNameIPv4Len22 = "ACL_IPV4_Match_length_22_tcp_range"
	aclNameIPv4Len24 = "ACL_IPV4_Match_length_24_tcp_range"
	aclNameHighScale = "ACL_IPV4_Match_high_scale_statements"

	aclTypeIPv4 = oc.Acl_ACL_TYPE_ACL_IPV4
	aclTypeIPv6 = oc.Acl_ACL_TYPE_ACL_IPV6

	ipProtoTCP = 6
	bgpPort    = 179
)

var prfxListSrcIpv4List = []string{"60.1.0.0", "70.1.0.0", "80.1.0.0", "90.1.0.0"}
var prfxListSrcSubnetList = []int{24, 26, 27, 30}
var prfxListDstIpv4List = []string{"61.1.0.0", "61.2.0.0", "61.3.0.0", "61.4.0.0"}
var prfxListDstSubnet = 30

var prfxListSrcIpv6List = []string{"1500:1::0", "2500:1::0", "3500:1::0", "4500:1::0"}
var prfxListSrcV6SubnetList = []uint32{48, 64, 96, 126}
var prfxListDstIpv6List = []string{"1500:2::0", "2500:2::0", "3500:2::0", "4500:2::0"}
var prfxListDstV6Subnet = 112

var srcPortIPv4Len22 = []string{"900", "80", "30", "40", "150", "1600", "2700", "21000 - 45000", "30000 - 50000"}
var dstPortIPv4Len22 = []string{"800", "900", "100 - 20000"}

var srcPortIPv4Len24 = []string{"100", "200", "300", "400", "500", "600", "700", "2000 - 4000", "20000 - 40000"}
var dstPortIPv4Len24 = []string{"100 - 20000"}

var (
	dutPort1 = &attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.168.1.1",
		IPv6:    "2001:db8::1",
		IPv4Len: 30,
		IPv6Len: 126,
		MAC:     "02:01:00:00:00:01",
	}

	atePort1 = &attrs.Attributes{
		Name:    "ATEport1",
		Desc:    "ATE to DUT port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.168.1.2",
		IPv6:    "2001:db8::2",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	dutPort2 = &attrs.Attributes{
		Desc:    "DUT to ATE Port2",
		MAC:     "02:02:00:00:00:01",
		IPv4:    "192.168.1.5",
		IPv6:    "2001:db8::5",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	atePort2 = &attrs.Attributes{
		Name:    "ATEport2",
		Desc:    "ATE to DUT port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.168.1.6",
		IPv6:    "2001:db8::6",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	dutPort3 = &attrs.Attributes{
		Desc:    "DUT to ATE Port3",
		MAC:     "02:03:00:00:00:01",
		IPv4:    "192.168.1.9",
		IPv6:    "2001:db8::9",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	atePort3 = &attrs.Attributes{
		Name:    "ATEport3",
		Desc:    "ATE to DUT port3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.168.1.10",
		IPv6:    "2001:db8::a",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	dutPort4 = &attrs.Attributes{
		Desc:    "DUT to ATE Port4",
		MAC:     "02:04:00:00:00:01",
		IPv4:    "192.168.1.13",
		IPv6:    "2001:db8::c",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	atePort4 = &attrs.Attributes{
		Name:    "ATEport4",
		Desc:    "ATE to DUT port4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.168.1.14",
		IPv6:    "2001:db8::d",
		IPv4Len: 30,
		IPv6Len: 126,
	}

	otgPorts = map[string]*attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
		"port3": atePort3,
		"port4": atePort4,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
		"port3": dutPort3,
		"port4": dutPort4,
	}
)

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	name       string
}

type aclConfig struct {
	name       string
	srcIp      string
	destIp     []string
	srcTCPPort []string
	dstTCPPort []string
	action     string
	isV4       bool
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureACL)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)

}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, configBgp bool) {
	configureHardwareInit(t, dut)
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutPort1, dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))
	p3 := dut.Port(t, "port3")
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), configInterfaceDUT(p3, dutPort3, dut))
	p4 := dut.Port(t, "port4")
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), configInterfaceDUT(p4, dutPort4, dut))

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p4.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
		fptest.SetPortSpeed(t, p4)
	}

	if configBgp {
		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		bgpNbrs := []*bgpNeighbor{
			{as: atePort1AS, neighborip: atePort1.IPv4, isV4: true},
			{as: atePort1AS, neighborip: atePort1.IPv6, isV4: false},
			{as: atePort2AS, neighborip: atePort2.IPv4, isV4: true},
			{as: atePort2AS, neighborip: atePort2.IPv6, isV4: false},
			{as: atePort3AS, neighborip: atePort3.IPv4, isV4: true},
			{as: atePort3AS, neighborip: atePort3.IPv6, isV4: false},
			{as: atePort4AS, neighborip: atePort4.IPv4, isV4: true},
			{as: atePort4AS, neighborip: atePort4.IPv6, isV4: false},
		}

		dutConf := createBGPNeighbor(dutPortAS, bgpNbrs, dut)
		gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
	}
}

func configACL(t *testing.T, dut *ondatra.DUTDevice, aclConfig aclConfig) {
	aclRoot := &oc.Root{}
	acl := aclRoot.GetOrCreateAcl()

	rangeFlag := 0

	var src int
	var dst int
	if aclConfig.isV4 {
		aclEntryId := 10
		aclv4 := acl.GetOrCreateAclSet(aclConfig.name, oc.Acl_ACL_TYPE_ACL_IPV4)
		for _, dstIp := range aclConfig.destIp {
			for _, srcPort := range aclConfig.srcTCPPort {
				subrange := strings.Split(srcPort, "-")
				if len(subrange) > 1 {
					srcPort = fmt.Sprintf("%s..%s", strings.TrimSpace(subrange[0]), strings.TrimSpace(subrange[1]))
					rangeFlag = 1
				} else {
					src, _ = strconv.Atoi(srcPort)
				}
				for _, dstPort := range aclConfig.dstTCPPort {
					aclEntry := aclv4.GetOrCreateAclEntry(uint32(aclEntryId))
					aclEntry.SetSequenceId(uint32(aclEntryId))
					aclEntry.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
					a := aclEntry.GetOrCreateIpv4()
					a.Protocol = oc.UnionUint8(ipProtoTCP)
					a.SetSourceAddress(aclConfig.srcIp)
					a.SetDestinationAddress(dstIp)

					if rangeFlag == 1 {
						setRangeValue := []oc.Acl_AclSet_AclEntry_Transport_SourcePort_Union{oc.UnionString(srcPort)}
						aclEntry.GetOrCreateTransport().SetSourcePort(setRangeValue[0])
						rangeFlag = 0
					} else {
						aclEntry.GetOrCreateTransport().SourcePort = oc.UnionUint16(src)
					}

					subrange := strings.Split(dstPort, "-")
					if len(subrange) > 1 {
						dstPort = fmt.Sprintf("%s..%s", strings.TrimSpace(subrange[0]), strings.TrimSpace(subrange[1]))
						rangeFlag = 1
					} else {
						dst, _ = strconv.Atoi(dstPort)
					}
					if rangeFlag == 1 {
						setRangeValue := []oc.Acl_AclSet_AclEntry_Transport_DestinationPort_Union{oc.UnionString(dstPort)}
						aclEntry.GetOrCreateTransport().SetDestinationPort(setRangeValue[0])
						rangeFlag = 0
					} else {
						aclEntry.GetOrCreateTransport().DestinationPort = oc.UnionUint16(dst)
					}

					aclEntryId += 10
				}
			}
		}
		if deviations.ConfigACLValueAnyOcUnsupported(dut) {
			cliConfig := ""
			switch dut.Vendor() {
			case ondatra.ARISTA:
				t.Log("Configure Acl to block BGP on port 179")
				cliConfig = fmt.Sprintf(`
					ip access-list %s
					%d permit ip any any
					%d permit tcp any eq 179 any
					`, aclConfig.name, aclEntryId, aclEntryId+10)
				helpers.GnmiCLIConfig(t, dut, cliConfig)
			}
		} else {
			aclEntry := aclv4.GetOrCreateAclEntry(uint32(aclEntryId))
			aclEntry.SetSequenceId(uint32(aclEntryId))
			aclEntry.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
			a := aclEntry.GetOrCreateIpv4()
			a.Protocol = oc.UnionUint8(ipProtoTCP)
			aclEntry.GetOrCreateTransport().SourcePort = oc.UnionUint16(bgpPort)
			a.SetSourceAddress(oc.Transport_DestinationPort_ANY.String())
			a.SetDestinationAddress(oc.Transport_DestinationPort_ANY.String())
		}
	} else {
		aclv6 := acl.GetOrCreateAclSet(aclConfig.name, oc.Acl_ACL_TYPE_ACL_IPV6)
		aclEntryId := 10
		for _, dstIp := range aclConfig.destIp {
			for _, srcPort := range aclConfig.srcTCPPort {
				subrange := strings.Split(srcPort, "-")
				if len(subrange) > 1 {
					srcPort = fmt.Sprintf("%s..%s", strings.TrimSpace(subrange[0]), strings.TrimSpace(subrange[1]))
					rangeFlag = 1
				} else {
					src, _ = strconv.Atoi(srcPort)
				}
				for _, dstPort := range aclConfig.dstTCPPort {
					aclEntry := aclv6.GetOrCreateAclEntry(uint32(aclEntryId))
					aclEntry.SetSequenceId(uint32(aclEntryId))
					aclEntry.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
					a := aclEntry.GetOrCreateIpv6()
					a.Protocol = oc.UnionUint8(ipProtoTCP)
					a.SetSourceAddress(aclConfig.srcIp)
					a.SetDestinationAddress(dstIp)

					if rangeFlag == 1 {
						setRangeValue := []oc.Acl_AclSet_AclEntry_Transport_SourcePort_Union{oc.UnionString(srcPort)}
						aclEntry.GetOrCreateTransport().SetSourcePort(setRangeValue[0])
						rangeFlag = 0
					} else {
						aclEntry.GetOrCreateTransport().SourcePort = oc.UnionUint16(src)
					}

					subrange := strings.Split(dstPort, "-")
					if len(subrange) > 1 {
						dstPort = fmt.Sprintf("%s..%s", strings.TrimSpace(subrange[0]), strings.TrimSpace(subrange[1]))
						rangeFlag = 1
					} else {
						dst, _ = strconv.Atoi(dstPort)
					}
					if rangeFlag == 1 {
						setRangeValue := []oc.Acl_AclSet_AclEntry_Transport_DestinationPort_Union{oc.UnionString(dstPort)}
						aclEntry.GetOrCreateTransport().SetDestinationPort(setRangeValue[0])
						rangeFlag = 0
					} else {
						aclEntry.GetOrCreateTransport().DestinationPort = oc.UnionUint16(dst)
					}
					aclEntryId += 10
				}
			}
		}
		t.Log("Configure Acl to block BGP on port 179")
		cliConfig := fmt.Sprintf(`
			ipv6 access-list %s
			%d permit ipv6 any any
			%d permit tcp any eq 179 any
			%d permit icmpv6 any any
			`, aclConfig.name, aclEntryId, aclEntryId+10, aclEntryId+20)
		helpers.GnmiCLIConfig(t, dut, cliConfig)
	}

	t.Log("Pushing ACL configuration...")
	gnmi.Update(t, dut, gnmi.OC().Acl().Config(), acl)
	t.Log("ACL configuration applied.")
}

// TODO: Raised issue  416164360 for unsupport of logging
func configHighScaleACL(t *testing.T, dut *ondatra.DUTDevice, name string, startCount int, lastCount int, srcAddr string, action string, log bool, aclType oc.E_Acl_ACL_TYPE) {
	if deviations.ConfigACLValueAnyOcUnsupported(dut) {
		ipType := ""
		switch aclType {
		case aclTypeIPv4:
			ipType = "ip"
		case aclTypeIPv6:
			ipType = "ipv6"
		}
		cliConfig := fmt.Sprintf("%s access-list %s\n", ipType, name)
		switch dut.Vendor() {
		case ondatra.ARISTA:
			for i := startCount; i <= lastCount; i++ {
				cliConfig += fmt.Sprintf("%d permit %s %s any\n", i, ipType, srcAddr)
			}
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("acl cli is not handled for the dut: %v", dut.Vendor())
		}
	} else {
		aclRoot := &oc.Root{}
		acl := aclRoot.GetOrCreateAcl()
		for i := startCount; i <= lastCount; i++ {
			aclEntry := acl.GetOrCreateAclSet(name, aclTypeIPv4).GetOrCreateAclEntry(uint32(i))
			aclEntry.SetSequenceId(uint32(i))
			if action == "ACCEPT" {
				aclEntry.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
			}

			if log {
				aclEntry.GetOrCreateActions().SetLogAction(oc.Acl_LOG_ACTION_LOG_SYSLOG)
			}

			a := aclEntry.GetOrCreateIpv4()
			a.SetSourceAddress(oc.Transport_DestinationPort_ANY.String())
			a.SetDestinationAddress(oc.Transport_DestinationPort_ANY.String())
		}
		gnmi.Update(t, dut, gnmi.OC().Acl().Config(), acl)
	}
}

func configACLInterface(t *testing.T, dut *ondatra.DUTDevice, portName string, aclName string, ingress bool, v4 bool) {
	d := &oc.Root{}
	ifName := dut.Port(t, portName).Name()

	aclConf := gnmi.OC().Acl().Interface(ifName)
	iFace := d.GetOrCreateAcl().GetOrCreateInterface(ifName)

	if v4 {
		if ingress {
			iFace.GetOrCreateIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
		} else {
			iFace.GetOrCreateEgressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
		}
	} else {
		if ingress {
			iFace.GetOrCreateIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV6)
		} else {
			iFace.GetOrCreateEgressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV6)
		}
	}

	iFace.GetOrCreateInterfaceRef().SetInterface(ifName)
	iFace.GetOrCreateInterfaceRef().SetSubinterface(0)

	gnmi.Replace(t, dut, aclConf.Config(), iFace)
}

func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.SetEnabled(true)
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()

	return i
}

func createBGPNeighbor(localAs uint32, bgpNbrs []*bgpNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.SetAs(localAs)
	global.SetRouterId(dutPort1.IPv4)

	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)

	for i, nbr := range bgpNbrs {
		if nbr.isV4 {
			pgv4 := bgp.GetOrCreatePeerGroup(fmt.Sprintf("%s-%d", peerGrpNamev4, i+1))
			pgv4.SetPeerAs(nbr.as)
			pgv4.SetPeerGroupName(fmt.Sprintf("%s-%d", peerGrpNamev4, i+1))

			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.SetPeerAs(nbr.as)
			nv4.SetEnabled(true)
			nv4.SetPeerGroup(fmt.Sprintf("%s-%d", peerGrpNamev4, i+1))

			afisafi := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			afisafi.SetEnabled(true)
			nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(false)
			pgafv4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			pgafv4.SetEnabled(true)

		} else {
			pgv6 := bgp.GetOrCreatePeerGroup(fmt.Sprintf("%s-%d", peerGrpNamev6, i+1))
			pgv6.SetPeerAs(nbr.as)
			pgv6.SetPeerGroupName(fmt.Sprintf("%s-%d", peerGrpNamev6, i+1))

			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.SetPeerAs(nbr.as)
			nv6.SetEnabled(true)
			nv6.SetPeerGroup(fmt.Sprintf("%s-%d", peerGrpNamev6, i+1))

			afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			afisafi6.SetEnabled(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(false)
			pgafv6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			pgafv6.SetEnabled(true)
		}
	}
	return niProto
}

func configureOTG(t *testing.T, otg *ondatra.ATEDevice, prefixConfig bool) gosnappi.Config {

	otgConfig := gosnappi.NewConfig()

	for portName, portAttrs := range otgPorts {
		port := otg.Port(t, portName)
		dutPort := dutPorts[portName]
		portAttrs.AddToOTG(otgConfig, port, dutPort)
	}

	bgpv4Devices := []*bgpNeighbor{
		{neighborip: atePort1.IPv4, as: atePort1AS, name: atePort1.Name, isV4: true},
		{neighborip: atePort2.IPv4, as: atePort2AS, name: atePort2.Name, isV4: true},
		{neighborip: atePort3.IPv4, as: atePort3AS, name: atePort3.Name, isV4: true},
		{neighborip: atePort4.IPv4, as: atePort4AS, name: atePort4.Name, isV4: true},
	}

	bgpv6Devices := []*bgpNeighbor{
		{neighborip: atePort1.IPv6, as: atePort1AS, name: atePort1.Name, isV4: false},
		{neighborip: atePort2.IPv6, as: atePort2AS, name: atePort2.Name, isV4: false},
		{neighborip: atePort3.IPv6, as: atePort3AS, name: atePort3.Name, isV4: false},
		{neighborip: atePort4.IPv6, as: atePort4AS, name: atePort4.Name, isV4: false},
	}

	multiplePrefixV4IPs := []struct {
		startIP string
		subnet  uint32
		isIPv6  bool
		count   int
	}{
		{prefixV4Address1, prefix1, false, pfxvLen22},
		{prefixV4Address2, prefix2, false, pfxvLen24},
		{prefixV4Address3, prefix3, false, pfxvLen30},
		{prefixV4Address4, prefix4, false, pfxvLen32},
	}

	multiplePrefixV6IPs := []struct {
		startIP string
		subnet  uint32
		isIPv6  bool
		count   int
	}{
		{prefixV6Address1, prefixV6_1, true, pfxvLen48},
		{prefixV6Address2, prefixV6_2, true, pfxvLen96},
		{prefixV6Address3, prefixV6_3, true, pfxvLen126},
		{prefixV6Address4, prefixV6_4, true, pfxvLen128},
	}

	devices := otgConfig.Devices().Items()

	// PrefixConfig flag is set to test the testcases using prefix-list, it will create routes used for prefix-list
	if prefixConfig {
		for i, peer := range bgpv4Devices {
			bgp := devices[i].Bgp().SetRouterId(peer.neighborip)
			iDut1Ipv4 := devices[i].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
			iDut1Bgp := bgp.SetRouterId(iDut1Ipv4.Address())

			iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(peer.name + ".BGP4.peer")
			iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(peer.as).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			// configure Ipv4 Network Group
			bgp4Peer := devices[i].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]

			// Configure IPv4 Network group with Source Prefix List
			netV4_2 := bgp4Peer.V4Routes().Add().SetName(fmt.Sprintf("v4-bgpNetSrc-devSrc%d-2", i+1))
			netV4_2.Addresses().Add().SetAddress(prfxListSrcIpv4List[i]).SetPrefix(uint32(prfxListSrcSubnetList[i])).SetCount(peerCountSrcPrefix)

			// Configure IPv4 Network group with Destination Prefix List
			netV4_3 := bgp4Peer.V4Routes().Add().SetName(fmt.Sprintf("v4-bgpNetDst-devDst%d-3", i+1))
			netV4_3.Addresses().Add().SetAddress(prfxListDstIpv4List[i]).SetPrefix(uint32(prfxListDstSubnet)).SetCount(peerCountDstPrefix)
		}

		for i, peer := range bgpv6Devices {
			bgp := devices[i].Bgp().SetRouterId(peer.neighborip)
			iDut1Ipv4 := devices[i].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
			iDut1Bgp := bgp.SetRouterId(iDut1Ipv4.Address())

			// eBGP v6 session on OTG Port.
			iDut1Ipv6 := devices[i].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
			iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(peer.name + ".BGP6.peer")
			iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(peer.as).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

			// configure Ipv6 Network Group
			bgp6Peer := devices[i].Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]

			// Configure IPv6 Network group with Source Prefix List
			netV6_2 := bgp6Peer.V6Routes().Add().SetName(fmt.Sprintf("v6-bgpNetSrc-devSrc%d-2", i+1))
			netV6_2.Addresses().Add().SetAddress(prfxListSrcIpv6List[i]).SetPrefix(prfxListSrcV6SubnetList[i]).SetCount(peerCountSrcPrefix)

			// Configure IPv6 Network group with Destination Prefix List
			netV6_3 := bgp6Peer.V6Routes().Add().SetName(fmt.Sprintf("v6-bgpNetDst-devDst%d-3", i+1))
			netV6_3.Addresses().Add().SetAddress(prfxListDstIpv6List[i]).SetPrefix(uint32(prfxListDstV6Subnet)).SetCount(peerCountDstPrefix)
		}
	} else {
		for i, peer := range bgpv4Devices {
			bgp := devices[i].Bgp().SetRouterId(peer.neighborip)
			iDut1Ipv4 := devices[i].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
			iDut1Bgp := bgp.SetRouterId(iDut1Ipv4.Address())

			iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(peer.name + ".BGP4.peer")
			iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(peer.as).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			// configure Ipv4 Network Group
			bgp4Peer := devices[i].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]

			for index, prefixIPs := range multiplePrefixV4IPs {
				// Configure IPv4 Network group with multiple prefixes
				netV4_1 := bgp4Peer.V4Routes().Add().SetName(fmt.Sprintf("v4-bgpNet-dev%d-1-prfx%d", i+1, index+1))
				netV4_1.Addresses().Add().SetAddress(prefixIPs.startIP).SetPrefix(prefixIPs.subnet).SetCount(uint32(prefixIPs.count))
			}

		}

		for i, peer := range bgpv6Devices {
			bgp := devices[i].Bgp().SetRouterId(peer.neighborip)
			iDut1Ipv4 := devices[i].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
			iDut1Bgp := bgp.SetRouterId(iDut1Ipv4.Address())

			// eBGP v6 session on OTG Port.
			iDut1Ipv6 := devices[i].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
			iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(peer.name + ".BGP6.peer")
			iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(peer.as).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

			// configure Ipv6 Network Group
			bgp6Peer := devices[i].Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]

			for index, prefixIPs := range multiplePrefixV6IPs {
				// Configure IPv6 Network group with multiple prefixes
				netV6 := bgp6Peer.V6Routes().Add().SetName(fmt.Sprintf("v6-bgpNet-dev%d-1-prfx%d", i+1, index+1))
				netV6.Addresses().Add().SetAddress(prefixIPs.startIP).SetPrefix(prefixIPs.subnet).SetCount(uint32(prefixIPs.count))
			}
		}

	}

	return otgConfig
}

func configureTrafficPolicy(t *testing.T, dut *ondatra.DUTDevice, policyName string, protocolType string, srcAddress []string, dstAddress string, srcPort string, dstPort string, intfName string) {
	aclTrafficPolicyConfig := cfgplugins.ACLTrafficPolicyParams{
		PolicyName:   policyName,
		ProtocolType: protocolType,
		SrcPrefix:    srcAddress,
		DstPrefix:    dstAddress,
		SrcPort:      srcPort,
		DstPort:      dstPort,
		IntfName:     intfName,
	}

	cfgplugins.ConfigureTrafficPolicyACL(t, dut, aclTrafficPolicyConfig)
}

// createFlow returns a flow from atePort1 to the dstPfx
func createFlow(flowName string, srcPort []string, dstPort []string, srcAddress []string, dstAddress []string, tcpSrcPort uint32, tcpDstPort uint32, protocolType string) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames(srcPort).SetRxNames(dstPort)
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetPps(trafficPps)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(noOfPackets))

	flow.Packet().Add().Ethernet()

	if protocolType == "ipv4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValues(srcAddress)
		v4.Dst().SetValues(dstAddress)
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValues(srcAddress)
		v6.Dst().SetValues(dstAddress)
	}

	tcp := flow.Packet().Add().Tcp()
	tcp.SrcPort().SetValue(tcpSrcPort)
	tcp.DstPort().SetValue(tcpDstPort)

	return flow
}

func withdrawBGPRoutes(t *testing.T, conf gosnappi.Config, routeNames []string) {

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.WITHDRAW)
	otg.SetControlState(t, cs)

}

func verifyACLOnInterface(t *testing.T, dut *ondatra.DUTDevice, expectedName string, port string, ingress bool) {
	ifName := dut.Port(t, port).Name()
	intfPath := gnmi.OC().Acl().Interface(ifName)
	var aclName string
	if ingress {
		aclName = gnmi.GetAll(t, dut, intfPath.IngressAclSetAny().State())[0].GetSetName()
	} else {
		aclName = gnmi.GetAll(t, dut, intfPath.EgressAclSetAny().State())[0].GetSetName()
	}

	if aclName == expectedName {
		t.Logf("ACL is configured correctly on %s", ifName)
	} else {
		t.Logf("ACL is not configured on %s. Expected: %s, got: %s", ifName, expectedName, aclName)
	}
}

func validateTrafficLoss(t *testing.T, otgConfig *otg.OTG, flowName string) {
	outPkts := float32(gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flowName).Counters().OutPkts().State()))
	inPkts := float32(gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flowName).Counters().InPkts().State()))
	t.Logf("outPkts: %v, inPkts: %v", outPkts, inPkts)
	if outPkts == 0 {
		t.Fatalf("OutPkts for flow %s is 0, want > 0", flowName)
	}
	if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
		t.Fatalf("LossPct for flow %s: got %v, want 0", flowName, got)
	}
}

// getACLMatchedPackets retrieves the matched packet count for a specific ACL entry applied to the control plane.
// TODO: Validation of logging (Raised issue 416164360)
// func getACLMatchedPackets(t *testing.T, dut *ondatra.DUTDevice, aclName string, aclType oc.E_Acl_ACL_TYPE, seqID uint32) uint64 {
// 	t.Helper()
// 	counterQuery := gnmi.OC().System().ControlPlaneTraffic().Ingress().AclSet(aclName, aclType).AclEntry(seqID).MatchedPackets().State()
// 	val := gnmi.Lookup(t, dut, counterQuery)
// 	count, present := val.Val()
// 	if !present {
// 		t.Logf("ACL counter not present for ACL %s, Type %s, Seq %d. Assuming 0.", aclName, aclType, seqID)
// 		return 0 // Return 0 if the counter path doesn't exist yet
// 	}
// 	return count
// }

func removeAClOnInterface(t *testing.T, dut *ondatra.DUTDevice, portName string) {
	ifName := dut.Port(t, portName).Name()
	aclConf := gnmi.OC().Acl().Interface(ifName)
	gnmi.Delete(t, dut, aclConf.Config())
}

func TestAclLargeScale(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configure DUT
	t.Logf("Configuring on DUT")
	configureDUT(t, dut, true)

	// Configure OTG
	t.Logf("Configure on OTG")
	otgConfig := ate.OTG()
	config := configureOTG(t, ate, false)
	otgConfig.PushConfig(t, config)

	testCases := []struct {
		desc     string
		testFunc func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig *otg.OTG, config gosnappi.Config)
	}{
		{
			desc:     "ACL-1.1.1 - ACL IPv4 Address scale",
			testFunc: testv4AddressScale,
		},
		{
			desc:     "ACL-1.1.2 - ACL IPv6 Address scale",
			testFunc: testv6AddressScale,
		},
		{
			desc:     "ACL-1.2.1 - ACL IPv4 Address scale using prefix-list",
			testFunc: testv4PrefixList,
		},
		{
			desc:     "ACL-1.2.2 - ACL IPv6 Address scale using prefix-list",
			testFunc: testv6PrefixList,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("Description: %s", tc.desc)
			tc.testFunc(t, dut, ate, otgConfig, config)
		})
	}
}

func testv4AddressScale(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig *otg.OTG, config gosnappi.Config) {
	destIps_1 := []string{fmt.Sprintf("%s/%d", prefixV4Address2, prefix2), fmt.Sprintf("%s/%d", prefixV4Address3, prefix3), fmt.Sprintf("%s/%d", prefixV4Address4, prefix4)}
	destIps_2 := []string{fmt.Sprintf("%s/%d", prefixV4Address1, prefix1), fmt.Sprintf("%s/%d", prefixV4Address2, prefix2), fmt.Sprintf("%s/%d", prefixV4Address3, prefix3)}

	aclConfigs := []aclConfig{
		{
			name:       aclNameIPv4Len22,
			srcIp:      fmt.Sprintf("%s/%d", prefixV4Address1, prefix1),
			destIp:     destIps_1,
			srcTCPPort: srcPortIPv4Len22,
			dstTCPPort: dstPortIPv4Len22,
			action:     "ACCEPT",
			isV4:       true,
		},
		{
			name:       aclNameIPv4Len24,
			srcIp:      fmt.Sprintf("%s/%d", prefixV4Address4, prefix4),
			destIp:     destIps_2,
			srcTCPPort: srcPortIPv4Len24,
			dstTCPPort: dstPortIPv4Len24,
			action:     "ACCEPT",
			isV4:       true,
		},
	}

	for _, acl := range aclConfigs {
		configACL(t, dut, acl)
	}

	highScaleACL := []struct {
		name       string
		startCount int
		lastCount  int
		srcIp      string
		action     string
		log        bool
	}{
		{aclNameHighScale, 1, 100, fmt.Sprintf("%s/%d", prefixV4Address1, prefix1), "ACCEPT", false},
		{aclNameHighScale, 126, 150, fmt.Sprintf("%s/%d", prefixV4Address2, prefix2), "ACCEPT", true},
		{aclNameHighScale, 151, 175, fmt.Sprintf("%s/%d", prefixV4Address3, prefix3), "ACCEPT", false},
		{aclNameHighScale, 176, 200, fmt.Sprintf("%s/%d", prefixV4Address4, prefix4), "ACCEPT", false},
	}
	for _, acl := range highScaleACL {
		configHighScaleACL(t, dut, acl.name, acl.startCount, acl.lastCount, acl.srcIp, acl.action, acl.log, aclTypeIPv4)
	}

	// Apply ACL_IPV4_Match_length_22_tcp_range on DUT-port1 Ingress
	configACLInterface(t, dut, "port1", aclNameIPv4Len22, true, true)

	// Apply ACL_IPV4_Match_length_24_tcp_range on DUT-port2 Ingress
	configACLInterface(t, dut, "port2", aclNameIPv4Len24, true, true)

	// Apply ACL_IPV4_Match_high_scale_statements on DUT-port3 Ingress
	configACLInterface(t, dut, "port3", aclNameHighScale, true, true)

	// Apply ACL_IPV4_Match_high_scale_statements on DUT-port4 Egress
	configACLInterface(t, dut, "port4", aclNameHighScale, false, true)

	// Verification of ACL on interfaces as Ingress & Egress
	var expectedACLs = []struct {
		Name    string
		Ingress bool
		port    string
	}{
		{Name: aclNameIPv4Len22, Ingress: true, port: "port1"},
		{Name: aclNameIPv4Len24, Ingress: true, port: "port2"},
		{Name: aclNameHighScale, Ingress: true, port: "port3"},
		{Name: aclNameHighScale, Ingress: false, port: "port4"},
	}
	for _, aclSet := range expectedACLs {
		verifyACLOnInterface(t, dut, aclSet.Name, aclSet.port, aclSet.Ingress)
	}

	config.Flows().Clear()

	var flowList = []struct {
		Name              string
		srcDevice         []string
		dstDevice         []string
		srcAddr           []string
		dstAddr           []string
		tcpSrcPort        uint32
		tcpDstPort        uint32
		protocol          string
		withdrawBGPRoutes []string
	}{
		{
			Name:      "port1ToMany",
			srcDevice: []string{"v4-bgpNet-dev1-1-prfx1"},
			dstDevice: []string{
				"v4-bgpNet-dev2-1-prfx2", "v4-bgpNet-dev2-1-prfx3", "v4-bgpNet-dev2-1-prfx4",
				"v4-bgpNet-dev3-1-prfx2", "v4-bgpNet-dev3-1-prfx3", "v4-bgpNet-dev3-1-prfx4",
				"v4-bgpNet-dev4-1-prfx2", "v4-bgpNet-dev4-1-prfx3", "v4-bgpNet-dev4-1-prfx4",
			},
			srcAddr:    []string{strings.Split(prefixV4Address1, "/")[0]},
			dstAddr:    []string{strings.Split(prefixV4Address2, "/")[0], strings.Split(prefixV4Address3, "/")[0], strings.Split(prefixV4Address4, "/")[0]},
			tcpSrcPort: 500,
			tcpDstPort: 2000,
			protocol:   "ipv4",
			withdrawBGPRoutes: []string{
				"v4-bgpNet-dev1-1-prfx2", "v4-bgpNet-dev1-1-prfx3", "v4-bgpNet-dev1-1-prfx4",
				"v4-bgpNet-dev2-1-prfx1", "v4-bgpNet-dev3-1-prfx1", "v4-bgpNet-dev4-1-prfx1"},
		},
		{
			Name:      "port2ToMany",
			srcDevice: []string{"v4-bgpNet-dev2-1-prfx2"},
			dstDevice: []string{
				"v4-bgpNet-dev1-1-prfx1", "v4-bgpNet-dev1-1-prfx3", "v4-bgpNet-dev1-1-prfx4",
				"v4-bgpNet-dev3-1-prfx1", "v4-bgpNet-dev3-1-prfx3", "v4-bgpNet-dev3-1-prfx4",
				"v4-bgpNet-dev4-1-prfx1", "v4-bgpNet-dev4-1-prfx3", "v4-bgpNet-dev4-1-prfx4"},
			srcAddr:    []string{strings.Split(prefixV4Address2, "/")[0]},
			dstAddr:    []string{strings.Split(prefixV4Address1, "/")[0], strings.Split(prefixV4Address3, "/")[0], strings.Split(prefixV4Address4, "/")[0]},
			tcpSrcPort: 500,
			tcpDstPort: 2000,
			protocol:   "ipv4",
			withdrawBGPRoutes: []string{
				"v4-bgpNet-dev1-1-prfx2", "v4-bgpNet-dev3-1-prfx2", "v4-bgpNet-dev4-1-prfx2",
				"v4-bgpNet-dev2-1-prfx1", "v4-bgpNet-dev2-1-prfx3", "v4-bgpNet-dev2-1-prfx4"},
		},
		{
			Name:      "port3ToMany",
			srcDevice: []string{"v4-bgpNet-dev3-1-prfx3"},
			dstDevice: []string{
				"v4-bgpNet-dev2-1-prfx2", "v4-bgpNet-dev2-1-prfx1", "v4-bgpNet-dev2-1-prfx4",
				"v4-bgpNet-dev1-1-prfx2", "v4-bgpNet-dev1-1-prfx1", "v4-bgpNet-dev1-1-prfx4",
				"v4-bgpNet-dev4-1-prfx2", "v4-bgpNet-dev4-1-prfx1", "v4-bgpNet-dev4-1-prfx4"},
			srcAddr:    []string{strings.Split(prefixV4Address3, "/")[0]},
			dstAddr:    []string{strings.Split(prefixV4Address1, "/")[0], strings.Split(prefixV4Address2, "/")[0], strings.Split(prefixV4Address4, "/")[0]},
			tcpSrcPort: 500,
			tcpDstPort: 2000,
			protocol:   "ipv4",
			withdrawBGPRoutes: []string{
				"v4-bgpNet-dev3-1-prfx2", "v4-bgpNet-dev3-1-prfx1", "v4-bgpNet-dev3-1-prfx4",
				"v4-bgpNet-dev2-1-prfx3", "v4-bgpNet-dev1-1-prfx3", "v4-bgpNet-dev4-1-prfx3"},
		},
		{
			Name:      "port4ToMany",
			srcDevice: []string{"v4-bgpNet-dev4-1-prfx4"},
			dstDevice: []string{
				"v4-bgpNet-dev2-1-prfx2", "v4-bgpNet-dev2-1-prfx3", "v4-bgpNet-dev2-1-prfx1",
				"v4-bgpNet-dev3-1-prfx2", "v4-bgpNet-dev3-1-prfx3", "v4-bgpNet-dev3-1-prfx1",
				"v4-bgpNet-dev1-1-prfx2", "v4-bgpNet-dev1-1-prfx3", "v4-bgpNet-dev1-1-prfx1"},
			srcAddr:    []string{strings.Split(prefixV4Address4, "/")[0]},
			dstAddr:    []string{strings.Split(prefixV4Address1, "/")[0], strings.Split(prefixV4Address2, "/")[0], strings.Split(prefixV4Address3, "/")[0]},
			tcpSrcPort: 500,
			tcpDstPort: 2000,
			protocol:   "ipv4",
			withdrawBGPRoutes: []string{
				"v4-bgpNet-dev4-1-prfx2", "v4-bgpNet-dev4-1-prfx3", "v4-bgpNet-dev4-1-prfx1",
				"v4-bgpNet-dev2-1-prfx4", "v4-bgpNet-dev3-1-prfx4", "v4-bgpNet-dev1-1-prfx4"},
		},
	}

	for _, flows := range flowList {

		flow := createFlow(
			flows.Name,
			flows.srcDevice,
			flows.dstDevice,
			flows.srcAddr,
			flows.dstAddr,
			flows.tcpSrcPort,
			flows.tcpDstPort,
			flows.protocol,
		)

		config.Flows().Append(flow)

		otgConfig.PushConfig(t, config)
		otgConfig.StartProtocols(t)
		time.Sleep(time.Second * 60)

		t.Logf("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, ate, 6*time.Minute)

		t.Logf("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, dut)

		withdrawBGPRoutes(t, config, flows.withdrawBGPRoutes)

		otgConfig.StartTraffic(t)
		time.Sleep(time.Second * 60)
		otgConfig.StopTraffic(t)

		// Verify Traffic
		otgutils.LogFlowMetrics(t, otgConfig, config)
		otgutils.LogPortMetrics(t, otgConfig, config)
		validateTrafficLoss(t, otgConfig, flows.Name)
		config.Flows().Clear()

	}

	// TODO: Validation of logging (Raised issue 416164360)

}

func testv6AddressScale(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig *otg.OTG, config gosnappi.Config) {

	aclConfigs := []aclConfig{
		{
			name:  "ACL_IPV6_Match_length_48_tcp_range",
			srcIp: fmt.Sprintf("%s/%d", prefixV6Address1, prefixV6_1),
			destIp: []string{
				fmt.Sprintf("%s/%d", prefixV6Address2, prefixV6_2),
				fmt.Sprintf("%s/%d", prefixV6Address3, prefixV6_3),
				fmt.Sprintf("%s/%d", prefixV6Address4, prefixV6_4)},
			srcTCPPort: srcPortIPv4Len22,
			dstTCPPort: dstPortIPv4Len22,
			action:     "ACCEPT",
			isV4:       false,
		},
		{
			name:  "ACL_IPV6_Match_length_96_tcp_range",
			srcIp: fmt.Sprintf("%s/%d", prefixV6Address4, prefixV6_4),
			destIp: []string{
				fmt.Sprintf("%s/%d", prefixV6Address2, prefixV6_2),
				fmt.Sprintf("%s/%d", prefixV6Address3, prefixV6_3),
				fmt.Sprintf("%s/%d", prefixV6Address1, prefixV6_1)},
			srcTCPPort: srcPortIPv4Len24,
			dstTCPPort: dstPortIPv4Len24,
			action:     "ACCEPT",
			isV4:       false,
		},
	}

	for _, acl := range aclConfigs {
		configACL(t, dut, acl)
	}

	highScaleACL := []struct {
		name       string
		startCount int
		lastCount  int
		srcIp      string
		action     string
		log        bool
	}{
		{"ACL_IPV6_Match_high_scale_statements", 1, 125, fmt.Sprintf("%s/%d", prefixV6Address1, prefixV6_1), "ACCEPT", false},
		{"ACL_IPV6_Match_high_scale_statements", 126, 150, fmt.Sprintf("%s/%d", prefixV6Address2, prefixV6_2), "ACCEPT", true},
		{"ACL_IPV6_Match_high_scale_statements", 151, 175, fmt.Sprintf("%s/%d", prefixV6Address3, prefixV6_3), "ACCEPT", false},
		{"ACL_IPV6_Match_high_scale_statements", 176, 200, fmt.Sprintf("%s/%d", prefixV6Address4, prefixV6_4), "ACCEPT", false},
	}
	for _, acl := range highScaleACL {
		configHighScaleACL(t, dut, acl.name, acl.startCount, acl.lastCount, acl.srcIp, acl.action, acl.log, aclTypeIPv6)
	}

	// Apply ACL_IPV4_Match_length_22_tcp_range on DUT-port1 Ingress
	configACLInterface(t, dut, "port1", "ACL_IPV6_Match_length_48_tcp_range", true, false)

	// Apply ACL_IPV4_Match_length_24_tcp_range on DUT-port2 Ingress
	configACLInterface(t, dut, "port2", "ACL_IPV6_Match_length_96_tcp_range", true, false)

	// Apply ACL_IPV4_Match_high_scale_statements on DUT-port3 Ingress
	configACLInterface(t, dut, "port3", "ACL_IPV6_Match_high_scale_statements", true, false)

	// Apply ACL_IPV4_Match_high_scale_statements on DUT-port4 Egress
	configACLInterface(t, dut, "port4", "ACL_IPV6_Match_high_scale_statements", false, false)

	// Verify ACL is applied on the interfaces
	var expectedACLs = []struct {
		Name    string
		Ingress bool
		port    string
	}{
		{Name: "ACL_IPV6_Match_length_48_tcp_range", Ingress: true, port: "port1"},
		{Name: "ACL_IPV6_Match_length_96_tcp_range", Ingress: true, port: "port2"},
		{Name: "ACL_IPV6_Match_high_scale_statements", Ingress: true, port: "port3"},
		{Name: "ACL_IPV6_Match_high_scale_statements", Ingress: false, port: "port4"},
	}
	for _, aclSet := range expectedACLs {
		verifyACLOnInterface(t, dut, aclSet.Name, aclSet.port, aclSet.Ingress)
	}

	config.Flows().Clear()

	var flowList = []struct {
		Name              string
		srcDevice         []string
		dstDevice         []string
		srcAddr           []string
		dstAddr           []string
		tcpSrcPort        uint32
		tcpDstPort        uint32
		protocol          string
		withdrawBGPRoutes []string
	}{
		{
			Name:      "port1ToMany",
			srcDevice: []string{"v6-bgpNet-dev1-1-prfx1"},
			dstDevice: []string{
				"v6-bgpNet-dev2-1-prfx2", "v6-bgpNet-dev2-1-prfx3", "v6-bgpNet-dev2-1-prfx4",
				"v6-bgpNet-dev3-1-prfx2", "v6-bgpNet-dev3-1-prfx3", "v6-bgpNet-dev3-1-prfx4",
				"v6-bgpNet-dev4-1-prfx2", "v6-bgpNet-dev4-1-prfx3", "v6-bgpNet-dev4-1-prfx4"},
			srcAddr:    []string{prefixV6Address1},
			dstAddr:    []string{prefixV6Address2, prefixV6Address3, prefixV6Address4},
			tcpSrcPort: 150,
			tcpDstPort: 2000,
			protocol:   "ipv6",
			withdrawBGPRoutes: []string{
				"v6-bgpNet-dev1-1-prfx2", "v6-bgpNet-dev1-1-prfx3", "v6-bgpNet-dev1-1-prfx4",
				"v6-bgpNet-dev2-1-prfx1", "v6-bgpNet-dev3-1-prfx1", "v6-bgpNet-dev4-1-prfx1"},
		},
		{
			Name:      "port2ToMany",
			srcDevice: []string{"v6-bgpNet-dev2-1-prfx2"},
			dstDevice: []string{
				"v6-bgpNet-dev1-1-prfx1", "v6-bgpNet-dev1-1-prfx3", "v6-bgpNet-dev1-1-prfx4",
				"v6-bgpNet-dev3-1-prfx1", "v6-bgpNet-dev3-1-prfx3", "v6-bgpNet-dev3-1-prfx4",
				"v6-bgpNet-dev4-1-prfx1", "v6-bgpNet-dev4-1-prfx3", "v6-bgpNet-dev4-1-prfx4"},
			srcAddr:    []string{prefixV6Address2},
			dstAddr:    []string{prefixV6Address1, prefixV6Address3, prefixV6Address4},
			tcpSrcPort: 150,
			tcpDstPort: 2000,
			protocol:   "ipv6",
			withdrawBGPRoutes: []string{
				"v6-bgpNet-dev1-1-prfx2", "v6-bgpNet-dev3-1-prfx2", "v6-bgpNet-dev4-1-prfx2",
				"v6-bgpNet-dev2-1-prfx1", "v6-bgpNet-dev2-1-prfx3", "v6-bgpNet-dev2-1-prfx4"},
		},
		{
			Name:      "port3ToMany",
			srcDevice: []string{"v6-bgpNet-dev3-1-prfx3"},
			dstDevice: []string{
				"v6-bgpNet-dev2-1-prfx2", "v6-bgpNet-dev2-1-prfx1", "v6-bgpNet-dev2-1-prfx4",
				"v6-bgpNet-dev1-1-prfx2", "v6-bgpNet-dev1-1-prfx1", "v6-bgpNet-dev1-1-prfx4",
				"v6-bgpNet-dev4-1-prfx2", "v6-bgpNet-dev4-1-prfx1", "v6-bgpNet-dev4-1-prfx4"},
			srcAddr:    []string{prefixV6Address3},
			dstAddr:    []string{prefixV6Address1, prefixV6Address2, prefixV6Address4},
			tcpSrcPort: 150,
			tcpDstPort: 2000,
			protocol:   "ipv6",
			withdrawBGPRoutes: []string{
				"v6-bgpNet-dev3-1-prfx2", "v6-bgpNet-dev3-1-prfx1", "v6-bgpNet-dev3-1-prfx4",
				"v6-bgpNet-dev2-1-prfx3", "v6-bgpNet-dev1-1-prfx3", "v6-bgpNet-dev4-1-prfx3"},
		},
		{
			Name:      "port4ToMany",
			srcDevice: []string{"v6-bgpNet-dev4-1-prfx4"},
			dstDevice: []string{
				"v6-bgpNet-dev2-1-prfx2", "v6-bgpNet-dev2-1-prfx3", "v6-bgpNet-dev2-1-prfx1",
				"v6-bgpNet-dev3-1-prfx2", "v6-bgpNet-dev3-1-prfx3", "v6-bgpNet-dev3-1-prfx1",
				"v6-bgpNet-dev1-1-prfx2", "v6-bgpNet-dev1-1-prfx3", "v6-bgpNet-dev1-1-prfx1"},
			srcAddr:    []string{prefixV6Address4},
			dstAddr:    []string{prefixV6Address2, prefixV6Address3, prefixV6Address1},
			tcpSrcPort: 150,
			tcpDstPort: 2000,
			protocol:   "ipv6",
			withdrawBGPRoutes: []string{
				"v6-bgpNet-dev4-1-prfx2", "v6-bgpNet-dev4-1-prfx3", "v6-bgpNet-dev4-1-prfx1",
				"v6-bgpNet-dev2-1-prfx4", "v6-bgpNet-dev3-1-prfx4", "v6-bgpNet-dev1-1-prfx4"},
		},
	}

	for _, flows := range flowList {

		flow := createFlow(
			flows.Name,
			flows.srcDevice,
			flows.dstDevice,
			flows.srcAddr,
			flows.dstAddr,
			flows.tcpSrcPort,
			flows.tcpDstPort,
			flows.protocol,
		)

		config.Flows().Append(flow)

		otgConfig.PushConfig(t, config)
		otgConfig.StartProtocols(t)
		time.Sleep(time.Second * 300)

		t.Logf("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, ate, 6*time.Minute)

		t.Logf("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, dut)

		withdrawBGPRoutes(t, config, flows.withdrawBGPRoutes)

		otgConfig.StartTraffic(t)
		time.Sleep(time.Second * 60)
		otgConfig.StopTraffic(t)

		// Verify Traffic
		otgutils.LogFlowMetrics(t, otgConfig, config)
		otgutils.LogPortMetrics(t, otgConfig, config)
		validateTrafficLoss(t, otgConfig, flows.Name)
		config.Flows().Clear()

	}

	// TODO: Validation of logging (Raised issue 416164360)
}

func testv4PrefixList(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig *otg.OTG, config gosnappi.Config) {
	// Remove the ACL configs from interface
	removeAClOnInterface(t, dut, "port1")
	removeAClOnInterface(t, dut, "port2")
	removeAClOnInterface(t, dut, "port3")
	removeAClOnInterface(t, dut, "port4")

	// Configure DUT
	configureDUT(t, dut, false)

	// Configure OTG
	configV4 := configureOTG(t, ate, true)
	otgConfig.PushConfig(t, configV4)

	// Configure ACL_IPV4_Match_using_prefix_list_prfxv4-1
	ports := "100-65500"
	srcPrefixAddress := []string{
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[1], prfxListSrcSubnetList[1]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[2], prfxListSrcSubnetList[2]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[3], prfxListSrcSubnetList[3]),
	}

	dstPrefixAddress := fmt.Sprintf("%s/%d", prfxListDstIpv4List[0], prfxListDstSubnet)

	configureTrafficPolicy(t, dut, "ACL_IPV4_Match_using_prefix_list_prfxv4-1", "ipv4", srcPrefixAddress, dstPrefixAddress, ports, ports, dut.Port(t, "port1").Name())

	// Configure ACL_IPV4_Match_using_prefix_list_prfxv4-2
	srcPort := "115, 215, 980, 1090, 8000"
	dstPort := "30000-45000"
	srcPrefixAddress = []string{
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[0], prfxListSrcSubnetList[0]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[2], prfxListSrcSubnetList[2]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[3], prfxListSrcSubnetList[3]),
	}

	dstPrefixAddress = fmt.Sprintf("%s/%d", prfxListDstIpv4List[1], prfxListDstSubnet)

	configureTrafficPolicy(t, dut, "ACL_IPV4_Match_using_prefix_list_prfxv4-2", "ipv4", srcPrefixAddress, dstPrefixAddress, srcPort, dstPort, dut.Port(t, "port2").Name())

	// Configure ACL_IPV4_Match_using_prefix_list_prfxv4-3
	srcPort = "280, 700, 1150, 5110, 1899"
	dstPort = "5000-10999"
	srcPrefixAddress = []string{
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[0], prfxListSrcSubnetList[0]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[1], prfxListSrcSubnetList[1]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[3], prfxListSrcSubnetList[3]),
	}

	dstPrefixAddress = fmt.Sprintf("%s/%d", prfxListDstIpv4List[2], prfxListDstSubnet)

	configureTrafficPolicy(t, dut, "ACL_IPV4_Match_using_prefix_list_prfxv4-3", "ipv4", srcPrefixAddress, dstPrefixAddress, srcPort, dstPort, dut.Port(t, "port3").Name())

	// Configure ACL_IPV4_Match_using_prefix_list_prfxv4-4
	srcPort = "50-100, 200-5000, 800-6550"
	dstPort = "80"
	srcPrefixAddress = []string{
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[0], prfxListSrcSubnetList[0]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[1], prfxListSrcSubnetList[1]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv4List[2], prfxListSrcSubnetList[3]),
	}

	dstPrefixAddress = fmt.Sprintf("%s/%d", prfxListDstIpv4List[3], prfxListDstSubnet)

	configureTrafficPolicy(t, dut, "ACL_IPV4_Match_using_prefix_list_prfxv4-4", "ipv4", srcPrefixAddress, dstPrefixAddress, srcPort, dstPort, dut.Port(t, "port4").Name())

	config.Flows().Clear()

	// Traffic from ATE port1 to other ports
	flow := createFlow(
		"prfxListPort1ToMany",
		[]string{"v4-bgpNetSrc-devSrc1-2"},
		[]string{"v4-bgpNetDst-devDst2-3", "v4-bgpNetDst-devDst3-3", "v4-bgpNetDst-devDst4-3"},
		[]string{prfxListSrcIpv4List[0]},
		[]string{prfxListDstIpv4List[1], prfxListDstIpv4List[2], prfxListDstIpv4List[3]},
		10000,
		20000,
		"ipv4")
	configV4.Flows().Append(flow)

	// Traffic from ATE port2 to other ports
	flow = createFlow(
		"prfxListPort2ToMany",
		[]string{"v4-bgpNetSrc-devSrc2-2"},
		[]string{
			"v4-bgpNetDst-devDst1-3", "v4-bgpNetDst-devDst3-3", "v4-bgpNetDst-devDst4-3"},
		[]string{prfxListSrcIpv4List[1]},
		[]string{prfxListDstIpv4List[0], prfxListDstIpv4List[2], prfxListDstIpv4List[3]},
		10000,
		20000,
		"ipv4")
	configV4.Flows().Append(flow)

	// Traffic from ATE port3 to other ports
	flow = createFlow(
		"prfxListPort3ToMany",
		[]string{"v4-bgpNetSrc-devSrc3-2"},
		[]string{"v4-bgpNetDst-devDst1-3", "v4-bgpNetDst-devDst2-3", "v4-bgpNetDst-devDst4-3"},
		[]string{prfxListSrcIpv4List[2]},
		[]string{prfxListDstIpv4List[0], prfxListDstIpv4List[1], prfxListDstIpv4List[3]},
		10000,
		20000,
		"ipv4")
	configV4.Flows().Append(flow)

	// Traffic from ATE port4 to other ports
	flow = createFlow(
		"prfxListPort4ToMany",
		[]string{"v4-bgpNetSrc-devSrc4-2"},
		[]string{"v4-bgpNetDst-devDst1-3", "v4-bgpNetDst-devDst2-3", "v4-bgpNetDst-devDst3-3"},
		[]string{prfxListSrcIpv4List[3]},
		[]string{prfxListDstIpv4List[0], prfxListDstIpv4List[1], prfxListDstIpv4List[2]},
		10000,
		20000,
		"ipv4")
	configV4.Flows().Append(flow)

	otgConfig.PushConfig(t, configV4)

	otgConfig.StartProtocols(t)

	otgutils.WaitForARP(t, otgConfig, configV4, "IPv4")
	otgutils.WaitForARP(t, otgConfig, configV4, "IPv6")

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate, 6*time.Minute)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	otgConfig.StartTraffic(t)
	time.Sleep(time.Second * 60)
	otgConfig.StopTraffic(t)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, otgConfig, configV4)
	otgutils.LogPortMetrics(t, otgConfig, configV4)
	for _, flowName := range []string{"prfxListPort1ToMany", "prfxListPort2ToMany", "prfxListPort3ToMany", "prfxListPort4ToMany"} {
		validateTrafficLoss(t, otgConfig, flowName)
	}
}

func testv6PrefixList(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig *otg.OTG, config gosnappi.Config) {
	// Configure OTG
	configV6 := configureOTG(t, ate, true)
	otgConfig.PushConfig(t, configV6)

	// Configure ACL_IPV6_Match_using_prefix_list_prfxv6-1
	ports := "100-65500"
	srcPrefixAddress := []string{
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[0], prfxListSrcV6SubnetList[0]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[1], prfxListSrcV6SubnetList[1]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[3], prfxListSrcV6SubnetList[3]),
	}
	dstPrefixAddress := fmt.Sprintf("%s/%d", prfxListDstIpv6List[1], prfxListDstV6Subnet)
	configureTrafficPolicy(t, dut, "ACL_IPV6_Match_using_prefix_list_prfxv6-1", "ipv6", srcPrefixAddress, dstPrefixAddress, ports, ports, dut.Port(t, "port1").Name())

	// Configure ACL_IPV6_Match_using_prefix_list_prfxv6-2
	srcPort := "115, 215, 980, 1090, 8000"
	dstPort := "30000-45000"
	srcPrefixAddress = []string{
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[0], prfxListSrcV6SubnetList[0]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[2], prfxListSrcV6SubnetList[2]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[3], prfxListSrcV6SubnetList[3]),
	}
	dstPrefixAddress = fmt.Sprintf("%s/%d", prfxListDstIpv6List[1], prfxListDstV6Subnet)
	configureTrafficPolicy(t, dut, "ACL_IPV6_Match_using_prefix_list_prfxv6-2", "ipv6", srcPrefixAddress, dstPrefixAddress, srcPort, dstPort, dut.Port(t, "port2").Name())

	// Configure ACL_IPV6_Match_using_prefix_list_prfxv6-3
	srcPort = "280, 700, 1150, 5110, 1899"
	dstPort = "5000-10999"
	srcPrefixAddress = []string{
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[0], prfxListSrcV6SubnetList[0]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[1], prfxListSrcV6SubnetList[1]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[3], prfxListSrcV6SubnetList[3]),
	}
	dstPrefixAddress = fmt.Sprintf("%s/%d", prfxListDstIpv6List[2], prfxListDstV6Subnet)
	configureTrafficPolicy(t, dut, "ACL_IPV6_Match_using_prefix_list_prfxv6-3", "ipv6", srcPrefixAddress, dstPrefixAddress, srcPort, dstPort, dut.Port(t, "port3").Name())

	// Configure ACL_IPV6_Match_using_prefix_list_prfxv6-4
	srcPort = "50-100, 200-5000, 800-6550"
	dstPort = "80"
	srcPrefixAddress = []string{
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[0], prfxListSrcV6SubnetList[0]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[1], prfxListSrcV6SubnetList[1]),
		fmt.Sprintf("%s/%d", prfxListSrcIpv6List[2], prfxListSrcV6SubnetList[2]),
	}
	dstPrefixAddress = fmt.Sprintf("%s/%d", prfxListDstIpv6List[3], prfxListDstV6Subnet)
	configureTrafficPolicy(t, dut, "ACL_IPV6_Match_using_prefix_list_prfxv6-4", "ipv6", srcPrefixAddress, dstPrefixAddress, srcPort, dstPort, dut.Port(t, "port4").Name())

	configV6.Flows().Clear()

	// Traffic from ATE port1 to other ports
	flow := createFlow(
		"prfxv6ListPort1ToMany",
		[]string{"v6-bgpNetSrc-devSrc1-2"},
		[]string{"v6-bgpNetDst-devDst2-3", "v6-bgpNetDst-devDst3-3", "v6-bgpNetDst-devDst4-3"},
		[]string{prfxListSrcIpv6List[0]},
		[]string{prfxListDstIpv6List[1], prfxListDstIpv6List[2], prfxListDstIpv6List[3]},
		100,
		2000,
		"ipv6",
	)
	configV6.Flows().Append(flow)

	// Traffic from ATE port2 to other ports
	flow = createFlow(
		"prfxv6ListPort2ToMany",
		[]string{"v6-bgpNetSrc-devSrc2-2"},
		[]string{"v6-bgpNetDst-devDst1-3", "v6-bgpNetDst-devDst3-3", "v6-bgpNetDst-devDst4-3"},
		[]string{prfxListSrcIpv6List[1]},
		[]string{prfxListDstIpv6List[0], prfxListDstIpv6List[2], prfxListDstIpv6List[3]},
		100,
		2000,
		"ipv6",
	)
	configV6.Flows().Append(flow)

	// Traffic from ATE port3 to other ports
	flow = createFlow(
		"prfxv6ListPort3ToMany",
		[]string{"v6-bgpNetSrc-devSrc3-2"},
		[]string{"v6-bgpNetDst-devDst1-3", "v6-bgpNetDst-devDst2-3", "v6-bgpNetDst-devDst4-3"},
		[]string{prfxListSrcIpv6List[2]},
		[]string{prfxListDstIpv6List[0], prfxListDstIpv6List[1], prfxListDstIpv6List[3]},
		100,
		2000,
		"ipv6",
	)
	configV6.Flows().Append(flow)

	// Traffic from ATE port4 to other ports
	flow = createFlow(
		"prfxv6ListPort4ToMany",
		[]string{"v6-bgpNetSrc-devSrc4-2"},
		[]string{"v6-bgpNetDst-devDst1-3", "v6-bgpNetDst-devDst2-3", "v6-bgpNetDst-devDst3-3"},
		[]string{prfxListSrcIpv6List[3]},
		[]string{prfxListDstIpv6List[0], prfxListDstIpv6List[1], prfxListDstIpv6List[2]},
		100,
		2000,
		"ipv6",
	)
	configV6.Flows().Append(flow)

	otgConfig.PushConfig(t, configV6)

	otgConfig.StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	otgConfig.StartTraffic(t)
	time.Sleep(time.Second * 60)
	otgConfig.StopTraffic(t)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, otgConfig, configV6)
	otgutils.LogPortMetrics(t, otgConfig, configV6)
	for _, flowName := range []string{"prfxv6ListPort1ToMany", "prfxv6ListPort2ToMany", "prfxv6ListPort3ToMany", "prfxv6ListPort4ToMany"} {
		validateTrafficLoss(t, otgConfig, flowName)
	}
}
