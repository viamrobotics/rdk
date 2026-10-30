package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/cli/module_generate/modulegen"
	gen "go.viam.com/rdk/cli/module_generate/scripts"
)

func TestRewritePlaceholdersRemovesGeneratorNames(t *testing.T) {
	modules, err := inlineModules(golang)
	test.That(t, err, test.ShouldBeNil)
	module := modules[0]
	pairs := placeholderPairs(module)
	raw := strings.Join([]string{
		module.Namespace + ":" + module.ModuleName + ":" + module.ModelName,
		module.ModulePascal,
		module.ModuleCamel,
		module.ModuleSnake,
		module.ModuleLowercase,
		module.ModuleName,
		module.ModelPascal,
		module.ModelCamel,
		module.ModelSnake,
		module.ModelName,
		module.Namespace,
	}, " ")
	rewritten := rewritePlaceholders(raw, pairs)
	for _, placeholder := range []string{
		"${inline_namespace}",
		"${inline_module_name}",
		"${inline_module_name_pascal}",
		"${inline_module_name_camel}",
		"${inline_module_name_snake}",
		"${inline_module_name_lower}",
		"${inline_model_name}",
		"${inline_model_name_pascal}",
		"${inline_model_name_camel}",
		"${inline_model_name_snake}",
	} {
		test.That(t, rewritten, test.ShouldContainSubstring, placeholder)
	}
	assertNoGeneratorNames(t, rewritten, pairs)
}

func TestGoBundleUsesLocalStubsAndPinsTheTag(t *testing.T) {
	root := rdkRoot(t)
	const tag = "v1.2.3"
	var calls []string
	previous := runCommand
	runCommand = func(dir, name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if len(args) > 0 && args[0] == "mod" {
			// tidy rewrites a single require line into a block. The checker has to accept that.
			block := "module inlinemodule\n\ngo 1.25.0\n\nrequire (\n\tgo.viam.com/rdk " + tag + "\n)\n"
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(block), 0o644); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, "go.sum"), []byte("go.viam.com/rdk "+tag+" h1:abc\n"), 0o644)
		}
		return nil
	}
	t.Cleanup(func() { runCommand = previous })

	bundle, err := buildGoBundle(root, tag)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bundle.SDKVersion, test.ShouldEqual, tag)
	test.That(t, calls, test.ShouldResemble, []string{
		"go get go.viam.com/rdk@" + tag,
		"go mod tidy",
	})
	test.That(t, goModRequires(bundle.Shell["go.mod"], "go.viam.com/rdk", tag), test.ShouldBeTrue)
	test.That(t, bundle.Shell["go.mod"], test.ShouldContainSubstring, "go.viam.com/rdk "+tag)
	test.That(t, bundle.Shell["go.sum"], test.ShouldContainSubstring, "go.viam.com/rdk "+tag)

	shell := bundle.Shell["module.go"]
	for _, token := range []string{
		"${inline_imports}",
		"${inline_registration}",
		"${inline_init}",
		"${inline_methods}",
		"${inline_return_type}",
		"${inline_module_name_pascal}",
		"${inline_model_name_pascal}",
	} {
		test.That(t, shell, test.ShouldContainSubstring, token)
	}
	test.That(t, shell, test.ShouldNotContainSubstring, "/*inline-slot:")
	modules, modErr := inlineModules(golang)
	test.That(t, modErr, test.ShouldBeNil)
	pairs := placeholderPairs(modules[0])
	assertNoGeneratorNames(t, shell, pairs)
	assertNoGeneratorNames(t, bundle.Shell["cmd/module/main.go"], pairs)

	motor := bundle.StubSections["rdk:component:motor"]
	test.That(t, motor.Methods, test.ShouldContainSubstring, "GoFor")
	test.That(t, motor.ReturnType, test.ShouldContainSubstring, "motor.Motor")
	test.That(t, motor.EditablePath, test.ShouldEqual, "module.go")
	assertNoGeneratorNames(t, motor.Methods, pairs)
	assertNoGeneratorNames(t, motor.MainModels, pairs)

	generic := bundle.StubSections["rdk:service:generic"]
	test.That(t, generic.API, test.ShouldEqual, "rdk:service:generic")
	test.That(t, generic.ReturnType, test.ShouldEqual, "resource.Resource")
}

func TestGoShellIsSharedAcrossAPIs(t *testing.T) {
	root := rdkRoot(t)
	previous := gen.LocalSDKRoot
	gen.LocalSDKRoot = root
	t.Cleanup(func() { gen.LocalSDKRoot = previous })

	modules, err := inlineModules(golang)
	test.That(t, err, test.ShouldBeNil)
	var genericShell, motorShell string
	for _, module := range modules {
		if module.API != "rdk:service:generic" && module.API != "rdk:component:motor" {
			continue
		}
		rendered, renderErr := gen.RenderGoTemplatesWithSlots(module, false)
		test.That(t, renderErr, test.ShouldBeNil)
		shell, _, splitErr := splitGoModule(string(rendered))
		test.That(t, splitErr, test.ShouldBeNil)
		if module.API == "rdk:service:generic" {
			genericShell = shell
		} else {
			motorShell = shell
		}
	}
	test.That(t, genericShell, test.ShouldNotEqual, "")
	test.That(t, motorShell, test.ShouldEqual, genericShell)
}

func TestPlainGoRenderHasNoSlotMarkers(t *testing.T) {
	root := rdkRoot(t)
	previous := gen.LocalSDKRoot
	gen.LocalSDKRoot = root
	t.Cleanup(func() { gen.LocalSDKRoot = previous })

	modules, err := inlineModules(golang)
	test.That(t, err, test.ShouldBeNil)
	var generic modulegen.ModuleInputs
	for _, module := range modules {
		if module.API == "rdk:service:generic" {
			generic = module
			break
		}
	}
	rendered, err := gen.RenderGoTemplates(generic, false)
	test.That(t, err, test.ShouldBeNil)
	source := string(rendered)
	test.That(t, source, test.ShouldNotContainSubstring, "/*inline-slot:")
	test.That(t, source, test.ShouldContainSubstring, "resource.Resource")
	test.That(t, source, test.ShouldContainSubstring, "func init()")
	test.That(t, source, test.ShouldContainSubstring, "errUnimplemented")
}

func TestPythonSDKRequirementPinsTheVersion(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	script := filepath.Join("module_generate", "scripts", "generate_stubs.py")
	cmd := exec.Command("python3", "-c", `
import importlib.util, os, sys
spec = importlib.util.spec_from_file_location("generate_stubs", sys.argv[1])
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
os.environ["VIAM_PYTHON_SDK_VERSION"] = "0.80.0"
assert mod.sdk_requirement() == "viam-sdk==0.80.0", mod.sdk_requirement()
assert mod.sdk_requirement("mlmodel") == "viam-sdk[mlmodel]==0.80.0"
os.environ["VIAM_PYTHON_SDK_VERSION"] = ""
assert mod.sdk_requirement() == "viam-sdk"
targeted = mod.pip_install_command(["viam-sdk==0.80.0"], "/tmp/inline-deps")
assert "--target" in targeted and "/tmp/inline-deps" in targeted, targeted
assert "--isolated" in targeted
plain = mod.pip_install_command(["viam-sdk"])
assert "--target" not in plain, plain
`, script)
	out, err := cmd.CombinedOutput()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(out), test.ShouldEqual, "")
}

func rdkRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	test.That(t, err, test.ShouldBeNil)
	_, statErr := os.Stat(filepath.Join(root, "components", "motor", "client.go"))
	test.That(t, statErr, test.ShouldBeNil)
	return root
}

func assertNoGeneratorNames(t *testing.T, content string, pairs [][2]string) {
	t.Helper()
	stripped := regexp.MustCompile(`\$\{inline_[a-z0-9_]+\}`).ReplaceAllString(content, "")
	for _, pair := range pairs {
		test.That(t, stripped, test.ShouldNotContainSubstring, pair[0])
	}
}
