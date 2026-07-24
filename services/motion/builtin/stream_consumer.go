package builtin

import (
	"context"
	"fmt"
	"math"
	"time"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/utils"
)

type pvatConsumer struct {
	arm     arm.Arm
	cfg     *streamConfig
	pvatsCh <-chan pvatChItem
	trace   *pipelineTrace

	stream *armStream // the current arm stream (one per trajex session)
}

func (c *pvatConsumer) run(ctx context.Context, r *consumerRunHandle) {
	defer close(r.done)
	fail := func(err error) {
		r.err = err
		r.cancel()
	}

	sendToArmInterval := time.Duration(c.cfg.SendToArmIntervalMs) * time.Millisecond
	ticker := time.NewTicker(sendToArmInterval)
	defer ticker.Stop()
	runway := time.Duration(c.cfg.BufferAheadInArmMs) * time.Millisecond
	floor := sendToArmInterval

	for {
		// Accept committed points only until the arm is buffered a full runway ahead.
		// Leaving recvCh nil disables the receive case.
		var recvCh <-chan pvatChItem
		if c.stream == nil || c.stream.estimatedDurationRemainingInArm() < runway {
			recvCh = c.pvatsCh
		}

		select {
		// Cancel was called.
		case <-ctx.Done():
			if c.stream != nil {
				c.stream.close() //nolint:errcheck
				c.trace.recordEvent(pipeEventStreamClose, "")
			}
			return

		// A new message on the pvat channel is available.
		case it, ok := <-recvCh:
			if !ok {
				// pvat channel closed
				if err := c.finishStream(ctx); err != nil {
					fail(err)
				}
				return
			}
			c.trace.record(pipeChanStreamQ, pipeOpDequeue, len(c.pvatsCh), cap(c.pvatsCh))
			if it.closeStream {
				if err := c.finishStream(ctx); err != nil {
					fail(err)
					return
				}
				continue
			}
			if c.stream == nil {
				c.stream = newArmStream(ctx, c.arm)
				c.trace.recordEvent(pipeEventStreamOpen, "")
			}
			c.trace.recordVelocity(maxAbsJointVelDegPerSec(it.p.velocities))
			if err := c.stream.add(it.p); err != nil {
				fail(err)
				return
			}

		// Time to send the accumulated batch.
		case <-ticker.C:
			// If this is the first batch, don't send it until it has enough to fill the runway.
			if c.stream == nil || (!c.stream.started() && c.stream.estimatedDurationRemainingInArm() < runway) {
				continue
			}
			sendStart := time.Now()
			if err := c.stream.send(ctx); err != nil {
				c.trace.recordEvent(pipeEventStreamDied, "")
				fail(err)
				return
			}
			c.trace.recordTiming(pipeTimingSendPoint, time.Since(sendStart))
			// A running arm whose buffer has drained to the floor means the producer can't keep it fed.
			remaining := c.stream.estimatedDurationRemainingInArm()
			// armQ occupancy is the estimated arm-side buffer in ms (len) against the runway target (cap).
			c.trace.record(pipeChanArmPending, pipeOpDequeue, int(remaining.Milliseconds()), c.cfg.BufferAheadInArmMs)
			if remaining <= floor {
				fail(fmt.Errorf("producer fell behind: arm buffer drained to %v (floor %v)", remaining, floor))
				return
			}
		}
	}
}

func (c *pvatConsumer) finishStream(ctx context.Context) error {
	if c.stream == nil {
		return nil
	}
	defer func() { c.stream = nil }()
	sendStart := time.Now()
	if err := c.stream.send(ctx); err != nil {
		return err
	}
	c.trace.recordTiming(pipeTimingSendPoint, time.Since(sendStart))
	c.trace.recordEvent(pipeEventStreamClose, "")
	return c.stream.close()
}

// maxAbsJointVelDegPerSec returns the largest absolute joint velocity (deg/s) across all joints
// of one PVAT. The input velocities are the trajex output, in rad/s.
func maxAbsJointVelDegPerSec(velocitiesRadPerSec []float64) float64 {
	var maxAbs float64
	for _, v := range velocitiesRadPerSec {
		if a := math.Abs(v); a > maxAbs {
			maxAbs = a
		}
	}
	return utils.RadToDeg(maxAbs)
}
