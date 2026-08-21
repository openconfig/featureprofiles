package cfgplugins

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	ethertypeIPv4 = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6 = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	seqIDBase     = uint32(10)
)

// DecapPolicyParams defines parameters for the Decap MPLS in GRE policy and related MPLS configs.
type DecapPolicyParams struct {
	PolicyID                  string
	RuleSeqID                 uint32
	IPv4DestAddress           string // For the match criteria in the decap rule
	MPLSInterfaceID           string // For MPLS global interface attributes (e.g., "Aggregate4")
	StaticLSPNameIPv4         string
	StaticLSPLabelIPv4        uint32
	StaticLSPNextHopIPv4      string
	StaticLSPNameIPv6         string
	StaticLSPLabelIPv6        uint32
	StaticLSPNextHopIPv6      string
	StaticLSPNameMulticast    string
	StaticLSPLabelMulticast   uint32
	StaticLSPNextHopMulticast string
	DecapMPLSParams           DecapMPLSParams
}

// OcPolicyForwardingParams holds parameters for generating the OC Policy Forwarding config.
type OcPolicyForwardingParams struct {
	NetworkInstanceName string
	InterfaceID         string
	AppliedPolicyName   string
	RemovePolicyName    bool

	// Policy Rule specific params
	InnerDstIPv6       string
	InnerDstIPv4       string
	CloudV4NHG         string
	CloudV6NHG         string
	DecapPolicy        DecapPolicyParams
	GUEPort            uint32
	IPType             string
	DecapProtocol      string
	Dynamic            bool
	TunnelIP           string
	InterfaceName      string              // InterfaceName specifies the DUT interface where the policy will be applied.
	PolicyName         string              // PolicyName refers to the traffic policy that is bound to the given interface in CLI-based configuration.
	NetworkInstanceObj *oc.NetworkInstance // NetworkInstanceObj represents the OpenConfig network instance (default/non-default VRF).
	HasMPLS            bool                // HasMPLS indicates whether the policy forwarding configuration involves an MPLS overlay.
	MatchTTL           int
	ActionSetTTL       int
	ActionNHGName      string
	RemovePolicy       bool
	AggID              string
	Attributes         []*attrs.Attributes
}

// PolicyForwardingRule holds parameters for generating Policy Forwarding config for GRE.
type PolicyForwardingRule struct {
	Id                 uint32
	Name               string
	IpType             string
	SourceAddress      string
	DestinationAddress string
	TTL                []uint8
	Dscp               uint8
	Action             *oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action
}

// GueEncapPolicyParams defines parameters required to configure a GUE (Generic UDP Encapsulation) policy-based forwarding rule on the DUT.
type GueEncapPolicyParams struct {
	IPFamily         string // IPFamily specifies the IP address family for encapsulation. For example, "V4Udp" for IPv4-over-UDP or "V6Udp" for IPv6-over-UDP.
	PolicyName       string
	NexthopGroupName string
	SrcIntfName      string
	DstAddr          []string
	SrcAddr          []string
	Ttl              uint8
	Rule             uint8
}

// ACLTrafficPolicyParams holds parameters for configuring ACL forwarding configs.
type ACLTrafficPolicyParams struct {
	PolicyName   string
	ProtocolType string
	SrcPrefix    []string
	DstPrefix    []string
	SrcPort      string
	DstPort      string
	IntfName     string
	Direction    string
	Action       string
}

var (

	// PolicyForwardingConfigv4Arista configuration for policy-forwarding for ipv4.
	PolicyForwardingConfigv4Arista = `
Traffic-policies
   traffic-policy tp_cloud_id_3_20
      match bgpsetttlv4 ipv4
         ttl 1
         actions
            redirect next-hop group 1V4_vlan_3_20 ttl 1
            set traffic class 3
      match icmpechov4 ipv4
         destination prefix 169.254.0.11/32
         protocol icmp type echo-reply code all
      match ipv4-all-default ipv4
         actions
            redirect next-hop group 1V4_vlan_3_20
            set traffic class 3
      match ipv6-all-default ipv6
   !
     `
	// PolicyForwardingConfigv6Arista configuration for policy-forwarding for ipv6.
	PolicyForwardingConfigv6Arista = `
Traffic-policies
    traffic-policy tp_cloud_id_3_21
    match bgpsetttlv6 ipv6
       ttl 1
       !
       actions
          count
          redirect next-hop group 1V6_vlan_3_21 ttl 1
          set traffic class 3
    !
    match icmpv6 ipv6
       destination prefix 2600:2d00:0:1:8000:10:0:ca33/128
       protocol icmpv6 type echo-reply neighbor-advertisement code all
       !
       
    !
    match ipv4-all-default ipv4
    !
    match ipv6-all-default ipv6
       actions
          count
          redirect next-hop group 1V6_vlan_3_21
          set traffic class 3
 !
    `
	// PolicyForwardingConfigDualStackArista configuration for policy-forwarding for the dualstack.
	PolicyForwardingConfigDualStackArista = `
   Traffic-policies
    traffic-policy tp_cloud_id_3_22
    match bgpsetttlv6 ipv6
       ttl 1
       !
       actions
          count
          redirect next-hop group 1V6_vlan_3_22 ttl 1
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
          redirect next-hop group 1V4_vlan_3_22 ttl 1
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
          redirect next-hop group 1V4_vlan_3_22
          set traffic class 3
    !
    match ipv6-all-default ipv6
       actions
          count
          redirect next-hop group 1V6_vlan_3_22
          set traffic class 3
 !`
	// PolicyForwardingConfigMulticloudAristav4 configuration for policy-forwarding for multicloud ipv4.
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
					redirect next-hop group 1V4_vlan_3_23 ttl 1
					set traffic class 3
		!
		match ipv4-all-default ipv4
			 actions
					count
					redirect next-hop group 1V4_vlan_3_23
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
	decapGroupGREArista = `
ip decap-group gre-decap
  tunnel type gre
  tunnel decap-ip 11.0.0.0/8
  tunnel overlay mpls qos map mpls-traffic-class to traffic-class
!`

	decapGroupGUEArista = `
!
ip decap-group type udp destination port 6635 payload mpls
!
ip decap-group gre-decap
  tunnel type udp
  tunnel decap-ip 11.0.0.0/8
  tunnel overlay mpls qos map mpls-traffic-class to traffic-class
!`

	staticLSPArista = `
mpls static top-label 99991 169.254.0.12 pop payload-type ipv4 access-list bypass
mpls static top-label 99992 2600:2d00:0:1:8000:10:0:ca32 pop payload-type ipv6 access-list bypass
mpls static top-label 99993 169.254.0.26 pop payload-type ipv4 access-list bypass
mpls static top-label 99994 2600:2d00:0:1:7000:10:0:ca32 pop payload-type ipv6 access-list bypass
`

	decapGroupGREAristaMPLSTemplate = `
ip decap-group %s
  tunnel type gre
  tunnel decap-ip %s
  tunnel decap-interface %s
  tunnel overlay mpls qos map mpls-traffic-class to traffic-class
!`

	interfaceTrafficPolicyAristaTemplate = `
interface %s
traffic-policy input %s
!`

	interfaceTrafficPolicyAristaCloud = `
interface %s.%d
traffic-policy input tp_cloud_id_3_%d
!`
)

// InterfacelocalProxyConfig configures the interface local-proxy-arp.
func InterfacelocalProxyConfig(t *testing.T, dut *ondatra.DUTDevice, a *attrs.Attributes, aggID string) {
	if deviations.LocalProxyOCUnsupported(dut) {
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

// InterfaceLocalProxyConfigScale configures local-proxy-arp on multiple subinterfaces.
// When the device does not support the OpenConfig path, vendor-specific CLI commands
// are applied.
func InterfaceLocalProxyConfigScale(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, params OcPolicyForwardingParams) *gnmi.SetBatch {
	if deviations.LocalProxyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			interfacelocalproxy := new(strings.Builder)
			for _, a := range params.Attributes {
				if a.IPv4 != "" {
					fmt.Fprintf(interfacelocalproxy, `
					interface %s.%d
					 ip local-proxy-arp 
					!`, params.AggID, a.Subinterface)
				}
			}
			helpers.GnmiCLIConfig(t, dut, interfacelocalproxy.String())
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'local-proxy-arp'", dut.Vendor())
		}
		return nil
	} else {
		ocInterface := &oc.Interface{}
		for i, a := range params.Attributes {
			s := ocInterface.GetOrCreateSubinterface(a.Subinterface)
			b4 := s.GetOrCreateIpv4()
			parp := b4.GetOrCreateProxyArp()
			parp.SetMode(oc.E_ProxyArp_Mode(3))
			gnmi.BatchUpdate(sb, gnmi.OC().Interface(a.Name).Subinterface(uint32(i)).Config(), s)
		}
		return sb
	}
}

// InterfaceQosClassificationConfigScale configures qos-classification on multiple subinterfaces.
// When the device does not support the OpenConfig path, vendor-specific CLI commands
// are applied.
func InterfaceQosClassificationConfigScale(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, params OcPolicyForwardingParams) *gnmi.SetBatch {
	if deviations.QosClassificationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			interfaceqosconfig := new(strings.Builder)
			for _, a := range params.Attributes {
				fmt.Fprintf(interfaceqosconfig, `
				interface %s.%d
				 service-policy type qos input af3 
				!`, params.AggID, a.Subinterface)
			}
			helpers.GnmiCLIConfig(t, dut, interfaceqosconfig.String())
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'qos classification'", dut.Vendor())
		}
		return nil
	} else {
		d := &oc.Root{}
		ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		ni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
		ni.GetOrCreatePolicyForwarding()
		gnmi.BatchUpdate(sb, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni)
		return sb
	}
}

// InterfacePolicyForwardingConfigScale configures policy forwarding on multiple subinterfaces.
// When the device does not support the OpenConfig path, vendor-specific CLI commands
// are applied.
func InterfacePolicyForwardingConfigScale(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, pf *oc.NetworkInstance_PolicyForwarding, params OcPolicyForwardingParams) *gnmi.SetBatch {
	t.Helper()

	// Check if the DUT requires CLI-based configuration due to an OpenConfig deviation.
	if deviations.InterfacePolicyForwardingOCUnsupported(dut) {
		// If deviations exist, apply configuration using vendor-specific CLI commands.
		switch dut.Vendor() {
		case ondatra.ARISTA: // Currently supports Arista devices for CLI deviations.
			// Format and apply the CLI command for traffic policy input.
			trafficpolicyconfig := new(strings.Builder)
			for _, a := range params.Attributes {
				fmt.Fprintf(trafficpolicyconfig, `
				interface %[1]s.%[2]d  
				 traffic-policy input tp_cloud_id_%[2]d
				!`, params.AggID, a.Subinterface)
			}
			helpers.GnmiCLIConfig(t, dut, trafficpolicyconfig.String())
		default:
			// Log a message if the vendor is not supported for this specific CLI deviation.
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
		return nil
	} else {
		d := &oc.Root{}
		ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		ni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
		pf := ni.GetOrCreatePolicyForwarding()
		iface := pf.GetOrCreateInterface(params.InterfaceID)
		iface.ApplyForwardingPolicy = ygot.String(params.AppliedPolicyName)
		gnmi.BatchUpdate(sb, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
		return sb
	}
}

// InterfaceQosClassificationConfig configures the interface qos classification.
func InterfaceQosClassificationConfig(t *testing.T, dut *ondatra.DUTDevice, a *attrs.Attributes, aggID string) {
	if deviations.QosClassificationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d \n service-policy type qos input af3 \n", aggID, a.Subinterface))
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'qos classification'", dut.Vendor())
		}
	}
}

// InterfacePolicyForwardingConfig configures the interface policy-forwarding config.
func InterfacePolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, a *attrs.Attributes, aggID string, pf *oc.NetworkInstance_PolicyForwarding, params OcPolicyForwardingParams) {
	t.Helper()

	// Check if the DUT requires CLI-based configuration due to an OpenConfig deviation.
	if deviations.InterfacePolicyForwardingOCUnsupported(dut) {
		// If deviations exist, apply configuration using vendor-specific CLI commands.
		switch dut.Vendor() {
		case ondatra.ARISTA: // Currently supports Arista devices for CLI deviations.
			// Format and apply the CLI command for traffic policy input.
			var cliConfig string
			if params.Dynamic && a == nil && aggID == "" && params.AppliedPolicyName != "" && params.InterfaceID != "" {
				cliConfig = fmt.Sprintf(interfaceTrafficPolicyAristaTemplate, params.InterfaceID, params.AppliedPolicyName)
			} else {
				cliConfig = fmt.Sprintf(interfaceTrafficPolicyAristaCloud, aggID, a.Subinterface, a.Subinterface)
			}
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			// Log a message if the vendor is not supported for this specific CLI deviation.
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
	} else {
		ApplyPolicyToInterfaceOC(t, pf, params.InterfaceID, params.AppliedPolicyName)

	}
}

// MplsConfig configures the interface mpls.
func MplsConfig(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.MplsOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, "mpls ip")
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'mpls ip'", dut.Vendor())
		}
	} else {
		t.Log("Currently do not have support to enable Mpls through OC, need to uncomment once implemented")
		// TODO: Currently do not have support to enable Mpls through OC, need to uncomment once implemented.
		// d := &oc.Root{}
		// ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		// mpls := ni.GetOrCreateMpls()
		// mpls.Enabled = ygot.Bool(true)
	}
}

// QosClassificationConfig configures the interface qos classification.
func QosClassificationConfig(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.QosClassificationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, qosconfigArista)
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'qos classification'", dut.Vendor())
		}
	} else {
		QosClassificationOCConfig(t)
	}
}

// LabelRangeConfig configures the interface label range.
func LabelRangeConfig(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.MplsLabelClassificationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, mplsLabelRangeArista)
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'mpls label range'", dut.Vendor())
		}
	} else {
		LabelRangeOCConfig(t, dut)
	}
}

// PolicyForwardingConfig configures the interface policy-forwarding config.
func PolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, traffictype string, pf *oc.NetworkInstance_PolicyForwarding, params OcPolicyForwardingParams) {
	t.Helper()

	// Check if the DUT requires CLI-based configuration due to an OpenConfig deviation.
	if deviations.PolicyForwardingOCUnsupported(dut) {
		// If deviations exist, apply configuration using vendor-specific CLI commands.
		switch dut.Vendor() {
		case ondatra.ARISTA: // Currently supports Arista devices for CLI deviations.
			// Select and apply the appropriate CLI snippet based on 'traffictype'.
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
			// Log a message if the vendor is not supported for this specific CLI deviation.
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
	} else {

		RulesAndActions(params, pf)

	}
}

// NewPolicyForwardingMatchAndSetTTL configures a policy-forwarding rule that matches packets based on IP TTL and rewrites the TTL before redirecting traffic to a specified next-hop group.
func NewPolicyForwardingMatchAndSetTTL(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, params OcPolicyForwardingParams) {
	t.Helper()
	// Check if the DUT requires CLI-based configuration due to an OpenConfig deviation.
	if deviations.PolicyForwardingOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if params.RemovePolicy {
				removeCmd := fmt.Sprintf(`
				traffic-policies
				  no traffic-policy %s
				`, params.PolicyName)
				helpers.GnmiCLIConfig(t, dut, removeCmd)
				return
			} else {
				switch params.IPType {
				case "ipv4":
					policyForwardingConfigv4Vrf := fmt.Sprintf(`
						traffic-policies
						traffic-policy %[1]s
							match rewritettlv4 ipv4
							ttl %[2]d
							!
							actions
								count
								redirect next-hop group %[3]s ttl %[4]d
							!
							interface %[5]s
							traffic-policy input %[1]s
						!`,
						params.PolicyName,
						params.MatchTTL,
						params.ActionNHGName,
						params.ActionSetTTL,
						params.InterfaceName,
					)
					helpers.GnmiCLIConfig(t, dut, policyForwardingConfigv4Vrf)

				case "ipv6":
					policyForwardingConfigv6Vrf := fmt.Sprintf(`
						traffic-policies
						no traffic-policy %[1]s
						traffic-policy %[1]s
							match rewritettlv6 ipv6
							ttl %[2]d
							!
							actions
								count
								redirect next-hop group %[3]s ttl %[4]d
							!
							interface %[5]s
							traffic-policy input %[1]s
						!`,
						params.PolicyName,
						params.MatchTTL,
						params.ActionNHGName,
						params.ActionSetTTL,
						params.InterfaceName,
					)
					helpers.GnmiCLIConfig(t, dut, policyForwardingConfigv6Vrf)

				default:
					t.Logf("Unsupported traffictype %s for TTL policy", params.IPType)
				}
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
	} else {
		RulesAndActions(params, pf)
	}
}

// PolicyForwardingConfigScale configures policy forwarding using multiple traffic-policies.
// When the device does not support the OpenConfig path, vendor-specific CLI commands
// are applied.
func PolicyForwardingConfigScale(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, encapparams OCEncapsulationParams, pf *oc.NetworkInstance_PolicyForwarding, params OcPolicyForwardingParams) *gnmi.SetBatch {
	t.Helper()

	// Check if the DUT requires CLI-based configuration due to an OpenConfig deviation.
	if deviations.PolicyForwardingOCUnsupported(dut) {
		// If deviations exist, apply configuration using vendor-specific CLI commands.
		switch dut.Vendor() {
		case ondatra.ARISTA: // Currently supports Arista devices for CLI deviations.
			PolicyForwardingConfig := new(strings.Builder)

			PolicyForwardingConfig.WriteString("traffic-policies\n")

			for i := 1; i <= encapparams.NextHopGroupCount; i++ {

				fmt.Fprintf(PolicyForwardingConfig, `
    traffic-policy tp_cloud_id_%[1]d
    match setttlv6 ipv6
       ttl 1
       !
       actions
          count
          redirect next-hop group nh_vlan_%[1]d
          set traffic class 3
    !
    match setttlv4 ipv4
       ttl 1
       !
       actions
          count
          redirect next-hop group nh_vlan_%[1]d
          set traffic class 3
    !
    match ipv4-all-default ipv4
       actions
          count
          redirect next-hop group nh_vlan_%[1]d
          set traffic class 3
    !
    match ipv6-all-default ipv6
       actions
          count
          redirect next-hop group nh_vlan_%[1]d
          set traffic class 3
    !`, i)
			}

			helpers.GnmiCLIConfig(t, dut, PolicyForwardingConfig.String())

			for i := encapparams.Count - encapparams.NextHopGroupCount + 1; i <= encapparams.Count; i++ {

				fmt.Fprintf(PolicyForwardingConfig, `
    traffic-policy tp_cloud_id_%[1]d
    match setttlv6 ipv6
       ttl 1
       !
       actions
          count
          redirect next-hop group nh_vlan_%[2]d
          set traffic class 3
    !
    match setttlv4 ipv4
       ttl 1
       !
       actions
          count
          redirect next-hop group nh_vlan_%[2]d
          set traffic class 3
    !
    match ipv4-all-default ipv4
       actions
          count
          redirect next-hop group nh_vlan_%[2]d
          set traffic class 3
    !
    match ipv6-all-default ipv6
       actions
          count
          redirect next-hop group nh_vlan_%[2]d
          set traffic class 3
    !`, i, i-encapparams.NextHopGroupCount)
			}

			helpers.GnmiCLIConfig(t, dut, PolicyForwardingConfig.String())

		default:
			// Log a message if the vendor is not supported for this specific CLI deviation.
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
		return nil
	} else {
		ruleSeq := uint32(1)
		for i := 1; i <= encapparams.Count; i++ {
			// https://partnerissuetracker.corp.google.com/issues/417988636
			//  pols := pf.GetOrCreatePolicy(fmt.Sprintf("tp_cloud_id_3_%d", i))
			// rule := pols.GetOrCreateRule(ruleSeq)
			// rule.GetOrCreateAction().Count = ygot.Bool(true)
			// groupName := fmt.Sprintf("V4_vlan_3_%d", i)
			// rule.GetOrCreateAction().SetNextHopGroup(groupName)
			// rule.GetOrCreateAction().SetTtl(1)
		}
		ruleSeq++
		gnmi.BatchReplace(sb, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
		return sb
	}
}

// SetupPolicyForwardingInfraOC creates a new OpenConfig root object, the specified network instance,
// and the policy-forwarding container within it.
// It returns the root, the network instance, and the policy-forwarding container.
func SetupPolicyForwardingInfraOC(networkInstanceName string) (*oc.Root, *oc.NetworkInstance, *oc.NetworkInstance_PolicyForwarding) {
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(networkInstanceName)
	pf := ni.GetOrCreatePolicyForwarding()
	return root, ni, pf
}

// RulesAndActions defines forwarding policies, their rules, and associated next-hop groups
// within the provided policy-forwarding container, based on canonical OpenConfig modeling.
func RulesAndActions(params OcPolicyForwardingParams, pf *oc.NetworkInstance_PolicyForwarding) {
	// --- Define the Main Policy (e.g., "customer1") and its Rules ---
	pols := pf.GetOrCreatePolicy("customer1")
	var ruleSeq uint32 = 1

	// Rule 1: (derived from JSON "customer1_prefixv6_and_icmpv6_ns")
	rule1 := pols.GetOrCreateRule(ruleSeq)
	rule1.GetOrCreateIpv4().DestinationAddress = ygot.String(params.InnerDstIPv6)
	rule1.GetOrCreateIpv6().GetOrCreateIcmpv6().Type = oc.Icmpv6Types_TYPE_NEIGHBOR_SOLICITATION

	// TODO: b/417988636 - Set the action to count
	// rule1.GetOrCreateAction().Count = ygot.Bool(true)
	ruleSeq++

	// Rule 2: (derived from JSON "customer1_prefixv6_and_icmpv6_na")
	rule2 := pols.GetOrCreateRule(ruleSeq)
	rule2.GetOrCreateIpv4().DestinationAddress = ygot.String(params.InnerDstIPv6)
	rule2.GetOrCreateIpv6().GetOrCreateIcmpv6().Type = oc.Icmpv6Types_TYPE_NEIGHBOR_ADVERTISEMENT

	// TODO: b/417988636 - Set the action to count
	// rule2.GetOrCreateAction().Count = ygot.Bool(true)
	ruleSeq++

	// Rule 3: (derivGetOrCreateRules().ed from JSON "customer1_prefixv4_and_icmp")
	rule3 := pols.GetOrCreateRule(ruleSeq)
	rule3.GetOrCreateIpv4().DestinationAddress = ygot.String(params.InnerDstIPv4)
	rule3.GetOrCreateIpv4().GetOrCreateIcmpv4().Type = oc.Icmpv4Types_TYPE_EXT_ECHO_REPLY

	// TODO: b/417988636 - Set the action to count
	// rule3.GetOrCreateAction().Count = ygot.Bool(true)
	ruleSeq++

	// Rule 4: (derived from JSON "customer1_prefixv6_and_icmp")
	rule4 := pols.GetOrCreateRule(ruleSeq)
	rule4.GetOrCreateIpv6().DestinationAddress = ygot.String(params.InnerDstIPv6)
	rule4.GetOrCreateIpv6().GetOrCreateIcmpv6().Type = oc.Icmpv6Types_TYPE_EXT_ECHO_REPLY

	// TODO: b/417988636 - Set the action to count
	// rule4.GetOrCreateAction().Count = ygot.Bool(true)
	ruleSeq++

	// Rule 5: (derived from JSON "customer1_ttl_v4")
	rule5 := pols.GetOrCreateRule(ruleSeq)
	rule5.GetOrCreateIpv4().HopLimit = ygot.Uint8(1)

	// TODO: b/417988636 - Set the action to count
	// rule5.GetOrCreateAction().Count = ygot.Bool(true)
	// rule5.GetOrCreateAction().NextHopGroup = ygot.String(params.CloudV4NHG)
	// rule5.GetOrCreateAction().SetTtl = ygot.Uint8(1)
	ruleSeq++

	// Rule 6: (derived from JSON "customer1_ttl_v6")
	rule6 := pols.GetOrCreateRule(ruleSeq)
	rule6.GetOrCreateIpv6().HopLimit = ygot.Uint8(1)

	// TODO: b/417988636 - Set the action to count
	// rule6.GetOrCreateAction().Count = ygot.Bool(true)
	// rule6.GetOrCreateAction().NextHopGroup = ygot.String(params.CloudV6NHG)
	// rule6.GetOrCreateAction().SetHopLimit = ygot.Uint8(1)
	ruleSeq++

	// Rule 7: (derived from JSON "customer1_default_v4")
	rule7 := pols.GetOrCreateRule(ruleSeq)
	rule7.GetOrCreateIpv4().DestinationAddress = ygot.String(params.InnerDstIPv4)
	// TODO: b/417988636 - Set the action to count
	// rule7.GetOrCreateAction().Count = ygot.Bool(true)
	// rule7.GetOrCreateAction().NextHopGroup = ygot.String(params.CloudV4NHG)
	ruleSeq++

	// Rule 8: (derived from JSON "customer1_default_v6")
	rule8 := pols.GetOrCreateRule(ruleSeq)
	rule8.GetOrCreateIpv6().DestinationAddress = ygot.String(params.InnerDstIPv6)
	// TODO: sancheetaroy - Set the action to count
	// rule8.GetOrCreateAction().Count = ygot.Bool(true)
	// rule8.GetOrCreateAction().NextHopGroup = ygot.String(params.CloudV6NHG)
	ruleSeq++
}

// DecapPolicyRulesandActionsGre configures the "decap MPLS in GRE" policy and related MPLS global and static LSP settings.
func DecapPolicyRulesandActionsGre(t *testing.T, pf *oc.NetworkInstance_PolicyForwarding, params OcPolicyForwardingParams) {
	t.Helper()

	pols := pf.GetOrCreatePolicy("customer10")
	var ruleSeq uint32 = 10
	var protocol uint8 = 4

	rule10 := pols.GetOrCreateRule(ruleSeq)
	rule10.GetOrCreateIpv4().DestinationAddress = ygot.String(params.InnerDstIPv4)
	rule10.GetOrCreateIpv4().Protocol = oc.UnionUint8(protocol)

	rule10.GetOrCreateAction().DecapsulateGre = ygot.Bool(true)
}

// DecapPolicyRulesandActionsGue configures the "decap MPLS in GUE" policy and related MPLS global and static LSP settings.
func DecapPolicyRulesandActionsGue(t *testing.T, pf *oc.NetworkInstance_PolicyForwarding, params OcPolicyForwardingParams) {
	t.Helper()

	pols := pf.GetOrCreatePolicy("customer10")
	var ruleSeq uint32 = 10
	var protocol uint8 = 4

	rule10 := pols.GetOrCreateRule(ruleSeq)
	rule10.GetOrCreateIpv4().DestinationAddress = ygot.String(params.InnerDstIPv4)
	rule10.GetOrCreateIpv4().Protocol = oc.UnionUint8(protocol)

	rule10.GetOrCreateAction().DecapsulateGue = ygot.Bool(true)
}

// ApplyPolicyToInterfaceOC configures the policy-forwarding interfaces section to apply the specified
// policy to the given interface ID.
func ApplyPolicyToInterfaceOC(t *testing.T, pf *oc.NetworkInstance_PolicyForwarding, interfaceID string, appliedPolicyName string) {
	t.Helper()
	iface := pf.GetOrCreateInterface(interfaceID)
	iface.ApplyForwardingPolicy = ygot.String(appliedPolicyName)
}

// PushPolicyForwardingConfig pushes the complete Policy Forwarding config.
func PushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}

// DecapGroupConfigGre configures the interface decap-group.
func DecapGroupConfigGre(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ocPFParams OcPolicyForwardingParams) {
	if deviations.GueGreDecapUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if ocPFParams.Dynamic {
				t.Logf("Going into decap")
				aristaGreDecapCLIConfig(t, dut, ocPFParams)
			} else {
				helpers.GnmiCLIConfig(t, dut, decapGroupGREArista)
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'decap-group config'", dut.Vendor())
		}
	} else {
		DecapPolicyRulesandActionsGre(t, pf, ocPFParams)
	}
}

// DecapGroupConfigGue configures the interface decap-group for GUE.
func DecapGroupConfigGue(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ocPFParams OcPolicyForwardingParams) {
	if deviations.GueGreDecapUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if ocPFParams.Dynamic {
				aristaGueDecapCLIConfig(t, dut, ocPFParams)
			} else {
				helpers.GnmiCLIConfig(t, dut, decapGroupGUEArista)
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'decap-group config'", dut.Vendor())
		}
	} else {
		DecapPolicyRulesandActionsGue(t, pf, ocPFParams)
	}
}

// RemoveDecapGroupGue deletes a GUE decap-group previously created by
// DecapGroupConfigGue, allowing tests to revert the DUT to its original state.
func RemoveDecapGroupGue(t *testing.T, dut *ondatra.DUTDevice, ocPFParams OcPolicyForwardingParams) {
	t.Helper()
	if deviations.GueGreDecapUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("no ip decap-group %s\n", ocPFParams.AppliedPolicyName))
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'decap-group removal'", dut.Vendor())
		}
	} else {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(ocPFParams.AppliedPolicyName).Config())
	}
}

// aristaGueDecapCLIConfig configures GUEDEcapConfig for Arista
func aristaGueDecapCLIConfig(t *testing.T, dut *ondatra.DUTDevice, params OcPolicyForwardingParams) {

	decapProto := params.DecapProtocol
	if decapProto == "" {
		decapProto = params.IPType
	}

	cliConfig := fmt.Sprintf(`
		                    ip decap-group type udp destination port %v payload %s
							ip decap-group %s
							tunnel type UDP
							tunnel decap-ip %s
							`, params.GUEPort, decapProto, params.AppliedPolicyName, params.TunnelIP)
	if params.InterfaceID != "" {
		cliConfig += fmt.Sprintf("tunnel decap-interface %s", params.InterfaceID)
	}
	helpers.GnmiCLIConfig(t, dut, cliConfig)
}

// aristaGreDecapCLIConfig configures GREDEcapConfig for Arista
func aristaGreDecapCLIConfig(t *testing.T, dut *ondatra.DUTDevice, params OcPolicyForwardingParams) {
	var cliConfig string
	if params.HasMPLS {
		cliConfig = fmt.Sprintf(decapGroupGREAristaMPLSTemplate, params.AppliedPolicyName, params.TunnelIP, params.InterfaceID)
	} else {
		cliConfig = fmt.Sprintf(`
			ip decap-group %s
			 tunnel type gre
			 tunnel decap-ip %s
			`, params.AppliedPolicyName, params.TunnelIP)
	}
	helpers.GnmiCLIConfig(t, dut, cliConfig)

}

// QosClassificationConfig configures the interface qos classification.
func ConfigureTOSGUE(t *testing.T, dut *ondatra.DUTDevice, policyName string, dscpValue uint32, port string, deleteTOS bool) {
	if deviations.QosClassificationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if deleteTOS {
				cli := fmt.Sprintf(`
				policy-map type quality-of-service %s
   					class class-default
      				no set dscp %d`, policyName, dscpValue)
				helpers.GnmiCLIConfig(t, dut, cli)
			} else {
				cli := fmt.Sprintf(`
				policy-map type quality-of-service %s
   					class class-default
      				set dscp %d
				
				qos rewrite dscp

				interface %s
					service-policy type qos input %s
					`, policyName, dscpValue, port, policyName)
				helpers.GnmiCLIConfig(t, dut, cli)
			}

		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'qos classification'", dut.Vendor())
		}
	}

}

// Configure GRE decapsulated. Adding deviation when device doesn't support OC
func PolicyForwardingGreDecapsulation(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, decapIP string, policyName string, portName string, decapGrpName string) {
	if deviations.GreDecapsulationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig := fmt.Sprintf(`
			ip decap-group %s
			 tunnel type gre
			 tunnel decap-ip %s
			`, decapGrpName, strings.Split(decapIP, "/")[0])
			helpers.GnmiCLIConfig(t, dut, cliConfig)

		default:
			t.Errorf("deviation GreDecapsulationUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	} else {
		d := &oc.Root{}
		ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		ni1.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
		npf := ni1.GetOrCreatePolicyForwarding()
		np := npf.GetOrCreatePolicy(policyName)
		np.PolicyId = ygot.String(policyName)
		np.Type = oc.Policy_Type_PBR_POLICY

		npRule := np.GetOrCreateRule(10)
		ip := npRule.GetOrCreateIpv4()
		ip.DestinationAddressPrefixSet = ygot.String(decapIP)
		npAction := npRule.GetOrCreateAction()
		npAction.DecapsulateGre = ygot.Bool(true)

		port := dut.Port(t, portName)
		ingressPort := port.Name()
		t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)

		intf := npf.GetOrCreateInterface(ingressPort)
		intf.ApplyForwardingPolicy = ygot.String(policyName)
		intf.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)

		gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni1)
	}
}

func ConfigureVrfSelectionPolicy(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, policyName string, vrfRules []VrfRule) {
	t.Logf("Configuring VRF Selection Policy")
	policy := pf.GetOrCreatePolicy(policyName)
	policy.Type = oc.Policy_Type_VRF_SELECTION_POLICY

	for _, vrfRule := range vrfRules {
		rule := policy.GetOrCreateRule(vrfRule.Index)
		switch vrfRule.IpType {
		case IPv4:
			rule.GetOrCreateIpv4().SourceAddress = ygot.String(fmt.Sprintf("%s/%d", vrfRule.SourcePrefix, vrfRule.PrefixLength))
		case IPv6:
			rule.GetOrCreateIpv6().SourceAddress = ygot.String(fmt.Sprintf("%s/%d", vrfRule.SourcePrefix, vrfRule.PrefixLength))
		default:
			t.Fatalf("Unsupported IP type %s in vrf rule", vrfRule.IpType)
		}
		rule.GetOrCreateTransport()
		ruleAction := rule.GetOrCreateAction()
		ruleAction.SetNetworkInstance(vrfRule.NetInstName)
	}
}

func ApplyVrfSelectionPolicyToInterfaceOC(t *testing.T, pf *oc.NetworkInstance_PolicyForwarding, interfaceID string, appliedPolicyName string) {
	t.Helper()
	iface := pf.GetOrCreateInterface(interfaceID)
	iface.ApplyVrfSelectionPolicy = ygot.String(appliedPolicyName)
	iface.GetOrCreateInterfaceRef().Interface = ygot.String(interfaceID)
	iface.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
}

func NewPolicyForwardingEncapGre(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, policyName string, interfaceName string, targetName string, rules []PolicyForwardingRule) {
	if deviations.PolicyForwardingGreEncapsulationOcUnsupported(dut) || deviations.PolicyForwardingToNextHopOcUnsupported(dut) {
		t.Logf("Configuring pf through CLI")
		newPolicyForwardingEncapGreFromCli(t, dut, policyName, interfaceName, targetName, rules)
	} else {
		t.Logf("Configuring pf through OC")
		newPolicyForwardingEncapGreFromOC(t, pf, policyName, interfaceName, rules)
	}
}

func newPolicyForwardingEncapGreFromCli(t *testing.T, dut *ondatra.DUTDevice, policyName string, interfaceName string, targetName string, rules []PolicyForwardingRule) {
	gnmiClient := dut.RawAPIs().GNMI(t)
	tpConfig := trafficPolicyCliConfig(t, dut, policyName, interfaceName, targetName, rules)
	t.Logf("Push the CLI Policy config:%s", dut.Vendor())
	gpbSetRequest := buildCliSetRequest(tpConfig)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Errorf("failed to set policy forwarding from cli: %v", err)
	}
}

func newPolicyForwardingEncapGreFromOC(t *testing.T, pf *oc.NetworkInstance_PolicyForwarding, policyName string, interfaceName string, rules []PolicyForwardingRule) {
	t.Helper()
	policy := pf.GetOrCreatePolicy(policyName)
	policy.Type = oc.Policy_Type_PBR_POLICY
	for _, ruleConfig := range rules {
		t.Logf("Processing rule %s", ruleConfig.Name)
		rule := policy.GetOrCreateRule(ruleConfig.Id)
		switch ruleConfig.IpType {
		case IPv4:
			ruleIPv4 := rule.GetOrCreateIpv4()
			if ruleConfig.SourceAddress != "" {
				ruleIPv4.SourceAddress = ygot.String(ruleConfig.SourceAddress)
			}
			if ruleConfig.DestinationAddress != "" {
				ruleIPv4.DestinationAddress = ygot.String(ruleConfig.DestinationAddress)
			}
			if ruleConfig.Dscp != 0 {
				ruleIPv4.Dscp = ygot.Uint8(ruleConfig.Dscp)
			}
		case IPv6:
			ruleIPv6 := rule.GetOrCreateIpv6()
			if ruleConfig.SourceAddress != "" {
				ruleIPv6.SourceAddress = ygot.String(ruleConfig.SourceAddress)
			}
			if ruleConfig.DestinationAddress != "" {
				ruleIPv6.DestinationAddress = ygot.String(ruleConfig.DestinationAddress)
			}
			if ruleConfig.Dscp != 0 {
				ruleIPv6.Dscp = ygot.Uint8(ruleConfig.Dscp)
			}
		default:
			t.Errorf("unknown IP type %s in PolicyForwardingRule", ruleConfig.IpType)
			return
		}
		if ruleConfig.Action != nil {
			rule.Action = ruleConfig.Action
		}
	}
}

func trafficPolicyCliConfig(t *testing.T, dut *ondatra.DUTDevice, policyName string, interfaceName string, targetName string, rules []PolicyForwardingRule) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var nhGroups, matchRules string
		var nhGroupTargets = make(map[string][]string)
		var nhGroupsBySource = make(map[string]string)
		var nhTTlBySource = make(map[string]uint8)
		for _, ruleConfig := range rules {
			var matchTarget string
			t.Logf("Processing rule %s", ruleConfig.Name)
			if ruleConfig.Action == nil ||
				ruleConfig.Name == "" {
				t.Errorf("invalid rule configuration: %v", ruleConfig)
				return ""
			}
			if ruleConfig.DestinationAddress != "" {
				matchTarget += fmt.Sprintf("destination prefix %s\n", ruleConfig.DestinationAddress)
			}
			if ruleConfig.SourceAddress != "" {
				matchTarget += fmt.Sprintf("source prefix %s\n", ruleConfig.SourceAddress)
			}
			if len(ruleConfig.TTL) > 0 {
				ttlStrs := make([]string, len(ruleConfig.TTL))
				for i, v := range ruleConfig.TTL {
					ttlStrs[i] = fmt.Sprintf("%d", v)
				}
				ttlValues := strings.Join(ttlStrs, ", ")
				matchTarget += fmt.Sprintf("ttl %s\n", ttlValues)
			}
			if matchTarget == "" {
				t.Errorf("rule %s must have either SourceAddress, DestinationAddress or TTL defined", ruleConfig.Name)
				return ""
			}
			switch ruleConfig.IpType {
			case IPv4, IPv6:
				matchRules += fmt.Sprintf(`
                match %s %s
                %s
                actions
                count`, ruleConfig.Name, strings.ToLower(ruleConfig.IpType), matchTarget)
				if (*ruleConfig.Action).NextHop != nil {
					matchRules += fmt.Sprintf(`
                redirect next-hop %s
                !`, *(*ruleConfig.Action).NextHop)
				} else if (*ruleConfig.Action).EncapsulateGre != nil {
					for _, targetKey := range slices.Sorted(maps.Keys((*ruleConfig.Action).EncapsulateGre.Target)) {
						target := (*ruleConfig.Action).EncapsulateGre.Target[targetKey]
						if target != nil {
							if target.Source == nil || target.Destination == nil {
								t.Errorf("target in EncapsulateGre action must have Source and Destination defined")
								return ""
							}
							if !slices.Contains(nhGroupTargets[*(target.Source)], *target.Destination) {
								nhGroupTargets[*(target.Source)] = append(nhGroupTargets[*(target.Source)], *target.Destination)
							}
							if target.IpTtl != nil {
								nhTTlBySource[*(target.Source)] = *target.IpTtl
							}
						}
					}
					index := 1
					for source := range nhGroupTargets {
						nhGroupName := fmt.Sprintf("%s_%d", targetName, index)
						nhGroupsBySource[source] = nhGroupName
						nhGroups += fmt.Sprintf("%s ", nhGroupName)
					}
					matchRules += fmt.Sprintf(`
                    redirect next-hop group %s
                    !`, nhGroups)
				}
			default:
				t.Errorf("unknown IP type %s in PolicyForwardingRule %s", ruleConfig.IpType, ruleConfig.Name)
				return ""
			}
		}

		var ipv4GreNHs string
		for src, destinations := range nhGroupTargets {
			ipv4GreNHs += fmt.Sprintf(`
            nexthop-group %s type gre`, nhGroupsBySource[src])
			if len(nhTTlBySource) > 0 && nhTTlBySource[src] > 0 {
				ipv4GreNHs += fmt.Sprintf(`
                ttl %d`, nhTTlBySource[src])
			}
			ipv4GreNHs += fmt.Sprintf(`
            tunnel-source %s`, src)
			for index, dest := range destinations {
				ipv4GreNHs += fmt.Sprintf(`
                entry %d tunnel-destination %s`, index, dest)
			}
		}

		// Apply Policy on the interface
		trafficPolicyConfig := fmt.Sprintf(`
            traffic-policies
            traffic-policy %s
            %s
            %s
            !
            interface %s
            traffic-policy input %s
            `, policyName, matchRules, ipv4GreNHs, interfaceName, policyName)
		return trafficPolicyConfig
	default:
		return ""
	}
}

// Configure GRE decapsulated. Adding deviation when device doesn't support OC
func NewConfigureGRETunnel(t *testing.T, dut *ondatra.DUTDevice, decapIp string, decapGrpName string) {
	if deviations.GreDecapsulationOCUnsupported(dut) {
		var decapIPAddr string
		if strings.Contains(decapIp, "/") {
			decapIPAddr = strings.Split(decapIp, "/")[0]
		} else {
			decapIPAddr = decapIp
		}
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig := fmt.Sprintf(`
			ip decap-group %s
			 tunnel type gre
			 tunnel decap-ip %s
			`, decapGrpName, decapIPAddr)
			helpers.GnmiCLIConfig(t, dut, cliConfig)

		default:
			t.Errorf("deviation GreDecapsulationUnsupported is not handled for the dut: %v", dut.Vendor())
		}
	} else {
		d := &oc.Root{}
		ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		ni1.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
		npf := ni1.GetOrCreatePolicyForwarding()
		np := npf.GetOrCreatePolicy("PBR-MAP")
		np.PolicyId = ygot.String("PBR-MAP")
		np.Type = oc.Policy_Type_PBR_POLICY

		npRule := np.GetOrCreateRule(10)
		ip := npRule.GetOrCreateIpv4()
		ip.DestinationAddressPrefixSet = ygot.String(decapIp)
		npAction := npRule.GetOrCreateAction()
		npAction.DecapsulateGre = ygot.Bool(true)

		port := dut.Port(t, "port1")
		ingressPort := port.Name()
		t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)

		intf := npf.GetOrCreateInterface(ingressPort)
		intf.ApplyForwardingPolicy = ygot.String("PBR-MAP")
		intf.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)

		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni1)
	}
}

// ConfigureDutWithGueDecap configures the DUT to decapsulate GUE (Generic UDP Encapsulation) traffic. It supports both native CLI configuration (for vendors like Arista) and OpenConfig (GNMI) configuration.
func ConfigureDutWithGueDecap(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType, tunIP, decapInt, policyName string, policyId int) {
	t.Logf("Configure DUT with decapsulation UDP port %v", guePort)
	if deviations.DecapsulateGueOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig := fmt.Sprintf(`
                            ip decap-group type udp destination port %[1]d payload %[2]s 
                            tunnel type %[2]s-over-udp udp destination port %[1]d
                            ip decap-group test
                            tunnel type UDP
                            tunnel decap-ip %[3]s
                            tunnel decap-interface %[4]s
                            `, guePort, ipType, tunIP, decapInt)
			helpers.GnmiCLIConfig(t, dut, cliConfig)

		default:
			t.Errorf("deviation decapsulateGueOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
	} else {
		// TODO: As per the latest OpenConfig GNMI OC schema — the Encapsulation/Decapsulation sub-tree is not fully implemented, need to add OC commands once implemented.
		d := &oc.Root{}
		ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		npf := ni1.GetOrCreatePolicyForwarding()
		np := npf.GetOrCreatePolicy(policyName)
		np.PolicyId = ygot.String(policyName)
		npRule := np.GetOrCreateRule(uint32(policyId))
		ip := npRule.GetOrCreateIpv4()
		ip.DestinationAddressPrefixSet = ygot.String(tunIP)
		ip.Protocol = oc.PacketMatchTypes_IP_PROTOCOL_IP_UDP
		// transport := npRule.GetOrCreateTransport()
		// transport.SetDestinationPort()
	}
}

// PbrRule defines a policy-based routing rule configuration
type PbrRule struct {
	Sequence  uint32
	EtherType oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2_Ethertype_Union
	EncapVrf  string
}

// PolicyForwardingConfigName defines the configuration parameters for PBR VRF selection.
type PolicyForwardingConfigName struct {
	Name string // Policy name (e.g., "VRF-SELECT-POLICY")
}

// NewPolicyForwardingVRFSelection configures Policy-Based Routing for VRF selection.
func NewPolicyForwardingVRFSelection(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, cfg PolicyForwardingConfigName) *gnmi.SetBatch {
	t.Helper()

	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()
	p := pf.GetOrCreatePolicy(cfg.Name)
	p.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pRule := range getPbrRules(dut) {
		r := p.GetOrCreateRule(seqIDOffset(dut, pRule.Sequence))

		// Optional default rule match requirement.
		if deviations.PfRequireMatchDefaultRule(dut) && pRule.EtherType != nil {
			r.GetOrCreateL2().Ethertype = pRule.EtherType
		}

		// Set forwarding action (encap VRF)
		if pRule.EncapVrf != "" {
			r.GetOrCreateAction().SetNetworkInstance(pRule.EncapVrf)
		}
	}

	// Push policy forwarding configuration via GNMI batch.
	gnmi.BatchUpdate(sb,
		gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(),
		pf,
	)

	t.Logf("Configured policy forwarding VRF selection: policy=%s", cfg.Name)

	return sb
}

// getPbrRules returns policy-based routing rules for VRF selection
func getPbrRules(dut *ondatra.DUTDevice) []PbrRule {
	vrfDefault := deviations.DefaultNetworkInstance(dut)

	if deviations.PfRequireMatchDefaultRule(dut) {
		return []PbrRule{
			{
				Sequence:  17,
				EtherType: ethertypeIPv4,
				EncapVrf:  vrfDefault,
			},
			{
				Sequence:  18,
				EtherType: ethertypeIPv6,
				EncapVrf:  vrfDefault,
			},
		}
	}
	return []PbrRule{
		{
			Sequence: 17,
			EncapVrf: vrfDefault,
		},
	}
}

// seqIDOffset returns sequence ID with base offset to ensure proper ordering
func seqIDOffset(dut *ondatra.DUTDevice, i uint32) uint32 {
	if deviations.PfRequireSequentialOrderPbrRules(dut) {
		return i + seqIDBase
	}
	return i
}

// NewPolicyForwardingGueEncap configure policy forwarding for GUE encapsulation.
func NewPolicyForwardingGueEncap(t *testing.T, dut *ondatra.DUTDevice, params GueEncapPolicyParams) {
	t.Helper()
	_, _, pf := SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))

	// Configure traffic policy
	if deviations.PolicyForwardingOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			switch params.IPFamily {
			case "V4Udp":
				createPolicyForwardingNexthopCLIConfig(t, dut, params.PolicyName, "rule1", "ipv4", params.NexthopGroupName)
			case "V6Udp":
				createPolicyForwardingNexthopCLIConfig(t, dut, params.PolicyName, "rule2", "ipv6", params.NexthopGroupName)
			default:
				t.Logf("Unsupported address family type %s", params.IPFamily)
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
	} else {
		policy := pf.GetOrCreatePolicy(params.PolicyName)
		policy.Type = oc.Policy_Type_PBR_POLICY

		rule1 := policy.GetOrCreateRule(1)
		rule1.GetOrCreateTransport()
		if len(params.DstAddr) != 0 {
			for _, addr := range params.DstAddr {
				rule1.GetOrCreateIpv4().DestinationAddress = ygot.String(addr)
			}
		}
		if len(params.SrcAddr) != 0 {
			for _, addr := range params.SrcAddr {
				rule1.GetOrCreateIpv4().SourceAddress = ygot.String(addr)
			}
		}
		// Validate NexthopGroupName before applying it to the rule.
		if params.NexthopGroupName != "" {
			rule1.GetOrCreateAction().SetNextHop(params.NexthopGroupName)
		} else {
			t.Errorf("NexthopGroupName is required for OpenConfig policy-forwarding GUE encapsulation rules")
		}
	}
}

// createPolicyForwardingNexthopCLIConfig configure nexthop policy forwarding through CLI.
func createPolicyForwardingNexthopCLIConfig(t *testing.T, dut *ondatra.DUTDevice, policyName string, ruleName string, traffictype string, nhGrpName string) {
	t.Helper()
	// Check if the DUT requires CLI-based configuration due to an OpenConfig deviation.
	if deviations.PolicyForwardingOCUnsupported(dut) {
		// If deviations exist, apply configuration using vendor-specific CLI commands.
		cli := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			// Select and apply the appropriate CLI snippet based on 'traffictype'.
			cli = fmt.Sprintf(`
				traffic-policies
				traffic-policy %s
      			match %s %s
         		actions
            	redirect next-hop group %s`, policyName, ruleName, traffictype, nhGrpName)
			helpers.GnmiCLIConfig(t, dut, cli)
		default:
			// Log a message if the vendor is not supported for this specific CLI deviation.
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
	}
}

// InterfacePolicyForwardingApply configure to apply policy forwarding.
func InterfacePolicyForwardingApply(t *testing.T, dut *ondatra.DUTDevice, params OcPolicyForwardingParams) {
	t.Helper()
	// Check if the DUT requires CLI-based configuration due to an OpenConfig deviation.
	if deviations.InterfacePolicyForwardingOCUnsupported(dut) {
		// If deviations exist, apply configuration using vendor-specific CLI commands.
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if params.RemovePolicyName {
				helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s \n no traffic-policy input %s \n", params.InterfaceName, params.PolicyName))
			} else {
				pfa := fmt.Sprintf(`interface %s
				traffic-policy input %s`, params.InterfaceName, params.PolicyName)
				helpers.GnmiCLIConfig(t, dut, pfa)
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'policy-forwarding config'", dut.Vendor())
		}
	} else {
		// params.NetworkInstanceObj represents the OpenConfig network instance (default/non-default VRF) where the policy-forwarding configuration will be applied.
		// It provides access to the PolicyForwarding container for interface-level policy bindings.
		policyForward := params.NetworkInstanceObj.GetOrCreatePolicyForwarding()
		iface := policyForward.GetOrCreateInterface(params.InterfaceID)
		iface.ApplyForwardingPolicy = ygot.String(params.AppliedPolicyName)
	}
}

// ConfigureTrafficPolicyACL configures acl related configs
func ConfigureTrafficPolicyACL(t *testing.T, dut *ondatra.DUTDevice, params ACLTrafficPolicyParams) {
	if deviations.ConfigACLWithPrefixListNotSupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if len(params.SrcPrefix) != 0 && len(params.DstPrefix) != 0 {
				cliConfig += fmt.Sprintf(`
					traffic-policies
					traffic-policy %s
					match rule1 %s
					source prefix %s
					destination prefix %s
			`, params.PolicyName, params.ProtocolType, strings.Join(params.SrcPrefix, " "), strings.Join(params.DstPrefix, " "))
			}
			if params.DstPort != "" && params.SrcPort != "" {
				cliConfig += fmt.Sprintf(`protocol tcp source port %s destination port %s`, params.SrcPort, params.DstPort)
			}

			if params.Action != "" {
				cliConfig += fmt.Sprintf(`
				actions
				%s
				`, params.Action)
			}

			if params.IntfName != "" {
				cliConfig += fmt.Sprintf(`
					interface %s
					traffic-policy %s %s
				`, params.IntfName, params.Direction, params.PolicyName)
			}
		default:
			t.Errorf("traffic policy CLI is not handled for the dut: %v", dut.Vendor())
		}
		helpers.GnmiCLIConfig(t, dut, cliConfig)
	} else {
		// TODO: Created issue 41616436 for unsupport of prefix list inside ACL
		root := &oc.Root{}
		rp := root.GetOrCreateRoutingPolicy()
		prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(params.PolicyName)
		prefixSet.GetOrCreatePrefix(strings.Join(params.SrcPrefix, " "), "exact")
		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(params.PolicyName).Config(), prefixSet)
	}
}

// ConfigureVRFSelectionPolicyOC configures vrf_selection_policy_c on DUT port1.
func ConfigureVRFSelectionPolicyOC(t *testing.T, dut *ondatra.DUTDevice, encapVRFs []string) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	defaultVRF := deviations.DefaultNetworkInstance(dut)
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(defaultVRF)
	pf := ni.GetOrCreatePolicyForwarding()
	pol := pf.GetOrCreatePolicy(VRFPolC)
	pol.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	seq := uint32(1)

	for i, vrf := range encapVRFs {
		d1, d2 := EncapVRFDSCP(i)
		dscpSet := []uint8{d1, d2}
		for _, src := range []string{IPv4OuterSrc222, IPv4OuterSrc111} {
			fallback := TransitVRF222Str
			if src == IPv4OuterSrc111 {
				fallback = TransitVRF111Str
			}
			for _, proto := range []oc.UnionUint8{4, 41} {
				r := pol.GetOrCreateRule(seq)
				ip4 := r.GetOrCreateIpv4()
				ip4.Protocol = proto
				ip4.SourceAddress = ygot.String(fmt.Sprintf("%s/%d", src, IPv4HostMask))
				ip4.DscpSet = dscpSet
				act := r.GetOrCreateAction()
				act.DecapNetworkInstance = ygot.String(DecapVRFStr)
				act.PostDecapNetworkInstance = ygot.String(vrf)
				act.DecapFallbackNetworkInstance = ygot.String(fallback)
				seq++
			}
		}
	}

	for _, entry := range []struct {
		proto    oc.UnionUint8
		src      string
		fallback string
	}{
		{4, IPv4OuterSrc222, TransitVRF222Str},
		{41, IPv4OuterSrc222, TransitVRF222Str},
		{4, IPv4OuterSrc111, TransitVRF111Str},
		{41, IPv4OuterSrc111, TransitVRF111Str},
	} {
		r := pol.GetOrCreateRule(seq)
		ip4 := r.GetOrCreateIpv4()
		ip4.Protocol = entry.proto
		ip4.SourceAddress = ygot.String(fmt.Sprintf("%s/%d", entry.src, IPv4HostMask))
		act := r.GetOrCreateAction()
		act.DecapNetworkInstance = ygot.String(DecapVRFStr)
		act.PostDecapNetworkInstance = ygot.String(defaultVRF)
		act.DecapFallbackNetworkInstance = ygot.String(entry.fallback)
		seq++
	}

	for i, vrf := range encapVRFs {
		d1, d2 := EncapVRFDSCP(i)
		dscpSet := []uint8{d1, d2}
		r4 := pol.GetOrCreateRule(seq)
		r4.GetOrCreateIpv4().DscpSet = dscpSet
		r4.GetOrCreateAction().NetworkInstance = ygot.String(vrf)
		seq++
		r6 := pol.GetOrCreateRule(seq)
		r6.GetOrCreateIpv6().DscpSet = dscpSet
		r6.GetOrCreateAction().NetworkInstance = ygot.String(vrf)
		seq++
	}

	if deviations.PfRequireMatchDefaultRule(dut) {
		r4 := pol.GetOrCreateRule(seq)
		r4.GetOrCreateL2().SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4)
		r4.GetOrCreateAction().NetworkInstance = ygot.String(defaultVRF)
		seq++
		r6 := pol.GetOrCreateRule(seq)
		r6.GetOrCreateL2().SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6)
		r6.GetOrCreateAction().NetworkInstance = ygot.String(defaultVRF)
	} else {
		pol.GetOrCreateRule(seq).GetOrCreateAction().NetworkInstance = ygot.String(defaultVRF)
	}

	interfaceID := p1.Name()
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = p1.Name() + ".0"
	}
	intf := pf.GetOrCreateInterface(interfaceID)
	intf.ApplyVrfSelectionPolicy = ygot.String(VRFPolC)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(defaultVRF).PolicyForwarding().Config(), pf)
}

// ConfigureCLIDecapVRFMode enables next-hop decapsulation VRF mode required for VRF selection policy decapsulation forwarding.
func ConfigureCLIDecapVRFMode(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		return
	}
	cliConfig := `
		vrf selection policy
		next-hop decapsulation vrf
		!
		`
	t.Log("Enabling next-hop decapsulation VRF mode")
	helpers.GnmiCLIConfig(t, dut, cliConfig)
}

// GueDecapV6ScaleParams holds the parameters used to program a scaled MPLSoGUE
// decapsulation configuration matching on unique IPv6 outer headers.
type GueDecapV6ScaleParams struct {
	// PolicyID is the policy-forwarding policy name.
	PolicyID string
	// OuterSrcIPv6s is the list of unique outer IPv6 source addresses; one
	// decapsulation rule is programmed per address.
	OuterSrcIPv6s []string
	// SrcPrefixLen is the prefix length applied to each outer IPv6 source match.
	SrcPrefixLen int
	// DecapIPv6Prefix is the IPv6 prefix owned by the DUT that the outer
	// destination addresses fall within (the decap address range).
	DecapIPv6Prefix string
	// GUEPort is the UDP destination port carrying the MPLSoGUE payload.
	GUEPort uint32
	// IngressInterfaceIDs are the ingress aggregate interfaces the policy is
	// applied to.
	IngressInterfaceIDs []string
	// PfInstance is the policy-forwarding instance used to configure the decapsulation policy.
	PfInstance *oc.NetworkInstance_PolicyForwarding
	// Enabled indicates whether the decapsulation group should be configured or removed.
	Enabled             bool
	NetworkInstanceName string
}

// DecapGroupConfigGueV6Scale configures scaled MPLS-over-GUE decapsulation
// matching unique IPv6 outer headers. On platforms where policy-forwarding OC
// is unsupported the equivalent native configuration is pushed instead, so that
// the operational intent (decapsulate MPLSoGUE arriving on the IPv6 decap range
// and forward the inner payload) is identical in both cases.
func DecapGroupConfigGueV6Scale(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams) {
	t.Helper()
	if deviations.GueGreDecapUnsupported(dut) || deviations.PolicyForwardingOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			aristaGueDecapV6ScaleCLIConfig(t, dut, params)
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'decap-group config'", dut.Vendor())
		}
		return
	}
	decapPolicyRulesandActionsGueV6Scale(t, params)
}

// decapPolicyRulesandActionsGueV6Scale builds the OC policy-forwarding policy
// with one decapsulate-mpls-in-udp rule per unique outer IPv6 source address and
// applies it to the ingress interfaces.
func decapPolicyRulesandActionsGueV6Scale(t *testing.T, params GueDecapV6ScaleParams) {
	t.Helper()
	policy := params.PfInstance.GetOrCreatePolicy(params.PolicyID)
	policy.PolicyId = ygot.String(params.PolicyID)
	policy.Type = oc.Policy_Type_PBR_POLICY
	for i, src := range params.OuterSrcIPv6s {
		rule := policy.GetOrCreateRule(uint32(i + 1))
		v6 := rule.GetOrCreateIpv6()
		v6.SourceAddress = ygot.String(fmt.Sprintf("%s/%d", src, params.SrcPrefixLen))
		v6.Protocol = oc.PacketMatchTypes_IP_PROTOCOL_IP_UDP
		rule.GetOrCreateAction().DecapsulateMplsInUdp = ygot.Bool(true)
	}
	for _, intfID := range params.IngressInterfaceIDs {
		intf := params.PfInstance.GetOrCreateInterface(intfID)
		intf.ApplyForwardingPolicy = ygot.String(params.PolicyID)
		intf.GetOrCreateInterfaceRef().Interface = ygot.String(intfID)
	}
}

// aristaGueDecapV6ScaleCLIConfig configures MPLSoGUE decapsulation for an IPv6
// outer header on Arista.
//
// EOS models decapsulation groups as a single address-family agnostic construct
// under "ip decap-group"; there is no "ipv6 decap-group" hierarchy. The address
// family is inferred from the "tunnel decap-ip" value, so the IPv6 decap prefix
// is configured in the same "ip decap-group" block used for IPv4.
//
// The per-outer-source scale rules (one per unique outer IPv6 source address)
// are expressed as an IPv6 traffic-policy applied on ingress, which is the EOS
// equivalent of the OC policy-forwarding rules built by
// DecapPolicyRulesandActionsGueV6Scale.
func aristaGueDecapV6ScaleCLIConfig(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams) {
	t.Helper()
	helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("ip decap-group type udp destination port %d payload mpls\n!", params.GUEPort))
	aristaSetV6ScaleDecapGroup(t, dut, params, true)

	// Per-rule counters are not maintained unless the counter granularity is
	// enabled. This must be configured before the traffic policy is created and
	// applied, otherwise the rules are programmed without counter resources and
	// every "count" action keeps reporting 0 packets.
	enableCLITrafficPolicyCounters(t, dut)
	// One match rule per unique outer IPv6 source address.
	var b strings.Builder
	b.WriteString("traffic-policies\n")
	b.WriteString(fmt.Sprintf("   traffic-policy %s\n", params.PolicyID))
	for i, src := range params.OuterSrcIPv6s {
		b.WriteString(fmt.Sprintf("      match %s ipv6\n", aristaV6ScaleMatchName(i)))
		b.WriteString(fmt.Sprintf("         source prefix %s/%d\n", src, params.SrcPrefixLen))
		b.WriteString(fmt.Sprintf("         protocol udp destination port %d\n", params.GUEPort))
		b.WriteString("         actions\n")
		b.WriteString("            count\n")
		b.WriteString("         !\n")
	}
	b.WriteString("!\n")
	for _, intfID := range params.IngressInterfaceIDs {
		b.WriteString(fmt.Sprintf("interface %s\n   traffic-policy input %s\n!\n", intfID, params.PolicyID))
	}
	helpers.GnmiCLIConfig(t, dut, b.String())
}

// aristaV6ScaleDecapGroupTemplate is the decapsulation group carrying the
// MPLSoGUE traffic; the address family is inferred from the decap-ip value.
const aristaV6ScaleDecapGroupTemplate = `
ip decap-group %s
  tunnel type udp
  tunnel decap-ip %s
  tunnel overlay mpls qos map mpls-traffic-class to traffic-class
!`

// aristaSetV6ScaleDecapGroup creates or removes the MPLSoGUE decap group.
func aristaSetV6ScaleDecapGroup(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams, enabled bool) {
	t.Helper()
	if !enabled {
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("no ip decap-group %s\n!", params.PolicyID))
		return
	}
	helpers.GnmiCLIConfig(t, dut, fmt.Sprintf(aristaV6ScaleDecapGroupTemplate, params.PolicyID, params.DecapIPv6Prefix))
}

// aristaV6ScaleMatchPrefix is the name prefix used for the per-source traffic
// policy match rules, so that they can be counted back from the running config.
const aristaV6ScaleMatchPrefix = "gue-decap-v6-rule-"

// aristaTrafficPolicyCounterCmds lists the counter-granularity configurations
// used to enable per-rule traffic-policy counters. The accepted syntax differs
// between EOS releases and platforms, so each candidate is attempted until the
// device reports the counters as enabled.
// EOS only accepts "counter interface per-interface ingress" under
// traffic-policies ("egress", "per-vlan-interface" and "per-port" are not
// valid), and additionally requires the global hardware counter feature to be
// enabled on some platforms.
var aristaTrafficPolicyCounterCmds = []string{
	"traffic-policies\n   counter interface per-interface ingress\n!",
	"hardware counter feature traffic-policy in\n!",
}

// EnableTrafficPolicyCounters enables the per-rule traffic-policy packet
// counters on the DUT and reports whether they are active afterwards.
func EnableTrafficPolicyCounters(t *testing.T, dut *ondatra.DUTDevice) bool {
	t.Helper()
	if !deviations.PolicyRuleCountersOCUnsupported(dut) {
		// The OpenConfig rule counters are always maintained; there is no
		// corresponding enablement leaf in the model.
		return true
	}
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return enableCLITrafficPolicyCounters(t, dut)
	default:
		t.Logf("No native traffic-policy counter configuration known for vendor %s", dut.Vendor())
		return false
	}
}

// enableCLITrafficPolicyCounters enables the traffic-policy counter
// granularity so that the per-rule "count" actions maintain packet counters.
//
// It returns true when the device reports the ingress counters as enabled. The
// configuration is pushed non-fatally because unsupported syntax is rejected by
// some EOS releases/platforms.
func enableCLITrafficPolicyCounters(t *testing.T, dut *ondatra.DUTDevice) bool {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		return false
	}
	if aristaTrafficPolicyCountersEnabled(t, dut) {
		return true
	}
	for _, cmd := range aristaTrafficPolicyCounterCmds {
		if err := gnmiCLIConfigNonFatal(t, dut, cmd); err != nil {
			t.Logf("Traffic-policy counter config %q rejected: %v", strings.ReplaceAll(cmd, "\n", "; "), err)
			continue
		}
		t.Logf("Applied traffic-policy counter config: %s", strings.ReplaceAll(cmd, "\n", "; "))
	}
	// The counter granularity and the hardware counter feature are complementary
	// on EOS, so both are applied before re-checking.
	if aristaTrafficPolicyCountersEnabled(t, dut) {
		return true
	}
	t.Logf("WARNING: unable to enable traffic-policy counter granularity; per-rule packet counters will read 0")
	return false
}

// TrafficPolicyCountersEnabled reports whether the DUT maintains per-rule
// traffic-policy counters, so that callers can distinguish "no traffic matched"
// from "counters are not maintained by the platform".
func TrafficPolicyCountersEnabled(t *testing.T, dut *ondatra.DUTDevice) bool {
	t.Helper()
	if !deviations.PolicyRuleCountersOCUnsupported(dut) {
		return true
	}
	if dut.Vendor() != ondatra.ARISTA {
		return false
	}
	return aristaTrafficPolicyCountersEnabled(t, dut)
}

// aristaTrafficPolicyCountersEnabled reports whether the device maintains
// traffic-policy counters on ingress.
func aristaTrafficPolicyCountersEnabled(t *testing.T, dut *ondatra.DUTDevice) bool {
	t.Helper()
	// The authoritative source is the configuration itself: EOS only accepts
	// "counter interface per-interface ingress" when the granularity is
	// supported, and the show command may still print a per-interface
	// "counter not enabled" note for interfaces without a policy applied.
	cfg := helpers.RunCliCommand(t, dut, "show running-config all section traffic-policies")
	if strings.Contains(cfg, "counter interface per-interface ingress") {
		return true
	}
	out := helpers.RunCliCommand(t, dut, "show traffic-policy interface input counters")
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "counter granularity") && strings.Contains(line, "not enabled") {
			return false
		}
	}
	return true
}

// gnmiCLIConfigNonFatal pushes CLI configuration and returns the error instead
// of failing the test, so that alternative syntax can be attempted.
func gnmiCLIConfigNonFatal(t *testing.T, dut *ondatra.DUTDevice, config string) error {
	t.Helper()
	req, err := helpers.BuildCliConfigRequest(config)
	if err != nil {
		return err
	}
	_, err = dut.RawAPIs().GNMI(t).Set(context.Background(), req)
	return err
}

// aristaV6ScaleMatchName returns the traffic-policy match rule name for the i-th
// outer IPv6 source address.
func aristaV6ScaleMatchName(i int) string {
	return fmt.Sprintf("%s%d", aristaV6ScaleMatchPrefix, i+1)
}

// GueDecapV6ScaleRuleCounters returns the matched packet count of every
// per-outer-source decap rule programmed through the native/CLI path, keyed by
// the rule name. It is the native equivalent of the OC
// .../rules/rule/state/matched-pkts telemetry.
//
// A nil map is returned when the vendor has no native implementation.
func GueDecapV6ScaleRuleCounters(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams) map[string]uint64 {
	t.Helper()
	if !deviations.PolicyRuleCountersOCUnsupported(dut) {
		return gueDecapV6ScaleRuleCountersOC(t, dut, params)
	}
	switch dut.Vendor() {
	case ondatra.ARISTA:
		// "show traffic-policy interface input counters" reports the ingress
		// match counters of the applied policy, which is what the per-rule
		// "count" actions maintain.
		if counters := parseAristaTrafficPolicyCounters(helpers.RunCliCommand(t, dut, "show traffic-policy interface input counters")); anyNonZero(counters) {
			return counters
		}
		if counters := parseAristaTrafficPolicyCounters(helpers.RunCliCommand(t, dut, fmt.Sprintf("show traffic-policy %s counters", params.PolicyID))); anyNonZero(counters) {
			return counters
		}
		t.Logf("No non-zero traffic-policy counters found for policy %q", params.PolicyID)
		return map[string]uint64{}
	default:
		t.Logf("Unsupported vendor %s for native traffic-policy counter verification", dut.Vendor())
		return nil
	}
}

// gueDecapV6ScaleRuleCountersOC reads the OpenConfig matched-pkts counter of
// every decap rule, keyed by the same rule names the native path reports so
// that both paths are interchangeable to callers.
func gueDecapV6ScaleRuleCountersOC(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams) map[string]uint64 {
	t.Helper()
	policyPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(params.PolicyID)
	counters := make(map[string]uint64, len(params.OuterSrcIPv6s))
	for i := range params.OuterSrcIPv6s {
		pkts, ok := gnmi.Lookup(t, dut, policyPath.Rule(uint32(i+1)).MatchedPkts().State()).Val()
		if !ok {
			continue
		}
		counters[aristaV6ScaleMatchName(i)] = pkts
	}
	return counters
}

// anyNonZero reports whether at least one counter recorded traffic.
func anyNonZero(counters map[string]uint64) bool {
	for _, v := range counters {
		if v > 0 {
			return true
		}
	}
	return false
}

// parseAristaTrafficPolicyCounters extracts the matched packet counts from the
// output of "show traffic-policy <name> counters".
//
// EOS renders the counters as:
//
//	Traffic policy gue-decap-scale-v6
//	   match rule: gue-decap-v6-rule-1: 12345 packets
//
// Tabular output where the rule name is the first field followed by the packet
// count is also accepted.
func parseAristaTrafficPolicyCounters(out string) map[string]uint64 {
	counters := map[string]uint64{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		name := ""
		rest := []string{}
		switch {
		case len(fields) >= 4 && fields[0] == "match" && fields[1] == "rule:":
			// "match rule: <name>: <n> packets"
			name = strings.TrimSuffix(fields[2], ":")
			rest = fields[3:]
		case len(fields) >= 2:
			name = strings.TrimSuffix(fields[0], ":")
			rest = fields[1:]
		}
		if !strings.HasPrefix(name, aristaV6ScaleMatchPrefix) {
			continue
		}
		for _, f := range rest {
			// The first purely numeric column is the packet count.
			if v, err := strconv.ParseUint(strings.ReplaceAll(f, ",", ""), 10, 64); err == nil {
				counters[name] = v
				break
			}
		}
	}
	return counters
}

// SetGueDecapV6ScaleDecapGroup enables or disables the MPLSoGUE decap group.
//
// Disabling it makes the DUT forward the encapsulated packets without
// terminating the tunnel, so that the outer IPv6 header remains visible to the
// ingress classifiers and the per-outer-source counters can be observed.
func SetGueDecapV6ScaleDecapGroup(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams) {
	t.Helper()
	if !deviations.GueGreDecapUnsupported(dut) && !deviations.PolicyForwardingOCUnsupported(dut) {
		setGueDecapV6ScaleDecapActionOC(t, dut, params, params.Enabled)
		return
	}
	if dut.Vendor() != ondatra.ARISTA {
		t.Logf("Unsupported vendor %s for native MPLSoGUE decap group configuration", dut.Vendor())
		return
	}
	aristaSetV6ScaleDecapGroup(t, dut, params, params.Enabled)
}

// setGueDecapV6ScaleDecapActionOC toggles the decapsulate-mpls-in-udp action of
// every decap rule in a single batched gNMI Set, leaving the match criteria (and
// therefore the per-rule counters) in place.
func setGueDecapV6ScaleDecapActionOC(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams, enabled bool) {
	t.Helper()
	policyPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(params.PolicyID)
	sb := &gnmi.SetBatch{}
	for i := range params.OuterSrcIPv6s {
		gnmi.BatchUpdate(sb, policyPath.Rule(uint32(i+1)).Action().DecapsulateMplsInUdp().Config(), enabled)
	}
	sb.Set(t, dut)
	t.Logf("Set decapsulate-mpls-in-udp=%v on %d rules of policy %q", enabled, len(params.OuterSrcIPv6s), params.PolicyID)
}

// ClearGueDecapV6ScaleCounters resets the per-outer-source counters so that a
// subsequent traffic run can be measured in isolation.
func ClearGueDecapV6ScaleCounters(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams) {
	t.Helper()
	if !deviations.PolicyRuleCountersOCUnsupported(dut) {
		t.Logf("Policy rule counters are read-only in OpenConfig; skipping counter reset for policy %q", params.PolicyID)
		return
	}
	if dut.Vendor() == ondatra.ARISTA {
		if err := gnmiCLIConfigNonFatal(t, dut, "clear traffic-policy counters"); err != nil {
			t.Logf("Clearing traffic-policy counters failed: %v", err)
		}
	}

}

// GueDecapV6ScaleRuleNames returns the names of all per-outer-source decap rules
// that DecapGroupConfigGueV6Scale programs for the given parameters.
func GueDecapV6ScaleRuleNames(params GueDecapV6ScaleParams) []string {
	names := make([]string, 0, len(params.OuterSrcIPv6s))
	for i := range params.OuterSrcIPv6s {
		names = append(names, aristaV6ScaleMatchName(i))
	}
	return names
}

// CountGueDecapV6ScaleRulesCLI returns the number of per-outer-source decap
// rules currently programmed through the native/CLI path. It is the CLI
// equivalent of counting the OC policy-forwarding rules in state.
func CountGueDecapV6ScaleRulesCLI(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams) int {
	t.Helper()
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		return countGueDecapV6ScaleRulesOC(t, dut, params)
	}
	switch dut.Vendor() {
	case ondatra.ARISTA:
		// "show running-config section" takes a regular expression and only
		// prints the matching sections, which does not reliably include the
		// nested match rules. Read the whole traffic-policies configuration and
		// count the generated rule names instead.
		out := helpers.RunCliCommand(t, dut, "show running-config all section traffic-policies")
		if !strings.Contains(out, aristaV6ScaleMatchPrefix) {
			out = helpers.RunCliCommand(t, dut, "show running-config")
		}
		count := 0
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "match" && strings.HasPrefix(fields[1], aristaV6ScaleMatchPrefix) {
				count++
			}
		}
		if count == 0 {
			// Help debugging: show whether the policy exists at all and whether
			// it is applied to the ingress interfaces.
			t.Logf("No %q match rules found. traffic-policy %q present in running-config: %v", aristaV6ScaleMatchPrefix, params.PolicyID, strings.Contains(out, "traffic-policy "+params.PolicyID))
			t.Logf("traffic-policies running-config:\n%s", out)
		}
		return count
	default:
		t.Logf("Unsupported vendor %s for CLI verification of scaled GUE decap rules", dut.Vendor())
		return -1
	}
}

// countGueDecapV6ScaleRulesOC counts the decap rules present in the OpenConfig
// policy-forwarding state of the policy.
func countGueDecapV6ScaleRulesOC(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams) int {
	t.Helper()
	policyPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(params.PolicyID)
	count := 0
	for _, seq := range gnmi.LookupAll(t, dut, policyPath.RuleAny().SequenceId().State()) {
		if _, ok := seq.Val(); ok {
			count++
		}
	}
	return count
}

// RemoveDecapGroupConfigGueV6Scale reverts the configuration applied by
// DecapGroupConfigGueV6Scale.
func RemoveDecapGroupConfigGueV6Scale(t *testing.T, dut *ondatra.DUTDevice, params GueDecapV6ScaleParams) {
	t.Helper()
	if deviations.GueGreDecapUnsupported(dut) || deviations.PolicyForwardingOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			var b strings.Builder
			for _, intfID := range params.IngressInterfaceIDs {
				b.WriteString(fmt.Sprintf("interface %s\n   no traffic-policy input\n!\n", intfID))
			}
			b.WriteString(fmt.Sprintf("traffic-policies\n   no traffic-policy %s\n   no counter interface per-interface ingress\n!\n", params.PolicyID))
			b.WriteString(fmt.Sprintf("no ip decap-group %s\nno ip decap-group type udp destination port %d payload mpls\n!\n", params.PolicyID, params.GUEPort))
			helpers.GnmiCLIConfig(t, dut, b.String())
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'decap-group config'", dut.Vendor())
		}
		return
	}
	pfPath := gnmi.OC().NetworkInstance(params.NetworkInstanceName).PolicyForwarding()
	for _, intfID := range params.IngressInterfaceIDs {
		gnmi.Delete(t, dut, pfPath.Interface(intfID).ApplyForwardingPolicy().Config())
	}
	gnmi.Delete(t, dut, pfPath.Policy(params.PolicyID).Config())
}
