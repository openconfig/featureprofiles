// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package add_remove_policy_bound_interface_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	trafficDuration        = 1 * time.Minute
	plen4                  = 30
	plen6                  = 126
	vrf100Name             = "VRF-100"
	vrfSelectionPolicyName = "vrf100policy-ipv4"
	vrfPolicyv6            = "vrf100policy-ipv6"

	ipv4NetA    = "192.168.200.0/24"
	ipv6NetA    = "3008:DB8::/126"
	ipv4NetAdst = "192.168.200.1"
	ipv6NetAdst = "2001:DB2::2"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.168.100.1",
		IPv6:    "2001:DB2::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.168.100.2",
		IPv6:    "2001:DB2::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv4:    "192.168.100.5",
		IPv6:    "2001:DB2::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.168.100.6",
		IPv6:    "2001:DB2::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT Port 3",
		IPv4:    "192.168.100.9",
		IPv6:    "2001:DB2::9",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.168.100.10",
		IPv6:    "2001:DB2::a",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

// assignToNetworkInstance attaches a subinterface to a network instance.
func assignToNetworkInstance(t testing.TB, d *ondatra.DUTDevice, i string, ni string, si uint32) {
	t.Helper()
	if ni == "" {
		t.Fatalf("Network instance not provided for interface assignment")
	}
	netInst := &oc.NetworkInstance{Name: ygot.String(ni)}
	intf := &oc.Interface{Name: ygot.String(i)}
	netInstIntf, err := netInst.NewInterface(intf.GetName())
	if err != nil {
		t.Errorf("Error fetching NewInterface for %s", intf.GetName())
	}
	netInstIntf.Interface = ygot.String(intf.GetName())
	netInstIntf.Subinterface = ygot.Uint32(si)
	id := intf.GetName()
	if deviations.RequireRoutedSubinterface0(d) {
		id = intf.GetName() + "." + fmt.Sprint(si)
	}
	netInstIntf.Id = ygot.String(id)
	if intf.GetOrCreateSubinterface(si) != nil {
		gnmi.Update(t, d, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}

func configInterfaceDUT(i *oc.Interface, dutPort *attrs.Attributes, dut *ondatra.DUTDevice, applyIP bool) *oc.Interface {
	i.Description = ygot.String(dutPort.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	if deviations.RequireRoutedSubinterface0(dut) {
		s0 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		if !deviations.IPv4MissingEnabled(dut) {
			s0.Enabled = ygot.Bool(true)
		}
	}

	if applyIP {
		s := i.GetOrCreateSubinterface(0)
		ifv4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			ifv4.Enabled = ygot.Bool(true)
		}
		ifv4.GetOrCreateAddress(dutPort.IPv4).PrefixLength = ygot.Uint8(dutPort.IPv4Len)
		ifv6 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			ifv6.Enabled = ygot.Bool(true)
		}
		ifv6.GetOrCreateAddress(dutPort.IPv6).PrefixLength = ygot.Uint8(dutPort.IPv6Len)
	}
	return i
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, attrs *attrs.Attributes, niName string) {
	d := gnmi.OC()
	i := &oc.Interface{Name: ygot.String(p.Name())}
	t.Logf("*** Configuring int interface %s on DUT ...", p.Name())
	isNonDefaultVRF := niName != deviations.DefaultNetworkInstance(dut)
	applyIPInitially := !deviations.InterfaceConfigVRFBeforeAddress(dut) && !isNonDefaultVRF
	gnmi.Update(t, dut, d.Interface(p.Name()).Config(), configInterfaceDUT(i, attrs, dut, applyIPInitially))
	if isNonDefaultVRF || deviations.ExplicitInterfaceInDefaultVRF(dut) {
		assignToNetworkInstance(t, dut, p.Name(), niName, 0)
	}
	if deviations.InterfaceConfigVRFBeforeAddress(dut) || isNonDefaultVRF {
		configInterfaceDUT(i, attrs, dut, true)
		gnmi.Update(t, dut, d.Interface(p.Name()).Config(), i)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	t.Logf("Configuring VRF %s", vrf100Name)
	ni100 := cfgplugins.ConfigureNetworkInstance(t, dut, vrf100Name, false)
	ni100.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrf100Name, ni100)
	// Configure ingress interface
	t.Logf("*** Configuring interfaces on DUT ...")
	configureDUTPort(t, dut, p2, &dutPort2, deviations.DefaultNetworkInstance(dut))
	configureDUTPort(t, dut, p3, &dutPort3, deviations.DefaultNetworkInstance(dut))
	configureDUTPort(t, dut, p1, &dutPort1, vrf100Name)

	configureStaticRoutes(t, dut, "VRF-100", map[string]string{
		ipv4NetA: atePort1.IPv4,
		ipv6NetA: atePort1.IPv6,
	})

	configurePBFPolicy(t, dut)
}

func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, vrfName string, routes map[string]string) {
	t.Logf("*** Configuring static routes in %s network-instance ...", vrfName)
	ni := oc.NetworkInstance{Name: ygot.String(vrfName)}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

	for prefix, nextHop := range routes {
		sr := static.GetOrCreateStatic(prefix)
		nh := sr.GetOrCreateNextHop("0")
		nh.NextHop = oc.UnionString(nextHop)
	}
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

func configurePBFPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Logf("Configuring PBF policies")
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()

	// VRF Selection Policy for IPv4
	p4 := pf.GetOrCreatePolicy(vrfSelectionPolicyName)
	p4.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	r10 := p4.GetOrCreateRule(10)
	r10.GetOrCreateIpv4().SetSourceAddress(atePort2.IPv4 + "/32")
	r10.GetOrCreateAction().NetworkInstance = ygot.String(vrf100Name)
	r11 := p4.GetOrCreateRule(20)
	r11.GetOrCreateIpv4().SetSourceAddress(atePort3.IPv4 + "/32")
	r11.GetOrCreateAction().NetworkInstance = ygot.String(vrf100Name)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(vrfSelectionPolicyName).Config(), p4)

	// VRF Selection Policy for IPv6
	p6 := pf.GetOrCreatePolicy(vrfPolicyv6)
	p6.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	r20 := p6.GetOrCreateRule(10)
	r20.GetOrCreateIpv6().SetSourceAddress(atePort2.IPv6 + "/128")
	r20.GetOrCreateAction().NetworkInstance = ygot.String(vrf100Name)
	r21 := p6.GetOrCreateRule(20)
	r21.GetOrCreateIpv6().SetSourceAddress(atePort3.IPv6 + "/128")
	r21.GetOrCreateAction().NetworkInstance = ygot.String(vrf100Name)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(vrfPolicyv6).Config(), p6)
}

func applyPolicy(t *testing.T, dut *ondatra.DUTDevice, intfName string, policyNames []string) {
	t.Helper()
	d := &oc.Root{}
	ni := deviations.DefaultNetworkInstance(dut)
	interfaceID := intfName
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = intfName + ".0"
	}
	path := gnmi.OC().NetworkInstance(ni).PolicyForwarding().Interface(interfaceID)

	if len(policyNames) == 0 {
		t.Logf("No policies provided to apply for interface %s", intfName)
		return
	}
	policyName := policyNames[0]

	intf := d.GetOrCreateNetworkInstance(ni).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
	intf.InterfaceId = ygot.String(interfaceID)
	intf.ApplyVrfSelectionPolicy = ygot.String(policyName)
	if !deviations.InterfaceRefConfigUnsupported(dut) {
		intf.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
		intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	}

	c, err := ygot.Marshal7951(intf)
	if err != nil {
		t.Errorf("Error marshaling policy-forwarding interface: %v", err)
	}
	t.Logf("Applying policy %s to interface %s using apply-forwarding-policy config: %s", policyName, interfaceID, string(c))
	gnmi.Replace(t, dut, path.Config(), intf)

}

func deletePolicy(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	t.Helper()
	interfaceID := intfName
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = intfName + ".0"
	}
	t.Logf("Deleting policy from interface %s", interfaceID)
	ni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(ni).PolicyForwarding().Interface(interfaceID)
	gnmi.Delete(t, dut, path.Config())
}

type flow struct {
	name     string
	wantLoss bool
}

type ateTraffic struct {
	flows map[string]gosnappi.Flow
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) (gosnappi.Config, *ateTraffic) {
	t.Helper()
	topo := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	atePort1.AddToOTG(topo, p1, &dutPort1)
	atePort2.AddToOTG(topo, p2, &dutPort2)
	atePort3.AddToOTG(topo, p3, &dutPort3)

	t.Logf("*** Configuring OTG flows ...")
	flows := make(map[string]gosnappi.Flow)
	// v4FlowPort2: atePort2 -> atePort1
	flows["v4FlowPort2"] = createFlow(t, topo, "v4FlowPort2", &atePort2, &atePort1, ipv4NetAdst, "IPv4")
	// v4FlowPort3: atePort3 -> atePort1
	flows["v4FlowPort3"] = createFlow(t, topo, "v4FlowPort3", &atePort3, &atePort1, ipv4NetAdst, "IPv4")
	// v6FlowPort2: atePort2 -> atePort1
	flows["v6FlowPort2"] = createFlow(t, topo, "v6FlowPort2", &atePort2, &atePort1, ipv6NetAdst, "IPv6")
	// v6FlowPort3: atePort3 -> atePort1
	flows["v6FlowPort3"] = createFlow(t, topo, "v6FlowPort3", &atePort3, &atePort1, ipv6NetAdst, "IPv6")

	t.Logf("Pushing config to ATE and starting protocols...")
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	return topo, &ateTraffic{flows}
}

func createFlow(t *testing.T, top gosnappi.Config, name string, src, dst *attrs.Attributes, dstIP, ipType string) gosnappi.Flow {
	flow := top.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{src.Name + "." + ipType}).SetRxNames([]string{dst.Name + "." + ipType})
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(src.MAC)
	if ipType == "IPv4" {
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(src.IPv4)
		ip.Dst().SetValue(dstIP)
	} else {
		ip := flow.Packet().Add().Ipv6()
		ip.Src().SetValue(src.IPv6)
		ip.Dst().SetValue(dstIP)
	}
	flow.Size().SetFixed(512)
	flow.Rate().SetPps(100)
	return flow
}

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, flows []flow) {
	t.Helper()
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), topo)
	for _, f := range flows {
		t.Run(f.name, func(t *testing.T) {
			flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.name).State())
			outPkts := flowMetrics.GetCounters().GetOutPkts()
			inPkts := flowMetrics.GetCounters().GetInPkts()
			lostPkts := outPkts - inPkts
			if outPkts == 0 {
				t.Fatalf("Sent packets for flow %s is 0", f.name)
			}
			if f.wantLoss {
				if lostPkts != outPkts {
					t.Errorf("Flow %s got %d/%d packets, want 100%% loss", f.name, inPkts, outPkts)
				}
			} else {
				if lostPkts > 0 {
					t.Errorf("Flow %s got %d/%d packets, want 0%% loss", f.name, inPkts, outPkts)
				}
			}
		})
	}
}

func TestPolicyBoundInterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)
	topo, _ := configureATE(t, ate)

	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	t.Run("IPv4 Policy", func(t *testing.T) {
		t.Log("Applying IPv4 policy")
		applyPolicy(t, dut, p2.Name(), []string{vrfSelectionPolicyName})
		applyPolicy(t, dut, p3.Name(), []string{vrfSelectionPolicyName})
		t.Run("Initial traffic check IPv4", func(t *testing.T) {
			flows := []flow{
				{name: "v4FlowPort2", wantLoss: false},
				{name: "v4FlowPort3", wantLoss: false},
			}
			verifyTraffic(t, ate, topo, flows)
		})

		t.Run("PF-1.24.1 Remove interface from policy IPv4", func(t *testing.T) {
			deletePolicy(t, dut, p2.Name())
			flows := []flow{
				{name: "v4FlowPort2", wantLoss: true},
				{name: "v4FlowPort3", wantLoss: false},
			}
			verifyTraffic(t, ate, topo, flows)
		})

		t.Run("PF-1.24.2 Add interface to policy IPv4", func(t *testing.T) {
			applyPolicy(t, dut, p2.Name(), []string{vrfSelectionPolicyName})
			flows := []flow{
				{name: "v4FlowPort2", wantLoss: false},
				{name: "v4FlowPort3", wantLoss: false},
			}
			verifyTraffic(t, ate, topo, flows)
		})

		t.Run("PF-1.24.3 Remove interface from policy and device IPv4", func(t *testing.T) {
			deletePolicy(t, dut, p2.Name())
			if deviations.ExplicitInterfaceInDefaultVRF(dut) {
				gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Interface(p2.Name()).Config())
				gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Interface(p2.Name()+".0").Config())
			}
			gnmi.Delete(t, dut, gnmi.OC().Interface(p2.Name()).Subinterface(0).Config())
			flows := []flow{
				{name: "v4FlowPort2", wantLoss: true},
				{name: "v4FlowPort3", wantLoss: false},
			}
			verifyTraffic(t, ate, topo, flows)
		})
	})

	t.Log("Reconfiguring Port2 for IPv6 tests")
	d := gnmi.OC()
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	configInterfaceDUT(i2, &dutPort2, dut, true)
	gnmi.Update(t, dut, d.Interface(p2.Name()).Config(), i2)
	gnmi.Update(t, dut, d.Interface(p2.Name()).Subinterface(0).Config(), i2.Subinterface[0])
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		assignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	t.Run("IPv6 Policy", func(t *testing.T) {
		t.Log("Applying IPv6 policy")
		applyPolicy(t, dut, p2.Name(), []string{vrfPolicyv6})
		applyPolicy(t, dut, p3.Name(), []string{vrfPolicyv6})

		ate.OTG().StopProtocols(t)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
		t.Run("Initial traffic check IPv6", func(t *testing.T) {
			flows := []flow{
				{name: "v4FlowPort2", wantLoss: true},
				{name: "v4FlowPort3", wantLoss: true},
				{name: "v6FlowPort2", wantLoss: false},
				{name: "v6FlowPort3", wantLoss: false},
			}
			verifyTraffic(t, ate, topo, flows)
		})

		t.Run("PF-1.24.1 Remove interface from policy IPv6", func(t *testing.T) {
			deletePolicy(t, dut, p2.Name())
			flows := []flow{
				{name: "v6FlowPort2", wantLoss: true},
				{name: "v6FlowPort3", wantLoss: false},
			}
			verifyTraffic(t, ate, topo, flows)
		})

		t.Run("PF-1.24.2 Add interface to policy IPv6", func(t *testing.T) {
			applyPolicy(t, dut, p2.Name(), []string{vrfPolicyv6})
			flows := []flow{
				{name: "v6FlowPort2", wantLoss: false},
				{name: "v6FlowPort3", wantLoss: false},
			}
			verifyTraffic(t, ate, topo, flows)
		})

		t.Run("PF-1.24.3 Remove interface from policy and device IPv6", func(t *testing.T) {
			deletePolicy(t, dut, p2.Name())
			if deviations.ExplicitInterfaceInDefaultVRF(dut) {
				gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Interface(p2.Name()).Config())
				gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Interface(p2.Name()+".0").Config())
			}
			gnmi.Delete(t, dut, gnmi.OC().Interface(p2.Name()).Subinterface(0).Config())
			flows := []flow{
				{name: "v6FlowPort2", wantLoss: true},
				{name: "v6FlowPort3", wantLoss: false},
			}
			verifyTraffic(t, ate, topo, flows)
		})
	})
}
