package builtin

import (
	"context"
	"math"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/servo"
)

// servoParamsFromConfig turns StreamServo's Init into servoParams, with the same defaults as servo_start.
// extra may carry "solver", "resync_deg" and "ramp_ms".
func servoParamsFromConfig(name string, cfg motion.ServoConfig, extra map[string]interface{}) (servoParams, error) {
	p := servoParams{arm: name, streamed: true}
	switch cfg.Mode {
	case motion.ServoModePose:
		p.mode = servo.ModePose
	case motion.ServoModeVelocity:
		p.mode = servo.ModeVelocity
	case motion.ServoModeUnspecified:
	}
	orDef := func(v *float64, i int) float64 {
		if v != nil {
			return *v
		}
		return servoRanges[i].def
	}
	p.maxAngDegs = orDef(cfg.MaxAngularVelDegsPerSec, 0)
	p.timeoutMS = servoRanges[1].def
	if cfg.TimeoutMs != nil {
		p.timeoutMS = float64(*cfg.TimeoutMs)
	}
	var err error
	if p.rampMS, err = num(extra, "ramp_ms", servoRanges[2].def); err != nil {
		return p, err
	}
	p.stepMM, p.stepDeg = orDef(cfg.MaxTargetStepMM, 3), orDef(cfg.MaxTargetStepDeg, 4)
	if cfg.Limits != nil {
		p.opts = *cfg.Limits
	}
	return p, p.parseExtra(extra)
}

func servoStateFrom(s servo.State) motion.ServoState {
	switch s {
	case servo.StateRunning:
		return motion.ServoStateRunning
	case servo.StateConverged:
		return motion.ServoStateConverged
	case servo.StateStalled:
		return motion.ServoStateStalled
	case servo.StateStopped:
		return motion.ServoStateStopped
	case servo.StateStoppedTimeout:
		return motion.ServoStateStoppedTimeout
	case servo.StateFaulted:
		return motion.ServoStateFaulted
	}
	return motion.ServoStateUnspecified
}

func servoLimitFrom(l servo.Limit) motion.ServoLimit {
	switch l {
	case servo.LimitJointPosition:
		return motion.ServoLimitJointPosition
	case servo.LimitJointSpeed:
		return motion.ServoLimitJointSpeed
	case servo.LimitTCPSpeed:
		return motion.ServoLimitTCPSpeed
	case servo.LimitSingularity:
		return motion.ServoLimitSingularity
	}
	return motion.ServoLimitUnspecified
}

func servoStatusFrom(s servo.Status) motion.ServoStatus {
	out := motion.ServoStatus{State: servoStateFrom(s.State), ErrMM: s.ErrMM, ErrDeg: s.ErrDeg}
	for _, l := range s.LimitedBy {
		out.LimitedBy = append(out.LimitedBy, servoLimitFrom(l))
	}
	return out
}

func servoStartedFrom(st servoStarted, p servoParams) *motion.ServoStarted {
	o := arm.MoveOptions{MaxVelRadsJoints: st.limits.Vel, MaxAccRadsJoints: st.limits.Acc}
	if !math.IsInf(st.limits.MaxLinear, 0) {
		o.MaxTCPSpeedMPerSec = &st.limits.MaxLinear
	}
	return &motion.ServoStarted{
		Limits: o, MaxAngularVelDegsPerSec: p.maxAngDegs, TimeoutMs: uint32(p.timeoutMS), RateHz: st.rateHz,
	}
}

// servoFFFrom converts a feed-forward twist (mm/s, deg/s) to the session's (m/s, rad/s); an empty frame is world.
func servoFFFrom(t *motion.ServoTwist) *servo.FeedForward {
	if t == nil {
		return nil
	}
	return &servo.FeedForward{Lin: t.LinearMMPerSec.Mul(1e-3), Ang: t.AngularDegsPerSec.Mul(deg2rad), Frame: worldIfEmpty(t.ReferenceFrame)}
}

func worldIfEmpty(f string) string {
	if f == "" {
		return "world"
	}
	return f
}

// applyServoUpdate hands u to the session. Its error is a rejection (or the session having ended).
func applyServoUpdate(sess *servo.Session, u motion.ServoUpdate) error {
	if u.Target != nil {
		t := u.Target
		if t.Parent() == "" {
			t = referenceframe.NewPoseInFrame("world", t.Pose())
		}
		_, err := sess.Target(t, nil, servoFFFrom(u.FeedForward))
		return err
	}
	v := u.Velocity
	_, err := sess.Velocity(v.LinearMMPerSec.Mul(1e-3), v.AngularDegsPerSec.Mul(deg2rad), worldIfEmpty(v.ReferenceFrame), nil)
	return err
}

// StreamServo runs one servo session for the life of the stream; see motion.Service.
func (ms *builtIn) StreamServo(
	ctx context.Context,
	componentName string,
	cfg motion.ServoConfig,
	updates <-chan motion.ServoUpdate,
	responses chan<- motion.ServoResponse,
	extra map[string]interface{},
) error {
	p, err := servoParamsFromConfig(componentName, cfg, extra)
	if err == nil {
		err = p.validate()
	}
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	ms.mu.RLock()
	_, known := ms.components[componentName]
	ms.mu.RUnlock()
	if !known {
		return status.Errorf(codes.NotFound, "no component named %q is known to the motion service", componentName)
	}
	st, err := ms.startServo(ctx, p) // an error here means nothing moved
	if err != nil {
		return status.Error(codes.FailedPrecondition, err.Error()) // busy arm, or one that cannot stream
	}
	e := st.entry
	sess := e.session
	send := func(r motion.ServoResponse) bool {
		select {
		case responses <- r:
			return true
		case <-ctx.Done():
			return false
		}
	}
	// finish ramps the session down (unless it is already ending, which keeps its own final state), waits for
	// its cleanup on a context that ignores ctx's cancellation, sends the final Status and picks the return.
	finish := func(cause error) error {
		if !sess.Stopping() {
			sess.BeginStop(servo.StateStopped)
		}
		werr := waitEntry(context.WithoutCancel(ctx), e)
		if werr != nil {
			ms.logger.Warnf("StreamServo on %q: %v", componentName, werr)
		}
		fin := sess.Status().Status
		if cause == nil && fin.State == servo.StateFaulted {
			if cause = sess.Err(); cause == nil {
				cause = status.Error(codes.Internal, "servo session faulted")
			}
		}
		if cause == nil {
			cause = ctx.Err()
		}
		if ctx.Err() == nil {
			s := servoStatusFrom(fin)
			send(motion.ServoResponse{Status: &s})
		}
		if cause != nil {
			return cause
		}
		return werr
	}

	if !send(motion.ServoResponse{Started: servoStartedFrom(st, p)}) {
		return finish(ctx.Err())
	}
	var n uint64
	last := servoStateFrom(sess.Status().State)
	tick := time.NewTicker(time.Duration(float64(time.Second) / st.rateHz))
	defer tick.Stop()
	for {
		select {
		case u, ok := <-updates:
			if !ok { // the client closed its send side: ramp to a stop
				return finish(nil)
			}
			n++
			if (cfg.Mode == motion.ServoModePose) != (u.Target != nil) || (u.Target == nil) == (u.Velocity == nil) ||
				(u.Velocity != nil && u.FeedForward != nil) {
				return finish(status.Errorf(codes.InvalidArgument,
					"update %d: must carry exactly one command, of the session's kind (%s); feed-forward goes with a target", n, p.mode))
			}
			var rej *string
			if err := applyServoUpdate(sess, u); err != nil {
				if sess.Stopping() { // the session ended on its own; its final Status follows
					return finish(nil)
				}
				msg := err.Error()
				rej = &msg
			}
			s := servoStatusFrom(sess.Status().Status)
			ack := n
			s.Ack, s.Rejected = &ack, rej
			if !send(motion.ServoResponse{Status: &s}) {
				return finish(ctx.Err())
			}
			last = s.State
		case <-tick.C:
			if cur := sess.Status().Status; servoStateFrom(cur.State) != last && !isTerminal(cur.State) {
				s := servoStatusFrom(cur)
				if !send(motion.ServoResponse{Status: &s}) {
					return finish(ctx.Err())
				}
				last = s.State
			}
		case <-sess.Done():
			return finish(nil)
		case <-ctx.Done():
			return finish(ctx.Err())
		}
	}
}

func isTerminal(s servo.State) bool {
	return s == servo.StateStopped || s == servo.StateStoppedTimeout || s == servo.StateFaulted
}
