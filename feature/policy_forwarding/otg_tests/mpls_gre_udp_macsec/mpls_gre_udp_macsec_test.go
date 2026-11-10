package mpls_gre_udp_macsec

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	baseSubinterfaceIndex = 0

	ateCustLagName     = "Lag1"
	ateCoreLagName     = "Lag2"
	ateCoreLag2Name    = "Lag3"
	ateCustDev         = "d1"
	ateCustClearDev    = "d5"
	ateCustGUEClearDev = "d6"
	ateCoreDev         = "d2"
	ateCustGUEDev      = "d3"
	ateCore2Dev        = "d4"

	custVlan    = 20
	custVlanGUE = 30

	flowIPv4Fwd             = "Flow-IPv4-Encap"
	flowIPv6Fwd             = "Flow-IPv6-Encap"
	flowIPv4Decap           = "Flow-IPv4-Decap"
	flowIPv4DecapCore2      = "Flow-IPv4-Decap-Core2"
	flowShouldSecureIPv4    = "Flow-Should-Secure-IPv4-Cleartext"
	flowShouldSecureIPv6    = "Flow-Should-Secure-IPv6-Cleartext"
	flowGUEIPv4Fwd          = "Flow-GUE-IPv4-Encap"
	flowGUEIPv6Fwd          = "Flow-GUE-IPv6-Encap"
	flowGUEIPv4Decap        = "Flow-GUE-IPv4-Decap"
	flowGUEIPv4DecapCore2   = "Flow-GUE-IPv4-Decap-Core2"
	flowShouldSecureGUEIPv4 = "Flow-Should-Secure-GUE-IPv4-Cleartext"
	flowShouldSecureGUEIPv6 = "Flow-Should-Secure-GUE-IPv6-Cleartext"

	trafficPPS                = 100
	packetCapturePPS          = trafficPPS
	forwardTrafficPercent     = float32(25)
	shouldSecureFlowPercent   = float32(25)
	reverseTrafficPercent     = float32(25)
	trafficDuration           = time.Second
	forwardingTrafficDuration = 10 * time.Second
	packetCaptureDuration     = time.Second
	maxFlowValidationAttempts = 2
	maxForwardingLossPercent  = 1.0
	minDropLossPercent        = 99.0
	standardSAKRekeyInterval  = 28800
	macsecReplayProtection    = 64

	sessionUpTimeout = 90 * time.Second

	macsecNumPSKs                = 5
	macsecAnchorOffset           = 30 * time.Second
	macsecKeyLifetime            = 2 * time.Minute
	macsecKeyStagger             = 1 * time.Minute
	macsecKeyServerPriority      = 15
	macsecSAKRekeyInterval       = 30
	macsecUpTimeout              = macsecAnchorOffset + macsecChainDuration
	macsecChainDuration          = (macsecNumPSKs-1)*macsecKeyStagger + macsecKeyLifetime
	macsecPostExpiryMargin       = 10 * time.Second
	macsecRotationSetupExtension = 2 * time.Minute

	mustSecureSSProfile   = "MACSEC-KEYS-MUST-SECURE"
	shouldSecureSSProfile = "MACSEC-KEYS-SHOULD-SECURE"
	mustSecureProfile     = "MUST-SECURE"
	shouldSecureProfile   = "SHOULD-SECURE"

	mplsGreNHGName       = "MPLSOGRE_NHG"
	mplsGreTrafficPolicy = "MPLSOGRE_TRAFFIC_POLICY"
	mplsGreLabel         = 16000
	mplsGreOuterTTL      = 64

	decapGroupName = "gre-decap"
	decapTunnelIP  = "192.0.2.0/24"
	decapOuterDst  = "192.0.2.3"
	decapOuterSrc  = "100.64.0.1"
	decapLabel     = 99991
	decapInnerSrc  = "203.0.113.1"
	decapInnerDst  = "198.18.0.10"
	mplsEthertype  = 34887

	mplsGueNHGName       = "MPLSOGUE_NHG"
	mplsGueTrafficPolicy = "MPLSOGUE_TRAFFIC_POLICY"
	mplsGueLabel         = 16001
	mplsGueOuterTTL      = 64
	mplsGueUDPDstPort    = 6635
	mplsGueTunnelDst     = "198.18.2.1"

	gueDecapGroupName = "gue-decap"
	gueDecapTunnelIP  = "198.51.100.0/24"
	gueDecapOuterDst  = "198.51.100.3"
	gueDecapOuterSrc  = "100.64.0.2"
	gueDecapLabel     = 99992
	gueDecapInnerSrc  = "203.0.113.2"
	gueDecapInnerDst  = "198.18.0.18"
)

var (
	// DUT/ATE port groupings.
	// custPorts is the customer-facing (MACsec) 2-member aggregate; corePorts1 and
	// corePorts2 are the two core aggregates (MPLSoGRE/GUE egress, no MACsec) that
	// the tunnel traffic is ECMP load-balanced across.
	custPorts  = []string{"port1", "port2"}
	corePorts1 = []string{"port3", "port4"}
	corePorts2 = []string{"port5", "port6"}

	// macsecPeerNames holds the OTG MACsec secure-entity/peer name per customer LAG member port.
	macsecPeerNames = []string{"Peer A", "Peer B"}

	// MACsec 5-PSK rotation chain (PF-1.17.1): each pre-shared key has a unique
	// 64-char CKN (CAK Name) and 64-char CAK (256-bit, GCM-AES-XPN-256). Index i
	// maps to the DUT `secret <CKN> 0 <CAK>` line and the OTG PSK CakName/CakValue.
	macsecCKNs = []string{
		"0123456789abcdef0123456789abcdeff123456789abcdef0123456789abcde1",
		"0123456789abcdef0123456789abcdeff123456789abcdef0123456789abcde2",
		"0123456789abcdef0123456789abcdeff123456789abcdef0123456789abcde3",
		"0123456789abcdef0123456789abcdeff123456789abcdef0123456789abcde4",
		"0123456789abcdef0123456789abcdeff123456789abcdef0123456789abcde5",
	}
	macsecCAKs = []string{
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde1",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde2",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde3",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde4",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde5",
	}

	// DUT customer subinterface: a single dual-stack subinterface on custVlan.
	custIntf = attrs.Attributes{
		Desc:         "Customer dual-stack",
		MTU:          1500,
		IPv4:         "198.18.0.9",
		IPv4Len:      29,
		IPv6:         "2001:db8:0:1::1",
		IPv6Len:      125,
		Subinterface: custVlan,
	}
	// DUT GUE customer subinterface: a second dual-stack subinterface (on the same
	// customer LAG) used for the MPLSoGUE encap/decap path.
	custIntfGUE = attrs.Attributes{
		Desc:         "Customer dual-stack GUE",
		MTU:          1500,
		IPv4:         "198.18.0.17",
		IPv4Len:      29,
		IPv6:         "2001:db8:0:2::1",
		IPv6Len:      125,
		Subinterface: custVlanGUE,
	}
	// DUT core interface 1 (corePorts1, MPLSoGRE/GUE egress).
	coreIntf = attrs.Attributes{
		Desc:    "Core interface",
		IPv4:    "192.0.2.1",
		IPv4Len: 24,
		IPv6:    "2001:db8:0:3::1",
		IPv6Len: 126,
		MTU:     9202,
	}
	// DUT core interface 2 (corePorts2, second MPLSoGRE/GUE egress for ECMP).
	coreIntf2 = attrs.Attributes{
		Desc:    "Core interface 2",
		IPv4:    "198.51.100.1",
		IPv4Len: 24,
		IPv6:    "2001:db8:0:4::1",
		IPv6Len: 126,
		MTU:     9202,
	}

	// MPLSoGRE tunnel endpoints: source is the DUT core interface, destination is
	// routed to the ATE core via the static route.
	mplsGreTunnelSrc = coreIntf.IPv4
	mplsGreTunnelDst = "198.18.2.1"

	// ATE customer addresses (single dual-stack device).
	ateCustV4   = "198.18.0.10"
	ateCustV6   = "2001:db8:0:1::2"
	ateCustV4Gw = custIntf.IPv4
	ateCustV6Gw = custIntf.IPv6
	ateCustMAC  = "00:00:11:01:01:01"
	// Per-member port MACs for the customer LAG.
	ateCustLagPMACs = []string{"00:16:01:00:00:02", "00:16:01:00:00:03"}

	// ATE GUE customer addresses (second dual-stack device on the customer LAG).
	ateCustGUEV4   = "198.18.0.18"
	ateCustGUEV6   = "2001:db8:0:2::2"
	ateCustGUEV4Gw = custIntfGUE.IPv4
	ateCustGUEV6Gw = custIntfGUE.IPv6
	ateCustGUEMAC  = "00:00:11:03:03:03"

	// ATE core LAG 1 addresses (corePorts1 device d2).
	ateCoreV4   = "192.0.2.2"
	ateCoreV6   = "2001:db8:0:3::2"
	ateCoreV4Gw = coreIntf.IPv4
	ateCoreV6Gw = coreIntf.IPv6
	ateCoreMAC  = "00:00:12:02:02:03"
	// Per-member port MACs for core LAG 1.
	ateCoreLagPMACs = []string{"00:17:01:00:00:01", "00:17:01:00:00:02"}

	// ATE core LAG 2 addresses (corePorts2 device d4).
	ateCore2V4   = "198.51.100.2"
	ateCore2V6   = "2001:db8:0:4::2"
	ateCore2V4Gw = coreIntf2.IPv4
	ateCore2V6Gw = coreIntf2.IPv6
	ateCore2MAC  = "00:00:12:02:02:06"
	// Per-member port MACs for core LAG 2.
	ateCore2LagPMACs = []string{"00:17:01:00:00:03", "00:17:01:00:00:04"}

	// Inner (payload) destinations for the encapsulated flows.
	innerV4Dst = "198.18.2.1"
	innerV6Dst = "2001:db8:0:5::1"

	// IMIX profile shared by all flows.
	sizeWeightProfile = []sizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 20},
		{Size: 1024, Weight: 20},
		{Size: 1500, Weight: 20},
	}
)

// Encapsulated-packet validation on the core port. It confirms MPLSoGRE
// encapsulation by matching the pushed MPLS label (deterministic from the
// next-hop-group configuration).
var (
	encapValidations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateMPLSLayer,
	}
)

// GUE-encapsulated-packet validation on the core port. MPLSoGUE encapsulation is
// confirmed by the outer UDP destination port (RFC 7510, 6635); the DUT derives
// the UDP source port from a flow hash, so it is not checked. It reuses the same
// core-port capture as the GRE encap validation.
var (
	gueEncapValidations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateUDPHeader,
		packetvalidationhelpers.ValidateMPLSLayer,
	}
)

// Decapsulated-packet validation on the customer LAG member ports. In the
// reverse direction the DUT pops the MPLSoGRE header and delivers the inner
// IPv4 packet to the customer MACsec-encrypted, so on the wire it appears as a
// MACsec (802.1AE) frame. The decap flow hashes to a single LAG member, so we
// capture on every customer port and require the MACsec frame on at least one.
var (
	decapValidations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateMacsecHeader,
	}
)

// configureDUT configures customer MACsec, core LAGs, routing, and enabled MPLS paths.
type dutTestConfig struct {
	custAggID   string
	coreAggID   string
	core2AggID  string
	macsec      bool
	staticRoute bool
	greEncap    bool
	gueEncap    bool
	greDecap    bool
	gueDecap    bool
}

// sizeWeightPair defines a frame size and its relative weight for the IMIX.
type sizeWeightPair struct {
	Size   uint32
	Weight float32
}

// ateOptions parameterizes ATE topology generation.
type ateOptions struct {
	anchor              time.Time
	initialKeyExtension time.Duration
	// corruptKeys sets a mismatched CAK on every customer MKA peer so MKA cannot
	// secure the link, used by the must-/should-secure negative scenarios.
	corruptKeys       bool
	shouldSecureProbe bool
	probeOnly         bool
	dutCustMAC        string
	dutCoreMAC        string
	dutCore2MAC       string
}

// encapFlowSet parameterizes the pair of IPv4/IPv6 customer -> core encap flows.
// rxV4Names/rxV6Names list every core device that may receive the encapped
// traffic (both core LAGs under ECMP).
type encapFlowSet struct {
	v4Name, v6Name             string
	txV4, txV6                 string
	rxV4Names, rxV6Names       []string
	srcMAC                     string
	dstMAC                     string
	includeMACsec              bool
	ratePercent                float32
	vlan                       uint32
	srcV4, dstV4, srcV6, dstV6 string
}

// testArgs holds shared handles for the PF-1.17 subtests. anchor/top are mutated
// by scenarios that re-anchor or re-push the MACsec chain.
type testArgs struct {
	dut               *ondatra.DUTDevice
	ate               *ondatra.ATEDevice
	top               gosnappi.Config
	anchor            time.Time
	custAggID         string
	shouldSecureProbe bool
	dutCustMAC        string
	dutCoreMAC        string
	dutCore2MAC       string
}

type flowCounterSnapshot struct {
	tx uint64
	rx uint64
}

type portFrameCounters struct {
	in  uint64
	out uint64
}

func macsecKeyChainForProfile(profile string) string {
	if profile == shouldSecureProfile {
		return shouldSecureSSProfile
	}
	return mustSecureSSProfile
}

func cleanupDUTConfig(t *testing.T, dut *ondatra.DUTDevice, config *dutTestConfig) {
	t.Helper()
	if config.macsec {
		bindingBatch := &gnmi.SetBatch{}
		cfgplugins.ConfigureMACsecProfileBinding(t, dut, bindingBatch, cfgplugins.MACsecProfileBindingConfig{
			PortIDs: custPorts,
			Remove:  true,
		})
		bindingBatch.Set(t, dut)
		if deviations.MacsecOcUnsupported(dut) {
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf(`
mac security
   no profile %s
   no profile %s
!
management security
   no session shared-secret profile %s
   no session shared-secret profile %s
			`, mustSecureProfile, shouldSecureProfile, mustSecureSSProfile, shouldSecureSSProfile))
		} else {
			for _, profile := range []string{mustSecureProfile, shouldSecureProfile} {
				gnmi.Delete(t, dut, gnmi.OC().Macsec().Mka().Policy(profile).Config())
				gnmi.Delete(t, dut, gnmi.OC().Keychain(macsecKeyChainForProfile(profile)).Config())
			}
		}
	}

	if config.greEncap || config.gueEncap {
		if deviations.PolicyForwardingOCUnsupported(dut) || deviations.NextHopGroupOCUnsupported(dut) {
			var cli strings.Builder
			if config.greEncap {
				fmt.Fprintf(&cli, "no nexthop-group %s\ninterface %s.%d\n   no traffic-policy input %s\n!\n", mplsGreNHGName, config.custAggID, custVlan, mplsGreTrafficPolicy)
			}
			if config.gueEncap {
				fmt.Fprintf(&cli, "no nexthop-group %s\ninterface %s.%d\n   no traffic-policy input %s\n!\n", mplsGueNHGName, config.custAggID, custVlanGUE, mplsGueTrafficPolicy)
			}
			cli.WriteString("traffic-policies\n")
			if config.greEncap {
				fmt.Fprintf(&cli, "   no traffic-policy %s\n", mplsGreTrafficPolicy)
			}
			if config.gueEncap {
				fmt.Fprintf(&cli, "   no traffic-policy %s\n", mplsGueTrafficPolicy)
			}
			cli.WriteString("!\n")
			helpers.GnmiCLIConfig(t, dut, cli.String())
		} else {
			policyForwarding := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
			if config.greEncap {
				gnmi.Delete(t, dut, policyForwarding.Interface(fmt.Sprintf("%s.%d", config.custAggID, custVlan)).Config())
				gnmi.Delete(t, dut, policyForwarding.Policy(mplsGreTrafficPolicy).Config())
			}
			if config.gueEncap {
				gnmi.Delete(t, dut, policyForwarding.Interface(fmt.Sprintf("%s.%d", config.custAggID, custVlanGUE)).Config())
				gnmi.Delete(t, dut, policyForwarding.Policy(mplsGueTrafficPolicy).Config())
			}
		}
	}

	if config.greDecap || config.gueDecap {
		if deviations.GueGreDecapUnsupported(dut) || deviations.PolicyForwardingOCUnsupported(dut) {
			var cli strings.Builder
			if config.greDecap {
				fmt.Fprintf(&cli, "no ip decap-group %s\n", decapGroupName)
			}
			if config.gueDecap {
				fmt.Fprintf(&cli, "no ip decap-group %s\nno ip decap-group type udp destination port %d payload mpls\n", gueDecapGroupName, mplsGueUDPDstPort)
			}
			helpers.GnmiCLIConfig(t, dut, cli.String())
		}
		if !deviations.PolicyForwardingOCUnsupported(dut) {
			policyForwarding := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
			gnmi.Delete(t, dut, policyForwarding.Policy("customer10").Config())
		}
		for _, cleanup := range []struct {
			configured bool
			name       string
			label      uint32
			next       string
		}{
			{config.greDecap, decapGroupName + "-lsp", decapLabel, ateCustV4},
			{config.gueDecap, gueDecapGroupName + "-lsp", gueDecapLabel, ateCustGUEV4},
		} {
			if !cleanup.configured {
				continue
			}
			if deviations.StaticMplsLspOCUnsupported(dut) {
				cfgplugins.RemoveMPLSStaticLSP(t, &gnmi.SetBatch{}, dut, cleanup.name, cleanup.label, cleanup.next, "ipv4", false)
			} else {
				gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Lsps().StaticLsp(cleanup.name).Config())
			}
		}
	}

	if config.staticRoute {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static("198.18.2.0/24").Config())
	}
	for _, aggregate := range []struct {
		id    string
		ports []string
	}{
		{config.custAggID, custPorts},
		{config.coreAggID, corePorts1},
		{config.core2AggID, corePorts2},
	} {
		if aggregate.id == "" {
			continue
		}
		for _, portID := range aggregate.ports {
			interfaceName := dut.Port(t, portID).Name()
			gnmi.Delete(t, dut, gnmi.OC().Interface(interfaceName).Ethernet().AggregateId().Config())
		}
		gnmi.Delete(t, dut, gnmi.OC().Interface(aggregate.id).Config())
	}
}

// macsecChainAnchor returns a future whole-minute DUT time for the shared PSK schedule.
func macsecChainAnchor(t *testing.T, dut *ondatra.DUTDevice) time.Time {
	t.Helper()
	nowStr := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	now, err := time.Parse(time.RFC3339, nowStr)
	if err != nil {
		t.Fatalf("failed to parse DUT current-datetime %q: %v", nowStr, err)
	}
	anchor := now.UTC().Add(macsecAnchorOffset).Truncate(time.Minute)
	if anchor.Sub(now.UTC()) < 90*time.Second {
		anchor = anchor.Add(time.Minute)
	}
	t.Logf("DUT clock %s; anchoring MACsec PSK chain at %s UTC", now.UTC().Format(time.RFC3339), anchor.Format(time.RFC3339))
	return anchor
}

// configureInterfaceAddress configures the IPv4/IPv6 addresses on a subinterface.
func configureInterfaceAddress(dut *ondatra.DUTDevice, s *oc.Interface_Subinterface, a *attrs.Attributes) {
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	if a.IPv4 != "" {
		s4.GetOrCreateAddress(a.IPv4).PrefixLength = ygot.Uint8(a.IPv4Len)
	}
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	if a.IPv6 != "" {
		s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(a.IPv6Len)
	}
}

// configDUTInterface builds the aggregate interface with its subinterfaces.
func configDUTInterface(i *oc.Interface, subinterfaces []*attrs.Attributes, dut *ondatra.DUTDevice) {
	if len(subinterfaces) > 0 {
		i.Description = ygot.String(subinterfaces[0].Desc)
	}
	for _, a := range subinterfaces {
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		s0 := i.GetOrCreateSubinterface(baseSubinterfaceIndex)
		b4 := s0.GetOrCreateIpv4()
		b6 := s0.GetOrCreateIpv6()
		b4.Mtu = ygot.Uint16(a.MTU)
		b6.Mtu = ygot.Uint32(uint32(a.MTU))
		if deviations.InterfaceEnabled(dut) {
			b4.Enabled = ygot.Bool(true)
		}
		if a.Subinterface != 0 {
			s := i.GetOrCreateSubinterface(a.Subinterface)
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(uint16(a.Subinterface))
			configureInterfaceAddress(dut, s, a)
		} else {
			configureInterfaceAddress(dut, s0, a)
		}
	}
}

// configureInterfaces configures the aggregate (LAG) and its member ports.
func configureInterfaces(t *testing.T, dut *ondatra.DUTDevice, dutPorts []string, subinterfaces []*attrs.Attributes, aggID string) {
	t.Helper()
	d := gnmi.OC()
	dutAggPorts := []*ondatra.Port{}
	for _, port := range dutPorts {
		dutAggPorts = append(dutAggPorts, dut.Port(t, port))
	}
	if deviations.AggregateAtomicUpdate(dut) {
		cfgplugins.DeleteAggregate(t, dut, aggID, dutAggPorts)
	}
	gnmi.Delete(t, dut, d.Lacp().Interface(aggID).Config())
	root := &oc.Root{}
	lag := root.GetOrCreateInterface(aggID)
	lag.Name = ygot.String(aggID)
	lag.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	lag.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	for _, port := range dutAggPorts {
		member := root.GetOrCreateInterface(port.Name())
		member.Name = ygot.String(port.Name())
		member.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		member.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		if deviations.InterfaceEnabled(dut) {
			member.Enabled = ygot.Bool(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), root)

	agg := &oc.Interface{Name: ygot.String(aggID)}
	configDUTInterface(agg, subinterfaces, dut)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	aggPath := d.Interface(aggID)
	gnmi.Replace(t, dut, aggPath.Config(), agg)
}

func waitForCondition[T any](t testing.TB, dev gnmi.DeviceOrOpts, query ygnmi.SingletonQuery[T], timeout time.Duration, condition func(*ygnmi.Value[T]) bool) bool {
	if condition == nil {
		return false
	}
	_, ok := gnmi.Watch(t, dev, query, timeout, condition).Await(t)
	return ok
}

func macsecKeyRotationConfig(anchor time.Time) cfgplugins.MACsecKeyRotationConfig {
	keys := make([]cfgplugins.MACsecRotationKey, len(macsecCKNs))
	for keyIndex := range macsecCKNs {
		keys[keyIndex] = cfgplugins.MACsecRotationKey{CKN: macsecCKNs[keyIndex], CAK: macsecCAKs[keyIndex]}
	}
	return cfgplugins.MACsecKeyRotationConfig{
		Keys:                keys,
		Anchor:              anchor,
		KeyLifetime:         macsecKeyLifetime,
		KeyStagger:          macsecKeyStagger,
		KeyServerPriority:   macsecKeyServerPriority,
		SAKRekeyInterval:    macsecSAKRekeyInterval,
		ReplayProtection:    macsecReplayProtection,
		IncludeICVIndicator: true,
		IncludeSCI:          true,
		Profiles: []cfgplugins.MACsecRotationProfile{
			{ProfileName: mustSecureProfile, SharedSecretProfile: mustSecureSSProfile},
			{ProfileName: shouldSecureProfile, SharedSecretProfile: shouldSecureSSProfile, AllowUnprotected: true},
		},
	}
}

// configureStaticRoute configures an ECMP route to the tunnel destination via
// both core next-hops.
func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          "198.18.2.0/24",
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(ateCoreV4),
			"1": oc.UnionString(ateCore2V4),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
}

func configureMPLSEncap(t *testing.T, dut *ondatra.DUTDevice, custAggID, nextHopGroupName, trafficPolicyName string, subinterface, label uint32, ttl uint8, tunnelSource, tunnelDestination string, udpDestinationPort uint16) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	batch = cfgplugins.MPLSEncapConfig(t, dut, batch, cfgplugins.MPLSEncapConfigParams{
		NextHopGroupName:   nextHopGroupName,
		TrafficPolicyName:  trafficPolicyName,
		InterfaceName:      custAggID,
		Subinterface:       subinterface,
		MPLSLabel:          label,
		TunnelSource:       tunnelSource,
		TunnelDestination:  tunnelDestination,
		TTL:                ttl,
		UDPDestinationPort: udpDestinationPort,
	})
	if batch != nil {
		batch.Set(t, dut)
	}
}

func configureMPLSoGREEncap(t *testing.T, dut *ondatra.DUTDevice, custAggID string) {
	configureMPLSEncap(t, dut, custAggID, mplsGreNHGName, mplsGreTrafficPolicy,
		custVlan, mplsGreLabel, mplsGreOuterTTL, mplsGreTunnelSrc, mplsGreTunnelDst, 0)
}

func configureMPLSoGREDecap(t *testing.T, dut *ondatra.DUTDevice) {
	configureMPLSDecap(t, dut, decapGroupName, decapTunnelIP, decapLabel, ateCustV4, false)
}

func configureMPLSoGUEEncap(t *testing.T, dut *ondatra.DUTDevice, custAggID string) {
	configureMPLSEncap(t, dut, custAggID, mplsGueNHGName, mplsGueTrafficPolicy,
		custVlanGUE, mplsGueLabel, mplsGueOuterTTL, mplsGreTunnelSrc, mplsGueTunnelDst, uint16(mplsGueUDPDstPort))
}

func configureMPLSoGUEDecap(t *testing.T, dut *ondatra.DUTDevice) {
	configureMPLSDecap(t, dut, gueDecapGroupName, gueDecapTunnelIP, gueDecapLabel, ateCustGUEV4, true)
}

func configureMPLSDecap(t *testing.T, dut *ondatra.DUTDevice, policyName, tunnelIP string, label uint32, nextHop string, gue bool) {
	t.Helper()
	networkInstanceName := deviations.DefaultNetworkInstance(dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(networkInstanceName)
	ni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	params := cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: networkInstanceName,
		AppliedPolicyName:   policyName,
		InnerDstIPv4:        nextHop + "/32",
		TunnelIP:            tunnelIP,
		Dynamic:             true,
		GUEPort:             uint32(mplsGueUDPDstPort),
		DecapProtocol:       "mpls",
		HasMPLS:             true,
		MPLSQoSMapping:      true,
	}
	if gue {
		cfgplugins.DecapGroupConfigGue(t, dut, pf, params)
	} else {
		cfgplugins.DecapGroupConfigGre(t, dut, pf, params)
	}
	lspBatch := &gnmi.SetBatch{}
	cfgplugins.MPLSStaticLSP(t, lspBatch, dut, policyName+"-lsp", label, nextHop, "", "ipv4")
	if !deviations.StaticMplsLspOCUnsupported(dut) {
		lspBatch.Set(t, dut)
	}
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		cfgplugins.PushPolicyForwardingConfig(t, dut, ni)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, anchor time.Time, testConfig *dutTestConfig) string {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if dut.Vendor() == ondatra.ARISTA {
		// Configure hardware TCAM profile on the DUT to allocate hardware resources for
		// policy forwarding, routing redirect actions, and IPv6 tunnel encapsulation.
		profile := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureNGPR)
		cfgplugins.PushDUTHardwareInitConfig(t, dut, profile)
	}

	interfaceName := dut.Port(t, custPorts[0]).Name()
	if !waitForCondition(t, dut, gnmi.OC().Interface(interfaceName).OperStatus().State(), 5*time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, ok := val.Val()
		return ok && status == oc.Interface_OperStatus_UP
	}) {
		t.Fatalf("DUT interface %s did not become operationally UP within 5m", interfaceName)
	}

	// Customer aggregate (MACsec edge) with one dual-stack subinterface per
	// enabled encapsulation path.
	custSubs := []*attrs.Attributes{}

	custSubs = append(custSubs, &custIntf)

	custSubs = append(custSubs, &custIntfGUE)

	custAggID := netutil.NextAggregateInterface(t, dut)
	testConfig.custAggID = custAggID
	configureInterfaces(t, dut, custPorts, custSubs, custAggID)

	// Enable MACsec on both customer aggregate member ports.
	// Configure the MACsec 5-PSK rotation chain (PF-1.17.1) on both customer
	// aggregate member ports and bind the must-secure policy by default.

	macsecBatch := &gnmi.SetBatch{}
	testConfig.macsec = true
	cfgplugins.ConfigureMACsecKeyRotation(t, dut, macsecBatch, macsecKeyRotationConfig(anchor))
	cfgplugins.ConfigureMACsecProfileBinding(t, dut, macsecBatch, cfgplugins.MACsecProfileBindingConfig{
		PortIDs:          custPorts,
		ProfileName:      mustSecureProfile,
		KeyChainName:     mustSecureSSProfile,
		ReplayProtection: macsecReplayProtection,
	})
	macsecBatch.Set(t, dut)

	// Two core aggregates (MPLSoGRE/MPLSoGUE egress), ECMP for the tunnel traffic.
	core1AggID := netutil.NextAggregateInterface(t, dut)
	testConfig.coreAggID = core1AggID
	configureInterfaces(t, dut, corePorts1, []*attrs.Attributes{&coreIntf}, core1AggID)
	core2AggID := netutil.NextAggregateInterface(t, dut)
	testConfig.core2AggID = core2AggID
	configureInterfaces(t, dut, corePorts2, []*attrs.Attributes{&coreIntf2}, core2AggID)

	testConfig.staticRoute = true
	configureStaticRoute(t, dut)

	cfgplugins.MplsConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)

	testConfig.greEncap = true
	configureMPLSoGREEncap(t, dut, custAggID)

	testConfig.gueEncap = true
	configureMPLSoGUEEncap(t, dut, custAggID)

	testConfig.greDecap = true
	configureMPLSoGREDecap(t, dut)

	testConfig.gueDecap = true
	configureMPLSoGUEDecap(t, dut)

	return custAggID
}

// corruptCAK returns a CAK guaranteed not to match the given one by flipping its
// last hex nibble, used to force an MKA key mismatch.
func corruptCAK(cak string) string {
	if cak == "" {
		return "0"
	}
	last := cak[len(cak)-1]
	repl := byte('0')
	if last == '0' {
		repl = '1'
	}
	return cak[:len(cak)-1] + string(repl)
}

// addEncapFlows adds the IPv4 and IPv6 customer -> core encap flows for one
// customer subinterface (MACsec header included when enabled).
func addEncapFlows(top gosnappi.Config, f encapFlowSet) {
	flowV4 := top.Flows().Add().SetName(f.v4Name)
	flowV4.TxRx().Device().SetTxNames([]string{f.txV4}).SetRxNames(f.rxV4Names)
	for _, sw := range sizeWeightProfile {
		flowV4.Size().WeightPairs().Custom().Add().SetSize(sw.Size).SetWeight(sw.Weight)
	}
	ratePercent := f.ratePercent
	if ratePercent == 0 {
		ratePercent = forwardTrafficPercent
	}
	flowV4.Rate().SetPercentage(ratePercent)
	flowV4.Duration().Continuous()
	flowV4.Metrics().SetEnable(true)
	eth4 := flowV4.Packet().Add().Ethernet()
	eth4.Src().SetValue(f.srcMAC)
	if f.dstMAC != "" {
		eth4.Dst().SetValue(f.dstMAC)
	}
	if f.includeMACsec {
		flowV4.Packet().Add().Macsec()
	}
	flowV4.Packet().Add().Vlan().Id().SetValue(f.vlan)
	v4 := flowV4.Packet().Add().Ipv4()
	v4.Src().Increment().SetStart(f.srcV4).SetStep("0.0.0.1").SetCount(100)
	v4.Dst().SetValue(f.dstV4)

	flowV6 := top.Flows().Add().SetName(f.v6Name)
	flowV6.TxRx().Device().SetTxNames([]string{f.txV6}).SetRxNames(f.rxV6Names)
	for _, sw := range sizeWeightProfile {
		flowV6.Size().WeightPairs().Custom().Add().SetSize(sw.Size).SetWeight(sw.Weight)
	}
	flowV6.Rate().SetPercentage(ratePercent)
	flowV6.Duration().Continuous()
	flowV6.Metrics().SetEnable(true)
	eth6 := flowV6.Packet().Add().Ethernet()
	eth6.Src().SetValue(f.srcMAC)
	if f.dstMAC != "" {
		eth6.Dst().SetValue(f.dstMAC)
	}
	if f.includeMACsec {
		flowV6.Packet().Add().Macsec()
	}
	flowV6.Packet().Add().Vlan().Id().SetValue(f.vlan)
	v6 := flowV6.Packet().Add().Ipv6()
	v6.Src().Increment().SetStart(f.srcV6).SetStep("::1").SetCount(100)
	v6.Dst().SetValue(f.dstV6)
}

func addGREDecapFlow(top gosnappi.Config, flowName, txDevice, rxDevice, srcMAC, dstMAC, outerSrc, outerDst string, label uint32, innerSrc, innerDst string) {
	flow := top.Flows().Add().SetName(flowName)
	flow.TxRx().Device().SetTxNames([]string{txDevice}).SetRxNames([]string{rxDevice})
	for _, sizeWeight := range sizeWeightProfile {
		flow.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}
	flow.Rate().SetPercentage(reverseTrafficPercent)
	flow.Duration().Continuous()
	flow.Metrics().SetEnable(true)
	ethernet := flow.Packet().Add().Ethernet()
	ethernet.Src().SetValue(srcMAC)
	if dstMAC != "" {
		ethernet.Dst().SetValue(dstMAC)
	}
	outer := flow.Packet().Add().Ipv4()
	outer.Src().SetValue(outerSrc)
	outer.Dst().SetValue(outerDst)
	flow.Packet().Add().Gre().Protocol().SetValue(mplsEthertype)
	flow.Packet().Add().Mpls().Label().SetValue(label)
	inner := flow.Packet().Add().Ipv4()
	inner.Src().Increment().SetStart(innerSrc).SetStep("0.0.0.1").SetCount(100)
	inner.Dst().SetValue(innerDst)
}

func addGUEDecapFlow(top gosnappi.Config, flowName, txDevice, rxDevice, srcMAC, dstMAC string) {
	flow := top.Flows().Add().SetName(flowName)
	flow.TxRx().Device().SetTxNames([]string{txDevice}).SetRxNames([]string{rxDevice})
	for _, sizeWeight := range sizeWeightProfile {
		flow.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}
	flow.Rate().SetPercentage(reverseTrafficPercent)
	flow.Duration().Continuous()
	flow.Metrics().SetEnable(true)
	ethernet := flow.Packet().Add().Ethernet()
	ethernet.Src().SetValue(srcMAC)
	if dstMAC != "" {
		ethernet.Dst().SetValue(dstMAC)
	}
	outer := flow.Packet().Add().Ipv4()
	outer.Src().SetValue(gueDecapOuterSrc)
	outer.Dst().SetValue(gueDecapOuterDst)
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValue(49152)
	udp.DstPort().SetValue(mplsGueUDPDstPort)
	flow.Packet().Add().Mpls().Label().SetValue(gueDecapLabel)
	inner := flow.Packet().Add().Ipv4()
	inner.Src().Increment().SetStart(gueDecapInnerSrc).SetStep("0.0.0.1").SetCount(100)
	inner.Dst().SetValue(gueDecapInnerDst)
}

// configureATE builds the OTG topology: a MACsec-enabled customer LAG and a core LAG, plus encap flows.
func configureATE(t *testing.T, opts ateOptions) gosnappi.Config {
	t.Helper()
	anchor := opts.anchor
	top := gosnappi.NewConfig()

	// Customer LAG with MACsec + MKA on every member port (2 members).
	custLag := top.Lags().Add().SetName(ateCustLagName)
	custLag.Protocol().Static().SetLagId(1)
	for i, portName := range custPorts {
		p := top.Ports().Add().SetName(portName)
		lagPort := custLag.Ports().Add().SetPortName(p.Name())
		lagPort.Ethernet().SetName(fmt.Sprintf("lag1Eth%d", i+1)).SetMac(ateCustLagPMACs[i]).SetMtu(uint32(custIntf.MTU))

		macsec := lagPort.Macsec()
		secy := macsec.SecureEntity().SetName(macsecPeerNames[i])
		secy.DataPlane().Encapsulation().CryptoEngine().EncryptDecrypt().HardwareAcceleration().InlineCrypto()
		mka := secy.KeyGenerationProtocol().Mka().SetName(fmt.Sprintf("Peer%d-Mka", i+1))
		basic := mka.Basic()
		basic.KeySource().Psk()
		basic.SetActorPriority("00")
		basic.SetKeyDerivationFunction(gosnappi.MkaBasicKeyDerivationFunction.AES_CMAC_256)
		basic.SetSendIcvIndicatiorInMkpdu(true)
		basic.SetMkaVersion(2)
		mka.KeyServer().SetCipherSuite(gosnappi.MkaKeyServerCipherSuite.GCM_AES_XPN_256)
		scs := basic.SupportedCipherSuites()
		scs.SetGcmAes128(false)
		scs.SetGcmAes256(false)
		scs.SetGcmAesXpn128(false)
		scs.SetGcmAesXpn256(true)
		// Anchor the whole PSK chain to the shared future start time so the DUT and
		// OTG evaluate identical absolute key windows.
		utc := basic.PskChainStartTime().Utc()
		utc.SetYear(uint32(anchor.Year())).SetMonth(uint32(anchor.Month())).SetDay(uint32(anchor.Day()))
		utc.SetHour(uint32(anchor.Hour())).SetMinute(uint32(anchor.Minute())).SetSecond(uint32(anchor.Second()))
		// Offsets are HH:MM relative to the chain anchor.
		for k := 0; k < macsecNumPSKs; k++ {
			psk := basic.KeySource().Psks().Add()
			psk.SetCakName(macsecCKNs[k])
			cakValue := macsecCAKs[k]
			if opts.corruptKeys {
				cakValue = corruptCAK(cakValue)
			}
			psk.SetCakValue(cakValue)
			startOffset := time.Duration(k) * macsecKeyStagger
			endOffset := startOffset + macsecKeyLifetime + opts.initialKeyExtension
			if k > 0 {
				startOffset += opts.initialKeyExtension
			}
			psk.StartOffsetTime().SetHh(uint32(startOffset / time.Hour)).SetMm(uint32(startOffset / time.Minute % 60))
			psk.EndOffsetTime().SetHh(uint32(endOffset / time.Hour)).SetMm(uint32(endOffset / time.Minute % 60))
		}
		mka.Tx().SecureChannels().Add().SetName(fmt.Sprintf("SecureChannel%d", i+1)).SetSystemId(lagPort.Ethernet().Mac())
	}

	// Core LAG 1 (corePorts1) and its dual-stack device.
	d2 := top.Devices().Add().SetName(ateCoreDev)
	coreLag := top.Lags().Add().SetName(ateCoreLagName)
	coreLag.Protocol().Static().SetLagId(2)
	for i, portName := range corePorts1 {
		p := top.Ports().Add().SetName(portName)
		lagPort := coreLag.Ports().Add().SetPortName(p.Name())
		lagPort.Ethernet().SetName(fmt.Sprintf("lag2Eth%d", i+1)).SetMac(ateCoreLagPMACs[i]).SetMtu(uint32(coreIntf.MTU))
	}

	d2Eth := d2.Ethernets().Add().SetName("d2Eth").SetMac(ateCoreMAC)
	d2Eth.Connection().SetLagName(coreLag.Name())
	d2v4 := d2Eth.Ipv4Addresses().Add().SetName("p2d2ipv4").SetAddress(ateCoreV4).SetGateway(ateCoreV4Gw).SetPrefix(uint32(coreIntf.IPv4Len))
	d2v6 := d2Eth.Ipv6Addresses().Add().SetName("p2d2ipv6").SetAddress(ateCoreV6).SetGateway(ateCoreV6Gw).SetPrefix(uint32(coreIntf.IPv6Len))

	// Core LAG 2 (corePorts2) and its dual-stack device (ECMP peer for the tunnel).
	d4 := top.Devices().Add().SetName(ateCore2Dev)
	coreLag2 := top.Lags().Add().SetName(ateCoreLag2Name)
	coreLag2.Protocol().Static().SetLagId(3)
	for i, portName := range corePorts2 {
		p := top.Ports().Add().SetName(portName)
		lagPort := coreLag2.Ports().Add().SetPortName(p.Name())
		lagPort.Ethernet().SetName(fmt.Sprintf("lag3Eth%d", i+1)).SetMac(ateCore2LagPMACs[i]).SetMtu(uint32(coreIntf2.MTU))
	}

	d4Eth := d4.Ethernets().Add().SetName("d4Eth").SetMac(ateCore2MAC)
	d4Eth.Connection().SetLagName(coreLag2.Name())
	d4v4 := d4Eth.Ipv4Addresses().Add().SetName("p2d4ipv4").SetAddress(ateCore2V4).SetGateway(ateCore2V4Gw).SetPrefix(uint32(coreIntf2.IPv4Len))
	d4v6 := d4Eth.Ipv6Addresses().Add().SetName("p2d4ipv6").SetAddress(ateCore2V6).SetGateway(ateCore2V6Gw).SetPrefix(uint32(coreIntf2.IPv6Len))
	if opts.dutCore2MAC != "" {
		d4v4.GatewayMac().SetValue(opts.dutCore2MAC)
		d4v6.GatewayMac().SetValue(opts.dutCore2MAC)
	}

	{
		forwardRate := forwardTrafficPercent
		if opts.shouldSecureProbe {
			forwardRate = shouldSecureFlowPercent
		}
		// Customer dual-stack device (VLAN custVlan) on the MACsec LAG.
		d1 := top.Devices().Add().SetName(ateCustDev)
		d1Eth := d1.Ethernets().Add().SetName("d1Eth").SetMac(ateCustMAC)
		d1Eth.Connection().SetLagName(custLag.Name())
		d1Eth.Vlans().Add().SetName("d1Vlan").SetId(custVlan)
		d1v4ip := d1Eth.Ipv4Addresses().Add().SetName("p1d1ipv4").SetAddress(ateCustV4).SetGateway(ateCustV4Gw).SetPrefix(uint32(custIntf.IPv4Len))
		d1v6ip := d1Eth.Ipv6Addresses().Add().SetName("p1d1ipv6").SetAddress(ateCustV6).SetGateway(ateCustV6Gw).SetPrefix(uint32(custIntf.IPv6Len))
		if opts.dutCustMAC != "" {
			d1v4ip.GatewayMac().SetValue(opts.dutCustMAC)
			d1v6ip.GatewayMac().SetValue(opts.dutCustMAC)
		}

		if opts.shouldSecureProbe {
			if !opts.probeOnly {
				addEncapFlows(top, encapFlowSet{
					v4Name: flowIPv4Fwd, v6Name: flowIPv6Fwd,
					txV4: d1v4ip.Name(), txV6: d1v6ip.Name(),
					rxV4Names: []string{d2v4.Name(), d4v4.Name()}, rxV6Names: []string{d2v6.Name(), d4v6.Name()},
					srcMAC: ateCustMAC, dstMAC: opts.dutCustMAC, vlan: custVlan,
					srcV4: ateCustV4, dstV4: innerV4Dst, srcV6: ateCustV6, dstV6: innerV6Dst,
					ratePercent:   forwardRate,
					includeMACsec: true,
				})
			}
			// The probe duplicates the secured device's identity but sends without
			// a MACsec packet layer after the should-secure key chain expires.
			clearDev := top.Devices().Add().SetName(ateCustClearDev)
			clearEth := clearDev.Ethernets().Add().SetName("d5Eth").SetMac(ateCustMAC)
			clearEth.Connection().SetLagName(custLag.Name())
			clearEth.Vlans().Add().SetName("d5Vlan").SetId(custVlan)
			clearV4 := clearEth.Ipv4Addresses().Add().SetName("p1d5ipv4").SetAddress(ateCustV4).SetGateway(ateCustV4Gw).SetPrefix(uint32(custIntf.IPv4Len))
			clearV6 := clearEth.Ipv6Addresses().Add().SetName("p1d5ipv6").SetAddress(ateCustV6).SetGateway(ateCustV6Gw).SetPrefix(uint32(custIntf.IPv6Len))
			if opts.dutCustMAC != "" {
				clearV4.GatewayMac().SetValue(opts.dutCustMAC)
				clearV6.GatewayMac().SetValue(opts.dutCustMAC)
			}
			addEncapFlows(top, encapFlowSet{
				v4Name: flowShouldSecureIPv4, v6Name: flowShouldSecureIPv6,
				txV4: clearV4.Name(), txV6: clearV6.Name(),
				rxV4Names: []string{d2v4.Name(), d4v4.Name()}, rxV6Names: []string{d2v6.Name(), d4v6.Name()},
				srcMAC: ateCustMAC, dstMAC: opts.dutCustMAC, vlan: custVlan,
				srcV4: ateCustV4, dstV4: innerV4Dst, srcV6: ateCustV6, dstV6: innerV6Dst,
				ratePercent:   shouldSecureFlowPercent,
				includeMACsec: false,
			})
		} else if !opts.probeOnly {
			addEncapFlows(top, encapFlowSet{
				v4Name: flowIPv4Fwd, v6Name: flowIPv6Fwd,
				txV4: d1v4ip.Name(), txV6: d1v6ip.Name(),
				rxV4Names: []string{d2v4.Name(), d4v4.Name()}, rxV6Names: []string{d2v6.Name(), d4v6.Name()},
				srcMAC: ateCustMAC, dstMAC: opts.dutCustMAC, vlan: custVlan,
				srcV4: ateCustV4, dstV4: innerV4Dst, srcV6: ateCustV6, dstV6: innerV6Dst,
				includeMACsec: true,
				ratePercent:   forwardRate,
			})
		}
		if !opts.probeOnly {
			addGREDecapFlow(top, flowIPv4Decap, d2v4.Name(), d1v4ip.Name(), ateCoreMAC, opts.dutCoreMAC, decapOuterSrc, decapOuterDst, decapLabel, decapInnerSrc, decapInnerDst)
			addGREDecapFlow(top, flowIPv4DecapCore2, d4v4.Name(), d1v4ip.Name(), ateCore2MAC, opts.dutCore2MAC, decapOuterSrc, decapOuterDst, decapLabel, decapInnerSrc, decapInnerDst)
		}
	}

	{
		forwardRate := forwardTrafficPercent
		if opts.shouldSecureProbe {
			forwardRate = shouldSecureFlowPercent
		}
		// Second customer dual-stack device (VLAN custVlanGUE) on the same MACsec LAG.
		d3 := top.Devices().Add().SetName(ateCustGUEDev)
		d3Eth := d3.Ethernets().Add().SetName("d3Eth").SetMac(ateCustGUEMAC)
		d3Eth.Connection().SetLagName(custLag.Name())
		d3Eth.Vlans().Add().SetName("d3Vlan").SetId(custVlanGUE)
		d3v4ip := d3Eth.Ipv4Addresses().Add().SetName("p1d3ipv4").SetAddress(ateCustGUEV4).SetGateway(ateCustGUEV4Gw).SetPrefix(uint32(custIntfGUE.IPv4Len))
		d3v6ip := d3Eth.Ipv6Addresses().Add().SetName("p1d3ipv6").SetAddress(ateCustGUEV6).SetGateway(ateCustGUEV6Gw).SetPrefix(uint32(custIntfGUE.IPv6Len))

		if !opts.probeOnly {
			addEncapFlows(top, encapFlowSet{
				v4Name: flowGUEIPv4Fwd, v6Name: flowGUEIPv6Fwd,
				txV4: d3v4ip.Name(), txV6: d3v6ip.Name(),
				rxV4Names: []string{d2v4.Name(), d4v4.Name()}, rxV6Names: []string{d2v6.Name(), d4v6.Name()},
				srcMAC: ateCustGUEMAC, dstMAC: opts.dutCustMAC, vlan: custVlanGUE,
				srcV4: ateCustGUEV4, dstV4: innerV4Dst, srcV6: ateCustGUEV6, dstV6: innerV6Dst,
				ratePercent:   forwardRate,
				includeMACsec: true,
			})
		}
		if opts.shouldSecureProbe {
			clearGUEDev := top.Devices().Add().SetName(ateCustGUEClearDev)
			clearGUEEth := clearGUEDev.Ethernets().Add().SetName("d6Eth").SetMac(ateCustGUEMAC)
			clearGUEEth.Connection().SetLagName(custLag.Name())
			clearGUEEth.Vlans().Add().SetName("d6Vlan").SetId(custVlanGUE)
			clearGUEv4 := clearGUEEth.Ipv4Addresses().Add().SetName("p1d6ipv4").SetAddress(ateCustGUEV4).SetGateway(ateCustGUEV4Gw).SetPrefix(uint32(custIntfGUE.IPv4Len))
			clearGUEv6 := clearGUEEth.Ipv6Addresses().Add().SetName("p1d6ipv6").SetAddress(ateCustGUEV6).SetGateway(ateCustGUEV6Gw).SetPrefix(uint32(custIntfGUE.IPv6Len))
			if opts.dutCustMAC != "" {
				clearGUEv4.GatewayMac().SetValue(opts.dutCustMAC)
				clearGUEv6.GatewayMac().SetValue(opts.dutCustMAC)
			}
			addEncapFlows(top, encapFlowSet{
				v4Name: flowShouldSecureGUEIPv4, v6Name: flowShouldSecureGUEIPv6,
				txV4: clearGUEv4.Name(), txV6: clearGUEv6.Name(),
				rxV4Names: []string{d2v4.Name(), d4v4.Name()}, rxV6Names: []string{d2v6.Name(), d4v6.Name()},
				srcMAC: ateCustGUEMAC, dstMAC: opts.dutCustMAC, vlan: custVlanGUE,
				srcV4: ateCustGUEV4, dstV4: innerV4Dst, srcV6: ateCustGUEV6, dstV6: innerV6Dst,
				ratePercent:   shouldSecureFlowPercent,
				includeMACsec: false,
			})
		}
		if !opts.probeOnly {
			addGUEDecapFlow(top, flowGUEIPv4Decap, d2v4.Name(), d3v4ip.Name(), ateCoreMAC, opts.dutCoreMAC)
			addGUEDecapFlow(top, flowGUEIPv4DecapCore2, d4v4.Name(), d3v4ip.Name(), ateCore2MAC, opts.dutCore2MAC)
		}
	}

	return top
}

func encapPacketValidations() []*packetvalidationhelpers.PacketValidation {
	var validations []*packetvalidationhelpers.PacketValidation
	for _, portName := range append(append([]string{}, corePorts1...), corePorts2...) {
		validations = append(validations, &packetvalidationhelpers.PacketValidation{
			PortName:           portName,
			CaptureName:        "encap-" + portName,
			IPv4Layer:          &packetvalidationhelpers.IPv4Layer{Protocol: packetvalidationhelpers.GRE, DstIP: mplsGreTunnelDst, TTL: mplsGreOuterTTL},
			MatchEncapsulation: true,
			MPLSLayer:          &packetvalidationhelpers.MPLSLayer{Label: mplsGreLabel, Tc: 0},
			Validations:        encapValidations,
		})
	}
	return validations
}

func decapPacketValidations() []*packetvalidationhelpers.PacketValidation {
	var validations []*packetvalidationhelpers.PacketValidation
	for _, portName := range custPorts {
		validations = append(validations, &packetvalidationhelpers.PacketValidation{
			PortName: portName, CaptureName: "decap-capture-" + portName,
			MacsecLayer: &packetvalidationhelpers.MacsecLayer{}, Validations: decapValidations,
		})
	}
	return validations
}

// buildATE builds the ATE topology for the given options and configures packet
// capture on the encap/decap ports.
func buildATE(t *testing.T, opts ateOptions) gosnappi.Config {
	t.Helper()
	top := configureATE(t, opts)
	for _, flow := range top.Flows().Items() {
		flow.Rate().SetPps(trafficPPS)
	}

	for _, pv := range encapPacketValidations() {
		packetvalidationhelpers.ConfigurePacketCapture(t, top, pv)
	}

	for _, dv := range decapPacketValidations() {
		packetvalidationhelpers.ConfigurePacketCapture(t, top, dv)
	}
	return top
}

// pushAndStartOTG pushes the OTG config and starts protocols without waiting for
// sessions to come up.
func pushAndStartOTG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	otg := ate.OTG()
	t.Logf("Pushing OTG configuration")
	otg.PushConfig(t, top)
	t.Logf("Starting OTG protocols")
	otg.StartProtocols(t)
	t.Logf("OTG protocols started successfully")
	coreConfig := gosnappi.NewConfig()
	for _, device := range top.Devices().Items() {
		if device.Name() == ateCoreDev || device.Name() == ateCore2Dev {
			coreConfig.Devices().Append(device)
		}
	}
	otgutils.WaitForARP(t, otg, coreConfig, "IPv4")
	otgutils.WaitForARP(t, otg, coreConfig, "IPv6")
}

// waitForOTGMACSecUp waits for the OTG MACsec session to become UP.
func waitForOTGMACSecUp(t *testing.T, ate *ondatra.ATEDevice, ifName string, timeout time.Duration) {
	waitForOTGMACSecState(t, ate, ifName, otgtelemetry.Interface_SessionState_UP, timeout)
}

func waitForOTGMACSecState(t *testing.T, ate *ondatra.ATEDevice, ifName string, state otgtelemetry.E_Interface_SessionState, timeout time.Duration) {
	t.Helper()
	otg := ate.OTG()
	t.Logf("Waiting for OTG MACsec session on %s to be %v", ifName, state)
	if !waitForCondition(t, otg, gnmi.OTG().Macsec().Interface(ifName).SessionState().State(), timeout, func(val *ygnmi.Value[otgtelemetry.E_Interface_SessionState]) bool {
		got, ok := val.Val()
		return ok && got == state
	}) {
		finalState := gnmi.Get(t, otg, gnmi.OTG().Macsec().Interface(ifName).SessionState().State())
		t.Fatalf("MACsec session on %s did not become %v within %v, final state=%v", ifName, state, timeout, finalState)
	}
}

// waitForOTGLAGUp waits for the named OTG LAG to become operationally UP.
func waitForOTGLAGUp(t *testing.T, ate *ondatra.ATEDevice, lagName string, wantMembersUp uint64, timeout time.Duration) {
	t.Helper()
	otg := ate.OTG()
	t.Logf("Waiting for OTG LAG %s to be UP with %d member(s)", lagName, wantMembersUp)
	if !waitForCondition(t, otg, gnmi.OTG().Lag(lagName).State(), timeout, func(val *ygnmi.Value[*otgtelemetry.Lag]) bool {
		lag, ok := val.Val()
		return ok && lag != nil && lag.GetOperStatus() == otgtelemetry.Lag_OperStatus_UP && lag.GetCounters().GetMemberPortsUp() == wantMembersUp
	}) {
		finalOper := gnmi.Get(t, otg, gnmi.OTG().Lag(lagName).OperStatus().State())
		finalMembersUp := gnmi.Get(t, otg, gnmi.OTG().Lag(lagName).Counters().MemberPortsUp().State())
		t.Fatalf("OTG LAG %s did not become operationally UP within %v: oper-status=%v member-ports-up=%d (want %d)", lagName, timeout, finalOper, finalMembersUp, wantMembersUp)
	}
}

// bringUpOTG pushes the OTG config, starts protocols, and waits for the
// MACsec/MKA sessions, both LAGs, and ARP/ND to come up.
func bringUpOTG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, skipCustomerARP bool) {
	t.Helper()
	pushAndStartOTG(t, ate, top)

	// One MACsec secure entity per customer member port.
	for i := range custPorts {
		waitForOTGMACSecUp(t, ate, macsecPeerNames[i], macsecUpTimeout)
	}

	waitForOTGLAGUp(t, ate, ateCustLagName, uint64(len(custPorts)), sessionUpTimeout)
	waitForOTGLAGUp(t, ate, ateCoreLagName, uint64(len(corePorts1)), sessionUpTimeout)
	waitForOTGLAGUp(t, ate, ateCoreLag2Name, uint64(len(corePorts2)), sessionUpTimeout)
	if !skipCustomerARP {
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
	}
}

// resolveDUTMAC waits for and returns the resolved link-layer MAC address for a gateway neighbor on an OTG interface.
func resolveDUTMAC(t *testing.T, ate *ondatra.ATEDevice, ifName, gwIP string) string {
	t.Helper()
	resolved, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Interface(ifName).Ipv4Neighbor(gwIP).LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		resolved, ok := val.Val()
		return ok && resolved != ""
	}).Await(t)
	if !ok {
		t.Fatalf("Failed to resolve MAC for %s neighbor %s within 1m", ifName, gwIP)
		return ""
	}
	mac, _ := resolved.Val()
	return mac
}

// reanchorMACsec reconfigures the DUT and ATE MACsec chain with a fresh anchor a
// few minutes ahead so the entire rotation window is in the future, then brings
// the session up. It unbinds any active profile first to cleanly reset MKA, binds
// the requested profile, and updates args.anchor and args.top.
func reanchorMACsec(t *testing.T, args *testArgs, profile string) {
	t.Helper()
	reanchorMACsecWithExtension(t, args, profile, 0)
}

func reanchorMACsecWithExtension(t *testing.T, args *testArgs, profile string, initialKeyExtension time.Duration) {
	t.Helper()
	removeBatch := &gnmi.SetBatch{}
	cfgplugins.ConfigureMACsecProfileBinding(t, args.dut, removeBatch, cfgplugins.MACsecProfileBindingConfig{
		PortIDs: custPorts,
		Remove:  true,
	})
	removeBatch.Set(t, args.dut)
	newAnchor := macsecChainAnchor(t, args.dut)
	macsecBatch := &gnmi.SetBatch{}
	rotationConfig := macsecKeyRotationConfig(newAnchor)
	rotationConfig.InitialKeyExtension = initialKeyExtension
	cfgplugins.ConfigureMACsecKeyRotation(t, args.dut, macsecBatch, rotationConfig)
	cfgplugins.ConfigureMACsecProfileBinding(t, args.dut, macsecBatch, cfgplugins.MACsecProfileBindingConfig{
		PortIDs:          custPorts,
		ProfileName:      profile,
		KeyChainName:     macsecKeyChainForProfile(profile),
		ReplayProtection: macsecReplayProtection,
	})
	macsecBatch.Set(t, args.dut)
	args.anchor = newAnchor
	args.top = buildATE(t, ateOptions{
		anchor:              newAnchor,
		initialKeyExtension: initialKeyExtension,
		dutCustMAC:          args.dutCustMAC,
		dutCoreMAC:          args.dutCoreMAC,
		dutCore2MAC:         args.dutCore2MAC,
	})
	bringUpOTG(t, args.ate, args.top, args.shouldSecureProbe)
}

func readMACsecProfileConfig(t *testing.T, dut *ondatra.DUTDevice, profile string) string {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		t.Fatalf("native MACsec profile verification is unsupported for %v", dut.Vendor())
	}
	runningConfig := helpers.RunCliCommand(t, dut, "show running-config section mac security")
	profileStart := strings.Index(runningConfig, "profile "+profile)
	if profileStart < 0 {
		t.Fatalf("DUT running config does not contain MACsec profile %s", profile)
	}
	profileLines := strings.Split(runningConfig[profileStart:], "\n")
	for lineIndex := 1; lineIndex < len(profileLines); lineIndex++ {
		if strings.HasPrefix(strings.TrimSpace(profileLines[lineIndex]), "profile ") {
			profileLines = profileLines[:lineIndex]
			break
		}
	}
	return strings.Join(profileLines, "\n")
}

func verifyDUTMACsecState(t *testing.T, dut *ondatra.DUTDevice, portID, profile string) {
	t.Helper()
	if deviations.MacsecOcUnsupported(dut) {
		profileConfig := readMACsecProfileConfig(t, dut, profile)
		for _, required := range []string{
			"cipher aes256-gcm-xpn",
			"mka key-server priority 15",
			"mka session rekey-period 30",
			"replay protection window 64",
			"sci",
		} {
			if !strings.Contains(profileConfig, required) {
				t.Errorf("DUT MACsec profile %s running config is missing %q", profile, required)
			}
		}
		unprotected := "traffic unprotected drop"
		if profile == shouldSecureProfile {
			unprotected = "traffic unprotected allow"
		}
		if !strings.Contains(profileConfig, unprotected) {
			t.Errorf("DUT MACsec profile %s running config is missing %q", profile, unprotected)
		}
		return
	}
	interfaceName := dut.Port(t, portID).Name()
	state := gnmi.Get(t, dut, gnmi.OC().Macsec().Interface(interfaceName).State())
	if state.GetName() != interfaceName {
		t.Errorf("DUT MACsec interface state name: got %q, want %q", state.GetName(), interfaceName)
	}
	if !state.GetEnable() {
		t.Errorf("DUT MACsec interface %s is not enabled", interfaceName)
	}
	if got := state.GetReplayProtection(); got != macsecReplayProtection {
		t.Errorf("DUT MACsec interface %s replay-protection=%d, want %d", interfaceName, got, macsecReplayProtection)
	}
	if got := state.GetMka().GetMkaPolicy(); got != profile {
		t.Errorf("DUT MACsec interface %s MKA policy=%q, want %q", interfaceName, got, profile)
	}
	policy := gnmi.Get(t, dut, gnmi.OC().Macsec().Mka().Policy(profile).State())
	if got := policy.GetConfidentialityOffset(); got != oc.Macsec_ConfidentialityOffset_0_BYTES {
		t.Errorf("DUT MACsec policy %s confidentiality-offset=%v, want 0_BYTES", profile, got)
	}
	if !policy.GetIncludeIcvIndicator() || !policy.GetIncludeSci() {
		t.Errorf("DUT MACsec policy %s include-icv-indicator=%t include-sci=%t, want true/true", profile, policy.GetIncludeIcvIndicator(), policy.GetIncludeSci())
	}
}

func testGenerateConfig(t *testing.T, args *testArgs) {

	for i := range custPorts {
		state := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Macsec().Interface(macsecPeerNames[i]).SessionState().State())
		if state != otgtelemetry.Interface_SessionState_UP {
			t.Errorf("MACsec session on %s: got %v, want UP", macsecPeerNames[i], state)
		}
		verifyDUTMACsecState(t, args.dut, custPorts[i], mustSecureProfile)
	}
}

func enabledFlows() []string {
	var flows []string

	flows = append(flows, flowIPv4Fwd, flowIPv6Fwd, flowIPv4Decap, flowIPv4DecapCore2)

	flows = append(flows, flowGUEIPv4Fwd, flowGUEIPv6Fwd, flowGUEIPv4Decap, flowGUEIPv4DecapCore2)

	return flows
}

func readFlowCounters(t *testing.T, ate *ondatra.ATEDevice, flowNames []string) map[string]flowCounterSnapshot {
	t.Helper()
	counters := make(map[string]flowCounterSnapshot, len(flowNames))
	flows := make(map[string]*otgtelemetry.Flow)
	for _, flow := range gnmi.GetAll(t, ate.OTG(), gnmi.OTG().FlowAny().State()) {
		flows[flow.GetName()] = flow
	}
	for _, flowName := range flowNames {
		m, ok := flows[flowName]
		if !ok {
			t.Fatalf("no OTG counter telemetry for flow %s", flowName)
		}
		counters[flowName] = flowCounterSnapshot{
			tx: m.GetCounters().GetOutPkts(),
			rx: m.GetCounters().GetInPkts(),
		}
	}
	return counters
}

func readDUTEncryptedCounters(t *testing.T, dut *ondatra.DUTDevice, portIDs []string) map[string]uint64 {
	t.Helper()
	counters := make(map[string]uint64, len(portIDs))
	for _, portID := range portIDs {
		interfaceName := dut.Port(t, portID).Name()
		values := gnmi.GetAll(t, dut, gnmi.OC().Macsec().Interface(interfaceName).ScsaTxAny().Counters().ScEncrypted().State())
		for _, value := range values {
			counters[portID] += value
		}
	}
	return counters
}

func readOTGPortCounters(t *testing.T, ate *ondatra.ATEDevice, portNames []string) map[string]portFrameCounters {
	t.Helper()
	counters := make(map[string]portFrameCounters, len(portNames))
	ports := make(map[string]*otgtelemetry.Port)
	for _, port := range gnmi.GetAll(t, ate.OTG(), gnmi.OTG().PortAny().State()) {
		ports[port.GetName()] = port
	}
	for _, portName := range portNames {
		portID := ate.Port(t, portName).ID()
		port, ok := ports[portID]
		if !ok {
			t.Fatalf("no OTG counter telemetry for port %s", portID)
		}
		counters[portName] = portFrameCounters{
			in:  port.GetCounters().GetInFrames(),
			out: port.GetCounters().GetOutFrames(),
		}
	}
	return counters
}

func allBundlePorts() []string {
	ports := append([]string{}, custPorts...)
	ports = append(ports, corePorts1...)
	return append(ports, corePorts2...)
}

func verifyBundleLoadBalancing(t *testing.T, ate *ondatra.ATEDevice, baseline map[string]portFrameCounters) {
	t.Helper()
	after := readOTGPortCounters(t, ate, allBundlePorts())
	verifyBalancedMembers(t, "customer LAG ATE-to-DUT", custPorts, baseline, after, true)
	verifyBalancedMembers(t, "customer LAG DUT-to-ATE", custPorts, baseline, after, false)
	verifyBalancedMembers(t, "core LAG 1 DUT-to-ATE", corePorts1, baseline, after, false)
	verifyBalancedMembers(t, "core LAG 1 ATE-to-DUT", corePorts1, baseline, after, true)
	verifyBalancedMembers(t, "core LAG 2 DUT-to-ATE", corePorts2, baseline, after, false)
	verifyBalancedMembers(t, "core LAG 2 ATE-to-DUT", corePorts2, baseline, after, true)
}

func verifyBalancedMembers(t *testing.T, label string, members []string, before, after map[string]portFrameCounters, useOut bool) {
	t.Helper()
	var total uint64
	deltas := make(map[string]uint64, len(members))
	for _, member := range members {
		oldCount, newCount := before[member].in, after[member].in
		if useOut {
			oldCount, newCount = before[member].out, after[member].out
		}
		if newCount < oldCount {
			t.Errorf("%s counter reset on %s: before=%d after=%d", label, member, oldCount, newCount)
			return
		}
		deltas[member] = newCount - oldCount
		total += deltas[member]
	}
	if total == 0 {
		t.Errorf("%s carried no frames", label)
		return
	}
	for _, member := range members {
		share := float64(deltas[member]) / float64(total)
		if share < 0.45 || share > 0.55 {
			t.Errorf("%s member %s carried %.1f%% of frames, want 45-55%% (deltas=%v)", label, member, share*100, deltas)
		}
	}
}

func runTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, flowNames []string) {
	runTrafficFlowsFor(t, ate, flowNames, trafficDuration)
}

func runTrafficFlowsFor(t *testing.T, ate *ondatra.ATEDevice, flowNames []string, duration time.Duration) {
	t.Helper()
	if duration <= 0 {
		return
	}
	controlState := gosnappi.NewControlState()
	controlState.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.START).SetFlowNames(flowNames)
	ate.OTG().SetControlState(t, controlState)
	time.Sleep(duration)
	controlState = gosnappi.NewControlState()
	controlState.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.STOP).SetFlowNames(flowNames)
	ate.OTG().SetControlState(t, controlState)
}

func readStoppedFlowCounters(t *testing.T, ate *ondatra.ATEDevice, flowNames []string) map[string]flowCounterSnapshot {
	t.Helper()
	const quietPeriod = 3 * time.Second
	counters := make(map[string]flowCounterSnapshot, len(flowNames))
	lastChanged := make(map[string]time.Time, len(flowNames))
	var reset error
	quiet := waitForCondition(t, ate.OTG(), gnmi.OTG().Flow(flowNames[0]).State(), 30*time.Second, func(val *ygnmi.Value[*otgtelemetry.Flow]) bool {
		for _, name := range flowNames {
			flow, ok := gnmi.Lookup(t, ate.OTG(), gnmi.OTG().Flow(name).State()).Val()
			if !ok || flow == nil {
				continue
			}
			current := flowCounterSnapshot{tx: flow.GetCounters().GetOutPkts(), rx: flow.GetCounters().GetInPkts()}
			previous, seen := counters[name]
			if seen && (current.tx < previous.tx || current.rx < previous.rx) {
				reset = fmt.Errorf("%s counters reset while stopped: before=%+v after=%+v", name, previous, current)
				return true
			}
			if !seen || current != previous {
				lastChanged[name] = time.Now()
			}
			counters[name] = current
		}
		if reset != nil {
			return true
		}
		for _, name := range flowNames {
			if changed, ok := lastChanged[name]; !ok || time.Since(changed) < quietPeriod {
				return false
			}
		}
		return true
	})
	if !quiet {
		t.Fatalf("stopped flow counters did not remain quiet for %v within 30s: %v", quietPeriod, counters)
	}
	if reset != nil {
		t.Fatal(reset)
	}
	return counters
}

func verifyDUTEncryptedCounterDelta(t *testing.T, dut *ondatra.DUTDevice, flowNames []string, flowBefore, flowAfter map[string]flowCounterSnapshot, portIDs []string, macsecBefore map[string]uint64) {
	t.Helper()
	var wantEncrypted uint64
	for _, flowName := range flowNames {
		before, after := flowBefore[flowName], flowAfter[flowName]
		if after.tx < before.tx {
			t.Fatalf("%s TX counter reset during MACsec counter window", flowName)
		}
		wantEncrypted += after.tx - before.tx
	}
	if wantEncrypted == 0 {
		t.Fatal("reverse flows transmitted no packets during the MACsec counter window")
	}
	interfaces := make(map[string]string, len(portIDs))
	for _, portID := range portIDs {
		if _, ok := macsecBefore[portID]; !ok {
			t.Fatalf("no MACsec counter snapshot for port %s", portID)
		}
		interfaces[portID] = dut.Port(t, portID).Name()
	}
	var gotEncrypted uint64
	var reset error
	ready := waitForCondition(t, dut, gnmi.OC().Macsec().State(), 30*time.Second, func(val *ygnmi.Value[*oc.Macsec]) bool {
		macsec, ok := val.Val()
		if !ok || macsec == nil {
			return false
		}
		gotEncrypted = 0
		for portID, name := range interfaces {
			intf := macsec.Interface[name]
			if intf == nil || len(intf.ScsaTx) == 0 {
				return false
			}
			var encrypted uint64
			for _, association := range intf.ScsaTx {
				encrypted += association.GetCounters().GetScEncrypted()
			}
			if encrypted < macsecBefore[portID] {
				reset = fmt.Errorf("DUT MACsec sc-encrypted counter reset on %s", portID)
				return true
			}
			gotEncrypted += encrypted - macsecBefore[portID]
		}
		return gotEncrypted >= wantEncrypted
	})
	if !ready {
		t.Errorf("DUT MACsec sc-encrypted delta=%d did not reach reverse ATE TX packet delta=%d within 30s (flows=%v)", gotEncrypted, wantEncrypted, flowNames)
	}
	if reset != nil {
		t.Fatal(reset)
	}
}

// verifyTraffic verifies that the named flow forwarded with acceptable loss.
func verifyTraffic(t *testing.T, flowName string, before, after flowCounterSnapshot) error {
	t.Helper()
	if after.tx < before.tx || after.rx < before.rx {
		return fmt.Errorf("%s: flow counters reset: before=%+v after=%+v", flowName, before, after)
	}
	tx, rx := after.tx-before.tx, after.rx-before.rx
	if tx == 0 {
		return fmt.Errorf("%s: traffic verification failed: FramesTx=0", flowName)
	}
	if rx > tx {
		rx = tx
	}
	lossPct := 100 * float64(tx-rx) / float64(tx)
	if lossPct > maxForwardingLossPercent {
		return fmt.Errorf("%s: traffic verification failed: FramesTx=%d FramesRx=%d (loss=%.2f%%, want <= %.1f%%)", flowName, tx, rx, lossPct, maxForwardingLossPercent)
	}
	t.Logf("%s: traffic verification passed: FramesTx=%d FramesRx=%d (loss=%.2f%%)", flowName, tx, rx, lossPct)
	return nil
}

func gueEncapPacketValidations() []*packetvalidationhelpers.PacketValidation {
	var validations []*packetvalidationhelpers.PacketValidation
	for _, portName := range append(append([]string{}, corePorts1...), corePorts2...) {
		validations = append(validations, &packetvalidationhelpers.PacketValidation{
			PortName:           portName,
			CaptureName:        "encap-" + portName,
			IPv4Layer:          &packetvalidationhelpers.IPv4Layer{Protocol: packetvalidationhelpers.UDP, DstIP: mplsGueTunnelDst, TTL: mplsGueOuterTTL},
			UDPLayer:           &packetvalidationhelpers.UDPLayer{DstPort: mplsGueUDPDstPort},
			MatchEncapsulation: true,
			MPLSLayer:          &packetvalidationhelpers.MPLSLayer{Label: mplsGueLabel, Tc: 0},
			Validations:        gueEncapValidations,
		})
	}
	return validations
}

func validatePacketOnAnyPort(t *testing.T, ate *ondatra.ATEDevice, validations []*packetvalidationhelpers.PacketValidation, encapsulation string) error {
	t.Helper()
	var errs []error
	for _, pv := range validations {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, pv); err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", pv.PortName, err))
			continue
		}
		t.Logf("Validated %s encapsulation on %s", encapsulation, pv.PortName)
		return nil
	}
	return fmt.Errorf("no core LAG member carried a valid %s packet: %v", encapsulation, errs)
}

// validateDecapMACsec checks MACsec headers on the customer link, not the
// encrypted inner IPv4 fields. The decap flow hashes to a single
// customer LAG member, so the capture is validated on every customer port and
// the check passes if at least one port carries the MACsec frame.
func validateDecapMACsec(t *testing.T, ate *ondatra.ATEDevice) error {
	t.Helper()
	var errs []error
	for _, dv := range decapPacketValidations() {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, dv); err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", dv.PortName, err))
			continue
		}
		t.Logf("Validated decapped MACsec traffic on %s", dv.PortName)
		return nil
	}
	return fmt.Errorf("no customer port carried the decapped MACsec frame: %v", errs)
}

// verifyForwarding runs one capture+traffic window and verifies all enabled
// flows forward without loss, encapsulation headers are correct, and decapped
// traffic egresses MACsec-encrypted.
func verifyForwarding(t *testing.T, args *testArgs) {
	waitForOTGLAGUp(t, args.ate, ateCustLagName, uint64(len(custPorts)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLagName, uint64(len(corePorts1)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLag2Name, uint64(len(corePorts2)), sessionUpTimeout)
	// The capture-start port flap may also reset the MACsec session; wait for it
	// to re-secure and allow the inline crypto data plane to converge before
	// starting traffic.

	for i := range custPorts {
		waitForOTGMACSecUp(t, args.ate, macsecPeerNames[i], macsecUpTimeout)
	}

	otgutils.WaitForARP(t, args.ate.OTG(), args.top, "IPv4")
	otgutils.WaitForARP(t, args.ate.OTG(), args.top, "IPv6")
	flowNames := enabledFlows()
	flowBaseline := readFlowCounters(t, args.ate, flowNames)
	var macsecBaseline map[string]uint64
	if !deviations.MacsecOcUnsupported(args.dut) {
		macsecBaseline = readDUTEncryptedCounters(t, args.dut, custPorts)
	} else {
		t.Log("Skipping DUT sc-encrypted OC counter check: MACsec OC telemetry is unsupported by deviation")
	}
	portBaseline := readOTGPortCounters(t, args.ate, allBundlePorts())
	runTrafficFlowsFor(t, args.ate, flowNames, forwardingTrafficDuration)
	flowAfter := readStoppedFlowCounters(t, args.ate, flowNames)
	verifyBundleLoadBalancing(t, args.ate, portBaseline)
	if !deviations.MacsecOcUnsupported(args.dut) {
		reverseFlows := []string{flowIPv4Decap, flowIPv4DecapCore2, flowGUEIPv4Decap, flowGUEIPv4DecapCore2}
		verifyDUTEncryptedCounterDelta(t, args.dut, reverseFlows, flowBaseline, flowAfter, custPorts, macsecBaseline)
	}

	otgutils.LogFlowMetrics(t, args.ate.OTG(), args.top)
	for _, flowName := range flowNames {
		if err := verifyTraffic(t, flowName, flowBaseline[flowName], flowAfter[flowName]); err != nil {
			t.Errorf("traffic verification failed: %v", err)
		}
	}

	var captureFlowNames []string
	var greValidations, gueValidations []*packetvalidationhelpers.PacketValidation

	captureFlowNames = append(captureFlowNames, flowIPv4Fwd, flowIPv6Fwd, flowIPv4Decap, flowIPv4DecapCore2)
	greValidations = encapPacketValidations()

	captureFlowNames = append(captureFlowNames, flowGUEIPv4Fwd, flowGUEIPv6Fwd, flowGUEIPv4Decap, flowGUEIPv4DecapCore2)
	gueValidations = gueEncapPacketValidations()

	cs := packetvalidationhelpers.StartCapture(t, args.ate)
	waitForOTGLAGUp(t, args.ate, ateCustLagName, uint64(len(custPorts)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLagName, uint64(len(corePorts1)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLag2Name, uint64(len(corePorts2)), sessionUpTimeout)

	for i := range custPorts {
		waitForOTGMACSecUp(t, args.ate, macsecPeerNames[i], macsecUpTimeout)
	}

	runTrafficFlowsFor(t, args.ate, captureFlowNames, packetCaptureDuration)
	packetvalidationhelpers.StopCapture(t, args.ate, cs)

	if err := validatePacketOnAnyPort(t, args.ate, greValidations, "MPLSoGRE"); err != nil {
		t.Errorf("MPLSoGRE encap validation failed: %v", err)
	}

	if err := validatePacketOnAnyPort(t, args.ate, gueValidations, "MPLSoGUE"); err != nil {
		t.Errorf("MPLSoGUE encap validation failed: %v", err)
	}

	if err := validateDecapMACsec(t, args.ate); err != nil {
		t.Errorf("decap MACsec validation failed: %v", err)
	}
}

// forceKeyMismatch re-pushes the ATE with mismatched customer keys and waits for
// MKA to renegotiate, breaking the MACsec session on the customer LAG.
func forceKeyMismatch(t *testing.T, args *testArgs, probeOnly bool) {
	t.Log("Forcing MKA renegotiation with mismatched customer keys")
	badTop := buildATE(t, ateOptions{
		anchor:            args.anchor,
		corruptKeys:       true,
		shouldSecureProbe: probeOnly,
		probeOnly:         probeOnly,
		dutCustMAC:        args.dutCustMAC,
		dutCoreMAC:        args.dutCoreMAC,
		dutCore2MAC:       args.dutCore2MAC,
	})
	pushAndStartOTG(t, args.ate, badTop)
	args.top = badTop
	for _, peerName := range macsecPeerNames {
		waitForOTGMACSecState(t, args.ate, peerName, otgtelemetry.Interface_SessionState_DOWN, sessionUpTimeout)
	}
}

func waitForTrafficDuration(t *testing.T, ate *ondatra.ATEDevice, flowNames []string, duration time.Duration) {
	t.Helper()
	if duration <= 0 {
		return
	}
	baseline := readFlowCounters(t, ate, flowNames)
	time.Sleep(duration)
	minimumPackets := uint64(math.Ceil(float64(trafficPPS) * duration.Seconds()))
	if err := awaitFlowTX(t, ate, flowNames, baseline, minimumPackets, 30*time.Second); err != nil {
		t.Fatal(err)
	}
}

func awaitFlowTX(t *testing.T, ate *ondatra.ATEDevice, flowNames []string, baseline map[string]flowCounterSnapshot, minimumPackets uint64, timeout time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for _, flowName := range flowNames {
		counterBase := baseline[flowName].tx
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("traffic TX readiness deadline exceeded before checking flow %s", flowName)
		}
		var current uint64
		present := false
		ready := waitForCondition(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().OutPkts().State(), remaining, func(val *ygnmi.Value[uint64]) bool {
			value, ok := val.Val()
			if !ok {
				return false
			}
			current, present = value, true
			return current < counterBase || current-counterBase >= minimumPackets
		})
		if present && current < counterBase {
			return fmt.Errorf("flow %s TX counter reset: before=%d after=%d", flowName, counterBase, current)
		}
		if !ready || !present || current-counterBase < minimumPackets {
			return fmt.Errorf("flow %s TX did not advance by %d packets within %v: baseline=%d current=%d present=%t", flowName, minimumPackets, timeout, counterBase, current, present)
		}
	}
	return nil
}

// verifyTrafficLoss runs a traffic window and asserts every enabled flow either
// hit ~100% loss (wantDrop) or forwarded with negligible loss (!wantDrop).
func verifyTrafficLoss(t *testing.T, args *testArgs, wantDrop bool) {
	t.Helper()
	flowNames := enabledFlows()
	startState := gosnappi.NewControlState()
	startState.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.START).SetFlowNames(flowNames)
	stopped := true
	stopTraffic := func() {
		if stopped {
			return
		}
		stopState := gosnappi.NewControlState()
		stopState.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.STOP).SetFlowNames(flowNames)
		args.ate.OTG().SetControlState(t, stopState)
		stopped = true
	}
	defer stopTraffic()
	for attempt := 1; attempt <= maxFlowValidationAttempts; attempt++ {
		stopped = false
		args.ate.OTG().SetControlState(t, startState)
		startupBaseline := readFlowCounters(t, args.ate, flowNames)
		if err := awaitFlowTX(t, args.ate, flowNames, startupBaseline, 1, 30*time.Second); err != nil {
			if wantDrop || attempt == maxFlowValidationAttempts {
				t.Fatalf("traffic did not become ready before loss measurement (attempt %d): %v", attempt, err)
			}
			t.Logf("traffic TX did not advance before measurement; restarting once: %v", err)
			stopTraffic()
			waitForOTGLAGUp(t, args.ate, ateCustLagName, uint64(len(custPorts)), sessionUpTimeout)
			waitForOTGLAGUp(t, args.ate, ateCoreLagName, uint64(len(corePorts1)), sessionUpTimeout)
			waitForOTGLAGUp(t, args.ate, ateCoreLag2Name, uint64(len(corePorts2)), sessionUpTimeout)

			for _, peerName := range macsecPeerNames {
				waitForOTGMACSecUp(t, args.ate, peerName, macsecUpTimeout)
			}

			continue
		}
		break
	}
	baseline := readFlowCounters(t, args.ate, flowNames)
	waitForTrafficDuration(t, args.ate, flowNames, trafficDuration)
	stopTraffic()
	after := readStoppedFlowCounters(t, args.ate, flowNames)
	for _, f := range flowNames {
		before, current := baseline[f], after[f]
		if current.tx < before.tx || current.rx < before.rx {
			t.Errorf("%s: flow counters reset during traffic window: before=%+v after=%+v", f, before, current)
			continue
		}
		tx, rx := current.tx-before.tx, current.rx-before.rx
		if tx == 0 {
			t.Errorf("%s: transmitted zero packets; cannot evaluate forwarding or drop behavior", f)
			continue
		}
		loss := 100.0
		if tx > 0 {
			if rx > tx {
				rx = tx
			}
			loss = 100 * float64(tx-rx) / float64(tx)
		}
		switch {
		case wantDrop && loss < minDropLossPercent:
			t.Errorf("%s: got %.1f%% loss, want ~100%% (traffic must be dropped)", f, loss)
		case !wantDrop && loss > maxForwardingLossPercent:
			t.Errorf("%s: got %.1f%% loss, want ~0%% (traffic must be forwarded)", f, loss)
		default:
			t.Logf("%s: %.1f%% loss (wantDrop=%v)", f, loss, wantDrop)
		}
	}
}

func flowCounterDeltas(t *testing.T, flowNames []string, before, after map[string]flowCounterSnapshot, phase string) map[string]flowCounterSnapshot {
	t.Helper()
	deltas := make(map[string]flowCounterSnapshot, len(flowNames))
	for _, flowName := range flowNames {
		oldCount, newCount := before[flowName], after[flowName]
		if newCount.tx < oldCount.tx || newCount.rx < oldCount.rx {
			t.Errorf("%s (%s): flow counters reset during traffic window: before=%+v after=%+v", flowName, phase, oldCount, newCount)
			continue
		}
		deltas[flowName] = flowCounterSnapshot{tx: newCount.tx - oldCount.tx, rx: newCount.rx - oldCount.rx}
	}
	return deltas
}

func logFlowCounters(t *testing.T, flowNames []string, counters map[string]flowCounterSnapshot, phase string) {
	t.Helper()
	for _, flowName := range flowNames {
		tx := counters[flowName].tx
		rx := counters[flowName].rx
		loss := 100.0
		if tx > 0 {
			if rx > tx {
				rx = tx
			}
			loss = 100 * float64(tx-rx) / float64(tx)
		}
		t.Logf("%s (%s): FramesTx=%d FramesRx=%d loss=%.1f%%",
			flowName, phase, tx, rx, loss)
	}
}

func validateFlowCounters(t *testing.T, flowNames []string, counters map[string]flowCounterSnapshot, wantDrop map[string]bool, phase string) {
	t.Helper()
	for _, flowName := range flowNames {
		tx := counters[flowName].tx
		rx := counters[flowName].rx
		if tx == 0 {
			t.Errorf("%s (%s): transmitted zero packets", flowName, phase)
			continue
		}
		if rx > tx {
			rx = tx
		}
		loss := 100 * float64(tx-rx) / float64(tx)
		switch {
		case wantDrop[flowName] && loss < minDropLossPercent:
			t.Errorf("%s (%s): got %.1f%% loss, want ~100%%", flowName, phase, loss)
		case !wantDrop[flowName] && loss > maxForwardingLossPercent:
			t.Errorf("%s (%s): got %.1f%% loss, want ~0%%", flowName, phase, loss)
		}
	}
}

// checkShouldSecureFlows runs the selected flows individually and validates each
// using packet-counter deltas for this window.
func checkShouldSecureFlows(t *testing.T, args *testArgs, phase string, wantDrop map[string]bool) {
	t.Helper()
	flowNames := make([]string, 0, len(wantDrop))
	for flowName := range wantDrop {
		if flowName != "" {
			flowNames = append(flowNames, flowName)
		}
	}
	sort.Strings(flowNames)
	counters := make(map[string]flowCounterSnapshot, len(flowNames))
	for _, flowName := range flowNames {
		flowList := []string{flowName}
		baseline := readFlowCounters(t, args.ate, flowList)
		runTrafficFlows(t, args.ate, flowList)
		after := readStoppedFlowCounters(t, args.ate, flowList)
		deltas := flowCounterDeltas(t, flowList, baseline, after, phase)
		counters[flowName] = deltas[flowName]
	}
	logFlowCounters(t, flowNames, counters, phase)
	validateFlowCounters(t, flowNames, counters, wantDrop, phase)
}

// checkCleartextTraffic starts the already-configured cleartext flows and
// checks their loss against the expected policy behavior.
func checkCleartextTraffic(t *testing.T, args *testArgs, wantDrop bool) {
	t.Helper()
	checkShouldSecureFlows(t, args, "cleartext", map[string]bool{
		flowShouldSecureIPv4:    wantDrop,
		flowShouldSecureIPv6:    wantDrop,
		flowShouldSecureGUEIPv4: wantDrop,
		flowShouldSecureGUEIPv6: wantDrop,
	})
}

// verifyCleartextTraffic configures a cleartext probe while MKA is down, then
// checks the expected must-/should-secure behavior.
func verifyCleartextTraffic(t *testing.T, args *testArgs, wantDrop bool) {
	probeTop := buildATE(t, ateOptions{
		anchor:            args.anchor,
		corruptKeys:       true,
		shouldSecureProbe: true,
		probeOnly:         true,
		dutCustMAC:        args.dutCustMAC,
		dutCoreMAC:        args.dutCoreMAC,
		dutCore2MAC:       args.dutCore2MAC,
	})
	pushAndStartOTG(t, args.ate, probeTop)
	args.top = probeTop
	for _, peerName := range macsecPeerNames {
		waitForOTGMACSecState(t, args.ate, peerName, otgtelemetry.Interface_SessionState_DOWN, sessionUpTimeout)
	}
	waitForOTGLAGUp(t, args.ate, ateCustLagName, uint64(len(custPorts)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLagName, uint64(len(corePorts1)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLag2Name, uint64(len(corePorts2)), sessionUpTimeout)
	checkCleartextTraffic(t, args, wantDrop)
}

// testMustSecure implements PF-1.17.2: with the must-secure policy bound, verify
// forwarding, then break the keys and confirm traffic is fully dropped.
func testMustSecure(t *testing.T, args *testArgs) {
	verifyForwarding(t, args)

	forceKeyMismatch(t, args, false)
	verifyTrafficLoss(t, args, true)      // must-secure: mismatch -> 100% drop
	verifyCleartextTraffic(t, args, true) // must-secure: cleartext probe -> 100% drop
}

func checkOTGMACsecSessions(t *testing.T, args *testArgs, wantUp bool, phase string) {
	t.Helper()
	for _, peerName := range macsecPeerNames {
		state := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Macsec().Interface(peerName).SessionState().State())
		if gotUp := state == otgtelemetry.Interface_SessionState_UP; gotUp != wantUp {
			t.Errorf("MACsec session %s %s: got state %v, want UP=%t", peerName, phase, state, wantUp)
		}
	}
}

func rateLimitedCaptureConfig(t *testing.T, source gosnappi.Config, flowNames []string) gosnappi.Config {
	t.Helper()
	captureTop, err := source.Clone()
	if err != nil {
		t.Fatalf("failed to clone OTG config for packet capture: %v", err)
	}
	wantedFlows := make(map[string]bool, len(flowNames))
	for _, flowName := range flowNames {
		wantedFlows[flowName] = true
	}
	flows := captureTop.Flows().Items()
	captureTop.Flows().Clear()
	for _, flow := range flows {
		if wantedFlows[flow.Name()] {
			delete(wantedFlows, flow.Name())
		}
		flow.Rate().SetPps(packetCapturePPS)
		captureTop.Flows().Append(flow)
	}
	if len(wantedFlows) != 0 {
		t.Fatalf("OTG config is missing capture flows: %v", wantedFlows)
	}
	return captureTop
}

func runAllTrafficFor(t *testing.T, ate *ondatra.ATEDevice, duration time.Duration) {
	t.Helper()
	if duration <= 0 {
		return
	}
	ate.OTG().StartTraffic(t)
	time.Sleep(duration)
	ate.OTG().StopTraffic(t)
}

// testShouldSecure implements PF-1.17.3: verify cleartext traffic is forwarded
// after the should-secure key chain expires.
func testShouldSecure(t *testing.T, args *testArgs) {
	verifyForwarding(t, args)
	forceKeyMismatch(t, args, true)
	checkOTGMACsecSessions(t, args, false, "after key mismatch")
	wantDrop := map[string]bool{
		flowShouldSecureIPv4:    false,
		flowShouldSecureIPv6:    false,
		flowShouldSecureGUEIPv4: false,
		flowShouldSecureGUEIPv6: false,
	}
	checkShouldSecureFlows(t, args, "after key mismatch", wantDrop)
	var captureFlowNames []string
	var greValidations, gueValidations []*packetvalidationhelpers.PacketValidation

	captureFlowNames = append(captureFlowNames, flowShouldSecureIPv4, flowShouldSecureIPv6)
	greValidations = encapPacketValidations()

	captureFlowNames = append(captureFlowNames, flowShouldSecureGUEIPv4, flowShouldSecureGUEIPv6)
	gueValidations = gueEncapPacketValidations()

	captureTop := rateLimitedCaptureConfig(t, args.top, captureFlowNames)
	pushAndStartOTG(t, args.ate, captureTop)
	for _, peerName := range macsecPeerNames {
		waitForOTGMACSecState(t, args.ate, peerName, otgtelemetry.Interface_SessionState_DOWN, sessionUpTimeout)
	}
	waitForOTGLAGUp(t, args.ate, ateCustLagName, uint64(len(custPorts)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLagName, uint64(len(corePorts1)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLag2Name, uint64(len(corePorts2)), sessionUpTimeout)
	cs := packetvalidationhelpers.StartCapture(t, args.ate)
	runAllTrafficFor(t, args.ate, packetCaptureDuration)
	packetvalidationhelpers.StopCapture(t, args.ate, cs)

	if err := validatePacketOnAnyPort(t, args.ate, greValidations, "MPLSoGRE after key mismatch"); err != nil {
		t.Errorf("MPLSoGRE header validation failed: %v", err)
	}

	if err := validatePacketOnAnyPort(t, args.ate, gueValidations, "MPLSoGUE after key mismatch"); err != nil {
		t.Errorf("MPLSoGUE header validation failed: %v", err)
	}

}
func readDUTTime(t *testing.T, dut *ondatra.DUTDevice) time.Time {
	t.Helper()
	clock := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	current, err := time.Parse(time.RFC3339, clock)
	if err != nil {
		t.Fatalf("failed to parse DUT current-datetime %q: %v", clock, err)
	}
	return current
}

func requireDUTTimeBefore(t *testing.T, dut *ondatra.DUTDevice, target time.Time, phase string) {
	t.Helper()
	if current := readDUTTime(t, dut); !current.Before(target) {
		t.Fatalf("%s missed its scheduled window: DUT clock=%s deadline=%s; refusing to skip key transitions", phase, current.Format(time.RFC3339), target.Format(time.RFC3339))
	}
}

func waitForDUTTime(t *testing.T, dut *ondatra.DUTDevice, target time.Time) {
	t.Helper()
	current := readDUTTime(t, dut)
	if !current.Before(target) {
		return
	}
	timeout := target.Sub(current) + sessionUpTimeout
	if !waitForCondition(t, dut, gnmi.OC().System().CurrentDatetime().State(), timeout, func(val *ygnmi.Value[string]) bool {
		clock, ok := val.Val()
		if !ok {
			return false
		}
		current, err := time.Parse(time.RFC3339, clock)
		return err == nil && !current.Before(target)
	}) {
		t.Fatalf("DUT clock did not reach %s; current clock=%s", target.UTC().Format(time.RFC3339),
			gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State()))
	}
}

// testKeyRotation implements PF-1.17.4: with the must-secure policy bound, verify
// no loss across the five-key rotation, then confirm 100% loss once all keys
// expire.
func testKeyRotation(t *testing.T, args *testArgs) {
	// Fresh chain so the whole rotation window is ahead of the current time.
	reanchorMACsecWithExtension(t, args, mustSecureProfile, macsecRotationSetupExtension)
	checkOTGMACsecSessions(t, args, true, "before key rotation")
	flowNames := enabledFlows()
	controlState := gosnappi.NewControlState()
	controlState.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.START).SetFlowNames(flowNames)
	defer func() {
		stopState := gosnappi.NewControlState()
		stopState.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.STOP).SetFlowNames(flowNames)
		args.ate.OTG().SetControlState(t, stopState)
	}()
	startupDeadline := args.anchor.Add(macsecRotationSetupExtension + macsecKeyStagger).Add(-10 * time.Second)
	api := args.ate.RawAPIs().OTG(t)
	for attempt := 1; attempt <= maxFlowValidationAttempts; attempt++ {
		requireDUTTimeBefore(t, args.dut, startupDeadline, "rotation traffic startup")
		response, err := api.SetControlState(controlState)
		if err != nil {
			t.Fatalf("rotation traffic START failed: %v", err)
		}
		if response != nil && len(response.Warnings()) > 0 {
			t.Logf("rotation traffic START warnings: %v", response.Warnings())
		}
		startupBaseline := readFlowCounters(t, args.ate, flowNames)
		err = awaitFlowTX(t, args.ate, flowNames, startupBaseline, 1, 15*time.Second)
		if err != nil {
			if attempt == maxFlowValidationAttempts {
				t.Fatalf("rotation traffic did not start after %d attempts: %v", attempt, err)
			}
			t.Logf("rotation traffic startup failed before measurement; restarting once: %v", err)
			stopState := gosnappi.NewControlState()
			stopState.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.STOP).SetFlowNames(flowNames)
			args.ate.OTG().SetControlState(t, stopState)
			for lagName, members := range map[string][]string{ateCustLagName: custPorts, ateCoreLagName: corePorts1, ateCoreLag2Name: corePorts2} {
				waitForOTGLAGUp(t, args.ate, lagName, uint64(len(members)), sessionUpTimeout)
			}

			for _, peerName := range macsecPeerNames {
				waitForOTGMACSecUp(t, args.ate, peerName, macsecUpTimeout)
			}

			continue
		}
		break
	}
	requireDUTTimeBefore(t, args.dut, startupDeadline, "rotation measurement startup")
	baseline := readFlowCounters(t, args.ate, flowNames)
	checkTrafficWindow := func(phase string, wantDrop bool) {
		otgutils.LogFlowMetrics(t, args.ate.OTG(), args.top)
		after := readFlowCounters(t, args.ate, flowNames)
		deltas := make(map[string]flowCounterSnapshot, len(flowNames))
		expectations := make(map[string]bool, len(flowNames))
		for _, flowName := range flowNames {
			before := baseline[flowName]
			current := after[flowName]
			if current.tx < before.tx || current.rx < before.rx {
				t.Fatalf("%s (%s): flow counters reset during continuous traffic: before=%+v after=%+v", flowName, phase, before, current)
			}
			deltas[flowName] = flowCounterSnapshot{tx: current.tx - before.tx, rx: current.rx - before.rx}
			expectations[flowName] = wantDrop
		}
		logFlowCounters(t, flowNames, deltas, phase)
		validateFlowCounters(t, flowNames, deltas, expectations, phase)
		baseline = after
	}
	waitForTrafficDuration(t, args.ate, flowNames, trafficDuration)
	checkTrafficWindow("before key rotation", false)

	// Step 2: Verify hitless forwarding across key rotation transitions (Keys 1 -> 2 -> 3 -> 4 -> 5).
	// Keys stagger by 1 minute; test traffic across each transition window.
	for k := 1; k < macsecNumPSKs; k++ {
		nextKeyTime := args.anchor.Add(macsecRotationSetupExtension + time.Duration(k)*macsecKeyStagger)
		requireDUTTimeBefore(t, args.dut, nextKeyTime, fmt.Sprintf("Key %d transition", k+1))
		waitForDUTTime(t, args.dut, nextKeyTime.Add(10*time.Second))
		t.Logf("Key %d: DUT clock=%s runner clock=%s customer aggregate oper-status=%v", k+1,
			gnmi.Get(t, args.dut, gnmi.OC().System().CurrentDatetime().State()), time.Now().UTC().Format(time.RFC3339),
			gnmi.Get(t, args.dut, gnmi.OC().Interface(args.custAggID).OperStatus().State()))
		for _, lagName := range []string{ateCustLagName, ateCoreLagName, ateCoreLag2Name} {
			oper := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Lag(lagName).OperStatus().State())
			membersUp := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Lag(lagName).Counters().MemberPortsUp().State())
			t.Logf("Key %d: OTG LAG %s oper-status=%v member-ports-up=%d", k+1, lagName, oper, membersUp)
			if oper != otgtelemetry.Lag_OperStatus_UP {
				t.Errorf("OTG LAG %s is not UP during Key %d rotation", lagName, k+1)
			}
		}
		sessionUpTime := 20 * time.Second
		for i := range custPorts {
			stateVal := gnmi.Lookup(t, args.ate.OTG(), gnmi.OTG().Macsec().Interface(macsecPeerNames[i]).SessionState().State())
			state, ok := stateVal.Val()
			if !ok || state != otgtelemetry.Interface_SessionState_UP {
				t.Logf("MACsec session on %s is not UP after key change, waiting up to %v for recovery", macsecPeerNames[i], sessionUpTime)
				if !waitForCondition(t, args.ate.OTG(), gnmi.OTG().Macsec().Interface(macsecPeerNames[i]).SessionState().State(), sessionUpTime, func(val *ygnmi.Value[otgtelemetry.E_Interface_SessionState]) bool {
					sessionState, present := val.Val()
					return present && sessionState == otgtelemetry.Interface_SessionState_UP
				}) {
					t.Errorf("MACsec session on %s did not recover during Key %d transition within %v; session-state=%v", macsecPeerNames[i], k+1, sessionUpTime,
						gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Macsec().Interface(macsecPeerNames[i]).SessionState().State()))
				}
			}
		}
		t.Logf("Verifying traffic forwarding during Key %d rotation window", k+1)
		checkTrafficWindow(fmt.Sprintf("Key %d rotation", k+1), false)
	}

	// Step 3: Wait until all 5 keys expire and verify 100% loss.
	forwardingEnd := args.anchor.Add(macsecRotationSetupExtension + macsecChainDuration).Add(-macsecPostExpiryMargin)
	requireDUTTimeBefore(t, args.dut, forwardingEnd, "final key forwarding")
	waitForDUTTime(t, args.dut, forwardingEnd)
	checkTrafficWindow("final key forwarding", false)
	expiry := args.anchor.Add(macsecRotationSetupExtension + macsecChainDuration).Add(macsecPostExpiryMargin)
	waitForDUTTime(t, args.dut, expiry)
	for _, peerName := range macsecPeerNames {
		waitForOTGMACSecState(t, args.ate, peerName, otgtelemetry.Interface_SessionState_DOWN, sessionUpTimeout)
	}
	baseline = readFlowCounters(t, args.ate, flowNames)
	waitForTrafficDuration(t, args.ate, flowNames, trafficDuration)
	checkTrafficWindow("after all keys expired", true)
}

// testSATimer implements PF-1.17.5: sets the security association key rekey interval
// to 28800 seconds, verifies the MACsec session is up, and verifies forwarding at trafficPPS.
func testSATimer(t *testing.T, args *testArgs) {
	reanchorMACsec(t, args, mustSecureProfile)
	timerBatch := &gnmi.SetBatch{}
	cfgplugins.ConfigureMACsecSAKRekeyInterval(t, args.dut, timerBatch, cfgplugins.MACsecSAKRekeyIntervalConfig{
		ProfileName: mustSecureProfile,
		Interval:    standardSAKRekeyInterval,
	})
	timerBatch.Set(t, args.dut)
	t.Cleanup(func() {
		restoreBatch := &gnmi.SetBatch{}
		cfgplugins.ConfigureMACsecSAKRekeyInterval(t, args.dut, restoreBatch, cfgplugins.MACsecSAKRekeyIntervalConfig{
			ProfileName: mustSecureProfile,
			Interval:    macsecSAKRekeyInterval,
		})
		restoreBatch.Set(t, args.dut)
	})
	if deviations.MacsecOcUnsupported(args.dut) {
		profileConfig := readMACsecProfileConfig(t, args.dut, mustSecureProfile)
		if !strings.Contains(profileConfig, fmt.Sprintf("mka session rekey-period %d", standardSAKRekeyInterval)) {
			t.Fatalf("DUT MACsec profile %s running config does not contain the 28800-second SAK rekey interval", mustSecureProfile)
		}
	} else if got := gnmi.Get(t, args.dut, gnmi.OC().Macsec().Mka().Policy(mustSecureProfile).SakRekeyInterval().State()); got != standardSAKRekeyInterval {
		t.Fatalf("DUT SAK rekey interval=%d, want %d seconds", got, standardSAKRekeyInterval)
	}

	for i := range custPorts {
		waitForOTGMACSecUp(t, args.ate, macsecPeerNames[i], macsecUpTimeout)
	}

	// Simplified verification: just check traffic forwarding without packet format validation.
	// SA timer test is about the rekey interval, not packet encapsulation details.
	waitForOTGLAGUp(t, args.ate, ateCustLagName, uint64(len(custPorts)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLagName, uint64(len(corePorts1)), sessionUpTimeout)
	waitForOTGLAGUp(t, args.ate, ateCoreLag2Name, uint64(len(corePorts2)), sessionUpTimeout)
	verifyTrafficLoss(t, args, false)
}

func testIPv6MPLSoGREUDP(t *testing.T, args *testArgs) {
	if deviations.MplsIPv6TunnelSourceUnsupported(args.dut) {
		t.Skip("PF-1.17.6 requires IPv6 MPLS tunnel-source support")
	}
	t.Skip("PF-1.17.6 IPv6-outer MPLSoGRE/UDP dataplane validation is not implemented yet")
}

func TestMPLSGREUDPMACSEC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dutConfig := &dutTestConfig{}
	t.Cleanup(func() { cleanupDUTConfig(t, dut, dutConfig) })
	t.Cleanup(func() {
		ate.OTG().StopTraffic(t)
		ate.OTG().StopProtocols(t)
	})

	// Start with the static customer LAG used by every subtest.
	anchor := macsecChainAnchor(t, dut)
	custAggID := configureDUT(t, dut, anchor, dutConfig)
	dutCustMAC := gnmi.Get(t, dut, gnmi.OC().Interface(custAggID).Ethernet().MacAddress().State())
	top := buildATE(t, ateOptions{anchor: anchor, dutCustMAC: dutCustMAC})
	bringUpOTG(t, ate, top, false)
	dutCoreMAC := resolveDUTMAC(t, ate, "d2Eth", ateCoreV4Gw)
	dutCore2MAC := resolveDUTMAC(t, ate, "d4Eth", ateCore2V4Gw)

	args := &testArgs{
		dut:         dut,
		ate:         ate,
		top:         top,
		anchor:      anchor,
		custAggID:   custAggID,
		dutCustMAC:  dutCustMAC,
		dutCoreMAC:  dutCoreMAC,
		dutCore2MAC: dutCore2MAC,
	}
	reanchorMACsec(t, args, mustSecureProfile)

	cases := []struct {
		name              string
		desc              string
		shouldSecureProbe bool
		setupProfile      string
		run               func(t *testing.T, args *testArgs)
	}{
		{"PF_1_17_1_GenerateConfig", "Generate DUT configuration; verify MACsec sessions up", false, "", testGenerateConfig},
		{"PF_1_17_2_MustSecure", "MPLSoGRE/MPLSoGUE forwarding with must-secure policy", false, "", testMustSecure},
		{"PF_1_17_3_ShouldSecure", "MPLSoGRE/MPLSoGUE forwarding with should-secure policy", true, shouldSecureProfile, testShouldSecure},
		{"PF_1_17_4_KeyRotation", "MACsec key rotation across five PSKs", false, "", testKeyRotation},
		{"PF_1_17_5_SAKTimer", "Verify standard Security-Association timer", false, "", testSATimer},
		{"PF_1_17_6_IPv6OuterTunnel", "IPv6 outer MPLSoGRE/UDP encapsulation over MACsec", false, "", testIPv6MPLSoGREUDP},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Log(tc.desc)
			args.shouldSecureProbe = tc.shouldSecureProbe
			if tc.setupProfile != "" {
				reanchorMACsec(t, args, tc.setupProfile)
			}
			tc.run(t, args)
		})
	}
}
