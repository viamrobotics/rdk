package inference

import (
	"testing"

	"go.viam.com/test"
)

func TestIsValidType(t *testing.T) {
	isValid := isValidType("hello")
	test.That(t, isValid, test.ShouldBeFalse)
	isValid = isValidType("tflite")
	test.That(t, isValid, test.ShouldBeTrue)
}
