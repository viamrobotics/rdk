package motion_test

import (
	"context"
	"math"
	"net"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/movementsensor"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testMotionServiceName  = motion.Named("motion1")
	testMotionServiceName2 = motion.Named("motion2")
)

func TestClient(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectMS := &inject.MotionService{}
	resources := map[resource.Name]motion.Service{
		testMotionServiceName: injectMS,
	}
	svc, err := resource.NewAPIResourceCollection(motion.API, resources)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[motion.Service](motion.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, svc), test.ShouldBeNil)

	go func() {
		test.That(t, rpcServer.Serve(listener1), test.ShouldBeNil)
	}()

	defer func() {
		test.That(t, rpcServer.Stop(), test.ShouldBeNil)
	}()

	zeroPose := spatialmath.NewZeroPose()
	zeroPoseInFrame := referenceframe.NewPoseInFrame("", zeroPose)
	globeDest := geo.NewPoint(0.0, 0.0)
	gripperName := gripper.Named("fake")
	baseName := base.Named("test-base")
	gpsName := movementsensor.Named("test-gps")
	slamName := slam.Named("test-slam")

	notYetImplementedErr := errors.New("Not yet implemented")

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	t.Run("motion client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)

		test.That(t, err, test.ShouldBeNil)

		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		receivedTransforms := make(map[string]*referenceframe.LinkInFrame)
		success := true
		injectMS.MoveFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destination *referenceframe.PoseInFrame,
			worldState *referenceframe.WorldState,
			constraints *servicepb.Constraints,
			extra map[string]interface{},
		) (bool, error) {
			return success, nil
		}
		injectMS.MoveOnGlobeFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destination *geo.Point,
			heading float64,
			movementSensorName resource.Name,
			obstacles []*spatialmath.GeoObstacle,
			motionCfg *motion.MotionConfiguration,
			extra map[string]interface{},
		) (bool, error) {
			return false, errors.New("Not yet implemented")
		}
		injectMS.GetPoseFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destinationFrame string,
			supplementalTransforms []*referenceframe.LinkInFrame,
			extra map[string]interface{},
		) (*referenceframe.PoseInFrame, error) {
			for _, tf := range supplementalTransforms {
				receivedTransforms[tf.Name()] = tf
			}
			return referenceframe.NewPoseInFrame(
				destinationFrame+componentName.Name, spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})), nil
		}

		// Move
		result, err := client.Move(ctx, gripperName, zeroPoseInFrame, nil, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldEqual, success)

		// MoveOnGlobe
		globeResult, err := client.MoveOnGlobe(ctx, baseName, globeDest, math.NaN(), gpsName, nil, &motion.MotionConfiguration{}, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, notYetImplementedErr.Error())
		test.That(t, globeResult, test.ShouldEqual, false)

		// GetPose
		testPose := spatialmath.NewPose(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)
		transforms := []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame("arm1", testPose, "frame1", nil),
			referenceframe.NewLinkInFrame("frame1", testPose, "frame2", nil),
		}

		tfMap := make(map[string]*referenceframe.LinkInFrame)
		for _, tf := range transforms {
			tfMap[tf.Name()] = tf
		}
		poseResult, err := client.GetPose(context.Background(), arm.Named("arm1"), "foo", transforms, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, poseResult.Parent(), test.ShouldEqual, "fooarm1")
		test.That(t, poseResult.Pose().Point().X, test.ShouldEqual, 1)
		test.That(t, poseResult.Pose().Point().Y, test.ShouldEqual, 2)
		test.That(t, poseResult.Pose().Point().Z, test.ShouldEqual, 3)
		for name, tf := range tfMap {
			receivedTf := receivedTransforms[name]
			test.That(t, tf.Name(), test.ShouldEqual, receivedTf.Name())
			test.That(t, tf.Parent(), test.ShouldEqual, receivedTf.Parent())
			test.That(t, spatialmath.PoseAlmostEqual(tf.Pose(), receivedTf.Pose()), test.ShouldBeTrue)
		}
		test.That(t, receivedTransforms, test.ShouldNotBeNil)

		// DoCommand
		injectMS.DoCommandFunc = testutils.EchoFunc
		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	// broken
	t.Run("motion client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		passedErr := errors.New("fake move error")
		injectMS.MoveFunc = func(
			ctx context.Context,
			componentName resource.Name,
			grabPose *referenceframe.PoseInFrame,
			worldState *referenceframe.WorldState,
			constraints *servicepb.Constraints,
			extra map[string]interface{},
		) (bool, error) {
			return false, passedErr
		}
		passedErr = errors.New("fake moveonglobe error")
		injectMS.MoveOnGlobeFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destination *geo.Point,
			heading float64,
			movementSensorName resource.Name,
			obstacles []*spatialmath.GeoObstacle,
			motionCfg *motion.MotionConfiguration,
			extra map[string]interface{},
		) (bool, error) {
			return false, passedErr
		}
		passedErr = errors.New("fake GetPose error")
		injectMS.GetPoseFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destinationFrame string,
			supplementalTransform []*referenceframe.LinkInFrame,
			extra map[string]interface{},
		) (*referenceframe.PoseInFrame, error) {
			return nil, passedErr
		}

		// Move
		resp, err := client2.Move(ctx, gripperName, zeroPoseInFrame, nil, nil, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, resp, test.ShouldEqual, false)

		// MoveOnGlobe
		resp, err = client2.MoveOnGlobe(ctx, baseName, globeDest, math.NaN(), gpsName, nil, &motion.MotionConfiguration{}, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, resp, test.ShouldEqual, false)

		// GetPose
		_, err = client2.GetPose(context.Background(), arm.Named("arm1"), "foo", nil, map[string]interface{}{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("MoveOnMapNew", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)

		test.That(t, err, test.ShouldBeNil)

		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		t.Run("returns error without calling client if params can't be cast to proto", func(t *testing.T) {
			injectMS.MoveOnMapNewFunc = func(ctx context.Context,
				componentName resource.Name,
				destination spatialmath.Pose,
				slamName resource.Name,
				motionConfig *motion.MotionConfiguration,
				extra map[string]interface{}) (motion.ExecutionID, error) {
				t.Log("should not be called")
				t.FailNow()
				return uuid.Nil, errors.New("should not be reached")
			}

			// nil destination is can't be converted to proto
			executionID, err := client.MoveOnMapNew(ctx, baseName, spatialmath.NewZeroPose(), slamName, &motion.MotionConfiguration{}, nil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err, test.ShouldBeError, errors.New("must provide a destination"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns error if client returns error", func(t *testing.T) {
			errExpected := errors.New("some client error")
			injectMS.MoveOnMapNewFunc = func(ctx context.Context,
				componentName resource.Name,
				destination spatialmath.Pose,
				slamName resource.Name,
				motionConfig *motion.MotionConfiguration,
				extra map[string]interface{}) (motion.ExecutionID, error) {
				return uuid.Nil, errExpected
			}

			executionID, err := client.MoveOnMapNew(ctx, baseName, spatialmath.NewZeroPose(), slamName, &motion.MotionConfiguration{}, nil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, errExpected.Error())
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("otherwise returns success with an executionID", func(t *testing.T) {
			expectedExecutionID := uuid.New()
			injectMS.MoveOnMapNewFunc = func(ctx context.Context,
				componentName resource.Name,
				destination spatialmath.Pose,
				slamName resource.Name,
				motionConfig *motion.MotionConfiguration,
				extra map[string]interface{}) (motion.ExecutionID, error) {
				return expectedExecutionID, nil
			}

			executionID, err := client.MoveOnMapNew(ctx, baseName, spatialmath.NewZeroPose(), slamName, &motion.MotionConfiguration{}, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, executionID, test.ShouldEqual, expectedExecutionID)
		})

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("MoveOnGlobeNew", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)

		test.That(t, err, test.ShouldBeNil)

		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		t.Run("returns error without calling client if params can't be cast to proto", func(t *testing.T) {
			injectMS.MoveOnGlobeNewFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
				t.Log("should not be called")
				t.FailNow()
				return uuid.Nil, errors.New("should not be reached")
			}

			req := motion.MoveOnGlobeReq{
				ComponentName:      baseName,
				Heading:            math.NaN(),
				MovementSensorName: gpsName,
				MotionCfg:          &motion.MotionConfiguration{},
			}
			// nil destination is can't be converted to proto
			executionID, err := client.MoveOnGlobeNew(ctx, req)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err, test.ShouldBeError, errors.New("must provide a destination"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns error if client returns error", func(t *testing.T) {
			errExpected := errors.New("some client error")
			injectMS.MoveOnGlobeNewFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
				return uuid.Nil, errExpected
			}

			req := motion.MoveOnGlobeReq{
				ComponentName:      baseName,
				Destination:        globeDest,
				Heading:            math.NaN(),
				MovementSensorName: gpsName,
				MotionCfg:          &motion.MotionConfiguration{},
			}
			executionID, err := client.MoveOnGlobeNew(ctx, req)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, errExpected.Error())
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("otherwise returns success with an executionID", func(t *testing.T) {
			expectedExecutionID := uuid.New()
			injectMS.MoveOnGlobeNewFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
				return expectedExecutionID, nil
			}

			req := motion.MoveOnGlobeReq{
				ComponentName:      baseName,
				Destination:        globeDest,
				Heading:            math.NaN(),
				MovementSensorName: gpsName,
				MotionCfg:          &motion.MotionConfiguration{},
			}
			executionID, err := client.MoveOnGlobeNew(ctx, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, executionID, test.ShouldEqual, expectedExecutionID)
		})

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("StopPlan", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)

		test.That(t, err, test.ShouldBeNil)

		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		t.Run("returns error if client returns error", func(t *testing.T) {
			errExpected := errors.New("some client error")
			injectMS.StopPlanFunc = func(
				ctx context.Context,
				req motion.StopPlanReq,
			) error {
				return errExpected
			}

			req := motion.StopPlanReq{ComponentName: base.Named("mybase")}
			err := client.StopPlan(ctx, req)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, errExpected.Error())
		})

		t.Run("otherwise returns nil", func(t *testing.T) {
			injectMS.StopPlanFunc = func(
				ctx context.Context,
				req motion.StopPlanReq,
			) error {
				return nil
			}

			req := motion.StopPlanReq{ComponentName: base.Named("mybase")}
			err := client.StopPlan(ctx, req)
			test.That(t, err, test.ShouldBeNil)
		})

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("ListPlanStatuses", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		t.Run("returns error if client returns error", func(t *testing.T) {
			errExpected := errors.New("some client error")
			injectMS.ListPlanStatusesFunc = func(
				ctx context.Context,
				req motion.ListPlanStatusesReq,
			) ([]motion.PlanStatusWithID, error) {
				return nil, errExpected
			}
			req := motion.ListPlanStatusesReq{}
			resp, err := client.ListPlanStatuses(ctx, req)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, errExpected.Error())
			test.That(t, resp, test.ShouldBeEmpty)
		})

		t.Run("otherwise returns a slice of PlanStautsWithID", func(t *testing.T) {
			planID := uuid.New()

			executionID := uuid.New()

			status := motion.PlanStatus{State: motion.PlanStateInProgress, Timestamp: time.Now().UTC(), Reason: nil}

			expectedResp := []motion.PlanStatusWithID{
				{PlanID: planID, ComponentName: base.Named("mybase"), ExecutionID: executionID, Status: status},
			}

			injectMS.ListPlanStatusesFunc = func(
				ctx context.Context,
				req motion.ListPlanStatusesReq,
			) ([]motion.PlanStatusWithID, error) {
				return expectedResp, nil
			}

			req := motion.ListPlanStatusesReq{}
			resp, err := client.ListPlanStatuses(ctx, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, resp, test.ShouldResemble, expectedResp)
		})

		t.Run("supports returning multiple PlanStautsWithID", func(t *testing.T) {
			planIDA := uuid.New()

			executionIDA := uuid.New()
			test.That(t, err, test.ShouldBeNil)

			statusA := motion.PlanStatus{State: motion.PlanStateInProgress, Timestamp: time.Now().UTC(), Reason: nil}

			planIDB := uuid.New()

			executionIDB := uuid.New()

			reason := "failed reason"
			statusB := motion.PlanStatus{State: motion.PlanStateInProgress, Timestamp: time.Now().UTC(), Reason: &reason}

			expectedResp := []motion.PlanStatusWithID{
				{PlanID: planIDA, ComponentName: base.Named("mybase"), ExecutionID: executionIDA, Status: statusA},
				{PlanID: planIDB, ComponentName: base.Named("mybase"), ExecutionID: executionIDB, Status: statusB},
			}

			injectMS.ListPlanStatusesFunc = func(
				ctx context.Context,
				req motion.ListPlanStatusesReq,
			) ([]motion.PlanStatusWithID, error) {
				return expectedResp, nil
			}

			req := motion.ListPlanStatusesReq{}
			resp, err := client.ListPlanStatuses(ctx, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, resp, test.ShouldResemble, expectedResp)
		})

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("PlanHistory", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		t.Run("returns error if client returns error", func(t *testing.T) {
			errExpected := errors.New("some client error")
			injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
				return nil, errExpected
			}

			req := motion.PlanHistoryReq{ComponentName: base.Named("mybase")}
			resp, err := client.PlanHistory(ctx, req)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, errExpected.Error())
			test.That(t, resp, test.ShouldBeEmpty)
		})

		t.Run("otherwise returns a slice of PlanWithStatus", func(t *testing.T) {
			steps := []motion.PlanStep{
				{base.Named("mybase"): zeroPose},
			}
			reason := "some reason"
			id := uuid.New()
			executionID := uuid.New()

			timeA := time.Now().UTC()
			timeB := time.Now().UTC()

			plan := motion.Plan{
				ID:            id,
				ComponentName: base.Named("mybase"),
				ExecutionID:   executionID,
				Steps:         steps,
			}
			statusHistory := []motion.PlanStatus{
				{motion.PlanStateFailed, timeB, &reason},
				{motion.PlanStateInProgress, timeA, nil},
			}
			expectedResp := []motion.PlanWithStatus{{Plan: plan, StatusHistory: statusHistory}}
			injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
				return expectedResp, nil
			}

			req := motion.PlanHistoryReq{ComponentName: base.Named("mybase")}
			resp, err := client.PlanHistory(ctx, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, resp, test.ShouldResemble, expectedResp)
		})

		t.Run("supports returning a slice of PlanWithStatus with more than one plan", func(t *testing.T) {
			steps := []motion.PlanStep{{base.Named("mybase"): zeroPose}}
			reason := "some reason"

			idA := uuid.New()
			test.That(t, err, test.ShouldBeNil)

			executionID := uuid.New()
			test.That(t, err, test.ShouldBeNil)

			timeAA := time.Now().UTC()
			timeAB := time.Now().UTC()

			planA := motion.Plan{
				ID:            idA,
				ComponentName: base.Named("mybase"),
				ExecutionID:   executionID,
				Steps:         steps,
			}
			statusHistoryA := []motion.PlanStatus{
				{motion.PlanStateFailed, timeAB, &reason},
				{motion.PlanStateInProgress, timeAA, nil},
			}

			idB := uuid.New()
			test.That(t, err, test.ShouldBeNil)
			timeBA := time.Now().UTC()
			planB := motion.Plan{
				ID:            idB,
				ComponentName: base.Named("mybase"),
				ExecutionID:   executionID,
				Steps:         steps,
			}

			statusHistoryB := []motion.PlanStatus{
				{motion.PlanStateInProgress, timeBA, nil},
			}

			expectedResp := []motion.PlanWithStatus{
				{Plan: planB, StatusHistory: statusHistoryB},
				{Plan: planA, StatusHistory: statusHistoryA},
			}

			injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
				return expectedResp, nil
			}

			req := motion.PlanHistoryReq{ComponentName: base.Named("mybase")}
			resp, err := client.PlanHistory(ctx, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, resp, test.ShouldResemble, expectedResp)
		})

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
