// Package gRIBI MPLS Dataplane Test implements tests of the MPLS dataplane that
// use gRIBI as the programming mechanism.
package gribi_mpls_dataplane_test

import (
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	mplscompliance "github.com/openconfig/featureprofiles/feature/gribi/tests/mpls"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	defNIName         = "default"
	baseLabel         = 42
	destinationLabel  = 100
	maximumStackDepth = 20
)

var (
	// sleepTime allows a user to specify that the test should sleep after setting
	// up all elements (configuration, gRIBI forwarding entries, traffic flows etc.).
	sleepTime = flag.Duration("sleep", 10*time.Second, "duration for which the test should sleep after setup")
)

var (
	// ateSrc describes the configuration parameters for the ATE port sourcing
	// a flow.
	ateSrc = &attrs.Attributes{
		Name:    "port1",
		Desc:    "ATE_SRC_PORT",
		IPv4:    "192.0.2.0",
		IPv4Len: 31,
		MAC:     "02:00:01:01:01:01",
		IPv6:    "2001:db8::0",
		IPv6Len: 127,
	}
	// dutSrc describes the configuration parameters for the DUT port connected
	// to the ATE src port.
	dutSrc = &attrs.Attributes{
		Desc:    "DUT_SRC_PORT",
		IPv4:    "192.0.2.1",
		IPv4Len: 31,
		IPv6:    "2001:db8::1",
		IPv6Len: 127,
	}
	// ateDst describes the configuration parameters for the ATE port that acts
	// as the traffic sink.
	ateDst = &attrs.Attributes{
		Name:    "port2",
		Desc:    "ATE_DST_PORT",
		IPv4:    "192.0.2.2",
		IPv4Len: 31,
		MAC:     "02:00:02:01:01:01",
		IPv6:    "2001:db8::2",
		IPv6Len: 127,
	}
	// dutDst describes the configuration parameters for the DUT port that is
	// connected to the ate destination port.
	dutDst = &attrs.Attributes{
		Desc:    "DUT_DST_PORT",
		IPv4:    "192.0.2.3",
		IPv4Len: 31,
		IPv6:    "2001:db8::3",
		IPv6Len: 127,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// dutIntf generates the configuration for an interface on the DUT in OpenConfig.
// It returns the generated configuration, or an error if the config could not be
// generated.
func dutIntf(intf *attrs.Attributes) ([]*oc.Interface, error) {
	if intf == nil {
		return nil, fmt.Errorf("invalid nil interface, %v", intf)
	}

	i := &oc.Interface{
		Name:        ygot.String(intf.Name),
		Description: ygot.String(intf.Desc),
		Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Enabled:     ygot.Bool(true),
	}

	// TODO(robjs): make neutral.
	aggName := fmt.Sprintf("Port-Channel%s", strings.Replace(intf.Name, "Ethernet", "", -1))
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

// configureATEInterfaces configures all the interfaces of the ATE according to the
// supplied ports (srcATE, srcDUT, dstATE, dstDUT) attributes. It returns the gosnappi
// OTG configuration that was applied to the ATE, or an error.
func configureATEInterfaces(t *testing.T, ate *ondatra.ATEDevice, srcATE, srcDUT, dstATE, dstDUT *attrs.Attributes) (gosnappi.Config, error) {
	otg := ate.OTG()
	topology := otg.NewConfig(t)
	for _, p := range []struct {
		ate, dut *attrs.Attributes
	}{
		{ate: srcATE, dut: srcDUT},
		{ate: dstATE, dut: dstDUT},
	} {
		topology.Ports().Add().SetName(p.ate.Name)
		dev := topology.Devices().Add().SetName(p.ate.Name)
		eth := dev.Ethernets().Add().SetName(fmt.Sprintf("%s_ETH", p.ate.Name))
		eth.SetPortName(dev.Name()).SetMac(p.ate.MAC)
		ip := eth.Ipv4Addresses().Add().SetName(fmt.Sprintf("%s_IPV4", dev.Name()))
		ip.SetAddress(p.ate.IPv4).SetGateway(p.dut.IPv4).SetPrefix(int32(p.ate.IPv4Len))

		ip6 := eth.Ipv6Addresses().Add().SetName(fmt.Sprintf("%s_IPV6", dev.Name()))
		ip6.SetAddress(p.ate.IPv6).SetGateway(p.dut.IPv6).SetPrefix(int32(p.ate.IPv6Len))
	}

	c, err := topology.ToJson()
	if err != nil {
		return topology, err
	}
	fmt.Printf("config is %s\n", c)

	otg.PushConfig(t, topology)
	return topology, nil
}

// pushBaseConfigs pushes the base configuration to the ATE and DUT devices in
// the test topology.
func pushBaseConfigs(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) gosnappi.Config {
	dutSrc.Name = dut.Port(t, "port1").Name()
	dutDst.Name = dut.Port(t, "port2").Name()

	otgCfg, err := configureATEInterfaces(t, ate, ateSrc, dutSrc, ateDst, dutDst)
	if err != nil {
		t.Fatalf("cannot configure ATE interfaces via OTG, %v", err)
	}

	/*dut.Config().New().WithAristaText(`
	mpls routing
	mpls ip
	router traffic-engineering
	   segment-routing
	!
	platform tfa personality python
	!
	mpls static top-label 32768 192.0.2.2 swap-label 32768
	`).Append(t)
	*/

	d := &oc.Root{}
	// configure ports on the DUT. note that each port maps to two interfaces
	// because we create a LAG.
	for _, i := range []*attrs.Attributes{dutSrc, dutDst} {
		cfgs, err := dutIntf(i)
		if err != nil {
			t.Fatalf("cannot generate configuration for interface %s, err: %v", i.Name, err)
		}
		/*for _, cfg := range cfgs {
			t.Logf("replacing interface %s config...", *cfg.Name)
			fptest.LogYgot(t, *cfg.Name, dut.Config().Interface(*cfg.Name), cfg)
			dut.Config().Interface(*cfg.Name).Replace(t, cfg)
		}*/
		for _, intf := range cfgs {
			d.AppendInterface(intf)
		}
	}
	fptest.LogQuery(t, "", gnmi.OC().Config(), d)
	gnmi.Update(t, dut, gnmi.OC().Config(), d)

	// TODO(robjs): make start protocols in fake more robust to allow for retry.
	time.Sleep(1 * time.Second)
	ate.OTG().StartProtocols(t)

	return otgCfg
}

// TestMPLSLabelPushDepth validates the gRIBI actions that are used to push N labels onto
// as part of routing towards a next-hop. Note that this test does not validate against the
// dataplane, but solely the gRIBI control-plane support.
func TestMPLSLabelPushDepth(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))

	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	otg := ondatra.ATE(t, "ate").OTG()

	testMPLSFlow := func(t *testing.T, _ []uint32) {
		// We configure a traffic flow from ateSrc -> ateDst (passes through
		// ateSrc -> [ dutSrc -- dutDst ] --> ateDst.
		//
		// Since EgressLabelStack pushes N labels but has a label forwarding
		// entry of 100 that points at that next-hop, we only need this value
		// to check whether traffic is forwarded.
		//
		// TODO(robjs): in the future, extend this test to check that the
		// received label stack is as we expected.

		// wait for ARP to resolve.
		t.Logf("looking on interface %s_ETH for %s", ateSrc.Name, dutSrc.IPv4)
		var dstMAC string
		gnmi.WatchAll(t, otg, gnmi.OTG().Interface(ateSrc.Name+"_ETH").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			dstMAC, _ = val.Val()
			return val.IsPresent()
		}).Await(t)
		t.Logf("MAC discovered was %s", dstMAC)

		// TODO(robjs): MPLS is currently not supported in OTG.
		otgCfg.Flows().Clear().Items()
		mplsFlow := otgCfg.Flows().Add().SetName("MPLS_FLOW")
		mplsFlow.Metrics().SetEnable(true)
		mplsFlow.TxRx().Port().SetTxName(ateSrc.Name).SetRxName(ateDst.Name)

		mplsFlow.Rate().SetChoice("pps").SetPps(1)

		// Set up ethernet layer.
		eth := mplsFlow.Packet().Add().Ethernet()
		eth.Src().SetChoice("value").SetValue(ateSrc.MAC)
		eth.Dst().SetChoice("value").SetValue(dstMAC)

		// Set up MPLS layer with destination label 100.
		mpls := mplsFlow.Packet().Add().Mpls()
		mpls.Label().SetChoice("value").SetValue(destinationLabel)
		mpls.BottomOfStack().SetChoice("value").SetValue(1)

		otg.PushConfig(t, otgCfg)

		t.Logf("Starting MPLS traffic...")
		otg.StartTraffic(t)
		t.Logf("Sleeping for %s...", *sleepTime)
		time.Sleep(*sleepTime)
		t.Logf("Stopping MPLS traffic...")
		otg.StopTraffic(t)

		otgutils.LogPortMetrics(t, otg, otgCfg)

		// TODO(robjs): validate traffic counters and received headers.
	}

	baseLabel := 42
	for i := 1; i <= maximumStackDepth; i++ {
		t.Run(fmt.Sprintf("push %d labels", i), func(t *testing.T) {
			t.Logf("running MPLS compliance test with %d labels.", i)
			mplscompliance.EgressLabelStack(t, c, defNIName, baseLabel, i, testMPLSFlow)
		})
	}
}

// TestMPLSPushToIP - inject IP flow from OTG, packet capture to validate that MPLS label stack was applied
//		      as expected.
// TestPopTopLabel - inject MPLS flow with multiple labels and validate that top-most label was removed.
//		   - inject MPLS flow with 1 label, and validate IP packet was exposed.
// TestPopNLabels  - inject MPLS flow with multiple labels and validate that the popped labels were removed.
// TestPopOnePushN - inject MPLS flow with multiple labels, and validate that one label was popped and N were pushed.
//		   - inject MPLS flow with 1 label, and validate that one label was popped and N were pushed.

func TestMPLSPushToIP(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)

	testIPFlow := func(t *testing.T, _ []uint32) {
		// We configure a traffic flow from ateSrc -> ateDst (passes through
		// ateSrc -> [ dutSrc -- dutDst ] --> ateDst.
		//
		// Since EgressLabelStack pushes N labels but has a label forwarding
		// entry of 100 that points at that next-hop, we only need this value
		// to check whether traffic is forwarded.
		//
		// TODO(robjs): in the future, extend this test to check that the
		// received label stack is as we expected.

		// wait for ARP to resolve.
		otg := ondatra.ATE(t, "ate").OTG()
		var dstMAC string
		gnmi.WatchAll(t, otg, gnmi.OTG().Interface(ateSrc.Name+"_ETH").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			dstMAC, _ = val.Val()
			return val.IsPresent()
		}).Await(t)

		// Remove any stale flows.
		otgCfg.Flows().Clear().Items()
		ipFlow := otgCfg.Flows().Add().SetName("MPLS_FLOW")
		ipFlow.Metrics().SetEnable(true)
		ipFlow.TxRx().Port().SetTxName(ateSrc.Name).SetRxName(ateDst.Name)

		// Set up ethernet layer.
		eth := ipFlow.Packet().Add().Ethernet()
		eth.Src().SetValue(ateSrc.MAC)
		eth.Dst().SetChoice("value").SetValue(dstMAC)

		ip4 := ipFlow.Packet().Add().Ipv4()
		ip4.Src().SetValue(ateSrc.IPv4)
		ip4.Dst().SetValue("10.0.0.1") // this must be in 10/8.

		otg.PushConfig(t, otgCfg)

		t.Logf("Starting IP traffic...")
		otg.StartTraffic(t)
		time.Sleep(120 * time.Second)
		t.Logf("Stopping IP traffic...")
		otg.StopTraffic(t)

		otgutils.LogPortMetrics(t, otg, otgCfg)
		otgutils.LogFlowMetrics(t, otg, otgCfg)

		// TODO(robjs): validate traffic counters and received headers.
	}

	baseLabel := 42
	numLabels := 20
	for i := 1; i <= numLabels; i++ {
		t.Run(fmt.Sprintf("push %d labels to IP", i), func(t *testing.T) {
			mplscompliance.PushToIPPacket(t, c, defNIName, baseLabel, i, testIPFlow)
		})
	}
}

func TestPopTopLabelMPLSInner(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)
	_ = otgCfg

	// TODO(robjs): define trafficFunc
	mplscompliance.PopTopLabel(t, c, defNIName, nil)
}

func TestPopTopLabelIPInner(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)
	_ = otgCfg

	// TODO(robjs): define trafficFunc
	mplscompliance.PopTopLabel(t, c, defNIName, nil)
}

func TestPopNLabels(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)
	_ = otgCfg

	for _, stack := range [][]uint32{{100}, {100, 42}, {100, 42, 43, 44, 45}} {
		// TODO(robjs): define trafficFunc

		t.Run(fmt.Sprintf("pop N labels, stack %v", stack), func(t *testing.T) {
			mplscompliance.PopNLabels(t, c, defNIName, stack, nil)
		})
	}
}

func TestPopOnePushN(t *testing.T) {
	otgCfg := pushBaseConfigs(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))
	gribic := ondatra.DUT(t, "dut").RawAPIs().GRIBI().Default(t)
	c := fluent.NewClient()
	c.Connection().WithStub(gribic)
	_ = otgCfg

	stacks := [][]uint32{
		{100}, // swap for label 100, pop+push for label 200
		{100, 200, 300, 400},
		{100, 200, 300, 400, 500, 600},
	}
	for _, stack := range stacks {
		// TODO(robjs): define trafficFunc
		t.Run(fmt.Sprintf("pop one, push N, stack: %v", stack), func(t *testing.T) {
			mplscompliance.PopOnePushN(t, c, defNIName, stack, nil)
		})
	}
}
