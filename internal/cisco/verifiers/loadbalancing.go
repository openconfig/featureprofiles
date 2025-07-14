// Package verifiers provides verifiers APIs to verify oper data for different component verifications.
package verifiers

import (
	// "time"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"

	// "text/tabwriter"
	// "fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	// "github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"testing"

	"github.com/openconfig/ondatra/gnmi/oc"
)

type loadbalancingVerifier struct{}

type EgressLBDistribution struct {
	Weight           uint64
	OutPkts          uint64
	OutPPS           uint64
	WantDistribution float64
	GotDistribution  float64
}

func (v *loadbalancingVerifier) VerifyPacketEgressDistributionPerWeight(t *testing.T, dut *ondatra.DUTDevice, outIFWeight map[string]uint64, trfDistTolerance float64, forBundle bool, trafficType string) (map[string]EgressLBDistribution, bool) {
	distrStruct := EgressLBDistribution{}
	trafficDistribution := make(map[string]EgressLBDistribution)
	var balancedPerWeight bool = true // Initialize as true

	// Calculate total weight
	var wantWeights, gotWeights []float64
	var outPacketList, weightList []uint64
	// Iterate over interfaces and calculate distribution
	for intf, wt := range outIFWeight {
		weightList = append(weightList, wt)
		var intfCounter *oc.Interface_Counters
		var intfV4Counter *oc.Interface_Subinterface_Ipv4_Counters
		var intfV6Coubter *oc.Interface_Subinterface_Ipv6_Counters
		if forBundle && trafficType == "ipv4" {
			intfV4Counter = helper.InterfaceHelper().GetPerInterfaceV4Counters(t, dut, intf)
			outPacketList = append(outPacketList, intfV4Counter.GetOutPkts())
			distrStruct.OutPkts = intfV4Counter.GetOutPkts()
		} else if forBundle && trafficType == "ipv6" {
			intfV6Coubter = helper.InterfaceHelper().GetPerInterfaceV6Counters(t, dut, intf)
			outPacketList = append(outPacketList, intfV6Coubter.GetOutPkts())
			distrStruct.OutPkts = intfV6Coubter.GetOutPkts()
		} else {
			intfCounter = helper.InterfaceHelper().GetPerInterfaceCounters(t, dut, intf)
			outPacketList = append(outPacketList, intfCounter.GetOutUnicastPkts())
			distrStruct.OutPkts = intfCounter.GetOutUnicastPkts()
		}

		distrStruct.Weight = wt
		trafficDistribution[intf] = distrStruct
	}
	wantWeights, _ = helper.LoadbalancingHelper().Normalize(weightList)
	gotWeights, _ = helper.LoadbalancingHelper().Normalize(outPacketList)

	t.Log("compare", wantWeights, gotWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		balancedPerWeight = false
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
	// Print table with tabwriter
	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"Device", "Interface", "Weight", "Out_Packet_Count", "Want_Distribution", "Got_Distribution"})
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
			dut.Name(),
			intf,
			fmt.Sprintf("%d", data.Weight),
			fmt.Sprintf("%d", data.OutPkts),
			fmt.Sprintf("%.4f", wantDist),
			fmt.Sprintf("%.4f", gotDist),
		})
		index++
	}
	table.Render()
	return trafficDistribution, balancedPerWeight
}

func (v *loadbalancingVerifier) VerifyPPSEgressDistributionPerWeight(t *testing.T, dut *ondatra.DUTDevice, outIFWeight map[string]uint64, trfDistTolerance float64, forBundle bool, trafficType string) (map[string]EgressLBDistribution, bool) {
	distrStruct := EgressLBDistribution{}
	trafficDistribution := make(map[string]EgressLBDistribution)
	var balancedPerWeight bool = true // Initialize as true

	// Calculate total weight
	var wantWeights, gotWeights []float64
	var outPacketPPS, weightList []uint64
	// Iterate over interfaces and calculate distribution
	for intf, wt := range outIFWeight {
		weightList = append(weightList, wt)
		distrStruct.OutPPS = GetInterfaceOutPPS(t, dut, intf)
		outPacketPPS = append(outPacketPPS, distrStruct.OutPPS)

		distrStruct.Weight = wt
		trafficDistribution[intf] = distrStruct
	}
	wantWeights, _ = helper.LoadbalancingHelper().Normalize(weightList)
	gotWeights, _ = helper.LoadbalancingHelper().Normalize(outPacketPPS)

	t.Log("compare", wantWeights, gotWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		balancedPerWeight = false
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
	// Print table with tabwriter
	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"Device", "Interface", "Weight", "OutPPS", "Want_Distribution", "Got_Distribution"})
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
			dut.Name(),
			intf,
			fmt.Sprintf("%d", data.Weight),
			fmt.Sprintf("%d", data.OutPPS),
			fmt.Sprintf("%.4f", wantDist),
			fmt.Sprintf("%.4f", gotDist),
		})
		index++
	}
	table.Render()
	return trafficDistribution, balancedPerWeight
}
