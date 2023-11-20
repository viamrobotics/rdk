package arm_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/referenceframe/urdf"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

var (
	errGetPoseFailed             = errors.New("can't get pose")
	errGetJointsFailed           = errors.New("can't get joint positions")
	errMoveToPositionFailed      = errors.New("can't move to pose")
	errMoveToJointPositionFailed = errors.New("can't move to joint positions")
	errStopUnimplemented         = errors.New("Stop unimplemented")
	errArmUnimplemented          = errors.New("not found")
)

func newServer() (pb.ArmServiceServer, *inject.Arm, *inject.Arm, error) {
	injectArm := &inject.Arm{}
	injectArm2 := &inject.Arm{}
	arms := map[resource.Name]arm.Arm{
		arm.Named(testArmName): injectArm,
		arm.Named(failArmName): injectArm2,
	}
	armSvc, err := resource.NewAPIResourceCollection(arm.API, arms)
	if err != nil {
		return nil, nil, nil, err
	}
	return arm.NewRPCServiceServer(armSvc).(pb.ArmServiceServer), injectArm, injectArm2, nil
}

func TestServer(t *testing.T) {
	armServer, injectArm, injectArm2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var (
		capArmPos      spatialmath.Pose
		capArmJointPos *pb.JointPositions
		extraOptions   map[string]interface{}
	)

	pose1 := spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})
	positionDegs1 := &pb.JointPositions{Values: []float64{1.0, 2.0, 3.0}}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		extraOptions = extra
		return pose1, nil
	}
	injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
		extraOptions = extra
		return positionDegs1, nil
	}
	injectArm.MoveToPositionFunc = func(ctx context.Context, ap spatialmath.Pose, extra map[string]interface{}) error {
		capArmPos = ap
		extraOptions = extra
		return nil
	}

	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *pb.JointPositions, extra map[string]interface{}) error {
		capArmJointPos = jp
		extraOptions = extra
		return nil
	}
	injectArm.ModelFrameFunc = func() referenceframe.Model {
		model, err := urdf.ParseModelXMLFile(utils.ResolveFile("referenceframe/urdf/testfiles/ur5_viam.urdf"), "foo")
		if err != nil {
			return nil
		}
		return model
	}
	injectArm.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extraOptions = extra
		return nil
	}

	pose2 := &commonpb.Pose{X: 4, Y: 5, Z: 6}
	positionDegs2 := &pb.JointPositions{Values: []float64{4.0, 5.0, 6.0}}
	injectArm2.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return nil, errGetPoseFailed
	}
	injectArm2.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
		return nil, errGetJointsFailed
	}
	injectArm2.MoveToPositionFunc = func(ctx context.Context, ap spatialmath.Pose, extra map[string]interface{}) error {
		capArmPos = ap
		return errMoveToPositionFailed
	}

	injectArm2.MoveToJointPositionsFunc = func(ctx context.Context, jp *pb.JointPositions, extra map[string]interface{}) error {
		capArmJointPos = jp
		return errMoveToJointPositionFailed
	}
	injectArm2.ModelFrameFunc = func() referenceframe.Model {
		return nil
	}
	injectArm2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errStopUnimplemented
	}

	t.Run("arm position", func(t *testing.T) {
		_, err := armServer.GetEndPosition(context.Background(), &pb.GetEndPositionRequest{Name: missingArmName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errArmUnimplemented.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "EndPosition"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := armServer.GetEndPosition(context.Background(), &pb.GetEndPositionRequest{Name: testArmName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Pose.String(), test.ShouldResemble, spatialmath.PoseToProtobuf(pose1).String())

		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "EndPosition"})

		_, err = armServer.GetEndPosition(context.Background(), &pb.GetEndPositionRequest{Name: failArmName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetPoseFailed.Error())
	})

	t.Run("move to position", func(t *testing.T) {
		_, err = armServer.MoveToPosition(context.Background(), &pb.MoveToPositionRequest{Name: missingArmName, To: pose2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errArmUnimplemented.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "MoveToPosition"})
		test.That(t, err, test.ShouldBeNil)
		_, err = armServer.MoveToPosition(context.Background(), &pb.MoveToPositionRequest{Name: testArmName, To: pose2, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostCoincident(capArmPos, spatialmath.NewPoseFromProtobuf(pose2)), test.ShouldBeTrue)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "MoveToPosition"})

		_, err = armServer.MoveToPosition(context.Background(), &pb.MoveToPositionRequest{
			Name: failArmName,
			To:   spatialmath.PoseToProtobuf(pose1),
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errMoveToPositionFailed.Error())
		test.That(t, spatialmath.PoseAlmostCoincident(capArmPos, pose1), test.ShouldBeTrue)
	})

	t.Run("arm joint position", func(t *testing.T) {
		_, err := armServer.GetJointPositions(context.Background(), &pb.GetJointPositionsRequest{Name: missingArmName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errArmUnimplemented.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "JointPositions"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := armServer.GetJointPositions(context.Background(), &pb.GetJointPositionsRequest{Name: testArmName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Positions.String(), test.ShouldResemble, positionDegs1.String())
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "JointPositions"})

		_, err = armServer.GetJointPositions(context.Background(), &pb.GetJointPositionsRequest{Name: failArmName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetJointsFailed.Error())
	})

	t.Run("move to joint position", func(t *testing.T) {
		_, err = armServer.MoveToJointPositions(
			context.Background(),
			&pb.MoveToJointPositionsRequest{Name: missingArmName, Positions: positionDegs2},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errArmUnimplemented.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "MoveToJointPositions"})
		test.That(t, err, test.ShouldBeNil)
		_, err = armServer.MoveToJointPositions(
			context.Background(),
			&pb.MoveToJointPositionsRequest{Name: testArmName, Positions: positionDegs2, Extra: ext},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, positionDegs2.String())
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "MoveToJointPositions"})

		_, err = armServer.MoveToJointPositions(
			context.Background(),
			&pb.MoveToJointPositionsRequest{Name: failArmName, Positions: positionDegs1},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errMoveToJointPositionFailed.Error())
		test.That(t, capArmJointPos.String(), test.ShouldResemble, positionDegs1.String())
	})

	t.Run("get kinematics", func(t *testing.T) {
		_, err = armServer.GetKinematics(context.Background(), &commonpb.GetKinematicsRequest{Name: missingArmName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errArmUnimplemented.Error())

		kinematics, err := armServer.GetKinematics(context.Background(), &commonpb.GetKinematicsRequest{Name: testArmName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, kinematics.Format, test.ShouldResemble, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_URDF)

		kinematics, err = armServer.GetKinematics(context.Background(), &commonpb.GetKinematicsRequest{Name: failArmName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, kinematics.Format, test.ShouldResemble, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED)
	})

	t.Run("stop", func(t *testing.T) {
		_, err = armServer.Stop(context.Background(), &pb.StopRequest{Name: missingArmName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errArmUnimplemented.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "Stop"})
		test.That(t, err, test.ShouldBeNil)
		_, err = armServer.Stop(context.Background(), &pb.StopRequest{Name: testArmName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "Stop"})

		_, err = armServer.Stop(context.Background(), &pb.StopRequest{Name: failArmName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStopUnimplemented.Error())
	})
}
