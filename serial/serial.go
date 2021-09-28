// Package serial provides utilities for searching for and working with serial based devices.
package serial

import (
	"io"
	"time"

	"github.com/go-errors/errors"

	ser "go.bug.st/serial"
)

// Description describes a specific serial device/
type Description struct {
	Type Type
	Path string
}

// Type identifies a specific serial device type, like an arduino.
type Type string

// The known device types.
const (
	TypeUnknown    = "unknown"
	TypeArduino    = "arduino"
	TypeJetson     = "nvidia-jetson"
	TypeNumatoGPIO = "numato-gpio"
)

// Options to be passed to Open(), closly mirrors goserial.OpenOptions
type Options struct {
	BaudRate          int
	DataBits          int
	StopBits          StopBits
	RTSCTSFlowControl bool
	ReadTimeout       int
	Parity            Parity
}

// Parity describes a serial port parity setting
type Parity int

const (
	// NoParity disable parity control (default)
	NoParity Parity = iota
	// OddParity enable odd-parity check
	OddParity
	// EvenParity enable even-parity check
	EvenParity
	// MarkParity enable mark-parity (always 1) check
	MarkParity
	// SpaceParity enable space-parity (always 0) check
	SpaceParity
)

// StopBits describe a serial port stop bits setting
type StopBits int

const (
	// OneStopBit sets 1 stop bit (default)
	OneStopBit StopBits = iota
	// OnePointFiveStopBits sets 1.5 stop bits
	OnePointFiveStopBits
	// TwoStopBits sets 2 stop bits
	TwoStopBits
)

// Open attempts to open a serial device on the given path. It's a variable
// in case you need to override it during tests.
var Open = func(devicePath string, options Options) (io.ReadWriteCloser, error) {

	mode := &ser.Mode{
		BaudRate: options.BaudRate,
		Parity:   ser.Parity(options.Parity),
		DataBits: options.DataBits,
		StopBits: ser.StopBits(options.StopBits),
	}

	device, err := ser.Open(devicePath, mode)
	if err != nil {
		return nil, err
	}
	err = device.SetReadTimeout(time.Duration(options.ReadTimeout) * time.Millisecond)
	if err != nil {
		return nil, err
	}

	return device, nil
}

// SetOptions to change the cofiguration of a serial port already open
var SetOptions = func(b io.ReadWriteCloser, options Options) error {
	mode := &ser.Mode{
		BaudRate: options.BaudRate,
		Parity:   ser.Parity(options.Parity),
		DataBits: options.DataBits,
		StopBits: ser.StopBits(options.DataBits),
	}
	p, ok := b.(ser.Port)
	if !ok {
		return errors.New("couldn't convert to underlying Port interface")
	}
	err := p.SetMode(mode)
	if err != nil {
		return err
	}
	return nil
}
