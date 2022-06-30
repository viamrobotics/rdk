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

				if attr.HoldPos == nil {
					theServo.holdPos = true
				} else {
					theServo.holdPos = false
				}

				if attr.StartPos == nil { // sets and holds the starting position to the middle of servo range if StartPos is not set
					setPos := C.gpioServo(theServo.pin, C.uint(1500)) // a 1500ms pulsewidth positions the servo at 90 degrees
					if setPos != 0 {
						return nil, errors.Errorf("gpioServo failed with %d", setPos)
					}

				} else { // sets and holds the starting position to the user setting.

					setPos := C.gpioServo(theServo.pin, C.uint(angleToVal(uint8(*attr.StartPos))))
					if setPos != 0 {
						return nil, errors.Errorf("gpioServo failed with %d", setPos)
					}

				}

				return theServo, nil
			},
		},
	)
}

var _ = servo.LocalServo(&piPigpioServo{})

// piPigpioServo implements a servo.Servo using pigpio.
type piPigpioServo struct {
	generic.Unimplemented
	pin      C.uint
	min, max uint8
	opMgr    operation.SingleOperationManager
	val      float64
	holdPos  bool
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

	val := 500 + (2000.0 * float64(angle) / 180.0)
	res := C.gpioServo(s.pin, C.uint(val))

	s.val = val

	if res != 0 {
		switch res {
		case -93:
			return errors.Errorf("gpioservo pin %d is not set up to send and receive pulsewidths")
		case -7:
			return errors.Errorf("gpioservo on pin %d trying to reach out of range position", s.pin)
		default:
			return errors.Errorf("gpioServo on pin %d failed with %d", s.pin, res)
		}

	}

	if !s.holdPos { // the following logic disables a servo once it has reached a position or after a certain amount of time has been reached
		time.Sleep(500 * time.Millisecond) // time before a stop is sent
		setPos := C.gpioServo(s.pin, C.uint(0))
		// if setPos == C.int(pulseErr) {
		if setPos < 0 {
			return errors.Errorf("servo on pin %d failed with code %d", s.pin, setPos)
		}
	}
	return nil
}

func (s *piPigpioServo) GetPosition(ctx context.Context) (uint8, error) {
	res := C.gpioGetServoPulsewidth(s.pin)
	if res <= 0 {
		switch res {
		case -93:
			return 0, errors.Errorf("gpioservo pin %d is not set up to send and receive pulsewidths")
		case -7:
			return 0, errors.Errorf("gpioservo on pin %d trying to reach out of range position", s.pin)
		default:
			return 0, errors.Errorf("gpioServo on pin %d failed with %d", s.pin, res)
		}
	}
	return uint8(180 * (float64(res) - 500.0) / 2000), nil
}

func angleToVal(angle uint8) float64 {
	val := 500 + (2000.0 * float64(angle) / 180.0)
	return val
}

func valToAngle(val float64) uint8 {
	angle := 180 * (float64(val) - 500.0) / 2000.0
	return uint8(angle)
}

func (s *piPigpioServo) Stop(ctx context.Context) error {
	_, done := s.opMgr.New(ctx)
	defer done()
	getPos := C.gpioServo(s.pin, C.uint(0))
	if int(getPos) != int(0) {
		return errors.Errorf("gpioServo failed with %d", getPos)
	}
	return nil
}

func (s *piPigpioServo) IsMoving(ctx context.Context) (bool, error) {
	// RSDK-434: Refine implementation
	return s.opMgr.OpRunning(), nil
}
