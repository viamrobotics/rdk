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

	o1 := &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

	l1 := &referenceframe.LinkConfig{
		ID:          "frame1",
		Parent:      referenceframe.World,
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
		Orientation: o1Cfg,
		Geometry:    &spatialmath.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
	}
	lif1, err := l1.ParseConfig()
	test.That(t, err, test.ShouldBeNil)

	l2 := &referenceframe.LinkConfig{
		ID:          "frame2",
		Parent:      "frame1",
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
	}
	lif2, err := l2.ParseConfig()
	test.That(t, err, test.ShouldBeNil)

	fsConfigs := []*referenceframe.FrameSystemPart{
		{
			FrameConfig: lif1,
		},
		{
			FrameConfig: lif2,
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

	o1 := &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

	l1 := &referenceframe.LinkConfig{
		ID:          "frame1",
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
		Orientation: o1Cfg,
	}
	lif1, err := l1.ParseConfig()
	test.That(t, err, test.ShouldBeNil)

	badFSConfigs := []*referenceframe.FrameSystemPart{
		{
			FrameConfig: lif1,
		},
	}
	fs, err := framesystem.NewFrameSystemFromParts("", "", badFSConfigs, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "there are no robot parts that connect to a 'world' node.")
	test.That(t, fs, test.ShouldBeNil)
}
