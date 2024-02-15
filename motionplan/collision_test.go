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
	//      - collisions reported between robot and obstacles
	//      - no collision between two obstacle geometries or robot geometries
	bc1, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)
	robot := []spatial.Geometry{}
	robot = append(robot, bc1.Transform(spatial.NewZeroPose()))
	robot[0].SetLabel("robotCube000")
	robot = append(robot, bc1.Transform(spatial.NewPoseFromPoint(r3.Vector{3, 3, 3})))
	robot[1].SetLabel("robotCube333")
	robot = append(robot, bc1.Transform(spatial.NewPoseFromPoint(r3.Vector{9, 9, 9})))
	robot[2].SetLabel("robotCube999")

	obstacles := []spatial.Geometry{}
	obstacles = append(obstacles, bc1.Transform(spatial.NewZeroPose()))
	obstacles[0].SetLabel("obstacleCube000")
	obstacles = append(obstacles, bc1.Transform(spatial.NewPoseFromPoint(r3.Vector{4, 4, 4})))
	obstacles[1].SetLabel("obstacleCube444")
	obstacles = append(obstacles, bc1.Transform(spatial.NewPoseFromPoint(r3.Vector{6, 6, 6})))
	obstacles[2].SetLabel("obstacleCube666")
	cg, err := newCollisionGraph(robot, obstacles, nil, true, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	expectedCollisions := []Collision{
		{"robotCube333", "obstacleCube444", -1},
		{"robotCube000", "obstacleCube000", -2},
	}
	test.That(t, collisionListsAlmostEqual(cg.collisions(defaultCollisionBufferMM), expectedCollisions), test.ShouldBeTrue)

	// case 2: zero position of xArm6 arm - should have number of collisions = to number of geometries - 1
	// no external geometries considered, self collision only
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	gf, _ := m.Geometries(make([]frame.Input, len(m.DoF())))
	test.That(t, gf, test.ShouldNotBeNil)
	cg, err = newCollisionGraph(gf.Geometries(), gf.Geometries(), nil, true, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cg.collisions(defaultCollisionBufferMM)), test.ShouldEqual, 4)
}

func TestUniqueCollisions(t *testing.T) {
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// zero position of xarm6 arm
	input := make([]frame.Input, len(m.DoF()))
	internalGeometries, _ := m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	zeroPositionCG, err := newCollisionGraph(
		internalGeometries.Geometries(),
		internalGeometries.Geometries(),
		nil,
		true,
		defaultCollisionBufferMM,
	)
	test.That(t, err, test.ShouldBeNil)

	// case 1: no self collision - check no new collisions are returned
	input[0] = frame.Input{Value: 1}
	internalGeometries, _ = m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	cg, err := newCollisionGraph(
		internalGeometries.Geometries(),
		internalGeometries.Geometries(),
		zeroPositionCG,
		true,
		defaultCollisionBufferMM,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cg.collisions(defaultCollisionBufferMM)), test.ShouldEqual, 0)

	// case 2: self collision - check only new collisions are returned
	input[4] = frame.Input{Value: 2}
	internalGeometries, _ = m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)
	cg, err = newCollisionGraph(
		internalGeometries.Geometries(),
		internalGeometries.Geometries(),
		zeroPositionCG,
		true,
		defaultCollisionBufferMM,
	)
	test.That(t, err, test.ShouldBeNil)
	expectedCollisions := []Collision{{"xArm6:base_top", "xArm6:wrist_link", -66.6}, {"xArm6:wrist_link", "xArm6:upper_arm", -48.1}}
	test.That(t, collisionListsAlmostEqual(cg.collisions(defaultCollisionBufferMM), expectedCollisions), test.ShouldBeTrue)

	// case 3: add a collision specification that the last element of expectedCollisions should be ignored
	zeroPositionCG.addCollisionSpecification(&expectedCollisions[1])

	cg, err = newCollisionGraph(
		internalGeometries.Geometries(),
		internalGeometries.Geometries(),
		zeroPositionCG,
		true,
		defaultCollisionBufferMM,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collisionListsAlmostEqual(cg.collisions(defaultCollisionBufferMM), expectedCollisions[:1]), test.ShouldBeTrue)
}
