package cfgplugins

import (
	"fmt"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"testing"
)

const (
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
)

var (
	top                          = gosnappi.NewConfig()
	nextHopGroupConfigIPV4Arista = `
	nexthop-group 1V4_baybridge_vlan_3_20 type mpls-over-gre
surajrawal
Remove extra space
sancheetaroy
Apr 21, 5:21 PM
Done.
Resolved
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
nexthop-group 1V6_baybridge_vlan_3_21 type mpls-over-gre
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
nexthop-group 1V4_baybridge_vlan_3_22 type mpls-over-gre
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
   nexthop-group 1V6_baybridge_vlan_3_22 type mpls-over-gre
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
	nextHopGroupConfigMulticloudIPV4Arista = `
	 nexthop-group 1V4_baybridge_vlan_3_23 type mpls-over-gre
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
	PolicyForwardingConfigv4Arista = `
Golint
comments
Apr 21, 5:20 PM
exported var PolicyForwardingConfigv4Arista should have comment or be unexported

go/go-style/decisions#doc-comments
AI-fix contributed by FixbotFindingsRepair:Golint
Actionable
Was this helpful? 
Traffic-policies
   traffic-policy tp_cloud_id_3_20
      match bgpsetttlv4 ipv4
         ttl 1
         actions
            redirect next-hop group 1V4_baybridge_vlan_3_20 ttl 1
            set traffic class 3
      match icmpechov4 ipv4
         destination prefix 169.254.0.11/32
         protocol icmp type echo-reply code all
      match ipv4-all-default ipv4
         actions
            redirect next-hop group 1V4_baybridge_vlan_3_20
            set traffic class 3
      match ipv6-all-default ipv6
   !
	 `
	PolicyForwardingConfigv6Arista = `
Traffic-policies
    traffic-policy tp_cloud_id_3_21
    match bgpsetttlv6 ipv6
       ttl 1
       !
       actions
          count
          redirect next-hop group 1V6_baybridge_vlan_3_21 ttl 1
          set traffic class 3
    !
    match icmpv6 ipv6
       destination prefix 2600:2d00:0:1:8000:10:0:ca33/128
       protocol icmpv6 type echo-reply neighbor-advertisement code all
       !
       actions
          count
    !
    match ipv4-all-default ipv4
    !
    match ipv6-all-default ipv6
       actions
          count
          redirect next-hop group 1V6_baybridge_vlan_3_21
          set traffic class 3
 !
    `
	PolicyForwardingConfigDualStackArista = `
   Traffic-policies
    traffic-policy tp_cloud_id_3_22
    match bgpsetttlv6 ipv6
       ttl 1
       !
       actions
          count
          redirect next-hop group 1V6_baybridge_vlan_3_22 ttl 1
          set traffic class 3
    !
    match icmpv6 ipv6
       destination prefix 2600:2d00:0:1:7000:10:0:ca33/128
       protocol icmpv6 type echo-reply neighbor-advertisement code all
       !
       actions
          count
    !
    match bgpsetttlv4 ipv4
       ttl 1
       !
       actions
          count
          redirect next-hop group 1V4_baybridge_vlan_3_22 ttl 1
          set traffic class 3
    !
    match icmpechov4 ipv4
       destination prefix 169.254.0.27/32
       protocol icmp type echo-reply code all
       !
       actions
          count
    !
    match ipv4-all-default ipv4
       actions
          count
          redirect next-hop group 1V4_baybridge_vlan_3_22
          set traffic class 3
    !
    match ipv6-all-default ipv6
       actions
          count
          redirect next-hop group 1V6_baybridge_vlan_3_22
          set traffic class 3
 !`
	PolicyForwardingConfigMulticloudAristav4 = `
 Traffic-policies
 counter interface per-interface ingress
 !
 traffic-policy tp_cloud_id_3_23
		match icmpechov4 ipv4
			 destination prefix 169.254.0.33/32
			 protocol icmp type echo-reply code all
			 !
			 actions
					count
		!
		match bgpsetttlv4 ipv4
			 ttl 1
			 !
			 actions
					count
					redirect next-hop group 1V4_baybridge_vlan_3_23 ttl 1
					set traffic class 3
		!
		match ipv4-all-default ipv4
			 actions
					count
					redirect next-hop group 1V4_baybridge_vlan_3_23
					set traffic class 3
		!
		match ipv6-all-default ipv6
 !
`
	qosconfigArista = `
 qos map dscp 0 1 2 3 4 5 6 7 to traffic-class 0
 qos map dscp 8 9 10 11 12 13 14 15 to traffic-class 1
 qos map dscp 40 41 42 43 44 45 46 47 to traffic-class 4
 qos map dscp 48 49 50 51 52 53 54 55 to traffic-class 7
!
 policy-map type quality-of-service af3
   class class-default
      set traffic-class 3
!
`
	mplsLabelRangeArista = `
mpls label range bgp-sr 16 0
mpls label range dynamic 16 0
mpls label range isis-sr 16 0
mpls label range l2evpn 16 0
mpls label range l2evpn ethernet-segment 16 0
mpls label range ospf-sr 16 0
mpls label range srlb 16 0
mpls label range static 16 1048560
!
`
)

// InterfacelocalProxyConfig configures the interface local-proxy-arp.
func InterfacelocalProxyConfig(t *testing.T, dut *ondatra.DUTDevice, a *attrs.Attributes, aggID string) {
	if deviations.LocalProxyUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if a.IPv4 != "" {
				helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d \n ip local-proxy-arp \n", aggID, a.Subinterface))
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'local-proxy-arp'", dut.Vendor())
		}
	}
}

// InterfaceQosClassificationConfig configures the interface qos classification.
func InterfaceQosClassificationConfig(t *testing.T, dut *ondatra.DUTDevice, a *attrs.Attributes, aggID string) {
	if deviations.QosClassificationUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d \n service-policy type qos input af3 \n", aggID, a.Subinterface))
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'qos classification'", dut.Vendor())
		}
	}
}

// InterfacePolicyForwardingConfig configures the interface policy-forwarding.
func InterfacePolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, a *attrs.Attributes, aggID string) {
	if deviations.InterfacePolicyForwardingUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d \n traffic-policy input tp_cloud_id_3_%d \n", aggID, a.Subinterface, a.Subinterface))
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
	}
}

// MplsConfig configures the interface mpls.
func MplsConfig(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.MplsUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, "mpls ip")
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'mpls ip'", dut.Vendor())
		}
	}
}

// QosClassificationConfig configures the interface qos classification.
func QosClassificationConfig(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.QosClassificationUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, qosconfigArista)
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'qos classification'", dut.Vendor())
		}
	}
}

// LabelRangeConfig configures the interface label range.
func LabelRangeConfig(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.MplsLabelClassificationUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, mplsLabelRangeArista)
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'mpls label range'", dut.Vendor())
		}
	}
}

// NextHopGroupConfig configures the interface next-hop-group config.
func NextHopGroupConfig(t *testing.T, dut *ondatra.DUTDevice, traffictype string) {
	if deviations.NextHopGroupConfigUnsupported(dut) {
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
	}
}

// PolicyForwardingConfig configures the interface traffic-policy.
func PolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, traffictype string) {
	if deviations.PolicyForwardingUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if traffictype == "v4" {
				helpers.GnmiCLIConfig(t, dut, PolicyForwardingConfigv4Arista)
			} else if traffictype == "v6" {
				helpers.GnmiCLIConfig(t, dut, PolicyForwardingConfigv6Arista)
			} else if traffictype == "dualstack" {
				helpers.GnmiCLIConfig(t, dut, PolicyForwardingConfigDualStackArista)
			} else if traffictype == "multicloudv4" {
				helpers.GnmiCLIConfig(t, dut, PolicyForwardingConfigMulticloudAristav4)
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
	}
}
