// Package packetvalidationhelpers provides helper functions to setup Protocol configurations on traffic generators.
package packetvalidationhelpers

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
)

/*

For Encap traffic validation, the following validations are performed:
1. Validate the outer IPv4 header (ValidateIPv4Header).
2. Validate the MPLS layer (ValidateMPLSLayer).
3. Validate the inner IPv4 header (ValidateInnerIPv4Header).

Validations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateMPLSLayer,
		packetvalidationhelpers.ValidateInnerIPv4Header,
	}

	EncapPacketValidation = &packetvalidationhelpers.PacketValidation{
		PortName:         "port3",
		Ipv4layer:     	IPlayerIPv4,
		MplsLayer:        MplsLayer,
		Validations:      Validations,
		InnerIplayerIPv4: InnerIPlayerIPv4,
		InnerIplayerIPv6: InnerIPlayerIPv6,
		TCPLayer:         &packetvalidationhelpers.TCPLayer{SrcPort: 49152, DstPort: 179},
		UDPLayer:         &packetvalidationhelpers.UDPLayer{SrcPort: 49152, DstPort: 3784},
	}

	createflow(t, top, FlowIPv4, true)
	packetvalidationhelpers.ConfigurePacketCapture(t, top, []string{"port3"}, EncapPacketValidation)
	sendTrafficCapture(t, ate)
	if err := FlowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("ValidateLossOnFlows(): got err: %q", err)
	}

	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, top, ate, EncapPacketValidation); err != nil {
		t.Errorf("CaptureAndValidatePackets(): got err: %q", err)
	}

	For Decap traffic validation, the following validation is performed:
1. Validate the IPv4 header : actual customer traffic (ValidateIPv4Header).
2. Validate the IPV6 header (ValidateIPv6Header).
*/

// IPv4 and IPv6 are the IP protocol types.
const (
	IPv4 = "IPv4"
	IPv6 = "IPv6"
	TCP  = 6  // TCP protocol number as seen on the wire.
	UDP  = 17 // UDP protocol number as seen on the wire.
)

// ValidationType defines the type of validation to perform.
type ValidationType string

const (
	// ValidateIPv4Header validates the  IPv4 header.
	ValidateIPv4Header ValidationType = "ValidateIPv4Header"
	// ValidateIPv6Header validates the IPv6 header.
	ValidateIPv6Header ValidationType = "ValidateIPv6Header"
	// ValidateInnerIPv4Header validates the inner IPv4 header.
	ValidateInnerIPv4Header ValidationType = "ValidateInnerIPv4Header"
	// ValidateInnerIPv6Header validates the inner IPv6 header.
	ValidateInnerIPv6Header ValidationType = "ValidateInnerIPv6Header"
	// ValidateMPLSLayer validates the MPLS layer.
	ValidateMPLSLayer ValidationType = "ValidateMPLSLayer"
	// ValidateTCPHeader validates the TCP header.
	ValidateTCPHeader ValidationType = "ValidateTCPHeader"
	// ValidateUDPHeader validates the UDP header.
	ValidateUDPHeader ValidationType = "ValidateUDPHeader"
)

// PacketValidation is a struct to hold the packet validation parameters.
type PacketValidation struct {
	PortName         string
	CaptureName      string
	CaptureCount     int
	IPv4Layer        *IPv4Layer
	IPv6Layer        *IPv6Layer
	GreLayer         *GreLayer
	MPLSLayer        *MPLSLayer
	TCPLayer         *TCPLayer
	UDPLayer         *UDPLayer
	InnerIPLayerIPv4 *IPv4Layer
	InnerIPLayerIPv6 *IPv6Layer
	// Validations is a list of validations to perform on the captured packets.
	Validations []ValidationType
}

// IPv4Layer is a struct to hold the IP layer parameters.
type IPv4Layer struct {
	Protocol uint32
	DstIP    string
	Tos      uint8
	TTL      uint8
}

// IPv6Layer is a struct to hold the IP layer parameters.
type IPv6Layer struct {
	DstIP        string
	TrafficClass uint8
	HopLimit     uint8
	NextHeader   uint32
}

// GreLayer is a struct to hold the GRE layer parameters.
type GreLayer struct {
	Protocol uint32
}

// MPLSLayer holds MPLS layer properties
type MPLSLayer struct {
	Label uint32
	Tc    uint8
}

// TCPLayer holds the TCP layer parameters.
type TCPLayer struct {
	SrcPort uint32
	DstPort uint32
}

// UDPLayer holds the UDP layer parameters.
type UDPLayer struct {
	SrcPort uint32
	DstPort uint32
}

// StartCapture starts the capture on the port.
func StartCapture(t *testing.T, ate *ondatra.ATEDevice) gosnappi.ControlState {
	t.Helper()

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	ate.OTG().SetControlState(t, cs)

	return cs
}

// StopCapture stops the capture on the port.
func StopCapture(t *testing.T, ate *ondatra.ATEDevice, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	ate.OTG().SetControlState(t, cs)
}

// CaptureAndValidatePackets captures the packets and validates the traffic.
func CaptureAndValidatePackets(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, packetVal *PacketValidation) error {
	t.Helper()
	bytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(packetVal.PortName))
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		return fmt.Errorf("could not create temporary pcap file: %v", err)
	}
	if _, err := f.Write(bytes); err != nil {
		return fmt.Errorf("could not write bytes to pcap file: %v", err)
	}
	defer os.Remove(f.Name()) // Clean up the temporary file
	f.Close()

	handle, err := pcap.OpenOffline(f.Name())
	if err != nil {
		t.Fatalf("could not open pcap file")
		return err
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	// Iterate over the validations specified in packetVal.Validations.
	for _, validation := range packetVal.Validations {
		switch validation {
		case ValidateIPv4Header:
			if err := validateIPv4Header(t, packetSource, packetVal); err != nil {
				return err
			}
		case ValidateInnerIPv4Header:
			if err := validateInnerIPv4Header(t, packetSource, packetVal); err != nil {
				return err
			}
		case ValidateIPv6Header:
			if err := validateIPv6Header(t, packetSource, packetVal); err != nil {
				return err
			}
		case ValidateInnerIPv6Header:
			if err := validateInnerIPv6Header(t, packetSource, packetVal); err != nil {
				return err
			}
		case ValidateMPLSLayer:
			if err := validateMPLSLayer(t, packetSource, packetVal); err != nil {
				return err
			}
		case ValidateTCPHeader:
			if err := validateTCPHeader(t, packetSource, packetVal); err != nil {
				return err
			}
		case ValidateUDPHeader:
			if err := validateUDPHeader(t, packetSource, packetVal); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown validation type: %s", validation)
		}
	}
	return nil
}

// ClearCapture clears the capture on the port.
func ClearCapture(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice) {
	t.Helper()
	top.Captures().Clear()
	ate.OTG().PushConfig(t, top)
}

// validateIPv4Header validates the outer IPv4 header.
func validateIPv4Header(t *testing.T, packetSource *gopacket.PacketSource, packetVal *PacketValidation) error {
	t.Helper()
	t.Log("Validating IPv4 header")

	for packet := range packetSource.Packets() {
		t.Logf("packet: %v", packet)
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)

			if uint32(ip.Protocol) != packetVal.IPv4Layer.Protocol {
				return fmt.Errorf("packet is not encapsulated properly. Encapsulated protocol is: %d, expected: %d", ip.Protocol, packetVal.IPv4Layer.Protocol)
			}
			if ip.DstIP.String() != packetVal.IPv4Layer.DstIP {
				return fmt.Errorf("IP Dst IP is not set properly. Expected: %s, Actual: %s", packetVal.IPv4Layer.DstIP, ip.DstIP)
			}
			if ip.TTL != packetVal.IPv4Layer.TTL {
				return fmt.Errorf("IP TTL value is altered to: %d, expected: %d", ip.TTL, packetVal.IPv4Layer.TTL)
			}
			if ip.TOS != packetVal.IPv4Layer.Tos {
				return fmt.Errorf("DSCP(TOS) value is altered to: %d, expected: %d", ip.TOS, packetVal.IPv4Layer.Tos)
			}
			// If validation is successful for one packet, we can return.
			return nil
		}
	}
	return fmt.Errorf("no IPv4 packets found")
}

// validateIPv6Header validates the outer IPv4 header.
func validateIPv6Header(t *testing.T, packetSource *gopacket.PacketSource, packetVal *PacketValidation) error {
	t.Helper()
	t.Log("Validating IPv6 header")

	for packet := range packetSource.Packets() {
		t.Logf("packet: %v", packet)
		if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
			ipv6, _ := ipLayer.(*layers.IPv6)

			if ipv6.DstIP.String() != packetVal.IPv6Layer.DstIP {
				return fmt.Errorf("IPv6 Dst IP is not set properly. Expected: %s, Actual: %s", packetVal.IPv6Layer.DstIP, ipv6.DstIP)
			}
			if ipv6.HopLimit != packetVal.IPv6Layer.HopLimit {
				return fmt.Errorf("IPv6 HopLimit value is altered to: %d. Expected: %d", ipv6.HopLimit, packetVal.IPv6Layer.HopLimit)
			}
			if ipv6.TrafficClass != packetVal.IPv6Layer.TrafficClass {
				return fmt.Errorf("Traffic Class value is altered to: %d. Expected: %d", ipv6.TrafficClass, packetVal.IPv6Layer.TrafficClass)
			}
			if packetVal.IPv6Layer.NextHeader != 0 {
				if uint32(ipv6.NextHeader) != packetVal.IPv6Layer.NextHeader {
					return fmt.Errorf("Next Header value is altered to: %d. Expected: %d", ipv6.NextHeader, packetVal.IPv6Layer.NextHeader)
				}
			}
			// If validation is successful for one packet, we can return.
			return nil
		}
	}
	return fmt.Errorf("no IPv6 packets found")
}

// validateInnerIPv4Header validates the inner IPv4 header.
func validateInnerIPv4Header(t *testing.T, packetSource *gopacket.PacketSource, packetVal *PacketValidation) error {
	t.Helper()
	t.Log("Validating inner IPv4 header")

	for packet := range packetSource.Packets() {
		if greLayer := packet.Layer(layers.LayerTypeGRE); greLayer != nil {
			gre := greLayer.(*layers.GRE)
			encapPacket := gopacket.NewPacket(gre.Payload, gre.NextLayerType(), gopacket.Default)

			if ipLayer := encapPacket.Layer(layers.LayerTypeIPv4); ipLayer != nil {
				ip, _ := ipLayer.(*layers.IPv4)

				if ip.DstIP.String() != packetVal.InnerIPLayerIPv4.DstIP {
					return fmt.Errorf("IP Dst IP is not set properly. Expected: %s, Actual: %s", packetVal.InnerIPLayerIPv4.DstIP, ip.DstIP)
				}
				if ip.TTL != packetVal.InnerIPLayerIPv4.TTL {
					return fmt.Errorf("IP TTL value is altered to: %d. Expected: %d", ip.TTL, packetVal.InnerIPLayerIPv4.TTL)
				}
				if ip.TOS != packetVal.InnerIPLayerIPv4.Tos {
					return fmt.Errorf("DSCP(TOS) value is altered to: %d .Expected: %d", ip.TOS, packetVal.InnerIPLayerIPv4.Tos)
				}
				if packetVal.InnerIPLayerIPv4.Protocol != 0 {
					if uint32(ip.Protocol) != packetVal.InnerIPLayerIPv4.Protocol {
						return fmt.Errorf("Protocol value is altered to: %d. Expected: %d", ip.Protocol, packetVal.InnerIPLayerIPv4.Protocol)
					}
				}
				// If validation is successful for one packet, we can return.
				return nil
			}
		}
	}
	return fmt.Errorf("no inner IPv4 packets found")
}

// validateInnerIPv6Header validates the inner IPv6 header.
func validateInnerIPv6Header(t *testing.T, packetSource *gopacket.PacketSource, packetVal *PacketValidation) error {
	t.Helper()
	t.Log("Validating inner IPv6 header")

	for packet := range packetSource.Packets() {
		if greLayer := packet.Layer(layers.LayerTypeGRE); greLayer != nil {
			gre := greLayer.(*layers.GRE)
			encapPacket := gopacket.NewPacket(gre.Payload, gre.NextLayerType(), gopacket.Default)

			if ipv6Layer := encapPacket.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
				ipv6, _ := ipv6Layer.(*layers.IPv6)
				if ipv6.DstIP.String() != packetVal.InnerIPLayerIPv6.DstIP {
					return fmt.Errorf("IPv6 Dst IP is not set properly. Expected: %s, Actual: %s", packetVal.InnerIPLayerIPv6.DstIP, ipv6.DstIP)
				}
				if ipv6.HopLimit != packetVal.InnerIPLayerIPv6.HopLimit {
					return fmt.Errorf("IPv6 HopLimit value is altered to: %d. Expected: %d", ipv6.HopLimit, packetVal.InnerIPLayerIPv6.HopLimit)
				}
				if ipv6.TrafficClass != packetVal.InnerIPLayerIPv6.TrafficClass {
					return fmt.Errorf("Traffic Class value is altered to: %d. Expected: %d", ipv6.TrafficClass, packetVal.InnerIPLayerIPv6.TrafficClass)
				}
				if packetVal.InnerIPLayerIPv6.NextHeader != 0 {
					if uint32(ipv6.NextHeader) != packetVal.InnerIPLayerIPv6.NextHeader {
						return fmt.Errorf("Next Header value is altered to: %d. Expected: %d", ipv6.NextHeader, packetVal.InnerIPLayerIPv6.NextHeader)
					}
				}
				// If validation is successful for one packet, we can return.
				return nil
			}
		}
	}
	return fmt.Errorf("no inner IPv6 packets found")
}

// validateMPLSLayer validates the MPLS layer.
func validateMPLSLayer(t *testing.T, packetSource *gopacket.PacketSource, packetVal *PacketValidation) error {
	t.Helper()
	t.Log("Validating MPLS layer")

	for packet := range packetSource.Packets() {
		if mplsLayer := packet.Layer(layers.LayerTypeMPLS); mplsLayer != nil {
			mpls, _ := mplsLayer.(*layers.MPLS)

			if mpls.Label != packetVal.MPLSLayer.Label {
				return fmt.Errorf("Mpls label is not set properly. Expected: %d, Actual: %d", packetVal.MPLSLayer.Label, mpls.Label)
			}
			if mpls.TrafficClass != packetVal.MPLSLayer.Tc {
				return fmt.Errorf("Mpls traffic class is not set properly. Expected: %d, Actual: %d", packetVal.MPLSLayer.Tc, mpls.TrafficClass)
			}
			// If validation is successful for one packet, we can return.
			return nil
		}
	}
	return fmt.Errorf("no MPLS packets found")
}

// validateTCPHeader validates the TCP header.
func validateTCPHeader(t *testing.T, packetSource *gopacket.PacketSource, packetVal *PacketValidation) error {
	t.Helper()
	t.Log("Validating TCP header")

	for packet := range packetSource.Packets() {
		if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
			tcp, _ := tcpLayer.(*layers.TCP)

			if uint32(tcp.DstPort) != packetVal.TCPLayer.DstPort {
				return fmt.Errorf("TCP Dst Port is not set properly. Expected: %d, Actual: %d", packetVal.TCPLayer.DstPort, tcp.DstPort)
			}
			if uint32(tcp.SrcPort) != packetVal.TCPLayer.SrcPort {
				return fmt.Errorf("TCP Src Port is not set properly. Expected: %d, Actual: %d", packetVal.TCPLayer.SrcPort, tcp.SrcPort)
			}
			// If validation is successful for one packet, we can return.
			return nil
		}
	}
	return fmt.Errorf("no TCP packets found")
}

// validateUDPHeader validates the UDP header.
func validateUDPHeader(t *testing.T, packetSource *gopacket.PacketSource, packetVal *PacketValidation) error {
	t.Helper()
	t.Log("Validating UDP header")

	for packet := range packetSource.Packets() {
		if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
			udp, _ := udpLayer.(*layers.UDP)

			if uint32(udp.DstPort) != packetVal.UDPLayer.DstPort {
				return fmt.Errorf("UDP Dst Port is not set properly. Expected: %d, Actual: %d", packetVal.UDPLayer.DstPort, udp.DstPort)
			}
			if uint32(udp.SrcPort) != packetVal.UDPLayer.SrcPort {
				return fmt.Errorf("UDP Src Port is not set properly. Expected: %d, Actual: %d", packetVal.UDPLayer.SrcPort, udp.SrcPort)
			}
			// If validation is successful for one packet, we can return.
			return nil
		}
	}
	return fmt.Errorf("no UDP packets found")
}

// ConfigurePacketCapture configures the packet capture on the port.
func ConfigurePacketCapture(t *testing.T, top gosnappi.Config, packetVal *PacketValidation) {
	t.Helper()
	ports := []string{}
	ports = append(ports, packetVal.PortName)
	top.Captures().Add().SetName(packetVal.CaptureName).
		SetPortNames(ports).
		SetFormat(gosnappi.CaptureFormat.PCAP)
}
