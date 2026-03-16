package motionplan

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// collisionListsAlmostEqual compares two lists of Collisions and returns if they are almost equal.
func collisionListsAlmostEqual(cs1, cs2 []Collision) bool {
	if len(cs1) != len(cs2) {
		return false
	}

	// loop through list 1 and match with elements in list 2, mark on list of used indexes
	used := make([]bool, len(cs1))
	for _, c1 := range cs1 {
		for i, c2 := range cs2 {
			if collisionsEqual(c1, c2) {
				used[i] = true
				break
			}
		}
	}

	// loop through list of used indexes
	for _, c := range used {
		if !c {
			return false
		}
	}
	return true
}

func TestCollisionsEqual(t *testing.T) {
	expected := Collision{name1: "a", name2: "b"}
	cases := []struct {
		collision Collision
		success   bool
	}{
		{Collision{name1: "a", name2: "b"}, true},
		{Collision{name1: "b", name2: "a"}, true},
		{Collision{name1: "a", name2: "c"}, false},
		{Collision{name1: "c", name2: "a"}, false},
	}
	for _, c := range cases {
		test.That(t, c.success == collisionsEqual(expected, c.collision), test.ShouldBeTrue)
	}
}

func TestCollisionListsEqual(t *testing.T) {
	c1a := Collision{name1: "a", name2: "b"}
	c1b := Collision{name1: "a", name2: "b"}
	c2a := Collision{name1: "c", name2: "d"}
	c2b := Collision{name1: "d", name2: "c"}
	c3a := Collision{name1: "e", name2: "f"}
	c3b := Collision{name1: "f", name2: "e"}
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

	collisions, _, err := CheckCollisions(robot, obstacles, nil, defaultCollisionBufferMM, true)
	test.That(t, err, test.ShouldBeNil)
	expectedCollisions := []Collision{
		{"robotCube333", "obstacleCube444"},
		{"robotCube000", "obstacleCube000"},
	}
	test.That(t, collisionListsAlmostEqual(collisions, expectedCollisions), test.ShouldBeTrue)

	// case 2: zero position of xArm6 arm - should have number of collisions = to number of geometries - 1
	// no external geometries considered, self collision only
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	gf, _ := m.Geometries(make([]referenceframe.Input, len(m.DoF())))
	test.That(t, gf, test.ShouldNotBeNil)

	selfGeoms := gf.Geometries()
	collisions, _, err = CheckCollisions(selfGeoms, selfGeoms, nil, defaultCollisionBufferMM, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(collisions), test.ShouldEqual, 5)
}

func TestUniqueCollisions(t *testing.T) {
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// zero position of xarm6 arm - get initial collisions to use as allowed collisions
	input := make([]referenceframe.Input, len(m.DoF()))
	internalGeometries, _ := m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)

	zeroGeoms := internalGeometries.Geometries()
	zeroPositionCollisions, _, err := CheckCollisions(zeroGeoms, zeroGeoms, nil, defaultCollisionBufferMM, true)
	test.That(t, err, test.ShouldBeNil)

	// case 1: no self collision - check no new collisions are returned
	input[0] = 1
	internalGeometries, _ = m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)

	geoms := internalGeometries.Geometries()
	collisions, _, err := CheckCollisions(geoms, geoms, zeroPositionCollisions, defaultCollisionBufferMM, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(collisions), test.ShouldEqual, 0)

	// case 2: self collision - check only new collisions are returned
	input[4] = 2
	internalGeometries, _ = m.Geometries(input)
	test.That(t, internalGeometries, test.ShouldNotBeNil)

	geoms = internalGeometries.Geometries()
	collisions, _, err = CheckCollisions(geoms, geoms, zeroPositionCollisions, defaultCollisionBufferMM, true)
	test.That(t, err, test.ShouldBeNil)
	expectedCollisions := []Collision{
		{"xArm6:base_top", "xArm6:gripper_mount"},
		{"xArm6:base_top", "xArm6:wrist_link"},
	}
	test.That(t, collisionListsAlmostEqual(collisions, expectedCollisions), test.ShouldBeTrue)

	// case 3: add a collision specification that the last element of expectedCollisions should be ignored
	zeroPositionCollisions = append(zeroPositionCollisions, expectedCollisions[len(expectedCollisions)-1])

	collisions, _, err = CheckCollisions(geoms, geoms, zeroPositionCollisions, defaultCollisionBufferMM, true)
	test.That(t, err, test.ShouldBeNil)

	test.That(
		t,
		collisionListsAlmostEqual(collisions, expectedCollisions[:len(expectedCollisions)-1]),
		test.ShouldBeTrue,
	)
}

func TestCollisionMapErrors(t *testing.T) {
	bc1, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)

	t.Run("duplicate geometry names", func(t *testing.T) {
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("duplicate")
		geom2 := bc1.Transform(spatial.NewZeroPose())
		geom2.SetLabel("duplicate")
		_, err := createUniqueCollisionMap([]spatial.Geometry{geom1, geom2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "duplicate")
	})

	t.Run("unnamed geometries", func(t *testing.T) {
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("")
		geom2 := bc1.Transform(spatial.NewZeroPose())
		geom2.SetLabel("")
		geomMap, err := createUniqueCollisionMap([]spatial.Geometry{geom1, geom2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(geomMap), test.ShouldEqual, 2)
		// Verify unnamed geometries get unique names
		for name := range geomMap {
			test.That(t, name, test.ShouldStartWith, unnamedCollisionGeometryPrefix)
		}
	})
}

func TestCollisionMinDistance(t *testing.T) {
	bc1, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{1, 1, 1}, "")
	test.That(t, err, test.ShouldBeNil)

	// Create two non-colliding geometries with known separation
	geom1 := bc1.Transform(spatial.NewZeroPose())
	geom1.SetLabel("box1")
	geom2 := bc1.Transform(spatial.NewPoseFromPoint(r3.Vector{10, 0, 0}))
	geom2.SetLabel("box2")

	collisions, minDist, err := CheckCollisions(
		[]spatial.Geometry{geom1}, []spatial.Geometry{geom2}, nil, defaultCollisionBufferMM, true,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(collisions), test.ShouldEqual, 0)
	test.That(t, minDist, test.ShouldBeGreaterThan, 8.0)
}

func TestCollisionEarlyExit(t *testing.T) {
	bc1, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)

	// Create multiple colliding geometries
	geom1 := bc1.Transform(spatial.NewZeroPose())
	geom1.SetLabel("box1")
	geom2 := bc1.Transform(spatial.NewZeroPose())
	geom2.SetLabel("box2")
	geom3 := bc1.Transform(spatial.NewZeroPose())
	geom3.SetLabel("box3")

	geoms := []spatial.Geometry{geom1, geom2, geom3}

	// With collectAllCollisions=false, should return first collision only
	collisions, _, err := CheckCollisions(geoms, geoms, nil, defaultCollisionBufferMM, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(collisions), test.ShouldEqual, 1)

	// With collectAllCollisions=true, should return all collisions
	collisions, _, err = CheckCollisions(geoms, geoms, nil, defaultCollisionBufferMM, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(collisions), test.ShouldBeGreaterThan, 1)
}
