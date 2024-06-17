package builtin

import (
	"bytes"
	"context"
	"flag"
	"math"
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

const numTests = 10
const timeout = 5.0 // seconds

var nameFlag = flag.String("name", "", "name of test to run")

var resultsDirectory = "results"

func createBaseSceneConfig(
	startInput []referenceframe.Input,
	goalPose spatialmath.Pose,
	artifactPath string,
	logger logging.Logger,
	seed int,
	scene int,
	useNew bool,
	earlyExit bool,
	earlyExitThreshold float64,
) (*motionplan.PlanRequest, error) {
	injectSlam := inject.NewSLAMService("test_slam")
	injectSlam.PointCloudMapFunc = func(ctx context.Context, returnEditedMap bool) (func() ([]byte, error), error) {

		return getPointCloudMap(filepath.Clean(artifact.MustPath(artifactPath)))
	}
	injectSlam.PositionFunc = func(ctx context.Context) (spatialmath.Pose, error) {
		return spatialmath.NewZeroPose(), nil
	}

	// create fake base
	baseCfg := resource.Config{
		Name:  "test_base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 20}},
	}
	fakeBase, _ := fake.NewBase(context.Background(), nil, baseCfg, logger)
	kb, _ := kinematicbase.WrapWithFakePTGKinematics(
		context.Background(),
		fakeBase.(*fake.Base),
		logger,
		referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewZeroPose()),
		kinematicbase.NewKinematicBaseOptions(),
		nil,
		5,
	)

	// Add frame system and needed frames
	fs := referenceframe.NewEmptyFrameSystem("test")
	fs.AddFrame(kb.Kinematics(), fs.World())

	// get point cloud data in the form of bytes from pcd
	pointCloudData, _ := slam.PointCloudMapFull(context.Background(), injectSlam, false)
	// store slam point cloud data  in the form of a recursive octree for collision checking
	octree, _ := pointcloud.ReadPCDToBasicOctree(bytes.NewReader(pointCloudData))
	worldState, _ := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{
		referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{octree}),
	}, nil)

	startMap := referenceframe.StartPositions(fs)

	options := make(map[string]interface{})
	options["rseed"] = seed
	options["scene"] = scene
	options["useNew"] = useNew
	options["earlyExit"] = earlyExit
	options["earlyExitThreshold"] = earlyExitThreshold

	return &motionplan.PlanRequest{
		Logger:             logger,
		StartConfiguration: startMap,
		Goal:               referenceframe.NewPoseInFrame(referenceframe.World, goalPose),
		Frame:              kb.Kinematics(),
		WorldState:         worldState,
		FrameSystem:        fs,
		StartPose:          spatialmath.NewZeroPose(),
		Options:            options,
	}, nil
}

func scene13(logger logging.Logger, seed int, useNew bool, earlyExit bool, earlyExitThreshold float64) (*motionplan.PlanRequest, error) {
	startInput := referenceframe.FloatsToInputs([]float64{0, 0, 0})
	goalPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0.277 * 1000, Y: 0.593 * 1000})
	return createBaseSceneConfig(startInput, goalPose, "pointcloud/octagonspace.pcd", logger, seed, 13, useNew, earlyExit, earlyExitThreshold)
}

func scene14(logger logging.Logger, seed int, useNew bool, earlyExit bool, earlyExitThreshold float64) (*motionplan.PlanRequest, error) {
	startInput := referenceframe.FloatsToInputs([]float64{0, 0, 0})
	goalPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 1.32 * 1000, Y: 0})
	return createBaseSceneConfig(startInput, goalPose, "pointcloud/octagonspace.pcd", logger, seed, 14, useNew, earlyExit, earlyExitThreshold)
}

func scene15(logger logging.Logger, seed int, useNew bool, earlyExit bool, earlyExitThreshold float64) (*motionplan.PlanRequest, error) {
	startInput := referenceframe.FloatsToInputs([]float64{-6.905 * 1000, 0.623 * 1000, 0})
	goalPose := spatialmath.NewPoseFromPoint(r3.Vector{X: -29.164 * 1000, Y: 3.433 * 1000})
	return createBaseSceneConfig(startInput, goalPose, "slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd", logger, seed, 15, useNew, earlyExit, earlyExitThreshold)
}

func scene16(logger logging.Logger, seed int, useNew bool, earlyExit bool, earlyExitThreshold float64) (*motionplan.PlanRequest, error) {
	startInput := referenceframe.FloatsToInputs([]float64{-19.376 * 1000, 2.305 * 1000, 0})
	goalPose := spatialmath.NewPoseFromPoint(r3.Vector{X: -27.946 * 1000, Y: -4.406 * 1000})
	return createBaseSceneConfig(startInput, goalPose, "slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd", logger, seed, 16, useNew, earlyExit, earlyExitThreshold)
}

func scene17(logger logging.Logger, seed int, useNew bool, earlyExit bool, earlyExitThreshold float64) (*motionplan.PlanRequest, error) {
	startInput := referenceframe.FloatsToInputs([]float64{0, 0, 0})
	goalPose := spatialmath.NewPoseFromPoint(r3.Vector{X: -5.959 * 1000, Y: -5.542 * 1000})
	return createBaseSceneConfig(startInput, goalPose, "slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd", logger, seed, 17, useNew, earlyExit, earlyExitThreshold)
}

func scene18(logger logging.Logger, seed int, useNew bool, earlyExit bool, earlyExitThreshold float64) (*motionplan.PlanRequest, error) {
	startInput := referenceframe.FloatsToInputs([]float64{0, 0, 0})
	goalPose := spatialmath.NewPoseFromPoint(r3.Vector{X: -52.555 * 1000, Y: -27.215 * 1000})
	return createBaseSceneConfig(startInput, goalPose, "slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd", logger, seed, 18, useNew, earlyExit, earlyExitThreshold)
}

type SceneFunc func(logger logging.Logger, seed int, useNew bool, earlyExit bool, earlyExitThreshold float64) (*motionplan.PlanRequest, error)

func TestPtgWithSlam(t *testing.T) {
	logger := logging.NewTestLogger(t)
	intToFunc := map[int]SceneFunc{
		13: scene13,
		14: scene14,
		15: scene15,
		16: scene16,
		17: scene17,
		18: scene18,
	}

	for sceneNum := 13; sceneNum <= 18; sceneNum++ {
		logger.Debugf("$DEBUG,---------------SCENE_%v---------------\n", sceneNum)
		for seed := 0; seed < 5; seed++ {
			// Start by running old on this seed to get baseline cutoff
			logger.Debugf("$DEBUG,Baseline:")
			useNew := false
			earlyExit := false
			earlyExitThreshold := 0.0
			scene, err := intToFunc[sceneNum](logger, seed, useNew, earlyExit, earlyExitThreshold)
			test.That(t, err, test.ShouldBeNil)
			_, motionerr := motionplan.PlanMotion(context.Background(), scene)
			test.That(t, motionerr, test.ShouldBeNil)

			// Get proper earlyExitThreshold
			earlyExitThreshold = scene.Options["earlyExitThreshold"].(float64)
			if earlyExitThreshold > 1000 {
				earlyExitThreshold = math.Ceil(earlyExitThreshold/1000+1) * 1000 // rounding to the nearest 1000
			} else {
				earlyExitThreshold = math.Ceil(earlyExitThreshold/100+1) * 100 // rounding to the nearest 100
			}

			logger.Debugf("$DEBUG,earlyExitThreshold:%v\n", earlyExitThreshold)

			// Run the old method with earlyExiting
			logger.Debugf("$DEBUG,old_w_early_exit:")
			useNew = false
			earlyExit = true
			scene, err = intToFunc[sceneNum](logger, seed, useNew, earlyExit, earlyExitThreshold)
			test.That(t, err, test.ShouldBeNil)
			_, motionerr = motionplan.PlanMotion(context.Background(), scene)
			test.That(t, motionerr, test.ShouldBeNil)

			// Run the new method with earlyExiting
			logger.Debugf("$DEBUG,new_w_early_exit:")
			useNew = true
			earlyExit = true
			scene, err = intToFunc[sceneNum](logger, seed, useNew, earlyExit, earlyExitThreshold)
			test.That(t, err, test.ShouldBeNil)
			_, motionerr = motionplan.PlanMotion(context.Background(), scene)
			test.That(t, motionerr, test.ShouldBeNil)
		}
	}

}
