// Package mustcheck defines an Analyzer that reports .MustVerb usages
package mustcheck

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"

	"github.com/fatih/camelcase"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "mustcheck",
	Doc:      "reports various .MustVerb() usages",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	inspect.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) bool {
		// TODO: set via config
		if f := pass.Fset.File(n.Pos()); f != nil &&
			(strings.HasSuffix(f.Name(), "_test.go") ||
				strings.Contains(f.Name(), "utils/test")) {
			return false
		}
		ce := n.(*ast.CallExpr)

		if len(ce.Args) != 0 {
			return false
		}

		se, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}

		splitted := camelcase.Split(se.Sel.String())
		if splitted[0] != "Must" {
			return false
		}

		// if outermost func decl is init or global decl, ignore
		var outerFD *ast.FuncDecl
		var outerGD *ast.GenDecl
		for _, stackElem := range stack {
			switch t := stackElem.(type) {
			case *ast.FuncDecl:
				outerFD = t
			case *ast.GenDecl:
				outerGD = t
			}
		}
		if outerFD != nil && outerFD.Name.String() == "init" {
			return false
		}
		if outerGD != nil && len(stack) == 4 {
			return false
		}

		pass.Reportf(se.Sel.Pos(), "MustVerb() usage found %q",
			render(pass.Fset, se.Sel))
		return false
	})

	return nil, nil
}

// render returns the pretty-print of the given node
func render(fset *token.FileSet, x interface{}) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, x); err != nil {
		panic(err)
	}
	return buf.String()
}
