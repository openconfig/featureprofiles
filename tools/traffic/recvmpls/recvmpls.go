package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"k8s.io/klog/v2"
)

var (
	intf    = flag.String("interface", "", "Interface to capture packets on.")
	timeout = 30 * time.Second
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if *intf == "" {
		klog.Exitf("Interface is required.")
	}

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

	ps := gopacket.NewPacketSource(handle, handle.LinkType())
	for p := range ps.Packets() {
		klog.Infof("received packet %s", p)
	}
}
