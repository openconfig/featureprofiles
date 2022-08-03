package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/open-traffic-generator/snappi/gosnappi/otg"
	"github.com/openconfig/featureprofiles/tools/traffic/lwotg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"
)

var port string

func init() {
	flag.StringVar(&port, "port", os.Getenv("PORT"), "Port to listen on")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		klog.Exitf("cannot listen on port %d, err: %v", port, err)
	}

	// OTG is expected to be inscure in ONDATRA currently.
	t := insecure.NewCredentials()
	s := grpc.NewServer(grpc.Creds(t))
	reflection.Register(s)
	otg.RegisterOpenapiServer(s, lwotg.New())
	klog.Infof("Listening on %s", l.Addr())
	go s.Serve(l)

	// Handle signals that might be sent to this process to ask it to exit.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, os.Interrupt)
	sig := <-sigs
	klog.Infof("Received signal %v", sig)
	os.Exit(1)

}
