package policy_test

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func flushServer(t *testing.T, args *testArgs) {
	c := args.clientA.Fluent(t)
	if _, err := c.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}
}

func configureBaseDoubleRecusionVip1Entry(ctx context.Context, t *testing.T, args *testArgs) {
	t.Helper()
	c := args.clientA.Fluent(t)
	c.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithIndex(args.prefix.vip1NhIndex+2).WithIPAddress(atePort2.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithIndex(args.prefix.vip1NhIndex+3).WithIPAddress(atePort3.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithIndex(args.prefix.vip1NhIndex+4).WithIPAddress(atePort4.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithID(args.prefix.vip1NhgIndex+1).
			AddNextHop(args.prefix.vip1NhIndex+2, 10).
			AddNextHop(args.prefix.vip1NhIndex+3, 20).
			AddNextHop(args.prefix.vip1NhIndex+4, 30),
		fluent.IPv4Entry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithPrefix(util.GetIPPrefix(args.prefix.vip1Ip, 0, args.prefix.vipPrefixLength)).WithNextHopGroup(args.prefix.vip1NhgIndex+1),
	)

	if err := args.clientA.AwaitTimeout(ctx, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", c, err)
	}

	// Verification part
	// NH Verification
	for _, nhIndex := range []uint64{args.prefix.vip1NhIndex + 2, args.prefix.vip1NhIndex + 3, args.prefix.vip1NhIndex + 4} {
		chk.HasResult(t, c.Results(t),
			fluent.OperationResult().
				WithNextHopOperation(nhIndex).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInRIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// NHG Verification
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(args.prefix.vip1NhgIndex+1).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	// IPv4
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().WithIPv4Operation(util.GetIPPrefix(args.prefix.vip1Ip, 0, args.prefix.vipPrefixLength)).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

}

func configureBaseDoubleRecusionVip2Entry(ctx context.Context, t *testing.T, args *testArgs) {
	t.Helper()
	c := args.clientA.Fluent(t)
	c.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithIndex(args.prefix.vip2NhIndex+5).WithIPAddress(atePort5.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithIndex(args.prefix.vip2NhIndex+6).WithIPAddress(atePort6.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithIndex(args.prefix.vip2NhIndex+7).WithIPAddress(atePort7.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithIndex(args.prefix.vip2NhIndex+8).WithIPAddress(atePort8.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithID(args.prefix.vip2NhgIndex+1).
			AddNextHop(args.prefix.vip2NhIndex+5, 10).
			AddNextHop(args.prefix.vip2NhIndex+6, 20).
			AddNextHop(args.prefix.vip2NhIndex+7, 30).
			AddNextHop(args.prefix.vip2NhIndex+8, 40),
		fluent.IPv4Entry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithPrefix(util.GetIPPrefix(args.prefix.vip2Ip, 0, args.prefix.vipPrefixLength)).WithNextHopGroup(args.prefix.vip2NhgIndex+1),
	)

	if err := args.clientA.AwaitTimeout(ctx, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", c, err)
	}

	// Verification part
	// NH Verification
	for _, nhIndex := range []uint64{args.prefix.vip2NhIndex + 5, args.prefix.vip2NhIndex + 6, args.prefix.vip2NhIndex + 7, args.prefix.vip2NhIndex + 8} {
		chk.HasResult(t, c.Results(t),
			fluent.OperationResult().
				WithNextHopOperation(nhIndex).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInRIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// NHG Verification
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(args.prefix.vip2NhgIndex+1).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	// IPv4
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().WithIPv4Operation(util.GetIPPrefix(args.prefix.vip2Ip, 0, args.prefix.vipPrefixLength)).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

func configureBaseDoubleRecusionVrfEntry(ctx context.Context, t *testing.T, scale int, hostIP, prefixLength string, args *testArgs) {
	t.Helper()
	c := args.clientA.Fluent(t)
	c.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithIndex(args.prefix.vrfNhIndex+1).WithIPAddress(args.prefix.vip1Ip),
		fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithIndex(args.prefix.vrfNhIndex+2).WithIPAddress(args.prefix.vip2Ip),
		fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.PbrInstance).WithID(args.prefix.vrfNhgIndex+1).
			AddNextHop(args.prefix.vrfNhIndex+1, 15).
			AddNextHop(args.prefix.vrfNhIndex+2, 85),
	)
	entries := []fluent.GRIBIEntry{}
	for i := 0; i < scale; i++ {
		entries = append(entries,
			fluent.IPv4Entry().
				WithNetworkInstance(args.prefix.vrfName).
				WithPrefix(util.GetIPPrefix(hostIP, i, prefixLength)).
				WithNextHopGroup(args.prefix.vrfNhgIndex+1).
				WithNextHopGroupNetworkInstance(*ciscoFlags.PbrInstance))
	}
	c.Modify().AddEntry(t, entries...)

	if err := args.clientA.AwaitTimeout(ctx, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via %v, got err: %v", c, err)
	}

	// Verification part
	// NH Verification
	for _, nhIndex := range []uint64{args.prefix.vrfNhIndex + 1, args.prefix.vrfNhIndex + 2} {
		chk.HasResult(t, c.Results(t),
			fluent.OperationResult().
				WithNextHopOperation(nhIndex).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInRIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// NHG Verification
	chk.HasResult(t, c.Results(t),
		fluent.OperationResult().
			WithNextHopGroupOperation(args.prefix.vrfNhgIndex+1).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	// IPv4
	for i := 0; i < scale; i++ {
		chk.HasResult(t, c.Results(t),
			fluent.OperationResult().WithIPv4Operation(util.GetIPPrefix(hostIP, i, prefixLength)).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInRIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

func createNameSpace(t *testing.T, dut *ondatra.DUTDevice, name, intfname string, subint uint32) {
	//create empty subinterface
	si := &oc.Interface_Subinterface{}
	si.Index = ygot.Uint32(subint)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intfname).Subinterface(subint).Config(), si)

	//create vrf and apply on subinterface
	v := &oc.NetworkInstance{
		Name: ygot.String(name),
	}
	vi := v.GetOrCreateInterface(intfname + "." + strconv.Itoa(int(subint)))
	vi.Subinterface = ygot.Uint32(subint)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(name).Config(), v)
}

func getSubInterface(ipv4 string, prefixlen4 uint8, ipv6 string, prefixlen6 uint8, vlanID uint16, index uint32) *oc.Interface_Subinterface {
	s := &oc.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen4)
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(ipv6)
	a6.PrefixLength = ygot.Uint8(prefixlen6)
	v := s.GetOrCreateVlan()
	m := v.GetOrCreateMatch()
	if index != 0 {
		m.GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
	}
	return s
}

func addIpv6Address(i *oc.Interface, ipv6 string, prefixlen uint8, index uint32) *oc.Interface {
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	s := i.GetOrCreateSubinterface(index)
	s4 := s.GetOrCreateIpv6()
	s4a := s4.GetOrCreateAddress(ipv6)
	s4a.PrefixLength = ygot.Uint8(prefixlen)

	return i
}
func configureIpv6AndVlans(t *testing.T, dut *ondatra.DUTDevice) {
	//Configure IPv6 address on Bundle-Ether120, Bundle-Ether121
	i1 := &oc.Interface{Name: ygot.String("Bundle-Ether120")}
	i2 := &oc.Interface{Name: ygot.String("Bundle-Ether121")}
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether120").Config(), addIpv6Address(i1, dutPort1.IPv6, dutPort1.IPv6Len, 0))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether121").Config(), addIpv6Address(i2, dutPort2.IPv6, dutPort2.IPv6Len, 0))

	//Configure VLANs on Bundle-Ether121
	for i := 1; i <= 3; i++ {
		//Create VRFs and VRF enabled subinterfaces
		createNameSpace(t, dut, fmt.Sprintf("VRF%d", i*10), "Bundle-Ether121", uint32(i))
		//Add IPv4/IPv6 address on VLANs
		subint := getSubInterface(fmt.Sprintf("100.121.%d.1", i*10), 24, fmt.Sprintf("2000::100:121:%d:1", i*10), 126, uint16(i*10), uint32(i))
		gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether121").Subinterface(uint32(i)).Config(), subint)
	}

}

// SortPorts sorts the ports by their ID in the testbed.  Otherwise
// Ondatra returns the ports in arbitrary order.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		idi, idj := ports[i].ID(), ports[j].ID()
		if len(idi) < len(idj) {
			return true // "port2" < "port10"
		}
		return idi < idj
	})
	return ports
}
