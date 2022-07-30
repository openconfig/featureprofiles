package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"k8s.io/klog/v2"
)

type labels []uint32

func (l *labels) String() string {
	return fmt.Sprintf("MPLS labels %v", *l)
}

func (l *labels) Set(value string) error {
	for _, lbl := range strings.Split(value, ",") {
		v, err := strconv.ParseUint(lbl, 10, 32)
		if err != nil {
			return fmt.Errorf("cannot parse label %s, err: %v", lbl, err)
		}
		*l = append(*l, uint32(v))
	}
	return nil
}

var (
	labelFlag labels

	dstMAC   = flag.String("dst_mac", "", "destination MAC address that should be used for the packet.")
	srcMAC   = flag.String("src_mac", "00:01:01:01:01:01", "source MAC address that should be used for the packet.")
	intf     = flag.String("interface", "veth0", "Interface to write packets to.")
	interval = flag.Uint("interval", 1, "Seconds between subsequent packets.")

	timeout = 30 * time.Second
)

func init() {
	flag.Var(&labelFlag, "labels", "comma-separated list of labels to apply from bottom to top.")
}

func main() {
	flag.Parse()

	if *srcMAC == "" || *dstMAC == "" {
		klog.Exitf("Source and destination MAC must be specified, source: %s, destination: %s", *srcMAC, *dstMAC)
	}

	parsedSrc, err := net.ParseMAC(*srcMAC)
	if err != nil {
		klog.Exitf("Invalid source MAC %s, err: %v", *srcMAC, err)
	}

	parsedDst, err := net.ParseMAC(*dstMAC)
	if err != nil {
		klog.Exitf("Invalid destination MAC %s, err: %v", *dstMAC, err)
	}

	hdrStack := []gopacket.SerializableLayer{
		&layers.Ethernet{
			SrcMAC:       parsedSrc,
			DstMAC:       parsedDst,
			EthernetType: layers.EthernetTypeMPLSUnicast,
		},
	}
	for i, label := range labelFlag {
		hdr := &layers.MPLS{
			Label: label,
			TTL:   32,
		}
		if i == 0 {
			hdr.StackBottom = true
		}
		hdrStack = append(hdrStack, hdr)
	}

	klog.Infof("Compiled header stack: ")
	for i, h := range hdrStack {
		l, ok := h.(gopacket.Layer)
		if !ok {
			continue
		}
		klog.Infof("%d: %s\n", i, gopacket.LayerString(l))
	}

	buf := gopacket.NewSerializeBuffer()
	gopacket.SerializeLayers(buf, gopacket.SerializeOptions{}, hdrStack...)

	handle, err := pcap.OpenLive(*intf,
		9000,    // capture 9K
		true,    // promiscuous handle
		timeout, // # of seconds to run
	)
	if err != nil {
		klog.Exitf("Cannot open interface %s to write to, err: %v", intf, err)
	}
	defer handle.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, os.Interrupt)

	go func() {
		sig := <-sigs
		klog.Infof("Received signal %v", sig)
		os.Exit(1)
	}()

	for {
		time.Sleep(time.Duration(*interval) * time.Second)
		klog.Infof("Sending packet...")
		if err := handle.WritePacketData(buf.Bytes()); err != nil {
			klog.Errorf("Cannot send packet, %v", err)
		}
	}

}
