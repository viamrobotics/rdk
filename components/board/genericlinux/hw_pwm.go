// Package genericlinux is for Linux boards. This particular file is for using sysfs to
// interact with PWM devices.
package genericlinux

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
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

const pwmRootPath := "/sys/class/pwm"

func NewPwmDevice(chipName string, line int) pwmDevice {
	chipPath := fmt.Sprintf("%s/%s", pwmRootPath, chipName)
	linePath := fmt.Sprintf("%s/pwm%d", chipPath, line),
	return pwmDevice{chipPath: chipPath, line: line, linePath: linePath}
}

func writeValue(filepath string, value int) error {
	// The permissions on the file (the third argument) aren't important: if the file needs to be
	// created, something has gone horribly wrong.
	return os.WriteFile(filepath, []byte(fmt.Sprintf("%d", value)), 0o660)
}

func (pwm *pwmDevice) chipFile(filename string) string {
	return fmt.Sprintf("%s/%s", pwm.chipPath, filename)
}

func (pwm *pwmDevice) lineFile(filename string) string {
	return fmt.Sprintf("%s/%s", pwm.linePath, filename)
}

func (pwm *pwmDevice) Export() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	if pwm.isExported {
		return nil // Already exported
	}
	pwm.isEnabled = true
	return writeValue(pwm.chipFile("export"), pwm.line)
}

func (pwm *pwmDevice) Unexport() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	if !pwm.isExported {
		return nil // Already done
	}
	pwm.isEnabled = false
	return writeValue(pwm.chipFile("unexport"), pwm.line)
}

func (pwm *pwmDevice) Enable() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	if pwm.isEnabled {
		return nil // Already enabled
	}
	pwm.isEnabled = true
	return writeValue(pwm.lineFile(("enable"), 1)
}

func (pwm *pwmDevice) Disable() error {
	pwm.mu.Lock()
	defer pwm.mu.Unlock()

	if !pwm.isEnabled {
		return nil // Already disabled
	}
	pwm.isEnabled = false
	return writeValue(pwm.lineFile("enable"), 0)
}

func (pwm *pwmDevice) SetPwm(freqHz uint, dutyCycle float64) {
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
	pwm.mu.Lock()
	defer pwm.mu.Unlock()
	return pwm.Unexport()
}
