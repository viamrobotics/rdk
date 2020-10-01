package arm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

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

func (j JointData) AngleDegrees() float64 {
	return convertRtoD(j.Qactual)
}

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

func (c CartesianInfo) SimpleString() string {
	return fmt.Sprintf("x: %f | y: %f | z: %f | Rx: %f | Ry: %f | Rz : %f",
		c.X, c.Y, c.Z, c.Rx, c.Ry, c.Rz)
}

type KinematicInfo struct {
	Cheksum int32
	DHtheta float64
	DHa     float64
	Dhd     float64
	Dhalpha float64
}

type ForceModeData struct {
	Fx             float64
	Fy             float64
	Fz             float64
	Frx            float64
	Fry            float64
	Frz            float64
	RobotDexterity float64
}

type RobotState struct {
	RobotModeData
	Joints []JointData
	ToolData
	MasterboardData
	CartesianInfo
	Kinematics []KinematicInfo
	ForceModeData
}

func readRobotStateMessage(buf []byte) (RobotState, error) {
	state := RobotState{}

	for len(buf) > 0 {
		sz := binary.BigEndian.Uint32(buf)
		packageType := buf[4]
		content := buf[5:sz]
		buf = buf[sz:]

		buffer := bytes.NewBuffer(content)

		if packageType == 0 {
			binary.Read(buffer, binary.BigEndian, &state.RobotModeData)
		} else if packageType == 1 {
			for {
				d := JointData{}
				err := binary.Read(buffer, binary.BigEndian, &d)
				if err != nil {
					if err == io.EOF {
						break
					}
					return state, err
				}
				state.Joints = append(state.Joints, d)
			}
		} else if packageType == 2 {
			binary.Read(buffer, binary.BigEndian, &state.ToolData)
		} else if packageType == 3 {
			binary.Read(buffer, binary.BigEndian, &state.MasterboardData)
		} else if packageType == 4 {
			binary.Read(buffer, binary.BigEndian, &state.CartesianInfo)
		} else if packageType == 5 {

			for buffer.Len() > 4 {
				d := KinematicInfo{}
				err := binary.Read(buffer, binary.BigEndian, &d)
				if err != nil {
					if err == io.EOF {
						break
					}
					return state, err
				}
				state.Kinematics = append(state.Kinematics, d)
			}

		} else if packageType == 6 {
			// Configuration data, skipping, don't think we need
		} else if packageType == 7 {
			binary.Read(buffer, binary.BigEndian, &state.ForceModeData)
		} else if packageType == 8 {
			// Additional Info, skipping, don't think we need
		} else if packageType == 9 {
			// Calibration data, skipping, don't think we need
		} else if packageType == 10 {
			// Safety data, skipping, don't think we need
		} else if packageType == 11 {
			// Tool communication info, skipping, don't think we need
		} else if packageType == 12 {
			// Tool mode info, skipping, don't think we need
		} else {
			fmt.Printf("unknown packageType: %d size: %d content size: %d\n", packageType, sz, len(content))
		}
	}

	return state, nil
}
