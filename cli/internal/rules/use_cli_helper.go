package cli

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var EnforceCreateCommandWithT = &analysis.Analyzer{
	Name: "createcommandwitht",
	Doc:  "Use CreateCommandWithT",
	Run:  enforceCreateCommandWithTRun,
}

func enforceCreateCommandWithTRun(pass *analysis.Pass) (interface{}, error) {
	var actionType types.Type

	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Name() == "cli" {
			actionType = pkg.Scope().Lookup("ActionFunc").Type()
		}
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			callExpr, isCallExpr := node.(*ast.CallExpr)
			if !isCallExpr {
				return true
			}

			callExprFunType := pass.TypesInfo.TypeOf(callExpr.Fun)
			if callExprFunType == actionType {
			}
			//node.(*ast.Pack)
		})
	}
	return nil, nil
}
