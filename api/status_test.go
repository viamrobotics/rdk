package api_test

import (
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/test"
)

func TestGetSensorDeviceType(t *testing.T) {
	a := &inject.Compass{}
	test.That(t, api.GetSensorDeviceType(a), test.ShouldEqual, compass.DeviceType)
	b := &inject.RelativeCompass{}
	test.That(t, api.GetSensorDeviceType(b), test.ShouldEqual, compass.RelativeDeviceType)
	test.That(t, api.GetSensorDeviceType(someSensor{}), test.ShouldEqual, "")
}

type someSensor struct {
	sensor.Device
}
