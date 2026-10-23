// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package helpers provides helper APIs to simplify writing FP test cases.
package helpers

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/prototext"
)

// FetchOperStatusUPIntfs function uses telemetry to generate a list of all up interfaces.
// When CheckInterfacesInBinding is set to true, all interfaces that are not defined in binding file are excluded.
func FetchOperStatusUPIntfs(t *testing.T, dut *ondatra.DUTDevice, checkInterfacesInBinding bool) []string {
	t.Helper()
	var intfsOperStatusUP []string
	intfs := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().Name().State())
	bindedIntf := make(map[string]bool)
	for _, port := range dut.Ports() {
		bindedIntf[port.Name()] = true
	}
	for _, intf := range intfs {
		if checkInterfacesInBinding && !bindedIntf[intf] {
			continue
		}
		operStatus, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(intf).OperStatus().State()).Val()
		if present && operStatus == oc.Interface_OperStatus_UP {
			intfsOperStatusUP = append(intfsOperStatusUP, intf)
		}
	}
	sort.Strings(intfsOperStatusUP)
	if len(intfsOperStatusUP) == 0 {
		t.Log("No up interface is found")
	}
	return intfsOperStatusUP
}

// ValidateOperStatusUPIntfs function takes a list of interfaces and validates if they are up.
// if any of the given interfaces is not up, it fails the test and logs the failed interfaces.
func ValidateOperStatusUPIntfs(t *testing.T, dut *ondatra.DUTDevice, upIntfs []string, timeout time.Duration) {
	t.Helper()
	t.Logf("Validate interface OperStatus.")
	if len(upIntfs) == 0 {
		t.Log("Len of upIntfs is 0, skipping the validation of OperStatus.")
		return
	}
	batch := gnmi.OCBatch()
	upInterfaces := make(map[string]bool)
	for _, port := range upIntfs {
		batch.AddPaths(gnmi.OC().Interface(port).OperStatus())
	}
	watch := gnmi.Watch(t, dut, batch.State(), timeout, func(val *ygnmi.Value[*oc.Root]) bool {
		root, present := val.Val()
		if !present {
			return false
		}
		for _, port := range upIntfs {
			if root.GetInterface(port).GetOperStatus() != oc.Interface_OperStatus_UP {
				upInterfaces[port] = false
				return false
			}
			upInterfaces[port] = true
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		for intf, up := range upInterfaces {
			if !up {
				gnmi.Get(t, dut, gnmi.OC().Interface(intf).State())
				t.Logf("Interface %s is not up", intf)
			}
		}
		t.Fatalf("DUT did not reach target state: got %v", val)
	}
}

// GNMINotifString builds a string from a gnmi notification message
func GNMINotifString(n *gpb.Notification) string {
	var build strings.Builder
	prefix, err := ygot.PathToString(n.Prefix)
	if err != nil {
		return prototext.Format(n)
	}
	build.WriteString(fmt.Sprintf("prefix: %s\n", prefix))
	build.WriteString(fmt.Sprintf("timestamp: %d\n", n.GetTimestamp()))
	for _, d := range n.Delete {
		path, err := ygot.PathToString(d)
		if err != nil {
			return prototext.Format(n)
		}
		build.WriteString(fmt.Sprintf("delete: %s\n", path))
	}
	for _, u := range n.Update {
		path, err := ygot.PathToString(u.GetPath())
		if err != nil {
			return prototext.Format(n)
		}
		build.WriteString(fmt.Sprintf("update %s: %v\n", path, u.GetVal()))
	}
	return build.String()
}

// GnmiCLIConfig sets config built with buildCliConfigRequest.
func GnmiCLIConfig(t testing.TB, dut *ondatra.DUTDevice, config string) {
	gnmiClient := dut.RawAPIs().GNMI(t)
	gpbSetRequest, err := buildCliConfigRequest(config)
	if err != nil {
		t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
	}

	t.Log("gnmiClient Set CLI config")
	if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

// buildCliConfigRequest Build config with Origin set to cli and Ascii encoded config.
func buildCliConfigRequest(config string) (*gpb.SetRequest, error) {
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}
	return gpbSetRequest, nil
}

// BuildCliConfigRequest Build config with Origin set to cli and Ascii encoded config.
func BuildCliConfigRequest(config string) (*gpb.SetRequest, error) {
	return buildCliConfigRequest(config)
}

// GetOrCreateLoopback ensures the requested loopback interface/subinterface exists on the DUT
// and returns the loopback interface name. If the interface does not exist, it is created
// (softwareLoopback type) with both IPv4 and IPv6 addresses from loopAttrs in one update. If it
// already exists, any missing address family is configured from loopAttrs, and any existing
// address is read back into loopAttrs so callers always have the correct in-use addresses after
// this call.
func GetOrCreateLoopback(t *testing.T, dut *ondatra.DUTDevice, loopbackID int, subinterface uint32, loopAttrs *attrs.Attributes) string {
	t.Helper()
	loopbackIntfName := netutil.LoopbackInterface(t, dut, loopbackID)
	loopIntf := gnmi.Lookup(t, dut, gnmi.OC().Interface(loopbackIntfName).State())
	if _, ok := loopIntf.Val(); !ok {
		// Interface does not exist: create it with both addresses in one update.
		loopCfg := *loopAttrs
		loopCfg.Subinterface = subinterface
		loop1 := loopCfg.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(loopbackIntfName).Config(), loop1)
		t.Logf("Created loopback interface %s with IPv4=%s IPv6=%s", loopbackIntfName, loopAttrs.IPv4, loopAttrs.IPv6)
	} else {
		// Interface exists: check each family independently.
		lo := gnmi.OC().Interface(loopbackIntfName).Subinterface(subinterface)
		ipv4Addrs := gnmi.LookupAll(t, dut, lo.Ipv4().AddressAny().State())
		if len(ipv4Addrs) == 0 {
			gnmi.Update(t, dut, lo.Ipv4().Address(loopAttrs.IPv4).PrefixLength().Config(), uint8(loopAttrs.IPv4Len))
			t.Logf("Configured missing IPv4 loopback address: %v", loopAttrs.IPv4)
		} else if v4, ok := ipv4Addrs[0].Val(); ok {
			loopAttrs.IPv4 = v4.GetIp()
			t.Logf("Got DUT IPv4 loopback address: %v", loopAttrs.IPv4)
		}
		ipv6Addrs := gnmi.LookupAll(t, dut, lo.Ipv6().AddressAny().State())
		if len(ipv6Addrs) == 0 {
			gnmi.Update(t, dut, lo.Ipv6().Address(loopAttrs.IPv6).PrefixLength().Config(), uint8(loopAttrs.IPv6Len))
			t.Logf("Configured missing IPv6 loopback address: %v", loopAttrs.IPv6)
		} else if v6, ok := ipv6Addrs[0].Val(); ok {
			loopAttrs.IPv6 = v6.GetIp()
			t.Logf("Got DUT IPv6 loopback address: %v", loopAttrs.IPv6)
		}
	}
	return loopbackIntfName
}

// GetRouterTime gets the current time from the router via gNMI to avoid clock skew issues.
func GetRouterTime(t *testing.T, dut *ondatra.DUTDevice) time.Time {
	t.Helper()
	routerTimeStr := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	startTime, err := time.Parse(time.RFC3339Nano, routerTimeStr)
	if err != nil {
		// Fallback to RFC3339 if nano precision is not available.
		startTime, err = time.Parse(time.RFC3339, routerTimeStr)
		if err != nil {
			t.Fatalf("Failed parsing router current-datetime %q: %v", routerTimeStr, err)
		}
	}
	t.Logf("Router current-datetime: %s (parsed UTC: %s)", routerTimeStr, startTime.UTC().Format(time.RFC3339Nano))
	return startTime
}

// GetGnmiCLIOutput sets config built with buildCliConfigRequest and returns the output.
func ExecuteShowCLI(t testing.TB, dut *ondatra.DUTDevice, config string) *gpb.GetResponse {
	getReq := &gpb.GetRequest{
		Path: []*gpb.Path{
			{
				Origin: "cli",
				Elem: []*gpb.PathElem{
					{Name: config},
				},
			},
		},
		Encoding: gpb.Encoding_ASCII,
		Type:     gpb.GetRequest_ALL,
	}

	resp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("gnmiClient.Get() with unexpected error: %v", err)
	}
	return resp
}

// RunCliCommand runs a CLI command on the DUT and returns the output.
func RunCliCommand(t *testing.T, dut *ondatra.DUTDevice, cliCommand string) string {
	cliClient := dut.RawAPIs().CLI(t)
	output, err := cliClient.RunCommand(context.Background(), cliCommand)
	if err != nil {
		t.Fatalf("Failed to execute CLI command '%q': %v", cliCommand, err)
	}
	return output.Output()
}

// WaitForIPSECTunnelsParams contains parameters for waiting for a range of tunnels to come UP.
type WaitForIPSECTunnelsParams struct {
	StartTunnel int
	Count       int
	Timeout     time.Duration
}

// WaitForAllIPSECTunnelsUP waits for the IPSec tunnels numbered [startTunnel,
// startTunnel+count-1] to reach UP state on both DUTs.
func WaitForAllIPSECTunnelsUP(t *testing.T, dut1, dut2 *ondatra.DUTDevice, params WaitForIPSECTunnelsParams) {
	t.Helper()

	endTunnel := params.StartTunnel + params.Count - 1
	t.Logf("Waiting for IPSec tunnels %d-%d to come UP on both DUTs (timeout: %v)...", params.StartTunnel, endTunnel, params.Timeout)

	for i := params.StartTunnel; i <= endTunnel; i++ {
		tunName := fmt.Sprintf("Tunnel%d", i)

		// Check DUT1 tunnel
		path1 := gnmi.OC().Interface(tunName).OperStatus().State()
		_, ok1 := gnmi.Watch(t, dut1, path1, params.Timeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			status, present := val.Val()
			return present && status == oc.Interface_OperStatus_UP
		}).Await(t)
		if !ok1 {
			got1 := gnmi.Get(t, dut1, path1)
			t.Fatalf("DUT1 tunnel %s did not come UP within %v, final status: %v", tunName, params.Timeout, got1)
		}

		// Check DUT2 tunnel
		path2 := gnmi.OC().Interface(tunName).OperStatus().State()
		_, ok2 := gnmi.Watch(t, dut2, path2, params.Timeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			status, present := val.Val()
			return present && status == oc.Interface_OperStatus_UP
		}).Await(t)
		if !ok2 {
			got2 := gnmi.Get(t, dut2, path2)
			t.Fatalf("DUT2 tunnel %s did not come UP within %v, final status: %v", tunName, params.Timeout, got2)
		}

		if (i-params.StartTunnel+1)%32 == 0 || i == endTunnel {
			t.Logf("Tunnels up through %d on both DUTs", i)
		}
	}

	t.Logf("All IPSec tunnels %d-%d are UP on both DUTs", params.StartTunnel, endTunnel)
}

// ReadMemberOutPkts reads out-pkts telemetry counters for all specified member ports on the DUT.
func ReadMemberOutPkts(t *testing.T, dut *ondatra.DUTDevice, memberPorts []*ondatra.Port) map[string]uint64 {
	t.Helper()
	vals := make(map[string]uint64)
	for _, p := range memberPorts {
		vals[p.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Counters().OutPkts().State())
	}
	return vals
}

// DUTDUTLoadBalanceParams contains parameters for verifying load balance across member ports.
type DUTDUTLoadBalanceParams struct {
	MemberPorts    []*ondatra.Port
	Baseline       map[string]uint64
	Tolerance      float64
	WantSingleLink bool
}

// VerifyDUTDUTLoadBalance verifies that traffic delta across member ports matches the expected balance.
func VerifyDUTDUTLoadBalance(t *testing.T, dut *ondatra.DUTDevice, params DUTDUTLoadBalanceParams) error {
	t.Helper()

	after := ReadMemberOutPkts(t, dut, params.MemberPorts)
	delta := make(map[string]uint64)
	var total uint64
	active := 0

	for _, p := range params.MemberPorts {
		name := p.Name()
		if after[name] > params.Baseline[name] {
			delta[name] = after[name] - params.Baseline[name]
		}
		total += delta[name]
		if delta[name] > 0 {
			active++
		}
	}

	if total == 0 {
		return fmt.Errorf("no packets observed on DUT-to-DUT member links")
	}

	var errs []error

	if params.WantSingleLink {
		if active != 1 {
			errs = append(errs, fmt.Errorf("single-link expectation failed: active members got %d, want 1", active))
		}
	} else {
		if active != len(params.MemberPorts) {
			errs = append(errs, fmt.Errorf("balanced load expectation failed: active members got %d, want %d", active, len(params.MemberPorts)))
		}

		evenShare := 1.0 / float64(len(params.MemberPorts))
		for _, p := range params.MemberPorts {
			name := p.Name()
			share := float64(delta[name]) / float64(total)
			if math.Abs(share-evenShare) > params.Tolerance {
				errs = append(errs, fmt.Errorf("member %s share got %.3f, want %.3f +/- %.3f", name, share, evenShare, params.Tolerance))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("load balance verification failed: %v", errs)
	}
	return nil
}

// InterfaceCountersMonitor streams /interfaces/interface[name]/state/counters
// over a single gNMI STREAM subscription (gnmi.Watch) and records any sample in
// which a cumulative counter (in-pkts, out-pkts, in-octets, out-octets) is
// lower than in the previous sample, i.e. the counter decremented or was reset
// to zero. It is intended for validating that counters are reported
// monotonically while a disruptive event (linecard reboot, process restart,
// switchover, ...) happens in the background.
//
// Use StartInterfaceCountersMonitor to begin monitoring and Stop to end it and
// collect the result.
type InterfaceCountersMonitor struct {
	intf        string
	maxDuration time.Duration
	start       time.Time
	watcher     *gnmi.Watcher[*oc.Interface_Counters]

	mu         sync.Mutex
	samples    int
	prev       map[string]uint64
	violations []string
}

// monotonicInterfaceCounters returns the cumulative interface counters that
// must never decrease, keyed by their OpenConfig leaf name.
func monotonicInterfaceCounters(c *oc.Interface_Counters) map[string]*uint64 {
	return map[string]*uint64{
		"in-pkts":    c.InPkts,
		"out-pkts":   c.OutPkts,
		"in-octets":  c.InOctets,
		"out-octets": c.OutOctets,
	}
}

// StartInterfaceCountersMonitor starts monitoring the counters of interface
// intf on dut. The subscription is bounded by maxDuration; Stop should be
// called before maxDuration expires, otherwise Stop reports that part of the
// monitoring window was not covered.
func StartInterfaceCountersMonitor(t *testing.T, dut *ondatra.DUTDevice, intf string, maxDuration time.Duration) *InterfaceCountersMonitor {
	t.Helper()
	m := &InterfaceCountersMonitor{
		intf:        intf,
		maxDuration: maxDuration,
		start:       time.Now(),
	}
	// The predicate never returns true: the watch keeps recording samples
	// until Stop cancels it (or maxDuration expires).
	m.watcher = gnmi.Watch(t, dut, gnmi.OC().Interface(intf).Counters().State(), maxDuration, func(v *ygnmi.Value[*oc.Interface_Counters]) bool {
		m.record(v)
		return false
	})
	t.Logf("Started counters monitor for interface %s (max duration %v)", intf, maxDuration)
	return m
}

// record stores one streamed sample and notes any decrement versus the
// previous sample.
//
// Samples that carry no OpenConfig counter value are ignored: some targets
// (e.g. Juniper) also stream notifications under the counters container that
// contain only non-OpenConfig (vendor) leaves, which ygnmi reports as not
// present. They are neither a decrement nor a reset to zero. A counter that is
// reset to zero still arrives as a value lower than the previous sample and is
// reported as a decrement.
func (m *InterfaceCountersMonitor) record(v *ygnmi.Value[*oc.Interface_Counters]) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := v.Val()
	if !ok {
		return
	}
	cur := make(map[string]uint64)
	for name, val := range monotonicInterfaceCounters(c) {
		if val == nil {
			continue
		}
		cur[name] = *val
		if prev, ok := m.prev[name]; ok && *val < prev {
			m.violations = append(m.violations, fmt.Sprintf("%s %s decreased from %d to %d (timestamp %v)", m.intf, name, prev, *val, v.Timestamp))
		}
	}
	if len(cur) == 0 {
		return
	}
	m.samples++
	// Keep the last known value for counters missing from this sample so a
	// later sample is still compared against it.
	for name, val := range m.prev {
		if _, ok := cur[name]; !ok {
			cur[name] = val
		}
	}
	m.prev = cur
}

// Stop ends the monitoring subscription and returns the number of samples
// received and an error describing every decrement observed, if any. An error
// is also returned if no samples were received or if the subscription expired
// (maxDuration) before Stop was called.
func (m *InterfaceCountersMonitor) Stop(t *testing.T) (int, error) {
	t.Helper()
	expired := time.Since(m.start) >= m.maxDuration
	m.watcher.Cancel()
	// The predicate never returns true, so Await always reports false once
	// the subscription is cancelled or times out.
	m.watcher.Await(t)

	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []string
	if m.samples == 0 {
		errs = append(errs, fmt.Sprintf("%s: no counter samples received", m.intf))
	}
	if expired {
		errs = append(errs, fmt.Sprintf("%s: monitor expired after %v before Stop was called; part of the window was not monitored", m.intf, m.maxDuration))
	}
	errs = append(errs, m.violations...)
	t.Logf("Stopped counters monitor for interface %s after %v: %d samples, %d violations", m.intf, time.Since(m.start).Round(time.Second), m.samples, len(m.violations))
	if len(errs) > 0 {
		return m.samples, fmt.Errorf("interface counters monitor detected issues: %s", strings.Join(errs, "; "))
	}
	return m.samples, nil
}

// AwaitSupervisorRoles (gNOI-3.3.1 Step 2 / gNOI-3.3.2 Step 3) awaits the expected
// PRIMARY and SECONDARY redundant-role states across supervisor switchovers.
func AwaitSupervisorRoles(t *testing.T, dut *ondatra.DUTDevice, expectedPrimary, expectedSecondary string, timeout time.Duration) error {
	t.Helper()

	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
	if err != nil {
		return fmt.Errorf("failed to create ygnmi client: %w", err)
	}

	qPrimary := gnmi.OC().Component(expectedPrimary).RedundantRole().State()
	qSecondary := gnmi.OC().Component(expectedSecondary).RedundantRole().State()

	primaryReady := false
	secondaryReady := false

	var opts []ygnmi.Option
	if deviations.SwitchoverSubscribeUnsupported(dut) {
		t.Logf("WARNING: [AwaitSupervisorRoles] Deviation SwitchoverSubscribeUnsupported on %s (%s): polling redundant-role via unary gNMI.Get for up to %v (awaiting %s=PRIMARY, %s=SECONDARY).", dut.Name(), dut.Model(), timeout, expectedPrimary, expectedSecondary)
		opts = append(opts, ygnmi.WithUseGet())
	} else {
		t.Logf("INFO: [AwaitSupervisorRoles] Polling redundant-role via one-shot gNMI.Subscribe (mode: ONCE) on %s (%s) for up to %v (awaiting %s=PRIMARY, %s=SECONDARY)...", dut.Name(), dut.Model(), timeout, expectedPrimary, expectedSecondary)
	}

	start := time.Now()
	for time.Since(start) < timeout {
		if !primaryReady {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			val, err := ygnmi.Lookup(ctx, c, qPrimary, opts...)
			cancel()
			if err != nil && len(opts) > 0 && (strings.Contains(err.Error(), "Unimplemented") || strings.Contains(err.Error(), "Unsupported 'type'")) {
				t.Logf("INFO: [AwaitSupervisorRoles] Device %s (%s) does not support unary gNMI.Get for STATE paths; switching polling to one-shot gNMI.Subscribe (mode: ONCE) via ygnmi.Lookup.", dut.Name(), dut.Model())
				opts = nil
				ctxRetry, cancelRetry := context.WithTimeout(context.Background(), 10*time.Second)
				val, err = ygnmi.Lookup(ctxRetry, c, qPrimary)
				cancelRetry()
			}

			if err != nil {
				t.Logf("DEBUG: [%.1fs] Error fetching primary role: %v", time.Since(start).Seconds(), err)
			} else if role, present := val.Val(); present {
				t.Logf("DEBUG: [%.1fs] %s RedundantRole = %v", time.Since(start).Seconds(), expectedPrimary, role)
				if role == oc.Platform_ComponentRedundantRole_PRIMARY {
					primaryReady = true
					t.Logf("INFO: Supervisor %q reached PRIMARY role.", expectedPrimary)
				}
			}
		}

		if !secondaryReady {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			val, err := ygnmi.Lookup(ctx, c, qSecondary, opts...)
			cancel()
			if err != nil && len(opts) > 0 && (strings.Contains(err.Error(), "Unimplemented") || strings.Contains(err.Error(), "Unsupported 'type'")) {
				t.Logf("INFO: [AwaitSupervisorRoles] Device %s (%s) does not support unary gNMI.Get for STATE paths; switching polling to one-shot gNMI.Subscribe (mode: ONCE) via ygnmi.Lookup.", dut.Name(), dut.Model())
				opts = nil
				ctxRetry, cancelRetry := context.WithTimeout(context.Background(), 10*time.Second)
				val, err = ygnmi.Lookup(ctxRetry, c, qSecondary)
				cancelRetry()
			}

			if err != nil {
				t.Logf("DEBUG: [%.1fs] Error fetching secondary role: %v", time.Since(start).Seconds(), err)
			} else if role, present := val.Val(); present {
				t.Logf("DEBUG: [%.1fs] %s RedundantRole = %v", time.Since(start).Seconds(), expectedSecondary, role)
				if role == oc.Platform_ComponentRedundantRole_SECONDARY {
					secondaryReady = true
					t.Logf("INFO: Supervisor %q reached SECONDARY role.", expectedSecondary)
				}
			}
		}

		if primaryReady && secondaryReady {
			t.Logf("SUCCESS: Supervisors reached expected redundant roles.")
			return nil
		}

		time.Sleep(10 * time.Second)
	}

	var errMsgs []string
	if !primaryReady {
		errMsgs = append(errMsgs, fmt.Sprintf("supervisor %q failed to reach PRIMARY role within %v", expectedPrimary, timeout))
	}
	if !secondaryReady {
		errMsgs = append(errMsgs, fmt.Sprintf("supervisor %q failed to reach SECONDARY role within %v", expectedSecondary, timeout))
	}
	return fmt.Errorf("supervisor switchover validation failed due to timeout: %s", strings.Join(errMsgs, "; "))
}
