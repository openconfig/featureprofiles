package static_route_isis_redistribution_test_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"

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
	lossTolerance   = float64(1)
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	isisInstance    = "DEFAULT"
	ateAreaAddr     = "49.0002"
	ate1SysID       = "640000000001"
	v4Route         = "192.168.10.0"
	v4TrafficStart  = "192.168.10.1"
	v4RoutePrefix   = uint32(24)
	v6Route         = "2024:db8:128:128::"
	v6TrafficStart  = "2024:db8:128:128::1"
	v6RoutePrefix   = uint32(64)
	dp2v4Route      = "192.168.1.4"
	dp2v4Prefix     = uint32(30)
	dp2v6Route      = "2001:DB8::0"
	dp2v6Prefix     = uint32(126)
	v4Flow          = "v4Flow"
	v6Flow          = "v6Flow"
	trafficDuration = 30 * time.Second
	prefixMatch     = "exact"
	v4RoutePolicy   = "route-policy-v4"
	v4Statement     = "statement-v4"
	v4PrefixSet     = "prefix-set-v4"
	v6RoutePolicy   = "route-policy-v6"
	v6Statement     = "statement-v6"
	v6PrefixSet     = "prefix-set-v6"
	protoSrc        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC
	protoDst        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
)

var (
	advertisedIPv4 = ipAddr{address: dp2v4Route, prefix: dp2v4Prefix}
	advertisedIPv6 = ipAddr{address: dp2v6Route, prefix: dp2v6Prefix}

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

func isisImportPolicyConfig(t *testing.T, dut *ondatra.DUTDevice, policyName string,
	srcProto oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE,
	dstProto oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE,
	addfmly oc.E_Types_ADDRESS_FAMILY,
	metricPropagation bool) {

	t.Log("configure redistribution under isis")

	dni := deviations.DefaultNetworkInstance(dut)

	d := oc.Root{}
	tableConn := d.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(srcProto, dstProto, addfmly)
	tableConn.SetImportPolicy([]string{policyName})
	tableConn.SetDisableMetricPropagation(metricPropagation)

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(dni).TableConnection(srcProto,
		dstProto, addfmly).Config(), tableConn)

}

func configureRoutePolicy(ipPrefixSet string, prefixSet string,
	rplName string, prefixSubnetRange string, statement string,
	rplType oc.E_RoutingPolicy_PolicyResultType, tagSetName string, tagValue oc.UnionUint32) (*oc.RoutingPolicy, error) {

	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()

	pdef := rp.GetOrCreatePolicyDefinition(rplName)

	// Condition for prefix set configuration
	if prefixSet != "" && ipPrefixSet != "" && prefixSubnetRange != "" {
		pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSet)
		pset.GetOrCreatePrefix(ipPrefixSet, prefixSubnetRange)
	}

	// Create a common statement. This can be adjusted based on unique requirements.
	stmt, err := pdef.AppendNewStatement(statement)
	if err != nil {
		return nil, err
	}
	stmt.GetOrCreateActions().SetPolicyResult(rplType)

	// Condition for tag set configuration
	if tagSetName != "" {
		// Create or get the tag set and set its value.
		tagSet := rp.GetOrCreateDefinedSets().GetOrCreateTagSet(tagSetName)
		tagSet.SetTagValue([]oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{tagValue})

		// Assuming conditions specific to tag set need to be set on the common statement.
		stmt.GetOrCreateConditions().GetOrCreateMatchTagSet().SetTagSet(tagSetName)
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

	staticRoute1 := ipv4Route + "/" + ipv4Mask
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
	nh2.NextHop = oc.UnionString(atePort2.IPv6)
	nh2.Metric = ygot.Uint32(metricValueV6)

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(
		oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		deviations.StaticProtocolName(dut)).Config(),
		static)
}

func configureOTGFlows(t *testing.T,
	top gosnappi.Config,
	devs []gosnappi.Device) {
	t.Helper()

	otgP1 := devs[0]
	otgP2 := devs[1]

	srcV4 := otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	srcV6 := otgP1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	dst1V4 := otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst1V6 := otgP2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	v4F := top.Flows().Add()
	v4F.SetName(v4Flow).Metrics().SetEnable(true)
	v4F.TxRx().Device().SetTxNames([]string{srcV4.Name()}).SetRxNames([]string{dst1V4.Name()})

	v4FEth := v4F.Packet().Add().Ethernet()
	v4FEth.Src().SetValue(atePort1.MAC)

	v4FIp := v4F.Packet().Add().Ipv4()
	v4FIp.Src().SetValue(srcV4.Address())
	v4FIp.Dst().Increment().SetStart(v4TrafficStart).SetCount(254)

	eth := v4F.EgressPacket().Add().Ethernet()
	ethTag := eth.Dst().MetricTags().Add()
	ethTag.SetName("MACTrackingv4").SetOffset(36).SetLength(12)

	v6F := top.Flows().Add()
	v6F.SetName(v6Flow).Metrics().SetEnable(true)
	v6F.TxRx().Device().SetTxNames([]string{srcV6.Name()}).SetRxNames([]string{dst1V6.Name()})

	v6FEth := v6F.Packet().Add().Ethernet()
	v6FEth.Src().SetValue(atePort1.MAC)

	v6FIP := v6F.Packet().Add().Ipv6()
	v6FIP.Src().SetValue(srcV6.Address())
	v6FIP.Dst().Increment().SetStart(v6TrafficStart).SetCount(1)

	eth = v6F.EgressPacket().Add().Ethernet()
	ethTag = eth.Dst().MetricTags().Add()
	ethTag.SetName("MACTrackingv6").SetOffset(36).SetLength(12)

}

func awaitISISAdjacency(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, isisName string) error {
	t.Helper()
	isis := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).Isis()
	intf := isis.Interface(p.Name())
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		intf = isis.Interface(p.Name() + ".0")
	}
	query := intf.Level(2).AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(v *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
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

func advertiseRoutesWithISIS(t *testing.T, devs []gosnappi.Device) {
	t.Helper()
	otgP1 := devs[0]

	dev1ISIS := otgP1.Isis().SetSystemId(ate1SysID).SetName(otgP1.Name() + ".ISIS")
	dev1ISIS.Basic().SetHostname(dev1ISIS.Name()).SetLearnedLspFilter(true)
	dev1ISIS.Advanced().SetAreaAddresses([]string{strings.Replace(ateAreaAddr, ".", "", -1)})
	dev1IsisInt := dev1ISIS.Interfaces().Add().
		SetEthName(otgP1.Ethernets().Items()[0].Name()).SetName("dev1IsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	dev1IsisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	// configure emulated network params
	net2v4 := otgP1.Isis().V4Routes().Add().SetName("v4-isisNet-dev1").SetLinkMetric(10)
	net2v4.Addresses().Add().SetAddress(advertisedIPv4.address).SetPrefix(advertisedIPv4.prefix)
	net2v6 := otgP1.Isis().V6Routes().Add().SetName("v6-isisNet-dev1").SetLinkMetric(10)
	net2v6.Addresses().Add().SetAddress(advertisedIPv6.address).SetPrefix(advertisedIPv6.prefix)

}

func verifyRplConfig(t *testing.T, dut *ondatra.DUTDevice, tagSetName string,
	tagValue oc.UnionUint32) {

	tagSetState := gnmi.Get(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().TagSet(tagSetName).TagValue().State())
	tagNameState := gnmi.Get(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().TagSet(tagSetName).Name().State())

	setTagValue := []oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{tagValue}

	for _, value := range tagSetState {
		configuredTagValue := []oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{value}
		if setTagValue[0] == configuredTagValue[0] {
			t.Logf("Passed: setTagValue is %v and configuredTagValue is %v",
				setTagValue[0], configuredTagValue[0])
		} else {
			t.Errorf("Failed: setTagValue is %v and configuredTagValue is %v",
				setTagValue[0], configuredTagValue[0])
		}
	}
	t.Logf("verify tag name matches expected")
	if tagNameState != tagSetName {
		t.Errorf("Failed to get tag-set name got %s wanted %s", tagNameState, tagSetName)
	} else {
		t.Logf("Passed Found tag-set name got %s wanted %s", tagNameState, tagSetName)
	}
}

func TestStaticToISISRedistribution(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	devs := configureOTG(t, ate, top)
	p1Dut := dut.Port(t, "port1")
	p2Dut := dut.Port(t, "port2")

	t.Log(devs, p1Dut, p2Dut)

	t.Run("Initial Setup", func(t *testing.T) {

		t.Run("Configure Static Route on DUT", func(t *testing.T) {
			ipv4Mask := strconv.FormatUint(uint64(v4RoutePrefix), 10)
			ipv6Mask := strconv.FormatUint(uint64(v6RoutePrefix), 10)

			configureStaticRoute(t, dut, v4Route, ipv4Mask, 40, 104,
				v6Route, ipv6Mask, 60, 106)
		})

		t.Run("Configure ISIS on DUT", func(t *testing.T) {
			configureDutISIS(t)
		})

		t.Run("OTG Configuration", func(t *testing.T) {

			configureOTGFlows(t, top, devs)
			advertiseRoutesWithISIS(t, devs)
			ate.OTG().PushConfig(t, top)
			ate.OTG().StartProtocols(t)
			otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
			otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
			t.Log(top.String())

			t.Run("Await ISIS Status", func(t *testing.T) {
				if err := awaitISISAdjacency(t, dut, p1Dut, isisInstance); err != nil {
					t.Fatal(err)
				}
			})
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
		verifyTrafficStats        bool
		trafficFlows              []string
		tagSet                    string
		tagValue                  oc.UnionUint32
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
		verifyTrafficStats: true,
		trafficFlows:       []string{v4Flow},
	}, {
		desc:               "RT-2.12.7: Redistribute IPv4 static route to IS-IS matching a tag",
		importPolicyConfig: true,
		protoAf:            oc.Types_ADDRESS_FAMILY_IPV4,
		RplName:            v4RoutePolicy,
		RplStatement:       v4Statement,
		metricPropogation:  true,
		policyStmtType:     oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyTrafficStats: true,
		trafficFlows:       []string{v4Flow},
		tagSet:             "tag-set-v4",
		tagValue:           100,
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
		verifyTrafficStats: true,
		trafficFlows:       []string{v6Flow},
	}, {
		desc:               "RT-2.12.9: Redistribute IPv6 static route to IS-IS matching a prefix using a route-policy",
		importPolicyConfig: true,
		protoAf:            oc.Types_ADDRESS_FAMILY_IPV4,
		RplName:            v6RoutePolicy,
		RplStatement:       v6Statement,
		metricPropogation:  true,
		policyStmtType:     oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyTrafficStats: true,
		trafficFlows:       []string{v4Flow},
		tagSet:             "tag-set-v6",
		tagValue:           100,
	}}

	for _, tc := range cases {
		dni := deviations.DefaultNetworkInstance(dut)

		t.Run(tc.desc, func(t *testing.T) {

			if tc.defaultImportPolicyConfig {
				t.Run(fmt.Sprintf("Config Default Policy Type %s", tc.policyStmtType.String()), func(t *testing.T) {

					rpl, err := configureRoutePolicy(tc.rplPrefixMatch, tc.PrefixSet,
						tc.RplName, tc.prefixMatchMask, tc.RplStatement, tc.policyStmtType, tc.tagSet, tc.tagValue)
					if err != nil {
						fmt.Println("Error configuring route policy:", err)
						return
					}
					gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)

				})
				t.Run(fmt.Sprintf("Attach RPL %v Type %v to ISIS %v", tc.RplName, tc.policyStmtType.String(), dni), func(t *testing.T) {
					isisImportPolicyConfig(t, dut, tc.RplName, protoSrc, protoDst, tc.protoAf, tc.metricPropogation)
				})

				t.Run(fmt.Sprintf("Verify RPL %v Attributes", tc.RplName), func(t *testing.T) {
					getAndVerifyIsisImportPolicy(t, dut, tc.metricPropogation, tc.RplName, tc.protoAf.String())

					path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableConnection(
						oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
						oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
						oc.Types_ADDRESS_FAMILY_IPV4,
					)

					output := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithUseGet(),
						ygnmi.WithEncoding(gpb.Encoding_JSON_IETF)), path.State())

					t.Log(output)

				})
			}

			if tc.importPolicyConfig {
				t.Run(fmt.Sprintf("Config Import Policy Type %v", tc.policyStmtType.String()), func(t *testing.T) {

					t.Run(fmt.Sprintf("Config %v Route-Policy", tc.protoAf), func(t *testing.T) {
						rpl, err := configureRoutePolicy(tc.rplPrefixMatch, tc.PrefixSet, tc.RplName,
							tc.prefixMatchMask, tc.RplStatement, tc.policyStmtType, tc.tagSet, tc.tagValue)
						if err != nil {
							t.Fatalf("Failed to configure Route Policy: %v", err)
						}
						gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)

						if tc.tagSet != "" {
							t.Run(fmt.Sprintf("Verify Configuration for RPL %v value %v",
								tc.tagSet, tc.tagValue), func(t *testing.T) {
								verifyRplConfig(t, dut, tc.tagSet, tc.tagValue)
							})
						}

					})
					t.Run(fmt.Sprintf("Attach RPL %v To ISIS", tc.RplName), func(t *testing.T) {
						isisImportPolicyConfig(t, dut, tc.RplName, protoSrc, protoDst, tc.protoAf, tc.metricPropogation)
					})

					t.Run(fmt.Sprintf("Verify RPL %v Attributes", tc.RplName), func(t *testing.T) {
						getAndVerifyIsisImportPolicy(t, dut, tc.metricPropogation, tc.RplName, tc.protoAf.String())

						path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableConnection(
							oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
							oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
							oc.Types_ADDRESS_FAMILY_IPV4)

						output := gnmi.LookupConfig(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithUseGet(),
							ygnmi.WithEncoding(gpb.Encoding_JSON_IETF)), path.Config())

						t.Log(output)
					})

				})
			}

			if tc.verifyTrafficStats {
				t.Run(fmt.Sprintf("Verify traffic for %s", tc.trafficFlows), func(t *testing.T) {

					ate.OTG().StartTraffic(t)
					time.Sleep(trafficDuration)
					ate.OTG().StopTraffic(t)

					for _, flow := range tc.trafficFlows {
						loss := otgutils.GetFlowLossPct(t, ate.OTG(), flow, 20*time.Second)
						if loss > lossTolerance {
							t.Errorf("Traffic loss too high for flow %s", flow)
						} else {
							t.Logf("Traffic loss for flow %s is %v", flow, loss)
						}
					}
				})

			}

			t.Run("Verify Route on OTG", func(t *testing.T) {
				// TODO: Verify routes are learned on the ATE device. This is pending a fix from IXIA and OTG
				// TODO: https://github.com/open-traffic-generator/fp-testbed-cisco/issues/10#issuecomment-2015756900
				t.Skip("Skipping this due to OTG issue not learning routes.")

				configuredMetric := uint32(100)
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
		})
	}
}
