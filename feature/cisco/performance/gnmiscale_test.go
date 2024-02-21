package main

import (
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestGNMIUpdateScale(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	beforeTime := time.Now()
	for i := 0; i <= 10; i++ {
		gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test"+strconv.Itoa(i))
	}
	t.Logf("Time to do 100 gnmi update is %s", time.Since(beforeTime).String())
	if int(time.Since(beforeTime).Seconds()) >= 180 {
		t.Fatalf("GNMI Scale Took too long")
	}
}

// func TestGNMIBigSetRequest(t *testing.T) {
// 	// Perform a gNMI Set Request with 20 MB of Data
// 	dut := ondatra.DUT(t, "dut")
// 	RestartEmsd(t, dut)
// 	ControlPlaneVerification(ygnmiCli)
// }


func Setup(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	wg := CollectCpuData(t, dut, 50*time.Millisecond, 5*time.Second)
	defer wg.Wait()
}

func TestCpuCollector(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	wg := CollectCpuData(t, dut, 50*time.Millisecond, 5*time.Second)
	wg.Wait()
}

func TestEmsdRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	wg := CollectCpuData(t, dut, 1*time.Second, 60*time.Second)
	
	RestartEmsd(t, dut)
	
	wg.Wait()
}
