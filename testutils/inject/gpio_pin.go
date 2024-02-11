package inject

import (
	"context"

	"go.viam.com/rdk/components/board"
)

// GPIOPin is an injected GPIOPin.
type GPIOPin struct {
	board.GPIOPin

	SetFunc        func(ctx context.Context, high bool, extra map[string]any) error
	setCap         []any
	GetFunc        func(ctx context.Context, extra map[string]any) (bool, error)
	getCap         []any
	PWMFunc        func(ctx context.Context, extra map[string]any) (float64, error)
	pwmCap         []any
	SetPWMFunc     func(ctx context.Context, dutyCyclePct float64, extra map[string]any) error
	setPWMCap      []any
	PWMFreqFunc    func(ctx context.Context, extra map[string]any) (uint, error)
	pwmFreqCap     []any
	SetPWMFreqFunc func(ctx context.Context, freqHz uint, extra map[string]any) error
	setPWMFreqCap  []any
}

// Set calls the injected Set or the real version.
func (gp *GPIOPin) Set(ctx context.Context, high bool, extra map[string]any) error {
	gp.setCap = []any{ctx, high}
	if gp.SetFunc == nil {
		return gp.GPIOPin.Set(ctx, high, extra)
	}
	return gp.SetFunc(ctx, high, extra)
}

// Get calls the injected Get or the real version.
func (gp *GPIOPin) Get(ctx context.Context, extra map[string]any) (bool, error) {
	gp.getCap = []any{ctx}
	if gp.GetFunc == nil {
		return gp.GPIOPin.Get(ctx, extra)
	}
	return gp.GetFunc(ctx, extra)
}

// PWM calls the injected PWM or the real version.
func (gp *GPIOPin) PWM(ctx context.Context, extra map[string]any) (float64, error) {
	gp.pwmCap = []any{ctx}
	if gp.PWMFunc == nil {
		return gp.GPIOPin.PWM(ctx, extra)
	}
	return gp.PWMFunc(ctx, extra)
}

// SetPWM calls the injected SetPWM or the real version.
func (gp *GPIOPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]any) error {
	gp.setPWMCap = []any{ctx, dutyCyclePct}
	if gp.SetPWMFunc == nil {
		return gp.GPIOPin.SetPWM(ctx, dutyCyclePct, extra)
	}
	return gp.SetPWMFunc(ctx, dutyCyclePct, extra)
}

// PWMFreq calls the injected PWMFreq or the real version.
func (gp *GPIOPin) PWMFreq(ctx context.Context, extra map[string]any) (uint, error) {
	gp.pwmFreqCap = []any{ctx}
	if gp.PWMFreqFunc == nil {
		return gp.GPIOPin.PWMFreq(ctx, extra)
	}
	return gp.PWMFreqFunc(ctx, extra)
}

// SetPWMFreq calls the injected SetPWMFreq or the real version.
func (gp *GPIOPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]any) error {
	gp.setPWMFreqCap = []any{ctx, freqHz}
	if gp.SetPWMFreqFunc == nil {
		return gp.GPIOPin.SetPWMFreq(ctx, freqHz, extra)
	}
	return gp.SetPWMFreqFunc(ctx, freqHz, extra)
}

// SetCap returns the last parameters received by Set, and then clears them.
func (gp *GPIOPin) SetCap() []any {
	if gp == nil {
		return nil
	}
	defer func() { gp.setCap = nil }()
	return gp.setCap
}

// GetCap returns the last parameters received by Get, and then clears them.
func (gp *GPIOPin) GetCap() []any {
	if gp == nil {
		return nil
	}
	defer func() { gp.getCap = nil }()
	return gp.getCap
}

// PWMCap returns the last parameters received by PWM, and then clears them.
func (gp *GPIOPin) PWMCap() []any {
	if gp == nil {
		return nil
	}
	defer func() { gp.pwmCap = nil }()
	return gp.pwmCap
}

// SetPWMCap returns the last parameters received by SetPWM, and then clears them.
func (gp *GPIOPin) SetPWMCap() []any {
	if gp == nil {
		return nil
	}
	defer func() { gp.setPWMCap = nil }()
	return gp.setPWMCap
}

// PWMFreqCap returns the last parameters received by PWMFreq, and then clears them.
func (gp *GPIOPin) PWMFreqCap() []any {
	if gp == nil {
		return nil
	}
	defer func() { gp.pwmFreqCap = nil }()
	return gp.pwmFreqCap
}

// SetPWMFreqCap returns the last parameters received by SetPWMFreq, and then clears them.
func (gp *GPIOPin) SetPWMFreqCap() []any {
	if gp == nil {
		return nil
	}
	defer func() { gp.setPWMFreqCap = nil }()
	return gp.setPWMFreqCap
}
