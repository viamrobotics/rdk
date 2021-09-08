package javascript_test

import (
	"fmt"
	"strings"
	"testing"

	"go.viam.com/test"

	functionvm "go.viam.com/core/function/vm"
	_ "go.viam.com/core/function/vm/engines/javascript"
)

func TestEngine(t *testing.T) {
	engine, err := functionvm.NewEngine(functionvm.EngineNameJavaScript)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, engine, test.ShouldNotBeNil)

	engine.ImportFunction("importFunc1", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		var strs []string
		for _, arg := range args {
			strs = append(strs, arg.Stringer())
		}
		return []functionvm.Value{functionvm.NewString(fmt.Sprintf("cool => %s", strings.Join(strs, " ")))}, nil
	})

	results, err := engine.ExecuteCode(`libFunc1("omg", "it", "works")`)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, results, test.ShouldHaveLength, 1)
	test.That(t, results[0].MustString(), test.ShouldEqual, "done => omg it works")

	results, err = engine.ExecuteCode(`importFunc1("omg", "it", "works")`)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, results, test.ShouldHaveLength, 1)
	test.That(t, results[0].MustString(), test.ShouldEqual, "cool => omg it works")
}
