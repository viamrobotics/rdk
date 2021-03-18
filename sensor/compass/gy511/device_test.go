package gy511

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"testing"
	"time"

	"go.viam.com/robotcore/serial"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestDevice(t *testing.T) {
	logger := golog.NewTestLogger(t)
	_, err := New(context.Background(), "/", logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "directory")

	defaultOpenDeviceFunc := func(devicePath string) (io.ReadWriteCloser, error) {
		return nil, fmt.Errorf("cannot open %s", devicePath)
	}
	prevOpenDeviceFunc := serial.OpenDevice
	openDeviceFunc := defaultOpenDeviceFunc
	serial.OpenDevice = func(devicePath string) (io.ReadWriteCloser, error) {
		if openDeviceFunc == nil {
			return prevOpenDeviceFunc(devicePath)
		}
		return openDeviceFunc(devicePath)
	}
	defer func() {
		serial.OpenDevice = prevOpenDeviceFunc
	}()

	_, err = New(context.Background(), "/", logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot")

	openDeviceFunc = func(devicePath string) (io.ReadWriteCloser, error) {
		return &inject.ReadWriteCloser{
			ReadFunc: func(p []byte) (int, error) {
				return 0, errors.New("whoops1")
			},
			WriteFunc: func(p []byte) (int, error) {
				return 0, errors.New("whoops2")
			},
			CloseFunc: func() error {
				return errors.New("whoops3")
			},
		}, nil
	}

	_, err = New(context.Background(), "/", logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops2")
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops3")

	rd := &RawDevice{}
	openDeviceFunc = func(devicePath string) (io.ReadWriteCloser, error) {
		rd.SetHeading(5)
		return rd, nil
	}

	t.Run("normal device", func(t *testing.T) {
		dev, err := New(context.Background(), "/", logger)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(time.Second)
		heading, err := dev.Heading(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, heading, test.ShouldEqual, 5)
		test.That(t, dev.StartCalibration(context.Background()), test.ShouldBeNil)
		heading, err = dev.Heading(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, math.IsNaN(heading), test.ShouldBeTrue)
		test.That(t, dev.StopCalibration(context.Background()), test.ShouldBeNil)
		readings, err := dev.Readings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings, test.ShouldResemble, []interface{}{5.0})
		test.That(t, dev.Close(context.Background()), test.ShouldBeNil)
	})

	t.Run("failing to make device", func(t *testing.T) {
		rd.SetFail(true)
		_, err = New(context.Background(), "/", logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "fail")
	})

	t.Run("failing to use device", func(t *testing.T) {
		rd.SetFail(false)
		dev, err := New(context.Background(), "/", logger)
		test.That(t, err, test.ShouldBeNil)
		rd.SetFail(true)
		time.Sleep(time.Second)
		heading, err := dev.Heading(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, math.IsNaN(heading), test.ShouldBeTrue)
		err = dev.StartCalibration(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "fail")
		heading, err = dev.Heading(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, math.IsNaN(heading), test.ShouldBeTrue)
		err = dev.StopCalibration(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "fail")
		readings, err := dev.Readings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings, test.ShouldHaveLength, 1)
		test.That(t, math.IsNaN(readings[0].(float64)), test.ShouldBeTrue)
		test.That(t, dev.Close(context.Background()), test.ShouldBeNil)
	})
}
