// Package genericlinux is for Linux boards. This particular file is for using sysfs to
// interact with PWM devices.
package genericlinux

import (
	"fmt"
	"os"
	"sync"
)

type pwmDevice struct {
	// These values are immutable
	chipPath string
	line     int
	linePath string

	mu sync.Mutex

	// These values are mutable
	periodNs         uint64
	activeDurationNs uint64
	isExported       bool
	isEnabled        bool
}

func NewPwmDevice(chipName string, line int) pwmDevice {
	// Everything in /sys/class/pwm is a symlink to this other directory, which uses the chip names
	// instead of their aliases. These true names match up with the ones in our pin definitions.
	chipPath := fmt.Sprintf("/sys/devices/platform/%s", chipName)
	linePath := fmt.Sprintf("%s/pwm%d", chipPath, line),
	return pwmDevice{chipPath: chipPath, line: line, linePath: linePath}
}

func writeValue(filepath string, value int) error {
	// The permissions on the file (the third argument) aren't important: if the file needs to be
	// created, something has gone horribly wrong!
	return os.WriteFile(filepath, []byte(fmt.Sprintf("%d", value)), 0o660)
}

func (pwm *pwmDevice) chipFile(filename string) string {
	return fmt.Sprintf("%s/%s", pwm.chipPath, filename)
}

func (pwm *pwmDevice) lineFile(filename string) string {
	return fmt.Sprintf("%s/%s", pwm.linePath, filename)
}

// Export tells the OS that this pin is in use, and enables configuration via sysfs.
func (pwm *pwmDevice) Export() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	if pwm.isExported {
		return nil // Already exported
	}
	if err := writeValue(pwm.chipFile("export"), pwm.line); err != nil {
		return err
	}
	pwm.isExported = true
	return nil
}

// Unexport tells the OS that this pin is no longer in use, and turns off any PWM signal the pin
// was providing.
func (pwm *pwmDevice) Unexport() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	if !pwm.isExported {
		return nil // Already unexported
	}
	if err := writeValue(pwm.chipFile("unexport"), pwm.line); err != nil {
		return err
	}
	pwm.isExported = false
	return nil
}

// Enable tells an exported pin to output the PWM signal it has been configured with.
func (pwm *pwmDevice) Enable() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	if pwm.isEnabled {
		return nil // Already enabled
	}
	if err := writeValue(pwm.lineFile(("enable"), 1); err != nil {
		return err
	}
	pwm.isEnabled = true
	return nil
}

// Disable tells an exported pin to stop outputting its PWM signal.
func (pwm *pwmDevice) Disable() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	if !pwm.isEnabled {
		return nil // Already disabled
	}
	if err := writeValue(pwm.lineFile("enable"), 0); err != nil {
		return err
	}
	pwm.isEnabled = false
	return nil
}

// SetPwm configures an exported pin and enables its output signal.
// Warning: if this function returns a non-nil error, it could leave the pin in an indeterminate
// state. Maybe it's exported, maybe not. Maybe it's enabled, maybe not. The new frequency and duty
// cycle each might or might not be set.
func (pwm *pwmDevice) SetPwm(freqHz uint, dutyCycle float64) error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	// What we really want in this function is a monad: for every interaction with sysfs, check if
	// it returned an error, and if so return early.
	if err := pwm.Export(); err != nil {
		return err
	}
	if err := pwm.Disable(); err != nil {
		return err
	}

	// Sysfs has a pseudofile named duty_cycle which contains the number of nanoseconds that the
	// pin should be high within a period. It's not how the rest of the world defines a duty cycle,
	// so we will refer to it as the active duration.
	periodNs := 1000 * 1000 * 1000 / freqHz
	activeDurationNs := int(periodNs * dutyCycle)

	// We are never allowed to set the active duration higher than the period. Change the order we
	// set the values to ensure this.
	if periodNs < pwm.activeDurationNs {
		// The new period is smaller than the old active duration. Change the active duration
		// first. 
		if err := writeValue(pwm.lineFile("duty_cycle"), activeDurationNs); err != nil {
			return err
		}
		pwm.activeDurationNs = activeDurationNs

		if err := writeValue(pwm.lineFile("period"), periodNs); err != nil {
			return err
		}
		pwm.periodNs = periodNs
	} else {
		// The new period is at least as large as the old active duration. It's safe to change the
		// period first.
		if err := writeValue(pwm.lineFile("period"), periodNs); err != nil {
			return err
		}
		pwm.periodNs = periodNs

		if err := writeValue(pwm.lineFile("duty_cycle"), activeDurationNs); err != nil {
			return err
		}
		pwm.activeDurationNs = activeDurationNs
	}

	if err := pwm.Enable(); err != nil {
		return err
	}
}

func (pwm *pwmDevice) Close() error {
	// Don't lock the mutex here: it gets locked in Unexport.
	return pwm.Unexport()
}
