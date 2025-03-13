package union_replace_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	log "github.com/golang/glog"
	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/protobuf/encoding/prototext"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var baseConfiguration string

func baseConfig(t *testing.T, dut *ondatra.DUTDevice) string {
	runningConfig := config.CMDViaGNMI(context.Background(), t, dut, "show running-config")
	lines := strings.Split(runningConfig, "\n")
	var inTargetSection bool

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "username") ||
			strings.HasPrefix(trimmedLine, "grpc") ||
			strings.HasPrefix(trimmedLine, "interface Mgmt") {
			inTargetSection = true
		}
		if inTargetSection && trimmedLine == "!" {
			inTargetSection = false
			baseConfiguration += trimmedLine + "\n\n"
		}
		if inTargetSection {
			baseConfiguration += line + "\n"
		}
	}

	t.Log(baseConfiguration)
	return baseConfiguration

}
func validate[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T], expected T) {
	getResponse := gnmi.Get(t, dut, q)
	diff := cmp.Diff(getResponse, expected)
	if diff != "" {
		t.Errorf("Get on %v not as expected , want %v got %v \n", q, expected, getResponse)
	}

}
func bgpValidator(t *testing.T, dut *ondatra.DUTDevice) {
	gnmi.Get(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().Config())
	time.Sleep(10 * time.Second)
	gnmi.Get(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().State())

	validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().As().Config(), 65111)
	validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().As().State(), 65111)

	validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouterId().Config(), "108.170.235.207")
	validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouterId().State(), "108.170.235.207")

	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouteSelectionOptions().AlwaysCompareMed().Config(), true)
	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouteSelectionOptions().AlwaysCompareMed().State(), true)

	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouteSelectionOptions().ExternalCompareRouterId().Config(), true)
	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouteSelectionOptions().ExternalCompareRouterId().State(), true)

	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().UseMultiplePaths().Ebgp().AllowMultipleAs().Config(), true)
	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().UseMultiplePaths().Ebgp().AllowMultipleAs().State(), true)

	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)).Enabled().Config(), true)
	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)).Enabled().State(), true)

	validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)).UseMultiplePaths().Ebgp().MaximumPaths().Config(), 64)
	validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)).UseMultiplePaths().Ebgp().MaximumPaths().State(), 64)

	validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)).UseMultiplePaths().Ebgp().MaximumPaths().Config(), 64)
	validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)).UseMultiplePaths().Ebgp().MaximumPaths().State(), 64)

	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)).Enabled().Config(), true)
	validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)).Enabled().State(), true)

	validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)).UseMultiplePaths().Ebgp().MaximumPaths().Config(), 64)
	validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)).UseMultiplePaths().Ebgp().MaximumPaths().State(), 64)

	peerGroupValidation := []struct {
		name            string
		as              uint32
		keepalive       uint32
		hold            uint32
		mai             uint32
		safi            oc.E_BgpTypes_AFI_SAFI_TYPE
		safiEn          bool
		maxprefix       uint32
		warThreshold    uint32
		preventTeardown bool
		exportPolicy    string
		importpolicy    string
	}{
		{
			name:            "METRO_PR",
			as:              15169,
			keepalive:       5,
			hold:            15,
			mai:             1,
			safi:            3,
			safiEn:          true,
			maxprefix:       20000,
			warThreshold:    75,
			preventTeardown: false,
			exportPolicy:    "CX-PR-OUT",
			importpolicy:    "METRO-PR",
		},
		{
			name:            "SATELLITE6",
			as:              65516,
			keepalive:       0,
			hold:            0,
			mai:             1,
			safi:            5,
			safiEn:          true,
			maxprefix:       1000,
			warThreshold:    75,
			preventTeardown: false,
			exportPolicy:    "SATELLITE",
			importpolicy:    "CX-SATELLITE-IN",
		},
		{
			name:            "TOOLS_RECEIVERS",
			as:              65111,
			keepalive:       100,
			hold:            300,
			mai:             1,
			safi:            3,
			safiEn:          true,
			maxprefix:       1000,
			warThreshold:    80,
			preventTeardown: true,
			exportPolicy:    "SATELLITE",
			importpolicy:    "NO-ROUTES",
		},
	}
	for _, tc := range peerGroupValidation {
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).PeerGroupName().Config(), tc.name)
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).PeerGroupName().State(), tc.name)

		validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).PeerAs().Config(), tc.as)
		validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).PeerAs().State(), tc.as)

		validate[oc.E_BgpTypes_AFI_SAFI_TYPE](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).AfiSafiName().Config(), tc.safi)
		validate[oc.E_BgpTypes_AFI_SAFI_TYPE](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).AfiSafiName().State(), tc.safi)

		validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Enabled().Config(), tc.safiEn)
		validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Enabled().State(), tc.safiEn)
		if tc.safi == 3 {
			validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv4Unicast().PrefixLimit().MaxPrefixes().Config(), tc.maxprefix)
			validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv4Unicast().PrefixLimit().MaxPrefixes().State(), tc.maxprefix)

			validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv4Unicast().PrefixLimit().PreventTeardown().Config(), tc.preventTeardown)
			validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv4Unicast().PrefixLimit().PreventTeardown().State(), tc.preventTeardown)

			validate[uint8](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv4Unicast().PrefixLimit().WarningThresholdPct().Config(), uint8(tc.warThreshold))
			validate[uint8](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv4Unicast().PrefixLimit().WarningThresholdPct().State(), uint8(tc.warThreshold))

		} else {
			validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv6Unicast().PrefixLimit().MaxPrefixes().Config(), tc.maxprefix)
			validate[uint32](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv6Unicast().PrefixLimit().MaxPrefixes().State(), tc.maxprefix)

			validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv6Unicast().PrefixLimit().PreventTeardown().Config(), tc.preventTeardown)
			validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv6Unicast().PrefixLimit().PreventTeardown().State(), tc.preventTeardown)

			validate[uint8](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv6Unicast().PrefixLimit().WarningThresholdPct().Config(), uint8(tc.warThreshold))
			validate[uint8](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).Ipv6Unicast().PrefixLimit().WarningThresholdPct().State(), uint8(tc.warThreshold))

		}

		validate[[]string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).ApplyPolicy().ExportPolicy().Config(), []string{tc.exportPolicy})
		validate[[]string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).ApplyPolicy().ExportPolicy().Config(), []string{tc.exportPolicy})

		validate[[]string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).ApplyPolicy().ImportPolicy().Config(), []string{tc.importpolicy})
		validate[[]string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().PeerGroup(tc.name).AfiSafi(tc.safi).ApplyPolicy().ImportPolicy().Config(), []string{tc.importpolicy})
	}
	pneighbourValidation := []struct {
		naddress          string
		peergroup         string
		description       string
		localaddress      string
		localaddressState string
	}{{
		naddress:          "142.251.240.4",
		peergroup:         "METRO_PR",
		description:       "pr01.dia01",
		localaddress:      "142.251.240.5",
		localaddressState: "0.0.0.0",
	},
		{naddress: "2001:4860:0:1::6a25",
			peergroup:         "SATELLITE6",
			description:       "sr03.dia02",
			localaddress:      "2001:4860:0:1::6a24",
			localaddressState: "::",
		},
	}
	time.Sleep(10 * time.Second)
	for _, tc := range pneighbourValidation {
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Neighbor(tc.naddress).NeighborAddress().Config(), tc.naddress)
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Neighbor(tc.naddress).NeighborAddress().State(), tc.naddress)

		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Neighbor(tc.naddress).PeerGroup().Config(), tc.peergroup)
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Neighbor(tc.naddress).PeerGroup().State(), tc.peergroup)

		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Neighbor(tc.naddress).Description().Config(), tc.description)
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Neighbor(tc.naddress).Description().State(), tc.description)

		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Neighbor(tc.naddress).Transport().LocalAddress().Config(), tc.localaddress)
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Neighbor(tc.naddress).Transport().LocalAddress().State(), tc.localaddressState)

	}

}

func vrfValidator(t *testing.T, dut *ondatra.DUTDevice) {

	descriptionExpectedState := "$Id: //depot/google3/configs/production/network/inet/configlets/PROD/IOSXR/BASE/management_vrf.conf#1 $"
	validate[string](t, dut, gnmi.OC().NetworkInstance("MGMT").Description().Config(), "Management VRF")
	validate[string](t, dut, gnmi.OC().NetworkInstance("MGMT").Description().State(), descriptionExpectedState)
}

func isisValidator(t *testing.T, dut *ondatra.DUTDevice) {
	time.Sleep(30 * time.Second)

	validate[oc.E_Isis_LevelType](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().LevelCapability().Config(), oc.E_Isis_LevelType(oc.Isis_LevelType_LEVEL_2))
	validate[oc.E_Isis_LevelType](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().LevelCapability().State(), oc.E_Isis_LevelType(oc.Isis_LevelType_LEVEL_2))

	validate[uint8](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().MaxEcmpPaths().Config(), 32)
	validate[uint8](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().MaxEcmpPaths().State(), 32)

	validate[[]string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().Net().Config(), []string{"39.752f.0100.0014.0000.9000.0001.0000.4002.bf40.00"})
	validate[[]string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().Net().State(), []string{"39.752f.0100.0014.0000.9000.0001.0000.4002.bf40.00"})

	validate[uint16](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().Timers().LspLifetimeInterval().Config(), 3600)
	validate[uint16](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().Timers().LspLifetimeInterval().State(), 3600)

	validate[uint16](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().Timers().LspRefreshInterval().Config(), 3283)
	validate[uint16](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Global().Timers().LspRefreshInterval().State(), 3283)

	levelValidation := []struct {
		levelnumber uint8
		metricStyle oc.E_Isis_MetricStyle
		authType    oc.E_KeychainTypes_AUTH_TYPE
		authMode    oc.E_IsisTypes_AUTH_MODE
		authPass    string
	}{{
		levelnumber: 1,
		metricStyle: oc.E_Isis_MetricStyle(oc.Isis_MetricStyle_NARROW_METRIC),
		authType:    oc.E_KeychainTypes_AUTH_TYPE(oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY),
		authMode:    oc.E_IsisTypes_AUTH_MODE(oc.IsisTypes_AUTH_MODE_MD5),
		authPass:    "!gOOgl3y",
	},
		{
			levelnumber: 2,
			metricStyle: oc.E_Isis_MetricStyle(oc.Isis_MetricStyle_WIDE_METRIC),
			authType:    oc.E_KeychainTypes_AUTH_TYPE(oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY),
			authMode:    oc.E_IsisTypes_AUTH_MODE(oc.IsisTypes_AUTH_MODE_MD5),
			authPass:    "!gOOgl3y",
		},
	}
	for _, tc := range levelValidation {
		gnmi.Get(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().State())
		validate[oc.E_Isis_MetricStyle](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Level(tc.levelnumber).MetricStyle().Config(), tc.metricStyle)
		validate[oc.E_KeychainTypes_AUTH_TYPE](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Level(tc.levelnumber).Authentication().AuthType().Config(), tc.authType)
		validate[oc.E_IsisTypes_AUTH_MODE](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Level(tc.levelnumber).Authentication().AuthMode().Config(), tc.authMode)
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Level(tc.levelnumber).Authentication().AuthPassword().Config(), tc.authPass)

	}

	interfaceValidation := []struct {
		intfName     string
		enabledd     bool
		passivee     bool
		hellopadding oc.E_Isis_HelloPaddingType
		intfLevel    uint8
	}{{
		intfName:     "Loopback0",
		enabledd:     true,
		passivee:     true,
		hellopadding: oc.E_Isis_HelloPaddingType(oc.Isis_HelloPaddingType_STRICT),
	},
		{
			intfName:     "Bundle-Ether11",
			enabledd:     true,
			passivee:     false,
			hellopadding: oc.E_Isis_HelloPaddingType(oc.Isis_HelloPaddingType_LOOSE),
		},
	}
	for _, tc := range interfaceValidation {
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).InterfaceId().Config(), tc.intfName)
		validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).InterfaceId().State(), tc.intfName)

		validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).Passive().Config(), tc.passivee)
		validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).Passive().State(), tc.passivee)

		validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).Enabled().Config(), tc.enabledd)
		validate[bool](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).Enabled().State(), false)

		validate[oc.E_Isis_HelloPaddingType](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).HelloPadding().Config(), tc.hellopadding)
		validate[oc.E_Isis_HelloPaddingType](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).HelloPadding().State(), tc.hellopadding)

		validate[oc.E_IsisTypes_SAFI_TYPE](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).Af(oc.E_IsisTypes_AFI_TYPE(oc.IsisTypes_AFI_TYPE_IPV4), oc.E_IsisTypes_SAFI_TYPE(oc.IsisTypes_SAFI_TYPE_UNICAST)).SafiName().Config(), oc.E_IsisTypes_SAFI_TYPE(oc.IsisTypes_SAFI_TYPE_UNICAST))
		validate[oc.E_IsisTypes_SAFI_TYPE](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "b2").Isis().Interface(tc.intfName).Af(oc.E_IsisTypes_AFI_TYPE(oc.IsisTypes_AFI_TYPE_IPV4), oc.E_IsisTypes_SAFI_TYPE(oc.IsisTypes_SAFI_TYPE_UNICAST)).SafiName().State(), oc.E_IsisTypes_SAFI_TYPE(oc.IsisTypes_SAFI_TYPE_UNICAST))
	}

}

func mplsValidator(t *testing.T, dut *ondatra.DUTDevice) {
	staticLSP := "ariadne-142.251.240.4-1000702"
	nextHop := "142.251.240.4"
	validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Mpls().Lsps().StaticLsp(staticLSP).Name().Config(), staticLSP)
	validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Mpls().Lsps().StaticLsp(staticLSP).Name().State(), staticLSP)
	gnmi.Get(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Mpls().Lsps().StaticLsp(staticLSP).Egress().IncomingLabel().State())
	incomingLabel := oc.NetworkInstance_Mpls_Lsps_StaticLsp_Egress_IncomingLabel_Union(oc.UnionUint32(1000702))
	incomingConfig := gnmi.Get(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Mpls().Lsps().StaticLsp(staticLSP).Egress().IncomingLabel().Config())
	incomingState := gnmi.Get(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Mpls().Lsps().StaticLsp(staticLSP).Egress().IncomingLabel().State())

	if incomingState != incomingLabel || incomingConfig != incomingLabel {
		t.Errorf("Incoming Lable for MPLS not as expected : Config got %v, State got %v , want %v", incomingConfig, incomingState, incomingLabel)
	}
	validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Mpls().Lsps().StaticLsp(staticLSP).Egress().NextHop().Config(), nextHop)
	validate[string](t, dut, gnmi.OC().NetworkInstance("DEFAULT").Mpls().Lsps().StaticLsp(staticLSP).Egress().NextHop().State(), nextHop)

}

func aclValidator(t *testing.T, dut *ondatra.DUTDevice) {
	aclValidation := []struct {
		aclName string
		seqID   []uint32
		aclType oc.E_Acl_ACL_TYPE
		actions []oc.Acl_AclSet_AclEntry_Actions
	}{
		{
			aclName: "dropTZbit",
			seqID:   []uint32{101, 1000},
			aclType: oc.Acl_ACL_TYPE_ACL_IPV6,
			actions: []oc.Acl_AclSet_AclEntry_Actions{{ForwardingAction: oc.E_Acl_FORWARDING_ACTION(oc.Acl_FORWARDING_ACTION_REJECT)}, {ForwardingAction: oc.E_Acl_FORWARDING_ACTION(oc.Acl_FORWARDING_ACTION_ACCEPT)}},
		},
		{
			aclName: "IPV6-ESPRESSO-NHER",
			seqID:   []uint32{10, 9999},
			aclType: oc.Acl_ACL_TYPE_ACL_IPV6,
			actions: []oc.Acl_AclSet_AclEntry_Actions{{ForwardingAction: oc.E_Acl_FORWARDING_ACTION(oc.Acl_FORWARDING_ACTION_ACCEPT)}, {ForwardingAction: oc.E_Acl_FORWARDING_ACTION(oc.Acl_FORWARDING_ACTION_ACCEPT)}},
		},
		{
			aclName: "GRE_DECAP",
			seqID:   []uint32{30},
			aclType: oc.Acl_ACL_TYPE_ACL_IPV4,
			actions: []oc.Acl_AclSet_AclEntry_Actions{{ForwardingAction: oc.E_Acl_FORWARDING_ACTION(oc.Acl_FORWARDING_ACTION_ACCEPT)}},
		},
		{
			aclName: "ipv4-ntp-servers",
			seqID:   []uint32{130},
			aclType: oc.Acl_ACL_TYPE_ACL_IPV4,
			actions: []oc.Acl_AclSet_AclEntry_Actions{{ForwardingAction: oc.E_Acl_FORWARDING_ACTION(oc.Acl_FORWARDING_ACTION_ACCEPT)}},
		},
		{
			aclName: "IPV4-ESPRESSO-NHER",
			seqID:   []uint32{10},
			aclType: oc.Acl_ACL_TYPE_ACL_IPV4,
			actions: []oc.Acl_AclSet_AclEntry_Actions{{ForwardingAction: oc.E_Acl_FORWARDING_ACTION(oc.Acl_FORWARDING_ACTION_ACCEPT)}},
		},
		{
			aclName: "ipv4-management-af4",
			seqID:   []uint32{130},
			aclType: oc.Acl_ACL_TYPE_ACL_IPV4,
			actions: []oc.Acl_AclSet_AclEntry_Actions{{ForwardingAction: oc.E_Acl_FORWARDING_ACTION(oc.Acl_FORWARDING_ACTION_ACCEPT)}},
		},
	}
	for _, tc := range aclValidation {
		for i, seqid := range tc.seqID {
			validate[uint32](t, dut, gnmi.OC().Acl().AclSet(tc.aclName, tc.aclType).AclEntry(seqid).SequenceId().Config(), seqid)
			validate[uint32](t, dut, gnmi.OC().Acl().AclSet(tc.aclName, tc.aclType).AclEntry(seqid).SequenceId().State(), seqid)
			actionsGotConfig := gnmi.Get(t, dut, gnmi.OC().Acl().AclSet(tc.aclName, tc.aclType).AclEntry(seqid).Actions().Config())
			actionsGotState := gnmi.Get(t, dut, gnmi.OC().Acl().AclSet(tc.aclName, tc.aclType).AclEntry(seqid).Actions().State())
			if actionsGotConfig.ForwardingAction.String() != tc.actions[i].ForwardingAction.String() || actionsGotState.ForwardingAction.String() != tc.actions[i].ForwardingAction.String() {
				t.Errorf("Acl Actions for ACL %v not got as expected got config : %v state %v, want %v", tc.aclName, actionsGotConfig, actionsGotState, &tc.actions[i])
			}

		}

	}

}

func sflowValidator(t *testing.T, dut *ondatra.DUTDevice) {
	validate[string](t, dut, gnmi.OC().Sampling().Sflow().AgentIdIpv4().Config(), "209.85.251.66")
	validate[string](t, dut, gnmi.OC().Sampling().Sflow().AgentIdIpv4().State(), "209.85.251.66")

	validate[uint8](t, dut, gnmi.OC().Sampling().Sflow().Dscp().Config(), uint8(5))
	validate[uint8](t, dut, gnmi.OC().Sampling().Sflow().Dscp().State(), uint8(5))

	validate[bool](t, dut, gnmi.OC().Sampling().Sflow().Enabled().Config(), true)
	validate[bool](t, dut, gnmi.OC().Sampling().Sflow().Enabled().State(), true)

	validate[uint16](t, dut, gnmi.OC().Sampling().Sflow().SampleSize().Config(), 150)
	validate[uint16](t, dut, gnmi.OC().Sampling().Sflow().SampleSize().State(), 150)

	validate[uint16](t, dut, gnmi.OC().Sampling().Sflow().PollingInterval().Config(), 30)
	validate[uint16](t, dut, gnmi.OC().Sampling().Sflow().PollingInterval().State(), 30)

	validate[uint16](t, dut, gnmi.OC().Sampling().Sflow().Collector("109.171.243.104", 6343).Port().Config(), 6343)
	validate[uint16](t, dut, gnmi.OC().Sampling().Sflow().Collector("109.171.243.104", 6343).Port().State(), 6343)

	validate[string](t, dut, gnmi.OC().Sampling().Sflow().Collector("109.171.243.104", 6343).SourceAddress().Config(), "209.85.251.66")
	validate[string](t, dut, gnmi.OC().Sampling().Sflow().Collector("109.171.243.104", 6343).SourceAddress().State(), "209.85.251.66")

	validate[bool](t, dut, gnmi.OC().Sampling().Sflow().Interface("Bundle-Ether6601").Enabled().Config(), true)
	validate[bool](t, dut, gnmi.OC().Sampling().Sflow().Interface("Bundle-Ether6601").Enabled().State(), true)

	validate[string](t, dut, gnmi.OC().Sampling().Sflow().Interface("Bundle-Ether6601").Name().Config(), "Bundle-Ether6601")
	validate[string](t, dut, gnmi.OC().Sampling().Sflow().Interface("Bundle-Ether6601").Name().State(), "Bundle-Ether6601")

}

func qosegressValidator(t *testing.T, dut *ondatra.DUTDevice) {
	qosValidation := []struct {
		inputID     string
		inputWeight uint64
		inputQueue  string
	}{
		{
			inputID:     "nc1",
			inputWeight: 7,
			inputQueue:  "nc1",
		},
		{
			inputID:     "af4",
			inputWeight: 6,
			inputQueue:  "af4",
		},
		{
			inputID:     "af3",
			inputWeight: 5,
			inputQueue:  "af3",
		},
		{
			inputID:     "af2",
			inputWeight: 4,
			inputQueue:  "af2",
		},
		{
			inputID:     "af1",
			inputWeight: 3,
			inputQueue:  "af1",
		},
		{
			inputID:     "be1",
			inputWeight: 2,
			inputQueue:  "be1",
		},
	}
	egressPolicy := "EGRESS_POLICY"
	validate[string](t, dut, gnmi.OC().Qos().SchedulerPolicy(egressPolicy).Name().Config(), egressPolicy)

	for _, tc := range qosValidation {
		validate[string](t, dut, gnmi.OC().Qos().SchedulerPolicy(egressPolicy).Scheduler(1).Input(tc.inputID).Id().Config(), tc.inputID)
		validate[uint64](t, dut, gnmi.OC().Qos().SchedulerPolicy(egressPolicy).Scheduler(1).Input(tc.inputID).Weight().Config(), tc.inputWeight)
		validate[string](t, dut, gnmi.OC().Qos().SchedulerPolicy(egressPolicy).Scheduler(1).Input(tc.inputID).Queue().Config(), tc.inputQueue)
		validate[string](t, dut, gnmi.OC().Qos().Queue(tc.inputID).Name().Config(), tc.inputID)
		validate[string](t, dut, gnmi.OC().Qos().Interface("Bundle-Ether111").Output().Queue(tc.inputID).Name().Config(), tc.inputID)
		validate[string](t, dut, gnmi.OC().Qos().Interface("Bundle-Ether111").Output().SchedulerPolicy().Name().Config(), egressPolicy)
	}
}

func interfaceValidator(t *testing.T, dut *ondatra.DUTDevice) {
	intfStruct := []struct {
		intfName    string
		intfType    oc.E_IETFInterfaces_InterfaceType
		mtu         uint32
		description string
		ipv4add     string
		ipv6Add     string
		ipv6SubNet  uint8
		ipv4SubNet  uint8
	}{
		{
			intfName:    "Bundle-Ether11",
			intfType:    oc.IETFInterfaces_InterfaceType_ieee8023adLag,
			mtu:         9216,
			description: "SR03.DIA02.PO-51 [T=euBC]",
			ipv6Add:     "2001:4860:0:1::6a24",
			ipv6SubNet:  127,
		},
		{
			intfName:    "FourHundredGigE0/0/0/1",
			intfType:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			description: "ME01.DIA08.HU-0-0-0-11 [BE10][T=euME]",
		},
		{
			intfName:    "Loopback0",
			intfType:    oc.IETFInterfaces_InterfaceType_softwareLoopback,
			description: "ME01.DIA08.LoopBack0",
			ipv4add:     "10.61.60.94",
			ipv4SubNet:  32,
			ipv6Add:     "2607:f8b0:8007:5140::11",
			ipv6SubNet:  128,
		},
	}
	for _, tc := range intfStruct {
		validate[string](t, dut, gnmi.OC().Interface(tc.intfName).Name().Config(), tc.intfName)
		validate[string](t, dut, gnmi.OC().Interface(tc.intfName).Name().State(), tc.intfName)

		validate[string](t, dut, gnmi.OC().Interface(tc.intfName).Description().Config(), tc.description)
		validate[string](t, dut, gnmi.OC().Interface(tc.intfName).Description().State(), tc.description)

		validate[oc.E_IETFInterfaces_InterfaceType](t, dut, gnmi.OC().Interface(tc.intfName).Type().Config(), tc.intfType)
		validate[oc.E_IETFInterfaces_InterfaceType](t, dut, gnmi.OC().Interface(tc.intfName).Type().State(), tc.intfType)
		if tc.ipv6Add != "" {
			validate[string](t, dut, gnmi.OC().Interface(tc.intfName).Subinterface(0).Ipv6().Address(tc.ipv6Add).Ip().Config(), tc.ipv6Add)
			validate[uint8](t, dut, gnmi.OC().Interface(tc.intfName).Subinterface(0).Ipv6().Address(tc.ipv6Add).PrefixLength().Config(), tc.ipv6SubNet)
			validate[string](t, dut, gnmi.OC().Interface(tc.intfName).Subinterface(0).Ipv6().Address(tc.ipv6Add).Ip().State(), tc.ipv6Add)
			validate[uint8](t, dut, gnmi.OC().Interface(tc.intfName).Subinterface(0).Ipv6().Address(tc.ipv6Add).PrefixLength().State(), tc.ipv6SubNet)
		}
		if tc.ipv4add != "" {
			validate[string](t, dut, gnmi.OC().Interface(tc.intfName).Subinterface(0).Ipv4().Address(tc.ipv4add).Ip().Config(), tc.ipv4add)
			validate[uint8](t, dut, gnmi.OC().Interface(tc.intfName).Subinterface(0).Ipv4().Address(tc.ipv4add).PrefixLength().Config(), tc.ipv4SubNet)
			validate[string](t, dut, gnmi.OC().Interface(tc.intfName).Subinterface(0).Ipv4().Address(tc.ipv4add).Ip().State(), tc.ipv4add)
			validate[uint8](t, dut, gnmi.OC().Interface(tc.intfName).Subinterface(0).Ipv4().Address(tc.ipv4add).PrefixLength().State(), tc.ipv4SubNet)
		}
		validate[oc.E_Lacp_LacpPeriodType](t, dut, gnmi.OC().Lacp().Interface("FourHundredGigE0/0/0/1").Interval().Config(), oc.Lacp_LacpPeriodType_FAST)
		validate[bool](t, dut, gnmi.OC().Lldp().Interface("Bundle-Ether11").Enabled().Config(), true)
		validate[bool](t, dut, gnmi.OC().Sampling().Sflow().Interface("Bundle-Ether11").Enabled().Config(), true)
		validate[bool](t, dut, gnmi.OC().Sampling().Sflow().Interface("Bundle-Ether11").Enabled().State(), true)

	}

}

func systemValidator(t *testing.T, dut *ondatra.DUTDevice) {

	validate[string](t, dut, gnmi.OC().System().DomainName().Config(), "net.google.com")
	validate[string](t, dut, gnmi.OC().System().DomainName().State(), "net.google.com")

	validate[string](t, dut, gnmi.OC().System().Clock().TimezoneName().Config(), "US/Pacific")
	validate[string](t, dut, gnmi.OC().System().Clock().TimezoneName().State(), "US/Pacific")

	validate[oc.E_Server_AssociationType](t, dut, gnmi.OC().System().Ntp().Server("216.239.35.12").AssociationType().Config(), oc.Server_AssociationType_SERVER)
	validate[oc.E_Server_AssociationType](t, dut, gnmi.OC().System().Ntp().Server("216.239.35.12").AssociationType().State(), oc.Server_AssociationType_SERVER)

	validate[string](t, dut, gnmi.OC().System().Hostname().Config(), "cx02.dia02")
	validate[string](t, dut, gnmi.OC().System().Hostname().State(), "cx02.dia02")

	gnmi.Get(t, dut, gnmi.OC().System().Logging().Config())
	validate[string](t, dut, gnmi.OC().System().Logging().RemoteServer("172.20.0.191").Host().Config(), "172.20.0.191")
	remoteServers := gnmi.GetAll(t, dut, gnmi.OC().System().Logging().RemoteServerAny().Config())
	for _, rem := range remoteServers {
		if rem.GetHost() != "172.20.0.191" {
			t.Errorf("Remote Server host not as expected , want %v got %v", "172.20.0.191", rem.GetHost())

		}
	}
	selector := gnmi.GetAll(t, dut, gnmi.OC().System().Logging().RemoteServer("172.20.0.191").SelectorAny().Config())
	for _, sel := range selector {
		if (sel.Facility != oc.SystemLogging_SYSLOG_FACILITY_LOCAL7) || (sel.Severity != oc.SystemLogging_SyslogSeverity_DEBUG) {
			t.Errorf("Selector Facility or Serverity not as expected got Facility %v Severity %v, want Facility %v and Serverifty %v", sel.Facility, sel.Severity, oc.SystemLogging_SYSLOG_FACILITY_LOCAL7, oc.SystemLogging_SyslogSeverity_DEBUG)
		}
	}
}

func memoryCheck(t *testing.T, dut *ondatra.DUTDevice) uint8 {
	query := gnmi.OC().System().ProcessAny().State()
	processes := gnmi.GetAll(t, dut, query)
	for _, process := range processes {
		processName := process.GetName()
		if processName == "emsd" {
			return process.GetMemoryUtilization()
		}
	}
	return 0
}

func processRestart(t *testing.T, dut *ondatra.DUTDevice) {
	cli := "process restart emsd "
	config.CMDViaGNMI(context.Background(), t, dut, cli)
	time.Sleep(60 * time.Second)
}
func telemetryUnionReplace(t *testing.T, dut *ondatra.DUTDevice, jsonietfVal []byte, asciiVal string) {
	var telemetrySystem map[string]interface{}
	err := json.Unmarshal(jsonietfVal, &telemetrySystem)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	t.Logf("Input JSON (ietfVal): %s", string(jsonietfVal))

	telemetrySystemValue, ok := telemetrySystem["openconfig-telemetry:telemetry-system"]
	if !ok {
		t.Errorf("Key 'openconfig-telemetry:telemetry-system' not found in the JSON. Err: %v", ok)
	}
	jsonVal, err := json.Marshal(telemetrySystemValue)
	if err != nil {
		t.Errorf("Could not marshal json %v ", err)
	}

	gnmiC := dut.RawAPIs().GNMI(t)
	nyconfig := []*gpb.Update{{
		Path: &gpb.Path{
			Origin: "cli",
			Elem:   []*gpb.PathElem{},
		},
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{
				AsciiVal: asciiVal,
			},
		},
	}}
	occonfig := []*gpb.Update{{
		Path: &gpb.Path{
			Origin: "openconfig",
			Elem:   []*gpb.PathElem{{Name: "telemetry-system"}},
		},
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonIetfVal{
				JsonIetfVal: jsonVal,
			},
		},
	}}
	setReq := &gpb.SetRequest{Prefix: &gpb.Path{Target: "DUT", Origin: ""}, UnionReplace: []*gpb.Update{occonfig[0], nyconfig[0]}}
	log.V(1).Infof("SetResponse:\n%s", prototext.Format(setReq))
	_, err = gnmiC.Set(context.Background(), setReq)
	if err != nil {
		t.Errorf("Error while set union replace with oc+ny combination %v", err)
	}
	path := []*gpb.Path{
		{Origin: "openconfig", Elem: []*gpb.PathElem{
			{Name: "telemetry-system"}}},
	}
	sam := &gpb.GetRequest{Path: path, Type: gpb.GetRequest_CONFIG, Encoding: gpb.Encoding_JSON_IETF}
	getres, err := gnmiC.Get(context.Background(), sam)
	if err != nil {
		t.Errorf("Error while set union replace with oc+ny combination %v", err)
	}

	log.V(1).Infof("get cli via gnmi reply: \n %s", prototext.Format(getres))

	if err := json.Unmarshal(jsonVal, &telemetrySystem); err != nil {
		t.Errorf("Error unmarshaling JSON1: %v", err)
	}
	if err := json.Unmarshal(getres.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal(), &telemetrySystem); err != nil {
		t.Errorf("Error unmarshaling JSON2: %v", err)
	}

	equal := reflect.DeepEqual(telemetrySystem, telemetrySystem)
	if !equal {
		t.Errorf("The confugured telemetry data does not match data sent via Union Replace")
		t.Logf("Union replace json %v", string(jsonVal))
		t.Logf("Configured json %v", string(getres.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()))
	}
}

type testcases struct {
	configPath string
	checks     func(*testing.T, *ondatra.DUTDevice)
}

func TestGnmiUnionReplace(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	initialMemory := memoryCheck(t, dut)
	t.Run("Union Replace with Empty Origins", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		emptyOriginPath := &gpb.Update{Path: &gpb.Path{Origin: ""}}
		setReq := &gpb.SetRequest{Prefix: &gpb.Path{Target: "DUT", Origin: ""}, UnionReplace: []*gpb.Update{emptyOriginPath, emptyOriginPath}}
		gnmiC := dut.RawAPIs().GNMI(t)
		setResponse, err := gnmiC.Set(context.Background(), setReq)
		if err == nil {
			t.Error("Expected error with empty config : ", err)

		}
		t.Log(setResponse.GetResponse())
		t.Log(err.Error())
	})
	t.Run("Union Replace with OC and Invalid Base config CLI", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		var jsonietfVal []byte
		occliConfig, err := os.ReadFile("testdata/vrf.txt")
		if err != nil {
			panic(fmt.Sprintf("Cannot load base config: %v", err))
		}
		req := &gpb.SetRequest{}
		prototext.Unmarshal(occliConfig, req)
		replaceContents := req.Replace
		for _, path := range replaceContents {
			jsonietfVal = path.Val.GetJsonIetfVal()
		}
		b := &gnmi.SetBatch{}
		ocRoot := &oc.Root{}
		opts := []ytypes.UnmarshalOpt{
			&ytypes.PreferShadowPath{},
		}
		if err := oc.Unmarshal(jsonietfVal, ocRoot, opts...); err != nil {
			panic(fmt.Sprintf("Cannot unmarshal json config: %v", err))
		}
		gnmi.BatchUnionReplace(b, gnmi.OC().Config(), ocRoot)
		gnmi.BatchUnionReplaceCLI(b, "cisco", "histname$ cisco")
		t.Log("############# STARTED UNION REPLACE ###############")
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			b.Set(t, dut)
		}); errMsg == nil {
			t.Errorf("Expected Fatal Error as CLI config is invalid, but response %v", errMsg)
		}
	})

	t.Run("Union Replace empty OC and Base config CLI", func(t *testing.T) {

		dut := ondatra.DUT(t, "dut")
		gnmiC := dut.RawAPIs().GNMI(t)
		inGetRequest := &gpb.GetRequest{
			Prefix: &gpb.Path{
				Origin: "cli",
			},
			Path: []*gpb.Path{
				{
					Elem: []*gpb.PathElem{{
						Name: "sh run ",
					}},
				},
			},
			Encoding: gpb.Encoding_ASCII,
		}

		gotRes, err := gnmiC.Get(context.Background(), inGetRequest)
		cliJson := gotRes.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal()
		index := strings.Index(cliJson, "hostname")
		if index == -1 {
			t.Errorf("No 'hostname' found in the configuration string. Error : %v ", err)
		}
		result := cliJson[index:]
		jsonietfVal := []byte(`{}`)
		nyconfig := []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cisco_cli",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: result,
				},
			},
		}}
		occonfig := []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "openconfig",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonIetfVal{
					JsonIetfVal: jsonietfVal,
				},
			},
		}}

		setReq := &gpb.SetRequest{Prefix: &gpb.Path{Target: "DUT", Origin: ""}, UnionReplace: []*gpb.Update{occonfig[0], nyconfig[0]}}
		log.V(1).Infof("SetResponse:\n%s", prototext.Format(setReq))
		_, err = gnmiC.Set(context.Background(), setReq)
		if err == nil {
			t.Errorf("Expected error while set union replace with empty oc and base CLI %v", err)
		}
	})
	t.Run("Union Replace with OC and Native Yang config", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		baseConfig := baseConfig(t, dut)
		cliBaseconfig := []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cisco_cli",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: baseConfig,
				},
			},
		}}
		gpbReplaceReq := &gpb.SetRequest{Replace: cliBaseconfig}
		//Replace with base config on the box
		setRes, _ := dut.RawAPIs().GNMI(t).Set(context.Background(), gpbReplaceReq)
		log.V(1).Infof("SetResponse:\n%s", prototext.Format(gpbReplaceReq))
		log.V(1).Infof("SetResponse:\n%s", prototext.Format(setRes))
		var jsonietfVal []byte
		t.Logf("Input JSON (ietfVal): %s", string(jsonietfVal))
		occliConfig, err := os.ReadFile("testdata/vrf.txt")
		if err != nil {
			panic(fmt.Sprintf("Cannot load base config: %v", err))
		}
		req := &gpb.SetRequest{}
		prototext.Unmarshal(occliConfig, req)
		replaceContents := req.Replace
		for _, path := range replaceContents {
			jsonietfVal = path.Val.GetJsonIetfVal()
		}

		ocRoot := &oc.Root{}
		opts := []ytypes.UnmarshalOpt{
			&ytypes.PreferShadowPath{},
		}
		if err := oc.Unmarshal(jsonietfVal, ocRoot, opts...); err != nil {
			panic(fmt.Sprintf("Cannot unmarshal json config: %v", err))
		}
		gnmiC := dut.RawAPIs().GNMI(t)
		inGetRequest := &gpb.GetRequest{
			Prefix: &gpb.Path{
				Origin: "cli",
			},
			Path: []*gpb.Path{
				{
					Elem: []*gpb.PathElem{{
						Name: "sh run | json unified-model",
					}},
				},
			},
			Encoding: gpb.Encoding_ASCII,
		}

		gotRes, _ := gnmiC.Get(context.Background(), inGetRequest)
		cliJson := gotRes.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal()
		startIndex := strings.Index(cliJson, "{")
		jsonString := cliJson[startIndex:]
		jsonString = strings.Replace(jsonString, "[null]", "null", -1)
		jsonString = jsonString[:strings.LastIndex(jsonString, "}")+1]

		var data map[string]interface{}
		err = json.Unmarshal([]byte(jsonString), &data)
		if err != nil {
			t.Error("Error:", err)
			return
		}

		dataContent, ok := data["data"]
		if !ok {
			t.Errorf("Key 'data' not found in the JSON")
		}

		extractedJSON, err := json.Marshal(dataContent)
		if err != nil {
			t.Errorf("Could not marshal data from json: %v", err)
		}
		t.Log(string(extractedJSON))

		///////////new code here //////////
		t.Log(dataContent)

		nyconfig := []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cisco_native",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonIetfVal{
					JsonIetfVal: extractedJSON,
				},
			},
		}}
		var updates []*gpb.Update

		occonfig := []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "openconfig",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonIetfVal{
					JsonIetfVal: jsonietfVal,
				},
			},
		}}
		t.Log("######Update Request sent with NY all in one and OC#########")

		setReqNY := &gpb.SetRequest{Prefix: &gpb.Path{Target: "DUT", Origin: ""}, Update: []*gpb.Update{occonfig[0], nyconfig[0]}}
		log.V(1).Infof("SetResponse:\n%s", prototext.Format(setReqNY))
		_, err = gnmiC.Set(context.Background(), setReqNY)
		if err != nil {
			t.Errorf("Error while set union replace with oc+ny combination %v", err)
		}
		t.Log("############end of UPDATE ###############")

		updates = append(updates, occonfig...)
		// Unmarshal the JSON string
		var jsonData map[string]interface{}
		err = json.Unmarshal(extractedJSON, &jsonData)
		if err != nil {
			t.Errorf("Error decoding JSON: %v", err)

		}
		for key, value := range jsonData {
			jsonValue, err := json.Marshal(value)
			if err != nil {
				t.Logf("Error marshaling value for key %s: %v\n", key, err)
				continue
			}

			update := &gpb.Update{
				Path: &gpb.Path{
					Origin: "cisco_native",
					Elem: []*gpb.PathElem{
						{Name: key},
					},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonIetfVal{
						JsonIetfVal: jsonValue,
					},
				},
			}
			updates = append(updates, update)
		}

		t.Log("######Update Request sent with NY in series and OC#########")

		setReqNY = &gpb.SetRequest{Prefix: &gpb.Path{Target: "DUT", Origin: ""}, Update: updates}
		log.V(1).Infof("SetResponse:\n%s", prototext.Format(setReqNY))
		_, err = gnmiC.Set(context.Background(), setReqNY)
		if err != nil {
			t.Errorf("Error while set union replace with oc+ny combination %v", err)
		}
		t.Log("############end of UPDATE ###############")
		t.Log("##########Union Replace sent along with OC###########")
		setReqNY = &gpb.SetRequest{Prefix: &gpb.Path{Target: "DUT", Origin: ""}, UnionReplace: updates}
		log.V(1).Infof("SetResponse:\n%s", prototext.Format(setReqNY))
		_, err = gnmiC.Set(context.Background(), setReqNY)
		if err != nil {
			t.Errorf("Error while set union replace with oc+ny combination %v", err)
		}
		t.Log("############end of UNION REPLACE ###############")
		//time.Sleep(30 * time.Minute)
		// setReq := &gpb.SetRequest{Prefix: &gpb.Path{Target: "DUT", Origin: ""}, UnionReplace: []*gpb.Update{occonfig[0], nyconfig[0]}}
		// log.V(1).Infof("SetResponse:\n%s", prototext.Format(setReq))
		// _, err = gnmiC.Set(context.Background(), setReq)
		// if err != nil {
		// 	t.Errorf("Error while set union replace with oc+ny combination %v", err)
		// }
	})
	gnmi.Delete(t, dut, gnmi.OC().Qos().Config())
	baseConfig := baseConfig(t, dut)
	cases := []testcases{
		{
			configPath: "testdata/vrf.txt",
			checks:     vrfValidator,
		},
		{
			configPath: "testdata/telemetry.txt",
		},
		{
			configPath: "testdata/bgp.txt",
			checks:     bgpValidator,
		},
		{
			configPath: "testdata/isis.txt",
			checks:     isisValidator,
		},
		{
			configPath: "testdata/mpls.txt",
			checks:     mplsValidator,
		},
		{
			configPath: "testdata/acl.txt",
			checks:     aclValidator,
		},
		{
			configPath: "testdata/sflow.txt",
			checks:     sflowValidator,
		},
		{
			configPath: "testdata/qos-egress.txt",
			checks:     qosegressValidator,
		},
		{
			configPath: "testdata/interfaces.txt",
			checks:     interfaceValidator,
		},
		{
			configPath: "testdata/system.txt",
			checks:     systemValidator,
		},
	}

	for _, tc := range cases {
		fileName := filepath.Base(tc.configPath)
		part := fileName[:len(fileName)-len(filepath.Ext(fileName))]
		t.Run(fmt.Sprintf("Union Replace with OC %v and Base config CLI", strings.ToUpper(part)), func(t *testing.T) {
			cliBaseconfig := []*gpb.Update{{
				Path: &gpb.Path{
					Origin: "cisco_cli",
					Elem:   []*gpb.PathElem{},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_AsciiVal{
						AsciiVal: baseConfig,
					},
				},
			}}
			gpbReplaceReq := &gpb.SetRequest{Replace: cliBaseconfig}
			//Replace with base config on the box
			setRes, _ := dut.RawAPIs().GNMI(t).Set(context.Background(), gpbReplaceReq)
			log.V(1).Infof("SetResponse:\n%s", prototext.Format(gpbReplaceReq))
			log.V(1).Infof("SetResponse:\n%s", prototext.Format(setRes))

			var jsonietfVal []byte
			var asciiVal string
			occliConfig, err := os.ReadFile(tc.configPath)
			if err != nil {
				panic(fmt.Sprintf("Cannot load base config: %v", err))
			}
			req := &gpb.SetRequest{}
			prototext.Unmarshal(occliConfig, req)
			replaceContents := req.Replace
			for _, path := range replaceContents {
				jsonietfVal = path.Val.GetJsonIetfVal()
			}
			t.Log(string(jsonietfVal))
			updateContents := req.Update
			for _, path := range updateContents {
				asciiVal = path.Val.GetAsciiVal()
			}
			t.Log(asciiVal)
			asciiVal = baseConfig + asciiVal
			if tc.configPath == "testdata/telemetry.txt" {
				telemetryUnionReplace(t, dut, jsonietfVal, asciiVal)

			} else {
				ocRoot := &oc.Root{}
				opts := []ytypes.UnmarshalOpt{
					&ytypes.PreferShadowPath{},
				}
				if err := oc.Unmarshal(jsonietfVal, ocRoot, opts...); err != nil {
					panic(fmt.Sprintf("Cannot unmarshal json config: %v", err))
				}

				t.Log(asciiVal)

				b := &gnmi.SetBatch{}
				gnmi.BatchUnionReplace(b, gnmi.OC().Config(), ocRoot)
				gnmi.BatchUnionReplaceCLI(b, "cisco", asciiVal)
				t.Log("############# STARTED UNION REPLACE ###############")
				b.Set(t, dut)
				t.Log("############# DONE UNION REPLACE ###############")
				tc.checks(t, dut)
				if tc.configPath == "testdata/vrf.txt" {
					processRestart(t, dut)
					b.Set(t, dut)
					tc.checks(t, dut)
				}
			}
		})
	}
	finalMemory := memoryCheck(t, dut)
	t.Logf("Memory Utilization of process emsd : Initial %v Final %v", initialMemory, finalMemory)
	if finalMemory > initialMemory {
		t.Errorf("Memory Utilization for emsd increased after running Union Replace Testcases: Final Memory %v, Initial Memory %v\n", initialMemory, finalMemory)
	}

}
