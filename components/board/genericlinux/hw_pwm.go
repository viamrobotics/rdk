// Package genericlinux is for Linux boards. This particular file is for using sysfs to
// interact with PWM devices. All of these functions are idempotent: you can double-export a pin or
// double-close it with no problems.
package genericlinux

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

type pwmDevice struct {
	// These values are immutable
	chipPath string
	line     int

	mu sync.Mutex

	// These values are mutable
	periodNs         uint64
	activeDurationNs uint64
	isExported       bool
	isEnabled        bool
}

func NewPwmDevice(chipName string, line int) (*pwmDevice, error) {
	// There should be a single directory within /sys/devices/platform/<chipName>/pwm/, whose name
	// is mirrored in /sys/class/pwm. That's the one we want to use.
	chipDir := fmt.Sprintf("/sys/devices/platform/%s/pwm", chipName)
	files, err := ioutil.ReadDir(chipDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if strings.Contains(file.Name(), "pwmchip") && file.IsDir() {
			chipPath := fmt.Sprintf("/sys/class/pwm/%s", file.Name())
			return &pwmDevice{chipPath: chipPath, line: line}, nil
		}
	}
	return nil, errors.Errorf("Could not find any PWM device with name %s", chipName)
}

func writeValue(filepath string, value uint64) error {
	// The permissions on the file (the third argument) aren't important: if the file needs to be
	// created, something has gone horribly wrong!
	return os.WriteFile(filepath, []byte(fmt.Sprintf("%d", value)), 0o660)
}

func (pwm *pwmDevice) writeChip(filename string, value uint64) error {
	return writeValue(fmt.Sprintf("%s/%s", pwm.chipPath, filename), value)
}

func (pwm *pwmDevice) writeLine(filename string, value uint64) error {
	return writeValue(fmt.Sprintf("%s/pwm%d/%s", pwm.chipPath, pwm.line, filename), value)
}

// Export tells the OS that this pin is in use, and enables configuration via sysfs.
func (pwm *pwmDevice) export() error {
	if pwm.isExported {
		return nil // Already exported
	}
	if err := pwm.writeChip("export", uint64(pwm.line)); err != nil {
		return err
	}
	pwm.isExported = true
	return nil
}

// Unexport tells the OS that this pin is no longer in use (so it can be reused as an input pin,
// etc.), and turns off any PWM signal the pin was providing.
func (pwm *pwmDevice) unexport() error {
	if !pwm.isExported {
		return nil // Already unexported
	}
	if err := pwm.writeChip("unexport", uint64(pwm.line)); err != nil {
		return err
	}
	pwm.isExported = false
	return nil
}

// Enable tells an exported pin to output the PWM signal it has been configured with.
func (pwm *pwmDevice) enable() error {
	if pwm.isEnabled {
		return nil // Already enabled
	}
	if err := pwm.writeLine("enable", 1); err != nil {
		return err
	}
	pwm.isEnabled = true
	return nil
}

// Disable tells an exported pin to stop outputting its PWM signal, but it is still available for
// reconfiguring and re-enabling.
func (pwm *pwmDevice) disable() error {
	if !pwm.isEnabled {
		return nil // Already disabled
	}
	if err := pwm.writeLine("enable", 0); err != nil {
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
	if err := pwm.export(); err != nil {
		return err
	}
	if err := pwm.disable(); err != nil {
		return err
	}

	// Sysfs has a pseudofile named duty_cycle which contains the number of nanoseconds that the
	// pin should be high within a period. It's not how the rest of the world defines a duty cycle,
	// so we will refer to it here as the active duration.
	periodNs := 1000 * 1000 * 1000 / uint64(freqHz)
	activeDurationNs := uint64(float64(periodNs) * dutyCycle)

	// We are never allowed to set the active duration higher than the period. Change the order we
	// set the values to ensure this.
	if periodNs < pwm.activeDurationNs {
		// The new period is smaller than the old active duration. Change the active duration
		// first. 
		if err := pwm.writeLine("duty_cycle", activeDurationNs); err != nil {
			return err
		}
		pwm.activeDurationNs = activeDurationNs

		if err := pwm.writeLine("period", periodNs); err != nil {
			return err
		}
		pwm.periodNs = periodNs
	} else {
		// The new period is at least as large as the old active duration. It's safe to change the
		// period first.
		if err := pwm.writeLine("period", periodNs); err != nil {
			return err
		}
		pwm.periodNs = periodNs

		if err := pwm.writeLine("duty_cycle", activeDurationNs); err != nil {
			return err
		}
		pwm.activeDurationNs = activeDurationNs
	}

	return pwm.enable()
}

func (pwm *pwmDevice) Close() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()
	return pwm.unexport()
}
