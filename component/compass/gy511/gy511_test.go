package gy511

import (
	"context"
	"io"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/serial"
	"go.viam.com/rdk/testutils/inject"
)

func TestDevice(t *testing.T) {
	logger := golog.NewTestLogger(t)
	_, err := New(context.Background(), "/", logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "directory")

	defaultOpenFunc := func(devicePath string) (io.ReadWriteCloser, error) {
		return nil, errors.Errorf("cannot open %s", devicePath)
	}
	prevOpenFunc := serial.Open
	var injectedOpenDeviceFunc func(devicePath string) io.ReadWriteCloser
	openDeviceFunc := defaultOpenFunc
	serial.Open = func(devicePath string) (io.ReadWriteCloser, error) {
		if injectedOpenDeviceFunc != nil {
			return injectedOpenDeviceFunc(devicePath), nil
		}
		return openDeviceFunc(devicePath)
	}
	defer func() {
		serial.Open = prevOpenFunc
	}()

	_, err = New(context.Background(), "/", logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot")

	injectedOpenDeviceFunc = func(_ string) io.ReadWriteCloser {
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
		}
	}

	_, err = New(context.Background(), "/", logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops2")
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops3")

	rd := NewRawGY511()
	injectedOpenDeviceFunc = func(_ string) io.ReadWriteCloser {
		rd.SetHeading(5)
		return rd
	}

	t.Run("normal device", func(t *testing.T) {
		dev, err := New(context.Background(), "/", logger)
		test.That(t, err, test.ShouldBeNil)
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
		test.That(t, dev.Close(), test.ShouldBeNil)
	})

	t.Run("failing to make device", func(t *testing.T) {
		rd.SetFailAfter(0)
		_, err = New(context.Background(), "/", logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "fail")
	})

	t.Run("failing to use device", func(t *testing.T) {
		rd.SetFailAfter(4)
		dev, err := New(context.Background(), "/", logger)
		test.That(t, err, test.ShouldBeNil)
		heading, err := dev.Heading(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, math.IsNaN(heading), test.ShouldBeFalse)
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
		test.That(t, math.IsNaN(readings[0].(float64)), test.ShouldBeFalse)
		test.That(t, dev.Close(), test.ShouldBeNil)
	})
}
