package sflow

import (
	"testing"

	"github.com/google/gopacket/layers"
	"github.com/openconfig/lemming/gnmi/oc"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

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

// flowConfig and IPType are provided in the original prompt
type SflowConfig struct {
	Name            string
	PacketsToSend   uint32
	PpsRate         uint64
	FrameSize       uint32
	SflowDscp       uint8
	SamplingRate    uint
	SampleTolerance float32
	IP              IPType
	InputInterface  []uint32
	OutputInterface []uint32
}

type SFlowConfig struct {
	LoopbackIPv4        string
	LoopbackIPv6        string
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
	inputInterface, outputInterface uint32,
) *SfRecordExtendedRouterData {
	return &SfRecordExtendedRouterData{
		NextHop: nextHop,
	}
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

// ConfigureSFlow configures sFlow sampling with the provided configuration
func ConfigureSFlow(t *testing.T, dut *ondatra.DUTDevice, config *SFlowConfig) {
	root := &oc.Root{}

	sf := root.GetOrCreateSampling().GetOrCreateSflow()
	sf.SetEnabled(true)
	sf.SetAgentIdIpv6(config.LoopbackIPv6)
	sf.SetSampleSize(config.SampleSize)
	sf.SetDscp(config.DSCP)

	// Add a collector (destination + port)
	collector := sf.GetOrCreateCollector(config.CollectorIPv6, config.CollectorPort)
	collector.SetSourceAddress(config.LoopbackIPv6)

	// Set sampling rate
	sf.SetIngressSamplingRate(config.IngressSamplingRate)

	// Configure per-interface settings
	intSf := sf.GetOrCreateInterface(config.InterfaceName)
	intSf.SetEnabled(true)

	gnmi.Replace(t, dut, gnmi.OC().Sampling().Sflow().Config(), sf)
}
