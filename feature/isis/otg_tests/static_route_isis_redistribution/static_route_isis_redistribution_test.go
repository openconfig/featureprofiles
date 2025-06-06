// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package static_route_isis_redistribution_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	lossTolerance   = float64(1)
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	v4Route         = "192.168.10.0"
	v4TrafficStart  = "192.168.10.1"
	v4RoutePrefix   = uint32(24)
	v6Route         = "2024:db8:128:128:0:0:0:0"
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
	v4tagSet        = "tag-set-v4"
	v4RoutePolicy   = "route-policy-v4"
	v4Statement     = "statement-v4"
	v4PrefixSet     = "prefix-set-v4"
	v6tagSet        = "tag-set-v6"
	v6RoutePolicy   = "route-policy-v6"
	v6Statement     = "statement-v6"
	v6PrefixSet     = "prefix-set-v6"
	protoSrc        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC
	protoDst        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	dummyV6         = "2001:db8::192:0:2:d"
	dummyMAC        = "00:1A:11:00:0A:BC"
	tagValue        = 100
	v4Metric        = uint32(104)
	v6Metric        = uint32(106)
	isisMetric      = uint32(1000)
	metricZero      = uint32(0)
	shouldBePresent = true
	V4tagValue      = 40
	V6tagValue      = 60
)

var (
	advertisedIPv4 = ipAddr{address: dp2v4Route, prefix: dp2v4Prefix}
	advertisedIPv6 = ipAddr{address: dp2v6Route, prefix: dp2v6Prefix}
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
	t.Log("Received parameters of table connections")

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
				if !deviations.SkipSettingDisableMetricPropagation(dut) {
					if config.DisableMetricPropagation != DisableMetricValue {
						t.Fatalf("disable-metric-propagation is not set to %v as expected", DisableMetricValue)
					}
				}
				for _, i := range config.ImportPolicy {
					if i != RplName {
						t.Fatalf("import-policy is not set to %s as expected", RplName)
					}
				}
				t.Logf("Table Connection Details:\n"+
					"SRC PROTO GOT %v WANT STATIC\n"+
					"DST PROTO GOT %v WANT ISIS\n"+
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
	metricPropagation bool, operation string) {

	t.Log("configure redistribution under isis")

	dni := deviations.DefaultNetworkInstance(dut)

	batchSet := &gnmi.SetBatch{}
	d := oc.Root{}
	if operation == "set" {
		tableConn := d.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(srcProto, dstProto, addfmly)
		tableConn.SetImportPolicy([]string{policyName})
		if !deviations.SkipSettingDisableMetricPropagation(dut) {
			tableConn.SetDisableMetricPropagation(metricPropagation)
		}
		if deviations.EnableTableConnections(dut) {
			fptest.ConfigEnableTbNative(t, dut)
		}
		gnmi.BatchReplace(batchSet, gnmi.OC().NetworkInstance(dni).TableConnection(srcProto, dstProto, addfmly).Config(), tableConn)

		if deviations.SamePolicyAttachedToAllAfis(dut) {
			if addfmly == oc.Types_ADDRESS_FAMILY_IPV4 {
				addfmly = oc.Types_ADDRESS_FAMILY_IPV6
			} else {
				addfmly = oc.Types_ADDRESS_FAMILY_IPV4
			}
			tableConn1 := d.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(srcProto, dstProto, addfmly)
			tableConn1.SetImportPolicy([]string{policyName})
			if !deviations.SkipSettingDisableMetricPropagation(dut) {
				tableConn1.SetDisableMetricPropagation(metricPropagation)
			}
			gnmi.BatchReplace(batchSet, gnmi.OC().NetworkInstance(dni).TableConnection(srcProto, dstProto, addfmly).Config(), tableConn1)
		}

		batchSet.Set(t, dut)
	} else if operation == "delete" {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(dni).TableConnection(srcProto, dstProto, addfmly).Config())
	}
}

func configureRoutePolicy(dut *ondatra.DUTDevice, rplName string, statement string, prefixSetCond, tagSetCond bool,
	rplType oc.E_RoutingPolicy_PolicyResultType) (*oc.RoutingPolicy, error) {

	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(rplName)

	if prefixSetCond {
		// Condition for prefix set configuration
		stmt1, err := pdef.AppendNewStatement(v4Statement)
		if err != nil {
			return nil, err
		}
		v4Prefix := v4Route + "/" + strconv.FormatUint(uint64(v4RoutePrefix), 10)
		pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v4PrefixSet)
		pset.GetOrCreatePrefix(v4Prefix, prefixMatch)
		if !deviations.SkipPrefixSetMode(dut) {
			pset.SetMode(oc.PrefixSet_Mode_IPV4)
		}
		stmt1.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v4PrefixSet)
		stmt1.GetOrCreateActions().SetPolicyResult(rplType)
		stmt1.GetOrCreateActions().GetOrCreateIsisActions().SetSetLevel(2)
		stmt1.GetOrCreateActions().GetOrCreateIsisActions().SetSetMetricStyleType(oc.IsisPolicy_MetricStyle_WIDE_METRIC)
		stmt1.GetOrCreateActions().GetOrCreateIsisActions().SetSetMetric(isisMetric)

		stmt2, err := pdef.AppendNewStatement(v6Statement)
		if err != nil {
			return nil, err
		}
		v6Prefix := v6Route + "/" + strconv.FormatUint(uint64(v6RoutePrefix), 10)
		pset = rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v6PrefixSet)
		pset.GetOrCreatePrefix(v6Prefix, prefixMatch)
		if !deviations.SkipPrefixSetMode(dut) {
			pset.SetMode(oc.PrefixSet_Mode_IPV6)
		}
		stmt2.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v6PrefixSet)
		stmt2.GetOrCreateActions().SetPolicyResult(rplType)
		stmt2.GetOrCreateActions().GetOrCreateIsisActions().SetSetLevel(2)
		stmt2.GetOrCreateActions().GetOrCreateIsisActions().SetSetMetricStyleType(oc.IsisPolicy_MetricStyle_WIDE_METRIC)
		stmt2.GetOrCreateActions().GetOrCreateIsisActions().SetSetMetric(isisMetric)
	} else if tagSetCond {
		// Condition for tag set configuration
		stmt1, err := pdef.AppendNewStatement(v4Statement)
		if err != nil {
			return nil, err
		}
		tagSet1 := rp.GetOrCreateDefinedSets().GetOrCreateTagSet(v4tagSet)
		tagSet1.SetTagValue([]oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{oc.UnionUint32(tagValue)})
		stmt1.GetOrCreateConditions().GetOrCreateMatchTagSet().SetTagSet(v4tagSet)
		stmt1.GetOrCreateActions().SetPolicyResult(rplType)

		stmt2, err := pdef.AppendNewStatement(v6Statement)
		if err != nil {
			return nil, err
		}
		tagSet2 := rp.GetOrCreateDefinedSets().GetOrCreateTagSet(v6tagSet)
		tagSet2.SetTagValue([]oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{oc.UnionUint32(tagValue)})
		stmt2.GetOrCreateConditions().GetOrCreateMatchTagSet().SetTagSet(v6tagSet)
		stmt2.GetOrCreateActions().SetPolicyResult(rplType)
	} else {
		// Create a common statement
		stmt, err := pdef.AppendNewStatement(statement)
		if err != nil {
			return nil, err
		}
		stmt.GetOrCreateActions().SetPolicyResult(rplType)
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
	nh.NextHop = oc.UnionString(isissession.ATEISISAttrs.IPv4)
	nh.Metric = ygot.Uint32(metricValueV4)

	sr2 := static.GetOrCreateStatic(staticRoute2)
	sr2.SetTag, _ = sr.To_NetworkInstance_Protocol_Static_SetTag_Union(tagValueV6)
	nh2 := sr2.GetOrCreateNextHop("0")
	nh2.NextHop = oc.UnionString(isissession.ATEISISAttrs.IPv6)
	nh2.Metric = ygot.Uint32(metricValueV6)

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(
		oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		deviations.StaticProtocolName(dut)).Config(),
		static)
}

func configureOTGFlows(t *testing.T, top gosnappi.Config, ts *isissession.TestSession) {
	t.Helper()

	srcV4 := ts.ATEIntf2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	srcV6 := ts.ATEIntf2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	dst1V4 := ts.ATEIntf1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dst1V6 := ts.ATEIntf1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	v4F := top.Flows().Add()
	v4F.SetName(v4Flow).Metrics().SetEnable(true)
	v4F.TxRx().Device().SetTxNames([]string{srcV4.Name()}).SetRxNames([]string{dst1V4.Name()})

	v4FEth := v4F.Packet().Add().Ethernet()
	v4FEth.Src().SetValue(isissession.ATETrafficAttrs.MAC)

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
	v6FEth.Src().SetValue(isissession.ATETrafficAttrs.MAC)

	v6FIP := v6F.Packet().Add().Ipv6()
	v6FIP.Src().SetValue(srcV6.Address())
	v6FIP.Dst().Increment().SetStart(v6TrafficStart).SetCount(1)

	eth = v6F.EgressPacket().Add().Ethernet()
	ethTag = eth.Dst().MetricTags().Add()
	ethTag.SetName("MACTrackingv6").SetOffset(36).SetLength(12)
}

func advertiseRoutesWithISIS(t *testing.T, ts *isissession.TestSession) {
	t.Helper()

	// configure emulated network params
	net2v4 := ts.ATEIntf1.Isis().V4Routes().Add().SetName("v4-isisNet-dev1").SetLinkMetric(10)
	net2v4.Addresses().Add().SetAddress(advertisedIPv4.address).SetPrefix(advertisedIPv4.prefix)
	net2v6 := ts.ATEIntf1.Isis().V6Routes().Add().SetName("v6-isisNet-dev1").SetLinkMetric(10)
	net2v6.Addresses().Add().SetAddress(advertisedIPv6.address).SetPrefix(advertisedIPv6.prefix)
}

func verifyRplConfig(t *testing.T, dut *ondatra.DUTDevice, tagSetName string, tagValue oc.UnionUint32) {
	tagSetState := gnmi.Get(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().TagSet(tagSetName).TagValue().State())
	tagNameState := gnmi.Get(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().TagSet(tagSetName).Name().State())

	setTagValue := []oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{tagValue}

	for _, value := range tagSetState {
		configuredTagValue := []oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{value}
		if setTagValue[0] == configuredTagValue[0] {
			t.Logf("Passed: setTagValue is %v and configuredTagValue is %v", setTagValue[0], configuredTagValue[0])
		} else {
			t.Errorf("Failed: setTagValue is %v and configuredTagValue is %v", setTagValue[0], configuredTagValue[0])
		}
	}
	t.Logf("verify tag name matches expected")
	if tagNameState != tagSetName {
		t.Errorf("Failed to get tag-set name got %s wanted %s", tagNameState, tagSetName)
	} else {
		t.Logf("Passed Found tag-set name got %s wanted %s", tagNameState, tagSetName)
	}
}

func verifyPrefix(t *testing.T, ts *isissession.TestSession, shouldBePresent bool) {

	t.Run("Verify Route on OTG", func(t *testing.T) {
		_, ok := gnmi.WatchAll(t, ts.ATE.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().Prefix(v4Route).State(), time.Minute, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_ExtendedIpv4Reachability_Prefix]) bool {
			prefix, present := v.Val()
			if !shouldBePresent {
				return !present
			}
			return present && prefix.GetPrefix() == v4Route
		}).Await(t)
		if shouldBePresent {
			if !ok {
				t.Errorf("Prefix not found, want: %s", v4Route)
			}
		} else {
			if ok {
				t.Errorf("Prefix found, not want: %s", v4Route)
			}
		}
	})
}

func verifyV6Prefix(t *testing.T, ts *isissession.TestSession, shouldBePresent bool) {

	t.Run("Verify Route on OTG", func(t *testing.T) {
		_, ok := gnmi.WatchAll(t, ts.ATE.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().Ipv6Reachability().Prefix(v6Route).State(), 30*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_Ipv6Reachability_Prefix]) bool {
			prefix, present := v.Val()
			return present && prefix.GetPrefix() == v6Route
		}).Await(t)
		if shouldBePresent {
			if !ok {
				t.Errorf("Prefix not found, want: %s", v6Route)
			}
		} else {
			if ok {
				t.Errorf("Prefix found, not want: %s", v6Route)
			}
		}
	})
}

func verifyPrefixMetric(t *testing.T, ts *isissession.TestSession, expectedMetric uint32) {

	t.Run("Verify Route Metric on OTG", func(t *testing.T) {
		_, ok := gnmi.WatchAll(t, ts.ATE.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().Prefix(v4Route).Metric().State(), 30*time.Second, func(v *ygnmi.Value[uint32]) bool {
			if !v.IsPresent() {
				return false
			}
			if metricInReceivedLsp, _ := v.Val(); metricInReceivedLsp == expectedMetric {
				t.Logf("Metric matched for v4 route, got: %d & want: %d", metricInReceivedLsp, expectedMetric)
				return true
			}
			return false
		}).Await(t)
		if !ok {
			t.Error("ERROR: Metrics mismatched for v4 route")
		}
	})
}

func verifyV6PrefixMetric(t *testing.T, ts *isissession.TestSession, expectedMetric uint32) {

	t.Run("Verify Route Metric on OTG", func(t *testing.T) {
		_, ok := gnmi.WatchAll(t, ts.ATE.OTG(), gnmi.OTG().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().Ipv6Reachability().Prefix(v6Route).Metric().State(), 30*time.Second, func(v *ygnmi.Value[uint32]) bool {
			if !v.IsPresent() {
				return false
			}
			if metricInReceivedLsp, _ := v.Val(); metricInReceivedLsp == expectedMetric {
				t.Logf("Metric matched for v6 route, got: %d & want: %d", metricInReceivedLsp, expectedMetric)
				return true
			}
			return false
		}).Await(t)
		if !ok {
			t.Error("ERROR: Metrics mismatched for v6 route")
		}
	})
}

func verifyMatchingPrefixWithoutMetricPropagation(t *testing.T, ts *isissession.TestSession) {

	verifyPrefix(t, ts, shouldBePresent)
	if !deviations.SkipSettingDisableMetricPropagation(ts.DUT) {
		verifyPrefixMetric(t, ts, metricZero)
	}
}

func verifyMatchingV6PrefixWithoutMetricPropagation(t *testing.T, ts *isissession.TestSession) {

	verifyV6Prefix(t, ts, shouldBePresent)
	if !deviations.SkipSettingDisableMetricPropagation(ts.DUT) {
		verifyV6PrefixMetric(t, ts, metricZero)
	}
}

func verifyMatchingPrefixWithMetricPropagation(t *testing.T, ts *isissession.TestSession) {

	verifyPrefix(t, ts, shouldBePresent)
	verifyPrefixMetric(t, ts, v4Metric)
}

func verifyMatchingV6PrefixWithMetricPropagation(t *testing.T, ts *isissession.TestSession) {

	verifyV6Prefix(t, ts, shouldBePresent)
	verifyV6PrefixMetric(t, ts, v6Metric)
}

func verifyNonMatchingPrefix(t *testing.T, ts *isissession.TestSession) {

	verifyPrefix(t, ts, !shouldBePresent)
}

func verifyMatchingPrefixWithMetricPropagationWithRoutePolicy(t *testing.T, ts *isissession.TestSession) {

	verifyPrefix(t, ts, shouldBePresent)
	verifyPrefixMetric(t, ts, isisMetric)
}

func verifyMatchingV6PrefixWithMetricPropagationWithRoutePolicy(t *testing.T, ts *isissession.TestSession) {

	verifyV6Prefix(t, ts, shouldBePresent)
	verifyV6PrefixMetric(t, ts, isisMetric)
}

func verifyMatchingPrefixWithTag(t *testing.T, ts *isissession.TestSession) {

	verifyPrefix(t, ts, !shouldBePresent)

	t.Run("Configuring correct tag value", func(t *testing.T) {
		gnmi.Replace(t, ts.DUT, gnmi.OC().RoutingPolicy().DefinedSets().TagSet(v4tagSet).TagValue().Config(), []oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{oc.UnionUint32(V4tagValue)})
	})
	if !deviations.RoutingPolicyTagSetEmbedded(ts.DUT) {
		t.Run("Verify Configuration for RPL TagSet in V4", func(t *testing.T) {
			verifyRplConfig(t, ts.DUT, v4tagSet, oc.UnionUint32(V4tagValue))
		})
	}
	verifyPrefix(t, ts, shouldBePresent)
}

func verifyMatchingV6PrefixWithTag(t *testing.T, ts *isissession.TestSession) {

	verifyV6Prefix(t, ts, !shouldBePresent)

	t.Run("Configuring correct tag value", func(t *testing.T) {
		gnmi.Replace(t, ts.DUT, gnmi.OC().RoutingPolicy().DefinedSets().TagSet(v6tagSet).TagValue().Config(), []oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{oc.UnionUint32(V6tagValue)})
	})
	if !deviations.RoutingPolicyTagSetEmbedded(ts.DUT) {
		t.Run("Verify Configuration for RPL TagSet in V6", func(t *testing.T) {
			verifyRplConfig(t, ts.DUT, v6tagSet, oc.UnionUint32(V6tagValue))
		})
	}
	verifyV6Prefix(t, ts, shouldBePresent)
}

func TestStaticToISISRedistribution(t *testing.T) {
	var ts *isissession.TestSession

	t.Run("Initial Setup", func(t *testing.T) {
		t.Run("Configure ISIS on DUT", func(t *testing.T) {
			ts = isissession.MustNew(t).WithISIS()
			if err := ts.PushDUT(context.Background(), t); err != nil {
				t.Fatalf("Unable to push initial DUT config: %v", err)
			}
		})

		t.Run("Configure Static Route on DUT", func(t *testing.T) {
			ipv4Mask := strconv.FormatUint(uint64(v4RoutePrefix), 10)
			ipv6Mask := strconv.FormatUint(uint64(v6RoutePrefix), 10)
			configureStaticRoute(t, ts.DUT, v4Route, ipv4Mask, 40, 104, v6Route, ipv6Mask, 60, 106)
		})

		t.Run("OTG Configuration", func(t *testing.T) {
			configureOTGFlows(t, ts.ATETop, ts)
			advertiseRoutesWithISIS(t, ts)
			ts.PushAndStart(t)
			adj := ts.MustAdjacency(t)
			t.Logf("ISIS adjacency established with ID: %s", adj)

			otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv4")
			otgutils.WaitForARP(t, ts.ATE.OTG(), ts.ATETop, "IPv6")
		})
	})

	cases := []struct {
		desc               string
		policyStmtType     oc.E_RoutingPolicy_PolicyResultType
		metricPropogation  bool
		protoAf            oc.E_Types_ADDRESS_FAMILY
		RplName            string
		RplStatement       string
		verifyTrafficStats bool
		verifyRouteFunc    func(t *testing.T, ts *isissession.TestSession)
		trafficFlows       []string
		TagSetCondition    bool
		PrefixSetCondition bool
	}{{
		desc:              "RT-2.12.1: Redistribute IPv4 static route to IS-IS with metric propagation disabled",
		metricPropogation: true,
		protoAf:           oc.Types_ADDRESS_FAMILY_IPV4,
		RplName:           "DEFAULT-POLICY-PASS-ALL-V4",
		RplStatement:      "PASS-ALL",
		policyStmtType:    oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyRouteFunc:   verifyMatchingPrefixWithoutMetricPropagation,
	}, {
		desc:              "RT-2.12.2: Redistribute IPv6 static route to IS-IS with metric propagation disabled",
		metricPropogation: true,
		protoAf:           oc.Types_ADDRESS_FAMILY_IPV6,
		RplName:           "DEFAULT-POLICY-PASS-ALL-V6",
		RplStatement:      "PASS-ALL",
		policyStmtType:    oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyRouteFunc:   verifyMatchingV6PrefixWithoutMetricPropagation,
	}, {
		desc:              "RT-2.12.3: Redistribute IPv4 static route to IS-IS with metric propagation enabled",
		metricPropogation: false,
		protoAf:           oc.Types_ADDRESS_FAMILY_IPV4,
		RplName:           "DEFAULT-POLICY-PASS-ALL-V4",
		RplStatement:      "PASS-ALL",
		policyStmtType:    oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyRouteFunc:   verifyMatchingPrefixWithMetricPropagation,
	}, {
		desc:              "RT-2.12.4: Redistribute IPv6 static route to IS-IS with metric propogation enabled",
		metricPropogation: false,
		protoAf:           oc.Types_ADDRESS_FAMILY_IPV6,
		RplName:           "DEFAULT-POLICY-PASS-ALL-V6",
		RplStatement:      "PASS-ALL",
		policyStmtType:    oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyRouteFunc:   verifyMatchingV6PrefixWithMetricPropagation,
	}, {
		desc:              "RT-2.12.5: Redistribute IPv4 and IPv6 static route to IS-IS with default-import-policy set to reject",
		metricPropogation: false,
		protoAf:           oc.Types_ADDRESS_FAMILY_IPV4,
		RplName:           "DEFAULT-POLICY-PASS-ALL-V4",
		RplStatement:      "PASS-ALL",
		policyStmtType:    oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE,
		verifyRouteFunc:   verifyNonMatchingPrefix,
	}, {
		desc:               "RT-2.12.6: Redistribute IPv4 static route to IS-IS matching a prefix using a route-policy",
		protoAf:            oc.Types_ADDRESS_FAMILY_IPV4,
		RplName:            v4RoutePolicy,
		metricPropogation:  false,
		policyStmtType:     oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyTrafficStats: true,
		trafficFlows:       []string{v4Flow},
		PrefixSetCondition: true,
		verifyRouteFunc:    verifyMatchingPrefixWithMetricPropagationWithRoutePolicy,
	}, {
		desc:               "RT-2.12.7: Redistribute IPv4 static route to IS-IS matching a tag",
		protoAf:            oc.Types_ADDRESS_FAMILY_IPV4,
		RplName:            v4RoutePolicy,
		metricPropogation:  false,
		policyStmtType:     oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyTrafficStats: true,
		trafficFlows:       []string{v4Flow},
		TagSetCondition:    true,
		verifyRouteFunc:    verifyMatchingPrefixWithTag,
	}, {
		desc:               "RT-2.12.8: Redistribute IPv6 static route to IS-IS matching a prefix using a route-policy",
		protoAf:            oc.Types_ADDRESS_FAMILY_IPV6,
		RplName:            v6RoutePolicy,
		metricPropogation:  true,
		policyStmtType:     oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyTrafficStats: true,
		trafficFlows:       []string{v6Flow},
		PrefixSetCondition: true,
		verifyRouteFunc:    verifyMatchingV6PrefixWithMetricPropagationWithRoutePolicy,
	}, {
		desc:               "RT-2.12.9: Redistribute IPv6 static route to IS-IS matching a prefix using a tag",
		protoAf:            oc.Types_ADDRESS_FAMILY_IPV6,
		RplName:            v6RoutePolicy,
		metricPropogation:  false,
		policyStmtType:     oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		verifyTrafficStats: true,
		trafficFlows:       []string{v6Flow},
		TagSetCondition:    true,
		verifyRouteFunc:    verifyMatchingV6PrefixWithTag,
	}}

	for _, tc := range cases {
		if deviations.MatchTagSetConditionUnsupported(ts.DUT) && tc.TagSetCondition {
			t.Skipf("Skipping test case %s due to match tag set condition not supported", tc.desc)
		}

		dni := deviations.DefaultNetworkInstance(ts.DUT)

		t.Run(tc.desc, func(t *testing.T) {
			t.Run(fmt.Sprintf("Configure Policy Type %s", tc.policyStmtType.String()), func(t *testing.T) {
				rpl, err := configureRoutePolicy(ts.DUT, tc.RplName, tc.RplStatement, tc.PrefixSetCondition,
					tc.TagSetCondition, tc.policyStmtType)
				if err != nil {
					fmt.Println("Error configuring route policy:", err)
					return
				}
				gnmi.Replace(t, ts.DUT, gnmi.OC().RoutingPolicy().Config(), rpl)
			})

			if tc.TagSetCondition {
				if !deviations.RoutingPolicyTagSetEmbedded(ts.DUT) {
					t.Run("Verify Configuration for RPL TagSet", func(t *testing.T) {
						if tc.protoAf == oc.Types_ADDRESS_FAMILY_IPV4 {
							verifyRplConfig(t, ts.DUT, v4tagSet, oc.UnionUint32(tagValue))
						} else {
							verifyRplConfig(t, ts.DUT, v6tagSet, oc.UnionUint32(tagValue))
						}
					})
				}
			}

			t.Run(fmt.Sprintf("Attach RPL %v Type %v to ISIS %v", tc.RplName, tc.policyStmtType.String(), dni), func(t *testing.T) {
				isisImportPolicyConfig(t, ts.DUT, tc.RplName, protoSrc, protoDst, tc.protoAf, tc.metricPropogation, "set")
			})
			defer isisImportPolicyConfig(t, ts.DUT, tc.RplName, protoSrc, protoDst, tc.protoAf, tc.metricPropogation, "delete")

			t.Run(fmt.Sprintf("Verify RPL %v Attributes", tc.RplName), func(t *testing.T) {
				getAndVerifyIsisImportPolicy(t, ts.DUT, tc.metricPropogation, tc.RplName, tc.protoAf.String())
			})

			tc.verifyRouteFunc(t, ts)

			if tc.verifyTrafficStats {
				t.Run(fmt.Sprintf("Verify traffic for %s", tc.trafficFlows), func(t *testing.T) {

					ts.ATE.OTG().StartTraffic(t)
					time.Sleep(trafficDuration)
					ts.ATE.OTG().StopTraffic(t)

					for _, flow := range tc.trafficFlows {
						loss := otgutils.GetFlowLossPct(t, ts.ATE.OTG(), flow, 20*time.Second)
						if loss > lossTolerance {
							t.Errorf("Traffic loss too high for flow %s", flow)
						} else {
							t.Logf("Traffic loss for flow %s is %v", flow, loss)
						}
					}
				})
			}
		})
	}
}
