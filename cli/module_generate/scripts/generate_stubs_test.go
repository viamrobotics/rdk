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
