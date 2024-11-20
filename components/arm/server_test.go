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
		capArmJointPos []referenceframe.Input
		moveOptions    arm.MoveOptions
		extraOptions   map[string]interface{}
	)

	pose1 := spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})
	positions := []float64{1., 2., 3., 1., 2., 3.}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		extraOptions = extra
		return pose1, nil
	}
	injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
		extraOptions = extra
		return referenceframe.FloatsToInputs(positions), nil
	}
	injectArm.MoveToPositionFunc = func(ctx context.Context, ap spatialmath.Pose, extra map[string]interface{}) error {
		capArmPos = ap
		extraOptions = extra
		return nil
	}
	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp []referenceframe.Input, extra map[string]interface{}) error {
		capArmJointPos = jp
		extraOptions = extra
		return nil
	}
	injectArm.MoveThroughJointPositionsFunc = func(
		ctx context.Context,
		positions [][]referenceframe.Input,
		options *arm.MoveOptions,
		extra map[string]interface{},
	) error {
		capArmJointPos = positions[len(positions)-1]
		moveOptions = *options
		extraOptions = extra
		return nil
	}
	injectArm.ModelFrameFunc = func() referenceframe.Model {
		model, err := urdf.ParseModelXMLFile(utils.ResolveFile("referenceframe/urdf/testfiles/ur5e.urdf"), "foo")
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
	positionDegs2 := &pb.JointPositions{Values: []float64{4.0, 5.0, 6.0, 4.0, 5.0, 6.0}}
	injectArm2.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return nil, errGetPoseFailed
	}
	injectArm2.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
		return nil, errGetJointsFailed
	}
	injectArm2.MoveToPositionFunc = func(ctx context.Context, ap spatialmath.Pose, extra map[string]interface{}) error {
		capArmPos = ap
		return errMoveToPositionFailed
	}

	injectArm2.MoveToJointPositionsFunc = func(ctx context.Context, jp []referenceframe.Input, extra map[string]interface{}) error {
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

		// Redefine EndPositionFunc to test nil return.
		injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
			return nil, nil
		}
		resp, err = armServer.GetEndPosition(context.Background(), &pb.GetEndPositionRequest{Name: testArmName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Pose, test.ShouldResemble, &commonpb.Pose{})
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
		test.That(t, referenceframe.JointPositionsToRadians(resp.Positions), test.ShouldResemble, positions)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "JointPositions"})

		_, err = armServer.GetJointPositions(context.Background(), &pb.GetJointPositionsRequest{Name: failArmName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGetJointsFailed.Error())

		// Redefine JointPositionsFunc to test nil return.
		//nolint: nilnil
		injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
			return nil, nil
		}
		_, err = armServer.GetJointPositions(context.Background(), &pb.GetJointPositionsRequest{Name: testArmName})
		test.That(t, err.Error(), test.ShouldResemble, referenceframe.NewIncorrectDoFError(0, len(positions)).Error())
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
		positionsRads2, err := referenceframe.InputsFromJointPositions(nil, positionDegs2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos, test.ShouldResemble, positionsRads2)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "MoveToJointPositions"})

		_, err = armServer.MoveToJointPositions(
			context.Background(),
			&pb.MoveToJointPositionsRequest{Name: failArmName, Positions: &pb.JointPositions{Values: positions}},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errMoveToJointPositionFailed.Error())
	})

	t.Run("move through joint positions", func(t *testing.T) {
		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "MoveThroughJointPositions"})
		test.That(t, err, test.ShouldBeNil)
		positionDegs3 := &pb.JointPositions{Values: []float64{1.0, 5.0, 6.0, 1.0, 5.0, 6.0}}
		positions := []*pb.JointPositions{positionDegs2, positionDegs3}
		positionRads3, err := referenceframe.InputsFromJointPositions(nil, positionDegs3)
		test.That(t, err, test.ShouldBeNil)
		expectedVelocity := 180.
		expectedMoveOptions := &pb.MoveOptions{MaxVelDegsPerSec: &expectedVelocity}
		_, err = armServer.MoveThroughJointPositions(
			context.Background(),
			&pb.MoveThroughJointPositionsRequest{Name: testArmName, Positions: positions, Options: expectedMoveOptions, Extra: ext},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos, test.ShouldResemble, positionRads3)
		test.That(t, moveOptions, test.ShouldResemble, arm.MoveOptions{MaxVelRads: utils.DegToRad(expectedVelocity)})
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "MoveThroughJointPositions"})
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
