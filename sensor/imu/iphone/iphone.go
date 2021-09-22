// Package iPhone defines an IMU using sensor data provided by an iPhone.
package iphone

import (
	"context"
	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/sensor/imu"
	"log"
	"net"
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

// ScanOptions modify how scan results will be produced and are subject
// to interpretation by the lidar implementation.
type ScanOptions struct {
	// Count determines how many scans to perform.
	Count int

	// NoFilter hints to the implementation to give as raw results as possible.
	// Normally an implementation may do some extra work to eliminate false
	// positives but this can be expensive and undesired.
	NoFilter bool
}

// init registers the iphone IMU type.
func init() {
	registry.RegisterSensor(imu.Type, ModelName, func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		return New(ctx, config.Host, logger)
	})
}

// IPhone is an iPhone based IMU.
type IPhone struct {
	conn net.Conn // Connection to iPhone to pull sensor data from.
}

// New returns a new IPhone IMU that that pulls data from the iPhone at host.
func New(ctx context.Context, host string, logger golog.Logger) (imu *IPhone, err error) {
	conn, err := getConn(host, 3 * time.Second)
	if err != nil {
		return nil, err
	}

	// TODO: check if iPhone is actually sending necessary data? fail fast and all

	return &IPhone{conn: conn}, nil
}



// Desc returns a description of the compass.
func (ip *IPhone) Desc() sensor.Description {
	return sensor.Description{Type: imu.Type, Path: ""}
}

// TODO: think i need to rework this. What if dial takes longer than timeout? this must be a solved problem,
//       overthinking
// Attempts to connect to host over tcp. Times out after timeoutMillis.
func getConn(host string, timeout time.Duration) (net.Conn, error) {
	var err error
	for start := time.Now(); time.Since(start) < timeout; {
		conn, err := net.Dial("tcp", host)
		if err == nil {
			return conn, nil
		}
	}
	return nil, err
}


