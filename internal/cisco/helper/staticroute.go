package helper

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	// ProtocolSTATIC is the static routing protocol type
	ProtocolSTATIC = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC
)

type staticRouteHelper struct{}

// AddStaticRouteToNull adds a static route to null (DROP) for an IPv4 or IPv6 prefix
// prefix should include CIDR notation (e.g., "192.168.1.0/24" or "2001:db8::/64")
// vrf is the network instance name (use "DEFAULT" for default VRF)
func (h *staticRouteHelper) AddStaticRouteToNull(t *testing.T, dut *ondatra.DUTDevice, prefix string, vrf string) {
	t.Helper()

	ni := oc.NetworkInstance{Name: ygot.String(vrf)}
	static := ni.GetOrCreateProtocol(ProtocolSTATIC, vrf)
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop("0")

	// Set next-hop to DROP to send traffic to null
	nh.SetNextHop(oc.UnionString("DROP"))

	t.Logf("Configuring static route %s to null in VRF %s", prefix, vrf)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrf).
		Protocol(ProtocolSTATIC, vrf).Config(), static)
}

// AddStaticRoutesToNull adds static routes to null for both IPv4 and IPv6 prefixes
// ipv4Prefix should be in CIDR notation (e.g., "192.168.1.0/24")
// ipv6Prefix should be in CIDR notation (e.g., "2001:db8::/64")
// vrf is the network instance name (use "DEFAULT" for default VRF)
func (h *staticRouteHelper) AddStaticRoutesToNull(t *testing.T, dut *ondatra.DUTDevice, ipv4Prefix, ipv6Prefix, vrf string) {
	t.Helper()

	// Add IPv4 static route to null
	if ipv4Prefix != "" {
		h.AddStaticRouteToNull(t, dut, ipv4Prefix, vrf)
	}

	// Add IPv6 static route to null
	if ipv6Prefix != "" {
		h.AddStaticRouteToNull(t, dut, ipv6Prefix, vrf)
	}

	t.Logf("Successfully configured static routes to null")
}

// DeleteStaticRoute removes a static route configuration
func DeleteStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix, vrf string) {
	t.Helper()

	t.Logf("Deleting static route %s from VRF %s", prefix, vrf)
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(vrf).
		Protocol(ProtocolSTATIC, vrf).Static(prefix).Config())
}

// DeleteStaticRoutes removes multiple static route configurations
func DeleteStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, prefixes []string, vrf string) {
	t.Helper()

	for _, prefix := range prefixes {
		DeleteStaticRoute(t, dut, prefix, vrf)
	}

	t.Logf("Successfully deleted %d static route(s)", len(prefixes))
}
