package javascript_test

import (
	"testing"

	"go.viam.com/test"

	functionvm "go.viam.com/core/function/vm"
	_ "go.viam.com/core/function/vm/engines/javascript"
)

func TestEngine(t *testing.T) {
	engine, err := functionvm.NewEngine(functionvm.EngineNameJavaScript)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, engine, test.ShouldNotBeNil)

	results, err := engine.ExecuteCode(`console.log(libFunc1("omg")); 1+1+""+" world"`)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, results, test.ShouldHaveLength, 1)
	test.That(t, results[0].MustString(), test.ShouldEqual, "2 world")
}
