package javascript_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	functionvm "go.viam.com/core/function/vm"
	_ "go.viam.com/core/function/vm/engines/javascript"
)

func TestEngineImport(t *testing.T) {
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

	results, err := engine.ExecuteSource(`importFunc1("omg", "it", "works")`)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, results, test.ShouldHaveLength, 1)
	test.That(t, results[0].MustString(), test.ShouldEqual, "cool => omg it works")
}

func TestEngineError(t *testing.T) {
	engine, err := functionvm.NewEngine(functionvm.EngineNameJavaScript)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, engine, test.ShouldNotBeNil)

	engine.ImportFunction("importFunc1", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		return nil, errors.New("whoops")
	})

	_, err = engine.ExecuteSource(`importFunc1()`)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
}

func TestEngineValidateSource(t *testing.T) {
	test.That(t, functionvm.ValidateSource("javascript", "1+1"), test.ShouldBeNil)
	err := functionvm.ValidateSource("javascript", "1+")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected token")
}

func TestEngineOutput(t *testing.T) {
	engine, err := functionvm.NewEngine(functionvm.EngineNameJavaScript)
	test.That(t, err, test.ShouldBeNil)

	_, err = engine.ExecuteSource(`
console.log("hello");
// from https://developers.google.com/web/updates/2012/06/How-to-convert-ArrayBuffer-to-and-from-String
function str2ab(str) {
  var buf = new ArrayBuffer(str.length);
  var bufView = new Uint8Array(buf);
  for (var i=0, strLen=str.length; i < strLen; i++) {
    bufView[i] = str.charCodeAt(i);
  }
  return buf;
}
(async function() {
	const std = await import('std');
	"wrote", std.err.write(str2ab("world"), 0, 5);
})()
1
`)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, engine.StandardOutput(), test.ShouldEqual, "hello\n")
	test.That(t, engine.StandardError(), test.ShouldEqual, "world")
}
