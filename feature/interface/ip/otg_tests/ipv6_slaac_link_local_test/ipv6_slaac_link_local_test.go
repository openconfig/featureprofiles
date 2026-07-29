package ipv6_slaac_link_local_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

var (
	ipv6BySLAAC     = `fe80::.+/64`
	intfDesc        = "dutInfSLAAC"
	waitForAssigned = time.Minute
	reIPv6BySLAAC   = regexp.MustCompile(ipv6BySLAAC)
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureDUTLinkLocalInterface(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) {
	t.Helper()

	intf := &oc.Interface{Name: ygot.String(p.Name())}
	intf.Description = ygot.String(intfDesc)
	intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := intf.GetOrCreateSubinterface(0)
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s.GetOrCreateIpv4().SetEnabled(true)
	}
	if deviations.InterfaceEnabled(dut) {
		s.GetOrCreateIpv6().SetEnabled(true)
	}
	s.GetOrCreateIpv6().GetOrCreateAutoconf()
	gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Config(), intf)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, intf.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func getAllIPv6Addresses(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) []string {
	var allIPv6 []string
	deadline := time.Now().Add(waitForAssigned)
	for time.Now().Before(deadline) {
		time.Sleep(10 * time.Second)
		ipv6Addrs := gnmi.LookupAll(t, dut, gnmi.OC().Interface(p.Name()).Subinterface(0).Ipv6().AddressAny().State())
		t.Logf("number of ipv6: %d", len(ipv6Addrs))
		for _, ipv6Addr := range ipv6Addrs {
			t.Logf("ipv6Addr: %v", ipv6Addr)
			if v6, ok := ipv6Addr.Val(); ok {
				allIPv6 = append(allIPv6, fmt.Sprintf("%s/%d", v6.GetIp(), v6.GetPrefixLength()))
				t.Logf("allIPv6: %v", allIPv6)
			}
		}
		if hasSLAACGeneratedAddress(allIPv6) {
			break
		}
	}
	return allIPv6
}

func hasSLAACGeneratedAddress(ipv6Addrs []string) bool {
	for _, ipv6Addr := range ipv6Addrs {
		if reIPv6BySLAAC.MatchString(ipv6Addr) {
			return true
		}
	}
	return false
}

func TestIpv6LinkLocakGenBySLAAC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	configureDUTLinkLocalInterface(t, dut, p1)
	ipv6 := getAllIPv6Addresses(t, dut, p1)
	if deviations.SlaacPrefixLength128(dut) {
		ipv6BySLAAC = `fe80::.+/128`
		reIPv6BySLAAC = regexp.MustCompile(ipv6BySLAAC)
		t.Logf("ipv6BySLAAC: %s, reIPv6BySLAAC: %s", ipv6BySLAAC, reIPv6BySLAAC)
		found := false
		for _, ipv6Addr := range ipv6 {
			if reIPv6BySLAAC.MatchString(ipv6Addr) {
				t.Logf("SLAAC generated IPv6 address found, ")
				found = true
				break
			}
		}
		if !found {
			t.Errorf("No SLAAC generated IPv6 address found ")
		}
	} else {
		if !hasSLAACGeneratedAddress(ipv6) {
			t.Errorf("No SLAAC generated IPv6 address found , got: %s, want: %s", ipv6, ipv6BySLAAC)
		}
	}
}
