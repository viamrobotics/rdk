// Package iphone defines an IMU and Compass using sensor data provided by an iPhone.
package iphone

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.viam.com/utils"

	"github.com/edaniels/golog"

	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/imu"
)

// A Measurement is a struct representing the data collected by the IPhone.
type Measurement struct {
	RotationRateX *float64 `json:"motionRotationRateX"`
	RotationRateY *float64 `json:"motionRotationRateY"`
	RotationRateZ *float64 `json:"motionRotationRateZ"`
	Pitch         *float64 `json:"motionPitch,string"`
	Roll          *float64 `json:"motionRoll,string"`
	Yaw           *float64 `json:"motionYaw,string"`
	Heading       *float64 `json:"locationTrueHeading,string"`
}

// IPhone is an iPhone based IMU.
type IPhone struct {
	host        string        // The host name of the iPhone being connected to.
	reader      *bufio.Reader // Read connection to iPhone to pull sensor data from.
	log         golog.Logger
	mut         *sync.RWMutex // Mutex to ensure only one goroutine or thread is reading from reader at a time.
	measurement atomic.Value  // The latest measurement value read from reader.
}

const (
	defaultTimeoutMs = 1000
)

// Desc returns a description of the IMU.
func (ip *IPhone) Desc() sensor.Description {
	return sensor.Description{Type: imu.Type, Path: ""}
}

// AngularVelocity returns an array of AngularVelocity data across x, y, and z axes.
func (ip *IPhone) AngularVelocity(ctx context.Context) ([3]float64, error) {
	var ret [3]float64

	imuReading := ip.measurement.Load().(Measurement)

	ret[0], ret[1], ret[2] = *imuReading.RotationRateX, *imuReading.RotationRateY, *imuReading.RotationRateZ

	return ret, nil
}

// Heading returns the heading of the IPhone based on the most recently received measurement.
func (ip *IPhone) Heading(ctx context.Context) (float64, error) {
	imuReading := ip.measurement.Load().(Measurement)
	return *imuReading.Heading, nil
}

// New returns a new IPhone IMU that that pulls data from the iPhone at host.
func New(ctx context.Context, host string, logger golog.Logger) (imu *IPhone, err error) {
	//conn, err := net.DialTimeout("tcp", host, defaultTimeoutMs*time.Millisecond)
	resp, err := http.Get(host)
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(resp.Body)
	ip := &IPhone{reader: r, log: logger, mut: &sync.RWMutex{}, host: host} //, conn: conn}

	imuReading, err := ip.readNextMeasurement(ctx)
	if err != nil {
		logger.Debugw("error reading iphone data", "error", err)
		return nil, err
	}

	ip.measurement.Store(*imuReading)

	utils.ManagedGo(func() {
		for {
			imuReading, err := ip.readNextMeasurement(ctx)
			if err != nil {
				logger.Debugw("error reading iphone data", "error", err)
			} else {
				ip.measurement.Store(*imuReading)
			}
		}
	}, func() {
	})

	return ip, nil
}

// Orientation returns an array of orientation data containing pitch, roll, and yaw.
func (ip *IPhone) Orientation(ctx context.Context) ([3]float64, error) {
	var ret [3]float64

	imuReading := ip.measurement.Load().(Measurement)

	ret[0], ret[1], ret[2] = *imuReading.Pitch, *imuReading.Roll, *imuReading.Yaw

	return ret, nil
}

// Readings returns an array containing:
// [0]: [3]float64 of angular velocities in rads/s
// [1]: [3]float64 of pitch, roll, yaw in rads
// [2]: float64 of the heading in degrees
func (ip *IPhone) Readings(ctx context.Context) ([]interface{}, error) {
	meas := ip.measurement.Load().(Measurement)
	var velo [3]float64
	var orient [3]float64
	var heading float64

	velo[0], velo[1], velo[2] = *meas.RotationRateX, *meas.RotationRateY, *meas.RotationRateZ
	orient[0], orient[1], orient[2] = *meas.Pitch, *meas.Roll, *meas.Yaw
	heading = *meas.Heading

	return []interface{}{velo, orient, heading}, nil
}

// StartCalibration does nothing.
func (ip *IPhone) StartCalibration(ctx context.Context) error {
	return nil
}

// StopCalibration does nothing.
func (ip *IPhone) StopCalibration(ctx context.Context) error {
	return nil
}

func (ip *IPhone) readNextMeasurement(ctx context.Context) (*Measurement, error) {
	timeout := time.Now().Add(defaultTimeoutMs * time.Millisecond)
	ctx, cancel := context.WithDeadline(ctx, timeout)
	defer cancel()

	ch := make(chan string, 1)
	go func() {
		ip.mut.Lock()
		defer ip.mut.Unlock()
		measurement, err := ip.reader.ReadString('\n')
		if err != nil {
			// If we can't pull the next measurement, it's possible the connection was lost. Try to get another.
			conn, err := net.DialTimeout("tcp", ip.host, defaultTimeoutMs*time.Millisecond)
			if err != nil {
				ip.log.Error("failed to reconnect to iphone", "error", err)
			}
			ip.reader = bufio.NewReader(conn)
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
