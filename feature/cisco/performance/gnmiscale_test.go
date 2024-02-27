package main

import (
	"os"
	"testing"
	"time"

	"github.com/markkurossi/tabulate"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

// func TestGNMIUpdateScale(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	beforeTime := time.Now()
// 	for i := 0; i <= 10; i++ {
// 		gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test"+strconv.Itoa(i))
// 	}
// 	t.Logf("Time to do 100 gnmi update is %s", time.Since(beforeTime).String())
// 	if int(time.Since(beforeTime).Seconds()) >= 180 {
// 		t.Fatalf("GNMI Scale Took too long")
// 	}
// }

// func TestGNMIBigSetRequest(t *testing.T) {
// 	// Perform a gNMI Set Request with 20 MB of Data
// 	dut := ondatra.DUT(t, "dut")
// 	RestartEmsd(t, dut)
// 	ControlPlaneVerification(ygnmiCli)
// }

func TestCpuCollector(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Log("Starting CPU data collection")
	collector := CollectAllData(t, dut, 50*time.Millisecond, 5*time.Second)
	collector.Wait()
	tab := tabulate.New(tabulate.ASCII)
	tab.Header("Time").SetAlign(tabulate.TL)
	tab.Header("Value")
	err := tabulate.Reflect(tab, 0, nil, collector.CpuLogs)
	if err != nil {
		t.Errorf("Error tabulating data: %s", err)
	}
	tab.Print(os.Stdout)
	err = tabulate.Reflect(tab, 0, nil, collector.MemLogs)
	if err != nil {
		t.Errorf("Error tabulating data: %s", err)
	}
	tab.Print(os.Stdout)
	t.Log("CPU data collection finished")
}

func TestEmsdRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Log("Starting CPU data collection")
	collector := CollectAllData(t, dut, 14*time.Second, 60*time.Second)

	// guarantee a few timestamps before emsd restart occurs
	time.Sleep(5*time.Second)

	t.Log("Restarting emsd")
	RestartEmsd(t, dut)

	collector.Wait()
	t.Log("CPU data collection finished")
}
