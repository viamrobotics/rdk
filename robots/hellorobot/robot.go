package hellorobot

import (
	"github.com/sbinet/go-python"
)

const ModelName = "hellorobot"

func init() {
	err := python.Initialize()
	if err != nil {
		panic(err.Error())
	}
}

type Robot struct {
	robotObj *python.PyObject
}

func New() *Robot {
	transportMod := python.PyImport_ImportModule("stretch_body.transport")
	transportMod.SetAttr(python.PyString_FromString("dbg_on"), python.PyInt_FromLong(0))
	robotMod := python.PyImport_ImportModule("stretch_body.robot")
	robot := robotMod.CallMethod("Robot")
	return &Robot{robotObj: robot}
}

func (r *Robot) Startup() {
	r.robotObj.CallMethod("startup")
}

func (r *Robot) Stop() {
	r.robotObj.CallMethod("stop")
}

func (r *Robot) Home() {
	r.robotObj.CallMethod("home")
}

func (r *Robot) pushCommand() {
	r.robotObj.CallMethod("push_command")
}

func (r *Robot) Base() *Base {
	base := r.robotObj.GetAttrString("base")
	return &Base{robot: r, baseObj: base}
}

func (r *Robot) Arm() *Arm {
	arm := r.robotObj.GetAttrString("arm")
	return &Arm{robot: r, armObj: arm}
}
