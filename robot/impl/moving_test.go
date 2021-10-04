package robotimpl_test

import (
	"context"
	"testing"

	"go.viam.com/core/config"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/spatialmath"
	"go.viam.com/test"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
)

func TestArmThatDoesntMove(t *testing.T) {
	cfg, err := config.Read("data/fake.json")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(r.CameraNames()), test.ShouldEqual, 1)
	cameraName := r.CameraNames()[0]
	// point to move to in camera frame is (500, 0, 0)
	err = robotimpl.MoveGripper(context.Background(), r, spatialmath.NewPoseFromPoint(r3.Vector{500, 0, 0}), cameraName)
}
