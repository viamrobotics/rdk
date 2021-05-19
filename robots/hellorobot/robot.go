// Package hellorobot implements the Stretch robot from hello robot.
package hellorobot

import (
	"context"
	"fmt"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"
	"github.com/sbinet/go-python"

	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

// ModelName is the name we use for referring to the hello robot.
const ModelName = "hellorobot"

func init() {
	err := python.Initialize()
	if err != nil {
		panic(err.Error())
	}
	registry.RegisterProvider(ModelName, func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (robot.Provider, error) {
		return New()
	})
}

// Robot represents a physical hello robot.
type Robot struct {
	robotObj *python.PyObject
	logger   golog.Logger
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

// New returns a new instance of a hello robot.
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

// Ready does nothing.
func (r *Robot) Ready(theRobot robot.Robot) error {
	return nil
}

// Reconfigure replaces this provider with the given provider.
func (r *Robot) Reconfigure(newProvider robot.Provider) {
	actual, ok := newProvider.(*Robot)
	if !ok {
		panic(fmt.Errorf("expected new provider to be %T but got %T", actual, newProvider))
	}
	*r = *actual
}

func (r *Robot) pushCommand() error {
	r.robotObj.CallMethod("push_command")
	return checkPythonErr()
}

// Base returns the base of the hello robot.
func (r *Robot) Base() (*Base, error) {
	base := r.robotObj.GetAttrString("base")
	return &Base{robot: r, baseObj: base}, checkPythonErr()
}
