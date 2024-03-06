package main

import (
	// "context"
	// "encoding/json"
	"sync"
	"testing"
	"time"

	// gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

type Collector struct {
	sync.WaitGroup
	CpuLogs [][]*oc.System_Cpu
	MemLogs []*oc.System_Memory
}

func CollectAllData(t *testing.T, dut *ondatra.DUTDevice, frequency time.Duration, duration time.Duration) *Collector {
	t.Helper()
	collector := &Collector{
		CpuLogs: make([][]*oc.System_Cpu, 0),
		MemLogs: make([]*oc.System_Memory, 0),
	}
	collector.Add(2)
	go receiveCpuData(t, getCpuData(t, dut, frequency, duration), collector)
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
		MemLogs: make([]*oc.System_Memory, 0),
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
					data = gnmi.GetAll[*oc.System_Cpu](t, dut, gnmi.OC().System().CpuAny().State())
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

func getMemData(t *testing.T, dut *ondatra.DUTDevice, freq time.Duration, dur time.Duration) chan *oc.System_Memory {
	// oc leaves for memory do not work!! and cpu information require extra analysis, commenting this code for now
	t.Helper()
	memChan := make(chan *oc.System_Memory, 100)

	go func() {
		ticker := time.NewTicker(freq)
		timer := time.NewTimer(dur)
		done := false
		defer close(memChan)
		for !done {
			select {
			case <-ticker.C:
				var data *oc.System_Memory
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					data = gnmi.Get[*oc.System_Memory](t, dut, gnmi.OC().System().Memory().State())
				}); errMsg != nil {
					t.Logf("Memory collector failed: %s", *errMsg)
					continue
				}
				memChan <- data
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
		// change from log to capture
		t.Logf("\nCPU INFO:, t: %s\n%s\n", time.Now(), PrettyPrint(cpuData))
		collector.CpuLogs = append(collector.CpuLogs, cpuData)
	}
}

func receiveMemData(t *testing.T, memChan chan *oc.System_Memory, collector *Collector) {
	t.Helper()
	defer collector.Done()
	for memData := range memChan {
		// change from log to capture
		t.Logf("\nMemory INFO:, t: %s\n%s\n", time.Now(), PrettyPrint(memData))
		collector.MemLogs = append(collector.MemLogs, memData)
	}
}

// func AsyncVerify[T func()ygnmi.SingletonQuery[T]](ygnmiCli *ygnmi.Client) chan *ygnmi.Collector[T] {
//
// 	collectorChan := make(chan *ygnmi.Collector[*oc.System_Cpu], 100)
// 	go func() {
// 		for {
// 			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
// 			//defer cancel()
// 			// data, err := ygnmi.CollectAll(ctx, ygnmiCli, gnmi.OC().System().CpuAny().State()).Await()
// 			data := ygnmi.Get(ctx, ygnmiCli, reflect.ValueOf(T).Call().(T))
// 			// gnmi.OC().System().CpuAny().State()
// 			// if cancel != nil {
// 			// 	fmt.Printf("Error %v /n", err)
// 			// }
// 			// if err != nil {
// 			// 	fmt.Printf("Error %v /n", err)
// 			// }
// 			collectorChan <- data
// 		}
// 	}()
//
// 	return collectorChan
// }
