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
	"golang.org/x/tools/go/ast/astutil"

	"go.viam.com/rdk/cli/module_generate/modulegen"
)

//go:embed tmpl-module
var goTmpl string

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

func cppSDKBaseURL(version string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/viamrobotics/viam-cpp-sdk/refs/tags/releases/v%s", version)
}

// We upload the conan.lock as a release artifact but it does not exist in the repo itself, so we need a different URL
func cppSDKReleaseURL(version string) string {
	return fmt.Sprintf("https://github.com/viamrobotics/viam-cpp-sdk/releases/download/releases/v%s", version)
}

// FetchRawTemplate fetches the content at url and returns it as a string.
// It is a package-level var to allow overriding in tests.
var FetchRawTemplate = func(url string) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", errors.Wrapf(err, "cannot fetch template from %s", url)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "cannot fetch template from %s", url)
	}
	defer utils.UncheckedErrorFunc(resp.Body.Close)
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected http GET status: %s getting %s", resp.Status, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "error reading response body from %s", url)
	}
	return string(body), nil
}

func getCppTemplate(base string) (string, error) {
	return FetchRawTemplate(base + "/res/module_generator/_templates/main.cpp.in")
}

func getCppCMakeTemplate(base string) (string, error) {
	return FetchRawTemplate(base + "/res/module_generator/_templates/CMakeLists.txt.in")
}

func getCppConanTemplate(base string) (string, error) {
	return FetchRawTemplate(base + "/res/module_generator/_templates/conanfile.py.in")
}

func getCppTypeTemplate(base string, module modulegen.ModuleInputs) (string, error) {
	url := fmt.Sprintf("%s/res/module_generator/_templates/%ss/%s.cpp.in", base, module.ResourceType, module.ResourceSubtypeSnake)
	return FetchRawTemplate(url)
}

func getCppTypeHeaderTemplate(base string, module modulegen.ModuleInputs) (string, error) {
	url := fmt.Sprintf("%s/res/module_generator/_templates/%ss/%s.hpp.in", base, module.ResourceType, module.ResourceSubtypeSnake)
	return FetchRawTemplate(url)
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
	additionalModel bool,
) (*modulegen.GoModuleTmpl, error) {
	var goTmplInputs modulegen.GoModuleTmpl
	if additionalModel {
		goTmplInputs.ConfigType = module.ModelPascal + "Config"
		goTmplInputs.IsAdditionalModel = true
	} else {
		goTmplInputs.ConfigType = "Config"
	}

	if module.ResourceSubtype == "input" {
		module.ResourceSubtypePascal = "Controller"
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", clientCode, parser.AllErrors)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse client code")
	}

	helpers, helperFields := findHelperConstructors(node)

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
					functions = append(functions, formatStruct(typeSpec, module.ModuleCamel+module.ModelPascal, helperFields))
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
				body := formatNotImplementedBody(returns)
				if h, ok := helpers[name]; ok && receiver == module.ModuleCamel+module.ModelPascal {
					body = h.body(receiver)
				}
				functions = append(functions, formatFunctionWithDoc(doc, receiver, name, args, returns, body))
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

// formatType formats typeExpr as a readable string. Types declared in the resource's own package
// are unqualified in client.go, so every exported identifier that is not already package-qualified
// gets the resource package prefix, at any depth (e.g. <-chan []Point).
func formatType(typeExpr ast.Expr, resourceSubtype string) string {
	if resourceSubtype == "switch" {
		resourceSubtype = "sw"
	}
	qualified := astutil.Apply(typeExpr, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.SelectorExpr:
			return false
		case *ast.Ident:
			// Names of func-typed params and results are not types.
			if n.IsExported() && c.Name() != "Names" {
				c.Replace(&ast.SelectorExpr{X: ast.NewIdent(resourceSubtype), Sel: ast.NewIdent(n.Name)})
			}
		}
		return true
	}, nil)

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), qualified); err != nil {
		return fmt.Sprintf("Error formatting type: %v", err)
	}
	return buf.String()
}

// helperConstructor is a client method that returns a sub-resource helper struct, such as
// board's AnalogByName returning &analogClient{...}.
type helperConstructor struct {
	typeName string
	// fields are the struct fields set from the method's parameters, as "field: param".
	fields    []string
	extraNils int
}

// body returns a stub body that builds the helper around the model, which keeps the helper type
// and its method stubs reachable.
func (h helperConstructor) body(modelType string) string {
	fields := append([]string{modelType + ": s"}, h.fields...)
	return fmt.Sprintf("\treturn &%s{%s}%s", h.typeName, strings.Join(fields, ", "), strings.Repeat(", nil", h.extraNils))
}

// findHelperConstructors finds client methods whose body is a single `return &fooClient{...}, nil`,
// keyed by method name. It also returns, per helper type, the fields those constructors set from
// method parameters. Other fields are copied from client state the generated model does not have,
// so formatStruct drops them.
func findHelperConstructors(node *ast.File) (map[string]helperConstructor, map[string]map[string]bool) {
	helpers := map[string]helperConstructor{}
	fields := map[string]map[string]bool{}
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || funcDecl.Body == nil || len(funcDecl.Body.List) != 1 {
			continue
		}
		ret, ok := funcDecl.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		unary, ok := ret.Results[0].(*ast.UnaryExpr)
		if !ok || unary.Op != token.AND {
			continue
		}
		lit, ok := unary.X.(*ast.CompositeLit)
		if !ok {
			continue
		}
		typeIdent, ok := lit.Type.(*ast.Ident)
		if !ok || !strings.Contains(typeIdent.Name, "Client") {
			continue
		}
		restNil := true
		for _, r := range ret.Results[1:] {
			if ident, ok := r.(*ast.Ident); !ok || ident.Name != "nil" {
				restNil = false
			}
		}
		if !restNil {
			continue
		}

		params := map[string]bool{}
		for _, p := range funcDecl.Type.Params.List {
			for _, n := range p.Names {
				params[n.Name] = true
			}
		}
		h := helperConstructor{typeName: typeIdent.Name, extraNils: len(ret.Results) - 1}
		if fields[h.typeName] == nil {
			fields[h.typeName] = map[string]bool{}
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, keyOK := kv.Key.(*ast.Ident)
			val, valOK := kv.Value.(*ast.Ident)
			if keyOK && valOK && params[val.Name] {
				h.fields = append(h.fields, key.Name+": "+val.Name)
				fields[h.typeName][key.Name] = true
			}
		}
		helpers[funcDecl.Name.Name] = h
	}
	return helpers, fields
}

// formatStruct formats a helper struct from client.go, embedding the model in place of the client.
// For a helper that a constructor builds, only the fields that constructor sets are kept.
func formatStruct(typeSpec *ast.TypeSpec, modelType string, helperFields map[string]map[string]bool) string {
	if kept, ok := helperFields[typeSpec.Name.Name]; ok {
		if st, ok := typeSpec.Type.(*ast.StructType); ok {
			var list []*ast.Field
			for _, f := range st.Fields.List {
				if len(f.Names) == 0 || kept[f.Names[0].Name] {
					list = append(list, f)
				}
			}
			st.Fields.List = list
		}
	}
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
		return name, receiver, args, returns
	}

	// Function name
	funcName := funcDecl.Name.Name
	if !unicode.IsUpper(rune(funcName[0])) {
		return name, receiver, args, returns
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

	// The module template already defines these on the model. Helper types still need them,
	// e.g. board.DigitalInterrupt requires Name() string.
	if receiver == modelType && (funcName == "Close" || funcName == "Name" || funcName == "Reconfigure") {
		return "", "", "", nil
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
			strings.HasPrefix(typ, "func(") ||
			strings.HasPrefix(typ, "chan ") ||
			strings.HasPrefix(typ, "chan<- ") ||
			strings.HasPrefix(typ, "<-chan ") ||
			strings.HasPrefix(typ, "interface{") ||
			typ == "any" {
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
// It returns zero values and errUnimplemented according to the function's return types.
func formatNotImplementedBody(returns []string) string {
	switch len(returns) {
	case 0:
		return "\t// not implemented"
	case 1:
		if returns[0] == "error" {
			return "\treturn errUnimplemented"
		}
		returnVar, needsVar := zeroValueForType(returns[0], 1)
		if needsVar {
			return fmt.Sprintf("\tvar %s %s\n\treturn %s", returnVar, returns[0], returnVar)
		} else { //nolint: revive
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
				vals[i] = "errUnimplemented"
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

// formatFunctionWithDoc generates a method with the given body, preceded by the component
// interface's doc comment if it has one.
func formatFunctionWithDoc(doc, receiver, funcName, args string, returns []string, body string) string {
	returnDef := formatReturnDef(returns)

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

// renderCppTemplate fetches and executes a single C++ template, returning the rendered bytes.
func renderCppTemplate(name, rawTmpl string, module modulegen.ModuleInputs) ([]byte, error) {
	tmpl, err := template.New(name).Parse(rawTmpl)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err = tmpl.Execute(&output, module); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// RenderCppTemplates outputs the rendered C++ files for the created module.
func RenderCppTemplates(module modulegen.ModuleInputs) (modulegen.CppRenderedFiles, error) {
	base := cppSDKBaseURL(module.SDKVersion)

	mainRaw, err := getCppTemplate(base)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}
	main, err := renderCppTemplate("main", mainRaw, module)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}

	typeRaw, err := getCppTypeTemplate(base, module)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}
	typeCpp, err := renderCppTemplate("type", typeRaw, module)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}

	headerRaw, err := getCppTypeHeaderTemplate(base, module)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}
	header, err := renderCppTemplate("header", headerRaw, module)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}

	cmakeRaw, err := getCppCMakeTemplate(base)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}
	cmake, err := renderCppTemplate("cmake", cmakeRaw, module)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}

	conanRaw, err := getCppConanTemplate(base)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}
	conan, err := renderCppTemplate("conan", conanRaw, module)
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}

	conanLockRaw, err := FetchRawTemplate(cppSDKReleaseURL(module.SDKVersion) + "/conan.lock")
	if err != nil {
		return modulegen.CppRenderedFiles{}, err
	}

	return modulegen.CppRenderedFiles{
		Main:       main,
		Type:       typeCpp,
		Header:     header,
		CMakeLists: cmake,
		ConanFile:  conan,
		ConanLock:  []byte(conanLockRaw),
	}, nil
}

// RenderGoTemplates outputs the method stubs for created module.
// additionalModel must be true when generating a second (or later) model in the same package,
// which changes Config → <ModelPascal>Config and omits the package-level errUnimplemented.
func RenderGoTemplates(module modulegen.ModuleInputs, additionalModel bool) ([]byte, error) {
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
	goModule, err := setGoModuleTemplate(clientCode, module, docMap, additionalModel)
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
