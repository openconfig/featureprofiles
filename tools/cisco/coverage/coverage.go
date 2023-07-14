// Package coverage implements the tool to generate oc coverage report for tests
package coverage

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

type pathMap struct {
	m map[string]string
}

// AFact makes sure pathMap adheres to the analysis.Fact interface
func (pm *pathMap) AFact() {

}

func (pm *pathMap) getPath(t string) string {
	return pm.m[t]
}

func (pm *pathMap) putPath(t, p string) {
	if pm.m == nil {
		pm.m = make(map[string]string)
	}
	pm.m[t] = p
}

// CoverageAnalyzer reports all operations on OC paths.
var CoverageAnalyzer = &analysis.Analyzer{
	Name:      "Coverage",
	Doc:       "Tool to print all covered OC paths",
	Requires:  []*analysis.Analyzer{inspect.Analyzer},
	Run:       run,
	FactTypes: []analysis.Fact{new(pathMap)},
}

func run(pass *analysis.Pass) (interface{}, error) {
	pkgPath := pass.Pkg.Path()
	if strings.HasPrefix(pkgPath, "github.com/openconfig/ondatra") {
		return handleOndatraPkg(pass)
	}

	if !strings.HasPrefix(pkgPath, "github.com/openconfig/featureprofiles") {
		return nil, nil
	}

	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncDecl:
				funcName := node.Name.Name
				ast.Inspect(node.Body, func(node ast.Node) bool {
					switch node := node.(type) {
					case *ast.CallExpr:
						selexpr, ok := node.Fun.(*ast.SelectorExpr)
						if !ok {
							return true
						}

						if selexpr.Sel.Name == "Replace" || selexpr.Sel.Name == "Update" ||
							selexpr.Sel.Name == "Delete" || selexpr.Sel.Name == "Get" ||
							selexpr.Sel.Name == "Watch" {
							if len(node.Args) < 3 {
								return true
							}

							t := pass.TypesInfo.TypeOf(node.Args[2])
							if !strings.HasPrefix(t.String(), "github.com/openconfig/ygnmi") {
								return true
							}

							ast.Inspect(node.Args[2], func(node ast.Node) bool {
								switch node := node.(type) {
								case *ast.CallExpr:
									selexpr, ok := node.Fun.(*ast.SelectorExpr)
									if !ok {
										return true
									}
									if selexpr.Sel.Name != "State" && selexpr.Sel.Name != "Config" {
										return true
									}

									o := pass.TypesInfo.ObjectOf(selexpr.Sel)
									m := new(pathMap)
									pass.ImportPackageFact(o.Pkg(), m)
									p := m.getPath(o.String())
									if p != "" {
										fmt.Printf("%v,%v,%v,%v,%v\n", pass.Fset.Position(selexpr.Pos()),
											pkgPath, funcName, selexpr.Sel.Name, p)
									}
								}
								return true
							})
						}
					}
					return true
				})

			}
			return true
		})
	}
	return nil, nil
}

func handleOndatraPkg(pass *analysis.Pass) (interface{}, error) {
	if !strings.HasPrefix(pass.Pkg.Path(), "github.com/openconfig/ondatra/gnmi") {
		return nil, nil
	}

	docRe := regexp.MustCompile(`Path from root:.*?"(/.+)"`)

	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch decl := node.(type) {
			case *ast.FuncDecl:
				doc := decl.Doc.Text()
				matches := docRe.FindStringSubmatch(doc)
				if matches == nil {
					return true
				}
				m := new(pathMap)
				pass.ImportPackageFact(pass.Pkg, m)
				id := pass.TypesInfo.ObjectOf(decl.Name).String()
				path := matches[1]
				m.putPath(id, path)
				pass.ExportPackageFact(m)
			}
			return true
		})
	}
	return nil, nil
}
