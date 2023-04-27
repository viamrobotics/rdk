package referenceframe

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

func TestWorldStateConstruction(t *testing.T) {
	foo, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "foo")
	test.That(t, err, test.ShouldBeNil)
	bar, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "bar")
	test.That(t, err, test.ShouldBeNil)
	noname, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "")
	test.That(t, err, test.ShouldBeNil)
	unnamed, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "")
	test.That(t, err, test.ShouldBeNil)
	expectedErr := NewDuplicateGeometryNameError(foo.Label()).Error()

	// test that you can add two geometries of different names
	_, err = NewWorldState([]*GeometriesInFrame{NewGeometriesInFrame("", []spatialmath.Geometry{foo, bar})}, nil)
	test.That(t, err, test.ShouldBeNil)

	// test that you can't add two "foos" to the same frame
	_, err = NewWorldState([]*GeometriesInFrame{NewGeometriesInFrame("", []spatialmath.Geometry{foo, foo})}, nil)
	test.That(t, err.Error(), test.ShouldResemble, expectedErr)

	// test that you can't add two "foos" to different frames
	_, err = NewWorldState(
		[]*GeometriesInFrame{
			NewGeometriesInFrame("", []spatialmath.Geometry{foo, bar}),
			NewGeometriesInFrame("", []spatialmath.Geometry{foo}),
		},
		nil,
	)
	test.That(t, err.Error(), test.ShouldResemble, expectedErr)

	// test that you can add multiple geometries with no name
	_, err = NewWorldState([]*GeometriesInFrame{NewGeometriesInFrame("", []spatialmath.Geometry{noname, unnamed})}, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestWorldStateProtoConversions(t *testing.T) {
}
