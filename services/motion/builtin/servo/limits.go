package servo

import (
	"fmt"
	"math"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

// Limits are the per-joint bounds a session never exceeds, in rad, rad/s and rad/s², plus the
// Cartesian caps in m/s and rad/s.
type Limits struct {
	Lo, Hi, Vel, Acc []float64
	MaxLinear        float64 // m/s; +Inf if unset
	MaxAngular       float64 // rad/s
}

// NewLimits merges the model's DoF limits with the caller's MoveOptions and angular cap (deg/s).
// The caller may only tighten. A joint whose velocity or acceleration the model leaves unset needs
// a caller value.
func NewLimits(model []referenceframe.Limit, opts arm.MoveOptions, maxAngularDegPerS float64) (Limits, error) {
	n := len(model)
	for _, s := range [][]float64{opts.MaxVelRadsJoints, opts.MaxAccRadsJoints} {
		if len(s) != 0 && len(s) != n {
			return Limits{}, fmt.Errorf("limits: per-joint options have %d entries, model has %d joints", len(s), n)
		}
	}
	l := Limits{
		Lo: make([]float64, n), Hi: make([]float64, n), Vel: make([]float64, n), Acc: make([]float64, n),
		MaxLinear:  math.Inf(1),
		MaxAngular: maxAngularDegPerS * math.Pi / 180,
	}
	for i, m := range model {
		l.Lo[i], l.Hi[i] = m.Min, m.Max
		var err error
		if l.Vel[i], err = merge(i, "velocity", "rad/s", m.MaxVelocity, opts.MaxVelRadsJoints, opts.MaxVelRads); err != nil {
			return Limits{}, err
		}
		if l.Acc[i], err = merge(i, "acceleration", "rad/s²", m.MaxAcceleration, opts.MaxAccRadsJoints, opts.MaxAccRads); err != nil {
			return Limits{}, err
		}
	}
	if v := opts.MaxTCPSpeedMPerSec; v != nil && *v > 0 {
		l.MaxLinear = *v
	}
	return l, nil
}

// merge resolves one joint's limit: the per-joint caller value, else the all-joints one, may only
// tighten the model's. A 0 entry in the per-joint slice falls back to the model's value, not to the
// all-joints option. (A model value ≤ 0 or nil is unset).
func merge(i int, name, unit string, model *float64, perJoint []float64, all float64) (float64, error) {
	caller := all
	if len(perJoint) > 0 {
		caller = perJoint[i]
	}
	m := 0.0
	if model != nil {
		m = *model
	}
	switch {
	case m <= 0 && caller <= 0:
		return 0, fmt.Errorf("limits: joint %d has no %s limit in the model and none was given", i, name)
	case m <= 0:
		return caller, nil
	case caller > m:
		return 0, fmt.Errorf("limits: joint %d %s %.3g %s is above the model's %.3g", i, name, caller, unit, m)
	case caller > 0:
		return caller, nil
	}
	return m, nil
}

// TightenLinear lowers MaxLinear to mPerS; values ≤ 0 are ignored.
func (l *Limits) TightenLinear(mPerS float64) {
	if mPerS > 0 {
		l.MaxLinear = math.Min(l.MaxLinear, mPerS)
	}
}

// brakeCap returns the largest step from which a joint can stop within d, decelerating
// by at most c = a·dt² per tick.
func brakeCap(d, c float64) float64 {
	if !(d > 0) || !(c > 0) { // also catches NaN
		return 0
	}
	if math.IsInf(d, 1) {
		return d
	}
	for n := 0.0; ; n++ {
		if s := (d + c*n*(n+1)/2) / (n + 1); s < (n+1)*c {
			return s
		}
	}
}

// Limited records which limits shaped a step.
type Limited uint8

// The Limited flags.
const (
	LimitedJointSpeed    Limited = 1 << iota // velocity or acceleration scaling
	LimitedJointPosition                     // braking toward, or clamping at, a position limit
)

// Apply turns a solver's joint step dq (rad) at joints q into the step to command, given the last
// commanded step: scale all joints together to the velocity caps, cap each joint at its exact
// discrete braking distance toward its position limit, scale the change from last to at most a·dt²
// per joint (all joints together), brake again (the scaling can push a joint back above its cap),
// then clamp to the position limits as a safety net. It reports which limits acted (zero if none).
// The v·dt bound assumes last already respects the current caps; if it doesn't, the acceleration
// clamp takes priority. A joint already outside its limits (q past Lo/Hi) is never moved further
// out and never pulled back in a single tick.
func (l Limits) Apply(q, dq, last []float64, dt float64) (step []float64, by Limited) {
	n := len(dq)
	if len(last) != n { // missing or mismatched last step: at rest
		last = make([]float64, n)
	}
	step = make([]float64, n)
	k := 1.0
	for i, d := range dq {
		if lim := l.Vel[i] * dt; math.Abs(d)*k > lim {
			k = lim / math.Abs(d)
		}
	}
	for i, d := range dq {
		step[i] = d * k
	}
	if k < 1 {
		by |= LimitedJointSpeed
	}
	brake := func() {
		for i := range step {
			d := l.Hi[i] - q[i]
			if step[i] < 0 {
				d = q[i] - l.Lo[i]
			}
			if c := brakeCap(d, l.Acc[i]*dt*dt); math.Abs(step[i]) > c {
				step[i] = math.Copysign(c, step[i])
				by |= LimitedJointPosition
			}
		}
	}
	brake()
	k = 1.0
	for i := range step {
		lv := last[i]
		if dv, c := math.Abs(step[i]-lv), l.Acc[i]*dt*dt; dv*k > c {
			k = c / dv
		}
	}
	if k < 1 {
		for i := range step {
			lv := last[i]
			step[i] = lv + k*(step[i]-lv)
		}
		by |= LimitedJointSpeed
	}
	brake()
	for i := range step { // should never engage for q inside the limits
		lo, hi := math.Min(l.Lo[i], q[i]), math.Max(l.Hi[i], q[i])
		if v := q[i] + step[i]; v < lo || v > hi {
			step[i] = math.Min(math.Max(v, lo), hi) - q[i]
			by |= LimitedJointPosition
		}
	}
	return step, by
}
