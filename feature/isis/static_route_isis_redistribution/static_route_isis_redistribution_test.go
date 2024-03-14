package basic_static_route_support_test

import (
	"fmt"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygot/ygot"
	"math"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ipv4PrefixLen         = 30
	ipv6PrefixLen         = 126
	isisInstance          = "DEFAULT"
	dutAreaAddr           = "49.0001"
	ateAreaAddr           = "49.0002"
	dutSysID              = "1920.0000.2001"
	ate1SysID             = "640000000001"
	ate2SysID             = "640000000002"
	v4Route               = "192.168.10.0"
	v4TrafficStart        = "192.168.10.1"
	v4RoutePrefix         = uint32(24)
	v6Route               = "2024:db8:128:128::"
	v6TrafficStart        = "2001:db8:128:128::1"
	v6RoutePrefix         = uint32(64)
	v4LoopbackRoute       = "192.168.1.4"
	v4LoopbackRoutePrefix = uint32(30)
	v6LoopbackRoute       = "2001:db8:64:64::1"
	v6LoopbackRoutePrefix = uint32(128)
	v4Flow                = "v4Flow"
	v6Flow                = "v6Flow"
	trafficDuration       = 2 * time.Minute
	lossTolerance         = float64(1)
	v4RoutePolicy         = "route-policy-v4"
	v4Statement           = "statement-v4"
	v4PrefixSet           = "prefix-set-v4"
	StaticRouteTag1       = uint32(40)
	StaticRouteMetric     = 100
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:01:01:01:02",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:9",
		IPv6Len: ipv6PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:01:01:01:03",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type ipAddr struct {
	address string
	prefix  uint32
}

func (ip *ipAddr) cidr(t *testing.T) string {
	_, net, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.address, ip.prefix))
	if err != nil {
		t.Fatal(err)
	}
	return net.String()
}

type testData struct {
	dut            *ondatra.DUTDevice
	ate            *ondatra.ATEDevice
	top            gosnappi.Config
	otgP1          gosnappi.Device
	otgP2          gosnappi.Device
	otgP3          gosnappi.Device
	staticIPv4     ipAddr
	staticIPv6     ipAddr
	advertisedIPv4 ipAddr
	advertisedIPv6 ipAddr
}

func configureRoutePolicy(d *oc.Root) (*oc.RoutingPolicy, error) {

	ipPrefixSet := "203.0.113.0"
	prefixSet := "prefix-set-v4"
	prefixSubnetRange := "exact"
	allowConnected := "route-policy-v4"
	aclStatement1 := "1"
	rp := d.GetOrCreateRoutingPolicy()
	pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSet)
	pset.GetOrCreatePrefix(ipPrefixSet, prefixSubnetRange)
	pdef := rp.GetOrCreatePolicyDefinition(allowConnected)
	stmt, err := pdef.AppendNewStatement(aclStatement1)
	if err != nil {
		return nil, err
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(prefixSet)
	if err != nil {
		return nil, err
	}
	return rp, nil
}

func configureStaticRoute(t *testing.T,
	dut *ondatra.DUTDevice,
	ipv4Route string,
	ipv4Mask string,
	tagValueV4 uint32,
	metricValueV4 uint32,
	ipv6Route string,
	ipv6Mask string,
	tagValueV6 uint32,
	metricValueV6 uint32) {

	// For IPv4
	staticRoute1 := ipv4Route + "/" + ipv4Mask

	// For IPv6
	staticRoute2 := ipv6Route + "/" + ipv6Mask

	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(staticRoute1)
	sr.SetTag, _ = sr.To_NetworkInstance_Protocol_Static_SetTag_Union(tagValueV4)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(atePort2.IPv4)
	nh.Metric = ygot.Uint32(metricValueV4)

	sr2 := static.GetOrCreateStatic(staticRoute2)
	sr2.SetTag, _ = sr.To_NetworkInstance_Protocol_Static_SetTag_Union(tagValueV6)
	nh2 := sr2.GetOrCreateNextHop("0")
	nh2.NextHop = oc.UnionString(atePort1.IPv6)
	nh2.Metric = ygot.Uint32(metricValueV6)

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(
		oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		deviations.StaticProtocolName(dut)).Config(),
		static)
}

func (td *testData) configureOTGFlows(t *testing.T) {
	t.Helper()

	srcV4 := td.otgP3.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	srcV6 := td.otgP3.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	dst1V4 := td.otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst1V6 := td.otgP1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	dst2V4 := td.otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst2V6 := td.otgP2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	v4F := td.top.Flows().Add()
	v4F.SetName(v4Flow).Metrics().SetEnable(true)
	v4F.TxRx().Device().SetTxNames([]string{srcV4.Name()}).SetRxNames([]string{dst1V4.Name(), dst2V4.Name()})

	v4FEth := v4F.Packet().Add().Ethernet()
	v4FEth.Src().SetValue(atePort3.MAC)

	v4FIp := v4F.Packet().Add().Ipv4()
	v4FIp.Src().SetValue(srcV4.Address())
	v4FIp.Dst().Increment().SetStart(v4TrafficStart).SetCount(254)

	eth := v4F.EgressPacket().Add().Ethernet()
	ethTag := eth.Dst().MetricTags().Add()
	ethTag.SetName("MACTrackingv4").SetOffset(36).SetLength(12)

	v6F := td.top.Flows().Add()
	v6F.SetName(v6Flow).Metrics().SetEnable(true)
	v6F.TxRx().Device().SetTxNames([]string{srcV6.Name()}).SetRxNames([]string{dst1V6.Name(), dst2V6.Name()})

	v6FEth := v6F.Packet().Add().Ethernet()
	v6FEth.Src().SetValue(atePort3.MAC)

	v6FIP := v6F.Packet().Add().Ipv6()
	v6FIP.Src().SetValue(srcV6.Address())
	v6FIP.Dst().Increment().SetStart(v6TrafficStart).SetCount(math.MaxInt32)

	eth = v6F.EgressPacket().Add().Ethernet()
	ethTag = eth.Dst().MetricTags().Add()
	ethTag.SetName("MACTrackingv6").SetOffset(36).SetLength(12)
}

func pollOTGIsisLSD(t *testing.T,
	ate *ondatra.ATEDevice,
	isisName string,
	isisPrefix string) {

	path := gnmi.OTG().IsisRouter(isisName).LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().Prefix(isisPrefix).State()
	_, ok := gnmi.WatchAll(t, ate.OTG(),
		path, 30*time.Second,
		func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_ExtendedIpv4Reachability_Prefix]) bool {
			prefix, present := v.Val()
			return present && prefix.GetPrefix() == isisPrefix
		}).Await(t)
	if ok {
		t.Errorf("Prefix found, not want:")
	}
}

func (td *testData) awaitISISAdjacency(t *testing.T, p *ondatra.Port, isisName string) error {
	t.Helper()
	isis := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).Isis()
	intf := isis.Interface(p.Name())
	if deviations.ExplicitInterfaceInDefaultVRF(td.dut) {
		intf = isis.Interface(p.Name() + ".0")
	}
	query := intf.Level(2).AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, td.dut, query, time.Minute, func(v *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		state, _ := v.Val()
		return v.IsPresent() && state == oc.Isis_IsisInterfaceAdjState_UP
	}).Await(t)

	if !ok {
		return fmt.Errorf("timeout - waiting for adjacency state")
	}
	return nil
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	b := &gnmi.SetBatch{}
	gnmi.BatchReplace(b, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.BatchReplace(b, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.BatchReplace(b, gnmi.OC().Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))
	b.Set(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) []gosnappi.Device {
	t.Helper()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	d1 := atePort1.AddToOTG(top, p1, &dutPort1)
	d2 := atePort2.AddToOTG(top, p2, &dutPort2)
	d3 := atePort3.AddToOTG(top, p3, &dutPort3)
	return []gosnappi.Device{d1, d2, d3}
}

func configureDutISIS(t *testing.T, dut *ondatra.DUTDevice, intfName []string, dutAreaAddress, dutSysID string) {
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}

	for _, intf := range intfName {
		isisIntf := isis.GetOrCreateInterface(intf)
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		// Configure ISIS level at global mode if true else at interface mode
		if deviations.ISISInterfaceLevel1DisableRequired(dut) {
			isisIntf.GetOrCreateLevel(1).Enabled = ygot.Bool(false)
		} else {
			isisIntf.GetOrCreateLevel(2).Enabled = ygot.Bool(true)
		}
		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)
		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(200)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)

		//isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		//isisIntfLevelAfiv6.Metric = ygot.Uint32(200)
		//isisIntfLevelAfiv6.Enabled = ygot.Bool(true)

		if deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntfLevel.Af = nil
		}
	}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Config(), prot)
}

func (td *testData) advertiseRoutesWithISIS(t *testing.T) {
	t.Helper()

	dev1ISIS := td.otgP1.Isis().SetSystemId(ate1SysID).SetName(td.otgP1.Name() + ".ISIS")
	dev1ISIS.Basic().SetHostname(dev1ISIS.Name()).SetLearnedLspFilter(true)
	dev1ISIS.Advanced().SetAreaAddresses([]string{strings.Replace(ateAreaAddr, ".", "", -1)})
	dev1IsisInt := dev1ISIS.Interfaces().Add().
		SetEthName(td.otgP1.Ethernets().Items()[0].Name()).SetName("dev1IsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	dev1IsisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	dev2ISIS := td.otgP2.Isis().SetSystemId(ate2SysID).SetName(td.otgP2.Name() + ".ISIS")
	dev2ISIS.Basic().SetHostname(dev2ISIS.Name()).SetLearnedLspFilter(true)
	dev2ISIS.Advanced().SetAreaAddresses([]string{strings.Replace(ateAreaAddr, ".", "", -1)})
	dev2IsisInt := dev2ISIS.Interfaces().Add().
		SetEthName(td.otgP2.Ethernets().Items()[0].Name()).SetName("dev2IsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	dev2IsisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	// configure emulated network params
	net2v4 := td.otgP1.Isis().V4Routes().Add().SetName("v4-isisNet-dev1").SetLinkMetric(10)
	net2v4.Addresses().Add().SetAddress(td.advertisedIPv4.address).SetPrefix(td.advertisedIPv4.prefix)
	net2v6 := td.otgP1.Isis().V6Routes().Add().SetName("v6-isisNet-dev1").SetLinkMetric(10)
	net2v6.Addresses().Add().SetAddress(td.advertisedIPv6.address).SetPrefix(td.advertisedIPv6.prefix)

	net3v4 := td.otgP2.Isis().V4Routes().Add().SetName("v4-isisNet-dev2").SetLinkMetric(10)
	net3v4.Addresses().Add().SetAddress(td.advertisedIPv4.address).SetPrefix(td.advertisedIPv4.prefix)
	net3v6 := td.otgP2.Isis().V6Routes().Add().SetName("v6-isisNet-dev2").SetLinkMetric(10)
	net3v6.Addresses().Add().SetAddress(td.advertisedIPv6.address).SetPrefix(td.advertisedIPv6.prefix)
}

func TestStaticToISISRedistribution(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	devs := configureOTG(t, ate, top)
	p1Dut := dut.Port(t, "port1")
	p2Dut := dut.Port(t, "port2")

	td := testData{
		dut:            dut,
		ate:            ate,
		top:            top,
		otgP1:          devs[0],
		otgP2:          devs[1],
		otgP3:          devs[2],
		staticIPv4:     ipAddr{address: v4Route, prefix: v4RoutePrefix},
		staticIPv6:     ipAddr{address: v6Route, prefix: v6RoutePrefix},
		advertisedIPv4: ipAddr{address: v4LoopbackRoute, prefix: v4LoopbackRoutePrefix},
		advertisedIPv6: ipAddr{address: v6LoopbackRoute, prefix: v6LoopbackRoutePrefix},
	}

	t.Run("Initial Setup", func(t *testing.T) {

		t.Run("Configure Static Route on DUT", func(t *testing.T) {
			ipv4Mask := strconv.FormatUint(uint64(v4RoutePrefix), 10)
			ipv6Mask := strconv.FormatUint(uint64(v6RoutePrefix), 10)

			configureStaticRoute(t, dut, v4Route, ipv4Mask, 40, 104,
				v6Route, ipv6Mask, 60, 106)
		})

		t.Run("Configure ISIS on DUT", func(t *testing.T) {
			dut1PortNames := []string{p1Dut.Name(), p2Dut.Name()}
			configureDutISIS(t, dut, dut1PortNames, dutAreaAddr, dutSysID)
		})

		t.Run("OTG Configuration", func(t *testing.T) {
			td.advertiseRoutesWithISIS(t)
			td.configureOTGFlows(t)
			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)
			//defer ate.OTG().StopProtocols(t)
			otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
			otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
			t.Log(top.String())
		})

		t.Run("Await ISIS Status", func(t *testing.T) {
			if err := td.awaitISISAdjacency(t, dut.Port(t, "port1"), isisInstance); err != nil {
				t.Fatal(err)
			}
			if err := td.awaitISISAdjacency(t, dut.Port(t, "port2"), isisInstance); err != nil {
				t.Fatal(err)
			}

		})

		t.Run("Configure Route-Policy", func(t *testing.T) {
			d := &oc.Root{}
			rpl, err := configureRoutePolicy(d)
			if err != nil {
				t.Fatalf("Failed to configure Route Policy: %v", err)
			}
			gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)

		})

		t.Run("Attach Route Policy to ISIS", func(t *testing.T) {
			dni := deviations.DefaultNetworkInstance(dut)
			tblconn := &oc.Root{}

			tableConn := tblconn.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(
				oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
				oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
				oc.Types_ADDRESS_FAMILY_IPV4)

			tableConn.ImportPolicy = []string{v4RoutePolicy}

			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(dni).TableConnection(
				oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
				oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
				oc.Types_ADDRESS_FAMILY_IPV4).Config(), tableConn)

		})

	})

	cases := []struct {
		desc                      string
		staticMetricv4            uint32
		staticv4Tag               uint32
		staticMetricv6            uint32
		staticv6Tag               uint32
		policyStmtType            oc.E_RoutingPolicy_PolicyResultType
		DefaultPolicyStmtType     oc.E_RoutingPolicy_DefaultPolicyType
		policyType                string
		metricPropogation         bool
		protoSrc                  oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE
		protoDst                  oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE
		protoAf                   oc.E_Types_ADDRESS_FAMILY
		importPolicyConfig        bool
		importPolicyVerify        bool
		defaultImportPolicyConfig bool
		defaultImportPolicyVerify bool
	}{{
		desc: "RT-2.12.1: Redistribute IPv4 static route to IS-IS " +
			"with metric propogation diabled",
		staticMetricv4:            uint32(104),
		staticv4Tag:               uint32(40),
		staticMetricv6:            uint32(106),
		staticv6Tag:               uint32(60),
		metricPropogation:         true,
		DefaultPolicyStmtType:     oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE,
		protoSrc:                  oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		protoDst:                  oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
		protoAf:                   oc.Types_ADDRESS_FAMILY_IPV4,
		policyType:                "ACCEPT",
		defaultImportPolicyConfig: true,
		defaultImportPolicyVerify: true,
	}, {
		desc:                      "RT-2.12.2: Redistribute IPv4 static route to IS-IS with metric propogation enabled",
		metricPropogation:         false,
		defaultImportPolicyConfig: true,
		defaultImportPolicyVerify: true,
	}, {
		desc: "RT-2.12.3: Redistribute IPv6 static route to IS-IS with metric propogation diabled",
	}, {
		desc: "RT-2.12.4: Redistribute IPv6 static route to IS-IS with metric propogation enabled",
	}, {
		desc: "RT-2.12.5: Redistribute IPv4 and IPv6 static route to IS-IS with default-import-policy set to reject",
	}, {
		desc: "RT-2.12.6: Redistribute IPv4 static route to IS-IS matching a prefix using a route-policy",
	}, {
		desc: "RT-2.12.7: Redistribute IPv4 static route to IS-IS matching a tag",
	}, {
		desc: "RT-2.12.8: Redistribute IPv6 static route to IS-IS matching a prefix using a route-policy",
	}, {
		desc: "RT-2.12.9: Redistribute IPv6 static route to IS-IS matching a prefix using a route-policy",
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			dni := deviations.DefaultNetworkInstance(dut)
			tblconn := &oc.Root{}

			if tc.defaultImportPolicyConfig {
				t.Run(fmt.Sprintf("Config Default Policy Type %v", tc.policyType), func(t *testing.T) {
					tableConn := tblconn.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(
						tc.protoSrc, tc.protoDst, tc.protoAf)
					tableConn.SetDefaultImportPolicy(tc.DefaultPolicyStmtType)
					tableConn.SetDisableMetricPropagation(tc.metricPropogation)

					gnmi.Update(t, dut, gnmi.OC().NetworkInstance(dni).TableConnection(
						tc.protoSrc, tc.protoDst, tc.protoAf).Config(), tableConn)
				})

			}

			if tc.importPolicyConfig {
				t.Run(fmt.Sprintf("Config Default Policy Type %v", tc.policyType), func(t *testing.T) {
					tableConn := tblconn.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(
						tc.protoSrc, tc.protoDst, tc.protoAf)

					tableConn.ImportPolicy = []string{v4RoutePolicy}

					gnmi.Update(t, dut, gnmi.OC().NetworkInstance(dni).TableConnection(
						tc.protoSrc, tc.protoDst, tc.protoAf).Config(), tableConn)

				})
			}

			if tc.defaultImportPolicyVerify {
				t.Run(fmt.Sprintf("Verify Route propagation for %v", v4Route), func(t *testing.T) {
					pollOTGIsisLSD(t, ate, "atePort1.ISIS", v4Route)
				})
			}

		})
	}
}
