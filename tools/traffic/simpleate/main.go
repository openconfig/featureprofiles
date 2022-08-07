package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-ping/ping"
	"github.com/open-traffic-generator/snappi/gosnappi/otg"
	"github.com/openconfig/featureprofiles/tools/traffic/lwotg"
	"github.com/openconfig/featureprofiles/tools/traffic/lwotgtelem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	telemPort, port   string
	certFile, keyFile string
)

func main() {
	// Some dependency is depending on glog.
	if flag.CommandLine.Lookup("log_dir") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}
	klog.InitFlags(nil)
	flag.StringVar(&port, "port", os.Getenv("PORT"), "Port to listen on")
	flag.StringVar(&telemPort, "telemetry_port", os.Getenv("TELEMETRY_PORT"), "Telemetry port to listen on.")
	flag.StringVar(&certFile, "certfile", "", "Certificate file for gNMI.")
	flag.StringVar(&keyFile, "keyfile", "", "Key file for gNMI.")
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

	// In this implementation our 'start protocols' is just sending gratuitous ARPs.
	lw.SetProtocolHandler(
		func(cfg *otg.Config, _ otg.ProtocolState_State_Enum) error {
			gw := []string{}
			for _, d := range cfg.GetDevices() {
				for _, i := range d.GetEthernets() {
					for _, a4 := range i.GetIpv4Addresses() {
						gw = append(gw, a4.GetGateway())
					}
				}
			}
			klog.Infof("ping gateways %v", gw)

			for _, a := range gw {
				pinger, err := ping.NewPinger(a)
				if err != nil {
					return fmt.Errorf("cannot parse gateway address %s, %v", a, err)
				}
				pinger.SetPrivileged(true)
				pinger.Count = 1
				if err := pinger.Run(); err != nil {
					return fmt.Errorf("cannot ping address %s, %v", a, err)
				}
				klog.Infof("ping statistics, %v", pinger.Statistics())
			}
			return nil
		},
		/*func(_ *otg.Config, _ otg.ProtocolState_State_Enum) error {
			return intf.SendARP(false)
		},*/
	)

	klog.Infof("finished new")
	hintCh := make(chan lwotg.Hint, 1000)
	lw.SetHintChannel(hintCh)
	ts.SetHintChannel(context.Background(), hintCh)

	// OTG is expected to be insecure in ONDATRA currently.
	t := insecure.NewCredentials()
	s := grpc.NewServer(grpc.Creds(t))
	reflection.Register(s)
	otg.RegisterOpenapiServer(s, lw)
	klog.Infof("Listening on %s", l.Addr())

	// OTG telemetry server.
	tl, err := net.Listen("tcp", fmt.Sprintf(":%s", telemPort))

	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		klog.Exitf("cannot create creds, %v", err)
	}

	gs := grpc.NewServer(grpc.Creds(creds))
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
