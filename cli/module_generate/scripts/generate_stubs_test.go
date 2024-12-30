package scripts

import (
	"fmt"
	"go/ast"
	"testing"

	"go.viam.com/test"
)

func TestGenerateStubs(t *testing.T) {
	t.Run("test type formatting", func(t *testing.T) {
		subtype := "resource"
		testType := "Test"

		paramType := formatType(ast.NewIdent(testType), subtype)
		test.That(t, paramType, test.ShouldEqual, fmt.Sprintf("%s.%s", subtype, testType))
		for _, prefix := range typePrefixes {
			paramType := formatType(ast.NewIdent(prefix + testType), subtype)
			test.That(t, paramType, test.ShouldEqual, fmt.Sprintf("%s%s.%s", prefix, subtype, testType))
		}
	})
}
