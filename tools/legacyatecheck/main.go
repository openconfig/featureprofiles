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

// Command legacyatecheck is a type-checked static analyzer that flags and
// rejects deprecated Ondatra legacy ATE and IxNetwork API usage in favor of
// the Open Traffic Generator (OTG) API (ate.OTG() and gosnappi).
package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	ondatraPkg        = "github.com/openconfig/ondatra"
	ondatraBindingPkg = "github.com/openconfig/ondatra/binding"
	exemptBindingPkg  = "github.com/openconfig/featureprofiles/topologies/binding"
)

var forbiddenImportPrefixes = []string{
	"github.com/openconfig/ondatra/ixnet",
	"github.com/openconfig/ondatra/binding/ixweb",
	"github.com/openconfig/ondatra/internal/ixconfig",
	"github.com/openconfig/ondatra/internal/ate",
}

var forbiddenATEDeviceMethods = map[string]bool{
	"Topology": true,
	"Traffic":  true,
	"Actions":  true,
}

// forbiddenOndatraSymbols lists all exported legacy ATE constructors and types
// in github.com/openconfig/ondatra (topology.go, interface.go, lag.go,
// traffic.go, actions.go, and headers.go). Shared device/test symbols such as
// ATE, ATEDevice, DUT, DUTDevice, Device, Port, Speed, PMD, Vendor, Config,
// Debug, Report, EventListener, ReservationID, SkipReservationRelease,
// ExecuteTests, and RunTests are intentionally allowed.
var forbiddenOndatraSymbols = map[string]bool{
	// Topology, Interface, and LAG types (topology.go, interface.go, lag.go).
	"Topology":    true,
	"ATETopology": true,
	"Interface":   true,
	"LAG":         true,
	"LACP":        true,

	// Traffic and Flow types (traffic.go).
	"Traffic":        true,
	"IMIXCustom":     true,
	"Flow":           true,
	"Endpoint":       true,
	"Transmission":   true,
	"EgressTracking": true,

	// Actions types (actions.go).
	"Actions":             true,
	"SetPortState":        true,
	"SetLACPState":        true,
	"BGPPeerNotification": true,
	"BGPGracefulRestart":  true,

	// Packet Header constructors and types (headers.go).
	"Header":                           true,
	"NewEthernetHeader":                true,
	"EthernetHeader":                   true,
	"NewGREHeader":                     true,
	"GREHeader":                        true,
	"NewIPv4Header":                    true,
	"IPv4Header":                       true,
	"NewIPv6Header":                    true,
	"IPv6Header":                       true,
	"NewMPLSHeader":                    true,
	"MPLSHeader":                       true,
	"NewTCPHeader":                     true,
	"TCPHeader":                        true,
	"NewUDPHeader":                     true,
	"UDPHeader":                        true,
	"NewHTTPHeader":                    true,
	"HTTPHeader":                       true,
	"NewICMPHeader":                    true,
	"ICMPHeader":                       true,
	"ICMPEchoReply":                    true,
	"ICMPDestinationUnreachable":       true,
	"ICMPRedirectMessage":              true,
	"ICMPEchoRequest":                  true,
	"ICMPTimeExceeded":                 true,
	"ICMPParameterProblem":             true,
	"ICMPTimestamp":                    true,
	"ICMPTimestampReply":               true,
	"NewOSPFHeader":                    true,
	"OSPFHeader":                       true,
	"OSPFHello":                        true,
	"OSPFDatabaseDescription":          true,
	"OSPFLinkStateRequest":             true,
	"OSPFLinkStateAdvertisementHeader": true,
	"OSPFLinkStateUpdate":              true,
	"OSPFAdvertisement":                true,
	"OSPFLinkStateAck":                 true,
	"NewRSVPHeader":                    true,
	"RSVPHeader":                       true,
	"NewPIMHeader":                     true,
	"PIMHeader":                        true,
	"PIMHello":                         true,
	"NewLDPHeader":                     true,
	"LDPHeader":                        true,
	"LDPHello":                         true,
	"NewPwMplsControlWordHeader":       true,
	"PwMplsControlWordHeader":          true,
	"NewMACsecHeader":                  true,
	"MACsecHeader":                     true,
	"NewMacSecHeader":                  true,
	"MacSecHeader":                     true,
	"NewESP":                           true,
	"ESPHeader":                        true,
	"NewESPOverMACSec":                 true,
	"ESPOverMACSecHeader":              true,
	"NewVLANHeader":                    true,
	"VLANHeader":                       true,
	"NewPBBHeader":                     true,
	"PBBHeader":                        true,
}

type diagnostic struct {
	File    string
	Line    int
	Col     int
	Message string
}

type rawDiag struct {
	pos     token.Pos
	message string
}

type packageError struct {
	ImportStack []string `json:"ImportStack"`
	Pos         string   `json:"Pos"`
	Err         string   `json:"Err"`
}

type goListPackage struct {
	Dir        string            `json:"Dir"`
	ImportPath string            `json:"ImportPath"`
	Name       string            `json:"Name"`
	ForTest    string            `json:"ForTest"`
	Export     string            `json:"Export"`
	DepOnly    bool              `json:"DepOnly"`
	GoFiles    []string          `json:"GoFiles"`
	Imports    []string          `json:"Imports"`
	ImportMap  map[string]string `json:"ImportMap"`
	Error      *packageError     `json:"Error"`
	DepsErrors []*packageError   `json:"DepsErrors"`
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [packages...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "legacyatecheck: getwd: %v\n", err)
		os.Exit(1)
	}

	code := run(flag.Args(), workDir, os.Getenv("GITHUB_ACTIONS") == "true", os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(patterns []string, workDir string, githubActions bool, stdout, stderr io.Writer) int {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	pkgs, exports, err := loadPackages(workDir, patterns)
	if err != nil {
		fmt.Fprintf(stderr, "legacyatecheck: %v\n", err)
		return 1
	}

	fset := token.NewFileSet()
	baseImporter := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		exportPath, ok := exports[path]
		if !ok || exportPath == "" {
			return nil, fmt.Errorf("missing export data for %q", path)
		}
		return os.Open(exportPath)
	})

	var allDiags []diagnostic
	var checkErrs []error
	matchedTargets := 0

	for _, pkg := range pkgs {
		if pkg.DepOnly || isSyntheticTestMain(pkg) {
			continue
		}
		matchedTargets++

		if isExemptPackage(canonicalPkgPath(pkg)) || len(pkg.GoFiles) == 0 {
			continue
		}

		diags, err := analyzeGoListPackage(fset, pkg, exports, baseImporter, workDir)
		if err != nil {
			checkErrs = append(checkErrs, err)
			continue
		}
		allDiags = append(allDiags, diags...)
	}

	if matchedTargets == 0 {
		fmt.Fprintf(stderr, "legacyatecheck: no packages matched %v\n", patterns)
		return 1
	}

	for _, err := range checkErrs {
		fmt.Fprintf(stderr, "legacyatecheck: %v\n", err)
	}

	sortedDiags := sortAndDedupeDiagnostics(allDiags)
	writeDiagnostics(stdout, sortedDiags, githubActions)

	if len(checkErrs) > 0 || len(sortedDiags) > 0 {
		return 1
	}
	return 0
}

func loadPackages(workDir string, patterns []string) ([]*goListPackage, map[string]string, error) {
	// First resolve all target packages (including internal and external test
	// variants) without compiling export archives for hundreds of unimported leaf
	// test binaries.
	pkgs, err := runGoList(workDir, append([]string{"list", "-deps", "-test", "-e", "-json"}, patterns...))
	if err != nil {
		return nil, nil, err
	}

	exports := make(map[string]string)
	normalImportsSet := make(map[string]bool)
	testExportPkgsSet := make(map[string]bool)

	for _, p := range pkgs {
		if p.Export != "" {
			exports[p.ImportPath] = p.Export
		}
		if p.DepOnly || isSyntheticTestMain(p) || isExemptPackage(canonicalPkgPath(p)) || len(p.GoFiles) == 0 {
			continue
		}
		for _, imp := range p.Imports {
			if imp == "unsafe" || imp == "C" {
				continue
			}
			if baseImp, _, hasTestVariant := strings.Cut(imp, " ["); hasTestVariant {
				testExportPkgsSet[baseImp] = true
				continue
			}
			if mapped, ok := p.ImportMap[imp]; ok && strings.Contains(mapped, " [") {
				baseImp, _, _ := strings.Cut(mapped, " [")
				testExportPkgsSet[baseImp] = true
				continue
			}
			if strings.HasPrefix(imp, "vendor/") {
				for orig, mapped := range p.ImportMap {
					if mapped == imp {
						imp = orig
						break
					}
				}
			}
			normalImportsSet[imp] = true
		}
	}

	if len(normalImportsSet) > 0 {
		normalImports := make([]string, 0, len(normalImportsSet))
		for imp := range normalImportsSet {
			normalImports = append(normalImports, imp)
		}
		slices.Sort(normalImports)

		expPkgs, err := runGoList(workDir, append([]string{"list", "-deps", "-export", "-e", "-json"}, normalImports...))
		if err != nil {
			return nil, nil, err
		}
		for _, ep := range expPkgs {
			if ep.Export != "" {
				exports[ep.ImportPath] = ep.Export
			}
		}
	}

	if len(testExportPkgsSet) > 0 {
		testExportPkgs := make([]string, 0, len(testExportPkgsSet))
		for tp := range testExportPkgsSet {
			testExportPkgs = append(testExportPkgs, tp)
		}
		slices.Sort(testExportPkgs)

		expTestPkgs, err := runGoList(workDir, append([]string{"list", "-deps", "-export", "-test", "-e", "-json"}, testExportPkgs...))
		if err != nil {
			return nil, nil, err
		}
		for _, ep := range expTestPkgs {
			if ep.Export != "" {
				exports[ep.ImportPath] = ep.Export
			}
		}
	}

	return pkgs, exports, nil
}

func runGoList(workDir string, args []string) ([]*goListPackage, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = workDir
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("go list stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting go list: %w", err)
	}

	var pkgs []*goListPackage
	var loadErrs []string

	dec := json.NewDecoder(stdoutPipe)
	for {
		var p goListPackage
		if err := dec.Decode(&p); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			_ = cmd.Wait()
			return nil, fmt.Errorf("decoding go list output: %w", err)
		}
		if p.Error != nil {
			loadErrs = append(loadErrs, fmt.Sprintf("package %s: %s", p.ImportPath, strings.TrimSpace(p.Error.Err)))
		}
		if !p.DepOnly {
			for _, depErr := range p.DepsErrors {
				loadErrs = append(loadErrs, fmt.Sprintf("package %s dependency error: %s", p.ImportPath, strings.TrimSpace(depErr.Err)))
			}
		}
		pkgs = append(pkgs, &p)
	}

	if err := cmd.Wait(); err != nil {
		msg := strings.TrimSpace(stderrBuf.String())
		if msg != "" {
			return nil, fmt.Errorf("go list failed: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("go list failed: %w", err)
	}
	if len(loadErrs) > 0 {
		slices.Sort(loadErrs)
		loadErrs = slices.Compact(loadErrs)
		return nil, fmt.Errorf("package load errors:\n%s", strings.Join(loadErrs, "\n"))
	}
	return pkgs, nil
}

func isSyntheticTestMain(p *goListPackage) bool {
	return p.Name == "main" && strings.HasSuffix(p.ImportPath, ".test")
}

func canonicalPkgPath(p *goListPackage) string {
	if p.ForTest != "" {
		return p.ForTest
	}
	base, _, _ := strings.Cut(p.ImportPath, " [")
	return base
}

func typeCheckPkgPath(p *goListPackage) string {
	base, _, _ := strings.Cut(p.ImportPath, " [")
	return base
}

func isExemptPackage(pkgPath string) bool {
	base, _, _ := strings.Cut(pkgPath, " [")
	return base == exemptBindingPkg
}

type importMapImporter struct {
	importMap map[string]string
	inner     types.Importer
}

func (m *importMapImporter) Import(path string) (*types.Package, error) {
	if mapped, ok := m.importMap[path]; ok && !strings.Contains(mapped, " [") {
		path = mapped
	}
	return m.inner.Import(path)
}

func importerForPackage(fset *token.FileSet, pkg *goListPackage, exports map[string]string, baseImporter types.Importer) types.Importer {
	if len(pkg.ImportMap) == 0 {
		return baseImporter
	}
	inner := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		if mapped, ok := pkg.ImportMap[path]; ok {
			path = mapped
		}
		exportPath, ok := exports[path]
		if !ok || exportPath == "" {
			return nil, fmt.Errorf("missing export data for %q", path)
		}
		return os.Open(exportPath)
	})
	return &importMapImporter{
		importMap: pkg.ImportMap,
		inner:     inner,
	}
}

func analyzeGoListPackage(fset *token.FileSet, pkg *goListPackage, exports map[string]string, baseImporter types.Importer, workDir string) ([]diagnostic, error) {
	files := make([]*ast.File, 0, len(pkg.GoFiles))
	for _, name := range pkg.GoFiles {
		fullPath := name
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(pkg.Dir, name)
		}
		f, err := parser.ParseFile(fset, fullPath, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", fullPath, err)
		}
		files = append(files, f)
	}

	imp := importerForPackage(fset, pkg, exports, baseImporter)
	rawDiags, err := typeCheckAndInspect(fset, typeCheckPkgPath(pkg), canonicalPkgPath(pkg), files, imp)
	if err != nil {
		return nil, fmt.Errorf("type-checking %s: %w", pkg.ImportPath, err)
	}

	diags := make([]diagnostic, 0, len(rawDiags))
	for _, rd := range rawDiags {
		pos := fset.Position(rd.pos)
		diags = append(diags, diagnostic{
			File:    displayPath(workDir, pos.Filename),
			Line:    pos.Line,
			Col:     pos.Column,
			Message: rd.message,
		})
	}
	return diags, nil
}

func typeCheckAndInspect(fset *token.FileSet, checkPkgPath, originPkgPath string, files []*ast.File, imp types.Importer) ([]rawDiag, error) {
	if isExemptPackage(originPkgPath) || isExemptPackage(checkPkgPath) {
		return nil, nil
	}

	info := &types.Info{
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	cfg := &types.Config{
		Importer:    imp,
		FakeImportC: true,
	}
	if _, err := cfg.Check(checkPkgPath, fset, files, info); err != nil {
		return nil, err
	}

	return inspectCheckedFiles(originPkgPath, files, info), nil
}

func inspectCheckedFiles(pkgPath string, files []*ast.File, info *types.Info) []rawDiag {
	if isExemptPackage(pkgPath) {
		return nil
	}

	var diags []rawDiag
	visitedSelIdent := make(map[*ast.Ident]bool)

	for _, file := range files {
		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if isForbiddenImport(importPath) {
				diags = append(diags, rawDiag{
					pos:     imp.Pos(),
					message: fmt.Sprintf("legacy ATE/IxNetwork import %q is deprecated and banned; use ate.OTG() and gosnappi instead", importPath),
				})
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SelectorExpr:
				if sel, ok := info.Selections[node]; ok {
					if msg, bad := checkSelection(sel); bad {
						diags = append(diags, rawDiag{
							pos:     node.Pos(),
							message: msg,
						})
					}
					return true
				}
				visitedSelIdent[node.Sel] = true
				if msg, bad := checkUsedObject(info.Uses[node.Sel]); bad {
					diags = append(diags, rawDiag{
						pos:     node.Pos(),
						message: msg,
					})
				}
			case *ast.Ident:
				if visitedSelIdent[node] {
					return true
				}
				if msg, bad := checkUsedObject(info.Uses[node]); bad {
					diags = append(diags, rawDiag{
						pos:     node.Pos(),
						message: msg,
					})
				}
			}
			return true
		})
	}

	return diags
}

func isForbiddenImport(importPath string) bool {
	for _, prefix := range forbiddenImportPrefixes {
		if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
			return true
		}
	}
	return false
}

func checkSelection(sel *types.Selection) (string, bool) {
	if sel == nil || sel.Obj() == nil {
		return "", false
	}
	name := sel.Obj().Name()

	if forbiddenATEDeviceMethods[name] {
		if isNamedType(declaredRecvType(sel.Obj()), ondatraPkg, "ATEDevice") || isNamedType(sel.Recv(), ondatraPkg, "ATEDevice") {
			return fmt.Sprintf("legacy ATE method (*ondatra.ATEDevice).%s is deprecated and banned; use ate.OTG() and gosnappi instead", name), true
		}
	}

	if name == "DialIxNetwork" {
		if isNamedType(declaredRecvType(sel.Obj()), ondatraBindingPkg, "ATE") ||
			isNamedType(sel.Recv(), ondatraBindingPkg, "ATE") ||
			signatureReturnsNamedType(sel.Type(), ondatraBindingPkg, "IxNetwork") ||
			signatureReturnsNamedType(sel.Obj().Type(), ondatraBindingPkg, "IxNetwork") {
			return "legacy IxNetwork method DialIxNetwork is deprecated and banned; use DialOTG / ate.OTG() and gosnappi instead", true
		}
	}

	return "", false
}

func checkUsedObject(obj types.Object) (string, bool) {
	if obj == nil || obj.Pkg() == nil {
		return "", false
	}
	if obj.Parent() != obj.Pkg().Scope() {
		return "", false
	}
	switch obj.Pkg().Path() {
	case ondatraPkg:
		if forbiddenOndatraSymbols[obj.Name()] {
			return fmt.Sprintf("legacy ATE symbol ondatra.%s is deprecated and banned; use ate.OTG() and gosnappi instead", obj.Name()), true
		}
	case ondatraBindingPkg:
		if obj.Name() == "IxNetwork" {
			return "legacy IxNetwork type binding.IxNetwork is deprecated and banned; use ate.OTG() and gosnappi instead", true
		}
	}
	return "", false
}

func declaredRecvType(obj types.Object) types.Type {
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil
	}
	return sig.Recv().Type()
}

func signatureReturnsNamedType(t types.Type, pkgPath, typeName string) bool {
	if t == nil {
		return false
	}
	sig, ok := types.Unalias(t).(*types.Signature)
	if !ok || sig.Results() == nil {
		return false
	}
	results := sig.Results()
	for i := 0; i < results.Len(); i++ {
		if isNamedType(results.At(i).Type(), pkgPath, typeName) {
			return true
		}
	}
	return false
}

func isNamedType(t types.Type, pkgPath, typeName string) bool {
	named := unwrapNamed(t)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == pkgPath && named.Obj().Name() == typeName
}

func unwrapNamed(t types.Type) *types.Named {
	for t != nil {
		t = types.Unalias(t)
		ptr, ok := t.(*types.Pointer)
		if !ok {
			break
		}
		t = ptr.Elem()
	}
	named, _ := t.(*types.Named)
	return named
}

func displayPath(workDir, filename string) string {
	if workDir != "" {
		if rel, err := filepath.Rel(workDir, filename); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(filename)
}

func sortAndDedupeDiagnostics(diags []diagnostic) []diagnostic {
	if len(diags) == 0 {
		return nil
	}
	sorted := slices.Clone(diags)
	slices.SortFunc(sorted, func(a, b diagnostic) int {
		if c := cmp.Compare(a.File, b.File); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Line, b.Line); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Col, b.Col); c != 0 {
			return c
		}
		return cmp.Compare(a.Message, b.Message)
	})
	return slices.Compact(sorted)
}

func writeDiagnostics(w io.Writer, diags []diagnostic, githubActions bool) {
	for _, d := range diags {
		fmt.Fprintf(w, "%s:%d:%d: %s\n", d.File, d.Line, d.Col, d.Message)
		if githubActions {
			fmt.Fprintf(w, "::error file=%s,line=%d,col=%d::%s\n", d.File, d.Line, d.Col, d.Message)
		}
	}
}
