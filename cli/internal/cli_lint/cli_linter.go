// Package main is the CLI-specific linter itself.
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/kyoh86/nolint"
	"golang.org/x/exp/slices"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
)

func isOrgType(val string) bool {
	return slices.Contains([]string{
		"generalFlagOrgID",
		"generalFlagOrganization",
		"generalFlagAliasOrg",
		"generalFlagAliasOrgName",
		"org-id",
		"organization",
		"org",
		"org-name",
	},
		val,
	)
}

func isLocationType(val string) bool {
	return slices.Contains([]string{
		"generalFlagLocationID",
		"generalFlagLocation",
		"generalFlagAliasLocationName",
		"location-id",
		"location",
		"location-name",
	},
		val,
	)
}

func enforceFlagOptionalRun(pass *analysis.Pass, isFlagTypeFunc func(string) bool, flagName string) (any, error) {
	noLinter := pass.ResultOf[nolint.Analyzer].(*nolint.NoLinter)
	var flagsType types.Type

	for _, pkg := range pass.Pkg.Imports() {
		// we want to differentiate the upstream `cli` package and our own `cli` package
		if pkg.Name() == "cli" && !strings.Contains(pkg.Path(), "viam") {
			flagsType = pkg.Scope().Lookup("StringFlag").Type()
		}
	}
	if flagsType == nil { // no CLI imports so we can skip
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

			// we aren't looking at a CLI flags struct, so no need to evaluate further
			if compositeType.String() != flagsType.String() {
				return true
			}

			required := false
			isFlagType := false
			for _, elt := range composite.Elts {
				keyValue, isKeyValue := elt.(*ast.KeyValueExpr)

				// we aren't looking at assignment of a struct param, so no need to evaluate further
				if !isKeyValue {
					return true
				}
				key := keyValue.Key.(*ast.Ident)

				if key.Name == "Name" {
					if identVal, isIdentVal := keyValue.Value.(*ast.Ident); isIdentVal {
						isFlagType = isFlagTypeFunc(identVal.String())
					} else if basicLitVal, isBasicLitVal := keyValue.Value.(*ast.BasicLit); isBasicLitVal {
						isFlagType = isFlagTypeFunc(basicLitVal.Value)
					}
				}

				var pos token.Pos
				var end token.Pos
				if key.Name == "Required" {
					basicLitVal, isBasicLitVal := keyValue.Value.(*ast.Ident)
					if isBasicLitVal {
						pos = basicLitVal.Pos()
						end = basicLitVal.End()
						required = basicLitVal.String() == "true"
					}

					nolintCategory := fmt.Sprintf("enforce%soptional", flagName)

					// `Action` was assigned a literal value, rather than a function call.
					if required && isFlagType && !noLinter.IgnoreNode(node, nolintCategory) {
						pass.Report(analysis.Diagnostic{
							Pos:     pos,
							End:     end,
							Message: fmt.Sprintf("%s flags must be optional in order to respect default %s values", flagName, flagName),
						})
						return true
					}
				}
			}
			return true
		})
	}

	return nil, nil
}

var enforceOrgOptional = &analysis.Analyzer{
	Name:     "enforceorgoptional",
	Doc:      "Enforces that org arguments are optional",
	Run:      enforceOrgOptionalRun,
	Requires: []*analysis.Analyzer{nolint.Analyzer},
}

func enforceOrgOptionalRun(pass *analysis.Pass) (any, error) {
	return enforceFlagOptionalRun(pass, isOrgType, "org")
}

var enforceLocationOptional = &analysis.Analyzer{
	Name:     "enforcelocationoptional",
	Doc:      "Enforces that location arguments are optional",
	Run:      enforceLocationOptionalRun,
	Requires: []*analysis.Analyzer{nolint.Analyzer},
}

func enforceLocationOptionalRun(pass *analysis.Pass) (any, error) {
	return enforceFlagOptionalRun(pass, isLocationType, "location")
}

var enforceCreateCommandWithT = &analysis.Analyzer{
	Name: "createcommandwitht",
	Doc:  "Enforces CreateCommandWithT usage in the CLI codebase",
	Run:  enforceCreateCommandWithTRun,
}

func enforceCreateCommandWithTRun(pass *analysis.Pass) (any, error) {
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
	multichecker.Main(enforceOrgOptional, enforceLocationOptional, enforceCreateCommandWithT)
}
