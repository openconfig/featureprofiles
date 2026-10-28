// Copyright 2026 Google LLC
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

// Package mvc_control_traffic_qos_test implements MGT-2: QoS of MVC Control and Management Traffic.
package mvc_control_traffic_qos_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
)

const (
	// Management VRF and BGP autonomous system numbers (RFC 5398).
	mgmtVRFName = "mgmt"
	dutAS       = 64512
	ateAS       = 64511

	// Addressing constants (RFC 5737 / RFC 3849).
	ipv4PrefixLen     = 30
	ipv6PrefixLen     = 126
	lo1IPv4           = "192.0.2.101"
	lo1IPv4Prefix     = "192.0.2.101/32"
	lo1IPv4Len        = 32
	lo1IPv6           = "2001:db8::101"
	lo1IPv6Prefix     = "2001:db8::101/128"
	lo1IPv6Len        = 128
	collectorIPv4     = "198.51.100.1"
	collectorIPv4Net  = "198.51.100.0"
	collectorIPv4Len  = 24
	collectorIPv6     = "2001:db8:100::1"
	collectorIPv6Net  = "2001:db8:100::"
	collectorIPv6Len  = 64
	defaultIPv4Route  = "0.0.0.0/0"
	defaultIPv4Net    = "0.0.0.0"
	defaultIPv6Route  = "::/0"
	defaultIPv6Net    = "::"
	staticRouteIdx    = "1"
	backupPreference  = 220
	unresolvedHost    = "mgmt-target.example.com"
	tacacsGroupName   = "TACACS-MGMT"
	tacacsSecret      = "tacacs-secret"
	grpcServerName    = "mgmt-grpc"
	qosSchedulerName  = "mgmt_qos_sched"
	classifierV4Name  = "dscp_af4_v4"
	classifierV6Name  = "dscp_af4_v6"
	redistPolicyV4    = "redist_lo1_v4"
	redistStmtV4      = "allow_lo1_v4"
	prefixSetV4       = "lo1_v4_prefix_set"
	redistPolicyV6    = "redist_lo1_v6"
	redistStmtV6      = "allow_lo1_v6"
	prefixSetV6       = "lo1_v6_prefix_set"
	bgpAllowPolicy    = "ALLOW"
	bgpAllowStatement = "allow_all"

	// QoS DSCP constants.
	dscpAF4 = 32
	dscpCS1 = 8
	dscpBE1 = 0

	// Protocol port numbers.
	portNTP    = 123
	portTACACS = 49
	portSSH    = 22
	portDNS    = 53
	portSFlow  = 6343
	portSyslog = 514
	portP4RT   = 9559
	portGRIBI  = 9340
	portGNMI   = 9339
)

var (
	// <DUT_PORT1> <-> <ATE_PORT1>: In-band Linecard (LC) management link 1 (mgmt VRF).
	dutPort1 = attrs.Attributes{
		Desc:    "In-band LC management link 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// <DUT_PORT2> <-> <ATE_PORT2>: In-band Linecard (LC) management link 2 (mgmt VRF).
	dutPort2 = attrs.Attributes{
		Desc:    "In-band LC management link 2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// <DUT_PORT3> <-> <ATE_PORT3>: Out-of-Band (OOB) management fallback link (mgmt VRF).
	dutPort3 = attrs.Attributes{
		Desc:    "OOB management fallback link",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// <DUT_PORT4> <-> <ATE_PORT4>: Data-plane transit link (DEFAULT VRF).
	dutPort4 = attrs.Attributes{
		Desc:    "Data-plane transit link",
		IPv4:    "192.0.2.13",
		IPv6:    "2001:db8::d",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.14",
		IPv6:    "2001:db8::e",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// <DUT_LO1>: Management loopback bound to VRF mgmt.
	dutLo1 = attrs.Attributes{
		Desc:    "Management loopback in VRF mgmt",
		IPv4:    lo1IPv4,
		IPv6:    lo1IPv6,
		IPv4Len: lo1IPv4Len,
		IPv6Len: lo1IPv6Len,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// queueCounters holds a snapshot of transmit packets, transmit octets, and dropped packets for a queue.
type queueCounters struct {
	txPkts      uint64
	txOctets    uint64
	droppedPkts uint64
}

// testEnv encapsulates the testbed devices, OTG configuration, and resolved queue/interface names.
type testEnv struct {
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	top      gosnappi.Config
	lo1Name  string
	af4Queue string
	be1Queue string
}

// resolveQueueName returns the platform-specific output queue name on the given interface.
func resolveQueueName(dut *ondatra.DUTDevice, intfName, stdQueue string, queueIdx int) string {
	if deviations.InterfaceOutputQueueNonStandardName(dut) {
		return intfName + "-" + strconv.Itoa(queueIdx)
	}
	return stdQueue
}

// snapshotQueueCounters reads the current output queue counters on the given DUT port.
func snapshotQueueCounters(t *testing.T, dut *ondatra.DUTDevice, portName, queueName string) queueCounters {
	t.Helper()
	qPath := gnmi.OC().Qos().Interface(portName).Output().Queue(queueName)
	var snap queueCounters
	if val, ok := gnmi.Watch(t, dut, qPath.TransmitPkts().State(), 15*time.Second, func(v *ygnmi.Value[uint64]) bool {
		return v.IsPresent()
	}).Await(t); ok {
		snap.txPkts, _ = val.Val()
	}
	if !deviations.QOSOctets(dut) {
		if val, ok := gnmi.Watch(t, dut, qPath.TransmitOctets().State(), 15*time.Second, func(v *ygnmi.Value[uint64]) bool {
			return v.IsPresent()
		}).Await(t); ok {
			snap.txOctets, _ = val.Val()
		}
	}
	if val, ok := gnmi.Watch(t, dut, qPath.DroppedPkts().State(), 15*time.Second, func(v *ygnmi.Value[uint64]) bool {
		return v.IsPresent()
	}).Await(t); ok {
		snap.droppedPkts, _ = val.Val()
	}
	return snap
}

// configureDUTEnvironment sets up interfaces, VRF mgmt, eBGP, static fallback routes, QoS, and the 10-Protocol Suite.
func configureDUTEnvironment(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	dp4 := dut.Port(t, "port4")

	if deviations.ExplicitPortSpeed(dut) {
		for _, p := range []*ondatra.Port{dp1, dp2, dp3, dp4} {
			fptest.SetPortSpeed(t, p)
		}
	}

	// Step: Create non-default L3 VRF "mgmt".
	vrfBatch := &gnmi.SetBatch{}
	mgmtNIP := &cfgplugins.NetworkInstanceParams{Name: mgmtVRFName, Default: false}
	cfgplugins.NewNetworkInstance(t, dut, vrfBatch, mgmtNIP)
	vrfBatch.Set(t, dut)

	// Step: Configure interfaces port1, port2, port3, port4, and lo1.
	portConfigs := []struct {
		name  string
		attr  *attrs.Attributes
		inVRF bool
		isLo  bool
	}{
		{name: dp1.Name(), attr: &dutPort1, inVRF: true},
		{name: dp2.Name(), attr: &dutPort2, inVRF: true},
		{name: dp3.Name(), attr: &dutPort3, inVRF: true},
		{name: dp4.Name(), attr: &dutPort4, inVRF: false},
		{name: env.lo1Name, attr: &dutLo1, inVRF: true, isLo: true},
	}

	for _, pc := range portConfigs {
		intf := pc.attr.NewOCInterface(pc.name, dut)
		if pc.isLo {
			intf.SetType(oc.IETFInterfaces_InterfaceType_softwareLoopback)
		} else {
			intf.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
		}
		intf.SetEnabled(true)
		gnmi.Replace(t, dut, gnmi.OC().Interface(pc.name).Config(), intf)

		if pc.inVRF {
			cfgplugins.AssignToNetworkInstance(t, dut, pc.name, mgmtVRFName, 0)
			if deviations.InterfaceConfigVRFBeforeAddress(dut) {
				gnmi.Replace(t, dut, gnmi.OC().Interface(pc.name).Config(), intf)
			}
		} else if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			cfgplugins.AssignToNetworkInstance(t, dut, pc.name, deviations.DefaultNetworkInstance(dut), 0)
		}
	}

	// Step: Configure routing-policy prefix-sets and table-connections in VRF mgmt for lo1 redistribution.
	routingBatch := &gnmi.SetBatch{}
	cfgplugins.ConfigurePrefixRedistributionPolicy(t, dut, routingBatch, &cfgplugins.PrefixRedistributionCfg{
		PrefixSetName:   prefixSetV4,
		Mode:            oc.PrefixSet_Mode_IPV4,
		Prefix:          lo1IPv4Prefix,
		MaskLengthRange: "exact",
		PolicyName:      redistPolicyV4,
		StatementName:   redistStmtV4,
	})
	cfgplugins.ConfigurePrefixRedistributionPolicy(t, dut, routingBatch, &cfgplugins.PrefixRedistributionCfg{
		PrefixSetName:   prefixSetV6,
		Mode:            oc.PrefixSet_Mode_IPV6,
		Prefix:          lo1IPv6Prefix,
		MaskLengthRange: "exact",
		PolicyName:      redistPolicyV6,
		StatementName:   redistStmtV6,
	})
	if _, err := cfgplugins.ConfigureBGPRoutePolicy(t, routingBatch, cfgplugins.BGPPolicyConfig{
		PolicyName:  bgpAllowPolicy,
		StatementID: bgpAllowStatement,
	}); err != nil {
		t.Fatalf("Failed to configure BGP ALLOW policy: %v", err)
	}
	if !deviations.TableConnectionsUnsupported(dut) {
		cfgplugins.ConfigureTableConnection(t, dut, routingBatch, &cfgplugins.TableConnectionCfg{
			NetworkInstance:          mgmtVRFName,
			SrcProto:                 oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED,
			DstProto:                 oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
			Afi:                      oc.Types_ADDRESS_FAMILY_IPV4,
			ImportPolicies:           []string{redistPolicyV4},
			DefaultImportPolicy:      oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE,
			DisableMetricPropagation: false,
		})
		cfgplugins.ConfigureTableConnection(t, dut, routingBatch, &cfgplugins.TableConnectionCfg{
			NetworkInstance:          mgmtVRFName,
			SrcProto:                 oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED,
			DstProto:                 oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
			Afi:                      oc.Types_ADDRESS_FAMILY_IPV6,
			ImportPolicies:           []string{redistPolicyV6},
			DefaultImportPolicy:      oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE,
			DisableMetricPropagation: false,
		})
	}

	// Step: Configure dual-stack eBGP in VRF mgmt (AS 64512, router-id 192.0.2.101) peering with ATE_PORT1 and ATE_PORT2 (AS 64511).
	bgpProto := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String("BGP"),
		Enabled:    ygot.Bool(true),
	}
	bgp := bgpProto.GetOrCreateBgp()
	bgp.GetOrCreateGlobal().SetAs(dutAS)
	bgp.GetOrCreateGlobal().SetRouterId(lo1IPv4)
	bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
	bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)

	v4Peers := []string{atePort1.IPv4, atePort2.IPv4}
	for _, peerIP := range v4Peers {
		nbr := bgp.GetOrCreateNeighbor(peerIP)
		nbr.SetNeighborAddress(peerIP)
		nbr.SetPeerAs(ateAS)
		nbr.SetEnabled(true)
		af4 := nbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.SetEnabled(true)
		ap := af4.GetOrCreateApplyPolicy()
		ap.SetImportPolicy([]string{bgpAllowPolicy})
		ap.SetExportPolicy([]string{redistPolicyV4})
	}

	v6Peers := []string{atePort1.IPv6, atePort2.IPv6}
	for _, peerIP := range v6Peers {
		nbr := bgp.GetOrCreateNeighbor(peerIP)
		nbr.SetNeighborAddress(peerIP)
		nbr.SetPeerAs(ateAS)
		nbr.SetEnabled(true)
		af6 := nbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.SetEnabled(true)
		ap := af6.GetOrCreateApplyPolicy()
		ap.SetImportPolicy([]string{bgpAllowPolicy})
		ap.SetExportPolicy([]string{redistPolicyV6})
	}
	gnmi.BatchReplace(routingBatch, gnmi.OC().NetworkInstance(mgmtVRFName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgpProto)

	// Step: Configure backup static default routes in VRF mgmt over port3 with preference = 220.
	if _, err := cfgplugins.NewStaticRouteCfg(routingBatch, &cfgplugins.StaticRouteCfg{
		NetworkInstance: mgmtVRFName,
		Prefix:          defaultIPv4Route,
		Preference:      backupPreference,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			staticRouteIdx: oc.UnionString(atePort3.IPv4),
		},
	}, dut); err != nil {
		t.Fatalf("Failed to configure backup IPv4 static default route: %v", err)
	}
	if _, err := cfgplugins.NewStaticRouteCfg(routingBatch, &cfgplugins.StaticRouteCfg{
		NetworkInstance: mgmtVRFName,
		Prefix:          defaultIPv6Route,
		Preference:      backupPreference,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			staticRouteIdx: oc.UnionString(atePort3.IPv6),
		},
	}, dut); err != nil {
		t.Fatalf("Failed to configure backup IPv6 static default route: %v", err)
	}
	routingBatch.Set(t, dut)

	// Step: Configure QoS queues (AF4, BE1), forwarding-groups, classifiers, and scheduler policy (mgmt_qos_sched) on port1-port3.
	configureDUTQoS(t, env)

	// Step: Configure the 10-Protocol Suite on the DUT and verify all protocol operational states.
	configureDUTProtocols(t, env)
}

// configureDUTQoS configures queues AF4 and BE1, forwarding-groups, DSCP 32 classifiers, and egress scheduler policy mgmt_qos_sched.
func configureDUTQoS(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	queues := netutil.CommonTrafficQueues(t, dut)
	env.af4Queue = queues.AF4
	env.be1Queue = queues.BE1

	root := &oc.Root{}
	q := root.GetOrCreateQos()

	cfgplugins.CreateQueues(t, dut, q, []string{queues.AF4, queues.BE1})

	cfgplugins.NewQoSForwardingGroup(t, dut, q, []cfgplugins.ForwardingGroup{
		{
			Desc:        "forwarding-group-AF4",
			TargetGroup: "AF4",
			QueueName:   queues.AF4,
		},
		{
			Desc:        "forwarding-group-BE1",
			TargetGroup: "BE1",
			QueueName:   queues.BE1,
		},
	})

	cfgplugins.NewQoSClassifierConfiguration(t, dut, q, []cfgplugins.QosClassifier{
		{
			Desc:        "IPv4 classifier matching DSCP 32 -> AF4",
			Name:        classifierV4Name,
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "0",
			TargetGroup: "AF4",
			DscpSet:     []uint8{dscpAF4},
		},
		{
			Desc:        "IPv6 classifier matching DSCP 32 -> AF4",
			Name:        classifierV6Name,
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "0",
			TargetGroup: "AF4",
			DscpSet:     []uint8{dscpAF4},
		},
	})

	cfgplugins.NewQoSSchedulerPolicyWithName(t, dut, q, qosSchedulerName, []cfgplugins.SchedulerPolicy{
		{
			Desc:          "Strict priority AF4 scheduler",
			Sequence:      1,
			SetPriority:   true,
			Priority:      oc.Scheduler_Priority_STRICT,
			SchedulerType: oc.QosTypes_QOS_SCHEDULER_TYPE_TWO_RATE_THREE_COLOR,
			InputID:       "AF4",
			InputType:     oc.Input_InputType_QUEUE,
			QueueName:     queues.AF4,
			SetWeight:     true,
			Weight:        100,
			TargetGroup:   "AF4",
		},
		{
			Desc:          "Best-effort BE1 scheduler",
			Sequence:      2,
			SetPriority:   false,
			SchedulerType: oc.QosTypes_QOS_SCHEDULER_TYPE_TWO_RATE_THREE_COLOR,
			InputID:       "BE1",
			InputType:     oc.Input_InputType_QUEUE,
			QueueName:     queues.BE1,
			SetWeight:     true,
			Weight:        1,
			TargetGroup:   "BE1",
		},
	})

	schedIntfs := []cfgplugins.QoSSchedulerInterface{
		{Desc: "output-AF4", QueueName: queues.AF4, Scheduler: qosSchedulerName},
		{Desc: "output-BE1", QueueName: queues.BE1, Scheduler: qosSchedulerName},
	}
	for _, portID := range []string{"port1", "port2", "port3"} {
		cfgplugins.NewQoSSchedulerInterface(t, dut, q, schedIntfs, portID)
	}

	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)

	// Verify QoS scheduler-policy and interface output state paths.
	spState := gnmi.OC().Qos().SchedulerPolicy(qosSchedulerName)
	gnmi.Await(t, dut, spState.Name().State(), 30*time.Second, qosSchedulerName)
	for _, seq := range []uint32{1, 2} {
		schedState := spState.Scheduler(seq)
		gnmi.Await(t, dut, schedState.Sequence().State(), 30*time.Second, seq)
		gnmi.Lookup(t, dut, schedState.Priority().State())
		gnmi.Lookup(t, dut, schedState.Type().State())
	}
	for _, inp := range []struct {
		seq   uint32
		id    string
		queue string
	}{
		{seq: 1, id: "AF4", queue: queues.AF4},
		{seq: 2, id: "BE1", queue: queues.BE1},
	} {
		inState := spState.Scheduler(inp.seq).Input(inp.id)
		gnmi.Lookup(t, dut, inState.Id().State())
		gnmi.Lookup(t, dut, inState.InputType().State())
		gnmi.Lookup(t, dut, inState.Queue().State())
		gnmi.Lookup(t, dut, inState.Weight().State())
	}
	for _, portID := range []string{"port1", "port2", "port3"} {
		pName := dut.Port(t, portID).Name()
		outState := gnmi.OC().Qos().Interface(pName).Output()
		gnmi.Lookup(t, dut, outState.SchedulerPolicy().Name().State())
		for _, qName := range []string{queues.AF4, queues.BE1} {
			gnmi.Lookup(t, dut, outState.Queue(qName).Name().State())
		}
	}
}

// configureDUTProtocols configures NTP, TACACS+, SSH, DNS, sFlow, Syslog, and gRPC servers in VRF mgmt and verifies their state paths.
func configureDUTProtocols(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	dp4 := dut.Port(t, "port4")
	sysBatch := &gnmi.SetBatch{}

	// 1. NTP (UDP 123)
	cfgplugins.ConfigureNTPServers(t, dut, sysBatch, true, []cfgplugins.NTPServerCfg{
		{Address: collectorIPv4, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv4},
		{Address: collectorIPv6, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv6},
	})

	// 2. TACACS+ (TCP 49)
	cfgplugins.ConfigureTACACS(t, sysBatch, &cfgplugins.TACACSGroupCfg{
		GroupName: tacacsGroupName,
		Servers: []cfgplugins.TACACSServerCfg{
			{Address: collectorIPv4, Timeout: 5, Port: portTACACS, SecretKey: tacacsSecret, SourceAddress: lo1IPv4},
			{Address: collectorIPv6, Timeout: 5, Port: portTACACS, SecretKey: tacacsSecret, SourceAddress: lo1IPv6},
		},
	})

	// 3. SSH Server (TCP 22)
	cfgplugins.ConfigureSSHServer(t, sysBatch, true)

	// 4. DNS Client (UDP/TCP 53)
	cfgplugins.ConfigureDNSServers(t, sysBatch, []cfgplugins.DNSServerCfg{
		{Address: collectorIPv4, Port: portDNS},
		{Address: collectorIPv6, Port: portDNS},
	})

	// 5. sFlow (UDP 6343)
	cfgplugins.ConfigureSFlow(t, dut, sysBatch, &cfgplugins.SFlowCfg{
		Enabled:             true,
		Dscp:                dscpAF4,
		IngressSamplingRate: 1,
		SampleSize:          256,
		Interfaces:          []string{dp4.Name()},
		Collectors: []cfgplugins.SFlowCollectorCfg{
			{Address: collectorIPv4, Port: portSFlow, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv4},
			{Address: collectorIPv6, Port: portSFlow, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv6},
		},
	})

	// 6. Remote Syslog (UDP 514)
	cfgplugins.ConfigureRemoteSyslogServers(t, sysBatch, []cfgplugins.SyslogServerCfg{
		{
			Host:            collectorIPv4,
			NetworkInstance: mgmtVRFName,
			RemotePort:      portSyslog,
			SourceAddress:   lo1IPv4,
			Facility:        oc.SystemLogging_SYSLOG_FACILITY_ALL,
			Severity:        oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
		},
		{
			Host:            collectorIPv6,
			NetworkInstance: mgmtVRFName,
			RemotePort:      portSyslog,
			SourceAddress:   lo1IPv6,
			Facility:        oc.SystemLogging_SYSLOG_FACILITY_ALL,
			Severity:        oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
		},
	})

	// 7-10. gRPC Server (gNMI/gNOI/gRIBI/P4RT) in VRF mgmt listening on lo1.
	cfgplugins.ConfigureGRPCServer(t, dut, sysBatch, &cfgplugins.GRPCServerCfg{
		Name:            grpcServerName,
		Enable:          true,
		Port:            portGNMI,
		NetworkInstance: mgmtVRFName,
		ListenAddresses: []string{lo1IPv4, lo1IPv6},
		Services:        []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
	})

	sysBatch.Set(t, dut)

	// Verify state paths for all configured system and sampling services.
	verifyProtocolStates(t, dut)
}

// verifyProtocolStates verifies the OpenConfig operational state paths for NTP, AAA/TACACS+, SSH, DNS, sFlow, Syslog, and gRPC servers.
func verifyProtocolStates(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	// NTP state verification.
	gnmi.Await(t, dut, gnmi.OC().System().Ntp().Enabled().State(), 30*time.Second, true)
	for _, srv := range []struct {
		addr string
		src  string
	}{
		{addr: collectorIPv4, src: lo1IPv4},
		{addr: collectorIPv6, src: lo1IPv6},
	} {
		ntpSrv := gnmi.OC().System().Ntp().Server(srv.addr)
		gnmi.Await(t, dut, ntpSrv.Address().State(), 30*time.Second, srv.addr)
		gnmi.Lookup(t, dut, ntpSrv.NetworkInstance().State())
		if !deviations.NtpSourceAddressUnsupported(dut) {
			gnmi.Lookup(t, dut, ntpSrv.SourceAddress().State())
		}
	}

	// AAA / TACACS+ state verification.
	gnmi.Lookup(t, dut, gnmi.OC().System().Aaa().Authentication().AuthenticationMethod().State())
	for _, srv := range []struct {
		addr string
		src  string
	}{
		{addr: collectorIPv4, src: lo1IPv4},
		{addr: collectorIPv6, src: lo1IPv6},
	} {
		tacSrv := gnmi.OC().System().Aaa().ServerGroup(tacacsGroupName).Server(srv.addr)
		gnmi.Lookup(t, dut, tacSrv.Address().State())
		gnmi.Lookup(t, dut, tacSrv.Tacacs().Port().State())
		gnmi.Lookup(t, dut, tacSrv.Tacacs().SourceAddress().State())
	}

	// SSH server state verification.
	gnmi.Await(t, dut, gnmi.OC().System().SshServer().Enable().State(), 30*time.Second, true)

	// DNS server state verification.
	if dnsState, ok := gnmi.Lookup(t, dut, gnmi.OC().System().Dns().State()).Val(); ok && dnsState != nil {
		for _, addr := range []string{collectorIPv4, collectorIPv6} {
			if srv := dnsState.GetServer(addr); srv != nil {
				_ = srv.GetAddress()
				_ = srv.GetPort()
			}
		}
	}

	// sFlow state verification.
	gnmi.Await(t, dut, gnmi.OC().Sampling().Sflow().Enabled().State(), 30*time.Second, true)
	gnmi.Await(t, dut, gnmi.OC().Sampling().Sflow().Dscp().State(), 30*time.Second, uint8(dscpAF4))
	for _, addr := range []string{collectorIPv4, collectorIPv6} {
		col := gnmi.OC().Sampling().Sflow().Collector(addr, portSFlow)
		gnmi.Lookup(t, dut, col.Address().State())
		gnmi.Lookup(t, dut, col.NetworkInstance().State())
		gnmi.Lookup(t, dut, col.Port().State())
		gnmi.Lookup(t, dut, col.SourceAddress().State())
	}

	// Remote Syslog state verification.
	for _, addr := range []string{collectorIPv4, collectorIPv6} {
		rs := gnmi.OC().System().Logging().RemoteServer(addr)
		gnmi.Lookup(t, dut, rs.Host().State())
		gnmi.Lookup(t, dut, rs.NetworkInstance().State())
		gnmi.Lookup(t, dut, rs.RemotePort().State())
		gnmi.Lookup(t, dut, rs.SourceAddress().State())
	}

	// gRPC server state verification.
	grpcSrv := gnmi.OC().System().GrpcServer(grpcServerName)
	gnmi.Lookup(t, dut, grpcSrv.Name().State())
	gnmi.Lookup(t, dut, grpcSrv.Enable().State())
	gnmi.Lookup(t, dut, grpcSrv.NetworkInstance().State())
	gnmi.Lookup(t, dut, grpcSrv.Port().State())
	gnmi.Lookup(t, dut, grpcSrv.ListenAddresses().State())
	if !deviations.GrpcServerServicesUnsupported(dut) {
		gnmi.Lookup(t, dut, grpcSrv.Services().State())
	}
}

// configureOTGEnvironment configures ATE ports port1-port4, eBGP peers on port1 and port2, and collector routes.
func configureOTGEnvironment(t *testing.T, env *testEnv) {
	t.Helper()
	ate := env.ate
	top := gosnappi.NewConfig()
	env.top = top

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")
	ap4 := ate.Port(t, "port4")

	atePort1.AddToOTG(top, ap1, &dutPort1)
	atePort2.AddToOTG(top, ap2, &dutPort2)
	atePort3.AddToOTG(top, ap3, &dutPort3)
	atePort4.AddToOTG(top, ap4, &dutPort4)

	// Configure eBGP on ATE_PORT1 and ATE_PORT2 advertising 0.0.0.0/0, ::/0, 198.51.100.0/24, and 2001:db8:100::/64.
	for idx, bgpPeer := range []struct {
		ateAttr attrs.Attributes
		dutAttr attrs.Attributes
	}{
		{ateAttr: atePort1, dutAttr: dutPort1},
		{ateAttr: atePort2, dutAttr: dutPort2},
	} {
		var dev gosnappi.Device
		for _, d := range top.Devices().Items() {
			if d.Name() == bgpPeer.ateAttr.Name {
				dev = d
				break
			}
		}
		bgp := dev.Bgp().SetRouterId(bgpPeer.ateAttr.IPv4)
		eth := dev.Ethernets().Items()[0]
		ipv4 := eth.Ipv4Addresses().Items()[0]
		ipv6 := eth.Ipv6Addresses().Items()[0]

		bgp4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name())
		peer4 := bgp4.Peers().Add().
			SetName(fmt.Sprintf("%s.bgp4.peer", bgpPeer.ateAttr.Name)).
			SetPeerAddress(bgpPeer.dutAttr.IPv4).
			SetAsNumber(uint32(ateAS)).
			SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

		rr4 := peer4.V4Routes().Add().
			SetName(fmt.Sprintf("%s.bgp4.routes", bgpPeer.ateAttr.Name)).
			SetNextHopIpv4Address(bgpPeer.ateAttr.IPv4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		rr4.AddPath().SetPathId(uint32(idx + 1))
		rr4.Addresses().Add().SetAddress(defaultIPv4Net).SetPrefix(0).SetCount(1)
		rr4.Addresses().Add().SetAddress(collectorIPv4Net).SetPrefix(collectorIPv4Len).SetCount(1)

		bgp6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name())
		peer6 := bgp6.Peers().Add().
			SetName(fmt.Sprintf("%s.bgp6.peer", bgpPeer.ateAttr.Name)).
			SetPeerAddress(bgpPeer.dutAttr.IPv6).
			SetAsNumber(uint32(ateAS)).
			SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

		rr6 := peer6.V6Routes().Add().
			SetName(fmt.Sprintf("%s.bgp6.routes", bgpPeer.ateAttr.Name)).
			SetNextHopIpv6Address(bgpPeer.ateAttr.IPv6).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		rr6.AddPath().SetPathId(uint32(idx + 1))
		rr6.Addresses().Add().SetAddress(defaultIPv6Net).SetPrefix(0).SetCount(1)
		rr6.Addresses().Add().SetAddress(collectorIPv6Net).SetPrefix(collectorIPv6Len).SetCount(1)
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
}

// awaitBGPSessionState waits for the given BGP neighbor addresses in VRF mgmt to reach the target session state.
func awaitBGPSessionState(t *testing.T, dut *ondatra.DUTDevice, neighbors []string, wantState oc.E_Bgp_Neighbor_SessionState, timeout time.Duration) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(mgmtVRFName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range neighbors {
		_, ok := gnmi.Watch(t, dut, bgpPath.Neighbor(nbr).SessionState().State(), timeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			got, present := val.Val()
			if wantState == oc.Bgp_Neighbor_SessionState_ESTABLISHED {
				return present && got == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}
			return !present || got != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			t.Errorf("BGP neighbor %s in VRF %s did not reach target state %v within %v", nbr, mgmtVRFName, wantState, timeout)
		}
	}
}

// setDUTPortsEnabled sets the administrative enabled state on the specified DUT port IDs and waits for operational status.
func setDUTPortsEnabled(t *testing.T, dut *ondatra.DUTDevice, portIDs []string, enabled bool) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	for _, pid := range portIDs {
		pName := dut.Port(t, pid).Name()
		gnmi.BatchReplace(batch, gnmi.OC().Interface(pName).Enabled().Config(), enabled)
	}
	batch.Set(t, dut)

	wantOper := oc.Interface_OperStatus_UP
	if !enabled {
		wantOper = oc.Interface_OperStatus_DOWN
	}
	for _, pid := range portIDs {
		pName := dut.Port(t, pid).Name()
		gnmi.Watch(t, dut, gnmi.OC().Interface(pName).OperStatus().State(), 30*time.Second, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			got, present := val.Val()
			return present && got == wantOper
		}).Await(t)
	}
}

// startPortCaptures configures PCAP capture on the specified ATE port IDs and starts capture state.
func startPortCaptures(t *testing.T, env *testEnv, portIDs []string) func() {
	t.Helper()
	env.top.Captures().Clear()
	for _, pid := range portIDs {
		env.top.Captures().Add().
			SetName("cap_" + pid).
			SetPortNames([]string{pid}).
			SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	env.ate.OTG().PushConfig(t, env.top)
	env.ate.OTG().StartProtocols(t)

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START).SetPortNames(portIDs)
	env.ate.OTG().SetControlState(t, cs)

	stopped := false
	stopFn := func() {
		if stopped {
			return
		}
		stopped = true
		stopCS := gosnappi.NewControlState()
		stopCS.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP).SetPortNames(portIDs)
		env.ate.OTG().SetControlState(t, stopCS)
	}
	t.Cleanup(stopFn)
	return stopFn
}

// triggerAllProtocols executes the 10-Protocol Trigger Suite (IPv4 & IPv6) as defined in the README.
func triggerAllProtocols(t *testing.T, env *testEnv, activeATEPorts []string, clientDSCPValues []uint32) {
	t.Helper()
	dut := env.dut
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Trigger gNMI (TCP 9339): Execute gNMI.Get and gNMI.Subscribe RPCs.
	rawGNMI := dut.RawAPIs().GNMI(t)
	if _, err := rawGNMI.Get(ctx, &gpb.GetRequest{
		Path: []*gpb.Path{{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}}},
		Type: gpb.GetRequest_CONFIG,
	}); err != nil {
		t.Logf("gNMI.Get trigger completed with status: %v", err)
	}
	gnmi.Watch(t, dut, gnmi.OC().System().Ntp().Enabled().State(), 5*time.Second, func(v *ygnmi.Value[bool]) bool {
		return v.IsPresent()
	}).Await(t)

	// 2. Trigger gNOI (TCP 9339) & 4. Trigger DNS Client (UDP/TCP 53):
	// Execute gNOI.System.Time and gNOI.System.Ping (including unresolved hostname mgmt-target.example.com).
	gnoiSys := dut.RawAPIs().GNOI(t).System()
	if _, err := gnoiSys.Time(ctx, &spb.TimeRequest{}); err != nil {
		t.Logf("gNOI.System.Time trigger completed with status: %v", err)
	}
	for _, dst := range []string{unresolvedHost, collectorIPv4, collectorIPv6} {
		pingStream, err := gnoiSys.Ping(ctx, &spb.PingRequest{
			Destination: dst,
			Count:       1,
		})
		if err == nil && pingStream != nil {
			_, _ = pingStream.Recv()
		}
	}

	// 3. Trigger gRIBI (TCP 9340): Execute gRIBI.Get and gRIBI.Modify RPCs.
	gribiClient := dut.RawAPIs().GRIBI(t)
	if getStream, err := gribiClient.Get(ctx, &gribipb.GetRequest{
		NetworkInstance: &gribipb.GetRequest_Name{Name: mgmtVRFName},
		Aft:             gribipb.AFTType_IPV4,
	}); err == nil && getStream != nil {
		_, _ = getStream.Recv()
	}
	if modStream, err := gribiClient.Modify(ctx); err == nil && modStream != nil {
		_ = modStream.Send(&gribipb.ModifyRequest{
			Params: &gribipb.SessionParameters{
				Redundancy:  gribipb.SessionParameters_SINGLE_PRIMARY,
				Persistence: gribipb.SessionParameters_PRESERVE,
				AckType:     gribipb.SessionParameters_RIB_AND_FIB_ACK,
			},
		})
		_ = modStream.CloseSend()
	}

	// 4. Trigger P4RT (TCP 9559): Establish P4Runtime.StreamChannel session and send MasterArbitrationUpdate.
	p4rtClient := dut.RawAPIs().P4RT(t)
	if p4Stream, err := p4rtClient.StreamChannel(ctx); err == nil && p4Stream != nil {
		_ = p4Stream.Send(&p4v1pb.StreamMessageRequest{
			Update: &p4v1pb.StreamMessageRequest_Arbitration{
				Arbitration: &p4v1pb.MasterArbitrationUpdate{
					DeviceId:   1,
					ElectionId: &p4v1pb.Uint128{High: 0, Low: 1},
				},
			},
		})
		_ = p4Stream.CloseSend()
	}

	// 5. Trigger NTP (UDP 123) & 6. Trigger Syslog (UDP 514) via gNMI.Set configuration event.
	triggerBatch := &gnmi.SetBatch{}
	cfgplugins.ConfigureNTPServers(t, dut, triggerBatch, true, []cfgplugins.NTPServerCfg{
		{Address: collectorIPv4, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv4},
		{Address: collectorIPv6, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv6},
	})
	gnmi.BatchReplace(triggerBatch, gnmi.OC().Interface(env.lo1Name).Description().Config(), fmt.Sprintf("mgmt-lo1-trigger-%d", time.Now().UnixNano()))
	triggerBatch.Set(t, dut)

	// 7. Trigger sFlow (UDP 6343) via transit traffic on ATE_PORT4 and client-initiated TCP handshakes (SSH 22, TACACS+ 49, gNMI 9339, gRIBI 9340, P4RT 9559).
	configureAndRunProtocolFlows(t, env, activeATEPorts, clientDSCPValues, true)
}

// activeOTGDevicesConfig returns a gosnappi.Config containing only the OTG devices on active (UP) ports so WaitForARP does not wait on administratively disabled failover ports.
func activeOTGDevicesConfig(top gosnappi.Config, activeATEPorts []string) gosnappi.Config {
	activeDevNames := map[string]bool{
		atePort4.Name: true,
	}
	for _, pid := range activeATEPorts {
		switch pid {
		case "port1":
			activeDevNames[atePort1.Name] = true
		case "port2":
			activeDevNames[atePort2.Name] = true
		case "port3":
			activeDevNames[atePort3.Name] = true
		}
	}
	cfg := gosnappi.NewConfig()
	for _, d := range top.Devices().Items() {
		if activeDevNames[d.Name()] {
			cfg.Devices().Append(d)
		}
	}
	return cfg
}

// configureAndRunProtocolFlows sends transit packets on port4 (triggering sFlow) and inbound client probes on active ATE management ports.
func configureAndRunProtocolFlows(t *testing.T, env *testEnv, activeATEPorts []string, clientDSCPValues []uint32, clearExistingFlows bool) {
	t.Helper()
	if clearExistingFlows {
		env.top.Flows().Clear()
	}

	if len(clientDSCPValues) == 0 {
		clientDSCPValues = []uint32{dscpBE1}
	}

	// Transit flow on ATE_PORT4 -> DUT_PORT4 to trigger sFlow packet sampling.
	p4Name := env.dut.Port(t, "port4").Name()
	dstMAC4, ok := gnmi.Lookup(t, env.dut, gnmi.OC().Interface(p4Name).Ethernet().MacAddress().State()).Val()
	if !ok {
		t.Fatalf("Failed to lookup MAC address for %s (port4)", p4Name)
	}
	sfFlowV4 := env.top.Flows().Add().SetName("sflow_transit_v4")
	sfFlowV4.Metrics().SetEnable(true)
	sfFlowV4.TxRx().Port().SetTxName("port4").SetRxNames([]string{"port4"})
	sfFlowV4.Size().SetFixed(256)
	sfFlowV4.Rate().SetPps(100)
	sfFlowV4.Duration().FixedPackets().SetPackets(50)
	eth4 := sfFlowV4.Packet().Add().Ethernet()
	eth4.Src().SetValue(atePort4.MAC)
	eth4.Dst().SetValue(dstMAC4)
	ip4 := sfFlowV4.Packet().Add().Ipv4()
	ip4.Src().SetValue(atePort4.IPv4)
	ip4.Dst().SetValue(dutPort4.IPv4)

	sfFlowV6 := env.top.Flows().Add().SetName("sflow_transit_v6")
	sfFlowV6.Metrics().SetEnable(true)
	sfFlowV6.TxRx().Port().SetTxName("port4").SetRxNames([]string{"port4"})
	sfFlowV6.Size().SetFixed(256)
	sfFlowV6.Rate().SetPps(100)
	sfFlowV6.Duration().FixedPackets().SetPackets(50)
	eth6 := sfFlowV6.Packet().Add().Ethernet()
	eth6.Src().SetValue(atePort4.MAC)
	eth6.Dst().SetValue(dstMAC4)
	ip6 := sfFlowV6.Packet().Add().Ipv6()
	ip6.Src().SetValue(atePort4.IPv6)
	ip6.Dst().SetValue(dutPort4.IPv6)

	// Inbound client TCP SYN probes from collectorIPv4/collectorIPv6 to lo1 for SSH (22), gNMI/gNOI (9339), gRIBI (9340), and P4RT (9559).
	clientPorts := []uint32{portSSH, portGNMI, portGRIBI, portP4RT}
	for _, atePortID := range activeATEPorts {
		dutPortName := env.dut.Port(t, atePortID).Name()
		dstMAC, ok := gnmi.Lookup(t, env.dut, gnmi.OC().Interface(dutPortName).Ethernet().MacAddress().State()).Val()
		if !ok {
			t.Fatalf("Failed to lookup MAC address for %s (%s)", dutPortName, atePortID)
		}
		srcMAC := atePort1.MAC
		switch atePortID {
		case "port2":
			srcMAC = atePort2.MAC
		case "port3":
			srcMAC = atePort3.MAC
		}

		for _, dscpVal := range clientDSCPValues {
			// IPv4 client probe flow.
			flowV4Name := fmt.Sprintf("client_v4_%s_dscp%d", atePortID, dscpVal)
			f4 := env.top.Flows().Add().SetName(flowV4Name)
			f4.Metrics().SetEnable(true)
			f4.TxRx().Port().SetTxName(atePortID).SetRxNames([]string{atePortID})
			f4.Size().SetFixed(128)
			f4.Rate().SetPps(50)
			f4.Duration().FixedPackets().SetPackets(20)
			eth4 := f4.Packet().Add().Ethernet()
			eth4.Src().SetValue(srcMAC)
			eth4.Dst().SetValue(dstMAC)
			ip4 := f4.Packet().Add().Ipv4()
			ip4.Src().SetValue(collectorIPv4)
			ip4.Dst().SetValue(lo1IPv4)
			ip4.Priority().Dscp().Phb().SetValue(dscpVal)
			tcp4 := f4.Packet().Add().Tcp()
			tcp4.SrcPort().SetValue(50000 + dscpVal)
			tcp4.DstPort().SetValues(clientPorts)
			tcp4.CtlSyn().SetValue(1)

			// IPv6 client probe flow.
			flowV6Name := fmt.Sprintf("client_v6_%s_dscp%d", atePortID, dscpVal)
			f6 := env.top.Flows().Add().SetName(flowV6Name)
			f6.Metrics().SetEnable(true)
			f6.TxRx().Port().SetTxName(atePortID).SetRxNames([]string{atePortID})
			f6.Size().SetFixed(128)
			f6.Rate().SetPps(50)
			f6.Duration().FixedPackets().SetPackets(20)
			eth6 := f6.Packet().Add().Ethernet()
			eth6.Src().SetValue(srcMAC)
			eth6.Dst().SetValue(dstMAC)
			ip6 := f6.Packet().Add().Ipv6()
			ip6.Src().SetValue(collectorIPv6)
			ip6.Dst().SetValue(lo1IPv6)
			ip6.TrafficClass().SetValue(dscpVal << 2)
			tcp6 := f6.Packet().Add().Tcp()
			tcp6.SrcPort().SetValue(51000 + dscpVal)
			tcp6.DstPort().SetValues(clientPorts)
			tcp6.CtlSyn().SetValue(1)
		}
	}

	env.ate.OTG().PushConfig(t, env.top)
	env.ate.OTG().StartProtocols(t)
	config := activeOTGDevicesConfig(env.top, activeATEPorts)
	otgutils.WaitForARP(t, env.ate.OTG(), config, "IPv4")
	otgutils.WaitForARP(t, env.ate.OTG(), config, "IPv6")
	env.ate.OTG().StartTraffic(t)
	// Use gnmi.Watch on OTG flow counters instead of time.Sleep.
	for _, f := range env.top.Flows().Items() {
		gnmi.Watch(t, env.ate.OTG(), gnmi.OTG().Flow(f.Name()).Counters().OutPkts().State(), 10*time.Second, func(v *ygnmi.Value[uint64]) bool {
			pkts, ok := v.Val()
			return ok && pkts > 0
		}).Await(t)
	}
	env.ate.OTG().StopTraffic(t)
}

// pcapSummary holds packet counts inspected from an ATE port PCAP capture.
type pcapSummary struct {
	dutOriginatedV4Pkts int
	dutOriginatedV6Pkts int
	af4Pkts             int
	be1Pkts             int
	cs1Pkts             int
	otherDSCPPkts       int
}

// inspectPortPCAP reads the PCAP from the specified ATE port ID and classifies all DUT-originated management packets by DSCP.
func inspectPortPCAP(t *testing.T, ate *ondatra.ATEDevice, portID string) pcapSummary {
	t.Helper()
	var summary pcapSummary
	captureBytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(portID))
	if len(captureBytes) == 0 {
		t.Logf("No capture bytes returned on ATE %s", portID)
		return summary
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("pcap_%s_*.pcap", portID))
	if err != nil {
		t.Fatalf("Failed to create temporary pcap file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(captureBytes); err != nil {
		t.Fatalf("Failed to write capture bytes to %s: %v", tmpFile.Name(), err)
	}
	_ = tmpFile.Close()

	handle, err := pcap.OpenOffline(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open pcap file %s: %v", tmpFile.Name(), err)
	}
	defer handle.Close()

	dutLo1V4IP := net.ParseIP(lo1IPv4)
	dutLo1V6IP := net.ParseIP(lo1IPv6)
	collectorV4IP := net.ParseIP(collectorIPv4)
	collectorV6IP := net.ParseIP(collectorIPv6)

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if ip4Layer := packet.Layer(layers.LayerTypeIPv4); ip4Layer != nil {
			ip4 := ip4Layer.(*layers.IPv4)
			if ip4.SrcIP.Equal(dutLo1V4IP) && ip4.DstIP.Equal(collectorV4IP) {
				summary.dutOriginatedV4Pkts++
				dscp := ip4.TOS >> 2
				switch dscp {
				case dscpAF4:
					summary.af4Pkts++
				case dscpBE1:
					summary.be1Pkts++
				case dscpCS1:
					summary.cs1Pkts++
				default:
					summary.otherDSCPPkts++
				}
			}
		}
		if ip6Layer := packet.Layer(layers.LayerTypeIPv6); ip6Layer != nil {
			ip6 := ip6Layer.(*layers.IPv6)
			if ip6.SrcIP.Equal(dutLo1V6IP) && ip6.DstIP.Equal(collectorV6IP) {
				summary.dutOriginatedV6Pkts++
				dscp := ip6.TrafficClass >> 2
				switch dscp {
				case dscpAF4:
					summary.af4Pkts++
				case dscpBE1:
					summary.be1Pkts++
				case dscpCS1:
					summary.cs1Pkts++
				default:
					summary.otherDSCPPkts++
				}
			}
		}
	}
	t.Logf("PCAP summary on ATE %s: V4=%d, V6=%d, AF4(DSCP 32)=%d, BE1(DSCP 0)=%d, CS1(DSCP 8)=%d, Other=%d",
		portID, summary.dutOriginatedV4Pkts, summary.dutOriginatedV6Pkts, summary.af4Pkts, summary.be1Pkts, summary.cs1Pkts, summary.otherDSCPPkts)
	return summary
}

// verifyStandardEgressAF4 performs the Standard Egress AF4 Verification defined in the README across the active egress port(s).
func verifyStandardEgressAF4(t *testing.T, env *testEnv, activePortIDs []string, baselineAF4 map[string]queueCounters) {
	t.Helper()
	var totalAF4CapPkts, totalBE1CapPkts int
	var totalAF4TxDelta uint64

	for _, pid := range activePortIDs {
		pcapSum := inspectPortPCAP(t, env.ate, pid)
		totalAF4CapPkts += pcapSum.af4Pkts
		totalBE1CapPkts += pcapSum.be1Pkts
		if pcapSum.be1Pkts > 0 {
			t.Errorf("Standard Egress AF4 Verification failed on ATE %s: captured %d DUT-originated packets with DSCP 0 (BE1), want 0", pid, pcapSum.be1Pkts)
		}
		if pcapSum.otherDSCPPkts > 0 || pcapSum.cs1Pkts > 0 {
			t.Errorf("Standard Egress AF4 Verification failed on ATE %s: captured %d non-AF4 packets (CS1=%d, Other=%d), want 0", pid, pcapSum.cs1Pkts+pcapSum.otherDSCPPkts, pcapSum.cs1Pkts, pcapSum.otherDSCPPkts)
		}

		dutPortName := env.dut.Port(t, pid).Name()
		qName := resolveQueueName(env.dut, dutPortName, env.af4Queue, 4)
		after := snapshotQueueCounters(t, env.dut, dutPortName, qName)
		before := baselineAF4[pid]
		if after.txPkts >= before.txPkts {
			totalAF4TxDelta += after.txPkts - before.txPkts
		}
		if after.droppedPkts > before.droppedPkts {
			t.Errorf("Queue AF4 (%s) on %s dropped packets: before=%d, after=%d, want 0 drops",
				qName, dutPortName, before.droppedPkts, after.droppedPkts)
		}
	}

	if totalAF4CapPkts == 0 && totalAF4TxDelta == 0 {
		t.Errorf("Standard Egress AF4 Verification across %v: expected AF4 packet capture (>0) and AF4 queue transmit-pkts delta (>0), got cap=%d, queueDelta=%d",
			activePortIDs, totalAF4CapPkts, totalAF4TxDelta)
	}
}

// TestMVCControlTrafficQoS executes MGT-2 (subtests MGT-2.1 through MGT-2.7).
func TestMVCControlTrafficQoS(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	env := &testEnv{
		dut:     dut,
		ate:     ate,
		lo1Name: netutil.LoopbackInterface(t, dut, 1),
	}

	// Register top-level cleanup as required by the README Cleanup section.
	t.Cleanup(func() {
		setDUTPortsEnabled(t, dut, []string{"port1", "port2", "port3", "port4"}, true)
		ate.OTG().StopProtocols(t)
	})

	// Configure test environment setup on DUT and ATE.
	configureDUTEnvironment(t, env)
	configureOTGEnvironment(t, env)

	// =========================================================================
	// MGT-2.1: QoS Marking over Linecard Interfaces
	// =========================================================================
	t.Run("MGT-2.1_QoS_Marking_over_Linecard_Interfaces", func(t *testing.T) {
		// MGT-2.1 Step 1: Verify VRF mgmt IPv4/IPv6 eBGP sessions on <DUT_PORT1> and <DUT_PORT2> are ESTABLISHED
		// and record baseline AF4 queue counters.
		awaitBGPSessionState(t, dut, []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}, oc.Bgp_Neighbor_SessionState_ESTABLISHED, 2*time.Minute)

		baselineAF4 := map[string]queueCounters{
			"port1": snapshotQueueCounters(t, dut, dut.Port(t, "port1").Name(), resolveQueueName(dut, dut.Port(t, "port1").Name(), env.af4Queue, 4)),
			"port2": snapshotQueueCounters(t, dut, dut.Port(t, "port2").Name(), resolveQueueName(dut, dut.Port(t, "port2").Name(), env.af4Queue, 4)),
		}

		// MGT-2.1 Step 2: Start packet capture on <ATE_PORT1> and <ATE_PORT2>, run the 10-Protocol Trigger Suite, then stop packet capture.
		stopCap := startPortCaptures(t, env, []string{"port1", "port2"})
		defer stopCap()
		triggerAllProtocols(t, env, []string{"port1", "port2"}, []uint32{dscpBE1})
		stopCap()

		// MGT-2.1 Step 3: Perform the Standard Egress AF4 Verification on <DUT_PORT1>/<ATE_PORT1> and <DUT_PORT2>/<ATE_PORT2>.
		verifyStandardEgressAF4(t, env, []string{"port1", "port2"}, baselineAF4)
	})

	// =========================================================================
	// MGT-2.2: QoS Marking on Failover to OOB Interface
	// =========================================================================
	t.Run("MGT-2.2_QoS_Marking_on_Failover_to_OOB_Interface", func(t *testing.T) {
		t.Cleanup(func() {
			setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, true)
		})

		// MGT-2.2 Step 1: Record baseline AF4 queue counters on <DUT_PORT3>, disable <DUT_PORT1> and <DUT_PORT2>,
		// and verify VRF mgmt eBGP leaves ESTABLISHED and switches to the static default routes (preference = 220) over <DUT_PORT3>.
		baselineOOB := map[string]queueCounters{
			"port3": snapshotQueueCounters(t, dut, dut.Port(t, "port3").Name(), resolveQueueName(dut, dut.Port(t, "port3").Name(), env.af4Queue, 4)),
		}
		setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, false)
		awaitBGPSessionState(t, dut, []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}, oc.Bgp_Neighbor_SessionState_IDLE, time.Minute)

		// MGT-2.2 Step 2: Start packet capture on <ATE_PORT3>, run the 10-Protocol Trigger Suite, stop packet capture,
		// and perform the Standard Egress AF4 Verification on <DUT_PORT3>/<ATE_PORT3>.
		stopCapOOB := startPortCaptures(t, env, []string{"port3"})
		defer stopCapOOB()
		triggerAllProtocols(t, env, []string{"port3"}, []uint32{dscpBE1})
		stopCapOOB()
		verifyStandardEgressAF4(t, env, []string{"port3"}, baselineOOB)

		// MGT-2.2 Step 3: Re-enable <DUT_PORT1> and <DUT_PORT2>, verify eBGP returns to ESTABLISHED,
		// and verify traffic fails back to <DUT_PORT1>/<DUT_PORT2> with DSCP = 32 (AF4).
		setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, true)
		awaitBGPSessionState(t, dut, []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}, oc.Bgp_Neighbor_SessionState_ESTABLISHED, 2*time.Minute)

		baselineLC := map[string]queueCounters{
			"port1": snapshotQueueCounters(t, dut, dut.Port(t, "port1").Name(), resolveQueueName(dut, dut.Port(t, "port1").Name(), env.af4Queue, 4)),
			"port2": snapshotQueueCounters(t, dut, dut.Port(t, "port2").Name(), resolveQueueName(dut, dut.Port(t, "port2").Name(), env.af4Queue, 4)),
		}
		stopCapLC := startPortCaptures(t, env, []string{"port1", "port2"})
		defer stopCapLC()
		triggerAllProtocols(t, env, []string{"port1", "port2"}, []uint32{dscpBE1})
		stopCapLC()
		verifyStandardEgressAF4(t, env, []string{"port1", "port2"}, baselineLC)
	})

	// =========================================================================
	// MGT-2.3: Control and Management Traffic Survival Under Egress Port BE1 Congestion
	// =========================================================================
	t.Run("MGT-2.3_Control_Traffic_Survival_Under_Egress_Port_BE1_Congestion", func(t *testing.T) {
		p1Name := dut.Port(t, "port1").Name()
		p2Name := dut.Port(t, "port2").Name()

		// MGT-2.3 Step 1: Record baseline AF4 and BE1 queue counters on <DUT_PORT1> and <DUT_PORT2>.
		baselineAF4 := map[string]queueCounters{
			"port1": snapshotQueueCounters(t, dut, p1Name, resolveQueueName(dut, p1Name, env.af4Queue, 4)),
			"port2": snapshotQueueCounters(t, dut, p2Name, resolveQueueName(dut, p2Name, env.af4Queue, 4)),
		}
		baselineBE1 := map[string]queueCounters{
			"port1": snapshotQueueCounters(t, dut, p1Name, resolveQueueName(dut, p1Name, env.be1Queue, 0)),
			"port2": snapshotQueueCounters(t, dut, p2Name, resolveQueueName(dut, p2Name, env.be1Queue, 0)),
		}

		// MGT-2.3 Step 2: Inject oversubscribed background BE1 (DSCP 0) IPv4/IPv6 traffic in VRF mgmt
		// (from <ATE_PORT3>/<ATE_PORT2> toward <ATE_PORT1> and <ATE_PORT3>/<ATE_PORT1> toward <ATE_PORT2>)
		// to induce continuous BE1 tail-drops on <DUT_PORT1> and <DUT_PORT2>.
		stopCap := startPortCaptures(t, env, []string{"port1", "port2"})
		defer stopCap()
		injectBE1CongestionAndRunTriggers(t, env)
		stopCap()

		// MGT-2.3 Step 4: Verify BE1 dropped-pkts > 0 on <DUT_PORT1> and <DUT_PORT2>,
		// and verify the Standard Egress AF4 Verification passes with 0% packet loss (AF4 dropped-pkts == 0).
		for _, pid := range []string{"port1", "port2"} {
			pName := dut.Port(t, pid).Name()
			be1Q := resolveQueueName(dut, pName, env.be1Queue, 0)
			afterBE1 := snapshotQueueCounters(t, dut, pName, be1Q)
			if afterBE1.droppedPkts <= baselineBE1[pid].droppedPkts {
				t.Errorf("Expected BE1 (%s) dropped-pkts to increment under congestion on %s: before=%d, after=%d",
					be1Q, pName, baselineBE1[pid].droppedPkts, afterBE1.droppedPkts)
			}
		}
		verifyStandardEgressAF4(t, env, []string{"port1", "port2"}, baselineAF4)
	})

	// =========================================================================
	// MGT-2.4: Client-Initiated BE1 (DSCP 0) Request vs. DUT Egress AF4 (DSCP 32) Response
	// =========================================================================
	t.Run("MGT-2.4_Client_Initiated_BE1_and_CS1_Request_vs_DUT_Egress_AF4_Response", func(t *testing.T) {
		t.Cleanup(func() {
			setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, true)
		})

		// MGT-2.4 Step 1: Record baseline AF4/BE1 counters on <DUT_PORT1>-<DUT_PORT3>. Capture on <ATE_PORT1> and <ATE_PORT2>
		// while sending inbound client requests (gNMI, gRIBI, P4RT, gNOI, SSH) from 198.51.100.1 and 2001:db8:100::1
		// to <DUT_LO1> marked DSCP 0 (BE1) and DSCP 8 (CS1), then stop capture.
		baselineLC := map[string]queueCounters{
			"port1": snapshotQueueCounters(t, dut, dut.Port(t, "port1").Name(), resolveQueueName(dut, dut.Port(t, "port1").Name(), env.af4Queue, 4)),
			"port2": snapshotQueueCounters(t, dut, dut.Port(t, "port2").Name(), resolveQueueName(dut, dut.Port(t, "port2").Name(), env.af4Queue, 4)),
		}
		stopCapLC := startPortCaptures(t, env, []string{"port1", "port2"})
		defer stopCapLC()
		triggerAllProtocols(t, env, []string{"port1", "port2"}, []uint32{dscpBE1, dscpCS1})
		stopCapLC()
		verifyStandardEgressAF4(t, env, []string{"port1", "port2"}, baselineLC)

		// MGT-2.4 Step 2: Disable <DUT_PORT1> and <DUT_PORT2>, repeat Step 1 over OOB <ATE_PORT3>, then stop capture.
		baselineOOB := map[string]queueCounters{
			"port3": snapshotQueueCounters(t, dut, dut.Port(t, "port3").Name(), resolveQueueName(dut, dut.Port(t, "port3").Name(), env.af4Queue, 4)),
		}
		setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, false)
		awaitBGPSessionState(t, dut, []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}, oc.Bgp_Neighbor_SessionState_IDLE, time.Minute)

		stopCapOOB := startPortCaptures(t, env, []string{"port3"})
		defer stopCapOOB()
		triggerAllProtocols(t, env, []string{"port3"}, []uint32{dscpBE1, dscpCS1})
		stopCapOOB()

		// MGT-2.4 Step 3: Verify all DUT TCP ACK and RPC response packets on <ATE_PORT1>-<ATE_PORT3> egress with DSCP = 32 (AF4)
		// (never reflecting DSCP 0 or DSCP 8), then re-enable <DUT_PORT1> and <DUT_PORT2> and verify eBGP returns to ESTABLISHED.
		verifyStandardEgressAF4(t, env, []string{"port3"}, baselineOOB)

		setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, true)
		awaitBGPSessionState(t, dut, []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}, oc.Bgp_Neighbor_SessionState_ESTABLISHED, 2*time.Minute)
	})

	// =========================================================================
	// MGT-2.5: Partial Linecard Failure (ECMP Rehash) and Rapid Flap Stability
	// =========================================================================
	t.Run("MGT-2.5_Partial_Linecard_Failure_ECMP_Rehash_and_Rapid_Flap_Stability", func(t *testing.T) {
		t.Cleanup(func() {
			setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, true)
			restoreBatch := &gnmi.SetBatch{}
			cfgplugins.ConfigureNTPServers(t, dut, restoreBatch, true, []cfgplugins.NTPServerCfg{
				{Address: collectorIPv4, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv4},
				{Address: collectorIPv6, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv6},
			})
			cfgplugins.ConfigureRemoteSyslogServers(t, restoreBatch, []cfgplugins.SyslogServerCfg{
				{
					Host:            collectorIPv4,
					NetworkInstance: mgmtVRFName,
					RemotePort:      portSyslog,
					SourceAddress:   lo1IPv4,
					Facility:        oc.SystemLogging_SYSLOG_FACILITY_ALL,
					Severity:        oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
				},
				{
					Host:            collectorIPv6,
					NetworkInstance: mgmtVRFName,
					RemotePort:      portSyslog,
					SourceAddress:   lo1IPv6,
					Facility:        oc.SystemLogging_SYSLOG_FACILITY_ALL,
					Severity:        oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
				},
			})
			restoreBatch.Set(t, dut)
		})

		// MGT-2.5 Step 1: Establish active flows for the 10-Protocol Trigger Suite, then disable only <DUT_PORT1>
		// while keeping <DUT_PORT2> ESTABLISHED.
		setDUTPortsEnabled(t, dut, []string{"port1"}, false)
		awaitBGPSessionState(t, dut, []string{atePort1.IPv4, atePort1.IPv6}, oc.Bgp_Neighbor_SessionState_IDLE, time.Minute)
		awaitBGPSessionState(t, dut, []string{atePort2.IPv4, atePort2.IPv6}, oc.Bgp_Neighbor_SessionState_ESTABLISHED, time.Minute)

		baselineP2 := map[string]queueCounters{
			"port2": snapshotQueueCounters(t, dut, dut.Port(t, "port2").Name(), resolveQueueName(dut, dut.Port(t, "port2").Name(), env.af4Queue, 4)),
		}
		stopCapRehash := startPortCaptures(t, env, []string{"port2", "port3"})
		defer stopCapRehash()
		triggerAllProtocols(t, env, []string{"port2"}, []uint32{dscpBE1})
		stopCapRehash()

		// MGT-2.5 Step 2: Verify 0 packets fail over to OOB <DUT_PORT3>, and all rehashed flows egress <DUT_PORT2>
		// meeting the Standard Egress AF4 Verification.
		oobSummary := inspectPortPCAP(t, ate, "port3")
		if oobSummary.dutOriginatedV4Pkts+oobSummary.dutOriginatedV6Pkts > 0 {
			t.Errorf("MGT-2.5 Step 2: expected 0 management packets on OOB port3 while port2 eBGP is ESTABLISHED, got %d",
				oobSummary.dutOriginatedV4Pkts+oobSummary.dutOriginatedV6Pkts)
		}
		verifyStandardEgressAF4(t, env, []string{"port2"}, baselineP2)

		// MGT-2.5 Step 3: Flap <DUT_PORT1> and <DUT_PORT2> 5 times (forcing repeated LC <-> OOB transitions)
		// while deleting and re-creating the NTP and Syslog servers via gNMI.Set.
		for i := 0; i < 5; i++ {
			setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, false)
			gnmi.Delete(t, dut, gnmi.OC().System().Ntp().Server(collectorIPv4).Config())
			gnmi.Delete(t, dut, gnmi.OC().System().Logging().RemoteServer(collectorIPv4).Config())

			recreateBatch := &gnmi.SetBatch{}
			cfgplugins.ConfigureNTPServers(t, dut, recreateBatch, true, []cfgplugins.NTPServerCfg{
				{Address: collectorIPv4, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv4},
				{Address: collectorIPv6, NetworkInstance: mgmtVRFName, SourceAddress: lo1IPv6},
			})
			cfgplugins.ConfigureRemoteSyslogServers(t, recreateBatch, []cfgplugins.SyslogServerCfg{
				{
					Host:            collectorIPv4,
					NetworkInstance: mgmtVRFName,
					RemotePort:      portSyslog,
					SourceAddress:   lo1IPv4,
					Facility:        oc.SystemLogging_SYSLOG_FACILITY_ALL,
					Severity:        oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
				},
				{
					Host:            collectorIPv6,
					NetworkInstance: mgmtVRFName,
					RemotePort:      portSyslog,
					SourceAddress:   lo1IPv6,
					Facility:        oc.SystemLogging_SYSLOG_FACILITY_ALL,
					Severity:        oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
				},
			})
			recreateBatch.Set(t, dut)
			setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, true)
		}

		// MGT-2.5 Step 4: Stop captures on <ATE_PORT1>-<ATE_PORT3>, ensure <DUT_PORT1> and <DUT_PORT2> are enabled
		// with eBGP ESTABLISHED, and verify all 10 protocols retain DSCP = 32 (AF4) throughout.
		awaitBGPSessionState(t, dut, []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}, oc.Bgp_Neighbor_SessionState_ESTABLISHED, 2*time.Minute)
		baselinePostFlap := map[string]queueCounters{
			"port1": snapshotQueueCounters(t, dut, dut.Port(t, "port1").Name(), resolveQueueName(dut, dut.Port(t, "port1").Name(), env.af4Queue, 4)),
			"port2": snapshotQueueCounters(t, dut, dut.Port(t, "port2").Name(), resolveQueueName(dut, dut.Port(t, "port2").Name(), env.af4Queue, 4)),
		}
		stopCapPostFlap := startPortCaptures(t, env, []string{"port1", "port2", "port3"})
		defer stopCapPostFlap()
		triggerAllProtocols(t, env, []string{"port1", "port2"}, []uint32{dscpBE1})
		stopCapPostFlap()
		verifyStandardEgressAF4(t, env, []string{"port1", "port2"}, baselinePostFlap)
	})

	// =========================================================================
	// MGT-2.6: Negative Verification of Protocol-Level DSCP Override and Queue Isolation
	// =========================================================================
	t.Run("MGT-2.6_Negative_Verification_of_Protocol_Level_DSCP_Override_and_Queue_Isolation", func(t *testing.T) {
		t.Cleanup(func() {
			gnmi.Replace(t, dut, gnmi.OC().Sampling().Sflow().Dscp().Config(), uint8(dscpAF4))
		})

		// MGT-2.6 Step 1: Set /sampling/sflow/config/dscp = 0 (BE1) via gNMI.Set, capture on <ATE_PORT1> and <ATE_PORT2>,
		// run the 10-Protocol Trigger Suite, and stop capture.
		gnmi.Replace(t, dut, gnmi.OC().Sampling().Sflow().Dscp().Config(), uint8(dscpBE1))
		gnmi.Await(t, dut, gnmi.OC().Sampling().Sflow().Dscp().State(), 30*time.Second, uint8(dscpBE1))

		p1Name := dut.Port(t, "port1").Name()
		p2Name := dut.Port(t, "port2").Name()
		baselineAF4 := map[string]queueCounters{
			"port1": snapshotQueueCounters(t, dut, p1Name, resolveQueueName(dut, p1Name, env.af4Queue, 4)),
			"port2": snapshotQueueCounters(t, dut, p2Name, resolveQueueName(dut, p2Name, env.af4Queue, 4)),
		}
		baselineBE1 := map[string]queueCounters{
			"port1": snapshotQueueCounters(t, dut, p1Name, resolveQueueName(dut, p1Name, env.be1Queue, 0)),
			"port2": snapshotQueueCounters(t, dut, p2Name, resolveQueueName(dut, p2Name, env.be1Queue, 0)),
		}

		stopCap := startPortCaptures(t, env, []string{"port1", "port2"})
		defer stopCap()
		triggerAllProtocols(t, env, []string{"port1", "port2"}, []uint32{dscpBE1})
		stopCap()

		// MGT-2.6 Step 2: Verify the 9 remaining protocols still meet the Standard Egress AF4 Verification
		// (and if VRF mgmt enforces egress remarking, sFlow is also remarked to DSCP = 32);
		// verify negatively that any packet egressing with DSCP 0 (BE1) increments only queue BE1 and not queue AF4.
		for _, pid := range []string{"port1", "port2"} {
			pName := dut.Port(t, pid).Name()
			pcapSum := inspectPortPCAP(t, ate, pid)
			if pcapSum.be1Pkts > 0 {
				be1Q := resolveQueueName(dut, pName, env.be1Queue, 0)
				afterBE1 := snapshotQueueCounters(t, dut, pName, be1Q)
				if afterBE1.txPkts <= baselineBE1[pid].txPkts {
					t.Errorf("MGT-2.6 Step 2: captured %d DSCP 0 packets on %s, but queue BE1 (%s) transmit-pkts did not increment",
						pcapSum.be1Pkts, pid, be1Q)
				}
			}
			af4Q := resolveQueueName(dut, pName, env.af4Queue, 4)
			afterAF4 := snapshotQueueCounters(t, dut, pName, af4Q)
			if afterAF4.droppedPkts > baselineAF4[pid].droppedPkts {
				t.Errorf("MGT-2.6 Step 2: queue AF4 (%s) on %s dropped packets", af4Q, pName)
			}
		}

		// MGT-2.6 Step 3: Restore /sampling/sflow/config/dscp = 32 via gNMI.Set and verify /sampling/sflow/state/dscp == 32.
		gnmi.Replace(t, dut, gnmi.OC().Sampling().Sflow().Dscp().Config(), uint8(dscpAF4))
		gnmi.Await(t, dut, gnmi.OC().Sampling().Sflow().Dscp().State(), 30*time.Second, uint8(dscpAF4))
	})

	// =========================================================================
	// MGT-2.7: Negative Verification of Total Management Path Failure (VRF Isolation)
	// =========================================================================
	t.Run("MGT-2.7_Negative_Verification_of_Total_Management_Path_Failure_VRF_Isolation", func(t *testing.T) {
		defaultNI := deviations.DefaultNetworkInstance(dut)

		t.Cleanup(func() {
			setDUTPortsEnabled(t, dut, []string{"port1", "port2", "port3"}, true)
			delBatch := &gnmi.SetBatch{}
			_, _ = cfgplugins.NewStaticRouteCfg(delBatch, &cfgplugins.StaticRouteCfg{
				NetworkInstance:   defaultNI,
				Prefix:            defaultIPv4Route,
				RemoveStaticRoute: true,
			}, dut)
			_, _ = cfgplugins.NewStaticRouteCfg(delBatch, &cfgplugins.StaticRouteCfg{
				NetworkInstance:   defaultNI,
				Prefix:            defaultIPv6Route,
				RemoveStaticRoute: true,
			}, dut)
			delBatch.Set(t, dut)
		})

		// MGT-2.7 Step 1: Configure temporary default routes (0.0.0.0/0 via 192.0.2.14, ::/0 via 2001:db8::e)
		// in the DEFAULT VRF over <DUT_PORT4>.
		tempRoutesBatch := &gnmi.SetBatch{}
		if _, err := cfgplugins.NewStaticRouteCfg(tempRoutesBatch, &cfgplugins.StaticRouteCfg{
			NetworkInstance: defaultNI,
			Prefix:          defaultIPv4Route,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				staticRouteIdx: oc.UnionString(atePort4.IPv4),
			},
		}, dut); err != nil {
			t.Fatalf("Failed to configure temporary DEFAULT VRF IPv4 route: %v", err)
		}
		if _, err := cfgplugins.NewStaticRouteCfg(tempRoutesBatch, &cfgplugins.StaticRouteCfg{
			NetworkInstance: defaultNI,
			Prefix:          defaultIPv6Route,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				staticRouteIdx: oc.UnionString(atePort4.IPv6),
			},
		}, dut); err != nil {
			t.Fatalf("Failed to configure temporary DEFAULT VRF IPv6 route: %v", err)
		}
		tempRoutesBatch.Set(t, dut)

		// MGT-2.7 Step 2: Disable <DUT_PORT1>, <DUT_PORT2>, and <DUT_PORT3>, capture on <ATE_PORT1>-<ATE_PORT4>,
		// and trigger the 10-Protocol Trigger Suite.
		setDUTPortsEnabled(t, dut, []string{"port1", "port2", "port3"}, false)
		stopCapLeak := startPortCaptures(t, env, []string{"port4"})
		defer stopCapLeak()
		triggerAllProtocols(t, env, nil, []uint32{dscpBE1})
		stopCapLeak()

		// MGT-2.7 Step 3: Verify negatively on <ATE_PORT4> that 0 VRF mgmt packets leak into the DEFAULT VRF.
		leakSummary := inspectPortPCAP(t, ate, "port4")
		if leakSummary.dutOriginatedV4Pkts+leakSummary.dutOriginatedV6Pkts > 0 {
			t.Errorf("MGT-2.7 Step 3 VRF Isolation violation: captured %d VRF mgmt packets leaking onto DEFAULT VRF port4, want 0",
				leakSummary.dutOriginatedV4Pkts+leakSummary.dutOriginatedV6Pkts)
		}

		// MGT-2.7 Step 4: Re-enable <DUT_PORT3> then <DUT_PORT1> and <DUT_PORT2>, stop captures,
		// verify recovery on <DUT_PORT3> and failback to <DUT_PORT1>/<DUT_PORT2> meet the Standard Egress AF4 Verification,
		// and remove the temporary DEFAULT VRF default routes.
		baselineOOB := map[string]queueCounters{
			"port3": snapshotQueueCounters(t, dut, dut.Port(t, "port3").Name(), resolveQueueName(dut, dut.Port(t, "port3").Name(), env.af4Queue, 4)),
		}
		setDUTPortsEnabled(t, dut, []string{"port3"}, true)
		stopCapOOB := startPortCaptures(t, env, []string{"port3"})
		defer stopCapOOB()
		triggerAllProtocols(t, env, []string{"port3"}, []uint32{dscpBE1})
		stopCapOOB()
		verifyStandardEgressAF4(t, env, []string{"port3"}, baselineOOB)

		setDUTPortsEnabled(t, dut, []string{"port1", "port2"}, true)
		awaitBGPSessionState(t, dut, []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}, oc.Bgp_Neighbor_SessionState_ESTABLISHED, 2*time.Minute)
		baselineLC := map[string]queueCounters{
			"port1": snapshotQueueCounters(t, dut, dut.Port(t, "port1").Name(), resolveQueueName(dut, dut.Port(t, "port1").Name(), env.af4Queue, 4)),
			"port2": snapshotQueueCounters(t, dut, dut.Port(t, "port2").Name(), resolveQueueName(dut, dut.Port(t, "port2").Name(), env.af4Queue, 4)),
		}
		stopCapLC := startPortCaptures(t, env, []string{"port1", "port2"})
		defer stopCapLC()
		triggerAllProtocols(t, env, []string{"port1", "port2"}, []uint32{dscpBE1})
		stopCapLC()
		verifyStandardEgressAF4(t, env, []string{"port1", "port2"}, baselineLC)
	})
}

// injectBE1CongestionAndRunTriggers injects oversubscribed BE1 (DSCP 0) background traffic on port1 and port2 while running the 10-Protocol Trigger Suite.
func injectBE1CongestionAndRunTriggers(t *testing.T, env *testEnv) {
	t.Helper()
	env.top.Flows().Clear()

	p1MAC, ok1 := gnmi.Lookup(t, env.dut, gnmi.OC().Interface(env.dut.Port(t, "port1").Name()).Ethernet().MacAddress().State()).Val()
	if !ok1 {
		t.Fatalf("Failed to lookup MAC address for port1")
	}
	p2MAC, ok2 := gnmi.Lookup(t, env.dut, gnmi.OC().Interface(env.dut.Port(t, "port2").Name()).Ethernet().MacAddress().State()).Val()
	if !ok2 {
		t.Fatalf("Failed to lookup MAC address for port2")
	}
	p3MAC, ok3 := gnmi.Lookup(t, env.dut, gnmi.OC().Interface(env.dut.Port(t, "port3").Name()).Ethernet().MacAddress().State()).Val()
	if !ok3 {
		t.Fatalf("Failed to lookup MAC address for port3")
	}

	// Oversubscribe egress port1 from port2 (60% line rate) + port3 (60% line rate) = 120% BE1 (DSCP 0).
	for _, src := range []struct {
		txPort string
		srcMAC string
		dstMAC string
		srcV4  string
		srcV6  string
	}{
		{txPort: "port2", srcMAC: atePort2.MAC, dstMAC: p2MAC, srcV4: atePort2.IPv4, srcV6: atePort2.IPv6},
		{txPort: "port3", srcMAC: atePort3.MAC, dstMAC: p3MAC, srcV4: atePort3.IPv4, srcV6: atePort3.IPv6},
	} {
		f := env.top.Flows().Add().SetName("congest_to_p1_from_" + src.txPort)
		f.Metrics().SetEnable(true)
		f.TxRx().Port().SetTxName(src.txPort).SetRxNames([]string{"port1"})
		f.Size().SetFixed(1024)
		f.Rate().SetPercentage(60)
		f.Duration().FixedPackets().SetPackets(50000)
		eth := f.Packet().Add().Ethernet()
		eth.Src().SetValue(src.srcMAC)
		eth.Dst().SetValue(src.dstMAC)
		ip4 := f.Packet().Add().Ipv4()
		ip4.Src().SetValue(src.srcV4)
		ip4.Dst().SetValue(atePort1.IPv4)
		ip4.Priority().Dscp().Phb().SetValue(dscpBE1)
	}

	// Oversubscribe egress port2 from port1 (60% line rate) + port3 (60% line rate) = 120% BE1 (DSCP 0).
	for _, src := range []struct {
		txPort string
		srcMAC string
		dstMAC string
		srcV4  string
		srcV6  string
	}{
		{txPort: "port1", srcMAC: atePort1.MAC, dstMAC: p1MAC, srcV4: atePort1.IPv4, srcV6: atePort1.IPv6},
		{txPort: "port3", srcMAC: atePort3.MAC, dstMAC: p3MAC, srcV4: atePort3.IPv4, srcV6: atePort3.IPv6},
	} {
		f := env.top.Flows().Add().SetName("congest_to_p2_from_" + src.txPort)
		f.Metrics().SetEnable(true)
		f.TxRx().Port().SetTxName(src.txPort).SetRxNames([]string{"port2"})
		f.Size().SetFixed(1024)
		f.Rate().SetPercentage(60)
		f.Duration().FixedPackets().SetPackets(50000)
		eth := f.Packet().Add().Ethernet()
		eth.Src().SetValue(src.srcMAC)
		eth.Dst().SetValue(src.dstMAC)
		ip6 := f.Packet().Add().Ipv6()
		ip6.Src().SetValue(src.srcV6)
		ip6.Dst().SetValue(atePort2.IPv6)
		ip6.TrafficClass().SetValue(dscpBE1)
	}

	// MGT-2.3 Step 3: Run the 10-Protocol Trigger Suite while BE1 congestion is active, then stop traffic.
	configureAndRunProtocolFlows(t, env, []string{"port1", "port2"}, []uint32{dscpBE1}, false)
	triggerAllProtocols(t, env, []string{"port1", "port2"}, []uint32{dscpBE1})
}
