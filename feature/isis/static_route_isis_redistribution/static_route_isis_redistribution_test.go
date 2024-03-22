package basic_static_route_support_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openconfig/featureprofiles/internal/isissession"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"math"
	"net"
	"strconv"
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
	prefixMatch           = "exact"
	v4RoutePolicy         = "route-policy-v4"
	v4Statement           = "statement-v4"
	v4PrefixSet           = "prefix-set-v4"

	v6RoutePolicy = "route-policy-v6"
	v6Statement   = "statement-v6"
	v6PrefixSet   = "prefix-set-v6"

	StaticRouteTag1   = uint32(40)
	StaticRouteMetric = 100
	protoSrc          = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC
	protoDst          = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
)

var (
	globalTS isissession.TestSession
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

type TableConnectionConfig struct {
	ImportPolicy             []string `json:"import-policy"`
	DisableMetricPropagation bool     `json:"disable-metric-propagation"`
	DstProtocol              string   `json:"dst-protocol"`
	AddressFamily            string   `json:"address-family"`
	SrcProtocol              string   `json:"src-protocol"`
}

func getAndVerifyIsisImportPolicy(t *testing.T,
	dut *ondatra.DUTDevice, DisableMetricValue bool,
	RplName string, addressFamily string) {

	gnmiClient := dut.RawAPIs().GNMI(t)
	getResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{
				{Name: "network-instances"},
				{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
				{Name: "table-connections"},
				{Name: "table-connection", Key: map[string]string{
					"src-protocol":   "STATIC",
					"dst-protocol":   "ISIS",
					"address-family": addressFamily}},
				{Name: "config"},
			},
		}},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: gpb.Encoding_JSON_IETF,
	})

	if err != nil {
		t.Fatalf("failed due to %v", err)
	}
	t.Log(getResponse)

	t.Log("Verify Get outputs ")
	for _, notification := range getResponse.Notification {
		for _, update := range notification.Update {
			if update.Path != nil {
				var config TableConnectionConfig
				err = json.Unmarshal(update.Val.GetJsonIetfVal(), &config)
				if err != nil {
					t.Fatalf("Failed to unmarshal JSON: %v", err)
				}
				if config.SrcProtocol != "openconfig-policy-types:STATIC" {
					t.Fatalf("src-protocol is not set to STATIC as expected")
				}
				if config.DstProtocol != "openconfig-policy-types:ISIS" {
					t.Fatalf("dst-protocol is not set to ISIS as expected")
				}
				addressFamilyMatchString := fmt.Sprintf("openconfig-types:%s", addressFamily)
				if config.AddressFamily != addressFamilyMatchString {
					t.Fatalf("address-family is not set to %s as expected", addressFamily)
				}
				if config.DisableMetricPropagation != DisableMetricValue {
					t.Fatalf("disable-metric-propagation is not set to %v as expected", DisableMetricValue)
				}
				for _, i := range config.ImportPolicy {
					if i != RplName {
						t.Fatalf("import-policy is not set to %s as expected", RplName)
					}
				}
				t.Logf("Table Connection Details:"+
					"SRC PROTO GOT %v WANT STATIC\n"+
					"DST PRTO GOT %v WANT ISIS\n"+
					"ADDRESS FAMILY GOT %v WANT %v\n"+
					"DISABLEMETRICPROPAGATION GOT %v WANT %v\n", config.SrcProtocol,
					config.DstProtocol, config.AddressFamily, addressFamily,
					config.DisableMetricPropagation, DisableMetricValue)
			}

		}
	}
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

func IsisImportPolicyConfig(t *testing.T, dut *ondatra.DUTDevice, policyName string,
	srcProto oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE,
	dstProto oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE,
	addfmly oc.E_Types_ADDRESS_FAMILY,
	metricPropagation bool) {

	t.Log("configure redistribution under isis")
	dut = ondatra.DUT(t, "dut")
	dni := deviations.DefaultNetworkInstance(dut)

	d := oc.Root{}
	tableConn := d.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(srcProto, dstProto, addfmly)
	tableConn.SetImportPolicy([]string{policyName})
	tableConn.SetDisableMetricPropagation(metricPropagation)

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(dni).TableConnection(srcProto,
		dstProto, addfmly).Config(), tableConn)

}

func configureRoutePolicy(ipPrefixSet string, prefixSet string,
	allowConnected string, prefixSubnetRange string, statement string, rplType oc.E_RoutingPolicy_PolicyResultType) (*oc.RoutingPolicy, error) {

	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()

	if prefixSet != "" && ipPrefixSet != "" && prefixSubnetRange != "" {
		pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSet)
		pset.GetOrCreatePrefix(ipPrefixSet, prefixSubnetRange)
	}
	pdef := rp.GetOrCreatePolicyDefinition(allowConnected)

	if prefixSet != "" {
		stmt, err := pdef.AppendNewStatement(statement + "_matchPrefix")
		if err != nil {
			return nil, err
		}
		stmt.GetOrCreateActions().PolicyResult = rplType
		if prefixSet != "" {
			stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(prefixSet)
		}
	}

	stmtAll, err := pdef.AppendNewStatement(statement)
	if err != nil {
		return nil, err
	}
	stmtAll.GetOrCreateActions().PolicyResult = rplType

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

func configureDutISIS(t *testing.T) (ts *isissession.TestSession) {

	ts = isissession.MustNew(t).WithISIS()
	if err := ts.PushDUT(context.Background(), t); err != nil {
		t.Fatalf("Unable to push initial DUT config: %v", err)
	}

	return ts
}

func OTGIsisSessions(t *testing.T, ts isissession.TestSession) {
	t.Helper()

	isisPath := isissession.ISISPath(ts.DUT)
	t.Log(isisPath)

	ts.ATE = ondatra.ATE(t, "ate")
	//otg := ts.ATE.OTG()
	//ts.PushAndStart(t)
	//ts.MustAdjacency(t)

	t.Log(ts)
	t.Log("matthew ts is above")
}

func TestStaticToISISRedistribution(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")

	top := gosnappi.NewConfig()
	//devs := configureOTG(t, ate, top)
	//p1Dut := dut.Port(t, "port1")
	//p2Dut := dut.Port(t, "port2")

	t.Run("Initial Setup", func(t *testing.T) {

		t.Run("Configure Static Route on DUT", func(t *testing.T) {
			ipv4Mask := strconv.FormatUint(uint64(v4RoutePrefix), 10)
			ipv6Mask := strconv.FormatUint(uint64(v6RoutePrefix), 10)

			configureStaticRoute(t, dut, v4Route, ipv4Mask, 40, 104,
				v6Route, ipv6Mask, 60, 106)
		})

		t.Run("Configure ISIS on DUT", func(t *testing.T) {

			configureDutISIS(t)
			//OTGIsisSessions(t, *ts)
			//otg := ts.ATE.OTG()

			//t.Logf(otg.String())

			//globalTS = *ts

		})

		t.Run("OTG Configuration", func(t *testing.T) {

			//configureOTGFlows(t)
			//ate.OTG().PushConfig(t, top)
			//ate.OTG().StopProtocols(t)
			//defer ate.OTG().StopProtocols(t)
			//otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
			//otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
			t.Log(top.String())
		})

	})

	cases := []struct {
		desc                      string
		policyMetric              string
		policyLevel               string
		policyStmtType            oc.E_RoutingPolicy_PolicyResultType
		metricPropogation         bool
		protoSrc                  oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE
		protoDst                  oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE
		protoAf                   oc.E_Types_ADDRESS_FAMILY
		importPolicyConfig        bool
		importPolicyVerify        bool
		defaultImportPolicyConfig bool
		defaultImportPolicyVerify bool
		rplPrefixMatch            string
		PrefixSet                 string
		RplName                   string
		prefixMatchMask           string
		RplStatement              string
	}{{
		desc: "RT-2.12.1: Redistribute IPv4 static route to IS-IS " +
			"with metric propogation diabled",
		metricPropogation:         false,
		protoAf:                   oc.Types_ADDRESS_FAMILY_IPV4,
		defaultImportPolicyConfig: true,
		RplName:                   "DEFAULT-POLICY-PASS-ALL-V4",
		RplStatement:              "PASS-ALL",
		policyStmtType:            oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	}, {
		desc:                      "RT-2.12.2: Redistribute IPv4 static route to IS-IS with metric propogation enabled",
		metricPropogation:         false,
		protoAf:                   oc.Types_ADDRESS_FAMILY_IPV6,
		defaultImportPolicyConfig: true,
		RplName:                   "DEFAULT-POLICY-PASS-ALL-V6",
		RplStatement:              "PASS-ALL",
		policyStmtType:            oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	}, {
		desc:                      "RT-2.12.3: Redistribute IPv6 static route to IS-IS with metric propogation diabled",
		metricPropogation:         true,
		protoAf:                   oc.Types_ADDRESS_FAMILY_IPV4,
		defaultImportPolicyConfig: true,
		RplName:                   "DEFAULT-POLICY-PASS-ALL-V4",
		RplStatement:              "PASS-ALL",
		policyStmtType:            oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	}, {
		desc:                      "RT-2.12.4: Redistribute IPv6 static route to IS-IS with metric propogation enabled",
		metricPropogation:         true,
		protoAf:                   oc.Types_ADDRESS_FAMILY_IPV6,
		defaultImportPolicyConfig: true,
		RplName:                   "DEFAULT-POLICY-PASS-ALL-V6",
		RplStatement:              "PASS-ALL",
		policyStmtType:            oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	}, {
		desc:                      "RT-2.12.5: Redistribute IPv4 and IPv6 static route to IS-IS with default-import-policy set to reject",
		metricPropogation:         false,
		protoAf:                   oc.Types_ADDRESS_FAMILY_IPV4,
		defaultImportPolicyConfig: true,
		RplName:                   "DEFAULT-POLICY-PASS-ALL-V4",
		RplStatement:              "PASS-ALL",
		policyStmtType:            oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE,
	}, {
		desc:               "RT-2.12.6: Redistribute IPv4 static route to IS-IS matching a prefix using a route-policy",
		importPolicyConfig: true,
		protoAf:            oc.Types_ADDRESS_FAMILY_IPV4,
		rplPrefixMatch:     v4Route,
		PrefixSet:          v4PrefixSet,
		RplName:            v4RoutePolicy,
		prefixMatchMask:    prefixMatch,
		RplStatement:       v4Statement,
		metricPropogation:  true,
		policyStmtType:     oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	}, {
		desc: "RT-2.12.7: Redistribute IPv4 static route to IS-IS matching a tag",
	}, {
		desc:               "RT-2.12.8: Redistribute IPv6 static route to IS-IS matching a prefix using a route-policy",
		importPolicyConfig: true,
		protoAf:            oc.Types_ADDRESS_FAMILY_IPV6,
		rplPrefixMatch:     v6Route,
		PrefixSet:          v6PrefixSet,
		RplName:            v6RoutePolicy,
		prefixMatchMask:    prefixMatch,
		RplStatement:       v6Statement,
		policyStmtType:     oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	}, {
		desc:               "RT-2.12.9: Redistribute IPv6 static route to IS-IS matching a prefix using a route-policy",
		importPolicyConfig: true,
		protoAf:            oc.Types_ADDRESS_FAMILY_IPV4,
		rplPrefixMatch:     v6Route,
		PrefixSet:          v6PrefixSet,
		RplName:            v6RoutePolicy,
		prefixMatchMask:    prefixMatch,
		RplStatement:       v4Statement,
		metricPropogation:  true,
		policyStmtType:     oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	}}

	for _, tc := range cases {
		dni := deviations.DefaultNetworkInstance(dut)

		t.Run(tc.desc, func(t *testing.T) {

			if tc.defaultImportPolicyConfig {
				t.Run(fmt.Sprintf("Config Default Policy Type %s", tc.policyStmtType.String()), func(t *testing.T) {

					rpl, err := configureRoutePolicy(tc.rplPrefixMatch, tc.PrefixSet,
						tc.RplName, tc.prefixMatchMask, tc.RplStatement, tc.policyStmtType)
					if err != nil {
						fmt.Println("Error configuring route policy:", err)
						return
					}
					gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)

				})
				t.Run(fmt.Sprintf("Attach RPL %v Type %v to ISIS %v", tc.RplName, tc.policyStmtType.String(), dni), func(t *testing.T) {
					IsisImportPolicyConfig(t, dut, tc.RplName, protoSrc, protoDst, tc.protoAf, tc.metricPropogation)
				})

				t.Run(fmt.Sprintf("Verify RPL %v Attributes", tc.RplName), func(t *testing.T) {
					getAndVerifyIsisImportPolicy(t, dut, tc.metricPropogation, tc.RplName, tc.protoAf.String())
				})

				t.Run(fmt.Sprintf("Verify route if %s route is learned on OTG", tc.protoAf), func(t *testing.T) {

					configuredMetric := uint32(104)
					_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("atePort1.ISIS").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().PrefixAny().Metric().State(), time.Minute, func(v *ygnmi.Value[uint32]) bool {
						metric, present := v.Val()
						if present {
							if metric == configuredMetric {
								return true
							}
						}
						return false
					}).Await(t)

					metricInReceivedLsp := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().IsisRouter("atePort1.ISIS").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().PrefixAny().Metric().State())[0]
					if !ok {
						t.Fatalf("Metric not matched. Expected %d got %d ", configuredMetric, metricInReceivedLsp)
					}

				})

			}

			if tc.importPolicyConfig {
				t.Run(fmt.Sprintf("Config Import Policy Type %v", tc.policyStmtType.String()), func(t *testing.T) {

					t.Run(fmt.Sprintf("Config %v Route-Policy", tc.protoAf), func(t *testing.T) {
						rpl, err := configureRoutePolicy(tc.rplPrefixMatch, tc.PrefixSet, tc.RplName,
							tc.prefixMatchMask, tc.RplStatement, tc.policyStmtType)
						if err != nil {
							t.Fatalf("Failed to configure Route Policy: %v", err)
						}
						gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)
						t.Logf("Matthew update is done")

					})
					t.Run(fmt.Sprintf("Attach RPL %v To ISIS", tc.RplName), func(t *testing.T) {
						IsisImportPolicyConfig(t, dut, tc.RplName, protoSrc, protoDst, tc.protoAf, tc.metricPropogation)
					})

					t.Run(fmt.Sprintf("Verify RPL %v Attributes", tc.RplName), func(t *testing.T) {
						getAndVerifyIsisImportPolicy(t, dut, tc.metricPropogation, tc.RplName, tc.protoAf.String())
					})

					t.Run(fmt.Sprintf("Verify Route on OTG"), func(t *testing.T) {

						//configuredMetric := uint32(100)
						//_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("atePort1.ISIS").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().PrefixAny().Metric().State(), time.Minute, func(v *ygnmi.Value[uint32]) bool {
						//	metric, present := v.Val()
						//	if present {
						//		if metric == configuredMetric {
						//			return true
						//		}
						//	}
						//	return false
						//}).Await(t)
						//
						//metricInReceivedLsp := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().IsisRouter("atePort1.ISIS").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().PrefixAny().Metric().State())[0]
						//if !ok {
						//	t.Fatalf("Metric not matched. Expected %d got %d ", configuredMetric, metricInReceivedLsp)
						//}

					})

				})
			}
		})
	}
}
