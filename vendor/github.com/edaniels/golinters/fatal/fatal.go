// Package fatal defines an Analyzer that reports various (golog.logger).Fatal usages
package fatal

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
	Name:     "fatal",
	Doc:      "reports various (golog.Logger).Fatal* usages",
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
		f := pass.Fset.File(n.Pos())
		if f != nil &&
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
		i, ok := se.X.(*ast.Ident)
		if !ok || !strings.HasPrefix(se.Sel.String(), "Fatal") {
			return
		}

		t, ok := pass.TypesInfo.Types[i]
		if !ok || !strings.HasSuffix(t.Type.String(), "Logger") {
			return
		}

		pass.Reportf(se.Sel.Pos(), "Log fatal usage detected: (%v).%s(...)", t.Type, render(pass.Fset, se.Sel))

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
