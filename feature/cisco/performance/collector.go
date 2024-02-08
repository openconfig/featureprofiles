package main

import (
	"context"
	"fmt"
	"time"

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

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

func CCpuVerify(ygnmiCli *ygnmi.Client, isRunning *bool) chan []*oc.System_Cpu {
	// oc leaves for memory do not work!! and cpu information require extra analysis, commenting this code for now
	memUseChan := make(chan []*oc.System_Cpu, 100)
	go func() {
		for *isRunning {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			//defer cancel()
			// data, err := ygnmi.CollectAll(ctx, ygnmiCli, gnmi.OC().System().CpuAny().State()).Await()
			// getall vs get vs collect)
			data, err := ygnmi.GetAll[*oc.System_Cpu](ctx, ygnmiCli, gnmi.OC().System().CpuAny().State())
			if cancel != nil {
				fmt.Printf("Error %v /n", err)
			}
			if err != nil {
				fmt.Printf("Error %v /n", err)
			}
			memUseChan <- data
			// for _, memUse := range data {
			// 	fmt.Printf("Cpu info at %v : %v\n", memUse.Timestamp, PrettyPrint(memUse))
			// 	memUseChan <- memUse
			// }
		}
	}()

	return memUseChan

}
