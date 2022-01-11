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
	// case 1: small collection of custom volumes
	bc := spatial.NewBox(r3.Vector{1, 1, 1})
	vols := make(map[string]spatial.Volume)
	vols["cube000"] = bc.NewVolume(spatial.NewZeroPose())
	vols["cube222"] = bc.NewVolume(spatial.NewPoseFromPoint(r3.Vector{3, 3, 3}))
	vols["cube333"] = bc.NewVolume(spatial.NewPoseFromPoint(r3.Vector{4, 4, 4}))
	cg, err := CheckCollisions(vols)
	test.That(t, err, test.ShouldBeNil)
	collisions := cg.Collisions()
	test.That(t, len(collisions), test.ShouldEqual, 1)
	test.That(t, collisionEqual(collisions[0], Collision{"cube222", "cube333", 1}), test.ShouldBeTrue)

	// case 2: zero position of ur5e arm
	m, err := frame.ParseJSONFile(utils.ResolveFile("component/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	vols, _ = m.Volumes(make([]frame.Input, len(m.DoF())))
	test.That(t, vols, test.ShouldNotBeNil)
	cg, err = CheckCollisions(vols)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cg.Collisions()), test.ShouldEqual, len(m.DoF()))
}

func TestUniqueCollisions(t *testing.T) {
	m, err := frame.ParseJSONFile(utils.ResolveFile("component/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// zero position of ur5e arm
	input := make([]frame.Input, len(m.DoF()))
	vols, _ := m.Volumes(input)
	test.That(t, vols, test.ShouldNotBeNil)
	zeroPositionCG, err := CheckCollisions(vols)
	test.That(t, err, test.ShouldBeNil)

	// case 1: no self collision - check no new collisions are returned
	input[3] = frame.Input{1}
	vols, _ = m.Volumes(input)
	test.That(t, vols, test.ShouldNotBeNil)
	cg, err := CheckUniqueCollisions(vols, zeroPositionCG)
	test.That(t, err, test.ShouldBeNil)
	a := cg.Collisions()
	test.That(t, len(cg.Collisions()), test.ShouldEqual, 0)
	test.That(t, len(a), test.ShouldEqual, 0)

	// case 2: self collision - check only new collisions are returned
	input[4] = frame.Input{2.5}
	vols, _ = m.Volumes(input)
	test.That(t, vols, test.ShouldNotBeNil)
	cg, err = CheckUniqueCollisions(vols, zeroPositionCG)
	test.That(t, err, test.ShouldBeNil)
	cols := cg.Collisions()
	test.That(t, len(cols), test.ShouldEqual, 2)
	equal := collisionsEqual(cols, [2]Collision{{"UR5e:forearm_link", "UR5e:ee_link", 19.5}, {"UR5e:wrist_1_link", "UR5e:ee_link", 0}})
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
