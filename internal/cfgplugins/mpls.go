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
)

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
