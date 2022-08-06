package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/open-traffic-generator/snappi/gosnappi/otg"
	"github.com/openconfig/featureprofiles/tools/traffic/lwotg"
	"github.com/openconfig/featureprofiles/tools/traffic/lwotgtelem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	telemPort, port string
)

func init() {
	flag.StringVar(&port, "port", os.Getenv("PORT"), "Port to listen on")
	flag.StringVar(&telemPort, "telemetry_port", os.Getenv("TELEMETRY_PORT"), "Telemetry port to listen on.")
}

func main() {
	// Some dependency is depending on glog.
	if flag.CommandLine.Lookup("log_dir") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}
	klog.InitFlags(nil)
	flag.Parse()
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		klog.Exitf("cannot listen on port %d, err: %v", port, err)
	}

	klog.Infof("running new...")
	lw := lwotg.New()
	ts, err := lwotgtelem.New(context.Background(), "ate")
	if err != nil {
		klog.Exitf("cannot open telemetry server, err: %v", err)
	}

	klog.Infof("finished new")
	hintCh := make(chan lwotg.Hint, 1000)
	fmt.Printf("1:setting hint channnel...")
	lw.SetHintChannel(hintCh)
	fmt.Printf("2:setting hint channnel...")
	ts.SetHintChannel(context.Background(), hintCh)

	// OTG is expected to be insecure in ONDATRA currently.
	t := insecure.NewCredentials()
	s := grpc.NewServer(grpc.Creds(t))
	reflection.Register(s)
	otg.RegisterOpenapiServer(s, lw)
	klog.Infof("Listening on %s", l.Addr())

	// OTG telemetry server.
	tl, err := net.Listen("tcp", fmt.Sprintf(":%s", telemPort))
	gs := grpc.NewServer(grpc.Creds(t))
	reflection.Register(gs)
	gpb.RegisterGNMIServer(gs, ts.GNMIServer)
	klog.Infof("Telemetry listening on %s", tl.Addr())

	go s.Serve(l)
	go gs.Serve(tl)

	// Handle signals that might be sent to this process to ask it to exit.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, os.Interrupt)
	sig := <-sigs
	klog.Infof("Received signal %v", sig)
	os.Exit(1)

}
