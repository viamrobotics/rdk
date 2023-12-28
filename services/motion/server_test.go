package motion_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	vprotoutils "go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func newServer(resources map[resource.Name]motion.Service) (pb.MotionServiceServer, error) {
	coll, err := resource.NewAPIResourceCollection(motion.API, resources)
	if err != nil {
		return nil, err
	}
	return motion.NewRPCServiceServer(coll).(pb.MotionServiceServer), nil
}

func TestServerMove(t *testing.T) {
	grabRequest := &pb.MoveRequest{
		Name:          testMotionServiceName.ShortName(),
		ComponentName: protoutils.ResourceNameToProto(gripper.Named("fake")),
		Destination:   referenceframe.PoseInFrameToProtobuf(referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())),
	}

	resources := map[resource.Name]motion.Service{}
	server, err := newServer(resources)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:motion/motion1\" not found"))

	// error
	injectMS := &inject.MotionService{}
	resources = map[resource.Name]motion.Service{
		testMotionServiceName: injectMS,
	}
	server, err = newServer(resources)
	test.That(t, err, test.ShouldBeNil)
	passedErr := errors.New("fake move error")
	injectMS.MoveFunc = func(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *referenceframe.WorldState,
		constraints *pb.Constraints,
		extra map[string]interface{},
	) (bool, error) {
		return false, passedErr
	}

	_, err = server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, passedErr)

	// returns response
	successfulMoveFunc := func(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *referenceframe.WorldState,
		constraints *pb.Constraints,
		extra map[string]interface{},
	) (bool, error) {
		return true, nil
	}
	injectMS.MoveFunc = successfulMoveFunc
	resp, err := server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSuccess(), test.ShouldBeTrue)

	// Multiple Servies names Valid
	injectMS = &inject.MotionService{}
	resources = map[resource.Name]motion.Service{
		testMotionServiceName:  injectMS,
		testMotionServiceName2: injectMS,
	}
	server, _ = newServer(resources)
	injectMS.MoveFunc = successfulMoveFunc
	resp, err = server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSuccess(), test.ShouldBeTrue)
	grabRequest2 := &pb.MoveRequest{
		Name:          testMotionServiceName2.ShortName(),
		ComponentName: protoutils.ResourceNameToProto(gripper.Named("fake")),
		Destination:   referenceframe.PoseInFrameToProtobuf(referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())),
	}
	resp, err = server.Move(context.Background(), grabRequest2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSuccess(), test.ShouldBeTrue)
}

func TestServerMoveOnGlobe(t *testing.T) {
	injectMS := &inject.MotionService{}
	resources := map[resource.Name]motion.Service{
		testMotionServiceName: injectMS,
	}
	server, err := newServer(resources)
	test.That(t, err, test.ShouldBeNil)
	t.Run("returns error without calling MoveOnGlobe if req.Name doesn't map to a resource", func(t *testing.T) {
		moveOnGlobeRequest := &pb.MoveOnGlobeRequest{
			ComponentName:      protoutils.ResourceNameToProto(base.Named("test-base")),
			Destination:        &commonpb.GeoPoint{Latitude: 0.0, Longitude: 0.0},
			MovementSensorName: protoutils.ResourceNameToProto(movementsensor.Named("test-gps")),
		}
		injectMS.MoveOnGlobeFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
			t.Log("should not be called")
			t.FailNow()
			return uuid.Nil, errors.New("should not be called")
		}

		moveOnGlobeResponse, err := server.MoveOnGlobe(context.Background(), moveOnGlobeRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:motion/\" not found"))
		test.That(t, moveOnGlobeResponse, test.ShouldBeNil)
	})

	t.Run("returns error if destination is nil without calling MoveOnGlobe", func(t *testing.T) {
		moveOnGlobeRequest := &pb.MoveOnGlobeRequest{
			Name:               testMotionServiceName.ShortName(),
			ComponentName:      protoutils.ResourceNameToProto(base.Named("test-base")),
			MovementSensorName: protoutils.ResourceNameToProto(movementsensor.Named("test-gps")),
		}
		injectMS.MoveOnGlobeFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
			t.Log("should not be called")
			t.FailNow()
			return uuid.Nil, errors.New("should not be called")
		}

		moveOnGlobeResponse, err := server.MoveOnGlobe(context.Background(), moveOnGlobeRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("must provide a destination"))
		test.That(t, moveOnGlobeResponse, test.ShouldBeNil)
	})

	validMoveOnGlobeRequest := &pb.MoveOnGlobeRequest{
		Name:               testMotionServiceName.ShortName(),
		ComponentName:      protoutils.ResourceNameToProto(base.Named("test-base")),
		Destination:        &commonpb.GeoPoint{Latitude: 0.0, Longitude: 0.0},
		MovementSensorName: protoutils.ResourceNameToProto(movementsensor.Named("test-gps")),
	}

	t.Run("returns error when MoveOnGlobe returns an error", func(t *testing.T) {
		notYetImplementedErr := errors.New("Not yet implemented")

		injectMS.MoveOnGlobeFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
			return uuid.Nil, notYetImplementedErr
		}
		moveOnGlobeResponse, err := server.MoveOnGlobe(context.Background(), validMoveOnGlobeRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, notYetImplementedErr.Error())
		test.That(t, moveOnGlobeResponse, test.ShouldBeNil)
	})

	t.Run("sets heading to NaN if nil in request", func(t *testing.T) {
		firstExecutionID := uuid.New()
		secondExecutionID := uuid.New()
		injectMS.MoveOnGlobeFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
			test.That(t, math.IsNaN(req.Heading), test.ShouldBeTrue)
			return firstExecutionID, nil
		}
		moveOnGlobeResponse, err := server.MoveOnGlobe(context.Background(), validMoveOnGlobeRequest)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moveOnGlobeResponse.ExecutionId, test.ShouldEqual, firstExecutionID.String())

		reqHeading := 6.
		injectMS.MoveOnGlobeFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
			test.That(t, req.Heading, test.ShouldAlmostEqual, reqHeading)
			return secondExecutionID, nil
		}

		validMoveOnGlobeRequest.Heading = &reqHeading
		moveOnGlobeResponse, err = server.MoveOnGlobe(context.Background(), validMoveOnGlobeRequest)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moveOnGlobeResponse.ExecutionId, test.ShouldEqual, secondExecutionID.String())
	})

	t.Run("returns success when MoveOnGlobe returns success", func(t *testing.T) {
		expectedComponentName := base.Named("test-base")
		expectedMovSensorName := movementsensor.Named("test-gps")
		reqHeading := 3.

		boxDims := r3.Vector{X: 5, Y: 50, Z: 10}

		geometries1, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{X: 50, Y: 0, Z: 0}),
			boxDims,
			"wall")
		test.That(t, err, test.ShouldBeNil)

		geometries2, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 70, Z: 0}),
			boxDims,
			"other wall")
		test.That(t, err, test.ShouldBeNil)

		geoObstacle1 := spatialmath.NewGeoObstacle(geo.NewPoint(70, 40), []spatialmath.Geometry{geometries1})
		geoObstacle2 := spatialmath.NewGeoObstacle(geo.NewPoint(-70, 40), []spatialmath.Geometry{geometries2})
		obs := []*commonpb.GeoObstacle{
			spatialmath.GeoObstacleToProtobuf(geoObstacle1),
			spatialmath.GeoObstacleToProtobuf(geoObstacle2),
		}
		angularDegsPerSec := 1.
		linearMPerSec := 2.
		planDeviationM := 3.
		obstaclePollingFrequencyHz := 4.
		positionPollingFrequencyHz := 5.
		obstacleDetectorsPB := []*pb.ObstacleDetector{
			{
				VisionService: protoutils.ResourceNameToProto(vision.Named("vision service 1")),
				Camera:        protoutils.ResourceNameToProto(camera.Named("camera 1")),
			},
			{
				VisionService: protoutils.ResourceNameToProto(vision.Named("vision service 2")),
				Camera:        protoutils.ResourceNameToProto(camera.Named("camera 2")),
			},
		}

		moveOnGlobeRequest := &pb.MoveOnGlobeRequest{
			Name:               testMotionServiceName.ShortName(),
			Heading:            &reqHeading,
			ComponentName:      protoutils.ResourceNameToProto(expectedComponentName),
			Destination:        &commonpb.GeoPoint{Latitude: 1.0, Longitude: 2.0},
			MovementSensorName: protoutils.ResourceNameToProto(expectedMovSensorName),
			Obstacles:          obs,
			MotionConfiguration: &pb.MotionConfiguration{
				AngularDegsPerSec:          &angularDegsPerSec,
				LinearMPerSec:              &linearMPerSec,
				PlanDeviationM:             &planDeviationM,
				ObstaclePollingFrequencyHz: &obstaclePollingFrequencyHz,
				PositionPollingFrequencyHz: &positionPollingFrequencyHz,
				ObstacleDetectors:          obstacleDetectorsPB,
			},
		}

		firstExecutionID := uuid.New()
		injectMS.MoveOnGlobeFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
			test.That(t, req.ComponentName, test.ShouldResemble, expectedComponentName)
			test.That(t, req.Destination, test.ShouldNotBeNil)
			test.That(t, req.Destination, test.ShouldResemble, geo.NewPoint(1, 2))
			test.That(t, req.Heading, test.ShouldResemble, reqHeading)
			test.That(t, req.MovementSensorName, test.ShouldResemble, expectedMovSensorName)
			test.That(t, len(req.Obstacles), test.ShouldEqual, 2)
			test.That(t, req.Obstacles[0], test.ShouldResemble, geoObstacle1)
			test.That(t, req.Obstacles[1], test.ShouldResemble, geoObstacle2)
			test.That(t, req.MotionCfg.AngularDegsPerSec, test.ShouldAlmostEqual, angularDegsPerSec)
			test.That(t, req.MotionCfg.LinearMPerSec, test.ShouldAlmostEqual, linearMPerSec)
			test.That(t, req.MotionCfg.PlanDeviationMM, test.ShouldAlmostEqual, planDeviationM*1000)
			test.That(t, req.MotionCfg.ObstaclePollingFreqHz, test.ShouldAlmostEqual, obstaclePollingFrequencyHz)
			test.That(t, req.MotionCfg.PositionPollingFreqHz, test.ShouldAlmostEqual, positionPollingFrequencyHz)
			test.That(t, len(req.MotionCfg.ObstacleDetectors), test.ShouldAlmostEqual, 2)
			test.That(t, req.MotionCfg.ObstacleDetectors[0].VisionServiceName, test.ShouldResemble, vision.Named("vision service 1"))
			test.That(t, req.MotionCfg.ObstacleDetectors[0].CameraName, test.ShouldResemble, camera.Named("camera 1"))
			test.That(t, req.MotionCfg.ObstacleDetectors[1].VisionServiceName, test.ShouldResemble, vision.Named("vision service 2"))
			test.That(t, req.MotionCfg.ObstacleDetectors[1].CameraName, test.ShouldResemble, camera.Named("camera 2"))
			return firstExecutionID, nil
		}
		moveOnGlobeResponse, err := server.MoveOnGlobe(context.Background(), moveOnGlobeRequest)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moveOnGlobeResponse.ExecutionId, test.ShouldEqual, firstExecutionID.String())
	})
}

func TestServerMoveOnMapNew(t *testing.T) {
	injectMS := &inject.MotionService{}
	resources := map[resource.Name]motion.Service{
		testMotionServiceName: injectMS,
	}
	server, err := newServer(resources)
	test.That(t, err, test.ShouldBeNil)

	t.Run("returns error without calling MoveOnMapNew if req.Name doesn't map to a resource", func(t *testing.T) {
		moveOnMapNewRequest := &pb.MoveOnMapNewRequest{
			ComponentName:   protoutils.ResourceNameToProto(base.Named("test-base")),
			Destination:     spatialmath.PoseToProtobuf(spatialmath.NewZeroPose()),
			SlamServiceName: protoutils.ResourceNameToProto(slam.Named("test-slam")),
		}
		injectMS.MoveOnMapNewFunc = func(ctx context.Context, req motion.MoveOnMapReq) (motion.ExecutionID, error) {
			t.Log("should not be called")
			t.FailNow()
			return uuid.Nil, errors.New("should not be called")
		}

		moveOnMapNewRespose, err := server.MoveOnMapNew(context.Background(), moveOnMapNewRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:motion/\" not found"))
		test.That(t, moveOnMapNewRespose, test.ShouldBeNil)
	})

	t.Run("returns error if destination is nil without calling MoveOnMapNew", func(t *testing.T) {
		moveOnMapNewRequest := &pb.MoveOnMapNewRequest{
			Name:            testMotionServiceName.ShortName(),
			ComponentName:   protoutils.ResourceNameToProto(base.Named("test-base")),
			Destination:     nil,
			SlamServiceName: protoutils.ResourceNameToProto(slam.Named("test-slam")),
		}
		injectMS.MoveOnMapNewFunc = func(ctx context.Context, req motion.MoveOnMapReq) (motion.ExecutionID, error) {
			t.Log("should not be called")
			t.FailNow()
			return uuid.Nil, errors.New("should not be called")
		}

		moveOnMapNewRespose, err := server.MoveOnMapNew(context.Background(), moveOnMapNewRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("received nil *commonpb.Pose for destination"))
		test.That(t, moveOnMapNewRespose, test.ShouldBeNil)
	})

	validMoveOnMapNewRequest := &pb.MoveOnMapNewRequest{
		Name:            testMotionServiceName.ShortName(),
		ComponentName:   protoutils.ResourceNameToProto(base.Named("test-base")),
		Destination:     spatialmath.PoseToProtobuf(spatialmath.NewZeroPose()),
		SlamServiceName: protoutils.ResourceNameToProto(slam.Named("test-slam")),
	}

	t.Run("returns error when MoveOnMapNew returns an error", func(t *testing.T) {
		notYetImplementedErr := errors.New("Not yet implemented")

		injectMS.MoveOnMapNewFunc = func(ctx context.Context, req motion.MoveOnMapReq) (motion.ExecutionID, error) {
			return uuid.Nil, notYetImplementedErr
		}
		moveOnMapNewRespose, err := server.MoveOnMapNew(context.Background(), validMoveOnMapNewRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, notYetImplementedErr)
		test.That(t, moveOnMapNewRespose, test.ShouldBeNil)
	})

	t.Run("returns success when MoveOnMapNew returns success", func(t *testing.T) {
		expectedComponentName := base.Named("test-base")
		expectedSlamName := slam.Named("test-slam")
		expectedDestination := spatialmath.PoseToProtobuf(spatialmath.NewZeroPose())

		angularDegsPerSec := 1.
		linearMPerSec := 2.
		planDeviationM := 3.
		obstaclePollingFrequencyHz := 4.
		positionPollingFrequencyHz := 5.
		obstacleDetectorsPB := []*pb.ObstacleDetector{
			{
				VisionService: protoutils.ResourceNameToProto(vision.Named("vision service 1")),
				Camera:        protoutils.ResourceNameToProto(camera.Named("camera 1")),
			},
			{
				VisionService: protoutils.ResourceNameToProto(vision.Named("vision service 2")),
				Camera:        protoutils.ResourceNameToProto(camera.Named("camera 2")),
			},
		}

		moveOnMapNewRequest := &pb.MoveOnMapNewRequest{
			Name: testMotionServiceName.ShortName(),

			ComponentName:   protoutils.ResourceNameToProto(expectedComponentName),
			Destination:     expectedDestination,
			SlamServiceName: protoutils.ResourceNameToProto(expectedSlamName),

			MotionConfiguration: &pb.MotionConfiguration{
				AngularDegsPerSec:          &angularDegsPerSec,
				LinearMPerSec:              &linearMPerSec,
				PlanDeviationM:             &planDeviationM,
				ObstaclePollingFrequencyHz: &obstaclePollingFrequencyHz,
				PositionPollingFrequencyHz: &positionPollingFrequencyHz,
				ObstacleDetectors:          obstacleDetectorsPB,
			},
		}

		firstExecutionID := uuid.New()
		injectMS.MoveOnMapNewFunc = func(ctx context.Context, req motion.MoveOnMapReq) (motion.ExecutionID, error) {
			test.That(t, req.ComponentName, test.ShouldResemble, expectedComponentName)
			test.That(t, req.Destination, test.ShouldNotBeNil)
			test.That(t,
				spatialmath.PoseAlmostEqualEps(req.Destination, spatialmath.NewPoseFromProtobuf(expectedDestination), 1e-5),
				test.ShouldBeTrue,
			)
			test.That(t, req.SlamName, test.ShouldResemble, expectedSlamName)
			test.That(t, req.MotionCfg.AngularDegsPerSec, test.ShouldAlmostEqual, angularDegsPerSec)
			test.That(t, req.MotionCfg.LinearMPerSec, test.ShouldAlmostEqual, linearMPerSec)
			test.That(t, req.MotionCfg.PlanDeviationMM, test.ShouldAlmostEqual, planDeviationM*1000)
			test.That(t, req.MotionCfg.ObstaclePollingFreqHz, test.ShouldAlmostEqual, obstaclePollingFrequencyHz)
			test.That(t, req.MotionCfg.PositionPollingFreqHz, test.ShouldAlmostEqual, positionPollingFrequencyHz)
			test.That(t, len(req.MotionCfg.ObstacleDetectors), test.ShouldAlmostEqual, 2)
			test.That(t, req.MotionCfg.ObstacleDetectors[0].VisionServiceName, test.ShouldResemble, vision.Named("vision service 1"))
			test.That(t, req.MotionCfg.ObstacleDetectors[0].CameraName, test.ShouldResemble, camera.Named("camera 1"))
			test.That(t, req.MotionCfg.ObstacleDetectors[1].VisionServiceName, test.ShouldResemble, vision.Named("vision service 2"))
			test.That(t, req.MotionCfg.ObstacleDetectors[1].CameraName, test.ShouldResemble, camera.Named("camera 2"))
			return firstExecutionID, nil
		}

		moveOnMapNewRespose, err := server.MoveOnMapNew(context.Background(), moveOnMapNewRequest)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moveOnMapNewRespose.ExecutionId, test.ShouldEqual, firstExecutionID.String())
	})
}

func TestServerStopPlan(t *testing.T) {
	injectMS := &inject.MotionService{}
	resources := map[resource.Name]motion.Service{
		testMotionServiceName: injectMS,
	}
	server, err := newServer(resources)
	test.That(t, err, test.ShouldBeNil)

	expectedComponentName := base.Named("test-base")

	validStopPlanRequest := &pb.StopPlanRequest{
		ComponentName: protoutils.ResourceNameToProto(expectedComponentName),
		Name:          testMotionServiceName.ShortName(),
	}

	t.Run("returns error without calling StopPlan if req.Name doesn't map to a resource", func(t *testing.T) {
		stopPlanRequest := &pb.StopPlanRequest{
			ComponentName: protoutils.ResourceNameToProto(expectedComponentName),
		}

		injectMS.StopPlanFunc = func(
			ctx context.Context,
			req motion.StopPlanReq,
		) error {
			t.Log("should not be called")
			t.FailNow()
			return errors.New("should not be called")
		}

		stopPlanResponse, err := server.StopPlan(context.Background(), stopPlanRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:motion/\" not found"))
		test.That(t, stopPlanResponse, test.ShouldBeNil)
	})

	t.Run("returns error if StopPlan returns an error", func(t *testing.T) {
		errExpected := errors.New("stop error")
		injectMS.StopPlanFunc = func(
			ctx context.Context,
			req motion.StopPlanReq,
		) error {
			return errExpected
		}

		stopPlanResponse, err := server.StopPlan(context.Background(), validStopPlanRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errExpected)
		test.That(t, stopPlanResponse, test.ShouldBeNil)
	})

	t.Run("otherwise returns a success response", func(t *testing.T) {
		injectMS.StopPlanFunc = func(
			ctx context.Context,
			req motion.StopPlanReq,
		) error {
			test.That(t, req.ComponentName, test.ShouldResemble, expectedComponentName)
			return nil
		}

		stopPlanResponse, err := server.StopPlan(context.Background(), validStopPlanRequest)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, stopPlanResponse, test.ShouldResemble, &pb.StopPlanResponse{})
	})
}

func TestServerListPlanStatuses(t *testing.T) {
	injectMS := &inject.MotionService{}
	resources := map[resource.Name]motion.Service{
		testMotionServiceName: injectMS,
	}
	server, err := newServer(resources)
	test.That(t, err, test.ShouldBeNil)

	validListPlanStatusesRequest := &pb.ListPlanStatusesRequest{
		Name: testMotionServiceName.ShortName(),
	}

	t.Run("returns error without calling ListPlanStatuses if req.Name doesn't map to a resource", func(t *testing.T) {
		listPlanStatusesRequest := &pb.ListPlanStatusesRequest{}
		injectMS.ListPlanStatusesFunc = func(
			ctx context.Context,
			req motion.ListPlanStatusesReq,
		) ([]motion.PlanStatusWithID, error) {
			t.Log("should not be called")
			t.FailNow()
			return nil, errors.New("should not be called")
		}

		listPlanStatusesResponse, err := server.ListPlanStatuses(context.Background(), listPlanStatusesRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:motion/\" not found"))
		test.That(t, listPlanStatusesResponse, test.ShouldBeNil)
	})

	t.Run("returns error if ListPlanStatuses returns an error", func(t *testing.T) {
		errExpected := errors.New("ListPlanStatuses error")
		injectMS.ListPlanStatusesFunc = func(
			ctx context.Context,
			req motion.ListPlanStatusesReq,
		) ([]motion.PlanStatusWithID, error) {
			return nil, errExpected
		}

		listPlanStatusesResponse, err := server.ListPlanStatuses(context.Background(), validListPlanStatusesRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errExpected)
		test.That(t, listPlanStatusesResponse, test.ShouldBeNil)
	})

	t.Run("otherwise returns a success response", func(t *testing.T) {
		executionID := uuid.New()
		planID1 := uuid.New()
		planID2 := uuid.New()
		planID3 := uuid.New()
		planID4 := uuid.New()

		expectedComponentName := base.Named("test-base")
		failedReason := "some reason for failure"

		status1 := motion.PlanStatus{State: motion.PlanStateFailed, Timestamp: time.Now(), Reason: &failedReason}
		status2 := motion.PlanStatus{State: motion.PlanStateSucceeded, Timestamp: time.Now()}
		status3 := motion.PlanStatus{State: motion.PlanStateStopped, Timestamp: time.Now()}
		status4 := motion.PlanStatus{State: motion.PlanStateInProgress, Timestamp: time.Now()}

		pswid1 := motion.PlanStatusWithID{
			PlanID:        planID1,
			ExecutionID:   executionID,
			ComponentName: expectedComponentName,
			Status:        status1,
		}
		pswid2 := motion.PlanStatusWithID{
			PlanID:        planID2,
			ExecutionID:   executionID,
			ComponentName: expectedComponentName,
			Status:        status2,
		}
		pswid3 := motion.PlanStatusWithID{
			PlanID:        planID3,
			ExecutionID:   executionID,
			ComponentName: expectedComponentName,
			Status:        status3,
		}
		pswid4 := motion.PlanStatusWithID{
			PlanID:        planID4,
			ExecutionID:   executionID,
			ComponentName: expectedComponentName,
			Status:        status4,
		}

		injectMS.ListPlanStatusesFunc = func(
			ctx context.Context,
			req motion.ListPlanStatusesReq,
		) ([]motion.PlanStatusWithID, error) {
			return []motion.PlanStatusWithID{
				pswid1,
				pswid2,
				pswid3,
				pswid4,
			}, nil
		}

		listPlanStatusesResponse, err := server.ListPlanStatuses(context.Background(), validListPlanStatusesRequest)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(listPlanStatusesResponse.PlanStatusesWithIds), test.ShouldEqual, 4)
		test.That(t, listPlanStatusesResponse.PlanStatusesWithIds[0], test.ShouldResemble, pswid1.ToProto())
		test.That(t, listPlanStatusesResponse.PlanStatusesWithIds[1], test.ShouldResemble, pswid2.ToProto())
		test.That(t, listPlanStatusesResponse.PlanStatusesWithIds[2], test.ShouldResemble, pswid3.ToProto())
		test.That(t, listPlanStatusesResponse.PlanStatusesWithIds[3], test.ShouldResemble, pswid4.ToProto())
	})
}

func TestServerGetPlan(t *testing.T) {
	injectMS := &inject.MotionService{}
	resources := map[resource.Name]motion.Service{
		testMotionServiceName: injectMS,
	}
	server, err := newServer(resources)
	test.That(t, err, test.ShouldBeNil)

	expectedComponentName := base.Named("test-base")
	uuidID := uuid.New()
	id := uuidID.String()

	validGetPlanRequest := &pb.GetPlanRequest{
		ComponentName: protoutils.ResourceNameToProto(expectedComponentName),
		Name:          testMotionServiceName.ShortName(),
		LastPlanOnly:  false,
		ExecutionId:   &id,
	}

	t.Run("returns error without calling GetPlan if req.Name doesn't map to a resource", func(t *testing.T) {
		getPlanRequest := &pb.GetPlanRequest{}

		injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			t.Log("should not be called")
			t.FailNow()
			return nil, errors.New("should not be called")
		}

		getPlanResponse, err := server.GetPlan(context.Background(), getPlanRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:motion/\" not found"))
		test.That(t, getPlanResponse, test.ShouldBeNil)
	})

	t.Run("returns error if GetPlan returns an error", func(t *testing.T) {
		errExpected := errors.New("stop error")
		injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			return nil, errExpected
		}

		getPlanResponse, err := server.GetPlan(context.Background(), validGetPlanRequest)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errExpected)
		test.That(t, getPlanResponse, test.ShouldBeNil)
	})

	t.Run("otherwise returns a success response", func(t *testing.T) {
		executionID := uuid.New()
		planID1 := uuid.New()
		planID2 := uuid.New()

		base1 := base.Named("base1")
		steps := []motionplan.PlanStep{{base1: spatialmath.NewZeroPose()}}

		plan1 := motion.Plan{
			ID:            planID1,
			ComponentName: base1,
			ExecutionID:   executionID,
			Steps:         steps,
		}

		plan2 := motion.Plan{
			ID:            planID2,
			ComponentName: base1,
			ExecutionID:   executionID,
			Steps:         steps,
		}

		time1A := time.Now()
		time1B := time.Now()

		statusHistory1 := []motion.PlanStatus{
			{
				State:     motion.PlanStateSucceeded,
				Timestamp: time1B,
				Reason:    nil,
			},
			{
				State:     motion.PlanStateInProgress,
				Timestamp: time1A,
				Reason:    nil,
			},
		}

		time2A := time.Now()
		time2B := time.Now()

		reason := "some failed reason"
		statusHistory2 := []motion.PlanStatus{
			{
				State:     motion.PlanStateFailed,
				Timestamp: time2B,
				Reason:    &reason,
			},
			{
				State:     motion.PlanStateInProgress,
				Timestamp: time2A,
				Reason:    nil,
			},
		}

		planWithStatus2 := motion.PlanWithStatus{Plan: plan2, StatusHistory: statusHistory2}
		planWithStatus1 := motion.PlanWithStatus{Plan: plan1, StatusHistory: statusHistory1}

		injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			test.That(t, req.ComponentName, test.ShouldResemble, expectedComponentName)
			test.That(t, req.LastPlanOnly, test.ShouldResemble, validGetPlanRequest.LastPlanOnly)
			test.That(t, req.ExecutionID.String(), test.ShouldResemble, *validGetPlanRequest.ExecutionId)
			return []motion.PlanWithStatus{
				planWithStatus2,
				planWithStatus1,
			}, nil
		}

		getPlanResponse, err := server.GetPlan(context.Background(), validGetPlanRequest)
		test.That(t, err, test.ShouldBeNil)

		expectedResponse := &pb.GetPlanResponse{
			CurrentPlanWithStatus: &pb.PlanWithStatus{
				Plan:          plan2.ToProto(),
				Status:        statusHistory2[0].ToProto(),
				StatusHistory: []*pb.PlanStatus{statusHistory2[1].ToProto()},
			},
			ReplanHistory: []*pb.PlanWithStatus{planWithStatus1.ToProto()},
		}
		test.That(t, getPlanResponse, test.ShouldResemble, expectedResponse)
	})
}

func TestServerDoCommand(t *testing.T) {
	resourceMap := map[resource.Name]motion.Service{
		testMotionServiceName: &inject.MotionService{
			DoCommandFunc: testutils.EchoFunc,
		},
	}
	server, err := newServer(resourceMap)
	test.That(t, err, test.ShouldBeNil)

	cmd, err := vprotoutils.StructToStructPb(testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	doCommandRequest := &commonpb.DoCommandRequest{
		Name:    testMotionServiceName.ShortName(),
		Command: cmd,
	}
	doCommandResponse, err := server.DoCommand(context.Background(), doCommandRequest)
	test.That(t, err, test.ShouldBeNil)

	// Assert that do command response is an echoed request.
	respMap := doCommandResponse.Result.AsMap()
	test.That(t, respMap["command"], test.ShouldResemble, "test")
	test.That(t, respMap["data"], test.ShouldResemble, 500.0)
}
