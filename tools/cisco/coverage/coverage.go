//Package coverage implements the tool to generate oc coverage report for tests
package coverage


import (
	"fmt"
	"go/ast"
	"go/types"
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

							t := pass.TypesInfo.TypeOf(selexpr.X)
							if strings.HasPrefix(t.String(), "*github.com/openconfig/ondatra/") {
								tDeref := t.(*types.Pointer).Elem()
								tNamed := tDeref.(*types.Named)
								tName := tNamed.Obj().Id()
								tPkg := tNamed.Obj().Pkg()
								m := new(pathMap)
								pass.ImportPackageFact(tPkg, m)
								fmt.Printf("%v,%v,%v,%v,%v\n", pass.Fset.Position(selexpr.Pos()),
									pkgPath, funcName, selexpr.Sel.Name, m.getPath(tName))
							}
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
	if !strings.HasPrefix(pass.Pkg.Path(), "github.com/openconfig/ondatra") {
		return nil, nil
	}
	docRe := regexp.MustCompile(`(.+)\srepresents\sthe\s(?:wildcard\sversion\sof\sthe\s)?(/.+)\sYANG\sschema\selement.`)

	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch decl := node.(type) {
			case *ast.GenDecl:
				doc := decl.Doc.Text()
				matches := docRe.FindStringSubmatch(doc)
				if matches == nil {
					return true
				}
				m := new(pathMap)
				pass.ImportPackageFact(pass.Pkg, m)
				tName := matches[1]
				path := "/" + strings.Join(strings.Split(matches[2], "/")[2:], "/")
				m.putPath(tName, path)
				pass.ExportPackageFact(m)
			}
			return true
		})
	}
	return nil, nil
}
