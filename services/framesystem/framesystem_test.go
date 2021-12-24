package framesystem_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/testutils/inject"
)

// A robot with no components should return a frame system with just a world referenceframe.
func TestEmptyConfigFrameService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := &inject.Robot{}
	cfg := config.Config{
		Components: []config.Component{},
	}
	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return &cfg, nil
	}

	ctx := context.Background()
	service, err := framesystem.New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	fs, err := service.LocalFrameSystem(ctx, "test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 0)

	parts, err := service.FrameSystemConfig(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, parts, test.ShouldHaveLength, 0)
}
