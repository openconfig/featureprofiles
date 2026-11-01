// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package components provides functions to enumerate components from the device.
package components

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/openconfig/featureprofiles/internal/deviations"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

// componentsCache stores fetched components per device.
// The key is the DUT name. This cache persists across different test functions
// within the same execution of `go test`.
var componentsCache = make(map[string][]*oc.Component)

// fetchCachedComponents fetches all components from the DUT, using a package-level cache.
func fetchCachedComponents(t *testing.T, dut *ondatra.DUTDevice) []*oc.Component {
	dutName := dut.Name()
	if cached, ok := componentsCache[dutName]; ok {
		t.Logf("Using cached components for DUT %s.", dutName)
		return cached
	}
	t.Logf("Fetching all components for DUT %s.", dutName)
	components := gnmi.GetAll[*oc.Component](t, dut, gnmi.OC().ComponentAny().State())
	componentsCache[dutName] = components
	return components
}

// FindComponentsByType finds the list of components based on hardware type.
func FindComponentsByType(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	t.Helper()
	components := fetchCachedComponents(t, dut)
	var s []string
	for _, c := range components {
		switch v := c.GetType().(type) {
		case oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
			if v == cType {
				s = append(s, c.GetName())
			}
		}
	}
	return s
}

// FindActiveComponentsByType finds the list of active components based on hardware type.
func FindActiveComponentsByType(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	t.Helper()
	components := gnmi.GetAll[*oc.Component](t, dut, gnmi.OC().ComponentAny().State())
	var s []string
	for _, c := range components {
		switch v := c.GetType().(type) {
		case oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
			if v == cType && c.OperStatus == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				s = append(s, c.GetName())
			}
		}
	}
	return s
}

// FindSWComponentsByType finds the list of SW components based on a type.
func FindSWComponentsByType(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT) []string {
	t.Helper()
	components := fetchCachedComponents(t, dut)
	var s []string
	for _, c := range components {
		switch v := c.GetType().(type) {
		case oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT:
			if v == cType {
				s = append(s, c.GetName())
			}
		}
	}
	return s
}

// FindMatchingStrings filters out the components list based on regex pattern.
func FindMatchingStrings(components []string, r *regexp.Regexp) []string {
	var s []string
	for _, c := range components {
		if r.MatchString(c) {
			s = append(s, c)
		}
	}
	return s
}

// AwaitSwitchoverReady waits for the active controller to report switchover-ready.
// The active supervisor's switchover-ready leaf accurately reflects whether the
// standby is ready to take over; the standby's own leaf is not reliably maintained.
func AwaitSwitchoverReady(t *testing.T, dut *ondatra.DUTDevice, active string, timeout time.Duration) {
	t.Helper()
	switchoverReady := gnmi.OC().Component(active).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), timeout, true)
	t.Logf("SwitchoverReady: %v", gnmi.Get(t, dut, switchoverReady.State()))
}

// GetSubcomponentPath creates a gNMI path based on the component name.
// If useNameOnly is true, returns a path to the specified name instead of a full subcomponent path.
func GetSubcomponentPath(name string, useNameOnly bool) *tpb.Path {
	if useNameOnly {
		return &tpb.Path{
			Elem: []*tpb.PathElem{{Name: name}},
		}
	}
	return &tpb.Path{
		Origin: "openconfig",
		Elem: []*tpb.PathElem{
			{Name: "components"},
			{Name: "component", Key: map[string]string{"name": name}},
		},
	}
}

// Y provides the ygnmi based components helper.  A ygnmi.Client is tied to a specific
// DUT.
type Y struct {
	*ygnmi.Client
	// typeCache caches the results of FindByType.
	// Key: oc.Component_Type_Union, Value: []string (component names).
	typeCache map[oc.Component_Type_Union][]string
}

// New creates a new ygnmi based helper from a *ondatra.DUTDevice.
func New(t testing.TB, dut *ondatra.DUTDevice) Y {
	gnmic := dut.RawAPIs().GNMI(t)
	yc, err := ygnmi.NewClient(gnmic)
	if err != nil {
		t.Fatalf("Could not create ygnmi.Client: %v", err)
	}
	return Y{
		Client:    yc,
		typeCache: make(map[oc.Component_Type_Union][]string),
	}
}

// FindByType finds the list of components based on component type.
func (y Y) FindByType(ctx context.Context, want oc.Component_Type_Union) ([]string, error) {
	if y.typeCache == nil {
		y.typeCache = make(map[oc.Component_Type_Union][]string)
	}
	if cached, ok := y.typeCache[want]; ok {
		return cached, nil
	}

	var names []string

	anyTypePath := ocpath.Root().ComponentAny().Type()
	values, err := ygnmi.LookupAll(ctx, y.Client, anyTypePath.State())
	if err != nil {
		return nil, err
	}

	for _, value := range values {
		if got, ok := value.Val(); ok {
			if got != want {
				continue
			}
			name := value.Path.GetElem()[1].GetKey()["name"]
			names = append(names, name)
		}
	}

	if len(names) < 1 {
		return nil, fmt.Errorf("none of the %d components match %v", len(values), want)
	}

	y.typeCache[want] = names
	return names, nil
}

// FindStandbyControllerCard gets a list of two components and finds out the active and standby controller_cards.
func FindStandbyControllerCard(t *testing.T, dut *ondatra.DUTDevice, supervisors []string) (string, string) {
	var activeCC, standbyCC string
	for _, supervisor := range supervisors {
		watch := gnmi.Watch(t, dut, gnmi.OC().Component(supervisor).RedundantRole().State(), 10*time.Minute, func(val *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
			return val.IsPresent()
		})
		if val, ok := watch.Await(t); !ok {
			t.Fatalf("DUT did not reach target state within %v: got %v", 10*time.Minute, val)
		}
		role := gnmi.Get(t, dut, gnmi.OC().Component(supervisor).RedundantRole().State())
		t.Logf("Component(supervisor).RedundantRole().Get(t): %v, Role: %v", supervisor, role)
		if role == standbyController {
			standbyCC = supervisor
		} else if role == activeController {
			activeCC = supervisor
		} else {
			t.Fatalf("Expected controller %s to be active or standby, got %v", supervisor, role)
		}
	}
	if standbyCC == "" || activeCC == "" {
		t.Fatalf("Expected non-empty activeCC and standbyCC, got activeCC: %v, standbyCC: %v", activeCC, standbyCC)
	}
	t.Logf("Detected activeCC: %v, standbyCC: %v", activeCC, standbyCC)

	return standbyCC, activeCC
}

// OpticalChannelComponentFromPort finds the optical channel component for a port.
func OpticalChannelComponentFromPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	t.Helper()

	if deviations.MissingPortToOpticalChannelMapping(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
			return fmt.Sprintf("%s-Optical0", transceiverName)
		case ondatra.CISCO:
			return "OpticalChannel" + strings.TrimLeftFunc(p.Name(), unicode.IsLetter)
		default:
			t.Fatal("Manual Optical channel name required when deviation missing_port_to_optical_channel_component_mapping applied.")
		}
	}
	transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
	if transceiverName == "" {
		t.Fatalf("Associated Transceiver for Interface (%v) not found!", p.Name())
	}
	opticalChannelName := gnmi.Get(t, dut, gnmi.OC().Component(transceiverName).Transceiver().Channel(0).AssociatedOpticalChannel().State())
	if opticalChannelName == "" {
		t.Fatalf("Associated Optical Channel for Transceiver (%v) not found!", transceiverName)
	}
	return opticalChannelName
}

// ErrNoLinecardComponents is returned by LinecardsFromPorts when the device
// does not report any component of type LINECARD (e.g. fixed form factor
// devices).
var ErrNoLinecardComponents = errors.New("device reports no LINECARD components")

// componentNameFromPath returns the component name key of a
// /components/component[name=...]/... path, or "" if absent.
func componentNameFromPath(p *gpb.Path) string {
	for _, e := range p.GetElem() {
		if e.GetName() == "component" {
			return e.GetKey()["name"]
		}
	}
	return ""
}

// LinecardsFromPorts returns, keyed by DUT interface name, the name of the
// LINECARD component that hosts each of the given ports.
//
// For each port it reads /interfaces/interface[name]/state/hardware-port and
// then follows /components/component/state/parent up the component tree until
// it reaches a component whose type is LINECARD. The data is fetched with three
// gNMI requests in total (one batched Get for the hardware-ports and one
// wildcard Get each for the component types and parents) and the tree is
// walked in memory, regardless of the number of ports or the tree depth.
//
// ErrNoLinecardComponents is returned (wrapped) when the device reports no
// LINECARD component at all; any other error means a port could not be mapped
// to a linecard.
func LinecardsFromPorts(t testing.TB, dut *ondatra.DUTDevice, ports []*ondatra.Port) (map[string]string, error) {
	t.Helper()
	if len(ports) == 0 {
		return nil, errors.New("LinecardsFromPorts: no ports given")
	}
	types := make(map[string]oc.Component_Type_Union)
	hasLinecard := false
	for _, v := range gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().Type().State()) {
		if typ, ok := v.Val(); ok {
			name := componentNameFromPath(v.Path)
			types[name] = typ
			if typ == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD {
				hasLinecard = true
			}
		}
	}
	if !hasLinecard {
		return nil, fmt.Errorf("%w (%d components checked)", ErrNoLinecardComponents, len(types))
	}
	parents := make(map[string]string)
	for _, v := range gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().Parent().State()) {
		if parent, ok := v.Val(); ok {
			parents[componentNameFromPath(v.Path)] = parent
		}
	}

	batch := gnmi.OCBatch()
	for _, p := range ports {
		batch.AddPaths(gnmi.OC().Interface(p.Name()).HardwarePort())
	}
	root, _ := gnmi.Lookup(t, dut, batch.State()).Val()

	lcs := make(map[string]string)
	for _, p := range ports {
		hwPort := root.GetInterface(p.Name()).GetHardwarePort()
		if hwPort == "" {
			return nil, fmt.Errorf("hardware-port not reported for interface %s", p.Name())
		}
		visited := make(map[string]bool)
		var lc string
		for current := hwPort; current != ""; current = parents[current] {
			if visited[current] {
				return nil, fmt.Errorf("loop detected in component tree at %q while walking up from hardware-port %q", current, hwPort)
			}
			visited[current] = true
			if types[current] == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD {
				lc = current
				break
			}
		}
		if lc == "" {
			return nil, fmt.Errorf("no LINECARD component found in the component tree above hardware-port %q of interface %s", hwPort, p.Name())
		}
		lcs[p.Name()] = lc
	}
	return lcs, nil
}

// WatchWithRetry watches a query until the predicate returns true, catching temporary connection errors
// and retrying every retryInterval until timeout expires.
//
// The timeout is measured in wall-clock time. If a watch attempt fails before
// its retryInterval slice has elapsed (e.g. the gNMI server is unreachable
// during a controller card switchover and Subscribe fails immediately), the
// remainder of the slice is waited out before re-subscribing. This back-off is
// required because no gNMI Watch can be established while the server is
// unreachable; without it, fast failures would either spin or exhaust the
// timeout budget within seconds.
func WatchWithRetry[T any](t *testing.T, dut *ondatra.DUTDevice, query ygnmi.SingletonQuery[T], timeout, retryInterval time.Duration, predicate func(*ygnmi.Value[T]) bool) (*ygnmi.Value[T], bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, false
		}
		waitDur := min(remaining, retryInterval)
		sliceEnd := time.Now().Add(waitDur)
		var val *ygnmi.Value[T]
		var ok bool
		testt.CaptureFatal(t, func(t testing.TB) {
			val, ok = gnmi.Watch(t, dut, query, waitDur, predicate).Await(t)
		})
		if ok {
			return val, true
		}
		if r := time.Until(sliceEnd); r > 0 {
			time.Sleep(r)
		}
	}
}

// WaitForSwitchover verifies the stability of the SUP cards after switchover by watching the system datetime.
func WaitForSwitchover(t *testing.T, dut *ondatra.DUTDevice, maxSwitchoverTime time.Duration) {
	t.Helper()
	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by watching the system datetime telemetry.")

	val, ok := watchCurrentDatetime(t, dut, maxSwitchoverTime)
	if !ok {
		t.Fatalf("RP switchover timed out after %v: system datetime not reachable", maxSwitchoverTime)
	}
	currentTime, _ := val.Val()
	t.Logf("RP switchover has completed successfully in %.2f seconds with received time: %v", time.Since(startSwitchover).Seconds(), currentTime)
}

// AwaitGNMIReachable waits up to timeout until the DUT serves
// /system/state/current-datetime over gNMI, tolerating disconnects. Unlike
// WaitForSwitchover it does not fail the test, so it is safe to use in
// cleanup and retry paths. It returns whether the DUT became reachable.
func AwaitGNMIReachable(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) bool {
	t.Helper()
	_, ok := watchCurrentDatetime(t, dut, timeout)
	return ok
}

// watchCurrentDatetime waits up to timeout, tolerating gNMI disconnects, until
// the DUT reports a non-empty /system/state/current-datetime.
func watchCurrentDatetime(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) (*ygnmi.Value[string], bool) {
	t.Helper()
	return WatchWithRetry(t, dut, gnmi.OC().System().CurrentDatetime().State(), timeout, 30*time.Second, func(val *ygnmi.Value[string]) bool {
		currentTime, present := val.Val()
		return present && currentTime != ""
	})
}

// SetLinecardPowerAdminState sets /components/component/linecard/config/power-admin-state
// of the linecard component name to state and waits up to timeout for the
// corresponding state leaf to report the same value. The
// PowerDisableEnableLeafRefValidation deviation is honoured by creating the
// component config node first. When state is POWER_ENABLED and the
// MissingValueForDefaults deviation is set, the state leaf is not verified since
// the device does not report default values. The test is failed fatally if the
// state leaf does not converge.
func SetLinecardPowerAdminState(t *testing.T, dut *ondatra.DUTDevice, name string, state oc.E_Platform_ComponentPowerType, timeout time.Duration) {
	t.Helper()
	c := gnmi.OC().Component(name)
	if deviations.PowerDisableEnableLeafRefValidation(dut) {
		gnmi.Update(t, dut, c.Config(), &oc.Component{Name: ygot.String(name)})
	}
	t.Logf("Setting linecard %s power-admin-state to %v", name, state)
	gnmi.Replace(t, dut, c.Linecard().PowerAdminState().Config(), state)

	if state == oc.Platform_ComponentPowerType_POWER_ENABLED && deviations.MissingValueForDefaults(dut) {
		return
	}
	got, ok := gnmi.Await(t, dut, c.Linecard().PowerAdminState().State(), timeout, state).Val()
	if !ok {
		t.Fatalf("Linecard %s power-admin-state: got %v, want %v", name, got, state)
	}
}

// AwaitOperStatus waits up to timeout, using gnmi.Watch, until the oper-status of
// component name equals any of want. It returns the last observed oper-status and
// whether one of the wanted values was observed.
func AwaitOperStatus(t *testing.T, dut *ondatra.DUTDevice, name string, timeout time.Duration, want ...oc.E_PlatformTypes_COMPONENT_OPER_STATUS) (oc.E_PlatformTypes_COMPONENT_OPER_STATUS, bool) {
	t.Helper()
	val, ok := gnmi.Watch(t, dut, gnmi.OC().Component(name).OperStatus().State(), timeout, func(v *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
		s, present := v.Val()
		return present && slices.Contains(want, s)
	}).Await(t)
	if val == nil {
		return oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET, false
	}
	s, _ := val.Val()
	return s, ok
}

// AwaitOperStatusOrAbsent waits up to timeout until the oper-status of component
// name equals any of want or the component no longer reports an oper-status, e.g.
// a subcomponent that is removed from the component tree while its parent
// linecard is powered off. It returns the last observed oper-status, which is
// UNSET when the oper-status is no longer reported, and whether the wait
// succeeded.
//
// A STREAM subscription to a leaf that is already absent or whose parent
// component is removed without a leaf-level delete notification may not deliver
// any notification, so gnmi.Lookup is checked before each bounded gnmi.Watch
// slice until timeout expires.
func AwaitOperStatusOrAbsent(t *testing.T, dut *ondatra.DUTDevice, name string, timeout time.Duration, want ...oc.E_PlatformTypes_COMPONENT_OPER_STATUS) (oc.E_PlatformTypes_COMPONENT_OPER_STATUS, bool) {
	t.Helper()
	const watchSlice = 10 * time.Second
	query := gnmi.OC().Component(name).OperStatus().State()
	deadline := time.Now().Add(timeout)
	var last oc.E_PlatformTypes_COMPONENT_OPER_STATUS
	for {
		s, present := gnmi.Lookup(t, dut, query).Val()
		if !present || slices.Contains(want, s) {
			return s, true
		}
		last = s
		rem := time.Until(deadline)
		if rem <= 0 {
			return last, false
		}
		if rem > watchSlice {
			rem = watchSlice
		}
		val, ok := gnmi.Watch(t, dut, query, rem, func(v *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
			s, present := v.Val()
			return !present || slices.Contains(want, s)
		}).Await(t)
		if ok {
			if val == nil {
				return oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET, true
			}
			s, _ := val.Val()
			return s, true
		}
	}
}

// switchoverWatchRetryInterval is the interval at which watches that may span a
// controller card switchover are re-established after a gNMI disconnect.
const switchoverWatchRetryInterval = 30 * time.Second

// SwitchoverTimeout returns base extended by the number of minutes configured
// in the SwitchoverStabilizeDelayM deviation. Use it for any wait that spans a
// controller card switchover so slow platforms are handled without vendor
// checks.
func SwitchoverTimeout(dut *ondatra.DUTDevice, base time.Duration) time.Duration {
	return base + time.Duration(deviations.SwitchoverStabilizeDelayM(dut))*time.Minute
}

// PowerAdminStateQueries returns the config and state queries of the
// power-admin-state leaf of component name, which is of hardware type cType:
//
//	/components/component/controller-card/{config|state}/power-admin-state
//	/components/component/linecard/{config|state}/power-admin-state
//	/components/component/fabric/{config|state}/power-admin-state
//
// An error is returned for hardware types that do not model power-admin-state.
func PowerAdminStateQueries(name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) (ygnmi.ConfigQuery[oc.E_Platform_ComponentPowerType], ygnmi.SingletonQuery[oc.E_Platform_ComponentPowerType], error) {
	c := gnmi.OC().Component(name)
	switch cType {
	case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD:
		return c.ControllerCard().PowerAdminState().Config(), c.ControllerCard().PowerAdminState().State(), nil
	case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD:
		return c.Linecard().PowerAdminState().Config(), c.Linecard().PowerAdminState().State(), nil
	case oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC:
		return c.Fabric().PowerAdminState().Config(), c.Fabric().PowerAdminState().State(), nil
	default:
		return nil, nil, fmt.Errorf("component type %v does not support power-admin-state", cType)
	}
}

// TrySetPowerAdminState replaces the config/power-admin-state leaf of component
// name, which is of hardware type cType, with power. When the
// PowerDisableEnableLeafRefValidation or ConfigLeafCreateRequired deviation is
// set, the component config node is created first so the leafref to the
// component name resolves.
//
// Unlike SetPowerAdminState, a rejected gNMI Set is returned as an error rather
// than failing the test, which allows negative scenarios (e.g. powering down
// the last controller card) to be validated.
func TrySetPowerAdminState(t *testing.T, dut *ondatra.DUTDevice, name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, power oc.E_Platform_ComponentPowerType) error {
	t.Helper()
	config, _, err := PowerAdminStateQueries(name, cType)
	if err != nil {
		return err
	}
	if deviations.PowerDisableEnableLeafRefValidation(dut) || deviations.ConfigLeafCreateRequired(dut) {
		if msg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, gnmi.OC().Component(name).Config(), &oc.Component{Name: ygot.String(name)})
		}); msg != nil {
			return fmt.Errorf("creating config node of component %s: %s", name, *msg)
		}
	}
	t.Logf("Setting %v component %s config/power-admin-state to %v", cType, name, power)
	if msg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Replace(t, dut, config, power)
	}); msg != nil {
		return fmt.Errorf("setting component %s config/power-admin-state to %v: %s", name, power, *msg)
	}
	return nil
}

// SetPowerAdminState is like TrySetPowerAdminState but fails the test fatally
// if the gNMI Set is rejected.
func SetPowerAdminState(t *testing.T, dut *ondatra.DUTDevice, name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, power oc.E_Platform_ComponentPowerType) {
	t.Helper()
	if err := TrySetPowerAdminState(t, dut, name, cType, power); err != nil {
		t.Fatalf("SetPowerAdminState: %v", err)
	}
}

// AwaitPowerAdminState waits up to timeout until the state/power-admin-state
// leaf of component name, which is of hardware type cType, equals want. The
// watch tolerates gNMI disconnects (e.g. during a controller card switchover).
// It returns the last observed value and whether want was observed.
//
// When want is POWER_ENABLED (the YANG default) and the MissingValueForDefaults
// deviation is set, an absent leaf is treated as POWER_ENABLED.
func AwaitPowerAdminState(t *testing.T, dut *ondatra.DUTDevice, name string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, want oc.E_Platform_ComponentPowerType, timeout time.Duration) (oc.E_Platform_ComponentPowerType, bool) {
	t.Helper()
	_, state, err := PowerAdminStateQueries(name, cType)
	if err != nil {
		t.Fatalf("AwaitPowerAdminState: %v", err)
	}
	absentIsWant := want == oc.Platform_ComponentPowerType_POWER_ENABLED && deviations.MissingValueForDefaults(dut)
	// A STREAM subscription to an absent leaf may not deliver any notification,
	// so check the current value first when an absent leaf is acceptable.
	if absentIsWant {
		absent := false
		testt.CaptureFatal(t, func(t testing.TB) {
			_, present := gnmi.Lookup(t, dut, state).Val()
			absent = !present
		})
		if absent {
			return want, true
		}
	}
	val, ok := WatchWithRetry(t, dut, state, timeout, switchoverWatchRetryInterval, func(v *ygnmi.Value[oc.E_Platform_ComponentPowerType]) bool {
		got, present := v.Val()
		if !present {
			return absentIsWant
		}
		return got == want
	})
	if !ok {
		var last oc.E_Platform_ComponentPowerType
		testt.CaptureFatal(t, func(t testing.TB) {
			last, _ = gnmi.Lookup(t, dut, state).Val()
		})
		return last, false
	}
	got, present := val.Val()
	if !present {
		return want, true
	}
	return got, true
}

// AwaitOperStatusWithRetry is like AwaitOperStatus but tolerates gNMI
// disconnects, e.g. while a controller card switchover is in progress. It
// returns the last observed oper-status and whether one of want was observed.
func AwaitOperStatusWithRetry(t *testing.T, dut *ondatra.DUTDevice, name string, timeout time.Duration, want ...oc.E_PlatformTypes_COMPONENT_OPER_STATUS) (oc.E_PlatformTypes_COMPONENT_OPER_STATUS, bool) {
	t.Helper()
	query := gnmi.OC().Component(name).OperStatus().State()
	val, ok := WatchWithRetry(t, dut, query, timeout, switchoverWatchRetryInterval, func(v *ygnmi.Value[oc.E_PlatformTypes_COMPONENT_OPER_STATUS]) bool {
		s, present := v.Val()
		return present && slices.Contains(want, s)
	})
	if !ok {
		var last oc.E_PlatformTypes_COMPONENT_OPER_STATUS
		testt.CaptureFatal(t, func(t testing.TB) {
			last, _ = gnmi.Lookup(t, dut, query).Val()
		})
		return last, false
	}
	s, _ := val.Val()
	return s, true
}

// findInstalled returns the components of hardware type cType that are not
// reported as empty and, when requireRemovable is true, are reported as
// removable. The empty and removable leaves of all candidates are fetched in a
// single batched gNMI Get. A component that reports neither leaf is not
// considered installed; otherwise a leaf that is not reported is treated as
// false.
func findInstalled(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, requireRemovable bool) []string {
	t.Helper()
	names := FindComponentsByType(t, dut, cType)
	if len(names) == 0 {
		return nil
	}
	batch := gnmi.OCBatch()
	for _, name := range names {
		batch.AddPaths(
			gnmi.OC().Component(name).Empty(),
			gnmi.OC().Component(name).Removable(),
		)
	}
	// root is nil if none of the leaves is reported; the generated getters are
	// nil-safe.
	root, _ := gnmi.Lookup(t, dut, batch.State()).Val()
	var installed []string
	for _, name := range names {
		c := root.GetComponent(name)
		if c == nil || c.GetEmpty() || (requireRemovable && !c.GetRemovable()) {
			continue
		}
		installed = append(installed, name)
	}
	return installed
}

// FindInstalledComponents returns the components of hardware type cType whose
// /components/component/state/empty leaf is not true, i.e. whose slot is
// populated. Components that report neither state/empty nor state/removable
// are not returned.
func FindInstalledComponents(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	t.Helper()
	return findInstalled(t, dut, cType, false)
}

// FindInstalledRemovableComponents returns the components of hardware type
// cType that are installed (state/empty is not true) and removable
// (state/removable is true), e.g. linecards and fabric cards that can be
// powered down individually.
func FindInstalledRemovableComponents(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	t.Helper()
	return findInstalled(t, dut, cType, true)
}

// minPerComponentWait is the minimum time the multi-component Await helpers
// wait for each component, so that every component is checked at least once
// even if earlier components used up the shared timeout.
const minPerComponentWait = 30 * time.Second

// waitUntil returns the time left until deadline, but at least
// minPerComponentWait.
func waitUntil(deadline time.Time) time.Duration {
	return max(time.Until(deadline), minPerComponentWait)
}

// The multi-component Await helpers below wait on each component with its own
// single-leaf gnmi.Watch, sharing one timeout, instead of one batched watch:
// some devices (e.g. Juniper) deliver the values of a batched multi-path STREAM
// subscription only after many minutes, while single-leaf subscriptions are
// served immediately.

// AwaitComponentsOperStatus waits, sharing timeout across all components (with
// at least minPerComponentWait per component), until the oper-status of every
// component in names equals want. Each component is watched with
// AwaitOperStatusWithRetry, which tolerates gNMI disconnects. It returns the
// components that did not reach want and whether all components reached want.
func AwaitComponentsOperStatus(t *testing.T, dut *ondatra.DUTDevice, names []string, timeout time.Duration, want oc.E_PlatformTypes_COMPONENT_OPER_STATUS) ([]string, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var notMatching []string
	for _, name := range names {
		if _, ok := AwaitOperStatusWithRetry(t, dut, name, waitUntil(deadline), want); !ok {
			notMatching = append(notMatching, name)
		}
	}
	return notMatching, len(notMatching) == 0
}

// AwaitRedundantRole waits up to timeout until the state/redundant-role of
// controller card name equals want. The watch is re-established every 30
// seconds so that a gNMI disconnect caused by a controller card switchover
// does not fail the test. It returns the observed role (UNSET if want was not
// observed) and whether want was observed.
func AwaitRedundantRole(t *testing.T, dut *ondatra.DUTDevice, name string, want oc.E_Platform_ComponentRedundantRole, timeout time.Duration) (oc.E_Platform_ComponentRedundantRole, bool) {
	t.Helper()
	val, ok := WatchWithRetry(t, dut, gnmi.OC().Component(name).RedundantRole().State(), timeout, switchoverWatchRetryInterval, func(v *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
		role, present := v.Val()
		return present && role == want
	})
	if !ok || val == nil {
		return oc.Platform_ComponentRedundantRole_UNSET, false
	}
	role, _ := val.Val()
	return role, true
}

// AwaitComponentsPowerAdminState waits, sharing timeout across all components
// (with at least minPerComponentWait per component), until the
// state/power-admin-state of every component in names, which are of hardware
// type cType, equals want. Each component is checked with AwaitPowerAdminState,
// which tolerates gNMI disconnects. It returns the components that did not
// reach want and whether all components reached want.
//
// When want is POWER_ENABLED (the YANG default) and the MissingValueForDefaults
// deviation is set, an absent leaf is treated as POWER_ENABLED.
func AwaitComponentsPowerAdminState(t *testing.T, dut *ondatra.DUTDevice, names []string, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, want oc.E_Platform_ComponentPowerType, timeout time.Duration) ([]string, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var notMatching []string
	for _, name := range names {
		if _, ok := AwaitPowerAdminState(t, dut, name, cType, want, waitUntil(deadline)); !ok {
			notMatching = append(notMatching, name)
		}
	}
	return notMatching, len(notMatching) == 0
}

// AwaitComponentsSwitchoverReady waits, sharing timeout across all controller
// cards (with at least minPerComponentWait per controller card), until every
// controller card in names reports /components/component/state/switchover-ready
// true. Each controller card is watched with a single-leaf watch that tolerates
// gNMI disconnects. It returns the controller cards that did not become
// switchover-ready and whether all of them became switchover-ready.
func AwaitComponentsSwitchoverReady(t *testing.T, dut *ondatra.DUTDevice, names []string, timeout time.Duration) ([]string, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var notReady []string
	for _, name := range names {
		if _, ok := WatchWithRetry(t, dut, gnmi.OC().Component(name).SwitchoverReady().State(), waitUntil(deadline), switchoverWatchRetryInterval, func(v *ygnmi.Value[bool]) bool {
			ready, present := v.Val()
			return present && ready
		}); !ok {
			notReady = append(notReady, name)
		}
	}
	return notReady, len(notReady) == 0
}
