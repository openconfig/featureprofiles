package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func BenchMark(ygnmiCli *ygnmi.Client) {
	ctx := context.Background()
	data, err := ygnmi.CollectAll(ctx, ygnmiCli, gnmi.OC().System().CpuAny().State()).Await()
	if err != nil {
		fmt.Printf("Error %v /n", err)
	}
	for _, memUse := range data {
		usedMem, _ := memUse.Val()
		fmt.Printf("Cpu info at %v : %v\n", memUse.Timestamp, PrettyPrint(usedMem))
	}
}

func CpuVerify(ygnmiCli *ygnmi.Client, isRunning *bool) {
	// oc leaves for memory do not work!! and cpu information require extra analysis, commenting this code for now
	go func() {
		for *isRunning {
			ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
			//defer cancel()
			data, err := ygnmi.CollectAll(ctx, ygnmiCli, gnmi.OC().System().CpuAny().State()).Await()
			if cancel != nil {
				fmt.Printf("Error %v /n", err)
			}
			if err != nil {
				fmt.Printf("Error %v /n", err)
			}
			for _, memUse := range data {
				usedMem, _ := memUse.Val()
				fmt.Printf("Cpu info at %v : %v\n", memUse.Timestamp, PrettyPrint(usedMem))
			}
		}
	}()
}

// func MemoryVerify(ygnmiCli *ygnmi.Client, isRunning *bool) {
// 	// oc leaves for memory do not work!! and cpu information require extra analysis, commenting this code for now
// 	go func() {
// 		for *isRunning {
// 			ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
// 			//defer cancel()
// 			data, err := ygnmi.Collect(ctx, ygnmiCli, gnmi.OC().System().State()).Await()
// 			PrettyPrint(data)
// 			if cancel != nil {
// 				fmt.Printf("Error %v /n", err)
// 			}
// 			if err != nil {
// 				fmt.Printf("Error %v /n", err)
// 			}
// 			for _, memUse := range data {
// 				usedMem, _ := memUse.Val()
// 				fmt.Printf("Cpu info at %v : %v\n", memUse.Timestamp, PrettyPrint(usedMem))
// 			}
// 		}
// 	}()
// }

// gnmi.OC().System().Memory().State()

func ControlPlaneVerification(ygnmiCli *ygnmi.Client) {
	// TODO1:Check for Crash
	// TODO2: Check for Traces
	// TODO3: Check for Memory Usage
	BenchMark(ygnmiCli)
}
