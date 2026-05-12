// Copyright 2026 Google LLC
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
// Package telemetry_interfaces_test implements "gNMI-1.28"
package telemetry_interfaces_test

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// TestMain sets up the test environment.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func testTelemetryInterfacesStateCounters(t *testing.T, dut *ondatra.DUTDevice, ports []string) {
	t.Helper()
	p := gnmi.OC()

	intfStatePassedCounters := make(map[string][]any)
	intfStateFailedCounters := make(map[string]string)

	for _, port := range ports {
		t.Logf("\n\n Iteration on port %s on DUT %s: \n\n", port, dut.Name())

		inOctets := p.Interface(port).Counters().InOctets().State()
		outOctets := p.Interface(port).Counters().OutOctets().State()
		inUnicastPkts := p.Interface(port).Counters().InUnicastPkts().State()
		outUnicastPkts := p.Interface(port).Counters().OutUnicastPkts().State()
		inBroadcastPkts := p.Interface(port).Counters().InBroadcastPkts().State()
		outBroadcastPkts := p.Interface(port).Counters().OutBroadcastPkts().State()
		inMulticastPkts := p.Interface(port).Counters().InMulticastPkts().State()
		outMulticastPkts := p.Interface(port).Counters().OutMulticastPkts().State()
		inDiscards := p.Interface(port).Counters().InDiscards().State()
		outDiscards := p.Interface(port).Counters().OutDiscards().State()
		inErrors := p.Interface(port).Counters().InErrors().State()
		outErrors := p.Interface(port).Counters().OutErrors().State()

		inOctetsKey := strings.Join([]string{port, "in-octets"}, ":")
		outOctetsKey := strings.Join([]string{port, "out-octets"}, ":")
		inUnicastPktsKey := strings.Join([]string{port, "in-unicast-pkts"}, ":")
		outUnicastPktsKey := strings.Join([]string{port, "out-unicast-pkts"}, ":")
		inBroadcastPktsKey := strings.Join([]string{port, "in-broadcast-pkts"}, ":")
		outBroadcastPktsKey := strings.Join([]string{port, "out-broadcast-pkts"}, ":")
		inMulticastPktsKey := strings.Join([]string{port, "in-multicast-pkts"}, ":")
		outMulticastPktsKey := strings.Join([]string{port, "out-multicast-pkts"}, ":")
		inDiscardsKey := strings.Join([]string{port, "in-discards"}, ":")
		outDiscardsKey := strings.Join([]string{port, "out-discards"}, ":")
		inErrorsKey := strings.Join([]string{port, "in-errors"}, ":")
		outErrorsKey := strings.Join([]string{port, "out-errors"}, ":")

		/* intfStatePassed: Key: port:leaf, Value: []any{isLeafPresent, leafValue} */
		intfStatePassedCounters[inOctetsKey] = []any{gnmi.Lookup(t, dut, inOctets).IsPresent()}
		intfStatePassedCounters[outOctetsKey] = []any{gnmi.Lookup(t, dut, outOctets).IsPresent()}
		intfStatePassedCounters[inUnicastPktsKey] = []any{gnmi.Lookup(t, dut, inUnicastPkts).IsPresent()}
		intfStatePassedCounters[outUnicastPktsKey] = []any{gnmi.Lookup(t, dut, outUnicastPkts).IsPresent()}
		intfStatePassedCounters[inBroadcastPktsKey] = []any{gnmi.Lookup(t, dut, inBroadcastPkts).IsPresent()}
		intfStatePassedCounters[outBroadcastPktsKey] = []any{gnmi.Lookup(t, dut, outBroadcastPkts).IsPresent()}
		intfStatePassedCounters[inMulticastPktsKey] = []any{gnmi.Lookup(t, dut, inMulticastPkts).IsPresent()}
		intfStatePassedCounters[outMulticastPktsKey] = []any{gnmi.Lookup(t, dut, outMulticastPkts).IsPresent()}
		intfStatePassedCounters[inDiscardsKey] = []any{gnmi.Lookup(t, dut, inDiscards).IsPresent()}
		intfStatePassedCounters[outDiscardsKey] = []any{gnmi.Lookup(t, dut, outDiscards).IsPresent()}
		intfStatePassedCounters[inErrorsKey] = []any{gnmi.Lookup(t, dut, inErrors).IsPresent()}
		intfStatePassedCounters[outErrorsKey] = []any{gnmi.Lookup(t, dut, outErrors).IsPresent()}

		for leaf, value := range intfStatePassedCounters {
			if value[0].(bool) && strings.Contains(leaf, port) {
				switch leaf {
				case inOctetsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, inOctets))
				case outOctetsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, outOctets))
				case inUnicastPktsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, inUnicastPkts))
				case outUnicastPktsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, outUnicastPkts))
				case inBroadcastPktsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, inBroadcastPkts))
				case outBroadcastPktsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, outBroadcastPkts))
				case inMulticastPktsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, inMulticastPkts))
				case outMulticastPktsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, outMulticastPkts))
				case inDiscardsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, inDiscards))
				case outDiscardsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, outDiscards))
				case inErrorsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, inErrors))
				case outErrorsKey:
					intfStatePassedCounters[leaf] = append(intfStatePassedCounters[leaf], gnmi.Get(t, dut, outErrors))
				}

				// Check if the leaf value is present or not. nil, '', 0 or empty slice is considered as not present.
				if intfStatePassedCounters[leaf][1] == nil || reflect.ValueOf(intfStatePassedCounters[leaf][1]).IsZero() || (reflect.ValueOf(intfStatePassedCounters[leaf][1]).Kind() == reflect.Slice && reflect.ValueOf(intfStatePassedCounters[leaf][1]).Len() == 0) {
					intfStateFailedCounters[leaf] = fmt.Sprintf("value: '%v' is not as expected", intfStatePassedCounters[leaf][1])
				} else {
					t.Logf("[PASSED]: leaf: '%v' value: '%v' is as expected", leaf, intfStatePassedCounters[leaf][1])
				}
			} else if !value[0].(bool) && strings.Contains(leaf, port) {
				intfStateFailedCounters[leaf] = "is not present"
			}
		}
	}

	t.Logf("\n\n")
	for leaf, value := range intfStateFailedCounters {
		t.Errorf("[FAILED]: leaf: '%v' %v", leaf, value)
	}
	t.Logf("\n\n")
}

func testTelemetryInterfacesState(t *testing.T, dut *ondatra.DUTDevice, ports []string) {
	t.Helper()
	p := gnmi.OC()

	intfStatePassed := make(map[string][]any)
	intfStateFailed := make(map[string]string)

	for _, port := range ports {
		t.Logf("\n\n Iteration on port %s on DUT %s: \n\n", port, dut.Name())
		lastChange := p.Interface(port).LastChange().State()
		operStatus := p.Interface(port).OperStatus().State()
		adminStatus := p.Interface(port).AdminStatus().State()
		description := p.Interface(port).Description().State()

		lastChangeKey := strings.Join([]string{port, "last-change"}, ":")
		operStatusKey := strings.Join([]string{port, "oper-status"}, ":")
		adminStatusKey := strings.Join([]string{port, "admin-status"}, ":")
		descriptionKey := strings.Join([]string{port, "description"}, ":")

		/* intfStatePassed: Key: port:leaf, Value: []any{isLeafPresent, leafValue} */
		intfStatePassed[lastChangeKey] = []any{gnmi.Lookup(t, dut, lastChange).IsPresent()}
		intfStatePassed[operStatusKey] = []any{gnmi.Lookup(t, dut, operStatus).IsPresent()}
		intfStatePassed[adminStatusKey] = []any{gnmi.Lookup(t, dut, adminStatus).IsPresent()}
		intfStatePassed[descriptionKey] = []any{gnmi.Lookup(t, dut, description).IsPresent()}

		for leaf, value := range intfStatePassed {
			if value[0].(bool) && strings.Contains(leaf, port) {
				switch leaf {
				case lastChangeKey:
					intfStatePassed[leaf] = append(intfStatePassed[leaf], gnmi.Get(t, dut, lastChange))
				case operStatusKey:
					intfStatePassed[leaf] = append(intfStatePassed[leaf], gnmi.Get(t, dut, operStatus))
				case adminStatusKey:
					intfStatePassed[leaf] = append(intfStatePassed[leaf], gnmi.Get(t, dut, adminStatus))
				case descriptionKey:
					intfStatePassed[leaf] = append(intfStatePassed[leaf], gnmi.Get(t, dut, description))
				}
				// Check if the leaf value is present or not. nil, '', 0 or empty slice is considered as not present.
				if intfStatePassed[leaf][1] == nil || reflect.ValueOf(intfStatePassed[leaf][1]).IsZero() || (reflect.ValueOf(intfStatePassed[leaf][1]).Kind() == reflect.Slice && reflect.ValueOf(intfStatePassed[leaf][1]).Len() == 0) {
					intfStateFailed[leaf] = fmt.Sprintf("value: '%v' is not as expected", intfStatePassed[leaf][1])
				} else {
					t.Logf("[PASSED]: leaf: '%v' value: '%v' is as expected", leaf, intfStatePassed[leaf][1])
				}
			} else if !value[0].(bool) && strings.Contains(leaf, port) {
				intfStateFailed[leaf] = "is not present"
			}
		}
	}

	t.Logf("\n\n")
	for leaf, value := range intfStateFailed {
		t.Errorf("[FAILED]: leaf: '%v' %v", leaf, value)
	}
	t.Logf("\n\n")
}

func testTelemetryInterfacesAggregation(t *testing.T, dut *ondatra.DUTDevice, ports []string) {
	t.Helper()
	p := gnmi.OC()

	intfAggregationPassed := make(map[string][]any)
	intfAggregationFailed := make(map[string]string)

	for _, port := range ports {
		t.Logf("\n\n Iteration on port %s on DUT %s: \n\n", port, dut.Name())

		t.Run("test_interfaces_aggregation_config", func(t *testing.T) {
			i := &oc.Interface{Name: ygot.String(port)}
			// Configure link aggregation using gNMI update.
			i.Aggregation = &oc.Interface_Aggregation{
				LagType: oc.IfAggregate_AggregationType_STATIC,
			}
			gnmi.Update(t, dut, gnmi.OC().Interface(port).Aggregation().Config(), i.Aggregation)
			t.Logf("Waiting for commit to take effect")
			time.Sleep(2 * time.Minute)
		})

		t.Run("test_interfaces_aggregation_st", func(t *testing.T) {
			member := p.Interface(port).Aggregation().Member().State()
			lagType := p.Interface(port).Aggregation().LagType().Config()

			memberKey := strings.Join([]string{port, "aggregation/state/member"}, ":")
			lagTypeKey := strings.Join([]string{port, "aggregation/config/lag-type"}, ":")

			/* intfAggregationPassed: Key: port:leaf, Value: []any{isLeafPresent, leafValue} */
			intfAggregationPassed[memberKey] = []any{gnmi.Lookup(t, dut, member).IsPresent()}
			intfAggregationPassed[lagTypeKey] = []any{gnmi.Lookup(t, dut, lagType).IsPresent()}

			for leaf, value := range intfAggregationPassed {
				if value[0].(bool) && strings.Contains(leaf, port) {
					switch leaf {
					case memberKey:
						intfAggregationPassed[leaf] = append(intfAggregationPassed[leaf], gnmi.Get(t, dut, member))
					case lagTypeKey:
						intfAggregationPassed[leaf] = append(intfAggregationPassed[leaf], gnmi.Get(t, dut, lagType))
					}
					// Check if the leaf value is present or not. nil, '', 0 or empty slice is considered as not present.
					if intfAggregationPassed[leaf][1] == nil || reflect.ValueOf(intfAggregationPassed[leaf][1]).IsZero() || (reflect.ValueOf(intfAggregationPassed[leaf][1]).Kind() == reflect.Slice && reflect.ValueOf(intfAggregationPassed[leaf][1]).Len() == 0) {
						intfAggregationFailed[leaf] = fmt.Sprintf("value: '%v' is not as expected", intfAggregationPassed[leaf][1])
					} else {
						t.Logf("[PASSED]: leaf: '%v' value: '%v' is as expected", leaf, intfAggregationPassed[leaf][1])
					}
				} else if !value[0].(bool) && strings.Contains(leaf, port) {
					intfAggregationFailed[leaf] = "is not present"
				}
			}
			t.Logf("\n\n")
			for leaf, value := range intfAggregationFailed {
				t.Errorf("[FAILED]: leaf: '%v' %v", leaf, value)
			}
			t.Logf("\n\n")
		})
		t.Run("test_interfaces_aggregation_delete", func(t *testing.T) {
			gnmi.Delete(t, dut, gnmi.OC().Interface(port).Aggregation().Config())
			t.Logf("Waiting for commit to take effect")
			time.Sleep(2 * time.Minute)
		})
	}
}

func testTelemetryInterfacesStateRate(t *testing.T, dut *ondatra.DUTDevice, ports []string) {
	t.Helper()
	p := gnmi.OC()

	intfStateRatePassed := make(map[string][]any)
	intfStateRateFailed := make(map[string]string)

	for _, port := range ports {
		t.Logf("\n\n Iteration on port %s on DUT %s: \n\n", port, dut.Name())
		inRate := p.Interface(port).InRate().State()
		outRate := p.Interface(port).OutRate().State()

		inRateKey := strings.Join([]string{port, "in-rate"}, ":")
		outRateKey := strings.Join([]string{port, "out-rate"}, ":")

		/* intfStatePassed: Key: port:leaf, Value: []any{isLeafPresent, leafValue} */
		intfStateRatePassed[inRateKey] = []any{gnmi.Lookup(t, dut, inRate).IsPresent()}
		intfStateRatePassed[outRateKey] = []any{gnmi.Lookup(t, dut, outRate).IsPresent()}

		for leaf, value := range intfStateRatePassed {
			if value[0].(bool) && strings.Contains(leaf, port) {
				switch leaf {
				case inRateKey:
					intfStateRatePassed[leaf] = append(intfStateRatePassed[leaf], gnmi.Get(t, dut, inRate))
				case outRateKey:
					intfStateRatePassed[leaf] = append(intfStateRatePassed[leaf], gnmi.Get(t, dut, outRate))
				}
				// Check if the leaf value is present or not. nil, '', 0 or empty slice is considered as not present.
				if intfStateRatePassed[leaf][1] == nil || reflect.ValueOf(intfStateRatePassed[leaf][1]).IsZero() || (reflect.ValueOf(intfStateRatePassed[leaf][1]).Kind() == reflect.Slice && reflect.ValueOf(intfStateRatePassed[leaf][1]).Len() == 0) {
					intfStateRateFailed[leaf] = fmt.Sprintf("value: '%v' is not as expected", intfStateRatePassed[leaf][1])
				} else {
					t.Logf("[PASSED]: leaf: '%v' value: '%v' is as expected", leaf, intfStateRatePassed[leaf][1])
				}
			} else if !value[0].(bool) && strings.Contains(leaf, port) {
				intfStateRateFailed[leaf] = "is not present"
			}
		}
	}

	t.Logf("\n\n")
	for leaf, value := range intfStateRateFailed {
		t.Errorf("[FAILED]: leaf: '%v' %v", leaf, value)
	}
	t.Logf("\n\n")
}

func testTelemetryInterfacesStateSubinterface(t *testing.T, dut *ondatra.DUTDevice, ports []string) {
	t.Helper()
	p := gnmi.OC()
	subIntfIndex := uint32(100)
	description := "test description"

	for _, port := range ports {
		t.Logf("\n\n Iteration on port %s on DUT %s: \n\n", port, dut.Name())

		i := &oc.Interface_Subinterface{Index: ygot.Uint32(subIntfIndex)}
		gnmi.Update(t, dut, p.Interface(port).Subinterface(subIntfIndex).Config(), i)
		time.Sleep(2 * time.Minute)

		subIntf := p.Interface(port).Subinterface(subIntfIndex)
		if gnmi.Lookup(t, dut, subIntf.Index().State()).IsPresent() {
			indexValue := gnmi.Get(t, dut, subIntf.Index().State())
			if indexValue == subIntfIndex {

				t.Logf("\n\n [PASSED]: port: '%s' subinterface: '%v' index value: '%v' is as expected \n\n", port, subIntfIndex, indexValue)
			} else {
				t.Errorf("\n\n [FAILED]: Subinterface index value got: '%v' expected: '%v' on port '%s' subinterface: '%v'\n\n", indexValue, subIntfIndex, port, subIntfIndex)
			}
		} else {
			t.Errorf("\n\n [FAILED]: leaf: Subinterface index is not present on port %s subinterface: '%v'\n\n", port, subIntfIndex)
		}

		if gnmi.Lookup(t, dut, subIntf.OperStatus().State()).IsPresent() {
			operStatusValue := gnmi.Get(t, dut, subIntf.OperStatus().State())
			t.Logf("\n\n [PASSED]: port: '%s' subinterface: '%v' oper-status value: '%v' is present \n\n", port, subIntfIndex, operStatusValue)
		} else {
			t.Errorf("\n\n [FAILED]: leaf: Subinterface oper-status is not present on port %s subinterface: '%v'\n\n", port, subIntfIndex)
		}

		subinterface := p.Interface(port).Subinterface(subIntfIndex)

		gnmi.Update(t, dut, subinterface.Description().Config(), description)
		time.Sleep(2 * time.Minute)

		if gnmi.Lookup(t, dut, subinterface.Description().State()).IsPresent() {
			descriptionValue := gnmi.Get(t, dut, subinterface.Description().State())
			if descriptionValue == description {
				t.Logf("\n\n [PASSED]: port: '%s' subinterface: '%v' description value: '%v' is as expected \n\n", port, subIntfIndex, descriptionValue)
			} else {
				t.Errorf("\n\n [FAILED]: Subinterface description value got: '%v' expected: 'test-description' on port '%s' subinterface: '%v' \n\n", descriptionValue, port, subinterface)
			}
		} else {
			t.Errorf("\n\n [FAILED]: leaf: Subinterface description is not present on port %s subinterface: '%v' \n\n", port, subinterface)
		}

		gnmi.Delete(t, dut, subinterface.Config())
	}
}

func testTelemetryInterfacesConfig(t *testing.T, dut *ondatra.DUTDevice, ports []string) {
	t.Helper()
	p := gnmi.OC()
	macAddressConfig := "00:00:00:00:00:01"
	for index, port := range ports {
		t.Logf("\n\n Iteration on port %s on DUT %s: \n\n", port, dut.Name())

		t.Run("test_telemetry_interfaces_config_enabled", func(t *testing.T) {
			i := &oc.Interface{Name: ygot.String(port)}
			if deviations.ExplicitPortSpeed(dut) {
				fptest.SetPortSpeed(t, dut.Port(t, "port"+strconv.Itoa(index+1)))
			}
			i.Enabled = ygot.Bool(true)
			gnmi.Update(t, dut, p.Interface(port).Config(), i)
			time.Sleep(2 * time.Minute)

			enabledValue := gnmi.Get(t, dut, p.Interface(port).Enabled().State())
			if enabledValue == true {
				t.Logf("\n\n [PASSED]: port: '%s' enabled value: '%v' is as expected \n\n", port, enabledValue)
			} else {
				t.Errorf("\n\n [FAILED]: Interface enabled value got: '%v' expected: 'true' on port '%s' \n\n", enabledValue, port)
			}
		})

		t.Run("test_telemetry_interfaces_config_type", func(t *testing.T) {
			i := &oc.Interface{Name: ygot.String(port)}
			if deviations.ExplicitPortSpeed(dut) {
				fptest.SetPortSpeed(t, dut.Port(t, "port"+strconv.Itoa(index+1)))
			}
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			gnmi.Update(t, dut, p.Interface(port).Config(), i)
			time.Sleep(2 * time.Minute)

			intfTypeValue := gnmi.Get(t, dut, p.Interface(port).Type().State())
			if intfTypeValue == oc.IETFInterfaces_InterfaceType_ethernetCsmacd {
				t.Logf("\n\n [PASSED]: port: '%s' interface type value: '%v' is as expected \n\n", port, intfTypeValue)
			} else {
				t.Errorf("\n\n [FAILED]: Interface type value got: '%v' expected: '%v' on port '%s' \n\n", intfTypeValue, oc.IETFInterfaces_InterfaceType_ethernetCsmacd, port)
			}
		})

		t.Run("test_telemetry_interfaces_config_mac_address", func(t *testing.T) {
			macAddress := gnmi.Get(t, dut, p.Interface(port).Ethernet().MacAddress().State())
			t.Logf("macAddress: %s", macAddress)

			i := &oc.Interface{Name: ygot.String(port)}
			if deviations.ExplicitPortSpeed(dut) {
				fptest.SetPortSpeed(t, dut.Port(t, "port"+strconv.Itoa(index+1)))
			}
			i.Ethernet.MacAddress = ygot.String(macAddressConfig)
			gnmi.Update(t, dut, p.Interface(port).Config(), i)
			time.Sleep(2 * time.Minute)

			macAddressValue := gnmi.Get(t, dut, p.Interface(port).Ethernet().MacAddress().State())
			if macAddressValue == macAddressConfig {
				t.Logf("\n\n [PASSED]: port: '%s' mac address value: '%v' is as expected \n\n", port, macAddressValue)
			} else {
				t.Errorf("\n\n [FAILED]: Interface mac address value got: '%v' expected: '%v' on port '%s' \n\n", macAddressValue, macAddressConfig, port)
			}

			gnmi.Update(t, dut, p.Interface(port).Ethernet().MacAddress().Config(), macAddress)
			time.Sleep(2 * time.Minute)
		})
	}
}

// TestTelemetryInterfaces tests interfaces oc paths.
func TestTelemetryInterfaces(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	port1 := strings.ToLower(dut.Port(t, "port1").Name())

	// Test cases.
	type testCase struct {
		name     string
		ports    []string
		testFunc func(t *testing.T, dut *ondatra.DUTDevice, ports []string)
	}

	testCases := []testCase{
		{
			name:     "TEST1: Test_Telemetry_Interfaces_State",
			ports:    []string{port1},
			testFunc: testTelemetryInterfacesState,
		},
		{
			name:     "TEST2: Test_Telemetry_Interfaces_State_Counters",
			ports:    []string{port1},
			testFunc: testTelemetryInterfacesStateCounters,
		},
		{
			name:     "TEST3: Test_Telemetry_Interfaces_State_Rate",
			ports:    []string{port1},
			testFunc: testTelemetryInterfacesStateRate,
		},
		{
			name:     "TEST4: Test_Telemetry_Interfaces_State_Subinterface",
			ports:    []string{port1},
			testFunc: testTelemetryInterfacesStateSubinterface,
		},
		{
			name:     "TEST5: Test_Telemetry_Interfaces_Config",
			ports:    []string{port1},
			testFunc: testTelemetryInterfacesConfig,
		},
		{
			name:     "TEST6: Test_Telemetry_Interfaces_Aggregation",
			ports:    []string{port1},
			testFunc: testTelemetryInterfacesAggregation,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.name)
			tc.testFunc(t, dut, tc.ports)
		})
	}
}
