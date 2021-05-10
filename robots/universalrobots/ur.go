// Package universalrobots implements the UR arm from Universal Robots.
package universalrobots

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"go.uber.org/multierr"

	"go.viam.com/robotcore/api"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

func init() {
	api.RegisterArm("ur", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (api.Arm, error) {
		return URArmConnect(ctx, config.Host, logger)
	})
}

type URArm struct {
	mu                      sync.Mutex
	conn                    net.Conn
	state                   RobotState
	runtimeError            error
	debug                   bool
	haveData                bool
	logger                  golog.Logger
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup
}

const waitBackgroundWorkersDur = 5 * time.Second

func (arm *URArm) Close() error {
	arm.cancel()

	closeConn := func() {
		if err := arm.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			arm.logger.Errorw("error closing arm connection", "error", err)
		}
	}

	// give the worker some time to close but otherwise we must close the connection
	// since net.Conns do not utilize contexts.
	waitCtx, cancel := context.WithTimeout(context.Background(), waitBackgroundWorkersDur)
	defer cancel()
	utils.PanicCapturingGo(func() {
		<-waitCtx.Done()
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			closeConn()
		}
	})

	arm.activeBackgroundWorkers.Wait()
	cancel()
	closeConn()

	return nil
}

func URArmConnect(ctx context.Context, host string, logger golog.Logger) (*URArm, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host+":30001")
	if err != nil {
		return nil, err
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	arm := &URArm{
		conn:     conn,
		debug:    false,
		haveData: false,
		logger:   logger,
		cancel:   cancel,
	}

	onData := make(chan struct{})
	var onDataOnce sync.Once
	arm.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		if err := reader(cancelCtx, conn, arm, func() {
			onDataOnce.Do(func() {
				close(onData)
			})
		}); err != nil {
			logger.Errorw("reader failed", "error", err)
		}
	}, arm.activeBackgroundWorkers.Done)

	respondTimeout := 2 * time.Second
	timer := time.NewTimer(respondTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, multierr.Combine(ctx.Err(), arm.Close())
	case <-timer.C:
		return nil, multierr.Combine(fmt.Errorf("arm failed to respond in time (%s)", respondTimeout), arm.Close())
	case <-onData:
		return arm, nil
	}
}

func (arm *URArm) setRuntimeError(re error) {
	arm.mu.Lock()
	arm.runtimeError = re
	arm.mu.Unlock()
}

func (arm *URArm) getAndResetRuntimeError() error {
	arm.mu.Lock()
	defer arm.mu.Unlock()
	re := arm.runtimeError
	arm.runtimeError = nil
	return re
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

func (arm *URArm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	if joint < 0 || joint > 5 {
		return errors.New("invalid joint")
	}

	radians := []float64{}
	state := arm.State()
	for _, j := range state.Joints {
		radians = append(radians, j.Qactual)
	}

	radians[joint] += utils.DegToRad(amountDegs)

	return arm.MoveToJointPositionRadians(ctx, radians)
}

func (arm *URArm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	return arm.MoveToJointPositionRadians(ctx, api.JointPositionsToRadians(joints))
}

func (arm *URArm) MoveToJointPositionRadians(ctx context.Context, radians []float64) error {
	if len(radians) != 6 {
		return errors.New("need 6 joints")
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

		// TODO(erh): make responsive on new message
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}
		slept += 10
	}

}

func (arm *URArm) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
	x := float64(pos.X) / 1000
	y := float64(pos.Y) / 1000
	z := float64(pos.Z) / 1000
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
		cur, err := arm.CurrentPosition(ctx)
		if err != nil {
			return err
		}
		if api.ArmPositionGridDiff(pos, cur) <= 1.5 &&
			api.ArmPositionRotationDiff(pos, cur) <= 1.0 {
			return nil
		}

		err = arm.getAndResetRuntimeError()
		if err != nil {
			return err
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
			return fmt.Errorf("can't reach position.\n want: %v\n   at: %v\n diffs: %f %f",
				pos, cur,
				api.ArmPositionGridDiff(pos, cur), api.ArmPositionRotationDiff(pos, cur))
		}
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}
	}

}

func (arm *URArm) AddToLog(msg string) error {
	// TODO(erh): check for " in msg
	cmd := fmt.Sprintf("textmsg(\"%s\")\r\n", msg)
	_, err := arm.conn.Write([]byte(cmd))
	return err
}

func reader(ctx context.Context, conn io.Reader, arm *URArm, onHaveData func()) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		sizeBuf, err := utils.ReadBytes(conn, 4)
		if err != nil {
			return err
		}

		msgSize := binary.BigEndian.Uint32(sizeBuf)
		if msgSize <= 4 || msgSize > 10000 {
			return fmt.Errorf("invalid msg size: %d", msgSize)
		}

		buf, err := utils.ReadBytes(conn, int(msgSize-4))
		if err != nil {
			return err
		}

		switch buf[0] {
		case 16:
			state, err := readRobotStateMessage(buf[1:], arm.logger)
			if err != nil {
				return err
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
			userErr, err := readURRobotMessage(buf, arm.logger)
			if err != nil {
				return err
			}
			if userErr != nil {
				arm.setRuntimeError(userErr)
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
