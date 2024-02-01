package main

import (
	"context"
	"fmt"
	"time"

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

func CCpuVerify(ygnmiCli *ygnmi.Client, isRunning *bool) {
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
