package arm

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"
)

func movej(conn net.Conn, base float64, shoulder float64, elbow float64, w1 float64, w2 float64, w3 float64) error {
	return movejRadians(conn, convertDtoR(base), convertDtoR(shoulder), convertDtoR(elbow), convertDtoR(w1), convertDtoR(w2), convertDtoR(w3))
}

func movejRadians(conn net.Conn, base float64, shoulder float64, elbow float64, w1 float64, w2 float64, w3 float64) error {
	//fmt.Printf("moving to %f %f %f %f %f %f\n", base, shoulder, elbow, w1, w2, w3)
	_, err := fmt.Fprintf(conn, "movej([%f,%f,%f,%f,%f,%f],a=0.1, v=0.1, t=0, r=0)\r\n", base, shoulder, elbow, w1, w2, w3)
	if err == nil {
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

var state RobotState

func moveToPosition(conn net.Conn, x, y, z, rx, ry, rz float64) error {
	_, err := fmt.Fprintf(conn, "movel(p[%f,%f,%f,%f,%f,%f], a=0.1, v=0.1, r=0)\r\n", x, y, z, rx, ry, rz)
	if err != nil {
		return err
	}

	for {
		if math.Round(x*100) == math.Round(state.CartesianInfo.X*100) &&
			math.Round(y*100) == math.Round(state.CartesianInfo.Y*100) &&
			math.Round(z*100) == math.Round(state.CartesianInfo.Z*100) &&
			math.Round(rx*100) == math.Round(state.CartesianInfo.Rx*100) &&
			math.Round(ry*100) == math.Round(state.CartesianInfo.Ry*100) &&
			math.Round(rz*100) == math.Round(state.CartesianInfo.Rz*100) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}

}

func reader(conn net.Conn) {
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

		if buf[0] == 16 {
			state, err = readRobotStateMessage(buf[1:])
			if err != nil {
				panic(err)
			}
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
	}
}

func main() {
	fmt.Println("hi")

	conn, err := net.Dial("tcp", "192.168.2.155:30001")
	if err != nil {
		panic(err)
	}

	go reader(conn)

	time.Sleep(500 * time.Millisecond)

	rx := -math.Pi
	ry := 0.0
	rz := 0.0

	moveToPosition(conn, .3, .4, 0.3, rx, ry, rz) // desk back left
	moveToPosition(conn, .3, -.15, 0.3, rx, ry, rz) // desk front left
	moveToPosition(conn, .6, -.15, 0.3, rx, ry, rz) // desck front right
	moveToPosition(conn, .6, .4, 0.3, rx, ry, rz) // desk back right

	moveToPosition(conn, .45, .1, 0, rx, ry, rz)   // desk middle down
	moveToPosition(conn, .45, .1, 0.5, rx, ry, rz) // desk middle up

}
