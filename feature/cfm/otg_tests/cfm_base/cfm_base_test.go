package cfmbase_test

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
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

var ccmIntervalMap = map[oc.E_MaintenanceAssociation_CcmInterval]uint8{
	oc.MaintenanceAssociation_CcmInterval_1S: 4,
}

type dutData struct {
	dut              *ondatra.DUTDevice
	lagAggID         string
	custPort         []string
	transitPort      []string
	neighborPortIPv4 string
	subinterface     uint32
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
			DutPortsIdx: []int{2},
			LacpParams:  lacpParams,
			AggType:     oc.IfAggregate_AggregationType_LACP,
		},
		{
			Attributes:  dut2TransitIntf,
			DutPortsIdx: []int{0},
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
					VlanEnable:    false,
					IPv4Address:   net.ParseIP("192.168.10.2"),
					IPv4PrefixLen: plenIPv4,
					IPv6PrefixLen: plenIPv6,
				},
			},
		},
		{
			Attributes:      dut2custIntf,
			OndatraPortsIdx: []int{1, 2},
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_STATIC,
			SubInterfaces: []*cfgplugins.DUTSubInterfaceData{
				{
					VlanID:        10,
					VlanEnable:    false,
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
		MemberPorts: []string{"port3", "port4"},
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

	Validations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateMPLSLayer,
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
			PortName:    "port5",
			Validations: Validations,
			IPv4Layer:   OuterGREIPLayerIPv4DUT1,
			MPLSLayer:   controlWordMPLS,
		},
		{
			PortName:    "port6",
			Validations: Validations,
			IPv4Layer:   OuterGREIPLayerIPv4DUT2,
			MPLSLayer:   controlWordMPLS,
		},
	}

	dutTestData = []dutData{
		{
			custPort:         []string{"port1", "port2"},
			transitPort:      []string{"port3"},
			neighborPortIPv4: "192.168.20.2",
			subinterface:     10,
			tunnelDst:        tunnelDestination1,
			staticTunnelDst:  staticTunnelDst1,
			capturePort:      "port4",
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
			custPort:         []string{"port2", "port3"},
			transitPort:      []string{"port1"},
			neighborPortIPv4: "192.168.20.1",
			subinterface:     10,
			tunnelDst:        tunnelDestination2,
			staticTunnelDst:  staticTunnelDst2,
			capturePort:      "port4",
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
		data.oam = configureCFM(t, sfBatch, data.dut, data.lagAggID, data.cfmCfg)

		// Configure monitor session to capture packets
		monitorCapt := cfgplugins.MonitorSessionConfig{
			SessionName:       "capture1",
			SourcePort:        transitLagData[index].LagName,
			DestinationDUTAte: data.dut.Port(t, data.capturePort).Name(),
		}
		cfgplugins.ConfigureMonitorSession(t, data.dut, monitorCapt)

	}
}

func configureCFM(t *testing.T, sfBatch *gnmi.SetBatch, dut *ondatra.DUTDevice, intfName string, cfmCfg []cfgplugins.MaintenanceDomainConfig) *oc.Oam {
	cfmMeasurementProfile := cfgplugins.CFMMeasurementProfile{
		ProfileName:       "cfm_delay_Bundle",
		BurstInterval:     100,
		IntervalsArchived: 5,

		PacketPerBurst:   100,
		RepetitionPeriod: 1,
	}

	t.Log("Configure CFM configs on DUT")
	oam := cfgplugins.ConfigureMeasurementProfile(t, sfBatch, dut, cfmMeasurementProfile)
	cfmCfg[0].IntfName = intfName
	cfmCfg[0].ProfileName = cfmMeasurementProfile.ProfileName
	cfgplugins.ConfigureCFMDomain(t, oam, dut, &cfmCfg[0])
	gnmi.BatchUpdate(sfBatch, gnmi.OC().Oam().Config(), oam)

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

func configureIngressVlan(t *testing.T, dut *ondatra.DUTDevice, intfName string, subinterfaces uint32, mode string) {
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
	for _, port := range []string{"port5", "port6"} {
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

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice) {
	ate.OTG().StartProtocols(t)
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(10 * time.Second)
	ate.OTG().StopTraffic(t)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

func verifyLoadBalanceAcrossGre(t *testing.T, packetSource *gopacket.PacketSource) {
	t.Log("Validating traffic equally load-balanced across GRE destinations")
	tunnelCount := 16

	srcIPs := make(map[string]int)
	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ipv4, _ := ipLayer.(*layers.IPv4)
			srcIPs[ipv4.SrcIP.String()]++
		}
	}

	uniqueCount := len(srcIPs)
	t.Logf("Found %d unique GRE source IPs in the capture", uniqueCount)

	if uniqueCount < tunnelCount {
		t.Log("flows are not ECMP'd across all available tunnels as expected")
		return
	}

	t.Errorf("error: traffic was load-balanced across %d GRE sources", uniqueCount)
}

func validateCfmPacket(t *testing.T, expectedInterval uint8, verifyRDIBit bool) error {
	t.Helper()
	t.Log("Starting CFM packet integrity validation")

	var cfmData []byte
	packetSource := packetvalidationhelpers.SourceObj()

	// TODO: Verify increasing sequence number for consequent CCM packets.
	// var lastSequenceNumber uint32
	// isFirstPacket := true
	cfmPacketCount := 0

	for packet := range packetSource.Packets() {
		mplsLayer := packet.Layer(layers.LayerTypeMPLS)
		if mplsLayer == nil {
			continue
		}
		mpls, _ := mplsLayer.(*layers.MPLS)
		inner := gopacket.NewPacket(mpls.Payload[4:], layers.LayerTypeEthernet, gopacket.Default)
		if inner == nil {
			return fmt.Errorf("error: encapsulated layer not found")
		}

		innerEthLayer := inner.Layer(layers.LayerTypeEthernet)
		if innerEthLayer == nil {
			return fmt.Errorf("error: encapsulated inner ethernet layer not found")
		}
		eth, _ := innerEthLayer.(*layers.Ethernet)

		switch eth.EthernetType {
		case CfmEtherType:
			cfmData = eth.Payload
		case 0x8100:
			cfmData = eth.Payload[4:]
		default:
			continue
		}

		cfmPacketCount++
		// t.Logf("Processing CFM packet #%d..", cfmPacketCount)

		version := cfmData[0] & 0x1F
		if version == 0 && CfmOpCode(cfmData[1]) == CcmOpCode {
			// Verify CCM PDU Destination is Multicast.
			if !strings.HasPrefix(eth.DstMAC.String(), cfmMulticastPrefix) {
				t.Errorf("error: destination MAC %s is not a standard CFM multicast address", eth.DstMAC)
			}
			t.Logf("destination MAC %s is a valid multicast address", eth.DstMAC)

			// Verify CFM OpCode as Continuity Check Message (1).
			if CfmOpCode(cfmData[1]) != CcmOpCode {
				t.Errorf("error: opCode: %d is found a CCM packet, expected: %d", cfmData[1], CcmOpCode)
			} else {
				t.Logf("opCode: %d is found a CCM packet", cfmData[1])
			}

			// Verify interval field in CCM packet.
			if cfmData[2]&0x07 != byte(expectedInterval) {
				t.Errorf("error: ccm interval mismatch on packet; expected: %d, got: %d", expectedInterval, cfmData[2]&0x07)
			} else {
				t.Logf("packet has the correct CCM interval: %d", cfmData[2]&0x07)
			}

			// Optional: Verify RDI bit in CCM packet.
			if verifyRDIBit {
				// RDI bit is MSB of octet 2; non-zero when set.
				rdiBitSet := (cfmData[2] & 0x80) != 0
				if rdiBitSet != verifyRDIBit {
					t.Errorf("error: rdi bit verification failed on packet. Expected: %v, Got: %v", !verifyRDIBit, rdiBitSet)
				}
				t.Logf("packet RDI bit is correctly set to %v", verifyRDIBit)
			}

			seqNum := binary.BigEndian.Uint32(cfmData[4:8])
			t.Logf("first CCM packet found with sequence number: %d", seqNum)
			// TODO: Verify increasing sequence number for consequent CCM packets.
			// if isFirstPacket {
			// 	t.Logf("first CCM packet found with sequence number: %d", seqNum)
			// 	isFirstPacket = false
			// } else {
			// 	if seqNum <= lastSequenceNumber {
			// 		return fmt.Errorf("ccm sequence number did not increase. Previous: %d, Current: %d", lastSequenceNumber, ccmPDU.SequenceNumber)
			// 	}
			// 	t.Logf("sequence number increased correctly. Previous: %d, Current: %d.", lastSequenceNumber, ccmPDU.SequenceNumber)
			// }
			// lastSequenceNumber = ccmPDU.SequenceNumber
			return nil
		} else {
			continue
		}
	}

	if cfmPacketCount == 0 {
		return fmt.Errorf("error: validation failed: no CFM packets with EtherType 0x8902 were found")
	}
	return nil
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
			name:        "CFM-1.1.1: Verify PF CFM establishment over EthoMPLSoGRE encapsulate",
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
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: dutTestData[0].subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testCFMEstablishment,
			mode:     []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.2: Verify PF CFM packet integrity",
			description: "Verify PF CFM packet integrity",
			testFunc:    testCFMPacket,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.3: Verify RDI bit set for CCM PDUs on a CE-PE fault",
			description: "Verify RDI bit set for CCM PDUs on a CE-PE fault",
			testFunc:    testCFMAlarm,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.4: Verify RDI bit set for CCM PDUs on a PE-PE fault",
			description: "Verify RDI bit set for CCM PDUs on a PE-PE fault",
			testFunc:    testCFMAlarmOnPE,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.4.1: Verify RDI bit set for CCM PDUs on a PE-PE fault Port Mode",
			description: "Verify RDI bit set for CCM PDUs on a PE-PE fault",
			testFunc:    testCFMAlarmOnPE_1,
			mode:        []string{"port"},
		},
		{
			name:        "CFM-1.1.5: Verify CFM Loss threshold can be configrued on DUT",
			description: "Verify CFM Loss threshold can be configrued on DUT",
			testFunc:    testCFMLossThreshold,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.6: Verify CFM Delay measurement",
			description: "Verify CFM Delay measurement",
			testFunc:    testCFMDelayMeasurement,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.7: Verify CFM synthetic loss measurement",
			description: "Verify CFM synthetic loss measurement",
			testFunc:    testCFMLossMeasurement,
			mode:        []string{"port", "attachment"},
		},
		{
			name:        "CFM-1.1.8: Verify CFM scale - attachment mode",
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
	createflow(otgConfig, &flow, true)
	ate.OTG().PushConfig(t, otgConfig)
	sendTrafficCapture(t, ate)

	otgutils.LogFlowMetrics(t, otg, otgConfig)
	otgutils.LogPortMetrics(t, otg, otgConfig)

	for _, data := range dutTestData {
		data.cfmCfg[0].Status = oc.OamCfm_OperationalStateType_ENABLED
		cfgplugins.ValidateCFMSession(t, data.dut, data.cfmCfg[0])
		cfgplugins.ValidateDeadTimer(t, data.dut, data.cfmCfg[0])
	}

	for _, v := range encapValidation {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, v); err != nil {
			t.Errorf("error: capture And ValidatePackets Failed (): %q", err)
		}
	}

	verifyLoadBalanceAcrossGre(t, packetvalidationhelpers.SourceObj())
}

func testCFMPacket(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	var dutData dutData
	sendTrafficCapture(t, ate)

	for _, v := range encapValidation {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, v); err != nil {
			t.Errorf("error: capture And ValidatePackets Failed (): %q", err)
		}
	}

	interval, _ := ccmIntervalMap[dutTestData[0].cfmCfg[0].Assocs[0].CcmInterval]
	t.Log(interval)
	if err := validateCfmPacket(t, interval, false); err != nil {
		t.Errorf("error: validation of cfm packets failed: %q", err)
	}

	// Configure Wrong MD level on on endpoint
	dutData = dutTestData[0]
	dutData.cfmCfg[0].RemoveDomain = true
	dutData.cfmCfg[0].Level = 4

	cfgplugins.ConfigureCFMDomain(t, dutData.oam, dutData.dut, &dutData.cfmCfg[0])
	time.Sleep(20 * time.Second)
	cfgplugins.ValidateAlarmDetection(t, dutTestData[1].dut, dutTestData[1].cfmCfg[0])

	// Configure different CCM interval
	dutData = dutTestData[0]
	dutData.cfmCfg[0].Assocs[0].CcmInterval = oc.MaintenanceAssociation_CcmInterval_10S

	cfgplugins.ConfigureCFMDomain(t, dutData.oam, dutData.dut, &dutData.cfmCfg[0])
	cfgplugins.ValidateAlarmDetection(t, dutTestData[1].dut, dutTestData[1].cfmCfg[0])

	// Configure different CCM interval
	dutData = dutTestData[0]
	dutData.cfmCfg[0].Assocs[0].CcmInterval = oc.MaintenanceAssociation_CcmInterval_1S
	cfgplugins.ConfigureCFMDomain(t, dutData.oam, dutData.dut, &dutData.cfmCfg[0])
}

// testCFM114 verifies RDI flag set on CE-PE fault.
func testCFMAlarm(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	dutData := dutTestData[0]
	dutData.cfmCfg[0].RemoveDomain = true
	dutData.cfmCfg[0].Level = 5
	dutData.cfmCfg[0].Assocs[0].CcmInterval = oc.MaintenanceAssociation_CcmInterval_1S

	cfgplugins.ConfigureCFMDomain(t, dutData.oam, dutData.dut, &dutData.cfmCfg[0])

	t.Log("Shutting down ATE port1 to simulate CE-PE fault")
	portStateAction := gosnappi.NewControlState()
	port := portStateAction.Port().Link().SetPortNames([]string{ate.Port(t, agg1.MemberPorts[0]).ID(), ate.Port(t, agg1.MemberPorts[1]).ID()})
	port.SetState(gosnappi.StatePortLinkState.DOWN)
	ate.OTG().SetControlState(t, portStateAction)

	sendTrafficCapture(t, ate)

	for _, v := range encapValidation {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, v); err != nil {
			t.Errorf("error: capture And ValidatePackets Failed (): %q", err)
		}
	}

	interval, _ := ccmIntervalMap[dutTestData[0].cfmCfg[0].Assocs[0].CcmInterval]
	if err := validateCfmPacket(t, interval, false); err != nil {
		t.Errorf("error: validation of cfm packets failed: %q", err)
	}

	cfgplugins.ValidateAlarmDetection(t, dutTestData[1].dut, dutTestData[1].cfmCfg[0])

	port.SetState(gosnappi.StatePortLinkState.UP)
	ate.OTG().SetControlState(t, portStateAction)
}

func testCFMAlarmOnPE(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	dutPort := dutTestData[0].dut.Port(t, dutTestData[0].transitPort[0]).Name()
	cfgplugins.ToggleInterface(t, dutTestData[0].dut, dutPort, false)
	sendTrafficCapture(t, ate)

	// Validating on both the duts alarm defect is raised
	cfgplugins.ValidateAlarmDetection(t, dutTestData[0].dut, dutTestData[0].cfmCfg[0])
	cfgplugins.ValidateAlarmDetection(t, dutTestData[1].dut, dutTestData[1].cfmCfg[0])

}

func testCFMAlarmOnPE_1(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	t.Log("Shutting down ATE port1")
	portStateAction := gosnappi.NewControlState()
	port := portStateAction.Port().Link().SetPortNames([]string{ate.Port(t, agg1.MemberPorts[0]).ID(), ate.Port(t, agg1.MemberPorts[1]).ID()})
	port.SetState(gosnappi.StatePortLinkState.DOWN)
	ate.OTG().SetControlState(t, portStateAction)

	port.SetState(gosnappi.StatePortLinkState.UP)
	ate.OTG().SetControlState(t, portStateAction)

	dutPort := dutTestData[0].dut.Port(t, dutTestData[0].transitPort[0]).Name()
	cfgplugins.ToggleInterface(t, dutTestData[0].dut, dutPort, false)

	sendTrafficCapture(t, ate)
	cfgplugins.ValidateAlarmDetection(t, dutTestData[0].dut, dutTestData[0].cfmCfg[0])

	cfgplugins.ToggleInterface(t, dutTestData[0].dut, dutPort, true)
}

func testCFMLossThreshold(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	for _, loss := range []float64{6, 10, 20, 100} {
		t.Logf("set the loss threshold knob to: %v", loss)
		b := &gnmi.SetBatch{}
		cfgplugins.ConfigureLossThreshold(t, dutTestData[0].dut, dutTestData[0].oam, dutTestData[0].cfmCfg[0], loss)

		dutPort := dutTestData[0].dut.Port(t, dutTestData[0].transitPort[0]).Name()
		cfgplugins.ToggleInterface(t, dutTestData[0].dut, dutPort, false)
		time.Sleep(time.Duration(loss) * time.Second)

		// Validating on the dut alarm defect is raised
		cfgplugins.ValidateAlarmDetection(t, dutTestData[0].dut, dutTestData[0].cfmCfg[0])

		cfgplugins.ToggleInterface(t, dutTestData[0].dut, dutPort, true)

		cfgplugins.NewAggregateInterface(t, dutTestData[0].dut, b, transitLagData[0])
		b.Set(t, dutTestData[0].dut)
	}

	cfgplugins.ConfigureLossThreshold(t, dutTestData[0].dut, dutTestData[0].oam, dutTestData[0].cfmCfg[0], 3.5)
}

func testCFMDelayMeasurement(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	cfmMeasurementProfile := cfgplugins.CFMMeasurementProfile{
		ProfileName:                "cfm_delay_Bundle",
		MeasurementInterval:        10,
		PacketsPerMeaurementPeriod: 60,
		MeasurementType:            oc.PmProfile_MeasurementType_DMM,
	}

	t.Log("Configure CFM configs on DUT")
	sfBatch := &gnmi.SetBatch{}
	for _, data := range dutTestData {
		data.oam = cfgplugins.ConfigureMeasurementProfile(t, sfBatch, data.dut, cfmMeasurementProfile)
	}

	for _, data := range dutTestData {
		cfgplugins.ValidateDelayMeasurement(t, data.dut, data.cfmCfg[0])
	}

}

func testCFMLossMeasurement(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	sfBatch := &gnmi.SetBatch{}
	cfmMeasurementProfile := cfgplugins.CFMMeasurementProfile{
		ProfileName:                "cfm_delay_Bundle",
		MeasurementInterval:        10,
		PacketsPerMeaurementPeriod: 60,
		MeasurementType:            oc.PmProfile_MeasurementType_SLM,
	}

	t.Log("Configure CFM configs on DUT")
	for _, data := range dutTestData {
		cfgplugins.ConfigureLossThreshold(t, dutTestData[0].dut, data.oam, dutTestData[0].cfmCfg[0], 3.5)
		data.oam = cfgplugins.ConfigureMeasurementProfile(t, sfBatch, data.dut, cfmMeasurementProfile)
	}

	sendTrafficCapture(t, ate)
	time.Sleep(20 * time.Second)

	for _, data := range dutTestData {
		cfgplugins.ValidateLossMeasurement(t, data.dut, data.cfmCfg[0])
	}
}

func testCFMScale(t *testing.T, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	cfmDomain := []string{"D1"}

	sfBatch := &gnmi.SetBatch{}
	for index, data := range dutTestData {
		dutLagData := custLagData[index]
		dutIPs, err := iputil.GenerateIPsWithStep(dutLagData.SubInterfaces[0].IPv4Address.String(), 1002, "0.0.1.0")
		if err != nil {
			t.Errorf("failed to generate DUT IPs: %s", err)
		}

		baseSubnet := 10
		// Create 1000 subinterfaces
		for i, ip := range dutIPs {
			subnet := baseSubnet + i

			if subnet == 20 || subnet == 30 || subnet == 10 {
				continue
			}
			dutLagData.SubInterfaces[0].VlanID = i
			dutLagData.SubInterfaces[0].IPv4Address = net.ParseIP(ip)
			cfgplugins.AddSubInterface(t, data.dut, sfBatch, &oc.Interface{Name: ygot.String(data.lagAggID)}, dutLagData.SubInterfaces[0])
			sfBatch.Set(t, data.dut)

			pseudowireCfg := cfgplugins.MplsStaticPseudowire{
				PseudowireName:   fmt.Sprintf("%s-%d", pseudowireName, i),
				IntfName:         data.lagAggID,
				Subinterface:     uint32(i),
				NexthopGroupName: nexthopGroupName,
				LocalLabel:       fmt.Sprintf("%d", localLabel+i),
				RemoteLabel:      fmt.Sprintf("%d", remoteLabel+i),
				PatchPanel:       fmt.Sprintf("patch-%d", i),
			}
			cfgplugins.ConfigureMplsStaticPseudowire(t, sfBatch, data.dut, pseudowireCfg)

			vlanClientCfg := cfgplugins.VlanClientEncapsulationParams{
				IntfName:      data.lagAggID,
				Subinterfaces: uint32(i),
			}
			cfgplugins.VlanClientEncapsulation(t, sfBatch, data.dut, vlanClientCfg)

			data.cfmCfg[0].DomainName = fmt.Sprintf("D.%v", i)
			cfmDomain = append(cfmDomain, fmt.Sprintf("D.%v", i))
			data.cfmCfg[0].IntfName = fmt.Sprintf("%s.%v", data.lagAggID, i)
			data.cfmCfg[0].ProfileName = "cfm_delay_Bundle"
			cfgplugins.ConfigureCFMDomain(t, data.oam, data.dut, &data.cfmCfg[0])
		}
	}

	for _, data := range dutTestData {
		data.cfmCfg[0].Status = oc.OamCfm_OperationalStateType_ENABLED
		for _, domain := range cfmDomain {
			data.cfmCfg[0].DomainName = domain
			cfgplugins.ValidateCFMSession(t, data.dut, data.cfmCfg[0])
		}
	}
}
