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
	}
	configureOTGBundle(t, top, ateAggPorts, aggID)

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
	ate *ondatra.ATEDevice,
	dut *ondatra.DUTDevice) time.Time {

	p1 := ondatra.ATE(t, "ate").Port(t, "port1")
	portStateAction := gosnappi.NewControlState()
	timestamp := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())

	timeObj, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		t.Errorf("Failed to parse time string: %v", timestamp)
		return timeObj
	}

	// make sure interface is not down
	portStateAction.Port().Link().SetPortNames([]string{p1.ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
	ate.OTG().SetControlState(t, portStateAction)

	return timeObj
}

func configureOTGBundle(t *testing.T,

	top gosnappi.Config,
	aggPorts []*ondatra.Port,
	aggID string) {
	t.Helper()
	agg := top.Lags().Add().SetName(lagName)
	lagID, _ := strconv.Atoi(aggID)
	agg.Protocol().Static().SetLagId(uint32(lagID))

	for i, p := range aggPorts {
		port := top.Ports().Add().SetName(p.ID())
		agg.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(ateSrc.MAC).SetName("LAGRx-" + strconv.Itoa(i))
	}

	dstDev := top.Devices().Add().SetName(agg.Name() + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(lagName + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(lagName + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
}

func displaySummaryTable(t *testing.T,
	preActionTS,
	postActionTS string,
	actualDurationInMS,
	minToleranceInMS,
	maxToleranceInMS int64,
	expectedOperStatus,
	actualOperStatus string,
	pass bool) {

	result := "FAIL"
	if pass {
		result = "PASS"
	}

	// Prepare the strings for output.
	expectedDurationStr := "300ms" // Assuming this is a constant value

	minToleranceStr := strconv.Itoa(int(minToleranceInMS)) + "ms"
	maxToleranceStr := strconv.Itoa(int(maxToleranceInMS)) + "ms"
	actualDurationStr := strconv.Itoa(int(actualDurationInMS)) + "ms"

	// Create a slice of metrics and corresponding values.
	metrics := []string{"Pre-action TS",
		"Post-action TS",
		"Expected Duration",
		"Actual Duration",
		"Min Tolerance",
		"Max Tolerance",
		"Expected Oper Status",
		"Actual Oper Status",
		"Result"}
	values := []string{preActionTS,
		postActionTS,
		expectedDurationStr,
		actualDurationStr,
		minToleranceStr,
		maxToleranceStr,
		expectedOperStatus,
		actualOperStatus,
		result}

	// Find the maximum width for the metrics to align the values.
	maxMetricWidth := 0
	for _, metric := range metrics {
		if len(metric) > maxMetricWidth {
			maxMetricWidth = len(metric)
		}
	}

	// Create the vertical table.
	table := ""
	for i, metric := range metrics {
		table += fmt.Sprintf("%-*s: %s\n", maxMetricWidth, metric, values[i])
	}

	t.Logf("\n%s", table)
}

func flapOTGInterface(t *testing.T,
	ate *ondatra.ATEDevice,
	dut *ondatra.DUTDevice,
	actionState string) (time.Time, time.Time, string, string) {

	// Shut down OTG Interface
	p1 := ondatra.ATE(t, "ate").Port(t, "port1")
	portStateAction := gosnappi.NewControlState()

	var otgStateChangeTsStr string

	// TC2 Step 1 Read timestamp of last oper-status change  form DUT port-1
	preStateTSSTR := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())
	DutLastChangeTS1 := time.Unix(0, int64(preStateTSSTR)).UTC().Format(time.RFC3339Nano)
	t.Logf("Step1. DutLastChangeTS1 is: %v", DutLastChangeTS1)
	if actionState == "UP" {
		portStateAction.Port().Link().SetPortNames([]string{p1.ID()}).SetState(gosnappi.StatePortLinkState.UP)
		otgStateChangeTsStr = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		ate.OTG().SetControlState(t, portStateAction)
	} else if actionState == "DOWN" {
		// TC2 Step 2 Bring Down OTG Interface
		t.Log("RT-5.5.2: Bring Down OTG Interface")
		portStateAction.Port().Link().SetPortNames([]string{p1.ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
		otgStateChangeTsStr = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		ate.OTG().SetControlState(t, portStateAction)

		// TC2 Step 3
		t.Log("Step 3 sleeping 500ms")
		time.Sleep(500 * time.Millisecond)
	}

	// Step 4. Read timestamp of last oper-status change  form DUT port-1 (DUT_LAST_CHANGE_TS)
	postStateTSSTR := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())
	DutLastChangeTS2STR := time.Unix(0, int64(postStateTSSTR)).UTC().Format(time.RFC3339Nano)
	DutLastChangeOper2 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State())

	var expectedStatus oc.E_Interface_OperStatus
	if actionState == "UP" {
		expectedStatus = oc.Interface_OperStatus_UP
	} else if actionState == "DOWN" {
		expectedStatus = oc.Interface_OperStatus_DOWN
	}

	// Step 5. verify oper-status is DOWN
	if DutLastChangeOper2 != expectedStatus {
		t.Errorf("Interface %s status got %v, want %v", aggID, DutLastChangeTS2STR, expectedStatus.String())
	} else {
		t.Logf("Interface %s status got %v, want %v", aggID, DutLastChangeTS2STR, expectedStatus.String())
	}

	// convert string type change to time.time
	otgStateChangeTs, err := time.Parse(time.RFC3339Nano, otgStateChangeTsStr)
	if err != nil {
		t.Fatalf("failed to parse event timestamp: %v %v", err, otgStateChangeTs)
	}

	DutLastChangeTS2, err := time.Parse(time.RFC3339Nano, DutLastChangeTS2STR)
	if err != nil {
		t.Fatalf("failed to parse event timestamp: %v %v", err, DutLastChangeTS2)
	}

	// Step 6. verify oper-status last change time has changed
	t.Log("Compare if pre and post timestamps are the same for the last change before and after shut event")
	if DutLastChangeTS1 == DutLastChangeTS2STR {
		t.Fatalf("Before Trigger Last Change was %v after trigger Last Change was %v", DutLastChangeTS1, DutLastChangeTS2STR)
	} else {
		t.Logf("Before Trigger Last Change was %v after trigger Last Change was %v", DutLastChangeTS1, DutLastChangeTS2STR)
	}

	// convert to time objects
	otgStateChangeTs = otgStateChangeTs.UTC()
	DutLastChangeTS2 = DutLastChangeTS2.UTC()

	return otgStateChangeTs, DutLastChangeTS2, expectedStatus.String(), DutLastChangeOper2.String()

}

// verifyPortsUp asserts that each port on the device is operating.
func verifyPortsStatus(t *testing.T, dut *ondatra.DUTDevice, portState string, waitTime time.Duration) {
	t.Helper()

	t.Logf("Checking Oper Status on %s", aggID)

	// Determine the expected status based on the portState argument.
	var want oc.E_Interface_OperStatus
	if portState == "UP" {
		want = oc.Interface_OperStatus_UP
		gnmi.Await(t, dut,
			gnmi.OC().Interface(aggID).OperStatus().State(),
			time.Second*waitTime,
			oc.Interface_OperStatus_UP)
	} else {
		want = oc.Interface_OperStatus_DOWN
		gnmi.Await(t, dut,
			gnmi.OC().Interface(aggID).OperStatus().State(),
			time.Second*waitTime,
			oc.Interface_OperStatus_DOWN)
	}

	status := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State())

	// check the status and log the result.
	if status != want {
		t.Fatalf("Failed: %s Status: got %v, want %v", aggID, status, want)
	} else {
		t.Logf("Pass: %s Status: got %v, want %v", aggID, status, want)
	}
}

func TestHoldTimeConfig(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	dutPort1Intf = dut.Port(t, "port1")
	t.Run("ConfigureDUT Interfaces", func(t *testing.T) {
		// Configure the DUT
		aggID = netutil.NextAggregateInterface(t, dut)
		t.Log(dutPort1Intf)
		configureDUT(t, dut, aggID)

	})

	t.Run("Configure Hold Timers on DUT", func(t *testing.T) {
		// Construct the hold-time config object
		holdTimeConfig := &oc.Interface_HoldTime{
			Up:   ygot.Uint32(upTimer),
			Down: ygot.Uint32(downTimer),
		}

		intfPath := gnmi.OC().Interface(dutPort1Intf.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)

	})

	t.Run("ConfigureOTG", func(t *testing.T) {
		t.Logf("Configure ATE")
		configureOTG(t, ate, aggID)

	})

	t.Run(fmt.Sprintf("Verify Interface State for %s", aggID), func(t *testing.T) {
		// Verify Port Status
		t.Logf("Verifying port status for %s", aggID)
		verifyPortsStatus(t, dut, "UP", 45)
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

	var otgStateChangeTs, DutLastChangeTS2 time.Time
	var expectedOper, actualOper string

	t.Run(fmt.Sprintf("Shut down OTG interface to cause remote fault on %s", aggID), func(t *testing.T) {
		otgStateChangeTs, DutLastChangeTS2, expectedOper, actualOper = flapOTGInterface(t, ate, dut, "DOWN")
		if expectedOper != actualOper {
			t.Errorf("expectedOper and actualOper do not match: expected %s, got %s", expectedOper, actualOper)
		}
	})

	duration := DutLastChangeTS2.Sub(otgStateChangeTs)
	durationInMS := duration.Milliseconds()

	// Define the expected delay and tolerance
	expectedDelayMS := 300 // Expected delay in milliseconds
	minDuration := int64(expectedDelayMS - toleranceMS)
	maxDuration := int64(expectedDelayMS + toleranceMS)

	// Check if the actual duration falls within the expected range
	pass := durationInMS <= maxDuration

	t.Run(fmt.Sprintf("Calculate fault duration on %s", aggID), func(t *testing.T) {
		t.Logf("Shutdown triggered at: %v", otgStateChangeTs)
		t.Logf("Last change reported at: %v", DutLastChangeTS2)
		t.Logf("Duration between shutdown triggered and last change reported: %v ms", durationInMS)

		if pass {
			t.Logf("PASS: Duration is within the expected range; got %d ms", durationInMS)
		} else {
			t.Errorf("FAIL: Expected duration to be within %d ms to %d ms; got %d ms", minDuration, maxDuration, durationInMS)
		}
	})

	t.Run("Bring back UP OTG Interface", func(t *testing.T) {
		OTGInterfaceUP(t, ate)
		t.Logf("Verifying port status for %s", aggID)
		verifyPortsStatus(t, dut, "UP", 45)
	})

	t.Run("Verify test results", func(t *testing.T) {
		displaySummaryTable(t, otgStateChangeTs.Format(time.RFC3339Nano), DutLastChangeTS2.Format(time.RFC3339Nano),
			durationInMS, minDuration, maxDuration, expectedOper, actualOper, pass)

	})

}

func TestTC3ShortUP(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("Start sending Ethernet Remote Fault on OTG", func(t *testing.T) {

		// shutting down OTG interface to emulate the RF
		OTGInterfaceDOWN(t, ate, dut)
		oper1 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State())
		change1 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())
		t.Log(oper1)
		t.Log(change1)

		// bring port back up for 4 seconds below the 5000 ms hold up timer
		OTGInterfaceUP(t, ate)
		// shut the OTG interface back to down state
		OTGInterfaceDOWN(t, ate, dut)
		oper2 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State())
		change2 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())

		// ensure the LAG interface is still down
		verifyPortsStatus(t, dut, "DOWN", 4)
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
		// verify interface is up for next test case
		verifyPortsStatus(t, dut, "UP", 45)

	})

}

func TestTC4SLongUP(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("Start sending Ethernet Remote Fault on OTG", func(t *testing.T) {

		// shutting down OTG interface to emulate the RF
		OTGInterfaceDOWN(t, ate, dut)
		time.Sleep(1 * time.Second)
		change1 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).LastChange().State())
		t.Log(change1)

		// bring port back up for 4 seconds below the 5000 ms hold up timer
		OTGInterfaceUP(t, ate)
		// ensure the LAG interface is still down
		verifyPortsStatus(t, dut, "UP", 30)

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

	var time1 time.Time
	var change1 *oc.Interface

	// Construct the hold-time config object
	holdTimeConfig := &oc.Interface_HoldTime{
		Up:   ygot.Uint32(upTimer),
		Down: ygot.Uint32(2000),
	}

	t.Run("Update hold timer configs down", func(t *testing.T) {
		intfPath := gnmi.OC().Interface(dutPort1Intf.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)

	})

	t.Run("Flap OTG Interfaces", func(t *testing.T) {

		t.Log("Verify Interface State before TC Start")
		verifyPortsStatus(t, dut, "UP", 10)
		// shutting down OTG interface to emulate the RF
		t.Log("Shutdown OTG Interface")
		change1 = gnmi.Get(t, dut, gnmi.OC().Interface(aggID).State())
		t.Logf("change1 last change is %v and status is %v", change1.LastChange, change1.AdminStatus)

		time1 = OTGInterfaceDOWN(t, ate, dut)
		time.Sleep(200 * time.Millisecond)
		t.Log("Bring OTG Interface Back UP")
		OTGInterfaceUP(t, ate)

	})

	t.Run("Verify Short Down Results", func(t *testing.T) {

		// Start building the log message
		logMessage := "Interface Status Timeline\n" +
			"----------------------------------------------------\n" +
			"Event                   | Time                             | Oper Status\n" +
			"----------------------------------------------------\n" +
			"Last-change time 1      | %v                               | %v\n" +
			"Trigger Start Time      | %v                               | -\n" +
			"Last-change Re-check    | %v                               | %v\n"

		change2 := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).State())

		if *change2.LastChange == *change1.LastChange && change2.OperStatus == change1.OperStatus {
			time2 := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())

			// Dereference the value and convert to int64 before passing to time.Unix function
			change2LastChangeTime := time.Unix(0, int64(*change2.LastChange)).UTC().Format(time.RFC3339Nano)
			change1LastChangeTime := time.Unix(0, int64(*change1.LastChange)).UTC().Format(time.RFC3339Nano)
			t1 := time1.UTC().Format(time.RFC3339Nano)
			t2, err := time.Parse(time.RFC3339Nano, time2)
			if err != nil {
				t.Errorf("Failed to parse time string: %v", err)
				return
			}

			timeDiff := t2.Sub(time1).Milliseconds()

			logMessage += fmt.Sprintf("End Time                | %v                               | -\n"+
				"-----------------------------------------------------\n"+
				"Total Elapsed Time: %vms\n", t2, timeDiff)
			t.Logf(logMessage, change1LastChangeTime, change1.OperStatus, t1, change2LastChangeTime, change2.OperStatus)

		} else {
			// Dereference the value and convert to int64 before passing to time.Unix function
			change2LastChangeTime := time.Unix(0, int64(*change2.LastChange)).UTC().Format(time.RFC3339Nano)
			change1LastChangeTime := time.Unix(0, int64(*change1.LastChange)).UTC().Format(time.RFC3339Nano)
			t1 := time1.UTC().Format(time.RFC3339Nano)

			// Log failure message and the partially built log message without end time
			t.Log("Failed due to an unexpected match such as last-change time or interface oper-status")
			t.Fatalf(logMessage, change1LastChangeTime, change1.OperStatus, t1, change2LastChangeTime, change2.OperStatus)
		}
	})

	t.Run("Verify port status UP", func(t *testing.T) {
		t.Log("re-verify that the interface state is still up")
		verifyPortsStatus(t, dut, "UP", 10)

	})
}
