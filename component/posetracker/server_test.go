package posetracker_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/posetracker"
	pb "go.viam.com/rdk/proto/api/component/posetracker/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

const (
	workingPTName = "workingPT"
	failingPTName = "failingPT"
	notPTName     = "notAPT"
	bodyName      = "body1"
	bodyFrame     = "bodyFrame"
)

func newServer() (pb.PoseTrackerServiceServer, *inject.PoseTracker, *inject.PoseTracker, error) {
	injectPT1 := &inject.PoseTracker{}
	injectPT2 := &inject.PoseTracker{}

	resourceMap := map[resource.Name]interface{}{
		posetracker.Named(workingPTName): injectPT1,
		posetracker.Named(failingPTName): injectPT2,
		posetracker.Named(notPTName):     "not a pose tracker",
	}

	injectSvc, err := subtype.New(resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return posetracker.NewServer(injectSvc), injectPT1, injectPT2, nil
}

func TestGetPoses(t *testing.T) {
	ptServer, workingPT, failingPT, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	workingPT.GetPosesFunc = func(ctx context.Context, bodyNames []string) (
		posetracker.BodyToPoseInFrame, error,
	) {
		zeroPose := spatialmath.NewZeroPose()
		return posetracker.BodyToPoseInFrame{
			bodyName: referenceframe.NewPoseInFrame(bodyFrame, &zeroPose),
		}, nil
	}
	poseFailureErr := errors.New("failure to get poses")
	failingPT.GetPosesFunc = func(ctx context.Context, bodyNames []string) (
		posetracker.BodyToPoseInFrame, error,
	) {
		return nil, poseFailureErr
	}

	t.Run("get poses fails on failing pose tracker", func(t *testing.T) {
		req := pb.GetPosesRequest{
			Name: failingPTName, BodyNames: []string{bodyName},
		}
		resp, err := ptServer.GetPoses(context.Background(), &req)
		test.That(t, err, test.ShouldBeError, poseFailureErr)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("get poses fails on improperly implemented pose tracker", func(t *testing.T) {
		req := pb.GetPosesRequest{
			Name: notPTName, BodyNames: []string{bodyName},
		}
		resp, err := ptServer.GetPoses(context.Background(), &req)
		test.That(t, err, test.ShouldBeError, posetracker.NewResourceIsNotPoseTracker(notPTName))
		test.That(t, resp, test.ShouldBeNil)
	})

	req := pb.GetPosesRequest{
		Name: workingPTName, BodyNames: []string{bodyName},
	}
	req2 := pb.GetPosesRequest{
		Name: workingPTName,
	}
	resp1, err := ptServer.GetPoses(context.Background(), &req)
	test.That(t, err, test.ShouldBeNil)
	resp2, err := ptServer.GetPoses(context.Background(), &req2)
	test.That(t, err, test.ShouldBeNil)

	workingTestCases := []struct {
		testStr string
		resp    *pb.GetPosesResponse
	}{
		{
			testStr: "get poses succeeds with working pose tracker and body names passed",
			resp:    resp1,
		},
		{
			testStr: "get poses succeeds with working pose tracker but no body names passed",
			resp:    resp2,
		},
	}
	for _, tc := range workingTestCases {
		t.Run(tc.testStr, func(t *testing.T) {
			framedPoses := tc.resp.GetBodyPoses()
			poseInFrame, ok := framedPoses[bodyName]
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, poseInFrame.GetFrame(), test.ShouldEqual, bodyFrame)
			pose := poseInFrame.GetPose()
			test.That(t, pose.GetX(), test.ShouldEqual, 0)
			test.That(t, pose.GetY(), test.ShouldEqual, 0)
			test.That(t, pose.GetZ(), test.ShouldEqual, 0)
			test.That(t, pose.GetTheta(), test.ShouldEqual, 0)
			test.That(t, pose.GetOX(), test.ShouldEqual, 0)
			test.That(t, pose.GetOY(), test.ShouldEqual, 0)
			test.That(t, pose.GetOZ(), test.ShouldEqual, 1)
		})
	}
}
