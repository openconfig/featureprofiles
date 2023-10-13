// Copyright 2023 Google LLC
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

package gnmi_set_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"flag"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

var (
	// These flags skip unwanted test cases that can speed up development or debugging.
	skipRootOp      = flag.Bool("skip_root_op", false, "Skip RootOp test cases.")
	skipContainerOp = flag.Bool("skip_container_op", false, "Skip ContainerOp test cases.")
	skipItemOp      = flag.Bool("skip_item_op", false, "Skip ItemOp test cases.")

	// The following experimental flags fine-tune the RootOp and ContainerOp behavior.  Some
	// devices require the config to be pruned for these to work.  We are still undecided
	// whether they should be deviations; pending OpenConfig clarifications.
	pruneComponents      = flag.Bool("prune_components", true, "Prune components that are not ports.  Use this to preserve the breakout-mode settings.")
	pruneLLDP            = flag.Bool("prune_lldp", true, "Prune LLDP config.")
	setEthernetFromState = flag.Bool("set_ethernet_from_state", true, "Set interface/ethernet config from state, mostly to get the port-speed settings correct.")

	// This has no known effect except to reduce logspam while debugging.
	pruneQoS = flag.Bool("prune_qos", true, "Prune QoS config.")

	// Experimental flags that will likely become a deviation.
	cannotDeleteVRF = flag.Bool("cannot_delete_vrf", true, "Device cannot delete VRF.") // See "Note about cannotDeleteVRF" below.
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: 126,
	}
)

// Implementation Note
//
// Tests have three push variants: ItemOp, ContainerOp, and RootOp.
// The forEachPushOp construct allows us to share as much test code as possible, in a way
// that also preserves the baseline config of the DUT to avoid disrupting the management
// plane.
//
// While the test could modify the full config inside forEachPushOp, for each port and
// non-default VRF that Ondatra designates to the test, the test should do a pair of:
//
//   - DeleteInterface/GetOrCreateInterface
//   - DeleteNetworkInstance/GetOrCreateNetworkInstance (except the default VRF)
//
// So we can ensure that the content of these entities are reset to a clean slate and not
// polluted by the baseline.

// Note about cannotDeleteVRF
//
// If a device cannot delete a VRF, the initialization phase of a test will try to replace
// the VRF with an empty instance instead of deleting it, so the test is able to make
// progress.  Tests will still try to delete the VRF during cleanup.

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGetSet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	// Configuring basic interface and network instance as some devices only populate OC after configuration.
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Type().Config(),
		oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	scope := defaultPushScope(dut)

	forEachPushOp(t, dut, func(t *testing.T, op pushOp, config *oc.Root) {
		op.push(t, dut, config, scope)
		// TODO: after push, do a get again to check the config diff.
	})
}

func TestDeleteInterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	scope := defaultPushScope(dut)

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	q1 := gnmi.OC().Interface(p1.Name()).Description().State()
	q2 := gnmi.OC().Interface(p2.Name()).Description().State()

	const (
		want1 = "dut:port1"
		want2 = "dut:port2"
	)

	forEachPushOp(t, dut, func(t *testing.T, op pushOp, config *oc.Root) {
		t.Log("Initialize")

		config.GetOrCreateInterface(p1.Name()).Description = ygot.String(want1)
		config.GetOrCreateInterface(p2.Name()).Description = ygot.String(want2)
		op.push(t, dut, config, scope)

		t.Run("VerifyBeforeDelete", func(t *testing.T) {
			v1 := gnmi.Lookup(t, dut, q1)
			if got1, ok := v1.Val(); !ok || got1 != want1 {
				t.Errorf("State got %v, want %v", v1, want1)
			}
			v2 := gnmi.Lookup(t, dut, q2)
			if got2, ok := v2.Val(); !ok || got2 != want2 {
				t.Errorf("State got %v, want %v", v2, want2)
			}
		})

		t.Log("Delete Interfaces")

		config.DeleteInterface(p1.Name())
		config.DeleteInterface(p2.Name())
		op.push(t, dut, config, scope)

		t.Run("VerifyAfterDelete", func(t *testing.T) {
			if v := gnmi.Lookup(t, dut, q1); v.IsPresent() {
				t.Errorf("State got unwanted %v", v)
			}
			if v := gnmi.Lookup(t, dut, q2); v.IsPresent() {
				t.Errorf("State got unwanted %v", v)
			}
		})
	})
}

func TestReuseIP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	aggs := nextAggregates(t, dut, 2)
	agg1 := aggs[0]
	agg2 := aggs[1]

	t.Logf("Using dut:agg1 = %q, dut:agg2 = %q", agg1, agg2)

	scope := &pushScope{
		interfaces: []string{
			p1.Name(),
			p2.Name(),
			agg1,
			agg2,
		},
	}

	forEachPushOp(t, dut, func(t *testing.T, op pushOp, config *oc.Root) {
		t.Log("Initialize")

		config.DeleteInterface(p1.Name())
		config.DeleteInterface(agg1)
		configMember(config.GetOrCreateInterface(p1.Name()), agg1, dut)
		configAggregate(config.GetOrCreateInterface(agg1), &ip1, dut)

		config.DeleteInterface(p2.Name())
		config.DeleteInterface(agg2)
		configMember(config.GetOrCreateInterface(p2.Name()), agg2, dut)
		configAggregate(config.GetOrCreateInterface(agg2), &ip2, dut)

		op.push(t, dut, config, scope)

		t.Run("VerifyBeforeReuse", func(t *testing.T) {
			verifyMember(t, p1, agg1)
			verifyAggregate(t, dut, agg1, &ip1)

			verifyMember(t, p2, agg2)
			verifyAggregate(t, dut, agg2, &ip2)
		})

		t.Log("Modify Interfaces")

		config.Interface[p1.Name()].Ethernet.AggregateId = nil
		config.DeleteInterface(agg1)
		config.DeleteInterface(agg2)
		configAggregate(config.GetOrCreateInterface(agg2), &ip1, dut)

		op.push(t, dut, config, scope)

		t.Run("VerifyAfterReuse", func(t *testing.T) {
			verifyAggregate(t, dut, agg2, &ip1)
		})

		t.Log("Cleanup")

		config.Interface[p2.Name()].Ethernet.AggregateId = nil
		config.DeleteInterface(agg2)

		op.push(t, dut, config, scope)
	})
}

func TestSwapIPs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	scope := defaultPushScope(dut)

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	forEachPushOp(t, dut, func(t *testing.T, op pushOp, config *oc.Root) {
		t.Log("Initialize")

		config.DeleteInterface(p1.Name())
		config.DeleteInterface(p2.Name())
		ip1.ConfigOCInterface(config.GetOrCreateInterface(p1.Name()), dut)
		ip2.ConfigOCInterface(config.GetOrCreateInterface(p2.Name()), dut)

		op.push(t, dut, config, scope)

		t.Run("VerifyBeforeSwap", func(t *testing.T) {
			verifyInterface(t, dut, p1.Name(), &ip1)
			verifyInterface(t, dut, p2.Name(), &ip2)
		})

		t.Log("Modify Interfaces")

		config.DeleteInterface(p1.Name())
		config.DeleteInterface(p2.Name())
		ip2.ConfigOCInterface(config.GetOrCreateInterface(p1.Name()), dut)
		ip1.ConfigOCInterface(config.GetOrCreateInterface(p2.Name()), dut)

		op.push(t, dut, config, scope)

		t.Run("VerifyAfterSwap", func(t *testing.T) {
			verifyInterface(t, dut, p1.Name(), &ip2)
			verifyInterface(t, dut, p2.Name(), &ip1)
		})
	})
}

func TestDeleteNonExistingVRF(t *testing.T) {
	const vrf = "GREEN"

	dut := ondatra.DUT(t, "dut")
	scope := &pushScope{
		interfaces:       nil,
		networkInstances: []string{vrf},
	}

	forEachPushOp(t, dut, func(t *testing.T, op pushOp, config *oc.Root) {
		config.DeleteNetworkInstance(vrf)
		op.push(t, dut, config, scope)
	})
}

func TestDeleteNonDefaultVRF(t *testing.T) {
	const vrf = "BLUE"

	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	scope := &pushScope{
		interfaces:       []string{p1.Name(), p2.Name()},
		networkInstances: []string{vrf},
	}

	forEachPushOp(t, dut, func(t *testing.T, op pushOp, config *oc.Root) {
		t.Log("Initialize")

		config.DeleteInterface(p1.Name())
		config.DeleteInterface(p2.Name())

		ip1.ConfigOCInterface(config.GetOrCreateInterface(p1.Name()), dut)
		ip2.ConfigOCInterface(config.GetOrCreateInterface(p2.Name()), dut)

		config.DeleteNetworkInstance(vrf)
		ni := config.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF

		id1 := attachInterface(ni, p1.Name(), 0)
		id2 := attachInterface(ni, p2.Name(), 0)

		op.push(t, dut, config, scope)

		t.Run("Verify", func(t *testing.T) {
			verifyInterface(t, dut, p1.Name(), &ip1)
			verifyInterface(t, dut, p2.Name(), &ip2)
			verifyAttachment(t, dut, vrf, id1, p1.Name())
			verifyAttachment(t, dut, vrf, id2, p2.Name())
		})

		t.Log("Cleanup")

		config.DeleteNetworkInstance(vrf)
		op.push(t, dut, config, scope)

		t.Run("VerifyAfterCleanup", func(t *testing.T) {
			q := gnmi.OC().NetworkInstance(vrf).Type().State()
			if v := gnmi.Lookup(t, dut, q); v.IsPresent() {
				t.Errorf("State got unwanted %v", v)
			}
		})
	})
}

func TestMoveInterface(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("DefaultToNonDefaultVRF", func(t *testing.T) {
		testMoveInterfaceBetweenVRF(t, dut, deviations.DefaultNetworkInstance(dut), "BLUE")
	})
	t.Run("NonDefaultToNonDefaultVRF", func(t *testing.T) {
		testMoveInterfaceBetweenVRF(t, dut, "RED", "BLUE")
	})
}

func testMoveInterfaceBetweenVRF(t *testing.T, dut *ondatra.DUTDevice, firstVRF, secondVRF string) {
	defaultVRF := deviations.DefaultNetworkInstance(dut)

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	scope := &pushScope{
		interfaces:       []string{p1.Name(), p2.Name()},
		networkInstances: []string{firstVRF, secondVRF},
	}

	forEachPushOp(t, dut, func(t *testing.T, op pushOp, config *oc.Root) {
		t.Log("Initialize")

		config.DeleteInterface(p1.Name())
		config.DeleteInterface(p2.Name())
		ip1.ConfigOCInterface(config.GetOrCreateInterface(p1.Name()), dut)
		ip2.ConfigOCInterface(config.GetOrCreateInterface(p2.Name()), dut)

		if firstVRF != defaultVRF {
			config.DeleteNetworkInstance(firstVRF)
			ni := config.GetOrCreateNetworkInstance(firstVRF)
			ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		}

		firstni := config.GetOrCreateNetworkInstance(firstVRF)
		id1 := attachInterface(firstni, p1.Name(), 0)
		id2 := attachInterface(firstni, p2.Name(), 0)

		config.DeleteNetworkInstance(secondVRF)
		if *cannotDeleteVRF {
			ni := config.GetOrCreateNetworkInstance(secondVRF)
			ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		}

		op.push(t, dut, config, scope)

		t.Run("VerifyBeforeMove", func(t *testing.T) {
			verifyInterface(t, dut, p1.Name(), &ip1)
			verifyInterface(t, dut, p2.Name(), &ip2)
			verifyAttachment(t, dut, firstVRF, id1, p1.Name())
			verifyAttachment(t, dut, firstVRF, id2, p2.Name())

			// We don't check /network-instances/network-instance/vlans/vlan/members because
			// these are for L2 switched ports, not L3 routed ports.
		})

		t.Log("Modify Attachment")

		if firstVRF != defaultVRF {
			// It is not necessary to explicitly remove the interface attachments since the VRF
			// is being deleted.
			config.DeleteNetworkInstance(firstVRF)
		} else {
			// Remove just the interface attachments but keep the VRF.
			firstni.DeleteInterface(id1)
			firstni.DeleteInterface(id2)
		}

		secondni := config.GetOrCreateNetworkInstance(secondVRF)
		secondni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		attachInterface(secondni, p1.Name(), 0)
		attachInterface(secondni, p2.Name(), 0)

		op.push(t, dut, config, scope)

		t.Run("VerifyAfterMove", func(t *testing.T) {
			verifyInterface(t, dut, p1.Name(), &ip1)
			verifyInterface(t, dut, p2.Name(), &ip2)
			verifyAttachment(t, dut, secondVRF, id1, p1.Name())
			verifyAttachment(t, dut, secondVRF, id2, p2.Name())
		})

		t.Log("Cleanup")

		config.DeleteNetworkInstance(secondVRF)

		op.push(t, dut, config, scope)
	})
}

func TestStaticProtocol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	defaultVRF := deviations.DefaultNetworkInstance(dut)
	staticName := deviations.StaticProtocolName(dut)

	const (
		otherVRF  = "BLUE"
		unusedVRF = "RED"
		prefix1   = "198.51.100.0/24"
		prefix2   = "203.0.113.0/24"
		nhip1     = "192.0.2.2"
		nhip2     = "192.0.2.6"
	)

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	sp := gnmi.OC().NetworkInstance(otherVRF).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, staticName)
	q1 := sp.Static(prefix1).NextHop("0").InterfaceRef().Interface().State()
	q2 := sp.Static(prefix2).NextHop("0").InterfaceRef().Interface().State()

	scope := &pushScope{
		interfaces:       []string{p1.Name(), p2.Name()},
		networkInstances: []string{defaultVRF, otherVRF, unusedVRF},
	}

	forEachPushOp(t, dut, func(t *testing.T, op pushOp, config *oc.Root) {
		t.Log("Initialize")

		config.DeleteInterface(p1.Name())
		config.DeleteInterface(p2.Name())
		ip1.ConfigOCInterface(config.GetOrCreateInterface(p1.Name()), dut)
		ip2.ConfigOCInterface(config.GetOrCreateInterface(p2.Name()), dut)

		config.DeleteNetworkInstance(otherVRF)
		otherni := config.GetOrCreateNetworkInstance(otherVRF)
		otherni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF

		id1 := attachInterface(otherni, p1.Name(), 0)
		id2 := attachInterface(otherni, p2.Name(), 0)

		protocol := otherni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, staticName)

		nh1 := protocol.GetOrCreateStatic(prefix1).GetOrCreateNextHop("0")
		nh1.NextHop = oc.UnionString(nhip1)
		nh1.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())

		nh2 := protocol.GetOrCreateStatic(prefix2).GetOrCreateNextHop("0")
		nh2.NextHop = oc.UnionString(nhip2)
		nh2.GetOrCreateInterfaceRef().Interface = ygot.String(p2.Name())

		ni := config.GetOrCreateNetworkInstance(defaultVRF)
		ni.DeleteInterface(id1)
		ni.DeleteInterface(id2)

		// Avoid cascading failure when trying to remove unusedVRF leftover from
		// TestMoveInterface.
		config.DeleteNetworkInstance(unusedVRF)
		if *cannotDeleteVRF {
			ni := config.GetOrCreateNetworkInstance(unusedVRF)
			ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		}

		op.push(t, dut, config, scope)

		t.Run("VerifyBeforeModify", func(t *testing.T) {
			verifyInterface(t, dut, p1.Name(), &ip1)
			verifyInterface(t, dut, p2.Name(), &ip2)

			v1 := gnmi.Lookup(t, dut, q1)
			if got, ok := v1.Val(); !ok || got != p1.Name() {
				t.Errorf("State got %v, want %v", v1, p1.Name())
			} else {
				t.Logf("Verified %v", v1)
			}
			v2 := gnmi.Lookup(t, dut, q2)
			if got, ok := v2.Val(); !ok || got != p2.Name() {
				t.Errorf("State got %v, want %v", v2, p2.Name())
			} else {
				t.Logf("Verified %v", v2)
			}
		})

		t.Log("Modify Static Protocol")

		nh1.NextHop = oc.UnionString(nhip2)
		nh1.InterfaceRef.Interface = ygot.String(p2.Name())
		nh2.NextHop = oc.UnionString(nhip1)
		nh2.InterfaceRef.Interface = ygot.String(p1.Name())

		op.push(t, dut, config, scope)

		t.Run("VerifyAfterModify", func(t *testing.T) {
			verifyInterface(t, dut, p1.Name(), &ip1)
			verifyInterface(t, dut, p2.Name(), &ip2)

			v1 := gnmi.Lookup(t, dut, q1)
			if got, ok := v1.Val(); !ok || got != p2.Name() {
				t.Errorf("State got %v, want %v", v1, p2.Name())
			} else {
				t.Logf("Verified %v", v1)
			}
			v2 := gnmi.Lookup(t, dut, q2)
			if got, ok := v2.Val(); !ok || got != p1.Name() {
				t.Errorf("State got %v, want %v", v2, p1.Name())
			} else {
				t.Logf("Verified %v", v2)
			}
		})

		t.Log("Cleanup")

		config.DeleteNetworkInstance(otherVRF)
		config.DeleteNetworkInstance(unusedVRF)
		op.push(t, dut, config, scope)
	})
}

// Test Utilities for the Test Plan

var (
	ip1 = attrs.Attributes{IPv4: "192.0.2.1", IPv4Len: 30}
	ip2 = attrs.Attributes{IPv4: "192.0.2.5", IPv4Len: 30}
)

const (
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
)

var numRE = regexp.MustCompile(`(\d+)`)

// nextAggregates is like netutil.NextAggregateInterface but obtains multiple
// aggregate interfaces.
func nextAggregates(t *testing.T, dut *ondatra.DUTDevice, n int) []string {
	// netutil.NextAggregateInterface does not reserve an aggregate interface,
	// so it will return the same aggregate interface when called repeatedly.
	firstAgg := netutil.NextAggregateInterface(t, dut)
	start, err := strconv.Atoi(numRE.FindString(firstAgg))
	if err != nil {
		t.Fatalf("Cannot extract integer from %q: %v", firstAgg, err)
	}
	aggs := []string{firstAgg}
	for i := start + 1; i < start+n; i++ {
		agg := numRE.ReplaceAllStringFunc(firstAgg, func(_ string) string {
			return strconv.Itoa(i)
		})
		aggs = append(aggs, agg)
	}
	return aggs
}

// configMember configures an interface as a member of aggID interface.
func configMember(i *oc.Interface, aggID string, dut *ondatra.DUTDevice) {
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.Type = ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(aggID)
}

// configAggregate configures an interface as a STATIC LAG interface.
func configAggregate(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) {
	a.ConfigOCInterface(i, dut)

	// Overrides for LAG specific settings.
	i.Ethernet = nil
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = oc.IfAggregate_AggregationType_STATIC
}

// verifyMember verifies an interface as a member of aggID interface.
func verifyMember(t testing.TB, p *ondatra.Port, aggID string) {
	t.Helper()
	q := gnmi.OC().Interface(p.Name()).Ethernet().AggregateId().State()
	v := gnmi.Lookup(t, p.Device(), q)
	if got, ok := v.Val(); !ok || got != aggID {
		t.Errorf("State got %v, want %v", v, aggID)
	}
}

// verifyAggregate verifies an interface as a STATIC LAG aggregate.
func verifyAggregate(t testing.TB, dev gnmi.DeviceOrOpts, aggID string, a *attrs.Attributes) {
	t.Helper()
	q := gnmi.OC().Interface(aggID).Aggregation().LagType().State()
	v := gnmi.Lookup(t, dev, q)
	const want = oc.IfAggregate_AggregationType_STATIC
	if got, ok := v.Val(); !ok || got != want {
		t.Errorf("State got %v, want %v", v, want)
	}
	verifyInterface(t, dev, aggID, a)
}

// verifyInterface verifies the IP address configured on the interface.
func verifyInterface(t testing.TB, dev gnmi.DeviceOrOpts, name string, a *attrs.Attributes) {
	t.Helper()
	q := gnmi.OC().Interface(name).Subinterface(0).Ipv4().Address(a.IPv4).PrefixLength().State()
	v := gnmi.Lookup(t, dev, q)
	if got, ok := v.Val(); !ok || got != a.IPv4Len {
		t.Errorf("State got %v, want %v", v, a.IPv4Len)
	} else {
		t.Logf("Verified %v", v)
	}
}

// attachInterface attaches an interface name and subinterface sub to a network instance.
func attachInterface(ni *oc.NetworkInstance, name string, sub int) string {
	id := name // Possibly vendor specific?  May have to use sub.
	niface := ni.GetOrCreateInterface(id)
	niface.Interface = ygot.String(name)
	return id
}

// verifyAttachment verifies that an interface is attached to a VRF.  The id identifies
// the attachment returned by attachInterface, and name is the interface name.
func verifyAttachment(t testing.TB, dev gnmi.DeviceOrOpts, vrf string, id string, name string) {
	t.Helper()
	q := gnmi.OC().NetworkInstance(vrf).Interface(id).Interface().State()
	v := gnmi.Lookup(t, dev, q)
	if got, ok := v.Val(); !ok || got != name {
		t.Errorf("State got %v, want %v", v, name)
	} else {
		t.Logf("Verified %v", v)
	}
}

// Test Utilities for Config Push

// defaultPushScope builds a push scope that includes the Ondatra reserved DUT ports and
// the default network instance.  This excludes the DUT ports that are not part of the
// testbed reservation.
func defaultPushScope(dut *ondatra.DUTDevice) *pushScope {
	var interfaces []string
	for _, port := range dut.Ports() {
		interfaces = append(interfaces, port.Name())
	}

	return &pushScope{
		interfaces:       interfaces,
		networkInstances: []string{deviations.DefaultNetworkInstance(dut)},
	}
}

// baselineConfig and baselineConfigOnce let us remember the baseline configuration when the
// test first starts and only initialize once in forEachPushOp.
var (
	baselineConfig     *oc.Root
	baselineConfigOnce sync.Once
)

// forEachPushOp calls a test function with item, container and root push strategies and
// its own copy of the baseline config to modify.  The test function can modify and push
// this config as many times as it wants.
func forEachPushOp(
	t *testing.T,
	dut *ondatra.DUTDevice,
	f func(t *testing.T, op pushOp, config *oc.Root),
) {
	baselineConfigOnce.Do(func() {
		baselineConfig = getDeviceConfig(t, dut)
	})

	for _, op := range []pushOp{
		itemOp{baselineConfig}, containerOp{baselineConfig}, rootOp{baselineConfig},
	} {
		t.Run(op.string(), func(t *testing.T) {
			if op.shouldSkip() {
				t.Skip()
			}
			o, err := ygot.DeepCopy(baselineConfig)
			if err != nil {
				t.Fatalf("Cannot copy baseConfig: %v", err)
			}
			config := o.(*oc.Root)
			f(t, op, config)
		})
	}
}

// getDeviceConfig gets a full config from a device but refurbishes it enough so it can be
// pushed out again.  Ideally, we should be able to push the config we get from the same
// device without modification, but this is not explicitly defined in OpenConfig.
func getDeviceConfig(t testing.TB, dev gnmi.DeviceOrOpts) *oc.Root {
	t.Helper()

	// Gets all the config (read-write) paths from root, not the state (read-only) paths.
	config := gnmi.Get[*oc.Root](t, dev, gnmi.OC().Config())
	fptest.WriteQuery(t, "Untouched", gnmi.OC().Config(), config)

	if *pruneComponents {
		for cname, component := range config.Component {
			// Keep the port components in order to preserve the breakout-mode config.
			if component.GetPort() == nil {
				delete(config.Component, cname)
				continue
			}
			// Need to prune subcomponents that may have a leafref to a component that was
			// pruned.
			component.Subcomponent = nil
		}
	}

	if *setEthernetFromState {
		for iname, iface := range config.Interface {
			if iface.GetEthernet() == nil {
				continue
			}
			// Ethernet config may not contain meaningful values if it wasn't explicitly
			// configured, so use its current state for the config, but prune non-config leaves.
			intf := gnmi.Get(t, dev, gnmi.OC().Interface(iname).State())
			breakout := config.GetComponent(intf.GetHardwarePort()).GetPort().GetBreakoutMode()
			e := intf.GetEthernet()
			// Set port speed to unknown for non breakout interfaces
			if breakout.GetGroup(1) == nil && e != nil {
				e.SetPortSpeed(oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN)
			}
			ygot.PruneConfigFalse(oc.SchemaTree["Interface_Ethernet"], e)
			if e.PortSpeed != 0 && e.PortSpeed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
				iface.Ethernet = e
			}
		}
	}

	if *pruneLLDP && config.Lldp != nil {
		config.Lldp.ChassisId = nil
		config.Lldp.ChassisIdType = oc.Lldp_ChassisIdType_UNSET
	}

	if *pruneQoS {
		config.Qos = nil
	}

	pruneUnsupportedPaths(config)

	fptest.WriteQuery(t, "Touched", gnmi.OC().Config(), config)
	return config
}

func pruneUnsupportedPaths(config *oc.Root) {
	for _, ni := range config.NetworkInstance {
		ni.Fdb = nil
	}
}

// pushScope describe the config scope that the test case wants to modify.  This is for
// itemOp only; rootOp and containerOp ignore this.
type pushScope struct {
	interfaces       []string
	networkInstances []string
}

// pushOp describes a push operation.
type pushOp interface {
	string() string
	shouldSkip() bool
	push(t testing.TB, dev gnmi.DeviceOrOpts, config *oc.Root, scope *pushScope)
}

// setEthernetFromBase merges the ethernet config from the interfaces in base config into
// the destination config.
func setEthernetFromBase(t testing.TB, base *oc.Root, config *oc.Root) {
	t.Helper()

	for iname, iface := range config.Interface {
		eb := base.GetInterface(iname).GetEthernet()
		ec := iface.GetOrCreateEthernet()
		if eb == nil || ec == nil {
			continue
		}
		if err := ygot.MergeStructInto(ec, eb); err != nil {
			t.Errorf("Cannot merge %s ethernet: %v", iname, err)
		}
	}
}

// rootOp pushes config using replace at root.
type rootOp struct{ base *oc.Root }

func (rootOp) string() string   { return "RootOp" }
func (rootOp) shouldSkip() bool { return *skipRootOp }

func (op rootOp) push(t testing.TB, dev gnmi.DeviceOrOpts, config *oc.Root, _ *pushScope) {
	t.Helper()
	if *setEthernetFromState {
		setEthernetFromBase(t, op.base, config)
	}
	fptest.WriteQuery(t, "RootOp", gnmi.OC().Config(), config)
	gnmi.Replace(t, dev, gnmi.OC().Config(), config)
}

// containerOp pushes config using replace of containers of lists directly under root in
// the same SetRequest.
type containerOp struct{ base *oc.Root }

func (containerOp) string() string   { return "ContainerOp" }
func (containerOp) shouldSkip() bool { return *skipContainerOp }

func (op containerOp) push(t testing.TB, dev gnmi.DeviceOrOpts, config *oc.Root, _ *pushScope) {
	t.Helper()
	if *setEthernetFromState {
		setEthernetFromBase(t, op.base, config)
	}
	fptest.WriteQuery(t, "ContainerOp", gnmi.OC().Config(), config)

	batch := &gnmi.SetBatch{}
	gnmi.BatchReplace(batch, interfacesQuery, &Interfaces{Interface: config.Interface})
	gnmi.BatchReplace(batch, networkInstancesQuery, &NetworkInstances{NetworkInstance: config.NetworkInstance})
	batch.Set(t, dev)
}

// itemOp pushes individual configuration items in the same SetRequest.
type itemOp struct{ base *oc.Root }

func (itemOp) string() string   { return "ItemOp" }
func (itemOp) shouldSkip() bool { return *skipItemOp }

func (op itemOp) push(t testing.TB, dev gnmi.DeviceOrOpts, config *oc.Root, scope *pushScope) {
	t.Helper()
	if *setEthernetFromState {
		setEthernetFromBase(t, op.base, config)
	}
	fptest.WriteQuery(t, "ItemOp", gnmi.OC().Config(), config)

	batch := &gnmi.SetBatch{}
	var out strings.Builder
	fmt.Fprintln(&out, "ItemOp SetRequest:")

	for _, iname := range scope.interfaces {
		iface := config.GetInterface(iname)
		if iface != nil {
			fmt.Fprintf(&out, "  - Replace interface: %s\n", iname)
			gnmi.BatchReplace(batch, gnmi.OC().Interface(iname).Config(), iface)
		} else {
			fmt.Fprintf(&out, "  - Delete interface: %s\n", iname)
			gnmi.BatchDelete(batch, gnmi.OC().Interface(iname).Config())
		}
	}

	for _, niname := range scope.networkInstances {
		ni := config.GetNetworkInstance(niname)
		if ni != nil {
			fmt.Fprintf(&out, "  - Replace network-instance: %s\n", niname)
			gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(niname).Config(), ni)
		} else {
			fmt.Fprintf(&out, "  - Delete network-instance: %s\n", niname)
			gnmi.BatchDelete(batch, gnmi.OC().NetworkInstance(niname).Config())
		}
	}

	t.Log(out.String())
	batch.Set(t, dev)
}

// Reusable container queries.  These are the ygnmi queries representing the uncompressed
// paths.  Normally, the ygnmi queries only provides the compressed paths.
var (
	interfacesQuery       ygnmi.ConfigQuery[*Interfaces]       // Path: /interfaces
	networkInstancesQuery ygnmi.ConfigQuery[*NetworkInstances] // Path: /network-instances
)

func init() {
	var err error
	interfacesQuery, err = schemaless.NewConfig[*Interfaces]("/interfaces", "openconfig")
	if err != nil {
		panic(err)
	}
	networkInstancesQuery, err = schemaless.NewConfig[*NetworkInstances]("/network-instances", "openconfig")
	if err != nil {
		panic(err)
	}
}

type Interfaces struct {
	Interface map[string]*oc.Interface `path:"interface" module:"openconfig-interfaces"`
}

func (*Interfaces) IsYANGGoStruct() {}

type NetworkInstances struct {
	NetworkInstance map[string]*oc.NetworkInstance `path:"network-instance" module:"openconfig-network-instance"`
}

func (*NetworkInstances) IsYANGGoStruct() {}
