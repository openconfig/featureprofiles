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

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/system"
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
	"google.golang.org/protobuf/encoding/prototext"
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

const (
	active_rp  = "0/RP0/CPU0"
	standby_rp = "0/RP1/CPU0"
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

func FlapBulkInterfaces(t *testing.T, dut *ondatra.DUTDevice, intfList []string) {

	var flapDuration time.Duration = 2
	var adminState bool

	adminState = false
	SetInterfaceStateScale(t, dut, intfList, adminState)
	time.Sleep(flapDuration * time.Second)
	adminState = true
	SetInterfaceStateScale(t, dut, intfList, adminState)
}

func SetInterfaceStateScale(t *testing.T, dut *ondatra.DUTDevice, intfList []string,
	adminState bool) {

	var intfType oc.E_IETFInterfaces_InterfaceType
	batchConfig := &gnmi.SetBatch{}

	for i := 0; i < len(intfList); i++ {
		if intfList[i][:6] == "Bundle" {
			intfType = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		} else {
			intfType = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		}
		j := &oc.Interface{
			Enabled: ygot.Bool(adminState),
			Name:    ygot.String(intfList[i]),
			Type:    intfType,
		}
		gnmi.BatchUpdate(batchConfig, gnmi.OC().Interface(intfList[i]).Config(), j)
	}
	batchConfig.Set(t, dut)
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
func GNMIWithText(ctx context.Context, t testing.TB, dut *ondatra.DUTDevice, config string) {
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
		t.Fatalf("error applying config: %v", err)
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

func generateSubifDUTConfig(d *oc.Root, dut *ondatra.DUTDevice, interfaceName string, index uint32, vlanID uint16, ipv4Addr string, ipv4PrefixLen uint8, ipv6Addr string, ipv6PrefixLen uint8) *oc.Interface {
	// Get or create the interface
	i := d.GetOrCreateInterface(interfaceName)
	i.Name = ygot.String(interfaceName) // Explicitly set the name field
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	// Get or create the subinterface
	s := i.GetOrCreateSubinterface(index)

	// Configure VLAN if applicable
	if vlanID != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}

	// Configure IPv4
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4Addr)
	a.PrefixLength = ygot.Uint8(uint8(ipv4PrefixLen))
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}

	// Configure IPv6
	s6 := s.GetOrCreateIpv6()
	a1 := s6.GetOrCreateAddress(ipv6Addr)
	a1.PrefixLength = ygot.Uint8(uint8(ipv6PrefixLen))

	// Return the interface configuration
	return i
}

// LinkIPs holds the IPv4/IPv6 addresses for DUT and Peer for a interface link.
type LinkIPs struct {
	DutIPv4  string `json:"dut_ipv4"`
	DutIPv6  string `json:"dut_ipv6"`
	PeerIPv4 string `json:"peer_ipv4"`
	PeerIPv6 string `json:"peer_ipv6"`
}

// ExtractLinkIPsField returns a slice of the requested field from a map[string]LinkIPs.
// field can be "dutipv4", "dutipv6", "peeripv4", or "peeripv6".
func ExtractLinkIPsField(linkMap map[string]LinkIPs, field string) []string {
	var result []string
	var getField func(LinkIPs) string

	switch strings.ToLower(field) {
	case "dutipv4":
		getField = func(l LinkIPs) string { return l.DutIPv4 }
	case "dutipv6":
		getField = func(l LinkIPs) string { return l.DutIPv6 }
	case "peeripv4":
		getField = func(l LinkIPs) string { return l.PeerIPv4 }
	case "peeripv6":
		getField = func(l LinkIPs) string { return l.PeerIPv6 }
	default:
		return result
	}

	for _, v := range linkMap {
		result = append(result, getField(v))
	}
	return result
}

// createSubInterfaces creates subinterfaces for the given links, configuring both DUT and Peer interfaces.
// It assigns /31 for IPv4 and /127 for IPv6 addresses.
func CreateBundleSubInterfaces(t *testing.T, dut *ondatra.DUTDevice, peer *ondatra.DUTDevice, links []string, subIntCount int, nextIPv4, nextIPv6 net.IP) (net.IP, net.IP, map[string]LinkIPs) {
	t.Helper()
	t.Logf("Creating subinterfaces for %d links", len(links))

	// Calculate the number of subinterfaces per link and the remainder
	subIntPerLink := subIntCount / len(links)
	remainder := subIntCount % len(links)

	// Create batch configurations for DUT and Peer
	dutBatchConfig := &gnmi.SetBatch{}
	peerBatchConfig := &gnmi.SetBatch{}
	subIntfIPMap := make(map[string]LinkIPs)

	for i, link := range links {
		// Determine the number of subinterfaces for this link
		subIntForThisLink := subIntPerLink
		if i < remainder {
			subIntForThisLink++ // Distribute the remainder among the first few links
		}
		t.Logf("Creating %d subinterfaces for link %s", subIntForThisLink, link)
		for subIntID := 1; subIntID <= subIntForThisLink; subIntID++ {
			// Generate DUT subinterface config
			dutIPv4 := nextIPv4
			dutIPv6 := nextIPv6
			dutConfig := generateSubifDUTConfig(&oc.Root{}, dut, link, uint32(subIntID), uint16(subIntID), nextIPv4.String(), 31, nextIPv6.String(), 127)
			gnmi.BatchUpdate(dutBatchConfig, gnmi.OC().Interface(link).Config(), dutConfig)
			nextIPv4 = incrementIP(nextIPv4, 1)
			nextIPv6 = incrementIPv6(nextIPv6)

			peerIPv4 := nextIPv4
			peerIPv6 := nextIPv6
			// Generate Peer subinterface config
			peerConfig := generateSubifDUTConfig(&oc.Root{}, dut, link, uint32(subIntID), uint16(subIntID), nextIPv4.String(), 31, nextIPv6.String(), 127)
			gnmi.BatchUpdate(peerBatchConfig, gnmi.OC().Interface(link).Config(), peerConfig)
			// Save peer subinterface IPv4 address
			subIntfIPMap[fmt.Sprintf("%s.%d", link, subIntID)] = LinkIPs{
				DutIPv4:  dutIPv4.String(),
				DutIPv6:  dutIPv6.String(),
				PeerIPv4: peerIPv4.String(),
				PeerIPv6: peerIPv6.String(),
			}
			nextIPv4 = incrementIP(nextIPv4, 1)
			nextIPv6 = incrementIPv6(nextIPv6)
		}
	}
	// Push batch configurations to DUT and Peer
	dutBatchConfig.Set(t, dut)
	peerBatchConfig.Set(t, peer)

	return nextIPv4, nextIPv6, subIntfIPMap
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
func FaultInjectionMechanism(t *testing.T, dut *ondatra.DUTDevice, componentName string, faultPointNumber string, returnValue string, activate bool) {
	cliHandle := dut.RawAPIs().CLI(t)
	lcs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
	for _, lc := range lcs {
		parts := strings.Split(lc, "/")
		var lineCard string
		for i, part := range parts {
			if part == "CPU0" && i > 0 {
				lineCard = parts[i-1]
				break
			}
		}
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

// addISISOC, configures ISIS on DUT
func AddISISOCWithSysAreaID(t *testing.T, device *ondatra.DUTDevice, ifaceName, sysID, areaID, instanceName string) {
	t.Helper()

	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(PTISIS, instanceName)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Net = []string{fmt.Sprintf("%v.%v.00", areaID, sysID)}
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

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(PTISIS, instanceName)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(PTISIS, instanceName)
	gnmi.Update(t, device, dutNode.Config(), dutConf)
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

func ParallelReloadLineCards(t *testing.T, dut *ondatra.DUTDevice, wgg *sync.WaitGroup) error {

	defer wgg.Done()

	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	lcs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
	wg := sync.WaitGroup{}
	relaunched := make([]string, 0)

	for _, lc := range lcs {
		t.Logf("Restarting LC %v\n", lc)
		if empty := gnmi.Get(t, dut, gnmi.OC().Component(lc).Empty().State()); empty {
			t.Logf("Linecard Component %s is empty, skipping", lc)
		}
		if removable := gnmi.Get(t, dut, gnmi.OC().Component(lc).Removable().State()); !removable {
			t.Logf("Linecard Component %s is non-removable, skipping", lc)
		}
		oper := gnmi.Get(t, dut, gnmi.OC().Component(lc).OperStatus().State())

		if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
			t.Logf("Linecard Component %s is already INACTIVE, skipping", lc)
		}

		lineCardPath := components.GetSubcomponentPath(lc, false)
		resp, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot line card without delay",
			Subcomponents: []*tpb.Path{
				lineCardPath,
			},
			Force: true,
		})
		if err == nil {
			wg.Add(1)
			relaunched = append(relaunched, lc)
		} else {
			t.Fatalf("Reboot failed %v", err)
		}
		t.Logf("Reboot response: \n%v\n", resp)
	}

	// wait for all line cards to be back up
	for _, lc := range relaunched {
		go func(lc string) {
			defer wg.Done()
			timeout := time.Minute * 30
			t.Logf("Awaiting relaunch of linecard: %s", lc)
			oper := gnmi.Await[oc.E_PlatformTypes_COMPONENT_OPER_STATUS](
				t, dut,
				gnmi.OC().Component(lc).OperStatus().State(),
				timeout,
				oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
			)
			if val, ok := oper.Val(); !ok {
				t.Errorf("Reboot timed out, received status: %s", val)
				// check status if failed
			}
		}(lc)
	}

	wg.Wait()
	t.Log("All linecards successfully relaunched")

	return nil
}

func ParallelProcessRestart(t *testing.T, dut *ondatra.DUTDevice, processName string, wg *sync.WaitGroup) {

	defer wg.Done()

	waitForRestart := true
	pid := system.FindProcessIDByName(t, dut, processName)
	if pid == 0 {
		t.Fatalf("process %s not found on device", processName)
	}
	gnoiClient := dut.RawAPIs().GNOI(t)
	killProcessRequest := &spb.KillProcessRequest{
		Signal:  spb.KillProcessRequest_SIGNAL_KILL,
		Name:    processName,
		Pid:     uint32(pid),
		Restart: true,
	}
	gnoiClient.System().KillProcess(context.Background(), killProcessRequest)
	time.Sleep(30 * time.Second)

	if waitForRestart {
		gnmi.WatchAll(
			t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gnmipb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().System().ProcessAny().State(),
			time.Minute,
			func(p *ygnmi.Value[*oc.System_Process]) bool {
				val, ok := p.Val()
				if !ok {
					return false
				}
				return val.GetName() == processName && val.GetPid() != pid
			},
		)
	}
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

// Convert []any to []string
func ToStringSlice(in []any) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = fmt.Sprintf("%v", v)
	}
	return out
}

func ParallelReloadRouter(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) error {

	defer wg.Done()

	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())

	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	Resp, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Reboot failed %v", err)
	}
	t.Logf("Reload Response %v ", Resp)

	startReboot := time.Now()
	time.Sleep(5 * time.Second)
	const maxRebootTime = 30
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(90 * time.Second)
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
	return nil

}

type InterfacePhysicalLink struct {
	Intf               *oc.Interface
	IntfName           string
	IntfV4Addr         string
	IntfV6Addr         string
	LineCardNumber     string
	PeerIntfName       string
	PeerV4Addr         string
	PeerV6Addr         string
	PeerIntf           *oc.Interface
	PeerLineCardNumber string
}

type BundleLinks struct {
	Name       string
	IntfV4Addr string
	IntfV6Addr string
	PeerV4Addr string
	PeerV6Addr string
	Links      []InterfacePhysicalLink
}

// var bundles []BundleLinks

// ExtractBundleLinkField returns a slice of the requested field from all InterfacePhysicalLink in all bundles.
// field can be "intf", "intfv4addr", or "intfname".
// ExtractBundleLinkField returns a slice of the requested field from all InterfacePhysicalLink in all bundles.
// field can be any field in InterfacePhysicalLink or "name" for the bundle name.
func ExtractBundleLinkField(bundles []BundleLinks, field string) []any {
	var result []interface{}
	switch strings.ToLower(field) {
	case "name":
		for _, bundle := range bundles {
			result = append(result, bundle.Name)
		}
	case "intfv4addr":
		for _, bundle := range bundles {
			result = append(result, bundle.IntfV4Addr)
		}
	case "intfv6addr":
		for _, bundle := range bundles {
			result = append(result, bundle.IntfV6Addr)
		}
	case "peerv4addr":
		for _, bundle := range bundles {
			result = append(result, bundle.PeerV4Addr)
		}
	case "peerv6addr":
		for _, bundle := range bundles {
			result = append(result, bundle.PeerV6Addr)

		}
	case "peerlinecardnumber":
		for _, bundle := range bundles {
			for _, link := range bundle.Links {
				result = append(result, link.PeerLineCardNumber)
			}
		}
	}
	return result
}

// Assumptions
// dut is connected only to peer (no other lldp device in the topology)
// no  bundle is configured before calling this function
func ConfigureBundleIntfDynamic(t *testing.T, dut *ondatra.DUTDevice, peer *ondatra.DUTDevice, memberCount int, ipv4Subnet, ipv6Subnet string) []BundleLinks {

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
	// Fixed sleep time for the LLDP convergence
	time.Sleep(35 * time.Second)
	// update dut interface after enabling the interface
	dutInterfaces = getAllInterfaces(t, dut)
	t.Logf("DUT interface list: %v", dutInterfaces)
	time.Sleep(35 * time.Second)
	// update dut interface after enabling the interface
	dutInterfaces = getAllInterfaces(t, dut)

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
	// bundles := make([]BundleLinks, numBundles)
	// for i := 0; i < numBundles; i++ {
	// 	bundleName := fmt.Sprintf("Bundle-Ether%d", 100+i)
	// 	bundleMap[bundleName] = []InterfacePhysicalLink{}
	// }
	var bundles []BundleLinks
	for i := 0; i < numBundles; i++ {
		bundleName := fmt.Sprintf("Bundle-Ether%d", 100+i)
		bundles = append(bundles, BundleLinks{Name: bundleName, Links: []InterfacePhysicalLink{}})
	}

	// Distribute the sortedLinkInfos into bundles in a round-robin fashion
	for i, link := range sortedLinkInfos {
		bundleIdx := i % numBundles
		bundles[bundleIdx].Links = append(bundles[bundleIdx].Links, link)
	}

	t.Logf("Total Links: %d, Member Count: %d, Number of Bundles: %d\n", linkCount, memberCount, numBundles)
	for _, bundle := range bundles {
		t.Logf("Bundle: %s\n", bundle.Name)
		for _, link := range bundle.Links {
			t.Logf("  DUT Interface: %v, DUT LC: %v, Peer Interface: %v, Peer LC: %v\n",
				link.IntfName, link.LineCardNumber, link.PeerIntfName, link.PeerLineCardNumber)
		}
	}

	// Create bundles in both DUT and Peer devices
	Bundles(bundles).CreateBundles(t, dut, peer, ipv4Subnet, ipv6Subnet)

	return bundles
}

func configureBundle(ocRoot *oc.Root, bundleName string, bundleIP net.IP, ipv6Addr net.IP) {
	bundle := ocRoot.GetOrCreateInterface(bundleName)
	bundle.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	bundle.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	bundle.Enabled = ygot.Bool(true)
	// Configure subinterface 0 with IPv4 address
	subIntf := bundle.GetOrCreateSubinterface(0)
	subIntfV4 := subIntf.GetOrCreateIpv4()
	address4 := subIntfV4.GetOrCreateAddress(bundleIP.String())
	address4.PrefixLength = ygot.Uint8(30)
	// Configure subinterface 0 with IPv6 address
	subIntfV6 := subIntf.GetOrCreateIpv6()
	address6 := subIntfV6.GetOrCreateAddress(ipv6Addr.String())
	address6.PrefixLength = ygot.Uint8(126)
}

type Bundles []BundleLinks

func (bundles Bundles) CreateBundles(t *testing.T, dut, peer *ondatra.DUTDevice, ipv4Subnet, ipv6Subnet string) {
	dutBatchConfig := &gnmi.SetBatch{}
	peerBatchConfig := &gnmi.SetBatch{}
	// ipAddress := net.IP{192, 192, 1, 0}
	_, ipnet, err := net.ParseCIDR(ipv4Subnet)
	if err != nil {
		t.Fatalf("Failed to parse subnet: %v", err)
	}
	ipAddress := ipnet.IP

	_, ipnet6, err := net.ParseCIDR(ipv6Subnet)
	if err != nil {
		t.Fatalf("Failed to parse IPv6 subnet: %v", err)
	}
	ipv6Addr := ipnet6.IP

	for i, bundle := range bundles {
		ipAddress = incrementIP(ipAddress, 1)
		ipv6Addr = incrementIPv6(ipv6Addr)

		// Create bundle interface on DUT
		dutRoot := &oc.Root{}
		configureBundle(dutRoot, bundle.Name, ipAddress, ipv6Addr)
		bundles[i].IntfV4Addr = ipAddress.String()
		bundles[i].IntfV6Addr = ipv6Addr.String()

		// Add member interfaces to the bundle on DUT
		for _, link := range bundle.Links {
			member := dutRoot.GetOrCreateInterface(link.IntfName)
			member.GetOrCreateEthernet().AggregateId = ygot.String(bundle.Name)
			member.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		}
		gnmi.BatchUpdate(dutBatchConfig, gnmi.OC().Config(), dutRoot)

		ipAddress = incrementIP(ipAddress, 1)
		ipv6Addr = incrementIPv6(ipv6Addr)
		// Create bundle interface on Peer
		peerRoot := &oc.Root{}
		configureBundle(peerRoot, bundle.Name, ipAddress, ipv6Addr)
		bundles[i].PeerV4Addr = ipAddress.String()
		bundles[i].PeerV6Addr = ipv6Addr.String()

		// Add member interfaces to the bundle on Peer
		for _, link := range bundle.Links {
			member := peerRoot.GetOrCreateInterface(link.PeerIntfName)
			member.GetOrCreateEthernet().AggregateId = ygot.String(bundle.Name)
			member.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		}
		gnmi.BatchUpdate(peerBatchConfig, gnmi.OC().Config(), peerRoot)
		ipAddress = incrementIP(ipAddress, 2)
		ipv6Addr = incrementIPv6(ipv6Addr)
		ipv6Addr = incrementIPv6(ipv6Addr)
	}
	dutBatchConfig.Set(t, dut)
	peerBatchConfig.Set(t, peer)
}

// Function to increment the ipv4 address by `increment` times
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

// incrementSubnetCIDR takes a CIDR and an increment index to generate the next subnet CIDR.
func IncrementSubnetCIDR(cidr string, index int) (string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("parsing CIDR failed: %v", err)
	}

	// Determine the number of bits for the subnet.
	ones, bits := ipnet.Mask.Size()

	// Calculate the new subnet based on the index and number of hosts.
	subnetIncrement := big.NewInt(1)
	subnetIncrement.Lsh(subnetIncrement, uint(bits-ones))
	subnetIncrement.Mul(subnetIncrement, big.NewInt(int64(index)))

	ipBigInt := big.NewInt(0).SetBytes(ip)
	ipBigInt.Add(ipBigInt, subnetIncrement)

	// Convert big.Int back to net.IP.
	newIP := make(net.IP, len(ip))
	if len(ip) == net.IPv4len {
		ipBigInt.FillBytes(newIP[12:16]) // IPv4 addresses are the last 4 bytes of the IPv6 space.
	} else {
		newIP = ipBigInt.FillBytes(make([]byte, net.IPv6len))
	}

	return (&net.IPNet{IP: newIP, Mask: ipnet.Mask}).String(), nil
}

func incrementIPv6(ip net.IP) net.IP {
	ip = ip.To16()
	ipInt := big.NewInt(0).SetBytes(ip)

	ipInt.Add(ipInt, big.NewInt(1))

	ipBytes := ipInt.Bytes()
	newIP := make(net.IP, net.IPv6len)
	copy(newIP[net.IPv6len-len(ipBytes):], ipBytes)

	return newIP
}

// getUsableIPs takes a subnet and returns the first and second usable IP addresses.
func GetUsableIPs(cidr string) (net.IP, net.IP) {
	_, subnet, _ := net.ParseCIDR(cidr)
	if subnet.IP.To4() != nil {
		// It's an IPv4 address
		firstIP := incrementIP(subnet.IP, 1)
		secondIP := incrementIP(firstIP, 1)
		return firstIP, secondIP
	} else {
		// It's an IPv6 address
		firstIP := incrementIPv6(subnet.IP)
		secondIP := incrementIPv6(firstIP)
		return firstIP, secondIP
	}
}

// generateMAC is a placeholder for a function that generates a MAC address based on some logic.
func generateMAC(vlanID int) string {
	return fmt.Sprintf("00:1A:11:%02X:00:01", vlanID)
}

// generateSubnetAttributes calculates subnet attributes based on a base IPv4 and IPv6 address, VLAN ID, and an index.
func GenerateSubnetAttributes(baseIPv4, baseIPv6 string, vlanID, index int) (attrs.Attributes, attrs.Attributes, error) {

	newIPv4Subnet, err := IncrementSubnetCIDR(baseIPv4, index)
	if err != nil {
		return attrs.Attributes{}, attrs.Attributes{}, err
	}

	newIPv6Subnet, err := IncrementSubnetCIDR(baseIPv6, index)
	if err != nil {
		return attrs.Attributes{}, attrs.Attributes{}, err
	}

	dutIPv4, ateIPv4 := GetUsableIPs(newIPv4Subnet)
	dutIPv6, ateIPv6 := GetUsableIPs(newIPv6Subnet)

	dutMAC := generateMAC(vlanID)
	ateMAC := generateMAC(vlanID)

	dutAttrs := attrs.Attributes{
		Name:    fmt.Sprintf("DUTport%d", vlanID),
		IPv4:    dutIPv4.String(),
		IPv6:    dutIPv6.String(),
		MAC:     dutMAC,
		Desc:    fmt.Sprintf("DUT Port %d", vlanID),
		IPv4Len: 30,
		IPv6Len: 126,
	}

	ateAttrs := attrs.Attributes{
		Name:    fmt.Sprintf("ATEport%d", vlanID),
		IPv4:    ateIPv4.String(),
		IPv6:    ateIPv6.String(),
		MAC:     ateMAC,
		Desc:    fmt.Sprintf("ATE Port %d", vlanID),
		IPv4Len: 30,
		IPv6Len: 126,
	}

	return dutAttrs, ateAttrs, nil
}

func GnmiProtoSetConfigPush(t *testing.T, dut *ondatra.DUTDevice, configFilePath string, timeout time.Duration) {
	b, err := os.ReadFile(configFilePath)
	if err != nil {
		panic(err)
	}

	setReq := &gnmipb.SetRequest{}

	if err := prototext.Unmarshal(b, setReq); err != nil {
		panic(err)
	}

	gNMIC := dut.RawAPIs().GNMI(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if _, err = gNMIC.Set(ctx, setReq); err != nil {
		t.Fatalf("gnmi Set Config Push failed with unexpected error: %v", err)
	}
}

// SwitchoverReady checks if the RP is ready for switchover.
func SwitchoverReady(t *testing.T, dut *ondatra.DUTDevice, controller string, timeout time.Duration) bool {
	switchoverReady := gnmi.OC().Component(controller).SwitchoverReady()
	_, ok := gnmi.Watch(t, dut, switchoverReady.State(), timeout, func(val *ygnmi.Value[bool]) bool {
		ready, present := val.Val()
		return present && ready
	}).Await(t)
	return ok
}

// GetVersion fetches the software version from the device and splits it into components.
func GetVersion(t *testing.T, dut *ondatra.DUTDevice) (majorVersion, minorVersion, runningVersion, labelVersion string, err error) {
	// fetching the version string from the device.
	path := gnmi.OC().System().SoftwareVersion()
	versionString := gnmi.Get(t, dut, path.State())
	return splitVersionString(t, versionString)
}

func splitVersionString(t *testing.T, versionString string) (majorVersion, minorVersion, runningVersion, labelVersion string, err error) {
	// Split the version string by '.' to get the parts.
	parts := strings.Split(versionString, ".")
	t.Logf("Debug: Split version string into parts: %v\n", parts)

	// Ensure the version string has at least three parts.
	if len(parts) < 3 {
		err := fmt.Errorf("unexpected version format: %s", versionString)
		t.Logf("Error: %v\n", err)
		return "", "", "", "", err
	}

	// Assign the mandatory parts to their respective variables.
	majorVersion = parts[0]
	minorVersion = parts[1]
	runningVersion = parts[2]
	t.Logf("Debug: Parsed Major: %s, Minor: %s, Running: %s\n", majorVersion, minorVersion, runningVersion)

	// Check if there is an optional label version.
	if len(parts) > 3 {
		labelVersion = parts[3]
		t.Logf("Debug: Parsed Label: %s\n", labelVersion)
	} else {
		t.Log("Debug: No label version found.")
	}

	return majorVersion, minorVersion, runningVersion, labelVersion, nil
}

// SupervisorSwitchover does a SSO on the dut device and also checkes the interfaces are up after SSO
func SupervisorSwitchover(t *testing.T, dut *ondatra.DUTDevice) {

	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
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

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
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

// Helper function to run a command on the DUT and return the output
func SshRunCommand(t *testing.T, dut *ondatra.DUTDevice, cmd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	sshClient := dut.RawAPIs().CLI(t)

	if result, err := sshClient.RunCommand(ctx, cmd); err == nil {
		t.Logf("%s> %s", dut.ID(), cmd)
		t.Log(result.Output())
		return result.Output()
	} else {
		t.Logf("%s> %s", dut.ID(), cmd)
		t.Log(err.Error())
		return ""
	}
}

// IsPlatformVXR checks if the platform is a VXR (true) or a HW (false)
func IsPlatformVXR(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) bool {
	resp := config.CMDViaGNMI(ctx, t, dut, "show version")
	t.Logf("Response: %s", resp)

	if strings.Contains(resp, "VXR") {
		t.Logf("Platform is VXR")
		return true
	}
	return false
}

// IsMajorVersionSame checks if the major version of the DUT is the same as the expected version.
func IsMajorVersionSame(t *testing.T, dut *ondatra.DUTDevice, expectedVersion string) (b bool, e error) {
	majorVersionDut, _, _, _, err := GetVersion(t, dut)
	if err != nil {
		return false, err
	}
	majorVersionExpected, _, _, _, err := splitVersionString(t, expectedVersion)
	if err != nil {
		return false, err
	}

	if majorVersionDut == majorVersionExpected {
		return true, nil
	}
	return
}

// IsMinorVersionSame checks if the minor version of the DUT is the same as the expected version.
func IsMinorVersionSame(t *testing.T, dut *ondatra.DUTDevice, expectedVersion string) (b bool, e error) {
	_, minorVersionDut, _, _, err := GetVersion(t, dut)
	if err != nil {
		return false, err
	}
	_, minorVersionExpected, _, _, err := splitVersionString(t, expectedVersion)
	if err != nil {
		return false, err
	}

	if minorVersionDut == minorVersionExpected {
		return true, nil
	}
	return
}

// EnableVxrInternalPxeBoot executes the command to initiate PXE boot on the device.
// only applicable for VXR platform
// reference: http://pyvxr.cisco.com/pyvxr/README.html#internal-pxe
func EnableVxrInternalPxeBoot(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) {
	if IsPlatformVXR(ctx, t, dut) {
		t.Log("Platform is VXR and version differs, initiating PXE boot.")
		sshClient := dut.RawAPIs().CLI(t)
		removeBootDirCmd := "run rm -r /boot/efi/EFI"
		removeBootDirCmdResult, err := sshClient.RunCommand(ctx, removeBootDirCmd)
		if err != nil {
			t.Error("failed to enable vxr internal PXE boot")
		}
		t.Logf("%s> %s\n%v", dut.Name(), removeBootDirCmd, removeBootDirCmdResult.Output())

		treeBootDirCmd := "run tree /boot/efi/EFI"
		treeBootDirCmdResult, err := sshClient.RunCommand(ctx, treeBootDirCmd)
		if err != nil {
			if strings.Contains(treeBootDirCmdResult.Output(), "BOOT") {
				t.Fatal("failed to remove /boot/efi/EFI")
			}
		}
		t.Logf("%s> %s\n%v", dut.Name(), treeBootDirCmd, treeBootDirCmdResult.Output())
	} else {
		t.Fatal("Error Not a VXR platform")
	}
}

func ParallelRPFO(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()

	var supervisors []string
	active_state := gnmi.OC().Component(active_rp).Name().State()
	active := gnmi.Get(t, dut, active_state)
	standby_state := gnmi.OC().Component(standby_rp).Name().State()
	standby := gnmi.Get(t, dut, standby_state)
	supervisors = append(supervisors, active, standby)

	// find active and standby RP
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, supervisors)
	t.Logf("Detected activeRP: %v, standbyRP: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch)

	// make sure standby RP is reach
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
	// gnoiClient := dut.RawAPIs().GNOI(t)
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	//useNameOnly := deviations.GNOISubcomponentPath(dut)
	useNameOnly := false
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
	if useNameOnly {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
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

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyControllerCard(t, dut, supervisors)
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
}

// CMDViaGNMI runs a command on the DUT via GNMI and returns the output
func CMDViaGNMI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cmd string) string {
	gnmiC := dut.RawAPIs().GNMI(t)
	getRequest := &gnmipb.GetRequest{
		Prefix: &gnmipb.Path{
			Origin: "cli",
		},
		Path: []*gnmipb.Path{
			{
				Elem: []*gnmipb.PathElem{{
					Name: cmd,
				}},
			},
		},
		Encoding: gnmipb.Encoding_ASCII,
	}
	log.V(1).Infof("get cli (%s) via GNMI: \n %s", cmd, prototext.Format(getRequest))
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Get(ctx, getRequest)
	if err != nil {
		t.Fatalf("running cmd (%s) via GNMI is failed: %v", cmd, err)
	}
	log.V(1).Infof("get cli via gnmi reply: \n %s", prototext.Format(resp))
	return string(resp.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
}
