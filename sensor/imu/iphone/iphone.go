// Package iPhone defines an IMU using sensor data provided by an iPhone.
package iphone

import (
	"bufio"
	"context"
	"encoding/json"
	"github.com/edaniels/golog"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/imu"
	"net"
	"strconv"
	"time"
)

// ModelName is used to register the sensor to a model name.
const ModelName = "iphone"

type iphoneMeasurement struct {
	RotationRateX *string `json:"motionRotationRateX"`
	RotationRateY *string `json:"motionRotationRateY"`
	RotationRateZ *string `json:"motionRotationRateZ"`
	Pitch *string `json:"motionPitch"`
	Roll *string `json:"motionRoll"`
	Yaw *string `json:"motionYaw"`
}

// init registers the iphone IMU type.
func init() {
	registry.RegisterSensor(imu.Type, ModelName, func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		return New(ctx, config.Host, logger)
	})
}

// IPhone is an iPhone based IMU.
type IPhone struct {
	reader *bufio.Reader // Read connection to iPhone to pull sensor data from.
}

// New returns a new IPhone IMU that that pulls data from the iPhone at host.
func New(ctx context.Context, host string, logger golog.Logger) (imu *IPhone, err error) {
	conn, err := net.DialTimeout("tcp", host, 3 * time.Second)
	if err != nil {
		return nil, err
	}

	// TODO: check if iPhone is actually sending necessary data? fail fast and all
	//       but also that would require the iphone to be on whenever this component is initialized.
	//       So when would that happen?
	if err = validateReceivingIMUData(conn); err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)

	return &IPhone{reader: r}, nil
}

func validateReceivingIMUData(conn net.Conn) error {
	return nil
}

// Desc returns a description of the compass.
func (ip *IPhone) Desc() sensor.Description {
	return sensor.Description{Type: imu.Type, Path: ""}
}

func (ip *IPhone) AngularVelocities(ctx context.Context) ([3]float64, error) {
	var ret [3]float64

	measurement, err := ip.reader.ReadString('\n')
	if err != nil {
		return ret, err
	}

	var imuReading iphoneMeasurement
	err = json.Unmarshal([]byte(measurement), &imuReading)
	if err != nil {
		return ret, err
	}

	if err = measurementToVelocityArr(imuReading, ret); err != nil {
		return ret, err
	}

	return ret, nil
}

func (ip *IPhone) Orientation(ctx context.Context) ([3]float64, error) {
	var ret [3]float64

	measurement, err := ip.reader.ReadString('\n')
	if err != nil {
		return ret, err
	}

	var imuReading iphoneMeasurement
	err = json.Unmarshal([]byte(measurement), &imuReading)
	if err != nil {
		return ret, err
	}

	if err = measurementToOrientationArr(imuReading, ret); err != nil {
		return ret, err
	}

	return ret, nil
}

func measurementToVelocityArr(meas iphoneMeasurement, arr [3]float64) error {
	if x, err := strconv.ParseFloat(*meas.RotationRateX, 64); err != nil {
		return err
	} else {
		arr[0] = x
	}

	if y, err := strconv.ParseFloat(*meas.RotationRateX, 64); err != nil {
		return err
	} else {
		arr[1] = y
	}

	if z, err := strconv.ParseFloat(*meas.RotationRateX, 64); err != nil {
		return err
	} else {
		arr[2] = z
	}

	return nil
}

func measurementToOrientationArr(meas iphoneMeasurement, arr [3]float64) error {
	if x, err := strconv.ParseFloat(*meas.Pitch, 64); err != nil {
		return err
	} else {
		arr[0] = x
	}

	if y, err := strconv.ParseFloat(*meas.Roll, 64); err != nil {
		return err
	} else {
		arr[1] = y
	}

	if z, err := strconv.ParseFloat(*meas.Yaw, 64); err != nil {
		return err
	} else {
		arr[2] = z
	}

	return nil
}
