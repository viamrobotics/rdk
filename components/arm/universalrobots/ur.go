// Package universalrobots implements the UR arm from Universal Robots.
package universalrobots

import (
	"bufio"
	"context"
	// for embedding model file.
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/arm/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// Model is the name of the UR5e model of an arm component.
var Model = resource.DefaultModelFamily.WithModel("ur5e")

// Config is used for converting config attributes.
type Config struct {
	SpeedDegsPerSec     float64 `json:"speed_degs_per_sec"`
	Host                string  `json:"host"`
	ArmHostedKinematics bool    `json:"arm_hosted_kinematics,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.Host == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "host")
	}
	if cfg.SpeedDegsPerSec > 180 || cfg.SpeedDegsPerSec < 3 {
		return nil, errors.New("speed for universalrobots has to be between 3 and 180 degrees per second")
	}
	return []string{}, nil
}

//go:embed ur5e.json
var ur5modeljson []byte

func init() {
	resource.RegisterComponent(arm.API, Model, resource.Registration[arm.Arm, *Config]{
		Constructor: func(
			ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger,
		) (arm.Arm, error) {
			return URArmConnect(ctx, conf, logger)
		},
	})
}

// MakeModelFrame returns the kinematics model of the ur arm, also has all Frame information.
func MakeModelFrame(name string) (referenceframe.Model, error) {
	return referenceframe.UnmarshalModelJSON(ur5modeljson, name)
}

// URArm TODO.
type URArm struct {
	resource.Named
	io.Closer
	muMove                  sync.Mutex
	connControl             net.Conn
	debug                   bool
	haveData                bool
	logger                  logging.Logger
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup
	model                   referenceframe.Model
	opMgr                   *operation.SingleOperationManager

	mu                       sync.Mutex
	state                    RobotState
	runtimeError             error
	inRemoteMode             bool
	speedRadPerSec           float64
	urHostedKinematics       bool
	dashboardConnection      net.Conn
	readRobotStateConnection net.Conn
	host                     string
	isConnected              bool
}

const waitBackgroundWorkersDur = 5 * time.Second

// Reconfigure atomically reconfigures this arm in place based on the new config.
func (ua *URArm) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	ua.mu.Lock()
	defer ua.mu.Unlock()
	if ua.host != newConf.Host {
		ua.host = newConf.Host
		if ua.dashboardConnection != nil {
			goutils.UncheckedError(ua.dashboardConnection.Close())
		}
		if ua.readRobotStateConnection != nil {
			goutils.UncheckedError(ua.readRobotStateConnection.Close())
		}
		return nil
	}
	ua.speedRadPerSec = rdkutils.DegToRad(newConf.SpeedDegsPerSec)
	ua.urHostedKinematics = newConf.ArmHostedKinematics
	return nil
}

// Close TODO.
func (ua *URArm) Close(ctx context.Context) error {
	ua.cancel()

	closeConn := func() {
		if err := ua.dashboardConnection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			ua.logger.Errorw("error closing arm's Dashboard connection", "error", err)
		}
		if err := ua.readRobotStateConnection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			ua.logger.Errorw("error closing arm's State connection", "error", err)
		}
		if ua.connControl != nil {
			if err := ua.connControl.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				ua.logger.Errorw("error closing arm's control connection", "error", err)
			}
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
func URArmConnect(ctx context.Context, conf resource.Config, logger logging.Logger) (arm.Arm, error) {
	// this is to speed up component build failure if the UR arm is not reachable
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
	defer cancel()
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	model, err := MakeModelFrame(conf.Name)
	if err != nil {
		return nil, err
	}

	var d net.Dialer

	connReadRobotState, err := d.DialContext(ctx, "tcp", newConf.Host+":30011")
	if err != nil {
		return nil, fmt.Errorf("can't connect to ur arm (%s): %w", newConf.Host, err)
	}
	connDashboard, err := d.DialContext(ctx, "tcp", newConf.Host+":29999")
	if err != nil {
		return nil, fmt.Errorf("can't connect to ur arm's dashboard (%s): %w", newConf.Host, err)
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	newArm := &URArm{
		Named:                    conf.ResourceName().AsNamed(),
		connControl:              nil,
		speedRadPerSec:           rdkutils.DegToRad(newConf.SpeedDegsPerSec),
		debug:                    false,
		haveData:                 false,
		logger:                   logger,
		cancel:                   cancel,
		model:                    model,
		opMgr:                    operation.NewSingleOperationManager(),
		urHostedKinematics:       newConf.ArmHostedKinematics,
		inRemoteMode:             false,
		readRobotStateConnection: connReadRobotState,
		dashboardConnection:      connDashboard,
		host:                     newConf.Host,
		isConnected:              true,
	}

	newArm.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		for {
			readerWriter := bufio.NewReadWriter(bufio.NewReader(newArm.dashboardConnection), bufio.NewWriter(newArm.dashboardConnection))
			err := dashboardReader(cancelCtx, *readerWriter, newArm)
			if err != nil &&
				(errors.Is(err, syscall.ECONNRESET) || errors.Is(err, io.ErrClosedPipe) || os.IsTimeout(err) || errors.Is(err, io.EOF)) {
				newArm.mu.Lock()
				newArm.inRemoteMode = false
				newArm.mu.Unlock()
				for {
					if err := cancelCtx.Err(); err != nil {
						return
					}
					logger.Debug("attempting to reconnect to ur arm dashboard")
					time.Sleep(1 * time.Second)
					connDashboard, err = d.DialContext(cancelCtx, "tcp", newArm.host+":29999")
					if err == nil {
						newArm.mu.Lock()
						newArm.dashboardConnection = connDashboard
						newArm.isConnected = true
						newArm.mu.Unlock()
						break
					} else {
						newArm.mu.Lock()
						newArm.isConnected = false
						newArm.mu.Unlock()
					}
					if !goutils.SelectContextOrWait(cancelCtx, 1*time.Second) {
						return
					}
				}
			} else if err != nil {
				logger.Errorw("dashboard reader failed", "error", err)
				newArm.mu.Lock()
				newArm.isConnected = false
				newArm.mu.Unlock()
				return
			}
		}
	}, newArm.activeBackgroundWorkers.Done)

	onData := make(chan struct{})
	var onDataOnce sync.Once
	newArm.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		for {
			err := reader(cancelCtx, newArm.readRobotStateConnection, newArm, func() {
				onDataOnce.Do(func() {
					close(onData)
				})
			})
			if err != nil &&
				(errors.Is(err, syscall.ECONNRESET) || errors.Is(err, io.ErrClosedPipe) || os.IsTimeout(err) || errors.Is(err, io.EOF)) {
				for {
					if err := cancelCtx.Err(); err != nil {
						return
					}
					logger.Debug("attempting to reconnect to ur arm 30011")
					connReadRobotState, err = d.DialContext(cancelCtx, "tcp", newArm.host+":30011")
					if err == nil {
						newArm.mu.Lock()
						newArm.readRobotStateConnection = connReadRobotState
						newArm.mu.Unlock()
						break
					}
					if !goutils.SelectContextOrWait(cancelCtx, 1*time.Second) {
						return
					}
				}
			} else if err != nil {
				logger.Errorw("reader failed", "error", err)
				return
			}
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
	return motionplan.ComputeOOBPosition(ua.model, joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
// If the UR arm was configured with "arm_hosted_kinematics = 'true'" or extra["arm_hosted_kinematics"] = true is specified at runtime
// this command will use the kinematics hosted by the Universal Robots arm.
func (ua *URArm) MoveToPosition(ctx context.Context, pos spatialmath.Pose, extra map[string]interface{}) error {
	if !ua.inRemoteMode {
		return errors.New("UR5 is in local mode; use the polyscope to switch it to remote control mode")
	}
	ctx, done := ua.opMgr.New(ctx)
	defer done()

	// Apply config hook first; if runtime setting exists, use that instead
	usingHostedKinematics := ua.urHostedKinematics
	if runtimeKinematicsSetting, ok := extra["arm_hosted_kinematics"].(bool); ok {
		usingHostedKinematics = runtimeKinematicsSetting
	}

	if usingHostedKinematics {
		return ua.moveWithURHostedKinematics(ctx, pos)
	}
	return arm.Move(ctx, ua.logger, ua, pos)
}

// MoveToJointPositions TODO.
func (ua *URArm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions, extra map[string]interface{}) error {
	// check that joint positions are not out of bounds
	if err := arm.CheckDesiredJointPositions(ctx, ua, joints); err != nil {
		return err
	}
	return ua.MoveToJointPositionRadians(ctx, referenceframe.JointPositionsToRadians(joints))
}

// Stop stops the arm with some deceleration.
func (ua *URArm) Stop(ctx context.Context, extra map[string]interface{}) error {
	if !ua.inRemoteMode {
		return errors.New("UR5 is in local mode; use the polyscope to switch it to remote control mode")
	}
	_, done := ua.opMgr.New(ctx)
	defer done()
	cmd := fmt.Sprintf("stopj(a=%1.2f)\r\n", 5.0*ua.speedRadPerSec)

	_, err := ua.connControl.Write([]byte(cmd))
	return err
}

// IsMoving returns whether the arm is moving.
func (ua *URArm) IsMoving(ctx context.Context) (bool, error) {
	return ua.opMgr.OpRunning(), nil
}

// MoveToJointPositionRadians TODO.
func (ua *URArm) MoveToJointPositionRadians(ctx context.Context, radians []float64) error {
	if !ua.inRemoteMode {
		return errors.New("UR5 is in local mode; use the polyscope to switch it to remote control mode")
	}
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
		0.8*ua.speedRadPerSec,
		ua.speedRadPerSec,
	)

	_, err := ua.connControl.Write([]byte(cmd))
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
			_, err := ua.connControl.Write([]byte(cmd))
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
	// check that joint positions are not out of bounds
	positionDegs := ua.model.ProtobufFromInput(goal)
	if err := arm.CheckDesiredJointPositions(ctx, ua, positionDegs); err != nil {
		return err
	}
	return ua.MoveToJointPositions(ctx, positionDegs, nil)
}

// Geometries returns the list of geometries associated with the resource, in any order. The poses of the geometries reflect their
// current location relative to the frame of the resource.
func (ua *URArm) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	// TODO (pl): RSDK-3316 abstract this to general arm function
	inputs, err := ua.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	gif, err := ua.model.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return gif.Geometries(), nil
}

// AddToLog TODO.
func (ua *URArm) AddToLog(msg string) error {
	if !ua.inRemoteMode {
		return errors.New("UR5 is in local mode; use the polyscope to switch it to remote control mode")
	}
	// TODO(erh): check for " in msg
	cmd := fmt.Sprintf("textmsg(\"%s\")\r\n", msg)
	_, err := ua.connControl.Write([]byte(cmd))
	return err
}

func dashboardReader(ctx context.Context, conn bufio.ReadWriter, ua *URArm) error {
	// Discard first line which is hello from dashboard
	if err := ua.dashboardConnection.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
		return err
	}
	if _, _, err := conn.ReadLine(); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := ua.dashboardConnection.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return err
		}
		if _, err := conn.WriteString("is in remote control\n"); err != nil {
			return err
		}
		if err := ua.dashboardConnection.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return err
		}
		if err := conn.Flush(); err != nil {
			return err
		}
		if err := ua.dashboardConnection.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return err
		}
		line, _, err := conn.ReadLine()
		if err != nil {
			return err
		}

		isRemote := strings.Contains(string(line), "true")
		if isRemote != ua.inRemoteMode {
			processControlEvent := func() error {
				ua.mu.Lock()
				defer ua.mu.Unlock()
				if isRemote {
					ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
					defer cancel()
					var d net.Dialer
					connControl, err := d.DialContext(ctx, "tcp", ua.host+":30001")
					if err != nil {
						return errors.Wrapf(err, "while the arm is not in local mode couldn't connect to ur arm (%s)", ua.host)
					}
					ua.connControl = connControl
				} else if ua.connControl != nil {
					if err := ua.connControl.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
						return err
					}
				}
				ua.inRemoteMode = isRemote
				return nil
			}
			if err := processControlEvent(); err != nil {
				return err
			}
		}
		if !goutils.SelectContextOrWait(ctx, 1*time.Second) {
			return ctx.Err()
		}
	}
}

func reader(ctx context.Context, conn net.Conn, ua *URArm, onHaveData func()) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := conn.SetReadDeadline(time.Now().Add(time.Millisecond * 1000)); err != nil {
			return err
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
	_, err := ua.connControl.Write([]byte(cmd))
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
			_, err := ua.connControl.Write([]byte(cmd))
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
