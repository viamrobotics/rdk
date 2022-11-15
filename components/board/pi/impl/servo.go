//go:build linux && (arm64 || arm)

package piimpl

// #include <stdlib.h>
// #include <pigpio.h>
// #cgo LDFLAGS: -lpigpio
// #include "pi.h"
import "C"

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	picommon "go.viam.com/rdk/components/board/pi/common"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
)

var holdTime = 250000000 // 250ms in nanoseconds

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

				theServo.pinname = attr.Pin

				if attr.StartPos == nil {
					setPos := C.gpioServo(theServo.pin, C.uint(1500)) // a 1500ms pulsewidth positions the servo at 90 degrees
					if setPos != 0 {
						return nil, errors.Errorf("gpioServo failed with %d", setPos)
					}
				} else {
					setPos := C.gpioServo(theServo.pin, C.uint(angleToPulseWidth(int(*attr.StartPos))))
					if setPos != 0 {
						return nil, errors.Errorf("gpioServo failed with %d", setPos)
					}
				}
				if attr.HoldPos == nil || *attr.HoldPos {
					theServo.holdPos = true
				} else {
					theServo.res = C.gpioGetServoPulsewidth(theServo.pin)
					theServo.holdPos = false
					C.gpioServo(theServo.pin, C.uint(0)) // disables servo
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
	pin        C.uint
	pinname    string
	res        C.int
	min, max   uint8
	opMgr      operation.SingleOperationManager
	pulseWidth int // pulsewidth value, 500-2500us is 0-180 degrees, 0 is off
	holdPos    bool
}

// Move moves the servo to the given angle (0-180 degrees)
// This will block until done or a new operation cancels this one
func (s *piPigpioServo) Move(ctx context.Context, angle uint8, extra map[string]interface{}) error {
	ctx, done := s.opMgr.New(ctx)
	defer done()

	if s.min > 0 && angle < s.min {
		angle = s.min
	}
	if s.max > 0 && angle > s.max {
		angle = s.max
	}

	pulseWidth := angleToPulseWidth(int(angle))
	res := C.gpioServo(s.pin, C.uint(pulseWidth))

	s.pulseWidth = pulseWidth

	if res != 0 {
		err := s.pigpioErrors(int(res))
		return err
	}

	utils.SelectContextOrWait(ctx, time.Duration(pulseWidth)*time.Microsecond) // duration of pulswidth send on pin and servo moves

	if !s.holdPos { // the following logic disables a servo once it has reached a position or after a certain amount of time has been reached
		time.Sleep(time.Duration(holdTime)) // time before a stop is sent
		setPos := C.gpioServo(s.pin, C.uint(0))
		if setPos < 0 {
			return errors.Errorf("servo on pin %s failed with code %d", s.pinname, setPos)
		}
	}
	return nil
}

// returns piGPIO specific errors to user
func (s *piPigpioServo) pigpioErrors(res int) error {
	switch {
	case res == C.PI_NOT_SERVO_GPIO:
		return errors.Errorf("gpioservo pin %s is not set up to send and receive pulsewidths", s.pinname)
	case res == C.PI_BAD_PULSEWIDTH:
		return errors.Errorf("gpioservo on pin %s trying to reach out of range position", s.pinname)
	case res == 0:
		return nil
	case res < 0 && res != C.PI_BAD_PULSEWIDTH && res != C.PI_NOT_SERVO_GPIO:
		return errors.Errorf("gpioServo on pin %s failed with %d", s.pinname, res)
	default:
		return nil
	}
}

// Position returns the current set angle (degrees) of the servo.
func (s *piPigpioServo) Position(ctx context.Context, extra map[string]interface{}) (uint8, error) {
	res := C.gpioGetServoPulsewidth(s.pin)
	err := s.pigpioErrors(int(res))
	if int(res) != 0 {
		s.res = res
	}
	if err != nil {
		return 0, err
	}
	return uint8(pulseWidthToAngle(int(s.res))), nil
}

// angleToPulseWidth changes the input angle in degrees
// into the corresponding pulsewidth value in microsecond
func angleToPulseWidth(angle int) int {
	pulseWidth := 500 + (2000 * angle / 180)
	return pulseWidth
}

// pulseWidthToAngle changes the pulsewidth value in microsecond
// to the corresponding angle in degrees
func pulseWidthToAngle(pulseWidth int) int {
	angle := 180 * (pulseWidth + 1 - 500) / 2000
	return angle
}

// Stop stops the servo. It is assumed the servo stops immediately.
func (s *piPigpioServo) Stop(ctx context.Context, extra map[string]interface{}) error {
	_, done := s.opMgr.New(ctx)
	defer done()
	getPos := C.gpioServo(s.pin, C.uint(0))
	if int(getPos) != int(0) {
		return errors.Errorf("gpioServo failed with %d", getPos)
	}
	return nil
}

func (s *piPigpioServo) IsMoving(ctx context.Context) (bool, error) {
	err := s.pigpioErrors(int(s.res))
	if err != nil {
		return false, err
	}
	if int(s.res) == 0 {
		return false, nil
	}
	return s.opMgr.OpRunning(), nil
}
