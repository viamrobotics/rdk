package inject

import (
	"context"

	"go.viam.com/rdk/component/board"
)

// GPIOPin is an injected GPIOPin.
type GPIOPin struct {
	board.GPIOPin

	SetFunc        func(ctx context.Context, high bool) error
	setCap         []interface{}
	GetFunc        func(ctx context.Context) (bool, error)
	getCap         []interface{}
	PWMFunc        func(ctx context.Context) (float64, error)
	pwmCap         []interface{}
	SetPWMFunc     func(ctx context.Context, dutyCyclePct float64) error
	setPWMCap      []interface{}
	PWMFreqFunc    func(ctx context.Context) (uint, error)
	pwmFreqCap     []interface{}
	SetPWMFreqFunc func(ctx context.Context, freqHz uint) error
	setPWMFreqCap  []interface{}
}

// Set calls the injected Set or the real version.
func (gp *GPIOPin) Set(ctx context.Context, high bool) error {
	gp.setCap = []interface{}{ctx, high}
	if gp.SetFunc == nil {
		return gp.GPIOPin.Set(ctx, high)
	}
	return gp.SetFunc(ctx, high)
}

// Get calls the injected Get or the real version.
func (gp *GPIOPin) Get(ctx context.Context) (bool, error) {
	gp.getCap = []interface{}{ctx}
	if gp.GetFunc == nil {
		return gp.GPIOPin.Get(ctx)
	}
	return gp.GetFunc(ctx)
}

// PWM calls the injected PWM or the real version.
func (gp *GPIOPin) PWM(ctx context.Context) (float64, error) {
	gp.pwmCap = []interface{}{ctx}
	if gp.PWMFunc == nil {
		return gp.GPIOPin.PWM(ctx)
	}
	return gp.PWMFunc(ctx)
}

// SetPWM calls the injected SetPWM or the real version.
func (gp *GPIOPin) SetPWM(ctx context.Context, dutyCyclePct float64) error {
	gp.setPWMCap = []interface{}{ctx, dutyCyclePct}
	if gp.SetPWMFunc == nil {
		return gp.GPIOPin.SetPWM(ctx, dutyCyclePct)
	}
	return gp.SetPWMFunc(ctx, dutyCyclePct)
}

// PWMFreq calls the injected PWMFreq or the real version.
func (gp *GPIOPin) PWMFreq(ctx context.Context) (uint, error) {
	gp.pwmFreqCap = []interface{}{ctx}
	if gp.PWMFreqFunc == nil {
		return gp.GPIOPin.PWMFreq(ctx)
	}
	return gp.PWMFreqFunc(ctx)
}

// SetPWMFreq calls the injected SetPWMFreq or the real version.
func (gp *GPIOPin) SetPWMFreq(ctx context.Context, freqHz uint) error {
	gp.setPWMFreqCap = []interface{}{ctx, freqHz}
	if gp.SetPWMFreqFunc == nil {
		return gp.GPIOPin.SetPWMFreq(ctx, freqHz)
	}
	return gp.SetPWMFreqFunc(ctx, freqHz)
}

// SetCap returns the last parameters received by Set, and then clears them.
func (gp *GPIOPin) SetCap() []interface{} {
	if gp == nil {
		return nil
	}
	defer func() { gp.setCap = nil }()
	return gp.setCap
}

// GetCap returns the last parameters received by Get, and then clears them.
func (gp *GPIOPin) GetCap() []interface{} {
	if gp == nil {
		return nil
	}
	defer func() { gp.getCap = nil }()
	return gp.getCap
}

// PWMCap returns the last parameters received by PWM, and then clears them.
func (gp *GPIOPin) PWMCap() []interface{} {
	if gp == nil {
		return nil
	}
	defer func() { gp.pwmCap = nil }()
	return gp.pwmCap
}

// SetPWMCap returns the last parameters received by SetPWM, and then clears them.
func (gp *GPIOPin) SetPWMCap() []interface{} {
	if gp == nil {
		return nil
	}
	defer func() { gp.setPWMCap = nil }()
	return gp.setPWMCap
}

// PWMFreqCap returns the last parameters received by PWMFreq, and then clears them.
func (gp *GPIOPin) PWMFreqCap() []interface{} {
	if gp == nil {
		return nil
	}
	defer func() { gp.pwmFreqCap = nil }()
	return gp.pwmFreqCap
}

// SetPWMFreqCap returns the last parameters received by SetPWMFreq, and then clears them.
func (gp *GPIOPin) SetPWMFreqCap() []interface{} {
	if gp == nil {
		return nil
	}
	defer func() { gp.setPWMFreqCap = nil }()
	return gp.setPWMFreqCap
}
