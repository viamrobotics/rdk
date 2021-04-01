package client

import (
	"bytes"
	"context"
	"errors"
	"image"
	"runtime/debug"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"

	"google.golang.org/grpc"
)

var errUnimplemented = errors.New("unimplemented")

type RobotClient struct {
	conn   *grpc.ClientConn
	client pb.RobotServiceClient

	armNames     []string
	baseNames    []string
	gripperNames []string
	boardNames   []string
}

func NewRobotClient(ctx context.Context, address string) (api.Robot, error) {
	// TODO(erd): address insecure
	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	client := pb.NewRobotServiceClient(conn)
	rc := &RobotClient{
		conn:   conn,
		client: client,
	}
	if err := rc.populateNames(ctx); err != nil {
		return nil, err
	}
	return rc, nil
}

func (rc *RobotClient) Close(ctx context.Context) error {
	return rc.conn.Close()
}

func (rc *RobotClient) RemoteByName(name string) api.Robot {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rc *RobotClient) ArmByName(name string) api.Arm {
	return &armClient{rc, name}
}

func (rc *RobotClient) BaseByName(name string) api.Base {
	return &baseClient{rc, name}
}

func (rc *RobotClient) GripperByName(name string) api.Gripper {
	return &gripperClient{rc, name}
}

func (rc *RobotClient) CameraByName(name string) gostream.ImageSource {
	return &cameraClient{rc, name}
}

func (rc *RobotClient) LidarDeviceByName(name string) lidar.Device {
	// TODO(erd): converge lidar grpc client here
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rc *RobotClient) BoardByName(name string) board.Board {
	return &boardClient{rc, name}
}

func (rc *RobotClient) populateNames(ctx context.Context) error {
	status, err := rc.Status(ctx)
	if err != nil {
		return err
	}
	for name := range status.Arms {
		rc.armNames = append(rc.armNames, name)
	}
	for name := range status.Bases {
		rc.baseNames = append(rc.baseNames, name)
	}
	for name := range status.Grippers {
		rc.gripperNames = append(rc.gripperNames, name)
	}
	for name := range status.Boards {
		rc.boardNames = append(rc.boardNames, name)
	}
	return nil
}

func (rc *RobotClient) RemoteNames() []string {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rc *RobotClient) ArmNames() []string {
	// TODO(erd): copy?
	return rc.armNames
}

func (rc *RobotClient) GripperNames() []string {
	// TODO(erd): copy?
	return rc.gripperNames
}

func (rc *RobotClient) CameraNames() []string {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rc *RobotClient) LidarDeviceNames() []string {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rc *RobotClient) BaseNames() []string {
	// TODO(erd): copy?
	return rc.baseNames
}

func (rc *RobotClient) BoardNames() []string {
	// TODO(erd): copy?
	return rc.boardNames
}

func (rc *RobotClient) GetConfig(ctx context.Context) (api.Config, error) {
	debug.PrintStack()
	return api.Config{}, errUnimplemented
}

func (rc *RobotClient) Status(ctx context.Context) (*pb.Status, error) {
	resp, err := rc.client.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

func (rc *RobotClient) ProviderByModel(model string) api.Provider {
	return nil
}

func (rc *RobotClient) AddProvider(p api.Provider, c api.Component) {}

func (rc *RobotClient) Logger() golog.Logger {
	return nil
}

type baseClient struct {
	rc   *RobotClient
	name string
}

func (bc *baseClient) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	_, err := bc.rc.client.ControlBase(ctx, &pb.ControlBaseRequest{
		Name: bc.name,
		Action: &pb.ControlBaseRequest_Move{
			Move: &pb.MoveBase{
				Speed: millisPerSec,
				Option: &pb.MoveBase_StraightDistanceMillis{
					StraightDistanceMillis: int64(distanceMillis),
				},
			},
		},
	})
	return err
}

func (bc *baseClient) Spin(ctx context.Context, angleDeg float64, speed int, block bool) error {
	_, err := bc.rc.client.ControlBase(ctx, &pb.ControlBaseRequest{
		Name: bc.name,
		Action: &pb.ControlBaseRequest_Move{
			Move: &pb.MoveBase{
				Speed: float64(speed),
				Option: &pb.MoveBase_SpinAngleDeg{
					SpinAngleDeg: angleDeg,
				},
			},
		},
	})
	return err
}

func (bc *baseClient) Stop(ctx context.Context) error {
	_, err := bc.rc.client.ControlBase(ctx, &pb.ControlBaseRequest{
		Name:   bc.name,
		Action: &pb.ControlBaseRequest_Stop{Stop: true},
	})
	return err
}

func (bc *baseClient) Close(ctx context.Context) error {
	// TODO(erd): this should probably be removed from interface
	return nil
}

func (bc *baseClient) WidthMillis(ctx context.Context) (int, error) {
	debug.PrintStack()
	return 0, errUnimplemented
}

type armClient struct {
	rc   *RobotClient
	name string
}

func (ac *armClient) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	resp, err := ac.rc.client.ArmCurrentPosition(ctx, &pb.ArmCurrentPositionRequest{
		Name: ac.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Position, nil
}

func (ac *armClient) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	_, err := ac.rc.client.MoveArmToPosition(ctx, &pb.MoveArmToPositionRequest{
		Name: ac.name,
		To:   c,
	})
	return err
}

func (ac *armClient) MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error {
	_, err := ac.rc.client.MoveArmToJointPositions(ctx, &pb.MoveArmToJointPositionsRequest{
		Name: ac.name,
		To:   pos,
	})
	return err
}

func (ac *armClient) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	resp, err := ac.rc.client.ArmCurrentJointPositions(ctx, &pb.ArmCurrentJointPositionsRequest{
		Name: ac.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Positions, nil
}

func (ac *armClient) JointMoveDelta(ctx context.Context, joint int, amount float64) error {
	debug.PrintStack()
	return errUnimplemented
}

// TODO(erd): this should probably be removed from interface
func (ac *armClient) Close(ctx context.Context) {}

type gripperClient struct {
	rc   *RobotClient
	name string
}

func (gc *gripperClient) Open(ctx context.Context) error {
	_, err := gc.rc.client.ControlGripper(ctx, &pb.ControlGripperRequest{
		Name:   gc.name,
		Action: pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_OPEN,
	})
	return err
}

func (gc *gripperClient) Grab(ctx context.Context) (bool, error) {
	resp, err := gc.rc.client.ControlGripper(ctx, &pb.ControlGripperRequest{
		Name:   gc.name,
		Action: pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_GRAB,
	})
	if err != nil {
		return false, err
	}
	return resp.Grabbed, nil
}

func (gc *gripperClient) Close(ctx context.Context) error {
	// TODO(erd): this should probably be removed from interface
	return nil
}

type boardClient struct {
	rc   *RobotClient
	name string
}

func (bc *boardClient) Motor(name string) board.Motor {
	return &motorClient{
		rc:        bc.rc,
		boardName: bc.name,
		motorName: name,
	}
}

func (bc *boardClient) Servo(name string) board.Servo {
	return &servoClient{
		rc:        bc.rc,
		boardName: bc.name,
		servoName: name,
	}
}

func (bc *boardClient) AnalogReader(name string) board.AnalogReader {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (bc *boardClient) DigitalInterrupt(name string) board.DigitalInterrupt {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (bc *boardClient) Close(ctx context.Context) error {
	// TODO(erd): this should probably be removed from interface
	return nil
}

func (bc *boardClient) GetConfig(ctx context.Context) (board.Config, error) {
	debug.PrintStack()
	return board.Config{}, errUnimplemented
}

func (bc *boardClient) Status(ctx context.Context) (*pb.BoardStatus, error) {
	resp, err := bc.rc.client.BoardStatus(ctx, &pb.BoardStatusRequest{
		Name: bc.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

type motorClient struct {
	rc        *RobotClient
	boardName string
	motorName string
}

func (mc *motorClient) Force(ctx context.Context, force byte) error {
	debug.PrintStack()
	return errUnimplemented
}

func (mc *motorClient) Go(ctx context.Context, d pb.DirectionRelative, force byte) error {
	_, err := mc.rc.client.ControlBoardMotor(ctx, &pb.ControlBoardMotorRequest{
		BoardName: mc.boardName,
		MotorName: mc.motorName,
		Direction: d,
		Speed:     float64(force),
	})
	return err
}

func (mc *motorClient) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	_, err := mc.rc.client.ControlBoardMotor(ctx, &pb.ControlBoardMotorRequest{
		BoardName: mc.boardName,
		MotorName: mc.motorName,
		Direction: d,
		Speed:     rpm,
		Rotations: rotations,
	})
	return err
}

func (mc *motorClient) Position(ctx context.Context) (int64, error) {
	debug.PrintStack()
	return 0, errUnimplemented
}

func (mc *motorClient) PositionSupported(ctx context.Context) (bool, error) {
	debug.PrintStack()
	return false, errUnimplemented
}

func (mc *motorClient) Off(ctx context.Context) error {
	debug.PrintStack()
	return errUnimplemented
}

func (mc *motorClient) IsOn(ctx context.Context) (bool, error) {
	debug.PrintStack()
	return false, errUnimplemented
}

type servoClient struct {
	rc        *RobotClient
	boardName string
	servoName string
}

func (sc *servoClient) Move(ctx context.Context, angle uint8) error {
	_, err := sc.rc.client.ControlBoardServo(ctx, &pb.ControlBoardServoRequest{
		BoardName: sc.boardName,
		ServoName: sc.servoName,
		AngleDeg:  uint32(angle),
	})
	return err
}

func (sc *servoClient) Current(ctx context.Context) (uint8, error) {
	debug.PrintStack()
	return 0, errUnimplemented
}

type cameraClient struct {
	rc   *RobotClient
	name string
}

func (cc *cameraClient) Next(ctx context.Context) (image.Image, func(), error) {
	resp, err := cc.rc.client.CameraFrame(ctx, &pb.CameraFrameRequest{
		Name: cc.name,
	})
	if err != nil {
		return nil, nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(resp.Frame))
	return img, func() {}, err
}

func (cc *cameraClient) Close() error {
	// TODO(erd): this should probably be removed from interface
	return nil
}
