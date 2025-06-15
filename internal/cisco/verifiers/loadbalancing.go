// Package loadbalancing provides verifiers APIs to verify oper data for loadbalancing verifications.
package verifiers

import (
	// "time"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"os"
	// "text/tabwriter"
	// "fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/ondatra"

	// "github.com/openconfig/ondatra/gnmi"
	// "os"
	// "text/tabwriter"
	// "github.com/openconfig/ygnmi/ygnmi"
	// "github.com/openconfig/ondatra/gnmi/oc"
	"testing"
)

type LoadbalancingVerifier struct{}

type EgressLBDistribution struct {
	Weight           uint64
	OutPkts          uint64
	WantDistribution float64
	GotDistribution  float64
}

func (v *LoadbalancingVerifier) VerifyEgressDistributionPerWeight(t *testing.T, dut *ondatra.DUTDevice, outIFWeight map[string]uint64, totalInPackets uint64, trfDistTolerance float64) (map[string]EgressLBDistribution, bool) {
	distrStruct := EgressLBDistribution{}
	trafficDistribution := make(map[string]EgressLBDistribution)
	var balancedPerWeight bool = true // Initialize as true

	// Calculate total weight
	var wantWeights, gotWeights []float64
	var outPacketList, weightList []uint64
	// Iterate over interfaces and calculate distribution
	for intf, wt := range outIFWeight {
		weightList = append(weightList, wt)
		intfCounter := helper.Interface.GetPerInterfaceCounters(t, dut, intf)
		outPacketList = append(outPacketList, intfCounter.GetOutUnicastPkts())
		distrStruct.Weight = wt
		distrStruct.OutPkts = intfCounter.GetOutUnicastPkts()
		trafficDistribution[intf] = distrStruct
	}
	wantWeights, _ = helper.Loadbalancing.Normalize(weightList)
	gotWeights, _ = helper.Loadbalancing.Normalize(outPacketList)

	t.Log("compare", wantWeights, gotWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		balancedPerWeight = false
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
	// Print table with tabwriter
	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"Interface", "AFTWeight", "PacketCount", "WantDistribution", "GotDistribution"})
	index := 0
	for intf, data := range trafficDistribution {
		var wantDist, gotDist float64

		// Retrieve WantDistribution and GotDistribution by index
		if index < len(wantWeights) {
			wantDist = wantWeights[index]
		}
		if index < len(gotWeights) {
			gotDist = gotWeights[index]
		}

		// Update the data in the map
		data.WantDistribution = wantDist
		data.GotDistribution = gotDist
		trafficDistribution[intf] = data

		// Add a row to the table
		table.Append([]string{
			intf,
			fmt.Sprintf("%d", data.Weight),
			fmt.Sprintf("%d", data.OutPkts),
			fmt.Sprintf("%.2f", wantDist),
			fmt.Sprintf("%.2f", gotDist),
		})
		index++
	}
	table.Render()
	return trafficDistribution, balancedPerWeight
}
