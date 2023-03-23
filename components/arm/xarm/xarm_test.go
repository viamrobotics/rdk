package xarm

import (
	"context"
	"math"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	pb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	wbY   = -426.
)

// This will test solving the path to write the word "VIAM" on a whiteboard.
func TestWriteViam(t *testing.T) {
	fs := frame.NewEmptySimpleFrameSystem("test")

	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerOriginFrame, err := frame.NewStaticFrame(
		"marker_origin",
		spatial.NewPoseFromOrientation(&spatial.OrientationVectorDegrees{OY: -1, OZ: 1}),
	)
	test.That(t, err, test.ShouldBeNil)
	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerOriginFrame, m)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, markerOriginFrame)
	test.That(t, err, test.ShouldBeNil)

	eraserOriginFrame, err := frame.NewStaticFrame(
		"eraser_origin",
		spatial.NewPoseFromOrientation(&spatial.OrientationVectorDegrees{OY: 1, OZ: 1}),
	)
	test.That(t, err, test.ShouldBeNil)
	eraserFrame, err := frame.NewStaticFrame("eraser", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(eraserOriginFrame, m)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(eraserFrame, eraserOriginFrame)
	test.That(t, err, test.ShouldBeNil)

	moveFrame := eraserFrame

	// draw pos start
	goal := spatial.NewPoseFromProtobuf(&pb.Pose{
		X:  230,
		Y:  wbY + 10,
		Z:  600,
		OY: -1,
	})

	seedMap := map[string][]frame.Input{}

	seedMap[m.Name()] = home7

	steps, err := motionplan.PlanMotion(ctx, logger, frame.NewPoseInFrame(fs.World().Name(), goal), moveFrame, seedMap, fs, nil, nil, nil)
	test.That(t, err, test.ShouldBeNil)

	opt := map[string]interface{}{"motion_profile": motionplan.LinearMotionProfile}

	goToGoal := func(seedMap map[string][]frame.Input, goal spatial.Pose) map[string][]frame.Input {
		goalPiF := frame.NewPoseInFrame(fs.World().Name(), goal)

		waysteps, err := motionplan.PlanMotion(ctx, logger, goalPiF, moveFrame, seedMap, fs, nil, nil, opt)
		test.That(t, err, test.ShouldBeNil)
		return waysteps[len(waysteps)-1]
	}

	seed := steps[len(steps)-1]
	for _, goal = range viamPoints {
		seed = goToGoal(seed, goal)
	}
}

var viamPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 200, Y: wbY + 1.5, Z: 595, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: wbY + 1.5, Z: 595, OY: -1}),
}

func TestUpdateAction(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		defer server.Close()
	}()

	defer client.Close()

	cfg := config.Component{
		Name: "testarm",
		ConvertedAttributes: &AttrConfig{
			Speed:        0.3,
			Host:         "irrelevant",
			Acceleration: 0.1,
		},
	}

	shouldNotReconfigureCfg := config.Component{
		Name: "testarm",
		ConvertedAttributes: &AttrConfig{
			Speed:        0.5,
			Host:         "",
			Acceleration: 0.3,
		},
	}

	shouldReconfigureCfg := config.Component{
		Name: "testarm",
		ConvertedAttributes: &AttrConfig{
			Speed:        0.6,
			Host:         "new",
			Acceleration: 0.34,
		},
	}

	attrs, ok := cfg.ConvertedAttributes.(*AttrConfig)
	test.That(t, ok, test.ShouldBeTrue)

	xArm := &xArm{
		speed: attrs.Speed * math.Pi / 180,
		accel: attrs.Acceleration * math.Pi / 180,
		conn:  client,
	}

	// scenario where we do not reconfigure
	test.That(t, xArm.UpdateAction(&shouldNotReconfigureCfg), test.ShouldEqual, config.None)

	// scenario where we have to configure
	test.That(t, xArm.UpdateAction(&shouldReconfigureCfg), test.ShouldEqual, config.Reconfigure)

	// wrap with reconfigurable arm to test the codepath that will be executed during reconfigure
	reconfArm, err := arm.WrapWithReconfigurable(xArm, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	// scenario where we do not reconfigure
	obj, canUpdate := reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldNotReconfigureCfg), test.ShouldEqual, config.None)

	// scenario where we have to configure
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldReconfigureCfg), test.ShouldEqual, config.Reconfigure)
}
