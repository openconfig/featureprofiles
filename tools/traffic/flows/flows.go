package flows

import (
	"fmt"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi/otg"
	"github.com/openconfig/featureprofiles/tools/traffic/lwotg"
	"k8s.io/klog/v2"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	// timeout generically specifies how long to wait to open a PCAP handle.
	timeout = 30 * time.Second
)

func SimpleMPLSFlowHandler(flow *otg.Flow, intfs map[string]string) (lwotg.TXRXFn, bool, error) {
	hdrs, err := simpleMPLSPacketHeaders(flow)
	if err != nil {
		return nil, false, err
	}

	pps, err := rate(flow, hdrs)
	if err != nil {
		return nil, false, err
	}

	tx, rx, err := ports(flow, intfs)
	if err != nil {
		return nil, false, err
	}

	klog.Infof("flow rate is %d PPS", pps)
	klog.Infof("ports: tx: %s, rx: %s", tx, rx)

	genFunc := func(stop chan struct{}, ch chan *gpb.Update, errCh chan error) {
		klog.Infof("SimpleMPLSFlowHandler send function  started.")
		buf := gopacket.NewSerializeBuffer()
		gopacket.SerializeLayers(buf, gopacket.SerializeOptions{}, hdrs...)
		klog.Infof("opening interface %s for send.", tx)
		handle, err := pcap.OpenLive(tx, 9000, true, timeout)
		if err != nil {
			klog.Errorf("cannot open PCAP handle in Tx goroutine for SimpleMPLS, %v", err)
			errCh <- err
			return
		}
		defer handle.Close()

		for {
			klog.Infof("entering select.")
			select {
			case <-stop:
				klog.Infof("simple MPLS Tx complete, returning")
				return
			default:
				klog.Infof("Sending %d packets...", pps)
				for i := 1; i <= int(pps); i++ {
					if err := handle.WritePacketData(buf.Bytes()); err != nil {
						klog.Errorf("cannot write packet in Go routine for SimpleMPLS, %v", err)
						return
					}
				}
				time.Sleep(1 * time.Second)
			}
		}
	}

	recvFunc := func(stop chan struct{}, ch chan *gpb.Update, errCh chan error) {
		klog.Infof("SimpleMPLSFlowHandler recv function  started.")
		klog.Infof("Opening rx port %s", rx)
		handle, err := pcap.OpenLive(rx, 9000, true, timeout)
		if err != nil {
			klog.Errorf("cannot open PCAP handle in Rx goroutine for SimpleMPLS, %v", err)
			errCh <- err
			return
		}
		defer handle.Close()
		ps := gopacket.NewPacketSource(handle, handle.LinkType())
		packetCh := ps.Packets()
		for {
			select {
			case <-stop:
				klog.Infof("stopping MPLS receive function...")
				return
			case p := <-packetCh:
				klog.Infof("SimpleMPLS received packet")
				upd, err := simpleMPLSRX(p)
				if err != nil {
					klog.Errorf("cannot parse packets received, %v", err)
					errCh <- fmt.Errorf("cannot parse received packet, %v", err)
					return
				}
				_ = upd
				//ch <- upd
			}
		}
	}

	return func(tx, rx *lwotg.FlowListener) {
		go genFunc(tx.Stop, tx.GNMI, tx.Err)
		go recvFunc(rx.Stop, rx.GNMI, rx.Err)
	}, true, nil

}

func ports(flow *otg.Flow, intfs map[string]string) (string, string, error) {
	if flow.GetTxRx() == nil || flow.GetTxRx().GetChoice() != otg.FlowTxRx_Choice_port {
		return "", "", fmt.Errorf("unsupported type of Tx/Rx specification, %v", flow.GetTxRx())
	}
	tx := flow.GetTxRx().GetPort().GetTxName()
	rx := flow.GetTxRx().GetPort().GetRxName()

	for _, s := range []string{tx, rx} {
		if _, ok := intfs[s]; !ok {
			return "", "", fmt.Errorf("port %s not found in interfaces %v", s, intfs)
		}
	}
	return intfs[tx], intfs[rx], nil

}

// rate takes an input flow and a sequence of headers and returns the number of packets that should
// be sent per second. It returns an error if it cannot parse or support the speciifed rate.
func rate(flow *otg.Flow, hdrs []gopacket.SerializableLayer) (int64, error) {
	// TODO(robjs): In the future allow us to specify more than PPS. This will use the packet headers to calculate
	// size.

	if flowT := flow.GetRate().GetChoice(); flowT != otg.FlowRate_Choice_unspecified && flowT != otg.FlowRate_Choice_pps {
		// default is PPS, hence we allow unspecified.
		return 0, fmt.Errorf("unsupported flow rate specification, %v", flowT)
	}
	pps := flow.GetRate().GetPps()
	if pps == 0 {
		return 1000, nil //default in the schema.
	}
	return pps, nil
}

func simpleMPLSPacketHeaders(flow *otg.Flow) ([]gopacket.SerializableLayer, error) {
	// Determine whether we are able to handle this packet. MPLSFlowHandler only
	// handles Ethernet + MPLS packets.
	var (
		ethernet *otg.FlowHeader
		mpls     []*otg.FlowHeader
	)

	for _, layer := range flow.Packet {
		switch t := layer.GetChoice(); t {
		case otg.FlowHeader_Choice_ethernet:
			if ethernet != nil {
				return nil, fmt.Errorf("simple MPLS does not handle Ethernet in MPLS, multiple Ethernet layers detected.")
			}
			ethernet = layer
		case otg.FlowHeader_Choice_mpls:
			mpls = append(mpls, layer)
		default:
			return nil, fmt.Errorf("simple MPLS does not handle layer %s", t)
		}
	}

	if dstT := ethernet.GetEthernet().GetDst().GetChoice(); dstT != otg.PatternFlowEthernetDst_Choice_value {
		return nil, fmt.Errorf("simple MPLS does not handle non-explicit destination MAC, got: %s", dstT)
	}
	if srcT := ethernet.GetEthernet().GetSrc().GetChoice(); srcT != otg.PatternFlowEthernetSrc_Choice_value {
		return nil, fmt.Errorf("simple MPLS does not handle non-explicit src MAC, got: %v", srcT)
	}

	srcMAC, err := net.ParseMAC(ethernet.GetEthernet().GetSrc().GetValue())
	if err != nil {
		return nil, fmt.Errorf("cannot parse source MAC, %v", err)
	}
	dstMAC, err := net.ParseMAC(ethernet.GetEthernet().GetDst().GetValue())
	if err != nil {
		return nil, fmt.Errorf("cannot parse destination MAC, %v", err)
	}

	pktLayers := []gopacket.SerializableLayer{
		&layers.Ethernet{
			SrcMAC:       srcMAC,
			DstMAC:       dstMAC,
			EthernetType: layers.EthernetTypeMPLSUnicast,
		},
	}

	// OTG says that the order of the layers must be the order on the wire - initially, we return
	// an error if the last label isn't BOS to ensure that we'e generating 'valid' packets.
	for _, m := range mpls {
		if valT := m.GetMpls().GetLabel().GetChoice(); valT != otg.PatternFlowMplsLabel_Choice_value {
			return nil, fmt.Errorf("simple MPLS does not handle labels that do not have an explicit value, got: %v", valT)
		}
		if bosT := m.GetMpls().GetBottomOfStack().GetChoice(); bosT != otg.PatternFlowMplsBottomOfStack_Choice_value {
			// It doesn't make sense here to have increment value - it can be 0 or 1. Possibly 'auto' should be suported.
			return nil, fmt.Errorf("bottom of stack with non-explicit value requested, must be explicit, %v", bosT)
		}
		var ttl uint8
		switch ttlT := m.GetMpls().GetTimeToLive().GetChoice(); ttlT {
		case otg.PatternFlowMplsTimeToLive_Choice_value:
			ttl = uint8(m.GetMpls().GetTimeToLive().GetValue())
		case otg.PatternFlowMplsTimeToLive_Choice_unspecified:
			ttl = 64 // default
		default:
			return nil, fmt.Errorf("simple MPLS does not handle TTLs that are not explicitly set.")
		}
		ll := &layers.MPLS{
			Label:       uint32(m.GetMpls().GetLabel().GetValue()),
			TTL:         ttl,
			StackBottom: m.GetMpls().GetBottomOfStack().GetValue() == 1,
		}
		pktLayers = append(pktLayers, ll)
	}

	return pktLayers, nil
}

func simpleMPLSRX(p gopacket.Packet) (*gpb.Update, error) {
	klog.Infof("received packet %v", p)
	return nil, nil
}
