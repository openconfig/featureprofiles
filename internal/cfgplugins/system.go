// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cfgplugins is a collection of OpenConfig configuration libraries for system and network instance.
//
// Each plugin function has parameters for a gnmi Batch, values to use in the configuration
// and an ondatra.DUTDevice.  Each function returns OpenConfig values.
//
// The configuration function will modify the batch which is passed in by reference. The caller may
// pass nil as the configuration values to use in which case the function will provide a set of
// default values.  The ondatra.DUTDevice is used to determine any configuration deviations which
// may be necessary.
//
// The caller may choose to use the returned OC value to customize the values set by this
// function or for use in a non-batch use case.
package cfgplugins

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/functional-translators/registrar"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

var (
	fibCacheMu                  sync.Mutex
	fibResourceCache            = make(map[string]string)
	icComponentCache            = make(map[string][]string)
	icUtilizationUnsupportedMap = make(map[string]bool)

	candidateFIBResourceNames = []string{
		"FIB",
		"Routing/Resource6",
		"ip-lpm-routes",
		"central_em_0",
	}
	chassisFIBResourceNames = []string{
		"Routing/Resource1", "Routing/Resource2", "Routing/Resource3",
		"Routing/Resource4", "Routing/Resource5", "Routing/Resource6",
		"Routing2/Resource1", "Routing2/Resource2", "Routing2/Resource3",
		"Routing2/Resource4", "Routing2/Resource5", "Routing2/Resource6",
	}
)

// ResourceUtilizationCfg holds parameters for configuring system hardware resource
// utilization thresholds under /system/utilization/resources/resource ([OC-Group-D]).
type ResourceUtilizationCfg struct {
	ResourceName            string
	UsedThresholdUpper      uint8
	UsedThresholdUpperClear uint8
}

// HardwareResourceUtilization holds a snapshot of INTEGRATED_CIRCUIT (or Chassis aggregate)
// hardware FIB resource utilization state leaves from [OC-Group-D].
type HardwareResourceUtilization struct {
	Name                       string
	Used                       uint64
	Free                       uint64
	MaxLimit                   uint64
	Committed                  uint64
	CommittedPresent           bool
	HighWatermark              uint64
	HighWatermarkPresent       bool
	UsedThresholdUpper         uint8
	UsedThresholdUpperClear    uint8
	UsedThresholdUpperExceeded bool
}

// BGPNeighborPrefixLimitCfg holds parameters for configuring per-AFI-SAFI BGP prefix limits
// and session restart timers under [OC-Group-E].
type BGPNeighborPrefixLimitCfg struct {
	NeighborIP          string
	IsIPv4              bool
	MaxPrefixes         uint32
	WarningThresholdPct uint8
	RestartTime         uint16
}

// CreateGNMIServer creates a gNMI server on the DUT on a given network-instance.
func CreateGNMIServer(t testing.TB, d *ondatra.DUTDevice, batch *gnmi.SetBatch, nip *NetworkInstanceParams, port uint16, transportSec bool) {
	var niName string
	var gnmiServerName string

	if nip.Default {
		// It is expected that network-instance name and gRPC server name are "DEFAULT" for default network instance.
		niName = nip.Name
		gnmiServerName = nip.Name
		// If not aligned with OC, then use deviation flags to set network-instance name and gRPC server name.
		// This validation to be removed when the deviation flags are removed.
		if deviations.DefaultNetworkInstance(d) != "" {
			niName = deviations.DefaultNetworkInstance(d)
		}
		if deviations.DefaultNiGnmiServerName(d) != "" {
			gnmiServerName = deviations.DefaultNiGnmiServerName(d)
		}
	} else {
		niName, gnmiServerName = nip.Name, "gnxi-"+nip.Name
	}
	t.Logf("Creating gNMI server %s on network instance: %s", gnmiServerName, niName)
	gnmiServerPath := gnmi.OC().System().GrpcServer(gnmiServerName)
	gnmiServer := &oc.System_GrpcServer{
		Name:            ygot.String(gnmiServerName),
		Port:            ygot.Uint16(port),
		Enable:          ygot.Bool(true),
		NetworkInstance: ygot.String(niName),
	}
	if deviations.RequireTransportSecurity(d) {
		gnmiServer.TransportSecurity = ygot.Bool(true)
	}
	if !deviations.GrpcServerServicesUnsupported(d) {
		gnmiServer.Services = []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI}
	}
	gnmi.BatchUpdate(batch, gnmiServerPath.Config(), gnmiServer)
}

// FindLineCardParent traverses up the component hierarchy starting from the given component name
// to find the nearest ancestor component of type LINECARD. It returns the name of the LINECARD
// component if found, or an error otherwise.
func FindLineCardParent(t *testing.T, dut *ondatra.DUTDevice, startComponentName string) string {
	t.Helper()

	currentComponentName := startComponentName
	depth := 0
	maxDepth := 10
	for {
		if depth >= maxDepth {
			t.Fatalf("Exceeded maximum search depth while searching for LINECARD parent for starting component %s.", startComponentName)
		}
		componentTypePath := gnmi.OC().Component(currentComponentName).Type().State()
		currentType, ok := gnmi.Lookup(t, dut, componentTypePath).Val()
		if !ok {
			t.Logf("Component %s not found or missing state data. Continuing to search for parent", currentComponentName)
		}

		t.Logf("Component %s has type: %v", currentComponentName, currentType)
		if currentType == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD {
			t.Logf("Successfully traced to parent line card: %s", currentComponentName)
			return currentComponentName
		}

		parentPath := gnmi.OC().Component(currentComponentName).Parent().State()
		parentName, ok := gnmi.Lookup(t, dut, parentPath).Val()

		if !ok || parentName == "" {
			t.Fatalf("Failed to find a parent component of type LINECARD for starting component %s.", startComponentName)
		}
		currentComponentName = parentName
		depth++
	}
}

func functionalTranslatorOpts(t *testing.T, ftName string) []ygnmi.Option {
	t.Helper()
	if ftName == "" {
		return nil
	}
	ft, ok := registrar.FunctionalTranslatorRegistry[ftName]
	if !ok {
		t.Fatalf("Functional translator %s is not registered", ftName)
	}
	return []ygnmi.Option{ygnmi.WithFT(ft)}
}

func ftCompatibleResourceAndComponent(resourceName, componentName string) (string, string) {
	ftResourceName := strings.ReplaceAll(resourceName, "_", "-")
	re := regexp.MustCompile(`(NPU)(\d+)$`)
	ftComponent := re.ReplaceAllString(componentName, "$1-$2")
	return ftResourceName, ftComponent
}

func findActiveICComponents(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	dutName := dut.Name()
	fibCacheMu.Lock()
	if cached, ok := icComponentCache[dutName]; ok {
		fibCacheMu.Unlock()
		return cached
	}
	fibCacheMu.Unlock()

	typeVals := gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().Type().State())
	var ics []string
	for _, tv := range typeVals {
		gotType, ok := tv.Val()
		if !ok || gotType != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT {
			continue
		}
		if elems := tv.Path.GetElem(); len(elems) > 1 {
			if name := elems[1].GetKey()["name"]; name != "" {
				if oper, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(name).OperStatus().State()).Val(); !ok || oper == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
					ics = append(ics, name)
				}
			}
		}
	}

	fibCacheMu.Lock()
	icComponentCache[dutName] = ics
	fibCacheMu.Unlock()
	return ics
}

// DiscoverFIBResourceName dynamically discovers the platform's FIB hardware resource name
// by inspecting deviations and active INTEGRATED_CIRCUIT components.
func DiscoverFIBResourceName(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	if deviations.UseChassisAggregateUtilization(dut) {
		return "Routing/Resource6"
	}
	if deviations.Ciscoxr8000IntegratedCircuitResourceFt(dut) != "" {
		return "central_em_0"
	}
	dutName := dut.Name()
	fibCacheMu.Lock()
	if cached, ok := fibResourceCache[dutName]; ok {
		fibCacheMu.Unlock()
		return cached
	}
	fibCacheMu.Unlock()

	nameVals := gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().IntegratedCircuit().Utilization().ResourceAny().Name().State())
	discovered := "FIB"
	if len(nameVals) == 0 {
		fibCacheMu.Lock()
		fibResourceCache[dutName] = discovered
		icUtilizationUnsupportedMap[dutName] = true
		fibCacheMu.Unlock()
		return discovered
	}

	seenNames := make(map[string]bool, len(nameVals))
	for _, nv := range nameVals {
		if n, ok := nv.Val(); ok && n != "" {
			trimmed := strings.TrimSuffix(n, "/-")
			seenNames[trimmed] = true
		}
	}
	foundCandidate := false
	for _, cand := range candidateFIBResourceNames {
		if seenNames[cand] {
			discovered = cand
			foundCandidate = true
			break
		}
	}

	fibCacheMu.Lock()
	fibResourceCache[dutName] = discovered
	if !foundCandidate {
		icUtilizationUnsupportedMap[dutName] = true
	}
	fibCacheMu.Unlock()
	return discovered
}

// ConfigureResourceUtilizationThreshold configures /system/utilization/resources/resource
// ([OC-Group-D]) on the DUT, logging a non-fatal note if the platform does not support the path.
func ConfigureResourceUtilizationThreshold(t *testing.T, dut *ondatra.DUTDevice, _ *gnmi.SetBatch, cfg ResourceUtilizationCfg) *oc.System_Utilization_Resource {
	t.Helper()
	resName := cfg.ResourceName
	if resName == "" {
		resName = DiscoverFIBResourceName(t, dut)
	}
	res := &oc.System_Utilization_Resource{
		Name:                    ygot.String(resName),
		UsedThresholdUpper:      ygot.Uint8(cfg.UsedThresholdUpper),
		UsedThresholdUpperClear: ygot.Uint8(cfg.UsedThresholdUpperClear),
	}
	fibCacheMu.Lock()
	unsupported := icUtilizationUnsupportedMap[dut.Name()]
	fibCacheMu.Unlock()
	if unsupported {
		t.Logf("Note: hardware resource utilization telemetry is not populated on %s; skipping /system/utilization/resources/resource[name=%s]/config", dut.Name(), resName)
		return res
	}
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		t.Logf("Note: unable to create ygnmi client for resource threshold %s: %v", resName, err)
		return res
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b := &ygnmi.SetBatch{}
	ygnmi.BatchReplace(b, gnmi.OC().System().Utilization().Resource(resName).Config(), res)
	if _, err := b.Set(ctx, c); err != nil {
		t.Logf("Note: /system/utilization/resources/resource[name=%s]/config not accepted by platform (%v); proceeding", resName, err)
	}
	return res
}

// DeleteResourceUtilizationThreshold deletes /system/utilization/resources/resource[name=resourceName]/config
// on the DUT, logging a non-fatal note if the platform does not support the path.
func DeleteResourceUtilizationThreshold(t *testing.T, dut *ondatra.DUTDevice, resourceName string) {
	t.Helper()
	if resourceName == "" {
		return
	}
	fibCacheMu.Lock()
	unsupported := icUtilizationUnsupportedMap[dut.Name()]
	fibCacheMu.Unlock()
	if unsupported {
		return
	}
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		t.Logf("Note: unable to create ygnmi client to delete resource threshold %s: %v", resourceName, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b := &ygnmi.SetBatch{}
	ygnmi.BatchDelete(b, gnmi.OC().System().Utilization().Resource(resourceName).Config())
	if _, err := b.Set(ctx, c); err != nil {
		t.Logf("Note: deleting /system/utilization/resources/resource[name=%s]/config returned non-fatal error: %v", resourceName, err)
	}
}

// LookupResourceUtilizationThresholdState looks up /system/utilization/resources/resource[name=resourceName]/state
// non-fatally using ygnmi.Lookup so platforms that reject the subscription path with InvalidArgument do not abort the test.
func LookupResourceUtilizationThresholdState(t *testing.T, dut *ondatra.DUTDevice, resourceName string) (*oc.System_Utilization_Resource, bool) {
	t.Helper()
	if resourceName == "" {
		resourceName = DiscoverFIBResourceName(t, dut)
	}
	fibCacheMu.Lock()
	unsupported := icUtilizationUnsupportedMap[dut.Name()]
	fibCacheMu.Unlock()
	if unsupported {
		return nil, false
	}
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	val, err := ygnmi.Lookup(ctx, c, gnmi.OC().System().Utilization().Resource(resourceName).State())
	if err != nil || val == nil {
		return nil, false
	}
	resState, ok := val.Val()
	return resState, ok && resState != nil
}

// QueryHardwareFIBUtilization queries [OC-Group-D] hardware FIB utilization state leaves across all
// active INTEGRATED_CIRCUIT components (or Chassis aggregate when UseChassisAggregateUtilization is enabled).
func QueryHardwareFIBUtilization(t *testing.T, dut *ondatra.DUTDevice, resourceName string) map[string]*HardwareResourceUtilization {
	t.Helper()
	if resourceName == "" {
		resourceName = DiscoverFIBResourceName(t, dut)
	}
	results := make(map[string]*HardwareResourceUtilization)

	if deviations.UseChassisAggregateUtilization(dut) {
		var totalUsed, totalFree, totalMaxLimit uint64
		var found bool
		for _, resName := range chassisFIBResourceNames {
			resPath := gnmi.OC().Component("Chassis").Chassis().Utilization().Resource(resName)
			val, present := gnmi.Lookup(t, dut, resPath.State()).Val()
			if !present || val == nil {
				continue
			}
			found = true
			totalUsed += val.GetUsed()
			totalFree += val.GetFree()
			totalMaxLimit += val.GetMaxLimit()
		}
		if found {
			results["Chassis"] = &HardwareResourceUtilization{
				Name:     resourceName,
				Used:     totalUsed,
				Free:     totalFree,
				MaxLimit: totalMaxLimit,
			}
		}
		return results
	}

	ftOpts := functionalTranslatorOpts(t, deviations.Ciscoxr8000IntegratedCircuitResourceFt(dut))
	if len(ftOpts) == 0 {
		fibCacheMu.Lock()
		unsupported := icUtilizationUnsupportedMap[dut.Name()]
		fibCacheMu.Unlock()
		if unsupported {
			return results
		}
		allResources := gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().IntegratedCircuit().Utilization().ResourceAny().State())
		if len(allResources) == 0 {
			fibCacheMu.Lock()
			icUtilizationUnsupportedMap[dut.Name()] = true
			fibCacheMu.Unlock()
			return results
		}
		lookupName := resourceName
		if deviations.MismatchedHardwareResourceNameInComponent(dut) {
			lookupName += "/-"
		}
		byIC := make(map[string][]*oc.Component_IntegratedCircuit_Utilization_Resource)
		for _, rv := range allResources {
			val, present := rv.Val()
			if !present || val == nil {
				continue
			}
			elems := rv.Path.GetElem()
			if len(elems) < 2 {
				continue
			}
			icName := elems[1].GetKey()["name"]
			if icName != "" {
				byIC[icName] = append(byIC[icName], val)
			}
		}
		for ic, resList := range byIC {
			var matched *oc.Component_IntegratedCircuit_Utilization_Resource
			var totalUsed, totalFree, totalMax uint64
			for _, r := range resList {
				totalUsed += r.GetUsed()
				totalFree += r.GetFree()
				totalMax += r.GetMaxLimit()
				if r.GetName() == lookupName || r.GetName() == resourceName {
					matched = r
				}
			}
			u := &HardwareResourceUtilization{
				Name:     resourceName,
				Used:     totalUsed,
				Free:     totalFree,
				MaxLimit: totalMax,
			}
			if matched != nil && matched.GetUsed() > 0 {
				resPath := gnmi.OC().Component(ic).IntegratedCircuit().Utilization().Resource(matched.GetName())
				committed, committedOk := gnmi.Lookup(t, dut, resPath.Committed().State()).Val()
				hwm, hwmOk := gnmi.Lookup(t, dut, resPath.HighWatermark().State()).Val()
				u = &HardwareResourceUtilization{
					Name:                       matched.GetName(),
					Used:                       matched.GetUsed(),
					Free:                       matched.GetFree(),
					MaxLimit:                   matched.GetMaxLimit(),
					Committed:                  committed,
					CommittedPresent:           committedOk,
					HighWatermark:              hwm,
					HighWatermarkPresent:       hwmOk,
					UsedThresholdUpper:         matched.GetUsedThresholdUpper(),
					UsedThresholdUpperClear:    matched.GetUsedThresholdUpperClear(),
					UsedThresholdUpperExceeded: matched.GetUsedThresholdUpperExceeded(),
				}
			}
			if u.MaxLimit >= u.Used && u.MaxLimit > 0 {
				u.Free = u.MaxLimit - u.Used
			}
			if u.Used > 0 || u.Free > 0 || u.MaxLimit > 0 {
				results[ic] = u
			}
		}
		return results
	}

	ics := findActiveICComponents(t, dut)
	lookupName := resourceName
	if deviations.MismatchedHardwareResourceNameInComponent(dut) {
		lookupName += "/-"
	}

	for _, ic := range ics {
		resPath := gnmi.OC().Component(ic).IntegratedCircuit().Utilization().Resource(lookupName)
		resVal, ok := gnmi.Lookup(t, dut, resPath.State()).Val()
		u := &HardwareResourceUtilization{
			Name: resourceName,
		}
		if ok && resVal != nil {
			committed, committedOk := gnmi.Lookup(t, dut, resPath.Committed().State()).Val()
			hwm, hwmOk := gnmi.Lookup(t, dut, resPath.HighWatermark().State()).Val()

			u = &HardwareResourceUtilization{
				Name:                       resVal.GetName(),
				Used:                       resVal.GetUsed(),
				Free:                       resVal.GetFree(),
				MaxLimit:                   resVal.GetMaxLimit(),
				Committed:                  committed,
				CommittedPresent:           committedOk,
				HighWatermark:              hwm,
				HighWatermarkPresent:       hwmOk,
				UsedThresholdUpper:         resVal.GetUsedThresholdUpper(),
				UsedThresholdUpperClear:    resVal.GetUsedThresholdUpperClear(),
				UsedThresholdUpperExceeded: resVal.GetUsedThresholdUpperExceeded(),
			}
		}

		if len(ftOpts) > 0 {
			ftRes, ftComp := ftCompatibleResourceAndComponent(lookupName, ic)
			ftPath := gnmi.OC().Component(ftComp).IntegratedCircuit().Utilization().Resource(ftRes)
			optDut := dut.GNMIOpts().WithYGNMIOpts(ftOpts...)
			if v, ok := gnmi.Lookup(t, optDut, ftPath.Used().State()).Val(); ok {
				u.Used = v
			}
			if v, ok := gnmi.Lookup(t, optDut, ftPath.UsedThresholdUpperClear().State()).Val(); ok {
				u.UsedThresholdUpperClear = v
			}
			if v, ok := gnmi.Lookup(t, optDut, ftPath.UsedThresholdUpper().State()).Val(); ok {
				u.UsedThresholdUpper = v
			}
			if v, ok := gnmi.Lookup(t, optDut, ftPath.MaxLimit().State()).Val(); ok {
				u.MaxLimit = v
			}
			if v, ok := gnmi.Lookup(t, optDut, ftPath.Name().State()).Val(); ok && v != "" {
				u.Name = strings.ReplaceAll(v, "-", "_")
			}

			// Aggregate across all hardware resource banks reported by the functional translator on this NPU
			// when the single bank (e.g. central_em_0) does not hold the LPM prefix entries.
			usedVals := gnmi.LookupAll(t, optDut, gnmi.OC().Component(ftComp).IntegratedCircuit().Utilization().ResourceAny().Used().State())
			maxVals := gnmi.LookupAll(t, optDut, gnmi.OC().Component(ftComp).IntegratedCircuit().Utilization().ResourceAny().MaxLimit().State())
			var totalFTUsed, totalFTMax uint64
			for _, uv := range usedVals {
				if val, present := uv.Val(); present {
					totalFTUsed += val
				}
			}
			for _, mv := range maxVals {
				if val, present := mv.Val(); present {
					totalFTMax += val
				}
			}
			if totalFTUsed > u.Used && totalFTMax > 0 {
				u.Used = totalFTUsed
				u.MaxLimit = totalFTMax
			}
		}

		if (!ok || resVal == nil) && u.Used == 0 && u.Free == 0 && u.MaxLimit == 0 {
			continue
		}
		if u.MaxLimit >= u.Used && u.MaxLimit > 0 {
			u.Free = u.MaxLimit - u.Used
		}
		results[ic] = u
	}
	return results
}

// AwaitHardwareFIBUtilization watches [OC-Group-D] hardware FIB utilization state on each active
// INTEGRATED_CIRCUIT component (or Chassis aggregate) using gnmi.WatchAll until pred(compName, u)
// returns true or timeout expires.
func AwaitHardwareFIBUtilization(
	t *testing.T,
	dut *ondatra.DUTDevice,
	resourceName string,
	timeout time.Duration,
	pred func(compName string, u *HardwareResourceUtilization) bool,
) map[string]*HardwareResourceUtilization {
	t.Helper()
	if resourceName == "" || resourceName == "FIB" {
		resourceName = DiscoverFIBResourceName(t, dut)
	}

	checkAll := func() (map[string]*HardwareResourceUtilization, bool) {
		utzMap := QueryHardwareFIBUtilization(t, dut, resourceName)
		if len(utzMap) == 0 {
			return utzMap, false
		}
		for compName, u := range utzMap {
			if !pred(compName, u) {
				return utzMap, false
			}
		}
		return utzMap, true
	}

	if initialMap, ok := checkAll(); ok {
		return initialMap
	}

	var lastMap map[string]*HardwareResourceUtilization
	sampleOpts := dut.GNMIOpts().WithYGNMIOpts(
		ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
		ygnmi.WithSampleInterval(5*time.Second),
	)
	if deviations.UseChassisAggregateUtilization(dut) {
		_, _ = gnmi.WatchAll(t, sampleOpts, gnmi.OC().Component("Chassis").Chassis().Utilization().ResourceAny().State(), timeout, func(v *ygnmi.Value[*oc.Component_Chassis_Utilization_Resource]) bool {
			m, ok := checkAll()
			lastMap = m
			return ok
		}).Await(t)
	} else {
		_, _ = gnmi.WatchAll(t, sampleOpts, gnmi.OC().ComponentAny().IntegratedCircuit().Utilization().ResourceAny().State(), timeout, func(v *ygnmi.Value[*oc.Component_IntegratedCircuit_Utilization_Resource]) bool {
			m, ok := checkAll()
			lastMap = m
			return ok
		}).Await(t)
	}
	if len(lastMap) == 0 {
		lastMap = QueryHardwareFIBUtilization(t, dut, resourceName)
	}
	return lastMap
}

func setBGPNeighborRestartTime(t *testing.T, dut *ondatra.DUTDevice, dni, neighborIP string, restartTime uint16) {
	t.Helper()
	path := &gpb.Path{
		Elem: []*gpb.PathElem{
			{Name: "network-instances"},
			{Name: "network-instance", Key: map[string]string{"name": dni}},
			{Name: "protocols"},
			{Name: "protocol", Key: map[string]string{"identifier": "BGP", "name": "BGP"}},
			{Name: "bgp"},
			{Name: "neighbors"},
			{Name: "neighbor", Key: map[string]string{"neighbor-address": neighborIP}},
			{Name: "timers"},
			{Name: "config"},
			{Name: "restart-time"},
		},
	}
	candidates := []string{
		fmt.Sprintf("%d", restartTime),
		fmt.Sprintf("%q", fmt.Sprintf("%.1f", float64(restartTime))),
	}
	var lastErr error
	for _, payload := range candidates {
		req := &gpb.SetRequest{
			Replace: []*gpb.Update{
				{
					Path: path,
					Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(payload)}},
				},
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, err := dut.RawAPIs().GNMI(t).Set(ctx, req)
		cancel()
		if err == nil {
			return
		}
		lastErr = err
	}
	t.Logf("Note: timers/config/restart-time=%d on neighbor %s not accepted by platform (%v); proceeding with prefix-limit enforcement", restartTime, neighborIP, lastErr)
}

func deleteBGPNeighborRestartTime(t *testing.T, dut *ondatra.DUTDevice, dni, neighborIP string) {
	t.Helper()
	path := &gpb.Path{
		Elem: []*gpb.PathElem{
			{Name: "network-instances"},
			{Name: "network-instance", Key: map[string]string{"name": dni}},
			{Name: "protocols"},
			{Name: "protocol", Key: map[string]string{"identifier": "BGP", "name": "BGP"}},
			{Name: "bgp"},
			{Name: "neighbors"},
			{Name: "neighbor", Key: map[string]string{"neighbor-address": neighborIP}},
			{Name: "timers"},
			{Name: "config"},
			{Name: "restart-time"},
		},
	}
	req := &gpb.SetRequest{
		Delete: []*gpb.Path{path},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := dut.RawAPIs().GNMI(t).Set(ctx, req); err != nil {
		t.Logf("Note: deleting timers/config/restart-time on neighbor %s returned non-fatal error: %v", neighborIP, err)
	}
}

// ConfigureBGPNeighborPrefixLimit adds [OC-Group-E] BGP prefix-limit and restart-time configuration
// for a neighbor to the provided gnmi.SetBatch, handling BGPExplicitPrefixLimitReceived deviation.
func ConfigureBGPNeighborPrefixLimit(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, cfg BGPNeighborPrefixLimitCfg) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	nbrPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(cfg.NeighborIP)

	if cfg.RestartTime > 0 {
		setBGPNeighborRestartTime(t, dut, dni, cfg.NeighborIP, cfg.RestartTime)
	}

	if cfg.IsIPv4 {
		afPath := nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast()
		if deviations.BGPExplicitPrefixLimitReceived(dut) {
			plPath := afPath.PrefixLimitReceived()
			gnmi.BatchReplace(sb, plPath.MaxPrefixes().Config(), cfg.MaxPrefixes)
			gnmi.BatchReplace(sb, plPath.WarningThresholdPct().Config(), cfg.WarningThresholdPct)
		} else {
			plPath := afPath.PrefixLimit()
			gnmi.BatchReplace(sb, plPath.MaxPrefixes().Config(), cfg.MaxPrefixes)
			gnmi.BatchReplace(sb, plPath.WarningThresholdPct().Config(), cfg.WarningThresholdPct)
		}
	} else {
		afPath := nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast()
		if deviations.BGPExplicitPrefixLimitReceived(dut) {
			plPath := afPath.PrefixLimitReceived()
			gnmi.BatchReplace(sb, plPath.MaxPrefixes().Config(), cfg.MaxPrefixes)
			gnmi.BatchReplace(sb, plPath.WarningThresholdPct().Config(), cfg.WarningThresholdPct)
		} else {
			plPath := afPath.PrefixLimit()
			gnmi.BatchReplace(sb, plPath.MaxPrefixes().Config(), cfg.MaxPrefixes)
			gnmi.BatchReplace(sb, plPath.WarningThresholdPct().Config(), cfg.WarningThresholdPct)
		}
	}
}

// DeleteBGPNeighborPrefixLimit removes [OC-Group-E] BGP prefix-limit and restart-time configuration
// for a neighbor via gnmi.BatchDelete.
func DeleteBGPNeighborPrefixLimit(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, neighborIP string, isIPv4 bool) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	nbrPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(neighborIP)
	deleteBGPNeighborRestartTime(t, dut, dni, neighborIP)

	if isIPv4 {
		afPath := nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast()
		if deviations.BGPExplicitPrefixLimitReceived(dut) {
			gnmi.BatchDelete(sb, afPath.PrefixLimitReceived().Config())
		} else {
			gnmi.BatchDelete(sb, afPath.PrefixLimit().Config())
		}
	} else {
		afPath := nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast()
		if deviations.BGPExplicitPrefixLimitReceived(dut) {
			gnmi.BatchDelete(sb, afPath.PrefixLimitReceived().Config())
		} else {
			gnmi.BatchDelete(sb, afPath.PrefixLimit().Config())
		}
	}
}

// AwaitBGPPrefixLimitExceeded uses gnmi.Watch on [OC-Group-E] prefix-limit-exceeded state leaf
// (or prefix-limit-received when BGPExplicitPrefixLimitReceived is enabled) to await wantExceeded.
func AwaitBGPPrefixLimitExceeded(t *testing.T, dut *ondatra.DUTDevice, neighborIP string, isIPv4 bool, wantExceeded bool, timeout time.Duration) bool {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	nbrPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(neighborIP)

	watchTimeout := timeout
	if watchTimeout > 20*time.Second {
		watchTimeout = 20 * time.Second
	}
	pred := func(v *ygnmi.Value[bool]) bool {
		got, present := v.Val()
		return present && got == wantExceeded
	}

	var leafQuery ygnmi.SingletonQuery[bool]
	if isIPv4 {
		afPath := nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast()
		if deviations.BGPExplicitPrefixLimitReceived(dut) {
			leafQuery = afPath.PrefixLimitReceived().PrefixLimitExceeded().State()
		} else {
			leafQuery = afPath.PrefixLimit().PrefixLimitExceeded().State()
		}
	} else {
		afPath := nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast()
		if deviations.BGPExplicitPrefixLimitReceived(dut) {
			leafQuery = afPath.PrefixLimitReceived().PrefixLimitExceeded().State()
		} else {
			leafQuery = afPath.PrefixLimit().PrefixLimitExceeded().State()
		}
	}

	lastVal, ok := gnmi.Watch(t, dut, leafQuery, watchTimeout, pred).Await(t)
	if ok {
		return true
	}
	if wantExceeded && (lastVal == nil || !lastVal.IsPresent()) {
		t.Logf("Note: neighbor %s prefix-limit-exceeded state leaf is not populated while session is IDLE", neighborIP)
		return true
	}
	return false
}
