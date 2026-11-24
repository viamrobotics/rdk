package builtin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
)

// Defaults and tunables for the teleop chaser.
const (
	// defaultTeleopChaseGainPerSec is how hard the chaser closes the last of a position error:
	// near the target, velocity is this gain times the error, so the error decays with a time
	// constant of 1/gain.
	defaultTeleopChaseGainPerSec = 15.0
	// teleopChaseBrakeFraction is the share of the acceleration limit the chaser plans to brake
	// with. Keeping some in reserve leaves room for the acceleration ramp.
	teleopChaseBrakeFraction = 0.6
	// teleopChaseAccelRampTime is how long acceleration takes to go from zero to its limit. It
	// bounds jerk, so acceleration never steps.
	teleopChaseAccelRampTime = 40 * time.Millisecond
	// teleopChaseVelSettleSteps is how many steps the chaser takes to close a small velocity gap.
	// More than one keeps acceleration from flipping sign every step around the wanted velocity.
	teleopChaseVelSettleSteps = 2.0
	// teleopChaseVelHeadroom is the fraction of a joint's velocity limit the chaser aims for at
	// most, leaving room for the small overshoot of a discretely ramped acceleration.
	teleopChaseVelHeadroom = 0.98
	// teleopChaseRestVel and teleopChaseRestErr are how slow and how close, in radians, every
	// joint must be for the chaser to count as at rest on its target.
	teleopChaseRestVel = 1e-3
	teleopChaseRestErr = 1e-4
	// teleopChaseDrainTimeout bounds how long a closing stream waits for the arm to settle on
	// its last target.
	teleopChaseDrainTimeout = 3 * time.Second
	// teleopChaseMaxBatch bounds how many points one batch carries when the loop has fallen
	// behind and is catching up.
	teleopChaseMaxBatch       = 50
	teleopChaseStopArmTimeout = 10 * time.Second
)

// jointChaser generates joint motion that follows a moving target within per-joint velocity and
// acceleration limits. Its speed toward the target is proportional to how far behind it is, so
// it trails a moving target by a steady 1/gain seconds and eases to rest when the target stops.
// It does not predict where the target is going: an earlier version added the target's
// estimated velocity, which cut the lag but overshot on short, sharp movements. Each joint is
// chased independently.
type jointChaser struct {
	pos, vel, acc []float64
	velMax        []float64
	accMax        []float64

	gain   float64
	target []float64
}

func newJointChaser(start, velMax, accMax []float64, gain float64) *jointChaser {
	n := len(start)
	return &jointChaser{
		pos:    append([]float64(nil), start...),
		vel:    make([]float64, n),
		acc:    make([]float64, n),
		velMax: velMax,
		accMax: accMax,
		gain:   gain,
		target: append([]float64(nil), start...),
	}
}

// setTarget moves the target. A target with the wrong number of joints is ignored.
func (c *jointChaser) setTarget(target []float64) {
	if len(target) != len(c.target) {
		return
	}
	copy(c.target, target)
}

// approachVel is the speed to close a position error with: proportional to the error near the
// target, and the fastest speed that can still brake to a stop at the target further out.
func approachVel(err, gain, brake float64) float64 {
	linear := brake / (gain * gain)
	mag := math.Abs(err)
	var v float64
	if mag > linear {
		v = math.Sqrt(2 * brake * (mag - linear/2))
	} else {
		v = gain * mag
	}
	return math.Copysign(v, err)
}

// step advances the chaser by dt.
func (c *jointChaser) step(dt time.Duration) {
	h := dt.Seconds()
	for i := range c.pos {
		brake := teleopChaseBrakeFraction * c.accMax[i]
		jerk := c.accMax[i] / teleopChaseAccelRampTime.Seconds()

		// The joint cannot start braking at once: acceleration has to swing from its current
		// value to the braking value at the jerk limit, and the joint keeps travelling while it
		// does. coast is how far. Choosing speed from the error that will remain after that
		// swing, not the error now, is what stops a short sharp move from running past its
		// target: at the end of such a move the joint is still accelerating hard.
		moving := 1.0
		if c.vel[i] < 0 || (c.vel[i] == 0 && c.target[i] < c.pos[i]) {
			moving = -1.0
		}
		swing := math.Max(0, moving*c.acc[i]+brake) / jerk
		// This leaves out the small amount the falling acceleration takes back off the
		// distance, so it errs toward braking early. It also makes coast exactly zero at rest,
		// which lets the joint settle on its target.
		coast := c.vel[i]*swing + c.acc[i]*swing*swing/2
		want := approachVel(c.target[i]-c.pos[i]-coast, c.gain, brake)
		// Aim a little under the velocity limit: acceleration ramps down in discrete steps, so
		// velocity settles a hair past the value aimed for.
		wantMax := teleopChaseVelHeadroom * c.velMax[i]
		want = math.Max(-wantMax, math.Min(wantMax, want))

		// Choose the acceleration to have at the end of this step. Working along the direction
		// velocity needs to change in, it is limited four ways: in proportion to the velocity gap
		// (so it eases off as the gap closes), by the arm's limit, by how fast it may change
		// (jerk), and so that it can still be ramped back to zero by the time velocity reaches
		// the value wanted, which keeps velocity from running past it.
		dir := 1.0
		if want < c.vel[i] {
			dir = -1.0
		}
		gap := math.Abs(want - c.vel[i])
		prevAlong := dir * c.acc[i]
		// Ramping an acceleration a down to zero at this jerk adds a*a/(2*jerk) of velocity.
		// This step itself adds (prevAlong+a)/2*h on the way to a. rampable is the largest a
		// for which both still fit in the gap.
		room := math.Max(0, gap-prevAlong*h/2)
		rampable := -jerk*h/2 + math.Sqrt(jerk*jerk*h*h/4+2*jerk*room)
		along := math.Min(gap/(teleopChaseVelSettleSteps*h), math.Min(rampable, c.accMax[i]))
		maxStep := jerk * h
		along = math.Max(prevAlong-maxStep, math.Min(prevAlong+maxStep, along))
		acc := dir * math.Max(-c.accMax[i], math.Min(c.accMax[i], along))

		// acc is the acceleration at the end of this step. Over the step, acceleration ramps
		// linearly from the previous value to it (constant jerk), and velocity and position are
		// the exact integrals of that ramp. This matters to the arm: it fits a spline through
		// each pair of points using their positions, velocities and accelerations, and that
		// spline reproduces a constant-jerk segment exactly. Values that are not mutually
		// consistent force the spline to make up the difference within one interval, with
		// accelerations many times the limit (a UR rejects that: C306A43, "acceleration failed
		// to pass sanity check").
		prevAcc := c.acc[i]
		vel := c.vel[i] + (prevAcc+acc)/2*h
		if math.Abs(vel) > c.velMax[i] {
			// Not expected given the headroom above, but the limit is never to be exceeded.
			acc = 2*(math.Copysign(c.velMax[i], vel)-c.vel[i])/h - prevAcc
			acc = math.Max(-c.accMax[i], math.Min(c.accMax[i], acc))
			vel = c.vel[i] + (prevAcc+acc)/2*h
		}
		c.pos[i] += c.vel[i]*h + (2*prevAcc+acc)/6*h*h
		c.vel[i] = vel
		c.acc[i] = acc
	}
}

// atRest reports whether every joint has stopped on its target.
func (c *jointChaser) atRest() bool {
	for i := range c.pos {
		if math.Abs(c.vel[i]) > teleopChaseRestVel || math.Abs(c.target[i]-c.pos[i]) > teleopChaseRestErr {
			return false
		}
	}
	return true
}

func (c *jointChaser) point(t time.Duration) arm.TrajectoryPoint {
	return arm.TrajectoryPoint{
		Time:      t,
		Positions: append([]referenceframe.Input(nil), c.pos...),
		Constraints: &arm.KinematicConstraints{
			Velocities:    append([]float64(nil), c.vel...),
			Accelerations: append([]float64(nil), c.acc...),
		},
	}
}

// teleopChase is the tuning of the chaser one teleop pipeline uses.
type teleopChase struct {
	gain float64
}

// run drives one arm's MoveThroughJointPositionsStreamed stream from a moving joint target. It
// reads targets, always chasing the most recent one, and every opts.SendToArmInterval sends the
// arm enough new points to keep opts.ArmSideTargetRunway of motion buffered there. It sends
// points even while the arm is at rest, so the arm never sees its stream run dry. When targets
// is closed it settles the arm on the last target and ends the stream; when ctx is cancelled, or
// on any error, it stops the arm.
func (tc teleopChase) run(
	ctx context.Context,
	logger logging.Logger,
	a arm.Arm,
	opts streaming.StreamOptions,
	targets <-chan []referenceframe.Input,
	seed []referenceframe.Input,
	diag *diagnostics.SingleSessionDiagnostics,
) (err error) {
	velMax, accMax := opts.MoveOptions.MaxVelRadsJoints, opts.MoveOptions.MaxAccRadsJoints
	if len(velMax) != len(seed) || len(accMax) != len(seed) {
		return fmt.Errorf("need one velocity and one acceleration limit per joint: have %d and %d for %d joints",
			len(velMax), len(accMax), len(seed))
	}
	for i := range seed {
		if velMax[i] <= 0 || accMax[i] <= 0 {
			return fmt.Errorf("joint %d has a non-positive velocity or acceleration limit", i)
		}
	}
	interval, runway := opts.SendToArmInterval, opts.ArmSideTargetRunway
	if interval <= 0 || runway <= 0 {
		return errors.New("send interval and arm-side runway must be positive")
	}

	rpcCtx, cancelRPC := context.WithCancel(ctx)
	batches := make(chan []arm.TrajectoryPoint)
	responses := make(chan arm.Response)
	rpcDone := make(chan struct{})
	var rpcErr error
	diag.RecordArmStreamOpenEvent()
	go func() {
		rpcErr = a.MoveThroughJointPositionsStreamed(rpcCtx, batches, responses, nil)
		diag.RecordArmStreamCloseEvent()
		close(responses)
		close(rpcDone)
	}()
	go func() {
		// Drain acknowledgments so the arm never blocks writing them.
		for range responses {
		}
	}()
	batchesOpen := true
	defer func() {
		if err == nil {
			cancelRPC()
			return
		}
		// On error, cancel first so the RPC is interrupted, then make sure the arm is stopped.
		cancelRPC()
		if batchesOpen {
			close(batches)
		}
		<-rpcDone
		stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(ctx), teleopChaseStopArmTimeout)
		defer cancelStop()
		if stopErr := a.Stop(stopCtx, nil); stopErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to stop arm, arm not guaranteed to be stopped: %w", stopErr))
		}
	}()

	chaser := newJointChaser(seed, velMax, accMax, tc.gain)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var (
		started    time.Time     // when the first batch was sent
		sentUpTo   time.Duration // trajectory time of the last point sent
		sentAny    bool
		draining   bool
		drainSince time.Time
	)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-rpcDone:
			// Cancelling ctx also ends the RPC, and either may be seen first.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("arm streaming RPC ended while the stream was open: %w", rpcErr)
		case target, ok := <-targets:
			if !ok {
				targets = nil
				draining = true
				drainSince = time.Now()
				continue
			}
			if len(target) != len(seed) {
				return fmt.Errorf("target has %d joint positions, but the arm has %d joints", len(target), len(seed))
			}
			diag.RecordReceivedJointPositionTargetEvent()
			chaser.setTarget(target)
		case <-ticker.C:
			var elapsed time.Duration
			if sentAny {
				elapsed = time.Since(started)
			}
			diag.RecordArmRunway(sentUpTo - elapsed)

			var batch []arm.TrajectoryPoint
			if !sentAny {
				// The first point of a stream is the arm's starting state at time zero.
				batch = append(batch, chaser.point(0))
				diag.RecordSampledPVAT(chaser.pos, chaser.vel, chaser.acc, 0)
			}
			for sentUpTo < elapsed+runway && len(batch) < teleopChaseMaxBatch {
				chaser.step(interval)
				sentUpTo += interval
				batch = append(batch, chaser.point(sentUpTo))
				diag.RecordSampledPVAT(chaser.pos, chaser.vel, chaser.acc, sentUpTo)
			}
			if len(batch) > 0 {
				sendStart := time.Now()
				select {
				case batches <- batch:
				case <-rpcDone:
					if ctx.Err() != nil {
						return ctx.Err()
					}
					return fmt.Errorf("arm streaming RPC ended before batch could be sent: %w", rpcErr)
				case <-ctx.Done():
					return ctx.Err()
				}
				diag.RecordSendToArmLatency(sendStart, time.Since(sendStart))
				if !sentAny {
					sentAny = true
					started = time.Now()
				}
			}

			if draining && (chaser.atRest() || time.Since(drainSince) > teleopChaseDrainTimeout) {
				if !chaser.atRest() {
					logger.CWarnf(ctx, "teleop chaser: arm had not settled %s after the stream was closed", teleopChaseDrainTimeout)
				}
				// Ending the batches tells the arm the trajectory is complete; it returns once
				// it has executed what it was sent.
				batchesOpen = false
				close(batches)
				<-rpcDone
				if rpcErr != nil {
					return fmt.Errorf("failed to close arm stream: %w", rpcErr)
				}
				return nil
			}
		}
	}
}
