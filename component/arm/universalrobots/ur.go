// Package universalrobots implements the UR arm from Universal Robots.
package universalrobots

import (
	"context"
	// for embedding model file.
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const (
	modelname = "ur"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	Speed float64 `json:"speed"`
	Host  string  `json:"host"`
}

//go:embed ur5e.json
var ur5modeljson []byte

//go:embed ur5e_DH.json
var ur5DHmodeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return URArmConnect(ctx, r, config, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

// Ur5eModel() returns the kinematics model of the xArm, also has all Frame information.
func ur5eModel() (referenceframe.Model, error) {
	return referenceframe.UnmarshalModelJSON(ur5modeljson, "")
}

// URArm TODO.
type URArm struct {
	generic.Unimplemented
	io.Closer
	mu                      *sync.Mutex
	muMove                  sync.Mutex
	conn                    net.Conn
	speed                   float64
	state                   RobotState
	runtimeError            error
	debug                   bool
	haveData                bool
	logger                  golog.Logger
	cancel                  func()
	activeBackgroundWorkers *sync.WaitGroup
	model                   referenceframe.Model
	opMgr                   operation.SingleOperationManager
	robot                   robot.Robot
}

const waitBackgroundWorkersDur = 5 * time.Second

// Close TODO.
func (ua *URArm) Close(ctx context.Context) error {
	ua.cancel()

	closeConn := func() {
		if err := ua.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			ua.logger.Errorw("error closing arm connection", "error", err)
		}
	}

	// give the worker some time to close but otherwise we must close the connection
	// since net.Conns do not utilize contexts.
	waitCtx, cancel := context.WithTimeout(ctx, waitBackgroundWorkersDur)
	defer cancel()
	goutils.PanicCapturingGo(func() {
		<-waitCtx.Done()
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			closeConn()
		}
	})

	ua.activeBackgroundWorkers.Wait()
	cancel()
	closeConn()
	return waitCtx.Err()
}

// URArmConnect TODO.
func URArmConnect(ctx context.Context, r robot.Robot, cfg config.Component, logger golog.Logger) (arm.LocalArm, error) {
	speed := cfg.ConvertedAttributes.(*AttrConfig).Speed
	host := cfg.ConvertedAttributes.(*AttrConfig).Host
	if speed > 1 || speed < .1 {
		return nil, errors.New("speed for universalrobots has to be between .1 and 1")
	}

	model, err := ur5eModel()
	if err != nil {
		return nil, err
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host+":30001")
	if err != nil {
		return nil, fmt.Errorf("can't connect to ur arm (%s): %w", host, err)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	newArm := &URArm{
		mu:                      &sync.Mutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		conn:                    conn,
		speed:                   speed,
		debug:                   false,
		haveData:                false,
		logger:                  logger,
		cancel:                  cancel,
		model:                   model,
		robot:                   r,
	}

	onData := make(chan struct{})
	var onDataOnce sync.Once
	newArm.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		if err := reader(cancelCtx, conn, newArm, func() {
			onDataOnce.Do(func() {
				close(onData)
			})
		}); err != nil {
			logger.Errorw("reader failed", "error", err)
		}
	}, newArm.activeBackgroundWorkers.Done)

	respondTimeout := 2 * time.Second
	timer := time.NewTimer(respondTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, multierr.Combine(ctx.Err(), newArm.Close(ctx))
	case <-timer.C:
		return nil, multierr.Combine(errors.Errorf("arm failed to respond in time (%s)", respondTimeout), newArm.Close(ctx))
	case <-onData:
		return newArm, nil
	}
}

// ModelFrame returns all the information necessary for including the arm in a FrameSystem.
func (ua *URArm) ModelFrame() referenceframe.Model {
	return ua.model
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

// State TODO.
func (ua *URArm) State() (RobotState, error) {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	age := time.Since(ua.state.creationTime)
	if age > time.Second {
		return ua.state, fmt.Errorf("ur status is too old %v from: %v", age, ua.state.creationTime)
	}
	return ua.state, nil
}

// GetJointPositions TODO.
func (ua *URArm) GetJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	radians := []float64{}
	state, err := ua.State()
	if err != nil {
		return nil, err
	}
	for _, j := range state.Joints {
		radians = append(radians, j.Qactual)
	}
	return referenceframe.JointPositionsFromRadians(radians), nil
}

// GetEndPosition computes and returns the current cartesian position.
func (ua *URArm) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	joints, err := ua.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(ua.model, joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (ua *URArm) MoveToPosition(ctx context.Context, pos *commonpb.Pose, worldState *commonpb.WorldState) error {
	ctx, done := ua.opMgr.New(ctx)
	defer done()
	return arm.Move(ctx, ua.robot, ua, pos, worldState)
}

// MoveToJointPositions TODO.
func (ua *URArm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	return ua.MoveToJointPositionRadians(ctx, referenceframe.JointPositionsToRadians(joints))
}

// Stop stops the arm with some deceleration.
func (ua *URArm) Stop(ctx context.Context) error {
	_, done := ua.opMgr.New(ctx)
	defer done()
	cmd := fmt.Sprintf("stopj(a=%1.2f)\r\n", 5.0*ua.speed)

	_, err := ua.conn.Write([]byte(cmd))
	return err
}

// IsMoving returns whether the arm is moving.
func (ua *URArm) IsMoving() bool {
	return ua.opMgr.OpRunning()
}

// MoveToJointPositionRadians TODO.
func (ua *URArm) MoveToJointPositionRadians(ctx context.Context, radians []float64) error {
	ctx, done := ua.opMgr.New(ctx)
	defer done()

	ua.muMove.Lock()
	defer ua.muMove.Unlock()

	if len(radians) != 6 {
		return errors.New("need 6 joints")
	}

	cmd := fmt.Sprintf("movej([%f,%f,%f,%f,%f,%f], a=%1.2f, v=%1.2f, r=0)\r\n",
		radians[0],
		radians[1],
		radians[2],
		radians[3],
		radians[4],
		radians[5],
		5.0*ua.speed,
		4.0*ua.speed,
	)

	_, err := ua.conn.Write([]byte(cmd))
	if err != nil {
		return err
	}

	retried := false
	slept := 0
	for {
		good := true
		state, err := ua.State()
		if err != nil {
			return err
		}
		for idx, r := range radians {
			if math.Round(r*100) != math.Round(state.Joints[idx].Qactual*100) {
				good = false
			}
		}

		if good {
			return nil
		}

		err = ua.getAndResetRuntimeError()
		if err != nil {
			return err
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
		if !goutils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}
		slept += 10
	}
}

// CurrentInputs TODO.
func (ua *URArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := ua.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.JointPosToInputs(res), nil
}

// GoToInputs TODO.
func (ua *URArm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return ua.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(goal))
}

// AddToLog TODO.
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
		sizeBuf, err := goutils.ReadBytes(ctx, conn, 4)
		if err != nil {
			return err
		}

		msgSize := binary.BigEndian.Uint32(sizeBuf)
		if msgSize <= 4 || msgSize > 10000 {
			return errors.Errorf("invalid msg size: %d", msgSize)
		}

		buf, err := goutils.ReadBytes(ctx, conn, int(msgSize-4))
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
			userErr := readURRobotMessage(buf, ua.logger)
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
