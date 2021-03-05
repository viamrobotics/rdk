package hellorobot

import (
	"math"

	"go.viam.com/robotcore/utils"

	"github.com/sbinet/go-python"
)

type Compass struct {
	robot   *Robot
	pimuObj *python.PyObject
}

func (c *Compass) Readings() ([]interface{}, error) {
	heading, err := c.Heading()
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (c *Compass) Heading() (float64, error) {
	c.pimuObj.CallMethod("pull_status")
	if err := checkPythonErr(); err != nil {
		return math.NaN(), err
	}
	imuStatus := c.pimuObj.GetAttrString("imu").CallMethod("get_status")
	if err := checkPythonErr(); err != nil {
		return math.NaN(), err
	}
	headingObj := python.PyDict_GetItem(imuStatus, python.PyString_FromString("heading"))
	if err := checkPythonErr(); err != nil {
		return math.NaN(), err
	}
	value := python.PyFloat_AsDouble(headingObj)
	if err := checkPythonErr(); err != nil {
		return math.NaN(), err
	}
	return utils.RadToDeg(value), nil
}
