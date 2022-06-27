//go:build linux && (arm64 || arm)

package piimpl

// #include <stdlib.h>
// #include <pigpio.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	picommon "go.viam.com/rdk/component/board/pi/common"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
)

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

				getPos := C.gpioGetServoPulsewidth(theServo.pin)
				fmt.Println(int(theServo.pin), "init pin is at", getPos)

				setPos := C.gpioServo(theServo.pin, C.uint(0))
				if setPos != 0 {
					return nil, errors.Errorf("gpioServo failed with %d", setPos)
				}

				getPos = C.gpioGetServoPulsewidth(theServo.pin)

				fmt.Println(int(theServo.pin), "set pin is at", getPos)

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

	getPos, err := s.GetPosition(ctx)
	if err != nil {
		return err
	}
	fmt.Println("init pos is", getPos)

	movePos := angleToVal(angle)
	moveVal := C.gpioServo(s.pin, C.uint(movePos))
	fmt.Println(int(s.pin), " val is ", moveVal)
	if moveVal != 0 {
		return errors.Errorf("gpioServo failed with %d", moveVal)
	}

	// setPos := C.gpioServo(s.pin, C.uint(0))
	// if setPos != 0 {
	// return errors.Errorf("gpioServo failed with %d", setPos)
	// }

	// fmt.Println("write pin to low")

	getPos, err = s.GetPosition(ctx)
	if err != nil {
		return err
	}
	fmt.Println("final pos is", getPos)

	// s.Stop(ctx)

	return nil
}

func angleToVal(angle uint8) float64 {
	val := 500 + (2000.0 * float64(angle) / 180.0)
	return val
}

func (s *piPigpioServo) GetPosition(ctx context.Context) (uint8, error) {
	res := C.gpioGetServoPulsewidth(s.pin)
	fmt.Println(int(s.pin), " res is", res)
	if res <= 0 {
		// this includes, errors, we'll ignore
		return 0, nil
	}
	return valToAngle(float64(res)), nil
}

func valToAngle(val float64) uint8 {
	angle := 180 * (float64(val) - 500.0) / 2000.0
	return uint8(angle)
}

func (s *piPigpioServo) Stop(ctx context.Context) error {
	ctx, done := s.opMgr.New(ctx)
	defer done()
	res := C.gpioServo(s.pin, C.uint(0))
	if res != 0 {
		return errors.Errorf("gpioServo failed with %d", res)
	}
	return nil
}

func (s *piPigpioServo) IsMoving() bool {
	return s.opMgr.OpRunning()
}
