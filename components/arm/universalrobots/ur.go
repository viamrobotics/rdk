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
	pb "go.viam.com/api/component/arm/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ModelName is the resource model.
var ModelName = resource.NewDefaultModel("ur5e")

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	Speed               float64 `json:"speed_degs_per_sec"`
	Host                string  `json:"host"`
	ArmHostedKinematics bool    `json:"arm_hosted_kinematics,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	if cfg.Host == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "host")
	}
	if cfg.Speed > 1 || cfg.Speed < .1 {
		return nil, errors.New("speed for universalrobots has to be between .1 and 1")
	}
	return []string{}, nil
}

//go:embed ur5e.json
var ur5modeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, ModelName, registry.Component{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return URArmConnect(ctx, r, config, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.Subtype, ModelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

// Model returns the kinematics model of the ur arm, also has all Frame information.
func Model(name string) (referenceframe.Model, error) {
	return referenceframe.UnmarshalModelJSON(ur5modeljson, name)
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
	urHostedKinematics      bool
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
	attrs, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(attrs, cfg.ConvertedAttributes)
	}

	if attrs.Speed > 1 || attrs.Speed < .1 {
		return nil, errors.New("speed for universalrobots has to be between .1 and 1")
	}

	model, err := Model(cfg.Name)
	if err != nil {
		return nil, err
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", attrs.Host+":30001")
	if err != nil {
		return nil, fmt.Errorf("can't connect to ur arm (%s): %w", attrs.Host, err)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	newArm := &URArm{
		mu:                      &sync.Mutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		conn:                    conn,
		speed:                   attrs.Speed,
		debug:                   false,
		haveData:                false,
		logger:                  logger,
		cancel:                  cancel,
		model:                   model,
		robot:                   r,
		urHostedKinematics:      attrs.ArmHostedKinematics,
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

// JointPositions TODO.
func (ua *URArm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
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

// EndPosition computes and returns the current cartesian position.
func (ua *URArm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	joints, err := ua.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(ua.model, joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
// If the UR arm was configured with "arm_hosted_kinematics = 'true'" or extra["arm_hosted_kinematics"] = true is specified at runtime
// this command will use the kinematics hosted by the Universal Robots arm.  If these are used with obstacles
// or interaction spaces embedded in the world state an error will  be thrown, as the hosted planning does not support these constraints.
func (ua *URArm) MoveToPosition(
	ctx context.Context,
	pos spatialmath.Pose,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) error {
	ctx, done := ua.opMgr.New(ctx)
	defer done()

	usingHostedKinematics, err := ua.useURHostedKinematics(worldState, extra)
	if err != nil {
		return err
	}
	if usingHostedKinematics {
		return ua.moveWithURHostedKinematics(ctx, pos)
	}
	return arm.Move(ctx, ua.robot, ua, pos, worldState)
}

// MoveToJointPositions TODO.
func (ua *URArm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions, extra map[string]interface{}) error {
	return ua.MoveToJointPositionRadians(ctx, referenceframe.JointPositionsToRadians(joints))
}

// Stop stops the arm with some deceleration.
func (ua *URArm) Stop(ctx context.Context, extra map[string]interface{}) error {
	_, done := ua.opMgr.New(ctx)
	defer done()
	cmd := fmt.Sprintf("stopj(a=%1.2f)\r\n", 5.0*ua.speed)

	_, err := ua.conn.Write([]byte(cmd))
	return err
}

// IsMoving returns whether the arm is moving.
func (ua *URArm) IsMoving(ctx context.Context) (bool, error) {
	return ua.opMgr.OpRunning(), nil
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
	res, err := ua.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return ua.model.InputFromProtobuf(res), nil
}

// GoToInputs TODO.
func (ua *URArm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return ua.MoveToJointPositions(ctx, ua.model.ProtobufFromInput(goal), nil)
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
					state.Joints[0].AngleValues(),
					state.Joints[1].AngleValues(),
					state.Joints[2].AngleValues(),
					state.Joints[3].AngleValues(),
					state.Joints[4].AngleValues(),
					state.Joints[5].AngleValues(),
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

const errURHostedKinematics = "cannot use UR hosted kinematics with obstacles or interaction spaces"

func (ua *URArm) useURHostedKinematics(worldState *referenceframe.WorldState, extra map[string]interface{}) (bool, error) {
	// function to error out if trying to use world state with hosted kinematics
	checkWorldState := func(usingHostedKinematics bool) (bool, error) {
		if usingHostedKinematics && worldState != nil && (len(worldState.Obstacles) != 0 || len(worldState.InteractionSpaces) != 0) {
			return false, errors.New(errURHostedKinematics)
		}
		return usingHostedKinematics, nil
	}

	// if runtime preference is specified, obey that
	if extra != nil {
		if usingAtRuntime, ok := extra["arm_hosted_kinematics"].(bool); ok {
			return checkWorldState(usingAtRuntime)
		}
	}

	// otherwise default to option provided at config time
	return checkWorldState(ua.urHostedKinematics)
}

func (ua *URArm) moveWithURHostedKinematics(ctx context.Context, pose spatialmath.Pose) error {
	// UR5 arm takes R3 angle axis as input
	pt := pose.Point()
	aa := pose.Orientation().AxisAngles().ToR3()

	// write command to arm, need to request position in meters
	cmd := fmt.Sprintf("movej(get_inverse_kin(p[%f,%f,%f,%f,%f,%f]), a=1.4, v=4, r=0)\r\n",
		0.001*pt.X,
		0.001*pt.Y,
		0.001*pt.Z,
		aa.X,
		aa.Y,
		aa.Z,
	)
	_, err := ua.conn.Write([]byte(cmd))
	if err != nil {
		return err
	}

	retried := false
	slept := 0
	for {
		cur, err := ua.EndPosition(ctx, nil)
		if err != nil {
			return err
		}
		delta := spatialmath.PoseDelta(pose, cur)
		if delta.Point().Norm() <= 1.5 && delta.Orientation().AxisAngles().ToR3().Norm() <= 1.0 {
			return nil
		}

		err = ua.getAndResetRuntimeError()
		if err != nil {
			return err
		}

		slept += 10

		if slept > 5000 && !retried {
			_, err := ua.conn.Write([]byte(cmd))
			if err != nil {
				return err
			}
			retried = true
		}

		if slept > 10000 {
			delta = spatialmath.PoseDelta(pose, cur)
			return errors.Errorf("can't reach position.\n want: %v\n\tat: %v\n diffs: %f %f",
				pose, cur, delta.Point().Norm(), delta.Orientation().AxisAngles().ToR3().Norm(),
			)
		}
		if !goutils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}
	}
}
