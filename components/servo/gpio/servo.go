// Package gpio implements a pin based servo
package gpio

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

const (
	defaultMinDeg float64 = 0.0
	defaultMaxDeg float64 = 180.0
	minWidthUs    uint    = 500  // absolute minimum PWM width
	maxWidthUs    uint    = 2500 // absolute maximum PWM width
	defaultFreq   uint    = 300
	minFreqHz     uint    = 50
	maxFreqHz     uint    = 450
)

// We want to distinguish values that are 0 because the user set them to 0 from ones that are 0
// because that's the default when the user didn't set them. Consequently, all numerical fields in
// this struct are pointers. They'll be nil if they were unset, and point to a value (possibly 0!)
// if they were set.
type servoConfig struct {
	Pin   string `json:"pin"`   // Pin is a GPIO pin with PWM capabilities.
	Board string `json:"board"` // Board is a board that exposes GPIO pins.
	// MinDeg is the minimum angle the servo can reach. Note this doesn't affect PWM calculation.
	MinDeg *float64 `json:"min_angle_deg,omitempty"`
	// MaxDeg is the maximum angle the servo can reach. Note this doesn't affect PWM calculation.
	MaxDeg *float64 `json:"max_angle_deg,omitempty"`
	// StartPos is the starting position of the servo in degrees.
	StartPos *float64 `json:"starting_position_deg,omitempty"`
	// Frequency at which to drive the PWM
	Frequency *uint `json:"frequency_hz,omitempty"`
	// Resolution of the PWM driver (eg number of ticks for a full period). If omitted or 0, the
	// driver will attempt to estimate the resolution.
	Resolution *uint `json:"pwm_resolution,omitempty"`
	// MinWidthUs overrides the safe minimum PWM width in microseconds.
	MinWidthUs *uint `json:"min_width_us,omitempty"`
	// MaxWidthUs overrides the safe maximum PWM width in microseconds.
	MaxWidthUs *uint `json:"max_width_us,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *servoConfig) Validate(path string) ([]string, []string, error) {
	var deps []string
	if config.Board == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, config.Board)
	if config.Pin == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "pin")
	}

	if config.StartPos != nil {
		minDeg := defaultMinDeg
		maxDeg := defaultMaxDeg
		if config.MinDeg != nil {
			minDeg = *config.MinDeg
		}
		if config.MaxDeg != nil {
			maxDeg = *config.MaxDeg
		}
		if *config.StartPos < minDeg || *config.StartPos > maxDeg {
			return nil, nil, resource.NewConfigValidationError(path,
				errors.Errorf("starting_position_deg should be between minimum (%.1f) and maximum (%.1f) positions", minDeg, maxDeg))
		}
	}

	if config.MinDeg != nil && *config.MinDeg < 0 {
		return nil, nil, resource.NewConfigValidationError(path, errors.New("min_angle_deg cannot be lower than 0"))
	}
	if config.MinWidthUs != nil && *config.MinWidthUs < minWidthUs {
		return nil, nil, resource.NewConfigValidationError(path, errors.Errorf("min_width_us cannot be lower than %d", minWidthUs))
	}
	if config.MaxWidthUs != nil && *config.MaxWidthUs > maxWidthUs {
		return nil, nil, resource.NewConfigValidationError(path, errors.Errorf("max_width_us cannot be higher than %d", maxWidthUs))
	}
	return deps, nil, nil
}

var model = resource.DefaultModelFamily.WithModel("gpio")

func init() {
	resource.RegisterComponent(servo.API, model,
		resource.Registration[servo.Servo, *servoConfig]{
			Constructor: newGPIOServo,
		})
}

type servoGPIO struct {
	resource.Named
	resource.TriviallyCloseable
	pin       board.GPIOPin
	minDeg    float64
	maxDeg    float64
	logger    logging.Logger
	opMgr     *operation.SingleOperationManager
	frequency uint
	minUs     uint
	maxUs     uint
	pwmRes    uint
	currPct   float64
	mu        sync.Mutex
}

func newGPIOServo(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (servo.Servo, error) {
	servo := &servoGPIO{
		Named:   conf.ResourceName().AsNamed(),
		logger:  logger,
		opMgr:   operation.NewSingleOperationManager(),
		currPct: 0,
	}

	// reconfigure
	if err := servo.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return servo, nil
}

func (s *servoGPIO) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newConf, err := resource.NativeConfig[*servoConfig](conf)
	if err != nil {
		return err
	}

	boardName := newConf.Board

	b, err := board.FromProvider(deps, boardName)
	if err != nil {
		return errors.Wrap(err, "board doesn't exist")
	}

	s.pin, err = b.GPIOPinByName(newConf.Pin)
	if err != nil {
		return errors.Wrap(err, "couldn't get servo pin")
	}

	s.minDeg = defaultMinDeg
	if newConf.MinDeg != nil {
		s.minDeg = *newConf.MinDeg
	}
	startPos := 0.0
	if newConf.StartPos != nil {
		startPos = *newConf.StartPos
	}

	s.maxDeg = defaultMaxDeg
	if newConf.MaxDeg != nil {
		s.maxDeg = *newConf.MaxDeg
	}

	s.minUs = minWidthUs
	if newConf.MinWidthUs != nil {
		s.minUs = *newConf.MinWidthUs
	}

	s.maxUs = maxWidthUs
	if newConf.MaxWidthUs != nil {
		s.maxUs = *newConf.MaxWidthUs
	}

	// If the frequency isn't specified in the config, we'll use whatever it's currently set to
	// instead. If it's currently set to 0, we'll default to using 300 Hz.
	s.frequency, err = s.pin.PWMFreq(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "couldn't get servo pin pwm frequency")
	}

	if s.frequency == 0 {
		s.frequency = defaultFreq
	}

	if newConf.Frequency != nil {
		if *newConf.Frequency > maxFreqHz || *newConf.Frequency < minFreqHz {
			return errors.Errorf(
				"PWM frequencies should not be above %dHz or below %dHz, have %dHz", maxFreqHz, minFreqHz, newConf.Frequency)
		}

		s.frequency = *newConf.Frequency
	}

	// We need the pin to be high for up to maxUs microseconds, plus the motor's deadband width
	// time spent low before going high again. The deadband width is usually at least 1
	// microsecond, but rarely over 10. Call it 50 microseconds just to be safe.
	const maxDeadbandWidthUs = 50
	if maxFrequency := 1e6 / (s.maxUs + maxDeadbandWidthUs); s.frequency > maxFrequency {
		// if the frequency wasn't set in the config we will default to whatever the driver sets as default frequency
		// we silence the warning below if clamping the frequency was needed
		if newConf.Frequency != nil {
			s.logger.CWarnf(ctx, "servo frequency (%dHz) is above maximum (%dHz), setting to max instead",
				s.frequency, maxFrequency)
		}
		s.frequency = maxFrequency
	}

	if err := s.pin.SetPWMFreq(ctx, s.frequency, nil); err != nil {
		return errors.Wrap(err, "error setting servo pin frequency")
	}

	// Try to detect the PWM resolution.
	if err := s.Move(ctx, uint32(startPos), nil); err != nil {
		return errors.Wrap(err, "couldn't move servo to start position")
	}

	if err := s.findPWMResolution(ctx); err != nil {
		return errors.Wrap(err, "failed to guess the pwm resolution")
	}

	if err := s.Move(ctx, uint32(startPos), nil); err != nil {
		return errors.Wrap(err, "couldn't move servo back to start position")
	}

	return nil
}

// Given minUs, maxUs, deg, and frequency attempt to calculate the corresponding duty cycle pct.
func mapDegToDutyCylePct(minUs, maxUs uint, minDeg, maxDeg, deg float64, frequency uint) float64 {
	period := 1.0 / float64(frequency)
	degRange := maxDeg - minDeg
	uSRange := float64(maxUs - minUs)

	usPerDeg := uSRange / degRange

	pwmWidthUs := float64(minUs) + (deg-minDeg)*usPerDeg
	return (pwmWidthUs / (1000 * 1000)) / period
}

// Given minUs, maxUs, duty cycle pct, and frequency returns the position in degrees.
func mapDutyCylePctToDeg(minUs, maxUs uint, minDeg, maxDeg, pct float64, frequency uint) float64 {
	period := 1.0 / float64(frequency)
	pwmWidthUs := pct * period * 1000 * 1000
	degRange := maxDeg - minDeg
	uSRange := float64(maxUs - minUs)

	pwmWidthUs = math.Max(float64(minUs), pwmWidthUs)
	pwmWidthUs = math.Min(float64(maxUs), pwmWidthUs)

	degsPerUs := degRange / uSRange
	returnVal := math.Round(minDeg + (pwmWidthUs-float64(minUs))*degsPerUs)
	return returnVal
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
func (s *servoGPIO) findPWMResolution(ctx context.Context) error {
	periodUs := (1.0 / float64(s.frequency)) * 1000 * 1000
	currPct := s.currPct
	realPct, err := s.pin.PWM(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cannot find PWM resolution")
	}

	// The direction will be towards whichever extreme duration (minUs or maxUs) is farther away.
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
		pct := currPct + dir/float64(val)
		err := s.pin.SetPWM(ctx, pct, nil)
		if err != nil {
			return errors.Wrap(err, "couldn't search for PWM resolution")
		}
		if !viamutils.SelectContextOrWait(ctx, 3*time.Millisecond) {
			return errors.New("context canceled while looking for servo's PWM resolution")
		}
		realPct, err := s.pin.PWM(ctx, nil)
		s.logger.CDebugf(ctx, "starting step %d currPct %.7f target Pct %.14f realPct %.14f", val, currPct, pct, realPct)
		if err != nil {
			return errors.Wrap(err, "couldn't find servo PWM resolution")
		}
		if realPct != currPct {
			if realPct == pct {
				s.pwmRes = uint(val)
			} else {
				val = int(math.Abs(math.Round(1 / (currPct - realPct))))
				s.logger.CDebugf(ctx, "the servo moved but the expected duty cyle (%.7f) is not the one reported (%.7f) we are guessing %d",
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
func (s *servoGPIO) Move(ctx context.Context, ang uint32, extra map[string]interface{}) error {
	ctx, done := s.opMgr.New(ctx)
	defer done()

	angle := float64(ang)

	if angle < s.minDeg {
		angle = s.minDeg
	}
	if angle > s.maxDeg {
		angle = s.maxDeg
	}

	pct := mapDegToDutyCylePct(s.minUs, s.maxUs, s.minDeg, s.maxDeg, angle, s.frequency)
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
func (s *servoGPIO) Position(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	pct, err := s.pin.PWM(ctx, nil)
	if err != nil {
		return 0, errors.Wrap(err, "couldn't get servo pin duty cycle")
	}
	// Since Stop() sets the dutyCycle to 0.0 in order to maintain the position of the servo,
	// we are setting the dutyCycle back to the last known dutyCycle to prevent the servo
	// from going back to zero position.
	if pct == 0 {
		pct = s.currPct
		err := s.pin.SetPWM(ctx, pct, extra)
		if err != nil {
			return 0, errors.Wrap(err, "couldn't get servo pin duty cycle")
		}
	}

	return uint32(mapDutyCylePctToDeg(s.minUs, s.maxUs, s.minDeg, s.maxDeg, pct, s.frequency)), nil
}

// Stop stops the servo. It is assumed the servo stops immediately.
func (s *servoGPIO) Stop(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := s.opMgr.New(ctx)
	defer done()
	// Turning the pin all the way off (i.e., setting the duty cycle to 0%) will cut power to the
	// motor. If you wanted to send it to position 0, you should set it to `minUs` instead.
	if err := s.pin.SetPWM(ctx, 0.0, nil); err != nil {
		return errors.Wrap(err, "couldn't stop servo")
	}
	return nil
}

// IsMoving returns whether or not the servo is moving.
func (s *servoGPIO) IsMoving(ctx context.Context) (bool, error) {
	res, err := s.pin.PWM(ctx, nil)
	if err != nil {
		return false, errors.Wrap(err, "servo error while checking if moving")
	}
	if int(res) == 0 {
		return false, nil
	}
	return s.opMgr.OpRunning(), nil
}
