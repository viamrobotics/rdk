package scripts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"go.viam.com/test"
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
