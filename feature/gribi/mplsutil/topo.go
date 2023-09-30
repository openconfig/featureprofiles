package mplsutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/entity-naming/entname"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

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

var (
	// entmap provides a mapping between an ONDATRA vendor and a vendor
	// within the entity naming library.
	entmap = map[ondatra.Vendor]entname.Vendor{
		ondatra.ARISTA: entname.VendorArista,
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

// PushBaseConfigs pushes the base configuration to the ATE and DUT devices in
// the test topology.
func PushBaseConfigs(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) gosnappi.Config {
	DUTSrc.Name = dut.Port(t, "port1").Name()
	DUTDst.Name = dut.Port(t, "port2").Name()

	otgCfg, err := configureATEInterfaces(t, ate, ATESrc, DUTSrc, ATEDst, DUTDst)
	if err != nil {
		t.Fatalf("cannot configure ATE interfaces via OTG, %v", err)
	}

	ate.OTG().PushConfig(t, otgCfg)

	d := &oc.Root{}
	// configure ports on the DUT. note that each port maps to two interfaces
	// because we create a LAG.
	for index, i := range []*attrs.Attributes{DUTSrc, DUTDst} {
		cfgs, err := dutIntf(dut, i, index)
		if err != nil {
			t.Fatalf("cannot generate configuration for interface %s, err: %v", i.Name, err)
		}
		for _, intf := range cfgs {
			d.AppendInterface(intf)
		}
	}
	fptest.LogQuery(t, "", gnmi.OC().Config(), d)
	gnmi.Update(t, dut, gnmi.OC().Config(), d)

	// Sleep for 1 second to ensure that OTG has absorbed configuration.
	time.Sleep(1 * time.Second)
	ate.OTG().StartProtocols(t)

	return otgCfg
}
