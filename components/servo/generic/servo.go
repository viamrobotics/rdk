// Package servogeneric implements a generic servo
package servogeneric

import (
	"context"
	"math"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
)

const (
	minDeg     float64 = 0.0
	maxDeg     float64 = 180.0
	minWidthUs uint    = 500  // absolute minimum pwm width
	maxWidthUs uint    = 2500 // absolute maximum pwm width
)

type servoConfig struct {
	Pin   string `json:"pin"`   // Pin a GPIO pin with pwm capabilities
	Board string `json:"board"` // Board a board that exposes GPIO pins
	// MinDeg minimum angle the servo can reach, note this doesn't affect PWM calculation
	MinDeg *float64 `json:"min_angle_deg,omitempty"`
	// MaxDeg maximum angle the servo can reach, note this doesn't affect PWM calculation
	MaxDeg *float64 `json:"max_angle_deg,omitempty"`
	// StartPos starting position of the servo in degree
	StartPos *float64 `json:"starting_position_deg,omitempty"`
	// Frequency when set the servo driver will attempt to change the GPIO pin's Frequency
	Frequency *uint `json:"frequency_hz,omitempty"`
	// Resolution resolution of the PWM driver (eg number of ticks for a full period) if left or 0
	// the driver will attempt to estimate the resolution
	Resolution *uint `json:"pwm_resolution"`
	// MinWidthUS override the safe minimum width in us this affect PWM calculation
	MinWidthUS *uint `json:"min_width_us"`
	// MaxWidthUS Override the safe maximum width in us this affect PWM calculation
	MaxWidthUS *uint `json:"max_width_us"`
}

// Validate ensures all parts of the config are valid.
func (config *servoConfig) Validate(path string) ([]string, error) {
	var deps []string
	if config.Board == "" {
		return nil, viamutils.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, config.Board)
	if config.Pin == "" {
		return nil, viamutils.NewConfigValidationFieldRequiredError(path, "pin")
	}
	if config.StartPos != nil {
		if config.MinDeg != nil && *config.StartPos < *config.MinDeg {
			return nil, viamutils.NewConfigValidationError(path, errors.New("starting_position_degs cannot be lower than min_angle_deg"))
		}
		if config.MaxDeg != nil && *config.StartPos > *config.MaxDeg {
			return nil, viamutils.NewConfigValidationError(path, errors.New("starting_position_degs cannot be higher than max_angle_deg"))
		}
		if *config.StartPos < minDeg || *config.StartPos > maxDeg {
			return nil, viamutils.NewConfigValidationError(path, errors.New("starting_position_degs should be between 0 and 180"))
		}
	}
	if config.MinDeg != nil && *config.MinDeg < minDeg {
		return nil, viamutils.NewConfigValidationError(path, errors.Errorf("min_angle_deg cannot be lower than %f", minDeg))
	}
	if config.MaxDeg != nil && *config.MaxDeg > maxDeg {
		return nil, viamutils.NewConfigValidationError(path, errors.Errorf("max_angle_deg cannot be higher than %f", maxDeg))
	}
	if config.MinWidthUS != nil && *config.MinWidthUS < minWidthUs {
		return nil, viamutils.NewConfigValidationError(path, errors.Errorf("min_width_us cannot be lower than %d", minWidthUs))
	}
	if config.MaxWidthUS != nil && *config.MaxWidthUS > maxWidthUs {
		return nil, viamutils.NewConfigValidationError(path, errors.Errorf("max_width_us cannot be higher than %d", maxWidthUs))
	}
	return deps, nil
}

const model = "generic"

func init() {
	registry.RegisterComponent(servo.Subtype, model,
		registry.Component{
			Constructor: newGenericServo,
		})
	config.RegisterComponentAttributeMapConverter(servo.SubtypeName, model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr servoConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		},
		&servoConfig{})
}

type servoGeneric struct {
	generic.Unimplemented
	pin       board.GPIOPin
	min       float64
	max       float64
	logger    golog.Logger
	opMgr     operation.SingleOperationManager
	frequency uint
	minUs     uint
	maxUs     uint
	pwmRes    uint
	currPct   float64
}

func newGenericServo(ctx context.Context, deps registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
	attr, ok := cfg.ConvertedAttributes.(*servoConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(&servoConfig{}, cfg.ConvertedAttributes)
	}

	boardName := attr.Board
	b, err := board.FromDependencies(deps, boardName)
	if err != nil {
		return nil, errors.Wrap(err, "board doesn't exist")
	}

	pin, err := b.GPIOPinByName(attr.Pin)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get servo pin")
	}

	frequency, err := pin.PWMFreq(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get servo pin pwm frequency")
	}
	if attr.Frequency != nil {
		if frequency > 450 || frequency == 0 {
			return nil, errors.Errorf("PWM frequencies should not be above 450Hz or 0, have %d", frequency)
		}

		err = pin.SetPWMFreq(ctx, *attr.Frequency, nil)
		if err != nil {
			return nil, errors.Wrap(err, "error setting servo pin frequency")
		}
		frequency = *attr.Frequency
	}

	minDeg := minDeg
	maxDeg := maxDeg
	if attr.MinDeg != nil {
		minDeg = *attr.MinDeg
	}
	if attr.MaxDeg != nil {
		maxDeg = *attr.MaxDeg
	}
	startPos := 0.0
	if attr.StartPos != nil {
		startPos = *attr.StartPos
	}
	minUs := minWidthUs
	maxUs := maxWidthUs
	if attr.MinWidthUS != nil {
		minUs = *attr.MinWidthUS
	}
	if attr.MaxWidthUS != nil {
		maxUs = *attr.MaxWidthUS
	}

	servo := &servoGeneric{
		min:       minDeg,
		max:       maxDeg,
		frequency: frequency,
		pin:       pin,
		logger:    logger,
		minUs:     minUs,
		maxUs:     maxUs,
		currPct:   0,
	}

	if err := servo.Move(ctx, uint8(startPos), nil); err != nil {
		return nil, errors.Wrap(err, "couldn't move servo to start position")
	}
	if servo.pwmRes == 0 {
		if err := servo.findPWMResolution(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to guess the pwm resolution")
		}
		if err := servo.Move(ctx, uint8(startPos), nil); err != nil {
			return nil, errors.Wrap(err, "couldn't move servo to start position")
		}
	}
	return servo, nil
}

var _ = servo.LocalServo(&servoGeneric{})

// Given minUs, maxUs, deg and frequency attempt to calculate the corresponding duty cycle pct.
func mapDegToDutyCylePct(minUs, maxUs uint, deg float64, frequency uint) float64 {
	period := 1.0 / float64(frequency) // dutyCycle in s
	degRange := maxDeg - minDeg        // servo moves from minDeg to maxDeg
	uSRange := float64(maxUs - minUs)  // pulse width between minUs to maxUs

	scale := uSRange / degRange

	pwmWidthUs := float64(minUs) + (deg-minDeg)*scale
	return (pwmWidthUs / (1000 * 1000)) / period
}

// Given minUs, maxUs, deg and frequency returns the corresponding duty cycle pct.
func mapDutyCylePctToDeg(minUs, maxUs uint, pct float64, frequency uint) float64 {
	period := 1.0 / float64(frequency) // dutyCycle in s
	pwmWidthUs := pct * period * 1000 * 1000
	degRange := maxDeg - minDeg       // servo moves from minDeg to maxDeg
	uSRange := float64(maxUs - minUs) // pulse width between minUs to maxUs

	pwmWidthUs = math.Max(float64(minUs), pwmWidthUs)
	pwmWidthUs = math.Min(float64(maxUs), pwmWidthUs)

	scale := degRange / uSRange

	return math.Round(minDeg + (pwmWidthUs-float64(minUs))*scale)
}

// Attempt to find the PWM resolution assuming a hardware PWM
//
//  1. assume a resolution of any 16,15,14,12,or 8 bit timer
//
//  2. Starting from the current PWM duty cycle we increase the duty cycle by
//     1/(1<<resolution) and check each new resolution until the returned duty cycle changes
//
//     if both the expected duty cycle and returned duty cycle are different we approximate
//     the resolution
func (s *servoGeneric) findPWMResolution(ctx context.Context) error {
	periodUs := (1.0 / float64(s.frequency)) * 1000 * 1000
	currPct := s.currPct
	realPct, err := s.pin.PWM(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "couldn't find PWM resolution")
	}
	dir := 1.0
	lDist := s.currPct*periodUs - float64(s.minUs)
	rDist := float64(s.maxUs) - s.currPct*periodUs
	if lDist > rDist {
		dir = -1.0
	}

	if realPct != currPct {
		if err := s.pin.SetPWM(ctx, realPct, nil); err != nil {
			return errors.Wrap(err, "couldn't set PWM to realPct")
		}
		r2, err := s.pin.PWM(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "couldn't find PWM resolution")
		}
		if r2 == realPct {
			currPct = r2
		} else {
			return errors.Errorf("giving up searching for the resolution tried to match %.7f but got %.7f", realPct, r2)
		}
	}
	resolution := []int{16, 15, 14, 12, 8}
	for _, r := range resolution {
		val := (1 << r) - 1
		pct := currPct + dir*1/float64(val)
		err := s.pin.SetPWM(ctx, pct, nil)
		if err != nil {
			return errors.Wrap(err, "couldn't search for PWM resolution")
		}
		if !viamutils.SelectContextOrWait(ctx, 3*time.Millisecond) {
			return errors.New("context canceled while looking for servo's PWM resolution")
		}
		realPct, err := s.pin.PWM(ctx, nil)
		s.logger.Debugf("starting step %d currPct %.7f target Pct %.14f realPct %.14f", val, currPct, pct, realPct)
		if err != nil {
			return errors.Wrap(err, "couldn't find PWM find servo PWM resolution")
		}
		if realPct != currPct {
			if realPct == pct {
				s.pwmRes = uint(val)
			} else {
				val = int(math.Abs(math.Round(1 / (currPct - realPct))))
				s.logger.Debugf("the servo moved but the expected duty cyle (%.7f) is not the one reported (%.7f) we are guessing %d",
					pct, realPct, val)
				s.pwmRes = uint(val)
			}
			break
		}
	}
	return nil
}

// Move moves the servo to the given angle (0-180 degrees)
// This will block until done or a new operation cancels this one.
func (s *servoGeneric) Move(ctx context.Context, ang uint8, extra map[string]interface{}) error {
	ctx, done := s.opMgr.New(ctx)
	defer done()
	angle := float64(ang)
	if angle < s.min {
		angle = s.min
	}
	if angle > s.max {
		angle = s.max
	}
	pct := mapDegToDutyCylePct(s.minUs, s.maxUs, angle, s.frequency)
	if s.pwmRes != 0 {
		realTick := math.Round(pct * float64(s.pwmRes))
		pct = realTick / float64(s.pwmRes)
	}
	if err := s.pin.SetPWM(ctx, pct, nil); err != nil {
		return errors.Wrap(err, "couldn't move the servo")
	}
	s.currPct = pct
	return nil
}

// Position returns the current set angle (degrees) of the servo.
func (s *servoGeneric) Position(ctx context.Context, extra map[string]interface{}) (uint8, error) {
	pct, err := s.pin.PWM(ctx, nil)
	if err != nil {
		return 0, errors.Wrap(err, "couldn't get servo pin duty cycle")
	}
	return uint8(mapDutyCylePctToDeg(s.minUs, s.maxUs, pct, s.frequency)), nil
}

// Stop stops the servo. It is assumed the servo stops immediately.
func (s *servoGeneric) Stop(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := s.opMgr.New(ctx)
	defer done()
	if err := s.pin.SetPWM(ctx, 0.0, nil); err != nil {
		return errors.Wrap(err, "couldn't stop servo")
	}
	return nil
}

// IsMoving returns whether or not the servo is moving.
func (s *servoGeneric) IsMoving(ctx context.Context) (bool, error) {
	res, err := s.pin.PWM(ctx, nil)
	if err != nil {
		return false, errors.Wrap(err, "servo error while checking if moving")
	}
	if int(res) == 0 {
		return false, nil
	}
	return s.opMgr.OpRunning(), nil
}
