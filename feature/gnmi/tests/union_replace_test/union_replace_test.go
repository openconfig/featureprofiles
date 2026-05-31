package union_replace_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ocOrigin     = "openconfig"
	cliOrigin    = "cli"
	defaultSubif = uint32(0)

	port1         = "port1"
	port2         = "port2"
	port1IPv4     = "192.0.2.1"
	addedIPv4     = "192.0.2.5"
	ipv4PrefixLen = uint8(30)

	addedMTU      = uint16(9000)
	overlapMTUOC  = uint16(9000)
	overlapMTUCLI = uint16(1500)
	nonOverlapMTU = uint16(8000)
	moveIPMTU     = uint16(1500)

	descBaselineP1   = "baseline-p1"
	descBaselineP2   = "baseline-p2"
	descAddOC        = "gNMI-3.2.1-OC-added"
	descAddCLI       = "gNMI-3.2.2-CLI-added"
	descInitialOC    = "initial-desc"
	descChangedOC    = "gNMI-3.3.1-changed"
	descInitialCLI   = "initial-desc-cli"
	descChangedCLI   = "gNMI-3.3.2-changed"
	descIntf1Present = "intf1-present"
	descIntf2Present = "intf2-present"
	descCLIIntf1     = "cli-intf1"
	descCLIIntf2     = "cli-intf2"
	descMoveHasIP    = "has-ip"
	descMoveNoIP     = "no-ip-yet"
	descMoveIPMoved  = "ip-moved-away"
	descMoveHasIPNow = "has-ip-now"
	descOverlapTest  = "overlap-test"
	descOverlapSame  = "overlap-same"
	descOCDescP1     = "oc-desc-p1"
	descCLIDescP2    = "cli-desc-p2"

	bgpProtocolName = "BGP"
	bgpASOC         = uint32(65100)
	bgpASCLI        = uint32(65200)
	rejectedPeerAS  = uint32(65001)
	badPolicyOC     = "NON_EXISTENT_POLICY"
	badPolicyCLI    = "NON_EXISTENT_POLICY_CLI"
	badQueueName    = "NON_EXISTENT_QUEUE_999"
	schedulerName   = "NEW_SCHEDULER"
	badNeighborOC   = "198.51.100.1"
	badNeighborCLI  = "198.51.100.2"
	policyName      = "OVERLAP_POLICY_1"

	portSpeed50GCLI    = "50g"
	portSpeedBreakout  = "50g-2"
	breakoutNumGroups  = uint8(2)
	breakoutNumChannel = uint8(2)

	awaitStateTimeout = 60 * time.Second
	awaitMTUTimeout   = 10 * time.Second
)

var (
	sharedBaseline  string
	connectivityCLI string
	defaultNI       string
)

type cliInterfaceConfigOpts struct {
	Name          string
	Description   string
	MTU           uint16
	IPv4          string
	IPv4PrefixLen uint8
	Speed         string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func firstInterfaceWithoutTransceiver(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()

	allInterfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	for _, intf := range allInterfaces {
		name := intf.GetName()
		if !strings.HasPrefix(name, "Ethernet") {
			continue
		}
		if intf.GetOperStatus() == oc.Interface_OperStatus_NOT_PRESENT && len(intf.GetPhysicalChannel()) == 0 {
			return name
		}
	}

	t.Fatalf("unable to find an Ethernet interface with no transceiver connected")
	return ""
}

func baselineCLIConfig(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	showCmd := cliShowRunningConfigCommand(t, dut)
	req := &gpb.GetRequest{
		Path: []*gpb.Path{{
			Origin: cliOrigin,
			Elem:   []*gpb.PathElem{{Name: showCmd}},
		}},
		Encoding: gpb.Encoding_ASCII,
	}
	resp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
	if err != nil {
		t.Fatalf("baselineCLIConfig: gNMI Get CLI config: %v", err)
	}
	for _, notif := range resp.Notification {
		for _, update := range notif.Update {
			if s := update.Val.GetAsciiVal(); s != "" {
				return s
			}
		}
	}
	t.Fatalf("baselineCLIConfig: no ASCII config in gNMI Get response")
	return ""
}

func cliShowRunningConfigCommand(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return "show running-config"
	default:
		t.Fatalf("unsupported vendor %v for CLI show running-config command", dut.Vendor())
		return ""
	}
}

func extractConnectivityCLI(t *testing.T, dut *ondatra.DUTDevice, fullCLI string) string {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return extractNonEthernetCLI(fullCLI)
	default:
		return fullCLI
	}
}

func extractNonEthernetCLI(fullCLI string) string {
	var sb strings.Builder
	skip := false
	for _, line := range strings.Split(fullCLI, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "!" {
			skip = false
			sb.WriteString(line)
			sb.WriteByte('\n')
			continue
		}
		if strings.HasPrefix(line, "interface ") {
			rest := strings.TrimPrefix(line, "interface ")
			if strings.HasPrefix(rest, "Ethernet") || strings.HasPrefix(rest, "Port-Channel") {
				skip = true
				continue
			}
			skip = false
		}
		if skip {
			continue
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

func unionReplace(t *testing.T, dut *ondatra.DUTDevice, updates ...*gpb.Update) error {
	t.Helper()

	if len(updates) == 0 {
		return nil
	}
	updates = append(updates, cliUpdate(t, connectivityCLI))
	_, err := dut.RawAPIs().GNMI(t).Set(context.Background(), &gpb.SetRequest{UnionReplace: updates})
	return err
}

func cliUpdate(t *testing.T, cli string) *gpb.Update {
	t.Helper()
	return &gpb.Update{
		Path: &gpb.Path{Origin: cliOrigin},
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_AsciiVal{AsciiVal: cli}},
	}
}

func ocUpdate(t *testing.T, ocCfg *oc.Root) *gpb.Update {
	t.Helper()
	jsonBytes, err := ygot.Marshal7951(ocCfg, &ygot.RFC7951JSONConfig{
		AppendModuleName: true,
		PreferShadowPath: true,
	})
	if err != nil {
		t.Fatalf("ygot.Marshal7951: %v", err)
	}
	return &gpb.Update{
		Path: &gpb.Path{Origin: ocOrigin},
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: jsonBytes}},
	}
}

func configureOCInterface(t *testing.T, root *oc.Root, dut *ondatra.DUTDevice, intfName, desc string, mtu uint16, ipv4 string, ipv4Len uint8) *oc.Interface {
	intf := root.GetOrCreateInterface(intfName)
	intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		intf.Enabled = ygot.Bool(true)
	}
	if desc != "" {
		intf.Description = ygot.String(desc)
	}
	if mtu > 0 {
		intf.Mtu = ygot.Uint16(mtu)
	}
	if ipv4 != "" {
		subif := defaultSubif
		s := intf.GetOrCreateSubinterface(subif)
		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		a4 := s4.GetOrCreateAddress(ipv4)
		if ipv4Len > 0 {
			a4.PrefixLength = ygot.Uint8(ipv4Len)
		}
	}
	return intf
}

func cliInterface(cli, intfName string) string {
	var sb strings.Builder
	insideInterface := false
	for _, line := range strings.Split(cli, "\n") {
		if line == "interface "+intfName {
			insideInterface = true
		}
		if insideInterface {
			sb.WriteString(line)
			sb.WriteByte('\n')
			if strings.TrimSpace(line) == "!" {
				break
			}
		}
	}
	return sb.String()
}

func verifyInterfaceDescription(t *testing.T, dut *ondatra.DUTDevice, intfName, wantDesc string) error {
	t.Helper()
	got, ok := gnmi.Await(t, dut, gnmi.OC().Interface(intfName).Description().State(), awaitStateTimeout, wantDesc).Val()
	if !ok {
		return fmt.Errorf("interface %s description: got %q, want %q", intfName, got, wantDesc)
	}
	return nil
}

func verifyInterfaceIP(t *testing.T, dut *ondatra.DUTDevice, intfName, wantIPv4 string, wantLen uint8) error {
	t.Helper()
	got, ok := gnmi.Await(t, dut, gnmi.OC().Interface(intfName).Subinterface(defaultSubif).Ipv4().Address(wantIPv4).PrefixLength().State(), awaitStateTimeout, wantLen).Val()
	if !ok {
		return fmt.Errorf("interface %s IPv4 %s prefix-length: got %v, want %v", intfName, wantIPv4, got, wantLen)
	}
	return nil
}

func verifyInterfaceNoIP(t *testing.T, dut *ondatra.DUTDevice, intfName, ipv4 string) error {
	t.Helper()
	v := gnmi.Lookup(t, dut, gnmi.OC().Interface(intfName).Subinterface(defaultSubif).Ipv4().Address(ipv4).PrefixLength().State())
	if v.IsPresent() {
		got, _ := v.Val()
		return fmt.Errorf("interface %s IPv4 %s should not be present, got prefix-length %v", intfName, ipv4, got)
	}
	return nil
}

func verifyInterfaceMTU(t *testing.T, dut *ondatra.DUTDevice, intfName string, wantMTU uint16) error {
	t.Helper()
	configPath := gnmi.OC().Interface(intfName).Mtu().Config()
	if got, ok := gnmi.Await(t, dut, configPath, awaitMTUTimeout, wantMTU).Val(); ok {
		t.Logf("Interface %s MTU config verified: %v", intfName, got)
		return nil
	}
	if verifyCLIMTU(t, dut, intfName, wantMTU) {
		return nil
	}
	got, ok := gnmi.Await(t, dut, gnmi.OC().Interface(intfName).Mtu().State(), awaitMTUTimeout, wantMTU).Val()
	if !ok {
		interfaceConfig := cliInterface(baselineCLIConfig(t, dut), intfName)
		return fmt.Errorf("interface %s MTU: got %v, want %v; CLI config:\n%s", intfName, got, wantMTU, interfaceConfig)
	}
	return nil
}

func verifyCLIMTU(t *testing.T, dut *ondatra.DUTDevice, intfName string, wantMTU uint16) bool {
	t.Helper()
	interfaceConfig := cliInterface(baselineCLIConfig(t, dut), intfName)
	if strings.Contains(interfaceConfig, fmt.Sprintf("mtu %d", wantMTU)) {
		t.Logf("Interface %s MTU verified in CLI running-config", intfName)
		return true
	}
	return false
}

func verifyInterfaceNotPresent(t *testing.T, dut *ondatra.DUTDevice, intfName string) error {
	t.Helper()
	v := gnmi.Lookup(t, dut, gnmi.OC().Interface(intfName).Description().State())
	if v.IsPresent() {
		val, _ := v.Val()
		if val != "" {
			return fmt.Errorf("interface %s should not have description after deletion, got %q", intfName, val)
		}
	}
	return nil
}

func verifyPortSpeed(t *testing.T, dut *ondatra.DUTDevice, intfName string, wantSpeed oc.E_IfEthernet_ETHERNET_SPEED) error {
	t.Helper()
	got := gnmi.Get(t, dut, gnmi.OC().Interface(intfName).Ethernet().PortSpeed().Config())
	if got != wantSpeed {
		return fmt.Errorf("interface %s port-speed config: got %v, want %v", intfName, got, wantSpeed)
	}
	return nil
}

func verifyInterfaceOperStatus(t *testing.T, dut *ondatra.DUTDevice, intfName string, wantStatus oc.E_Interface_OperStatus) error {
	t.Helper()
	got, ok := gnmi.Await(t, dut, gnmi.OC().Interface(intfName).OperStatus().State(), awaitStateTimeout, wantStatus).Val()
	if !ok {
		return fmt.Errorf("interface %s oper-status: got %v, want %v", intfName, got, wantStatus)
	}
	return nil
}

func cliInterfaceConfig(t *testing.T, dut *ondatra.DUTDevice, opts cliInterfaceConfigOpts) string {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var sb strings.Builder
		fmt.Fprintf(&sb, "interface %s\n", opts.Name)
		if opts.Description != "" {
			fmt.Fprintf(&sb, "  description %s\n", opts.Description)
		}
		if opts.MTU > 0 {
			fmt.Fprintf(&sb, "  mtu %d\n", opts.MTU)
		}
		if opts.IPv4 != "" && opts.IPv4PrefixLen > 0 {
			fmt.Fprintf(&sb, "  no switchport\n  ip address %s/%d\n", opts.IPv4, opts.IPv4PrefixLen)
		}
		if opts.Speed != "" {
			fmt.Fprintf(&sb, "  speed %s\n", opts.Speed)
		}
		return sb.String()
	default:
		t.Fatalf("unsupported vendor %v for CLI interface config", dut.Vendor())
		return ""
	}
}

func ocBreakoutMode(t *testing.T, dut *ondatra.DUTDevice, root *oc.Root, intfName string) {
	t.Helper()
	hwPort, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(intfName).HardwarePort().State()).Val()
	if !ok || hwPort == "" {
		t.Logf("Skipping breakout-mode OC config: no hardware-port for %s", intfName)
		return
	}
	comp := root.GetOrCreateComponent(hwPort)
	grp := comp.GetOrCreatePort().GetOrCreateBreakoutMode().GetOrCreateGroup(1)
	grp.Index = ygot.Uint8(1)
	grp.NumBreakouts = ygot.Uint8(breakoutNumGroups)
	grp.BreakoutSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB
	grp.NumPhysicalChannels = ygot.Uint8(breakoutNumChannel)
}

func cliBreakoutMode(t *testing.T, dut *ondatra.DUTDevice, intfName string) string {
	return cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intfName, Speed: portSpeedBreakout})
}

func configureUnionReplaceSupport(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		helpers.GnmiCLIConfig(t, dut, "management api gnmi\n   operation set namespace\n")
	}
}

func TestUnionReplace(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	defaultNI = deviations.DefaultNetworkInstance(dut)
	configureUnionReplaceSupport(t, dut)
	intf1Name := dut.Port(t, port1).Name()
	intf2Name := dut.Port(t, port2).Name()

	sharedBaseline = baselineCLIConfig(t, dut)
	connectivityCLI = extractConnectivityCLI(t, dut, sharedBaseline)
	interfaceWithoutTransceiver := firstInterfaceWithoutTransceiver(t, dut)
	t.Logf("First interface without transceiver: %s", interfaceWithoutTransceiver)
	resetConfig := func() {
		t.Log("Resetting baseline configuration")
		if err := unionReplace(t, dut, cliUpdate(t, sharedBaseline)); err != nil {
			t.Fatalf("Failed to reset baseline configuration: %v", err)
		}
	}
	testCases := []struct {
		name string
		desc string
		fn   func(t *testing.T) error
	}{
		{
			name: "gNMI-3.1-IdempotentConfiguration",
			desc: "Verify the same configuration already on the device can be pushed and accepted without changing the configuration.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				ocDelta := &oc.Root{}
				ocDelta.GetOrCreateInterface(intf1Name).Description = ygot.String(descBaselineP1)
				ocDelta.GetOrCreateInterface(intf2Name).Description = ygot.String(descBaselineP2)

				t.Log("Push baseline configuration via union_replace")
				if err := unionReplace(t, dut, ocUpdate(t, ocDelta)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Get configuration and verify it matches baseline")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descBaselineP1))
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descBaselineP2))

				t.Log("Push the same configuration again")
				if err := unionReplace(t, dut, ocUpdate(t, ocDelta)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Get configuration A.1 again and verify unchanged")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descBaselineP1))
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descBaselineP2))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.2.1-AddConfigurationOC",
			desc: "Add an interface configuration using OC to the baseline configuration.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Generate a new interface configuration using OC")
				ocDelta := &oc.Root{}
				configureOCInterface(t, ocDelta, dut, intf1Name, descAddOC, addedMTU, port1IPv4, ipv4PrefixLen)
				t.Log("Push configuration via union_replace")
				if err := unionReplace(t, dut, ocUpdate(t, ocDelta)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Get configuration and verify it matches the expected values")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descAddOC))
				errs = append(errs, verifyInterfaceIP(t, dut, intf1Name, port1IPv4, ipv4PrefixLen))
				errs = append(errs, verifyInterfaceMTU(t, dut, intf1Name, addedMTU))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.2.2-AddConfigurationCLI",
			desc: "Add an interface configuration using CLI to the baseline configuration.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Generate a new interface configuration using CLI")
				extraCLI := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Description: descAddCLI, MTU: addedMTU, IPv4: addedIPv4, IPv4PrefixLen: ipv4PrefixLen})
				t.Log("Push configuration + CLI via union_replace")
				if err := unionReplace(t, dut, cliUpdate(t, extraCLI)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Get configuration and verify the CLI addition is present")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descAddCLI))
				errs = append(errs, verifyInterfaceIP(t, dut, intf1Name, addedIPv4, ipv4PrefixLen))
				if !verifyCLIMTU(t, dut, intf1Name, addedMTU) {
					errs = append(errs, verifyInterfaceMTU(t, dut, intf1Name, addedMTU))
				}
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.3.1-ChangeConfigurationOC",
			desc: "Change the interface description using OC via union_replace.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				initOC := &oc.Root{}
				initOC.GetOrCreateInterface(intf1Name).Description = ygot.String(descInitialOC)
				if err := unionReplace(t, dut, ocUpdate(t, initOC)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descInitialOC))

				t.Log("Change the description using OC")
				ocDelta := &oc.Root{}
				ocDelta.GetOrCreateInterface(intf1Name).Description = ygot.String(descChangedOC)

				t.Log("Push OC delta via union_replace")
				if err := unionReplace(t, dut, ocUpdate(t, ocDelta)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Get configuration and verify it matches the expected values")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descChangedOC))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.3.2-ChangeConfigurationCLI",
			desc: "Change the interface description using CLI via union_replace.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				initialCLI := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Description: descInitialCLI})
				if err := unionReplace(t, dut, cliUpdate(t, initialCLI)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descInitialCLI))

				t.Log("Change description using CLI")
				changedCLI := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Description: descChangedCLI})
				t.Log("Push baseline + changed CLI via union_replace")
				if err := unionReplace(t, dut, cliUpdate(t, changedCLI)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Verify CLI description change is applied")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descChangedCLI))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.4.1-DeleteByOmissionOC",
			desc: "Remove an interface by omitting it in OC via union_replace.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Add both interfaces via OC union_replace")
				bothOC := &oc.Root{}
				bothOC.GetOrCreateInterface(intf1Name).Description = ygot.String(descIntf1Present)
				bothOC.GetOrCreateInterface(intf2Name).Description = ygot.String(descIntf2Present)
				if err := unionReplace(t, dut, ocUpdate(t, bothOC)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descIntf1Present))
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descIntf2Present))

				t.Log("Omit interface 2 in OC union_replace")
				intf1Only := &oc.Root{}
				intf1Only.GetOrCreateInterface(intf1Name).Description = ygot.String(descIntf1Present)
				if err := unionReplace(t, dut, ocUpdate(t, intf1Only)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Verify interface 2 configuration is removed")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descIntf1Present))
				errs = append(errs, verifyInterfaceNotPresent(t, dut, intf2Name))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.4.2-DeleteByOmissionCLI",
			desc: "Remove an interface by omitting it in CLI via union_replace.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Add both interfaces via CLI union_replace")
				cli1And2 := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Description: descCLIIntf1}) + "\n" +
					cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf2Name, Description: descCLIIntf2})
				if err := unionReplace(t, dut, cliUpdate(t, cli1And2)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descCLIIntf1))
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descCLIIntf2))

				t.Log("Omit interface 2 in CLI union_replace")
				cli1Only := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Description: descCLIIntf1})
				if err := unionReplace(t, dut, cliUpdate(t, cli1Only)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Verify interface 2 configuration is removed")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descCLIIntf1))
				errs = append(errs, verifyInterfaceNotPresent(t, dut, intf2Name))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.5.1-MoveIPBetweenInterfacesOC",
			desc: "Move an IP address from interface 1 to interface 2 using OC via union_replace.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Configure IP on port1, no IP on port2")
				ocConfig := &oc.Root{}
				configureOCInterface(t, ocConfig, dut, intf1Name, descMoveHasIP, 0, port1IPv4, ipv4PrefixLen)
				ocConfig.GetOrCreateInterface(intf2Name).Description = ygot.String(descMoveNoIP)
				if err := unionReplace(t, dut, ocUpdate(t, ocConfig)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}
				errs = append(errs, verifyInterfaceIP(t, dut, intf1Name, port1IPv4, ipv4PrefixLen))

				t.Log("Move IP from port1 to port2")
				ocConfig = &oc.Root{}
				ocConfig.GetOrCreateInterface(intf1Name).Description = ygot.String(descMoveIPMoved)
				configureOCInterface(t, ocConfig, dut, intf2Name, descMoveHasIPNow, 0, port1IPv4, ipv4PrefixLen)
				if err := unionReplace(t, dut, ocUpdate(t, ocConfig)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Verify IP is now on port2")
				errs = append(errs, verifyInterfaceNoIP(t, dut, intf1Name, port1IPv4))
				errs = append(errs, verifyInterfaceIP(t, dut, intf2Name, port1IPv4, ipv4PrefixLen))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.5.2-MoveIPBetweenInterfacesCLI",
			desc: "Move an IP address from interface 1 to interface 2 using CLI via union_replace.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Configure IP on port1 via CLI")
				cli1 := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Description: descMoveHasIP, MTU: moveIPMTU, IPv4: port1IPv4, IPv4PrefixLen: ipv4PrefixLen})
				if err := unionReplace(t, dut, cliUpdate(t, cli1)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Move IP to port2 via CLI, omitting port1")
				cli2 := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf2Name, Description: descMoveHasIPNow, MTU: moveIPMTU, IPv4: port1IPv4, IPv4PrefixLen: ipv4PrefixLen})
				if err := unionReplace(t, dut, cliUpdate(t, cli2)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Verify IP is now on port2")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descMoveHasIPNow))
				errs = append(errs, verifyInterfaceNoIP(t, dut, intf1Name, port1IPv4))
				errs = append(errs, verifyInterfaceIP(t, dut, intf2Name, port1IPv4, ipv4PrefixLen))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.6.1-HardwareMismatchOC",
			desc: "Verify configuration with OC hardware mismatch (wrong port-speed) is accepted.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Generate OC delta with a port-speed mismatch")
				ocDelta := &oc.Root{}
				intf := ocDelta.GetOrCreateInterface(intf1Name)
				intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
				eth := intf.GetOrCreateEthernet()
				eth.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB
				eth.DuplexMode = oc.Ethernet_DuplexMode_FULL
				eth.AutoNegotiate = ygot.Bool(false)

				t.Log("Push configuration via union_replace")
				if err := unionReplace(t, dut, ocUpdate(t, ocDelta)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Verify accepted and values match expected")
				errs = append(errs, verifyPortSpeed(t, dut, intf1Name, oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB))
				errs = append(errs, verifyInterfaceOperStatus(t, dut, intf1Name, oc.Interface_OperStatus_DOWN))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.6.2-HardwareMismatchCLI",
			desc: "Verify configuration with CLI hardware mismatch is accepted.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Generate CLI configuration with port-speed mismatch")
				cli := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Speed: portSpeed50GCLI})
				t.Log("Push configuration via union_replace")
				if err := unionReplace(t, dut, cliUpdate(t, cli)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}

				t.Log("Verify accepted and values match expected")
				errs = append(errs, verifyPortSpeed(t, dut, intf1Name, oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB))
				errs = append(errs, verifyInterfaceOperStatus(t, dut, intf1Name, oc.Interface_OperStatus_DOWN))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.7-RejectedInvalidConfigOC",
			desc: "Verify a DUT rejects and rolls back a union_replace with invalid configuration.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")

				t.Run("gNMI-3.7.1-LeafrefValidationError", func(t *testing.T) {
					var errs []error
					t.Log("Build OC delta with non-existent BGP import policy")
					ocDelta := &oc.Root{}
					ni := ocDelta.GetOrCreateNetworkInstance(defaultNI)
					ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
					protocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName)
					protocol.Enabled = ygot.Bool(true)
					bgp := protocol.GetOrCreateBgp()
					bgp.GetOrCreateGlobal().As = ygot.Uint32(bgpASOC)
					neighbor := bgp.GetOrCreateNeighbor(badNeighborOC)
					neighbor.PeerAs = ygot.Uint32(rejectedPeerAS)
					afi := neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
					afi.Enabled = ygot.Bool(true)
					afi.GetOrCreateApplyPolicy().ImportPolicy = []string{badPolicyOC}

					t.Log("Push configuration via union_replace")
					err := unionReplace(t, dut, ocUpdate(t, ocDelta))

					t.Log("Confirm DUT rejects the gnmi.Set")
					if err == nil {
						errs = append(errs, fmt.Errorf("expected gnmi.Set to be rejected with invalid policy reference, but it succeeded"))
					} else {
						t.Logf("gnmi.Set rejected as expected: %v", err)
						t.Log("Get configuration and verify config unchanged (invalid neighbor not added)")
						if v := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp().Neighbor(badNeighborOC).PeerAs().Config()); v.IsPresent() {
							got, _ := v.Val()
							errs = append(errs, fmt.Errorf("expected neighbor %s not to be configured after rollback, got peer-as %d", badNeighborOC, got))
						}
					}
					if err := errors.Join(errs...); err != nil {
						t.Errorf("%v", err)
					}
				})

				t.Run("gNMI-3.7.2-SemanticErrorInOC", func(t *testing.T) {
					var errs []error
					t.Log("Build OC delta referencing a non-existent QoS scheduler-policy")
					ocDelta := &oc.Root{}
					qos := ocDelta.GetOrCreateQos()
					qos.GetOrCreateSchedulerPolicy(schedulerName).GetOrCreateScheduler(0).GetOrCreateInput("0").Queue = ygot.String(badQueueName)
					qi := qos.GetOrCreateInterface(intf1Name)
					qi.InterfaceId = ygot.String(intf1Name)
					qi.GetOrCreateOutput().GetOrCreateQueue(badQueueName)

					t.Log("Push via union_replace")
					err := unionReplace(t, dut, ocUpdate(t, ocDelta))

					t.Log("Confirm DUT rejects the gnmi.Set")
					if err == nil {
						errs = append(errs, fmt.Errorf("expected gnmi.Set to be rejected with non-existent scheduler-policy reference %q, but it succeeded", badQueueName))
					} else {
						t.Logf("gnmi.Set rejected as expected: %v", err)
						t.Log("Verify config unchanged (invalid scheduler-policy not applied)")
						if v := gnmi.LookupConfig(t, dut, gnmi.OC().Qos().Interface(intf1Name).Output().SchedulerPolicy().Name().Config()); v.IsPresent() {
							got, _ := v.Val()
							errs = append(errs, fmt.Errorf("output scheduler-policy name should not be present after rollback, got %q", got))
						}
					}
					if err := errors.Join(errs...); err != nil {
						t.Errorf("%v", err)
					}
				})

				t.Run("gNMI-3.7.3-ReferenceErrorInCLI", func(t *testing.T) {
					var errs []error
					t.Log("Get configuration from DUT")
					t.Log("Generate CLI config referencing a non-existent BGP import policy")
					var badCLI string
					switch dut.Vendor() {
					case ondatra.ARISTA:
						badCLI = fmt.Sprintf("router bgp 65000\n  neighbor %s remote-as %d\n  address-family ipv4\n    neighbor %s route-map %s in\n", badNeighborCLI, rejectedPeerAS, badNeighborCLI, badPolicyCLI)
					default:
						t.Fatalf("CLI error test not implemented for vendor %v", dut.Vendor())
					}

					t.Log("Push via union_replace with bad CLI (no OC)")
					err := unionReplace(t, dut, cliUpdate(t, badCLI))

					t.Log("Confirm DUT rejects the gnmi.Set")
					if err == nil {
						errs = append(errs, fmt.Errorf("expected gnmi.Set to be rejected with invalid CLI policy reference, but it succeeded"))
					} else {
						t.Logf("gnmi.Set rejected as expected: %v", err)
						t.Log("Get configuration and verify config unchanged (invalid CLI policy not added)")
						if got := gnmi.LookupConfig(t, dut, gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp().Neighbor(badNeighborCLI).Config()); got.IsPresent() {
							errs = append(errs, fmt.Errorf("expected CLI-rejected BGP neighbor %s not to be configured after rollback", badNeighborCLI))
						}
					}
					if err := errors.Join(errs...); err != nil {
						t.Errorf("%v", err)
					}
				})
				return nil
			},
		},
		{
			name: "gNMI-3.8-OverlapRejected",
			desc: "Verify union_replace is rejected (option 1) or accepted (option 2) when CLI and OC configure the same leaf.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")

				t.Run("gNMI-3.8.1-DifferentMTUValues", func(t *testing.T) {
					var errs []error
					t.Log("Get configuration from DUT")
					configPath := gnmi.OC().Interface(intf1Name).Mtu().Config()
					statePath := gnmi.OC().Interface(intf1Name).Mtu().State()
					beforeConfig, beforeConfigPresent := gnmi.Lookup(t, dut, configPath).Val()
					beforeState, beforeStatePresent := gnmi.Lookup(t, dut, statePath).Val()

					t.Logf("Set MTU=%d in OC and MTU=%d in CLI (conflict)", overlapMTUOC, overlapMTUCLI)
					ocDelta := &oc.Root{}
					ocDelta.GetOrCreateInterface(intf1Name).Mtu = ygot.Uint16(overlapMTUOC)
					cli := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Description: descOverlapTest, MTU: overlapMTUCLI, IPv4: port1IPv4, IPv4PrefixLen: ipv4PrefixLen})

					t.Log("Push via union_replace")
					err := unionReplace(t, dut, ocUpdate(t, ocDelta), cliUpdate(t, cli))

					if err != nil {
						if s, ok := status.FromError(err); ok && s.Code() == codes.InvalidArgument {
							t.Logf("Option 1 behavior: DUT rejected overlap with INVALID_ARGUMENT: %v", s.Message())
							afterConfig, afterConfigPresent := gnmi.Lookup(t, dut, configPath).Val()
							afterState, afterStatePresent := gnmi.Lookup(t, dut, statePath).Val()
							if afterConfigPresent != beforeConfigPresent || (afterConfigPresent && afterConfig != beforeConfig) {
								errs = append(errs, fmt.Errorf("MTU config changed after rejected overlap: got (%v, present=%v), want (%v, present=%v)", afterConfig, afterConfigPresent, beforeConfig, beforeConfigPresent))
							}
							if afterStatePresent != beforeStatePresent || (afterStatePresent && afterState != beforeState) {
								errs = append(errs, fmt.Errorf("MTU state changed after rejected overlap: got (%v, present=%v), want (%v, present=%v)", afterState, afterStatePresent, beforeState, beforeStatePresent))
							}
						} else {
							errs = append(errs, fmt.Errorf("gnmi.Set failed with unexpected error: %w", err))
						}
					} else {
						t.Log("Option 2 behavior: DUT accepted overlapping config, CLI value should take effect in state")
						gotConfig := gnmi.Get(t, dut, configPath)
						if gotConfig != overlapMTUOC {
							errs = append(errs, fmt.Errorf("MTU config: got %d, want %d", gotConfig, overlapMTUOC))
						}
						if !verifyCLIMTU(t, dut, intf1Name, overlapMTUCLI) {
							gotState, ok := gnmi.Await(t, dut, statePath, awaitMTUTimeout, overlapMTUCLI).Val()
							if !ok {
								errs = append(errs, fmt.Errorf("MTU state: got %d, want %d (CLI value)", gotState, overlapMTUCLI))
							}
						}
					}
					if err := errors.Join(errs...); err != nil {
						t.Errorf("%v", err)
					}
				})

				t.Run("gNMI-3.8.2-SameMTUValues", func(t *testing.T) {
					var errs []error
					t.Log("Get configuration from DUT")
					configPath := gnmi.OC().Interface(intf1Name).Mtu().Config()
					statePath := gnmi.OC().Interface(intf1Name).Mtu().State()
					beforeConfig, beforeConfigPresent := gnmi.Lookup(t, dut, configPath).Val()
					beforeState, beforeStatePresent := gnmi.Lookup(t, dut, statePath).Val()

					t.Logf("Set MTU=%d in both OC and CLI (same value overlap)", overlapMTUOC)
					ocDelta := &oc.Root{}
					ocDelta.GetOrCreateInterface(intf1Name).Mtu = ygot.Uint16(overlapMTUOC)
					cli := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Description: descOverlapSame, MTU: overlapMTUOC, IPv4: port1IPv4, IPv4PrefixLen: ipv4PrefixLen})

					t.Log("Push via union_replace")
					err := unionReplace(t, dut, ocUpdate(t, ocDelta), cliUpdate(t, cli))

					if err != nil {
						if s, ok := status.FromError(err); ok && s.Code() == codes.InvalidArgument {
							afterConfig, afterConfigPresent := gnmi.Lookup(t, dut, configPath).Val()
							afterState, afterStatePresent := gnmi.Lookup(t, dut, statePath).Val()
							if afterConfigPresent != beforeConfigPresent || (afterConfigPresent && afterConfig != beforeConfig) {
								errs = append(errs, fmt.Errorf("MTU config changed after rejected same-value overlap: got (%v, present=%v), want (%v, present=%v)", afterConfig, afterConfigPresent, beforeConfig, beforeConfigPresent))
							}
							if afterStatePresent != beforeStatePresent || (afterStatePresent && afterState != beforeState) {
								errs = append(errs, fmt.Errorf("MTU state changed after rejected same-value overlap: got (%v, present=%v), want (%v, present=%v)", afterState, afterStatePresent, beforeState, beforeStatePresent))
							}
						} else {
							errs = append(errs, fmt.Errorf("gnmi.Set failed with unexpected error: %w", err))
						}
					} else {
						t.Log("Option 2 behavior: DUT accepted same-value overlapping config")
						gotConfig := gnmi.Get(t, dut, configPath)
						if gotConfig != overlapMTUOC {
							errs = append(errs, fmt.Errorf("MTU config: got %d, want %d", gotConfig, overlapMTUOC))
						}
						errs = append(errs, verifyInterfaceMTU(t, dut, intf1Name, overlapMTUOC))
					}
					if err := errors.Join(errs...); err != nil {
						t.Errorf("%v", err)
					}
				})

				t.Run("gNMI-3.8.3-BGPModelOverlap", func(t *testing.T) {
					var errs []error
					t.Log("Get configuration from DUT")
					t.Log("Set BGP AS in OC and CLI (overlap)")
					configPath := gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp().Global().As().Config()
					statePath := gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp().Global().As().State()
					beforeConfig, beforeConfigPresent := gnmi.Lookup(t, dut, configPath).Val()
					beforeState, beforeStatePresent := gnmi.Lookup(t, dut, statePath).Val()

					ocDelta := &oc.Root{}
					ni := ocDelta.GetOrCreateNetworkInstance(defaultNI)
					ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
					bgp := ni.
						GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).
						GetOrCreateBgp()
					bgp.GetOrCreateGlobal().As = ygot.Uint32(bgpASOC)

					var bgpCLI string
					switch dut.Vendor() {
					case ondatra.ARISTA:
						bgpCLI = fmt.Sprintf("router bgp %d\n", bgpASCLI)
					default:
						t.Fatalf("BGP overlap test not implemented for vendor %v", dut.Vendor())
					}

					t.Log("Push via union_replace")
					err := unionReplace(t, dut, ocUpdate(t, ocDelta), cliUpdate(t, bgpCLI))

					if err != nil {
						if s, ok := status.FromError(err); ok && s.Code() == codes.InvalidArgument {
							t.Logf("Option 1 behavior: DUT rejected BGP AS overlap with INVALID_ARGUMENT: %v", s.Message())
							afterConfig, afterConfigPresent := gnmi.Lookup(t, dut, configPath).Val()
							afterState, afterStatePresent := gnmi.Lookup(t, dut, statePath).Val()
							if afterConfigPresent != beforeConfigPresent || (afterConfigPresent && afterConfig != beforeConfig) {
								errs = append(errs, fmt.Errorf("BGP AS config changed after rejected overlap: got (%v, present=%v), want (%v, present=%v)", afterConfig, afterConfigPresent, beforeConfig, beforeConfigPresent))
							}
							if afterStatePresent != beforeStatePresent || (afterStatePresent && afterState != beforeState) {
								errs = append(errs, fmt.Errorf("BGP AS state changed after rejected overlap: got (%v, present=%v), want (%v, present=%v)", afterState, afterStatePresent, beforeState, beforeStatePresent))
							}
						} else {
							errs = append(errs, fmt.Errorf("gnmi.Set failed with unexpected error: %w", err))
						}
					} else {
						t.Log("Option 2 behavior: DUT accepted BGP AS overlap")
						gotConfig := gnmi.Get(t, dut, configPath)
						if gotConfig != bgpASOC {
							errs = append(errs, fmt.Errorf("BGP AS config: got %d, want %d", gotConfig, bgpASOC))
						}
						gotState := gnmi.Get(t, dut, statePath)
						if gotState != bgpASCLI {
							errs = append(errs, fmt.Errorf("BGP AS state: got %d, want %d", gotState, bgpASCLI))
						}
					}
					if err := errors.Join(errs...); err != nil {
						t.Errorf("%v", err)
					}
				})

				t.Run("gNMI-3.8.4-RoutingPolicyOverlap", func(t *testing.T) {
					var errs []error
					t.Log("Get configuration from DUT")
					t.Log("Set routing-policy name in OC and CLI (overlap)")
					policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(policyName).Config()

					ocDelta := &oc.Root{}
					rp := ocDelta.GetOrCreateRoutingPolicy()
					pd := rp.GetOrCreatePolicyDefinition(policyName)
					stmt, err := pd.AppendNewStatement("10")
					if err != nil {
						t.Fatalf("AppendNewStatement: %v", err)
					}
					stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

					var rpCLI string
					switch dut.Vendor() {
					case ondatra.ARISTA:
						rpCLI = fmt.Sprintf("route-map %s permit 10\n", policyName)
					default:
						t.Fatalf("Routing-policy overlap test not implemented for vendor %v", dut.Vendor())
					}

					t.Log("Push via union_replace")
					setErr := unionReplace(t, dut, ocUpdate(t, ocDelta), cliUpdate(t, rpCLI))

					if setErr != nil {
						if s, ok := status.FromError(setErr); ok && s.Code() == codes.InvalidArgument {
							t.Logf("Option 1 behavior: DUT rejected routing-policy overlap with INVALID_ARGUMENT: %v", s.Message())
							if got := gnmi.LookupConfig(t, dut, policyPath); got.IsPresent() {
								errs = append(errs, fmt.Errorf("policy %s should not be present after rejected overlap", policyName))
							}
						} else {
							errs = append(errs, fmt.Errorf("gnmi.Set failed with unexpected error: %w", setErr))
						}
					} else {
						t.Log("Option 2 behavior: DUT accepted routing-policy overlap")
						if got := gnmi.LookupConfig(t, dut, policyPath); !got.IsPresent() {
							errs = append(errs, fmt.Errorf("policy %s should be present after accepted overlap", policyName))
						}
					}
					if err := errors.Join(errs...); err != nil {
						t.Errorf("%v", err)
					}
				})
				return nil
			},
		},
		{
			name: "gNMI-3.9.1-NonOverlapCLIAndOC",
			desc: "CLI and OC non-overlap in the same OC configuration tree should be accepted.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Set port1 description and MTU via OC, port2 description via CLI")
				ocDelta := &oc.Root{}
				p1Intf := ocDelta.GetOrCreateInterface(intf1Name)
				p1Intf.Description = ygot.String(descOCDescP1)
				p1Intf.Mtu = ygot.Uint16(nonOverlapMTU)

				cli := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf2Name, Description: descCLIDescP2})

				t.Log("Push OC + CLI via union_replace")
				if err := unionReplace(t, dut, ocUpdate(t, ocDelta), cliUpdate(t, cli)); err != nil {
					errs = append(errs, fmt.Errorf("union_replace gnmi.Set failed: %w", err))
				}
				t.Log("Verify OC description and MTU on port1, CLI description on port2")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descOCDescP1))
				errs = append(errs, verifyInterfaceMTU(t, dut, intf1Name, nonOverlapMTU))
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descCLIDescP2))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.10.1-MissingHardwareOC",
			desc: "Configure an interface with a missing transceiver using OC; config must be accepted, state should show oper-status DOWN.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Generate OC delta with port-speed and breakout-mode for missing-hardware interface")
				ocDelta := &oc.Root{}
				ocBreakoutMode(t, dut, ocDelta, interfaceWithoutTransceiver)
				intf := ocDelta.GetOrCreateInterface(interfaceWithoutTransceiver)
				intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
				eth := intf.GetOrCreateEthernet()
				eth.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB
				eth.DuplexMode = oc.Ethernet_DuplexMode_FULL
				eth.AutoNegotiate = ygot.Bool(false)

				intfNameParts := strings.Split(interfaceWithoutTransceiver, "/")
				breakoutIntfNum, err := strconv.Atoi(intfNameParts[1])
				if err != nil {
					return fmt.Errorf("failed to parse interface name %s: %w", interfaceWithoutTransceiver, err)
				}
				breakoutIntfName := fmt.Sprintf("%s/%d", intfNameParts[0], breakoutIntfNum+1)
				breakoutIntf := ocDelta.GetOrCreateInterface(breakoutIntfName)
				breakoutIntf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
				brEth := breakoutIntf.GetOrCreateEthernet()
				brEth.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB
				brEth.DuplexMode = oc.Ethernet_DuplexMode_FULL
				brEth.AutoNegotiate = ygot.Bool(false)

				t.Log("Push via union_replace and verify accepted")
				if err := unionReplace(t, dut, ocUpdate(t, ocDelta)); err != nil {
					t.Logf("union_replace rejected: %v", err)
					return err
				}

				t.Log("Verify configuration applied and interface oper-status")
				errs = append(errs, verifyPortSpeed(t, dut, interfaceWithoutTransceiver, oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB))
				errs = append(errs, verifyInterfaceOperStatus(t, dut, interfaceWithoutTransceiver, oc.Interface_OperStatus_NOT_PRESENT))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.10.2-MissingHardwareCLI",
			desc: "Configure an interface with a missing transceiver using CLI; config must be accepted.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				var errs []error

				t.Log("Generate CLI configuration with port-speed and breakout-mode")
				cli := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: interfaceWithoutTransceiver, Speed: portSpeed50GCLI})
				if breakoutCLI := cliBreakoutMode(t, dut, interfaceWithoutTransceiver); breakoutCLI != "" {
					cli += "\n" + breakoutCLI
				}

				t.Log("Push via union_replace with CLI and verify accepted")
				if err := unionReplace(t, dut, cliUpdate(t, cli)); err != nil {
					t.Logf("union_replace rejected: %v", err)
					return err
				}

				t.Log("Verify configuration applied")
				errs = append(errs, verifyPortSpeed(t, dut, interfaceWithoutTransceiver, oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB))
				errs = append(errs, verifyInterfaceOperStatus(t, dut, interfaceWithoutTransceiver, oc.Interface_OperStatus_NOT_PRESENT))
				return errors.Join(errs...)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer resetConfig()
			t.Log(tc.desc)
			if err := tc.fn(t); err != nil {
				t.Errorf("%v", err)
			}
		})
	}
}
