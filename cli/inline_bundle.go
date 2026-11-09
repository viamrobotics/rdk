package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	"go.viam.com/utils"
	"golang.org/x/tools/imports"

	"go.viam.com/rdk/cli/module_generate/modulegen"
	gen "go.viam.com/rdk/cli/module_generate/scripts"
)

// Sentinel names passed through the generator, then rewritten to ${inline_...}
// placeholders. They have to be ordinary module names: populateAdditionalInfo
// title-cases its inputs, so a placeholder cannot be the name the generator sees.
const (
	// Ordinary names. Their pascal, camel, snake, and lower forms are all different,
	// and none of those forms appear inside the ${inline_...} placeholders, so a
	// later replacement cannot rewrite a placeholder that was already written.
	inlineNamespace  = "ns-sent"
	inlineModuleName = "mod-sent"
	inlineModelName  = "mdl-sent"
)

// stubSection is one API's imports, registration, and method stubs. A composite
// concatenates several of these into one shell.
type stubSection struct {
	API             string `json:"api"`
	ResourceType    string `json:"resource_type"`
	ResourceSubtype string `json:"resource_subtype"`
	ModelName       string `json:"model_name"`
	EditablePath    string `json:"editable_path"`
	Imports         string `json:"imports"`
	Registration    string `json:"registration"`
	Init            string `json:"init"`
	Bases           string `json:"bases"`
	Methods         string `json:"methods"`
	ReturnType      string `json:"return_type"`
	MainImports     string `json:"main_imports"`
	MainModels      string `json:"main_models"`
}

// languageBundle is the shell for one language plus one stub section per API.
type languageBundle struct {
	Language     string                 `json:"language"`
	SDKVersion   string                 `json:"sdk_version"`
	Shell        map[string]string      `json:"shell"`
	StubSections map[string]stubSection `json:"stub_sections"`
}

type inlineBundleArgs struct {
	Out string
}

// runCommand runs a command in dir. Tests replace it so go get and go mod tidy
// are not executed outside the tag job.
var runCommand = func(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "%s %s: %s", name, strings.Join(args, " "), buf.String())
	}
	return nil
}

// InlineBundleAction builds the inline-module catalogs for one RDK tag and
// writes them under --out. Go stubs come from that tag. go get and go mod tidy
// run once, here, against the same tag.
func InlineBundleAction(ctx context.Context, cmd *cli.Command, args inlineBundleArgs) error {
	out := args.Out
	if out == "" {
		out = "inline-bundle"
	}
	root, err := findSDKRoot(".")
	if err != nil {
		return err
	}
	goVersion, err := resolveGoVersion(ctx)
	if err != nil {
		return err
	}
	goRoot, err := goSDKRoot(ctx, root, goVersion)
	if err != nil {
		return err
	}
	pythonVersion, err := resolvePythonVersion(ctx)
	if err != nil {
		return err
	}
	goBundle, err := buildGoBundle(ctx, goRoot, goVersion)
	if err != nil {
		return err
	}
	pythonBundle, err := buildPythonBundle(ctx, pythonVersion)
	if err != nil {
		return err
	}
	if err := writeLanguageBundle(out, goBundle); err != nil {
		return err
	}
	return writeLanguageBundle(out, pythonBundle)
}

func writeLanguageBundle(dir string, bundle languageBundle) error {
	dest := filepath.Join(dir, bundle.Language, bundle.SDKVersion)
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dest, "catalog.json"), append(payload, '\n'), 0o644) //nolint:gosec
}

func findSDKRoot(start string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("VIAM_INLINE_SDK_ROOT")); env != "" {
		return env, nil
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "components", "motor", "client.go")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("cannot find the RDK checkout (components/motor/client.go)")
		}
		dir = parent
	}
}

// exactCheckoutTag is the tag pointing at HEAD, when there is exactly one.
// Tests replace it. A shallow clone of a tag still reports that tag.
var exactCheckoutTag = func(ctx context.Context, dir string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "describe", "--tags", "--exact-match")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	tag := strings.TrimSpace(string(out))
	if !strings.HasPrefix(tag, "v") {
		return "", false
	}
	return tag, true
}

// lookupModuleDir downloads go.viam.com/rdk at version and returns its module
// cache directory. Tests replace it.
var lookupModuleDir = downloadRDKModule

// goSDKRoot is the tree whose Go APIs are copied into the catalog.
//
// The tag job is checked out at the tag it publishes, and the module proxy may
// not have that tag yet, so the checkout is the source. Any other checkout
// downloads the requested tag. Copying APIs from a newer tree and then
// requiring the older module makes go mod tidy select a newer RDK, because the
// newer tree imports packages the old module does not contain.
func goSDKRoot(ctx context.Context, checkout, version string) (string, error) {
	if tag, ok := exactCheckoutTag(ctx, checkout); ok && tag == version {
		return checkout, nil
	}
	dir, err := lookupModuleDir(ctx, version)
	if err != nil {
		return "", errors.Wrapf(err, "this checkout is not %s", version)
	}
	return dir, nil
}

func downloadRDKModule(ctx context.Context, version string) (string, error) {
	dir, err := os.MkdirTemp("", "inline-rdk-src")
	if err != nil {
		return "", err
	}
	defer utils.UncheckedErrorFunc(func() error { return os.RemoveAll(dir) })
	// The download runs in its own module. Running it inside this repo would
	// treat go.viam.com/rdk as the main module and refuse the version query.
	goMod := "module inlinepin\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil { //nolint:gosec
		return "", err
	}
	cmd := exec.CommandContext(ctx, "go", "mod", "download", "-json", "go.viam.com/rdk@"+version) //nolint:gosec
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(string(out))
		}
		return "", errors.Wrapf(err, "go mod download go.viam.com/rdk@%s: %s", version, detail)
	}
	return moduleDirFromDownload(out, version)
}

func moduleDirFromDownload(out []byte, version string) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var info struct {
			Path    string
			Version string
			Dir     string
			Error   string
		}
		if err := dec.Decode(&info); err != nil {
			if err == io.EOF {
				break
			}
			return "", errors.Wrap(err, "cannot read go mod download output")
		}
		if info.Path != "go.viam.com/rdk" || info.Version != version {
			continue
		}
		if info.Error != "" {
			return "", errors.New(info.Error)
		}
		if info.Dir == "" {
			return "", errors.Errorf("go mod download go.viam.com/rdk@%s returned no directory", version)
		}
		return info.Dir, nil
	}
	return "", errors.Errorf("go mod download did not return go.viam.com/rdk@%s", version)
}

func resolveGoVersion(ctx context.Context) (string, error) {
	if version := strings.TrimSpace(os.Getenv("GITHUB_REF_NAME")); version != "" {
		if !strings.HasPrefix(version, "v") {
			return "", errors.Errorf("GITHUB_REF_NAME %q is not a v* tag", version)
		}
		return version, nil
	}
	out, err := exec.CommandContext(ctx, "git", "describe", "--tags", "--exact-match").Output()
	if err != nil {
		return "", errors.Wrap(err, "GITHUB_REF_NAME is unset and HEAD is not a version tag")
	}
	version := strings.TrimSpace(string(out))
	if !strings.HasPrefix(version, "v") {
		return "", errors.Errorf("git describe returned %q", version)
	}
	return version, nil
}

func resolvePythonVersion(ctx context.Context) (string, error) {
	if version := strings.TrimSpace(os.Getenv("VIAM_INLINE_PYTHON_VERSION")); version != "" {
		return strings.TrimPrefix(version, "v"), nil
	}
	return latestPythonSDKVersion(ctx)
}

func latestPythonSDKVersion(ctx context.Context) (string, error) {
	url := "https://api.github.com/repos/viamrobotics/viam-python-sdk/releases"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	//nolint:bodyclose
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "cannot get latest python SDK release")
	}
	defer utils.UncheckedErrorFunc(resp.Body.Close)
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected http GET status: %s", resp.Status)
	}
	var releases []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", errors.Wrap(err, "could not decode python SDK releases")
	}
	if len(releases) == 0 || releases[0].TagName == "" {
		return "", errors.New("could not get latest python SDK release")
	}
	return strings.TrimPrefix(releases[0].TagName, "v"), nil
}

func inlineModules(language string) ([]modulegen.ModuleInputs, error) {
	modules := make([]modulegen.ModuleInputs, 0, len(modulegen.Resources))
	for _, resource := range modulegen.Resources {
		parts := strings.Split(resource, " ")
		module := modulegen.ModuleInputs{
			ModuleName:      inlineModuleName,
			Language:        language,
			Namespace:       inlineNamespace,
			ModelName:       inlineModelName,
			Visibility:      moduleVisibilityPrivate,
			ResourceSubtype: parts[0],
			ResourceType:    parts[1],
		}
		if err := module.CheckResourceAndSetType(); err != nil {
			return nil, err
		}
		populateAdditionalInfo(&module)
		modules = append(modules, module)
	}
	return modules, nil
}

func buildGoBundle(ctx context.Context, sdkRoot, goVersion string) (languageBundle, error) {
	previous := gen.LocalSDKRoot
	gen.LocalSDKRoot = sdkRoot
	defer func() { gen.LocalSDKRoot = previous }()

	modules, err := inlineModules(golang)
	if err != nil {
		return languageBundle{}, err
	}
	var generic modulegen.ModuleInputs
	foundGeneric := false
	for _, module := range modules {
		if module.API == "rdk:service:generic" {
			generic = module
			foundGeneric = true
			break
		}
	}
	if !foundGeneric {
		return languageBundle{}, errors.New("generic service is not in the generator resource list")
	}

	shell, err := renderLanguageFiles(generic)
	if err != nil {
		return languageBundle{}, err
	}
	slotted, err := gen.RenderGoTemplatesWithSlots(generic, false)
	if err != nil {
		return languageBundle{}, err
	}
	moduleGo, genericSection, err := splitGoModule(string(slotted))
	if err != nil {
		return languageBundle{}, err
	}
	shell["module.go"] = moduleGo
	if mainGo, ok := shell["cmd/module/main.go"]; ok {
		shell["cmd/module/main.go"] = slotGoMain(mainGo, generic)
	}

	sections := make(map[string]stubSection, len(modules))
	for _, module := range modules {
		rendered, renderErr := gen.RenderGoTemplatesWithSlots(module, false)
		if renderErr != nil {
			return languageBundle{}, errors.Wrapf(renderErr, "generate %s", module.API)
		}
		_, section, splitErr := splitGoModule(string(rendered))
		if splitErr != nil {
			return languageBundle{}, errors.Wrapf(splitErr, "split %s", module.API)
		}
		section.API = module.API
		section.ResourceType = module.ResourceType
		section.ResourceSubtype = module.ResourceSubtype
		section.ModelName = module.ResourceSubtype
		section.EditablePath = "module.go"
		section.MainImports = goMainImport(module)
		section.MainModels = goMainModel(module)
		sections[module.API] = section
	}
	// The shell was split from generic, so keep generic's extracted init and registration.
	sections[generic.API] = fillGoIdentity(genericSection, generic)

	// client.go imports are copied verbatim, including ones the stub never uses.
	// goimports drops those and adds the ones the shell needs, such as fmt.
	for api, section := range sections {
		cleaned, cleanErr := tidyGoSectionImports(sdkRoot, shell["module.go"], section)
		if cleanErr != nil {
			return languageBundle{}, cleanErr
		}
		sections[api] = cleaned
	}

	pairs := placeholderPairs(generic)
	for path, content := range shell {
		shell[path] = rewritePlaceholders(content, pairs)
	}
	for api, section := range sections {
		sections[api] = rewriteSection(section, pairs)
	}

	goMod, goSum, err := pinGoModule(ctx, goVersion, importPaths(sections))
	if err != nil {
		return languageBundle{}, err
	}
	// The tidy directory uses a real module path. The catalog copy uses the
	// same placeholder as the rest of the shell.
	shell["go.mod"] = replaceModuleLine(rewritePlaceholders(goMod, pairs), "${inline_module_name_lower}")
	shell["go.sum"] = goSum

	return languageBundle{
		Language:     golang,
		SDKVersion:   goVersion,
		Shell:        shell,
		StubSections: sections,
	}, nil
}

func fillGoIdentity(section stubSection, module modulegen.ModuleInputs) stubSection {
	section.API = module.API
	section.ResourceType = module.ResourceType
	section.ResourceSubtype = module.ResourceSubtype
	section.ModelName = module.ResourceSubtype
	section.EditablePath = "module.go"
	section.MainImports = goMainImport(module)
	section.MainModels = goMainModel(module)
	return section
}

func goMainImport(module modulegen.ModuleInputs) string {
	return fmt.Sprintf("\t%s \"go.viam.com/rdk/%ss/%s\"",
		module.ResourceSubtypeAlias, module.ResourceType, module.ResourceSubtype)
}

func goMainModel(module modulegen.ModuleInputs) string {
	return fmt.Sprintf("resource.APIModel{ %s.API, %s.%s}",
		module.ResourceSubtypeAlias, module.ModuleLowercase, module.ModelPascal)
}

// assembleGoModule splices one API into the shell. Name placeholders are still
// the generator sentinels, so the result is ordinary Go.
func assembleGoModule(shell string, section stubSection) string {
	content := strings.Replace(shell, "${inline_imports}", section.Imports, 1)
	content = strings.Replace(content, "${inline_registration}", section.Registration, 1)
	content = strings.Replace(content, "${inline_init}", section.Init, 1)
	content = strings.Replace(content, "${inline_methods}", section.Methods, 1)
	return strings.ReplaceAll(content, "${inline_return_type}", section.ReturnType)
}

// tidyGoSectionImports runs goimports on the assembled module and keeps the
// import block that actually compiles. The shell still has the example-import
// comments, so those are not stored again on the section.
func tidyGoSectionImports(sdkRoot, shell string, section stubSection) (stubSection, error) {
	formatted, err := imports.Process(
		filepath.Join(sdkRoot, "inline_module.go"),
		[]byte(assembleGoModule(shell, section)),
		nil,
	)
	if err != nil {
		return section, errors.Wrapf(err, "format imports for %s", section.API)
	}
	block, err := parenthesizedImportBlock(string(formatted))
	if err != nil {
		return section, errors.Wrapf(err, "format imports for %s", section.API)
	}
	section.Imports = stripInlineExampleImportComments(block)
	return section, nil
}

func parenthesizedImportBlock(source string) (string, error) {
	const start = "import ("
	i := strings.Index(source, start)
	if i < 0 {
		return "", errors.New("formatted module has no import block")
	}
	rest := source[i+len(start):]
	j := strings.Index(rest, "\n)")
	if j < 0 {
		return "", errors.New("formatted module has an unclosed import block")
	}
	return strings.Trim(rest[:j], "\n"), nil
}

func stripInlineExampleImportComments(body string) string {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "inline-example:imports") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n") + "\n"
}

func replaceModuleLine(mod, modulePath string) string {
	lines := strings.Split(mod, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "module ") {
			lines[i] = "module " + modulePath
			break
		}
	}
	return strings.Join(lines, "\n")
}

func slotGoMain(content string, module modulegen.ModuleInputs) string {
	content = strings.Replace(content, goMainImport(module), "\t${inline_main_imports}", 1)
	return strings.Replace(content, goMainModel(module), "${inline_main_models}", 1)
}

func splitGoModule(rendered string) (string, stubSection, error) {
	var section stubSection
	var err error
	section.Imports, err = extractSlot(rendered, "imports")
	if err != nil {
		return "", section, err
	}
	section.Registration, err = extractSlot(rendered, "registration")
	if err != nil {
		return "", section, err
	}
	section.Init, err = extractSlot(rendered, "init")
	if err != nil {
		return "", section, err
	}
	section.Methods, err = extractSlot(rendered, "methods")
	if err != nil {
		return "", section, err
	}
	section.ReturnType, err = extractSlot(rendered, "return")
	if err != nil {
		return "", section, err
	}
	shell := rendered
	for _, slot := range []struct{ name, repl string }{
		{"imports", "${inline_imports}"},
		{"registration", "${inline_registration}"},
		{"init", "${inline_init}"},
		{"methods", "${inline_methods}"},
	} {
		shell, err = replaceSlot(shell, slot.name, slot.repl)
		if err != nil {
			return "", section, err
		}
	}
	shell, err = replaceAllSlots(shell, "return", "${inline_return_type}")
	if err != nil {
		return "", section, err
	}
	section.Imports = strings.TrimSpace(section.Imports)
	section.Registration = strings.TrimSpace(section.Registration)
	section.Init = strings.TrimSpace(section.Init)
	section.Methods = strings.TrimSpace(section.Methods)
	section.ReturnType = strings.TrimSpace(section.ReturnType)
	return shell, section, nil
}

func extractSlot(content, name string) (string, error) {
	start, end := "/*inline-slot:"+name+"*/", "/*inline-slot:"+name+"-end*/"
	i := strings.Index(content, start)
	if i < 0 {
		return "", errors.Errorf("missing %s", start)
	}
	rest := content[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return "", errors.Errorf("missing %s", end)
	}
	return rest[:j], nil
}

func replaceSlot(content, name, repl string) (string, error) {
	start, end := "/*inline-slot:"+name+"*/", "/*inline-slot:"+name+"-end*/"
	i := strings.Index(content, start)
	if i < 0 {
		return "", errors.Errorf("missing %s", start)
	}
	j := strings.Index(content[i:], end)
	if j < 0 {
		return "", errors.Errorf("missing %s", end)
	}
	return content[:i] + repl + content[i+j+len(end):], nil
}

func replaceAllSlots(content, name, repl string) (string, error) {
	var err error
	start := "/*inline-slot:" + name + "*/"
	for strings.Contains(content, start) {
		content, err = replaceSlot(content, name, repl)
		if err != nil {
			return "", err
		}
	}
	return content, nil
}

func placeholderPairs(module modulegen.ModuleInputs) [][2]string {
	triple := module.Namespace + ":" + module.ModuleName + ":" + module.ModelName
	return [][2]string{
		{triple, "${inline_namespace}:${inline_module_name}:${inline_model_name}"},
		{module.ModulePascal, "${inline_module_name_pascal}"},
		{module.ModuleCamel, "${inline_module_name_camel}"},
		{module.ModuleSnake, "${inline_module_name_snake}"},
		{module.ModuleLowercase, "${inline_module_name_lower}"},
		{module.ModuleName, "${inline_module_name}"},
		{module.ModelPascal, "${inline_model_name_pascal}"},
		{module.ModelCamel, "${inline_model_name_camel}"},
		{module.ModelSnake, "${inline_model_name_snake}"},
		{module.ModelName, "${inline_model_name}"},
		{module.Namespace, "${inline_namespace}"},
	}
}

func rewritePlaceholders(content string, pairs [][2]string) string {
	for _, pair := range pairs {
		content = strings.ReplaceAll(content, pair[0], pair[1])
	}
	return content
}

func rewriteSection(section stubSection, pairs [][2]string) stubSection {
	section.Imports = rewritePlaceholders(section.Imports, pairs)
	section.Registration = rewritePlaceholders(section.Registration, pairs)
	section.Init = rewritePlaceholders(section.Init, pairs)
	section.Bases = rewritePlaceholders(section.Bases, pairs)
	section.Methods = rewritePlaceholders(section.Methods, pairs)
	section.ReturnType = rewritePlaceholders(section.ReturnType, pairs)
	section.MainImports = rewritePlaceholders(section.MainImports, pairs)
	section.MainModels = rewritePlaceholders(section.MainModels, pairs)
	section.EditablePath = rewritePlaceholders(section.EditablePath, pairs)
	return section
}

var quotedImportRE = regexp.MustCompile(`"([^"]+)"`)

func importPaths(sections map[string]stubSection) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, section := range sections {
		for _, match := range quotedImportRE.FindAllStringSubmatch(section.Imports, -1) {
			importPath := match[1]
			if !strings.Contains(importPath, ".") {
				continue
			}
			if _, ok := seen[importPath]; ok {
				continue
			}
			seen[importPath] = struct{}{}
			paths = append(paths, importPath)
		}
		apiImport := fmt.Sprintf("go.viam.com/rdk/%ss/%s", section.ResourceType, section.ResourceSubtype)
		if _, ok := seen[apiImport]; !ok {
			seen[apiImport] = struct{}{}
			paths = append(paths, apiImport)
		}
	}
	return paths
}

// pinGoModule writes a module that imports every generated Go API, requires the
// triggering tag, and tidies once. The returned go.mod still requires that tag.
func pinGoModule(ctx context.Context, goVersion string, paths []string) (string, string, error) {
	dir, err := os.MkdirTemp("", "inline-gomod")
	if err != nil {
		return "", "", err
	}
	defer utils.UncheckedErrorFunc(func() error { return os.RemoveAll(dir) })

	var source bytes.Buffer
	source.WriteString("package inlinemodule\n\nimport (\n")
	for _, importPath := range paths {
		fmt.Fprintf(&source, "\t_ %q\n", importPath)
	}
	source.WriteString(")\n")
	if err := os.WriteFile(filepath.Join(dir, "tidy.go"), source.Bytes(), 0o644); err != nil { //nolint:gosec
		return "", "", err
	}
	// A real module path. ${inline_module_name_lower} is not a path go get can fetch.
	// replaceModuleLine writes that placeholder onto the catalog copy after tidy.
	goMod := fmt.Sprintf("module inlinemodule\n\ngo 1.25\n\nrequire go.viam.com/rdk %s\n", goVersion)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil { //nolint:gosec
		return "", "", err
	}
	if err := runCommand(ctx, dir, "go", "get", "go.viam.com/rdk@"+goVersion); err != nil {
		return "", "", err
	}
	if err := runCommand(ctx, dir, "go", "mod", "tidy"); err != nil {
		return "", "", err
	}
	modBytes, err := os.ReadFile(filepath.Join(dir, "go.mod")) //nolint:gosec
	if err != nil {
		return "", "", err
	}
	sumBytes, err := os.ReadFile(filepath.Join(dir, "go.sum")) //nolint:gosec
	if err != nil {
		return "", "", err
	}
	if !goModRequires(string(modBytes), "go.viam.com/rdk", goVersion) {
		resolved := requiredVersion(string(modBytes), "go.viam.com/rdk")
		if resolved == "" {
			resolved = "no version"
		}
		return "", "", errors.Errorf(
			"go mod tidy resolved go.viam.com/rdk %s instead of %s; the stubs import a package that %s does not contain",
			resolved, goVersion, goVersion,
		)
	}
	return string(modBytes), string(sumBytes), nil
}

// goModRequires reports whether mod requires modulePath at version.
// go mod tidy rewrites a single require line into a parenthesized block, so both forms count.
func goModRequires(mod, modulePath, version string) bool {
	return requiredVersion(mod, modulePath) == version
}

// requiredVersion is the version modulePath is required at, or empty.
func requiredVersion(mod, modulePath string) string {
	for _, line := range strings.Split(mod, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "require" && fields[1] == modulePath {
			return fields[2]
		}
		if len(fields) >= 2 && fields[0] == modulePath {
			return fields[1]
		}
	}
	return ""
}

func renderLanguageFiles(module modulegen.ModuleInputs) (map[string]string, error) {
	files := map[string]string{}
	languagePath := path.Join(templatesPath, module.Language)
	tempDir, err := fs.Sub(templates, languagePath)
	if err != nil {
		return nil, err
	}
	err = fs.WalkDir(tempDir, ".", func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		data, readErr := fs.ReadFile(tempDir, filePath)
		if readErr != nil {
			return readErr
		}
		dest := filepath.ToSlash(strings.ReplaceAll(filePath, templatePrefix, ""))
		if strings.HasPrefix(d.Name(), templatePrefix) {
			rendered, renderErr := renderTextTemplate(filePath, string(data), module)
			if renderErr != nil {
				return renderErr
			}
			files[dest] = rendered
			return nil
		}
		files[dest] = string(data)
		return nil
	})
	return files, err
}

func renderTextTemplate(name, raw string, module modulegen.ModuleInputs) (string, error) {
	tmpl, err := template.New(name).Parse(raw)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, module); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type pyRendered struct {
	ResourceType    string `json:"resource_type"`
	ResourceSubtype string `json:"resource_subtype"`
	File            string `json:"file"`
}

func buildPythonBundle(ctx context.Context, pythonVersion string) (languageBundle, error) {
	modules, err := inlineModules(python)
	if err != nil {
		return languageBundle{}, err
	}
	pythonCmd := findPythonCommand()
	if pythonCmd == "" {
		return languageBundle{}, errors.New("python runtime not found")
	}
	rendered, err := renderPythonSlots(ctx, pythonCmd, pythonVersion, modules)
	if err != nil {
		return languageBundle{}, err
	}
	bySubtype := map[string]pyRendered{}
	for _, item := range rendered {
		bySubtype[item.ResourceType+"/"+item.ResourceSubtype] = item
	}

	var generic modulegen.ModuleInputs
	for _, module := range modules {
		if module.API == "rdk:service:generic" {
			generic = module
			break
		}
	}
	generic.SDKVersion = pythonVersion
	renderedShell, err := renderLanguageFiles(generic)
	if err != nil {
		return languageBundle{}, err
	}
	delete(renderedShell, "src/models/resource.py")

	genericFile, ok := bySubtype[generic.ResourceType+"/"+generic.ResourceSubtype]
	if !ok {
		return languageBundle{}, errors.New("python bundle did not include generic service")
	}
	modelShell, genericSection, err := splitPythonModel(genericFile.File)
	if err != nil {
		return languageBundle{}, err
	}
	pairs := placeholderPairs(generic)
	shell := map[string]string{}
	for filePath, content := range renderedShell {
		shell[rewritePlaceholders(filePath, pairs)] = rewritePlaceholders(content, pairs)
	}
	shell["src/models/${inline_model_name_snake}.py"] = rewritePlaceholders(modelShell, pairs)

	sections := map[string]stubSection{}
	for _, module := range modules {
		item, found := bySubtype[module.ResourceType+"/"+module.ResourceSubtype]
		if !found {
			// generate_stubs.py skips an API the installed SDK cannot import.
			fmt.Fprintf(os.Stderr, "inline bundle: python SDK %s has no %s\n", pythonVersion, module.API)
			continue
		}
		_, section, splitErr := splitPythonModel(item.File)
		if splitErr != nil {
			return languageBundle{}, errors.Wrapf(splitErr, "split %s", module.API)
		}
		section.API = module.API
		section.ResourceType = module.ResourceType
		section.ResourceSubtype = module.ResourceSubtype
		section.ModelName = module.ResourceSubtype
		section.EditablePath = "src/models/${inline_model_name_snake}.py"
		sections[module.API] = rewriteSection(section, pairs)
	}
	genericSection.API = generic.API
	genericSection.ResourceType = generic.ResourceType
	genericSection.ResourceSubtype = generic.ResourceSubtype
	genericSection.ModelName = generic.ResourceSubtype
	genericSection.EditablePath = "src/models/${inline_model_name_snake}.py"
	sections[generic.API] = rewriteSection(genericSection, pairs)

	return languageBundle{
		Language:     python,
		SDKVersion:   pythonVersion,
		Shell:        shell,
		StubSections: sections,
	}, nil
}

func splitPythonModel(file string) (string, stubSection, error) {
	var section stubSection
	var err error
	section.Imports, err = extractSlot(file, "imports")
	if err != nil {
		return "", section, err
	}
	section.Bases, err = extractSlot(file, "bases")
	if err != nil {
		return "", section, err
	}
	section.Methods, err = extractSlot(file, "methods")
	if err != nil {
		return "", section, err
	}
	shell, err := replaceSlot(file, "imports", "${inline_imports}")
	if err != nil {
		return "", section, err
	}
	shell, err = replaceSlot(shell, "bases", "${inline_bases}")
	if err != nil {
		return "", section, err
	}
	shell, err = replaceSlot(shell, "methods", "${inline_methods}")
	if err != nil {
		return "", section, err
	}
	section.Imports = strings.TrimSpace(section.Imports)
	section.Bases = strings.TrimSpace(section.Bases)
	// Keep the class indent. TrimSpace would unindent only the first method.
	section.Methods = trimKeepingIndent(section.Methods)
	return shell, section, nil
}

// trimKeepingIndent drops blank lines around a slot without unindenting code.
func trimKeepingIndent(content string) string {
	return strings.TrimRight(strings.Trim(content, "\n"), " \t")
}

func renderPythonSlots(ctx context.Context, pythonCmd, version string, modules []modulegen.ModuleInputs) ([]pyRendered, error) {
	script, err := scripts.ReadFile(path.Join(scriptsPath, "generate_stubs.py"))
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "inline-python")
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(func() error { return os.RemoveAll(dir) })
	scriptPath := filepath.Join(dir, "generate_stubs.py")
	if err := os.WriteFile(scriptPath, script, 0o700); err != nil { //nolint:gosec
		return nil, err
	}
	type request struct {
		ResourceType    string `json:"resource_type"`
		ResourceSubtype string `json:"resource_subtype"`
		Namespace       string `json:"namespace"`
		ModuleName      string `json:"module_name"`
		ModelName       string `json:"model_name"`
	}
	requests := make([]request, 0, len(modules))
	for _, module := range modules {
		requests = append(requests, request{
			ResourceType:    module.ResourceType,
			ResourceSubtype: module.ResourceSubtype,
			Namespace:       module.Namespace,
			ModuleName:      module.ModuleName,
			ModelName:       module.ModelName,
		})
	}
	payload, err := json.Marshal(requests)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, pythonCmd, scriptPath, "--bundle", version) //nolint:gosec
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = append(os.Environ(), "VIAM_PYTHON_SDK_VERSION="+version)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "python stub bundle: %s", stderr.String())
	}
	var rendered []pyRendered
	if err := json.Unmarshal(out, &rendered); err != nil {
		return nil, errors.Wrapf(err, "python stub bundle returned %s", truncate(string(out), 400))
	}
	return rendered, nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
