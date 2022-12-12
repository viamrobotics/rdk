package referenceframe

import (
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestModelLoading(t *testing.T) {
	m, err := ParseModelJSONFile(utils.ResolveFile("components/arm/trossen/trossen_wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Name(), test.ShouldEqual, "trossen-wx250s")
	simpleM, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, len(m.DoF()), test.ShouldEqual, 6)

	err = simpleM.validInputs(FloatsToInputs([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1}))
	test.That(t, err, test.ShouldBeNil)
	err = simpleM.validInputs(FloatsToInputs([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 99.1}))
	test.That(t, err, test.ShouldNotBeNil)

	orig := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1}
	orig[5] += math.Pi * 2
	orig[4] -= math.Pi * 4

	randpos := GenerateRandomConfiguration(m, rand.New(rand.NewSource(1)))
	test.That(t, simpleM.validInputs(FloatsToInputs(randpos)), test.ShouldBeNil)

	m, err = ParseModelJSONFile(utils.ResolveFile("components/arm/trossen/trossen_wx250s_kinematics.json"), "foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Name(), test.ShouldEqual, "foo")
}

//nolint:dupl
func TestTransform(t *testing.T) {
	m, err := ParseModelJSONFile(utils.ResolveFile("components/arm/trossen/trossen_wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	simpleM, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	joints := []Frame{}
	for _, tform := range simpleM.OrdTransforms {
		if len(tform.DoF()) > 0 {
			joints = append(joints, tform)
		}
	}
	test.That(t, len(joints), test.ShouldEqual, 6)
	pose, err := joints[0].Transform([]Input{{0}})
	test.That(t, err, test.ShouldBeNil)
	firstJov := pose.Orientation().OrientationVectorRadians()
	firstJovExpect := &spatial.OrientationVector{Theta: 0, OX: 0, OY: 0, OZ: 1}
	test.That(t, firstJov, test.ShouldResemble, firstJovExpect)

	pose, err = joints[0].Transform([]Input{{1.5708}})
	test.That(t, err, test.ShouldBeNil)
	firstJov = pose.Orientation().OrientationVectorRadians()
	firstJovExpect = &spatial.OrientationVector{Theta: 1.5708, OX: 0, OY: 0, OZ: 1}
	test.That(t, firstJov.Theta, test.ShouldAlmostEqual, firstJovExpect.Theta)
	test.That(t, firstJov.OX, test.ShouldAlmostEqual, firstJovExpect.OX)
	test.That(t, firstJov.OY, test.ShouldAlmostEqual, firstJovExpect.OY)
	test.That(t, firstJov.OZ, test.ShouldAlmostEqual, firstJovExpect.OZ)
}

func TestIncorrectInputs(t *testing.T) {
	m, err := ParseModelJSONFile(utils.ResolveFile("components/arm/trossen/trossen_wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	dof := len(m.DoF())

	// test incorrect number of inputs
	pose, err := m.Transform(make([]Input, dof+1))
	test.That(t, pose, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, NewIncorrectInputLengthError(dof+1, dof).Error())

	// test incorrect number of inputs to Geometries
	gf, err := m.Geometries(make([]Input, dof-1))
	test.That(t, gf, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, NewIncorrectInputLengthError(dof-1, dof).Error())
}

func TestModelGeometries(t *testing.T) {
	// build a test model
	offset := spatial.NewPoseFromPoint(r3.Vector{0, 0, 10})
	bc, err := spatial.NewBoxCreator(r3.Vector{1, 1, 1}, offset, "")
	test.That(t, err, test.ShouldBeNil)
	// m, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/model_test.json"), "")
	frame1, err := NewStaticFrameWithGeometry("link1", offset, bc)
	test.That(t, err, test.ShouldBeNil)
	frame2, err := NewRotationalFrame("", spatial.R4AA{RY: 1}, Limit{Min: -360, Max: 360})
	test.That(t, err, test.ShouldBeNil)
	frame3, err := NewStaticFrameWithGeometry("link2", offset, bc)
	test.That(t, err, test.ShouldBeNil)
	m := &SimpleModel{baseFrame: &baseFrame{name: "test"}, OrdTransforms: []Frame{frame1, frame2, frame3}}

	// test zero pose of model
	inputs := make([]Input, len(m.DoF()))
	geometries, _ := m.Geometries(inputs)
	test.That(t, geometries, test.ShouldNotBeNil)
	link1 := geometries.Geometries()["test:link1"].Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(link1, r3.Vector{0, 0, 10}, 1e-8), test.ShouldBeTrue)
	link2 := geometries.Geometries()["test:link2"].Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(link2, r3.Vector{0, 0, 20}, 1e-8), test.ShouldBeTrue)

	// transform the model 90 degrees at the joint
	inputs[0] = Input{math.Pi / 2}
	geometries, _ = m.Geometries(inputs)
	test.That(t, geometries, test.ShouldNotBeNil)
	link1 = geometries.Geometries()["test:link1"].Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(link1, r3.Vector{0, 0, 10}, 1e-8), test.ShouldBeTrue)
	link2 = geometries.Geometries()["test:link2"].Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(link2, r3.Vector{10, 0, 10}, 1e-8), test.ShouldBeTrue)
}
