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

// Configure static MPLS label binding using OC on device
func MPLSStaticLSP(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, nextHopIP string, intfName string, protocolType string) {
	if deviations.StaticMplsLspOCUnsupported(dut) {
		cliConfig := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig = fmt.Sprintf(`
				mpls ip
				mpls static top-label %v %s %s pop payload-type %s
				`, incomingLabel, intfName, nextHopIP, protocolType)
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
