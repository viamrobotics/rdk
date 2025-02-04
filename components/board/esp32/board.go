// Package esp32 exists for the sole purpose of exposing the esp32 as a micro-rdk configuration in app.viam.com
// The ESP32 is supported by the micro-rdk only (https://github.com/viamrobotics/micro-rdk)
package esp32

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var (
	espModel       = resource.DefaultModelFamily.WithModel("esp32")
	errUnsupported = errors.New("The ESP32 board is not supported in the RDK. " +
		"Please use with the micro-rdk (https://github.com/viamrobotics/micro-rdk).")
)

type digitalInterruptConfig struct {
	Pin int `json:"pin"`
}

type analogConfig struct {
	Name string `json:"name"`
	Pin  int    `json:"pin"`
}

type i2CConfig struct {
	Name     string `json:"name"`
	Bus      string `json:"bus"`
	DataPin  int    `json:"data_pin,omitempty"`
	ClockPin int    `json:"clock_pin,omitempty"`
	BaudRate int    `json:"baudrate_hz,omitempty"`
	Timeout  int    `json:"timeout_ns,omitempty"`
}

// Config mirrors the config structure in (https://github.com/viamrobotics/micro-rdk/blob/main/micro-rdk/src/esp32/board.rs).
type Config struct {
	Pins              []int                    `json:"pins,omitempty"`
	DigitalInterrupts []digitalInterruptConfig `json:"digital_interrupts,omitempty"`
	Analogs           []analogConfig           `json:"analogs,omitempty"`
	I2Cs              []i2CConfig              `json:"i2cs,omitempty"`
}

func init() {
	resource.RegisterComponent(
		board.API,
		espModel,
		resource.Registration[board.Board, *Config]{
			Constructor: newEsp32Board,
		})
}

// Validate for esp32 will always return an unsupported error.
func (conf *Config) Validate(path string) ([]string, error) {
	return []string{}, errUnsupported
}

func newEsp32Board(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (board.Board, error) {
	return nil, errUnsupported
}
