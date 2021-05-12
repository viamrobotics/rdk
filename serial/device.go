// Package serial provides utilities for searching for and working with serial based devices.
package serial

import (
	"io"

	goserial "github.com/jacobsa/go-serial/serial"
)

// DeviceDescription describes a specific serial device/
type DeviceDescription struct {
	Type DeviceType
	Path string
}

// DeviceType identifies a specific serial device type, like an arduino.
type DeviceType string

// The known device types.
const (
	DeviceTypeUnknown    = "unknown"
	DeviceTypeArduino    = "arduino"
	DeviceTypeJetson     = "nvidia-jetson"
	DeviceTypeNumatoGPIO = "numato-gpio"
)

// OpenDevice attempts to open a serial device on the given path. It's a variable
// in case you need to override it during tests.
var OpenDevice = func(devicePath string) (io.ReadWriteCloser, error) {
	options := goserial.OpenOptions{
		PortName:        devicePath,
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	device, err := goserial.Open(options)
	if err != nil {
		return nil, err
	}

	return device, nil
}
