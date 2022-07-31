// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Binary sendmpls is a simple traffic generator program that sends traffic on
// a specific interface with a specified label stack.
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

// labels is a slice of uint32s that can be parsed as a custom flag.
type labels []uint32

// String implements the stringer interface required by a flag.
func (l *labels) String() string {
	return fmt.Sprintf("MPLS labels %v", *l)
}

// Set receives a set of labels from a comma-separated list and inserts them
// into the labels slice.
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
	// labelFlag is an instance of the custom flag type labels.
	labelFlag labels

	dstMAC   = flag.String("dst_mac", "", "destination MAC address that should be used for the packet.")
	srcMAC   = flag.String("src_mac", "00:01:01:01:01:01", "source MAC address that should be used for the packet.")
	intf     = flag.String("interface", "veth0", "Interface to write packets to.")
	interval = flag.Uint("interval", 1, "Seconds between subsequent packets.")

	// timeout is the time to wait when opening the pcap session to the interface
	// to write to.
	timeout = 30 * time.Second
)

func init() {
	flag.Var(&labelFlag, "labels", "comma-separated list of labels to apply from bottom to top.")
}

func main() {
	klog.InitFlags(nil)
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

	// Construct the packet - we have an Ethernet header followed by N MPLS
	// headers dependent upon the input labels.
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
		9000,    // capture 9KiB of data (less relevant since we are only writing)
		true,    // promiscuous access to the interface.
		timeout, // number of seconds to wait for opening the pcap session.
	)
	if err != nil {
		klog.Exitf("Cannot open interface %s to write to, err: %v", intf, err)
	}
	defer handle.Close()

	// Handle signals that might be sent to this process to ask it to exit.
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
