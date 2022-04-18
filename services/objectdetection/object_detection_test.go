package objectdetection_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/config"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/objectdetection"
	"go.viam.com/test"
)

func TestObjectDetection(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)
	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	srv, err := objectdetection.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	names, err := srv.GetDetectors(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "detector_3")
}
