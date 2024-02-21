package main

import (
	"sync"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func cpuVerify(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup, freq time.Duration, dur time.Duration) chan []*oc.System_Cpu {
	// oc leaves for memory do not work!! and cpu information require extra analysis, commenting this code for now
	t.Helper()
	cpuChan := make(chan []*oc.System_Cpu, 100)
	go func() {
		ticker := time.NewTicker(freq)
		timer := time.NewTimer(dur)
		done := false
		for !done {
			select {
			case <- ticker.C:
				data := gnmi.GetAll[*oc.System_Cpu](t, dut, gnmi.OC().System().CpuAny().State())
				cpuChan <- data
			case <- timer.C:
				close(cpuChan)
				done = true
			}
		}
	}()

	return cpuChan

}

func receiveCpuVerify(t *testing.T, cpuChan chan []*oc.System_Cpu, wg *sync.WaitGroup) {
	t.Helper()
	defer wg.Done()
	for cpuData := range cpuChan {
		// change from log to capture
		t.Logf("\nCPU INFO:\n%s\n", PrettyPrint(cpuData))
	}
}

func CollectCpuData(t *testing.T, dut *ondatra.DUTDevice, frequency time.Duration, duration time.Duration) *sync.WaitGroup {
	t.Helper()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go receiveCpuVerify(t, cpuVerify(t, dut, wg, frequency, duration), wg)
	return wg
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
