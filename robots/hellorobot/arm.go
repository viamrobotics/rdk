package hellorobot

import (
	"math"
	"time"

	"github.com/sbinet/go-python"
)

type Arm struct {
	robot  *Robot
	armObj *python.PyObject
}

const armMoveSpeed = 1.0 / 4 // m/sec

func (a *Arm) MoveBy(meters float64) {
	a.armObj.CallMethod("move_by", python.PyFloat_FromDouble(meters))
	a.robot.pushCommand()
	time.Sleep(time.Duration(math.Ceil(math.Abs(meters)/armMoveSpeed)) * time.Second)
}
