package board

import (
	"io"
)

// SerialGPIOBoard is a merger of SerialBoard and GPIOBoard
type SerialGPIOBoard interface {
	SerialNames() []string
	Serial(name string) Serial
	GPIOBoard
}

// SerialBoard is a board that supports one or more shareable serial devices.
type SerialBoard interface {
	SerialNames() []string
	Serial(name string) Serial
}

// Serial represents a shareable serial port on the board (for modbus, RS485, and other chainable serial devices.)
// Port options (baudrate, data/stop bits, etc.) are controlled by the config.json
// Mutex (lock/unlock) functions are provided, and should always be locked before reading/writing and unlocked afterward.
type Serial interface {
	Lock()
	Unlock()
	io.ReadWriteCloser
}
