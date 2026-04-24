// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgplugins

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

type StaticLSPParams struct {
	Name         string
	Label        uint32
	Interface    string
	NextHop      string
	ProtocolType string
	VRF          string
}

type DecapMPLSParams struct {
	ScaleStaticLSP          bool
	MplsStaticLabels        []int
	MplsStaticLabelsForIPv6 []int
	NextHops                []string
	NextHopsV6              []string
}

// MPLSStaticLSP configures static MPLS label binding using OC on device.
func MPLSStaticLSP(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, nextHopIP string, intfName string, protocolType string) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if intfName != "" {
				cliConfig = fmt.Sprintf(`
					mpls ip
					mpls static top-label %v %s %s pop payload-type %s
					`, incomingLabel, intfName, nextHopIP, protocolType)
			} else {
				cliConfig = fmt.Sprintf(`
					mpls ip
					mpls static top-label %v %s pop payload-type %s
					`, incomingLabel, nextHopIP, protocolType)
			}
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	d := &oc.Root{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	staticMplsCfg := mplsCfg.GetOrCreateLsps().GetOrCreateStaticLsp(lspName)
	staticMplsCfg.GetOrCreateEgress().SetIncomingLabel(oc.UnionUint32(incomingLabel))
	staticMplsCfg.GetOrCreateEgress().SetNextHop(nextHopIP)
	staticMplsCfg.GetOrCreateEgress().SetPushLabel(oc.Egress_PushLabel_IMPLICIT_NULL)

	gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}

// NewStaticMplsLspPopLabel configures static MPLS label binding (LBL1) using CLI with deviation, if OC is unsupported on the device.
func NewStaticMplsLspPopLabel(t *testing.T, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, intfName string, nextHopIP string, protocolType string) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if intfName != "" {
				cliConfig = fmt.Sprintf(`
					mpls ip
					mpls static top-label %v %s %s pop payload-type %s
					`, incomingLabel, intfName, nextHopIP, protocolType)
			} else {
				cliConfig = fmt.Sprintf(`
					mpls ip
					mpls static top-label %v %s pop payload-type %s
					`, incomingLabel, nextHopIP, protocolType)
			}
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	d := &oc.Root{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	staticMplsCfg := mplsCfg.GetOrCreateLsps().GetOrCreateStaticLsp(lspName)
	staticMplsCfg.GetOrCreateEgress().SetIncomingLabel(oc.UnionUint32(incomingLabel))
	staticMplsCfg.GetOrCreateEgress().SetNextHop(nextHopIP)
	staticMplsCfg.GetOrCreateEgress().SetPushLabel(oc.Egress_PushLabel_IMPLICIT_NULL)

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}

// NewStaticMplsLspVRFPopLabel configures static MPLS label binding (LBL1) using CLI with deviation, if OC is unsupported on the device. It also supports VRF selection
func NewStaticMplsLspVRFPopLabel(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params StaticLSPParams) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if params.Interface != "" {
				t.Logf("Configuring static LSP on interface %s", params.Interface)
				cliConfig = fmt.Sprintf(`
					mpls ip
					mpls static top-label %v %s %s pop payload-type %s
					`, params.Label, params.Interface, params.NextHop, params.ProtocolType)
			} else {
				if params.VRF != "" {
					t.Logf("Configuring static LSP on VRF %s", params.VRF)
					cliConfig = fmt.Sprintf(`
					mpls ip
					mpls static top-label %v vrf %s %s pop payload-type %s
					`, params.Label, params.VRF, params.NextHop, params.ProtocolType)
				} else {
					cliConfig = fmt.Sprintf(`
					mpls ip
					mpls static top-label %v %s pop payload-type %s
					`, params.Label, params.NextHop, params.ProtocolType)
				}
			}
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	d := &oc.Root{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	staticMplsCfg := mplsCfg.GetOrCreateLsps().GetOrCreateStaticLsp(params.Name)
	staticMplsCfg.GetOrCreateEgress().SetIncomingLabel(oc.UnionUint32(params.Label))
	staticMplsCfg.GetOrCreateEgress().SetNextHop(params.NextHop)
	staticMplsCfg.GetOrCreateEgress().SetPushLabel(oc.Egress_PushLabel_IMPLICIT_NULL)
	// TODO Set VRF in LSP config when available in OC model
	gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}

// RemoveStaticMplsLspPopLabel removes static MPLS POP label binding using CLI with deviation, if OC is unsupported on the device.
func RemoveStaticMplsLspPopLabel(t *testing.T, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, intfName string, nextHopIP string, protocolType string) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if intfName != "" {
				cliConfig = fmt.Sprintf(`
					no mpls static top-label %v %s %s pop payload-type %s
					`, incomingLabel, intfName, nextHopIP, protocolType)
			} else {
				cliConfig = fmt.Sprintf(`
					no mpls static top-label %v %s pop payload-type %s
					`, incomingLabel, nextHopIP, protocolType)
			}
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	d := &oc.Root{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	mplsCfg.GetOrCreateLsps().DeleteStaticLsp(lspName)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}

// NewStaticMplsLspSwapLabel configures a static MPLS LSP and swaps label.
func NewStaticMplsLspSwapLabel(t *testing.T, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, nextHopIP string, mplsSwapLabelTo uint32, lspNextHopIndex uint32) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:

			cliConfig = fmt.Sprintf(`
			    mpls ip
    			mpls static top-label %v %s swap-label %v
				`, incomingLabel, nextHopIP, mplsSwapLabelTo)

			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	d := &oc.Root{}
	// ConfigureDefaultNetworkInstance configures the default network instance name and type.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	staticMplsCfg := mplsCfg.GetOrCreateLsps().GetOrCreateStaticLsp(lspName)
	staticMplsCfg.GetOrCreateEgress().SetIncomingLabel(oc.UnionUint32(incomingLabel))
	staticMplsCfg.GetOrCreateEgress().GetOrCreateLspNextHop(lspNextHopIndex).SetIpAddress(nextHopIP)
	staticMplsCfg.GetOrCreateEgress().GetOrCreateLspNextHop(lspNextHopIndex).SetPushLabel(oc.UnionUint32(mplsSwapLabelTo))
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}

// RemoveStaticMplsLspSwapLabel removes a static MPLS LSP and swaps label.
func RemoveStaticMplsLspSwapLabel(t *testing.T, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, nextHopIP string, mplsSwapLabelTo uint32) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:

			cliConfig = fmt.Sprintf(`
				no mpls static top-label %v %s swap-label %v
				`, incomingLabel, nextHopIP, mplsSwapLabelTo)

			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	d := &oc.Root{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	mplsCfg.GetOrCreateLsps().DeleteStaticLsp(lspName)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}

// NewStaticMplsLspPushLabel configures a static MPLS LSP.
func NewStaticMplsLspPushLabel(t *testing.T, dut *ondatra.DUTDevice, lspName string, intfName string, nextHopIP string, destIP string, mplsPushLabel uint32, lspNextHopIndex uint32, protocolType string) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:

			cliConfig = fmt.Sprintf(`
    			mpls ip
				nexthop-group TestGrp type mpls
   					entry 0 push label-stack %v nexthop %s
				traffic-policies
					traffic-policy MPLS_TRAFFIC_POLICY
						match DA %s
						destination prefix %s
						actions
							count
							redirect next-hop group TestGrp
						match ipv4-all-default ipv4
						match ipv6-all-default ipv6
				interface %s
					traffic-policy input MPLS_TRAFFIC_POLICY
				`, mplsPushLabel, nextHopIP, protocolType, destIP, intfName)

			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	d := &oc.Root{}
	// ConfigureDefaultNetworkInstance configures the default network instance name and type.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	staticMplsCfg := mplsCfg.GetOrCreateLsps().GetOrCreateStaticLsp(lspName)
	staticMplsCfg.GetOrCreateEgress().GetOrCreateLspNextHop(lspNextHopIndex).SetIpAddress(nextHopIP)
	staticMplsCfg.GetOrCreateEgress().GetOrCreateLspNextHop(lspNextHopIndex).SetPushLabel(oc.UnionUint32(mplsPushLabel))
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}

// RemoveStaticMplsLspPushLabel removes a static MPLS LSP.
func RemoveStaticMplsLspPushLabel(t *testing.T, dut *ondatra.DUTDevice, lspName string, intfName string) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:

			cliConfig = fmt.Sprintf(`
				interface %s
				no traffic-policy input MPLS_TRAFFIC_POLICY
				traffic-policies
					no traffic-policy MPLS_TRAFFIC_POLICY
				no nexthop-group TestGrp type mpls
				`, intfName)

			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	d := &oc.Root{}
	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	mplsCfg.GetOrCreateLsps().DeleteStaticLsp(lspName)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}

func MPLSStaticLSPByPass(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, nextHopIP string, protocolType string, byPass bool) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig = fmt.Sprintf(`
					mpls ip
					mpls static top-label %v %s pop payload-type %s access-list bypass
					`, incomingLabel, nextHopIP, protocolType)
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	} else {
		d := &oc.Root{}
		fptest.ConfigureDefaultNetworkInstance(t, dut)
		mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
		staticMplsCfg := mplsCfg.GetOrCreateLsps().GetOrCreateStaticLsp(lspName)
		staticMplsCfg.GetOrCreateEgress().SetIncomingLabel(oc.UnionUint32(incomingLabel))
		staticMplsCfg.GetOrCreateEgress().SetNextHop(nextHopIP)
		staticMplsCfg.GetOrCreateEgress().SetPushLabel(oc.Egress_PushLabel_IMPLICIT_NULL)

		gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
	}
}

// mplsGlobalStaticLspAttributes configures the MPLS global static LSP attributes.
func mplsGlobalStaticLspAttributes(t *testing.T, ni *oc.NetworkInstance, params OcPolicyForwardingParams) {
	t.Helper()
	if params.DecapPolicy.DecapMPLSParams.ScaleStaticLSP {
		mplsCfgv4 := ni.GetOrCreateMpls()
		for i, nexthop := range params.DecapPolicy.DecapMPLSParams.NextHops {
			staticMplsCfgv4 := mplsCfgv4.GetOrCreateLsps().GetOrCreateStaticLsp(
				fmt.Sprintf("%s%d", params.DecapPolicy.StaticLSPNameIPv4, i),
			)
			egressv4 := staticMplsCfgv4.GetOrCreateEgress()
			egressv4.IncomingLabel = oc.UnionUint32(params.DecapPolicy.DecapMPLSParams.MplsStaticLabels[i])
			egressv4.NextHop = ygot.String(nexthop)
		}
		mplsCfgv6 := ni.GetOrCreateMpls()
		for i, nexthop := range params.DecapPolicy.DecapMPLSParams.NextHopsV6 {
			staticMplsCfgv6 := mplsCfgv6.GetOrCreateLsps().GetOrCreateStaticLsp(
				fmt.Sprintf("%s%d", params.DecapPolicy.StaticLSPNameIPv6, i),
			)
			egressv6 := staticMplsCfgv6.GetOrCreateEgress()
			egressv6.IncomingLabel = oc.UnionUint32(params.DecapPolicy.DecapMPLSParams.MplsStaticLabelsForIPv6[i])
			egressv6.NextHop = ygot.String(nexthop)
		}

	} else {
		mplsCfgv4 := ni.GetOrCreateMpls()
		staticMplsCfgv4 := mplsCfgv4.GetOrCreateLsps().GetOrCreateStaticLsp(params.DecapPolicy.StaticLSPNameIPv4)
		egressv4 := staticMplsCfgv4.GetOrCreateEgress()
		egressv4.IncomingLabel = oc.UnionUint32(params.DecapPolicy.StaticLSPLabelIPv4)
		egressv4.NextHop = ygot.String(params.DecapPolicy.StaticLSPNextHopIPv4)

		mplsCfgv6 := ni.GetOrCreateMpls()
		staticMplsCfgv6 := mplsCfgv6.GetOrCreateLsps().GetOrCreateStaticLsp(params.DecapPolicy.StaticLSPNameIPv6)
		egressv6 := staticMplsCfgv6.GetOrCreateEgress()
		egressv6.IncomingLabel = oc.UnionUint32(params.DecapPolicy.StaticLSPLabelIPv6)
		egressv6.NextHop = ygot.String(params.DecapPolicy.StaticLSPNextHopIPv6)
	}
}

// MPLSStaticLSPConfig configures the interface mpls static lsp.
// TODO: Need to refactor this function by adding one more parameter ocMPLSStaticLSPParams
func MPLSStaticLSPConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance, ocPFParams OcPolicyForwardingParams) {
	if deviations.StaticMplsUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if ocPFParams.DecapPolicy.DecapMPLSParams.ScaleStaticLSP {
				var mplsStaticLspConfig string
				var mplsStaticLspConfigV6 string
				for i, nexthop := range ocPFParams.DecapPolicy.DecapMPLSParams.NextHops {
					mplsStaticLspConfig += fmt.Sprintf("mpls static top-label %d %s pop payload-type ipv4 access-list bypass\n", ocPFParams.DecapPolicy.DecapMPLSParams.MplsStaticLabels[i], nexthop)
				}
				helpers.GnmiCLIConfig(t, dut, mplsStaticLspConfig)
				for i, nexthopIpv6 := range ocPFParams.DecapPolicy.DecapMPLSParams.NextHopsV6 {
					mplsStaticLspConfigV6 += fmt.Sprintf("mpls static top-label %d %s pop payload-type ipv6 access-list bypass\n", ocPFParams.DecapPolicy.DecapMPLSParams.MplsStaticLabelsForIPv6[i], nexthopIpv6)
				}
				helpers.GnmiCLIConfig(t, dut, mplsStaticLspConfigV6)
			} else {
				helpers.GnmiCLIConfig(t, dut, staticLSPArista)
			}
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'mpls static lsp'", dut.Vendor())
		}
	} else {
		mplsGlobalStaticLspAttributes(t, ni, ocPFParams)
	}
}

// MPLSSRConfigBasic holds all parameters needed to configure MPLS and SR on the DUT.
type MPLSSRConfigBasic struct {
	InstanceName   string
	SrgbName       string
	SrgbStartLabel uint32
	SrgbEndLabel   uint32
	SrgbID         string
}

// NewMPLSSRBasic configures MPLS on the DUT using OpenConfig.
func NewMPLSSRBasic(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, cfg MPLSSRConfigBasic) {
	if deviations.IsisSrgbSrlbUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig = fmt.Sprintf("mpls ip\nmpls label range isis-sr %v %v", cfg.SrgbStartLabel, cfg.SrgbEndLabel)
		default:
			t.Errorf("Deviation IsisSrgbSrlbUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		helpers.GnmiCLIConfig(t, dut, cliConfig)
	} else {
		t.Helper()
		d := &oc.Root{}
		netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

		// Set Protocol Config
		mpls := netInstance.GetOrCreateMpls()
		mplsGlobal := mpls.GetOrCreateGlobal()

		rlb := mplsGlobal.GetOrCreateReservedLabelBlock(cfg.SrgbName)

		rlb.SetLowerBound(oc.UnionUint32(cfg.SrgbStartLabel))
		rlb.SetUpperBound(oc.UnionUint32(cfg.SrgbEndLabel))

		sr := netInstance.GetOrCreateSegmentRouting()
		srgbConfig := sr.GetOrCreateSrgb(cfg.SrgbName)
		srgbConfig.SetMplsLabelBlocks([]string{cfg.SrgbName})
		srgbConfig.SetLocalId(cfg.SrgbID)

		// === Add protocol subtree into the batch ===
		gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mpls)
		gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).SegmentRouting().Config(), sr)
	}
}

// LabelRangeOCConfig configures MPLS label ranges on the DUT using OpenConfig.
func LabelRangeOCConfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	mplsObj := ni.GetOrCreateMpls().GetOrCreateGlobal()
	// Map of local-id → [lowerBound, upperBound]
	labelRanges := map[string][2]uint32{
		"bgp-sr":                  {16, 0},
		"dynamic":                 {16, 0},
		"isis-sr":                 {16, 0},
		"l2evpn":                  {16, 0},
		"l2evpn ethernet-segment": {16, 0},
		"ospf-sr":                 {16, 0},
		"srlb":                    {16, 0},
		"static":                  {16, 1048560},
	}
	t.Logf("Mpls Object %v, label range %v", mplsObj, labelRanges)
	for localID, bounds := range labelRanges {
		rlb := mplsObj.GetOrCreateReservedLabelBlock(localID)
		rlb.LocalId = ygot.String(localID)
		rlb.LowerBound = oc.UnionUint32(bounds[0])
		rlb.UpperBound = oc.UnionUint32(bounds[1])
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}
