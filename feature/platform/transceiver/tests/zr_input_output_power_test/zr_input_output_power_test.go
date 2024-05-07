// Copyright 2024 Google LLC
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

package zr_input_output_power_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	gnps "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

const (
	opticalChannelTransceiverType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_OPTICAL_CHANNEL
	maxRebootTime                 = 900
	maxCompWaitTime               = 600
	samplingTime                  = 10000000000 // 10 Seconds.
	targetOutputPower             = -9
	interfaceFlapTimeOut          = 30 // Seconds.
	outputFreqLowerBound          = 184500000
	outputFreqUpperBound          = 196000000
	rxSignalPowerLowerBound       = -14
	rxSignalPowerUpperBound       = 0
	txOutputPowerLowerBound       = -10
	txOutputPowerUpperBound       = -6
)

type testData struct {
	transceiverName               string
	dut                           *ondatra.DUTDevice
	transceiverOpticalChannelName string
	interfaceName                 string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Removes any breakout configuration if present, and configures the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, transceiverName string) {
	port := dut.Port(t, "port1")
	// Remove any breakout configuration.
	hardwareComponentName := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).HardwarePort().State())
	gnmi.Delete(t, dut, gnmi.OC().Component(hardwareComponentName).Port().BreakoutMode().Config())

	i1 := &oc.Interface{Name: ygot.String(port.Name())}
	i1.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.ExplicitPortSpeed(dut) {
		i1.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, port)
	}

	gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i1)
	gnmi.Replace(t, dut, gnmi.OC().Component(transceiverName).Transceiver().Enabled().Config(), *ygot.Bool(true))
	t.Logf("Configured port %v", port.Name())
}

// Verifies valid Rx Signal power.
func verifyValidRxInputPower(t testing.TB, transceiverOpticalChanelInputPower *oc.Component_OpticalChannel_InputPower, isInterfaceDisabled bool, transceiverOpticalChannelName string, transceiverName string) {
	t.Logf("Checking Transceiver = %v  Optical Channel Input Power Statistics", transceiverOpticalChannelName)
	t.Logf("Optical channel = %v , Instant Input Power = %v", transceiverOpticalChannelName, transceiverOpticalChanelInputPower.GetInstant())
	t.Logf("Optical channel = %v , Average Input Power = %v", transceiverOpticalChannelName, transceiverOpticalChanelInputPower.GetAvg())
	t.Logf("Optical channel = %v , Min Input Power = %v", transceiverOpticalChannelName, transceiverOpticalChanelInputPower.GetMin())
	t.Logf("Optical channel = %v , Max Input Power = %v", transceiverOpticalChannelName, transceiverOpticalChanelInputPower.GetMax())

	if transceiverInputPowerInstantType := reflect.TypeOf(transceiverOpticalChanelInputPower.GetInstant()).Kind(); transceiverInputPowerInstantType != reflect.Float64 {
		t.Fatalf("[Error]: Expected Optical Channel %v  Instant InputPower  data type = %v  , Got =%v", transceiverOpticalChannelName, reflect.Float64, transceiverInputPowerInstantType)
	}
	if transceiverInputPowerAvgType := reflect.TypeOf(transceiverOpticalChanelInputPower.GetAvg()).Kind(); transceiverInputPowerAvgType != reflect.Float64 {
		t.Fatalf("[Error]: Expected Optical Channel %v  Avg InputPower data type = %v  , Got =%v", transceiverOpticalChannelName, reflect.Float64, transceiverInputPowerAvgType)
	}
	if transceiverInputPowerMinType := reflect.TypeOf(transceiverOpticalChanelInputPower.GetMin()).Kind(); transceiverInputPowerMinType != reflect.Float64 {
		t.Fatalf("[Error]: Expected Optical Channel %v  Min InputPower data type = %v  , Got =%v", transceiverOpticalChannelName, reflect.Float64, transceiverInputPowerMinType)
	}
	if transceiverInputPowerMaxType := reflect.TypeOf(transceiverOpticalChanelInputPower.GetMax()).Kind(); transceiverInputPowerMaxType != reflect.Float64 {
		t.Fatalf("[Error]: Expected Optical Channel %v Max InputPower data type = %v  , Got =%v", transceiverOpticalChannelName, reflect.Float64, transceiverInputPowerMaxType)
	}

	if isInterfaceDisabled {
		if transceiverOpticalChanelInputPower.GetInstant() > 0 {
			t.Fatalf("[Error]: Expected Optical Channel %v  Instant InputPower   = %v  ,Got =%v", transceiverOpticalChannelName, 0, transceiverOpticalChanelInputPower.GetInstant())
		}
		if transceiverOpticalChanelInputPower.GetAvg() > 0 {
			t.Fatalf("[Error]: Expected Optical Channel %v  Avg InputPower   = %v  ,Got =%v", transceiverOpticalChannelName, 0, transceiverOpticalChanelInputPower.GetAvg())
		}
		if transceiverOpticalChanelInputPower.GetMin() > 0 {
			t.Fatalf("[Error]: Expected Optical Channel %v  Min InputPower   = %v  ,Got =%v", transceiverOpticalChannelName, 0, transceiverOpticalChanelInputPower.GetMin())
		}
		if transceiverOpticalChanelInputPower.GetMax() > 0 {
			t.Fatalf("[Error]: Expected Optical Channel %v  Max InputPower   = %v  ,Got =%v", transceiverOpticalChannelName, 0, transceiverOpticalChanelInputPower.GetMax())
		}

	} else if transceiverOpticalChanelInputPower.GetMin() < rxSignalPowerLowerBound || transceiverOpticalChanelInputPower.GetMax() > rxSignalPowerUpperBound {
		t.Fatalf("Transciever %v RX Input power range Expected = %v to %v dbm, Got = %v to %v  ", transceiverName, rxSignalPowerLowerBound, rxSignalPowerUpperBound, transceiverOpticalChanelInputPower.GetMin(), transceiverOpticalChanelInputPower.GetMax())
	} else if transceiverOpticalChanelInputPower.GetMin() > transceiverOpticalChanelInputPower.GetAvg() || transceiverOpticalChanelInputPower.GetAvg() > transceiverOpticalChanelInputPower.GetMax() || transceiverOpticalChanelInputPower.GetMin() > transceiverOpticalChanelInputPower.GetInstant() || transceiverOpticalChanelInputPower.GetInstant() > transceiverOpticalChanelInputPower.GetMax() {
		t.Fatalf("Transciever %v RX Input power not following min <= avg/instant <= max. Got instant = %v ,min= %v , avg= %v , max =%v  ", transceiverName, transceiverOpticalChanelInputPower.GetInstant(), transceiverOpticalChanelInputPower.GetMin(), transceiverOpticalChanelInputPower.GetAvg(), transceiverOpticalChanelInputPower.GetMax())
	}
}

// Verifies valid Tx Output Power.
func verifyValidTxOutputPower(t testing.TB, transceiverOpticalChanelOutputPower *oc.Component_OpticalChannel_OutputPower, isInterfaceDisabled bool, transceiverOpticalChannelName string, transceiverName string) {
	t.Logf("Checking Transceiver = %v  Optical Channel Output Power Statistics", transceiverOpticalChannelName)
	t.Logf("Optical channel = %v , Instant Output Power = %v", transceiverOpticalChannelName, transceiverOpticalChanelOutputPower.GetInstant())
	t.Logf("Optical channel = %v , Average Output Power = %v", transceiverOpticalChannelName, transceiverOpticalChanelOutputPower.GetAvg())
	t.Logf("Optical channel = %v , Min Output Power = %v", transceiverOpticalChannelName, transceiverOpticalChanelOutputPower.GetMin())
	t.Logf("Optical channel = %v , Max Output Power = %v", transceiverOpticalChannelName, transceiverOpticalChanelOutputPower.GetMax())
	if transceiverOutputPowerInstantType := reflect.TypeOf(transceiverOpticalChanelOutputPower.GetInstant()).Kind(); transceiverOutputPowerInstantType != reflect.Float64 {
		t.Fatalf("[Error]: Expected Optical Channel %v  Instant OutputPower  data type = %v  , Got =%v", transceiverOpticalChannelName, reflect.Float64, transceiverOutputPowerInstantType)
	}
	if transceiverOutputPowerAvgType := reflect.TypeOf(transceiverOpticalChanelOutputPower.GetAvg()).Kind(); transceiverOutputPowerAvgType != reflect.Float64 {
		t.Fatalf("[Error]: Expected Optical Channel %v  Avg  OutputPower data type = %v  , Got =%v", transceiverOpticalChannelName, reflect.Float64, transceiverOutputPowerAvgType)
	}
	if transceiverOutputPowerMinType := reflect.TypeOf(transceiverOpticalChanelOutputPower.GetMin()).Kind(); transceiverOutputPowerMinType != reflect.Float64 {
		t.Fatalf("[Error]: Expected Optical Channel %v  Min  OutputPower data type = %v  , Got =%v", transceiverOpticalChannelName, reflect.Float64, transceiverOutputPowerMinType)
	}
	if transceiverOutputPowerMaxType := reflect.TypeOf(transceiverOpticalChanelOutputPower.GetMax()).Kind(); transceiverOutputPowerMaxType != reflect.Float64 {
		t.Fatalf("[Error]: Expected Optical Channel %v Max  OutputPower data type = %v  , Got =%v", transceiverOpticalChannelName, reflect.Float64, transceiverOutputPowerMaxType)
	}

	if isInterfaceDisabled {

		if transceiverOpticalChanelOutputPower.GetInstant() != -40 {
			t.Fatalf("[Error]: Expected Optical Channel %v  Instant OutputPower   = %v  ,Got =%v", transceiverOpticalChannelName, -40, transceiverOpticalChanelOutputPower.GetInstant())
		}
		if transceiverOpticalChanelOutputPower.GetAvg() != -40 {
			t.Fatalf("[Error]: Expected Optical Channel %v  Avg OutputPower   = %v  ,Got =%v", transceiverOpticalChannelName, -40, transceiverOpticalChanelOutputPower.GetAvg())
		}
		if transceiverOpticalChanelOutputPower.GetMin() != -40 {
			t.Fatalf("[Error]: Expected Optical Channel %v  Min OutputPower   = %v  ,Got =%v", transceiverOpticalChannelName, -40, transceiverOpticalChanelOutputPower.GetMin())
		}
		if transceiverOpticalChanelOutputPower.GetMax() != -40 {
			t.Fatalf("[Error]: Expected Optical Channel %v  Max OutputPower   = %v  ,Got =%v", transceiverOpticalChannelName, -40, transceiverOpticalChanelOutputPower.GetMax())
		}

	} else if transceiverOpticalChanelOutputPower.GetMin() < txOutputPowerLowerBound || transceiverOpticalChanelOutputPower.GetMax() > txOutputPowerUpperBound {
		t.Fatalf("[Error]:Transciever %v  TX Output power range Expected = %v to %v dbm, Got = %v to %v  ", transceiverName, txOutputPowerLowerBound, txOutputPowerUpperBound, transceiverOpticalChanelOutputPower.GetMin(), transceiverOpticalChanelOutputPower.GetMax())
	} else if transceiverOpticalChanelOutputPower.GetMin() > transceiverOpticalChanelOutputPower.GetAvg() || transceiverOpticalChanelOutputPower.GetAvg() > transceiverOpticalChanelOutputPower.GetMax() || transceiverOpticalChanelOutputPower.GetMin() > transceiverOpticalChanelOutputPower.GetInstant() || transceiverOpticalChanelOutputPower.GetInstant() > transceiverOpticalChanelOutputPower.GetMax() {
		t.Fatalf("Transciever %v TX Output power not following min <= avg/instant <= max . Got instant = %v ,min= %v , avg= %v , max =%v  ", transceiverName, transceiverOpticalChanelOutputPower.GetInstant(), transceiverOpticalChanelOutputPower.GetMin(), transceiverOpticalChanelOutputPower.GetAvg(), transceiverOpticalChanelOutputPower.GetMax())
	}
}

// Verifies whether  inputPower , outputPower ,Frequency are as expected.
func verifyInputOutputPower(t *testing.T, tc *testData) {
	transceiverComponentPath := gnmi.OC().Component(tc.transceiverName)
	transceiverOpticalChannelPath := gnmi.OC().Component(tc.transceiverOpticalChannelName)
	t.Logf("Checking if transceiver = %v is Enabled", tc.transceiverName)

	isTransceiverEnabled := gnmi.Get(t, tc.dut, transceiverComponentPath.Transceiver().Enabled().State())
	if isTransceiverEnabled != true {
		t.Errorf("[Error]:Tranciever  %v is not enabled ", tc.transceiverName)
	}
	t.Logf("Transceiver = %v is in Enabled state", tc.transceiverName)

	t.Logf("Checking Transceiver = %v  Output Frequency ", tc.transceiverName)
	outputFrequency := gnmi.Get(t, tc.dut, transceiverComponentPath.Transceiver().Channel(0).OutputFrequency().State())
	t.Logf("Transceiver = %v  Output Frequency = %v ", tc.transceiverName, outputFrequency)
	if reflect.TypeOf(outputFrequency).Kind() != reflect.Uint64 {
		t.Errorf("[Error]: Expected output frequency data type =%v  Got = %v", reflect.Uint64, reflect.TypeOf(outputFrequency))
	}

	if outputFrequency > outputFreqUpperBound || outputFrequency < outputFreqLowerBound {
		t.Errorf("[Error]: Output Frequency is not a valid frequency %v  =%v,  Got = %v", outputFreqLowerBound, outputFreqUpperBound, outputFrequency)
	}
	t.Logf("Transceiver = %v  Output Frequency is in the expected range", tc.transceiverName)

	opticalInputPowerStream := samplestream.New(t, tc.dut, transceiverOpticalChannelPath.OpticalChannel().InputPower().State(), 10*time.Second)
	defer opticalInputPowerStream.Close()
	transceiverOpticalOutputPowerStream := samplestream.New(t, tc.dut, transceiverOpticalChannelPath.OpticalChannel().OutputPower().State(), 10*time.Second)
	defer transceiverOpticalOutputPowerStream.Close()

	transceiverOpticalChanelOutputPower, ok := transceiverOpticalOutputPowerStream.Next().Val()
	if !ok {
		t.Errorf("[Error]:Trasceiver = %v Output Power not received !", tc.transceiverOpticalChannelName)
	}

	transceiverOpticalChanelInputPower, ok := opticalInputPowerStream.Next().Val()
	if !ok {
		t.Errorf("[Error]:Trasceiver = %v Power not received !", tc.transceiverOpticalChannelName)
	}

	verifyValidTxOutputPower(t, transceiverOpticalChanelOutputPower, false, tc.transceiverOpticalChannelName, tc.transceiverName)
	verifyValidRxInputPower(t, transceiverOpticalChanelInputPower, false, tc.transceiverOpticalChannelName, tc.transceiverName)

	rxTotalInputPowerSamplingTime := gnmi.Get(t, tc.dut, transceiverComponentPath.Transceiver().Channel(0).InputPower().Interval().State())
	rxTotalInputPowerStream := samplestream.New(t, tc.dut, transceiverComponentPath.Transceiver().Channel(0).InputPower().State(), time.Nanosecond*time.Duration(rxTotalInputPowerSamplingTime))
	defer rxTotalInputPowerStream.Close()
	nexInputPower := rxTotalInputPowerStream.Next()
	transceiverRxTotalInputPower, got := nexInputPower.Val()

	if transceiverRxTotalInputPower == nil || got == false {
		t.Errorf("[Error]:Didn't recieve Rx total InputPower sample in 10 seconds for the transceiever = %v", tc.transceiverName)
	}

	t.Logf("Transceiver = %v RX Total Instant Input Power = %v", tc.transceiverName, transceiverRxTotalInputPower.GetInstant())
	t.Logf("Transceiver = %v RX Total Average Input Power = %v", tc.transceiverName, transceiverRxTotalInputPower.GetAvg())
	t.Logf("Transceiver = %v RX Total Min Input Power = %v", tc.transceiverName, transceiverRxTotalInputPower.GetMin())
	t.Logf("Transceiver = %v RX Total channel Max Input Power = %v", tc.transceiverName, transceiverRxTotalInputPower.GetMax())

	if transceiverOpticalChanelInputPower == nil || got == false {
		t.Errorf("[Error]:Didn't recieve Optical channel InputPower sample in 10 seconds for the transceiever = %v", tc.transceiverName)
	}
	if transceiverOpticalChanelInputPower.GetMax() > transceiverRxTotalInputPower.GetMax() {
		t.Errorf("[Error]:Transciever %v RX Signal Power = %v is more than Total RX Signal Power = %v ", tc.transceiverName, transceiverOpticalChanelInputPower.GetMax(), transceiverRxTotalInputPower.GetMax())
	}

}

// Reboots the device and verifies Rx InputPower, TxOutputPower data while components are booting.
func dutRebootRxInputTxOutputPowerCheck(t *testing.T, tc *testData) {
	gnoiClient, err := tc.dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect to gnoi server, err: %v", err)
	}
	rebootRequest := &gnps.RebootRequest{
		Method: gnps.RebootMethod_COLD,
		Force:  true,
	}
	preRebootCompStatus := gnmi.GetAll(t, tc.dut, gnmi.OC().ComponentAny().OperStatus().State())
	preRebootCompDebug := gnmi.GetAll(t, tc.dut, gnmi.OC().ComponentAny().State())
	preCompMatrix := []string{}
	for _, preComp := range preRebootCompDebug {
		if preComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
			preCompMatrix = append(preCompMatrix, preComp.GetName()+":"+preComp.GetOperStatus().String())
		}
	}

	bootTimeBeforeReboot := gnmi.Get(t, tc.dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	var currentTime string
	currentTime = gnmi.Get(t, tc.dut, gnmi.OC().System().CurrentDatetime().State())
	t.Logf("Time Before Reboot : %v", currentTime)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	t.Logf("Got Reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}
	for {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, tc.dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Log("Reboot is started")
			break
		}
		t.Log("Wait for reboot to be started")
		time.Sleep(30 * time.Second)
	}
	startReboot := time.Now()
	t.Logf("Waiting for DUT to boot up by polling the telemetry output.")
	for {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, tc.dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg == nil {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}
		if uint64(time.Since(startReboot).Seconds()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	time.Sleep(30 * time.Second)
	startComp := time.Now()
	for {

		postRebootCompStatus := gnmi.GetAll(t, tc.dut, gnmi.OC().ComponentAny().OperStatus().State())
		postRebootCompDebug := gnmi.GetAll(t, tc.dut, gnmi.OC().ComponentAny().State())
		postCompMatrix := []string{}

		if verErrMsg := testt.CaptureFatal(t, func(t testing.TB) {
			interfaceEnabled := gnmi.Get(t, tc.dut, gnmi.OC().Interface(tc.interfaceName).Enabled().Config())
			transceiverOpticalChannelPath := gnmi.OC().Component(tc.transceiverOpticalChannelName)
			transceiverOpticalChanelInputPower := gnmi.Get(t, tc.dut, transceiverOpticalChannelPath.OpticalChannel().InputPower().State())
			transceiverOpticalChanelOutputPower := gnmi.Get(t, tc.dut, transceiverOpticalChannelPath.OpticalChannel().OutputPower().State())
			verifyValidRxInputPower(t, transceiverOpticalChanelInputPower, !interfaceEnabled, tc.transceiverOpticalChannelName, tc.transceiverName)
			verifyValidTxOutputPower(t, transceiverOpticalChanelOutputPower, !interfaceEnabled, tc.transceiverOpticalChannelName, tc.transceiverName)
		}); strings.HasPrefix(*verErrMsg, "[Error]") {
			t.Fatal(*verErrMsg)
		}

		for _, postComp := range postRebootCompDebug {
			if postComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
				postCompMatrix = append(postCompMatrix, postComp.GetName()+":"+postComp.GetOperStatus().String())
			}
		}
		if len(preRebootCompStatus) == len(postRebootCompStatus) {
			if rebootDiff := cmp.Diff(preCompMatrix, postCompMatrix); rebootDiff == "" {
				time.Sleep(10 * time.Second)
				break
			}
		}
		if uint64(time.Since(startComp).Seconds()) > maxCompWaitTime {
			t.Logf("DUT components status post reboot: %v", postRebootCompStatus)
			if rebootDiff := cmp.Diff(preCompMatrix, postCompMatrix); rebootDiff != "" {
				t.Logf("[DEBUG] Unexpected diff after reboot (-component missing from pre reboot, +component added from pre reboot): %v ", rebootDiff)
			}
			t.Fatalf("There's a difference in components obtained in pre reboot: %v and post reboot: %v.", len(preRebootCompStatus), len(postRebootCompStatus))
			break
		}
		time.Sleep(10 * time.Second)
	}

}

// Verifies Rx InputPower, TxOutputPower data is streamed correctly after interface flaps.
func verifyRxInputTxOutputAfterFlap(t *testing.T, tc *testData) {

	interfacePath := gnmi.OC().Interface(tc.interfaceName)

	transceiverOpticalChannelPath := gnmi.OC().Component(tc.transceiverOpticalChannelName)
	opticalInputPowerStream := samplestream.New(t, tc.dut, transceiverOpticalChannelPath.OpticalChannel().InputPower().State(), 10*time.Second)
	defer opticalInputPowerStream.Close()
	transceiverOpticalOutputPowerStream := samplestream.New(t, tc.dut, transceiverOpticalChannelPath.OpticalChannel().OutputPower().State(), 10*time.Second)
	defer transceiverOpticalOutputPowerStream.Close()

	transceiverOpticalChanelInputPower, _ := opticalInputPowerStream.Next().Val()
	transceiverOpticalChanelOutputPower, _ := transceiverOpticalOutputPowerStream.Next().Val()

	verifyValidRxInputPower(t, transceiverOpticalChanelInputPower, false, tc.transceiverOpticalChannelName, tc.transceiverName)
	verifyValidTxOutputPower(t, transceiverOpticalChanelOutputPower, false, tc.transceiverOpticalChannelName, tc.transceiverName)

	t.Logf("Disbaling the interface = %v ", tc.interfaceName)
	gnmi.Replace(t, tc.dut, interfacePath.Enabled().Config(), *ygot.Bool(false))
	gnmi.Await(t, tc.dut, interfacePath.Enabled().State(), time.Minute, *ygot.Bool(false))

	t.Logf("Disabled the interface = %v", tc.interfaceName)
	transceiverOpticalChanelInputPower, _ = opticalInputPowerStream.Nexts(5)[4].Val()
	transceiverOpticalChanelOutputPower, _ = transceiverOpticalOutputPowerStream.Nexts(5)[4].Val()
	verifyValidRxInputPower(t, transceiverOpticalChanelInputPower, true, tc.transceiverOpticalChannelName, tc.transceiverName)
	verifyValidTxOutputPower(t, transceiverOpticalChanelOutputPower, true, tc.transceiverOpticalChannelName, tc.transceiverName)

	t.Logf("Re-enabling the interface = %v ", tc.interfaceName)
	gnmi.Replace(t, tc.dut, interfacePath.Enabled().Config(), *ygot.Bool(true))
	gnmi.Await(t, tc.dut, interfacePath.Enabled().State(), time.Minute, *ygot.Bool(true))
	t.Logf("Re-enabled the interface = %v", tc.interfaceName)

	transceiverOpticalChanelInputPower, _ = opticalInputPowerStream.Nexts(5)[4].Val()
	transceiverOpticalChanelOutputPower, _ = transceiverOpticalOutputPowerStream.Nexts(5)[4].Val()
	verifyValidRxInputPower(t, transceiverOpticalChanelInputPower, false, tc.transceiverOpticalChannelName, tc.transceiverName)
	verifyValidTxOutputPower(t, transceiverOpticalChanelOutputPower, false, tc.transceiverOpticalChannelName, tc.transceiverName)

}

func TestZrInputOutputPower(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	transceiver1Port := dut1.Port(t, "port1")
	transceiver2Port := dut2.Port(t, "port1")
	transceiver1Name := gnmi.Get(t, dut1, gnmi.OC().Interface(transceiver1Port.Name()).Transceiver().State())
	transceiver2Name := gnmi.Get(t, dut2, gnmi.OC().Interface(transceiver2Port.Name()).Transceiver().State())

	configureDUT(t, dut1, transceiver1Name)
	configureDUT(t, dut2, transceiver2Name)

	transceiver1OpticalChannelName := gnmi.Get(t, dut1, gnmi.OC().Component(transceiver1Name).Transceiver().Channel(0).AssociatedOpticalChannel().State())
	transceiver2OpticalChannelName := gnmi.Get(t, dut2, gnmi.OC().Component(transceiver2Name).Transceiver().Channel(0).AssociatedOpticalChannel().State())

	testCases := []testData{
		{
			transceiverName:               transceiver1Name,
			dut:                           dut1,
			transceiverOpticalChannelName: transceiver1OpticalChannelName,
			interfaceName:                 transceiver1Port.Name(),
		},
		{
			transceiverName:               transceiver2Name,
			dut:                           dut2,
			transceiverOpticalChannelName: transceiver2OpticalChannelName,
			interfaceName:                 transceiver2Port.Name(),
		},
	}

	t.Run("RT-4.1: Testing Input, Output Power telemetry", func(t *testing.T) {
		for _, testDataObj := range testCases {
			verifyInputOutputPower(t, &testDataObj)
		}
	})

	t.Run("RT-4.2: Testing Rx Input Power, Tx Output Power telemetry  during DUT reboot", func(t *testing.T) {
		for _, testDataObj := range testCases {
			dutRebootRxInputTxOutputPowerCheck(t, &testDataObj)
		}
	})
	t.Run("RT-4.3: Interface flap Rx Input Power Tx Output Power telemetry test", func(t *testing.T) {
		for _, testDataObj := range testCases {
			verifyRxInputTxOutputAfterFlap(t, &testDataObj)
		}
	})

	// Todo: ## TRANSCEIVER-4.4 .

}
