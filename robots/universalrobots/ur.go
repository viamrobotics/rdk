package universalrobots

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"go.viam.com/robotcore/api"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

func init() {
	api.RegisterArm("ur", func(ctx context.Context, r api.Robot, config api.Component, logger golog.Logger) (api.Arm, error) {
		return URArmConnect(config.Host, logger)
	})
}

type URArm struct {
	mu       sync.Mutex
	conn     net.Conn
	state    RobotState
	debug    bool
	haveData bool
	logger   golog.Logger
}

func (arm *URArm) Close() error {
	// TODO(erh): stop thread
	// TODO(erh): close socket
	return nil
}

func URArmConnect(host string, logger golog.Logger) (*URArm, error) {
	conn, err := net.Dial("tcp", host+":30001")
	if err != nil {
		return nil, err
	}

	arm := &URArm{conn: conn, debug: false, haveData: false, logger: logger}

	onData := make(chan struct{})
	var onDataOnce sync.Once
	go reader(conn, arm, func() {
		onDataOnce.Do(func() {
			close(onData)
		})
	}) // TODO(erh): how to shutdown

	respondTimeout := 2 * time.Second
	timer := time.NewTimer(respondTimeout)
	select {
	case <-onData:
		return arm, nil
	case <-timer.C:
		return nil, fmt.Errorf("arm failed to respond in time (%s)", respondTimeout)
	}
}

func (arm *URArm) setState(state RobotState) {
	arm.mu.Lock()
	arm.state = state
	arm.mu.Unlock()
}

func (arm *URArm) State() RobotState {
	arm.mu.Lock()
	defer arm.mu.Unlock()
	return arm.state
}

func (arm *URArm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	radians := []float64{}
	state := arm.State()
	for _, j := range state.Joints {
		radians = append(radians, j.Qactual)
	}
	return api.JointPositionsFromRadians(radians), nil
}

func (arm *URArm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	s := arm.State().CartesianInfo
	return api.NewPositionFromMetersAndRadians(s.X, s.Y, s.Z, s.Rx, s.Ry, s.Rz), nil
}

func (arm *URArm) JointMoveDelta(ctx context.Context, joint int, amount float64) error {
	if joint < 0 || joint > 5 {
		return fmt.Errorf("invalid joint")
	}

	radians := []float64{}
	state := arm.State()
	for _, j := range state.Joints {
		radians = append(radians, j.Qactual)
	}

	radians[joint] += amount

	return arm.MoveToJointPositionRadians(ctx, radians)
}

func (arm *URArm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	return arm.MoveToJointPositionRadians(ctx, api.JointPositionsToRadians(joints))
}

func (arm *URArm) MoveToJointPositionRadians(ctx context.Context, radians []float64) error {
	if len(radians) != 6 {
		return fmt.Errorf("need 6 joints")
	}

	cmd := fmt.Sprintf("movej([%f,%f,%f,%f,%f,%f], a=5, v=4, r=0)\r\n",
		radians[0],
		radians[1],
		radians[2],
		radians[3],
		radians[4],
		radians[5])
	_, err := arm.conn.Write([]byte(cmd))
	if err != nil {
		return err
	}

	retried := false
	slept := 0
	for {
		good := true
		state := arm.State()
		for idx, r := range radians {
			if math.Round(r*100) != math.Round(state.Joints[idx].Qactual*100) {
				//arm.logger.Debugf("joint %d want: %f have: %f\n", idx, r, arm.State.Joints[idx].Qactual)
				good = false
			}
		}

		if good {
			return nil
		}

		if slept > 5000 && !retried {
			_, err := arm.conn.Write([]byte(cmd))
			if err != nil {
				return err
			}
			retried = true
		}

		if slept > 10000 {
			return fmt.Errorf("can't reach joint position.\n want: %f %f %f %f %f %f\n   at: %f %f %f %f %f %f",
				radians[0], radians[1], radians[2], radians[3], radians[4], radians[5],
				state.Joints[0].Qactual,
				state.Joints[1].Qactual,
				state.Joints[2].Qactual,
				state.Joints[3].Qactual,
				state.Joints[4].Qactual,
				state.Joints[5].Qactual,
			)
		}

		time.Sleep(10 * time.Millisecond) // TODO(erh): make responsive on new message
		slept += 10
	}

}

func (arm *URArm) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
	x := pos.X
	y := pos.Y
	z := pos.Z
	rx := utils.DegToRad(pos.RX)
	ry := utils.DegToRad(pos.RY)
	rz := utils.DegToRad(pos.RZ)

	cmd := fmt.Sprintf("movej(get_inverse_kin(p[%f,%f,%f,%f,%f,%f]), a=1.4, v=4, r=0)\r\n", x, y, z, rx, ry, rz)

	_, err := arm.conn.Write([]byte(cmd))
	if err != nil {
		return err
	}

	retried := false

	slept := 0
	for {
		state := arm.State()
		if math.Round(x*100) == math.Round(state.CartesianInfo.X*100) &&
			math.Round(y*100) == math.Round(state.CartesianInfo.Y*100) &&
			math.Round(z*100) == math.Round(state.CartesianInfo.Z*100) &&
			math.Round(rx*20) == math.Round(state.CartesianInfo.Rx*20) &&
			math.Round(ry*20) == math.Round(state.CartesianInfo.Ry*20) &&
			math.Round(rz*20) == math.Round(state.CartesianInfo.Rz*20) {
			return nil
		}
		slept = slept + 10

		if slept > 5000 && !retried {
			_, err := arm.conn.Write([]byte(cmd))
			if err != nil {
				return err
			}
			retried = true
		}

		if slept > 10000 {
			return fmt.Errorf("can't reach position.\n want: %f %f %f %f %f %f\n   at: %f %f %f %f %f %f",
				x, y, z, rx, ry, rz,
				state.CartesianInfo.X, state.CartesianInfo.Y, state.CartesianInfo.Z,
				state.CartesianInfo.Rx, state.CartesianInfo.Ry, state.CartesianInfo.Rz)

		}
		time.Sleep(10 * time.Millisecond)

	}

}

func (arm *URArm) AddToLog(msg string) error {
	// TODO(erh): check for " in msg
	cmd := fmt.Sprintf("textmsg(\"%s\")\r\n", msg)
	_, err := arm.conn.Write([]byte(cmd))
	return err
}

func reader(conn io.Reader, arm *URArm, onHaveData func()) {
	for {
		buf := make([]byte, 4)
		n, err := conn.Read(buf)
		if err == nil && n != 4 {
			err = fmt.Errorf("didn't read full int, got: %d", n)
		}
		if err != nil {
			panic(err)
		}

		//msgSize := binary.BigEndian.Uint32(buf)
		msgSize := binary.BigEndian.Uint32(buf)

		restToRead := msgSize - 4
		buf = make([]byte, restToRead)
		n, err = conn.Read(buf)
		if err == nil && n != int(restToRead) {
			err = fmt.Errorf("didn't read full msg, got: %d instead of %d", n, restToRead)
		}
		if err != nil {
			panic(err)
		}

		switch buf[0] {
		case 16:
			state, err := readRobotStateMessage(buf[1:], arm.logger)
			if err != nil {
				panic(err)
			}
			arm.setState(state)
			onHaveData()
			if arm.debug {
				arm.logger.Debugf("isOn: %v stopped: %v joints: %f %f %f %f %f %f cartesian: %f %f %f %f %f %f\n",
					state.RobotModeData.IsRobotPowerOn,
					state.RobotModeData.IsEmergencyStopped || state.RobotModeData.IsProtectiveStopped,
					state.Joints[0].AngleDegrees(),
					state.Joints[1].AngleDegrees(),
					state.Joints[2].AngleDegrees(),
					state.Joints[3].AngleDegrees(),
					state.Joints[4].AngleDegrees(),
					state.Joints[5].AngleDegrees(),
					state.CartesianInfo.X,
					state.CartesianInfo.Y,
					state.CartesianInfo.Z,
					state.CartesianInfo.Rx,
					state.CartesianInfo.Ry,
					state.CartesianInfo.Rz)
			}
		case 20:
			err := readURRobotMessage(buf, arm.logger)
			if err != nil {
				panic(err)
			}
		case 5: // MODBUS_INFO_MESSAGE
			data := binary.BigEndian.Uint32(buf[1:])
			if data != 0 {
				arm.logger.Debugf("got unexpected MODBUS_INFO_MESSAGE %d\n", data)
			}

		case 23: // SAFETY_SETUP_BROADCAST_MESSAGE
			break
		case 24: // SAFETY_COMPLIANCE_TOLERANCES_MESSAGE
			break
		case 25: // PROGRAM_STATE_MESSAGE
			if len(buf) != 12 {
				arm.logger.Debug("got bad PROGRAM_STATE_MESSAGE ??")
			} else {
				a := binary.BigEndian.Uint32(buf[1:])
				b := buf[9]
				c := buf[10]
				d := buf[11]
				if a != 4294967295 || b != 1 || c != 0 || d != 0 {
					arm.logger.Debugf("got unknown PROGRAM_STATE_MESSAGE %v %v %v %v\n", a, b, c, d)
				}
			}
		default:
			arm.logger.Debugf("ur: unknown messageType: %v size: %d %v\n", buf[0], len(buf), buf)
		}
	}
}
