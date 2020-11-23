package arm

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"
)

type URArm struct {
	conn     net.Conn
	State    RobotState
	Debug    bool
	haveData bool
}

func URArmConnect(host string) (*URArm, error) {
	conn, err := net.Dial("tcp", host+":30001")
	if err != nil {
		return nil, err
	}

	arm := &URArm{conn: conn, Debug: false, haveData: false}

	go reader(conn, arm) // TODO: how to shutdown

	slept := 0
	for !arm.haveData {
		time.Sleep(100 * time.Millisecond)
		slept += 1

		if slept > 20 {
			return nil, fmt.Errorf("arm isn't respond")
		}
	}

	return arm, nil
}

func (arm *URArm) JointMoveDelta(joint int, amount float64) error {
	if joint < 0 || joint > 5 {
		return fmt.Errorf("invalid joint")
	}

	radians := []float64{}
	for _, j := range arm.State.Joints {
		radians = append(radians, j.Qactual)
	}

	radians[joint] += amount

	return arm.MoveToJointPositionRadians(radians)
}

func (arm *URArm) MoveToJointPositionRadians(radians []float64) error {
	if len(radians) != 6 {
		return fmt.Errorf("need 6 joints")
	}

	_, err := fmt.Fprintf(arm.conn, "movej([%f,%f,%f,%f,%f,%f], a=5, v=4, r=0)\r\n",
		radians[0],
		radians[1],
		radians[2],
		radians[3],
		radians[4],
		radians[5])
	if err != nil {
		return err
	}

	for {
		good := true
		for idx, r := range radians {
			if math.Round(r*100) != math.Round(arm.State.Joints[idx].Qactual*100) {
				//fmt.Printf("joint %d want: %f have: %f\n", idx, r, arm.State.Joints[idx].Qactual)
				good = false
			}
		}

		if good {
			return nil
		}

		time.Sleep(10 * time.Millisecond) // TODO: make responsive on new message
	}

}

func (arm *URArm) MoveToPositionC(c CartesianInfo) error {
	return arm.MoveToPosition(
		c.X,
		c.Y,
		c.Z,
		c.Rx,
		c.Ry,
		c.Rz,
	)
}

func (arm *URArm) MoveToPosition(x, y, z, rx, ry, rz float64) error {
	cmd := fmt.Sprintf("movej(get_inverse_kin(p[%f,%f,%f,%f,%f,%f]), a=1.4, v=4, r=0)\r\n", x, y, z, rx, ry, rz)
	//fmt.Println(cmd)
	_, err := arm.conn.Write([]byte(cmd))
	if err != nil {
		return err
	}

	retried := false

	slept := 0
	for {
		if math.Round(x*100) == math.Round(arm.State.CartesianInfo.X*100) &&
			math.Round(y*100) == math.Round(arm.State.CartesianInfo.Y*100) &&
			math.Round(z*100) == math.Round(arm.State.CartesianInfo.Z*100) &&
			math.Round(rx*20) == math.Round(arm.State.CartesianInfo.Rx*20) &&
			math.Round(ry*20) == math.Round(arm.State.CartesianInfo.Ry*20) &&
			math.Round(rz*20) == math.Round(arm.State.CartesianInfo.Rz*20) {
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
			return fmt.Errorf("can't reach position.\n want: %f %f %f %f %f %f\n   at: %f %f %f %f %f %f\n",
				x, y, z, rx, ry, rz,
				arm.State.CartesianInfo.X, arm.State.CartesianInfo.Y, arm.State.CartesianInfo.Z,
				arm.State.CartesianInfo.Rx, arm.State.CartesianInfo.Ry, arm.State.CartesianInfo.Rz)

		}
		time.Sleep(10 * time.Millisecond)

	}

}

func (arm *URArm) AddToLog(msg string) error {
	// TODO: check for " in msg
	cmd := fmt.Sprintf("textmsg(\"%s\")\r\n", msg)
	_, err := arm.conn.Write([]byte(cmd))
	return err
}

func reader(conn net.Conn, arm *URArm) {
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
			state, err := readRobotStateMessage(buf[1:])
			if err != nil {
				panic(err)
			}
			arm.State = state
			arm.haveData = true
			if arm.Debug {
				fmt.Printf("isOn: %v stopped: %v joints: %f %f %f %f %f %f cartesian: %f %f %f %f %f %f\n",
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
			err := readURRobotMessage(buf)
			if err != nil {
				panic(err)
			}
		case 5: // MODBUS_INFO_MESSAGE
			data := binary.BigEndian.Uint32(buf[1:])
			if data != 0 {
				fmt.Printf("got unexpected MODBUS_INFO_MESSAGE %d\n", data)
			}

		case 23: // SAFETY_SETUP_BROADCAST_MESSAGE
			break
		case 24: // SAFETY_COMPLIANCE_TOLERANCES_MESSAGE
			break
		case 25: // PROGRAM_STATE_MESSAGE
			if len(buf) != 12 {
				fmt.Println("got bad PROGRAM_STATE_MESSAGE ??")
			} else {
				a := binary.BigEndian.Uint32(buf[1:])
				b := buf[9]
				c := buf[10]
				d := buf[11]
				if a != 4294967295 || b != 1 || c != 0 || d != 0 {
					fmt.Printf("got unknown PROGRAM_STATE_MESSAGE %v %v %v %v\n", a, b, c, d)
				}
			}
		default:
			fmt.Printf("ur: unknown messageType: %v size: %d %v\n", buf[0], len(buf), buf)
		}
	}
}
