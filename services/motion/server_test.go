package motion_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/gripper"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/motion/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func newServer(omMap map[resource.Name]interface{}) (pb.MotionServiceServer, error) {
	omSvc, err := subtype.New(omMap)
	if err != nil {
		return nil, err
	}
	return motion.NewServer(omSvc), nil
}

func TestServerMove(t *testing.T) {
	grabRequest := &pb.MoveRequest{
		ComponentName: protoutils.ResourceNameToProto(gripper.Named("fake")),
		Destination:   referenceframe.PoseInFrameToProtobuf(referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())),
	}

	omMap := map[resource.Name]interface{}{}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:motion\" not found"))

	// set up the robot with something that is not an motion service
	omMap = map[resource.Name]interface{}{motion.Name: "not motion"}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("motion.Service", "string"))

	// error
	injectMS := &inject.MotionService{}
	omMap = map[resource.Name]interface{}{
		motion.Name: injectMS,
	}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	passedErr := errors.New("fake move error")
	injectMS.MoveFunc = func(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *commonpb.WorldState,
	) (bool, error) {
		return false, passedErr
	}

	_, err = server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, passedErr)

	// returns response
	injectMS.MoveFunc = func(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *commonpb.WorldState,
	) (bool, error) {
		return true, nil
	}
	resp, err := server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSuccess(), test.ShouldBeTrue)
}
