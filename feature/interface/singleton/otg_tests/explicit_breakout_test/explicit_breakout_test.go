package explicit_breakout_test

import (
	"testing"
	"time"

	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
)

var componentName string
var schemaValue uint8

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func verifyOperStatus(t *testing.T, timeout time.Duration, dut *ondatra.DUTDevice,
	p *ondatra.Port, up bool) {
	t.Helper()
	want := oc.Interface_OperStatus_DOWN
	if up {
		want = oc.Interface_OperStatus_UP
	}
	path := gnmi.OC().Interface(p.Name()).OperStatus().State()
	gnmi.Await(t, dut, path, timeout, want)
	t.Logf("Port status is as expected: Port: %v, oper-status: %v", p.Name(), want)
}

func deleteBreakout(t *testing.T, dut *ondatra.DUTDevice, dutPort string) {
	t.Logf("deleting breakout: componentName %v, portName %v", componentName, dutPort)
	gnmi.Delete(t, dut, gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue).Config())
}

func verifyBreakoutConfigRemoved(t *testing.T, dut *ondatra.DUTDevice) {
	path := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue)
	t.Logf("/components/component/port/breakout-mode/groups/group/state/index should not exist")
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Get(t, dut, path.Index().State())
	}); errMsg != nil {
		t.Logf("Expected failure as this verifies the breakout does not exist: %v", *errMsg)
	} else {
		t.Errorf("GNMI get should have failed")
	}

	t.Logf("/components/component/port/breakout-mode/groups/group/config/index should not exist")
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Get(t, dut, path.Index().Config())
	}); errMsg != nil {
		t.Logf("Expected failure as this verifies the breakout does not exist: %v", *errMsg)
	} else {
		t.Errorf("GNMI get should have failed")
	}

	t.Logf("/components/component/port/breakout-mode/groups/group/state/breakout-speed should not exist")
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Get(t, dut, path.BreakoutSpeed().State())
	}); errMsg != nil {
		t.Logf("Expected failure as this verifies the breakout does not exist: %v", *errMsg)
	} else {
		t.Errorf("GNMI get should have failed")
	}

	t.Logf("/components/component/port/breakout-mode/groups/group/config/breakout-speed should not exist")
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Get(t, dut, path.BreakoutSpeed().Config())
	}); errMsg != nil {
		t.Logf("Expected failure as this verifies the breakout does not exist: %v", *errMsg)
	} else {
		t.Errorf("GNMI get should have failed")
	}
}

func setPortSpeedTo100GB(t *testing.T, dut *ondatra.DUTDevice, dutPort string, dp *ondatra.Port) {
	speed := oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	dutInterfacePath := gnmi.OC().Interface(dutPort).Ethernet().PortSpeed()
	gnmi.Replace(t, dut, dutInterfacePath.Config(), speed)
	gnmi.Await(t, dut, dutInterfacePath.State(), 1*time.Minute, speed)
}

func configureBreakout(t *testing.T, dut *ondatra.DUTDevice, dutPort string, dp *ondatra.Port) {
	numOfBreakouts := uint8(4)
	breakoutspeed := oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB

	configContainer := &oc.Component_Port_BreakoutMode_Group{
		Index:         ygot.Uint8(schemaValue),
		NumBreakouts:  ygot.Uint8(numOfBreakouts),
		BreakoutSpeed: oc.E_IfEthernet_ETHERNET_SPEED(breakoutspeed),
	}

	path := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue)
	gnmi.Replace(t, dut, path.Config(), configContainer)

	t.Logf("verify breakouts are configured on %v", dutPort)
	state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue)
	groupDetails := gnmi.Get(t, dut, state.Config())
	indexGot := *groupDetails.Index
	numBreakoutsGot := *groupDetails.NumBreakouts
	breakoutSpeedGot := groupDetails.BreakoutSpeed
	if indexGot != schemaValue || numBreakoutsGot != numOfBreakouts || breakoutSpeedGot != breakoutspeed {
		t.Errorf("Index: got %v, want 1; numOfBreakouts: got %v, want %v; breakoutSpeed: got %v, want %v", indexGot, numBreakoutsGot, numOfBreakouts, breakoutSpeedGot, breakoutspeed)
	}
}

func getComponentName(t *testing.T, dut *ondatra.DUTDevice, dutPort string) string {
	components := gnmi.GetAll[*oc.Component](t, dut, gnmi.OC().ComponentAny().State())
	hardwarePort := gnmi.Get(t, dut, gnmi.OC().Interface(dutPort).HardwarePort().State())
	for _, c := range components {
		if c.GetType() != nil && c.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT {
			if hardwarePort == c.GetName() {
				t.Logf("Found componentName: %v", c.GetName())
				break
			}
		}
	}
	return hardwarePort
}

func verifyExplicitBreakoutConfig(t *testing.T, dut *ondatra.DUTDevice, dutPort string, dp *ondatra.Port) {
	t.Logf("Ensure no breakout is configured: Deleting breakout on %v", dutPort)
	deleteBreakout(t, dut, dutPort)

	t.Logf("verifing breakout configuration does not exist on %v", dutPort)
	verifyBreakoutConfigRemoved(t, dut)

	t.Logf("Fetch Intial port-speed and verify oper-status is UP for port: %v", dutPort)
	dutInterfacePath := gnmi.OC().Interface(dutPort)
	origPortSpeed := gnmi.Get(t, dut, dutInterfacePath.Ethernet().PortSpeed().State())
	t.Logf("Initial speed %v on port %v on DUT", origPortSpeed, dutPort)
	verifyOperStatus(t, 1*time.Minute, dut, dp, true)

	t.Logf("change port speed to 100G and verify the oper-status is down")
	t.Logf("Setting port: %v speed to 100G", dutPort)
	setPortSpeedTo100GB(t, dut, dutPort, dp)

	t.Logf("verify oper-status is down on port: %v", dutPort)
	verifyOperStatus(t, 1*time.Minute, dut, dp, false)

	t.Logf("verify breakout is not set implicitly on %v, after speed change to 100G on a 400G intf", dutPort)
	verifyBreakoutConfigRemoved(t, dut)
}

func verifySetPortSpeedWithBreakoutConfigFails(t *testing.T, dut *ondatra.DUTDevice, dutPort string, dp *ondatra.Port, speed oc.E_IfEthernet_ETHERNET_SPEED) {
	dutintfEthPortSpeedPath := gnmi.OC().Interface(dutPort).Ethernet().PortSpeed()

	t.Logf("Configuring breakout on %v", dutPort)
	configureBreakout(t, dut, dutPort, dp)

	t.Logf("Setting port speed to 100G on port %v", dutPort)
	gnmi.Replace(t, dut, dutintfEthPortSpeedPath.Config(), speed)

	t.Logf("Verify port-speed is rejected on %v", dutPort)
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Get(t, dut, dutintfEthPortSpeedPath.State())
	}); errMsg != nil {
		t.Logf("Expected failure as this verifies ethernet port speed does not exist: %v", *errMsg)
		t.Logf("/interfaces/interface/ethernet/state/port-speed does not exist on intf %v:", dutPort)
	} else {
		t.Logf("should have failed, interface: %v", dutPort)
		t.Errorf("/interfaces/interface/ethernet/state/port-speed is expected not to be present")
	}
}

func verifyDeleteInterfaceConfigure(t *testing.T, dut *ondatra.DUTDevice, dutPort string, dp *ondatra.Port) {
	t.Logf("Deleting interface ethernet portspeed and breakout configuration")
	batchConfig := &gnmi.SetBatch{}
	gnmi.BatchDelete(batchConfig, gnmi.OC().Interface(dutPort).Ethernet().PortSpeed().Config())
	gnmi.BatchDelete(batchConfig, gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue).Config())
	batchConfig.Set(t, dut)

	t.Logf("verify breakout configuration is removed")
	verifyBreakoutConfigRemoved(t, dut)
	t.Logf("verify port-speed configuration is removed, and the default port speed is streamed")
	pathEtherPortSpeed := gnmi.OC().Interface(dutPort).Ethernet().PortSpeed().State()
	gnmi.Await(t, dut, pathEtherPortSpeed, 1*time.Minute, oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB)
	portSpeed := gnmi.Get(t, dut, pathEtherPortSpeed)
	t.Logf("Port speed is: %v after the breakout config and interface ethernet port-speed config is deleted on Port: %v", portSpeed, dutPort)
}

func TestExplicitBreakoutConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	dutPort := dut.Port(t, "port1").Name()
	if componentName == "" {
		componentName = getComponentName(t, dut, dutPort)
	}
	t.Logf("componentName: %v", componentName)
	var schemaValue uint8
	if uint8(deviations.BreakOutSchemaValueFlag(dut)) == 1 {
		schemaValue = uint8(1)
	} else {
		schemaValue = uint8(0)
	}
	t.Logf("schemaValue: %v", schemaValue)

	t.Logf("RT-5.1.4: Breakout must be explicitly configured by gNMI client")
	verifyExplicitBreakoutConfig(t, dut, dutPort, dp)
}

func TestPortSeedWithBreakoutConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	dutPort := dut.Port(t, "port1").Name()
	if componentName == "" {
		componentName = getComponentName(t, dut, dutPort)
	}
	t.Logf("componentName: %v", componentName)
	speed := oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	var schemaValue uint8
	if uint8(deviations.BreakOutSchemaValueFlag(dut)) == 1 {
		schemaValue = uint8(1)
	} else {
		schemaValue = uint8(0)
	}
	t.Logf("schemaValue: %v", schemaValue)

	t.Logf("RT-5.1.5: Setting port-speed on interface that have breakout configured should not be allowed")
	verifySetPortSpeedWithBreakoutConfigFails(t, dut, dutPort, dp, speed)
}

func TestDeleteInterfaceConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")
	dutPort := dut.Port(t, "port1").Name()
	if componentName == "" {
		componentName = getComponentName(t, dut, dutPort)
	}
	t.Logf("componentName: %v", componentName)

	t.Logf("RT-5.1.6: Remove breakout and interface config to delete the interface config")
	verifyDeleteInterfaceConfigure(t, dut, dutPort, dp)
}
