// Copyright 2026 Google LLC
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

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var stubSources = map[string]string{
	"github.com/openconfig/ondatra/binding/ixweb": `
package ixweb
type IxWeb struct{}
type Session struct{}
`,
	"github.com/openconfig/ondatra/binding": `
package binding
import (
	"context"
	"github.com/openconfig/ondatra/binding/ixweb"
)
type IxNetwork struct {
	Session *ixweb.Session
}
type ATE interface {
	DialIxNetwork(context.Context) (*IxNetwork, error)
	DialOTG(context.Context) error
}
`,
	"github.com/openconfig/ondatra/ixnet": `
package ixnet
type BGPPeer struct{}
`,
	"github.com/openconfig/ondatra/internal/ixconfig": `
package ixconfig
type Ixnetwork struct{}
`,
	"github.com/openconfig/ondatra/internal/ate": `
package ate
type Topology struct{}
`,
	"github.com/openconfig/ondatra/otg": `
package otg
import "testing"
type OTG struct{}
func (*OTG) PushConfig(testing.TB) {}
func (*OTG) StartProtocols(testing.TB) {}
func (*OTG) StartTraffic(testing.TB) {}
`,
	"github.com/openconfig/ondatra/raw": `
package raw
import "github.com/openconfig/ondatra/binding"
type ATEAPIs struct{}
func (*ATEAPIs) BindingATE() binding.ATE { return nil }
`,
	"github.com/openconfig/ondatra": `
package ondatra
import (
	"testing"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ondatra/raw"
)
type Device struct{}
func (*Device) Port(testing.TB, string) *Port { return nil }
func (*Device) Ports() []*Port { return nil }
type Port struct{}
func (*Port) ID() string { return "" }
type DUTDevice struct { *Device }
type ATEDevice struct { *Device }
func (*ATEDevice) OTG() *otg.OTG { return nil }
func (*ATEDevice) Topology() *Topology { return nil }
func (*ATEDevice) Traffic() *Traffic { return nil }
func (*ATEDevice) Actions() *Actions { return nil }
func (*ATEDevice) RawAPIs() *raw.ATEAPIs { return nil }
func ATE(testing.TB, string) *ATEDevice { return nil }
func DUT(testing.TB, string) *DUTDevice { return nil }
func RunTests(*testing.M, func() error) {}

type Topology struct{}
func (*Topology) New() *ATETopology { return nil }
type ATETopology struct{}
func (*ATETopology) AddInterface(string) *Interface { return nil }
func (*ATETopology) AddLAG(string) *LAG { return nil }
type Interface struct{}
type LAG struct{}
type LACP struct{}

type Traffic struct{}
func (*Traffic) NewFlow(string) *Flow { return nil }
type Flow struct{}
type Endpoint interface{ EndpointPB() }
type Header interface{ header() }
type EthernetHeader struct{}
func (*EthernetHeader) header() {}
func NewEthernetHeader() *EthernetHeader { return nil }
type IPv4Header struct{}
func (*IPv4Header) header() {}
func NewIPv4Header() *IPv4Header { return nil }
type IPv6Header struct{}
func (*IPv6Header) header() {}
func NewIPv6Header() *IPv6Header { return nil }
type MPLSHeader struct{}
func (*MPLSHeader) header() {}
func NewMPLSHeader() *MPLSHeader { return nil }
type TCPHeader struct{}
func (*TCPHeader) header() {}
func NewTCPHeader() *TCPHeader { return nil }
type UDPHeader struct{}
func (*UDPHeader) header() {}
func NewUDPHeader() *UDPHeader { return nil }
type ICMPHeader struct{}
func (*ICMPHeader) header() {}
func NewICMPHeader() *ICMPHeader { return nil }
type GREHeader struct{}
func (*GREHeader) header() {}
func NewGREHeader() *GREHeader { return nil }
type MACsecHeader struct{}
func (*MACsecHeader) header() {}
func NewMACsecHeader() *MACsecHeader { return nil }

type Actions struct{}
func (*Actions) NewSetPortState() *SetPortState { return nil }
func (*Actions) NewSetLACPState() *SetLACPState { return nil }
type SetPortState struct{}
type SetLACPState struct{}
`,
	"github.com/open-traffic-generator/snappi/gosnappi": `
package gosnappi
type Config interface {
	Flows() []string
}
func NewConfig() Config { return nil }
type ControlTraffic interface {
	Start()
}
type ControlState interface {
	Traffic() ControlTraffic
}
func NewControlState() ControlState { return nil }
`,
	"github.com/openconfig/ondatra/gnmi/oc": `
package oc
type ActionsObj struct{}
type Statement struct{}
func (*Statement) Actions() *ActionsObj { return nil }
func (*Statement) GetOrCreateActions() *ActionsObj { return nil }
func (*Statement) Topology() string { return "" }
`,
}

type stubImporter struct {
	mu      sync.Mutex
	fset    *token.FileSet
	stdImp  types.Importer
	checked map[string]*types.Package
}

func newStubImporter() *stubImporter {
	fset := token.NewFileSet()
	return &stubImporter{
		fset:    fset,
		stdImp:  importer.Default(),
		checked: make(map[string]*types.Package),
	}
}

func (s *stubImporter) Import(path string) (*types.Package, error) {
	s.mu.Lock()
	if pkg, ok := s.checked[path]; ok {
		s.mu.Unlock()
		return pkg, nil
	}
	s.mu.Unlock()

	src, ok := stubSources[path]
	if !ok {
		return s.stdImp.Import(path)
	}

	f, err := parser.ParseFile(s.fset, path+".go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing stub %s: %w", path, err)
	}
	cfg := &types.Config{Importer: s}
	pkg, err := cfg.Check(path, s.fset, []*ast.File{f}, nil)
	if err != nil {
		return nil, fmt.Errorf("type-checking stub %s: %w", path, err)
	}

	s.mu.Lock()
	s.checked[path] = pkg
	s.mu.Unlock()
	return pkg, nil
}

var sharedStubImp = newStubImporter()

func analyzeSnippet(t *testing.T, pkgPath, src string) []diagnostic {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "subject_test.go", src, 0)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	rawDiags, err := typeCheckAndInspect(fset, pkgPath, pkgPath, []*ast.File{f}, sharedStubImp)
	if err != nil {
		t.Fatalf("typeCheckAndInspect(%s) failed: %v", pkgPath, err)
	}
	var out []diagnostic
	for _, rd := range rawDiags {
		pos := fset.Position(rd.pos)
		out = append(out, diagnostic{
			File:    pos.Filename,
			Line:    pos.Line,
			Col:     pos.Column,
			Message: rd.message,
		})
	}
	return sortAndDedupeDiagnostics(out)
}

func TestLegacyATERejected(t *testing.T) {
	tests := []struct {
		name        string
		pkgPath     string
		src         string
		wantSubstrs []string
	}{
		{
			name:    "ate.Topology call",
			pkgPath: "github.com/openconfig/featureprofiles/feature/foo/otg_tests/foo_test",
			src: `package foo_test
import (
	"testing"
	"github.com/openconfig/ondatra"
)
func TestTopology(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	ate.Topology().New()
}
`,
			wantSubstrs: []string{"(*ondatra.ATEDevice).Topology"},
		},
		{
			name:    "ate.Traffic call",
			pkgPath: "github.com/openconfig/featureprofiles/feature/foo/otg_tests/foo_test",
			src: `package foo_test
import (
	"testing"
	"github.com/openconfig/ondatra"
)
func TestTraffic(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	ate.Traffic().NewFlow("f1")
}
`,
			wantSubstrs: []string{"(*ondatra.ATEDevice).Traffic"},
		},
		{
			name:    "ate.Actions call",
			pkgPath: "github.com/openconfig/featureprofiles/feature/foo/otg_tests/foo_test",
			src: `package foo_test
import (
	"testing"
	"github.com/openconfig/ondatra"
)
func TestActions(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	ate.Actions().NewSetPortState()
}
`,
			wantSubstrs: []string{"(*ondatra.ATEDevice).Actions"},
		},
		{
			name:    "method value and method expression on ATEDevice",
			pkgPath: "github.com/openconfig/featureprofiles/internal/helper",
			src: `package helper
import "github.com/openconfig/ondatra"
func capture(ate *ondatra.ATEDevice) {
	fn := ate.Topology
	_ = fn
	expr := (*ondatra.ATEDevice).Actions
	_ = expr
}
`,
			wantSubstrs: []string{
				"(*ondatra.ATEDevice).Topology",
				"(*ondatra.ATEDevice).Actions",
			},
		},
		{
			name:    "struct field and embedded ATEDevice",
			pkgPath: "github.com/openconfig/featureprofiles/internal/helper",
			src: `package helper
import "github.com/openconfig/ondatra"
type testCase struct {
	ate *ondatra.ATEDevice
}
type embeddedATE struct {
	*ondatra.ATEDevice
}
func runCase(tc testCase, emb embeddedATE) {
	tc.ate.Traffic()
	emb.Topology()
}
`,
			wantSubstrs: []string{
				"(*ondatra.ATEDevice).Traffic",
				"(*ondatra.ATEDevice).Topology",
			},
		},
		{
			name:    "legacy header constructors and types",
			pkgPath: "github.com/openconfig/featureprofiles/feature/bar/otg_tests/bar_test",
			src: `package bar_test
import "github.com/openconfig/ondatra"
func buildHeaders() {
	_ = ondatra.NewEthernetHeader()
	_ = ondatra.NewIPv4Header()
	_ = ondatra.NewIPv6Header()
	_ = ondatra.NewMPLSHeader()
	_ = ondatra.NewTCPHeader()
	_ = ondatra.NewUDPHeader()
	_ = ondatra.NewICMPHeader()
	_ = ondatra.NewGREHeader()
	_ = ondatra.NewMACsecHeader()
}
`,
			wantSubstrs: []string{
				"ondatra.NewEthernetHeader",
				"ondatra.NewIPv4Header",
				"ondatra.NewIPv6Header",
				"ondatra.NewMPLSHeader",
				"ondatra.NewTCPHeader",
				"ondatra.NewUDPHeader",
				"ondatra.NewICMPHeader",
				"ondatra.NewGREHeader",
				"ondatra.NewMACsecHeader",
			},
		},
		{
			name:    "legacy ATE topology flow and action types",
			pkgPath: "github.com/openconfig/featureprofiles/internal/legacy",
			src: `package legacy
import "github.com/openconfig/ondatra"
type holder struct {
	topo *ondatra.ATETopology
	flow *ondatra.Flow
	intf *ondatra.Interface
	lag  *ondatra.LAG
	ep   ondatra.Endpoint
	hdr  ondatra.Header
	ps   *ondatra.SetPortState
}
`,
			wantSubstrs: []string{
				"ondatra.ATETopology",
				"ondatra.Flow",
				"ondatra.Interface",
				"ondatra.LAG",
				"ondatra.Endpoint",
				"ondatra.Header",
				"ondatra.SetPortState",
			},
		},
		{
			name:    "DialIxNetwork on binding.ATE and via RawAPIs",
			pkgPath: "github.com/openconfig/featureprofiles/feature/foo/tests/foo_test",
			src: `package foo_test
import (
	"context"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
)
func dial(ctx context.Context, ate *ondatra.ATEDevice, b binding.ATE) {
	_, _ = b.DialIxNetwork(ctx)
	_, _ = ate.RawAPIs().BindingATE().DialIxNetwork(ctx)
}
`,
			wantSubstrs: []string{
				"DialIxNetwork",
				"DialIxNetwork",
			},
		},
		{
			name:    "custom wrapper DialIxNetwork returning binding.IxNetwork",
			pkgPath: "github.com/openconfig/featureprofiles/internal/custombind",
			src: `package custombind
import (
	"context"
	"github.com/openconfig/ondatra/binding"
)
type customATE struct{}
func (*customATE) DialIxNetwork(context.Context) (*binding.IxNetwork, error) { return nil, nil }
func callCustom(ctx context.Context, c *customATE) {
	_, _ = c.DialIxNetwork(ctx)
}
`,
			wantSubstrs: []string{
				"binding.IxNetwork",
				"DialIxNetwork",
			},
		},
		{
			name:    "forbidden import ondatra/ixnet",
			pkgPath: "github.com/openconfig/featureprofiles/feature/foo/otg_tests/foo_test",
			src: `package foo_test
import _ "github.com/openconfig/ondatra/ixnet"
`,
			wantSubstrs: []string{`import "github.com/openconfig/ondatra/ixnet"`},
		},
		{
			name:    "forbidden import ondatra/binding/ixweb",
			pkgPath: "github.com/openconfig/featureprofiles/internal/foo",
			src: `package foo
import _ "github.com/openconfig/ondatra/binding/ixweb"
`,
			wantSubstrs: []string{`import "github.com/openconfig/ondatra/binding/ixweb"`},
		},
		{
			name:    "forbidden internal ondatra ate and ixconfig imports",
			pkgPath: "github.com/openconfig/featureprofiles/internal/foo",
			src: `package foo
import (
	_ "github.com/openconfig/ondatra/internal/ate"
	_ "github.com/openconfig/ondatra/internal/ixconfig"
)
`,
			wantSubstrs: []string{
				`import "github.com/openconfig/ondatra/internal/ate"`,
				`import "github.com/openconfig/ondatra/internal/ixconfig"`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := analyzeSnippet(t, tc.pkgPath, tc.src)
			if len(diags) != len(tc.wantSubstrs) {
				t.Fatalf("got %d diagnostics (%+v), want %d (%v)", len(diags), diags, len(tc.wantSubstrs), tc.wantSubstrs)
			}
			for i, want := range tc.wantSubstrs {
				if !strings.Contains(diags[i].Message, want) {
					t.Errorf("diag[%d] = %q, want substring %q", i, diags[i].Message, want)
				}
				if !strings.Contains(diags[i].Message, "gosnappi") {
					t.Errorf("diag[%d] = %q, expected guidance mentioning gosnappi", i, diags[i].Message)
				}
			}
		})
	}
}

func TestAllowedZeroFalsePositives(t *testing.T) {
	tests := []struct {
		name    string
		pkgPath string
		src     string
	}{
		{
			name:    "valid OTG gosnappi OpenConfig device and comments",
			pkgPath: "github.com/openconfig/featureprofiles/feature/bgp/otg_tests/valid_test",
			src: `package valid_test
import (
	"context"
	"testing"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// Comment mentioning ate.Topology(), ate.Traffic(), ate.Actions(), IxNetwork, and ondatra/ixnet.
func TestMain(m *testing.M) {
	ondatra.RunTests(m, nil)
}

func TestValidOTG(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	_ = dut.Port(t, "port1")

	ate := ondatra.ATE(t, "ate")
	for _, p := range ate.Ports() {
		_ = p.ID()
	}
	p1 := ate.Port(t, "port1")
	_ = p1

	cfg := gosnappi.NewConfig()
	_ = cfg.Flows()
	ate.OTG().PushConfig(t)
	ate.OTG().StartProtocols(t)
	ate.OTG().StartTraffic(t)

	// gosnappi ControlState.Traffic() must NOT false-positive.
	cs := gosnappi.NewControlState()
	cs.Traffic().Start()

	// OpenConfig .Actions() and .Topology() must NOT false-positive.
	stmt := &oc.Statement{}
	_ = stmt.Actions()
	_ = stmt.GetOrCreateActions()
	_ = stmt.Topology()

	// RawAPIs().BindingATE().DialOTG() must NOT false-positive.
	var b binding.ATE = ate.RawAPIs().BindingATE()
	_ = b.DialOTG(context.Background())

	msg := "legacy ate.Topology() and IxNetwork mentioned in string literal"
	_ = msg
}
`,
		},
		{
			name:    "exempted topologies/binding package",
			pkgPath: "github.com/openconfig/featureprofiles/topologies/binding",
			src: `package binding
import (
	"context"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/ixweb"
)

type staticATE struct {
	ixweb  *ixweb.IxWeb
	ixsess *ixweb.Session
}

func (a *staticATE) DialIxNetwork(ctx context.Context) (*binding.IxNetwork, error) {
	return &binding.IxNetwork{Session: a.ixsess}, nil
}

func reserve(ctx context.Context, a *staticATE) error {
	_, err := a.DialIxNetwork(ctx)
	return err
}
`,
		},
		{
			name:    "exempted topologies/binding internal test variant",
			pkgPath: "github.com/openconfig/featureprofiles/topologies/binding [github.com/openconfig/featureprofiles/topologies/binding.test]",
			src: `package binding
import (
	"context"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/ixweb"
)

func testIxWeb(ctx context.Context, b binding.ATE, w *ixweb.IxWeb) (*binding.IxNetwork, error) {
	_ = w
	return b.DialIxNetwork(ctx)
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := analyzeSnippet(t, tc.pkgPath, tc.src)
			if len(diags) != 0 {
				t.Fatalf("expected 0 diagnostics for %s, got %d: %+v", tc.name, len(diags), diags)
			}
		})
	}
}

func TestSortDedupeAndGitHubActionsAnnotations(t *testing.T) {
	raw := []diagnostic{
		{File: "b/file.go", Line: 20, Col: 3, Message: "msg B"},
		{File: "a/file.go", Line: 10, Col: 5, Message: "msg A"},
		{File: "a/file.go", Line: 10, Col: 5, Message: "msg A"}, // duplicate from p and p [p.test]
		{File: "a/file.go", Line: 2, Col: 8, Message: "msg early"},
	}

	got := sortAndDedupeDiagnostics(raw)
	if len(got) != 3 {
		t.Fatalf("expected 3 deduplicated diagnostics, got %d: %+v", len(got), got)
	}

	var plainOut bytes.Buffer
	writeDiagnostics(&plainOut, got, false)
	wantPlain := strings.Join([]string{
		"a/file.go:2:8: msg early",
		"a/file.go:10:5: msg A",
		"b/file.go:20:3: msg B",
		"",
	}, "\n")
	if plainOut.String() != wantPlain {
		t.Errorf("plain output mismatch:\ngot:\n%s\nwant:\n%s", plainOut.String(), wantPlain)
	}

	var ghaOut bytes.Buffer
	writeDiagnostics(&ghaOut, got, true)
	wantGHA := strings.Join([]string{
		"a/file.go:2:8: msg early",
		"::error file=a/file.go,line=2,col=8::msg early",
		"a/file.go:10:5: msg A",
		"::error file=a/file.go,line=10,col=5::msg A",
		"b/file.go:20:3: msg B",
		"::error file=b/file.go,line=20,col=3::msg B",
		"",
	}, "\n")
	if ghaOut.String() != wantGHA {
		t.Errorf("GitHub Actions output mismatch:\ngot:\n%s\nwant:\n%s", ghaOut.String(), wantGHA)
	}
}

func TestEndToEndRunPipeline(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"go.mod": "module github.com/openconfig/ondatra\n\ngo 1.22\n",
		"ate.go": `package ondatra
type Topology struct{}
func (*Topology) New() {}
type ATEDevice struct{}
func (*ATEDevice) Topology() *Topology { return nil }
func (*ATEDevice) OTG() {}
`,
		"sub/bad/bad.go": `package bad
import "github.com/openconfig/ondatra"
func BadProd(ate *ondatra.ATEDevice) {
	ate.Topology().New()
}
`,
		"sub/bad/bad_test.go": `package bad
import (
	"testing"
	"github.com/openconfig/ondatra"
)
func TestBad(t *testing.T) {
	var ate ondatra.ATEDevice
	ate.Topology().New()
}
`,
		"sub/clean/clean_test.go": `package clean_test
import (
	"testing"
	"github.com/openconfig/ondatra"
)
func TestClean(t *testing.T) {
	var ate ondatra.ATEDevice
	ate.OTG()
}
`,
		"sub/mixed/mixed.go": `package mixed
func Exported() int { return 1 }
`,
		"sub/mixed/export_test.go": `package mixed
func ForTestOnly() int { return 2 }
`,
		"sub/mixed/mixed_ext_test.go": `package mixed_test
import (
	"testing"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/sub/mixed"
)
func TestMixed(t *testing.T) {
	_ = mixed.Exported() + mixed.ForTestOnly()
	var ate ondatra.ATEDevice
	ate.OTG()
}
`,
	}

	for rel, content := range files {
		full := filepath.Join(tmpDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", full, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"./sub/bad/..."}, tmpDir, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 for violating package, got %d (stderr=%s)", code, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	// Expect exactly 2 unique violations (bad.go and bad_test.go), each with 1 plain line + 1 ::error line = 4 lines,
	// proving bad.go compiled in both bad and bad [bad.test] is deduplicated.
	if len(lines) != 4 {
		t.Fatalf("expected 4 output lines (2 diagnostics + 2 GHA annotations), got %d:\n%s", len(lines), stdout.String())
	}
	if !strings.HasPrefix(lines[0], "sub/bad/bad.go:4:2: ") {
		t.Errorf("unexpected line[0]: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "::error file=sub/bad/bad.go,line=4,col=2::") {
		t.Errorf("unexpected line[1]: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "sub/bad/bad_test.go:8:2: ") {
		t.Errorf("unexpected line[2]: %q", lines[2])
	}
	if !strings.HasPrefix(lines[3], "::error file=sub/bad/bad_test.go,line=8,col=2::") {
		t.Errorf("unexpected line[3]: %q", lines[3])
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"./sub/clean/...", "./sub/mixed/..."}, tmpDir, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for clean packages, got %d (stdout=%s, stderr=%s)", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout for clean package, got:\n%s", stdout.String())
	}
}
