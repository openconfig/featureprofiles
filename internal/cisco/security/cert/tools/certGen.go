// Package main provide utility to generate certificate for user and routers.
package main

import (
	"net"

	certUtil "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
)

func main() {
	// add your router or proxy address to the array of ip
	certUtil.GenCERT("ems", 500, []net.IP{net.IPv4(173, 39, 51, 67), net.IPv4(10, 85, 84, 159), net.IPv4(10, 85, 84, 38)}, "", "")
	// add your users if you use any users other than the followings
	certUtil.GenCERT("cisco", 100, []net.IP{}, "cisco", "")
	certUtil.GenCERT("cafyauto", 100, []net.IP{}, "cafyauto", "")
}
