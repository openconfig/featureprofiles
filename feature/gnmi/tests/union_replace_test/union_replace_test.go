package union_replace_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	ocOrigin     = "openconfig"
	cliOrigin    = "cli"
	defaultSubif = uint32(0)

	port1         = "port1"
	port2         = "port2"
	port1IPv4     = "192.0.2.1"
	ipv4PrefixLen = uint8(30)

	overlapMTUOC  = uint16(9000)
	overlapMTUCLI = uint16(1500)
	nonOverlapMTU = uint16(8000)
	moveIPMTU     = uint16(1500)

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
	bgpASOC         = uint32(64496)
	bgpASCLI        = uint32(64497)
	policyName      = "OVERLAP_POLICY_1"
	nonExistentIntf = "Ethernet999/1/1"
	badIntfMTU      = uint16(5000)

	portSpeed50GCLI    = "50g"
	portSpeedBreakout  = "50g-2"
	breakoutNumGroups  = uint8(2)
	breakoutNumChannel = uint8(2)

	awaitStateTimeOut = 60 * time.Second
	awaitTimeOut      = 10 * time.Second

	groupIndex = uint8(1)
)

var (
	sharedBaseline string
	defaultNI      string
)

type cliInterfaceConfigOpts struct {
	Name          string
	Description   string
	MTU           uint16
	IPv4          string
	IPv4PrefixLen uint8
	Speed         string
}
type testCase struct {
	name string
	desc string
	fn   func(t *testing.T) error
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	dutIntf = attrs.Attributes{
		Desc:    "unionreplacetest",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: 30,
		IPv6Len: 64,
		Duplex:  "FULL",
	}
)

var portSpeed = map[ondatra.Speed]oc.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed10Gb:  oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
}

func configOCInterface(t *testing.T, sb *gnmi.SetBatch, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	i := dutIntf.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	if deviations.ExplicitPortSpeed(dut) {
		i.GetOrCreateEthernet().SetPortSpeed(fptest.GetIfSpeed(t, dp1))
	}
	inf := gnmi.OC().Interface(dp1.Name())
	// TODO: add handling for ExplicitPortSpeed deviation and ExplicitInterfaceInDefaultVRF deviation

	gnmi.BatchUnionReplace(sb, inf.Config(), i)
}

func awaitStateEq[T comparable](t *testing.T, dut *ondatra.DUTDevice, path ygnmi.SingletonQuery[T], want T, timeout time.Duration) (T, bool) {
	t.Helper()
	var got T
	_, ok := gnmi.Watch(t, dut, path, timeout, func(val *ygnmi.Value[T]) bool {
		v, present := val.Val()
		if !present {
			return false
		}
		got = v
		return v == want
	}).Await(t)
	return got, ok
}

// prettyPrintYgnmiResult formats a *ygnmi.Result as JSON for logging.
// Note: ygnmi.Result contains a protobuf (SetResponse) rather than YANG data,
// so it is formatted as standard JSON via protojson rather than RFC7951.
func prettyPrintYgnmiResult(setResult *ygnmi.Result) string {
	if setResult == nil || setResult.RawResponse == nil {
		return ""
	}
	opts := protojson.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}
	b, err := opts.Marshal(setResult.RawResponse)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func setCLINoMTU(t *testing.T, dut *ondatra.DUTDevice, portName string) {
	t.Helper()
	var cli string
	if dut.Vendor() == ondatra.ARISTA {
		cli = fmt.Sprintf("configure terminal\ninterface %s\nno mtu\n", portName)
	} else {
		t.Fatalf("unsupported vendor: %v", dut.Vendor())
	}
	helpers.GnmiCLIConfig(t, dut, cli)
	// Wait for the MTU to be removed (i.e., not equal to 1500).
	gnmi.Watch(t, dut, gnmi.OC().Interface(portName).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
		m, present := val.Val()
		if !present {
			t.Logf("Got MTU not present, want 1500.")
			return false
		}
		if m == 1500 {
			return true
		}
		return false
	}).Await(t)
}

// setCLIunionReplace adds any necessary modifications to the base CLI configuration
// for union replace.
func setCLIunionReplace(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	clicfg1 := ""

	if dut.Vendor() == ondatra.ARISTA {
		// Add  "operation set namespace" command from the CLI config as this is required for Arista
		// when using union replace.
		clicfg1 = "configure terminal\nmanagement api gnmi\n  provider eos-native\n  operation set persistence\n  operation set namespace\n"
		helpers.GnmiCLIConfig(t, dut, clicfg1)
		// Poll for config to be applied.
		start := time.Now()
		for {
			if time.Since(start) > 1*time.Minute {
				t.Fatal("setCLIunionReplace config not applied in time")
			}
			clicfg2 := cliConfig(t, dut)
			if strings.Contains(clicfg2, "operation set namespace") && strings.Contains(clicfg2, "operation set persistence") {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

}

// Remove description lines from the base CLI config.
// Since some vendors give priority to CLI config over OC config during union_replace,
// having descriptions like "description [AVAILABLE]" in the base CLI config
// will override the descriptions set by the tests via OC, causing test failures.
func stripDescription(config string) string {
	re := regexp.MustCompile(`(?m)^\s*description .*$\n?`)
	return re.ReplaceAllString(config, "")
}

func cliConfig(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	config := cliConfigGNMI(t, dut)
	if config == "" {
		t.Logf("Fallback to SSH to get baseline config")
		config = cliConfigSSH(t, dut)
		if config == "" {
			t.Fatal("Unable to get baseline CLI config from GNMI or SSH")
		}
	}
	return stripDescription(config)
}

func cliConfigGNMI(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	showCmd, _ := cliShowRunningConfigCommand(t, dut)
	req := &gpb.GetRequest{
		Path: []*gpb.Path{{
			Origin: cliOrigin,
			Elem:   []*gpb.PathElem{{Name: showCmd}},
		}},
		Encoding: gpb.Encoding_ASCII,
	}
	resp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
	if err == nil {
		for _, notif := range resp.Notification {
			for _, update := range notif.Update {
				if s := update.Val.GetAsciiVal(); s != "" {
					return s
				}
			}
		}
	} else {
		t.Logf("Got GNMI baseline config error: %v", err)
	}
	return ""

}

// cliConfigSSH returns the CLI config of the DUT as a string
func cliConfigSSH(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	runCommand, commandEchoPrefix := cliShowRunningConfigCommand(t, dut)
	if runCommand == "" {
		return ""
	}
	return stripSSHCommandEcho(helpers.RunCliCommand(t, dut, runCommand), commandEchoPrefix)
}

// stripSSHCommandEcho removes command-echo lines (lines starting with the given prefix)
// that are echoed by the SSH session and are not valid CLI configuration input.
func stripSSHCommandEcho(raw string, commandEchoPrefix string) string {
	if commandEchoPrefix == "" {
		return raw
	}
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		if !strings.HasPrefix(line, commandEchoPrefix) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
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

func operStatusNoTransceiver(t *testing.T, dut *ondatra.DUTDevice) oc.E_Interface_OperStatus {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return oc.Interface_OperStatus_NOT_PRESENT
	default:
		return oc.Interface_OperStatus_DOWN
	}
}

func cliShowRunningConfigCommand(t *testing.T, dut *ondatra.DUTDevice) (string, string) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return "show running-config", ">"
	case ondatra.CISCO:
		return "show running-config", "#"
	case ondatra.JUNIPER:
		return "show | display set", ""
	case ondatra.NOKIA:
		return "info | as-set", ""
	default:
		t.Fatalf("unsupported vendor %v for CLI show running-config command", dut.Vendor())
		return "", ""
	}
}

func unionReplaceErr(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch) error {
	t.Helper()
	if fatalMsg := testt.CaptureFatal(t, func(t testing.TB) {
		sb.Set(t, dut)
	}); fatalMsg != nil {
		if strings.Contains(*fatalMsg, codes.InvalidArgument.String()) {
			return status.Error(codes.InvalidArgument, *fatalMsg)
		}
		return errors.New(*fatalMsg)
	}
	return nil
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
	if got, ok := awaitStateEq(t, dut, gnmi.OC().Interface(intfName).Description().State(), wantDesc, awaitStateTimeOut); !ok {
		return fmt.Errorf("interface %s description: got %q, want %q", intfName, got, wantDesc)
	}
	return nil
}

func verifyInterfaceIP(t *testing.T, dut *ondatra.DUTDevice, intfName, wantIPv4 string, wantLen uint8) error {
	t.Helper()
	if got, ok := awaitStateEq(t, dut, gnmi.OC().Interface(intfName).Subinterface(defaultSubif).Ipv4().Address(wantIPv4).PrefixLength().State(), wantLen, awaitStateTimeOut); !ok {
		return fmt.Errorf("interface %s IPv4 %s prefix-length: got %v, want %v", intfName, wantIPv4, got, wantLen)
	}
	return nil
}

func verifyInterfaceNoIP(t *testing.T, dut *ondatra.DUTDevice, intfName, ipv4 string) error {
	t.Helper()
	_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(intfName).Subinterface(defaultSubif).Ipv4().Address(ipv4).PrefixLength().State(), awaitStateTimeOut, func(val *ygnmi.Value[uint8]) bool {
		return !val.IsPresent()
	}).Await(t)
	if !ok {
		v := gnmi.Lookup(t, dut, gnmi.OC().Interface(intfName).Subinterface(defaultSubif).Ipv4().Address(ipv4).PrefixLength().State())
		got, _ := v.Val()
		return fmt.Errorf("interface %s IPv4 %s should not be present, got prefix-length %v", intfName, ipv4, got)
	}
	return nil
}

func verifyInterfaceMTU(t *testing.T, dut *ondatra.DUTDevice, intfName string, wantMTU uint16) error {
	t.Helper()
	if got, ok := awaitStateEq(t, dut, gnmi.OC().Interface(intfName).Mtu().State(), wantMTU, awaitTimeOut); ok {
		t.Logf("Interface %s MTU state verified: %v", intfName, got)
		return nil
	}
	if verifyCLIMTU(t, dut, intfName, wantMTU) {
		return nil
	}
	return fmt.Errorf("interface %s MTU validation failed, want %v", intfName, wantMTU)
}

func verifyCLIMTU(t *testing.T, dut *ondatra.DUTDevice, intfName string, wantMTU uint16) bool {
	t.Helper()
	interfaceConfig := cliInterface(cliConfig(t, dut), intfName)
	if strings.Contains(interfaceConfig, fmt.Sprintf("mtu %d", wantMTU)) {
		t.Logf("Interface %s MTU verified in CLI running-config", intfName)
		return true
	}
	return false
}

func verifyInterfaceNotPresent(t *testing.T, dut *ondatra.DUTDevice, intfName string) error {
	t.Helper()
	_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(intfName).State(), awaitStateTimeOut, func(v *ygnmi.Value[*oc.Interface]) bool {
		return !v.IsPresent()
	}).Await(t)
	if !ok {
		return fmt.Errorf("interface %s should not be present after deletion", intfName)
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
	got, ok := gnmi.Await(t, dut, gnmi.OC().Interface(intfName).OperStatus().State(), awaitStateTimeOut, wantStatus).Val()
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

func breakoutInterfaces(t *testing.T, dut *ondatra.DUTDevice, baseIntfName string, noOfIntfs uint8) []string {
	t.Helper()
	var intfs []string
	switch dut.Vendor() {
	case ondatra.ARISTA:
		lastIndex := strings.LastIndex(baseIntfName, "/")
		if lastIndex == -1 {
			t.Fatalf("invalid interface name format: %s", baseIntfName)
		}
		baseName := baseIntfName[:lastIndex]
		for index := range noOfIntfs {
			intfs = append(intfs, fmt.Sprintf("%s/%d", baseName, index+1))
		}
	default:
		t.Logf("unsupported vendor %v for breakout interfaces", dut.Vendor())
	}
	return intfs
}

func ocBreakoutMode(t *testing.T, dut *ondatra.DUTDevice, root *oc.Root, intfName string) {
	t.Helper()
	hwPort, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(intfName).HardwarePort().State()).Val()
	if !ok || hwPort == "" {
		t.Logf("Skipping breakout-mode OC config: no hardware-port for %s", intfName)
		return
	}
	comp := root.GetOrCreateComponent(hwPort)
	grp := comp.GetOrCreatePort().GetOrCreateBreakoutMode().GetOrCreateGroup(groupIndex)
	grp.Index = ygot.Uint8(1)
	grp.NumBreakouts = ygot.Uint8(breakoutNumGroups)
	grp.BreakoutSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB
	grp.NumPhysicalChannels = ygot.Uint8(breakoutNumChannel)
}

func cliBreakoutMode(t *testing.T, dut *ondatra.DUTDevice, intfName string) string {
	return cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intfName, Speed: portSpeedBreakout})
}

func verifyBreakoutModeConfig(t *testing.T, dut *ondatra.DUTDevice, intfName string) error {
	t.Helper()
	hwPort, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(intfName).HardwarePort().State()).Val()
	if !ok || hwPort == "" {
		t.Logf("Skipping breakout-mode config verification: no hardware-port for %s", intfName)
		return nil
	}
	var errs []error
	grp := gnmi.OC().Component(hwPort).Port().BreakoutMode().Group(groupIndex)
	if got, ok := gnmi.Lookup(t, dut, grp.NumBreakouts().Config()).Val(); ok {
		t.Logf("component %s breakout group num-breakouts: %d", hwPort, got)
	} else {
		errs = append(errs, fmt.Errorf("component %s breakout group num-breakouts: not present", hwPort))
	}
	if got, ok := gnmi.Lookup(t, dut, grp.BreakoutSpeed().Config()).Val(); !ok || got != oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB {
		errs = append(errs, fmt.Errorf("component %s breakout group breakout-speed: got %v (present=%v), want SPEED_50GB", hwPort, got, ok))
	}
	if got, ok := gnmi.Lookup(t, dut, grp.NumPhysicalChannels().Config()).Val(); !ok || got != breakoutNumChannel {
		errs = append(errs, fmt.Errorf("component %s breakout group num-physical-channels: got %v (present=%v), want %d", hwPort, got, ok, breakoutNumChannel))
	}
	return errors.Join(errs...)
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

	sharedBaseline = cliConfig(t, dut)
	interfaceWithoutTransceiver := firstInterfaceWithoutTransceiver(t, dut)
	noTransceiverOperStatus := operStatusNoTransceiver(t, dut)
	t.Logf("First interface without transceiver: %s", interfaceWithoutTransceiver)
	resetConfig := func() {
		t.Log("Resetting baseline configuration")
		sb := &gnmi.SetBatch{}
		gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline)
		sb.Set(t, dut)
	}
	testCases := []testCase{
		// TestUnionReplace3_1_idempotentConfig verifies the gNMI UnionReplace operation with CLI config only.
		// gNMI-3.1 - Idempotent configuration
		{
			name: "gNMI-3.1-IdempotentConfiguration",
			desc: "Verify the same configuration already on the device can be pushed and accepted without changing the configuration.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")

				// ensure union replace is enabled on the DUT and get the CLI config using union replace after setting
				// the base CLI config.
				setCLIunionReplace(t, dut)
				clicfg1 := cliConfig(t, dut)
				sb1 := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb1, cliOrigin, clicfg1)
				sb1.Set(t, dut)
				time.Sleep(5 * time.Second)
				clicfg2 := cliConfig(t, dut)

				// second, set the same CLI config again.
				sb2 := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb2, cliOrigin, clicfg2)
				sb2.Set(t, dut)
				time.Sleep(5 * time.Second)

				// verify the CLI config has not changed.
				clicfg3 := cliConfig(t, dut)
				if clicfg2 != clicfg3 {
					return fmt.Errorf("cliConfig before and after do not match")
				}
				return nil
			},
		},
		// TestUnionReplace3_2_1_addOCMTU verifies the gNMI UnionReplace with CLI for a base config and
		// adds MTU of an interface using OC only.  This tests that the MTU leaf is updated even though the
		// interface already exists in the CLI config.  It assumes that the CLI config does not contain an MTU
		// value for the interface.
		// gNMI-3.2.1 - Add interface using OC
		{
			name: "gNMI-3.2.1-AddConfigurationOC",
			desc: "Add an interface configuration using OC to the baseline configuration.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				dp1 := dut.Port(t, "port1")
				sb1 := &gnmi.SetBatch{}

				setCLIunionReplace(t, dut)
				setCLINoMTU(t, dut, dp1.Name())

				// Add MTU to the interface using OC config.
				cliConfig2 := cliConfig(t, dut)
				gnmi.BatchUnionReplaceCLI(sb1, cliOrigin, cliConfig2)
				gnmi.BatchUnionReplace(sb1, gnmi.OC().Interface(dp1.Name()).Mtu().Config(), 1400)
				t.Logf("Generated BatchUnionReplace: %#v\n", sb1.String())

				setResult := sb1.Set(t, dut)
				t.Logf("\nSetResult: %#v\n", prettyPrintYgnmiResult(setResult))

				return verifyInterfaceMTU(t, dut, dp1.Name(), 1400)
			},
		},
		// TestUnionReplace3_2_2_addCLIInterface verifies the gNMI UnionReplace with CLI for a base config
		// and configures MTU of an interface using CLI and OC, therefore mixing interfaces with CLI and OC.
		// gNMI-3.2.2 - Add interface using CLI
		{
			name: "gNMI-3.2.2-AddConfigurationCLI",
			desc: "Add an interface configuration using CLI to the baseline configuration.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				dp2 := dut.Port(t, "port2")
				sb1 := &gnmi.SetBatch{}
				sb2 := &gnmi.SetBatch{}

				// ensure that the device has union_replace enabled and the base CLI config does not contain an MTU
				// config for the interface being tested.
				setCLIunionReplace(t, dut)
				setCLINoMTU(t, dut, dp2.Name())

				// Add MTU to the interface using OC config to a known value.
				cliConfig1 := cliConfig(t, dut)
				gnmi.BatchUnionReplaceCLI(sb1, cliOrigin, cliConfig1)
				gnmi.BatchUnionReplace(sb1, gnmi.OC().Interface(dp2.Name()).Mtu().Config(), 1400)
				sb1.Set(t, dut)
				if err := verifyInterfaceMTU(t, dut, dp2.Name(), 1400); err != nil {
					return err
				}

				// Change the MTU to a different value using CLI config.
				cliConfig2 := cliConfig(t, dut)
				switch dut.Vendor() {
				case ondatra.ARISTA:
					cliConfig2 += fmt.Sprintf("interface %s\nmtu 1300\n", dp2.Name())
				case ondatra.CISCO:
					cliConfig2 += fmt.Sprintf("interface %s\nmtu 1300\n", dp2.Name())
				case ondatra.JUNIPER:
					cliConfig2 += fmt.Sprintf("set interfaces %s mtu 1300\n", dp2.Name())
				default:
					return fmt.Errorf("unsupported vendor: %v", dut.Vendor())
				}
				gnmi.BatchUnionReplaceCLI(sb2, cliOrigin, cliConfig2)
				setResult := sb2.Set(t, dut)
				t.Logf("\nSetResult: %#v\n", prettyPrintYgnmiResult(setResult))

				// If union_replace option for CLI overriding OC is the DUT behavior, verify the MTU is updated
				// to the new, CLI configured value. If union_replace option for CLI and OC config error is the
				// DUT behavior, verify the MTU is not updated to the new, CLI configured value.
				switch dut.Vendor() {
				case ondatra.ARISTA:
					// CLI overrides OC
					if err := verifyInterfaceMTU(t, dut, dp2.Name(), 1300); err != nil {
						return err
					}
				case ondatra.CISCO, ondatra.JUNIPER, ondatra.NOKIA:
					// OC and CLI conflict generates an error, MTU stays at 1400
					if err := verifyInterfaceMTU(t, dut, dp2.Name(), 1400); err != nil {
						return err
					}
				default:
					return fmt.Errorf("unsupported vendor: %v", dut.Vendor())
				}
				return nil
			},
		},
		// TestUnionReplace3_3_1_changeOCConfig verifies the gNMI UnionReplace with CLI for a base config and
		// changes an OC config.
		// gNMI-3.3.1 - Change OC config
		{
			name: "gNMI-3.3.1-ChangeConfigurationOC",
			desc: "Change the interface description using OC via union_replace.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				setCLIunionReplace(t, dut)
				portName := dut.Port(t, "port2").Name()
				sb := &gnmi.SetBatch{}

				// reset the CLI config for the port to remove any previous MTU config.
				setCLINoMTU(t, dut, portName)

				// Add MTU to the interface using OC config.
				cliConfig1 := cliConfig(t, dut)
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, cliConfig1)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(portName).Mtu().Config(), 1450)
				sb.Set(t, dut)

				if err := verifyInterfaceMTU(t, dut, portName, 1450); err != nil {
					return err
				}

				// Change the MTU using OC config.
				// reuse the same CLI config without any MTU config.
				sb2 := &gnmi.SetBatch{}
				gnmi.BatchUnionReplace(sb2, gnmi.OC().Interface(portName).Mtu().Config(), 1440)
				gnmi.BatchUnionReplaceCLI(sb2, cliOrigin, cliConfig1)
				sb2.Set(t, dut)

				return verifyInterfaceMTU(t, dut, portName, 1440)
			},
		},
		// TestUnionReplace3_3_2_changeCLIConfig verifies the gNMI UnionReplace with CLI for a base config and
		// changes a CLI config.
		// gNMI-3.3.2 - Change CLI config
		{
			name: "gNMI-3.3.2-ChangeConfigurationCLI",
			desc: "Change the interface description using CLI via union_replace.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				setCLIunionReplace(t, dut)
				port1Name := dut.Port(t, "port1").Name()
				port1DescriptionOC := "unionreplacetest gnmi-3.3.2 OC"
				port1DescriptionCLI := "unionreplacetest gnmi-3.3.2 CLI"
				sb1 := &gnmi.SetBatch{}

				// Set the interface description to a known value using OC config.
				// Add OC interface and set description on the interface.
				gnmi.BatchUnionReplace(sb1, gnmi.OC().Interface(port1Name).Description().Config(), port1DescriptionOC)
				cliConfig1 := cliConfig(t, dut)
				gnmi.BatchUnionReplaceCLI(sb1, cliOrigin, cliConfig1)
				sb1.Set(t, dut)
				gnmi.Watch(t, dut, gnmi.OC().Interface(port1Name).Description().State(), awaitTimeOut, func(val *ygnmi.Value[string]) bool {
					desc, present := val.Val()
					if !present {
						t.Logf("Description not present. Want: %q, got: not present", port1DescriptionOC)
						return false
					}
					if desc != port1DescriptionOC {
						t.Logf("Description not set to OC configured value. Want: %q, got: %q", port1DescriptionOC, desc)
						return false
					}
					return true
				}).Await(t)

				// Change the interface description using CLI config.
				// the OC configuration does not include an interface description.
				sb2 := &gnmi.SetBatch{}
				cliConfig2 := cliConfig(t, dut)
				cliConfig2 += fmt.Sprintf("interface %s\ndescription "+port1DescriptionCLI+"\n", dut.Port(t, "port1").Name())
				gnmi.BatchUnionReplaceCLI(sb2, cliOrigin, cliConfig2)
				sb2.Set(t, dut)

				// Watch for the description to be updated to the CLI configured value.
				gnmi.Watch(t, dut, gnmi.OC().Interface(port1Name).Description().State(), awaitTimeOut, func(val *ygnmi.Value[string]) bool {
					desc, present := val.Val()
					if !present {
						t.Logf("Description not present. Want: %q, got: not present", port1DescriptionCLI)
						return false
					}
					if desc != port1DescriptionCLI {
						t.Logf("Description does not match the CLI configured value.  want: %q, got: %q", port1DescriptionCLI, desc)
						return false
					}
					return true // Description is now port1DescriptionCLI
				}).Await(t)
				return nil
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
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), bothOC)
				sb.Set(t, dut)
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descIntf1Present))
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descIntf2Present))

				t.Log("Omit interface 2 in OC union_replace")
				intf1Only := &oc.Root{}
				intf1Only.GetOrCreateInterface(intf1Name).Description = ygot.String(descIntf1Present)
				sb2 := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb2, cliOrigin, sharedBaseline)
				gnmi.BatchUnionReplace(sb2, gnmi.OC().Config(), intf1Only)
				sb2.Set(t, dut)

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
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline+"\n"+cli1And2)
				sb.Set(t, dut)
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descCLIIntf1))
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descCLIIntf2))

				t.Log("Omit interface 2 in CLI union_replace")
				cli1Only := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf1Name, Description: descCLIIntf1})
				sb2 := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb2, cliOrigin, sharedBaseline+"\n"+cli1Only)
				sb2.Set(t, dut)

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
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocConfig)
				sb.Set(t, dut)
				errs = append(errs, verifyInterfaceIP(t, dut, intf1Name, port1IPv4, ipv4PrefixLen))

				t.Log("Move IP from port1 to port2")
				ocConfig = &oc.Root{}
				ocConfig.GetOrCreateInterface(intf1Name).Description = ygot.String(descMoveIPMoved)
				configureOCInterface(t, ocConfig, dut, intf2Name, descMoveHasIPNow, 0, port1IPv4, ipv4PrefixLen)
				sb2 := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb2, cliOrigin, sharedBaseline)
				gnmi.BatchUnionReplace(sb2, gnmi.OC().Config(), ocConfig)
				sb2.Set(t, dut)

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
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline+"\n"+cli1)
				sb.Set(t, dut)

				t.Log("Move IP to port2 via CLI, omitting port1")
				cli2 := cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intf2Name, Description: descMoveHasIPNow, MTU: moveIPMTU, IPv4: port1IPv4, IPv4PrefixLen: ipv4PrefixLen})
				sb2 := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb2, cliOrigin, sharedBaseline+"\n"+cli2)
				sb2.Set(t, dut)

				t.Log("Verify IP is now on port2")
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descMoveHasIPNow))
				errs = append(errs, verifyInterfaceNoIP(t, dut, intf1Name, port1IPv4))
				errs = append(errs, verifyInterfaceIP(t, dut, intf2Name, port1IPv4, ipv4PrefixLen))
				return errors.Join(errs...)
			},
		},
		// TestUnionReplace3_6_1 tests the gNMI union_replace accepted with hardware mismatch.
		// load the cli config from DUT
		// generate OC config for 1 DUT 100Gbps port but set port speed to 10Gbps (intentionally mismatch)
		// build the union replace request with the cli config and OC config
		// send the request to the DUT
		// verify the DUT OC config contains the port speed of 10Gbps
		// verify the DUT OC /interfaces/interface/state/oper-status is DOWN
		{
			name: "gNMI-3.6.1-HardwareMismatchOC",
			desc: "Verify configuration with OC hardware mismatch (wrong port-speed) is accepted.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				setCLIunionReplace(t, dut)
				sb := &gnmi.SetBatch{}
				targetSpeed := oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB

				// confirm the testbed defined and DUT reported port speed are not the target speed.
				dp1 := dut.Port(t, "port1")
				speedCurrent := portSpeed[dp1.Speed()]
				t.Logf("DUT %v port speed defined in the testbed is %v", dp1.Name(), speedCurrent)
				beforeSpeed := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().State())
				t.Logf("DUT reported PortSpeed state before any changes: %v", beforeSpeed)

				if speedCurrent == targetSpeed {
					return fmt.Errorf("Need a different topology for this test. DUT port %q current port speed must not be %q", dp1.Name(), targetSpeed)
				}

				t.Logf("Configuring DUT port %q to mismatched port-speed %q using gNMI union_replace.", dp1.Name(), targetSpeed)
				// get the cli config from DUT and add it to the SetBatch.
				clicfg1 := cliConfig(t, dut)
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, clicfg1)
				/*
				   These Arista EOS CLI commands would allow EOS to accept the port speed mismatch but are not
				   included as they are not accepted as a deviation.
				   system l1
				     unsupported speed action warn
				*/

				// add configuration of the OC interface to the SetBatch
				configOCInterface(t, sb, dut)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().Config(), targetSpeed)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(dp1.Name()).Ethernet().DuplexMode().Config(), oc.Ethernet_DuplexMode_FULL)
				t.Logf("Generated BatchUnionReplace: %#v\n", sb.String())

				// send the request to the DUT.
				setResult := sb.Set(t, dut)
				t.Logf("SetResult:\n%s", prettyPrintYgnmiResult(setResult))

				// Verify the port speed CONFIG leaf is the targetSpeed.  It is expected that the port speed config
				// leaf is updated to the target speed.
				gnmi.Watch(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().Config(), awaitTimeOut, func(val *ygnmi.Value[oc.E_IfEthernet_ETHERNET_SPEED]) bool {
					speed, present := val.Val()
					if !present {
						t.Logf("PortSpeed config not present. Want: %v, got: not present", targetSpeed)
						return false
					}
					if speed != targetSpeed {
						t.Logf("PortSpeed config not set to target speed. Want: %v, got: %v", targetSpeed, speed)
						return false
					}
					t.Logf("PortSpeed config is set to target speed: %v", speed)
					return true
				}).Await(t)

				// Verify the port speed state leaf is the beforeSpeed or UNKNOWN.   It is expected that the
				// PortSpeed state leaf was not affected by the new configuration and reflects the actual
				// operating speed of the port.
				foundSpeed := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().State())
				if foundSpeed != beforeSpeed && foundSpeed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
					return fmt.Errorf("DUT port1 PortSpeed state: got %v, want %v or unknown", foundSpeed, beforeSpeed)
				}

				want := oc.Interface_OperStatus_DOWN
				gnmi.Watch(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), awaitTimeOut, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
					status, present := val.Val()
					if !present {
						t.Logf("OperStatus not present yet")
						return false
					}
					if status != want {
						t.Logf("OperStatus not in expected state.  Want: %v, got: %v", want, status)
						return false
					}
					t.Logf("OperStatus is in expected state: %v", status)
					return true
				}).Await(t)
				return nil
			},
		},
		// TestUnionReplace3.6.2 tests the gNMI union_replace accepted with hardware mismatch using CLI.
		// load the cli config from DUT
		// generate CLI config for 1 DUT 100Gbps port but set port speed to 10Gbps (intentionally mismatch)
		// build the union replace request with the cli config and OC config
		// send the request to the DUT
		// verify the DUT OC config contains the port speed of 10Gbps
		// verify the DUT OC /interfaces/interface/state/oper-status is DOWN
		{
			name: "gNMI-3.6.2-HardwareMismatchCLI",
			desc: "Verify configuration with CLI hardware mismatch is accepted.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				setCLIunionReplace(t, dut)
				sb := &gnmi.SetBatch{}
				targetSpeed := oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB

				// confirm the testbed defined and DUT reported port speed are not the target speed.
				dp1 := dut.Port(t, "port1")
				speedCurrent := portSpeed[dp1.Speed()]
				t.Logf("DUT %v port speed defined in the testbed is %v", dp1.Name(), speedCurrent)
				beforeSpeed := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().State())
				t.Logf("DUT reported PortSpeed state before any changes: %v", beforeSpeed)

				if speedCurrent == targetSpeed {
					return fmt.Errorf("Need a different topology for this test. DUT port %q current port speed must not be %q", dp1.Name(), targetSpeed)
				}

				t.Logf("Configuring DUT port %q to mismatched port-speed %q using gNMI union_replace CLI.", dp1.Name(), targetSpeed)
				// get the cli config from DUT, modify it to introduce the port speed mismatch, and add it to the SetBatch.
				clicfg1 := cliConfig(t, dut)
				switch dut.Vendor() {
				case ondatra.ARISTA:
					clicfg1 += fmt.Sprintf("interface %s\nspeed 10g\n", dp1.Name())
				case ondatra.CISCO:
					clicfg1 += fmt.Sprintf("interface %s\nspeed 10000\n", dp1.Name())
				case ondatra.JUNIPER:
					clicfg1 += fmt.Sprintf("set interfaces %s speed 10g\n", dp1.Name())
				default:
					return fmt.Errorf("unsupported vendor: %v", dut.Vendor())
				}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, clicfg1)
				/*
				    These Arista EOS CLI commands would allow EOS to accept the port speed mismatch but are not
				    included as they are not accepted as a deviation.
				    system l1
				   unsupported speed action warn
				*/

				// add configuration of the OC interface to the SetBatch
				configOCInterface(t, sb, dut)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(dp1.Name()).Ethernet().DuplexMode().Config(), oc.Ethernet_DuplexMode_FULL)
				t.Logf("Generated BatchUnionReplace: %#v\n", sb.String())

				// send the request to the DUT.
				setResult := sb.Set(t, dut)
				t.Logf("SetResult:\n%s", prettyPrintYgnmiResult(setResult))

				// Verify the port speed CONFIG leaf is the before speed.  It is expected that the port speed config
				// leaf is updated to the target speed.
				gnmi.Watch(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().Config(), awaitTimeOut, func(val *ygnmi.Value[oc.E_IfEthernet_ETHERNET_SPEED]) bool {
					speed, present := val.Val()
					if !present {
						t.Logf("PortSpeed config not present. Want: %v, got: not present", targetSpeed)
						return false
					}
					if speed != targetSpeed {
						t.Logf("PortSpeed config not set to target speed. Want: %v, got: %v", targetSpeed, speed)
						return false
					}
					t.Logf("PortSpeed config is set to target speed: %v", speed)
					return true
				}).Await(t)

				// Verify the port speed state leaf is the beforeSpeed or UNKNOWN.   It is expected that the
				// PortSpeed state leaf was not affected by the new configuration and reflects the actual
				// operating speed of the port.
				foundSpeed := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().PortSpeed().State())
				if foundSpeed != beforeSpeed && foundSpeed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
					return fmt.Errorf("DUT port1 PortSpeed state: got %v, want %v or unknown", foundSpeed, beforeSpeed)
				}

				want := oc.Interface_OperStatus_DOWN
				gnmi.Watch(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), awaitTimeOut, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
					status, present := val.Val()
					if !present {
						t.Logf("OperStatus not present yet")
						return false
					}
					if status != want {
						t.Logf("OperStatus not in expected state.  Want: %v, got: %v", want, status)
						return false
					}
					t.Logf("OperStatus is in expected state: %v", status)
					return true
				}).Await(t)
				return nil
			},
		},
		{
			name: "gNMI-3.7-RejectedInvalidConfig",
			desc: "Verify a DUT rejects and rolls back a union_replace with invalid configuration (non-existent interface).",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")

				t.Run("gNMI-3.7.1-InvalidInterfaceOC", func(t *testing.T) {
					var errs []error
					t.Log("Build OC delta with MTU configured on a non-existent interface")
					ocDelta := &oc.Root{}
					intf := ocDelta.GetOrCreateInterface(nonExistentIntf)
					intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
					intf.Mtu = ygot.Uint16(badIntfMTU)

					t.Log("Push configuration via union_replace")
					sb := &gnmi.SetBatch{}
					gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocDelta)
					gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline)
					err := unionReplaceErr(t, dut, sb)

					t.Log("Confirm DUT rejects the gnmi.Set")
					if err == nil {
						errs = append(errs, fmt.Errorf("expected gnmi.Set to be rejected for non-existent interface %s in OC, but it succeeded", nonExistentIntf))
					} else {
						t.Logf("gnmi.Set rejected as expected: %v", err)
						t.Log("Get configuration and verify config unchanged (non-existent interface not configured)")
						if v := gnmi.Lookup(t, dut, gnmi.OC().Interface(nonExistentIntf).Mtu().Config()); v.IsPresent() {
							got, _ := v.Val()
							errs = append(errs, fmt.Errorf("expected interface %s not to be configured after rollback, got MTU %d", nonExistentIntf, got))
						}
					}
					if err := errors.Join(errs...); err != nil {
						t.Errorf("%v", err)
					}
				})

				t.Run("gNMI-3.7.2-InvalidInterfaceCLI", func(t *testing.T) {
					var errs []error
					t.Log("Build CLI config with MTU configured on a non-existent interface")
					var badCLI string
					switch dut.Vendor() {
					case ondatra.ARISTA:
						badCLI = fmt.Sprintf("interface %s\n  mtu %d\n", nonExistentIntf, badIntfMTU)
					default:
						t.Fatalf("CLI invalid interface test not implemented for vendor %v", dut.Vendor())
					}

					t.Log("Push via union_replace with bad CLI")
					sb := &gnmi.SetBatch{}
					gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline+"\n"+badCLI)
					err := unionReplaceErr(t, dut, sb)

					t.Log("Confirm DUT rejects the gnmi.Set")
					if err == nil {
						errs = append(errs, fmt.Errorf("expected gnmi.Set to be rejected for non-existent interface %s in CLI, but it succeeded", nonExistentIntf))
					} else {
						t.Logf("gnmi.Set rejected as expected: %v", err)
						t.Log("Get configuration and verify config unchanged (non-existent interface not configured via CLI)")
						if v := gnmi.Lookup(t, dut, gnmi.OC().Interface(nonExistentIntf).Mtu().Config()); v.IsPresent() {
							got, _ := v.Val()
							errs = append(errs, fmt.Errorf("expected interface %s not to be configured after rollback, got MTU %d", nonExistentIntf, got))
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
					sb := &gnmi.SetBatch{}
					gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocDelta)
					gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline+"\n"+cli)
					err := unionReplaceErr(t, dut, sb)

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
							gotState, ok := gnmi.Await(t, dut, statePath, awaitTimeOut, overlapMTUCLI).Val()
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
					sb := &gnmi.SetBatch{}
					gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocDelta)
					gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline+"\n"+cli)
					err := unionReplaceErr(t, dut, sb)

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
					sb := &gnmi.SetBatch{}
					gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocDelta)
					gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline+"\n"+bgpCLI)
					err := unionReplaceErr(t, dut, sb)

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
					sb := &gnmi.SetBatch{}
					gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocDelta)
					gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline+"\n"+rpCLI)
					setErr := unionReplaceErr(t, dut, sb)

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
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline+"\n"+cli)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocDelta)
				sb.Set(t, dut)
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

				for _, breakoutIntfName := range breakoutInterfaces(t, dut, interfaceWithoutTransceiver, breakoutNumGroups) {
					t.Logf("Adding breakout configuration for interface %s", breakoutIntfName)
					breakoutIntf := ocDelta.GetOrCreateInterface(breakoutIntfName)
					breakoutIntf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
					brEth := breakoutIntf.GetOrCreateEthernet()
					brEth.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB
					brEth.DuplexMode = oc.Ethernet_DuplexMode_FULL
					brEth.AutoNegotiate = ygot.Bool(false)
				}
				t.Log("Push via union_replace and verify accepted")
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocDelta)
				sb.Set(t, dut)

				t.Log("Verify configuration applied and interface oper-status")
				errs = append(errs, verifyBreakoutModeConfig(t, dut, interfaceWithoutTransceiver))
				errs = append(errs, verifyPortSpeed(t, dut, interfaceWithoutTransceiver, oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB))
				errs = append(errs, verifyInterfaceOperStatus(t, dut, interfaceWithoutTransceiver, noTransceiverOperStatus))
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
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline+"\n"+cli)
				sb.Set(t, dut)

				t.Log("Verify configuration applied")
				errs = append(errs, verifyBreakoutModeConfig(t, dut, interfaceWithoutTransceiver))
				errs = append(errs, verifyPortSpeed(t, dut, interfaceWithoutTransceiver, oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB))
				errs = append(errs, verifyInterfaceOperStatus(t, dut, interfaceWithoutTransceiver, noTransceiverOperStatus))
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
