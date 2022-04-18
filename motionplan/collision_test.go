package motionplan

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestCheckCollisions(t *testing.T) {
	// case 1: small collection of custom geometries, expecting:
	//      - a collision between two internal geometries
	//      - a collision between an internal and external geometry
	//      - no collision between two external geometries
	bc, err := spatial.NewBoxCreator(r3.Vector{2, 2, 2}, spatial.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	internalGeometries := make(map[string]spatial.Geometry)
	internalGeometries["internalCube000"] = bc.NewGeometry(spatial.NewZeroPose())
	internalGeometries["internalCube222"] = bc.NewGeometry(spatial.NewPoseFromPoint(r3.Vector{3, 3, 3}))
	internalGeometries["internalCube333"] = bc.NewGeometry(spatial.NewPoseFromPoint(r3.Vector{4, 4, 4}))
	externalGeometries := make(map[string]spatial.Geometry)
	externalGeometries["externalCube000"] = bc.NewGeometry(spatial.NewZeroPose())
	externalGeometries["externalCube888"] = bc.NewGeometry(spatial.NewPoseFromPoint(r3.Vector{8, 8, 8}))
	externalGeometries["externalCube999"] = bc.NewGeometry(spatial.NewPoseFromPoint(r3.Vector{9, 9, 9}))
	cg, err := CheckCollisions(internalGeometries, externalGeometries)
	test.That(t, err, test.ShouldBeNil)
	collisions := cg.Collisions()
	test.That(t, len(collisions), test.ShouldEqual, 2)
	expectedCollisions := [2]Collision{{"internalCube222", "internalCube333", 1}, {"externalCube000", "internalCube000", 2}}
	test.That(t, collisionsEqual(collisions, expectedCollisions), test.ShouldBeTrue)

	// case 2: zero position of xArm6 arm - should have number of collisions = to number of geometries - 1
	// no external geometries considered, self collision only
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	internalGeometries, _ = m.Geometries(make([]frame.Input, len(m.DoF())))
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	cg, err = CheckCollisions(internalGeometries, map[string]spatial.Geometry{})
	test.That(t, err, test.ShouldBeNil)
	cols := cg.Collisions()
	test.That(t, len(cols), test.ShouldEqual, len(cg.indices)-1)
}

func TestUniqueCollisions(t *testing.T) {
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// zero position of ur5e arm
	input := make([]frame.Input, len(m.DoF()))
	internalGeometries, _ := m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	zeroPositionCG, err := CheckCollisions(internalGeometries, map[string]spatial.Geometry{})
	test.That(t, err, test.ShouldBeNil)

	// case 1: no self collision - check no new collisions are returned
	input[0] = frame.Input{Value: 1, Units: frame.Radians}
	internalGeometries, _ = m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	cg, err := CheckUniqueCollisions(internalGeometries, map[string]spatial.Geometry{}, zeroPositionCG)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cg.Collisions()), test.ShouldEqual, 0)

	// case 2: self collision - check only new collisions are returned
	input[4] = frame.Input{Value: 2, Units: frame.Radians}
	internalGeometries, _ = m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	cg, err = CheckUniqueCollisions(internalGeometries, map[string]spatial.Geometry{}, zeroPositionCG)
	test.That(t, err, test.ShouldBeNil)
	cols := cg.Collisions()
	test.That(t, len(cols), test.ShouldEqual, 2)
	equal := collisionsEqual(cols, [2]Collision{{"xArm6:base_top", "xArm6:wrist_link", 0}, {"xArm6:wrist_link", "xArm6:upper_arm", 0}})
	test.That(t, equal, test.ShouldBeTrue)
}

// collisionsEqual is a helper function to compare two Collision lists because they can be out of order due to random nature of maps.
func collisionsEqual(c1 []Collision, c2 [2]Collision) bool {
	return (collisionEqual(c1[0], c2[0]) && collisionEqual(c1[1], c2[1])) || (collisionEqual(c1[0], c2[1]) && collisionEqual(c1[1], c2[0]))
}

// collisionEqual is a helper function to compare two Collisions because their strings can be out of order due to random nature of maps.
func collisionEqual(c1, c2 Collision) bool {
	return ((c1.name1 == c2.name1 && c1.name2 == c2.name2) || (c1.name1 == c2.name2 && c1.name2 == c2.name1)) &&
		utils.Float64AlmostEqual(c1.penetrationDepth, c2.penetrationDepth, 0.1)
}
