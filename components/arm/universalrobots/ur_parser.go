package universalrobots

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

// RobotModeData TODO.
type RobotModeData struct {
	Timestamp                uint64
	IsRealRobotConnected     bool
	IsRealRobotEnabled       bool
	IsRobotPowerOn           bool
	IsEmergencyStopped       bool
	IsProtectiveStopped      bool
	IsProgramRunning         bool
	IsProgramPaused          bool
	RobotMode                byte
	ControlMode              byte
	TargetSpeedFraction      float64
	SpeedScaling             float64
	TargetSpeedFractionLimit float64
}

// JointData TODO.
type JointData struct {
	Qactual   float64 // angle currently in radians
	Qtarget   float64 // angle target in radians
	QDactual  float64
	Iactual   float32
	Vactual   float32
	Tmotor    float32
	Tmicro    float32
	JointMode byte
}

// AngleValues TODO.
func (j JointData) AngleValues() float64 {
	return utils.RadToDeg(j.Qactual)
}

// ToolData TODO.
type ToolData struct {
	AnalogInputRange0 byte
	AnalogInputRange1 byte
	AnalogInput0      float64
	AnalogInput1      float64
	ToolVoltage48V    float32
	ToolOutputVoltage byte
	ToolCurrent       float32
	ToolTemperature   float32
	ToolMode          byte
}

// MasterboardData TODO.
type MasterboardData struct {
	DigitalInputBits                 int32
	DigitalOutputBits                int32
	AnalogInputRange0                byte
	AnalogInputRange1                byte
	AnalogInput0                     float64
	AnalogInput1                     float64
	AnalogOutputDomain0              byte
	AnalogOutputDomain1              byte
	AnalogOutput0                    float64
	AnalogOutput1                    float64
	MasterBoardTemperature           float32
	RobotVoltage48V                  float32
	RobotCurrent                     float32
	MasterIOCurrent                  float32
	SafetyMode                       byte
	InReducedMode                    byte
	Euromap67InterfaceInstalled      byte
	NotUsed1                         uint32
	OperationalModeSelectorInput     byte
	ThreePositionEnablingDeviceInput byte
	NotUsed2                         byte
}

// CartesianInfo TODO.
type CartesianInfo struct {
	X           float64
	Y           float64
	Z           float64
	Rx          float64
	Ry          float64
	Rz          float64
	TCPOffsetX  float64
	TCPOffsetY  float64
	TCPOffsetZ  float64
	TCPOffsetRx float64
	TCPOffsetRy float64
	TCPOffsetRz float64
}

// SimpleString TODO.
func (c CartesianInfo) SimpleString() string {
	return fmt.Sprintf("x: %f | y: %f | z: %f | Rx: %f | Ry: %f | Rz : %f",
		c.X, c.Y, c.Z, c.Rx, c.Ry, c.Rz)
}

// NondelimitedString TODO.
func (c CartesianInfo) NondelimitedString() string {
	return fmt.Sprintf("%f %f %f %f %f %f",
		c.X, c.Y, c.Z, c.Rx, c.Ry, c.Rz)
}

// KinematicInfo TODO.
type KinematicInfo struct {
	Cheksum int32
	DHtheta float64
	DHa     float64
	Dhd     float64
	Dhalpha float64
}

// ForceModeData TODO.
type ForceModeData struct {
	Fx             float64
	Fy             float64
	Fz             float64
	Frx            float64
	Fry            float64
	Frz            float64
	RobotDexterity float64
}

// AdditionalInfo additional info from ur arm.
type AdditionalInfo struct {
	TpButtonState          byte
	FreedriveButtonEnabled bool
	IOEnabledFreedrive     bool
	Reserved               byte
}

// RobotState TODO.
type RobotState struct {
	RobotModeData
	Joints []JointData
	ToolData
	MasterboardData
	CartesianInfo
	Kinematics []KinematicInfo
	ForceModeData
	AdditionalInfo
	creationTime time.Time
}

func readRobotStateMessage(buf []byte, logger logging.Logger) (RobotState, error) {
	state := RobotState{
		creationTime: time.Now(),
	}

	for len(buf) > 0 {
		sz := binary.BigEndian.Uint32(buf)
		packageType := buf[4]
		content := buf[5:sz]
		buf = buf[sz:]

		buffer := bytes.NewBuffer(content)

		switch packageType {
		case 0:
			if err := binary.Read(buffer, binary.BigEndian, &state.RobotModeData); err != nil && !errors.Is(err, io.EOF) {
				return state, err
			}
		case 1:
			for {
				d := JointData{}
				err := binary.Read(buffer, binary.BigEndian, &d)
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					return state, err
				}
				state.Joints = append(state.Joints, d)
			}
		case 2:
			if err := binary.Read(buffer, binary.BigEndian, &state.ToolData); err != nil && !errors.Is(err, io.EOF) {
				return state, err
			}
		case 3:
			if err := binary.Read(buffer, binary.BigEndian, &state.MasterboardData); err != nil && !errors.Is(err, io.EOF) {
				return state, err
			}
		case 4:
			if err := binary.Read(buffer, binary.BigEndian, &state.CartesianInfo); err != nil && !errors.Is(err, io.EOF) {
				return state, err
			}
		case 5:
			for buffer.Len() > 4 {
				d := KinematicInfo{}
				err := binary.Read(buffer, binary.BigEndian, &d)
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					return state, err
				}
				state.Kinematics = append(state.Kinematics, d)
			}
		case 6:
			// Configuration data, skipping, don't think we need
		case 7:
			if err := binary.Read(buffer, binary.BigEndian, &state.ForceModeData); err != nil && !errors.Is(err, io.EOF) {
				return state, err
			}
		case 8:
			if err := binary.Read(buffer, binary.BigEndian, &state.AdditionalInfo); err != nil && !errors.Is(err, io.EOF) {
				return state, err
			}
		case 9:
			// Calibration data, skipping, don't think we need
		case 10:
			// Safety data, skipping, don't think we need
		case 11:
			// Tool communication info, skipping, don't think we need
		case 12:
			// Tool mode info, skipping, don't think we need
		default:
			logger.Debugf("unknown packageType: %d size: %d content size: %d\n", packageType, sz, len(content))
		}
	}

	return state, nil
}

// return userErr, error.
func readURRobotMessage(buf []byte, logger logging.Logger) error {
	ts := binary.BigEndian.Uint64(buf[1:])
	// messageSource := buf[9]
	robotMessageType := buf[10]

	buf = buf[11:]

	switch robotMessageType {
	case 0: // text?
		logger.Debugf("ur log: %s\n", string(buf))

	case 6: // error
		robotMessageCode := binary.BigEndian.Uint32(buf)
		robotMessageArgument := binary.BigEndian.Uint32(buf[4:])
		robotMessageReportLevel := binary.BigEndian.Uint32(buf[8:])
		robotMessageDataType := binary.BigEndian.Uint32(buf[12:])
		robotMessageData := binary.BigEndian.Uint32(buf[16:])
		robotCommTextMessage := string(buf[20:])

		logger.Debugf("robot error! code: C%dA%d reportLevel: %d, dataType: %d, data: %d, msg: %s\n",
			robotMessageCode, robotMessageArgument, robotMessageReportLevel, robotMessageDataType, robotMessageData, robotCommTextMessage)

	case 3: // Version

		projectNameSize := buf[0]
		// projectName := string(buf[12:12+projectNameSize])
		pos := projectNameSize + 1
		majorVersion := buf[pos]
		minorVersion := buf[pos+1]
		bugFixVersion := binary.BigEndian.Uint32(buf[pos+2:])
		buildNumber := binary.BigEndian.Uint32(buf[pos+8:])

		logger.Debugf("UR version %v.%v.%v.%v\n", majorVersion, minorVersion, bugFixVersion, buildNumber)

	case 12: // i have no idea what this is
		if len(buf) != 9 {
			logger.Debugf("got a weird robot message of type 12 with bad length: %d\n", len(buf))
		} else {
			a := binary.BigEndian.Uint64(buf)
			b := buf[8]
			if a != 0 || b != 1 {
				logger.Debugf("got a weird robot message of type 12 with bad data: %v %v\n", a, b)
			}
		}

	case 7: // KeyMessage
		robotMessageCode := binary.BigEndian.Uint32(buf)
		robotMessageArgument := binary.BigEndian.Uint32(buf[4:])
		robotMessageTitleSize := buf[8]
		robotMessageTitle := string(buf[9 : 9+robotMessageTitleSize])
		keyTextMessage := string(buf[9+robotMessageTitleSize:])

		if false {
			// TODO(erh): this is better than sleeping in other code, be smart!!
			logger.Debugf("KeyMessage robotMessageCode: %d robotMessageArgument: %d robotMessageTitle: %s keyTextMessage: %s\n",
				robotMessageCode, robotMessageArgument, robotMessageTitle, keyTextMessage)
		}
	case 10: // ROBOT_MESSAGE_TYPE_RUNTIME_EXCEPTION
		scriptLineNumber := binary.BigEndian.Uint32(buf)
		scriptColumnNumber := binary.BigEndian.Uint32(buf[4:])
		msg := string(buf[9:])
		runtimeErr := errors.Errorf("runtime error at line: %d col: %d msg: %s", scriptLineNumber, scriptColumnNumber, msg)
		return runtimeErr
	default:
		logger.Debugf("unknown robotMessageType: %d ts: %v %v\n", robotMessageType, ts, buf)
		return nil
	}

	return nil
}
