// Package main is the CLI-specific linter itself.
package main

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var enforceCreateCommandWithT = &analysis.Analyzer{
	Name: "createcommandwitht",
	Doc:  "Enforces CreateCommandWithT usage in the CLI codebase",
	Run:  enforceCreateCommandWithTRun,
}

func enforceCreateCommandWithTRun(pass *analysis.Pass) (interface{}, error) {
	var commandType types.Type

	for _, pkg := range pass.Pkg.Imports() {
		// we want to differentiate the upstream `cli` package and our own `cli` package
		if pkg.Name() == "cli" && !strings.Contains(pkg.Path(), "viam") {
			commandType = pkg.Scope().Lookup("Command").Type()
			commandType = types.NewPointer(commandType) // actual `Commands` in the app are pointers
		}
	}
	if commandType == nil { // no CLI imports so we can skip
		return nil, nil
	}

	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			composite, isComposite := node.(*ast.CompositeLit)

			// we aren't looking at a struct, so no need to evaluate further
			if !isComposite {
				return true
			}
			compositeType := pass.TypesInfo.TypeOf(composite)

			// we aren't looking at a CLI command, so no need to evaluate further
			if compositeType.String() != commandType.String() {
				return true
			}

			for _, elt := range composite.Elts {
				keyValue, isKeyValue := elt.(*ast.KeyValueExpr)

				// we aren't looking at assignment of a struct param, so no need to evaluate further
				if !isKeyValue {
					return true
				}
				key := keyValue.Key.(*ast.Ident)

				// "Action", "Before", and "After" are the three types of CLI actions for which
				// `createCommandWithT` was designed
				if key.Name == "Action" || key.Name == "Before" ||
					key.Name == "After" {
					callExpr, isCallExpr := keyValue.Value.(*ast.CallExpr)

					// `Action` was assigned a literal value, rather than a function call.
					if !isCallExpr {
						pass.Report(analysis.Diagnostic{
							Pos:     keyValue.Pos(),
							End:     keyValue.End(),
							Message: "must use createCommandWithT when constructing a CLI action",
						})
						return true
					}

					callExprFunc, isCallExprFunc := callExpr.Fun.(*ast.IndexExpr)
					if !isCallExprFunc { // this should never happen, but just for explicitness
						return true
					}
					funcIdent, isFuncIdent := callExprFunc.X.(*ast.Ident)
					if !isFuncIdent { // as above, this should never happen
						return true
					}

					if funcIdent.Name != "createCommandWithT" {
						// some other func was used to generate the action
						pass.Report(analysis.Diagnostic{
							Pos:     funcIdent.Pos(),
							End:     funcIdent.End(),
							Message: "must use createCommandWithT when constructing a CLI action",
						})
					}
				}
			}
			return true
		})
	}

	return nil, nil
}

func main() {
	singlechecker.Main(enforceCreateCommandWithT)
}
