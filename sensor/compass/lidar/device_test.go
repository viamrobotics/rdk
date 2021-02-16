package lidar

import (
	"errors"
	"image"
	"testing"

	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/utils"

	"github.com/edaniels/test"
)

func TestNew(t *testing.T) {
	// unknown type
	_, err := New(lidar.DeviceDescription{Type: "what"})
	test.That(t, err, test.ShouldNotBeNil)

	devType := lidar.DeviceType(utils.RandomAlphaString(5))
	var newFunc func(desc lidar.DeviceDescription) (lidar.Device, error)
	lidar.RegisterDeviceType(devType, lidar.DeviceTypeRegistration{
		New: func(desc lidar.DeviceDescription) (lidar.Device, error) {
			return newFunc(desc)
		},
	})

	desc := lidar.DeviceDescription{Type: devType, Path: "somewhere"}
	newErr := errors.New("woof")
	newFunc = func(innerDesc lidar.DeviceDescription) (lidar.Device, error) {
		test.That(t, innerDesc, test.ShouldResemble, desc)
		return nil, newErr
	}

	_, err = New(desc)
	test.That(t, err, test.ShouldEqual, newErr)

	injectDev := &injectDevice{}
	newFunc = func(innerDesc lidar.DeviceDescription) (lidar.Device, error) {
		return injectDev, nil
	}

	dev, err := New(desc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dev, test.ShouldNotBeNil)
}

type injectDevice struct {
	lidar.Device
	StartFunc             func()
	StopFunc              func()
	CloseFunc             func() error
	ScanFunc              func(options lidar.ScanOptions) (lidar.Measurements, error)
	RangeFunc             func() int
	BoundsFunc            func() (image.Point, error)
	AngularResolutionFunc func() float64
}

func (ij *injectDevice) Start() {
	if ij.StartFunc == nil {
		ij.Device.Start()
	}
	ij.StartFunc()
}

func (ij *injectDevice) Stop() {
	if ij.StopFunc == nil {
		ij.Device.Stop()
	}
	ij.StopFunc()
}

func (ij *injectDevice) Close() error {
	if ij.CloseFunc == nil {
		return ij.Device.Close()
	}
	return ij.CloseFunc()
}

func (ij *injectDevice) Scan(options lidar.ScanOptions) (lidar.Measurements, error) {
	if ij.ScanFunc == nil {
		return ij.Device.Scan(options)
	}
	return ij.ScanFunc(options)
}

func (ij *injectDevice) Range() int {
	if ij.RangeFunc == nil {
		return ij.Device.Range()
	}
	return ij.RangeFunc()
}

func (ij *injectDevice) Bounds() (image.Point, error) {
	if ij.BoundsFunc == nil {
		return ij.Device.Bounds()
	}
	return ij.BoundsFunc()
}

func (ij *injectDevice) AngularResolution() float64 {
	if ij.AngularResolutionFunc == nil {
		return ij.Device.AngularResolution()
	}
	return ij.AngularResolutionFunc()
}
