package main

import (
	"net"
	certUtil "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
)


func main(){
	certUtil.GenCERT("ems",500,[]net.IP{net.IPv4(10,85,84,159),net.IPv4(10,85,84,38)}, "")
}