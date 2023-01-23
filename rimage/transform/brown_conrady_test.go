package transform

import (
	"testing"

	"go.viam.com/test"
)

func TestBrownConradyCheckValid(t *testing.T) {
	distortionsA := &BrownConrady{}
	test.That(t, distortionsA.CheckValid(), test.ShouldBeNil)
	var nilBrownConradyPtr *BrownConrady
	err := nilBrownConradyPtr.CheckValid()
	expected := "BrownConrady shaped distortion_parameters not provided: invalid distortion_parameters"
	test.That(t, err.Error(), test.ShouldContainSubstring, expected)
}
