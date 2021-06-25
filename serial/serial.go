// Package serial provides utilities for searching for and working with serial based devices.
package serial

import (
	"io"

	goserial "github.com/jacobsa/go-serial/serial"
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
	BaudRate          uint
	DataBits          uint
	StopBits          uint
	RTSCTSFlowControl bool

	// An inter-character timeout value, in milliseconds, and a minimum number of
	// bytes to block for on each read. A call to Read() that otherwise may block
	// waiting for more data will return immediately if the specified amount of
	// time elapses between successive bytes received from the device or if the
	// minimum number of bytes has been exceeded.
	//
	// Note that the inter-character timeout value may be rounded to the nearest
	// 100 ms on some systems, and that behavior is undefined if calls to Read
	// supply a buffer whose length is less than the minimum read size.
	//
	// Behaviors for various settings for these values are described below. For
	// more information, see the discussion of VMIN and VTIME here:
	//
	//     http://www.unixwiz.net/techtips/termios-vmin-vtime.html
	//
	// InterCharacterTimeout = 0 and MinimumReadSize = 0 (the default):
	//     This arrangement is not legal; you must explicitly set at least one of
	//     these fields to a positive number. (If MinimumReadSize is zero then
	//     InterCharacterTimeout must be at least 100.)
	//
	// InterCharacterTimeout > 0 and MinimumReadSize = 0
	//     If data is already available on the read queue, it is transferred to
	//     the caller's buffer and the Read() call returns immediately.
	//     Otherwise, the call blocks until some data arrives or the
	//     InterCharacterTimeout milliseconds elapse from the start of the call.
	//     Note that in this configuration, InterCharacterTimeout must be at
	//     least 100 ms.
	//
	// InterCharacterTimeout > 0 and MinimumReadSize > 0
	//     Calls to Read() return when at least MinimumReadSize bytes are
	//     available or when InterCharacterTimeout milliseconds elapse between
	//     received bytes. The inter-character timer is not started until the
	//     first byte arrives.
	//
	// InterCharacterTimeout = 0 and MinimumReadSize > 0
	//     Calls to Read() return only when at least MinimumReadSize bytes are
	//     available. The inter-character timer is not used.
	//
	// For windows usage, these options (termios) do not conform well to the
	//     windows serial port / comms abstractions.  Please see the code in
	//		 open_windows setCommTimeouts function for full documentation.
	//   	 Summary:
	//			Setting MinimumReadSize > 0 will cause the serialPort to block until
	//			until data is available on the port.
	//			Setting IntercharacterTimeout > 0 and MinimumReadSize == 0 will cause
	//			the port to either wait until IntercharacterTimeout wait time is
	//			exceeded OR there is character data to return from the port.
	//

	InterCharacterTimeout uint
	MinimumReadSize       uint
}

// Open attempts to open a serial device on the given path. It's a variable
// in case you need to override it during tests.
var Open = func(devicePath string, options Options) (io.ReadWriteCloser, error) {

	openOptions := goserial.OpenOptions{
		PortName:              devicePath,
		BaudRate:              options.BaudRate,
		DataBits:              options.DataBits,
		StopBits:              options.StopBits,
		InterCharacterTimeout: options.InterCharacterTimeout,
		MinimumReadSize:       options.MinimumReadSize,
	}

	device, err := goserial.Open(openOptions)
	if err != nil {
		return nil, err
	}

	return device, nil
}
