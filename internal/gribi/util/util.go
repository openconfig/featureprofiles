package util

import (
	"context"
	"encoding/binary"
	"math"
	"math/rand"
	"net"
	"strconv"
	"testing"
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

// GetIPPrefix returns the ip range with prefix
func GetIPPrefix(IPAddr string, i int, prefixLen string) string {
	ip := net.ParseIP(IPAddr)
	ip = ip.To4()
	ip[3] = ip[3] + byte(i%256)
	ip[2] = ip[2] + byte(i/256)
	ip[1] = ip[1] + byte(i/(256*256))
	return ip.String() + "/" + prefixLen
}

// CheckTrafficPassViaPortPktCounter checks traffic stats via port statistics
func CheckTrafficPassViaPortPktCounter(pktCounters []*telemetry.Interface_Counters, threshold ...float64) bool {
	thresholdValue := float64(0.99)
	if len(threshold) > 0 {
		thresholdValue = threshold[0]
	}
	totalIn := uint64(0)
	totalOut := uint64(0)

	for _, s := range pktCounters {
		totalIn = s.GetInPkts() + totalIn
		totalOut = s.GetOutPkts() + totalOut
	}
	return float64(totalIn)/float64(totalOut) >= thresholdValue
}

func CheckTrafficPassViaRate(stats []*telemetry.Flow) []string {
	lossFlow := []string{}
	for _, flow := range stats {
		// Tx Rate
		// Need to convert byte[] to float, then take the integer part
		txRate := int(math.Float32frombits(binary.BigEndian.Uint32(flow.OutFrameRate)))
		// Rx Rate
		// Need to convert byte[] to float, then take the integer part
		rxRate := int(math.Float32frombits(binary.BigEndian.Uint32(flow.InFrameRate)))

		if txRate-rxRate > 1 {
			lossFlow = append(lossFlow, *flow.Name)
		}
	}
	return lossFlow
}

// ReloadDUT reloads the router using GNMI APIs
func ReloadDUT(t *testing.T, dut *ondatra.DUTDevice) {
	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	time.Sleep(600 * time.Second)
}

// GNMIWithText applies the cisco text config using gnmi
func GNMIWithText(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, config string) {
	r := &gnmipb.SetRequest{
		Update: []*gnmipb.Update{
			{
				Path: &gnmipb.Path{Origin: "cli"},
				Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_AsciiVal{AsciiVal: config}},
			},
		},
	}
	_, err := dut.RawAPIs().GNMI().Default(t).Set(ctx, r)
	if err != nil {
		t.Errorf("There is error when applying the config")
	}
}

func FlushServer(c *fluent.GRIBIClient, t testing.TB) {
	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)

	t.Logf("Flush Entries in All Network Instances.")

	if _, err := c.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}
}

func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

func DoModifyOps(c *fluent.GRIBIClient, t testing.TB, ops []func(), wantACK fluent.ProgrammingResult, randomise bool, electionId uint64) []*client.OpResult {
	conn := c.Connection().WithRedundancyMode(fluent.ElectedPrimaryClient).WithInitialElectionID(electionId, 0).WithPersistence()

	if wantACK == fluent.InstalledInFIB {
		conn.WithFIBACK()
	}

	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("got unexpected error from server - session negotiation, got: %v, want: nil", err)
	}

	// If randomise is specified, we go and do the operations in a random order.
	// In this case, the caller MUST
	if randomise {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(ops), func(i, j int) { ops[i], ops[j] = ops[j], ops[i] })
	}

	for _, fn := range ops {
		fn()
	}

	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("got unexpected error from server - entries, got: %v, want: nil", err)
	}
	return c.Results(t)
}

//getIpv4Net returns network in CIDR format ("192.168.1.1/32", "192.168.1.0/24", "192.168.0.0/16")
func GetIpv4Net(prefix string, mask_length int) string {
	_, ipv4Net, _ := net.ParseCIDR(prefix + "/" + strconv.Itoa(mask_length))
	return ipv4Net.String()
}

func CreateBundleInterface(t *testing.T, dut *ondatra.DUTDevice, interface_name string, bundle_name string) {

	member := &telemetry.Interface{
		Ethernet: &telemetry.Interface_Ethernet{
			AggregateId: ygot.String(bundle_name),
		},
	}
	update_response := dut.Config().Interface(interface_name).Update(t, member)
	t.Logf("Update response : %v", update_response)
	SetInterfaceState(t, dut, bundle_name, true)
}

func SetInterfaceState(t *testing.T, dut *ondatra.DUTDevice, interface_name string, admin_state bool) {

	i := &telemetry.Interface{
		Enabled: ygot.Bool(admin_state),
		Name:    ygot.String(interface_name),
	}
	update_response := dut.Config().Interface(interface_name).Update(t, i)
	t.Logf("Update response : %v", update_response)
	currEnabledState := dut.Telemetry().Interface(interface_name).Get(t).GetEnabled()
	if currEnabledState != admin_state {
		t.Fatalf("Failed to set interface admin_state to :%v", admin_state)
	} else {
		t.Logf("Interface admin_state set to :%v", admin_state)
	}
}

func FlapInterface(t *testing.T, dut *ondatra.DUTDevice, interface_name string, flap_duration time.Duration) {

	initialState := dut.Telemetry().Interface(interface_name).Get(t).GetEnabled()
	transientState := !initialState
	SetInterfaceState(t, dut, interface_name, transientState)
	time.Sleep(flap_duration * time.Second)
	SetInterfaceState(t, dut, interface_name, initialState)
}

func GetSubInterface(ipv4 string, prefixlen uint8, index uint32) *telemetry.Interface_Subinterface {
	s := &telemetry.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return s
}

func GetCopyOfIpv4SubInterfaces(t *testing.T, dut *ondatra.DUTDevice, interface_names []string, index uint32) map[string]*telemetry.Interface_Subinterface {
	copiedSubInterfaces := make(map[string]*telemetry.Interface_Subinterface)
	for _, interface_name := range interface_names {
		a := dut.Telemetry().Interface(interface_name).Subinterface(index).Ipv4().Get(t)
		copiedSubInterfaces[interface_name] = &telemetry.Interface_Subinterface{}
		ipv4 := copiedSubInterfaces[interface_name].GetOrCreateIpv4()
		for _, ipval := range a.Address {
			t.Logf("*** Copying address: %v/%v for interface %s", ipval.GetIp(), ipval.GetPrefixLength(), interface_name)
			ipv4addr := ipv4.GetOrCreateAddress(ipval.GetIp())
			ipv4addr.PrefixLength = ygot.Uint8(ipval.GetPrefixLength())
		}

	}
	return copiedSubInterfaces
}

func AddAteISISL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaId, network_name string, metric uint32, prefix string, count uint32) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	network := intfs[atePort].AddNetwork(network_name)
	//IPReachabilityConfig :=
	network.ISIS().WithIPReachabilityMetric(metric + 1)
	network.IPv4().WithAddress(prefix).WithCount(count)
	intfs[atePort].ISIS().WithAreaID(areaId).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

func AddAteEBGPPeer(t *testing.T, topo *ondatra.ATETopology, atePort, peerAddress string, localAsn uint32, network_name, nexthop, prefix string, count uint32, useLoopback bool) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	//Add network instance
	network := intfs[atePort].AddNetwork(network_name)
	bgpAttribute := network.BGP()
	bgpAttribute.WithActive(true).WithNextHopAddress(nexthop)
	//Add prefixes
	network.IPv4().WithAddress(prefix).WithCount(count)
	//Create BGP instance
	bgp := intfs[atePort].BGP()
	bgpPeer := bgp.AddPeer().WithPeerAddress(peerAddress).WithLocalASN(localAsn).WithTypeExternal()
	bgpPeer.WithOnLoopback(useLoopback)

	//Update bgpCapabilities
	bgpPeer.Capabilities().WithIPv4UnicastEnabled(true).WithIPv6UnicastEnabled(true).WithGracefulRestart(true)
}

func AddLoopback(t *testing.T, topo *ondatra.ATETopology, port, loopback_prefix string) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].WithIPv4Loopback(loopback_prefix)
}
func AddIpv4Network(t *testing.T, topo *ondatra.ATETopology, port, network_name, address_CIDR string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].AddNetwork(network_name).IPv4().WithAddress(address_CIDR).WithCount(count)
}
func AddIpv6Network(t *testing.T, topo *ondatra.ATETopology, port, network_name, address_CIDR string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].AddNetwork(network_name).IPv6().WithAddress(address_CIDR).WithCount(count)
}

func GetBoundedFlow(t *testing.T, ate *ondatra.ATEDevice, topo *ondatra.ATETopology, srcPort, dstPort, srcNetwork, dstNetwork, flowName string, dscp uint8, ttl ...uint8) *ondatra.Flow {

	intfs := topo.Interfaces()
	flow := ate.Traffic().NewFlow(flowName)
	t.Logf("Setting up flow -> %s", flowName)
	networks1 := intfs[srcPort].Networks()
	networks2 := intfs[dstPort].Networks()
	ethheader := ondatra.NewEthernetHeader()
	ipheader1 := ondatra.NewIPv4Header().WithDSCP(dscp)
	if len(ttl) > 0 {
		ipheader1.WithTTL(ttl[0])
	}
	flow.WithHeaders(ethheader, ipheader1)
	flow.WithSrcEndpoints(networks1[srcNetwork])
	flow.WithDstEndpoints(networks2[dstNetwork])
	flow.WithFrameRateFPS(100)
	flow.WithFrameSize(1024)
	return flow
}

func GetIpv4Acl(name string, sequenceId uint32, dscp, hopLimit uint8, action telemetry.E_Acl_FORWARDING_ACTION) *telemetry.Acl {

	acl := (&telemetry.Device{}).GetOrCreateAcl()
	aclSet := acl.GetOrCreateAclSet(name, telemetry.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry := aclSet.GetOrCreateAclEntry(sequenceId)
	aclEntryIpv4 := aclEntry.GetOrCreateIpv4()
	aclEntryIpv4.Dscp = ygot.Uint8(dscp)
	aclEntryIpv4.HopLimit = ygot.Uint8(hopLimit)
	aclEntryAction := aclEntry.GetOrCreateActions()
	aclEntryAction.ForwardingAction = action
	return acl
}

func AddIpv6Address(ipv6 string, prefixlen uint8, index uint32) *telemetry.Interface_Subinterface {
	s := &telemetry.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv6()
	a := s4.GetOrCreateAddress(ipv6)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return s
}
