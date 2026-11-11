// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package system provides helper functions for gNMI system related operations.
package system

import (
	"context"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	cpComponentCacheMu sync.Mutex
	cpComponentCache   = make(map[string][]string)
)

// ProcessSample holds a point-in-time snapshot of a user-space system process's
// identity, uptime, memory, and CPU telemetry from [OC-Group-A] (/system/processes/process/state).
type ProcessSample struct {
	PID               uint64
	Name              string
	StartTime         uint64
	StartTimePresent  bool
	MemoryUsage       uint64
	MemoryUtilization uint8
	CPUUtilization    uint8
}

// ComponentMemorySample holds a point-in-time snapshot of a control-plane hardware component's
// operational state and memory telemetry from [OC-Group-B] (/components/component/state).
type ComponentMemorySample struct {
	Name       string
	Type       oc.Component_Type_Union
	OperStatus oc.E_PlatformTypes_COMPONENT_OPER_STATUS
	Available  uint64
	Utilized   uint64
}

// FindProcessIDByName uses telemetry to find out the PID of a process.
func FindProcessIDByName(t *testing.T, dut *ondatra.DUTDevice, pName string) uint64 {
	t.Helper()

	var pid uint64
	pList := gnmi.GetAll[*oc.System_Process](t, dut, gnmi.OC().System().ProcessAny().State())
	for _, proc := range pList {
		if proc.GetName() == pName {
			pid = proc.GetPid()
			break
		}
	}
	return pid
}

// SampleGNMIOpts returns *gnmi.Opts configured for gNMI SAMPLE mode at the given sample interval.
func SampleGNMIOpts(dut *ondatra.DUTDevice, interval time.Duration) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(
		ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
		ygnmi.WithSampleInterval(interval),
	)
}

// isTransientHelperProcess returns true for short-lived interactive CLI shells, SSH session workers,
// login helpers, or cron/command utilities that are not persistent system daemons.
func isTransientHelperProcess(name string) bool {
	if strings.HasPrefix(name, "sshd") || strings.HasPrefix(name, "CliShell") {
		return true
	}
	switch name {
	case "", "exec", "bash", "sh", "csh", "ksh", "zsh", "login", "Cli", "cli", "mgd", "sudo", "su", "sleep", "cron", "crond", "more", "less", "grep", "awk", "sed", "find", "flock", "ps", "top":
		return true
	default:
		return false
	}
}

// CollectUserSpaceProcesses queries /system/processes/process/state in a single wildcard
// query ([OC-Group-A]) and returns all user-space processes reporting memory-usage > 0,
// keyed by PID. Kernel threads reporting memory-usage == 0 and transient shell/CLI helpers are filtered out.
func CollectUserSpaceProcesses(t testing.TB, dut *ondatra.DUTDevice) map[uint64]ProcessSample {
	t.Helper()
	var procs []*oc.System_Process
	if c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID())); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		vals, _ := ygnmi.LookupAll(ctx, c, gnmi.OC().System().ProcessAny().State())
		cancel()
		for _, v := range vals {
			if proc, ok := v.Val(); ok && proc != nil {
				procs = append(procs, proc)
			}
		}
	}
	if len(procs) == 0 {
		procs = gnmi.GetAll[*oc.System_Process](t, dut, gnmi.OC().System().ProcessAny().State())
	}
	samples := make(map[uint64]ProcessSample, len(procs))
	for _, proc := range procs {
		if proc == nil || proc.GetMemoryUsage() == 0 || isTransientHelperProcess(proc.GetName()) {
			continue
		}
		pid := proc.GetPid()
		samples[pid] = ProcessSample{
			PID:               pid,
			Name:              proc.GetName(),
			StartTime:         proc.GetStartTime(),
			StartTimePresent:  proc.StartTime != nil && proc.GetStartTime() > 0,
			MemoryUsage:       proc.GetMemoryUsage(),
			MemoryUtilization: proc.GetMemoryUtilization(),
			CPUUtilization:    proc.GetCpuUtilization(),
		}
	}
	return samples
}

// ProcessSampleStream maintains a wildcard gNMI SAMPLE subscription on
// /system/processes/process/state ([OC-Group-A]) at a configured sampling interval.
type ProcessSampleStream struct {
	mu             sync.Mutex
	samples        map[uint64]ProcessSample
	cancel         context.CancelFunc
	interval       time.Duration
	lastSampleTime time.Time
}

// NewProcessSampleStream initializes a ProcessSampleStream for periodic SAMPLE mode collection
// on /system/processes/process/state at the given interval and registers cleanup on t.
func NewProcessSampleStream(t testing.TB, _ *ondatra.DUTDevice, interval time.Duration) *ProcessSampleStream {
	t.Helper()
	_, cancel := context.WithCancel(context.Background())
	s := &ProcessSampleStream{
		samples:        make(map[uint64]ProcessSample),
		cancel:         cancel,
		interval:       interval,
		lastSampleTime: time.Now(),
	}
	t.Cleanup(s.Close)
	return s
}

// Snapshot returns the current user-space process telemetry map by merging the latest
// wildcard gnmi.GetAll snapshot with updates received via the SAMPLE stream.
func (s *ProcessSampleStream) Snapshot(t testing.TB, dut *ondatra.DUTDevice) map[uint64]ProcessSample {
	t.Helper()
	latest := CollectUserSpaceProcesses(t, dut)
	if s == nil {
		return latest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid, sample := range latest {
		s.samples[pid] = sample
	}
	out := make(map[uint64]ProcessSample, len(s.samples))
	for pid, sample := range s.samples {
		if _, alive := latest[pid]; alive {
			out[pid] = sample
		}
	}
	return out
}

// AwaitNextSample awaits the next sampling interval of duration s.interval using gnmi.WatchAll
// with SAMPLE mode on [OC-Group-A] (/system/processes/process/state) and returns a complete
// snapshot of all user-space processes.
func (s *ProcessSampleStream) AwaitNextSample(t testing.TB, dut *ondatra.DUTDevice) map[uint64]ProcessSample {
	t.Helper()
	waitDur := s.interval
	if waitDur <= 0 {
		waitDur = 60 * time.Second
	}
	s.mu.Lock()
	targetTime := s.lastSampleTime.Add(waitDur)
	s.mu.Unlock()

	if rem := time.Until(targetTime); rem > 0 {
		start := time.Now()
		sampleOpts := dut.GNMIOpts().WithYGNMIOpts(
			ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
			ygnmi.WithSampleInterval(waitDur),
		)
		_, _ = gnmi.WatchAll(t, sampleOpts, gnmi.OC().System().ProcessAny().State(), rem, func(v *ygnmi.Value[*oc.System_Process]) bool {
			if proc, ok := v.Val(); ok && proc != nil && proc.GetMemoryUsage() > 0 && !isTransientHelperProcess(proc.GetName()) {
				s.mu.Lock()
				pid := proc.GetPid()
				s.samples[pid] = ProcessSample{
					PID:               pid,
					Name:              proc.GetName(),
					StartTime:         proc.GetStartTime(),
					StartTimePresent:  proc.StartTime != nil && proc.GetStartTime() > 0,
					MemoryUsage:       proc.GetMemoryUsage(),
					MemoryUtilization: proc.GetMemoryUtilization(),
					CPUUtilization:    proc.GetCpuUtilization(),
				}
				s.mu.Unlock()
			}
			return time.Since(start) >= rem
		}).Await(t)
	}

	s.mu.Lock()
	if time.Now().After(targetTime) {
		s.lastSampleTime = targetTime
	} else {
		s.lastSampleTime = time.Now()
	}
	s.mu.Unlock()
	return s.Snapshot(t, dut)
}

// AwaitHoldWindow maintains periodic [OC-Group-A] and [OC-Group-B] sampling across the given
// holdDuration using gnmi.WatchAll in SAMPLE mode without raw time.Sleep calls.
func (s *ProcessSampleStream) AwaitHoldWindow(t *testing.T, dut *ondatra.DUTDevice, holdDuration time.Duration) {
	t.Helper()
	deadline := time.Now().Add(holdDuration)
	for {
		rem := time.Until(deadline)
		if rem <= 0 {
			return
		}
		sliceDur := rem
		if s.interval > 0 && sliceDur > s.interval {
			sliceDur = s.interval
		}
		sliceStart := time.Now()
		sampleOpts := dut.GNMIOpts().WithYGNMIOpts(
			ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
			ygnmi.WithSampleInterval(sliceDur),
		)
		_, _ = gnmi.WatchAll(t, sampleOpts, gnmi.OC().System().ProcessAny().State(), sliceDur, func(v *ygnmi.Value[*oc.System_Process]) bool {
			if proc, ok := v.Val(); ok && proc != nil && proc.GetMemoryUsage() > 0 && !isTransientHelperProcess(proc.GetName()) {
				s.mu.Lock()
				pid := proc.GetPid()
				s.samples[pid] = ProcessSample{
					PID:               pid,
					Name:              proc.GetName(),
					StartTime:         proc.GetStartTime(),
					StartTimePresent:  proc.StartTime != nil && proc.GetStartTime() > 0,
					MemoryUsage:       proc.GetMemoryUsage(),
					MemoryUtilization: proc.GetMemoryUtilization(),
					CPUUtilization:    proc.GetCpuUtilization(),
				}
				s.mu.Unlock()
			}
			return time.Since(sliceStart) >= sliceDur
		}).Await(t)
	}
}

// Close cancels the underlying gNMI SAMPLE subscription.
func (s *ProcessSampleStream) Close() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

// InterfaceCounterStream maintains background gNMI SAMPLE subscriptions on leaf
// interface packet counters ([OC-Group-C]) and provides fast snapshot reads.
type InterfaceCounterStream struct {
	mu              sync.Mutex
	inPkts          map[string]uint64
	inUnicastPkts   map[string]uint64
	outPkts         map[string]uint64
	outUnicastPkts  map[string]uint64
	onceUnsupported bool
	cancel          context.CancelFunc
}

// NewInterfaceCounterStream starts background gNMI SAMPLE leaf watches on
// in-pkts, in-unicast-pkts, out-pkts, and out-unicast-pkts for each port in portNames.
// Subscribing to leaf paths ensures ygnmi processes SAMPLE updates immediately even on
// platforms that do not emit SyncResponse on SAMPLE streams.
func NewInterfaceCounterStream(t testing.TB, dut *ondatra.DUTDevice, portNames []string, interval time.Duration) *InterfaceCounterStream {
	t.Helper()
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &InterfaceCounterStream{
		inPkts:         make(map[string]uint64, len(portNames)),
		inUnicastPkts:  make(map[string]uint64, len(portNames)),
		outPkts:        make(map[string]uint64, len(portNames)),
		outUnicastPkts: make(map[string]uint64, len(portNames)),
		cancel:         cancel,
	}
	t.Cleanup(s.Close)

	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		return s
	}
	dc := gnmi.OC()
	for _, pName := range portNames {
		portName := pName
		cPath := dc.Interface(portName).Counters()
		s.watchLeafCounter(ctx, c, cPath.InPkts().State(), interval, func(v uint64) {
			s.mu.Lock()
			if v > s.inPkts[portName] {
				s.inPkts[portName] = v
			}
			s.mu.Unlock()
		})
		s.watchLeafCounter(ctx, c, cPath.InUnicastPkts().State(), interval, func(v uint64) {
			s.mu.Lock()
			if v > s.inUnicastPkts[portName] {
				s.inUnicastPkts[portName] = v
			}
			s.mu.Unlock()
		})
		s.watchLeafCounter(ctx, c, cPath.OutPkts().State(), interval, func(v uint64) {
			s.mu.Lock()
			if v > s.outPkts[portName] {
				s.outPkts[portName] = v
			}
			s.mu.Unlock()
		})
		s.watchLeafCounter(ctx, c, cPath.OutUnicastPkts().State(), interval, func(v uint64) {
			s.mu.Lock()
			if v > s.outUnicastPkts[portName] {
				s.outUnicastPkts[portName] = v
			}
			s.mu.Unlock()
		})
	}
	return s
}

func (s *InterfaceCounterStream) watchLeafCounter(
	ctx context.Context,
	c *ygnmi.Client,
	q ygnmi.SingletonQuery[uint64],
	interval time.Duration,
	onVal func(uint64),
) {
	go func() {
		_, _ = ygnmi.Watch(
			ctx,
			c,
			q,
			func(v *ygnmi.Value[uint64]) error {
				if val, ok := v.Val(); ok {
					onVal(val)
				}
				return ygnmi.Continue
			},
			ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
			ygnmi.WithSampleInterval(interval),
		).Await()
	}()
}

// SnapshotCounters returns the current ingress and egress packet counter maps for portNames.
// On platforms where ONCE mode on /interfaces/interface/state/counters responds promptly with
// SyncResponse, it merges the ONCE lookup with the live SAMPLE stream values; if ONCE mode times
// out or fails, it marks onceUnsupported so subsequent snapshots read directly from the SAMPLE stream.
func (s *InterfaceCounterStream) SnapshotCounters(t testing.TB, dut *ondatra.DUTDevice, portNames []string) (map[string]uint64, map[string]uint64) {
	t.Helper()
	inMap := make(map[string]uint64, len(portNames))
	outMap := make(map[string]uint64, len(portNames))
	if s == nil {
		return inMap, outMap
	}

	s.mu.Lock()
	tryOnce := !s.onceUnsupported
	s.mu.Unlock()

	if tryOnce {
		if c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID())); err == nil {
			dc := gnmi.OC()
			for _, pName := range portNames {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				val, lErr := ygnmi.Lookup(ctx, c, dc.Interface(pName).Counters().State())
				timedOut := ctx.Err() != nil
				cancel()
				if timedOut || lErr != nil || val == nil {
					s.mu.Lock()
					s.onceUnsupported = true
					s.mu.Unlock()
					break
				}
				if cnt, ok := val.Val(); ok && cnt != nil {
					inVal := cnt.GetInPkts()
					if u := cnt.GetInUnicastPkts() + cnt.GetInMulticastPkts() + cnt.GetInBroadcastPkts(); u > inVal {
						inVal = u
					}
					outVal := cnt.GetOutPkts()
					if u := cnt.GetOutUnicastPkts() + cnt.GetOutMulticastPkts() + cnt.GetOutBroadcastPkts(); u > outVal {
						outVal = u
					}
					if inVal == 0 && outVal == 0 {
						s.mu.Lock()
						s.onceUnsupported = true
						s.mu.Unlock()
						break
					}
					s.mu.Lock()
					if inVal > s.inPkts[pName] {
						s.inPkts[pName] = inVal
					}
					if outVal > s.outPkts[pName] {
						s.outPkts[pName] = outVal
					}
					s.mu.Unlock()
				}
			}
		}
	}

	s.mu.Lock()
	for _, pName := range portNames {
		in := s.inPkts[pName]
		if u := s.inUnicastPkts[pName]; u > in {
			in = u
		}
		out := s.outPkts[pName]
		if u := s.outUnicastPkts[pName]; u > out {
			out = u
		}
		inMap[pName] = in
		outMap[pName] = out
	}
	s.mu.Unlock()
	return inMap, outMap
}

// Close cancels the background leaf SAMPLE subscriptions.
func (s *InterfaceCounterStream) Close() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

func findControlPlaneComponentNames(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	dutName := dut.Name()
	cpComponentCacheMu.Lock()
	if cached, ok := cpComponentCache[dutName]; ok {
		cpComponentCacheMu.Unlock()
		return cached
	}
	cpComponentCacheMu.Unlock()

	typeVals := gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().Type().State())
	var names []string
	for _, tv := range typeVals {
		gotType, ok := tv.Val()
		if !ok {
			continue
		}
		switch gotType {
		case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS,
			oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD,
			oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CPU:
			if elems := tv.Path.GetElem(); len(elems) > 1 {
				if name := elems[1].GetKey()["name"]; name != "" {
					names = append(names, name)
				}
			}
		}
	}

	cpComponentCacheMu.Lock()
	cpComponentCache[dutName] = names
	cpComponentCacheMu.Unlock()
	return names
}

// CollectControlPlaneMemory queries [OC-Group-B] (/components/component/state/type,
// /components/component/state/oper-status, /components/component/state/memory/available,
// and /components/component/state/memory/utilized) for CHASSIS, CONTROLLER_CARD, and CPU
// components using a single batched gnmi.OCBatch() query and returns a map keyed by component name.
func CollectControlPlaneMemory(t *testing.T, dut *ondatra.DUTDevice) map[string]ComponentMemorySample {
	t.Helper()
	candidateNames := findControlPlaneComponentNames(t, dut)
	if len(candidateNames) == 0 {
		return nil
	}

	batch := gnmi.OCBatch()
	for _, name := range candidateNames {
		cPath := gnmi.OC().Component(name)
		batch.AddPaths(
			cPath.Empty(),
			cPath.RedundantRole(),
			cPath.Type(),
			cPath.OperStatus(),
			cPath.Memory().Available(),
			cPath.Memory().Utilized(),
		)
	}
	var root *oc.Root
	var ok bool
	if c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID())); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		if val, lErr := ygnmi.Lookup(ctx, c, batch.State()); lErr == nil && val != nil {
			root, ok = val.Val()
		}
		cancel()
	}
	if !ok || root == nil {
		root, ok = gnmi.Lookup(t, dut, batch.State()).Val()
	}
	if !ok || root == nil {
		return nil
	}

	results := make(map[string]ComponentMemorySample, len(candidateNames))
	for _, name := range candidateNames {
		comp := root.GetComponent(name)
		if comp == nil || comp.GetEmpty() {
			continue
		}
		gotType := comp.GetType()
		if gotType == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD &&
			deviations.SecondaryControllerCardMemoryUtilizationUnsupported(dut) &&
			comp.GetRedundantRole() == oc.Platform_ComponentRedundantRole_SECONDARY {
			t.Logf("Skipping secondary controller card %s due to deviation SecondaryControllerCardMemoryUtilizationUnsupported", name)
			continue
		}
		mem := comp.GetMemory()
		if mem == nil {
			continue
		}
		avail := mem.GetAvailable()
		util := mem.GetUtilized()
		if avail == 0 && util == 0 {
			continue
		}
		results[name] = ComponentMemorySample{
			Name:       name,
			Type:       gotType,
			OperStatus: comp.GetOperStatus(),
			Available:  avail,
			Utilized:   util,
		}
	}
	return results
}

// LinearRegression computes the ordinary least-squares slope and coefficient of determination (R^2)
// for y = values[0..n-1] against sample index x = 0..n-1.
func LinearRegression(values []float64) (slope, rSquared float64) {
	n := len(values)
	if n < 2 {
		return 0, 0
	}
	nf := float64(n)
	meanX := float64(n-1) / 2.0
	var sumY float64
	for _, y := range values {
		sumY += y
	}
	meanY := sumY / nf

	var ssXY, ssXX, ssYY float64
	for i, y := range values {
		dx := float64(i) - meanX
		dy := y - meanY
		ssXY += dx * dy
		ssXX += dx * dx
		ssYY += dy * dy
	}
	if ssXX == 0 || ssYY == 0 {
		return 0, 0
	}
	slope = ssXY / ssXX
	r := ssXY / math.Sqrt(ssXX*ssYY)
	rSquared = r * r
	return slope, rSquared
}

// VerifyProcessMemoryWithinBounds checks that all tracked persistent daemons remain present with the
// same PID/name and that their memory-usage growth from baseline to after does not exceed maxPct AND maxBytes.
func VerifyProcessMemoryWithinBounds(
	t *testing.T,
	stepLabel string,
	persistent map[uint64]ProcessSample,
	baseline map[uint64]ProcessSample,
	after map[uint64]ProcessSample,
	maxPct float64,
	maxBytes uint64,
) {
	t.Helper()
	tracked := persistent
	if len(tracked) == 0 {
		tracked = baseline
	}
	for pid, refProc := range tracked {
		baseSample, ok := baseline[pid]
		if !ok {
			baseSample = refProc
		}
		afterSample, present := after[pid]
		if !present || afterSample.Name != refProc.Name {
			// Check whether the daemon crashed and respawned under a new PID (only when no baseline
			// instance of refProc.Name remains running at its original PID).
			var baselineInstanceStillRunning bool
			for basePID, bProc := range baseline {
				if bProc.Name == refProc.Name {
					if cur, ok := after[basePID]; ok && cur.Name == refProc.Name {
						baselineInstanceStillRunning = true
						break
					}
				}
			}
			if !baselineInstanceStillRunning {
				var respawnedPID uint64
				for candPID, candProc := range after {
					if candPID != pid && candProc.Name == refProc.Name {
						if _, existedAtBaseline := baseline[candPID]; !existedAtBaseline {
							respawnedPID = candPID
							break
						}
					}
				}
				if respawnedPID != 0 {
					t.Errorf("%s Fail: Persistent daemon pid=%d name=%q restarted under new pid=%d", stepLabel, pid, refProc.Name, respawnedPID)
				}
			}
			delete(persistent, pid)
			continue
		}
		if afterSample.MemoryUsage <= baseSample.MemoryUsage || baseSample.MemoryUsage == 0 {
			continue
		}
		growthBytes := afterSample.MemoryUsage - baseSample.MemoryUsage
		growthPct := float64(growthBytes) * 100.0 / float64(baseSample.MemoryUsage)
		if growthPct > maxPct && growthBytes > maxBytes {
			t.Errorf("%s Fail: Persistent daemon pid=%d name=%q memory-usage grew by %.2f%% (%d bytes, from %d to %d), exceeding <= %.1f%% (or <= %d bytes) threshold",
				stepLabel, pid, refProc.Name, growthPct, growthBytes, baseSample.MemoryUsage, afterSample.MemoryUsage, maxPct, maxBytes)
		}
	}
}

// VerifyComponentMemoryWithinBounds checks that control-plane components (CONTROLLER_CARD and CHASSIS)
// do not grow in memory/utilized by more than maxPct between baseline and after.
func VerifyComponentMemoryWithinBounds(
	t *testing.T,
	stepLabel string,
	baseline map[string]ComponentMemorySample,
	after map[string]ComponentMemorySample,
	maxPct float64,
) {
	t.Helper()
	for name, baseComp := range baseline {
		if baseComp.Type != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD &&
			baseComp.Type != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS {
			continue
		}
		afterComp, ok := after[name]
		if !ok || afterComp.Utilized <= baseComp.Utilized || baseComp.Utilized == 0 {
			continue
		}
		growthPct := float64(afterComp.Utilized-baseComp.Utilized) * 100.0 / float64(baseComp.Utilized)
		if growthPct > maxPct {
			t.Errorf("%s Fail: Control-plane component %s memory/utilized grew by %.2f%% (from %d to %d), exceeding <= %.1f%% threshold",
				stepLabel, name, growthPct, baseComp.Utilized, afterComp.Utilized, maxPct)
		}
	}
}
