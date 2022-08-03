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

var (
	tlsKey  = flag.String("keyfile", "", "TLS keyfile to use for server.")
	tlsCert = flag.String("certfile", "", "TLS certificate to use for server.")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		klog.Exitf("cannot listen on port %d, err: %v", port, err)
	}

	if *tlsKey == "" || *tlsCert == "" {
		klog.Exitf("must specify TLS credentials, key: %s, cert: %s", *tlsKey, *tlsCert)
	}

	/*t, err := credentials.NewServerTLSFromFile(*tlsCert, *tlsKey)
	if err != nil {
		klog.Exitf("cannot read certificate files, err: %v", err)
	}*/
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
