package helpers

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

type Attributes struct {
	IPv4    string
	IPv6    string
	MAC     string
	Name    string // Interface name, only applied to ATE ports.
	Desc    string // Description, only applied to DUT interfaces.
	IPv4Len uint8  // Prefix length for IPv4.
	IPv6Len uint8  // Prefix length for IPv6.
	MTU     uint16
}

type BgpNeighbor struct {
	As         uint32
	NeighborIP string
	IsV4       bool
}

// RemoveInterface returns a new *oc.Interface configured no sub-interface
func RemoveInterface(name string) *oc.Interface {
	intf := &oc.Interface{Name: ygot.String(name)}
	intf.DeleteSubinterface(0)
	return intf
}

// NewInterface returns a new *oc.Interface configured with these attributes
func (a *Attributes) NewInterface(name string) *oc.Interface {
	return a.ConfigInterface(&oc.Interface{Name: ygot.String(name)})
}

// ConfigInterface configures an OpenConfig interface with these attributes.
func (a *Attributes) ConfigInterface(intf *oc.Interface) *oc.Interface {
	if a.Desc != "" {
		intf.Description = ygot.String(a.Desc)
	}
	intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	intf.Enabled = ygot.Bool(true)
	e := intf.GetOrCreateEthernet()
	if a.MAC != "" {
		e.MacAddress = ygot.String(a.MAC)
	}

	s := intf.GetOrCreateSubinterface(0)

	if a.IPv4 != "" {
		s4 := s.GetOrCreateIpv4()
		if a.MTU > 0 {
			s4.Mtu = ygot.Uint16(a.MTU)
		}
		a4 := s4.GetOrCreateAddress(a.IPv4)
		if a.IPv4Len > 0 {
			a4.PrefixLength = ygot.Uint8(a.IPv4Len)
		}
		// a4.AddrType = oc.AristaIntfAugments_AristaAddrType_PRIMARY
	}

	if a.IPv6 != "" {
		s6 := s.GetOrCreateIpv6()
		if a.MTU > 0 {
			s6.Mtu = ygot.Uint32(uint32(a.MTU))
		}

		a6 := s6.GetOrCreateAddress(a.IPv6)
		if a.IPv6Len > 0 {
			a6.PrefixLength = ygot.Uint8(a.IPv6Len)
		}

		s6.AppendAddress(a6)

	}
	return intf
}

// BgpAppendNbr configure BGP Neighbors an OpenConfig BGP Protocol Neighbor with these attributes.
func BgpAppendNbr(as uint32, nbrs []*BgpNeighbor) *oc.NetworkInstance_Protocol_Bgp {
	bgp := &oc.NetworkInstance_Protocol_Bgp{}
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(as)

	for _, nbr := range nbrs {
		if nbr.IsV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.NeighborIP)
			nv4.PeerAs = ygot.Uint32(nbr.As)
			nv4.Enabled = ygot.Bool(true)
			nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.NeighborIP)
			nv6.PeerAs = ygot.Uint32(nbr.As)
			nv6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
		}
	}
	return bgp
}

// BgpDeleteNbr delete BGP Neighbors an OpenConfig BGP Protocol Neighbor with given IP.
func BgpDeleteNbr(nbrs []*BgpNeighbor) *oc.NetworkInstance_Protocol_Bgp {
	bgp := &oc.NetworkInstance_Protocol_Bgp{}
	for _, nbr := range nbrs {
		bgp.DeleteNeighbor(nbr.NeighborIP)
	}
	return bgp
}

func VerifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := dev.Telemetry().Interface(p.Name()).OperStatus().Get(t)
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		} else {
			t.Logf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

// StripLen returns a cidr string with the prefix-length suffix removed.
func StripLen(cidr string) string {
	return strings.Split(cidr, "/")[0]
}

// ygotToText serializes any validatable ygot struct to a JSON string.
// This is mainly useful in tests for debugging, as a convenient way
// to format an OpenConfig struct or telemetry struct.
//
// When used to generate a config, it will return an error if
// validation fails.  Note that ygot.ValidatedGoStruct is a struct
// that can be validated, not one that already has been validated.
func ygotToText(obj ygot.ValidatedGoStruct, config bool) (string, error) {
	return ygot.EmitJSON(obj, &ygot.EmitJSONConfig{
		Format: ygot.RFC7951,
		RFC7951Config: &ygot.RFC7951JSONConfig{
			AppendModuleName: true,
			PreferShadowPath: config,
		},
		Indent:         "  ",
		SkipValidation: !config,
	})
}

// pathToText converts a ygot path to a string.
func pathToText(n ygot.PathStruct) string {
	path, _, errs := ygot.ResolvePath(n)
	if len(errs) > 0 {
		return fmt.Sprintf("<ygot.ResolvePath errs: %v>", errs)
	}
	text, err := ygot.PathToString(path)
	if err != nil {
		return fmt.Sprintf("<ygot.PathToString err: %v>", err)
	}
	return text
}

// sanitizeFilename keeps letters, digits, and safe punctuations, but removes
// unsafe punctuations and other characters.
func sanitizeFilename(filename string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		switch r {
		case '+', ',', '-', '.', ':', ';', '=', '^', '|', '~':
			return r
		case '(', ')', '<', '>', '[', ']', '{', '}':
			return r
		case ' ', '/', '_':
			return '_'
		default:
			return -1 // drop
		}
	}, filename)
}

// outputsDir is the path to the undeclared test outputs directory;
// see Bazel Test Encyclopedia.
// https://docs.bazel.build/versions/main/test-encyclopedia.html
var outputsDir = os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR")

// writeOutput writes content to a file in the undeclared test
// outputs, after sanitizing the filename and making it unique.
func writeOutput(filename, suffix string, content string) error {
	if outputsDir == "" {
		return nil
	}
	template := fmt.Sprintf(
		"%s.%s%s%s",
		sanitizeFilename(filename),
		time.Now().Format("03:04:05"), // order by time to help discovery.
		".*",                          // randomize for os.CreateTemp()
		suffix)
	f, err := os.CreateTemp(outputsDir, template)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write([]byte(content))
	return err
}

// isConfig determines whether the ygot path is defined in Ondatra's
// config package or telemetry package.
func isConfig(path ygot.PathStruct) bool {
	ty := reflect.TypeOf(path)
	if ty.Kind() == reflect.Ptr {
		ty = ty.Elem()
	}
	pkg := ty.PkgPath()
	return strings.Contains(pkg, "/ondatra/config/") ||
		strings.HasSuffix(pkg, "/ondatra")
}

// LogYgot logs a ygot GoStruct at path as either config or telemetry,
// depending on the path.  It also writes a copy to a *.json file in
// the directory specified by the TEST_UNDECLARED_OUTPUTS_DIR
// environment variable.
//
// Ondatra has separate paths for config (dut.Config()) and telemetry
// (dut.Telemetry()), but both share the same GoStruct defined in
// telemetry.  This is why we use the path to decide whether to format
// the object as config or telemetry.  The object alone looks the
// same.
func LogYgot(t testing.TB, what string, path ygot.PathStruct, obj ygot.ValidatedGoStruct) {
	logYgot(t, what, path, obj, true)
}

// WriteYgot is like LogYgot but only writes to undeclared test output
// so it does not pollute the test log.
func WriteYgot(t testing.TB, what string, path ygot.PathStruct, obj ygot.ValidatedGoStruct) {
	logYgot(t, what, path, obj, false)
}

func logYgot(t testing.TB, what string, path ygot.PathStruct, obj ygot.ValidatedGoStruct, shouldLog bool) {
	t.Helper()
	pathText := pathToText(path)
	config := isConfig(path)

	var title string
	if config {
		title = "Config"
	} else {
		title = "Telemetry"
	}

	header := fmt.Sprintf("%s for %s at %s", title, what, pathText)
	text, err := ygotToText(obj, config)
	if err != nil {
		t.Errorf("%s render error: %v", header, err)
	}
	if shouldLog {
		t.Logf("%s:\n%s", header, text)
	}
	if err := writeOutput(t.Name()+" "+header, ".json", text); err != nil {
		t.Logf("Could not write undeclared output: %v", err)
	}
}
