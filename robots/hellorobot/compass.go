package hellorobot

import (
	"github.com/viamrobotics/robotcore/utils"

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
	imuStatus := c.pimuObj.GetAttrString("imu").CallMethod("get_status")
	headingObj := python.PyDict_GetItem(imuStatus, python.PyString_FromString("heading"))
	value := python.PyFloat_AsDouble(headingObj)
	return utils.RadToDeg(value), nil
}
