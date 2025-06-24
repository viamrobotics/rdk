// Package deferfor defines an Analyzer that reports defer usage within a for
package deferfor

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
	Name:     "deferfor",
	Doc:      "reports defer usage within a for",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.DeferStmt)(nil),
	}

	inspect.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) bool {
		// TODO: set via config
		if f := pass.Fset.File(n.Pos()); f != nil && strings.HasSuffix(f.Name(), "_test.go") {
			return false
		}

		ds := n.(*ast.DeferStmt)

		// check if defer occurs within an immediate for before we encounter a function
	loop:
		for i := len(stack) - 2; i >= 0; i-- {
			stackElem := stack[i]
			switch stackElem.(type) {
			case *ast.ForStmt:
				break loop
			case *ast.FuncDecl, *ast.FuncLit:
				return false
			}
		}

		pass.Reportf(ds.Pos(), "defer within an immediate outer for usage found %q",
			render(pass.Fset, ds))
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
