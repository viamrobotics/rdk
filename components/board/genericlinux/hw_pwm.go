//go:build linux

// Package genericlinux is for Linux boards. This particular file is for using sysfs to
// interact with PWM devices. All of these functions are idempotent: you can double-export a pin or
// double-close it with no problems.
package genericlinux

import (
	"fmt"
	"os"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

type pwmDevice struct {
	chipPath string
	line     int

	// We have no mutable state, but the mutex is used to write to multiple pseudofiles atomically.
	mu     sync.Mutex
	logger golog.Logger
}

func newPwmDevice(chipPath string, line int, logger golog.Logger) *pwmDevice {
	return &pwmDevice{chipPath: chipPath, line: line, logger: logger}
}

func writeValue(filepath string, value uint64) error {
	data := []byte(fmt.Sprintf("%d", value))
	// The file permissions (the third argument) aren't important: if the file needs to be created,
	// something has gone horribly wrong!
	err := os.WriteFile(filepath, data, 0o600)
	return errors.Wrap(err, filepath)
}

func (pwm *pwmDevice) writeChip(filename string, value uint64) error {
	return writeValue(fmt.Sprintf("%s/%s", pwm.chipPath, filename), value)
}

func (pwm *pwmDevice) linePath() string {
	return fmt.Sprintf("%s/pwm%d", pwm.chipPath, pwm.line)
}

func (pwm *pwmDevice) writeLine(filename string, value uint64) error {
	return writeValue(fmt.Sprintf("%s/%s", pwm.linePath(), filename), value)
}

// Export tells the OS that this pin is in use, and enables configuration via sysfs.
func (pwm *pwmDevice) export() error {
	if _, err := os.Lstat(pwm.linePath()); err != nil {
		if os.IsNotExist(err) {
			// The pseudofile we're trying to export doesn't yet exist. Export it now. This is the
			// happy path.
			return pwm.writeChip("export", uint64(pwm.line))
		}
		return err // Something unexpected has gone wrong.
	}
	// Otherwise, the line we're trying to export already exists.
	pwm.logger.Debugf("Skipping re-export of already-exported line %d on HW PWM chip %s",
		pwm.line, pwm.chipPath)
	return nil
}

// Unexport turns off any PWM signal the pin was providing, and tells the OS that this pin is no
// longer in use (so it can be reused as an input pin, etc.).
func (pwm *pwmDevice) unexport() error {
	if _, err := os.Lstat(pwm.linePath()); err != nil {
		if os.IsNotExist(err) {
			pwm.logger.Debugf("Skipping unexport of already-unexported line %d on HW PWM chip %s",
				pwm.line, pwm.chipPath)
			return nil
		}
		return err // Something has gone wrong.
	}

	// If we unexport the pin while it is enabled, it might continue outputting a PWM signal,
	// causing trouble if you start using the pin for something else.
	if err := pwm.disable(); err != nil {
		return err
	}
	if err := pwm.writeChip("unexport", uint64(pwm.line)); err != nil {
		return err
	}
	return nil
}

// Enable tells an exported pin to output the PWM signal it has been configured with.
func (pwm *pwmDevice) enable() error {
	// There is no harm in enabling an already-enabled pin; no errors will be returned if we try.
	return pwm.writeLine("enable", 1)
}

// Disable tells an exported pin to stop outputting its PWM signal, but it is still available for
// reconfiguring and re-enabling.
func (pwm *pwmDevice) disable() error {
	// There is no harm in disabling an already-disabled pin; no errors will be returned if we try.
	return pwm.writeLine("enable", 0)
}

// Only call this from public functions, to avoid double-wrapping the errors.
func (pwm *pwmDevice) wrapError(err error) error {
	// Note that if err is nil, errors.Wrap() will return nil, too.
	return errors.Wrap(err, fmt.Sprintf("HW PWM chipPath %s, line %d", pwm.chipPath, pwm.line))
}

// SetPwm configures an exported pin and enables its output signal.
// Warning: if this function returns a non-nil error, it could leave the pin in an indeterminate
// state. Maybe it's exported, maybe not. Maybe it's enabled, maybe not. The new frequency and duty
// cycle each might or might not be set.
func (pwm *pwmDevice) SetPwm(freqHz uint, dutyCycle float64) (err error) {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	// If there is ever an error in here, annotate it with which sysfs device and line we're using.
	defer func() {
		err = pwm.wrapError(err)
	}()

	// Every time this pin is used as a (non-PWM) GPIO input or output, it gets unexported on the
	// PWM chip. Make sure to re-export it here.
	if err := pwm.export(); err != nil {
		return err
	}
	if err := pwm.disable(); err != nil {
		// This is (surprisingly) okay: disabling the pin will return errors if its period and
		// active duration are 0, for example when they haven't been set since the last time the
		// system was rebooted. If the error was something more serious than that, we'll encounter
		// it again later in this function, and will return it then. (Note: disabling the pin will
		// not return errors if the period and active duration are nonzero, even if the pin is
		// already disabled!)
		pwm.logger.Debugf("Ignoring trouble disabling HW PWM device %s line %d: %s",
			pwm.chipPath, pwm.line, err)
	}

	// Sysfs has a pseudofile named duty_cycle which contains the number of nanoseconds that the
	// pin should be high within a period. It's not how the rest of the world defines a duty cycle,
	// so we will refer to it here as the active duration.
	periodNs := 1e9 / uint64(freqHz)
	activeDurationNs := uint64(float64(periodNs) * dutyCycle)

	// If we ever try setting the active duration higher than the period (or the period lower than
	// the active duration), we will get an error. Try setting one, then the other, then the first
	// one again. If the first of those three had errors by being too small/large, the last one
	// should take care of it. So, we purposely ignore any errors on the first value we try to
	// write.
	if err := pwm.writeLine("period", periodNs); err != nil {
		// This is okay; we'll change the active duration and then try setting the period again.
		pwm.logger.Debugf("Ignoring trouble setting the period on HW PWM device %s line %d: %s",
			pwm.chipPath, pwm.line, err)
	}
	if err := pwm.writeLine("duty_cycle", activeDurationNs); err != nil {
		return err
	}
	if err := pwm.writeLine("period", periodNs); err != nil {
		return err
	}

	return pwm.enable()
}

func (pwm *pwmDevice) Close() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()
	return pwm.wrapError(pwm.unexport())
}
