package main

import (
	// "fmt"
	// "os"
	// "strconv"
	"testing"
	"time"

	// "github.com/markkurossi/tabulate"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	// "github.com/openconfig/ondatra/gnmi"
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

func TestGNMIBigSetRequest(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	numLeaves := 400000

	replace := true
	// Perform a gNMI Set Request with 13 MB of Data
	set := CreateInterfaceSetFromOCRoot(LoadJSONOC(t, "./big_set.json"), replace)

	t.Logf("Starting collector at %s", time.Now())
	colletor := CollectAllData(t, dut, 5*time.Second, 5*time.Minute)
	
	t.Logf("Starting batch programming of %d leaves at %s", numLeaves, time.Now())
	BatchSet(t, dut, set, numLeaves)
	t.Logf("Finished batch programming of %d leaves at %s", numLeaves, time.Now())

	colletor.Wait()
	t.Logf("Collector finished at %s", time.Now())
}

func TestCpuCollector(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())
	collector := CollectCpuData(t, dut, 50*time.Millisecond, 5*time.Second)
	collector.Wait()
	t.Logf("Collector finished at %s", time.Now())
	
	// tabulation
	// tab := tabulate.New(tabulate.ASCII)
	// err := tabulate.Reflect(tab, 0, nil, collector.CpuLogs)
	// if err != nil {
	// 	t.Errorf("Error tabulating data: %s", err)
	// }
	// fmt.Print("CPU Logs:\n")
	// tab.Print(os.Stdout)
	// tab2 := tabulate.New(tabulate.ASCII)
	// err = tabulate.Reflect(tab2, 0, nil, collector.MemLogs)
	// if err != nil {
	// 	t.Errorf("Error tabulating data: %s", err)
	// }
	// fmt.Print("Memory Logs:\n")
	// tab2.Print(os.Stdout)
	// 
	// t.Log("CPU data collection finished")
}

func TestEmsdRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())
	collector := CollectAllData(t, dut, 1*time.Second, 30*time.Second)

	// guarantee a few timestamps before emsd restart occurs
	time.Sleep(5*time.Second)

	t.Logf("Restarting emsd at %s", time.Now())
	RestartEmsd(t, dut)
	t.Logf("Restart emsd finished at %s", time.Now())

	collector.Wait()
	t.Logf("Collector finished at %s", time.Now())
}

func TestReloadLineCards(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())
	collector := CollectAllData(t, dut, 5*time.Second, 5*time.Minute)
	
	// guarantee a few timestamps before router reload occurs
	time.Sleep(15*time.Second)
	
	t.Logf("Restarting Line Cards at %s", time.Now())
	ReloadLineCards(t, dut)
	t.Logf("Line Cards restart finished at %s", time.Now())
	
	collector.Wait()
	t.Logf("Collector finished at %s", time.Now())
}

func TestReloadRouter(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())
	// collect a few timestamps before reloading router
	collector := CollectAllData(t, dut, 1*time.Second, 10*time.Second)
	collector.Wait()
	
	t.Logf("Restarting Router at %s", time.Now())
	ReloadRouter(t, dut)
	t.Logf("Router restart finished at %s", time.Now())

	collectorPostReboot := CollectAllData(t, dut, 5*time.Second, 2*time.Minute)
	
	t.Log("Waiting on main thread")
	collectorPostReboot.Wait()
	t.Logf("Collector finished at %s", time.Now())
}

