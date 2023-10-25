package xarm

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
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
	fs := frame.NewEmptyFrameSystem("test")

	ctx := context.Background()
	logger := logging.NewTestLogger(t)
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

	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             logger,
		Goal:               frame.NewPoseInFrame(frame.World, goal),
		Frame:              moveFrame,
		StartConfiguration: seedMap,
		FrameSystem:        fs,
	})
	test.That(t, err, test.ShouldBeNil)

	opt := map[string]interface{}{"motion_profile": motionplan.LinearMotionProfile}
	goToGoal := func(seedMap map[string][]frame.Input, goal spatial.Pose) map[string][]frame.Input {
		plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
			Logger:             logger,
			Goal:               frame.NewPoseInFrame(fs.World().Name(), goal),
			Frame:              moveFrame,
			StartConfiguration: seedMap,
			FrameSystem:        fs,
			Options:            opt,
		})
		test.That(t, err, test.ShouldBeNil)
		return plan[len(plan)-1]
	}

	seed := plan[len(plan)-1]
	for _, goal = range viamPoints {
		seed = goToGoal(seed, goal)
	}
}

var viamPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 200, Y: wbY + 1.5, Z: 595, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: wbY + 1.5, Z: 595, OY: -1}),
}

func TestReconfigure(t *testing.T) {
	listener1, err := net.Listen("tcp4", "127.0.0.1:0")
	test.That(t, err, test.ShouldBeNil)
	defer listener1.Close()
	addr1 := listener1.Addr().String()
	listener2, err := net.Listen("tcp4", "127.0.0.1:0")
	test.That(t, err, test.ShouldBeNil)
	defer listener2.Close()
	addr2 := listener2.Addr().String()
	host1, port1Str, err := net.SplitHostPort(addr1)
	test.That(t, err, test.ShouldBeNil)
	host2, port2Str, err := net.SplitHostPort(addr2)
	test.That(t, err, test.ShouldBeNil)

	port1, err := strconv.ParseInt(port1Str, 10, 32)
	test.That(t, err, test.ShouldBeNil)
	port2, err := strconv.ParseInt(port2Str, 10, 32)
	test.That(t, err, test.ShouldBeNil)

	cfg := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:        0.3,
			Host:         host1,
			Port:         int(port1),
			parsedPort:   port1Str,
			Acceleration: 0.1,
		},
	}

	shouldNotReconnectCfg := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:        0.5,
			Host:         host1,
			Port:         int(port1),
			parsedPort:   port1Str,
			Acceleration: 0.3,
		},
	}

	shouldReconnectCfg := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:        0.6,
			Host:         host2,
			Port:         int(port2),
			parsedPort:   port2Str,
			Acceleration: 0.34,
		},
	}

	conf, err := resource.NativeConfig[*Config](cfg)
	test.That(t, err, test.ShouldBeNil)
	confNotReconnect, ok := shouldNotReconnectCfg.ConvertedAttributes.(*Config)
	test.That(t, ok, test.ShouldBeTrue)

	conn1, err := net.Dial("tcp", listener1.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	xArm := &xArm{
		speed:  float32(utils.DegToRad(float64(conf.Speed))),
		logger: logging.NewTestLogger(t),
	}
	xArm.mu.Lock()
	xArm.conn = conn1
	xArm.mu.Unlock()

	ctx := context.Background()

	// scenario where we do no nothing
	prevSpeed := xArm.speed
	test.That(t, xArm.Reconfigure(ctx, nil, cfg), test.ShouldBeNil)

	xArm.mu.Lock()
	currentConn := xArm.conn
	xArm.mu.Unlock()
	test.That(t, currentConn, test.ShouldEqual, conn1)
	test.That(t, xArm.speed, test.ShouldEqual, prevSpeed)

	// scenario where we do not reconnect
	test.That(t, xArm.Reconfigure(ctx, nil, shouldNotReconnectCfg), test.ShouldBeNil)

	xArm.mu.Lock()
	currentConn = xArm.conn
	xArm.mu.Unlock()
	test.That(t, currentConn, test.ShouldEqual, conn1)
	test.That(t, xArm.speed, test.ShouldEqual, float32(utils.DegToRad(float64(confNotReconnect.Speed))))

	// scenario where we have to reconnect
	err = xArm.Reconfigure(ctx, nil, shouldReconnectCfg)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "failed to start")

	xArm.mu.Lock()
	currentConn = xArm.conn
	xArm.mu.Unlock()
	test.That(t, currentConn, test.ShouldNotEqual, conn1)
	test.That(t, xArm.speed, test.ShouldEqual, float32(utils.DegToRad(float64(confNotReconnect.Speed))))
}
