package supervisor_switchover_test

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	// ISISInstance is ISIS instance name.
	ISISInstance = "DEFAULT"
	// PeerGrpName is BGP peer group name.
	PeerGrpName = "BGP-PEER-GROUP"

	// DUTAs is DUT AS.
	DUTAs = 64500
	// ATEAs is ATE AS.
	ATEAs = 64501
	// ATEAs2 is ATE source port AS
	ATEAs2 = 64502
	// ISISMetric is Metric for ISIS
	ISISMetric = 100
	// RouteCount for both BGP and ISIS
	RouteCount = 200
	// AdvertiseBGPRoutesv4 is the starting IPv4 address advertised by ATE Port 1.
	AdvertiseBGPRoutesv4 = "203.0.113.1"

	dutAreaAddress        = "49.0001"
	dutSysID              = "1920.0000.2001"
	dutStartIPAddr        = "192.0.2.1"
	ateStartIPAddr        = "192.0.2.2"
	plenIPv4              = 30
	authPassword          = "ISISAuthPassword"
	advertiseISISRoutesv4 = "198.18.0.0"
	setALLOWPolicy        = "ALLOW"
)

// DUTIPList, ATEIPList are lists of DUT and ATE interface ip addresses.
// ISISMetricList, ISISSetBitList are ISIS metric and setbit lists.
var (
	DUTIPList      = make(map[string]net.IP)
	ATEIPList      = make(map[string]net.IP)
	ISISMetricList []uint32
	ISISSetBitList []bool
)

// buildPortIPs generates ip addresses for the ports in binding file.
// (Both DUT and ATE ports).
func buildPortIPs(dut *ondatra.DUTDevice) {
	var dutIPIndex, ipSubnet, ateIPIndex int = 1, 2, 2
	var endSubnetIndex = 253
	for _, dp := range dut.Ports() {
		dutNextIP := nextIP(net.ParseIP(dutStartIPAddr), dutIPIndex, ipSubnet)
		ateNextIP := nextIP(net.ParseIP(ateStartIPAddr), ateIPIndex, ipSubnet)
		DUTIPList[dp.ID()] = dutNextIP
		ATEIPList[dp.ID()] = ateNextIP

		// Increment DUT and ATE host ip index by 4.
		dutIPIndex = dutIPIndex + 4
		ateIPIndex = ateIPIndex + 4

		// Reset DUT and ATE ip indexes when it is greater than endSubnetIndex.
		if dutIPIndex > int(endSubnetIndex) {
			ipSubnet = ipSubnet + 1
			dutIPIndex = 1
			ateIPIndex = 2
		}
	}
}

// nextIP returns ip address based on hostIndex and subnetIndex provided.
func nextIP(ip net.IP, hostIndex int, subnetIndex int) net.IP {
	s := ip.String()
	sa := strings.Split(s, ".")
	sa[2] = strconv.Itoa(subnetIndex)
	sa[3] = strconv.Itoa(hostIndex)
	s = strings.Join(sa, ".")
	return net.ParseIP(s)
}

func BuildConfig(t *testing.T) *oc.Root {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}

	// Generate ip addresses to configure DUT and ATE ports.
	buildPortIPs(dut)

	// Network instance and BGP configs.
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	bgp := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(DUTAs)
	global.RouterId = ygot.String(dutStartIPAddr)

	afi := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afi.Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup(PeerGrpName)
	pg.PeerAs = ygot.Uint32(ATEAs)
	pg.PeerGroupName = ygot.String(PeerGrpName)
	afipg := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afipg.Enabled = ygot.Bool(true)
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(setALLOWPolicy)
	stmt, err := pdef.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{setALLOWPolicy})
		rpl.SetImportPolicy([]string{setALLOWPolicy})
	} else {
		rpl := afipg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{setALLOWPolicy})
		rpl.SetImportPolicy([]string{setALLOWPolicy})
	}

	// ISIS configs.
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(ISISInstance)
	}
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.AuthenticationCheck = ygot.Bool(true)
	if deviations.ISISGlobalAuthenticationNotRequired(dut) {
		globalISIS.AuthenticationCheck = nil
	}
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	lspBit := globalISIS.GetOrCreateLspBit().GetOrCreateOverloadBit()
	lspBit.SetBit = ygot.Bool(false)
	isisTimers := globalISIS.GetOrCreateTimers()
	isisTimers.LspLifetimeInterval = ygot.Uint16(600)
	isisTimers.LspRefreshInterval = ygot.Uint16(250)
	spfTimers := isisTimers.GetOrCreateSpf()
	spfTimers.SpfHoldInterval = ygot.Uint64(5000)
	spfTimers.SpfFirstInterval = ygot.Uint64(600)

	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC

	if deviations.ISISLevelEnabled(dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}
	isisLevel2Auth := isisLevel2.GetOrCreateAuthentication()
	isisLevel2Auth.Enabled = ygot.Bool(true)
	if deviations.ISISExplicitLevelAuthenticationConfig(dut) {
		isisLevel2Auth.DisableCsnp = ygot.Bool(false)
		isisLevel2Auth.DisableLsp = ygot.Bool(false)
		isisLevel2Auth.DisablePsnp = ygot.Bool(false)
	}
	isisLevel2Auth.AuthPassword = ygot.String(authPassword)
	isisLevel2Auth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
	isisLevel2Auth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY

	for _, dp := range dut.Ports() {
		// Interfaces config.
		i := d.GetOrCreateInterface(dp.Name())
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		i.Description = ygot.String("from oc")
		i.Name = ygot.String(dp.Name())

		s := i.GetOrCreateSubinterface(0)
		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		a4 := s4.GetOrCreateAddress(DUTIPList[dp.ID()].String())
		a4.Type = oc.IfIp_Ipv4AddressType_SECONDARY
		a4.PrefixLength = ygot.Uint8(plenIPv4)

		if deviations.ExplicitPortSpeed(dut) {
			i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, dp)
		}

		// BGP neighbor configs.
		nv4 := bgp.GetOrCreateNeighbor(ATEIPList[dp.ID()].String())
		nv4.PeerGroup = ygot.String(PeerGrpName)
		if dp.ID() == "port1" {
			nv4.PeerAs = ygot.Uint32(ATEAs2)
		} else {
			nv4.PeerAs = ygot.Uint32(ATEAs)
		}
		nv4.Enabled = ygot.Bool(true)

		// ISIS configs.
		intfName := dp.Name()
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			intfName = dp.Name() + ".0"
		}
		isisIntf := isis.GetOrCreateInterface(intfName)
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntfAfi := isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfAfi.Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntf.Af = nil
		}

		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)

		isisIntfLevelTimers := isisIntfLevel.GetOrCreateTimers()
		isisIntfLevelTimers.HelloInterval = ygot.Uint32(1)
		isisIntfLevelTimers.HelloMultiplier = ygot.Uint8(5)

		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(200)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)

		// Configure ISIS AfiSafi enable flag at the global level
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			isisIntfLevelAfi.Enabled = nil
		}
	}
	p := gnmi.OC()
	fptest.LogQuery(t, "DUT", p.Config(), d)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, dp := range dut.Ports() {
			ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
			niIntf, _ := ni.NewInterface(dp.Name())
			niIntf.Interface = ygot.String(dp.Name())
			niIntf.Subinterface = ygot.Uint32(0)
			niIntf.Id = ygot.String(dp.Name() + ".0")
		}
	}
	return d
}
