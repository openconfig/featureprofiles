//Package main provide utility to generate certificate for user and routers. 
package main

import (
	"net"
	certUtil "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
)


func main(){
	// certUtil.GenCERT("ems",500,[]net.IP{net.IPv4(10,85,84,159),net.IPv4(10,85,84,38)}, "", "")
	certUtil.GenCERT("cisco", 100, []net.IP{}, "cisco","")
	//genCERT("Moji_SFD", 100, []net.IP{net.IPv4(10,85,84,159)})
	// in our lab env we add all ips for proxy routers here, this way we use the same certificate for all lab routers.
	//GenCERT("ems", 100, []net.IP{net.IPv4(10,85,84,159)}, "")
}