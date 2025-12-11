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
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// BMPConfigParams holds BMP related configs
type BMPConfigParams struct {
	DutAS        uint32
	BGPObj       *oc.NetworkInstance_Protocol_Bgp
	Source       string
	LocalAddr    string
	StationAddr  string
	StationPort  uint16
	StatsTimeOut uint16
}

// ConfigureBMP applies BMP station configuration on DUT.
func ConfigureBMP(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, cfgParams BMPConfigParams) {
	t.Helper()
	if deviations.BMPOCUnsupported(dut) {

		bmpConfig := new(strings.Builder)

		fmt.Fprintf(bmpConfig, `
router bgp %d
bgp monitoring
! BMP station
monitoring station BMP_STN
update-source %s
statistics
connection address %s
connection mode active port %d
`, cfgParams.DutAS, cfgParams.Source, cfgParams.StationAddr, cfgParams.StationPort)

		helpers.GnmiCLIConfig(t, dut, bmpConfig.String())

	} else {
		// TODO: BMP support is not yet available, so the code below is commented out and will be enabled once BMP is implemented.
		t.Log("BMP support is not yet available, so the code below is commented out and will be enabled once BMP is implemented.")
		// // === BMP Configuration ===
		// bmp := cfgParams.BGPObj.Global.GetOrCreateBmp()
		// bmp.LocalAddress = ygot.String(cfgParams.LocalAddr)
		// bmp.StatisticsTimeout = ygot.Uint16(cfgParams.StatsTimeOut)

		// // --- Create BMP Station ---
		// st := bmp.GetOrCreateStation("BMP_STN")
		// st.Address = ygot.String(cfgParams.StationAddr)
		// st.Port = ygot.Uint16(cfgParams.StationPort)
		// st.ConnectionMode = oc.BgpTypes_BMPStationMode_ACTIVE
		// st.Description = ygot.String("ATE BMP station")
		// st.PolicyType = oc.BgpTypes_BMPPolicyType_POST_POLICY
		// st.ExcludeNoneligible = ygot.Bool(true)
		// // Push configuration
		// gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bmp)
	}
}
