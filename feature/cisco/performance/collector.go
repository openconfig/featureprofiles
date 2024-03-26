package performance

import (
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

type Collector struct {
	sync.WaitGroup
	CpuLogs [][]*oc.System_Cpu
	// MemLogs []*oc.System_Memory
	MemLogs []MemData
}

func CollectAllData(t *testing.T, dut *ondatra.DUTDevice, frequency time.Duration, duration time.Duration) *Collector {
	t.Helper()
	collector := &Collector{
		CpuLogs: make([][]*oc.System_Cpu, 0),
		// MemLogs: make([]*oc.System_Memory, 0),
		MemLogs: make([]MemData, 0),
	}
	collector.Add(2)
	go receiveCpuData(t, getCpuData(t, dut, frequency, duration), collector)
	// go receiveMemData(t, getMemData(t, dut, frequency, duration), collector)
	go receiveMemData(t, getMemData(t, dut, frequency, duration), collector)
	return collector
}

func CollectCpuData(t *testing.T, dut *ondatra.DUTDevice, frequency time.Duration, duration time.Duration) *Collector {
	t.Helper()
	collector := &Collector{
		CpuLogs: make([][]*oc.System_Cpu, 0),
	}
	collector.Add(1)
	go receiveCpuData(t, getCpuData(t, dut, frequency, duration), collector)
	return collector
}

func CollectMemData(t *testing.T, dut *ondatra.DUTDevice, frequency time.Duration, duration time.Duration) *Collector {
	t.Helper()
	collector := &Collector{
		MemLogs: make([]MemData, 0),
	}
	collector.Add(1)
	go receiveMemData(t, getMemData(t, dut, frequency, duration), collector)
	return collector
}

func getCpuData(t *testing.T, dut *ondatra.DUTDevice, freq time.Duration, dur time.Duration) chan []*oc.System_Cpu {
	// oc leaves for memory do not work!! and cpu information require extra analysis, commenting this code for now
	t.Helper()
	cpuChan := make(chan []*oc.System_Cpu, 100)

	go func() {
		ticker := time.NewTicker(freq)
		timer := time.NewTimer(dur)
		done := false
		defer close(cpuChan)
		for !done {
			select {
			case <-ticker.C:
				var data []*oc.System_Cpu
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					data = gnmi.GetAll(t, dut, gnmi.OC().System().CpuAny().State())
					// Cisco-IOS-XR-wdsysmon-fd-oper:system-monitoring/cpu-utilization
					t.Logf("CPU Data: \n %s\n", util.PrettyPrintJson(data))
				}); errMsg != nil {
					t.Logf("CPU collector failed: %s", *errMsg)
					continue
				}
				cpuChan <- data
			case <-timer.C:
				done = true
			}
		}
	}()

	return cpuChan
}

func getMemData(t *testing.T, dut *ondatra.DUTDevice, freq time.Duration, dur time.Duration) chan MemData {
	// oc leaves for memory do not work!! and cpu information require extra analysis, commenting this code for now
	t.Helper()
	// memChan := make(chan *oc.System_Memory, 100)
	memChan := make(chan MemData, 100)

	go func() {
		ticker := time.NewTicker(freq)
		timer := time.NewTimer(dur)
		done := false
		defer close(memChan)
		for !done {
			select {
			case <-ticker.C:
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					// Cisco-IOS-XR-wd-oper:watchdog/nodes/node/memory-state
					// var data MemData
					// data, err := GetAllNativeModel(t, dut, "Cisco-IOS-XR-wd-oper:watchdog/nodes/node/memory-state")
					data, err := DeserializeMemData(t, dut)
					if err != nil {
						t.Logf("Memory collector failed: %s", err)
					}
					
					// Cisco-IOS-XR-procmem-oper:processes-memory/nodes/node/process-ids/process-id
					// nativeModelObj2, err := GetAllNativeModel(t, dut, "Cisco-IOS-XR-procmem-oper:processes-memory/nodes/node/process-ids/process-id")
					// if err != nil {
					// 	t.Logf("Memory collector failed: %s", err)
					// } else {
					// 	t.Logf("Mem Data: \n %s\n", util.PrettyPrintJson(nativeModelObj2))
					// }
					memChan <- *data
				}); errMsg != nil {
					t.Logf("Memory collector failed: %s", *errMsg)
					continue
				}
			case <-timer.C:
				done = true
			}
		}
	}()

	return memChan
}

func receiveCpuData(t *testing.T, cpuChan chan []*oc.System_Cpu, collector *Collector) {
	t.Helper()
	defer collector.Done()
	for cpuData := range cpuChan {
		// TODO: change from log to capture
		t.Logf("\nCPU INFO:, t: %s\n%s\n", time.Now(), util.PrettyPrintJson(cpuData))
		collector.CpuLogs = append(collector.CpuLogs, cpuData)
	}
}

func receiveMemData(t *testing.T, memChan chan MemData, collector *Collector) {
	t.Helper()
	defer collector.Done()
	for memData := range memChan {
		// TODO: change from log to capture
		t.Logf("\nMemory INFO:, t: %s\n%s\n", time.Now(), util.PrettyPrintJson(memData))
		collector.MemLogs = append(collector.MemLogs, memData)
	}
}
