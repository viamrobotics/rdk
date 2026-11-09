package scripts

import (
	"fmt"
	"go/ast"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/cli/module_generate/modulegen"
)

func TestGenerateStubs(t *testing.T) {
	t.Run("test type formatting", func(t *testing.T) {
		subtype := "resource"
		testType := "test"
		testTypeUpper := "Test"

		test.That(t, formatType(ast.NewIdent(testType), subtype), test.ShouldEqual, testType)

		paramType := formatType(ast.NewIdent(testTypeUpper), subtype)
		test.That(t, paramType, test.ShouldEqual, fmt.Sprintf("%s.%s", subtype, testTypeUpper))
		for _, prefix := range typePrefixes {
			paramType = formatType(ast.NewIdent(prefix+testType), subtype)
			test.That(t, paramType, test.ShouldEqual, fmt.Sprintf("%s%s", prefix, testType))
			paramType := formatType(ast.NewIdent(prefix+testTypeUpper), subtype)
			test.That(t, paramType, test.ShouldEqual, fmt.Sprintf("%s%s.%s", prefix, subtype, testTypeUpper))
		}
	})
}

func TestPythonStubImportsSameFileTypes(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	_, filename, _, ok := runtime.Caller(0)
	test.That(t, ok, test.ShouldBeTrue)
	script := filepath.Join(filepath.Dir(filename), "generate_stubs.py")
	out, err := exec.Command("python3", script, "--self-test").CombinedOutput()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(out), test.ShouldBeEmpty)
}

func TestRenderGoTemplatesWithSlotsIgnoresCRLFTemplate(t *testing.T) {
	root, err := filepath.Abs("../../..")
	test.That(t, err, test.ShouldBeNil)
	previousRoot := LocalSDKRoot
	previousTmpl := goTmpl
	LocalSDKRoot = root
	goTmpl = strings.ReplaceAll(strings.ReplaceAll(goTmpl, "\r\n", "\n"), "\n", "\r\n")
	t.Cleanup(func() {
		LocalSDKRoot = previousRoot
		goTmpl = previousTmpl
	})

	module := modulegen.ModuleInputs{
		ModuleName:      "inline-module",
		Language:        "go",
		Namespace:       "org",
		ModelName:       "generic-service",
		ResourceSubtype: "generic_service",
		ResourceType:    "service",
	}
	test.That(t, module.CheckResourceAndSetType(), test.ShouldBeNil)
	rendered, err := RenderGoTemplatesWithSlots(module, false)
	test.That(t, err, test.ShouldBeNil)
	source := string(rendered)
	test.That(t, strings.Contains(source, "\r"), test.ShouldBeFalse)
	test.That(t, strings.Count(source, "// inline-example:imports\n"), test.ShouldEqual, 1)
}
