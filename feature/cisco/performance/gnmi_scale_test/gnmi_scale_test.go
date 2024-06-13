package gnmi_scale_test

import (
	"strconv"
	"testing"
	"time"

	perf "github.com/openconfig/featureprofiles/feature/cisco/performance"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCollector(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	collector, err := perf.RunCollector(t, dut, "GNMI", "GNMIUpdateScale", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer collector.EndCollector()
	
	time.Sleep(time.Second * 10)
}

func TestGNMIUpdateScale(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	beforeTime := time.Now()
	collector, err := perf.RunCollector(t, dut, "GNMI", "GNMIUpdateScale", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer collector.EndCollector()
	for i := 0; i <= 100; i++ {
		gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test"+strconv.Itoa(i))
	}
	t.Logf("Time to do 100 gnmi update is %s", time.Since(beforeTime).String())
	if int(time.Since(beforeTime).Seconds()) >= 180 {
		t.Fatalf("GNMI Scale Took too long")
	}
}

func TestGNMIBigSetRequest(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Perform a gNMI Set Request with 13 MB of Data
	collector, err := perf.RunCollector(t, dut, "GNMI", "GNMIBigSetRequest", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer collector.EndCollector()
	set := perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "../big_set.json"), true)
	err = perf.GNMIBigSetRequest(t, dut, set, 400000)

	if err != nil {
		t.Fatal(err)
	}
}

func TestEmsdRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Logf("Restarting emsd at %s", time.Now())
	// perf.RestartEmsd(t, dut)
	collector, err := perf.RunCollector(t, dut, "General", "EmsdRestart", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer collector.EndCollector()

	err = perf.RestartProcess(t, dut, "emsd")
	if err != nil {
		t.Fatal(err)
	}
}

func TestReloadLineCards(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())

	collector, err := perf.RunCollector(t, dut, "General", "ReloadLineCards", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer collector.EndCollector()
	err = perf.ReloadLineCards(t, dut)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReloadRouter(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())

	collector, err := perf.RunCollector(t, dut, "General", "ReloadRouter", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer collector.EndCollector()

	err = perf.ReloadRouter(t, dut)
	if err != nil {
		t.Fatal(err)
	}
}
