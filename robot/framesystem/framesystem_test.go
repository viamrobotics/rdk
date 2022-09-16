package framesystem_test

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
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
	service := framesystem.New(ctx, injectRobot, logger)
	parts, err := service.Config(ctx, nil)
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
				Translation: r3.Vector{X: 1, Y: 2, Z: 3},
				Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
			},
		},
		{
			Name: "frame2",
			FrameConfig: &config.Frame{
				Parent:      "frame1",
				Translation: r3.Vector{X: 1, Y: 2, Z: 3},
			},
		},
	}
	frameSys, err := framesystem.NewFrameSystemFromParts("", "", fsConfigs, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frameSys, test.ShouldNotBeNil)
	frame1 := frameSys.Frame("frame1")
	frame1Origin := frameSys.Frame("frame1_origin")
	frame2 := frameSys.Frame("frame2")
	frame2Origin := frameSys.Frame("frame2_origin")

	resFrame, err := frameSys.Parent(frame2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame2Origin)
	resFrame, err = frameSys.Parent(frame2Origin)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame1)
	resFrame, err = frameSys.Parent(frame1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame1Origin)
	resFrame, err = frameSys.Parent(frame1Origin)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frameSys.World())
}

func TestNewFrameSystemFromPartsBadConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	badFSConfigs := []*config.FrameSystemPart{
		{
			Name: "frame1",
			FrameConfig: &config.Frame{
				Translation: r3.Vector{X: 1, Y: 2, Z: 3},
				Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
			},
		},
	}
	fs, err := framesystem.NewFrameSystemFromParts("", "", badFSConfigs, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "there are no robot parts that connect to a 'world' node.")
	test.That(t, fs, test.ShouldBeNil)
}
