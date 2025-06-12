package util

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	// DefaultVerifierTolerance is for default tolerance value
	DefaultVerifierTolerance = float64(0.01)
	// DefaultTelemtryInterval is for default telemetry interval
	DefaultTelemtryInterval = time.Duration(35 * time.Second)
)

// CalculateDistribution computes the fraction of each input out of the total sum.
// All input is assumed to be non-negative. The result's sum should be equal to 1
// unless all input is 0, in which case all 0s are returned.
func CalculateDistribution(data []float64) []float64 {
	distribution := make([]float64, len(data))
	total := float64(0)
	for _, v := range data {
		total += v
	}
	if total != 0 {
		for i, v := range data {
			distribution[i] = v / total
		}
	}
	return distribution
}

// CheckRates verifies that the provided rates are equal to expected within a tolerance
// following the formula: |rates[i]-expected[i]| <= tolerance * expected[i].
// An optional tolerance in range [0,1] can be provided to override default.
// Returns the success/fail for each entry and percent error (edge cases: 0 if 0/0 and -1 is non-zero/0)
func CheckRates(rates []float64, expected []float64, tolerance ...float64) ([]bool, []float64, error) {
	n := len(rates)
	if n == 0 {
		return nil, nil, errors.New("CheckRates input must have at least one rate")
	} else if n > len(expected) {
		return nil, nil, fmt.Errorf("CheckRates input had %d rates, but only %d expected rates", n, len(expected))
	}
	t := DefaultVerifierTolerance
	if len(tolerance) > 0 {
		t = tolerance[0]
	}
	if t < 0 || t > 1 {
		return nil, nil, fmt.Errorf("CheckRates tolerance %f must be within range of 0 to 1", t)
	}

	success := make([]bool, n)
	percenterror := make([]float64, n)
	for i, r := range rates {
		difference := math.Abs(r - expected[i])
		success[i] = (difference <= t*expected[i])
		if expected[i] != 0.0 {
			percenterror[i] = 100.0 * difference / expected[i]
		} else if difference != 0.0 {
			percenterror[i] = -1.0
		}
	}
	return success, percenterror, nil
}

// CheckRatesPercent verifies that the provided rates are distributed within a tolerance
// of the expectedPercent values. It is assumed that SUM(expectedPercent) == 100.
// An optional tolerance in range [0,1] can be provided to override default.
func CheckRatesPercent(rates []float64, expectedPercent []float64, tolerance ...float64) ([]bool, []float64, error) {
	n := len(rates)
	if n == 0 {
		return nil, nil, errors.New("CheckRatesPercent input must have at least one rate")
	}
	total := float64(0)
	for _, r := range rates {
		total += r
	}
	expected := make([]float64, n)
	for i, p := range expectedPercent {
		expected[i] = p * total / 100.0
	}
	return CheckRates(rates, expected, tolerance...)
}

// CheckEqualRates verifies that the provided rates are equally distributed within a tolerance
// following the formula: |rates[i] - AVG(rates)| <= tolerance * AVG(rates).
// An optional tolerance in range [0,1] can be provided to override default.
func CheckEqualRates(rates []float64, tolerance ...float64) ([]bool, []float64, error) {
	n := len(rates)
	if n == 0 {
		return nil, nil, errors.New("CheckEqualRates input must have at least one rate")
	}
	expectedPercent := make([]float64, n)
	equalPercent := float64(100.0) / float64(n)
	for i := range expectedPercent {
		expectedPercent[i] = equalPercent
	}
	return CheckRatesPercent(rates, expectedPercent, tolerance...)
}

// GetAllInterfaceCounters returns a ONCE telemetry sample of all interfaces' counters
// The returned data is stored in a map for easy lookup based on interface name.
//
// Model: openconfig-interfaces.
// YANG path: /interfaces/interface/state/counters
func GetAllInterfaceCounters(t *testing.T, dut *ondatra.DUTDevice) map[string]*ygnmi.Value[*oc.Interface_Counters] {
	got := gnmi.LookupAll(t, dut, gnmi.OC().InterfaceAny().Counters().State())
	data := make(map[string]*ygnmi.Value[*oc.Interface_Counters])
	for _, counters := range got {
		if intf, ok := counters.Path.Elem[1].Key["name"]; ok {
			data[intf] = counters
		}
	}
	return data
}

// GetAllInterfaceIpv4Counters returns a ONCE telemetry sample of all interfaces' IPv4 counters
// The returned data is stored in a map for easy lookup based on interface name.
//
// Model: openconfig-interfaces.
// YANG path: /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters
func GetAllInterfaceIpv4Counters(t *testing.T, dut *ondatra.DUTDevice) map[string]*ygnmi.Value[*oc.Interface_Subinterface_Ipv4_Counters] {
	got := gnmi.LookupAll(t, dut, gnmi.OC().InterfaceAny().Subinterface(0).Ipv4().Counters().State())
	data := make(map[string]*ygnmi.Value[*oc.Interface_Subinterface_Ipv4_Counters])
	for _, counters := range got {
		if intf, ok := counters.Path.Elem[1].Key["name"]; ok {
			data[intf] = counters
		}
	}
	return data
}

// InterfaceRates Struct to construct interface statistics
type InterfaceRates struct {
	SampleInterval     float64
	InPktsRate         float64
	InUnicastPktsRate  float64
	OutPktsRate        float64
	OutUnicastPktsRate float64
}

// GetDUTInterfaceRates outputs the packets rates in PPS for each interface in intfs.
// An optional interval to wait between samples can be provided.
//
// TODO: Use a STREAM collection once the sampling interval can be changed in Ondatra.
func GetDUTInterfaceRates(t *testing.T, dut *ondatra.DUTDevice, intfs []string, interval ...time.Duration) []*InterfaceRates {
	if len(intfs) == 0 {
		t.Errorf("GetDUTInterfaceRates input had no interfaces to get traffic rates on")
		return nil
	}
	sampleInterval := DefaultTelemtryInterval
	if len(interval) > 0 {
		sampleInterval = interval[0]
	}

	// TODO: Currently, the rate is calculated based on 2 separate telemetry ONCE output due
	// to inability to change sampling interval in Ondatra. When we are able to adjust interval,
	// this can be changed to a STREAM subscription.
	sample1 := GetAllInterfaceCounters(t, dut)
	time.Sleep(sampleInterval)
	sample2 := GetAllInterfaceCounters(t, dut)

	rates := make([]*InterfaceRates, len(intfs))
	for i, intf := range intfs {
		s1, ok := sample1[intf]
		if !ok {
			t.Errorf("no interface %v in 1st telemetry sample", intf)
			continue
		}
		s2, ok := sample2[intf]
		if !ok {
			t.Errorf("no interface %v in 2nd telemetry sample", intf)
			continue
		}
		sampleInterval := s2.Timestamp.Sub(s1.Timestamp).Seconds()
		counters1, _ := s1.Val()
		counters2, _ := s2.Val()
		rates[i] = &InterfaceRates{
			SampleInterval:     sampleInterval,
			InPktsRate:         float64(*counters2.InPkts-*counters1.InPkts) / sampleInterval,
			InUnicastPktsRate:  float64(*counters2.InUnicastPkts-*counters1.InUnicastPkts) / sampleInterval,
			OutPktsRate:        float64(*counters2.OutPkts-*counters1.OutPkts) / sampleInterval,
			OutUnicastPktsRate: float64(*counters2.OutUnicastPkts-*counters1.OutUnicastPkts) / sampleInterval,
		}
	}
	return rates
}

// InterfaceIpv4Rates Struct to construct interface IPv4 statistics
type InterfaceIpv4Rates struct {
	SampleInterval float64
	InPktsRate     float64
	OutPktsRate    float64
}

// GetDUTInterfaceIpv4Rates outputs the IPv4 packets rates in PPS for each interface in intfs.
func GetDUTInterfaceIpv4Rates(t *testing.T, dut *ondatra.DUTDevice, intfs []string, interval ...time.Duration) []*InterfaceIpv4Rates {
	if len(intfs) == 0 {
		t.Errorf("GetDUTInterfaceIpv4Rates input had no interfaces to get traffic rates on")
		return nil
	}
	sampleInterval := DefaultTelemtryInterval
	if len(interval) > 0 {
		sampleInterval = interval[0]
	}

	// TODO: Currently, the rate is calculated based on 2 separate telemetry ONCE output due
	// to inability to change sampling interval in Ondatra. When we are able to adjust interval,
	// this can be changed to a STREAM subscription.
	sample1 := GetAllInterfaceIpv4Counters(t, dut)
	time.Sleep(sampleInterval)
	sample2 := GetAllInterfaceIpv4Counters(t, dut)

	rates := make([]*InterfaceIpv4Rates, len(intfs))
	for i, intf := range intfs {
		s1, ok := sample1[intf]
		if !ok {
			t.Errorf("no interface %v in 1st telemetry sample", intf)
			continue
		}
		s2, ok := sample2[intf]
		if !ok {
			t.Errorf("no interface %v in 2nd telemetry sample", intf)
			continue
		}
		sampleInterval := s2.Timestamp.Sub(s1.Timestamp).Seconds()
		counters1, _ := s1.Val()
		counters2, _ := s2.Val()
		rates[i] = &InterfaceIpv4Rates{
			SampleInterval: sampleInterval,
			InPktsRate:     float64(*counters2.InPkts-*counters1.InPkts) / sampleInterval,
			OutPktsRate:    float64(*counters2.OutPkts-*counters1.OutPkts) / sampleInterval,
		}
	}
	return rates
}

// CheckDUTTrafficViaInterfaceTelemetry checks that:
// (1) total incoming packet rate from `in` interfaces is equal to total outgoing packet rate from `out` interfaces
// (2) the distribution of outgoing traffic matches the input `weights`
//
// Some optional arguments can be passed through variadic argument parsing where a `float64` will be used as
// tolerance, a `time.Duration` will be used as telemetry sampling interval, and a `bool` will determine whether
// to skip pass/failing tests when there are zero incoming packets based on above checks.
//
// Returns the success state of all verifiers and whether the traffic verification was skipped due to SIM NGDP issue
// where interface counters don't increase (can be disabled by passing in `false` as argument).
//
// TODO:
// (1) split up verifiers to be called separately.
// (2) consider returning boolean rather than directly Fail to allow user to have better control on behavior.
func CheckDUTTrafficViaInterfaceTelemetry(t *testing.T, dut *ondatra.DUTDevice, in []string, out []string, weights []float64, args ...interface{}) (bool, bool) {
	tolerance := DefaultVerifierTolerance
	interval := DefaultTelemtryInterval
	skipZeroTraffic := true // workaround zero packet counters issue on pyVXR
	for _, arg := range args {
		switch value := arg.(type) {
		case time.Duration:
			interval = value
		case float64:
			tolerance = value
		case bool:
			skipZeroTraffic = value
		default:
			t.Errorf("unknown argument of type %v", value)
		}
	}

	weightsDistribution := CalculateDistribution(weights)
	rates := GetDUTInterfaceRates(t, dut, append(in, out...), interval)
	var totalInPktsRate, totalInUnicastPktsRate, totalOutPktsRate, totalOutUnicastPktsRate float64

	numInIntfs := len(in)
	for i, intf := range in {
		t.Logf("interface %s InPkts  rate (PPS) = %8.3f", intf, rates[i].InPktsRate)
		// t.Logf("Interface %s InUnicastPkts  rate (PPS) = %8.3f", intf, rates[i].InUnicastPktsRate)
		totalInPktsRate += rates[i].InPktsRate
		totalInUnicastPktsRate += rates[i].InUnicastPktsRate
	}
	for i, intf := range out {
		t.Logf("interface %s OutPkts rate (PPS) = %8.3f", intf, rates[i+numInIntfs].OutPktsRate)
		// t.Logf("Interface %s OutUnicastPkts rate (PPS) = %8.3f", intf, rates[i+numInIntfs].OutUnicastPktsRate)
		totalOutPktsRate += rates[i+numInIntfs].OutPktsRate
		totalOutUnicastPktsRate += rates[i+numInIntfs].OutUnicastPktsRate
	}
	t.Logf("total InPkts  rate (PPS) = %8.3f", totalInPktsRate)
	t.Logf("total OutPkts rate (PPS) = %8.3f", totalOutPktsRate)
	if skipZeroTraffic && totalInPktsRate == 0.0 {
		t.Logf("skipping verification of 0 incoming traffic due to SIM issue")
		return true, true
	}
	success, percenterror, _ := CheckRates([]float64{totalInPktsRate}, []float64{totalOutPktsRate}, tolerance)
	result := success[0]
	logAndSetResult(t, success[0], "%6.3f%% error between InPkts/s and OutPkts/s. Tolerance was %.3f%%", percenterror[0], tolerance*100.0)

	// FIXME: Look into incoming unicast packet rate difference from total incoming packets
	//
	// t.Logf("Total InUnicastPkts  rate (PPS) = %5.5f", totalInUnicastPktsRate)
	// t.Logf("Total OutUnicastPkts rate (PPS) = %5.5f", totalOutUnicastPktsRate)
	// if success, _ := CheckRates([]float64{totalInUnicastPktsRate}, []float64{totalOutUnicastPktsRate}, tolerance); !success {
	// 	t.Errorf("FAIL: InUnicastPkts/s != OutUnicastPkts/s within tolerance %v", tolerance)
	// } else {
	// 	t.Logf("PASS: InUnicastPkts/s == OutUnicastPkts/s within tolerance %v", tolerance)
	// }

	outPktsRates := make([]float64, len(out))
	for i, r := range rates[numInIntfs:] {
		outPktsRates[i] = r.OutPktsRate
	}
	outPktsRates = CalculateDistribution(outPktsRates)
	success, percenterror, _ = CheckRates(outPktsRates, weightsDistribution, tolerance)
	allSuccess := true
	for i, intf := range out {
		allSuccess = allSuccess && success[i]
		logAndSetResult(t, success[i], "interface %s - Actual %6.3f%% - Expected %6.3f%% - Percent Error %6.3f%%", intf, outPktsRates[i]*100, weightsDistribution[i]*100, percenterror[i])
	}
	logAndSetResult(t, allSuccess, "expected weights %v within tolerance of %.3f%%", weights, tolerance*100.0)
	if !allSuccess {
		for _, weight := range weights[1:] {
			if math.Abs(weight-weights[0]) > tolerance*weights[0] {
				t.Log("NOTE: when testing UCMP, make sure number of flows is high enough to guarantee accurate results")
				break
			}
		}
	}

	return result && allSuccess, false
}

func logAndSetResult(t *testing.T, success bool, format string, args ...interface{}) {
	if success {
		t.Logf("PASS: "+format, args...)
	} else {
		t.Errorf("FAIL: "+format, args...)
	}
}
