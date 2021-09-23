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
	"time"
)

// ModelName is used to register the sensor to a model name.
const ModelName = "iphone"

type iphoneMeasurement struct {
	RotationRateX *float64 `json:"motionRotationRateX,string"`
	RotationRateY *float64 `json:"motionRotationRateY,string"`
	RotationRateZ *float64 `json:"motionRotationRateZ,string"`
	Pitch *float64 `json:"motionPitc,string"`
	Roll *float64 `json:"motionRoll,string"`
	Yaw *float64 `json:"motionYaw,string"`
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

// Desc returns a description of the IMU.
func (ip *IPhone) Desc() sensor.Description {
	return sensor.Description{Type: imu.Type, Path: ""}
}

func (ip *IPhone) AngularVelocities(ctx context.Context) ([3]float64, error) {
	var ret [3]float64

	imuReading, err := ip.readNextMeasurement()
	if err != nil {
		return ret, err
	}

	ret[0], ret[1], ret[2] = *imuReading.RotationRateX, *imuReading.RotationRateY, *imuReading.RotationRateZ

	return ret, nil
}

func (ip *IPhone) Orientation(ctx context.Context) ([3]float64, error) {
	var ret [3]float64

	imuReading, err := ip.readNextMeasurement()
	if err != nil {
		return ret, err
	}

	ret[0], ret[1], ret[2] = *imuReading.Pitch, *imuReading.Roll, *imuReading.Yaw

	return ret, nil
}

// TODO: maybe this should just constantly be running in the background pushing to some buffer, and the
//       actual AngularVelocity/Orientation methods can just read from it
func (ip *IPhone) readNextMeasurement() (*iphoneMeasurement, error) {
	measurement, err := ip.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	var imuReading *iphoneMeasurement
	err = json.Unmarshal([]byte(measurement), imuReading)
	if err != nil {
		return nil, err
	}

	return imuReading, nil
}

// Readings returns the currently predicted heading.
func (ip *IPhone) Readings(ctx context.Context) ([]interface{}, error) {
	velos, err := ip.AngularVelocities(ctx)
	if err != nil {
		return nil, err
	}

	orient, err := ip.Orientation(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{velos, orient}, nil
}
