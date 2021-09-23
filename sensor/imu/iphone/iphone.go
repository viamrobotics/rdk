// Package iphone defines an IMU and Compass using sensor data provided by an iPhone.
package iphone

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/imu"
)

// An Measurement is a struct representing the data collected by the IPhone.
type Measurement struct {
	RotationRateX *float64 `json:"motionRotationRateX,string"`
	RotationRateY *float64 `json:"motionRotationRateY,string"`
	RotationRateZ *float64 `json:"motionRotationRateZ,string"`
	Pitch         *float64 `json:"motionPitch,string"`
	Roll          *float64 `json:"motionRoll,string"`
	Yaw           *float64 `json:"motionYaw,string"`
	Heading       *float64 `json:"locationHeadingZ,string"`
}

// IPhone is an iPhone based IMU.
type IPhone struct {
	// TODO: Our reader will be bufSize out of date at any given point. Maybe a problem?
	reader *bufio.Reader // Read connection to iPhone to pull sensor data from.
	log    golog.Logger
	mut    *sync.Mutex
}

// New returns a new IPhone IMU that that pulls data from the iPhone at host.
func New(host string, logger golog.Logger) (imu *IPhone, err error) {
	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)

	return &IPhone{reader: r, log: logger, mut: &sync.Mutex{}}, nil
}

// Desc returns a description of the IMU.
func (ip *IPhone) Desc() sensor.Description {
	return sensor.Description{Type: imu.Type, Path: ""}
}

// AngularVelocity returns an array of AngularVelocity data across x, y, and z axes.
func (ip *IPhone) AngularVelocity(ctx context.Context) ([3]float64, error) {
	var ret [3]float64

	imuReading, err := ip.readNextMeasurement(ctx)
	if err != nil {
		return ret, err
	}

	ret[0], ret[1], ret[2] = *imuReading.RotationRateX, *imuReading.RotationRateY, *imuReading.RotationRateZ

	return ret, nil
}

// Orientation returns an array of orientation data containing pitch, roll, and yaw.
func (ip *IPhone) Orientation(ctx context.Context) ([3]float64, error) {
	var ret [3]float64

	imuReading, err := ip.readNextMeasurement(ctx)
	if err != nil {
		return ret, err
	}

	ret[0], ret[1], ret[2] = *imuReading.Pitch, *imuReading.Roll, *imuReading.Yaw

	return ret, nil
}

// Heading returns the heading of the IPhone based on the most recently received measurement.
func (ip *IPhone) Heading(ctx context.Context) (float64, error) {
	imuReading, err := ip.readNextMeasurement(ctx)
	if err != nil {
		return 0, err
	}

	return *imuReading.Heading, nil
}

// StartCalibration does nothing.
func (ip *IPhone) StartCalibration(ctx context.Context) error {
	return nil
}

// StopCalibration does nothing.
func (ip *IPhone) StopCalibration(ctx context.Context) error {
	return nil
}

// TODO: maybe this should just constantly be running in the background pushing to some buffer, and the
//       actual AngularVelocity/Orientation methods can just read from it
func (ip *IPhone) readNextMeasurement(ctx context.Context) (*Measurement, error) {
	timeout := time.Now().Add(500 * time.Millisecond)
	ctx, cancel := context.WithDeadline(ctx, timeout)
	defer cancel()

	ch := make(chan string, 1)
	go func() {
		ip.mut.Lock()
		defer ip.mut.Unlock()
		measurement, err := ip.reader.ReadString('\n')
		if err != nil {
			ip.log.Errorf(err.Error(), err)
		}
		ch <- measurement
	}()

	select {
	case measurement := <-ch:
		var imuReading Measurement
		err := json.Unmarshal([]byte(measurement), &imuReading)
		if err != nil {
			return nil, err
		}

		return &imuReading, nil
	case <-ctx.Done():
		return nil, errors.New("timed out waiting for iphone measurement")
	}
}

// Readings returns an array containing:
// [0]: [3]float64 of angular velocities in rads/s
// [1]: [3]float64 of pitch, roll, yaw in rads
// [2]: float64 of the heading in degrees
func (ip *IPhone) Readings(ctx context.Context) ([]interface{}, error) {
	velo, err := ip.AngularVelocity(ctx)
	if err != nil {
		return nil, err
	}

	orient, err := ip.Orientation(ctx)
	if err != nil {
		return nil, err
	}

	heading, err := ip.Heading(ctx)
	if err != nil {
		return nil, err
	}

	return []interface{}{velo, orient, heading}, nil
}
