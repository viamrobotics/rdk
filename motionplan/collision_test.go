package motionplan

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestCollisionsEqual(t *testing.T) {
	expected := Collision{name1: "a", name2: "b", penetrationDepth: 1}
	cases := []struct {
		collision Collision
		success   bool
	}{
		{Collision{name1: "a", name2: "b", penetrationDepth: 1}, true},
		{Collision{name1: "b", name2: "a", penetrationDepth: 1}, true},
		{Collision{name1: "a", name2: "c", penetrationDepth: 1}, false},
		{Collision{name1: "b", name2: "a", penetrationDepth: 2}, false},
	}
	for _, c := range cases {
		test.That(t, c.success == collisionsAlmostEqual(expected, c.collision), test.ShouldBeTrue)
	}
}

func TestCollisionListsEqual(t *testing.T) {
	c1a := Collision{name1: "a", name2: "b", penetrationDepth: 1}
	c1b := Collision{name1: "a", name2: "b", penetrationDepth: 1}
	c2a := Collision{name1: "c", name2: "d", penetrationDepth: 2}
	c2b := Collision{name1: "d", name2: "c", penetrationDepth: 2}
	c3a := Collision{name1: "e", name2: "f", penetrationDepth: 2}
	c3b := Collision{name1: "f", name2: "e", penetrationDepth: 2}
	list1 := []Collision{c1a, c2b, c3a}
	list2 := []Collision{c3b, c1a, c2a}
	list3 := []Collision{c3b, c1a, c1b}
	test.That(t, collisionListsAlmostEqual(list1, list2), test.ShouldBeTrue)
	test.That(t, collisionListsAlmostEqual(list1, list3), test.ShouldBeFalse)
	test.That(t, collisionListsAlmostEqual(list1, []Collision{}), test.ShouldBeFalse)
}

func TestCheckCollisions(t *testing.T) {
	// case 1: small collection of custom geometries, expecting:
	//      - a collision between two internal geometries
	//      - a collision between an internal and external geometry
	//      - no collision between two external geometries
	bc1, err := spatial.NewBoxCreator(r3.Vector{2, 2, 2}, spatial.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	robot := make(map[string]spatial.Geometry)
	robot["robotCube000"] = bc1.NewGeometry(spatial.NewZeroPose())
	robot["robotCube222"] = bc1.NewGeometry(spatial.NewPoseFromPoint(r3.Vector{3, 3, 3}))
	robot["robotCube333"] = bc1.NewGeometry(spatial.NewPoseFromPoint(r3.Vector{4, 4, 4}))
	obstacles := make(map[string]spatial.Geometry)
	obstacles["obstacleCube000"] = bc1.NewGeometry(spatial.NewZeroPose())
	obstacles["obstacleCube888"] = bc1.NewGeometry(spatial.NewPoseFromPoint(r3.Vector{8, 8, 8}))
	obstacles["obstacleCube999"] = bc1.NewGeometry(spatial.NewPoseFromPoint(r3.Vector{9, 9, 9}))
	interactionSpaces := make(map[string]spatial.Geometry)
	interactionSpaces["iSCube000"] = bc1.NewGeometry(spatial.NewZeroPose())
	interactionSpaces["iSCube222"] = bc1.NewGeometry(spatial.NewPoseFromPoint(r3.Vector{3, 3, 3}))
	robotEntities, err := NewObjectCollisionEntities(robot)
	test.That(t, err, test.ShouldBeNil)
	obstacleEntities, err := NewObjectCollisionEntities(obstacles)
	test.That(t, err, test.ShouldBeNil)
	spaceEntities, err := NewSpaceCollisionEntities(interactionSpaces)
	test.That(t, err, test.ShouldBeNil)
	cs, err := NewCollisionSystem(robotEntities, []CollisionEntities{obstacleEntities, spaceEntities})
	test.That(t, err, test.ShouldBeNil)
	expectedCollisions := []Collision{
		{"robotCube222", "robotCube333", 1},
		{"obstacleCube000", "robotCube000", 2},
		{"iSCube000", "robotCube333", 1},
		{"iSCube222", "robotCube333", 1},
	}
	test.That(t, collisionListsAlmostEqual(cs.Collisions(), expectedCollisions), test.ShouldBeTrue)

	// case 2: zero position of xArm6 arm - should have number of collisions = to number of geometries - 1
	// no external geometries considered, self collision only
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	robot, _ = m.Geometries(make([]frame.Input, len(m.DoF())))
	test.That(t, robot, test.ShouldNotBeNil)
	robotEntities, err = NewObjectCollisionEntities(robot)
	test.That(t, robot, test.ShouldNotBeNil)
	cs, err = NewCollisionSystem(robotEntities, []CollisionEntities{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cs.Collisions()), test.ShouldEqual, 4)
}

func TestUniqueCollisions(t *testing.T) {
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// zero position of ur5e arm
	input := make([]frame.Input, len(m.DoF()))
	internalGeometries, _ := m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	internalEntities, err := NewObjectCollisionEntities(internalGeometries)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	zeroPositionCG, err := NewCollisionSystem(internalEntities, []CollisionEntities{})
	test.That(t, err, test.ShouldBeNil)

	// case 1: no self collision - check no new collisions are returned
	input[0] = frame.Input{0}
	internalGeometries, _ = m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	internalEntities, err = NewObjectCollisionEntities(internalGeometries)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	cs, err := NewCollisionSystemFromReference(internalEntities, []CollisionEntities{}, zeroPositionCG)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cs.Collisions()), test.ShouldEqual, 0)

	// case 2: self collision - check only new collisions are returned
	input[4] = frame.Input{2}
	internalGeometries, _ = m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	internalEntities, err = NewObjectCollisionEntities(internalGeometries)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	cs, err = NewCollisionSystemFromReference(internalEntities, []CollisionEntities{}, zeroPositionCG)
	test.That(t, err, test.ShouldBeNil)
	expectedCollisions := []Collision{{"xArm6:base_top", "xArm6:wrist_link", 041.61}, {"xArm6:wrist_link", "xArm6:upper_arm", 48.18}}
	test.That(t, collisionListsAlmostEqual(cs.Collisions(), expectedCollisions), test.ShouldBeTrue)
}
