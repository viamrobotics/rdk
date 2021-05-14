package referenceframe

import (
	"testing"

	"go.viam.com/test"
)

func TestOffset(t *testing.T) {
	// this is a camera looking down 300mm left and 300m above the base of an arm
	o := Offset{Position{X: 300, Y: 0, Z: 300}, Rotation{Rx: 0, Ry: 180, Rz: 0}}

	// the camera found something straight ahead 300mm
	p := Position{X: 0, Y: 0, Z: 300}

	p = o.UnProject(p)
	test.That(t, p.X, test.ShouldEqual, 0)
	test.That(t, p.Y, test.ShouldEqual, 300)
	test.That(t, p.Z, test.ShouldEqual, 0)
}

/*
func TestFindTranslation(t *testing.T) {
	world := &basicFrame{name:"base"}
	arm := &basicFrame{name:"arm"}
	camera := &basicFrame{name:"camera"}

	world.AddChild(arm, Offset{Position{X: 100, Y: 100, Z: 100}, Rotation{}})
	world.AddChild(camera, Offset{Position{X: 200, Y: 200, Z: 200}, Rotation{Ry: 180}})
}
*/
