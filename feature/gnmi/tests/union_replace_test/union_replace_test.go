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
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
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

	awaitStateTimeOut = 60 * time.Second
	awaitTimeOut      = 10 * time.Second

	groupIndex = uint8(1)

	addMTUOC     = uint16(2000)
	addMTUCLI    = uint16(2100)
	changeMTUOC1 = uint16(2200)
	changeMTUOC2 = uint16(2300)

	otnChannelIndex = uint32(13030)
	ethChannelIndex = uint32(13034)
	channelAlloc    = float64(400)

	// Nokia SR Linux CLI values.
	// nokiaCLIPrefix is prepended to every Nokia SR Linux flat CLI statement.
	nokiaCLIPrefix   = "set / "
	nokiaBGPRouterID = "192.0.2.1"
)

var (
	sharedBaseline string
	defaultNI      string
)

// breakoutProfile describes the breakout and child port-speed used by gNMI-3.10.
type breakoutProfile struct {
	numPorts    uint8                          // number of breakout child ports
	numChannels uint8                          // physical channels per child port
	ocSpeed     oc.E_IfEthernet_ETHERNET_SPEED // breakout and child port-speed (OC)
	cliSpeed    string                         // child port-speed (CLI), e.g. "50g"
	cliBreakout string                         // breakout speed token (CLI), e.g. "50g-2"
}

var (
	// breakout2x50G is the default gNMI-3.10 breakout.
	breakout2x50G = breakoutProfile{numPorts: 2, numChannels: 2, ocSpeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_50GB, cliSpeed: "50g", cliBreakout: "50g-2"}
	// breakout4x25G is used when the DUT does not support 50G breakout/port-speed.
	breakout4x25G = breakoutProfile{numPorts: 4, numChannels: 1, ocSpeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_25GB, cliSpeed: "25g", cliBreakout: "25g-4"}
)

// breakoutProfileFor returns the gNMI-3.10 (gNMI-3.10.1 and gNMI-3.10.2)
// breakout profile for dut.
func breakoutProfileFor(dut *ondatra.DUTDevice) breakoutProfile {
	switch dut.Vendor() {
	case ondatra.NOKIA:
		return breakout4x25G
	default:
		return breakout2x50G
	}
}

type cliInterfaceConfigOpts struct {
	Name          string
	Description   string
	MTU           uint16
	IPv4          string
	IPv4PrefixLen uint8
	Speed         string
	// BreakoutPorts, when non-zero, requests a breakout of the port into this
	// many child ports of speed Speed.
	BreakoutPorts uint8
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
	if deviations.DuplexModeUnsupported(dut) {
		// attrs.NewOCInterface always sets duplex-mode when Duplex is set.
		i.GetOrCreateEthernet().DuplexMode = oc.Ethernet_DuplexMode_UNSET
	}
	if deviations.ExplicitPortSpeed(dut) {
		i.GetOrCreateEthernet().SetPortSpeed(fptest.GetIfSpeed(t, dp1))
	}
	inf := gnmi.OC().Interface(dp1.Name())
	gnmi.BatchUnionReplace(sb, inf.Config(), i)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		// Push the default network-instance with name, type, and interface.
		id := fmt.Sprintf("%s.%d", dp1.Name(), defaultSubif)
		ni := &oc.NetworkInstance{
			Name: ygot.String(defaultNI),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
		}
		niIntf := ni.GetOrCreateInterface(id)
		niIntf.Interface = ygot.String(dp1.Name())
		niIntf.Subinterface = ygot.Uint32(defaultSubif)
		gnmi.BatchUnionReplace(sb, gnmi.OC().NetworkInstance(defaultNI).Config(), ni)
	}
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
	mtuConfig := gnmi.OC().Interface(portName).Mtu().Config()
	before, hadMTU := gnmi.Lookup(t, dut, mtuConfig).Val()

	var cli string
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli = fmt.Sprintf("configure terminal\ninterface %s\nno mtu\n", portName)
	case ondatra.CISCO:
		cli = fmt.Sprintf("interface %s\nno mtu\n", portName)
	case ondatra.JUNIPER:
		cli = fmt.Sprintf("delete interfaces %s mtu\n", portName)
	case ondatra.NOKIA:
		cli = fmt.Sprintf("delete / interface %s mtu\n", portName)
	default:
		t.Fatalf("unsupported vendor: %v", dut.Vendor())
	}
	helpers.GnmiCLIConfig(t, dut, cli)

	switch dut.Vendor() {
	case ondatra.ARISTA:
		gnmi.Watch(t, dut, gnmi.OC().Interface(portName).Mtu().State(), awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
			m, present := val.Val()
			if !present {
				t.Logf("Got MTU not present, want 1500.")
				return false
			}
			return m == 1500
		}).Await(t)
	case ondatra.CISCO, ondatra.JUNIPER, ondatra.NOKIA:
		if !hadMTU {
			return
		}
		mtuState := gnmi.OC().Interface(portName).Mtu().State()
		if _, ok := gnmi.Watch(t, dut, mtuState, awaitTimeOut, func(val *ygnmi.Value[uint16]) bool {
			m, present := val.Val()
			return !present || m != before
		}).Await(t); !ok {
			t.Fatalf("interface %s MTU %d was not removed within %v", portName, before, awaitTimeOut)
		}
	default:
		t.Fatalf("unsupported vendor: %v", dut.Vendor())
	}
}

// setCLIunionReplace adds any necessary modifications to the base CLI configuration
// for union replace.
func setCLIunionReplace(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		// Add "operation set namespace" command from the CLI config as this is required for Arista
		// when using union replace.
		clicfg1 := "configure terminal\nmanagement api gnmi\n  provider eos-native\n  operation set persistence\n  operation set namespace\n"
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
func stripDescription(t *testing.T, dut *ondatra.DUTDevice, config string) string {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA, ondatra.CISCO:
		re := regexp.MustCompile(`(?m)^\s*description .*$\n?`)
		return re.ReplaceAllString(config, "")
	case ondatra.JUNIPER:
		re := regexp.MustCompile(`(?m)^\s*set\s+interfaces\s+\S+\s+description\s+.*$\n?`)
		return re.ReplaceAllString(config, "")
	case ondatra.NOKIA:
		// Nokia SR Linux "info flat" format, e.g.
		//   set / interface ethernet-1/1/6 description "[AVAILABLE]"
		nokiaRE := regexp.MustCompile(`(?m)^\s*(set\s+)?/\s*interface\s+\S+\s+description\s+.*$\n?`)
		return nokiaRE.ReplaceAllString(config, "")
	default:
		t.Fatalf("unsupported vendor %v for stripDescription", dut.Vendor())
		return config
	}
}

// withoutPortSpeed removes the port-speed line of intfName from a baseline CLI
// config for subtests gNMI-3.10.1 (MissingHardwareOC) and gNMI-3.10.2
// (MissingHardwareCLI) before breakout configuration is added.
func withoutPortSpeed(t *testing.T, dut *ondatra.DUTDevice, config, intfName string) string {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA, ondatra.CISCO:
		return config
	case ondatra.JUNIPER:
		re := regexp.MustCompile(`(?m)^\s*set\s+interfaces\s+` + regexp.QuoteMeta(intfName) + `\s+speed\s+\S+[ \t]*$\n?`)
		return re.ReplaceAllString(config, "")
	case ondatra.NOKIA:
		re := regexp.MustCompile(`(?m)^\s*(set\s+)?/\s*interface\s+` + regexp.QuoteMeta(intfName) + `\s+ethernet\s+port-speed\s+\S+[ \t]*$\n?`)
		return re.ReplaceAllString(config, "")
	default:
		t.Fatalf("unsupported vendor %v for withoutPortSpeed", dut.Vendor())
		return config
	}
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
	return stripDescription(t, dut, config)
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

// firstInterfaceWithoutTransceiver returns a front-panel Ethernet port with no
// transceiver installed, along with the oper-status the DUT reports for it.
func firstInterfaceWithoutTransceiver(t *testing.T, dut *ondatra.DUTDevice) (string, oc.E_Interface_OperStatus) {
	t.Helper()
	reserved := map[string]bool{
		dut.Port(t, port1).Name(): true,
		dut.Port(t, port2).Name(): true,
	}
	wantStatus := operStatusNoTransceiver(t, dut)
	var candidates []*oc.Interface
	for _, intf := range gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State()) {
		if !reserved[intf.GetName()] && isFrontPanelEthernet(dut, intf) {
			candidates = append(candidates, intf)
		}
	}

	for _, intf := range candidates {
		if intf.GetOperStatus() == wantStatus && transceiverMissing(t, dut, intf) {
			return intf.GetName(), wantStatus
		}
	}

	t.Fatalf("unable to find an Ethernet interface with no transceiver connected (checked %d candidate ports, excluding %v)", len(candidates), reserved)
	return "", oc.Interface_OperStatus_UNSET
}

func operStatusNoTransceiver(t *testing.T, dut *ondatra.DUTDevice) oc.E_Interface_OperStatus {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return oc.Interface_OperStatus_NOT_PRESENT
	case ondatra.CISCO, ondatra.JUNIPER, ondatra.NOKIA:
		return oc.Interface_OperStatus_DOWN
	default:
		return oc.Interface_OperStatus_DOWN
	}
}

// isFrontPanelEthernet reports whether intf is a physical Ethernet port,
// excluding management ports.
func isFrontPanelEthernet(dut *ondatra.DUTDevice, intf *oc.Interface) bool {
	switch intf.GetType() {
	case oc.IETFInterfaces_InterfaceType_ethernetCsmacd, oc.IETFInterfaces_InterfaceType_UNSET:
		name := intf.GetName()
		switch dut.Vendor() {
		case ondatra.ARISTA:
			return strings.HasPrefix(name, "Ethernet")
		case ondatra.CISCO:
			return strings.HasPrefix(name, "HundredGigE") || strings.HasPrefix(name, "FourHundredGigE")
		case ondatra.JUNIPER:
			return strings.HasPrefix(name, "et-")
		case ondatra.NOKIA:
			return strings.HasPrefix(name, "ethernet-")
		default:
			return strings.HasPrefix(strings.ToLower(name), "ethernet")
		}
	default:
		return false
	}
}

// transceiverMissing reports whether the interface has no transceiver
// installed. It prefers the transceiver component's "empty" leaf and falls
// back to the absence of physical channels when that leaf is not available.
func transceiverMissing(t *testing.T, dut *ondatra.DUTDevice, intf *oc.Interface) bool {
	t.Helper()
	tr := intf.GetTransceiver()
	if tr != "" {
		var empty, present bool
		if fatalMsg := testt.CaptureFatal(t, func(t testing.TB) {
			empty, present = gnmi.Lookup(t, dut, gnmi.OC().Component(tr).Empty().State()).Val()
		}); fatalMsg == nil && present {
			return empty
		}
	}
	return len(intf.GetPhysicalChannel()) == 0
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
		return "info flat", ""
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
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			id := fmt.Sprintf("%s.%d", intfName, subif)
			ni := root.GetOrCreateNetworkInstance(defaultNI)
			ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
			niIntf := ni.GetOrCreateInterface(id)
			niIntf.Interface = ygot.String(intfName)
			niIntf.Subinterface = ygot.Uint32(subif)
		}
	}
	return intf
}

// ocInterface returns the named interface from root with its type and, when
// required by deviation, enabled leaf set.
func ocInterface(root *oc.Root, dut *ondatra.DUTDevice, intfName string) *oc.Interface {
	intf := root.GetOrCreateInterface(intfName)
	intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		intf.Enabled = ygot.Bool(true)
	}
	return intf
}

// batchUnionReplaceOCInterface adds an OC union_replace of the interface
// intfName to sb, initialized via ocInterface and customized by set.
func batchUnionReplaceOCInterface(sb *gnmi.SetBatch, dut *ondatra.DUTDevice, intfName string, set func(*oc.Interface)) {
	intf := ocInterface(&oc.Root{}, dut, intfName)
	set(intf)
	gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(intfName).Config(), intf)
}

// nonExistentInterface returns an interface name that does not exist on the
// DUT, formatted according to the DUT vendor's naming convention.
func nonExistentInterface(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return nonExistentIntf
	case ondatra.CISCO, ondatra.JUNIPER, ondatra.NOKIA:
		p1 := dut.Port(t, port1).Name()
		return regexp.MustCompile(`\d+`).ReplaceAllString(p1, "99")
	default:
		t.Fatalf("unsupported vendor %v for nonExistentInterface", dut.Vendor())
		return ""
	}
}

// cliInterface returns the CLI lines for intfName.
func cliInterface(t *testing.T, dut *ondatra.DUTDevice, cli, intfName string) string {
	t.Helper()
	var sb strings.Builder
	switch dut.Vendor() {
	case ondatra.ARISTA, ondatra.CISCO:
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
	case ondatra.JUNIPER:
		setRE := regexp.MustCompile(`^\s*set\s+interfaces\s+` + regexp.QuoteMeta(intfName) + `(\s|$)`)
		for _, line := range strings.Split(cli, "\n") {
			if setRE.MatchString(line) {
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
	case ondatra.NOKIA:
		flatRE := regexp.MustCompile(`^\s*(set\s+)?/\s*interface\s+` + regexp.QuoteMeta(intfName) + `(\s|$)`)
		for _, line := range strings.Split(cli, "\n") {
			if flatRE.MatchString(line) {
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
	default:
		t.Fatalf("unsupported vendor %v for cliInterface", dut.Vendor())
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
	interfaceConfig := cliInterface(t, dut, cliConfig(t, dut), intfName)
	var re *regexp.Regexp
	switch dut.Vendor() {
	case ondatra.ARISTA, ondatra.CISCO:
		re = regexp.MustCompile(fmt.Sprintf(`(?m)^\s*mtu\s+%d\s*$`, wantMTU))
	case ondatra.JUNIPER, ondatra.NOKIA:
		// Match "mtu <N>" as a whole word so that e.g. "ip-mtu <N>" is not matched.
		re = regexp.MustCompile(fmt.Sprintf(`(?m)(^|\s)mtu\s+%d(\s|$)`, wantMTU))
	default:
		t.Fatalf("unsupported vendor %v for verifyCLIMTU", dut.Vendor())
		return false
	}
	if re.MatchString(interfaceConfig) {
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
	case ondatra.CISCO:
		var sb strings.Builder
		fmt.Fprintf(&sb, "interface %s\n", opts.Name)
		if opts.Description != "" {
			fmt.Fprintf(&sb, " description %s\n", opts.Description)
		}
		if opts.MTU > 0 {
			fmt.Fprintf(&sb, " mtu %d\n", opts.MTU)
		}
		if opts.IPv4 != "" && opts.IPv4PrefixLen > 0 {
			fmt.Fprintf(&sb, " ipv4 address %s/%d\n", opts.IPv4, opts.IPv4PrefixLen)
		}
		if opts.Speed != "" {
			fmt.Fprintf(&sb, " speed %s\n", opts.Speed)
		}
		fmt.Fprintf(&sb, "!\n")
		return sb.String()
	case ondatra.JUNIPER:
		var sb strings.Builder
		p := "set interfaces " + opts.Name
		if opts.Description != "" {
			fmt.Fprintf(&sb, "%s description %q\n", p, opts.Description)
		}
		if opts.MTU > 0 {
			fmt.Fprintf(&sb, "%s mtu %d\n", p, opts.MTU)
		}
		if opts.IPv4 != "" && opts.IPv4PrefixLen > 0 {
			fmt.Fprintf(&sb, "%s unit %d family inet address %s/%d\n", p, defaultSubif, opts.IPv4, opts.IPv4PrefixLen)
		}
		if opts.Speed != "" {
			fmt.Fprintf(&sb, "%s speed %s\n", p, opts.Speed)
		}
		return sb.String()
	case ondatra.NOKIA:
		// SR Linux flat CLI, one statement per line.
		var sb strings.Builder
		p := nokiaCLIPrefix + "interface " + opts.Name
		if opts.BreakoutPorts == 0 {
			fmt.Fprintf(&sb, "%s admin-state enable\n", p)
		}
		if opts.Description != "" {
			fmt.Fprintf(&sb, "%s description %q\n", p, opts.Description)
		}
		if opts.MTU > 0 {
			fmt.Fprintf(&sb, "%s mtu %d\n", p, opts.MTU)
		}
		if opts.IPv4 != "" && opts.IPv4PrefixLen > 0 {
			fmt.Fprintf(&sb, "%s subinterface %d admin-state enable\n", p, defaultSubif)
			fmt.Fprintf(&sb, "%s subinterface %d ipv4 admin-state enable\n", p, defaultSubif)
			fmt.Fprintf(&sb, "%s subinterface %d ipv4 address %s/%d\n", p, defaultSubif, opts.IPv4, opts.IPv4PrefixLen)
			fmt.Fprintf(&sb, "%snetwork-instance %s interface %s.%d\n", nokiaCLIPrefix, defaultNI, opts.Name, defaultSubif)
		}
		if opts.BreakoutPorts > 0 {
			// e.g. "50g-2" -> "50G"
			speed := strings.ToUpper(strings.SplitN(opts.Speed, "-", 2)[0])
			fmt.Fprintf(&sb, "%s breakout-mode num-breakout-ports %d breakout-port-speed %s\n", p, opts.BreakoutPorts, speed)
		} else if opts.Speed != "" {
			fmt.Fprintf(&sb, "%s ethernet port-speed %s\n", p, strings.ToUpper(opts.Speed))
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
	case ondatra.CISCO, ondatra.NOKIA:
		for index := range noOfIntfs {
			intfs = append(intfs, fmt.Sprintf("%s/%d", baseIntfName, index+1))
		}
	case ondatra.JUNIPER:
		for index := range noOfIntfs {
			intfs = append(intfs, fmt.Sprintf("%s:%d", baseIntfName, index))
		}
	default:
		t.Logf("unsupported vendor %v for breakout interfaces", dut.Vendor())
	}
	return intfs
}

// missingHWCheckIntf returns the interface whose port-speed and oper-status are
// verified in gNMI-3.10. Normally the base interface is itself a breakout lane.
// When breakout creates new child interfaces (ExplicitBreakoutInterfaceConfig),
// the first child interface is checked instead.
func missingHWCheckIntf(t *testing.T, dut *ondatra.DUTDevice, baseIntfName string) string {
	t.Helper()
	if deviations.ExplicitBreakoutInterfaceConfig(dut) {
		if intfs := breakoutInterfaces(t, dut, baseIntfName, breakoutProfileFor(dut).numPorts); len(intfs) > 0 {
			return intfs[0]
		}
	}
	return baseIntfName
}

func ocBreakoutMode(t *testing.T, dut *ondatra.DUTDevice, root *oc.Root, intfName string) {
	t.Helper()
	hwPort, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(intfName).HardwarePort().State()).Val()
	if !ok || hwPort == "" {
		t.Logf("Skipping breakout-mode OC config: no hardware-port for %s", intfName)
		return
	}
	bp := breakoutProfileFor(dut)
	comp := root.GetOrCreateComponent(hwPort)
	grp := comp.GetOrCreatePort().GetOrCreateBreakoutMode().GetOrCreateGroup(groupIndex)
	grp.Index = ygot.Uint8(1)
	grp.NumBreakouts = ygot.Uint8(bp.numPorts)
	grp.BreakoutSpeed = bp.ocSpeed
	grp.NumPhysicalChannels = ygot.Uint8(bp.numChannels)
}

func cliBreakoutMode(t *testing.T, dut *ondatra.DUTDevice, intfName string) string {
	bp := breakoutProfileFor(dut)
	return cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: intfName, Speed: bp.cliBreakout, BreakoutPorts: bp.numPorts})
}

func verifyBreakoutModeConfig(t *testing.T, dut *ondatra.DUTDevice, intfName string) error {
	t.Helper()
	hwPort, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(intfName).HardwarePort().State()).Val()
	if !ok || hwPort == "" {
		t.Logf("Skipping breakout-mode config verification: no hardware-port for %s", intfName)
		return nil
	}
	bp := breakoutProfileFor(dut)
	var errs []error
	grp := gnmi.OC().Component(hwPort).Port().BreakoutMode().Group(groupIndex)
	if got, ok := gnmi.Lookup(t, dut, grp.NumBreakouts().Config()).Val(); ok {
		t.Logf("component %s breakout group num-breakouts: %d", hwPort, got)
	} else {
		errs = append(errs, fmt.Errorf("component %s breakout group num-breakouts: not present", hwPort))
	}
	if got, ok := gnmi.Lookup(t, dut, grp.BreakoutSpeed().Config()).Val(); !ok || got != bp.ocSpeed {
		errs = append(errs, fmt.Errorf("component %s breakout group breakout-speed: got %v (present=%v), want %v", hwPort, got, ok, bp.ocSpeed))
	}
	if got, ok := gnmi.Lookup(t, dut, grp.NumPhysicalChannels().Config()).Val(); !ok || got != bp.numChannels {
		errs = append(errs, fmt.Errorf("component %s breakout group num-physical-channels: got %v (present=%v), want %d", hwPort, got, ok, bp.numChannels))
	}
	return errors.Join(errs...)
}

func otnLogicalChannelsConfig(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) *oc.Root {
	t.Helper()
	params := &cfgplugins.ConfigParameters{
		TribProtocol:        oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE,
		RateClass:           oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G,
		Allocation:          channelAlloc,
		TransceiverNames:    map[string]string{},
		OpticalChannelNames: map[string]string{p.Name(): components.OpticalChannelComponentFromPort(t, dut, p)},
		OTNIndexes:          map[string]uint32{p.Name(): otnChannelIndex},
		ETHIndexes:          map[string]uint32{p.Name(): ethChannelIndex},
	}
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		params.TransceiverNames[p.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
	}
	root := &oc.Root{}
	root.GetOrCreateTerminalDevice().Channel = map[uint32]*oc.TerminalDevice_Channel{
		otnChannelIndex: cfgplugins.OTNChannelConfig(dut, p, params),
		ethChannelIndex: cfgplugins.ETHChannelConfig(dut, p, params),
	}
	return root
}

func verifyLogicalChannels(t *testing.T, dut *ondatra.DUTDevice, want *oc.Root) error {
	t.Helper()
	var errs []error
	for idx, wantCh := range want.GetTerminalDevice().Channel {
		got, ok := gnmi.Lookup(t, dut, gnmi.OC().TerminalDevice().Channel(idx).Config()).Val()
		if !ok {
			errs = append(errs, fmt.Errorf("logical channel %d config not present", idx))
			continue
		}
		if lcJSON, err := ygot.EmitJSON(got, &ygot.EmitJSONConfig{
			Format: ygot.RFC7951,
			Indent: "  ",
			RFC7951Config: &ygot.RFC7951JSONConfig{
				AppendModuleName: true,
			},
		}); err == nil {
			t.Logf("Received LogicalChannel %d config: %v", idx, lcJSON)
		}
		if got.GetLogicalChannelType() != wantCh.GetLogicalChannelType() {
			errs = append(errs, fmt.Errorf("logical channel %d type: got %v, want %v", idx, got.GetLogicalChannelType(), wantCh.GetLogicalChannelType()))
		}
		if !deviations.EthChannelIngressParametersUnsupported(dut) {
			if got.GetIngress().GetInterface() != wantCh.Ingress.GetInterface() || got.GetIngress().GetTransceiver() != wantCh.Ingress.GetTransceiver() {
				errs = append(errs, fmt.Errorf("logical channel %d ingress: got %v, want %v", idx, got.GetIngress(), wantCh.Ingress))
			}
		}
		for aIdx, wantA := range wantCh.Assignment {
			gotA := got.GetAssignment(aIdx)
			if gotA == nil {
				errs = append(errs, fmt.Errorf("logical channel %d assignment %d not present", idx, aIdx))
				continue
			}
			if gotA.GetAssignmentType() != wantA.GetAssignmentType() || gotA.GetOpticalChannel() != wantA.GetOpticalChannel() || gotA.GetLogicalChannel() != wantA.GetLogicalChannel() {
				errs = append(errs, fmt.Errorf("logical channel %d assignment %d: got %v, want %v", idx, aIdx, gotA, wantA))
			}
		}
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
	interfaceWithoutTransceiver, noTransceiverOperStatus := firstInterfaceWithoutTransceiver(t, dut)
	t.Logf("First interface without transceiver: %s (oper-status %v)", interfaceWithoutTransceiver, noTransceiverOperStatus)
	// resetConfig takes the subtest's t so that a failed reset fails only that
	// subtest instead of calling FailNow on the parent test from a subtest.
	resetConfig := func(t *testing.T) {
		t.Helper()
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
				mtu := addMTUOC
				cliConfig2 := cliConfig(t, dut)
				gnmi.BatchUnionReplaceCLI(sb1, cliOrigin, cliConfig2)
				batchUnionReplaceOCInterface(sb1, dut, dp1.Name(), func(i *oc.Interface) { i.Mtu = ygot.Uint16(mtu) })
				t.Logf("Generated BatchUnionReplace: %#v\n", sb1.String())

				setResult := sb1.Set(t, dut)
				t.Logf("\nSetResult: %#v\n", prettyPrintYgnmiResult(setResult))

				return verifyInterfaceMTU(t, dut, dp1.Name(), mtu)
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
				ocMTU := addMTUOC
				cliMTU := addMTUCLI
				cliConfig1 := cliConfig(t, dut)
				gnmi.BatchUnionReplaceCLI(sb1, cliOrigin, cliConfig1)
				batchUnionReplaceOCInterface(sb1, dut, dp2.Name(), func(i *oc.Interface) { i.Mtu = ygot.Uint16(ocMTU) })
				sb1.Set(t, dut)
				if err := verifyInterfaceMTU(t, dut, dp2.Name(), ocMTU); err != nil {
					return err
				}

				// Change the MTU to a different value using CLI config.
				cliConfig2 := cliConfig(t, dut)
				switch dut.Vendor() {
				case ondatra.ARISTA:
					cliConfig2 += fmt.Sprintf("interface %s\nmtu %d\n", dp2.Name(), cliMTU)
				case ondatra.CISCO:
					cliConfig2 += fmt.Sprintf("interface %s\nmtu %d\n", dp2.Name(), cliMTU)
				case ondatra.JUNIPER:
					cliConfig2 += fmt.Sprintf("set interfaces %s mtu %d\n", dp2.Name(), cliMTU)
				case ondatra.NOKIA:
					cliConfig2 += fmt.Sprintf("%sinterface %s mtu %d\n", nokiaCLIPrefix, dp2.Name(), cliMTU)
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
					if err := verifyInterfaceMTU(t, dut, dp2.Name(), cliMTU); err != nil {
						return err
					}
				case ondatra.CISCO, ondatra.JUNIPER, ondatra.NOKIA:
					// OC and CLI conflict generates an error, MTU stays at the OC value
					if err := verifyInterfaceMTU(t, dut, dp2.Name(), ocMTU); err != nil {
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
				mtu1 := changeMTUOC1
				mtu2 := changeMTUOC2
				cliConfig1 := cliConfig(t, dut)
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, cliConfig1)
				batchUnionReplaceOCInterface(sb, dut, portName, func(i *oc.Interface) { i.Mtu = ygot.Uint16(mtu1) })
				sb.Set(t, dut)

				if err := verifyInterfaceMTU(t, dut, portName, mtu1); err != nil {
					return err
				}

				// Change the MTU using OC config.
				// reuse the same CLI config without any MTU config.
				sb2 := &gnmi.SetBatch{}
				batchUnionReplaceOCInterface(sb2, dut, portName, func(i *oc.Interface) { i.Mtu = ygot.Uint16(mtu2) })
				gnmi.BatchUnionReplaceCLI(sb2, cliOrigin, cliConfig1)
				sb2.Set(t, dut)

				return verifyInterfaceMTU(t, dut, portName, mtu2)
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
				batchUnionReplaceOCInterface(sb1, dut, port1Name, func(i *oc.Interface) { i.Description = ygot.String(port1DescriptionOC) })
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
				switch dut.Vendor() {
				case ondatra.ARISTA, ondatra.CISCO:
					cliConfig2 += fmt.Sprintf("interface %s\ndescription %s\n", port1Name, port1DescriptionCLI)
				case ondatra.JUNIPER:
					cliConfig2 += fmt.Sprintf("set interfaces %s description %q\n", port1Name, port1DescriptionCLI)
				case ondatra.NOKIA:
					cliConfig2 += fmt.Sprintf("%sinterface %s description %q\n", nokiaCLIPrefix, port1Name, port1DescriptionCLI)
				default:
					return fmt.Errorf("unsupported vendor: %v", dut.Vendor())
				}
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
				ocInterface(bothOC, dut, intf1Name).Description = ygot.String(descIntf1Present)
				ocInterface(bothOC, dut, intf2Name).Description = ygot.String(descIntf2Present)
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), bothOC)
				sb.Set(t, dut)
				errs = append(errs, verifyInterfaceDescription(t, dut, intf1Name, descIntf1Present))
				errs = append(errs, verifyInterfaceDescription(t, dut, intf2Name, descIntf2Present))

				t.Log("Omit interface 2 in OC union_replace")
				intf1Only := &oc.Root{}
				ocInterface(intf1Only, dut, intf1Name).Description = ygot.String(descIntf1Present)
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
				ocInterface(ocConfig, dut, intf2Name).Description = ygot.String(descMoveNoIP)
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocConfig)
				sb.Set(t, dut)
				errs = append(errs, verifyInterfaceIP(t, dut, intf1Name, port1IPv4, ipv4PrefixLen))

				t.Log("Move IP from port1 to port2")
				ocConfig = &oc.Root{}
				ocInterface(ocConfig, dut, intf1Name).Description = ygot.String(descMoveIPMoved)
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
				if !deviations.DuplexModeUnsupported(dut) {
					gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(dp1.Name()).Ethernet().DuplexMode().Config(), oc.Ethernet_DuplexMode_FULL)
				}
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
				case ondatra.NOKIA:
					clicfg1 += fmt.Sprintf("%sinterface %s ethernet port-speed 10G\n", nokiaCLIPrefix, dp1.Name())
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
				if !deviations.DuplexModeUnsupported(dut) {
					gnmi.BatchUnionReplace(sb, gnmi.OC().Interface(dp1.Name()).Ethernet().DuplexMode().Config(), oc.Ethernet_DuplexMode_FULL)
				}
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
				nonExistentIntf := nonExistentInterface(t, dut)
				t.Logf("Using non-existent interface %s", nonExistentIntf)

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
					case ondatra.ARISTA, ondatra.CISCO:
						badCLI = fmt.Sprintf("interface %s\n  mtu %d\n", nonExistentIntf, badIntfMTU)
					case ondatra.JUNIPER:
						badCLI = fmt.Sprintf("set interfaces %s mtu %d\n", nonExistentIntf, badIntfMTU)
					case ondatra.NOKIA:
						badCLI = fmt.Sprintf("%sinterface %s mtu %d\n", nokiaCLIPrefix, nonExistentIntf, badIntfMTU)
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
					ocInterface(ocDelta, dut, intf1Name).Mtu = ygot.Uint16(overlapMTUOC)
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
					ocInterface(ocDelta, dut, intf1Name).Mtu = ygot.Uint16(overlapMTUOC)
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
					global := bgp.GetOrCreateGlobal()
					global.As = ygot.Uint32(bgpASOC)

					var bgpCLI string
					switch dut.Vendor() {
					case ondatra.ARISTA, ondatra.CISCO:
						bgpCLI = fmt.Sprintf("router bgp %d\n", bgpASCLI)
					case ondatra.JUNIPER:
						bgpCLI = fmt.Sprintf("set routing-options autonomous-system %d\n", bgpASCLI)
					case ondatra.NOKIA:
						global.RouterId = ygot.String(nokiaBGPRouterID)
						global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
						bgpPath := fmt.Sprintf("%snetwork-instance %s protocols bgp", nokiaCLIPrefix, defaultNI)
						bgpCLI = fmt.Sprintf("%[1]s admin-state enable\n%[1]s autonomous-system %[2]d\n%[1]s router-id %[3]s\n%[1]s afi-safi ipv4-unicast admin-state enable\n",
							bgpPath, bgpASCLI, nokiaBGPRouterID)
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
					case ondatra.CISCO:
						rpCLI = fmt.Sprintf("route-policy %s\n  pass\nend-policy\n", policyName)
					case ondatra.JUNIPER:
						rpCLI = fmt.Sprintf("set policy-options policy-statement %s term 10 then accept\n", policyName)
					case ondatra.NOKIA:
						rpCLI = fmt.Sprintf("%srouting-policy policy %s statement 10 action policy-result accept\n", nokiaCLIPrefix, policyName)
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
				p1Intf := ocInterface(ocDelta, dut, intf1Name)
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
				bp := breakoutProfileFor(dut)
				ocDelta := &oc.Root{}
				ocBreakoutMode(t, dut, ocDelta, interfaceWithoutTransceiver)

				for _, breakoutIntfName := range breakoutInterfaces(t, dut, interfaceWithoutTransceiver, bp.numPorts) {
					t.Logf("Adding breakout configuration for interface %s", breakoutIntfName)
					breakoutIntf := ocDelta.GetOrCreateInterface(breakoutIntfName)
					breakoutIntf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
					brEth := breakoutIntf.GetOrCreateEthernet()
					brEth.PortSpeed = bp.ocSpeed
					if !deviations.DuplexModeUnsupported(dut) {
						brEth.DuplexMode = oc.Ethernet_DuplexMode_FULL
					}
					if !deviations.AutoNegotiateUnsupported(dut) {
						brEth.AutoNegotiate = ygot.Bool(false)
					}
				}
				t.Log("Push via union_replace and verify accepted")
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, withoutPortSpeed(t, dut, sharedBaseline, interfaceWithoutTransceiver))
				gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocDelta)
				sb.Set(t, dut)

				t.Log("Verify configuration applied and interface oper-status")
				checkIntf := missingHWCheckIntf(t, dut, interfaceWithoutTransceiver)
				errs = append(errs, verifyBreakoutModeConfig(t, dut, interfaceWithoutTransceiver))
				errs = append(errs, verifyPortSpeed(t, dut, checkIntf, bp.ocSpeed))
				errs = append(errs, verifyInterfaceOperStatus(t, dut, checkIntf, noTransceiverOperStatus))
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
				bp := breakoutProfileFor(dut)
				checkIntf := missingHWCheckIntf(t, dut, interfaceWithoutTransceiver)
				var cli string
				switch dut.Vendor() {
				case ondatra.ARISTA, ondatra.CISCO, ondatra.JUNIPER:
					cli = cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: interfaceWithoutTransceiver, Speed: bp.cliSpeed})
					if breakoutCLI := cliBreakoutMode(t, dut, interfaceWithoutTransceiver); breakoutCLI != "" {
						cli += "\n" + breakoutCLI
					}
				case ondatra.NOKIA:
					// Breakout creates new child interfaces, so configure the breakout
					// first and then set the port-speed on the child interface.
					cli = cliBreakoutMode(t, dut, interfaceWithoutTransceiver) + "\n" +
						cliInterfaceConfig(t, dut, cliInterfaceConfigOpts{Name: checkIntf, Speed: bp.cliSpeed})
				default:
					return fmt.Errorf("unsupported vendor %v for MissingHardwareCLI", dut.Vendor())
				}

				t.Log("Push via union_replace with CLI and verify accepted")
				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, withoutPortSpeed(t, dut, sharedBaseline, interfaceWithoutTransceiver)+"\n"+cli)
				sb.Set(t, dut)

				t.Log("Verify configuration applied")
				errs = append(errs, verifyBreakoutModeConfig(t, dut, interfaceWithoutTransceiver))
				errs = append(errs, verifyPortSpeed(t, dut, checkIntf, bp.ocSpeed))
				errs = append(errs, verifyInterfaceOperStatus(t, dut, checkIntf, noTransceiverOperStatus))
				return errors.Join(errs...)
			},
		},
		{
			name: "gNMI-3.11-OTNLogicalChannelsOC",
			desc: "Configure OTN and ETH logical channels using OC via union_replace; config must be applied and readable.",
			fn: func(t *testing.T) error {
				dut := ondatra.DUT(t, "dut")
				ocConfig := otnLogicalChannelsConfig(t, dut, dut.Port(t, port1))

				sb := &gnmi.SetBatch{}
				gnmi.BatchUnionReplaceCLI(sb, cliOrigin, sharedBaseline)
				gnmi.BatchUnionReplace(sb, gnmi.OC().Config(), ocConfig)
				sb.Set(t, dut)
				return verifyLogicalChannels(t, dut, ocConfig)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() { resetConfig(t) })
			t.Log(tc.desc)
			if err := tc.fn(t); err != nil {
				t.Errorf("%v", err)
			}
		})
	}
}
