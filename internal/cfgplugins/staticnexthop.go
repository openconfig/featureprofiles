package cfgplugins

import (
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

	nextHopGroupGreConfigIPV4Arista = `
nexthop-group gre_ecmp type gre
   ttl 64
   fec hierarchical
   entry  0 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.150
   entry  1 tunnel-destination 10.99.2.1 tunnel-source 10.235.143.151
   entry  2 tunnel-destination 10.99.3.1 tunnel-source 10.235.143.152
   entry  3 tunnel-destination 10.99.4.1 tunnel-source 10.235.143.153
   entry  4 tunnel-destination 10.99.5.1 tunnel-source 10.235.143.154
   entry  5 tunnel-destination 10.99.6.1 tunnel-source 10.235.143.155
   entry  6 tunnel-destination 10.99.7.1 tunnel-source 10.235.143.156
   entry  7 tunnel-destination 10.99.8.1 tunnel-source 10.235.143.157
   entry  8 tunnel-destination 10.99.9.1 tunnel-source 10.235.143.158
   entry  9 tunnel-destination 10.99.10.1 tunnel-source 10.235.143.159
   entry  10 tunnel-destination 10.99.11.1 tunnel-source 10.235.143.160
   entry  11 tunnel-destination 10.99.12.1 tunnel-source 10.235.143.161
   entry  12 tunnel-destination 10.99.13.1 tunnel-source 10.235.143.162
   entry  13 tunnel-destination 10.99.14.1 tunnel-source 10.235.143.163
   entry  14 tunnel-destination 10.99.15.1 tunnel-source 10.235.143.164
   entry  15 tunnel-destination 10.99.16.1 tunnel-source 10.235.143.165
   entry  16 tunnel-destination 10.99.17.1 tunnel-source 10.235.143.166
   entry  17 tunnel-destination 10.99.18.1 tunnel-source 10.235.143.167
   entry  18 tunnel-destination 10.99.19.1 tunnel-source 10.235.143.168
   entry  19 tunnel-destination 10.99.20.1 tunnel-source 10.235.143.169
   entry  20 tunnel-destination 10.99.21.1 tunnel-source 10.235.143.170
   entry  21 tunnel-destination 10.99.22.1 tunnel-source 10.235.143.171
   entry  22 tunnel-destination 10.99.23.1 tunnel-source 10.235.143.172
   entry  23 tunnel-destination 10.99.24.1 tunnel-source 10.235.143.173
   entry  24 tunnel-destination 10.99.25.1 tunnel-source 10.235.143.174
   entry  25 tunnel-destination 10.99.26.1 tunnel-source 10.235.143.175
   entry  26 tunnel-destination 10.99.27.1 tunnel-source 10.235.143.176
   entry  27 tunnel-destination 10.99.28.1 tunnel-source 10.235.143.177
   entry  28 tunnel-destination 10.99.29.1 tunnel-source 10.235.143.180
   entry  29 tunnel-destination 10.99.30.1 tunnel-source 10.235.143.181
   entry  30 tunnel-destination 10.99.31.1 tunnel-source 10.235.143.182
   entry  31 tunnel-destination 10.99.32.1 tunnel-source 10.235.143.183
!
`
	nextHopGroupGreConfigIPV6Arista = `
nexthop-group gre_ecmp_v6 type gre
   ttl 64
   fec hierarchical
   entry  0 tunnel-destination 10.99.1.1 tunnel-source 10.235.143.150
   entry  1 tunnel-destination 10.99.2.1 tunnel-source 10.235.143.151
   entry  2 tunnel-destination 10.99.3.1 tunnel-source 10.235.143.152
   entry  3 tunnel-destination 10.99.4.1 tunnel-source 10.235.143.153
   entry  4 tunnel-destination 10.99.5.1 tunnel-source 10.235.143.154
   entry  5 tunnel-destination 10.99.6.1 tunnel-source 10.235.143.155
   entry  6 tunnel-destination 10.99.7.1 tunnel-source 10.235.143.156
   entry  7 tunnel-destination 10.99.8.1 tunnel-source 10.235.143.157
   entry  8 tunnel-destination 10.99.9.1 tunnel-source 10.235.143.158
   entry  9 tunnel-destination 10.99.10.1 tunnel-source 10.235.143.159
   entry  10 tunnel-destination 10.99.11.1 tunnel-source 10.235.143.160
   entry  11 tunnel-destination 10.99.12.1 tunnel-source 10.235.143.161
   entry  12 tunnel-destination 10.99.13.1 tunnel-source 10.235.143.162
   entry  13 tunnel-destination 10.99.14.1 tunnel-source 10.235.143.163
   entry  14 tunnel-destination 10.99.15.1 tunnel-source 10.235.143.164
   entry  15 tunnel-destination 10.99.16.1 tunnel-source 10.235.143.165
   entry  16 tunnel-destination 10.99.17.1 tunnel-source 10.235.143.166
   entry  17 tunnel-destination 10.99.18.1 tunnel-source 10.235.143.167
   entry  18 tunnel-destination 10.99.19.1 tunnel-source 10.235.143.168
   entry  19 tunnel-destination 10.99.20.1 tunnel-source 10.235.143.169
   entry  20 tunnel-destination 10.99.21.1 tunnel-source 10.235.143.170
   entry  21 tunnel-destination 10.99.22.1 tunnel-source 10.235.143.171
   entry  22 tunnel-destination 10.99.23.1 tunnel-source 10.235.143.172
   entry  23 tunnel-destination 10.99.24.1 tunnel-source 10.235.143.173
   entry  24 tunnel-destination 10.99.25.1 tunnel-source 10.235.143.174
   entry  25 tunnel-destination 10.99.26.1 tunnel-source 10.235.143.175
   entry  26 tunnel-destination 10.99.27.1 tunnel-source 10.235.143.176
   entry  27 tunnel-destination 10.99.28.1 tunnel-source 10.235.143.177
   entry  28 tunnel-destination 10.99.29.1 tunnel-source 10.235.143.180
   entry  29 tunnel-destination 10.99.30.1 tunnel-source 10.235.143.181
   entry  30 tunnel-destination 10.99.31.1 tunnel-source 10.235.143.182
   entry  31 tunnel-destination 10.99.32.1 tunnel-source 10.235.143.183
!
`
)

// NextHopGroupConfig configures the interface next-hop-group config.
func NextHopGroupConfig(t *testing.T, dut *ondatra.DUTDevice, traffictype string, ni *oc.NetworkInstance, params StaticNextHopGroupParams) {
	if deviations.NextHopGroupOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if params.StaticNHGName == "GRE_Encap" {
				if traffictype == "dualstack" {
					// TODO: Change this hard-coded values
					helpers.GnmiCLIConfig(t, dut, nextHopGroupGreConfigIPV4Arista)
					helpers.GnmiCLIConfig(t, dut, nextHopGroupGreConfigIPV6Arista)
				}
			} else {
				if traffictype == "v4" {
					helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigIPV4Arista)
				} else if traffictype == "dualstack" {
					helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigDualStackIPV4Arista)
					helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigDualStackIPV6Arista)
				} else if traffictype == "v6" {
					helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigIPV6Arista)
				} else if traffictype == "multicloudv4" {
					helpers.GnmiCLIConfig(t, dut, nextHopGroupConfigMulticloudIPV4Arista)
				}
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
	StaticNHGName    string
	NHIPAddr1        string
	NHIPAddr2        string
	OuterIpv4DstDef  string
	OuterIpv4Src1Def string
	OuterIpv4Src2Def string
	OuterDscpDef     uint8
	OuterTTLDef      uint8

	// TODO: b/417988636 - Set the MplsLabel to the correct value.

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
