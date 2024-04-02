package referenceframe

import (
	"fmt"
	"testing"

	"github.com/jedib0t/go-pretty/v6/table"
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
	_, err = NewWorldState([]*GeometriesInFrame{NewGeometriesInFrame("world", []spatialmath.Geometry{foo, bar})}, nil)
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

func TestString(t *testing.T) {
	foo, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "foo")
	test.That(t, err, test.ShouldBeNil)
	bar, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 5, "bar")
	test.That(t, err, test.ShouldBeNil)
	testgeo, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 7, "testgeo")
	test.That(t, err, test.ShouldBeNil)

	ws, err := NewWorldState([]*GeometriesInFrame{
		NewGeometriesInFrame("world", []spatialmath.Geometry{foo, bar}),
		NewGeometriesInFrame("camera", []spatialmath.Geometry{testgeo}),
	}, nil)
	test.That(t, err, test.ShouldBeNil)

	testTable := table.NewWriter()
	testTable.AppendHeader(table.Row{"Name", "Geometry Type", "Parent"})
	testTable.AppendRow([]interface{}{
		"foo",
		foo.(fmt.Stringer).String(),
		"world",
	})
	testTable.AppendRow([]interface{}{
		"bar",
		bar.(fmt.Stringer).String(),
		"world",
	})
	testTable.AppendRow([]interface{}{
		"testgeo",
		testgeo.(fmt.Stringer).String(),
		"camera",
	})

	test.That(t, ws.String(), test.ShouldEqual, testTable.Render())
}
