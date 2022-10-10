package gnmi_test

import (
	"testing"
	"time"
	"strconv"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/featureprofiles/internal/attrs"

)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGNMI(t *testing.T) {

	// Configuration to be set in DUT Ports
	configDUT := []attrs.Attributes{
		{
			Name: "port1",
			IPv4: "192.0.2.12",
			IPv4Len: 31,
			Desc: "From Ixia",
		},
		{
			Name: "port2",
			IPv4: "192.0.2.22",
			IPv4Len: 31,
			Desc: "To Ixia",
		},
		{
			Name: "port3",
			IPv4: "192.0.2.32",
			IPv4Len: 31,
			Desc: "To Ixia",
		},
		{
			Name: "port4",
			IPv4: "192.0.2.42",
			IPv4Len: 31,
			Desc: "To Ixia",
		},
	}


	// Configure a DUT

	dut := ondatra.DUT(t, "dut")

	for _, attributes := range configDUT { 
		ifCfg := &telemetry.Interface{
			Name: ygot.String(attributes.Name),
			Description: ygot.String(attributes.Desc),
		}

		ifCfg.GetOrCreateSubinterface(0).
		GetOrCreateIpv4().
		GetOrCreateAddress(attributes.IPv4).PrefixLength = ygot.Uint8(attributes.IPv4Len)

		dut.Config().Interface(attributes.Name).Update(t, ifCfg)
	}

	Ni1 := &telemetry.NetworkInstance{}

	Ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "static").
		GetOrCreateStatic("10.0.0.0/24").
		GetOrCreateNextHop("h1").NextHop = telemetry.UnionString("192.0.2.23")

	dut.Config().NetworkInstance("default").Update(t, Ni1)

	// ATE Ports Config
	configATE := []attrs.Attributes {
		{
			Name: "src",
			IPv4: "192.0.2.13",
			MAC: "02:1a:c0:00:02:01",
			IPv4Len: 31,
		},
		{
			Name: "dst1",
			IPv4: "192.0.2.23",
			MAC: "02:1a:c0:00:02:02",
			IPv4Len: 31,
		},
		{
			Name: "dst2",
			IPv4: "192.0.2.33",
			MAC: "02:1a:c0:00:02:03",
			IPv4Len: 31,
		},
		{
			Name: "dst3",
			IPv4: "192.0.2.43",
			MAC: "02:1a:c0:00:02:04",
			IPv4Len: 31,
		},
	}

	//  Configure an ATE

	ate := ondatra.ATE(t, "ate")
	top := ate.OTG().NewConfig(t)

	for index,attributes := range configATE {
		top.Ports().Add().SetName("port"+strconv.Itoa(index+1))
		i := top.Devices().Add().SetName("port"+strconv.Itoa(index+1))
		eth := i.Ethernets().Add().SetName(attributes.Name+".Eth").SetPortName(i.Name()).SetMac(attributes.MAC)
		eth.Ipv4Addresses().Add().SetName(attributes.Name+".IPv4").SetAddress(attributes.IPv4).SetGateway(configDUT[index].IPv4).SetPrefix(int32(attributes.IPv4Len))
	}
	
	ate.OTG().PushConfig(t,top)
	ate.OTG().StartProtocols(t)	


	flow := top.Flows().Add().SetName("Flow")
	flow.TxRx().Device().SetTxNames([]string{"src.IPv4"}).SetRxNames([]string{"dst1.IPv4"})
	flow.Metrics().Msg().Enable = ygot.Bool(true)
	endpoint := flow.Packet().Add().Ipv4()
	endpoint.Src().SetValue("192.0.2.13")
	endpoint.Dst().SetValue("10.0.0.0")
	ate.OTG().PushConfig(t,top)

	ate.OTG().StartTraffic(t)
	time.Sleep(5 * time.Second)
	ate.OTG().StopTraffic(t)

	fp := ate.OTG().Telemetry().Flow(flow.Name()).Get(t)
	fpc := fp.GetCounters()

	outoctets := fpc.GetOutOctets()
	outpkts := fpc.GetOutPkts()
	inpkts := fpc.GetInPkts()

	t.Logf("outoctets are %d",outoctets)
	t.Logf("inpkts are %d",inpkts)
	t.Logf("outpkts are %d",outpkts)

	lossPct := float32((outpkts - inpkts) * 100 / outpkts)
	t.Logf("flow loss-pct %f", lossPct)

	if lossPct>0 {
		t.Errorf("Packets are not received. Got %f loss percentage and wanted 0",lossPct)
	}

}