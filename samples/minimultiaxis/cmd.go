package main

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/spatialmath"
	webserver "go.viam.com/rdk/web/server"
)

var (
	logger      = golog.NewDevelopmentLogger("minimultiaxis")
	gantryModel = "minimultiaxis"
)

func main() {
	utils.ContextualMain(webserver.RunServer, logger)
}

func init() {
	action.RegisterAction("home", home)
}

func home(ctx context.Context, r robot.Robot) {
	fs, err := framesystem.RobotFrameSystem(ctx, r)
	if err != nil {
		logger.Error(err)
		return
	}
	gantryFrame := fs.GetFrame(gantryModel)
	sfs := motionplan.NewSolvableFrameSystem(fs, logger)
	// seedMap := referenceframe.StartPositions(h.fs)
	opt := motionplan.NewDefaultPlannerOptions()
	opt.SetMetric(motionplan.NewPositionOnlyMetric())
	mma, err := gantry.FromRobot(r, gantryModel)
	if err != nil {
		logger.Error(err)
		return
	}
	curPos, err := mma.CurrentInputs(ctx)
	if err != nil {
		logger.Error(err)
	}
	seedMap := referenceframe.StartPositions(fs)
	logger.Debugf("Frame Inputs: %+v, %v", seedMap, gantryFrame)
	seedMap[gantryModel] = curPos
	home := spatialmath.NewPoseFromProtobuf(&commonpb.Pose{
		X: 45,
		Y: 45,
		Z: 2,
	})
	outputs, err := sfs.SolvePoseWithOptions(ctx, seedMap, home, gantryModel, referenceframe.World, opt)
	if err != nil {
		logger.Error(err)
		return
	}
	logger.Debugf("Output solution: %+v", outputs[len(outputs)-1][gantryModel])
	mma.GoToInputs(ctx, outputs[len(outputs)-1][gantryModel])
}
