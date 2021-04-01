package lidar_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/usb"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestRegistration(t *testing.T) {
	logger := golog.NewTestLogger(t)
	devType1 := lidar.DeviceType("1")
	devType2 := lidar.DeviceType("2")
	err1 := errors.New("whoops1")
	var capturedDesc lidar.DeviceDescription
	lidar.RegisterDeviceType(devType1, lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			capturedDesc = desc
			return nil, err1
		},
	})
	err2 := errors.New("whoops2")
	lidar.RegisterDeviceType(devType2, lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			capturedDesc = desc
			return nil, err2
		},
	})

	dev1Desc := lidar.DeviceDescription{Type: devType1, Path: "foo"}
	_, err := lidar.CreateDevice(context.Background(), dev1Desc, logger)
	test.That(t, err, test.ShouldEqual, err1)
	test.That(t, capturedDesc, test.ShouldResemble, dev1Desc)
	dev2Desc := lidar.DeviceDescription{Type: devType2, Path: "bar"}
	_, err = lidar.CreateDevice(context.Background(), dev2Desc, logger)
	test.That(t, err, test.ShouldEqual, err2)
	test.That(t, capturedDesc, test.ShouldResemble, dev2Desc)

	_, err = lidar.CreateDevices(context.Background(), []lidar.DeviceDescription{dev1Desc, dev2Desc}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, err1.Error())
	test.That(t, err.Error(), test.ShouldContainSubstring, err2.Error())

	injectDev2 := &inject.LidarDevice{}
	injectDev2.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	lidar.RegisterDeviceType(devType2, lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			capturedDesc = desc
			return injectDev2, nil
		},
	})
	_, err = lidar.CreateDevice(context.Background(), dev1Desc, logger)
	test.That(t, err, test.ShouldEqual, err1)
	test.That(t, capturedDesc, test.ShouldResemble, dev1Desc)
	dev, err := lidar.CreateDevice(context.Background(), dev2Desc, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dev, test.ShouldEqual, injectDev2)

	_, err = lidar.CreateDevices(context.Background(), []lidar.DeviceDescription{dev1Desc, dev2Desc}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, err1.Error())

	err3 := errors.New("whoops3")
	injectDev2.CloseFunc = func(ctx context.Context) error {
		return err3
	}
	_, err = lidar.CreateDevices(context.Background(), []lidar.DeviceDescription{dev1Desc, dev2Desc}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, err1.Error())
	test.That(t, err.Error(), test.ShouldContainSubstring, err3.Error())
	injectDev2.CloseFunc = func(ctx context.Context) error {
		return nil
	}

	devType := lidar.CheckProductDeviceIDs(1, 2)
	test.That(t, devType, test.ShouldEqual, lidar.DeviceTypeUnknown)
	lidar.RegisterDeviceType(devType2, lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			capturedDesc = desc
			return injectDev2, nil
		},
		USBInfo: &usb.Identifier{
			Vendor:  1,
			Product: 2,
		},
	})
	devType3 := lidar.DeviceType("3")
	lidar.RegisterDeviceType(devType3, lidar.DeviceTypeRegistration{
		USBInfo: &usb.Identifier{
			Vendor:  3,
			Product: 4,
		},
	})

	devType = lidar.CheckProductDeviceIDs(0, 1)
	test.That(t, devType, test.ShouldEqual, lidar.DeviceTypeUnknown)
	devType = lidar.CheckProductDeviceIDs(1, 2)
	test.That(t, devType, test.ShouldEqual, devType2)
	devType = lidar.CheckProductDeviceIDs(3, 4)
	test.That(t, devType, test.ShouldEqual, devType3)
}
