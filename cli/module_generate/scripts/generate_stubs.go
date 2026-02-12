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

// CreateGetClientCodeRequest creates a request to get the client code of the specified resource type.
var CreateGetClientCodeRequest = func(module modulegen.ModuleInputs) (*http.Request, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/viamrobotics/rdk/refs/tags/v%s/%ss/%s/client.go",
		module.SDKVersion, module.ResourceType, module.ResourceSubtype)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get client code")
	}
	return req, nil
}

// getClientCode grabs client.go code of component type.
func getClientCode(module modulegen.ModuleInputs) (string, error) {
	req, err := CreateGetClientCodeRequest(module)
	if err != nil {
		return "", err
	}

	//nolint:bodyclose
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get client code")
	}
	defer utils.UncheckedErrorFunc(resp.Body.Close)
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected http GET status: %s getting %s", resp.Status, req.URL.String())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return req.URL.String(), errors.Wrapf(err, "error reading response body")
	}
	clientCode := string(body)
	return clientCode, nil
}

// CreateGetResourceCodeRequest creates a request to get the resource code of the specified resource type.
var CreateGetResourceCodeRequest = func(module modulegen.ModuleInputs, usesnake bool) (*http.Request, error) {
	subtype := module.ResourceSubtype
	if usesnake {
		subtype = module.ResourceSubtypeSnake
	}
	url := fmt.Sprintf("https://raw.githubusercontent.com/viamrobotics/rdk/refs/tags/v%s/%ss/%s/%s.go",
		module.SDKVersion, module.ResourceType, module.ResourceSubtype, subtype)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get resource code")
	}
	return req, nil
}

// getResourceCode fetches the resource.go source code.
func getResourceCode(module modulegen.ModuleInputs) (string, error) {
	// It tries twice: first with the snake cased resource sub type, then without.
	// Different components are named with and without snake case, this tries both before returning an error
	attempts := []bool{true, false}

	var resp *http.Response
	var req *http.Request
	var err error

	for _, usesnake := range attempts {
		// Create the HTTP request for fetching resource code
		req, err = CreateGetResourceCodeRequest(module, usesnake)
		if err != nil {
			return "", err
		}

		// Send the HTTP request
		//nolint:bodyclose
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return "", errors.Wrapf(err, "cannot get resource code")
		}

		if resp.StatusCode == http.StatusOK {
			// Read and return the response body if the request succeeded
			defer utils.UncheckedErrorFunc(resp.Body.Close)
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return req.URL.String(), errors.Wrapf(err, "error reading response body")
			}
			resourceCode := string(body)
			return resourceCode, nil
		}

		// Close response body if not OK to prevent leaks before retrying
		utils.UncheckedErrorFunc(resp.Body.Close)
	}
	return "", errors.Errorf("unexpected http GET status: %s getting %s", resp.Status, req.URL.String())
}

// extractInterfaceMethodDocs parses Go source code and returns a map of
// method names to their associated documentation comments for the first
// interface type found in the code.
func extractInterfaceMethodDocs(resourceCode string) (map[string]*ast.CommentGroup, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", resourceCode, parser.ParseComments)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse resource code")
	}

	docMap := make(map[string]*ast.CommentGroup)

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if genDecl.Tok != token.TYPE {
			continue
		}

		if len(genDecl.Specs) == 0 {
			continue
		}

		spec := genDecl.Specs[0]
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		if ifaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
			for _, method := range ifaceType.Methods.List {
				if len(method.Names) == 0 {
					continue
				}
				methodName := method.Names[0].Name
				docMap[methodName] = method.Doc
			}
			break
		}
	}

	return docMap, nil
}

// setGoModuleTemplate sets the imports and functions for the go method stubs.
func setGoModuleTemplate(
	clientCode string,
	module modulegen.ModuleInputs,
	docMap map[string]*ast.CommentGroup,
) (*modulegen.GoModuleTmpl, error) {
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
			if name != "" && name != "NewClientFromConn" {
				var doc string
				if docGroup, ok := docMap[name]; ok && docGroup != nil {
					doc = docGroup.Text()
				}
				functions = append(functions, formatEmptyFunctionWithDoc(doc, receiver, name, args, returns))
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
	if resourceSubtype == "switch" {
		resourceSubtype = "sw"
	}

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

// formatReturnDef converts a slice of return types into a properly formatted Go return type string.
func formatReturnDef(returns []string) string {
	switch len(returns) {
	case 0:
		return ""
	case 1:
		return " " + returns[0]
	default:
		return " (" + strings.Join(returns, ", ") + ")"
	}
}

// zeroValueForType returns the zero value literal as a string for the given Go type.
// If no literal zero value exists, it returns a generated variable name
// that will later be declared and returned as the empty value for that type.
func zeroValueForType(typ string, suffix int) (string, bool) {
	// for basic built-in types use their literal zero values
	switch typ {
	case "string":
		return `""`, false
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "complex64", "complex128", "byte", "rune":
		return "0", false
	case "bool":
		return "false", false
	default:
		// return nil for pointer, slice, map, and function types
		if strings.HasPrefix(typ, "*") ||
			strings.HasPrefix(typ, "[]") ||
			strings.HasPrefix(typ, "map[") ||
			strings.HasPrefix(typ, "func(") {
			return "nil", false
		}
		// otherwise, generate a variable name like "myTypeRetVal"
		// that will later be declared and returned as the empty value for that type.
		varName := varNameFromType(typ + "RetVal")
		// add suffix if multiple values of the same type are being returned
		if suffix > 1 {
			varName += fmt.Sprintf("%d", suffix)
		}
		return varName, true
	}
}

// varNameFromType returns a variable-style name from a type name,
// lowercasing the first letter and stripping any package prefix.
func varNameFromType(typ string) string {
	if typ == "" {
		return "ret"
	}
	// strip package prefix if present ("pkg.Type" will become "Type")
	if idx := strings.LastIndex(typ, "."); idx != -1 {
		typ = typ[idx+1:]
	}
	// lowercase the first character of the type name
	chars := []rune(typ)
	chars[0] = unicode.ToLower(chars[0])
	// This return is a valid Go variable name to be used as an empty return value
	// when a literal zero value isn't available.
	return string(chars)
}

// formatNotImplementedBody generates the Go function body for an unimplemented stub.
// It returns zero values and a "not implemented" error according to the function's return types.
func formatNotImplementedBody(returns []string) string {
	switch len(returns) {
	case 0:
		return "\t// not implemented"
	case 1:
		if returns[0] == "error" {
			return "\treturn fmt.Errorf(\"not implemented\")"
		}
		returnVar, needsVar := zeroValueForType(returns[0], 1)
		if needsVar {
			return fmt.Sprintf("\tvar %s %s\n\treturn %s", returnVar, returns[0], returnVar)
		} else {
			return "\treturn " + returnVar
		}
	default:
		typeCount := make(map[string]int)
		vals := make([]string, len(returns))
		var vars []string
		for i, r := range returns {
			typeCount[r]++
			returnVar, needsVar := zeroValueForType(r, typeCount[r])
			if r == "error" {
				vals[i] = "fmt.Errorf(\"not implemented\")"
			} else {
				if needsVar {
					vars = append(vars, fmt.Sprintf("\tvar %s %s\n", returnVar, r))
				}
				vals[i] = returnVar
			}
		}
		body := ""
		if len(vars) > 0 {
			body += strings.Join(vars, "\n") + "\n"
		}
		body += "\treturn " + strings.Join(vals, ", ")
		return body
	}
}

// formatEmptyFunction generates a stub method for the given receiver,
// inserting a "not implemented" body with appropriate zero-value returns.
func formatEmptyFunction(receiver, funcName, args string, returns []string) string {
	returnDef := formatReturnDef(returns)
	body := formatNotImplementedBody(returns)
	return fmt.Sprintf("func (s *%s) %s(%s)%s {\n%s\n}\n\n", receiver, funcName, args, returnDef, body)
}

// formatEmptyFunctionWithDoc does the same as formatEmptyFunction but adds doc comment if the component interface has one.
func formatEmptyFunctionWithDoc(doc, receiver, funcName, args string, returns []string) string {
	returnDef := formatReturnDef(returns)
	body := formatNotImplementedBody(returns)

	var docComment string
	if doc != "" {
		doc = strings.TrimSpace(doc)
		lines := strings.Split(doc, "\n")
		for i := range lines {
			lines[i] = "// " + strings.TrimSpace(lines[i])
		}
		docComment = strings.Join(lines, "\n") + "\n"
	}

	return fmt.Sprintf("%sfunc (s *%s) %s(%s)%s {\n%s\n}\n\n", docComment, receiver, funcName, args, returnDef, body)
}

// RenderGoTemplates outputs the method stubs for created module.
func RenderGoTemplates(module modulegen.ModuleInputs) ([]byte, error) {
	clientCode, err := getClientCode(module)
	var empty []byte
	if err != nil {
		return empty, err
	}
	resourceCode, err := getResourceCode(module)
	if err != nil {
		return empty, err
	}
	docMap, err := extractInterfaceMethodDocs(resourceCode)
	if err != nil {
		return empty, err
	}
	goModule, err := setGoModuleTemplate(clientCode, module, docMap)
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
