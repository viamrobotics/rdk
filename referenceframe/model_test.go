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
	m, err := ParseModelJSONFile(utils.ResolveFile("component/arm/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Name(), test.ShouldEqual, "wx250s")
	simpleM, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, simpleM.OperationalDoF(), test.ShouldEqual, 1)
	test.That(t, len(m.DoF()), test.ShouldEqual, 6)

	isValid := IsConfigurationValid(simpleM, []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1})
	test.That(t, isValid, test.ShouldBeTrue)
	isValid = IsConfigurationValid(simpleM, []float64{0.1, 0.1, 0.1, 0.1, 0.1, 99.1})
	test.That(t, isValid, test.ShouldBeFalse)

	orig := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1}
	orig[5] += math.Pi * 2
	orig[4] -= math.Pi * 4

	randpos := GenerateRandomConfiguration(m, rand.New(rand.NewSource(1)))
	test.That(t, IsConfigurationValid(simpleM, randpos), test.ShouldBeTrue)

	m, err = ParseModelJSONFile(utils.ResolveFile("component/arm/wx250s/wx250s_kinematics.json"), "foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Name(), test.ShouldEqual, "foo")
}

func TestTransform(t *testing.T) {
	m, err := ParseModelJSONFile(utils.ResolveFile("component/arm/wx250s/wx250s_kinematics.json"), "")
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

func TestModelGeometries(t *testing.T) {
	m, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/model_test.json"), "")
	test.That(t, err, test.ShouldBeNil)
	sm, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	inputs := make([]Input, len(sm.DoF()))
	geometries, _ := sm.Geometries(inputs)

	test.That(t, geometries, test.ShouldNotBeNil)
	expected, err := sm.inputsToFrames(inputs, true)
	test.That(t, err, test.ShouldBeNil)
	geoms, err := m.Geometries(inputs)
	_ = geoms

	inputs[0] = Input{math.Pi / 2}
	expected, err = sm.inputsToFrames(inputs, true)
	test.That(t, err, test.ShouldBeNil)
	pose := expected[3].transform.Point()
	geoms, err = m.Geometries(inputs)
	_ = geoms
	pose = geoms.Geometries()["test:link2"].Pose().Point()
	_ = pose
	numGeometries := 0
	for _, joint := range expected {
		if joint.geometryCreator != nil {
			numGeometries++
			var offset r3.Vector
			for _, tf := range m.(*SimpleModel).OrdTransforms {
				if tf.Name() == joint.Name() {
					geometry, err := tf.Geometries([]Input{})
					test.That(t, err, test.ShouldBeNil)
					offset = geometry.Geometries()[tf.Name()].Pose().Point().Sub(tf.(*staticFrame).transform.Point())
				}
			}
			expectedGeometry := joint.transform.Point().Add(offset)
			geometryCenter := geometries.Geometries()[sm.Name()+":"+joint.Name()].Pose().Point()
			test.That(t, spatial.R3VectorAlmostEqual(expectedGeometry, geometryCenter, 1e-3), test.ShouldBeTrue)
		}
	}
	test.That(t, numGeometries, test.ShouldEqual, 5)
}
