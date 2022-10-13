package gnmi_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

var (
	dutPorts = map[string]attrs.Attributes{
		"port1": {
			IPv4:    "192.0.2.12",
			IPv6:    "2001:db8::12",
			IPv4Len: 31,
			IPv6Len: 127,
			Desc:    "ATE port 1 to DUT port 1",
		},
		"port2": {
			IPv4:    "192.0.2.22",
			IPv6:    "2001:db8::22",
			IPv4Len: 31,
			IPv6Len: 127,
			Desc:    "DUT port 2 to ATE port 2",
		},
		// "port3": {
		// 	IPv4:    "192.0.2.32",
		// 	IPv6:    "2001:db8::32",
		// 	IPv4Len: 31,
		// 	IPv6Len: 127,
		// 	Desc:    "DUT port 3 to ATE port 3",
		// },
		// "port4": {
		// 	IPv4:    "192.0.2.42",
		// 	IPv6:    "2001:db8::42",
		// 	IPv4Len: 31,
		// 	IPv6Len: 127,
		// 	Desc:    "DUT port 4 to ATE port 4",
		// },
	}

	atePorts = map[string]attrs.Attributes{
		"port1": {
			IPv4:    "192.0.2.13",
			IPv6:    "2001:db8::13",
			MAC:     "02:1a:c0:00:02:01",
			IPv4Len: 31,
			IPv6Len: 127,
		},
		"port2": {
			IPv4:    "192.0.2.23",
			IPv6:    "2001:db8::23",
			MAC:     "02:1a:c0:00:02:02",
			IPv4Len: 31,
			IPv6Len: 127,
		},
		// "port3": {
		// 	IPv4:    "192.0.2.33",
		// 	IPv6:    "2001:db8::33",
		// 	MAC:     "02:1a:c0:00:02:03",
		// 	IPv4Len: 31,
		// 	IPv6Len: 127,
		// },
		// "port4": {
		// 	IPv4:    "192.0.2.43",
		// 	IPv6:    "2001:db8::43",
		// 	MAC:     "02:1a:c0:00:02:04",
		// 	IPv4Len: 31,
		// 	IPv6Len: 127,
		// },
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestStaticRouteSingleDestinationPort(t *testing.T) {

	// Configure a DUT
	dut := ondatra.DUT(t, "dut")

	for name, attributes := range dutPorts {
		ifCfg := &telemetry.Interface{
			Name:        ygot.String(name),
			Description: ygot.String(attributes.Desc),
		}
		// ifCfg.GetOrCreateSubinterface(0).GetOrCreateIpv4().Enabled = ygot.Bool(true)
		ifCfg.GetOrCreateSubinterface(0).
			GetOrCreateIpv4().
			GetOrCreateAddress(attributes.IPv4).PrefixLength = ygot.Uint8(attributes.IPv4Len)

		dut.Config().Interface(name).Update(t, ifCfg)
	}

	ni := &telemetry.NetworkInstance{}
	ni.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "static").
		GetOrCreateStatic("10.0.0.0/24").
		GetOrCreateNextHop("h").NextHop = telemetry.UnionString(atePorts["port2"].IPv4)

	dut.Config().NetworkInstance("default").Update(t, ni)

	//  Configure an ATE

	ate := ondatra.ATE(t, "ate")
	top := ate.OTG().NewConfig(t)

	for name, attributes := range atePorts {
		top.Ports().Add().SetName(name)
		i := top.Devices().Add().SetName(name)
		eth := i.Ethernets().Add().SetName(name + ".Eth").SetPortName(i.Name()).SetMac(attributes.MAC)
		eth.Ipv4Addresses().Add().SetName(name + ".IPv4").SetAddress(attributes.IPv4).SetGateway(dutPorts[name].IPv4).SetPrefix(int32(attributes.IPv4Len))
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Flow
	flow := top.Flows().Add().SetName("Flow")
	flow.TxRx().Device().SetTxNames([]string{"port1.IPv4"}).SetRxNames([]string{"port2.IPv4"})
	flow.Metrics().SetEnable(true)
	endpoint := flow.Packet().Add().Ipv4()
	endpoint.Src().SetValue(atePorts["port1"].IPv4)
	endpoint.Dst().SetValue("1.0.0.25")
	ate.OTG().PushConfig(t, top)

	ate.OTG().StartTraffic(t)
	time.Sleep(180 * time.Second)
	ate.OTG().StopTraffic(t)

	fp := ate.OTG().Telemetry().Flow(flow.Name()).Get(t)
	fpc := fp.GetCounters()

	outoctets := fpc.GetOutOctets()
	outpkts := fpc.GetOutPkts()
	inpkts := fpc.GetInPkts()

	t.Logf("IPv4 Flow Details")
	t.Logf("outoctets are %d", outoctets)
	t.Logf("inpkts are %d", inpkts)
	t.Logf("outpkts are %d", outpkts)
	lossPct := float32((outpkts - inpkts) * 100 / outpkts)
	t.Logf("flow loss-pct %f", lossPct)
	if lossPct > 0 {
		t.Errorf("Packets are not received. Got %f loss percentage and wanted 0", lossPct)
	}

}
