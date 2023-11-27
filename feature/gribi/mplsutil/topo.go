package mplsutil

import (
	"fmt"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/entity-naming/entname"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// Tests initialised by the GRIBIMPLSTest helper rely on a specific topology
// of the form:
//
//	 -----         2001:db8::0/127      -----
//	|     |-port1--192.0.2.0/31--port1-|     |
//	| ate |                            | dut |
//	|     |-port2--192.0.2.2/31--port2-|     |
//	 -----         2001:db8::2/127      -----
//
// The following variables set up the device configurations for this topology.
var (
	// ATESrc describes the configuration parameters for the ATE port sourcing
	// a flow.
	ATESrc = &attrs.Attributes{
		Name:    "port1",
		Desc:    "ATE_SRC_PORT",
		IPv4:    "192.0.2.0",
		IPv4Len: 31,
		MAC:     "02:00:01:01:01:01",
		IPv6:    "2001:db8::0",
		IPv6Len: 127,
	}
	// DUTSrc describes the configuration parameters for the DUT port connected
	// to the ATE src port.
	DUTSrc = &attrs.Attributes{
		Desc:    "DUT_SRC_PORT",
		IPv4:    "192.0.2.1",
		IPv4Len: 31,
		IPv6:    "2001:db8::1",
		IPv6Len: 127,
	}
	// ATEDst describes the configuration parameters for the ATE port that acts
	// as the traffic sink.
	ATEDst = &attrs.Attributes{
		Name:    "port2",
		Desc:    "ATE_DST_PORT",
		IPv4:    "192.0.2.2",
		IPv4Len: 31,
		MAC:     "02:00:02:01:01:01",
		IPv6:    "2001:db8::2",
		IPv6Len: 127,
	}
	// DUTDst describes the configuration parameters for the DUT port that is
	// connected to the ate destination port.
	DUTDst = &attrs.Attributes{
		Desc:    "DUT_DST_PORT",
		IPv4:    "192.0.2.3",
		IPv4Len: 31,
		IPv6:    "2001:db8::3",
		IPv6Len: 127,
	}
)

const (
	// ipv4Prefix is the IPv4 prefix that the test sends packets towards
	// that results in MPLS labels being pushed to the packets.
	dutRoutedIPv4Prefix = "198.18.1.0/24"
	// staticMPLSToATE is an MPLS label that the test sends packets towards
	// that results in packets being routed to the ATE - it is configured
	// through gRIBI.
	staticMPLSToATE = 100
)

var (
	// entmap provides a mapping between an ONDATRA vendor and a vendor
	// within the entity naming library.
	entmap = map[ondatra.Vendor]entname.Vendor{
		ondatra.ARISTA:  entname.VendorArista,
		ondatra.NOKIA:   entname.VendorNokia,
		ondatra.JUNIPER: entname.VendorJuniper,
		ondatra.CISCO:   entname.VendorCisco,
	}
)

// dutIntf generates the configuration for an interface on the DUT in OpenConfig.
// It returns the generated configuration, or an error if the config could not be
// generated.
func dutIntf(dut *ondatra.DUTDevice, intf *attrs.Attributes, index int) ([]*oc.Interface, error) {
	if intf == nil {
		return nil, fmt.Errorf("invalid nil interface, %v", intf)
	}

	i := &oc.Interface{
		Name:        ygot.String(intf.Name),
		Description: ygot.String(intf.Desc),
		Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Enabled:     ygot.Bool(true),
	}

	dev := &entname.DeviceParams{Vendor: entmap[dut.Vendor()]}
	aggName, err := entname.AggregateInterface(dev, index)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate aggregate interface name for vendor %s", dut.Vendor())
	}
	i.GetOrCreateEthernet().AggregateId = ygot.String(aggName)

	pc := &oc.Interface{
		Name:        ygot.String(aggName),
		Description: ygot.String(fmt.Sprintf("LAG for %s", intf.Name)),
		Type:        oc.IETFInterfaces_InterfaceType_ieee8023adLag,
		Enabled:     ygot.Bool(true),
	}
	pc.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC

	v4 := pc.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	v4.Enabled = ygot.Bool(true)
	v4Addr := v4.GetOrCreateAddress(intf.IPv4)
	v4Addr.PrefixLength = ygot.Uint8(intf.IPv4Len)

	return []*oc.Interface{pc, i}, nil
}

// ateIntfConfig returns the configuration required for the ATE interfaces.
func configureATEInterfaces(t *testing.T, ate *ondatra.ATEDevice, srcATE, srcDUT, dstATE, dstDUT *attrs.Attributes) (gosnappi.Config, error) {
	topology := gosnappi.NewConfig()
	for _, p := range []struct {
		ate, dut *attrs.Attributes
	}{
		{ate: srcATE, dut: srcDUT},
		{ate: dstATE, dut: dstDUT},
	} {
		topology.Ports().Add().SetName(p.ate.Name)
		dev := topology.Devices().Add().SetName(p.ate.Name)
		eth := dev.Ethernets().Add().SetName(fmt.Sprintf("%s_ETH", p.ate.Name)).SetMac(p.ate.MAC)
		eth.Connection().SetPortName(dev.Name())
		ip := eth.Ipv4Addresses().Add().SetName(fmt.Sprintf("%s_IPV4", dev.Name()))
		ip.SetAddress(p.ate.IPv4).SetGateway(p.dut.IPv4).SetPrefix(uint32(p.ate.IPv4Len))

		ip6 := eth.Ipv6Addresses().Add().SetName(fmt.Sprintf("%s_IPV6", dev.Name()))
		ip6.SetAddress(p.ate.IPv6).SetGateway(p.dut.IPv6).SetPrefix(uint32(p.ate.IPv6Len))
	}

	c, err := topology.ToJson()
	if err != nil {
		return topology, err
	}
	t.Logf("configuration for OTG is %s", c)

	return topology, nil
}
