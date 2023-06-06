package optics_power_and_bias_current_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	ethernetCsmacd         = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	transceiverType        = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	sleepDuration          = time.Minute
	minOpticsPower         = -40.0
	maxOpticsPower         = 10.0
	minOpticsHighThreshold = 1.0
	maxOpticsLowThreshold  = -1.0
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology:
//   ate:port1 <--> port1:dut:port2 <--> ate:port2
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestOpticsPowerBiasCurrent(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	transceivers := components.FindComponentsByType(t, dut, transceiverType)
	t.Logf("Found transceiver list: %v", transceivers)
	if len(transceivers) == 0 {
		t.Fatalf("Get transceiver list for %q: got 0, want > 0", dut.Model())
	}

	for _, transceiver := range transceivers {
		t.Logf("Validate transceiver: %s", transceiver)
		component := gnmi.OC().Component(transceiver)

		if !gnmi.Lookup(t, dut, component.MfgName().State()).IsPresent() {
			t.Logf("component.MfgName().Lookup(t).IsPresent() for %q is false. skip it", transceiver)
			continue
		}
		if strings.Contains(gnmi.Lookup(t, dut, gnmi.OC().Component(transceiver).Description().State()).String(), "ZR") {
			t.Logf("Transceiver %s has ZR optics", transceiver)
			//No sysdb paths found for yang path components/component/transceiver/state/enabled
			/*enabled := gnmi.Get(t, dut, component.Transceiver().Enabled().State())
			  t.Log(enabled)*/

			present := gnmi.Get(t, dut, component.Transceiver().Present().State())
			t.Logf("Transceiver %s present: %s", transceiver, present)

			formFactor := gnmi.Get(t, dut, component.Transceiver().FormFactor().State())
			t.Logf("Transceiver %s formFactor: %s", transceiver, formFactor)

			connectorType := gnmi.Get(t, dut, component.Transceiver().ConnectorType().State())
			t.Logf("Transceiver %s connectorType: %s", transceiver, connectorType)

			vendor := gnmi.Get(t, dut, component.Transceiver().Vendor().State())
			t.Logf("Transceiver %s vendor: %s", transceiver, vendor)

			vendorPart := gnmi.Get(t, dut, component.Transceiver().VendorPart().State())
			t.Logf("Transceiver %s vendorPart: %s", transceiver, vendorPart)

			vendorRev := gnmi.Get(t, dut, component.Transceiver().VendorRev().State())
			t.Logf("Transceiver %s vendorRev: %s", transceiver, vendorRev)

			sonetSdhComplianceCode := gnmi.Get(t, dut, component.Transceiver().SonetSdhComplianceCode().State())
			t.Logf("Transceiver %s sonetSdhComplianceCode: %s", transceiver, sonetSdhComplianceCode)

			otnComplianceCode := gnmi.Get(t, dut, component.Transceiver().OtnComplianceCode().State())
			t.Logf("Transceiver %s otnComplianceCode: %s", transceiver, otnComplianceCode)

			serialNo := gnmi.Get(t, dut, component.Transceiver().SerialNo().State())
			t.Logf("Transceiver %s serialNo: %s", transceiver, serialNo)

			//Unmarshalling failed : Bug in Library code, ISSUE raised with google
			/*dateCode := gnmi.Get(t, dut, component.Transceiver().DateCode().State())
			  t.Log(dateCode)*/

			faultCondition := gnmi.Get(t, dut, component.Transceiver().FaultCondition().State())
			t.Logf("Transceiver %s faultCondition: %t", transceiver, faultCondition)

			mfgName := gnmi.Get(t, dut, component.MfgName().State())
			t.Logf("Transceiver %s MfgName: %s", transceiver, mfgName)

			index := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().Index().State())
			t.Logf("Transceiver %s Index: %v", transceiver, index)
			if len(index) == 0 {
				t.Errorf("Get Index list for %q: got 0, want > 0", transceiver)
			}

			opFreq := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().OutputFrequency().State())
			t.Logf("Transceiver %s OutputFrequency: %v", transceiver, opFreq)
			if len(opFreq) == 0 {
				t.Errorf("Get OutputFrequency list for %q: got 0, want > 0", transceiver)
			}

			//No sysdb paths found for yang path components/component/transceiver/physical-channels/channel/state/target-output-power\x00"}
			/*targetOpPower := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().TargetOutputPower().State())
			  t.Logf("Transceiver %s TargetOutputPower: %v", transceiver, targetOpPower)
			  if len(targetOpPower) == 0 {
			          t.Errorf("Get TargetOutputPower list for %q: got 0, want > 0", transceiver)
			  }*/

			inputPowers := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().InputPower().Instant().State())
			t.Logf("Transceiver %s inputPowerInstant: %v", transceiver, inputPowers)
			if len(inputPowers) == 0 {
				t.Errorf("Get inputPowerInstant list for %q: got 0, want > 0", transceiver)
			}
			inputPowerAvg := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().InputPower().Avg().State())
			t.Logf("Transceiver %s inputPowerAvg : %v", transceiver, inputPowerAvg)
			if len(inputPowerAvg) == 0 {
				t.Errorf("Get inputPowerAvg list for %q: got 0, want > 0", transceiver)
			}
			inputPowerInterval := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().InputPower().Interval().State())
			t.Logf("Transceiver %s inputPowerInterval: %v", transceiver, inputPowerInterval)
			if len(inputPowerInterval) == 0 {
				t.Errorf("Get inputPowerInterval list for %q: got 0, want > 0", transceiver)
			}
			inputPowerMax := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().InputPower().Max().State())
			t.Logf("Transceiver %s inputPowerMax: %v", transceiver, inputPowerMax)
			if len(inputPowerMax) == 0 {
				t.Errorf("Get inputPowerMax list for %q: got 0, want > 0", transceiver)
			}
			inputPowerMaxTime := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().InputPower().MaxTime().State())
			t.Logf("Transceiver %s inputPowerMaxTime: %v", transceiver, inputPowerMaxTime)
			if len(inputPowerMaxTime) == 0 {
				t.Errorf("Get inputPowerMaxTime list for %q: got 0, want > 0", transceiver)
			}
			inputPowerMin := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().InputPower().Min().State())
			t.Logf("Transceiver %s inputPowerMin: %v", transceiver, inputPowerMin)
			if len(inputPowerMin) == 0 {
				t.Errorf("Get inputPowerMin list for %q: got 0, want > 0", transceiver)
			}
			inputPowerMinTime := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().InputPower().MinTime().State())
			t.Logf("Transceiver %s MinTime: %v", transceiver, inputPowerMinTime)
			if len(inputPowerMinTime) == 0 {
				t.Errorf("Get inputPowerMinTime list for %q: got 0, want > 0", transceiver)
			}

			outputPowers := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().OutputPower().Instant().State())
			t.Logf("Transceiver %s inputPowerInstant: %v", transceiver, outputPowers)
			if len(outputPowers) == 0 {
				t.Errorf("Get outputPowerInstant list for %q: got 0, want > 0", transceiver)
			}
			outputPowerAvg := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().OutputPower().Avg().State())
			t.Logf("Transceiver %s outputPowerAvg: %v", transceiver, outputPowerAvg)
			if len(outputPowerAvg) == 0 {
				t.Errorf("Get outputPowerAvg list for %q: got 0, want > 0", transceiver)
			}
			outputPowerInterval := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().OutputPower().Interval().State())
			t.Logf("Transceiver %s outputPowerInterval: %v", transceiver, outputPowerInterval)
			if len(outputPowerInterval) == 0 {
				t.Errorf("Get outputPowerInterval list for %q: got 0, want > 0", transceiver)
			}
			outputPowerMax := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().OutputPower().Max().State())
			t.Logf("Transceiver %s outputPowerMax: %v", transceiver, outputPowerMax)
			if len(outputPowerMax) == 0 {
				t.Errorf("Get outputPowerMax list for %q: got 0, want > 0", transceiver)
			}
			outputPowerMaxTime := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().OutputPower().MaxTime().State())
			t.Logf("Transceiver %s outputPowerMaxTime: %v", transceiver, outputPowerMaxTime)
			if len(outputPowerMaxTime) == 0 {
				t.Errorf("Get outputPowerMaxTime list for %q: got 0, want > 0", transceiver)
			}
			outputPowerMin := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().OutputPower().Min().State())
			t.Logf("Transceiver %s outputPowerMin: %v", transceiver, outputPowerMin)
			if len(outputPowerMin) == 0 {
				t.Errorf("Get outputPowerMin list for %q: got 0, want > 0", transceiver)
			}
			outputPowerMinTime := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().OutputPower().MinTime().State())
			t.Logf("Transceiver %s outputPowerMinTime: %v", transceiver, outputPowerMinTime)
			if len(outputPowerMinTime) == 0 {
				t.Errorf("Get outputPowerMinTime list for %q: got 0, want > 0", transceiver)
			}

			biasCurrents := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().LaserBiasCurrent().Instant().State())
			t.Logf("Transceiver %s inputPowerInstant: %v", transceiver, biasCurrents)
			if len(biasCurrents) == 0 {
				t.Errorf("Get biasCurrentInstant list for %q: got 0, want > 0", transceiver)
			}
			biasCurrentAvg := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().LaserBiasCurrent().Avg().State())
			t.Logf("Transceiver %s biasCurrentAvg: %v", transceiver, biasCurrentAvg)
			if len(biasCurrentAvg) == 0 {
				t.Errorf("Get biasCurrentAvg list for %q: got 0, want > 0", transceiver)
			}
			biasCurrentInterval := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().LaserBiasCurrent().Interval().State())
			t.Logf("Transceiver %s biasCurrentInterval: %v", transceiver, biasCurrentInterval)
			if len(biasCurrentInterval) == 0 {
				t.Errorf("Get biasCurrentInterval list for %q: got 0, want > 0", transceiver)
			}
			biasCurrentMax := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().LaserBiasCurrent().Max().State())
			t.Logf("Transceiver %s biasCurrentMax: %v", transceiver, biasCurrentMax)
			if len(biasCurrentMax) == 0 {
				t.Errorf("Get biasCurrentMax list for %q: got 0, want > 0", transceiver)
			}
			biasCurrentMaxTime := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().LaserBiasCurrent().MaxTime().State())
			t.Logf("Transceiver %s biasCurrentMaxTime: %v", transceiver, biasCurrentMaxTime)
			if len(biasCurrentMaxTime) == 0 {
				t.Errorf("Get biasCurrentMaxTime list for %q: got 0, want > 0", transceiver)
			}
			biasCurrentMin := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().LaserBiasCurrent().Min().State())
			t.Logf("Transceiver %s biasCurrentMin: %v", transceiver, biasCurrentMin)
			if len(biasCurrentMin) == 0 {
				t.Errorf("Get biasCurrentMin list for %q: got 0, want > 0", transceiver)
			}
			biasCurrentMinTime := gnmi.GetAll(t, dut, component.Transceiver().ChannelAny().LaserBiasCurrent().MinTime().State())
			t.Logf("Transceiver %s biasCurrentMinTime: %v", transceiver, biasCurrentMinTime)
			if len(biasCurrentMinTime) == 0 {
				t.Errorf("Get biasCurrentMinTime list for %q: got 0, want > 0", transceiver)
			}
		}
	}
}

func TestOpticsPowerUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())

	cases := []struct {
		desc                string
		IntfStatus          bool
		expectedStatus      oc.E_Interface_OperStatus
		expectedMaxOutPower float64
		checkMinOutPower    bool
	}{{
		// Check both input and output optics power are in normal range.
		desc:                "Check initial input and output optics powers are OK",
		IntfStatus:          true,
		expectedStatus:      oc.Interface_OperStatus_UP,
		expectedMaxOutPower: maxOpticsPower,
		checkMinOutPower:    true,
	}, {
		desc:                "Check output optics power is very small after interface is disabled",
		IntfStatus:          false,
		expectedStatus:      oc.Interface_OperStatus_DOWN,
		expectedMaxOutPower: minOpticsPower,
		checkMinOutPower:    false,
	}, {
		desc:                "Check output optics power is normal after interface is re-enabled",
		IntfStatus:          true,
		expectedStatus:      oc.Interface_OperStatus_UP,
		expectedMaxOutPower: maxOpticsPower,
		checkMinOutPower:    true,
	}}
	for _, tc := range cases {
		t.Log(tc.desc)
		intUpdateTime := 2 * time.Minute
		t.Run(tc.desc, func(t *testing.T) {
			i.Enabled = ygot.Bool(tc.IntfStatus)
			i.Type = ethernetCsmacd
			util.FlapInterface(t, dut, dp.Name(), 10)
			if deviations.ExplicitPortSpeed(dut) {
				i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, dp)
			}
			gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, tc.expectedStatus)

			transceiverName, err := findTransceiverName(dut, dp.Name())
			if err != nil {
				t.Fatalf("findTransceiver(%s, %s): %v", dut.Name(), dp.Name(), err)
			}

			component := gnmi.OC().Component(transceiverName)
			if !gnmi.Lookup(t, dut, component.MfgName().State()).IsPresent() {
				t.Skipf("component.MfgName().Lookup(t).IsPresent() for %q is false. skip it", transceiverName)
			}

			mfgName := gnmi.Get(t, dut, component.MfgName().State())
			t.Logf("Transceiver MfgName: %s", mfgName)

			channels := gnmi.OC().Component(dp.Name()).Transceiver().ChannelAny()
			inputPowers := gnmi.LookupAll(t, dut, channels.InputPower().Instant().State())
			outputPowers := gnmi.LookupAll(t, dut, channels.OutputPower().Instant().State())
			for _, inputPower := range inputPowers {
				inPower, ok := inputPower.Val()
				if !ok {
					t.Errorf("Get inputPower for port %q: got 0, want > 0", dp.Name())
					continue
				}
				if inPower > maxOpticsPower || inPower < minOpticsPower {
					t.Errorf("Get inputPower for port %q): got %.2f, want within [%f, %f]", dp.Name(), inPower, minOpticsPower, maxOpticsPower)
				}
			}
			for _, outputPower := range outputPowers {
				outPower, ok := outputPower.Val()
				if !ok {
					t.Errorf("Get outputPower for port %q: got 0, want > 0", dp.Name())
					continue
				}
				if outPower > tc.expectedMaxOutPower {
					t.Errorf("Get outPower for port %q): got %.2f, want < %f", dp.Name(), outPower, tc.expectedMaxOutPower)
				}
				if tc.checkMinOutPower && outPower < minOpticsPower {
					t.Errorf("Get outPower for port %q): got %.2f, want > %f", dp.Name(), outPower, minOpticsPower)
				}
			}
		})
	}
}

// findTransceiverName provides name of transciever port corresponding to interface name
func findTransceiverName(dut *ondatra.DUTDevice, interfaceName string) (string, error) {
	var (
		transceiverMap = map[ondatra.Vendor]string{
			ondatra.ARISTA:  " transceiver",
			ondatra.CISCO:   "",
			ondatra.JUNIPER: "",
		}
	)
	transceiverName := interfaceName
	name, ok := transceiverMap[dut.Vendor()]
	if !ok {
		return "", fmt.Errorf("No transceiver interface available for DUT vendor %v", dut.Vendor())
	}
	if name != "" {
		interfaceSplit := strings.Split(interfaceName, "/")
		interfaceSplitres := interfaceSplit[:len(interfaceSplit)-1]
		transceiverName = strings.Join(interfaceSplitres, "/") + name

	}
	return transceiverName, nil
}
