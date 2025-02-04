package gripper_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/gripper/fake"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

const (
	testGripperName    = "gripper1"
	testGripperName2   = "gripper2"
	failGripperName    = "gripper3"
	missingGripperName = "gripper4"
)

func TestGetGeometries(t *testing.T) {
	cfg := resource.Config{
		Name:  "fakeGripper",
		API:   gripper.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{X: 10, Y: 5, Z: 10}},
	}
	gripper, err := fake.NewGripper(context.Background(), nil, cfg, nil)
	test.That(t, err, test.ShouldBeNil)

	geometries, err := gripper.Geometries(context.Background(), nil)
	expected, _ := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 10, Y: 5, Z: 10}, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometries, test.ShouldResemble, []spatialmath.Geometry{expected})
}
