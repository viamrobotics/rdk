// Package printf defines an Analyzer that reports various printf usages
package printf

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "printf",
	Doc:      "reports various fmt.(Fp|P)rintf* usages",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		// TODO: set via config
		if f := pass.Fset.File(n.Pos()); f != nil &&
			(strings.HasSuffix(f.Name(), "_test.go") ||
				strings.Contains(f.Name(), "/cmd/") ||
				strings.Contains(f.Name(), "/example") ||
				strings.Contains(f.Name(), "/samples") ||
				strings.Contains(f.Name(), "/codegen/") ||
				strings.Contains(f.Name(), "/migration/")) {
			return
		}
		ce := n.(*ast.CallExpr)

		se, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}

		if i, ok := se.X.(*ast.Ident); !ok || i.String() != "fmt" {
			return
		}

		hasFPrint := strings.HasPrefix(se.Sel.String(), "Fprint")
		if !strings.HasPrefix(se.Sel.String(), "Print") && !hasFPrint {
			return
		}

		if hasFPrint {
			if len(ce.Args) == 0 {
				return
			}

			fileArg := ce.Args[0]
			fileSE, ok := fileArg.(*ast.SelectorExpr)
			if !ok {
				return
			}
			if i, ok := fileSE.X.(*ast.Ident); !ok || i.String() != "os" {
				return
			}

			if fileSE.Sel.String() != "Stdout" {
				return
			}
		}

		pass.Reportf(se.Sel.Pos(), "fmt.(Fp|P)rintf* usage found %q",
			render(pass.Fset, se.Sel))
		return
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
