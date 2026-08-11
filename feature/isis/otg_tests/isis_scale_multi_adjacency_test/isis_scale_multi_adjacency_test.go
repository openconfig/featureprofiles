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

package isis_scale_multi_adjacency_test

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	isisscalehelpers "github.com/openconfig/featureprofiles/internal/isisscale"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/ondatra/gnmi/oc"
)

type descriptor struct {
	name           string
	dimension      []int
	linkMultiplier int
	blockCount     int
}

func initializeMultiAdjISISScaleTestData(t *testing.T) *isisscalehelpers.TestInfo {
	t.Helper()

	blocksDescriptors := []*descriptor{
		{
			name:           "RoutersTypeA",
			dimension:      []int{12, 12},
			linkMultiplier: 4,
			blockCount:     4,
		},
		{
			name:           "RoutersTypeB",
			dimension:      []int{12, 12},
			linkMultiplier: 4,
			blockCount:     4,
		},
		{
			name:           "RoutersTypeC",
			dimension:      []int{12, 12},
			linkMultiplier: 4,
			blockCount:     4,
		},
		{
			name:           "Dynamic",
			dimension:      []int{12, 12},
			linkMultiplier: 4,
			blockCount:     1,
		},
	}
	aggregateCount := 4
	// NOTE(scale): subInterfacesCountPerAggregate is set to 76 across 4 aggregate interfaces
	// to establish 304 total IS-IS Level 2 adjacencies (4 x 76 = 304). This allows running on 4-port
	// physical testbeds (testbed_dut_ate_4links.textproto) while maintaining the 304 adjacency target.
	subInterfacesCountPerAggregate := 76
	initialVlanID := 1000
	initialIPv4Address := net.ParseIP("192.0.0.1")
	initialIPv6Address := net.ParseIP("2001:db8::1")

	// Create DUT data.
	dutData := &isisscalehelpers.DutData{
		Lags: isisscalehelpers.CreateDUTAggregateInterfacesData(t, aggregateCount, subInterfacesCountPerAggregate, initialVlanID, initialIPv4Address, initialIPv6Address),
		IsisData: &cfgplugins.ISISGlobalParams{
			DUTArea:  "49.0001",
			DUTSysID: "1920.0000.2001",
		},
		ISISAuthKey: "google_isis_key",
	}

	// Create ATE data.
	ateEmulatedRouterData := isisscalehelpers.CreateATEEmulatedRouterData(t, dutData.Lags)
	for _, er := range ateEmulatedRouterData {
		er.ISISAuthKey = "google_isis_key"
	}
	lagToErouterMap := make(map[int][]*otgconfighelpers.AteEmulatedRouterData)
	for i := 0; i < aggregateCount; i++ {
		lagToErouterMap[i] = ateEmulatedRouterData[i*subInterfacesCountPerAggregate : (i+1)*subInterfacesCountPerAggregate]
	}
	ateData := isisscalehelpers.CreateATEData(lagToErouterMap)

	// Create ISIS blocks for ATE routers.
	isisOTGBlocks := createATEISISBlocks(blocksDescriptors)
	// Add ISIS blocks to the ATE routers
	blockIndex := 0
	for _, b := range blocksDescriptors {
		for i := 1; i <= b.blockCount; i++ {
			blockName := b.name + "_" + strconv.Itoa(i)
			if block, ok := isisOTGBlocks[blockName]; ok {
				ateData.Lags[blockIndex%aggregateCount].Erouters[0].ISISBlocks = append(ateData.Lags[blockIndex%aggregateCount].Erouters[0].ISISBlocks, block)
				blockIndex++
			}
		}
	}

	testInfo := &isisscalehelpers.TestInfo{
		DutData:                  dutData,
		ATEData:                  ateData,
		CorrectAggInterfaceCount: aggregateCount,
		CorrectISISAdjCount:      aggregateCount * subInterfacesCountPerAggregate,
		CorrectISISLSPCount:      calculateLSPCount(blocksDescriptors, dutData.Lags),
		CorrectIPRouteCount:      calculateRouteCount(blocksDescriptors, dutData.Lags),
	}

	return testInfo
}

func calculateLSPCount(descriptors []*descriptor, dutAggData []*isisscalehelpers.DutAggregateInterfaceData) int {
	lspCount := 0
	for _, d := range descriptors {
		lspCount += d.dimension[0] * d.dimension[1] * d.blockCount
	}

	for _, l := range dutAggData {
		lspCount += len(l.SubInterfaces)
	}

	return lspCount
}

func calculateRouteCount(descriptors []*descriptor, dutAggData []*isisscalehelpers.DutAggregateInterfaceData) map[oc.E_Types_ADDRESS_FAMILY]int {
	routesCount := map[oc.E_Types_ADDRESS_FAMILY]int{
		oc.Types_ADDRESS_FAMILY_IPV4: 0,
		oc.Types_ADDRESS_FAMILY_IPV6: 0,
	}

	for _, d := range descriptors {
		routesCount[oc.Types_ADDRESS_FAMILY_IPV4] += d.dimension[0] * d.dimension[1] * isisscalehelpers.ScaleRoutesPerBlock["v4"] * d.blockCount
		routesCount[oc.Types_ADDRESS_FAMILY_IPV6] += d.dimension[0] * d.dimension[1] * isisscalehelpers.ScaleRoutesPerBlock["v6"] * d.blockCount
	}

	for _, l := range dutAggData {
		routesCount[oc.Types_ADDRESS_FAMILY_IPV4] += len(l.SubInterfaces)
		routesCount[oc.Types_ADDRESS_FAMILY_IPV6] += len(l.SubInterfaces)
	}

	return routesCount
}

func createATEISISBlocks(descriptors []*descriptor) map[string]*otgconfighelpers.ISISOTGBlock {
	blocks := make(map[string]*otgconfighelpers.ISISOTGBlock)
	for _, d := range descriptors {
		for i := 1; i <= d.blockCount; i++ {
			blockName := d.name + "_" + strconv.Itoa(i)
			if strings.Contains(blockName, "Dynamic") {
				blocks[blockName] = otgconfighelpers.NewDynamicISISBlock(d.name, d.dimension[0], d.dimension[1], d.linkMultiplier, false)
			} else {
				blocks[blockName] = otgconfighelpers.NewStandardISISBlock(d.name, d.dimension[0], d.dimension[1], d.linkMultiplier)
			}
		}
	}

	return blocks
}

func TestISISScale(t *testing.T) {
	testInfo := initializeMultiAdjISISScaleTestData(t)
	isisscalehelpers.SetupISISScale(t, testInfo)

	dut := testInfo.DutData.DUT
	// Pre-test checks
	cases := []struct {
		desc string
	}{
		{
			desc: "MultiAdjISISScale",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			var count int
			var ok bool
			t.Logf("===========Conducting pre-test checks===========")
			// Check Aggregate on DUT are UP
			count, ok = isisscalehelpers.CheckIntsOpState(t, dut, 4*time.Minute)
			switch {
			case ok && count == testInfo.CorrectAggInterfaceCount:
				t.Logf("Check passed: All interfaces participating in ISIS are operationally up  need %v up interfaces got %v", testInfo.CorrectAggInterfaceCount, count)
			default:
				t.Fatalf("check failed: not all interfaces participating in ISIS are operationally up: need %v up interfaces got %v", testInfo.CorrectAggInterfaceCount, count)
			}

			// Check ISIS Adjacency
			if deviations.ISISAdjacencyStreamUnsupported(dut) {
				count, ok = isisscalehelpers.FindISISAdjCountNonStream(t, dut, 4*time.Minute, testInfo.CorrectISISAdjCount)
			} else {
				count, ok = isisscalehelpers.FindISISAdjCount(t, dut, 4*time.Minute, testInfo.CorrectISISAdjCount)
			}
			switch {
			case !ok:
				t.Fatalf("check failed: time limit exceeded: not all ISIS adjacencies are up : need %v up adjacencies got %v", testInfo.CorrectISISAdjCount, count)
			case count == testInfo.CorrectISISAdjCount:
				t.Logf("Check passed: All ISIS adjacencies are up  need %v up adjacencies got %v", testInfo.CorrectISISAdjCount, count)
			default:
				t.Fatalf("check failed: not all ISIS adjacencies are up : need %v up adjacencies got %v", testInfo.CorrectISISAdjCount, count)
			}

			t.Logf("===========Sleep for 5 minutes to check DUT stabilty===========")
			// Test will not check any metrics for 5 minutes to make sure DUT is stable.
			time.Sleep(5 * 60 * time.Second)

			// Checking ISIS LSPs count
			t.Run("LSP_Count", func(t *testing.T) {
				count, ok := isisscalehelpers.FindISISLSPCount(t, dut, 4*time.Minute, testInfo.CorrectISISLSPCount)
				if !ok {
					t.Errorf("check failed: incorrect ISIS LSP count need %v lsps got %v", testInfo.CorrectISISLSPCount, count)
					return
				}
				t.Logf("Check passed: correct ISIS LSP count need %v lsps got %v", testInfo.CorrectISISLSPCount, count)
			})

			// Checking routing table count
			t.Run("Route_Count", func(t *testing.T) {
				var wg sync.WaitGroup
				for _, f := range []oc.E_Types_ADDRESS_FAMILY{oc.Types_ADDRESS_FAMILY_IPV4, oc.Types_ADDRESS_FAMILY_IPV6} {
					family := f
					wg.Add(1)
					go func() {
						defer wg.Done()
						if deviations.AFTSummaryOCUnsupported(dut) {
							count, ok := isisscalehelpers.FindProtocolRouteCount(t, dut, family, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, 4*time.Minute, testInfo.CorrectIPRouteCount[family])
							if !ok {
								t.Errorf("check failed: incorrect %s route count need %v routes got %v", family.String(), testInfo.CorrectIPRouteCount[family], count)
								return
							}
							t.Logf("Check passed: correct %s route count need %v routes got %v", family.String(), testInfo.CorrectIPRouteCount[family], count)
						} else {
							count := isisscalehelpers.FindProtocolSummaryRouteCount(t, dut, family, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, 4*time.Minute, testInfo.CorrectIPRouteCount[family])
							if count >= testInfo.CorrectIPRouteCount[family] {
								t.Logf("Check passed: correct route count for the family %s need %v routes got %v", family.String(), testInfo.CorrectIPRouteCount[family], count)
							} else {
								t.Errorf("check failed: incorrect %s route count need %v routes got %v", family.String(), testInfo.CorrectIPRouteCount[family], count)
							}
						}
					}()
				}
				wg.Wait()
			})

			t.Run("Traffic_Loss", func(t *testing.T) {
				// Start and stop traffic
				testInfo.ATEData.ATE.OTG().StartTraffic(t)
				time.Sleep(60 * time.Second)
				testInfo.ATEData.ATE.OTG().StopTraffic(t)

				// Checking traffic loss
				isisscalehelpers.VerifyTrafficLoss(t, testInfo)
			})
		})
	}
}
