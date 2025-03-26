// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      hfdp://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fabric_redundancy_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

const (
	fabricType                = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC
	ipv6                      = "IPv6"
	ipv4PrefixLen             = 30
	ipv6PrefixLen             = 126
	mtu                       = 4000
	trafficStopWaitDuration   = 10 * time.Second
	acceptablePacketSizeDelta = 0.5
	acceptableLossPercent     = 0.001
	subInterfaceIndex         = 0
	ppsRate                   = 100000
	packetsToSend             = 16000000
)

var (
	fabricLeafOrValuePresent    = make(map[string][]any)
	fabricLeafOrValueNotPresent = make(map[string]string)
	dutSrc                      = &attrs.Attributes{
		Name:    "dutSrc",
		MAC:     "00:12:01:01:01:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	dutDst = &attrs.Attributes{
		Name:    "dutDst",
		MAC:     "00:12:02:01:01:01",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	ateSrc = &attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	ateDst = &attrs.Attributes{
		Name:    "ateDst",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}
	dutPorts = map[string]*attrs.Attributes{
		"port1": dutSrc,
		"port2": dutDst,
	}

	atePorts = map[string]*attrs.Attributes{
		"port1": ateSrc,
		"port2": ateDst,
	}

	fd = flowDefinition{
		name:     "flow_size_4000",
		desc:     "4000 byte flow",
		flowSize: 4000,
	}
)

type flowDefinition struct {
	name     string
	desc     string
	flowSize uint32
}

type otgData struct {
	flowProto string
	otg       *otg.OTG
	otgConfig gosnappi.Config
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func (d *otgData) waitInterface(t *testing.T) {
	otgutils.WaitForARP(t, d.otg, d.otgConfig, d.flowProto)
}

func createFlow(flowName string, flowSize uint32, ipv6 string) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(flowSize)
	flow.Rate().SetPps(ppsRate)
	flow.Duration().FixedPackets().SetPackets(packetsToSend)
	flow.TxRx().Device().
		SetTxNames([]string{fmt.Sprintf("%s.%s", ateSrc.Name, ipv6)}).
		SetRxNames([]string{fmt.Sprintf("%s.%s", ateDst.Name, ipv6)})
	ethHdr := flow.Packet().Add().Ethernet()
	ethHdr.Src().SetValue(ateSrc.MAC)
	flow.SetSize(gosnappi.NewFlowSize().SetFixed(flowSize))

	v6 := flow.Packet().Add().Ipv6()
	v6.Src().SetValue(ateSrc.IPv6)
	v6.Dst().SetValue(ateDst.IPv6)

	flow.EgressPacket().Add().Ethernet()

	return flow
}

func configureDUTPort(
	t *testing.T,
	dut *ondatra.DUTDevice,
	port *ondatra.Port,
	portAttrs *attrs.Attributes,
) {
	gnmi.Replace(
		t,
		dut,
		gnmi.OC().Interface(port.Name()).Config(),
		portAttrs.NewOCInterface(port.Name(), dut),
	)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, port)
	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, port.Name(), deviations.DefaultNetworkInstance(dut), subInterfaceIndex)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	for portName, portAfdrs := range dutPorts {
		port := dut.Port(t, portName)
		configureDUTPort(t, dut, port, portAfdrs)
		verifyDUTPort(t, dut, port.Name())
	}
}

func verifyDUTPort(t *testing.T, dut *ondatra.DUTDevice, portName string) {
	switch {
	case deviations.OmitL2MTU(dut):
		configuredIpv6SubInterfaceMtu := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Subinterface(subInterfaceIndex).Ipv6().Mtu().State())
		expectedSubInterfaceMtu := mtu

		if int(configuredIpv6SubInterfaceMtu) != expectedSubInterfaceMtu {
			t.Errorf(
				"dut %s configured mtu is incorrect, got: %d, want: %d",
				dut.Name(), configuredIpv6SubInterfaceMtu, expectedSubInterfaceMtu,
			)
		}
	default:
		configuredInterfaceMtu := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Mtu().State())
		expectedInterfaceMtu := mtu + 14

		if int(configuredInterfaceMtu) != expectedInterfaceMtu {
			t.Errorf(
				"dut %s configured mtu is incorrect, got: %d, want: %d",
				dut.Name(), configuredInterfaceMtu, expectedInterfaceMtu,
			)
		}
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()

	for portName, portAfdrs := range atePorts {
		port := ate.Port(t, portName)
		dutPort := dutPorts[portName]
		portAfdrs.AddToOTG(otgConfig, port, dutPort)
	}

	return otgConfig
}

func testFabricInventory(t *testing.T, dut *ondatra.DUTDevice, fabrics []string, od otgData) {
	for _, fabric := range fabrics {
		t.Logf("\n\n VALIDATE %s: \n\n", fabric)
		description := gnmi.OC().Component(fabric).Description()
		hardwareVersion := gnmi.OC().Component(fabric).HardwareVersion()
		id := gnmi.OC().Component(fabric).Id()
		mfgName := gnmi.OC().Component(fabric).MfgName()
		name := gnmi.OC().Component(fabric).Name()
		operStatus := gnmi.OC().Component(fabric).OperStatus()
		parent := gnmi.OC().Component(fabric).Parent()
		partNo := gnmi.OC().Component(fabric).PartNo()
		serialNo := gnmi.OC().Component(fabric).SerialNo()
		typeVal := gnmi.OC().Component(fabric).Type()
		location := gnmi.OC().Component(fabric).Location()
		lastReboofdime := gnmi.OC().Component(fabric).LastRebootTime()
		powerAdminState := gnmi.OC().Component(fabric).Fabric().PowerAdminState()

		descriptionKey := strings.Join([]string{fabric, "description"}, ":")
		hardwareVersionKey := strings.Join([]string{fabric, "hardware-version"}, ":")
		idKey := strings.Join([]string{fabric, "id"}, ":")
		mfgNameKey := strings.Join([]string{fabric, "mfg-name"}, ":")
		nameKey := strings.Join([]string{fabric, "name"}, ":")
		operStatusKey := strings.Join([]string{fabric, "oper-status"}, ":")
		parentKey := strings.Join([]string{fabric, "parent"}, ":")
		partNoKey := strings.Join([]string{fabric, "part-no"}, ":")
		serialNoKey := strings.Join([]string{fabric, "serial-no"}, ":")
		typeValKey := strings.Join([]string{fabric, "type"}, ":")
		locationKey := strings.Join([]string{fabric, "location"}, ":")
		lastReboofdimeKey := strings.Join([]string{fabric, "last-reboot-time"}, ":")
		powerAdminStateConfigKey := strings.Join([]string{fabric, "config/power-admin-state"}, ":")
		powerAdminStateStateKey := strings.Join([]string{fabric, "state/power-admin-state"}, ":")

		/* fabricLeafOrValuePresent: Key: fabric:leaf, Value: []any{isLeafPresent, leafValue} */
		fabricLeafOrValuePresent[descriptionKey] = []any{gnmi.Lookup(t, dut, description.State()).IsPresent()}
		fabricLeafOrValuePresent[hardwareVersionKey] = []any{gnmi.Lookup(t, dut, hardwareVersion.State()).IsPresent()}
		fabricLeafOrValuePresent[idKey] = []any{gnmi.Lookup(t, dut, id.State()).IsPresent()}
		fabricLeafOrValuePresent[mfgNameKey] = []any{gnmi.Lookup(t, dut, mfgName.State()).IsPresent()}
		fabricLeafOrValuePresent[nameKey] = []any{gnmi.Lookup(t, dut, name.State()).IsPresent()}
		fabricLeafOrValuePresent[operStatusKey] = []any{gnmi.Lookup(t, dut, operStatus.State()).IsPresent()}
		fabricLeafOrValuePresent[parentKey] = []any{gnmi.Lookup(t, dut, parent.State()).IsPresent()}
		fabricLeafOrValuePresent[partNoKey] = []any{gnmi.Lookup(t, dut, partNo.State()).IsPresent()}
		fabricLeafOrValuePresent[serialNoKey] = []any{gnmi.Lookup(t, dut, serialNo.State()).IsPresent()}
		fabricLeafOrValuePresent[typeValKey] = []any{gnmi.Lookup(t, dut, typeVal.State()).IsPresent()}
		fabricLeafOrValuePresent[locationKey] = []any{gnmi.Lookup(t, dut, location.State()).IsPresent()}
		fabricLeafOrValuePresent[lastReboofdimeKey] = []any{gnmi.Lookup(t, dut, lastReboofdime.State()).IsPresent()}
		fabricLeafOrValuePresent[powerAdminStateConfigKey] = []any{gnmi.Lookup(t, dut, powerAdminState.Config()).IsPresent()}
		fabricLeafOrValuePresent[powerAdminStateStateKey] = []any{gnmi.Lookup(t, dut, powerAdminState.State()).IsPresent()}

		for leaf, value := range fabricLeafOrValuePresent {
			if value[0].(bool) && strings.Contains(leaf, fabric) {
				switch leaf {
				case descriptionKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, description.State()))
				case hardwareVersionKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, hardwareVersion.State()))
				case idKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, id.State()))
				case mfgNameKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, mfgName.State()))
				case nameKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, name.State()))
				case operStatusKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, operStatus.State()))
				case parentKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, parent.State()))
				case partNoKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, partNo.State()))
				case serialNoKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, serialNo.State()))
				case typeValKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, typeVal.State()))
				case locationKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, location.State()))
				case lastReboofdimeKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, lastReboofdime.State()))
				case powerAdminStateConfigKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, powerAdminState.Config()))
				case powerAdminStateStateKey:
					fabricLeafOrValuePresent[leaf] = append(fabricLeafOrValuePresent[leaf], gnmi.Get(t, dut, powerAdminState.State()))
				}

				// Check if the leaf value is present or not. nil, '', 0 or empty slice is considered as not present.
				if fabricLeafOrValuePresent[leaf][1] == nil || reflect.ValueOf(fabricLeafOrValuePresent[leaf][1]).IsZero() || (reflect.ValueOf(fabricLeafOrValuePresent[leaf][1]).Kind() == reflect.Slice && reflect.ValueOf(fabricLeafOrValuePresent[leaf][1]).Len() == 0) {
					fabricLeafOrValueNotPresent[leaf] = fmt.Sprintf("value: '%v' is not as expected", fabricLeafOrValuePresent[leaf][1])
				} else {
					t.Logf("PASSED: leaf: '%v' value: '%v' is as expected", leaf, fabricLeafOrValuePresent[leaf][1])
				}
			} else if !value[0].(bool) && strings.Contains(leaf, fabric) {
				fabricLeafOrValueNotPresent[leaf] = "is not present"
			}
		}
	}

	t.Logf("\n\n")
	for leaf, value := range fabricLeafOrValueNotPresent {
		t.Errorf("[FAILED]: leaf: '%v' %v", leaf, value)
	}
}

func testFabricLastRebootTime(t *testing.T, dut *ondatra.DUTDevice, fabrics []string, od otgData) {
	// Create a new random source with a specific seed
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	// Generate a random index within the range of the slice
	randomIndex := random.Intn(len(fabrics))

	// Access the fabric at the random index
	fabric := fabrics[randomIndex]

	t.Logf("\n\n VALIDATE %s: \n\n", fabric)
	lastReboofdime := gnmi.OC().Component(fabric).LastRebootTime()
	lastReboofdimeBefore := gnmi.Get(t, dut, lastReboofdime.State())

	gnmi.Replace(t, dut, gnmi.OC().Component(fabric).Fabric().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_DISABLED)
	gnmi.Await(t, dut, gnmi.OC().Component(fabric).Fabric().PowerAdminState().State(), time.Minute, oc.Platform_ComponentPowerType_POWER_DISABLED)

	t.Logf("Waiting for 90s after power disable...")
	time.Sleep(90 * time.Second)

	gnmi.Replace(t, dut, gnmi.OC().Component(fabric).Fabric().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_ENABLED)

	if deviations.MissingValueForDefaults(dut) {
		time.Sleep(time.Minute)
	} else {
		if power, ok := gnmi.Await(t, dut, gnmi.OC().Component(fabric).Fabric().PowerAdminState().State(), time.Minute, oc.Platform_ComponentPowerType_POWER_ENABLED).Val(); !ok {
			t.Errorf("Component %s, power-admin-state got: %v, want: %v", fabric, power, oc.Platform_ComponentPowerType_POWER_ENABLED)
		}
	}
	if oper, ok := gnmi.Await(t, dut, gnmi.OC().Component(fabric).OperStatus().State(), 2*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE).Val(); !ok {
		t.Errorf("Component %s oper-status after POWER_ENABLED, got: %v, want: %v", fabric, oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	}

	t.Logf("Waiting for 90s after power enable...")
	time.Sleep(90 * time.Second)

	lastReboofdimeAfter := gnmi.Get(t, dut, lastReboofdime.State())

	if lastReboofdimeBefore > lastReboofdimeAfter {
		t.Errorf("Component %s, last-reboot-time before power disable is same as after power enable", fabric)
	}
}

func testFabricRedundancy(t *testing.T, dut *ondatra.DUTDevice, fabrics []string, od otgData) {
	t.Logf("Name: %s, Description: %s", fd.name, fd.desc)

	flowParams := createFlow(fd.name, fd.flowSize, od.flowProto)
	od.otgConfig.Flows().Clear()
	od.otgConfig.Flows().Append(flowParams)
	od.otg.PushConfig(t, od.otgConfig)
	time.Sleep(time.Second * 30)

	disabledFabric := ""
	// Create a new random source with a specific seed
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	// Generate a random index within the range of the slice
	randomIndex := random.Intn(len(fabrics))

	// Access the fabric at the random index
	disabledFabric = fabrics[randomIndex]

	gnmi.Replace(t, dut, gnmi.OC().Component(disabledFabric).Fabric().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_DISABLED)
	gnmi.Await(t, dut, gnmi.OC().Component(disabledFabric).Fabric().PowerAdminState().State(), time.Minute, oc.Platform_ComponentPowerType_POWER_DISABLED)

	t.Logf("Waiting for 90s after power disable...")
	time.Sleep(90 * time.Second)

	od.otg.StartProtocols(t)
	od.waitInterface(t)

	sleepTime := time.Duration(packetsToSend/uint32(ppsRate)) + 5
	od.otg.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)

	od.otg.StopTraffic(t)
	time.Sleep(trafficStopWaitDuration)

	otgutils.LogFlowMetrics(t, od.otg, od.otgConfig)

	flow := gnmi.OTG().Flow(fd.name)
	flowCounters := flow.Counters()

	outPkts := gnmi.Get(t, od.otg, flowCounters.OutPkts().State())
	inPkts := gnmi.Get(t, od.otg, flowCounters.InPkts().State())
	inOctets := gnmi.Get(t, od.otg, flowCounters.InOctets().State())

	if outPkts == 0 || inPkts == 0 {
		t.Error("flow did not send or receive any packets, this should not happen")
	}

	lossPercent := (float32(outPkts-inPkts) / float32(outPkts)) * 100

	if lossPercent > acceptableLossPercent {
		t.Errorf(
			"flow sent '%d' packets and received '%d' packets, resulting in a "+
				"loss percent of '%.2f'. max acceptable loss percent is '%.2f'",
			outPkts, inPkts, lossPercent, acceptableLossPercent,
		)
	}

	avgPacketSize := uint32(inOctets / inPkts)
	packetSizeDelta := float32(avgPacketSize-fd.flowSize) / (float32(avgPacketSize+fd.flowSize) / 2) * 100

	if packetSizeDelta > acceptablePacketSizeDelta {
		t.Errorf(
			"flow sent '%d' packets and received '%d' packets, resulting in "+
				"averagepacket size of '%d' and a packet size delta of '%.2f' percent. "+
				"packet size delta should not exceed '%.2f'",
			outPkts, inPkts, avgPacketSize, packetSizeDelta, acceptablePacketSizeDelta,
		)
	}

	gnmi.Replace(t, dut, gnmi.OC().Component(disabledFabric).Fabric().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_ENABLED)
	if deviations.MissingValueForDefaults(dut) {
		time.Sleep(time.Minute)
	} else {
		if power, ok := gnmi.Await(t, dut, gnmi.OC().Component(disabledFabric).Fabric().PowerAdminState().State(), time.Minute, oc.Platform_ComponentPowerType_POWER_ENABLED).Val(); !ok {
			t.Errorf("Component %s, power-admin-state got: %v, want: %v", disabledFabric, power, oc.Platform_ComponentPowerType_POWER_ENABLED)
		}
	}
	if oper, ok := gnmi.Await(t, dut, gnmi.OC().Component(disabledFabric).OperStatus().State(), 2*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE).Val(); !ok {
		t.Errorf("Component %s oper-status after POWER_ENABLED, got: %v, want: %v", disabledFabric, oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	}

	t.Logf("Waiting for 90s after power enable...")
	time.Sleep(90 * time.Second)

}

func TestFabricRedundancy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Get fabric list that are inserted in the DUT.
	fabrics := components.FindComponentsByType(t, dut, fabricType)
	t.Logf("Found fabric list: %v", fabrics)
	index := 0
	for _, fabric := range fabrics {
		empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(fabric).Empty().State()).Val()
		if !(ok && empty) {
			fabrics[index] = fabric
			index++
		} else {
			t.Logf("Fabric Component %s is empty, hence skipping", fabric)
		}
	}
	fabrics = fabrics[:index]
	t.Logf("Found non-empty fabric list: %v", fabrics)

	// Validate that there are at least 2 non-empty fabrics in the DUT.
	if len(fabrics) < 2 {
		t.Fatalf("No of Fabrics on DUT (%q): got %v, want => 2", dut.Model(), len(fabrics))
	}

	/* configure the ATE device to send/receive 16 millions of packets
	   at 100kpps rate and using 4000B packets (with 10E-6 tolerance). */
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	configureDUT(t, dut)
	otgConfig := configureATE(t, ate)
	od := otgData{
		flowProto: ipv6,
		otg:       otg,
		otgConfig: otgConfig,
	}

	// Cleanup the DUT config.
	t.Cleanup(func() {
		deleteBatch := &gnmi.SetBatch{}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			netInst := &oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}

			for portName := range dutPorts {
				gnmi.BatchDelete(
					deleteBatch,
					gnmi.OC().
						NetworkInstance(*netInst.Name).
						Interface(fmt.Sprintf("%s.%d", dut.Port(t, portName).Name(), subInterfaceIndex)).
						Config(),
				)
			}
		}

		for portName := range dutPorts {
			gnmi.BatchDelete(
				deleteBatch,
				gnmi.OC().
					Interface(dut.Port(t, portName).Name()).
					Subinterface(subInterfaceIndex).
					Config(),
			)
			gnmi.BatchDelete(deleteBatch, gnmi.OC().Interface(dut.Port(t, portName).Name()).Mtu().Config())
		}
		deleteBatch.Set(t, dut)
	})

	// Test cases.
	type testCase struct {
		name     string
		fabrics  []string
		testFunc func(t *testing.T, dut *ondatra.DUTDevice, fabrics []string, od otgData)
	}

	testCases := []testCase{
		{
			name:     "TEST 1: Fabric inventory",
			fabrics:  fabrics,
			testFunc: testFabricInventory,
		},
		{
			name:     "TEST 2: Fabric redundancy",
			fabrics:  fabrics,
			testFunc: testFabricRedundancy,
		},
		{
			name:     "TEST 3: Fabric last reboot time",
			fabrics:  fabrics,
			testFunc: testFabricLastRebootTime,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.name)
			tc.testFunc(t, dut, tc.fabrics, od)
		})
	}
}
