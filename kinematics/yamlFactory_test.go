package kinematics

import (
	//~ "fmt"
	"testing"

	"github.com/edaniels/test"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/viamrobotics/robotcore/testutils"
)

// Tests orientation setting
func TestSetOrient(t *testing.T) {
	trans := NewTransform()
	o1 := Orientation{ThreeD{0, 0, 0}, ThreeD{0, 0, 0}}
	o2 := Orientation{ThreeD{45, 45, 70}, ThreeD{0, 0, 0}}
	o3 := Orientation{ThreeD{0, 0, 0}, ThreeD{1, 2, 3}}
	o4 := Orientation{ThreeD{60, -30, 40}, ThreeD{4, 5, 6}}

	m2 := mgl64.Mat3FromRows(mgl64.Vec3{0.241845, -0.493453, 0.835473},
		mgl64.Vec3{0.664463, 0.711691, 0.228002},
		mgl64.Vec3{-0.707107, 0.500000, 0.500000})

	m4 := mgl64.Mat3FromRows(mgl64.Vec3{0.663414, -0.653101, 0.365159},
		mgl64.Vec3{0.556670, 0.104687, -0.824111},
		mgl64.Vec3{0.500000, 0.750000, 0.433013})

	setOrient(o1, trans)
	if !trans.t.Mat.ApproxEqual(mgl64.Ident4()) {
		t.Fatalf("Zero Orientation not producing identity matrix")
	}

	trans = NewTransform()
	setOrient(o2, trans)
	if !trans.t.Mat.Col(3).ApproxEqual(mgl64.Vec4{0, 0, 0, 1}) {
		t.Fatalf("o2 translation incorrect")
	}
	if !trans.t.Mat.Mat3().ApproxEqualThreshold(m2, 0.000001) {
		t.Fatalf("o2 rotation incorrect")
	}

	trans = NewTransform()
	setOrient(o3, trans)
	if !trans.t.Mat.Col(3).ApproxEqual(mgl64.Vec4{1, 2, 3, 1}) {
		t.Fatalf("o3 translation incorrect")
	}
	if !trans.t.Mat.Mat3().ApproxEqual(mgl64.Ident3()) {
		t.Fatalf("o3 rotation incorrect")
	}

	trans = NewTransform()
	setOrient(o4, trans)
	if !trans.t.Mat.Col(3).ApproxEqual(mgl64.Vec4{4, 5, 6, 1}) {
		t.Fatalf("o4 translation incorrect")
	}
	if !trans.t.Mat.Mat3().ApproxEqualThreshold(m4, 0.000001) {
		t.Fatalf("o4 rotation incorrect")
	}

}

// Tests that yml files are properly parsed and correctly loaded into the model
// Should not need to actually test the contained rotation/translation values
// since that will be caught by tests to the actual kinematics
// So we'll just check that we read in the right number of joints
func TestParseYmlFile(t *testing.T) {
	model, err := ParseYmlFile(testutils.ResolveFile("kinematics/models/mdl/wx250s_test.yml"))
	test.That(t, err, test.ShouldBeNil)

	if len(model.Joints) != 6 {
		t.Fatalf("Incorrect number of joints loaded")
	}
}
