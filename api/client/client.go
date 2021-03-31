package client

import (
	"bytes"
	"context"
	"image"
	"time"

	pb "go.viam.com/robotcore/proto/api/v1"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

type RobotClient struct {
	conn   *grpc.ClientConn
	client pb.RobotServiceClient
}

func NewRobotClient(ctx context.Context, address string) (*RobotClient, error) {
	// TODO(erd): address insecure
	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	client := pb.NewRobotServiceClient(conn)
	return &RobotClient{conn, client}, nil
}

func (rc *RobotClient) Close() error {
	return rc.conn.Close()
}

func (rc *RobotClient) Status(ctx context.Context) (*pb.Status, error) {
	resp, err := rc.client.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

func (rc *RobotClient) StatusStream(ctx context.Context, every time.Duration) (*StatusStream, error) {
	client, err := rc.client.StatusStream(ctx, &pb.StatusStreamRequest{
		Every: durationpb.New(every),
	})
	if err != nil {
		return nil, err
	}
	return &StatusStream{client}, nil
}

type StatusStream struct {
	client pb.RobotService_StatusStreamClient
}

func (ss *StatusStream) Next() (*pb.Status, error) {
	resp, err := ss.client.Recv()
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

func (rc *RobotClient) DoAction(ctx context.Context, name string) error {
	_, err := rc.client.DoAction(ctx, &pb.DoActionRequest{
		Name: name,
	})
	return err
}

func (rc *RobotClient) StopBase(ctx context.Context, name string) error {
	_, err := rc.client.ControlBase(ctx, &pb.ControlBaseRequest{
		Name:   name,
		Action: &pb.ControlBaseRequest_Stop{Stop: true},
	})
	return err
}

func (rc *RobotClient) MoveBase(ctx context.Context, name string, distanceMillis int64, speed float64) error {
	_, err := rc.client.ControlBase(ctx, &pb.ControlBaseRequest{
		Name: name,
		Action: &pb.ControlBaseRequest_Move{
			Move: &pb.MoveBase{
				Speed: speed,
				Option: &pb.MoveBase_StraightDistanceMillis{
					StraightDistanceMillis: distanceMillis,
				},
			},
		},
	})
	return err
}

func (rc *RobotClient) SpinBase(ctx context.Context, name string, angleDeg float64) error {
	_, err := rc.client.ControlBase(ctx, &pb.ControlBaseRequest{
		Name: name,
		Action: &pb.ControlBaseRequest_Move{
			Move: &pb.MoveBase{
				Option: &pb.MoveBase_SpinAngleDeg{
					SpinAngleDeg: angleDeg,
				},
			},
		},
	})
	return err
}

func (rc *RobotClient) MoveArmToPosition(ctx context.Context, name string, pos *pb.ArmPosition) error {
	_, err := rc.client.MoveArmToPosition(ctx, &pb.MoveArmToPositionRequest{
		Name: name,
		To:   pos,
	})
	return err
}

func (rc *RobotClient) MoveArmToJointPositions(ctx context.Context, name string, pos *pb.JointPositions) error {
	_, err := rc.client.MoveArmToJointPositions(ctx, &pb.MoveArmToJointPositionsRequest{
		Name: name,
		To:   pos,
	})
	return err
}

func (rc *RobotClient) OpenGripper(ctx context.Context, name string) error {
	_, err := rc.client.ControlGripper(ctx, &pb.ControlGripperRequest{
		Name:   name,
		Action: pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_OPEN,
	})
	return err
}

func (rc *RobotClient) GrabGripper(ctx context.Context, name string) (bool, error) {
	resp, err := rc.client.ControlGripper(ctx, &pb.ControlGripperRequest{
		Name:   name,
		Action: pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_GRAB,
	})
	if err != nil {
		return false, err
	}
	return resp.Grabbed, nil
}

func (rc *RobotClient) GoMotor(ctx context.Context, boardName, motorName string, dir pb.DirectionRelative, speed, rotations float64) error {
	_, err := rc.client.ControlBoardMotor(ctx, &pb.ControlBoardMotorRequest{
		BoardName: boardName,
		MotorName: motorName,
		Direction: dir,
		Speed:     speed,
		Rotations: rotations,
	})
	return err
}

func (rc *RobotClient) MoveServo(ctx context.Context, boardName, servoName string, angleDeg uint8) error {
	_, err := rc.client.ControlBoardServo(ctx, &pb.ControlBoardServoRequest{
		BoardName: boardName,
		ServoName: servoName,
		AngleDeg:  uint32(angleDeg),
	})
	return err
}

func (rc *RobotClient) CameraFrame(ctx context.Context, name string) (image.Image, error) {
	resp, err := rc.client.CameraFrame(ctx, &pb.CameraFrameRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(resp.Frame))
	return img, err
}
