package performance

import (
	"context"
	"encoding/json"
	"fmt"

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

func ControlPlaneVerification(ygnmiCli *ygnmi.Client) {
	// TODO1:Check for Crash
	// TODO2: Check for Traces
	// TODO3: Check for Memory Usage
	BenchMark(ygnmiCli)
}
