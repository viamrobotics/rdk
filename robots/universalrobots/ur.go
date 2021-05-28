// Package universalrobots implements the UR arm from Universal Robots.
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

	"github.com/go-errors/errors"

	"go.uber.org/multierr"

	"go.viam.com/core/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics/kinmath"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
)

func init() {
	registry.RegisterArm("ur", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		return URArmConnect(ctx, config.Host, logger)
	})
}

// URArm TODO
type URArm struct {
	mu                      *sync.Mutex
	conn                    net.Conn
	state                   RobotState
	runtimeError            error
	debug                   bool
	haveData                bool
	logger                  golog.Logger
	cancel                  func()
	activeBackgroundWorkers *sync.WaitGroup
}

const waitBackgroundWorkersDur = 5 * time.Second

// Close TODO
func (ua *URArm) Close() error {
	ua.cancel()

	closeConn := func() {
		if err := ua.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			ua.logger.Errorw("error closing arm connection", "error", err)
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

	ua.activeBackgroundWorkers.Wait()
	cancel()
	closeConn()

	return nil
}

// URArmConnect TODO
func URArmConnect(ctx context.Context, host string, logger golog.Logger) (*URArm, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host+":30001")
	if err != nil {
		return nil, err
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	arm := &URArm{
		mu:                      &sync.Mutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		conn:                    conn,
		debug:                   false,
		haveData:                false,
		logger:                  logger,
		cancel:                  cancel,
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
		return nil, multierr.Combine(errors.Errorf("arm failed to respond in time (%s)", respondTimeout), arm.Close())
	case <-onData:
		return arm, nil
	}
}

func (ua *URArm) setRuntimeError(re error) {
	ua.mu.Lock()
	ua.runtimeError = re
	ua.mu.Unlock()
}

func (ua *URArm) getAndResetRuntimeError() error {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	re := ua.runtimeError
	ua.runtimeError = nil
	return re
}

func (ua *URArm) setState(state RobotState) {
	ua.mu.Lock()
	ua.state = state
	ua.mu.Unlock()
}

// State TODO
func (ua *URArm) State() RobotState {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	return ua.state
}

// CurrentJointPositions TODO
func (ua *URArm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	radians := []float64{}
	state := ua.State()
	for _, j := range state.Joints {
		radians = append(radians, j.Qactual)
	}
	return arm.JointPositionsFromRadians(radians), nil
}

// CurrentPosition TODO
func (ua *URArm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	s := ua.State().CartesianInfo
	// The UR5 arm does not use Euler angles. It returns its orientation in R3 angle axis form, we convert to R4.
	r3aa := kinmath.R3AA{s.Rx, s.Ry, s.Rz}
	r4aa := r3aa.ToR4()
	return arm.NewPositionFromMetersAndAngleAxis(s.X, s.Y, s.Z, r4aa.Theta, r4aa.RX, r4aa.RY, r4aa.RZ), nil
}

// JointMoveDelta TODO
func (ua *URArm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	if joint < 0 || joint > 5 {
		return errors.New("invalid joint")
	}

	radians := []float64{}
	state := ua.State()
	for _, j := range state.Joints {
		radians = append(radians, j.Qactual)
	}

	radians[joint] += utils.DegToRad(amountDegs)

	return ua.MoveToJointPositionRadians(ctx, radians)
}

// MoveToJointPositions TODO
func (ua *URArm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	return ua.MoveToJointPositionRadians(ctx, arm.JointPositionsToRadians(joints))
}

// MoveToJointPositionRadians TODO
func (ua *URArm) MoveToJointPositionRadians(ctx context.Context, radians []float64) error {
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
	_, err := ua.conn.Write([]byte(cmd))
	if err != nil {
		return err
	}

	retried := false
	slept := 0
	for {
		good := true
		state := ua.State()
		for idx, r := range radians {
			if math.Round(r*100) != math.Round(state.Joints[idx].Qactual*100) {
				good = false
			}
		}

		if good {
			return nil
		}

		if slept > 5000 && !retried {
			_, err := ua.conn.Write([]byte(cmd))
			if err != nil {
				return err
			}
			retried = true
		}

		if slept > 10000 {
			return errors.Errorf("can't reach joint position.\n want: %f %f %f %f %f %f\n   at: %f %f %f %f %f %f",
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

// MoveToPosition TODO
func (ua *URArm) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
	x := pos.X / 1000
	y := pos.Y / 1000
	z := pos.Z / 1000
	// UR5 arm takes R3 angle axis as input
	rx := pos.RX * pos.Theta
	ry := pos.RY * pos.Theta
	rz := pos.RZ * pos.Theta

	cmd := fmt.Sprintf("movej(get_inverse_kin(p[%f,%f,%f,%f,%f,%f]), a=1.4, v=4, r=0)\r\n", x, y, z, rx, ry, rz)

	_, err := ua.conn.Write([]byte(cmd))
	if err != nil {
		return err
	}

	retried := false

	slept := 0
	for {
		cur, err := ua.CurrentPosition(ctx)
		if err != nil {
			return err
		}
		if arm.PositionGridDiff(pos, cur) <= 1.5 &&
			arm.PositionRotationDiff(pos, cur) <= 1.0 {
			return nil
		}

		err = ua.getAndResetRuntimeError()
		if err != nil {
			return err
		}

		slept = slept + 10

		if slept > 5000 && !retried {
			_, err := ua.conn.Write([]byte(cmd))
			if err != nil {
				return err
			}
			retried = true
		}

		if slept > 10000 {
			return errors.Errorf("can't reach position.\n want: %v\n   at: %v\n diffs: %f %f",
				pos, cur,
				arm.PositionGridDiff(pos, cur), arm.PositionRotationDiff(pos, cur))
		}
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}
	}

}

// AddToLog TODO
func (ua *URArm) AddToLog(msg string) error {
	// TODO(erh): check for " in msg
	cmd := fmt.Sprintf("textmsg(\"%s\")\r\n", msg)
	_, err := ua.conn.Write([]byte(cmd))
	return err
}

func reader(ctx context.Context, conn io.Reader, ua *URArm, onHaveData func()) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		sizeBuf, err := utils.ReadBytes(ctx, conn, 4)
		if err != nil {
			return err
		}

		msgSize := binary.BigEndian.Uint32(sizeBuf)
		if msgSize <= 4 || msgSize > 10000 {
			return errors.Errorf("invalid msg size: %d", msgSize)
		}

		buf, err := utils.ReadBytes(ctx, conn, int(msgSize-4))
		if err != nil {
			return err
		}

		switch buf[0] {
		case 16:
			state, err := readRobotStateMessage(buf[1:], ua.logger)
			if err != nil {
				return err
			}
			ua.setState(state)
			onHaveData()
			if ua.debug {
				ua.logger.Debugf("isOn: %v stopped: %v joints: %f %f %f %f %f %f cartesian: %f %f %f %f %f %f\n",
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
			userErr, err := readURRobotMessage(buf, ua.logger)
			if err != nil {
				return err
			}
			if userErr != nil {
				ua.setRuntimeError(userErr)
			}
		case 5: // MODBUS_INFO_MESSAGE
			data := binary.BigEndian.Uint32(buf[1:])
			if data != 0 {
				ua.logger.Debugf("got unexpected MODBUS_INFO_MESSAGE %d\n", data)
			}
		case 23: // SAFETY_SETUP_BROADCAST_MESSAGE
		case 24: // SAFETY_COMPLIANCE_TOLERANCES_MESSAGE
		case 25: // PROGRAM_STATE_MESSAGE
			if len(buf) != 12 {
				ua.logger.Debug("got bad PROGRAM_STATE_MESSAGE ??")
			} else {
				a := binary.BigEndian.Uint32(buf[1:])
				b := buf[9]
				c := buf[10]
				d := buf[11]
				if a != 4294967295 || b != 1 || c != 0 || d != 0 {
					ua.logger.Debugf("got unknown PROGRAM_STATE_MESSAGE %v %v %v %v\n", a, b, c, d)
				}
			}
		default:
			ua.logger.Debugf("ur: unknown messageType: %v size: %d %v\n", buf[0], len(buf), buf)
		}
	}
}
