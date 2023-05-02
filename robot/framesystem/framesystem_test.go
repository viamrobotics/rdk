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
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/spatialmath"
)

// A robot with no components should return a frame system with just a world referenceframe.
func TestEmptyConfigFrameService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	r, err := robotimpl.New(ctx, &config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)
	fsCfg, err := r.FrameSystemConfig(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsCfg.Parts, test.ShouldHaveLength, 0)
	fs, err := framesystem.NewFrameSystemFromConfig("test", fsCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 0)
}

func TestNewFrameSystemFromParts(t *testing.T) {
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

	parts := []*referenceframe.FrameSystemPart{
		{
			FrameConfig: lif1,
		},
		{
			FrameConfig: lif2,
		},
	}
	frameSys, err := framesystem.NewFrameSystemFromConfig("test", &framesystem.Config{Parts: parts})
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

	badParts := []*referenceframe.FrameSystemPart{
		{
			FrameConfig: lif1,
		},
	}
	fs, err := framesystem.NewFrameSystemFromConfig("test", &framesystem.Config{Parts: badParts})
	test.That(t, err.Error(), test.ShouldContainSubstring, "there are no robot parts that connect to a 'world' node.")
	test.That(t, fs, test.ShouldBeNil)
}
