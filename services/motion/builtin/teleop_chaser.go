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

const (
	defaultTeleopChaseGainPerSec = 15.0
	teleopChaseMaxGainPerSec     = 40.0
	teleopChaseBrakeFraction     = 0.9
	teleopChaseAccelRampTime     = 20 * time.Millisecond
	teleopChaseVelSettleSteps    = 2.0
	teleopChaseVelHeadroom       = 0.98
	teleopChaseRestVel           = 1e-3
	teleopChaseRestErr           = 1e-4
	teleopChaseDrainTimeout      = 3 * time.Second
	teleopChaseMaxBatch          = 50
	teleopChaseStopArmTimeout    = 10 * time.Second
)

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

func (c *jointChaser) setTarget(target []float64) {
	if len(target) != len(c.target) {
		return
	}
	copy(c.target, target)
}

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

func (c *jointChaser) step(dt time.Duration) {
	h := dt.Seconds()
	for i := range c.pos {
		brake := teleopChaseBrakeFraction * c.accMax[i]
		jerk := c.accMax[i] / teleopChaseAccelRampTime.Seconds()

		moving := 1.0
		if c.vel[i] < 0 || (c.vel[i] == 0 && c.target[i] < c.pos[i]) {
			moving = -1.0
		}
		swing := math.Max(0, moving*c.acc[i]+brake) / jerk
		coast := c.vel[i]*swing + c.acc[i]*swing*swing/2
		want := approachVel(c.target[i]-c.pos[i]-coast, c.gain, brake)
		wantMax := teleopChaseVelHeadroom * c.velMax[i]
		want = math.Max(-wantMax, math.Min(wantMax, want))

		dir := 1.0
		if want < c.vel[i] {
			dir = -1.0
		}
		gap := math.Abs(want - c.vel[i])
		prevAlong := dir * c.acc[i]
		room := math.Max(0, gap-prevAlong*h/2)
		rampable := -jerk*h/2 + math.Sqrt(jerk*jerk*h*h/4+2*jerk*room)
		along := math.Min(gap/(teleopChaseVelSettleSteps*h), math.Min(rampable, c.accMax[i]))
		maxStep := jerk * h
		along = math.Max(prevAlong-maxStep, math.Min(prevAlong+maxStep, along))
		acc := dir * math.Max(-c.accMax[i], math.Min(c.accMax[i], along))

		prevAcc := c.acc[i]
		vel := c.vel[i] + (prevAcc+acc)/2*h
		if math.Abs(vel) > c.velMax[i] {
			acc = 2*(math.Copysign(c.velMax[i], vel)-c.vel[i])/h - prevAcc
			acc = math.Max(-c.accMax[i], math.Min(c.accMax[i], acc))
			vel = c.vel[i] + (prevAcc+acc)/2*h
		}
		c.pos[i] += c.vel[i]*h + (2*prevAcc+acc)/6*h*h
		c.vel[i] = vel
		c.acc[i] = acc
	}
}

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

type teleopChase struct {
	gain float64
}

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
	defer cancelRPC()
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
		for range responses {
		}
	}()
	defer func() {
		if err != nil {
			cancelRPC()
		}
		close(batches)
		<-rpcDone
		if err == nil && rpcErr != nil {
			err = fmt.Errorf("failed to close arm stream: %w", rpcErr)
		}
		if err == nil {
			return
		}
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
		armClockZero time.Time
		sentUpTo     time.Duration
		sentAny      bool
		draining     bool
		drainSince   time.Time
	)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-rpcDone:
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
			var armRunway time.Duration
			if sentAny {
				armRunway = sentUpTo - time.Since(armClockZero)
			}
			diag.RecordArmRunway(armRunway)
			if armRunway < 0 {
				armClockZero = armClockZero.Add(-armRunway)
				armRunway = 0
			}

			var batch []arm.TrajectoryPoint
			if !sentAny {
				batch = append(batch, chaser.point(0))
				diag.RecordSampledPVAT(chaser.pos, chaser.vel, chaser.acc, 0)
			}
			for armRunway < runway && len(batch) < teleopChaseMaxBatch {
				chaser.step(interval)
				sentUpTo += interval
				armRunway += interval
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
					armClockZero = time.Now()
				}
			}

			if draining && (chaser.atRest() || time.Since(drainSince) > teleopChaseDrainTimeout) {
				if !chaser.atRest() {
					logger.CWarnf(ctx, "teleop chaser: arm had not settled %s after the stream was closed", teleopChaseDrainTimeout)
				}
				return nil
			}
		}
	}
}
