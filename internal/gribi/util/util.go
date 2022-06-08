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

// CheckTrafficPassViaRate checks traffic stats via Rate statistics
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

// FlushServer flushes all the entries
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

// DoModifyOps modifies programming
func DoModifyOps(c *fluent.GRIBIClient, t testing.TB, ops []func(), wantACK fluent.ProgrammingResult, randomise bool, electionID uint64) []*client.OpResult {
	conn := c.Connection().WithRedundancyMode(fluent.ElectedPrimaryClient).WithInitialElectionID(electionID, 0).WithPersistence()

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
func GetIpv4Net(prefix string, maskLength int) string {
	_, ipv4Net, _ := net.ParseCIDR(prefix + "/" + strconv.Itoa(maskLength))
	return ipv4Net.String()
}

//CreateBundleInterface creates bundle interface
func CreateBundleInterface(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, bundleName string) {

	member := &telemetry.Interface{
		Ethernet: &telemetry.Interface_Ethernet{
			AggregateId: ygot.String(bundleName),
		},
	}
	updateResponse := dut.Config().Interface(interfaceName).Update(t, member)
	t.Logf("Update response : %v", updateResponse)
	SetInterfaceState(t, dut, bundleName, true)
}

//SetInterfaceState, sets interface state
func SetInterfaceState(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, adminState bool) {

	i := &telemetry.Interface{
		Enabled: ygot.Bool(adminState),
		Name:    ygot.String(interfaceName),
	}
	updateResponse := dut.Config().Interface(interfaceName).Update(t, i)
	t.Logf("Update response : %v", updateResponse)
	currEnabledState := dut.Telemetry().Interface(interfaceName).Get(t).GetEnabled()
	if currEnabledState != adminState {
		t.Fatalf("Failed to set interface admin_state to :%v", adminState)
	} else {
		t.Logf("Interface admin_state set to :%v", adminState)
	}
}

// FlapInterface, flaps an interface
func FlapInterface(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, flapDuration time.Duration) {

	initialState := dut.Telemetry().Interface(interfaceName).Get(t).GetEnabled()
	transientState := !initialState
	SetInterfaceState(t, dut, interfaceName, transientState)
	time.Sleep(flapDuration * time.Second)
	SetInterfaceState(t, dut, interfaceName, initialState)
}

//GetSubInterface, returns subinterface
func GetSubInterface(ipv4 string, prefixlen uint8, index uint32) *telemetry.Interface_Subinterface {
	s := &telemetry.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return s
}

//GetCopyOfIpv4SubInterfaces, returns subinterface ipv4 address
func GetCopyOfIpv4SubInterfaces(t *testing.T, dut *ondatra.DUTDevice, interfaceNames []string, index uint32) map[string]*telemetry.Interface_Subinterface {
	copiedSubInterfaces := make(map[string]*telemetry.Interface_Subinterface)
	for _, interfaceName := range interfaceNames {
		a := dut.Telemetry().Interface(interfaceName).Subinterface(index).Ipv4().Get(t)
		copiedSubInterfaces[interfaceName] = &telemetry.Interface_Subinterface{}
		ipv4 := copiedSubInterfaces[interfaceName].GetOrCreateIpv4()
		for _, ipval := range a.Address {
			t.Logf("*** Copying address: %v/%v for interface %s", ipval.GetIp(), ipval.GetPrefixLength(), interfaceName)
			ipv4addr := ipv4.GetOrCreateAddress(ipval.GetIp())
			ipv4addr.PrefixLength = ygot.Uint8(ipval.GetPrefixLength())
		}

	}
	return copiedSubInterfaces
}

// AddAteISISL2, appends ISIS configuration to ATETOPO obj
func AddAteISISL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaID, networkName string, metric uint32, prefix string, count uint32) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	network := intfs[atePort].AddNetwork(networkName)
	//IPReachabilityConfig :=
	network.ISIS().WithIPReachabilityMetric(metric + 1)
	network.IPv4().WithAddress(prefix).WithCount(count)
	intfs[atePort].ISIS().WithAreaID(areaID).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

// AddAteEBGPPeer, appends EBGP configuration to ATETOPO obj
func AddAteEBGPPeer(t *testing.T, topo *ondatra.ATETopology, atePort, peerAddress string, localAsn uint32, networkName, nexthop, prefix string, count uint32, useLoopback bool) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	//Add network instance
	network := intfs[atePort].AddNetwork(networkName)
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

// AddLoopback, adds loopback
func AddLoopback(t *testing.T, topo *ondatra.ATETopology, port, loopbackPrefix string) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].WithIPv4Loopback(loopbackPrefix)
}

// AddIpv4Network, adds Ipv4 Address
func AddIpv4Network(t *testing.T, topo *ondatra.ATETopology, port, networkName, addressCIDR string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].AddNetwork(networkName).IPv4().WithAddress(addressCIDR).WithCount(count)
}

// AddIpv6Network, adds Ipv4 Address
func AddIpv6Network(t *testing.T, topo *ondatra.ATETopology, port, networkName, addressCIDR string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].AddNetwork(networkName).IPv6().WithAddress(addressCIDR).WithCount(count)
}

// GetBoundedFlow, returns BoundedFlow
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

//GetIpv4Acl, returns Ipv4 ACL
func GetIpv4Acl(name string, sequenceID uint32, dscp, hopLimit uint8, action telemetry.E_Acl_FORWARDING_ACTION) *telemetry.Acl {

	acl := (&telemetry.Device{}).GetOrCreateAcl()
	aclSet := acl.GetOrCreateAclSet(name, telemetry.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry := aclSet.GetOrCreateAclEntry(sequenceID)
	aclEntryIpv4 := aclEntry.GetOrCreateIpv4()
	aclEntryIpv4.Dscp = ygot.Uint8(dscp)
	aclEntryIpv4.HopLimit = ygot.Uint8(hopLimit)
	aclEntryAction := aclEntry.GetOrCreateActions()
	aclEntryAction.ForwardingAction = action
	return acl
}

// AddIpv6Address, adds ipv6 address
func AddIpv6Address(ipv6 string, prefixlen uint8, index uint32) *telemetry.Interface_Subinterface {
	s := &telemetry.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv6()
	a := s4.GetOrCreateAddress(ipv6)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return s
}
