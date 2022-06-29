//go:build linux && (arm64 || arm)

package piimpl

// #include <stdlib.h>
// #include <pigpio.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	picommon "go.viam.com/rdk/component/board/pi/common"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
)

// type AttrConfig {
// 	max
// 	minterface
// }

// func confgi Validate() {}

// init registers a pi servo based on pigpio.
func init() {
	registry.RegisterComponent(
		servo.Subtype,
		picommon.ModelName,
		registry.Component{
			Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
				attr, ok := config.ConvertedAttributes.(*picommon.ServoConfig)
				if !ok {
					return nil, errors.New("need servo configuration")
				}

				if attr.Pin == "" {
					return nil, errors.New("need pin for pi servo")
				}

				bcom, have := broadcomPinFromHardwareLabel(attr.Pin)
				if !have {
					return nil, errors.Errorf("no hw mapping for %s", attr.Pin)
				}

				theServo := &piPigpioServo{pin: C.uint(bcom)}
				if attr.Min > 0 {
					theServo.min = uint8(attr.Min)
				}
				if attr.Max > 0 {
					theServo.max = uint8(attr.Max)
				}

				theServo.res = C.int(0)

				setPos := C.gpioServo(theServo.pin, C.uint(1500))
				if setPos != 0 {
					return nil, errors.Errorf("gpioServo failed with %d", setPos)
				}

				theServo.res = C.int(0)

				return theServo, nil
			},
		},
	)
}

var _ = servo.LocalServo(&piPigpioServo{})

const pulseErr = -93

// piPigpioServo implements a servo.Servo using pigpio.
type piPigpioServo struct {
	generic.Unimplemented
	pin      C.uint
	min, max uint8
	opMgr    operation.SingleOperationManager
	res      C.int
	relaxPos bool
}

func (s *piPigpioServo) Move(ctx context.Context, angle uint8) error {
	_, done := s.opMgr.New(ctx)
	defer done()

	if s.min > 0 && angle < s.min {
		angle = s.min
	}

	if s.max > 0 && angle > s.max {
		angle = s.max

	}

	initVal := C.gpioServo(s.pin, C.uint(s.res))
	if initVal != 0 {
		return errors.Errorf("gpioServo failed with %d", initVal)
	}

	movePos := angleToVal(angle)
	moveVal := C.gpioServo(s.pin, C.uint(movePos))
	if moveVal == C.int(pulseErr) {
		return errors.Errorf("gpioServo pin not set up for Pulsewidths")
	}

	s.res = C.gpioGetServoPulsewidth(s.pin)

	if s.relaxPos {
		time.Sleep(500 * time.Millisecond)
		setPos := C.gpioServo(s.pin, C.uint(0))
		if setPos == C.int(pulseErr) {
			return errors.Errorf("gpioServo failed with %d", setPos)
		}
	}

	return nil
}

func angleToVal(angle uint8) float64 {
	val := 500 + (2000.0 * float64(angle) / 180.0)
	return val
}

func (s *piPigpioServo) GetPosition(ctx context.Context) (uint8, error) {
	s.res = C.gpioGetServoPulsewidth(s.pin)
	if s.res <= 0 {
		// ignores errors where res is -7 (bad pulsewidth, servo position out of bounds) or -93 (gpio not set up for pulsewidths by user)
		return 0, nil
	}
	return uint8(s.res), nil

	// return valToAngle(float64(s.res)), nil
}

func valToAngle(val float64) uint8 {
	angle := 180 * (float64(val) - 500.0) / 2000.0
	return uint8(angle)
}

func (s *piPigpioServo) Stop(ctx context.Context) error {
	ctx, done := s.opMgr.New(ctx)
	defer done()
	getPos := C.gpioServo(s.pin, C.uint(0))
	if getPos != 0 {
		return errors.Errorf("gpioServo failed with %d", getPos)
	}
	return nil
}

func (s *piPigpioServo) IsMoving() bool {
	return s.opMgr.OpRunning()
}
