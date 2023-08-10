// Package limo implements the AgileX Limo base
package limo

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// default port for limo serial comm.
const (
	defaultSerialPath  = "/dev/ttyTHS1"
	minTurningRadiusM  = 0.4 // from datasheet at: https://www.wevolver.com/specs/agilex-limo
	defaultBaseTreadMm = 172 // "Tread" from datasheet at: https://www.wevolver.com/specs/agilex-limo
)

// valid steering modes for limo.
const (
	DIFFERENTIAL = steeringMode(iota)
	ACKERMANN
	OMNI
)

type steeringMode uint8

func (m steeringMode) String() string {
	switch m {
	case DIFFERENTIAL:
		return "differential"
	case ACKERMANN:
		return "ackermann"
	case OMNI:
		return "omni"
	}
	return "Unknown"
}

var model = resource.DefaultModelFamily.WithModel("agilex-limo")

func init() {
	resource.RegisterComponent(base.API, model, resource.Registration[base.Base, *Config]{Constructor: createLimoBase})
}

type limoFrame struct {
	id   uint16
	data []uint8
}

type limoState struct {
	controlThreadStarted                    bool
	velocityLinearGoal, velocityAngularGoal r3.Vector
}
type limoBase struct {
	resource.Named
	resource.AlwaysRebuild
	driveMode          string
	opMgr              operation.SingleOperationManager
	cancel             context.CancelFunc
	waitGroup          sync.WaitGroup
	width              int
	wheelbase          int
	maxInnerAngle      float64
	rightAngleScale    float64
	maxLinearVelocity  int
	maxAngularVelocity int
	geometries         []spatialmath.Geometry

	logger golog.Logger

	serialMutex sync.Mutex
	serialPort  io.ReadWriteCloser
	testChan    chan []uint8

	stateMutex sync.Mutex
	state      limoState
}

// Config is how you configure a limo base.
type Config struct {
	resource.TriviallyValidateConfig
	DriveMode    string `json:"drive_mode"`
	SerialDevice string `json:"serial_path"` // path to /dev/ttyXXXX file

	// TestChan is a fake "serial" path for test use only
	TestChan chan []uint8 `json:"-"`
}

// createLimoBase returns a AgileX limo base.
func createLimoBase(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (base.Base, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	logger.Debugf("creating limo base with config %+v", newConf)

	if newConf.DriveMode == "" {
		return nil, errors.New("drive mode must be defined and one of differential, ackermann, or omni")
	}

	sDevice := newConf.SerialDevice
	if sDevice == "" {
		sDevice = defaultSerialPath
	}

	lb := &limoBase{
		Named:              conf.ResourceName().AsNamed(),
		driveMode:          newConf.DriveMode,
		testChan:           newConf.TestChan, // for testing only
		logger:             logger,
		width:              defaultBaseTreadMm,
		wheelbase:          200,
		maxLinearVelocity:  3000,
		maxAngularVelocity: 180,
		maxInnerAngle:      .48869, // 28 degrees in radians
		rightAngleScale:    1.64,
	}

	geometries, err := kinematicbase.CollisionGeometry(conf.Frame)
	if err != nil {
		logger.Warnf("base %v %s", lb.Name(), err.Error())
	}
	lb.geometries = geometries

	if newConf.TestChan == nil {
		logger.Debugf("creating serial connection to: ", sDevice)
		lb.serialPort, err = initSerialConnection(sDevice)
		if err != nil {
			logger.Error("error creating serial connection", err)
			return nil, err
		}
	}

	// enable commanded mode
	frame := new(limoFrame)
	frame.id = 0x421
	frame.data = make([]uint8, 8)
	frame.data[0] = 0x01
	frame.data[1] = 0
	frame.data[2] = 0
	frame.data[3] = 0
	frame.data[4] = 0
	frame.data[5] = 0
	frame.data[6] = 0
	frame.data[7] = 0

	logger.Debug("sending init frame")
	err = lb.sendFrame(frame)
	if err != nil && !strings.HasPrefix(err.Error(), "error enabling commanded mode") {
		sererr := lb.closeSerial()
		if sererr != nil {
			return nil, multierr.Combine(err, sererr)
		}

		return nil, err
	}

	lb.stateMutex.Lock()
	if !lb.state.controlThreadStarted {
		lb.startControlThread()
		lb.state.controlThreadStarted = true
	}
	lb.stateMutex.Unlock()

	logger.Debug("base initialized")

	return lb, nil
}

func initSerialConnection(sDevice string) (io.ReadWriteCloser, error) {
	serialOptions := serial.OpenOptions{
		PortName:          sDevice,
		BaudRate:          460800,
		DataBits:          8,
		StopBits:          1,
		MinimumReadSize:   1,
		RTSCTSFlowControl: true,
	}

	port, err := serial.Open(serialOptions)
	if err != nil {
		return nil, err
	}

	return port, nil
}

// this rover requires messages to be sent continuously or the motors will shut down after 100ms.
func (lb *limoBase) startControlThread() {
	var ctx context.Context
	ctx, lb.cancel = context.WithCancel(context.Background())
	lb.logger.Debug("starting control thread")

	lb.waitGroup.Add(1)
	go func() {
		defer lb.waitGroup.Done()

		for {
			utils.SelectContextOrWait(ctx, time.Duration(float64(time.Millisecond)*10))
			err := lb.controlThreadLoopPass(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				lb.logger.Warn(err)
			}
		}
	}()
}

func (lb *limoBase) controlThreadLoopPass(ctx context.Context) error {
	lb.stateMutex.Lock()
	linearGoal := lb.state.velocityLinearGoal
	angularGoal := lb.state.velocityAngularGoal
	lb.stateMutex.Unlock()

	var err error
	switch lb.driveMode {
	case DIFFERENTIAL.String():
		err = lb.setMotionCommand(linearGoal.Y, -angularGoal.Z, 0, 0)
	case ACKERMANN.String():
		r := linearGoal.Y / angularGoal.Z
		if math.Abs(r) < float64(lb.width)/2.0 {
			// Note: Do we need a tolerance comparison here? Don't think so, as velocityLinearGoal.Y should always be exactly zero
			// when we expect it to be.
			if r == 0 {
				r = angularGoal.Z / math.Abs(angularGoal.Z) * (float64(lb.width)/2.0 + 10)
			} else {
				r = r / math.Abs(r) * (float64(lb.width)/2.0 + 10)
			}
		}
		centralAngle := math.Atan(float64(lb.wheelbase) / r)
		innerAngle := math.Atan((2 * float64(lb.wheelbase) * math.Sin(centralAngle) /
			(2*float64(lb.wheelbase)*math.Cos(math.Abs(centralAngle)) - float64(lb.width)*math.Sin(math.Abs(centralAngle)))))

		if innerAngle > lb.maxInnerAngle {
			innerAngle = lb.maxInnerAngle
		}
		if innerAngle < -lb.maxInnerAngle {
			innerAngle = -lb.maxInnerAngle
		}

		steeringAngle := innerAngle / lb.rightAngleScale
		// steering angle is in unit of .001 radians
		err = lb.setMotionCommand(linearGoal.Y, 0, 0, -steeringAngle*1000)
	case OMNI.String():
		err = lb.setMotionCommand(linearGoal.Y, -angularGoal.Z, linearGoal.X, 0)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// Sends the serial frame. Must be run inside a lock.
func (lb *limoBase) sendFrame(frame *limoFrame) error {
	var checksum uint32
	var frameLen uint8 = 0x0e
	data := make([]uint8, 14)
	data[0] = 0x55
	data[1] = frameLen // frame length
	data[2] = uint8(frame.id >> 8)
	data[3] = uint8(frame.id & 0xff)
	for i := 0; i < 8; i++ {
		data[i+4] = frame.data[i]
		checksum += uint32(frame.data[i])
	}
	data[frameLen-1] = uint8(checksum & 0xff)

	lb.serialMutex.Lock()
	defer lb.serialMutex.Unlock()

	if lb.testChan != nil {
		lb.logger.Debug("writing to test chan")
		lb.testChan <- data
	} else {
		_, err := lb.serialPort.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
}

// see https://github.com/agilexrobotics/limo_ros/blob/master/limo_base/src/limo_driver.cpp
func (lb *limoBase) setMotionCommand(linearVel float64,
	angularVel, lateralVel, steeringAngle float64,
) error {
	frame := new(limoFrame)
	frame.id = 0x111
	linearCmd := int16(linearVel)
	angularCmd := int16(angularVel)
	lateralCmd := int16(lateralVel)
	steeringCmd := int16(steeringAngle)

	frame.data = make([]uint8, 8)
	frame.data[0] = uint8(linearCmd >> 8)
	frame.data[1] = uint8(linearCmd & 0x00ff)
	frame.data[2] = uint8(angularCmd >> 8)
	frame.data[3] = uint8(angularCmd & 0x00ff)
	frame.data[4] = uint8(lateralCmd >> 8)
	frame.data[5] = uint8(lateralCmd & 0x00ff)
	frame.data[6] = uint8(steeringCmd >> 8)
	frame.data[7] = uint8(steeringCmd & 0x00ff)

	err := lb.sendFrame(frame)
	if err != nil {
		lb.logger.Error(err)
		return err
	}

	return nil
}

// positive angleDeg spins base left. degsPerSec is a positive angular velocity.
func (lb *limoBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	lb.logger.Debugf("Spin(%f, %f)", angleDeg, degsPerSec)
	if degsPerSec <= 0 {
		return errors.New("degrees per second must be a positive, non-zero value")
	}
	secsToRun := math.Abs(angleDeg / degsPerSec)
	var err error
	if lb.driveMode == DIFFERENTIAL.String() || lb.driveMode == OMNI.String() {
		dir := 1.0
		if math.Signbit(angleDeg) {
			dir = -1.0
		}
		err = lb.SetVelocity(ctx, r3.Vector{}, r3.Vector{Z: dir * degsPerSec}, extra)
	} else if lb.driveMode == ACKERMANN.String() {
		// TODO: this is not the correct math
		linear := float64(lb.maxLinearVelocity) * (degsPerSec / 360) * math.Pi
		// max angular translates to max steering angle for ackermann+
		angular := math.Copysign(float64(lb.maxAngularVelocity), angleDeg)
		err = lb.SetVelocity(ctx, r3.Vector{Y: linear}, r3.Vector{Z: angular}, extra)
	}

	if err != nil {
		return err
	}
	// stop lb after calculated time
	timeToRun := time.Millisecond * time.Duration(secsToRun*1000)
	lb.logger.Debugf("Will run for duration %f", timeToRun)
	utils.SelectContextOrWait(ctx, timeToRun)
	return lb.Stop(ctx, extra)
}

func (lb *limoBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	lb.logger.Debugf("MoveStraight(%d, %f)", distanceMm, mmPerSec)
	err := lb.SetVelocity(ctx, r3.Vector{Y: mmPerSec}, r3.Vector{}, extra)
	if err != nil {
		return err
	}

	// stop lb after calculated time
	timeToRun := time.Millisecond * time.Duration(math.Abs(float64(distanceMm)/mmPerSec)*1000)
	lb.logger.Debugf("Will run for duration %f", timeToRun)
	utils.SelectContextOrWait(ctx, timeToRun)
	return lb.Stop(ctx, extra)
}

// linear is in mm/sec, angular in degrees/sec.
// positive angular velocity turns base left.
func (lb *limoBase) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	lb.logger.Debugf("Will set linear velocity %f angular velocity %f", linear, angular)

	_, done := lb.opMgr.New(ctx)
	defer done()

	// this lb expects angular velocity to be expressed in .001 radians/sec, convert
	angular.Z = rdkutils.DegToRad(-angular.Z) * 1000

	lb.stateMutex.Lock()
	lb.state.velocityLinearGoal = linear
	lb.state.velocityAngularGoal = angular
	lb.stateMutex.Unlock()

	return nil
}

func (lb *limoBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	lb.logger.Debugf("Will set power linear %f angular %f", linear, angular)
	linY := linear.Y * float64(lb.maxLinearVelocity)
	angZ := angular.Z * float64(lb.maxAngularVelocity)
	err := lb.SetVelocity(ctx, r3.Vector{Y: linY}, r3.Vector{Z: angZ}, extra)
	if err != nil {
		return err
	}
	return nil
}

func (lb *limoBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	lb.logger.Debug("Stop()")
	err := lb.SetVelocity(ctx, r3.Vector{}, r3.Vector{}, extra)
	if err != nil {
		return err
	}
	lb.opMgr.CancelRunning(ctx)
	return nil
}

func (lb *limoBase) IsMoving(ctx context.Context) (bool, error) {
	lb.logger.Debug("IsMoving()")
	lb.stateMutex.Lock()
	defer lb.stateMutex.Unlock()
	if lb.state.velocityLinearGoal.ApproxEqual(r3.Vector{}) && lb.state.velocityAngularGoal.ApproxEqual(r3.Vector{}) {
		return false, nil
	}
	return true, nil
}

func (lb *limoBase) Properties(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
	var lbTurnRadiusM float64

	switch lb.driveMode {
	case ACKERMANN.String():
		lbTurnRadiusM = minTurningRadiusM
	default:
		lbTurnRadiusM = 0.0 // omni and differential can turn in place
	}

	return base.Properties{
		TurningRadiusMeters: lbTurnRadiusM,
		WidthMeters:         float64(lb.width) * 0.001, // convert from mm to meters
	}, nil
}

func (lb *limoBase) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return lb.geometries, nil
}

// DoCommand executes additional commands beyond the Base{} interface.
func (lb *limoBase) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	name, ok := cmd["command"]
	if !ok {
		return nil, errors.New("missing 'command' value")
	}
	switch name {
	case "drive_mode":
		modeRaw, ok := cmd["mode"]
		if !ok {
			return nil, errors.New("mode must be set, one of differential|ackermann|omni")
		}
		mode, ok := modeRaw.(string)
		if !ok {
			return nil, errors.New("mode value must be a string")
		}
		mode = strings.ToLower(mode)
		if !((mode == DIFFERENTIAL.String()) || (mode == ACKERMANN.String()) || (mode == OMNI.String())) {
			return nil, errors.New("mode value must be one of differential|ackermann|omni")
		}
		lb.driveMode = mode
		return map[string]interface{}{"return": mode}, nil
	default:
		return nil, fmt.Errorf("no such command: %s", name)
	}
}

func (lb *limoBase) Close(ctx context.Context) error {
	lb.logger.Debug("Close()")
	if err := lb.Stop(ctx, nil); err != nil {
		return err
	}

	if lb.cancel != nil {
		lb.logger.Debug("calling cancel() on control thread")
		lb.cancel()
		lb.cancel = nil
		lb.waitGroup.Wait()
		lb.logger.Debug("done waiting on cancel on control thread")
	}

	if err := lb.closeSerial(); err != nil {
		return err
	}

	return nil
}

func (lb *limoBase) closeSerial() error {
	lb.serialMutex.Lock()
	defer lb.serialMutex.Unlock()

	if lb.serialPort != nil {
		if err := lb.serialPort.Close(); err != nil {
			lb.logger.Warn("failed to close serial", err)
			return err
		}
		lb.serialPort = nil
	}

	return nil
}
