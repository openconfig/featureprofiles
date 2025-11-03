package helper

import (
	"fmt"
	"testing"

	"github.com/google/gopacket/layers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
)

type sflowHelper struct{}

type IPType string

const (
	IPv4 = "IPv4"
	IPv6 = "IPv6"
)

type SflowSample struct {
	InputInterface  string
	OutputInterface string
	RawPktHdr       *SfRecordRawPacketHeader
	ExtdRtrData     *SfRecordExtendedRouterData
	ExtdGtwData     *SfRecordExtendedGatewayData
}

// AddRawPacketHeader adds a new empty SfRecordRawPacketHeader to the SflowSample and returns it
func (s *SflowSample) AddRawPacketHeader() *SfRecordRawPacketHeader {
	s.RawPktHdr = &SfRecordRawPacketHeader{}
	return s.RawPktHdr
}

// AddExtendedRouterData adds a new empty SfRecordExtendedRouterData to the SflowSample and returns it
func (s *SflowSample) AddExtendedRouterData() *SfRecordExtendedRouterData {
	s.ExtdRtrData = &SfRecordExtendedRouterData{}
	return s.ExtdRtrData
}

// AddExtendedGatewayData adds a new empty SfRecordExtendedGatewayData to the SflowSample and returns it
func (s *SflowSample) AddExtendedGatewayData() *SfRecordExtendedGatewayData {
	s.ExtdGtwData = &SfRecordExtendedGatewayData{}
	return s.ExtdGtwData
}

// flowConfig and IPType are provided in the original prompt
type SflowAttr struct {
	PacketsToSend   uint32
	PpsRate         uint64
	SflowDscp       uint8
	SamplingRate    uint
	SampleTolerance float32
	IP              IPType
	InputInterface  []uint32
	OutputInterface []uint32
	SfSample        *SflowSample
}

// NewSflowAttr creates a new instance of SflowAttr with default values
func NewSflowAttr() *SflowAttr {
	return &SflowAttr{
		SfSample: &SflowSample{},
	}
}

// AddSflowSample adds a new SflowSample to the SflowAttr and returns it
func (s *SflowAttr) AddSflowSample() *SflowSample {
	s.SfSample = &SflowSample{}
	return s.SfSample
}

type SFlowConfig struct {
	SourceIPv4          string
	SourceIPv6          string
	IP                  IPType
	CollectorIPv4       string
	CollectorIPv6       string
	CollectorPort       uint16
	DSCP                uint8
	SampleSize          uint16
	IngressSamplingRate uint32
	InterfaceName       string
	ExporterMapName     string
	PacketLength        uint32
	DfBitSet            bool
}

type SfRecordRawPacketHeader struct {
	Protocol    uint32 // Protocol of the sampled packet
	FrameLength uint32 // Original length of the packet
	Stripped    uint32 // Number of bytes stripped from the packet
	Header      []byte // Header bytes of the sampled packet
}

type SfRecordExtendedRouterData struct {
	NextHop                string // IP address of the next hop router
	BaseFlowRecord         layers.SFlowBaseFlowRecord
	NextHopSourceMask      uint32
	NextHopDestinationMask uint32
}

type SfRecordExtendedGatewayData struct {
	BaseFlowRecord layers.SFlowBaseFlowRecord
	NextHop        string
	AS             uint32
	SourceAS       uint32
	PeerAS         uint32
	ASPathCount    uint32
	ASPath         []uint32 // AS path
	Communities    []uint32 // BGP communities
	LocalPref      uint32   // BGP local preference
}

// NewSfRecordRawPacketHeader creates a new instance of SfRecordRawPacketHeader
func NewSfRecordRawPacketHeader(protocol, frameLength, stripped uint32, header []byte) *SfRecordRawPacketHeader {
	return &SfRecordRawPacketHeader{
		Protocol:    protocol,
		FrameLength: frameLength,
		Stripped:    stripped,
		Header:      header,
	}
}

// Setters for SfRecordRawPacketHeader fields
func (s *SfRecordRawPacketHeader) SetProtocol(protocol uint32) {
	s.Protocol = protocol
}
func (s *SfRecordRawPacketHeader) SetFrameLength(frameLength uint32) {
	s.FrameLength = frameLength
}
func (s *SfRecordRawPacketHeader) SetStripped(stripped uint32) {
	s.Stripped = stripped
}
func (s *SfRecordRawPacketHeader) SetHeader(header []byte) {
	s.Header = header
}

// Getters for SfRecordRawPacketHeader fields
func (s *SfRecordRawPacketHeader) GetProtocol() uint32 {
	return s.Protocol
}
func (s *SfRecordRawPacketHeader) GetFrameLength() uint32 {
	return s.FrameLength
}
func (s *SfRecordRawPacketHeader) GetStripped() uint32 {
	return s.Stripped
}
func (s *SfRecordRawPacketHeader) GetHeader() []byte {
	return s.Header
}

// NewSfRecordExtendedRouterData creates a new instance of SfRecordExtendedRouterData
func NewSfRecordExtendedRouterData(
	nextHop string,
	nextHopSourceMask, nextHopDestinationMask uint32,
) *SfRecordExtendedRouterData {
	return &SfRecordExtendedRouterData{
		NextHop:                nextHop,
		NextHopSourceMask:      nextHopSourceMask,
		NextHopDestinationMask: nextHopDestinationMask,
	}
}

// Setters for SfRecordExtendedRouterData fields
func (s *SfRecordExtendedRouterData) SetBaseFlowRecord(baseFlowRecord layers.SFlowBaseFlowRecord) {
	s.BaseFlowRecord = baseFlowRecord
}

func (s *SfRecordExtendedRouterData) SetNextHopSourceMask(nextHopSourceMask uint32) {
	s.NextHopSourceMask = nextHopSourceMask
}

func (s *SfRecordExtendedRouterData) SetNextHopDestinationMask(nextHopDestinationMask uint32) {
	s.NextHopDestinationMask = nextHopDestinationMask
}

// Getters for SfRecordExtendedRouterData fields
func (s *SfRecordExtendedRouterData) GetBaseFlowRecord() layers.SFlowBaseFlowRecord {
	return s.BaseFlowRecord
}
func (s *SfRecordExtendedRouterData) GetNextHopSourceMask() uint32 {
	return s.NextHopSourceMask
}
func (s *SfRecordExtendedRouterData) GetNextHopDestinationMask() uint32 {
	return s.NextHopDestinationMask
}

// Setters for SfRecordExtendedRouterData fields
func (s *SfRecordExtendedRouterData) SetNextHop(nextHop string) {
	s.NextHop = nextHop
}

// Getters for SfRecordExtendedRouterData fields
func (s *SfRecordExtendedRouterData) GetNextHop() string {
	return s.NextHop
}

// NewSfRecordExtendedGatewayData creates a new instance of SfRecordExtendedGatewayData
func NewSfRecordExtendedGatewayData(
	nextHop string,
	as uint32,
	sourceAS uint32,
	peerAS uint32,
	asPathCount uint32,
	asPath []uint32,
	communities []uint32,
	localPref uint32,
) *SfRecordExtendedGatewayData {
	return &SfRecordExtendedGatewayData{
		NextHop:     nextHop,
		AS:          as,
		SourceAS:    sourceAS,
		PeerAS:      peerAS,
		ASPathCount: asPathCount,
		ASPath:      asPath,
		Communities: communities,
		LocalPref:   localPref,
	}
}

// Setters for SfRecordExtendedGatewayData fields
func (s *SfRecordExtendedGatewayData) SetNextHop(nextHop string) {
	s.NextHop = nextHop
}
func (s *SfRecordExtendedGatewayData) SetASPath(asPath []uint32) {
	s.ASPath = asPath
}
func (s *SfRecordExtendedGatewayData) SetCommunities(communities []uint32) {
	s.Communities = communities
}
func (s *SfRecordExtendedGatewayData) SetLocalPref(localPref uint32) {
	s.LocalPref = localPref
}

// Getters for SfRecordExtendedGatewayData fields
func (s *SfRecordExtendedGatewayData) GetNextHop() string {
	return s.NextHop
}
func (s *SfRecordExtendedGatewayData) GetASPath() []uint32 {
	return s.ASPath
}
func (s *SfRecordExtendedGatewayData) GetCommunities() []uint32 {
	return s.Communities
}
func (s *SfRecordExtendedGatewayData) GetLocalPref() uint32 {
	return s.LocalPref
}

// ConfigureSFlowOptimized provides an optimized version of sFlow configuration
func ConfigureSFlow(t *testing.T, dut *ondatra.DUTDevice, config *SFlowConfig) {
	sf := &oc.Sampling_Sflow{}
	sf.SetEnabled(true)
	sf.SetDscp(config.DSCP)
	sf.SetSampleSize(config.SampleSize)
	sf.SetIngressSamplingRate(config.IngressSamplingRate)

	// Configure IP version specific settings in a single block
	var collectorAddr, sourceAddr string
	if config.IP == IPv4 {
		sf.SetAgentIdIpv4(config.SourceIPv4)
		collectorAddr = config.CollectorIPv4
		sourceAddr = config.SourceIPv4
	} else {
		sf.SetAgentIdIpv6(config.SourceIPv6)
		collectorAddr = config.CollectorIPv6
		sourceAddr = config.SourceIPv6
	}

	// Configure collector and interface in one go
	collector := sf.GetOrCreateCollector(collectorAddr, config.CollectorPort)
	collector.SetSourceAddress(sourceAddr)

	sf.GetOrCreateInterface(config.InterfaceName).SetEnabled(true)

	gnmi.Replace(t, dut, gnmi.OC().Sampling().Sflow().Config(), sf)
}

// ConfigureUnsupportedSFlowFeatures configures vendor-specific sFlow features not supported by OpenConfig
func ConfigureUnsupportedSFlowFeatures(t *testing.T, dut *ondatra.DUTDevice, config *SFlowConfig) {
	batchSet := &gnmi.SetBatch{}
	cliPath, _ := schemaless.NewConfig[string]("", "cli")

	unsupportedConfig := fmt.Sprintf(`
	flow exporter-map %s
	dfbit set
	packet-length %d
	end
	`, config.ExporterMapName, config.PacketLength)

	gnmi.BatchUpdate(batchSet, cliPath, unsupportedConfig)
	batchSet.Set(t, dut)
}

func UpdateSFlowSamplingRate(t *testing.T, dut *ondatra.DUTDevice, rate uint32) {
	root := &oc.Root{}
	sf := root.GetOrCreateSampling().GetOrCreateSflow()
	sf.SetIngressSamplingRate(rate)
	gnmi.Update(t, dut, gnmi.OC().Sampling().Sflow().IngressSamplingRate().Config(), rate)
}

// EnableSFlowOnInterfaces enables sFlow on a list of interfaces
// interfaces is a slice of interface names (e.g., ["eth0", "eth1", "Bundle-Ether1"])
func EnableSFlowOnInterfaces(t *testing.T, dut *ondatra.DUTDevice, interfaces []string) {
	t.Helper()

	for _, intfName := range interfaces {
		t.Logf("Enabling sFlow on interface: %s", intfName)
		intf := &oc.Sampling_Sflow_Interface{}
		intf.SetName(intfName)
		intf.SetEnabled(true)

		gnmi.Update(t, dut, gnmi.OC().Sampling().Sflow().Interface(intfName).Config(), intf)
	}

	t.Logf("Successfully enabled sFlow on %d interface(s)", len(interfaces))
}

// DisableSFlowOnInterfaces disables sFlow on a list of interfaces
// interfaces is a slice of interface names (e.g., ["eth0", "eth1", "Bundle-Ether1"])
func DisableSFlowOnInterfaces(t *testing.T, dut *ondatra.DUTDevice, interfaces []string) {
	t.Helper()

	for _, intfName := range interfaces {
		t.Logf("Disabling sFlow on interface: %s", intfName)
		gnmi.Delete(t, dut, gnmi.OC().Sampling().Sflow().Interface(intfName).Config())
	}

	t.Logf("Successfully disabled sFlow on %d interface(s)", len(interfaces))
}
