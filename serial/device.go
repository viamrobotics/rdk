package serial

import (
	"io"

	goserial "github.com/jacobsa/go-serial/serial"
)

type DeviceDescription struct {
	Type DeviceType
	Path string
}

type DeviceType string

const (
	DeviceTypeUnknown = "unknown"
	DeviceTypeArduino = "arduino"
	DeviceTypeJetson  = "nvidia-jetson"
)

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
