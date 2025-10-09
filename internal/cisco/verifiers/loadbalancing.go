// Package verifiers offers APIs to verify operational data for components.
package verifiers

import (
	"context"
	// "time"
	"fmt"
	"os"
	"strconv"
	// "text/tabwriter"
	// "fmt"
	"testing"

	"github.com/olekukonko/tablewriter"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	textfsm "github.com/openconfig/featureprofiles/exec/utils/textfsm/textfsm"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"

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

type extendedEntropy struct {
	offset1 int
	offset2 int
	width1  int
	width2  int
}

type HashingParameters struct {
	LCId            string
	NPUId           int
	EcmpSeed        uint16
	SpaSeed         uint16
	LBNodeID        int
	HardShift       int
	SoftShift       int
	RTFValue        int
	extendedEntropy map[string]extendedEntropy
}

// VerifyPacketEgressDistributionPerWeight verifies if loadbalancing distribution packet count is per give interface:weight map using OC IF counters.
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
		err := table.Append([]string{
			dut.Name(),
			intf,
			fmt.Sprintf("%d", data.Weight),
			fmt.Sprintf("%d", data.OutPkts),
			fmt.Sprintf("%.4f", wantDist),
			fmt.Sprintf("%.4f", gotDist),
		})
		if err != nil {
			return nil, false
		}
		index++
	}
	table.Render()
	return trafficDistribution, balancedPerWeight
}

// VerifyPPSEgressDistributionPerWeight verifies if loadbalancing distribution PPS is per given interface:weight map, using show interface CLI.
func (v *loadbalancingVerifier) VerifyPPSEgressDistributionPerWeight(t *testing.T, dut *ondatra.DUTDevice, outIFWeight map[string]uint64, trfDistTolerance float64, OutIFWeightNormalized map[string]float64, bunIntfName ...string) (map[string]EgressLBDistribution, bool) {
	distrStruct := EgressLBDistribution{}
	trafficDistribution := make(map[string]EgressLBDistribution)
	var balancedPerWeight bool = true // Initialize as true

	// Calculate total weight
	var wantWeights, gotWeights []float64
	var outPacketPPS, weightList []uint64
	// Iterate over interfaces and calculate distribution

	// Normalize weights and PPS to compare ratios
	if len(OutIFWeightNormalized) == 0 {
		for intf, wt := range outIFWeight {
			weightList = append(weightList, wt)
			distrStruct.OutPPS = Interfaceverifier().GetInterfaceOutPPS(t, dut, intf)
			outPacketPPS = append(outPacketPPS, distrStruct.OutPPS)
			distrStruct.Weight = wt
			trafficDistribution[intf] = distrStruct
			wantWeights, _ = helper.LoadbalancingHelper().Normalize(weightList)
		}
	} else {
		for intf, wt := range OutIFWeightNormalized {
			distrStruct.OutPPS = Interfaceverifier().GetInterfaceOutPPS(t, dut, intf)
			outPacketPPS = append(outPacketPPS, distrStruct.OutPPS)
			distrStruct.Weight = uint64(wt)
			trafficDistribution[intf] = distrStruct
			wantWeights = append(wantWeights, wt)
		}
	}

	// Check if all PPS values are zero - indicates no traffic
	totalPPS := uint64(0)
	for _, pps := range outPacketPPS {
		totalPPS += pps
	}

	if totalPPS == 0 {
		t.Errorf("WARNING: All interfaces show 0 PPS - no traffic detected. Check traffic timing and ensure traffic is running when collecting stats.")
		t.Logf("DEBUG: Interface PPS values: %v", outPacketPPS)
		t.Logf("DEBUG: Interfaces checked: %v", func() []string {
			var intfs []string
			if len(OutIFWeightNormalized) == 0 {
				for intf := range outIFWeight {
					intfs = append(intfs, intf)
				}
			} else {
				for intf := range OutIFWeightNormalized {
					intfs = append(intfs, intf)
				}
			}
			return intfs
		}())

		// Return early with error indication but don't crash
		for intf, data := range trafficDistribution {
			data.WantDistribution = 0.0
			data.GotDistribution = 0.0
			trafficDistribution[intf] = data
		}
		return trafficDistribution, false
	}

	gotWeights, _ = helper.LoadbalancingHelper().Normalize(outPacketPPS)

	t.Log("compare", wantWeights, gotWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		balancedPerWeight = false
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}

	//bundleName only for when measuring distrubution across bundle members, and need to add name in table.
	bundleName := ""

	// Print table with tabwriter
	table := tablewriter.NewWriter(os.Stdout)
	if len(bunIntfName) > 0 {
		bundleName = bunIntfName[0]
		table.Header([]string{"Device", "BundleInterface", "MemberInterface", "Weight", "OutPPS", "Want_Distribution", "Got_Distribution"})
	} else {
		table.Header([]string{"Device", "Interface", "Weight", "OutPPS", "Want_Distribution", "Got_Distribution"})
	}

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
		var row []string
		if len(bunIntfName) > 0 {
			row = []string{
				dut.Name(), // Device name
				bundleName, // Bundle interface name
				intf,       // Member interface name
				fmt.Sprintf("%d", data.Weight),
				fmt.Sprintf("%d", data.OutPPS),
				fmt.Sprintf("%.4f", wantDist),
				fmt.Sprintf("%.4f", gotDist),
			}
		} else {
			row = []string{
				dut.Name(), // Device name
				intf,       // Bundle interface name
				fmt.Sprintf("%d", data.Weight),
				fmt.Sprintf("%d", data.OutPPS),
				fmt.Sprintf("%.4f", wantDist),
				fmt.Sprintf("%.4f", gotDist),
			}
		}
		// Prepare row data

		// Add the row to the table
		err := table.Append(row)
		if err != nil {
			return nil, false
		}
		index++
	}
	table.Render()
	return trafficDistribution, balancedPerWeight
}

func (v *loadbalancingVerifier) GetDumpPolarizationDebugCLI(t *testing.T, ctx context.Context, dut *ondatra.DUTDevice, lcIDs ...string) map[string][]HashingParameters {
	result := make(map[string][]HashingParameters)

	// Helper to parse seeds that may be hex (0xNNN) or decimal.
	parseSeed := func(fieldName, val string) uint16 {
		parsed, err := strconv.ParseInt(val, 0, 64) // base 0 handles 0x and decimal
		if err != nil {
			t.Fatalf("Failed to parse %s value %q: %v", fieldName, val, err)
		}
		return uint16(parsed)
	}

	for _, lcID := range lcIDs {
		cliOutput := config.CMDViaGNMI(ctx, t, dut,
			fmt.Sprintf("show controllers npu debugshell 0 'script lb_hash_info dump_polarization_info_all_np' location %s", lcID))

		dumpPolarizeTextfsm := textfsm.ShowDebugDumpPolarization{}
		if err := dumpPolarizeTextfsm.Parse(cliOutput); err != nil {
			t.Fatalf("Failed to parse output for LC %s: %v", lcID, err)
		}
		t.Logf("LC %s raw parsed rows: %+v", lcID, dumpPolarizeTextfsm.Rows)

		// Per-NPU aggregation
		npuParams := make(map[string]*HashingParameters)

		for _, entry := range dumpPolarizeTextfsm.Rows {
			// Skip empty / malformed rows
			if entry.Npu == "" || entry.Type == "" {
				continue
			}

			npuID := entry.Npu
			hp, exists := npuParams[npuID]
			if !exists {
				hp = &HashingParameters{
					LCId:            lcID,
					NPUId:           util.StringToInt(t, entry.Npu),
					EcmpSeed:        parseSeed("ECMP seed", entry.EcmpHashSeed),
					SpaSeed:         parseSeed("Bundle seed", entry.BundleSeed),
					LBNodeID:        util.StringToInt(t, entry.LbNodeId),
					HardShift:       util.StringToInt(t, entry.Hard),
					SoftShift:       util.StringToInt(t, entry.Soft),
					RTFValue:        util.StringToInt(t, entry.Rtf),
					extendedEntropy: make(map[string]extendedEntropy),
				}
				npuParams[npuID] = hp
			} else {
				// Re-parse seeds for consistency check (no util.StringToInt on hex)
				ecmpParsed := parseSeed("ECMP seed", entry.EcmpHashSeed)
				spaParsed := parseSeed("Bundle seed", entry.BundleSeed)
				if hp.EcmpSeed != ecmpParsed || hp.SpaSeed != spaParsed ||
					hp.LBNodeID != util.StringToInt(t, entry.LbNodeId) {
					t.Logf("Warning: inconsistent scalar parameters for LC %s NPU %s", lcID, npuID)
				}
			}

			// Attach / update extended entropy per type
			hp.extendedEntropy[entry.Type] = extendedEntropy{
				offset1: util.StringToInt(t, entry.Offset1),
				width1:  util.StringToInt(t, entry.Width1),
				offset2: util.StringToInt(t, entry.Offset2),
				width2:  util.StringToInt(t, entry.Width2),
			}
		}

		// Flatten per LC
		var hashingParamsList []HashingParameters
		for _, hp := range npuParams {
			hashingParamsList = append(hashingParamsList, *hp)
		}
		result[lcID] = hashingParamsList
	}

	t.Log("Hashing parameters:", result)
	return result
}
