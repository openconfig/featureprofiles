// Package util provides util APIs to simplify writing  test cases.
package util

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/args"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	PTISIS         = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	DUTAreaAddress = "47.0001"
	DUTSysID       = "0000.0000.0001"
	ISISName       = "osiris"
	pLen4          = 30
	pLen6          = 126
	PTBGP          = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	BGPAS          = 65000
)

// used by SupervisorSwitchover to test Supervisor Switchover
const (
	maxSwitchoverTime = 900
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

// FlapInterface flaps Interface and check State
func FlapInterface(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, flapDuration time.Duration, intftype ...oc.E_IETFInterfaces_InterfaceType) {

	initialState := gnmi.Get(t, dut, gnmi.OC().Interface(interfaceName).State()).GetEnabled()
	transientState := !initialState
	if len(intftype) != 0 {
		SetInterfaceState(t, dut, interfaceName, transientState, intftype[0])
	} else {
		SetInterfaceState(t, dut, interfaceName, transientState)
	}
	time.Sleep(flapDuration * time.Second)
	if len(intftype) != 0 {
		SetInterfaceState(t, dut, interfaceName, initialState, intftype[0])
	} else {
		SetInterfaceState(t, dut, interfaceName, initialState)
	}
}

// SetInterfaceState sets interface adminState
func SetInterfaceState(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, adminState bool, intftype ...oc.E_IETFInterfaces_InterfaceType) {

	i := &oc.Interface{
		Enabled: ygot.Bool(adminState),
		Name:    ygot.String(interfaceName),
	}
	if len(intftype) != 0 {
		i = &oc.Interface{
			Enabled: ygot.Bool(adminState),
			Name:    ygot.String(interfaceName),
			Type:    intftype[0],
		}
	}
	updateResponse := gnmi.Update(t, dut, gnmi.OC().Interface(interfaceName).Config(), i)
	t.Logf("Update response : %v", updateResponse)
	currEnabledState := gnmi.Get(t, dut, gnmi.OC().Interface(interfaceName).Enabled().State())
	if currEnabledState != adminState {
		t.Fatalf("Failed to set interface adminState to :%v", adminState)
	} else {
		t.Logf("Interface adminState set to :%v", adminState)
	}
}

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
func CheckTrafficPassViaPortPktCounter(pktCounters []*oc.Interface_Counters, threshold ...float64) bool {
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
func CheckTrafficPassViaRate(stats []*oc.Flow) []string {
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
	_, err := dut.RawAPIs().GNMI(t).Set(ctx, r)
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

// GetIpv4Net returns network in CIDR format ("192.168.1.1/32", "192.168.1.0/24", "192.168.0.0/16")
func GetIpv4Net(prefix string, maskLength int) string {
	_, ipv4Net, _ := net.ParseCIDR(prefix + "/" + strconv.Itoa(maskLength))
	return ipv4Net.String()
}

// CreateBundleInterface creates bundle interface
func CreateBundleInterface(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, bundleName string) {

	member := &oc.Interface{
		Ethernet: &oc.Interface_Ethernet{
			AggregateId: ygot.String(bundleName),
		},
	}
	updateResponse := gnmi.Update(t, dut, gnmi.OC().Interface(interfaceName).Config(), member)
	t.Logf("Update response : %v", updateResponse)
	SetInterfaceState(t, dut, bundleName, true)
}

// GetSubInterface returns subinterface
func GetSubInterface(ipv4 string, prefixlen uint8, index uint32) *oc.Interface_Subinterface {
	s := &oc.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return s
}

// GetCopyOfIpv4SubInterfaces returns subinterface ipv4 address
func GetCopyOfIpv4SubInterfaces(t *testing.T, dut *ondatra.DUTDevice, interfaceNames []string, index uint32) map[string]*oc.Interface_Subinterface {
	copiedSubInterfaces := make(map[string]*oc.Interface_Subinterface)
	for _, interfaceName := range interfaceNames {
		a := gnmi.Get(t, dut, gnmi.OC().Interface(interfaceName).Subinterface(index).Ipv4().State())
		copiedSubInterfaces[interfaceName] = &oc.Interface_Subinterface{}
		ipv4 := copiedSubInterfaces[interfaceName].GetOrCreateIpv4()
		for _, ipval := range a.Address {
			t.Logf("*** Copying address: %v/%v for interface %s", ipval.GetIp(), ipval.GetPrefixLength(), interfaceName)
			ipv4addr := ipv4.GetOrCreateAddress(ipval.GetIp())
			ipv4addr.PrefixLength = ygot.Uint8(ipval.GetPrefixLength())
		}

	}
	return copiedSubInterfaces
}

// GetInterface returns subinterface
func GetInterface(interfaceName string, ipv4 string, prefixlen uint8, index uint32) *oc.Interface {
	i := &oc.Interface{Type: oc.IETFInterfaces_InterfaceType_ieee8023adLag, Enabled: ygot.Bool(true),
		Name: ygot.String(interfaceName)}
	s := i.GetOrCreateSubinterface(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return i
}

// GetCopyOfIpv4Interfaces returns subinterface ipv4 address
func GetCopyOfIpv4Interfaces(t *testing.T, dut *ondatra.DUTDevice, interfaceNames []string, index uint32) map[string]*oc.Interface {
	copiedSubInterfaces := make(map[string]*oc.Interface)
	for _, interfaceName := range interfaceNames {
		a := gnmi.Get(t, dut, gnmi.OC().Interface(interfaceName).Subinterface(index).Ipv4().State())
		copiedSubInterfaces[interfaceName] = &oc.Interface{}
		copiedSubInterfaces[interfaceName].Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		copiedSubInterfaces[interfaceName].Enabled = ygot.Bool(true)
		copiedSubInterfaces[interfaceName].Name = ygot.String(interfaceName)
		ipv4 := copiedSubInterfaces[interfaceName].GetOrCreateSubinterface(index).GetOrCreateIpv4()
		for _, ipval := range a.Address {
			t.Logf("*** Copying address: %v/%v for interface %s", ipval.GetIp(), ipval.GetPrefixLength(), interfaceName)
			ipv4addr := ipv4.GetOrCreateAddress(ipval.GetIp())
			ipv4addr.PrefixLength = ygot.Uint8(ipval.GetPrefixLength())
		}

	}
	return copiedSubInterfaces
}

// AddAteISISL2 appends ISIS configuration to ATETOPO obj
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

// AddAteEBGPPeer appends EBGP configuration to ATETOPO obj
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

// AddLoopback adds loopback
func AddLoopback(t *testing.T, topo *ondatra.ATETopology, port, loopbackPrefix string) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].WithIPv4Loopback(loopbackPrefix)
}

// AddIpv4Network adds Ipv4 Address
func AddIpv4Network(t *testing.T, topo *ondatra.ATETopology, port, networkName, addressCIDR string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].AddNetwork(networkName).IPv4().WithAddress(addressCIDR).WithCount(count)
}

// AddIpv6Network adds Ipv4 Address
func AddIpv6Network(t *testing.T, topo *ondatra.ATETopology, port, networkName, addressCIDR string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].AddNetwork(networkName).IPv6().WithAddress(addressCIDR).WithCount(count)
}

// GetBoundedFlow returns BoundedFlow
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

// GetIpv4Acl returns Ipv4 ACL
func GetIpv4Acl(name string, sequenceID uint32, dscp, hopLimit uint8, action oc.E_Acl_FORWARDING_ACTION) *oc.Acl {

	acl := (&oc.Root{}).GetOrCreateAcl()
	aclSet := acl.GetOrCreateAclSet(name, oc.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry := aclSet.GetOrCreateAclEntry(sequenceID)
	aclEntryIpv4 := aclEntry.GetOrCreateIpv4()
	aclEntryIpv4.Dscp = ygot.Uint8(dscp)
	aclEntryIpv4.HopLimit = ygot.Uint8(hopLimit)
	aclEntryAction := aclEntry.GetOrCreateActions()
	aclEntryAction.ForwardingAction = action
	return acl
}

// AddIpv6Address adds ipv6 address
func AddIpv6Address(ipv6 string, prefixlen uint8, index uint32) *oc.Interface_Subinterface {
	s := &oc.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv6()
	a := s4.GetOrCreateAddress(ipv6)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return s
}

// FaultInjectionMechanism injects faults on a line card for a given component name and fault-point number
// lcnumber takes linecard numbers to be given as a list []string{"0", "1"}
// componentName specifies the component on which the fault point is injected eg : ofa_la_srv
// faultPointNumber speicifes the fault point eg : 3 indicates IPV4_ROUTE_RDESC_OOR
// returnValue specifies perticluar error to simulate eg : 3482356236 indicates Route programming failure
// to activate fault point use true and to deactivate use false
func FaultInjectionMechanism(t *testing.T, dut *ondatra.DUTDevice, lcNumber []string, componentName string, faultPointNumber string, returnValue string, activate bool) {
	cliHandle := dut.RawAPIs().CLI(t)
	for _, lineCard := range lcNumber {
		var fimActivate string
		var fimDeactivate string
		if activate {
			fimActivate = fmt.Sprintf("run ssh -oStrictHostKeyChecking=no 172.0.%s.1 /pkg/bin/fim_cli -c %s -a %s:%s", lineCard, componentName, faultPointNumber, returnValue)
			t.Logf("The fim activate string %v", fimActivate)
			fimRes, err := cliHandle.RunCommand(context.Background(), fimActivate)
			time.Sleep(60 * time.Second)
			if strings.Contains(fimRes.Output(), fmt.Sprintf("Enabling FP#%s", faultPointNumber)) {
				t.Logf("Successfull Injected Fault for component %v on fault number %v", componentName, faultPointNumber)
			} else {
				t.Fatalf("FaultPointNumber for component %v on faultnumber %v not enabled", componentName, faultPointNumber)
			}
			if err != nil {
				t.Fatalf("Error while sending enable fault point %v", err)
			}
			t.Logf("The fim actvate result %v", fimRes)
		} else {
			fimDeactivate = fmt.Sprintf("run ssh -oStrictHostKeyChecking=no 172.0.%s.1 /pkg/bin/fim_cli -c %s -r %s:%s", lineCard, componentName, faultPointNumber, returnValue)
			t.Logf("The fim deactivate string %v", fimDeactivate)
			fimRes, err := cliHandle.RunCommand(context.Background(), fimDeactivate)
			time.Sleep(60 * time.Second)
			if strings.Contains(fimRes.Output(), fmt.Sprintf("Disabling FP#%s", faultPointNumber)) {
				t.Logf("Successfull Disabled Injected Fault for component %v on fault number %v", componentName, faultPointNumber)
			} else {
				t.Fatalf("FaultPointNumber for component %v on faultnumber %v not disabled", componentName, faultPointNumber)
			}
			if err != nil {
				t.Fatalf("Error while sending disable fault point %v", err)
			}
			t.Logf("The fim deactivate result %v", fimRes)
		}

	}

}

// addISISOC, configures ISIS on DUT
func AddISISOC(t *testing.T, dut *ondatra.DUTDevice, ifaceName string) {
	t.Helper()

	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(PTISIS, ISISName)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Net = []string{fmt.Sprintf("%v.%v.00", DUTAreaAddress, DUTSysID)}
	glob.LevelCapability = 2
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf := isis.GetOrCreateInterface(ifaceName)
	intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intf.Enabled = ygot.Bool(true)
	intf.HelloPadding = 1
	intf.Passive = ygot.Bool(false)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = 2

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(PTISIS, ISISName)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(PTISIS, ISISName)
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

// addBGPOC, configures ISIS on DUT
func AddBGPOC(t *testing.T, dut *ondatra.DUTDevice, neighbor string) {
	t.Helper()

	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(PTBGP, *ciscoFlags.DefaultNetworkInstance)
	bgp := prot.GetOrCreateBgp()
	glob := bgp.GetOrCreateGlobal()
	glob.As = ygot.Uint32(BGPAS)
	glob.RouterId = ygot.String("1.1.1.1")
	glob.GetOrCreateGracefulRestart().Enabled = ygot.Bool(true)
	glob.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup("BGP-PEER-GROUP")
	pg.PeerAs = ygot.Uint32(64001)
	pg.LocalAs = ygot.Uint32(63001)
	pg.PeerGroupName = ygot.String("BGP-PEER-GROUP")

	peer := bgp.GetOrCreateNeighbor(neighbor)
	peer.PeerGroup = ygot.String("BGP-PEER-GROUP")
	peer.GetOrCreateEbgpMultihop().Enabled = ygot.Bool(true)
	peer.GetOrCreateEbgpMultihop().MultihopTtl = ygot.Uint8(255)
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(PTBGP, *ciscoFlags.DefaultNetworkInstance)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(PTBGP, *ciscoFlags.DefaultNetworkInstance)
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

func PrettyPrintJson(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// load oc from a file
func LoadJsonFileToOC(t *testing.T, path string) *oc.Root {
	var ocRoot oc.Root
	jsonConfig, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Cannot load base config: %v", err)
	}
	opts := []ytypes.UnmarshalOpt{
		&ytypes.PreferShadowPath{},
	}
	if err := oc.Unmarshal(jsonConfig, &ocRoot, opts...); err != nil {
		t.Fatalf("Cannot unmarshal base config: %v", err)
	}
	return &ocRoot
}

// SliceEqual checks if two slices of strings contain the same elements in any order.
// It returns true if both slices have the same elements with the same frequencies (counts),
// otherwise it returns false. The function does not modify the input slices.
func SliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	counts := make(map[string]int)
	// count the occurrences of each string in the first slice
	for _, v := range a {
		counts[v]++
	}

	for _, v := range b {
		// if we find a string in the second slice that is not in the map or the count goes below zero, we know the slices are not equal and return false
		if count, ok := counts[v]; !ok || count == 0 {
			return false
		}
		counts[v]--
	}

	return true
}

// UniqueValues returns a list of all unique values from a given input map.
func UniqueValues(t *testing.T, m map[string]string) []string {
	seen := make(map[string]bool) // a set of seen values
	var result []string           // a slice to hold unique values

	for _, value := range m {
		if _, ok := seen[value]; !ok {
			// If the value hasn't been seen yet, add it to the result slice
			result = append(result, value)
			// And mark it as seen
			seen[value] = true
		}
	}
	return result
}

// GetLCList returns a list of LCs on the device
func GetLCList(t *testing.T, dut *ondatra.DUTDevice) []string {
	lcList := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
	t.Logf("List of linecard on device: %v", lcList)
	return lcList
}

// GetLCSlotID returns the LC slot ID on the device for a location.
func GetLCSlotID(t *testing.T, lcloc string) uint8 {
	if strings.Contains(lcloc, "RP") {
		return 0
	}
	lcSl := strings.Split(lcloc, "/")
	lcslotID, err := strconv.Atoi(lcSl[1])
	if err != nil {
		t.Fatalf("error in int conversion %v", err)
	}
	return uint8(lcslotID)
}

// StringToInt converts int values in string format to int.
func StringToInt(t *testing.T, intString string) int {
	intVal, err := strconv.Atoi(intString)
	if err != nil {
		t.Fatalf("error in int conversion %v", err)
	}
	return intVal
}

// ReloadLinecards reloads linecards, passed as list in the argument, on the device.
func ReloadLinecards(t *testing.T, lcList []string) {
	const linecardBoottime = 5 * time.Minute
	dut := ondatra.DUT(t, "dut")

	gnoiClient := dut.RawAPIs().GNOI(t)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method:        spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{},
	}

	req := &spb.RebootStatusRequest{
		Subcomponents: []*tpb.Path{},
	}

	for _, lc := range lcList {
		rebootSubComponentRequest.Subcomponents = append(rebootSubComponentRequest.Subcomponents, components.GetSubcomponentPath(lc, false))
		req.Subcomponents = append(req.Subcomponents, components.GetSubcomponentPath(lc, false))
	}

	t.Logf("Reloading linecards: %v", lcList)
	startTime := time.Now()
	_, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
	}

	rebootDeadline := startTime.Add(linecardBoottime)
	for retry := true; retry; {
		t.Log("Waiting for 10 seconds before checking linecard status.")
		time.Sleep(10 * time.Second)
		if time.Now().After(rebootDeadline) {
			retry = false
			break
		}
		resp, err := gnoiClient.System().RebootStatus(context.Background(), req)
		switch {
		case status.Code(err) == codes.Unimplemented:
			t.Fatalf("Unimplemented RebootStatus RPC: %v", err)
		case err == nil:
			retry = resp.GetActive()
		default:
			// any other error just sleep.
		}
	}
	t.Logf("It took %v minutes to reboot linecards.", time.Since(startTime).Minutes())
}

// RebootDevice reboots the device gracefully and waits for the device to come back up.
func RebootDevice(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	const (
		// Delay to allow router complete system backup and start rebooting
		pollingDelay = 180 * time.Second
		// Maximum reboot time is 900 seconds (15 minutes).
		maxRebootTime = 900
	)

	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Message: "Reboot chassis with cold method gracefully",
		Force:   false,
	}

	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	if err != nil {
		t.Fatalf("Failed parsing current-datetime: %s", err)
	}

	t.Logf("Send reboot request: %v", rebootRequest)
	startReboot := time.Now()
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	defer gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
	t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}

	t.Logf("Wait for the device to gracefully complete system backup and start rebooting.")
	time.Sleep(pollingDelay)

	t.Logf("Check if router has booted by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since reboot started.", time.Since(startReboot).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Seconds()) > maxRebootTime {
			t.Errorf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f seconds", time.Since(startReboot).Seconds())
}

type InterfacePhysicalLink struct {
	Intf               *oc.Interface
	IntfName           string
	LineCardNumber     string
	PeerIntfName       string
	PeerIntf           *oc.Interface
	PeerLineCardNumber string
}

// Assumptions
// dut is connected only to peer (no other lldp device in the topology)
// no  bundle is configured before calling this function
func ConfigureBundleIntfDynamic(t *testing.T, dut *ondatra.DUTDevice, peer *ondatra.DUTDevice, memberCount int) map[string][]InterfacePhysicalLink {

	if !enableLldp(t, dut) {
		t.Fatalf("LLDP configuration for device %s failed", dut.Name())
	}
	if !enableLldp(t, peer) {
		t.Fatalf("LLDP configuration for device %s failed", peer.Name())
	}

	dutInterfaces := getAllInterfaces(t, dut)
	peerInterfaces := getAllInterfaces(t, peer)

	// Enable all dut and peer interfaces
	// DUT
	if !enableInterfacesAll(t, dut, dutInterfaces) {
		t.Fatalf("Failed to enable all interfaces for device %s", dut.Name())
	}
	// PEER
	if !enableInterfacesAll(t, peer, peerInterfaces) {
		t.Fatalf("Failed to enable all interfaces for device %s", peer.Name())
	}

	// Enable LLDP on all DUT and PEER interfaces
	// DUT
	if !enableInterfaceLldp(t, dut, dutInterfaces) {
		t.Fatalf("Failed to enable LLDP on all interfaces for device %s", dut.Name())
	}
	// PEER
	if !enableInterfaceLldp(t, peer, peerInterfaces) {
		t.Fatalf("Failed to enable LLDP on all interfaces for device %s", peer.Name())
	}

	// Get only enabled interfaces of DUT
	dutEnabledInterfaces := getEnabledInterfaces(dutInterfaces)
	linkInfos := make([]InterfacePhysicalLink, 0)

	lldpIntfStatePathAny := gnmi.OC().Lldp().InterfaceAny().State()
	lldpIntfStateAny := gnmi.GetAll(t, dut, lldpIntfStatePathAny)

	peerIntfPathAny := gnmi.OC().InterfaceAny().State()
	peerIntfAny := gnmi.GetAll(t, peer, peerIntfPathAny)

	// logic to create the link by using LLDP neighbour
	// re := regexp.MustCompile(`\d`)
	re := regexp.MustCompile(`\d+/(\d+)/\d+/\d+`)
	for _, dutIntf := range dutEnabledInterfaces {
		intfName := dutIntf.GetName()
		// lcNumber := re.FindString(intfName)
		matches := re.FindStringSubmatch(intfName)
		var lcNumber string
		if len(matches) > 1 {
			lcNumber = matches[1]
		}

		// Get the peer interface name using LLDP data
		peerIntfName := getPeerInterfaceName(lldpIntfStateAny, intfName)

		// TODO logic to fectch the NPU
		// npu := getNpu(t,dut,intfName)
		// PeerNpu := getNpu(t,peer,peerIntfName)

		if peerIntfName != "" && strings.Contains(peerIntfName, "Gig") {
			// Retrieve the peer interface state
			// peerIntfPath := gnmi.OC().Interface(peerIntfName).State()
			// peerIntf := gnmi.Get(t, peer, peerIntfPath)
			var peerIntf *oc.Interface
			for _, intf := range peerIntfAny {
				if intf.GetName() == peerIntfName {
					peerIntf = intf
					break
				}
			}

			if peerIntf != nil {
				// peerLcNumber := re.FindString(peerIntfName)
				peerMatches := re.FindStringSubmatch(peerIntfName)
				var peerLcNumber string
				if len(peerMatches) > 1 {
					peerLcNumber = peerMatches[1]
				}
				linkInfo := InterfacePhysicalLink{
					Intf:           dutIntf,
					IntfName:       intfName,
					LineCardNumber: lcNumber,
					// Npu:				npu,
					PeerIntfName:       peerIntfName,
					PeerIntf:           peerIntf,
					PeerLineCardNumber: peerLcNumber,
					// PeerNpu:		  	peerNpu,
				}
				linkInfos = append(linkInfos, linkInfo)
			}
		}
	}

	// Sort the linkInfos based on LineCardNumber
	sortedLinkInfos := sortLinkInfosByLineCardNumber(linkInfos)

	// Count the number of InterfacePhysicalLink
	linkCount := len(sortedLinkInfos)

	// Calculate the number of bundles
	numBundles := linkCount / memberCount
	if linkCount%memberCount != 0 {
		numBundles++
	}

	// Create the map with keys as bundle names and values as slices of InterfacePhysicalLink
	bundleMap := make(map[string][]InterfacePhysicalLink)
	for i := 0; i < numBundles; i++ {
		bundleName := fmt.Sprintf("Bundle-Ether%d", 100+i)
		bundleMap[bundleName] = []InterfacePhysicalLink{}
	}

	// Distribute the sortedLinkInfos into bundles in a round-robin fashion
	for i, link := range sortedLinkInfos {
		bundleName := fmt.Sprintf("Bundle-Ether%d", 100+(i%numBundles))
		bundleMap[bundleName] = append(bundleMap[bundleName], link)
	}

	fmt.Printf("Total Links: %d, Member Count: %d, Number of Bundles: %d\n", linkCount, memberCount, numBundles)

	for bundleName, links := range bundleMap {
		fmt.Printf("Bundle: %s\n", bundleName)
		for _, link := range links {
			fmt.Printf("  DUT Interface: %v, DUT LC: %v, Peer Interface: %v, Peer LC: %v\n",
				link.IntfName, link.LineCardNumber, link.PeerIntfName, link.PeerLineCardNumber)
		}
	}

	// Create bundles in both DUT and Peer devices
	createBundles(t, dut, peer, bundleMap)

	return bundleMap
}

func configureBundle(ocRoot *oc.Root, bundleName string, bundleIP net.IP) {
	bundle := ocRoot.GetOrCreateInterface(bundleName)
	bundle.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	bundle.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	bundle.Enabled = ygot.Bool(true)
	// subIntf := bundle.GetOrCreateSubinterface(0)
	// //  subIntf.Enabled = ygot.Bool(true)  // deviation
	// subIntfV4 := subIntf.GetOrCreateIpv4()
	// //  subIntfV4.Enabled = ygot.Bool(true)  // deviation
	// address4 := subIntfV4.GetOrCreateAddress(bundleIP.String())
	// address4.PrefixLength = ygot.Uint8(30)
	// address4.Type = oc.IfIp_Ipv4AddressType_PRIMARY
	// subIntfV6 := subIntf.GetOrCreateIpv6()
	// //  subIntfV6.Enabled = ygot.Bool(true)  // deviation
	// ipv6Address := fmt.Sprintf("2002::%s:%s", hex.EncodeToString(bundleIP[:2]), hex.EncodeToString(bundleIP[2:]))
	// address6 := subIntfV6.GetOrCreateAddress(ipv6Address)
	// address6.PrefixLength = ygot.Uint8(126)
	// address6.Type = oc.IfIp_Ipv6AddressType_GLOBAL_UNICAST
}

func createBundles(t *testing.T, dut, peer *ondatra.DUTDevice, bundleMap map[string][]InterfacePhysicalLink) {
	dutBatchConfig := &gnmi.SetBatch{}
	peerBatchConfig := &gnmi.SetBatch{}
	ipAddress := net.IP{192, 192, 1, 0}
	for bundleName, links := range bundleMap {
		ipAddress = incrementIP(ipAddress, 1)
		// Create bundle interface on DUT
		dutRoot := &oc.Root{}
		configureBundle(dutRoot, bundleName, ipAddress)

		// Add member interfaces to the bundle on DUT
		for _, link := range links {
			member := dutRoot.GetOrCreateInterface(link.IntfName)
			member.GetOrCreateEthernet().AggregateId = ygot.String(bundleName)
			member.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		}
		// gnmi.Update(t, dut, gnmi.OC().Config(), dutRoot)
		gnmi.BatchUpdate(dutBatchConfig, gnmi.OC().Config(), dutRoot)

		// increment the ip
		ipAddress = incrementIP(ipAddress, 1)
		// Create bundle interface on Peer
		peerRoot := &oc.Root{}
		configureBundle(peerRoot, bundleName, ipAddress)

		// Add member interfaces to the bundle on Peer
		for _, link := range links {
			member := peerRoot.GetOrCreateInterface(link.PeerIntfName)
			member.GetOrCreateEthernet().AggregateId = ygot.String(bundleName)
			member.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		}
		gnmi.BatchUpdate(peerBatchConfig, gnmi.OC().Config(), peerRoot)
		// gnmi.Update(t, peer, gnmi.OC().Config(), peerRoot)
		ipAddress = incrementIP(ipAddress, 2)
	}
	dutBatchConfig.Set(t, dut)
	peerBatchConfig.Set(t, peer)
}

// Function to increment the ip address by `increment` times
func incrementIP(ip net.IP, increment int) net.IP {
	ip = ip.To4()
	if ip == nil {
		return nil
	}
	ipInt := big.NewInt(0).SetBytes(ip)
	ipInt.Add(ipInt, big.NewInt(int64(increment)))
	return net.IP(ipInt.Bytes())
}

// Define a custom type that implements the sort.Interface
type ByLineCardNumber []InterfacePhysicalLink

func (a ByLineCardNumber) Len() int           { return len(a) }
func (a ByLineCardNumber) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByLineCardNumber) Less(i, j int) bool { return a[i].LineCardNumber < a[j].LineCardNumber }

// sortLinkInfosByLineCardNumber sorts the linkInfos based on LineCardNumber
func sortLinkInfosByLineCardNumber(linkInfos []InterfacePhysicalLink) []InterfacePhysicalLink {
	sort.Sort(ByLineCardNumber(linkInfos))
	return linkInfos
}

func enableLldp(t *testing.T, device *ondatra.DUTDevice) bool {
	path := gnmi.OC().Lldp().Enabled()
	gnmi.Update(t, device, path.Config(), true)
	// TODO: logic to capture the gnmi result and validate
	return true
}

// getAllInterfaces retrieves all interfaces whose name contains the string "Gig".
func getAllInterfaces(t *testing.T, device *ondatra.DUTDevice) []*oc.Interface {
	path := gnmi.OC().InterfaceAny().State()
	allInterfaces := gnmi.GetAll(t, device, path)
	var filteredInterfaces []*oc.Interface

	for _, intf := range allInterfaces {
		intfName := intf.GetName()
		if strings.Contains(intfName, "Gig") {
			filteredInterfaces = append(filteredInterfaces, intf)
		}
	}
	return filteredInterfaces
}

// enableInterfacesAll enables all interfaces provided in the list.
func enableInterfacesAll(t *testing.T, device *ondatra.DUTDevice, interfaces []*oc.Interface) bool {
	batchConfig := &gnmi.SetBatch{}
	for _, intf := range interfaces {
		d := &oc.Root{}
		i := d.GetOrCreateInterface(intf.GetName())
		i.Enabled = ygot.Bool(true)
		i.Type = intf.Type // type is mandatory
		path := gnmi.OC().Interface(intf.GetName()).Config()
		gnmi.BatchUpdate(batchConfig, path, i)
	}
	// TODO logic to capture any failure during batchconfig
	batchConfig.Set(t, device)
	return true
}

// getEnabledInterfaces returns a list of interfaces that are enabled.
func getEnabledInterfaces(interfaces []*oc.Interface) []*oc.Interface {
	var enabledInterfaces []*oc.Interface
	for _, intf := range interfaces {
		if (intf.GetEnabled()) && (intf.GetOperStatus() == oc.Interface_OperStatus_UP) {
			enabledInterfaces = append(enabledInterfaces, intf)
		}
	}
	return enabledInterfaces
}

// enableInterfaceLldp enables LLDP on all interfaces provided in the list.
func enableInterfaceLldp(t *testing.T, device *ondatra.DUTDevice, interfaces []*oc.Interface) bool {
	batchConfig := &gnmi.SetBatch{}
	for _, intf := range interfaces {
		d := &oc.Root{}
		i, _ := d.GetOrCreateLldp().NewInterface(intf.GetName())
		i.Enabled = ygot.Bool(true)
		path := gnmi.OC().Lldp().Interface(intf.GetName()).Config()
		gnmi.BatchUpdate(batchConfig, path, i)
	}
	batchConfig.Set(t, device)
	return true
}

// getPeerInterfaceName retrieves the peer interface name using LLDP data.
func getPeerInterfaceName(lldpIntfStateAny []*oc.Lldp_Interface, intfName string) string {
	// Retrieve the LLDP interface state
	// lldpIntfStatePathAny := gnmi.OC().Lldp().InterfaceAny().State()
	// lldpIntfStateAny := gnmi.GetAll(t, device, lldpIntfStatePathAny)
	// lldpIntfStatePath := gnmi.OC().Lldp().Interface(intfName).State()
	// lldpIntfState := gnmi.Get(t, device, lldpIntfStatePath)

	for _, lldpIntfState := range lldpIntfStateAny {
		if lldpIntfState.GetName() == intfName {
			// Check if neighbors are present
			if lldpIntfState != nil && lldpIntfState.Neighbor != nil {
				for _, neighbor := range lldpIntfState.Neighbor {
					if neighbor.PortId != nil && strings.Contains(*neighbor.PortId, "Gig") {
						return *neighbor.PortId
					}
				}
			}
		}
	}

	return ""
}

// GNMIStreamManager manages multiple gNMI streams.
type GNMIStreamManager struct {
	ctx        context.Context    // Context for managing cancellation.
	cancel     context.CancelFunc // Function to cancel the context.
	wg         sync.WaitGroup     // WaitGroup to synchronize goroutines.
	numStreams int                // Number of gNMI streams to manage.
}

// StartScaledStreams starts the specified number of gNMI streams without blocking.
func StartScaledStreams(t *testing.T, dut *ondatra.DUTDevice, numStreams int) *GNMIStreamManager {
	ctx, cancel := context.WithCancel(context.Background())
	manager := &GNMIStreamManager{
		ctx:        ctx,
		cancel:     cancel,
		numStreams: numStreams,
	}

	// Obtain the gNMI client
	gnmiClient := dut.RawAPIs().GNMI(t)
	if gnmiClient == nil {
		t.Fatalf("Failed to get gNMI client from DUT")
	}

	// Start the specified number of gNMI streams
	for i := 0; i < numStreams; i++ {
		manager.wg.Add(1)
		go manager.startGNMISubscription(t, gnmiClient, i)
	}

	return manager
}

// startGNMISubscription starts a single gNMI subscription in a goroutine.
func (manager *GNMIStreamManager) startGNMISubscription(t *testing.T, gnmiClient gnmipb.GNMIClient, streamID int) {
	defer manager.wg.Done()

	// Define the subscription path
	path := &gnmipb.Path{
		Elem: []*gnmipb.PathElem{
			{Name: "system"},
			{Name: "state"},
			{Name: "current-datetime"},
		},
	}

	// Create the Subscribe stream
	subClient, err := gnmiClient.Subscribe(manager.ctx)
	if err != nil {
		t.Logf("Stream %d: Failed to create Subscribe stream: %v", streamID, err)
		return
	}

	// Create the SubscribeRequest
	subReq := &gnmipb.SubscribeRequest{
		Request: &gnmipb.SubscribeRequest_Subscribe{
			Subscribe: &gnmipb.SubscriptionList{
				Subscription: []*gnmipb.Subscription{
					{
						Path:           path,
						Mode:           gnmipb.SubscriptionMode_SAMPLE,
						SampleInterval: uint64(2 * time.Second.Nanoseconds()), // 2 seconds
					},
				},
				Mode:     gnmipb.SubscriptionList_STREAM,
				Encoding: gnmipb.Encoding_JSON_IETF,
			},
		},
	}

	// Send the SubscribeRequest
	if err := subClient.Send(subReq); err != nil {
		t.Logf("Stream %d: Failed to send SubscribeRequest: %v", streamID, err)
		return
	}

	// Receive updates in the background
	for {
		select {
		case <-manager.ctx.Done():
			t.Logf("Stream %d: Context canceled, stopping subscription", streamID)
			return
		default:
			_, err := subClient.Recv()
			if err != nil {
				if err == io.EOF {
					t.Logf("Stream %d: Subscription stream closed", streamID)
				} else {
					grpcStatus, ok := status.FromError(err)
					if ok {
						t.Logf("Stream %d: gRPC error received: %v, details: %v", streamID, grpcStatus.Code(), grpcStatus.Message())
					} else {
						t.Logf("Stream %d: Error receiving response: %v", streamID, err)
					}
				}
				return
			}
		}
	}
}

// StopScaledStreams stops all running gNMI streams and cancels the context.
func (manager *GNMIStreamManager) StopScaledStreams() {
	manager.cancel()
}

// GetActiveGrpcStreams checks the number of active gRPC streams on the DUT.
// It parses the output of the "show grpc streams" CLI command.
func GetActiveGrpcStreams(t *testing.T, dut *ondatra.DUTDevice, expectedStreams int) int {
	t.Logf("GetActiveGrpcStreams: Starting check for active streams")
	cliCommand := "show grpc streams"
	cliClient := dut.RawAPIs().CLI(t)
	t.Logf("GetActiveGrpcStreams: Executing CLI command: %s", cliCommand)
	output, err := cliClient.RunCommand(context.Background(), cliCommand)
	if err != nil {
		t.Errorf("GetActiveGrpcStreams: Error running CLI command '%s': %v", cliCommand, err)
		return 0
	}

	t.Logf("GetActiveGrpcStreams: CLI output:\n%s", output.Output())

	re := regexp.MustCompile(`Streaming gRPCs: (\d+)`)
	matches := re.FindStringSubmatch(output.Output())
	if len(matches) < 2 {
		t.Errorf("GetActiveGrpcStreams: Failed to parse the number of active gRPC streams from the output")
		return 0
	}

	activeStreams, err := strconv.Atoi(matches[1])
	if err != nil {
		t.Errorf("GetActiveGrpcStreams: Failed to convert active gRPC streams to integer: %v", err)
		return 0
	}

	t.Logf("GetActiveGrpcStreams: Active gRPC streams: %d", activeStreams)

	if activeStreams < expectedStreams {
		t.Logf("GetActiveGrpcStreams: Warning: Lower number of active gRPC streams than expected. Got: %d, Expected: %d", activeStreams, expectedStreams)
	} else {
		t.Logf("GetActiveGrpcStreams: Number of active gRPC streams is within expected range: %d", activeStreams)
	}

	return activeStreams
}

// GetVersion fetches the software version from the device and splits it into components.
func GetVersion(t *testing.T, dut *ondatra.DUTDevice) (majorVersion, minorVersion, runningVersion, labelVersion string, err error) {
	// Simulate fetching the version string from the device.
	path := gnmi.OC().System().SoftwareVersion()
	versionString := gnmi.Get(t, dut, path.State())

	// Split the version string by '.' to get the parts.
	parts := strings.Split(versionString, ".")
	fmt.Printf("Debug: Split version string into parts: %v\n", parts)

	// Ensure the version string has at least three parts.
	if len(parts) < 3 {
		err := fmt.Errorf("unexpected version format: %s", versionString)
		fmt.Printf("Error: %v\n", err)
		return "", "", "", "", err
	}

	// Assign the mandatory parts to their respective variables.
	majorVersion = parts[0]
	minorVersion = parts[1]
	runningVersion = parts[2]
	fmt.Printf("Debug: Parsed Major: %s, Minor: %s, Running: %s\n", majorVersion, minorVersion, runningVersion)

	// Check if there is an optional label version.
	if len(parts) > 3 {
		labelVersion = parts[3]
		fmt.Printf("Debug: Parsed Label: %s\n", labelVersion)
	} else {
		fmt.Println("Debug: No label version found.")
	}

	return majorVersion, minorVersion, runningVersion, labelVersion, nil
}

func SupervisorSwitchover(t *testing.T, dut *ondatra.DUTDevice) {
	// dut := ondatra.DUT(t, "dut")

	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}

	intfsOperStatusUPBeforeSwitch := helpers.FetchOperStatusUPIntfs(t, dut, *args.CheckInterfacesInBinding)
	t.Logf("intfsOperStatusUP interfaces before switchover: %v", intfsOperStatusUPBeforeSwitch)
	if got, want := len(intfsOperStatusUPBeforeSwitch), 0; got == want {
		t.Errorf("Get the number of intfsOperStatusUP interfaces for %q: got %v, want > %v", dut.ID(), got, want)
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := rpStandbyBeforeSwitch
	got := ""
	if deviations.GNOISubcomponentPath(dut) {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}
	if got, want := switchoverResponse.GetVersion(), ""; got == want {
		t.Errorf("switchoverResponse.GetVersion(): got %v, want non-empty version", got)
	}
	if got := switchoverResponse.GetUptime(); got == 0 {
		t.Errorf("switchoverResponse.GetUptime(): got %v, want > 0", got)
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
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(maxSwitchoverTime); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, dut, controllerCards)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	helpers.ValidateOperStatusUPIntfs(t, dut, intfsOperStatusUPBeforeSwitch, 5*time.Minute)

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)

	swTime, swTimePresent := gnmi.Watch(t, dut, activeRP.LastSwitchoverTime().State(), 1*time.Minute, func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }).Await(t)
	if !swTimePresent {
		t.Errorf("activeRP.LastSwitchoverTime().Watch(t).IsPresent(): got %v, want %v", false, true)
	} else {
		st, _ := swTime.Val()
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", st)
		// TODO: validate that last switchover time is correct
	}

	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
}
