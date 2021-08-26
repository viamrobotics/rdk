package referenceframe

import (
	"go.viam.com/test"
	"testing"
)

func TestFrameSystemFromJSON(t *testing.T) {
	jsonPath := "example_frame_system.json"
	fs, err := NewFrameSystemFromJSON(jsonPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.Name(), test.ShouldEqual, "xArm6")
	test.That(t, len(fs.Frames()), test.ShouldEqual, 12)
}
