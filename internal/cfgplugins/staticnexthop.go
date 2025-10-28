package cfgplugins

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// OCEncapsulationParams holds parameters for generating the OC Encapsulations config.
type OCEncapsulationParams struct {
	Count                        int
	MPLSLabelCount               int
	MPLSStaticLabels             []int
	MPLSStaticLabelsForIPv6      []int
	MPLSLabelStartForIPv4        int
	MPLSLabelStartForIPv6        int
	MPLSLabelStep                int
	GRETunnelSources             []string
	GRETunnelDestinationsStartIP string
}

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
	if deviations.NextHopGroupOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
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
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'next-hop-group config'", dut.Vendor())
		}
	} else {
		configureNextHopGroups(t, ni, params)
	}

}

// NextHopGroupConfigScale configures the interface next-hop-group config.
func NextHopGroupConfigScale(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, encapparams OCEncapsulationParams, ni *oc.NetworkInstance) *gnmi.SetBatch {
	if deviations.NextHopGroupOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			n, _, err := net.ParseCIDR(encapparams.GRETunnelDestinationsStartIP)
			if err != nil {
				t.Fatalf("invalid IP address %q provided in GRETunnelDestinationsStartIP - err: %v", encapparams.GRETunnelDestinationsStartIP, err)
			}
			buildConfig := func(prefix string, labels []int) string {
				b := new(strings.Builder)
				for i := 1; i <= encapparams.Count; i++ {
					ip := incrementIP(n, i-1)
					ipPrefix := ip.String()
					fmt.Fprintf(b, `
nexthop-group %s%d type mpls-over-gre
 tos 96
 ttl 64
 fec hierarchical`, prefix, i)
					for entry, src := range encapparams.GRETunnelSources {
						fmt.Fprintf(b, `
 entry  %d push label-stack %d tunnel-destination %s tunnel-source %s`,
							entry, labels[i-1], ipPrefix, src)
					}
					b.WriteString("\n!")
				}
				return b.String()
			}

			helpers.GnmiCLIConfig(t, dut, buildConfig("1V4_vlan_3_", encapparams.MPLSStaticLabels))
			helpers.GnmiCLIConfig(t, dut, buildConfig("1V6_vlan_3_", encapparams.MPLSStaticLabelsForIPv6))

		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'next-hop-group config'", dut.Vendor())
		}
	} else {

		greTunnelDestinations := iputil.GenerateIPs(encapparams.GRETunnelDestinationsStartIP, encapparams.Count)

		for i := 1; i <= encapparams.Count; i++ {

			nhg := ni.GetOrCreateStatic().GetOrCreateNextHopGroup("MPLS_in_GRE_Encap")
			nhg.GetOrCreateNextHop("Dest A-NH1").SetIndex("Dest A-NH1")
			ni.GetOrCreateStatic().
				GetOrCreateNextHop("Dest A-NH1").
				SetNextHop(oc.UnionString(fmt.Sprintf("%s.%d", greTunnelDestinations[i], i)))

			eh := ni.GetOrCreateStatic().GetOrCreateNextHop("Dest A-NH1").GetOrCreateEncapHeader(1)
			ueh := eh.GetOrCreateUdpV4()
			ueh.SetSrcIp(encapparams.GRETunnelSources[i])
			ueh.SetDstIp(greTunnelDestinations[i])

			// https://partnerissuetracker.corp.google.com/issues/417988636
			// ueh.GetOrCreateMpls().Label.Set(encapparams.MPLSStaticLabels[i])
		}
		gnmi.BatchUpdate(sb, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni)
	}
	return sb
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

// incrementIP takes an IPv4 address and increments it by a given integer value.
// It returns a new net.IP object representing the incremented IP address.
func incrementIP(ip net.IP, inc int) net.IP {
	ip = ip.To4()
	newIP := make(net.IP, len(ip))
	copy(newIP, ip)
	for i := len(newIP) - 1; i >= 0; i-- {
		inc += int(newIP[i])
		newIP[i] = byte(inc % 256)
		inc /= 256
	}
	return newIP
}
