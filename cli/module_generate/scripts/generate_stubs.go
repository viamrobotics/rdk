// Package scripts contains scripts that generate method stubs for modules
package scripts

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"net/http"
	"strings"
	"text/template"
	"unicode"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/cli/module_generate/modulegen"
)

//go:embed tmpl-module
var goTmpl string

// typePrefixes lists possible prefixes before function parameter and return types.
var typePrefixes = []string{"*", "[]*", "[]", "chan "}

// getClientCode grabs client.go code of component type.
func getClientCode(module modulegen.ModuleInputs) (string, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/viamrobotics/rdk/refs/tags/v%s/%ss/%s/client.go",
		module.SDKVersion, module.ResourceType, module.ResourceSubtype)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get client code")
	}

	//nolint:bodyclose
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get client code")
	}
	defer utils.UncheckedErrorFunc(resp.Body.Close)
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected http GET status: %s getting %s", resp.Status, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return url, errors.Wrapf(err, "error reading response body")
	}
	clientCode := string(body)
	return clientCode, nil
}

// setGoModuleTemplate sets the imports and functions for the go method stubs.
func setGoModuleTemplate(clientCode string, module modulegen.ModuleInputs) (*modulegen.GoModuleTmpl, error) {
	var goTmplInputs modulegen.GoModuleTmpl

	if module.ResourceSubtype == "input" {
		module.ResourceSubtypePascal = "Controller"
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", clientCode, parser.AllErrors)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse client code")
	}

	var imports []string
	for _, imp := range node.Imports {
		path := imp.Path.Value
		// check for the specific import path and set the alias
		if path == `"go.viam.com/rdk/vision"` {
			imp.Name = &ast.Ident{Name: "vis"}
		}
		if imp.Name != nil {
			path = fmt.Sprintf("%s %s", imp.Name.Name, path)
		}
		imports = append(imports, path)
	}
	var functions []string
	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if _, ok := typeSpec.Type.(*ast.StructType); ok {
				if strings.Contains(typeSpec.Name.Name, "Client") {
					functions = append(functions, formatStruct(typeSpec, module.ModuleCamel+module.ModelPascal))
				}
			}
		}
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			name, receiver, args, returns := parseFunctionSignature(
				module.ResourceSubtype,
				module.ModuleCamel+module.ModelPascal,
				funcDecl,
			)
			if name != "" {
				functions = append(functions, formatEmptyFunction(receiver, name, args, returns))
			}
		}
		return true
	})

	// add DoCommand function stub to mlmodel
	if module.ResourceSubtype == "mlmodel" {
		doCommandFunction := formatEmptyFunction(module.ModuleCamel+module.ModelPascal,
			"DoCommand",
			"ctx context.Context, cmd map[string]interface{}",
			[]string{"map[string]interface{}", "error"})
		functions = append(functions, doCommandFunction)
	}

	goTmplInputs.Imports = strings.Join(imports, "\n")
	goTmplInputs.ModelType = module.ModuleCamel + module.ModelPascal
	goTmplInputs.Functions = strings.Join(functions, " ")
	goTmplInputs.Module = module

	return &goTmplInputs, nil
}

// formatType formats typeExpr as readable string with correct attribution if applicable.
func formatType(typeExpr ast.Expr, resourceSubtype string) string {
	var buf bytes.Buffer
	err := printer.Fprint(&buf, token.NewFileSet(), typeExpr)
	if err != nil {
		return fmt.Sprintf("Error formatting type: %v", err)
	}
	typeString := buf.String()

	// checkUpper adds "<resourceSubtype>." to the type if type is capitalized after prefix.
	checkUpper := func(str, prefix string) string {
		prefixLen := len(prefix)
		if unicode.IsUpper(rune(str[prefixLen])) {
			return fmt.Sprintf("%s%s.%s", prefix, resourceSubtype, str[prefixLen:])
		}
		return str
	}
	for _, prefix := range typePrefixes {
		if strings.HasPrefix(typeString, prefix) {
			return checkUpper(typeString, prefix)
		}
	}
	if strings.HasPrefix(typeString, "map[") {
		endStr := strings.Index(typeString, "]")
		keyType := strings.TrimSpace(typeString[4:endStr])
		valueType := strings.TrimSpace(typeString[endStr+1:])
		if unicode.IsUpper(rune(keyType[0])) {
			keyType = checkUpper(keyType, "")
		}
		if unicode.IsUpper(rune(valueType[0])) {
			valueType = checkUpper(valueType, "")
		}
		return fmt.Sprintf("map[%s]%s", keyType, valueType)
	}
	return checkUpper(typeString, "")
}

func formatStruct(typeSpec *ast.TypeSpec, modelType string) string {
	var buf bytes.Buffer
	err := printer.Fprint(&buf, token.NewFileSet(), typeSpec)
	if err != nil {
		return fmt.Sprintf("Error formatting type: %v", err)
	}
	return "type " + strings.ReplaceAll(buf.String(), "*client", "*"+modelType) + "\n\n"
}

// parseFunctionSignature parses function declarations into the function name, the arguments, and the return types.
func parseFunctionSignature(
	resourceSubtype string,
	modelType string,
	funcDecl *ast.FuncDecl,
) (name, receiver, args string, returns []string) {
	if funcDecl == nil {
		return
	}

	// Function name
	funcName := funcDecl.Name.Name
	if !unicode.IsUpper(rune(funcName[0])) {
		return
	}
	if funcName == "Close" || funcName == "Name" || funcName == "Reconfigure" {
		return
	}

	// Receiver
	receiver = modelType
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		field := funcDecl.Recv.List[0]
		if starExpr, ok := field.Type.(*ast.StarExpr); ok {
			if ident, ok := starExpr.X.(*ast.Ident); ok {
				if ident.Name != "client" {
					receiver = ident.Name
				}
			}
		}
	}

	// Parameters
	var params []string
	if funcDecl.Type.Params != nil {
		for _, param := range funcDecl.Type.Params.List {
			paramType := formatType(param.Type, resourceSubtype)
			for _, name := range param.Names {
				params = append(params, name.Name+" "+paramType)
			}
		}
	}

	// Return types
	if funcDecl.Type.Results != nil {
		for _, result := range funcDecl.Type.Results.List {
			str := formatType(result.Type, resourceSubtype)
			// fixing vision service package imports
			if strings.Contains(str, "vision.Object") {
				str = strings.ReplaceAll(str, "vision.Object", "vis.Object")
			}
			returns = append(returns, str)
		}
	}

	return funcName, receiver, strings.Join(params, ", "), returns
}

// formatEmptyFunction outputs the new function that removes the function body, adds the panic unimplemented statement,
// and replaces the receiver with the new model type.
func formatEmptyFunction(receiver, funcName, args string, returns []string) string {
	var returnDef string
	switch {
	case len(returns) == 0:
		returnDef = ""
	case len(returns) == 1:
		returnDef = returns[0]
	default:
		returnDef = fmt.Sprintf("(%s)", strings.Join(returns, ","))
	}
	newFunc := fmt.Sprintf("func (s *%s) %s(%s) %s{\n\tpanic(\"not implemented\")\n}\n\n", receiver, funcName, args, returnDef)
	return newFunc
}

// RenderGoTemplates outputs the method stubs for created module.
func RenderGoTemplates(module modulegen.ModuleInputs) ([]byte, error) {
	clientCode, err := getClientCode(module)
	var empty []byte
	if err != nil {
		return empty, err
	}
	goModule, err := setGoModuleTemplate(clientCode, module)
	if err != nil {
		return empty, err
	}

	var output bytes.Buffer
	tmpl, err := template.New("module").Parse(goTmpl)
	if err != nil {
		return empty, err
	}

	err = tmpl.Execute(&output, goModule)
	if err != nil {
		return empty, err
	}

	return output.Bytes(), nil
}
