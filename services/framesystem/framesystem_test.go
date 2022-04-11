package framesystem_test

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/spatialmath"
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
	injectRobot.RemoteNamesFunc = func() []string {
		return []string{}
	}

	ctx := context.Background()
	service, err := framesystem.New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	parts, err := service.Config(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, parts, test.ShouldHaveLength, 0)
	fs, err := framesystem.NewFrameSystemFromParts("test", "", parts, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 0)
}

func TestNewFrameSystemFromParts(t *testing.T) {
	logger := golog.NewTestLogger(t)
	fsConfigs := []*config.FrameSystemPart{
		{
			Name: "frame1",
			FrameConfig: &config.Frame{
				Parent:      referenceframe.World,
				Translation: spatialmath.TranslationConfig{X: 1, Y: 2, Z: 3},
				Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
			},
		},
		{
			Name: "frame2",
			FrameConfig: &config.Frame{
				Parent:      "frame1",
				Translation: spatialmath.TranslationConfig{X: 1, Y: 2, Z: 3},
			},
		},
	}
	frameSys, err := framesystem.NewFrameSystemFromParts("", "", fsConfigs, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frameSys, test.ShouldNotBeNil)
	frame1 := frameSys.GetFrame("frame1")
	frame1Offset := frameSys.GetFrame("frame1_offset")
	frame2 := frameSys.GetFrame("frame2")
	frame2Offset := frameSys.GetFrame("frame2_offset")

	resFrame, err := frameSys.Parent(frame2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame2Offset)
	resFrame, err = frameSys.Parent(frame2Offset)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame1)
	resFrame, err = frameSys.Parent(frame1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame1Offset)
	resFrame, err = frameSys.Parent(frame1Offset)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frameSys.World())
}

func TestNewFrameSystemFromPartsBadConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	badFSConfigs := []*config.FrameSystemPart{
		{
			Name: "frame1",
			FrameConfig: &config.Frame{
				Translation: spatialmath.TranslationConfig{X: 1, Y: 2, Z: 3},
				Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
			},
		},
	}
	fs, err := framesystem.NewFrameSystemFromParts("", "", badFSConfigs, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "there are no robot parts that connect to a 'world' node.")
	test.That(t, fs, test.ShouldBeNil)
}
