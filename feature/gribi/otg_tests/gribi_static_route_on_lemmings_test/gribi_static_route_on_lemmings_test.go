package basic_static_route_support_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google3/third_party/open_traffic_generator/gosnappi/gosnappi"
	"google3/third_party/openconfig/featureprofiles/internal/attrs/attrs"
	"google3/third_party/openconfig/featureprofiles/internal/deviations/deviations"
	"google3/third_party/openconfig/featureprofiles/internal/fptest/fptest"
	"google3/third_party/openconfig/featureprofiles/internal/gribi/gribi"
	"google3/third_party/openconfig/gribigo/chk/chk"
	"google3/third_party/openconfig/gribigo/constants/constants"
	"google3/third_party/openconfig/gribigo/fluent/fluent"
	"google3/third_party/openconfig/ondatra/gnmi/gnmi"
	"google3/third_party/openconfig/ondatra/ondatra"
	"google3/third_party/openconfig/ondatra/otg/otg"
)

const (
	niTransitTeVrf          = "DEFAULT"
	ipv4OuterDst111         = "198.50.100.64"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		Desc:    "ATE Port 1",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		Desc:    "ATE Port 2",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut       *ondatra.DUTDevice
	ctx       context.Context
	client    *fluent.GRIBIClient
	ate       *ondatra.ATEDevice
	otgConfig gosnappi.Config
	otg       *otg.OTG
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGRIBI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
	configureOTG(t, dut)
	configureGribiRoute(t, dut)

	// gnmi.Get(t, dut, gnmi.OC().NetworkInstance(networkInstance).Afts().Ipv4Entry(ipv4OuterDst111+"/"+mask).State())

	aft := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().State())
	t.Logf("dut1 system: %v", aft)

	afts := gnmi.LookupAll(t, dut, gnmi.OC().NetworkInstanceAny().Afts().State())
	t.Logf("dut1 system: %v", afts)

	fibt := gnmi.LookupAll(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).ProtocolAny().State())
	t.Logf("dut1 system: %v", fibt)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	t.Logf("configureDUT")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureOTG(t *testing.T, dut *ondatra.DUTDevice) {
	t.Logf("configure OTG")
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	cfg := gosnappi.NewConfig()
	cfg.Ports().Add().SetName(ap1.ID())
	intf1 := cfg.Devices().Add().SetName("intf1")
	eth1Name := fmt.Sprintf("%s.eth", intf1.Name())
	eth1 := intf1.Ethernets().Add().SetName(eth1Name).SetMac(atePort1.MAC)
	eth1.Connection().SetPortName(ap1.ID())
	ip1Name := fmt.Sprintf("%s.ipv4", intf1.Name())
	eth1.Ipv4Addresses().Add().
		SetName(ip1Name).
		SetAddress(atePort1.IPv4).
		SetPrefix(30).
		SetGateway(dutPort1.IPv4)
	cfg.Ports().Add().SetName(ap2.ID())
	intf2 := cfg.Devices().Add().SetName("intf2")
	eth2Name := fmt.Sprintf("%s.eth", intf2.Name())
	eth2 := intf2.Ethernets().Add().SetName(eth2Name).SetMac(atePort2.MAC)
	eth2.Connection().SetPortName(ap2.ID())
	ip2Name := fmt.Sprintf("%s.ipv4", intf2.Name())
	eth2.Ipv4Addresses().Add().
		SetName(ip2Name).
		SetAddress(atePort2.IPv4).
		SetPrefix(30).
		SetGateway(dutPort2.IPv4)
	ate.OTG().PushConfig(t, cfg)
	ate.OTG().StartProtocols(t)
}

func configureGribiRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Configure GRIBI")
	t.Helper()
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(12, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()
	client.Start(ctx, t)
	defer client.Stop(t)
	gribi.FlushAll(client)
	defer gribi.FlushAll(client)
	client.StartSending(ctx, t)
	gribi.BecomeLeader(t, client)

	tcArgs := &testArgs{
		ctx:    ctx,
		client: client,
		dut:    dut,
	}

	tcArgs.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithIndex(uint64(2)).WithIPAddress(atePort2.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithID(uint64(2)).AddNextHop(uint64(2), uint64(1)),

		fluent.IPv4Entry().WithNetworkInstance(niTransitTeVrf).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(ipv4OuterDst111+"/32").WithNextHopGroup(uint64(2)))

	if err := awaitTimeout(tcArgs.ctx, t, tcArgs.client, 90*time.Second); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, tcArgs.client.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(ipv4OuterDst111+"/32").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

func awaitTimeout(ctx context.Context, t testing.TB, c *fluent.GRIBIClient, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}
