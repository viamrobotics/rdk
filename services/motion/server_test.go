package motion_test

import (
	"context"
	"errors"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	vprotoutils "go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
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
		Name:          testMotionServiceName,
		ComponentName: protoutils.ResourceNameToProto(gripper.Named("fake")),
		Destination:   referenceframe.PoseInFrameToProtobuf(referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())),
	}

	omMap := map[resource.Name]interface{}{}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:motion/motion1\" not found"))

	// set up the robot with something that is not an motion service
	omMap = map[resource.Name]interface{}{motion.Named(testMotionServiceName): "not motion"}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, motion.NewUnimplementedInterfaceError("string"))

	// error
	injectMS := &inject.MotionService{}
	omMap = map[resource.Name]interface{}{
		motion.Named(testMotionServiceName): injectMS,
	}
	server, err = newServer(omMap)
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
	omMap = map[resource.Name]interface{}{
		motion.Named(testMotionServiceName):  injectMS,
		motion.Named(testMotionServiceName2): injectMS,
	}
	server, _ = newServer(omMap)
	injectMS.MoveFunc = successfulMoveFunc
	resp, err = server.Move(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSuccess(), test.ShouldBeTrue)
	grabRequest2 := &pb.MoveRequest{
		Name:          testMotionServiceName2,
		ComponentName: protoutils.ResourceNameToProto(gripper.Named("fake")),
		Destination:   referenceframe.PoseInFrameToProtobuf(referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())),
	}
	resp, err = server.Move(context.Background(), grabRequest2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSuccess(), test.ShouldBeTrue)
}

func TestServerDoCommand(t *testing.T) {
	resourceMap := map[resource.Name]interface{}{
		motion.Named(testMotionServiceName): &inject.MotionService{
			DoCommandFunc: generic.EchoFunc,
		},
	}
	server, err := newServer(resourceMap)
	test.That(t, err, test.ShouldBeNil)

	cmd, err := vprotoutils.StructToStructPb(generic.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	doCommandRequest := &commonpb.DoCommandRequest{
		Name:    testMotionServiceName,
		Command: cmd,
	}
	doCommandResponse, err := server.DoCommand(context.Background(), doCommandRequest)
	test.That(t, err, test.ShouldBeNil)

	// Assert that do command response is an echoed request.
	respMap := doCommandResponse.Result.AsMap()
	test.That(t, respMap["command"], test.ShouldResemble, "test")
	test.That(t, respMap["data"], test.ShouldResemble, 500.0)
}
