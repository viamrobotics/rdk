//go:build linux

package genericlinux

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
)

type periphGpioPin struct {
	b              *sysfsBoard
	pin            gpio.PinIO
	pinName        string
	hwPWMSupported bool
}

func (gp periphGpioPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	delete(gp.b.pwms, gp.pinName)

	return gp.set(high)
}

// This function is separate from Set(), above, because this one does not remove the pin from the
// board's pwms map. When simulating PWM in software, we use this function to turn the pin on and
// off while continuing to treat it as a PWM pin.
func (gp periphGpioPin) set(high bool) error {
	l := gpio.Low
	if high {
		l = gpio.High
	}
	return gp.pin.Out(l)
}

func (gp periphGpioPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return gp.pin.Read() == gpio.High, nil
}

func (gp periphGpioPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	gp.b.mu.RLock()
	defer gp.b.mu.RUnlock()

	pwm, ok := gp.b.pwms[gp.pinName]
	if !ok {
		return 0, fmt.Errorf("missing pin %s", gp.pinName)
	}
	return float64(pwm.dutyCycle) / float64(gpio.DutyMax), nil
}

func (gp periphGpioPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	last, alreadySet := gp.b.pwms[gp.pinName]
	var freqHz physic.Frequency
	if last.frequency != 0 {
		freqHz = last.frequency
	}
	duty := gpio.Duty(dutyCyclePct * float64(gpio.DutyMax))
	last.dutyCycle = duty
	gp.b.pwms[gp.pinName] = last

	if gp.hwPWMSupported {
		err := gp.pin.PWM(duty, freqHz)
		// TODO: [RSDK-569] (rh) find or implement a PWM sysfs that works with hardware pwm mappings
		// periph.io does not implement PWM
		if err != nil {
			return errors.New("sysfs PWM not currently supported, use another pin for software PWM loops")
		}
	}

	if !alreadySet {
		gp.b.startSoftwarePWMLoop(gp)
	}

	return nil
}

func (gp periphGpioPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	gp.b.mu.RLock()
	defer gp.b.mu.RUnlock()

	return uint(gp.b.pwms[gp.pinName].frequency / physic.Hertz), nil
}

func (gp periphGpioPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	last, alreadySet := gp.b.pwms[gp.pinName]
	var duty gpio.Duty
	if last.dutyCycle != 0 {
		duty = last.dutyCycle
	}
	frequency := physic.Hertz * physic.Frequency(freqHz)
	last.frequency = frequency
	gp.b.pwms[gp.pinName] = last

	if gp.hwPWMSupported {
		return gp.pin.PWM(duty, frequency)
	}

	if !alreadySet {
		gp.b.startSoftwarePWMLoop(gp)
	}

	return nil
}
