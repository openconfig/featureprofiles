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

package mpls_label_block_with_isis_test

import (
	"github.com/openconfig/ondatra"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (

	//SRReservedLabelblockName                  = "sr-reserved-label-block"
	SRReservedLabelblockName                  = "default-srgb"
	SRReservedLabelblockLowerbound            = 1000000
	SRReservedLabelblockUpperbound            = 1048575
	SRReservedLabelblockLowerboundReconfigure = 1110000
	SRReservedLabelblockUpperboundReconfigure = 1048575
	srgbMplsLabelBlockName                    = "400000 465001"
	srlbMplsLabelBlockName                    = "40000 41000"
	srgbGblBlockReconfigure                   = "101000 102001"
	srgbLclBlockReconfigure                   = "200000 201001"
	srgbGlobalLowerBound                      = 400000
	srgbGlobalUpperBound                      = 465001
	srgbLocalLowerBound                       = 40000
	srgbLocalUpperBound                       = 41000
	srgbLocalID                               = "100.1.1.1"
	srlbLocalID                               = "200.1.1.1"
	plenIPv4                                  = 30
	plenIPv6                                  = 126
	password                                  = "google"
	ateV4Route                                = "203.0.113.0/30"
	ateV6Route                                = "2001:db8::203:0:113:0/126"
	v4IP                                      = "203.0.113.1"
	v6IP                                      = "2001:db8::203:0:113:1"
	v4Route                                   = "203.0.113.0"
	v6Route                                   = "2001:db8::203:0:113:0"
	dutV4Metric                               = 100
	dutV6Metric                               = 100
	ateV4Metric                               = 200
	ateV6Metric                               = 200
	dutV4Route                                = "192.0.2.0/30"
	dutV6Route                                = "2001:db8::/126"
	v4NetName                                 = "isisv4Net"
	v6NetName                                 = "isisv6Net"
	v4FlowName                                = "v4Flow"
	v6FlowName                                = "v6Flow"
	devIsisName                               = "devIsis"
)

// configureSRGBGlobalPath
func configureSRGBViaMplsGlobalPath(LowerBoundLabel int, UpperBoundLabel int) *oc.Root {

	d := &oc.Root{}

	netInstance := d.GetOrCreateNetworkInstance("DEFAULT")
	netInstance.Name = ygot.String("DEFAULT")
	mplsGlobal := netInstance.GetOrCreateMpls().GetOrCreateGlobal()

	rlb := mplsGlobal.GetOrCreateReservedLabelBlock(SRReservedLabelblockName)
	rlb.LocalId = ygot.String(SRReservedLabelblockName)
	rlb.LowerBound = oc.UnionUint32(LowerBoundLabel)
	rlb.UpperBound = oc.UnionUint32(UpperBoundLabel)

	sr := netInstance.GetOrCreateSegmentRouting()
	srgb := sr.GetOrCreateSrgb(SRReservedLabelblockName)
	srgb.LocalId = ygot.String(SRReservedLabelblockName)
	srgb.SetMplsLabelBlocks([]string{SRReservedLabelblockName})

	return d
}

func ReconfigureSRGBViaMplsGlobalPath(t *testing.T, dut *ondatra.DUTDevice) {
	t.Run("Segment Routing state checks - SR, SRGB and SRLB", func(t *testing.T) {

		// Update SR Config
		srgbGlobalReConfig := configureSRGBViaMplsGlobalPath(srgbGlobalLowerBound, srgbGlobalUpperBound)
		gnmi.Update(t, dut, gnmi.OC().Config(), srgbGlobalReConfig)

		// Verify Reconfig
		srReconfigPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Global().ReservedLabelBlock(SRReservedLabelblockName).State()
		srReconfigResponse := gnmi.Get(t, dut, srReconfigPath)

		if got := srReconfigResponse.GetLowerBound(); got != oc.UnionUint32(srgbGlobalLowerBound) {
			t.Errorf("FAIL- SR Reserved Block is not present on DUT, got %d, want %d", got, srgbGlobalLowerBound)
		} else {
			t.Logf("SR Reserved Block is present on DUT value: %d, want %d", got, srgbGlobalLowerBound)
		}

		if got := srReconfigResponse.GetUpperBound(); got != oc.UnionUint32(srgbGlobalUpperBound) {
			t.Errorf("FAIL- SR Reserved Block is not present on DUT, got %d, want %d", got, srgbGlobalUpperBound)
		} else {
			t.Logf("SR Reserved Block is present on DUT value: %d, want %d", got, srgbGlobalUpperBound)
		}
	})
}

// configureISISMPLSSRReconfigure configures isis and MPLS SR on DUT with new label block bounds.
func configureISISMPLSSRReconfigure(t *testing.T, ts *isissession.TestSession, SRReservedLabelblockLowerbound uint32, SRReservedLabelblockUpperbound uint32, srgbGblBlock string, srgbLclBlock string) {
	t.Helper()
	d := ts.DUTConf
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName)
	prot.Enabled = ygot.Bool(false)
	mplsprot := netInstance.GetOrCreateMpls().GetOrCreateGlobal()
	mplsprot.GetOrCreateReservedLabelBlock(SRReservedLabelblockName).LowerBound = oc.UnionUint32(SRReservedLabelblockLowerbound)
	mplsprot.GetOrCreateReservedLabelBlock(SRReservedLabelblockName).UpperBound = oc.UnionUint32(SRReservedLabelblockUpperbound)
	// SRGB and SRLB Configurations
	segmentrouting := netInstance.GetOrCreateSegmentRouting()
	srgb := segmentrouting.GetOrCreateSrgb("99.99.99.99")
	srgb.LocalId = ygot.String("99.99.99.99")
	srgb.SetMplsLabelBlocks([]string{srgbGblBlock})

	srlb := segmentrouting.GetOrCreateSrlb("88.88.88.88")
	srlb.LocalId = ygot.String("88.88.88.88")
	srlb.SetMplsLabelBlock(srgbLclBlock)
}

// configureISIS configures isis on DUT.
func configureISISMPLSSR(t *testing.T, ts *isissession.TestSession) {
	t.Helper()
	d := ts.DUTConf
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName)
	prot.Enabled = ygot.Bool(true)

	// ISIS Segment Routing configurations
	isissr := prot.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateSegmentRouting()
	isissr.Enabled = ygot.Bool(true)

	// MPLS reserved label block.
	mplsprot := netInstance.GetOrCreateMpls().GetOrCreateGlobal()
	mplsprot.GetOrCreateReservedLabelBlock(SRReservedLabelblockName).LowerBound = oc.UnionUint32(SRReservedLabelblockLowerbound)
	mplsprot.GetOrCreateReservedLabelBlock(SRReservedLabelblockName).UpperBound = oc.UnionUint32(SRReservedLabelblockUpperbound)

	// SRGB and SRLB Configurations
	segmentrouting := netInstance.GetOrCreateSegmentRouting()
	srgb := segmentrouting.GetOrCreateSrgb("srgb-global")
	srgb.LocalId = ygot.String(srgbLocalID)
	srgb.SetMplsLabelBlocks([]string{srgbMplsLabelBlockName})

	srlb := segmentrouting.GetOrCreateSrlb(srgbLocalID)
	srlb.LocalId = ygot.String(srlbLocalID)
	srlb.SetMplsLabelBlock(srlbMplsLabelBlockName)

	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()

	// Global configs.
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.AuthenticationCheck = ygot.Bool(true)
	globalISIS.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE

	// Level configs.
	level := isis.GetOrCreateLevel(2)
	level.LevelNumber = ygot.Uint8(2)
	level.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC

	// Authentication configs.
	auth := level.GetOrCreateAuthentication()
	auth.Enabled = ygot.Bool(true)
	auth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
	auth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
	auth.AuthPassword = ygot.String(password)

	// Interface configs.
	intfName := ts.DUTPort1.Name()
	if deviations.ExplicitInterfaceInDefaultVRF(ts.DUT) {
		intfName += ".0"
	}
	intf := isis.GetOrCreateInterface(intfName)

	// Interface timers.
	isisIntfTimers := intf.GetOrCreateTimers()
	isisIntfTimers.CsnpInterval = ygot.Uint16(5)
	if deviations.ISISTimersCsnpIntervalUnsupported(ts.DUT) {
		isisIntfTimers.CsnpInterval = nil
	}
	isisIntfTimers.LspPacingInterval = ygot.Uint64(150)

	// Interface level configs.
	isisIntfLevel := intf.GetOrCreateLevel(2)
	isisIntfLevel.LevelNumber = ygot.Uint8(2)
	isisIntfLevel.SetEnabled(true)
	isisIntfLevel.Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateHelloAuthentication().Enabled = ygot.Bool(true)
	isisIntfLevel.GetHelloAuthentication().AuthPassword = ygot.String(password)
	isisIntfLevel.GetHelloAuthentication().AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
	isisIntfLevel.GetHelloAuthentication().AuthMode = oc.IsisTypes_AUTH_MODE_MD5

	isisIntfLevelTimers := isisIntfLevel.GetOrCreateTimers()
	isisIntfLevelTimers.HelloInterval = ygot.Uint32(5)
	isisIntfLevelTimers.HelloMultiplier = ygot.Uint8(3)

	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(dutV4Metric)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(dutV6Metric)
	if deviations.MissingIsisInterfaceAfiSafiEnable(ts.DUT) {
		isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = nil
		isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = nil
	}
}

// configureOTG configures isis and traffic on OTG.
func configureOTG(t *testing.T, ts *isissession.TestSession) {
	t.Helper()

	ts.ATEIntf1.Isis().Basic().SetEnableWideMetric(true)
	ts.ATEIntf1.Isis().RouterAuth().AreaAuth().SetAuthType("md5").SetMd5(password)
	ts.ATEIntf1.Isis().RouterAuth().DomainAuth().SetAuthType("md5").SetMd5(password)
	ts.ATEIntf1.Isis().Interfaces().Items()[0].Authentication().SetAuthType("md5").SetMd5(password)

	// netv4 is a simulated network containing the ipv4 addresses specified by targetNetwork
	netv4 := ts.ATEIntf1.Isis().V4Routes().Add().SetName(v4NetName).SetLinkMetric(ateV4Metric)
	netv4.Addresses().Add().SetAddress(v4Route).SetPrefix(uint32(isissession.ATEISISAttrs.IPv4Len))

	// netv6 is a simulated network containing the ipv6 addresses specified by targetNetwork
	netv6 := ts.ATEIntf1.Isis().V6Routes().Add().SetName(v6NetName).SetLinkMetric(ateV6Metric)
	netv6.Addresses().Add().SetAddress(v6Route).SetPrefix(uint32(isissession.ATEISISAttrs.IPv6Len))

	// We generate traffic entering along port2 and destined for port1
	srcIpv4 := ts.ATEIntf2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	srcIpv6 := ts.ATEIntf2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	t.Log("Configuring v4 traffic flow ")

	v4Flow := ts.ATETop.Flows().Add().SetName(v4FlowName)
	v4Flow.Metrics().SetEnable(true)
	v4Flow.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{v4NetName})
	v4Flow.Size().SetFixed(512)
	v4Flow.Rate().SetPps(100)
	v4Flow.Duration().Continuous()
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(isissession.ATEISISAttrs.MAC)
	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(isissession.ATEISISAttrs.IPv4)
	v4.Dst().Increment().SetStart(v4IP).SetCount(1)

	t.Log("Configuring v6 traffic flow ")

	v6Flow := ts.ATETop.Flows().Add().SetName(v6FlowName)
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{v6NetName})
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)
	v6Flow.Duration().Continuous()
	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(isissession.ATEISISAttrs.MAC)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(isissession.ATEISISAttrs.IPv6)
	v6.Dst().Increment().SetStart(v6IP).SetCount(1)
}

// verifyISIS verifies ISIS on DUT.
func verifyISIS(t *testing.T, ts *isissession.TestSession) {
	statePath := isissession.ISISPath(ts.DUT)

	intfName := ts.DUTPort1.Name()
	if deviations.ExplicitInterfaceInDefaultVRF(ts.DUT) {
		intfName += ".0"
	}
	t.Run("ISIS telemetry", func(t *testing.T) {
		time.Sleep(time.Minute * 2)

		// Checking adjacency
		ateSysID, err := ts.AwaitAdjacency()
		if err != nil {
			t.Fatalf("Adjacency state invalid: %v", err)
		}
		t.Run("Adjacency state checks", func(t *testing.T) {
			adjPath := statePath.Interface(intfName).Level(2).Adjacency(ateSysID)
			if got := gnmi.Get(t, ts.DUT, adjPath.SystemId().State()); got != ateSysID {
				t.Errorf("FAIL- Expected neighbor system id not found, got %s, want %s", got, ateSysID)
			}
			want := []string{isissession.ATEAreaAddress, isissession.DUTAreaAddress}
			if got := gnmi.Get(t, ts.DUT, adjPath.AreaAddress().State()); !cmp.Equal(got, want, cmpopts.SortSlices(func(a, b string) bool { return a < b })) {
				t.Errorf("FAIL- Expected area address not found, got %s, want %s", got, want)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.LocalExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected local extended circuit id not found,expected non-zero value, got %d", got)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.MultiTopology().State()); got != false {
				t.Errorf("FAIL- Expected value for multi topology not found, got %t, want %t", got, false)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.NeighborCircuitType().State()); got != oc.Isis_LevelType_LEVEL_2 {
				t.Errorf("FAIL- Expected value for circuit type not found, got %s, want %s", got, oc.Isis_LevelType_LEVEL_2)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.NeighborIpv4Address().State()); got != isissession.ATEISISAttrs.IPv4 {
				t.Errorf("FAIL- Expected value for ipv4 address not found, got %s, want %s", got, isissession.ATEISISAttrs.IPv4)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.NeighborExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected neighbor extended circuit id not found,expected non-zero value, got %d", got)
			}
			snpaAddress := gnmi.Get(t, ts.DUT, adjPath.NeighborSnpa().State())
			mac, err := net.ParseMAC(snpaAddress)
			if !(mac != nil && err == nil) {
				t.Errorf("FAIL- Expected value for snpa address not found, got %s", snpaAddress)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.Nlpid().State()); !cmp.Equal(got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}) {
				t.Errorf("FAIL- Expected address families not found, got %s, want %s", got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6})
			}
			ipv6Address := gnmi.Get(t, ts.DUT, adjPath.NeighborIpv6Address().State())
			ip := net.ParseIP(ipv6Address)
			if !(ip != nil && ip.To16() != nil) {
				t.Errorf("FAIL- Expected ipv6 address not found, got %s", ipv6Address)
			}
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.Priority().State()).Val(); !ok {
				t.Errorf("FAIL- Priority is not present")
			}
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart status not present")
			}
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.RestartSupport().State()).Val(); !ok {
				t.Errorf("FAIL- Restart support not present")
			}
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart suppress not present")
			}
		})
	})
}

// verifyMPLSSR verifies MPLS SR on DUT.
func verifyMPLSSR(t *testing.T, ts *isissession.TestSession, LowerBoundLabel int, UpperBoundLabel int) {
	t.Helper()
	netInstance := ts.DUTConf.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	pcl := ts.DUTConf.GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName)
	t.Run("Segment Routing state checks - SR, SRGB and SRLB", func(t *testing.T) {
		SREnabled := pcl.GetIsis().GetGlobal().GetSegmentRouting().GetEnabled()
		if !SREnabled {
			t.Errorf("FAIL- Segment Routing is not enabled on DUT")
		}
		if ts.DUT.Vendor() == ondatra.CISCO {
			t.Log("Skipping Protocol Checks")
		} else {
			srgbValue := pcl.GetIsis().GetGlobal().GetSegmentRouting().GetSrgb()
			if srgbValue == "nil" || srgbValue == "" {
				t.Errorf("FAIL- SRGB is not present on DUT")
			} else {
				t.Logf("SRGB is present on DUT value: %s", srgbValue)
			}
			srlbValue := pcl.GetIsis().GetGlobal().GetSegmentRouting().GetSrlb()
			if srlbValue == "nil" || srlbValue == "" {
				t.Errorf("FAIL- SRLB is not present on DUT")
			} else {
				t.Logf("SRLB is present on DUT value: %s", srlbValue)
			}
		}

		mplsprot := netInstance.GetOrCreateMpls().GetOrCreateGlobal()
		if got := mplsprot.GetReservedLabelBlock(SRReservedLabelblockName).GetLowerBound(); got != oc.UnionUint32(LowerBoundLabel) {
			t.Errorf("FAIL- SR Reserved Block is not present on DUT, got %d, want %d", got, LowerBoundLabel)
		} else {
			t.Logf("SR Reserved Block is present on DUT value: %d, want %d", got, LowerBoundLabel)
		}
		if got := mplsprot.GetReservedLabelBlock(SRReservedLabelblockName).GetUpperBound(); got != oc.UnionUint32(UpperBoundLabel) {
			t.Errorf("FAIL- SR Reserved Block is not present on DUT, got %d, want %d", got, UpperBoundLabel)
		} else {
			t.Logf("SR Reserved Block is present on DUT value: %d, want %d", got, UpperBoundLabel)
		}
	})
}

// TestMPLSLabelBlockWithISIS verifies MPLS label block SRGB and SRLB on the DUT.
func TestMPLSLabelBlockWithISIS(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ts := isissession.MustNew(t).WithISIS()
	configureISISMPLSSR(t, ts)

	if ts.DUT.Vendor() == ondatra.CISCO {
		t.Log("configure SR label block via MPLS OC path for Cisco")
		srgbGlobalConfig := configureSRGBViaMplsGlobalPath(SRReservedLabelblockLowerbound, SRReservedLabelblockUpperbound)
		gnmi.Update(t, dut, gnmi.OC().Config(), srgbGlobalConfig)
	}

	configureOTG(t, ts)
	otg := ts.ATE.OTG()
	pcl := ts.DUTConf.GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName)
	fptest.LogQuery(t, "Protocol ISIS", isissession.ProtocolPath(ts.DUT).Config(), pcl)
	isissr := ts.DUTConf.GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName).GetIsis().GetGlobal().GetSegmentRouting()
	fptest.LogQuery(t, "Protocol ISIS Global Segment Routing", isissession.ProtocolPath(ts.DUT).Config(), isissr)
	if ts.DUT.Vendor() == ondatra.CISCO {
		t.Log("Skipping SR Protocol Check")
	} else {
		sr := ts.DUTConf.GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).GetMpls().GetGlobal()
		fptest.LogQuery(t, "Protocol MPLS and SR", isissession.ProtocolPath(ts.DUT).Config(), sr)
	}

	ts.PushAndStart(t)
	time.Sleep(time.Minute * 2)

	// Checking ISIS
	verifyISIS(t, ts)

	// Checking MPLS SR
	verifyMPLSSR(t, ts, SRReservedLabelblockLowerbound, SRReservedLabelblockUpperbound)

	// Reconfigure MPLS SR
	if ts.DUT.Vendor() == ondatra.CISCO {
		// Verify SR Config via MPLS OC Path
		ReconfigureSRGBViaMplsGlobalPath(t, ts.DUT)
	} else {
		configureISISMPLSSRReconfigure(t, ts, SRReservedLabelblockLowerboundReconfigure, SRReservedLabelblockUpperboundReconfigure, srgbGblBlockReconfigure, srgbLclBlockReconfigure)
		// Checking MPLS SR
		verifyMPLSSR(t, ts, srgbGlobalLowerBound, srgbGlobalUpperBound)
	}

	// Traffic checks
	t.Run("Traffic checks", func(t *testing.T) {
		t.Logf("Starting traffic")
		otg.StartTraffic(t)
		time.Sleep(time.Second * 15)
		t.Logf("Stop traffic")
		otg.StopTraffic(t)

		otgutils.LogFlowMetrics(t, otg, ts.ATETop)
		otgutils.LogPortMetrics(t, otg, ts.ATETop)

		for _, flow := range []string{v4FlowName, v6FlowName} {
			t.Log("Checking flow telemetry...")
			recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).State())
			txPackets := recvMetric.GetCounters().GetOutPkts()
			rxPackets := recvMetric.GetCounters().GetInPkts()
			lostPackets := txPackets - rxPackets
			lossPct := lostPackets * 100 / txPackets

			if lossPct > 1 {
				t.Errorf("FAIL- Got %v%% packet loss for %s ; expected < 1%%", lossPct, flow)
			}
		}
	})
}
