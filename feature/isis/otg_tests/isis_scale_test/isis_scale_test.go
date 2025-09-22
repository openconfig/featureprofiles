package isis_scale_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	linkIpv4PLen   = 30
	linkIpv6PLen   = 126
	dutAreaAddress = "49.0001"
	dutSysID       = "1920.0000.2001"
	lagType        = oc.IfAggregate_AggregationType_LACP
)

type testData struct {
	dutData                  *dutData
	ateData                  *otgconfighelpers.ATEData
	correctAggInterfaceCount int
	correctISISAdjCount      int
}

type dutData struct {
	isisData *cfgplugins.ISISGlobalParams
	lags     []*cfgplugins.DUTAggData
}

var (
	defaultNetworkInstance = ""
	isisOTGBlockRR1        = otgconfighelpers.ISISOTGBlock{
		Name:            "RR1",
		Col:             20,
		Row:             20,
		ISISIDFirstOct:  "20",
		LinkIP4FirstOct: 21,
		V6Pfx:           otgconfighelpers.Pfx{FirstOctet: "22", PfxLen: 64, Count: 1},
		LinkMultiplier:  2,
	}
	isisOTGBlockRR2 = otgconfighelpers.ISISOTGBlock{
		Name:            "RR2",
		Col:             20,
		Row:             20,
		ISISIDFirstOct:  "23",
		LinkIP4FirstOct: 24,
		V6Pfx:           otgconfighelpers.Pfx{FirstOctet: "25", PfxLen: 64, Count: 1},
		LinkMultiplier:  2,
	}
	isisOTGBlockDynamic = otgconfighelpers.ISISOTGBlock{
		Name:            "Dynamic",
		Col:             2,
		Row:             2,
		ISISIDFirstOct:  "56",
		LinkIP4FirstOct: 57,
		V4Pfx:           otgconfighelpers.Pfx{FirstOctet: "58", PfxLen: 25, Count: 1},
		V6Pfx:           otgconfighelpers.Pfx{FirstOctet: "58", PfxLen: 64, Count: 2},
		LinkMultiplier:  1,
	}

	activity = oc.Lacp_LacpActivityType_ACTIVE
	period   = oc.Lacp_LacpPeriodType_FAST

	lacpParams = &cfgplugins.LACPParams{
		Activity: &activity,
		Period:   &period,
	}

	test2Data = testData{
		dutData:                  &dut2Data,
		ateData:                  &ate2Data,
		correctAggInterfaceCount: 2,
		correctISISAdjCount:      3,
	}

	dutagg21 = attrs.Attributes{
		Desc:    "DUT to ATE LAG1",
		IPv4Len: linkIpv4PLen,
		IPv6Len: linkIpv6PLen,
	}

	dutagg22 = attrs.Attributes{
		Desc:    "DUT to ATE LAG2",
		IPv4Len: linkIpv4PLen,
		IPv6Len: linkIpv6PLen,
	}

	dut2Data = dutData{
		isisData: &cfgplugins.ISISGlobalParams{
			DUTArea:  "49.0001",
			DUTSysID: "1920.0000.2001",
		},
		lags: []*cfgplugins.DUTAggData{
			{
				Attributes: dutagg21,
				SubInterfaces: []*cfgplugins.DUTSubInterfaceData{
					{
						VlanID:        100,
						IPv4Address:   net.ParseIP("192.0.2.13"),
						IPv6Address:   net.ParseIP("2003:db8::1"),
						IPv4PrefixLen: linkIpv4PLen,
						IPv6PrefixLen: linkIpv6PLen,
					},
				},
				OndatraPortsIdx: []int{0},
				LacpParams:      lacpParams,
				AggType:         lagType,
			},
			{
				Attributes: dutagg22,
				SubInterfaces: []*cfgplugins.DUTSubInterfaceData{
					{
						VlanID:        101,
						IPv4Address:   net.ParseIP("192.0.2.17"),
						IPv6Address:   net.ParseIP("2004:db8::1"),
						IPv4PrefixLen: linkIpv4PLen,
						IPv6PrefixLen: linkIpv6PLen,
					},
					{
						VlanID:        102,
						IPv4Address:   net.ParseIP("192.0.2.21"),
						IPv6Address:   net.ParseIP("2005:db8::1"),
						IPv4PrefixLen: linkIpv4PLen,
						IPv6PrefixLen: linkIpv6PLen,
					},
				},
				OndatraPortsIdx: []int{1},
				LacpParams:      lacpParams,
				AggType:         lagType,
			},
		},
	}

	r21 = &otgconfighelpers.AteEmulatedRouterData{
		Name:            "R1",
		DUTIPv4:         "192.0.2.13",
		ATEIPv4:         "192.0.2.14",
		LinkIPv4PLen:    linkIpv4PLen,
		DUTIPv6:         "2003:db8::1",
		ATEIPv6:         "2003:db8::2",
		LinkIPv6PLen:    linkIpv6PLen,
		EthMAC:          "02:55:10:10:10:02",
		ISISAreaAddress: "490001",
		ISISSysID:       "640000000003",
		VlanID:          100,
		ISISBlocks:      []*otgconfighelpers.ISISOTGBlock{&isisOTGBlockRR1},
	}
	r22 = &otgconfighelpers.AteEmulatedRouterData{
		Name:            "R2",
		DUTIPv4:         "192.0.2.17",
		ATEIPv4:         "192.0.2.18",
		LinkIPv4PLen:    linkIpv4PLen,
		DUTIPv6:         "2004:db8::1",
		ATEIPv6:         "2004:db8::2",
		LinkIPv6PLen:    linkIpv6PLen,
		EthMAC:          "02:55:20:20:20:02",
		ISISAreaAddress: "490001",
		ISISSysID:       "640000000004",
		VlanID:          101,
		ISISBlocks:      []*otgconfighelpers.ISISOTGBlock{&isisOTGBlockRR2},
	}
	r23 = &otgconfighelpers.AteEmulatedRouterData{
		Name:                   "R3",
		DUTIPv4:                "192.0.2.21",
		ATEIPv4:                "192.0.2.22",
		LinkIPv4PLen:           linkIpv4PLen,
		DUTIPv6:                "2005:db8::1",
		ATEIPv6:                "2005:db8::2",
		LinkIPv6PLen:           linkIpv6PLen,
		EthMAC:                 "02:55:30:30:30:02",
		ISISAreaAddress:        "490001",
		ISISSysID:              "640000000005",
		VlanID:                 102,
		ISISBlocks:             []*otgconfighelpers.ISISOTGBlock{&isisOTGBlockDynamic},
		ISISLspRefreshInterval: 10,
	}

	ate2Data = otgconfighelpers.ATEData{
		ConfigureISIS: true,
		Lags: []*otgconfighelpers.ATELagData{
			{
				Name:     "lag1",
				Mac:      "02:55:10:10:10:01",
				Ports:    []otgconfighelpers.ATEPortData{{Name: "port1", Mac: "02:55:10:10:10:03", OndatraPortIdx: 0}},
				Erouters: []*otgconfighelpers.AteEmulatedRouterData{r21},
			},
			{
				Name:     "lag2",
				Mac:      "02:55:20:20:20:01",
				Ports:    []otgconfighelpers.ATEPortData{{Name: "port2", Mac: "02:55:20:20:20:03", OndatraPortIdx: 1}},
				Erouters: []*otgconfighelpers.AteEmulatedRouterData{r22, r23},
			},
		},
		TrafficFlowsMap: map[*otgconfighelpers.AteEmulatedRouterData][]*otgconfighelpers.AteEmulatedRouterData{
			r21: {r22, r23},
			r22: {r21},
			r23: {r21},
		},
	}
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, dutData *dutData) {
	t.Logf("===========Configuring DUT===========")
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	for _, l := range dutData.lags {
		b := &gnmi.SetBatch{}
		// Create LAG interface
		l.LagName = netutil.NextAggregateInterface(t, dut)
		agg := cfgplugins.NewAggregateInterface(t, dut, b, l)
		b.Set(t, dut)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			for k := range agg.GetOrCreateSubinterfaceMap() {
				fptest.AssignToNetworkInstance(t, dut, l.LagName, defaultNetworkInstance, k)
			}
		}
	}
	// Wait for LAG interfaces to be AdminStatus UP
	for _, l := range dutData.lags {
		gnmi.Await(t, dut, gnmi.OC().Interface(l.LagName).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_UP)
	}
	dutData.isisData.ISISInterfaceNames = createISISInterfaceNames(t, dut, dutData)
	b := &gnmi.SetBatch{}
	cfgplugins.NewISIS(t, dut, dutData.isisData, b)
	b.Set(t, dut)
}

func createISISInterfaceNames(t *testing.T, dut *ondatra.DUTDevice, dt *dutData) []string {
	t.Helper()
	loopback0 := netutil.LoopbackInterface(t, dut, 0)
	interfaceNames := []string{loopback0}
	for _, l := range dt.lags {
		if l.Attributes.IPv4 != "" {
			interfaceNames = append(interfaceNames, l.LagName)
		} else {
			for _, s := range l.SubInterfaces {
				interfaceNames = append(interfaceNames, fmt.Sprintf("%s.%d", l.LagName, s.VlanID))
			}
		}
	}
	return interfaceNames
}

func TestISISScaleStatic(t *testing.T) {

	t.Logf("===========Configuring ATE===========")
	testInfo := test2Data
	testInfo.ateData.ATE = ondatra.ATE(t, "ate")
	top := otgconfighelpers.ConfigureATE(t, testInfo.ateData.ATE, testInfo.ateData)
	testInfo.ateData.ATE.OTG().PushConfig(t, top)
	testInfo.ateData.AppendTrafficFlows(t, top)

	// Start protocols on ATE
	testInfo.ateData.ATE.OTG().StartProtocols(t)

	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	defaultNetworkInstance = deviations.DefaultNetworkInstance(dut)
	testInfo.dutData.isisData.NetworkInstanceName = defaultNetworkInstance
	configureDUT(t, dut, testInfo.dutData)
}
