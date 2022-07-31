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

// Binary recvmpls is a simple traffic generation receiver that logs the packets
// that it receives on a specified interface.
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"k8s.io/klog/v2"
)

var (
	intf = flag.String("interface", "", "Interface to capture packets on.")
	// timeout is the default timeout for opening a pcap session.
	timeout = 30 * time.Second
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if *intf == "" {
		klog.Exitf("Interface is required.")
	}

	handle, err := pcap.OpenLive(*intf,
		9000,    // capture 9KiB of packets if possible.
		true,    // promiscuous mode, receive any packets.
		timeout, // time to wait before timing out opening connections.
	)
	if err != nil {
		klog.Exitf("Cannot open interface %s to write to, err: %v", intf, err)
	}
	defer handle.Close()

	// Register to receive OS signals so we can capture interrupts (e.g., ctrl+c)
	// and stop the process.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, os.Interrupt)
	go func() {
		sig := <-sigs
		klog.Infof("Received signal %v", sig)
		os.Exit(1)
	}()

	ps := gopacket.NewPacketSource(handle, handle.LinkType())
	for p := range ps.Packets() {
		klog.Infof("%s: received packet %s", time.Now(), p)
		if mpls := p.Layer(layers.LayerTypeMPLS); mpls != nil {
			klog.Infof("%s: received MPLS packet: %s", time.Now(), mpls)
			// TODO(robjs): In the future capture some flow statistics here.
		}
	}
}
