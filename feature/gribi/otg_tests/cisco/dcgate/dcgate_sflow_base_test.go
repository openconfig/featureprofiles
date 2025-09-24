package dcgate_test

import (
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	s "github.com/openconfig/featureprofiles/internal/cisco/sflow"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

var sfAttr = &s.SflowAttr{
	PacketsToSend:   uint32(*sf_trafficDuration * int(*sf_fps)),
	PpsRate:         uint64(*sf_fps),
	SflowDscp:       32,
	SamplingRate:    samplingRate,
	SampleTolerance: sampleTolerance,
	IP:              "IPv6",
}

// NewDefaultSflowAttr returns a SflowAttr with default values
func NewDefaultSflowAttr(ingressInterface string) *s.SFlowConfig {
	return &s.SFlowConfig{
		SourceIPv4:          dutLoopback0.IPv4,
		SourceIPv6:          dutLoopback0.IPv6,
		CollectorIPv6:       "2001:0db8::192:0:2:1e",
		IP:                  s.IPv6,
		CollectorPort:       6343,
		DSCP:                32,
		SampleSize:          343,
		IngressSamplingRate: samplingRate,
		InterfaceName:       ingressInterface,
		ExporterMapName:     "OC-FEM-GLOBAL",
		PacketLength:        8968,
		DfBitSet:            true,
	}
}

// Get interface indexes based on bundle mode
func getInterfaceIndexes(t *testing.T, dut *ondatra.DUTDevice, bundleName, intfName string, configBundleMode bool) []uint32 {
	if configBundleMode {
		bmIndexes := GetBundleMemberIfIndexes(t, dut, []string{bundleName})
		return bmIndexes[bundleName]
	}
	// Get interface indexes for physical ports
	portName := dut.Port(t, intfName).Name()
	portIndex := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Ifindex().State())
	return []uint32{portIndex}
}

func configureLoopbackAndSFlow(t *testing.T, dut *ondatra.DUTDevice) {
	config := NewDefaultSflowAttr(getPolicyInterface(t, dut, *bundleMode))

	// Configure Loopback0 interface
	configureLoopback0(t, dut, dutLoopback0)

	// Configure sFlow
	s.ConfigureSFlow(t, dut, config)

	// Configure unsupported features
	s.ConfigureUnsupportedSFlowFeatures(t, dut, config)
}

func validateSflowCapture(t *testing.T, args *testArgs, ports []string, sfc *s.SflowAttr) {
	if sfc == nil && !*capture_sflow {
		return
	}

	// Store original values
	origState := struct {
		ports    []string
		ttl      uint32
		dscp     int
		duration int
		fps      uint64
	}{
		ports:    args.capture_ports,
		ttl:      args.pattr.ttl,
		dscp:     args.pattr.dscp,
		duration: args.trafficDuration,
		fps:      getFlowsRate(args.flows),
	}

	// Apply sFlow configuration
	args.capture_ports = ports
	args.pattr.sfAttr = sfc
	args.pattr.dscp = int(sfc.SflowDscp)
	args.pattr.ttl = 255
	args.trafficDuration = *sf_trafficDuration
	SetFlowsRate(args.flows, sfc.PpsRate)

	// Ensure cleanup always happens
	defer func() {
		args.capture_ports = origState.ports
		args.pattr.sfAttr = nil
		args.pattr.ttl = origState.ttl
		args.pattr.dscp = origState.dscp
		args.trafficDuration = origState.duration
		SetFlowsRate(args.flows, origState.fps)
	}()

	// Enable capture, send traffic, and validate
	enableCapture(t, args.ate.OTG(), args.topo, args.capture_ports)
	defer clearCapture(t, args.ate.OTG(), args.topo)
	sendTraffic(t, args, args.flows, true)
	validatePacketCapture(t, args, args.capture_ports, args.pattr)
}

// Validate sFlow samples and their records from a single packet.
func validateSflowSampleFields(l *Logger, packet gopacket.Packet, pa *packetAttr) {
	l.LogOnce("Inside ValidateSflowSampleFields")
	if pa == nil || pa.sfAttr == nil {
		return
	}
	sflowLayer := packet.Layer(layers.LayerTypeSFlow)
	if sflowLayer == nil {
		l.LogOnce("sflowLayer is nil")
		return
	}
	datagram, ok := sflowLayer.(*layers.SFlowDatagram)
	if !ok {
		l.LogOnceErrorf("Warning: Could not cast SFlow layer to *layers.SFlowDatagram")
		return
	}

	for _, flowSample := range datagram.FlowSamples {
		// Sampling rate check if provided
		if pa.sfAttr.SamplingRate != 0 && flowSample.SamplingRate != uint32(pa.sfAttr.SamplingRate) {
			l.LogOnceErrorf("SFlow SamplingRate mismatch: got %d, want %d", flowSample.SamplingRate, pa.sfAttr.SamplingRate)
		} else if pa.sfAttr.SamplingRate != 0 {
			l.LogOncef("SFlow SamplingRate matched: %d", flowSample.SamplingRate)
		}

		// Input interface check if provided
		if len(pa.sfAttr.InputInterface) > 0 {
			match := false
			for _, idx := range pa.sfAttr.InputInterface {
				if flowSample.InputInterface == idx {
					match = true
					break
				}
			}
			if !match {
				l.LogOnceErrorf("SFlow InputInterface unexpected: got %d, allowed %v", flowSample.InputInterface, pa.sfAttr.InputInterface)
			} else {
				l.LogOncef("SFlow InputInterface matched: %d", flowSample.InputInterface)
			}
		}

		// Output interface check if provided
		if len(pa.sfAttr.OutputInterface) > 0 {
			match := false
			for _, idx := range pa.sfAttr.OutputInterface {
				if flowSample.OutputInterface == idx {
					match = true
					break
				}
			}
			if !match {
				l.LogOnceErrorf("SFlow OutputInterface unexpected: got %d, allowed %v", flowSample.OutputInterface, pa.sfAttr.OutputInterface)
			} else {
				l.LogOncef("SFlow OutputInterface matched: %d", flowSample.OutputInterface)
			}
		}

		// Optional deeper validation driven by pa.sfAttr.SfSample expectations.
		if pa.sfAttr.SfSample != nil {
			for _, rec := range flowSample.Records {
				switch r := rec.(type) {
				case layers.SFlowRawPacketFlowRecord:
					validateSflowRawPacketRecord(l, r, pa.sfAttr.SfSample.RawPktHdr)
				case layers.SFlowExtendedRouterFlowRecord:
					validateSflowExtendedRouterRecord(l, r, pa.sfAttr.SfSample.ExtdRtrData)
				case layers.SFlowExtendedGatewayFlowRecord:
					validateSflowExtendedGatewayRecord(l, r, pa.sfAttr.SfSample.ExtdGtwData)
				default:
					l.LogOncef("SFlow unhandled record type %T", r)
				}
			}
		}
	}
}

func validateSflowRawPacketRecord(l *Logger, r layers.SFlowRawPacketFlowRecord, exp *s.SfRecordRawPacketHeader) {
	if exp == nil {
		return
	}
	if exp.FrameLength != 0 && r.FrameLength != exp.FrameLength {
		l.LogOnceErrorf("SFlow RawPacket FrameLength mismatch: got %d, want %d", r.FrameLength, exp.FrameLength)
	} else if exp.FrameLength != 0 {
		l.LogOncef("SFlow RawPacket FrameLength ok: %d", r.FrameLength)
	}
	if exp.Protocol != 0 && uint32(r.HeaderProtocol) != exp.Protocol {
		l.LogOnceErrorf("SFlow RawPacket HeaderProtocol mismatch: got %d, want %d", r.HeaderProtocol, exp.Protocol)
	} else if exp.Protocol != 0 {
		l.LogOncef("SFlow RawPacket HeaderProtocol ok: %d", r.HeaderProtocol)
	}
}

func validateSflowExtendedRouterRecord(l *Logger, r layers.SFlowExtendedRouterFlowRecord, exp *s.SfRecordExtendedRouterData) {
	if exp == nil {
		return
	}
	if exp.NextHop != "" && r.NextHop.String() != exp.NextHop {
		l.LogOnceErrorf("SFlow ExtRouter NextHop mismatch: got %s, want %s", r.NextHop, exp.NextHop)
	} else if exp.NextHop != "" {
		l.LogOncef("SFlow ExtRouter NextHop ok: %s", r.NextHop)
	}
	if exp.NextHopSourceMask != 0 && r.NextHopSourceMask != exp.NextHopSourceMask {
		l.LogOnceErrorf("SFlow ExtRouter NextHopSourceMask mismatch: got %d, want %d", r.NextHopSourceMask, exp.NextHopSourceMask)
	} else if exp.NextHopSourceMask != 0 {
		l.LogOncef("SFlow ExtRouter NextHopSourceMask ok: %d", r.NextHopSourceMask)
	}
	if exp.NextHopDestinationMask != 0 && r.NextHopDestinationMask != exp.NextHopDestinationMask {
		l.LogOnceErrorf("SFlow ExtRouter NextHopDestinationMask mismatch: got %d, want %d", r.NextHopDestinationMask, exp.NextHopDestinationMask)
	} else if exp.NextHopDestinationMask != 0 {
		l.LogOncef("SFlow ExtRouter NextHopDestinationMask ok: %d", r.NextHopDestinationMask)
	}
}

func validateSflowExtendedGatewayRecord(l *Logger, r layers.SFlowExtendedGatewayFlowRecord, exp *s.SfRecordExtendedGatewayData) {
	if exp == nil {
		return
	}
	if exp.NextHop != "" && r.NextHop.String() != exp.NextHop {
		l.LogOnceErrorf("SFlow ExtGateway NextHop mismatch: got %s, want %s", r.NextHop, exp.NextHop)
	} else if exp.NextHop != "" {
		l.LogOncef("SFlow ExtGateway NextHop ok: %s", r.NextHop)
	}
	if exp.AS != 0 && r.AS != exp.AS {
		l.LogOnceErrorf("SFlow ExtGateway AS mismatch: got %d, want %d", r.AS, exp.AS)
	} else if exp.AS != 0 {
		l.LogOncef("SFlow ExtGateway AS ok: %d", r.AS)
	}
	if exp.SourceAS != 0 && r.SourceAS != exp.SourceAS {
		l.LogOnceErrorf("SFlow ExtGateway SourceAS mismatch: got %d, want %d", r.SourceAS, exp.SourceAS)
	} else if exp.SourceAS != 0 {
		l.LogOncef("SFlow ExtGateway SourceAS ok: %d", r.SourceAS)
	}
	if exp.PeerAS != 0 && r.PeerAS != exp.PeerAS {
		l.LogOnceErrorf("SFlow ExtGateway PeerAS mismatch: got %d, want %d", r.PeerAS, exp.PeerAS)
	} else if exp.PeerAS != 0 {
		l.LogOncef("SFlow ExtGateway PeerAS ok: %d", r.PeerAS)
	}
	if exp.ASPathCount != 0 && r.ASPathCount != exp.ASPathCount {
		l.LogOnceErrorf("SFlow ExtGateway ASPathCount mismatch: got %d, want %d", r.ASPathCount, exp.ASPathCount)
	} else if exp.ASPathCount != 0 {
		l.LogOncef("SFlow ExtGateway ASPathCount ok: %d", r.ASPathCount)
	}
	if len(exp.ASPath) > 0 {
		if len(r.ASPath) != len(exp.ASPath) {
			l.LogOnceErrorf("SFlow ExtGateway ASPath length mismatch: got %d, want %d", len(r.ASPath), len(exp.ASPath))
		} else {
			for i, want := range exp.ASPath {
				if i < len(r.ASPath) && r.ASPath[0].Members[i] != want {
					l.LogOnceErrorf("SFlow ExtGateway ASPath[%d] mismatch: got %d, want %d", i, r.ASPath[0].Members[i], want)
				}
			}
		}
	}
	if len(exp.Communities) > 0 {
		for _, want := range exp.Communities {
			found := false
			for _, got := range r.Communities {
				if got == want {
					found = true
					break
				}
			}
			if !found {
				l.LogOnceErrorf("SFlow ExtGateway missing community: %d in %v", want, r.Communities)
			}
		}
	}
	if exp.LocalPref != 0 && r.LocalPref != exp.LocalPref {
		l.LogOnceErrorf("SFlow ExtGateway LocalPref mismatch: got %d, want %d", r.LocalPref, exp.LocalPref)
	} else if exp.LocalPref != 0 {
		l.LogOncef("SFlow ExtGateway LocalPref ok: %d", r.LocalPref)
	}
}
