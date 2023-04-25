package referenceframe

import (
	"testing"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func TestWorldStateConstruction(t *testing.T) {
	ws := NewEmptyWorldState()
	foo, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "foo")
	test.That(t, err, test.ShouldBeNil)
	bar, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "bar")
	test.That(t, err, test.ShouldBeNil)
	noname, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "")
	test.That(t, err, test.ShouldBeNil)
	unnamed, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "")
	test.That(t, err, test.ShouldBeNil)
	expectedErr := NewWorldStateNameError(foo.Label()).Error()

	// test that you can add two geometries of different names
	err = ws.AddObstacles("", foo, bar)
	test.That(t, err, test.ShouldBeNil)

	// test that you can't add two "foos" to the same frame
	err = ws.AddObstacles("", foo, foo)
	test.That(t, err.Error(), test.ShouldResemble, expectedErr)

	// test that you can't add two "foos" to different frames
	err = ws.AddObstacles("", foo)
	test.That(t, err.Error(), test.ShouldResemble, expectedErr)

	// test that you can add multiple geometries with no name
	err = ws.AddObstacles("", noname, unnamed)
	test.That(t, err, test.ShouldBeNil)
}

func TestWorldStateProtoConversions(t *testing.T) {

}
