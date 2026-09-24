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
		// Use a robust polling loop with bound contexts to prevent zombie Subscribe hangs
		// when the control plane is actively flapping.
		start := time.Now()
		var role oc.E_Platform_ComponentRedundantRole

		c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
		if err != nil {
			t.Fatalf("Failed to create ygnmi client for FindStandbyControllerCard: %v", err)
		}

		for time.Since(start) < 10*time.Minute {
			var opts []ygnmi.Option
			// Use discrete Get probes for everything except Juniper, which strictly requires Subscribe
			if dut.Vendor() != ondatra.JUNIPER {
				opts = append(opts, ygnmi.WithUseGet())
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			val, err := ygnmi.Lookup(ctx, c, gnmi.OC().Component(supervisor).RedundantRole().State(), opts...)
			cancel()
			if err == nil {
				if r, present := val.Val(); present && r !=
					oc.Platform_ComponentRedundantRole_UNSET {
					role = r
					break
				}
			}
			time.Sleep(10 * time.Second)
		}

		if role == oc.Platform_ComponentRedundantRole_UNSET {
			t.Fatalf("DUT did not reach target state within %v for %v", 10*time.Minute, supervisor)
		}

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
func WatchWithRetry[T any](t *testing.T, dut *ondatra.DUTDevice, query ygnmi.SingletonQuery[T], timeout, retryInterval time.Duration, predicate func(*ygnmi.Value[T]) bool) (*ygnmi.Value[T], bool) {
	t.Helper()
	for remaining := timeout; remaining > 0; remaining -= retryInterval {
		waitDur := remaining
		if waitDur > retryInterval {
			waitDur = retryInterval
		}
		var val *ygnmi.Value[T]
		var ok bool
		testt.CaptureFatal(t, func(t testing.TB) {
			val, ok = gnmi.Watch(t, dut, query, waitDur, predicate).Await(t)
		})
		if ok {
			return val, true
		}
	}
	return nil, false
}

// WaitForSwitchover verifies the stability of the SUP cards after switchover by watching the system datetime.
func WaitForSwitchover(t *testing.T, dut *ondatra.DUTDevice, maxSwitchoverTime time.Duration) {
	t.Helper()
	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by watching the system datetime telemetry.")

	val, ok := WatchWithRetry(t, dut, gnmi.OC().System().CurrentDatetime().State(), maxSwitchoverTime, 30*time.Second, func(val *ygnmi.Value[string]) bool {
		currentTime, present := val.Val()
		return present && currentTime != ""
	})
	if !ok {
		t.Fatalf("RP switchover timed out after %v: system datetime not reachable", maxSwitchoverTime)
	}
	currentTime, _ := val.Val()
	t.Logf("RP switchover has completed successfully in %.2f seconds with received time: %v", time.Since(startSwitchover).Seconds(), currentTime)
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

// SetControllerCardPowerState (gNOI-3.3.2 Step 2 / Step 3) updates the controller card
// power-admin-state (POWER_DISABLED / POWER_ENABLED) and verifies state and oper-status transition.
func SetControllerCardPowerState(t *testing.T, dut *ondatra.DUTDevice, cardName string, powerType oc.E_Platform_ComponentPowerType, timeout time.Duration) {
	t.Helper()
	c := gnmi.OC().Component(cardName)
	if deviations.PowerDisableEnableLeafRefValidation(dut) {
		t.Logf("WARNING: [PowerAdminState] Deviation PowerDisableEnableLeafRefValidation is enabled for %s (%s): pre-creating /components/component[name=%s]/config leafref before updating power-admin-state.", dut.Name(), dut.Model(), cardName)
		gnmi.Update(t, dut, c.Config(), &oc.Component{
			Name: ygot.String(cardName),
		})
	}
	start := time.Now()
	t.Logf("INFO: [PowerAdminState] Setting %s power-admin-state to %v...", cardName, powerType)
	gnmi.Replace(t, dut, c.ControllerCard().PowerAdminState().Config(), powerType)

	var opts []ygnmi.Option
	if deviations.SwitchoverSubscribeUnsupported(dut) {
		t.Logf("WARNING: [PowerAdminState] Deviation SwitchoverSubscribeUnsupported on %s (%s): gNMI Subscribe streams drop or stall during supervisor power toggles. Bypassing gNMI Subscribe stream via unary gNMI.Get (ygnmi.WithUseGet()) polling to prevent test/DUT hang.", dut.Name(), dut.Model())
		opts = append(opts, ygnmi.WithUseGet())
	}
	yc, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
	if err != nil {
		t.Fatalf("Failed to create ygnmi client in SetControllerCardPowerState: %v", err)
	}

	powerMatched := false
	for time.Since(start) < timeout {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		val, err := ygnmi.Lookup(ctx, yc, c.ControllerCard().PowerAdminState().State(), opts...)
		cancel()
		if err != nil && len(opts) > 0 && (strings.Contains(err.Error(), "Unimplemented") || strings.Contains(err.Error(), "Unsupported 'type'")) {
			t.Logf("INFO: [PowerAdminState] Device %s (%s) does not support unary gNMI.Get for STATE paths; switching polling to one-shot gNMI.Subscribe (mode: ONCE) via ygnmi.Lookup.", dut.Name(), dut.Model())
			opts = nil
			ctxRetry, cancelRetry := context.WithTimeout(context.Background(), 10*time.Second)
			val, err = ygnmi.Lookup(ctxRetry, yc, c.ControllerCard().PowerAdminState().State())
			cancelRetry()
		}
		if err == nil {
			if power, present := val.Val(); present && power == powerType {
				powerMatched = true
				t.Logf("Component %s, power-admin-state reached %v after %.2f minutes", cardName, power, time.Since(start).Minutes())
				break
			}
		}
		time.Sleep(10 * time.Second)
	}
	if !powerMatched {
		t.Errorf("Component %s, power-admin-state did not reach %v within %v", cardName, powerType, timeout)
	}

	wantOper := oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED
	if powerType == oc.Platform_ComponentPowerType_POWER_ENABLED {
		wantOper = oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE
	}

	startOper := time.Now()
	operMatched := false
	for time.Since(startOper) < timeout {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		val, err := ygnmi.Lookup(ctx, yc, c.OperStatus().State(), opts...)
		cancel()
		if err != nil && len(opts) > 0 && (strings.Contains(err.Error(), "Unimplemented") || strings.Contains(err.Error(), "Unsupported 'type'")) {
			t.Logf("INFO: [PowerAdminState] Device %s (%s) does not support unary gNMI.Get for STATE paths; switching polling to one-shot gNMI.Subscribe (mode: ONCE) via ygnmi.Lookup.", dut.Name(), dut.Model())
			opts = nil
			ctxRetry, cancelRetry := context.WithTimeout(context.Background(), 10*time.Second)
			val, err = ygnmi.Lookup(ctxRetry, yc, c.OperStatus().State())
			cancelRetry()
		}
		if err == nil {
			if oper, present := val.Val(); present && oper == wantOper {
				operMatched = true
				t.Logf("Component %s, oper-status reached %v after %.2f minutes", cardName, oper, time.Since(startOper).Minutes())
				break
			}
		}
		time.Sleep(10 * time.Second)
	}
	if !operMatched {
		t.Errorf("Component %s oper-status did not reach %v within %v", cardName, wantOper, timeout)
	}
}
