package holddown_times_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/ondatra/netutil"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	lagName       = "LAGRx" // OTG LAG NAME
	upTimer       = 5000
	downTimer     = 300
	toleranceMS   = 200 // Define the tolerance in milliseconds
)

var (
	top          *gosnappi.Config
	aggID        string
	dutPort1Intf *ondatra.Port
	ateSrc       = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	ateDst = attrs.Attributes{
		Name:    "ateDst",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureDUTBundle(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()

	agg := dutDst.NewOCInterface(aggID, dut)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	gnmi.Replace(t, dut, gnmi.OC().Interface(aggID).Config(), agg)

	for _, port := range aggPorts {
		d := &oc.Root{}

		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, aggID string) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	dutAggPorts := []*ondatra.Port{
		dut.Port(t, "port1"),
		// dut.Port(t, "port2"),
	}
	configureDUTBundle(t, dut, dutAggPorts, aggID)

}

func configureOTG(t *testing.T,
	ate *ondatra.ATEDevice,
	aggID string) {
	t.Helper()

	top := gosnappi.NewConfig()

	ateAggPorts := []*ondatra.Port{
		ate.Port(t, "port1"),
		// ate.Port(t, "port2"),
	}
	configureOTGBundle(t, ate, top, ateAggPorts, aggID)

	t.Log(top.String())
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	OTGInterfaceUP(t, ate)
}

func OTGInterfaceUP(t *testing.T,
	ate *ondatra.ATEDevice) {

	p1 := ondatra.ATE(t, "ate").Port(t, "port1")
	portStateAction := gosnappi.NewControlState()

	// make sure interface is not down
	portStateAction.Port().Link().SetPortNames([]string{p1.ID()}).SetState(gosnappi.StatePortLinkState.UP)
	ate.OTG().SetControlState(t, portStateAction)
}

func OTGInterfaceDOWN(t *testing.T,
	ate *ondatra.ATEDevice) {

	p1 := ondatra.ATE(t, "ate").Port(t, "port1")
	portStateAction := gosnappi.NewControlState()

	// make sure interface is not down
	portStateAction.Port().Link().SetPortNames([]string{p1.ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
	ate.OTG().SetControlState(t, portStateAction)
}

func configureOTGBundle(t *testing.T,
	ate *ondatra.ATEDevice,
	top gosnappi.Config,
	aggPorts []*ondatra.Port,
	aggID string) {
	t.Helper()
	agg := top.Lags().Add().SetName(lagName)
	lagID, _ := strconv.Atoi(aggID)
	agg.Protocol().Static().SetLagId(uint32(lagID))

	for i, p := range aggPorts {
		port := top.Ports().Add().SetName(p.ID())
		// newMac, err := incrementMAC(ateDst.MAC, i+1)
		// if err != nil {
		// 	t.Fatal(err)
		// }
		agg.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(ateSrc.MAC).SetName("LAGRx-" + strconv.Itoa(i))
	}

	dstDev := top.Devices().Add().SetName(agg.Name() + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(lagName + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(lagName + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
}

func flapOTGInterface(t *testing.T,
	ate *ondatra.ATEDevice,
	dut *ondatra.DUTDevice,
	actionState string) (time.Time, time.Time) {

	// Shut down OTG Interface
	p1 := ondatra.ATE(t, "ate").Port(t, "port1")
	portStateAction := gosnappi.NewControlState()

	var beforeEventChangeStr string

	preStateTS := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())

	if actionState == "UP" {

		portStateAction.Port().Link().SetPortNames([]string{p1.ID()}).SetState(gosnappi.StatePortLinkState.UP)
		beforeEventChangeStr = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		ate.OTG().SetControlState(t, portStateAction)

	} else if actionState == "DOWN" {
		// Bring Down OTG Interface
		portStateAction.Port().Link().SetPortNames([]string{p1.ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
		beforeEventChangeStr = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		ate.OTG().SetControlState(t, portStateAction)
	}

	time.Sleep(5 * time.Second)

	status := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State())

	var expectedStatus oc.E_Interface_OperStatus
	if actionState == "UP" {
		expectedStatus = oc.Interface_OperStatus_UP
	} else if actionState == "DOWN" {
		expectedStatus = oc.Interface_OperStatus_DOWN
	}

	// Validate the operational status
	if status != expectedStatus {
		t.Errorf("Interface %s status got %v, want %v", aggID, status, expectedStatus)
	} else {
		t.Logf("Interface %s status got %v, want %v", aggID, status, expectedStatus)
	}

	EventChange := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())
	// Convert the nanoseconds timestamp to a time.Time object in UTC
	eventChangeTime := time.Unix(0, int64(EventChange)).UTC()

	// convert string type change to time.time
	beforeEvent, err := time.Parse(time.RFC3339, beforeEventChangeStr)
	if err != nil {
		t.Fatalf("failed to parse event timestamp: %v", err)
	}

	t.Log("Compare if pre and post timestamps are the same for the last change before and after shut event")
	if preStateTS == EventChange {
		t.Fatalf("Before Trigger Last Change was %d after trigger Last Change was %d", preStateTS, EventChange)
	} else {
		t.Logf("Before Trigger Last Change was %d after trigger Last Change was %d", preStateTS, EventChange)
	}

	// Convert the parsed time to UTC
	beforeEventChange := beforeEvent.UTC()

	return beforeEventChange, eventChangeTime

}

// verifyPortsUp asserts that each port on the device is operating.
func verifyPortsStatus(t *testing.T, dut *ondatra.DUTDevice, portState string) {
	t.Helper()

	t.Logf("Checking Oper Status on %s", aggID)
	status := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State())

	// Determine the expected status based on the portState argument.
	var want oc.E_Interface_OperStatus
	if portState == "UP" {
		want = oc.Interface_OperStatus_UP
	} else {
		want = oc.Interface_OperStatus_DOWN
	}

	// Use a single if statement to check the status and log the result.
	if status != want {
		t.Fatalf("Failed: %s Status: got %v, want %v", aggID, status, want)
	} else {
		t.Logf("Pass: %s Status: got %v, want %v", aggID, status, want)
	}
}

func TestHoldTimeConfig(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	// top := gosnappi.NewConfig()
	dutPort1Intf = dut.Port(t, "port1")
	t.Run(fmt.Sprintf("configureDUT Interfaces"), func(t *testing.T) {
		// Configure the DUT
		aggID = netutil.NextAggregateInterface(t, dut)
		t.Log(dutPort1Intf)
		configureDUT(t, dut, aggID)

	})

	t.Run(fmt.Sprintf("Configure Hold Timers on DUT"), func(t *testing.T) {
		// Construct the hold-time config object
		holdTimeConfig := &oc.Interface_HoldTime{
			Up:   ygot.Uint32(upTimer),
			Down: ygot.Uint32(downTimer),
		}

		intfPath := gnmi.OC().Interface(dutPort1Intf.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)

	})

	t.Run(fmt.Sprintf("ConfigureOTG"), func(t *testing.T) {
		t.Logf("Configure ATE")
		configureOTG(t, ate, aggID)

	})

	t.Run(fmt.Sprintf("Verify Interface State for %s", aggID), func(t *testing.T) {
		// Verify Port Status
		t.Logf("Verifying port status for %s", aggID)
		time.Sleep(45 * time.Second)
		verifyPortsStatus(t, dut, "UP")

	})

}

func TestTC1ValidateTimersConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	holdTimePath := gnmi.OC().Interface(dutPort1Intf.Name()).HoldTime().State()

	holdTimeState := gnmi.Get(t, dut, holdTimePath)
	t.Log(holdTimeState)
	if *holdTimeState.Up == upTimer && *holdTimeState.Down == downTimer {
		t.Logf("Successfully configured times as up timer is %d and down timer"+
			" is %d", *holdTimeState.Up, *holdTimeState.Down)
	} else {
		t.Errorf("TC Failed: Configured up and down timers dont match what was configured "+
			"expected up %d got %d expected down %d got %d", upTimer, *holdTimeState.Up,
			downTimer, *holdTimeState.Down)
	}
}

func TestTC2LongDown(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	var beforeEventChange time.Time
	var eventTimeStampStr time.Time

	t.Run(fmt.Sprintf("Shut down OTG interface to cause remote fault on %s", aggID), func(t *testing.T) {

		beforeEventChange, eventTimeStampStr = flapOTGInterface(t, ate, dut, "DOWN")

		// Log the start and end times
		t.Logf("Shutdown triggered at: %v", beforeEventChange)
		t.Logf("Last change reported at: %v", eventTimeStampStr)

	})

	t.Run(fmt.Sprintf("Calculate fault duration on %s", aggID), func(t *testing.T) {

		// Calculate the difference in time
		duration := eventTimeStampStr.Sub(beforeEventChange)

		// Convert the duration to milliseconds
		durationInMS := duration.Milliseconds()
		t.Logf("Duration between shutdown triggered and last change reported: %v ms", durationInMS)

		if durationInMS >= downTimer {
			t.Logf("PASS: Duration is within the expected range; got %d MS", durationInMS)
		} else {
			t.Fatalf("FAIL: Expected duration to be at least %d MS; got %d MS", downTimer, durationInMS)
		}

	})

	t.Run(fmt.Sprintf("Bring back UP OTG Interface"), func(t *testing.T) {

		OTGInterfaceUP(t, ate)
		time.Sleep(45 * time.Second)
		t.Logf("Verifying port status for %s", aggID)
		verifyPortsStatus(t, dut, "UP")

	})

}

func TestTC3ShortUP(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run(fmt.Sprintf("Start sending Ethernet Remote Fault on OTG"), func(t *testing.T) {

		// shutting down OTG interface to emulate the RF
		OTGInterfaceDOWN(t, ate)
		oper1 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State())
		change1 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())
		t.Log(oper1)
		t.Log(change1)

		// bring port back up for 4 seconds below the 5000 ms hold up timer
		OTGInterfaceUP(t, ate)
		time.Sleep(4 * time.Second)

		// shut the OTG interface back to down state
		OTGInterfaceDOWN(t, ate)
		oper2 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State())
		change2 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())

		// ensure the LAG interface is still down
		verifyPortsStatus(t, dut, "DOWN")
		t.Log(oper2)

		change1Time := time.Unix(0, int64(change1)).UTC()
		change2Time := time.Unix(0, int64(change2)).UTC()

		// Compare the times and ensure there is no change in the last change
		if change1Time.Before(change2Time) || change1Time.After(change2Time) {
			t.Errorf("Time 1 %v and Time 2 dont match %v", change1Time, change2Time)
		} else if change1Time.Equal(change2Time) {
			t.Logf("Time 1 %v and Time 2 the the same which is expected %v", change1Time, change2Time)
		}

		// bring OTG port back up
		OTGInterfaceUP(t, ate)
		time.Sleep(45 * time.Second)
		// verify interface is up for next test case
		verifyPortsStatus(t, dut, "UP")

	})

}

func TestTC4SLongUP(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run(fmt.Sprintf("Start sending Ethernet Remote Fault on OTG"), func(t *testing.T) {

		// shutting down OTG interface to emulate the RF
		OTGInterfaceDOWN(t, ate)
		time.Sleep(1 * time.Second)
		change1 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())
		t.Log(change1)

		// bring port back up for 4 seconds below the 5000 ms hold up timer
		OTGInterfaceUP(t, ate)
		time.Sleep(30 * time.Second)

		// ensure the LAG interface is still down
		verifyPortsStatus(t, dut, "UP")

		// Collecting time stamp of interface up
		change2 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())

		change1Time := time.Unix(0, int64(change1)).UTC()
		change2Time := time.Unix(0, int64(change2)).UTC()

		// Calculate the difference in time
		duration := change2Time.Sub(change1Time)

		// Convert the duration to milliseconds
		durationInMS := duration.Milliseconds()
		t.Logf("Duration interface %v ms", durationInMS)

		if durationInMS >= upTimer {
			t.Logf("PASS: Expected interface up time delay of at least %v and got %v", upTimer, durationInMS)
		} else {
			t.Fatalf("FAIL: Expected interface up time delay of at least %v and got %v", upTimer, durationInMS)
		}

	})

}

func TestTC5ShortDOWN(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run(fmt.Sprintf("Start sending Ethernet Remote Fault on OTG"), func(t *testing.T) {

		t.Log("Verify Interface State before TC Start")
		verifyPortsStatus(t, dut, "UP")
		// shutting down OTG interface to emulate the RF
		// t.Log("Shutdown OTG Interface")
		OTGInterfaceDOWN(t, ate)
		change1 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())
		// t.Log("Bring OTG Interface Back UP")
		OTGInterfaceUP(t, ate)
		time.Sleep(3000 * time.Millisecond)
		// ensure the LAG interface is still down

		// Collecting time stamp of interface up
		change2 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())

		change1Time := time.Unix(0, int64(change1)).UTC()
		change2Time := time.Unix(0, int64(change2)).UTC()

		// Calculate the difference in time
		duration := change2Time.Sub(change1Time)

		// Convert the duration to milliseconds
		durationInMS := duration.Milliseconds()
		t.Logf("Duration interface %v ms", durationInMS)

		// Compare the times and ensure there is no change in the last change
		if change1Time.Before(change2Time) || change1Time.After(change2Time) {
			t.Fatalf("FAILED: Time 1 %v and Time 2 dont match %v", change1Time, change2Time)
		} else if change1Time.Equal(change2Time) {
			t.Logf("PASS: Time 1 %v and Time 2 the the same which is expected %v", change1Time, change2Time)
		}

		verifyPortsStatus(t, dut, "UP")

	})

}
