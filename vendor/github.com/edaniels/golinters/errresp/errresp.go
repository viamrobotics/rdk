// Package errresp defines an Analyzer that reports ErrorResponse usage with no return after
package errresp

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
	Name:     "errresp",
	Doc:      "reports ErrorResponse usage with no return after",
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
		if f := pass.Fset.File(n.Pos()); f != nil && strings.HasSuffix(f.Name(), "_test.go") {
			return false
		}

		ce := n.(*ast.CallExpr)

		se, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}

		if se.Sel.String() != "ErrorResponse" {
			return false
		}

		// look for general structure of scoped block that does not return.
		// catches case where block doesn't return but the surrounding block
		// does immediately.

		// find surrounding block scopes
		var outerIsExpr bool
		var lastExprAt int
		for i := len(stack) - 1; i >= 0; i-- {
			stackElem := stack[i]
			switch stackElem.(type) {
			case *ast.BlockStmt, *ast.CaseClause:
				var stmts []ast.Stmt
				switch t := stackElem.(type) {
				case *ast.BlockStmt:
					stmts = t.List
				case *ast.CaseClause:
					stmts = t.Body
				}
				if len(stmts) < 1 {
					continue
				}
				switch stmt := stmts[len(stmts)-1].(type) {
				case *ast.ReturnStmt, *ast.IfStmt, *ast.SwitchStmt:
					if outerIsExpr {
						allCond := true
						// find which stmt our expr is in and start there
						var afterExprStmt bool
					loop:
						for _, blockStmt := range stmts[:len(stmts)-1] {
							if !afterExprStmt {
								ast.Inspect(blockStmt, func(child ast.Node) bool {
									if child == ce {
										afterExprStmt = true
										return false
									}
									return true
								})
								continue
							}
							switch blockStmt.(type) {
							case *ast.IfStmt:
							default:
								allCond = false
								break loop
							}
						}
						if allCond {
							return false
						}
					}
					outerIsExpr = false
				case *ast.ExprStmt:
					if stmt.X == n {
						lastExprAt = i
						outerIsExpr = true
					}
					continue
				default:
					outerIsExpr = false
					continue
				}
				if len(stmts) < 2 {
					continue
				}
				es, ok := stmts[len(stmts)-2].(*ast.ExprStmt)
				if !ok {
					continue
				}
				if !outerIsExpr && es.X == n {
					return false
				}
			}
		}

		// if last thing we saw was expr, check if we are closest to a func decl
		if outerIsExpr && lastExprAt == 2 {
			return false
		}

		pass.Reportf(se.Sel.Pos(), "ErrorResponse() not immediately followed by return")
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
