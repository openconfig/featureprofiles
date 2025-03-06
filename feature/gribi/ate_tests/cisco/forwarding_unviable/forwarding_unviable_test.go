package forwarding_unviable_test

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/cisco/performance"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	ft "github.com/openconfig/featureprofiles/tools/inputcisco/feature"
	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

const (
	with_scale            = false                    // run entire script with or without scale (Support not yet coded)
	with_RPFO             = true                     // run entire script with or without RFPO
	base_config           = "case2_decap_encap_exit" // Will run all the tcs with set base programming case, options : case1_backup_decap, case2_decap_encap_exit, case3_decap_encap, case4_decap_encap_recycle
	active_rp             = "0/RP0/CPU0"
	standby_rp            = "0/RP1/CPU0"
	lc                    = "0/2/CPU0" // set value for lc_oir tc, if empty it means no lc, example: 0/0/CPU0
	process_restart_count = 1
	microdropsRepeat      = 1
	programming_RFPO      = 1
	viable                = true
	unviable              = false
	ipv4PrefixLen         = 24
	dst                   = "202.1.0.1"
	v4mask                = "32"
	dstCount              = 1
	totalBgpPfx           = 1
	minInnerDstPrefixBgp  = "202.1.0.1"
	totalIsisPrefix       = 1 //set value for scale isis setup ex: 10000
	minInnerDstPrefixIsis = "201.1.0.1"
	ipv6PrefixLen         = 126
	policyTypeIsis        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	dutAreaAddress        = "47.0001"
	dutSysId              = "0000.0000.0001"
	isisName              = "osisis"
	policyTypeBgp         = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	bgpAs                 = 65000
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "BundlePort1",
		IPv4:    "100.120.1.1",
		MAC:     "1.2.0",
		IPv4Len: ipv4PrefixLen,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "100.120.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "BundlePort2",
		IPv4:    "100.121.1.1",
		MAC:     "1.2.1",
		IPv4Len: ipv4PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort3 = attrs.Attributes{ //non-bundle member
		Desc:    "dutPort3",
		IPv4:    "100.123.2.3",
		IPv4Len: ipv4PrefixLen,
	}
	// atePort3 = attrs.Attributes{
	// 	Name:    "atePort3",
	// 	IPv4:    "100.123.2.4",
	// 	IPv4Len: ipv4PrefixLen,
	// }
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology
}

var (
	prefixes   = []string{}
	rpfo_count = 0 // used to track rpfo_count if its more than 10 then reset to 0 and reload the HW
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configure Lag on Dut
func configureDutLag(t *testing.T, dut *ondatra.DUTDevice) {
	dutPorts := sortPorts(dut.Ports())
	d := gnmi.OC()

	// incoming interface is Bundle-Ether121 with only 1 member (port1)
	incoming := &oc.Interface{Name: ygot.String("Bundle-Ether1")}
	gnmi.Replace(t, dut, d.Interface(*incoming.Name).Config(), configInterfaceDUT(dut, incoming, &dutPort1))
	srcPort := dutPorts[0]
	dutSource := generateBundleMemberInterfaceConfig(srcPort.Name(), *incoming.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(srcPort.Name()).Config(), dutSource)

	outgoing := &oc.Interface{Name: ygot.String("Bundle-Ether2")}
	outgoingData := configInterfaceDUT(dut, outgoing, &dutPort2)
	g := outgoingData.GetOrCreateAggregation()
	g.LagType = oc.IfAggregate_AggregationType_LACP
	gnmi.Replace(t, dut, d.Interface(*outgoing.Name).Config(), configInterfaceDUT(dut, outgoing, &dutPort2))

	for _, port := range dutPorts[1:3] {
		dutDest := generateBundleMemberInterfaceConfig(port.Name(), *outgoing.Name)
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), dutDest)
	}
}

// configureAteLag configures the ATE with the given topology.
func configureAteLag(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	t.Helper()
	atePorts := sortPorts(ate.Ports())
	top := ate.Topology().New()

	p1 := atePorts[0]
	i1 := top.AddInterface(p1.Name()).WithPort(atePorts[0])
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)

	i2 := top.AddInterface("lag1")
	lag := top.AddLAG("lag1").WithPorts(atePorts[1:3]...)
	lag.LACP().WithEnabled(true)
	i2.WithLAG(lag)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)

	return top
}

func configIsis(t *testing.T, dut *ondatra.DUTDevice, intfNames []string) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	prot := inst.GetOrCreateProtocol(policyTypeIsis, isisName)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysId)}
	glob.LevelCapability = 2
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	for _, intfName := range intfNames {
		intf := isis.GetOrCreateInterface(intfName)
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intf.Enabled = ygot.Bool(true)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	}
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = 2

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(policyTypeIsis, isisName)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(policyTypeIsis, isisName)
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

// configAteIsisL2 configures ISIS on the ATE
func configAteIsisL2(t *testing.T, topo *ondatra.ATETopology, networkName string, metric uint32, v4prefix string, count uint32) {
	intfs := topo.Interfaces()["lag1"]

	network := intfs.AddNetwork(networkName)
	network.ISIS().WithIPReachabilityMetric(metric + 1)
	network.IPv4().WithAddress(v4prefix).WithCount(count)
	intfs.ISIS().WithAreaID(dutAreaAddress).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

// configAteRoutingProtocols configures routing protocol configurations on the ATE
func configAteRoutingProtocols(t *testing.T, top *ondatra.ATETopology) {
	configAteIsisL2(t, top, "isis_network", 20, minInnerDstPrefixIsis+"/"+v4mask, totalIsisPrefix)
}

func getSubInterface(dut *ondatra.DUTDevice, dutPort *attrs.Attributes, index uint32, vlanID uint16) *oc.Interface_Subinterface {
	s := &oc.Interface_Subinterface{}
	//unshut sub/interface
	if deviations.InterfaceEnabled(dut) {
		s.Enabled = ygot.Bool(true)
	}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(dutPort.IPv4)
	a.PrefixLength = ygot.Uint8(dutPort.IPv4Len)
	v := s.GetOrCreateVlan()
	m := v.GetOrCreateMatch()
	if index != 0 {
		m.GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
	}
	return s
}

func configInterfaceDUT(dut *ondatra.DUTDevice, i *oc.Interface, dutPort *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(dutPort.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	i.AppendSubinterface(getSubInterface(dut, dutPort, 0, 0))
	return i
}

// generateBundleMemberInterfaceConfig returns interface configuration populated with bundle info
func generateBundleMemberInterfaceConfig(name, bundleID string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

// sortPorts sorts the given slice of ports by the testbed port ID in ascending order.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// testFlow sends traffic across ATE ports and verifies continuity.
func testtrafficFlow(
	t *testing.T,
	ate *ondatra.ATEDevice,
	top *ondatra.ATETopology,
) float32 {
	atePorts := sortPorts(ate.Ports())
	flow := ate.Traffic().NewFlow("flow-unviable")
	t.Log("Setting up base flow...")

	flow.WithSrcEndpoints(top.Interfaces()[atePorts[0].Name()])
	flow.WithDstEndpoints(top.Interfaces()["lag1"])

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()

	flow.WithHeaders(ethHeader, ipv4Header)
	flow.WithFrameRateFPS(100)
	flow.WithFrameSize(512)

	fmt.Print("TRAFFIC STARTED")
	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)
	lossPct := gnmi.Get(t, ate, gnmi.OC().Flow("flow-unviable").LossPct().State())
	t.Logf("Loss Packet %v ", lossPct)
	return lossPct
}

// testRPFO is the main function to test RPFO
func testRPFO(t *testing.T, dut *ondatra.DUTDevice) {

	client := gribi.Client{
		DUT:                   dut,
		FibACK:                *ciscoFlags.GRIBIFIBCheck,
		Persistence:           true,
		InitialElectionIDLow:  1,
		InitialElectionIDHigh: 0,
	}
	defer client.Close(t)
	if err := client.Start(t); err != nil {
		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		if err = client.Start(t); err != nil {
			t.Fatalf("gRIBI Connection could not be established: %v", err)
		}
	}
	// ctx := context.Background()

	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
		randomItems := client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	for i := 0; i < programming_RFPO; i++ {

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			rpfo(t, dut, &client, true)
		}
	}
}

// rpfo is the main function to test RPFO
func rpfo(t *testing.T, dut *ondatra.DUTDevice, client *gribi.Client, gribi_reconnect bool) {

	// reload the HW is rfpo count is 10 or more
	if rpfo_count == 10 {
		gnoiClient := dut.RawAPIs().GNOI(t)
		rebootRequest := &spb.RebootRequest{
			Method: spb.RebootMethod_COLD,
			Force:  true,
		}
		rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
		t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
		if err != nil {
			t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
		}
		rpfo_count = 0
		time.Sleep(time.Minute * 20)
	}
	// supervisor info
	var supervisors []string
	active_state := gnmi.OC().Component(active_rp).Name().State()
	active := gnmi.Get(t, dut, active_state)
	standby_state := gnmi.OC().Component(standby_rp).Name().State()
	standby := gnmi.Get(t, dut, standby_state)
	supervisors = append(supervisors, active, standby)

	// find active and standby RP
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, supervisors)
	t.Logf("Detected activeRP: %v, standbyRP: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch)

	// make sure standby RP is reach
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	for {
		if err := client.Start(t); err != nil {
			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		} else {
			t.Logf("gRIBI Connection established")
			switchoverRequest := &spb.SwitchControlProcessorRequest{
				ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
			}
			t.Logf("switchoverRequest: %v", switchoverRequest)
			switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
			if err != nil {
				t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
			}
			if err == nil {
				t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)
				want := rpStandbyBeforeSwitch
				got := ""
				if useNameOnly {
					got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
				} else {
					got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
				}
				if got != want {
					t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
				}
				break
			}
		}
		time.Sleep(time.Minute * 2)
	}

	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(900); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, dut, supervisors)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)
	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverTime().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", gnmi.Get(t, dut, activeRP.LastSwitchoverTime().State()))
	}

	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}

	// reestablishing gribi connection
	if gribi_reconnect {
		client.Start(t)
	}
}

// checks the interface status UP/DOWN
func checkIntfstatus(t *testing.T, args *testArgs, intf, expectstatus string) {
	stateBundleInterface := gnmi.Get(t, args.dut, gnmi.OC().Interface(intf).OperStatus().State()).String()
	if stateBundleInterface == expectstatus {
		t.Logf("The interface state is expected %v, got %v , want %v", intf, stateBundleInterface, expectstatus)
	} else {
		t.Errorf("The interface state is not expected %v, got %v , want %v", intf, stateBundleInterface, expectstatus)
	}
}

// shut/unshut the ports
func flapPorts(t *testing.T, args *testArgs, dutPorts []*ondatra.Port, flap bool) {
	for _, port := range dutPorts {
		bundleMember := port.Name()
		path := gnmi.OC().Interface(bundleMember).Enabled()
		gnmi.Update(t, args.dut, path.Config(), flap)
	}
}

// shut/unshut the interfaces
func flapInterface(t *testing.T, args *testArgs, intfs []string, flap bool) {
	for _, intf := range intfs {
		path := gnmi.OC().Interface(intf).Enabled()
		gnmi.Update(t, args.dut, path.Config(), flap)
	}
}

// configure the bundle interface
func ConfigBundleMember(t *testing.T, args *testArgs, bundle string, dutPorts []*ondatra.Port) {
	for _, port := range dutPorts {
		dutDest := generateBundleMemberInterfaceConfig(port.Name(), bundle)
		gnmi.Update(t, args.dut, gnmi.OC().Interface(port.Name()).Config(), dutDest)
	}
}

// unConfigure the bundle interface
func unConfigBundleMember(t *testing.T, args *testArgs, dutPorts []*ondatra.Port) {
	for _, port := range dutPorts {
		t.Logf("unConfigBundleMember under the interface - %v", port.Name())
		path := gnmi.OC().Interface(port.Name()).Ethernet().AggregateId()
		gnmi.Delete(t, args.dut, path.Config())
	}
}

// unconfigure the forwarding viable
func unConfigViable(t *testing.T, args *testArgs, dutPorts []*ondatra.Port) {
	for _, port := range dutPorts {
		path := gnmi.OC().Interface(port.Name()).ForwardingViable()
		gnmi.Delete(t, args.dut, path.Config())
	}
}

// configForwardingViable on DUT ports
func configForwardingViable(t *testing.T, dut *ondatra.DUTDevice, dutPorts []*ondatra.Port, forwardingViable []bool) {
	for index, port := range dutPorts {
		t.Logf("configForwardingViable Intf - %v and viable state - %v", port.Name(), forwardingViable[index])
		gnmi.Update(t, dut, gnmi.OC().Interface(port.Name()).ForwardingViable().Config(), forwardingViable[index])
	}
}

// checks the viable status of the ports
func checkViablestatus(t *testing.T, args *testArgs, dutPorts []*ondatra.Port, expectstatus []bool, trafficstate bool) {
	for index, port := range dutPorts {
		viablePath := gnmi.OC().Interface(port.Name()).ForwardingViable().State()
		viableState := gnmi.Get(t, args.dut, viablePath)
		// t.Logf("the viableState - %v", viableState)
		if viableState == expectstatus[index] {
			t.Logf("The interface viable state %v, got %v , want %v", port.Name(), viableState, expectstatus[index])
		} else {
			t.Errorf("Error: The interface viable state %v, got %v , want %v", port.Name(), viableState, expectstatus[index])
		}
	}

	lossPckt := testtrafficFlow(t, args.ate, args.top)
	if trafficstate {
		if lossPckt != 0 {
			t.Errorf("Error: Traffic Loss NOT Expected , Got %v , want 0", lossPckt)
		}
	} else {
		if lossPckt != 100 {
			t.Errorf("Error: Traffic Loss Expected , Got %v , want 100", lossPckt)
		}
	}
}

// checks the forwarding status of the interfaces
func checkForwardingClistatus(t *testing.T, args *testArgs, intf string, expectstatus bool) {
	cli := fmt.Sprintf("Show im database verbose interface %v", intf)
	showim := config.CMDViaGNMI(context.Background(), t, args.dut, cli)
	reTrue := regexp.MustCompile(`\s*Forwarding Viable\s*:\s*TRUE`)
	reFalse := regexp.MustCompile(`\s*Forwarding Viable\s*:\s*FALSE`)

	if expectstatus == viable {
		if !reTrue.MatchString(showim) {
			t.Error("Not matching Forwarding Viable : FALSE")
		}
	} else if expectstatus == unviable {
		if !reFalse.MatchString(showim) {
			t.Error("Not matching Forwarding Viable : TRUE")
		}
	} else {
		t.Error("No matching Forwarding Viable status found.")
	}
}

// checks the forwarding status of the interfaces
func checkCefCli(t *testing.T, args *testArgs, ip, ipaddress string) {
	cli := fmt.Sprintf("show cef %v %v", ip, ipaddress)
	t.Logf("CLI command : %v", cli)
	showcli := config.CMDViaGNMI(context.Background(), t, args.dut, cli)
	// Define a regular expression to match the word "drop"
	re := regexp.MustCompile(`(?i)\bdrop\b`)
	if !re.MatchString(showcli) {
		t.Error("CEF Drop Not found")
	}
}

func TestBundleForwardUnViable(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	bundlelist := []string{"Bundle-Ether1", "Bundle-Ether2"}
	aggID := bundlelist[1]
	t.Logf("aggID %v", aggID)
	configureDutLag(t, dut)
	ate := ondatra.ATE(t, "ate")
	top := configureAteLag(t, ate)
	be3 := []*ondatra.Port{dut.Port(t, "port4"), dut.Port(t, "port5")}
	be2 := []*ondatra.Port{dut.Port(t, "port2"), dut.Port(t, "port3")}
	t.Logf("be2 %v", be2)
	t.Logf("be3 %v", be3)
	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
	}
	top.Push(t).StartProtocols(t)

	// gnmi.Update(t, ate, gnmi.OC().Interface("port1").Enabled().Config(), false)

	t.Run("Validate traffic", func(t *testing.T) {
		lossPckt := testtrafficFlow(t, ate, top)
		if lossPckt != 0 {
			t.Errorf("Traffic Loss NOT Expected , Got %v , want 0", lossPckt)
		}
	})

	// verify an unviable bundle interface shows peer ipv4 address as drop adjacency in  CLI " show cef peer_ipaddress"
	t.Run("verify an unviable bundle interface shows peer ipv4 address as drop adjacency in  CLI show cef peer_ipaddress", func(t *testing.T) {

		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})
		checkForwardingClistatus(t, args, "Bundle-Ether2", unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)
		checkCefCli(t, args, "ipv4", atePort2.IPv4)
	})

	// With bundle interface having 2 members, mark one interface unviable and verify bundle interface is not marked as unviable
	t.Run("With bundle interface having 2 members, mark one interface unviable and verify bundle interface is not marked as unviable", func(t *testing.T) {
		viableStatus := []bool{unviable, unviable}
		var trafficvalidate bool
		for index, member := range be2 {
			t.Logf("Configure Forwarding UnViable member.Name() %v", member.Name())
			configForwardingViable(t, args.dut, be2[:index+1], viableStatus[:index+1])
			// checkForwardingstatus(t, args, "Bundle-Ether2", true)

			if index < 1 {
				trafficvalidate = true
			} else {
				trafficvalidate = false
			}
			t.Logf("check traffic status %v", trafficvalidate)
			checkForwardingClistatus(t, args, "Bundle-Ether2", trafficvalidate)
			checkViablestatus(t, args, be2[:index+1], viableStatus[:index+1], trafficvalidate)

			if int(index) >= 1 {
				t.Logf("bring back unviable to viable state and verify that bundle interface is not marked as unviable.")
				configForwardingViable(t, args.dut, be2, []bool{viable, viable})
				// checkForwardingstatus(t, args, "Bundle-Ether2", true)
				checkForwardingClistatus(t, args, "Bundle-Ether2", true)
				checkViablestatus(t, args, be2, []bool{viable, viable}, true)
			}
		}
	})

	// verify bundle interface admin state as well as viable state after removing and adding back unviable members.
	t.Run("verify bundle interface admin state as well as viable state after removing and adding back unviable members", func(t *testing.T) {

		// Configure Forwarding Viable
		t.Logf("Configure Forwarding UnViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		// Remove the Bundle member config
		t.Logf("Remove the Bundle member config")
		unConfigBundleMember(t, args, be2)
		checkForwardingClistatus(t, args, aggID, unviable)
		checkIntfstatus(t, args, aggID, "DOWN")
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		// Add the bundle member config
		t.Logf("Add the bundle member config")
		ConfigBundleMember(t, args, aggID, be2)
		time.Sleep(1 * time.Minute)
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)
	})

	// Verify bundle interface state stays unviable after deleting bundle interface and adding it back with unviable members.
	t.Run("Verify bundle interface state stays unviable after deleting bundle interface and adding it back with unviable members", func(t *testing.T) {

		// Configure Forwarding unViable
		t.Logf("Configure Forwarding unViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		// Delete the Bundle Interface
		t.Logf("Delete the Bundle Interface")
		config := gnmi.OC().Interface(aggID).Config()
		gnmi.Delete(t, dut, config)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		// Add the bundle Interface
		t.Logf("Add the bundle Interface back")
		outgoing := &oc.Interface{Name: ygot.String(aggID)}
		outgoingData := configInterfaceDUT(dut, outgoing, &dutPort2)
		g := outgoingData.GetOrCreateAggregation()
		g.LagType = oc.IfAggregate_AggregationType_LACP
		gnmi.Update(t, dut, gnmi.OC().Interface(*outgoing.Name).Config(), configInterfaceDUT(dut, outgoing, &dutPort2))
		time.Sleep(1 * time.Minute)

		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)
	})

	// Verify bundle interface state stays viable after deleting bundle interface and adding it back with viable members.
	t.Run("Verify bundle interface state stays viable after deleting bundle interface and adding it back with viable members", func(t *testing.T) {

		// Configure Forwarding Viable
		configForwardingViable(t, args.dut, be2, []bool{viable, viable})
		checkForwardingClistatus(t, args, aggID, viable)
		checkViablestatus(t, args, be2, []bool{viable, viable}, true)

		// Delete the Bundle Interface
		t.Logf("Delete the Bundle Interface")
		config := gnmi.OC().Interface(aggID).Config()
		gnmi.Delete(t, dut, config)
		checkViablestatus(t, args, be2, []bool{viable, viable}, false)

		// Add the bundle Interface
		t.Logf("Add the bundle Interface back")
		outgoing := &oc.Interface{Name: ygot.String(aggID)}
		outgoingData := configInterfaceDUT(dut, outgoing, &dutPort2)
		g := outgoingData.GetOrCreateAggregation()
		g.LagType = oc.IfAggregate_AggregationType_LACP
		gnmi.Update(t, dut, gnmi.OC().Interface(*outgoing.Name).Config(), configInterfaceDUT(dut, outgoing, &dutPort2))
		time.Sleep(1 * time.Minute)

		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, viable)
		checkViablestatus(t, args, be2, []bool{viable, viable}, true)
	})

	// Verify Bundle Interface state with unviable members stays unviable after router reload.
	t.Run("Verify bundle interface state with unviable members stays unviable after router reload", func(t *testing.T) {

		t.Logf("Configure Forwarding unViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})

		t.Logf("Validate Bundle status before router reload")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		client := gribi.Client{
			DUT:                   dut,
			FibACK:                *ciscoFlags.GRIBIFIBCheck,
			Persistence:           true,
			InitialElectionIDLow:  1,
			InitialElectionIDHigh: 0,
		}
		defer client.Close(t)
		if err := client.Start(t); err != nil {
			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
			if err = client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection could not be established: %v", err)
			}
		}

		time.Sleep(1 * time.Minute)
		gnoiClient := dut.RawAPIs().GNOI(t)
		_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot chassis without delay",
			Force:   true,
		})
		if err != nil {
			t.Fatalf("Reboot failed %v", err)
		}
		startReboot := time.Now()
		const maxRebootTime = 30
		t.Logf("Wait for DUT to boot up by polling the telemetry output.")
		for {
			var currentTime string
			t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

			time.Sleep(3 * time.Minute)
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			}); errMsg != nil {
				t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
			} else {
				t.Logf("Device rebooted successfully with received time: %v", currentTime)
				break
			}

			if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
				t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
			}
		}
		t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
		time.Sleep(30 * time.Second)

		t.Logf("Validate Bundle status after router reload")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

	})

	// Verify bundle interface state with unviable members stays unviable after reloding linecard hosting the member links.
	t.Run("Verify bundle interface state with unviable members stays unviable after reloding linecard hosting the member links", func(t *testing.T) {

		t.Logf("Configure Forwarding unViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})

		t.Logf("Validate Bundle status before LC reload")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		performance.ReloadLineCards(t, dut)

		t.Logf("Validate Bundle status after LC reload")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

	})

	// Verify bundle interface state with unviable members stays unviable after performing RPFO
	t.Run("Verify bundle interface state with unviable members stays unviable after performing RPFO", func(t *testing.T) {

		t.Logf("Configure Forwarding unViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})

		t.Logf("Validate Bundle status before RPFO")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		testRPFO(t, dut)
		time.Sleep(60 * time.Second)

		t.Log("Validate Bundle status after RPFO")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

	})

	// Verify bundle interface goes down when its all members viable and unviable are shutdown
	t.Run("Verify bundle interface goes down when its all members viable and unviable are shutdown", func(t *testing.T) {

		t.Logf("Configure Forwarding unViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})

		t.Log("Validate Bundle status - unViable before Flap the bundle members")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		flapPorts(t, args, be2, false)

		t.Log("Validate Bundle status - unViable after Flap the bundle members")
		checkIntfstatus(t, args, aggID, "DOWN")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		flapPorts(t, args, be2, true)
		time.Sleep(time.Minute * 1)

		t.Log("Validate Bundle status - unViable after Flap the bundle members")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

	})

	// Verify a viable bundle interface state does not change on shutting down bundle interface.
	// Verify changing all members state to unviable also makes an already shutdown bundle interface unviable.
	t.Run("Verify a viable bundle interface state does not change on shutting down bundle interface", func(t *testing.T) {

		t.Logf("Configure Forwarding Viable")
		configForwardingViable(t, args.dut, be2, []bool{viable, viable})

		t.Log("Validate Bundle status - Viable before Flap the bundle interface")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, viable)
		checkViablestatus(t, args, be2, []bool{viable, viable}, true)

		t.Log("shutdown the Bundle Interface")
		flapInterface(t, args, []string{aggID}, false)

		t.Log("Validate Bundle status - Viable after Flap the bundle interface")
		checkIntfstatus(t, args, aggID, "DOWN")
		checkForwardingClistatus(t, args, aggID, viable)
		checkViablestatus(t, args, be2, []bool{viable, viable}, false)

		t.Logf("Configure Forwarding unViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})

		t.Log("Validate Bundle status - unViable")
		checkIntfstatus(t, args, aggID, "DOWN")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		t.Log("UnShutdown the Bundle Interface")
		flapInterface(t, args, []string{aggID}, true)
		time.Sleep(time.Minute * 1)

		t.Log("Validate Bundle status - unViable")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		t.Logf("Configure Forwarding Viable")
		configForwardingViable(t, args.dut, be2, []bool{viable, viable})

		t.Log("Validate Bundle status - Viable")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, viable)
		checkViablestatus(t, args, be2, []bool{viable, viable}, true)

	})

	// Verify bundle interface status is up when 2 members are unviable and minimum-active links 1 is configured. Verify traffic is dropped
	// Verify bundle interface status is up when 2 members are unviable and minimum-active links 2 is configured. Verify traffic is dropped
	// Verify bundle interface status is down when 2 members are unviable and minimum-active links 3 is configured. Verify traffic is dropped
	t.Run("Verify bundle interface status is up when 2 members are unviable and minimum-active links is configured. Verify traffic is dropped", func(t *testing.T) {

		// Cleanup
		t.Cleanup(func() {
			gnmi.Delete(t, dut, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())
		})

		t.Logf("Configure Forwarding unViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})

		t.Log("Validate Bundle status - unViable before min active link config")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		// configure minimu active link under bundle
		var link uint16
		for link = 1; link <= 3; link++ {
			t.Run(fmt.Sprintf("configure minimum active link -%v under bundle", link), func(t *testing.T) {
				gnmi.Update(t, dut, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config(), link)

				t.Log("Validate Bundle status - unViable after min active link config")
				intfState := "UP"
				if link == 3 {
					intfState = "DOWN"
				}
				checkIntfstatus(t, args, aggID, intfState)
				checkForwardingClistatus(t, args, aggID, unviable)
				checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)
			})
		}
	})

	// Verify bundle interface status is up when 2 members are unviable and maximum-active links 1 is configured. Verify traffic is dropped
	// Verify bundle interface status is up when 2 members are unviable and maximum-active links 2 is configured. Verify traffic is dropped
	// Verify bundle interface status is down when 2 members are unviable and maximum-active links 3 is configured. Verify traffic is dropped
	t.Run("Verify bundle interface status is up when 2 members are unviable and maximum-active links is configured. Verify traffic is dropped", func(t *testing.T) {

		ctx := context.Background()
		// Cleanup
		t.Cleanup(func() {
			configToChange := fmt.Sprintf("int %v\nno bundle maximum-active links", aggID)
			util.GNMIWithText(ctx, t, dut, configToChange)
		})

		t.Logf("Configure Forwarding unViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})

		t.Log("Validate Bundle status - unViable before min active link config")
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		// configure maximum active link under bundle
		var link uint16
		for link = 1; link <= 3; link++ {
			t.Run(fmt.Sprintf("configure maximum active link -%v under bundle", link), func(t *testing.T) {
				configToChange := fmt.Sprintf("int be1\nbundle maximum-active links %d", link)
				util.GNMIWithText(ctx, t, dut, configToChange)

				t.Log("Validate Bundle status - unViable after min active link config")
				intfState := "UP"
				checkIntfstatus(t, args, aggID, intfState)
				checkForwardingClistatus(t, args, aggID, unviable)
				checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)
			})
		}
	})

	// Remove/add the unviable config and verify the interface forwarding status changes to unviable
	t.Run("Remove/add the unviable config and verify the interface forwarding status changes to unviable", func(t *testing.T) {

		// Configure Forwarding Viable
		t.Logf("Configure Forwarding UnViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})
		checkForwardingClistatus(t, args, aggID, unviable)
		checkIntfstatus(t, args, aggID, "UP")
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		// Remove the unviable config
		t.Logf("Remove the unviable config")
		unConfigViable(t, args, be2)
		checkForwardingClistatus(t, args, aggID, viable)
		checkIntfstatus(t, args, aggID, "UP")
		checkViablestatus(t, args, be2, []bool{viable, viable}, true)

		// Add the unviable config
		t.Logf("Add the unviable config")
		configForwardingViable(t, args.dut, be2, []bool{viable, viable})
		checkForwardingClistatus(t, args, aggID, viable)
		checkIntfstatus(t, args, aggID, "UP")
		checkViablestatus(t, args, be2, []bool{viable, viable}, true)
	})

	// Restart the process and verify that interface forwarding unviable status does not change and traffic continues to flow
	processes := []string{"bundlemgr_distrib", "bundlemgr_adj", "bundlemgr_local", "bundlemgr_check", "ifmgr", "rib_mgr"}
	for _, process := range processes {
		t.Run(fmt.Sprintf("Restart the process - %v and verify that interface forwarding unviable status does not change and traffic continues to flow", process), func(t *testing.T) {

			t.Logf("Configure Forwarding UnViable and validate traffic before process restart")
			configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})
			checkForwardingClistatus(t, args, aggID, unviable)
			checkIntfstatus(t, args, aggID, "UP")
			checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

			ctx := context.Background()
			client := gribi.Client{
				DUT:                   dut,
				FibACK:                *ciscoFlags.GRIBIFIBCheck,
				Persistence:           true,
				InitialElectionIDLow:  1,
				InitialElectionIDHigh: 0,
			}
			defer client.Close(t)
			if err := client.Start(t); err != nil {
				t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
				if err = client.Start(t); err != nil {
					t.Fatalf("gRIBI Connection could not be established: %v", err)
				}
			}

			config.CMDViaGNMI(ctx, t, dut, fmt.Sprintf("process restart %v", process))
			time.Sleep(time.Second * 10)
			for {
				if err := client.Start(t); err != nil {
					t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
				} else {
					t.Logf("gRIBI Connection established")
					break
				}
				time.Sleep(2 * time.Minute)
			}

			time.Sleep(30 * time.Second)

			t.Logf("validate traffic after process restart")
			checkForwardingClistatus(t, args, aggID, unviable)
			checkIntfstatus(t, args, aggID, "UP")
			checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)
		})
	}

	// Check ISIS adjacency is established with forwarding unviable until hold-down timer expires
	t.Run("Check ISIS adjacency is established with forwarding unviable until hold-down timer expires", func(t *testing.T) {
		// t.Skip()

		configIsis(t, dut, []string{"Bundle-Ether1", aggID})
		configAteRoutingProtocols(t, top)

		// Configure Forwarding Viable
		t.Logf("Configure Forwarding UnViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})
		checkForwardingClistatus(t, args, aggID, unviable)
		checkIntfstatus(t, args, aggID, "UP")
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		isisPath := gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).Isis()
		isisadjPath := isisPath.Interface(aggID).Level(uint8(ft.GetIsisLevelType(proto.Input_ISIS_level_2))).Adjacency("6401.0001.0000") //"6401.0001.0000"
		time.Sleep(120 * time.Second)

		t.Log("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/system-id")
		state := isisadjPath.SystemId()
		val := gnmi.Get(t, dut, state.State())
		if val != "6401.0001.0000" {
			t.Errorf("ISIS Adj SystemId: got %s, want %s", val, "6401.0001.0000")
		}
		t.Log("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-snpa")
		state1 := isisadjPath.NeighborSnpa()
		val = gnmi.Get(t, dut, state1.State())
		if val == "" {
			t.Errorf("ISIS Adj NeighborsSNPA: got %s, want !=%s", val, "''")
		}
		t.Log("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-status")
		state2 := isisadjPath.RestartStatus()
		val1 := gnmi.Get(t, dut, state2.State())
		if val1 != false {
			t.Errorf("ISIS Adj RestartStatus: got %v, want %v", val, false)
		}
		t.Log("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-support")
		state3 := isisadjPath.RestartSupport()
		val3 := gnmi.Get(t, dut, state3.State())
		if val3 != false {
			t.Errorf("ISIS Adj RestartSupport: got %v, want %v", val, false)
		}

		t.Log("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-suppress")
		state4 := isisadjPath.RestartSuppress()
		val4 := gnmi.Get(t, dut, state4.State())
		if val4 != false {
			t.Errorf("ISIS Adj RestartSuppress: got %v, want %v", val, false)
		}

		t.Log("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/multi-topology")
		state5 := isisadjPath.MultiTopology()
		val5 := gnmi.Get(t, dut, state5.State())
		if val5 != false {
			t.Errorf("ISIS Adj MultiTopology: got %v, want %v", val, false)
		}

		t.Log("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state")
		state6 := isisadjPath.AdjacencyState()
		val6 := gnmi.Get(t, dut, state6.State())
		if val6 != oc.Isis_IsisInterfaceAdjState_UP {
			t.Errorf("ISIS Adj State: got %v, want %v", val, oc.Isis_IsisInterfaceAdjState_UP)
		}

		t.Log("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-circuit-type")
		state7 := isisadjPath.NeighborCircuitType()
		val7 := gnmi.Get(t, dut, state7.State())
		if val7 != oc.Isis_LevelType_LEVEL_2 {
			t.Errorf("ISIS Adj NeighborCircuitType: got %v, want %v", val, oc.Isis_LevelType_LEVEL_2)
		}
	})

	// Verify when unviable member is attached to a bundle interface with all existing members as unviable, turns viable.
	// Verify when a viable member is removed from a bundle with remaining members as unviable, turns univable.
	t.Run("Verify when unviable/viable member is attached to a bundle interface with all existing members as unviable/viable, turns viable/unviable", func(t *testing.T) {
		// Configure Forwarding Viable
		t.Logf("Configure Forwarding Viable and UnViable")
		configForwardingViable(t, args.dut, be2, []bool{viable, unviable})
		checkForwardingClistatus(t, args, aggID, viable)
		checkIntfstatus(t, args, aggID, "UP")
		checkViablestatus(t, args, be2, []bool{viable, unviable}, true)

		// Existing the Bundle member
		forwardingState := []bool{unviable, viable}
		var trafficState bool
		var member []*ondatra.Port
		for _, state := range forwardingState {
			t.Logf("Existing the %v member", state)
			if state == viable {
				trafficState = false
				member = be2[:1]
			} else {
				trafficState = true
				member = be2[1:]
			}
			unConfigBundleMember(t, args, member)
			checkForwardingClistatus(t, args, aggID, state)
			checkIntfstatus(t, args, aggID, "UP")
			checkViablestatus(t, args, be2, []bool{viable, unviable}, trafficState)

			// Add the bundle member config
			t.Logf("Add the bundle member config")
			ConfigBundleMember(t, args, aggID, be2)
			time.Sleep(1 * time.Minute)
		}
	})

	// Verify when unviable member is attached to a shutdown bundle interface with all existing members as unviable, turns viable.
	// Verify when a viable member is removed from a shutdown bundle with remaining members as unviable, turns univable.
	t.Run("Verify when unviable/viable member is attached to a shutdown bundle interface with all existing members as unviable/viable, turns viable/unviable", func(t *testing.T) {
		// Configure Forwarding Viable
		t.Logf("Configure Forwarding Viable and UnViable")
		configForwardingViable(t, args.dut, be2, []bool{viable, unviable})
		checkForwardingClistatus(t, args, aggID, viable)

		t.Log("shutdown the Bundle Interface")
		flapInterface(t, args, []string{aggID}, false)

		checkIntfstatus(t, args, aggID, "DOWN")
		checkViablestatus(t, args, be2, []bool{viable, unviable}, false)

		// Existing the Bundle member
		forwardingState := []bool{unviable, viable}
		var trafficState bool
		var member []*ondatra.Port
		for _, state := range forwardingState {
			t.Logf("Existing the %v member", state)
			if state == viable {
				member = be2[:1]
				trafficState = false
			} else {
				member = be2[1:]
				trafficState = false
			}
			unConfigBundleMember(t, args, member)
			checkForwardingClistatus(t, args, aggID, state)
			checkIntfstatus(t, args, aggID, "DOWN")
			checkViablestatus(t, args, be2, []bool{viable, unviable}, trafficState)
		}

		t.Cleanup(func() {
			t.Log("Unshutdown the Bundle Interface")
			flapInterface(t, args, []string{aggID}, true)
			ConfigBundleMember(t, args, aggID, be2)
			time.Sleep(1 * time.Minute)
			checkIntfstatus(t, args, aggID, "UP")
		})
	})

	// Verify when all members are removed from a bundle, it goes unviable
	// Verify when a viable interface is added to a bundle without members, turns viable
	t.Run("Verify when all members are removed from a bundle, it goes unviable", func(t *testing.T) {
		// Configure Forwarding Viable
		t.Logf("Configure Forwarding UnViable")
		configForwardingViable(t, args.dut, be2, []bool{viable, viable})
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, viable)
		checkViablestatus(t, args, be2, []bool{viable, viable}, true)

		// Remove the Bundle member config
		t.Logf("Remove the Bundle member config")
		unConfigBundleMember(t, args, be2)
		checkForwardingClistatus(t, args, aggID, unviable)
		checkIntfstatus(t, args, aggID, "DOWN")
		// checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		// Add the bundle member config
		t.Logf("Add the bundle member config")
		ConfigBundleMember(t, args, aggID, be2)
		time.Sleep(1 * time.Minute)
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, viable)
		checkViablestatus(t, args, be2, []bool{viable, viable}, true)
	})

	// Verify a bundle interface without lacp goes unviable with all unviable members.
	t.Run("Verify a bundle interface without lacp goes unviable with all unviable members", func(t *testing.T) {
		// Configure passive mode under bundle interface
		outgoing := &oc.Interface{Name: ygot.String(aggID)}
		gnmi.Replace(t, dut, gnmi.OC().Interface(*outgoing.Name).Config(), configInterfaceDUT(dut, outgoing, &dutPort2))

		// Configure Forwarding Viable
		t.Logf("Configure Forwarding Viable")
		configForwardingViable(t, args.dut, be2, []bool{viable, viable})
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, viable)
		checkViablestatus(t, args, be2, []bool{viable, viable}, true)

		t.Logf("Configure Forwarding UnViable")
		configForwardingViable(t, args.dut, be2, []bool{unviable, unviable})
		checkIntfstatus(t, args, aggID, "UP")
		checkForwardingClistatus(t, args, aggID, unviable)
		checkViablestatus(t, args, be2, []bool{unviable, unviable}, false)

		t.Cleanup(func() {
			outgoing := &oc.Interface{Name: ygot.String(aggID)}
			outgoingData := configInterfaceDUT(dut, outgoing, &dutPort2)
			g := outgoingData.GetOrCreateAggregation()
			g.LagType = oc.IfAggregate_AggregationType_LACP
			gnmi.Update(t, dut, gnmi.OC().Interface(*outgoing.Name).Config(), configInterfaceDUT(dut, outgoing, &dutPort2))
		})
	})

	// With bundle3 having two unviable members and bundle2 having one unviable and one viable member, verify bundle3 interface stays unviable when the unviable member from bundle2 is added to it. verify bundle2 stays viable.
	// With bundle3 having two unviable members and bundle2 having one unviable member, verify bundle3 interface stays unviable when the unviable member from bundle2 is added to it. verify bundle2 goes unviable in the absence of a member
	// With bundle3 having two unviable members and bundle2 having one unviable and one unviable member, verify bundle3 interface goes viable when the viable member from bundle2 is added to it. verify bundle2 goes unviable in the absence of the viable member.
	t.Run("With bundle3 having two unviable members and bundle2 having one unviable and one viable member, verify bundle3 interface stays unviable when the unviable member from bundle2 is added to it. verify bundle2 stays viable", func(t *testing.T) {
		incoming := &oc.Interface{Name: ygot.String("Bundle-Ether3")}
		gnmi.Replace(t, dut, gnmi.OC().Interface(*incoming.Name).Config(), configInterfaceDUT(dut, incoming, &dutPort3))

		// configure bundle member to bundle1
		for _, port := range be3 {
			dutDest := generateBundleMemberInterfaceConfig(port.Name(), *incoming.Name)
			gnmi.Update(t, dut, gnmi.OC().Interface(port.Name()).Config(), dutDest)
		}

		t.Logf("Configure Forwarding unViable on Bundle-Ether3")
		configForwardingViable(t, args.dut, be3, []bool{unviable, unviable})
		checkIntfstatus(t, args, "Bundle-Ether3", "UP")
		checkForwardingClistatus(t, args, "Bundle-Ether3", unviable)
		checkViablestatus(t, args, be3, []bool{unviable, unviable}, true)

		t.Logf("Configure Forwarding Viable and unViable on Bundle-Ether2")
		configForwardingViable(t, args.dut, be2, []bool{viable, unviable})
		checkIntfstatus(t, args, "Bundle-Ether2", "UP")
		checkForwardingClistatus(t, args, "Bundle-Ether2", viable)
		checkViablestatus(t, args, be2, []bool{viable, unviable}, true)

		for index, port := range be2 {
			ConfigBundleMember(t, args, "Bundle-Ether3", []*ondatra.Port{port})
			if index == 0 {
				checkForwardingClistatus(t, args, "Bundle-Ether3", unviable)
				checkForwardingClistatus(t, args, "Bundle-Ether2", viable)
			} else {
				checkForwardingClistatus(t, args, "Bundle-Ether3", viable)
				checkForwardingClistatus(t, args, "Bundle-Ether2", unviable)
			}
			ConfigBundleMember(t, args, "Bundle-Ether2", be2)
		}

		t.Cleanup(func() {
			unConfigBundleMember(t, args, be3)
			t.Log("Unconfigure the Forwarding Viable")
			unConfigViable(t, args, be3)
		})

	})

	// With bundle3 having one viable member and bundle2 having one unviable and one unviable member, verify bundle3 interface stays viable when the viable member from bundle2 is added to it. verify bundle2 goes unviable in the absence of the viable member.
	// With bundle3 having one viable member and bundle2 having one unviable and one unviable member, verify bundle3 interface stays viable when the unviable member from bundle2 is added to it. verify bundle2 stays viable.
	t.Run("With bundle3 having one viable members and bundle2 having one unviable and one viable member, verify bundle3 interface stays unviable when the unviable member from bundle2 is added to it. verify bundle2 stays viable", func(t *testing.T) {

		incoming := &oc.Interface{Name: ygot.String("Bundle-Ether3")}
		gnmi.Replace(t, dut, gnmi.OC().Interface(*incoming.Name).Config(), configInterfaceDUT(dut, incoming, &dutPort3))

		// configure bundle member to bundle1
		for _, port := range be3 {
			dutDest := generateBundleMemberInterfaceConfig(port.Name(), *incoming.Name)
			gnmi.Update(t, dut, gnmi.OC().Interface(port.Name()).Config(), dutDest)
		}

		t.Logf("Configure Forwarding unViable on Bundle-Ether3")
		configForwardingViable(t, args.dut, be3, []bool{viable, unviable})
		checkIntfstatus(t, args, "Bundle-Ether3", "UP")
		checkForwardingClistatus(t, args, "Bundle-Ether3", viable)
		checkViablestatus(t, args, be3, []bool{viable, unviable}, true)

		t.Logf("Configure Forwarding Viable and unViable on Bundle-Ether2")
		configForwardingViable(t, args.dut, be2, []bool{viable, unviable})
		checkIntfstatus(t, args, "Bundle-Ether2", "UP")
		checkForwardingClistatus(t, args, "Bundle-Ether2", viable)
		checkViablestatus(t, args, be2, []bool{viable, unviable}, true)

		for index, port := range be2 {
			ConfigBundleMember(t, args, "Bundle-Ether3", []*ondatra.Port{port})
			if index == 0 {
				checkForwardingClistatus(t, args, "Bundle-Ether3", viable)
				checkForwardingClistatus(t, args, "Bundle-Ether2", unviable)
			} else {
				checkForwardingClistatus(t, args, "Bundle-Ether3", viable)
				checkForwardingClistatus(t, args, "Bundle-Ether2", viable)
			}
			ConfigBundleMember(t, args, "Bundle-Ether2", be2)
		}

		t.Cleanup(func() {
			unConfigBundleMember(t, args, be3)
			t.Log("Unconfigure the Forwarding Viable")
			unConfigViable(t, args, be3)
		})

	})
}
