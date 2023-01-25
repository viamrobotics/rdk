package transform

import (
	"testing"

	"go.viam.com/test"
)

func TestBrownConradyCheckValid(t *testing.T) {
	t.Run("nil &BrownConrady{} are invalid", func(t *testing.T) {
		var nilBrownConradyPtr *BrownConrady
		err := nilBrownConradyPtr.CheckValid()
		expected := "BrownConrady shaped distortion_parameters not provided: invalid distortion_parameters"
		test.That(t, err.Error(), test.ShouldContainSubstring, expected)
	})

	t.Run("non nil &BrownConrady{} are valid", func(t *testing.T) {
		distortionsA := &BrownConrady{}
		test.That(t, distortionsA.CheckValid(), test.ShouldBeNil)
	})
}
