//go:build linux && arm64

package pi

// #include <stdlib.h>
// #include <pigpio.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

type servoConfig struct {
	Pin string `json:"pin"`
	Min int    `json:"min,omitempty"`
	Max int    `json:"max,omitempty"`
}

// init registers a pi servo based on pigpio.
func init() {
	registry.RegisterComponent(
		servo.Subtype,
		modelName,
		registry.Component{
			Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
				attr := config.ConvertedAttributes.(*servoConfig)

				if attr.Pin == "" {
					return nil, errors.New("need pin for pi servo")
				}

				bcom, have := broadcomPinFromHardwareLabel(attr.Pin)
				if !have {
					return nil, errors.Errorf("no hw mapping for %s", attr.Pin)
				}

				theServo := &piPigpioServo{C.uint(bcom)}
				if attr.Min > 0 {
					theServo.min = uint8(attr.Min)
				}
				if attr.Max > 0 {
					theServo.max = uint8(attr.Max)
				}
				return theServo, nil
			},
		},
	)
	config.RegisterComponentAttributeMapConverter(
		config.ComponentTypeServo,
		modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf servoConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}

// piPigpioServo implements a servo.Servo using pigpio.
type piPigpioServo struct {
	pin      C.uint
	min, max uint8
}

func (s *piPigpioServo) Move(ctx context.Context, angle uint8) error {
	if s.min > 0 && angle < s.min {
		angle = s.min
	}
	if s.max > 0 && angle > s.max {
		angle = s.max
	}
	val := 500 + (2000.0 * float64(angle) / 180.0)
	res := C.gpioServo(s.pin, C.uint(val))
	if res != 0 {
		return errors.Errorf("gpioServo failed with %d", res)
	}
	return nil
}

func (s *piPigpioServo) GetPosition(ctx context.Context) (uint8, error) {
	res := C.gpioGetServoPulsewidth(s.pin)
	if res <= 0 {
		// this includes, errors, we'll ignore
		return 0, nil
	}
	return uint8(180 * (float64(res) - 500.0) / 2000), nil
}
