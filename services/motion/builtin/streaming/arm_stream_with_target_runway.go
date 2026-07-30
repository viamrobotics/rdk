//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"fmt"
	"time"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

type armStreamWithTargetRunway struct {
	arm    arm.Arm
	pvatCh <-chan pvat

	targetRunway      time.Duration
	sendToArmInterval time.Duration

	timeInTrajectoryClockOfLastSentPVAT time.Duration
	timeFirstBatchWasSent               time.Time

	currentBatch []arm.TrajectoryPoint

	// State for the current underlying RPC stream.
	started     bool
	batchesCh   chan []arm.TrajectoryPoint
	responsesCh chan arm.Response
	done        chan struct{}
	err         error
}

// run receives PVATs off pvatCh and paces sending batches of PVATs to the arm to try to
// maintain the target runway duration buffered in the arm resource. It returns once pvatCh
// closes and any final batch has been flushed, or on the first error or cancellation.
func (s *armStreamWithTargetRunway) run(ctx context.Context, r *armStreamRunHandle) {
	defer close(r.done)

	ticker := time.NewTicker(s.sendToArmInterval)
	defer ticker.Stop()

	for {
		// Accept pvat points only while the arm is under the target runway.
		var recvCh <-chan pvat
		if s.currentEstimatedRunwayInArm()+s.pendingBatchDuration() < s.targetRunway {
			recvCh = s.pvatCh
		}

		select {
		// Cancel was called.
		case <-ctx.Done():
			if s.started {
				s.close() //nolint:errcheck
			}
			r.err = ctx.Err()
			return

		// A new message on the pvat channel is available.
		case p, ok := <-recvCh:
			if !ok {
				// pvat channel closed
				if err := s.flush(ctx); err != nil {
					r.err = err
					r.cancel()
				}
				return
			}
			if !s.started {
				s.startStream(ctx)
			}
			s.addToBatch(p)

		// It's time to check if we should send the current batch.
		case <-ticker.C:
			if !s.started {
				continue
			}
			if err := s.maybeSendBatch(ctx); err != nil {
				r.err = err
				r.cancel()
				return
			}
		}
	}
}

func (s *armStreamWithTargetRunway) startStream(ctx context.Context) {
	s.batchesCh = make(chan []arm.TrajectoryPoint)
	s.responsesCh = make(chan arm.Response)
	s.done = make(chan struct{})
	s.started = true

	go func() {
		err := s.arm.MoveThroughJointPositionsStreamed(ctx, s.batchesCh, s.responsesCh, nil)
		s.err = err
		close(s.responsesCh)
		close(s.done)
	}()
	go func() {
		// Drain acks so the impl never blocks writing them.
		for range s.responsesCh {
		}
	}()
}

func (s *armStreamWithTargetRunway) addToBatch(p pvat) {
	s.currentBatch = append(s.currentBatch, arm.TrajectoryPoint{
		Time:      p.time,
		Positions: append([]referenceframe.Input(nil), p.positions...),
		Constraints: &arm.KinematicConstraints{
			Velocities:    append([]float64(nil), p.velocities...),
			Accelerations: append([]float64(nil), p.accelerations...),
		},
	})
}

func (s *armStreamWithTargetRunway) maybeSendBatch(ctx context.Context) error {
	if !s.firstBatchSent() && s.pendingBatchDuration() < s.targetRunway {
		return nil
	}
	return s.sendBatch(ctx)
}

func (s *armStreamWithTargetRunway) sendBatch(ctx context.Context) error {
	if len(s.currentBatch) == 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return fmt.Errorf("arm streaming RPC ended before batch could be sent: %w", s.err)
	case s.batchesCh <- s.currentBatch:
	}

	s.timeInTrajectoryClockOfLastSentPVAT = s.currentBatch[len(s.currentBatch)-1].Time
	if !s.firstBatchSent() {
		// First batch was just sent; assume the arm begins executing around now.
		s.timeFirstBatchWasSent = time.Now()
	}
	s.currentBatch = nil
	return nil
}

func (s *armStreamWithTargetRunway) flush(ctx context.Context) error {
	if !s.started {
		return nil
	}
	// The trajectory is complete, so send whatever is pending.
	if err := s.sendBatch(ctx); err != nil {
		return err
	}
	return s.close()
}

func (s *armStreamWithTargetRunway) close() error {
	close(s.batchesCh)
	<-s.done
	s.started = false
	return s.err
}

func (s *armStreamWithTargetRunway) firstBatchSent() bool {
	return !s.timeFirstBatchWasSent.IsZero()
}

func (s *armStreamWithTargetRunway) pendingBatchDuration() time.Duration {
	if len(s.currentBatch) == 0 {
		return 0
	}
	return s.currentBatch[len(s.currentBatch)-1].Time - s.timeInTrajectoryClockOfLastSentPVAT
}

func (s *armStreamWithTargetRunway) currentEstimatedRunwayInArm() time.Duration {
	if !s.firstBatchSent() {
		return 0
	}
	// Elapsed wall-clock time since the arm started executing, converted to the trajectory
	// clock: assuming real-time playback, this is the arm's estimated current position.
	estimatedArmPositionInTrajectoryClock := time.Since(s.timeFirstBatchWasSent)
	return s.timeInTrajectoryClockOfLastSentPVAT - estimatedArmPositionInTrajectoryClock
}
