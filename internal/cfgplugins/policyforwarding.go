package cfgplugins

import (
	"context"
	"fmt"
	"maps"
	"slices"
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
}

// OcPolicyForwardingParams holds parameters for generating the OC Policy Forwarding config.
type OcPolicyForwardingParams struct {
	NetworkInstanceName string
	InterfaceID         string
	AppliedPolicyName   string

	// Policy Rule specific params
	InnerDstIPv6 string
	InnerDstIPv4 string
	CloudV4NHG   string
	CloudV6NHG   string
	DecapPolicy  DecapPolicyParams
	GuePort      uint32
	IpType       string
	Dynamic      bool
	TunnelIP     string
}

type PolicyForwardingRule struct {
	Id                 uint32
	Name               string
	IpType             string
	SourceAddress      string
	DestinationAddress string
	Dscp               uint8
	Action             *oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action
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
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d \n traffic-policy input tp_cloud_id_3_%d \n", aggID, a.Subinterface, a.Subinterface))
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

// MplsGlobalStaticLspAttributes configures the MPLS global static LSP attributes.
func MplsGlobalStaticLspAttributes(t *testing.T, ni *oc.NetworkInstance, params OcPolicyForwardingParams) {
	t.Helper()
	mplsCfgv4 := ni.GetOrCreateMpls()
	staticMplsCfgv4 := mplsCfgv4.GetOrCreateLsps().GetOrCreateStaticLsp(params.DecapPolicy.StaticLSPNameIPv4)
	egressv4 := staticMplsCfgv4.GetOrCreateEgress()
	egressv4.IncomingLabel = oc.UnionUint32(params.DecapPolicy.StaticLSPLabelIPv4)
	egressv4.NextHop = ygot.String(params.DecapPolicy.StaticLSPNextHopIPv4)

	mplsCfgv6 := ni.GetOrCreateMpls()
	staticMplsCfgv6 := mplsCfgv6.GetOrCreateLsps().GetOrCreateStaticLsp(params.DecapPolicy.StaticLSPNameIPv6)
	egressv6 := staticMplsCfgv6.GetOrCreateEgress()
	egressv6.IncomingLabel = oc.UnionUint32(params.DecapPolicy.StaticLSPLabelIPv6)
	egressv6.NextHop = ygot.String(params.DecapPolicy.StaticLSPNextHopIPv6)
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
				t.Logf("Going into decap")
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

// aristaGueDecapCLIConfig configures GUEDEcapConfig for Arista
func aristaGueDecapCLIConfig(t *testing.T, dut *ondatra.DUTDevice, params OcPolicyForwardingParams) {

	cliConfig := fmt.Sprintf(`
		                    ip decap-group type udp destination port %v payload %s
							tunnel type %s-over-udp udp destination port %v
							ip decap-group %s
							tunnel type UDP
							tunnel decap-ip %s
							tunnel decap-interface %s
							`, params.GuePort, params.IpType, params.IpType, params.GuePort, params.AppliedPolicyName, params.TunnelIP, params.InterfaceID)
	helpers.GnmiCLIConfig(t, dut, cliConfig)

}

// aristaGreDecapCLIConfig configures GREDEcapConfig for Arista
func aristaGreDecapCLIConfig(t *testing.T, dut *ondatra.DUTDevice, params OcPolicyForwardingParams) {

	cliConfig := fmt.Sprintf(`
			ip decap-group %s
			 tunnel type gre
			 tunnel decap-ip %s
			`, params.AppliedPolicyName, params.TunnelIP)
	helpers.GnmiCLIConfig(t, dut, cliConfig)

}

// MPLSStaticLSPConfig configures the interface mpls static lsp.
func MPLSStaticLSPConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance, ocPFParams OcPolicyForwardingParams) {
	if deviations.StaticMplsUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, staticLSPArista)
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'mpls static lsp'", dut.Vendor())
		}
	} else {
		MplsGlobalStaticLspAttributes(t, ni, ocPFParams)
	}
}

// Configure GRE decapsulated. Adding deviation when device doesn't support OC
func PolicyForwardingGreDecapsulation(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, decapIp string, policyName string, portName string, decapGrpName string) {
	if deviations.GreDecapsulationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig := fmt.Sprintf(`
			ip decap-group %s
			 tunnel type gre
			 tunnel decap-ip %s
			`, decapGrpName, strings.Split(decapIp, "/")[0])
			helpers.GnmiCLIConfig(t, dut, cliConfig)

		default:
			t.Errorf("Deviation GreDecapsulationUnsupported is not handled for the dut: %v", dut.Vendor())
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
		ip.DestinationAddressPrefixSet = ygot.String(decapIp)
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
	tpConfig := getTrafficPolicyCliConfig(t, dut, policyName, interfaceName, targetName, rules)
	t.Logf("Push the CLI Policy config:%s", dut.Vendor())
	gpbSetRequest := buildCliSetRequest(tpConfig)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Errorf("Failed to set policy forwarding from cli: %v", err)
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
			ruleIpv4 := rule.GetOrCreateIpv4()
			if ruleConfig.SourceAddress != "" {
				ruleIpv4.SourceAddress = ygot.String(ruleConfig.SourceAddress)
			}
			if ruleConfig.DestinationAddress != "" {
				ruleIpv4.DestinationAddress = ygot.String(ruleConfig.DestinationAddress)
			}
			if ruleConfig.Dscp != 0 {
				ruleIpv4.Dscp = ygot.Uint8(ruleConfig.Dscp)
			}
		case IPv6:
			ruleIpv6 := rule.GetOrCreateIpv6()
			if ruleConfig.SourceAddress != "" {
				ruleIpv6.SourceAddress = ygot.String(ruleConfig.SourceAddress)
			}
			if ruleConfig.DestinationAddress != "" {
				ruleIpv6.DestinationAddress = ygot.String(ruleConfig.DestinationAddress)
			}
			if ruleConfig.Dscp != 0 {
				ruleIpv6.Dscp = ygot.Uint8(ruleConfig.Dscp)
			}
		default:
			t.Errorf("Unknown IP type %s in PolicyForwardingRule", ruleConfig.IpType)
			return
		}
		if ruleConfig.Action != nil {
			rule.Action = ruleConfig.Action
		}
	}
}

func getTrafficPolicyCliConfig(t *testing.T, dut *ondatra.DUTDevice, policyName string, interfaceName string, targetName string, rules []PolicyForwardingRule) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var matchRules, matchTarget string
		var nhGroupTargets = make(map[string][]string)
		var nhGroupsBySource = make(map[string]string)
		var nhTTlBySource = make(map[string]uint8)
		for _, ruleConfig := range rules {
			t.Logf("Processing rule %s", ruleConfig.Name)
			if ruleConfig.Action == nil ||
				ruleConfig.Name == "" {
				t.Errorf("Invalid rule configuration: %v", ruleConfig)
				return ""
			}
			if ruleConfig.DestinationAddress != "" {
				matchTarget = fmt.Sprintf("destination prefix %s", ruleConfig.DestinationAddress)
			} else if ruleConfig.SourceAddress != "" {
				matchTarget = fmt.Sprintf("source prefix %s", ruleConfig.SourceAddress)
			} else {
				t.Errorf("Rule %s must have either SourceAddress or DestinationAddress defined", ruleConfig.Name)
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
								t.Errorf("Target in EncapsulateGre action must have Source and Destination defined")
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
					nhGroups := ""
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
				t.Errorf("Unknown IP type %s in PolicyForwardingRule %s", ruleConfig.IpType, ruleConfig.Name)
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
			t.Errorf("Deviation GreDecapsulationUnsupported is not handled for the dut: %v", dut.Vendor())
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

func ConfigureDutWithGueDecap(t *testing.T, dut *ondatra.DUTDevice, guePort int, ipType, tunIp, decapInt, policyName string, policyId int) {
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
                            `, guePort, ipType, tunIp, decapInt)
			helpers.GnmiCLIConfig(t, dut, cliConfig)

		default:
			t.Errorf("Deviation decapsulateGueOCUnsupported is not handled for the dut: %v", dut.Vendor())
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
		ip.DestinationAddressPrefixSet = ygot.String(tunIp)
		ip.Protocol = oc.PacketMatchTypes_IP_PROTOCOL_IP_UDP
		// transport := npRule.GetOrCreateTransport()
		// transport.SetDestinationPort()
	}
}
