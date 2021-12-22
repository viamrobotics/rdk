package motionplan

import (
	"testing"

	"github.com/golang/geo/r3"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"
	"go.viam.com/test"
)

func TestCheckCollisions(t *testing.T) {
	// case 1: small collection of custom volumes
	bc := spatial.NewBox(r3.Vector{0.5, 0.5, 0.5})
	vols := make(map[string]spatial.Volume)
	vols["cube000"] = bc.NewVolume(spatial.NewZeroPose())
	vols["cube222"] = bc.NewVolume(spatial.NewPoseFromPoint(r3.Vector{2, 2, 2}))
	vols["cube333"] = bc.NewVolume(spatial.NewPoseFromPoint(r3.Vector{3, 3, 3}))
	cg, err := CheckCollisions(vols)
	test.That(t, err, test.ShouldBeNil)
	collisions := cg.Collisions()
	test.That(t, len(collisions), test.ShouldEqual, 1)
	collisionEqual(t, collisions[0], collision{"cube222", "cube333"})

	// case 2: zero position of ur5e arm
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	vols, err = m.Volume(make([]frame.Input, len(m.DoF())))
	test.That(t, err, test.ShouldBeNil)
	cg, err = CheckCollisions(vols)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cg.Collisions()), test.ShouldEqual, len(m.DoF()))
}

func TestUniqueCollisions(t *testing.T) {
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// zero position of ur5e arm
	input := make([]frame.Input, len(m.DoF()))
	vols, err := m.Volume(input)
	test.That(t, err, test.ShouldBeNil)
	zeroPositionCG, err := CheckCollisions(vols)
	test.That(t, err, test.ShouldBeNil)

	// case 1: no self collision - check no new collisions are returned
	input[3] = frame.Input{2}
	vols, err = m.Volume(input)
	test.That(t, err, test.ShouldBeNil)
	cg, err := CheckUniqueCollisions(vols, zeroPositionCG)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cg.Collisions()), test.ShouldEqual, 0)

	// case 2: self collision - check only new collisions are returned
	input[4] = frame.Input{2}
	vols, err = m.Volume(input)
	test.That(t, err, test.ShouldBeNil)
	cg, err = CheckUniqueCollisions(vols, zeroPositionCG)
	test.That(t, err, test.ShouldBeNil)
	collisions := cg.Collisions()
	test.That(t, len(collisions), test.ShouldEqual, 2)
	collisionEqual(t, collisions[0], collision{"UR5e:forearm_link", "UR5e:ee_link"})
	collisionEqual(t, collisions[1], collision{"UR5e:wrist_1_link", "UR5e:ee_link"})
}

// collisionEqual is a helper function to test equality between collision objects
func collisionEqual(t *testing.T, c1, c2 collision) {
	// fields can be out of order due to random nature of maps, check both cases
	equal := (c1.a == c2.a && c1.b == c2.b) || (c1.a == c2.b && c1.b == c2.a)
	test.That(t, equal, test.ShouldBeTrue)
}
