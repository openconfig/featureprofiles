package cfmbase_test

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/iputil"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	packetvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	defaultMTU          = 9216
	plenIPv4            = 24
	plenIPv6            = 126
	tunnelCount         = 16
	tunnelDestinationIP = "203.0.113.0"
	tunnelDestination1  = "203.0.113.10"
	staticTunnelDst1    = "203.0.113.0/24"
	tunnelDestination2  = "203.0.113.10"
	staticTunnelDst2    = "203.0.113.0/24"
	startTunnelSrcIP    = "192.168.80.%d"
	decapGrpName        = "Decap1"
	nexthopGroupName    = "nexthop-gre"
	nexthopType         = "gre"
	pseudowireName      = "PSW"
	localLabel          = 100
	remoteLabel         = 100
	greProtocol         = 47
	cfmMulticastPrefix  = "01:80:c2:00:00:35"
	cfmProfileName      = "cfm_delay_Bundle"
	// scaleSessionCount is the number of subinterfaces considered for
	// CFM-1.1.10; the subinterfaces already used by the topology are skipped.
	scaleSessionCount = 1002
	// alarmMargin is added to the expected CCM loss time when waiting for an
	// alarm to be raised or cleared.
	alarmMargin = 30 * time.Second
	// alarmClearAfterLinkRestoreTimeout bounds the wait for the defects to clear
	// after the ATE customer links are restored, which includes the time for the
	// customer LAG and the pseudowire to come back up.
	alarmClearAfterLinkRestoreTimeout = 2 * time.Minute
	// dmTxIntervalMillis is the delay measurement (DMM) transmission interval.
	dmTxIntervalMillis = 1000
)

// CfmOpCode represents the CFM OpCode.
type CfmOpCode uint8

// CCMInterval represents the CCM transmission interval encoding.
type CCMInterval uint8

const (
	// CfmEtherType is the EtherType for Connectivity Fault Management.
	CfmEtherType layers.EthernetType = 0x8902

	// CcmOpCode is the OpCode for Continuity Check Message.
	CcmOpCode CfmOpCode = 1

	// CcmInterval300ms is the encoded value for a 300ms CCM interval, as per IEEE 802.1Q.
	CcmInterval300ms CCMInterval = 3

	CcmInterval1S CCMInterval = 4
)

// ccmIntervalMap maps the OC CCM interval to the CCM PDU interval encoding.
// The sessions use a 1S CCM interval because the OC ccm-interval enumeration
// does not support the 100ms interval described in the README.
var ccmIntervalMap = map[oc.E_MaintenanceAssociation_CcmInterval]uint8{
	oc.MaintenanceAssociation_CcmInterval_1S: 4,
}

type dutData struct {
	dut              *ondatra.DUTDevice
	lagAggID         string
	custPort         []string
	transitPort      []string
	neighborPortIPv4 string
	subinterface     int
	cfmCfg           []cfgplugins.MaintenanceDomainConfig
	tunnelDst        string
	staticTunnelDst  string
	capturePort      string
	oam              *oc.Oam
}

var (
	// sfBatch *gnmi.SetBatch
	// oam *oc.Oam

	activity = oc.Lacp_LacpActivityType_ACTIVE
	period   = oc.Lacp_LacpPeriodType_FAST

	lacpParams = &cfgplugins.LACPParams{
		Activity: &activity,
		Period:   &period,
	}

	transitLagData = []*cfgplugins.DUTAggData{
		{
			Attributes:  dut1TransitIntf,
			DutPortsIdx: []int{3},
			LacpParams:  lacpParams,
			AggType:     oc.IfAggregate_AggregationType_LACP,
		},
		{
			Attributes:  dut2TransitIntf,
			DutPortsIdx: []int{3},
			LacpParams:  lacpParams,
			AggType:     oc.IfAggregate_AggregationType_LACP,
		},
	}

	custLagData = []*cfgplugins.DUTAggData{
		{
			Attributes:      dut1custIntf,
			OndatraPortsIdx: []int{0, 1},
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_STATIC,
			SubInterfaces: []*cfgplugins.DUTSubInterfaceData{
				{
					VlanID:        10,
					VlanEnable:    ygot.Bool(false),
					IPv4Address:   net.ParseIP("192.168.10.2"),
					IPv4PrefixLen: plenIPv4,
					IPv6PrefixLen: plenIPv6,
				},
			},
		},
		{
			Attributes:      dut2custIntf,
			OndatraPortsIdx: []int{0, 1},
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_STATIC,
			SubInterfaces: []*cfgplugins.DUTSubInterfaceData{
				{
					VlanID:        10,
					VlanEnable:    ygot.Bool(false),
					IPv4Address:   net.ParseIP("192.168.30.2"),
					IPv4PrefixLen: plenIPv4,
					IPv6PrefixLen: plenIPv6,
				},
			},
		},
	}

	dut1custIntf = attrs.Attributes{
		Desc:    "DUT1 Customer_connect",
		MTU:     defaultMTU,
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dut1TransitIntf = attrs.Attributes{
		Desc:    "DUT1 Transit_Interface",
		MTU:     defaultMTU,
		IPv4:    "192.168.20.1",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:168:20:1",
		IPv6Len: plenIPv6,
	}

	dut2TransitIntf = attrs.Attributes{
		Desc:    "DUT2 Transit_Interface",
		MTU:     defaultMTU,
		IPv4:    "192.168.20.2",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:168:20:2",
		IPv6Len: plenIPv6,
	}

	dut2custIntf = attrs.Attributes{
		Desc:    "DUT2 Customer_connect",
		MTU:     defaultMTU,
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	agg1 = &otgconfighelpers.Port{
		Name:        "Port-Channel1",
		AggMAC:      "02:00:01:01:01:02",
		MemberPorts: []string{"port1", "port2"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{otgIntf1},
		LagID:       1,
		IsLag:       true,
	}

	agg2 = &otgconfighelpers.Port{
		Name:        "Port-Channel3",
		AggMAC:      "02:00:01:01:01:02",
		MemberPorts: []string{"port5", "port6"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{otgIntf2},
		LagID:       2,
		IsLag:       true,
	}

	otgIntf1 = &otgconfighelpers.InterfaceProperties{
		Name: "otgPort1",
		MAC:  "02:00:01:01:01:04",
	}

	otgIntf2 = &otgconfighelpers.InterfaceProperties{
		Name: "ateLag2",
		MAC:  "02:00:01:01:01:05",
	}

	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 18},
		{Size: 1024, Weight: 18},
	}

	FlowIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Flow: &otgvalidationhelpers.FlowParams{TolerancePct: 0.5},
	}

	// The MPLS layer is validated on CCM PDUs by validateCCMEncap, because the
	// capture can also contain customer data frames with a different traffic
	// class.
	Validations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
	}

	OuterGREIPLayerIPv4DUT1 = &packetvalidationhelpers.IPv4Layer{
		Protocol: greProtocol,
		DstIP:    tunnelDestination1,
		TTL:      64,
		Tos:      96,
	}

	OuterGREIPLayerIPv4DUT2 = &packetvalidationhelpers.IPv4Layer{
		Protocol: greProtocol,
		DstIP:    tunnelDestination2,
		TTL:      64,
		Tos:      96,
	}

	MPLSLayer = &packetvalidationhelpers.MPLSLayer{
		Label: uint32(remoteLabel),
		Tc:    1,
	}

	controlWordMPLS = &packetvalidationhelpers.MPLSLayer{
		Label:               uint32(remoteLabel),
		Tc:                  1,
		ControlWordHeader:   true,
		ControlWordSequence: 0,
	}

	encapValidation = []*packetvalidationhelpers.PacketValidation{
		{
			PortName:    "port3",
			Validations: Validations,
			IPv4Layer:   OuterGREIPLayerIPv4DUT1,
			MPLSLayer:   controlWordMPLS,
		},
		{
			PortName:    "port7",
			Validations: Validations,
			IPv4Layer:   OuterGREIPLayerIPv4DUT2,
			MPLSLayer:   controlWordMPLS,
		},
	}

	dutTestData = []dutData{
		{
			custPort:         []string{"port1", "port2"},
			transitPort:      []string{"port4"},
			neighborPortIPv4: "192.168.20.2",
			subinterface:     10,
			tunnelDst:        tunnelDestination1,
			staticTunnelDst:  staticTunnelDst1,
			capturePort:      "port3",
			cfmCfg: []cfgplugins.MaintenanceDomainConfig{
				{
					DomainName: "D1",
					Level:      5,
					MdID:       "10",
					MdNameType: oc.MaintenanceDomain_MdNameType_CHARACTER_STRING,
					Assocs: []cfgplugins.AssociationConfig{
						{
							GroupName:        "GEO_1",
							CcmInterval:      oc.MaintenanceAssociation_CcmInterval_1S,
							LossThreshold:    3,
							MaID:             "S1",
							MaNameType:       oc.MaintenanceAssociation_MaNameType_UINT16,
							LocalMEPID:       1,
							CcmEnabled:       true,
							Direction:        oc.MepEndpoint_Direction_UP,
							TransmitOnDefect: true,
							RemoteMEPID:      2,
						},
					},
				},
			},
		},
		{
			custPort:         []string{"port1", "port2"},
			transitPort:      []string{"port4"},
			neighborPortIPv4: "192.168.20.1",
			subinterface:     10,
			tunnelDst:        tunnelDestination2,
			staticTunnelDst:  staticTunnelDst2,
			capturePort:      "port3",
			cfmCfg: []cfgplugins.MaintenanceDomainConfig{
				{
					DomainName: "D1",
					Level:      5,
					MdID:       "10",
					MdNameType: oc.MaintenanceDomain_MdNameType_CHARACTER_STRING,
					Assocs: []cfgplugins.AssociationConfig{
						{
							GroupName:        "GEO_4",
							CcmInterval:      oc.MaintenanceAssociation_CcmInterval_1S,
							LossThreshold:    3,
							MaID:             "S1",
							MaNameType:       oc.MaintenanceAssociation_MaNameType_UINT16,
							LocalMEPID:       2,
							CcmEnabled:       true,
							Direction:        oc.MepEndpoint_Direction_UP,
							TransmitOnDefect: true,
							RemoteMEPID:      1,
						},
					},
				},
			},
		},
	}
)

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureCFM)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
}

func configureDut(t *testing.T) {
	for index, data := range dutTestData {
		tunnelSrcIPs := []string{}
		sfBatch := &gnmi.SetBatch{}
		fptest.ConfigureDefaultNetworkInstance(t, data.dut)

		data.dut.Port(t, data.capturePort)

		// Get default parameters for OC Policy Forwarding
		ocPFParams := GetDefaultOcPolicyForwardingParams()
		configureHardwareInit(t, data.dut)
		dutLagData := custLagData[index]

		// Create Customer LAG interface
		dutLagData.LagName = netutil.NextAggregateInterface(t, data.dut)
		data.lagAggID = dutLagData.LagName
		dutTestData[index].lagAggID = dutLagData.LagName
		cfgplugins.NewAggregateInterface(t, data.dut, sfBatch, dutLagData)
		sfBatch.Set(t, data.dut)

		// Create transit LAG interface
		transitLagData[index].LagName = netutil.NextAggregateInterface(t, data.dut)
		cfgplugins.NewAggregateInterface(t, data.dut, sfBatch, transitLagData[index])
		sfBatch.Set(t, data.dut)

		// Configure 16 tunnels having single destination address
		for index := range tunnelCount {
			tunnelSrcIPs = append(tunnelSrcIPs, fmt.Sprintf(startTunnelSrcIP, index+10))
		}
		_, ni, _ := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
		greNextHopGroupCfg := cfgplugins.GreNextHopGroupParams{
			NetworkInstance:  ni,
			NexthopGroupName: nexthopGroupName,
			GroupType:        nexthopType,
			SrcAddr:          tunnelSrcIPs,
			DstAddr:          []string{data.tunnelDst},
			TTL:              0,
			Dscp:             96,
		}

		cfgplugins.NextHopGroupConfigForMultipleIP(t, sfBatch, data.dut, greNextHopGroupCfg)

		//Configure MPLS label ranges and qos configs
		cfgplugins.MplsConfig(t, data.dut)
		cfgplugins.QosClassificationConfig(t, data.dut)
		cfgplugins.LabelRangeConfig(t, data.dut)

		// Configure static route from tunnel destination to transit ports
		configureStaticRoute(t, sfBatch, data.dut, data.staticTunnelDst, data.neighborPortIPv4)

		// Configure Decap GRE policy
		cfgplugins.PolicyForwardingGreDecapsulation(t, sfBatch, data.dut, data.tunnelDst, "trafficPolicyName", data.lagAggID, decapGrpName)

		// Configure CFM configs on customer interfaces
		data.oam = configureCFM(t, data.dut, data.lagAggID, data.cfmCfg)
		dutTestData[index].oam = data.oam

		// Configure monitor session to capture packets
		monitorCapt := cfgplugins.MonitorSessionConfig{
			SessionName:       "capture1",
			SourcePort:        transitLagData[index].LagName,
			DestinationDUTAte: data.dut.Port(t, data.capturePort).Name(),
		}
		cfgplugins.ConfigureMonitorSession(t, data.dut, monitorCapt)

	}
}

func configureCFM(t *testing.T, dut *ondatra.DUTDevice, intfName string, cfmCfg []cfgplugins.MaintenanceDomainConfig) *oc.Oam {
	cfmMeasurementProfile := cfgplugins.CFMMeasurementProfile{
		ProfileName:       cfmProfileName,
		BurstInterval:     100,
		IntervalsArchived: 5,

		PacketPerBurst:   100,
		RepetitionPeriod: 1,
	}

	t.Log("Configure CFM performance measurement profile and domain on DUT")
	sfBatch := &gnmi.SetBatch{}
	oam := cfgplugins.ConfigureMeasurementProfile(t, sfBatch, dut, cfmMeasurementProfile)
	if oam != nil {
		sfBatch.Set(t, dut)
	}
	cfmCfg[0].IntfName = intfName
	cfmCfg[0].ProfileName = cfmMeasurementProfile.ProfileName
	cfgplugins.ConfigureCFMDomain(t, oam, dut, &cfmCfg[0])

	return oam
}

func configureStaticRoute(t *testing.T, sfBatch *gnmi.SetBatch, dut *ondatra.DUTDevice, dstAddr string, nexthopIp string) {
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          dstAddr,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nexthopIp),
		},
	}

	if _, err := cfgplugins.NewStaticRouteCfg(sfBatch, sV4, dut); err != nil {
		t.Fatalf("Failed to configure static route: %v", err)
	}
	sfBatch.Set(t, dut)
}

func configureIngressVlan(t *testing.T, dut *ondatra.DUTDevice, intfName string, subinterfaces int, mode string) {
	sfBatch := &gnmi.SetBatch{}
	// Configuring port/attachment mode
	pseudowireCfg := cfgplugins.MplsStaticPseudowire{
		PseudowireName:   pseudowireName,
		NexthopGroupName: nexthopGroupName,
		LocalLabel:       fmt.Sprintf("%d", localLabel),
		RemoteLabel:      fmt.Sprintf("%d", remoteLabel),
		IntfName:         intfName,
		PatchPanel:       "patch-psw",
	}

	vlanClientCfg := cfgplugins.VlanClientEncapsulationParams{
		IntfName:      intfName,
		Subinterfaces: subinterfaces,
	}

	switch mode {
	case "port":
		// Accepts packets from all VLANs
		cfgplugins.ConfigureMplsStaticPseudowire(t, sfBatch, dut, pseudowireCfg)
	case "attachment":
		// Accepts packets only for the specified VLAN
		cfgplugins.RemoveMplsStaticPseudowire(t, sfBatch, dut)
		pseudowireCfg.Subinterface = subinterfaces
		cfgplugins.ConfigureMplsStaticPseudowire(t, sfBatch, dut, pseudowireCfg)
		vlanClientCfg.RemoveVlanConfig = false
		cfgplugins.VlanClientEncapsulation(t, sfBatch, dut, vlanClientCfg)
	case "remove":
		cfgplugins.RemoveMplsStaticPseudowire(t, sfBatch, dut)
		vlanClientCfg.RemoveVlanConfig = true
		cfgplugins.VlanClientEncapsulation(t, sfBatch, dut, vlanClientCfg)
	}
}

func GetDefaultOcPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
	}
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	otgConfig := gosnappi.NewConfig()

	// Create a slice of aggPortData for easier iteration
	aggs := []*otgconfighelpers.Port{agg1, agg2}

	// Configure OTG Interfaces
	for _, agg := range aggs {
		otgconfighelpers.ConfigureNetworkInterface(t, otgConfig, ate, agg)
	}

	// Configuring dummy ports for monitor session to capture packets of DUT interfaces
	for _, port := range []string{"port3", "port7"} {
		portObj := otgConfig.Ports().Add().SetName(port)
		port1Dev := otgConfig.Devices().Add().SetName(port + ".dev")
		port1Eth := port1Dev.Ethernets().Add().SetName(port + ".Eth")
		port1Eth.Connection().SetPortName(portObj.Name())
	}

	return otgConfig
}

func createflow(top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool) {
	if clearFlows {
		top.Flows().Clear()
	}

	params.CreateFlow(top)

	params.AddEthHeader()

	if params.VLANFlow != nil {
		params.AddVLANHeader()
	}

	if params.IPv4Flow != nil {
		params.AddIPv4Header()
	}

	if params.GREFlow != nil {
		params.AddGREHeader()
	}

	if params.MPLSFlow != nil {
		params.AddMPLSHeader()
	}
}

// sendTrafficCapture captures the CCM PDUs received by the ATE for 10 seconds
// while sending the customer traffic flow. The flow is only created by
// CFM-1.1.2, so when it is not configured (e.g. a single subtest is run) the
// capture is taken without traffic; the CCM PDUs are generated by the DUTs.
func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	t.Helper()
	ate.OTG().StartProtocols(t)
	cs := otgutils.StartCapture(t, ate.OTG())
	defer otgutils.StopCapture(t, ate.OTG(), cs)
	hasFlows := len(otgConfig.Flows().Items()) > 0
	if hasFlows {
		ate.OTG().StartTraffic(t)
	} else {
		t.Log("No ATE flows are configured, capturing CCM PDUs without traffic")
	}
	time.Sleep(10 * time.Second)
	if hasFlows {
		ate.OTG().StopTraffic(t)
	}
	otgutils.StopCapture(t, ate.OTG(), cs)
}

// verifyLoadBalanceAcrossGre verifies that the CCM PDUs of each MEP, being a
// constant flow, are carried by a single GRE tunnel and not ECMP'd across all
// available tunnels.
func verifyLoadBalanceAcrossGre(t *testing.T, captures [][]gopacket.Packet) {
	t.Helper()
	t.Log("Validating CCM PDUs are not ECMP'd across the GRE tunnels")

	tunnelSrcs := map[string]map[string]bool{} // MEP MAC -> outer GRE source IPs
	for _, capture := range captures {
		for _, packet := range capture {
			ipv4, ok := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			if !ok {
				continue
			}
			mpls, ok := packet.Layer(layers.LayerTypeMPLS).(*layers.MPLS)
			if !ok || len(mpls.Payload) < 4 {
				continue
			}
			eth, _, ok := ccmPDU(mpls.Payload[4:])
			if !ok {
				continue
			}
			mep := eth.SrcMAC.String()
			if tunnelSrcs[mep] == nil {
				tunnelSrcs[mep] = map[string]bool{}
			}
			tunnelSrcs[mep][ipv4.SrcIP.String()] = true
		}
	}

	if len(tunnelSrcs) == 0 {
		t.Errorf("no encapsulated CCM PDUs found in the capture")
		return
	}
	for mep, srcs := range tunnelSrcs {
		if len(srcs) != 1 {
			t.Errorf("CCM PDUs from MEP %s were ECMP'd across %d GRE tunnels, want 1", mep, len(srcs))
			continue
		}
		t.Logf("CCM PDUs from MEP %s are carried by a single GRE tunnel", mep)
	}
}

// ccmPDU parses frame as an Ethernet frame, optionally 802.1Q tagged, and
// returns the outer Ethernet header and the CFM PDU if it carries a Continuity
// Check Message.
func ccmPDU(frame []byte) (*layers.Ethernet, []byte, bool) {
	inner := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
	eth, ok := inner.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
	if !ok {
		return nil, nil, false
	}
	var cfm []byte
	if dot1q, ok := inner.Layer(layers.LayerTypeDot1Q).(*layers.Dot1Q); ok {
		if dot1q.Type != CfmEtherType {
			return nil, nil, false
		}
		cfm = dot1q.Payload
	} else {
		if eth.EthernetType != CfmEtherType {
			return nil, nil, false
		}
		cfm = eth.Payload
	}
	// The CFM version is the low 5 bits of the first octet, the OpCode is the
	// second octet and the CCM sequence number is in octets 4-7.
	if len(cfm) < 8 || cfm[0]&0x1F != 0 || CfmOpCode(cfm[1]) != CcmOpCode {
		return nil, nil, false
	}
	return eth, cfm, true
}

// validateCfmPacket validates every encapsulated CCM PDU in capture:
// multicast destination, CCM OpCode, CCM interval and optionally the RDI flag.
func validateCfmPacket(t *testing.T, capture []gopacket.Packet, expectedInterval uint8, verifyRDIBit bool) error {
	t.Helper()
	t.Log("Starting CFM packet integrity validation")

	meps := map[string]bool{}
	ccmCount := 0
	for _, packet := range capture {
		mpls, ok := packet.Layer(layers.LayerTypeMPLS).(*layers.MPLS)
		// The CCM follows the 4 byte pseudowire control word.
		if !ok || len(mpls.Payload) < 4 {
			continue
		}
		eth, cfm, ok := ccmPDU(mpls.Payload[4:])
		if !ok {
			continue
		}
		ccmCount++
		meps[eth.SrcMAC.String()] = true

		// Verify CCM PDU destination is multicast.
		if !strings.HasPrefix(eth.DstMAC.String(), cfmMulticastPrefix) {
			return fmt.Errorf("destination MAC %s of CCM PDU is not a CFM multicast address", eth.DstMAC)
		}
		// Verify interval field in CCM packet.
		if got := cfm[2] & 0x07; got != expectedInterval {
			return fmt.Errorf("ccm interval mismatch; got: %d, want: %d", got, expectedInterval)
		}
		// Verify RDI bit (MSB of the flags octet) in CCM packet.
		if verifyRDIBit && cfm[2]&0x80 == 0 {
			return fmt.Errorf("rdi bit not set on CCM PDU from %s", eth.SrcMAC)
		}
		// TODO: Verify increasing sequence number for consecutive CCM packets
		// of a MEP (CCM sequence number in cfm[4:8]).
	}

	if ccmCount == 0 {
		return fmt.Errorf("no CCM PDUs with EtherType 0x8902 were found")
	}
	t.Logf("Validated %d CCM PDUs from %d MEPs: multicast destination, OpCode %d, interval %d", ccmCount, len(meps), CcmOpCode, expectedInterval)
	return nil
}

// captureAndValidateEncap fetches the full capture from each encapValidation
// port, validates the outer GRE header and the EthoCWoMPLSoGRE encapsulation of
// the CCM PDUs, and returns the captured packets for further validation.
//
// The captures are read in full instead of through
// packetvalidationhelpers.SourceObj, because its packet source only holds the
// packets buffered before its pcap handle was closed.
func captureAndValidateEncap(t *testing.T, ate *ondatra.ATEDevice) [][]gopacket.Packet {
	t.Helper()
	var captures [][]gopacket.Packet
	for _, v := range encapValidation {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, v); err != nil {
			t.Errorf("error: capture And ValidatePackets Failed (): %q", err)
		}
		capture, err := readCapture(t, ate, v.PortName)
		if err != nil {
			t.Errorf("error: could not read capture on %s: %v", v.PortName, err)
			continue
		}
		if err := validateCCMEncap(t, capture, v.PortName, v.MPLSLayer); err != nil {
			t.Errorf("error: CCM encapsulation validation failed on %s: %q", v.PortName, err)
		}
		captures = append(captures, capture)
	}
	return captures
}

// readCapture returns all the packets captured on portName.
func readCapture(t *testing.T, ate *ondatra.ATEDevice, portName string) ([]gopacket.Packet, error) {
	t.Helper()
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		return nil, fmt.Errorf("could not create temporary pcap file: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(portName))); err != nil {
		f.Close()
		return nil, fmt.Errorf("could not write bytes to pcap file: %v", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("could not close pcap file: %v", err)
	}
	handle, err := pcap.OpenOffline(f.Name())
	if err != nil {
		return nil, fmt.Errorf("could not open pcap file: %v", err)
	}
	defer handle.Close()

	var packets []gopacket.Packet
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for {
		packet, err := packetSource.NextPacket()
		if err == io.EOF {
			return packets, nil
		}
		if err != nil {
			return nil, fmt.Errorf("could not read pcap file: %v", err)
		}
		packets = append(packets, packet)
	}
}

// validateCCMEncap validates the MPLS label, traffic class and control word of
// every encapsulated CCM PDU in capture. Only CCM PDUs are validated because
// the capture also contains encapsulated customer data frames, which can carry
// a different traffic class than CCMs.
func validateCCMEncap(t *testing.T, capture []gopacket.Packet, portName string, want *packetvalidationhelpers.MPLSLayer) error {
	t.Helper()
	if want == nil {
		return fmt.Errorf("no expected MPLS layer to validate")
	}
	cwLen := 0
	if want.ControlWordHeader {
		cwLen = 4
	}
	ccmCount := 0
	for _, packet := range capture {
		mpls, ok := packet.Layer(layers.LayerTypeMPLS).(*layers.MPLS)
		if !ok || len(mpls.Payload) < cwLen || !isCCMFrame(mpls.Payload[cwLen:]) {
			continue
		}
		ccmCount++
		if mpls.Label != want.Label {
			return fmt.Errorf("mpls label of CCM PDU is not set properly. expected: %d, actual: %d", want.Label, mpls.Label)
		}
		if mpls.TrafficClass != want.Tc {
			return fmt.Errorf("mpls traffic class of CCM PDU is not set properly. expected: %d, actual: %d", want.Tc, mpls.TrafficClass)
		}
		// The first nibble of a pseudowire control word is 0 (RFC 4385).
		if want.ControlWordHeader && mpls.Payload[0]>>4 != 0 {
			return fmt.Errorf("control word not found between the MPLS label stack and the CCM PDU")
		}
	}
	if ccmCount == 0 {
		return fmt.Errorf("no EthoCWoMPLSoGRE-encapsulated CCM PDUs found")
	}
	t.Logf("All %d CCM PDUs on %s are EthoCWoMPLSoGRE-encapsulated with mpls label: %d, traffic class: %d", ccmCount, portName, want.Label, want.Tc)
	return nil
}

// isCCMFrame reports whether frame is an Ethernet frame, optionally 802.1Q
// tagged, that carries a CFM Continuity Check Message.
func isCCMFrame(frame []byte) bool {
	_, _, ok := ccmPDU(frame)
	return ok
}

func TestCFMBase(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dutTestData[0].dut = dut1

	dut2 := ondatra.DUT(t, "dut2")
	dutTestData[1].dut = dut2

	ate := ondatra.ATE(t, "ate")

	// Pass ocPFParams to configure dut
	configureDut(t)

	// Configure on OTG
	otgConfig := configureOTG(t, ate)
	for _, v := range encapValidation {
		packetvalidationhelpers.ConfigurePacketCapture(t, otgConfig, v)
	}

	ate.OTG().PushConfig(t, otgConfig)

	type testCase struct {
		name        string
		description string
		flow        otgconfighelpers.Flow
		mode        []string
		testFunc    func(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow)
	}
	testCases := []testCase{
		{
			name:        "CFM-1.1.2: Verify PF CFM establishment over EthoMPLSoGRE encapsulate",
			description: "Verify PF CFM establishment over EthoMPLSoGRE encapsulate",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Lags().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				PacketsToSend:     1000,
				PpsRate:           100,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "CFMFlow",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(dutTestData[0].subinterface)},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testCFMEstablishment,
			mode:     []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.3: Verify PF CFM packet integrity",
			description: "Verify PF CFM packet integrity",
			testFunc:    testCFMPacket,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.4: Verify RDI bit set for CCM PDUs on a CE-PE fault",
			description: "Verify RDI bit set for CCM PDUs on a CE-PE fault",
			testFunc:    testCFMAlarm,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.5: Verify RDI bit set for CCM PDUs on a PE-PE fault",
			description: "Verify RDI bit set for CCM PDUs on a PE-PE fault",
			testFunc:    testCFMAlarmOnPE,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.5.1: Verify TX actuation on a remote defect or a CCM timeout",
			description: "Verify RDI bit set for CCM PDUs on a PE-PE fault",
			testFunc:    testCFMAlarmOnPEPortMode,
			mode:        []string{"port"},
		},
		{
			name:        "CFM-1.1.6: Verify CFM Loss threshold can be configrued on DUT",
			description: "Verify CFM Loss threshold can be configrued on DUT",
			testFunc:    testCFMLossThreshold,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.8: Verify CFM Delay measurement",
			description: "Verify CFM Delay measurement",
			testFunc:    testCFMDelayMeasurement,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.9: Verify CFM synthetic loss measurement",
			description: "Verify CFM synthetic loss measurement",
			testFunc:    testCFMLossMeasurement,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.10: Verify CFM scale - attachment mode",
			description: "Verify CFM scale",
			testFunc:    testCFMScale,
			mode:        []string{"attachment"},
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		for _, mode := range tc.mode {
			for _, data := range dutTestData {
				configureIngressVlan(t, data.dut, data.lagAggID, data.subinterface, "remove")
				switch mode {
				case "port":
					controlWordMPLS.Tc = 1
					configureIngressVlan(t, data.dut, data.lagAggID, data.subinterface, "port")
					data.cfmCfg[0].IntfName = data.lagAggID
					cfgplugins.ConfigureCFMDomain(t, data.oam, data.dut, &data.cfmCfg[0])
				case "attachment":
					controlWordMPLS.Tc = 7
					configureIngressVlan(t, data.dut, data.lagAggID, data.subinterface, "attachment")
					data.cfmCfg[0].IntfName = fmt.Sprintf("%s.%v", data.lagAggID, data.subinterface)
					cfgplugins.ConfigureCFMDomain(t, data.oam, data.dut, &data.cfmCfg[0])
				}
			}

			t.Run(tc.name, func(t *testing.T) {
				t.Logf("Description: %s - %s", tc.name, mode)
				tc.testFunc(t, ate, ate.OTG(), otgConfig, tc.flow)
			})
		}
	}
}

func testCFMEstablishment(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	t.Log("CFM-1.1.2: Send IPv4 traffic with 64-1024 byte frames between the customer interfaces")
	createflow(otgConfig, &flow, true)
	ate.OTG().PushConfig(t, otgConfig)
	sendTrafficCapture(t, ate, otgConfig)

	t.Log("Verify CFM session is established with default timers on both DUTs")
	for _, data := range dutTestData {
		cfg := data.cfmCfg[0]
		cfg.Status = oc.OamCfm_OperationalStateType_ENABLED
		cfgplugins.ValidateCFMSession(t, data.dut, cfg)
		t.Log("Verify deadtimer at 3.5 times the configured keepalive")
		cfgplugins.ValidateDeadTimer(t, data.dut, cfg)
	}

	t.Log("Verify all CCM PDUs are EthoCWoMPLSoGRE-encapsulated")
	packetSource := captureAndValidateEncap(t, ate)
	t.Log("Verify CCM PDUs are not ECMP'd across all available tunnels")
	verifyLoadBalanceAcrossGre(t, packetSource)
}

// restoreDUT1CFM reverts the MD level and CCM interval of the DUT1 CFM domain.
// cfmCfg is a slice shared with dutTestData, so changes would otherwise leak
// into later tests.
func restoreDUT1CFM(t *testing.T) {
	t.Helper()
	cfg := &dutTestData[0].cfmCfg[0]
	cfg.RemoveDomain = true
	cfg.Level = 5
	cfg.Assocs[0].CcmInterval = oc.MaintenanceAssociation_CcmInterval_1S
	cfgplugins.ConfigureCFMDomain(t, dutTestData[0].oam, dutTestData[0].dut, cfg)
	cfg.RemoveDomain = false
}

func testCFMPacket(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	t.Log("CFM-1.1.3: Verify CCM PDU destination, OpCode and interval")
	sendTrafficCapture(t, ate, otgConfig)
	packetSource := captureAndValidateEncap(t, ate)

	interval := ccmIntervalMap[dutTestData[0].cfmCfg[0].Assocs[0].CcmInterval]
	for _, pkt := range packetSource {
		if err := validateCfmPacket(t, pkt, interval, false); err != nil {
			t.Errorf("error: validation of cfm packets failed: %q", err)
		}
	}

	t.Cleanup(func() { restoreDUT1CFM(t) })
	dut1Cfg := &dutTestData[0].cfmCfg[0]

	t.Log("CFM-1.1.3.1: Configure wrong MD level on DUT1 and verify the alarm on the endpoints")
	dut1Cfg.RemoveDomain = true
	dut1Cfg.Level = 4
	cfgplugins.ConfigureCFMDomain(t, dutTestData[0].oam, dutTestData[0].dut, dut1Cfg)
	validateAlarmOnEndpoints(t)

	t.Log("CFM-1.1.3.1: Revert the MD level and verify the alarm clears")
	restoreDUT1CFM(t)
	validateNoAlarms(t)

	t.Log("CFM-1.1.3.1: Configure a different CCM interval on DUT1 and verify the alarm on the endpoints")
	dut1Cfg.RemoveDomain = true
	dut1Cfg.Assocs[0].CcmInterval = oc.MaintenanceAssociation_CcmInterval_10S
	cfgplugins.ConfigureCFMDomain(t, dutTestData[0].oam, dutTestData[0].dut, dut1Cfg)
	validateAlarmOnEndpoints(t)
}

// validateAlarmOnEndpoints validates that both endpoints report a defect.
func validateAlarmOnEndpoints(t *testing.T) {
	t.Helper()
	for _, data := range dutTestData {
		cfgplugins.ValidateAlarmDetection(t, data.dut, data.cfmCfg[0])
	}
}

// validateNoAlarms validates that no endpoint reports a defect, so that a
// following fault check cannot pass on a defect left by a previous step.
func validateNoAlarms(t *testing.T) {
	t.Helper()
	for _, data := range dutTestData {
		cfgplugins.ValidateAlarmCleared(t, data.dut, data.cfmCfg[0], 4*cfmIntervalDuration(t, data.cfmCfg[0])+alarmMargin)
	}
}

// validateNoAlarmsWithin validates that no endpoint reports a defect within
// timeout and logs how long it took for the defects to clear.
func validateNoAlarmsWithin(t *testing.T, timeout time.Duration) {
	t.Helper()
	start := time.Now()
	for _, data := range dutTestData {
		cfgplugins.ValidateAlarmCleared(t, data.dut, data.cfmCfg[0], max(timeout-time.Since(start), time.Second))
		t.Logf("%s: defects cleared check finished after %v", data.dut.Name(), time.Since(start).Round(time.Second))
	}
}

// setATECustomerLinks sets the link state of the ATE ports of the DUT1
// customer interface. When the links are brought up, the OTG protocols are
// restarted so that the ATE LAG comes back up.
func setATECustomerLinks(t *testing.T, ate *ondatra.ATEDevice, state gosnappi.StatePortLinkStateEnum) {
	t.Helper()
	portStateAction := gosnappi.NewControlState()
	portStateAction.Port().Link().SetPortNames([]string{ate.Port(t, agg1.MemberPorts[0]).ID(), ate.Port(t, agg1.MemberPorts[1]).ID()}).SetState(state)
	ate.OTG().SetControlState(t, portStateAction)
	if state == gosnappi.StatePortLinkState.UP {
		ate.OTG().StartProtocols(t)
	}
}

// shutATECustomerLinks brings down the ATE ports of the DUT1 customer interface
// and registers a cleanup that brings them back up.
func shutATECustomerLinks(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	t.Cleanup(func() { setATECustomerLinks(t, ate, gosnappi.StatePortLinkState.UP) })
	setATECustomerLinks(t, ate, gosnappi.StatePortLinkState.DOWN)
}

// setDUTPortEnabled sets only the enabled leaf of a DUT port. Unlike
// cfgplugins.ToggleInterface, which replaces the whole interface, it preserves
// the IP addresses of the transit port that carry the GRE tunnel.
func setDUTPortEnabled(t *testing.T, dut *ondatra.DUTDevice, port string, enabled bool) {
	t.Helper()
	gnmi.Replace(t, dut, gnmi.OC().Interface(port).Enabled().Config(), enabled)
}

// shutDUT1TransitPort shuts down DUT1 port 2 and registers a cleanup that
// re-enables it.
func shutDUT1TransitPort(t *testing.T) string {
	t.Helper()
	dutPort := dutTestData[0].dut.Port(t, dutTestData[0].transitPort[0]).Name()
	t.Cleanup(func() { setDUTPortEnabled(t, dutTestData[0].dut, dutPort, true) })
	setDUTPortEnabled(t, dutTestData[0].dut, dutPort, false)
	return dutPort
}

// testCFMAlarm verifies RDI flag set on CE-PE fault.
func testCFMAlarm(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	restoreDUT1CFM(t)
	t.Log("Verify no defect is present before inducing the fault")
	validateNoAlarms(t)

	t.Log("CFM-1.1.4: Shut down ATE port 1 to simulate a CE-PE fault")
	shutATECustomerLinks(t, ate)

	sendTrafficCapture(t, ate, otgConfig)
	packetSource := captureAndValidateEncap(t, ate)

	interval := ccmIntervalMap[dutTestData[0].cfmCfg[0].Assocs[0].CcmInterval]
	for _, pkt := range packetSource {
		if err := validateCfmPacket(t, pkt, interval, false); err != nil {
			t.Errorf("error: validation of cfm packets failed: %q", err)
		}
	}

	t.Log("Verify a Remote Defect Detected alarm is raised at the remote end-point")
	cfgplugins.ValidateAlarmDetection(t, dutTestData[1].dut, dutTestData[1].cfmCfg[0])
}

func testCFMAlarmOnPE(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	t.Log("Verify no defect is present before inducing the fault")
	validateNoAlarms(t)

	t.Log("CFM-1.1.5: Shut down DUT1 port 2 to simulate a PE-PE fault")
	shutDUT1TransitPort(t)
	sendTrafficCapture(t, ate, otgConfig)

	t.Log("Verify both DUT1 and DUT2 observe a CCM timeout and emit CCMs with the RDI flag set")
	cfgplugins.ValidateAlarmDetection(t, dutTestData[0].dut, dutTestData[0].cfmCfg[0])
	cfgplugins.ValidateAlarmDetection(t, dutTestData[1].dut, dutTestData[1].cfmCfg[0])
}

func testCFMAlarmOnPEPortMode(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	t.Log("Verify no defect is present before inducing the fault")
	validateNoAlarms(t)

	t.Log("CFM-1.1.5.1: Shut down ATE port 1 and verify the remote defect on DUT2")
	shutATECustomerLinks(t, ate)
	cfgplugins.ValidateAlarmDetection(t, dutTestData[1].dut, dutTestData[1].cfmCfg[0])

	t.Log("Revert TX on ATE port 1 and verify the remote defect clears on both DUTs")
	setATECustomerLinks(t, ate, gosnappi.StatePortLinkState.UP)
	validateNoAlarmsWithin(t, alarmClearAfterLinkRestoreTimeout)

	t.Log("Shut down DUT1 port 2 and verify the CCM timeout on DUT1")
	shutDUT1TransitPort(t)
	sendTrafficCapture(t, ate, otgConfig)
	cfgplugins.ValidateAlarmDetection(t, dutTestData[0].dut, dutTestData[0].cfmCfg[0])
}

// cfmIntervalDuration returns the CCM interval of cfg.
func cfmIntervalDuration(t *testing.T, cfg cfgplugins.MaintenanceDomainConfig) time.Duration {
	t.Helper()
	if len(cfg.Assocs) == 0 {
		t.Fatalf("CFM maintenance domain %q: Assocs must not be empty", cfg.DomainName)
	}
	switch interval := cfg.Assocs[0].CcmInterval; interval {
	case oc.MaintenanceAssociation_CcmInterval_300MS:
		return 300 * time.Millisecond
	case oc.MaintenanceAssociation_CcmInterval_1S:
		return time.Second
	case oc.MaintenanceAssociation_CcmInterval_10S:
		return 10 * time.Second
	default:
		t.Fatalf("CFM maintenance domain %q: unsupported CCM interval %v", cfg.DomainName, interval)
		return 0
	}
}

func testCFMLossThreshold(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	dut1 := dutTestData[0]
	interval := cfmIntervalDuration(t, dut1.cfmCfg[0])
	dutPort := dut1.dut.Port(t, dut1.transitPort[0]).Name()

	t.Cleanup(func() {
		setDUTPortEnabled(t, dut1.dut, dutPort, true)
		cfgplugins.ConfigureLossThreshold(t, dut1.dut, dut1.oam, dut1.cfmCfg[0], 3.5)
	})

	t.Log("Verify no defect is present before inducing the fault")
	validateNoAlarms(t)

	// TODO: Add the 255 loss threshold once supported; Arista rejects a
	// continuity-check timeout multiplier of 255.
	for _, loss := range []float64{6, 10, 20, 100} {
		t.Run(fmt.Sprintf("loss-threshold-%v", loss), func(t *testing.T) {
			t.Logf("CFM-1.1.6: Set the loss threshold knob to %v", loss)
			cfgplugins.ConfigureLossThreshold(t, dut1.dut, dut1.oam, dut1.cfmCfg[0], loss)
			lossTimeout := time.Duration(loss*float64(interval)) + alarmMargin

			t.Log("Shut down DUT1 port 2 and verify the CCM timeout is detected after the loss threshold")
			setDUTPortEnabled(t, dut1.dut, dutPort, false)
			cfgplugins.ValidateAlarmDetectionWithin(t, dut1.dut, dut1.cfmCfg[0], lossTimeout)

			t.Log("Re-enable DUT1 port 2 and verify the alarm clears")
			setDUTPortEnabled(t, dut1.dut, dutPort, true)
			b := &gnmi.SetBatch{}
			cfgplugins.NewAggregateInterface(t, dut1.dut, b, transitLagData[0])
			b.Set(t, dut1.dut)
			cfgplugins.ValidateAlarmCleared(t, dut1.dut, dut1.cfmCfg[0], 4*interval+alarmMargin)
		})
	}
}

// measurementProfile returns the CFM measurement profile with 60 second
// measurement intervals for measurementType.
func measurementProfile(measurementType oc.E_PmProfile_MeasurementType) cfgplugins.CFMMeasurementProfile {
	return cfgplugins.CFMMeasurementProfile{
		ProfileName:                cfmProfileName,
		BurstInterval:              100,
		IntervalsArchived:          5,
		MeasurementInterval:        10,
		MeasurementType:            measurementType,
		PacketPerBurst:             100,
		PacketsPerMeaurementPeriod: 60,
		RepetitionPeriod:           1,
	}
}

// configureMeasurementProfiles configures the measurement profile on both DUTs
// and returns the earliest DUT time from which new measurements are expected.
func configureMeasurementProfiles(t *testing.T, profile cfgplugins.CFMMeasurementProfile) time.Time {
	t.Helper()
	since := helpers.GetRouterTime(t, dutTestData[0].dut)
	if s := helpers.GetRouterTime(t, dutTestData[1].dut); s.Before(since) {
		since = s
	}
	for _, data := range dutTestData {
		b := &gnmi.SetBatch{}
		if oam := cfgplugins.ConfigureMeasurementProfile(t, b, data.dut, profile); oam != nil {
			b.Set(t, data.dut)
		}
	}
	return since
}

func testCFMDelayMeasurement(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	t.Log("CFM-1.1.8: Configure a 2-way DM measurement profile with 60 second measurement intervals")
	dmProfile := measurementProfile(oc.PmProfile_MeasurementType_DMM)
	dmProfile.MeasurementInterval = dmTxIntervalMillis
	since := configureMeasurementProfiles(t, dmProfile)

	t.Log("Verify DUT1 and DUT2 observe min, max and mean delay measurements")
	for _, data := range dutTestData {
		cfgplugins.ValidateMeasurementProfile(t, data.dut, cfmProfileName)
		cfgplugins.ValidateDelayMeasurement(t, data.dut, data.cfmCfg[0], since)
	}
}

func testCFMLossMeasurement(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	t.Log("CFM-1.1.9: Configure a synthetic loss measurement profile with 60 second measurement intervals")
	for _, data := range dutTestData {
		cfgplugins.ConfigureLossThreshold(t, data.dut, data.oam, data.cfmCfg[0], 3.5)
	}
	since := configureMeasurementProfiles(t, measurementProfile(oc.PmProfile_MeasurementType_SLM))

	sendTrafficCapture(t, ate, otgConfig)

	t.Log("Verify DUT1 and DUT2 observe min, max and mean far-end and near-end frame loss ratios")
	cfgplugins.ValidateLossMeasurement(t, []*ondatra.DUTDevice{dutTestData[0].dut, dutTestData[1].dut}, []cfgplugins.MaintenanceDomainConfig{dutTestData[0].cfmCfg[0], dutTestData[1].cfmCfg[0]}, since)
}

// isTopologySubinterface reports whether the scale subinterface index i maps
// to a subnet already used by the topology: the customer subinterfaces
// (192.168.10.0/24, 192.168.30.0/24) and the transit link (192.168.20.0/24).
func isTopologySubinterface(i int) bool {
	return i == 0 || i == 10 || i == 20
}

func testCFMScale(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	var scaleParams []cfgplugins.CFMScaleParams

	t.Log("CFM-1.1.10: Configure 1000 attachments, pseudowires and CFM sessions")
	for index, data := range dutTestData {
		base := *custLagData[index].SubInterfaces[0]
		dutIPs, err := iputil.GenerateIPsWithStep(base.IPv4Address.String(), scaleSessionCount, "0.0.1.0")
		if err != nil {
			t.Fatalf("failed to generate DUT IPs: %v", err)
		}

		b := &gnmi.SetBatch{}
		var subIntfs []int
		for i, ip := range dutIPs {
			if isTopologySubinterface(i) {
				continue
			}
			sub := base
			sub.VlanID = i
			sub.IPv4Address = net.ParseIP(ip)
			cfgplugins.AddSubInterface(t, data.dut, b, &oc.Interface{Name: ygot.String(data.lagAggID)}, &sub)
			subIntfs = append(subIntfs, i)
		}

		params := cfgplugins.CFMScaleParams{
			IntfName:         data.lagAggID,
			Subinterfaces:    subIntfs,
			PseudowireName:   pseudowireName,
			NexthopGroupName: nexthopGroupName,
			LocalLabelBase:   localLabel,
			RemoteLabelBase:  remoteLabel,
			DomainPrefix:     "D.",
			CFM:              data.cfmCfg[0],
		}
		dut, lag := data.dut, data.lagAggID
		t.Cleanup(func() {
			t.Logf("Remove the CFM scale configuration from %s", dut.Name())
			if rb := cfgplugins.RemoveCFMScale(t, dut, &gnmi.SetBatch{}, params); rb != nil {
				rb.Set(t, dut)
			}
			db := &gnmi.SetBatch{}
			for _, i := range subIntfs {
				gnmi.BatchDelete(db, gnmi.OC().Interface(lag).Subinterface(uint32(i)).Config())
			}
			db.Set(t, dut)
		})

		b.Set(t, data.dut)
		if sb := cfgplugins.ConfigureCFMScale(t, data.dut, &gnmi.SetBatch{}, params); sb != nil {
			sb.Set(t, data.dut)
		}
		scaleParams = append(scaleParams, params)
	}

	t.Log("Verify all CFM sessions are established")
	for index, data := range dutTestData {
		cfgs := append([]cfgplugins.MaintenanceDomainConfig{data.cfmCfg[0]}, scaleParams[index].DomainConfigs()...)
		for _, cfg := range cfgs {
			cfg.Status = oc.OamCfm_OperationalStateType_ENABLED
			cfgplugins.ValidateCFMSession(t, data.dut, cfg)
		}
	}
	// TODO: Verify device health and soak for at least 6 hours.
}
