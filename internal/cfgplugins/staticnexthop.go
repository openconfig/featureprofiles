package cfgplugins

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

var (
	nextHopGroupConfigIPV4Arista = `
	nexthop-group 1V4_vlan_3_20 type mpls-over-gre
   tos 96 
   ttl 64 
   fec hierarchical
   entry 0 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.208
   entry 1 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.209
   entry 2 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.210
   entry 3 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.211
   entry 4 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.212
   entry 5 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.213
   entry 6 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.215
   entry 7 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.216
   entry 8 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.217
   entry 9 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.218
   entry 10 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.219
   entry 11 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.220
   entry 12 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.221
   entry 13 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.222
   entry 14 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.223
   entry 15 push label-stack 116383 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.224
!
`
	nextHopGroupConfigIPV6Arista = `
nexthop-group 1V6_vlan_3_21 type mpls-over-gre
tos 96 
ttl 64 
fec hierarchical
entry 0 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.208
entry 1 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.209
entry 2 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.210
entry 3 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.211
entry 4 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.212
entry 5 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.213
entry 6 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.215
entry 7 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.216
entry 8 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.217
entry 9 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.218
entry 10 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.219
entry 11 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.220
entry 12 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.221
entry 13 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.222
entry 14 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.223
entry 15 push label-stack 99999 tunnel-destination 10.99.1.2 tunnel-source 10.235.143.224
!
`
	nextHopGroupConfigDualStackIPV4Arista = `
nexthop-group 1V4_vlan_3_22 type mpls-over-gre
tos 96 
ttl 64 
fec hierarchical
entry 0 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.208
entry 1 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.209
entry 2 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.210
entry 3 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.211
entry 4 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.212
entry 5 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.213
entry 6 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.215
entry 7 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.216
entry 8 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.217
entry 9 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.218
entry 10 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.219
entry 11 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.220
entry 12 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.221
entry 13 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.222
entry 14 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.223
entry 15 push label-stack 362143 tunnel-destination 10.99.1.3 tunnel-source 10.235.143.224
!
   `
	nextHopGroupConfigDualStackIPV6Arista = `
   nexthop-group 1V6_vlan_3_22 type mpls-over-gre
   tos 96 
   ttl 64 
   fec hierarchical
   entry 0 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.208
   entry 1 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.209
   entry 2 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.210
   entry 3 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.211
   entry 4 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.212
   entry 5 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.213
   entry 6 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.215
   entry 7 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.216
   entry 8 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.217
   entry 9 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.218
   entry 10 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.219
   entry 11 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.220
   entry 12 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.221
   entry 13 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.222
   entry 14 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.223
   entry 15 push label-stack 899999 tunnel-destination 10.99.1.4 tunnel-source 10.235.143.224
!
  
	`
	// nextHopGroupConfigMulticloudIPV4Arista : Arista specific configuration for next-hop-group for multicloud ipv4.
	nextHopGroupConfigMulticloudIPV4Arista = `
	 nexthop-group 1V4_vlan_3_23 type mpls-over-gre
   tos 96 
   ttl 64 
   fec hierarchical
   entry 0 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.208
   entry 1 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.209
   entry 2 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.210
   entry 3 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.211
   entry 4 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.212
   entry 5 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.213
   entry 6 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.215
   entry 7 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.216
   entry 8 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.217
   entry 9 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.218
   entry 10 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.219
   entry 11 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.220
   entry 12 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.221
   entry 13 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.222
   entry 14 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.223
   entry 15 push label-stack 965535 tunnel-destination 10.99.1.5 tunnel-source 10.235.143.224
!
	`
)

// NextHopGroupConfig configures the interface next-hop-group config.
func NextHopGroupConfig(t *testing.T, dut *ondatra.DUTDevice, traffictype string, ni *oc.NetworkInstance, params StaticNextHopGroupParams) {
	t.Helper()
	if deviations.NextHopGroupOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if traffictype == "v4" {
				if params.DynamicVal {
					for _, dynamicValues := range params.DynamicValues {
						nextHopGroupConfigIPV4AristaDyn := fmt.Sprintf(`
						nexthop-group %s type %s
						ttl %d
						entry  %d push label-stack %d tunnel-destination %s tunnel-source %s					
						`, dynamicValues.NexthopGrpName, dynamicValues.NexthopType, dynamicValues.TTL,
							dynamicValues.EntryValue, dynamicValues.MplsLabel,
							dynamicValues.TunnelDst, dynamicValues.TunnelSrc)
						helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigIPV4AristaDyn)
					}
				} else {
					helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigIPV4Arista)
				}
			} else if traffictype == "dualstack" {
				helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigDualStackIPV4Arista)
				helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigDualStackIPV6Arista)
			} else if traffictype == "v6" {
				if params.DynamicVal {
					for _, dynamicValues := range params.DynamicValues {
						nextHopGroupConfigIPV4AristaDyn := fmt.Sprintf(`
						nexthop-group %s type %s
						ttl %d
						entry  %d push label-stack %d tunnel-destination %s tunnel-source %s					
						`, dynamicValues.NexthopGrpName, dynamicValues.NexthopType, dynamicValues.TTL,
							dynamicValues.EntryValue, dynamicValues.MplsLabel,
							dynamicValues.TunnelDst, dynamicValues.TunnelSrc)
						helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigIPV4AristaDyn)
					}
				} else {
					helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigIPV6Arista)
				}
			} else if traffictype == "multicloudv4" {
				helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigMulticloudIPV4Arista)
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'next-hop-group config'", dut.Vendor())
		}
	} else {
		configureNextHopGroups(t, ni, params)
	}
}

// StaticNextHopGroupParams holds parameters for generating the OC Static Next Hop Group config.
type StaticNextHopGroupParams struct {

	// For the "MPLS_in_GRE_Encap" Next-Hop Group definition from JSON's "static" block
	StaticNHGName string
	NHIPAddr1     string
	NHIPAddr2     string
	// OuterIpv4Src*Def / OuterIpv4DstDef are used only when DynamicVal == false.
	// When DynamicVal == true, tunnel src/dst must be provided per-entry via DynamicStructParams and these fields are ignored.
	OuterIpv4DstDef  string
	OuterIpv4Src1Def string
	OuterIpv4Src2Def string
	OuterDscpDef     uint8
	OuterTTLDef      uint8
	DynamicValues    []DynamicStructParams
	DynamicVal       bool
	// TODO: b/417988636 - Set the MplsLabel to the correct value.

}

type DynamicStructParams struct {
	NexthopGrpName string
	NexthopType    string
	TTL            int
	TunnelSrc      string
	TunnelDst      string
	MplsLabel      int
	EntryValue     int
}

// NexthopGroupUDPParams defines the parameters used to create or configure a Next Hop Group that performs UDP encapsulation for traffic forwarding.
type NexthopGroupUDPParams struct {
	IPFamily           string // IPFamily specifies the IP address family for encapsulation. For example, "V4Udp" for IPv4-over-UDP or "V6Udp" for IPv6-over-UDP.
	NexthopGrpName     string
	DstIp              []string
	SrcIp              string
	DstUdpPort         uint16
	SrcUdpPort         uint16
	TTL                uint8
	DSCP               uint8
	NetworkInstanceObj *oc.NetworkInstance
}

// configureNextHopGroups configures the next-hop groups and their encapsulation headers.
func configureNextHopGroups(t *testing.T, ni *oc.NetworkInstance, params StaticNextHopGroupParams) {
	t.Helper()
	nhg := ni.GetOrCreateStatic().GetOrCreateNextHopGroup("MPLS_in_GRE_Encap")
	nhg.GetOrCreateNextHop("Dest A-NH1").Index = ygot.String("Dest A-NH1")
	nhg.GetOrCreateNextHop("Dest A-NH2").Index = ygot.String("Dest A-NH2")
	ni.GetOrCreateStatic().GetOrCreateNextHop("Dest A-NH1").NextHop = oc.UnionString(params.NHIPAddr1)
	ni.GetOrCreateStatic().GetOrCreateNextHop("Dest A-NH2").NextHop = oc.UnionString(params.NHIPAddr2)

	// Set the encap header for each next-hop
	ueh1 := ni.GetOrCreateStatic().GetOrCreateNextHop("Dest A-NH1").GetOrCreateEncapHeader(1)
	ueh1.GetOrCreateUdpV4().DstIp = ygot.String(params.OuterIpv4DstDef)
	ueh1.GetOrCreateUdpV4().SrcIp = ygot.String(params.OuterIpv4Src1Def)
	ueh1.GetOrCreateUdpV4().Dscp = ygot.Uint8(params.OuterDscpDef)
	ueh1.GetOrCreateUdpV4().IpTtl = ygot.Uint8(params.OuterTTLDef)

	// TODO: b/417988636 -  mpls to equal params.MplsLabel
	// if params.MplsLabel != nil && len(params.MplsLabel) > 0 {
	// 	ueh1.GetOrCreateMpls().Label = ygot.Uint32(100)

	ueh2 := ni.GetOrCreateStatic().GetOrCreateNextHop("Dest A-NH2").GetOrCreateEncapHeader(1)
	ueh2.GetOrCreateUdpV4().DstIp = ygot.String(params.OuterIpv4DstDef)
	ueh2.GetOrCreateUdpV4().SrcIp = ygot.String(params.OuterIpv4Src2Def)
	ueh2.GetOrCreateUdpV4().Dscp = ygot.Uint8(params.OuterDscpDef)
	ueh2.GetOrCreateUdpV4().IpTtl = ygot.Uint8(params.OuterTTLDef)

	// TODO: b/417988636 -  mpls to equal params.MplsLabel
	// if params.MplsLabel != nil && len(params.MplsLabel) > 0 {
	// 	ueh2.GetOrCreateMpls().Label = ygot.Uint32(100)
}

// NextHopGroupConfigForMulticloud configures the interface next-hop-group config for multicloud.
func NextHopGroupConfigForMulticloud(t *testing.T, dut *ondatra.DUTDevice, traffictype string, ni *oc.NetworkInstance, params StaticNextHopGroupParams) {
	if deviations.NextHopGroupOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if traffictype == "multicloudv4" {
				helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigMulticloudIPV4Arista)
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'next-hop-group config'", dut.Vendor())
		}
	} else {
		configureNextHopGroups(t, ni, params)
	}
}

// NextHopGroupConfigForIpOverUdp configures the interface next-hop-group config for ip over udp.
func NextHopGroupConfigForIpOverUdp(t *testing.T, dut *ondatra.DUTDevice, params NexthopGroupUDPParams) {
	t.Helper()
	if deviations.NextHopGroupOCUnsupported(dut) {
		var cli string
		var groupType string
		switch dut.Vendor() {
		case ondatra.ARISTA:
			switch params.IPFamily {
			case "V4Udp":
				groupType = "ipv4-over-udp"
			case "V6Udp":
				groupType = "ipv6-over-udp"
			default:
				t.Fatalf("Unsupported address family type %q", params.IPFamily)
			}
			if len(params.DstIp) > 0 {
				var tunnelDst string
				for i, addr := range params.DstIp {
					tunnelDst += fmt.Sprintf("entry %d tunnel-destination %s \n", i, addr)
				}
				cli = fmt.Sprintf(`
					nexthop-group %s type %s
					tunnel-source %s
					fec hierarchical
   					%s
					`, params.NexthopGrpName, groupType, params.SrcIp, tunnelDst)
				helpers.GnmiCLIConfig(t, dut, cli)
			}
			if params.TTL != 0 {
				cli = fmt.Sprintf(`
					nexthop-group %s type %s
					ttl %v
					`, params.NexthopGrpName, groupType, params.TTL)
				helpers.GnmiCLIConfig(t, dut, cli)
			}

			if params.DSCP != 0 {
				cli = fmt.Sprintf(`
					nexthop-group %s type %s
					tos %v
					`, params.NexthopGrpName, groupType, params.DSCP)
				helpers.GnmiCLIConfig(t, dut, cli)
			}

			if params.DstUdpPort != 0 {
				// Select and apply the appropriate CLI snippet based on 'traffictype'.
				cli = fmt.Sprintf(`tunnel type %s udp destination port %v`, groupType, params.DstUdpPort)
				helpers.GnmiCLIConfig(t, dut, cli)
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'next-hop-group config'", dut.Vendor())
		}
	} else {
		t.Helper()
		nhg := params.NetworkInstanceObj.GetOrCreateStatic().GetOrCreateNextHopGroup(params.NexthopGrpName)
		nhg.GetOrCreateNextHop("Dest A-NH1").Index = ygot.String("Dest A-NH1")

		// Set the encap header for each next-hop
		ueh1 := params.NetworkInstanceObj.GetOrCreateStatic().GetOrCreateNextHop("Dest A-NH1").GetOrCreateEncapHeader(1)
		for _, addr := range params.DstIp {
			ueh1.GetOrCreateUdpV4().DstIp = ygot.String(addr)
		}
		if params.TTL != 0 {
			ueh1.GetOrCreateUdpV4().IpTtl = ygot.Uint8(params.TTL)
		}
		ueh1.GetOrCreateUdpV4().SetSrcIp(params.SrcIp)
		ueh1.GetOrCreateUdpV4().SetDscp(params.DSCP)
		ueh1.GetOrCreateUdpV4().SetDstUdpPort(params.DstUdpPort)
		ueh1.GetOrCreateUdpV4().SetSrcUdpPort(params.SrcUdpPort)
	}
}
