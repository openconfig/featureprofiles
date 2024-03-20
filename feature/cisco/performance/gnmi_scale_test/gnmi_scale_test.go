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

func TestGNMIUpdateScale(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	beforeTime := time.Now()
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
	numLeaves := 400000

	replace := true
	// Perform a gNMI Set Request with 13 MB of Data
	set := perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "../big_set.json"), replace)

	t.Logf("Starting collector at %s", time.Now())
	collector := perf.CollectAllData(t, dut, 25*time.Second, 5*time.Minute)

	t.Logf("Starting batch programming of %d leaves at %s", numLeaves, time.Now())
	perf.BatchSet(t, dut, set, numLeaves)
	t.Logf("Finished batch programming of %d leaves at %s", numLeaves, time.Now())

	collector.Wait()
	t.Logf("Collector finished at %s", time.Now())
}

func TestCpuCollector(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())
	perf.CollectCpuData(t, dut, 50*time.Millisecond, 5*time.Second).Wait()
	t.Logf("Collector finished at %s", time.Now())

	// TODO: tabulate once correct yang model is provided

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

func TestMemCollector(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting Memory data collection at %s", time.Now())
	perf.CollectMemData(t, dut, 50*time.Millisecond, 5*time.Second).Wait()
	t.Logf("Collector finished at %s", time.Now())
}

func TestEmsdRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())
	collector := perf.CollectAllData(t, dut, 4*time.Second, 30*time.Second)

	// guarantee a few timestamps before emsd restart occurs
	time.Sleep(5 * time.Second)

	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
	t.Logf("Restart emsd finished at %s", time.Now())

	collector.Wait()
	t.Logf("Collector finished at %s", time.Now())
}

func TestReloadLineCards(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())

	// synchronous snapshot before router reload begins
	perf.CollectAllData(t, dut, 2500*time.Millisecond, 6*time.Second).Wait()

	// background concurrent collection
	collector := perf.CollectAllData(t, dut, 25*time.Second, 5*time.Minute)

	t.Logf("Restarting Line Cards at %s", time.Now())
	perf.ReloadLineCards(t, dut)
	t.Logf("Line Cards restart finished at %s", time.Now())

	collector.Wait()
	t.Logf("Collector finished at %s", time.Now())
}

func TestReloadRouter(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Starting CPU data collection at %s", time.Now())

	// example of synchronous snapshot of cpu usage
	perf.CollectAllData(t, dut, 2500*time.Millisecond, 6*time.Second).Wait()

	t.Logf("Restarting Router at %s", time.Now())
	perf.ReloadRouter(t, dut)
	t.Logf("Router restart finished at %s", time.Now())

	perf.CollectAllData(t, dut, 2500*time.Millisecond, 6*time.Second).Wait()

	t.Log("Waiting on main thread")
	t.Logf("Collector finished at %s", time.Now())
}

func TestPathParser(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	paths := []string{
		"Cisco-IOS-XR-wdsysmon-fd-oper:system-monitoring/cpu-utilization",
		"Cisco-IOS-XR-wd-oper:watchdog/nodes/node/memory-state",
		"Cisco-IOS-XR-procmem-oper:processes-memory/nodes/node/process-ids/process-id",
	}
	for _, path := range paths {
		rawJson, err := perf.GetAllNativeModel(t, dut, path)
		if err != nil {
			t.Errorf("Path parsing failed: %s", err)
		}
		t.Logf("Json response for: \"%s\"\n %s\n", path, util.PrettyPrintJson(rawJson))
	}
}
