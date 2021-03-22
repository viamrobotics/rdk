package hellorobot

import (
	"errors"

	"github.com/sbinet/go-python"

	"go.viam.com/robotcore/api"
)

const ModelName = "hellorobot"

func init() {
	err := python.Initialize()
	if err != nil {
		panic(err.Error())
	}
	api.RegisterProvider(ModelName, func(r api.Robot, config api.Component) (api.Provider, error) {
		return New()
	})
}

type Robot struct {
	robotObj *python.PyObject
}

func checkPythonErr() error {
	exc, val, _ := python.PyErr_Fetch()
	if exc == nil || exc.GetCPointer() == nil {
		return nil
	}
	if val != nil {
		return errors.New(val.String())
	}
	return errors.New(exc.String())
}

func New() (*Robot, error) {
	transportMod := python.PyImport_ImportModule("stretch_body.transport")
	if err := checkPythonErr(); err != nil {
		return nil, err
	}
	transportMod.SetAttr(python.PyString_FromString("dbg_on"), python.PyInt_FromLong(0))
	robotMod := python.PyImport_ImportModule("stretch_body.robot")
	if err := checkPythonErr(); err != nil {
		return nil, err
	}
	robot := robotMod.CallMethod("Robot")
	if err := checkPythonErr(); err != nil {
		return nil, err
	}
	return &Robot{robotObj: robot}, nil
}

func (r *Robot) Ready(theRobot api.Robot) error {
	return nil
}

func (r *Robot) pushCommand() error {
	r.robotObj.CallMethod("push_command")
	return checkPythonErr()
}

func (r *Robot) Base() (*Base, error) {
	base := r.robotObj.GetAttrString("base")
	return &Base{robot: r, baseObj: base}, checkPythonErr()
}
