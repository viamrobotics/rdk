package scripts

import (
	"go/ast"
	"go/parser"
	"go/token"
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
		for _, tc := range []struct {
			in, subtype, want string
		}{
			{"test", "resource", "test"},
			{"Test", "resource", "resource.Test"},
			{"*Test", "resource", "*resource.Test"},
			{"[]*Test", "resource", "[]*resource.Test"},
			{"[]test", "resource", "[]test"},
			{"map[string]Test", "resource", "map[string]resource.Test"},
			{"chan *Chunk", "audioin", "chan *audioin.Chunk"},
			{"<-chan []Point", "arm", "<-chan []arm.Point"},
			{"chan<- Response", "motion", "chan<- motion.Response"},
			{"func(Name string) error", "resource", "func(Name string) error"},
			{"referenceframe.Input", "arm", "referenceframe.Input"},
			{"[]referenceframe.Input", "arm", "[]referenceframe.Input"},
			{"State", "switch", "sw.State"},
		} {
			expr, err := parser.ParseExpr(tc.in)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, formatType(expr, tc.subtype), test.ShouldEqual, tc.want)
		}
	})

	t.Run("channel return types have nil zero values", func(t *testing.T) {
		for _, typ := range []string{"chan *audioin.Chunk", "<-chan int", "chan<- int"} {
			val, needsVar := zeroValueForType(typ, 1)
			test.That(t, val, test.ShouldEqual, "nil")
			test.That(t, needsVar, test.ShouldBeFalse)
		}
	})

	t.Run("helper constructors build the helper around the model", func(t *testing.T) {
		src := `package board

type client struct{ boardName string }

func (c *client) AnalogByName(name string) (Analog, error) {
	return &analogClient{
		client:     c,
		boardName:  c.boardName,
		analogName: name,
	}, nil
}

func (c *client) SetPowerMode() error { return nil }

type analogClient struct {
	*client
	boardName  string
	analogName string
}
`
		node, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
		test.That(t, err, test.ShouldBeNil)
		helpers, fields := findHelperConstructors(node)
		test.That(t, len(helpers), test.ShouldEqual, 1)
		test.That(t, helpers["AnalogByName"].body("myModel"), test.ShouldEqual,
			"\treturn &analogClient{myModel: s, analogName: name}, nil")

		var spec *ast.TypeSpec
		ast.Inspect(node, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "analogClient" {
				spec = ts
			}
			return true
		})
		test.That(t, formatStruct(spec, "myModel", fields), test.ShouldEqual,
			"type analogClient struct {\n\t*myModel\n\tanalogName\tstring\n}\n\n")
	})
}

func TestPythonStubImportsSameFileTypes(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	_, filename, _, ok := runtime.Caller(0)
	test.That(t, ok, test.ShouldBeTrue)
	script := filepath.Join(filepath.Dir(filename), "generate_stubs.py")
	out, err := exec.CommandContext(t.Context(), "python3", script, "--self-test").CombinedOutput()
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
