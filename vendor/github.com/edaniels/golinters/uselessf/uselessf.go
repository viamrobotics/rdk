// Package uselessf defines an Analyzer that reports useless prints with f usages
package uselessf

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
	Name:     "errorf",
	Doc:      "reports useless prints with f usages",
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
		ce := n.(*ast.CallExpr)

		se, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}

		sel := se.Sel.String()
		switch sel {
		case "DPanicf", "Debugf", "Errorf", "Infof", "Panicf", "Warnf", "Printf":
		default:
			return
		}

		if len(ce.Args) == 1 {
			if i, ok := se.X.(*ast.Ident); ok && i.String() != "fmt" && strings.HasPrefix(se.Sel.String(), "Errorf") {
				pass.Reportf(se.Sel.Pos(), "uesless fmt.Errorf usage found; use errors.New instead")
				return
			}
			pass.Reportf(se.Sel.Pos(), "useless print with f usage found %q; use non-f version",
				render(pass.Fset, se.Sel))

		}
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
